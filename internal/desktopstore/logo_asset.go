package desktopstore

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	storeLogoCacheDir    = "desktop_store_logos"
	storeLogoRemoteBase  = "https://cdn.jsdelivr.net/gh/homarr-labs/dashboard-icons/png"
	storeLogoHTTPTimeout = 30 * time.Second
	storeLogoMaxSize     = 2 << 20
)

// StoreLogoURL returns a first-party URL for a cached store catalog logo.
func StoreLogoURL(slug string) string {
	slug = sanitizeStoreLogoSlug(slug)
	if slug == "" {
		return ""
	}
	return fmt.Sprintf("/api/desktop/store/logos/%s.png", slug)
}

// EnsureStoreLogoCached downloads a catalog logo once and stores it under dataDir.
func EnsureStoreLogoCached(dataDir, slug string) (string, error) {
	slug = sanitizeStoreLogoSlug(slug)
	if slug == "" {
		return "", fmt.Errorf("logo slug is required")
	}
	cacheDir := filepath.Join(dataDir, storeLogoCacheDir)
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return "", fmt.Errorf("create store logo cache: %w", err)
	}
	destPath := filepath.Join(cacheDir, slug+".png")
	if _, err := os.Stat(destPath); err == nil {
		return destPath, nil
	}

	sourceURL := fmt.Sprintf("%s/%s.png", storeLogoRemoteBase, slug)
	client := &http.Client{Timeout: storeLogoHTTPTimeout}
	resp, err := client.Get(sourceURL)
	if err != nil {
		return "", fmt.Errorf("fetch store logo: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("store logo fetch returned status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, storeLogoMaxSize+1))
	if err != nil {
		return "", fmt.Errorf("read store logo body: %w", err)
	}
	if len(body) > storeLogoMaxSize {
		return "", fmt.Errorf("store logo exceeds maximum size of 2 MB")
	}
	if err := os.WriteFile(destPath, body, 0644); err != nil {
		return "", fmt.Errorf("write store logo cache: %w", err)
	}
	return destPath, nil
}

func sanitizeStoreLogoSlug(slug string) string {
	slug = strings.TrimSpace(slug)
	slug = strings.ReplaceAll(slug, "..", "")
	slug = strings.ReplaceAll(slug, "/", "")
	slug = strings.ReplaceAll(slug, "\\", "")
	return filepath.Base(slug)
}