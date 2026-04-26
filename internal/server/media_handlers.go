package server

import (
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

		items, total, err := tools.SearchMedia(s.MediaRegistryDB, query, mediaType, nil, limit, offset)
		if err != nil {
			s.Logger.Error("Failed to search media", "query", query, "media_type", mediaType, "error", err)
			json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": "Failed to load media"})
			return
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "ok",
			"items":  items,
			"total":  total,
			"limit":  limit,
			"offset": offset,
		})
	}
}

func (s *Server) deleteMediaItemByID(id int64, dataDir string) error {
	item, err := tools.GetMedia(s.MediaRegistryDB, id)
	if err != nil {
		return errMediaItemNotFound
	}
	if err := tools.DeleteMedia(s.MediaRegistryDB, id); err != nil {
		return fmt.Errorf("failed to delete media item: %w", err)
	}

	if item.FilePath != "" {
		_ = os.Remove(item.FilePath)
		return nil
	}
	if item.Filename == "" {
		return nil
	}

	var subDir string
	switch item.MediaType {
	case "audio", "music":
		subDir = "audio"
	case "tts":
		subDir = "tts"
	case "document":
		subDir = "documents"
	case "image":
		subDir = "generated_images"
	case "video":
		subDir = "generated_videos"
	}
	if subDir != "" {
		_ = os.Remove(filepath.Join(dataDir, subDir, item.Filename))
	}
	return nil
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
