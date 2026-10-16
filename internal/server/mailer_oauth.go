package server

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"sync"
	"time"

	"aurago/internal/config"
	"aurago/internal/gog"
	"aurago/internal/i18n"
	"aurago/internal/security"
)

const (
	mailerOAuthCallbackPath = "/api/desktop/mailer/oauth/callback"
	mailerOAuthCookieName   = "mailer_oauth_nonce"
	mailerOAuthStatePrefix  = "oauth_mailer_state_"
	mailerMicrosoftAuthURL  = "https://login.microsoftonline.com/common/oauth2/v2.0/authorize"
	mailerMicrosoftTokenURL = "https://login.microsoftonline.com/common/oauth2/v2.0/token"
	mailerGoogleTokenURL    = "https://oauth2.googleapis.com/token"
)

var mailerOAuthRefreshMu sync.Mutex
var newMailerOAuthHTTPClient = security.NewSSRFProtectedHTTPClientSameOrigin

type mailerOAuthPending struct {
	Version             int               `json:"version"`
	State               string            `json:"state"`
	NonceHash           string            `json:"nonce_hash"`
	AccountID           string            `json:"account_id"`
	Provider            string            `json:"provider"`
	RedirectURI         string            `json:"redirect_uri"`
	CreatedAt           time.Time         `json:"created_at"`
	ExpiresAt           time.Time         `json:"expires_at"`
	Verifier            string            `json:"verifier"`
	Scopes              []string          `json:"scopes,omitempty"`
	Previous            string            `json:"previous,omitempty"`
	GogAccount          config.GogAccount `json:"gog_account,omitempty"`
	EmailProtocol       string            `json:"email_protocol,omitempty"`
	EmailOAuthAccountID string            `json:"email_oauth_account_id,omitempty"`
	IdentityKind        string            `json:"identity_kind"`
	Identity            string            `json:"identity,omitempty"`
}

