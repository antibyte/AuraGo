package ui

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
)

func TestSoftwareStoreDisposesSensitiveDialogsBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	bin, ok := browserExecutable()
	if !ok {
		t.Fatal("Chrome or Edge required")
	}
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(Content)))
	mux.HandleFunc("/fixture", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, `<!doctype html><html><body><div id="host"></div>
<script src="/js/desktop/apps/software-store.js"></script><script>
window.holdCredential=false;window.holdConfig=false;window.resolveCredential=null;window.resolveConfig=null;
const catalog={catalog:[
 {id:'credentials',name:'Credentials',generated_secrets:[{expose:true}]},
 {id:'gods-eye-view',name:'Gods Eye'}],
 installed:[{app_id:'credentials',status:'running'},{app_id:'gods-eye-view',status:'running'}]};
const api=url=>{
 if(url==='/api/desktop/store/catalog')return Promise.resolve(catalog);
 if(url.endsWith('/credentials'))return window.holdCredential?new Promise(resolve=>window.resolveCredential=resolve):Promise.resolve({credentials:[{label:'Key',value:'synthetic-test-secret'}]});
 if(url.endsWith('/config'))return window.holdConfig?new Promise(resolve=>window.resolveConfig=resolve):Promise.resolve({configured:{},allowed_origins:[]});
 return Promise.resolve({});
};
window.mountStore=()=>SoftwareStoreApp.render(document.getElementById('host'),'audit',{api,t:key=>key,esc:value=>String(value??'')});
</script></body></html>`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	launch := launcher.New().Bin(bin).Headless(true).NoSandbox(true)
	browser := rod.New().ControlURL(launch.MustLaunch()).MustConnect()
	defer launch.Cleanup()
	defer browser.Close()
	page := browser.MustPage(srv.URL + "/fixture").Timeout(30 * time.Second)
	defer page.Close()
	page.MustWaitLoad()
	page.MustWait(`()=>!!window.mountStore`)
	result := page.MustEval(`async()=>{
 const until=async predicate=>{for(let i=0;i<100;i++){if(predicate())return;await new Promise(r=>setTimeout(r,30))}throw Error('store condition timed out')};
 mountStore();await until(()=>!!document.querySelector('[data-action=credentials]'));
 document.querySelector('[data-action=credentials]').click();await until(()=>!!document.querySelector('.vd-store-credential input'));
 const credential=document.querySelector('.vd-store-credential input');
 if(credential.value!=='synthetic-test-secret')throw Error('fixture credential missing');
 document.querySelector('.vd-store-modal [data-action=close]').click();
 if(credential.value||credential.getAttribute('value')||credential.isConnected||document.querySelector('.vd-modal-backdrop'))throw Error('credential retained after close');
 document.querySelector('[data-action=configure-gev]').click();await until(()=>!!document.querySelector('.vd-gev-config'));
 const key=document.querySelector('.vd-gev-config input[name=OPENAI_API_KEY]');key.value='synthetic-config-secret';
 SoftwareStoreApp.dispose('audit');
 if(key.value||key.getAttribute('value')||key.isConnected||document.querySelector('.vd-modal-backdrop'))throw Error('config secret retained after disposal');
 mountStore();await until(()=>!!document.querySelector('[data-action=credentials]'));
 window.holdCredential=true;document.querySelector('[data-action=credentials]').click();await until(()=>!!window.resolveCredential);
 SoftwareStoreApp.dispose('audit');window.resolveCredential({credentials:[{label:'Key',value:'late-secret'}]});
 await new Promise(r=>setTimeout(r,100));
 if(document.querySelector('.vd-modal-backdrop'))throw Error('late credential response reopened dialog');
 return true;
 }`)
	if !result.Bool() {
		t.Fatal("store disposal browser regression")
	}
}
