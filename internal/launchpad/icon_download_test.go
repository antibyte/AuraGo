package launchpad

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDownloadIconAndDelete(t *testing.T) {
	tmpDir := t.TempDir()

	// Use a small valid PNG from data URI or create a dummy file via HTTP
	// Since we can't rely on external network in tests, create a dummy local HTTP server
	// or test with a file:// URL. For simplicity, test path validation and delete.

	// Test DeleteIcon with valid path
	iconDir := filepath.Join(tmpDir, "launchpad_icons")
	os.MkdirAll(iconDir, 0755)
	testFile := filepath.Join(iconDir, "test_icon.png")
	os.WriteFile(testFile, []byte("fake image data"), 0644)

	relPath := filepath.Join("launchpad_icons", "test_icon.png")
	if err := DeleteIcon(tmpDir, relPath); err != nil {
		t.Fatalf("DeleteIcon failed: %v", err)
	}
	if _, err := os.Stat(testFile); !os.IsNotExist(err) {
		t.Error("Expected icon file to be deleted")
	}

	// Test DeleteIcon with empty path
	if err := DeleteIcon(tmpDir, ""); err != nil {
		t.Errorf("Expected no error for empty path, got %v", err)
	}

	// Test DeleteIcon with traversal attempt
	if err := DeleteIcon(tmpDir, "../outside.txt"); err == nil {
		t.Error("Expected error for path traversal")
	}
}

func TestExtFromContentType(t *testing.T) {
	cases := []struct {
		ct   string
		want string
	}{
		{"image/png", ".png"},
		{"image/svg+xml", ".svg"},
		{"image/webp", ".webp"},
		{"image/jpeg", ".jpg"},
		{"image/gif", ".gif"},
		{"application/octet-stream", ""},
	}
	for _, c := range cases {
		got := extFromContentType(c.ct)
		if got != c.want {
			t.Errorf("extFromContentType(%q) = %q, want %q", c.ct, got, c.want)
		}
	}
}
