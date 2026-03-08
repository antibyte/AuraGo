package server

import (
	"context"
	"encoding/json"
	"time"

	"aurago/internal/config"
)

// startOAuthRefreshLoop runs a background goroutine that periodically checks
// all OAuth2 providers and refreshes tokens that are about to expire.
// Pattern matches the probeLoop in failover.go.
func startOAuthRefreshLoop(s *Server, ctx context.Context) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				s.Logger.Error("[OAuth Refresh] Goroutine panic recovered", "error", r)
			}
		}()

		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()

		// Also do an initial check shortly after startup
		initialTimer := time.NewTimer(30 * time.Second)
		defer initialTimer.Stop()

		for {
			select {
			case <-ctx.Done():
				s.Logger.Info("[OAuth Refresh] Shutting down")
				return
			case <-initialTimer.C:
				refreshAllOAuthTokens(s)
			case <-ticker.C:
				refreshAllOAuthTokens(s)
			}
		}
	}()
}

// refreshAllOAuthTokens iterates all providers with auth_type "oauth2" and
// refreshes tokens that expire within the next 10 minutes.
func refreshAllOAuthTokens(s *Server) {
	if s.Vault == nil {
		return
	}

	s.CfgMu.RLock()
	providers := make([]config.ProviderEntry, len(s.Cfg.Providers))
	copy(providers, s.Cfg.Providers)
	s.CfgMu.RUnlock()

	refreshed := 0
	for _, prov := range providers {
		if prov.AuthType != "oauth2" {
			continue
		}
		if prov.OAuthTokenURL == "" {
			continue
		}

		raw, err := s.Vault.ReadSecret("oauth_" + prov.ID)
		if err != nil || raw == "" {
			continue
		}

		var tok config.OAuthToken
		if err := json.Unmarshal([]byte(raw), &tok); err != nil {
			continue
		}

		// No refresh token → nothing to refresh
		if tok.RefreshToken == "" {
			continue
		}

		// Check if token needs refresh (expires within 10 minutes)
		if tok.Expiry != "" {
			expiry, err := time.Parse(time.RFC3339, tok.Expiry)
			if err == nil && time.Until(expiry) > 10*time.Minute {
				continue // token still valid for more than 10 minutes
			}
		}

		// Refresh the token
		s.Logger.Info("[OAuth Refresh] Refreshing token", "provider", prov.ID)
		newTok, err := refreshOAuthToken(prov, tok.RefreshToken)
		if err != nil {
			s.Logger.Error("[OAuth Refresh] Failed to refresh token", "provider", prov.ID, "error", err)
			continue
		}

		// Update vault
		updated := config.OAuthToken{
			AccessToken:  newTok.AccessToken,
			TokenType:    newTok.TokenType,
			RefreshToken: newTok.RefreshToken,
		}
		// Some providers don't return a new refresh_token; keep the old one
		if updated.RefreshToken == "" {
			updated.RefreshToken = tok.RefreshToken
		}
		if newTok.ExpiresIn > 0 {
			updated.Expiry = time.Now().Add(time.Duration(newTok.ExpiresIn) * time.Second).UTC().Format(time.RFC3339)
		}

		tokJSON, _ := json.Marshal(updated)
		if err := s.Vault.WriteSecret("oauth_"+prov.ID, string(tokJSON)); err != nil {
			s.Logger.Error("[OAuth Refresh] Failed to store refreshed token", "provider", prov.ID, "error", err)
			continue
		}

		refreshed++
		s.Logger.Info("[OAuth Refresh] Token refreshed successfully", "provider", prov.ID)
	}

	if refreshed > 0 {
		// Apply updated tokens to live config
		s.CfgMu.Lock()
		s.Cfg.ApplyOAuthTokens(s.Vault)
		s.CfgMu.Unlock()
		s.Logger.Info("[OAuth Refresh] Applied refreshed tokens", "count", refreshed)
	}
}
