package server

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/kaggle"
	"aurago/internal/notebooks"
	"aurago/ui"
	"github.com/go-rod/rod/lib/input"
)

// The real Docker smoke qualifies Python state. This fixture deliberately sends
// events before the HTTP response to exercise browser correlation deterministically.
type kaggleBrowserRuntime struct{}

func (kaggleBrowserRuntime) EnsureImage(context.Context) error { return nil }
func (kaggleBrowserRuntime) Create(_ context.Context, id string, _ config.KaggleLocalConfig, _ bool) (string, error) {
	return id, nil
}
func (kaggleBrowserRuntime) Remove(context.Context, string) error { return nil }
func (kaggleBrowserRuntime) Cleanup(context.Context) error        { return nil }
func (kaggleBrowserRuntime) CopyIn(context.Context, string, *kaggle.Service, string, []string) error {
	return nil
}
func (kaggleBrowserRuntime) Attach(ctx context.Context, _ string) (io.ReadWriteCloser, error) {
	a, b := net.Pipe()
	go func() { <-ctx.Done(); b.Close() }()
	go func() {
		defer b.Close()
		enc := json.NewEncoder(b)
		enc.Encode(map[string]any{"protocol": 1, "kind": "ready"})
		scan := bufio.NewScanner(b)
		for scan.Scan() {
			var m map[string]any
			if json.Unmarshal(scan.Bytes(), &m) != nil {
				return
			}
			if m["command"] != "execute" {
				continue
			}
			event := map[string]any{"protocol": 1, "kind": "output", "execution_id": m["execution_id"], "revision": m["revision"], "cell_id": m["cell_id"], "output_type": "stream", "content": map[string]any{"name": "stdout", "text": "42\n"}}
			enc.Encode(event)
			event["kind"] = "done"
			delete(event, "content")
			enc.Encode(event)
		}
	}()
	return a, nil
}

