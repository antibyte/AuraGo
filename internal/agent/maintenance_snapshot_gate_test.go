package agent

import (
	"bytes"
	"context"
	"database/sql"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/memory"

	"github.com/sashabaranov/go-openai"
)

func snapshotGateConfig(t *testing.T, createSources bool) *config.Config {
	t.Helper()
	cfg := &config.Config{}
	cfg.Directories.DataDir = t.TempDir()
	cfg.Directories.VectorDBDir = filepath.Join(cfg.Directories.DataDir, "vectors")
	cfg.SQLite.ShortTermPath = filepath.Join(cfg.Directories.DataDir, "stm.db")
	cfg.SQLite.KnowledgeGraphPath = filepath.Join(cfg.Directories.DataDir, "kg.db")
	if createSources {
		if err := os.MkdirAll(cfg.Directories.VectorDBDir, 0700); err != nil {
			t.Fatal(err)
		}
		for _, path := range []string{cfg.SQLite.ShortTermPath, cfg.SQLite.KnowledgeGraphPath, config.DocumentCorpusPath(cfg)} {
			db, err := sql.Open("sqlite", path)
			if err != nil {
				t.Fatal(err)
			}
			_, err = db.Exec(`CREATE TABLE fixture(value TEXT)`)
			db.Close()
			if err != nil {
				t.Fatal(err)
			}
		}
	}
	return cfg
}

func TestMaintenanceSnapshotGateBlocksDestructiveStepsOnlyWithoutSnapshot(t *testing.T) {
	now := time.Now()
	if ok, reason := maintenanceSnapshotGate(t.Context(), &config.Config{}, now); !ok {
		t.Fatalf("unconfigured snapshots must not block maintenance: %s", reason)
	}
	if ok, reason := maintenanceSnapshotGate(t.Context(), snapshotGateConfig(t, false), now); ok || !strings.Contains(reason, "failed") {
		t.Fatalf("failed snapshot must block destructive steps: ok=%v reason=%q", ok, reason)
	}
	partial := &config.Config{}
	partial.Directories.DataDir = t.TempDir()
	partial.SQLite.ShortTermPath = filepath.Join(partial.Directories.DataDir, "stm.db")
	if ok, reason := maintenanceSnapshotGate(t.Context(), partial, now); ok || reason != "snapshot_config_incomplete" {
		t.Fatalf("partial store config must block destructive steps: ok=%v reason=%q", ok, reason)
	}
	cfg := snapshotGateConfig(t, true)
	if ok, reason := maintenanceSnapshotGate(t.Context(), cfg, now); !ok {
		t.Fatalf("verified snapshot must allow destructive steps: %s", reason)
	}
	if ok, reason := maintenanceSnapshotGate(t.Context(), cfg, now); !ok {
		t.Fatalf("existing same-day snapshot must allow destructive steps: %s", reason)
	}
}

