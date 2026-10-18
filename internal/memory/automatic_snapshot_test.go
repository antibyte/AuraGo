package memory

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"aurago/internal/config"
)

func TestAutomaticSnapshotsRetainThreeAndProtectUnknownAndIncomplete(t *testing.T) {
	cfg := &config.Config{}
	cfg.Directories.DataDir = t.TempDir()
	cfg.Directories.VectorDBDir = filepath.Join(cfg.Directories.DataDir, "vectors")
	if err := os.MkdirAll(cfg.Directories.VectorDBDir, 0700); err != nil {
		t.Fatal(err)
	}
	cfg.SQLite.ShortTermPath = filepath.Join(cfg.Directories.DataDir, "stm.db")
	cfg.SQLite.KnowledgeGraphPath = filepath.Join(cfg.Directories.DataDir, "kg.db")
	for _, path := range []string{cfg.SQLite.ShortTermPath, cfg.SQLite.KnowledgeGraphPath, config.DocumentCorpusPath(cfg)} {
		db, err := sql.Open("sqlite", path)
		if err != nil {
			t.Fatal(err)
		}
		_, err = db.Exec(`CREATE TABLE fixture(value TEXT); INSERT INTO fixture VALUES ('retained')`)
		db.Close()
		if err != nil {
			t.Fatal(err)
		}
	}
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	for _, days := range []int{60, 59, 58, 57} {
		if err := CreateAutomaticSnapshot(context.Background(), cfg, now.AddDate(0, 0, -days)); err != nil {
			t.Fatal(err)
		}
	}
	root := filepath.Join(cfg.Directories.DataDir, "memory_backups")
	before, err := verifiedAutomaticSnapshots(root)
	if err != nil || len(before) != 4 {
		t.Fatalf("before: %d %v", len(before), err)
	}
	// Newer incomplete manifests must not displace the three complete snapshots.
	for i := 0; i < 3; i++ {
		dir := filepath.Join(root, fmt.Sprintf("auto-subset-%d", i))
		if err := os.Mkdir(dir, 0700); err != nil {
			t.Fatal(err)
		}
		file := filepath.Join(dir, "short_term.sqlite")
		if err := os.WriteFile(file, []byte("subset fixture"), 0600); err != nil {
			t.Fatal(err)
		}
		digest, _ := automaticSnapshotDigest(file)
		data, _ := json.Marshal(automaticSnapshotManifest{Kind: "auto-memory-v1", CreatedAt: now.AddDate(0, 0, -i), Files: map[string]string{"short_term.sqlite": digest}})
		if err := os.WriteFile(filepath.Join(dir, "complete.json"), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	// Corrupt files cannot qualify for retention or be deleted as complete backups.
	tampered := filepath.Join(root, before[len(before)-1].name, "short_term.sqlite")
	if err := os.WriteFile(tampered, []byte("corrupted fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"manual.bak", "quarantine", "auto-incomplete"} {
		if err := os.Mkdir(filepath.Join(root, name), 0700); err != nil {
			t.Fatal(err)
		}
	}
	if err := CreateAutomaticSnapshot(context.Background(), cfg, now); err != nil {
		t.Fatal(err)
	}
	after, err := verifiedAutomaticSnapshots(root)
	if err != nil || len(after) != 3 {
		t.Fatalf("after: %d %v", len(after), err)
	}
	for _, snapshot := range after {
		if len(snapshot.manifest.Files) != 3 {
			t.Fatalf("incomplete snapshot marked complete: %+v", snapshot)
		}
	}
	for _, path := range []string{tampered, filepath.Join(root, "manual.bak"), filepath.Join(root, "quarantine"), filepath.Join(root, "auto-incomplete")} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("unverified artifact deleted: %s %v", path, err)
		}
	}
	for i := 0; i < 3; i++ {
		if _, err := os.Stat(filepath.Join(root, fmt.Sprintf("auto-subset-%d", i), "short_term.sqlite")); err != nil {
			t.Fatalf("partial snapshot removed: %v", err)
		}
	}
	if err := CreateAutomaticSnapshot(context.Background(), cfg, now); err != nil {
		t.Fatal(err)
	}
	final, _ := verifiedAutomaticSnapshots(root)
	if len(final) != 3 {
		t.Fatal("duplicate same-day snapshot")
	}
}

func TestAutomaticSnapshotFailureNeverPublishesCompletion(t *testing.T) {
	cfg := &config.Config{}
	cfg.Directories.DataDir = t.TempDir()
	cfg.SQLite.ShortTermPath = filepath.Join(cfg.Directories.DataDir, "missing.db")
	cfg.SQLite.KnowledgeGraphPath = filepath.Join(cfg.Directories.DataDir, "missing-kg.db")
	cfg.Directories.VectorDBDir = filepath.Join(cfg.Directories.DataDir, "missing-vectors")
	if err := CreateAutomaticSnapshot(context.Background(), cfg, time.Now()); err == nil {
		t.Fatal("missing source accepted")
	}
	snapshots, err := verifiedAutomaticSnapshots(filepath.Join(cfg.Directories.DataDir, "memory_backups"))
	if err != nil || len(snapshots) != 0 {
		t.Fatalf("failed snapshot completed: %v %v", snapshots, err)
	}
}
