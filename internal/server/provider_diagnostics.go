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
			ProviderID string `json:"provider_id"`
			Kind       string `json:"probe_kind"`
		}
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
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
			if normalizeProviderAuthType(p.AuthType) == "oauth2" {
				_, p.APIKey = speechLabRuntimeChatProvider(&p, s.Vault)
			}
		}
		s.CfgMu.RUnlock()
		if saved == nil {
			writeAIGatewayError(w, nil, "provider_not_found", 404, 0)
			return
		}
		writeSavedProviderProbe(w, s, r.Context(), cfg, p, request.Kind, map[string]any{"provider_id": p.ID, "probe_kind": request.Kind}, revision)
	}
}

func writeSavedProviderProbe(w http.ResponseWriter, s *Server, parent context.Context, cfg *config.Config, p config.ProviderEntry, kind string, result map[string]any, revision string) {
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
	observation := providerProbeObservation{Kind: kind, Revision: revision, Status: providerProbeStringValue(body["status"]), Code: providerProbeStringValue(body["code"]), FinishedAt: time.Now(), Current: current}
	s.providerProbeHistory.Store(p.ID, observation)
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
