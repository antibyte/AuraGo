package memory

import (
	"log/slog"
	"path/filepath"
	"testing"
	"testing/synctest"
)

func TestInMemorySnapshotCoordinatorOwnedByStore(t *testing.T) {
	var coordinator *MemorySnapshotCoordinator
	synctest.Test(t, func(t *testing.T) {
		store, err := NewSQLiteMemory(":memory:", slog.Default())
		if err != nil {
			t.Fatal(err)
		}
		defer store.Close()
		coordinator, err = store.snapshotCoordinator(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		again, err := store.snapshotCoordinator(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		if again != coordinator {
			t.Fatal("one in-memory store returned different coordinators")
		}
		_, release, err := store.BeginMemoryPublication(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		release()
		if err := store.Close(); err != nil {
			t.Fatal(err)
		}
	})
	memorySnapshotCoordinators.Range(func(_, value any) bool {
		if value == coordinator {
			t.Error("global registry retained an in-memory store's snapshot coordinator")
			return false
		}
		return true
	})
}

func TestFileSnapshotCoordinatorSurvivesHandleClose(t *testing.T) {
	path := filepath.Join(t.TempDir(), "memory.db")
	first, err := NewSQLiteMemory(path, slog.Default())
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	second, err := NewSQLiteMemory(path, slog.Default())
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	before, err := first.snapshotCoordinator(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	after, err := second.snapshotCoordinator(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if before != after {
		t.Fatal("closing one file-backed handle replaced the shared coordinator")
	}
}
