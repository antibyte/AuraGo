package ui

import (
	"encoding/json"
	"os/exec"
	"testing"

	"aurago/internal/config"
)

func TestKaggleConfigLimitsMatchBackendAndRejectOverflow(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is required")
	}
	c := config.DefaultKaggleConfig()
	c.RequestTimeoutSeconds, c.RemoteTimeoutMinutes = 1000000, 1000000
	c.Local.MaxSessions, c.Local.IdleMinutes, c.Local.CellTimeoutMinutes = 1000000, 1000000, 1000000
	config.NormalizeKaggleConfig(&c)
	limits, _ := json.Marshal(map[string]int{"kaggle.request_timeout_seconds": c.RequestTimeoutSeconds, "kaggle.remote_timeout_minutes": c.RemoteTimeoutMinutes, "kaggle.local.max_sessions": c.Local.MaxSessions, "kaggle.local.idle_minutes": c.Local.IdleMinutes, "kaggle.local.cell_timeout_minutes": c.Local.CellTimeoutMinutes})
	script := `const fs=require('fs'),vm=require('vm'),assert=require('assert');
const context={window:{},document:{dispatchEvent(){}},CustomEvent:class{},structuredClone};vm.createContext(context);
for(const path of ['js/config/catalog.js','js/config/state.js'])vm.runInContext(fs.readFileSync(path,'utf8'),context);
const state=context.window.AuraConfigState,rules=context.window.AuraConfigCatalog.validationRules;
const limits=JSON.parse(process.argv[1]);
for(const [path,max] of Object.entries(limits)){
 assert.strictEqual(rules[path].max,max,path);state.init({});state.setRules(rules);
 for(const value of [1,max]){state.set(path,value);assert(!state.validate().errors.some(e=>e.path===path),path+' boundary');}
 for(const value of [0,max+1]){state.set(path,value);assert(state.validate().errors.some(e=>e.path===path),path+' overflow');}
}
console.log('Kaggle UI rules match normalized backend bounds');`
	if out, err := exec.Command(node, "-e", script, string(limits)).CombinedOutput(); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
}

func TestKaggleConfigLimitsBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	origin := configRefreshFixtureOrigin(t, "de", false)
	browser := newSmokeBrowser(t)
	page := browser.MustPage(origin + "/config#kaggle")
	defer page.MustClose()
	waitForJSBool(t, page, `()=>!!document.querySelector('[data-path="kaggle.request_timeout_seconds"]')`)
	page.MustEval(`()=>{window.configWrites=0;const original=window.fetch;window.fetch=(url,init)=>{if(String(url)==='/api/config'&&init?.method==='PUT')configWrites++;return original(url,init);};}`)
	for path, max := range map[string]int{"kaggle.request_timeout_seconds": 300, "kaggle.remote_timeout_minutes": 720, "kaggle.local.max_sessions": 8, "kaggle.local.idle_minutes": 240, "kaggle.local.cell_timeout_minutes": 720} {
		if !page.MustEval(`async(path,max)=>{
const el=document.querySelector('[data-path="'+path+'"]');if(Number(el.max)!==max)return false;
el.value=max+1;el.dispatchEvent(new Event('input',{bubbles:true}));AuraConfigState.syncFromDOM();
await saveConfig();
const rejected=configWrites===0&&el.getAttribute('aria-invalid')==='true'&&AuraConfigState.validate().errors.some(e=>e.path===path&&e.code==='max');
el.value=max;el.dispatchEvent(new Event('input',{bubbles:true}));AuraConfigState.syncFromDOM();
return rejected&&!AuraConfigState.validate().errors.some(e=>e.path===path);
}`, path, max).Bool() {
			t.Fatalf("invalid UI boundary: %s", path)
		}
	}
	page.MustEval(`async()=>{await saveConfig();window.removeEventListener('beforeunload',handleConfigBeforeUnload);}`)
	if page.MustEval(`()=>configWrites`).Int() != 1 {
		t.Fatal("valid limits were not saved exactly once")
	}
	page.MustReload().MustWaitLoad()
	waitForJSBool(t, page, `()=>!!document.querySelector('[data-path="kaggle.request_timeout_seconds"]')`)
	if !page.MustEval(`()=>['kaggle.request_timeout_seconds','kaggle.remote_timeout_minutes','kaggle.local.max_sessions','kaggle.local.idle_minutes','kaggle.local.cell_timeout_minutes'].every(path=>{const el=document.querySelector('[data-path="'+path+'"]');return el.value===el.max;})`).Bool() {
		t.Fatal("saved limits did not survive reload")
	}
}
