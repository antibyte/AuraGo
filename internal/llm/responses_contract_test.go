package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"

	"aurago/internal/config"
	"github.com/sashabaranov/go-openai"
)

func responseFunctionTools() []openai.Tool {
	return []openai.Tool{{Type: "function", Function: &openai.FunctionDefinition{Name: "lookup", Parameters: map[string]any{"type": "object", "properties": map[string]any{}}}}}
}

func TestPhysicalRequestUsesSelectedReasoningProtocol(t *testing.T) {
	for _, tc := range []struct {
		provider, model, effort string
		tools, responses        bool
	}{
		{"openai", "gpt-4o", "", false, false},
		{"openai", "o1-mini", "", false, false},
		{"openai", "o3-mini", "", false, false},
		{"openai", "o4-mini", "", false, false},
		{"openai", "gpt-5", "", false, false},
		{"openai", "gpt-6-astra", "", true, true},
		{"openai", "gpt-6.1-sol", "high", true, true},
		{"openai", "gpt-6-sol", "none", true, false},
		{"openai", "gpt-6-luna", "high", true, true},
		{"openrouter", "openai/gpt-6-astra", "", true, true},
		{"openai", "gpt-5.2-codex", "", true, true},
		{"custom", "custom-alias", "", false, false},
		{"custom", "openai/gpt-6-astra", "", true, false},
	} {
		t.Run(tc.model+"/"+tc.effort, func(t *testing.T) {
			fixture := `{"choices":[{"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}]}`
			if tc.responses {
				fixture = `{"id":"r","status":"completed","output":[{"type":"message","content":[{"type":"output_text","text":"ok"}]}]}`
			}
			calls := 0
			client := responseFixtureClient(t, tc.provider, fixture, func(req *http.Request, body map[string]json.RawMessage) {
				calls++
				path := "/v1/chat/completions"
				if tc.responses {
					path = "/v1/responses"
				}
				if req.URL.Path != path || string(body["model"]) != fmt.Sprintf("%q", tc.model) {
					t.Fatalf("wrong route: %s %s", req.URL.Path, body["model"])
				}
				if tc.model == "gpt-4o" || tc.provider == "custom" {
					if string(body["max_tokens"]) != "128" || string(body["temperature"]) != "0.7" {
						t.Fatal("ordinary request changed")
					}
				} else {
					key := "max_completion_tokens"
					if tc.responses {
						key = "max_output_tokens"
					}
					if string(body[key]) != "128" || body["max_tokens"] != nil {
						t.Fatalf("wrong output allowance: %s", body)
					}
					if tc.effort != "none" && body["temperature"] != nil {
						t.Fatal("unsupported sampling reached reasoning provider")
					}
				}
			})
			request := openai.ChatCompletionRequest{Model: tc.model, MaxTokens: 128, Temperature: 0.7, ReasoningEffort: tc.effort, Messages: []openai.ChatCompletionMessage{{Role: "user", Content: "hello"}}}
			if tc.tools {
				request.Tools = responseFunctionTools()
			}
			if _, err := client.CreateChatCompletion(t.Context(), request); err != nil {
				t.Fatal(err)
			}
			if calls != 1 || request.MaxTokens != 128 || request.Temperature != 0.7 {
				t.Fatal("request was changed or sent more than once")
			}
		})
	}
}

