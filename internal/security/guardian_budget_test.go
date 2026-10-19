package security

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"aurago/internal/budget"
	"aurago/internal/config"
	"github.com/sashabaranov/go-openai"
)

func TestGuardianBudgetExhaustionNeverUsesPermissiveFailSafe(t *testing.T) {
	cfg := &config.Config{}
	cfg.Budget.Enabled = true
	cfg.Budget.DailyLimitUSD = 1
	cfg.Budget.Enforcement = "full"
	cfg.LLMGuardian.FailSafe = "allow"
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	tracker := budget.NewTracker(cfg, logger, t.TempDir())
	defer tracker.Flush()
	tracker.RecordCost(2)
	// Any unexpected provider attempt would fail; an allow fail-safe must never
	// turn the budget refusal into permission to run the original operation.
	clientCfg := openai.DefaultConfig("fixture")
	clientCfg.BaseURL = "http://127.0.0.1:1/v1"
	g := &LLMGuardian{cfg: cfg, client: openai.NewClientWithConfig(clientCfg), model: "fixture", logger: logger, Metrics: &GuardianMetrics{}}
	ctx := budget.WithTracking(context.Background(), tracker, "guardian")
	if result := g.Evaluate(ctx, GuardianCheck{Operation: "execute_shell"}); result.Decision != DecisionBlock {
		t.Fatalf("tool scan budget bypass: %+v", result)
	}
	if result := g.EvaluateClarification(ctx, GuardianCheck{Operation: "execute_shell"}); result.Decision != DecisionBlock {
		t.Fatalf("clarification budget bypass: %+v", result)
	}
	result, err := g.EvaluateContentStrict(ctx, "webhook", "ordinary request")
	if err == nil || result.Decision == DecisionAllow {
		t.Fatalf("content scan budget bypass: %+v %v", result, err)
	}
}
