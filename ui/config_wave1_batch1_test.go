package ui

import (
	"strings"
	"testing"
)

// Config UX wave 1, batch 1: adguard, agentmail, evomap, grafana, jellyfin, koofr, mqtt,
// obsidian, telnyx, truenas, uptime_kuma and webdav use the shared result line, state
// banner, save dock for secrets and draft tests.
type wave1Batch1Module struct {
	section, file, testButton, resultID, bannerID, testFn, testEndpoint, draftSection string
	secretPaths                                                                       []string
}

var wave1Batch1Modules = []wave1Batch1Module{
	{"adguard", "cfg/adguard.js", "adg-test-btn", "adg-test-result", "adg-status-banner", "adgTestConnection", "/api/adguard/test", "adguard", []string{"adguard.password"}},
	{"agentmail", "cfg/agentmail.js", "agentmail-test-btn", "agentmail-test-result", "agentmail-status-banner", "agentMailTestConnection", "/api/agentmail/test", "agentmail", []string{"agentmail.api_key"}},
	{"evomap", "cfg/evomap.js", "evomap-test-btn", "evomap-test-result", "evomap-status-banner", "evomapTestConnection", "/api/evomap/test", "evomap", []string{"evomap.api_key", "evomap.node_secret"}},
	{"grafana", "cfg/grafana.js", "grafana-test-btn", "grafana-test-result", "grafana-status-banner", "grafanaTestConnection", "/api/grafana/test", "grafana", []string{"grafana.api_key"}},
	{"jellyfin", "cfg/jellyfin.js", "jellyfin-test-btn", "jellyfin-test-result", "jellyfin-status-banner", "jellyfinTestConnection", "/api/jellyfin/test", "jellyfin", []string{"jellyfin.api_key"}},
	{"koofr", "cfg/koofr.js", "koofr-test-btn", "koofr-test-result", "koofr-status-banner", "koofrTestConnection", "/api/koofr/test", "koofr", []string{"koofr.app_password"}},
	{"mqtt", "cfg/mqtt.js", "mqtt-test-btn", "mqtt-test-result", "mqtt-status-banner", "mqttTestConnection", "/api/mqtt/test", "mqtt", []string{"mqtt.password"}},
	{"obsidian", "cfg/obsidian.js", "obsidian-test-btn", "obsidian-test-result", "obsidian-status-banner", "obsidianTestConnection", "/api/obsidian/test", "obsidian", []string{"obsidian.api_key"}},
	{"telnyx", "cfg/telnyx.js", "telnyx-test-btn", "telnyx-test-result", "telnyx-status-banner", "telnyxTestConnection", "/api/telnyx/test", "telnyx", []string{"telnyx.api_key"}},
	{"truenas", "cfg/truenas.js", "truenas-test-btn", "truenas-test-result", "truenas-status-banner", "truenasTestConnection", "/api/truenas/test", "truenas", []string{"truenas.api_key"}},
	{"uptime_kuma", "cfg/uptime_kuma.js", "uptime-kuma-test-btn", "uptime-kuma-test-result", "uk-status-banner", "uptimeKumaTestConnection", "/api/uptime-kuma/test", "uptime_kuma", []string{"uptime_kuma.api_key"}},
	{"webdav", "cfg/webdav.js", "webdav-test-btn", "webdav-test-result", "webdav-status-banner", "webdavTestConnection", "/api/webdav/test", "webdav", []string{"webdav.password", "webdav.token"}},
}

func TestConfigWave1Batch1ModulesUseSharedFeedback(t *testing.T) {
	t.Parallel()
	for _, m := range wave1Batch1Modules {
		src := readWave0File(t, m.file)
		for _, want := range []string{
			"cfgActionResult(",
			"cfgStatusBanner.check(",
			"cfgDraftPatch(['" + m.draftSection + "'])",
			`data-config-action-draft="true"`,
			m.testEndpoint,
		} {
			if !strings.Contains(src, want) {
				t.Errorf("%s missing %q", m.file, want)
			}
		}
		for _, banned := range []string{"/api/vault/secrets", "adg-save-btn", "SetBannerState", "adg-status-banner\"", "🟢", "🔴", "⚪", "🟡"} {
			if strings.Contains(src, banned) {
				t.Errorf("%s still contains %q", m.file, banned)
			}
		}
		for _, path := range m.secretPaths {
			if !strings.Contains(src, path) {
				t.Errorf("%s does not bind the secret %s to the save dock", m.file, path)
			}
		}
	}
}

