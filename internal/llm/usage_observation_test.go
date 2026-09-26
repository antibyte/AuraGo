package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sashabaranov/go-openai"
)

func TestProviderUsageObservation(t *testing.T) {
	for _, provider := range []string{"stepfun", "openai", "anthropic", "openrouter"} {
		for _, stream := range []bool{false, true} {
			for _, reported := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/stream=%t/reported=%t", provider, stream, reported), func(t *testing.T) {
					model := "fixture"
					if provider == "openrouter" {
						model = "stepfun/step-5-preview"
					}
					server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						usage := ""
						if reported {
							switch provider {
							case "stepfun", "openrouter":
								usage = `,"usage":{"prompt_tokens":100,"completion_tokens":7,"total_tokens":107,"cached_tokens":60}`
							case "openai":
								usage = `,"usage":{"prompt_tokens":100,"completion_tokens":7,"total_tokens":107,"prompt_tokens_details":{"cached_tokens":60}}`
							case "anthropic":
								usage = `,"usage":{"input_tokens":20,"output_tokens":7,"cache_read_input_tokens":60,"cache_creation_input_tokens":20}`
							}
						}
						if stream {
							w.Header().Set("Content-Type", "text/event-stream")
							if provider == "anthropic" {
								fmt.Fprintf(w, "event: message_start\ndata: {\"message\":{\"id\":\"msg_fixture\",\"model\":%q%s}}\n\n", model, usage)
								for range 2 {
									fmt.Fprintf(w, "event: message_delta\ndata: {\"delta\":{\"stop_reason\":\"end_turn\"}%s}\n\n", usage)
								}
								fmt.Fprint(w, "event: message_stop\ndata: {}\n\n")
							} else {
								for range 2 {
									fmt.Fprintf(w, "data: {\"model\":%q,\"choices\":[{\"index\":0,\"delta\":{\"content\":\"ok\"},\"finish_reason\":\"stop\"}]%s}\n\n", model, usage)
								}
								fmt.Fprint(w, "data: [DONE]\n\n")
							}
						} else {
							w.Header().Set("Content-Type", "application/json")
							if provider == "anthropic" {
								fmt.Fprintf(w, `{"id":"msg_fixture","model":%q,"content":[{"type":"text","text":"ok"}],"stop_reason":"end_turn"%s}`, model, usage)
							} else {
								fmt.Fprintf(w, `{"model":%q,"choices":[{"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}]%s}`, model, usage)
							}
						}
					}))
					defer server.Close()
					client := NewClientFromProvider(provider, server.URL+"/v1", "fixture")
					ctx, capture := CaptureUsage(context.Background())
					req := openai.ChatCompletionRequest{Model: model, Messages: []openai.ChatCompletionMessage{{Role: "user", Content: "fixture"}}}
					var sdk openai.Usage
					if stream {
						response, err := client.CreateChatCompletionStream(ctx, req)
						if err != nil {
							t.Fatal(err)
						}
						for {
							chunk, err := response.Recv()
							if err == io.EOF {
								break
							}
							if err != nil {
								t.Fatal(err)
							}
							if chunk.Usage != nil {
								sdk = *chunk.Usage
							}
						}
						_ = response.Close()
					} else {
						response, err := client.CreateChatCompletion(ctx, req)
						if err != nil {
							t.Fatal(err)
						}
						sdk = response.Usage
					}
					items := capture.Snapshot()
					if len(items) != 1 {
						t.Fatalf("wanted one transport attempt, got %d", len(items))
					}
					got := items[0]
					if got.Provider != provider || got.Model != model {
						t.Fatalf("route lost: %+v", got)
					}
					if reported {
						if got.InputTokens == nil || *got.InputTokens != 100 || got.OutputTokens == nil || *got.OutputTokens != 7 || got.CacheReadTokens == nil || *got.CacheReadTokens != 60 {
							t.Fatalf("usage lost/doubled: %+v", got)
						}
						if sdk.PromptTokens != 100 || sdk.PromptTokensDetails == nil || sdk.PromptTokensDetails.CachedTokens != 60 {
							t.Fatalf("SDK normalization: %+v", sdk)
						}
						if provider == "anthropic" && (got.CacheWriteTokens == nil || *got.CacheWriteTokens != 20) {
							t.Fatal("cache write missing")
						}
					} else if got.InputTokens != nil || got.OutputTokens != nil || got.CacheReadTokens != nil || got.CacheWriteTokens != nil {
						t.Fatal("missing cache usage became zero")
					}
					if got.EstimatedCostUSD != nil {
						t.Fatal("custom endpoint got invented pricing")
					}
				})
			}
		}
	}
}

func TestUsageUnknownZeroAndCumulative(t *testing.T) {
	var got RequestUsage
	for _, data := range []string{
		`{"usage":{"prompt_tokens":100,"completion_tokens":1,"prompt_tokens_details":{"audio_tokens":5}}}`,
		`{"usage":{"completion_tokens":7,"prompt_tokens_details":{"cached_tokens":0}}}`,
		`{"usage":{"completion_tokens":7,"prompt_tokens_details":{"cached_tokens":0}}}`,
	} {
		normalizeObservedUsage([]byte(data), false, &got)
	}
	if *got.InputTokens != 100 || *got.OutputTokens != 7 || got.CacheReadTokens == nil || *got.CacheReadTokens != 0 || got.CacheWriteTokens != nil {
		t.Fatalf("invalid aggregation: %+v", got)
	}
	var unknown RequestUsage
	normalizeObservedUsage([]byte(`{"usage":{"prompt_tokens_details":{}}}`), false, &unknown)
	if unknown.InputTokens != nil || unknown.OutputTokens != nil || unknown.CacheReadTokens != nil {
		t.Fatal("unknown is not zero")
	}
	if knownMeteredUsageRoute("stepfun", "https://api.stepfun.ai/step_plan/v1") || knownMeteredUsageRoute("openai", "https://proxy.invalid/v1") {
		t.Fatal("subscription/custom route priced")
	}
	if estimateMeasuredCost(RequestUsage{Provider: "stepfun", Model: "unknown"}) != nil {
		t.Fatal("unknown price estimated")
	}
}

