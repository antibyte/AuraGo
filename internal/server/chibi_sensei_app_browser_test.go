package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"aurago/internal/chibisensei"
	"aurago/internal/chibisensei/chatrt/chatrttest"
	"aurago/internal/desktop"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
)

// The Chibi Sensei app against the REAL handlers: the real desktop shell and the real app modules run in the installed Chrome,
// handleChibiSensei is mounted on /api/desktop/chibi-sensei/, and the test creates a project, adds one example, runs the dataset
// checks and cross-checks the result through the API. Gated by AURAGO_RUN_BROWSER_SMOKE=1:
//
//	AURAGO_RUN_BROWSER_SMOKE=1 go test ./internal/server -run TestChibiSenseiAppRealServerBrowser -count=1 -v -timeout 10m

func csnbRead(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "ui", filepath.FromSlash(name)))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func csnbJS(s string) string {
	data, _ := json.Marshal(s)
	return string(data)
}

// csnbWords merges the desktop words and the app words of one language into one map.
func csnbWords(t *testing.T, lang string) map[string]string {
	t.Helper()
	words := map[string]string{}
	for _, dir := range []string{"desktop", "chibi_sensei"} {
		part := map[string]string{}
		if err := json.Unmarshal([]byte(csnbRead(t, "lang/"+dir+"/"+lang+".json")), &part); err != nil {
			t.Fatal(err)
		}
		for key, value := range part {
			words[key] = value
		}
	}
	return words
}

// csnbPage serves the desktop shell, the ui tree and the real Chibi Sensei handlers, opens the page in the installed Chrome and
// returns it with the origin of the test server.
func csnbPage(t *testing.T, s *Server) (*rod.Page, string) {
	t.Helper()
	html := regexp.MustCompile(`(?s)<script\b[^>]*>.*?</script>`).ReplaceAllString(csnbRead(t, "desktop.html"), "")
	html = regexp.MustCompile(`\{\{[^}]*\}\}`).ReplaceAllString(html, "")
	html = strings.Replace(html, "</body>", `<script>window.fixtureWords=`+csnbRead(t, "lang/desktop/en.json")+`;window.t=(k,v)=>{let s=fixtureWords[k]||k;for(const [key,value] of Object.entries(v||{}))s=s.replaceAll('{{'+key+'}}',value).replaceAll('{'+key+'}',value);return s;};window.fixtureErrors=[];window.failedChibi=[];addEventListener('error',e=>fixtureErrors.push(e.message));addEventListener('unhandledrejection',e=>fixtureErrors.push(String(e.reason)));window.WebSocket=class extends EventTarget{close(){}};</script><script src="/js/shared/lazy-assets.js"></script><script src="/js/desktop/core/module-loader.js"></script><script src="/audit-shell.js"></script></body>`, 1)
	shell := csnbRead(t, "js/desktop/bundles/main.bundle.js")
	cut := strings.LastIndex(shell, "    ensureDesktopRadialMenuAnchor();")
	if cut < 0 {
		t.Fatal("startup seam missing in main.bundle.js")
	}
	shell = shell[:cut] + `window.auditTest={state,openApp,closeWindow};` + shell[cut:]
	settings := desktop.DesktopSettingDefaults()
	for key, value := range map[string]string{"windows.restore_session": "false", "windows.animations": "false", "pet.enabled": "false", "phone_gadget.enabled": "false", "desktop.show_widgets": "false"} {
		settings[key] = value
	}

	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.Dir(filepath.Join("..", "..", "ui"))))
	mux.HandleFunc("/fixture", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, html)
	})
	mux.HandleFunc("/audit-shell.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/javascript")
		fmt.Fprint(w, shell)
	})
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api/desktop/bootstrap" {
			_ = json.NewEncoder(w).Encode(map[string]any{"enabled": true, "builtin_apps": desktop.BuiltinApps(), "installed_apps": []any{}, "widgets": []any{}, "shortcuts": []any{}, "desktop_files": []any{}, "workspace": map[string]any{"readonly": false}, "settings": settings})
		} else {
			fmt.Fprint(w, `{"status":"ok","files":[],"pets":[],"settings":{},"enabled":false}`)
		}
	})
	mux.Handle(chibiSenseiAPIBase+"/", handleChibiSensei(s))
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	chrome, ok := launcher.LookPath()
	if !ok {
		t.Skip("no Chrome or Chromium is installed")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 9*time.Minute)
	t.Cleanup(cancel)
	l := launcher.New().Context(ctx).Bin(chrome).Headless(true).NoSandbox(true).Set("disable-gpu").Set("disable-dev-shm-usage")
	control, err := l.Launch()
	if err != nil {
		t.Skipf("the browser did not start: %v", err)
	}
	t.Cleanup(l.Cleanup)
	browser := rod.New().Context(ctx).ControlURL(control)
	if err := browser.Connect(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = browser.Close() })
	page := browser.MustPage().Timeout(8 * time.Minute)
	t.Cleanup(func() { _ = page.Close() })
	page.MustSetViewport(1280, 900, 1, false)
	page.MustNavigate(srv.URL + "/fixture").MustWaitLoad()
	page.MustWait(`()=>!!window.auditTest?.state.ws`)
	page.MustEval(`(words)=>Object.assign(fixtureWords, words)`, csnbWords(t, "en"))
	// Record every failed request to the app's API, whether the network failed or the server answered with an error.
	page.MustEval(`()=>{const real=window.fetch.bind(window);window.fetch=async(...args)=>{const url=String(args[0]&&args[0].url||args[0]);const mine=url.includes('/api/desktop/chibi-sensei/');try{const res=await real(...args);if(mine&&!res.ok)failedChibi.push(res.status+' '+url);return res;}catch(e){if(mine)failedChibi.push('network '+url+' '+e);throw e;}};}`)
	return page, srv.URL
}

