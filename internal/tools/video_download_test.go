package tools

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/config"
)

func TestParseYtDlpSearch(t *testing.T) {
	output := strings.Join([]string{
		`{"id":"abc123","title":"Example","uploader":"Uploader","duration":42,"view_count":1000}`,
		`warning: ignored`,
		`{"id":"def456","title":"Second","url":"https://example.test/video","thumbnail":"https://img.test/t.jpg"}`,
	}, "\n")

	items := parseYtDlpSearch(output)
	if len(items) != 2 {
		t.Fatalf("len(items) = %d, want 2", len(items))
	}
	if items[0].URL != "https://www.youtube.com/watch?v=abc123" {
		t.Fatalf("first URL = %q", items[0].URL)
	}
	if items[1].URL != "https://example.test/video" {
		t.Fatalf("second URL = %q", items[1].URL)
	}
}

func TestResolveDownloadedFilePathMapsContainerPath(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "video.mp4")
	if err := os.WriteFile(filePath, []byte("x"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	cfg := &config.Config{}
	cfg.Tools.VideoDownload.Mode = "docker"

	got, err := resolveDownloadedFilePath(cfg, dir, "/downloads/video.mp4\n")
	if err != nil {
		t.Fatalf("resolveDownloadedFilePath() error = %v", err)
	}
	if got != filePath {
		t.Fatalf("path = %q, want %q", got, filePath)
	}
}

func TestEnforceVideoDownloadSize(t *testing.T) {
	cfg := &config.Config{}
	cfg.Tools.VideoDownload.MaxFileSizeMB = 1
	if err := enforceVideoDownloadSize(cfg, 1024*1024); err != nil {
		t.Fatalf("size at limit returned error: %v", err)
	}
	if err := enforceVideoDownloadSize(cfg, 1024*1024+1); err == nil {
		t.Fatal("expected error for oversized file")
	}
}

func TestVideoDownloadModeDefaultsToDocker(t *testing.T) {
	cfg := &config.Config{}
	if got := videoDownloadMode(cfg); got != "docker" {
		t.Fatalf("mode = %q, want docker", got)
	}
	cfg.Tools.VideoDownload.Mode = "native"
	if got := videoDownloadMode(cfg); got != "native" {
		t.Fatalf("mode = %q, want native", got)
	}
}
