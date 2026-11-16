package a2a

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"aurago/internal/config"
)

// AuthMiddleware wraps an http.Handler and enforces A2A auth based on config.
// It skips authentication for the Agent Card endpoint.
func AuthMiddleware(cfg *config.Config, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Agent Card endpoint is always public (discovery must work unauthenticated)
		if strings.HasSuffix(r.URL.Path, "/.well-known/agent-card.json") {
			next.ServeHTTP(w, r)
			return
		}

		if validCredentials(cfg, r.Header.Get("X-API-Key"), r.Header.Get("Authorization")) {
			next.ServeHTTP(w, r)
			return
		}

		http.Error(w, "Unauthorized", http.StatusUnauthorized)
	})
}

// Both transports use the configured credentials; discovery is HTTP-only.
func validCredentials(cfg *config.Config, apiKey, authorization string) bool {
	if cfg == nil {
		return false
	}
	auth := &cfg.A2A.Auth
	if auth.APIKeyEnabled && auth.APIKey != "" && apiKey != "" && subtle.ConstantTimeCompare([]byte(apiKey), []byte(auth.APIKey)) == 1 {
		return true
	}
	token, bearer := strings.CutPrefix(authorization, "Bearer ")
	return auth.BearerEnabled && auth.BearerSecret != "" && bearer && token != "" && subtle.ConstantTimeCompare([]byte(token), []byte(auth.BearerSecret)) == 1
}
