package ui

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/gorilla/websocket"
)

// codeStudioFixtureBackend is an in-memory stand-in for the Docker-backed
// Code Studio API so the real shell, bundle, CodeMirror and xterm can run
// headless without a container.
type codeStudioFixtureBackend struct {
	mu    sync.Mutex
	files map[string]string
	dirs  map[string]bool
	execs []string
	chats []map[string]interface{}
}

func newCodeStudioFixtureBackend() *codeStudioFixtureBackend {
	return &codeStudioFixtureBackend{
		files: map[string]string{
			"/workspace/main.go":             "package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Println(\"hello from code studio\")\n}\n",
			"/workspace/README.md":           "# Demo\n\nEin kleines Projekt.\n",
			"/workspace/package.json":        "{\n  \"name\": \"demo\",\n  \"version\": \"1.0.0\"\n}\n",
			"/workspace/styles.css":          ".app { color: red; }\n",
			"/workspace/src/app.js":          "export function greet(name) {\n  return 'Hi ' + name;\n}\n",
			"/workspace/src/util/helpers.py": "def add(a, b):\n    return a + b\n",
			"/workspace/src/util/format.py":  "def fmt(v):\n    return str(v)\n",
		},
		dirs: map[string]bool{"/workspace": true, "/workspace/src": true, "/workspace/src/util": true, "/workspace/docs": true},
	}
}

func (b *codeStudioFixtureBackend) parent(path string) string {
	idx := strings.LastIndex(path, "/")
	if idx <= 0 {
		return "/"
	}
	return path[:idx]
}

func (b *codeStudioFixtureBackend) list(dir string) []map[string]interface{} {
	entries := []map[string]interface{}{}
	for path, content := range b.files {
		if b.parent(path) == dir {
			entries = append(entries, map[string]interface{}{"name": filepath.Base(path), "path": path, "type": "file", "size": len(content), "modified": time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)})
		}
	}
	for path := range b.dirs {
		if path != dir && b.parent(path) == dir {
			entries = append(entries, map[string]interface{}{"name": filepath.Base(path), "path": path, "type": "directory", "size": 0, "modified": time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)})
		}
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i]["path"].(string) < entries[j]["path"].(string) })
	return entries
}

