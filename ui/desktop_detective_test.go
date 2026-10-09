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
	failNextStart := false
	pendingEvent := false
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
				if failNextStart {
					c.Run.Status = "draft"
					c.Sources = nil
					c.Findings = nil
					c.Reports = nil
				}
				w.WriteHeader(http.StatusCreated)
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
			events := []detective.Event{{ID: 1, Kind: "plan", Text: "Primärquellen zur Nutzung prüfen", At: time.Now()}}
			if pendingEvent {
				pendingEvent = false
				events = append(events, detective.Event{ID: 2, Kind: "plan", Text: "Lauf angehalten", At: time.Now()})
			}
			json.NewEncoder(w).Encode(map[string]any{"events": events})
		case "cases/case_fixture/live":
			json.NewEncoder(w).Encode(fixtureLive(c))
		case "cases/case_fixture/run":
			var data map[string]string
			json.NewDecoder(r.Body).Decode(&data)
			if data["action"] == "start" && failNextStart {
				failNextStart = false
				w.WriteHeader(http.StatusInternalServerError)
				json.NewEncoder(w).Encode(map[string]string{"error": "research could not start"})
				return
			}
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
	mu.Lock()
	finished := c
	c.Run.Status = "running"
	c.Run.Usage.ActiveMS = 1000
	c.Sources = []detective.Source{
		{ID: "src_a", Title: "Quelle A", URL: "https://example.org/a", Status: "read", Excerpt: "Auszug A", RetrievedAt: time.Now()},
		{ID: "src_b", Title: "Quelle B", URL: "https://example.org/b", Status: "read", Excerpt: "Auszug B", RetrievedAt: time.Now()},
	}
	c.Findings = nil
	c.Reports = nil
	mu.Unlock()
	page.MustEval(`()=>DetectiveApp.render(document.querySelector('#host'),'test',ctx)`)
	page.MustElement(`.dt-case`)
	waitDetectiveQuiet(t, page)
	page.MustElement(`.dt-case`).MustClick()
	waitDetectiveQuiet(t, page)
	page.MustElement(`[data-tab=report]`).MustClick()
	waitDetectiveQuiet(t, page)
	page.MustEval(`()=>{const n=document.querySelector('.dt-content');const mark=document.createElement('i');mark.id='dt-stay';n.appendChild(mark);const bar=document.querySelector('.dt-export');const pin=document.createElement('i');pin.id='dt-export-stay';bar.appendChild(pin)}`)
	mu.Lock()
	c.Run.Usage.ActiveMS = 2000
	mu.Unlock()
	time.Sleep(4 * time.Second)
	if !page.MustEval(`()=>!!document.querySelector('.dt-content > #dt-stay')`).Bool() {
		t.Fatal("empty report content was replaced during live poll")
	}
	if !page.MustEval(`()=>!!document.querySelector('.dt-export > #dt-export-stay')`).Bool() {
		t.Fatal("export bar was rebuilt without a revision change")
	}
	mu.Lock()
	c.Reports = finished.Reports
	c.Findings = finished.Findings
	mu.Unlock()
	time.Sleep(4 * time.Second)
	for _, format := range []string{"md", "pdf", "docx"} {
		href := page.MustEval(`format => document.querySelector('a.dt-download[href*="format='+format+'"]')?.getAttribute('href') || ''`, format).Str()
		if !strings.Contains(href, "revision=1") {
			t.Fatal("export link did not follow the visible revision", format, href)
		}
	}
	if selected := page.MustEval(`()=>document.querySelector('.dt-revision')?.value || ''`).Str(); selected != "1" {
		t.Fatal("revision select did not follow the visible revision", selected)
	}
	page.MustElement(`[data-tab=sources]`).MustClick()
	page.MustElement(`.dt-source[data-id="src_a"] summary`).MustClick()
	if !page.MustEval(`()=>document.querySelector('.dt-source[data-id="src_a"]').open`).Bool() {
		t.Fatal("src_a did not expand")
	}
	mu.Lock()
	c.Run.Status = "waiting_for_user"
	c.Run.Reason = "budget_exhausted"
	mu.Unlock()
	time.Sleep(4 * time.Second)
	if !page.MustEval(`()=>!!document.querySelector('.dt-answer')`).Bool() {
		t.Fatal("answer field missing while waiting for the user")
	}
	if reason := page.MustElement(`.dt-status > p`).MustText(); reason != "Budget aufgebraucht" {
		t.Fatal("reason did not follow the run", reason)
	}
	page.MustEval(`()=>{const box=document.querySelector('.dt-answer');box.focus();box.value='Regenfass'}`)
	mu.Lock()
	c.Run.Usage.ActiveMS = 3500
	mu.Unlock()
	time.Sleep(4 * time.Second)
	if !page.MustEval(`()=>{const box=document.querySelector('.dt-answer');return !!(box&&document.activeElement===box&&box.value==='Regenfass')}`).Bool() {
		t.Fatal("focused answer was replaced while still waiting")
	}
	mu.Lock()
	c.Run.Status = "cancelled"
	c.Run.Reason = "user_stopped"
	c.Run.Usage.ActiveMS = 5000
	pendingEvent = true
	mu.Unlock()
	time.Sleep(4 * time.Second)
	status := page.MustElement(`.dt-status`).MustText()
	if !strings.Contains(status, "Gestoppt") || strings.Contains(status, "Recherche läuft") {
		t.Fatal("status did not follow the run", status)
	}
	if page.MustEval(`()=>!!document.querySelector('[data-do=stop]')`).Bool() {
		t.Fatal("stop remained after the run was cancelled")
	}
	if page.MustEval(`()=>!!document.querySelector('.dt-answer')`).Bool() {
		t.Fatal("answer field remained after the run left waiting_for_user")
	}
	if reason := page.MustElement(`.dt-status > p`).MustText(); reason != "Vom Nutzer gestoppt" {
		t.Fatal("reason did not follow the run", reason)
	}
	if sources := page.MustEval(`()=>document.querySelectorAll('.dt-source').length`).Int(); sources != 2 {
		t.Fatal("sources were discarded", sources)
	}
	if !page.MustEval(`()=>{const n=document.querySelector('.dt-source[data-id="src_a"]');return !!(n&&n.open)}`).Bool() {
		t.Fatal("src_a disclosure did not survive the poll")
	}
	event1 := page.MustEval(`()=>[...document.querySelectorAll('.dt-activity li')].filter(li=>li.textContent.includes('Primärquellen zur Nutzung prüfen')).length`).Int()
	event2 := page.MustEval(`()=>[...document.querySelectorAll('.dt-activity li')].filter(li=>li.textContent.includes('Lauf angehalten')).length`).Int()
	if event2 != 1 || event1 != 1 {
		t.Fatal("activity entries", event1, event2)
	}
	mu.Lock()
	c.Reports = finished.Reports
	c.Findings = finished.Findings
	mu.Unlock()
	page.MustElement(`[data-case="case_fixture"]`).MustClick()
	waitDetectiveQuiet(t, page)
	page.MustElement(`[data-tab=report]`).MustClick()
	page.MustElement(`.dt-report`)
	selectedWord := page.MustEval(`() => {
		const root = document.querySelector('.dt-report');
		const needle = 'Speicher';
		const walker = document.createTreeWalker(root, NodeFilter.SHOW_TEXT);
		let node;
		while ((node = walker.nextNode())) {
			const at = node.textContent.indexOf(needle);
			if (at < 0) continue;
			const range = document.createRange();
			range.setStart(node, at);
			range.setEnd(node, at + needle.length);
			const sel = window.getSelection();
			sel.removeAllRanges();
			sel.addRange(range);
			return sel.toString();
		}
		return '';
	}`).Str()
	if selectedWord != "Speicher" {
		t.Fatal("could not select report text", selectedWord)
	}
	mu.Lock()
	c.Run.Usage.ActiveMS = 8000
	mu.Unlock()
	time.Sleep(4 * time.Second)
	if got := page.MustEval(`()=>window.getSelection().toString()`).Str(); got != "Speicher" {
		t.Fatal("report selection was discarded", got)
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
	mu.Lock()
	failNextStart = true
	mu.Unlock()
	page.MustElement(`[data-do=new]`).MustClick()
	page.MustElement(`[name=topic]`).MustInput("Neuer Fall")
	page.MustElement(`.dt-form [type=submit]`).MustClick()
	waitDetectiveQuiet(t, page)
	page.MustElement(`.dt-error:not([hidden])`)
	draft := page.MustElement(`.dt-case.is-selected`).MustText()
	if !strings.Contains(draft, "Entwurf") || !strings.Contains(draft, "Neuer Fall") {
		t.Fatal("draft case was not selected", draft)
	}
	mu.Lock()
	c = finished
	failNextStart = false
	mu.Unlock()
	page.MustElement(`[data-case="case_fixture"]`).MustClick()
	waitDetectiveQuiet(t, page)
	page.MustElement(`.dt-report`)
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

func waitDetectiveQuiet(t *testing.T, page *rod.Page) {
	t.Helper()
	if !page.MustEval(`async () => {
		let last = -1;
		for (let i = 0; i < 40; i++) {
			await new Promise(resolve => setTimeout(resolve, 100));
			if (calls === last) return true;
			last = calls;
		}
		return false;
	}`).Bool() {
		t.Fatal("detective fixture did not settle")
	}
}

func fixtureLive(c detective.Case) detective.LiveView {
	view := detective.LiveView{
		ID: c.ID, UpdatedAt: c.UpdatedAt, Status: c.Run.Status, Phase: c.Run.Phase, Reason: c.Run.Reason,
		Effort: c.Request.Effort, Topic: c.Request.Topic, Usage: c.Run.Usage, Profile: c.Run.Profile,
		Sources: len(c.Sources), Findings: len(c.Findings), Reports: len(c.Reports),
	}
	if n := len(c.Reports); n > 0 {
		view.LatestRevision = c.Reports[n-1].Revision
	}
	if n := len(c.Sources); n > 0 {
		view.LatestSourceID = c.Sources[n-1].ID
	}
	return view
}