func TestKaggleNotebookBrowser(t *testing.T) {
	auditFile := os.Getenv("AURAGO_KAGGLE_AUDIT_URL_FILE")
	if auditFile == "" && os.Getenv("AURAGO_RUN_BROWSER_SMOKE") != "1" {
		t.Skip("set AURAGO_RUN_BROWSER_SMOKE=1 for browser verification")
	}
	cfg := &config.Config{}
	cfg.Directories.DataDir = t.TempDir()
	cfg.VirtualDesktop.Enabled = true
	cfg.Agent.AllowFilesystemWrite = true
	cfg.Agent.AllowPython = true
	cfg.Agent.AllowNetworkRequests = true
	cfg.Docker.Enabled = true
	cfg.Kaggle = config.DefaultKaggleConfig()
	cfg.Kaggle.Enabled = true
	cfg.Kaggle.ReadOnly = false
	cfg.Kaggle.Local.Enabled = true
	cfg.Kaggle.Local.AllowExecute = true
	s := &Server{Cfg: cfg}
	b, err := s.ensureKaggleServices()
	if err != nil {
		t.Fatal(err)
	}
	defer s.closeKaggleServices()
	b.Kernels.Close()
	b.Kernels = notebooks.NewKernels(context.Background(), b.Platform, b.Documents, kaggleBrowserRuntime{})
	p, err := b.Platform.CreateProject(context.Background(), "Browser fixture")
	if err != nil {
		t.Fatal(err)
	}
	raw := `{"nbformat":4,"nbformat_minor":5,"metadata":{"vendor":{"preserve":true}},"cells":[{"id":"one","cell_type":"code","source":"answer = 40","metadata":{},"outputs":[{"output_type":"display_data","metadata":{},"data":{"text/html":"<table><tr><td>Safe table</td></tr></table><script>fetch('/attack')</script><img src='/attack' onerror='parent.pwned=true'>","application/javascript":"fetch('/attack')"}}],"execution_count":null},{"id":"two","cell_type":"code","source":"print(answer + 2)","metadata":{},"outputs":[],"execution_count":null},{"id":"md","cell_type":"markdown","source":"![a](attachment:a.png)","metadata":{},"attachments":{"a.png":{"image/png":"eA=="}}}]}`
	if _, err = b.Documents.Save(context.Background(), p.ID, "fixture.ipynb", "*", []byte(raw)); err != nil {
		t.Fatal(err)
	}
	translations, err := ui.Content.ReadFile("lang/kaggle/de.json")
	if err != nil {
		t.Fatal(err)
	}
	var attacks atomic.Int32
	mux := http.NewServeMux()
	mux.HandleFunc("/attack", func(w http.ResponseWriter, r *http.Request) { attacks.Add(1); w.WriteHeader(204) })
	mux.HandleFunc("/api/kaggle/", func(w http.ResponseWriter, r *http.Request) {
		// Manual review offers synthetic search results, never a provider call.
		if auditFile != "" {
			switch r.URL.Path {
			case "/api/kaggle/status":
				status := b.Platform.Status()
				status["configured"] = true
				kaggleJSON(w, 200, status)
				return
			case "/api/kaggle/test":
				kaggleJSON(w, 200, map[string]string{"account": "local-review-fixture"})
				return
			case "/api/kaggle/quotas":
				kaggleJSON(w, 200, map[string]any{"notebooks": map[string]int{"cpuHoursRemaining": 30}})
				return
			case "/api/kaggle/operations":
				var request kaggle.Request
				if !decodeKaggle(w, r, &request) {
					return
				}
				op, err := kaggle.Lookup(request.Domain, request.Operation)
				if err != nil || op.Grant != "read" {
					kaggleJSON(w, 403, map[string]string{"error": "Remote mutations are disabled in this local fixture"})
					return
				}
				if request.Operation == "get_kernel" {
					kaggleJSON(w, 200, map[string]any{"data": map[string]any{"blob": map[string]string{"source": raw}}})
					return
				}
				titles := map[string]string{"datasets": "Iris flowers", "models": "Classification model", "notebooks": "Iris analysis", "competitions": "Tabular practice", "benchmarks": "Classification accuracy", "forums": "Working with notebook data"}
				kaggleJSON(w, 200, map[string]any{"data": map[string]any{"items": []map[string]string{{"title": titles[request.Domain], "ref": "review/iris", "subtitle": "Synthetic local review data", "description": "Local fixture. External text stays inert: <script>alert(1)</script>"}}}})
				return
			}
		}
		if r.URL.Path == "/api/kaggle/runtime" && r.Method == "POST" {
			body, _ := io.ReadAll(r.Body)
			r.Body = io.NopCloser(strings.NewReader(string(body)))
			if strings.Contains(string(body), `"operation":"start"`) || strings.Contains(string(body), `"operation":"execute"`) || strings.Contains(string(body), `"operation":"restart"`) {
				record := httptest.NewRecorder()
				s.handleKaggle(record, r)
				time.Sleep(650 * time.Millisecond)
				for k, values := range record.Header() {
					w.Header()[k] = values
				}
				w.WriteHeader(record.Code)
				w.Write(record.Body.Bytes())
				return
			}
		}
		s.handleKaggle(w, r)
	})
	mux.HandleFunc("/fixture", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<!doctype html><meta charset="utf-8"><link rel="stylesheet" href="/css/desktop-app-kaggle.css"><style>body{margin:0;background:#0d1420}.host{height:760px;width:940px;margin:12px;display:inline-block;vertical-align:top}#b{width:360px}</style><body data-theme="standard"><div class="host" id="a"></div><div class="host" id="b"></div><script>window.I18N=%s;window.errors=[];window.addEventListener('error',e=>errors.push(e.message));window.addEventListener('unhandledrejection',e=>errors.push(String(e.reason)));window.pwned=false;</script><script src="/js/vendor/purify.min.js"></script><script src="/js/desktop/apps/kaggle-notebook.js"></script><script src="/js/desktop/apps/kaggle.js"></script><script>const ctx={t:k=>I18N[k]||k};KaggleApp.render(document.getElementById('a'),'a',ctx);KaggleApp.render(document.getElementById('b'),'b',ctx);</script>`, translations)
	})
	mux.HandleFunc("/review", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<!doctype html><html lang="de"><meta charset="utf-8"><meta name="viewport" content="width=device-width"><link rel="stylesheet" href="/css/desktop-app-kaggle.css"><style>body{margin:0;background:#0d1420}header{padding:8px;color:#cbd5e1;font:12px system-ui}header a{color:#7dd3fc;margin:0 10px}.host{height:calc(100vh - 38px);width:100%%;max-width:100%%}body.narrow .host{width:390px}body[data-theme=fruity]{background:#edf0f5}</style><body><header>Lokale UI-Prüfung · keine Kaggle-Verbindung <a href="?theme=standard">Standard</a><a href="?theme=fruity">Fruity</a><a href="?theme=standard&narrow=1">Schmal</a></header><div class="host" id="a"></div><script>window.I18N=%s;const query=new URLSearchParams(location.search);document.body.dataset.theme=query.get('theme')==='fruity'?'fruity':'standard';document.body.classList.toggle('narrow',query.has('narrow'));window.errors=[];window.addEventListener('error',e=>errors.push(e.message));window.addEventListener('unhandledrejection',e=>errors.push(String(e.reason)));</script><script src="/js/vendor/purify.min.js"></script><script src="/js/desktop/apps/kaggle-notebook.js"></script><script src="/js/desktop/apps/kaggle.js"></script><script>KaggleApp.render(document.getElementById('a'),'review',{t:k=>I18N[k]||k});</script></html>`, translations)
	})
	mux.Handle("/", http.FileServer(http.FS(ui.Content.FS)))
	origin := httptest.NewServer(mux)
	defer origin.Close()
	// Opt-in manual review uses the same real UI and local services as this test.
	// It never contacts Kaggle and stops after the reviewer writes <file>.stop.
	if auditFile != "" {
		if err := os.WriteFile(auditFile, []byte(origin.URL+"/review"), 0600); err != nil {
			t.Fatal(err)
		}
		deadline := time.NewTimer(30 * time.Minute)
		defer deadline.Stop()
		tick := time.NewTicker(time.Second)
		defer tick.Stop()
		for {
			select {
			case <-deadline.C:
				t.Fatal("manual review deadline reached")
			case <-tick.C:
				if _, err := os.Stat(auditFile + ".stop"); err == nil {
					return
				}
			}
		}
	}
	browser := personalRadioBrowser(t)
	page := browser.MustPage().Timeout(45 * time.Second)
	defer page.Close()
	page.MustNavigate(origin.URL + "/fixture").MustWaitLoad()
	page.MustWait(`()=>document.querySelectorAll('.kg-resource a[download]').length===2`)
	if !page.MustEval(`()=>getComputedStyle(document.querySelector('#b .kg-nav')).flexDirection==='row'`).Bool() {
		t.Fatal("narrow window does not use horizontal navigation")
	}
	// Invalid and conflicting creation keeps the dialog and its draft available.
	page.MustEval(`()=>{const button=[...document.querySelectorAll('#a button')].find(b=>b.textContent===I18N['kaggle.new_notebook']);button.focus();button.click();}`)
	page.MustEval(`()=>[...document.querySelectorAll('#a dialog button')].find(b=>b.textContent===I18N['kaggle.continue']).click()`)
	page.MustWait(`()=>document.querySelector('#a dialog[open] [role="alert"]')?.textContent.includes(I18N['kaggle.required'])`)
	if !page.MustEval(`()=>document.activeElement===document.querySelector('#a dialog input')&&document.querySelector('#a dialog').getAttribute('aria-label')===I18N['kaggle.new_notebook']`).Bool() {
		t.Fatal("creation dialog lost its accessible name or invalid field focus")
	}
	page.MustElement("#a dialog input").MustInput("fixture.ipynb")
	page.MustEval(`()=>[...document.querySelectorAll('#a dialog button')].find(b=>b.textContent===I18N['kaggle.continue']).click()`)
	page.MustWait(`()=>document.querySelector('#a dialog[open] [role="alert"]')?.textContent.includes(I18N['kaggle.conflict'])`)
	if page.MustElement("#a dialog input").MustProperty("value").Str() != "fixture.ipynb" {
		t.Fatal("conflicting filename draft was lost")
	}
	page.MustEval(`()=>[...document.querySelectorAll('#a dialog button')].find(b=>b.textContent===I18N['kaggle.cancel']).click()`)
	if !page.MustEval(`()=>document.activeElement.textContent===I18N['kaggle.new_notebook']`).Bool() {
		t.Fatal("dialog close lost the originating keyboard focus")
	}
	page.MustElement("#a .kg-resource button").MustClick()
	page.MustElement("#b .kg-resource button").MustClick()
	page.MustWait(`()=>document.querySelectorAll('.cm-editor').length===6`)
	if !page.MustEval(`()=>[...document.querySelectorAll('.cm-content')].every(e=>e.getAttribute('aria-label'))`).Bool() {
		t.Fatal("notebook editors have no accessible cell labels")
	}
	if !page.MustEval(`()=>[...document.querySelectorAll('.kg-html-output')].every(f=>f.getAttribute('sandbox')===''&&!f.srcdoc.includes('<script')&&!f.srcdoc.includes('onerror'))&&!window.pwned`).Bool() {
		t.Fatal("active notebook HTML")
	}
	if attacks.Load() != 0 {
		t.Fatal("notebook external content executed")
	}
	// A real CodeMirror edit, conditional save, and a stale second window.
	page.MustElement("#a .cm-content").MustKeyActions().Press(input.ControlLeft).Type(input.KeyA).Release(input.ControlLeft).MustDo()
	page.MustElement("#a .cm-content").MustInput("answer = 41")
	page.MustElement(`#a [data-action="save"]`).MustClick()
	page.MustWait(`()=>document.querySelector('#a .kg-state').textContent===I18N['kaggle.saved']`)
	page.MustElement("#b .cm-content").MustKeyActions().Press(input.ControlLeft).Type(input.KeyA).Release(input.ControlLeft).MustDo()
	page.MustElement("#b .cm-content").MustInput("answer = 99")
	page.MustElement(`#b [data-action="save"]`).MustClick()
	page.MustWait(`()=>document.querySelector('#b .kg-state').textContent.includes(I18N['kaggle.conflict'])`)
	if !page.MustEval(`()=>Object.keys(localStorage).some(k=>k.startsWith('aurago.kaggle.draft.')&&JSON.parse(localStorage[k]).dirty)`).Bool() {
		t.Fatal("conflicting draft lost")
	}
	// The ready/output events arrive before their POST responses.
	page.MustElement(`#a [data-action="local"]`).MustClick()
	page.MustWait(`()=>document.querySelector('#a dialog[open]')!==null`)
	page.MustEval(`()=>[...document.querySelectorAll('#a dialog button')].find(b=>b.textContent===I18N['kaggle.continue']).click()`)
	page.MustWait(`()=>document.querySelector('#a .kg-state').textContent===I18N['kaggle.kernel_ready']`)
	page.MustElement(`#a [data-action="run_cell"]`).MustClick()
	page.MustWait(`()=>document.querySelector('#a .kg-outputs').textContent.includes('42')`)
	page.MustElement(`#a [data-action="restart"]`).MustClick()
	page.MustWait(`()=>document.querySelector('#a .kg-state').textContent.includes(I18N['kaggle.state_lost'])&&document.querySelector('#a .kg-state').textContent.includes(I18N['kaggle.kernel_ready'])`)
	page.MustElement(`#a [data-action="run_cell"]`).MustClick()
	page.MustWait(`()=>document.querySelector('#a .kg-outputs').textContent.includes('42')`)
	page.MustElement(`#a [data-action="save"]`).MustClick()
	page.MustWait(`()=>document.querySelector('#a .kg-state').textContent===I18N['kaggle.saved']`)
	for _, theme := range []string{"standard", "fruity"} {
		page.MustEval(`theme=>document.body.dataset.theme=theme`, theme)
		if !page.MustEval(`()=>document.querySelector('#b .kg-main').scrollWidth<=document.querySelector('#b .kg-main').clientWidth+1`).Bool() {
			t.Fatal(theme, "horizontal overflow")
		}
		if dir := os.Getenv("AURAGO_KAGGLE_SCREENSHOTS"); dir != "" {
			os.MkdirAll(dir, 0700)
			page.MustScreenshot(filepath.Join(dir, "kaggle-"+theme+".png"))
		}
	}
	page.MustEval(`()=>{KaggleApp.dispose('a');KaggleApp.dispose('b');}`)
	page.MustWait(`()=>document.querySelectorAll('dialog[open]').length===0`)
	if got := page.MustEval(`()=>JSON.stringify(window.errors)`).Str(); got != "[]" {
		t.Fatal("browser errors", got)
	}
	doc, err := b.Documents.Load(p.ID, "fixture.ipynb")
	if err != nil || !strings.Contains(string(doc.Content), "vendor") || !strings.Contains(string(doc.Content), "attachments") || strings.Contains(string(doc.Content), "answer = 99") {
		t.Fatal("notebook preservation/conflict failed", err)
	}
	active := 0
	for _, session := range b.Kernels.List() {
		if session.Status == "idle" {
			active++
		} else if session.Status != "stopped" {
			t.Fatal("unexpected kernel state", session.Status)
		}
	}
	if active != 1 {
		t.Fatal("closing window stopped or duplicated kernel")
	}
}
