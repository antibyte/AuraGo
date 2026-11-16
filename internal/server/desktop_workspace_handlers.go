package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"aurago/internal/desktop"
)

func handleDesktopWorkspaces(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		requiredScope := desktopScopeRead
		if r.Method == http.MethodPost {
			requiredScope = desktopScopeAdmin
		}
		if !authenticateDesktopPermission(s, w, r, requiredScope) {
			return
		}
		if r.Method == http.MethodPost {
			s.CfgMu.RLock()
			readOnly := s.Cfg != nil && s.Cfg.VirtualDesktop.ReadOnly
			s.CfgMu.RUnlock()
			if readOnly {
				writeDesktopWorkspaceError(w, desktop.ErrWorkspaceReadOnly, desktop.WorkspaceState{})
				return
			}
		}
		if !checkDesktopOperation(s, w, r, desktopRequestOperation(r)) {
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		svc, hub, err := s.getDesktopService(r.Context())
		if err != nil {
			jsonError(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		if r.Method == http.MethodGet {
			state, err := svc.GetWorkspaceState(r.Context())
			if err != nil {
				jsonError(w, "Failed to load desktop workspaces", http.StatusInternalServerError)
				return
			}
			writeDesktopWorkspaceState(w, http.StatusOK, state)
			return
		}

		var mutation desktop.WorkspaceMutation
		if err := decodeDesktopJSON(w, r, &mutation, desktopSmallJSONBodyLimit); err != nil {
			jsonError(w, "Invalid workspace request", http.StatusBadRequest)
			return
		}
		state, err := svc.MutateWorkspace(r.Context(), mutation, desktop.SourceUser)
		if err != nil {
			writeDesktopWorkspaceError(w, err, state)
			return
		}
		broadcastDesktopEvent(s, hub, desktop.Event{
			Type:      "desktop_changed",
			Payload:   map[string]interface{}{"operation": "workspace_layout", "revision": state.Layout.Revision},
			CreatedAt: time.Now().UTC(),
		})
		writeDesktopWorkspaceState(w, http.StatusOK, state)
	}
}

func writeDesktopWorkspaceState(w http.ResponseWriter, status int, state desktop.WorkspaceState) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "ok", "layout": state.Layout, "settings": state.Settings,
	})
}

func writeDesktopWorkspaceError(w http.ResponseWriter, err error, state desktop.WorkspaceState) {
	status := http.StatusInternalServerError
	code := "workspace_unavailable"
	var wallpaperErr *desktop.WallpaperError
	switch {
	case errors.Is(err, desktop.ErrWorkspaceRevisionConflict):
		status, code = http.StatusConflict, "workspace_revision_conflict"
	case errors.Is(err, desktop.ErrWorkspaceRevisionRequired):
		status, code = http.StatusConflict, "workspace_revision_required"
	case errors.Is(err, desktop.ErrWorkspaceLimit):
		status, code = http.StatusConflict, "workspace_limit"
	case errors.Is(err, desktop.ErrWorkspaceNotFound):
		status, code = http.StatusNotFound, "workspace_not_found"
	case errors.Is(err, desktop.ErrWorkspaceReadOnly):
		status, code = http.StatusLocked, "desktop_readonly"
	case errors.Is(err, desktop.ErrWorkspaceInvalid):
		status, code = http.StatusBadRequest, "workspace_invalid"
	case errors.Is(err, desktop.ErrWorkspaceStateInvalid):
		status, code = http.StatusInternalServerError, "workspace_state_invalid"
	case errors.As(err, &wallpaperErr):
		code = wallpaperErr.Code
		switch wallpaperErr.Code {
		case "wallpaper_invalid_path":
			status = http.StatusBadRequest
		case "wallpaper_not_found":
			status = http.StatusNotFound
		case "wallpaper_too_large", "wallpaper_dimensions_exceeded", "wallpaper_pixel_limit":
			status = http.StatusRequestEntityTooLarge
		case "wallpaper_unsupported_format", "wallpaper_format_mismatch", "wallpaper_invalid_image":
			status = http.StatusUnsupportedMediaType
		case "wallpaper_changed":
			status = http.StatusConflict
		default:
			status = http.StatusServiceUnavailable
		}
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	body := map[string]interface{}{"status": "error", "code": code, "error": code}
	if state.Layout.Version != 0 {
		body["state"] = state
	}
	_ = json.NewEncoder(w).Encode(body)
}
