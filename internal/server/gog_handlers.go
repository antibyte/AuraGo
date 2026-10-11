package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"time"

	"aurago/internal/config"
	"aurago/internal/gog"
	"aurago/internal/security"
	"aurago/internal/tools"
)

type gogOAuthSession struct {
	oauthSession
	Account  config.GogAccount `json:"account"`
	Previous string            `json:"previous"`
	Scopes   []string          `json:"scopes"`
}

func (s *Server) gogService() *gog.Service {
	return tools.NewGogService(s.ConfigSnapshot(), s.Vault, s.ConfigSnapshot)
}

func decodeGogRequest(w http.ResponseWriter, r *http.Request, target any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 256<<10)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if err := d.Decode(target); err != nil {
		return fmt.Errorf("invalid_request")
	}
	if d.Decode(new(any)) != io.EOF {
		return fmt.Errorf("invalid_request")
	}
	return nil
}

func handleGog(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		op := strings.TrimPrefix(r.URL.Path, "/api/gog/")
		if r.Method != http.MethodPost && !(r.Method == http.MethodGet && (op == "status" || op == "accounts" || op == "oauth/callback")) {
			jsonError(w, "Method not allowed", 405)
			return
		}
		cfg := s.ConfigSnapshot()
		if cfg == nil {
			jsonError(w, "Configuration unavailable", 503)
			return
		}
		svc := s.gogService()
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
		defer cancel()
		if s.integrationCtx != nil {
			stop := context.AfterFunc(s.integrationCtx, cancel)
			defer stop()
		}
		if op == "status" || op == "accounts" {
			_, e := svc.VerifiedBinary()
			accounts := []map[string]any{}
			for _, a := range cfg.Gog.Accounts {
				secret := false
				if s.Vault != nil {
					v, _ := s.Vault.ReadSecret(gog.ClientKey(a.ID))
					secret = v != ""
				}
				accounts = append(accounts, map[string]any{"account": a, "connected": svc.Connected(a.ID), "client_secret_present": secret})
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"version": gog.Version, "installed": e == nil, "enabled": cfg.Gog.Enabled, "readonly": cfg.Gog.ReadOnly, "accounts": accounts})
			return
		}
		if op == "install" {
			if !cfg.Agent.AllowNetworkRequests || !cfg.Agent.AllowFilesystemWrite {
				jsonError(w, "Network and filesystem write permissions are required", 403)
				return
			}
			if err := svc.Install(ctx); err != nil {
				jsonError(w, security.Scrub(err.Error()), 502)
				return
			}
			_ = json.NewEncoder(w).Encode(gog.Result{Status: "ok"})
			return
		}
		if op == "oauth/callback" || op == "oauth/manual" {
			s.finishGogOAuth(w, r, ctx, svc)
			return
		}
		var body struct {
			AccountID  string `json:"account_id"`
			Secret     string `json:"secret,omitempty"`
			Service    string `json:"service,omitempty"`
			ResourceID string `json:"resource_id,omitempty"`
		}
		if err := decodeGogRequest(w, r, &body); err != nil {
			jsonError(w, err.Error(), 400)
			return
		}
		a, ok := cfg.Gog.Account(body.AccountID)
		if !ok || !config.ValidGogAccountID(a.ID) {
			jsonError(w, "Unknown account", 404)
			return
		}
		if s.Vault == nil {
			jsonError(w, "Vault unavailable", 503)
			return
		}
		switch op {
		case "client-secret":
			if body.Secret == "" || len(body.Secret) > 4096 {
				jsonError(w, "Invalid client secret", 400)
				return
			}
			security.RegisterSensitive(body.Secret)
			if err := svc.Disconnect(a.ID); err != nil {
				jsonError(w, "Could not disconnect account", 500)
				return
			}
			if err := s.Vault.WriteSecret(gog.ClientKey(a.ID), body.Secret); err != nil {
				jsonError(w, "Could not save client secret", 500)
				return
			}
		case "disconnect":
			if err := svc.Disconnect(a.ID); err != nil {
				jsonError(w, "Could not disconnect account", 500)
				return
			}
		case "oauth/start":
			if !cfg.Gog.Enabled || !a.Enabled || !cfg.Agent.AllowNetworkRequests || a.ClientID == "" {
				jsonError(w, "Enable and configure the account first", 403)
				return
			}
			secret, _ := s.Vault.ReadSecret(gog.ClientKey(a.ID))
			if secret == "" {
				jsonError(w, "Save the client secret first", 400)
				return
			}
			redirect := strings.TrimSuffix(buildRedirectURI(r, cfg.Server.Host, cfg.Server.Port, cfg.Server.OAuthRedirectBaseURL), "/api/oauth/callback") + "/api/gog/oauth/callback"
			session, e := newOAuthSession("gog/"+a.ID, oauthFlowModeBrowserCallback, redirect, time.Now())
			if e != nil {
				jsonError(w, "Could not start sign-in", 500)
				return
			}
			scopes := gog.Scopes(a, cfg.Gog.ReadOnly)
			if len(scopes) == 0 {
				jsonError(w, "Select at least one service permission", 400)
				return
			}
			previous, e := svc.BeginLogin(a.ID, session.State, a.ClientID)
			if e != nil {
				jsonError(w, "Could not store sign-in", 500)
				return
			}
			saved := gogOAuthSession{session, a, previous, scopes}
			data, _ := json.Marshal(saved)
			if e = s.Vault.WriteSecret("oauth_state_"+session.State, string(data)); e != nil {
				jsonError(w, "Could not store sign-in", 500)
				return
			}
			auth, e := buildOAuthAuthorizationURL(config.ProviderEntry{OAuthAuthURL: gog.AuthURL, OAuthClientID: a.ClientID, OAuthScopes: strings.Join(scopes, " ")}, session)
			if e != nil {
				jsonError(w, "Could not build sign-in URL", 500)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok", "auth_url": auth, "redirect_uri": redirect})
			return
		case "test":
			requests := map[string]gog.Request{
				"gmail": {Operation: "labels"}, "calendar": {Operation: "calendars"}, "drive": {Operation: "list", Args: map[string]any{"max": 1}},
				"docs":     {Operation: "info", Args: map[string]any{"doc_id": body.ResourceID}},
				"sheets":   {Operation: "info", Args: map[string]any{"spreadsheet_id": body.ResourceID}},
				"slides":   {Operation: "read", Args: map[string]any{"presentation_id": body.ResourceID}},
				"contacts": {Operation: "list", Args: map[string]any{"max": 1}}, "tasks": {Operation: "lists", Args: map[string]any{"max": 1}},
			}
			req, ok := requests[body.Service]
			if !ok {
				jsonError(w, "Choose a service", 400)
				return
			}
			if (body.Service == "docs" || body.Service == "sheets" || body.Service == "slides") && body.ResourceID == "" {
				writeJSON(w, gog.Failure("test_object_required"))
				return
			}
			req.AccountID = a.ID
			if req.Service == "" {
				req.Service = body.Service
			}
			result := svc.Execute(ctx, "test", req, nil)
			result.Data = nil
			_ = json.NewEncoder(w).Encode(result)
			return
		default:
			jsonError(w, "Not found", 404)
			return
		}
		_ = json.NewEncoder(w).Encode(gog.Result{Status: "ok"})
	}
}

