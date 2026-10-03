package webhooks

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/security"
)

func TestHandlerRateLimitedRequestDoesNotTouchToken(t *testing.T) {
	t.Parallel()

	vault, err := security.NewVault("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", filepath.Join(t.TempDir(), "vault.bin"))
	if err != nil {
		t.Fatalf("NewVault() error = %v", err)
	}
	tokenManager, err := security.NewTokenManager(vault, filepath.Join(t.TempDir(), "tokens.bin"))
	if err != nil {
		t.Fatalf("NewTokenManager() error = %v", err)
	}
	rawToken, tokenMeta, err := tokenManager.Create("webhook test", []string{"webhook"}, nil)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	manager, err := NewManager(filepath.Join(t.TempDir(), "webhooks.json"), filepath.Join(t.TempDir(), "webhooks.log"))
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	if _, err := manager.Create(Webhook{
		Name:     "Limited Hook",
		Slug:     "limited-hook",
		Enabled:  true,
		TokenID:  tokenMeta.ID,
		Format:   WebhookFormat{AcceptedContentTypes: []string{"application/json"}},
		Delivery: DeliveryConfig{Mode: DeliveryModeSilent},
	}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	handler := NewHandler(manager, tokenManager, vault, nil, nil, &config.Config{}, slog.Default(), 8080, 4096, 1)
	if !handler.rateLimiter.Allow(tokenMeta.ID) { // consume the only token of the bucket
		t.Fatal("priming call was rate limited")
	}

	req := httptest.NewRequest(http.MethodPost, "/webhook/limited-hook", strings.NewReader(`{"ok":true}`))
	req.Header.Set("Authorization", "Bearer "+rawToken)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429", rec.Code)
	}
	meta, err := tokenManager.Get(tokenMeta.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if meta.LastUsedAt != nil {
		t.Fatalf("rate-limited webhook recorded token use at %v", meta.LastUsedAt)
	}
}
