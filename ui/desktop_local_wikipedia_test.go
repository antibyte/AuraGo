package ui

import (
	"strings"
	"testing"
)

func TestDesktopLocalWikipediaModuleContract(t *testing.T) {
	t.Parallel()

	app := rawDesktopAssetText(t, "js/desktop/apps/local-wikipedia.js")
	for _, marker := range []string{
		"window.LocalWikipediaApp = { render, dispose }",
		"const instances = new Map()",
		"function render(host, windowId, context)",
		"function dispose(windowId)",
		"const controller = new AbortController()",
		"st.controller.abort()",
		"instances.delete(windowId)",
		"const SUGGEST_DELAY_MS = 200",
		"const POLL_LOADING_MS = 1000",
		"const POLL_RETRY_MS = 5000",
		"function resumeReading()",
		"function backTarget()",
		"st.statusStale = true",
		"v.adoptLinks(doc, window.location.origin)",
		"v.toolbarTarget(",
		"button.setAttribute('tabindex'",
		"loadSuggestions(query, true)",
		"frame.contentWindow.location.replace(url)",
		`meta[name="aurago-local-wikipedia-error"]`,
		"const SETTINGS_URL = '/config#local_wikipedia'",
		"window.open(SETTINGS_URL, '_blank', 'noopener')",
	} {
		if !strings.Contains(app, marker) {
			t.Errorf("local-wikipedia.js missing %q", marker)
		}
	}
	views := rawDesktopAssetText(t, "js/desktop/apps/local-wikipedia-views.js")
	for _, marker := range []string{
		"window.LocalWikipediaViews = {",
		"status.readable === true",
		"status.loading === true",
		"doc.querySelectorAll('a, area')",
		"link.removeAttribute('ping')",
		"link.removeAttribute('attributionsrc')",
		"link.setAttribute('rel', 'noopener noreferrer')",
		"function toolbarTarget(",
		`sandbox="allow-same-origin allow-popups allow-popups-to-escape-sandbox"`,
		`role="combobox"`,
		`role="listbox"`,
		`role="toolbar"`,
	} {
		if !strings.Contains(views, marker) {
			t.Errorf("local-wikipedia-views.js missing %q", marker)
		}
	}
	for name, source := range map[string]string{"local-wikipedia.js": app, "local-wikipedia-views.js": views} {
		for _, forbidden := range []string{"allow-scripts", "eval(", "new Function", "localStorage", "alert(", "window.confirm(", ".recommendation"} {
			if strings.Contains(source, forbidden) {
				t.Errorf("%s must not contain %q", name, forbidden)
			}
		}
	}
	css := rawDesktopAssetText(t, "css/desktop-app-local-wikipedia.css")
	for _, marker := range []string{
		".lw-app {",
		"background: var(--vd-theme-app-bg);",
		"background: var(--vd-theme-chrome-bg);",
		"background: var(--vd-theme-control-bg);",
		"border: 1px solid var(--vd-theme-border);",
		":focus-visible",
		"@container (max-width: 640px)",
		"@media (prefers-reduced-motion: reduce)",
		"background: #ffffff;",
	} {
		if !strings.Contains(css, marker) {
			t.Errorf("desktop-app-local-wikipedia.css missing %q", marker)
		}
	}
	if strings.Contains(css, `data-theme="fruity"`) || strings.Contains(css, `data-theme="standard"`) {
		t.Error("shell theme tokens own standard and fruity light/dark; no per-theme overrides")
	}
}
