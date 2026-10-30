package ui

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/go-rod/rod"
)

// Shared feedback building blocks of the config page (js/config/feedback.js).

func TestConfigFeedbackScriptContract(t *testing.T) {
	t.Parallel()

	feedback := normalizeAssetText(mustReadUIFile(t, "js/config/feedback.js"))
	for _, marker := range []string{
		"window.AuraConfigFeedback",
		"read: read",
		"reason: reason",
		"busy: busy",
		"actionResult: actionResult",
		"showResult: showResult",
		"resultHTML: resultHTML",
		"statusBanner: statusBanner",
		"bannerHTML: bannerHTML",
		"checkBanner: checkBanner",
		"draftPatch: draftPatch",
		"BANNER_STATES",
		"window.cfgActionResult",
		"window.cfgStatusBanner",
		"window.cfgResponseReason",
		"window.cfgDraftPatch",
		"cfgActionResult.show",
		"cfgActionResult.html",
		"cfgStatusBanner.html",
		"cfgStatusBanner.check",
		`data-config-action-busy`,
		`aria-busy`,
		`cfg-action-result`,
		`cfg-state-banner`,
		`config.common.network_error`,
	} {
		if !strings.Contains(feedback, marker) {
			t.Errorf("feedback.js missing %q", marker)
		}
	}
	// The legacy audit test and the dead CSS reserve these names.
	for _, forbidden := range []string{"cfg-status-banner", "cfg-status-"} {
		if strings.Contains(feedback, forbidden) {
			t.Errorf("feedback.js must not use the reserved class name %q", forbidden)
		}
	}

	html := normalizeAssetText(mustReadUIFile(t, "config.html"))
	state := strings.Index(html, "/js/config/state.js")
	feedbackAt := strings.Index(html, "/js/config/feedback.js")
	actions := strings.Index(html, "/js/config/actions.js")
	if state < 0 || feedbackAt < state || actions < feedbackAt {
		t.Error("config.html must load feedback.js between state.js and actions.js")
	}

	actionsJS := normalizeAssetText(mustReadUIFile(t, "js/config/actions.js"))
	for _, marker := range []string{"data-config-action-draft", "configActionDraft", "config.feedback.runtime_only", "data-cfg-confirm-enable", "confirmOnEnable", "spec.result"} {
		if !strings.Contains(actionsJS, marker) {
			t.Errorf("actions.js missing %q", marker)
		}
	}

	css := normalizeAssetText(mustReadUIFile(t, "css/config-workspace.css"))
	for _, marker := range []string{".cfg-action-result", ".cfg-state-banner", ".cfg-state-dot", ".cfg-state-label", ".cfg-state-detail", ".cfg-state-retry", "prefers-reduced-motion"} {
		if !strings.Contains(css, marker) {
			t.Errorf("config-workspace.css missing %q", marker)
		}
	}
	if strings.Contains(css, ".cfg-status-") {
		t.Error("config-workspace.css must not define .cfg-status-* classes")
	}

	builder := normalizeAssetText(mustReadUIFile(t, "cfg/form-builder.js"))
	for _, marker := range []string{"actionResult,", "statusBanner,", `' data-path="' + attr(options.path)`, ".cfg-state-banner"} {
		if !strings.Contains(builder, marker) {
			t.Errorf("form-builder.js missing %q", marker)
		}
	}

	integration := normalizeAssetText(mustReadUIFile(t, "js/config/integration_actions.js"))
	for _, marker := range []string{"cfgActionResult.html", `data-config-action-draft="true"`, "draftPatch", "JSON.stringify({ draft:", "successDetail: true"} {
		if !strings.Contains(integration, marker) {
			t.Errorf("integration_actions.js missing %q", marker)
		}
	}

	catalog := normalizeAssetText(mustReadUIFile(t, "js/config/catalog.js"))
	for _, path := range []string{
		"composio.allow_destructive", "github.allow_delete", "here_now.allow_delete", "network_shares.allow_delete", "huggingface.allow_delete",
		"game_maker.allow_delete", "jellyfin.allow_destructive", "obsidian.allow_destructive", "truenas.allow_destructive",
		"rustfs.permissions.allow_delete", "proxmox.allow_destructive", "agent.sudo_enabled", "agent.sudo_unrestricted", "agent.allow_shell",
		"agent.allow_python", "agent.allow_unsafe_host_execution", "docker.allow_host_access", "agent.allow_filesystem_write",
		"agent.allow_remote_shell", "agent.allow_self_update", "agent.allow_package_manager", "agent.allow_unsandboxed_shell",
		"shell_sandbox.allow_unsafe_fallback", "sandbox.allow_local_python_fallback", "huggingface.allow_job_token_injection",
	} {
		if !strings.Contains(catalog, "'"+path+"'") {
			t.Errorf("catalog.js confirmOnEnable missing %q", path)
		}
	}
	if !strings.Contains(normalizeAssetText(mustReadUIFile(t, "cfg/mcp.js")), `id="mcp-m-allow-destructive" type="checkbox" data-cfg-confirm-enable`) {
		t.Error("the MCP modal destructive checkbox must ask before it is turned on")
	}
	if !strings.Contains(normalizeAssetText(mustReadUIFile(t, "cfg/sql_connections.js")), `id="sqlconn-perm-delete" data-cfg-confirm-enable`) {
		t.Error("the SQL modal delete permission must ask before it is turned on")
	}
}

