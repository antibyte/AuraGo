package server

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"aurago/internal/tools"
)

// handleImageGenerationTest returns a handler that tests image generation with a simple prompt.
func handleImageGenerationTest(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s.CfgMu.RLock()
		cfg := s.Cfg
		s.CfgMu.RUnlock()

		w.Header().Set("Content-Type", "application/json")

		if !cfg.ImageGeneration.Enabled {
			json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": "Image generation is not enabled"})
			return
		}
		if cfg.ImageGeneration.APIKey == "" {
			json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": "No API key configured for image generation provider"})
			return
		}

		genCfg := tools.ImageGenConfig{
			ProviderType: cfg.ImageGeneration.ProviderType,
			BaseURL:      cfg.ImageGeneration.BaseURL,
			APIKey:       cfg.ImageGeneration.APIKey,
			Model:        cfg.ImageGeneration.ResolvedModel,
			DataDir:      cfg.Directories.DataDir,
		}
		opts := tools.ImageGenOptions{
			Size:    "512x512",
			Quality: "standard",
		}

		result, err := tools.GenerateImage(genCfg, "A simple test image: a colorful geometric pattern", opts)
		if err != nil {
			json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": err.Error()})
			return
		}

		// Save to gallery
		result.Prompt = "Test generation"
		tools.SaveGeneratedImage(s.ImageGalleryDB, result)

		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":   "ok",
			"message":  "Image generated successfully",
			"web_path": result.WebPath,
		})
	}
}

// handleImageGalleryList returns a handler that lists generated images with pagination.
func handleImageGalleryList(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		provider := r.URL.Query().Get("provider")
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

		images, total, err := tools.ListGeneratedImages(s.ImageGalleryDB, provider, query, limit, offset)
		if err != nil {
			json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": err.Error()})
			return
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "ok",
			"images": images,
			"total":  total,
			"limit":  limit,
			"offset": offset,
		})
	}
}

// handleImageGalleryByID routes GET and DELETE for /api/image-gallery/{id}.
func handleImageGalleryByID(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		// Extract ID from path: /api/image-gallery/{id}
		parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/image-gallery/"), "/")
		if len(parts) == 0 || parts[0] == "" {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": "missing image ID"})
			return
		}
		id, err := strconv.ParseInt(parts[0], 10, 64)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": "invalid image ID"})
			return
		}

		s.CfgMu.RLock()
		dataDir := s.Cfg.Directories.DataDir
		s.CfgMu.RUnlock()

		switch r.Method {
		case http.MethodGet:
			img, err := tools.GetGeneratedImage(s.ImageGalleryDB, id)
			if err != nil {
				w.WriteHeader(http.StatusNotFound)
				json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": "Image not found"})
				return
			}
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status": "ok",
				"image":  img,
			})

		case http.MethodDelete:
			if err := tools.DeleteGeneratedImage(s.ImageGalleryDB, id, dataDir); err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": err.Error()})
				return
			}
			json.NewEncoder(w).Encode(map[string]string{"status": "ok", "message": "Image deleted"})

		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
			json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": "method not allowed"})
		}
	}
}
