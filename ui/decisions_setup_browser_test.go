package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDecisionsSetupCheckBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	browser := newSmokeBrowser(t)
	page := browser.MustPage(configRefreshFixtureOrigin(t, "de", false) + "/config#overview").Timeout(60 * time.Second)
	defer page.MustClose()
	page.MustSetViewport(1440, 1000, 1, false)
	waitForJSBool(t, page, `()=>!!document.querySelector('.pw-overview-card')`)
	page.MustEval(`()=>{
		window.checkCalls=0;window.installCalls=0;
		window.decisionStatus={enabled:true,installed:false,ready:false,state:'error',code:'runtime_artifact_missing',deployment:'auto'};
		const previous=fetch;
		window.fetch=async(url,options)=>{
			if(url==='/api/decisions/status')return new Response(JSON.stringify(decisionStatus));
			if(url==='/api/decisions/check'){
				checkCalls++;
				if(window.holdCheck)return new Promise(resolve=>window.resolveCheck=resolve);
				if(window.checkAvailable)return new Response(JSON.stringify({deployment:'native',target:'linux-amd64',runtime_available:true}));
				return new Response(JSON.stringify({deployment:'native',target:'linux-amd64',runtime_available:false,code:'runtime_artifact_missing'}));
			}
			if(url==='/api/decisions/install'){installCalls++;return new Response('{}');}
			return previous(url,options);
		};
	}`)
	page.MustElement(`.sidebar-group:has([data-section="decisions"]) .sidebar-group-header`).MustClick()
	page.MustElement(`[data-section="decisions"]`).MustClick()
	waitForJSBool(t, page, `()=>!!document.querySelector('[data-decisions-action="check"]')`)
	if !page.MustEval(`()=>checkCalls===0 && installCalls===0 && document.querySelector('[data-decisions-action="install"]').disabled && document.querySelector('[data-decisions-action="start"]').disabled`).Bool() {
		t.Fatal("opening settings performed a check or enabled unprepared actions")
	}
	page.MustElement(`[data-decisions-action="check"]`).MustClick()
	waitForJSBool(t, page, `()=>document.querySelector('[data-decisions-check]')?.textContent.includes('linux-amd64')`)
	if !page.MustEval(`()=>!document.querySelector('[data-decisions-check]').hidden && document.querySelector('[data-decisions-check]').textContent.includes('fehlt') && document.querySelector('[data-decisions-action="install"]').disabled && !document.querySelector('[data-decisions-status]').textContent.includes('fehlt') && installCalls===0`).Bool() {
		t.Fatal("missing package feedback is unclear, duplicated, or permits install")
	}
	for _, width := range []int{390, 1440} {
		for _, theme := range []string{"light", "dark"} {
			page.MustSetViewport(width, 1000, 1, width == 390)
			page.MustEval(`theme=>{document.documentElement.dataset.theme=theme;document.body.dataset.theme=theme;}`, theme)
			page.MustEval(`()=>document.querySelector('[data-decisions-mode="local"]').scrollIntoView({block:'start'})`)
			page.MustEval(`()=>new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r)))`)
			if page.MustEval(`()=>document.documentElement.scrollWidth>innerWidth+1 || document.getElementById('content').scrollWidth>document.getElementById('content').clientWidth+1`).Bool() {
				t.Fatalf("setup overflows at %d in %s", width, theme)
			}
			if dir := os.Getenv("AURAGO_DECISIONS_SCREENSHOTS"); dir != "" {
				if err := os.MkdirAll(dir, 0o755); err != nil {
					t.Fatal(err)
				}
				page.MustScreenshot(filepath.Join(dir, fmt.Sprintf("decisions-setup-%d-%s.png", width, theme)))
			}
		}
	}
	page.MustEval(`async()=>{document.querySelector('[data-path="decisions.enabled"]').click();await saveConfig();window.checkAvailable=true;}`)
	waitForJSBool(t, page, `()=>!document.querySelector('[data-decisions-action="check"]').disabled`)
	page.MustEval(`()=>document.querySelector('[data-decisions-action="check"]').click()`)
	waitForJSBool(t, page, `()=>!document.querySelector('[data-decisions-action="install"]').disabled`)
	if !page.MustEval(`()=>document.querySelector('[data-decisions-check]').textContent.includes('gefunden') && !document.querySelector('[data-decisions-status]').textContent.includes('fehlt')`).Bool() {
		t.Fatal("a successful recheck retained the obsolete missing-package error")
	}
	page.MustEval(`()=>{window.holdCheck=true;document.querySelector('[data-decisions-action="check"]').click();}`)
	waitForJSBool(t, page, `()=>typeof resolveCheck==='function'`)
	page.MustEval(`async()=>{
		const field=document.querySelector('[data-path="decisions.deployment"]');field.value='docker';field.dispatchEvent(new Event('change',{bubbles:true}));
		resolveCheck(new Response(JSON.stringify({deployment:'native',runtime_available:true})));
		await new Promise(resolve=>setTimeout(resolve,30));
	}`)
	if !page.MustEval(`()=>document.querySelector('[data-decisions-check]').hidden && document.querySelector('[data-decisions-action="install"]').disabled && !document.querySelector('[data-decisions-save-notice]').hidden`).Bool() {
		t.Fatal("late check response unlocked a changed unsaved setup")
	}
}
