package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestConfigRefreshInteractionsBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	browser := newSmokeBrowser(t)
	page := browser.MustPage(configRefreshOrigin(t) + "/config#server")
	defer func() {
		page.MustEval(`() => window.removeEventListener('beforeunload', handleConfigBeforeUnload)`)
		page.MustClose()
	}()
	waitForJSBool(t, page, `() => !!document.querySelector('[data-path="server.port"]')`)
	page.MustSetViewport(1440, 900, 1, false)
	change := func(value int) {
		page.MustEval(`value => {const el=document.querySelector('[data-path="server.port"]'); el.value=value; el.dispatchEvent(new Event('input',{bubbles:true}));}`, value)
	}
	waitForJSBool(t, page, `() => document.querySelector('[data-path="server.port"]').hasAttribute('aria-labelledby')`)
	change(-1)
	if page.MustEval(`async () => await saveConfig()`).Bool() {
		t.Fatal("invalid port was saved")
	}
	if !page.MustEval(`() => document.activeElement.matches('[data-path="server.port"][aria-invalid="true"]')`).Bool() {
		t.Fatal("invalid control was not focused")
	}
	change(9090)
	page.MustEval(`() => { window.__navigation = navigateToConfigSection('adguard'); }`)
	waitForJSBool(t, page, `() => !!document.getElementById('cfg-unsaved-decision')`)
	page.MustElement(`[data-decision="stay"]`).MustClick()
	if page.MustEval(`async () => await window.__navigation`).Bool() {
		t.Fatal("stay navigated away")
	}
	if got := page.MustEval(`() => document.querySelector('[data-path="server.port"]').value`).Str(); got != "9090" {
		t.Fatalf("draft lost: %s", got)
	}
	page.MustEval(`() => { window.__navigation = navigateToConfigSection('adguard'); }`)
	waitForJSBool(t, page, `() => !!document.getElementById('cfg-unsaved-decision')`)
	page.MustElement(`[data-decision="save"]`).MustClick()
	if !page.MustEval(`async () => await window.__navigation`).Bool() {
		t.Fatalf("save and continue failed: %s", page.MustEval(`() => ({status:document.getElementById('saveStatus').textContent, errors:AuraConfigState.validate(), dirty:hasUnsavedConfigChanges()})`).String())
	}
	page.MustEval(`async () => {await selectSection('server'); resetDirtySnapshot();}`)
	if got := page.MustEval(`() => document.querySelector('[data-path="server.port"]').value`).Str(); got != "9090" {
		t.Fatalf("saved port not retained: %s", got)
	}
	change(9091)
	page.MustEval(`() => {window.__originalFetch=window.fetch; window.fetch=(url,opts)=>String(url)==='/api/config' && opts?.method==='PUT' ? Promise.resolve(new Response(JSON.stringify({message:'Fixture save failed'}),{status:500,headers:{'Content-Type':'application/json'}})) : window.__originalFetch(url,opts);}`)
	if page.MustEval(`async () => await saveConfig()`).Bool() {
		t.Fatal("failed save returned success")
	}
	if !page.MustEval(`() => hasUnsavedConfigChanges() && document.querySelector('[data-path="server.port"]').value==='9091' && document.getElementById('saveStatus').textContent.includes('Fixture save failed')`).Bool() {
		t.Fatal("failed save lost draft or feedback")
	}
	page.MustEval(`() => { window.fetch=window.__originalFetch; window.__navigation=navigateToConfigSection('adguard'); }`)
	waitForJSBool(t, page, `() => !!document.getElementById('cfg-unsaved-decision')`)
	page.MustElement(`[data-decision="discard"]`).MustClick()
	page.MustEval(`async () => await window.__navigation`)
	page.MustEval(`async () => {await selectSection('server'); resetDirtySnapshot();}`)
	if got := page.MustEval(`() => document.querySelector('[data-path="server.port"]').value`).Str(); got != "9090" {
		t.Fatalf("discard did not restore saved value: %s", got)
	}
	change(9092)
	page.MustEval(`() => {window.fetch=(url,opts)=>String(url)==='/api/config' && (!opts || opts.method!=='PUT') ? Promise.resolve(new Response('{}',{status:503})) : window.__originalFetch(url,opts);}`)
	if !page.MustEval(`async () => await saveConfig()`).Bool() {
		t.Fatal("successful PUT was lost after reload failure")
	}
	page.MustEval(`async () => {window.fetch=window.__originalFetch; await selectSection('server'); resetDirtySnapshot();}`)
	if page.MustEval(`() => document.querySelector('[data-path="server.port"]').value`).Str() != "9092" {
		t.Fatal("reload failure lost saved inputs")
	}
	page.MustEval(`async () => {window.__adguardModule=SECTION_MODULES.adguard.m; SECTION_MODULES.adguard.m='fixture-missing'; await selectSection('adguard');}`)
	if !page.MustEval(`() => !!document.querySelector('[data-config-retry]') && !_moduleCache['fixture-missing']`).Bool() {
		t.Fatal("failed module cannot be retried")
	}
	page.MustEval(`() => {SECTION_MODULES.adguard.m=window.__adguardModule;}`)
	page.MustElement("[data-config-retry]").MustClick()
	waitForJSBool(t, page, `() => !!document.querySelector('[data-path="adguard.url"]')`)
	if !page.MustEval(`() => document.getElementById('adg-test-btn').getAttribute('aria-disabled')==='true' && !document.getElementById('adg-test-btn-reason').hidden`).Bool() {
		t.Fatal("connection test missing visible lock reason")
	}
	page.MustEval(`async () => {configData.adguard={enabled:true,url:'http://fixture.invalid',password:'••••••••'}; AuraConfigState.commit(configData); await selectSection('adguard'); resetDirtySnapshot();}`)
	page.MustElement("#adg-test-btn").MustClick()
	waitForJSBool(t, page, `() => document.getElementById('adg-test-result').classList.contains('is-danger')`)
	if !page.MustEval(`() => document.querySelector('[data-path="adguard.url"]').value==='http://fixture.invalid' && !!document.querySelector('.cfg-save-scope')`).Bool() {
		t.Fatal("failed test lost inputs or credential scope")
	}
	page.MustEval(`() => {const input=document.getElementById('sidebarSearchInput'); input.value='context window'; input.dispatchEvent(new Event('input',{bubbles:true}));}`)
	page.MustElement(`.sidebar-item[data-section="optimizations"]`).MustClick()
	waitForJSBool(t, page, `() => document.activeElement.dataset.path==='agent.context_window'`)
	page.MustEval(`() => {const input=document.getElementById('sidebarSearchInput'); input.value=''; input.dispatchEvent(new Event('input',{bubbles:true}));}`)

	// Explicit tiers stay within topic groups, unknown paths and required fields remain visible.
	page.MustEval(`() => {
	 document.getElementById('content').innerHTML='<div class="cfg-section active"><div id="topic-a"><div class="field-group"><input data-path="server.debug_mode"></div></div><div id="topic-b"><div class="field-group"><input data-path="agent.debug_mode" required></div><div class="field-group"><input data-path="service.max_connections"></div><div class="field-group"><input data-path="circuit_breaker.llm_timeout_seconds" type="number" value="30"></div></div></div>';
	 enhanceConfigSectionLayout('server'); enhanceConfigSectionLayout('server');
	}`)
	if !page.MustEval(`() => document.querySelectorAll('.pw-advanced').length===2 && !!document.querySelector('#topic-a > details') && !!document.querySelector('#topic-b > details') && !document.querySelector('[required]').closest('details') && !document.querySelector('[data-path="service.max_connections"]').closest('details')`).Bool() {
		t.Fatal("advanced disclosure crossed a topic boundary or hid a required/unknown field")
	}
	page.MustEval(`() => showConfigValidationErrors([{path:'circuit_breaker.llm_timeout_seconds',code:'number',message:'Invalid'}])`)
	if !page.MustEval(`() => document.activeElement.dataset.path==='circuit_breaker.llm_timeout_seconds' && document.activeElement.closest('details').open`).Bool() {
		t.Fatal("validation did not open the advanced field")
	}
	page.MustEval(`() => focusConfigField('server.debug_mode')`)
	waitForJSBool(t, page, `() => document.activeElement.dataset.path==='server.debug_mode' && document.activeElement.closest('details').open`)

	page.MustEval(`async () => {await selectSection('providers'); resetDirtySnapshot();}`)
	page.MustElement(".prov-section-actions button").MustClick()
	waitForJSBool(t, page, `() => !!document.getElementById('provider-modal-overlay')`)
	for _, width := range []int{390, 768, 1024, 1440, 1920} {
		page.MustSetViewport(width, 900, 1, width == 390)
		if page.MustEval(`() => document.documentElement.scrollWidth>innerWidth+1 || document.querySelector('.prov-modal-panel').getBoundingClientRect().right>innerWidth+1`).Bool() {
			t.Errorf("provider dialog overflows at %d", width)
		}
	}
	waitForJSBool(t, page, `() => document.getElementById('provider-modal-overlay').contains(document.activeElement)`)
	page.MustEval(`() => {const tab=document.querySelector('[data-prov-tab="identity"]'); tab.focus(); tab.dispatchEvent(new KeyboardEvent('keydown',{key:'ArrowRight',bubbles:true,cancelable:true}));}`)
	if !page.MustEval(`() => document.activeElement.dataset.provTab==='model' && document.activeElement.getAttribute('aria-selected')==='true'`).Bool() {
		t.Fatal("provider tabs lack keyboard navigation")
	}
	page.MustEval(`() => {const last=document.getElementById('prov-save-btn'); last.focus(); last.dispatchEvent(new KeyboardEvent('keydown',{key:'Tab',bubbles:true,cancelable:true}));}`)
	if !page.MustEval(`() => document.activeElement.id==='provider-modal-close-btn'`).Bool() {
		t.Fatal("provider dialog does not trap Tab")
	}
	for _, theme := range []string{"dark", "light"} {
		for _, width := range []int{390, 1440} {
			page.MustSetViewport(width, 900, 1, false)
			page.MustEval(`theme => {document.documentElement.dataset.theme=theme;document.body.dataset.theme=theme;}`, theme)
			page.MustEval(`() => new Promise(resolve => setTimeout(resolve,250))`)
			if dir := os.Getenv("AURAGO_BROWSER_ARTIFACT_DIR"); dir != "" {
				if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("refresh-provider-dialog-%s-%d.png", theme, width)), page.MustScreenshot(), 0o644); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
	if !page.MustEval(`() => document.querySelector('#provider-modal-overlay [role="dialog"], #provider-modal-overlay[role="dialog"]')!==null`).Bool() {
		t.Fatal("provider editor lacks dialog semantics")
	}
	page.MustElement("#prov-save-btn").MustClick()
	if !page.MustEval(`() => document.activeElement.id==='prov-id' && document.activeElement.getAttribute('aria-invalid')==='true' && document.querySelector('[data-prov-tab="identity"]').getAttribute('aria-selected')==='true'`).Bool() {
		t.Fatal("provider validation did not reveal and focus the affected tab")
	}
	page.MustElement("#provider-modal-cancel-btn").MustClick()
	waitForJSBool(t, page, `() => !document.getElementById('provider-modal-overlay') && document.activeElement.matches('.prov-section-actions button')`)
	page.MustSetViewport(1024, 768, 1, false)
	if !page.MustEval(`() => getComputedStyle(document.getElementById('cfg-hamburger')).display !== 'none'`).Bool() {
		t.Fatalf("drawer button hidden: %s", page.MustEval(`() => ({width:innerWidth, match:matchMedia('(max-width:1099px)').matches,header:document.querySelector('.header-left').getBoundingClientRect().toJSON()})`).String())
	}
	page.MustElement("#cfg-hamburger").MustClick()
	if !page.MustEval(`() => !document.getElementById('sidebar').inert && document.getElementById('cfg-hamburger').getAttribute('aria-expanded')==='true' && document.activeElement.id==='sidebarSearchInput'`).Bool() {
		t.Fatal("drawer did not open accessibly")
	}
	page.MustEval(`() => document.getElementById('sidebarSearchInput').dispatchEvent(new KeyboardEvent('keydown',{key:'Escape',bubbles:true,cancelable:true}))`)
	if !page.MustEval(`() => document.getElementById('sidebar').inert && document.activeElement.id==='cfg-hamburger'`).Bool() {
		t.Fatal("drawer close did not restore focus")
	}
	page.MustElement("#cfg-hamburger").MustClick()
	page.MustSetViewport(1440, 900, 1, false)
	waitForJSBool(t, page, `() => !document.getElementById('sidebar').inert && !document.getElementById('sidebar-backdrop').classList.contains('open')`)
	if !page.MustEval(`() => [...document.querySelectorAll('.sidebar-group.collapsed .sidebar-item')].every(el=>getComputedStyle(el).visibility==='hidden')`).Bool() {
		t.Fatal("collapsed navigation items remain keyboard-visible")
	}
}
