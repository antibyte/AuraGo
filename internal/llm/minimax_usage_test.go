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

	"github.com/sashabaranov/go-openai"
)

func TestMiniMaxM3StreamPreservesToolReasoningAndReportsUsage(t *testing.T) {
	for _, model := range []string{"MiniMax-M3", "MiniMax-M3.1-Flash"} {
		t.Run(model, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var request struct {
					Stream  bool `json:"stream"`
					Options struct {
						IncludeUsage bool `json:"include_usage"`
					} `json:"stream_options"`
					Messages []map[string]any `json:"messages"`
				}
				if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
					t.Error(err)
				}
				if !request.Stream || !request.Options.IncludeUsage {
					t.Error("MiniMax stream must request usage")
				}
				for _, message := range request.Messages {
					if message["role"] == "assistant" && message["reasoning_content"] != "private tool reasoning" {
						t.Error("MiniMax continuation lost reasoning_content")
					}
				}
				w.Header().Set("Content-Type", "text/event-stream")
				fmt.Fprint(w, "data: {\"choices\":[{\"index\":0,\"delta\":{\"reasoning_content\":\"next thought\"}}]}\n\n")
				fmt.Fprint(w, "data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"done\"},\"finish_reason\":\"stop\"}]}\n\n")
				if request.Options.IncludeUsage {
					fmt.Fprint(w, "data: {\"choices\":[],\"usage\":{\"prompt_tokens\":100,\"completion_tokens\":7,\"prompt_tokens_details\":{\"cached_tokens\":60}}}\n\n")
				}
				fmt.Fprint(w, "data: [DONE]\n\n")
			}))
			defer server.Close()
			client := NewClientFromProvider("minimax", server.URL+"/v1", "fixture")
			ctx, capture := CaptureUsage(context.Background())
			response, err := client.CreateChatCompletionStream(ctx, openai.ChatCompletionRequest{
				Model: model,
				Messages: []openai.ChatCompletionMessage{
					{Role: "user", Content: "Inspect the game plan"},
					{Role: "assistant", ReasoningContent: "private tool reasoning", ToolCalls: []openai.ToolCall{{
						ID: "call-plan", Type: "function", Function: openai.FunctionCall{Name: "game_maker_project", Arguments: `{"operation":"get_plan"}`},
					}}},
					{Role: "tool", ToolCallID: "call-plan", Content: `{"status":"ok"}`},
				},
			})
			if err != nil {
				t.Fatal(err)
			}
			var reasoning strings.Builder
			for {
				chunk, err := response.Recv()
				if err == io.EOF {
					break
				}
				if err != nil {
					t.Fatal(err)
				}
				for _, choice := range chunk.Choices {
					reasoning.WriteString(choice.Delta.ReasoningContent)
				}
			}
			if err := response.Close(); err != nil {
				t.Fatal(err)
			}
			if reasoning.String() != "next thought" {
				t.Fatal("stream reasoning was not preserved")
			}
			items := capture.Snapshot()
			if len(items) != 1 || items[0].InputTokens == nil || *items[0].InputTokens != 100 ||
				items[0].OutputTokens == nil || *items[0].OutputTokens != 7 || items[0].CacheReadTokens == nil || *items[0].CacheReadTokens != 60 {
				t.Fatalf("missing or duplicate MiniMax usage: %+v", items)
			}
		})
	}
}
