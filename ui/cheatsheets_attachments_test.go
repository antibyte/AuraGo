package ui

import (
	"strings"
	"testing"
)

func TestCheatsheetKnowledgeAttachmentModalContract(t *testing.T) {
	t.Parallel()

	html := readDesktopAssetText(t, "cheatsheets.html")
	for _, marker := range []string{
		`onclick="openKnowledgePicker()"`,
		`id="knowledge-picker-modal"`,
		`id="knowledge-picker-list"`,
		`onclick="confirmKnowledgePick()"`,
	} {
		if !strings.Contains(html, marker) {
			t.Fatalf("cheatsheet knowledge picker missing marker %q", marker)
		}
	}

	cheatsheetsJS := readDesktopAssetText(t, "js/cheatsheets/main.js")
	for _, marker := range []string{
		"async function openKnowledgePicker()",
		"async function confirmKnowledgePick()",
		"toggleKnowledgePick(this)",
		"/api/knowledge",
		"/api/cheatsheets/${csID}/attachments",
	} {
		if !strings.Contains(cheatsheetsJS, marker) {
			t.Fatalf("cheatsheet attachment JS missing marker %q", marker)
		}
	}
	if strings.Contains(cheatsheetsJS, "alert(") {
		t.Fatal("cheatsheet attachments UI must use modals/toasts instead of alert()")
	}
}

func TestCheatsheetListUsesSafeDeleteActionContract(t *testing.T) {
	t.Parallel()

	cheatsheetsJS := readDesktopAssetText(t, "js/cheatsheets/main.js")
	for _, marker := range []string{
		"data-action=\"delete-sheet\"",
		"data-sheet-id",
		"data-sheet-name",
		"bindSheetActionEvents()",
	} {
		if !strings.Contains(cheatsheetsJS, marker) {
			t.Fatalf("cheatsheet delete action missing safe marker %q", marker)
		}
	}
	if strings.Contains(cheatsheetsJS, "requestDelete('${escJs(s.id)}', '${esc(s.name)}')") {
		t.Fatal("cheatsheet delete action must not put HTML-escaped names inside inline JavaScript")
	}
}

func TestCheatsheetEditorTabsUseStableDataAttributes(t *testing.T) {
	t.Parallel()

	html := readDesktopAssetText(t, "cheatsheets.html")
	for _, marker := range []string{
		`data-editor-tab="edit"`,
		`data-editor-tab="preview"`,
	} {
		if !strings.Contains(html, marker) {
			t.Fatalf("cheatsheet editor tab missing marker %q", marker)
		}
	}

	cheatsheetsJS := readDesktopAssetText(t, "js/cheatsheets/main.js")
	if !strings.Contains(cheatsheetsJS, "el.dataset.editorTab === tab") {
		t.Fatal("cheatsheet editor tab state must use data-editor-tab, not localized text")
	}
	if strings.Contains(cheatsheetsJS, "textContent.toLowerCase().includes(tab)") {
		t.Fatal("cheatsheet editor tab state must not depend on localized button text")
	}
}

func TestSharedModalStackReactivatesNestedModal(t *testing.T) {
	t.Parallel()

	sharedJS := readDesktopAssetText(t, "shared.js")
	for _, marker := range []string{
		"function liftModalFromBackgroundInert(modal)",
		"const selfRestore = liftModalFromBackgroundInert(modal);",
		"const entry = { modal, trigger, hidden, selfRestore };",
		"entry.selfRestore()",
	} {
		if !strings.Contains(sharedJS, marker) {
			t.Fatalf("shared modal stack missing nested-modal inert restore marker %q", marker)
		}
	}
}
