package server

import (
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

	"aurago/internal/config"
)

func TestHandlePutMCPServersInvalidJSONIsGeneric(t *testing.T) {
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

func TestHandlePutMCPServersReloadsVaultBackedAuthSecrets(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte("auth:\n  enabled: true\nmcp:\n  enabled: false\n  servers: []\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	vaultPath := filepath.Join(tmpDir, "vault.bin")
	vault, err := security.NewVault(strings.Repeat("b", 64), vaultPath)
	if err != nil {
		t.Fatalf("new vault: %v", err)
	}
	passwordHash, err := HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	if err := vault.WriteSecret("auth_password_hash", passwordHash); err != nil {
		t.Fatalf("write auth hash: %v", err)
	}

	s := &Server{
		Cfg:    &config.Config{ConfigPath: configPath},
		Logger: logger,
		Vault:  vault,
	}

	req := httptest.NewRequest(http.MethodPut, "/api/mcp-servers", strings.NewReader(`{
		"enabled": true,
		"servers": [{
			"name": "minimax",
			"command": "uvx",
			"args": ["minimax-coding-plan-mcp"],
			"env": {"MINIMAX_API_HOST": "https://api.minimax.io"},
			"enabled": true
		}]
	}`))
	rec := httptest.NewRecorder()

	handlePutMCPServers(s, rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if !s.Cfg.Auth.Enabled {
		t.Fatalf("auth.enabled = false, want true")
	}
	if s.Cfg.Auth.PasswordHash != passwordHash {
		t.Fatalf("password hash not reloaded from vault; got %q", s.Cfg.Auth.PasswordHash)
	}
}

func TestHandlePutMCPServersReinitializesRuntimeManager(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte("agent:\n  allow_mcp: true\nmcp:\n  enabled: false\n  servers: []\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	vaultPath := filepath.Join(tmpDir, "vault.bin")
	vault, err := security.NewVault(strings.Repeat("c", 64), vaultPath)
	if err != nil {
		t.Fatalf("new vault: %v", err)
	}

	var (
		shutdownCalls int
		initCalls     int
		gotConfigs    []tools.MCPServerConfig
	)
	oldInit := initExternalMCPManager
	oldShutdown := shutdownExternalMCPManager
	initExternalMCPManager = func(configs []tools.MCPServerConfig, _ *slog.Logger) *tools.MCPManager {
		initCalls++
		gotConfigs = append([]tools.MCPServerConfig(nil), configs...)
		return nil
	}
	shutdownExternalMCPManager = func() {
		shutdownCalls++
	}
	defer func() {
		initExternalMCPManager = oldInit
		shutdownExternalMCPManager = oldShutdown
	}()

	s := &Server{
		Cfg:    &config.Config{ConfigPath: configPath},
		Logger: logger,
		Vault:  vault,
	}

	req := httptest.NewRequest(http.MethodPut, "/api/mcp-servers", strings.NewReader(`{
		"enabled": true,
		"servers": [{
			"name": "minimax",
			"command": "uvx",
			"args": ["minimax-coding-plan-mcp"],
			"env": {"MINIMAX_API_HOST": "https://api.minimax.io"},
			"enabled": true
		}]
	}`))
	rec := httptest.NewRecorder()

	handlePutMCPServers(s, rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if shutdownCalls != 1 {
		t.Fatalf("shutdown calls = %d, want 1", shutdownCalls)
	}
	if initCalls != 1 {
		t.Fatalf("init calls = %d, want 1", initCalls)
	}
	if len(gotConfigs) != 1 {
		t.Fatalf("config count = %d, want 1", len(gotConfigs))
	}
	if gotConfigs[0].Name != "minimax" || gotConfigs[0].Command != "uvx" {
		t.Fatalf("unexpected runtime config: %+v", gotConfigs[0])
	}
}

func TestHandlePutMCPServersPreservesHiddenSecurityFields(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	initial := `agent:
  allow_mcp: true
mcp:
  enabled: true
  servers:
    - name: minimax
      command: uvx
      args: ["old-package"]
      enabled: true
      allowed_tools:
        - understand_image
        - text_to_audio
      allow_destructive: true
`
	if err := os.WriteFile(configPath, []byte(initial), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	vaultPath := filepath.Join(tmpDir, "vault.bin")
	vault, err := security.NewVault(strings.Repeat("9", 64), vaultPath)
	if err != nil {
		t.Fatalf("new vault: %v", err)
	}

	oldInit := initExternalMCPManager
	oldShutdown := shutdownExternalMCPManager
	initExternalMCPManager = func(configs []tools.MCPServerConfig, _ *slog.Logger) *tools.MCPManager {
		return nil
	}
	shutdownExternalMCPManager = func() {}
	defer func() {
		initExternalMCPManager = oldInit
		shutdownExternalMCPManager = oldShutdown
	}()

	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	cfg.ConfigPath = configPath
	s := &Server{
		Cfg:    cfg,
		Logger: logger,
		Vault:  vault,
	}

	req := httptest.NewRequest(http.MethodPut, "/api/mcp-servers", strings.NewReader(`{
		"enabled": true,
		"servers": [{
			"name": "minimax",
			"command": "uvx",
			"args": ["new-package"],
			"enabled": true
		}]
	}`))
	rec := httptest.NewRecorder()

	handlePutMCPServers(s, rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	reloaded, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("reload config: %v", err)
	}
	if len(reloaded.MCP.Servers) != 1 {
		t.Fatalf("server count = %d, want 1", len(reloaded.MCP.Servers))
	}
	got := reloaded.MCP.Servers[0]
	if strings.Join(got.AllowedTools, ",") != "understand_image,text_to_audio" {
		t.Fatalf("AllowedTools = %#v, want preserved values", got.AllowedTools)
	}
	if !got.AllowDestructive {
		t.Fatal("AllowDestructive was not preserved")
	}
}

func TestHandlePutMCPServersAllowsClearingSecurityFields(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	initial := `agent:
  allow_mcp: true
mcp:
  enabled: true
  servers:
    - name: minimax
      command: uvx
      enabled: true
      allowed_tools:
        - understand_image
      allow_destructive: true
`
	if err := os.WriteFile(configPath, []byte(initial), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	vaultPath := filepath.Join(tmpDir, "vault.bin")
	vault, err := security.NewVault(strings.Repeat("8", 64), vaultPath)
	if err != nil {
		t.Fatalf("new vault: %v", err)
	}

	oldInit := initExternalMCPManager
	oldShutdown := shutdownExternalMCPManager
	initExternalMCPManager = func(configs []tools.MCPServerConfig, _ *slog.Logger) *tools.MCPManager {
		return nil
	}
	shutdownExternalMCPManager = func() {}
	defer func() {
		initExternalMCPManager = oldInit
		shutdownExternalMCPManager = oldShutdown
	}()

	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	cfg.ConfigPath = configPath
	s := &Server{Cfg: cfg, Logger: logger, Vault: vault}

	req := httptest.NewRequest(http.MethodPut, "/api/mcp-servers", strings.NewReader(`{
		"enabled": true,
		"servers": [{
			"name": "minimax",
			"command": "uvx",
			"enabled": true,
			"allowed_tools": [],
			"allow_destructive": false
		}]
	}`))
	rec := httptest.NewRecorder()

	handlePutMCPServers(s, rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	reloaded, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("reload config: %v", err)
	}
	got := reloaded.MCP.Servers[0]
	if len(got.AllowedTools) != 0 {
		t.Fatalf("AllowedTools = %#v, want empty after explicit clear", got.AllowedTools)
	}
	if got.AllowDestructive {
		t.Fatal("AllowDestructive should be false after explicit clear")
	}
}

func TestHandlePutMCPPreferencesPersistsCapabilitySelections(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	initial := "agent:\n  allow_mcp: true\nmcp:\n  enabled: true\n  servers:\n    - name: minimax\n      command: uvx\n      enabled: true\n"
	if err := os.WriteFile(configPath, []byte(initial), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	vaultPath := filepath.Join(tmpDir, "vault.bin")
	vault, err := security.NewVault(strings.Repeat("d", 64), vaultPath)
	if err != nil {
		t.Fatalf("new vault: %v", err)
	}

	oldInit := initExternalMCPManager
	oldShutdown := shutdownExternalMCPManager
	initExternalMCPManager = func(configs []tools.MCPServerConfig, _ *slog.Logger) *tools.MCPManager {
		return nil
	}
	shutdownExternalMCPManager = func() {}
	defer func() {
		initExternalMCPManager = oldInit
		shutdownExternalMCPManager = oldShutdown
	}()

	s := &Server{
		Cfg:    &config.Config{ConfigPath: configPath},
		Logger: logger,
		Vault:  vault,
	}

	req := httptest.NewRequest(http.MethodPut, "/api/mcp-preferences", strings.NewReader(`{
		"web_search": {"server": "minimax", "tool": "web_search"},
		"vision": {"server": "minimax", "tool": "image_analysis"}
	}`))
	rec := httptest.NewRecorder()

	handlePutMCPPreferences(s, rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("reload config: %v", err)
	}
	if cfg.MCP.PreferredCapabilities.WebSearch.Server != "minimax" || cfg.MCP.PreferredCapabilities.WebSearch.Tool != "web_search" {
		t.Fatalf("unexpected web_search preference: %+v", cfg.MCP.PreferredCapabilities.WebSearch)
	}
	if cfg.MCP.PreferredCapabilities.Vision.Server != "minimax" || cfg.MCP.PreferredCapabilities.Vision.Tool != "image_analysis" {
		t.Fatalf("unexpected vision preference: %+v", cfg.MCP.PreferredCapabilities.Vision)
	}
}

func TestHandlePutMCPSecretsPersistsMetadataAndVaultValue(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	initial := "agent:\n  allow_mcp: true\nmcp:\n  enabled: true\n  secrets: []\n  servers: []\n"
	if err := os.WriteFile(configPath, []byte(initial), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	vaultPath := filepath.Join(tmpDir, "vault.bin")
	vault, err := security.NewVault(strings.Repeat("e", 64), vaultPath)
	if err != nil {
		t.Fatalf("new vault: %v", err)
	}
	s := &Server{
		Cfg:    &config.Config{ConfigPath: configPath},
		Logger: logger,
		Vault:  vault,
	}

	req := httptest.NewRequest(http.MethodPut, "/api/mcp-secrets", strings.NewReader(`{
		"secrets": [{
			"alias": "api-token",
			"label": "MiniMax API",
			"description": "main key",
			"value": "secret-value"
		}]
	}`))
	rec := httptest.NewRecorder()

	handlePutMCPSecrets(s, rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("reload config: %v", err)
	}
	if len(cfg.MCP.Secrets) != 1 || cfg.MCP.Secrets[0].Alias != "api-token" {
		t.Fatalf("unexpected MCP secrets config: %+v", cfg.MCP.Secrets)
	}
	if got, err := vault.ReadSecret("mcp_secret_api-token"); err != nil || got != "secret-value" {
		t.Fatalf("vault secret = %q, err=%v", got, err)
	}
}

func TestBuildRuntimeMCPConfigsResolvesSafeWorkdirAndSecrets(t *testing.T) {
	workspaceDir := t.TempDir()
	cfg := &config.Config{}
	cfg.Directories.WorkspaceDir = workspaceDir
	cfg.MCP.Secrets = []config.MCPSecret{{Alias: "api-token", Label: "MiniMax"}}
	cfg.MCP.Servers = []config.MCPServer{{
		Name:             "minimax",
		Command:          "uvx",
		Enabled:          true,
		Runtime:          "docker",
		HostWorkdir:      "../unsafe",
		ContainerWorkdir: "/workspace",
		Env:              map[string]string{"API_KEY": "{{api-token}}"},
		AllowedTools:     []string{"understand_image"},
		AllowDestructive: true,
	}}

	vaultPath := filepath.Join(t.TempDir(), "vault.bin")
	vault, err := security.NewVault(strings.Repeat("f", 64), vaultPath)
	if err != nil {
		t.Fatalf("new vault: %v", err)
	}
	if err := vault.WriteSecret("mcp_secret_api-token", "secret-value"); err != nil {
		t.Fatalf("WriteSecret() error = %v", err)
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	configs := buildRuntimeMCPConfigs(cfg, vault, logger)
	if len(configs) != 1 {
		t.Fatalf("len(configs) = %d, want 1", len(configs))
	}
	if got := configs[0].Secrets["api-token"]; got != "secret-value" {
		t.Fatalf("resolved secret = %q, want secret-value", got)
	}
	expectedRoot := filepath.Join(workspaceDir, "mcp", "minimax")
	if configs[0].HostWorkdir != expectedRoot {
		t.Fatalf("HostWorkdir = %q, want %q", configs[0].HostWorkdir, expectedRoot)
	}
	if len(configs[0].AllowedTools) != 1 || configs[0].AllowedTools[0] != "understand_image" {
		t.Fatalf("AllowedTools = %#v, want understand_image", configs[0].AllowedTools)
	}
	if !configs[0].AllowDestructive {
		t.Fatal("AllowDestructive was not copied into runtime MCP config")
	}
}
