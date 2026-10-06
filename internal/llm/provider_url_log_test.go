package llm

import (
	"bytes"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	"aurago/internal/config"

	"github.com/sashabaranov/go-openai"
)

type syncLogBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncLogBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncLogBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func TestFailoverReconfigureLogsRedactedBaseURL(t *testing.T) {
	var logs syncLogBuffer
	logger := slog.New(slog.NewTextHandler(&logs, nil))
	factory := WithClientFactory(func(*config.Config) *openai.Client { return openai.NewClient("") })

	initial := &config.Config{}
	initial.LLM.ProviderType = "openai"
	initial.LLM.Model = "initial-model"
	initial.LLM.BaseURL = "http://127.0.0.1:1/v1"
	fm := NewFailoverManager(initial, logger, factory)
	defer fm.Stop()

	next := &config.Config{}
	next.LLM.ProviderType = "openai"
	next.LLM.Model = "next-model"
	next.LLM.BaseURL = "https://user:s3cr3t-pw@host.example/v1?key=q-s3cr3t#frag-s3cr3t"
	fm.Reconfigure(next)

	got := logs.String()
	if !strings.Contains(got, "FailoverManager reconfigured") {
		t.Fatalf("reconfigure record missing; logs=%s", got)
	}
	if strings.Contains(got, "s3cr3t") || strings.Contains(got, "user:") {
		t.Fatalf("reconfigure log leaked provider URL credentials: %s", got)
	}
	if !strings.Contains(got, "base_url=https://host.example/v1") {
		t.Fatalf("reconfigure log does not carry the redacted base URL: %s", got)
	}
}

func TestQueryModelsEndpointLogsRedactedURL(t *testing.T) {
	var logs syncLogBuffer
	logger := slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"data":[{"id":"probe-model","context_length":32768}]}`)),
		}, nil
	})}

	got := queryModelsEndpoint(client, "https://user:s3cr3t-pw@host.example/v1/models?key=q-s3cr3t", "", "probe-model", logger)
	if got != 32768 {
		t.Fatalf("queryModelsEndpoint() = %d, want 32768", got)
	}
	out := logs.String()
	if !strings.Contains(out, "Detected model context window") {
		t.Fatalf("detection record missing; logs=%s", out)
	}
	if strings.Contains(out, "s3cr3t") {
		t.Fatalf("context detection log leaked provider URL credentials: %s", out)
	}
	if !strings.Contains(out, "url=https://host.example/v1/models") {
		t.Fatalf("context detection log does not carry the redacted URL: %s", out)
	}
}

// http.Client wraps transport failures in *url.Error, whose message repeats the
// request URL with the username and the full query (only the password masked).
func TestQueryModelsEndpointRedactsURLInsideTransportError(t *testing.T) {
	var logs syncLogBuffer
	logger := slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return nil, errors.New("connection refused")
	})}

	if got := queryModelsEndpoint(client, "https://user:s3cr3t-pw@host.example/v1/models?key=q-s3cr3t", "", "probe-model", logger); got != 0 {
		t.Fatalf("queryModelsEndpoint() = %d, want 0 on transport failure", got)
	}
	out := logs.String()
	if !strings.Contains(out, "Failed to query models API") || !strings.Contains(out, "connection refused") {
		t.Fatalf("transport failure record missing; logs=%s", out)
	}
	if strings.Contains(out, "user:") || strings.Contains(out, "s3cr3t") {
		t.Fatalf("transport error leaked provider URL credentials: %s", out)
	}
}

func TestDetectContextWindowOllamaRedactsURLInsideTransportError(t *testing.T) {
	var logs syncLogBuffer
	logger := slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	// Local fixture that drops every connection, so client.Do fails with *url.Error.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, _, err := w.(http.Hijacker).Hijack()
		if err == nil {
			_ = conn.Close()
		}
	}))
	t.Cleanup(server.Close)
	baseURL := strings.Replace(server.URL, "http://", "http://user:s3cr3t-pw@", 1) + "/v1"

	if got := detectContextWindowOllama(baseURL, "probe-model", logger); got != 0 {
		t.Fatalf("detectContextWindowOllama() = %d, want 0 on transport failure", got)
	}
	out := logs.String()
	if !strings.Contains(out, "Failed to query /api/show") {
		t.Fatalf("transport failure record missing; logs=%s", out)
	}
	if strings.Contains(out, "user:") || strings.Contains(out, "s3cr3t") {
		t.Fatalf("Ollama transport error leaked provider URL credentials: %s", out)
	}
}

func TestRedactProviderErrKeepsOtherErrorsUnchanged(t *testing.T) {
	plain := errors.New("plain failure")
	if got := redactProviderErr(plain); got != plain {
		t.Fatalf("redactProviderErr(plain) = %v, want the same error", got)
	}
	if redactProviderErr(nil) != nil {
		t.Fatal("redactProviderErr(nil) != nil")
	}
	wrapped := &url.Error{Op: "Get", URL: "https://user:pw@host.example/v1?key=k", Err: plain}
	got := redactProviderErr(wrapped)
	if strings.Contains(got.Error(), "user") || strings.Contains(got.Error(), "key=k") || !errors.Is(got, plain) {
		t.Fatalf("redactProviderErr(url.Error) = %v, want redacted URL and the cause kept", got)
	}
}
