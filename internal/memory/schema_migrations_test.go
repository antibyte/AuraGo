package memory

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/dbutil"
)

func migrationTestLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func TestMemorySchemaRejectsFutureVersionBeforeChanges(t *testing.T) {
	path := filepath.Join(t.TempDir(), "future.db")
	db, err := dbutil.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(fmt.Sprintf(`CREATE TABLE future_only(value TEXT); INSERT INTO future_only VALUES('preserve'); PRAGMA user_version=%d`, shortTermSchemaVersion+1)); err != nil {
		t.Fatal(err)
	}
	db.Close()
	if s, err := NewSQLiteMemory(path, migrationTestLogger()); err == nil || !strings.Contains(err.Error(), "newer than supported") || s != nil {
		t.Fatalf("store=%v err=%v", s, err)
	}
	db, err = dbutil.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var version, tables int
	if err := db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type='table'`).Scan(&tables); err != nil {
		t.Fatal(err)
	}
	if version != shortTermSchemaVersion+1 || tables != 1 {
		t.Fatalf("future database mutated: version=%d tables=%d", version, tables)
	}
}

func TestMemorySchemaStepRollsBackDataAndVersion(t *testing.T) {
	db, err := dbutil.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	injected := errors.New("injected migration failure")
	step := memorySchemaStep{version: 12, apply: func(ctx context.Context, tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `CREATE TABLE migration_probe(value TEXT); INSERT INTO migration_probe VALUES('secret fixture')`); err != nil {
			return err
		}
		return injected
	}}
	if err := applyMemorySchemaStep(context.Background(), db, step); !errors.Is(err, injected) {
		t.Fatalf("error=%v", err)
	}
	var version, exists int
	db.QueryRow("PRAGMA user_version").Scan(&version)
	db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE name='migration_probe'`).Scan(&exists)
	if version != 0 || exists != 0 {
		t.Fatalf("failed migration committed: version=%d exists=%d", version, exists)
	}
	step.apply = func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `CREATE TABLE migration_probe(value TEXT)`)
		return err
	}
	if err := applyMemorySchemaStep(context.Background(), db, step); err != nil {
		t.Fatal(err)
	}
	db.QueryRow("PRAGMA user_version").Scan(&version)
	if version != 12 {
		t.Fatalf("version=%d", version)
	}
}

func TestMemorySchemaBridgeBackupRestartAndIntegrity(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.db")
	s, err := NewSQLiteMemory(path, migrationTestLogger())
	if err != nil {
		t.Fatal(err)
	}
	// A version-11 fixture is a complete pre-versioned database. Removing a
	// late optional schema reproduces an older installation's transition.
	if _, err := s.db.Exec(`PRAGMA user_version=11; DROP TABLE memory_learnings; DROP TABLE memory_learning_revisions;
		INSERT INTO messages(session_id,role,content) VALUES('fixture','user','preserved source')`); err != nil {
		t.Fatal(err)
	}
	s.Close()
	s, err = NewSQLiteMemory(path, migrationTestLogger())
	if err != nil {
		t.Fatal(err)
	}
	s.Close()
	backups, err := filepath.Glob(path + ".schema-v*.bak")
	if err != nil || len(backups) != 1 {
		t.Fatalf("backups=%v err=%v", backups, err)
	}
	for _, candidate := range []string{path, backups[0]} {
		db, err := dbutil.Open(candidate)
		if err != nil {
			t.Fatal(err)
		}
		for _, pragma := range []string{"quick_check", "integrity_check"} {
			var result string
			if err := db.QueryRow("PRAGMA " + pragma).Scan(&result); err != nil || result != "ok" {
				t.Fatalf("%s %s=%q err=%v", candidate, pragma, result, err)
			}
		}
		var content string
		if err := db.QueryRow(`SELECT content FROM messages WHERE session_id='fixture'`).Scan(&content); err != nil || content != "preserved source" {
			t.Fatalf("source=%q err=%v", content, err)
		}
		db.Close()
	}
	s, err = NewSQLiteMemory(path, migrationTestLogger())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	after, _ := filepath.Glob(path + ".schema-v*.bak")
	if len(after) != len(backups) {
		t.Fatal("current schema reran migration/backup")
	}
}

