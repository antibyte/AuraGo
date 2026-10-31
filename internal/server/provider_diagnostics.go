package server

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"aurago/internal/config"
	"aurago/internal/llm"
)

type providerProbeObservation struct {
	Kind       string    `json:"kind"`
	Revision   string    `json:"revision"`
	Status     string    `json:"status"`
	Code       string    `json:"code"`
	FinishedAt time.Time `json:"finished_at"`
	Current    bool      `json:"current"`
}
type providerProbeBuffer struct {
	header http.Header
	status int
	bytes.Buffer
}

func (b *providerProbeBuffer) Header() http.Header { return b.header }
func (b *providerProbeBuffer) WriteHeader(status int) {
	if b.status == 0 {
		b.status = status
	}
}
func (b *providerProbeBuffer) Write(data []byte) (int, error) {
	if b.status == 0 {
		b.status = 200
	}
	return b.Buffer.Write(data)
}

func handleProviderTest(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", 405)
			return
		}
		var request struct {
			ProviderID string             `json:"provider_id"`
			Kind       string             `json:"probe_kind"`
			Provider   *providerTestDraft `json:"provider"`
		}
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10))
		decoder.DisallowUnknownFields()
		if decoder.Decode(&request) != nil || decoder.Decode(&struct{}{}) != io.EOF {
			writeAIGatewayError(w, nil, "invalid_request", 400, 0)
			return
		}
		if request.Kind == "" {
			request.Kind = "metadata"
		}
		if request.Kind != "metadata" && request.Kind != "generation" {
			writeAIGatewayError(w, nil, "invalid_probe_kind", 400, 0)
			return
		}
		s.CfgMu.RLock()
		cfg := s.Cfg.Clone()
		revision := s.providerETag()
		var p config.ProviderEntry
		saved := cfg.FindProvider(request.ProviderID)
		if saved != nil {
			p = *saved
			if request.Provider == nil && normalizeProviderAuthType(p.AuthType) == "oauth2" {
				_, p.APIKey = speechLabRuntimeChatProvider(&p, s.Vault)
			}
		}
		if request.Provider != nil {
			// The form as typed in: the probed entry comes from the request, the
			// saved entry only supplies secrets that stay valid for the same target.
			draft := request.Provider.entry(request.ProviderID, saved)
			if saved != nil && providerDraftSameTarget(saved, draft) && normalizeProviderAuthType(draft.AuthType) == "oauth2" {
				_, draft.APIKey = speechLabRuntimeChatProvider(saved, s.Vault)
			}
			p = draft
		}
		s.CfgMu.RUnlock()
		if saved == nil && request.Provider == nil {
			writeAIGatewayError(w, nil, "provider_not_found", 404, 0)
			return
		}
		if p.ID == "" {
			p.ID = "draft"
		}
		writeProviderProbe(w, s, r.Context(), cfg, p, request.Kind, map[string]any{"provider_id": p.ID, "probe_kind": request.Kind}, revision, request.Provider == nil)
	}
}

// providerTestDraft is the provider form as typed in. Secrets are optional: an
// empty or masked value means "the saved one", which only applies while the
// target (type, URL, account, OAuth endpoints) is unchanged.
type providerTestDraft struct {
	Name              string `json:"name"`
	Type              string `json:"type"`
	BaseURL           string `json:"base_url"`
	APIKey            string `json:"api_key"`
	Model             string `json:"model"`
	AuthType          string `json:"auth_type"`
	AccountID         string `json:"account_id"`
	GatewayID         string `json:"gateway_id"`
	OAuthAuthURL      string `json:"oauth_auth_url"`
	OAuthTokenURL     string `json:"oauth_token_url"`
	OAuthClientID     string `json:"oauth_client_id"`
	OAuthClientSecret string `json:"oauth_client_secret"`
	OAuthScopes       string `json:"oauth_scopes"`
}

// entry builds the probed provider; saved may be nil (new provider).
func (d *providerTestDraft) entry(id string, saved *config.ProviderEntry) config.ProviderEntry {
	p := config.ProviderEntry{
		ID: strings.TrimSpace(id), Name: d.Name, Type: strings.TrimSpace(d.Type), BaseURL: strings.TrimSpace(d.BaseURL),
		Model: strings.TrimSpace(d.Model), AuthType: d.AuthType, AccountID: strings.TrimSpace(d.AccountID), GatewayID: strings.TrimSpace(d.GatewayID),
		OAuthAuthURL: strings.TrimSpace(d.OAuthAuthURL), OAuthTokenURL: strings.TrimSpace(d.OAuthTokenURL),
		OAuthClientID: strings.TrimSpace(d.OAuthClientID), OAuthScopes: d.OAuthScopes,
	}
	sameTarget := saved != nil && providerDraftSameTarget(saved, p)
	if key := strings.TrimSpace(d.APIKey); key != "" && key != maskedKey {
		p.APIKey = key
	} else if sameTarget && normalizeProviderAuthType(p.AuthType) != "oauth2" {
		p.APIKey = saved.APIKey
	}
	if secret := strings.TrimSpace(d.OAuthClientSecret); secret != "" && secret != maskedKey {
		p.OAuthClientSecret = secret
	} else if sameTarget {
		p.OAuthClientSecret = saved.OAuthClientSecret
	}
	return p
}