// feedbackPage loads the real config scripts into a fixture page with minimal i18n,
// confirm and toast stubs. Answers to showConfirm come from window.__answer.
func feedbackPage(t *testing.T, body string) *rod.Page {
	t.Helper()
	requirePrecisionBrowserSmoke(t)
	browser := newSmokeBrowser(t)
	page := browser.MustPage(newPrecisionSmokeOrigin(t))
	t.Cleanup(func() { page.MustClose() })
	page.MustSetDocumentContent("<!doctype html><html><head><meta charset=\"utf-8\"></head><body><div id=\"content\">" + body + "</div></body></html>")
	page.MustEval(`() => {
		window.I18N = {
			'config.common.network_error': 'Network error',
			'config.feedback.running': 'Running',
			'config.feedback.done': 'Done',
			'config.feedback.failed': 'Failed',
			'config.feedback.runtime_only': 'Runtime only',
			'config.feedback.timeout': 'No answer in time',
			'config.status.error': 'Error',
			'config.status.connected': 'Connected',
			'config.status.unknown': 'Unknown',
			'config.status.retry': 'Check again',
			'config.precision.action_save_first': 'Save first',
			'config.precision.action_missing_fields': 'Missing fields',
			'config.precision.action_missing_credential': 'Missing credential',
			'config.confirm_enable.title': 'Enable {label}?',
			'config.confirm_enable.host': 'Host rights for {label}',
			'config.confirm_enable.delete': 'Delete rights'
		};
		window.t = (key, params) => {
			let text = window.I18N[key] || key;
			Object.entries(params || {}).forEach(([name, value]) => { text = text.replaceAll('{' + name + '}', () => value); });
			return text;
		};
		window.__confirms = [];
		window.__answer = true;
		window.showConfirm = async (title, message) => { window.__confirms.push([title, message]); return window.__answer; };
		window.__toasts = [];
		window.showToast = (message, kind) => { window.__toasts.push([message, kind]); };
	}`)
	for _, script := range []string{"js/config/catalog.js", "js/config/state.js", "js/config/feedback.js", "js/config/actions.js"} {
		if err := page.AddScriptTag("", normalizeAssetText(mustReadUIFile(t, script))); err != nil {
			t.Fatalf("load %s: %v", script, err)
		}
	}
	return page
}

// evalFailures runs a script that collects failed expectations and fails the test with all of them.
func evalFailures(t *testing.T, page *rod.Page, script string) {
	t.Helper()
	raw := page.MustEval(script).String()
	var failures []string
	if err := json.Unmarshal([]byte(raw), &failures); err != nil {
		t.Fatalf("unexpected result %q: %v", raw, err)
	}
	for _, failure := range failures {
		t.Error(failure)
	}
}

