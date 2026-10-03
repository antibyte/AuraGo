package ui

import (
	"strings"
	"testing"
)

// Fails personality/provider list loads and gives llm.provider a saved value.
const configChoiceListFailureFetch = `(() => {
	const nativeFetch = window.fetch.bind(window);
	window.__auraFailChoiceLists = true;
	const json = (body, status) => new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } });
	window.fetch = (input, init) => {
		const url = new URL(String((input && input.url) || input), location.href);
		const method = String((init && init.method) || 'GET').toUpperCase();
		if (window.__auraFailChoiceLists && (url.pathname === '/api/personalities' || url.pathname === '/api/providers')) {
			return Promise.resolve(json({ error: 'fixture_down' }, 500));
		}
		if (url.pathname === '/api/config' && method === 'GET') {
			return nativeFetch(input, init).then(resp => resp.json()).then(data => {
				data.llm = Object.assign({}, data.llm, { provider: 'fixture-main' });
				return json(data, 200);
			});
		}
		return nativeFetch(input, init);
	};
})();`

func TestConfigChoiceSelectsKeepSavedValueWhenListsFailBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	origin := configRefreshFixtureOrigin(t, "en", false)
	page := newSmokeBrowser(t).MustPage()
	defer page.MustClose()
	page.MustEvalOnNewDocument(configChoiceListFailureFetch)
	page.MustNavigate(origin + "/config#overview").MustWaitLoad()
	waitForJSBool(t, page, `() => !!document.querySelector('.pw-overview-card')`)
	// The fixture template data carries no i18nMeta; load the real field metadata.
	page.MustEval(`async () => {
		window.I18N_META = await (await fetch('/lang/meta.json')).json();
		await selectSection('personality', { scrollBehavior: 'auto' });
		resetDirtySnapshot();
	}`)

	personality := `document.querySelector('select[data-path="personality.core_personality"]')`
	if got := page.MustEval(`() => ` + personality + `.value`).String(); got != "friend" {
		t.Fatalf("personality select value = %q, want saved value friend", got)
	}
	if !page.MustEval(`() => !!document.querySelector('[data-config-choice-retry="personalities"]')`).Bool() {
		t.Fatal("an unloaded personality list must show the retry hint")
	}
	page.MustEval(`() => toggleBool(document.querySelector('.toggle[data-path="personality.engine"]'))`)
	if dirty := page.MustEval(`() => window.AuraConfigState.dirtyPaths().join(',')`).String(); strings.Contains(dirty, "personality.core_personality") {
		t.Fatalf("an unloaded personality list must not change the saved personality; dirty paths: %s", dirty)
	}

	page.MustEval(`async () => {
		window.AuraConfigState.discard();
		await selectSection('llm', { scrollBehavior: 'auto' });
		resetDirtySnapshot();
	}`)
	provider := `document.querySelector('select[data-path="llm.provider"]')`
	if got := page.MustEval(`() => ` + provider + `.value`).String(); got != "fixture-main" {
		t.Fatalf("provider select value = %q, want saved value fixture-main", got)
	}
	page.MustEval(`() => toggleBool(document.querySelector('.toggle[data-path="llm.helper_enabled"]'))`)
	if dirty := page.MustEval(`() => window.AuraConfigState.dirtyPaths().join(',')`).String(); strings.Contains(dirty, "llm.provider") {
		t.Fatalf("an unloaded provider list must not change the saved provider; dirty paths: %s", dirty)
	}

	page.MustEval(`() => {
		window.__auraFailChoiceLists = false;
		document.querySelector('[data-config-choice-retry="providers"]').click();
	}`)
	waitForJSBool(t, page, `() => !document.querySelector('[data-config-choice-retry="providers"]') && !`+provider+`.querySelector('option[data-config-choice-preserved]')`)
	if got := page.MustEval(`() => ` + provider + `.value`).String(); got != "fixture-main" {
		t.Fatalf("provider select value after retry = %q, want fixture-main", got)
	}
	if dirty := page.MustEval(`() => window.AuraConfigState.dirtyPaths().join(',')`).String(); strings.Contains(dirty, "llm.provider") {
		t.Fatalf("refreshing the provider list must keep the saved provider; dirty paths: %s", dirty)
	}
}
