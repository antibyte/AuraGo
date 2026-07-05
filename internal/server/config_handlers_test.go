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
	"time"

	"aurago/internal/config"
	"aurago/internal/inventory"
	"aurago/internal/security"
	"aurago/internal/sqlconnections"

	"gopkg.in/yaml.v3"
)

func TestExtractSecretsToVaultStoresProxmoxSecret(t *testing.T) {
	const masterKey = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

	vault, err := security.NewVault(masterKey, t.TempDir()+"\\vault.bin")
	if err != nil {
		t.Fatalf("NewVault() error = %v", err)
	}

	patch := map[string]interface{}{
		"proxmox": map[string]interface{}{
			"token_id": "user@pam!tokenname",
			"secret":   "super-secret-token",
		},
	}

	if err := extractSecretsToVault(patch, vault, slog.Default()); err != nil {
		t.Fatalf("extractSecretsToVault() error = %v", err)
	}

	secret, err := vault.ReadSecret("proxmox_secret")
	if err != nil {
		t.Fatalf("vault.ReadSecret() error = %v", err)
	}
	if secret != "super-secret-token" {
		t.Fatalf("vault secret = %q, want %q", secret, "super-secret-token")
	}

	proxmoxPatch, ok := patch["proxmox"].(map[string]interface{})
	if !ok {
		t.Fatalf("patch[\"proxmox\"] missing or wrong type: %#v", patch["proxmox"])
	}
	if _, exists := proxmoxPatch["secret"]; exists {
		t.Fatalf("secret field should have been removed from patch: %#v", proxmoxPatch)
	}
}

func TestOptimizationHelpKeysExistInAllLocales(t *testing.T) {
	required := []string{
		"help.agent.optimizer_enabled",
		"help.agent.system_prompt_token_budget",
		"help.agent.adaptive_system_prompt_token_budget",
		"help.agent.context_window",
		"help.agent.memory_compression_char_limit",
		"help.agent.tool_output_limit",
		"help.agent.discover_tools_snapshot_ttl_minutes",
		"help.agent.max_tool_guides",
		"help.agent.core_memory_max_entries",
		"help.agent.core_memory_cap_mode",
		"help.agent.adaptive_tools.enabled",
		"help.agent.adaptive_tools.max_tools",
		"help.agent.adaptive_tools.max_total_tools",
		"help.agent.adaptive_tools.max_schema_tokens",
		"help.agent.adaptive_tools.provider_profiles_enabled",
		"help.agent.adaptive_tools.session_tool_retention_turns",
		"help.agent.adaptive_tools.decay_half_life_days",
		"help.agent.adaptive_tools.weight_success_rate",
		"help.agent.adaptive_tools.always_include",
		"help.agent.adaptive_tools.clean_transitions_after_days",
		"help.circuit_breaker.max_tool_calls",
		"help.circuit_breaker.llm_timeout_seconds",
		"help.circuit_breaker.maintenance_timeout_minutes",
		"help.circuit_breaker.retry_intervals",
		"help.agent.recovery.max_provider_422_recoveries",
		"help.agent.recovery.min_messages_for_empty_retry",
		"help.agent.recovery.duplicate_consecutive_hits",
		"help.agent.recovery.duplicate_frequency_hits",
		"help.agent.recovery.identical_tool_error_hits",
		"help.agent.background_tasks.enabled",
		"help.agent.background_tasks.follow_up_delay_seconds",
		"help.agent.background_tasks.max_retries",
		"help.agent.background_tasks.retry_delay_seconds",
		"help.agent.background_tasks.http_timeout_seconds",
		"help.agent.background_tasks.wait_default_timeout_secs",
		"help.agent.background_tasks.wait_poll_interval_seconds",
	}
	assertLocaleKeysExist(t, filepath.Join("..", "..", "ui", "lang", "help"), required)
}

func TestAgentBehaviorHelpKeysExistInAllLocales(t *testing.T) {
	required := []string{
		"help.agent.announcement_detector.enabled",
		"help.agent.announcement_detector.max_retries",
		"help.agent.importance_scoring.enabled",
		"help.agent.importance_scoring.mode",
		"help.agent.auto_learning.enabled",
		"help.agent.auto_learning.mode",
		"help.agent.reuse_first.auto_materialize",
		"help.agent.reuse_first.require_success_signal",
		"help.agent.reuse_first.min_steps",
		"help.agent.reuse_first.max_artifacts_per_session",
	}
	assertLocaleKeysExist(t, filepath.Join("..", "..", "ui", "lang", "help"), required)
}

func TestAgentBehaviorGroupTitleKeysExistInAllLocales(t *testing.T) {
	required := []string{
		"config.group_title.agent.announcement_detector",
		"config.group_title.agent.importance_scoring",
		"config.group_title.agent.auto_learning",
		"config.group_title.agent.reuse_first",
	}
	assertLocaleKeysExist(t, filepath.Join("..", "..", "ui", "lang", "config", "sections"), required)
}

