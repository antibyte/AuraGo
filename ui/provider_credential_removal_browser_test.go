package ui

import (
	"testing"
	"time"
)

func TestProviderClearKeySurvivesAuthTypeChangeBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	page := newSmokeBrowser(t).MustPage(configRefreshFixtureOrigin(t, "en", false) + "/config#overview").Timeout(45 * time.Second)
	defer page.MustClose()
	waitForJSBool(t, page, `()=>!!document.querySelector('.pw-overview-card')`)
	page.MustEval(`()=>{
  window.providerReview={puts:[],revision:'"review-r1"',providers:[{id:'review-main',name:'Review',type:'openai',base_url:'http://fixture.local/v1',model:'review-model',api_key:'••••••••',auth_type:'api_key',models:[],capabilities:{auto:false,tool_calling:true}}]};
  const state=window.providerReview, original=window.fetch.bind(window);
  window.showConfirm=async()=>true;
  window.fetch=async(input,options={})=>{
   const path=new URL(typeof input==='string'?input:input.url,location.origin).pathname;
   if(path==='/api/providers') {
    if(options.method==='PUT') {state.puts.push(JSON.parse(options.body));return new Response(JSON.stringify({status:'ok',revision:'"review-r2"'}),{headers:{ETag:'"review-r2"'}})}
    return new Response(JSON.stringify(state.providers),{headers:{ETag:state.revision}});
   }
   if(path==='/api/providers/status')return new Response(JSON.stringify({revision:state.revision,missing_references:[],providers:[]}));
   if(path==='/api/providers/capabilities')return new Response(JSON.stringify({auto:false,source:'manual'}));
   return original(input,options);
  };
 }`)
	page.MustEval(`async()=>{await selectSection('providers',{scrollBehavior:'auto'});await loadProviders();providerEdit(0);}`)
	waitForJSBool(t, page, `()=>!!document.getElementById('prov-clear-api-key')`)
	page.MustEval(`()=>{document.getElementById('prov-clear-api-key').click();const auth=document.getElementById('prov-auth-type');auth.value='oauth2';auth.dispatchEvent(new Event('change',{bubbles:true}));document.getElementById('prov-save-btn').click();}`)
	waitForJSBool(t, page, `()=>window.providerReview.puts.length===1`)
	if got := page.MustEval(`()=>window.providerReview.puts[0][0].api_key`).Str(); got != "" {
		t.Fatal("explicit API key removal was discarded after switching to OAuth")
	}
}
