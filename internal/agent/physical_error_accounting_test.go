package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"aurago/internal/budget"
	"aurago/internal/config"
	"aurago/internal/llm"
	openai "github.com/sashabaranov/go-openai"
)

// This local HTTP fixture exercises the real SDK/usage-observation boundary:
// error responses carry physical usage while the SDK returns an empty response.
func physicalErrorFixture(t *testing.T, cfg *config.Config, reported bool, toolRounds int, success bool) (*openai.Client, *atomic.Int32) {
	t.Helper()
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := int(calls.Add(1))
		w.Header().Set("Content-Type", "application/json")
		if n <= toolRounds {
			fmt.Fprintf(w, `{"model":"gpt-4o-mini","usage":{"prompt_tokens":5,"completion_tokens":2,"total_tokens":7},"choices":[{"index":0,"finish_reason":"tool_calls","message":{"role":"assistant","tool_calls":[{"id":"fixture-%d","type":"function","function":{"name":"list_skills","arguments":"{}"}}]}}]}`, n)
			return
		}
		if success {
			fmt.Fprint(w, `{"model":"gpt-4o-mini","usage":{"prompt_tokens":17,"completion_tokens":3,"total_tokens":20},"choices":[{"index":0,"finish_reason":"stop","message":{"role":"assistant","content":"{\"description\":\"fixture\"}"}}]}`)
			return
		}
		body := map[string]interface{}{"model": "gpt-4o-mini", "error": map[string]string{"message": "fixture request rejected", "type": "invalid_request_error", "code": "fixture_error"}}
		if reported {
			body["usage"] = map[string]int{"prompt_tokens": 17, "completion_tokens": 3, "total_tokens": 20}
		}
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(body)
	}))
	t.Cleanup(server.Close)
	cfg.LLM.ProviderType, cfg.LLM.BaseURL, cfg.LLM.Model = "openai", server.URL+"/v1", "gpt-4o-mini"
	cfg.Agent.ContextWindow = 32768
	return llm.NewClient(cfg), &calls
}

func assertPhysicalErrorBooking(t *testing.T, tracker *budget.Tracker, wantCalls, input, output int) {
	t.Helper()
	usage := tracker.GetStatus().Models["gpt-4o-mini"]
	if usage.Calls != wantCalls || usage.InputTokens != input || usage.OutputTokens != output {
		t.Fatalf("physical response accounting = %+v, want calls=%d input=%d output=%d", usage, wantCalls, input, output)
	}
}

func TestPersonalityBooksPhysicalNon2xxUsageWithoutSDKUsage(t *testing.T) {
	for _, mode := range []string{"reported error", "unreported error", "success once"} {
		t.Run(mode, func(t *testing.T) {
			cfg := &config.Config{}
			client, calls := physicalErrorFixture(t, cfg, mode != "unreported error", 0, mode == "success once")
			tracker := newUsageRegressionTracker(t, cfg)
			adapter := &emotionCompletionClient{client: client, cfg: cfg, tracker: tracker}
			response, err := adapter.CreateChatCompletion(context.Background(), openai.ChatCompletionRequest{Model: cfg.LLM.Model, Messages: []openai.ChatCompletionMessage{{Role: "user", Content: "Return fixture emotion JSON."}}})
			if (err == nil) != (mode == "success once") || calls.Load() != 1 {
				t.Fatalf("error=%v HTTP calls=%d", err, calls.Load())
			}
			if mode != "success once" && (response.Usage.TotalTokens != 0 || len(response.Choices) != 0) {
				t.Fatalf("fixture must leave the SDK response empty: %+v", response)
			}
			if mode == "unreported error" {
				assertPhysicalErrorBooking(t, tracker, 0, 0, 0)
			} else {
				assertPhysicalErrorBooking(t, tracker, 1, 17, 3)
			}
		})
	}
}

