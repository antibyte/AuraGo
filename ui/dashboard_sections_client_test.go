package ui

import (
	"strings"
	"testing"
)

// TestDashboardSectionLoadersReplaceTheTabSystem pins the client contract: every sidebar section has a module that
// registers its loader with DashSections, loadSectionContent runs the registered loader (a false result keeps the
// last "Updated" stamp), DashRouter boot, DashAPI delegation, the section refresh button, the removal of the tab
// system and of the duplicate overview/logs/tool-stats fetches, and the router/api scripts loading before main.js.
func TestDashboardSectionLoadersReplaceTheTabSystem(t *testing.T) {
	t.Parallel()

	for _, id := range dashboardSectionIDs {
		module := readDesktopAssetText(t, "js/dashboard/sections/"+id+".js")
		if !strings.Contains(module, "DashSections.register({ id: '"+id+"', load: ") {
			t.Errorf("sections/%s.js does not register its loader with DashSections", id)
		}
	}
	mainJS := readDesktopAssetText(t, "js/dashboard/main.js")
	for _, marker := range []string{
		"async function loadSectionContent(id, { force = false } = {})",
		"const section = DashSections.get(id);",
		"const outcome = await section.load({ force });",
		"if (outcome === false) return;",
		"t('dashboard.section.updated').replace('{{time}}', new Date().toLocaleTimeString())",
		"const SectionState = { loaded: {} };",
		"DashRouter.init({ onShow: id => loadSectionContent(id) });",
		"e.target.closest('[data-section-refresh]')",
		"await loadSectionContent('logs', { force: true });",
		"get: (url, opts) => DashAPI.get(url, Object.assign({ ttl: 0 }, opts)),",
		"getWithStatus: (url, opts) => DashAPI.getWithStatus(url, Object.assign({ ttl: 0 }, opts)),",
	} {
		if !strings.Contains(mainJS, marker) {
			t.Errorf("main.js misses %q", marker)
		}
	}
	for _, gone := range []string{"SECTION_LOADERS", "VALID_TABS", "TabState", "showTab(", "initTabKeyboardNav", "loadTabContent", "fetchAll", "loadAgentBanner", "aurago-dash-tab", "loadTabOverview", "loadTabAgent", "loadTabUser", "loadTabSystem", "dashTabs"} {
		if strings.Contains(mainJS, gone) {
			t.Errorf("main.js still contains %q", gone)
		}
	}
	// Counted and searched across every dashboard script: the section modules may hold any of them.
	dashJS := readDashboardJS(t)
	for url, want := range map[string]int{
		"'/api/dashboard/overview'":       1, // system loader (card-operations); the overview reads the summary feed
		"'/api/dashboard/logs?lines=100'": 1,
		"'/api/dashboard/tool-stats'":     1,
	} {
		if got := strings.Count(dashJS, url); got != want {
			t.Errorf("the dashboard scripts request %s %d times, want %d", url, got, want)
		}
	}
	for _, marker := range []string{
		"await loadSectionContent('memory', { force: true });",
		"loadSectionContent('user', { force: true });",
		"DashRouter.current() !== 'audit'",
	} {
		if !strings.Contains(dashJS, marker) {
			t.Errorf("the dashboard scripts miss %q", marker)
		}
	}

	widgets := readDesktopAssetText(t, "js/dashboard/dashboard-widgets.js")
	// A narrative toggle changes /api/personality/state without a personality_update SSE, so the
	// cached state must be dropped by the mutation itself.
	if at := strings.Index(widgets, "async function characterNoteAction("); at < 0 {
		t.Error("dashboard-widgets.js misses characterNoteAction")
	} else if end := strings.Index(widgets[at:], "function renderCharacterNotes("); end < 0 ||
		!strings.Contains(widgets[at:at+end], "DashAPI.invalidate('/api/personality/');") {
		t.Error("characterNoteAction must call DashAPI.invalidate('/api/personality/') after a successful POST")
	}
	for _, gone := range []string{"TabState", "loadTabAgent", "loadTabUser", "function loadErrorPatterns"} {
		if strings.Contains(dashJS, gone) {
			t.Errorf("the dashboard scripts still contain %q", gone)
		}
	}

	speech := readDesktopAssetText(t, "js/dashboard/realtime-speech-status.js")
	if !strings.Contains(speech, "window.loadRealtimeSpeechStatus = loadRealtimeSpeechStatus;") || strings.Contains(speech, "DOMContentLoaded") {
		t.Error("realtime-speech-status.js must hand its loader to the system section instead of loading on DOMContentLoaded")
	}

	html := readDesktopAssetText(t, "dashboard.html")
	mainAt := strings.Index(html, "/js/dashboard/main.js")
	for _, script := range []string{"/js/dashboard/dash-router.js", "/js/dashboard/dash-api.js"} {
		at := strings.Index(html, script)
		if at < 0 || mainAt < 0 || at > mainAt {
			t.Errorf("dashboard.html must load %s before main.js", script)
			continue
		}
		end := strings.Index(html[at:], ">")
		if end < 0 || !strings.Contains(html[at:at+end], " defer") {
			t.Errorf("%s must load with defer", script)
		}
	}
}

// TestDashboardEventsFollowTheRouter: SSE handlers and the theme reload use DashRouter.current() and
// drop the DashAPI cache entries their event makes stale.
func TestDashboardEventsFollowTheRouter(t *testing.T) {
	t.Parallel()

	events := readDesktopAssetText(t, "js/dashboard/dashboard-events.js")
	for _, marker := range []string{
		"DashRouter.current() === 'personality'",
		"DashRouter.current() === 'automations'",
		"Object.keys(SectionState.loaded)",
		"const active = DashRouter.current();",
		"DashAPI.invalidate('/api/dashboard/memory');",
		"DashAPI.invalidate('/api/personality/');",
		"DashAPI.invalidate('/api/daemons');",
		"DashAPI.invalidate('/api/dashboard/audit');",
		"DashAPI.invalidate('/api/budget');",
	} {
		if !strings.Contains(events, marker) {
			t.Errorf("dashboard-events.js misses %q", marker)
		}
	}
	for _, gone := range []string{"TabState", "loadTabContent"} {
		if strings.Contains(events, gone) {
			t.Errorf("dashboard-events.js still contains %q", gone)
		}
	}
}
