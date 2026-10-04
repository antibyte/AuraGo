package server

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"aurago/internal/desktop"
	"aurago/internal/tools"
)

// handlePixelConfig returns GET /api/pixel/config — image generation capabilities.
func handlePixelConfig(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireDesktopOperation(s, w, r, desktopScopeRead, desktopRead) {
			return
		}
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
			"enabled":            cfg.Enabled,
			"provider_type":      cfg.ProviderType,
			"model":              cfg.ResolvedModel,
			"supports_img2img":   supportsImg2Img,
			"supports_remove_bg": cfg.Enabled && supportsImg2Img,
			"supports_upscale":   true,
			"default_size":       cfg.DefaultSize,
			"default_quality":    cfg.DefaultQuality,
			"default_style":      cfg.DefaultStyle,
			"max_monthly":        cfg.MaxMonthly,
			"daily_count":        tools.ImageGenDailyCount(),
		})
	}
}

// handlePixelGenerate returns POST /api/pixel/generate — generate image from text prompt.
func handlePixelGenerate(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireDesktopOperation(s, w, r, desktopScopeWrite, desktopExecute) {
			return
		}
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
		if err := decodeDesktopJSON(w, r, &req, desktopSmallJSONBodyLimit); err != nil || strings.TrimSpace(req.Prompt) == "" {
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
			"status": "ok",
			"path":   "Photos/" + result.Filename,
			"url":    result.WebPath,
			"width":  width,
			"height": height,
			"format": "png",
		})
	}
}

// handlePixelEnhance returns POST /api/pixel/enhance — enhance image with AI.
func handlePixelEnhance(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireDesktopOperation(s, w, r, desktopScopeWrite, desktopExecute) {
			return
		}
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
		if err := decodeDesktopJSON(w, r, &req, 44<<20); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": "invalid request"})
			return
		}

		if req.SourcePath == "" && req.SourceData == "" {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": "source_path or source_data is required"})
			return
		}

		data, err := pixelSourceBytes(s, r, req.SourcePath, req.SourceData)
		if err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		sourcePath, cleanup, err := pixelWriteTempSource(cfg.Directories.DataDir, data, "pixel_enhance")
		if err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		defer cleanup()

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
			"status": "ok",
			"path":   "Photos/" + result.Filename,
			"url":    result.WebPath,
			"width":  width,
			"height": height,
			"format": "png",
		})
	}
}

// handlePixelRemoveBG returns POST /api/pixel/remove-bg — AI background removal via img2img.
func handlePixelRemoveBG(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireDesktopOperation(s, w, r, desktopScopeWrite, desktopExecute) {
			return
		}
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

		supportsImg2Img := false
		switch strings.ToLower(cfg.ImageGeneration.ProviderType) {
		case "openai", "openrouter", "stability":
			supportsImg2Img = true
		}
		if !supportsImg2Img {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": "Provider does not support background removal"})
			return
		}

		var req struct {
			SourcePath string `json:"source_path"`
			SourceData string `json:"source_data"`
		}
		if err := decodeDesktopJSON(w, r, &req, 44<<20); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": "invalid request"})
			return
		}
		if req.SourcePath == "" && req.SourceData == "" {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": "source_path or source_data is required"})
			return
		}

		data, err := pixelSourceBytes(s, r, req.SourcePath, req.SourceData)
		if err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		sourcePath, cleanup, err := pixelWriteTempSource(cfg.Directories.DataDir, data, "pixel_remove_bg")
		if err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		defer cleanup()

		genCfg := tools.ImageGenConfig{
			ProviderType: cfg.ImageGeneration.ProviderType,
			BaseURL:      cfg.ImageGeneration.BaseURL,
			APIKey:       cfg.ImageGeneration.APIKey,
			Model:        cfg.ImageGeneration.ResolvedModel,
			DataDir:      cfg.Directories.DataDir,
		}

		opts := tools.ImageGenOptions{
			Size:        cfg.ImageGeneration.DefaultSize,
			Quality:     cfg.ImageGeneration.DefaultQuality,
			Style:       cfg.ImageGeneration.DefaultStyle,
			SourceImage: sourcePath,
		}

		prompt := "Remove the background completely. Transparent background, isolated subject, clean cutout, no backdrop."
		result, err := tools.GenerateImage(genCfg, prompt, opts)
		if err != nil {
			s.Logger.Error("Pixel remove-bg failed", "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": err.Error()})
			return
		}

		tools.SaveGeneratedImage(s.ImageGalleryDB, result)

		imgPath := filepath.Join(cfg.Directories.DataDir, "generated_images", result.Filename)
		width, height := imageDimensions(imgPath)

		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "ok",
			"path":   "Photos/" + result.Filename,
			"url":    result.WebPath,
			"width":  width,
			"height": height,
			"format": "png",
		})
	}
}

// handlePixelUpscale returns POST /api/pixel/upscale — 2× Lanczos upscale (Pure Go).
func handlePixelUpscale(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireDesktopOperation(s, w, r, desktopScopeWrite, desktopExecute) {
			return
		}
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
			Scale      float64 `json:"scale"`
		}
		if err := decodeDesktopJSON(w, r, &req, 44<<20); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": "invalid request"})
			return
		}
		if req.SourcePath == "" && req.SourceData == "" {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": "source_path or source_data is required"})
			return
		}

		scale := req.Scale
		if scale <= 1 {
			scale = 2
		}

		imgBytes, err := pixelSourceBytes(s, r, req.SourcePath, req.SourceData)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": err.Error()})
			return
		}
		width, height, err := pixelScaledDimensions(imgBytes, scale)
		if err != nil || width <= 0 || height <= 0 {
			jsonError(w, "Invalid upscale dimensions", http.StatusBadRequest)
			return
		}

		upscaled, err := tools.UpscaleImageBytes(imgBytes, scale)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": err.Error()})
			return
		}

		outDir := filepath.Join(cfg.Directories.DataDir, "generated_images")
		if err := os.MkdirAll(outDir, 0755); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": "failed to prepare output directory"})
			return
		}
		filename := fmt.Sprintf("pixel_upscale_%d.png", time.Now().UnixNano())
		outPath := filepath.Join(outDir, filename)
		if err := os.WriteFile(outPath, upscaled.PNGBytes, 0644); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": "failed to write upscaled image"})
			return
		}

		webPath := "/files/generated_images/" + filename
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "ok",
			"path":   "Photos/" + filename,
			"url":    webPath,
			"width":  upscaled.Width,
			"height": upscaled.Height,
			"format": "png",
		})
	}
}

