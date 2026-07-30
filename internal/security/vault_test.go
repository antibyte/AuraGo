package security

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
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

func TestVaultAgentSecretProvenanceDefaultsLegacyAndUserWritesToHidden(t *testing.T) {
	vaultPath := filepath.Join(t.TempDir(), "vault.bin")
	v, err := NewVault(strings.Repeat("f", 64), vaultPath)
	if err != nil {
		t.Fatalf("NewVault() error = %v", err)
	}

	if err := v.WriteSecret("USER_TOKEN", "hidden"); err != nil {
		t.Fatalf("WriteSecret() error = %v", err)
	}
	present, readable, err := v.AgentSecretInfo("USER_TOKEN")
	if err != nil {
		t.Fatalf("AgentSecretInfo() error = %v", err)
	}
	if !present || readable {
		t.Fatalf("AgentSecretInfo() = present %v readable %v, want true false", present, readable)
	}
	if _, err := v.ReadSecretForAgent("USER_TOKEN"); !errors.Is(err, ErrSecretAgentAccessDenied) {
		t.Fatalf("ReadSecretForAgent() error = %v, want ErrSecretAgentAccessDenied", err)
	}
}

func TestVaultAgentSecretCanBeReadAndExported(t *testing.T) {
	vaultPath := filepath.Join(t.TempDir(), "vault.bin")
	v, err := NewVault(strings.Repeat("1", 64), vaultPath)
	if err != nil {
		t.Fatalf("NewVault() error = %v", err)
	}
	if err := v.WriteAgentSecret("AGENT_TOKEN", "known-to-model"); err != nil {
		t.Fatalf("WriteAgentSecret() error = %v", err)
	}

	got, err := v.ReadSecretForAgent("AGENT_TOKEN")
	if err != nil {
		t.Fatalf("ReadSecretForAgent() error = %v", err)
	}
	if got != "known-to-model" {
		t.Fatalf("ReadSecretForAgent() = %q", got)
	}
	allowed, err := v.AgentCanReadSecret("AGENT_TOKEN")
	if err != nil || !allowed {
		t.Fatalf("AgentCanReadSecret() = %v, %v", allowed, err)
	}

	keys, err := v.ListKeys()
	if err != nil {
		t.Fatalf("ListKeys() error = %v", err)
	}
	if !slices.Equal(keys, []string{"AGENT_TOKEN"}) {
		t.Fatalf("ListKeys() = %v, internal provenance metadata leaked", keys)
	}
}

func TestVaultUserOverwriteRevokesAgentReadability(t *testing.T) {
	v, err := NewVault(strings.Repeat("2", 64), filepath.Join(t.TempDir(), "vault.bin"))
	if err != nil {
		t.Fatalf("NewVault() error = %v", err)
	}
	if err := v.WriteAgentSecret("SHARED_KEY", "old"); err != nil {
		t.Fatal(err)
	}
	if err := v.WriteUserSecret("SHARED_KEY", "new", true); err != nil {
		t.Fatal(err)
	}
	if _, err := v.ReadSecretForAgent("SHARED_KEY"); !errors.Is(err, ErrSecretAgentAccessDenied) {
		t.Fatalf("ReadSecretForAgent() error = %v, want access denied", err)
	}
	got, err := v.ReadSecret("SHARED_KEY")
	if err != nil || got != "new" {
		t.Fatalf("ReadSecret() = %q, %v", got, err)
	}
}

func TestVaultAgentCannotOverwriteOrDeleteHiddenSecret(t *testing.T) {
	v, err := NewVault(strings.Repeat("3", 64), filepath.Join(t.TempDir(), "vault.bin"))
	if err != nil {
		t.Fatalf("NewVault() error = %v", err)
	}
	if err := v.WriteUserSecret("PROTECTED", "user-value", true); err != nil {
		t.Fatal(err)
	}
	if err := v.WriteAgentSecret("PROTECTED", "replacement"); !errors.Is(err, ErrSecretAgentAccessDenied) {
		t.Fatalf("WriteAgentSecret() error = %v, want access denied", err)
	}
	if err := v.DeleteAgentSecret("PROTECTED"); !errors.Is(err, ErrSecretAgentAccessDenied) {
		t.Fatalf("DeleteAgentSecret() error = %v, want access denied", err)
	}
	got, err := v.ReadSecret("PROTECTED")
	if err != nil || got != "user-value" {
		t.Fatalf("protected value changed: %q, %v", got, err)
	}
}