func TestResponsesPreservesToolsAndOpaqueContinuation(t *testing.T) {
	fixture := `{"id":"r","model":"gpt-6-astra","status":"completed","output":[{"id":"rs1","type":"reasoning","encrypted_content":"opaque-1","summary":[]},{"id":"fc1","type":"function_call","call_id":"call1","name":"lookup","arguments":"{}"},{"id":"rs2","type":"reasoning","encrypted_content":"opaque-2","future_native_field":true,"summary":[]},{"id":"fc2","type":"function_call","call_id":"call2","name":"lookup","arguments":"{}"}]} `
	requests := 0
	client := responseFixtureClient(t, "openai", fixture, func(req *http.Request, body map[string]json.RawMessage) {
		requests++
		var tools []map[string]json.RawMessage
		if err := json.Unmarshal(body["tools"], &tools); err != nil {
			t.Fatal(err)
		}
		if string(tools[0]["name"]) != `"lookup"` || tools[0]["function"] != nil || string(tools[0]["strict"]) != "false" || string(body["store"]) != "false" {
			t.Fatal("native schema, strictness or storage policy changed")
		}
		var choice map[string]string
		_ = json.Unmarshal(body["tool_choice"], &choice)
		if choice["name"] != "lookup" {
			t.Fatal("forced function choice was lost")
		}
		if requests == 2 {
			var input []map[string]json.RawMessage
			if err := json.Unmarshal(body["input"], &input); err != nil {
				t.Fatal(err)
			}
			want := []string{"reasoning", "function_call", "reasoning", "function_call", "function_call_output", "function_call_output"}
			if len(input) != 7 {
				t.Fatalf("continuation items=%d", len(input))
			}
			for i, kind := range want {
				if string(input[i+1]["type"]) != fmt.Sprintf("%q", kind) {
					t.Fatalf("continuation order: %s", body["input"])
				}
			}
			if string(input[3]["future_native_field"]) != "true" || string(input[1]["encrypted_content"]) != `"opaque-1"` {
				t.Fatal("opaque native reasoning changed")
			}
		}
	})
	req := openai.ChatCompletionRequest{Model: "gpt-6-astra", MaxTokens: 128, Tools: responseFunctionTools(), ToolChoice: openai.ToolChoice{Type: "function", Function: openai.ToolFunction{Name: "lookup"}}, Messages: []openai.ChatCompletionMessage{{Role: "user", Content: "lookup"}}}
	first, err := client.CreateChatCompletion(t.Context(), req)
	if err != nil {
		t.Fatal(err)
	}
	message := first.Choices[0].Message
	if len(message.ToolCalls) != 2 || ReasoningText(message.ReasoningContent) != "" || first.Choices[0].FinishReason != "tool_calls" {
		t.Fatal("tools or private continuation were lost")
	}
	encoded, _ := json.Marshal(message)
	if err := json.Unmarshal(encoded, &message); err != nil {
		t.Fatal(err)
	}
	req.Messages = append(req.Messages, message, openai.ChatCompletionMessage{Role: "tool", ToolCallID: "call1", Content: "one"}, openai.ChatCompletionMessage{Role: "tool", ToolCallID: "call2", Content: "two"})
	if _, err := client.CreateChatCompletion(t.Context(), req); err != nil {
		t.Fatal(err)
	}
	other := responseFixtureClient(t, "openrouter", fixture, func(_ *http.Request, body map[string]json.RawMessage) {
		input := string(body["input"])
		if strings.Contains(input, "encrypted_content") || strings.Contains(input, responsesReasoningPrefix) || !strings.Contains(input, `"call_id":"call1"`) || !strings.Contains(input, `"function_call_output"`) {
			t.Fatalf("foreign-route state leaked or logical tools were lost: %s", input)
		}
	})
	if _, err := other.CreateChatCompletion(t.Context(), req); err != nil {
		t.Fatal(err)
	}
}

func TestResponsesNativeErrorBeforeOutput(t *testing.T) {
	fixture := "data: " + `{"type":"error","code":"rate_limit_exceeded","message":"fixture limit"}` + "\n\n"
	for _, buffered := range []bool{false, true} {
		t.Run(fmt.Sprintf("buffered=%t", buffered), func(t *testing.T) {
			attempts := 0
			client := responseFixtureClient(t, "copilot", fixture, func(*http.Request, map[string]json.RawMessage) { attempts++ })
			request := openai.ChatCompletionRequest{Model: "gpt-5-codex", MaxTokens: 128}
			var err error
			if buffered {
				var out openai.ChatCompletionResponse
				out, err = client.CreateChatCompletion(t.Context(), request)
				if len(out.Choices) != 0 {
					t.Fatal("native error became assistant text")
				}
			} else {
				var stream CompletionStream
				stream, err = client.CreateChatCompletionStream(t.Context(), request)
				if err != nil {
					t.Fatal(err)
				}
				defer stream.Close()
				_, err = stream.Recv()
			}
			if ClassifyError(err) != ErrCategoryRateLimit || attempts != 1 {
				t.Fatalf("attempts=%d err=%v", attempts, err)
			}
		})
	}
}

