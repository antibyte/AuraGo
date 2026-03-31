package server

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"aurago/internal/config"
)

// ── OAuth2 Authorization Code Flow ──────────────────────────────────────────

// handleOAuthStart initiates the OAuth2 Authorization Code flow for a provider.
// GET /api/oauth/start?provider=<id>
// Returns JSON: {"auth_url": "https://..."}
func handleOAuthStart(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		providerID := r.URL.Query().Get("provider")
		if providerID == "" {
			jsonError(w, "Missing 'provider' query parameter", http.StatusBadRequest)
			return
		}

		s.CfgMu.RLock()
		prov := s.Cfg.FindProvider(providerID)
		var entry config.ProviderEntry
		if prov != nil {
			entry = *prov
		}
		serverPort := s.Cfg.Server.Port
		serverHost := s.Cfg.Server.Host
		s.CfgMu.RUnlock()

		if prov == nil {
			jsonError(w, "Provider not found: "+providerID, http.StatusNotFound)
			return
		}
		if entry.AuthType != "oauth2" {
			jsonError(w, "Provider is not configured for OAuth2", http.StatusBadRequest)
			return
		}
		if entry.OAuthAuthURL == "" || entry.OAuthTokenURL == "" || entry.OAuthClientID == "" {
			jsonError(w, "OAuth2 configuration incomplete (need auth_url, token_url, client_id)", http.StatusBadRequest)
			return
		}

		if s.Vault == nil {
			jsonError(w, "Vault not available — cannot store OAuth state", http.StatusServiceUnavailable)
			return
		}

		// Generate random state for CSRF protection
		stateBytes := make([]byte, 32)
		if _, err := rand.Read(stateBytes); err != nil {
			s.Logger.Error("[OAuth] Failed to generate state", "error", err)
			jsonError(w, "Internal error", http.StatusInternalServerError)
			return
		}
		state := hex.EncodeToString(stateBytes)

		// Store state → provider mapping in vault (expires implicitly on use)
		stateData := map[string]string{
			"provider_id": providerID,
			"created_at":  time.Now().UTC().Format(time.RFC3339),
		}
		stateJSON, _ := json.Marshal(stateData)
		if err := s.Vault.WriteSecret("oauth_state_"+state, string(stateJSON)); err != nil {
			s.Logger.Error("[OAuth] Failed to store state", "error", err)
			jsonError(w, "Failed to store OAuth state", http.StatusInternalServerError)
			return
		}

		// Build redirect URI
		s.CfgMu.RLock()
		oauthBase := s.Cfg.Server.OAuthRedirectBaseURL
		s.CfgMu.RUnlock()
		redirectURI := buildRedirectURI(r, serverHost, serverPort, oauthBase)

		// Build authorization URL
		authURL, err := url.Parse(entry.OAuthAuthURL)
		if err != nil {
			jsonError(w, "Invalid oauth_auth_url", http.StatusBadRequest)
			return
		}
		q := authURL.Query()
		q.Set("response_type", "code")
		q.Set("client_id", entry.OAuthClientID)
		q.Set("redirect_uri", redirectURI)
		q.Set("state", state)
		if entry.OAuthScopes != "" {
			q.Set("scope", entry.OAuthScopes)
		}
		// Some providers want access_type=offline to return a refresh_token
		q.Set("access_type", "offline")
		q.Set("prompt", "consent")
		authURL.RawQuery = q.Encode()

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"auth_url": authURL.String(),
		})
	}
}

