package server

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"aurago/internal/config"
	"aurago/internal/tools"
)

func uptimeKumaToolConfig(s *Server) tools.UptimeKumaConfig {
	return uptimeKumaToolConfigFrom(s.Cfg.UptimeKuma)
}

func uptimeKumaToolConfigFrom(u config.UptimeKumaConfig) tools.UptimeKumaConfig {
	return tools.UptimeKumaConfig{
		BaseURL:        u.BaseURL,
		APIKey:         u.APIKey,
		InsecureSSL:    u.InsecureSSL,
		RequestTimeout: u.RequestTimeout,
	}
}

// handleUptimeKumaStatus returns the current scrape status for the config UI.
func handleUptimeKumaStatus(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if !s.Cfg.UptimeKuma.Enabled {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "disabled",
				"message": "Uptime Kuma integration is not enabled",
			})
			return
		}
		if s.Cfg.UptimeKuma.BaseURL == "" {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "no_url",
				"message": "Uptime Kuma base URL is not configured",
			})
			return
		}
		if s.Cfg.UptimeKuma.APIKey == "" {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "no_api_key",
				"message": "Uptime Kuma API key is not configured",
			})
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), time.Duration(max(15, s.Cfg.UptimeKuma.RequestTimeout))*time.Second)
		defer cancel()
		snapshot, err := tools.FetchUptimeKumaSnapshot(ctx, uptimeKumaToolConfig(s), s.Logger)
		if err != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "error",
				"message": err.Error(),
			})
			return
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "ok",
			"data": map[string]interface{}{
				"summary":  snapshot.Summary,
				"monitors": snapshot.Monitors,
			},
		})
	}
}

// handleUptimeKumaTest verifies the configured metrics endpoint.
func handleUptimeKumaTest(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		draft, err := s.draftTestConfig(r)
		if err != nil {
			writeDraftTestConfigError(w, err)
			return
		}
		ukcfg := draft.UptimeKuma
		if ukcfg.BaseURL == "" {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "error",
				"message": "Uptime Kuma base URL is not configured",
			})
			return
		}
		if ukcfg.APIKey == "" {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "error",
				"message": "Uptime Kuma API key is not configured",
			})
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), time.Duration(max(15, ukcfg.RequestTimeout))*time.Second)
		defer cancel()
		snapshot, err := tools.FetchUptimeKumaSnapshot(ctx, uptimeKumaToolConfigFrom(ukcfg), s.Logger)
		if err != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "error",
				"message": err.Error(),
			})
			return
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "ok",
			"message": "Connection successful",
			"summary": snapshot.Summary,
		})
	}
}
