package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/llm"
	"github.com/sashabaranov/go-openai"
)

type responsesAgentTransport func(*http.Request) (*http.Response, error)

func (f responsesAgentTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestResponsesAgentLoopsRetainToolContinuation(t *testing.T) {
	for _, mode := range []string{"main", "main-stream", "minimal"} {
		for _, providerType := range []string{"openai", "copilot"} {
			t.Run(providerType+"/"+mode, func(t *testing.T) {
				run, _, cleanup := newPromptPipelineTestRunConfig(t, t.Name(), "web")
				defer cleanup()
				model := "gpt-6-astra"
				if providerType == "copilot" {
					model = "gpt-5-codex"
				}
				run.Config.LLM.Provider, run.Config.LLM.ProviderType, run.Config.LLM.Model = "fixture", providerType, model
				run.Config.LLM.BaseURL, run.Config.LLM.UseNativeFunctions = "https://fixture.invalid/v1", true
				run.Config.Providers = []config.ProviderEntry{{ID: "fixture", Type: providerType, BaseURL: run.Config.LLM.BaseURL, Model: model, ContextWindow: 1000000, MaxOutputTokens: 4096}}
				run.IsMission, run.SuppressTurnSideEffects, run.StableSystemPrompt = true, true, true
				run.ToolCallLimit = 1
				run.NativeToolSchemas = []openai.Tool{{Type: "function", Function: &openai.FunctionDefinition{Name: "list_processes", Parameters: map[string]any{"type": "object", "properties": map[string]any{}}}}}
				run.PreparedPrompt, _ = NewPreparedPromptProfile("responses/continuation", "Inspect status, then summarize.", run.NativeToolSchemas)
				requests, dispatched := 0, 0
				cc := openai.DefaultConfig("fixture")
				cc.BaseURL = run.Config.LLM.BaseURL
				cc.HTTPClient = &http.Client{Transport: responsesAgentTransport(func(r *http.Request) (*http.Response, error) {
					requests++
					var wire map[string]json.RawMessage
					if err := json.NewDecoder(r.Body).Decode(&wire); err != nil {
						t.Fatal(err)
					}
					if r.URL.Path != "/v1/responses" || wire["messages"] != nil || wire["temperature"] != nil || wire["max_tokens"] != nil || wire["max_completion_tokens"] != nil || len(wire["max_output_tokens"]) == 0 {
						t.Fatalf("SDK request not adapted: %s %s", r.URL.Path, wire)
					}
					var output json.RawMessage
					if requests == 1 {
						output = json.RawMessage(`[{"id":"rs_fixture","type":"reasoning","encrypted_content":"opaque","summary":[]},{"id":"fc_fixture","type":"function_call","call_id":"call_fixture","name":"list_processes","arguments":"{}"}]`)
					} else {
						input := string(wire["input"])
						if requests != 2 || !strings.Contains(input, `"function_call_output"`) || !strings.Contains(input, `"encrypted_content":"opaque"`) || !strings.Contains(input, `"id":"fc_fixture"`) {
							t.Fatalf("tool continuation was lost: %s", input)
						}
						output = json.RawMessage(`[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"Done.<done/>"}]}]`)
					}
					native := map[string]any{"id": "resp_fixture", "model": model, "status": "completed", "output": output, "usage": map[string]int{"input_tokens": 8, "output_tokens": 3, "total_tokens": 11}}
					body, _ := json.Marshal(native)
					contentType := "application/json"
					if string(wire["stream"]) == "true" {
						contentType = "text/event-stream"
						body = []byte("data: " + `{"type":"response.completed","response":` + string(body) + "}\n\n")
					}
					return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {contentType}}, Body: io.NopCloser(strings.NewReader(string(body)))}, nil
				})}
				run.LLMClient = llm.WrapOpenAIClient(openai.NewClientWithConfig(cc))
				run.ExecutionHooks = &ExecutionHooks{HandleTool: func(context.Context, ToolCall) (string, bool) { dispatched++; return `{"status":"success"}`, true }}
				var err error
				if mode == "minimal" {
					_, _, err = ExecuteMinimalLoop(t.Context(), run.LLMClient, model, "", "Inspect status.", nil, &DispatchContext{Cfg: run.Config, ExecutionHooks: run.ExecutionHooks}, nil, run.Logger, &MinimalLoopOptions{PreparedPrompt: run.PreparedPrompt, MaxToolRounds: 1, MaxToolCalls: 1})
				} else {
					_, err = ExecuteAgentLoop(t.Context(), openai.ChatCompletionRequest{Model: model, Messages: []openai.ChatCompletionMessage{{Role: "user", Content: "Inspect status."}}}, run, mode == "main-stream", NoopBroker{})
				}
				if err != nil || requests != 2 || dispatched != 1 {
					t.Fatal(fmt.Sprintf("requests=%d dispatched=%d err=%v", requests, dispatched, err))
				}
			})
		}
	}
}