func mailerOAuthHash(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func mailerOAuthNonce() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func mailerOAuthRedirect(r *http.Request, cfg *config.Config) string {
	if cfg == nil {
		return ""
	}
	var parsed *url.URL
	if configured := strings.TrimSpace(cfg.Server.OAuthRedirectBaseURL); configured != "" {
		base, err := url.Parse(configured)
		if err != nil || (base.Scheme != "http" && base.Scheme != "https") || base.Host == "" || base.User != nil || base.RawQuery != "" || base.Fragment != "" {
			return ""
		}
		parsed = base
	} else {
		// Never derive an OAuth redirect from Request.Host or forwarded headers.
		// Those values are attacker-controlled unless a trusted proxy is configured.
		host := strings.TrimSpace(cfg.Server.Host)
		if host == "" || host == "0.0.0.0" || host == "::" || host == "[::]" {
			host = "localhost"
		}
		if ip := net.ParseIP(strings.Trim(host, "[]")); ip != nil && ip.IsPrivate() {
			host = "localhost"
		}
		if strings.Contains(host, ":") && !strings.HasPrefix(host, "[") {
			host = "[" + host + "]"
		}
		port := cfg.Server.Port
		if port <= 0 {
			return ""
		}
		scheme := "http"
		if cfg.Server.HTTPS.Enabled {
			scheme = "https"
		}
		parsed = &url.URL{Scheme: scheme, Host: net.JoinHostPort(strings.Trim(host, "[]"), fmt.Sprint(port))}
	}
	parsed.Path = mailerOAuthCallbackPath
	parsed.RawPath = ""
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String()
}

func mailerOAuthSetCookie(w http.ResponseWriter, r *http.Request, cfg *config.Config, nonce string) {
	secure := r.TLS != nil || strings.EqualFold(strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")), "https")
	if cfg != nil && cfg.Server.HTTPS.Enabled {
		secure = true
	}
	http.SetCookie(w, &http.Cookie{Name: mailerOAuthCookieName, Value: nonce, Path: mailerOAuthCallbackPath,
		HttpOnly: true, Secure: secure, SameSite: http.SameSiteLaxMode, MaxAge: int(oauthSessionTTL.Seconds())})
}

func mailerOAuthClearCookie(w http.ResponseWriter, r *http.Request, cfg *config.Config) {
	secure := r.TLS != nil || strings.EqualFold(strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")), "https")
	if cfg != nil && cfg.Server.HTTPS.Enabled {
		secure = true
	}
	http.SetCookie(w, &http.Cookie{Name: mailerOAuthCookieName, Value: "", Path: mailerOAuthCallbackPath,
		HttpOnly: true, Secure: secure, SameSite: http.SameSiteLaxMode, MaxAge: -1, Expires: time.Unix(1, 0)})
}

func (s *Server) mailerOAuthIdentity(r *http.Request, cfg *config.Config) (kind, identity string, ok bool) {
	if raw, present := bearerCredential(r.Header.Get("Authorization")); present {
		if raw == "" {
			return "", "", false
		}
		if !desktopTokenHasScope(s, raw, desktopScopeAdmin) {
			return "", "", false
		}
		security.RegisterSensitive(raw)
		return "bearer", raw, true
	}
	if cfg == nil || !cfg.Auth.Enabled {
		return "open", "", true
	}
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil || !validateSessionValue(cfg.Auth.SessionSecret, cookie.Value) {
		return "", "", false
	}
	return "session", cookie.Value, true
}

func (s *Server) mailerOAuthIdentityStillValid(p mailerOAuthPending) bool {
	cfg := s.ConfigSnapshot()
	if cfg == nil {
		return false
	}
	switch p.IdentityKind {
	case "open":
		return !cfg.Auth.Enabled
	case "session":
		return cfg.Auth.Enabled && validateSessionValue(cfg.Auth.SessionSecret, p.Identity)
	case "bearer":
		return desktopTokenHasScope(s, p.Identity, desktopScopeAdmin)
	default:
		return false
	}
}

func (s *Server) beginMailerOAuth(w http.ResponseWriter, r *http.Request, accountID, provider string, scopes []string, gogAccount *config.GogAccount, previous string) (map[string]string, error) {
	cfg := s.ConfigSnapshot()
	if cfg == nil || s.Vault == nil {
		return nil, fmt.Errorf("OAuth is unavailable")
	}
	identityKind, identity, ok := s.mailerOAuthIdentity(r, cfg)
	if !ok {
		return nil, fmt.Errorf("administrator authentication is no longer valid")
	}
	redirect := mailerOAuthRedirect(r, cfg)
	if redirect == "" {
		return nil, fmt.Errorf("callback URL is invalid")
	}
	session, err := newOAuthSession("mailer/"+provider, oauthFlowModeBrowserCallback, redirect, time.Now())
	if err != nil {
		return nil, err
	}
	nonce, err := mailerOAuthNonce()
	if err != nil {
		return nil, err
	}
	p := mailerOAuthPending{Version: 1, State: session.State, NonceHash: mailerOAuthHash(nonce), AccountID: accountID,
		Provider: provider, RedirectURI: redirect, CreatedAt: session.CreatedAt, ExpiresAt: session.ExpiresAt,
		Verifier: session.CodeVerifier, Scopes: append([]string(nil), scopes...), Previous: previous,
		IdentityKind: identityKind, Identity: identity}
	if gogAccount != nil {
		p.GogAccount = *gogAccount
	}
	if provider == "gog_protocol" {
		id := strings.TrimPrefix(accountID, "email:")
		for _, account := range cfg.EmailAccounts {
			if account.ID == id {
				p.EmailProtocol = account.ReceiveProtocol
				p.EmailOAuthAccountID = account.OAuthAccountID
				break
			}
		}
		if p.EmailProtocol == "" || p.EmailOAuthAccountID == "" {
			return nil, fmt.Errorf("Google protocol mailbox is unavailable")
		}
	}
	if identity != "" {
		security.RegisterSensitive(identity)
	}
	var authURL string
	switch provider {
	case "microsoft":
		entry := config.EmailAccount{}
		for _, a := range cfg.EmailAccounts {
			if "email:"+a.ID == accountID {
				entry = a
				break
			}
		}
		if entry.ID == "" || entry.OAuthClientID == "" || entry.OAuthProvider != "microsoft" || entry.OAuthSource != "mailer" {
			return nil, fmt.Errorf("Microsoft Mailer account is not configured")
		}
		values := url.Values{"client_id": {entry.OAuthClientID}, "response_type": {"code"}, "response_mode": {"query"},
			"redirect_uri": {redirect}, "scope": {strings.Join(scopes, " ")}, "state": {session.State},
			"code_challenge": {session.CodeChallenge}, "code_challenge_method": {session.CodeChallengeMethod}, "prompt": {"select_account"}}
		authURL = mailerMicrosoftAuthURL + "?" + values.Encode()
	case "gog", "gog_protocol":
		if gogAccount == nil {
			return nil, fmt.Errorf("Google account is unavailable")
		}
		prov := config.ProviderEntry{OAuthAuthURL: gog.AuthURL, OAuthClientID: gogAccount.ClientID, OAuthScopes: strings.Join(scopes, " ")}
		session.State = p.State
		session.RedirectURI = redirect
		session.CodeVerifier = p.Verifier
		session.CodeChallenge = func() string {
			sum := sha256.Sum256([]byte(p.Verifier))
			return base64.RawURLEncoding.EncodeToString(sum[:])
		}()
		session.CodeChallengeMethod = "S256"
		authURL, err = buildOAuthAuthorizationURL(prov, session)
		if err != nil {
			return nil, err
		}
	case "google_workspace":
		prov := cfg.FindProvider("google_workspace")
		if prov == nil || prov.OAuthClientID == "" || len(scopes) == 0 {
			return nil, fmt.Errorf("Google Workspace OAuth is not configured")
		}
		session.State = p.State
		session.RedirectURI = redirect
		session.CodeVerifier = p.Verifier
		session.CodeChallenge = func() string {
			sum := sha256.Sum256([]byte(p.Verifier))
			return base64.RawURLEncoding.EncodeToString(sum[:])
		}()
		session.CodeChallengeMethod = "S256"
		authURL, err = buildOAuthAuthorizationURL(*prov, session)
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unsupported OAuth provider")
	}
	if err := s.pinMailerOAuthPrevious(&p); err != nil {
		return nil, err
	}
	data, err := json.Marshal(p)
	if err != nil {
		return nil, err
	}
	if err := s.Vault.WriteSecret(mailerOAuthStatePrefix+p.State, string(data)); err != nil {
		return nil, err
	}
	mailerOAuthSetCookie(w, r, cfg, nonce)
	for _, c := range w.Header().Values("Set-Cookie") {
		if strings.HasPrefix(c, sessionCookieName+"=") {
			return nil, fmt.Errorf("refusing to modify the authentication session cookie")
		}
	}
	return map[string]string{"status": "ok", "auth_url": authURL, "redirect_uri": redirect}, nil
}

// pinMailerOAuthPrevious creates an agent-hidden first-login marker before
// the browser leaves AuraGo. Final token publication always CASes against this
// exact marker or the pre-existing token, so disconnect and concurrent logins
// cannot be undone by a late OAuth callback.
func (s *Server) pinMailerOAuthPrevious(p *mailerOAuthPending) error {
	if p == nil || s.Vault == nil {
		return fmt.Errorf("OAuth token store is unavailable")
	}
	key, err := mailerOAuthTokenKey(*p)
	if err != nil {
		return err
	}
	if p.Previous != "" {
		return nil
	}
	current, err := s.Vault.ReadSecret(key)
	if err == nil {
		if current == "" {
			return fmt.Errorf("stored OAuth token is empty")
		}
		p.Previous = current
		return nil
	}
	if !errors.Is(err, security.ErrSecretNotFound) {
		return fmt.Errorf("read prior OAuth token: %w", err)
	}
	markerData, err := json.Marshal(struct {
		PendingLogin string `json:"pending_login"`
	}{PendingLogin: p.State})
	if err != nil {
		return err
	}
	marker := string(markerData)
	if strings.HasPrefix(key, "oauth_gog_token:") {
		markerData, err = json.Marshal(gog.Token{PendingLogin: p.State, ClientID: p.GogAccount.ClientID})
		if err != nil {
			return err
		}
		marker = string(markerData)
	}
	if err := s.Vault.WriteSecretIfAbsent(key, marker); err == nil {
		p.Previous = marker
		return nil
	} else if !errors.Is(err, security.ErrSecretExists) {
		return fmt.Errorf("reserve first-login OAuth token: %w", err)
	}
	// A sibling start may have reserved the token or completed during the read.
	// Pin to the current exact value; only a single callback can then publish.
	current, err = s.Vault.ReadSecret(key)
	if err != nil || current == "" {
		return fmt.Errorf("OAuth token changed during start")
	}
	p.Previous = current
	return nil
}

func mailerOAuthTokenKey(p mailerOAuthPending) (string, error) {
	switch p.Provider {
	case "microsoft":
		if !strings.HasPrefix(p.AccountID, "email:") {
			return "", fmt.Errorf("invalid Microsoft OAuth account")
		}
		id := strings.TrimPrefix(p.AccountID, "email:")
		if id == "" {
			return "", fmt.Errorf("invalid Microsoft OAuth account")
		}
		return "email_" + id + "_oauth_token", nil
	case "google_workspace":
		if p.AccountID != "google_workspace:default" {
			return "", fmt.Errorf("invalid Google Workspace OAuth account")
		}
		return "oauth_google_workspace", nil
	case "gog", "gog_protocol":
		if !config.ValidGogAccountID(p.GogAccount.ID) || p.AccountID != "gog:"+p.GogAccount.ID && !(p.Provider == "gog_protocol" && strings.HasPrefix(p.AccountID, "email:")) {
			return "", fmt.Errorf("invalid GoG OAuth account")
		}
		return gog.TokenKey(p.GogAccount.ID), nil
	default:
		return "", fmt.Errorf("unsupported OAuth provider")
	}
}

func (s *Server) publishMailerOAuthToken(key, previous, next string) error {
	if previous == "" {
		return fmt.Errorf("OAuth first-login marker is missing")
	}
	if err := s.Vault.CompareAndSwapSecret(key, previous, next); err != nil {
		return fmt.Errorf("OAuth token changed before publication: %w", err)
	}
	return nil
}

func (s *Server) handleMailerOAuthStart(w http.ResponseWriter, r *http.Request) {
	if !s.mailerSetupAllowed(w, r) {
		return
	}
	var body struct {
		AccountID string `json:"account_id"`
	}
	if err := decodeMailerJSON(w, r, &body, 4096); err != nil || strings.TrimSpace(body.AccountID) == "" {
		mailerJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_request"})
		return
	}
	cfg := s.ConfigSnapshot()
	if cfg == nil || !cfg.Agent.AllowNetworkRequests {
		mailerJSON(w, http.StatusForbidden, map[string]string{"error": "network_disabled"})
		return
	}
	if s.Vault == nil {
		mailerJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "vault_unavailable"})
		return
	}
	var result map[string]string
	var err error
	switch {
	case strings.HasPrefix(body.AccountID, "email:"):
		id := strings.TrimPrefix(body.AccountID, "email:")
		var account *config.EmailAccount
		for i := range cfg.EmailAccounts {
			if cfg.EmailAccounts[i].ID == id {
				account = &cfg.EmailAccounts[i]
				break
			}
		}
		if account == nil || account.Disabled {
			mailerJSON(w, http.StatusNotFound, map[string]string{"error": "account_not_found"})
			return
		}
		switch {
		case account.OAuthSource == "mailer" && account.OAuthProvider == "microsoft":
			scopes := microsoftMailerScopes(*account)
			previous, _ := s.Vault.ReadSecret("email_" + id + "_oauth_token")
			result, err = s.beginMailerOAuth(w, r, body.AccountID, "microsoft", scopes, nil, previous)
		case account.OAuthSource == "gog" && account.OAuthProvider == "google":
			parent, exists := cfg.Gog.Account(account.OAuthAccountID)
			grants := parent.Services["gmail"]
			if !exists || !cfg.Gog.Enabled || !parent.Enabled || !parent.DesktopAccess || !grants.Read || !grants.Send || cfg.Gog.ReadOnly || !cfg.Agent.AllowNetworkRequests {
				mailerJSON(w, http.StatusForbidden, map[string]string{"error": "google_mail_permissions_required"})
				return
			}
			secret, secretErr := s.Vault.ReadSecret(gog.ClientKey(parent.ID))
			if secretErr != nil || secret == "" {
				mailerJSON(w, http.StatusBadRequest, map[string]string{"error": "client_secret_required"})
				return
			}
			security.RegisterSensitive(secret)
			scopes := gog.Scopes(parent, cfg.Gog.ReadOnly)
			scopes = appendUniqueMailerScope(scopes, "https://mail.google.com/")
			previous, readErr := s.Vault.ReadSecret(gog.TokenKey(parent.ID))
			if readErr != nil && readErr != security.ErrSecretNotFound {
				mailerJSON(w, http.StatusInternalServerError, map[string]string{"error": "oauth_start_failed"})
				return
			}
			result, err = s.beginMailerOAuth(w, r, body.AccountID, "gog_protocol", scopes, &parent, previous)
		default:
			mailerJSON(w, http.StatusNotFound, map[string]string{"error": "account_not_found"})
			return
		}
	case strings.HasPrefix(body.AccountID, "gog:"):
		id := strings.TrimPrefix(body.AccountID, "gog:")
		a, ok := cfg.Gog.Account(id)
		if !ok || !cfg.Gog.Enabled || !a.Enabled || !a.DesktopAccess {
			mailerJSON(w, http.StatusNotFound, map[string]string{"error": "account_not_found"})
			return
		}
		secret, _ := s.Vault.ReadSecret(gog.ClientKey(a.ID))
		if secret == "" {
			mailerJSON(w, http.StatusBadRequest, map[string]string{"error": "client_secret_required"})
			return
		}
		scopes := mailerGoogleAPIScopes(a, cfg.Gog.ReadOnly)
		if len(scopes) == 0 {
			mailerJSON(w, http.StatusBadRequest, map[string]string{"error": "gmail_scope_required"})
			return
		}
		previous, e := s.Vault.ReadSecret(gog.TokenKey(a.ID))
		if e != nil && e != security.ErrSecretNotFound {
			mailerJSON(w, http.StatusInternalServerError, map[string]string{"error": "oauth_start_failed"})
			return
		}
		result, err = s.beginMailerOAuth(w, r, body.AccountID, "gog", scopes, &a, previous)
	case body.AccountID == "google_workspace:default":
		gw := cfg.GoogleWorkspace
		prov := cfg.FindProvider("google_workspace")
		if !gw.Enabled || !gw.Gmail || gw.ClientID == "" || prov == nil {
			mailerJSON(w, http.StatusForbidden, map[string]string{"error": "google_workspace_unavailable"})
			return
		}
		scopes := strings.Fields(prov.OAuthScopes)
		previous, _ := s.Vault.ReadSecret("oauth_google_workspace")
		result, err = s.beginMailerOAuth(w, r, body.AccountID, "google_workspace", scopes, nil, previous)
	default:
		mailerJSON(w, http.StatusNotFound, map[string]string{"error": "account_not_found"})
		return
	}
	if err != nil {
		mailerJSON(w, http.StatusBadRequest, map[string]string{"error": "oauth_start_failed"})
		return
	}
	mailerJSON(w, http.StatusOK, result)
}

func microsoftMailerScopes(a config.EmailAccount) []string {
	protocol := strings.ToLower(strings.TrimSpace(a.ReceiveProtocol))
	scope := "https://outlook.office.com/IMAP.AccessAsUser.All"
	if protocol == "pop3" {
		scope = "https://outlook.office.com/POP.AccessAsUser.All"
	}
	return []string{"offline_access", scope, "https://outlook.office.com/SMTP.Send"}
}

func microsoftMailerRequiredScopes(a config.EmailAccount) []string {
	protocol := strings.ToLower(strings.TrimSpace(a.ReceiveProtocol))
	if protocol == "pop3" {
		return []string{"https://outlook.office.com/POP.AccessAsUser.All", "https://outlook.office.com/SMTP.Send"}
	}
	return []string{"https://outlook.office.com/IMAP.AccessAsUser.All", "https://outlook.office.com/SMTP.Send"}
}

func (s *Server) consumeMailerOAuthPending(r *http.Request, state string) (mailerOAuthPending, error) {
	if s.Vault == nil || len(state) != 64 {
		return mailerOAuthPending{}, fmt.Errorf("invalid OAuth state")
	}
	cookie, err := r.Cookie(mailerOAuthCookieName)
	if err != nil || cookie.Value == "" {
		return mailerOAuthPending{}, fmt.Errorf("OAuth browser binding is missing")
	}
	raw, err := s.Vault.ReadSecret(mailerOAuthStatePrefix + state)
	if err != nil || raw == "" {
		return mailerOAuthPending{}, fmt.Errorf("invalid OAuth state")
	}
	var p mailerOAuthPending
	if json.Unmarshal([]byte(raw), &p) != nil || p.Version != 1 || p.State != state || p.Verifier == "" || p.RedirectURI == "" || time.Now().After(p.ExpiresAt) || p.ExpiresAt.IsZero() {
		return mailerOAuthPending{}, fmt.Errorf("invalid or expired OAuth state")
	}
	if cfg := s.ConfigSnapshot(); cfg == nil || p.RedirectURI != mailerOAuthRedirect(r, cfg) {
		return mailerOAuthPending{}, fmt.Errorf("OAuth callback does not match the original redirect")
	}
	actualHash := mailerOAuthHash(cookie.Value)
	if subtle.ConstantTimeCompare([]byte(actualHash), []byte(p.NonceHash)) != 1 {
		return mailerOAuthPending{}, fmt.Errorf("OAuth browser binding mismatch")
	}
	if !s.mailerOAuthIdentityStillValid(p) {
		return mailerOAuthPending{}, fmt.Errorf("administrator authorization expired")
	}
	if err := s.Vault.CompareAndSwapSecret(mailerOAuthStatePrefix+state, raw, ""); err != nil {
		return mailerOAuthPending{}, fmt.Errorf("OAuth state already consumed")
	}
	security.RegisterSensitive(cookie.Value)
	return p, nil
}

func (s *Server) handleMailerOAuthCallback(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; script-src 'self' 'unsafe-inline'; style-src 'unsafe-inline'; base-uri 'none'; frame-ancestors 'none'")
	if r.Method != http.MethodGet {
		http.Error(w, "OAuth callback method not allowed", http.StatusMethodNotAllowed)
		return
	}
	cfg := s.ConfigSnapshot()
	state := r.URL.Query().Get("state")
	pending, err := s.consumeMailerOAuthPending(r, state)
	mailerOAuthClearCookie(w, r, cfg)
	if err != nil {
		writeMailerOAuthResult(w, http.StatusBadRequest, "error", cfg)
		return
	}
	if err := s.mailerOAuthCallbackAllowed(pending); err != nil {
		writeMailerOAuthResult(w, http.StatusForbidden, "error", cfg)
		return
	}
	if r.URL.Query().Get("error") != "" {
		writeMailerOAuthResult(w, http.StatusBadRequest, "error", cfg)
		return
	}
	code := r.URL.Query().Get("code")
	if code == "" || len(code) > 8192 {
		writeMailerOAuthResult(w, http.StatusBadRequest, "error", cfg)
		return
	}
	security.RegisterSensitive(code)
	ctx, cancel := context.WithTimeout(r.Context(), 25*time.Second)
	defer cancel()
	switch pending.Provider {
	case "microsoft":
		err = s.completeMailerMicrosoftOAuth(ctx, pending, code)
	case "google_workspace":
		err = s.completeMailerWorkspaceOAuth(ctx, pending, code)
	case "gog":
		err = s.completeMailerGogOAuth(ctx, pending, code)
	case "gog_protocol":
		err = s.completeMailerGogOAuth(ctx, pending, code)
	default:
		err = fmt.Errorf("unsupported OAuth provider")
	}
	if err != nil {
		security.RegisterSensitive(err.Error())
		writeMailerOAuthResult(w, http.StatusBadGateway, "error", cfg)
		return
	}
	if err := s.mailerOAuthCallbackAllowed(pending); err != nil {
		writeMailerOAuthResult(w, http.StatusForbidden, "error", cfg)
		return
	}
	if s.Mailer != nil {
		if err := s.Mailer.Connect(ctx, pending.AccountID); err != nil {
			security.RegisterSensitive(err.Error())
			writeMailerOAuthResult(w, http.StatusBadGateway, "error", cfg)
			return
		}
	}
	writeMailerOAuthResult(w, http.StatusOK, "ok", cfg)
}

func (s *Server) mailerOAuthCallbackAllowed(p mailerOAuthPending) error {
	cfg := s.ConfigSnapshot()
	if cfg == nil || !cfg.VirtualDesktop.Enabled || cfg.VirtualDesktop.ReadOnly || !cfg.Agent.AllowNetworkRequests || s.Vault == nil || s.Mailer == nil {
		return fmt.Errorf("Mailer setup is disabled")
	}
	if !s.mailerOAuthIdentityStillValid(p) {
		return fmt.Errorf("administrator authorization expired")
	}
	return nil
}

func writeMailerOAuthResult(w http.ResponseWriter, statusCode int, status string, cfg *config.Config) {
	lang := "en"
	if cfg != nil {
		lang = normalizeLang(cfg.Agent.SystemLanguage)
	}
	messageKey := "mailer.oauth_failed"
	if status == "ok" {
		messageKey = "mailer.account_connected"
	}
	message := i18n.T(lang, messageKey)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(statusCode)
	_, _ = io.WriteString(w, mailerOAuthResultHTML(status, message))
}

func mailerOAuthResultHTML(status, message string) string {
	if status != "ok" {
		status = "error"
	}
	// Both values are selected internally and never contain provider data.
	return `<!doctype html><html><head><meta charset="utf-8"><title>Mailer</title></head><body><p>` + html.EscapeString(message) + `</p><script>if(window.opener){window.opener.postMessage({type:"aurago-mailer-oauth",status:"` + status + `"},window.location.origin)}if("` + status + `"==="ok")window.close()</script></body></html>`
}

func (s *Server) completeMailerGogOAuth(ctx context.Context, p mailerOAuthPending, code string) error {
	cfg := s.ConfigSnapshot()
	if err := s.mailerOAuthCallbackAllowed(p); err != nil {
		return err
	}
	if p.Previous == "" {
		return fmt.Errorf("Google first-login marker is missing")
	}
	a, ok := cfg.Gog.Account(p.GogAccount.ID)
	if !ok || !cfg.Gog.Enabled || !a.Enabled || !a.DesktopAccess || !reflect.DeepEqual(a, p.GogAccount) {
		return fmt.Errorf("Google account changed")
	}
	protocolAccount := strings.HasPrefix(p.AccountID, "email:") && p.Provider == "gog_protocol"
	if protocolAccount {
		id := strings.TrimPrefix(p.AccountID, "email:")
		var emailAccount *config.EmailAccount
		for i := range cfg.EmailAccounts {
			if cfg.EmailAccounts[i].ID == id {
				emailAccount = &cfg.EmailAccounts[i]
				break
			}
		}
		if emailAccount == nil || emailAccount.Disabled || emailAccount.OAuthSource != "gog" || emailAccount.OAuthProvider != "google" ||
			emailAccount.OAuthAccountID != a.ID || emailAccount.ReceiveProtocol != p.EmailProtocol || p.EmailOAuthAccountID != a.ID ||
			(emailAccount.ReceiveProtocol != "imap" && emailAccount.ReceiveProtocol != "pop3") {
			return fmt.Errorf("Google protocol account changed")
		}
		grants := a.Services["gmail"]
		if !grants.Read || !grants.Send || cfg.Gog.ReadOnly || !cfg.Agent.AllowNetworkRequests {
			return fmt.Errorf("Google protocol permissions changed")
		}
		if !scopeSetContains(p.Scopes, []string{"https://mail.google.com/"}) {
			return fmt.Errorf("Google protocol scope is missing")
		}
	} else if strings.TrimPrefix(p.AccountID, "gog:") != a.ID || p.Provider != "gog" {
		return fmt.Errorf("Google account does not match OAuth state")
	}
	if a.ClientID == "" {
		return fmt.Errorf("Google account does not match OAuth state")
	}
	secret, err := s.Vault.ReadSecret(gog.ClientKey(a.ID))
	if err != nil || secret == "" {
		return fmt.Errorf("Google client secret is unavailable")
	}
	security.RegisterSensitive(secret)
	values := url.Values{"grant_type": {"authorization_code"}, "code": {code}, "redirect_uri": {p.RedirectURI}, "code_verifier": {p.Verifier}, "client_id": {a.ClientID}, "client_secret": {secret}}
	response, err := mailerOAuthTokenRequest(ctx, gog.TokenURL, values)
	if err != nil {
		return err
	}
	actualScopes := strings.Fields(response.Scope)
	if response.Scope == "" || response.ExpiresIn <= 0 || response.ExpiresIn > 86400 || !scopeSetContains(actualScopes, p.Scopes) {
		return fmt.Errorf("Google did not grant the requested mail scopes")
	}
	var previous gog.Token
	if json.Unmarshal([]byte(p.Previous), &previous) != nil {
		return fmt.Errorf("stored Google token is invalid")
	}
	if response.RefreshToken == "" {
		response.RefreshToken = previous.RefreshToken
	}
	if response.RefreshToken == "" {
		return fmt.Errorf("Google did not issue a refresh token")
	}
	rotated, err := json.Marshal(gog.Token{AccessToken: response.AccessToken, RefreshToken: response.RefreshToken,
		Expiry: time.Now().Add(time.Duration(response.ExpiresIn) * time.Second).UTC(), Scopes: actualScopes, ClientID: a.ClientID})
	if err != nil {
		return err
	}
	if err := s.mailerOAuthCallbackAllowed(p); err != nil {
		return err
	}
	key := gog.TokenKey(a.ID)
	return s.publishMailerOAuthToken(key, p.Previous, string(rotated))
}

func appendUniqueMailerScope(scopes []string, scope string) []string {
	for _, existing := range scopes {
		if strings.TrimSpace(existing) == scope {
			return scopes
		}
	}
	return append(scopes, scope)
}

func mailerGoogleAPIScopes(a config.GogAccount, readOnly bool) []string {
	scopes := gog.Scopes(a, readOnly)
	// Unlike GoG's ordinary delete grant (draft/label/trash operations), Mailer
	// permanent message deletion requires Google's full mail scope. Request it
	// only on an explicit Mailer OAuth flow and only when the existing GoG
	// account already grants delete.
	if !readOnly && a.Services["gmail"].Delete {
		filtered := make([]string, 0, len(scopes))
		for _, scope := range scopes {
			if strings.HasPrefix(scope, "https://www.googleapis.com/auth/gmail.") || scope == "https://mail.google.com/" {
				continue
			}
			filtered = append(filtered, scope)
		}
		scopes = filtered
		scopes = appendUniqueMailerScope(scopes, "https://mail.google.com/")
	}
	return scopes
}

func (s *Server) completeMailerWorkspaceOAuth(ctx context.Context, p mailerOAuthPending, code string) error {
	cfg := s.ConfigSnapshot()
	if err := s.mailerOAuthCallbackAllowed(p); err != nil {
		return err
	}
	if p.AccountID != "google_workspace:default" || !cfg.GoogleWorkspace.Enabled || !cfg.GoogleWorkspace.Gmail || cfg.GoogleWorkspace.ClientID == "" {
		return fmt.Errorf("Google Workspace configuration changed")
	}
	prov := cfg.FindProvider("google_workspace")
	if prov == nil || !sameScopeSet(p.Scopes, strings.Fields(prov.OAuthScopes)) {
		return fmt.Errorf("Google Workspace permissions changed")
	}
	if p.Previous == "" {
		return fmt.Errorf("Google Workspace first-login marker is missing")
	}
	secret := s.mailerSecret("google_workspace_client_secret", cfg.GoogleWorkspace.ClientSecret)
	if secret == "" {
		return fmt.Errorf("Google Workspace client secret is unavailable")
	}
	security.RegisterSensitive(code)
	data := url.Values{"grant_type": {"authorization_code"}, "code": {code}, "redirect_uri": {p.RedirectURI},
		"client_id": {prov.OAuthClientID}, "client_secret": {secret}, "code_verifier": {p.Verifier}}
	response, err := mailerOAuthTokenRequest(ctx, mailerGoogleTokenURL, data)
	if err != nil {
		return err
	}
	if strings.TrimSpace(response.Scope) == "" {
		return fmt.Errorf("provider omitted granted scopes")
	}
	actualScopes := strings.Fields(response.Scope)
	requiredScopes := []string{"https://www.googleapis.com/auth/gmail.readonly|https://www.googleapis.com/auth/gmail.modify|https://mail.google.com/"}
	if cfg.GoogleWorkspace.GmailSend && !cfg.GoogleWorkspace.ReadOnly {
		requiredScopes = append(requiredScopes, "https://www.googleapis.com/auth/gmail.send|https://www.googleapis.com/auth/gmail.compose|https://www.googleapis.com/auth/gmail.modify|https://mail.google.com/")
	}
	if cfg.GoogleWorkspace.GmailModifyLabels && !cfg.GoogleWorkspace.ReadOnly {
		requiredScopes = append(requiredScopes, "https://www.googleapis.com/auth/gmail.modify|https://mail.google.com/")
	}
	for _, required := range requiredScopes {
		if !mailerHasScope(actualScopes, required) {
			return fmt.Errorf("provider did not grant requested mail scopes")
		}
	}
	var previous config.OAuthToken
	if json.Unmarshal([]byte(p.Previous), &previous) != nil {
		return fmt.Errorf("stored Google Workspace token is invalid")
	}
	if response.RefreshToken == "" {
		response.RefreshToken = previous.RefreshToken
	}
	if response.RefreshToken == "" {
		return fmt.Errorf("provider did not issue a refresh token")
	}
	tok := config.OAuthToken{AccessToken: response.AccessToken, RefreshToken: response.RefreshToken, TokenType: response.TokenType,
		Scopes: actualScopes}
	if response.ExpiresIn > 0 {
		tok.Expiry = time.Now().Add(time.Duration(response.ExpiresIn) * time.Second).UTC().Format(time.RFC3339)
	}
	if err := s.mailerOAuthCallbackAllowed(p); err != nil {
		return err
	}
	return s.publishMailerOAuthToken("oauth_google_workspace", p.Previous, marshalOAuthToken(tok))
}

func (s *Server) completeMailerMicrosoftOAuth(ctx context.Context, p mailerOAuthPending, code string) error {
	cfg := s.ConfigSnapshot()
	if err := s.mailerOAuthCallbackAllowed(p); err != nil {
		return err
	}
	id := strings.TrimPrefix(p.AccountID, "email:")
	var account *config.EmailAccount
	for i := range cfg.EmailAccounts {
		if cfg.EmailAccounts[i].ID == id {
			account = &cfg.EmailAccounts[i]
			break
		}
	}
	if account == nil || account.Disabled || account.OAuthSource != "mailer" || account.OAuthProvider != "microsoft" || account.OAuthClientID == "" || !sameScopeSet(p.Scopes, microsoftMailerScopes(*account)) {
		return fmt.Errorf("Microsoft Mailer account changed")
	}
	if p.Previous == "" {
		return fmt.Errorf("Microsoft first-login marker is missing")
	}
	clientSecret := s.mailerSecret("email_"+id+"_oauth_client_secret", account.OAuthClientSecret)
	if clientSecret == "" {
		return fmt.Errorf("Microsoft client secret unavailable")
	}
	security.RegisterSensitive(clientSecret)
	data := url.Values{"grant_type": {"authorization_code"}, "code": {code}, "redirect_uri": {p.RedirectURI},
		"client_id": {account.OAuthClientID}, "client_secret": {clientSecret}, "code_verifier": {p.Verifier},
		"scope": {strings.Join(p.Scopes, " ")}}
	response, err := mailerOAuthTokenRequest(ctx, mailerMicrosoftTokenURL, data)
	if err != nil {
		return err
	}
	if strings.TrimSpace(response.Scope) == "" {
		return fmt.Errorf("provider omitted granted scopes")
	}
	actualScopes := strings.Fields(response.Scope)
	if !scopeSetContains(actualScopes, microsoftMailerRequiredScopes(*account)) {
		return fmt.Errorf("provider did not grant requested scopes")
	}
	var previous config.OAuthToken
	if json.Unmarshal([]byte(p.Previous), &previous) != nil {
		return fmt.Errorf("stored Microsoft token is invalid")
	}
	if response.RefreshToken == "" {
		response.RefreshToken = previous.RefreshToken
	}
	if response.RefreshToken == "" {
		return fmt.Errorf("provider did not issue a refresh token")
	}
	tok := config.OAuthToken{AccessToken: response.AccessToken, RefreshToken: response.RefreshToken, TokenType: response.TokenType, Scopes: actualScopes}
	if response.ExpiresIn > 0 {
		tok.Expiry = time.Now().Add(time.Duration(response.ExpiresIn) * time.Second).UTC().Format(time.RFC3339)
	}
	key := "email_" + id + "_oauth_token"
	if err := s.mailerOAuthCallbackAllowed(p); err != nil {
		return err
	}
	return s.publishMailerOAuthToken(key, p.Previous, marshalOAuthToken(tok))
}

func marshalOAuthToken(tok config.OAuthToken) string {
	b, _ := json.Marshal(tok)
	return string(b)
}

func sameScopeSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	return scopeSetContains(a, b) && scopeSetContains(b, a)
}

