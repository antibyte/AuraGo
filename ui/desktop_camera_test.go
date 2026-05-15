package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVirtualDesktopCameraCaptureButtonCentersRing(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile(filepath.Join("css", "camera.css"))
	if err != nil {
		t.Fatalf("read camera css: %v", err)
	}
	css := string(raw)

	button := virtualDesktopCameraCSSBlock(t, css, ".camera-btn")
	capture := virtualDesktopCameraCSSBlock(t, css, ".camera-btn-capture")
	ring := virtualDesktopCameraCSSBlock(t, css, ".camera-capture-ring")

	virtualDesktopCameraRequireCSS(t, button, "padding: 0;")
	virtualDesktopCameraRequireCSS(t, button, "box-sizing: border-box;")
	virtualDesktopCameraRequireCSS(t, capture, "display: grid;")
	virtualDesktopCameraRequireCSS(t, capture, "place-items: center;")
	virtualDesktopCameraRequireCSS(t, capture, "box-sizing: border-box;")
	virtualDesktopCameraRequireCSS(t, ring, "display: block;")
	virtualDesktopCameraRequireCSS(t, ring, "box-sizing: border-box;")
}

func virtualDesktopCameraCSSBlock(t *testing.T, css, selector string) string {
	t.Helper()

	start := strings.Index(css, selector+" {")
	if start < 0 {
		t.Fatalf("camera css missing %s block", selector)
	}
	blockStart := strings.Index(css[start:], "{")
	if blockStart < 0 {
		t.Fatalf("camera css missing opening brace for %s", selector)
	}
	blockStart += start
	blockEnd := strings.Index(css[blockStart:], "\n}")
	if blockEnd < 0 {
		t.Fatalf("camera css missing closing brace for %s", selector)
	}
	return css[blockStart : blockStart+blockEnd]
}

func virtualDesktopCameraRequireCSS(t *testing.T, block, want string) {
	t.Helper()

	if !strings.Contains(block, want) {
		t.Fatalf("camera capture button CSS block missing %q in:\n%s", want, block)
	}
}
