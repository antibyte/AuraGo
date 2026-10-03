package server

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"aurago/internal/desktop"
)

func handleDesktopArchive(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireDesktopPermission(s, w, r, desktopScopeWrite) {
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
			Paths []string `json:"paths"`
			Dest  string   `json:"dest"`
		}
		if err := decodeDesktopJSON(w, r, &body, 10*1024*1024); err != nil { // 10MB limit
			jsonError(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		if len(body.Paths) == 0 || body.Dest == "" {
			jsonError(w, "Missing paths or dest", http.StatusBadRequest)
			return
		}

		if err := svc.CreateArchive(r.Context(), body.Paths, body.Dest, desktop.SourceUser); err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}

		// Broadcast desktop changed event
		event := desktop.Event{
			Type:      "desktop_changed",
			Payload:   map[string]interface{}{"operation": "create_file", "path": body.Dest},
			CreatedAt: time.Now().UTC(),
		}
		broadcastDesktopEvent(s, hub, event)

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok"})
	}
}

func handleDesktopExtract(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireDesktopPermission(s, w, r, desktopScopeWrite) {
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
			Path string `json:"path"`
			Dest string `json:"dest"`
		}
		if err := decodeDesktopJSON(w, r, &body, desktopSmallJSONBodyLimit); err != nil {
			jsonError(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		if body.Path == "" || body.Dest == "" {
			jsonError(w, "Missing path or dest", http.StatusBadRequest)
			return
		}

		if err := svc.ExtractArchive(r.Context(), body.Path, body.Dest, desktop.SourceUser); err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}

		// Broadcast desktop changed event
		event := desktop.Event{
			Type:      "desktop_changed",
			Payload:   map[string]interface{}{"operation": "extract_zip", "path": body.Dest},
			CreatedAt: time.Now().UTC(),
		}
		broadcastDesktopEvent(s, hub, event)

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok"})
	}
}

func handleDesktopArchiveList(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireDesktopPermission(s, w, r, desktopScopeRead) {
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

		zipPath := r.URL.Query().Get("path")
		if zipPath == "" {
			jsonError(w, "Missing path parameter", http.StatusBadRequest)
			return
		}

		srcResolved, err := svc.ResolvePath(zipPath)
		if err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}

		reader, err := zip.OpenReader(srcResolved)
		if err != nil {
			jsonError(w, fmt.Sprintf("Failed to open zip file: %v", err), http.StatusBadRequest)
			return
		}
		defer reader.Close()

		type zipEntry struct {
			Name           string `json:"name"`
			Size           int64  `json:"size"`
			CompressedSize int64  `json:"compressed_size"`
			IsDir          bool   `json:"is_dir"`
			ModTime        string `json:"mod_time"`
		}

		entries := make([]zipEntry, 0, len(reader.File))
		for _, f := range reader.File {
			entries = append(entries, zipEntry{
				Name:           f.Name,
				Size:           int64(f.UncompressedSize64),
				CompressedSize: int64(f.CompressedSize64),
				IsDir:          f.FileInfo().IsDir(),
				ModTime:        f.Modified.Format(time.RFC3339),
			})
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"entries": entries})
	}
}

func handleDesktopArchiveEntry(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireDesktopPermission(s, w, r, desktopScopeRead) {
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
		zipPath := r.URL.Query().Get("path")
		entryName := r.URL.Query().Get("entry")
		if strings.TrimSpace(zipPath) == "" || strings.TrimSpace(entryName) == "" {
			jsonError(w, "Missing path or entry parameter", http.StatusBadRequest)
			return
		}
		entry, err := svc.ReadArchiveEntry(r.Context(), zipPath, entryName)
		if err != nil {
			writeDesktopArchiveEntryError(w, err)
			return
		}
		mimeType := entry.MIMEType
		if mimeType == "" {
			mimeType = desktop.MIMETypeForName(entry.Name)
		}
		if mimeType == "" {
			mimeType = "application/octet-stream"
		}
		w.Header().Set("Content-Type", mimeType)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Cache-Control", "private, max-age=60")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`inline; filename="%s"`, sanitizeContentDisposition(entry.Name)))
		http.ServeContent(w, r, entry.Name, entry.ModTime, bytes.NewReader(entry.Data))
	}
}

func writeDesktopArchiveEntryError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, desktop.ErrArchiveEntryNotFound):
		jsonError(w, err.Error(), http.StatusNotFound)
	case errors.Is(err, desktop.ErrArchiveEntryTooLarge):
		jsonError(w, err.Error(), http.StatusRequestEntityTooLarge)
	case errors.Is(err, desktop.ErrArchiveEntryNotPreviewable):
		jsonError(w, err.Error(), http.StatusUnsupportedMediaType)
	default:
		jsonError(w, err.Error(), http.StatusBadRequest)
	}
}

func handleDesktopBatchRename(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireDesktopPermission(s, w, r, desktopScopeWrite) {
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
			Operations []struct {
				OldPath string `json:"old_path"`
				NewName string `json:"new_name"`
			} `json:"operations"`
		}
		if err := decodeDesktopJSON(w, r, &body, 10*1024*1024); err != nil { // 10MB limit
			jsonError(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		for _, op := range body.Operations {
			if op.OldPath == "" || op.NewName == "" {
				jsonError(w, "Invalid rename operation parameters", http.StatusBadRequest)
				return
			}

			cleanName := filepath.Base(filepath.Clean(op.NewName))
			if cleanName == "." || cleanName == ".." || cleanName == "/" || cleanName == "" {
				jsonError(w, "Invalid destination filename", http.StatusBadRequest)
				return
			}

			dir := filepath.Dir(op.OldPath)
			newPath := filepath.ToSlash(filepath.Join(dir, cleanName))

			err := svc.MovePath(r.Context(), op.OldPath, newPath, desktop.SourceUser)
			if err != nil {
				jsonError(w, fmt.Sprintf("Failed to rename %s to %s: %v", op.OldPath, cleanName, err), http.StatusBadRequest)
				return
			}

			event := desktop.Event{
				Type:      "desktop_changed",
				Payload:   map[string]interface{}{"operation": "move_path", "old_path": op.OldPath, "new_path": newPath},
				CreatedAt: time.Now().UTC(),
			}
			broadcastDesktopEvent(s, hub, event)
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok"})
	}
}
