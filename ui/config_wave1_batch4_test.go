package ui

import (
	"regexp"
	"strings"
	"testing"
)

// Config UX wave 1, batch 4: a2a, github, here_now, homepage, netlify, remote_control,
// tailscale, vercel, virtual_computers and virtual_desktop use the shared result line,
// state banner, save dock for secrets and draft tests.
type wave1Batch4Module struct {
	section, file, testButton, resultID, bannerID, testFn, testEndpoint, draftSection string
	secretPaths                                                                       []string
}

// Modules whose test button is rendered whatever the fixture configuration is.
var wave1Batch4Modules = []wave1Batch4Module{
	{"github", "cfg/github.js", "github-test-btn", "github-test-result", "", "githubTestConnection", "/api/github/repos", "github", []string{"github.token"}},
	{"here_now", "cfg/here_now.js", "hn-test-btn", "hn-test-result", "hn-status-banner", "hereNowTestConnection", "/api/here-now/test-connection", "here_now", []string{"here_now.api_key"}},
	{"netlify", "cfg/netlify.js", "nf-test-btn", "nf-test-result", "nf-status-banner", "nfTestConnection", "/api/netlify/test-connection", "netlify", []string{"netlify.token"}},
	{"tailscale", "cfg/tailscale.js", "ts-api-test-btn", "ts-api-test-result", "ts-api-status-banner", "tsApiTestConnection", "/api/tailscale/test", "tailscale.enabled", []string{"tailscale.api_key"}},
	{"vercel", "cfg/vercel.js", "vercel-test-btn", "vercel-test-result", "vercel-status-banner", "vercelTestConnection", "/api/vercel/test-connection", "vercel", []string{"vercel.token"}},
}

var wave1Batch4EmojiDots = []string{"🟢", "🔴", "⚪", "🟡"}

func TestConfigWave1Batch4DraftModulesUseSharedFeedback(t *testing.T) {
	t.Parallel()
	for _, m := range wave1Batch4Modules {
		src := readWave0File(t, m.file)
		for _, want := range []string{
			"cfgActionResult(",
			"cfgDraftPatch(['" + m.draftSection + "'",
			`data-config-action-draft="true"`,
			m.testEndpoint,
		} {
			if !strings.Contains(src, want) {
				t.Errorf("%s missing %q", m.file, want)
			}
		}
		if m.bannerID != "" && !strings.Contains(src, "cfgStatusBanner") {
			t.Errorf("%s does not use cfgStatusBanner", m.file)
		}
		for _, banned := range append([]string{"/api/vault/secrets", "adg-save-btn", "wh-notice nf-status", "adg-status-banner"}, wave1Batch4EmojiDots...) {
			if strings.Contains(src, banned) {
				t.Errorf("%s still contains %q", m.file, banned)
			}
		}
		for _, path := range m.secretPaths {
			if !strings.Contains(src, `data-path="`+path+`"`) {
				t.Errorf("%s does not bind the secret %s to the save dock", m.file, path)
			}
		}
	}
}

