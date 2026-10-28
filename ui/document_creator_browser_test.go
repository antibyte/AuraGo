package ui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestDocumentCreatorLocales(t *testing.T) {
	for _, locale := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		data, err := os.ReadFile(filepath.Join("lang", "config", "document_creator", locale+".json"))
		if err != nil {
			t.Fatal(err)
		}
		var messages map[string]string
		if err := json.Unmarshal(data, &messages); err != nil {
			t.Fatal(err)
		}
		for _, key := range []string{"allow_remote_resources_label", "test_unreachable", "test_unverified", "test_verified", "restart_required", "test_saved_settings", "save_before_test"} {
			if messages["config.document_creator."+key] == "" {
				t.Errorf("%s lacks %s", locale, key)
			}
		}
		if messages["help.document_creator.allow_remote_resources"] == "" {
			t.Errorf("%s lacks policy help", locale)
		}
	}
}

func TestDocumentCreatorPolicyAndDiagnosticsBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	page := newSmokeBrowser(t).MustPage(configRefreshFixtureOrigin(t, "de", true) + "/config#overview")
	defer page.MustClose()
	waitForJSBool(t, page, `()=>!!document.querySelector('.pw-overview-card')`)
	page.MustEval(`async()=>{
configData.tools.document_creator={enabled:true,backend:'gotenberg',allow_remote_resources:false,output_dir:'data/documents',gotenberg:{url:'http://127.0.0.1:3000',timeout:120}};
AuraConfigState.init(configData);resetDirtySnapshot();await selectSection('document_creator');
window.dcTests=0;window.dcStatus={status:'success',reachable:true,security_verified:true};
const original=window.fetch;window.fetch=(url,options={})=>{
 if(String(url)==='/api/document-creator/test'){window.dcTests++;return Promise.resolve(new Response(JSON.stringify(window.dcStatus),{status:200,headers:{'Content-Type':'application/json'}}));}
 return original(url,options);
};
}`)
	waitForJSBool(t, page, `()=>!!document.querySelector('#dc-allow-remote-resources')`)
	if !page.MustEval(`()=>document.querySelector('#dc-allow-remote-resources').checked===false`).Bool() {
		t.Fatal("remote resources were enabled by default")
	}
	page.MustElement("#dc-allow-remote-resources").MustClick()
	if !page.MustEval(`()=>AuraConfigState.get('tools.document_creator.allow_remote_resources')===true&&typeof AuraConfigState.buildPatch().tools.document_creator.allow_remote_resources==='boolean'`).Bool() {
		t.Fatal("checkbox did not produce a boolean draft")
	}
	page.MustEval(`async()=>await dcTestGotenberg()`)
	if !page.MustEval(`()=>window.dcTests===0`).Bool() {
		t.Fatal("tested an unsaved grant")
	}
	page.MustEval(`()=>{AuraConfigState.discard();}`)
	if !page.MustEval(`()=>document.querySelector('#dc-allow-remote-resources').checked===false`).Bool() {
		t.Fatal("discard did not restore the checkbox")
	}
	page.MustElement("#dc-allow-remote-resources").MustClick()
	page.MustEval(`()=>{const sent=AuraConfigState.snapshot().draft;AuraConfigState.commitSent(sent,sent);resetDirtySnapshot();}`)
	if !page.MustEval(`()=>AuraConfigState.get('tools.document_creator.allow_remote_resources',{saved:true})===true&&document.querySelector('#dc-allow-remote-resources').checked===true`).Bool() {
		t.Fatal("saved checkbox was lost")
	}
	for _, state := range []struct {
		value string
		key   string
	}{
		{`{"status":"success","reachable":true,"security_verified":true}`, "test_verified"},
		{`{"status":"error","reachable":true,"security_verified":false}`, "test_unverified"},
		{`{"status":"error","reachable":false,"security_verified":false}`, "test_unreachable"},
		{`{"status":"error","reachable":true,"security_verified":false,"restart_required":true}`, "restart_required"},
	} {
		page.MustEval(`value=>{window.dcStatus=JSON.parse(value);}`, state.value)
		page.MustEval(`async()=>await dcTestGotenberg()`)
		if !page.MustEval(`key=>document.querySelector('#dc-test-result').textContent.includes(t('config.document_creator.'+key))`, state.key).Bool() {
			t.Fatalf("wrong diagnostic state: %s", state.key)
		}
	}
	for _, width := range []int{390, 1440} {
		page.MustSetViewport(width, 1000, 1, width == 390)
		for _, theme := range []string{"dark", "light"} {
			page.MustEval(`async theme=>{document.documentElement.dataset.theme=theme;document.body.dataset.theme=theme;await new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve)));}`, theme)
			if !page.MustEval(`()=>{const c=document.getElementById('content');return c.scrollWidth<=c.clientWidth+1;}`).Bool() {
				t.Fatalf("form overflow at %d/%s", width, theme)
			}
		}
	}
}
