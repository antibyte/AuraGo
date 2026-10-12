package ui

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDecisionsLocaleKeys(t *testing.T) {
	var reference map[string]string
	if err := json.Unmarshal(mustReadUIFile(t, "lang/config/en.json"), &reference); err != nil {
		t.Fatal(err)
	}
	for _, locale := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		var translated map[string]string
		if err := json.Unmarshal(mustReadUIFile(t, "lang/config/"+locale+".json"), &translated); err != nil {
			t.Fatal(err)
		}
		for key := range reference {
			if strings.HasPrefix(key, "config.decisions.") && strings.TrimSpace(translated[key]) == "" {
				t.Errorf("%s missing %s", locale, key)
			}
		}
	}
}

func TestDecisionsConfigBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	browser := newSmokeBrowser(t)
	page := browser.MustPage(configRefreshFixtureOrigin(t, "de", false) + "/config#overview")
	defer page.MustClose()
	waitForJSBool(t, page, `()=>!!document.querySelector('.pw-overview-card')`)
	page.MustEval(`async()=>{
        window.decisionActions=[];window.decisionStatus={enabled:false,installed:false,ready:false,state:'disabled',deployment:'native',code:'service_disabled'};
        const previous=window.fetch;
        window.fetch=async(url,options)=>{
            if(url==='/api/decisions/status') {
                if(window.holdDecisionStatus) return new Promise(resolve=>window.resolveDecisionStatus=resolve);
                return new Response(JSON.stringify(window.decisionStatus));
            }
            if(url.startsWith('/api/decisions/')) {window.decisionActions.push(url);return new Response('{}');}
            return previous(url,options);
        };
        await selectSection('decisions',{scrollBehavior:'auto'});
    }`)
	waitForJSBool(t, page, `()=>document.querySelector('[data-decisions-status]')?.textContent.includes('Deaktiviert')`)
	if page.MustEval(`()=>AuraConfigState.isDirty() || decisionActions.length!==0`).Bool() {
		t.Fatal("opening settings changed configuration or started service")
	}
	page.MustEval(`()=>document.querySelector('[data-path="decisions.enabled"]').click()`)
	if !page.MustEval(`()=>AuraConfigState.buildPatch().decisions.enabled===true && document.querySelector('[data-decisions-action="install"]').disabled`).Bool() {
		t.Fatal("activation draft/save gate failed")
	}
	page.MustEval(`async()=>{await saveConfig();window.decisionStatus={enabled:true,installed:false,ready:false,state:'blocked',code:'cpu_consent_required',deployment:'native'};await selectSection('decisions',{scrollBehavior:'auto'});}`)
	waitForJSBool(t, page, `()=>document.querySelector('[data-decisions-status]')?.textContent.includes('Freigabe')`)
	page.MustEval(`()=>document.querySelector('[data-path="decisions.cpu_fallback_allowed"]').dispatchEvent(new KeyboardEvent('keydown',{key:'Enter',bubbles:true}))`)
	if !page.MustEval(`async()=>{await saveConfig();const config=await(await fetch('/api/config')).json();return config.decisions.cpu_fallback_allowed===true;}`).Bool() {
		t.Fatal("explicit CPU permission not saved")
	}
	page.MustEval(`async()=>{window.decisionStatus={enabled:true,installed:true,ready:true,state:'ready',backend:'Vulkan',device:'Intel integrated GPU',deployment:'native'};await selectSection('decisions',{scrollBehavior:'auto'});}`)
	waitForJSBool(t, page, `()=>document.querySelector('[data-decisions-action="test"]')?.disabled===false`)
	page.MustEval(`()=>document.querySelector('[data-decisions-action="test"]').click()`)
	waitForJSBool(t, page, `()=>document.querySelector('[data-decisions-result]')?.textContent.includes('Entscheidungstest abgeschlossen')`)
	if page.MustEval(`()=>decisionActions.length`).Int() != 1 {
		t.Fatal("test made unexpected requests")
	}
	page.MustEval(`()=>document.querySelector('[data-path="decisions.cpu_fallback_allowed"]').click()`)
	if !page.MustEval(`async()=>{await saveConfig();const config=await(await fetch('/api/config')).json();return config.decisions.cpu_fallback_allowed===false;}`).Bool() {
		t.Fatal("CPU revocation not saved")
	}
	page.MustEval(`async()=>{window.decisionStatus={enabled:true,installed:true,ready:true,state:'ready',backend:'Vulkan',device:'GPU <img src=x onerror=alert(1)>',deployment:'native',execution_mode:'mixed',cpu_fallback_active:true,cpu_fallback_reason:'gpu_accelerator_partial',code:'cpu_fallback_consent_used'};await selectSection('decisions',{scrollBehavior:'auto'});}`)
	waitForJSBool(t, page, `()=>document.querySelector('[data-decisions-settings]')?.textContent.includes('CPU-Anteile sind aktiv')`)
	if !page.MustEval(`()=>document.querySelector('[data-decisions-settings]').textContent.includes('nur einen Teil des Modells') && document.querySelector('[data-decisions-settings]').textContent.includes('GPU + CPU')`).Bool() {
		t.Fatal("mixed CPU operation did not display actual mode and fallback cause")
	}
	if page.MustEval(`()=>!!document.querySelector('[data-decisions-settings] img')`).Bool() {
		t.Fatal("device data became HTML")
	}
	for _, width := range []int{390, 1440} {
		for _, theme := range []string{"light", "dark"} {
			t.Run(fmt.Sprintf("%d/%s", width, theme), func(t *testing.T) {
				page.MustSetViewport(width, 900, 1, width == 390)
				page.MustEval(`theme=>{document.documentElement.dataset.theme=theme;document.body.dataset.theme=theme;}`, theme)
				page.MustEval(`()=>new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r)))`)
				if page.MustEval(`()=>document.documentElement.scrollWidth>innerWidth+1 || document.getElementById('content').scrollWidth>document.getElementById('content').clientWidth+1`).Bool() {
					t.Fatal("settings overflow")
				}
				if !page.MustEval(`()=>[...document.querySelectorAll('[data-decisions-settings] button,[data-decisions-settings] [role="switch"]')].every(e=>e.getAttribute('aria-label')||e.textContent.trim())`).Bool() {
					t.Fatal("unlabelled controls")
				}
				if dir := os.Getenv("AURAGO_DECISIONS_SCREENSHOTS"); dir != "" {
					page.MustScreenshot(filepath.Join(dir, fmt.Sprintf("decisions-settings-%d-%s.png", width, theme)))
				}
			})
		}
	}
	// A late poll response must never repaint a different settings section.
	page.MustEval(`async()=>{window.holdDecisionStatus=true;await selectSection('decisions',{scrollBehavior:'auto'});}`)
	waitForJSBool(t, page, `()=>typeof resolveDecisionStatus==='function'`)
	page.MustEval(`async()=>{await selectSection('overview',{scrollBehavior:'auto'});resolveDecisionStatus(new Response(JSON.stringify(decisionStatus)));await Promise.resolve();}`)
	if page.MustEval(`()=>!!document.querySelector('[data-decisions-settings]')`).Bool() {
		t.Fatal("late status replaced newer section")
	}
}