func TestConfigFeedbackReadBrowser(t *testing.T) {
	page := feedbackPage(t, "")
	evalFailures(t, page, `async () => {
		const failures = [];
		const expect = (name, actual, wanted) => { if (JSON.stringify(actual) !== JSON.stringify(wanted)) failures.push(name + ': got ' + JSON.stringify(actual) + ', want ' + JSON.stringify(wanted)); };
		const F = window.AuraConfigFeedback;
		const response = (body, status) => new Response(typeof body === 'string' ? body : JSON.stringify(body), { status: status || 200 });
		const pick = async (value, opts) => { const out = await F.read(value, opts); return [out.state, out.reason, out.httpStatus]; };

		expect('error+details', await pick(response({ error: 'boom', details: 'why' }, 500)), ['error', 'boom why', 500]);
		expect('error only', await pick(response({ error: 'nope' }, 400)), ['error', 'nope', 400]);
		expect('200 status error', await pick(response({ status: 'error', message: 'bad host' })), ['error', 'bad host', 200]);
		expect('status ok', (await pick(response({ status: 'ok' })))[0], 'success');
		for (const status of ['success', 'connected', 'online', 'ready', 'running', 'healthy']) {
			expect('status ' + status, (await pick(response({ status })))[0], 'success');
		}
		for (const status of ['partial', 'warning', 'degraded', 'untested', 'skipped']) {
			expect('status ' + status, (await pick(response({ status, message: 'm' })))[0], 'warning');
		}
		expect('success true', (await pick(response({ success: true })))[0], 'success');
		expect('ok true', (await pick(response({ ok: true })))[0], 'success');
		expect('success false', (await pick(response({ success: false, message: 'denied' }))), ['error', 'denied', 200]);
		expect('ok false', (await pick(response({ ok: false, errors: ['first', 'second'] }))), ['error', 'first', 200]);
		expect('connected false', (await pick(response({ connected: false })))[0], 'error');
		expect('error field with 200', (await pick(response({ error: 'late failure' }))), ['error', 'late failure', 200]);
		expect('detail', (await pick(response({ status: 'error', detail: 'by detail' })))[1], 'by detail');
		expect('reason key', (await pick(response({ status: 'error', reason: 'by reason' })))[1], 'by reason');
		expect('plain text', await pick(response('gateway timeout', 504)), ['error', 'gateway timeout', 504]);
		expect('html is not shown', await pick(response('<html><body>Bad</body></html>', 500)), ['error', 'HTTP 500', 500]);
		expect('empty body error', await pick(response('', 502)), ['error', 'HTTP 502', 502]);
		expect('empty body ok', (await pick(new Response(null, { status: 204 })))[0], 'success');
		expect('success message', (await F.read(response({ status: 'ok', message: 'Version 1.2' }))).reason, 'Version 1.2');
		expect('already parsed body', (await pick({ status: 'error', message: 'parsed' }))[0], 'error');
		expect('network error', await pick(new TypeError('Failed to fetch')), ['error', 'Network error', 0]);
		expect('abort', (await F.read(new DOMException('aborted', 'AbortError'))).state, 'cancelled');
		expect('reasonText wins', (await F.read(response({ error: 'raw' }, 500), { reasonText: 'custom' })).reason, 'custom');
		expect('reasonText fn', (await F.read(response({ error: 'code_7' }, 500), { reasonText: body => 'Translated ' + body.error })).reason, 'Translated code_7');
		const long = (await F.read(response({ error: 'x'.repeat(900) }, 500))).reason;
		expect('reason is capped', long.length <= 300, true);
		expect('cfgResponseReason', window.cfgResponseReason({ message: 'direct' }), 'direct');
		const twice = response({ status: 'ok' });
		await F.read(twice);
		expect('body read once', twice.bodyUsed, true);
		return JSON.stringify(failures);
	}`)
}

