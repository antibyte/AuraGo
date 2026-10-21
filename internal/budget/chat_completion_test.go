package budget

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"aurago/internal/config"
	"aurago/internal/llm"
	"github.com/sashabaranov/go-openai"
)

type auxiliaryBudgetClient struct {
	calls    int
	response openai.ChatCompletionResponse
	err      error
	request  openai.ChatCompletionRequest
}

type retryFinalBudgetClient struct{ llm.ChatClient }

func (c retryFinalBudgetClient) CreateChatCompletion(ctx context.Context, request openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
	_, _ = c.ChatClient.CreateChatCompletion(ctx, request)
	return c.ChatClient.CreateChatCompletion(ctx, request)
}

func TestCompleteChatSupplementsOnlyFinalPhysicalResponse(t *testing.T) {
	for _, test := range []struct {
		name, usage, content string
		status               int
		calls, input, output int
		estimated            int
	}{
		{"missing_output", `,"usage":{"prompt_tokens":10}`, "OK", 200, 1, 10, 2, 1},
		{"missing_input", `,"usage":{"completion_tokens":3}`, "OK", 200, 1, 14, 3, 1},
		{"missing_all", "", "OK", 200, 1, 14, 2, 1},
		{"initial_zero_output", `,"usage":{"prompt_tokens":10,"completion_tokens":0,"total_tokens":10}`, "OK", 200, 1, 10, 2, 1},
		{"zero_output_without_text", `,"usage":{"prompt_tokens":10,"completion_tokens":0,"total_tokens":10}`, "", 200, 1, 10, 0, 1},
		{"complete_usage", `,"usage":{"prompt_tokens":10,"completion_tokens":3}`, "OK", 200, 1, 10, 3, 1},
		{"evidence_free_failure", "", "", 500, 0, 0, 0, 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			requests := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests++
				w.Header().Set("Content-Type", "application/json")
				if requests == 1 {
					w.WriteHeader(http.StatusBadRequest)
					fmt.Fprint(w, `{"model":"earlier-fixture","usage":{"prompt_tokens":7,"completion_tokens":2},"error":{"message":"fixture retry"}}`)
					return
				}
				w.WriteHeader(test.status)
				if test.status >= 400 {
					fmt.Fprint(w, `{"error":{"message":"fixture final failure"}}`)
					return
				}
				// The provider omits model; capture and ledger must retain the
				// effective request route instead of borrowing the earlier model.
				fmt.Fprintf(w, `{"choices":[{"finish_reason":"stop","message":{"role":"assistant","content":%q}}]%s}`, test.content, test.usage)
			}))
			defer server.Close()
			cfg := testConfig(100, "warn")
			cfg.LLM.ProviderType, cfg.LLM.BaseURL = "openai", server.URL+"/v1"
			cfg.Budget.DefaultCost = config.ModelCostRates{InputPerMillion: 1, OutputPerMillion: 1}
			tracker := NewTracker(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), t.TempDir())
			defer tracker.Flush()
			client := retryFinalBudgetClient{llm.WrapOpenAIClient(llm.NewClient(cfg))}
			_, err := CompleteChat(WithTracking(context.Background(), tracker, "helper"), client, openai.ChatCompletionRequest{Model: "final-fixture", Messages: []openai.ChatCompletionMessage{{Role: "user", Content: "fixture"}}}, llm.ModelRoute{ProviderType: "openai", Model: "final-fixture"}, "helper", 128)
			if (err != nil) != (test.status >= 400) || requests != 2 {
				t.Fatalf("err=%v physical requests=%d", err, requests)
			}
			status := tracker.GetStatus()
			earlier, final := status.Models["earlier-fixture"], status.Models["final-fixture"]
			if earlier.Calls != 1 || earlier.InputTokens != 7 || earlier.OutputTokens != 2 {
				t.Fatalf("earlier retry measurements changed: %+v", earlier)
			}
			if final.Calls != test.calls || final.InputTokens != test.input || final.OutputTokens != test.output || final.EstimatedCalls != test.estimated {
				t.Fatalf("final accounting=%+v, want calls=%d input=%d output=%d estimated=%d", final, test.calls, test.input, test.output, test.estimated)
			}
			wantCost := float64(9+test.input+test.output) / 1e6
			if status.SpentUSD < wantCost-1e-12 || status.SpentUSD > wantCost+1e-12 {
				t.Fatalf("cost=%g want %g", status.SpentUSD, wantCost)
			}
		})
	}
}

