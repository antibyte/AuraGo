package server

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/kaggle"
	"aurago/ui"
)

func TestKaggleJobControlsBrowser(t *testing.T) {
	browser := personalRadioBrowser(t)
	cfg := &config.Config{}
	cfg.Directories.DataDir = t.TempDir()
	cfg.VirtualDesktop.Enabled = true
	cfg.Kaggle = config.DefaultKaggleConfig()
	s := &Server{Cfg: cfg}
	defer s.closeKaggleServices()
	id := strings.Repeat("a", 32)
	seedKaggleUnknownJob(t, s, id, "unknown")
	translations, err := ui.Content.ReadFile("lang/kaggle/de.json")
	if err != nil {
		t.Fatal(err)
	}
	var admin, configured, network, reject atomic.Bool
	var resolves atomic.Int32
	admin.Store(true)
	mux := http.NewServeMux()
	mux.HandleFunc("/api/kaggle/", func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/kaggle/status":
			// Only the UI view changes. The service stays disabled without credentials
			// or network access, so this fixture cannot contact Kaggle.
			status := s.kaggleServices.Platform.Status()
			status["configured"], status["can_resolve_unknown"] = configured.Load(), admin.Load()
			status["access"] = map[string]bool{"network": network.Load()}
			kaggleJSON(w, 200, status)
		case "/api/kaggle/jobs":
			jobs, err := s.kaggleServices.Platform.Store.List(r.Context())
			if err != nil {
				t.Error(err)
				w.WriteHeader(500)
				return
			}
			for _, state := range []string{"queued", "running", "remote_running"} {
				jobs = append(jobs, kaggle.Job{ID: state, Domain: "benchmarks", Operation: state, Status: state})
			}
			kaggleJSON(w, 200, jobs)
		case "/api/kaggle/jobs/" + id + "/resolve-unknown":
			resolves.Add(1)
			if reject.Load() {
				kaggleJSON(w, 409, map[string]string{"error": "fixture conflict"})
				return
			}
			s.handleKaggle(w, r)
		case "/api/kaggle/events":
			w.WriteHeader(204)
		default:
			s.handleKaggle(w, r)
		}
	})
	mux.HandleFunc("/fixture", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<!doctype html><meta charset="utf-8"><link rel="stylesheet" href="/css/desktop-app-kaggle.css"><style>body{margin:0}.host{height:760px;width:390px}</style><body data-theme="standard"><div class="host" id="app"></div><script>window.I18N=%s;window.errors=[];addEventListener('error',e=>errors.push(e.message));addEventListener('unhandledrejection',e=>errors.push(String(e.reason)));</script><script src="/js/desktop/apps/kaggle.js"></script><script>KaggleApp.render(document.getElementById('app'),'jobs',{t:k=>I18N[k]||k});</script>`, translations)
	})
	mux.Handle("/", http.FileServer(http.FS(ui.Content.FS)))
	origin := httptest.NewServer(mux)
	defer origin.Close()
	page := browser.MustPage(origin.URL + "/fixture").Timeout(45 * time.Second)
	defer page.Close()
	page.MustWait(`()=>document.querySelector('.kg-stats')!==null`)
	refresh := func() {
		t.Helper()
		page.MustEval(`()=>document.querySelector('[data-domain="jobs"]').click()`)
		page.MustWait(`()=>document.querySelectorAll('.kg-resource').length===4`)
	}
	refresh()
	checkCancel := func(blocked bool, reason string) {
		t.Helper()
		if !page.MustEval(`(blocked,reason)=>['queued','running','remote_running'].every(state=>{
const row=[...document.querySelectorAll('.kg-resource')].find(r=>r.querySelector('strong').textContent===state);
const button=[...row.querySelectorAll('button')].find(b=>b.textContent===I18N['kaggle.cancel']);
return button.disabled===(state==='remote_running'&&blocked)&&(state!=='remote_running'||!blocked||button.title===reason);
})`, blocked, reason).Bool() {
			t.Fatal("cancel gating did not distinguish local from remote work")
		}
	}
	checkCancel(true, page.MustEval(`()=>I18N['kaggle.credentials_missing']`).Str())
	configured.Store(true)
	refresh()
	checkCancel(true, page.MustEval(`()=>I18N['kaggle.allow_internet']+': '+I18N['kaggle.disabled']`).Str())
	network.Store(true)
	refresh()
	checkCancel(false, "") // Disabled/read-only integration does not prevent administrative cancellation.
	admin.Store(false)
	refresh()
	if page.MustEval(`()=>[...document.querySelectorAll('.kg-resource button')].some(b=>b.textContent===I18N['kaggle.resolve_unknown'])`).Bool() {
		t.Fatal("resolution action visible to non-admin")
	}
	admin.Store(true)
	refresh()
	open := func() {
		page.MustEval(`()=>[...document.querySelectorAll('.kg-resource button')].find(b=>b.textContent===I18N['kaggle.resolve_unknown']).click()`)
		page.MustWait(`()=>document.querySelector('dialog[open]')!==null`)
	}
	open()
	if !page.MustEval(`()=>{
const d=document.querySelector('dialog');return d.getAttribute('aria-label')===I18N['kaggle.resolve_unknown']&&d.textContent.includes(I18N['kaggle.resolve_unknown_help'])&&d.textContent.includes(I18N['kaggle.resolve_unknown_confirm'])&&document.activeElement===d.querySelector('input[type=checkbox]')&&[...d.querySelectorAll('button')].find(b=>b.textContent===I18N['kaggle.resolve_unknown']).disabled;
}`).Bool() {
		t.Fatal("resolution dialog lacks explicit confirmation or accessible focus")
	}
	for _, theme := range []string{"standard", "fruity"} {
		page.MustEval(`theme=>document.body.dataset.theme=theme`, theme)
		if !page.MustEval(`()=>{const d=document.querySelector('dialog');return d.scrollWidth<=d.clientWidth+1}`).Bool() {
			t.Fatal(theme, "resolution dialog overflows")
		}
	}
	page.MustEval(`()=>[...document.querySelectorAll('dialog button')].find(b=>b.textContent===I18N['kaggle.cancel']).click()`)
	if resolves.Load() != 0 {
		t.Fatal("cancelled confirmation submitted")
	}
	open()
	reject.Store(true)
	page.MustElement("dialog input[type=checkbox]").MustClick()
	submit := `()=>{const b=[...document.querySelectorAll('dialog button')].find(b=>b.textContent===I18N['kaggle.resolve_unknown']);b.click();b.click();}`
	page.MustEval(submit)
	page.MustWait(`()=>document.querySelector('dialog [role=alert]')?.textContent.includes(I18N['kaggle.conflict'])&&!document.querySelector('dialog [aria-busy=true]')`)
	if resolves.Load() != 1 || !page.MustElement("dialog input").MustProperty("checked").Bool() {
		t.Fatal("failed confirmation lost its draft or duplicated a request")
	}
	reject.Store(false)
	page.MustEval(submit)
	page.MustWait(`()=>!document.querySelector('dialog')&&document.querySelector('.kg-panel').textContent.includes(I18N['kaggle.state_resolved_unknown'])`)
	if resolves.Load() != 2 {
		t.Fatal("double click duplicated resolution")
	}
	job, err := s.kaggleServices.Platform.Store.Get(t.Context(), id)
	if err != nil || job.Status != "resolved_unknown" || job.Error != "response lost" {
		t.Fatal("browser resolution did not preserve receipt", err)
	}
	if got := page.MustEval(`()=>JSON.stringify(errors)`).Str(); got != "[]" {
		t.Fatal("browser errors", got)
	}
	page.MustEval(`()=>KaggleApp.dispose('jobs')`)
}
