package webhooks

import (
	"errors"
	"fmt"
	"strings"
)

// Signature secret deletion failures keep the webhook in place. Callers map
// these sentinels to their own status text.
var (
	ErrSignatureVaultUnavailable = errors.New("vault unavailable for webhook signature secret")
	ErrSignatureSecretDelete     = errors.New("failed to delete webhook signature secret")
	ErrWebhookDeleteRestore      = errors.New("failed to delete webhook and restore its signature secret")
)

// SignatureSecretStore is the vault surface used when a webhook is deleted.
type SignatureSecretStore interface {
	ReadSecret(key string) (string, error)
	WriteSecret(key, value string) error
	DeleteSecret(key string) error
}

// SignatureSecretVaultKey returns the vault key used for a webhook's signature secret.
func SignatureSecretVaultKey(webhookID string) string {
	webhookID = strings.TrimSpace(webhookID)
	if webhookID == "" {
		return ""
	}
	return "webhook_" + webhookID + "_signature_secret"
}

func missingSignatureSecret(err error) bool {
	return err != nil && strings.EqualFold(strings.TrimSpace(err.Error()), "secret not found")
}

// DeleteWithSignatureSecret removes the vault secret before the webhook file.
// A failed file delete restores a secret that existed. A missing secret is not
// an error. A nil store deletes the webhook record only.
func (m *Manager) DeleteWithSignatureSecret(store SignatureSecretStore, id string) error {
	if _, err := m.Get(id); err != nil {
		return err
	}
	if store == nil {
		return m.Delete(id)
	}
	key := SignatureSecretVaultKey(id)
	previous, readErr := store.ReadSecret(key)
	if readErr != nil && !missingSignatureSecret(readErr) {
		return fmt.Errorf("%w: %v", ErrSignatureVaultUnavailable, readErr)
	}
	existed := readErr == nil
	if err := store.DeleteSecret(key); err != nil {
		return fmt.Errorf("%w: %v", ErrSignatureSecretDelete, err)
	}
	if err := m.Delete(id); err != nil {
		if existed {
			if restoreErr := store.WriteSecret(key, previous); restoreErr != nil {
				return fmt.Errorf("%w: %v", ErrWebhookDeleteRestore, restoreErr)
			}
		}
		return err
	}
	return nil
}
