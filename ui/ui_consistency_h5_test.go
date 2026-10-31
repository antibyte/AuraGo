package ui

import (
	"strings"
	"testing"
)

// UI consistency audit 2026-10-31 (H6, H7, M16): static contracts.

func TestTrueNASDialogsUseSharedModalPattern(t *testing.T) {
	t.Parallel()

	html := normalizeAssetText(mustReadUIFile(t, "truenas.html"))
	if strings.Contains(html, "truenas-modal") {
		t.Fatal("truenas.html still uses the private .truenas-modal pattern")
	}
	for _, id := range []string{"modal-dataset", "modal-snapshot", "modal-share"} {
		if !strings.Contains(html, `id="`+id+`" class="modal-overlay" role="dialog" aria-modal="true" aria-labelledby="`+id+`-title"`) {
			t.Fatalf("dialog %s must be a modal-overlay with dialog semantics", id)
		}
	}
	if got := strings.Count(html, `class="modal-close"`); got != 3 {
		t.Fatalf("expected 3 modal-close buttons, got %d", got)
	}
	js := normalizeAssetText(mustReadUIFile(t, "js/truenas.js"))
	for _, want := range []string{"openModal('modal-dataset')", "openModal('modal-snapshot')", "openModal('modal-share')", "closeModal(m.id)"} {
		if !strings.Contains(js, want) {
			t.Fatalf("truenas.js must use the shared modal helpers: missing %q", want)
		}
	}
}

func TestTrueNASNotConfiguredEmptyState(t *testing.T) {
	t.Parallel()

	html := normalizeAssetText(mustReadUIFile(t, "truenas.html"))
	for _, want := range []string{`id="truenas-not-configured"`, `href="/config#truenas"`, "truenas.not_configured_title"} {
		if !strings.Contains(html, want) {
			t.Fatalf("truenas.html missing %q", want)
		}
	}
	css := normalizeAssetText(mustReadUIFile(t, "css/truenas.css"))
	if !strings.Contains(css, `.pw-page[data-workspace-page="truenas"] .stats-grid.is-hidden {`) {
		t.Fatal("truenas.css must let .is-hidden win over the stats-grid display rule")
	}
	js := normalizeAssetText(mustReadUIFile(t, "js/truenas.js"))
	if !strings.Contains(js, `<span class="status-unknown">● ${t('truenas.status_not_configured')}`) {
		t.Fatal("not-configured header pill must be neutral (status-unknown)")
	}
	for _, want := range []string{"setNotConfigured(data.enabled === false)", "truenas.status_not_configured", "resetOverviewStats"} {
		if !strings.Contains(js, want) {
			t.Fatalf("truenas.js missing %q", want)
		}
	}
}

func TestContainersErrorStringsAreTranslated(t *testing.T) {
	t.Parallel()

	js := normalizeAssetText(mustReadUIFile(t, "js/containers/main.js"))
	for _, bad := range []string{"'Error loading logs'", "'Failed to load logs'", "'Failed to load details'"} {
		if strings.Contains(js, bad) {
			t.Fatalf("containers/main.js still hard-codes %s", bad)
		}
	}
	for _, want := range []string{"t('containers.logs_error')", "t('containers.logs_load_failed')", "t('containers.inspect_load_failed')"} {
		if !strings.Contains(js, want) {
			t.Fatalf("containers/main.js missing %q", want)
		}
	}
}
