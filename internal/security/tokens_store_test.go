package security

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func newTokenStoreTestVault(t *testing.T, dir, keyChar string) *Vault {
	t.Helper()
	v, err := NewVault(strings.Repeat(keyChar, 64), filepath.Join(dir, "vault-"+keyChar+".bin"))
	if err != nil {
		t.Fatalf("NewVault: %v", err)
	}
	return v
}

func TestNewTokenManagerRefusesToOverwriteUndecryptableStore(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tokens.bin")
	owner := newTokenStoreTestVault(t, dir, "a")
	tm, err := NewTokenManager(owner, path)
	if err != nil {
		t.Fatalf("NewTokenManager(owner): %v", err)
	}
	raw, _, err := tm.Create("display", []string{"cyd"}, nil)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	degraded, err := NewTokenManager(newTokenStoreTestVault(t, dir, "b"), path)
	if !errors.Is(err, ErrTokenStoreUnavailable) {
		t.Fatalf("NewTokenManager(wrong key) error = %v, want ErrTokenStoreUnavailable", err)
	}
	if degraded == nil || degraded.LoadError() == nil {
		t.Fatal("a wrong master key must yield a read-only manager that reports its load error")
	}
	if _, _, err := degraded.Create("replacement", []string{"webhook"}, nil); !errors.Is(err, ErrTokenStoreUnavailable) {
		t.Fatalf("Create on degraded store error = %v, want ErrTokenStoreUnavailable", err)
	}
	if after, _ := os.ReadFile(path); !bytes.Equal(before, after) {
		t.Fatal("degraded manager rewrote the token file")
	}

	reopened, err := NewTokenManager(owner, path)
	if err != nil {
		t.Fatalf("NewTokenManager(owner again): %v", err)
	}
	if _, ok := reopened.Validate(raw, "cyd"); !ok {
		t.Fatal("original token lost after a degraded write attempt")
	}
}

func TestNewTokenManagerRefusesToOverwriteCorruptStore(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tokens.bin")
	corrupt := []byte("this is not AES-GCM ciphertext at all")
	if err := os.WriteFile(path, corrupt, 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	tm, err := NewTokenManager(newTokenStoreTestVault(t, dir, "a"), path)
	if !errors.Is(err, ErrTokenStoreUnavailable) {
		t.Fatalf("NewTokenManager(corrupt) error = %v, want ErrTokenStoreUnavailable", err)
	}
	if _, _, err := tm.Create("replacement", []string{"webhook"}, nil); !errors.Is(err, ErrTokenStoreUnavailable) {
		t.Fatalf("Create on corrupt store error = %v, want ErrTokenStoreUnavailable", err)
	}
	if got, _ := os.ReadFile(path); !bytes.Equal(got, corrupt) {
		t.Fatal("corrupt token file was overwritten")
	}
}

func TestNewTokenManagerStartsEmptyAndWritableWhenFileIsMissing(t *testing.T) {
	dir := t.TempDir()
	tm, err := NewTokenManager(newTokenStoreTestVault(t, dir, "a"), filepath.Join(dir, "tokens.bin"))
	if err != nil {
		t.Fatalf("NewTokenManager(missing file): %v", err)
	}
	if tm.LoadError() != nil {
		t.Fatalf("LoadError() = %v, want nil", tm.LoadError())
	}
	if _, _, err := tm.Create("first", []string{"webhook"}, nil); err != nil {
		t.Fatalf("Create on fresh store: %v", err)
	}
}

func TestTokenManagerUpdateAndDeleteRollBackWhenSaveFails(t *testing.T) {
	dir := t.TempDir()
	tm, err := NewTokenManager(newTokenStoreTestVault(t, dir, "a"), filepath.Join(dir, "tokens.bin"))
	if err != nil {
		t.Fatalf("NewTokenManager: %v", err)
	}
	_, meta, err := tm.Create("original", []string{"webhook"}, nil)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	tm.filePath = filepath.Join(dir, "missing-dir", "tokens.bin") // every save now fails

	if err := tm.Update(meta.ID, "renamed", false); err == nil {
		t.Fatal("Update succeeded although save failed")
	}
	got, err := tm.Get(meta.ID)
	if err != nil || got.Name != "original" || !got.Enabled {
		t.Fatalf("Update kept unsaved changes: %+v, %v", got, err)
	}
	if err := tm.Delete(meta.ID); err == nil {
		t.Fatal("Delete succeeded although save failed")
	}
	if tm.Count() != 1 {
		t.Fatalf("Delete removed the token although save failed (count %d)", tm.Count())
	}
	if err := tm.Delete("no-such-id"); !errors.Is(err, ErrTokenNotFound) {
		t.Fatalf("Delete(unknown) error = %v, want ErrTokenNotFound", err)
	}
}

func TestRetiredTokenManagerRefusesWrites(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tokens.bin")
	tm, err := NewTokenManager(newTokenStoreTestVault(t, dir, "a"), path)
	if err != nil {
		t.Fatalf("NewTokenManager: %v", err)
	}
	_, meta, err := tm.Create("kept", []string{"webhook"}, nil)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	before, _ := os.ReadFile(path)
	tm.Retire()
	if err := tm.Update(meta.ID, "renamed", true); !errors.Is(err, ErrTokenStoreUnavailable) {
		t.Fatalf("Update on retired manager error = %v, want ErrTokenStoreUnavailable", err)
	}
	tm.TouchLastUsed(meta.ID)
	if after, _ := os.ReadFile(path); !bytes.Equal(before, after) {
		t.Fatal("retired manager rewrote the token file")
	}
}
