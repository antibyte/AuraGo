package memory

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
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
	unknownDir := filepath.Join(root, "auto-unknown-version")
	writeAutomaticSnapshotFixture(t, root, filepath.Base(unknownDir), now.AddDate(0, 0, -40))
	unknownManifestPath := filepath.Join(unknownDir, "complete.json")
	unknownData, err := os.ReadFile(unknownManifestPath)
	if err != nil {
		t.Fatal(err)
	}
	var unknownManifest automaticSnapshotManifest
	if err := json.Unmarshal(unknownData, &unknownManifest); err != nil {
		t.Fatal(err)
	}
	unknownManifest.Kind = "auto-memory-v9"
	unknownData, err = json.Marshal(unknownManifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(unknownManifestPath, unknownData, 0600); err != nil {
		t.Fatal(err)
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
	for _, path := range []string{tampered, filepath.Join(root, "manual.bak"), filepath.Join(root, "quarantine"), filepath.Join(root, "auto-incomplete"), unknownManifestPath} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("unverified artifact deleted: %s %v", path, err)
		}
	}
	if payload, err := os.ReadFile(filepath.Join(unknownDir, "short_term.sqlite")); err != nil || string(payload) != "auto-unknown-version:short_term.sqlite" {
		t.Fatalf("unknown-version backup changed: %q %v", payload, err)
	}
	for i := 0; i < 3; i++ {
		dir := filepath.Join(root, fmt.Sprintf("auto-subset-%d", i))
		if payload, err := os.ReadFile(filepath.Join(dir, "short_term.sqlite")); err != nil || string(payload) != "subset fixture" {
			t.Fatalf("legacy v1 snapshot changed: %q %v", payload, err)
		}
		if _, err := os.Stat(filepath.Join(dir, "complete.json")); err != nil {
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
	root := filepath.Join(cfg.Directories.DataDir, "memory_backups")
	snapshots, err := verifiedAutomaticSnapshots(root)
	if err != nil || len(snapshots) != 0 {
		t.Fatalf("failed snapshot completed: %v %v", snapshots, err)
	}
	// The partially written auto-* directory must not be left behind.
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 0 {
		t.Fatalf("failed snapshot left artifacts: %v %v", entries, err)
	}
	configured, have, err := AutomaticSnapshotStatus(cfg, time.Now())
	if err != nil || !configured || have {
		t.Fatalf("status after failure: configured=%v have=%v err=%v", configured, have, err)
	}
}

func TestAutomaticSnapshotRetentionPreservesEveryIncompleteDirectory(t *testing.T) {
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
		_, err = db.Exec(`CREATE TABLE fixture(value TEXT)`)
		db.Close()
		if err != nil {
			t.Fatal(err)
		}
	}
	root := filepath.Join(cfg.Directories.DataDir, "memory_backups")
	now := time.Now()
	if configured, have, err := AutomaticSnapshotStatus(cfg, now); err != nil || !configured || have {
		t.Fatalf("status before snapshot: %v %v %v", configured, have, err)
	}
	mk := func(name string, age time.Duration, extra string) string {
		dir := filepath.Join(root, name)
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "short_term.sqlite"), []byte("partial"), 0600); err != nil {
			t.Fatal(err)
		}
		if extra != "" {
			if err := os.WriteFile(filepath.Join(dir, extra), []byte("x"), 0600); err != nil {
				t.Fatal(err)
			}
		}
		old := now.Add(-age)
		if err := os.Chtimes(dir, old, old); err != nil {
			t.Fatal(err)
		}
		return dir
	}
	stale := mk("auto-stale", 48*time.Hour, "")
	fresh := mk("auto-fresh", time.Minute, "")
	foreign := mk("auto-stale-foreign", 48*time.Hour, "notes.txt")
	if err := os.WriteFile(filepath.Join(foreign, "notes.txt"), []byte("foreign payload"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := CreateAutomaticSnapshot(context.Background(), cfg, now); err != nil {
		t.Fatal(err)
	}
	for _, kept := range []string{stale, fresh, foreign} {
		if _, err := os.Stat(kept); err != nil {
			t.Fatalf("protected dir removed: %s %v", kept, err)
		}
	}
	if payload, err := os.ReadFile(filepath.Join(foreign, "notes.txt")); err != nil || string(payload) != "foreign payload" {
		t.Fatalf("foreign payload changed: %q %v", payload, err)
	}
	if configured, have, err := AutomaticSnapshotStatus(cfg, now); err != nil || !configured || !have {
		t.Fatalf("status after snapshot: %v %v %v", configured, have, err)
	}
	if configured, _, _ := AutomaticSnapshotStatus(&config.Config{}, now); configured {
		t.Fatal("unconfigured reported as configured")
	}
}

func TestAutomaticSnapshotPartialStoreConfigurationFailsClosed(t *testing.T) {
	cfg := &config.Config{}
	cfg.Directories.DataDir = t.TempDir()
	cfg.SQLite.ShortTermPath = filepath.Join(cfg.Directories.DataDir, "stm.db")
	cfg.SQLite.KnowledgeGraphPath = filepath.Join(cfg.Directories.DataDir, "kg.db")
	now := time.Now()
	if err := CreateAutomaticSnapshot(context.Background(), cfg, now); !errors.Is(err, ErrAutomaticSnapshotIncompleteConfig) {
		t.Fatalf("partial config error = %v", err)
	}
	configured, haveToday, err := AutomaticSnapshotStatus(cfg, now)
	if !configured || haveToday || !errors.Is(err, ErrAutomaticSnapshotIncompleteConfig) {
		t.Fatalf("partial config status = %v %v %v", configured, haveToday, err)
	}
}

func TestAutomaticSnapshotRetentionRevalidatesOnlySelectedCandidates(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	const count = 16
	for i := range count {
		writeAutomaticSnapshotFixture(t, root, fmt.Sprintf("auto-%02d", i), now.AddDate(0, 0, -(31+i)))
	}
	var reads automaticSnapshotReadCounts
	snapshots, err := verifiedAutomaticSnapshotsWithReads(root, &reads)
	if err != nil || len(snapshots) != count {
		t.Fatalf("inventory=%d err=%v", len(snapshots), err)
	}
	if reads.ManifestReads != count || reads.SnapshotFiles != 3*count {
		t.Fatalf("initial reads=%+v", reads)
	}
	if err := retainAutomaticSnapshotsWithReads(root, snapshots, now, &reads); err != nil {
		t.Fatal(err)
	}
	if want := 2*count - 3; reads.ManifestReads != want || reads.SnapshotFiles != 3*want {
		t.Fatalf("inventory plus selected rechecks=%+v; want %d manifests and %d file hashes", reads, want, 3*want)
	}
	remaining, err := verifiedAutomaticSnapshots(root)
	if err != nil || len(remaining) != 3 {
		t.Fatalf("retained=%d err=%v", len(remaining), err)
	}
}

func TestAutomaticSnapshotRetentionKeepsCandidateChangedAfterInventory(t *testing.T) {
	root := t.TempDir()
	now := time.Now().UTC()
	for i := range 4 {
		writeAutomaticSnapshotFixture(t, root, fmt.Sprintf("auto-%02d", i), now.AddDate(0, 0, -(40+i)))
	}
	snapshots, err := verifiedAutomaticSnapshots(root)
	if err != nil || len(snapshots) != 4 {
		t.Fatalf("inventory=%d err=%v", len(snapshots), err)
	}
	selected := snapshots[3]
	foreign := filepath.Join(root, selected.name, "short_term.sqlite")
	if err := os.WriteFile(foreign, []byte("corrupted after inventory"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := retainAutomaticSnapshots(root, snapshots, now); err != nil {
		t.Fatal(err)
	}
	if payload, err := os.ReadFile(foreign); err != nil || string(payload) != "corrupted after inventory" {
		t.Fatalf("changed candidate was deleted: %q %v", payload, err)
	}
}

func TestAutomaticSnapshotRetentionKeepsCandidateWithUnexpectedFile(t *testing.T) {
	root := t.TempDir()
	now := time.Now().UTC()
	for i := range 4 {
		writeAutomaticSnapshotFixture(t, root, fmt.Sprintf("auto-%02d", i), now.AddDate(0, 0, -(40+i)))
	}
	snapshots, err := verifiedAutomaticSnapshots(root)
	if err != nil || len(snapshots) != 4 {
		t.Fatalf("inventory=%d err=%v", len(snapshots), err)
	}
	selectedDir := filepath.Join(root, snapshots[3].name)
	foreign := filepath.Join(selectedDir, "foreign.txt")
	if err := os.WriteFile(foreign, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := retainAutomaticSnapshots(root, snapshots, now); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"short_term.sqlite", "knowledge_graph.sqlite", "documents.sqlite", "complete.json", "foreign.txt"} {
		if _, err := os.Stat(filepath.Join(selectedDir, name)); err != nil {
			t.Fatalf("candidate content %q was removed: %v", name, err)
		}
	}
}

func TestAutomaticSnapshotRetentionKeepsCandidateWithChangedIdentity(t *testing.T) {
	root := t.TempDir()
	now := time.Now().UTC()
	for i := range 4 {
		writeAutomaticSnapshotFixture(t, root, fmt.Sprintf("auto-%02d", i), now.AddDate(0, 0, -(40+i)))
	}
	snapshots, err := verifiedAutomaticSnapshots(root)
	if err != nil || len(snapshots) != 4 {
		t.Fatalf("inventory=%d err=%v", len(snapshots), err)
	}
	selectedDir := filepath.Join(root, snapshots[3].name)
	manifestPath := filepath.Join(selectedDir, "complete.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	var manifest automaticSnapshotManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	manifest.CreatedAt = manifest.CreatedAt.Add(time.Hour)
	data, err = json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manifestPath, data, 0600); err != nil {
		t.Fatal(err)
	}
	if err := retainAutomaticSnapshots(root, snapshots, now); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(manifestPath); err != nil {
		t.Fatalf("candidate with changed identity was removed: %v", err)
	}
}

func writeAutomaticSnapshotFixture(t testing.TB, root, name string, createdAt time.Time) {
	t.Helper()
	dir := filepath.Join(root, name)
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{}
	for _, file := range []string{"short_term.sqlite", "knowledge_graph.sqlite", "documents.sqlite"} {
		path := filepath.Join(dir, file)
		if err := os.WriteFile(path, []byte(name+":"+file), 0600); err != nil {
			t.Fatal(err)
		}
		digest, err := automaticSnapshotDigest(path)
		if err != nil {
			t.Fatal(err)
		}
		files[file] = digest
	}
	manifest := automaticSnapshotManifest{Kind: "auto-memory-v2", Consistency: memorySnapshotConsistency, CreatedAt: createdAt, Files: files}
	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "complete.json"), data, 0600); err != nil {
		t.Fatal(err)
	}
}

func BenchmarkAutomaticSnapshotRetention(b *testing.B) {
	for _, count := range []int{16, 64} {
		b.Run(fmt.Sprint(count), func(b *testing.B) {
			var totals automaticSnapshotReadCounts
			for i := 0; i < b.N; i++ {
				b.StopTimer()
				root := b.TempDir()
				now := time.Now().UTC()
				for n := range count {
					writeAutomaticSnapshotFixture(b, root, fmt.Sprintf("auto-%03d", n), now.AddDate(0, 0, -(40+n)))
				}
				b.StartTimer()
				var reads automaticSnapshotReadCounts
				snapshots, err := verifiedAutomaticSnapshotsWithReads(root, &reads)
				if err != nil {
					b.Fatal(err)
				}
				if err := retainAutomaticSnapshotsWithReads(root, snapshots, now, &reads); err != nil {
					b.Fatal(err)
				}
				totals.ManifestReads += reads.ManifestReads
				totals.SnapshotFiles += reads.SnapshotFiles
			}
			b.ReportMetric(float64(totals.ManifestReads)/float64(b.N), "manifest-reads/op")
			b.ReportMetric(float64(totals.SnapshotFiles)/float64(b.N), "file-hashes/op")
		})
	}
}
