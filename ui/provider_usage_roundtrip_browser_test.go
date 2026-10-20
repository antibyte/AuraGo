package ui

import (
	"encoding/json"
	"testing"
	"time"
)

func TestProviderUsageRoundTripKeepsGeneralDraftBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	page := newSmokeBrowser(t).MustPage(configRefreshFixtureOrigin(t, "en", false) + "/config#overview").Timeout(60 * time.Second)
	defer page.MustClose()
	waitForJSBool(t, page, `() => !!document.querySelector('.pw-overview-card')`)
	page.MustEval(`() => {
		window.fixtureProviders = [{
			id: 'fixture-oauth-profile', name: 'Fixture OAuth', usage: 'general', type: 'anthropic',
			base_url: 'https://custom.example.invalid/v1', model: 'claude/custom-model',
			context_window: 32768, max_output_tokens: 4096, auth_type: 'oauth2', api_key: '••••••••',
			oauth_auth_url: 'https://login.custom.example.invalid/authorize',
			oauth_token_url: 'https://login.custom.example.invalid/token', oauth_client_id: 'fixture-client',
			oauth_client_secret: '••••••••', oauth_scopes: 'profile:read custom:scope', account_id: '',
			models: [{name:'fixture-priced-model', input_per_million:1.25, output_per_million:2.5}],
			capabilities: {auto:false, tool_calling:true, structured_outputs:false, multimodal:true, detected_model:'claude/custom-model', source:'manual'}
		}];
		window.savedProviderPayloads = [];
        window.fixtureProviderRevision = '"usage-r1"';
		const originalFetch = window.fetch.bind(window);
		window.fetch = async (input, options = {}) => {
			const path = new URL(typeof input === 'string' ? input : input.url, location.origin).pathname;
			if (path === '/api/providers') {
				if (options.method === 'PUT') {
					if (options.headers['If-Match'] !== window.fixtureProviderRevision) throw new Error('missing or stale provider revision');
                    window.fixtureProviders = JSON.parse(options.body);
					window.savedProviderPayloads.push(window.fixtureProviders);
					window.fixtureProviderRevision = '"usage-r'+(window.savedProviderPayloads.length+1)+'"';
                    return new Response(JSON.stringify({status:'ok',revision:window.fixtureProviderRevision}), {headers:{ETag:window.fixtureProviderRevision}});
				}
				return new Response(JSON.stringify(window.fixtureProviders), {headers:{ETag:window.fixtureProviderRevision}});
			}
			if (path === '/api/providers/capabilities') {
				return new Response(JSON.stringify({auto:true, tool_calling:false, structured_outputs:false, multimodal:false, detected_model:'', source:'fixture', known:true}));
			}
			return originalFetch(input, options);
		};
	}`)
	page.MustEval(`async () => { await selectSection('providers', {scrollBehavior:'auto'}); await loadProviders(); }`)
	waitForJSBool(t, page, `() => !!document.querySelector('#providers-list') && typeof providerEdit === 'function' && typeof providerAdd === 'function' && providersLoaded`)
	page.MustEval(`() => { providersCache = window.fixtureProviders; providersLoaded = true; providerRenderCards(); providerEdit(0); }`)
	waitForJSBool(t, page, `() => !!document.querySelector('#provider-modal-overlay')`)
	page.MustEval(`() => {
		const usage = document.getElementById('prov-usage');
		usage.value = 'decisions'; usage.dispatchEvent(new Event('change', {bubbles:true}));
		usage.value = 'general'; usage.dispatchEvent(new Event('change', {bubbles:true}));
		document.getElementById('prov-save-btn').click();
	}`)
	waitForJSBool(t, page, `() => window.savedProviderPayloads.length === 1 || document.querySelector('#shared-modal-overlay')?.classList.contains('active') || !!document.querySelector('.pw-field-error[data-config-validation]')`)
	if page.MustEval(`() => window.savedProviderPayloads.length`).Int() != 1 {
		state := page.MustEval(`() => JSON.stringify({modal:!!document.querySelector('#provider-modal-overlay'), confirm:document.querySelector('#shared-modal-message')?.textContent || '', errors:[...document.querySelectorAll('.pw-field-error[data-config-validation]')].map(el=>el.textContent), id:document.getElementById('prov-id')?.value, usage:document.getElementById('prov-usage')?.value, type:document.getElementById('prov-type')?.value, auth:document.getElementById('prov-auth-type')?.value, url:document.getElementById('prov-url')?.value})`).Str()
		t.Fatalf("provider save did not reach PUT payload: %s", state)
	}
	var saved [][]map[string]any
	if err := json.Unmarshal([]byte(page.MustEval(`() => JSON.stringify(window.savedProviderPayloads)`).Str()), &saved); err != nil {
		t.Fatal(err)
	}
	if len(saved) != 1 || len(saved[0]) != 1 {
		t.Fatalf("expected one saved provider payload, got %#v", saved)
	}
	existing := saved[0][0]
	for key, want := range map[string]any{
		"type": "anthropic", "usage": "general", "base_url": "https://custom.example.invalid/v1", "model": "claude/custom-model",
		"context_window": float64(32768), "max_output_tokens": float64(4096), "auth_type": "oauth2",
		"oauth_auth_url": "https://login.custom.example.invalid/authorize", "oauth_token_url": "https://login.custom.example.invalid/token",
		"oauth_client_id": "fixture-client", "oauth_scopes": "profile:read custom:scope", "oauth_client_secret": "••••••••",
	} {
		if got := existing[key]; got != want {
			t.Errorf("existing provider payload %s = %#v, want %#v", key, got, want)
		}
	}
	if models, ok := existing["models"].([]any); !ok || len(models) != 1 || models[0].(map[string]any)["name"] != "fixture-priced-model" {
		t.Errorf("existing provider model prices were not preserved: %#v", existing["models"])
	}
	if caps, ok := existing["capabilities"].(map[string]any); !ok || caps["auto"] != false || caps["tool_calling"] != true || caps["multimodal"] != true {
		t.Errorf("existing provider capabilities were not preserved: %#v", existing["capabilities"])
	}

	waitForJSBool(t, page, `() => !document.getElementById('provider-modal-overlay') && !providersSaving`)
	page.MustEval(`() => providerAdd()`)
	waitForJSBool(t, page, `() => !!document.querySelector('#provider-modal-overlay')`)
	page.MustEval(`() => {
		const change = (id, value) => { const el = document.getElementById(id); el.value = value; el.dispatchEvent(new Event('change', {bubbles:true})); };
		change('prov-id', 'fixture-new-oauth-profile');
		change('prov-name', 'New OAuth profile');
		change('prov-type', 'google');
		document.getElementById('prov-url').value = 'https://new-custom.example.invalid/v1';
		document.getElementById('prov-model').value = 'gemini/custom-model';
		change('prov-auth-type', 'oauth2');
		document.getElementById('prov-oauth-auth-url').value = 'https://new-login.example.invalid/authorize';
		document.getElementById('prov-oauth-token-url').value = 'https://new-login.example.invalid/token';
		document.getElementById('prov-oauth-client-id').value = 'new-fixture-client';
		document.getElementById('prov-oauth-client-secret').value = 'test-secret-marker-not-for-storage';
		document.getElementById('prov-oauth-scopes').value = 'openid profile';
		change('prov-usage', 'decisions');
		change('prov-usage', 'general');
		document.getElementById('prov-save-btn').click();
	}`)
	waitForJSBool(t, page, `() => window.savedProviderPayloads.length === 2`)
	if !page.MustEval(`() => {
		const payload = window.savedProviderPayloads[1];
		const provider = payload.find(p => p.id === 'fixture-new-oauth-profile');
		return !!provider && provider.type === 'google' && provider.usage === 'general' &&
			provider.base_url === 'https://new-custom.example.invalid/v1' && provider.model === 'gemini/custom-model' &&
			provider.oauth_auth_url === 'https://new-login.example.invalid/authorize' &&
			provider.oauth_token_url === 'https://new-login.example.invalid/token' &&
			provider.oauth_client_id === 'new-fixture-client' && provider.oauth_scopes === 'openid profile' &&
			provider.oauth_client_secret === 'test-secret-marker-not-for-storage' &&
			!Object.keys(localStorage).some(key => localStorage.getItem(key)?.includes('test-secret-marker-not-for-storage'));
	}`).Bool() {
		t.Fatal("new provider save did not retain its general/OAuth draft or copied the secret into browser storage")
	}
}