func scopeSetContains(have, required []string) bool {
	set := make(map[string]struct{}, len(have))
	for _, item := range have {
		set[strings.TrimSpace(item)] = struct{}{}
	}
	for _, item := range required {
		if _, ok := set[strings.TrimSpace(item)]; !ok {
			return false
		}
	}
	return true
}

func mailerOAuthTokenRequest(ctx context.Context, endpoint string, values url.Values) (*tokenExchangeResponse, error) {
	for _, key := range []string{"client_secret", "code", "code_verifier", "refresh_token"} {
		security.RegisterSensitive(values.Get(key))
	}
	client := newMailerOAuthHTTPClient(20 * time.Second)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(values.Encode()))
	if err != nil {
		return nil, fmt.Errorf("build OAuth token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("OAuth token endpoint request failed")
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, (64<<10)+1))
	if err != nil || len(body) > 64<<10 || resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("OAuth token endpoint rejected request")
	}
	var token tokenExchangeResponse
	if json.Unmarshal(body, &token) != nil || token.AccessToken == "" || token.ExpiresIn < 0 || token.ExpiresIn > 31536000 {
		return nil, fmt.Errorf("OAuth token response is invalid")
	}
	security.RegisterSensitive(token.AccessToken)
	security.RegisterSensitive(token.RefreshToken)
	return &token, nil
}