func TestConfigFeedbackActionResultBrowser(t *testing.T) {
	page := feedbackPage(t, `<div class="cfg-actions-row"><button id="act-btn" type="button">Run</button><span id="act-btn-result"></span></div>
		<div id="host"><button id="stale-btn" type="button">Stale</button><span id="stale-btn-result"></span></div>
		<button id="sibling" type="button">Other</button><button id="sibling-off" type="button" disabled>Off</button>`)
	evalFailures(t, page, `async () => {
		const failures = [];
		const expect = (name, actual, wanted) => { if (JSON.stringify(actual) !== JSON.stringify(wanted)) failures.push(name + ': got ' + JSON.stringify(actual) + ', want ' + JSON.stringify(wanted)); };
		const ok = body => new Response(JSON.stringify(body), { status: 200 });
		const button = document.getElementById('act-btn');
		const line = document.getElementById('act-btn-result');

		// Busy markers during the run, restored afterwards; the label stays.
		let during = null;
		let pending = null;
		const first = window.cfgActionResult(button, async () => {
			during = [button.disabled, button.getAttribute('aria-busy'), button.dataset.configActionBusy, button.textContent];
			pending = [line.dataset.state, line.textContent];
			const skipped = await window.cfgActionResult(button, async () => ok({}), {});
			during.push(skipped.skipped);
			return ok({ status: 'ok', message: 'Server says hi' });
		}, { success: 'Worked', successDetail: false, lock: ['#sibling', '#sibling-off'] });
		const siblingDuring = document.getElementById('sibling').disabled;
		const outcome = await first;
		expect('busy during', during, [true, 'true', 'true', 'Run', 'busy']);
		expect('sibling locked', siblingDuring, true);
		expect('pending line', pending, ['pending', 'Running']);
		expect('success state', [outcome.state, line.dataset.state, line.textContent], ['success', 'success', 'Worked']);
		expect('busy after', [button.disabled, button.getAttribute('aria-busy'), button.dataset.configActionBusy], [false, 'false', 'false']);
		expect('siblings restored', [document.getElementById('sibling').disabled, document.getElementById('sibling-off').disabled], [false, true]);
		expect('line semantics', [line.getAttribute('role'), line.getAttribute('aria-live'), line.classList.contains('cfg-action-result')], ['status', 'polite', true]);

		// successDetail shows the server text; failures carry the reason.
		await window.cfgActionResult(button, async () => ok({ status: 'ok', message: 'Version 2' }), { success: 'Connected.', successDetail: true });
		expect('success detail', line.textContent, 'Connected. Version 2');
		await window.cfgActionResult(button, async () => new Response(JSON.stringify({ error: 'unauthorized' }), { status: 401 }), { failure: 'Test failed.' });
		expect('failure reason', [line.dataset.state, line.textContent], ['error', 'Test failed. unauthorized']);
		await window.cfgActionResult(button, async () => ok({ status: 'partial', message: 'IMAP fine, SMTP down' }));
		expect('warning', [line.dataset.state, line.textContent], ['warning', 'IMAP fine, SMTP down']);

		// Never rejects: a thrown error and a network error end in the line.
		let rejected = false;
		const thrown = await window.cfgActionResult(button, async () => { throw new Error('exploded'); }).catch(() => { rejected = true; });
		expect('throw does not reject', rejected, false);
		expect('throw shown', [thrown.state, line.dataset.state, line.textContent], ['error', 'error', 'Failed exploded']);
		await window.cfgActionResult(button, async () => { throw new TypeError('Failed to fetch'); }, { failure: 'Test failed.' });
		expect('network error shown', line.textContent, 'Test failed. Network error');
		expect('button usable after errors', [button.disabled, button.dataset.configActionBusy], [false, 'false']);
		await window.cfgActionResult(button, async () => { throw new DOMException('x', 'AbortError'); });
		expect('abort clears the line', [line.textContent, line.dataset.state], ['', undefined]);

		// The result line is created when none exists, after the reason line.
		const lone = document.createElement('button');
		lone.id = 'lone';
		document.getElementById('content').append(lone);
		await window.cfgActionResult(lone, async () => ok({ ok: true }));
		const created = document.getElementById('lone-result');
		expect('created line', [!!created, created && created.previousElementSibling === lone, created && created.textContent], [true, true, 'Done']);

		// Locked and confirm.
		button.dataset.configActionLocked = 'true';
		expect('locked', (await window.cfgActionResult(button, async () => ok({}))).skipped, 'locked');
		button.dataset.configActionLocked = 'false';
		window.__answer = false;
		let ran = false;
		const declined = await window.cfgActionResult(button, async () => { ran = true; return ok({}); }, { confirm: { title: 'Sure?', message: 'Really' } });
		expect('confirm declined', [declined.skipped, ran, window.__confirms.length], ['cancelled', false, 1]);
		window.__answer = true;
		await window.cfgActionResult(button, async () => { ran = true; return ok({}); }, { confirm: 'Really' });
		expect('confirm accepted', ran, true);

		// Toasts only on request.
		window.__toasts.length = 0;
		await window.cfgActionResult(button, async () => ok({ error: 'x' }), { toast: 'error' });
		await window.cfgActionResult(button, async () => ok({}), { toast: 'error' });
		expect('toast on error only', window.__toasts.length, 1);

		// A run that finishes after a newer run on a re-rendered button does not overwrite it.
		const host = document.getElementById('host');
		let release;
		const gate = new Promise(resolve => { release = resolve; });
		const slow = window.cfgActionResult(document.getElementById('stale-btn'), async () => { await gate; return ok({ error: 'old run failed' }); }, { failure: 'OLD' });
		host.innerHTML = '<button id="stale-btn" type="button">Stale</button><span id="stale-btn-result"></span>';
		await window.cfgActionResult(document.getElementById('stale-btn'), async () => ok({ status: 'ok' }), { success: 'NEW' });
		release();
		await slow;
		expect('stale run ignored', document.getElementById('stale-btn-result').textContent, 'NEW');

		// register(id, {run, result}) goes through the same busy system.
		const reg = document.createElement('button');
		reg.id = 'reg-btn';
		document.getElementById('content').append(reg);
		const regLine = document.createElement('span');
		regLine.id = 'reg-line';
		document.getElementById('content').append(regLine);
		let seen = null;
		window.AuraConfigActions.register('reg-btn', { run: async () => { seen = reg.dataset.configActionBusy; return ok({ status: 'ok' }); }, result: { target: 'reg-line', success: 'Registered' } });
		reg.click();
		await new Promise(resolve => setTimeout(resolve, 50));
		expect('register result', [seen, regLine.textContent, reg.dataset.configActionBusy], ['true', 'Registered', 'false']);

		// html helpers
		expect('result html', window.cfgActionResult.html('x-result').includes('role="status"') && window.cfgActionResult.html('x-result').includes('id="x-result"'), true);
		return JSON.stringify(failures);
	}`)
}

