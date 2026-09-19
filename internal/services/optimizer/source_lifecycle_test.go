package optimizer

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestActiveOverrideRevalidatesChangedAndMalformedManual(t *testing.T) {
	db, _ := openOptimizerTestDB(t)
	db.promptsDir = t.TempDir()
	dir := filepath.Join(db.promptsDir, "tools_manuals")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "execute_shell.md")
	write := func(body string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
	}
	write("# execute_shell\nCanonical local workflow")
	hash := fmt.Sprintf("%x", sha256.Sum256([]byte(db.loadCanonicalManual("execute_shell"))))
	if _, err := db.db.Exec(`INSERT INTO prompt_overrides(tool_name,mutated_prompt,original_hash,active) VALUES('execute_shell','Revised workflow',?,1)`, hash); err != nil {
		t.Fatal(err)
	}
	if db.GetActivePromptOverrides()["execute_shell"] == "" {
		t.Fatal("current override missing")
	}
	write("# execute_shell\nNew canonical local workflow")
	if db.GetActivePromptOverrides()["execute_shell"] != "" {
		t.Fatal("stale override remained active after disk edit")
	}
	write("---\ndescription: [broken\n---\nUNTRUSTED_INVALID_SOURCE")
	canonical := db.loadCanonicalManual("execute_shell")
	if strings.Contains(canonical, "UNTRUSTED_INVALID_SOURCE") || canonical == "(No existing manual found)" {
		t.Fatalf("invalid override must use valid embedded source: %q", canonical)
	}
}

func TestTraceRetentionAlsoPrunesUnreferencedExposures(t *testing.T) {
	db, _ := openOptimizerTestDB(t)
	for _, age := range []string{"-100 days", "-1 days"} {
		if _, err := db.db.Exec(`INSERT INTO prompt_exposures(manual,version,source_revision,prompt_revision,timestamp) VALUES('execute_shell','v1','s','p',datetime('now',?))`, age); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.CleanupOldTraces(90); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := db.db.QueryRow(`SELECT count(*) FROM prompt_exposures`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("exposures=%d err=%v", count, err)
	}
	if err := db.CleanupOldTraces(0); err == nil {
		t.Fatal("zero retention must not delete current exposures")
	}
}
