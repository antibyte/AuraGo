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

func TokenKey(id string) string  { return "oauth_gog_" + id }
func ClientKey(id string) string { return "oauth_gog_client_" + id }

type Token struct {
	AccessToken  string    `json:"access_token,omitempty"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	Expiry       time.Time `json:"expiry"`
	Scopes       []string  `json:"scopes,omitempty"`
	Login        string    `json:"login,omitempty"`
	ClientID     string    `json:"client_id,omitempty"`
}

func Scopes(a config.GogAccount, readonly bool) []string {
	var out []string
	for _, name := range config.GogServices {
		g := a.Services[name]
		if !g.Read && !g.Create && !g.Update && !g.Delete && !g.Send {
			continue
		}
		write := !readonly && (g.Create || g.Update || g.Delete || g.Send)
		scope := map[string]string{"gmail": "gmail.readonly", "calendar": "calendar.readonly", "drive": "drive.readonly", "docs": "documents.readonly", "sheets": "spreadsheets.readonly", "slides": "presentations.readonly", "contacts": "contacts.readonly", "tasks": "tasks.readonly"}[name]
		if write {
			scope = map[string]string{"gmail": "gmail.modify", "calendar": "calendar", "drive": "drive", "docs": "documents", "sheets": "spreadsheets", "slides": "presentations", "contacts": "contacts", "tasks": "tasks"}[name]
		}
		out = append(out, "https://www.googleapis.com/auth/"+scope)
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
	if !config.ValidGogAccountID(id) || s.Vault == nil {
		return "", fmt.Errorf("invalid_account")
	}
	raw, _ := json.Marshal(Token{Login: state, ClientID: clientID})
	err := s.changeAccount(id, func() error { return s.Vault.WriteSecret(TokenKey(id), string(raw)) })
	return string(raw), err
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
	if wire.Scope != "" {
		scopes = strings.Fields(wire.Scope)
	}
	if wire.RefreshToken == "" {
		var old Token
		_ = json.Unmarshal([]byte(previous), &old)
		wire.RefreshToken = old.RefreshToken
	}
	t := Token{AccessToken: wire.AccessToken, RefreshToken: wire.RefreshToken, Expiry: time.Now().Add(time.Duration(wire.ExpiresIn) * time.Second), Scopes: scopes, ClientID: a.ClientID}
	raw, _ := json.Marshal(t)
	return s.Vault.CompareAndSwapSecret(TokenKey(a.ID), previous, string(raw))
}

func (s *Service) accessToken(ctx context.Context, a config.GogAccount, op Operation) (string, error) {
	state := s.state(a.ID)
	state.refresh.Lock()
	defer state.refresh.Unlock()
	t, raw, err := s.token(a.ID)
	if err != nil || t.AccessToken == "" || t.ClientID != a.ClientID {
		return "", fmt.Errorf("sign_in_required")
	}
	if !tokenHasScope(t, op) {
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
	if !tokenHasScope(t, op) {
		return "", fmt.Errorf("missing_scope")
	}
	return t.AccessToken, nil
}

func tokenHasScope(t Token, op Operation) bool {
	needed := Scopes(config.GogAccount{Services: map[string]config.GogGrants{op.Service: {Read: true, Update: op.Grant != "read"}}}, op.Grant == "read")
	matched := false
	for _, want := range needed {
		for _, have := range t.Scopes {
			if have == want || have == strings.TrimSuffix(want, ".readonly") || op.Service == "gmail" && have == "https://www.googleapis.com/auth/gmail.modify" {
				matched = true
			}
		}
	}
	return matched
}