func TestConfigFeedbackStatusBannerBrowser(t *testing.T) {
	page := feedbackPage(t, `<div id="b1"></div><div id="b2"></div><div id="b3"></div>`)
	evalFailures(t, page, `async () => {
		const failures = [];
		const expect = (name, actual, wanted) => { if (JSON.stringify(actual) !== JSON.stringify(wanted)) failures.push(name + ': got ' + JSON.stringify(actual) + ', want ' + JSON.stringify(wanted)); };
		const F = window.AuraConfigFeedback;
		const el = document.getElementById('b1');
		expect('states', F.BANNER_STATES, ['off', 'unconfigured', 'checking', 'connected', 'degraded', 'error', 'unknown']);

		for (const state of F.BANNER_STATES) {
			window.cfgStatusBanner(el, state, { detail: 'detail-' + state });
			expect('state ' + state, [el.dataset.state, el.classList.contains('cfg-state-banner'), el.getAttribute('role'), el.querySelector('.cfg-state-detail').textContent, !!el.querySelector('.cfg-state-dot'), el.getAttribute('aria-busy')], [state, true, 'status', 'detail-' + state, true, state === 'checking' ? 'true' : 'false']);
		}
		window.cfgStatusBanner(el, 'error', { reason: 'refused', retry: () => {} });
		expect('error reason + retry', [el.querySelector('.cfg-state-detail').textContent, !!el.querySelector('.cfg-state-retry'), el.querySelector('.cfg-state-retry').textContent], ['refused', true, 'Check again']);
		window.cfgStatusBanner(el, 'connected', { retry: () => {} });
		expect('no retry when connected', !!el.querySelector('.cfg-state-retry'), false);
		window.cfgStatusBanner(el, 'unknown', { links: [{ href: 'https://example.com/docs', label: 'Docs' }, { href: 'javascript:alert(1)', label: 'bad' }] });
		expect('links', [...el.querySelectorAll('.cfg-state-link')].map(a => a.getAttribute('href')), ['https://example.com/docs']);
		window.cfgStatusBanner(el, 'error', { detail: '<img src=x onerror=alert(1)>' });
		expect('detail is text', [el.querySelector('img'), el.querySelector('.cfg-state-detail').textContent], [null, '<img src=x onerror=alert(1)>']);

		// html() starts from the explicit state and escapes.
		const host = document.getElementById('b2');
		host.innerHTML = window.cfgStatusBanner.html('b2-banner', 'unconfigured', { detail: 'a<b' });
		const banner = host.firstElementChild;
		expect('html', [banner.id, banner.dataset.state, banner.getAttribute('aria-live'), banner.querySelector('.cfg-state-detail').textContent], ['b2-banner', 'unconfigured', 'polite', 'a<b']);

		// check(): checking -> mapped state.
		const target = document.getElementById('b3');
		let release;
		const gate = new Promise(resolve => { release = resolve; });
		const mapped = window.cfgStatusBanner.check(target, async () => { await gate; return new Response(JSON.stringify({ status: 'ok', message: 'v1' })); }, (body) => ({ state: 'connected', detail: body.message }));
		expect('checking first', [target.dataset.state, target.getAttribute('aria-busy')], ['checking', 'true']);
		release();
		await mapped;
		expect('connected', [target.dataset.state, target.querySelector('.cfg-state-detail').textContent, target.getAttribute('aria-busy')], ['connected', 'v1', 'false']);

		// HTTP failure -> error(reason); network failure -> error(network text).
		await window.cfgStatusBanner.check(target, async () => new Response(JSON.stringify({ error: 'unauthorized' }), { status: 401 }), () => ({ state: 'connected' }));
		expect('http error', [target.dataset.state, target.querySelector('.cfg-state-detail').textContent, !!target.querySelector('.cfg-state-retry')], ['error', 'unauthorized', true]);
		await window.cfgStatusBanner.check(target, async () => { throw new TypeError('Failed to fetch'); }, () => ({ state: 'connected' }));
		expect('network error', [target.dataset.state, target.querySelector('.cfg-state-detail').textContent], ['error', 'Network error']);
		// A mapper may turn a 200 body into any state; a falsy mapping is unknown.
		await window.cfgStatusBanner.check(target, async () => new Response(JSON.stringify({ status: 'error', message: 'down' })), (body) => ({ state: 'error', detail: body.message }));
		expect('mapped error', [target.dataset.state, target.querySelector('.cfg-state-detail').textContent], ['error', 'down']);
		await window.cfgStatusBanner.check(target, async () => new Response('{}'), () => null);
		expect('falsy map is unknown', target.dataset.state, 'unknown');
		await window.cfgStatusBanner.check(target, async () => new Response(JSON.stringify({ status: 'partial', message: 'slow' })), (b, outcome) => outcome.state === 'warning' ? { state: 'degraded', detail: b.message } : null);
		expect('degraded', [target.dataset.state, !!target.querySelector('.cfg-state-retry')], ['degraded', true]);

		// Timeout -> unknown with retry; the retry runs the check again.
		let calls = 0;
		const fetcher = async signal => {
			calls++;
			if (calls === 1) return new Promise(() => {});
			return new Response(JSON.stringify({ status: 'ok' }));
		};
		const started = Date.now();
		await window.cfgStatusBanner.check(target, fetcher, () => ({ state: 'connected' }), { timeoutMs: 80 });
		expect('timeout', [target.dataset.state, target.getAttribute('aria-busy'), target.querySelector('.cfg-state-detail').textContent, Date.now() - started < 2000], ['unknown', 'false', 'No answer in time', true]);
		target.querySelector('.cfg-state-retry').click();
		await new Promise(resolve => setTimeout(resolve, 100));
		expect('retry', [calls, target.dataset.state], [2, 'connected']);

		// A newer check wins over a slower older one.
		let slowRelease;
		const slowGate = new Promise(resolve => { slowRelease = resolve; });
		const older = window.cfgStatusBanner.check(target, async () => { await slowGate; return new Response(JSON.stringify({ status: 'ok' })); }, () => ({ state: 'connected', detail: 'old' }));
		await window.cfgStatusBanner.check(target, async () => new Response(JSON.stringify({ status: 'ok' })), () => ({ state: 'degraded', detail: 'new' }));
		slowRelease();
		await older;
		expect('stale check ignored', target.querySelector('.cfg-state-detail').textContent, 'new');
		return JSON.stringify(failures);
	}`)
}

