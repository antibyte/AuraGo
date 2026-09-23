package ui

import "testing"

func TestDesktopLogViewerWheelScrollBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	browser := newSmokeBrowser(t)
	page := browser.MustPage(newPrecisionSmokeOrigin(t))
	defer page.MustClose()
	page.MustSetViewport(1100, 760, 1, false)
	css := string(mustReadUIFile(t, "css/desktop-app-log-viewer.css"))
	page.MustSetDocumentContent(`<!doctype html><html><head><style>
        * { box-sizing: border-box; }
        html, body { margin: 0; height: 100%; }
        #host { width: 920px; height: 640px; }
    ` + css + `</style></head><body><div id="host"></div></body></html>`)
	if err := page.AddScriptTag("", string(mustReadUIFile(t, "js/desktop/apps/log-viewer-filters.js"))); err != nil {
		t.Fatal(err)
	}
	if err := page.AddScriptTag("", string(mustReadUIFile(t, "js/desktop/apps/log-viewer.js"))); err != nil {
		t.Fatal(err)
	}
	page.MustEval(`() => {
        window.EventSource = class { close() {} };
        const lines = Array.from({length:600}, (_, index) => ({
            line_no:index+1, level:'INFO', msg:'Log line '+(index+1), raw:'Log line '+(index+1)
        }));
        LogViewerApp.render(document.getElementById('host'), 'fixture', {
            t:key=>key,
            esc:value=>String(value),
            api:async url=>url.endsWith('/files')
                ? {files:[{name:'aurago.log', size:60000}], log_dir:'logs'}
                : {lines, eof_offset:60000}
        });
    }`)
	waitForJSBool(t, page, `() => {
        const scroller=document.querySelector('.vd-logviewer-scroller');
        return scroller && scroller.scrollHeight>scroller.clientHeight+1000 && scroller.scrollTop>1000;
    }`)
	scroller := page.MustElement(".vd-logviewer-scroller")
	scroller.MustHover()
	before := page.MustEval(`() => document.querySelector('.vd-logviewer-scroller').scrollTop`).Num()
	if err := page.Mouse.Scroll(0, -120, 1); err != nil {
		t.Fatal(err)
	}
	page.MustEval(`() => new Promise(resolve=>setTimeout(resolve, 150))`)
	after := page.MustEval(`() => document.querySelector('.vd-logviewer-scroller').scrollTop`).Num()
	if after >= before || before-after > 500 {
		t.Fatalf("one wheel step should move up a short distance, got %.0f → %.0f", before, after)
	}
	if err := page.Mouse.Scroll(0, 120, 1); err != nil {
		t.Fatal(err)
	}
	page.MustEval(`() => new Promise(resolve=>setTimeout(resolve, 150))`)
	returned := page.MustEval(`() => document.querySelector('.vd-logviewer-scroller').scrollTop`).Num()
	if returned <= after || returned-after > 500 {
		t.Fatalf("one wheel step should move down a short distance, got %.0f → %.0f", after, returned)
	}
}
