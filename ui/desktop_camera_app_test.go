package ui

import (
	"strings"
	"testing"
)

func TestDesktopCameraAppMarkers(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/apps/camera.js")
	for _, marker := range []string{
		"window.CameraApp",
		"function dispose(windowId)",
		"getUserMedia",
		"stopStream",
		"showCameraContextMenu",
		"wireContextMenuBoundary",
		"/api/desktop/upload",
		"/api/desktop/chat/stream",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("camera.js missing marker %q", marker)
		}
	}
}

func TestDesktopCameraRenderPassesContextMenuInBundle(t *testing.T) {
	t.Parallel()

	routing := readDesktopAssetText(t, "js/desktop/core/menus-and-routing.js")
	if !strings.Contains(routing, "showContextMenu") || !strings.Contains(routing, "wireContextMenuBoundary") {
		t.Fatal("menus-and-routing must pass showContextMenu and wireContextMenuBoundary to CameraApp.render")
	}
	cameraBlock := routing[strings.Index(routing, "if (appId === 'camera')"):]
	if end := strings.Index(cameraBlock, "if (appId === 'zipper')"); end > 0 {
		cameraBlock = cameraBlock[:end]
	}
	if !strings.Contains(cameraBlock, "showContextMenu") || !strings.Contains(cameraBlock, "openApp") {
		t.Fatal("camera render context must include showContextMenu and openApp")
	}
}

func TestDesktopCameraHandoffWiring(t *testing.T) {
	t.Parallel()

	shell := readDesktopAssetText(t, "js/desktop/core/window-shell-runtime.js")
	if !strings.Contains(shell, "delete windowContext.importPath") || !strings.Contains(shell, "VideoStudioApp.receiveImport") {
		t.Fatal("video studio handoff must stay off the stored window context and reuse the open window")
	}
	studio := readDesktopAssetText(t, "js/desktop/apps/video-studio-projects.js")
	if !strings.Contains(studio, "data-camera-import") || !strings.Contains(studio, "cameraImportKept") {
		t.Fatal("video studio must ask before importing a camera clip and must be able to cancel")
	}
}