func TestConfigFeedbackDraftPatchAndSecretsBrowser(t *testing.T) {
	page := feedbackPage(t, `
		<input id="url" data-path="demo.url" value="https://saved.example">
		<input id="num" data-path="demo.port" type="number" value="25">
		<div class="toggle" id="on" data-path="demo.enabled"></div>
		<div class="password-wrap"><input id="key" type="password" data-path="demo.api_key" value=""></div>
		<div class="password-wrap"><input id="fresh" type="password" data-path="demo.token" value=""></div>
		<input id="other" data-path="other.url" value="x">`)
	evalFailures(t, page, `() => {
		const failures = [];
		const expect = (name, actual, wanted) => { if (JSON.stringify(actual) !== JSON.stringify(wanted)) failures.push(name + ': got ' + JSON.stringify(actual) + ', want ' + JSON.stringify(wanted)); };
		const mask = String.fromCharCode(8226).repeat(8);
		const S = window.AuraConfigState;
		S.init({ demo: { url: 'https://saved.example', port: 25, enabled: false, api_key: mask, token: '' }, other: { url: 'x' } });
		S.bind(document);
		const type = (id, value) => { const input = document.getElementById(id); input.value = value; input.dispatchEvent(new Event('input', { bubbles: true })); };

		// A stored secret reads as an empty field: not dirty, no '' in the patch.
		type('key', '');
		expect('empty field keeps the mask', [S.isDirty(), S.get('demo.api_key'), 'demo' in S.buildPatch()], [false, mask, false]);
		type('key', 'abc');
		expect('typed secret is a change', [S.isDirty(), S.buildPatch()], [true, { demo: { api_key: 'abc' } }]);
		type('key', '');
		expect('clearing the field reverts to the mask', [S.isDirty(), S.get('demo.api_key')], [false, mask]);
		// restoring the DOM never writes the mask into the field
		S.discard();
		expect('mask never shown', document.getElementById('key').value, '');
		// an unset secret that stays empty is not a change either
		type('fresh', '');
		expect('empty unset secret', S.isDirty(), false);

		// draftPatch: all data-path fields under the prefixes, same nested shape as the PUT patch.
		type('url', 'https://typed.example');
		type('num', '587');
		expect('patch', window.cfgDraftPatch(['demo']), { demo: { url: 'https://typed.example', port: 587, enabled: false } });
		type('fresh', 'new-secret');
		expect('typed secret in patch', window.cfgDraftPatch(['demo']).demo.token, 'new-secret');
		expect('stored secret left out', 'api_key' in window.cfgDraftPatch(['demo']).demo, false);
		expect('prefix only', Object.keys(window.cfgDraftPatch(['other'])), ['other']);
		expect('field path', window.cfgDraftPatch(['demo.url']), { demo: { url: 'https://typed.example' } });
		expect('no prefix, no patch', window.cfgDraftPatch([]), {});
		expect('API twin', window.AuraConfigFeedback.draftPatch(['other']), { other: { url: 'x' } });
		return JSON.stringify(failures);
	}`)
}