// csnbDump describes the visible state of the app for a failure message.
func csnbDump(page *rod.Page) string {
	var state string
	_ = rod.Try(func() {
		state = page.Timeout(10 * time.Second).MustEval(`()=>{const root=document.querySelector('[data-csn="app"]');const text=(root?root.innerText:'(no [data-csn="app"] root)').replace(/\s+/g,' ').slice(0,2000);return JSON.stringify({errors:window.fixtureErrors,failedChibi:window.failedChibi,app:text});}`).String()
	})
	return state
}

func csnbWait(t *testing.T, page *rod.Page, step, expr string) {
	t.Helper()
	if err := rod.Try(func() { page.Timeout(90 * time.Second).MustWait(expr) }); err != nil {
		t.Fatalf("step %q did not happen: %v\nwaited for: %s\nstate: %s", step, err, expr, csnbDump(page))
	}
}

// csnbClick waits for a selector to exist and be enabled, then clicks it.
func csnbClick(t *testing.T, page *rod.Page, step, selector string) {
	t.Helper()
	csnbWait(t, page, step, `()=>{const e=document.querySelector(`+csnbJS(selector)+`);return !!e&&!e.disabled;}`)
	page.MustEval(`(s)=>document.querySelector(s).click()`, selector)
}

// csnbType waits for a text field and types into it through real key events.
func csnbType(t *testing.T, page *rod.Page, step, selector, text string) {
	t.Helper()
	csnbWait(t, page, step, `()=>!!document.querySelector(`+csnbJS(selector)+`)`)
	if err := rod.Try(func() { page.Timeout(30 * time.Second).MustElement(selector).MustInput(text) }); err != nil {
		t.Fatalf("step %q: typing into %s failed: %v\nstate: %s", step, selector, err, csnbDump(page))
	}
}

