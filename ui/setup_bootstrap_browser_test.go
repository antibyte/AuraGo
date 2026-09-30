package ui

import (
	"regexp"
	"strings"
	"testing"

	"github.com/go-rod/rod"
)

// openSetupWithStatus loads setup.html with main.js against a fetch stub. The
// stub answers /api/setup/status via statusJS(headers) and records every call.
func openSetupWithStatus(t *testing.T, path, statusJS string) *rod.Page {
	t.Helper()
	requirePrecisionBrowserSmoke(t)
	browser := newSmokeBrowser(t)
	page := browser.MustPage(newPrecisionSmokeOrigin(t) + path)
	t.Cleanup(func() { page.MustClose() })
	page.MustWaitLoad()
	html := normalizeAssetText(mustReadUIFile(t, "setup.html"))
	html = strings.NewReplacer("{{.Lang}}", "en", "{{.BuildVersion}}", "test", "{{.TemplateDataJSON}}", "{}").Replace(html)
	html = regexp.MustCompile(`(?is)<script\s+src="[^"]+"[^>]*></script>`).ReplaceAllString(html, "")
	page.MustSetDocumentContent(html)
	page.MustEval(`() => {
        window.t = key => key;
        window.__setupRequests = [];
        window.__setupStatus = ` + statusJS + `;
        window.fetch = async (input, init) => {
            const url = String(input);
            const headers = Object.assign({}, (init && init.headers) || {});
            window.__setupRequests.push({url, headers});
            let payload = {profiles:[],personalities:[],auth:{password_set:false}};
            if (url.includes('/api/setup/status')) payload = window.__setupStatus(headers);
            if (url === '/api/setup') payload = {status:'saved', needs_restart:false};
            return {ok:true,status:200,json:async()=>payload,text:async()=>JSON.stringify(payload)};
        };
    }`)
	if err := page.AddScriptTag("", normalizeAssetText(mustReadUIFile(t, "js/setup/main.js"))); err != nil {
		t.Fatal(err)
	}
	return page
}

func TestSetupBootstrapTokenGateBrowser(t *testing.T) {
	page := openSetupWithStatus(t, "/setup#bootstrap=stale-token", `headers => ({
        needs_setup: true,
        csrf_token: 'fixture',
        bootstrap_token_required: true,
        bootstrap_token_valid: headers['X-Setup-Token'] === 'owner-token',
    })`)

	waitForJSBool(t, page, `() => document.getElementById('setup-bootstrap-gate')?.classList.contains('open') === true`)
	page.MustEval(`() => {
        if (location.hash !== '') throw Error('bootstrap token left in the address bar: ' + location.hash);
        const first = window.__setupRequests.find(r => r.url.includes('/api/setup/status'));
        if (!first || first.headers['X-Setup-Token'] !== 'stale-token') throw Error('fragment token was not sent to the status check');
        if (document.getElementById('setup-bootstrap-error').classList.contains('is-hidden')) throw Error('rejected token shows no error');
        const input = document.getElementById('setup-bootstrap-input');
        input.value = '  owner-token  ';
        document.getElementById('setup-bootstrap-form').dispatchEvent(new Event('submit', {cancelable: true, bubbles: true}));
    }`)
	waitForJSBool(t, page, `() => !document.getElementById('setup-bootstrap-gate').classList.contains('open')`)
	page.MustEval(`async () => {
        await saveConfig();
        const save = window.__setupRequests.find(r => r.url === '/api/setup');
        if (!save) throw Error('setup save was not sent');
        if (save.headers['X-Setup-Token'] !== 'owner-token') throw Error('setup save lacks the bootstrap token header');
        if (save.headers['X-CSRF-Token'] !== 'fixture') throw Error('setup save lost the CSRF header');
        const probe = setupRequestHeaders({'Content-Type': 'application/json'});
        if (probe['X-Setup-Token'] !== 'owner-token' || probe['Content-Type'] !== 'application/json') throw Error('shared setup headers incomplete');
    }`)
}

func TestSetupVaultLockedBrowser(t *testing.T) {
	page := openSetupWithStatus(t, "/setup", `() => ({needs_setup: true, vault_locked: true})`)

	waitForJSBool(t, page, `() => document.getElementById('setup-bootstrap-gate')?.classList.contains('open') === true`)
	page.MustEval(`() => {
        if (document.getElementById('setup-vault-locked').classList.contains('is-hidden')) throw Error('vault lock message hidden');
        if (!document.getElementById('setup-bootstrap-token-view').classList.contains('is-hidden')) throw Error('token form offered although setup is locked');
    }`)
}