func TestMemorySchemaRequiredInitializerFailureDoesNotAdvanceVersion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "failed.db")
	db, err := dbutil.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE VIEW activity_turns AS SELECT 'invalid' AS bad`); err != nil {
		t.Fatal(err)
	}
	db.Close()
	if s, err := NewSQLiteMemory(path, migrationTestLogger()); err == nil || s != nil {
		t.Fatalf("store=%v err=%v", s, err)
	}
	db, err = dbutil.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var version int
	if err := db.QueryRow("PRAGMA user_version").Scan(&version); err != nil || version != 0 {
		t.Fatalf("version=%d err=%v", version, err)
	}
	if _, err := db.Exec(`DROP VIEW activity_turns`); err != nil {
		t.Fatal(err)
	}
	db.Close()
	s, err := NewSQLiteMemory(path, migrationTestLogger())
	if err != nil {
		t.Fatalf("recovery: %v", err)
	}
	s.Close()
}

func TestMemorySchemaCurrentDoesNotReplayInitializers(t *testing.T) {
	path := filepath.Join(t.TempDir(), "current.db")
	s, err := NewSQLiteMemory(path, migrationTestLogger())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`CREATE TRIGGER reject_trait_reseed BEFORE INSERT ON personality_traits BEGIN SELECT RAISE(ABORT,'unexpected reseed'); END`); err != nil {
		t.Fatal(err)
	}
	s.Close()
	s, err = NewSQLiteMemory(path, migrationTestLogger())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`DROP TABLE activity_turns`); err != nil {
		t.Fatal(err)
	}
	s.Close()
	if s, err := NewSQLiteMemory(path, migrationTestLogger()); err == nil || s != nil {
		t.Fatalf("incomplete versioned schema silently repaired: store=%v error=%v", s, err)
	}
}

func TestMemorySchemaToolOperationMigrationBacksUpAndPreservesArchive(t *testing.T) {
	path := filepath.Join(t.TempDir(), "archive-v15.db")
	s, err := NewSQLiteMemory(path, migrationTestLogger())
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := s.StoreCompressedOutput(ctx, &CompressedToolOutput{SessionID: "fixture", ToolCallID: "mail", ToolName: "google_workspace", OriginalContent: "preserved mail", CompressedContent: "compact", FilterUsed: "vault"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`ALTER TABLE compressed_tool_outputs DROP COLUMN tool_operation; PRAGMA user_version=15`); err != nil {
		t.Fatal(err)
	}
	s.Close()
	s, err = NewSQLiteMemory(path, migrationTestLogger())
	if err != nil {
		t.Fatal(err)
	}
	out, err := s.RetrieveCompressedOutput(ctx, "fixture", "mail")
	if err != nil || out.ToolOperation != "" || out.ToolName != "google_workspace" || out.OriginalContent != "preserved mail" {
		t.Fatalf("legacy archive changed: %+v err=%v", out, err)
	}
	s.Close()
	backups, err := filepath.Glob(path + ".schema-v16-*.bak")
	if err != nil || len(backups) != 1 {
		t.Fatalf("migration backup missing: %v %v", backups, err)
	}
	db, err := dbutil.Open(backups[0])
	if err != nil {
		t.Fatal(err)
	}
	var version int
	var content, integrity string
	if err := db.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil || version != 15 {
		t.Fatalf("backup version=%d err=%v", version, err)
	}
	if err := db.QueryRow(`SELECT original_content FROM compressed_tool_outputs WHERE tool_call_id='mail'`).Scan(&content); err != nil || content != "preserved mail" {
		t.Fatalf("backup content=%q err=%v", content, err)
	}
	if err := db.QueryRow(`PRAGMA integrity_check`).Scan(&integrity); err != nil || integrity != "ok" {
		t.Fatalf("backup integrity=%q err=%v", integrity, err)
	}
	db.Close()
	s, err = NewSQLiteMemory(path, migrationTestLogger())
	if err != nil {
		t.Fatal(err)
	}
	s.Close()
	after, _ := filepath.Glob(path + ".schema-v16-*.bak")
	if len(after) != 1 {
		t.Fatal("restart repeated migration")
	}
}
