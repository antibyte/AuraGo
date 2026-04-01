package server

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
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
