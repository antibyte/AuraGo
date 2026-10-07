package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestGmailLabelPermissionBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	browser := newSmokeBrowser(t)
	page := browser.MustPage(configRefreshFixtureOrigin(t, "de", false) + "/config#google_workspace").Timeout(30 * time.Second)
	defer page.MustClose()
	waitForJSBool(t, page, `()=>!!document.querySelector('[data-path="google_workspace.gmail_modify_labels"]')`)
	page.MustEval(`()=>document.querySelectorAll('#content details').forEach(el=>el.open=true)`)
	clickToggle := func(path string) {
		page.MustEval(`path=>document.querySelector('[data-path="'+path+'"]').scrollIntoView({block:'center'})`, path)
		page.MustEval(`()=>new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r)))`)
		point := page.MustEval(`path=>{const r=document.querySelector('[data-path="'+path+'"]').getBoundingClientRect();return [r.x+r.width/2,r.y+r.height/2]}`, path).Arr()
		page.Mouse.MustMoveTo(point[0].Num(), point[1].Num()).MustClick("left")
	}
	clickToggle("google_workspace.enabled")
	if page.MustEval(`()=>buildConfigPatchFromForm().google_workspace.gmail_modify_labels`).Bool() {
		t.Fatal("label grant defaults on")
	}
	clickToggle("google_workspace.gmail_modify_labels")
	if !page.MustEval(`()=>buildConfigPatchFromForm().google_workspace.gmail_modify_labels===true && buildConfigPatchFromForm().google_workspace.gmail_send===false`).Bool() {
		t.Fatal("grants are not independently saved")
	}
	for _, width := range []int{390, 1440} {
		for _, theme := range []string{"dark", "light"} {
			page.MustSetViewport(width, 900, 1, width == 390)
			page.MustEval(`theme=>{document.documentElement.dataset.theme=theme;document.body.dataset.theme=theme}`, theme)
			page.MustElement(`[data-path="google_workspace.gmail_modify_labels"]`).MustScrollIntoView()
			page.MustEval(`()=>new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r)))`)
			if !page.MustEval(`()=>{const el=document.getElementById('content');return el.scrollWidth<=el.clientWidth+1}`).Bool() {
				t.Fatal("Gmail grant form overflows")
			}
			if dir := os.Getenv("AURAGO_BROWSER_ARTIFACT_DIR"); dir != "" {
				if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("gmail-permission-%s-%d.png", theme, width)), page.MustScreenshot(), 0600); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
}
