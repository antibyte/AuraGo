package server

import (
	"bytes"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"math"
	"net/http"
	"path"
	"strings"

	"aurago/internal/desktop"
)

const pixelMaxPixels = 32 * 1024 * 1024

func pixelScaledDimensions(data []byte, scale float64) (int, int, error) {
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	w, h := float64(cfg.Width)*scale, float64(cfg.Height)*scale
	if err != nil || math.IsNaN(scale) || math.IsInf(scale, 0) || scale < 1 || w*h > pixelMaxPixels {
		return 0, 0, fmt.Errorf("invalid upscale dimensions")
	}
	return int(w), int(h), nil
}

func pixelRelativePath(raw string) (string, error) {
	if raw == "" || strings.HasPrefix(raw, "/") || strings.ContainsAny(raw, "\\:\x00") {
		return "", fmt.Errorf("a workspace-relative path is required")
	}
	for _, part := range strings.Split(raw, "/") {
		if part == ".." {
			return "", fmt.Errorf("path traversal is not allowed")
		}
	}
	clean := path.Clean(raw)
	if clean == "." {
		return "", fmt.Errorf("a file path is required")
	}
	return clean, nil
}

func pixelByteLimit(svc *desktop.Service) int64 {
	limit := int64(svc.Config().MaxFileSizeMB) << 20
	if limit <= 0 || limit > 32<<20 {
		limit = 32 << 20
	}
	return limit
}

func pixelValidateImage(data []byte, limit int64) (string, error) {
	if int64(len(data)) > limit {
		return "", fmt.Errorf("image exceeds the size limit")
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || (format != "png" && format != "jpeg") {
		return "", fmt.Errorf("a valid PNG or JPEG image is required")
	}
	if cfg.Width <= 0 || cfg.Height <= 0 || int64(cfg.Width)*int64(cfg.Height) > pixelMaxPixels {
		return "", fmt.Errorf("image dimensions exceed the limit")
	}
	if _, _, err := image.Decode(bytes.NewReader(data)); err != nil {
		return "", fmt.Errorf("invalid image data")
	}
	return format, nil
}

func pixelSourceBytes(s *Server, r *http.Request, sourcePath, sourceData string) ([]byte, error) {
	svc, _, err := s.getDesktopService(r.Context())
	if err != nil {
		return nil, err
	}
	var data []byte
	if sourceData != "" {
		data, err = pixelDecodeDataURL(sourceData)
	} else {
		sourcePath, err = pixelRelativePath(sourcePath)
		if err == nil {
			data, _, err = svc.ReadFileBytes(r.Context(), sourcePath)
		}
	}
	if err != nil {
		return nil, fmt.Errorf("could not read source image")
	}
	if _, err := pixelValidateImage(data, pixelByteLimit(svc)); err != nil {
		return nil, err
	}
	return data, nil
}
