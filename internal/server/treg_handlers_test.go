package server

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/security"
)

func TestTregAdminRoutesAndLocalStatus(t *testing.T) {
	cfg := &config.Config{}
	cfg.Treg = config.DefaultTregConfig()
	cfg.WebConfig.Enabled = true
	cfg.Auth.Enabled = true
	cfg.Auth.SessionSecret = "treg-fixture-session"
	s := &Server{Cfg: cfg, Logger: slog.Default()}
	mux := http.NewServeMux()
	s.registerConfigAPIRoutes(mux, nil)
	for _, path := range []string{"status", "catalog", "endpoint", "balance", "test-connection"} {
		method := "GET"
		if path == "test-connection" {
			method = "POST"
		}
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(method, "/api/treg/"+path, nil))
		if rec.Code != http.StatusUnauthorized && rec.Code != http.StatusForbidden {
			t.Fatalf("admin route %s allowed: %d", path, rec.Code)
		}
	}
	vault, err := security.NewVault(strings.Repeat("a", 64), filepath.Join(t.TempDir(), "vault.bin"))
	if err != nil {
		t.Fatal(err)
	}
	s.Vault = vault
	const token = "treg-local-fixture-secret"
	if err := vault.WriteSecret("treg_token", token); err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	handleTreg(s).ServeHTTP(rec, httptest.NewRequest("GET", "/api/treg/status", nil))
	var status map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &status)
	if rec.Code != 200 || status["status"] != "disabled" || status["key_present"] != true || strings.Contains(rec.Body.String(), token) {
		t.Fatalf("bad local status %s", rec.Body)
	}
	rec = httptest.NewRecorder()
	handleTreg(s).ServeHTTP(rec, httptest.NewRequest("POST", "/api/treg/test-connection", strings.NewReader(`{"token":"untrusted","base_url":"http://localhost"}`)))
	if rec.Code != 403 {
		t.Fatalf("test bypassed network gate: %d", rec.Code)
	}
	raw := map[string]interface{}{}
	cfg.Treg.Token = token
	injectTregDefaults(raw, cfg)
	injectVaultIndicators(raw, vault)
	b, _ := json.Marshal(raw)
	if strings.Contains(string(b), token) || raw["treg"].(map[string]interface{})["max_call_cost_micro"] != float64(1_000_000) {
		t.Fatalf("config secret/default failure: %s", b)
	}
}
