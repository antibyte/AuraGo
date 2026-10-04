package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"aurago/internal/desktop"
)

func handleDesktopTrash(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireDesktopOperation(s, w, r, desktopScopeWrite, desktopWrite) {
			return
		}
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var body struct {
			Paths       []string                           `json:"paths"`
			Restore     bool                               `json:"restore"`
			Resolutions map[string]desktop.TrashResolution `json:"resolutions"`
		}
		if err := decodeDesktopJSON(w, r, &body, desktopSmallJSONBodyLimit); err != nil {
			jsonError(w, "Invalid JSON", http.StatusBadRequest)
			return
		}
		svc, hub, err := s.getDesktopService(r.Context())
		if err != nil {
			jsonError(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		moves, err := svc.TrashPaths(r.Context(), body.Paths, body.Restore, body.Resolutions)
		if len(moves) > 0 {
			broadcastDesktopEvent(s, hub, desktop.Event{Type: "desktop_changed", Payload: map[string]interface{}{"operation": "trash_move", "moves": moves}, CreatedAt: time.Now().UTC()})
		}
		if err != nil {
			var conflict *desktop.PathConflict
			if errors.As(err, &conflict) {
				code := "file_conflict"
				if conflict.Directory {
					code = "directory_conflict"
				}
				writeDesktopFileError(w, &desktopFileConflict{Code: code, Path: conflict.Path, Source: conflict.Source, Version: conflict.Version, Status: http.StatusPreconditionFailed})
			} else {
				jsonError(w, err.Error(), http.StatusBadRequest)
			}
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok", "moves": moves})
	}
}
