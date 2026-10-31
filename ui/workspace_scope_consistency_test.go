package ui

import (
	"strings"
	"testing"
)

func TestWorkspaceDensityToggleStyledGlobally(t *testing.T) {
	cfg := string(mustReadUIFile(t, "css/config-workspace.css"))
	if strings.Contains(cfg, `.pw-density-toggle`) {
		t.Fatal("density toggle styling must live in precision-pages.css, not config-workspace.css")
	}
	pp := string(mustReadUIFile(t, "css/precision-pages.css"))
	for _, want := range []string{".pw-page .pw-density-toggle:hover", ".pw-page .pw-density-toggle svg", ".pw-page .pw-density-toggle span", "font: inherit;"} {
		if !strings.Contains(pp, want) {
			t.Errorf("precision-pages.css missing %q", want)
		}
	}
	if !strings.Contains(string(mustReadUIFile(t, "js/precision/workspace.js")), "DENSITY_ICON") {
		t.Error("workspace.js must inject the density icon when absent")
	}
}

func TestWorkspaceThemeScopeLightDark(t *testing.T) {
	sc := string(mustReadUIFile(t, "js/shared/shared-chat.js"))
	for _, want := range []string{
		"function workspaceThemeFamily",
		"isWorkspaceThemeScope()",
		"workspaceThemeFamily(current) === 'light' ? 'dark' : 'light'",
	} {
		if !strings.Contains(sc, want) {
			t.Errorf("shared-chat.js missing %q", want)
		}
	}
	pre := string(mustReadUIFile(t, "js/shared/prepaint-theme.js"))
	for _, want := range []string{"themeScope === 'workspace'", "dataset.themeScope = 'workspace'"} {
		if !strings.Contains(pre, want) {
			t.Errorf("prepaint-theme.js missing %q", want)
		}
	}
}

func TestNotFoundPageSetsDocumentTitle(t *testing.T) {
	src := string(mustReadUIFile(t, "404.html"))
	if strings.Contains(src, "<title></title>") {
		t.Error("404.html must not have an empty title")
	}
	if !strings.Contains(src, "document.title = pageTitle") {
		t.Error("404.html must set document.title from I18N")
	}
}