func TestConfigWave1Batch4SecretsAndRuntimeActions(t *testing.T) {
	t.Parallel()

	tailscale := readWave0File(t, "cfg/tailscale.js")
	if !strings.Contains(tailscale, `data-path="tailscale.tsnet.auth_key"`) {
		t.Error("tailscale: the tsnet auth key is not in the save dock")
	}
	if regexp.MustCompile(`(^|[^A-Za-z.])(confirm|alert)\(`).MatchString(tailscale) || !strings.Contains(tailscale, "showConfirm(") {
		t.Error("tailscale: native confirm()/alert() must be replaced by showConfirm")
	}
	for _, want := range []string{"tsnet-node-result-", "_tsnetNodeAction", "cfgActionResult.html('tsnet-action-result')"} {
		if !strings.Contains(tailscale, want) {
			t.Errorf("tailscale: node results lack %q", want)
		}
	}

	a2a := readWave0File(t, "cfg/a2a.js")
	for _, want := range []string{
		`data-path="a2a.auth.api_key"`, `data-path="a2a.auth.bearer_secret"`,
		`.api_key" value=`, `.bearer_token" value=`,
		`id="a2a-test-btn" data-config-action-draft="false"`, `id="a2a-ra-test-' + idx + '" class="btn-save a2a-sm-btn" data-config-action-draft="false"`,
		"cfgStatusBanner.check(", "a2aTestRemoteAgent",
	} {
		if !strings.Contains(a2a, want) {
			t.Errorf("a2a missing %q", want)
		}
	}
	for _, banned := range append([]string{"a2aSaveVault", "/api/vault/secrets", "adg-save-btn", "a2a-save-btn-xs"}, wave1Batch4EmojiDots...) {
		if strings.Contains(a2a, banned) {
			t.Errorf("a2a still contains %q", banned)
		}
	}

	homepage := readWave0File(t, "cfg/homepage.js")
	for _, want := range []string{`data-path="homepage.deploy_password"`, `data-path="homepage.deploy_key"`, `id="hp-test-btn" data-config-action-draft="true"`, "cfgDraftPatch(['homepage'])"} {
		if !strings.Contains(homepage, want) {
			t.Errorf("homepage missing %q", want)
		}
	}
	for _, banned := range []string{"hpSaveCredentials", "/api/vault/secrets", "hp-cred-status"} {
		if strings.Contains(homepage, banned) {
			t.Errorf("homepage still contains %q", banned)
		}
	}

	vc := readWave0File(t, "cfg/virtual_computers.js")
	if got := strings.Count(vc, "/api/vault/secrets"); got != 1 || !strings.Contains(vc, "key: 'sudo_password'") {
		t.Errorf("virtual_computers: only the sudo password may use the vault endpoint (found %d uses)", got)
	}
	for _, want := range []string{"vc-sudo-save-btn", "cfgActionResult.html('vc-sudo-save-result')", "cfgActionResult.html('vc-cfg-result')", `data-config-action-draft="true"`} {
		if !strings.Contains(vc, want) {
			t.Errorf("virtual_computers missing %q", want)
		}
	}

	desktop := readWave0File(t, "cfg/virtual_desktop.js")
	if got := strings.Count(desktop, `data-config-action-draft="false"`); got != 3 {
		t.Errorf("virtual_desktop: want 3 runtime-only status checks, found %d", got)
	}
	for _, want := range []string{"cfgActionResult.html('vd-cfg-result')", "cfgActionResult.html('vd-video-studio-result')", "cfgActionResult.html('vd-retro-games-result')"} {
		if !strings.Contains(desktop, want) {
			t.Errorf("virtual_desktop missing %q", want)
		}
	}
	if strings.Contains(desktop, "dc-test-result") {
		t.Error("virtual_desktop still uses dc-test-result lines")
	}

	remote := readWave0File(t, "cfg/remote_control.js")
	if !strings.Contains(remote, "cfgActionResult.html('rc-token-status')") || !strings.Contains(remote, `id="rc-token-result"`) {
		t.Error("remote_control: the status line must use cfgActionResult and the token stays in #rc-token-result")
	}
	if strings.Contains(remote, "adg-test-result") {
		t.Error("remote_control still uses adg-test-result lines")
	}
}

