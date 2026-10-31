package ui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Wave 3 (T4): Plans and TrueNAS on the shared page system.

func TestPlansPageUsesPageSystem(t *testing.T) {
	t.Parallel()

	html := renderUIPage(t, "plans.html")
	for _, want := range []string{
		`<header class="page-header pw-page-heading">`,
		`<h1 class="pw-page-title" data-i18n="common.page_plans">`,
		`data-i18n="plans.page_description"`,
		`<h2 data-i18n="plans.list_title">`,
		`class="plans-toolbar pw-toolbar"`,
		`class="plans-page pw-page-frame"`,
		`/css/enhancements.css?v=`,
		`class="pw-loading-state"`,
		`class="pw-skeleton skeleton"`,
		`<label for="session-id"`,
		`<label for="status-filter"`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("plans.html missing %q", want)
		}
	}
	if strings.Count(html, "<h1") != 1 {
		t.Errorf("plans.html must carry exactly one H1, got %d", strings.Count(html, "<h1"))
	}
	for _, id := range []string{"blocker-modal", "split-modal"} {
		assertWave3Dialog(t, html, id)
		if !regexp.MustCompile(`<div class="modal plan-modal">`).MatchString(html) {
			t.Errorf("dialog %s must use .modal with the .plan-modal hook", id)
		}
	}
	for _, id := range []string{"blocker-reason", "split-items"} {
		if !strings.Contains(html, `for="`+id+`"`) {
			t.Errorf("textarea %s has no <label for>", id)
		}
	}

	js := normalizeAssetText(mustReadUIFile(t, "js/plans/main.js"))
	for _, want := range []string{
		"openModal('blocker-modal', {", "openModal('split-modal', {", "onClose:",
		"plans.empty_list_title", "plans.empty_list_desc", `class="pw-empty-state"`,
	} {
		if !strings.Contains(js, want) {
			t.Errorf("plans/main.js missing %q", want)
		}
	}
	css := normalizeAssetText(mustReadUIFile(t, "css/plans.css"))
	for _, bad := range []string{"modal-footer", "calc(100vw - 2rem)"} {
		if strings.Contains(css, bad) {
			t.Errorf("plans.css still carries %q", bad)
		}
	}
}

func TestTrueNASPageUsesPageSystem(t *testing.T) {
	t.Parallel()

	html := renderUIPage(t, "truenas.html")
	for _, want := range []string{
		`<header class="page-header pw-page-heading">`,
		`<h1 class="pw-page-title" data-i18n="common.page_truenas">`,
		`data-i18n="truenas.page_description"`,
		`class="truenas-container pw-page-frame"`,
		`role="tablist" data-i18n-aria-label="truenas.tablist_label"`,
		`<label for="setting-host"`,
		`<label for="setting-apikey"`,
		`<label for="snapshot-filter"`,
		`<span class="pw-badge" data-status="idle">`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("truenas.html missing %q", want)
		}
	}
	if strings.Count(html, "<h1") != 1 {
		t.Errorf("truenas.html must carry exactly one H1, got %d", strings.Count(html, "<h1"))
	}

	// Full tab ARIA: tab <-> panel pairing, roving tabindex.
	for i, tab := range []string{"overview", "pools", "datasets", "snapshots", "shares", "settings"} {
		selected, tabindex := "false", "-1"
		if i == 0 {
			selected, tabindex = "true", "0"
		}
		button := `id="tabbtn-` + tab + `" role="tab" aria-selected="` + selected + `" aria-controls="tab-` + tab + `" tabindex="` + tabindex + `"`
		if !strings.Contains(html, button) {
			t.Errorf("tab button for %s lacks ARIA wiring: want %s", tab, button)
		}
		panel := `id="tab-` + tab + `"`
		idx := strings.Index(html, panel)
		if idx < 0 {
			t.Errorf("panel tab-%s missing", tab)
			continue
		}
		end := idx + strings.Index(html[idx:], ">")
		if tag := html[idx:end]; !strings.Contains(tag, `role="tabpanel"`) || !strings.Contains(tag, `aria-labelledby="tabbtn-`+tab+`"`) {
			t.Errorf("panel tab-%s must be a tabpanel labelled by its tab: %s", tab, tag)
		}
	}

	for _, id := range []string{"modal-dataset", "modal-snapshot", "modal-share"} {
		assertWave3Dialog(t, html, id)
	}
	// Every form control in the dialogs and settings carries a label.
	for _, id := range []string{"dataset-name", "dataset-compression", "dataset-quota", "snapshot-dataset", "snapshot-name", "snapshot-retention", "share-type", "share-name", "share-path", "nfs-networks", "nfs-hosts"} {
		if !strings.Contains(html, `<label for="`+id+`"`) {
			t.Errorf("control %s has no <label for>", id)
		}
	}

	js := normalizeAssetText(mustReadUIFile(t, "js/truenas.js"))
	for _, want := range []string{
		"'ArrowRight'", "'ArrowLeft'", "'Home'", "'End'", "aria-selected",
		"requireText: name", "showConfirmDialog(",
		"{danger: true, confirmText: t('truenas.btn_delete')}", "{danger: true, confirmText: t('truenas.btn_rollback')}",
		"showToast(t('truenas.settings_saved')", "showToast(t('truenas.scrub_started')",
		`<div class="pw-empty-state">`, `<div class="pw-loading-state"`,
		"this.showError('settings-error'", "this.showError('dataset-error'",
	} {
		if !strings.Contains(js, want) {
			t.Errorf("truenas.js missing %q", want)
		}
	}
	for _, bad := range []string{"showAlert(", "showSuccess(", `class="status-badge`, `class="status-online`, `class="status-offline`} {
		if strings.Contains(js, bad) {
			t.Errorf("truenas.js still uses %q", bad)
		}
	}
}

func TestPlansTrueNASLocaleKeys(t *testing.T) {
	t.Parallel()

	want := map[string][]string{
		"plans":   {"plans.page_description", "plans.list_title", "plans.empty_list_title", "plans.empty_list_desc"},
		"truenas": {"truenas.page_description", "truenas.tablist_label", "truenas.empty_pools_desc", "truenas.empty_datasets_desc", "truenas.empty_snapshots_desc", "truenas.empty_shares_desc"},
	}
	locales := []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"}
	for section, keys := range want {
		for _, lang := range locales {
			raw, err := os.ReadFile(filepath.Join("lang", section, lang+".json"))
			if err != nil {
				t.Fatal(err)
			}
			var values map[string]string
			if err := json.Unmarshal(raw, &values); err != nil {
				t.Fatalf("%s/%s: %v", section, lang, err)
			}
			for _, key := range keys {
				if strings.TrimSpace(values[key]) == "" {
					t.Errorf("lang/%s/%s.json lacks %s", section, lang, key)
				}
			}
		}
	}
}
