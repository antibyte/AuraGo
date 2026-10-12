package ui

import "testing"

// TestConfigCYDCreatorSectionBrowser renders the enabled CYD Creator section with the real config
// page: defaults fill the fields, the agent model is a provider reference whose empty choice names
// the main LLM, every field sits in a topic card and an edit marks the draft dirty.
func TestConfigCYDCreatorSectionBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	// The populated fixture sends lang/meta.json (I18N_META), which turns the model into a provider choice.
	origin := configRefreshFixtureOrigin(t, "en", true)
	page := newSmokeBrowser(t).MustPage(origin + "/config#overview")
	defer page.MustClose()
	waitForJSBool(t, page, `() => !!document.querySelector('.pw-overview-card')`)
	page.MustEval(`async () => { configData.cyd_creator = {enabled: true}; AuraConfigState.commit(configData); await selectSection('cyd_creator', {scrollBehavior:'auto'}); resetDirtySnapshot(); }`)
	waitForJSBool(t, page, `() => document.querySelector('[data-path="cyd_creator.memory_mb"]')?.value==='2048' && document.querySelector('[data-path="cyd_creator.builder_image"]').placeholder==='ghcr.io/antibyte/aurago-cyd-builder:1' && document.querySelector('[data-path="cyd_creator.ota.heartbeat_seconds"]').value==='60'`)
	waitForJSBool(t, page, `() => {const s=document.querySelector('select[data-path="cyd_creator.model"]');return !!s && s.options[0].value==='' && s.options[0].textContent===t('config.cyd_creator.model_main')}`)
	if orphans := page.MustEval(`() => [...document.querySelectorAll('#content .field-group')].filter(el => !el.closest('.cfg-topic, .cfg-object, [role="dialog"], .modal-overlay, details')).length`).Int(); orphans != 0 {
		t.Fatalf("%d CYD Creator fields sit outside a topic card", orphans)
	}
	if text := page.MustEval(`() => document.getElementById('content').innerText`).Str(); text == "" {
		t.Fatal("section did not render")
	}
	page.MustEval(`() => {const i=document.querySelector('[data-path="cyd_creator.memory_mb"]');i.value='3072';i.dispatchEvent(new Event('input',{bubbles:true}));i.dispatchEvent(new Event('change',{bubbles:true}));}`)
	waitForJSBool(t, page, `() => AuraConfigState.isDirty()`)
}
