package agent

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/sashabaranov/go-openai"
)

func TestStrictStreamCompletionRejectsIncompleteWork(t *testing.T) {
	for _, kind := range []string{"stop", "tool_calls", "reasoning_length", "length", "missing_finish", "idle", "deadline"} {
		t.Run(kind, func(t *testing.T) {
			provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/event-stream")
				fmt.Fprint(w, "data: {\"choices\":[{\"index\":0,\"delta\":{\"reasoning_content\":\"private fragment\"}}]}\n\n")
				if kind != "reasoning_length" {
					// Even syntactically valid calls must not run before completion.
					fmt.Fprint(w, "data: {\"choices\":[{\"index\":0,\"delta\":{\"tool_calls\":[{\"index\":0,\"id\":\"write\",\"type\":\"function\",\"function\":{\"name\":\"game_maker_file\",\"arguments\":\"{}\"}}]}}]}\n\n")
				}
				w.(http.Flusher).Flush()
				if kind == "deadline" || kind == "idle" {
					<-r.Context().Done()
					return
				}
				if kind != "missing_finish" {
					finish := strings.TrimPrefix(kind, "reasoning_")
					fmt.Fprintf(w, "data: {\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":%q}]}\n\n", finish)
				}
				fmt.Fprint(w, "data: [DONE]\n\n")
			}))
			defer provider.Close()
			cfg := openai.DefaultConfig("test-only")
			cfg.BaseURL = provider.URL
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			if kind == "deadline" {
				cancel()
				ctx, cancel = context.WithTimeout(context.Background(), 150*time.Millisecond)
			}
			defer cancel()
			idle := time.Second
			if kind == "idle" {
				idle = 150 * time.Millisecond
			}
			retries := 0
			result := handleStreamingResponse(ctx, openai.ChatCompletionRequest{Model: "test", Stream: true}, openai.NewClientWithConfig(cfg), false, defaultRecoveryPolicy(), slog.New(slog.NewTextHandler(io.Discard, nil)), &captureBroker{}, AgentTelemetryScope{}, cancel, idle, &retries, true)
			valid := kind == "stop" || kind == "tool_calls" || kind == "reasoning_length"
			if valid {
				if result.err != nil || len(result.resp.Choices) != 1 || string(result.resp.Choices[0].FinishReason) != strings.TrimPrefix(kind, "reasoning_") {
					t.Fatalf("lost provider completion: %+v", result)
				}
			} else if result.err == nil || len(result.resp.Choices) != 0 || result.interruptedReasoning != "private fragment" {
				t.Fatalf("incomplete work survived or continuation was lost: %+v", result)
			}
		})
	}
}
