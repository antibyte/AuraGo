package ui

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestConfigWave3SidebarStateBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	origin := configRefreshFixtureOriginWith(t, "en", false, func(configuration map[string]any) {
		configuration["adguard"].(map[string]any)["enabled"] = true
		configuration["ldap"].(map[string]any)["enabled"] = false
		configuration["webdav"] = map[string]any{"enabled": false, "url": "https://dav.fixture.invalid", "username": "home-lab", "password": "••••••••"}
	})
	page := newSmokeBrowser(t).MustPage()
	defer page.MustClose()
	page.MustNavigate(origin + "/config#overview").MustWaitLoad()
	waitForJSBool(t, page, `() => !!document.querySelector('.pw-overview-card') && !!document.querySelector('.sidebar-item[data-section="adguard"] .cfg-state-dot')`)
	page.MustEval(`() => { sessionStorage.clear(); localStorage.removeItem('aurago-cfg-only-active'); }`)

	dot := func(section string) string {
		return page.MustEval(`(section) => {
			const item = document.querySelector('.sidebar-item[data-section="' + section + '"]');
			const dot = item.querySelector('.cfg-state-dot');
			const shown = getComputedStyle(dot).display !== 'none';
			const text = item.getAttribute('aria-describedby') ? document.getElementById(item.getAttribute('aria-describedby')).textContent : '';
			return item.dataset.state + '|' + dot.className.replace('cfg-state-dot ', '') + '|' + shown + '|' + text;
		}`, section).String()
	}
	waitForJSBool(t, page, `() => document.querySelector('.sidebar-item[data-section="adguard"]').dataset.state === 'active'`)
	if got := dot("adguard"); got != "active|cfg-state-active|true|Active" {
		t.Fatalf("enabled section dot = %q", got)
	}
	if got := dot("webdav"); got != "configured|cfg-state-configured|true|Set up, but off" {
		t.Fatalf("configured-off section dot = %q", got)
	}
	if got := dot("ldap"); !strings.HasPrefix(got, "off|cfg-state-none|false|") {
		t.Fatalf("off section dot = %q", got)
	}
	if got := dot("server"); !strings.HasPrefix(got, "none|cfg-state-none|false|") {
		t.Fatalf("core section dot = %q", got)
	}

	// The overview lists the active integration and counts them per group.
	waitForJSBool(t, page, `() => !!document.querySelector('.pw-state-chip[data-overview-section="adguard"]')`)
	if page.MustEval(`() => !!document.querySelector('.pw-state-chip[data-overview-section="ldap"]') || !!document.querySelector('.pw-state-block-attention') || !!document.querySelector('.pw-overview-stat')`).Bool() {
		t.Fatal("overview must not list an off section, an empty attention block or pw-overview-stat")
	}
	if !page.MustEval(`() => /^\d+ active$/.test((document.querySelector('.pw-overview-card:has([data-overview-section="adguard"]) .pw-overview-card-active') || {}).textContent || '')`).Bool() {
		t.Fatal("a group card must show how many sections are active")
	}

	// A switch toggled in the draft updates the dot before anything is saved.
	page.MustEval(`async () => { await navigateToConfigSection('ldap', { scrollBehavior: 'auto' }); }`)
	waitForJSBool(t, page, `() => !!document.querySelector('#content [data-path="ldap.enabled"]')`)
	page.MustEval(`() => document.querySelector('#content [data-path="ldap.enabled"]').click()`)
	waitForJSBool(t, page, `() => document.querySelector('.sidebar-item[data-section="ldap"]').dataset.state === 'active'`)
	page.MustEval(`() => document.querySelector('#content [data-path="ldap.enabled"]').click()`)
	waitForJSBool(t, page, `() => document.querySelector('.sidebar-item[data-section="ldap"]').dataset.state === 'off'`)
	// Rendering the section normalizes a few untouched fields; drop them so navigation is not blocked.
	page.MustEval(`() => { window.AuraConfigState.discard(); }`)

	// A banner error marks the section red; a later connected clears it.
	page.MustEval(`() => {
		const host = document.createElement('div');
		host.id = 'wave3-banner';
		document.body.appendChild(host);
		window.cfgStatusBanner(host, 'error', { reason: 'boom', section: 'adguard' });
	}`)
	if got := dot("adguard"); !strings.HasPrefix(got, "error|cfg-state-error|true|Error at the last check: boom") {
		t.Fatalf("banner error dot = %q", got)
	}
	// Without a section the banner belongs to the open one.
	page.MustEval(`async () => { await navigateToConfigSection('adguard', { scrollBehavior: 'auto' }); }`)
	page.MustEval(`() => window.cfgStatusBanner(document.getElementById('wave3-banner'), 'error', { reason: 'slow' })`)
	if got := dot("adguard"); !strings.HasPrefix(got, "error|cfg-state-error|true|") {
		t.Fatalf("banner without section must mark the open section, got %q", got)
	}
	page.MustEval(`async () => { await navigateToConfigSection('overview', { scrollBehavior: 'auto' }); }`)
	waitForJSBool(t, page, `() => !!document.querySelector('.pw-attention-item[data-overview-section="adguard"]')`)
	if text := page.MustEval(`() => document.querySelector('.pw-attention-item[data-overview-section="adguard"]').textContent`).String(); !strings.Contains(text, "slow") {
		t.Fatalf("attention item must show the reason, got %q", text)
	}
	page.MustEval(`() => {
		window.cfgStatusBanner(document.getElementById('wave3-banner'), 'connected', { section: 'adguard' });
		window.cfgStatusBanner(document.getElementById('wave3-banner'), 'connected', { section: 'ldap' });
	}`)
	waitForJSBool(t, page, `() => !document.querySelector('.pw-attention-item') && document.querySelector('.sidebar-item[data-section="adguard"]').dataset.state === 'active'`)

	// A stored failure of a section that is switched off is stale: no red dot, not in the overview.
	page.MustEval(`() => {
		sessionStorage.setItem('aurago.config.sectionStatus.v1', JSON.stringify({ adguard: { state: 'error', reason: 'old', at: Date.now() } }));
		updateSidebarStates();
	}`)
	waitForJSBool(t, page, `() => document.querySelector('.sidebar-item[data-section="adguard"]').dataset.state === 'error'`)
	page.MustEval(`async () => { await navigateToConfigSection('adguard', { scrollBehavior: 'auto' }); }`)
	waitForJSBool(t, page, `() => !!document.querySelector('#content [data-path="adguard.enabled"]')`)
	page.MustEval(`() => document.querySelector('#content [data-path="adguard.enabled"]').click()`)
	waitForJSBool(t, page, `() => document.querySelector('.sidebar-item[data-section="adguard"]').dataset.state === 'off'`)
	if got := dot("adguard"); !strings.HasPrefix(got, "off|cfg-state-none|false|") {
		t.Fatalf("switched-off section with a stored error = %q", got)
	}
	page.MustEval(`() => { window.AuraConfigState.discard(); }`)
	waitForJSBool(t, page, `() => document.querySelector('.sidebar-item[data-section="adguard"]').dataset.state === 'active'`)

	// A save that needs a restart marks the changed sections.
	page.MustEval(`async () => { await navigateToConfigSection('overview', { scrollBehavior: 'auto' }); }`)
	page.MustEval(`() => window.AuraConfigSectionState.rememberRestart(['webdav.url'])`)
	waitForJSBool(t, page, `() => document.querySelector('.sidebar-item[data-section="webdav"]').dataset.state === 'restart'`)
	waitForJSBool(t, page, `() => !!document.querySelector('.pw-attention-item[data-overview-section="webdav"][data-state="restart"]')`)
	page.MustEval(`() => { sessionStorage.removeItem('aurago.config.restartSections.v1'); updateSidebarStates(); }`)

	// Only active: inactive items go away, the choice persists, groups are stored by id.
	page.MustEval(`async () => { await navigateToConfigSection('adguard', { scrollBehavior: 'auto' }); }`)
	page.MustEval(`() => document.getElementById('sidebarOnlyActive').click()`)
	waitForJSBool(t, page, `() => document.querySelector('.sidebar-item[data-section="ldap"]').classList.contains('cfg-filter-hidden')`)
	filtered := page.MustEval(`() => {
		const shown = key => getComputedStyle(document.querySelector('.sidebar-item[data-section="' + key + '"]')).display !== 'none';
		const group = document.querySelector('.sidebar-group:has([data-section="danger_zone"])');
		return [shown('adguard'), shown('ldap'), shown('webdav'), getComputedStyle(group).display !== 'none', localStorage.getItem('aurago-cfg-only-active')].join('|');
	}`).String()
	if filtered != "true|false|false|false|1" {
		t.Fatalf("only-active filter state = %q", filtered)
	}
	ids := page.MustEval(`() => JSON.stringify({stored: JSON.parse(localStorage.getItem('aurago-cfg-groups-collapsed')), ids: [...document.querySelectorAll('.sidebar-group')].map(el => el.dataset.groupId)})`).String()
	var collapse struct {
		Stored []string `json:"stored"`
		IDs    []string `json:"ids"`
	}
	if err := json.Unmarshal([]byte(ids), &collapse); err != nil {
		t.Fatal(err)
	}
	known := map[string]bool{}
	for _, id := range collapse.IDs {
		known[id] = true
	}
	for _, id := range collapse.Stored {
		if !known[id] {
			t.Fatalf("collapsed groups must be stored by group id, found %q (ids %v)", id, collapse.IDs)
		}
	}
	// It combines with the search: a search without hits shows its own message, not the filter hint.
	page.MustEval(`() => { const input = document.getElementById('sidebarSearchInput'); input.value = 'zzzzqqq'; input.dispatchEvent(new Event('input', { bubbles: true })); }`)
	waitForJSBool(t, page, `() => !document.getElementById('sidebarSearchNoResults').classList.contains('hidden')`)
	if page.MustEval(`() => !document.getElementById('sidebarFilterEmpty').classList.contains('hidden')`).Bool() {
		t.Fatal("the filter hint must give way to the search result message")
	}
	// A search whose hits are all hidden by the filter shows the filter hint, not a blank sidebar.
	page.MustEval(`() => { const input = document.getElementById('sidebarSearchInput'); input.value = 'ldap'; input.dispatchEvent(new Event('input', { bubbles: true })); }`)
	waitForJSBool(t, page, `() => !document.getElementById('sidebarFilterEmpty').classList.contains('hidden')`)
	if !page.MustEval(`() => document.getElementById('sidebarSearchNoResults').classList.contains('hidden')`).Bool() {
		t.Fatal("a search with hits must not show the no-results message")
	}
	page.MustEval(`() => clearSidebarSearch(false)`)
	page.MustReload().MustWaitLoad()
	waitForJSBool(t, page, `() => !!document.querySelector('.sidebar-item[data-section="ldap"]')`)
	if !page.MustEval(`() => document.getElementById('sidebarOnlyActive').checked && document.querySelector('.sidebar-item[data-section="ldap"]').classList.contains('cfg-filter-hidden')`).Bool() {
		t.Fatal("the only-active filter must survive a reload")
	}
	page.MustEval(`() => document.getElementById('sidebarOnlyActive').click()`)
	waitForJSBool(t, page, `() => !document.querySelector('.sidebar-item.cfg-filter-hidden') && localStorage.getItem('aurago-cfg-only-active') === '0'`)
}
