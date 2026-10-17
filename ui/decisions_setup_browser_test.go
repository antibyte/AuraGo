package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-rod/rod/lib/input"
)

func TestDecisionsOneClickSetupBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	browser := newSmokeBrowser(t)
	page := browser.MustPage(configRefreshFixtureOrigin(t, "de", false) + "/config#overview").Timeout(60 * time.Second)
	defer page.MustClose()
	waitForJSBool(t, page, `()=>!!document.querySelector('.pw-overview-card')`)
	page.MustEval(`()=>{
        window.setupCalls=[];
        window.setupStatus={enabled:false,ready:false,installed:false,state:'disabled',code:'service_disabled'};
        const previous=window.fetch;
        window.fetch=async(url,options)=>{
            if(url==='/api/decisions/status')return new Response(JSON.stringify(setupStatus));
            if(url==='/api/decisions/setup'){
                const settings=JSON.parse(options.body);setupCalls.push(settings);
                setupStatus={enabled:true,setup_active:true,setup_id:'fixture-setup',ready:false,state:'installing',progress:{phase:'downloading_runtime',current_bytes:5000000,total_bytes:10000000,percent:50}};
                return new Response(JSON.stringify({setup_id:'fixture-setup',decisions:{...AuraConfigState.get('decisions',{saved:true}),...settings,enabled:true,backend:'local'}}),{status:202});
            }
            if(String(url).startsWith('/api/decisions/'))throw new Error('Unexpected manual lifecycle request '+url);
            return previous(url,options);
        };
    }`)
	page.MustElement(`.sidebar-group:has([data-section="decisions"]) .sidebar-group-header`).MustClick()
	page.MustElement(`[data-section="decisions"]`).MustClick()
	waitForJSBool(t, page, `()=>document.querySelector('[data-decisions-action="setup"]')?.disabled===false`)
	if !page.MustEval(`()=>!document.querySelector('[data-decisions-action="install"]') && document.querySelector('[data-decisions-setup-help]').hidden`).Bool() {
		t.Fatal("normal setup still shows redundant download or instructions")
	}
	page.MustEval(`()=>AuraConfigState.set('agent.name','unsaved elsewhere')`)
	page.MustElement(`[data-decisions-action="setup"]`).MustFocus().MustType(input.Enter)
	waitForJSBool(t, page, `()=>setupCalls.length===1 && AuraConfigState.get('decisions.enabled',{saved:true})===true && document.querySelector('[data-decisions-progress]').textContent.includes('Laufzeit herunterladen')`)
	if !page.MustEval(`()=>setupCalls[0].cpu_fallback_allowed===false && AuraConfigState.buildPatch().agent.name==='unsaved elsewhere' && !AuraConfigState.buildPatch().decisions && document.querySelector('[data-decisions-action="setup"]').disabled && !document.querySelector('[data-decisions-action="stop"]').disabled`).Bool() {
		t.Fatal("one-click setup lost draft state, changed consent or allowed duplicate setup")
	}
	page.MustEval(`()=>{setupStatus.progress={phase:'verifying_model',total_bytes:100,current_bytes:40,percent:40};document.dispatchEvent(new Event('aurago:config-saved'));}`)
	waitForJSBool(t, page, `()=>document.querySelector('[data-decisions-settings] progress')?.value===40 && document.querySelector('[data-decisions-progress]')?.textContent.includes('Modellintegrität')`)
	for _, width := range []int{1440, 390} {
		page.MustSetViewport(width, 900, 1, false)
		if page.MustEval(`()=>document.documentElement.scrollWidth>innerWidth+1`).Bool() {
			t.Fatalf("setup overflow at %d", width)
		}
		if dir := os.Getenv("AURAGO_DECISIONS_SCREENSHOTS"); dir != "" {
			if err := os.MkdirAll(dir, 0755); err != nil {
				t.Fatal(err)
			}
			page.MustScreenshot(filepath.Join(dir, fmt.Sprintf("decisions-one-click-%d.png", width)))
		}
	}
	page.MustEval(`()=>{setupStatus={...setupStatus,setup_active:false,state:'error',code:'runtime_download_failed'};document.dispatchEvent(new Event('aurago:config-saved'));}`)
	waitForJSBool(t, page, `()=>!document.querySelector('[data-decisions-action="setup"]').disabled && document.querySelector('[data-decisions-status]').textContent.includes('Verbindung')`)
	if !page.MustEval(`()=>!document.querySelector('[data-decisions-setup-help]').hidden && document.querySelector('[data-decisions-setup-help]').closest('[data-decisions-mode="local"]') && !document.querySelector('[data-decisions-setup-help]').closest('details')`).Bool() {
		t.Fatal("setup failure did not expose instructions outside advanced settings")
	}
	page.MustElement(`[data-decisions-action="setup"]`).MustClick()
	waitForJSBool(t, page, `()=>setupCalls.length===2`)
	page.MustEval(`()=>{setupStatus={...setupStatus,setup_active:false,ready:true,state:'ready',code:'',backend:'vulkan',device:'AMD GPU',deployment:'native'};document.dispatchEvent(new Event('aurago:config-saved'));}`)
	waitForJSBool(t, page, `()=>document.querySelector('[data-decisions-status]').textContent==='Decisions ist bereit' && !document.querySelector('[data-decisions-action="test"]').disabled`)
	if !page.MustEval(`()=>AuraConfigState.buildPatch().agent.name==='unsaved elsewhere' && document.querySelector('[data-decisions-action="setup"]').hidden && document.querySelector('[data-decisions-setup-help]').hidden`).Bool() {
		t.Fatal("ready setup lost unrelated draft")
	}
	page.MustEval(`()=>{setupStatus={enabled:true,ready:true,backend:'provider',state:'ready'};document.dispatchEvent(new Event('aurago:config-saved'));}`)
	waitForJSBool(t, page, `()=>!document.querySelector('[data-decisions-action="setup"]').hidden && document.querySelector('[data-decisions-action="test"]').hidden`)
}

