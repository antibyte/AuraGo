package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDesktopCheaterAppRegistration(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/apps/cheater.js")
	for _, marker := range []string{
		"window.CheaterApp",
		"window.CheaterApp.render = render",
		"window.CheaterApp.dispose = dispose",
		"data-cheater",
		"cheater-empty",
		"data-empty",
		"cheater-empty-title",
		"data-action=\"create\"",
		"Cmd/Ctrl + K",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("cheater app missing JS marker %q", marker)
		}
	}
}

func TestDesktopCheaterModuleLoaderRegistration(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/core/module-loader.js")
	if !strings.Contains(source, "'cheater':") {
		t.Fatalf("module-loader missing cheater registration")
	}
	if !strings.Contains(source, "/css/desktop-app-cheater.css") {
		t.Fatalf("module-loader missing cheater styles")
	}
	if !strings.Contains(source, "/js/desktop/apps/cheater.js") {
		t.Fatalf("module-loader missing cheater script")
	}
}

func TestDesktopCheaterRouting(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/core/menus-and-routing.js")
	if !strings.Contains(source, "appId === 'cheater'") {
		t.Fatalf("menus-and-routing missing cheater dispatch")
	}
}

func TestDesktopCheaterIcon(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/core/desktop-foundation.js")
	if !strings.Contains(source, "cheater: 'cheater'") {
		t.Fatalf("desktop-foundation missing cheater icon mapping")
	}
}

func TestDesktopCheaterIconAsset(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile(filepath.Join("img", "chat-ui-icons", "cheater.svg"))
	if err != nil {
		t.Fatalf("read cheater icon: %v", err)
	}
	if !strings.Contains(string(data), "<svg") {
		t.Fatalf("cheater icon is not a valid svg")
	}
}

func TestDesktopCheaterEmptyStateStyles(t *testing.T) {
	t.Parallel()

	css := readAllDesktopAppCSS(t)
	for _, marker := range []string{
		".cheater-app",
		".cheater-empty",
		".cheater-empty-icon",
		".cheater-empty-title",
		".cheater-primary",
		"prefers-reduced-motion",
	} {
		if !strings.Contains(css, marker) {
			t.Fatalf("cheater CSS missing marker %q", marker)
		}
	}
}

func TestDesktopCheaterEditorRender(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/apps/cheater.js")
	for _, marker := range []string{
		"function renderEditor",
		"function loadSheet",
		"function openSheet",
		"function bindEditorEvents",
		"function bindBackButton",
		"data-state=\"editor\"",
		"cheater-header",
		"cheater-content",
		"cheater-footer",
		"data-title",
		"data-source",
		"data-save",
		"/api/cheatsheets/",
		"window.CheaterApp.openSheet = openSheet",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("cheater editor missing JS marker %q", marker)
		}
	}
}

func TestDesktopCheaterEditorStyles(t *testing.T) {
	t.Parallel()

	css := readAllDesktopAppCSS(t)
	for _, marker := range []string{
		".cheater-header",
		".cheater-title",
		".cheater-content",
		".cheater-source",
		".cheater-footer",
		".cheater-back",
		".cheater-save",
		".cheater-attach-btn",
		"data-state=\"editor\"",
	} {
		if !strings.Contains(css, marker) {
			t.Fatalf("cheater editor CSS missing marker %q", marker)
		}
	}
}

func TestDesktopCheaterAutoSave(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/apps/cheater.js")
	for _, marker := range []string{
		"SAVE_DEBOUNCE_MS = 500",
		"function commitSave",
		"function scheduleSave",
		"function flushSave",
		"function renderSaveStatus",
		"function formatRelative",
		"function updateSearchIndexEntry",
		"new AbortController",
		"method: 'PUT'",
		"e.key === 's'",
		"e.preventDefault()",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("cheater auto-save missing JS marker %q", marker)
		}
	}
}

func TestDesktopCheaterSpotlight(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/apps/cheater-spotlight.js")
	for _, marker := range []string{
		"window.CheaterSpotlight",
		"window.CheaterSpotlight.open = openSpotlight",
		"function createSpotlight",
		"function openSpotlight",
		"function fuzzyFilter",
		"function scoreEntry",
		"setAttribute('role'",
		"setAttribute('aria-modal'",
		"data-backdrop",
		"data-input",
		"data-results",
		"e.key === 'Escape'",
		"e.key === 'ArrowDown'",
		"e.key === 'Enter'",
		"action === 'create'",
		"action === 'open'",
		"MAX_VISIBLE",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("cheater spotlight missing JS marker %q", marker)
		}
	}
}