func TestConfigWave1Batch4ModulesBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	browser := newSmokeBrowser(t)
	page := browser.MustPage(configRefreshOrigin(t) + "/config#overview")
	defer func() {
		page.MustEval(`() => window.removeEventListener('beforeunload', handleConfigBeforeUnload)`)
		page.MustClose()
	}()
	waitForJSBool(t, page, `() => !!document.querySelector('.pw-overview-card')`)
	page.MustSetViewport(1440, 900, 1, false)
	for _, m := range wave1Batch4Modules {
		m := m
		t.Run(m.section, func(t *testing.T) {
			page.MustEval(`async key => { await selectSection(key, {scrollBehavior:'auto'}); resetDirtySnapshot(); }`, m.section)
			state := page.MustEval(`(m) => {
				const banner = m.bannerID ? document.getElementById(m.bannerID) : null;
				const result = document.getElementById(m.resultID);
				const button = document.getElementById(m.testButton);
				const secrets = m.secretPaths.map(path => document.querySelector('input[type="password"][data-path="' + path + '"]')).filter(Boolean);
				return {
					banner: !m.bannerID || (!!banner && banner.classList.contains('cfg-state-banner') && ['off', 'unconfigured', 'checking', 'connected', 'degraded', 'error', 'unknown'].includes(banner.dataset.state)),
					result: !!result && result.classList.contains('cfg-action-result') && result.getAttribute('role') === 'status',
					draft: !!button && button.dataset.configActionDraft === 'true',
					secrets: secrets.length >= 1,
					vaultButtons: document.querySelectorAll('#content .adg-save-btn, #content [onclick*="Vault"], #content [onclick*="SaveKey"], #content [onclick*="SaveToken"]').length,
					emoji: /[\u{1F7E2}\u{1F534}\u{26AA}\u{1F7E1}]/u.test(banner ? banner.textContent : '')
				};
			}`, map[string]interface{}{"bannerID": m.bannerID, "resultID": m.resultID, "testButton": m.testButton, "secretPaths": m.secretPaths}).Map()
			for _, key := range []string{"banner", "result", "draft", "secrets"} {
				if !state[key].Bool() {
					t.Errorf("%s: %s check failed: %v", m.section, key, state)
				}
			}
			if state["vaultButtons"].Int() != 0 || state["emoji"].Bool() {
				t.Errorf("%s: leftover vault button or emoji dot: %v", m.section, state)
			}

			// The test button posts the draft and writes the server reason next to the button.
			outcome := page.MustEval(`async (m) => {
				const original = window.fetch;
				const calls = [];
				window.fetch = (url, init) => {
					if (String(url) === m.testEndpoint && init && init.method === 'POST') {
						calls.push({url: String(url), method: init.method, body: init.body});
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

func TestConfigWave1Batch4RuntimeAndListSecretsBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	browser := newSmokeBrowser(t)
	page := browser.MustPage(configRefreshOrigin(t) + "/config#overview")
	defer func() {
		page.MustEval(`() => window.removeEventListener('beforeunload', handleConfigBeforeUnload)`)
		page.MustClose()
	}()
	waitForJSBool(t, page, `() => !!document.querySelector('.pw-overview-card')`)
	page.MustSetViewport(1440, 900, 1, false)

	// a2a: remote agent credentials are bound to their list index; runtime tests lock while unsaved.
	page.MustEval(`async () => { await selectSection('a2a', {scrollBehavior:'auto'}); resetDirtySnapshot(); }`)
	a2a := page.MustEval(`async () => {
		const banner = document.getElementById('a2a-status-banner');
		a2aAddRemoteAgent();
		const key = document.getElementById('a2a-ra-apikey-0');
		const bearer = document.getElementById('a2a-ra-bearer-0');
		key.value = 'typed-agent-key';
		key.dispatchEvent(new Event('input', {bubbles: true}));
		key.dispatchEvent(new Event('change', {bubbles: true}));
		await new Promise(resolve => setTimeout(resolve, 50));
		const button = document.getElementById('a2a-ra-test-0');
		window.AuraConfigActions.refresh();
		const reason = document.getElementById('a2a-ra-test-0-reason');
		return {
			banner: !!banner && banner.classList.contains('cfg-state-banner'),
			keyPath: key.dataset.path, bearerPath: bearer.dataset.path,
			draftKey: window.AuraConfigState.get('a2a.client.remote_agents.0.api_key'),
			locked: button.dataset.configActionLocked, reason: reason ? reason.textContent : '',
			dockButtons: document.querySelectorAll('#content .adg-save-btn, #content .a2a-save-btn-xs').length
		};
	}`).Map()
	if !a2a["banner"].Bool() || a2a["keyPath"].Str() != "a2a.client.remote_agents.0.api_key" || a2a["bearerPath"].Str() != "a2a.client.remote_agents.0.bearer_token" {
		t.Errorf("a2a markup is not on the shared helpers: %v", a2a)
	}
	if a2a["draftKey"].Str() != "typed-agent-key" || a2a["dockButtons"].Int() != 0 {
		t.Errorf("a2a remote agent key did not reach the draft or a vault button is left: %v", a2a)
	}
	if a2a["locked"].Str() != "true" || strings.TrimSpace(a2a["reason"].Str()) == "" {
		t.Errorf("a2a runtime test must be locked with a reason while changes are unsaved: %v", a2a)
	}

	// virtual_desktop: an HTTP failure is an error with the server reason, a disabled desktop is a warning.
	page.MustEval(`async () => { await selectSection('virtual_desktop', {scrollBehavior:'auto'}); resetDirtySnapshot(); }`)
	desktop := page.MustEval(`async () => {
		const original = window.fetch;
		const answer = (status, body) => Promise.resolve(new Response(JSON.stringify(body), {status, headers: {'Content-Type': 'application/json'}}));
		const result = document.getElementById('vd-cfg-result');
		document.getElementById('vd-cfg-test-btn').dataset.configActionLocked = 'false';
		const out = {};
		window.fetch = url => String(url) === '/api/desktop/bootstrap' ? answer(500, {error: 'desktop exploded'}) : original(url);
		await vdCfgTestDesktop();
		out.failed = {state: result.dataset.state, text: result.textContent};
		document.getElementById('vd-cfg-test-btn').dataset.configActionLocked = 'false';
		window.fetch = url => String(url) === '/api/desktop/bootstrap' ? answer(200, {enabled: false}) : original(url);
		await vdCfgTestDesktop();
		out.disabled = {state: result.dataset.state, text: result.textContent};
		window.fetch = original;
		return out;
	}`).Map()
	failed, disabled := desktop["failed"].Map(), desktop["disabled"].Map()
	if failed["state"].Str() != "error" || !strings.Contains(failed["text"].Str(), "desktop exploded") {
		t.Errorf("virtual_desktop must show the HTTP failure as an error: %v", failed)
	}
	if disabled["state"].Str() != "warning" || strings.Contains(disabled["text"].Str(), "desktop exploded") {
		t.Errorf("virtual_desktop must show a disabled desktop as a warning: %v", disabled)
	}
}
