package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"aurago/internal/tools"
)

// mediaConversionApprovedBinaries lists the converter base names the health
// probe may execute. An operator may point the probe at a converter installed
// anywhere, but never at an arbitrary program.
var mediaConversionApprovedBinaries = map[string]bool{
	"ffmpeg":  true,
	"ffprobe": true,
	"magick":  true,
}

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

		// The probe executes the requested binary, so only an approved converter
		// may be started. Everything else is refused before a process is spawned.
		for _, binary := range []string{cfg.Tools.MediaConversion.FFmpegPath, cfg.Tools.MediaConversion.ImageMagickPath} {
			if err := validateMediaConversionBinaryPath(binary); err != nil {
				jsonError(w, err.Error(), http.StatusBadRequest)
				return
			}
		}

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()
		_ = json.NewEncoder(w).Encode(tools.MediaConversionHealth(ctx, &cfg.Tools.MediaConversion))
	}
}

// validateMediaConversionBinaryPath accepts only an existing regular file whose
// base name is an approved media converter. On Windows the bare name "convert"
// is refused as well, because C:\Windows\System32\convert.exe is the built-in
// FAT converter and not ImageMagick.
func validateMediaConversionBinaryPath(path string) error {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return nil
	}
	if len(trimmed) > 1024 {
		return fmt.Errorf("binary path is too long")
	}
	if strings.ContainsAny(trimmed, "\x00\r\n") {
		return fmt.Errorf("binary path contains control characters")
	}
	info, err := os.Stat(trimmed)
	if err != nil {
		return fmt.Errorf("binary %q was not found", trimmed)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("binary %q is not a regular file", trimmed)
	}
	base := strings.ToLower(strings.TrimSuffix(filepath.Base(trimmed), filepath.Ext(trimmed)))
	if mediaConversionApprovedBinaries[base] {
		return nil
	}
	if base == "convert" && runtime.GOOS != "windows" {
		return nil
	}
	return fmt.Errorf("binary %q is not an approved media converter (allowed: ffmpeg, ffprobe, magick)", trimmed)
}
