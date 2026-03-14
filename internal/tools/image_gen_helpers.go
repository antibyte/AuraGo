package tools

import (
	"bytes"
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