type partialBodyBudgetClient struct {
	llm.ChatClient
	observedOutput bool
}

func (c partialBodyBudgetClient) CreateChatCompletion(ctx context.Context, request openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
	response, err := c.ChatClient.CreateChatCompletion(ctx, request)
	if c.observedOutput {
		// Streaming application adapters may retain observed output even when
		// their underlying provider reports a terminal body failure.
		response.Choices = []openai.ChatCompletionChoice{{Message: openai.ChatCompletionMessage{Content: "OK"}}}
	}
	return response, err
}

func TestCompleteChatBooksObservedOutputAfter2xxBodyFailure(t *testing.T) {
	for _, observed := range []bool{false, true} {
		t.Run(fmt.Sprintf("output=%t", observed), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Content-Length", "1000")
				_, _ = io.WriteString(w, `{"choices":[`)
			}))
			defer server.Close()
			cfg := testConfig(100, "warn")
			cfg.LLM.ProviderType, cfg.LLM.BaseURL = "openai", server.URL+"/v1"
			tracker := NewTracker(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), t.TempDir())
			defer tracker.Flush()
			client := partialBodyBudgetClient{llm.WrapOpenAIClient(llm.NewClient(cfg)), observed}
			_, err := CompleteChat(WithTracking(context.Background(), tracker, "helper"), client, openai.ChatCompletionRequest{Model: "body-failure-fixture", Messages: []openai.ChatCompletionMessage{{Role: "user", Content: "fixture"}}}, llm.ModelRoute{ProviderType: "openai", Model: "body-failure-fixture"}, "helper", 128)
			if err == nil {
				t.Fatal("fixture did not expose terminal body failure")
			}
			usage := tracker.GetStatus().Models["body-failure-fixture"]
			if observed {
				if usage.Calls != 1 || usage.InputTokens != 14 || usage.OutputTokens != 2 || usage.EstimatedCalls != 1 {
					t.Fatalf("observed partial output lost: %+v", usage)
				}
			} else if usage.Calls != 0 {
				t.Fatalf("evidence-free transport failure billed: %+v", usage)
			}
		})
	}
}

func TestEstimateResponseUsageProbeWithoutProviderUsage(t *testing.T) {
	request := openai.ChatCompletionRequest{Model: "probe-fixture", MaxTokens: 1, Messages: []openai.ChatCompletionMessage{{Role: "user", Content: "ok"}}}
	response := openai.ChatCompletionResponse{Choices: []openai.ChatCompletionChoice{{Message: openai.ChatCompletionMessage{Content: "OK"}}}}
	count := TokenCounter(func(_ context.Context, text, _ string) int { return len(text) })
	input, output, observed := EstimateResponseUsage(context.Background(), request, response, count)
	if !observed || input != 9 || output != 2 {
		t.Fatalf("probe estimate=%d/%d observed=%t", input, output, observed)
	}
	if input, output, observed = EstimateResponseUsage(context.Background(), request, openai.ChatCompletionResponse{}, count); observed || input != 0 || output != 0 {
		t.Fatalf("evidence-free response estimated=%d/%d observed=%t", input, output, observed)
	}
}

func (c *auxiliaryBudgetClient) CreateChatCompletion(_ context.Context, request openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
	c.calls++
	c.request = request
	return c.response, c.err
}
func (c *auxiliaryBudgetClient) CreateChatCompletionStream(context.Context, openai.ChatCompletionRequest) (llm.CompletionStream, error) {
	return nil, errors.New("unexpected stream")
}

