package ui

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"aurago/internal/desktop"
	"github.com/go-rod/rod/lib/proto"
)

func TestDesktopSessionRestoreBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	html := regexp.MustCompile(`(?s)<script\b[^>]*>.*?</script>`).ReplaceAllString(readDesktopAssetText(t, "desktop.html"), "")
	html = regexp.MustCompile(`\{\{[^}]*\}\}`).ReplaceAllString(html, "")
	html = strings.Replace(html, "</body>", `<script>
window.fixtureErrors=[];
addEventListener('error', e=>fixtureErrors.push(e.message));
addEventListener('unhandledrejection', e=>fixtureErrors.push(String(e.reason)));
window.t=key=>key;
window.WebSocket=class extends EventTarget { close(){} };
</script><script src="/js/shared/lazy-assets.js"></script><script src="/js/desktop/core/module-loader.js"></script><script src="/session-shell.js"></script></body>`, 1)
	shell := readDesktopAssetText(t, "js/desktop/bundles/main.bundle.js")
	cut := strings.LastIndex(shell, "    ensureDesktopRadialMenuAnchor();")
	if cut < 0 {
		t.Fatal("desktop startup seam missing")
	}
	// Keep the real startup sequence, including restoration and URL app routing.
	shell = shell[:cut] + `window.sessionTest={state,openApp,closeWindow,focusWindow,minimizeWindow,toggleMaximizeWindow,assignWindowSpace,switchSpace,captureSessionSnapshot,setDockPins,setDefaultAppForExtension};` + shell[cut:]
	settings := desktop.DesktopSettingDefaults()
	for key, value := range map[string]string{"windows.restore_session": "false", "windows.animations": "false", "pet.enabled": "false", "phone_gadget.enabled": "false", "desktop.show_widgets": "false", "appearance.theme": "fruity", "appearance.fruity_mode": "dark"} {
		settings[key] = value
	}
	var mu sync.Mutex
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(Content)))
	mux.HandleFunc("/fixture", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, html) })
	mux.HandleFunc("/session-shell.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/javascript")
		fmt.Fprint(w, shell)
	})
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/desktop/bootstrap":
			json.NewEncoder(w).Encode(map[string]interface{}{"enabled": true, "builtin_apps": desktop.BuiltinApps(), "installed_apps": []interface{}{}, "widgets": []interface{}{}, "shortcuts": []interface{}{}, "desktop_files": []interface{}{}, "workspace": map[string]interface{}{"readonly": false}, "settings": settings})
		case "/api/desktop/settings":
			if r.Method == http.MethodPut {
				var update struct{ Key, Value string }
				if err := json.NewDecoder(r.Body).Decode(&update); err != nil || update.Key == "" {
					http.Error(w, `{"error":"invalid setting"}`, http.StatusBadRequest)
					return
				}
				settings[update.Key] = update.Value
			}
			json.NewEncoder(w).Encode(map[string]interface{}{"settings": settings})
		default:
			fmt.Fprint(w, `{"status":"ok","files":[],"pets":[],"settings":{},"enabled":false}`)
		}
	})
	server := httptest.NewServer(mux)
	defer server.Close()
	page := newSmokeBrowser(t).MustPage().Timeout(50 * time.Second)
	defer page.Close()
	page.MustSetViewport(1440, 1000, 1, false)
	waitBoot := func() {
		t.Helper()
		page.MustWaitLoad()
		page.MustWait(`()=>!!(window.sessionTest?.state.ws || window.fixtureErrors?.length)`)
		if errors := page.MustEval(`()=>JSON.stringify(fixtureErrors)`).Str(); errors != "[]" {
			t.Fatal(errors)
		}
	}
	snapshot := func() string {
		t.Helper()
		return page.MustEval(`()=>{const s=sessionTest.captureSessionSnapshot();s.windows.forEach(w=>delete w.z);s.windows.sort((a,b)=>a.appId.localeCompare(b.appId)||a.left-b.left);return JSON.stringify(s);}`).Str()
	}
	toggleRestore := func() {
		t.Helper()
		label := page.MustElement(`label:has([data-setting-key="windows.restore_session"])`)
		point := label.MustEval(`function(){const r=this.getBoundingClientRect();if(!this.contains(document.elementFromPoint(r.x+r.width/2,r.y+r.height/2)))throw Error('restore toggle is covered');return [r.x+r.width/2,r.y+r.height/2];}`).Arr()
		page.Mouse.MustMoveTo(point[0].Num(), point[1].Num()).MustClick(proto.InputMouseButtonLeft)
	}
	page.MustNavigate(server.URL + "/fixture")
	waitBoot()
	page.MustEval(`async()=>{
        await AuraDesktopModules.loadAppAssets('settings');
        await AuraDesktopModules.loadAppAssets('calculator');
        sessionTest.openApp('settings',{category:'windows'});
        sessionTest.openApp('calculator');
        const first=[...sessionTest.state.windows.values()].at(-1);
        Object.assign(first.element.style,{left:'120px',top:'90px',width:'470px',height:'790px'});
        sessionTest.minimizeWindow(first.id);
        await new Promise(r=>setTimeout(r,10));
        sessionTest.openApp('calculator',{forceNew:true});
        const second=[...sessionTest.state.windows.values()].at(-1);
        Object.assign(second.element.style,{left:'440px',top:'110px',width:'480px',height:'800px'});
        sessionTest.assignWindowSpace(second,'2');
        sessionTest.toggleMaximizeWindow(second.id);
        sessionTest.focusWindow([...sessionTest.state.windows.keys()][0]);
    }`)
	// Toggle the actual settings control; enabling must capture already-open windows.
	toggleRestore()
	page.MustWait(`()=>!!sessionTest.state.bootstrap.settings['session.windows']`)
	expected := snapshot()
	mu.Lock()
	persisted := settings["session.windows"]
	mu.Unlock()
	if !strings.Contains(persisted, `"appId":"calculator"`) {
		t.Fatalf("enabling restoration did not save open windows: %s", persisted)
	}
	page.MustReload()
	waitBoot()
	if actual := snapshot(); actual != expected {
		t.Fatalf("reload changed windows/bounds/state:\nwant %s\ngot  %s", expected, actual)
	}
	// A deep link must retain the restored session and reuse its settings window.
	page.MustNavigate(server.URL + "/fixture?app=settings")
	waitBoot()
	if actual := snapshot(); actual != expected {
		t.Fatalf("app deep link lost the session:\nwant %s\ngot  %s", expected, actual)
	}
	// Reload immediately after a change, while the 800 ms debounce is pending.
	page.MustEval(`()=>{
        const w=[...sessionTest.state.windows.values()].find(w=>w.appId==='calculator'&&w.maximized);
        sessionTest.toggleMaximizeWindow(w.id);
        if(w.element.style.left!=='440px'||w.element.style.top!=='110px')throw Error('maximized window lost its normal position');
        Object.assign(w.element.style,{left:'360px',top:'80px',width:'490px',height:'810px'});
        sessionTest.switchSpace('2');
    }`)
	expected = snapshot()
	page.MustNavigate(server.URL + "/fixture")
	waitBoot()
	if actual := snapshot(); actual != expected {
		t.Fatalf("reload lost pending geometry or active space:\nwant %s\ngot  %s", expected, actual)
	}
	// The same shell writer is used by dock pins and file associations.
	page.MustEval(`async()=>{await sessionTest.setDockPins(['calculator','settings']);await sessionTest.setDefaultAppForExtension('txt','editor');}`)
	mu.Lock()
	dock, defaults := settings["appearance.dock_pins"], settings["files.default_apps"]
	mu.Unlock()
	if dock != `["calculator","settings"]` || defaults != `{"txt":"editor"}` {
		t.Fatalf("shared settings writer failed: dock=%s defaults=%s", dock, defaults)
	}
	// Disabling the setting must survive reload and leave the desktop empty.
	page.MustEval(`()=>{sessionTest.switchSpace('1');sessionTest.openApp('settings',{category:'windows'});}`)
	toggleRestore()
	page.MustWait(`()=>sessionTest.state.bootstrap.settings['windows.restore_session']==='false'`)
	page.MustReload()
	waitBoot()
	if count := page.MustEval(`()=>sessionTest.state.windows.size`).Int(); count != 0 {
		t.Fatalf("restore disabled but reopened %d windows", count)
	}
}