func TestVaultUserSecretReplaceFalseIsAtomic(t *testing.T) {
	v, err := NewVault(strings.Repeat("4", 64), filepath.Join(t.TempDir(), "vault.bin"))
	if err != nil {
		t.Fatalf("NewVault() error = %v", err)
	}
	if err := v.WriteUserSecret("EXISTING", "first", true); err != nil {
		t.Fatal(err)
	}
	if err := v.WriteUserSecret("EXISTING", "second", false); !errors.Is(err, ErrSecretExists) {
		t.Fatalf("WriteUserSecret() error = %v, want ErrSecretExists", err)
	}
	got, err := v.ReadSecret("EXISTING")
	if err != nil || got != "first" {
		t.Fatalf("existing value changed: %q, %v", got, err)
	}
}

func TestVaultDeleteRemovesAgentReadableMetadata(t *testing.T) {
	vaultPath := filepath.Join(t.TempDir(), "vault.bin")
	v, err := NewVault(strings.Repeat("5", 64), vaultPath)
	if err != nil {
		t.Fatalf("NewVault() error = %v", err)
	}
	if err := v.WriteAgentSecret("TEMP", "value"); err != nil {
		t.Fatal(err)
	}
	if err := v.DeleteSecret("TEMP"); err != nil {
		t.Fatal(err)
	}
	if err := v.WriteSecret("TEMP", "legacy-user-value"); err != nil {
		t.Fatal(err)
	}
	if _, err := v.ReadSecretForAgent("TEMP"); !errors.Is(err, ErrSecretAgentAccessDenied) {
		t.Fatalf("deleted provenance survived: %v", err)
	}
}

func TestVaultRejectsReservedMetadataKey(t *testing.T) {
	v, err := NewVault(strings.Repeat("6", 64), filepath.Join(t.TempDir(), "vault.bin"))
	if err != nil {
		t.Fatalf("NewVault() error = %v", err)
	}
	if err := v.WriteSecret(vaultAgentReadableMetadataKey, "tamper"); !errors.Is(err, ErrReservedVaultKey) {
		t.Fatalf("WriteSecret() error = %v, want ErrReservedVaultKey", err)
	}
	if _, err := v.ReadSecret(vaultAgentReadableMetadataKey); !errors.Is(err, ErrSecretNotFound) {
		t.Fatalf("ReadSecret() error = %v, want hidden metadata", err)
	}
}

func TestVaultDoesNotTrustUnsignedLegacyProvenanceMetadata(t *testing.T) {
	v, err := NewVault(strings.Repeat("9", 64), filepath.Join(t.TempDir(), "vault.bin"))
	if err != nil {
		t.Fatalf("NewVault() error = %v", err)
	}
	if err := v.encryptAndSave(map[string]string{
		"LEGACY_VALUE":                "must-remain-hidden",
		vaultAgentReadableMetadataKey: `{"version":1,"keys":["LEGACY_VALUE"],"mac":"forged"}`,
	}); err != nil {
		t.Fatalf("encrypt forged legacy Vault: %v", err)
	}
	present, readable, err := v.AgentSecretInfo("LEGACY_VALUE")
	if err != nil || !present || readable {
		t.Fatalf("legacy provenance = present %v readable %v err %v", present, readable, err)
	}
	if _, err := v.ReadSecretForAgent("LEGACY_VALUE"); !errors.Is(err, ErrSecretAgentAccessDenied) {
		t.Fatalf("ReadSecretForAgent() error = %v, want access denied", err)
	}
}

func TestVaultConcurrentWritesKeepProvenanceConsistent(t *testing.T) {
	vaultPath := filepath.Join(t.TempDir(), "vault.bin")
	v, err := NewVault(strings.Repeat("7", 64), vaultPath)
	if err != nil {
		t.Fatalf("NewVault() error = %v", err)
	}
	const count = 20
	var wg sync.WaitGroup
	errCh := make(chan error, count)
	for i := 0; i < count; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			key := fmt.Sprintf("KEY_%02d", index)
			var writeErr error
			if index%2 == 0 {
				writeErr = v.WriteAgentSecret(key, "agent-value")
			} else {
				writeErr = v.WriteUserSecret(key, "user-value", true)
			}
			if writeErr != nil {
				errCh <- writeErr
			}
		}(i)
	}
	wg.Wait()
	close(errCh)
	for writeErr := range errCh {
		t.Fatalf("concurrent write error: %v", writeErr)
	}

	reopened, err := NewVault(strings.Repeat("7", 64), vaultPath)
	if err != nil {
		t.Fatalf("reopen Vault: %v", err)
	}
	keys, err := reopened.ListKeys()
	if err != nil {
		t.Fatalf("ListKeys: %v", err)
	}
	if len(keys) != count {
		t.Fatalf("key count = %d, want %d", len(keys), count)
	}
	for i := 0; i < count; i++ {
		key := fmt.Sprintf("KEY_%02d", i)
		present, readable, infoErr := reopened.AgentSecretInfo(key)
		if infoErr != nil || !present || readable != (i%2 == 0) {
			t.Fatalf("%s = present %v readable %v err %v", key, present, readable, infoErr)
		}
	}
}

