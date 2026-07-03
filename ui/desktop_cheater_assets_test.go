package ui

import (
	"encoding/json"
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
		"Ctrl+Shift+K",
		"data-state=\"library\"",
		"renderLibrary",
		"loadSearchIndex",
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

func TestDesktopCheaterLibraryStyles(t *testing.T) {
	t.Parallel()

	css := readDesktopAssetText(t, "css/desktop-app-cheater.css")
	for _, marker := range []string{
		".cheater-library-list",
		".cheater-library-card-btn",
		".cheater-library-search input:focus-visible",
		".cheater-loading-card",
		"data-state=\"library\"",
	} {
		if !strings.Contains(css, marker) {
			t.Fatalf("cheater library CSS missing marker %q", marker)
		}
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

func TestDesktopCheaterStylesUseReadableDesktopScopedTokens(t *testing.T) {
	t.Parallel()

	css := readDesktopAssetText(t, "css/desktop-app-cheater.css")
	for _, marker := range []string{
		"--cheater-text: var(--ds-color-fg-primary",
		"--cheater-muted: var(--ds-color-fg-muted",
		"--cheater-surface: var(--ds-color-surface-1",
		"--cheater-modal-surface: var(--ds-color-surface-3",
		"background: var(--cheater-surface",
		"color: var(--cheater-text",
		".cheater-field input::placeholder",
		".cheater-primary:disabled",
		".cheater-secondary:hover",
	} {
		if !strings.Contains(css, marker) {
			t.Fatalf("cheater CSS missing readable desktop token marker %q", marker)
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
		"e.shiftKey",
		"SPOTLIGHT_KEY",
		"e.key === 'n'",
		"window.CheaterSpotlight.open",
		"window.CheaterApp.openCreateModal",
		"/api/cheatsheets",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("cheater wiring missing JS marker %q", marker)
		}
	}
	if !strings.Contains(source, "openSheet,") && !strings.Contains(source, "openSheet:") {
		t.Fatalf("cheater state does not expose openSheet to spotlight selections")
	}
	if !strings.Contains(source, "loadSheet(nextState, entry.id)") {
		t.Fatalf("cheater spotlight selections must load full sheets before opening the editor")
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
		"function renderPreview",
		"function schedulePreview",
		"window.marked",
		"window.hljs",
		"window.DOMPurify",
		"DOMPurify.sanitize",
		"data-preview",
		"data-view-mode",
		"setViewMode",
		"PREVIEW_DEBOUNCE_MS",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("cheater preview render missing JS marker %q", marker)
		}
	}

	css := readAllDesktopAppCSS(t)
	for _, marker := range []string{
		".cheater-preview",
		".cheater-preview pre",
		".cheater-view-toggle",
		".cheater-view-btn.is-active",
		".cheater-toolbar",
		".cheater-tool-btn",
	} {
		if !strings.Contains(css, marker) {
			t.Fatalf("cheater preview CSS missing marker %q", marker)
		}
	}

	loader := readDesktopAssetText(t, "js/desktop/core/module-loader.js")
	if !strings.Contains(loader, "marked.min.js") || !strings.Contains(loader, "highlight.min.js") || !strings.Contains(loader, "purify.min.js") {
		t.Fatalf("cheater entry missing vendor libraries")
	}
	if !strings.Contains(loader, "hljs-github.min.css") {
		t.Fatalf("cheater entry missing hljs CSS")
	}
	if !strings.Contains(loader, "cheater-toolbar.js") {
		t.Fatalf("cheater entry missing toolbar module")
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

func TestDesktopCheaterAttachments(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/apps/cheater-attachments.js")
	for _, marker := range []string{
		"window.CheaterAttachments",
		"window.CheaterAttachments.open",
		"function openAttachmentPanel",
		"function uploadFile",
		"function deleteAttachment",
		"function showToast",
		"data-action=\"close\"",
		"data-action=\"delete\"",
		"cheater-attach-drop",
		"/attachments",
		"method: 'POST'",
		"method: 'DELETE'",
		"10 * 1024 * 1024",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("cheater attachments missing JS marker %q", marker)
		}
	}

	wiring := readDesktopAssetText(t, "js/desktop/apps/cheater.js")
	if !strings.Contains(wiring, "window.CheaterAttachments.open") {
		t.Fatalf("cheater.js does not wire the attach button")
	}

	loader := readDesktopAssetText(t, "js/desktop/core/module-loader.js")
	if !strings.Contains(loader, "cheater-attachments.js") {
		t.Fatalf("module-loader missing cheater-attachments.js")
	}

	css := readAllDesktopAppCSS(t)
	for _, marker := range []string{
		".cheater-attach-panel",
		".cheater-attach-drop",
		".cheater-attach-item",
		".cheater-attach-delete",
		".cheater-toast",
	} {
		if !strings.Contains(css, marker) {
			t.Fatalf("cheater attachments CSS missing marker %q", marker)
		}
	}
}

func TestDesktopCheaterDeleteAndPolling(t *testing.T) {
	t.Parallel()

	spotlight := readDesktopAssetText(t, "js/desktop/apps/cheater-spotlight.js")
	for _, marker := range []string{
		"function showContextMenu",
		"function deleteEntry",
		"contextmenu",
		"data-action=\"delete\"",
		"method: 'DELETE'",
		"setTimeout(commit, 5000)",
		"data-undo",
	} {
		if !strings.Contains(spotlight, marker) {
			t.Fatalf("cheater spotlight missing delete marker %q", marker)
		}
	}

	main := readDesktopAssetText(t, "js/desktop/apps/cheater.js")
	for _, marker := range []string{
		"function startPolling",
		"function stopPolling",
		"function pollRemote",
		"function showUpdateBadge",
		"POLL_INTERVAL_MS = 30000",
		"data-update-badge",
		"startPolling(state)",
		"if (state.pollTimer) clearInterval",
	} {
		if !strings.Contains(main, marker) {
			t.Fatalf("cheater polling missing JS marker %q", marker)
		}
	}

	css := readAllDesktopAppCSS(t)
	for _, marker := range []string{
		".cheater-context-menu",
		".cheater-update-bar",
	} {
		if !strings.Contains(css, marker) {
			t.Fatalf("cheater delete/polling CSS missing marker %q", marker)
		}
	}
}

func TestDesktopCheaterTranslations(t *testing.T) {
	t.Parallel()

	required := []string{
		"desktop.app_cheater",
		"cheater.app_name",
		"cheater.empty_subtitle",
		"cheater.empty_cta",
		"cheater.empty_hint",
		"cheater.library_loading",
		"cheater.library_search_placeholder",
		"cheater.library_count",
		"cheater.library_no_results",
		"cheater.library_open_spotlight",
		"cheater.untitled_sheet",
		"cheater.back",
		"cheater.attachments",
		"cheater.chars",
		"cheater.hover_help",
		"cheater.saving",
		"cheater.saved",
		"cheater.saved_ago",
		"cheater.save_error",
		"cheater.spotlight_placeholder",
		"cheater.spotlight_hint",
		"cheater.create_title",
		"cheater.field_title",
		"cheater.field_description",
		"cheater.field_tags",
		"cheater.field_template",
		"cheater.cancel",
		"cheater.create_submit",
		"cheater.delete",
		"cheater.undo",
		"cheater.template.empty",
		"cheater.template.deployment",
		"cheater.template.debug",
		"cheater.template.routine",
		"cheater.template.api",
		"cheater.template.backup",
		"cheater.attach_empty",
		"cheater.attach_drop_hint",
		"cheater.agent_badge",
		"cheater.update_available",
		"cheater.update_apply",
		"cheater.update_summary",
		"cheater.update_changed",
		"cheater.update_added",
		"cheater.update_removed",
		"cheater.close",
		"cheater.save",
		"cheater.creating",
		"cheater.editor_placeholder",
		"cheater.editor_help",
		"cheater.words",
		"cheater.lines",
		"cheater.view_mode_edit",
		"cheater.view_mode_split",
		"cheater.view_mode_preview",
		"cheater.field_title_required",
		"cheater.spotlight_create_fallback",
		"cheater.confirm_delete_title",
		"cheater.confirm_delete_text",
		"cheater.toolbar.bold",
		"cheater.toolbar.italic",
		"cheater.toolbar.code",
		"cheater.toolbar.inline_code",
		"cheater.toolbar.link",
		"cheater.toolbar.link_icon",
		"cheater.toolbar.heading",
		"cheater.toolbar.list",
		"cheater.toolbar.ordered_list",
		"cheater.toolbar.quote",
		"cheater.toolbar.divider",
		"cheater.toolbar.help",
		"cheater.help.save",
		"cheater.help.spotlight",
		"cheater.help.new_sheet",
		"cheater.help.cycle_view",
		"cheater.help.indent",
		"cheater.help.close",
	}

	languages := []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"}
	for _, lang := range languages {
		path := filepath.Join("lang", "desktop", lang+".json")
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		var doc map[string]interface{}
		if err := json.Unmarshal(data, &doc); err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		for _, key := range required {
			if _, ok := doc[key]; !ok {
				t.Errorf("language %s missing key %q", lang, key)
			}
		}
	}
}
