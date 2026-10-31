package ui

import (
	"regexp"
	"strings"
	"testing"
)

// Wave 3, content pages (knowledge, skills, cheatsheets, invasion): every dialog
// follows the shared dialog contract, deletes use the shared confirm component
// and the pages share the filter / empty / loading building blocks.

var wave3ContentPages = []struct {
	name     string
	template string
	js       []string
	css      string
}{
	{"knowledge", "knowledge.html", []string{"js/knowledge/main.js", "js/knowledge/appointments.js", "js/knowledge/todos.js"}, "css/knowledge.css"},
	{"skills", "skills.html", []string{"js/skills/main.js"}, "css/skills.css"},
	{"cheatsheets", "cheatsheets.html", []string{"js/cheatsheets/main.js"}, "css/cheatsheets.css"},
	{"invasion", "invasion_control.html", []string{"js/invasion/main.js"}, "css/invasion.css"},
}

func TestWave3ContentPagesDialogsFollowTheSharedContract(t *testing.T) {
	t.Parallel()

	overlayID := regexp.MustCompile(`<div class="modal-overlay[^"]*"[^>]*\bid="([^"]+)"`)
	for _, page := range wave3ContentPages {
		page := page
		t.Run(page.name, func(t *testing.T) {
			t.Parallel()
			html := renderUIPage(t, page.template)
			ids := overlayID.FindAllStringSubmatch(html, -1)
			if len(ids) == 0 {
				t.Fatalf("%s has no modal overlays", page.template)
			}
			for _, m := range ids {
				assertWave3Dialog(t, html, m[1])
			}
			if strings.Contains(html, `id="toast"`) {
				t.Errorf("%s still ships a static #toast element; showToast creates its own", page.template)
			}
			if strings.Contains(html, "modal-small") || strings.Contains(html, "modal-wide") || strings.Contains(html, "modal-lg\"") {
				t.Errorf("%s uses a legacy modal size class instead of .modal--sm/--lg/--xl", page.template)
			}
		})
	}
}

func TestWave3ContentPagesDeleteUsesSharedConfirm(t *testing.T) {
	t.Parallel()

	// The replaced page delete modals are gone.
	for template, ids := range map[string][]string{
		"knowledge.html":        {"delete-modal", "contact-delete-memories"},
		"skills.html":           {"delete-skill-modal", "delete-files-checkbox"},
		"cheatsheets.html":      {"delete-modal"},
		"invasion_control.html": {"delete-modal", "delete-confirm-input"},
	} {
		html := renderUIPage(t, template)
		for _, id := range ids {
			if strings.Contains(html, `id="`+id+`"`) {
				t.Errorf("%s still has the page-level delete element #%s", template, id)
			}
		}
	}

	cheatsheets := normalizeAssetText(mustReadUIFile(t, "js/cheatsheets/main.js"))
	for _, marker := range []string{"showConfirmDialog({", "danger: true,", "onConfirm: async () => {"} {
		if !strings.Contains(cheatsheets, marker) {
			t.Errorf("cheatsheets delete missing %q", marker)
		}
	}

	invasion := normalizeAssetText(mustReadUIFile(t, "js/invasion/main.js"))
	for _, marker := range []string{
		"showConfirmDialog({", "danger: true,", "requireText: name,",
		"requireTextHint: t('invasion.type_name_confirm'),",
	} {
		if !strings.Contains(invasion, marker) {
			t.Errorf("invasion delete missing type-to-confirm marker %q", marker)
		}
	}

	skills := normalizeAssetText(mustReadUIFile(t, "js/skills/main.js"))
	for _, marker := range []string{"showConfirmDialog({", "danger: true,", "checkbox: { label: t('skills.delete_files'), checked: true },"} {
		if !strings.Contains(skills, marker) {
			t.Errorf("skills delete missing %q", marker)
		}
	}

	knowledge := normalizeAssetText(mustReadUIFile(t, "js/knowledge/main.js"))
	if got := strings.Count(knowledge, "askDeleteKnowledgeItem("); got < 5 {
		t.Errorf("knowledge main.js routes %d delete entry points through askDeleteKnowledgeItem, want the definition plus 4", got)
	}
	if !strings.Contains(knowledge, "window.closeModal = function(...args) {") || !strings.Contains(knowledge, "_origCloseModal(...args);") {
		t.Error("knowledge closeModal override must forward all arguments (the close reason)")
	}
}

