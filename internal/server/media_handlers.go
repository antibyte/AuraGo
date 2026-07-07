package server

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"aurago/internal/tools"
)

var errMediaItemNotFound = errors.New("media item not found")

type mediaBulkDeleteRequest struct {
	IDs []int64 `json:"ids"`
}

type mediaBulkDeleteFailure struct {
	ID      int64  `json:"id"`
	Message string `json:"message"`
}

// handleMediaList handles GET /api/media — lists media items with optional type/search filter.
func handleMediaList(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		mediaType := r.URL.Query().Get("type") // "image", "audio", "document", or empty for all
		query := r.URL.Query().Get("q")
		limitStr := r.URL.Query().Get("limit")
		offsetStr := r.URL.Query().Get("offset")

		limit := 50
		offset := 0
		if v, err := strconv.Atoi(limitStr); err == nil && v > 0 && v <= 200 {
			limit = v
		}
		if v, err := strconv.Atoi(offsetStr); err == nil && v >= 0 {
			offset = v
		}

		s.CfgMu.RLock()
		dataDir := s.Cfg.Directories.DataDir
		s.CfgMu.RUnlock()

		items, err := searchAllMediaForServer(s.MediaRegistryDB, query, mediaType)
		if err != nil {
			s.Logger.Error("Failed to search media", "query", query, "media_type", mediaType, "error", err)
			json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": "Failed to load media"})
			return
		}
		var total int
		items, total = filterDisplayableMediaItems(dataDir, items, limit, offset)

		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "ok",
			"items":  items,
			"total":  total,
			"limit":  limit,
			"offset": offset,
		})
	}
}

func filterDisplayableMediaItems(dataDir string, items []tools.MediaItem, limit, offset int) ([]tools.MediaItem, int) {
	filtered := make([]tools.MediaItem, 0, len(items))
	for _, item := range items {
		webPath, ok := mediaRegistryItemDisplayWebPath(dataDir, item)
		if !ok {
			continue
		}
		item.WebPath = webPath
		filtered = append(filtered, item)
	}

	total := len(filtered)
	if offset > total {
		offset = total
	}
	end := offset + limit
	if end > total {
		end = total
	}
	return filtered[offset:end], total
}

func (s *Server) deleteMediaItemByID(id int64, dataDir string) error {
	item, err := tools.GetMedia(s.MediaRegistryDB, id)
	if err != nil {
		return errMediaItemNotFound
	}
	if err := tools.DeleteMedia(s.MediaRegistryDB, id); err != nil {
		return fmt.Errorf("failed to delete media item: %w", err)
	}
	removed, removeErr := removeMediaItemFileSafely(dataDir, *item)
	if removeErr != nil {
		s.Logger.Warn("Failed to remove media file", "media_id", id, "file_path", item.FilePath, "web_path", item.WebPath, "error", removeErr)
	}
	if !removed && strings.TrimSpace(item.FilePath) != "" {
		s.Logger.Warn("Skipped unsafe media file removal", "media_id", id, "file_path", item.FilePath, "web_path", item.WebPath)
	}
	return nil
}

func searchAllMediaForServer(db *sql.DB, query, mediaType string) ([]tools.MediaItem, error) {
	const pageSize = 1000
	var all []tools.MediaItem
	offset := 0
	for {
		page, total, err := tools.SearchMedia(db, query, mediaType, nil, pageSize, offset)
		if err != nil {
			return nil, err
		}
		if offset == 0 {
			all = make([]tools.MediaItem, 0, total)
		}
		all = append(all, page...)
		if len(page) == 0 || len(all) >= total {
			return all, nil
		}
		offset += len(page)
	}
}

func listAllGeneratedImagesForServer(db *sql.DB, provider, query string) ([]tools.GeneratedImageRecord, error) {
	const pageSize = 1000
	var all []tools.GeneratedImageRecord
	offset := 0
	for {
		page, total, err := tools.ListGeneratedImages(db, provider, query, pageSize, offset)
		if err != nil {
			return nil, err
		}
		if offset == 0 {
			all = make([]tools.GeneratedImageRecord, 0, total)
		}
		all = append(all, page...)
		if len(page) == 0 || len(all) >= total {
			return all, nil
		}
		offset += len(page)
	}
}

func removeMediaItemFileSafely(dataDir string, item tools.MediaItem) (bool, error) {
	candidates := []string{}
	hasFilePath := strings.TrimSpace(item.FilePath) != ""
	hasWebPath := strings.TrimSpace(item.WebPath) != ""
	if hasFilePath {
		candidates = append(candidates, item.FilePath)
	}
	if hasWebPath && !isExternalWebPath(item.WebPath) {
		if localPath, ok := mediaWebPathToLocalPath(dataDir, item.WebPath); ok {
			candidates = append(candidates, localPath)
		}
	}
	if !hasFilePath && !hasWebPath {
		if defaultWebPath, ok := defaultMediaWebPathForFilename(dataDir, item.MediaType, item.Filename); ok {
			localPath, ok := mediaWebPathToLocalPath(dataDir, defaultWebPath)
			if !ok {
				return false, nil
			}
			candidates = append(candidates, localPath)
		}
	}
	seen := map[string]bool{}
	for _, candidate := range candidates {
		candidate = filepath.Clean(strings.TrimSpace(candidate))
		if candidate == "" || seen[candidate] {
			continue
		}
		seen[candidate] = true
		localPath, ok := localMediaFilePathForRemoval(dataDir, candidate)
		if !ok {
			continue
		}
		if err := os.Remove(localPath); err != nil && !os.IsNotExist(err) {
			return false, err
		}
		return true, nil
	}
	return false, nil
}

