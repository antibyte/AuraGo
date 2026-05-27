package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"aurago/internal/desktop"
)

func handleDesktopBootstrap(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireDesktopPermission(s, w, r, desktopScopeRead) {
			return
		}
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		svc, _, err := s.getDesktopService(r.Context())
		var payload desktop.BootstrapPayload
		if err != nil {
			payload = s.disabledDesktopBootstrap()
		} else if payload, err = svc.Bootstrap(r.Context()); err != nil {
			jsonError(w, "Failed to load desktop", http.StatusInternalServerError)
			return
		}
		s.enrichDesktopBootstrap(&payload)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(payload)
	}
}

func handleDesktopFiles(s *Server) http.HandlerFunc {
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
		query := r.URL.Query()
		if query.Get("recursive") == "true" {
			offset := parseDesktopFilesInt(query.Get("offset"), 0)
			limit := parseDesktopFilesInt(query.Get("limit"), 500)
			if limit > 1000 {
				limit = 1000
			}
			files, hasMore, err := svc.ListFilesRecursive(r.Context(), query.Get("path"), offset, limit)
			if err != nil {
				jsonError(w, err.Error(), http.StatusBadRequest)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok", "files": files, "offset": offset, "limit": limit, "has_more": hasMore})
			return
		}
		files, err := svc.ListFiles(r.Context(), query.Get("path"))
		if err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok", "files": files})
	}
}

func parseDesktopFilesInt(raw string, fallback int) int {
	if strings.TrimSpace(raw) == "" {
		return fallback
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 0 {
		return fallback
	}
	return n
}

func handleDesktopFile(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireDesktopPermission(s, w, r, desktopMethodScope(r.Method)) {
			return
		}
		svc, hub, err := s.getDesktopService(r.Context())
		if err != nil {
			jsonError(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		switch r.Method {
		case http.MethodGet:
			content, entry, err := svc.ReadFile(r.Context(), r.URL.Query().Get("path"))
			if err != nil {
				if os.IsPermission(err) {
					jsonError(w, err.Error(), http.StatusForbidden)
				} else {
					jsonError(w, err.Error(), http.StatusBadRequest)
				}
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok", "entry": entry, "content": content})
		case http.MethodPut:
			var body struct {
				Path    string `json:"path"`
				Content string `json:"content"`
			}
			if err := decodeDesktopJSON(w, r, &body, desktopFileJSONBodyLimit(svc)); err != nil {
				jsonError(w, "Invalid JSON", http.StatusBadRequest)
				return
			}
			if body.Path == "" {
				body.Path = r.URL.Query().Get("path")
			}
			if err := svc.WriteFile(r.Context(), body.Path, body.Content, desktop.SourceUser); err != nil {
				jsonError(w, err.Error(), http.StatusBadRequest)
				return
			}
			event := desktop.Event{Type: "desktop_changed", Payload: map[string]interface{}{"operation": "write_file", "path": body.Path}, CreatedAt: time.Now().UTC()}
			broadcastDesktopEvent(s, hub, event)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok"})
		case http.MethodPatch:
			var body struct {
				OldPath string `json:"old_path"`
				NewPath string `json:"new_path"`
			}
			if err := decodeDesktopJSON(w, r, &body, desktopSmallJSONBodyLimit); err != nil {
				jsonError(w, "Invalid JSON", http.StatusBadRequest)
				return
			}
			if err := svc.MovePath(r.Context(), body.OldPath, body.NewPath, desktop.SourceUser); err != nil {
				jsonError(w, err.Error(), http.StatusBadRequest)
				return
			}
			event := desktop.Event{Type: "desktop_changed", Payload: map[string]interface{}{"operation": "move_path", "old_path": body.OldPath, "new_path": body.NewPath}, CreatedAt: time.Now().UTC()}
			broadcastDesktopEvent(s, hub, event)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok"})
		case http.MethodDelete:
			path := r.URL.Query().Get("path")
			if err := svc.DeletePath(r.Context(), path, desktop.SourceUser); err != nil {
				jsonError(w, err.Error(), http.StatusBadRequest)
				return
			}
			event := desktop.Event{Type: "desktop_changed", Payload: map[string]interface{}{"operation": "delete_path", "path": path}, CreatedAt: time.Now().UTC()}
			broadcastDesktopEvent(s, hub, event)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok"})
		default:
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

func handleDesktopDirectory(s *Server) http.HandlerFunc {
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
		}
		if err := decodeDesktopJSON(w, r, &body, desktopSmallJSONBodyLimit); err != nil {
			jsonError(w, "Invalid JSON", http.StatusBadRequest)
			return
		}
		if err := svc.CreateDirectory(r.Context(), body.Path, desktop.SourceUser); err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		event := desktop.Event{Type: "desktop_changed", Payload: map[string]interface{}{"operation": "create_directory", "path": body.Path}, CreatedAt: time.Now().UTC()}
		broadcastDesktopEvent(s, hub, event)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok"})
	}
}

func handleDesktopCopy(s *Server) http.HandlerFunc {
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
			SourcePath string `json:"source_path"`
			DestPath   string `json:"dest_path"`
		}
		if err := decodeDesktopJSON(w, r, &body, desktopSmallJSONBodyLimit); err != nil {
			jsonError(w, "Invalid JSON", http.StatusBadRequest)
			return
		}
		if body.SourcePath == "" || body.DestPath == "" {
			jsonError(w, "source_path and dest_path are required", http.StatusBadRequest)
			return
		}
		if err := svc.CopyPath(r.Context(), body.SourcePath, body.DestPath, desktop.SourceUser); err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		event := desktop.Event{Type: "desktop_changed", Payload: map[string]interface{}{"operation": "copy_path", "source_path": body.SourcePath, "dest_path": body.DestPath}, CreatedAt: time.Now().UTC()}
		broadcastDesktopEvent(s, hub, event)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok"})
	}
}

func handleDesktopPreview(s *Server) http.HandlerFunc {
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
		file, entry, mimeType, err := svc.OpenPreviewFile(r.Context(), r.URL.Query().Get("path"))
		if err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		defer file.Close()
		if !isDesktopPreviewImageMIME(mimeType) {
			jsonError(w, "desktop preview only supports image files", http.StatusUnsupportedMediaType)
			return
		}
		maxSize := int64(svc.Config().MaxFileSizeMB) * 1024 * 1024
		if maxSize <= 0 {
			maxSize = 50 * 1024 * 1024
		}
		if entry.Size > maxSize {
			jsonError(w, "desktop preview file exceeds max size", http.StatusRequestEntityTooLarge)
			return
		}
		w.Header().Set("Content-Type", mimeType)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Cache-Control", "private, max-age=120")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`inline; filename="%s"`, sanitizeContentDisposition(entry.Name)))
		http.ServeContent(w, r, entry.Name, entry.ModTime, file)
	}
}

func isDesktopPreviewImageMIME(mimeType string) bool {
	switch strings.ToLower(strings.TrimSpace(mimeType)) {
	case "image/avif", "image/bmp", "image/gif", "image/jpeg", "image/png", "image/webp":
		return true
	default:
		return false
	}
}

// sanitizeUploadFilename sanitizes a filename from a multipart upload to prevent path traversal.
// It extracts the base name, removes null bytes, strips remaining path separators,
// and falls back to a safe default if the result is empty or only dots.
func sanitizeUploadFilename(name string) string {
	// Extract base name only to strip any directory components
	base := filepath.Base(name)

	// Remove null bytes
	base = strings.ReplaceAll(base, "\x00", "")

	// Replace any remaining path separators
	base = strings.ReplaceAll(base, "/", "")
	base = strings.ReplaceAll(base, "\\", "")

	// Validate the result is not empty or just dots
	if base == "" || base == "." || base == ".." {
		return "unnamed_file"
	}

	return base
}

func handleDesktopUpload(s *Server) http.HandlerFunc {
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
		maxSize := int64(svc.Config().MaxFileSizeMB) * 1024 * 1024
		if maxSize <= 0 {
			maxSize = 50 * 1024 * 1024
		}
		r.Body = http.MaxBytesReader(w, r.Body, maxSize+1024)
		if err := r.ParseMultipartForm(maxSize); err != nil {
			jsonError(w, "File too large or invalid form data", http.StatusBadRequest)
			return
		}
		file, header, err := r.FormFile("file")
		if err != nil {
			jsonError(w, "Missing file field", http.StatusBadRequest)
			return
		}
		defer file.Close()
		destDir := r.FormValue("path")
		content, readErr := io.ReadAll(file)
		if readErr != nil {
			jsonError(w, "Failed to read upload", http.StatusBadRequest)
			return
		}
		safeName := sanitizeUploadFilename(header.Filename)
		destPath := strings.TrimRight(destDir, "/") + "/" + safeName
		if err := svc.WriteFileBytes(r.Context(), destPath, content, desktop.SourceUser); err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		event := desktop.Event{Type: "desktop_changed", Payload: map[string]interface{}{"operation": "upload_file", "path": destPath}, CreatedAt: time.Now().UTC()}
		broadcastDesktopEvent(s, hub, event)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok", "path": destPath})
	}
}

