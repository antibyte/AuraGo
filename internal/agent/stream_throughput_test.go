package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"aurago/internal/llm"

	openai "github.com/sashabaranov/go-openai"
)

// A slow reasoning model that keeps streaming past the per-call timeout
// finishes under a throughput policy; without one the fixed timeout still ends
// the call.
func TestStreamThroughputPolicyOutlastsFixedTimeout(t *testing.T) {
	for _, tc := range []struct {
		name      string
		policy    *llm.ThroughputPolicy
		wantError bool
	}{
		{name: "fixed timeout ends a long stream", wantError: true},
		{name: "steady output extends the call", policy: &llm.ThroughputPolicy{MinTokensPerSecond: 5, MaxFactor: 4, MinSample: 200 * time.Millisecond}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			runCfg, _, cleanup := newPromptPipelineTestRunConfig(t, "stream-throughput", "game_maker")
			defer cleanup()
			runCfg.Config.CircuitBreaker.LLMTimeoutSeconds = 1
			runCfg.Config.CircuitBreaker.LLMStreamChunkTimeoutSeconds = 1
			runCfg.RequireCompleteStream = true
			runCfg.PreserveReasoning = true
			runCfg.SuppressTurnSideEffects = true
			runCfg.ToolCallLimit = 2
			runCfg.StreamThroughput = tc.policy
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			var calls atomic.Int32
			provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var request openai.ChatCompletionRequest
				_ = json.NewDecoder(r.Body).Decode(&request)
				calls.Add(1)
				w.Header().Set("Content-Type", "text/event-stream")
				chunk, _ := json.Marshal(map[string]any{"choices": []any{map[string]any{"index": 0, "delta": map[string]any{"reasoning_content": strings.Repeat("think ", 16)}}}})
				for end := time.Now().Add(2500 * time.Millisecond); time.Now().Before(end); {
					select {
					case <-r.Context().Done():
						return
					case <-time.After(50 * time.Millisecond):
					}
					fmt.Fprintf(w, "data: %s\n\n", chunk)
					w.(http.Flusher).Flush()
				}
				fmt.Fprint(w, "data: {\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\",\"content\":\"Finished.<done/>\"},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n")
			}))
			defer provider.Close()
			clientCfg := openai.DefaultConfig("local-test")
			clientCfg.BaseURL = provider.URL
			runCfg.LLMClient = openai.NewClientWithConfig(clientCfg)
			_, err := ExecuteAgentLoop(ctx, openai.ChatCompletionRequest{Model: runCfg.Config.LLM.Model, Stream: true, Messages: []openai.ChatCompletionMessage{
				{Role: "user", Content: "Create the current game."},
			}}, runCfg, true, NoopBroker{})
			if (err != nil) != tc.wantError {
				t.Fatalf("error=%v, want error=%v (requests=%d)", err, tc.wantError, calls.Load())
			}
		})
	}
}

// Tool-free source generation reads its own stream; it reports progress too.
func TestMinimalLoopStreamReportsThroughput(t *testing.T) {
	for _, steady := range []bool{false, true} {
		t.Run(fmt.Sprintf("throughput=%t", steady), func(t *testing.T) {
			provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/event-stream")
				chunk, _ := json.Marshal(map[string]any{"choices": []any{map[string]any{"index": 0, "delta": map[string]any{"reasoning_content": strings.Repeat("think ", 16)}}}})
				for end := time.Now().Add(2500 * time.Millisecond); time.Now().Before(end); {
					select {
					case <-r.Context().Done():
						return
					case <-time.After(50 * time.Millisecond):
					}
					fmt.Fprintf(w, "data: %s\n\n", chunk)
					w.(http.Flusher).Flush()
				}
				fmt.Fprint(w, "data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"export const game = 1;\"},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n")
			}))
			defer provider.Close()
			clientCfg := openai.DefaultConfig("local-test")
			clientCfg.BaseURL = provider.URL
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			if steady {
				cancel()
				ctx, cancel = llm.NewThroughputDeadline(context.Background(), llm.ThroughputPolicy{Base: time.Second, ReserveTokens: 4096, MinTokensPerSecond: 5, MaxFactor: 4, MinSample: 200 * time.Millisecond})
			}
			defer cancel()
			response, err := minimalLoopStreamText(ctx, openai.NewClientWithConfig(clientCfg), openai.ChatCompletionRequest{Model: "test", Stream: true})
			if steady && (err != nil || response.Choices[0].Message.Content != "export const game = 1;") {
				t.Fatalf("steady stream did not finish: %v", err)
			}
			if !steady && err == nil {
				t.Fatal("fixed one-second deadline did not end the long stream")
			}
		})
	}
}
