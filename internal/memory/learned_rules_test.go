package memory

import (
	"io"
	"log/slog"
	"testing"
)

func setupLearnedRulesTest(t *testing.T) *SQLiteMemory {
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
	stm, err := NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	if err := stm.InitLearnedRulesTable(); err != nil {
		t.Fatalf("InitLearnedRulesTable: %v", err)
	}
	t.Cleanup(func() { stm.Close() })
	return stm
}

func TestInitLearnedRulesTable_Idempotent(t *testing.T) {
	stm := setupLearnedRulesTest(t)
	// Second init should not error
	if err := stm.InitLearnedRulesTable(); err != nil {
		t.Fatalf("second init failed: %v", err)
	}
}

func TestUpsertLearnedRule_InsertAndUpdate(t *testing.T) {
	stm := setupLearnedRulesTest(t)

	rule := &LearnedRule{
		ToolName:   "docker_run",
		Pattern:    "port already in use",
		Rule:       "check ports first",
		Confidence: 0.5,
		Hits:       1,
		Active:     true,
	}
	if err := stm.UpsertLearnedRule(rule); err != nil {
		t.Fatalf("upsert failed: %v", err)
	}

	count, err := stm.GetLearnedRulesCount()
	if err != nil {
		t.Fatalf("count failed: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 rule, got %d", count)
	}

	// Upsert again should update confidence and hits
	rule2 := &LearnedRule{
		ToolName:   "docker_run",
		Pattern:    "port already in use",
		Rule:       "updated rule text",
		Confidence: 0.6,
		Hits:       1,
		Active:     true,
	}
	if err := stm.UpsertLearnedRule(rule2); err != nil {
		t.Fatalf("second upsert failed: %v", err)
	}

	rules, err := stm.GetActiveLearnedRules(5)
	if err != nil {
		t.Fatalf("get active failed: %v", err)
	}
	if len(rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(rules))
	}
	if rules[0].Confidence <= 0.5 {
		t.Errorf("expected confidence bumped, got %f", rules[0].Confidence)
	}
	if rules[0].Hits != 2 {
		t.Errorf("expected hits=2, got %d", rules[0].Hits)
	}
}

func TestGetActiveLearnedRules_Ordering(t *testing.T) {
	stm := setupLearnedRulesTest(t)

	rules := []*LearnedRule{
		{ToolName: "a", Pattern: "p1", Rule: "r1", Confidence: 0.8, Active: true},
		{ToolName: "b", Pattern: "p2", Rule: "r2", Confidence: 0.9, Active: true},
		{ToolName: "c", Pattern: "p3", Rule: "r3", Confidence: 0.7, Active: true},
	}
	for _, r := range rules {
		if err := stm.UpsertLearnedRule(r); err != nil {
			t.Fatalf("upsert failed: %v", err)
		}
	}

	active, err := stm.GetActiveLearnedRules(5)
	if err != nil {
		t.Fatalf("get active failed: %v", err)
	}
	if len(active) != 3 {
		t.Fatalf("expected 3 rules, got %d", len(active))
	}
	// Highest confidence first
	if active[0].ToolName != "b" {
		t.Errorf("expected first rule 'b' (conf 0.9), got %s (conf %f)", active[0].ToolName, active[0].Confidence)
	}
}

func TestGetLearnedRulesForTools_Filtering(t *testing.T) {
	stm := setupLearnedRulesTest(t)

	rules := []*LearnedRule{
		{ToolName: "docker_run", Pattern: "p1", Rule: "r1", Confidence: 0.8, Active: true},
		{ToolName: "execute_shell", Pattern: "p2", Rule: "r2", Confidence: 0.9, Active: true},
	}
	for _, r := range rules {
		if err := stm.UpsertLearnedRule(r); err != nil {
			t.Fatalf("upsert failed: %v", err)
		}
	}

	filtered, err := stm.GetLearnedRulesForTools([]string{"docker_run"}, 5)
	if err != nil {
		t.Fatalf("get for tools failed: %v", err)
	}
	if len(filtered) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(filtered))
	}
	if filtered[0].ToolName != "docker_run" {
		t.Errorf("expected docker_run, got %s", filtered[0].ToolName)
	}
}

