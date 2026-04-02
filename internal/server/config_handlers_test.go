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
