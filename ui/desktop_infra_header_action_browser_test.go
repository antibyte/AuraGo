package ui

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestDesktopInfraHeaderActionFitsBrowser covers the inspector header action: its label ("In Schnellverbindung
// öffnen" in German) is longer than the 360 px inspector's text column in some locales, and the button grows with it
// (centred lines, no fixed height) instead of letting the text spill out of its box. It checks every locale and every
// linkable app, plus the "Edit" button of an asset without an app link.
func TestDesktopInfraHeaderActionFitsBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	fixture := newInfraAppBrowserFixture()
	fixture.world(25)
	linked := map[string]string{"fixture-node-0003": "quick-connect", "fixture-node-0004": "network-cameras", "fixture-node-0005": "software-store", "fixture-node-0006": "meshcore"}
	for _, node := range fixture.nodes {
		if app, ok := linked[node["id"].(string)]; ok {
			node["app_links"] = []any{map[string]any{"app": app, "target_id": "target-" + app}}
		}
	}
	page := infraBrowserPageWithFixture(t, fixture)
	defer page.Close()
	page.MustSetViewport(1920, 1200, 1, false)
	openInfraBrowserApp(page)
	page.MustWait(`()=>document.querySelectorAll('[data-nodes] [data-node]').length>0`)
	page.MustEval(`()=>{const win=document.querySelector('.vd-window[data-app-id="infra"]');win.classList.remove('maximized','vd-mobile-forced-maximized');win.style.right='auto';win.style.width='1500px';win.style.height='900px';win.style.left='210px';win.style.top='70px';}`)
	page.MustWait(`()=>document.querySelector('.infra-app').clientWidth>1000`)
	page.MustEval(`()=>{
		window.infraHeaderAction=()=>{
			const button=document.querySelector('[data-inspector] .infra-head-pills .infra-button');
			if(!button) return JSON.stringify({missing:true});
			const box=button.getBoundingClientRect(),range=document.createRange();
			range.selectNodeContents(button);
			const text=range.getBoundingClientRect(),column=document.querySelector('[data-inspector] .infra-head-main').getBoundingClientRect();
			const near=(a,b)=>a<=b+1;
			return JSON.stringify({label:button.textContent.trim(),
				fitsHeight:button.scrollHeight<=button.clientHeight+1,fitsWidth:button.scrollWidth<=button.clientWidth+1,
				textInside:near(box.top,text.top)&&near(text.bottom,box.bottom)&&near(box.left,text.left)&&near(text.right,box.right),
				insideColumn:near(column.left,box.left)&&near(box.right,column.right),lines:Math.round(text.height/parseFloat(getComputedStyle(button).lineHeight)),
				inspectorWidth:Math.round(document.querySelector('[data-inspector]').getBoundingClientRect().width)});
		};
	}`)

	// Every asset in turn, so each locale repaints the header; the order never repeats an asset back to back.
	assets := []struct{ id, app string }{{"fixture-node-0003", "quick-connect"}, {"fixture-node-0004", "network-cameras"}, {"fixture-node-0005", "software-store"}, {"fixture-node-0006", "meshcore"}, {"fixture-node-0010", ""}}
	checked, wrapped := 0, 0
	for _, code := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		var all map[string]any
		if err := json.Unmarshal([]byte(readDesktopAssetText(t, "lang/desktop/"+code+".json")), &all); err != nil {
			t.Fatal(err)
		}
		words := map[string]any{}
		for key, value := range all {
			if strings.HasPrefix(key, "desktop.infra") || strings.HasPrefix(key, "desktop.app_infra") {
				words[key] = value
			}
		}
		page.MustEval(`(words)=>Object.assign(window.fixtureWords,words)`, words)
		for _, asset := range assets {
			page.MustEval(`(id)=>document.querySelector('[data-node="'+id+'"]').click()`, asset.id)
			if asset.app != "" {
				page.MustWait(`(app)=>document.querySelector('[data-inspector] [data-action="open-first-app"]')?.dataset.openApp===app`, asset.app)
			} else {
				page.MustWait(`()=>!!document.querySelector('[data-inspector] [data-action="edit-details"]')`)
			}
			var got struct {
				Missing, FitsHeight, FitsWidth, TextInside, InsideColumn bool
				Label                                                    string
				Lines, InspectorWidth                                    int
			}
			if err := json.Unmarshal([]byte(page.MustEval(`()=>infraHeaderAction()`).Str()), &got); err != nil {
				t.Fatal(err)
			}
			if got.Missing || !got.FitsHeight || !got.FitsWidth || !got.TextInside || !got.InsideColumn || got.Label == "" || got.InspectorWidth != 360 {
				t.Fatalf("%s, %s: the header action %q does not sit inside its box at the default inspector width: %+v", code, asset.id, got.Label, got)
			}
			checked++
			if got.Lines > 1 {
				wrapped++
			}
			if asset.app != "" {
				want := strings.ReplaceAll(words["desktop.infra_open_in_app"].(string), "{app}", words["desktop.infra_app_"+strings.ReplaceAll(asset.app, "-", "_")].(string))
				if got.Label != want {
					t.Fatalf("%s, %s: the header action reads %q; want the full label %q", code, asset.id, got.Label, want)
				}
			}
		}
	}
	t.Logf("%d header actions fit their box; %d of them wrap to more than one line", checked, wrapped)
	if wrapped == 0 {
		t.Fatal("no locale wraps the header action: the test no longer exercises the long-label case")
	}
	requireInfraBrowserClean(t, page)
}
