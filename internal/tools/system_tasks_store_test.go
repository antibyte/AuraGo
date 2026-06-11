package tools

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSystemTaskStoreUsesPersistentConnection(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	store, err := newSystemTaskStore(dir)
	if err != nil {
		t.Fatalf("newSystemTaskStore: %v", err)
	}
	t.Cleanup(func() { _ = store.release() })

	if store.db == nil {
		t.Fatal("expected persistent database connection after init")
	}

	type payload struct {
		Value string `json:"value"`
	}
	if err := store.save("test_namespace", payload{Value: "alpha"}); err != nil {
		t.Fatalf("save: %v", err)
	}

	var loaded payload
	found, err := store.load("test_namespace", &loaded)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if !found || loaded.Value != "alpha" {
		t.Fatalf("loaded = %+v found=%v, want alpha/true", loaded, found)
	}

	if _, err := os.Stat(filepath.Join(dir, systemTaskStoreFile)); err != nil {
		t.Fatalf("expected sqlite file to exist: %v", err)
	}
}

func TestSystemTaskStorePoolSharesConnectionPerDataDir(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	first, err := newSystemTaskStore(dir)
	if err != nil {
		t.Fatalf("first newSystemTaskStore: %v", err)
	}
	second, err := newSystemTaskStore(dir)
	if err != nil {
		t.Fatalf("second newSystemTaskStore: %v", err)
	}
	if first != second {
		t.Fatal("expected pooled system task store instance per data dir")
	}

	if err := first.release(); err != nil {
		t.Fatalf("first release: %v", err)
	}
	if first.db == nil {
		t.Fatal("expected db to stay open while second reference is active")
	}

	if err := second.release(); err != nil {
		t.Fatalf("second release: %v", err)
	}
	if second.db != nil {
		t.Fatal("expected db handle to be cleared after final release")
	}
}

func TestSystemTaskStoreCloseIsIdempotent(t *testing.T) {
	t.Parallel()

	store, err := newSystemTaskStore(t.TempDir())
	if err != nil {
		t.Fatalf("newSystemTaskStore: %v", err)
	}
	if err := store.close(); err != nil {
		t.Fatalf("first close: %v", err)
	}
	if err := store.close(); err != nil {
		t.Fatalf("second close: %v", err)
	}
}