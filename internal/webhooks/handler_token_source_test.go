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

func TestHandlerResolvesTokenManagerThroughSource(t *testing.T) {
	t.Parallel()

	vault, err := security.NewVault("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", filepath.Join(t.TempDir(), "vault.bin"))
	if err != nil {
		t.Fatalf("NewVault() error = %v", err)
	}
	staleTokens, err := security.NewTokenManager(vault, filepath.Join(t.TempDir(), "stale-tokens.bin"))
	if err != nil {
		t.Fatalf("NewTokenManager(stale) error = %v", err)
	}
	liveTokens, err := security.NewTokenManager(vault, filepath.Join(t.TempDir(), "live-tokens.bin"))
	if err != nil {
		t.Fatalf("NewTokenManager(live) error = %v", err)
	}
	rawToken, tokenMeta, err := liveTokens.Create("live webhook", []string{"webhook"}, nil)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	manager, err := NewManager(filepath.Join(t.TempDir(), "webhooks.json"), filepath.Join(t.TempDir(), "webhooks.log"))
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	if _, err := manager.Create(Webhook{
		Name:     "Live Hook",
		Slug:     "live-hook",
		Enabled:  true,
		TokenID:  tokenMeta.ID,
		Format:   WebhookFormat{AcceptedContentTypes: []string{"application/json"}},
		Delivery: DeliveryConfig{Mode: DeliveryModeSilent},
	}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	handler := NewHandler(manager, staleTokens, vault, nil, nil, &config.Config{}, slog.Default(), 8080, 4096, 0)
	send := func() int {
		req := httptest.NewRequest(http.MethodPost, "/webhook/live-hook", strings.NewReader(`{"ok":true}`))
		req.Header.Set("Authorization", "Bearer "+rawToken)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec.Code
	}

	if code := send(); code != http.StatusUnauthorized {
		t.Fatalf("stale token manager accepted a token it does not hold: status %d", code)
	}
	handler.SetTokenManagerSource(func() *security.TokenManager { return liveTokens })
	if code := send(); code != http.StatusOK {
		t.Fatalf("handler did not resolve the live token manager: status %d", code)
	}
}
