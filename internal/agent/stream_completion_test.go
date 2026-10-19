package agent

import (
	"aurago/internal/budget"
	"aurago/internal/config"
	"aurago/internal/llm"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/sashabaranov/go-openai"
)

func TestInterruptedAnthropicInitialZeroUsageBooksObservedOutput(t *testing.T) {
	for _, minimal := range []bool{false, true} {
		for _, measuredOutput := range []int{0, 3} {
			t.Run(fmt.Sprintf("minimal=%t/output=%d", minimal, measuredOutput), func(t *testing.T) {
				provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "text/event-stream")
					fmt.Fprintf(w, "event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"id\":\"fixture\",\"model\":\"claude-stream-budget-fixture\",\"usage\":{\"input_tokens\":10,\"output_tokens\":%d,\"cache_read_input_tokens\":0,\"cache_creation_input_tokens\":0}}}\n\n", measuredOutput)
					fmt.Fprint(w, "event: content_block_start\ndata: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"text\",\"text\":\"\"}}\n\n")
					fmt.Fprint(w, "event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"Observed partial answer before interruption\"}}\n\n")
					// Connection ends before message_delta and message_stop.
					w.(http.Flusher).Flush()
				}))
				defer provider.Close()
				client := llm.WrapOpenAIClient(llm.NewClientFromProvider("anthropic", provider.URL+"/v1", "fixture"))
				ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
				defer cancel()
				ctx, capture := llm.CaptureUsage(ctx)
				request := openai.ChatCompletionRequest{Model: "claude-stream-budget-fixture", Stream: true, MaxTokens: 2048, Messages: []openai.ChatCompletionMessage{{Role: "user", Content: "Fixture task"}}}
				var input, output, total int
				var err error
				if minimal {
					response, streamErr := minimalLoopStreamText(ctx, client, request)
					err = streamErr
					_, input, output, total, _, _, _ = normalizeResponseUsage(request, response)
				} else {
					retries := 0
					result := handleStreamingResponse(ctx, request, client, false, defaultRecoveryPolicy(), slog.New(slog.NewTextHandler(io.Discard, nil)), NoopBroker{}, AgentTelemetryScope{}, cancel, time.Second, &retries, true)
					err, input, output, total = result.err, result.promptTokens, result.completionTokens, result.totalTokens
				}
				items := capture.Snapshot()
				if err == nil || len(items) != 1 || items[0].OutputTokens == nil || *items[0].OutputTokens != measuredOutput {
					t.Fatalf("fixture lost incomplete stream or explicit initial measurement: err=%v usage=%+v", err, items)
				}
				if input != 10 || output <= 0 || total != input+output || measuredOutput > 0 && output != measuredOutput {
					t.Fatalf("partial usage=%d/%d/%d initial output=%d", input, output, total, measuredOutput)
				}
				cfg := &config.Config{}
				cfg.Budget.Enabled, cfg.Budget.DailyLimitUSD = true, 100
				cfg.Budget.DefaultCost = config.ModelCostRates{InputPerMillion: 1, OutputPerMillion: 1}
				tracker := budget.NewTracker(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), t.TempDir())
				defer tracker.Flush()
				tracker.RecordResponseUsage("chat", "anthropic", request.Model, capture, input, output, true)
				usage := tracker.GetStatus().Models[request.Model]
				if usage.Calls != 1 || usage.InputTokens != 10 || usage.OutputTokens != output || math.Abs(usage.CostUSD-float64(10+output)/1e6) > 1e-12 {
					t.Fatalf("partial provider response not booked once: %+v", usage)
				}
			})
		}
	}
}