func (b *codeStudioFixtureBackend) register(mux *http.ServeMux) {
	writeJSON := func(w http.ResponseWriter, v interface{}) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(v)
	}
	mux.HandleFunc("/api/i18n", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]interface{}{"data": map[string]string{}})
	})
	mux.HandleFunc("/api/code-studio/status", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]interface{}{"status": "ok", "code_studio": map[string]interface{}{"enabled": true, "running": true}})
	})
	mux.HandleFunc("/api/code-studio/files", func(w http.ResponseWriter, r *http.Request) {
		b.mu.Lock()
		defer b.mu.Unlock()
		dir := r.URL.Query().Get("path")
		if dir == "" {
			dir = "/workspace"
		}
		if !b.dirs[dir] {
			w.WriteHeader(400)
			writeJSON(w, map[string]string{"error": "no such directory: " + dir})
			return
		}
		writeJSON(w, map[string]interface{}{"status": "ok", "path": dir, "files": b.list(dir)})
	})
	mux.HandleFunc("/api/code-studio/file", func(w http.ResponseWriter, r *http.Request) {
		b.mu.Lock()
		defer b.mu.Unlock()
		switch r.Method {
		case http.MethodGet:
			path := r.URL.Query().Get("path")
			content, ok := b.files[path]
			if !ok {
				w.WriteHeader(404)
				writeJSON(w, map[string]string{"error": "no such file: " + path})
				return
			}
			writeJSON(w, map[string]interface{}{"status": "ok", "path": path, "content": content})
		case http.MethodPut:
			var body struct {
				Path, Content string
				CreateOnly    bool `json:"create_only"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			if _, exists := b.files[body.Path]; body.CreateOnly && (exists || b.dirs[body.Path]) {
				w.WriteHeader(http.StatusConflict)
				writeJSON(w, map[string]string{"error": "file already exists"})
				return
			}
			b.files[body.Path] = body.Content
			writeJSON(w, map[string]interface{}{"status": "ok"})
		case http.MethodPatch:
			var body struct {
				OldPath string `json:"old_path"`
				NewPath string `json:"new_path"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			if content, ok := b.files[body.OldPath]; ok {
				delete(b.files, body.OldPath)
				b.files[body.NewPath] = content
			}
			writeJSON(w, map[string]interface{}{"status": "ok"})
		case http.MethodDelete:
			path := r.URL.Query().Get("path")
			delete(b.files, path)
			delete(b.dirs, path)
			writeJSON(w, map[string]interface{}{"status": "ok"})
		}
	})
	mux.HandleFunc("/api/code-studio/directory", func(w http.ResponseWriter, r *http.Request) {
		b.mu.Lock()
		defer b.mu.Unlock()
		var body struct{ Path string }
		_ = json.NewDecoder(r.Body).Decode(&body)
		b.dirs[body.Path] = true
		writeJSON(w, map[string]interface{}{"status": "ok"})
	})
	mux.HandleFunc("/api/code-studio/exec", func(w http.ResponseWriter, r *http.Request) {
		b.mu.Lock()
		defer b.mu.Unlock()
		var body struct{ Command string }
		_ = json.NewDecoder(r.Body).Decode(&body)
		b.execs = append(b.execs, body.Command)
		writeJSON(w, map[string]interface{}{"status": "ok", "output": "hello from code studio\n", "exit_code": 0})
	})
	mux.HandleFunc("/api/code-studio/search", func(w http.ResponseWriter, r *http.Request) {
		b.mu.Lock()
		defer b.mu.Unlock()
		q := strings.ToLower(r.URL.Query().Get("q"))
		results := []map[string]interface{}{}
		for path, content := range b.files {
			for i, line := range strings.Split(content, "\n") {
				if q != "" && strings.Contains(strings.ToLower(line), q) {
					results = append(results, map[string]interface{}{"path": path, "line": i + 1, "preview": strings.TrimSpace(line)})
				}
			}
		}
		writeJSON(w, map[string]interface{}{"status": "ok", "results": results})
	})
	mux.HandleFunc("/api/code-studio/git/status", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]interface{}{"status": "ok", "branch": "main", "changes": []map[string]string{{"status": "M", "path": "main.go"}, {"status": "??", "path": "src/util/format.py"}}, "log": []map[string]string{{"hash": "abc1234", "message": "initial commit"}}})
	})
	mux.HandleFunc("/api/code-studio/git/diff", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]interface{}{"status": "ok", "diff": "diff --git a/main.go b/main.go\n--- a/main.go\n+++ b/main.go\n@@ -1,3 +1,4 @@\n package main\n+// changed\n-old line\n"})
	})
	mux.HandleFunc("/api/code-studio/git/commit", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]interface{}{"status": "ok", "hash": "deadbeefcafe"})
	})
	mux.HandleFunc("/api/desktop/chat", func(w http.ResponseWriter, r *http.Request) {
		b.mu.Lock()
		defer b.mu.Unlock()
		var body map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&body)
		b.chats = append(b.chats, body)
		writeJSON(w, map[string]interface{}{"answer": "Hier ist ein Vorschlag:\n\n```go\npackage main\n\n// # not a heading\nfunc main() {\n\tx := a * b * c\n}\n```\n\n- Punkt eins\n- Punkt *zwei*\n\n1. erstens\n2. zweitens"})
	})
	upgrader := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
	mux.HandleFunc("/api/code-studio/terminal", func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		_ = conn.WriteMessage(websocket.TextMessage, []byte("dev@code-studio:/workspace$ "))
		for {
			_, payload, err := conn.ReadMessage()
			if err != nil {
				return
			}
			if strings.HasPrefix(string(payload), "{") {
				continue
			}
			_ = conn.WriteMessage(websocket.TextMessage, payload)
		}
	})
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) { writeJSON(w, map[string]interface{}{}) })
}

