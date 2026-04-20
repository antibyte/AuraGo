package server

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"aurago/internal/config"
	"aurago/internal/tools"
)

func ensureBrowserAutomationReady(ctx context.Context, cfg *config.Config, logger interface {
	Info(string, ...any)
	Warn(string, ...any)
	Error(string, ...any)
}) map[string]interface{} {
	sidecarCfg, err := tools.ResolveBrowserAutomationSidecarConfig(cfg)
	if err != nil {
		return map[string]interface{}{"status": "error", "message": err.Error()}
	}
	if strings.EqualFold(cfg.BrowserAutomation.Mode, "sidecar") {
		tools.EnsureBrowserAutomationSidecarRunning(cfg.Docker.Host, sidecarCfg, logger)
	}

	deadline := time.NewTimer(20 * time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(750 * time.Millisecond)
	defer ticker.Stop()

	for {
		health := tools.BrowserAutomationHealth(ctx, cfg)
		if status, _ := health["status"].(string); status == "success" {
			return health
		}
		select {
		case <-ctx.Done():
			if msg, ok := tools.BrowserAutomationHealth(context.Background(), cfg)["message"].(string); ok && msg != "" {
				return map[string]interface{}{"status": "error", "message": msg}
			}
			return map[string]interface{}{"status": "error", "message": ctx.Err().Error()}
		case <-deadline.C:
			if msg, ok := health["message"].(string); ok && msg != "" {
				return map[string]interface{}{"status": "error", "message": msg}
			}
			return map[string]interface{}{"status": "error", "message": "browser automation sidecar did not become ready in time"}
		case <-ticker.C:
		}
	}
}

func handleBrowserAutomationStatus(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		s.CfgMu.RLock()
		cfg := *s.Cfg
		s.CfgMu.RUnlock()
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()
		json.NewEncoder(w).Encode(tools.BrowserAutomationHealth(ctx, &cfg))
	}
}

func handleBrowserAutomationTest(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		s.CfgMu.RLock()
		cfg := *s.Cfg
		s.CfgMu.RUnlock()
		if !cfg.BrowserAutomation.Enabled {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "error",
				"message": "Browser automation is disabled",
			})
			return
		}
		if !cfg.Tools.BrowserAutomation.Enabled {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "error",
				"message": "Browser automation tool is disabled",
			})
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 25*time.Second)
		defer cancel()
		json.NewEncoder(w).Encode(ensureBrowserAutomationReady(ctx, &cfg, s.Logger))
	}
}
