package ui

import (
	"strings"
	"testing"
)

func TestDesktopWriterAppMarkers(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/apps/writer.js")
	for _, marker := range []string{
		"WriterApp.render",
		"WriterApp.dispose",
		"dispose(windowId);",
		"instAfter.quill = editor",
		"/api/desktop/office/document",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("writer.js missing marker %q", marker)
		}
	}
}

func TestDesktopSheetsAppMarkers(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/apps/sheets.js")
	for _, marker := range []string{
		"SheetsApp.render",
		"SheetsApp.dispose",
		"dispose(windowId);",
		"closeSheetContextMenu",
		"/api/desktop/office/workbook",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("sheets.js missing marker %q", marker)
		}
	}
}

func TestDesktopViewerAppMarkers(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/apps/viewer.js")
	for _, marker := range []string{
		"ViewerApp.render",
		"ViewerApp.dispose",
		"dispose(windowId);",
		"desktop.viewer_pdfjs_unavailable",
		"pdfLoadingTask",
		"/api/desktop/viewer/content",
		"window.DOMPurify",
		"DOMPurify.sanitize",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("viewer.js missing marker %q", marker)
		}
	}
}

func TestDesktopViewer3DAppMarkers(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/apps/viewer-3d.js")
	for _, marker := range []string{
		"Viewer3DApp",
		"desktop.stl_viewer",
		"desktop.stl_wireframe",
		"resizeObserver.disconnect",
		"THREE.STLLoader",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("viewer-3d.js missing marker %q", marker)
		}
	}
}
