package ui

import (
	"testing"
	"time"

	"github.com/go-rod/rod/lib/proto"
)

func TestHomelabSecurityConfigBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	origin := configRefreshFixtureOriginWith(t, "de", false, func(cfg map[string]any) {
		for _, key := range []string{"onedrive", "huggingface", "paperless_ngx"} {
			section := cfg[key].(map[string]any)
			section["enabled"] = true
			delete(section, "allow_public_share")
			delete(section, "allow_publish")
			delete(section, "allow_delete")
			if key == "huggingface" {
				section["read_only"] = true
			} else {
				section["readonly"] = true
			}
		}
		cfg["telnyx"].(map[string]any)["allowed_numbers"] = []string{"+15551234567"}
	})
	page := newSmokeBrowser(t).MustPage(origin + "/config#overview")
	defer func() {
		_, _ = page.Timeout(5 * time.Second).Eval(`()=>window.removeEventListener("beforeunload",handleConfigBeforeUnload)`)
		_ = page.Timeout(5 * time.Second).Close()
	}()
	waitForJSBool(t, page, `()=>!!document.querySelector('.pw-overview-card')`)
	for _, tc := range []struct{ section, grant, readonly string }{
		{"onedrive", "allow_public_share", "readonly"},
		{"huggingface", "allow_publish", "read_only"},
		{"paperless_ngx", "allow_delete", "readonly"},
	} {
		t.Run(tc.section, func(t *testing.T) {
			path := tc.section + "." + tc.grant
			page.Timeout(10*time.Second).MustEval(`async key=>{await selectSection(key);}`, tc.section)
			selector := `[data-path="` + path + `"]`
			control := page.Timeout(10 * time.Second).MustElement(selector)
			control.MustEval(`()=>{for(let p=this.parentElement;p;p=p.parentElement){if(p.tagName==="DETAILS")p.open=true;}this.scrollIntoView({block:"center",behavior:"instant"});}`)
			if control.MustEval(`()=>this.classList.contains('on')`).Bool() {
				t.Fatal("legacy configuration gained an implicit grant")
			}
			box := control.MustShape().Box()
			page.Mouse.MustMoveTo(box.X+box.Width/2, box.Y+box.Height/2).MustClick(proto.InputMouseButtonLeft)
			if tc.section == "paperless_ngx" {
				waitForJSBool(t, page, `()=>document.querySelector('#shared-modal-overlay')?.classList.contains('active')`)
				if page.MustEval(`()=>AuraConfigState.get('paperless_ngx.allow_delete')===true`).Bool() {
					t.Fatal("deletion enabled before confirmation")
				}
				page.MustElement("#shared-modal-confirm").MustClick()
			}
			waitForJSBool(t, page, `()=>document.querySelector('`+selector+`').classList.contains('on')`)
			if !page.Timeout(10 * time.Second).MustEval(`async()=>await saveConfig()`).Bool() {
				t.Fatal("configuration save failed")
			}
			if !page.MustEval(`async (section,grant,readonly)=>{const cfg=await (await fetch('/api/config')).json(); return cfg[section][grant]===true && cfg[section][readonly]===true;}`, tc.section, tc.grant, tc.readonly).Bool() {
				t.Fatal("save lost grant or changed read-only")
			}
			page.MustReload().MustWaitLoad()
			waitForJSBool(t, page, `()=>document.querySelector('`+selector+`')?.classList.contains('on')`)
		})
	}
	page.MustEval(`async()=>{await selectSection('telnyx');}`)
	page.MustElement(`[data-path="telnyx.owner_number"]`).MustInput("+15551234567")
	if !page.Timeout(10 * time.Second).MustEval(`async()=>await saveConfig()`).Bool() {
		t.Fatal("owner number save failed")
	}
	page.MustReload().MustWaitLoad()
	waitForJSBool(t, page, `()=>document.querySelector('[data-path="telnyx.owner_number"]')?.value==='+15551234567'`)
}