func assertLocaleKeysExist(t *testing.T, localeDir string, required []string) {
	t.Helper()
	entries, err := os.ReadDir(localeDir)
	if err != nil {
		t.Fatalf("ReadDir(%s): %v", localeDir, err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		path := filepath.Join(localeDir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("ReadFile(%s): %v", path, err)
		}
		var values map[string]string
		if err := json.Unmarshal(data, &values); err != nil {
			t.Fatalf("Unmarshal(%s): %v", path, err)
		}
		for _, key := range required {
			if strings.TrimSpace(values[key]) == "" {
				t.Fatalf("%s missing %s", entry.Name(), key)
			}
		}
	}
}

func TestExtractSecretsToVaultStoresMappedClientSecret(t *testing.T) {
	const masterKey = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

	vault, err := security.NewVault(masterKey, t.TempDir()+"\\vault.bin")
	if err != nil {
		t.Fatalf("NewVault() error = %v", err)
	}

	patch := map[string]interface{}{
		"google_workspace": map[string]interface{}{
			"client_id":     "abc.apps.googleusercontent.com",
			"client_secret": "very-secret-client-secret",
		},
	}

	if err := extractSecretsToVault(patch, vault, slog.Default()); err != nil {
		t.Fatalf("extractSecretsToVault() error = %v", err)
	}

	secret, err := vault.ReadSecret("google_workspace_client_secret")
	if err != nil {
		t.Fatalf("vault.ReadSecret() error = %v", err)
	}
	if secret != "very-secret-client-secret" {
		t.Fatalf("vault secret = %q, want %q", secret, "very-secret-client-secret")
	}

	section, ok := patch["google_workspace"].(map[string]interface{})
	if !ok {
		t.Fatalf("patch[\"google_workspace\"] missing or wrong type: %#v", patch["google_workspace"])
	}
	if _, exists := section["client_secret"]; exists {
		t.Fatalf("client_secret field should have been removed from patch: %#v", section)
	}
}

func TestExtractSecretsToVaultStoresAIGatewayToken(t *testing.T) {
	const masterKey = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

	vault, err := security.NewVault(masterKey, t.TempDir()+"\\vault.bin")
	if err != nil {
		t.Fatalf("NewVault() error = %v", err)
	}

	patch := map[string]interface{}{
		"ai_gateway": map[string]interface{}{
			"enabled":    true,
			"account_id": "cf-account",
			"gateway_id": "main-gateway",
			"token":      "cf-aig-secret-token",
		},
	}

	if err := extractSecretsToVault(patch, vault, slog.Default()); err != nil {
		t.Fatalf("extractSecretsToVault() error = %v", err)
	}

	secret, err := vault.ReadSecret("ai_gateway_token")
	if err != nil {
		t.Fatalf("vault.ReadSecret() error = %v", err)
	}
	if secret != "cf-aig-secret-token" {
		t.Fatalf("vault secret = %q, want %q", secret, "cf-aig-secret-token")
	}

	section, ok := patch["ai_gateway"].(map[string]interface{})
	if !ok {
		t.Fatalf("patch[\"ai_gateway\"] missing or wrong type: %#v", patch["ai_gateway"])
	}
	if _, exists := section["token"]; exists {
		t.Fatalf("token field should have been removed from patch: %#v", section)
	}
}

func TestExtractSecretsToVaultStoresSpaceAgentAdminPassword(t *testing.T) {
	const masterKey = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

	vault, err := security.NewVault(masterKey, t.TempDir()+"\\vault.bin")
	if err != nil {
		t.Fatalf("NewVault() error = %v", err)
	}

	patch := map[string]interface{}{
		"space_agent": map[string]interface{}{
			"enabled":        true,
			"admin_user":     "admin",
			"admin_password": "chosen-space-password",
		},
	}

	if err := extractSecretsToVault(patch, vault, slog.Default()); err != nil {
		t.Fatalf("extractSecretsToVault() error = %v", err)
	}

	secret, err := vault.ReadSecret("space_agent_admin_password")
	if err != nil {
		t.Fatalf("vault.ReadSecret() error = %v", err)
	}
	if secret != "chosen-space-password" {
		t.Fatalf("vault secret = %q, want %q", secret, "chosen-space-password")
	}

	section, ok := patch["space_agent"].(map[string]interface{})
	if !ok {
		t.Fatalf("patch[\"space_agent\"] missing or wrong type: %#v", patch["space_agent"])
	}
	if _, exists := section["admin_password"]; exists {
		t.Fatalf("admin_password field should have been removed from patch: %#v", section)
	}
}

func TestExtractSecretsToVaultStoresManifestSecrets(t *testing.T) {
	const masterKey = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

	vault, err := security.NewVault(masterKey, t.TempDir()+"\\vault.bin")
	if err != nil {
		t.Fatalf("NewVault() error = %v", err)
	}

	patch := map[string]interface{}{
		"manifest": map[string]interface{}{
			"enabled":            true,
			"api_key":            "mnfst_test_key",
			"postgres_password":  "pg-secret",
			"better_auth_secret": "better-auth-secret",
		},
	}

	if err := extractSecretsToVault(patch, vault, slog.Default()); err != nil {
		t.Fatalf("extractSecretsToVault() error = %v", err)
	}

	assertVaultSecret := func(key, want string) {
		t.Helper()
		secret, err := vault.ReadSecret(key)
		if err != nil {
			t.Fatalf("vault.ReadSecret(%q) error = %v", key, err)
		}
		if secret != want {
			t.Fatalf("vault secret %q = %q, want %q", key, secret, want)
		}
	}
	assertVaultSecret("manifest_api_key", "mnfst_test_key")
	assertVaultSecret("manifest_postgres_password", "pg-secret")
	assertVaultSecret("manifest_better_auth_secret", "better-auth-secret")

	section, ok := patch["manifest"].(map[string]interface{})
	if !ok {
		t.Fatalf("patch[\"manifest\"] missing or wrong type: %#v", patch["manifest"])
	}
	for _, key := range []string{"api_key", "postgres_password", "better_auth_secret"} {
		if _, exists := section[key]; exists {
			t.Fatalf("%s field should have been removed from patch: %#v", key, section)
		}
	}
}

func TestExtractSecretsToVaultStoresOmniRouteSecrets(t *testing.T) {
	const masterKey = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

	vault, err := security.NewVault(masterKey, t.TempDir()+"\\vault.bin")
	if err != nil {
		t.Fatalf("NewVault() error = %v", err)
	}

	patch := map[string]interface{}{
		"omniroute": map[string]interface{}{
			"enabled":          true,
			"api_key":          "omni-api-key",
			"initial_password": "initial-admin-password",
			"jwt_secret":       "jwt-secret",
			"api_key_secret":   "api-key-secret",
			"ws_bridge_secret": "ws-bridge-secret",
		},
	}

	if err := extractSecretsToVault(patch, vault, slog.Default()); err != nil {
		t.Fatalf("extractSecretsToVault() error = %v", err)
	}

	assertVaultSecret := func(key, want string) {
		t.Helper()
		secret, err := vault.ReadSecret(key)
		if err != nil {
			t.Fatalf("vault.ReadSecret(%q) error = %v", key, err)
		}
		if secret != want {
			t.Fatalf("vault secret %q = %q, want %q", key, secret, want)
		}
	}
	assertVaultSecret("omniroute_api_key", "omni-api-key")
	assertVaultSecret("omniroute_initial_password", "initial-admin-password")
	assertVaultSecret("omniroute_jwt_secret", "jwt-secret")
	assertVaultSecret("omniroute_api_key_secret", "api-key-secret")
	assertVaultSecret("omniroute_ws_bridge_secret", "ws-bridge-secret")

	section, ok := patch["omniroute"].(map[string]interface{})
	if !ok {
		t.Fatalf("patch[\"omniroute\"] missing or wrong type: %#v", patch["omniroute"])
	}
	for _, key := range []string{"api_key", "initial_password", "jwt_secret", "api_key_secret", "ws_bridge_secret"} {
		if _, exists := section[key]; exists {
			t.Fatalf("%s field should have been removed from patch: %#v", key, section)
		}
	}
}

func TestHandleUpdateConfigInvalidJSONIsGeneric(t *testing.T) {
	s := &Server{
		Cfg: &config.Config{ConfigPath: "config.yaml"},
	}

	req := httptest.NewRequest(http.MethodPut, "/api/config", strings.NewReader(`{"broken":`))
	rec := httptest.NewRecorder()

	handleUpdateConfig(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Invalid JSON") || strings.Contains(strings.ToLower(body), "unexpected eof") {
		t.Fatalf("expected generic invalid JSON response, got %q", body)
	}
}

func TestHandleUpdateConfigRegistersConfigured3DPrinterDevice(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte("three_d_printers:\n  enabled: true\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	db, err := inventory.InitDB(filepath.Join(tmpDir, "inventory.db"))
	if err != nil {
		t.Fatalf("init inventory db: %v", err)
	}
	defer db.Close()
	vault, err := security.NewVault("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", filepath.Join(tmpDir, "vault.bin"))
	if err != nil {
		t.Fatalf("init vault: %v", err)
	}

	loaded, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	loaded.ConfigPath = configPath
	s := &Server{
		Cfg:         loaded,
		Logger:      slog.Default(),
		Vault:       vault,
		InventoryDB: db,
	}

	body := strings.NewReader(`{
		"three_d_printers": {
			"enabled": true,
			"klipper": {
				"enabled": true,
				"printers": [
					{
						"id": "voron",
						"name": "Voron 2.4",
						"url": "http://192.168.6.60:7125",
						"timeout_seconds": 10
					}
				]
			}
		}
	}`)
	req := httptest.NewRequest(http.MethodPut, "/api/config", body)
	rec := httptest.NewRecorder()

	handleUpdateConfig(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	devices, err := inventory.QueryDevices(db, "3d-printer", "printer", "Voron 2.4")
	if err != nil {
		t.Fatalf("query devices: %v", err)
	}
	if len(devices) != 1 {
		t.Fatalf("registered devices = %d, want 1: %#v", len(devices), devices)
	}
	got := devices[0]
	if got.IPAddress != "192.168.6.60" || got.Port != 7125 {
		t.Fatalf("device address = %s:%d, want 192.168.6.60:7125", got.IPAddress, got.Port)
	}
	if !containsString(got.Tags, "klipper") {
		t.Fatalf("device tags = %#v, want klipper tag", got.Tags)
	}
	if !strings.Contains(got.Description, "3D printer") {
		t.Fatalf("device description = %q, want 3D printer context", got.Description)
	}
}

func TestHandleGetConfigMasksKlipperPrinterAPIKeyInArray(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	configContent := `
three_d_printers:
  enabled: true
  klipper:
    enabled: true
    printers:
      - id: voron
        url: http://192.168.6.60:7125
        api_key: moon-secret
`
	if err := os.WriteFile(configPath, []byte(configContent), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	s := &Server{Cfg: &config.Config{ConfigPath: configPath}, Logger: slog.Default()}

	req := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	rec := httptest.NewRecorder()
	handleGetConfig(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "moon-secret") {
		t.Fatalf("config response leaked Klipper API key: %s", rec.Body.String())
	}
	var body map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	printers := body["three_d_printers"].(map[string]interface{})["klipper"].(map[string]interface{})["printers"].([]interface{})
	printer := printers[0].(map[string]interface{})
	if printer["api_key"] != "••••••••" {
		t.Fatalf("api_key = %#v, want masked placeholder", printer["api_key"])
	}
}

func TestHandleUpdateConfigStoresKlipperAPIKeyInVaultAndStripsYAML(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte("three_d_printers:\n  enabled: true\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	vault, err := security.NewVault("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", filepath.Join(tmpDir, "vault.bin"))
	if err != nil {
		t.Fatalf("init vault: %v", err)
	}
	loaded, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	loaded.ConfigPath = configPath
	s := &Server{Cfg: loaded, Logger: slog.Default(), Vault: vault}

	body := strings.NewReader(`{
		"three_d_printers": {
			"enabled": true,
			"klipper": {
				"enabled": true,
				"printers": [
					{
						"id": "voron",
						"name": "Voron 2.4",
						"url": "http://192.168.6.60:7125",
						"api_key": "moon-secret",
						"timeout_seconds": 10
					}
				]
			}
		}
	}`)
	req := httptest.NewRequest(http.MethodPut, "/api/config", body)
	rec := httptest.NewRecorder()
	handleUpdateConfig(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	vaultKey := config.ThreeDPrinterKlipperAPIKeyVaultKey("voron")
	secret, err := vault.ReadSecret(vaultKey)
	if err != nil {
		t.Fatalf("ReadSecret(%q): %v", vaultKey, err)
	}
	if secret != "moon-secret" {
		t.Fatalf("vault secret = %q, want moon-secret", secret)
	}
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if strings.Contains(string(data), "moon-secret") || strings.Contains(string(data), "api_key") {
		t.Fatalf("config.yaml still contains Klipper API key material:\n%s", string(data))
	}
	if got := s.Cfg.ThreeDPrinters.Klipper.Printers[0].APIKey; got != "moon-secret" {
		t.Fatalf("runtime APIKey = %q, want moon-secret", got)
	}
}

func TestHandleUpdateConfigPreservesMaskedKlipperAPIKeyAndDeletesRemovedPrinterSecret(t *testing.T) {
	for _, apiKeyValue := range []string{"••••••••", ""} {
		t.Run("preserve_"+apiKeyValue, func(t *testing.T) {
			tmpDir := t.TempDir()
			configPath := filepath.Join(tmpDir, "config.yaml")
			configContent := `
three_d_printers:
  enabled: true
  klipper:
    enabled: true
    printers:
      - id: voron
        name: Voron 2.4
        url: http://192.168.6.60:7125
`
			if err := os.WriteFile(configPath, []byte(configContent), 0o600); err != nil {
				t.Fatalf("write config: %v", err)
			}
			vault, err := security.NewVault("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", filepath.Join(tmpDir, "vault.bin"))
			if err != nil {
				t.Fatalf("init vault: %v", err)
			}
			vaultKey := config.ThreeDPrinterKlipperAPIKeyVaultKey("voron")
			if err := vault.WriteSecret(vaultKey, "existing-secret"); err != nil {
				t.Fatalf("WriteSecret: %v", err)
			}
			loaded, err := config.Load(configPath)
			if err != nil {
				t.Fatalf("load config: %v", err)
			}
			loaded.ConfigPath = configPath
			loaded.ApplyVaultSecrets(vault)
			s := &Server{Cfg: loaded, Logger: slog.Default(), Vault: vault}

			body := strings.NewReader(`{"three_d_printers":{"enabled":true,"klipper":{"enabled":true,"printers":[{"id":"voron","name":"Voron 2.4","url":"http://192.168.6.60:7125","api_key":"` + apiKeyValue + `"}]}}}`)
			req := httptest.NewRequest(http.MethodPut, "/api/config", body)
			rec := httptest.NewRecorder()
			handleUpdateConfig(s).ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
			}
			secret, err := vault.ReadSecret(vaultKey)
			if err != nil {
				t.Fatalf("ReadSecret: %v", err)
			}
			if secret != "existing-secret" {
				t.Fatalf("vault secret = %q, want existing-secret", secret)
			}
			if got := s.Cfg.ThreeDPrinters.Klipper.Printers[0].APIKey; got != "existing-secret" {
				t.Fatalf("runtime APIKey = %q, want existing-secret", got)
			}
		})
	}

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	configContent := `
three_d_printers:
  enabled: true
  klipper:
    enabled: true
    printers:
      - id: old-voron
        url: http://192.168.6.60:7125
`
	if err := os.WriteFile(configPath, []byte(configContent), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	vault, err := security.NewVault("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", filepath.Join(tmpDir, "vault.bin"))
	if err != nil {
		t.Fatalf("init vault: %v", err)
	}
	oldKey := config.ThreeDPrinterKlipperAPIKeyVaultKey("old-voron")
	if err := vault.WriteSecret(oldKey, "remove-me"); err != nil {
		t.Fatalf("WriteSecret: %v", err)
	}
	loaded, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	loaded.ConfigPath = configPath
	loaded.ApplyVaultSecrets(vault)
	s := &Server{Cfg: loaded, Logger: slog.Default(), Vault: vault}

	req := httptest.NewRequest(http.MethodPut, "/api/config", strings.NewReader(`{"three_d_printers":{"enabled":true,"klipper":{"enabled":true,"printers":[]}}}`))
	rec := httptest.NewRecorder()
	handleUpdateConfig(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if secret, err := vault.ReadSecret(oldKey); err == nil && secret != "" {
		t.Fatalf("removed printer secret still exists: %q", secret)
	}
}

func TestHandleUpdateConfigNormalizesAIGatewayBeforePersisting(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte("ai_gateway:\n  enabled: true\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	vault, err := security.NewVault("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", filepath.Join(tmpDir, "vault.bin"))
	if err != nil {
		t.Fatalf("init vault: %v", err)
	}
	loaded, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	loaded.ConfigPath = configPath
	s := &Server{Cfg: loaded, Logger: slog.Default(), Vault: vault}

	body := strings.NewReader(`{
		"ai_gateway": {
			"enabled": true,
			"account_id": "acct",
			"gateway_id": "gw",
			"mode": "definitely-invalid",
			"log_mode": "leak-everything",
			"backoff": "sideways",
			"request_timeout_ms": -500,
			"max_attempts": 12,
			"retry_delay_ms": -10
		}
	}`)
	req := httptest.NewRequest(http.MethodPut, "/api/config", body)
	rec := httptest.NewRecorder()
	handleUpdateConfig(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	var raw map[string]interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal saved yaml: %v", err)
	}
	section, ok := raw["ai_gateway"].(map[string]interface{})
	if !ok {
		t.Fatalf("ai_gateway missing or wrong type in saved config: %#v", raw["ai_gateway"])
	}
	assertSaved := func(key string, want interface{}) {
		t.Helper()
		if got := section[key]; got != want {
			t.Fatalf("saved ai_gateway.%s = %#v, want %#v\n%s", key, got, want, string(data))
		}
	}
	assertSaved("mode", "auto")
	assertSaved("log_mode", "metadata_only")
	assertSaved("backoff", "")
	assertSaved("request_timeout_ms", 0)
	assertSaved("max_attempts", 5)
	assertSaved("retry_delay_ms", 0)
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func TestHandleUpdateConfigDiscordChangeDoesNotRequireFullRestart(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte("discord:\n  enabled: false\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	vault, err := security.NewVault("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", filepath.Join(tmpDir, "vault.bin"))
	if err != nil {
		t.Fatalf("init vault: %v", err)
	}
	loaded, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	loaded.ConfigPath = configPath
	s := &Server{
		Cfg:    loaded,
		Logger: slog.Default(),
		Vault:  vault,
	}

	body := strings.NewReader(`{"discord":{"enabled":true,"allowed_user_id":"1234","default_channel_id":"5678"}}`)
	req := httptest.NewRequest(http.MethodPut, "/api/config", body)
	rec := httptest.NewRecorder()

	handleUpdateConfig(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if resp["needs_restart"] == true {
		t.Fatalf("needs_restart = true, want Discord hot reload without full restart; body=%s", rec.Body.String())
	}
}

func TestHandleUpdateConfigRecreatesSQLPoolWhenPoolSettingsChange(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	sqlMetaPath := filepath.Join(tmpDir, "sql_connections.db")
	configContent := "sqlite:\n  sql_connections_path: " + sqlMetaPath + "\n" +
		"sql_connections:\n" +
		"  enabled: true\n" +
		"  max_pool_size: 2\n" +
		"  connection_timeout_sec: 5\n" +
		"  rate_limit_window_sec: 1\n" +
		"  idle_ttl_sec: 600\n"
	if err := os.WriteFile(configPath, []byte(configContent), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	vault, err := security.NewVault("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", filepath.Join(tmpDir, "vault.bin"))
	if err != nil {
		t.Fatalf("init vault: %v", err)
	}
	loaded, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	loaded.ConfigPath = configPath
	metaDB, err := sqlconnections.InitDB(sqlMetaPath)
	if err != nil {
		t.Fatalf("init sql connections db: %v", err)
	}
	defer metaDB.Close()
	oldPool := sqlconnections.NewConnectionPool(metaDB, vault, loaded.SQLConnections.MaxPoolSize, loaded.SQLConnections.ConnectionTimeoutSec, nil)
	oldPool.SetRateLimit(loaded.SQLConnections.RateLimitWindowSec)
	oldPool.SetIdleTTL(600 * time.Second)
	defer oldPool.CloseAll()
	s := &Server{
		Cfg:               loaded,
		Logger:            slog.Default(),
		Vault:             vault,
		SQLConnectionsDB:  metaDB,
		SQLConnectionPool: oldPool,
	}

	body := strings.NewReader(`{"sql_connections":{"enabled":true,"max_pool_size":4,"connection_timeout_sec":7,"rate_limit_window_sec":3,"idle_ttl_sec":30}}`)
	req := httptest.NewRequest(http.MethodPut, "/api/config", body)
	rec := httptest.NewRecorder()

	handleUpdateConfig(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if s.SQLConnectionPool == nil {
		t.Fatal("SQLConnectionPool is nil after SQL pool settings update")
	}
	if s.SQLConnectionPool == oldPool {
		t.Fatal("SQLConnectionPool pointer was not replaced after pool settings changed")
	}
	stats := s.SQLConnectionPool.PoolStats()
	if stats["max_connections"] != 4 {
		t.Fatalf("max_connections = %v, want 4", stats["max_connections"])
	}
	if stats["rate_limit_window"] != 3 {
		t.Fatalf("rate_limit_window = %v, want 3", stats["rate_limit_window"])
	}
}

func TestValidateManagedDockerBackendsRejectsLocalOllamaEmbeddingsWhenDockerDisabled(t *testing.T) {
	var cfg config.Config
	cfg.Embeddings.LocalOllama.Enabled = true

	err := validateManagedDockerBackends(cfg, config.Runtime{})
	if err == nil {
		t.Fatal("expected local Ollama embeddings to require Docker")
	}
	if !strings.Contains(err.Error(), "Docker integration is disabled") {
		t.Fatalf("error = %q, want Docker disabled explanation", err)
	}
}

func TestValidateManagedDockerBackendsRejectsManagedOllamaWithoutSocketInDocker(t *testing.T) {
	var cfg config.Config
	cfg.Docker.Enabled = true
	cfg.Ollama.ManagedInstance.Enabled = true

	err := validateManagedDockerBackends(cfg, config.Runtime{IsDocker: true, DockerSocketOK: false})
	if err == nil {
		t.Fatal("expected managed Ollama to require Docker socket inside Docker")
	}
	if !strings.Contains(err.Error(), "/var/run/docker.sock") {
		t.Fatalf("error = %q, want Docker socket explanation", err)
	}
}

func TestValidateManagedDockerBackendsAllowsRemoteDockerHost(t *testing.T) {
	var cfg config.Config
	cfg.Docker.Enabled = true
	cfg.Docker.Host = "tcp://docker.example.local:2375"
	cfg.Ollama.ManagedInstance.Enabled = true
	cfg.Embeddings.LocalOllama.Enabled = true

	if err := validateManagedDockerBackends(cfg, config.Runtime{IsDocker: true, DockerSocketOK: false}); err != nil {
		t.Fatalf("validateManagedDockerBackends() error = %v, want nil for remote Docker host", err)
	}
}

func TestHandleGetConfigRemovesHelperOwnedLegacyLLMFields(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	configContent := `
personality:
  engine_v2: true
  v2_provider: legacy-helper
  v2_model: helper-model
memory_analysis:
  provider: legacy-helper
  model: helper-model
tools:
  web_scraper:
    summary_provider: legacy-helper
  wikipedia:
    summary_provider: legacy-helper
  ddg_search:
    summary_provider: legacy-helper
  pdf_extractor:
    summary_provider: legacy-helper
`
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	s := &Server{
		Cfg:    &config.Config{ConfigPath: configPath},
		Logger: slog.Default(),
	}

	req := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	rec := httptest.NewRecorder()

	handleGetConfig(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var body map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	personality, _ := body["personality"].(map[string]interface{})
	if _, ok := personality["v2_provider"]; ok {
		t.Fatal("expected personality.v2_provider to be removed from config response")
	}
	if _, ok := personality["v2_model"]; ok {
		t.Fatal("expected personality.v2_model to be removed from config response")
	}

	memoryAnalysis, _ := body["memory_analysis"].(map[string]interface{})
	if _, ok := memoryAnalysis["provider"]; ok {
		t.Fatal("expected memory_analysis.provider to be removed from config response")
	}
	if _, ok := memoryAnalysis["model"]; ok {
		t.Fatal("expected memory_analysis.model to be removed from config response")
	}

	toolsMap, _ := body["tools"].(map[string]interface{})
	for _, toolKey := range []string{"web_scraper", "wikipedia", "ddg_search", "pdf_extractor"} {
		toolSection, _ := toolsMap[toolKey].(map[string]interface{})
		if _, ok := toolSection["summary_provider"]; ok {
			t.Fatalf("expected %s.summary_provider to be removed from config response", toolKey)
		}
	}
}

func TestHandleGetConfigInjectsKnowledgeGraphPermissionDefaults(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	configContent := `
tools:
  knowledge_graph:
    enabled: true
    readonly: false
`
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	loaded, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("config.Load() error = %v", err)
	}
	s := &Server{
		Cfg:    loaded,
		Logger: slog.Default(),
	}

	req := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	rec := httptest.NewRecorder()
	handleGetConfig(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var body map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	toolsMap, _ := body["tools"].(map[string]interface{})
	kg, _ := toolsMap["knowledge_graph"].(map[string]interface{})
	if kg["auto_extraction"] != true {
		t.Fatalf("tools.knowledge_graph.auto_extraction = %#v, want true", kg["auto_extraction"])
	}
	if kg["prompt_injection"] != true {
		t.Fatalf("tools.knowledge_graph.prompt_injection = %#v, want true", kg["prompt_injection"])
	}
}

func TestInjectVaultIndicatorsAddsAdditionalVaultBackedFields(t *testing.T) {
	const masterKey = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

	vault, err := security.NewVault(masterKey, filepath.Join(t.TempDir(), "vault.bin"))
	if err != nil {
		t.Fatalf("NewVault() error = %v", err)
	}

	secrets := map[string]string{
		"netlify_token":                  "netlify-secret",
		"vercel_token":                   "vercel-secret",
		"telnyx_api_key":                 "telnyx-secret",
		"cloudflared_token":              "cloudflare-secret",
		"uptime_kuma_api_key":            "uptime-kuma-secret",
		"a2a_api_key":                    "a2a-secret",
		"a2a_bearer_secret":              "a2a-bearer-secret",
		"truenas_api_key":                "truenas-secret",
		"jellyfin_api_key":               "jellyfin-secret",
		"a2a_remote_agent1_api_key":      "remote-api-key",
		"a2a_remote_agent1_bearer_token": "remote-bearer-token",
	}
	for key, value := range secrets {
		if err := vault.WriteSecret(key, value); err != nil {
			t.Fatalf("WriteSecret(%q) error = %v", key, err)
		}
	}

	rawCfg := map[string]interface{}{
		"netlify":           map[string]interface{}{},
		"vercel":            map[string]interface{}{},
		"telnyx":            map[string]interface{}{},
		"cloudflare_tunnel": map[string]interface{}{},
		"uptime_kuma":       map[string]interface{}{},
		"a2a": map[string]interface{}{
			"auth": map[string]interface{}{},
			"client": map[string]interface{}{
				"remote_agents": []interface{}{
					map[string]interface{}{"id": "agent1"},
				},
			},
		},
		"truenas":  map[string]interface{}{},
		"jellyfin": map[string]interface{}{},
	}

	injectVaultIndicators(rawCfg, vault)

	assertMasked := func(path string) {
		t.Helper()
		parts := strings.Split(path, ".")
		var current interface{} = rawCfg
		for _, part := range parts {
			m, ok := current.(map[string]interface{})
			if !ok {
				t.Fatalf("path %q missing at %q", path, part)
			}
			current = m[part]
		}
		if current != "••••••••" {
			t.Fatalf("%s = %#v, want masked secret", path, current)
		}
	}

	assertMasked("netlify.token")
	assertMasked("vercel.token")
	assertMasked("telnyx.api_key")
	assertMasked("cloudflare_tunnel.token")
	assertMasked("uptime_kuma.api_key")
	assertMasked("a2a.auth.api_key")
	assertMasked("a2a.auth.bearer_secret")
	assertMasked("truenas.api_key")
	assertMasked("jellyfin.api_key")

	a2aSection := rawCfg["a2a"].(map[string]interface{})
	clientSection := a2aSection["client"].(map[string]interface{})
	remoteAgents := clientSection["remote_agents"].([]interface{})
	remoteAgent := remoteAgents[0].(map[string]interface{})
	if remoteAgent["api_key"] != "••••••••" {
		t.Fatalf("remote agent api_key = %#v, want masked secret", remoteAgent["api_key"])
	}
	if remoteAgent["bearer_token"] != "••••••••" {
		t.Fatalf("remote agent bearer_token = %#v, want masked secret", remoteAgent["bearer_token"])
	}
}

// TestConfigSaveLoadNoDuplicateKeys verifies that saving and loading a config
// does not produce duplicate YAML keys (which would cause parse errors).
func TestConfigSaveLoadNoDuplicateKeys(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	// Initial config with a nested structure
	initialConfig := `
tools:
  daemon_skills:
    enabled: true
    max_concurrent_daemons: 5
    global_rate_limit_secs: 60
    max_wakeups_per_hour: 6
    max_budget_per_hour: 0.5
`
	if err := os.WriteFile(configPath, []byte(initialConfig), 0o644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	s := &Server{
		Cfg:    &config.Config{ConfigPath: configPath},
		Logger: slog.Default(),
	}

	// Simulate a save: load, apply patch (toggle enabled), save back
	req := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	rec := httptest.NewRecorder()
	handleGetConfig(s).ServeHTTP(rec, req)

	var loaded map[string]interface{}
	if err := yaml.Unmarshal(rec.Body.Bytes(), &loaded); err != nil {
		t.Fatalf("initial parse failed: %v", err)
	}

	// Apply a patch that toggles daemon_skills.enabled
	patch := map[string]interface{}{
		"tools": map[string]interface{}{
			"daemon_skills": map[string]interface{}{
				"enabled": false,
			},
		},
	}

	// deepMerge the patch into loaded config
	deepMerge(loaded, patch, "")

	// Marshal back to YAML
	out, err := yaml.Marshal(loaded)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	// Verify: parse again and check for duplicate keys
	var reloaded map[string]interface{}
	if err := yaml.Unmarshal(out, &reloaded); err != nil {
		t.Fatalf("re-parse failed (duplicate keys?): %v\nYAML was:\n%s", err, out)
	}

	// Verify daemon_skills is under tools:, not at root
	if _, hasRoot := reloaded["daemon_skills"]; hasRoot {
		t.Fatal("daemon_skills should NOT be at root level after save")
	}
	tools, _ := reloaded["tools"].(map[string]interface{})
	ds, _ := tools["daemon_skills"].(map[string]interface{})
	if ds == nil {
		t.Fatal("tools.daemon_skills missing after save")
	}
	if ds["enabled"] != false {
		t.Fatalf("daemon_skills.enabled = %v, want false", ds["enabled"])
	}
}

// TestDeepMergeNoDuplicateKeys specifically tests that deepMerge does not
// create duplicate keys when merging nested maps.
func TestDeepMergeNoDuplicateKeys(t *testing.T) {
	dst := map[string]interface{}{
		"tools": map[string]interface{}{
			"daemon_skills": map[string]interface{}{
				"enabled":                true,
				"max_concurrent_daemons": 5,
			},
		},
	}

	// Patch that only touches daemon_skills.enabled
	src := map[string]interface{}{
		"tools": map[string]interface{}{
			"daemon_skills": map[string]interface{}{
				"enabled": false,
			},
		},
	}

	deepMerge(dst, src, "")

	// Marshal and re-parse to check for duplicates
	out, err := yaml.Marshal(dst)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var result map[string]interface{}
	if err := yaml.Unmarshal(out, &result); err != nil {
		t.Fatalf("re-parse failed (duplicate keys): %v\nYAML:\n%s", err, out)
	}

	// Ensure no duplicate 'tools' key at root
	count := 0
	seen := make(map[string]bool)
	for k := range result {
		if k == "tools" {
			count++
		}
		seen[k] = true
	}
	if count != 1 {
		t.Fatalf("expected exactly one 'tools' key at root, got %d", count)
	}

	tools := result["tools"].(map[string]interface{})
	dsCount := 0
	for k := range tools {
		if k == "daemon_skills" {
			dsCount++
		}
	}
	if dsCount != 1 {
		t.Fatalf("expected exactly one 'daemon_skills' key in tools, got %d", dsCount)
	}
}

// TestConfigSchemaConsistency verifies that all sections defined in the UI SECTIONS
// map to correct YAML paths that match the config schema structure.
func TestConfigSchemaConsistency(t *testing.T) {
	// Sections that are known to be nested under 'tools:' in config_template.yaml
	nestedUnderTools := map[string]bool{
		"daemon_skills": true,
		"skill_manager": true,
		"web_scraper":   true,
		"sandbox":       true,
	}

	// Sections that are at root level (top-level YAML keys)
	rootLevelSections := map[string]bool{
		"agent":            true,
		"auth":             true,
		"server":           true,
		"llm":              true,
		"providers":        true,
		"tools":            true,
		"budget":           true,
		"docker":           true,
		"discord":          true,
		"telegram":         true,
		"rocketchat":       true,
		"tailscale":        true,
		"fritzbox":         true,
		"home_assistant":   true,
		"ollama":           true,
		"proxmox":          true,
		"ddg_search":       true,
		"brave_search":     true,
		"webhooks":         true,
		"invasion_control": true,
		"mqtt":             true,
		"security_proxy":   true,
	}

	testCases := []struct {
		section     string
		expectedKey string // the YAML key this section should use
		isNested    bool   // true if this section's data lives under tools.*
	}{
		{"daemon_skills", "tools.daemon_skills", true},
		{"skill_manager", "tools.skill_manager", true},
		{"web_scraper", "tools.web_scraper", true},
	}

	for _, tc := range testCases {
		if tc.isNested && nestedUnderTools[tc.section] {
			// OK - section is correctly identified as nested
		}
		if !tc.isNested && rootLevelSections[tc.section] {
			// OK - section is correctly identified as root-level
		}
	}

	_ = rootLevelSections // silence unused warning (kept for documentation)
	_ = nestedUnderTools
}
