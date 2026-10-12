package webhooks

import (
	"errors"
	"path/filepath"
	"testing"
)

type memSecretStore struct {
	secrets   map[string]string
	readErr   error
	deleteErr error
	writeErr  error
}

func (s *memSecretStore) ReadSecret(key string) (string, error) {
	if s.readErr != nil {
		return "", s.readErr
	}
	value, ok := s.secrets[key]
	if !ok {
		return "", errors.New("secret not found")
	}
	return value, nil
}

func (s *memSecretStore) WriteSecret(key, value string) error {
	if s.writeErr != nil {
		return s.writeErr
	}
	if s.secrets == nil {
		s.secrets = map[string]string{}
	}
	s.secrets[key] = value
	return nil
}

func (s *memSecretStore) DeleteSecret(key string) error {
	if s.deleteErr != nil {
		return s.deleteErr
	}
	delete(s.secrets, key)
	return nil
}

func TestDeleteWithSignatureSecretRestoresSecretWhenFileDeleteFails(t *testing.T) {
	dir := t.TempDir()
	mgr, err := NewManager(filepath.Join(dir, "webhooks.json"), filepath.Join(dir, "webhook_log.json"))
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	created, err := mgr.Create(Webhook{
		Name: "Signed", Slug: "signed-hook", Enabled: true,
		Delivery: DeliveryConfig{Mode: DeliveryModeSilent},
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	key := SignatureSecretVaultKey(created.ID)
	store := &memSecretStore{secrets: map[string]string{key: "signature-secret"}}
	mgr.filePath = filepath.Join(dir, "missing-parent", "webhooks.json")

	err = mgr.DeleteWithSignatureSecret(store, created.ID)
	if err == nil {
		t.Fatal("expected file delete to fail")
	}
	if errors.Is(err, ErrWebhookDeleteRestore) {
		t.Fatal("restore should succeed and the original delete error should remain")
	}
	if store.secrets[key] != "signature-secret" {
		t.Fatalf("secret = %q, want restored signature-secret", store.secrets[key])
	}
	if _, getErr := mgr.Get(created.ID); getErr != nil {
		t.Fatalf("webhook removed despite failed file delete: %v", getErr)
	}
}

func TestDeleteWithSignatureSecretRemovesVaultKey(t *testing.T) {
	dir := t.TempDir()
	mgr, err := NewManager(filepath.Join(dir, "webhooks.json"), filepath.Join(dir, "webhook_log.json"))
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	created, err := mgr.Create(Webhook{
		Name: "Signed", Slug: "signed-hook", Enabled: true,
		Delivery: DeliveryConfig{Mode: DeliveryModeSilent},
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	key := SignatureSecretVaultKey(created.ID)
	store := &memSecretStore{secrets: map[string]string{key: "signature-secret"}}
	if err := mgr.DeleteWithSignatureSecret(store, created.ID); err != nil {
		t.Fatalf("DeleteWithSignatureSecret: %v", err)
	}
	if _, ok := store.secrets[key]; ok {
		t.Fatal("vault key still present")
	}
	if _, err := mgr.Get(created.ID); err == nil {
		t.Fatal("webhook still present")
	}
}

func TestDeleteWithSignatureSecretKeepsWebhookWhenVaultDeleteFails(t *testing.T) {
	dir := t.TempDir()
	mgr, err := NewManager(filepath.Join(dir, "webhooks.json"), filepath.Join(dir, "webhook_log.json"))
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	created, err := mgr.Create(Webhook{
		Name: "Signed", Slug: "signed-hook", Enabled: true,
		Delivery: DeliveryConfig{Mode: DeliveryModeSilent},
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	key := SignatureSecretVaultKey(created.ID)
	store := &memSecretStore{
		secrets:   map[string]string{key: "signature-secret"},
		deleteErr: errors.New("vault locked"),
	}
	err = mgr.DeleteWithSignatureSecret(store, created.ID)
	if !errors.Is(err, ErrSignatureSecretDelete) {
		t.Fatalf("error = %v, want ErrSignatureSecretDelete", err)
	}
	if store.secrets[key] != "signature-secret" {
		t.Fatalf("secret changed despite failed vault delete: %q", store.secrets[key])
	}
	if _, getErr := mgr.Get(created.ID); getErr != nil {
		t.Fatalf("webhook removed despite vault failure: %v", getErr)
	}
}
