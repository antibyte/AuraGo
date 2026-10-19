package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"aurago/internal/budget"
	"aurago/internal/config"
	"aurago/internal/llm"
	"aurago/internal/prompts"
	openai "github.com/sashabaranov/go-openai"
)

func registerAIGatewayHandlers(mux *http.ServeMux, s *Server) {
	mux.HandleFunc("/api/ai-gateway/status", handleAIGatewayStatus(s))
	mux.HandleFunc("/api/ai-gateway/test", handleAIGatewayTest(s))
}

func handleAIGatewayStatus(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		writeAIGatewayProbeResult(w, s, r.Context(), false, "metadata", r.URL.Query().Get("provider_id"))
	}
}

func handleAIGatewayTest(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var request struct {
			ProviderID string `json:"provider_id"`
			ProbeKind  string `json:"probe_kind"`
		}
		if r.Body != nil {
			decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
			decoder.DisallowUnknownFields()
			if err := decoder.Decode(&request); err != nil && err != io.EOF {
				writeAIGatewayError(w, nil, "invalid_request", http.StatusBadRequest, 0)
				return
			}
			if decoder.Decode(&struct{}{}) != io.EOF {
				writeAIGatewayError(w, nil, "invalid_request", http.StatusBadRequest, 0)
				return
			}
		}
		if request.ProbeKind == "" {
			request.ProbeKind = "metadata"
		}
		if request.ProbeKind != "metadata" && request.ProbeKind != "generation" {
			writeAIGatewayError(w, nil, "invalid_probe_kind", http.StatusBadRequest, 0)
			return
		}
		writeAIGatewayProbeResult(w, s, r.Context(), true, request.ProbeKind, request.ProviderID)
	}
}