func TestMinimalLoopBooksPhysicalNon2xxUsageWithoutAggregate(t *testing.T) {
	for _, stream := range []bool{false, true} {
		for _, reported := range []bool{false, true} {
			t.Run(fmt.Sprintf("stream=%t/reported=%t", stream, reported), func(t *testing.T) {
				cfg := &config.Config{}
				client, calls := physicalErrorFixture(t, cfg, reported, 0, false)
				tracker := newUsageRegressionTracker(t, cfg)
				result, _, err := ExecuteMinimalLoop(context.Background(), llm.WrapOpenAIClient(client), cfg.LLM.Model, "Fixture system.", "Fixture task.", nil, &DispatchContext{Cfg: cfg, BudgetTracker: tracker}, nil, nil, &MinimalLoopOptions{MaxToolRounds: 0, StreamText: stream, BudgetCategory: "fixture"})
				if err == nil || calls.Load() != 1 || result.TotalTokens != 0 || result.Response != "" {
					t.Fatalf("error=%v HTTP calls=%d result=%+v", err, calls.Load(), result)
				}
				if reported {
					assertPhysicalErrorBooking(t, tracker, 1, 17, 3)
				} else {
					assertPhysicalErrorBooking(t, tracker, 0, 0, 0)
				}
			})
		}
	}
}

func TestMinimalLoopBooksPhysicalNon2xxSummaryWithoutDoubleChargingRounds(t *testing.T) {
	cfg := &config.Config{}
	client, calls := physicalErrorFixture(t, cfg, true, 2, false)
	tracker := newUsageRegressionTracker(t, cfg)
	dispatch := &DispatchContext{Cfg: cfg, BudgetTracker: tracker, ToolScopeRestricted: true, AllowedTools: map[string]struct{}{"list_skills": {}}, ExecutionHooks: &ExecutionHooks{HandleTool: func(context.Context, ToolCall) (string, bool) { return `{"status":"success"}`, true }}}
	result, _, err := ExecuteMinimalLoop(context.Background(), llm.WrapOpenAIClient(client), cfg.LLM.Model, "Fixture system.", "Fixture task.", []openai.Tool{testToolSchema("list_skills", "Fixture")}, dispatch, nil, nil, &MinimalLoopOptions{MaxToolRounds: 1, BudgetCategory: "fixture"})
	if err == nil || calls.Load() != 3 || result.ToolCalls != 2 || result.TotalTokens != 14 {
		t.Fatalf("error=%v HTTP calls=%d result=%+v", err, calls.Load(), result)
	}
	assertPhysicalErrorBooking(t, tracker, 3, 27, 7)
}

func TestAgentLoopBooksPhysicalNon2xxUsageWithoutTokenTelemetry(t *testing.T) {
	for _, stream := range []bool{false, true} {
		for _, reported := range []bool{false, true} {
			t.Run(fmt.Sprintf("stream=%t/reported=%t", stream, reported), func(t *testing.T) {
				run, _, cleanup := newPromptPipelineTestRunConfig(t, t.Name(), "web_chat")
				defer cleanup()
				client, calls := physicalErrorFixture(t, run.Config, reported, 0, false)
				tracker := newUsageRegressionTracker(t, run.Config)
				run.LLMClient, run.BudgetTracker, run.SuppressTurnSideEffects = llm.WrapOpenAIClient(client), tracker, true
				run.Checkpoint = func([]openai.ChatCompletionMessage) error { return nil }
				afterResponses := 0
				run.ExecutionHooks = &ExecutionHooks{AfterResponse: func(openai.Usage) error { afterResponses++; return nil }}
				broker := &usageRegressionBroker{}
				_, err := ExecuteAgentLoop(context.Background(), openai.ChatCompletionRequest{Model: run.Config.LLM.Model, Messages: []openai.ChatCompletionMessage{{Role: "user", Content: "Fixture task."}}}, run, stream, broker)
				if err == nil || calls.Load() != 1 || broker.updates != 0 || afterResponses != 0 {
					t.Fatalf("error=%v HTTP calls=%d token events=%d usage hooks=%d", err, calls.Load(), broker.updates, afterResponses)
				}
				if reported {
					assertPhysicalErrorBooking(t, tracker, 1, 17, 3)
				} else {
					assertPhysicalErrorBooking(t, tracker, 0, 0, 0)
				}
			})
		}
	}
}
