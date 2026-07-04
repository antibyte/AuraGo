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
		if !s.Cfg.Frigate.Enabled {
			writeFrigateTestError(w, http.StatusBadRequest, "Frigate integration is not enabled")
			return
		}
		if s.Cfg.Frigate.URL == "" {
			writeFrigateTestError(w, http.StatusBadRequest, "Frigate URL is not configured")
			return
		}

		raw := tools.FrigateStatus(tools.FrigateConfig{
			URL:           s.Cfg.Frigate.URL,
			APIToken:      s.Cfg.Frigate.APIToken,
			InternalPort:  s.Cfg.Frigate.InternalPort,
			Insecure:      s.Cfg.Frigate.Insecure,
			DefaultCamera: s.Cfg.Frigate.DefaultCamera,
			ReadOnly:      s.Cfg.Frigate.ReadOnly,
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
