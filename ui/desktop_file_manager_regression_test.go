package ui

import (
	"strings"
	"testing"
)

func TestDesktopFileManagerAvoidsKnownLayoutRegressions(t *testing.T) {
	t.Parallel()

	jsBytes, err := Content.ReadFile("js/desktop/file-manager.js")
	if err != nil {
		t.Fatalf("file manager script missing from embedded UI: %v", err)
	}
	js := string(jsBytes)
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

	cssBytes, err := Content.ReadFile("css/desktop.css")
	if err != nil {
		t.Fatalf("desktop stylesheet missing from embedded UI: %v", err)
	}
	css := string(cssBytes)
	for _, want := range []string{
		".fm-search-bar[hidden]",
		"grid-template-columns: minmax(160px, 1fr) 80px 110px 80px;",
		".fm-drop-overlay.visible",
		".fm-modal-overlay",
	} {
		if !strings.Contains(css, want) {
			t.Fatalf("desktop stylesheet missing file manager layout rule %q", want)
		}
	}
}