func TestResponsesStreamErrorsNeverEmitSuccessfulFinish(t *testing.T) {
	for _, ending := range []string{
		`{"type":"error","code":"server_error","message":"failed"}`,
		`{"type":"response.failed","response":{"id":"r","status":"failed","error":{"code":"server_error","message":"failed"},"usage":{"input_tokens":4,"output_tokens":2,"total_tokens":6}}}`,
		`{"type":"response.completed"}`,
		`{"type":"response.output_item.added"}`,
		`{"type":"response.function_call_arguments.delta","output_index":1,"delta":"{}"}`,
		`{"type":"response.failed","response":{"status":"completed","output":[]}}`,
		`{"type":"response.completed","response":{"status":"completed","output":[]}}`,
		`{"type":"response.completed","response":{"status":"completed","output":[{"type":"message","role":"system","content":[{"type":"output_text","text":"partial"}]},{"type":"function_call","call_id":"call","name":"lookup","arguments":"{}"}]}}`,
		`{"type":"response.completed","response":{"status":"completed","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"partial"}]},{"type":"function_call","status":"in_progress","call_id":"call","name":"lookup","arguments":"{}"}]}}`,
		`{"type":"response.completed","response":{"status":"completed","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"partial"}]}],"usage":{"input_tokens":4,"output_tokens":2,"total_tokens":6}}}`,
		"",
	} {
		for _, buffered := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/buffered=%t", ending, buffered), func(t *testing.T) {
				fixture := "data: " + `{"type":"response.output_text.delta","delta":"partial"}` + "\n\n"
				fixture += "data: " + `{"type":"response.output_item.added","output_index":0,"item":{"type":"function_call","call_id":"call","name":"lookup","arguments":""}}` + "\n\n"
				if ending != "" {
					fixture += "data: " + ending + "\n\n"
				}
				attempts := 0
				client := responseFixtureClient(t, "copilot", fixture, func(*http.Request, map[string]json.RawMessage) { attempts++ })
				ctx, capture := CaptureUsage(t.Context())
				request := openai.ChatCompletionRequest{Model: "gpt-5-codex", MaxTokens: 128}
				if buffered {
					out, err := client.CreateChatCompletion(ctx, request)
					if err == nil || len(out.Choices) != 0 {
						t.Fatal("failed native response became a completion")
					}
				} else {
					stream, err := client.CreateChatCompletionStream(ctx, request)
					if err != nil {
						t.Fatal(err)
					}
					for {
						chunk, err := stream.Recv()
						for _, choice := range chunk.Choices {
							if choice.FinishReason != "" {
								t.Fatal("failed native response emitted a finish marker")
							}
						}
						if err != nil {
							if err == io.EOF {
								t.Fatal("failed native response became successful EOF")
							}
							break
						}
					}
					_ = stream.Close()
				}
				if attempts != 1 || len(capture.Snapshot()) != 1 {
					t.Fatal("native stream was replayed or usage observation was duplicated")
				}
				if strings.Contains(ending, `"usage"`) && (capture.Snapshot()[0].OutputTokens == nil || *capture.Snapshot()[0].OutputTokens != 2) {
					t.Fatal("failed native response lost partial usage")
				}
			})
		}
	}
}

