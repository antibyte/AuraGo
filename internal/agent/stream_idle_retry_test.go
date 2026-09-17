package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	openai "github.com/sashabaranov/go-openai"
)

func TestIsolatedStreamIdleRetry(t *testing.T) {
	for _, tc := range []struct {
		name       string
		enabled    bool
		secondIdle bool
		cancel     bool
		length     bool
		wantCalls  int32
		wantError  bool
	}{
		{name: "retry keeps context", enabled: true, wantCalls: 2},
		{name: "default has no retry", wantCalls: 1, wantError: true},
		{name: "only one retry", enabled: true, secondIdle: true, wantCalls: 2, wantError: true},
		{name: "cancellation does not retry", enabled: true, cancel: true, wantCalls: 1, wantError: true},
		{name: "truncation does not retry", enabled: true, length: true, wantCalls: 1, wantError: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			runCfg, _, cleanup := newPromptPipelineTestRunConfig(t, "stream-idle", "game_maker")
			defer cleanup()
			runCfg.Config.CircuitBreaker.LLMTimeoutSeconds = 5
			runCfg.Config.CircuitBreaker.LLMStreamChunkTimeoutSeconds = 1
			runCfg.RequireCompleteStream = true
			runCfg.PreserveReasoning = true
			runCfg.RetryStreamIdle = tc.enabled
			runCfg.SuppressTurnSideEffects = true
			runCfg.ToolCallLimit = 2
			var checkpoint []openai.ChatCompletionMessage
			runCfg.Checkpoint = func(messages []openai.ChatCompletionMessage) error {
				checkpoint = append([]openai.ChatCompletionMessage(nil), messages...)
				return nil
			}
			ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
			defer cancel()
			requests := make(chan openai.ChatCompletionRequest, 4)
			var calls atomic.Int32
			provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var request openai.ChatCompletionRequest
				if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
					t.Error(err)
					return
				}
				requests <- request
				n := calls.Add(1)
				w.Header().Set("Content-Type", "text/event-stream")
				if n == 1 || tc.secondIdle {
					fmt.Fprint(w, "data: {\"choices\":[{\"index\":0,\"delta\":{\"reasoning_content\":\"private pending reasoning\",\"tool_calls\":[{\"index\":0,\"id\":\"partial-write\",\"type\":\"function\",\"function\":{\"name\":\"game_maker_file\",\"arguments\":\"{}\"}}]}}]}\n\n")
					w.(http.Flusher).Flush()
					if tc.length {
						fmt.Fprint(w, "data: {\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"length\"}]}\n\ndata: [DONE]\n\n")
						return
					}
					if tc.cancel {
						cancel()
					}
					<-r.Context().Done()
					return
				}
				fmt.Fprint(w, "data: {\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\",\"content\":\"Finished.<done/>\"},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n")
			}))
			defer provider.Close()
			clientCfg := openai.DefaultConfig("local-test")
			clientCfg.BaseURL = provider.URL
			runCfg.LLMClient = openai.NewClientWithConfig(clientCfg)
			_, err := ExecuteAgentLoop(ctx, openai.ChatCompletionRequest{Model: runCfg.Config.LLM.Model, Stream: true, Messages: []openai.ChatCompletionMessage{
				{Role: "user", Content: "Create the current game."},
				{Role: "assistant", ToolCalls: []openai.ToolCall{{ID: "completed-read", Type: "function", Function: openai.FunctionCall{Name: "game_maker_file", Arguments: `{"operation":"read"}`}}}},
				{Role: "tool", ToolCallID: "completed-read", Content: "known source result"},
				{Role: "user", Content: "Continue using this source."},
			}}, runCfg, true, NoopBroker{})
			if (err != nil) != tc.wantError || calls.Load() != tc.wantCalls {
				t.Fatalf("error=%v, requests=%d, want error=%v/requests=%d", err, calls.Load(), tc.wantError, tc.wantCalls)
			}
			if tc.cancel && !errors.Is(err, context.Canceled) {
				t.Fatalf("cancellation changed: %v", err)
			}
			for _, message := range checkpoint {
				for _, call := range message.ToolCalls {
					if call.ID == "partial-write" {
						t.Fatal("incomplete call survived in the checkpoint")
					}
				}
				if message.ToolCallID == "partial-write" {
					t.Fatal("incomplete call was dispatched")
				}
			}
			if tc.wantCalls == 2 {
				<-requests
				retried := <-requests
				knownResults, reasoning := 0, false
				for _, message := range retried.Messages {
					if message.ToolCallID == "completed-read" && message.Content == "known source result" {
						knownResults++
					}
					reasoning = reasoning || message.ReasoningContent == "private pending reasoning"
				}
				if knownResults != 1 || !reasoning {
					t.Fatalf("retry lost completed context or private reasoning: known=%d, reasoning=%v", knownResults, reasoning)
				}
			}
		})
	}
}
