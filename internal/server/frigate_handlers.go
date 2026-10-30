package server

import (
	"encoding/json"
	"net/http"

	"aurago/internal/tools"
)

func writeFrigateTestError(w http.ResponseWriter, status int, message string) {
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": "error", "message": message})
}

func handleFrigateTest(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		// Optional {"draft": ...} tests unsaved values (nothing is persisted).
		cfg, err := s.draftTestConfig(r)
		if err != nil {
			writeDraftTestConfigError(w, err)
			return
		}
		if !cfg.Frigate.Enabled {
			writeFrigateTestError(w, http.StatusBadRequest, "Frigate integration is not enabled")
			return
		}
		if cfg.Frigate.URL == "" {
			writeFrigateTestError(w, http.StatusBadRequest, "Frigate URL is not configured")
			return
		}

		raw := tools.FrigateStatus(r.Context(), tools.FrigateConfig{
			URL:           cfg.Frigate.URL,
			APIToken:      cfg.Frigate.APIToken,
			InternalPort:  cfg.Frigate.InternalPort,
			Insecure:      cfg.Frigate.Insecure,
			DefaultCamera: cfg.Frigate.DefaultCamera,
			ReadOnly:      cfg.Frigate.ReadOnly,
		})
		var data map[string]interface{}
		if err := json.Unmarshal([]byte(raw), &data); err != nil {
			writeFrigateTestError(w, http.StatusBadGateway, "Frigate returned an invalid health response")
			return
		}
		if data["status"] == "error" {
			w.WriteHeader(http.StatusBadGateway)
			json.NewEncoder(w).Encode(data)
			return
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "ok",
			"message": "Connection successful",
			"stats":   data,
		})
	}
}
