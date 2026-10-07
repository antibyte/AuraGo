package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestYepAPIReadOnlyProbeBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	browser := newSmokeBrowser(t)
	page := browser.MustPage(configRefreshFixtureOrigin(t, "de", false) + "/config#yepapi").Timeout(30 * time.Second)
	defer page.MustClose()
	waitForJSBool(t, page, `()=>typeof renderYepAPISection==='function' && !!document.querySelector('[data-path="yepapi.enabled"]')`)
	page.MustEval(`async()=>{configData.providers=[{id:'probe',name:'Probe',type:'yepapi'}];configData.yepapi={enabled:true,provider:'probe'};AuraConfigState.init(configData);await renderYepAPISection(null);window.__probeRequests=[];window.fetch=async(url,opts)=>{window.__probeRequests.push({url,method:opts?.method});return {json:async()=>({status:'ok',billable:false,authentication_verified:false})}}}`)
	page.MustElement("#yepapi-test-btn").MustClick()
	waitForJSBool(t, page, `()=>document.querySelector('#yepapi-test-result').classList.contains('is-success')`)
	if !page.MustEval(`()=>window.__probeRequests.length===1 && window.__probeRequests[0].method==='POST' && document.querySelector('#yepapi-test-result').textContent.includes('nicht geprüft') && !document.querySelector('#yepapi-test-btn').disabled`).Bool() {
		t.Fatal("probe method, result semantics or completion state is wrong")
	}
	for _, width := range []int{390, 1440} {
		for _, theme := range []string{"dark", "light"} {
			page.MustSetViewport(width, 900, 1, width == 390)
			page.MustEval(`theme=>{document.documentElement.dataset.theme=theme;document.body.dataset.theme=theme}`, theme)
			page.MustElement("#yepapi-test-btn").MustScrollIntoView()
			page.MustEval(`()=>new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r)))`)
			if !page.MustEval(`()=>{const c=document.getElementById('content');return c.scrollWidth<=c.clientWidth+1}`).Bool() {
				t.Fatal("probe controls overflow")
			}
			if dir := os.Getenv("AURAGO_BROWSER_ARTIFACT_DIR"); dir != "" {
				if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("yepapi-probe-%s-%d.png", theme, width)), page.MustScreenshot(), 0600); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
}
