package ui

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"aurago/internal/desktop"
	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
)

func TestDesktopNotesAppBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	bin, ok := browserExecutable()
	if !ok {
		t.Fatal("Chrome required")
	}
	var mu sync.Mutex
	content := "---\ntags: [design, aurora]\nowner: Preserve this field\n---\n# Kleine Ideen, große Möglichkeiten\n\nEin ruhiger Ort für Gedanken, Projekte und alles, was noch wachsen darf.\n\n## Der nächste Schritt\n\n- [x] Die Idee festhalten\n- [ ] Gemeinsam etwas Besonderes entwickeln\n- [ ] Die kleinen Details nicht vergessen\n\n## Werkstattnotizen\n\nGute Werkzeuge lassen uns **konzentriert arbeiten**. Der Inhalt steht im Mittelpunkt — die Oberfläche hält sich zurück.\n\n> Eine gute Idee beginnt oft mit einer kleinen Notiz.\n\n| Projekt | Status |\n| --- | --- |\n| Aurora Workstation | In Arbeit |\n| Leafy | Wächst weiter |\n\n## Später weiterdenken\n\nMit Notizen, die sich leicht wiederfinden lassen.\n"
	files := map[string]string{"Documents/Notes/aurora.md": content, "Documents/Notes/ideen.md": "# Ideen\n\nEine neue Perspektive.", "Documents/Notes/einkauf.md": "# Für die Werkstatt\n\n- [ ] Kaffee\n- [ ] Pflanzen"}
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.Dir(".")))
	mux.HandleFunc("/api/desktop/notes", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		path := r.URL.Query().Get("path")
		if r.Method == "PUT" || r.Method == "POST" {
			var body struct{ Path, Content, Title, Folder string }
			json.NewDecoder(r.Body).Decode(&body)
			if body.Path != "" {
				path = body.Path
			}
			if r.Method == "POST" {
				path = fmt.Sprintf("Documents/Notes/new-%d.md", len(files))
			}
			previous, exists := files[path]
			if exists && r.Header.Get("If-Match") != desktop.NoteVersion([]byte(previous)) || !exists && r.Method == "PUT" && r.Header.Get("If-None-Match") != "*" {
				w.WriteHeader(412)
				fmt.Fprint(w, `{"error":"Conflict"}`)
				return
			}
			files[path] = body.Content
		}
		if path != "" {
			raw, exists := files[path]
			if !exists {
				http.NotFound(w, r)
				return
			}
			title := strings.TrimPrefix(strings.Split(strings.TrimPrefix(raw, "---\ntags: [design, aurora]\nowner: Preserve this field\n---\n"), "\n")[0], "# ")
			json.NewEncoder(w).Encode(desktop.Note{Path: path, Title: title, Content: raw, Version: desktop.NoteVersion([]byte(raw)), Modified: time.Date(2026, 9, 10, 10, 0, 0, 0, time.UTC)})
			return
		}
		notes := []desktop.Note{}
		for path, raw := range files {
			if !strings.HasSuffix(path, ".md") {
				continue
			}
			title := strings.TrimPrefix(strings.Split(raw, "\n")[0], "# ")
			if strings.HasSuffix(path, "aurora.md") {
				title = "Kleine Ideen, große Möglichkeiten"
			}
			notes = append(notes, desktop.Note{Path: path, Title: title, Snippet: "Gedanken, Projekte und neue Perspektiven.", Tags: []string{"aurora"}, Version: desktop.NoteVersion([]byte(raw)), Modified: time.Date(2026, 9, 10, 10, 0, 0, 0, time.UTC)})
		}
		json.NewEncoder(w).Encode(desktop.NotesResult{Notes: notes, Total: len(notes), Folders: []string{}})
	})
	mux.HandleFunc("/notes-fixture", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<!doctype html><html lang="de"><head><meta charset="utf-8"><link rel="stylesheet" href="/js/vendor/notes/engine.css"><link rel="stylesheet" href="/css/desktop-app-notes.css"><link rel="stylesheet" href="/css/desktop-icons.css">
<style>@font-face{font-family:Geist;src:url('/fonts/geist/Geist-Variable.woff2')}html,body{margin:0;height:100%;font-family:Geist,system-ui}body{--vd-text:#e4e8f0;--vd-theme-app-bg:#11161d;--vd-theme-panel-bg:#1b202a;--vd-theme-chrome-bg:#1d2430;--vd-theme-control-bg:#252d3a;--vd-theme-border:#ffffff1a;--vd-theme-muted:#a7b1c3;--vd-accent:#f3b676}body[data-fruity-mode=light]{--vd-text:#212b3c;--vd-theme-app-bg:#e2e7ee;--vd-theme-panel-bg:#e5e9ef;--vd-theme-chrome-bg:#d7dfe9;--vd-theme-control-bg:#f8fafc;--vd-theme-border:#65748c40;--vd-theme-muted:#536279;--vd-accent:#996419}#host{height:100%}</style></head><body class="desktop-body" data-theme="default"><div id="host"></div>
<script>window.errors=[];addEventListener('error',e=>errors.push(e.error?.stack||e.message));addEventListener('unhandledrejection',e=>errors.push(String(e.reason)));</script>
<script src="/js/vendor/marked.min.js"></script><script src="/js/vendor/purify.min.js"></script><script src="/js/desktop/apps/writer-session.js"></script><script src="/js/desktop/apps/notes-frontmatter.js"></script><script src="/js/desktop/apps/notes-editor.js"></script><script src="/js/desktop/apps/notes.js"></script>
<script>window.ready=(async()=>{const labels=await(await fetch('/lang/desktop/de.json')).json();window.notesContext={path:'Documents/Notes/aurora.md',t:(key,params)=>String(labels[key]||key).replace(/\{\{(\w+)\}\}/g,(_,x)=>params?.[x]??''),promptDialog:async(title,value)=>value,confirmDialog:async()=>false,setWindowBeforeClose:(id,fn)=>window.closeGuard=fn,setWindowMenus:(id,menus)=>window.menus=menus,saveFileDialog:async()=>({path:'Documents/Notes/copy.md'}),openFileDialog:async()=>({path:'Documents/Notes/aurora.md'})};await NotesApp.render(document.getElementById('host'),'test',notesContext).ready;})();</script></body></html>`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	launch := launcher.New().Bin(bin).Headless(true).NoSandbox(true)
	browser := rod.New().ControlURL(launch.MustLaunch()).MustConnect()
	defer launch.Cleanup()
	defer browser.Close()
	page := browser.MustPage(srv.URL + "/notes-fixture").Timeout(90 * time.Second)
	defer page.Close()
	page.MustSetViewport(1366, 768, 1, false)
	page.MustWaitLoad()
	page.MustEval(`async()=>await window.ready`)
	t.Log(page.MustEval(`()=>({editor:!!NotesApp.instances.get('test').editor,errors,notice:document.querySelector('[data-notice-text]').textContent,html:document.querySelector('[data-editor]').innerHTML.slice(0,350)})`).JSON("", ""))
	if !page.MustEval(`()=>!!NotesApp.instances.get('test').editor`).Bool() {
		t.Fatal("Editor failed to load")
	}
	result := page.MustEval(`async()=>{
 const app=NotesApp.instances.get('test'),original=app.editor.content();
 if(!original.includes('owner: Preserve this field'))throw Error('Frontmatter lost during load');
 const view=app.editor.view;view.dispatch(view.state.tr.insertText('Hallo ',2));await app.session.save();
 if(app.session.dirty)throw Error('Save did not acknowledge revision');
 const rich=app.editor.content();await app.act('source');
 if(!app.editor.sourceMode)throw Error('Source mode missing');
 await app.act('source');if(app.editor.content()!==rich)throw Error('Mode switch changed content');
 app.editor.setContent(NotesFrontmatter.updateTags(app.editor.content(),['changed']));await app.session.save();
 if(!app.editor.content().includes('owner: Preserve this field'))throw Error('Tags rewrote opaque frontmatter');
 await app.act('outline');
 return {saved:!app.session.dirty,source:app.editor.sourceMode,errors,outline:document.querySelectorAll('[data-jump]').length};
 }`)
	t.Logf("Notes: %s", result.JSON("", ""))
	if len(result.Get("errors").Arr()) > 0 {
		t.Fatal(result.Get("errors").JSON("", ""))
	}
	functional, err := os.ReadFile("testdata/notes-functional.js")
	if err != nil {
		t.Fatal(err)
	}
	t.Log("Notes workflows:", page.MustEval(string(functional)).JSON("", ""))
	out := filepath.Join("..", "reports", "notes")
	os.MkdirAll(out, 0755)
	for _, theme := range []string{"default", "fruity-dark", "fruity-light"} {
		for _, density := range []string{"normal", "compact"} {
			for _, size := range [][2]int{{1920, 1080}, {1366, 768}, {430, 932}} {
				page.MustSetViewport(size[0], size[1], 1, false)
				page.MustEval(`(theme,density)=>{document.body.dataset.theme=theme.startsWith('fruity')?'fruity':'default';document.body.dataset.fruityMode=theme.endsWith('light')?'light':'dark';document.body.dataset.density=density;document.querySelector('.vd-notes-app').classList.toggle('notes-hide-library',innerWidth<900)}`, theme, density)
				page.MustScreenshot(filepath.Join(out, fmt.Sprintf("%s-%s-%dx%d.png", theme, density, size[0], size[1])))
			}
		}
	}
}
