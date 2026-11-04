package agent

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"aurago/internal/budget"
	"aurago/internal/config"

	"github.com/sashabaranov/go-openai"
)

func TestMinimalLoopBudgetAdmission(t *testing.T) {
	for _, tc := range []struct {
		name, mode, category                 string
		disabled, missing, inherited, stream bool
		want                                 error
	}{
		{name: "full chat", mode: "full", category: "chat", want: budget.ErrCompletionBudgetBlocked},
		{name: "full stream", mode: "full", category: "writer", stream: true, want: budget.ErrCompletionBudgetBlocked},
		{name: "partial auxiliary", mode: "partial", category: "meshcore", want: budget.ErrCompletionBudgetBlocked},
		{name: "partial chat", mode: "partial", category: "chat"},
		{name: "partial default chat", mode: "partial"},
		{name: "warning", mode: "warn", category: "writer"},
		{name: "disabled", mode: "full", disabled: true, missing: true},
		{name: "missing tracker", mode: "full", missing: true, want: budget.ErrCompletionBudgetUnavailable},
		{name: "context tracker", mode: "full", inherited: true, want: budget.ErrCompletionBudgetBlocked},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &config.Config{}
			cfg.Agent.ContextWindow = 6000
			cfg.Budget.Enabled, cfg.Budget.Enforcement, cfg.Budget.DailyLimitUSD = !tc.disabled, tc.mode, 1
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			tracker := budget.NewTracker(cfg, logger, t.TempDir())
			t.Cleanup(tracker.Flush)
			tracker.RecordCost(2)
			dc := &DispatchContext{Cfg: cfg, BudgetTracker: tracker}
			ctx := context.Background()
			if tc.inherited {
				ctx = budget.WithTracking(ctx, tracker, "parent")
			}
			if tc.missing || tc.inherited {
				dc.BudgetTracker = nil
			}
			client := &minimalLoopRouteClient{routes: minimalLoopTestRoutes()}
			_, _, err := ExecuteMinimalLoop(ctx, client, "primary-model", "Answer.", "Fixture", nil, dc, nil, logger, &MinimalLoopOptions{MaxToolRounds: 0, BudgetCategory: tc.category, StreamText: tc.stream})
			if !errors.Is(err, tc.want) || (tc.want != nil && len(client.requests) != 0) || (tc.want == nil && len(client.requests) != 1) {
				t.Fatalf("calls=%d err=%v, want %v", len(client.requests), err, tc.want)
			}
		})
	}
}

func TestMinimalLoopBudgetStopsContinuationCorrectionAndSummary(t *testing.T) {
	for _, tc := range []struct {
		name       string
		blockOn    int
		correction bool
	}{
		{"next tool round", 1, false},
		{"final summary", 2, false},
		{"format correction", 1, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &config.Config{}
			cfg.Agent.ContextWindow = 6000
			cfg.Budget.Enabled, cfg.Budget.Enforcement, cfg.Budget.DailyLimitUSD = true, "full", 1
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			tracker := budget.NewTracker(cfg, logger, t.TempDir())
			t.Cleanup(tracker.Flush)
			client := &minimalLoopRouteClient{routes: minimalLoopTestRoutes(), respond: func(_ openai.ChatCompletionRequest, n int) (openai.ChatCompletionResponse, error) {
				if n == tc.blockOn {
					tracker.RecordCost(2)
				}
				choice := openai.ChatCompletionChoice{FinishReason: openai.FinishReasonToolCalls, Message: openai.ChatCompletionMessage{Role: openai.ChatMessageRoleAssistant,
					ToolCalls: []openai.ToolCall{{ID: "fixture", Type: openai.ToolTypeFunction, Function: openai.FunctionCall{Name: "test_tool", Arguments: `{}`}}}}}
				if tc.correction {
					choice.FinishReason, choice.Message.ToolCalls, choice.Message.Content = openai.FinishReasonStop, nil, `<tool_call name="test_tool">{}</tool_call>`
				}
				return openai.ChatCompletionResponse{Model: "primary-model", Usage: openai.Usage{PromptTokens: 5, CompletionTokens: 2, TotalTokens: 7}, Choices: []openai.ChatCompletionChoice{choice}}, nil
			}}
			tool := openai.Tool{Type: openai.ToolTypeFunction, Function: &openai.FunctionDefinition{Name: "test_tool", Parameters: map[string]any{"type": "object"}}}
			result, _, err := ExecuteMinimalLoop(context.Background(), client, "primary-model", "Use native tools.", "Fixture", []openai.Tool{tool}, &DispatchContext{Cfg: cfg, BudgetTracker: tracker}, nil, logger, &MinimalLoopOptions{MaxToolRounds: 1, BudgetCategory: "looper"})
			if !errors.Is(err, budget.ErrCompletionBudgetBlocked) || len(client.requests) != tc.blockOn {
				t.Fatalf("calls=%d err=%v", len(client.requests), err)
			}
			usage := tracker.GetStatus().Models["primary-model"]
			if usage.Calls != tc.blockOn || result.TotalTokens != 7*tc.blockOn {
				t.Fatalf("usage=%+v result=%+v; each response must be booked once", usage, result)
			}
		})
	}
}
