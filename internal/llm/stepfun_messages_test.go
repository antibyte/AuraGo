package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	openai "github.com/sashabaranov/go-openai"
)

func TestStepFunChatMessagesUseAcceptedWireShape(t *testing.T) {
	for _, tc := range []struct {
		name, providerType, model string
		stream                    bool
		stepFun                   bool
	}{
		{name: "direct sync", providerType: "stepfun", model: "step-5-preview", stepFun: true},
		{name: "direct stream", providerType: "stepfun", model: "step-5-preview", stream: true, stepFun: true},
		{name: "openrouter stream", providerType: "openrouter", model: "stepfun/step-5-preview", stream: true, stepFun: true},
		{name: "other openrouter model unchanged", providerType: "openrouter", model: "deepseek/deepseek-chat", stream: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var wire map[string]json.RawMessage
			index := 0
			provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/v1/chat/completions" {
					t.Errorf("unexpected path %q", r.URL.Path)
				}
				body, err := io.ReadAll(r.Body)
				if err != nil {
					t.Error(err)
					return
				}
				if r.ContentLength != int64(len(body)) {
					t.Errorf("content length = %d, body = %d", r.ContentLength, len(body))
				}
				if err := json.Unmarshal(body, &wire); err != nil {
					t.Error(err)
					return
				}
				if tc.stream {
					w.Header().Set("Content-Type", "text/event-stream")
					fmt.Fprint(w, "data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"ok\"},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n")
					return
				}
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprint(w, `{"id":"test","object":"chat.completion","model":"step-5-preview","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}]}`)
			}))
			defer provider.Close()

			client := NewClientFromProviderWithConfig(nil, tc.providerType, provider.URL+"/v1", "test-only", "")
			request := openai.ChatCompletionRequest{Model: tc.model, Messages: []openai.ChatCompletionMessage{
				{Role: "user", Content: "Build a game."},
				{Role: "assistant", ReasoningContent: "private interrupted thought"},
				{Role: "assistant", ReasoningContent: "private tool reasoning", ToolCalls: []openai.ToolCall{{Index: &index, ID: "call-1", Type: openai.ToolTypeFunction, Function: openai.FunctionCall{Name: "game_maker_file", Arguments: `{}`}}}},
				{Role: "tool", ToolCallID: "call-1", Name: "game_maker_file", Content: "source read"},
				{Role: "user", Content: "Continue."},
			}}
			if tc.stream {
				stream, err := client.CreateChatCompletionStream(context.Background(), request)
				if err != nil {
					t.Fatal(err)
				}
				defer stream.Close()
				if _, err := stream.Recv(); err != nil {
					t.Fatal(err)
				}
			} else if _, err := client.CreateChatCompletion(context.Background(), request); err != nil {
				t.Fatal(err)
			}
			if request.Messages[1].ReasoningContent != "private interrupted thought" || request.Messages[2].ReasoningContent != "private tool reasoning" {
				t.Fatal("normalization changed the private conversation")
			}
			var messages []map[string]json.RawMessage
			if err := json.Unmarshal(wire["messages"], &messages); err != nil {
				t.Fatal(err)
			}
			if tc.stepFun {
				if len(messages) != 4 || string(messages[1]["content"]) != "null" {
					t.Fatalf("StepFun messages have invalid shape: %s", wire["messages"])
				}
				for _, message := range messages {
					if _, found := message["reasoning_content"]; found {
						t.Fatal("StepFun request replayed private reasoning")
					}
				}
				if _, found := messages[2]["name"]; found {
					t.Fatal("StepFun tool result retained a nonstandard name")
				}
				var calls []map[string]json.RawMessage
				if err := json.Unmarshal(messages[1]["tool_calls"], &calls); err != nil || len(calls) != 1 {
					t.Fatalf("StepFun tool calls: %s (%v)", messages[1]["tool_calls"], err)
				}
				if _, found := calls[0]["index"]; found {
					t.Fatal("StepFun tool call retained a stream-only index")
				}
			} else if len(messages) != 5 || !strings.Contains(string(wire["messages"]), "reasoning_content") {
				t.Fatalf("another OpenRouter model was modified: %s", wire["messages"])
			}
		})
	}
}

func TestIsStepFunAPIBaseURL(t *testing.T) {
	for _, tc := range []struct {
		url  string
		want bool
	}{
		{"https://api.stepfun.ai/v1", true},
		{"https://api.stepfun.com/step_plan/v1", true},
		{"https://api.stepfun.ai.evil.example/v1", false},
		{"https://openrouter.ai/api/v1", false},
	} {
		if got := isStepFunAPIBaseURL(tc.url); got != tc.want {
			t.Errorf("isStepFunAPIBaseURL(%q) = %v, want %v", tc.url, got, tc.want)
		}
	}
}
