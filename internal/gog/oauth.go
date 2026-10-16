package gog

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"aurago/internal/config"
	"aurago/internal/security"
)

const TokenURL = "https://oauth2.googleapis.com/token"
const AuthURL = "https://accounts.google.com/o/oauth2/v2/auth"

// The separator cannot occur in an account ID, including legacy client_* IDs.
func TokenKey(id string) string  { return "oauth_gog_token:" + id }
func ClientKey(id string) string { return "oauth_gog_client_" + id }

type Token struct {
	AccessToken  string    `json:"access_token,omitempty"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	Expiry       time.Time `json:"expiry"`
	Scopes       []string  `json:"scopes,omitempty"`
	Login        string    `json:"login,omitempty"`
	ClientID     string    `json:"client_id,omitempty"`
	PendingLogin string    `json:"pending_login,omitempty"`
}

func Scopes(a config.GogAccount, readonly bool) []string {
	var out []string
	seen := make(map[string]struct{})
	add := func(scope string) {
		if scope == "" {
			return
		}
		full := scope
		if !strings.Contains(full, "://") {
			full = "https://www.googleapis.com/auth/" + scope
		}
		if _, ok := seen[full]; ok {
			return
		}
		seen[full] = struct{}{}
		out = append(out, full)
	}
	for _, name := range config.GogServices {
		g := a.Services[name]
		if name == "gmail" {
			writeScope := !readonly && (g.Update || g.Delete)
			if g.Read && !writeScope {
				add("gmail.readonly")
			}
			if !readonly {
				if writeScope {
					// Existing GoG delete operations are drafts, labels, or trash;
					// they do not require permanent-delete's full-mail scope.
					add("gmail.modify")
				} else {
					if g.Create {
						// GoG's create grant covers both Gmail drafts and labels.
						// Keep each API permission narrow; the Mailer label adapter
						// separately requires gmail.labels (or a broader valid scope).
						add("gmail.compose")
						add("gmail.labels")
					}
					if g.Send && !g.Create {
						add("gmail.send")
					}
				}
			}
			continue
		}
		write := !readonly && (g.Create || g.Update || g.Delete || g.Send)
		if g.Read || write {
			scope := map[string]string{"calendar": "calendar.readonly", "drive": "drive.readonly", "docs": "documents.readonly", "sheets": "spreadsheets.readonly", "slides": "presentations.readonly", "contacts": "contacts.readonly", "tasks": "tasks.readonly"}[name]
			if write {
				scope = map[string]string{"calendar": "calendar", "drive": "drive", "docs": "documents", "sheets": "spreadsheets", "slides": "presentations", "contacts": "contacts", "tasks": "tasks"}[name]
			}
			add(scope)
		}
	}
	return out
}

func (s *Service) token(id string) (Token, string, error) {
	if s.Vault == nil {
		return Token{}, "", fmt.Errorf("vault_unavailable")
	}
	raw, err := s.Vault.ReadSecret(TokenKey(id))
	if err != nil {
		return Token{}, "", fmt.Errorf("sign_in_required")
	}
	var t Token
	err = json.Unmarshal([]byte(raw), &t)
	security.RegisterSensitive(t.AccessToken)
	security.RegisterSensitive(t.RefreshToken)
	return t, raw, err
}

func (s *Service) Connected(id string) bool {
	t, _, e := s.token(id)
	a, ok := s.Config().Gog.Account(id)
	return e == nil && ok && t.AccessToken != "" && t.ClientID == a.ClientID
}

func (s *Service) BeginLogin(id, state, clientID string) (string, error) {
	if !config.ValidGogAccountID(id) || state == "" || clientID == "" || s.Vault == nil {
		return "", fmt.Errorf("invalid_account")
	}
	raw, err := s.Vault.ReadSecret(TokenKey(id))
	if err == nil {
		if raw == "" {
			return "", fmt.Errorf("empty stored token")
		}
		return raw, nil
	}
	if err != security.ErrSecretNotFound {
		return "", err
	}
	markerBytes, err := json.Marshal(Token{PendingLogin: state, ClientID: clientID})
	if err != nil {
		return "", err
	}
	marker := string(markerBytes)
	if err := s.Vault.WriteSecretIfAbsent(TokenKey(id), marker); err == nil {
		return marker, nil
	} else if err != security.ErrSecretExists {
		return "", err
	}
	// A concurrent sign-in may have installed its marker or completed. Pin this
	// flow to the value it observed so only one final CAS can win.
	raw, err = s.Vault.ReadSecret(TokenKey(id))
	if err != nil || raw == "" {
		return "", fmt.Errorf("sign-in changed during start")
	}
	return raw, nil
}

func (s *Service) Disconnect(id string) error {
	if !config.ValidGogAccountID(id) || s.Vault == nil {
		return fmt.Errorf("invalid_account")
	}
	return s.changeAccount(id, func() error { return s.Vault.DeleteSecret(TokenKey(id)) })
}

func (s *Service) Exchange(ctx context.Context, a config.GogAccount, values url.Values, previous string, scopes []string) error {
	secret, err := s.Vault.ReadSecret(ClientKey(a.ID))
	if err != nil || secret == "" {
		return fmt.Errorf("client_secret_required")
	}
	security.RegisterSensitive(secret)
	security.RegisterSensitive(values.Get("code"))
	security.RegisterSensitive(values.Get("refresh_token"))
	values.Set("client_id", a.ClientID)
	values.Set("client_secret", secret)
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, TokenURL, strings.NewReader(values.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	client := s.tokenClient
	if client == nil {
		client = security.NewSSRFProtectedHTTPClientSameOrigin(20 * time.Second)
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("token_exchange_failed")
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, (64<<10)+1))
	if err != nil || len(data) > 64<<10 || resp.StatusCode != 200 {
		return fmt.Errorf("token_exchange_rejected")
	}
	var wire struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
		Scope        string `json:"scope"`
	}
	if json.Unmarshal(data, &wire) != nil || wire.AccessToken == "" || wire.ExpiresIn <= 0 || wire.ExpiresIn > 86400 {
		return fmt.Errorf("invalid_token_response")
	}
	security.RegisterSensitive(wire.AccessToken)
	security.RegisterSensitive(wire.RefreshToken)
	requestedScopes := append([]string(nil), scopes...)
	var previousToken Token
	if values.Get("grant_type") == "authorization_code" && previous == "" {
		return fmt.Errorf("oauth_pending_marker_missing")
	}
	if previous != "" && json.Unmarshal([]byte(previous), &previousToken) != nil {
		return fmt.Errorf("invalid_previous_token")
	}
	if strings.TrimSpace(wire.Scope) != "" {
		scopes = strings.Fields(wire.Scope)
	} else if values.Get("grant_type") == "refresh_token" {
		// Refresh responses may omit scope. Preserve the provider-granted Vault
		// set exactly; never reconstruct it from current local permissions.
		scopes = append([]string(nil), previousToken.Scopes...)
	} else {
		return fmt.Errorf("provider_omitted_granted_scopes")
	}
	if len(scopes) == 0 || values.Get("grant_type") != "refresh_token" && !scopeSetContains(scopes, requestedScopes) {
		return fmt.Errorf("provider_granted_scopes_invalid")
	}
	if wire.RefreshToken == "" {
		wire.RefreshToken = previousToken.RefreshToken
	}
	if wire.RefreshToken == "" && values.Get("grant_type") == "authorization_code" {
		return fmt.Errorf("provider_omitted_refresh_token")
	}
	t := Token{AccessToken: wire.AccessToken, RefreshToken: wire.RefreshToken, Expiry: time.Now().Add(time.Duration(wire.ExpiresIn) * time.Second), Scopes: scopes, ClientID: a.ClientID}
	if s.Config != nil {
		current := s.Config()
		if current == nil {
			return fmt.Errorf("account_changed")
		}
		latest, ok := current.Gog.Account(a.ID)
		if !ok || latest.ClientID != a.ClientID {
			return fmt.Errorf("account_changed")
		}
	}
	raw, _ := json.Marshal(t)
	return s.Vault.CompareAndSwapSecret(TokenKey(a.ID), previous, string(raw))
}

func (s *Service) accessToken(ctx context.Context, a config.GogAccount, op Operation) (string, error) {
	return s.accessTokenWithExtraScope(ctx, a, op, "")
}

func (s *Service) accessTokenWithExtraScope(ctx context.Context, a config.GogAccount, op Operation, extraScope string) (string, error) {
	state := s.state(a.ID)
	state.refresh.Lock()
	defer state.refresh.Unlock()
	t, raw, err := s.token(a.ID)
	if err != nil || t.AccessToken == "" || t.ClientID != a.ClientID {
		return "", fmt.Errorf("sign_in_required")
	}
	if !tokenHasScope(t, op) || extraScope != "" && !scopeSetContains(t.Scopes, []string{extraScope}) {
		return "", fmt.Errorf("missing_scope")
	}
	if time.Until(t.Expiry) > time.Minute {
		return t.AccessToken, nil
	}
	if t.RefreshToken == "" {
		return "", fmt.Errorf("sign_in_required")
	}
	if err = s.Exchange(ctx, a, url.Values{"grant_type": {"refresh_token"}, "refresh_token": {t.RefreshToken}}, raw, t.Scopes); err != nil {
		return "", err
	}
	t, _, err = s.token(a.ID)
	if err != nil {
		return "", err
	}
	if !tokenHasScope(t, op) || extraScope != "" && !scopeSetContains(t.Scopes, []string{extraScope}) {
		return "", fmt.Errorf("missing_scope")
	}
	return t.AccessToken, nil
}

// MailAccessToken exposes GoG's existing Gmail token for the Mailer without
// exposing Vault data or invoking the optional gog CLI. Provider scopes and
// current account grants are both checked before returning the access token.
func (s *Service) MailAccessToken(ctx context.Context, id, grant string) (string, error) {
	if s == nil || s.Config == nil {
		return "", fmt.Errorf("account_disabled")
	}
	cfg := s.Config()
	if cfg == nil || !cfg.Gog.Enabled || !cfg.Agent.AllowNetworkRequests {
		return "", fmt.Errorf("account_disabled")
	}
	a, ok := cfg.Gog.Account(id)
	if !ok || !a.Enabled || !a.DesktopAccess {
		return "", fmt.Errorf("account_disabled")
	}
	g := a.Services["gmail"]
	allowed := false
	switch grant {
	case "read":
		allowed = g.Read
	case "create":
		allowed = g.Create && !cfg.Gog.ReadOnly
	case "send":
		allowed = g.Send && !cfg.Gog.ReadOnly
	case "modify":
		allowed = g.Update && !cfg.Gog.ReadOnly
	case "delete":
		allowed = g.Delete && !cfg.Gog.ReadOnly
	default:
		return "", fmt.Errorf("invalid_grant")
	}
	if !allowed {
		return "", fmt.Errorf("permission_denied")
	}
	op := Operation{Service: "gmail", Grant: grant}
	extraScope := ""
	switch grant {
	case "create":
		// Mailer create is specifically Gmail label creation. Native GoG draft
		// creation continues to use the generic create grant and gmail.compose.
		op.Grant = "create_label"
	case "delete":
		// Native GoG delete is limited to supported draft/label/trash operations;
		// Mailer's permanent delete capability has a distinct full-mail gate.
		extraScope = "https://mail.google.com/"
	}
	return s.accessTokenWithExtraScope(ctx, a, op, extraScope)
}

// MailerGmailScopes reports provider scopes from the current Vault token without
// refreshing it or returning token material. Callers intersect these values
// with GoG's current per-operation grants before advertising Mailer features.
type MailerGmailScopes struct {
	Read            bool
	Send            bool
	Modify          bool
	CreateLabel     bool
	PermanentDelete bool
	Protocol        bool
}

func (s *Service) MailerGmailScopeCapabilities(id string) MailerGmailScopes {
	if s == nil || s.Config == nil || s.Vault == nil {
		return MailerGmailScopes{}
	}
	cfg := s.Config()
	if cfg == nil || !cfg.Gog.Enabled {
		return MailerGmailScopes{}
	}
	a, ok := cfg.Gog.Account(id)
	if !ok || !a.Enabled || !a.DesktopAccess {
		return MailerGmailScopes{}
	}
	raw, err := s.Vault.ReadSecret(TokenKey(id))
	if err != nil || raw == "" {
		return MailerGmailScopes{}
	}
	var token Token
	if json.Unmarshal([]byte(raw), &token) != nil || token.AccessToken == "" || token.ClientID != a.ClientID {
		return MailerGmailScopes{}
	}
	security.RegisterSensitive(token.AccessToken)
	has := func(scope string) bool { return scopeSetContains(token.Scopes, []string{scope}) }
	fullMail := has("https://mail.google.com/")
	modify := has("https://www.googleapis.com/auth/gmail.modify")
	return MailerGmailScopes{
		Read:            modify || fullMail || has("https://www.googleapis.com/auth/gmail.readonly"),
		Send:            modify || fullMail || has("https://www.googleapis.com/auth/gmail.send") || has("https://www.googleapis.com/auth/gmail.compose"),
		Modify:          modify || fullMail,
		CreateLabel:     modify || fullMail || has("https://www.googleapis.com/auth/gmail.labels"),
		PermanentDelete: fullMail,
		Protocol:        fullMail,
	}
}

// MailProtocolToken returns a GoG token for IMAP/POP3/SMTP XOAUTH2. These
// protocols require Google's full mail.google.com grant; Gmail API read/send
// scopes alone must never be reused for them.
func (s *Service) MailProtocolToken(ctx context.Context, id, grant string) (string, error) {
	if s == nil || s.Config == nil {
		return "", fmt.Errorf("account_disabled")
	}
	cfg := s.Config()
	if cfg == nil || !cfg.Gog.Enabled || !cfg.Agent.AllowNetworkRequests {
		return "", fmt.Errorf("account_disabled")
	}
	a, ok := cfg.Gog.Account(id)
	if !ok || !a.Enabled || !a.DesktopAccess {
		return "", fmt.Errorf("account_disabled")
	}
	g := a.Services["gmail"]
	var op Operation
	switch grant {
	case "read", "imap", "pop3":
		if !g.Read {
			return "", fmt.Errorf("permission_denied")
		}
		op = Operation{Service: "gmail", Grant: "read"}
	case "smtp", "send":
		if !g.Send || cfg.Gog.ReadOnly {
			return "", fmt.Errorf("permission_denied")
		}
		op = Operation{Service: "gmail", Grant: "send"}
	default:
		return "", fmt.Errorf("invalid_grant")
	}
	return s.accessTokenWithExtraScope(ctx, a, op, "https://mail.google.com/")
}

func tokenHasScope(t Token, op Operation) bool {
	if op.Service == "gmail" {
		has := func(scope string) bool {
			full := scope
			if !strings.Contains(full, "://") {
				full = "https://www.googleapis.com/auth/" + scope
			}
			return scopeSetContains(t.Scopes, []string{full})
		}
		switch op.Grant {
		case "read":
			return has("gmail.readonly") || has("gmail.modify") || has("https://mail.google.com/")
		case "create":
			return has("gmail.compose") || has("gmail.labels") || has("gmail.modify") || has("https://mail.google.com/")
		case "create_label":
			return has("gmail.labels") || has("gmail.modify") || has("https://mail.google.com/")
		case "update", "modify":
			return has("gmail.modify") || has("https://mail.google.com/")
		case "delete":
			return has("gmail.modify") || has("https://mail.google.com/")
		case "send":
			return has("gmail.send") || has("gmail.compose") || has("gmail.modify") || has("https://mail.google.com/")
		default:
			return false
		}
	}
	needed := Scopes(config.GogAccount{Services: map[string]config.GogGrants{op.Service: {Read: true, Update: op.Grant != "read"}}}, op.Grant == "read")
	if len(needed) == 0 {
		return false
	}
	return scopeSetContains(t.Scopes, needed) || op.Grant == "read" && scopeSetContains(t.Scopes, []string{strings.ReplaceAll(needed[0], ".readonly", "")})
}

func scopeSetContains(have, required []string) bool {
	set := make(map[string]struct{}, len(have))
	for _, scope := range have {
		set[strings.TrimSpace(scope)] = struct{}{}
	}
	for _, scope := range required {
		if _, ok := set[strings.TrimSpace(scope)]; !ok {
			return false
		}
	}
	return true
}
