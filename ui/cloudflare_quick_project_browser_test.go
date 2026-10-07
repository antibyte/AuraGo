package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestCloudflareQuickProjectBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	page := newSmokeBrowser(t).MustPage(configRefreshFixtureOrigin(t, "de", true) + "/config#overview")
	defer page.MustClose()
	waitForJSBool(t, page, `()=>!!document.querySelector('.pw-overview-card')`)
	page.MustEval(`async()=>{
  const originalFetch=window.fetch;
  window.fetch=(url,...args)=>String(url)==='/api/homepage/sites'?Promise.resolve(new Response(JSON.stringify({sites:[{project_dir:'site',name:'My site',status:'active'},{project_dir:'archived',name:'Old site',status:'archived'}]}),{status:200,headers:{'Content-Type':'application/json'}})):originalFetch(url,...args);
  configData.cloudflare_tunnel={enabled:true,auth_method:'quick',quick_project_dir:'site'};
  AuraConfigState.init(configData);resetDirtySnapshot();await selectSection('cloudflare_tunnel');
 }`)
	waitForJSBool(t, page, `()=>document.querySelector('#cf-quick-project')?.disabled===false`)
	if !page.MustEval(`()=>{
  const select=document.querySelector('#cf-quick-project');
  return select.value==='site' && select.options.length===2 && select.options[1].textContent==='My site (site)' && !document.querySelector('[data-path="cloudflare_tunnel.port"]');
 }`).Bool() {
		t.Fatal("project choice was not restricted to active registered sites")
	}
	for _, width := range []int{390, 1440} {
		page.MustSetViewport(width, 1000, 1, width == 390)
		for _, theme := range []string{"dark", "light"} {
			page.MustEval(`async theme=>{document.documentElement.dataset.theme=theme;document.body.dataset.theme=theme;document.querySelector('#cf-quick-project').scrollIntoView({block:'center'});await new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r)));}`, theme)
			if !page.MustEval(`()=>{const c=document.getElementById('content');return c.scrollWidth<=c.clientWidth+1}`).Bool() {
				t.Fatalf("publication selector overflow at %d %s", width, theme)
			}
			if dir := os.Getenv("AURAGO_BROWSER_ARTIFACT_DIR"); dir != "" {
				if err := os.MkdirAll(dir, 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("cloudflare-project-%s-%d.png", theme, width)), page.MustScreenshot(), 0644); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
	page.MustEval(`()=>{const select=document.querySelector('#cf-quick-project');select.value='';select.dispatchEvent(new Event('change',{bubbles:true}));}`)
	if !page.MustEval(`()=>configData.cloudflare_tunnel.quick_project_dir===''`).Bool() {
		t.Fatal("project deselection was not saved in the draft")
	}
	page.MustEval(`()=>resetDirtySnapshot()`)
}
