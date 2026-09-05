package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Exercise the reported layouts through their real lazy renderers, including
// empty results and saved credentials. Geometry checks complement the screenshots.
func TestConfigSpacingBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	page := newSmokeBrowser(t).MustPage(configRefreshFixtureOrigin(t, "de", true) + "/config#overview")
	defer page.MustClose()
	waitForJSBool(t, page, `() => !!document.querySelector('.pw-overview-card')`)
	page.MustEval(`() => { configData.github = {enabled:true, owner:'home-lab', token:'••••••••', allowed_repos:['home-lab/aurago','home-lab/automation']}; AuraConfigState.init(configData); resetDirtySnapshot(); }`)
	for _, example := range []struct{ key, target string }{
		{"github", "#github-test-btn"},
		{"telegram", ".cfg-integration-test"},
		{"webhooks", "#wh-panel-outgoing"},
		{"google_workspace", "[data-path='google_workspace.gmail']"},
		{"a2a", "[data-path='a2a.server.bindings.rest']"},
		{"browser_automation", "[data-path='browser_automation.readonly']"},
	} {
		t.Run(example.key, func(t *testing.T) {
			page.MustEval(`async key => {await selectSection(key,{scrollBehavior:'auto'}); resetDirtySnapshot();}`, example.key)
			for _, width := range []int{390, 768, 1024, 1440, 1920} {
				page.MustSetViewport(width, 1000, 1, width == 390)
				for _, theme := range []string{"dark", "light"} {
					for _, density := range []string{"comfortable", "compact"} {
						page.MustEval(`async (theme,density,target) => {
						 document.documentElement.dataset.theme=theme; document.body.dataset.theme=theme; AuraPrecisionWorkspace.setDensity(density);
						 document.querySelector(target).closest('.cfg-topic').scrollIntoView({block:'start',behavior:'instant'});
						 await new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve)));
						 await Promise.all(document.getAnimations().filter(a=>a.effect?.getTiming().iterations!==Infinity).map(a=>a.finished.catch(()=>{})));
						}`, theme, density, example.target)
						if dir := os.Getenv("AURAGO_BROWSER_ARTIFACT_DIR"); dir != "" {
							if err := os.MkdirAll(dir, 0o755); err != nil {
								t.Fatal(err)
							}
							if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("spacing-%s-%s-%s-%d.png", example.key, theme, density, width)), page.MustScreenshot(), 0o644); err != nil {
								t.Fatal(err)
							}
						}
						issues := page.MustEval(`target => {
						 const card=document.querySelector(target).closest('.cfg-topic'), issues=[];
						 const body=card.querySelector(':scope > .pw-panel-body');
						 if(!body) issues.push('missing shared card body');
						 else {
						   const b=body.getBoundingClientRect(), padding=parseFloat(getComputedStyle(body).paddingLeft);
						   const children=[...body.children].filter(el=>el.getClientRects().length && getComputedStyle(el).display!=='none');
						   if(children.some(el=>el.getBoundingClientRect().left<b.left+padding-1 || el.getBoundingClientRect().right>b.right-padding+1)) issues.push('unequal body insets');
						   if(children.length) {
						     const top=children[0].getBoundingClientRect().top-b.top, bottom=b.bottom-children.at(-1).getBoundingClientRect().bottom;
						     if(Math.abs(top-padding)>1 || Math.abs(bottom-padding)>1) issues.push('unequal vertical body padding: '+JSON.stringify({top,bottom,padding}));
						   }
						 }
						 const toggles=[...card.querySelectorAll('.toggle')].filter(el=>el.getClientRects().length);
						 if(card.querySelector('.toggle-wrap > .toggle[aria-hidden="true"]')) issues.push('hidden binding has a visible state wrapper');
						 const positions=toggles.map(toggle=>{
						   const field=toggle.closest('.cfg-switch-field'), label=field?.querySelector('.field-label');
						   if(!label || label.getBoundingClientRect().right>toggle.getBoundingClientRect().left) issues.push('switch label must precede control in left column');
						   if(label && !field.querySelector('.field-help')) {
						     const l=label.getBoundingClientRect(), c=toggle.parentElement.getBoundingClientRect();
						     if(Math.abs(l.top+l.height/2-c.top-c.height/2)>1) issues.push('switch and label are not vertically centered');
						   }
						   return toggle.getBoundingClientRect().left;
						 });
						 if(positions.length && Math.max(...positions)-Math.min(...positions)>1) issues.push('switch column is not aligned');
						 if([...document.querySelectorAll('.pw-action-reason[hidden]')].some(el=>el.getClientRects().length)) issues.push('hidden reason occupies a row');
						 const empty=card.querySelector('.wh-empty');
						 if(empty && !['none','normal'].includes(getComputedStyle(empty,'::before').content)) issues.push('empty state has an opaque overlay');
						 const content=document.getElementById('content');
						 if(content.scrollWidth>content.clientWidth+1) issues.push('horizontal overflow');
						 return issues;
						}`, example.target).Arr()
						if len(issues) != 0 {
							t.Errorf("%d %s %s: %v", width, theme, density, issues)
						}
					}
				}
			}
		})
	}
}

func TestConfigSpacingInteractionsBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	page := newSmokeBrowser(t).MustPage(configRefreshFixtureOrigin(t, "de", true) + "/config#webhooks")
	cleanupPage := page
	defer func() {
		cleanupPage.MustEval(`() => window.removeEventListener('beforeunload', handleConfigBeforeUnload)`)
		cleanupPage.MustClose()
	}()
	waitForJSBool(t, page, `() => !!document.querySelector('#wh-outgoing-list')`)
	page = page.Timeout(30 * time.Second)
	page.MustSetViewport(1440, 1000, 1, false)
	page.MustEval(`() => { window.spacingHeading=document.querySelector('#wh-panel-outgoing > .cfg-topic-heading'); }`)
	page.MustElement(`[onclick="ogShowModal(-1)"]`).MustClick()
	page.MustElement("#og-f-name").MustInput("Home lab notification")
	page.MustElement("#og-f-url").MustInput("https://webhook.fixture.invalid/notify")
	page.MustElement(`[onclick="ogSave()"]`).MustClick()
	waitForJSBool(t, page, `() => !!document.querySelector('#wh-outgoing-list .wh-card.cfg-object') && getComputedStyle(document.querySelector('#og-modal-overlay')).display==='none'`)
	if !page.MustEval(`() => spacingHeading.isConnected && ogWebhooks[0].url==='••••••••'`).Bool() {
		t.Error("saving a webhook lost its topic heading or masked server state")
	}
	page.MustEval(`() => document.querySelector('#wh-panel-outgoing').scrollIntoView({block:'start',behavior:'instant'})`)
	if dir := os.Getenv("AURAGO_BROWSER_ARTIFACT_DIR"); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "webhooks-saved.png"), page.MustScreenshot(), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	page.MustElement(`[onclick="ogDelete(0)"]`).MustClick()
	page.MustElement("#shared-modal-confirm").MustClick()
	waitForJSBool(t, page, `() => !!document.querySelector('#wh-outgoing-list .wh-empty')`)
	if !page.MustEval(`() => spacingHeading.isConnected`).Bool() {
		t.Error("deleting a webhook lost its topic heading")
	}

	page.MustEval(`async () => {await selectSection('a2a'); document.querySelector('[data-path="a2a.server.bindings.grpc"]').click();}`)
	if !page.MustEval(`() => {
	 const root=document.querySelector('#content > .cfg-section'), control=root.querySelector('[data-path="a2a.server.bindings.grpc"]');
	 let clicks=0; control.addEventListener('click',()=>clicks++);
	 const count=root.querySelectorAll('.pw-panel-body').length;
	 AuraConfigForm.layout(root,'a2a'); AuraConfigForm.layout(root,'a2a');
	 const retained=control.isConnected && control.classList.contains('on') && root.querySelectorAll('.pw-panel-body').length===count;
	 control.click(); return retained && clicks===1 && !control.classList.contains('on') && AuraConfigState.get('a2a.server.bindings.grpc')===false;
	}`).Bool() {
		t.Error("repeated presentation lost field state, listeners or created duplicate bodies")
	}
	page.MustEval(`() => { AuraConfigState.init(configData); resetDirtySnapshot(); }`)
}
