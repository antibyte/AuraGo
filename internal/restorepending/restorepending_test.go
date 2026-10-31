package restorepending

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func read(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func exists(path string) bool { _, err := os.Stat(path); return err == nil }

// stageFiles builds a staging tree from rel->content and returns entries.
func stageFiles(t *testing.T, root string, files map[string]string, sqlite map[string]bool) (string, []Entry) {
	t.Helper()
	staging, err := os.MkdirTemp(root, ".ago-import-*")
	if err != nil {
		t.Fatal(err)
	}
	var entries []Entry
	for rel, content := range files {
		write(t, StageFilePath(staging, rel), content)
		entries = append(entries, Entry{Rel: rel, Dest: filepath.Join(root, filepath.FromSlash(rel)), Mode: 0o644, SQLite: sqlite[rel]})
	}
	return staging, entries
}

func TestApplyReplacesFilesRemovesSidecarsAndWritesResult(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "config.yaml"), "old")
	write(t, filepath.Join(root, "data", "a.db"), "olddb")
	write(t, filepath.Join(root, "data", "a.db-wal"), "wal")
	write(t, filepath.Join(root, "data", "a.db-shm"), "shm")
	staging, entries := stageFiles(t, root, map[string]string{"config.yaml": "new", "data/a.db": "newdb"}, map[string]bool{"data/a.db": true})
	if _, err := Stage(root, staging, entries); err != nil {
		t.Fatal(err)
	}
	if st := ReadState(root); !st.Pending || st.PendingCreatedAt == "" {
		t.Fatalf("state = %+v", st)
	}
	if read(t, filepath.Join(root, "config.yaml")) != "old" {
		t.Fatal("stage must not touch live files")
	}
	res, err := Apply(root, Options{})
	if err != nil || res.Status != StatusApplied {
		t.Fatalf("res=%+v err=%v", res, err)
	}
	if read(t, filepath.Join(root, "config.yaml")) != "new" || read(t, filepath.Join(root, "data", "a.db")) != "newdb" {
		t.Fatal("files not replaced")
	}
	if exists(filepath.Join(root, "data", "a.db-wal")) || exists(filepath.Join(root, "data", "a.db-shm")) {
		t.Fatal("stale sidecars must be removed")
	}
	st := ReadState(root)
	if st.Pending || st.Last == nil || st.Last.Status != StatusApplied {
		t.Fatalf("state = %+v", st)
	}
	if res2, err := Apply(root, Options{}); res2 != nil || err != nil {
		t.Fatal("second apply must be a no-op")
	}
}

func TestApplyFailureRollsEverythingBack(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "config.yaml"), "old")
	write(t, filepath.Join(root, "data", "a.db"), "olddb")
	write(t, filepath.Join(root, "data", "a.db-wal"), "wal")
	staging, entries := stageFiles(t, root, map[string]string{"config.yaml": "new", "data/a.db": "newdb", "data/b.json": "b"}, map[string]bool{"data/a.db": true})
	if _, err := Stage(root, staging, entries); err != nil {
		t.Fatal(err)
	}
	calls := 0
	res, err := Apply(root, Options{afterPublish: func(i int, e Entry) error {
		calls++
		if calls == 3 {
			return errors.New("injected")
		}
		return nil
	}})
	if err == nil || res == nil || res.Status != StatusRolledBack {
		t.Fatalf("res=%+v err=%v", res, err)
	}
	if read(t, filepath.Join(root, "config.yaml")) != "old" || read(t, filepath.Join(root, "data", "a.db")) != "olddb" || read(t, filepath.Join(root, "data", "a.db-wal")) != "wal" {
		t.Fatal("old state not restored")
	}
	if exists(filepath.Join(root, "data", "b.json")) {
		t.Fatal("newly created file must be removed on rollback")
	}
	if ReadState(root).Pending {
		t.Fatal("pending must be cleared after failure")
	}
}

func TestApplyInvalidRestoredConfigRollsBack(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "config.yaml"), "old")
	staging, entries := stageFiles(t, root, map[string]string{"config.yaml": "broken"}, nil)
	if _, err := Stage(root, staging, entries); err != nil {
		t.Fatal(err)
	}
	res, err := Apply(root, Options{Validate: func() error { return errors.New("bad yaml") }})
	if err == nil || res.Status != StatusRolledBack || read(t, filepath.Join(root, "config.yaml")) != "old" {
		t.Fatalf("res=%+v err=%v", res, err)
	}
}

func TestStageReplacesOlderPending(t *testing.T) {
	root := t.TempDir()
	s1, e1 := stageFiles(t, root, map[string]string{"data/one.json": "1"}, nil)
	id1, err := Stage(root, s1, e1)
	if err != nil {
		t.Fatal(err)
	}
	s2, e2 := stageFiles(t, root, map[string]string{"data/two.json": "2"}, nil)
	id2, err := Stage(root, s2, e2)
	if err != nil || id1 == id2 {
		t.Fatalf("id1=%s id2=%s err=%v", id1, id2, err)
	}
	if exists(StagedPath(root, "data/one.json")) || !exists(StagedPath(root, "data/two.json")) {
		t.Fatal("newer pending must replace the older one")
	}
	if _, err := Apply(root, Options{}); err != nil {
		t.Fatal(err)
	}
	if exists(filepath.Join(root, "data", "one.json")) || read(t, filepath.Join(root, "data", "two.json")) != "2" {
		t.Fatal("only the newer restore may be applied")
	}
	left, _ := filepath.Glob(filepath.Join(root, "data", ".restore-pending*"))
	if len(left) != 0 {
		t.Fatalf("leftovers: %v", left)
	}
}
