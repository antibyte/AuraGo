package memory

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"aurago/internal/httporigin"
	"aurago/internal/testutil"
)

func TestOpenAICompatEmbeddingRequestAndSameOriginRedirect(t *testing.T) {
	var hits atomic.Int32
	server := testutil.NewHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if r.Method != http.MethodPost || r.URL.Query().Get("api-version") != "2025-01-01" {
			t.Errorf("request method/query = %s/%q", r.Method, r.URL.RawQuery)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("authorization = %q", r.Header.Get("Authorization"))
		}
		if r.URL.Path == "/v1/embeddings" {
			w.Header().Set("Location", "/v2/embeddings?api-version=2025-01-01")
			w.WriteHeader(http.StatusTemporaryRedirect)
			return
		}
		if r.URL.Path != "/v2/embeddings" {
			t.Errorf("redirect path = %q", r.URL.Path)
		}
		var request struct {
			Input string `json:"input"`
			Model string `json:"model"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Errorf("decode request: %v", err)
		} else if request.Input != "hello" || request.Model != "embed-model" {
			t.Errorf("request = %#v", request)
		}
		_, _ = io.WriteString(w, `{"data":[{"embedding":[3,4]}]}`)
	}))
	defer server.Close()

	baseURL := server.URL + "/v1/?api-version=2025-01-01"
	fn := newOpenAICompatEmbeddingFunc(baseURL, "test-key", "embed-model")
	got, err := fn(context.Background(), "hello")
	if err != nil {
		t.Fatalf("embedding: %v", err)
	}
	if len(got) != 2 || got[0] != 0.6 || got[1] != 0.8 {
		t.Fatalf("normalized vector = %v, want [0.6 0.8]", got)
	}
	if n := hits.Load(); n != 2 {
		t.Fatalf("request count = %d, want 2", n)
	}
}

func TestOpenAICompatEmbeddingRejectsCrossOriginRedirectWithoutRetry(t *testing.T) {
	var secondHits atomic.Int32
	second := testutil.NewHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		secondHits.Add(1)
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"data":[{"embedding":[1]}]}`)
	}))
	defer second.Close()
	var firstHits atomic.Int32
	first := testutil.NewHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		firstHits.Add(1)
		w.Header().Set("Location", second.URL+r.URL.Path)
		w.WriteHeader(http.StatusTemporaryRedirect)
	}))
	defer first.Close()

	oldDelay := embeddingRetryDelay
	embeddingRetryDelay = func(int) time.Duration { return 0 }
	t.Cleanup(func() { embeddingRetryDelay = oldDelay })
	fn := withEmbeddingRetry(newOpenAICompatEmbeddingFunc(first.URL+"/v1?token=private", "test-key", "model"), nil)
	_, err := fn(context.Background(), "hello")
	if !errors.Is(err, httporigin.ErrCrossOriginRedirect) {
		t.Fatalf("error = %v, want cross-origin redirect error", err)
	}
	if firstHits.Load() != 1 || secondHits.Load() != 0 {
		t.Fatalf("first/second hits = %d/%d, want 1/0", firstHits.Load(), secondHits.Load())
	}
	if strings.Contains(err.Error(), "private") || strings.Contains(err.Error(), "test-key") {
		t.Fatalf("error leaked credential material: %q", err)
	}
}

func TestOpenAICompatEmbeddingRetriesTypedStatusesAtMostThreeTimes(t *testing.T) {
	for _, status := range []int{http.StatusServiceUnavailable, http.StatusBadRequest} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			var hits atomic.Int32
			server := testutil.NewHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				hits.Add(1)
				w.WriteHeader(status)
				_, _ = io.WriteString(w, `{"error":"private provider response"}`)
			}))
			defer server.Close()

			oldDelay := embeddingRetryDelay
			embeddingRetryDelay = func(int) time.Duration { return 0 }
			t.Cleanup(func() { embeddingRetryDelay = oldDelay })
			fn := withEmbeddingRetry(newOpenAICompatEmbeddingFunc(server.URL+"/v1", "key", "model"), nil)
			_, err := fn(context.Background(), "hello")
			if err == nil {
				t.Fatal("expected status error")
			}
			wantHits := int32(1)
			if status == http.StatusServiceUnavailable {
				wantHits = embeddingRetryAttempts
			}
			if hits.Load() != wantHits {
				t.Fatalf("request count = %d, want %d", hits.Load(), wantHits)
			}
			if strings.Contains(err.Error(), "private provider response") {
				t.Fatalf("error exposed provider response body: %q", err)
			}
		})
	}
}

