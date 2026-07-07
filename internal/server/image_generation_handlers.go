package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"aurago/internal/tools"
)

var errImageGalleryItemNotFound = errors.New("image not found")

type imageGalleryBulkDeleteItem struct {
	ID     int64  `json:"id"`
	Source string `json:"source"`
}

type imageGalleryBulkDeleteRequest struct {
	Items []imageGalleryBulkDeleteItem `json:"items"`
}

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

func (s *Server) deleteImageGalleryItemByID(id int64, source, dataDir string) error {
	var filename string

	if source == "media_registry" && s.MediaRegistryDB != nil {
		item, err := tools.GetMedia(s.MediaRegistryDB, id)
		if err != nil {
			return errImageGalleryItemNotFound
		}
		filename = item.Filename
		if err := tools.DeleteMedia(s.MediaRegistryDB, id); err != nil {
			return fmt.Errorf("failed to delete image: %w", err)
		}
		removed, removeErr := removeMediaItemFileSafely(dataDir, *item)
		if removeErr != nil {
			s.Logger.Warn("Failed to remove media registry image file", "media_id", id, "file_path", item.FilePath, "web_path", item.WebPath, "error", removeErr)
		}
		if !removed && strings.TrimSpace(item.FilePath) != "" {
			s.Logger.Warn("Skipped unsafe media registry image file removal", "media_id", id, "file_path", item.FilePath, "web_path", item.WebPath)
		}
		if _, err := tools.DeleteGeneratedImagesByFilename(s.ImageGalleryDB, filename); err != nil {
			s.Logger.Warn("Failed to delete companion generated image record", "filename", filename, "error", err)
		}
	} else {
		img, err := tools.GetGeneratedImage(s.ImageGalleryDB, id)
		if err != nil {
			return errImageGalleryItemNotFound
		}
		filename = img.Filename
		if err := tools.DeleteGeneratedImage(s.ImageGalleryDB, id, dataDir); err != nil {
			return fmt.Errorf("failed to delete image: %w", err)
		}
		if _, err := tools.DeleteMediaImagesByFilename(s.MediaRegistryDB, filename); err != nil {
			s.Logger.Warn("Failed to delete companion media image record", "filename", filename, "error", err)
		}
	}

	return nil
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

		s.CfgMu.RLock()
		dataDir := s.Cfg.Directories.DataDir
		s.CfgMu.RUnlock()

		seen := make(map[string]bool)
		var all []unifiedImage

		// Primary source: Media Registry DB (has all images including agent-generated)
		if s.MediaRegistryDB != nil {
			items, err := searchAllMediaForServer(s.MediaRegistryDB, query, "image")
			if err == nil {
				for _, item := range items {
					if provider != "" && item.Provider != provider {
						continue
					}
					wp, ok := mediaRegistryItemDisplayWebPath(dataDir, item)
					if !ok {
						continue
					}
					key := item.Filename
					if key != "" && seen[key] {
						continue
					}
					seen[key] = true
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
			galleryImages, err := listAllGeneratedImagesForServer(s.ImageGalleryDB, provider, query)
			if err == nil {
				for _, img := range galleryImages {
					if !generatedImageFileExists(dataDir, img.Filename) {
						continue
					}
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

var mediaFileServerDataSubdirs = []struct {
	prefix string
	subdir string
}{
	{prefix: "/files/3d_printer_media/", subdir: "3d_printer_media"},
	{prefix: "/files/frigate_media/", subdir: "frigate_media"},
	{prefix: "/files/generated_images/", subdir: "generated_images"},
	{prefix: "/files/generated_videos/", subdir: "generated_videos"},
	{prefix: "/files/audio/", subdir: "audio"},
	{prefix: "/files/documents/", subdir: "documents"},
	{prefix: "/files/downloads/", subdir: "downloads"},
	{prefix: "/files/browser_screenshots/", subdir: "browser_screenshots"},
	{prefix: "/files/browser_downloads/", subdir: "browser_downloads"},
}

func mediaRegistryImageFileExists(dataDir string, item tools.MediaItem) bool {
	_, ok := mediaRegistryItemDisplayWebPath(dataDir, item)
	return ok
}

func mediaRegistryItemDisplayWebPath(dataDir string, item tools.MediaItem) (string, bool) {
	if webPath, ok := displayableMediaWebPath(dataDir, item.WebPath); ok {
		return webPath, true
	}
	if webPath, ok := displayableMediaWebPath(dataDir, item.FilePath); ok {
		return webPath, true
	}
	if webPath, ok := webPathForLocalMediaFile(dataDir, item.FilePath); ok {
		return webPath, true
	}
	return defaultMediaWebPathForFilename(dataDir, item.MediaType, item.Filename)
}

func displayableMediaWebPath(dataDir, rawPath string) (string, bool) {
	webPath := strings.TrimSpace(rawPath)
	if webPath == "" {
		return "", false
	}
	if isExternalWebPath(webPath) {
		return webPath, true
	}
	localPath, ok := mediaWebPathToLocalPath(dataDir, webPath)
	if !ok || !regularFileExists(localPath) {
		return "", false
	}
	return webPath, true
}

func webPathForLocalMediaFile(dataDir, filePath string) (string, bool) {
	if strings.TrimSpace(dataDir) == "" || strings.TrimSpace(filePath) == "" || !regularFileExists(filePath) {
		return "", false
	}
	cleanFilePath := filepath.Clean(filePath)
	for _, mapping := range mediaFileServerDataSubdirs {
		root := filepath.Clean(filepath.Join(dataDir, mapping.subdir))
		rel, err := filepath.Rel(root, cleanFilePath)
		if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
			continue
		}
		return mapping.prefix + filepath.ToSlash(rel), true
	}
	return "", false
}

func defaultMediaWebPathForFilename(dataDir, mediaType, filename string) (string, bool) {
	if strings.TrimSpace(dataDir) == "" || strings.TrimSpace(filename) == "" {
		return "", false
	}
	subdir, prefix, ok := defaultMediaFileLocation(mediaType)
	if !ok {
		return "", false
	}
	localPath := filepath.Join(dataDir, subdir, filename)
	if !regularFileExists(localPath) {
		return "", false
	}
	return prefix + filename, true
}

func defaultMediaFileLocation(mediaType string) (subdir, prefix string, ok bool) {
	switch mediaType {
	case "image":
		return "generated_images", "/files/generated_images/", true
	case "video":
		return "generated_videos", "/files/generated_videos/", true
	case "audio", "music":
		return "audio", "/files/audio/", true
	case "document":
		return "documents", "/files/documents/", true
	default:
		return "", "", false
	}
}

func generatedImageFileExists(dataDir, filename string) bool {
	if strings.TrimSpace(dataDir) == "" || strings.TrimSpace(filename) == "" {
		return false
	}
	return regularFileExists(filepath.Join(dataDir, "generated_images", filename))
}

func mediaWebPathToLocalPath(dataDir, rawPath string) (string, bool) {
	if strings.TrimSpace(dataDir) == "" {
		return "", false
	}
	webPath := strings.TrimSpace(rawPath)
	if webPath == "" {
		return "", false
	}
	if parsed, err := url.Parse(webPath); err == nil && parsed.Scheme != "" && parsed.Host != "" {
		webPath = parsed.EscapedPath()
	}
	if i := strings.IndexAny(webPath, "?#"); i >= 0 {
		webPath = webPath[:i]
	}
	decodedPath, err := url.PathUnescape(webPath)
	if err != nil {
		return "", false
	}
	cleanPath := path.Clean(decodedPath)
	for _, mapping := range mediaFileServerDataSubdirs {
		if !strings.HasPrefix(cleanPath, mapping.prefix) {
			continue
		}
		relPath := strings.TrimPrefix(cleanPath, mapping.prefix)
		if relPath == "" || relPath == "." {
			return "", false
		}
		root := filepath.Join(dataDir, mapping.subdir)
		localPath := filepath.Clean(filepath.Join(root, filepath.FromSlash(relPath)))
		rel, err := filepath.Rel(root, localPath)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
			return "", false
		}
		return localPath, true
	}
	return "", false
}

func regularFileExists(filePath string) bool {
	if strings.TrimSpace(filePath) == "" {
		return false
	}
	info, err := os.Stat(filePath)
	return err == nil && !info.IsDir()
}

func isExternalWebPath(webPath string) bool {
	parsed, err := url.Parse(strings.TrimSpace(webPath))
	return err == nil && parsed.Scheme != "" && parsed.Host != "" && !strings.EqualFold(parsed.Scheme, "file")
}

// handleImageGalleryBulkDelete handles POST /api/image-gallery/bulk-delete.
func handleImageGalleryBulkDelete(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": "method not allowed"})
			return
		}

		var req imageGalleryBulkDeleteRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": "invalid request body"})
			return
		}

		seen := make(map[string]bool)
		var items []imageGalleryBulkDeleteItem
		for _, item := range req.Items {
			source := strings.TrimSpace(item.Source)
			if source == "" {
				source = "image_gallery"
			}
			if item.ID <= 0 {
				continue
			}
			key := source + ":" + strconv.FormatInt(item.ID, 10)
			if seen[key] {
				continue
			}
			seen[key] = true
			items = append(items, imageGalleryBulkDeleteItem{ID: item.ID, Source: source})
		}
		if len(items) == 0 {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": "no images selected"})
			return
		}

		s.CfgMu.RLock()
		dataDir := s.Cfg.Directories.DataDir
		s.CfgMu.RUnlock()

		deleted := 0
		failures := []mediaBulkDeleteFailure{}
		for _, item := range items {
			if err := s.deleteImageGalleryItemByID(item.ID, item.Source, dataDir); err != nil {
				failures = append(failures, mediaBulkDeleteFailure{ID: item.ID, Message: err.Error()})
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
			if err := s.deleteImageGalleryItemByID(id, source, dataDir); err != nil {
				if errors.Is(err, errImageGalleryItemNotFound) {
					w.WriteHeader(http.StatusNotFound)
					json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": "Image not found"})
					return
				}
				w.WriteHeader(http.StatusInternalServerError)
				s.Logger.Error("Failed to delete image", "image_id", id, "source", source, "error", err)
				json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": "Failed to delete image"})
				return
			}
			json.NewEncoder(w).Encode(map[string]string{"status": "ok", "message": "Image deleted"})

		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
			json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": "method not allowed"})
		}
	}
}