func TestCompleteChatBlocksBeforeProviderAndBooksConsumedErrorsOnce(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := testConfig(1, "full")
	cfg.Budget.DefaultCost = config.ModelCostRates{InputPerMillion: 1, OutputPerMillion: 1}
	tracker := NewTracker(cfg, logger, t.TempDir())
	defer tracker.Flush()
	client := &auxiliaryBudgetClient{}
	request := openai.ChatCompletionRequest{Model: "aux-fixture", Messages: []openai.ChatCompletionMessage{{Role: "user", Content: "test"}}}
	ctx := WithTracking(context.Background(), tracker, "helper")
	tracker.RecordCost(2)
	if _, err := CompleteChat(ctx, client, request, llm.ModelRoute{}, "helper", 128); !errors.Is(err, ErrCompletionBudgetBlocked) || client.calls != 0 {
		t.Fatalf("blocked calls=%d err=%v", client.calls, err)
	}
	cfg.Budget.DailyLimitUSD = 100
	tracker.UpdateConfig(cfg)
	client.response = openai.ChatCompletionResponse{Model: "aux-fixture", Usage: openai.Usage{PromptTokens: 11, CompletionTokens: 7, TotalTokens: 18}}
	client.err = errors.New("injected provider failure after consumption")
	if _, err := CompleteChat(ctx, client, request, llm.ModelRoute{}, "helper", 128); err == nil {
		t.Fatal("provider failure lost")
	}
	usage := tracker.GetStatus().Models["aux-fixture"]
	if usage.Calls != 1 || usage.InputTokens != 11 || usage.OutputTokens != 7 {
		t.Fatalf("consumed error accounting=%+v", usage)
	}
	if max(client.request.MaxTokens, client.request.MaxCompletionTokens) != 128 {
		t.Fatalf("missing finite output limit: %+v", client.request)
	}
	client.response = openai.ChatCompletionResponse{}
	CompleteChat(ctx, client, request, llm.ModelRoute{}, "helper", 128)
	if got := tracker.GetStatus().Models["aux-fixture"].Calls; got != 1 {
		t.Fatalf("evidence-free transport error billed: %d", got)
	}
}

type auxiliaryAttemptClient struct{ llm.ChatClient }

func (c auxiliaryAttemptClient) CreateChatCompletion(ctx context.Context, request openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
	if _, err := c.ChatClient.CreateChatCompletion(ctx, request); err != nil {
		return openai.ChatCompletionResponse{}, err
	}
	return c.ChatClient.CreateChatCompletion(ctx, request)
}

func TestCompleteChatBooksPhysicalAttemptsAndIsolatesParentCapture(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"model":"aux-physical","choices":[{"finish_reason":"stop","message":{"role":"assistant","content":"OK"}}],"usage":{"prompt_tokens":10,"completion_tokens":3,"total_tokens":13}}`)
	}))
	defer server.Close()
	cfg := testConfig(100, "warn")
	cfg.LLM.ProviderType = "openai"
	cfg.LLM.APIKey = "test"
	cfg.LLM.BaseURL = server.URL + "/v1"
	cfg.Budget.DefaultCost = config.ModelCostRates{InputPerMillion: 1, OutputPerMillion: 1}
	tracker := NewTracker(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), t.TempDir())
	defer tracker.Flush()
	ctx, parent := llm.CaptureUsage(WithTracking(context.Background(), tracker, "helper"))
	client := auxiliaryAttemptClient{llm.WrapOpenAIClient(llm.NewClient(cfg))}
	_, err := CompleteChat(ctx, client, openai.ChatCompletionRequest{Model: "aux-physical", Messages: []openai.ChatCompletionMessage{{Role: "user", Content: "fixture"}}}, llm.ModelRoute{ProviderType: "openai", Model: "aux-physical"}, "helper", 128)
	if err != nil {
		t.Fatal(err)
	}
	usage := tracker.GetStatus().Models["aux-physical"]
	if usage.Calls != 2 || usage.InputTokens != 20 || usage.OutputTokens != 6 {
		t.Fatalf("physical attempts=%+v", usage)
	}
	if len(parent.Snapshot()) != 0 {
		t.Fatal("auxiliary usage leaked into parent capture and could be booked twice")
	}
}