func TestConfigFeedbackDraftLockRuleBrowser(t *testing.T) {
	page := feedbackPage(t, `
		<input id="url" data-path="demo.url" value="https://saved.example">
		<div class="password-wrap"><input id="key" type="password" data-path="demo.api_key" value=""></div>
		<button id="draft-btn" type="button" data-config-action-draft="true">Draft</button>
		<button id="plain-btn" type="button">Plain</button>
		<button id="runtime-btn" type="button" data-config-action-draft="false">Runtime</button>
		<button id="draft-test-btn" type="button" data-config-action-draft="true" aria-describedby="help-line">Auto</button>
		<span id="help-line">help</span>`)
	evalFailures(t, page, `async () => {
		const failures = [];
		const expect = (name, actual, wanted) => { if (JSON.stringify(actual) !== JSON.stringify(wanted)) failures.push(name + ': got ' + JSON.stringify(actual) + ', want ' + JSON.stringify(wanted)); };
		const mask = String.fromCharCode(8226).repeat(8);
		const S = window.AuraConfigState;
		const A = window.AuraConfigActions;
		S.init({ demo: { url: 'https://saved.example', api_key: mask } });
		S.bind(document);
		const type = (id, value) => { const input = document.getElementById(id); input.value = value; input.dispatchEvent(new Event('input', { bubbles: true })); };
		const noop = async () => {};
		const reasonOf = id => document.getElementById(id + '-reason').textContent;
		const locked = id => document.getElementById(id).dataset.configActionLocked;
		const spec = { requiresSaved: true, requiredPaths: ['demo.url'], credentialPaths: ['demo.api_key'], run: noop };
		A.register('draft-btn', spec);
		A.register('plain-btn', spec);
		A.register('runtime-btn', { requiresSaved: true, run: noop });
		A.register('draft-test-btn', spec);

		expect('clean: all open', ['draft-btn', 'plain-btn', 'runtime-btn', 'draft-test-btn'].map(locked), ['false', 'false', 'false', 'false']);
		type('url', 'https://typed.example');
		expect('dirty: draft button stays open', [locked('draft-btn'), locked('draft-test-btn')], ['false', 'false']);
		expect('dirty: plain button locked as before', [locked('plain-btn'), reasonOf('plain-btn')], ['true', 'Save first']);
		expect('dirty: runtime-only button explains', [locked('runtime-btn'), reasonOf('runtime-btn')], ['true', 'Runtime only']);

		// required values and the credential are read from the draft; a stored mask counts as present
		type('url', '');
		expect('empty required field', [locked('draft-btn'), reasonOf('draft-btn')], ['true', 'Missing fields']);
		type('url', 'https://typed.example');
		expect('field back', locked('draft-btn'), 'false');
		S.init({ demo: { url: 'https://saved.example' } });
		expect('no credential saved or typed', [locked('draft-btn'), reasonOf('draft-btn')], ['true', 'Missing credential']);
		type('key', 'typed-key');
		expect('credential typed, not yet saved', locked('draft-btn'), 'false');
		type('key', '');
		expect('credential removed again', locked('draft-btn'), 'true');

		// the reason joins the existing description instead of replacing it
		const described = document.getElementById('draft-test-btn').getAttribute('aria-describedby').split(' ');
		expect('describedby merged', [described.includes('help-line'), described.includes('draft-test-btn-reason')], [true, true]);

		// a busy button ignores further clicks (also through the capture guard)
		const auto = document.getElementById('draft-test-btn');
		let clicks = 0;
		auto.addEventListener('click', () => { clicks++; });
		auto.dataset.configActionBusy = 'true';
		auto.click();
		expect('busy click swallowed', clicks, 0);
		auto.dataset.configActionBusy = 'false';
		return JSON.stringify(failures);
	}`)
}