func TestGLMAgentLoopsPreserveEveryReasoningToolTurn(t *testing.T) {
	for _, mode := range []string{"main", "minimal"} {
		t.Run(mode, func(t *testing.T) {
			run, _, cleanup := newPromptPipelineTestRunConfig(t, t.Name(), "web")
			defer cleanup()
			const model = "glm-4.7"
			run.Config.LLM.Provider, run.Config.LLM.ProviderType, run.Config.LLM.Model = "fixture", "glm", model
			run.Config.LLM.BaseURL, run.Config.LLM.UseNativeFunctions = "https://fixture.invalid/v1", true
			run.Config.Providers = []config.ProviderEntry{{ID: "fixture", Type: "glm", BaseURL: run.Config.LLM.BaseURL, Model: model, ContextWindow: 1000000, MaxOutputTokens: 4096}}
			run.IsMission, run.SuppressTurnSideEffects, run.StableSystemPrompt = true, true, true
			run.ToolCallLimit = 2
			run.NativeToolSchemas = []openai.Tool{{Type: "function", Function: &openai.FunctionDefinition{Name: "list_processes", Parameters: map[string]any{"type": "object", "properties": map[string]any{}}}}}
			run.PreparedPrompt, _ = NewPreparedPromptProfile("glm/continuation", "Inspect status twice, then summarize.", run.NativeToolSchemas)
			requests, dispatched := 0, 0
			cc := openai.DefaultConfig("fixture")
			cc.BaseURL = run.Config.LLM.BaseURL
			cc.HTTPClient = &http.Client{Transport: responsesAgentTransport(func(r *http.Request) (*http.Response, error) {
				requests++
				var wire openai.ChatCompletionRequest
				if err := json.NewDecoder(r.Body).Decode(&wire); err != nil {
					t.Fatal(err)
				}
				if r.URL.Path != "/v1/chat/completions" {
					t.Fatalf("unexpected GLM path: %s", r.URL.Path)
				}
				reasoning := []string{}
				for _, m := range wire.Messages {
					if m.Role == "assistant" && len(m.ToolCalls) != 0 {
						reasoning = append(reasoning, m.ReasoningContent)
					}
				}
				if len(reasoning) != requests-1 {
					t.Fatalf("lost tool turns: %v", reasoning)
				}
				for i, value := range reasoning {
					if value != fmt.Sprintf("reasoning-%d", i+1) {
						t.Fatalf("GLM reasoning changed: %v", reasoning)
					}
				}
				message := openai.ChatCompletionMessage{Role: "assistant", Content: "Done.<done/>"}
				finish := openai.FinishReasonStop
				if requests <= 2 {
					message.Content, message.ReasoningContent = "", fmt.Sprintf("reasoning-%d", requests)
					message.ToolCalls = []openai.ToolCall{{ID: fmt.Sprintf("call-%d", requests), Type: "function", Function: openai.FunctionCall{Name: "list_processes", Arguments: "{}"}}}
					finish = openai.FinishReasonToolCalls
				}
				body, _ := json.Marshal(openai.ChatCompletionResponse{ID: "glm-fixture", Model: model, Choices: []openai.ChatCompletionChoice{{Message: message, FinishReason: finish}}, Usage: openai.Usage{PromptTokens: 8, CompletionTokens: 3, TotalTokens: 11}})
				return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {"application/json"}}, Body: io.NopCloser(strings.NewReader(string(body)))}, nil
			})}
			run.LLMClient = llm.WrapOpenAIClient(openai.NewClientWithConfig(cc))
			run.ExecutionHooks = &ExecutionHooks{HandleTool: func(context.Context, ToolCall) (string, bool) { dispatched++; return `{"status":"success"}`, true }}
			var err error
			if mode == "minimal" {
				_, _, err = ExecuteMinimalLoop(t.Context(), run.LLMClient, model, "", "Inspect status twice.", nil, &DispatchContext{Cfg: run.Config, ExecutionHooks: run.ExecutionHooks}, nil, run.Logger, &MinimalLoopOptions{PreparedPrompt: run.PreparedPrompt, MaxToolRounds: 2, MaxToolCalls: 2})
			} else {
				_, err = ExecuteAgentLoop(t.Context(), openai.ChatCompletionRequest{Model: model, Messages: []openai.ChatCompletionMessage{{Role: "user", Content: "Inspect status twice."}}}, run, false, NoopBroker{})
			}
			if err != nil || requests != 3 || dispatched != 2 {
				t.Fatalf("requests=%d dispatched=%d err=%v", requests, dispatched, err)
			}
		})
	}
}