func TestChibiSenseiAppRealServerBrowser(t *testing.T) {
	if os.Getenv("AURAGO_RUN_BROWSER_SMOKE") != "1" {
		t.Skip("set AURAGO_RUN_BROWSER_SMOKE=1 to run the Chibi Sensei real-server browser smoke")
	}
	s, _, _ := chibiAPIFixture(t)
	fake := chatrttest.New(true) // the chat runtime of the test-and-compare path: no process, no network
	fake.Replies["tuned:"], fake.Replies["base:"] = "Tuned answer.", "Base answer."
	s.chibiSenseiLauncherOverride = fake
	s.Cfg.Auth.Enabled = false
	page, origin := csnbPage(t, s)

	// 1. Open the app and wait for the start view.
	page.MustEval(`()=>auditTest.openApp('chibi-sensei')`)
	csnbWait(t, page, "the project list opens", `()=>!!document.querySelector('[data-csn="new-project"]')`)

	// 2. Create a project: recipe assistant, a name.
	csnbClick(t, page, "the new-project button opens the dialog", `[data-csn="new-project"]`)
	csnbWait(t, page, "the create dialog opens", `()=>document.querySelector('[data-csn="create-dialog"]')?.open===true`)
	csnbType(t, page, "the project name is typed", `[data-csn="create-name"]`, "Smoke project")
	page.MustEval(`()=>document.querySelector('[data-csn="create-recipe"][value="assistant"]').click()`)
	csnbWait(t, page, "the assistant recipe is selected", `()=>document.querySelector('[data-csn="create-recipe"][value="assistant"]')?.checked===true`)
	csnbClick(t, page, "the create button submits the dialog", `[data-csn="create-submit"]`)
	csnbWait(t, page, "the project opens", `()=>document.querySelector('[data-csn="project-name"]')?.textContent==='Smoke project'`)

	// 3. Goal step: the goal text and the first model profile, then the primary action leads to the Data step.
	csnbType(t, page, "the goal text is typed", `[data-csn="goal-text"]`, "Answer questions about my smoke test politely.")
	csnbClick(t, page, "the first model profile is chosen", `[data-csn="profile"]`)
	csnbWait(t, page, "the primary action is enabled once the goal is complete", `()=>{const b=document.querySelector('[data-csn="primary"]');return !!b&&!b.disabled;}`)
	csnbClick(t, page, "the primary action saves the goal", `[data-csn="primary"]`)
	csnbWait(t, page, "the Data step becomes the current step", `()=>!!document.querySelector('[data-csn="step"][data-step="data"][aria-current="step"]')`)

	// 4. Data step: one example through the editor, then the table shows one row.
	csnbClick(t, page, "the new-example button opens the editor", `[data-csn="new-example"]`)
	csnbType(t, page, "the user message is typed", `[data-csn="msg-content"][data-index="0"]`, "What is a smoke test?")
	csnbType(t, page, "the assistant answer is typed", `[data-csn="msg-content"][data-index="1"]`, "A quick check that the most important things work.")
	csnbClick(t, page, "the editor save button is clicked", `[data-csn="editor-save"]`)
	csnbWait(t, page, "the table shows exactly one row", `()=>document.querySelectorAll('[data-csn="row"]').length===1`)
	csnbWait(t, page, "the footer counts one example", `()=>(document.querySelector('[data-csn="rows-total"]')?.textContent||'').includes('1')`)

	// 5. Run the dataset checks through the primary action. One row is below the minimum count, so a blocking finding shows.
	csnbClick(t, page, "the primary action runs the checks", `[data-csn="primary"]`)
	csnbWait(t, page, "the checks list shows at least one finding", `()=>document.querySelectorAll('[data-csn="check"]').length>=1`)
	csnbWait(t, page, "the minimum-count check reports a problem", `()=>!!document.querySelector('[data-csn="check"][data-id="count_min"][data-level="block"]')`)

	// 6. Training step: the preflight answers with Kaggle switched off; nothing is started.
	csnbClick(t, page, "the training step opens", `[data-csn="step"][data-step="training"]`)
	csnbWait(t, page, "the preflight has loaded", `()=>!!document.querySelector('[data-csn="preflight"][aria-busy="false"]')`)
	csnbWait(t, page, "the preflight summary says blocked", `()=>!!document.querySelector('[data-csn="preflight-summary"][data-state="blocked"]')`)
	csnbWait(t, page, "Kaggle is reported as switched off", `()=>!!document.querySelector('[data-csn="preflight-item"][data-id="kaggle_enabled"][data-ok="0"][data-level="block"]')`)
	csnbWait(t, page, "the one-row dataset is reported as not ready", `()=>!!document.querySelector('[data-csn="preflight-item"][data-id="dataset_ready"][data-ok="0"]')`)
	csnbWait(t, page, "the primary action is disabled with a hint", `()=>{const b=document.querySelector('[data-csn="primary"]');const h=document.querySelector('[data-csn="primary-hint"]');return !!b&&b.disabled&&!!h&&h.textContent.trim()!=='';}`)
	csnbWait(t, page, "the enable button or the ask text is shown", `()=>!!document.querySelector('[data-csn="preflight-enable"],[data-csn="preflight-ask"]')`)
	csnbWait(t, page, "three preset cards are shown", `()=>document.querySelectorAll('[data-csn="preset-card"]').length===3`)

	// 7. No script error and no failed request to the app's API during the whole run.
	if got := page.MustEval(`()=>JSON.stringify(window.fixtureErrors)`).String(); got != "[]" {
		t.Fatalf("script errors in the page: %s\nstate: %s", got, csnbDump(page))
	}
	if got := page.MustEval(`()=>JSON.stringify(window.failedChibi)`).String(); got != "[]" {
		t.Fatalf("failed requests to /api/desktop/chibi-sensei/: %s\nstate: %s", got, csnbDump(page))
	}

	// 8. Cross-check through the API: the project exists with one row.
	res, err := http.Get(origin + chibiSenseiAPIBase + "/projects")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("GET /projects = %d %s", res.StatusCode, body)
	}
	var list struct {
		Projects []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
			Rows int    `json:"rows"`
		} `json:"projects"`
	}
	if err := json.Unmarshal(body, &list); err != nil {
		t.Fatalf("GET /projects: %v\n%s", err, body)
	}
	if len(list.Projects) != 1 || list.Projects[0].Name != "Smoke project" || list.Projects[0].Rows != 1 {
		t.Fatalf("projects through the API = %+v, want one \"Smoke project\" with 1 row", list.Projects)
	}

	// 9. The smoke never started a run.
	res, err = http.Get(origin + chibiSenseiAPIBase + "/projects/" + list.Projects[0].ID + "/runs")
	if err != nil {
		t.Fatal(err)
	}
	body, _ = io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != http.StatusOK || !strings.Contains(string(body), `"runs":[]`) {
		t.Fatalf("GET /runs = %d %s, want no run", res.StatusCode, body)
	}

	// 10. Test and compare: seed a project with a succeeded run (both model files, three examples and a report).
	rt, release, err := s.acquireChibiSensei()
	if err != nil {
		t.Fatal(err)
	}
	project2, err := rt.store.CreateProject(context.Background(), chibisensei.ProjectInput{Name: "Test project", Recipe: chibisensei.RecipeAssistant,
		GoalText: "Answer politely.", ProfileID: "qwen35-0p8b"})
	if err != nil {
		release()
		t.Fatal(err)
	}
	runID := chibiSucceededTestRun(t, rt, project2.ID, "qwen35-0p8b", true, 3)
	release()

	// 11. Back to the project list and into the seeded project.
	csnbClick(t, page, "the back button leads to the project list", `[data-csn="back"]`)
	card := `[data-csn="project-card"][data-id="` + project2.ID + `"]`
	csnbWait(t, page, "the seeded project is listed", `()=>!!document.querySelector(`+csnbJS(card)+`)`)
	csnbClick(t, page, "the seeded project opens", card)
	csnbWait(t, page, "the seeded project is open", `()=>document.querySelector('[data-csn="project-name"]')?.textContent==='Test project'`)

	// 12. The Test step is ready; open it.
	testStep := `[data-csn="step"][data-step="test"][data-state="ready"]`
	csnbWait(t, page, "the Test step is ready", `()=>!!document.querySelector(`+csnbJS(testStep)+`)`)
	csnbClick(t, page, "the Test step opens", testStep)
	csnbWait(t, page, "the Test view shows", `()=>!!document.querySelector('[data-csn="test"]')`)

	// 13. The report card.
	csnbClick(t, page, "the report view opens", `[data-csn="test-view"][data-view="report"]`)
	csnbWait(t, page, "the report card shows", `()=>!!document.querySelector('[data-csn="report-card"]')`)

	// 14. The examples: exactly three.
	csnbClick(t, page, "the examples view opens", `[data-csn="test-view"][data-view="samples"]`)
	csnbWait(t, page, "three examples show", `()=>document.querySelectorAll('[data-csn="sample"]').length===3`)

	// 15. The chat view: the runtime is installed (the fake), so the strip says ready and the comparison can be switched on.
	csnbClick(t, page, "the chat view opens", `[data-csn="test-view"][data-view="chat"]`)
	csnbWait(t, page, "the runtime strip says ready", `()=>!!document.querySelector('[data-csn="runtime-strip"][data-state="ready"]')`)
	csnbWait(t, page, "the compare switch is enabled", `()=>{const e=document.querySelector('[data-csn="chat-compare"]');return !!e&&!e.disabled;}`)

	// 16. Compare with the base model: two columns.
	csnbClick(t, page, "the compare switch is turned on", `[data-csn="chat-compare"]`)
	csnbWait(t, page, "the tuned column shows", `()=>!!document.querySelector('[data-csn="chat-column"][data-target="tuned"]')`)
	csnbWait(t, page, "the base column shows", `()=>!!document.querySelector('[data-csn="chat-column"][data-target="base"]')`)

	// 17. One message to both models.
	csnbType(t, page, "the chat message is typed", `[data-csn="chat-input"]`, "Hello there")
	csnbClick(t, page, "the send button sends the message", `[data-csn="chat-send"]`)

	// 18. Both answers arrive.
	csnbWait(t, page, "the tuned answer is done", `()=>(document.querySelector('[data-csn="chat-answer"][data-target="tuned"][data-state="done"]')?.textContent||'').includes('Tuned answer.')`)
	csnbWait(t, page, "the base answer is done", `()=>(document.querySelector('[data-csn="chat-answer"][data-target="base"][data-state="done"]')?.textContent||'').includes('Base answer.')`)

	// 19. Still no script error and no failed request to the app's API.
	if got := page.MustEval(`()=>JSON.stringify(window.fixtureErrors)`).String(); got != "[]" {
		t.Fatalf("script errors in the page after the chat: %s\nstate: %s", got, csnbDump(page))
	}
	if got := page.MustEval(`()=>JSON.stringify(window.failedChibi)`).String(); got != "[]" {
		t.Fatalf("failed requests to /api/desktop/chibi-sensei/ after the chat: %s\nstate: %s", got, csnbDump(page))
	}

	// 20. Cross-check through the API: the report was marked as seen, the Test step is done, two models were launched.
	if got := csnbGet(t, origin+chibiSenseiAPIBase+"/runs/"+runID); !strings.Contains(got, `"report_seen":true`) {
		t.Fatalf("GET /runs/%s = %s, want report_seen true", runID, got)
	}
	if got := csnbGet(t, origin+chibiSenseiAPIBase+"/projects/"+project2.ID); !strings.Contains(got, `"test":"done"`) {
		t.Fatalf("GET /projects/%s = %s, want steps.test done", project2.ID, got)
	}
	if n := len(fake.Launches()); n != 2 {
		t.Fatalf("%d models launched, want 2", n)
	}
}

// csnbGet reads one API answer as text and requires status 200.
func csnbGet(t *testing.T, url string) string {
	t.Helper()
	res, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("GET %s = %d %s", url, res.StatusCode, body)
	}
	return string(body)
}
