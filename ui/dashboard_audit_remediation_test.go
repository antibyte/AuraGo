package ui

import (
	"regexp"
	"strings"
	"testing"
)

func TestDashboardAuditRemediationContracts(t *testing.T) {
	t.Parallel()

	html := readDashboardMarkup(t)
	for _, marker := range []string{
		`<section class="dash-section" id="sec-personality" data-section="personality"`,
		`<section class="dash-section" id="sec-user" data-section="user"`,
		`<section class="dash-section" id="sec-system" data-section="system"`,
		`<button type="button" class="collapse-toggle"`,
		`aria-expanded="true"`,
		`id="log-scroll-btn"`,
		`<span data-i18n="dashboard.logs_scroll_to_bottom">`,
		`<span data-i18n="dashboard.logs_refresh">`,
		`<span data-i18n="knowledge.filesync_refresh">`,
		`<span data-i18n="knowledge.filesync_rescan">`,
		`data-i18n="dashboard.page_heading"`,
		`data-pw-density-toggle`,
		`class="dash-nav-group-label"`,
		`data-section-link="overview"`,
	} {
		if !strings.Contains(html, marker) {
			t.Fatalf("dashboard HTML missing remediation marker %q", marker)
		}
	}
	if strings.Contains(html, `<span class="collapse-toggle"`) {
		t.Fatal("collapse toggles must be buttons, not spans")
	}
	if strings.Contains(html, `id="log-scroll-btn" data-i18n="`) {
		t.Fatal("log scroll button must not carry data-i18n on the button itself")
	}
	if regexp.MustCompile(`(?i)\son[a-z]+\s*=\s*["']`).MatchString(html) {
		t.Fatal("dashboard markup must not use inline event handlers; use data-action and the delegated listener")
	}
	for _, marker := range []string{
		`<button type="button" class="btn btn-secondary btn-sm" data-action="filesync-refresh" id="btn-fs-refresh-dash">`,
		`<button type="button" class="btn btn-primary btn-sm" data-action="filesync-rescan" id="btn-fs-rescan-dash">`,
		`id="memory-reflection-run" class="mh-more-btn memory-reflection-run-btn" data-action="memory-reflection-run"`,
		`<button type="button" class="cf-add-btn cron-btn-cancel" data-action="cron-edit-cancel"`,
		`<button type="button" class="cf-add-btn" id="cronEditSaveBtn" data-action="cron-edit-save"`,
		`<div class="kc-sync-loading" data-i18n="knowledge.filesync_loading"></div>`,
		`<div id="budget-disabled" class="empty-state is-hidden" data-i18n="dashboard.budget_disabled"></div>`,
	} {
		if !strings.Contains(html, marker) {
			t.Fatalf("dashboard HTML missing declarative-action marker %q", marker)
		}
	}
	if strings.Contains(html, `kc-sync-loading">Loading`) {
		t.Fatal("File sync placeholders must be translated")
	}

	mainJS := readDesktopAssetText(t, "js/dashboard/main.js")
	for _, marker := range []string{
		"el.children.length > 0",
		":scope > [data-i18n-text], :scope > .i18n-text",
		"nodeType === Node.TEXT_NODE",
		"CardState.setLoading(id)",
	} {
		if !strings.Contains(mainJS, marker) {
			t.Fatalf("dashboard main missing remediation marker %q", marker)
		}
	}

	widgets := readDesktopAssetText(t, "js/dashboard/dashboard-widgets.js")
	for _, marker := range []string{
		"esc(String(s.val))",
		"esc(s.lbl)",
		"esc(s.sub)",
		"esc(enfMap[data.enforcement] || String(data.enforcement))",
		"esc(names[key] || key)",
		"CardState.setLoaded('card-helper-llm')",
		"CardState.setLoaded('card-cronjobs')",
		"CardState.setLoaded('card-audit-log')",
		"CardState.setLoaded('card-journal')",
		"dashboard.profile_delete_error",
		"dashboard.profile_save_error",
		"t('dashboard.btn_delete')",
		"t('dashboard.operations_buffered'",
		"t('dashboard.quickstatus_n_pending'",
		"t('dashboard.quickstatus_n_auto_disabled'",
		"data-bar-width",
	} {
		if !strings.Contains(widgets, marker) {
			t.Fatalf("dashboard widgets missing remediation marker %q", marker)
		}
	}

	knowledge := readDesktopAssetText(t, "js/dashboard/widgets-knowledge.js")
	for _, marker := range []string{
		"AbortController",
		"_kgDetailSeq",
		"data-badge-color",
		"applyDynamicSurfaceVars",
	} {
		if !strings.Contains(knowledge, marker) {
			t.Fatalf("knowledge widgets missing remediation marker %q", marker)
		}
	}
	if strings.Contains(knowledge, `style="--badge-color`) || strings.Contains(knowledge, `style="background:`) {
		t.Fatal("knowledge widgets must not use inline style attributes")
	}

	charts := readDesktopAssetText(t, "js/dashboard/dashboard-charts.js")
	if !strings.Contains(charts, "tooltip: {\n                            enabled: true") && !strings.Contains(charts, "enabled: true") {
		t.Fatal("gauge charts must enable tooltips")
	}

	events := readDesktopAssetText(t, "js/dashboard/dashboard-events.js")
	if !strings.Contains(events, "aurago:themechange") {
		t.Fatal("dashboard must rebuild charts on aurago:themechange")
	}
	if !strings.Contains(events, "aria-expanded") {
		t.Fatal("collapse toggles must sync aria-expanded")
	}
	if strings.Contains(events, `showAlert('Error'`) {
		t.Fatal("error alerts must use i18n title")
	}
	for _, marker := range []string{
		"'filesync-refresh': () => loadFileSyncStatus()",
		"'filesync-rescan': () => triggerFileSyncRescan()",
		"'memory-reflection-run': () => runWeeklyReflectionNow()",
		"'cron-edit-cancel': () => closeCronEditModal()",
		"'cron-edit-save': () => saveCronEdit()",
		"e.target.closest('[data-action]')",
	} {
		if !strings.Contains(events, marker) {
			t.Fatalf("dashboard events missing delegated action %q", marker)
		}
	}

	css := strings.ReplaceAll(readDesktopAssetText(t, "css/dashboard.css"), "\r\n", "\n")
	for _, marker := range []string{
		"dash-card-shimmer",
		".dash-tab-group-sep",
		"max-height: 148px",
		"width: var(--bar-width, 4%)",
		".cronjobs-row-actions",
		"width: auto;\n    min-width: 0;\n    height: auto;\n    min-height: 1.75rem;",
	} {
		if !strings.Contains(css, marker) {
			t.Fatalf("dashboard CSS missing remediation marker %q", marker)
		}
	}
	if strings.Contains(css, ".cronjobs-row-btn {\n    width: 2rem;") {
		t.Fatal("cronjobs row buttons must not stay icon-sized")
	}

	en := readDesktopAssetText(t, "lang/dashboard/en.json")
	for _, key := range []string{
		`"dashboard.quickstatus_n_pending"`,
		`"dashboard.quickstatus_n_auto_disabled"`,
		`"dashboard.page_heading"`,
		`"dashboard.btn_delete"`,
		`"dashboard.gauge_used"`,
		`"dashboard.profile_save_error"`,
	} {
		if !strings.Contains(en, key) {
			t.Fatalf("dashboard en translations missing %s", key)
		}
	}
}