func (s *Server) mailerOAuthAccessToken(ctx context.Context, a config.EmailAccount, grant string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if grant == "smtp" {
		grant = "send"
	}
	cfg := s.ConfigSnapshot()
	if cfg == nil || !cfg.Agent.AllowNetworkRequests || s.Vault == nil {
		return "", fmt.Errorf("mailer OAuth is unavailable")
	}
	switch strings.ToLower(strings.TrimSpace(a.OAuthSource)) {
	case "gog":
		protocolGrant := grant
		if grant == "read" {
			if strings.EqualFold(a.ReceiveProtocol, "pop3") {
				protocolGrant = "pop3"
			} else {
				protocolGrant = "imap"
			}
		} else if grant == "send" {
			protocolGrant = "smtp"
		}
		return s.gogService().MailProtocolToken(ctx, a.OAuthAccountID, protocolGrant)
	case "google_workspace":
		return s.googleWorkspaceMailerToken(ctx, grant, cfg)
	case "mailer":
		if strings.ToLower(a.OAuthProvider) != "microsoft" || a.ID == "" {
			return "", fmt.Errorf("unsupported mailer OAuth source")
		}
		if a.Disabled {
			return "", fmt.Errorf("account_disabled")
		}
		return s.microsoftMailerToken(ctx, a, grant)
	default:
		return "", fmt.Errorf("OAuth source is unavailable")
	}
}