func TestDecisionsSectionSavePreservesUnsubmittedAndInflightDrafts(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	browser := newSmokeBrowser(t)
	page := browser.MustPage(configRefreshFixtureOrigin(t, "de", false) + "/config#overview").Timeout(60 * time.Second)
	defer page.MustClose()
	waitForJSBool(t, page, `()=>!!window.AuraConfigState`)
	if !page.MustEval(`()=>{
        const s=AuraConfigState;
        s.set('agent.name','keep unrelated draft');
        s.set('decisions.model','keep unsubmitted model');
        s.set('decisions.cpu_fallback_allowed',true);
        const sent={enabled:true,backend:'local',deployment:'auto',cpu_fallback_allowed:true};
        s.set('decisions.cpu_fallback_allowed',false);
        s.commitSectionSent('decisions',{...s.get('decisions',{saved:true}),...sent},sent);
        const patch=s.buildPatch();
        return s.get('decisions.enabled',{saved:true})===true && s.get('decisions.cpu_fallback_allowed',{saved:true})===true &&
          patch.agent.name==='keep unrelated draft' && patch.decisions.model==='keep unsubmitted model' && patch.decisions.cpu_fallback_allowed===false;
    }`).Bool() {
		t.Fatal("section save discarded unsubmitted or in-flight edits")
	}
}

func TestDecisionsOptionalCheckRejectsLateDraftResponse(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	browser := newSmokeBrowser(t)
	page := browser.MustPage(configRefreshFixtureOrigin(t, "de", false) + "/config#overview").Timeout(60 * time.Second)
	defer page.MustClose()
	waitForJSBool(t, page, `()=>!!document.querySelector('.pw-overview-card')`)
	page.MustEval(`()=>{
        window.checkCalls=0;
        const previous=fetch;
        window.fetch=(url,options)=>{
          if(url==='/api/decisions/check'){checkCalls++;return new Promise(resolve=>window.resolveCheck=resolve);}
          return previous(url,options);
        };
    }`)
	page.MustElement(`.sidebar-group:has([data-section="decisions"]) .sidebar-group-header`).MustClick()
	page.MustElement(`[data-section="decisions"]`).MustClick()
	waitForJSBool(t, page, `()=>document.querySelector('[data-decisions-action="check"]')?.disabled===false`)
	if page.MustEval(`()=>checkCalls!==0`).Bool() {
		t.Fatal("opening settings performed a check")
	}
	page.MustEval(`()=>document.querySelector('[data-decisions-action="check"]').click()`)
	waitForJSBool(t, page, `()=>typeof resolveCheck==='function'`)
	page.MustEval(`async()=>{
        const field=document.querySelector('[data-path="decisions.deployment"]');field.value='docker';field.dispatchEvent(new Event('change',{bubbles:true}));
        resolveCheck(new Response(JSON.stringify({deployment:'native',runtime_available:true})));
        await new Promise(resolve=>setTimeout(resolve,30));
    }`)
	if !page.MustEval(`()=>document.querySelector('[data-decisions-check]').hidden && document.querySelector('[data-decisions-action="start"]').disabled`).Bool() {
		t.Fatal("late check response accepted for changed draft")
	}
}
