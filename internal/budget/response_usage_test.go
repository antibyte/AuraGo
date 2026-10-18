package budget

import (
	"aurago/internal/llm"
	"context"
	"github.com/sashabaranov/go-openai"
	"io"
	"log/slog"
	"math"
	"net/http"
	"net/http/httptest"

	"testing"
)

type usageFixtureTransport func(*http.Request) (*http.Response, error)

func (f usageFixtureTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestRecordResponseUsageBooksPhysicalResponsesWithCachePricingAndPersistsCosts(t *testing.T) {
	cfg := testConfig(100, "warn")
	cfg.LLM.ProviderType = "anthropic"
	cfg.LLM.BaseURL = "https://api.anthropic.com/v1"
	cfg.LLM.Model = "claude-fixture-metered"
	key := "anthropic/claude-fixture-metered"
	old, existed := llm.KnownModelRegistry[key]
	llm.KnownModelRegistry[key] = llm.ModelRegistryEntry{ID: cfg.LLM.Model, Provider: "anthropic", InputPricePer1M: 3, OutputPricePer1M: 15, CacheReadPricePer1M: .3, CacheWritePricePer1M: 3.75}
	defer func() {
		if existed {
			llm.KnownModelRegistry[key] = old
		} else {
			delete(llm.KnownModelRegistry, key)
		}
	}()
	dir := t.TempDir()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	tracker := NewTracker(cfg, log, dir)
	defer tracker.Flush()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"id":"msg_fixture","model":"claude-fixture-metered","content":[{"type":"text","text":"ok"}],"stop_reason":"end_turn","usage":{"input_tokens":20,"output_tokens":7,"cache_read_input_tokens":60,"cache_creation_input_tokens":20}}`)
	}))
	defer server.Close()
	cfg.LLM.BaseURL = server.URL + "/v1"
	client := llm.NewClient(cfg)
	ctx, capture := llm.CaptureUsage(context.Background())
	for range 2 {
		if _, err := client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{Model: cfg.LLM.Model, Messages: []openai.ChatCompletionMessage{{Role: "user", Content: "fixture"}}}); err != nil {
			t.Fatal(err)
		}
	}
	tracker.RecordResponseUsage("maintenance", "anthropic", cfg.LLM.Model, capture, 999, 999, true)
	want := 2 * (20*3 + 7*15 + 60*.3 + 20*3.75) / 1e6
	status := tracker.GetStatus()
	model := status.Models[cfg.LLM.Model]
	if math.Abs(status.SpentUSD-want) > 1e-12 || model.Calls != 2 || model.InputTokens != 200 || model.EstimatedCalls != 2 || math.Abs(model.CostUSD-want) > 1e-12 {
		t.Fatalf("incorrect cache accounting: %+v %+v want %f", status, model, want)
	}
	tracker.Flush()
	restored := NewTracker(cfg, log, dir)
	defer restored.Flush()
	if math.Abs(restored.GetStatus().Models[cfg.LLM.Model].CostUSD-want) > 1e-12 {
		t.Fatal("measured model cost not persisted")
	}
	restored.RecordResponseUsage("maintenance", "anthropic", cfg.LLM.Model, nil, 100, 7, true)
	if restored.GetStatus().Models[cfg.LLM.Model].EstimatedCalls != 3 {
		t.Fatal("fallback estimate not labeled")
	}
}