func handleDesktopDownload(s *Server) http.HandlerFunc {
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
		data, entry, err := svc.ReadFileBytes(r.Context(), r.URL.Query().Get("path"))
		if err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		mimeType := entry.MIMEType
		if mimeType == "" {
			mimeType = desktop.MIMETypeForName(entry.Name)
		}
		if mimeType == "" {
			mimeType = "application/octet-stream"
		}
		contentDisposition := "attachment"
		if r.URL.Query().Get("inline") == "1" && isDesktopDownloadInlineMIME(mimeType) {
			contentDisposition = "inline"
		}
		w.Header().Set("Content-Type", mimeType)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`%s; filename="%s"`, contentDisposition, sanitizeContentDisposition(entry.Name)))
		http.ServeContent(w, r, entry.Name, entry.ModTime, bytes.NewReader(data))
	}
}

func isDesktopDownloadInlineMIME(mimeType string) bool {
	mimeType = strings.ToLower(strings.TrimSpace(mimeType))
	return strings.HasPrefix(mimeType, "audio/") || strings.HasPrefix(mimeType, "video/")
}

// sanitizeContentDisposition strips characters that could inject HTTP headers
// from a Content-Disposition filename parameter.
func sanitizeContentDisposition(name string) string {
	replacer := strings.NewReplacer(`"`, ``, "\r", ``, "\n", ``, `\`, ``)
	return replacer.Replace(name)
}