func (s *Server) googleWorkspaceMailerToken(ctx context.Context, grant string, cfg *config.Config) (string, error) {
	gw := cfg.GoogleWorkspace
	if !gw.Enabled || !gw.Gmail {
		return "", fmt.Errorf("account_disabled")
	}
	need := "https://www.googleapis.com/auth/gmail.readonly"
	switch grant {
	case "read":
		need = "https://www.googleapis.com/auth/gmail.readonly|https://www.googleapis.com/auth/gmail.modify|https://mail.google.com/"
	case "send":
		if gw.ReadOnly || !gw.GmailSend {
			return "", fmt.Errorf("permission_denied")
		}
		need = "https://www.googleapis.com/auth/gmail.send|https://www.googleapis.com/auth/gmail.compose|https://www.googleapis.com/auth/gmail.modify|https://mail.google.com/"
	case "create":
		if gw.ReadOnly || !gw.GmailModifyLabels {
			return "", fmt.Errorf("permission_denied")
		}
		need = "https://www.googleapis.com/auth/gmail.labels|https://www.googleapis.com/auth/gmail.modify|https://mail.google.com/"
	case "modify":
		if gw.ReadOnly || !gw.GmailModifyLabels {
			return "", fmt.Errorf("permission_denied")
		}
		need = "https://www.googleapis.com/auth/gmail.modify|https://mail.google.com/"
	case "delete":
		return "", fmt.Errorf("permission_denied")
	default:
		return "", fmt.Errorf("invalid_grant")
	}
	return s.oauthTokenFromVault(ctx, "oauth_google_workspace", need, mailerGoogleTokenURL, gw.ClientID,
		s.mailerSecret("google_workspace_client_secret", gw.ClientSecret), "", nil)
}

