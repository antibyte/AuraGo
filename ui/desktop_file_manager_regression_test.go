package ui

import (
	"strings"
	"testing"
)

func TestDesktopFileManagerAvoidsKnownLayoutRegressions(t *testing.T) {
	t.Parallel()

	js := readDesktopAssetText(t, "js/desktop/file-manager.js")
	for _, bad := range []string{"'14px'", "'16px'", "'18px'", "'32px'", "'40px'", "'48px'"} {
		if strings.Contains(js, bad) {
			t.Fatalf("file manager should pass numeric icon sizes, found %s", bad)
		}
	}
	for _, want := range []string{
		"function t(key, fallback, vars)",
		"renderFileContent()",
		"attachFileItemEvents(root)",
	} {
		if !strings.Contains(js, want) {
			t.Fatalf("file manager missing regression guard behavior %q", want)
		}
	}

	css := readAllDesktopCSS(t)
	for _, want := range []string{
		".fm-search-bar[hidden]",
		"grid-template-columns: minmax(160px, 1fr) 80px 110px 80px;",
		".fm-drop-overlay.visible",
		".fm-modal-overlay",
		"container-type: inline-size",
		"@container fm-pane (max-width: 520px)",
		"@container file-manager (max-width: 640px)",
	} {
		if !strings.Contains(css, want) {
			t.Fatalf("desktop stylesheet missing file manager layout rule %q", want)
		}
	}
	fmCSS := readDesktopAssetText(t, "css/desktop-app-file-manager.css")
	if strings.Contains(fmCSS, "content-visibility: auto") {
		t.Fatal("file manager list rows must not use content-visibility: auto; it drops theme icons after resize")
	}
	if strings.Contains(fmCSS, ".fm-toolbtn:disabled {\n    opacity: 0.3;") {
		t.Fatal("disabled file-manager toolbar buttons must keep their icons visible")
	}
	if !strings.Contains(fmCSS, ".fm-toolbtn:disabled .fm-btn-icon") {
		t.Fatal("disabled file-manager toolbar icons need a dedicated muted opacity")
	}

	foundation := readDesktopAssetText(t, "js/desktop/core/desktop-foundation.js")
	for _, want := range []string{
		"Videos: 'video'",
		"Pets: 'heart'",
		"Widgets: 'apps'",
		"const base = String(name || '').split(/[\\\\/]/).filter(Boolean).pop()",
	} {
		if !strings.Contains(foundation, want) {
			t.Fatalf("directory icon lookup missing marker %q", want)
		}
	}

	render := readDesktopAssetText(t, "js/desktop/file-manager/core-render.js")
	if !strings.Contains(render, "iconForDirectory(baseName(dir) || dir)") {
		t.Fatal("sidebar icons must resolve from the directory basename")
	}

	chevron := readDesktopAssetText(t, "img/papirus/icons/chevron-right.svg")
	if !strings.Contains(chevron, `d="M 2,7 H 10 L 6.5,3.5 8,2 14,8 8,14 6.5,12.5 10,9 H 2 Z"`) {
		t.Fatal("papirus chevron-right must stay a centered next-arrow, not an empty-looking go-next shaft")
	}
}