// providerDraftSameTarget reports whether the draft still talks to the saved
// endpoint, so the saved credentials may be used for it.
func providerDraftSameTarget(saved *config.ProviderEntry, draft config.ProviderEntry) bool {
	same := func(a, b string) bool { return strings.EqualFold(strings.TrimSpace(a), strings.TrimSpace(b)) }
	return same(saved.Type, draft.Type) && same(saved.BaseURL, draft.BaseURL) && same(saved.AccountID, draft.AccountID) &&
		same(saved.GatewayID, draft.GatewayID) && same(saved.OAuthTokenURL, draft.OAuthTokenURL) && same(saved.OAuthAuthURL, draft.OAuthAuthURL)
}

func writeSavedProviderProbe(w http.ResponseWriter, s *Server, parent context.Context, cfg *config.Config, p config.ProviderEntry, kind string, result map[string]any, revision string) {
	writeProviderProbe(w, s, parent, cfg, p, kind, result, revision, true)
}

// writeProviderProbe runs the probe; record keeps the outcome as the provider's
// last test (not for drafts, which describe a form and not the saved entry).
func writeProviderProbe(w http.ResponseWriter, s *Server, parent context.Context, cfg *config.Config, p config.ProviderEntry, kind string, result map[string]any, revision string, record bool) {
	if _, busy := s.providerTests.LoadOrStore(p.ID, true); busy {
		writeAIGatewayError(w, result, "provider_test_busy", 409, 0)
		return
	}
	defer s.providerTests.Delete(p.ID)
	buffer := &providerProbeBuffer{header: make(http.Header)}
	executeProviderProbeNetwork(buffer, s, parent, cfg, p, kind, result)
	var body map[string]any
	if json.Unmarshal(buffer.Bytes(), &body) != nil {
		writeAIGatewayError(w, nil, "probe_failed", 500, 0)
		return
	}
	s.CfgMu.RLock()
	current := revision == s.providerETag()
	s.CfgMu.RUnlock()
	body["revision"] = revision
	body["current"] = current
	if !current {
		body["stale"] = true
	}
	if record {
		observation := providerProbeObservation{Kind: kind, Revision: revision, Status: providerProbeStringValue(body["status"]), Code: providerProbeStringValue(body["code"]), FinishedAt: time.Now(), Current: current}
		s.providerProbeHistory.Store(p.ID, observation)
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(buffer.status)
	_ = json.NewEncoder(w).Encode(body)
}
func providerProbeStringValue(value any) string { text, _ := value.(string); return text }

func handleProviderStatus(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", 405)
			return
		}
		s.CfgMu.RLock()
		cfg := s.Cfg.Clone()
		revision := s.providerETag()
		s.CfgMu.RUnlock()
		route := llm.ModelRoute{}
		if owner, ok := s.LLMClient.(interface{ ActiveRoute() llm.ModelRoute }); ok {
			route = owner.ActiveRoute()
		}
		providers := make([]map[string]any, 0, len(cfg.Providers))
		for _, p := range cfg.Providers {
			limits := llm.ResolveModelLimitsCached(llm.ModelRoute{ProviderID: p.ID, ProviderType: p.Type, BaseURL: p.BaseURL, APIKey: p.RuntimeAPIKey(), Model: p.Model, ContextWindowOverride: p.ContextWindow, MaxOutputTokensOverride: p.MaxOutputTokens, Primary: p.ID == cfg.LLM.Provider}, cfg.Agent.ContextWindow)
			errors := []string{}
			if p.APIKey == maskedKey || p.OAuthClientSecret == maskedKey {
				errors = append(errors, "invalid_masked_credential")
			}
			runtime, _ := speechLabRuntimeChatProvider(&p, s.Vault)
			if strings.TrimSpace(p.Type) == "" || strings.TrimSpace(p.BaseURL) == "" && p.Type != "openai" && p.Type != "cerebras" && p.Type != "perplexity" && p.Type != "workers-ai" && p.Type != "cloudflare-rest" {
				errors = append(errors, "endpoint_missing")
			}
			if err := config.ValidateGatewayProvider(p); err != nil {
				errors = append(errors, "gateway_incomplete")
			}
			if strings.TrimSpace(p.Model) == "" {
				errors = append(errors, "model_missing")
			}
			if !runtime.Configured {
				errors = append(errors, runtime.Reason)
			}
			entry := map[string]any{"provider_id": p.ID, "model": p.Model, "active": route.ProviderID != "" && route.ProviderID == p.ID, "references": providerReferences(cfg, p.ID), "health": llm.ProviderHealth(p.ID), "probe_state": limits.ProbeStatus, "oauth": agodeskProviderOAuthStatus(s, p.ID, ""), "errors": errors, "context_window": limits.ContextWindow, "max_output_tokens": limits.MaxOutputTokens, "context_source": limits.ContextSource, "output_source": limits.OutputSource, "metadata_source": limits.MetadataSource}
			if _, busy := s.providerTests.Load(p.ID); busy {
				entry["test_running"] = true
			}
			if recent, ok := s.providerProbeHistory.Load(p.ID); ok {
				observation := recent.(providerProbeObservation)
				observation.Current = observation.Revision == revision
				entry["last_test"] = observation
			}
			providers = append(providers, entry)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("ETag", revision)
		var failover any
		if manager, ok := s.LLMClient.(*llm.FailoverManager); ok {
			failover = manager.PassiveStatus()
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"failover": failover, "revision": revision, "active_route": map[string]any{"provider_id": route.ProviderID, "provider_type": route.ProviderType, "model": route.Model, "known": route.ProviderID != "", "fallback": route.ProviderID != "" && route.ProviderID == cfg.FallbackLLM.Provider}, "providers": providers, "missing_references": cfg.MissingProviderReferences()})
	}
}
