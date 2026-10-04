package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestRemoteEnrollmentTokenBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	browser := newSmokeBrowser(t)
	page := browser.MustPage(configRefreshOrigin(t) + "/config#overview")
	defer page.MustClose()
	waitForJSBool(t, page, `()=>!!document.querySelector('.pw-overview-card')`)
	page.MustEval(`async()=>{
		const original=window.fetch;window.remoteApproved=false;
		window.fetch=async(url,opts={})=>{
			if(String(url)==='/api/remote/devices/pending-device/approve'){
				window.remoteApproved=opts.method==='POST';
				return new Response(JSON.stringify({token:'fixture-one-time-token',expires_at:'2027-01-01T00:00:00Z'}));
			}
			if(String(url)==='/api/remote/devices') return new Response(JSON.stringify(window.remoteApproved?[]:[{id:'pending-device',name:'Pending desktop',status:'pending'}]));
			return original(url,opts);
		};
		configData.remote_control={enabled:true,allowed_paths:[]};
		await selectSection('remote_control',{scrollBehavior:'auto'});
	}`)
	waitForJSBool(t, page, `()=>!!document.querySelector('[data-approve-device]')`)
	for _, width := range []int{390, 1440} {
		for _, theme := range []string{"dark", "light"} {
			page.MustSetViewport(width, 900, 1, false)
			page.MustEval(`theme=>{document.documentElement.dataset.theme=theme;document.body.dataset.theme=theme}`, theme)
			page.MustElement("[data-approve-device]").MustScrollIntoView()
			if dir := os.Getenv("AURAGO_BROWSER_ARTIFACT_DIR"); dir != "" {
				if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("remote-approval-%d-%s.png", width, theme)), page.MustScreenshot(), 0644); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
	page.MustElement("[data-approve-device]").MustClick()
	waitForJSBool(t, page, `()=>document.getElementById('rc-token-value')?.textContent==='fixture-one-time-token' && !document.querySelector('[data-approve-device]')`)
	if page.MustEval(`()=>!!document.querySelector('[data-path="remote_control.auto_approve"]')`).Bool() {
		t.Fatal("unsafe auto-approval is still offered")
	}
}
