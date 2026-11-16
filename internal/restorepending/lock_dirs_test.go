package restorepending

import (
	"path/filepath"
	"slices"
	"testing"
)

func TestAffectedDataDirectoriesIncludesMappedTargetsAndRollback(t *testing.T) {
	root, mapped, previous := t.TempDir(), t.TempDir(), t.TempDir()
	staging, entries := stageFiles(t, root, map[string]string{"data/a.db": "db", "data/vectordb/documents.sqlite": "corpus"}, nil)
	id, err := Stage(root, staging, entries)
	if err != nil {
		t.Fatal(err)
	}
	rb := filepath.Join(dataDir(root), rollbackDirName, id)
	write(t, filepath.Join(rb, markerName), "sealed")
	if err := writeJSON(filepath.Join(rb, recordsName), []record{{Dest: filepath.Join(previous, "old.db")}}); err != nil {
		t.Fatal(err)
	}
	pendingBefore := read(t, filepath.Join(pendingDir(root), manifestName))
	dirs, err := AffectedDataDirectories(root, Options{ResolveDirs: func() (Dirs, error) { return Dirs{VectorDB: mapped}, nil }})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{filepath.Join(root, "data"), mapped, previous} {
		if !slices.Contains(dirs, want) {
			t.Errorf("missing %s in %v", want, dirs)
		}
	}
	if got := read(t, filepath.Join(pendingDir(root), manifestName)); got != pendingBefore {
		t.Fatal("read-only resolution changed pending restore")
	}
}
