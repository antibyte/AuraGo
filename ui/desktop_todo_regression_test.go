package ui

import (
	"encoding/json"
	"os"
	"path/filepath"
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

func TestDesktopTodoReadonlyDisablesMutations(t *testing.T) {
	t.Parallel()
	source := readDesktopAssetText(t, "js/desktop/apps/planning-gallery-music.js")
	for _, marker := range []string{
		"const todoReadonly = desktopReadonly()",
		"desktop.todo_readonly",
		"if (todoReadonly) return",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("todo readonly marker missing %q", marker)
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

func TestDesktopTodoMutationsKeepTheFormAndReportFailure(t *testing.T) {
	t.Parallel()
	source := readDesktopAssetText(t, "js/desktop/apps/planning-gallery-music.js")
	for _, marker := range []string{
		"function runTodoAction(",
		"function detailIsDirty(",
		"t('desktop.request_failed')",
		"replaceDetail",
		"desktop.todo_empty",
		"card.classList.toggle('active'",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("todo mutation marker missing %q", marker)
		}
	}
	if strings.Contains(source, "desktop.empty_folder") && strings.Contains(source, "vd-todo-list") {
		t.Fatal("todo empty list still uses the folder string")
	}
	css := readDesktopAssetText(t, "css/desktop-app-planning.css")
	for _, marker := range []string{
		"container-type: inline-size",
		"@container todo (max-width: 760px)",
		"color-mix(in srgb, var(--vd-text) 12%, transparent)",
	} {
		if !strings.Contains(css, marker) {
			t.Fatalf("todo layout marker missing %q", marker)
		}
	}
}

func TestDesktopTodoBundleCarriesTheCivilDay(t *testing.T) {
	t.Parallel()
	bundle := readDesktopAssetText(t, "js/desktop/main.js")
	for _, marker := range []string{
		"function todoDueCivilDate(",
		"desktop.todo_empty",
		"desktop.todo_readonly",
		"const TODO_REFRESH_MS = 60000",
	} {
		if !strings.Contains(bundle, marker) {
			t.Fatalf("desktop bundle missing %q", marker)
		}
	}
}

// A reload rebuilds the panes of the todo window. The focused field, the caret inside it and the checklist item the
// user was typing in must survive that, and the open form must say that it differs from the stored task.
func TestDesktopTodoPolishKeepsTheReadingPosition(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/apps/planning-gallery-music.js")
	for _, marker := range []string{
		"function captureTodoFocus(",
		"function restoreTodoFocus(",
		"function setTodoNotice(",
		"function syncTodoDirtyState(",
		"function revertTodoEdits(",
		"function renderTodoList(",
		"function updateTodoFilterCounts(",
		"function updateTodoListMeta(",
		"function todoMatchesQuery(",
		"restoreTodoFocus(pane, focus)",
		"restoreTodoFocus(list, listFocus)",
		"vd-todo-notice",
		"vd-todo-filter-count",
		"vd-todo-search-input",
		"vd-todo-card-chips",
		"vd-todo-chip",
		"vd-todo-dirty",
		"vd-todo-empty",
		"desktop.todo_unsaved_changes",
		"desktop.todo_saved",
		"desktop.todo_created",
		"desktop.todo_completed",
		"desktop.todo_reopened",
		"desktop.todo_deleted",
		"desktop.todo_failed",
		"desktop.todo_no_results",
		"desktop.todo_checklist_empty",
		"desktop.todo_title_label",
		"desktop.todo_progress_aria",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("todo polish marker missing %q", marker)
		}
	}

	css := readDesktopAssetText(t, "css/desktop-app-planning.css")
	for _, marker := range []string{
		"--todo-ok:",
		"--todo-busy:",
		"--todo-warn:",
		"--todo-bad:",
		".vd-todo-chip",
		".vd-todo-notice",
		".vd-todo-filter-count",
		".vd-todo-empty",
		".vd-todo-dirty",
		".vd-todo-card-title",
		":focus-visible",
		"prefers-reduced-motion",
	} {
		if !strings.Contains(css, marker) {
			t.Fatalf("todo polish style marker missing %q", marker)
		}
	}
}

// The polish adds eighteen user-visible strings. Every locale carries them, the placeholders survive the
// translation, and a locale that copies the English text is a fallback rather than a translation.
func TestDesktopTodoTranslations(t *testing.T) {
	t.Parallel()

	newKeys := []string{
		"desktop.todo_checklist_empty",
		"desktop.todo_clear_search",
		"desktop.todo_count",
		"desktop.todo_created",
		"desktop.todo_completed",
		"desktop.todo_deleted",
		"desktop.todo_dismiss",
		"desktop.todo_empty_hint",
		"desktop.todo_empty_title",
		"desktop.todo_failed",
		"desktop.todo_no_results",
		"desktop.todo_progress_aria",
		"desktop.todo_reopened",
		"desktop.todo_saved",
		"desktop.todo_search_placeholder",
		"desktop.todo_select_hint",
		"desktop.todo_title_label",
		"desktop.todo_unsaved_changes",
	}
	placeholders := map[string]string{
		"desktop.todo_count":         "{{count}}",
		"desktop.todo_no_results":    "{{query}}",
		"desktop.todo_progress_aria": "{{percent}}",
	}

	var english map[string]string
	for _, locale := range []string{"en", "de", "cs", "da", "el", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		b, err := os.ReadFile(filepath.Join("lang", "desktop", locale+".json"))
		if err != nil {
			t.Fatal(err)
		}
		labels := map[string]string{}
		if err := json.Unmarshal(b, &labels); err != nil {
			t.Fatal(err)
		}
		for _, key := range newKeys {
			if strings.TrimSpace(labels[key]) == "" {
				t.Errorf("%s missing %s", locale, key)
			}
		}
		for key, want := range placeholders {
			if !strings.Contains(labels[key], want) {
				t.Errorf("%s %s lost the %s placeholder", locale, key, want)
			}
		}
		if locale == "en" {
			english = labels
			continue
		}
		if english == nil {
			t.Fatal("en.json was not read first")
		}
		copies := 0
		for _, key := range newKeys {
			if labels[key] == english[key] {
				copies++
			}
		}
		if copies*2 >= len(newKeys) {
			t.Errorf("%s copies %d of the %d new todo strings from English", locale, copies, len(newKeys))
		}
	}
}
