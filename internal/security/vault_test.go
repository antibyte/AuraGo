package security

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestVaultWriteSecretPersistsAtomically(t *testing.T) {
	vaultPath := filepath.Join(t.TempDir(), "vault.bin")
	v, err := NewVault(strings.Repeat("a", 64), vaultPath)
	if err != nil {
		t.Fatalf("NewVault() error = %v", err)
	}

	if err := v.WriteSecret("demo", "value"); err != nil {
		t.Fatalf("WriteSecret() error = %v", err)
	}

	got, err := v.ReadSecret("demo")
	if err != nil {
		t.Fatalf("ReadSecret() error = %v", err)
	}
	if got != "value" {
		t.Fatalf("secret = %q, want value", got)
	}
}

// TestVaultReadSecretNotFound verifies that reading a non-existent key returns a clear error.
func TestVaultReadSecretNotFound(t *testing.T) {
	vaultPath := filepath.Join(t.TempDir(), "vault_empty.bin")
	v, err := NewVault(strings.Repeat("a", 64), vaultPath)
	if err != nil {
		t.Fatalf("NewVault() error = %v", err)
	}
	_, err = v.ReadSecret("nonexistent")
	if err == nil {
		t.Fatal("expected error for non-existent secret, got nil")
	}
}

// TestVaultDeleteSecret verifies that DeleteSecret removes a key and subsequent reads fail.
func TestVaultDeleteSecret(t *testing.T) {
	vaultPath := filepath.Join(t.TempDir(), "vault_del.bin")
	v, err := NewVault(strings.Repeat("b", 64), vaultPath)
	if err != nil {
		t.Fatalf("NewVault() error = %v", err)
	}
	if err := v.WriteSecret("to_delete", "secret_value"); err != nil {
		t.Fatalf("WriteSecret() error = %v", err)
	}
	if err := v.DeleteSecret("to_delete"); err != nil {
		t.Fatalf("DeleteSecret() error = %v", err)
	}
	_, err = v.ReadSecret("to_delete")
	if err == nil {
		t.Fatal("expected error after deleting secret, got nil")
	}
}

// TestVaultDeleteSecretNonexistent verifies that deleting a non-existent key does not error.
func TestVaultDeleteSecretNonexistent(t *testing.T) {
	vaultPath := filepath.Join(t.TempDir(), "vault_del_none.bin")
	v, err := NewVault(strings.Repeat("c", 64), vaultPath)
	if err != nil {
		t.Fatalf("NewVault() error = %v", err)
	}
	// Should not error even if the key doesn't exist.
	if err := v.DeleteSecret("does_not_exist"); err != nil {
		t.Fatalf("DeleteSecret() on non-existent key should not error, got: %v", err)
	}
}

// TestVaultWriteReadMultipleSecrets verifies that multiple keys coexist independently.
func TestVaultWriteReadMultipleSecrets(t *testing.T) {
	vaultPath := filepath.Join(t.TempDir(), "vault_multi.bin")
	v, err := NewVault(strings.Repeat("d", 64), vaultPath)
	if err != nil {
		t.Fatalf("NewVault() error = %v", err)
	}
	kv := map[string]string{
		"key1": "value1",
		"key2": "value2",
		"key3": "value3",
	}
	for k, val := range kv {
		if err := v.WriteSecret(k, val); err != nil {
			t.Fatalf("WriteSecret(%q) error = %v", k, err)
		}
	}
	for k, expected := range kv {
		got, err := v.ReadSecret(k)
		if err != nil {
			t.Fatalf("ReadSecret(%q) error = %v", k, err)
		}
		if got != expected {
			t.Fatalf("secret %q = %q, want %q", k, got, expected)
		}
	}
}

// TestVaultUpdateSecret verifies that overwriting a key returns the new value.
func TestVaultUpdateSecret(t *testing.T) {
	vaultPath := filepath.Join(t.TempDir(), "vault_update.bin")
	v, err := NewVault(strings.Repeat("e", 64), vaultPath)
	if err != nil {
		t.Fatalf("NewVault() error = %v", err)
	}
	if err := v.WriteSecret("updatable", "old_value"); err != nil {
		t.Fatalf("WriteSecret() error = %v", err)
	}
	if err := v.WriteSecret("updatable", "new_value"); err != nil {
		t.Fatalf("WriteSecret() update error = %v", err)
	}
	got, err := v.ReadSecret("updatable")
	if err != nil {
		t.Fatalf("ReadSecret() error = %v", err)
	}
	if got != "new_value" {
		t.Fatalf("secret = %q, want new_value", got)
	}
}

// TestVaultInvalidMasterKey verifies that an invalid master key format is rejected.
func TestVaultInvalidMasterKey(t *testing.T) {
	// Key too short.
	_, err := NewVault("too_short", "unused_path")
	if err == nil {
		t.Fatal("expected error for short master key, got nil")
	}
	// Invalid hex characters.
	_, err = NewVault("ZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZ", "unused_path")
	if err == nil {
		t.Fatal("expected error for invalid hex in master key, got nil")
	}
}
