package agent

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"log/slog"
	"path/filepath"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/memory"
	"aurago/internal/planner"
)

func maintenanceRegressionStores(t *testing.T) (*memory.SQLiteMemory, *slog.Logger) {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	stm, err := memory.NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = stm.Close() })
	if err := stm.InitJournalTables(); err != nil {
		t.Fatal(err)
	}
	return stm, logger
}

func TestMaintenanceDisabledPhasesPrecedeBudgetAndBacklog(t *testing.T) {
	stm, logger := maintenanceRegressionStores(t)
	for _, role := range []string{"user", "assistant", "user"} {
		if _, err := stm.InsertMessage("direct", role, "A remembered fact", false, false); err != nil {
			t.Fatal(err)
		}
	}
	if err := stm.DeleteOldMessages("direct", 1); err != nil {
		t.Fatal(err)
	}
	db, err := planner.InitDB(filepath.Join(t.TempDir(), "planner.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	reconcileMaintenancePhaseIssues(db, []memory.MaintenancePhaseResult{{Name: "entity_extraction", Status: "partial"}}, time.Now(), logger)
	cfg := &config.Config{}
	cfg.CircuitBreaker.MaintenanceTimeoutMinutes = 1
	cfg.Directories.PromptsDir = t.TempDir()
	runMaintenanceTask(context.Background(), cfg, logger, nil, nil, nil, nil, nil, &hierarchyVectorDB{}, stm, nil, nil, nil, nil, db, nil, nil, nil, nil)
	run, err := stm.GetLatestMaintenanceRun()
	if err != nil || run == nil {
		t.Fatalf("run=%v err=%v", run, err)
	}
	phases := map[string]memory.MaintenancePhaseResult{}
	for _, phase := range run.PhaseResults.Phases {
		phases[phase.Name] = phase
	}
	for _, name := range []string{"consolidation", "entity_extraction", "skill_quality", "daily_summary"} {
		if phases[name].Status != "skipped" || phases[name].Deferred != 0 {
			t.Fatalf("disabled %s: %+v", name, phases[name])
		}
	}
	if run.PhaseResults.ConsolidationBacklog != 0 {
		t.Fatal("disabled backlog was inspected")
	}
	if _, ok := phases["agent_loop"]; ok {
		t.Fatal("agent loop began before prompt loaded")
	}
	if phases["prompt_load"].Status != "failed" {
		t.Fatalf("prompt phase: %+v", phases["prompt_load"])
	}
	var status string
	if err := db.QueryRow(`SELECT status FROM operational_issues WHERE fingerprint = ?`, "maintenance|phase|entity_extraction").Scan(&status); err != nil || status != "open" {
		t.Fatalf("skipped issue = %s, %v", status, err)
	}
}

func TestMaintenanceCheatsheetAndRetentionErrorsAreReported(t *testing.T) {
	stm, logger := maintenanceRegressionStores(t)
	closed, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	_ = closed.Close()
	cfg := &config.Config{}
	cfg.Directories.PromptsDir = t.TempDir()
	runMaintenanceTask(context.Background(), cfg, logger, nil, nil, nil, nil, nil, nil, stm, nil, nil, nil, nil, nil, closed, nil, nil, nil)
	run, err := stm.GetLatestMaintenanceRun()
	if err != nil {
		t.Fatal(err)
	}
	for _, phase := range run.PhaseResults.Phases {
		if phase.Name == "profile_cleanup" && phase.Status != "partial" {
			t.Fatalf("profile phase swallowed database error: %+v", phase)
		}
	}
	_ = stm.Close()
	cfg.Consolidation.StmRetentionMessages = 1
	if err := RunSTMPRetentionMaintenance(cfg, logger, stm); err == nil {
		t.Fatal("retention failure was swallowed")
	}
}

func TestMaintenanceHelperAcknowledgesIndependentPersistence(t *testing.T) {
	stm, logger := maintenanceRegressionStores(t)
	kg, err := memory.NewKnowledgeGraph(":memory:", "", logger)
	if err != nil {
		t.Fatal(err)
	}
	defer kg.Close()
	result, err := parseHelperMaintenanceBatchResult(`{"daily_summary":"Stored summary","kg_extraction":"invalid"}`)
	if err != nil {
		t.Fatal(err)
	}
	outcome := persistMaintenanceSummaryAndKG(stm, kg, logger, "2026-09-21", nil, result, 100)
	if !outcome.SummaryStored || outcome.KGStored || outcome.KGErr == nil {
		t.Fatalf("outcome=%+v", outcome)
	}
	result, err = parseHelperMaintenanceBatchResult(`{"daily_summary":"","kg_extraction":{"nodes":[],"edges":[]}}`)
	if err != nil {
		t.Fatal(err)
	}
	outcome = persistMaintenanceSummaryAndKG(stm, kg, logger, "2026-09-20", nil, result, 100)
	if outcome.SummaryStored || !outcome.KGStored || outcome.SummaryErr == nil {
		t.Fatalf("outcome=%+v", outcome)
	}
	_ = stm.Close()
	result.DailySummary = "cannot persist"
	outcome = persistMaintenanceSummaryAndKG(stm, kg, logger, "2026-09-19", nil, result, 100)
	if outcome.SummaryStored || !outcome.KGStored || outcome.SummaryErr == nil {
		t.Fatalf("persistence outcome=%+v", outcome)
	}
}

func TestMaintenanceUnfinishedPhaseCannotResolveIssue(t *testing.T) {
	ledger := newMaintenanceRunLedger()
	ledger.beginPhase("first")
	ledger.beginPhase("second")
	if ledger.results().Phases[0].Status != "partial" {
		t.Fatal("unfinished phase was completed")
	}
}

func TestMaintenanceCombinedFailureSurvivesUnreachedPhase(t *testing.T) {
	ledger := newMaintenanceRunLedger()
	ledger.beginPhase("daily_summary")
	ledger.finishPhase("daily_summary", false)
	ledger.recordNamedError("entity_extraction", "batched_kg_persist", errors.New("write failed"))
	if ledger.status() != "partial" || len(ledger.results().Phases) != 2 || ledger.results().Phases[1].Status != "partial" {
		t.Fatalf("combined failure lost: %+v", ledger.results())
	}
}

func TestMaintenanceLedgerPersistenceFailureRecordsOriginalFailures(t *testing.T) {
	stm, logger := maintenanceRegressionStores(t)
	db, err := planner.InitDB(filepath.Join(t.TempDir(), "planner.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	_ = stm.Close()
	ledger := newMaintenanceRunLedger()
	ledger.beginPhase("skill_quality")
	ledger.recordError("daemon_restart", errors.New("restart failed"))
	ledger.finishPhase("skill_quality", false)
	completeMaintenanceRun(&config.Config{}, logger, stm, db, time.Now(), ledger)
	for _, phase := range []string{"skill_quality", "ledger_persist"} {
		var status string
		if err := db.QueryRow(`SELECT status FROM operational_issues WHERE fingerprint = ?`, "maintenance|phase|"+phase).Scan(&status); err != nil || status != "open" {
			t.Fatalf("%s issue=%s err=%v", phase, status, err)
		}
	}
}

func TestMaintenanceBriefingRemainsBoundToPersistedRunAndIdempotent(t *testing.T) {
	stm, logger := maintenanceRegressionStores(t)
	cfg := &config.Config{}
	startedAt := time.Date(2026, 9, 22, 4, 0, 0, 123, time.UTC)
	for range 2 {
		ledger := newMaintenanceRunLedger()
		ledger.beginPhase("daily_summary")
		ledger.applyOutcome("daily_summary", maintenancePhaseOutcome{Processed: 1})
		completeMaintenanceRun(cfg, logger, stm, nil, startedAt, ledger)
	}
	notes, err := stm.GetUnreadSystemNotifications()
	if err != nil || len(notes) != 1 {
		t.Fatalf("briefings=%v err=%v", notes, err)
	}
	if notes[0].Type != "morning_briefing" || notes[0].SourceID != "maintenance:"+startedAt.Format(time.RFC3339Nano) {
		t.Fatalf("briefing lost run identity: %+v", notes[0])
	}
	run, err := stm.GetLatestMaintenanceRun()
	if err != nil || run == nil || run.Status != "completed" {
		t.Fatalf("persisted run=%v err=%v", run, err)
	}
}
