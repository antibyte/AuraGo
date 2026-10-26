package server

import (
	"aurago/internal/config"
	"fmt"
	"time"
)

// OAuth exchanges run outside the save lock. Before storing their result, the
// original grant target must still describe the saved provider.
func sameProviderOAuthTarget(a, b config.ProviderEntry) bool {
	return a.ID == b.ID && normalizeProviderAuthType(b.AuthType) == "oauth2" &&
		a.OAuthAuthURL == b.OAuthAuthURL && a.OAuthTokenURL == b.OAuthTokenURL &&
		a.OAuthClientID == b.OAuthClientID && a.OAuthClientSecret == b.OAuthClientSecret
}
func storeProviderOAuthToken(s *Server, expected config.ProviderEntry, response *tokenExchangeResponse, now time.Time) error {
	s.CfgSaveMu.Lock()
	defer s.CfgSaveMu.Unlock()
	s.CfgMu.RLock()
	current := s.Cfg.FindProvider(expected.ID)
	matches := current != nil && sameProviderOAuthTarget(expected, *current)
	s.CfgMu.RUnlock()
	if !matches {
		return fmt.Errorf("provider OAuth settings changed; start authorization again")
	}
	if err := storeOAuthToken(s.Vault, expected.ID, response, now); err != nil {
		return err
	}
	applyOAuthTokenToRuntime(s)
	return nil
}
func storeRefreshedProviderOAuth(s *Server, expected config.ProviderEntry, old, updated string) error {
	s.CfgSaveMu.Lock()
	defer s.CfgSaveMu.Unlock()
	s.CfgMu.RLock()
	current := s.Cfg.FindProvider(expected.ID)
	matches := current != nil && sameProviderOAuthTarget(expected, *current)
	s.CfgMu.RUnlock()
	if !matches {
		return fmt.Errorf("provider OAuth settings changed during refresh")
	}
	if err := s.Vault.CompareAndSwapSecret("oauth_"+expected.ID, old, updated); err != nil {
		return err
	}
	applyOAuthTokenToRuntime(s)
	return nil
}
func revokeProviderOAuth(s *Server, id string) error {
	s.CfgSaveMu.Lock()
	defer s.CfgSaveMu.Unlock()
	if s.Vault != nil {
		if err := s.Vault.DeleteSecret("oauth_" + id); err != nil {
			return fmt.Errorf("revoke provider OAuth: %w", err)
		}
	}
	applyOAuthTokenToRuntime(s)
	s.invalidateDecisionsProvider(id)
	return nil
}
