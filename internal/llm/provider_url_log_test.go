package llm

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
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