func TestFailoverRepreparesEachPhysicalRequest(t *testing.T) {
	cfg := &config.Config{}
	cfg.LLM.ProviderType, cfg.LLM.BaseURL, cfg.LLM.Model = "openai", "https://primary.invalid/v1", "gpt-4o"
	cfg.FallbackLLM.Enabled, cfg.FallbackLLM.ProviderType, cfg.FallbackLLM.BaseURL, cfg.FallbackLLM.Model = true, "openai", "https://fallback.invalid/v1", "o3-mini"
	cfg.FallbackLLM.ErrorThreshold, cfg.FallbackLLM.ProbeIntervalSeconds = 1, 3600
	seen := []string{}
	fm := NewFailoverManager(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), WithClientFactory(func(view *config.Config) *openai.Client {
		cc := openai.DefaultConfig("fixture")
		cc.BaseURL = view.LLM.BaseURL
		cc.HTTPClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			var body map[string]json.RawMessage
			if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			seen = append(seen, req.URL.Host)
			if strings.Contains(req.URL.Host, "fallback") {
				if body["max_tokens"] != nil || body["temperature"] != nil || string(body["max_completion_tokens"]) != "128" {
					t.Fatal("fallback inherited primary sampling")
				}
			} else if string(body["max_tokens"]) != "128" || body["max_completion_tokens"] != nil {
				t.Fatal("fallback rewrite contaminated original request")
			}
			return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"choices":[{"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}]}`))}, nil
		})}
		return openai.NewClientWithConfig(cc)
	}))
	defer fm.Stop()
	fm.recordError(errors.New("connection reset"))
	req := openai.ChatCompletionRequest{Model: "gpt-4o", MaxTokens: 128, Temperature: 0.7}
	if _, err := fm.CreateChatCompletion(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	fm.mu.Lock()
	fm.isOnFallback = false
	fm.mu.Unlock()
	if _, err := fm.CreateChatCompletion(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	if len(seen) != 2 || req.MaxTokens != 128 || req.MaxCompletionTokens != 0 {
		t.Fatal("physical route/request identity changed")
	}
}

func TestResponsesInterleavedToolDeltasAndMeasuredUsage(t *testing.T) {
	var fixture strings.Builder
	emit := func(event any) { data, _ := json.Marshal(event); fmt.Fprintf(&fixture, "data: %s\n\n", data) }
	emit(map[string]any{"type": "response.created", "response": map[string]string{"id": "r", "model": "gpt-5-codex", "status": "in_progress"}})
	emit(map[string]any{"type": "response.output_item.added", "output_index": 1, "item": map[string]string{"id": "f1", "type": "function_call", "call_id": "one", "name": "lookup", "arguments": ""}})
	emit(map[string]any{"type": "response.function_call_arguments.delta", "output_index": 1, "delta": `{"a":`})
	emit(map[string]any{"type": "response.output_item.added", "output_index": 2, "item": map[string]string{"id": "f2", "type": "function_call", "call_id": "two", "name": "lookup", "arguments": ""}})
	emit(map[string]any{"type": "response.function_call_arguments.delta", "output_index": 2, "delta": `{}`})
	emit(map[string]any{"type": "response.function_call_arguments.delta", "output_index": 1, "delta": `1}`})
	emit(map[string]any{"type": "response.completed", "response": map[string]any{"id": "r", "model": "gpt-5-codex", "status": "completed", "service_tier": "flex", "output": []any{
		map[string]string{"type": "reasoning", "encrypted_content": "private", "id": "rs"},
		map[string]string{"type": "function_call", "call_id": "one", "name": "lookup", "arguments": `{"a":1}`},
		map[string]string{"type": "function_call", "call_id": "two", "name": "lookup", "arguments": `{}`},
	}, "usage": map[string]any{"input_tokens": 10, "output_tokens": 7, "total_tokens": 17, "input_tokens_details": map[string]int{"cached_tokens": 3}, "output_tokens_details": map[string]int{"reasoning_tokens": 2}}}})
	for _, buffered := range []bool{false, true} {
		t.Run(fmt.Sprintf("buffered=%t", buffered), func(t *testing.T) {
			client := responseFixtureClient(t, "copilot", fixture.String(), nil)
			ctx, capture := CaptureUsage(t.Context())
			request := openai.ChatCompletionRequest{Model: "gpt-5-codex", MaxTokens: 128, Tools: responseFunctionTools()}
			if buffered {
				response, err := client.CreateChatCompletion(ctx, request)
				if err != nil {
					t.Fatal(err)
				}
				calls := response.Choices[0].Message.ToolCalls
				if len(calls) != 2 || calls[0].ID != "one" || calls[0].Function.Arguments != `{"a":1}` || calls[1].ID != "two" || calls[1].Function.Arguments != `{}` || response.ServiceTier != "flex" || response.Usage.CompletionTokensDetails.ReasoningTokens != 2 {
					t.Fatalf("interleaved functions changed: %+v", response)
				}
			} else {
				stream, err := client.CreateChatCompletionStream(ctx, request)
				if err != nil {
					t.Fatal(err)
				}
				args := map[int]string{}
				finished := false
				for {
					chunk, err := stream.Recv()
					if err == io.EOF {
						break
					}
					if err != nil {
						t.Fatal(err)
					}
					for _, choice := range chunk.Choices {
						for _, call := range choice.Delta.ToolCalls {
							args[*call.Index] += call.Function.Arguments
						}
						finished = finished || choice.FinishReason == "tool_calls"
					}
				}
				_ = stream.Close()
				if !finished || args[0] != `{"a":1}` || args[1] != `{}` {
					t.Fatal("interleaved deltas changed function arguments")
				}
			}
			usage := capture.Snapshot()
			if len(usage) != 1 || usage[0].InputTokens == nil || *usage[0].InputTokens != 10 || usage[0].OutputTokens == nil || *usage[0].OutputTokens != 7 || usage[0].CacheReadTokens == nil || *usage[0].CacheReadTokens != 3 || usage[0].ServiceTier != "flex" {
				t.Fatalf("usage was lost or duplicated: %+v", usage)
			}
		})
	}
}

func TestResponsesInvalidInputAndCancellationMakeNoAttempt(t *testing.T) {
	for _, mutate := range []func(*openai.ChatCompletionRequest){
		func(r *openai.ChatCompletionRequest) { r.MaxCompletionTokens = 12 },
		func(r *openai.ChatCompletionRequest) { r.MaxTokens = -1 },
		func(r *openai.ChatCompletionRequest) { r.N = 2 },
		func(r *openai.ChatCompletionRequest) { r.Seed = new(int) },
	} {
		attempts := 0
		client := responseFixtureClient(t, "copilot", codexResponsesSSEFixture(), func(*http.Request, map[string]json.RawMessage) { attempts++ })
		request := openai.ChatCompletionRequest{Model: "gpt-5-codex", MaxTokens: 16}
		mutate(&request)
		if _, err := client.CreateChatCompletion(t.Context(), request); ClassifyError(err) != ErrCategoryNonRetryableConfig || attempts != 0 {
			t.Fatalf("invalid input reached provider: attempts=%d err=%v", attempts, err)
		}
	}
	attempts := 0
	client := responseFixtureClient(t, "copilot", codexResponsesSSEFixture(), func(*http.Request, map[string]json.RawMessage) { attempts++ })
	ctx, cancel := context.WithCancel(t.Context())
	stream, err := client.CreateChatCompletionStream(ctx, openai.ChatCompletionRequest{Model: "gpt-5-codex", MaxTokens: 16})
	if err != nil {
		t.Fatal(err)
	}
	cancel()
	if _, err := stream.Recv(); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation lost: %v", err)
	}
	if err := stream.Close(); err != nil {
		t.Fatal(err)
	}
	if err := stream.Close(); err != nil {
		t.Fatal(err)
	}
	if attempts != 1 {
		t.Fatal("cancelled stream was replayed")
	}
}

