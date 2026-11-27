package tresor

import (
	"context"
	"database/sql"
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
	h := Header{Salt: make([]byte, 32), PasswordEnvelope: make([]byte, 60), RecoveryEnvelope: make([]byte, 60), KDF: "argon2id$v=19$m=65536,t=3,p=4"}
	if err := store.Setup(ctx, h); err != nil {
		t.Fatal(err)
	}
	if got, err := store.Header(ctx); err != nil || got.KDF != h.KDF {
		t.Fatalf("stored key derivation label: %+v, %v", got, err)
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
	higher := h
	higher.KDF = "argon2id$v=19$m=131072,t=4,p=4"
	if err := store.Rewrap(ctx, higher, 2); err != nil {
		t.Fatalf("relabel header: %v", err)
	}
	if got, err := store.Header(ctx); err != nil || got.KDF != higher.KDF || got.Revision != 3 {
		t.Fatalf("relabelled header: %+v, %v", got, err)
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
	if err != nil || gotHeader.Revision != 3 || gotHeader.KDF != higher.KDF {
		t.Fatalf("restored header: %+v, %v", gotHeader, err)
	}
	if err := restored.Delete(ctx, item.ID, 1); !errors.Is(err, ErrConflict) {
		t.Fatalf("stale delete: %v", err)
	}
	if err := restored.Delete(ctx, item.ID, 2); err != nil {
		t.Fatal(err)
	}
}

// A vault written before the label existed has no meta row; reading it must not
// invent parameters, so the client falls back to the format default.
func TestHeaderWithoutKeyDerivationLabelStaysEmpty(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "tresor.db")
	legacy, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = legacy.Exec(`CREATE TABLE header (id INTEGER PRIMARY KEY CHECK (id=1), salt BLOB NOT NULL,
		password_envelope BLOB NOT NULL, recovery_envelope BLOB NOT NULL, revision INTEGER NOT NULL);
		INSERT INTO header(id,salt,password_envelope,recovery_envelope,revision)
			VALUES(1,zeroblob(32),zeroblob(60),zeroblob(60),1);`); err != nil {
		legacy.Close()
		t.Fatal(err)
	}
	if err = legacy.Close(); err != nil {
		t.Fatal(err)
	}
	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	got, err := store.Header(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got.KDF != "" || got.Revision != 1 || len(got.Salt) != 32 {
		t.Fatalf("legacy header: %+v", got)
	}
}

func TestValidKDFLabel(t *testing.T) {
	cases := map[string]bool{
		"":                                   true,
		"argon2id$v=19$m=65536,t=3,p=4":      true,
		"argon2id$v=19$m=131072,t=4,p=8":     true,
		"argon2id$v=19$m=65536,t=3,p=4 ":     false,
		"argon2id$v=19$m=65536,t=3":          false,
		"argon2id$v=19$m=99999999,t=3,p=4":   false,
		"bcrypt$v=19$m=65536,t=3,p=4":        false,
		"argon2id$v=19$m=65536,t=3,p=4;drop": false,
		"argon2id$v=19$m=65536,t=3,p=4\n":    false,
		"argon2id$v=19$m=65536,t=3,p=4extra": false,
		"argon2id$v=1$m=65536,t=3,p=4":       true,
	}
	for label, want := range cases {
		if got := ValidKDFLabel(label); got != want {
			t.Errorf("ValidKDFLabel(%q) = %v, want %v", label, got, want)
		}
	}
}
