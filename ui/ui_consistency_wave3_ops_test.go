package ui

import (
	"regexp"
	"strings"
	"testing"
)

// Wave 3, operations pages: containers, missions, media and dashboard share
// the dialog contract, filter pills, status tokens and openModal/closeModal.

func TestWave3OpsDialogsFollowTheDialogContract(t *testing.T) {
	t.Parallel()

	pages := map[string][]string{
		"containers.html":  {"log-modal", "inspect-modal", "terminal-modal", "protected-terminal-modal", "update-modal", "delete-modal"},
		"missions_v2.html": {"modal", "prep-modal"},
		"media.html":       {"audio-modal"},
		"dashboard.html":   {"cfOverlay", "cronEditOverlay", "kgDetailOverlay"},
	}
	for page, ids := range pages {
		html := renderUIPage(t, page)
		for _, id := range ids {
			assertWave3Dialog(t, html, id)
			tag := regexp.MustCompile(`<[a-zA-Z][^>]*\bid="` + regexp.QuoteMeta(id) + `"[^>]*>`).FindString(html)
			if !strings.Contains(tag, "modal-overlay") {
				t.Errorf("%s: dialog %q must be a .modal-overlay: %s", page, id, tag)
			}
		}
	}

	// The media dialog keeps its inner .modal.media-modal-content and drops the inline-hidden overlay.
	media := renderUIPage(t, "media.html")
	if !strings.Contains(media, `class="modal modal--sm media-modal-content"`) {
		t.Error("media.html: #audio-modal needs the inner .modal.media-modal-content")
	}
	if strings.Contains(media, `class="media-modal is-hidden"`) || strings.Contains(media, `onclick="closeAudioModal(event)"`) {
		t.Error("media.html: #audio-modal must not be a hand-rolled hidden overlay any more")
	}
}

func TestWave3OpsDeleteModalKeepsItsContract(t *testing.T) {
	t.Parallel()

	html := renderUIPage(t, "containers.html")
	for _, want := range []string{
		`id="delete-confirm-btn"`, `onclick="confirmDelete()"`, `id="delete-protected-warning"`, `class="modal modal--sm"`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("containers.html delete dialog lost %s", want)
		}
	}
	js := normalizeAssetText(mustReadUIFile(t, "js/containers/main.js"))
	for _, want := range []string{
		"openModal('delete-modal', { onClose: resetDeleteModal })",
		"closeModal('delete-modal')",
		"function setDeleteConfirmBusy(",
		"confirm=protected",
	} {
		if !strings.Contains(js, want) {
			t.Errorf("containers/main.js lost %q", want)
		}
	}
}

func TestWave3OpsFilterPillsKeepTheirHooks(t *testing.T) {
	t.Parallel()

	for page, hook := range map[string]string{"containers.html": "ct-filter-btn", "missions_v2.html": "missions-filter-btn"} {
		html := renderUIPage(t, page)
		buttons := regexp.MustCompile(`<button[^>]*class="[^"]*\b`+hook+`\b[^"]*"[^>]*>`).FindAllString(html, -1)
		if len(buttons) < 3 {
			t.Errorf("%s: expected filter buttons with hook %s, found %d", page, hook, len(buttons))
		}
		for _, b := range buttons {
			if !strings.Contains(b, "pw-filter-pill") || strings.Contains(b, "pw-badge") || !strings.Contains(b, "aria-pressed=") {
				t.Errorf("%s: filter button must be a pw-filter-pill with aria-pressed: %s", page, b)
			}
		}
	}
}

func TestWave3OpsContainersStateBadgeAndLoading(t *testing.T) {
	t.Parallel()

	js := normalizeAssetText(mustReadUIFile(t, "js/containers/main.js"))
	if !strings.Contains(js, `class="ct-card-state pw-badge ${stateClass}" data-status="${stateBadgeStatus(stateClass)}"`) {
		t.Error("container cards need a pw-badge state with data-status")
	}
	html := renderUIPage(t, "containers.html")
	if !strings.Contains(html, `class="pw-loading-state" id="ct-loading" role="status"`) {
		t.Error("containers.html lacks the loading placeholder")
	}
}

