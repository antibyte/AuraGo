package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
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
		writeAIGatewayProbeResult(w, s, true)
	}
}

func writeAIGatewayProbeResult(w http.ResponseWriter, s *Server, test bool) {
	s.CfgMu.RLock()
	cfg := s.Cfg.AIGateway
	s.CfgMu.RUnlock()

	if !cfg.Enabled {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "disabled",
			"message": "AI Gateway is not enabled",
		})
		return
	}
	if cfg.AccountID == "" || cfg.GatewayID == "" {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "no_credentials",
			"message": "AI Gateway account ID or gateway ID is not configured",
		})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	probeURL := fmt.Sprintf(
		"https://gateway.ai.cloudflare.com/v1/%s/%s/openai/models",
		cfg.AccountID,
		cfg.GatewayID,
	)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, probeURL, nil)
	if err != nil {
		if test {
			w.WriteHeader(http.StatusServiceUnavailable)
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": err.Error(),
		})
		return
	}
	if cfg.Token != "" {
		req.Header.Set("cf-aig-authorization", "Bearer "+cfg.Token)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		if test {
			w.WriteHeader(http.StatusServiceUnavailable)
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": err.Error(),
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
		if test {
			w.WriteHeader(http.StatusServiceUnavailable)
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": msg,
		})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "ok",
		"message": "AI Gateway connection successful",
	})
}