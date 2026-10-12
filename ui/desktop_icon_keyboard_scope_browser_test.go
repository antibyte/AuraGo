package ui

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"aurago/internal/desktop"
	"github.com/go-rod/rod/lib/input"
	"github.com/go-rod/rod/lib/proto"
)

// Enter, Delete and F2 act on a selected desktop icon only while the keyboard
// belongs to the desktop. Keys pressed inside an app window must never open,
// delete or rename that icon, while the global shell shortcuts keep working.
func TestDesktopIconShortcutsStayOnDesktopBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	html := regexp.MustCompile(`(?s)<script\b[^>]*>.*?</script>`).ReplaceAllString(readDesktopAssetText(t, "desktop.html"), "")
	html = regexp.MustCompile(`\{\{[^}]*\}\}`).ReplaceAllString(html, "")
	html = strings.Replace(html, "</body>", `<script src="/js/shared/lazy-assets.js"></script><script src="/js/desktop/core/module-loader.js"></script><script src="/icon-keys-shell.js"></script><script src="/testdata/aurora-fixture.js"></script></body>`, 1)
	shell := readDesktopAssetText(t, "js/desktop/bundles/main.bundle.js")
	cut := strings.LastIndex(shell, "    ensureDesktopRadialMenuAnchor();")
	if cut < 0 {
		t.Fatal("desktop startup seam missing")
	}
	shell = shell[:cut] + `window.aurora={state,loadIconManifest,applyDesktopSettings,renderIcons,renderStartApps,renderStartButtonIcon,wireShellChromeControls,bindViewportMetrics,handleDesktopKeydown,closeContextMenu,openApp,closeWindow};})();`
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(Content)))
	mux.HandleFunc("/fixture", func(w http.ResponseWriter, _ *http.Request) { fmt.Fprint(w, html) })
	mux.HandleFunc("/icon-keys-shell.js", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/javascript")
		fmt.Fprint(w, shell)
	})
	mux.HandleFunc("/fixture-words", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, readDesktopAssetText(t, "lang/desktop/de.json"))
	})
	mux.HandleFunc("/fixture-apps", func(w http.ResponseWriter, _ *http.Request) { json.NewEncoder(w).Encode(desktop.BuiltinApps()) })
	server := httptest.NewServer(mux)
	defer server.Close()
	page := newSmokeBrowser(t).MustPage(server.URL + "/fixture").Timeout(60 * time.Second)
	defer page.Close()
	page.MustWaitLoad()
	page.MustSetViewport(1366, 900, 1, false)
	// Silent deletion is the dangerous configuration: no dialog would catch a stray Delete.
	page.MustEval(`async()=>{
        await fixtureReady;
        await AuraDesktopModules.loadAppAssets('calculator');
        aurora.state.bootstrap.settings['files.confirm_delete']=false;
        aurora.state.desktopFiles=aurora.state.bootstrap.desktop_files;
        aurora.renderIcons();
        window.fixtureMutations=[];
        const inner=window.fetch;
        window.fetch=(url,options={})=>{
            const method=String(options.method||'GET').toUpperCase();
            if(method!=='GET'&&method!=='HEAD')fixtureMutations.push(method+' '+url);
            return inner(url,options);
        };
    }`)

	click := func(selector string) {
		t.Helper()
		element := page.MustElement(selector)
		if !element.MustEval(`function(){const r=this.getBoundingClientRect();return this.contains(document.elementFromPoint(r.x+r.width/2,r.y+r.height/2));}`).Bool() {
			t.Fatalf("%s is covered", selector)
		}
		point := element.MustEval(`function(){const r=this.getBoundingClientRect();return [r.x+r.width/2,r.y+r.height/2];}`).Arr()
		page.Mouse.MustMoveTo(point[0].Num(), point[1].Num()).MustClick(proto.InputMouseButtonLeft)
	}
	settle := func() {
		t.Helper()
		page.MustEval(`()=>new Promise(r=>setTimeout(r,150))`)
	}
	quiet := func(step string) {
		t.Helper()
		settle()
		result := page.MustEval(`()=>JSON.stringify({
            mutations:fixtureMutations,
            dialogs:document.querySelectorAll('.vd-modal-backdrop').length,
            windows:[...aurora.state.windows.values()].map(w=>w.appId),
            focus:document.activeElement?.className||document.activeElement?.tagName||''
        })`).Str()
		var got struct {
			Mutations []string `json:"mutations"`
			Dialogs   int      `json:"dialogs"`
			Windows   []string `json:"windows"`
			Focus     string   `json:"focus"`
		}
		if err := json.Unmarshal([]byte(result), &got); err != nil {
			t.Fatal(err)
		}
		if len(got.Mutations) != 0 || got.Dialogs != 0 || len(got.Windows) != 1 || got.Windows[0] != "calculator" {
			t.Fatalf("%s acted on the desktop icon (focus %q): %s", step, got.Focus, result)
		}
	}
	const icon = `.vd-icon[data-path="Welcome.md"]`
	selectIconThenOpenCalculator := func() {
		t.Helper()
		page.MustEval(`()=>{[...aurora.state.windows.keys()].forEach(id=>aurora.closeWindow(id));fixtureMutations.length=0;}`)
		click(icon)
		page.MustEval(`async()=>{
            await fixtureOpen('calculator');
            const win=[...aurora.state.windows.values()].at(-1).element;
            Object.assign(win.style,{left:'640px',top:'60px'});
        }`)
		waitForJSBool(t, page, `()=>document.querySelector('.vd-window [data-mode="scientific"]')!==null && document.querySelector('`+icon+`').classList.contains('selected')`)
	}

	selectIconThenOpenCalculator()
	// A focused button inside the app's own content.
	page.MustEval(`()=>document.querySelector('.vd-window [data-mode="scientific"]').focus()`)
	for _, key := range []input.Key{input.Delete, input.F2} {
		page.Keyboard.MustType(key)
		quiet("app content button + " + key.Info().Key)
	}
	// A focused window control outside the app's own keyboard handling.
	page.MustEval(`()=>document.querySelector('.vd-window .vd-window-button[data-action="close"]').focus()`)
	for _, key := range []input.Key{input.Delete, input.F2} {
		page.Keyboard.MustType(key)
		quiet("titlebar button + " + key.Info().Key)
	}
	// A press on window content that then drops focus to the page body, as when
	// an app re-renders the element that held focus.
	click(`.vd-window [data-result]`)
	page.MustEval(`()=>document.activeElement?.blur()`)
	if page.MustEval(`()=>document.activeElement===document.body`).Bool() == false {
		t.Fatal("focus did not fall back to the page body")
	}
	for _, key := range []input.Key{input.Enter, input.Delete, input.F2} {
		page.Keyboard.MustType(key)
		quiet("body after window press + " + key.Info().Key)
	}

	// Global shell shortcuts keep working from inside a window.
	page.MustEval(`()=>document.querySelector('.vd-window .vd-window-button[data-action="close"]').focus()`)
	page.Keyboard.MustType(input.F1)
	waitForJSBool(t, page, `()=>!!document.getElementById('vd-shortcuts-help')`)
	page.MustEval(`()=>document.getElementById('vd-shortcuts-help').remove()`)
	page.MustEval(`()=>document.querySelector('.vd-window .vd-window-button[data-action="close"]').focus()`)
	page.KeyActions().Press(input.ControlLeft).Type('k').MustDo()
	waitForJSBool(t, page, `()=>!!document.getElementById('vd-spotlight-backdrop')`)
	page.Keyboard.MustType(input.Escape)
	waitForJSBool(t, page, `()=>!document.getElementById('vd-spotlight-backdrop')`)
	page.MustEval(`()=>document.querySelector('.vd-window .vd-window-button[data-action="close"]').focus()`)
	page.Keyboard.MustType(input.F11)
	waitForJSBool(t, page, `()=>aurora.state.windows.get(aurora.state.activeWindowId)?.maximized===true`)
	page.MustEval(`()=>document.querySelector('.vd-window .vd-window-button[data-action="close"]').focus()`)
	page.KeyActions().Press(input.AltLeft).Type(input.F4).MustDo()
	waitForJSBool(t, page, `()=>aurora.state.windows.size===0`)

	// The desktop keeps its own shortcuts: a focused icon renames and deletes.
	selectIconThenOpenCalculator()
	click(icon)
	page.Keyboard.MustType(input.F2)
	waitForJSBool(t, page, `()=>document.querySelectorAll('.vd-modal-backdrop').length===1`)
	page.Keyboard.MustType(input.Escape)
	waitForJSBool(t, page, `()=>document.querySelectorAll('.vd-modal-backdrop').length===0 && document.activeElement?.matches('`+icon+`')`)
	// A key that another handler already consumed never reaches the icon.
	page.MustEval(`()=>document.addEventListener('keydown',e=>{if(e.key==='Delete')e.preventDefault();},{capture:true,once:true})`)
	page.Keyboard.MustType(input.Delete)
	settle()
	if got := page.MustEval(`()=>JSON.stringify(fixtureMutations)`).Str(); got != "[]" {
		t.Fatalf("a consumed Delete reached the desktop icon: %s", got)
	}
	page.Keyboard.MustType(input.Delete)
	waitForJSBool(t, page, `()=>fixtureMutations.length===1 && fixtureMutations[0]==='DELETE /api/desktop/file?path=Welcome.md'`)
	// So does the page body after a press on the desktop (marquee or Select All).
	page.MustEval(`()=>{fixtureMutations.length=0;}`)
	waitForJSBool(t, page, `()=>!!document.querySelector('`+icon+`')`)
	click(icon)
	page.MustEval(`()=>document.activeElement?.blur()`)
	page.Keyboard.MustType(input.Delete)
	waitForJSBool(t, page, `()=>fixtureMutations.length===1 && fixtureMutations[0]==='DELETE /api/desktop/file?path=Welcome.md'`)

	if errors := page.MustEval(`()=>JSON.stringify(fixtureErrors)`).Str(); errors != "[]" {
		t.Fatal(errors)
	}
}