func TestWave3ContentPagesOpenOverlaysThroughTheSharedController(t *testing.T) {
	t.Parallel()

	manual := regexp.MustCompile(`getElementById\('[a-z-]*modal'\)\.classList\.(add|remove)\('active'\)|overlay\.classList\.(add|remove)\('active'\)|overlay\.style\.display`)
	for _, page := range wave3ContentPages {
		for _, js := range page.js {
			src := normalizeAssetText(mustReadUIFile(t, js))
			if loc := manual.FindString(src); loc != "" {
				t.Errorf("%s shows or hides a modal by hand (%q); use openModal/closeModal so onClose and Esc work", js, loc)
			}
		}
	}

	skills := normalizeAssetText(mustReadUIFile(t, "js/skills/main.js"))
	for _, marker := range []string{
		"openModal('agent-resource-path-modal', {",
		"openModal('agent-file-delete-modal', {",
		"closeModal('agent-resource-path-modal');",
		"closeModal('agent-file-delete-modal');",
		"onClose: resetDetailModalState",
		"onClose: resetCodeModalState",
	} {
		if !strings.Contains(skills, marker) {
			t.Errorf("skills.js missing %q", marker)
		}
	}
}

func TestWave3ContentPagesShareFilterEmptyAndLoadingBlocks(t *testing.T) {
	t.Parallel()

	skills := renderUIPage(t, "skills.html")
	if !strings.Contains(skills, "pw-filter-pill") {
		t.Error("skills filters must use .pw-filter-pill")
	}
	for _, tag := range regexp.MustCompile(`<button[^>]*\b(?:sk-filter-btn|sk-pill)\b[^>]*>`).FindAllString(skills, -1) {
		if !strings.Contains(tag, "pw-filter-pill") || strings.Contains(tag, "pw-badge") || !strings.Contains(tag, "aria-pressed=") {
			t.Errorf("skills filter button is not a pressed-state pw-filter-pill: %s", tag)
		}
	}

	knowledge := renderUIPage(t, "knowledge.html")
	for _, id := range []string{"appointments-filter", "todos-filter"} {
		if !regexp.MustCompile(`<label[^>]*\bfor="` + id + `"`).MatchString(knowledge) {
			t.Errorf("knowledge select #%s has no visible <label for>", id)
		}
	}

	for _, page := range wave3ContentPages {
		html := renderUIPage(t, page.template)
		for _, m := range regexp.MustCompile(`<div class="empty-state[^"]*"[^>]*>`).FindAllString(html, -1) {
			if !strings.Contains(m, "pw-empty-state") {
				t.Errorf("%s empty state is not on .pw-empty-state: %s", page.template, m)
			}
		}
	}

	for template, ids := range map[string][]string{
		"invasion_control.html": {"nests-loading", "eggs-loading"},
		"cheatsheets.html":      {"sheets-loading-user", "sheets-loading-agent"},
	} {
		html := renderUIPage(t, template)
		for _, id := range ids {
			tag := regexp.MustCompile(`<div[^>]*\bid="` + id + `"[^>]*>`).FindString(html)
			if tag == "" || !strings.Contains(tag, "pw-loading-state") || !strings.Contains(tag, `role="status"`) {
				t.Errorf("%s loading placeholder #%s missing or not a role=status .pw-loading-state: %q", template, id, tag)
			}
			if !strings.Contains(html, `pw-skeleton skeleton`) {
				t.Errorf("%s loading placeholder must combine .pw-skeleton with .skeleton for the shimmer", template)
			}
		}
	}
	for _, js := range []string{"js/invasion/main.js", "js/cheatsheets/main.js"} {
		src := normalizeAssetText(mustReadUIFile(t, js))
		if !strings.Contains(src, "toggle('is-hidden', !loading)") {
			t.Errorf("%s never hides the loading placeholder", js)
		}
	}
}

func TestWave3ContentPagesDropPageModalSizeRules(t *testing.T) {
	t.Parallel()

	for _, page := range wave3ContentPages {
		css := normalizeAssetText(mustReadUIFile(t, page.css))
		for _, legacy := range []string{".modal-wide {", ".modal-small {", ".modal-lg {", ".modal--sm {"} {
			if strings.Contains(css, legacy) {
				t.Errorf("%s still defines %q; sizes come from the shared scale in precision-pages.css", page.css, legacy)
			}
		}
		if regexp.MustCompile(`(?m)^\.pw-page\[[^\]]+\] \.modal \{[^}]*\bwidth:`).MatchString(css) {
			t.Errorf("%s sets a width on .modal; use the shared size scale", page.css)
		}
	}
}
