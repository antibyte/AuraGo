package server

import (
	"bytes"
	"io/fs"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/i18n"
	"aurago/ui"
)

func TestUILanguageRequiresSessionAndPreservesOtherConfig(t *testing.T) {
	uiFS, err := fs.Sub(ui.Content, ".")
	if err != nil {
		t.Fatal(err)
	}
	i18n.Load(uiFS, slog.Default())

	path := filepath.Join(t.TempDir(), "config.yaml")
	initial := "server:\n  ui_language: en\nauth:\n  enabled: true\n"
	if err := os.WriteFile(path, []byte(initial), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{ConfigPath: path}
	cfg.Server.UILanguage = "en"
	cfg.Auth.Enabled = true
	cfg.Auth.PasswordHash = "configured"
	cfg.Auth.SessionSecret = strings.Repeat("a", 32)
	s := &Server{Cfg: cfg, Logger: slog.Default()}
	handler := authMiddleware(s, handleUILanguage(s))

	request := func(language string, authenticated bool) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(http.MethodPut, "http://example.com/api/ui-language", bytes.NewBufferString(`{"language":"`+language+`"}`))
		req.Header.Set("Origin", "http://example.com")
		if authenticated {
			req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: createSessionValue(cfg.Auth.SessionSecret, time.Now().Add(time.Hour))})
		}
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec
	}

	if rec := request("de", false); rec.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated status = %d, want 401", rec.Code)
	}
	if rec := request("unknown", true); rec.Code != http.StatusBadRequest {
		t.Fatalf("unsupported language status = %d, want 400", rec.Code)
	}
	if rec := request("de", true); rec.Code != http.StatusOK {
		t.Fatalf("authenticated status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(data, []byte("ui_language: de")) || !bytes.Contains(data, []byte("enabled: true")) {
		t.Fatalf("unexpected saved config: %s", data)
	}
	if s.ConfigSnapshot().Server.UILanguage != "de" {
		t.Fatal("live language was not published")
	}
}
