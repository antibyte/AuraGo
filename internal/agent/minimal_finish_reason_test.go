package agent

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"testing"

	"aurago/internal/budget"
	"aurago/internal/config"

	"github.com/sashabaranov/go-openai"
)

func TestMinimalIncompleteResponsesNeverExecuteAndChargeOnce(t *testing.T) {
	for _, tc := range []struct {
		stream, tool bool
		finish       openai.FinishReason
	}{
		{false, true, ""},
		{false, true, openai.FinishReasonLength},
		{false, true, openai.FinishReasonContentFilter},
		{true, true, openai.FinishReasonLength},
		{true, false, openai.FinishReasonLength},
	} {
		t.Run(fmt.Sprintf("stream=%t/tool=%t/finish=%s", tc.stream, tc.tool, tc.finish), func(t *testing.T) {
			cfg := &config.Config{}
			cfg.Agent.ContextWindow = 6000
			cfg.Budget.Enabled, cfg.Budget.DailyLimitUSD = true, 10
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			tracker := budget.NewTracker(cfg, logger, t.TempDir())
			t.Cleanup(tracker.Flush)
			usage := openai.Usage{PromptTokens: 10, CompletionTokens: 3, TotalTokens: 13}
			message := openai.ChatCompletionMessage{Role: openai.ChatMessageRoleAssistant, Content: "truncated output"}
			delta := openai.ChatCompletionStreamChoiceDelta{Content: message.Content}
			if tc.tool {
				message.ToolCalls = []openai.ToolCall{{ID: "must-not-execute", Type: openai.ToolTypeFunction, Function: openai.FunctionCall{Name: "core_memory", Arguments: `{"operation":"list"}`}}}
				delta.ToolCalls = message.ToolCalls
			}
			client := &usageRegressionClient{routes: minimalLoopTestRoutes()}
			client.responses = []openai.ChatCompletionResponse{{Model: "actual-route-model", Usage: usage, Choices: []openai.ChatCompletionChoice{{Message: message, FinishReason: tc.finish}}}}
			client.streams = []*usageRegressionStream{{chunks: []openai.ChatCompletionStreamResponse{{Model: "actual-route-model", Usage: &usage, Choices: []openai.ChatCompletionStreamChoice{{Index: 0, Delta: delta, FinishReason: tc.finish}}}}}}
			tool := openai.Tool{Type: openai.ToolTypeFunction, Function: &openai.FunctionDefinition{Name: "core_memory", Parameters: map[string]any{"type": "object"}}}
			opts := &MinimalLoopOptions{MaxToolRounds: 1}
			if tc.stream {
				opts.MaxToolRounds, opts.StreamText = 0, true
			}
			result, _, err := ExecuteMinimalLoop(context.Background(), client, "primary-model", "Complete the task.", "Current task", []openai.Tool{tool}, &DispatchContext{Cfg: cfg, BudgetTracker: tracker}, nil, logger, opts)
			if err == nil || result.ToolCalls != 0 || result.Response != "" || len(client.requests) != 1 {
				t.Fatalf("incomplete response applied or replayed: result=%+v error=%v requests=%d", result, err, len(client.requests))
			}
			// Tool-free source adapters need the finish marker to classify a
			// bounded new generation; it must never make this output executable.
			// Unexpected streamed calls are rejected before reading their marker.
			if (!tc.stream || !tc.tool) && result.FinishReason != tc.finish {
				t.Fatalf("incomplete response lost its diagnostic finish marker: got %q want %q", result.FinishReason, tc.finish)
			}
			booked := tracker.GetStatus().Models["actual-route-model"]
			if booked.Calls != 1 || booked.InputTokens != 10 || booked.OutputTokens != 3 {
				t.Fatalf("incomplete physical response was not charged exactly once: %+v", booked)
			}
		})
	}
}

func TestMinimalUnknownPanicAndApprovalStopBeforeSiblingsOrReplay(t *testing.T) {
	for _, kind := range []string{"panic", "unknown", "approval", "cancel"} {
		t.Run(kind, func(t *testing.T) {
			cfg := &config.Config{}
			cfg.Agent.ContextWindow = 6000
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			calls := 0
			dc := &DispatchContext{Cfg: cfg, Logger: logger, ExecutionHooks: &ExecutionHooks{HandleTool: func(context.Context, ToolCall) (string, bool) {
				calls++
				switch kind {
				case "panic":
					panic("private panic payload")
				case "unknown":
					return `{"status":"unknown","code":"outcome_unknown"}`, true
				case "approval":
					return `{"status":"deferred","code":"approval_required"}`, true
				default:
					cancel()
					return `{"status":"success"}`, true
				}
			}}}
			client := &minimalLoopRouteClient{routes: minimalLoopTestRoutes(), respond: func(openai.ChatCompletionRequest, int) (openai.ChatCompletionResponse, error) {
				return openai.ChatCompletionResponse{Choices: []openai.ChatCompletionChoice{{FinishReason: openai.FinishReasonToolCalls, Message: openai.ChatCompletionMessage{Role: openai.ChatMessageRoleAssistant, ToolCalls: []openai.ToolCall{
					{ID: "primary", Type: openai.ToolTypeFunction, Function: openai.FunctionCall{Name: "core_memory", Arguments: `{"operation":"add","content":"fixture"}`}},
					{ID: "sibling", Type: openai.ToolTypeFunction, Function: openai.FunctionCall{Name: "core_memory", Arguments: `{"operation":"list"}`}},
				}}}}}, nil
			}}
			tool := openai.Tool{Type: openai.ToolTypeFunction, Function: &openai.FunctionDefinition{Name: "core_memory", Parameters: map[string]any{"type": "object"}}}
			result, history, err := ExecuteMinimalLoop(ctx, client, "primary-model", "Complete the task.", "Current task", []openai.Tool{tool}, dc, nil, logger, &MinimalLoopOptions{MaxToolRounds: 1})
			if calls != 1 || result.ToolCalls != 1 || len(client.requests) != 1 {
				t.Fatalf("uncertain action dispatched a sibling or replay: handlers=%d result=%+v requests=%d error=%v", calls, result, len(client.requests), err)
			}
			switch kind {
			case "approval":
				if err != nil || result.Response != "This action is waiting for administrator approval." {
					t.Fatalf("approval boundary did not finish gracefully: %+v %v", result, err)
				}
			case "cancel":
				if !errors.Is(err, context.Canceled) {
					t.Fatalf("cancellation was lost: %v", err)
				}
			default:
				if !errors.Is(err, ErrToolOutcomeUnknown) {
					t.Fatalf("uncertainty boundary was lost: %v", err)
				}
			}
			var toolResults []openai.ChatCompletionMessage
			for _, message := range history {
				if message.Role == openai.ChatMessageRoleTool {
					toolResults = append(toolResults, message)
				}
				if strings.Contains(message.Content, "private panic payload") {
					t.Fatal("panic payload escaped into history")
				}
			}
			if len(toolResults) != 2 || toolResults[0].ToolCallID != "primary" || toolResults[1].ToolCallID != "sibling" || !strings.Contains(toolResults[1].Content, `"code":"tool_batch_stopped"`) {
				t.Fatalf("declared native results were not closed in order: %+v", toolResults)
			}
		})
	}
}
