package server

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/security"
)

// A config file with blank RuView values loads with the defaults, so an
// unrelated Config UI save must not be rejected by the RuView validation.
// Explicit invalid values stay rejected.
func TestHandleUpdateConfigNormalizesBlankRuViewValues(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "config.yaml")
	configYAML := "budget: {enabled: false, daily_limit_usd: 5}\nruview:\n  mode: \"\"\n  source: \"\"\n  http_port: 0\n  udp_port: 0\n  storage_mb: 0\n"
	if err := os.WriteFile(path, []byte(configYAML), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	cfg.ConfigPath = path
	vault, err := security.NewVault(strings.Repeat("7c", 32), filepath.Join(root, "vault.bin"))
	if err != nil {
		t.Fatal(err)
	}
	s := &Server{Cfg: cfg, Vault: vault, Logger: slog.Default()}

	put := httptest.NewRecorder()
	handleUpdateConfig(s).ServeHTTP(put, httptest.NewRequest(http.MethodPut, "/api/config", strings.NewReader(`{"budget":{"daily_limit_usd":7}}`)))
	if put.Code != http.StatusOK {
		t.Fatalf("unrelated PUT: %d %s", put.Code, put.Body.String())
	}
	if live := s.ConfigSnapshot(); live.Budget.DailyLimitUSD != 7 || live.RuView.Mode != "docker" {
		t.Fatalf("live config after PUT: daily_limit=%v ruview.mode=%q", live.Budget.DailyLimitUSD, live.RuView.Mode)
	}

	put = httptest.NewRecorder()
	handleUpdateConfig(s).ServeHTTP(put, httptest.NewRequest(http.MethodPut, "/api/config", strings.NewReader(`{"ruview":{"mode":"kubernetes"}}`)))
	if put.Code != http.StatusBadRequest || !strings.Contains(put.Body.String(), "RuView mode must be docker, external or native") {
		t.Fatalf("invalid mode PUT: %d %s", put.Code, put.Body.String())
	}
	reloaded, err := config.Load(path)
	if err != nil {
		t.Fatalf("reload after rejected PUT: %v", err)
	}
	if reloaded.RuView.Mode != "docker" {
		t.Fatalf("rejected PUT changed the file: ruview.mode=%q", reloaded.RuView.Mode)
	}
}
