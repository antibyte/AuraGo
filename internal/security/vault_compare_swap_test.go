package security

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

func TestVaultCompareSwapPreservesConcurrentLoginAndRevocation(t *testing.T) {
	v, err := NewVault(strings.Repeat("01", 32), filepath.Join(t.TempDir(), "vault.bin"))
	if err != nil {
		t.Fatal(err)
	}
	if err := v.WriteAgentSecret("fixture", "old"); err != nil {
		t.Fatal(err)
	}
	if err := v.CompareAndSwapSecret("fixture", "old", "rotated"); err != nil {
		t.Fatal(err)
	}
	if _, err := v.ReadSecretForAgent("fixture"); !errors.Is(err, ErrSecretAgentAccessDenied) {
		t.Fatal("rotation granted agent access")
	}
	if err := v.CompareAndSwapSecret("fixture", "old", "stale"); !errors.Is(err, ErrSecretChanged) {
		t.Fatal("stale write accepted")
	}
	if got, err := v.ReadSecret("fixture"); err != nil || got != "rotated" {
		t.Fatal("latest value lost")
	}
	if err := v.DeleteSecret("fixture"); err != nil {
		t.Fatal(err)
	}
	if err := v.CompareAndSwapSecret("fixture", "rotated", "revived"); !errors.Is(err, ErrSecretChanged) {
		t.Fatal("revoked secret revived")
	}
}
