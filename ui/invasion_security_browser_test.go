package ui

import (
	"regexp"
	"testing"
)

func TestInvasionPermissionRevocationBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	page := newSmokeBrowser(t).MustPage(newPrecisionSmokeOrigin(t))
	defer page.MustClose()
	html := regexp.MustCompile(`(?s)<script\b[^>]*>.*?</script>`).ReplaceAllString(normalizeAssetText(mustReadUIFile(t, "invasion_control.html")), "")
	page.MustSetDocumentContent(html)
	page.MustEval(`() => {
  window.t = key=>key; window.showToast = message=>{window.lastToast=message;};
  window.openModal = ()=>{}; window.closeModal = ()=>{};
  window.fetch = async (url,options={}) => {
   if(url.includes('config-history')) return {ok:true,json:async()=>[{id:'rev',nest_id:'nest',egg_id:window.otherEgg?'old-egg':'egg',permissions_json:JSON.stringify({allowed_tools:['shell','python'],allow_filesystem_write:true,allow_network_requests:true,allow_unsandboxed_shell:true,allow_remote_shell:true,allow_self_update:true})}]};
   if(options.method==='POST'){window.sent=JSON.parse(options.body);return {ok:!window.fail,json:async()=>window.fail?{error:'offline'}:{status:'applied'}};}
   return {ok:true,json:async()=>[]};
  };
 }`)
	if err := page.AddScriptTag("", normalizeAssetText(mustReadUIFile(t, "js/invasion/main.js"))); err != nil {
		t.Fatal(err)
	}
	page.MustEval(`async()=>{nestsData=[{id:'nest',egg_id:'egg',name:'fixture',applied_config_rev:'rev'}];await openReconfigureModal('nest');}`)
	if !page.MustEval(`()=>document.getElementById('reconfigure-allow-unsandboxed').checked && document.getElementById('reconfigure-allowed-tools').value.includes('shell')`).Bool() {
		t.Fatal("applied permissions were not loaded")
	}
	page.MustEval(`async()=>{
  document.getElementById('reconfigure-allowed-tools').value='[]';
  document.querySelectorAll('#reconfigure-modal input[type=checkbox]').forEach(el=>el.checked=false);
  window.fail=true;await applyReconfigure();
 }`)
	if !page.MustEval(`()=>sent.allowed_tools.length===0 && ['allow_filesystem_write','allow_network_requests','allow_unsandboxed_shell','allow_remote_shell','allow_self_update'].every(key=>sent[key]===false) && !document.getElementById('reconfigure-apply-btn').disabled && lastToast.includes('offline')`).Bool() {
		t.Fatal("revocation or failure recovery failed")
	}
	page.MustEval(`async()=>{window.fail=false;await applyReconfigure();}`)
	if !page.MustEval(`()=>lastToast==='invasion.reconfigure_success' && sent.allowed_tools.length===0`).Bool() {
		t.Fatal("retry did not submit the revocations")
	}
	page.MustEval(`async()=>{window.otherEgg=true;await openReconfigureModal('nest');}`)
	if !page.MustEval(`()=>document.getElementById('reconfigure-allowed-tools').value==='[]'&&Array.from(document.querySelectorAll('#reconfigure-modal input[type=checkbox]')).every(el=>!el.checked)`).Bool() {
		t.Fatal("another Egg's grants were copied into the review")
	}
}
