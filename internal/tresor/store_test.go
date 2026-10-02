package tresor

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestStoreRevisionsAndRestore(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "tresor.db")
	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	h := Header{Salt: make([]byte, 32), PasswordEnvelope: make([]byte, 60), RecoveryEnvelope: make([]byte, 60)}
	if err := store.Setup(ctx, h); err != nil {
		t.Fatal(err)
	}
	if err := store.Setup(ctx, h); !errors.Is(err, ErrConflict) {
		t.Fatalf("duplicate setup: %v", err)
	}
	if err := store.Rewrap(ctx, h, 1); err != nil {
		t.Fatal(err)
	}
	if err := store.Rewrap(ctx, h, 1); !errors.Is(err, ErrConflict) {
		t.Fatalf("stale header: %v", err)
	}
	item := Record{ID: "a8185ea8-4ec9-447d-bd6c-88cd4ed6228b", Meta: make([]byte, 28), Body: make([]byte, 28)}
	if err := store.Create(ctx, item); err != nil {
		t.Fatal(err)
	}
	if err := store.Create(ctx, item); !errors.Is(err, ErrConflict) {
		t.Fatalf("duplicate item: %v", err)
	}
	item.Body[27] = 1
	if err := store.Update(ctx, item, 1); err != nil {
		t.Fatal(err)
	}
	if err := store.Update(ctx, item, 1); !errors.Is(err, ErrConflict) {
		t.Fatalf("stale item: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	backup, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	restoredPath := filepath.Join(t.TempDir(), "tresor.db")
	if err := os.WriteFile(restoredPath, backup, 0600); err != nil {
		t.Fatal(err)
	}
	restored, err := Open(restoredPath)
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close()
	got, err := restored.Get(ctx, item.ID)
	if err != nil || got.Revision != 2 || got.Body[27] != 1 {
		t.Fatalf("restored item: %+v, %v", got, err)
	}
	gotHeader, err := restored.Header(ctx)
	if err != nil || gotHeader.Revision != 2 {
		t.Fatalf("restored header: %+v, %v", gotHeader, err)
	}
	if err := restored.Delete(ctx, item.ID, 1); !errors.Is(err, ErrConflict) {
		t.Fatalf("stale delete: %v", err)
	}
	if err := restored.Delete(ctx, item.ID, 2); err != nil {
		t.Fatal(err)
	}
}
