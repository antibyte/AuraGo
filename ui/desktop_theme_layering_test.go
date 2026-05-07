package ui

import (
	"strings"
	"testing"
)

func TestDesktopThemeLayeringZIndexScale(t *testing.T) {
	t.Parallel()

	desktopCSS := readDesktopAssetText(t, "css/desktop.css")
	for _, marker := range []string{
		"--vd-z-desktop:",
		"--vd-z-widget:",
		"--vd-z-window:",
		"--vd-z-dock:",
		"--vd-z-menu:",
		"--vd-z-modal:",
		"--vd-z-context-menu:",
		"--vd-z-toast:",
		".vd-context-menu",
		"z-index: var(--vd-z-context-menu);",
		".fm-context-menu",
		".fm-modal-overlay",
		"z-index: var(--vd-z-modal);",
		".office-sheet-context-menu",
		"z-index: var(--vd-z-menu);",
	} {
		if !strings.Contains(desktopCSS, marker) {
			t.Fatalf("desktop CSS is missing z-index scale marker %q", marker)
		}
	}

	codeStudioCSS := readDesktopAssetText(t, "css/code-studio.css")
	for _, marker := range []string{
		".cs-context-menu",
		"z-index: var(--vd-z-context-menu);",
		".cs-modal-backdrop",
		"z-index: var(--vd-z-modal);",
	} {
		if !strings.Contains(codeStudioCSS, marker) {
			t.Fatalf("Code Studio CSS is missing z-index scale marker %q", marker)
		}
	}

	radioCSS := readDesktopAssetText(t, "css/radio.css")
	if !strings.Contains(radioCSS, "z-index: var(--vd-z-toast);") {
		t.Fatal("Radio toast should use the shared desktop toast z-index")
	}
}

func TestDesktopStandardLightThemeAndWriterSurface(t *testing.T) {
	t.Parallel()

	desktopCSS := readDesktopAssetText(t, "css/desktop.css")
	for _, marker := range []string{
		"--vd-control-bg:",
		"--vd-control-hover:",
		"--vd-editor-bg: #ffffff;",
		"--vd-editor-text: #111827;",
		"--vd-editor-icon:",
		".desktop-body[data-theme=\"standard\"]",
		".desktop-body[data-theme=\"light\"]",
		".desktop-body[data-theme=\"standard\"] .vd-window-titlebar",
		".desktop-body[data-theme=\"light\"] .vd-window-titlebar",
		".office-writer",
		"background: var(--vd-editor-bg);",
		".office-writer .ql-stroke",
		"stroke: var(--vd-editor-icon);",
		".office-writer .ql-fill",
		"fill: var(--vd-editor-icon);",
		".office-writer-editor",
		".office-writer-editor.ql-container",
		".office-writer-editor .ql-editor",
		"background: var(--vd-editor-bg);",
		"color: var(--vd-editor-text);",
	} {
		if !strings.Contains(desktopCSS, marker) {
			t.Fatalf("desktop CSS is missing standard light/writer marker %q", marker)
		}
	}
}
