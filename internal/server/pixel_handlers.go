package server

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"aurago/internal/tools"
)

// handlePixelConfig returns GET /api/pixel/config — image generation capabilities.
func handlePixelConfig(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")

		s.CfgMu.RLock()
		cfg := s.Cfg.ImageGeneration
		s.CfgMu.RUnlock()

		supportsImg2Img := false
		switch strings.ToLower(cfg.ProviderType) {
		case "openai", "openrouter", "stability":
			supportsImg2Img = true
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"enabled":           cfg.Enabled,
			"provider_type":     cfg.ProviderType,
			"model":             cfg.ResolvedModel,
			"supports_img2img":  supportsImg2Img,
			"default_size":      cfg.DefaultSize,
			"default_quality":   cfg.DefaultQuality,
			"default_style":     cfg.DefaultStyle,
		})
	}
}

// handlePixelGenerate returns POST /api/pixel/generate — generate image from text prompt.
func handlePixelGenerate(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")

		s.CfgMu.RLock()
		cfg := s.Cfg
		s.CfgMu.RUnlock()

		if !cfg.ImageGeneration.Enabled {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": "Image generation is not enabled"})
			return
		}

		var req struct {
			Prompt  string `json:"prompt"`
			Size    string `json:"size"`
			Quality string `json:"quality"`
			Style   string `json:"style"`
			Model   string `json:"model"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Prompt) == "" {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": "prompt is required"})
			return
		}

		genCfg := tools.ImageGenConfig{
			ProviderType: cfg.ImageGeneration.ProviderType,
			BaseURL:      cfg.ImageGeneration.BaseURL,
			APIKey:       cfg.ImageGeneration.APIKey,
			Model:        cfg.ImageGeneration.ResolvedModel,
			DataDir:      cfg.Directories.DataDir,
		}
		if req.Model != "" {
			genCfg.Model = req.Model
		}
		if req.Size == "" {
			req.Size = cfg.ImageGeneration.DefaultSize
		}
		if req.Quality == "" {
			req.Quality = cfg.ImageGeneration.DefaultQuality
		}
		if req.Style == "" {
			req.Style = cfg.ImageGeneration.DefaultStyle
		}

		opts := tools.ImageGenOptions{
			Size:    req.Size,
			Quality: req.Quality,
			Style:   req.Style,
		}

		result, err := tools.GenerateImage(genCfg, req.Prompt, opts)
		if err != nil {
			s.Logger.Error("Pixel generate failed", "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": err.Error()})
			return
		}

		tools.SaveGeneratedImage(s.ImageGalleryDB, result)

		imgPath := filepath.Join(cfg.Directories.DataDir, "generated_images", result.Filename)
		width, height := imageDimensions(imgPath)

		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "ok",
			"path":    imgPath,
			"url":     result.WebPath,
			"width":   width,
			"height":  height,
			"format":  "png",
		})
	}
}

// handlePixelEnhance returns POST /api/pixel/enhance — enhance image with AI.
func handlePixelEnhance(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")

		s.CfgMu.RLock()
		cfg := s.Cfg
		s.CfgMu.RUnlock()

		if !cfg.ImageGeneration.Enabled {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": "Image generation is not enabled"})
			return
		}

		var req struct {
			SourcePath string  `json:"source_path"`
			SourceData string  `json:"source_data"`
			Prompt     string  `json:"prompt"`
			Strength   float64 `json:"strength"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": "invalid request"})
			return
		}

		if req.SourcePath == "" && req.SourceData == "" {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": "source_path or source_data is required"})
			return
		}

		sourcePath := req.SourcePath
		if sourcePath == "" && req.SourceData != "" {
			dataURL := req.SourceData
			comma := strings.Index(dataURL, ",")
			if comma >= 0 {
				dataURL = dataURL[comma+1:]
			}
			imgBytes, err := base64.StdEncoding.DecodeString(dataURL)
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": "invalid base64 source data"})
				return
			}
			tmpPath := filepath.Join(cfg.Directories.DataDir, "generated_images", fmt.Sprintf("pixel_enhance_%d.png", time.Now().UnixNano()))
			if err := os.MkdirAll(filepath.Dir(tmpPath), 0755); err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": "failed to prepare source"})
				return
			}
			if err := os.WriteFile(tmpPath, imgBytes, 0644); err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": "failed to write temp source"})
				return
			}
			sourcePath = tmpPath
			defer os.Remove(tmpPath)
		}

		genCfg := tools.ImageGenConfig{
			ProviderType: cfg.ImageGeneration.ProviderType,
			BaseURL:      cfg.ImageGeneration.BaseURL,
			APIKey:       cfg.ImageGeneration.APIKey,
			Model:        cfg.ImageGeneration.ResolvedModel,
			DataDir:      cfg.Directories.DataDir,
		}

		prompt := req.Prompt
		if prompt == "" {
			prompt = "enhance this image, improve quality and details"
		}

		opts := tools.ImageGenOptions{
			Size:        cfg.ImageGeneration.DefaultSize,
			Quality:     cfg.ImageGeneration.DefaultQuality,
			Style:       cfg.ImageGeneration.DefaultStyle,
			SourceImage: sourcePath,
		}

		result, err := tools.GenerateImage(genCfg, prompt, opts)
		if err != nil {
			s.Logger.Error("Pixel enhance failed", "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": err.Error()})
			return
		}

		tools.SaveGeneratedImage(s.ImageGalleryDB, result)

		imgPath := filepath.Join(cfg.Directories.DataDir, "generated_images", result.Filename)
		width, height := imageDimensions(imgPath)

		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "ok",
			"path":    imgPath,
			"url":     result.WebPath,
			"width":   width,
			"height":  height,
			"format":  "png",
		})
	}
}

