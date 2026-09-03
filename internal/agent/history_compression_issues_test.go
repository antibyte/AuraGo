package agent

import (
	"testing"

	"aurago/internal/planner"
)

func TestHistoryCompressionIssueRequiresRepeatAndResolves(t *testing.T) {
	db := newPlannerTestDB(t)
	t.Cleanup(func() { _ = db.Close() })
	runCfg := RunConfig{PlannerDB: db, SessionID: "summary-test"}
	recordHistoryCompressionFailure(runCfg)
	page, err := planner.ListOperationalIssues(db, planner.OperationalIssueListFilter{Status: "active", Source: "llm"})
	if err != nil || page.Total != 0 {
		t.Fatalf("first failure page=%#v err=%v", page, err)
	}
	recordHistoryCompressionFailure(runCfg)
	page, err = planner.ListOperationalIssues(db, planner.OperationalIssueListFilter{Status: "active", Source: "llm"})
	if err != nil || page.Total != 1 {
		t.Fatalf("repeated failure page=%#v err=%v", page, err)
	}
	resolveHistoryCompressionFailure(runCfg)
	page, err = planner.ListOperationalIssues(db, planner.OperationalIssueListFilter{Status: "active", Source: "llm"})
	if err != nil || page.Total != 0 {
		t.Fatalf("resolved failure page=%#v err=%v", page, err)
	}
}
