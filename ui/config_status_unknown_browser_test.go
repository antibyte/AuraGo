package ui

import (
	"strings"
	"testing"
)

const configStatusFailureFetch = `(() => {
	const nativeFetch = window.fetch.bind(window);
	window.__auraFailStatus = true;
	window.fetch = (input, init) => {
		const url = new URL(String((input && input.url) || input), location.href);
		if (window.__auraFailStatus && (url.pathname === '/api/runtime' || url.pathname === '/api/vault/status')) {
			return Promise.resolve(new Response('<html>bad gateway</html>', { status: 502, headers: { 'Content-Type': 'text/html' } }));
		}
		return nativeFetch(input, init);
	};
})();`

func TestConfigRuntimeStatusUnknownLocksSectionsBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	origin := configRefreshFixtureOrigin(t, "en", false)
	page := newSmokeBrowser(t).MustPage()
	defer page.MustClose()
	page.MustEvalOnNewDocument(configStatusFailureFetch)
	page.MustNavigate(origin + "/config#overview").MustWaitLoad()
	waitForJSBool(t, page, `() => !!document.querySelector('.pw-overview-card')`)

	masterKey := page.MustEval(`() => renderField('server.master_key', 'master_key', '', 'server')`).String()
	for _, want := range []string{`cfg-master-key-locked-input" type="password" value="" disabled`, `data-config-status-retry="vault"`} {
		if !strings.Contains(masterKey, want) {
			t.Fatalf("unknown vault status must lock the master key field; missing %q in %s", want, masterKey)
		}
	}
	if strings.Contains(masterKey, `data-path="server.master_key"`) {
		t.Fatal("unknown vault status must not offer an editable master key input")
	}

	page.MustEval(`async () => { await selectSection('firewall', { scrollBehavior: 'auto' }); }`)
	waitForJSBool(t, page, `() => !!document.querySelector('#content .feature-unavailable-fields') && !!document.querySelector('#content [data-config-status-retry="runtime"]')`)
	page.MustEval(`async () => { await selectSection('updates', { scrollBehavior: 'auto' }); }`)
	waitForJSBool(t, page, `() => !document.getElementById('updates-install-btn') && !!document.querySelector('#updates-body [data-config-status-retry="runtime"]')`)

	page.MustEval(`async () => {
		await selectSection('firewall', { scrollBehavior: 'auto' });
		window.__auraFailStatus = false;
		document.querySelector('#content [data-config-status-retry="runtime"]').click();
	}`)
	waitForJSBool(t, page, `() => !document.querySelector('#content .feature-unavailable-fields') && !document.querySelector('#content [data-config-status-retry]')`)
}

func TestSecretsSectionShowsUnknownVaultStatusBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	page := newSmokeBrowser(t).MustPage("about:blank")
	defer page.MustClose()
	page.MustSetDocumentContent(`<div id="secrets-host"></div>`)
	page.MustEval(`() => {
		window.I18N = {};
		window.t = key => key;
		window.__vaultStatus = 502;
		window.fetch = async input => {
			const url = String(input);
			if (url.includes('/api/vault/status')) {
				if (window.__vaultStatus !== 200) return new Response('<html>bad gateway</html>', { status: window.__vaultStatus, headers: { 'Content-Type': 'text/html' } });
				return new Response(JSON.stringify({ exists: true }), { status: 200, headers: { 'Content-Type': 'application/json' } });
			}
			if (url.includes('/api/vault/secrets')) return new Response('[]', { status: 200, headers: { 'Content-Type': 'application/json' } });
			return new Response('{}', { status: 404, headers: { 'Content-Type': 'application/json' } });
		};
	}`)
	if err := page.AddScriptTag("", normalizeAssetText(mustReadUIFile(t, "cfg/secrets.js"))); err != nil {
		t.Fatalf("load secrets module: %v", err)
	}
	page.MustEval(`() => renderSecretsSection({ key: 'secrets', label: 'Secrets', container: 'secrets-host' })`)
	waitForJSBool(t, page, `() => !!document.querySelector('[data-secrets-vault-retry]')`)
	if text := page.MustEval(`() => document.getElementById('secrets-host').textContent`).String(); strings.Contains(text, "config.secrets.no_vault") {
		t.Fatalf("an unreadable vault status must not claim that no vault exists: %s", text)
	}
	page.MustEval(`() => { window.__vaultStatus = 200; document.querySelector('[data-secrets-vault-retry]').click(); }`)
	waitForJSBool(t, page, `() => document.getElementById('secrets-host').textContent.includes('config.secrets.empty')`)
}