// handleOAuthCallback handles the redirect back from the authorization server.
// GET /api/oauth/callback?code=...&state=...
func handleOAuthCallback(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		code := r.URL.Query().Get("code")
		state := r.URL.Query().Get("state")
		errParam := r.URL.Query().Get("error")

		if errParam != "" {
			errDesc := r.URL.Query().Get("error_description")
			renderOAuthResult(w, false, fmt.Sprintf("Authorization denied: %s — %s", errParam, errDesc))
			return
		}

		if code == "" || state == "" {
			renderOAuthResult(w, false, "Missing code or state parameter")
			return
		}

		if s.Vault == nil {
			renderOAuthResult(w, false, "Vault not available")
			return
		}

		// Validate and consume state
		stateRaw, err := s.Vault.ReadSecret("oauth_state_" + state)
		if err != nil || stateRaw == "" {
			renderOAuthResult(w, false, "Invalid or expired state parameter")
			return
		}
		_ = s.Vault.DeleteSecret("oauth_state_" + state) // one-time use

		var stateData map[string]string
		if err := json.Unmarshal([]byte(stateRaw), &stateData); err != nil {
			renderOAuthResult(w, false, "Corrupt state data")
			return
		}
		providerID := stateData["provider_id"]

		// Look up provider config
		s.CfgMu.RLock()
		prov := s.Cfg.FindProvider(providerID)
		var entry config.ProviderEntry
		if prov != nil {
			entry = *prov
		}
		serverPort := s.Cfg.Server.Port
		serverHost := s.Cfg.Server.Host
		s.CfgMu.RUnlock()

		if prov == nil {
			renderOAuthResult(w, false, "Provider '"+providerID+"' not found in config")
			return
		}

		// Exchange authorization code for tokens
		s.CfgMu.RLock()
		oauthBase := s.Cfg.Server.OAuthRedirectBaseURL
		s.CfgMu.RUnlock()
		redirectURI := buildRedirectURI(r, serverHost, serverPort, oauthBase)

		tokenResp, err := exchangeCodeForToken(entry, code, redirectURI)
		if err != nil {
			s.Logger.Error("[OAuth] Token exchange failed", "provider", providerID, "error", err)
			renderOAuthResult(w, false, "Token exchange failed")
			return
		}

		// Store tokens in vault
		tok := config.OAuthToken{
			AccessToken:  tokenResp.AccessToken,
			RefreshToken: tokenResp.RefreshToken,
			TokenType:    tokenResp.TokenType,
		}
		if tokenResp.ExpiresIn > 0 {
			tok.Expiry = time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second).UTC().Format(time.RFC3339)
		}
		tokJSON, _ := json.Marshal(tok)
		if err := s.Vault.WriteSecret("oauth_"+providerID, string(tokJSON)); err != nil {
			s.Logger.Error("[OAuth] Failed to store token", "provider", providerID, "error", err)
			renderOAuthResult(w, false, "Failed to store token")
			return
		}

		// Apply updated token to live config
		s.CfgMu.Lock()
		s.Cfg.ApplyOAuthTokens(s.Vault)
		s.CfgMu.Unlock()

		s.Logger.Info("[OAuth] Authorization successful", "provider", providerID)
		renderOAuthResult(w, true, "Authorization successful for provider: "+providerID)
	}
}

// handleOAuthStatus returns the OAuth token status for a provider.
// GET /api/oauth/status?provider=<id>
func handleOAuthStatus(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		providerID := r.URL.Query().Get("provider")
		if providerID == "" {
			jsonError(w, "Missing 'provider' query parameter", http.StatusBadRequest)
			return
		}

		result := map[string]interface{}{
			"provider":   providerID,
			"authorized": false,
		}

		if s.Vault == nil {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(result)
			return
		}

		raw, err := s.Vault.ReadSecret("oauth_" + providerID)
		if err != nil || raw == "" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(result)
			return
		}

		var tok config.OAuthToken
		if err := json.Unmarshal([]byte(raw), &tok); err != nil {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(result)
			return
		}

		result["authorized"] = tok.AccessToken != ""
		if tok.Expiry != "" {
			if expiry, err := time.Parse(time.RFC3339, tok.Expiry); err == nil {
				result["expiry"] = tok.Expiry
				result["expired"] = time.Now().After(expiry)
			}
		}
		if tok.RefreshToken != "" {
			result["has_refresh_token"] = true
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	}
}

// handleOAuthRevoke deletes stored OAuth tokens for a provider.
// DELETE /api/oauth/revoke?provider=<id>
func handleOAuthRevoke(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		providerID := r.URL.Query().Get("provider")
		if providerID == "" {
			jsonError(w, "Missing 'provider' query parameter", http.StatusBadRequest)
			return
		}

		if s.Vault != nil {
			_ = s.Vault.DeleteSecret("oauth_" + providerID)
		}

		// Clear the live token
		s.CfgMu.Lock()
		if p := s.Cfg.FindProvider(providerID); p != nil && p.AuthType == "oauth2" {
			p.APIKey = ""
		}
		s.CfgMu.Unlock()

		s.Logger.Info("[OAuth] Token revoked", "provider", providerID)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}
}

// ── Helpers ─────────────────────────────────────────────────────────────────