func TestStrictStreamCompletionRejectsIncompleteWork(t *testing.T) {
	for _, kind := range []string{"stop", "tool_calls", "reasoning_length", "legacy_length", "length", "missing_finish", "idle", "deadline"} {
		t.Run(kind, func(t *testing.T) {
			provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/event-stream")
				fmt.Fprint(w, "data: {\"choices\":[{\"index\":0,\"delta\":{\"reasoning_content\":\"private fragment\"}}]}\n\n")
				if kind == "legacy_length" {
					fmt.Fprint(w, "data: {\"choices\":[{\"index\":0,\"delta\":{\"function_call\":{\"name\":\"game_maker_file\",\"arguments\":\"{\"}}}]}\n\n")
				} else if kind != "reasoning_length" {
					// Even syntactically valid calls must not run before completion.
					fmt.Fprint(w, "data: {\"choices\":[{\"index\":0,\"delta\":{\"tool_calls\":[{\"index\":0,\"id\":\"write\",\"type\":\"function\",\"function\":{\"name\":\"game_maker_file\",\"arguments\":\"{}\"}}]}}]}\n\n")
				}
				w.(http.Flusher).Flush()
				if kind == "deadline" || kind == "idle" {
					<-r.Context().Done()
					return
				}
				if kind != "missing_finish" {
					finish := strings.TrimPrefix(strings.TrimPrefix(kind, "reasoning_"), "legacy_")
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
			result := handleStreamingResponse(ctx, openai.ChatCompletionRequest{Model: "test", Stream: true}, llm.WrapOpenAIClient(openai.NewClientWithConfig(cfg)), false, defaultRecoveryPolicy(), slog.New(slog.NewTextHandler(io.Discard, nil)), &captureBroker{}, AgentTelemetryScope{}, cancel, idle, &retries, true)
			valid := kind == "stop" || kind == "tool_calls"
			if valid {
				if result.err != nil || len(result.resp.Choices) != 1 || string(result.resp.Choices[0].FinishReason) != strings.TrimPrefix(kind, "reasoning_") {
					t.Fatalf("lost provider completion: %+v", result)
				}
			} else if result.err == nil || len(result.resp.Choices) != 0 || result.interruptedReasoning != "private fragment" {
				t.Fatalf("incomplete work survived or continuation was lost: %+v", result)
			}
			if kind == "reasoning_length" || kind == "length" || kind == "legacy_length" {
				var incomplete *IncompleteCompletionError
				if !errors.As(result.err, &incomplete) || incomplete.FinishReason != openai.FinishReasonLength || incomplete.ReasoningOnly != (kind == "reasoning_length") || incomplete.HasToolCalls != (kind != "reasoning_length") {
					t.Fatalf("lost safe completion diagnostics: %#v, error=%v", incomplete, result.err)
				}
				if strings.Contains(result.err.Error(), "private fragment") {
					t.Fatal("provider reasoning escaped into completion diagnostics")
				}
			}
		})
	}
}

func TestStreamPreservesReportedCacheMissVersusUnknown(t *testing.T) {
	for _, tt := range []struct {
		name, details string
		known         bool
		cached        int
	}{
		{"unknown", "", false, 0},
		{"miss", `,"prompt_tokens_details":{"cached_tokens":0}`, true, 0},
		{"hit", `,"prompt_tokens_details":{"cached_tokens":80}`, true, 80},
	} {
		t.Run(tt.name, func(t *testing.T) {
			provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/event-stream")
				fmt.Fprint(w, "data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"Done\"},\"finish_reason\":\"stop\"}]}\n\n")
				fmt.Fprintf(w, "data: {\"choices\":[],\"usage\":{\"prompt_tokens\":100,\"completion_tokens\":1,\"total_tokens\":101%s}}\n\n", tt.details)
				fmt.Fprint(w, "data: [DONE]\n\n")
			}))
			defer provider.Close()
			cfg := openai.DefaultConfig("test-only")
			cfg.BaseURL = provider.URL
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			retries := 0
			result := handleStreamingResponse(ctx, openai.ChatCompletionRequest{Model: "test", Stream: true}, llm.WrapOpenAIClient(openai.NewClientWithConfig(cfg)), false, defaultRecoveryPolicy(), slog.New(slog.NewTextHandler(io.Discard, nil)), &captureBroker{}, AgentTelemetryScope{}, cancel, time.Second, &retries, true)
			details := result.resp.Usage.PromptTokensDetails
			if result.err != nil || (details != nil) != tt.known || (details != nil && details.CachedTokens != tt.cached) {
				t.Fatalf("cache usage: details=%+v, error=%v", details, result.err)
			}
		})
	}
}
