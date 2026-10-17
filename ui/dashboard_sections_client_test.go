package ui

import (
	"strings"
	"testing"
)

// TestDashboardSectionLoadersReplaceTheTabSystem pins the P2 client contract: one loader per sidebar
// section, DashRouter boot, DashAPI delegation, the section refresh button, the removal of the tab system
// and of the duplicate overview/logs/tool-stats fetches, and the router/api scripts loading before main.js.
func TestDashboardSectionLoadersReplaceTheTabSystem(t *testing.T) {
	t.Parallel()

	mainJS := readDesktopAssetText(t, "js/dashboard/main.js")
	for _, id := range []string{"overview", "personality", "memory", "user", "knowledge", "filesync", "problems", "automations", "system", "audit", "logs", "llm", "prompts", "tools"} {
		if !strings.Contains(mainJS, "'"+id+"': ") {
			t.Errorf("SECTION_LOADERS has no entry for section %q", id)
		}
	}
	for _, marker := range []string{
		"const SECTION_LOADERS = {",
		"async function loadSectionContent(id, { force = false } = {})",
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
	for _, gone := range []string{"VALID_TABS", "TabState", "showTab(", "initTabKeyboardNav", "loadTabContent", "fetchAll", "loadAgentBanner", "aurago-dash-tab", "loadTabOverview", "loadTabAgent", "loadTabUser", "loadTabSystem", "dashTabs"} {
		if strings.Contains(mainJS, gone) {
			t.Errorf("main.js still contains %q", gone)
		}
	}
	for url, want := range map[string]int{
		"'/api/dashboard/overview'":       2, // overview + system loaders share one cached answer
		"'/api/dashboard/logs?lines=100'": 1,
		"'/api/dashboard/tool-stats'":     1,
	} {
		if got := strings.Count(mainJS, url); got != want {
			t.Errorf("main.js requests %s %d times, want %d", url, got, want)
		}
	}

	widgets := readDesktopAssetText(t, "js/dashboard/dashboard-widgets.js")
	for _, marker := range []string{
		"await loadSectionContent('memory', { force: true });",
		"loadSectionContent('user', { force: true });",
		"DashRouter.current() !== 'audit'",
	} {
		if !strings.Contains(widgets, marker) {
			t.Errorf("dashboard-widgets.js misses %q", marker)
		}
	}
	for _, gone := range []string{"TabState", "loadTabAgent", "loadTabUser", "function loadErrorPatterns"} {
		if strings.Contains(widgets, gone) {
			t.Errorf("dashboard-widgets.js still contains %q", gone)
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
