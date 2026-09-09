package server

import (
	"aurago/internal/desktop"
	"encoding/json"
	"errors"
	"net/http"
	"time"
)

func handleDesktopPlant(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		read := r.URL.Path == "/api/desktop/plant" && r.Method == http.MethodGet
		write := r.URL.Path == "/api/desktop/plant/actions" && r.Method == http.MethodPost
		if !read && !write {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		scope := desktopScopeRead
		if write {
			scope = desktopScopeWrite
		}
		if !requireDesktopPermission(s, w, r, scope) {
			return
		}
		svc, hub, err := s.getDesktopService(r.Context())
		if err != nil {
			jsonError(w, "Desktop unavailable", http.StatusServiceUnavailable)
			return
		}
		if write && svc.Config().ReadOnly {
			jsonError(w, "desktop_read_only", http.StatusForbidden)
			return
		}
		var snapshot desktop.PlantSnapshot
		now := time.Now().UTC()
		if read {
			snapshot, err = svc.Plant(r.Context(), now)
		} else {
			var action desktop.PlantAction
			if err := decodeDesktopJSON(w, r, &action, desktopSmallJSONBodyLimit); err != nil {
				jsonError(w, "Invalid JSON", http.StatusBadRequest)
				return
			}
			snapshot, err = svc.ApplyPlantAction(r.Context(), action, now)
			if err == nil {
				broadcastDesktopEvent(s, hub, desktop.Event{Type: "plant_changed", Payload: map[string]interface{}{"revision": snapshot.Plant.Revision}, CreatedAt: now})
			}
		}
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Type", "application/json")
		if errors.Is(err, desktop.ErrPlantConflict) {
			w.WriteHeader(http.StatusConflict)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": "plant_revision_conflict", "snapshot": snapshot})
			return
		}
		if err != nil {
			status := http.StatusInternalServerError
			message := "plant_unavailable"
			if errors.Is(err, desktop.ErrPlantAction) {
				status = http.StatusBadRequest
				message = "invalid_plant_action"
			}
			jsonError(w, message, status)
			return
		}
		_ = json.NewEncoder(w).Encode(snapshot)
	}
}