func TestDesktopCheaterSpotlightWiring(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/apps/cheater.js")
	for _, marker := range []string{
		"function bindGlobalShortcuts",
		"e.key === 'k'",
		"e.key === 'n'",
		"window.CheaterSpotlight.open",
		"window.CheaterApp.openCreateModal",
		"/api/cheatsheets",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("cheater wiring missing JS marker %q", marker)
		}
	}
}

func TestDesktopCheaterSpotlightStyles(t *testing.T) {
	t.Parallel()

	css := readAllDesktopAppCSS(t)
	for _, marker := range []string{
		".cheater-spotlight",
		".cheater-spotlight-backdrop",
		".cheater-spotlight-panel",
		".cheater-spotlight-input",
		".cheater-spotlight-row",
		".cheater-spotlight-row.is-selected",
		".cheater-spotlight-hint",
		".cheater-pill",
		"backdrop-filter",
	} {
		if !strings.Contains(css, marker) {
			t.Fatalf("cheater spotlight CSS missing marker %q", marker)
		}
	}
}

func TestDesktopCheaterInlineRender(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/apps/cheater.js")
	for _, marker := range []string{
		"function bindInlineRender",
		"function renderBlock",
		"function unrenderBlock",
		"function splitIntoBlocks",
		"function applyBlockStructure",
		"data-md-block",
		"window.marked",
		"window.hljs",
		"HOVER_DELAY_MS",
		"block.dataset.pinned",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("cheater inline render missing JS marker %q", marker)
		}
	}

	css := readAllDesktopAppCSS(t)
	for _, marker := range []string{
		".cheater-md-block",
		".cheater-md-block.is-rendered",
		".cheater-md-block pre",
	} {
		if !strings.Contains(css, marker) {
			t.Fatalf("cheater inline render CSS missing marker %q", marker)
		}
	}

	loader := readDesktopAssetText(t, "js/desktop/core/module-loader.js")
	if !strings.Contains(loader, "marked.min.js") || !strings.Contains(loader, "highlight.min.js") {
		t.Fatalf("cheater entry missing vendor libraries")
	}
	if !strings.Contains(loader, "hljs-github.min.css") {
		t.Fatalf("cheater entry missing hljs CSS")
	}
}

func TestDesktopCheaterCreateModal(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/apps/cheater.js")
	for _, marker := range []string{
		"function openCreateModal",
		"function collectTags",
		"function addTag",
		"function removeTag",
		"data-template-id",
		"data-action=\"submit\"",
		"data-action=\"cancel\"",
		"cheater-template-card",
		"cheater-tag-chips",
		"method: 'POST'",
		"window.CheaterApp.openCreateModal",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("cheater create modal missing JS marker %q", marker)
		}
	}

	templates := readDesktopAssetText(t, "js/desktop/apps/cheater-templates.js")
	for _, marker := range []string{
		"window.CheaterTemplates",
		"function listTemplates",
		"function templateById",
		"'empty'",
		"'deployment'",
		"'debug'",
		"'routine'",
		"'api'",
		"'backup'",
	} {
		if !strings.Contains(templates, marker) {
			t.Fatalf("cheater templates missing marker %q", marker)
		}
	}

	css := readAllDesktopAppCSS(t)
	for _, marker := range []string{
		".cheater-modal",
		".cheater-modal-backdrop",
		".cheater-modal-panel",
		".cheater-field",
		".cheater-template-grid",
		".cheater-template-card.is-selected",
		".cheater-secondary",
	} {
		if !strings.Contains(css, marker) {
			t.Fatalf("cheater modal CSS missing marker %q", marker)
		}
	}
}

func TestDesktopCheaterAgentBadge(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/apps/cheater.js")
	for _, marker := range []string{
		"function renderAgentBadge",
		"data-agent-badge",
		"state.sheet.last_used_at",
		"agent_badge",
		"🤖",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("cheater agent badge missing JS marker %q", marker)
		}
	}
}