func pixelDecodeDataURL(dataURL string) ([]byte, error) {
	comma := strings.Index(dataURL, ",")
	if comma >= 0 {
		dataURL = dataURL[comma+1:]
	}
	return base64.StdEncoding.DecodeString(dataURL)
}

func pixelWriteTempSource(dataDir string, imgBytes []byte, prefix string) (path string, cleanup func(), err error) {
	dir := filepath.Join(dataDir, "generated_images")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", nil, fmt.Errorf("failed to prepare source")
	}
	f, err := os.CreateTemp(dir, prefix+"-*.png")
	if err != nil {
		return "", nil, fmt.Errorf("failed to prepare source")
	}
	cleanup = func() { _ = os.Remove(f.Name()) }
	_, writeErr := f.Write(imgBytes)
	closeErr := f.Close()
	if writeErr != nil || closeErr != nil {
		cleanup()
		return "", nil, fmt.Errorf("failed to write temp source")
	}
	return f.Name(), cleanup, nil
}

// handlePixelSave returns POST /api/pixel/save — save canvas data URL as file.
func handlePixelSave(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireDesktopOperation(s, w, r, desktopScopeWrite, desktopWrite) {
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")

		svc, hub, err := s.getDesktopService(r.Context())
		if err != nil {
			jsonError(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		precondition, err := desktopFilePrecondition(r)
		if err != nil {
			writeDesktopFileError(w, err)
			return
		}

		var req struct {
			Path    string `json:"path"`
			Data    string `json:"data"`
			Format  string `json:"format"`
			Quality int    `json:"quality"`
		}
		limit := pixelByteLimit(svc)
		if err := decodeDesktopJSON(w, r, &req, (limit+2)/3*4+4096); err != nil {
			var tooLarge *http.MaxBytesError
			status := http.StatusBadRequest
			if errors.As(err, &tooLarge) {
				status = http.StatusRequestEntityTooLarge
			}
			jsonError(w, "Invalid image request", status)
			return
		}
		savePath, err := pixelRelativePath(req.Path)
		if err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		data, err := pixelDecodeDataURL(req.Data)
		if err != nil {
			jsonError(w, "Invalid image encoding", http.StatusBadRequest)
			return
		}
		format, err := pixelValidateImage(data, limit)
		if err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(savePath)), ".")
		if ext == "jpg" {
			ext = "jpeg"
		}
		requested := strings.ToLower(req.Format)
		if requested == "jpg" {
			requested = "jpeg"
		}
		if ext != format || (requested != "" && requested != format) {
			jsonError(w, "Image format and filename must agree", http.StatusBadRequest)
			return
		}
		entry, err := svc.WriteFileBytesConditional(r.Context(), savePath, data, desktop.SourceUser, precondition)
		if err != nil {
			writeDesktopFileError(w, err)
			return
		}
		version := desktop.NoteVersion(data)
		w.Header().Set("ETag", version)
		broadcastDesktopEvent(s, hub, desktop.Event{Type: "desktop_changed", Payload: map[string]interface{}{"operation": "write_file", "path": entry.Path}, CreatedAt: time.Now().UTC()})
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok", "path": entry.Path, "size": entry.Size, "version": version})
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
	// WEBP (lossy VP8)
	if len(data) > 30 && data[0] == 'R' && data[1] == 'I' && data[2] == 'F' && data[3] == 'F' && data[8] == 'W' && data[9] == 'E' && data[10] == 'B' && data[11] == 'P' {
		if data[12] == 'V' && data[13] == 'P' && data[14] == '8' && data[15] != 'L' {
			if len(data) > 26 {
				w := int(data[26]) | int(data[27])<<8
				h := int(data[28]) | int(data[29])<<8
				return w & 0x3FFF, h & 0x3FFF
			}
		}
		if data[12] == 'V' && data[13] == 'P' && data[14] == '8' && data[15] == 'L' {
			if len(data) > 25 {
				b0 := uint32(data[21])
				b1 := uint32(data[22])
				b2 := uint32(data[23])
				w := int(b0|(b1<<8)&0x3FFF) + 1
				h := int((b1>>6)|(b2<<2)&0x3FFF) + 1
				return w, h
			}
		}
	}
	// BMP
	if len(data) > 26 && data[0] == 'B' && data[1] == 'M' {
		w := int(data[18]) | int(data[19])<<8 | int(data[20])<<16 | int(data[21])<<24
		h := int(data[22]) | int(data[23])<<8 | int(data[24])<<16 | int(data[25])<<24
		if h < 0 {
			h = -h
		}
		if w > 0 && h > 0 && w < 100000 && h < 100000 {
			return w, h
		}
	}
	// GIF
	if len(data) > 10 && data[0] == 'G' && data[1] == 'I' && data[2] == 'F' {
		w := int(data[6]) | int(data[7])<<8
		h := int(data[8]) | int(data[9])<<8
		return w, h
	}
	return 0, 0
}
