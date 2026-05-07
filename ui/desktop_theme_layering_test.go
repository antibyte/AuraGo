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

func TestDesktopStandardThemeStaysDarkAndWriterSurfaceStaysWhite(t *testing.T) {
	t.Parallel()

	desktopCSS := readDesktopAssetText(t, "css/desktop.css")
	for _, forbidden := range []string{
		".desktop-body[data-theme=\"standard\"],\n.desktop-body[data-theme=\"light\"]",
		".desktop-body[data-theme=\"standard\"] .vd-topbar",
		".desktop-body[data-theme=\"standard\"] .vd-taskbar",
		".desktop-body[data-theme=\"standard\"] .vd-window-titlebar",
		".desktop-body[data-theme=\"standard\"] .vd-window-content",
		".desktop-body[data-theme=\"standard\"] .vd-button",
	} {
		if strings.Contains(desktopCSS, forbidden) {
			t.Fatalf("standard theme must not use light theme override %q", forbidden)
		}
	}

	for _, marker := range []string{
		"--vd-bg: #11151c;",
		"--vd-surface: rgba(23, 28, 37, 0.88);",
		"--vd-text: #f6f7fb;",
		"--vd-control-bg: rgba(255, 255, 255, 0.08);",
		"--vd-control-hover: rgba(255, 255, 255, 0.12);",
		"--vd-editor-bg: #ffffff;",
		"--vd-editor-text: #111827;",
		"--vd-editor-icon:",
		".desktop-body[data-theme=\"light\"]",
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
			t.Fatalf("desktop CSS is missing standard dark/writer marker %q", marker)
		}
	}
}
