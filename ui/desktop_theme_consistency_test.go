package ui

import (
	"strings"
	"testing"
)

func TestDesktopThemesShareShellAndAppMaterials(t *testing.T) {
	t.Parallel()

	css := readAllDesktopCSS(t)
	for _, marker := range []string{
		"--vd-theme-app-bg:",
		"--vd-theme-panel-bg:",
		"--vd-theme-panel-bg-strong:",
		"--vd-theme-chrome-bg:",
		"--vd-theme-overlay-bg:",
		"--vd-theme-control-bg:",
		"--vd-theme-control-hover:",
		"--vd-theme-border:",
		"--vd-theme-radius-modal:",
		"--vd-theme-shadow-modal:",
		"--vd-theme-blur:",
		".desktop-body[data-theme=\"standard\"] :where(",
		".desktop-body[data-theme=\"fruity\"] :where(",
		".vd-file-dialog",
		".vd-shortcuts-modal",
		".vd-toast",
		".vd-window-content",
		".fm-modal",
		".fm-context-menu",
		".cs-modal",
		".cs-context-menu",
		".vd-qc-modal",
		".vd-store-modal",
		".cheater-modal-panel",
		".radio-toast",
		".teevee-toast",
		"--radio-glass: var(--vd-theme-panel-bg) !important;",
		"--cam-glass: var(--vd-theme-panel-bg) !important;",
		"--cs-panel: var(--vd-theme-panel-bg) !important;",
	} {
		if !strings.Contains(css, marker) {
			t.Fatalf("desktop theme consistency CSS is missing marker %q", marker)
		}
	}
}

func TestDesktopThemeBundleContainsConsistencyBridge(t *testing.T) {
	t.Parallel()

	bundle := readDesktopAssetText(t, "css/desktop-shell.bundle.css")
	for _, marker := range []string{
		"Theme material bridge: keep shell, apps, modals and toasts visually aligned",
		"--vd-theme-panel-bg-strong:",
		".desktop-body[data-theme=\"standard\"] :where(",
		".desktop-body[data-theme=\"fruity\"] :where(",
		".vd-modal-backdrop",
		".vd-file-dialog-backdrop",
		".vd-toast",
	} {
		if !strings.Contains(bundle, marker) {
			t.Fatalf("desktop shell bundle is missing theme consistency marker %q", marker)
		}
	}
}