// googleWorkspaceMailerScopeCapabilities reads the stored provider grant set
// without refreshing or returning OAuth token material. Config permissions are
// intersected by the caller; absent/legacy scopes fail closed.
func (s *Server) googleWorkspaceMailerScopeCapabilities() gog.MailerGmailScopes {
	cfg := s.ConfigSnapshot()
	if cfg == nil || !cfg.GoogleWorkspace.Enabled || !cfg.GoogleWorkspace.Gmail || s.Vault == nil {
		return gog.MailerGmailScopes{}
	}
	raw, err := s.Vault.ReadSecret("oauth_google_workspace")
	if err != nil || raw == "" {
		return gog.MailerGmailScopes{}
	}
	var token config.OAuthToken
	if json.Unmarshal([]byte(raw), &token) != nil || token.AccessToken == "" || len(token.Scopes) == 0 {
		return gog.MailerGmailScopes{}
	}
	security.RegisterSensitive(token.AccessToken)
	security.RegisterSensitive(token.RefreshToken)
	has := func(scope string) bool { return scopeSetContains(token.Scopes, []string{scope}) }
	fullMail := has("https://mail.google.com/")
	modify := has("https://www.googleapis.com/auth/gmail.modify")
	return gog.MailerGmailScopes{
		Read:            fullMail || modify || has("https://www.googleapis.com/auth/gmail.readonly"),
		Send:            fullMail || modify || has("https://www.googleapis.com/auth/gmail.compose") || has("https://www.googleapis.com/auth/gmail.send"),
		Modify:          fullMail || modify,
		CreateLabel:     fullMail || modify || has("https://www.googleapis.com/auth/gmail.labels"),
		PermanentDelete: fullMail,
		Protocol:        fullMail,
	}
}

