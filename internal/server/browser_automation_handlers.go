package server

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"aurago/internal/tools"
)

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
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()
		json.NewEncoder(w).Encode(tools.BrowserAutomationHealth(ctx, &cfg))
	}
}