func TestResponsesNativeOptionsAndTerminalReasons(t *testing.T) {
	for _, tc := range []struct{ status, details, output, finish string }{
		{"completed", "", `[{"type":"message","content":[{"type":"output_text","text":"{}"}]}]`, "stop"},
		{"incomplete", `,"incomplete_details":{"reason":"max_output_tokens"}`, `[{"type":"message","content":[{"type":"output_text","text":"partial"}]}]`, "length"},
		{"completed", "", `[{"type":"message","content":[{"type":"refusal","refusal":"refused"}]}]`, "content_filter"},
	} {
		for _, buffered := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/%s/buffered=%t", tc.status, tc.finish, buffered), func(t *testing.T) {
				response := fmt.Sprintf(`{"id":"r","status":%q,"output":%s%s}`, tc.status, tc.output, tc.details)
				fixture := fmt.Sprintf("data: {\"type\":\"response.%s\",\"response\":%s}\n\n", tc.status, response)
				client := responseFixtureClient(t, "copilot", fixture, func(_ *http.Request, body map[string]json.RawMessage) {
					if string(body["parallel_tool_calls"]) != "false" || string(body["tool_choice"]) != `"required"` || !strings.Contains(string(body["text"]), `"type":"json_schema"`) || !strings.Contains(string(body["reasoning"]), `"effort":"high"`) {
						t.Fatalf("native options lost: %s", body)
					}
					input := string(body["input"])
					for _, required := range []string{`"role":"system"`, `"role":"developer"`, `"type":"input_image"`} {
						if !strings.Contains(input, required) {
							t.Fatalf("native input missing %s: %s", required, input)
						}
					}
				})
				request := openai.ChatCompletionRequest{Model: "gpt-5-codex", MaxTokens: 128, Tools: responseFunctionTools(), ToolChoice: "required", ParallelToolCalls: false, ReasoningEffort: "high",
					ResponseFormat: &openai.ChatCompletionResponseFormat{Type: openai.ChatCompletionResponseFormatTypeJSONSchema, JSONSchema: &openai.ChatCompletionResponseFormatJSONSchema{Name: "result", Schema: json.RawMessage(`{"type":"object","properties":{},"additionalProperties":false}`), Strict: true}},
					Messages:       []openai.ChatCompletionMessage{{Role: "system", Content: "System."}, {Role: "developer", Content: "Developer."}, {Role: "user", MultiContent: []openai.ChatMessagePart{{Type: openai.ChatMessagePartTypeImageURL, ImageURL: &openai.ChatMessageImageURL{URL: "data:image/png;base64,AA=="}}}}}}
				if buffered {
					out, err := client.CreateChatCompletion(t.Context(), request)
					if err != nil || len(out.Choices) != 1 || string(out.Choices[0].FinishReason) != tc.finish {
						t.Fatalf("finish=%+v err=%v", out, err)
					}
				} else {
					stream, err := client.CreateChatCompletionStream(t.Context(), request)
					if err != nil {
						t.Fatal(err)
					}
					defer stream.Close()
					finish := ""
					for {
						chunk, err := stream.Recv()
						if err == io.EOF {
							break
						}
						if err != nil {
							t.Fatal(err)
						}
						for _, choice := range chunk.Choices {
							if choice.FinishReason != "" {
								finish = string(choice.FinishReason)
							}
						}
					}
					if finish != tc.finish {
						t.Fatalf("finish=%s want=%s", finish, tc.finish)
					}
				}
			})
		}
	}
}

func TestExcludedAlwaysOnClaudeIsNotAFallbackCandidate(t *testing.T) {
	cfg := &config.Config{}
	cfg.LLM.ProviderType, cfg.LLM.Model = "openai", "gpt-4o"
	cfg.LLM.AnthropicThinking.Enabled = true
	cfg.LLM.AnthropicThinking.ModelAllowlist = []string{"claude-sonnet-4-6"}
	cfg.FallbackLLM.Enabled, cfg.FallbackLLM.ProviderType, cfg.FallbackLLM.Model = true, "anthropic", "claude-opus-5-5"
	cfg.FallbackLLM.BaseURL, cfg.FallbackLLM.ProbeIntervalSeconds = "https://fixture.invalid/v1", 3600
	fm := NewFailoverManager(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	defer fm.Stop()
	if routes := fm.CandidateRoutes(openai.ChatCompletionRequest{Model: "gpt-4o", MaxTokens: 4096}); len(routes) != 1 {
		t.Fatalf("incompatible fallback was retained: %d routes", len(routes))
	}
}
