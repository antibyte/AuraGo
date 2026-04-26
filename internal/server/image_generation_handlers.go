package server

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"sort"
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
			s.Logger.Error("Image generation test failed", "error", err)
			json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": "Image generation test failed"})
			return
		}

		result.Prompt = "Test generation"
		tools.SaveGeneratedImage(s.ImageGalleryDB, result)

		if s.MediaRegistryDB != nil {
			imgPath := filepath.Join(cfg.Directories.DataDir, "generated_images", result.Filename)
			imgHash, _ := tools.ComputeMediaFileHash(imgPath)
			tools.RegisterMedia(s.MediaRegistryDB, tools.MediaItem{
				MediaType:        "image",
				SourceTool:       "generate_image",
				Filename:         result.Filename,
				FilePath:         imgPath,
				WebPath:          result.WebPath,
				Format:           "png",
				Provider:         result.Provider,
				Model:            result.Model,
				Prompt:           result.Prompt,
				Quality:          result.Quality,
				Style:            result.Style,
				Size:             result.Size,
				SourceImage:      result.SourceImage,
				GenerationTimeMs: int64(result.DurationMs),
				CostEstimate:     result.CostEstimate,
				Tags:             []string{"auto-generated"},
				Hash:             imgHash,
			})
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":   "ok",
			"message":  "Image generated successfully",
			"web_path": result.WebPath,
		})
	}
}

type unifiedImage struct {
	ID               int64   `json:"id"`
	CreatedAt        string  `json:"created_at"`
	Prompt           string  `json:"prompt"`
	EnhancedPrompt   string  `json:"enhanced_prompt,omitempty"`
	Provider         string  `json:"provider"`
	Model            string  `json:"model"`
	Size             string  `json:"size,omitempty"`
	Quality          string  `json:"quality,omitempty"`
	Style            string  `json:"style,omitempty"`
	Filename         string  `json:"filename"`
	FileSize         int64   `json:"file_size"`
	SourceImage      string  `json:"source_image,omitempty"`
	GenerationTimeMs int64   `json:"generation_time_ms"`
	CostEstimate     float64 `json:"cost_estimate"`
	WebPath          string  `json:"web_path"`
	SourceDB         string  `json:"source_db"`
}

func (u unifiedImage) GetCreatedAt() string {
	return u.CreatedAt
}

