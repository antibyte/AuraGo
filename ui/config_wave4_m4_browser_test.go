package ui

import (
	"strings"
	"testing"
)

// Wave 4 package M4 (integrations, system and security sections): every module renders without script errors,
// has no emoji or loose checkbox, and the dialogs, switches and inline messages it gained behave.
var configWave4M4Sections = []string{
	"email", "agentmail", "github", "google_workspace", "gog", "paperless_ngx", "obsidian", "yepapi", "s3", "webdav", "koofr",
	"onedrive", "netlify", "vercel", "here_now", "cloudflare_tunnel", "homepage", "server", "web_config", "backup_restore", "updates",
	"indexing", "firewall", "danger_zone", "sandbox", "sql_connections",
}

func TestConfigWave4M4ModulesBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	browser := newSmokeBrowser(t)
	page := browser.MustPage(configRefreshFixtureOriginWith(t, "en", true, nil) + "/config#overview")
	defer page.MustClose()
	waitForJSBool(t, page, `() => !!document.querySelector('.pw-overview-card') && typeof cfgModal === 'function'`)
	page.MustSetViewport(1440, 900, 1, false)
	page.MustEval(`() => {
		window.__m4Errors = [];
		window.addEventListener('error', event => window.__m4Errors.push((event.filename || '') + ' ' + event.message));
		window.addEventListener('unhandledrejection', event => window.__m4Errors.push(String(event.reason && (event.reason.stack || event.reason))));
	}`)

	known := map[string]bool{}
	for _, key := range page.MustEval(`() => SECTIONS.flatMap(group => group.items.map(item => item.key))`).Arr() {
		known[key.Str()] = true
	}
	for _, key := range configWave4M4Sections {
		if !known[key] {
			t.Errorf("section %q is not part of the config sidebar", key)
			continue
		}
		page.MustEval(`async key => { await selectSection(key, {scrollBehavior:'auto', history:'replace'}); await new Promise(resolve => setTimeout(resolve, 400)); }`, key)
		problems := page.MustEval(`key => {
			const root = document.getElementById('content');
			const out = [];
			const emoji = /[\u{1F000}-\u{1FFFF}\u{2600}-\u{27BF}\u{2B00}-\u{2BFF}\u{2300}-\u{23FF}]/u;
			// The danger zone mounts the shared action-policy widget (js/shared/action-policy.js), which is not part of M4.
			if (key !== 'danger_zone') {
				if (emoji.test(root.textContent)) out.push('emoji in text');
				if (root.querySelector('input[type="checkbox"]:not([data-cfg-choice])')) out.push('loose checkbox');
			}
			if (root.querySelector('[class*="-badge"]:not(.pw-badge):not(.hint-badge)')) out.push('own badge class');
			return out.join(', ');
		}`, key).String()
		if problems != "" {
			t.Errorf("section %s: %s", key, problems)
		}
	}

	check := func(name, expr string) {
		t.Helper()
		if !page.MustEval(expr).Bool() {
			t.Errorf("%s", name)
		}
	}

	// E15 + E8: the e-mail dialog tells what happens to a typed ID, uses switches and the shared dialog.
	page.MustEval(`async () => { await selectSection('email', {scrollBehavior:'auto', history:'replace'}); await new Promise(resolve => setTimeout(resolve, 300)); emailAccountShowModal('New', {}, async () => ({ok: true})); }`)
	check("email dialog is the shared dialog with switches", `() => {
		const dialog = document.getElementById('email-account-dialog');
		return !!dialog && dialog.classList.contains('modal-overlay') && dialog.classList.contains('open')
			&& dialog.getAttribute('role') === 'dialog' && !!dialog.querySelector('.modal-close svg')
			&& dialog.querySelectorAll('.toggle[role="switch"]').length === 3 && !dialog.querySelector('input[type="checkbox"]');
	}`)
	check("email ID change is announced instead of dropped silently", `() => {
		const input = document.getElementById('ea-id');
		input.value = 'Work Mail!';
		emailIdHint(input);
		const message = document.getElementById('ea-id-validation');
		return !message.hidden && message.getAttribute('role') === 'alert' && message.textContent.includes('workmail') && input.getAttribute('aria-invalid') === 'true';
	}`)
	check("email ID without a problem shows no message", `() => {
		const input = document.getElementById('ea-id');
		input.value = 'work-mail';
		emailIdHint(input);
		return document.getElementById('ea-id-validation').hidden && !input.hasAttribute('aria-invalid');
	}`)
	check("email dialog closes through closeModal", `() => { closeModal('email-account-dialog'); return !document.getElementById('email-account-dialog'); }`)

	// E8 + E4: SQL dialog, permission switches (the delete permission asks before it turns on).
	page.MustEval(`async () => { await selectSection('sql_connections', {scrollBehavior:'auto', history:'replace'}); await new Promise(resolve => setTimeout(resolve, 300)); sqlConnShowModal(); }`)
	check("sql dialog opens with permission switches", `() => {
		const dialog = document.getElementById('sqlconn-dialog');
		const on = id => document.getElementById(id).classList.contains('on');
		return !!dialog && dialog.classList.contains('open') && on('sqlconn-perm-read') && !on('sqlconn-perm-write') && !!dialog.querySelector('#sqlconn-perm-delete[data-cfg-confirm-enable]');
	}`)
	check("sql write permission flips without a prompt", `() => { document.getElementById('sqlconn-perm-write').click(); return document.getElementById('sqlconn-perm-write').classList.contains('on') && sqlConnPayload().allow_write === true; }`)
	check("sql delete permission asks first", `async () => {
		document.getElementById('sqlconn-perm-delete').click();
		await new Promise(resolve => setTimeout(resolve, 300));
		const overlay = document.getElementById('shared-modal-overlay');
		const asked = !!overlay && (overlay.classList.contains('open') || overlay.classList.contains('active'));
		const stillOff = !document.getElementById('sqlconn-perm-delete').classList.contains('on');
		const cancel = document.getElementById('shared-modal-cancel');
		if (cancel) cancel.click();
		return asked && stillOff;
	}`)
	page.MustEval(`() => { sqlConnCloseModal(); }`)

	// E8 + E7: secrets dialog.
	// secrets.js belongs to the knowledge page; load it the way that page does.
	page.MustEval(`async () => {
		await new Promise(resolve => { const script = document.createElement('script'); script.src = '/cfg/secrets.js'; script.onload = resolve; script.onerror = resolve; document.head.appendChild(script); });
		secretsShowAddModal();
	}`)
	check("secrets dialog is the shared dialog", `() => {
		const dialog = document.getElementById('secrets-dialog');
		const opened = !!dialog && dialog.classList.contains('open') && !!dialog.querySelector('#secret-key');
		closeModal('secrets-dialog');
		return opened && !document.getElementById('secrets-dialog');
	}`)

	// E4: backup options are switches.
	page.MustEval(`async () => { await selectSection('backup_restore', {scrollBehavior:'auto', history:'replace'}); await new Promise(resolve => setTimeout(resolve, 300)); }`)
	check("backup options are switches that feed the request", `() => {
		const toggle = document.getElementById('backup-opt-workdir');
		toggle.click();
		return toggle.classList.contains('on') && toggle.getAttribute('aria-checked') === 'true' && !isDirty;
	}`)

	// E15: indexing marks an empty path, gog reports the account limit.
	page.MustEval(`async () => { await selectSection('indexing', {scrollBehavior:'auto', history:'replace'}); await new Promise(resolve => setTimeout(resolve, 400)); document.getElementById('idx-new-dir').value = '  '; await idxAddDir(); }`)
	check("indexing empty path is reported inline", `() => {
		const input = document.getElementById('idx-new-dir');
		const line = document.getElementById('idx-dir-msg');
		const marked = input.getAttribute('aria-invalid') === 'true' && line.textContent.trim() !== '';
		input.dispatchEvent(new Event('input', {bubbles: true}));
		return marked && !input.hasAttribute('aria-invalid');
	}`)
	page.MustEval(`async () => {
		await selectSection('gog', {scrollBehavior:'auto', history:'replace'});
		await new Promise(resolve => setTimeout(resolve, 400));
		const add = document.getElementById('gog-add');
		for (let i = 0; i < 21; i++) add.click();
	}`)
	check("gog account limit is reported inline", `() => {
		const message = document.getElementById('gog-message').textContent;
		return message.includes('20') && document.querySelectorAll('#gog-accounts > .field-group').length === 20
			&& !document.querySelector('#gog-accounts input[type="checkbox"]:not([data-cfg-choice])');
	}`)

	// U19: HTTPS is the first switch of the server section; the cloudflare enable switch leads its section.
	page.MustEval(`async () => { await selectSection('server', {scrollBehavior:'auto', history:'replace'}); await new Promise(resolve => setTimeout(resolve, 400)); }`)
	check("server puts HTTPS before the preview switch", `() => {
		const https = document.querySelector('[data-path="server.https.enabled"]');
		const preview = document.querySelector('[data-path="server.preview_enabled"]');
		return !!https && !!preview && !!(https.compareDocumentPosition(preview) & Node.DOCUMENT_POSITION_FOLLOWING);
	}`)

	if errors := page.MustEval(`() => window.__m4Errors.filter(line => /\/cfg\/(email|agentmail|github|google_workspace|gog|paperless|obsidian|yepapi|s3|webdav|koofr|onedrive|netlify|vercel|here_now|cloudflare_tunnel|homepage|server|auth|auth_webauthn|backup|updates|indexing|firewall|secrets|danger|sandbox|sql_connections)\.js/.test(line)).join('\n')`).String(); strings.TrimSpace(errors) != "" {
		t.Errorf("script errors in M4 modules:\n%s", errors)
	}
}
