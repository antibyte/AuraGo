package budget

import (
	"context"
	"errors"
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