func (s *Server) finishGogOAuth(w http.ResponseWriter, r *http.Request, ctx context.Context, svc *gog.Service) {
	q := r.URL.Query()
	var pasted *url.URL
	if r.Method == http.MethodPost {
		var body struct {
			URL string `json:"url"`
		}
		if decodeGogRequest(w, r, &body) != nil {
			jsonError(w, "Invalid callback URL", 400)
			return
		}
		var e error
		pasted, e = url.Parse(body.URL)
		if e != nil {
			jsonError(w, "Invalid callback URL", 400)
			return
		}
		q = pasted.Query()
	}
	state := q.Get("state")
	if len(state) != 64 || q.Get("code") == "" || s.Vault == nil {
		jsonError(w, "Invalid OAuth response", 400)
		return
	}
	key := "oauth_state_" + state
	raw, e := s.Vault.ReadSecret(key)
	var saved gogOAuthSession
	if e != nil || json.Unmarshal([]byte(raw), &saved) != nil || saved.State != state || saved.ProviderID != "gog/"+saved.Account.ID || saved.CodeVerifier == "" || time.Now().After(saved.ExpiresAt) || saved.ExpiresAt.IsZero() {
		jsonError(w, "Invalid or expired OAuth state", 400)
		return
	}
	if pasted != nil {
		expected, _ := url.Parse(saved.RedirectURI)
		if pasted.Scheme != expected.Scheme || pasted.Host != expected.Host || pasted.Path != expected.Path || pasted.User != nil {
			jsonError(w, "Callback URL does not match this sign-in", 400)
			return
		}
	}
	if e = s.Vault.CompareAndSwapSecret(key, raw, ""); e != nil {
		jsonError(w, "OAuth state already consumed", 400)
		return
	}
	cfg := s.ConfigSnapshot()
	a, ok := cfg.Gog.Account(saved.Account.ID)
	if !ok || !cfg.Gog.Enabled || !a.Enabled || !cfg.Agent.AllowNetworkRequests || !reflect.DeepEqual(a, saved.Account) {
		jsonError(w, "Account changed; start sign-in again", 409)
		return
	}
	e = svc.Exchange(ctx, a, url.Values{"grant_type": {"authorization_code"}, "code": {q.Get("code")}, "redirect_uri": {saved.RedirectURI}, "code_verifier": {saved.CodeVerifier}}, saved.Previous, saved.Scopes)
	if e != nil {
		jsonError(w, security.Scrub(e.Error()), 400)
		return
	}
	_ = json.NewEncoder(w).Encode(gog.Result{Status: "ok", Message: "Google account connected. You can close this window."})
}
