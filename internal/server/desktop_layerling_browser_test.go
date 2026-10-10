package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"aurago/internal/i18n"
	"aurago/internal/layerling"
	"aurago/ui"
	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
)

// Exercises the real Desktop loader/channel, real CAD/WASM, broker and rooted files.
func TestLayerlingDesktopBrowser(t *testing.T) {
	if os.Getenv("AURAGO_RUN_BROWSER_SMOKE") != "1" {
		t.Skip("set AURAGO_RUN_BROWSER_SMOKE=1")
	}
	var bin string
	for _, candidate := range []string{`C:\Program Files\Google\Chrome\Application\chrome.exe`, `C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe`, "chromium"} {
		if p, e := exec.LookPath(candidate); e == nil {
			bin = p
			break
		}
	}
	if bin == "" {
		t.Skip("Chrome or Edge required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()
	control, err := launcher.New().Context(ctx).Bin(bin).Headless(true).NoSandbox(true).Set("enable-unsafe-swiftshader").Launch()
	if err != nil {
		t.Fatal(err)
	}
	browser := rod.New().ControlURL(control)
	if err = browser.Connect(); err != nil {
		t.Fatal(err)
	}
	defer browser.Close()
	s := newDesktopFilesystemTestServer(t)
	s.Cfg.VirtualDesktop.Layerling.Enabled = true
	s.Cfg.VirtualDesktop.Layerling.AgentAccess = "write"
	s.Cfg.VirtualDesktop.AllowAgentControl = true
	s.Cfg.Tools.VirtualDesktop.Enabled = true
	i18n.Load(ui.Content, slog.Default())
	read := func(p string) string {
		b, e := ui.Content.ReadFile(p)
		if e != nil {
			t.Fatal(e)
		}
		return string(b)
	}
	html := regexp.MustCompile(`(?s)<script\b[^>]*>.*?</script>`).ReplaceAllString(read("desktop.html"), "")
	html = regexp.MustCompile(`\{\{[^}]*\}\}`).ReplaceAllString(html, "")
	scripts := `<script type="application/json" id="aurago-template-data">__DATA__</script><script src="/js/shared/template-data.js"></script><script>window._auragoSharedInitialized=true;</script><script src="/js/shared/shared-core.js"></script><script src="/js/shared/lazy-assets.js"></script><script src="/js/desktop/core/module-loader.js"></script><script src="/layerling-shell.js"></script><script src="/layerling-fixture.js"></script>`
	html = strings.Replace(html, "</body>", scripts+"</body>", 1)
	shell := read("js/desktop/bundles/main.bundle.js")
	cut := strings.LastIndex(shell, "    ensureDesktopRadialMenuAnchor();")
	if cut < 0 {
		t.Fatal("shell seam missing")
	}
	shell = shell[:cut] + `ensureSDKFrameLifecycleObserver();window.addEventListener('message',handleSDKChannelHandshake);const oldOfficeContext=officeAppContext;officeAppContext=c=>({...oldOfficeContext(c),confirmDialog:async()=>true,loadBootstrap:async()=>{}});window.cadTest={state,openApp,loadIconManifest,closeWindow,buildWindowAIContext};})();`
	mux := http.NewServeMux()
	h := registerLayerlingRoutes(mux, s)
	h.assets = os.DirFS("../../ui/js/vendor/layerling")
	mux.Handle("/", http.FileServer(http.FS(ui.Content)))
	mux.HandleFunc("/fixture", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		data := uiTemplateData("de", "desktop")
		fmt.Fprint(w, strings.Replace(html, "__DATA__", fmt.Sprint(data["TemplateDataJSON"]), 1))
	})
	mux.HandleFunc("/api/i18n", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"data":%s}`, getI18NJSONForSections(normalizeLang(r.URL.Query().Get("lang")), strings.Split(r.URL.Query().Get("sections"), ",")...))
	})
	mux.HandleFunc("/layerling-shell.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/javascript")
		fmt.Fprint(w, shell)
	})
	mux.HandleFunc("/layerling-fixture.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/javascript")
		fmt.Fprint(w, layerlingBrowserFixture)
	})
	origin := httptest.NewServer(mux)
	defer origin.Close()
	page := browser.MustPage().Timeout(7 * time.Minute)
	defer page.Close()
	var networkMu sync.Mutex
	var external []string
	networkCtx, networkCancel := context.WithCancel(ctx)
	defer networkCancel()
	go page.Context(networkCtx).EachEvent(func(event *proto.NetworkRequestWillBeSent) {
		url := event.Request.URL
		if (strings.HasPrefix(url, "http:") || strings.HasPrefix(url, "https:")) && !strings.HasPrefix(url, origin.URL+"/") {
			networkMu.Lock()
			external = append(external, url)
			networkMu.Unlock()
		}
	})()
	page.MustSetViewport(1440, 1000, 1, false)
	page.MustNavigate(origin.URL + "/fixture").MustWaitLoad()
	deadline := time.Now().Add(70 * time.Second)
	for time.Now().Before(deadline) {
		if page.MustEval(`()=>!!window.LayerlingApp && [...cadTest.state.windows.values()].some(w=>LayerlingApp.editorId(w.id))`).Bool() {
			break
		}
		time.Sleep(150 * time.Millisecond)
	}
	owner := layerling.WithOwner(ctx, h.broker, "local-desktop", func() bool { return true })
	list, e := layerling.Execute(owner, "list_editors", "", json.RawMessage(`{}`))
	if e != nil {
		t.Fatal(e)
	}
	var editors []struct {
		ID       string `json:"editor_id"`
		WindowID string `json:"window_id"`
	}
	json.Unmarshal(list, &editors)
	if len(editors) != 1 {
		t.Fatalf("editor startup: %s; errors %s; status %s; frame %s", list, page.MustEval(`()=>JSON.stringify(cadErrors)`).Str(), page.MustEval(`()=>document.querySelector('.vd-layerling-status')?.textContent`).Str(), page.MustEval(`()=>document.querySelector('.vd-layerling-frame')?.contentDocument?.body?.innerText?.slice(0,1200)||'no frame'`).Str())
	}
	id := editors[0].ID
	firstID, firstWindow := id, editors[0].WindowID
	if !page.MustEval(`id=>cadTest.state.windows.get(id).maximized`, firstWindow).Bool() {
		t.Fatal("first start not maximized")
	}
	command := func(op string, args interface{}) map[string]interface{} {
		t.Helper()
		raw, _ := json.Marshal(args)
		result, err := layerling.Execute(owner, op, id, raw)
		if err != nil {
			t.Fatalf("%s: %v; browser %s", op, err, page.MustEval(`()=>JSON.stringify(cadErrors)`).Str())
		}
		var v map[string]interface{}
		if json.Unmarshal(result, &v) != nil {
			t.Fatalf("%s result: %s", op, result)
		}
		return v
	}
	scene := func() map[string]interface{} { return command("get_scene", map[string]interface{}{}) }
	if scene()["shapeCount"] != float64(0) {
		t.Fatal("new editor not empty")
	}
	page.MustEval(`id=>cadTest.state.windows.get(id).element.querySelector('[data-action="save"]').click()`, firstWindow)
	page.MustElement("[data-file-dialog-filename]").MustSelectAllText().MustInput("First.lyl")
	page.MustElement("[data-file-dialog-confirm]").MustClick()
	page.MustWait(`()=>[...cadTest.state.windows.values()].some(w=>w.context?.path==='Documents/Layerling/First.lyl')`)
	command("create_shape", map[string]interface{}{"kind": "box", "name": "Block", "width": 30, "depth": 20, "height": 10})
	state := scene()
	shapes, ok := state["shapes"].([]interface{})
	if !ok || len(shapes) != 1 {
		t.Fatalf("shape creation: %v", state)
	}
	block := shapes[0].(map[string]interface{})["id"].(string)
	command("update_object", map[string]interface{}{"id": block, "width": 32})
	command("create_shape", map[string]interface{}{"kind": "cylinder", "name": "Bore", "width": 5, "depth": 5, "height": 20, "elevation": -5})
	shapes = scene()["shapes"].([]interface{})
	var bore string
	for _, shape := range shapes {
		v := shape.(map[string]interface{})
		if v["name"] == "Bore" {
			bore = v["id"].(string)
		}
	}
	command("boolean_cut", map[string]interface{}{"solidIds": []string{block}, "holeIds": []string{bore}})
	shapes = scene()["shapes"].([]interface{})
	block = shapes[0].(map[string]interface{})["id"].(string)
	command("apply_edge_treatment", map[string]interface{}{"id": block, "kind": "fillet", "edgeIds": "top", "amount": 0.5})
	command("save_project", map[string]interface{}{"path": "Documents/Layerling/browser.lyl"})
	for _, format := range []string{"stl", "obj", "3mf", "step", "png"} {
		command("export_model", map[string]interface{}{"path": "Documents/Layerling/browser." + format, "format": format})
	}
	preview := command("capture_image", map[string]interface{}{})
	if !strings.HasPrefix(fmt.Sprint(preview["artifact"]), layerlingBase+"preview/") {
		t.Fatal("preview leaked image bytes")
	}
	page.MustEval(`()=>cadTest.openApp('layerling',{forceNew:true})`)
	deadline = time.Now().Add(40 * time.Second)
	for time.Now().Before(deadline) {
		list, _ = layerling.Execute(owner, "list_editors", "", json.RawMessage(`{}`))
		json.Unmarshal(list, &editors)
		if len(editors) == 2 {
			break
		}
		time.Sleep(150 * time.Millisecond)
	}
	if len(editors) != 2 {
		t.Fatalf("second editor: %s", list)
	}
	for _, editor := range editors {
		if editor.ID != id {
			id = editor.ID
			break
		}
	}
	if scene()["shapeCount"] != float64(0) {
		t.Fatal("editor state leaked")
	}
	command("open_project", map[string]interface{}{"path": "Documents/Layerling/browser.lyl"})
	if scene()["shapeCount"] == float64(0) {
		t.Fatal("saved project did not reopen")
	}
	secondID := id
	var secondWindow string
	for _, editor := range editors {
		if editor.ID == id {
			secondWindow = editor.WindowID
		}
	}
	// A stale agent save must fail; the user's conflict modal offers cancel and copy.
	id = firstID
	command("create_shape", map[string]interface{}{"kind": "box", "name": "Concurrent"})
	command("save_project", map[string]interface{}{"path": "Documents/Layerling/browser.lyl"})
	id = secondID
	command("create_shape", map[string]interface{}{"kind": "sphere", "name": "Other window"})
	if _, err := layerling.Execute(owner, "save_project", id, json.RawMessage(`{"path":"Documents/Layerling/browser.lyl"}`)); err == nil {
		t.Fatal("stale agent save overwrote project")
	}
	clickSave := func() {
		page.MustEval(`id=>cadTest.state.windows.get(id).element.querySelector('[data-action="save"]').click()`, secondWindow)
	}
	clickSave()
	page.MustElement(".vd-modal [data-cancel]").MustClick()
	clickSave()
	page.MustElement(".vd-modal [data-choice=copy]").MustClick()
	page.MustElement("[data-file-dialog-filename]").MustSelectAllText().MustInput("manual.lyl")
	page.MustElement("[data-file-dialog-confirm]").MustClick()
	page.MustWait(`()=>[...cadTest.state.windows.values()].some(w=>w.context?.path==='Documents/Layerling/manual.lyl')`)
	for _, format := range []string{"stl", "obj", "3mf", "step"} {
		command("import_model", map[string]interface{}{"path": "Documents/Layerling/browser." + format})
	}
	svg := `<svg xmlns="http://www.w3.org/2000/svg" width="20mm" height="20mm" viewBox="0 0 20 20"><path d="M0 0H20V20H0Z"/></svg>`
	page.MustEval(`async data=>{const r=await fetch('/api/desktop/layerling/file?path=Documents/Layerling/shape.svg',{method:'PUT',headers:{'If-None-Match':'*'},body:data});if(!r.ok)throw Error('SVG fixture');}`, svg)
	command("import_model", map[string]interface{}{"path": "Documents/Layerling/shape.svg"})
	command("save_project", map[string]interface{}{"path": "Documents/Layerling/imports.lyl"})
	for _, badPath := range []string{"Documents/Layerling/missing.lyl", "Documents/Layerling/broken.lyl"} {
		if strings.HasSuffix(badPath, "broken.lyl") {
			page.MustEval(`async()=>{const r=await fetch('/api/desktop/layerling/file?path=Documents/Layerling/broken.lyl',{method:'PUT',headers:{'If-None-Match':'*'},body:'invalid project'});if(!r.ok)throw Error('Invalid project fixture');}`)
		}
		args, _ := json.Marshal(map[string]string{"path": badPath})
		if _, err := layerling.Execute(owner, "open_project", id, args); err == nil {
			t.Fatal("invalid project accepted", badPath)
		}
	}
	if !page.MustEval(`async()=>{for(const file of ['guide/','anleitung/']){const r=await fetch('/api/desktop/layerling/ui/'+file);if(!r.ok||!(await r.text()).includes('/api/desktop/layerling/ui/assets/'))return false;}return (await navigator.serviceWorker.getRegistrations()).length===0;}`).Bool() {
		t.Fatal("offline guides or service worker isolation failed")
	}
	// Real close guards finish saving; reopening must preserve the imported scene.
	count := scene()["shapeCount"]
	page.MustEval(`async id=>{await cadTest.closeWindow(id);}`, secondWindow)
	page.MustEval(`()=>cadTest.openApp('layerling',{path:'Documents/Layerling/imports.lyl',forceNew:true})`)
	page.MustWait(`()=>[...cadTest.state.windows.values()].some(w=>w.context?.path==='Documents/Layerling/imports.lyl'&&LayerlingApp.editorId(w.id))`)
	id = page.MustEval(`()=>{const w=[...cadTest.state.windows.values()].find(w=>w.context?.path==='Documents/Layerling/imports.lyl');return LayerlingApp.editorId(w.id);}`).Str()
	if scene()["shapeCount"] != count {
		t.Fatal("close/reopen lost shapes")
	}
	// Restore an unsaved browser draft using the persisted Desktop window key.
	page.MustEval(`()=>cadTest.openApp('layerling',{forceNew:true,sessionRestore:{key:'cad-recovery-fixture'}})`)
	page.MustWait(`()=>[...cadTest.state.windows.values()].some(w=>w.sessionKey==='cad-recovery-fixture'&&LayerlingApp.editorId(w.id))`)
	id = page.MustEval(`()=>LayerlingApp.editorId([...cadTest.state.windows.values()].find(w=>w.sessionKey==='cad-recovery-fixture').id)`).Str()
	command("create_shape", map[string]interface{}{"kind": "box", "name": "Recovery"})
	page.MustEval(`async()=>{await cadTest.closeWindow([...cadTest.state.windows.values()].find(w=>w.sessionKey==='cad-recovery-fixture').id);}`)
	page.MustEval(`()=>cadTest.openApp('layerling',{forceNew:true,sessionRestore:{key:'cad-recovery-fixture'}})`)
	page.MustWait(`()=>[...cadTest.state.windows.values()].some(w=>w.sessionKey==='cad-recovery-fixture'&&LayerlingApp.editorId(w.id))`)
	id = page.MustEval(`()=>LayerlingApp.editorId([...cadTest.state.windows.values()].find(w=>w.sessionKey==='cad-recovery-fixture').id)`).Str()
	if scene()["shapeCount"] != float64(1) {
		t.Fatal("draft recovery failed")
	}
	page.MustEval(`async()=>{await cadTest.closeWindow([...cadTest.state.windows.values()].find(w=>w.sessionKey==='cad-recovery-fixture').id);}`)
	page.MustEval(`()=>cadTest.openApp('layerling',{forceNew:true,sessionRestore:{key:'cad-recovery-picker'}})`)
	page.MustWait(`()=>[...cadTest.state.windows.values()].some(w=>w.sessionKey==='cad-recovery-picker'&&LayerlingApp.editorId(w.id))`)
	id = page.MustEval(`()=>LayerlingApp.editorId([...cadTest.state.windows.values()].find(w=>w.sessionKey==='cad-recovery-picker').id)`).Str()
	page.MustEval(`()=>[...cadTest.state.windows.values()].find(w=>w.sessionKey==='cad-recovery-picker').element.querySelector('[data-action="recover"]').click()`)
	page.MustElement(".vd-modal [data-choice]").MustClick()
	page.MustWait(`()=>![...document.querySelectorAll('.vd-layerling-status')].some(el=>el.textContent.includes('geladen'))`)
	if scene()["shapeCount"] != float64(1) {
		t.Fatal("closed-window draft picker failed")
	}
	// All themes and viewport sizes exercise the actual embedded app.
	if err = os.MkdirAll("../../reports/layerling", 0700); err != nil {
		t.Fatal(err)
	}
	for _, theme := range []string{"standard", "fruity"} {
		for _, mode := range []string{"light", "dark"} {
			for _, width := range []int{1440, 760} {
				page.MustSetViewport(width, 900, 1, false)
				page.MustEval(`(theme,mode)=>{document.body.dataset.theme=theme;document.body.dataset.fruityMode=mode;}`, theme, mode)
				want := "dark"
				if theme == "fruity" && mode == "light" {
					want = "light"
				}
				page.MustWait(`theme=>[...document.querySelectorAll('.vd-layerling-frame')].every(f=>f.contentDocument.documentElement.dataset.theme===theme)`, want)
				if !page.MustEval(`()=>[...document.querySelectorAll('.vd-layerling')].every(el=>el.scrollWidth<=el.clientWidth+1)`).Bool() {
					t.Fatal("horizontal overflow", theme, mode, width)
				}
				page.MustScreenshot(fmt.Sprintf("../../reports/layerling/%s-%s-%d.png", theme, mode, width))
			}
		}
	}
	page.MustEval(`()=>document.documentElement.lang='fr'`)
	page.MustWait(`()=>[...document.querySelectorAll('.vd-layerling-frame')].every(f=>f.contentDocument.documentElement.lang==='en')`)
	page.MustEval(`()=>document.documentElement.lang='de'`)
	page.MustWait(`()=>[...document.querySelectorAll('.vd-layerling-frame')].every(f=>f.contentDocument.documentElement.lang==='de')`)
	// Unsupported GPU browsers retain a clear local message and no unusable editor.
	page.MustEval(`()=>{window.cadGetContext=HTMLCanvasElement.prototype.getContext;HTMLCanvasElement.prototype.getContext=function(kind,...args){return kind==='webgl2'?null:cadGetContext.call(this,kind,...args)};cadTest.openApp('layerling',{forceNew:true});}`)
	page.MustWait(`()=>[...document.querySelectorAll('.vd-layerling-status')].some(el=>el.textContent.includes('WebGL 2'))`)
	page.MustEval(`()=>HTMLCanvasElement.prototype.getContext=cadGetContext`)
	networkMu.Lock()
	defer networkMu.Unlock()
	if len(external) != 0 {
		t.Fatalf("external resource requests: %v", external)
	}
	if page.MustEval(`()=>cadErrors.length`).Int() != 0 {
		t.Fatalf("browser errors: %s", page.MustEval(`()=>JSON.stringify(cadErrors)`).Str())
	}
}

const layerlingBrowserFixture = `
window.cadErrors=[];addEventListener('error',e=>cadErrors.push(e.message));addEventListener('unhandledrejection',e=>cadErrors.push(String(e.reason)));
const realFetch=window.fetch.bind(window);window.fetch=(url,options)=>String(url).startsWith('/api/')&&!String(url).startsWith('/api/desktop/layerling/')&&!String(url).startsWith('/api/i18n?')?Promise.resolve(new Response('{}')):realFetch(url,options);
(async()=>{document.documentElement.lang='de';cadTest.state.bootstrap={enabled:true,builtin_apps:[{id:'layerling',name:'Layerling',icon:'layerling',metadata:{open_maximized:'true'}}],apps:[],widgets:[],shortcuts:[],desktop_files:[],settings:{'appearance.theme':'standard','windows.restore_session':false}};document.body.dataset.theme='standard';document.body.dataset.animations='false';document.getElementById('vd-disabled').hidden=true;await cadTest.loadIconManifest();cadTest.openApp('layerling');})();
`
