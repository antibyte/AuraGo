package server

import (
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"aurago/internal/config"
	"aurago/internal/security"
)

// ── OAuth2 Authorization Code Flow ──────────────────────────────────────────

// handleOAuthStart initiates the OAuth2 Authorization Code flow for a provider.
// GET /api/oauth/start?provider=<id>
// Returns JSON: {"auth_url": "https://...", "mode": "browser_callback", ...}
func handleOAuthStart(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		launch := r.URL.Query().Get("launch") == "1" || r.URL.Query().Get("redirect") == "1"
		fail := func(message string, status int) {
			if launch {
				renderOAuthResult(w, false, message)
				return
			}
			jsonError(w, message, status)
		}

		providerID := r.URL.Query().Get("provider")
		if providerID == "" {
			fail("Missing 'provider' query parameter", http.StatusBadRequest)
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
		oauthBase := s.Cfg.Server.OAuthRedirectBaseURL
		s.CfgMu.RUnlock()

		if prov == nil {
			fail("Provider not found: "+providerID, http.StatusNotFound)
			return
		}
		if entry.AuthType != "oauth2" {
			fail("Provider is not configured for OAuth2", http.StatusBadRequest)
			return
		}
		if entry.OAuthAuthURL == "" || entry.OAuthTokenURL == "" || entry.OAuthClientID == "" {
			fail("OAuth2 configuration incomplete (need auth_url, token_url, client_id)", http.StatusBadRequest)
			return
		}

		if s.Vault == nil {
			fail("Vault not available — cannot store OAuth state", http.StatusServiceUnavailable)
			return
		}

		redirectURI := buildRedirectURI(r, serverHost, serverPort, oauthBase)
		session, err := newOAuthSession(providerID, oauthFlowModeBrowserCallback, redirectURI, time.Now())
		if err != nil {
			s.Logger.Error("[OAuth] Failed to create session", "provider", providerID, "error", err)
			fail("Internal error", http.StatusInternalServerError)
			return
		}
		if err := storeOAuthSession(s.Vault, session); err != nil {
			s.Logger.Error("[OAuth] Failed to store session", "provider", providerID, "error", err)
			fail("Failed to store OAuth state", http.StatusInternalServerError)
			return
		}

		authURL, err := buildOAuthAuthorizationURL(entry, session)
		if err != nil {
			fail("Invalid oauth_auth_url", http.StatusBadRequest)
			return
		}

		if launch {
			http.Redirect(w, r, authURL, http.StatusFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"auth_url":       authURL,
			"mode":           session.Mode,
			"session_id":     session.State,
			"expires_at":     session.ExpiresAt.Format(time.RFC3339),
			"fallback_modes": session.FallbackModes,
			"message":        "Browser authorization started. If the callback cannot reach AuraGo, paste the final redirect URL.",
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

		if state == "" {
			renderOAuthResult(w, false, "Missing state parameter")
			return
		}

		if s.Vault == nil {
			renderOAuthResult(w, false, "Vault not available")
			return
		}

		session, err := consumeOAuthSession(s.Vault, state, time.Now())
		if err != nil {
			renderOAuthResult(w, false, "Invalid or expired state parameter")
			return
		}

		if errParam != "" {
			errDesc := r.URL.Query().Get("error_description")
			renderOAuthResult(w, false, fmt.Sprintf("Authorization denied: %s - %s", errParam, errDesc))
			return
		}

		if code == "" {
			renderOAuthResult(w, false, "Missing code parameter")
			return
		}

		providerID := session.ProviderID

		// Look up provider config
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
			renderOAuthResult(w, false, "Provider '"+providerID+"' not found in config")
			return
		}

		redirectURI := session.RedirectURI
		if redirectURI == "" {
			redirectURI = buildRedirectURI(r, serverHost, serverPort, oauthBase)
		}

		tokenResp, err := exchangeCodeForToken(entry, code, redirectURI, session.CodeVerifier)
		if err != nil {
			s.Logger.Error("[OAuth] Token exchange failed", "provider", providerID, "error", err)
			renderOAuthResult(w, false, "Token exchange failed")
			return
		}

		if err := storeOAuthToken(s.Vault, providerID, tokenResp, time.Now()); err != nil {
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
func exchangeCodeForToken(prov config.ProviderEntry, code, redirectURI, codeVerifier string) (*tokenExchangeResponse, error) {
	data := url.Values{
		"grant_type":   {"authorization_code"},
		"code":         {code},
		"redirect_uri": {redirectURI},
		"client_id":    {prov.OAuthClientID},
	}
	if prov.OAuthClientSecret != "" {
		data.Set("client_secret", prov.OAuthClientSecret)
	}
	if codeVerifier != "" {
		data.Set("code_verifier", codeVerifier)
	}

	client, err := security.NewSSRFProtectedHTTPClientForURL(prov.OAuthTokenURL, 15*time.Second)
	if err != nil {
		return nil, fmt.Errorf("OAuth token URL rejected: %w", err)
	}
	req, err := http.NewRequest(http.MethodPost, prov.OAuthTokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := client.Do(req)
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

	client, err := security.NewSSRFProtectedHTTPClientForURL(prov.OAuthTokenURL, 15*time.Second)
	if err != nil {
		return nil, fmt.Errorf("OAuth token URL rejected: %w", err)
	}
	req, err := http.NewRequest(http.MethodPost, prov.OAuthTokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create refresh request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := client.Do(req)
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

		session, err := consumeOAuthSession(s.Vault, state, time.Now())
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{"success": false, "message": "Invalid or expired state — please click Connect again to start a new OAuth flow"})
			return
		}
		providerID := session.ProviderID

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

		redirectURI := session.RedirectURI
		if redirectURI == "" {
			redirectURI = buildRedirectURI(r, serverHost, serverPort, oauthBase)
		}

		tokenResp, err := exchangeCodeForToken(entry, code, redirectURI, session.CodeVerifier)
		if err != nil {
			s.Logger.Error("[OAuth] Manual token exchange failed", "provider", providerID, "error", err)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{"success": false, "message": "Token exchange failed"})
			return
		}

		if err := storeOAuthToken(s.Vault, providerID, tokenResp, time.Now()); err != nil {
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
	safeTitle := html.EscapeString(title)
	safeMessage := html.EscapeString(message)
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
</style>
<script>
try {
  if (window.opener && !window.opener.closed) {
    window.opener.postMessage({type:"aurago:oauth-provider-connected", success:%t}, window.location.origin);
  }
} catch (e) {}
</script></head>
<body>
<div class="card">
<div style="font-size:3rem;">%s</div>
<h2>%s</h2>
<p>%s</p>
<button onclick="window.close()">Close Window</button>
</div>
</body></html>`, safeTitle, color, color, success, icon, safeTitle, safeMessage)
}
