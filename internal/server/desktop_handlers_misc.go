package server

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"aurago/internal/desktop"
)

func handleDesktopShortcuts(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireDesktopPermission(s, w, r, desktopScopeWrite) {
			return
		}
		svc, hub, err := s.getDesktopService(r.Context())
		if err != nil {
			jsonError(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		switch r.Method {
		case http.MethodPost:
			var body struct {
				AppID string `json:"app_id"`
			}
			if err := decodeDesktopJSON(w, r, &body, desktopSmallJSONBodyLimit); err != nil {
				jsonError(w, "Invalid JSON", http.StatusBadRequest)
				return
			}
			if err := svc.AddDesktopAppShortcut(r.Context(), body.AppID, desktop.SourceUser); err != nil {
				jsonError(w, err.Error(), http.StatusBadRequest)
				return
			}
			event := desktop.Event{Type: "desktop_changed", Payload: map[string]interface{}{"operation": "add_shortcut", "app_id": body.AppID}, CreatedAt: time.Now().UTC()}
			broadcastDesktopEvent(s, hub, event)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok"})
		case http.MethodDelete:
			id := r.URL.Query().Get("id")
			if err := svc.RemoveDesktopShortcut(r.Context(), id, desktop.SourceUser); err != nil {
				jsonError(w, err.Error(), http.StatusBadRequest)
				return
			}
			event := desktop.Event{Type: "desktop_changed", Payload: map[string]interface{}{"operation": "remove_shortcut", "shortcut_id": id}, CreatedAt: time.Now().UTC()}
			broadcastDesktopEvent(s, hub, event)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok"})
		default:
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

func handleDesktopEmbedToken(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireDesktopPermission(s, w, r, desktopScopeAdmin) {
			return
		}
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		svc, _, err := s.getDesktopService(r.Context())
		if err != nil {
			jsonError(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		rawPath := strings.TrimSpace(r.URL.Query().Get("path"))
		if rawPath == "" {
			jsonError(w, "path is required", http.StatusBadRequest)
			return
		}
		if _, err := svc.ResolvePath(rawPath); err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		normalizedPath, err := normalizeDesktopEmbedPath(rawPath)
		if err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		s.CfgMu.RLock()
		authEnabled := s.Cfg.Auth.Enabled
		secret := s.Cfg.Auth.SessionSecret
		s.CfgMu.RUnlock()
		now := time.Now().UTC()
		token := ""
		if authEnabled {
			var err error
			token, err = issueDesktopEmbedToken(secret, normalizedPath, now)
			if err != nil {
				jsonError(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"status":     "ok",
			"token":      token,
			"expires_at": now.Add(desktopEmbedTokenTTL).Format(time.RFC3339Nano),
		})
	}
}
