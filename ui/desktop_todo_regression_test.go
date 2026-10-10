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
		"plannerJSON('/api/desktop/integrations/todos', 'POST'",
		"await load(result.id);",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("todo add flow missing marker %q", marker)
		}
	}
}

func TestDesktopTodoLoadsKeepTheNewestResponse(t *testing.T) {
	t.Parallel()
	source := readDesktopAssetText(t, "js/desktop/apps/planning-gallery-music.js")
	for _, marker := range []string{
		"const TODO_REFRESH_MS = 60000",
		"host.dataset.todoGeneration",
		"if (host.dataset.todoGeneration !== generation) return",
		"filtered.find(todo => todo.id === selectedID)",
		"registerWindowCleanup(id, () => {",
		"clearInterval(host._todoTimer)",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("todo load marker missing %q", marker)
		}
	}
	if strings.Contains(source, "renderTodo(id);") {
		t.Fatal("filter changes still rebuild the todo shell")
	}
}

func TestDesktopTodoReopenKeepsChecklistItems(t *testing.T) {
	t.Parallel()
	source := readDesktopAssetText(t, "js/desktop/apps/planning-gallery-music.js")
	if strings.Contains(source, "is_done: false") {
		t.Fatal("unchecking a task still clears checklist items")
	}
	for _, marker := range []string{
		"name=\"status\"",
		"status: 'open'",
		"status: form.status.value",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("todo status control missing %q", marker)
		}
	}
}

func TestDesktopTodoDueDateRoundTripUsesTheCivilDay(t *testing.T) {
	t.Parallel()
	source := readDesktopAssetText(t, "js/desktop/apps/planning-gallery-music.js")
	for _, marker := range []string{
		"function todoDueCivilDate(",
		"function todoIsOverdue(",
		"function todoDueFieldValue(",
		"T00:00:00Z",
		"if (!due.omit) body.due_date = due.value",
		`value="${esc(todoDueCivilDate(todo.due_date))}"`,
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("todo date round-trip missing %q", marker)
		}
	}
	knowledge := string(mustReadUIFile(t, "js/knowledge/todos.js"))
	if !strings.Contains(knowledge, "return value;") || !strings.Contains(knowledge, "T00:00:00Z") {
		t.Fatal("knowledge date helpers still convert a civil day through toISOString")
	}
}
