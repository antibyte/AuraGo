package ui

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strconv"
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
	files := map[string]string{"Documents/Notes/aurora.md": content, "Documents/Notes/ideen.md": "# Ideen\n\nEine neue Perspektive. Mehr dazu in [[aurora]].", "Documents/Notes/verweise.md": "# Verweise\n\nSiehe [[ideen]] und [[Ideen#Plan|noch einmal]].\n", "Documents/Notes/einkauf.md": "# Für die Werkstatt\n\n- [ ] Kaffee\n- [ ] Pflanzen", "Documents/Notes/Projekte/plan.md": "# Projektplan\n\nMeilensteine für das Aurora-Vorhaben #aurora.\n"}
	extraFolders := map[string]bool{}
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.Dir(".")))
	mux.HandleFunc("/api/desktop/upload", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Header.Get("If-None-Match") != "*" {
			w.WriteHeader(http.StatusPreconditionRequired)
			fmt.Fprint(w, `{"error":"file_precondition_required"}`)
			return
		}
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		defer r.MultipartForm.RemoveAll()
		if r.FormValue("path") != "Documents/Notes/.attachments" || r.FormValue("unique") != "1" {
			http.Error(w, "Invalid attachment destination", http.StatusBadRequest)
			return
		}
		file, header, err := r.FormFile("file")
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		defer file.Close()
		data, err := io.ReadAll(file)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		path := r.FormValue("path") + "/" + header.Filename
		mu.Lock()
		files[path] = string(data)
		mu.Unlock()
		json.NewEncoder(w).Encode(map[string]string{"path": path})
	})
	mux.HandleFunc("/api/desktop/download", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		data, exists := files[r.URL.Query().Get("path")]
		if !exists {
			http.NotFound(w, r)
			return
		}
		w.Write([]byte(data))
	})
	mux.HandleFunc("/api/desktop/notes", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		path := r.URL.Query().Get("path")
		if r.Method == "GET" && r.URL.Query().Get("index") == "1" {
			result := desktop.NotesIndexResult{Notes: []desktop.NoteIndexEntry{}, Folders: []string{}, Attachments: []string{}}
			seen := map[string]bool{}
			for notePath, raw := range files {
				if strings.HasPrefix(notePath, "Trash/") {
					continue
				}
				if !strings.HasSuffix(notePath, ".md") {
					if strings.HasPrefix(notePath, "Documents/Notes/") && !strings.HasSuffix(notePath, ".json") {
						result.Attachments = append(result.Attachments, notePath)
					}
					continue
				}
				refs := desktop.ExtractNoteRefs(raw)
				title := strings.TrimSuffix(notePath[strings.LastIndex(notePath, "/")+1:], ".md")
				if refs.TitleSource == "frontmatter" || refs.TitleSource == "heading" {
					title = refs.Title
				}
				entry := desktop.NoteIndexEntry{Path: notePath, Title: title, Aliases: refs.Aliases, Tags: refs.Tags, Links: []desktop.NoteIndexLink{}, Modified: time.Date(2026, 9, 10, 10, 0, 0, 0, time.UTC), Size: int64(len(raw))}
				if entry.Aliases == nil {
					entry.Aliases = []string{}
				}
				if entry.Tags == nil {
					entry.Tags = []string{}
				}
				for _, link := range refs.Links {
					entry.Links = append(entry.Links, desktop.NoteIndexLink{Target: link.Value, Embed: link.Embed, Kind: link.Kind})
				}
				result.Notes = append(result.Notes, entry)
				if dir := notePath[:strings.LastIndex(notePath, "/")]; dir != "Documents/Notes" && !seen[dir] {
					seen[dir] = true
					result.Folders = append(result.Folders, dir)
				}
			}
			for dir := range extraFolders {
				if !seen[dir] {
					seen[dir] = true
					result.Folders = append(result.Folders, dir)
				}
			}
			json.NewEncoder(w).Encode(result)
			return
		}
		if r.Method == "PATCH" {
			var body struct {
				Path        string `json:"path"`
				NewPath     string `json:"new_path"`
				Operation   string `json:"operation"`
				UpdateLinks bool   `json:"update_links"`
			}
			json.NewDecoder(r.Body).Decode(&body)
			raw, exists := files[body.Path]
			if body.Operation != "move" || !exists || r.Header.Get("If-Match") != desktop.NoteVersion([]byte(raw)) {
				w.WriteHeader(412)
				fmt.Fprint(w, `{"error":"Conflict"}`)
				return
			}
			delete(files, body.Path)
			files[body.NewPath] = raw
			updated := 0
			if body.UpdateLinks {
				oldName := strings.TrimSuffix(body.Path[strings.LastIndex(body.Path, "/")+1:], ".md")
				newName := strings.TrimSuffix(body.NewPath[strings.LastIndex(body.NewPath, "/")+1:], ".md")
				for other, content := range files {
					if !strings.HasSuffix(other, ".md") || other == body.NewPath {
						continue
					}
					files[other] = desktop.RewriteNoteLinks(content, func(link desktop.NoteRefLink) (string, bool) {
						if link.Kind != "wiki" || !strings.EqualFold(desktop.ParseWikiValue(link.Value).Target, oldName) {
							return "", false
						}
						updated++
						return desktop.ReplaceWikiTarget(link.Value, newName), true
					})
				}
			}
			json.NewEncoder(w).Encode(struct {
				desktop.Note
				UpdatedLinks int `json:"updated_links"`
			}{desktop.Note{Path: body.NewPath, Title: strings.TrimPrefix(strings.Split(raw, "\n")[0], "# "), Content: raw, Version: desktop.NoteVersion([]byte(raw)), Modified: time.Date(2026, 9, 10, 10, 0, 0, 0, time.UTC)}, updated})
			return
		}
		if r.Method == "PUT" || r.Method == "POST" {
			var body struct{ Path, Content, Title, Folder, Operation string }
			json.NewDecoder(r.Body).Decode(&body)
			if r.Method == "POST" && body.Operation == "mkdir" {
				extraFolders[body.Folder] = true
				json.NewEncoder(w).Encode(map[string]string{"path": body.Folder})
				return
			}
			if body.Path != "" {
				path = body.Path
			}
			if r.Method == "POST" {
				folder := body.Folder
				if folder == "" {
					folder = "Documents/Notes"
				}
				path = fmt.Sprintf("%s/new-%d.md", folder, len(files))
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
		query := r.URL.Query()
		words := strings.Fields(strings.ToLower(query.Get("q")))
		wantTag, wantFolder, wantTrash := strings.ToLower(query.Get("tag")), query.Get("folder"), query.Get("trash") == "true"
		paths := []string{}
		for path := range files {
			if strings.HasSuffix(path, ".md") {
				paths = append(paths, path)
			}
		}
		sort.Strings(paths)
		notes := []desktop.Note{}
		for _, path := range paths {
			raw := files[path]
			if strings.HasPrefix(path, "Trash/") != wantTrash {
				continue
			}
			title := strings.TrimPrefix(strings.Split(raw, "\n")[0], "# ")
			if strings.HasSuffix(path, "aurora.md") {
				title = "Kleine Ideen, große Möglichkeiten"
			}
			if wantFolder != "" && path[:strings.LastIndex(path, "/")] != wantFolder {
				continue
			}
			tags := desktop.ExtractNoteRefs(raw).Tags
			if wantTag != "" {
				found := false
				for _, tag := range tags {
					if strings.ToLower(tag) == wantTag {
						found = true
					}
				}
				if !found {
					continue
				}
			}
			haystack := strings.ToLower(title + "\n" + raw)
			matched := true
			for _, word := range words {
				if !strings.Contains(haystack, word) {
					matched = false
				}
			}
			if !matched {
				continue
			}
			if tags == nil {
				tags = []string{}
			}
			notes = append(notes, desktop.Note{Path: path, Title: title, Snippet: "Gedanken, Projekte und neue Perspektiven.", Tags: tags, Version: desktop.NoteVersion([]byte(raw)), Modified: time.Date(2026, 9, 10, 10, 0, 0, 0, time.UTC)})
		}
		total := len(notes)
		if offset, err := strconv.Atoi(query.Get("offset")); err == nil && offset > 0 {
			if offset > len(notes) {
				offset = len(notes)
			}
			notes = notes[offset:]
		}
		if limit, err := strconv.Atoi(query.Get("limit")); err == nil && limit > 0 && limit < len(notes) {
			notes = notes[:limit]
		}
		json.NewEncoder(w).Encode(desktop.NotesResult{Notes: notes, Total: total, Folders: []string{}})
	})
	// Strings of this round that are not in the locale files yet: AURAGO_NOTES_EXTRA_LABELS points to a JSON object of desktop keys.
	mux.HandleFunc("/notes-extra-labels", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if data, err := os.ReadFile(os.Getenv("AURAGO_NOTES_EXTRA_LABELS")); err == nil {
			w.Write(data)
			return
		}
		fmt.Fprint(w, "{}")
	})
	mux.HandleFunc("/notes-fixture", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<!doctype html><html lang="de"><head><meta charset="utf-8"><link rel="stylesheet" href="/js/vendor/notes/engine.css"><link rel="stylesheet" href="/css/desktop-app-notes.css"><link rel="stylesheet" href="/css/desktop-app-notes-nav.css"><link rel="stylesheet" href="/css/desktop-icons.css">
<style>@font-face{font-family:Geist;src:url('/fonts/geist/Geist-Variable.woff2')}html,body{margin:0;height:100%;font-family:Geist,system-ui}body{--vd-text:#e4e8f0;--vd-theme-app-bg:#11161d;--vd-theme-panel-bg:#1b202a;--vd-theme-chrome-bg:#1d2430;--vd-theme-control-bg:#252d3a;--vd-theme-border:#ffffff1a;--vd-theme-muted:#a7b1c3;--vd-accent:#f3b676}body[data-fruity-mode=light]{--vd-text:#212b3c;--vd-theme-app-bg:#e2e7ee;--vd-theme-panel-bg:#e5e9ef;--vd-theme-chrome-bg:#d7dfe9;--vd-theme-control-bg:#f8fafc;--vd-theme-border:#65748c40;--vd-theme-muted:#536279;--vd-accent:#996419}#host{height:100%}</style></head><body class="desktop-body" data-theme="default"><div id="host"></div>
<script>window.errors=[];addEventListener('error',e=>errors.push(e.error?.stack||e.message));addEventListener('unhandledrejection',e=>errors.push(String(e.reason)));</script>
<script src="/js/vendor/marked.min.js"></script><script src="/js/vendor/purify.min.js"></script><script src="/js/desktop/apps/notes-frontmatter.js"></script><script src="/js/desktop/apps/notes-index.js"></script><script src="/js/desktop/apps/notes-templates.js"></script><script src="/js/desktop/apps/writer-session.js"></script><script src="/js/desktop/apps/notes-editor.js"></script><script src="/js/desktop/apps/notes-panels.js"></script><script src="/js/desktop/apps/notes-sidebar.js"></script><script src="/js/desktop/apps/notes-palette.js"></script><script src="/js/desktop/apps/notes-graph.js"></script><script src="/js/desktop/apps/notes.js"></script>
<script>window.ready=(async()=>{const labels={...await(await fetch('/notes-extra-labels')).json(),...await(await fetch('/lang/desktop/de.json')).json()};window.notesContext={path:'Documents/Notes/aurora.md',iconMarkup:name=>'<span class="notes-icon" aria-hidden="true" style="display:inline-grid;place-items:center;font-size:11px;font-weight:700">'+String(name||'?').charAt(0).toUpperCase()+'</span>',t:(key,params)=>String(labels[key]||key).replace(/\{\{(\w+)\}\}/g,(_,x)=>params?.[x]??''),promptDialog:async(title,value)=>value,confirmDialog:async()=>false,setWindowBeforeClose:(id,fn)=>window.closeGuard=fn,setWindowMenus:(id,menus)=>window.menus=menus,saveFileDialog:async()=>({path:'Documents/Notes/copy.md'}),openFileDialog:async()=>({path:'Documents/Notes/aurora.md'})};await NotesApp.render(document.getElementById('host'),'test',notesContext).ready;})();</script></body></html>`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	launch := launcher.New().Bin(bin).Headless(true).NoSandbox(true).Leakless(false)
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
	obsidian, err := os.ReadFile("testdata/notes-obsidian-functional.js")
	if err != nil {
		t.Fatal(err)
	}
	t.Log("Notes linking workflows:", page.MustEval(string(obsidian)).JSON("", ""))
	navigation, err := os.ReadFile("testdata/notes-navigation-functional.js")
	if err != nil {
		t.Fatal(err)
	}
	t.Log("Notes navigation workflows:", page.MustEval(string(navigation)).JSON("", ""))
	out := os.Getenv("AURAGO_BROWSER_ARTIFACT_DIR")
	if out == "" {
		out = filepath.Join("..", "reports", "notes")
	}
	if err := os.MkdirAll(out, 0755); err != nil {
		t.Fatal(err)
	}
	// Obsidian syntax demo: shown once in the main editor, then restored without saving.
	page.MustSetViewport(1366, 768, 1, false)
	page.MustEval(`async()=>{
 const app=NotesApp.instances.get('test');
 window.obsidianPrevious=app.editor.content();
 if(app.editor.sourceMode)await app.editor.mode(false);
 app.editor.setContent('# Obsidian Syntax\n\nVerweis auf [[aurora|Aurora Ideen]] und die fehlende Seite [[Fehlt noch#Plan]] mit #projekt/alpha im Text.\n\n#idee steht am Zeilenanfang und ==wichtig== ist markiert.\n\n> [!info] Hinweis\n> Siehe ![[skizze.png]] und [[aurora]].\n\n> [!warning] Achtung\n> Das ist eine **Warnung** mit Text.\n');
 app.editor.refreshDecorations();
 await new Promise(r=>setTimeout(r,300));
 }`)
	page.MustScreenshot(filepath.Join(out, "obsidian-syntax-1366x768.png"))
	page.MustEval(`()=>{const app=NotesApp.instances.get('test');app.editor.setContent(window.obsidianPrevious);}`)
	// Linking scene: right sidebar on Backlinks and a hover preview card, restored without saving.
	page.MustEval(`async()=>{
 const app=NotesApp.instances.get('test'),sleep=ms=>new Promise(r=>setTimeout(r,ms));
 if(app.editor.sourceMode)await app.editor.mode(false);
 window.linkingPrevious=app.editor.content();
 app.editor.setContent('# Ideen\n\nEine neue Perspektive. Mehr dazu in [[einkauf]] und [[aurora|Aurora Ideen]].\n\n> [!tip] Verknüpfen\n> Mit doppelten eckigen Klammern entstehen Verweise, zum Beispiel [[Neue Idee]].\n');
 app.editor.refreshDecorations();
 const open=!document.querySelector('[data-right]').hidden&&document.querySelector('[data-right] [role=tab][aria-selected=true]')?.dataset.rightTab==='backlinks';
 if(!open){if(!document.querySelector('[data-right]').hidden)await app.act('closePanel');await app.act('backlinks');}
 await sleep(700);
 const chip=[...document.querySelectorAll('.notes-wikilink')].find(el=>el.textContent.includes('einkauf'));
 chip.dispatchEvent(new MouseEvent('mouseenter'));
 await sleep(1100);
 }`)
	page.MustScreenshot(filepath.Join(out, "linking-1366x768.png"))
	page.MustEval(`()=>{const app=NotesApp.instances.get('test');document.querySelectorAll('.notes-wikilink').forEach(el=>el.dispatchEvent(new MouseEvent('mouseleave')));app.editor.setContent(window.linkingPrevious);}`)
	// Navigation scenes: Files tab with the current note highlighted, quick switcher with a query, command palette.
	page.MustSetViewport(1366, 768, 1, false)
	page.MustEval(`async()=>{
 const app=NotesApp.instances.get('test'),sleep=ms=>new Promise(r=>setTimeout(r,ms)),root=document.querySelector('.vd-notes-app');
 root.classList.remove('notes-hide-library');
 if(!document.querySelector('[data-right]').hidden)await app.act('closePanel');
 app.sidebar.setTab('files');
 await sleep(200);
 const target=document.querySelector('.notes-tree-row[data-path="Documents/Notes/Projekte/plan.md"]')||document.querySelector('.notes-tree-row[data-kind=note]');
 target.click();
 await sleep(900);
 }`)
	page.MustScreenshot(filepath.Join(out, "navigation-files-1366x768.png"))
	page.MustEval(`async()=>{
 const app=NotesApp.instances.get('test'),sleep=ms=>new Promise(r=>setTimeout(r,ms));
 app.editor.view.dom.dispatchEvent(new KeyboardEvent('keydown',{key:'o',ctrlKey:true,bubbles:true,cancelable:true}));
 await sleep(200);
 const input=document.querySelector('.notes-palette-input');input.value='ide';input.dispatchEvent(new Event('input',{bubbles:true}));
 await sleep(300);
 }`)
	page.MustScreenshot(filepath.Join(out, "navigation-switcher-1366x768.png"))
	page.MustEval(`async()=>{
 const app=NotesApp.instances.get('test'),sleep=ms=>new Promise(r=>setTimeout(r,ms));
 document.querySelector('.notes-palette-input').dispatchEvent(new KeyboardEvent('keydown',{key:'Escape',bubbles:true,cancelable:true}));
 await sleep(100);
 app.editor.view.dom.dispatchEvent(new KeyboardEvent('keydown',{key:'p',ctrlKey:true,bubbles:true,cancelable:true}));
 await sleep(200);
 const input=document.querySelector('.notes-palette-input');input.value='';input.dispatchEvent(new Event('input',{bubbles:true}));
 await sleep(300);
 }`)
	page.MustScreenshot(filepath.Join(out, "navigation-palette-1366x768.png"))
	page.MustEval(`async()=>{
 const app=NotesApp.instances.get('test'),sleep=ms=>new Promise(r=>setTimeout(r,ms));
 document.querySelector('.notes-palette-input').dispatchEvent(new KeyboardEvent('keydown',{key:'Escape',bubbles:true,cancelable:true}));
 await sleep(100);
 const aurora=document.querySelector('.notes-tree-row[data-path="Documents/Notes/aurora.md"]');
 if(aurora)aurora.click();
 await sleep(900);
 }`)
	// Find bar scene: Ctrl+F with a query, closed again before the theme matrix.
	page.MustEval(`async()=>{
 const app=NotesApp.instances.get('test'),sleep=ms=>new Promise(r=>setTimeout(r,ms));
 app.editor.view.dom.dispatchEvent(new KeyboardEvent('keydown',{key:'f',ctrlKey:true,bubbles:true,cancelable:true}));
 const input=document.querySelector('[data-find]');input.value='o';input.dispatchEvent(new Event('input',{bubbles:true}));
 await sleep(300);
 }`)
	page.MustScreenshot(filepath.Join(out, "navigation-find-1366x768.png"))
	page.MustSetViewport(430, 932, 1, false)
	page.MustScreenshot(filepath.Join(out, "navigation-find-430x932.png"))
	page.MustSetViewport(1366, 768, 1, false)
	page.MustEval(`()=>document.querySelector('[data-action=findClose]').click()`)
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
