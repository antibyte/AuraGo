package server

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/kaggle"
	"aurago/ui"
)

// TestKaggleDesktopPolishBrowser covers the desktop polish round: the sidebar
// counts the running work, the overview tile opens the job list, a result row
// links to the public Kaggle page and the query field answers to the keyboard.
func TestKaggleDesktopPolishBrowser(t *testing.T) {
	browser := personalRadioBrowser(t)
	translations, err := ui.Content.ReadFile("lang/kaggle/de.json")
	if err != nil {
		t.Fatal(err)
	}
	policy := config.DefaultKaggleConfig()
	policy.Enabled = true
	policy.ReadOnly = false
	status := map[string]any{
		"enabled": true, "read_only": false, "configured": true,
		"storage_bytes": int64(0), "max_storage_bytes": int64(1) << 30,
		"policy":        policy,
		"access":        map[string]bool{"network": true, "write": true, "python": true, "docker": true},
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/kaggle/", func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/kaggle/status":
			kaggleJSON(w, 200, status)
		case "/api/kaggle/capabilities":
			kaggleJSON(w, 200, map[string]any{"operations": kaggle.Catalog(), "definitions": kaggle.Definitions()})
		case "/api/kaggle/projects":
			kaggleJSON(w, 200, []any{})
		case "/api/kaggle/jobs":
			kaggleJSON(w, 200, []kaggle.Job{
				{ID: "run", Domain: "datasets", Operation: "download_dataset", Status: "running"},
				{ID: "queue", Domain: "notebooks", Operation: "save_kernel", Status: "queued"},
				{ID: "done", Domain: "datasets", Operation: "download_dataset", Status: "completed"},
			})
		case "/api/kaggle/operations":
			var request kaggle.Request
			if !decodeKaggle(w, r, &request) {
				return
			}
			if request.Operation == "list_datasets" {
				kaggleJSON(w, 200, map[string]any{"data": map[string]any{"items": []map[string]string{{"title": "Iris flowers", "ref": "review/iris", "subtitle": "Synthetic fixture"}}}})
				return
			}
			kaggleJSON(w, 200, map[string]any{"data": map[string]any{"items": []any{}}})
		case "/api/kaggle/events":
			w.WriteHeader(204)
		default:
			kaggleJSON(w, 200, map[string]any{})
		}
	})
	mux.HandleFunc("/fixture", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<!doctype html><meta charset="utf-8"><link rel="stylesheet" href="/css/desktop-app-kaggle.css"><style>body{margin:0}.host{height:760px;width:940px}</style><body data-theme="standard"><div class="host" id="app"></div><script>window.I18N=%s;window.errors=[];addEventListener('error',e=>errors.push(e.message));addEventListener('unhandledrejection',e=>errors.push(String(e.reason)));</script><script src="/js/desktop/apps/kaggle.js"></script><script>KaggleApp.render(document.getElementById('app'),'polish',{t:k=>I18N[k]||k});</script>`, translations)
	})
	mux.Handle("/", http.FileServer(http.FS(ui.Content.FS)))
	origin := httptest.NewServer(mux)
	defer origin.Close()
	page := browser.MustPage(origin.URL + "/fixture").Timeout(45 * time.Second)
	defer page.Close()
	page.MustWait(`()=>document.querySelector('.kg-stats')!==null`)

	// The sidebar counts the running work for every view.
	if !page.MustEval(`()=>{const nav=document.querySelector('[data-domain="jobs"]');const badge=nav.querySelector('.kg-nav-count');return !!badge&&!badge.hidden&&badge.textContent==='2'&&nav.textContent.includes(I18N['kaggle.active_jobs_count'].replace('{0}','2'));}`).Bool() {
		t.Fatal("sidebar does not count the active jobs")
	}
	// The connection state leads to the Kaggle settings and keeps its name.
	if !page.MustEval(`()=>{const chip=document.querySelector('.kg-nav-status');return !!chip&&chip.tagName==='A'&&chip.getAttribute('href')==='/config#kaggle'&&chip.getAttribute('aria-label').includes(I18N['kaggle.settings'])&&chip.getAttribute('aria-label').includes(chip.textContent);}`).Bool() {
		t.Fatal("sidebar state does not link to the Kaggle settings")
	}
	// The overview tile opens the job list.
	if !page.MustEval(`()=>{const tile=[...document.querySelectorAll('.kg-stats button')].find(b=>b.textContent.includes(I18N['kaggle.active_jobs']));return !!tile&&tile.textContent.includes('2');}`).Bool() {
		t.Fatal("overview does not offer the active job count")
	}
	page.MustEval(`()=>[...document.querySelectorAll('.kg-stats button')].find(b=>b.textContent.includes(I18N['kaggle.active_jobs'])).click()`)
	page.MustWait(`()=>document.querySelector('[data-domain="jobs"]').getAttribute('aria-current')==='page'&&document.querySelectorAll('.kg-resource').length===3`)

	// A result row links to the public Kaggle page.
	page.MustEval(`()=>document.querySelector('[data-domain="datasets"]').click()`)
	page.MustWait(`()=>document.querySelector('.kg-search input')!==null`)
	page.MustEval(`()=>[...document.querySelectorAll('.kg-toolbar button')].find(b=>b.textContent===I18N['kaggle.execute']).click()`)
	page.MustWait(`()=>document.querySelectorAll('.kg-resource').length===1`)
	if !page.MustEval(`()=>{const link=document.querySelector('.kg-resource a.kg-link-button');return !!link&&link.getAttribute('href')==='https://www.kaggle.com/datasets/review/iris'&&link.textContent===I18N['kaggle.open_on_kaggle']&&link.target==='_blank'&&link.rel.includes('noopener');}`).Bool() {
		t.Fatal("result row does not link to the public Kaggle page")
	}

	// The query field answers to the keyboard.
	page.MustEval(`()=>document.querySelector('.kg-app').dispatchEvent(new KeyboardEvent('keydown',{key:'/',bubbles:true}))`)
	if !page.MustEval(`()=>document.activeElement===document.querySelector('.kg-search input')`).Bool() {
		t.Fatal("the search shortcut does not reach the query field")
	}
	if !page.MustEval(`()=>{const input=document.querySelector('.kg-search input');input.value='iris';input.dispatchEvent(new Event('input',{bubbles:true}));input.dispatchEvent(new KeyboardEvent('keydown',{key:'Escape',bubbles:true}));return input.value==='';}`).Bool() {
		t.Fatal("escape does not clear the query field")
	}

	// A required field is marked in the dialog and keeps its accessible name.
	page.MustEval(`()=>document.querySelector('[data-domain="overview"]').click()`)
	page.MustWait(`()=>document.querySelector('.kg-stats')!==null`)
	page.MustEval(`()=>[...document.querySelectorAll('#app button')].find(b=>b.textContent===I18N['kaggle.new_project']).click()`)
	page.MustWait(`()=>document.querySelector('dialog[open]')!==null`)
	if !page.MustEval(`()=>{const mark=document.querySelector('dialog .kg-required');return !!mark&&mark.textContent==='*'&&document.querySelector('dialog input').getAttribute('aria-required')==='true';}`).Bool() {
		t.Fatal("required fields are not marked in the dialog")
	}
	page.MustEval(`()=>[...document.querySelectorAll('dialog button')].find(b=>b.textContent===I18N['kaggle.cancel']).click()`)

	// The status banner stays reachable and names its tone with an icon.
	if !page.MustEval(`()=>getComputedStyle(document.querySelector('.kg-notice')).position==='sticky'`).Bool() {
		t.Fatal("the status banner does not stay visible while a view scrolls")
	}
	for _, theme := range []string{"standard", "fruity"} {
		page.MustEval(`theme=>document.body.dataset.theme=theme`, theme)
		if dir := os.Getenv("AURAGO_KAGGLE_SCREENSHOTS"); dir != "" {
			os.MkdirAll(dir, 0700)
			page.MustScreenshot(filepath.Join(dir, "kaggle-overview-"+theme+".png"))
		}
	}
	if got := page.MustEval(`()=>JSON.stringify(window.errors)`).Str(); got != "[]" {
		t.Fatal("browser errors", got)
	}
	page.MustEval(`()=>KaggleApp.dispose('polish')`)
}
