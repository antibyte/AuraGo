package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"aurago/internal/desktop"
)

func handleDesktopFileUndo(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireDesktopOperation(s, w, r, desktopScopeWrite, desktopWrite) {
			return
		}
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		svc, hub, err := s.getDesktopService(r.Context())
		if err != nil {
			jsonError(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		var body struct {
			Receipt string `json:"receipt"`
		}
		if err := decodeDesktopJSON(w, r, &body, desktopSmallJSONBodyLimit); err != nil || strings.TrimSpace(body.Receipt) == "" {
			jsonError(w, "Invalid file undo receipt", http.StatusBadRequest)
			return
		}
		result, err := svc.UndoFileMutation(r.Context(), body.Receipt)
		if err != nil {
			writeDesktopUndoError(w, err)
			return
		}
		broadcastDesktopEvent(s, hub, desktop.Event{
			Type:      "desktop_changed",
			Payload:   map[string]interface{}{"operation": "file_undo", "from": result.From, "path": result.Path},
			CreatedAt: time.Now().UTC(),
		})
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "ok", "from": result.From, "path": result.Path, "undo_receipt": result.UndoReceipt,
		})
	}
}

func writeDesktopUndoError(w http.ResponseWriter, err error) {
	var pathConflict *desktop.PathConflict
	if errors.As(err, &pathConflict) {
		code := "file_conflict"
		if pathConflict.Directory {
			code = "directory_conflict"
		}
		writeDesktopFileError(w, &desktopFileConflict{
			Code: code, Path: pathConflict.Path, Source: pathConflict.Source,
			Version: pathConflict.Version, Status: http.StatusPreconditionFailed,
		})
		return
	}
	switch {
	case errors.Is(err, desktop.ErrFileUndoUnavailable):
		writeDesktopUndoFailure(w, http.StatusGone, "file_undo_unavailable")
	case errors.Is(err, desktop.ErrFileUndoLimit):
		writeDesktopUndoFailure(w, http.StatusRequestEntityTooLarge, "file_undo_limit")
	case errors.Is(err, desktop.ErrFileUndoUncertain):
		writeDesktopUndoFailure(w, http.StatusBadGateway, "file_undo_uncertain")
	case strings.Contains(strings.ToLower(err.Error()), "read-only"):
		writeDesktopUndoFailure(w, http.StatusLocked, "desktop_readonly")
	default:
		writeDesktopFileError(w, err)
	}
}

func writeDesktopUndoFailure(w http.ResponseWriter, status int, code string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "error", "error": code, "code": code})
}