const codeStudioBrowserFixture = `
window.fixtureErrors=[];
addEventListener('error',e=>fixtureErrors.push(e.error?.stack||e.message));
addEventListener('unhandledrejection',e=>fixtureErrors.push(String(e.reason)));
const consoleError=console.error.bind(console);console.error=(...args)=>{fixtureErrors.push('console.error: '+args.map(a=>a&&a.stack?a.stack:String(a)).join(' '));consoleError(...args);};
window.fixtureReady=(async()=>{
 const words=await (await fetch('/lang/desktop/de.json')).json();
 const tr=(key,vars)=>{let s=words[key]||key;Object.entries(vars||{}).forEach(([k,v])=>{s=s.split('{{'+k+'}}').join(String(v));});return s;};
 window.i18n={t:tr};window.t=tr;
 codeStudioTest.state.bootstrap={enabled:true,builtin_apps:[{id:'code-studio',name:'Code Studio',icon:'code-studio'}],apps:[],widgets:[],shortcuts:[],desktop_files:[],settings:{'appearance.theme':'standard','windows.restore_session':false,'windows.animations':false}};
 document.body.dataset.theme='standard';document.body.dataset.animations='false';
 document.getElementById('vd-disabled').hidden=true;
 await codeStudioTest.loadIconManifest();codeStudioTest.openApp('code-studio');
})();
window.csRoot=()=>document.querySelector('[data-code-studio]');
window.csState=()=>window.CodeStudioApp&&window.CodeStudioApp.state;
window.csKey=(target,init)=>{const el=target||document.activeElement||document.body;el.dispatchEvent(new KeyboardEvent('keydown',Object.assign({bubbles:true,cancelable:true},init)));};
window.csWait=(fn,ms)=>new Promise((resolve,reject)=>{const start=Date.now();const tick=()=>{let v;try{v=fn();}catch(e){}if(v)return resolve(v);if(Date.now()-start>(ms||5000))return reject(new Error('timeout waiting'));setTimeout(tick,40);};tick();});
window.csMenuAction=(menuId,itemId)=>{const id=codeStudioTest.state.windows.keys().next().value;const entry=codeStudioTest.state.windowMenus.get(id);const menus=(entry&&entry.rawMenus)||[];const menu=menus.find(m=>m.id===menuId);const item=menu&&menu.items.find(i=>i.id===itemId);if(item&&item.action){item.action();return true;}return false;};
`

func newCodeStudioBrowser(t *testing.T) (*rod.Page, *codeStudioFixtureBackend) {
	t.Helper()
	requirePrecisionBrowserSmoke(t)
	bin, ok := browserExecutable()
	if !ok {
		t.Skip("Chrome or Edge required")
	}
	html := readDesktopAssetText(t, "desktop.html")
	html = regexp.MustCompile(`(?s)<script\b[^>]*>.*?</script>`).ReplaceAllString(html, "")
	html = regexp.MustCompile(`\{\{[^}]*\}\}`).ReplaceAllString(html, "")
	html = strings.Replace(html, "</body>", `<script src="/js/desktop/core/module-loader.js"></script><script src="/code-studio-shell.js"></script><script src="/code-studio-fixture.js"></script></body>`, 1)
	shell := readDesktopAssetText(t, "js/desktop/bundles/main.bundle.js")
	cut := strings.LastIndex(shell, "    ensureDesktopRadialMenuAnchor();")
	if cut < 0 {
		t.Fatal("desktop startup seam missing")
	}
	shell = shell[:cut] + `window.codeStudioTest={state,openApp,loadIconManifest,closeWindow,applyDesktopSettings,renderTaskbar};})();`
	backend := newCodeStudioFixtureBackend()
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(Content)))
	backend.register(mux)
	for route, source := range map[string]string{"/fixture": html, "/code-studio-shell.js": shell, "/code-studio-fixture.js": codeStudioBrowserFixture} {
		route, source := route, source
		mux.HandleFunc(route, func(w http.ResponseWriter, r *http.Request) {
			if route == "/fixture" {
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
			} else {
				w.Header().Set("Content-Type", "text/javascript")
			}
			fmt.Fprint(w, source)
		})
	}
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	launch := launcher.New().Bin(bin).Headless(true).NoSandbox(true).Set("disable-gpu")
	browser := rod.New().ControlURL(launch.MustLaunch()).MustConnect()
	t.Cleanup(launch.Cleanup)
	t.Cleanup(browser.MustClose)
	page := browser.MustPage().Timeout(120 * time.Second)
	t.Cleanup(func() { _ = page.Close() })
	page.MustSetViewport(1440, 920, 1, false)
	page.MustNavigate(srv.URL + "/fixture").MustWaitLoad()
	page.MustEval(`async()=>{await fixtureReady;}`)
	return page, backend
}

