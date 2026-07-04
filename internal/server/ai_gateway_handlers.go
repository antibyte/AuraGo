package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"aurago/internal/config"
	"aurago/internal/llm"
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
		w.Header().Set("Content-Type", "application/json")
		writeAIGatewayProbeResult(w, s, false)
	}
}

func handleAIGatewayTest(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		var req struct {
			ProviderID string `json:"provider_id"`
		}
		if r.Body != nil {
			_ = json.NewDecoder(io.LimitReader(r.Body, 4096)).Decode(&req)
		}
		writeAIGatewayProbeResult(w, s, true, req.ProviderID)
	}
}

func writeAIGatewayProbeResult(w http.ResponseWriter, s *Server, test bool, providerID ...string) {
	s.CfgMu.RLock()
	cfgSnapshot := *s.Cfg
	s.CfgMu.RUnlock()
	config.NormalizeAIGatewayConfig(&cfgSnapshot)
	cfg := cfgSnapshot.AIGateway

	if !cfg.Enabled {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":          "disabled",
			"message":         "AI Gateway is not enabled",
			"route_supported": false,
			"mode":            cfg.Mode,
			"privacy_mode":    cfg.LogMode,
		})
		return
	}
	if cfg.AccountID == "" || cfg.GatewayID == "" {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":          "no_credentials",
			"message":         "AI Gateway account ID or gateway ID is not configured",
			"route_supported": false,
			"mode":            cfg.Mode,
			"privacy_mode":    cfg.LogMode,
		})
		return
	}

	providerType, providerAccountID, providerFound := aiGatewayProviderTarget(cfgSnapshot, firstString(providerID))
	if !providerFound {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":          "provider_not_found",
			"message":         "Provider was not found",
			"route_supported": false,
			"mode":            cfg.Mode,
			"privacy_mode":    cfg.LogMode,
		})
		return
	}
	route := llm.ResolveAIGatewayRoute(&cfgSnapshot, providerType, providerAccountID)
	diagnostics := aiGatewayDiagnostics(route)
	if !test {
		json.NewEncoder(w).Encode(diagnostics)
		return
	}
	if !route.RouteSupported {
		json.NewEncoder(w).Encode(diagnostics)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	probeURL := strings.TrimRight(route.Endpoint, "/") + "/models"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, probeURL, nil)
	if err != nil {
		if test {
			w.WriteHeader(http.StatusServiceUnavailable)
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": scrubAIGatewayProbeMessage(err.Error(), cfgSnapshot),
		})
		return
	}
	if cfg.Token != "" && route.AuthHeader == "cf-aig-authorization" {
		req.Header.Set("cf-aig-authorization", "Bearer "+cfg.Token)
	}
	if route.GatewayID != "" && providerType == "workers-ai" {
		req.Header.Set("cf-aig-gateway-id", route.GatewayID)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		if test {
			w.WriteHeader(http.StatusServiceUnavailable)
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": scrubAIGatewayProbeMessage(err.Error(), cfgSnapshot),
		})
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg := strings.TrimSpace(string(body))
		if msg == "" {
			msg = fmt.Sprintf("AI Gateway probe failed (HTTP %d)", resp.StatusCode)
		}
		msg = scrubAIGatewayProbeMessage(msg, cfgSnapshot)
		if test {
			w.WriteHeader(http.StatusServiceUnavailable)
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":          "error",
			"message":         msg,
			"provider":        route.Provider,
			"endpoint":        route.Endpoint,
			"privacy_mode":    route.PrivacyMode,
			"live_status":     "failed",
			"http_status":     resp.StatusCode,
			"route_supported": route.RouteSupported,
		})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":          "ok",
		"message":         "AI Gateway connection successful",
		"provider":        route.Provider,
		"endpoint":        route.Endpoint,
		"privacy_mode":    route.PrivacyMode,
		"route_supported": route.RouteSupported,
		"live_status":     "ok",
	})
}

func aiGatewayDiagnostics(route llm.AIGatewayRoute) map[string]interface{} {
	return map[string]interface{}{
		"status":          route.Status,
		"message":         route.Message,
		"provider":        route.Provider,
		"mode":            route.Mode,
		"endpoint":        route.Endpoint,
		"segment":         route.Segment,
		"route_supported": route.RouteSupported,
		"privacy_mode":    route.PrivacyMode,
		"warnings":        route.Warnings,
	}
}

func aiGatewayProviderTarget(cfg config.Config, providerID string) (providerType, accountID string, found bool) {
	providerID = strings.TrimSpace(providerID)
	if providerID != "" {
		for _, provider := range cfg.Providers {
			if provider.ID == providerID {
				return strings.ToLower(strings.TrimSpace(provider.Type)), strings.TrimSpace(provider.AccountID), true
			}
		}
		return "", "", false
	}
	if strings.TrimSpace(cfg.LLM.ProviderType) == "" && strings.TrimSpace(cfg.LLM.Provider) != "" {
		for _, provider := range cfg.Providers {
			if provider.ID == cfg.LLM.Provider {
				return strings.ToLower(strings.TrimSpace(provider.Type)), strings.TrimSpace(provider.AccountID), true
			}
		}
	}
	return strings.ToLower(strings.TrimSpace(cfg.LLM.ProviderType)), strings.TrimSpace(cfg.LLM.AccountID), true
}

func firstString(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

func scrubAIGatewayProbeMessage(message string, cfg config.Config) string {
	replacements := []string{cfg.AIGateway.Token, cfg.LLM.APIKey, cfg.LLM.HelperAPIKey, cfg.FallbackLLM.APIKey}
	for _, provider := range cfg.Providers {
		replacements = append(replacements, provider.APIKey)
	}
	for _, secret := range replacements {
		secret = strings.TrimSpace(secret)
		if secret == "" {
			continue
		}
		message = strings.ReplaceAll(message, secret, "••••••••")
	}
	return message
}
