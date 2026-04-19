package server

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/security"
)

func TestHandleVercelStatusDisabled(t *testing.T) {
	s := &Server{Cfg: &config.Config{}}

	req := httptest.NewRequest(http.MethodGet, "/api/vercel/status", nil)
	rec := httptest.NewRecorder()

	handleVercelStatus(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"status":"disabled"`) {
		t.Fatalf("unexpected body: %s", rec.Body.String())
	}
}

func TestHandleVercelTestConnectionMissingToken(t *testing.T) {
	const masterKey = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

	vault, err := security.NewVault(masterKey, filepath.Join(t.TempDir(), "vault.bin"))
	if err != nil {
		t.Fatalf("NewVault() error = %v", err)
	}

	s := &Server{
		Cfg:   &config.Config{},
		Vault: vault,
	}

	req := httptest.NewRequest(http.MethodPost, "/api/vercel/test-connection", strings.NewReader(`{}`))
	rec := httptest.NewRecorder()

	handleVercelTestConnection(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"status":"error"`) || !strings.Contains(rec.Body.String(), `vercel_token`) {
		t.Fatalf("unexpected body: %s", rec.Body.String())
	}
}
