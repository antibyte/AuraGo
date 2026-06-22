package ui

import (
	"strings"
	"testing"
)

func TestDesktopThemeLayeringZIndexScale(t *testing.T) {
	t.Parallel()

	desktopCSS := readAllDesktopCSS(t)
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

func TestDesktopStandardThemeKeepsOfficeSurfacesDark(t *testing.T) {
	t.Parallel()

	desktopCSS := readAllDesktopCSS(t)
	for _, forbidden := range []string{
		".desktop-body[data-theme=\"standard\"],\n.desktop-body[data-theme=\"light\"]",
	} {
		if strings.Contains(desktopCSS, forbidden) {
			t.Fatalf("standard theme must not use light theme override %q", forbidden)
		}
	}

	for _, marker := range []string{
		".desktop-body[data-theme=\"standard\"]",
		".desktop-body[data-theme=\"standard\"] .vd-taskbar",
		".desktop-body[data-theme=\"standard\"] .vd-start-button",
		".desktop-body[data-theme=\"standard\"] .vd-start-menu",
		".desktop-body[data-theme=\"standard\"] .vd-window",
		".desktop-body[data-theme=\"standard\"] .vd-window-titlebar",
		"--vd-shell-taskbar:",
		"--vd-shell-window-chrome:",
		"--vd-shell-accent-secondary:",
		"--vd-bg: #11151c;",
		"--vd-surface: rgba(18, 24, 36, 0.92);",
		"--vd-text: #f6f7fb;",
		"--vd-control-bg: rgba(255, 255, 255, 0.08);",
		"--vd-control-hover: rgba(255, 255, 255, 0.12);",
		".desktop-body[data-theme=\"light\"]",
		".desktop-body[data-theme=\"light\"] .vd-window-titlebar",
		".vd-editor",
		"background: var(--ds-color-bg-raised, #181f2c);",
		"color: var(--ds-color-fg-primary, #f6f7fb);",
		".vd-editor textarea",
		".office-writer",
		"--vd-editor-bg: var(--ds-color-bg-raised, #181f2c);",
		"--vd-editor-text: var(--ds-color-fg-primary, #f6f7fb);",
		"--vd-editor-icon: var(--ds-color-fg-muted, #a8bbd0);",
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
