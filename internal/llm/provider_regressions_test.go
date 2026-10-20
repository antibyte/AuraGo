package llm

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"aurago/internal/config"
	openai "github.com/sashabaranov/go-openai"
)

func TestProviderTaskCredentialsAreValidatedBeforeNetwork(t *testing.T) {
	for _, test := range []struct {
		name     string
		provider config.ProviderEntry
		blocked  bool
	}{
		{"masked_key", config.ProviderEntry{APIKey: "••••••••"}, true},
		{"masked_client_secret", config.ProviderEntry{OAuthClientSecret: " •••••••• "}, true},
		{"masked_oauth_token", config.ProviderEntry{AuthType: "oauth2", RuntimeOAuthToken: "••••••••"}, true},
		{"missing_oauth_token", config.ProviderEntry{AuthType: "oauth2"}, true},
		{"keyless_local", config.ProviderEntry{Type: "ollama"}, false},
		{"oauth_token", config.ProviderEntry{AuthType: "oauth2", RuntimeOAuthToken: "syntheticTaskToken123"}, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			var calls atomic.Int32
			fixture := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				calls.Add(1)
				w.Header().Set("Content-Type", "application/json")
				_, _ = io.WriteString(w, `{"choices":[{"message":{"role":"assistant","content":"OK"},"finish_reason":"stop"}]}`)
			}))
			defer fixture.Close()
			p := test.provider
			p.ID, p.Model, p.BaseURL = "fixture-task", "fixture", fixture.URL+"/v1"
			if p.Type == "" {
				p.Type = "openai"
			}
			cfg := &config.Config{Providers: []config.ProviderEntry{p}}
			view := TaskProviderConfig(cfg, p)
			_, err := NewClient(view).CreateChatCompletion(context.Background(), openai.ChatCompletionRequest{Model: p.Model})
			if test.blocked {
				if err == nil || calls.Load() != 0 {
					t.Fatal("invalid task credentials were not blocked before transport")
				}
			} else if err != nil || calls.Load() != 1 {
				t.Fatal("valid task route did not make exactly one request")
			}
		})
	}
}

type cancelledAttemptClient struct {
	cancel  context.CancelFunc
	failure error
	stream  *cancelledAttemptStream
}

func (c *cancelledAttemptClient) CreateChatCompletion(context.Context, openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
	c.cancel()
	return openai.ChatCompletionResponse{}, c.failure
}

func (c *cancelledAttemptClient) CreateChatCompletionStream(context.Context, openai.ChatCompletionRequest) (CompletionStream, error) {
	c.cancel()
	return c.stream, c.failure
}

type cancelledAttemptStream struct{ closed atomic.Bool }

func (s *cancelledAttemptStream) Close() error { s.closed.Store(true); return nil }
func (s *cancelledAttemptStream) Recv() (openai.ChatCompletionStreamResponse, error) {
	return openai.ChatCompletionStreamResponse{}, io.EOF
}

type cancellationHealthReporter struct{ errors, successes atomic.Int32 }

func (r *cancellationHealthReporter) ReportLLMHealthEvent(HealthEvent)     { r.errors.Add(1) }
func (r *cancellationHealthReporter) ReportLLMHealthSuccess(HealthSuccess) { r.successes.Add(1) }

func TestProviderCallerCancellationOverridesConcurrentTransportResult(t *testing.T) {
	for name, failure := range map[string]error{
		"timeout":      &TransportTimeoutError{Stage: "fixture", After: time.Second},
		"other_error":  errors.New("connection reset"),
		"late_success": nil,
	} {
		for _, streaming := range []bool{false, true} {
			mode := "chat"
			if streaming {
				mode = "stream"
			}
			t.Run(name+"/"+mode, func(t *testing.T) {
				healthReporterState.mu.RLock()
				previous := healthReporterState.reporter
				healthReporterState.mu.RUnlock()
				collector := &cancellationHealthReporter{}
				SetHealthReporter(collector)
				t.Cleanup(func() { SetHealthReporter(previous) })
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				client := &cancelledAttemptClient{cancel: cancel, failure: failure, stream: &cancelledAttemptStream{}}
				var err error
				if streaming {
					_, _, err = ExecuteStreamWithRetry(ctx, client, openai.ChatCompletionRequest{}, nil, nil)
					if !client.stream.closed.Load() {
						t.Error("cancelled opening leaked its stream")
					}
				} else {
					_, err = ExecuteWithRetry(ctx, client, openai.ChatCompletionRequest{}, nil, nil)
				}
				if !errors.Is(err, context.Canceled) {
					t.Errorf("caller cancellation was not returned: %v", err)
				}
				if collector.errors.Load() != 0 || collector.successes.Load() != 0 {
					t.Fatal("cancelled caller changed provider health")
				}
			})
		}
	}
}