// tokenExchangeResponse is the JSON response from the token endpoint.
type tokenExchangeResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"`
	Scope        string `json:"scope"`
	Error        string `json:"error"`
	ErrorDesc    string `json:"error_description"`
}

// exchangeCodeForToken performs the OAuth2 token exchange.
func exchangeCodeForToken(prov config.ProviderEntry, code, redirectURI string) (*tokenExchangeResponse, error) {
	data := url.Values{
		"grant_type":   {"authorization_code"},
		"code":         {code},
		"redirect_uri": {redirectURI},
		"client_id":    {prov.OAuthClientID},
	}
	if prov.OAuthClientSecret != "" {
		data.Set("client_secret", prov.OAuthClientSecret)
	}

	resp, err := http.Post(prov.OAuthTokenURL, "application/x-www-form-urlencoded", strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var tokenResp tokenExchangeResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, fmt.Errorf("failed to parse token response: %w — body: %s", err, string(body))
	}

	if tokenResp.Error != "" {
		return nil, fmt.Errorf("%s: %s", tokenResp.Error, tokenResp.ErrorDesc)
	}
	if tokenResp.AccessToken == "" {
		return nil, fmt.Errorf("no access_token in response — body: %s", string(body))
	}

	return &tokenResp, nil
}

// refreshOAuthToken refreshes an OAuth2 token using the refresh_token grant.
func refreshOAuthToken(prov config.ProviderEntry, refreshToken string) (*tokenExchangeResponse, error) {
	data := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
		"client_id":     {prov.OAuthClientID},
	}
	if prov.OAuthClientSecret != "" {
		data.Set("client_secret", prov.OAuthClientSecret)
	}

	resp, err := http.Post(prov.OAuthTokenURL, "application/x-www-form-urlencoded", strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var tokenResp tokenExchangeResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, fmt.Errorf("failed to parse token response: %w — body: %s", err, string(body))
	}

	if tokenResp.Error != "" {
		return nil, fmt.Errorf("%s: %s", tokenResp.Error, tokenResp.ErrorDesc)
	}
	if tokenResp.AccessToken == "" {
		return nil, fmt.Errorf("no access_token in refresh response")
	}

	return &tokenResp, nil
}

// isPrivateIP returns true if the given host is a private/LAN IP address.
// Google OAuth rejects private IPs as redirect URIs; localhost must be used instead.
func isPrivateIP(host string) bool {
	// Strip port if present
	h := host
	if idx := strings.LastIndex(host, ":"); idx != -1 {
		h = host[:idx]
	}
	ip := net.ParseIP(h)
	if ip == nil {
		return false
	}
	privateRanges := []string{
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
		"169.254.0.0/16",
	}
	for _, cidr := range privateRanges {
		_, network, err := net.ParseCIDR(cidr)
		if err == nil && network.Contains(ip) {
			return true
		}
	}
	return false
}

// buildRedirectURI constructs the OAuth callback URL.
// oauthBase, if non-empty, overrides auto-detection (e.g. "http://localhost:8088").
func buildRedirectURI(r *http.Request, serverHost string, serverPort int, oauthBase string) string {
	// Explicit override via config beats everything
	if oauthBase != "" {
		return strings.TrimRight(oauthBase, "/") + "/api/oauth/callback"
	}

	scheme := "http"
	if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	host := r.Host
	if host == "" {
		if serverHost == "0.0.0.0" || serverHost == "" {
			host = fmt.Sprintf("localhost:%d", serverPort)
		} else {
			host = fmt.Sprintf("%s:%d", serverHost, serverPort)
		}
	}
	// Google (and other providers) reject private/LAN IP addresses as redirect URIs.
	// Replace with localhost so the registered URI matches what Google allows.
	if isPrivateIP(host) {
		_, port, err := net.SplitHostPort(host)
		if err == nil && port != "" {
			host = "localhost:" + port
		} else {
			host = fmt.Sprintf("localhost:%d", serverPort)
		}
	}
	return fmt.Sprintf("%s://%s/api/oauth/callback", scheme, host)
}

