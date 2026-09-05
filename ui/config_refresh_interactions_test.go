package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-rod/rod/lib/input"
	"github.com/go-rod/rod/lib/proto"
)

func TestConfigNavigationOverlayBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	page := newSmokeBrowser(t).MustPage(configRefreshFixtureOrigin(t, "de", true) + "/config#overview")
	defer page.MustClose()
	waitForJSBool(t, page, `() => !!document.querySelector('.pw-overview-card') && !!document.getElementById('radialTrigger')`)
	page = page.Timeout(60 * time.Second)
	page.MustEval(`() => document.querySelector('.radial-item[href="/dashboard"]').addEventListener('click', event => {event.preventDefault(); window.navigationClicked=true;})`)
	for _, width := range []int{390, 768, 1024, 1440, 1920} {
		page.MustSetViewport(width, 1000, 1, width == 390)
		for _, theme := range []string{"dark", "light"} {
			for _, density := range []string{"comfortable", "compact"} {
				page.MustEval(`(theme,density) => {document.documentElement.dataset.theme=theme; document.body.dataset.theme=theme; AuraPrecisionWorkspace.setDensity(density); window.navigationClicked=false;}`, theme, density)
				page.MustElement("#radialTrigger").MustClick()
				page.MustEval(`async () => await Promise.all(document.getAnimations().filter(a=>a.effect?.getTiming().iterations!==Infinity).map(a=>a.finished.catch(()=>{})))`)
				if dir := os.Getenv("AURAGO_BROWSER_ARTIFACT_DIR"); dir != "" {
					if err := os.MkdirAll(dir, 0o755); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("navigation-%d-%s-%s.png", width, theme, density)), page.MustScreenshot(), 0o644); err != nil {
						t.Fatal(err)
					}
				}
				if !page.MustEval(`() => [...document.querySelectorAll('#radialTrigger, .radial-item[href]')].every(el=>{const r=el.getBoundingClientRect(); return el.contains(document.elementFromPoint(r.x+r.width/2,r.y+r.height/2));})`).Bool() {
					t.Errorf("%d %s %s: navigation controls are covered by the backdrop", width, theme, density)
				} else {
					point := page.MustEval(`() => {const r=document.querySelector('.radial-item[href="/dashboard"] .radial-item-label').getBoundingClientRect(); return [r.x+r.width/2,r.y+r.height/2];}`).Arr()
					page.Mouse.MustMoveTo(point[0].Num(), point[1].Num()).MustClick(proto.InputMouseButtonLeft)
					if !page.MustEval(`() => window.navigationClicked===true`).Bool() {
						t.Fatal("navigation link did not receive the pointer click")
					}
					page.MustElement("#radialTrigger").MustClick()
					if page.MustEval(`() => document.getElementById('radialMenu').classList.contains('open')`).Bool() {
						t.Fatal("trigger could not close the menu")
					}
					page.MustElement("#radialTrigger").MustClick()
					page.Mouse.MustMoveTo(8, 980).MustClick(proto.InputMouseButtonLeft)
					if page.MustEval(`() => document.getElementById('radialMenu').classList.contains('open')`).Bool() {
						t.Fatal("backdrop could not close the menu")
					}
					page.MustElement("#radialTrigger").MustClick()
				}
				page.Keyboard.MustType(input.Escape)
				if page.MustEval(`() => document.getElementById('radialMenu').classList.contains('open')`).Bool() {
					t.Fatal("Escape could not close the menu")
				}
			}
		}
	}
}

func TestConfigRefreshSIPStepsBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	browser := newSmokeBrowser(t)
	page := browser.MustPage(configRefreshFixtureOrigin(t, "de", true) + "/config#sip")
	defer page.MustClose()
	page.MustSetViewport(1440, 900, 1, false)
	waitForJSBool(t, page, `() => !!document.querySelector('[data-sip-profile="test"]')`)
	waitForJSBool(t, page, `() => document.getElementById('sip-status').textContent!==t('config.sip.loading_status')`)
	page.MustElement(`[data-sip-profile="test"]`).MustClick()
	waitForJSBool(t, page, `() => !!document.querySelector('.sip-test-diagnostic')`)
	// Status refresh replaces the profile; select its controls after that refresh.
	waitForJSBool(t, page, `() => document.getElementById('sip-status').textContent!==t('config.sip.loading_status')`)
	page.MustElement(`.sip-profile-actions [data-sip-profile="credentials"]`).MustClick()
	waitForJSBool(t, page, `() => !!document.querySelector('[data-sip-wizard-field="registrar"]')`)
	page.MustEval(`async () => await renderSIPSection()`)
	waitForJSBool(t, page, `() => document.getElementById('sip-status').textContent!==t('config.sip.loading_status')`)
	page.MustElement(`[data-sip-profile="test"]`).MustClick()
	waitForJSBool(t, page, `() => !!document.querySelector('.sip-test-diagnostic') && document.getElementById('sip-status').textContent!==t('config.sip.loading_status')`)
	page.MustElement(`.sip-test-diagnostic [data-sip-profile="credentials"]`).MustClick()
	waitForJSBool(t, page, `() => !!document.querySelector('[data-sip-wizard-field="registrar"]')`)
	page.MustEval(`() => {const el=document.querySelector('[data-sip-wizard-field="registrar"]'); el.value=''; el.dispatchEvent(new Event('input',{bubbles:true}));}`)
	page.MustElement(`[data-sip-wizard="calling"]`).MustClick()
	if !page.MustEval(`() => document.activeElement.dataset.sipWizardField==='registrar' && document.activeElement.getAttribute('aria-invalid')==='true'`).Bool() {
		t.Fatal("SIP validation did not focus the missing field")
	}
	page.MustEval(`() => {const el=document.querySelector('[data-sip-wizard-field="registrar"]'); el.value='sip.fixture.invalid'; el.dispatchEvent(new Event('input',{bubbles:true}));}`)
	for _, width := range []int{390, 768, 1024, 1440, 1920} {
		page.MustSetViewport(width, 900, 1, false)
		for _, theme := range []string{"dark", "light"} {
			page.MustEval(`theme => {document.documentElement.dataset.theme=theme;document.body.dataset.theme=theme;}`, theme)
			if !page.MustEval(`() => {const content=document.getElementById('content'); return content.scrollWidth<=content.clientWidth+1 && document.querySelector('.sip-wizard-fields').closest('.cfg-topic')!==null;}`).Bool() {
				t.Errorf("SIP credential layout failed at %d %s", width, theme)
			}
			if dir := os.Getenv("AURAGO_BROWSER_ARTIFACT_DIR"); dir != "" && (width == 390 || width == 1440) {
				if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("sip-credentials-%s-%d.png", theme, width)), page.MustScreenshot(), 0o644); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
	page.MustElement(`[data-sip-wizard="calling"]`).MustClick()
	waitForJSBool(t, page, `() => !!document.querySelector('[data-sip-wizard="review"]')`)
	page.MustElement(`[data-sip-wizard="review"]`).MustClick()
	waitForJSBool(t, page, `() => !!document.querySelector('[data-sip-wizard="apply"]')`)
	if !page.MustEval(`() => document.querySelector('.sip-wizard-shell').classList.contains('cfg-topic') && sipWizardValues.registrar==='sip.fixture.invalid'`).Bool() {
		t.Fatal("SIP step change lost its structure or inputs")
	}
}

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
	// A credential action uses its Vault endpoint without writing the YAML config.
	page.MustEval(`() => {window.__writes=[]; window.fetch=(url,opts)=>{if(opts?.method==='POST' || opts?.method==='PUT')window.__writes.push(String(url)); return String(url)==='/api/vault/secrets' ? Promise.resolve(new Response('{"status":"ok"}',{headers:{'Content-Type':'application/json'}})) : window.__originalFetch(url,opts);}; document.getElementById('adg-password').value='local-fixture-only'; adgSavePassword();}`)
	waitForJSBool(t, page, `() => document.getElementById('adg-password').value===''`)
	if !page.MustEval(`() => JSON.stringify(window.__writes)==='["/api/vault/secrets"]'`).Bool() {
		t.Fatal("credential save crossed its storage boundary")
	}
	page.MustEval(`async () => {window.fetch=window.__originalFetch; await selectSection('here_now'); resetDirtySnapshot();}`)
	page.MustEval(`() => {const row=document.querySelector('.cfg-toggle-row-compact'); window.__toggleCopy=row.querySelector('.cfg-toggle-copy').textContent; toggleBool(row.querySelector('.toggle'));}`)
	if !page.MustEval(`() => document.querySelector('.cfg-toggle-row-compact .cfg-toggle-copy').textContent===window.__toggleCopy`).Bool() {
		t.Fatal("toggle replaced its field description")
	}
	page.MustEval(`() => {toggleBool(document.querySelector('.cfg-toggle-row-compact .toggle')); AuraConfigState.commit(configData); resetDirtySnapshot();}`)
	page.MustEval(`() => {const input=document.getElementById('sidebarSearchInput'); input.value='context window'; input.dispatchEvent(new Event('input',{bubbles:true}));}`)
	page.MustElement(`.sidebar-item[data-section="optimizations"]`).MustClick()
	waitForJSBool(t, page, `() => document.activeElement.dataset.path==='agent.context_window'`)
	page.MustEval(`() => {const input=document.getElementById('sidebarSearchInput'); input.value=''; input.dispatchEvent(new Event('input',{bubbles:true}));}`)

	// Explicit tiers stay within topic groups, unknown paths and required fields remain visible.
	if !page.MustEval(`() => {
	 const root=document.createElement('div');
	 root.innerHTML='<h2 data-config-topic-title>Connection</h2><p class="section-desc">Description</p><p role="status">Ready</p><div class="field-group"><input value="draft"></div>';
	 AuraConfigForm.layout(root,'server'); const first=root.innerHTML;
	 AuraConfigForm.layout(root,'server');
	 return root.children.length===1 && root.querySelector('.pw-panel-body').children.length===3 && root.querySelector('input').value==='draft' && root.innerHTML===first;
	}`).Bool() {
		t.Fatal("topic layout split its description/status/field or changed an existing draft")
	}
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
		for _, theme := range []string{"dark", "light"} {
			for _, density := range []string{"comfortable", "compact"} {
				page.MustEval(`(theme,density) => {document.documentElement.dataset.theme=theme;document.body.dataset.theme=theme;AuraPrecisionWorkspace.setDensity(density);}`, theme, density)
				if page.MustEval(`() => document.documentElement.scrollWidth>innerWidth+1 || document.querySelector('.prov-modal-panel').getBoundingClientRect().right>innerWidth+1 || [...document.querySelectorAll('.prov-modal-panel .field-input')].some(el=>getComputedStyle(el).fontSize!=='16px')`).Bool() {
					t.Errorf("provider dialog layout fails at %d %s %s", width, theme, density)
				}
			}
		}
	}
	page.MustEval(`() => AuraPrecisionWorkspace.setDensity('comfortable')`)
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
	page.MustElement(".prov-section-actions button").MustClick()
	page.MustEval(`() => {
	 window.__writes=[];
	 window.fetch=(url,opts)=>{if(opts?.method==='PUT') window.__writes.push(String(url)); return String(url)==='/api/providers' && opts?.method==='PUT' ? Promise.resolve(new Response('{"status":"ok"}',{headers:{'Content-Type':'application/json'}})) : window.__originalFetch(url,opts);};
	 document.getElementById('prov-id').value='fixture-second'; document.getElementById('prov-name').value='Local preview';
	 document.getElementById('prov-url').value='https://provider.fixture.invalid/v1'; document.getElementById('prov-key').value='local-fixture-only';
	}`)
	page.MustElement("#prov-save-btn").MustClick()
	waitForJSBool(t, page, `() => !document.getElementById('provider-modal-overlay')`)
	if !page.MustEval(`() => JSON.stringify(window.__writes)==='["/api/providers"]'`).Bool() {
		t.Fatal("provider save crossed its storage boundary")
	}
	page.MustEval(`() => {window.fetch=window.__originalFetch;}`)
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