func TestPromptCacheKeyIgnoresAppendedFeedback(t *testing.T) {
	payload := map[string]any{"model": "fixture", "messages": []any{map[string]any{"role": "system", "content": "Stable rules"}, map[string]any{"role": "user", "content": "Job one"}}}
	before := buildOpenAIPromptCacheKey(payload)
	payload["messages"] = append(payload["messages"].([]any), map[string]any{"role": "system", "content": "Retry number two"})
	if buildOpenAIPromptCacheKey(payload) != before {
		t.Fatal("appended feedback changed prefix routing key")
	}
	payload["messages"].([]any)[0].(map[string]any)["content"] = "Changed rules"
	if buildOpenAIPromptCacheKey(payload) == before {
		t.Fatal("instruction change did not change routing key")
	}
}

func TestUsagePartialAnthropicAndMeteredPrice(t *testing.T) {
	var ant anthropicUsage
	if err := json.Unmarshal([]byte(`{"cache_read_input_tokens":60,"cache_creation_input_tokens":20,"output_tokens":7}`), &ant); err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(map[string]any{"usage": ant.openAI()})
	var got RequestUsage
	normalizeObservedUsage(data, false, &got)
	if got.InputTokens != nil || got.CacheReadTokens == nil || *got.CacheReadTokens != 60 || got.CacheWriteTokens == nil || *got.CacheWriteTokens != 20 || got.OutputTokens == nil || *got.OutputTokens != 7 {
		t.Fatal("partial Anthropic measurements invented an input total")
	}
	info, ok := GetModelInfo("openai", "gpt-4o")
	if !ok || info.InputPricePer1M <= 0 || info.CacheReadPricePer1M <= 0 {
		t.Fatal("missing metered registry fixture")
	}
	in, out, read := 100, 7, 60
	cost := estimateMeasuredCost(RequestUsage{Provider: "openai", Model: "gpt-4o", InputTokens: &in, OutputTokens: &out, CacheReadTokens: &read})
	want := (40*info.InputPricePer1M + 60*info.CacheReadPricePer1M + 7*info.OutputPricePer1M) / 1e6
	if cost == nil || math.Abs(*cost-want) > 1e-12 {
		t.Fatal("cache reads double billed at normal input price")
	}
	if estimateMeasuredCost(RequestUsage{Provider: "openai", Model: "gpt-4o", ServiceTier: "priority", InputTokens: &in, OutputTokens: &out, CacheReadTokens: &read}) != nil {
		t.Fatal("unknown priority tariff priced at the standard rate")
	}
}

func TestUsageCapturesSeparateTransportAttempts(t *testing.T) {
	attempts := 0
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		w.Header().Set("Content-Type", "application/json")
		if attempts == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			fmt.Fprint(w, `{"error":{"message":"fixture retry","type":"server_error"}}`)
			return
		}
		fmt.Fprint(w, `{"model":"fixture","choices":[{"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":100,"completion_tokens":7}}`)
	}))
	client := NewClientFromProvider("openai", provider.URL+"/v1", "fixture")
	ctx, capture := CaptureUsage(context.Background())
	req := openai.ChatCompletionRequest{Model: "fixture", Messages: []openai.ChatCompletionMessage{{Role: "user", Content: "fixture"}}}
	_, err := client.CreateChatCompletion(ctx, req)
	if err == nil {
		t.Fatal("first attempt must fail")
	}
	if _, err := client.CreateChatCompletion(ctx, req); err != nil {
		t.Fatal(err)
	}
	provider.Close()
	if _, err := client.CreateChatCompletion(ctx, req); err == nil {
		t.Fatal("closed fixture must fail transport")
	}
	items := capture.Snapshot()
	if len(items) != 3 || items[0].Status != 503 || items[0].InputTokens != nil || items[1].Status != 200 || items[1].InputTokens == nil || *items[1].InputTokens != 100 || !items[2].TransportError {
		t.Fatalf("attempts lost or double counted: %+v", items)
	}
}

func TestUsageRequestsOpenAIStreamCountersOnlyForObservedOfficialRoute(t *testing.T) {
	for _, tc := range []struct{ observed, official, stream bool }{
		{true, true, true}, {false, true, true}, {true, false, true}, {true, true, false},
	} {
		provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var payload struct {
				Options struct {
					IncludeUsage bool `json:"include_usage"`
				} `json:"stream_options"`
			}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Error(err)
			}
			if payload.Options.IncludeUsage != (tc.observed && tc.official && tc.stream) {
				t.Errorf("wrong stream usage opt-in: %+v", tc)
			}
			fmt.Fprint(w, `{}`)
		}))
		ctx := context.Background()
		if tc.observed {
			ctx, _ = CaptureUsage(ctx)
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, provider.URL+"/v1/chat/completions", strings.NewReader(fmt.Sprintf(`{"model":"fixture","stream":%t}`, tc.stream)))
		if err != nil {
			t.Fatal(err)
		}
		transport := &usageObservationTransport{base: http.DefaultTransport, provider: "openai", pricedRoute: tc.official}
		resp, err := transport.RoundTrip(req)
		if err != nil {
			t.Fatal(err)
		}
		_ = resp.Body.Close()
		provider.Close()
	}
}
