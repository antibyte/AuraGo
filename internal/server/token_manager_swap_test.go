package server

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/security"
)

func TestReplaceTokenManagerRetiresPreviousStore(t *testing.T) {
	dir := t.TempDir()
	vault, err := security.NewVault(strings.Repeat("e", 64), filepath.Join(dir, "vault.bin"))
	if err != nil {
		t.Fatalf("NewVault: %v", err)
	}
	oldPath := filepath.Join(dir, "old-tokens.bin")
	oldTM, err := security.NewTokenManager(vault, oldPath)
	if err != nil {
		t.Fatalf("NewTokenManager(old): %v", err)
	}
	if _, _, err := oldTM.Create("old", []string{"webhook"}, nil); err != nil {
		t.Fatalf("Create(old): %v", err)
	}
	before, _ := os.ReadFile(oldPath)
	newTM, err := security.NewTokenManager(vault, filepath.Join(dir, "new-tokens.bin"))
	if err != nil {
		t.Fatalf("NewTokenManager(new): %v", err)
	}

	s := &Server{TokenManager: oldTM}
	s.replaceTokenManager(newTM)

	if s.currentTokenManager() != newTM {
		t.Fatal("currentTokenManager did not return the replacement")
	}
	if _, _, err := oldTM.Create("late", []string{"webhook"}, nil); !errors.Is(err, security.ErrTokenStoreUnavailable) {
		t.Fatalf("retired manager Create error = %v, want ErrTokenStoreUnavailable", err)
	}
	if after, _ := os.ReadFile(oldPath); !bytes.Equal(before, after) {
		t.Fatal("retired manager rewrote its token file")
	}
}

func TestTokenAdminReportsReadOnlyStore(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tokens.bin")
	owner, err := security.NewVault(strings.Repeat("e", 64), filepath.Join(dir, "vault-e.bin"))
	if err != nil {
		t.Fatalf("NewVault(owner): %v", err)
	}
	tm, err := security.NewTokenManager(owner, path)
	if err != nil {
		t.Fatalf("NewTokenManager(owner): %v", err)
	}
	if _, _, err := tm.Create("existing", []string{"webhook"}, nil); err != nil {
		t.Fatalf("Create: %v", err)
	}
	wrongKey, err := security.NewVault(strings.Repeat("f", 64), filepath.Join(dir, "vault-f.bin"))
	if err != nil {
		t.Fatalf("NewVault(wrong): %v", err)
	}
	degraded, err := security.NewTokenManager(wrongKey, path)
	if !errors.Is(err, security.ErrTokenStoreUnavailable) {
		t.Fatalf("NewTokenManager(wrong key) error = %v, want ErrTokenStoreUnavailable", err)
	}

	rec := httptest.NewRecorder()
	handleCreateToken(degraded).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/tokens", strings.NewReader(`{"name":"new"}`)))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("create on read-only store status = %d, want 503; body=%s", rec.Code, rec.Body.String())
	}
	rec = httptest.NewRecorder()
	handleDeleteToken(tm).ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/api/tokens/no-such-id", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("delete unknown token status = %d, want 404", rec.Code)
	}
}
