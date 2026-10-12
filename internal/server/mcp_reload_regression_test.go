package server

import (
	"aurago/internal/config"
	"aurago/internal/security"
	"aurago/internal/tools"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMCPSecretReloadStartsOnceWithUpdatedVault(t *testing.T) {
	s := auditMCPServer(t)
	s.Cfg.ConfigPath = filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(s.Cfg.ConfigPath, []byte("agent:\n  allow_mcp: true\nmcp:\n  enabled: true\n"), 0600); err != nil {
		t.Fatal(err)
	}
	vault, err := security.NewVault(strings.Repeat("b", 64), filepath.Join(t.TempDir(), "vault.bin"))
	if err != nil {
		t.Fatal(err)
	}
	s.Vault = vault
	oldInit, oldStop := initExternalMCPManager, shutdownExternalMCPManager
	defer func() { initExternalMCPManager, shutdownExternalMCPManager = oldInit, oldStop }()
	starts, stops := 0, 0
	shutdownExternalMCPManager = func() uint64 { stops++; return 1 }
	initExternalMCPManager = func([]tools.MCPServerConfig, *slog.Logger, uint64) *tools.MCPManager {
		starts++
		value, err := vault.ReadSecret(mcpSecretVaultKey("test"))
		if err != nil || value != "fixture-value" {
			t.Fatal("reload saw stale secret")
		}
		return nil
	}
	w := httptest.NewRecorder()
	handlePutMCPSecrets(s, w, httptest.NewRequest(http.MethodPut, "/api/mcp-secrets", strings.NewReader(`{"secrets":[{"alias":"test","value":"fixture-value"}]}`)))
	if w.Code != http.StatusOK || starts != 1 || stops != 1 {
		t.Fatalf("status=%d starts=%d stops=%d", w.Code, starts, stops)
	}
}

func auditMCPServer(t *testing.T) *Server {
	t.Helper()
	cfg := &config.Config{}
	cfg.MCPServer.Enabled = true
	cfg.Directories.ToolsDir = t.TempDir()
	cfg.Directories.SkillsDir = t.TempDir()
	return &Server{Cfg: cfg, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
}

func TestRegressionMCPDograhOnlyReloadDropsRuntime(t *testing.T) {
	oldInit, oldShutdown := initExternalMCPManager, shutdownExternalMCPManager
	defer func() { initExternalMCPManager, shutdownExternalMCPManager = oldInit, oldShutdown }()
	starts, stops := 0, 0
	initExternalMCPManager = func([]tools.MCPServerConfig, *slog.Logger, uint64) *tools.MCPManager { starts++; return nil }
	shutdownExternalMCPManager = func() uint64 { stops++; return 0 }
	cfg := &config.Config{}
	cfg.Agent.AllowMCP, cfg.MCP.Enabled = true, true
	cfg.Dograh.Enabled, cfg.Dograh.MCPClientEnabled = true, true
	cfg.Dograh.APIURL, cfg.Dograh.APIKey = "http://127.0.0.1:8000", "fixture"
	if len(buildRuntimeMCPConfigs(cfg, nil, nil)) != 1 {
		t.Fatal("fixture missing auto Dograh")
	}
	syncExternalMCPRuntime(cfg, nil, slog.New(slog.NewTextHandler(io.Discard, nil)), shutdownExternalMCPManager())
	if stops != 1 || starts != 1 {
		t.Fatalf("unexpected stops=%d starts=%d", stops, starts)
	}
}

func TestRegressionMCPPersistenceHoldsGlobalConfigLockDuringConnect(t *testing.T) {
	s := auditMCPServer(t)
	s.Cfg.ConfigPath = filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(s.Cfg.ConfigPath, []byte("agent:\n  allow_mcp: true\nmcp:\n  enabled: true\n  servers:\n    - name: fixture\n      command: unused\n      enabled: true\n"), 0600); err != nil {
		t.Fatal(err)
	}
	vault, err := security.NewVault(strings.Repeat("a", 64), filepath.Join(t.TempDir(), "vault.bin"))
	if err != nil {
		t.Fatal(err)
	}
	s.Vault = vault
	oldInit, oldShutdown := initExternalMCPManager, shutdownExternalMCPManager
	defer func() { initExternalMCPManager, shutdownExternalMCPManager = oldInit, oldShutdown }()
	locked := false
	initExternalMCPManager = func([]tools.MCPServerConfig, *slog.Logger, uint64) *tools.MCPManager {
		acquired := s.CfgMu.TryRLock()
		if acquired {
			s.CfgMu.RUnlock()
		}
		locked = !acquired
		return nil
	}
	shutdownExternalMCPManager = func() uint64 { return 0 }
	if err := persistMCPSectionUpdate(s, func(map[string]interface{}) error { return nil }); err != nil {
		t.Fatal(err)
	}
	if locked {
		t.Fatal("global configuration lock held during connection")
	}
}
