package launchpad

import (
	"aurago/internal/security"
	"context"
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
	maxIconSize = 2 << 20 // 2 MB
	iconDir     = "launchpad_icons"
)

// DownloadIconResult holds the outcome of an icon download.
type DownloadIconResult struct {
	LocalPath string `json:"local_path"`
	FileName  string `json:"file_name"`
}

// DownloadIcon fetches an image from the given URL and stores it under dataDir/launchpad_icons/.
// It returns the relative path suitable for storing in LaunchpadLink.IconPath.
func DownloadIcon(dataDir, imageURL, linkID string) (*DownloadIconResult, error) {
	return DownloadIconContext(context.Background(), dataDir, imageURL, linkID)
}

// DownloadIconContext permits only public HTTP(S) targets, including redirects.
func DownloadIconContext(ctx context.Context, dataDir, imageURL, linkID string) (*DownloadIconResult, error) {
	return downloadIcon(ctx, dataDir, imageURL, linkID, security.NewStrictPublicHTTPClientForURLContext)
}

func downloadIcon(ctx context.Context, dataDir, imageURL, linkID string, clientForURL func(context.Context, string, time.Duration) (*http.Client, error)) (*DownloadIconResult, error) {
	ctx, cancel := context.WithTimeout(ctx, httpTimeout)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if strings.TrimSpace(imageURL) == "" {
		return nil, fmt.Errorf("image URL is required")
	}

	if name, format, ok := ParseCatalogIconAssetURL(imageURL); ok {
		return copyCatalogIconToLink(dataDir, name, format, linkID)
	}

	parsed, err := url.Parse(imageURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return nil, fmt.Errorf("invalid image URL")
	}

	var resp *http.Response
	for redirects := 0; ; redirects++ {
		client, err := clientForURL(ctx, parsed.String(), httpTimeout)
		if err != nil {
			return nil, fmt.Errorf("icon URL refused: %w", err)
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
		if err != nil {
			return nil, err
		}
		resp, err = client.Do(req)
		client.CloseIdleConnections()
		if err != nil {
			return nil, fmt.Errorf("failed to download icon: %w", err)
		}
		if resp.StatusCode != 301 && resp.StatusCode != 302 && resp.StatusCode != 303 && resp.StatusCode != 307 && resp.StatusCode != 308 {
			break
		}
		next, err := resp.Location()
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("invalid icon redirect: %w", err)
		}
		if redirects >= 10 {
			return nil, fmt.Errorf("too many icon redirects")
		}
		parsed = next
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
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	// Determine extension from URL or Content-Type
	ext := strings.ToLower(path.Ext(parsed.Path))
	if !allowedIconExtension(ext) {
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
	if relPath == "" || strings.HasPrefix(relPath, "https://") || strings.HasPrefix(relPath, "http://") {
		return nil
	}
	name, err := localIconName(relPath)
	if err != nil {
		return err
	}
	data, err := os.OpenRoot(dataDir)
	if err != nil {
		return err
	}
	defer data.Close()
	info, err := data.Lstat(iconDir)
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("invalid icon directory")
	}
	icons, err := data.OpenRoot(iconDir)
	if err != nil {
		return err
	}
	defer icons.Close()
	info, err = icons.Lstat(name)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("icon must be a regular file")
	}
	return icons.Remove(name)
}

func allowedIconExtension(ext string) bool {
	switch ext {
	case ".svg", ".png", ".webp", ".jpg", ".jpeg", ".gif":
		return true
	}
	return false
}

func localIconName(relPath string) (string, error) {
	parts := strings.Split(strings.ReplaceAll(relPath, "\\", "/"), "/")
	if len(parts) != 2 || parts[0] != iconDir || strings.ContainsAny(parts[1], ":\x00") || !allowedIconExtension(strings.ToLower(path.Ext(parts[1]))) {
		return "", fmt.Errorf("invalid icon path")
	}
	return parts[1], nil
}

func validateIconPath(iconPath string) error {
	if iconPath == "" {
		return nil
	}
	if strings.HasPrefix(iconPath, "https://") || strings.HasPrefix(iconPath, "http://") {
		u, err := url.Parse(iconPath)
		if err != nil || u.Hostname() == "" || u.User != nil {
			return fmt.Errorf("invalid icon URL")
		}
		return nil
	}
	_, err := localIconName(iconPath)
	return err
}

func copyCatalogIconToLink(dataDir, name, format, linkID string) (*DownloadIconResult, error) {
	cachedPath, err := EnsureCatalogIconCached(dataDir, name, format)
	if err != nil {
		return nil, err
	}
	body, err := os.ReadFile(cachedPath)
	if err != nil {
		return nil, fmt.Errorf("read cached catalog icon: %w", err)
	}
	ext := "." + normalizeCatalogIconFormat(format)
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
	relPath := filepath.Join(iconDir, fileName)
	return &DownloadIconResult{
		LocalPath: relPath,
		FileName:  fileName,
	}, nil
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
