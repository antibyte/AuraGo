package server

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/config"
)

func TestHomeDashConfigValidationRejectsBadValuesBeforeWrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	original := []byte("server:\n  host: 127.0.0.1\nhomedash:\n  enabled: true\n")
	if err := os.WriteFile(path, original, 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConfigPath = path
	s := &Server{Cfg: cfg, Logger: slog.Default()}
	patch := `{"homedash":{"limits":{"max_streams":1}}}`
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(patch), &m); err != nil {
		t.Fatal(err)
	}
	if _, err := applyConfigPatch(s, m); err == nil || !strings.Contains(err.Error(), "homedash:") {
		t.Fatalf("setup patch: %v", err)
	}
	rec := httptest.NewRecorder()
	handleUpdateConfig(s).ServeHTTP(rec, httptest.NewRequest(http.MethodPut, "/api/config", strings.NewReader(patch)))
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "homedash:") {
		t.Fatalf("config save: %d %s", rec.Code, rec.Body.String())
	}
	stored, err := os.ReadFile(path)
	if err != nil || string(stored) != string(original) {
		t.Fatalf("a rejected patch changed the config file: %v", err)
	}
}
