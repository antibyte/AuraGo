package webhooks

import (
	"path/filepath"
	"testing"
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
