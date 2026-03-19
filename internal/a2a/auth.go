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

		auth := &cfg.A2A.Auth
		if !auth.APIKeyEnabled && !auth.BearerEnabled {
			// No authentication configured — allow all
			next.ServeHTTP(w, r)
			return
		}

		// Try API Key auth (via X-API-Key header or query parameter)
		if auth.APIKeyEnabled && auth.APIKey != "" {
			if key := r.Header.Get("X-API-Key"); key != "" {
				if subtle.ConstantTimeCompare([]byte(key), []byte(auth.APIKey)) == 1 {
					next.ServeHTTP(w, r)
					return
				}
			}
			if key := r.URL.Query().Get("api_key"); key != "" {
				if subtle.ConstantTimeCompare([]byte(key), []byte(auth.APIKey)) == 1 {
					next.ServeHTTP(w, r)
					return
				}
			}
		}

		// Try Bearer token auth (via Authorization header)
		if auth.BearerEnabled && auth.BearerSecret != "" {
			if authHeader := r.Header.Get("Authorization"); authHeader != "" {
				if token, ok := strings.CutPrefix(authHeader, "Bearer "); ok {
					if subtle.ConstantTimeCompare([]byte(token), []byte(auth.BearerSecret)) == 1 {
						next.ServeHTTP(w, r)
						return
					}
				}
			}
		}

		http.Error(w, "Unauthorized", http.StatusUnauthorized)
	})
}
