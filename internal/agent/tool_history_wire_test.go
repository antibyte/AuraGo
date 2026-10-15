package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"aurago/internal/llm"
	openai "github.com/sashabaranov/go-openai"
)

func emptyArgumentToolHistory() []openai.ChatCompletionMessage {
	index := 0
	return []openai.ChatCompletionMessage{
		{Role: "user", Content: "Create a breakout game."},
		{Role: "assistant", ReasoningContent: "Private fixture reasoning", ToolCalls: []openai.ToolCall{{Index: &index, ID: "call-empty", Type: "function", Function: openai.FunctionCall{Name: "game_maker_project"}}}},
		{Role: "tool", ToolCallID: "call-empty", Content: `{"status":"error","message":"unknown project operation"}`},
	}
}

func TestFinalizePromptRequestNormalizesToolHistoryWithoutChangingArchive(t *testing.T) {
	for _, item := range []struct{ arguments, want string }{
		{"", "{}"},
		{" \n\t", "{}"},
		{`{"operation":"read"}`, `{"operation":"read"}`},
		{" \n{\"operation\":\"read\"}\t", " \n{\"operation\":\"read\"}\t"},
		{`{"unfinished":`, "{}"},
		{`{"operation": "set_design"`, "{}"},
		{`null`, "{}"},
		{`[]`, "{}"},
		{`"text"`, "{}"},
		{`42`, "{}"},
	} {
		t.Run(item.arguments, func(t *testing.T) {
			history := emptyArgumentToolHistory()
			history[1].ToolCalls[0].Function.Arguments = item.arguments
			before, _ := json.Marshal(history)
			req := openai.ChatCompletionRequest{Model: "fixture", Messages: history}
			result, err := finalizePromptRequestForSend(&req, promptRequestFinalizationTestBudget(), nil, "openai", nil, true)
			if err != nil || result.DroppedToolMessages != 0 {
				t.Fatalf("finalize: %+v, %v", result, err)
			}
			call := req.Messages[1].ToolCalls[0]
			if call.Index != nil || call.Function.Arguments != item.want || call.ID != "call-empty" {
				t.Fatalf("invalid outgoing call: %+v", call)
			}
			if req.Messages[1].ReasoningContent != history[1].ReasoningContent || !reflect.DeepEqual(req.Messages[2], history[2]) {
				t.Fatal("reasoning or paired tool feedback changed")
			}
			after, _ := json.Marshal(history)
			if string(before) != string(after) {
				t.Fatal("request normalization mutated the original history")
			}
		})
	}
}

func TestAgentLoopsSendValidToolArgumentHistory(t *testing.T) {
	for _, item := range []struct {
		name, arguments string
		minimal         bool
	}{
		{"main_empty", "", false},
		{"minimal_empty", "", true},
		{"main_truncated", `{"operation": "set_design"`, false},
		{"minimal_truncated", `{"operation": "set_design"`, true},
	} {
		t.Run(item.name, func(t *testing.T) {
			var accepted atomic.Int32
			provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost {
					w.WriteHeader(http.StatusNotFound)
					return
				}
				var request struct {
					Messages []struct {
						Role       string                       `json:"role"`
						ToolCallID string                       `json:"tool_call_id"`
						ToolCalls  []map[string]json.RawMessage `json:"tool_calls"`
					} `json:"messages"`
				}
				if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
					t.Error(err)
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				found := false
				for i, message := range request.Messages {
					for _, call := range message.ToolCalls {
						var function map[string]string
						_ = json.Unmarshal(call["function"], &function)
						if _, hasIndex := call["index"]; hasIndex || function["arguments"] != "{}" {
							w.WriteHeader(http.StatusBadRequest)
							fmt.Fprint(w, `{"error":{"message":"invalid tool-call history","type":"invalid_request_error"}}`)
							return
						}
						found = i+1 < len(request.Messages) && request.Messages[i+1].Role == "tool" && request.Messages[i+1].ToolCallID == "call-empty"
					}
				}
				if !found {
					t.Error("missing paired tool history")
				}
				accepted.Add(1)
				w.Header().Set("Content-Type", "text/event-stream")
				fmt.Fprint(w, "data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"Done.<done/>\"},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n")
			}))
			defer provider.Close()
			run, _, cleanup := newPromptPipelineTestRunConfig(t, t.Name(), "game_maker")
			defer cleanup()
			run.Config.LLM.Model, run.Config.LLM.ProviderType, run.Config.LLM.BaseURL = "tencent/hy4-preview", "openai", provider.URL
			run.Config.CircuitBreaker.LLMTimeoutSeconds = 5
			run.PreserveReasoning, run.RequireCompleteStream, run.SuppressTurnSideEffects = true, true, true
			run.PreparedPrompt, _ = NewPreparedPromptProfile("fixture/v1", "Continue the game after tool feedback.", nil)
			run.LLMClient = llm.WrapOpenAIClient(llm.NewClientFromProviderWithConfig(run.Config, "openai", provider.URL, "fixture", ""))
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			history := emptyArgumentToolHistory()
			history[1].ToolCalls[0].Function.Arguments = item.arguments
			var err error
			if item.minimal {
				_, _, err = ExecuteMinimalLoop(ctx, run.LLMClient, run.Config.LLM.Model, "", "Continue the game.", nil, &DispatchContext{Cfg: run.Config}, history, run.Logger, &MinimalLoopOptions{PreparedPrompt: run.PreparedPrompt, PreserveReasoning: true, StreamText: true, MaxToolRounds: 0})
			} else {
				_, err = ExecuteAgentLoop(ctx, openai.ChatCompletionRequest{Model: run.Config.LLM.Model, Messages: history}, run, true, NoopBroker{})
			}
			if err != nil || accepted.Load() != 1 {
				t.Fatalf("continuation rejected: accepted=%d err=%v", accepted.Load(), err)
			}
		})
	}
}

func TestAgentLoopMalformedCallAfterRecoveryLimit(t *testing.T) {
	client := &gameMakerCacheClient{}
	for i := 0; i < 3; i++ {
		response := gameMakerCacheToolResponse(fmt.Sprintf("invalid-%d", i), "status")
		response.Choices[0].Message.ToolCalls[0].Function.Arguments = `{"operation":"status"`
		client.responses = append(client.responses, response)
	}
	client.responses = append(client.responses, gameMakerCacheFinalResponse())
	run := gameMakerCacheRun(t, client)
	run.PreparedPrompt, _ = NewPreparedPromptProfile("fixture/v1", "Continue after tool feedback.", run.NativeToolSchemas)
	handled := 0
	run.ExecutionHooks = &ExecutionHooks{HandleTool: func(context.Context, ToolCall) (string, bool) {
		handled++
		return `{"status":"success"}`, true
	}}
	_, err := ExecuteAgentLoop(context.Background(), gameMakerCacheRequest(run), run, false, NoopBroker{})
	if err != nil || len(client.requests) != 4 || handled != 0 {
		t.Fatalf("malformed recovery: err=%v requests=%d dispatched=%d", err, len(client.requests), handled)
	}
	last := client.requests[3]
	feedback := toolResultForCall(t, last.Messages, "invalid-2")
	if !strings.Contains(feedback.Content, "invalid_arguments") {
		t.Fatalf("missing invalid-argument feedback: %s", feedback.Content)
	}
	for _, message := range last.Messages {
		for _, call := range message.ToolCalls {
			if call.Function.Arguments != "{}" {
				t.Fatalf("malformed arguments escaped into recovery: %q", call.Function.Arguments)
			}
		}
	}
}
