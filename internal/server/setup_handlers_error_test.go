package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"aurago/internal/config"
)

func TestHandleSetupSaveInvalidJSONIsGeneric(t *testing.T) {
	t.Parallel()

	// Set a known CSRF token for this test.
	setupCSRFMu.Lock()
	setupCSRFToken = "test-token"
	setupCSRFMu.Unlock()

	s := &Server{
		Cfg: &config.Config{ConfigPath: t.TempDir() + "\\config.yaml"},
	}
	if err := os.WriteFile(s.Cfg.ConfigPath, []byte("server:\n  ui_language: de\n"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/setup/save", strings.NewReader(`{"broken":`))
	req.Header.Set("X-CSRF-Token", "test-token")
	rec := httptest.NewRecorder()

	handleSetupSave(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Invalid JSON") || strings.Contains(strings.ToLower(body), "unexpected eof") {
		t.Fatalf("expected generic invalid JSON response, got %q", body)
	}
}
