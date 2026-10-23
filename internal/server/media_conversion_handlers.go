package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"aurago/internal/mediabins"
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

// validateMediaConversionBinaryPath accepts only a converter that resolves to an
// approved media tool.
//
// The configured value is resolved exactly like the tool resolves it, so a bare
// name such as "ffmpeg" is looked up on PATH and a valid configuration is never
// refused. The approved converter is then identified by the base name of the
// resolved file. On Windows the legacy name "convert" is refused, because
// C:\Windows\System32\convert.exe is the built-in FAT converter and not ImageMagick.
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
	resolved, err := mediabins.Resolve(trimmed, nil)
	if err != nil {
		return err
	}
	stem := strings.ToLower(strings.TrimSuffix(filepath.Base(resolved), filepath.Ext(resolved)))
	if !approvedMediaConverterStem(stem) {
		return fmt.Errorf("binary %q is not an approved media converter (allowed: ffmpeg, ffprobe, magick)", trimmed)
	}
	return nil
}

// approvedMediaConverterStem accepts the converter names and their versioned
// spellings (ffmpeg, ffmpeg.exe, ffmpeg-7.0, ffmpeg7). The legacy ImageMagick
// name "convert" is approved outside Windows only.
func approvedMediaConverterStem(stem string) bool {
	if stem == "convert" && runtime.GOOS != "windows" {
		return true
	}
	for _, name := range []string{"ffmpeg", "ffprobe", "magick"} {
		if stem == name {
			return true
		}
		if len(stem) <= len(name) || stem[:len(name)] != name {
			continue
		}
		rest := stem[len(name):]
		if rest[0] >= '0' && rest[0] <= '9' {
			return true
		}
		if (rest[0] == '-' || rest[0] == '_' || rest[0] == '.') && len(rest) > 1 && rest[1] >= '0' && rest[1] <= '9' {
			return true
		}
	}
	return false
}
