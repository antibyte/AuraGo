package ui

import (
	"strings"
	"testing"
)

// Config UX wave 1 review fixes: status words that are successes (token_required, supported,
// installed, stopped), the tailscale API test patch, here.now draft tests and the sudo banner.
func TestConfigWave1StatusWordsBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	browser := newSmokeBrowser(t)
	page := browser.MustPage(configRefreshOrigin(t) + "/config#overview")
	defer func() {
		page.MustEval(`() => window.removeEventListener('beforeunload', handleConfigBeforeUnload)`)
		page.MustClose()
	}()
	waitForJSBool(t, page, `() => !!document.querySelector('.pw-overview-card')`)
	page.MustSetViewport(1440, 900, 1, false)

	// remote_control: approving answers 200 token_required -> success, token card, device reload, token not in the line.
	rc := page.MustEval(`async () => {
		const original = window.fetch;
		const answer = (status, body) => Promise.resolve(new Response(JSON.stringify(body), {status, headers: {'Content-Type': 'application/json'}}));
		let reloads = 0;
		window.fetch = (url, init) => {
			url = String(url);
			if (url === '/api/remote/devices/pending-device/approve') return answer(200, {status: 'token_required', token: 'secret-one-time-token', expires_at: '2027-01-01T00:00:00Z'});
			if (url === '/api/remote/enroll') return answer(400, {error: 'enroll refused'});
			if (url === '/api/remote/devices') { reloads++; return answer(200, [{id: 'pending-device', name: 'Pending', status: 'pending'}]); }
			return original(url, init);
		};
		configData.remote_control = {enabled: true, allowed_paths: []};
		await selectSection('remote_control', {scrollBehavior: 'auto'});
		const line = document.getElementById('rc-token-status');
		const before = reloads;
		await rcCreateEnrollmentToken('pending-device');
		const ok = {state: line.dataset.state, text: line.textContent, token: document.getElementById('rc-token-value') && document.getElementById('rc-token-value').textContent, reloaded: reloads > before};
		await rcCreateEnrollmentToken();
		const bad = {state: line.dataset.state, text: line.textContent, card: !!document.getElementById('rc-token-value')};
		window.fetch = original;
		return {ok, bad};
	}`).Map()
	ok, bad := rc["ok"].Map(), rc["bad"].Map()
	if ok["state"].Str() != "success" || strings.Contains(ok["text"].Str(), "secret-one-time-token") || strings.Contains(ok["text"].Str(), "{") || ok["token"].Str() != "secret-one-time-token" || !ok["reloaded"].Bool() {
		t.Errorf("remote_control approve (token_required) must be a success with the token card only: %v", ok)
	}
	if bad["state"].Str() != "error" || !strings.Contains(bad["text"].Str(), "enroll refused") || bad["card"].Bool() {
		t.Errorf("remote_control enroll failure must stay an error with the server reason: %v", bad)
	}

	// virtual_computers: supported/installed are successes, unsupported a warning, unhealthy an error.
	page.MustEval(`async () => { configData.virtual_computers = Object.assign({}, configData.virtual_computers, {enabled: true, control_plane: {mode: 'local_host'}}); await selectSection('virtual_computers', {scrollBehavior: 'auto'}); resetDirtySnapshot(); }`)
	vc := page.MustEval(`async () => {
		const original = window.fetch;
		const line = document.getElementById('vc-cfg-result');
		const run = async (status, body) => {
			window.fetch = url => String(url).startsWith('/api/virtual-computers/setup/') ? Promise.resolve(new Response(JSON.stringify(body), {status, headers: {'Content-Type': 'application/json'}})) : original(url);
			document.getElementById('vc-preflight-btn').dataset.configActionLocked = 'false';
			await vcCfgPostSetup('/api/virtual-computers/setup/preflight', 'vc-preflight-btn');
			return {state: line.dataset.state, text: line.textContent};
		};
		const out = {
			supported: await run(200, {status: 'supported', message: 'Fine'}),
			warned: await run(200, {status: 'installed', message: 'Fine', warnings: ['low disk']}),
			unsupported: await run(200, {status: 'unsupported', message: 'Needs /dev/kvm'}),
			unhealthy: await run(200, {status: 'unhealthy', message: 'Agent down'}),
		};
		window.fetch = original;
		return out;
	}`).Map()
	for name, want := range map[string]string{"supported": "success", "warned": "warning", "unsupported": "warning", "unhealthy": "error"} {
		got := vc[name].Map()
		if got["state"].Str() != want {
			t.Errorf("virtual_computers %s: want %s, got %v", name, want, got)
		}
		if strings.Contains(got["text"].Str(), "{") || strings.Contains(got["text"].Str(), `"status"`) {
			t.Errorf("virtual_computers %s shows raw JSON: %v", name, got)
		}
	}
	if !strings.Contains(vc["warned"].Map()["text"].Str(), "low disk") || !strings.Contains(vc["unsupported"].Map()["text"].Str(), "/dev/kvm") {
		t.Errorf("virtual_computers warnings/unsupported reason missing: %v", vc)
	}

	// virtual_computers sudo banner: it starts on checking and settles through cfgStatusBanner.check.
	sudo := page.MustEval(`async () => {
		const original = window.fetch;
		const el = document.getElementById('vc-sudo-password-status');
		let release;
		window.fetch = url => String(url) === '/api/virtual-computers/setup/status' ? new Promise(resolve => { release = () => resolve(new Response(JSON.stringify({sudo_password_stored: true}), {status: 200})); }) : original(url);
		const pending = vcCfgRefreshSudoPasswordStatus();
		const during = el.dataset.state;
		release();
		await pending;
		const after = el.dataset.state;
		window.fetch = url => String(url) === '/api/virtual-computers/setup/status' ? Promise.resolve(new Response('boom', {status: 500})) : original(url);
		await vcCfgRefreshSudoPasswordStatus();
		const failed = {state: el.dataset.state, retry: !!el.querySelector('.cfg-state-retry')};
		window.fetch = original;
		return {during, after, failed};
	}`).Map()
	failed := sudo["failed"].Map()
	if sudo["during"].Str() != "checking" || sudo["after"].Str() != "connected" || failed["state"].Str() != "error" || !failed["retry"].Bool() {
		t.Errorf("sudo banner must use cfgStatusBanner.check (checking -> state, error with retry): %v", sudo)
	}

	// tailscale: the API test sends only the API fields; stopping the tsnet node is a plain success.
	page.MustEval(`async () => { configData.tailscale = Object.assign({}, configData.tailscale, {enabled: true, tailnet: 'example.com', tsnet: Object.assign({}, (configData.tailscale || {}).tsnet, {enabled: true, hostname: 'aurago-x'})}); await selectSection('tailscale', {scrollBehavior: 'auto'}); resetDirtySnapshot(); }`)
	ts := page.MustEval(`async () => {
		const original = window.fetch;
		const calls = [];
		window.fetch = (url, init) => {
			url = String(url);
			if (url === '/api/tailscale/test') { calls.push(init.body); return Promise.resolve(new Response(JSON.stringify({status: 'ok', message: 'fine', count: 2}), {status: 200})); }
			if (url === '/api/tsnet/stop') return Promise.resolve(new Response(JSON.stringify({status: 'stopped'}), {status: 200}));
			if (url.startsWith('/api/tsnet/')) return Promise.resolve(new Response(JSON.stringify({}), {status: 200}));
			return original(url, init);
		};
		document.getElementById('ts-api-test-btn').dataset.configActionLocked = 'false';
		await tsApiTestConnection();
		const draft = JSON.parse(calls[0]).draft;
		const out = {draftKeys: Object.keys(draft.tailscale || {}).sort().join(','), hasTsnet: !!(draft.tailscale && draft.tailscale.tsnet), stopFound: false};
		_tsnetRequireSavedConfig = () => true;
		const stop = document.getElementById('tsnet-btn-stop');
		if (stop) {
			out.stopFound = true;
			stop.dataset.configActionLocked = 'false';
			await _tsnetStop();
			const line = document.getElementById('tsnet-action-result');
			out.stopState = line.dataset.state;
			out.stopText = line.textContent;
		}
		window.fetch = original;
		return out;
	}`).Map()
	if ts["hasTsnet"].Bool() {
		t.Errorf("tailscale API test must not send tsnet fields: %v", ts)
	}
	if !ts["stopFound"].Bool() {
		t.Error("tsnet stop button not rendered in the fixture")
	} else if ts["stopState"].Str() != "success" || strings.Contains(ts["stopText"].Str(), "{") {
		t.Errorf("tsnet stop must be a plain success: %v", ts)
	}

	// here_now: a draft test must not turn the saved-state banner into Connected.
	page.MustEval(`async () => { await selectSection('here_now', {scrollBehavior: 'auto'}); resetDirtySnapshot(); }`)
	hn := page.MustEval(`async () => {
		const original = window.fetch;
		const banner = document.getElementById('hn-status-banner');
		const before = banner ? banner.dataset.state : '';
		window.fetch = (url, init) => String(url) === '/api/here-now/test-connection' ? Promise.resolve(new Response(JSON.stringify({status: 'ok', accounts: []}), {status: 200})) : original(url, init);
		document.getElementById('hn-test-btn').dataset.configActionLocked = 'false';
		await hereNowTestConnection();
		window.fetch = original;
		const after = banner ? banner.dataset.state : '';
		return {before, after, result: document.getElementById('hn-test-result').dataset.state};
	}`).Map()
	if hn["result"].Str() != "success" || hn["before"].Str() != hn["after"].Str() {
		t.Errorf("here_now draft test must not change the saved-state banner: %v", hn)
	}
}