func localMediaFilePathForRemoval(dataDir, filePath string) (string, bool) {
	if strings.TrimSpace(dataDir) == "" || strings.TrimSpace(filePath) == "" {
		return "", false
	}
	cleanPath, err := filepath.Abs(filepath.Clean(filePath))
	if err != nil {
		return "", false
	}
	info, err := os.Lstat(cleanPath)
	if err != nil || info.IsDir() {
		return "", false
	}
	resolvedPath := cleanPath
	if evalPath, evalErr := filepath.EvalSymlinks(cleanPath); evalErr == nil {
		resolvedPath = filepath.Clean(evalPath)
	}

	for _, mapping := range mediaFileServerDataSubdirs {
		root, err := filepath.Abs(filepath.Clean(filepath.Join(dataDir, mapping.subdir)))
		if err != nil {
			continue
		}
		resolvedRoot := root
		if evalRoot, evalErr := filepath.EvalSymlinks(root); evalErr == nil {
			resolvedRoot = filepath.Clean(evalRoot)
		}
		rel, relErr := filepath.Rel(resolvedRoot, resolvedPath)
		if relErr == nil && rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel) {
			return cleanPath, true
		}
	}
	return "", false
}

// handleMediaBulkDelete handles POST /api/media/bulk-delete for registry media
// such as audio, videos, and documents.
func handleMediaBulkDelete(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": "method not allowed"})
			return
		}

		var req mediaBulkDeleteRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": "invalid request body"})
			return
		}

		seen := make(map[int64]bool)
		var ids []int64
		for _, id := range req.IDs {
			if id <= 0 || seen[id] {
				continue
			}
			seen[id] = true
			ids = append(ids, id)
		}
		if len(ids) == 0 {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": "no media IDs selected"})
			return
		}

		s.CfgMu.RLock()
		dataDir := s.Cfg.Directories.DataDir
		s.CfgMu.RUnlock()

		deleted := 0
		failures := []mediaBulkDeleteFailure{}
		for _, id := range ids {
			if err := s.deleteMediaItemByID(id, dataDir); err != nil {
				failures = append(failures, mediaBulkDeleteFailure{ID: id, Message: err.Error()})
				continue
			}
			deleted++
		}

		status := "ok"
		if len(failures) > 0 {
			status = "partial"
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  status,
			"deleted": deleted,
			"failed":  failures,
		})
	}
}

// handleMediaByID routes GET, PATCH, and DELETE for /api/media/{id}.
func handleMediaByID(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		// Extract ID and optional sub-path from /api/media/{id}[/preview]
		pathSuffix := strings.TrimPrefix(r.URL.Path, "/api/media/")
		parts := strings.SplitN(pathSuffix, "/", 2)
		if len(parts) == 0 || parts[0] == "" {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": "missing media ID"})
			return
		}
		id, err := strconv.ParseInt(parts[0], 10, 64)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": "invalid media ID"})
			return
		}

		// Sub-action: /api/media/{id}/preview
		if len(parts) == 2 && parts[1] == "preview" {
			s.handleMediaPreview(w, r, id)
			return
		}

		s.CfgMu.RLock()
		dataDir := s.Cfg.Directories.DataDir
		s.CfgMu.RUnlock()

		switch r.Method {
		case http.MethodGet:
			item, err := tools.GetMedia(s.MediaRegistryDB, id)
			if err != nil {
				w.WriteHeader(http.StatusNotFound)
				json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": "media item not found"})
				return
			}
			json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok", "item": item})

		case http.MethodDelete:
			if err := s.deleteMediaItemByID(id, dataDir); err != nil {
				if errors.Is(err, errMediaItemNotFound) {
					w.WriteHeader(http.StatusNotFound)
					json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": "media item not found"})
					return
				}
				w.WriteHeader(http.StatusInternalServerError)
				s.Logger.Error("Failed to delete media item", "media_id", id, "error", err)
				json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": "Failed to delete media item"})
				return
			}
			json.NewEncoder(w).Encode(map[string]string{"status": "ok", "message": "Media item deleted"})

		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
			json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": "method not allowed"})
		}
	}
}

// handleMediaPreview serves a preview URL for a document. For PDFs it serves inline;
// for Office formats it redirects to the document file (downloading).
func (s *Server) handleMediaPreview(w http.ResponseWriter, r *http.Request, id int64) {
	item, err := tools.GetMedia(s.MediaRegistryDB, id)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": "media item not found"})
		return
	}
	if item.MediaType != "document" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": "preview only available for documents"})
		return
	}
	// Return the web path with ?inline=1 so browsers render it in-tab
	if item.WebPath != "" {
		previewURL := item.WebPath + "?inline=1"
		json.NewEncoder(w).Encode(map[string]string{"status": "ok", "preview_url": previewURL})
	} else {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": "no web path available for this document"})
	}
}
