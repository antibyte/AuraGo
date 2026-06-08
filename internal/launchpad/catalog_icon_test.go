package launchpad

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCatalogIconAssetURLUsesFirstPartyPath(t *testing.T) {
	got := CatalogIconAssetURL("docker", "png")
	if !strings.HasPrefix(got, "/api/launchpad/icons/asset?") {
		t.Fatalf("CatalogIconAssetURL = %q, want first-party asset path", got)
	}
	if strings.Contains(got, "cdn.jsdelivr.net") {
		t.Fatal("catalog icon asset URL must not expose external CDN hosts")
	}
}

func TestParseCatalogIconAssetURL(t *testing.T) {
	name, format, ok := ParseCatalogIconAssetURL("/api/launchpad/icons/asset?name=docker&format=svg")
	if !ok || name != "docker" || format != "svg" {
		t.Fatalf("ParseCatalogIconAssetURL() = (%q, %q, %v)", name, format, ok)
	}
}

func TestCopyCatalogIconToLinkFromAssetURL(t *testing.T) {
	tmpDir := t.TempDir()
	cacheDir := filepath.Join(tmpDir, catalogIconCacheDir)
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		t.Fatalf("mkdir cache: %v", err)
	}
	if err := os.WriteFile(filepath.Join(cacheDir, "docker.png"), []byte("png-bytes"), 0644); err != nil {
		t.Fatalf("write cache: %v", err)
	}

	result, err := DownloadIcon(tmpDir, CatalogIconAssetURL("docker", "png"), "link-1")
	if err != nil {
		t.Fatalf("DownloadIcon from asset URL failed: %v", err)
	}
	if result.LocalPath == "" || !strings.HasPrefix(filepath.ToSlash(result.LocalPath), "launchpad_icons/") {
		t.Fatalf("unexpected local path: %#v", result)
	}
	fullPath := filepath.Join(tmpDir, result.LocalPath)
	if body, err := os.ReadFile(fullPath); err != nil || string(body) != "png-bytes" {
		t.Fatalf("copied icon body mismatch: %v", err)
	}
}