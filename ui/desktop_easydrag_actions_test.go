package ui

import (
	"regexp"
	"strings"
	"testing"
)

// W1b Part A, the menu lint. EasyDrag menus list action ids and render through ED.actions.items, so a label, a
// shortcut and the reason an entry is off live in easydrag-actions.js only. This test fails when another EasyDrag module
//   - gives a menu entry a `shortcut` or `shortcutHint` of its own,
//   - builds a menu entry (a line with `label:` or `labelKey:` and `action:`, `disabledHint:` or, with an `icon:`, a literal `items:`), or
//   - opens a context menu or sets window menus with anything but ED.actions.items(...).
//
// The one exception is the panel width menu of easydrag-workspace.js: it offers pixel sizes, not actions.
func TestEasyDragMenusUseTheActionRegistry(t *testing.T) {
	keyField := regexp.MustCompile(`\bshortcut(?:Hint)?\s*:`)
	labelField := regexp.MustCompile(`\blabel(?:Key)?\s*:`)
	// A menu entry's action is a function; a toast's action is an object ("action: {").
	entryField := regexp.MustCompile(`\baction\s*:\s*[(A-Za-z_$]|\bdisabledHint\s*:`)
	itemsField := regexp.MustCompile(`\bitems\s*:`)
	iconField := regexp.MustCompile(`\bicon\s*:`)
	menuCall := regexp.MustCompile(`\b(?:showContextMenu|setWindowMenus)\(`)
	calls := 0
	for _, name := range easyDragModules {
		if name == "easydrag-actions.js" || name == "easydrag-workspace.js" {
			continue
		}
		text := strings.ReplaceAll(readDesktopAssetText(t, "js/desktop/apps/"+name), "\r\n", "\n")
		for n, line := range strings.Split(text, "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), "//") {
				continue
			}
			if keyField.MatchString(line) {
				t.Errorf("%s:%d gives a menu entry its own shortcut: the registry draws shortcuts\n\t%s", name, n+1, strings.TrimSpace(line))
			}
			if labelField.MatchString(line) && (entryField.MatchString(line) || itemsField.MatchString(line) && iconField.MatchString(line) && !strings.Contains(line, ".items(")) {
				t.Errorf("%s:%d builds a menu entry with its own label: list the action id and render through ED.actions.items\n\t%s", name, n+1, strings.TrimSpace(line))
			}
		}
		for _, at := range menuCall.FindAllStringIndex(text, -1) {
			calls++
			args := easyDragCallArguments(text, at[1]-1)
			if !strings.Contains(args, "ED.actions.items(") {
				line := 1 + strings.Count(text[:at[0]], "\n")
				t.Errorf("%s:%d opens a menu without ED.actions.items(...) in its arguments", name, line)
			}
		}
	}
	// The lint must see the menus it guards: editor (window menus, more menu), interact (card and canvas menu), home (card menu)
	// and the shell (start page window menu).
	if calls < 5 {
		t.Errorf("the lint found %d menu calls, expected at least 5 (window menus, more menu, card menu, canvas menu, start page menus, home card menu)", calls)
	}
	workspace := readDesktopAssetText(t, "js/desktop/apps/easydrag-workspace.js")
	if got := len(menuCall.FindAllStringIndex(workspace, -1)); got != 1 {
		t.Errorf("easydrag-workspace.js opens %d menus: the panel width menu is the one exception", got)
	}
}

// easyDragCallArguments returns the argument text of the call whose "(" is at open: up to and including the matching ")".
// It skips string literals, so a parenthesis inside a label does not count.
func easyDragCallArguments(text string, open int) string {
	depth := 0
	for i := open; i < len(text); i++ {
		switch c := text[i]; c {
		case '\'', '"', '`':
			for i++; i < len(text) && text[i] != c; i++ {
				if text[i] == '\\' {
					i++
				}
			}
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return text[open : i+1]
			}
		}
	}
	return text[open:]
}