func (s *Server) microsoftMailerToken(ctx context.Context, a config.EmailAccount, grant string) (string, error) {
	if grant == "delete" {
		return "", fmt.Errorf("permission_denied")
	}
	clientSecret := s.mailerSecret("email_"+a.ID+"_oauth_client_secret", a.OAuthClientSecret)
	return s.oauthTokenFromVault(ctx, "email_"+a.ID+"_oauth_token", microsoftGrantScope(a, grant), mailerMicrosoftTokenURL, a.OAuthClientID,
		clientSecret, strings.Join(microsoftMailerScopes(a), " "), microsoftMailerRequiredScopes(a))
}

func microsoftGrantScope(a config.EmailAccount, grant string) string {
	switch grant {
	case "read", "modify":
		if strings.EqualFold(a.ReceiveProtocol, "pop3") {
			return "https://outlook.office.com/POP.AccessAsUser.All"
		}
		return "https://outlook.office.com/IMAP.AccessAsUser.All"
	case "send":
		return "https://outlook.office.com/SMTP.Send"
	default:
		return ""
	}
}

func (s *Server) oauthTokenFromVault(ctx context.Context, key, requiredScope, endpoint, clientID, clientSecret, exactScope string, allowedScopes []string) (string, error) {
	if clientID == "" || clientSecret == "" {
		return "", fmt.Errorf("OAuth client is not configured")
	}
	security.RegisterSensitive(clientSecret)
	mailerOAuthRefreshMu.Lock()
	defer mailerOAuthRefreshMu.Unlock()
	raw, err := s.Vault.ReadSecret(key)
	if err != nil || raw == "" {
		return "", fmt.Errorf("sign_in_required")
	}
	var token config.OAuthToken
	if json.Unmarshal([]byte(raw), &token) != nil || token.AccessToken == "" {
		return "", fmt.Errorf("sign_in_required")
	}
	security.RegisterSensitive(token.AccessToken)
	security.RegisterSensitive(token.RefreshToken)
	if requiredScope != "" && !mailerHasScope(token.Scopes, requiredScope) {
		return "", fmt.Errorf("missing_scope")
	}
	refresh := false
	if token.Expiry != "" {
		expiry, parseErr := time.Parse(time.RFC3339, token.Expiry)
		if parseErr != nil {
			return "", fmt.Errorf("invalid_token_expiry")
		}
		refresh = time.Until(expiry) <= time.Minute
	}
	if !refresh {
		return token.AccessToken, nil
	}
	if token.RefreshToken == "" {
		return "", fmt.Errorf("sign_in_required")
	}
	values := url.Values{"grant_type": {"refresh_token"}, "client_id": {clientID}, "client_secret": {clientSecret}, "refresh_token": {token.RefreshToken}}
	if exactScope != "" {
		if strings.Contains(endpoint, "login.microsoftonline.com") {
			values.Set("scope", strings.Join(token.Scopes, " "))
		} else {
			values.Set("scope", exactScope)
		}
	}
	response, err := mailerOAuthTokenRequest(ctx, endpoint, values)
	if err != nil {
		return "", err
	}
	nextScopes := append([]string(nil), token.Scopes...)
	if strings.TrimSpace(response.Scope) != "" {
		nextScopes = strings.Fields(response.Scope)
	}
	if len(nextScopes) == 0 {
		return "", fmt.Errorf("missing_scope")
	}
	if allowedScopes != nil && !scopeSetContains(nextScopes, allowedScopes) {
		return "", fmt.Errorf("missing_scope")
	}
	if requiredScope != "" && !mailerHasScope(nextScopes, requiredScope) {
		return "", fmt.Errorf("missing_scope")
	}
	next := config.OAuthToken{AccessToken: response.AccessToken, RefreshToken: response.RefreshToken, TokenType: response.TokenType, Scopes: nextScopes}
	if next.RefreshToken == "" {
		next.RefreshToken = token.RefreshToken
	}
	if response.ExpiresIn > 0 {
		next.Expiry = time.Now().Add(time.Duration(response.ExpiresIn) * time.Second).UTC().Format(time.RFC3339)
	}
	if err := s.Vault.CompareAndSwapSecret(key, raw, marshalOAuthToken(next)); err != nil {
		return "", fmt.Errorf("token changed during refresh")
	}
	return next.AccessToken, nil
}

// A pipe separates alternative provider scopes that grant the same operation.
func mailerHasScope(scopes []string, required string) bool {
	for _, option := range strings.Split(required, "|") {
		if scopeSetContains(scopes, []string{option}) {
			return true
		}
	}
	return false
}
