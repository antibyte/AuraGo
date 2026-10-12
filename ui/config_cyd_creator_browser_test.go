package ui

import (
	"testing"

	"github.com/go-rod/rod/lib/proto"
)

// TestConfigCYDCreatorSectionBrowser renders the CYD Creator section with the real config page:
// clicking the main switch keeps the unsaved state, defaults fill the fields, the agent model is a
// provider reference whose empty choice names the main LLM, every field sits in a topic card and
// an edit ends up in the save patch.
func TestConfigCYDCreatorSectionBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	// The populated fixture sends lang/meta.json (I18N_META), which turns the model into a provider choice.
	origin := configRefreshFixtureOrigin(t, "en", true)
	page := newSmokeBrowser(t).MustPage(origin + "/config#overview")
	defer func() {
		// The real click gives the page user activation, so the unsaved-changes guard would block closing.
		_, _ = page.Eval(`() => window.removeEventListener('beforeunload', handleConfigBeforeUnload)`)
		page.MustClose()
	}()
	waitForJSBool(t, page, `() => !!document.querySelector('.pw-overview-card')`)
	page.MustEval(`async () => { configData.cyd_creator = {enabled: false}; AuraConfigState.commit(configData); await selectSection('cyd_creator', {scrollBehavior:'auto'}); resetDirtySnapshot(); }`)
	waitForJSBool(t, page, `() => !!document.querySelector('.toggle[data-path="cyd_creator.enabled"]') && !document.querySelector('[data-path="cyd_creator.memory_mb"]') && !AuraConfigState.isDirty()`)
	// The switch re-renders the section inside its click handler; the click still bubbles to the
	// config state listener, which must read the new value, not the detached old switch. A real
	// mouse click at the switch centre (rod's element click never reaches this inline switch).
	center := page.MustEval(`() => {const r=document.querySelector('.toggle[data-path="cyd_creator.enabled"]').getBoundingClientRect(); return {x: r.left+r.width/2, y: r.top+r.height/2}}`)
	page.Mouse.MustMoveTo(center.Get("x").Num(), center.Get("y").Num()).MustClick(proto.InputMouseButtonLeft)
	waitForJSBool(t, page, `() => AuraConfigState.isDirty() && AuraConfigState.get('cyd_creator.enabled') === true && document.querySelector('.toggle[data-path="cyd_creator.enabled"]')?.classList.contains('on')`)
	waitForJSBool(t, page, `() => document.querySelector('[data-path="cyd_creator.memory_mb"]')?.value==='2048' && document.querySelector('[data-path="cyd_creator.builder_image"]').placeholder==='ghcr.io/antibyte/aurago-cyd-builder:1' && document.querySelector('[data-path="cyd_creator.ota.heartbeat_seconds"]').value==='60'`)
	waitForJSBool(t, page, `() => {const s=document.querySelector('select[data-path="cyd_creator.model"]');return !!s && s.options[0].value==='' && s.options[0].textContent==='Main LLM'}`)
	if orphans := page.MustEval(`() => [...document.querySelectorAll('#content .field-group')].filter(el => !el.closest('.cfg-topic, .cfg-object, [role="dialog"], .modal-overlay, details')).length`).Int(); orphans != 0 {
		t.Fatalf("%d CYD Creator fields sit outside a topic card", orphans)
	}
	if text := page.MustEval(`() => document.getElementById('content').innerText`).Str(); text == "" {
		t.Fatal("section did not render")
	}
	// Take the switched-on state as saved, then edit one number: only that edit is in the patch.
	page.MustEval(`() => { AuraConfigState.commit(); resetDirtySnapshot(); }`)
	waitForJSBool(t, page, `() => !AuraConfigState.isDirty()`)
	page.MustEval(`() => {const i=document.querySelector('[data-path="cyd_creator.memory_mb"]');i.value='3072';i.dispatchEvent(new Event('input',{bubbles:true}));i.dispatchEvent(new Event('change',{bubbles:true}));}`)
	waitForJSBool(t, page, `() => {const p=AuraConfigState.buildPatch(); return AuraConfigState.isDirty() && p.cyd_creator?.memory_mb === 3072 && !('enabled' in p.cyd_creator)}`)
}
