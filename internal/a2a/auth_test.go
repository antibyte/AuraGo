package a2a

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"aurago/internal/config"
)

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})
}

func TestAuthMiddleware_NoAuthConfigured(t *testing.T) {
	cfg := &config.Config{}
	handler := AuthMiddleware(cfg, okHandler())

	req := httptest.NewRequest(http.MethodGet, "/a2a/tasks", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestAuthMiddleware_APIKeyValid(t *testing.T) {
	cfg := &config.Config{}
	cfg.A2A.Auth.APIKeyEnabled = true
	cfg.A2A.Auth.APIKey = "test-key-123"

	handler := AuthMiddleware(cfg, okHandler())

	req := httptest.NewRequest(http.MethodGet, "/a2a/tasks", nil)
	req.Header.Set("X-API-Key", "test-key-123")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestAuthMiddleware_APIKeyInvalid(t *testing.T) {
	cfg := &config.Config{}
	cfg.A2A.Auth.APIKeyEnabled = true
	cfg.A2A.Auth.APIKey = "test-key-123"

	handler := AuthMiddleware(cfg, okHandler())

	req := httptest.NewRequest(http.MethodGet, "/a2a/tasks", nil)
	req.Header.Set("X-API-Key", "wrong-key")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestAuthMiddleware_APIKeyQueryParam(t *testing.T) {
	cfg := &config.Config{}
	cfg.A2A.Auth.APIKeyEnabled = true
	cfg.A2A.Auth.APIKey = "test-key-123"

	handler := AuthMiddleware(cfg, okHandler())

	req := httptest.NewRequest(http.MethodGet, "/a2a/tasks?api_key=test-key-123", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestAuthMiddleware_BearerValid(t *testing.T) {
	cfg := &config.Config{}
	cfg.A2A.Auth.BearerEnabled = true
	cfg.A2A.Auth.BearerSecret = "my-token"

	handler := AuthMiddleware(cfg, okHandler())

	req := httptest.NewRequest(http.MethodGet, "/a2a/tasks", nil)
	req.Header.Set("Authorization", "Bearer my-token")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestAuthMiddleware_BearerInvalid(t *testing.T) {
	cfg := &config.Config{}
	cfg.A2A.Auth.BearerEnabled = true
	cfg.A2A.Auth.BearerSecret = "my-token"

	handler := AuthMiddleware(cfg, okHandler())

	req := httptest.NewRequest(http.MethodGet, "/a2a/tasks", nil)
	req.Header.Set("Authorization", "Bearer wrong-token")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestAuthMiddleware_AgentCardBypassesAuth(t *testing.T) {
	cfg := &config.Config{}
	cfg.A2A.Auth.APIKeyEnabled = true
	cfg.A2A.Auth.APIKey = "test-key-123"

	handler := AuthMiddleware(cfg, okHandler())

	req := httptest.NewRequest(http.MethodGet, "/.well-known/agent-card.json", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	// Agent Card should be accessible without auth
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for agent card, got %d", w.Code)
	}
}

func TestAuthMiddleware_NoCredentials(t *testing.T) {
	cfg := &config.Config{}
	cfg.A2A.Auth.APIKeyEnabled = true
	cfg.A2A.Auth.APIKey = "test-key-123"

	handler := AuthMiddleware(cfg, okHandler())

	req := httptest.NewRequest(http.MethodGet, "/a2a/tasks", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 with no credentials, got %d", w.Code)
	}
}
