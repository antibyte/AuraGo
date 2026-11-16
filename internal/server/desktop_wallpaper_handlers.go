package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"aurago/internal/desktop"
)

func handleDesktopWallpaper(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireDesktopPermission(s, w, r, desktopScopeRead) {
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			writeDesktopWallpaperError(w, "wallpaper_method_not_allowed", http.StatusMethodNotAllowed)
			return
		}
		svc, _, err := s.getDesktopService(r.Context())
		if err != nil {
			writeDesktopWallpaperError(w, "wallpaper_service_unavailable", http.StatusServiceUnavailable)
			return
		}
		image, err := svc.ReadCustomWallpaper(r.Context(), r.URL.Query().Get("path"))
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return
			}
			var wallpaperErr *desktop.WallpaperError
			if errors.As(err, &wallpaperErr) {
				writeDesktopWallpaperError(w, wallpaperErr.Code, desktopWallpaperErrorStatus(wallpaperErr.Code))
				return
			}
			writeDesktopWallpaperError(w, "wallpaper_service_unavailable", http.StatusServiceUnavailable)
			return
		}

		w.Header().Set("Content-Type", image.MIMEType)
		w.Header().Set("Content-Length", strconv.Itoa(len(image.Bytes)))
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Cache-Control", "private, no-store")
		w.Header().Set("Content-Disposition", "inline")
		w.WriteHeader(http.StatusOK)
		if r.Method == http.MethodGet {
			_, _ = w.Write(image.Bytes)
		}
	}
}

func desktopWallpaperErrorStatus(code string) int {
	switch code {
	case "wallpaper_invalid_path":
		return http.StatusBadRequest
	case "wallpaper_not_found":
		return http.StatusNotFound
	case "wallpaper_too_large", "wallpaper_dimensions_exceeded", "wallpaper_pixel_limit":
		return http.StatusRequestEntityTooLarge
	case "wallpaper_unsupported_format", "wallpaper_format_mismatch", "wallpaper_invalid_image":
		return http.StatusUnsupportedMediaType
	case "wallpaper_changed":
		return http.StatusConflict
	default:
		return http.StatusServiceUnavailable
	}
}

func writeDesktopWallpaperError(w http.ResponseWriter, code string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": code, "code": code})
}
