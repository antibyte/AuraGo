package server

import (
	"path/filepath"
	"testing"

	"aurago/internal/config"
	"aurago/internal/security"
	"aurago/internal/tools"
)

func TestLoadPlainSkillSecretsRejectsSystemManagedKeys(t *testing.T) {
	vaultPath := filepath.Join(t.TempDir(), "vault.bin")
	vault, err := security.NewVault("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", vaultPath)
	if err != nil {
		t.Fatalf("NewVault: %v", err)
	}
	if err := vault.WriteSecret("github_token", "system-secret"); err != nil {
		t.Fatalf("WriteSecret(system): %v", err)
	}
	if err := vault.WriteSecret("user_defined_api_key", "user-secret"); err != nil {
		t.Fatalf("WriteSecret(user): %v", err)
	}

	s := &Server{
		Cfg:   &config.Config{},
		Vault: vault,
	}
	s.Cfg.Tools.PythonSecretInjection.Enabled = true
	skill := &tools.SkillRegistryEntry{
		VaultKeys: []string{"github_token", "user_defined_api_key"},
	}

	got := loadPlainSkillSecrets(s, skill)
	if _, ok := got["github_token"]; ok {
		t.Fatalf("loadPlainSkillSecrets exposed system-managed github_token")
	}
	if got["user_defined_api_key"] != "user-secret" {
		t.Fatalf("loadPlainSkillSecrets user secret = %q, want user-secret", got["user_defined_api_key"])
	}
}

func TestLoadPlainSkillSecretsRequiresPythonSecretInjectionEnabled(t *testing.T) {
	vaultPath := filepath.Join(t.TempDir(), "vault.bin")
	vault, err := security.NewVault("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", vaultPath)
	if err != nil {
		t.Fatalf("NewVault: %v", err)
	}
	if err := vault.WriteSecret("user_defined_api_key", "user-secret"); err != nil {
		t.Fatalf("WriteSecret(user): %v", err)
	}

	s := &Server{
		Cfg:   &config.Config{},
		Vault: vault,
	}
	skill := &tools.SkillRegistryEntry{
		VaultKeys: []string{"user_defined_api_key"},
	}

	if got := loadPlainSkillSecrets(s, skill); len(got) != 0 {
		t.Fatalf("loadPlainSkillSecrets() = %#v, want no secrets when injection disabled", got)
	}
}
