package server

import (
	"log/slog"
	"testing"

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
