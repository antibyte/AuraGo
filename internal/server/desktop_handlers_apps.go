package server

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"aurago/internal/desktop"
)

func handleDesktopApps(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		requiredScope := desktopScopeAdmin
		if r.Method == http.MethodPatch {
			requiredScope = desktopScopeWrite
		}
		if !requireDesktopPermission(s, w, r, requiredScope) {
			return
		}
		svc, hub, err := s.getDesktopService(r.Context())
		if err != nil {
			jsonError(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		if r.Method == http.MethodDelete {
			id := r.URL.Query().Get("id")
			if err := svc.DeleteApp(r.Context(), id, desktop.SourceUser); err != nil {
				status := http.StatusBadRequest
				if strings.Contains(err.Error(), "built-in") {
					status = http.StatusForbidden
				}
				jsonError(w, err.Error(), status)
				return
			}
			event := desktop.Event{Type: "desktop_changed", Payload: map[string]interface{}{"operation": "delete_app", "app_id": id}, CreatedAt: time.Now().UTC()}
			broadcastDesktopEvent(s, hub, event)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok"})
			return
		}
		if r.Method == http.MethodPatch {
			id := r.URL.Query().Get("id")
			var body struct {
				DockVisible  *bool `json:"dock_visible"`
				StartVisible *bool `json:"start_visible"`
			}
			if err := decodeDesktopJSON(w, r, &body, desktopSmallJSONBodyLimit); err != nil {
				jsonError(w, "Invalid JSON", http.StatusBadRequest)
				return
			}
			if body.DockVisible == nil && body.StartVisible == nil {
				jsonError(w, "dock_visible or start_visible field is required", http.StatusBadRequest)
				return
			}
			if err := svc.SetAppVisibility(r.Context(), id, body.DockVisible, body.StartVisible, desktop.SourceUser); err != nil {
				jsonError(w, err.Error(), http.StatusBadRequest)
				return
			}
			payload := map[string]interface{}{"operation": "set_app_visibility", "app_id": id}
			if body.DockVisible != nil {
				payload["dock_visible"] = *body.DockVisible
			}
			if body.StartVisible != nil {
				payload["start_visible"] = *body.StartVisible
			}
			event := desktop.Event{Type: "desktop_changed", Payload: payload, CreatedAt: time.Now().UTC()}
			broadcastDesktopEvent(s, hub, event)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok"})
			return
		}
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var body struct {
			Manifest desktop.AppManifest `json:"manifest"`
			Files    map[string]string   `json:"files"`
		}
		if err := decodeDesktopJSON(w, r, &body, desktopLargeJSONBodyLimit); err != nil {
			jsonError(w, "Invalid JSON", http.StatusBadRequest)
			return
		}
		if err := svc.InstallApp(r.Context(), body.Manifest, body.Files, desktop.SourceUser); err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		event := desktop.Event{Type: "desktop_changed", Payload: map[string]interface{}{"operation": "install_app", "app_id": body.Manifest.ID}, CreatedAt: time.Now().UTC()}
		broadcastDesktopEvent(s, hub, event)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok"})
	}
}
