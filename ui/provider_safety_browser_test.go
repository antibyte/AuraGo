package ui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestProviderRevisionAndSavedTestsBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	page := newSmokeBrowser(t).MustPage(configRefreshFixtureOrigin(t, "en", false) + "/config#overview").Timeout(45 * time.Second)
	defer page.MustClose()
	waitForJSBool(t, page, `()=>!!document.querySelector('.pw-overview-card')`)
	page.MustEval(`() => {
  window.providerFixture={
   revision:'"fixture-r1"', providers:[{id:'fixture-main',name:'Saved profile',type:'openai',base_url:'http://custom.local/v1',model:'fixture-model',api_key:'••••••••',auth_type:'api_key',models:[],capabilities:{auto:false,tool_calling:true}}],
   puts:[],tests:[],confirmations:[],busy:false,conflict:false
  };
  const state=window.providerFixture;
  window.showConfirm=async(title,message)=>{state.confirmations.push({title,message});return true};
  const original=window.fetch.bind(window);
  window.fetch=async(input,options={})=>{
   const path=new URL(typeof input==='string'?input:input.url,location.origin).pathname;
   if(path==='/api/providers'){
    if(options.method==='PUT'){
     state.puts.push({headers:options.headers,body:JSON.parse(options.body)});
     if(state.busy) await new Promise(resolve=>state.releaseSave=resolve);
     if(state.conflict){state.conflict=false;return new Response(JSON.stringify({code:'provider_revision_conflict'}),{status:412})}
     state.providers=JSON.parse(options.body);state.revision='"fixture-r'+(state.puts.length+1)+'"';
     return new Response(JSON.stringify({status:'ok',revision:state.revision}),{headers:{ETag:state.revision}});
    }
    return new Response(JSON.stringify(state.providers),{headers:{ETag:state.revision}});
   }
   if(path==='/api/providers/status')return new Response(JSON.stringify({revision:state.revision,missing_references:[],providers:state.providers.map(p=>({provider_id:p.id,errors:[],active:true,oauth:{authorized:false},health:{},metadata_source:'model_only_fallback'}))}));
   if(path==='/api/providers/test'){
    const request=JSON.parse(options.body);state.tests.push(request);
    return new Response(JSON.stringify({status:'ok',live_status:'ok',code:request.probe_kind==='generation'?'model_test_ok':'metadata_probe_ok',revision:state.revision,current:true}));
   }
   if(path==='/api/providers/capabilities')return new Response(JSON.stringify({auto:false,tool_calling:true,source:'manual'}));
   return original(input,options);
  };
 }`)
	page.MustEval(`async()=>{await selectSection('providers',{scrollBehavior:'auto'});await loadProviders();providerRenderCards();providerEdit(0);}`)
	waitForJSBool(t, page, `()=>!!document.getElementById('prov-test-metadata')`)
	page.MustEval(`()=>document.querySelector('[data-prov-tab="diagnostics"]').click()`)
	waitForJSBool(t, page, `()=>!document.getElementById('prov-test-metadata').disabled`)
	page.MustEval(`()=>document.getElementById('prov-test-metadata').click()`)
	waitForJSBool(t, page, `()=>window.providerFixture.tests.length===1&&document.getElementById('prov-test-result').textContent.includes('Metadata')`)
	page.MustEval(`()=>{const input=document.getElementById('prov-name');input.value='Unsaved draft';input.dispatchEvent(new Event('input',{bubbles:true}));}`)
	// The tests run with the form as typed in: a dirty form keeps them enabled and is sent as the draft.
	if page.MustEval(`()=>document.getElementById('prov-test-generation').disabled`).Bool() {
		t.Fatal("an unsaved form blocked the draft test")
	}
	page.MustEval(`()=>document.getElementById('prov-test-generation').click()`)
	waitForJSBool(t, page, `()=>window.providerFixture.tests.length===2&&document.getElementById('prov-test-generation').dataset.configActionBusy==='false'`)
	if !page.MustEval(`()=>{const test=window.providerFixture.tests[1];return window.providerFixture.confirmations.some(entry=>entry.message.includes('charge'))&&window.providerFixture.tests.every(entry=>Object.keys(entry).sort().join(',')==='probe_kind,provider,provider_id')&&test.provider.name==='Unsaved draft'&&test.provider.base_url==='http://custom.local/v1'&&test.provider.api_key==='••••••••'&&test.provider.type==='openai'}`).Bool() {
		t.Fatal("tests did not require explicit generation approval or did not send the typed form")
	}
	// A typed key is sent as is; clearing the stored key cannot be tested before saving.
	page.MustEval(`()=>{const key=document.getElementById('prov-key');key.value='typed-key';key.dispatchEvent(new Event('input',{bubbles:true}));document.getElementById('prov-test-metadata').click();}`)
	waitForJSBool(t, page, `()=>window.providerFixture.tests.length===3&&window.providerFixture.tests[2].provider.api_key==='typed-key'`)
	page.MustEval(`()=>{const key=document.getElementById('prov-key');key.value='';key.dispatchEvent(new Event('input',{bubbles:true}));const input=document.getElementById('prov-name');input.value='Saved profile';input.dispatchEvent(new Event('input',{bubbles:true}));}`)
	if dir := os.Getenv("AURAGO_BROWSER_ARTIFACT_DIR"); dir != "" {
		_ = os.MkdirAll(dir, 0o755)
		for _, width := range []int{1440, 390} {
			page.MustSetViewport(width, 900, 1, false)
			name := "provider-status-desktop.png"
			if width == 390 {
				name = "provider-status-mobile.png"
			}
			if err := os.WriteFile(filepath.Join(dir, name), page.MustScreenshot(), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		page.MustSetViewport(1440, 1000, 1, false)
	}
	page.MustEval(`()=>{window.providerFixture.budgetBefore=JSON.stringify(configData.budget);updateProviderModelCost({id:'discarded-model',inputPerMillion:9,outputPerMillion:11});document.getElementById('provider-modal-cancel-btn').click();}`)
	waitForJSBool(t, page, `()=>!document.getElementById('provider-modal-overlay')`)
	if !page.MustEval(`()=>JSON.stringify(configData.budget)===window.providerFixture.budgetBefore&&window.providerFixture.puts.length===0`).Bool() {
		t.Fatal("cancelling provider editor changed unrelated budget draft or provider")
	}
	page.MustEval(`()=>{providerEdit(0);document.getElementById('prov-name').value='Old editor';providersRevision='"background-refresh"';document.getElementById('prov-save-btn').click();}`)
	waitForJSBool(t, page, `()=>!!document.getElementById('provider-conflict-notice')`)
	if page.MustEval(`()=>window.providerFixture.puts.length`).Int() != 0 {
		t.Fatal("background revision refresh authorized an old modal draft")
	}
	page.MustEval(`()=>document.querySelector('#provider-conflict-notice button').click()`)
	waitForJSBool(t, page, `()=>!document.getElementById('provider-modal-overlay')&&!providersConflict`)
	page.MustEval(`()=>{providerEdit(0);document.getElementById('prov-clear-api-key').click();window.providerFixture.busy=true;document.getElementById('prov-save-btn').click();document.getElementById('prov-save-btn').click();}`)
	waitForJSBool(t, page, `()=>!!window.providerFixture.releaseSave`)
	if !page.MustEval(`()=>window.providerFixture.puts.length===1&&window.providerFixture.puts[0].body[0].api_key===''&&window.providerFixture.puts[0].headers['If-Match']==='"fixture-r1"'`).Bool() {
		t.Fatal("double click or explicit credential removal violated save contract")
	}
	page.MustEval(`()=>{window.providerFixture.busy=false;window.providerFixture.releaseSave();}`)
	waitForJSBool(t, page, `()=>!document.getElementById('provider-modal-overlay')`)
	page.MustEval(`()=>{providerEdit(0);const name=document.getElementById('prov-name');name.value='Conflict draft';name.dispatchEvent(new Event('input',{bubbles:true}));window.providerFixture.conflict=true;document.getElementById('prov-save-btn').click();}`)
	waitForJSBool(t, page, `()=>!!document.getElementById('provider-conflict-notice')`)
	if page.MustEval(`()=>document.getElementById('prov-name').value`).Str() != "Conflict draft" {
		t.Fatal("revision conflict discarded modal draft")
	}
	page.MustEval(`()=>document.getElementById('prov-save-btn').click()`)
	if page.MustEval(`()=>window.providerFixture.puts.length`).Int() != 2 {
		t.Fatal("conflicting draft retried without explicit reload")
	}
	page.MustEval(`()=>document.querySelector('#provider-conflict-notice button').click()`)
	waitForJSBool(t, page, `()=>!document.getElementById('provider-modal-overlay')&&!providersConflict`)
	if page.MustEval(`()=>providersCache[0].name`).Str() != "Saved profile" {
		t.Fatal("conflict overwrote saved settings")
	}
}

func TestProviderSafetyLocalesComplete(t *testing.T) {
	var baseline map[string]string
	data, err := os.ReadFile(filepath.Join("lang", "config", "providers", "en.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &baseline); err != nil {
		t.Fatal(err)
	}
	keys := []string{"save_success", "revision_conflict", "reload_saved", "remove_credential", "url_invalid", "tab_diagnostics", "diagnostics_help", "saved_test_help", "metadata_test", "generation_test", "generation_confirm", "test_metadata_ok", "test_generation_ok", "test_unsupported", "test_stale", "test_failed", "status_unknown", "runtime_active", "runtime_inactive", "oauth_connected", "oauth_expired", "status_detail", "invalid_credential", "runtime_incomplete", "limit_source_model_only_fallback", "missing_references"}
	for _, locale := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		data, err := os.ReadFile(filepath.Join("lang", "config", "providers", locale+".json"))
		if err != nil {
			t.Fatal(err)
		}
		var translated map[string]string
		if err = json.Unmarshal(data, &translated); err != nil {
			t.Fatal(err)
		}
		for _, key := range keys {
			full := "config.providers." + key
			if translated[full] == "" {
				t.Errorf("%s misses %s", locale, key)
			}
			if locale != "en" && translated[full] == baseline[full] {
				t.Errorf("%s uses English for %s", locale, key)
			}
		}
	}
}
