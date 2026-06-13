package server

import (
	"encoding/json"
	"net/http"
	"strings"

	"aurago/internal/tools"
)

func registerKoofrHandlers(mux *http.ServeMux, s *Server) {
	mux.HandleFunc("/api/koofr/status", handleKoofrStatus(s))
	mux.HandleFunc("/api/koofr/test", handleKoofrTest(s))
}

func handleKoofrStatus(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		writeKoofrProbeResult(w, s, false)
	}
}

func handleKoofrTest(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		writeKoofrProbeResult(w, s, true)
	}
}

func writeKoofrProbeResult(w http.ResponseWriter, s *Server, test bool) {
	s.CfgMu.RLock()
	cfg := s.Cfg.Koofr
	s.CfgMu.RUnlock()

	if !cfg.Enabled {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "disabled",
			"message": "Koofr integration is not enabled",
		})
		return
	}
	if cfg.Username == "" || cfg.AppPassword == "" {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "no_credentials",
			"message": "Koofr username or app password is not configured",
		})
		return
	}

	koofrCfg := tools.KoofrConfig{
		BaseURL:     cfg.BaseURL,
		Username:    cfg.Username,
		AppPassword: cfg.AppPassword,
		ReadOnly:    cfg.ReadOnly,
	}
	raw := tools.ExecuteKoofr(koofrCfg, "list", "/", "", "", "", "", "")
	raw = strings.TrimPrefix(raw, "Tool Output: ")

	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		if test {
			w.WriteHeader(http.StatusServiceUnavailable)
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": "Failed to parse Koofr response",
		})
		return
	}

	status, _ := payload["status"].(string)
	if status != "success" {
		msg, _ := payload["message"].(string)
		if msg == "" {
			msg = "Koofr connection failed"
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
		"message": "Koofr connection successful",
	})
}