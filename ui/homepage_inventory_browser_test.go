package ui

import "testing"

func TestHomepageInventoryReasonsAndLocationsBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	page := newSmokeBrowser(t).MustPage(newPrecisionSmokeOrigin(t))
	defer page.MustClose()
	source := readDesktopAssetText(t, "js/desktop/apps/homepage-studio-sites.js")
	result := page.MustEval(`async source=>{
  const container=document.createElement('div');document.body.append(container);
  const escape=value=>String(value).replace(/[&<>"']/g,char=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[char]));
  eval(source);
  const site={id:1,name:'Campaign',project_dir:'campaign',local_root:'C:/sites/campaign',container_path:'/workspace/campaign',origin:'agent_created',drift_status:'clean',
    deploy_targets:[{provider:'vercel',url:'javascript:alert(1)',status:'inactive'}],
    events:[{id:4,operation_id:'one',reason:'Fix <img src=x onerror=alert(1)> menu',outcome:'succeeded',details:{files_changed:[{path:'index.html'}]}},
            {id:3,operation_id:'one',reason:'same operation',outcome:'succeeded'}]};
  const panel=window.HomepageStudioSites.create({container,esc:escape,t:key=>key,readonly:true,api:async()=>({sites:[site]})});
  await panel.load();panel.setSelected(1,site);
  const summary=container.querySelector('details summary');
  summary.click();
  return {locations:container.textContent.includes('C:/sites/campaign') && container.textContent.includes('/workspace/campaign'),
    escaped:container.querySelectorAll('img').length===0,grouped:container.querySelectorAll('details').length===1,
    selected:panel.getSelected()===1,open:container.querySelector('details').open,
    unsafeLink:container.querySelectorAll('a[href^="javascript:"]').length,
    stopped:container.textContent.includes('homepage_studio.sites_target_inactive')};
 }`, source)
	for _, key := range []string{"locations", "escaped", "grouped", "selected", "open", "stopped"} {
		if !result.Get(key).Bool() {
			t.Fatalf("%s failed: %s", key, result.JSON("", ""))
		}
	}
	if result.Get("unsafeLink").Int() != 0 {
		t.Fatal("unsafe target link rendered")
	}
}
