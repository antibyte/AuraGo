package memory

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"aurago/internal/httporigin"
	"aurago/internal/testutil"
)

// Live fixtures for the same-origin redirect policy of the multimodal
// embedding client (audit H9). Server A is the configured embedding endpoint
// and answers with a redirect to server B, a second origin on another loopback
// port. B must never see a request, the key or the file content.

func writeMultimodalRedirectFixture(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "photo.png")
	if err := os.WriteFile(path, []byte("SECRET-IMAGE-CONTENT"), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestMultimodalEmbedderDoesNotFollowCrossOriginRedirect(t *testing.T) {
	for _, format := range []string{"openai", "vertex"} {
		t.Run(format, func(t *testing.T) {
			var secondHits atomic.Int32
			var secondSawAuth atomic.Bool
			second := testutil.NewHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				secondHits.Add(1)
				if r.Header.Get("Authorization") != "" {
					secondSawAuth.Store(true)
				}
				w.WriteHeader(http.StatusOK)
			}))
			defer second.Close()
			var providerHits atomic.Int32
			provider := testutil.NewHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				providerHits.Add(1)
				w.Header().Set("Location", second.URL+r.URL.Path)
				w.WriteHeader(http.StatusTemporaryRedirect)
			}))
			defer provider.Close()

			embedder := NewMultimodalEmbedder(provider.URL+"/v1", "test-key", "embed-model", format, "", nil)
			_, err := embedder.EmbedFile(context.Background(), writeMultimodalRedirectFixture(t))
			if !errors.Is(err, httporigin.ErrCrossOriginRedirect) {
				t.Fatalf("error = %v, want httporigin.ErrCrossOriginRedirect", err)
			}
			if hits := secondHits.Load(); hits != 0 {
				t.Fatalf("second origin received %d request(s); auth=%v", hits, secondSawAuth.Load())
			}
			if hits := providerHits.Load(); hits != 1 {
				t.Fatalf("provider received %d request(s), want 1: a rejected redirect must not be retried", hits)
			}
		})
	}
}

// The rejection error carries the request URL, whose port may contain a
// "transient" status fragment such as 503; it must still not be retried.
func TestEmbeddingRetryTreatsRejectedRedirectAsFinal(t *testing.T) {
	err := fmt.Errorf("embedding request failed: %w", &url.Error{Op: "Post", URL: "http://127.0.0.1:50312/v1/embeddings", Err: httporigin.ErrCrossOriginRedirect})
	if shouldRetryEmbeddingError(context.Background(), err) {
		t.Fatalf("rejected redirect classified retryable: %v", err)
	}
}

func TestMultimodalEmbedderFollowsSameOriginRedirect(t *testing.T) {
	var hits atomic.Int32
	server := testutil.NewHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if r.URL.Path == "/v1/embeddings" {
			w.Header().Set("Location", "/v2/embeddings")
			w.WriteHeader(http.StatusTemporaryRedirect)
			return
		}
		body, _ := io.ReadAll(r.Body)
		if len(body) == 0 || r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("same-origin hop lost body or auth: body=%d bytes auth=%q", len(body), r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"data":[{"embedding":[0.6,0.8]}]}`)
	}))
	defer server.Close()

	embedder := NewMultimodalEmbedder(server.URL+"/v1", "test-key", "embed-model", "openai", "", nil)
	vec, err := embedder.EmbedFile(context.Background(), writeMultimodalRedirectFixture(t))
	if err != nil || len(vec) != 2 {
		t.Fatalf("same-origin redirect: vec %v, err %v", vec, err)
	}
	if n := hits.Load(); n != 2 {
		t.Fatalf("server hits = %d, want 2 (redirect + followed request)", n)
	}
}
