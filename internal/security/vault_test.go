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
