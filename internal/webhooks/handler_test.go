package webhooks

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"aurago/internal/config"
	"aurago/internal/security"
)

func TestHandlerUsesVaultBackedSignatureSecret(t *testing.T) {
	t.Parallel()

	vault, err := security.NewVault("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", filepath.Join(t.TempDir(), "vault.bin"))
	if err != nil {
		t.Fatalf("NewVault() error = %v", err)
	}
	tokenManager, err := security.NewTokenManager(vault, filepath.Join(t.TempDir(), "tokens.bin"))
	if err != nil {
		t.Fatalf("NewTokenManager() error = %v", err)
	}
	rawToken, _, err := tokenManager.Create("webhook test", []string{"webhook"}, nil)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	manager, err := NewManager(filepath.Join(t.TempDir(), "webhooks.json"), filepath.Join(t.TempDir(), "webhooks.log"))
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	created, err := manager.Create(Webhook{
		Name:    "Vault Hook",
		Slug:    "vault-hook",
		Enabled: true,
		TokenID: "tok-1",
		Format: WebhookFormat{
			AcceptedContentTypes: []string{"application/json"},
			SignatureHeader:      "X-Signature",
			SignatureAlgo:        "sha256",
		},
		Delivery: DeliveryConfig{Mode: DeliveryModeSilent, Priority: "queue"},
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if err := vault.WriteSecret(SignatureSecretVaultKey(created.ID), "vault-secret"); err != nil {
		t.Fatalf("WriteSecret() error = %v", err)
	}

	handler := NewHandler(manager, tokenManager, vault, nil, nil, &config.Config{}, nil, 8080, 4096, 0)
	body := []byte(`{"hello":"world"}`)
	mac := hmac.New(sha256.New, []byte("vault-secret"))
	mac.Write(body)
	signature := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	req := httptest.NewRequest(http.MethodPost, "/webhook/vault-hook", io.NopCloser(bytes.NewReader(body)))
	req.Header.Set("Authorization", "Bearer "+rawToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Signature", signature)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
}