func writeAIGatewayProbeResult(w http.ResponseWriter, s *Server, parent context.Context, test bool, kind, providerID string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	s.CfgMu.RLock()
	cfg := *s.Cfg
	cfg.Providers = append([]config.ProviderEntry(nil), s.Cfg.Providers...)
	s.CfgMu.RUnlock()
	providerID = strings.TrimSpace(providerID)
	if providerID == "" {
		providerID = cfg.LLM.Provider
	}
	p := config.ProviderEntry{ID: providerID, Type: cfg.LLM.ProviderType, BaseURL: cfg.LLM.BaseURL, APIKey: cfg.LLM.APIKey, AccountID: cfg.LLM.AccountID, GatewayID: cfg.LLM.GatewayID, Model: cfg.LLM.Model}
	if saved := cfg.FindProvider(providerID); saved != nil {
		p = *saved
	} else if providerID != "" && providerID != cfg.LLM.Provider {
		writeAIGatewayError(w, nil, "provider_not_found", http.StatusNotFound, 0)
		return
	}
	route := llm.ResolveProviderAIGatewayRoute(&cfg, p)
	result := aiGatewayDiagnostics(route)
	result["provider_id"] = providerID
	result["probe_kind"] = kind
	if !test || !route.RouteSupported {
		json.NewEncoder(w).Encode(result)
		return
	}
	RegisterLLMSecrets(&cfg)
	ctx, cancel := context.WithTimeout(parent, 20*time.Second)
	defer cancel()
	if s.integrationCtx != nil {
		stop := context.AfterFunc(s.integrationCtx, cancel)
		defer stop()
	}
	if kind == "generation" {
		if p.APIKey == "" || p.Model == "" {
			writeAIGatewayError(w, result, "provider_auth_missing", http.StatusConflict, 0)
			return
		}
		if s.BudgetTracker == nil {
			writeAIGatewayError(w, result, "budget_unavailable", http.StatusConflict, 0)
			return
		}
		client := llm.NewClientFromProviderWithConfig(&cfg, p.Type, p.BaseURL, p.APIKey, p.AccountID, p.GatewayID)
		ctx = budget.WithTracking(llm.WithSingleCompletionAttempt(ctx), s.BudgetTracker, "ai_gateway_test")
		request := openai.ChatCompletionRequest{Model: p.Model, MaxTokens: 32, Messages: []openai.ChatCompletionMessage{{Role: "user", Content: "Reply OK."}}}
		response, err := budget.CompleteChat(ctx, llm.WrapOpenAIClient(client), request, llm.ModelRoute{ProviderID: p.ID, ProviderType: p.Type, BaseURL: p.BaseURL, Model: p.Model}, "ai_gateway_test", 32, prompts.CountTokensForModelContext)
		if err != nil {
			code := "model_test_failed"
			upstreamStatus := 0
			var apiError *openai.APIError
			if errors.As(err, &apiError) {
				upstreamStatus = apiError.HTTPStatusCode
			}
			if errors.Is(err, budget.ErrCompletionBudgetBlocked) {
				code = "budget_blocked"
			} else if ctx.Err() != nil {
				code = "probe_cancelled"
			}
			writeAIGatewayError(w, result, code, http.StatusServiceUnavailable, upstreamStatus)
			return
		}
		if len(response.Choices) == 0 || response.Choices[0].FinishReason != openai.FinishReasonStop || strings.TrimSpace(response.Choices[0].Message.Content) == "" {
			writeAIGatewayError(w, result, "model_test_incomplete", http.StatusServiceUnavailable, 0)
			return
		}
		result["status"] = "ok"
		result["live_status"] = "ok"
		result["code"] = "model_test_ok"
		json.NewEncoder(w).Encode(result)
		return
	}
	if (p.Type == "workers-ai" || p.Type == "cloudflare-rest") && p.APIKey == "" {
		writeAIGatewayError(w, result, "provider_auth_missing", http.StatusConflict, 0)
		return
	}
	req, client, err := llm.AIGatewayMetadataProbe(ctx, &cfg, p)
	if err != nil {
		writeAIGatewayError(w, result, "probe_failed", http.StatusServiceUnavailable, 0)
		return
	}
	if req == nil {
		result["live_status"] = "skipped"
		result["code"] = "metadata_probe_unavailable"
		json.NewEncoder(w).Encode(result)
		return
	}
	resp, err := client.Do(req)
	if err != nil {
		code := "probe_failed"
		if ctx.Err() != nil {
			code = "probe_cancelled"
		}
		writeAIGatewayError(w, result, code, http.StatusServiceUnavailable, 0)
		return
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		code := "gateway_route_unavailable"
		if resp.StatusCode == 401 || resp.StatusCode == 403 {
			code = "gateway_auth_failed"
		} else if resp.StatusCode == 429 {
			code = "gateway_rate_limited"
		}
		writeAIGatewayError(w, result, code, http.StatusServiceUnavailable, resp.StatusCode)
		return
	}
	result["status"] = "ok"
	result["live_status"] = "ok"
	result["code"] = "metadata_probe_ok"
	json.NewEncoder(w).Encode(result)
}

func writeAIGatewayError(w http.ResponseWriter, result map[string]any, code string, status, upstreamStatus int) {
	if result == nil {
		result = map[string]any{}
	} else {
		result["live_status"] = "failed"
	}
	result["status"] = "error"
	result["code"] = code
	result["message"] = "AI Gateway test failed"
	if upstreamStatus > 0 {
		result["http_status"] = upstreamStatus
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(result)
}

func aiGatewayDiagnostics(route llm.AIGatewayRoute) map[string]any {
	return map[string]any{"status": route.Status, "message": route.Message, "provider": route.Provider, "mode": route.Mode, "endpoint": route.Endpoint, "segment": route.Segment, "route_supported": route.RouteSupported, "privacy_mode": route.PrivacyMode, "warnings": route.Warnings, "request_timeout_ms": route.RequestTimeoutMS, "request_timeout_effective_ms": route.EffectiveTimeoutMS}
}
