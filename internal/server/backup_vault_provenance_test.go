package server

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/security"
)

func TestVaultBackupRoundTripPreservesSecretProvenance(t *testing.T) {
	source, err := security.NewVault(strings.Repeat("c", 64), filepath.Join(t.TempDir(), "source.bin"))
	if err != nil {
		t.Fatal(err)
	}
	if err := source.WriteAgentSecret("AGENT_VALUE", "agent-secret"); err != nil {
		t.Fatal(err)
	}
	if err := source.WriteUserSecret("MODAL_VALUE", "modal-secret", true); err != nil {
		t.Fatal(err)
	}
	if err := source.WriteSecret("LEGACY_VALUE", "legacy-secret"); err != nil {
		t.Fatal(err)
	}
	blob, err := exportVaultSecrets(source, "backup-password")
	if err != nil {
		t.Fatal(err)
	}

	target, err := security.NewVault(strings.Repeat("d", 64), filepath.Join(t.TempDir(), "target.bin"))
	if err != nil {
		t.Fatal(err)
	}
	count, err := importVaultSecrets(target, blob, "backup-password")
	if err != nil || count != 3 {
		t.Fatalf("importVaultSecrets() count = %d, err = %v", count, err)
	}
	if value, err := target.ReadSecretForAgent("AGENT_VALUE"); err != nil || value != "agent-secret" {
		t.Fatalf("agent value = %q, err = %v", value, err)
	}
	for _, key := range []string{"MODAL_VALUE", "LEGACY_VALUE"} {
		if _, err := target.ReadSecretForAgent(key); !errors.Is(err, security.ErrSecretAgentAccessDenied) {
			t.Fatalf("%s became agent-readable: %v", key, err)
		}
	}
}

func TestLegacyVaultBackupImportsFailClosed(t *testing.T) {
	plain, err := json.Marshal(map[string]string{"LEGACY": "legacy-secret"})
	if err != nil {
		t.Fatal(err)
	}
	blob, err := encryptBackupPasswordBlob(vaultSecretsMagic, plain, "backup-password")
	if err != nil {
		t.Fatal(err)
	}
	target, err := security.NewVault(strings.Repeat("e", 64), filepath.Join(t.TempDir(), "target.bin"))
	if err != nil {
		t.Fatal(err)
	}
	if count, err := importVaultSecrets(target, blob, "backup-password"); err != nil || count != 1 {
		t.Fatalf("importVaultSecrets() count = %d, err = %v", count, err)
	}
	if _, err := target.ReadSecretForAgent("LEGACY"); !errors.Is(err, security.ErrSecretAgentAccessDenied) {
		t.Fatalf("legacy backup became agent-readable: %v", err)
	}
}
