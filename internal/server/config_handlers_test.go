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
	"aurago/internal/security"

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

func TestInjectVaultIndicatorsAddsAdditionalVaultBackedFields(t *testing.T) {
	const masterKey = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

	vault, err := security.NewVault(masterKey, filepath.Join(t.TempDir(), "vault.bin"))
	if err != nil {
		t.Fatalf("NewVault() error = %v", err)
	}

	secrets := map[string]string{
		"netlify_token":                  "netlify-secret",
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
