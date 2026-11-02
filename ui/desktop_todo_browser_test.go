package ui

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"aurago/internal/desktop"
	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/input"
)

// The todo window is polished, so the browser has to prove the promises that polish makes: the sidebar counts the
// tasks of every filter, the search narrows the list, the detail pane keeps the reading position across a reload
// and every request reports itself in the pane.
func TestDesktopTodoBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)

	html := regexp.MustCompile(`(?s)<script\b[^>]*>.*?</script>`).ReplaceAllString(readDesktopAssetText(t, "desktop.html"), "")
	html = regexp.MustCompile(`\{\{[^}]*\}\}`).ReplaceAllString(html, "")
	html = strings.Replace(html, "</body>", `<script src="/js/shared/lazy-assets.js"></script><script src="/js/desktop/core/module-loader.js"></script><script src="/todo-shell.js"></script><script src="/testdata/aurora-fixture.js"></script></body>`, 1)
	shell := readDesktopAssetText(t, "js/desktop/bundles/main.bundle.js")
	cut := strings.LastIndex(shell, "    ensureDesktopRadialMenuAnchor();")
	if cut < 0 {
		t.Fatal("desktop startup seam missing")
	}
	shell = shell[:cut] + `window.aurora={state,loadIconManifest,applyDesktopSettings,renderIcons,renderStartApps,renderStartButtonIcon,wireShellChromeControls,bindViewportMetrics,handleDesktopKeydown,closeContextMenu,openApp,closeWindow};})();`

	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(Content)))
	mux.HandleFunc("/fixture", func(w http.ResponseWriter, _ *http.Request) { fmt.Fprint(w, html) })
	mux.HandleFunc("/todo-shell.js", func(w http.ResponseWriter, _ *http.Request) {
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
	page.MustSetViewport(1440, 900, 1, false)

	// The planner answers with four tasks: two open, one in progress, one done and one overdue.
	page.MustEval(`async()=>{
        await fixtureReady;
        window.todoCalls=[];
        window.fixtureTodos=[
            {id:'t1',title:'Release plan',description:'Ship the desktop polish.',status:'open',priority:'high',due_date:'2030-12-24T00:00:00Z',remind_daily:true,progress_percent:0,items:[{id:'i1',title:'Write the release notes',is_done:true},{id:'i2',title:'Tag the build',is_done:false}]},
            {id:'t2',title:'Review the backlog',description:'',status:'in_progress',priority:'medium',due_date:'',remind_daily:false,progress_percent:50,items:[{id:'i3',title:'Sort the tickets',is_done:false}]},
            {id:'t3',title:'Overdue report',description:'',status:'open',priority:'low',due_date:'2020-01-05T00:00:00Z',remind_daily:false,progress_percent:0,items:[]},
            {id:'t4',title:'Done task',description:'',status:'done',priority:'low',due_date:'',remind_daily:false,progress_percent:100,items:[{id:'i4',title:'Archive the report',is_done:true}]}
        ];
        const inner=window.fetch;
        window.fetch=async(url,options={})=>{
            const path=String(url);
            if(!path.startsWith('/api/desktop/integrations/todos'))return inner(url,options);
            const method=String(options.method||'GET').toUpperCase();
            window.todoCalls.push(method+' '+path);
            const reply=(data,status=200)=>new Response(JSON.stringify(data),{status,headers:{'Content-Type':'application/json'}});
            const body=options.body?JSON.parse(options.body):{};
            if(method==='GET')return reply(window.fixtureTodos);
            const idMatch=/\/todos\/([^/?]+)/.exec(path);
            const id=idMatch?idMatch[1]:'';
            const todo=window.fixtureTodos.find(entry=>entry.id===id);
            if(method==='POST'&&path.endsWith('/todos')){
                const created={id:'new-task',title:body.title||'',description:'',status:body.status||'open',priority:body.priority||'medium',due_date:'',remind_daily:false,progress_percent:0,items:[]};
                window.fixtureTodos.unshift(created);
                return reply(created,201);
            }
            if(method==='POST'&&path.endsWith('/complete')){
                if(todo){todo.status='done';todo.progress_percent=100;(todo.items||[]).forEach(item=>item.is_done=true);}
                return reply(todo||{});
            }
            if(method==='POST'&&path.endsWith('/items')){
                const item={id:'new-item',title:body.title||'',is_done:false};
                if(todo)(todo.items=todo.items||[]).push(item);
                return reply(item,201);
            }
            if(method==='PUT'&&path.includes('/items/')){
                const itemMatch=/\/items\/([^/?]+)/.exec(path);
                const item=(todo&&todo.items||[]).find(entry=>entry.id===itemMatch[1]);
                if(item)Object.assign(item,body);
                return reply(item||{});
            }
            if(method==='PUT'){
                if(todo)Object.assign(todo,body);
                return reply(todo||{});
            }
            if(method==='DELETE'&&path.includes('/items/')){
                const itemMatch=/\/items\/([^/?]+)/.exec(path);
                if(todo)todo.items=(todo.items||[]).filter(entry=>entry.id!==itemMatch[1]);
                return reply({});
            }
            if(method==='DELETE'){
                window.fixtureTodos=window.fixtureTodos.filter(entry=>entry.id!==id);
                return reply({});
            }
            return reply({},404);
        };
        await fixtureOpen('todo');
    }`)

	page.MustElement(".vd-todo-card")

	// The sidebar counts the tasks of every filter.
	raw := page.MustEval(`()=>[...document.querySelectorAll('.vd-todo-filter-count')].map(node=>node.textContent)`).Arr()
	counts := make([]string, 0, len(raw))
	for _, item := range raw {
		counts = append(counts, item.Str())
	}
	if got := strings.Join(counts, ","); got != "4,2,1,1" {
		t.Fatalf("filter counts = %s, want 4,2,1,1", got)
	}
	if got := page.MustEval(`()=>document.querySelectorAll('.vd-todo-card').length`).Int(); got != 4 {
		t.Fatalf("rendered %d task cards, want 4", got)
	}
	if got := page.MustEval(`()=>document.querySelectorAll('.vd-todo-card .vd-todo-chip').length`).Int(); got < 4 {
		t.Fatalf("rendered %d state chips, want at least 4", got)
	}
	// The overdue task carries its own chip.
	overdue := page.MustEval(`()=>[...document.querySelectorAll('.vd-todo-card')].some(card=>card.classList.contains('overdue')&&[...card.querySelectorAll('.vd-todo-chip')].some(chip=>chip.dataset.tone==='bad'))`).Bool()
	if !overdue {
		t.Fatal("the overdue task lost its overdue chip")
	}

	// The search narrows the list, and Escape clears it again.
	page.MustElement(".vd-todo-search-input").MustInput("backlog")
	page.MustEval(`()=>new Promise(resolve=>setTimeout(resolve,400))`)
	if got := page.MustEval(`()=>document.querySelectorAll('.vd-todo-card').length`).Int(); got != 1 {
		t.Fatalf("search for \"backlog\" left %d cards, want 1", got)
	}
	if err := page.Keyboard.Press(input.Escape); err != nil {
		t.Fatal(err)
	}
	page.MustEval(`()=>new Promise(resolve=>setTimeout(resolve,400))`)
	if got := page.MustEval(`()=>document.querySelector('.vd-todo-search-input').value`).Str(); got != "" {
		t.Fatalf("Escape left the search at %q, want it empty", got)
	}
	if got := page.MustEval(`()=>document.querySelectorAll('.vd-todo-card').length`).Int(); got != 4 {
		t.Fatalf("clearing the search left %d cards, want 4", got)
	}

	// A task that matches nothing explains itself and offers the way out.
	page.MustElement(".vd-todo-search-input").MustInput("nothing matches this")
	page.MustEval(`()=>new Promise(resolve=>setTimeout(resolve,400))`)
	if got := page.MustEval(`()=>document.querySelectorAll('.vd-todo-list .vd-todo-empty').length`).Int(); got != 1 {
		t.Fatalf("an empty search result rendered %d empty states, want 1", got)
	}
	saveTodoShot(t, page, "todo-polish-empty.png")
	page.MustElement("[data-todo-clear-search]").MustClick()
	page.MustEval(`()=>new Promise(resolve=>setTimeout(resolve,400))`)

	// Opening a task shows its details.
	page.MustElement(".vd-todo-card-title").MustClick()
	page.MustElement(".vd-todo-detail-title")
	page.MustElement(".vd-todo-form [name=\"title\"]")

	// A changed field marks the form as unsaved, and Escape returns it to the stored task.
	titleField := page.MustElement(".vd-todo-form [name=\"title\"]")
	titleField.MustSelectAllText()
	titleField.MustInput("Release plan v2")
	page.MustEval(`()=>new Promise(resolve=>setTimeout(resolve,50))`)
	if hidden := page.MustEval(`()=>document.querySelector('.vd-todo-dirty').hidden`).Bool(); hidden {
		t.Fatal("the changed title did not mark the form as unsaved")
	}
	if value := titleField.MustEval(`function(){return this.value;}`).Str(); value != "Release plan v2" {
		t.Fatalf("the title field holds %q, want the typed text", value)
	}
	if err := page.Keyboard.Press(input.Escape); err != nil {
		t.Fatal(err)
	}
	page.MustEval(`()=>new Promise(resolve=>setTimeout(resolve,50))`)
	if value := page.MustElement(".vd-todo-form [name=\"title\"]").MustEval(`function(){return this.value;}`).Str(); value != "Release plan" {
		t.Fatalf("Escape left the title at %q, want the stored task", value)
	}

	// Saving reports the request in the pane.
	descriptionField := page.MustElement(".vd-todo-form [name=\"description\"]")
	descriptionField.MustSelectAllText()
	descriptionField.MustInput("Ship the desktop polish and the notes.")
	page.MustElement(".vd-todo-form").MustEval(`function(){this.requestSubmit();}`)
	page.MustEval(`()=>new Promise(resolve=>setTimeout(resolve,300))`)
	saved := page.MustEval(`()=>window.I18N['desktop.todo_saved']`).Str()
	if notice := page.MustEval(`()=>document.querySelector('.vd-todo-notice-text').textContent`).Str(); notice != saved {
		t.Fatalf("the notice says %q, want the saved message %q", notice, saved)
	}
	if tone := page.MustEval(`()=>document.querySelector('.vd-todo-notice').dataset.tone`).Str(); tone != "ok" {
		t.Fatalf("the notice tone is %q, want ok", tone)
	}
	if value := page.MustElement(".vd-todo-form [name=\"description\"]").MustEval(`function(){return this.value;}`).Str(); value != "Ship the desktop polish and the notes." {
		t.Fatalf("the saved description is %q, want the typed text", value)
	}

	// A half-written checklist point keeps its text, its focus and its caret across the reload that a tick causes.
	page.MustEval(`()=>{
        const input=document.querySelector('.vd-todo-item-title');
        input.focus();
        input.value='Write the release notes and the changelog';
        input.setSelectionRange(9,9);
        input.dispatchEvent(new Event('input',{bubbles:true}));
    }`)
	// The tick is fired without a pointer, so the focus stays in the title field exactly as a repaint would find it.
	page.MustEval(`()=>document.querySelector('.vd-todo-item-check').click()`)
	page.MustEval(`()=>new Promise(resolve=>setTimeout(resolve,500))`)
	focused := page.MustEval(`()=>({tag:document.activeElement&&document.activeElement.className,value:document.activeElement&&document.activeElement.value,caret:document.activeElement&&document.activeElement.selectionStart})`)
	if name := focused.Get("tag").Str(); !strings.Contains(name, "vd-todo-item-title") {
		t.Fatalf("the focused element after the reload is %q, want the checklist title", name)
	}
	if value := focused.Get("value").Str(); value != "Write the release notes and the changelog" {
		t.Fatalf("the checklist title lost its text across the reload: %q", value)
	}
	if caret := focused.Get("caret").Int(); caret != 9 {
		t.Fatalf("the caret sits at %d, want 9", caret)
	}

	// A tick with the pointer takes the focus with it: the checkbox keeps it after the rebuild.
	page.MustElement(".vd-todo-item-check").MustClick()
	page.MustEval(`()=>new Promise(resolve=>setTimeout(resolve,500))`)
	if name := page.MustEval(`()=>document.activeElement&&document.activeElement.className`).Str(); !strings.Contains(name, "vd-todo-item-check") {
		t.Fatalf("the focused element after the click is %q, want the checklist box", name)
	}

	// The window shows the task list, its chips and the open detail in one frame.
	page.MustEval(`()=>{document.querySelector('.vd-todo-detail').scrollTop=0;document.querySelector('.vd-todo-list').scrollTop=0;}`)
	saveTodoShot(t, page, "todo-polish.png")

	// The window keeps a record of the failures instead of only a desktop toast.
	page.MustEval(`()=>{window.fetch=async()=>new Response('{"error":"boom"}',{status:500,headers:{'Content-Type':'application/json'}});}`)
	page.MustElement(".vd-todo-form").MustEval(`function(){this.requestSubmit();}`)
	page.MustEval(`()=>new Promise(resolve=>setTimeout(resolve,300))`)
	failed := page.MustEval(`()=>window.I18N['desktop.todo_failed']`).Str()
	if notice := page.MustEval(`()=>document.querySelector('.vd-todo-notice-text').textContent`).Str(); notice != failed {
		t.Fatalf("the failure notice says %q, want %q", notice, failed)
	}
	if tone := page.MustEval(`()=>document.querySelector('.vd-todo-notice').dataset.tone`).Str(); tone != "bad" {
		t.Fatalf("the failure notice tone is %q, want bad", tone)
	}
	saveTodoShot(t, page, "todo-polish-failure.png")
}

// saveTodoShot writes a screenshot of the desktop with the todo window into the reports folder.
func saveTodoShot(t *testing.T, page *rod.Page, name string) {
	t.Helper()
	dir := filepath.Join("..", "reports", "todo")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	shot, err := page.Screenshot(false, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), shot, 0644); err != nil {
		t.Fatal(err)
	}
}
