package webhooks

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/security"
)

func TestManagerUpdateWithOptionsPreservesOmittedFields(t *testing.T) {
	t.Parallel()

	mgr, err := NewManager(filepath.Join(t.TempDir(), "webhooks.json"), filepath.Join(t.TempDir(), "webhooks.log"))
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	created, err := mgr.Create(Webhook{
		Name:    "Original",
		Slug:    "original-hook",
		Enabled: true,
		TokenID: "tok-1",
		Format: WebhookFormat{
			AcceptedContentTypes: []string{"application/json"},
			SignatureHeader:      "X-Signature",
			SignatureAlgo:        "sha256",
			SignatureSecret:      "secret",
		},
		Delivery: DeliveryConfig{Mode: DeliveryModeSilent, Priority: "queue"},
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	updated, err := mgr.UpdateWithOptions(created.ID, Webhook{Name: "Renamed"}, UpdateOptions{})
	if err != nil {
		t.Fatalf("UpdateWithOptions() error = %v", err)
	}
	if !updated.Enabled {
		t.Fatal("Enabled was changed even though enabled was omitted")
	}
	if updated.Format.SignatureHeader != "X-Signature" || updated.Format.SignatureAlgo != "sha256" || updated.Format.SignatureSecret != "secret" {
		t.Fatalf("signature fields were not preserved: %+v", updated.Format)
	}
}

func TestManagerMigratesPlaintextSignatureSecretsToVaultIdempotently(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	webhooksPath := filepath.Join(dir, "webhooks.json")
	mgr, err := NewManager(webhooksPath, filepath.Join(dir, "webhooks.log"))
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	created, err := mgr.Create(Webhook{
		Name:    "Legacy",
		Slug:    "legacy-hook",
		Enabled: true,
		Format: WebhookFormat{
			AcceptedContentTypes: []string{"application/json"},
			SignatureHeader:      "X-Signature",
			SignatureAlgo:        "sha256",
			SignatureSecret:      "legacy-secret",
		},
		Delivery: DeliveryConfig{Mode: DeliveryModeSilent},
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	vault, err := security.NewVault("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", filepath.Join(dir, "vault.bin"))
	if err != nil {
		t.Fatalf("NewVault() error = %v", err)
	}

	if err := mgr.MigrateSignatureSecrets(vault); err != nil {
		t.Fatalf("MigrateSignatureSecrets() error = %v", err)
	}
	if err := mgr.MigrateSignatureSecrets(vault); err != nil {
		t.Fatalf("second MigrateSignatureSecrets() error = %v", err)
	}
	secret, err := vault.ReadSecret(SignatureSecretVaultKey(created.ID))
	if err != nil || secret != "legacy-secret" {
		t.Fatalf("vault secret = %q, err=%v", secret, err)
	}
	data, err := os.ReadFile(webhooksPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if strings.Contains(string(data), "legacy-secret") {
		t.Fatalf("webhooks.json still contains plaintext secret: %s", data)
	}
	stored, err := mgr.Get(created.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if stored.Format.SignatureSecret != "" {
		t.Fatalf("runtime plaintext secret = %q, want empty", stored.Format.SignatureSecret)
	}
}

func TestManagerUpdateWithOptionsAllowsExplicitDisableAndSignatureClear(t *testing.T) {
	t.Parallel()

	mgr, err := NewManager(filepath.Join(t.TempDir(), "webhooks.json"), filepath.Join(t.TempDir(), "webhooks.log"))
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	created, err := mgr.Create(Webhook{
		Name:    "Original",
		Slug:    "clear-hook",
		Enabled: true,
		TokenID: "tok-1",
		Format: WebhookFormat{
			AcceptedContentTypes: []string{"application/json"},
			SignatureHeader:      "X-Signature",
			SignatureAlgo:        "sha256",
			SignatureSecret:      "secret",
		},
		Delivery: DeliveryConfig{Mode: DeliveryModeSilent, Priority: "queue"},
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	updated, err := mgr.UpdateWithOptions(created.ID, Webhook{Enabled: false}, UpdateOptions{
		EnabledSet:         true,
		SignatureHeaderSet: true,
		SignatureAlgoSet:   true,
		SignatureSecretSet: true,
	})
	if err != nil {
		t.Fatalf("UpdateWithOptions() error = %v", err)
	}
	if updated.Enabled {
		t.Fatal("Enabled = true, want false")
	}
	if updated.Format.SignatureHeader != "" || updated.Format.SignatureAlgo != "" || updated.Format.SignatureSecret != "" {
		t.Fatalf("signature fields were not cleared: %+v", updated.Format)
	}
}

func TestManagerNormalizesNewDeliveryPriorityToAsyncQueue(t *testing.T) {
	t.Parallel()

	mgr, err := NewManager(filepath.Join(t.TempDir(), "webhooks.json"), filepath.Join(t.TempDir(), "webhooks.log"))
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	created, err := mgr.Create(Webhook{
		Name:     "Priority",
		Slug:     "priority-hook",
		Delivery: DeliveryConfig{Mode: DeliveryModeSilent, Priority: "immediate"},
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if created.Delivery.Priority != "queue" {
		t.Fatalf("created priority = %q, want queue", created.Delivery.Priority)
	}
	updated, err := mgr.UpdateWithOptions(created.ID, Webhook{Delivery: DeliveryConfig{Priority: "immediate"}}, UpdateOptions{})
	if err != nil {
		t.Fatalf("UpdateWithOptions() error = %v", err)
	}
	if updated.Delivery.Priority != "queue" {
		t.Fatalf("updated priority = %q, want queue", updated.Delivery.Priority)
	}
}