// handleOAuthManual completes an OAuth flow using a redirect URL pasted by the user.
// This is the fallback for LAN setups where the browser's redirect fails (ERR_CONNECTION_REFUSED).
// POST /api/oauth/manual
// Body: {"url": "http://localhost:8088/api/oauth/callback?code=xxx&state=yyy"}
func handleOAuthManual(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var body struct {
			URL string `json:"url"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.URL == "" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{"success": false, "message": "Missing 'url' in request body"})
			return
		}

		parsed, err := url.Parse(body.URL)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{"success": false, "message": "Invalid URL"})
			return
		}

		code := parsed.Query().Get("code")
		state := parsed.Query().Get("state")
		errParam := parsed.Query().Get("error")

		if errParam != "" {
			errDesc := parsed.Query().Get("error_description")
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{"success": false, "message": fmt.Sprintf("Authorization denied: %s — %s", errParam, errDesc)})
			return
		}
		if code == "" || state == "" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{"success": false, "message": "URL does not contain 'code' or 'state' parameters"})
			return
		}

		if s.Vault == nil {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{"success": false, "message": "Vault not available"})
			return
		}

		// Validate and consume state (same one-time-use check as the normal callback)
		stateRaw, err := s.Vault.ReadSecret("oauth_state_" + state)
		if err != nil || stateRaw == "" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{"success": false, "message": "Invalid or expired state — please click Connect again to start a new OAuth flow"})
			return
		}
		_ = s.Vault.DeleteSecret("oauth_state_" + state)

		var stateData map[string]string
		if err := json.Unmarshal([]byte(stateRaw), &stateData); err != nil {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{"success": false, "message": "Corrupt state data"})
			return
		}
		providerID := stateData["provider_id"]

		s.CfgMu.RLock()
		prov := s.Cfg.FindProvider(providerID)
		var entry config.ProviderEntry
		if prov != nil {
			entry = *prov
		}
		serverPort := s.Cfg.Server.Port
		serverHost := s.Cfg.Server.Host
		oauthBase := s.Cfg.Server.OAuthRedirectBaseURL
		s.CfgMu.RUnlock()

		if prov == nil {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{"success": false, "message": "Provider '" + providerID + "' not found in config"})
			return
		}

		// Reconstruct the same redirect_uri used in the original authorization request
		redirectURI := buildRedirectURI(r, serverHost, serverPort, oauthBase)

		tokenResp, err := exchangeCodeForToken(entry, code, redirectURI)
		if err != nil {
			s.Logger.Error("[OAuth] Manual token exchange failed", "provider", providerID, "error", err)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{"success": false, "message": "Token exchange failed"})
			return
		}

		tok := config.OAuthToken{
			AccessToken:  tokenResp.AccessToken,
			RefreshToken: tokenResp.RefreshToken,
			TokenType:    tokenResp.TokenType,
		}
		if tokenResp.ExpiresIn > 0 {
			tok.Expiry = time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second).UTC().Format(time.RFC3339)
		}
		tokJSON, _ := json.Marshal(tok)
		if err := s.Vault.WriteSecret("oauth_"+providerID, string(tokJSON)); err != nil {
			s.Logger.Error("[OAuth] Failed to store token", "provider", providerID, "error", err)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{"success": false, "message": "Failed to store token"})
			return
		}

		s.CfgMu.Lock()
		s.Cfg.ApplyOAuthTokens(s.Vault)
		s.CfgMu.Unlock()

		s.Logger.Info("[OAuth] Manual authorization successful", "provider", providerID)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"success": true, "message": "Authorization successful for provider: " + providerID})
	}
}

// renderOAuthResult shows a simple HTML page for the OAuth callback result.
func renderOAuthResult(w http.ResponseWriter, success bool, message string) {
	icon := "❌"
	title := "Authorization Failed"
	color := "#e74c3c"
	if success {
		icon = "✅"
		title = "Authorization Successful"
		color = "#27ae60"
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<head><title>%s</title>
<style>
body{font-family:system-ui,-apple-system,sans-serif;display:flex;align-items:center;justify-content:center;min-height:100vh;margin:0;background:#1a1a2e;color:#e0e0e0;}
.card{text-align:center;padding:3rem;border-radius:16px;background:#16213e;border:1px solid #0f3460;max-width:400px;}
h2{color:%s;margin-bottom:0.5rem;}
p{color:#a0a0a0;margin-bottom:1.5rem;}
button{padding:0.6rem 1.5rem;border:none;border-radius:8px;background:%s;color:#fff;font-size:0.9rem;cursor:pointer;}
button:hover{opacity:0.85;}
</style></head>
<body>
<div class="card">
<div style="font-size:3rem;">%s</div>
<h2>%s</h2>
<p>%s</p>
<button onclick="window.close()">Close Window</button>
</div>
</body></html>`, title, color, color, icon, title, message)
}
