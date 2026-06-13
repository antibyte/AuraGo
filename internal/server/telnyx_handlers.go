package server

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"aurago/internal/telnyx"
)

func registerTelnyxHandlers(mux *http.ServeMux, s *Server) {
	mux.HandleFunc("/api/telnyx/status", handleTelnyxStatus(s))
	mux.HandleFunc("/api/telnyx/test", handleTelnyxTest(s))
}

func handleTelnyxStatus(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		writeTelnyxProbeResult(w, s, false)
	}
}

func handleTelnyxTest(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		writeTelnyxProbeResult(w, s, true)
	}
}

func writeTelnyxProbeResult(w http.ResponseWriter, s *Server, test bool) {
	s.CfgMu.RLock()
	cfg := s.Cfg.Telnyx
	s.CfgMu.RUnlock()

	if !cfg.Enabled {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "disabled",
			"message": "Telnyx integration is not enabled",
		})
		return
	}
	if cfg.APIKey == "" {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "no_credentials",
			"message": "Telnyx API key is not configured",
		})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client := telnyx.NewClient(cfg.APIKey, s.Logger)
	balance, err := client.GetBalance(ctx)
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

	out := map[string]interface{}{
		"status":  "ok",
		"message": "Telnyx connection successful",
	}
	if balance != nil && balance.Data.Balance != "" {
		out["balance"] = balance.Data.Balance
		out["currency"] = balance.Data.Currency
	}
	json.NewEncoder(w).Encode(out)
}