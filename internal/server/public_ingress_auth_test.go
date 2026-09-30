package server

import (
	"aurago/internal/config"
	"aurago/internal/security"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func newPublicIngressTestServer(t *testing.T) (*Server, string, http.Handler) {
	t.Helper()
	dir := t.TempDir()
	vault, err := security.NewVault(strings.Repeat("e", 64), filepath.Join(dir, "vault.bin"))
	if err != nil {
		t.Fatalf("NewVault: %v", err)
	}
	tokens, err := security.NewTokenManager(vault, filepath.Join(dir, "tokens.bin"))
	if err != nil {
		t.Fatalf("NewTokenManager: %v", err)
	}
	webhookToken, _, err := tokens.Create("hook", []string{"webhook"}, nil)
	if err != nil {
		t.Fatalf("Create token: %v", err)
	}
	s := &Server{Cfg: &config.Config{}, Logger: slog.Default(), TokenManager: tokens}
	s.Cfg.Auth.Enabled = true
	s.Cfg.Auth.SessionSecret = "0123456789abcdef0123456789abcdef"
	s.Cfg.Auth.PasswordHash = "configured"

	reached := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusAccepted) })
	mux := http.NewServeMux()
	mux.Handle("/webhook/", reached)
	mux.Handle("/api/telnyx/webhook", reached)
	mux.Handle("/api/config", reached)
	return s, webhookToken, authMiddleware(s, mux)
}

func postIngress(handler http.Handler, path string, headers map[string]string) int {
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{}`))
	req.RemoteAddr = "198.51.100.9:443"
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec.Code
}

func TestAuthMiddlewareLetsWebhookReceiverAuthenticateItself(t *testing.T) {
	_, webhookToken, handler := newPublicIngressTestServer(t)

	if code := postIngress(handler, "/webhook/github", map[string]string{"Authorization": "Bearer " + webhookToken}); code != http.StatusAccepted {
		t.Fatalf("webhook with webhook-scoped bearer: status = %d, want handler reached", code)
	}
	if code := postIngress(handler, "/webhook/github", map[string]string{"X-Hub-Signature-256": "sha256=abc"}); code != http.StatusAccepted {
		t.Fatalf("HMAC-signed webhook without session: status = %d, want handler reached", code)
	}
}

func TestAuthMiddlewareKeepsWebhookReceiverClosedDuringLockdown(t *testing.T) {
	s, webhookToken, handler := newPublicIngressTestServer(t)
	s.Cfg.Auth.PasswordHash = ""

	if code := postIngress(handler, "/webhook/github", map[string]string{"Authorization": "Bearer " + webhookToken}); code != http.StatusTemporaryRedirect {
		t.Fatalf("webhook during password lockdown: status = %d, want 307 to setup", code)
	}
}

func TestAuthMiddlewareBypassesOnlyTheRegisteredTelnyxWebhookPath(t *testing.T) {
	s, _, handler := newPublicIngressTestServer(t)

	if code := postIngress(handler, "/api/telnyx/webhook", nil); code != http.StatusUnauthorized {
		t.Fatalf("unregistered Telnyx path: status = %d, want 401", code)
	}

	s.registerTelnyxWebhookPath("/api/telnyx/webhook")
	if code := postIngress(handler, "/api/telnyx/webhook", map[string]string{"Telnyx-Signature-Ed25519": "sig"}); code != http.StatusAccepted {
		t.Fatalf("registered Telnyx webhook without session: status = %d, want handler reached", code)
	}

	// A later config edit must not move the bypass onto a different handler.
	s.Cfg.Telnyx.WebhookPath = "/api/config"
	if code := postIngress(handler, "/api/config", nil); code != http.StatusUnauthorized {
		t.Fatalf("/api/config after runtime Telnyx path change: status = %d, want 401", code)
	}
	if code := postIngress(handler, "/api/telnyx/webhook/extra", nil); code != http.StatusUnauthorized {
		t.Fatalf("Telnyx bypass leaked to a sub-path: status = %d, want 401", code)
	}
}
