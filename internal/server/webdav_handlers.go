package server

import (
	"encoding/json"
	"net/http"
	"strings"

	"aurago/internal/tools"
)

func registerWebDAVHandlers(mux *http.ServeMux, s *Server) {
	mux.HandleFunc("/api/webdav/status", handleWebDAVStatus(s))
	mux.HandleFunc("/api/webdav/test", handleWebDAVTest(s))
}

func handleWebDAVStatus(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		writeWebDAVProbeResult(w, s, false)
	}
}

func handleWebDAVTest(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		writeWebDAVProbeResult(w, s, true)
	}
}

func writeWebDAVProbeResult(w http.ResponseWriter, s *Server, test bool) {
	s.CfgMu.RLock()
	cfg := s.Cfg.WebDAV
	s.CfgMu.RUnlock()

	if !cfg.Enabled {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "disabled",
			"message": "WebDAV integration is not enabled",
		})
		return
	}
	if strings.TrimSpace(cfg.URL) == "" {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "no_url",
			"message": "WebDAV URL is not configured",
		})
		return
	}

	authType := strings.ToLower(strings.TrimSpace(cfg.AuthType))
	if authType == "bearer" {
		if cfg.Token == "" {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "no_credentials",
				"message": "WebDAV bearer token is not configured",
			})
			return
		}
	} else if cfg.Username == "" || cfg.Password == "" {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "no_credentials",
			"message": "WebDAV username or password is not configured",
		})
		return
	}

	davCfg := tools.WebDAVConfig{
		AuthType: cfg.AuthType,
		URL:      cfg.URL,
		Username: cfg.Username,
		Password: cfg.Password,
		Token:    cfg.Token,
		ReadOnly: cfg.ReadOnly,
	}
	raw := tools.WebDAVList(davCfg, "/")

	var payload struct {
		Status  string `json:"status"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		if test {
			w.WriteHeader(http.StatusServiceUnavailable)
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": "Failed to parse WebDAV response",
		})
		return
	}
	if payload.Status != "success" {
		msg := payload.Message
		if msg == "" {
			msg = "WebDAV connection failed"
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
		"message": "WebDAV connection successful",
	})
}