// handleImageGalleryList returns a handler that lists generated images with pagination.
// It merges results from both the Image Gallery DB and the Media Registry DB so that
// all images (agent-generated, seeded, screenshots) appear in the gallery view.
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

		seen := make(map[string]bool)
		var all []unifiedImage

		// Primary source: Media Registry DB (has all images including agent-generated)
		if s.MediaRegistryDB != nil {
			items, _, err := tools.SearchMedia(s.MediaRegistryDB, query, "image", nil, 5000, 0)
			if err == nil {
				for _, item := range items {
					if provider != "" && item.Provider != provider {
						continue
					}
					key := item.Filename
					if key != "" && seen[key] {
						continue
					}
					seen[key] = true
					wp := item.WebPath
					if wp == "" {
						wp = "/files/generated_images/" + item.Filename
					}
					all = append(all, unifiedImage{
						ID:               item.ID,
						CreatedAt:        item.CreatedAt,
						Prompt:           item.Prompt,
						EnhancedPrompt:   "",
						Provider:         item.Provider,
						Model:            item.Model,
						Size:             item.Size,
						Quality:          item.Quality,
						Style:            item.Style,
						Filename:         item.Filename,
						FileSize:         item.FileSize,
						SourceImage:      item.SourceImage,
						GenerationTimeMs: item.GenerationTimeMs,
						CostEstimate:     item.CostEstimate,
						WebPath:          wp,
						SourceDB:         "media_registry",
					})
				}
			}
		}

		// Secondary source: Image Gallery DB for records not yet in Media Registry
		if s.ImageGalleryDB != nil {
			galleryImages, _, err := tools.ListGeneratedImages(s.ImageGalleryDB, provider, query, 5000, 0)
			if err == nil {
				for _, img := range galleryImages {
					key := img.Filename
					if key != "" && seen[key] {
						continue
					}
					seen[key] = true
					wp := "/files/generated_images/" + img.Filename
					all = append(all, unifiedImage{
						ID:               img.ID,
						CreatedAt:        img.CreatedAt,
						Prompt:           img.Prompt,
						EnhancedPrompt:   img.EnhancedPrompt,
						Provider:         img.Provider,
						Model:            img.Model,
						Size:             img.Size,
						Quality:          img.Quality,
						Style:            img.Style,
						Filename:         img.Filename,
						FileSize:         img.FileSize,
						SourceImage:      img.SourceImage,
						GenerationTimeMs: img.GenerationTimeMs,
						CostEstimate:     img.CostEstimate,
						WebPath:          wp,
						SourceDB:         "image_gallery",
					})
				}
			}
		}

		sort.SliceStable(all, func(i, j int) bool {
			return all[i].CreatedAt > all[j].CreatedAt
		})

		total := len(all)
		if offset > total {
			offset = total
		}
		end := offset + limit
		if end > total {
			end = total
		}
		page := all[offset:end]

		if page == nil {
			page = []unifiedImage{}
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "ok",
			"images": page,
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

		source := r.URL.Query().Get("source")

		s.CfgMu.RLock()
		dataDir := s.Cfg.Directories.DataDir
		s.CfgMu.RUnlock()

		switch r.Method {
		case http.MethodGet:
			if source == "media_registry" && s.MediaRegistryDB != nil {
				item, err := tools.GetMedia(s.MediaRegistryDB, id)
				if err != nil {
					w.WriteHeader(http.StatusNotFound)
					json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": "Image not found"})
					return
				}
				wp := item.WebPath
				if wp == "" {
					wp = "/files/generated_images/" + item.Filename
				}
				json.NewEncoder(w).Encode(map[string]interface{}{
					"status": "ok",
					"image": unifiedImage{
						ID:               item.ID,
						CreatedAt:        item.CreatedAt,
						Prompt:           item.Prompt,
						Provider:         item.Provider,
						Model:            item.Model,
						Size:             item.Size,
						Quality:          item.Quality,
						Style:            item.Style,
						Filename:         item.Filename,
						FileSize:         item.FileSize,
						SourceImage:      item.SourceImage,
						GenerationTimeMs: item.GenerationTimeMs,
						CostEstimate:     item.CostEstimate,
						WebPath:          wp,
						SourceDB:         "media_registry",
					},
				})
				return
			}

			img, err := tools.GetGeneratedImage(s.ImageGalleryDB, id)
			if err != nil {
				w.WriteHeader(http.StatusNotFound)
				json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": "Image not found"})
				return
			}
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status": "ok",
				"image": unifiedImage{
					ID:               img.ID,
					CreatedAt:        img.CreatedAt,
					Prompt:           img.Prompt,
					EnhancedPrompt:   img.EnhancedPrompt,
					Provider:         img.Provider,
					Model:            img.Model,
					Size:             img.Size,
					Quality:          img.Quality,
					Style:            img.Style,
					Filename:         img.Filename,
					FileSize:         img.FileSize,
					SourceImage:      img.SourceImage,
					GenerationTimeMs: img.GenerationTimeMs,
					CostEstimate:     img.CostEstimate,
					WebPath:          "/files/generated_images/" + img.Filename,
					SourceDB:         "image_gallery",
				},
			})

		case http.MethodDelete:
			var filename string
			var filePath string

			if source == "media_registry" && s.MediaRegistryDB != nil {
				item, err := tools.GetMedia(s.MediaRegistryDB, id)
				if err != nil {
					w.WriteHeader(http.StatusNotFound)
					json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": "Image not found"})
					return
				}
				filename = item.Filename
				filePath = item.FilePath
				if err := tools.DeleteMedia(s.MediaRegistryDB, id); err != nil {
					w.WriteHeader(http.StatusInternalServerError)
					s.Logger.Error("Failed to delete media image", "id", id, "error", err)
					json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": "Failed to delete image"})
					return
				}
				if _, err := tools.DeleteGeneratedImagesByFilename(s.ImageGalleryDB, filename); err != nil {
					s.Logger.Warn("Failed to delete companion generated image record", "filename", filename, "error", err)
				}
			} else {
				img, err := tools.GetGeneratedImage(s.ImageGalleryDB, id)
				if err != nil {
					w.WriteHeader(http.StatusNotFound)
					json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": "Image not found"})
					return
				}
				filename = img.Filename
				if err := tools.DeleteGeneratedImage(s.ImageGalleryDB, id, dataDir); err != nil {
					w.WriteHeader(http.StatusInternalServerError)
					s.Logger.Error("Failed to delete generated image", "image_id", id, "error", err)
					json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": "Failed to delete image"})
					return
				}
				if _, err := tools.DeleteMediaImagesByFilename(s.MediaRegistryDB, filename); err != nil {
					s.Logger.Warn("Failed to delete companion media image record", "filename", filename, "error", err)
				}
			}

			// Best-effort: remove the physical file after both registries have been cleared.
			if filename != "" {
				if filePath != "" {
					os.Remove(filePath)
				} else {
					os.Remove(filepath.Join(dataDir, "generated_images", filename))
				}
			}

			json.NewEncoder(w).Encode(map[string]string{"status": "ok", "message": "Image deleted"})

		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
			json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": "method not allowed"})
		}
	}
}
