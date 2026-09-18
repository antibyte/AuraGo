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

	"aurago/internal/detective"
	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
)

func TestDetectiveTranslations(t *testing.T) {
	var reference map[string]string
	for _, lang := range []string{"en", "de", "cs", "da", "el", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		b, err := os.ReadFile(filepath.Join("lang", "desktop", lang+".json"))
		if err != nil {
			t.Fatal(err)
		}
		var labels map[string]string
		if err = json.Unmarshal(b, &labels); err != nil {
			t.Fatal(err)
		}
		if reference == nil {
			reference = labels
		}
		for k := range reference {
			if strings.HasPrefix(k, "desktop.detective_") && labels[k] == "" {
				t.Errorf("%s missing %s", lang, k)
			}
		}
	}
}

func TestDesktopDetectiveBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	bin, ok := browserExecutable()
	if !ok {
		t.Fatal("Chrome required")
	}
	var mu sync.Mutex
	c := detective.Case{ID: "case_fixture", Request: detective.Request{Topic: "Wie lässt sich Regenwasser im Garten nutzen?", Effort: "normal"}, Run: detective.Run{Status: "running", Phase: "research", Profile: detective.Profiles()["normal"]}}
	exists := false
	starts := 0
	failExport := false
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.Dir(".")))
	mux.HandleFunc("/api/desktop/detective/", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		path := strings.TrimPrefix(r.URL.Path, "/api/desktop/detective/")
		switch path {
		case "capabilities":
			json.NewEncoder(w).Encode(map[string]any{"enabled": true, "ready": true, "profiles": detective.Profiles(), "tools": []string{"ddg_search", "web_scraper"}, "providers": []any{map[string]string{"id": "p", "name": "Fixture", "model": "model"}}, "provider_id": "p"})
		case "cases":
			if r.Method == "POST" {
				exists = true
				_ = json.NewDecoder(r.Body).Decode(&c.Request)
				json.NewEncoder(w).Encode(c)
			} else {
				items := []detective.Case{}
				if exists {
					items = append(items, c)
				}
				json.NewEncoder(w).Encode(map[string]any{"cases": items})
			}
		case "cases/case_fixture":
			json.NewEncoder(w).Encode(c)
		case "cases/case_fixture/events":
			json.NewEncoder(w).Encode(map[string]any{"events": []detective.Event{{ID: 1, Kind: "plan", Text: "Primärquellen zur Nutzung prüfen", At: time.Now()}}})
		case "cases/case_fixture/run":
			var data map[string]string
			json.NewDecoder(r.Body).Decode(&data)
			switch data["action"] {
			case "start", "continue":
				starts++
				c.Run.Status = "running"
			case "stop":
				c.Run.Status = "cancelled"
			case "finish":
				c.Run.Status = "completed"
				c.Run.Phase = "finished"
				src := detective.Source{ID: "src_1", Title: "Umweltamt – Regenwasser", URL: "https://example.org/water", Status: "read", Excerpt: "Regenwasser kann für die Gartenbewässerung genutzt werden.", RetrievedAt: time.Now()}
				c.Sources = []detective.Source{src}
				c.Reports = []detective.Report{{Revision: 1, Title: c.Request.Topic, Summary: "Regenwasser lässt sich im Garten sammeln und für die Bewässerung verwenden.", CreatedAt: time.Now(), Blocks: []detective.Block{{Type: "paragraph", Text: "Ein abgedeckter Speicher schützt das gesammelte Wasser. <script>window.injected=true</script>", Evidence: []string{"ev_1"}}, {Type: "table", Rows: [][]string{{"Nutzung", "Einordnung"}, {"Bewässerung", "Belegt"}}, Evidence: []string{"ev_1"}}}, Sources: c.Sources, Findings: []detective.Finding{{ID: "ev_1", SourceID: "src_1"}}}}
			}
			json.NewEncoder(w).Encode(c)
		case "cases/case_fixture/export":
			if failExport {
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(map[string]string{"error": "The selected PDF font cannot display this script."})
				return
			}
			a, err := detective.Export(c.Reports[0], r.URL.Query().Get("format"))
			if err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
			w.Header().Set("Content-Type", a.MIME)
			w.Write(a.Data)
		default:
			http.NotFound(w, r)
		}
	})
	mux.HandleFunc("/detective-fixture", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<!doctype html><html lang="de"><meta charset="utf-8"><link rel="stylesheet" href="/css/desktop-app-detective.css"><style>html,body{margin:0;height:100%;font-family:system-ui}body{--vd-text:#e4e8f0;--vd-theme-app-bg:#11161d;--vd-theme-panel-bg:#1b202a;--vd-theme-chrome-bg:#1d2430;--vd-theme-control-bg:#252d3a;--vd-theme-control-hover:#333e4f;--vd-theme-border:#ffffff25;--vd-theme-muted:#a7b1c3;--vd-accent:#a994fc;--vd-theme-accent-soft:#a994fc22;--vd-coral:#e97c75}body[data-mode=light]{--vd-text:#212b3c;--vd-theme-app-bg:#f8fafc;--vd-theme-panel-bg:#e5e9ef;--vd-theme-chrome-bg:#d7dfe9;--vd-theme-control-bg:#f8fafc;--vd-theme-border:#65748c40;--vd-theme-muted:#536279;--vd-accent:#6552b5;--vd-theme-accent-soft:#6552b520}body[data-theme=fruity]{--vd-accent:#cb8144}#host{height:100%}</style><div id="host"></div><script>window.errors=[];window.calls=0;addEventListener('error',e=>errors.push(e.message));addEventListener('unhandledrejection',e=>errors.push(String(e.reason)));</script><script src="/js/desktop/apps/detective-views.js"></script><script src="/js/desktop/apps/detective.js"></script><script>window.ready=(async()=>{const labels=await(await fetch('/lang/desktop/de.json')).json();window.ctx={t:k=>labels[k]||k,confirmDialog:async()=>true,api:async(path,opts)=>{calls++;const r=await fetch(path,opts);const j=await r.json();if(!r.ok)throw new Error(j.error);return j}};DetectiveApp.render(document.querySelector('#host'),'test',ctx)})();</script></html>`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	launch := launcher.New().Bin(bin).Headless(true).NoSandbox(true)
	browser := rod.New().ControlURL(launch.MustLaunch()).MustConnect()
	defer launch.Cleanup()
	defer browser.Close()
	page := browser.MustPage(srv.URL + "/detective-fixture").Timeout(60 * time.Second)
	defer page.Close()
	page.MustSetViewport(1440, 900, 1, false)
	page.MustWaitLoad()
	page.MustElement(`[name=topic]`).MustInput(c.Request.Topic)
	page.MustElement(`.dt-form [type=submit]`).MustClick()
	page.MustElement(`[data-do=stop]`).MustClick()
	page.MustElement(`[data-do=continue]`).MustClick()
	page.MustElement(`[data-do=finish]`).MustClick()
	page.MustElement(`.dt-report table`)
	page.MustElement(`[data-tab=sources]`).MustClick()
	page.MustElement(`.dt-source summary`).MustClick()
	if !page.MustEval(`()=>document.querySelector('.dt-source').open`).Bool() {
		t.Fatal("source did not expand")
	}
	page.MustElement(`[data-tab=report]`).MustClick()
	if page.MustEval(`()=>!!window.injected`).Bool() {
		t.Fatal("source/report injection executed")
	}
	for _, format := range []string{"md", "pdf", "docx"} {
		value := page.MustEval(`async(format)=>{const r=await fetch('/api/desktop/detective/cases/case_fixture/export?format='+format+'&revision=1');return r.ok&&(await r.arrayBuffer()).byteLength>100}`, format)
		if !value.Bool() {
			t.Fatal("download failed", format)
		}
	}
	mu.Lock()
	failExport = true
	mu.Unlock()
	page.MustElement(`a.dt-download[href*="format=pdf"]`).MustClick()
	page.MustElement(`.dt-error:not([hidden])`)
	if !strings.Contains(page.MustElement(`.dt-error`).MustText(), "PDF font") || !strings.HasSuffix(page.MustInfo().URL, "/detective-fixture") {
		t.Fatal("failed download replaced the Desktop or hid its error")
	}
	mu.Lock()
	failExport = false
	mu.Unlock()
	page.MustElement(`[data-tab=report]`).MustClick()
	dir := filepath.Join("..", "reports", "detective")
	os.MkdirAll(dir, 0755)
	for _, theme := range []string{"standard", "fruity"} {
		for _, mode := range []string{"light", "dark"} {
			page.MustEval(`(theme,mode)=>{document.body.dataset.theme=theme;document.body.dataset.mode=mode}`, theme, mode)
			for _, width := range []int{1440, 420} {
				page.MustSetViewport(width, 900, 1, false)
				page.MustScreenshot(filepath.Join(dir, fmt.Sprintf("%s-%s-%d.png", theme, mode, width)))
				if page.MustEval(`()=>document.querySelector('.dt-app').scrollWidth>innerWidth+1`).Bool() {
					t.Fatal("horizontal overflow", theme, mode, width)
				}
			}
		}
	}
	for i := 0; i < 3; i++ {
		page.MustEval(`()=>{DetectiveApp.dispose('test');DetectiveApp.render(document.querySelector('#host'),'test',ctx)}`)
		page.MustElement(`.dt-report`)
	}
	page.MustEval(`()=>DetectiveApp.dispose('test')`)
	time.Sleep(100 * time.Millisecond)
	before := page.MustEval(`()=>calls`).Int()
	time.Sleep(3300 * time.Millisecond)
	if page.MustEval(`()=>calls`).Int() != before {
		t.Fatal("polling leaked after dispose")
	}
	if errors := page.MustEval(`()=>errors`).Arr(); len(errors) > 0 {
		t.Fatal(errors)
	}
	mu.Lock()
	defer mu.Unlock()
	if starts != 2 {
		t.Fatal("unexpected starts", starts)
	}
}
