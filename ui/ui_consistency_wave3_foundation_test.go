package ui

import (
	"regexp"
	"strings"
	"testing"
)

// assertWave3Dialog checks that the modal overlay with the given id follows the
// wave-3 dialog contract: dialog semantics on the overlay (D5), a labelled h2
// title, a .modal-close button, a .modal-actions row and no legacy
// .modal-footer. html must be the rendered page (see renderUIPage).
func assertWave3Dialog(t *testing.T, html, id string) {
	t.Helper()

	openTag := regexp.MustCompile(`<[a-zA-Z][^>]*\bid="` + regexp.QuoteMeta(id) + `"[^>]*>`).FindStringIndex(html)
	if openTag == nil {
		t.Errorf("dialog %q not found", id)
		return
	}
	tag := html[openTag[0]:openTag[1]]
	region := html[openTag[0]:]
	// The region ends where the next overlay begins.
	if next := regexp.MustCompile(`class="[^"]*\bmodal-overlay\b`).FindStringIndex(region[len(tag):]); next != nil {
		region = region[:len(tag)+next[0]]
	}

	if !strings.Contains(tag, `role="dialog"`) {
		t.Errorf("dialog %q: overlay lacks role=\"dialog\": %s", id, tag)
	}
	if !strings.Contains(tag, `aria-modal="true"`) {
		t.Errorf("dialog %q: overlay lacks aria-modal=\"true\": %s", id, tag)
	}
	label := regexp.MustCompile(`aria-labelledby="([^"]+)"`).FindStringSubmatch(tag)
	if label == nil {
		t.Errorf("dialog %q: overlay lacks aria-labelledby: %s", id, tag)
	} else if !regexp.MustCompile(`<h2\b[^>]*\bid="` + regexp.QuoteMeta(label[1]) + `"`).MatchString(region) {
		t.Errorf("dialog %q: aria-labelledby %q does not point at an <h2> title inside the dialog", id, label[1])
	}
	if !regexp.MustCompile(`class="[^"]*\bmodal-close\b`).MatchString(region) {
		t.Errorf("dialog %q: no .modal-close button", id)
	}
	if !regexp.MustCompile(`class="[^"]*\bmodal-actions\b`).MatchString(region) {
		t.Errorf("dialog %q: no .modal-actions row", id)
	}
	if strings.Contains(region, "modal-footer") {
		t.Errorf("dialog %q: legacy .modal-footer used, use .modal-actions", id)
	}
}

// sharedModalCode returns the source of the shared confirm/alert dialog in
// shared-core.js (from the module-level hooks to the end of showAlert).
func sharedModalCode(t *testing.T) string {
	t.Helper()
	src := normalizeAssetText(mustReadUIFile(t, "js/shared/shared-core.js"))
	start := strings.Index(src, "let _sharedModalOverlay = null;")
	end := strings.Index(src, "function ensureHeadAsset")
	if start < 0 || end < start {
		t.Fatal("shared modal code section not found in shared-core.js")
	}
	return src[start:end]
}

func TestSharedModalCodeHasNoInlineStylesOrMagicValues(t *testing.T) {
	t.Parallel()
	code := sharedModalCode(t)
	for _, banned := range []string{`style="`, `style='`, "#000", "9999", "rgba("} {
		if strings.Contains(code, banned) {
			t.Errorf("shared modal code must not contain %q (use classes and tokens)", banned)
		}
	}
}

func TestSharedModalKeepsIDsAndCompat(t *testing.T) {
	t.Parallel()
	code := sharedModalCode(t)
	for _, want := range []string{
		`id="shared-modal-overlay"`, `id="shared-modal-title"`, `id="shared-modal-message"`,
		`id="shared-modal-confirm"`, `id="shared-modal-cancel"`,
		`role="dialog"`, `aria-modal="true"`, `aria-labelledby="shared-modal-title"`,
		`<h2 id="shared-modal-title"`, `class="modal-close"`, `class="modal-actions"`,
		// chat reuses its own overlay
		`getElementById('modal-overlay')`, `#modal-confirm`,
		// compat: .active and inline display
		`style.display = 'flex'`, `classList.add('active')`,
	} {
		if !strings.Contains(code, want) {
			t.Errorf("shared modal code lost %q", want)
		}
	}
	if strings.Contains(code, "modal-footer") {
		t.Error("shared modal must use .modal-actions, not .modal-footer")
	}

	src := normalizeAssetText(mustReadUIFile(t, "js/shared/shared-core.js"))
	for _, want := range []string{
		"function showModal(title, message, isConfirm = false, options = {})",
		"function showConfirmDialog(spec)",
		"function openModal(id, options)",
		"function closeModal(id, reason)",
		"'modal:close'",
	} {
		if !strings.Contains(src, want) {
			t.Errorf("shared-core.js lost API %q", want)
		}
	}
}

