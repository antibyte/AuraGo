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

// TestVirtualDesktopCameraViewportStacksMediaCentered guards the centering fix:
// the live video and the captured photo preview must overlap in the same
// absolutely positioned stack instead of sharing flex row space (which used to
// squeeze each into one half of the viewport).
func TestVirtualDesktopCameraViewportStacksMediaCentered(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile(filepath.Join("css", "camera.css"))
	if err != nil {
		t.Fatalf("read camera css: %v", err)
	}
	css := string(raw)

	video := virtualDesktopCameraCSSBlock(t, css, ".camera-video")
	preview := virtualDesktopCameraCSSBlock(t, css, ".camera-preview")
	clip := virtualDesktopCameraCSSBlock(t, css, ".camera-preview-video")

	for _, block := range []string{video, preview, clip} {
		virtualDesktopCameraRequireCSS(t, block, "position: absolute;")
		virtualDesktopCameraRequireCSS(t, block, "inset: 0;")
	}
}

// TestVirtualDesktopCameraHiddenGuard ensures the `hidden` attribute always wins
// over the app's display rules, and that the recording badge is centered.
func TestVirtualDesktopCameraHiddenGuardAndRecBadge(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile(filepath.Join("css", "camera.css"))
	if err != nil {
		t.Fatalf("read camera css: %v", err)
	}
	css := string(raw)

	guard := virtualDesktopCameraCSSBlock(t, css, ".camera-app [hidden]")
	virtualDesktopCameraRequireCSS(t, guard, "display: none !important;")

	badge := virtualDesktopCameraCSSBlock(t, css, ".camera-rec-badge")
	virtualDesktopCameraRequireCSS(t, badge, "left: 50%;")
	virtualDesktopCameraRequireCSS(t, badge, "translateX(-50%)")
}

// TestVirtualDesktopCameraAppKeepsContracts pins critical camera.js contracts:
// lazy app export, no template artifacts, MediaRecorder-based video capture and
// the bounded desktop upload endpoint.
func TestVirtualDesktopCameraAppKeepsContracts(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile(filepath.Join("js", "desktop", "apps", "camera.js"))
	if err != nil {
		t.Fatalf("read camera app: %v", err)
	}
	js := string(raw)

	if strings.Contains(js, "__omp_shell") {
		t.Fatal("camera app must not contain unsubmitted template artifacts (__omp_shell)")
	}
	for _, want := range []string{
		"window.CameraApp = { render: render, dispose: dispose }",
		"MediaRecorder",
		"/api/desktop/upload",
		"data-rec-time",
		"devicechange",
	} {
		if !strings.Contains(js, want) {
			t.Fatalf("camera app missing %q", want)
		}
	}
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
