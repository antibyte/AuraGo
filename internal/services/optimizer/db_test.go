package optimizer

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	promptbuilder "aurago/internal/prompts"

	_ "modernc.org/sqlite"
)

func openOptimizerTestDB(t *testing.T) (*OptimizerDB, string) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "optimizer.db")
	db, err := InitDB(dbPath)
	if err != nil {
		t.Fatalf("InitDB returned error: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
		defaultDB = nil
	})
	return db, dbPath
}

func TestInitDBWithPromptsDirUsesConfiguredManualRoot(t *testing.T) {
	root := t.TempDir()
	manualDir := filepath.Join(root, "custom-prompts", "tools_manuals")
	if err := os.MkdirAll(manualDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(manualDir, "shell.md"), []byte("# shell\nCONFIGURED ROOT MANUAL"), 0o644); err != nil {
		t.Fatal(err)
	}
	db, err := InitDBWithPromptsDir(filepath.Join(root, "optimizer.db"), filepath.Join(root, "custom-prompts"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close(); defaultDB = nil })
	if got := db.loadCanonicalManual("shell"); !strings.Contains(got, "CONFIGURED ROOT MANUAL") {
		t.Fatalf("configured manual root ignored: %q", got)
	}
}

func TestInitDBMigratesLegacyPromptOverrides(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "legacy.db")
	legacy, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open legacy db: %v", err)
	}
	_, err = legacy.Exec(`CREATE TABLE prompt_overrides (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		tool_name TEXT NOT NULL UNIQUE,
		mutated_prompt TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		active BOOLEAN DEFAULT 1
	)`)
	if err != nil {
		t.Fatalf("create legacy table: %v", err)
	}
	_, err = legacy.Exec(`INSERT INTO prompt_overrides (tool_name, mutated_prompt) VALUES ('shell', 'legacy prompt')`)
	if err != nil {
		t.Fatalf("insert legacy row: %v", err)
	}
	if err := legacy.Close(); err != nil {
		t.Fatalf("close legacy db: %v", err)
	}

	db, err := InitDB(dbPath)
	if err != nil {
		t.Fatalf("InitDB returned error: %v", err)
	}
	defer db.Close()
	t.Cleanup(func() { defaultDB = nil })

	for _, column := range []string{"shadow", "original_hash"} {
		var exists bool
		err := db.db.QueryRow(`SELECT count(*) > 0 FROM pragma_table_info('prompt_overrides') WHERE name = ?`, column).Scan(&exists)
		if err != nil || !exists {
			t.Fatalf("expected migrated column %q, exists=%v err=%v", column, exists, err)
		}
	}
	if _, err := db.db.Exec(`INSERT INTO prompt_overrides (tool_name, mutated_prompt, active, shadow) VALUES ('shell', 'active', 1, 0)`); err != nil {
		t.Fatalf("expected duplicate tool_name insert after UNIQUE migration: %v", err)
	}
}

func TestPromptOverrideLookupAndTraceLogging(t *testing.T) {
	db, _ := openOptimizerTestDB(t)

	if err := db.LogToolTrace("shell", false, 2, "v1", "boom", 123); err != nil {
		t.Fatalf("LogToolTrace returned error: %v", err)
	}
	var success bool
	var loops int
	var version, errMsg string
	var execMS int64
	if err := db.db.QueryRow(`SELECT success, recovery_loops, prompt_version, error_message, execution_time_ms FROM tool_traces WHERE tool_name = 'shell'`).Scan(&success, &loops, &version, &errMsg, &execMS); err != nil {
		t.Fatalf("read trace: %v", err)
	}
	if success || loops != 2 || version != "v1" || errMsg != "boom" || execMS != 123 {
		t.Fatalf("unexpected trace row: success=%v loops=%d version=%s err=%s ms=%d", success, loops, version, errMsg, execMS)
	}

	hash := fmt.Sprintf("%x", sha256.Sum256([]byte(db.loadCanonicalManual("shell"))))
	if _, err := db.db.Exec(`INSERT INTO prompt_overrides (tool_name, mutated_prompt, original_hash, active, shadow) VALUES ('shell', 'active prompt', ?, 1, 0)`, hash); err != nil {
		t.Fatalf("insert active override: %v", err)
	}
	overrides := db.GetActivePromptOverrides()
	if overrides["shell"] != "active prompt" {
		t.Fatalf("active overrides = %#v", overrides)
	}
	if got := db.GetToolPromptVersion("shell"); got != "unobserved" {
		t.Fatalf("active prompt version = %q, want unobserved", got)
	}

	res, err := db.db.Exec(`INSERT INTO prompt_overrides (tool_name, mutated_prompt, active, shadow) VALUES ('shell', 'shadow prompt', 0, 1)`)
	if err != nil {
		t.Fatalf("insert shadow override: %v", err)
	}
	id, _ := res.LastInsertId()
	got := db.GetToolPromptVersion("shell")
	if got != "unobserved" {
		t.Fatalf("shadow prompt version = %q, must not infer exposure for shadow %d", got, id)
	}
}

