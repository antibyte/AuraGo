package llm

import (
	"bytes"
	"context"
	"errors"
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

// droppingProviderURL returns a credentialed base URL for a local fixture that
// drops every connection, so client.Do fails with a *url.Error whose message
// repeats the request URL with the username and the full query.
func droppingProviderURL(t *testing.T) string {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, _, err := w.(http.Hijacker).Hijack()
		if err == nil {
			_ = conn.Close()
		}
	}))
	t.Cleanup(server.Close)
	return strings.Replace(server.URL, "http://", "http://user:s3cr3t-pw@", 1)
}

func TestQueryModelLimitsEndpointRedactsURLInsideTransportError(t *testing.T) {
	var logs syncLogBuffer
	logger := slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	endpoint := droppingProviderURL(t) + "/v1/models?key=q-s3cr3t"

	if _, ok := queryModelLimitsEndpoint(context.Background(), endpoint, "", "probe-model", logger); ok {
		t.Fatal("queryModelLimitsEndpoint() succeeded, want a transport failure")
	}
	out := logs.String()
	if !strings.Contains(out, "Provider metadata probe failed") {
		t.Fatalf("transport failure record missing; logs=%s", out)
	}
	if strings.Contains(out, "user:") || strings.Contains(out, "s3cr3t") {
		t.Fatalf("transport error leaked provider URL credentials: %s", out)
	}
}

func TestProbeOllamaModelLimitsRedactsURLInsideTransportError(t *testing.T) {
	var logs syncLogBuffer
	logger := slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	route := ModelRoute{ProviderType: "ollama", BaseURL: droppingProviderURL(t) + "/v1", Model: "probe-model"}

	if got := probeOllamaModelLimits(context.Background(), route, logger); got.ContextWindow != 0 {
		t.Fatalf("probeOllamaModelLimits() = %+v, want no limits on transport failure", got)
	}
	out := logs.String()
	if !strings.Contains(out, "Ollama metadata probe failed") {
		t.Fatalf("transport failure record missing; logs=%s", out)
	}
	if strings.Contains(out, "user:") || strings.Contains(out, "s3cr3t") {
		t.Fatalf("Ollama transport error leaked provider URL credentials: %s", out)
	}
}

func TestRedactProviderErrorKeepsOtherErrorsAndTheCause(t *testing.T) {
	if got := redactProviderError(errors.New("plain failure")); got != "plain failure" {
		t.Fatalf("redactProviderError(plain) = %q, want the error text unchanged", got)
	}
	if got := redactProviderError(nil); got != "" {
		t.Fatalf("redactProviderError(nil) = %q, want empty", got)
	}
	wrapped := &url.Error{Op: "Get", URL: "https://user:pw@host.example/v1?key=k", Err: errors.New("plain failure")}
	got := redactProviderError(wrapped)
	if strings.Contains(got, "user") || strings.Contains(got, "key=k") || !strings.Contains(got, "plain failure") || !strings.Contains(got, "https://host.example/v1") {
		t.Fatalf("redactProviderError(url.Error) = %q, want redacted URL and the cause kept", got)
	}
}