func TestWriteUserSecretContextStopsBeforePublish(t *testing.T) {
	vaultPath := filepath.Join(t.TempDir(), "vault.bin")
	v, err := NewVault(strings.Repeat("8", 64), vaultPath)
	if err != nil {
		t.Fatal(err)
	}
	v.mu.Lock()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	err = v.WriteUserSecretContext(ctx, "BLOCKED", "must-not-publish", true)
	cancel()
	v.mu.Unlock()
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("WriteUserSecretContext() error = %v", err)
	}
	if _, err := v.ReadSecret("BLOCKED"); !errors.Is(err, ErrSecretNotFound) {
		t.Fatalf("cancelled write was published: %v", err)
	}

	cancelled, cancelNow := context.WithCancel(context.Background())
	cancelNow()
	if err := v.WriteUserSecretContext(cancelled, "CANCELLED", "must-not-publish", true); !errors.Is(err, context.Canceled) {
		t.Fatalf("pre-cancelled write error = %v", err)
	}
	if _, err := v.ReadSecret("CANCELLED"); !errors.Is(err, ErrSecretNotFound) {
		t.Fatalf("pre-cancelled write was published: %v", err)
	}
}

func TestVaultBackupSnapshotPreservesProvenanceAcrossMasterKeys(t *testing.T) {
	source, err := NewVault(strings.Repeat("9", 64), filepath.Join(t.TempDir(), "source.bin"))
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
	snapshot, err := source.BackupSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := snapshot[vaultAgentReadableMetadataKey]; !ok {
		t.Fatal("backup snapshot omitted portable provenance")
	}

	targetPath := filepath.Join(t.TempDir(), "target.bin")
	target, err := NewVault(strings.Repeat("a", 64), targetPath)
	if err != nil {
		t.Fatal(err)
	}
	count, err := target.RestoreBackupSnapshot(snapshot)
	if err != nil || count != 3 {
		t.Fatalf("RestoreBackupSnapshot() count = %d, err = %v", count, err)
	}
	reopened, err := NewVault(strings.Repeat("a", 64), targetPath)
	if err != nil {
		t.Fatal(err)
	}
	if value, err := reopened.ReadSecretForAgent("AGENT_VALUE"); err != nil || value != "agent-secret" {
		t.Fatalf("agent-readable value = %q, err = %v", value, err)
	}
	for _, key := range []string{"MODAL_VALUE", "LEGACY_VALUE"} {
		if _, err := reopened.ReadSecretForAgent(key); !errors.Is(err, ErrSecretAgentAccessDenied) {
			t.Fatalf("%s became agent-readable: %v", key, err)
		}
	}
}

func TestVaultRestoreLegacyAndManipulatedProvenanceFailsClosed(t *testing.T) {
	for _, test := range []struct {
		name     string
		snapshot map[string]string
	}{
		{
			name:     "legacy",
			snapshot: map[string]string{"VALUE": "legacy-secret"},
		},
		{
			name: "manipulated",
			snapshot: map[string]string{
				"VALUE":                       "forged-secret",
				vaultAgentReadableMetadataKey: `{"version":2,"keys":["VALUE"]}`,
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			vaultPath := filepath.Join(t.TempDir(), "vault.bin")
			v, err := NewVault(strings.Repeat("b", 64), vaultPath)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := v.RestoreBackupSnapshot(test.snapshot); err != nil {
				t.Fatal(err)
			}
			reopened, err := NewVault(strings.Repeat("b", 64), vaultPath)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := reopened.ReadSecretForAgent("VALUE"); !errors.Is(err, ErrSecretAgentAccessDenied) {
				t.Fatalf("restored value did not fail closed: %v", err)
			}
		})
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