func TestConfigFeedbackConfirmOnEnableBrowser(t *testing.T) {
	page := feedbackPage(t, `
		<div class="cfg-toggle-row"><span class="cfg-toggle-label" id="shell-label">Shell access</span>
			<div class="toggle" id="shell" data-path="agent.allow_shell" aria-labelledby="shell-label" onclick="this.classList.toggle('on')"></div></div>
		<div class="field-group"><div class="field-label">Delete files</div>
			<div class="toggle on" id="gh" data-path="github.allow_delete" onclick="this.classList.toggle('on')"></div></div>
		<div class="toggle" id="plain" data-path="demo.harmless" onclick="this.classList.toggle('on')"></div>
		<label><input type="checkbox" id="modal-destructive" data-cfg-confirm-enable="delete"> Allow deleting</label>
		<div class="toggle" id="locked" data-path="docker.allow_host_access" aria-disabled="true" onclick="this.classList.toggle('on')"></div>`)
	evalFailures(t, page, `async () => {
		const failures = [];
		const expect = (name, actual, wanted) => { if (JSON.stringify(actual) !== JSON.stringify(wanted)) failures.push(name + ': got ' + JSON.stringify(actual) + ', want ' + JSON.stringify(wanted)); };
		const settle = () => new Promise(resolve => setTimeout(resolve, 40));
		const shell = document.getElementById('shell');

		window.__answer = false;
		shell.click();
		await settle();
		expect('cancel keeps it off', [shell.classList.contains('on'), window.__confirms.length], [false, 1]);
		expect('dialog text', window.__confirms[0], ['Enable Shell access?', 'Host rights for Shell access']);

		window.__answer = true;
		shell.click();
		await settle();
		expect('OK turns it on', [shell.classList.contains('on'), window.__confirms.length], [true, 2]);

		window.__answer = false;
		shell.click();
		await settle();
		expect('turning off never asks', [shell.classList.contains('on'), window.__confirms.length], [false, 2]);

		// registry entry that is already on: turning it off does not ask either
		const gh = document.getElementById('gh');
		gh.click();
		await settle();
		expect('on -> off without question', [gh.classList.contains('on'), window.__confirms.length], [false, 2]);
		window.__answer = true;
		gh.click();
		await settle();
		expect('off -> on asks', [gh.classList.contains('on'), window.__confirms.length, window.__confirms[2]?.[0]], [true, 3, 'Enable Delete files?']);

		// unregistered and locked switches are not touched
		document.getElementById('plain').click();
		document.getElementById('locked').click();
		await settle();
		expect('plain and locked pass through', [document.getElementById('plain').classList.contains('on'), document.getElementById('locked').classList.contains('on'), window.__confirms.length], [true, true, 3]);

		// modal checkbox opt-in
		const box = document.getElementById('modal-destructive');
		window.__answer = false;
		box.click();
		await settle();
		expect('checkbox cancel', [box.checked, window.__confirms.length, window.__confirms[3]?.[1]], [false, 4, 'Delete rights']);
		window.__answer = true;
		box.click();
		await settle();
		expect('checkbox OK', [box.checked, window.__confirms.length], [true, 5]);
		box.click();
		await settle();
		expect('checkbox off never asks', [box.checked, window.__confirms.length], [false, 5]);

		// no re-entrancy: a pending question swallows a second click
		let release;
		window.showConfirm = () => new Promise(resolve => { release = resolve; });
		shell.click();
		shell.click();
		expect('swallowed while asking', shell.classList.contains('on'), false);
		release(true);
		await settle();
		expect('answered once', shell.classList.contains('on'), true);
		return JSON.stringify(failures);
	}`)
}
