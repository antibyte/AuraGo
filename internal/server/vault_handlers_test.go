package server

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/security"
)

func TestHandleSetVaultSecretInvalidJSONIsGeneric(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodPost, "/api/vault/secrets", strings.NewReader(`{"key":`))
	rec := httptest.NewRecorder()

	handleSetVaultSecret(&Server{}, rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
	if body := rec.Body.String(); !strings.Contains(body, "Invalid JSON") || strings.Contains(strings.ToLower(body), "unexpected eof") {
		t.Fatalf("expected generic invalid JSON response, got %q", body)
	}
}

func TestHandleSetVaultSecretCanonicalizesMappedConfigPath(t *testing.T) {
	vault, err := security.NewVault(strings.Repeat("a", 64), filepath.Join(t.TempDir(), "vault.bin"))
	if err != nil {
		t.Fatalf("NewVault() error = %v", err)
	}
	s := &Server{
		Cfg:    &config.Config{},
		Vault:  vault,
		Logger: slog.Default(),
	}

	req := httptest.NewRequest(http.MethodPost, "/api/vault/secrets", strings.NewReader(`{"key":"composio.api_key","value":"cmp-secret"}`))
	rec := httptest.NewRecorder()
	handleSetVaultSecret(s, rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if s.Cfg.Composio.APIKey != "cmp-secret" {
		t.Fatalf("live composio api key = %q, want cmp-secret", s.Cfg.Composio.APIKey)
	}
	if secret, err := vault.ReadSecret("composio_api_key"); err != nil || secret != "cmp-secret" {
		t.Fatalf("canonical vault secret = %q, err=%v", secret, err)
	}
	if _, err := vault.ReadSecret("composio.api_key"); err == nil {
		t.Fatal("expected dotted config path not to be stored as a separate vault key")
	}
}

func TestHandleDeleteVaultSecretRemovesMappedConfigPathAlias(t *testing.T) {
	vault, err := security.NewVault(strings.Repeat("b", 64), filepath.Join(t.TempDir(), "vault.bin"))
	if err != nil {
		t.Fatalf("NewVault() error = %v", err)
	}
	if err := vault.WriteSecret("composio_api_key", "canonical"); err != nil {
		t.Fatalf("WriteSecret canonical: %v", err)
	}
	if err := vault.WriteSecret("composio.api_key", "legacy"); err != nil {
		t.Fatalf("WriteSecret legacy: %v", err)
	}
	s := &Server{
		Cfg:    &config.Config{},
		Vault:  vault,
		Logger: slog.Default(),
	}

	req := httptest.NewRequest(http.MethodDelete, "/api/vault/secrets?key=composio.api_key", nil)
	rec := httptest.NewRecorder()
	handleDeleteVaultSecret(s, rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if _, err := vault.ReadSecret("composio_api_key"); err == nil {
		t.Fatal("expected canonical composio secret to be deleted")
	}
	if _, err := vault.ReadSecret("composio.api_key"); err == nil {
		t.Fatal("expected legacy dotted composio secret to be deleted")
	}
}
