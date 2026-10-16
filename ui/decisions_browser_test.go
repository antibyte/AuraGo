package ui

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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
			if (strings.HasPrefix(key, "config.decisions.") || strings.HasPrefix(key, "config.providers.usage_") || key == "config.providers.decisions_only_unavailable" || key == "config.providers.not_available_here" || key == "config.telephone_agent.provider_unavailable") && strings.TrimSpace(translated[key]) == "" {
				t.Errorf("%s missing %s", locale, key)
			}
		}
		var desktop map[string]string
		if err := json.Unmarshal(mustReadUIFile(t, "lang/desktop/"+locale+".json"), &desktop); err != nil {
			t.Fatal(err)
		}
		for _, key := range []string{"desktop.detective_provider_unavailable", "game_maker.provider_unavailable", "desktop.looper_provider_unavailable"} {
			if strings.TrimSpace(desktop[key]) == "" {
				t.Errorf("%s missing %s", locale, key)
			}
		}
		var invasion map[string]string
		if err := json.Unmarshal(mustReadUIFile(t, "lang/invasion/"+locale+".json"), &invasion); err != nil {
			t.Fatal(err)
		}
		if strings.TrimSpace(invasion["invasion.provider_unavailable"]) == "" {
			t.Errorf("%s missing invasion.provider_unavailable", locale)
		}
	}
}

func TestDecisionsProviderConfigBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	browser := newSmokeBrowser(t)
	page := browser.MustPage(configRefreshFixtureOrigin(t, "en", false) + "/config#overview").Timeout(60 * time.Second)
	defer page.MustClose()
	page.MustSetViewport(1440, 900, 1, false)
	waitForJSBool(t, page, `()=>!!document.querySelector('.pw-overview-card')`)
	page.MustEval(`()=>{
		window.decisionStatus={enabled:true,configured:true,ready:false,state:'configured',backend:'provider',deployment:'provider',provider:'fixture-decisions',model:'microsoft/microsoft-decision-1',last_success:'',last_error:''};
		window.decisionCatalogCalls=0;window.decisionTestCalls=0;window.holdDecisionCatalog=false;
		const previous=window.fetch;
		window.fetch=async(url,options)=>{
			const path=new URL(typeof url==='string'?url:url.url,location.origin).pathname;
			if(path==='/api/decisions/status')return new Response(JSON.stringify(window.decisionStatus));
			if(path==='/api/decisions/models'){
				window.decisionCatalogCalls++;
				if(window.holdDecisionCatalog)return new Promise(resolve=>window.resolveDecisionCatalog=resolve);
				return new Response(JSON.stringify({models:[
					{id:'microsoft/microsoft-decision-1',name:'Microsoft Decision 1',input_modalities:['text'],context_length:65536,pricing:{prompt:'0.000001',completion:'0.000004'},supports_decisions:true,supports_images:false},
					{id:'typesafe/jev-1.13',name:'Jev 1.13',input_modalities:['text'],context_length:32768,pricing:{prompt:'0',completion:'0'},supports_decisions:true,supports_images:false}
				]}));
			}
			if(path==='/api/decisions/test'&&options?.method==='POST'){
				window.decisionTestCalls++;
				if(window.decisionTestError)return new Response(JSON.stringify({code:window.decisionTestError}),{status:window.decisionTestStatus||401});
				window.decisionStatus={...window.decisionStatus,ready:true,state:'ready',last_success:'2026-10-13T12:00:00Z',last_error:''};
				return new Response(JSON.stringify({status:'ok'}));
			}
			return previous(url,options);
		};
		providersCache.push(
			{id:'fixture-openrouter-general',name:'General OpenRouter',type:'openrouter',usage:'general',base_url:'https://openrouter.ai/api/v1',model:'chat-model',has_api_key:true},
			{id:'fixture-decisions',name:'Decisions OpenRouter',type:'openrouter',usage:'decisions',base_url:'https://openrouter.ai/api/v1',model:'',has_api_key:true}
		);
	}`)
	page.MustElement(`.sidebar-group:has([data-section="decisions"]) .sidebar-group-header`).MustClick()
	page.MustElement(`[data-section="decisions"]`).MustClick()
	waitForJSBool(t, page, `()=>!!document.querySelector('[data-decisions-settings]')`)
	if !page.MustEval(`()=>document.querySelector('[data-decisions-mode="local"]')?.hidden===false && decisionCatalogCalls===0 && decisionTestCalls===0 && !configGeneralProviders().some(p=>p.id==='fixture-decisions') && configPreservedProviderOptionHTML('fixture-decisions',configGeneralProviders()).includes('Decisions-only profile')`).Bool() {
		t.Fatal("local rendering contacted OpenRouter or the shared provider filter did not preserve dedicated profiles")
	}
	page.MustEval(`()=>{
		document.querySelector('[data-path="decisions.enabled"]').click();
		const backend=document.querySelector('[data-path="decisions.backend"]');backend.value='provider';backend.dispatchEvent(new Event('change',{bubbles:true}));
		const provider=document.querySelector('[data-path="decisions.provider"]');provider.value='fixture-decisions';provider.dispatchEvent(new Event('change',{bubbles:true}));
	}`)
	if !page.MustEval(`()=>document.querySelector('[data-decisions-mode="local"]')?.hidden===true && document.querySelector('[data-decisions-mode="provider"]')?.hidden===false && document.querySelector('[data-decisions-action="install"]')?.hidden===true && document.querySelector('[data-decisions-action="test-provider"]')?.disabled===true && decisionCatalogCalls===0 && decisionTestCalls===0`).Bool() {
		t.Fatal("provider mode exposed local lifecycle actions, skipped the save-first gate, or made an automatic provider request")
	}
	if !page.MustEval(`()=>AuraConfigState.isDirty() && document.querySelector('[data-decisions-create-provider]')?.disabled===false`).Bool() {
		t.Fatal("creating an OpenRouter profile must remain available while the Decisions setup draft is incomplete")
	}
	page.MustEval(`()=>document.querySelector('[data-decisions-create-provider]').click()`)
	waitForJSBool(t, page, `()=>!!document.querySelector('#provider-modal-overlay')`)
	if !page.MustEval(`()=>{
		const modal=document.querySelector('#provider-modal-overlay');
		const tab=key=>modal.querySelector('[data-prov-tab="'+key+'"]');
		return document.querySelector('#prov-usage')?.value==='decisions' && document.querySelector('#prov-type')?.value==='openrouter' && document.querySelector('#prov-type')?.disabled===true && document.querySelector('#prov-url')?.value==='https://openrouter.ai/api/v1' && document.querySelector('#prov-url')?.disabled===true && document.querySelector('#prov-model')?.value==='' && ['model','capabilities','pricing'].every(key=>tab(key)?.classList.contains('is-hidden'));
	}`).Bool() {
		t.Fatal("Create Decisions profile did not open a canonical OpenRouter-only form without chat model fields")
	}
	page.MustElement(`#provider-modal-close-btn`).MustClick()
	if !page.MustEval(`()=>!document.querySelector('#provider-modal-overlay') && AuraConfigState.isDirty()`).Bool() {
		t.Fatal("closing the provider form lost the incomplete Decisions setup draft")
	}
	if !page.MustEval(`()=>document.querySelector('[data-decisions-action="test-provider"]')?.disabled===true`).Bool() {
		t.Fatal("paid provider test enabled before selecting a model")
	}
	// Catalog reads are explicit and may use the selected saved provider while
	// the Decisions configuration itself is still an unsaved draft.
	page.MustEval(`()=>{window.holdDecisionCatalog=true;document.querySelector('[data-decisions-load-models]').click()}`)
	waitForJSBool(t, page, `()=>typeof resolveDecisionCatalog==='function'`)
	page.MustEval(`()=>{const provider=document.querySelector('[data-path="decisions.provider"]');provider.value='fixture-openrouter-general';provider.dispatchEvent(new Event('change',{bubbles:true}));window.holdDecisionCatalog=false;resolveDecisionCatalog(new Response(JSON.stringify({models:[{id:'stale/model',name:'stale'}]})));}`)
	if !page.MustEval(`()=>document.querySelector('[data-path="decisions.model"]').options.length===1 && decisionCatalogCalls===1`).Bool() {
		t.Fatal("a late catalog response populated models for a provider that is no longer selected")
	}
	page.MustEval(`()=>{const provider=document.querySelector('[data-path="decisions.provider"]');provider.value='fixture-decisions';provider.dispatchEvent(new Event('change',{bubbles:true}));document.querySelector('[data-decisions-load-models]').click()}`)
	waitForJSBool(t, page, `()=>document.querySelector('[data-decisions-models]')?.textContent.includes('Loaded 2 models')`)
	if !page.MustEval(`()=>decisionCatalogCalls===2 && AuraConfigState.isDirty()===true`).Bool() {
		t.Fatal("explicit model catalog action changed the unsaved Decisions draft or failed to load")
	}
	if !page.MustEval(`()=>document.querySelector('[data-decisions-action="test-provider"]')?.disabled===true`).Bool() {
		t.Fatal("paid provider test enabled before selecting a model")
	}
	page.MustEval(`()=>{const model=document.querySelector('[data-path="decisions.model"]');model.value='microsoft/microsoft-decision-1';model.dispatchEvent(new Event('change',{bubbles:true}));}`)
	if !page.MustEval(`()=>document.querySelector('[data-decisions-action="test-provider"]')?.disabled===true && decisionTestCalls===0 && document.querySelector('[data-decisions-model-details]')?.textContent.includes((65536).toLocaleString()) && document.querySelector('[data-decisions-model-details]')?.textContent.includes('USD per token')`).Bool() {
		t.Fatal("paid provider test was enabled while the selected model was still an unsaved draft")
	}
	page.MustEval(`async()=>{await saveConfig();}`)
	waitForJSBool(t, page, `()=>AuraConfigState.isDirty()===false && document.querySelector('[data-decisions-action="test-provider"]')?.disabled===false`)
	page.MustEval(`()=>{const button=document.querySelector('[data-decisions-action="test-provider"]');button.click();button.click()}`)
	waitForJSBool(t, page, `()=>document.querySelector('#shared-modal-overlay')?.classList.contains('active')`)
	if !page.MustEval(`()=>document.querySelector('#shared-modal-message')?.textContent.includes('fixed Decisions sample request') && document.querySelector('#shared-modal-message')?.textContent.includes('Charges may apply') && decisionTestCalls===0`).Bool() {
		t.Fatal("paid provider confirmation did not describe the fixed external sample or fired before confirmation")
	}
	page.MustElement(`#shared-modal-confirm`).MustClick()
	waitForJSBool(t, page, `()=>decisionTestCalls===1 && document.querySelector('[data-decisions-provider-result]')?.textContent.includes('Decision test completed')`)
	if !page.MustEval(`()=>document.querySelector('[data-decisions-status]')?.textContent.includes('Provider evaluation succeeded') && document.querySelector('[data-decisions-settings]')?.textContent.includes('Last successful evaluation:')`).Bool() {
		t.Fatal("successful provider test did not update readiness and last-success status")
	}
	for n, check := range []struct {
		code, message string
		status        int
	}{
		{code: "provider_auth_failed", message: "rejected authentication", status: 401},
		{code: "provider_credit_exhausted", message: "no credits available", status: 402},
		{code: "provider_rate_limited", message: "rate limit was reached", status: 429},
	} {
		page.MustEval(`check=>{window.decisionTestError=check.code;window.decisionTestStatus=check.status;document.querySelector('[data-decisions-action="test-provider"]').click()}`, map[string]any{"code": check.code, "status": check.status})
		waitForJSBool(t, page, `()=>document.querySelector('#shared-modal-overlay')?.classList.contains('active')`)
		page.MustElement(`#shared-modal-confirm`).MustClick()
		waitForJSBool(t, page, fmt.Sprintf(`()=>decisionTestCalls===%d && document.querySelector('[data-decisions-provider-result]')?.textContent.includes(%q)`, n+2, check.message))
	}
	page.MustEval(`()=>{window.decisionTestError='';window.decisionTestStatus=0;}`)
	page.MustEval(`()=>{window.decisionStatus={...window.decisionStatus,ready:false,state:'error',last_error:'external_outcome_unknown'};document.dispatchEvent(new Event('aurago:config-saved'));}`)
	waitForJSBool(t, page, `()=>document.querySelector('[data-decisions-status]')?.textContent.includes('request outcome is unknown')`)
	page.MustEval(`()=>document.querySelector('[data-decisions-action="test-provider"]').click()`)
	waitForJSBool(t, page, `()=>document.querySelector('#shared-modal-overlay')?.classList.contains('active')`)
	if !page.MustEval(`()=>document.querySelector('#shared-modal-message')?.textContent.includes('previous request may have reached the provider') && document.querySelector('#shared-modal-message')?.textContent.includes('Check usage')`).Bool() {
		t.Fatal("uncertain provider outcome did not show a charge warning before retry")
	}
	page.MustElement(`#shared-modal-cancel`).MustClick()
	if page.MustEval(`()=>decisionTestCalls`).Int() != 4 {
		t.Fatal("cancelling retry confirmation sent another provider request")
	}
	for _, width := range []int{390, 1440} {
		for _, theme := range []string{"standard", "fruity"} {
			page.MustSetViewport(width, 900, 1, width == 390)
			page.MustEval(`theme=>{document.documentElement.dataset.theme=theme;document.body.dataset.theme=theme;}`, theme)
			page.MustEval(`()=>new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r)))`)
			if page.MustEval(`()=>document.documentElement.scrollWidth>innerWidth+1 || document.getElementById('content').scrollWidth>document.getElementById('content').clientWidth+1`).Bool() {
				t.Fatalf("provider Decisions settings overflow at %dpx in %s theme", width, theme)
			}
			if dir := os.Getenv("AURAGO_DECISIONS_SCREENSHOTS"); dir != "" {
				if err := os.MkdirAll(dir, 0o755); err != nil {
					t.Fatal(err)
				}
				size := "narrow"
				if width == 1440 {
					size = "wide"
				}
				page.MustScreenshot(filepath.Join(dir, "decisions-provider-"+theme+"-"+size+".png"))
			}
		}
	}
}

func TestDecisionsLifecycleFeedbackBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	browser := newSmokeBrowser(t)
	page := browser.MustPage(configRefreshFixtureOrigin(t, "de", false) + "/config#overview").Timeout(60 * time.Second)
	defer page.MustClose()
	page.MustSetViewport(1440, 900, 1, false)
	waitForJSBool(t, page, `()=>!!document.querySelector('.pw-overview-card')`)
	page.MustEval(`()=>{
		window.decisionStatus={enabled:true,installed:true,ready:false,state:'installed',deployment:'native'};
		const previous=window.fetch;
		window.fetch=async(url,options)=>{
			if(url==='/api/decisions/status') return new Response(JSON.stringify(decisionStatus));
			if(url==='/api/decisions/check') return new Response(JSON.stringify({runtime_available:true,deployment:'native',target:'linux-amd64'}));
			if(url==='/api/decisions/install'||url==='/api/decisions/start') {
				window.decisionStatus={...decisionStatus,state:url.endsWith('install')?'installing':'starting',code:'',progress:{phase:url.endsWith('install')?'verifying_runtime':'loading_model'}};
				if(window.holdDecisionStart) return new Promise(resolve=>window.resolveDecisionStart=resolve);
				return new Response('{"status":"accepted"}',{status:202});
			}
			if(url==='/api/decisions/stop') {
				window.decisionStatus={...decisionStatus,state:'stopped',code:'',ready:false};
				return new Response('{}');
			}
			return previous(url,options);
		};
	}`)
	page.MustElement(`.sidebar-group:has([data-section="decisions"]) .sidebar-group-header`).MustClick()
	page.MustElement(`[data-section="decisions"]`).MustClick()
	waitForJSBool(t, page, `()=>document.querySelector('[data-decisions-status]')?.textContent.includes('Modell heruntergeladen und geprüft')`)
	page.MustEval(`async()=>{document.querySelector('[data-path="decisions.enabled"]').click();await saveConfig();}`)
	page.MustEval(`()=>document.querySelector('[data-decisions-action="check"]').click()`)
	waitForJSBool(t, page, `()=>document.querySelector('[data-decisions-action="install"]')?.disabled===false`)
	if !page.MustEval(`()=>document.querySelector('[data-decisions-settings] a[href*="decisions_runtime/README.md"]')?.textContent.includes('Anleitung') && document.querySelector('[data-decisions-check]').textContent.includes('linux-amd64')`).Bool() {
		t.Fatal("runtime prerequisites are not discoverable before installation")
	}
	page.MustEval(`()=>document.querySelector('[data-decisions-action="install"]').click()`)
	waitForJSBool(t, page, `()=>document.querySelector('[data-decisions-result]')?.textContent.includes('Hintergrund') && document.querySelector('[data-decisions-progress]')?.textContent.includes('Laufzeitdateien')`)
	page.MustEval(`()=>{decisionStatus.progress={phase:'verifying_model',total_bytes:100,current_bytes:40,percent:40};document.dispatchEvent(new Event('aurago:config-saved'));}`)
	waitForJSBool(t, page, `()=>document.querySelector('[data-decisions-settings] progress')?.value===40 && document.querySelector('[data-decisions-progress]')?.textContent.includes('Modellintegrität')`)
	// Let the real poll finish the accepted action without re-rendering the section.
	page.MustEval(`async()=>{decisionStatus={...decisionStatus,state:'installed',progress:null};await new Promise(resolve=>setTimeout(resolve,5500));}`)
	waitForJSBool(t, page, `()=>document.querySelector('[data-decisions-result]')?.textContent.includes('Modell heruntergeladen und geprüft') && document.querySelector('[data-decisions-status]')?.textContent.includes('Modell heruntergeladen und geprüft')`)
	if !page.MustEval(`()=>document.querySelector('[data-decisions-action="start"]').disabled===false && document.querySelector('[data-decisions-action="test"]').disabled===true`).Bool() {
		t.Fatal("installed was confused with ready")
	}
	page.MustEval(`()=>document.querySelector('[data-decisions-action="start"]').click()`)
	waitForJSBool(t, page, `()=>document.querySelector('[data-decisions-progress]')?.textContent.includes('Modell wird geladen')`)
	if !page.MustEval(`()=>{const p=document.querySelector('[data-decisions-settings] progress');return !p.hidden && !p.hasAttribute('value') && !document.querySelector('[data-decisions-action="stop"]').disabled;}`).Bool() {
		t.Fatal("startup without byte progress must be indeterminate and cancellable")
	}
	page.MustEval(`()=>{decisionStatus.progress={phase:'warming_up'};document.dispatchEvent(new Event('aurago:config-saved'));}`)
	waitForJSBool(t, page, `()=>document.querySelector('[data-decisions-progress]')?.textContent.includes('Erster Lauf')`)
	for _, width := range []int{390, 1440} {
		for _, theme := range []string{"standard", "fruity"} {
			page.MustSetViewport(width, 900, 1, width == 390)
			page.MustEval(`theme=>{document.documentElement.dataset.theme=theme;document.body.dataset.theme=theme;}`, theme)
			page.MustEval(`()=>document.querySelector('[data-decisions-status]').scrollIntoView({block:'center'})`)
			page.MustEval(`()=>new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r)))`)
			if page.MustEval(`()=>document.documentElement.scrollWidth>innerWidth+1 || document.getElementById('content').scrollWidth>document.getElementById('content').clientWidth+1`).Bool() {
				t.Fatalf("startup feedback overflows at %dpx in %s", width, theme)
			}
			if dir := os.Getenv("AURAGO_DECISIONS_SCREENSHOTS"); dir != "" {
				if err := os.MkdirAll(dir, 0o755); err != nil {
					t.Fatal(err)
				}
				page.MustScreenshot(filepath.Join(dir, fmt.Sprintf("decisions-startup-%d-%s.png", width, theme)))
			}
		}
	}
	page.MustEval(`()=>{decisionStatus={...decisionStatus,state:'error',code:'runtime_start_timeout'};document.dispatchEvent(new Event('aurago:config-saved'));}`)
	waitForJSBool(t, page, `()=>document.querySelector('[data-decisions-result]')?.textContent.includes('fünf Minuten') && document.querySelector('[data-decisions-settings] code')?.textContent.includes('runtime_start_timeout')`)
	page.MustEval(`()=>document.querySelector('[data-decisions-action="start"]').click()`)
	waitForJSBool(t, page, `()=>document.querySelector('[data-decisions-result]')?.textContent.includes('Hintergrund')`)
	page.MustEval(`()=>{decisionStatus={...decisionStatus,state:'ready',ready:true};document.dispatchEvent(new Event('aurago:config-saved'));}`)
	waitForJSBool(t, page, `()=>document.querySelector('[data-decisions-action="test"]')?.disabled===false && document.querySelector('[data-decisions-result]')?.textContent===document.querySelector('[data-decisions-status]')?.textContent`)
	if !page.MustEval(`()=>document.querySelector('[data-decisions-result]').hidden && document.querySelector('[data-decisions-action="start"]').disabled`).Bool() {
		t.Fatal("ready state duplicated feedback or offered another start")
	}
	// Stop also interrupts the initial POST, and a late 202 cannot revive it.
	page.MustEval(`()=>{decisionStatus={...decisionStatus,state:'stopped',ready:false};document.dispatchEvent(new Event('aurago:config-saved'));}`)
	waitForJSBool(t, page, `()=>!document.querySelector('[data-decisions-action="start"]').disabled`)
	page.MustEval(`()=>{window.holdDecisionStart=true;document.querySelector('[data-decisions-action="start"]').click();}`)
	waitForJSBool(t, page, `()=>typeof resolveDecisionStart==='function'`)
	if page.MustEval(`()=>document.querySelector('[data-decisions-action="stop"]').disabled`).Bool() {
		t.Fatal("Stop disabled during startup POST")
	}
	page.MustEval(`()=>document.querySelector('[data-decisions-action="stop"]').click()`)
	waitForJSBool(t, page, `()=>document.querySelector('[data-decisions-result]')?.textContent.includes('Gestoppt')`)
	page.MustEval(`async()=>{resolveDecisionStart(new Response('{"status":"accepted"}',{status:202}));await new Promise(r=>setTimeout(r,0));}`)
	if !page.MustEval(`()=>document.querySelector('[data-decisions-result]').textContent.includes('Gestoppt')`).Bool() {
		t.Fatal("late startup response replaced Stop")
	}
}

func TestDecisionsConfigBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	browser := newSmokeBrowser(t)
	page := browser.MustPage(configRefreshFixtureOrigin(t, "de", false) + "/config#overview").Timeout(60 * time.Second)
	defer page.MustClose()
	page.MustSetViewport(1440, 900, 1, false)
	waitForJSBool(t, page, `()=>!!document.querySelector('.pw-overview-card')`)
	page.MustEval(`async()=>{
        window.decisionActions=[];window.holdDecisionTest=false;window.holdDecisionStop=false;window.decisionStatus={enabled:false,installed:false,ready:false,state:'disabled',deployment:'native',code:'service_disabled'};
        const previous=window.fetch;
        window.fetch=async(url,options)=>{
            if(url==='/api/decisions/status') {
                if(window.holdDecisionStatus) return new Promise(resolve=>window.resolveDecisionStatus=resolve);
                return new Response(JSON.stringify(window.decisionStatus));
            }
            if(url==='/api/decisions/test'&&window.holdDecisionTest) return new Promise(resolve=>window.resolveDecisionTest=resolve);
            if(url.startsWith('/api/decisions/')) {
                window.decisionActions.push(url);
                if(url==='/api/decisions/stop') {
                    window.decisionStatus={enabled:true,installed:true,ready:false,state:'stopped',deployment:'native'};
                    if(window.holdDecisionStop) return new Promise(resolve=>window.resolveDecisionStop=resolve);
                }
                return new Response('{}');
            }
            return previous(url,options);
        };
    }`)
	page.MustElement(`.sidebar-group:has([data-section="decisions"]) .sidebar-group-header`).MustClick()
	page.MustElement(`[data-section="decisions"]`).MustClick()
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
	if page.MustEval(`()=>document.querySelector('[data-decisions-action="start"]').disabled`).Bool() {
		t.Fatal("failed autostart must remain retryable when Installed is not yet true")
	}
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
	// Stop must interrupt a pending test, and its late response must not replace Stop's result.
	page.MustEval(`()=>{decisionActions=[];window.holdDecisionTest=true;window.holdDecisionStop=true;document.querySelector('[data-decisions-action="test"]').click();}`)
	waitForJSBool(t, page, `()=>typeof resolveDecisionTest==='function'`)
	if page.MustEval(`()=>document.querySelector('[data-decisions-action="stop"]').disabled`).Bool() {
		t.Fatal("Stop was disabled while a test was running")
	}
	page.MustEval(`()=>document.querySelector('[data-decisions-action="stop"]').click()`)
	waitForJSBool(t, page, `()=>typeof resolveDecisionStop==='function'`)
	page.MustEval(`()=>document.querySelector('[data-decisions-action="stop"]').click()`)
	if !page.MustEval(`()=>document.querySelector('[data-decisions-action="stop"]').disabled && decisionActions.filter(url=>url==='/api/decisions/stop').length===1`).Bool() {
		t.Fatal("pending Stop was not disabled or issued a duplicate request")
	}
	page.MustEval(`async()=>{window.holdDecisionTest=false;resolveDecisionTest(new Response('{}'));await new Promise(resolve=>setTimeout(resolve,30));}`)
	if !page.MustEval(`()=>document.querySelector('[data-decisions-action="stop"]').disabled && document.querySelector('[data-decisions-result]')?.textContent.includes('Wird ausgeführt') && decisionActions.filter(url=>url==='/api/decisions/stop').length===1`).Bool() {
		t.Fatal("late test response changed or unlocked a pending Stop")
	}
	page.MustEval(`()=>{window.holdDecisionStop=false;resolveDecisionStop(new Response('{}'));}`)
	waitForJSBool(t, page, `()=>document.querySelector('[data-decisions-action="stop"]').disabled && document.querySelector('[data-decisions-result]')?.textContent.includes('Gestoppt')`)
	if page.MustEval(`()=>document.querySelector('[data-decisions-result]')?.textContent.includes('Entscheidungstest abgeschlossen')`).Bool() {
		t.Fatal("late test response overwrote the completed Stop result")
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
