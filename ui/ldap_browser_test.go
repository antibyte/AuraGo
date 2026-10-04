package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLDAPTransportConfigBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	browser := newSmokeBrowser(t)
	page := browser.MustPage(configRefreshFixtureOrigin(t, "de", false) + "/config#ldap").Timeout(30 * time.Second)
	defer page.MustClose()
	waitForJSBool(t, page, `()=>!!document.querySelector('#ldap-transport')`)
	page.MustEval(`()=>document.querySelectorAll('#content details').forEach(el=>el.open=true)`)
	if !page.MustEval(`()=>document.querySelector('#ldap-transport').value==='ldaps'`).Bool() {
		t.Fatal("legacy TLS selection lost")
	}
	page.MustElement("#ldap-transport").MustSelect("StartTLS")
	if !page.MustEval(`()=>document.querySelector('[data-path="ldap.port"]').value==='389' && buildConfigPatchFromForm().ldap.tls_mode==='starttls'`).Bool() {
		t.Fatal("StartTLS draft or port missing")
	}
	page.MustElement(`[data-path="ldap.port"]`).MustSelectAllText().MustInput("1389")
	page.MustElement("#ldap-transport").MustSelect("LDAPS")
	if !page.MustEval(`()=>document.querySelector('[data-path="ldap.port"]').value==='1389'`).Bool() {
		t.Fatal("custom port overwritten")
	}
	for _, width := range []int{390, 1440} {
		for _, theme := range []string{"dark", "light"} {
			page.MustSetViewport(width, 900, 1, width == 390)
			page.MustEval(`theme=>{document.documentElement.dataset.theme=theme;document.body.dataset.theme=theme;document.querySelectorAll('#content details').forEach(el=>el.open=true)}`, theme)
			page.MustEval(`()=>new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r)))`)
			if !page.MustEval(`()=>{const el=document.getElementById('content');return el.scrollWidth<=el.clientWidth+1}`).Bool() {
				t.Fatal("LDAP form overflows")
			}
			if dir := os.Getenv("AURAGO_BROWSER_ARTIFACT_DIR"); dir != "" {
				if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("ldap-%s-%d.png", theme, width)), page.MustScreenshot(), 0600); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
}
