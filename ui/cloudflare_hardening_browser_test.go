package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestCloudflareLifecycleAndValidationBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	page := newSmokeBrowser(t).MustPage(configRefreshFixtureOrigin(t, "de", true) + "/config#overview")
	defer page.MustClose()
	waitForJSBool(t, page, `()=>!!document.querySelector('.pw-overview-card')`)
	page.MustEval(`()=>{
 const original=window.fetch;window.cfPosts=[];window.cfStatus={running:false,state_known:true};
 window.fetch=(url,options={})=>{
  const path=String(url);const json=value=>Promise.resolve(new Response(JSON.stringify(value),{status:200,headers:{'Content-Type':'application/json'}}));
  if(path==='/api/homepage/sites')return json({sites:[]});
  if(path==='/api/cloudflare-tunnel/status')return json({enabled:true,tunnel:window.cfStatus});
  if(path.startsWith('/api/cloudflare-tunnel/')&&options.method==='POST'){
   window.cfPosts.push({path,body:options.body});return new Promise(resolve=>window.cfResolve=()=>resolve(new Response(JSON.stringify(window.cfActionResult||{status:'ok',warnings:[{code:'tls_get',message:'fixture'}]}),{status:200})));
  }
  return original(url,options);
 };
 window.cfRender=async(auth,readOnly=false)=>{
  configData.cloudflare_tunnel={enabled:true,mode:'native',auth_method:auth,readonly:readOnly,custom_ingress:[{hostname:'site.example.com',service:'http://localhost:3000'}]};
  AuraConfigState.init(configData);resetDirtySnapshot();await selectSection('cloudflare_tunnel');
 };
 }`)
	for _, auth := range []string{"token", "named", "quick"} {
		page.MustEval(`async auth=>await window.cfRender(auth)`, auth)
		waitForJSBool(t, page, `()=>document.querySelectorAll('[data-cf-action]').length===3`)
		if !page.MustEval(`()=>!document.querySelector('[data-path="cloudflare_tunnel.auto_start"]').classList.contains('on')`).Bool() {
			t.Fatal("missing auto-start became true")
		}
		if auth == "token" && !page.MustEval(`()=>document.querySelector('#cf-expose-web-ui-val').value==='false'&&document.querySelector('#cf-expose-homepage-val').value==='false'`).Bool() {
			t.Fatal("missing publication grants became true")
		}
		if auth == "named" && !page.MustEval(`()=>!!document.querySelector('textarea[data-path="cloudflare_tunnel.custom_ingress"]')&&!document.querySelector('[data-path="cloudflare_tunnel.expose_web_ui"]')`).Bool() {
			t.Fatal("named routes still use automatic targets")
		}
		for _, width := range []int{390, 1440} {
			page.MustSetViewport(width, 1000, 1, width == 390)
			for _, theme := range []string{"dark", "light"} {
				page.MustEval(`async theme=>{document.documentElement.dataset.theme=theme;document.body.dataset.theme=theme;document.getElementById('content').scrollTop=0;await new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r)));}`, theme)
				if !page.MustEval(`()=>{const c=document.getElementById('content');return c.scrollWidth<=c.clientWidth+1&&Array.from(document.querySelectorAll('[data-cf-action]')).every(b=>b.getBoundingClientRect().width>30)}`).Bool() {
					t.Fatalf("actions overflow at %s %d %s: %s", auth, width, theme, page.MustEval(`()=>{const c=document.getElementById('content');return JSON.stringify({content:[c.scrollWidth,c.clientWidth],actions:Array.from(document.querySelectorAll('[data-cf-action]')).map(e=>[e.textContent,e.getBoundingClientRect().width]),section:document.querySelector('.cfg-section')?.className});}`).Str())
				}
				if dir := os.Getenv("AURAGO_BROWSER_ARTIFACT_DIR"); dir != "" {
					if err := os.MkdirAll(dir, 0755); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("cloudflare-%s-%s-%d.png", auth, theme, width)), page.MustScreenshot(), 0644); err != nil {
						t.Fatal(err)
					}
				}
			}
		}
		for _, action := range []string{"start", "stop", "restart"} {
			page.MustEval(`action=>{setNestedValue(configData,'cloudflare_tunnel.auth_method','unsaved');cloudflareTunnelAction(action);}`, action)
			waitForJSBool(t, page, `()=>window.cfResolve&&document.querySelector('#cf-action-status').dataset.pending==='true'`)
			if !page.MustEval(`action=>Array.from(document.querySelectorAll('[data-cf-action]')).every(b=>b.disabled)&&cfPosts.at(-1).body===undefined&&cfPosts.at(-1).path==='/api/cloudflare-tunnel/'+action`, action).Bool() {
				t.Fatal("pending actions or saved-settings boundary failed")
			}
			page.MustEval(`()=>{cfResolve();window.cfResolve=null;}`)
			waitForJSBool(t, page, `()=>document.querySelector('#cf-action-status').classList.contains('is-pending')&&!document.querySelector('[data-cf-action="start"]').disabled`)
		}
		page.MustEval(`()=>{window.cfActionResult={status:'error',message:'fixture action failed'};cloudflareTunnelAction('stop');}`)
		waitForJSBool(t, page, `()=>!!window.cfResolve`)
		page.MustEval(`()=>{cfResolve();window.cfResolve=null;window.cfActionResult=null;}`)
		waitForJSBool(t, page, `()=>document.querySelector('#cf-action-status').classList.contains('is-error')&&document.querySelector('#cf-action-status').textContent==='fixture action failed'&&!document.querySelector('[data-cf-action="stop"]').disabled`)
		page.MustEval(`async auth=>await window.cfRender(auth,true)`, auth)
		if !page.MustEval(`()=>Array.from(document.querySelectorAll('[data-cf-action]')).every(b=>b.disabled)`).Bool() {
			t.Fatal("read-only actions enabled")
		}
	}
	page.MustEval(`async()=>await window.cfRender('token')`)
	for _, value := range []string{"-1", "65536", "1.5"} {
		if !page.MustEval(`async value=>{const field=document.querySelector('[data-path="cloudflare_tunnel.metrics_port"]');field.value=value;field.dispatchEvent(new Event('input',{bubbles:true}));field.dispatchEvent(new Event('change',{bubbles:true}));const saved=await saveConfig();return saved===false&&field.value===value&&field.getAttribute('aria-invalid')==='true'&&AuraConfigState.get('cloudflare_tunnel.metrics_port')===Number(value);}`, value).Bool() {
			t.Fatalf("invalid port %s was lost or saved", value)
		}
	}
	if !page.MustEval(`async()=>{const port=configData.server.port;const field=document.querySelector('[data-path="cloudflare_tunnel.metrics_port"]');field.value=String(port);field.dispatchEvent(new Event('change',{bubbles:true}));return await saveConfig()===false&&field.getAttribute('aria-invalid')==='true';}`).Bool() {
		t.Fatal("active listener collision accepted")
	}
	page.MustEval(`()=>{window.cfStatus={running:null,state_known:false,status:'error'};cloudflareTunnelCheckStatus();}`)
	waitForJSBool(t, page, `()=>document.querySelector('#cf-tunnel-status-banner').classList.contains('is-danger')`)
	page.MustEval(`()=>{window.cfStatus={running:true,state_known:true,warnings:[{code:'tls_get'}]};cloudflareTunnelCheckStatus();}`)
	waitForJSBool(t, page, `()=>document.querySelector('#cf-tunnel-status-banner').classList.contains('is-warning')`)
	page.MustEval(`()=>resetDirtySnapshot()`)
}
