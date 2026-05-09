package server

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"aurago/internal/desktop"
)

func handleDesktopWidgets(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireDesktopPermission(s, w, r, desktopScopeWrite) {
			return
		}
		svc, hub, err := s.getDesktopService(r.Context())
		if err != nil {
			jsonError(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		if r.Method == http.MethodGet {
			allWidgets, err := svc.ListAllWidgets(r.Context())
			if err != nil {
				jsonError(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(allWidgets)
			return
		}
		if r.Method == http.MethodDelete {
			id := r.URL.Query().Get("id")
			if err := svc.DeleteWidget(r.Context(), id, desktop.SourceUser); err != nil {
				status := http.StatusBadRequest
				if strings.Contains(err.Error(), "built-in") {
					status = http.StatusForbidden
				}
				jsonError(w, err.Error(), status)
				return
			}
			event := desktop.Event{Type: "desktop_changed", Payload: map[string]interface{}{"operation": "delete_widget", "widget_id": id}, CreatedAt: time.Now().UTC()}
			broadcastDesktopEvent(s, hub, event)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok"})
			return
		}
		if r.Method == http.MethodPatch {
			id := r.URL.Query().Get("id")
			var body struct {
				Visible *bool `json:"visible"`
			}
			if err := decodeDesktopJSON(w, r, &body, desktopSmallJSONBodyLimit); err != nil {
				jsonError(w, "Invalid JSON", http.StatusBadRequest)
				return
			}
			if body.Visible == nil {
				jsonError(w, "visible field is required", http.StatusBadRequest)
				return
			}
			if err := svc.SetWidgetVisible(r.Context(), id, *body.Visible, desktop.SourceUser); err != nil {
				jsonError(w, err.Error(), http.StatusBadRequest)
				return
			}
			event := desktop.Event{Type: "desktop_changed", Payload: map[string]interface{}{"operation": "set_widget_visible", "widget_id": id, "visible": *body.Visible}, CreatedAt: time.Now().UTC()}
			broadcastDesktopEvent(s, hub, event)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok"})
			return
		}
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var widget desktop.Widget
		if err := decodeDesktopJSON(w, r, &widget, desktopMediumJSONBodyLimit); err != nil {
			jsonError(w, "Invalid JSON", http.StatusBadRequest)
			return
		}
		if err := svc.UpsertWidget(r.Context(), widget, desktop.SourceUser); err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		event := desktop.Event{Type: "desktop_changed", Payload: map[string]interface{}{"operation": "upsert_widget", "widget_id": widget.ID}, CreatedAt: time.Now().UTC()}
		broadcastDesktopEvent(s, hub, event)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok"})
	}
}
