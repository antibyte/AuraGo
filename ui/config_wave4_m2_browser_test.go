package ui

// Wave 4 package M2: the dialogs, validation messages and migrated sections of mcp, composio, webhooks,
// skill_manager, security_proxy, huggingface, daemon_skills and memory_analysis.

import "testing"

func TestConfigWave4M2DialogsBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	browser := newSmokeBrowser(t)
	page := browser.MustPage(configRefreshFixtureOriginWith(t, "en", true, nil) + "/config#overview")
	defer page.MustClose()
	waitForJSBool(t, page, `() => !!document.querySelector('.pw-overview-card') && typeof cfgModal === 'function'`)
	page.MustSetViewport(1440, 900, 1, false)

	run := func(name, expr string) {
		t.Helper()
		got := page.MustEval(expr).String()
		t.Logf("%s => %s", name, got)
		if got != "ok" {
			t.Errorf("%s failed: %s", name, got)
		}
	}
	run("mcp dialog", `async () => {
		await selectSection('mcp', {scrollBehavior:'auto'});
		mcpServersCache = []; mcpSecretsCache = []; mcpPreferencesCache = {};
		configData.mcp = configData.mcp || {};
		mcpServerShowModal({name:'', transport:'stdio', url:'', headers:{}, command:'npx', args:[], env:{}, enabled:true, runtime:'local', docker_image:'', docker_command:'', allow_local_fallback:false, allow_private_network:false, host_workdir:'', container_workdir:'/workspace'}, -1);
		const m = document.getElementById('mcp-server-dialog');
		if (!m || !m.classList.contains('open')) return 'not open';
		document.getElementById('mcp-m-name').value = 'srv';
		document.getElementById('mcp-m-env').value = 'GOOD=1\nbad line\n\nALSO BAD';
		document.getElementById('mcp-m-save').click();
		await new Promise(r => setTimeout(r, 100));
		const err = document.getElementById('mcp-m-error');
		if (err.classList.contains('is-hidden') || !/2, 4/.test(err.textContent)) return 'no kv error: ' + err.textContent;
		if (document.getElementById('mcp-m-env').getAttribute('aria-invalid') !== 'true') return 'no aria-invalid';
		const sw = document.getElementById('mcp-m-enabled');
		sw.click();
		if (sw.classList.contains('on') || sw.getAttribute('aria-checked') !== 'false') return 'toggle broken';
		document.getElementById('mcp-m-transport').value = 'sse'; document.getElementById('mcp-m-transport').dispatchEvent(new Event('change'));
		if (!document.getElementById('mcp-m-command').closest('.field-group').classList.contains('is-hidden')) return 'stdio field not hidden';
		const d = document.getElementById('mcp-m-allow-destructive');
		d.click();
		await new Promise(r => setTimeout(r, 300));
		const confirmBtn = document.getElementById('shared-modal-confirm');
		if (!confirmBtn) return 'no confirm for destructive';
		confirmBtn.click();
		await new Promise(r => setTimeout(r, 300));
		if (!d.classList.contains('on')) return 'destructive not on after confirm';
		closeModal('mcp-server-dialog');
		if (document.getElementById('mcp-server-dialog')) return 'dialog not removed';
		mcpSecretShowModal({alias:'', label:'', description:'', has_value:false}, '');
		if (!document.getElementById('mcp-secret-dialog').classList.contains('open')) return 'secret not open';
		closeModal('mcp-secret-dialog');
		return 'ok';
	}`)
	run("composio picker", `async () => {
		await selectSection('composio', {scrollBehavior:'auto'});
		window.fetch = async () => new Response('{"items":[]}', {status: 200});
		await composioOpenModal();
		const m = document.getElementById('composio-picker-dialog');
		if (!m || !m.classList.contains('open')) return 'not open';
		if (!document.getElementById('composio-picker-count')) return 'no count';
		composioCloseModal();
		return m.classList.contains('open') ? 'still open' : 'ok';
	}`)
	run("token prompt", `async () => {
		await selectSection('webhooks', {scrollBehavior:'auto'});
		const p = _whPromptTokenName();
		const input = document.getElementById('wh-token-name-input');
		input.value = 'tok';
		document.getElementById('wh-token-ok').click();
		const v = await p;
		return v === 'tok' && !document.getElementById('wh-token-name-dialog') ? 'ok' : 'got ' + v;
	}`)
	run("token prompt dismiss", `async () => {
		const p = _whPromptTokenName();
		closeModal('wh-token-name-dialog', 'esc');
		const v = await p;
		return v === '' ? 'ok' : 'got ' + v;
	}`)
	run("sql dialog", `async () => {
		await selectSection('skill_manager', {scrollBehavior:'auto'});
		window.fetch = async () => new Response('[]', {status: 200});
		await ptbOpenSQLConnectionsModal();
		const m = document.getElementById('ptb-sql-dialog');
		if (!m || !m.classList.contains('open')) return 'not open';
		const empty = m.querySelector('.pw-empty-state');
		closeModal('ptb-sql-dialog');
		return empty ? 'ok' : 'no empty state';
	}`)
	run("og header validation", `async () => {
		whLoadState.outgoing = null; ogWebhooks = [];
		window.fetch = async () => new Response('[]', {status: 200});
		const host = document.createElement('div'); host.id = 'wh-outgoing-list'; document.body.append(host);
		ogShowModal(-1);
		document.getElementById('og-f-name').value = 'x'; document.getElementById('og-f-url').value = 'https://example.invalid/';
		document.getElementById('og-f-headers').value = 'X-A: 1\nnocolon';
		await ogSave();
		const field = document.getElementById('og-f-headers');
		const box = document.querySelector('#og-dialog-error-host .wh-inline-error');
		const okk = field.getAttribute('aria-invalid') === 'true' && box && /2/.test(box.textContent);
		ogCloseModal();
		host.remove();
		return okk ? 'ok' : 'no inline error';
	}`)
	run("security proxy + daemon + huggingface render", `async () => {
		configData.security_proxy = {enabled: true};
		await selectSection('security_proxy', {scrollBehavior:'auto'});
		await new Promise(r => setTimeout(r, 300));
		if (!document.getElementById('proxy-destroy-btn') || !document.getElementById('proxy-action-result')) return 'proxy buttons missing';
		await selectSection('huggingface', {scrollBehavior:'auto'});
		await new Promise(r => setTimeout(r, 300));
		if (!document.getElementById('huggingface-token') || !document.getElementById('huggingface-test-btn') || !document.querySelector('[data-path="huggingface.allowed_repos"][data-type="array"]')) return 'hf controls missing';
		await selectSection('daemon_skills', {scrollBehavior:'auto'});
		await new Promise(r => setTimeout(r, 300));
		if (!document.getElementById('daemon-status-grid') || !document.querySelector('[data-path="tools.daemon_skills.enabled"]')) return 'daemon controls missing';
		await selectSection('memory_analysis', {scrollBehavior:'auto'});
		await new Promise(r => setTimeout(r, 300));
		if (!document.querySelector('.toggle[data-path="learnings.enabled"]') || document.querySelector('#content input[type=checkbox]')) return 'memory toggles missing';
		return 'ok';
	}`)
}