// handlePixelSave returns POST /api/pixel/save — save canvas data URL as file.
func handlePixelSave(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")

		s.CfgMu.RLock()
		readonly := s.Cfg.VirtualDesktop.ReadOnly
		dataDir := s.Cfg.Directories.DataDir
		s.CfgMu.RUnlock()

		if readonly {
			w.WriteHeader(http.StatusForbidden)
			json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": "read-only mode"})
			return
		}

		var req struct {
			Path    string `json:"path"`
			Data    string `json:"data"`
			Format  string `json:"format"`
			Quality int    `json:"quality"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Path) == "" || strings.TrimSpace(req.Data) == "" {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": "path and data are required"})
			return
		}

		dataURL := req.Data
		comma := strings.Index(dataURL, ",")
		if comma >= 0 {
			dataURL = dataURL[comma+1:]
		}
		imgBytes, err := base64.StdEncoding.DecodeString(dataURL)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": "invalid base64 data"})
			return
		}

		savePath := req.Path
		if !filepath.IsAbs(savePath) {
			savePath = filepath.Join(dataDir, "workspace", savePath)
		}
		if err := os.MkdirAll(filepath.Dir(savePath), 0755); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": fmt.Sprintf("failed to create directory: %v", err)})
			return
		}
		if err := os.WriteFile(savePath, imgBytes, 0644); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": fmt.Sprintf("failed to write file: %v", err)})
			return
		}

		info, _ := os.Stat(savePath)
		size := int64(0)
		if info != nil {
			size = info.Size()
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "ok",
			"path":   savePath,
			"size":   size,
		})
	}
}

// imageDimensions reads an image file header and returns width/height (0,0 on failure).
func imageDimensions(path string) (int, int) {
	data, err := os.ReadFile(path)
	if err != nil || len(data) < 30 {
		return 0, 0
	}
	// PNG
	if data[0] == 0x89 && data[1] == 0x50 && data[2] == 0x4E && data[3] == 0x47 && len(data) > 24 {
		w := int(data[16])<<24 | int(data[17])<<16 | int(data[18])<<8 | int(data[19])
		h := int(data[20])<<24 | int(data[21])<<16 | int(data[22])<<8 | int(data[23])
		return w, h
	}
	// JPEG
	if data[0] == 0xFF && data[1] == 0xD8 {
		i := 2
		for i < len(data)-1 {
			if data[i] != 0xFF {
				i++
				continue
			}
			marker := data[i+1]
			if marker == 0xC0 || marker == 0xC2 {
				if i+9 < len(data) {
					h := int(data[i+5])<<8 | int(data[i+6])
					w := int(data[i+7])<<8 | int(data[i+8])
					return w, h
				}
			}
			if i+3 < len(data) {
				length := int(data[i+2])<<8 | int(data[i+3])
				i += 2 + length
			} else {
				break
			}
		}
	}
	return 0, 0
}
