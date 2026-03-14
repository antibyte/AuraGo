package tools

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// loadSourceImage reads an image file from disk for image-to-image operations.
func loadSourceImage(path string) ([]byte, error) {
	if path == "" {
		return nil, fmt.Errorf("source image path is empty")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read source image %q: %w", path, err)
	}
	return data, nil
}

// buildMultipartForm builds a multipart/form-data request body from string fields and file data.
func buildMultipartForm(fields map[string]string, files map[string][]byte) (*bytes.Buffer, string, error) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)

	for key, val := range fields {
		if err := w.WriteField(key, val); err != nil {
			return nil, "", fmt.Errorf("failed to write field %q: %w", key, err)
		}
	}
	for key, data := range files {
		part, err := w.CreateFormFile(key, key+".png")
		if err != nil {
			return nil, "", fmt.Errorf("failed to create form file %q: %w", key, err)
		}
		if _, err := part.Write(data); err != nil {
			return nil, "", fmt.Errorf("failed to write form file data: %w", err)
		}
	}

	if err := w.Close(); err != nil {
		return nil, "", fmt.Errorf("failed to close multipart writer: %w", err)
	}
	return &buf, w.FormDataContentType(), nil
}

// downloadImage fetches an image from a URL and returns the raw bytes.
func downloadImage(url string) ([]byte, error) {
	resp, err := imageGenHTTPClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to download image: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("image download returned status %d", resp.StatusCode)
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, 50*1024*1024)) // 50MB max
	if err != nil {
		return nil, fmt.Errorf("failed to read image data: %w", err)
	}
	return data, nil
}

// detectFormat inspects the magic bytes of image data to determine the format.
func detectFormat(data []byte) string {
	if len(data) < 4 {
		return "png"
	}
	// PNG: 89 50 4E 47
	if data[0] == 0x89 && data[1] == 0x50 && data[2] == 0x4E && data[3] == 0x47 {
		return "png"
	}
	// JPEG: FF D8 FF
	if data[0] == 0xFF && data[1] == 0xD8 && data[2] == 0xFF {
		return "jpeg"
	}
	// WebP: RIFF....WEBP
	if len(data) >= 12 && string(data[:4]) == "RIFF" && string(data[8:12]) == "WEBP" {
		return "webp"
	}
	// GIF: GIF89a or GIF87a
	if string(data[:3]) == "GIF" {
		return "gif"
	}
	return "png"
}

// truncateError truncates an error message to a reasonable length for logging.
func truncateError(s string) string {
	const max = 500
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

// tryDownloadImageURL attempts to download an image from a URL string.
// Returns (data, ext, error). Only accepts http/https URLs.
func tryDownloadImageURL(rawURL string) ([]byte, string, error) {
	u := strings.TrimSpace(rawURL)
	if !strings.HasPrefix(u, "http://") && !strings.HasPrefix(u, "https://") {
		return nil, "", fmt.Errorf("not a URL")
	}
	data, err := downloadImage(u)
	if err != nil {
		return nil, "", err
	}
	if len(data) < 100 {
		return nil, "", fmt.Errorf("downloaded data too small to be an image")
	}
	return data, detectFormat(data), nil
}

func isRecognizedImageData(data []byte) bool {
	if len(data) < 4 {
		return false
	}
	// PNG
	if data[0] == 0x89 && data[1] == 0x50 && data[2] == 0x4E && data[3] == 0x47 {
		return true
	}
	// JPEG
	if len(data) >= 3 && data[0] == 0xFF && data[1] == 0xD8 && data[2] == 0xFF {
		return true
	}
	// WebP
	if len(data) >= 12 && string(data[:4]) == "RIFF" && string(data[8:12]) == "WEBP" {
		return true
	}
	// GIF
	if len(data) >= 6 && string(data[:3]) == "GIF" {
		return true
	}
	return false
}

func tryDecodeImageString(raw string) ([]byte, string, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return nil, "", fmt.Errorf("empty string")
	}

	// data:image/...;base64,...
	if idx := strings.Index(s, "base64,"); idx >= 0 {
		b64 := s[idx+7:]
		if end := strings.IndexAny(b64, "\" \n\r)"); end > 0 {
			b64 = b64[:end]
		}
		if data, err := base64.StdEncoding.DecodeString(b64); err == nil && isRecognizedImageData(data) {
			return data, detectFormat(data), nil
		}
	}

	// Raw base64 blob.
	if data, err := base64.StdEncoding.DecodeString(s); err == nil && isRecognizedImageData(data) {
		return data, detectFormat(data), nil
	}

	// Direct URL.
	if data, ext, err := tryDownloadImageURL(s); err == nil {
		return data, ext, nil
	}

	// Markdown image link: ![...](url)
	if idx := strings.Index(s, "]("); idx >= 0 {
		rest := s[idx+2:]
		if end := strings.IndexByte(rest, ')'); end > 0 {
			if data, ext, err := tryDownloadImageURL(rest[:end]); err == nil {
				return data, ext, nil
			}
		}
	}

	return nil, "", fmt.Errorf("string does not contain image payload")
}

func extractImageFromAnyResponse(payload interface{}) ([]byte, string, error) {
	var walk func(v interface{}, depth int) ([]byte, string, error)

	walk = func(v interface{}, depth int) ([]byte, string, error) {
		if depth > 10 {
			return nil, "", fmt.Errorf("max depth reached")
		}

		switch t := v.(type) {
		case string:
			return tryDecodeImageString(t)

		case []interface{}:
			for _, item := range t {
				if data, ext, err := walk(item, depth+1); err == nil {
					return data, ext, nil
				}
			}

		case map[string]interface{}:
			priorityKeys := []string{"image_url", "url", "b64_json", "image_base64", "base64", "data", "image", "content", "output", "result"}
			for _, k := range priorityKeys {
				if child, ok := t[k]; ok {
					if data, ext, err := walk(child, depth+1); err == nil {
						return data, ext, nil
					}
				}
			}
			for _, child := range t {
				if data, ext, err := walk(child, depth+1); err == nil {
					return data, ext, nil
				}
			}
		}

		return nil, "", fmt.Errorf("no image payload at this node")
	}

	if data, ext, err := walk(payload, 0); err == nil {
		return data, ext, nil
	}
	return nil, "", fmt.Errorf("no extractable image payload found")
}

// ResolveSourceImagePath resolves a source image path relative to workspace or data dir.
func ResolveSourceImagePath(path, workspaceDir, dataDir string) string {
	if filepath.IsAbs(path) {
		return path
	}
	// Try workspace first
	candidate := filepath.Join(workspaceDir, path)
	if _, err := os.Stat(candidate); err == nil {
		return candidate
	}
	// Try data dir
	candidate = filepath.Join(dataDir, path)
	if _, err := os.Stat(candidate); err == nil {
		return candidate
	}
	// Try generated_images subfolder
	candidate = filepath.Join(dataDir, "generated_images", path)
	if _, err := os.Stat(candidate); err == nil {
		return candidate
	}
	return path
}
