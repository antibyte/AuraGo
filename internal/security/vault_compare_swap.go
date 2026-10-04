package security

import (
	"errors"
	"fmt"
)

var ErrSecretChanged = errors.New("secret changed during operation")

// CompareAndSwapSecret publishes a rotated system secret only while the exact
// previous value still exists. Revocation or a concurrent login always wins.
// Written values remain hidden from the agent.
func (v *Vault) CompareAndSwapSecret(key, expected, value string) error {
	if isReservedVaultMetadataKey(key) {
		return ErrReservedVaultKey
	}
	v.mu.Lock()
	defer v.mu.Unlock()
	if err := v.fileLock.Lock(); err != nil {
		return fmt.Errorf("acquire vault lock: %w", err)
	}
	defer v.fileLock.Unlock()
	secrets, err := v.loadAndDecrypt()
	if err != nil {
		return err
	}
	current, exists := secrets[key]
	if !exists || current != expected {
		return ErrSecretChanged
	}
	readable, err := v.loadAgentReadableKeys(secrets)
	if err != nil {
		return err
	}
	secrets[key] = value
	delete(readable, key)
	if err := v.storeAgentReadableKeys(secrets, readable); err != nil {
		return err
	}
	return v.encryptAndSave(secrets)
}