func TestOpenAICompatEmbeddingHonorsShorterCallerContext(t *testing.T) {
	var hits atomic.Int32
	server := testutil.NewHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		time.Sleep(200 * time.Millisecond)
		_, _ = io.WriteString(w, `{"data":[{"embedding":[1]}]}`)
	}))
	defer server.Close()

	client := newEmbeddingHTTPClient(server.URL)
	if client.Timeout != embeddingRequestTimeout {
		t.Fatalf("HTTP client timeout = %v, want %v", client.Timeout, embeddingRequestTimeout)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	fn := withEmbeddingRetry(newOpenAICompatEmbeddingFunc(server.URL+"/v1", "", "model"), nil)
	started := time.Now()
	_, err := fn(ctx, "hello")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error = %v, want caller deadline", err)
	}
	if elapsed := time.Since(started); elapsed > 500*time.Millisecond {
		t.Fatalf("caller deadline took %v to return", elapsed)
	}
	if hits.Load() != 1 {
		t.Fatalf("request count = %d, want 1 after caller deadline", hits.Load())
	}
}

func TestLoopbackEmbeddingHTTPSAllowsSelfSignedTLS(t *testing.T) {
	server := testutil.NewHTTPSServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"data":[{"embedding":[1,0]}]}`)
	}))
	defer server.Close()

	if !isLoopbackEmbeddingHTTPS(server.URL) {
		t.Fatalf("test endpoint %q not recognized as loopback HTTPS", server.URL)
	}
	fn := newOpenAICompatEmbeddingFunc(server.URL+"/v1", "", "model")
	if _, err := fn(context.Background(), "hello"); err != nil {
		t.Fatalf("loopback self-signed request failed: %v", err)
	}
}

func TestRemoteEmbeddingHTTPSStillVerifiesCertificates(t *testing.T) {
	server := testutil.NewHTTPSServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("remote certificate failure must occur before HTTP request reaches handler")
	}))
	defer server.Close()

	defaultTransport, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		t.Fatal("default transport is not *http.Transport")
	}
	transport := defaultTransport.Clone()
	transport.Proxy = nil
	dialer := &net.Dialer{Timeout: 5 * time.Second}
	transport.DialContext = func(ctx context.Context, network, _ string) (net.Conn, error) {
		return dialer.DialContext(ctx, network, server.Listener.Addr().String())
	}
	t.Cleanup(transport.CloseIdleConnections)

	const remoteURL = "https://remote.invalid/v1"
	if isLoopbackEmbeddingHTTPS(remoteURL) {
		t.Fatal("remote HTTPS host incorrectly qualified for the loopback certificate exception")
	}
	client := newEmbeddingHTTPClientWithTransport(remoteURL, transport)
	if client.Transport != transport {
		t.Fatal("remote endpoint transport was replaced by the loopback exception")
	}
	fn := newOpenAICompatEmbeddingFuncWithClient(remoteURL, "", "model", client)
	_, err := fn(context.Background(), "hello")
	if err == nil || !strings.Contains(err.Error(), "embedding HTTP request failed") {
		t.Fatalf("remote self-signed TLS error = %v, want a transport verification failure", err)
	}
	if strings.Contains(err.Error(), "remote.invalid") {
		t.Fatalf("error leaked configured remote URL: %q", err)
	}
	if shouldRetryEmbeddingError(context.Background(), err) {
		t.Fatal("remote certificate verification failure must be final, not retried")
	}
}

func TestRedactEmbeddingURLDropsCredentialsAndQuery(t *testing.T) {
	got := redactEmbeddingURL("https://user:password@example.invalid/v1?api_key=secret#fragment")
	if got != "https://example.invalid" {
		t.Fatalf("redacted URL = %q", got)
	}
}
