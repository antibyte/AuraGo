package server

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"aurago/internal/tools"
)

func handleMediaConversionTest(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		w.Header().Set("Content-Type", "application/json")

		s.CfgMu.RLock()
		cfg := *s.Cfg
		s.CfgMu.RUnlock()

		var req struct {
			MediaConversion struct {
				Enabled         *bool  `json:"enabled"`
				ReadOnly        *bool  `json:"readonly"`
				FFmpegPath      string `json:"ffmpeg_path"`
				ImageMagickPath string `json:"imagemagick_path"`
				TimeoutSeconds  *int   `json:"timeout_seconds"`
			} `json:"media_conversion"`
		}

		if r.Body != nil {
			defer r.Body.Close()
			if data, err := io.ReadAll(io.LimitReader(r.Body, 1<<20)); err == nil && len(strings.TrimSpace(string(data))) > 0 {
				if err := json.Unmarshal(data, &req); err != nil {
					jsonError(w, "Invalid request payload", http.StatusBadRequest)
					return
				}
			}
		}

		if req.MediaConversion.Enabled != nil {
			cfg.Tools.MediaConversion.Enabled = *req.MediaConversion.Enabled
		}
		if req.MediaConversion.ReadOnly != nil {
			cfg.Tools.MediaConversion.ReadOnly = *req.MediaConversion.ReadOnly
		}
		if req.MediaConversion.FFmpegPath != "" {
			cfg.Tools.MediaConversion.FFmpegPath = req.MediaConversion.FFmpegPath
		}
		if req.MediaConversion.ImageMagickPath != "" {
			cfg.Tools.MediaConversion.ImageMagickPath = req.MediaConversion.ImageMagickPath
		}
		if req.MediaConversion.TimeoutSeconds != nil {
			cfg.Tools.MediaConversion.TimeoutSeconds = *req.MediaConversion.TimeoutSeconds
		}

		if !cfg.Tools.MediaConversion.Enabled {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "error",
				"message": "Media conversion is disabled",
			})
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()
		_ = json.NewEncoder(w).Encode(tools.MediaConversionHealth(ctx, &cfg.Tools.MediaConversion))
	}
}
