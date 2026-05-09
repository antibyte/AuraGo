package server

import (
	"encoding/json"
	"net/http"
	"time"

	"aurago/internal/desktop"
)

func handleDesktopSettings(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireDesktopPermission(s, w, r, desktopScopeAdmin) {
			return
		}
		svc, hub, err := s.getDesktopService(r.Context())
		if err != nil {
			jsonError(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		switch r.Method {
		case http.MethodGet:
			payload, err := svc.Bootstrap(r.Context())
			if err != nil {
				jsonError(w, "Failed to load desktop settings", http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok", "settings": payload.Settings})
		case http.MethodPut:
			var body struct {
				Key      string            `json:"key"`
				Value    string            `json:"value"`
				Settings map[string]string `json:"settings"`
			}
			if err := decodeDesktopJSON(w, r, &body, desktopMediumJSONBodyLimit); err != nil {
				jsonError(w, "Invalid JSON", http.StatusBadRequest)
				return
			}
			if body.Settings == nil {
				body.Settings = map[string]string{body.Key: body.Value}
			}
			if err := svc.SetSettings(r.Context(), body.Settings, desktop.SourceUser); err != nil {
				jsonError(w, err.Error(), http.StatusBadRequest)
				return
			}
			payload, err := svc.Bootstrap(r.Context())
			if err != nil {
				jsonError(w, "Failed to load desktop settings", http.StatusInternalServerError)
				return
			}
			event := desktop.Event{Type: "desktop_changed", Payload: map[string]interface{}{"operation": "set_settings", "settings": body.Settings}, CreatedAt: time.Now().UTC()}
			broadcastDesktopEvent(s, hub, event)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok", "settings": payload.Settings})
		default:
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}
}