func TestWave3FoundationCSS(t *testing.T) {
	t.Parallel()
	pages := string(mustReadUIFile(t, "css/precision-pages.css"))
	shared := string(mustReadUIFile(t, "shared-components.css"))

	for _, marker := range []string{
		".pw-page .modal--sm", ".pw-page .modal--lg", ".pw-page .modal--xl",
		".pw-page .modal-small", ".pw-page .modal-wide",
		"max-height: calc(100dvh - 1rem)",
		".pw-page .pw-filter-pill", `.pw-filter-pill[aria-pressed="true"]`,
		".pw-page .pw-empty-state .pw-empty-icon", ".pw-page .pw-loading-state .pw-skeleton",
		".pw-page .modal-overlay.pw-confirm-overlay",
	} {
		if !strings.Contains(pages, marker) {
			t.Errorf("precision-pages.css missing %q", marker)
		}
	}
	for _, marker := range []string{
		".modal-overlay.pw-confirm-overlay", ".pw-confirm-detail", ".pw-confirm-field",
		".pw-confirm-check", "calc(var(--z-modal, 1000) + 10)",
	} {
		if !strings.Contains(shared, marker) {
			t.Errorf("shared-components.css missing %q", marker)
		}
	}

	// Status tokens exist for the dark default and the light theme.
	darkBlock := regexp.MustCompile(`(?s)\n\.pw-page \{(.*?)\}`).FindAllStringSubmatch(pages, -1)
	lightBlock := regexp.MustCompile(`(?s)\[data-theme="light"\] \.pw-page,\s*\.pw-page\[data-theme="light"\] \{(.*?)\}`).FindAllStringSubmatch(pages, -1)
	if len(darkBlock) == 0 || len(lightBlock) == 0 {
		t.Fatal("status token blocks for dark and light themes not found")
	}
	for _, status := range []string{"ok", "warn", "error", "idle", "running"} {
		token := "--pw-status-" + status + ":"
		if !strings.Contains(darkBlock[len(darkBlock)-1][1], token) {
			t.Errorf("dark theme lacks %s", token)
		}
		if !strings.Contains(lightBlock[len(lightBlock)-1][1], token) {
			t.Errorf("light theme lacks %s", token)
		}
		if !strings.Contains(pages, `.pw-page .pw-badge[data-status="`+status+`"]`) {
			t.Errorf("pw-badge lacks data-status=%q", status)
		}
	}
	for _, alias := range []string{"success", "warning", "accent"} {
		if !strings.Contains(pages, `.pw-badge[data-status="`+alias+`"]`) {
			t.Errorf("pw-badge lost legacy alias %q", alias)
		}
	}

	// The font rule is zero-specificity and limited to operational pages.
	const fontRule = ":where(.pw-page.pw-operational-page textarea) {"
	if !strings.Contains(pages, fontRule) {
		t.Errorf("precision-pages.css lacks the zero-specificity operational font rule %q", fontRule)
	}
	for _, page := range []string{"config.html", "index.html", "desktop.html"} {
		html := string(mustReadUIFile(t, page))
		if strings.Contains(html, "pw-operational-page") {
			t.Errorf("%s must not opt into pw-operational-page", page)
		}
	}
	if strings.Contains(strings.ToLower(pages), "gradient(") {
		t.Error("precision-pages.css must not introduce gradients")
	}
}

func TestWave3I18NConfirmKeys(t *testing.T) {
	t.Parallel()
	for _, lang := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		raw := string(mustReadUIFile(t, "lang/common/"+lang+".json"))
		for _, key := range []string{`"common.confirm_type_hint"`, `"common.confirm_type_label"`} {
			if !strings.Contains(raw, key) {
				t.Errorf("lang/common/%s.json lacks %s", lang, key)
			}
		}
		if !strings.Contains(raw, "{text}") {
			t.Errorf("lang/common/%s.json confirm_type_hint lacks the {text} placeholder", lang)
		}
	}
}

func TestAssertWave3DialogAcceptsContractMarkup(t *testing.T) {
	t.Parallel()
	good := `<div class="modal-overlay" id="x-modal" role="dialog" aria-modal="true" aria-labelledby="x-title">
<div class="modal"><div class="modal-header"><h2 id="x-title">T</h2><button class="modal-close">x</button></div>
<div class="modal-actions"></div></div></div>`
	assertWave3Dialog(t, good, "x-modal")
}