func TestWave3OpsDashboardUsesSharedModalsAndButtons(t *testing.T) {
	t.Parallel()

	events := normalizeAssetText(mustReadUIFile(t, "js/dashboard/dashboard-events.js"))
	for _, gone := range []string{"ModalFocusTrap", "initModalFocusTrap"} {
		if strings.Contains(events, gone) {
			t.Errorf("dashboard-events.js still has the private %s", gone)
		}
	}
	for _, want := range []string{"openModal('cfOverlay')", "openModal('cronEditOverlay'", "closeModal('cfOverlay')", "closeModal('cronEditOverlay')"} {
		if !strings.Contains(events, want) {
			t.Errorf("dashboard-events.js lacks %q", want)
		}
	}
	knowledge := normalizeAssetText(mustReadUIFile(t, "js/dashboard/widgets-knowledge.js"))
	if !strings.Contains(knowledge, "openModal('kgDetailOverlay'") || !strings.Contains(knowledge, "closeModal('kgDetailOverlay')") {
		t.Error("widgets-knowledge.js must open and close the detail dialog through openModal/closeModal")
	}

	html := renderUIPage(t, "dashboard.html")
	for _, want := range []string{
		`id="audit-clear-filtered" class="btn btn-danger btn-sm log-btn"`,
		`id="operational-issues-confirm-apply" class="btn btn-danger btn-sm log-btn"`,
		`id="cronEditSaveBtn"`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("dashboard.html lacks %q", want)
		}
	}
	for _, old := range []string{`cf-overlay`, `cf-modal`, `cf-add-btn`, `audit-danger-btn`} {
		if strings.Contains(html, old) {
			t.Errorf("dashboard.html still uses the superseded class %s", old)
		}
	}

	css := dashboardCSSWithoutComments(t, "css/dashboard.css")
	for _, pill := range []string{"pill-running", "pill-idle", "pill-completed", "pill-failed", "pill-warning"} {
		m := regexp.MustCompile(`(?s)
` + regexp.QuoteMeta(`.pw-page[data-workspace-page="dashboard"] .`+pill) + `\s*\{([^{}]*)\}`).FindStringSubmatch(css)
		if m == nil {
			t.Errorf("dashboard.css lacks a stand-alone .%s rule", pill)
			continue
		}
		body := m[1]
		if !strings.Contains(body, "--pw-status-") {
			t.Errorf(".%s must take its colours from the --pw-status-* tokens: %s", pill, body)
		}
		if strings.Contains(body, "rgba(") || regexp.MustCompile(`#[0-9a-fA-F]{3,8}\b`).MatchString(body) {
			t.Errorf(".%s keeps a literal colour: %s", pill, body)
		}
	}
}

func TestWave3OpsMediaEmptyStatesHaveTitles(t *testing.T) {
	t.Parallel()

	js := normalizeAssetText(mustReadUIFile(t, "js/media/main.js"))
	for _, key := range []string{"media.empty_audio_title", "media.empty_videos_title", "media.empty_documents_title"} {
		if !strings.Contains(js, "t('"+key+"')") {
			t.Errorf("media/main.js does not render %s", key)
		}
	}
	gallery := normalizeAssetText(mustReadUIFile(t, "js/gallery/main.js"))
	if !strings.Contains(gallery, "media.empty_images_title") {
		t.Error("gallery/main.js does not render media.empty_images_title")
	}
	for _, lang := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		raw := string(mustReadUIFile(t, "lang/media/"+lang+".json"))
		for _, key := range []string{"images", "audio", "videos", "documents"} {
			if !strings.Contains(raw, `"media.empty_`+key+`_title"`) {
				t.Errorf("lang/media/%s.json lacks media.empty_%s_title", lang, key)
			}
		}
	}
}

// Page scripts open overlays through openModal so Esc, focus trap and onClose
// work; adding .active by hand leaves them without any of that.
func TestWave3OpsPageScriptsDoNotShowOverlaysByClass(t *testing.T) {
	t.Parallel()

	byClass := regexp.MustCompile(`(?i)(?:overlay|modal)[A-Za-z]*\)?\s*(?:\?\.)?\.classList\.add\(\s*['"]active['"]`)
	byID := regexp.MustCompile(`getElementById\(\s*['"]([^'"]*(?:modal|overlay)[^'"]*)['"]\s*\)\s*(?:\?\.)?\.classList\.add\(\s*['"]active['"]`)
	for _, file := range []string{
		"js/containers/main.js", "js/missions/main.js", "js/media/main.js", "js/gallery/main.js",
		"js/dashboard/dashboard-events.js", "js/dashboard/widgets-knowledge.js", "js/dashboard/operational-issues.js",
		"js/dashboard/sections/memory.js", "js/dashboard/main.js",
	} {
		src := normalizeAssetText(mustReadUIFile(t, file))
		if m := byClass.FindString(src); m != "" {
			t.Errorf("%s shows an overlay via classList.add('active'): %s", file, m)
		}
		if m := byID.FindString(src); m != "" {
			t.Errorf("%s shows an overlay via classList.add('active'): %s", file, m)
		}
	}
}
