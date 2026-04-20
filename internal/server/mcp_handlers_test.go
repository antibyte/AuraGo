package server

import (
	"aurago/internal/security"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/config"
)

func TestHandlePutMCPServersInvalidJSONIsGeneric(t *testing.T) {
	t.Parallel()

	s := &Server{
		Cfg: &config.Config{ConfigPath: "config.yaml"},
	}

	req := httptest.NewRequest(http.MethodPut, "/api/mcp-servers", strings.NewReader(`{"broken":`))
	rec := httptest.NewRecorder()

	handlePutMCPServers(s, rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Invalid JSON") || strings.Contains(strings.ToLower(body), "unexpected eof") {
		t.Fatalf("expected generic invalid JSON response, got %q", body)
	}
}

func TestHandlePutMCPServersPersistsEnabledAndAllFields(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte("mcp:\n  enabled: false\n  servers: []\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	vaultPath := filepath.Join(tmpDir, "vault.bin")
	vault, err := security.NewVault(strings.Repeat("a", 64), vaultPath)
	if err != nil {
		t.Fatalf("new vault: %v", err)
	}
	s := &Server{
		Cfg:    &config.Config{ConfigPath: configPath},
		Logger: logger,
		Vault:  vault,
	}

	req := httptest.NewRequest(http.MethodPut, "/api/mcp-servers", strings.NewReader(`{
		"enabled": true,
		"servers": [{
			"name": "demo",
			"command": "npx",
			"args": ["-y", "@demo/server"],
			"env": {"API_KEY": "secret", "MODE": "debug"},
			"enabled": true
		}]
	}`))
	rec := httptest.NewRecorder()

	handlePutMCPServers(s, rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("reload config: %v", err)
	}
	if !cfg.MCP.Enabled {
		t.Fatalf("mcp.enabled = false, want true")
	}
	if len(cfg.MCP.Servers) != 1 {
		t.Fatalf("server count = %d, want 1", len(cfg.MCP.Servers))
	}
	got := cfg.MCP.Servers[0]
	if got.Name != "demo" || got.Command != "npx" || !got.Enabled {
		t.Fatalf("unexpected server basics: %+v", got)
	}
	if len(got.Args) != 2 || got.Args[0] != "-y" || got.Args[1] != "@demo/server" {
		t.Fatalf("args = %#v, want full roundtrip", got.Args)
	}
	if got.Env["API_KEY"] != "secret" || got.Env["MODE"] != "debug" {
		t.Fatalf("env = %#v, want full roundtrip", got.Env)
	}
}