func TestDesktopCodeStudioBrowser(t *testing.T) {
	page, backend := newCodeStudioBrowser(t)
	snapshot := func(name string) {
		t.Helper()
		if dir := os.Getenv("AURAGO_BROWSER_ARTIFACT_DIR"); dir != "" {
			if err := os.MkdirAll(dir, 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, name+".png"), page.MustScreenshot(), 0644); err != nil {
				t.Fatal(err)
			}
		}
	}
	check := func(name, js string) {
		t.Helper()
		result, err := page.Eval(js)
		if err != nil {
			t.Errorf("%s: eval failed: %v; browser errors: %s", name, err, page.MustEval(`()=>JSON.stringify(fixtureErrors)`).Str())
			return
		}
		if !result.Value.Bool() {
			t.Errorf("%s; browser errors: %s", name, page.MustEval(`()=>JSON.stringify(fixtureErrors)`).Str())
		}
	}
	if _, err := page.Eval(`async()=>{await csWait(()=>csRoot()&&csRoot().querySelectorAll('.cs-tree-item').length>0,20000);}`); err != nil {
		snapshot("cs-00-boot-failed")
		t.Fatalf("Code Studio did not open: %v; errors=%s; windows=%s; html=%s", err, page.MustEval(`()=>JSON.stringify(fixtureErrors)`).Str(), page.MustEval(`()=>JSON.stringify([...codeStudioTest.state.windows.keys()])`).Str(), page.MustEval(`()=>JSON.stringify({content:(document.querySelector('.vd-window-content')||document.body).innerHTML.slice(0,600),codeStudio:typeof window.CodeStudio,terminal:typeof window.Terminal,ready:window.AuraDesktopModules.appAssetsReady('code-studio'),scripts:[...document.querySelectorAll('script[data-aurago-lazy-src]')].map(s=>s.dataset.auragoLazySrc+':'+(s.dataset.auragoLoaded||'0'))})`).Str())
	}
	snapshot("cs-01-open")
	t.Log("state:", page.MustEval(`()=>({errors:fixtureErrors,files:csState().files.length,editorType:csState().editorType,container:csState().containerStatus})`).JSON("", ""))
	check("no browser errors after open", `()=>fixtureErrors.length===0`)

	// Open a file from the tree and make sure CodeMirror mounts.
	page.MustEval(`async()=>{[...csRoot().querySelectorAll('.cs-tree-item')].find(el=>el.dataset.filePath==='/workspace/main.go').click();await csWait(()=>csRoot().querySelector('.cm-editor'),10000);}`)
	snapshot("cs-02-editor")
	check("CodeMirror editor mounted", `()=>!!csRoot().querySelector('.cm-editor') && csState().openTabs.length===1`)
	check("active file is highlighted in the tree", `()=>{const row=[...csRoot().querySelectorAll('.cs-tree-item')].find(el=>el.dataset.filePath==='/workspace/main.go');return !!row && row.classList.contains('active');}`)

	// File actions must be discoverable on hover.
	page.MustElement(`.cs-tree-item[data-file-path="/workspace/README.md"]`).MustHover()
	check("tree file actions visible on hover", `()=>{const row=csRoot().querySelector('.cs-tree-item[data-file-path="/workspace/README.md"]');return parseFloat(getComputedStyle(row.querySelector('.cs-file-actions')).opacity)>0.9;}`)
	check("tree rows are keyboard focusable", `()=>csRoot().querySelector('.cs-tree-item').tabIndex===0`)

	// Typing "?" inside the editor must not open the shortcut overlay.
	page.MustEval(`()=>{csRoot().querySelector('.cm-content').focus();csKey(csRoot().querySelector('.cm-content'),{key:'?'});}`)
	check("question mark in editor does not open shortcut overlay", `()=>!document.querySelector('.cs-shortcut-overlay')`)
	page.MustEval(`()=>{document.querySelectorAll('.cs-shortcut-overlay').forEach(el=>el.remove());}`)

	// Zoom shortcuts advertised in the menus must work.
	page.MustEval(`()=>{csRoot().querySelector('.cm-content').focus();csKey(csRoot().querySelector('.cm-content'),{key:'=',ctrlKey:true});}`)
	check("Ctrl+= zooms the editor", `()=>csState().editorFontSize===13`)
	page.MustEval(`()=>{csKey(csRoot().querySelector('.cm-content'),{key:'0',ctrlKey:true});}`)
	check("Ctrl+0 resets zoom", `()=>csState().editorFontSize===12`)

	// Zen mode must be exitable through its own button.
	page.MustEval(`()=>{csKey(csRoot().querySelector('.cm-content'),{key:'k',ctrlKey:true});}`)
	check("Ctrl+K enters zen mode", `()=>csRoot().dataset.zen==='true'`)
	snapshot("cs-03-zen")
	page.MustEval(`()=>{csRoot().querySelector('[data-zen-exit]').click();}`)
	check("zen exit button leaves zen mode", `()=>csRoot().dataset.zen==='false' && csState().zenMode===false`)

	// Git panel must sit beside the editor, not below the body grid.
	page.MustEval(`async()=>{csRoot().querySelector('[data-activity="git"]').click();await csWait(()=>csRoot().querySelector('.cs-git-change'),10000);}`)
	snapshot("cs-04-git")
	check("git panel is laid out inside the body row", `()=>{const body=csRoot().querySelector('.code-studio-body').getBoundingClientRect(),git=csRoot().querySelector('[data-git-panel]').getBoundingClientRect(),main=csRoot().querySelector('.code-studio-main').getBoundingClientRect();return Math.abs(git.top-body.top)<2 && git.height>body.height-2 && main.height>body.height-2 && git.left>=main.right-1;}`)
	check("git badge shows change count", `()=>csRoot().querySelector('[data-activity="git"] .cs-activity-badge')?.textContent==='2'`)
	page.MustEval(`async()=>{csRoot().querySelector('.cs-git-change').click();await csWait(()=>csRoot().querySelector('.cs-diff-view'),10000);}`)
	snapshot("cs-05-diff")
	check("diff view can be closed back to the editor", `()=>{const close=csRoot().querySelector('[data-diff-close]');if(!close)return false;close.click();return !!csRoot().querySelector('.cm-editor');}`)
	page.MustEval(`()=>{csRoot().querySelector('[data-activity="git"]').click();}`)

	// Agent chat renders code blocks without mangling them.
	page.MustEval(`async()=>{csRoot().querySelector('[data-activity="agent"]').click();const form=csRoot().querySelector('[data-agent-form]');form.elements.message.value='Hilf mir';form.requestSubmit();await csWait(()=>csRoot().querySelector('.cs-agent-message.agent pre code'),10000);}`)
	snapshot("cs-06-agent")
	check("agent code block keeps its lines intact", `()=>{const code=csRoot().querySelector('.cs-agent-message.agent pre code');return !!code && code.textContent.includes('x := a * b * c') && code.textContent.includes('// # not a heading') && !code.querySelector('p,h1,em,strong');}`)
	check("agent markdown lists render as lists", `()=>csRoot().querySelectorAll('.cs-agent-message.agent ul li').length===2 && csRoot().querySelectorAll('.cs-agent-message.agent ol li').length===2`)
	check("agent panel and git panel share the grid", `()=>{const body=csRoot().querySelector('.code-studio-body').getBoundingClientRect(),chat=csRoot().querySelector('[data-agent-panel]').getBoundingClientRect();return Math.abs(chat.top-body.top)<2 && chat.height>body.height-2;}`)
	page.MustEval(`()=>{csRoot().querySelector('[data-activity="agent"]').click();}`)

	// Search in files shows its own empty state and results.
	page.MustEval(`async()=>{csRoot().querySelector('[data-activity="search"]').click();const form=csRoot().querySelector('[data-search-form]');form.elements.q.value='return';form.requestSubmit();await csWait(()=>csRoot().querySelectorAll('.cs-search-result').length>0,10000);}`)
	snapshot("cs-07-search")
	check("search results found", `()=>csRoot().querySelectorAll('.cs-search-result').length>=3`)
	page.MustEval(`()=>{csRoot().querySelector('[data-activity="search"]').click();}`)

	// Modified tabs must not close silently.
	page.MustEval(`async()=>{const view=csState().openTabs[0].view;view.dispatch({changes:{from:0,insert:'// edit\n'}});await csWait(()=>csState().openTabs[0].modified,2000);csRoot().querySelector('.cs-tab-close').click();}`)
	check("closing a modified tab asks for confirmation", `()=>!!document.querySelector('.cs-modal-backdrop') && csState().openTabs.length===1`)
	snapshot("cs-08-close-confirm")
	page.MustEval(`()=>{const cancel=document.querySelector('.cs-modal-backdrop [data-cancel]');if(cancel)cancel.click();}`)
	check("cancel keeps the tab open", `()=>csState().openTabs.length===1 && !document.querySelector('.cs-modal-backdrop')`)

	// Split editor keeps both panes editable and stays in sync.
	page.MustEval(`async()=>{[...csRoot().querySelectorAll('.cs-tree-item')].find(el=>el.dataset.filePath==='/workspace/README.md').click();await csWait(()=>csState().openTabs.length===2,10000);}`)
	page.MustEval(`()=>csMenuAction('view','split-right')`)
	snapshot("cs-09-split")
	check("split right shows two live editors", `()=>csRoot().querySelectorAll('[data-editor] .cm-editor').length===2`)
	check("split panes stay in sync", `()=>{const tab=csState().openTabs[1];const views=tab.views||[];if(views.length!==2)return false;views[0].dispatch({changes:{from:0,insert:'SYNC '}});return views[1].state.doc.toString().startsWith('SYNC ');}`)
	page.MustEval(`()=>{[...csRoot().querySelectorAll('.cs-tab')][0].click();}`)
	check("switching tabs leaves split intact", `()=>csRoot().querySelectorAll('[data-editor] .cm-editor').length===2`)
	page.MustEval(`()=>csMenuAction('view','split-right')`)
	check("split can be closed again", `()=>csRoot().querySelectorAll('[data-editor] .cm-editor').length===1`)

	// Terminal is connected and survives toggling.
	check("terminal socket connected", `()=>!!csState().ws && csState().ws.readyState===1 && csRoot().querySelector('.xterm')!==null`)
	page.MustEval(`()=>{csRoot().querySelector('[data-activity="terminal"]').click();csRoot().querySelector('[data-activity="terminal"]').click();}`)
	check("terminal survives toggling", `()=>csRoot().dataset.terminal==='visible' && csRoot().querySelector('.xterm')!==null`)

	// Run sends the file to exec and prints into the terminal.
	page.MustEval(`async()=>{csRoot().querySelector('[data-action="run"]').click();await new Promise(r=>setTimeout(r,400));}`)
	backend.mu.Lock()
	execs := append([]string{}, backend.execs...)
	backend.mu.Unlock()
	if len(execs) != 1 || !strings.Contains(execs[0], "go run") {
		t.Errorf("run did not execute the active Go file: %v", execs)
	}

	// Unsaved changes guard the window close (Run saved the file, so dirty it again first).
	page.MustEval(`async()=>{const tab=csState().openTabs[csState().activeTabIndex];tab.view.dispatch({changes:{from:0,insert:'// dirty\n'}});await csWait(()=>tab.modified,2000);}`)
	check("status bar counts unsaved files", `()=>!!csRoot().querySelector('.cs-status-unsaved')`)
	page.MustEval(`()=>{codeStudioTest.closeWindow(codeStudioTest.state.windows.keys().next().value);}`)
	page.MustEval(`async()=>{await new Promise(r=>setTimeout(r,200));}`)
	check("window close with unsaved changes asks first", `()=>codeStudioTest.state.windows.size===1 && !!document.querySelector('.cs-modal-backdrop')`)
	snapshot("cs-10-close-window")
	page.MustEval(`()=>{const cancel=document.querySelector('.cs-modal-backdrop [data-cancel]');if(cancel)cancel.click();}`)

	t.Log("final:", page.MustEval(`()=>({errors:fixtureErrors})`).JSON("", ""))
	check("no browser errors at the end", `()=>fixtureErrors.length===0`)
}
