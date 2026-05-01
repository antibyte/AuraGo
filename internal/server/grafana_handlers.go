package server

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"aurago/internal/tools"
)

func grafanaToolConfig(s *Server) tools.GrafanaConfig {
	return tools.GrafanaConfig{
		BaseURL:        s.Cfg.Grafana.BaseURL,
		APIKey:         s.Cfg.Grafana.APIKey,
		InsecureSSL:    s.Cfg.Grafana.InsecureSSL,
		RequestTimeout: s.Cfg.Grafana.RequestTimeout,
	}
}

func handleGrafanaStatus(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if !s.Cfg.Grafana.Enabled {
			json.NewEncoder(w).Encode(map[string]interface{}{"status": "disabled", "message": "Grafana integration is not enabled"})
			return
		}
		if s.Cfg.Grafana.BaseURL == "" {
			json.NewEncoder(w).Encode(map[string]interface{}{"status": "no_url", "message": "Grafana base URL is not configured"})
			return
		}
		if s.Cfg.Grafana.APIKey == "" {
			json.NewEncoder(w).Encode(map[string]interface{}{"status": "no_api_key", "message": "Grafana API key is not configured"})
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), time.Duration(max(15, s.Cfg.Grafana.RequestTimeout))*time.Second)
		defer cancel()
		cfg := grafanaToolConfig(s)
		health, err := tools.FetchGrafanaHealth(ctx, cfg)
		if err != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{"status": "error", "message": err.Error()})
			return
		}

		dashboards, _ := tools.ListGrafanaDashboards(ctx, cfg, "")
		datasources, _ := tools.ListGrafanaDatasources(ctx, cfg)
		alerts, _ := tools.ListGrafanaAlerts(ctx, cfg)
		org, _ := tools.GetGrafanaOrg(ctx, cfg)

		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "ok",
			"data": map[string]interface{}{
				"health": health,
				"summary": map[string]interface{}{
					"dashboards":  len(dashboards),
					"datasources": len(datasources),
					"alerts":      len(alerts),
					"org":         org.Name,
				},
			},
		})
	}
}

func handleGrafanaTest(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if s.Cfg.Grafana.BaseURL == "" {
			json.NewEncoder(w).Encode(map[string]interface{}{"status": "error", "message": "Grafana base URL is not configured"})
			return
		}
		if s.Cfg.Grafana.APIKey == "" {
			json.NewEncoder(w).Encode(map[string]interface{}{"status": "error", "message": "Grafana API key is not configured"})
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), time.Duration(max(15, s.Cfg.Grafana.RequestTimeout))*time.Second)
		defer cancel()
		health, err := tools.FetchGrafanaHealth(ctx, grafanaToolConfig(s))
		if err != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{"status": "error", "message": err.Error()})
			return
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "ok",
			"message": "Connection successful",
			"health":  health,
		})
	}
}
