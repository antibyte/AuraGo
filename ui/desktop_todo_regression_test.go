package ui

import (
	"strings"
	"testing"
)

func TestDesktopTodoChecklistItemsAreEditableAndStable(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/main.js")
	for _, marker := range []string{
		"data-item-title",
		"data-todo-status-toggle",
		"function updateTodoItem(",
		"function setTodoDone(",
		"title: titleInput.value.trim()",
		"data-item-toggle",
		"todo-item-title",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("todo checklist editing missing marker %q", marker)
		}
	}

	css := readAllDesktopCSS(t)
	for _, marker := range []string{
		"grid-template-columns: 26px minmax(0, 1fr) auto;",
		"grid-template-columns: 22px minmax(0, 1fr) 30px;",
		".vd-todo-card-done",
		".vd-todo-item-title",
		".vd-todo-item-check",
	} {
		if !strings.Contains(css, marker) {
			t.Fatalf("todo checklist layout missing marker %q", marker)
		}
	}
}

func TestDesktopTodoAddUsesDefinedPlannerJSONHelper(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/main.js")
	for _, marker := range []string{
		"async function plannerJSON(",
		"plannerJSON('/api/todos', 'POST'",
		"await load(result.id);",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("todo add flow missing marker %q", marker)
		}
	}
}
