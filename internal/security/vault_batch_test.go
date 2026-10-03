package security

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

func TestVaultWriteSecretsAppliesSetAndRemoveTogether(t *testing.T) {
	v, err := NewVault(strings.Repeat("7", 64), filepath.Join(t.TempDir(), "vault.bin"))
	if err != nil {
		t.Fatalf("NewVault() error = %v", err)
	}
	if err := v.WriteSecret("auth_session_secret", "old-session"); err != nil {
		t.Fatal(err)
	}
	if err := v.WriteSecrets(map[string]string{"auth_password_hash": "new-hash", "auth_totp_secret": ""}, []string{"auth_session_secret"}); err != nil {
		t.Fatalf("WriteSecrets() error = %v", err)
	}
	if got, err := v.ReadSecret("auth_password_hash"); err != nil || got != "new-hash" {
		t.Fatalf("auth_password_hash = %q, %v", got, err)
	}
	if got, err := v.ReadSecret("auth_totp_secret"); err != nil || got != "" {
		t.Fatalf("auth_totp_secret = %q, %v", got, err)
	}
	if _, err := v.ReadSecret("auth_session_secret"); !errors.Is(err, ErrSecretNotFound) {
		t.Fatalf("removed key still present: %v", err)
	}
}

func TestVaultWriteSecretsRejectsReservedKeysWithoutPartialWrite(t *testing.T) {
	v, err := NewVault(strings.Repeat("8", 64), filepath.Join(t.TempDir(), "vault.bin"))
	if err != nil {
		t.Fatalf("NewVault() error = %v", err)
	}
	err = v.WriteSecrets(map[string]string{"ordinary": "value", vaultAgentReadableMetadataKey: "tamper"}, nil)
	if !errors.Is(err, ErrReservedVaultKey) {
		t.Fatalf("WriteSecrets() error = %v, want ErrReservedVaultKey", err)
	}
	if _, err := v.ReadSecret("ordinary"); !errors.Is(err, ErrSecretNotFound) {
		t.Fatalf("batch was partially written: %v", err)
	}
}
