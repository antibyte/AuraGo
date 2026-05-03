package ui

import (
	"strings"
	"testing"
)

func TestVirtualDesktopFooterOwnsSystemControls(t *testing.T) {
	t.Parallel()

	htmlBytes, err := Content.ReadFile("desktop.html")
	if err != nil {
		t.Fatalf("desktop template missing from embedded UI: %v", err)
	}
	html := string(htmlBytes)

	if strings.Contains(html, `class="vd-topbar"`) {
		t.Fatal("virtual desktop should not render the old header/topbar")
	}
	for _, marker := range []string{
		`<section class="vd-taskbar">`,
		`id="vd-clock"`,
		`id="radialMenuAnchor" class="vd-radial-anchor"`,
		`class="vd-taskbar-system"`,
	} {
		if !strings.Contains(html, marker) {
			t.Fatalf("desktop footer is missing system control marker %q", marker)
		}
	}
}

func TestVirtualDesktopMaximizeUsesFullWorkspace(t *testing.T) {
	t.Parallel()

	jsBytes, err := Content.ReadFile("js/desktop/main.js")
	if err != nil {
		t.Fatalf("desktop shell missing from embedded UI: %v", err)
	}
	js := string(jsBytes)
	for _, marker := range []string{
		"win.style.left = '0';",
		"win.style.top = '0';",
		"win.style.width = Math.max(WINDOW_MIN_W, bounds.width) + 'px';",
		"win.style.height = Math.max(WINDOW_MIN_H, bounds.height) + 'px';",
	} {
		if !strings.Contains(js, marker) {
			t.Fatalf("desktop maximize logic is missing full-workspace marker %q", marker)
		}
	}
}

func TestCodeStudioAgentPanelGetsVisibleColumn(t *testing.T) {
	t.Parallel()

	cssBytes, err := Content.ReadFile("css/code-studio.css")
	if err != nil {
		t.Fatalf("Code Studio stylesheet missing from embedded UI: %v", err)
	}
	css := string(cssBytes)
	for _, marker := range []string{
		`grid-template-columns: var(--cs-sidebar-width) minmax(0, 1fr) minmax(320px, 360px);`,
		".code-studio-main {\n    position: relative;\n    z-index: 1;",
		".code-studio-chat {\n    position: relative;\n    z-index: 2;",
		`min-width: 320px;`,
	} {
		if !strings.Contains(css, marker) {
			t.Fatalf("Code Studio agent panel CSS is missing visibility marker %q", marker)
		}
	}
}
