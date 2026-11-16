package localllm

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/llm"
	"github.com/sashabaranov/go-openai"
)

type requestTestBody struct {
	io.Reader
	closed int
}

func (b *requestTestBody) Close() error { b.closed++; return nil }

func TestLocalRequestErrorsNeverFailOver(t *testing.T) {
	for name, raw := range map[string]string{"invalid": "{private-input", "null": "null", "array": "[]", "large": strings.Repeat("x", maxLocalRequestBytes+1)} {
		for _, streaming := range []bool{false, true} {
			t.Run(name+map[bool]string{false: "/normal", true: "/stream"}[streaming], func(t *testing.T) {
				cfg := &config.Config{}
				cfg.Directories.DataDir = t.TempDir()
				cfg.LLM.Model = "local-fixture"
				cfg.FallbackLLM.Enabled, cfg.FallbackLLM.Model = true, "fallback-fixture"
				cfg.FallbackLLM.ErrorThreshold = 1
				cfg.FallbackLLM.ProbeIntervalSeconds = 3600
				logger := slog.New(slog.NewTextHandler(io.Discard, nil))
				m := NewManager(cfg, nil, logger)
				defer m.Close()
				primaryCalls, fallbackCalls := 0, 0
				var bodies []*requestTestBody
				transport := m.RoundTripper(roundTripperFunc(func(*http.Request) (*http.Response, error) {
					t.Fatal("invalid request reached runtime")
					return nil, nil
				}))
				fm := llm.NewFailoverManager(cfg, logger, llm.WithClientFactory(func(slot *config.Config) *openai.Client {
					cc := openai.DefaultConfig("fixture-key")
					cc.BaseURL = "http://local.invalid/v1"
					cc.HTTPClient = &http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
						if slot.LLM.Model == "fallback-fixture" {
							fallbackCalls++
							_ = req.Body.Close()
							return nil, errors.New("unexpected fallback")
						}
						primaryCalls++
						_ = req.Body.Close()
						body := &requestTestBody{Reader: strings.NewReader(raw)}
						bodies = append(bodies, body)
						req.Body = body
						return transport.RoundTrip(req)
					})}
					return openai.NewClientWithConfig(cc)
				}))
				defer fm.Stop()
				for range 3 {
					ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
					var err error
					if streaming {
						_, err = fm.CreateChatCompletionStream(ctx, openai.ChatCompletionRequest{})
					} else {
						_, err = fm.CreateChatCompletion(ctx, openai.ChatCompletionRequest{})
					}
					cancel()
					if llm.ClassifyError(err) != llm.ErrCategoryNonRetryableConfig {
						t.Fatalf("unexpected error: %v", err)
					}
					if strings.Contains(err.Error(), "private-input") {
						t.Fatal("request contents leaked")
					}
				}
				if primaryCalls != 3 || fallbackCalls != 0 {
					t.Fatalf("primary=%d fallback=%d", primaryCalls, fallbackCalls)
				}
				if _, model := fm.ActiveProviderAndModel(); model != "local-fixture" {
					t.Fatal("route changed")
				}
				for _, body := range bodies {
					if body.closed != 1 {
						t.Fatalf("body closes=%d", body.closed)
					}
				}
			})
		}
	}
}

type cancelledRequestReader struct{}

func (cancelledRequestReader) Read([]byte) (int, error) { return 0, context.Canceled }

func TestLocalRequestReadCancellationClosesBody(t *testing.T) {
	cfg := &config.Config{}
	cfg.Directories.DataDir = t.TempDir()
	m := NewManager(cfg, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
	defer m.Close()
	body := &requestTestBody{Reader: cancelledRequestReader{}}
	req, _ := http.NewRequest(http.MethodPost, "http://local.invalid/v1/chat/completions", body)
	_, err := m.RoundTripper(nil).RoundTrip(req)
	if !errors.Is(err, context.Canceled) || body.closed != 1 {
		t.Fatalf("error=%v closes=%d", err, body.closed)
	}
}
