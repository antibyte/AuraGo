package launchpad

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

const (
	maxIconSize     = 2 << 20 // 2 MB
	iconDir         = "launchpad_icons"
	allowedIconExts = ".svg.png.webp.jpg.jpeg.gif"
)

// DownloadIconResult holds the outcome of an icon download.
type DownloadIconResult struct {
	LocalPath string `json:"local_path"`
	FileName  string `json:"file_name"`
}

// DownloadIcon fetches an image from the given URL and stores it under dataDir/launchpad_icons/.
// It returns the relative path suitable for storing in LaunchpadLink.IconPath.
func DownloadIcon(dataDir, imageURL, linkID string) (*DownloadIconResult, error) {
	if strings.TrimSpace(imageURL) == "" {
		return nil, fmt.Errorf("image URL is required")
	}

	parsed, err := url.Parse(imageURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return nil, fmt.Errorf("invalid image URL")
	}

	client := &http.Client{Timeout: httpTimeout}
	resp, err := client.Get(imageURL)
	if err != nil {
		return nil, fmt.Errorf("failed to download icon: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download returned status %d", resp.StatusCode)
	}

	// Content-Type validation
	ct := strings.ToLower(resp.Header.Get("Content-Type"))
	if ct != "" && !strings.HasPrefix(ct, "image/") && !strings.HasPrefix(ct, "application/svg") && !strings.HasPrefix(ct, "text/svg") {
		return nil, fmt.Errorf("invalid content type: %s", ct)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxIconSize+1))
	if err != nil {
		return nil, fmt.Errorf("failed to read icon body: %w", err)
	}
	if len(body) > maxIconSize {
		return nil, fmt.Errorf("icon exceeds maximum size of 2 MB")
	}

	// Determine extension from URL or Content-Type
	ext := strings.ToLower(path.Ext(parsed.Path))
	if ext == "" || !strings.Contains(allowedIconExts, ext) {
		ext = extFromContentType(ct)
	}
	if ext == "" {
		ext = ".png" // fallback
	}

	// Ensure safe filename
	safeLinkID := strings.ReplaceAll(linkID, "..", "")
	safeLinkID = filepath.Base(safeLinkID)
	if safeLinkID == "" {
		safeLinkID = "icon"
	}

	destDir := filepath.Join(dataDir, iconDir)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create icon directory: %w", err)
	}

	timestamp := time.Now().UTC().Format("20060102_150405")
	fileName := fmt.Sprintf("%s_%s%s", safeLinkID, timestamp, ext)
	destPath := filepath.Join(destDir, fileName)

	if err := os.WriteFile(destPath, body, 0644); err != nil {
		return nil, fmt.Errorf("failed to write icon file: %w", err)
	}

	// Store relative path from dataDir for portability
	relPath := filepath.Join(iconDir, fileName)
	return &DownloadIconResult{
		LocalPath: relPath,
		FileName:  fileName,
	}, nil
}

// DeleteIcon removes an icon file from disk.
func DeleteIcon(dataDir, relPath string) error {
	if relPath == "" {
		return nil
	}
	fullPath := filepath.Join(dataDir, relPath)
	// Prevent path traversal
	if !strings.HasPrefix(filepath.Clean(fullPath), filepath.Clean(dataDir)) {
		return fmt.Errorf("invalid icon path")
	}
	return os.Remove(fullPath)
}

func extFromContentType(ct string) string {
	switch {
	case strings.Contains(ct, "svg"):
		return ".svg"
	case strings.Contains(ct, "webp"):
		return ".webp"
	case strings.Contains(ct, "png"):
		return ".png"
	case strings.Contains(ct, "jpeg") || strings.Contains(ct, "jpg"):
		return ".jpg"
	case strings.Contains(ct, "gif"):
		return ".gif"
	default:
		return ""
	}
}
