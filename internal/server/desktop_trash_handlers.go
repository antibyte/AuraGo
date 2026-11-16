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
		moves, err := svc.TrashPathsWithReceipts(r.Context(), body.Paths, body.Restore, body.Resolutions)
		if len(moves) > 0 {
			eventMoves := make([]map[string]string, 0, len(moves))
			for _, move := range moves {
				eventMoves = append(eventMoves, map[string]string{"from": move.From, "path": move.Path})
			}
			broadcastDesktopEvent(s, hub, desktop.Event{Type: "desktop_changed", Payload: map[string]interface{}{"operation": "trash_move", "moves": eventMoves}, CreatedAt: time.Now().UTC()})
		}
		if err != nil {
			if errors.Is(err, desktop.ErrFileUndoUncertain) {
				broadcastDesktopEvent(s, hub, desktop.Event{Type: "desktop_changed", Payload: map[string]interface{}{"operation": "trash_move_uncertain"}, CreatedAt: time.Now().UTC()})
			}
			if len(moves) > 0 {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusConflict)
				body := map[string]interface{}{"status": "partial", "code": "partial_failure", "error": err.Error(), "moves": moves}
				if errors.Is(err, desktop.ErrFileUndoUncertain) {
					body["uncertain"] = true
				}
				_ = json.NewEncoder(w).Encode(body)
				return
			}
			if errors.Is(err, desktop.ErrFileUndoLimit) {
				writeDesktopUndoError(w, err)
				return
			}
			if errors.Is(err, desktop.ErrFileUndoUncertain) {
				writeDesktopUndoError(w, err)
				return
			}
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