func TestMaintenanceSnapshotGateDiagnosticOnlyThenResumes(t *testing.T) {
	stm, _ := maintenanceRegressionStores(t)
	for range 5 {
		if _, err := stm.InsertMessage("snapshot-gate", "user", "The backup target is the NAS.", false, false); err != nil {
			t.Fatal(err)
		}
	}
	if err := stm.DeleteOldMessages("snapshot-gate", 1); err != nil {
		t.Fatal(err)
	}
	yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	if _, err := stm.InsertJournalEntry(memory.JournalEntry{Date: yesterday, EntryType: "activity", Title: "Backup", Content: "Configured NAS backups."}); err != nil {
		t.Fatal(err)
	}
	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, nil))
	cfg := &config.Config{}
	cfg.Directories.DataDir = t.TempDir()
	cfg.SQLite.ShortTermPath = filepath.Join(cfg.Directories.DataDir, "stm.db")
	cfg.Tools.Memory.Enabled = true
	cfg.Tools.Journal.Enabled = true
	cfg.Journal.DailySummary = true
	cfg.Consolidation.Enabled = true
	cfg.Consolidation.CatchupMinutes = 1
	cfg.Consolidation.MaxBatchMessages = 10
	cfg.LLM.Model = "test-model"
	cfg.CircuitBreaker.MaintenanceTimeoutMinutes = 5
	cfg.Directories.PromptsDir = t.TempDir()
	if err := os.WriteFile(filepath.Join(cfg.Directories.PromptsDir, "maintenance.md"), []byte("maintenance"), 0600); err != nil {
		t.Fatal(err)
	}
	client := maintenanceCompletionTestClient{complete: func(ctx context.Context, req openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
		return skillQualityTestClient{response: "Yesterday: configured NAS backups."}.CreateChatCompletion(ctx, req)
	}}

	// One active persistent store with missing peers must skip every mutating phase.
	runMaintenanceTask(t.Context(), cfg, logger, client, nil, nil, nil, nil, &hierarchyVectorDB{}, stm, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	first, err := stm.GetLatestMaintenanceRun()
	if err != nil || first == nil {
		t.Fatalf("blocked run=%+v err=%v", first, err)
	}
	firstPhases := maintenancePhaseMap(first.PhaseResults.Phases)
	if firstPhases["snapshot_gate"].Status != "partial" || len(firstPhases["snapshot_gate"].ErrorCodes) == 0 {
		t.Fatalf("snapshot failure not recorded: %+v", firstPhases["snapshot_gate"])
	}
	for _, name := range []string{
		"short_term_cleanup", "profile_cleanup", "memory_hygiene", "consolidation",
		"daily_summary", "entity_extraction", "activity_rollup", "weekly_reflection",
		"knowledge_graph", "memory_optimization", "skill_quality", "prompt_load",
		"agent_loop", "consolidation_catchup",
	} {
		if phase := firstPhases[name]; phase.Status != "skipped" {
			t.Fatalf("phase %q ran without a snapshot: %+v", name, phase)
		}
	}
	if len(first.PhaseResults.Errors) == 0 || strings.Contains(strings.Join(first.PhaseResults.Errors, " "), cfg.Directories.DataDir) {
		t.Fatalf("gate diagnostics are missing or expose paths: %v", first.PhaseResults.Errors)
	}
	if logs.String() == "" || strings.Contains(logs.String(), cfg.Directories.DataDir) || !strings.Contains(logs.String(), "configured_store_count=1") {
		t.Fatalf("diagnostics are unbounded or lack store metadata: %s", logs.String())
	}
	if summary, err := stm.GetDailySummary(yesterday); err != nil || summary != nil {
		t.Fatalf("blocked run wrote a summary: %+v %v", summary, err)
	}
	candidates, err := stm.GetConsolidationCandidates(20, 3)
	if err != nil || len(candidates) == 0 {
		t.Fatalf("blocked run lost consolidation work: %+v %v", candidates, err)
	}
	for _, candidate := range candidates {
		if candidate.ConsolidationStatus != "pending" || candidate.ConsolidationRetries != 0 {
			t.Fatalf("blocked run claimed consolidation work: %+v", candidate)
		}
	}

	// Repair the store configuration and verify a later run can snapshot and work.
	cfg.SQLite.KnowledgeGraphPath = filepath.Join(cfg.Directories.DataDir, "kg.db")
	cfg.Directories.VectorDBDir = filepath.Join(cfg.Directories.DataDir, "vectors")
	if err := os.MkdirAll(cfg.Directories.VectorDBDir, 0700); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{cfg.SQLite.ShortTermPath, cfg.SQLite.KnowledgeGraphPath, config.DocumentCorpusPath(cfg)} {
		db, err := sql.Open("sqlite", path)
		if err != nil {
			t.Fatal(err)
		}
		_, err = db.Exec(`CREATE TABLE fixture(value TEXT)`)
		_ = db.Close()
		if err != nil {
			t.Fatal(err)
		}
	}
	if ok, reason := maintenanceSnapshotGate(t.Context(), cfg, time.Now()); !ok {
		t.Fatalf("repaired configuration still blocked: %s", reason)
	}
	if _, err := stm.InsertJournalEntry(memory.JournalEntry{Date: yesterday, EntryType: "activity", Title: "Follow-up", Content: "Finished NAS setup."}); err != nil {
		t.Fatal(err)
	}
	runMaintenanceTask(t.Context(), cfg, logger, client, nil, nil, nil, nil, nil, stm, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	second, err := stm.GetLatestMaintenanceRun()
	if err != nil || second == nil {
		t.Fatalf("follow-up run=%+v err=%v", second, err)
	}
	secondPhases := maintenancePhaseMap(second.PhaseResults.Phases)
	if secondPhases["daily_summary"].Status == "skipped" {
		t.Fatalf("repaired run did not resume maintenance: %+v", secondPhases["daily_summary"])
	}
	if summary, err := stm.GetDailySummary(yesterday); err != nil || summary == nil {
		t.Fatalf("follow-up run did not write the expected summary: %+v %v; status=%s phases=%+v errors=%v", summary, err, second.Status, second.PhaseResults.Phases, second.PhaseResults.Errors)
	}
}

func maintenancePhaseMap(phases []memory.MaintenancePhaseResult) map[string]memory.MaintenancePhaseResult {
	result := make(map[string]memory.MaintenancePhaseResult, len(phases))
	for _, phase := range phases {
		result[phase.Name] = phase
	}
	return result
}
