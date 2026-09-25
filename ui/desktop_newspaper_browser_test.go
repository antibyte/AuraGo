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

	"aurago/internal/newspaper"
	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
)

func TestNewspaperTranslations(t *testing.T) {
	var reference map[string]string
	for _, locale := range []string{"en", "de", "cs", "da", "el", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		b, err := os.ReadFile(filepath.Join("lang", "desktop", locale+".json"))
		if err != nil {
			t.Fatal(err)
		}
		labels := map[string]string{}
		if err := json.Unmarshal(b, &labels); err != nil {
			t.Fatal(err)
		}
		if reference == nil {
			reference = labels
		}
		for key := range reference {
			if strings.HasPrefix(key, "newspaper.") && labels[key] == "" {
				t.Errorf("%s missing %s", locale, key)
			}
		}
		if labels["desktop.app_newspaper"] == "" {
			t.Errorf("%s missing app name", locale)
		}
		for _, path := range []string{filepath.Join("lang", "config", "newspaper", locale+".json"), filepath.Join("lang", "dashboard", locale+".json")} {
			if _, err := os.Stat(path); err != nil {
				t.Errorf("%s: %v", path, err)
			}
		}
	}
}

func TestDesktopNewspaperBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	bin, ok := browserExecutable()
	if !ok {
		t.Fatal("Chrome or Edge required")
	}
	var mu sync.Mutex
	correctionRequest := ""
	challengeCode := "agentmail_rejected"
	localDate := "2026-09-25"
	p := newspaper.DefaultProfile()
	p.Version = 1
	p.Name = "Der Morgen"
	p.EmailTo = "reader@example.org"
	p.EmailAccountID = "agentmail"
	now := time.Date(2026, 9, 25, 8, 0, 0, 0, time.UTC)
	e := newspaper.Edition{ID: "issue_1", LocalDate: "2026-09-25", Revision: 1, Title: p.Name, Language: "de", Place: "Berlin", CreatedAt: now, CutoffAt: now}
	for i := 0; i < 8; i++ {
		id := fmt.Sprintf("src-%d", i)
		headline := fmt.Sprintf("Die Stadt plant eine neue öffentliche Bibliothek im Bezirk %d", i)
		if i == 0 {
			headline = "Die Stadt plant eine neue öffentliche Bibliothek für alle Bezirke und eröffnet eine breite Debatte über Kultur und Bildung"
		}
		s := newspaper.Source{ID: id, URL: "https://example.org/article", Publisher: "Stadtblatt", Title: headline, RetrievedAt: now, Excerpt: "Der Stadtrat hat den Plan für eine neue öffentliche Bibliothek vorgestellt. Weitere Details folgen."}
		e.Sources = append(e.Sources, s)
		e.Stories = append(e.Stories, newspaper.Story{ID: fmt.Sprintf("story-%d", i), Section: "culture", Headline: headline, Deck: "Die Pläne werden im Herbst öffentlich diskutiert.", SourceIDs: []string{id}, SingleSource: true, Paragraphs: []newspaper.Paragraph{{Text: "Der Stadtrat stellte den Plan vor. <script>window.injected=true</script>", SourceIDs: []string{id}, EvidenceQuote: "Der Stadtrat hat den Plan für eine neue öffentliche Bibliothek vorgestellt."}}})
	}
	if err := e.Seal(); err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.Dir(".")))
	mux.HandleFunc("/api/desktop/newspaper/", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		path := strings.TrimPrefix(r.URL.Path, "/api/desktop/newspaper/")
		switch path {
		case "capabilities":
			json.NewEncoder(w).Encode(map[string]any{"enabled": true, "read_only": false, "research_ready": true, "email_ready": false, "telegram_ready": false, "email_allowed": true, "telegram_allowed": true, "email_accounts": []any{map[string]string{"id": "agentmail", "name": "AgentMail"}}, "daily": false, "local_date": localDate})
		case "profile":
			if r.Method == http.MethodPut {
				if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
					t.Error(err)
				}
				p.Version++
			}
			json.NewEncoder(w).Encode(p)
		case "editions":
			if r.Method == http.MethodPost {
				var req struct {
					CorrectionNote string `json:"correction_note"`
				}
				if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
					t.Error(err)
				}
				correctionRequest = req.CorrectionNote
				json.NewEncoder(w).Encode(newspaper.Run{ID: "run-2", Status: "running"})
				return
			}
			json.NewEncoder(w).Encode(map[string]any{"editions": []any{map[string]any{"id": e.ID, "local_date": e.LocalDate, "revision": 1, "title": e.Title, "lead": e.Stories[0].Headline, "headlines": []string{e.Stories[0].Headline}, "sections": []string{"culture"}, "stories": len(e.Stories)}}, "latest_run": newspaper.Run{}})
		case "editions/issue_1":
			json.NewEncoder(w).Encode(e)
		case "editions/issue_1/deliveries":
			json.NewEncoder(w).Encode(map[string]any{"deliveries": []any{}})
		case "email/challenge":
			w.WriteHeader(http.StatusBadGateway)
			json.NewEncoder(w).Encode(map[string]any{"code": challengeCode, "provider_status": 403, "error": "raw server error"})
		default:
			http.NotFound(w, r)
		}
	})
	mux.HandleFunc("/newspaper-fixture", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, `<!doctype html><html lang="de"><meta charset="utf-8"><link rel="stylesheet" href="/css/desktop-app-newspaper.css"><style>html,body{margin:0;height:100%}body{--vd-theme-app-bg:#e4e4e4;--vd-theme-panel-bg:#eee;--vd-theme-chrome-bg:#ddd;--vd-theme-border:#5555;--vd-theme-accent-soft:#b99b9b33;--vd-theme-muted:#555;--vd-accent:#8b3338;--vd-text:#222}#host{height:100%}</style><body class="desktop-body" data-theme="fruity" data-fruity-mode="light"><div id="host"></div><script>window.errors=[];window.calls=0;addEventListener('error',e=>errors.push(e.message));addEventListener('unhandledrejection',e=>errors.push(String(e.reason)));</script><script src="/js/desktop/apps/newspaper.js"></script><script>window.ready=(async()=>{const labels=await(await fetch('/lang/desktop/de.json')).json();window.ctx={t:k=>labels[k]||k,confirmDialog:async()=>true,api:async(path,opts)=>{calls++;const r=await fetch(path,opts);const j=await r.json();if(!r.ok){const err=new Error(j.error);err.body=j;throw err;}return j}};NewspaperApp.render(document.querySelector('#host'),'test',ctx)})();</script></html>`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	launch := launcher.New().Bin(bin).Headless(true).NoSandbox(true)
	browser := rod.New().ControlURL(launch.MustLaunch()).MustConnect()
	defer launch.Cleanup()
	defer browser.Close()
	page := browser.MustPage(srv.URL + "/newspaper-fixture").Timeout(60 * time.Second)
	defer page.Close()
	page.MustSetViewport(1440, 900, 1, false)
	page.MustElement(".np-teaser")
	dir := filepath.Join("..", "reports", "newspaper")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"light", "dark"} {
		page.MustEval(`mode=>document.body.dataset.fruityMode=mode`, mode)
		for _, width := range []int{1440, 420} {
			page.MustSetViewport(width, 900, 1, false)
			page.MustScreenshot(filepath.Join(dir, fmt.Sprintf("front-%s-%d.png", mode, width)))
			if page.MustEval(`()=>document.querySelector('.np-app').scrollWidth>document.querySelector('#host').clientWidth+1`).Bool() {
				t.Fatalf("front page overflow at %d %s", width, mode)
			}
		}
	}
	page.MustSetViewport(1440, 900, 1, false)
	page.MustElement(".np-revision summary").MustClick()
	if !page.MustEval(`()=>document.querySelector('.np-revision').open`).Bool() {
		t.Fatal("revision correction editor did not open")
	}
	page.MustElement("[name=correction_note]").MustInput("Falsches Eröffnungsdatum")
	page.MustElement(`[data-action=revision-submit]`).MustClick()
	page.MustElement(".np-banner")
	mu.Lock()
	correctionSent := correctionRequest == "Falsches Eröffnungsdatum"
	mu.Unlock()
	if !correctionSent {
		t.Fatal("correction note was not sent with the new revision")
	}
	page.MustElement(".np-teaser").MustClick()
	page.MustElement(".np-source summary").MustClick()
	if page.MustEval(`()=>!!window.injected`).Bool() {
		t.Fatal("untrusted article content executed")
	}
	if !page.MustEval(`()=>document.querySelector('.np-source').open`).Bool() {
		t.Fatal("source did not expand")
	}
	page.MustElement(`[data-view=archive]`).MustClick()
	page.MustElement(".np-archive-list button")
	page.MustElement(`[data-view=preferences]`).MustClick()
	page.MustEval(`()=>{document.querySelector('[name=name]').value='Der Neue Morgen';document.querySelector('[name=rss_url]').value='https://example.org/culture.xml'}`)
	page.MustElement(`[name=interests_input]`).MustInput("Lokales Theater")
	page.MustElement(`[data-add=interests]`).MustClick()
	if !strings.Contains(page.MustElement(".np-chips").MustText(), "Lokales Theater") {
		t.Fatal("interest chip was not added")
	}
	if !page.MustEval(`()=>document.querySelector('[name=name]').value==='Der Neue Morgen'&&document.querySelector('[name=rss_url]').value==='https://example.org/culture.xml'`).Bool() {
		t.Fatal("unsaved preferences were lost when adding an interest")
	}
	page.MustEval(`()=>{const input=document.querySelector('[name=rss_url]');input.value='file:///private.xml';input.dispatchEvent(new Event('input',{bubbles:true}))}`)
	page.MustElement(`[data-action=add-rss]`).MustClick()
	if page.MustElement(".np-rss-error").MustText() == "" || page.MustEval(`()=>document.querySelectorAll('.np-rss .np-chips>span').length`).Int() != 0 {
		t.Fatal("invalid RSS address was accepted without feedback")
	}
	page.MustEval(`()=>{const input=document.querySelector('[name=rss_url]');input.value='https://example.org/culture.xml';input.dispatchEvent(new Event('input',{bubbles:true}))}`)
	page.MustEval(`()=>{document.querySelector('[name=rss_section]').value='culture'}`)
	page.MustElement(`[data-action=add-rss]`).MustClick()
	if !strings.Contains(page.MustElement(".np-rss .np-chips").MustText(), "culture.xml") {
		t.Fatal("RSS feed was not added")
	}
	page.MustElement(".np-prefs [type=submit]").MustClick()
	page.MustElement(".np-banner")
	mu.Lock()
	saved := p.Name == "Der Neue Morgen" && len(p.Interests) == 1 && p.Interests[0] == "Lokales Theater" && len(p.RSSFeeds) == 1 && p.RSSFeeds[0].Section == "culture"
	gotInterests := append([]string(nil), p.Interests...)
	mu.Unlock()
	if !saved {
		t.Fatalf("interests were not saved: %#v, banner=%q", gotInterests, page.MustElement(".np-banner").MustText())
	}
	page.MustElementR(".np-banner", "Einstellungen gespeichert.")
	page.MustElement(`[data-action=email-challenge]`).MustClick()
	page.MustElement(".np-banner.np-error")
	if text := page.MustElement(".np-banner.np-error").MustText(); !strings.Contains(text, "AgentMail hat die Bestätigungsmail abgelehnt (HTTP 403)") || strings.Contains(text, "raw server error") {
		t.Fatalf("challenge rejection was not localized safely: %q", text)
	}
	mu.Lock()
	challengeCode = "agentmail_bounce_blocked"
	mu.Unlock()
	page.MustElement(`[data-action=email-challenge]`).MustClick()
	page.MustElementR(".np-banner.np-error", "nach einem Bounce")
	if text := page.MustElement(".np-banner.np-error").MustText(); strings.Contains(text, "raw server error") {
		t.Fatalf("bounce block leaked provider text: %q", text)
	}
	for _, mode := range []string{"light", "dark"} {
		page.MustEval(`mode=>document.body.dataset.fruityMode=mode`, mode)
		for _, width := range []int{1440, 420} {
			page.MustSetViewport(width, 900, 1, false)
			page.MustScreenshot(filepath.Join(dir, fmt.Sprintf("newspaper-%s-%d.png", mode, width)))
			if page.MustEval(`()=>document.querySelector('.np-app').scrollWidth>document.querySelector('#host').clientWidth+1`).Bool() {
				t.Fatalf("horizontal overflow at %d %s", width, mode)
			}
		}
	}
	mu.Lock()
	localDate = "2026-09-26"
	mu.Unlock()
	page.MustElement(`[data-view=today]`).MustClick()
	page.MustElement(".np-new-day [data-action=create]")
	if page.MustEval(`()=>!!document.querySelector('.np-revision')`).Bool() {
		t.Fatal("an old edition offered a correction revision for the new day")
	}
	page.MustEval(`()=>NewspaperApp.dispose('test')`)
	before := page.MustEval(`()=>calls`).Int()
	time.Sleep(5500 * time.Millisecond)
	if page.MustEval(`()=>calls`).Int() != before {
		t.Fatal("polling leaked after disposal")
	}
	if errs := page.MustEval(`()=>errors`).Arr(); len(errs) != 0 {
		t.Fatal(errs)
	}
}