func TestRecordLearnedRuleHitAndMiss(t *testing.T) {
	stm := setupLearnedRulesTest(t)

	rule := &LearnedRule{ToolName: "t", Pattern: "p", Rule: "r", Confidence: 0.5, Hits: 1, Active: true}
	if err := stm.UpsertLearnedRule(rule); err != nil {
		t.Fatalf("upsert failed: %v", err)
	}

	// Get the ID
	rules, err := stm.GetActiveLearnedRules(1)
	if err != nil {
		t.Fatalf("get active failed: %v", err)
	}
	if len(rules) == 0 {
		t.Fatal("expected 1 rule")
	}
	id := rules[0].ID
	if id == 0 {
		t.Fatal("expected non-zero rule id")
	}

	if err := stm.RecordLearnedRuleHit(id); err != nil {
		t.Fatalf("hit failed: %v", err)
	}

	// Verify hit took effect before calling miss
	rules, err = stm.GetActiveLearnedRules(1)
	if err != nil {
		t.Fatalf("get active after hit failed: %v", err)
	}
	if rules[0].Hits != 2 {
		t.Fatalf("expected hits=2 after hit, got %d", rules[0].Hits)
	}

	if err := stm.RecordLearnedRuleMiss(id); err != nil {
		t.Fatalf("miss failed: %v", err)
	}

	rules, err = stm.GetActiveLearnedRules(1)
	if err != nil {
		t.Fatalf("get active after hit/miss failed: %v", err)
	}
	if len(rules) == 0 {
		t.Fatal("expected 1 rule after hit/miss")
	}
	if rules[0].Hits != 2 {
		t.Errorf("expected hits=2, got %d", rules[0].Hits)
	}
	if rules[0].Misses != 1 {
		t.Errorf("expected misses=1, got %d", rules[0].Misses)
	}
}

func TestCleanOldLearnedRules(t *testing.T) {
	stm := setupLearnedRulesTest(t)

	rule := &LearnedRule{ToolName: "t", Pattern: "p", Rule: "r", Confidence: 0.05, Active: true}
	if err := stm.UpsertLearnedRule(rule); err != nil {
		t.Fatalf("upsert failed: %v", err)
	}

	deleted, err := stm.CleanOldLearnedRules(0.1, 0)
	if err != nil {
		t.Fatalf("clean failed: %v", err)
	}
	if deleted != 1 {
		t.Errorf("expected 1 deleted, got %d", deleted)
	}

	count, _ := stm.GetLearnedRulesCount()
	if count != 0 {
		t.Errorf("expected 0 rules after cleanup, got %d", count)
	}
}

func TestUpsertLearnedRule_NilOrEmpty(t *testing.T) {
	stm := setupLearnedRulesTest(t)
	if err := stm.UpsertLearnedRule(nil); err != nil {
		t.Errorf("expected nil rule to be a no-op, got error: %v", err)
	}
	if err := stm.UpsertLearnedRule(&LearnedRule{}); err != nil {
		t.Errorf("expected empty rule to be a no-op, got error: %v", err)
	}
}

func TestGetErrorCountInSession(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
	stm, err := NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	defer stm.Close()
	if err := stm.InitErrorLearningTable(); err != nil {
		t.Fatalf("InitErrorLearningTable: %v", err)
	}

	// Record an error twice
	_ = stm.RecordError("docker_run", "port already in use: 8080")
	_ = stm.RecordError("docker_run", "port already in use: 8080")

	count, err := stm.GetErrorCountInSession("docker_run", "port already in use: 8080")
	if err != nil {
		t.Fatalf("GetErrorCountInSession failed: %v", err)
	}
	if count != 2 {
		t.Errorf("expected count=2, got %d", count)
	}

	// Non-existent error
	count, err = stm.GetErrorCountInSession("docker_run", "unknown error")
	if err != nil {
		t.Fatalf("GetErrorCountInSession failed for unknown: %v", err)
	}
	if count != 0 {
		t.Errorf("expected count=0 for unknown, got %d", count)
	}
}