func TestConfigWave1Batch1ModulesBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	browser := newSmokeBrowser(t)
	page := browser.MustPage(configRefreshOrigin(t) + "/config#overview")
	defer func() {
		page.MustEval(`() => window.removeEventListener('beforeunload', handleConfigBeforeUnload)`)
		page.MustClose()
	}()
	waitForJSBool(t, page, `() => !!document.querySelector('.pw-overview-card')`)
	page.MustSetViewport(1440, 900, 1, false)
	for _, m := range wave1Batch1Modules {
		m := m
		t.Run(m.section, func(t *testing.T) {
			page.MustEval(`async key => { await selectSection(key, {scrollBehavior:'auto'}); resetDirtySnapshot(); }`, m.section)
			state := page.MustEval(`(m) => {
				const banner = document.getElementById(m.bannerID);
				const result = document.getElementById(m.resultID);
				const button = document.getElementById(m.testButton);
				const secrets = m.secretPaths.map(path => document.querySelector('input[type="password"][data-path="' + path + '"]')).filter(Boolean);
				return {
					banner: !!banner && banner.classList.contains('cfg-state-banner') && ['off', 'unconfigured', 'checking', 'connected', 'degraded', 'error', 'unknown'].includes(banner.dataset.state),
					result: !!result && result.classList.contains('cfg-action-result') && result.getAttribute('role') === 'status',
					draft: !!button && button.dataset.configActionDraft === 'true',
					secrets: secrets.length >= 1,
					vaultButtons: document.querySelectorAll('#content .adg-save-btn, #content [onclick*="Vault"], #content [onclick*="SaveKey"], #content [onclick*="SaveAPIKey"]').length,
					vaultHint: !!document.querySelector('#content .cfg-save-scope'),
					emoji: /[\u{1F7E2}\u{1F534}\u{26AA}\u{1F7E1}]/u.test(banner ? banner.textContent : '')
				};
			}`, map[string]interface{}{"bannerID": m.bannerID, "resultID": m.resultID, "testButton": m.testButton, "secretPaths": m.secretPaths}).Map()
			for _, key := range []string{"banner", "result", "draft", "secrets"} {
				if !state[key].Bool() {
					t.Errorf("%s: %s check failed: %v", m.section, key, state)
				}
			}
			if state["vaultButtons"].Int() != 0 || state["vaultHint"].Bool() || state["emoji"].Bool() {
				t.Errorf("%s: leftover vault button, hint or emoji dot: %v", m.section, state)
			}

			// The test button posts the draft and writes the server reason next to the button.
			outcome := page.MustEval(`async (m) => {
				const original = window.fetch;
				const calls = [];
				window.fetch = (url, init) => {
					if (String(url) === m.testEndpoint) {
						calls.push({url: String(url), method: init && init.method, body: init && init.body});
						return Promise.resolve(new Response(JSON.stringify({status: 'error', message: 'Fixture says no'}), {status: 502, headers: {'Content-Type': 'application/json'}}));
					}
					return original(url, init);
				};
				const button = document.getElementById(m.testButton);
				button.dataset.configActionLocked = 'false';
				await window[m.testFn]();
				window.fetch = original;
				const result = document.getElementById(m.resultID);
				let draft = null;
				try { draft = JSON.parse(calls[0].body).draft; } catch (_) { draft = undefined; }
				return {calls: calls.length, method: calls[0] && calls[0].method, hasDraft: draft !== undefined && typeof draft === 'object', state: result.dataset.state, text: result.textContent, busy: button.dataset.configActionBusy};
			}`, map[string]interface{}{"testEndpoint": m.testEndpoint, "testButton": m.testButton, "testFn": m.testFn, "resultID": m.resultID}).Map()
			if outcome["calls"].Int() != 1 || outcome["method"].Str() != "POST" || !outcome["hasDraft"].Bool() {
				t.Errorf("%s: test request was not a POST with a draft: %v", m.section, outcome)
			}
			if outcome["state"].Str() != "error" || !strings.Contains(outcome["text"].Str(), "Fixture says no") {
				t.Errorf("%s: server reason is not shown next to the button: %v", m.section, outcome)
			}
			if outcome["busy"].Str() == "true" {
				t.Errorf("%s: test button stayed busy", m.section)
			}
		})
	}
}
