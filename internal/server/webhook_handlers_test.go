package server

import (
	"aurago/internal/config"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/security"
	"aurago/internal/webhooks"
)

func TestHandleUpdateWebhookNotFoundReturnsGenericError(t *testing.T) {
	t.Parallel()

	mgr, err := webhooks.NewManager(t.TempDir()+"\\webhooks.json", t.TempDir()+"\\webhooks.log")
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodPut, "/api/webhooks/missing-id", strings.NewReader(`{"name":"updated"}`))
	rec := httptest.NewRecorder()

	handleUpdateWebhook(&Server{}, mgr).ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusNotFound, rec.Body.String())
	}
	if body := rec.Body.String(); !strings.Contains(body, "Webhook not found") || strings.Contains(strings.ToLower(body), "missing-id") {
		t.Fatalf("expected generic not-found response, got %q", body)
	}
}

func TestHandleCreateWebhookStoresSignatureSecretInVault(t *testing.T) {
	t.Parallel()

	vault, err := security.NewVault("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", filepath.Join(t.TempDir(), "vault.bin"))
	if err != nil {
		t.Fatalf("NewVault() error = %v", err)
	}
	mgr, err := webhooks.NewManager(filepath.Join(t.TempDir(), "webhooks.json"), filepath.Join(t.TempDir(), "webhooks.log"))
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	server := &Server{Vault: vault, Logger: slog.Default()}

	req := httptest.NewRequest(http.MethodPost, "/api/webhooks", strings.NewReader(`{
		"name":"GitHub Hook",
		"slug":"github-hook",
		"enabled":true,
		"token_id":"tok-1",
		"format":{"accepted_content_types":["application/json"],"signature_secret":"super-secret"},
		"delivery":{"mode":"message","priority":"queue"}
	}`))
	rec := httptest.NewRecorder()

	handleCreateWebhook(server, mgr).ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusCreated, rec.Body.String())
	}

	var created webhooks.Webhook
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if created.Format.SignatureSecret != maskedKey {
		t.Fatalf("response signature_secret = %q, want masked", created.Format.SignatureSecret)
	}

	stored, err := mgr.Get(created.ID)
	if err != nil {
		t.Fatalf("mgr.Get() error = %v", err)
	}
	if stored.Format.SignatureSecret != "" {
		t.Fatalf("stored signature_secret = %q, want empty manager storage", stored.Format.SignatureSecret)
	}

	secret, err := vault.ReadSecret(webhooks.SignatureSecretVaultKey(created.ID))
	if err != nil {
		t.Fatalf("vault.ReadSecret() error = %v", err)
	}
	if secret != "super-secret" {
		t.Fatalf("vault secret = %q, want super-secret", secret)
	}
}

func TestHandleListWebhooksMasksVaultBackedSignatureSecret(t *testing.T) {
	t.Parallel()

	vault, err := security.NewVault("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", filepath.Join(t.TempDir(), "vault.bin"))
	if err != nil {
		t.Fatalf("NewVault() error = %v", err)
	}
	mgr, err := webhooks.NewManager(filepath.Join(t.TempDir(), "webhooks.json"), filepath.Join(t.TempDir(), "webhooks.log"))
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	server := &Server{Vault: vault, Logger: slog.Default()}

	created, err := mgr.Create(webhooks.Webhook{
		Name:    "Masked Hook",
		Slug:    "masked-hook",
		Enabled: true,
		TokenID: "tok-1",
		Format: webhooks.WebhookFormat{
			AcceptedContentTypes: []string{"application/json"},
		},
		Delivery: webhooks.DeliveryConfig{Mode: webhooks.DeliveryModeMessage, Priority: "queue"},
	})
	if err != nil {
		t.Fatalf("mgr.Create() error = %v", err)
	}
	if err := vault.WriteSecret(webhooks.SignatureSecretVaultKey(created.ID), "stored-in-vault"); err != nil {
		t.Fatalf("vault.WriteSecret() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/webhooks", nil)
	rec := httptest.NewRecorder()

	handleListWebhooks(server, mgr).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var list []webhooks.Webhook
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("len(list) = %d, want 1", len(list))
	}
	if list[0].Format.SignatureSecret != maskedKey {
		t.Fatalf("signature_secret = %q, want masked", list[0].Format.SignatureSecret)
	}
}

func TestHandlePutOutgoingWebhooksReloadsVaultBackedAuthSecrets(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte("auth:\n  enabled: true\nwebhooks:\n  outgoing: []\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	vault, err := security.NewVault(strings.Repeat("d", 64), filepath.Join(tmpDir, "vault.bin"))
	if err != nil {
		t.Fatalf("NewVault() error = %v", err)
	}
	passwordHash, err := HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}
	if err := vault.WriteSecret("auth_password_hash", passwordHash); err != nil {
		t.Fatalf("vault.WriteSecret() error = %v", err)
	}

	server := &Server{
		Cfg:    &config.Config{ConfigPath: configPath},
		Vault:  vault,
		Logger: logger,
	}

	req := httptest.NewRequest(http.MethodPut, "/api/outgoing-webhooks", strings.NewReader(`[
		{
			"id":"wh_1",
			"name":"Test Webhook",
			"description":"demo",
			"method":"POST",
			"url":"https://example.com/hook",
			"headers":{"Content-Type":"application/json"},
			"parameters":[],
			"payload_type":"json",
			"body_template":"{\"ok\":true}"
		}
	]`))
	rec := httptest.NewRecorder()

	handlePutOutgoingWebhooks(server, rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if !server.Cfg.Auth.Enabled {
		t.Fatalf("auth.enabled = false, want true")
	}
	if server.Cfg.Auth.PasswordHash != passwordHash {
		t.Fatalf("password hash not reloaded from vault; got %q", server.Cfg.Auth.PasswordHash)
	}
}
