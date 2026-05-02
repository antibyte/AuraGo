package server

import (
	"encoding/json"
	"net/http"

	"aurago/internal/tools"
)

func handleFrigateTest(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if !s.Cfg.Frigate.Enabled {
			json.NewEncoder(w).Encode(map[string]interface{}{"status": "error", "message": "Frigate integration is not enabled"})
			return
		}
		if s.Cfg.Frigate.URL == "" {
			json.NewEncoder(w).Encode(map[string]interface{}{"status": "error", "message": "Frigate URL is not configured"})
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
			json.NewEncoder(w).Encode(map[string]interface{}{"status": "error", "message": "Frigate returned an invalid health response"})
			return
		}
		if data["status"] == "error" {
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
