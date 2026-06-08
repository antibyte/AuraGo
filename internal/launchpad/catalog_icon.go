package launchpad

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

const (
	catalogIconCacheDir = "launchpad_catalog_icons"
	catalogIconCDNPNG   = "https://cdn.jsdelivr.net/gh/homarr-labs/dashboard-icons/png"
	catalogIconCDNSVG   = "https://cdn.jsdelivr.net/gh/homarr-labs/dashboard-icons/svg"
	catalogIconCDNWEBP  = "https://cdn.jsdelivr.net/gh/homarr-labs/dashboard-icons/webp"
)

// CatalogIconAssetURL returns a first-party URL for a cached Homarr catalog icon.
func CatalogIconAssetURL(name, format string) string {
	format = normalizeCatalogIconFormat(format)
	return fmt.Sprintf("/api/launchpad/icons/asset?name=%s&format=%s", url.QueryEscape(strings.TrimSpace(name)), format)
}

// ParseCatalogIconAssetURL extracts name and format from a first-party catalog icon URL.
func ParseCatalogIconAssetURL(imageURL string) (name, format string, ok bool) {
	imageURL = strings.TrimSpace(imageURL)
	if imageURL == "" {
		return "", "", false
	}
	parsed, err := url.Parse(imageURL)
	if err != nil {
		return "", "", false
	}
	path := parsed.Path
	if !strings.HasSuffix(path, "/api/launchpad/icons/asset") && path != "/api/launchpad/icons/asset" {
		return "", "", false
	}
	name = strings.TrimSpace(parsed.Query().Get("name"))
	format = normalizeCatalogIconFormat(parsed.Query().Get("format"))
	if name == "" {
		return "", "", false
	}
	return name, format, true
}

// EnsureCatalogIconCached downloads a catalog icon once and stores it under dataDir.
func EnsureCatalogIconCached(dataDir, name, format string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("icon name is required")
	}
	format = normalizeCatalogIconFormat(format)
	cacheDir := filepath.Join(dataDir, catalogIconCacheDir)
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return "", fmt.Errorf("create catalog icon cache: %w", err)
	}
	fileName := fmt.Sprintf("%s.%s", sanitizeCatalogIconName(name), format)
	destPath := filepath.Join(cacheDir, fileName)
	if _, err := os.Stat(destPath); err == nil {
		return destPath, nil
	}

	sourceURL := catalogIconRemoteURL(name, format)
	client := &http.Client{Timeout: httpTimeout}
	resp, err := client.Get(sourceURL)
	if err != nil {
		return "", fmt.Errorf("fetch catalog icon: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("catalog icon fetch returned status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxIconSize+1))
	if err != nil {
		return "", fmt.Errorf("read catalog icon body: %w", err)
	}
	if len(body) > maxIconSize {
		return "", fmt.Errorf("catalog icon exceeds maximum size of 2 MB")
	}
	if err := os.WriteFile(destPath, body, 0644); err != nil {
		return "", fmt.Errorf("write catalog icon cache: %w", err)
	}
	return destPath, nil
}

func catalogIconRemoteURL(name, format string) string {
	switch format {
	case "svg":
		return fmt.Sprintf("%s/%s.svg", catalogIconCDNSVG, name)
	case "webp":
		return fmt.Sprintf("%s/%s.webp", catalogIconCDNWEBP, name)
	default:
		return fmt.Sprintf("%s/%s.png", catalogIconCDNPNG, name)
	}
}

// NormalizeCatalogIconFormat normalizes a catalog icon format token.
func NormalizeCatalogIconFormat(format string) string {
	return normalizeCatalogIconFormat(format)
}

func normalizeCatalogIconFormat(format string) string {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "svg":
		return "svg"
	case "webp":
		return "webp"
	default:
		return "png"
	}
}

func sanitizeCatalogIconName(name string) string {
	name = strings.ReplaceAll(name, "..", "")
	name = strings.ReplaceAll(name, "/", "")
	name = strings.ReplaceAll(name, "\\", "")
	return filepath.Base(name)
}