func TestEvaluationCyclePromotesAndRollsBackShadowPrompts(t *testing.T) {
	db, _ := openOptimizerTestDB(t)
	ctx := context.Background()
	worker := NewOptimizerWorker(db, nil, nil, 0)
	worker.evaluationLimit = 5

	promoteID := insertPromptOverride(t, db, "promote_tool", "better prompt", false, true)
	oldID := insertPromptOverride(t, db, "promote_tool", "old prompt", true, false)
	insertExposedTraces(t, db, "promote_tool", "active-"+itoa64(oldID), 5, 0)
	insertExposedTraces(t, db, "promote_tool", "v2-shadow-"+itoa64(promoteID), 5, 5)

	rollbackID := insertPromptOverride(t, db, "rollback_tool", "worse prompt", false, true)
	insertExposedTraces(t, db, "rollback_tool", "v1", 5, 5)
	insertExposedTraces(t, db, "rollback_tool", "v2-shadow-"+itoa64(rollbackID), 5, 0)

	generationBefore := promptbuilder.PromptCacheGeneration()
	worker.runEvaluationCycle(ctx)
	if generationAfter := promptbuilder.PromptCacheGeneration(); generationAfter <= generationBefore {
		t.Fatalf("prompt generation = %d, want > %d after promotion", generationAfter, generationBefore)
	}

	var active, shadow bool
	if err := db.db.QueryRow(`SELECT active, shadow FROM prompt_overrides WHERE id = ?`, promoteID).Scan(&active, &shadow); err != nil {
		t.Fatalf("promoted row missing: %v", err)
	}
	if !active || shadow {
		t.Fatalf("expected promoted shadow to become active, got active=%v shadow=%v", active, shadow)
	}

	var rollbackCount int
	if err := db.db.QueryRow(`SELECT COUNT(*) FROM prompt_overrides WHERE id = ? AND shadow=1`, rollbackID).Scan(&rollbackCount); err != nil {
		t.Fatalf("count rollback row: %v", err)
	}
	if rollbackCount != 0 {
		t.Fatalf("expected rollback row to be deleted, count=%d", rollbackCount)
	}

	var rejected int
	if err := db.db.QueryRow(`SELECT value FROM optimizer_metrics WHERE key = 'rejected_mutations'`).Scan(&rejected); err != nil {
		t.Fatalf("read rejected metric: %v", err)
	}
	if rejected != 1 {
		t.Fatalf("rejected_mutations = %d, want 1", rejected)
	}
}

func insertPromptOverride(t *testing.T, db *OptimizerDB, tool, prompt string, active, shadow bool) int64 {
	t.Helper()
	res, err := db.db.Exec(`INSERT INTO prompt_overrides (tool_name, mutated_prompt, active, shadow) VALUES (?, ?, ?, ?)`, tool, prompt, active, shadow)
	if err != nil {
		t.Fatalf("insert prompt override: %v", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("LastInsertId: %v", err)
	}
	return id
}

func insertTraces(t *testing.T, db *OptimizerDB, tool, version string, total, successes int) {
	t.Helper()
	for i := 0; i < total; i++ {
		if err := db.LogToolTrace(tool, i < successes, 0, version, "", 10); err != nil {
			t.Fatalf("insert trace: %v", err)
		}
	}
}

func itoa64(v int64) string {
	return strconv.FormatInt(v, 10)
}

func insertExposedTraces(t *testing.T, db *OptimizerDB, tool, version string, total, successes int) {
	t.Helper()
	for i := 0; i < total; i++ {
		exposures := db.recordPromptGuideExposures([]promptbuilder.ToolGuideExposure{{Manual: tool, Version: version, SourceRevision: "source"}}, "request")
		if err := db.LogToolTrace(tool, i < successes, 0, exposures[tool], "", 10); err != nil {
			t.Fatal(err)
		}
	}
}

func TestLegacyShadowLabelsNeverPromote(t *testing.T) {
	db, _ := openOptimizerTestDB(t)
	id := insertPromptOverride(t, db, "legacy", "candidate", false, true)
	insertTraces(t, db, "legacy", "v1", 10, 0)
	insertTraces(t, db, "legacy", "v2-shadow-"+itoa64(id), 10, 10)
	NewOptimizerWorker(db, nil, nil, 0).runEvaluationCycle(context.Background())
	var active bool
	if err := db.db.QueryRow(`SELECT active FROM prompt_overrides WHERE id=?`, id).Scan(&active); err != nil || active {
		t.Fatalf("legacy labels affected promotion: %v", err)
	}
}
