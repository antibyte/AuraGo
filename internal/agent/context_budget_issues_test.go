package agent

import (
	"testing"

	"aurago/internal/config"
	"aurago/internal/planner"
)

func TestContextBudgetIssueRequiresRepeatAndResolvesOnRouteSuccess(t *testing.T) {
	db := newPlannerTestDB(t)
	t.Cleanup(func() { _ = db.Close() })
	cfg := &config.Config{}
	cfg.LLM.Provider = "agnesai"
	cfg.LLM.ProviderType = "agnes"
	cfg.LLM.Model = "agnes-2.5-flash"
	runCfg := RunConfig{Config: cfg, PlannerDB: db}
	err := &ContextBudgetExceededError{
		Model: "agnes-2.5-flash", RequiredInput: 500000,
		CompletionReserve: 65536, SafetyMargin: 256, ContextWindow: 512000,
	}

	recordContextBudgetFailure(runCfg, cfg.LLM.Model, err)
	page, listErr := planner.ListOperationalIssues(db, planner.OperationalIssueListFilter{Status: "active", Source: "llm"})
	if listErr != nil || page.Total != 0 {
		t.Fatalf("first failure page = %#v, err:%v", page, listErr)
	}
	recordContextBudgetFailure(runCfg, cfg.LLM.Model, err)
	page, listErr = planner.ListOperationalIssues(db, planner.OperationalIssueListFilter{Status: "active", Source: "llm"})
	if listErr != nil || page.Total != 1 {
		t.Fatalf("repeated failure page = %#v, err:%v", page, listErr)
	}

	resolveContextBudgetFailure(runCfg, cfg.LLM.Model)
	page, listErr = planner.ListOperationalIssues(db, planner.OperationalIssueListFilter{Status: "active", Source: "llm"})
	if listErr != nil || page.Total != 0 {
		t.Fatalf("resolved failure page = %#v, err:%v", page, listErr)
	}
}
