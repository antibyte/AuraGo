package ui

import (
	"testing"
)

// Inline validation (E14) and the collapsed expert fields (E13) on the generic renderer, against the
// fixture built from config_template.yaml.
func TestConfigInlineValidationBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	browser := newSmokeBrowser(t)
	// The template has no oauth_redirect_base_url; the real schema does, so the fixture adds it.
	origin := configRefreshFixtureOriginWith(t, "de", false, func(configuration map[string]any) {
		configuration["server"].(map[string]any)["oauth_redirect_base_url"] = ""
	})
	page := browser.MustPage(origin + "/config#overview")
	defer page.MustClose()
	waitForJSBool(t, page, `()=>!!document.querySelector('.pw-overview-card')`)

	// edit types a value into a control and leaves it, as a user does.
	edit := func(path, value string) {
		t.Helper()
		page.MustEval(`(path, value) => {
			const el = document.querySelector('[data-path="' + path + '"]');
			el.value = value;
			el.dispatchEvent(new Event('input', { bubbles: true }));
			el.dispatchEvent(new Event('change', { bubbles: true }));
			el.dispatchEvent(new Event('focusout', { bubbles: true }));
		}`, path, value)
	}
	// state describes the inline error of a control: "invalid|<alert text>|<describedby includes the alert and the help>".
	state := func(path string) string {
		t.Helper()
		return page.MustEval(`path => {
			const el = document.querySelector('[data-path="' + path + '"]');
			const ids = (el.getAttribute('aria-describedby') || '').split(/\s+/).filter(Boolean);
			const alert = ids.map(id => document.getElementById(id)).find(node => node && node.getAttribute('role') === 'alert');
			const help = ids.map(id => document.getElementById(id)).some(node => node && node.classList.contains('field-help'));
			return (el.getAttribute('aria-invalid') === 'true' ? 'invalid' : 'valid') + '|' + (alert ? alert.textContent : '') + '|' + (help ? 'help' : 'nohelp');
		}`, path).String()
	}

	page.MustEval(`async()=>{await selectSection('server',{scrollBehavior:'auto'});}`)
	if got := page.MustEval(`()=>document.querySelector('[data-path="server.https.https_port"]').max`).String(); got != "65535" {
		t.Fatalf("number inputs must carry max from the rule, got %q", got)
	}

	edit("server.https.https_port", "70000")
	if got := state("server.https.https_port"); got != "invalid|Gib einen Wert von höchstens 65535 ein.|nohelp" && got != "invalid|Gib einen Wert von höchstens 65535 ein.|help" {
		t.Fatalf("port 70000 must be rejected inline with the range, got %q", got)
	}
	if page.MustEval(`()=>AuraConfigState.validate().errors.some(e=>e.path==='server.https.https_port' && e.code==='max')`).Bool() != true {
		t.Fatal("save must stay blocked while the port is invalid")
	}
	edit("server.https.https_port", "8443")
	if got := state("server.https.https_port"); got != "valid||nohelp" && got != "valid||help" {
		t.Fatalf("a valid port must clear the error, got %q", got)
	}
	if page.MustEval(`()=>document.querySelectorAll('.pw-field-error[data-config-validation]').length`).Int() != 0 {
		t.Fatal("a cleared error must leave no message element behind")
	}
	// 0 means "off" for the redirect port and has to stay valid.
	edit("server.https.http_port", "0")
	if got := state("server.https.http_port"); got[:5] != "valid" {
		t.Fatalf("0 must be accepted for ports whose default is off, got %q", got)
	}
	edit("server.https.http_port", "-3")
	if got := state("server.https.http_port"); got[:7] != "invalid" {
		t.Fatalf("a negative port must be rejected, got %q", got)
	}
	edit("server.https.http_port", "0")

	// URL without scheme.
	edit("server.oauth_redirect_base_url", "example.com")
	if got := state("server.oauth_redirect_base_url"); got[:7] != "invalid" {
		t.Fatalf("a URL without scheme must be rejected, got %q", got)
	}
	edit("server.oauth_redirect_base_url", "https://example.com")
	if got := state("server.oauth_redirect_base_url"); got[:5] != "valid" {
		t.Fatalf("a complete URL must clear the error, got %q", got)
	}
	edit("server.oauth_redirect_base_url", "")
	if got := state("server.oauth_redirect_base_url"); got[:5] != "valid" {
		t.Fatalf("an empty optional URL is valid, got %q", got)
	}

	// Negative timeout with its unit suffix.
	page.MustEval(`async()=>{await selectSection('tools',{scrollBehavior:'auto'});}`)
	if got := page.MustEval(`()=>{const el=document.querySelector('[data-path="tools.python_timeout_seconds"]');return el.min+'|'+(el.parentElement.classList.contains('cfg-input-unit')?el.parentElement.querySelector('.cfg-unit').textContent:'nounit')}`).String(); got != "0|s" {
		t.Fatalf("timeout input must carry min=0 and the seconds unit, got %q", got)
	}
	edit("tools.python_timeout_seconds", "-5")
	if got := state("tools.python_timeout_seconds"); got[:7] != "invalid" {
		t.Fatalf("a negative timeout must be rejected, got %q", got)
	}
	if !page.MustEval(`()=>{const el=document.querySelector('[data-path="tools.python_timeout_seconds"]');const alert=document.getElementById(el.getAttribute('aria-describedby').split(' ').find(id=>id.endsWith('-validation')));return !!alert && alert.previousElementSibling===el.parentElement}`).Bool() {
		t.Fatal("the alert belongs directly below the unit wrapper")
	}
	edit("tools.python_timeout_seconds", "0")
	if got := state("tools.python_timeout_seconds"); got[:5] != "valid" {
		t.Fatalf("0 must be accepted for a timeout, got %q", got)
	}
	edit("tools.python_timeout_seconds", "30")

	// Expert fields of the tool permissions are collapsed behind "Erweitert".
	page.MustEval(`async()=>{AuraConfigState.discard && AuraConfigState.discard(); await selectSection('tools',{scrollBehavior:'auto'});}`)
	got := page.MustEval(`()=>{
		const details = [...document.querySelectorAll('#content details.pw-advanced[data-config-disclosure]')];
		const inside = path => details.some(d => d.querySelector('[data-path="' + path + '"]'));
		const switchesOutside = [...document.querySelectorAll('#content .toggle[data-path$=".enabled"], #content .toggle[data-path$=".readonly"]')].every(el => !el.closest('details.pw-advanced'));
		return JSON.stringify({
			summary: details.length ? details[0].querySelector('summary strong').textContent : '',
			closed: details.every(d => !d.open),
			timeouts: inside('tools.python_timeout_seconds') && inside('tools.skill_timeout_seconds') && inside('tools.background_timeout_seconds'),
			tuning: inside('tools.knowledge_graph.max_prompt_nodes') && inside('tools.memory.ondemand_retrieval.max_available_chars'),
			switchesOutside,
			count: details.reduce((n, d) => n + d.querySelectorAll('[data-path]').length, 0)
		});
	}`).String()
	if got != `{"summary":"Erweitert","closed":true,"timeouts":true,"tuning":true,"switchesOutside":true,"count":17}` {
		t.Fatalf("tool permissions expert fields = %s", got)
	}

	// The sidebar field search must reach a collapsed field and open its disclosure.
	label := page.MustEval(`()=>document.querySelector('[data-path="tools.python_timeout_seconds"]').closest('.field-group').querySelector('.field-label').textContent.trim()`).String()
	page.MustEval(`async()=>{await selectSection('overview',{scrollBehavior:'auto'});}`)
	page.MustEval(`label => { const input = document.getElementById('sidebarSearchInput'); input.value = label; input.dispatchEvent(new Event('input', { bubbles: true })); }`, label)
	waitForJSBool(t, page, `()=>!!document.querySelector('.sidebar-item[data-search-target="tools.python_timeout_seconds"]:not(.hidden):not(.cfg-filter-hidden)')`)
	page.MustEval(`()=>document.querySelector('.sidebar-item[data-search-target="tools.python_timeout_seconds"]').click()`)
	waitForJSBool(t, page, `()=>{const el=document.querySelector('[data-path="tools.python_timeout_seconds"]');return !!el && !!el.closest('details.pw-advanced') && el.closest('details.pw-advanced').open && el.getClientRects().length>0}`)
}
