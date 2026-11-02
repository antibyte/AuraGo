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

	"aurago/internal/desktop"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
)

// The Storymaker app tour against the REAL handlers. ui/desktop_storymaker_browser_test.go runs the same app against a stand-in
// for the API (package ui cannot reach the real handlers: they need a configured Server, which only this package's tests can
// build). This tour opens the real desktop shell and the real app modules in the installed Chrome, mounts handleStorymaker on
// /api/desktop/storymaker/, and ends in an export that renders for real (FFmpeg and Chrome) and lands as an MP4 in the project
// folder. Gated by AURAGO_RUN_BROWSER_SMOKE=1:
//
//	AURAGO_RUN_BROWSER_SMOKE=1 go test ./internal/server -run TestStorymakerAppRealServerBrowser -count=1 -timeout 15m

func smappWords(t *testing.T, lang string) map[string]string {
	t.Helper()
	words := map[string]string{}
	for _, dir := range []string{"storymaker", "storymaker_catalog"} {
		data, err := os.ReadFile(filepath.Join("..", "..", "ui", "lang", dir, lang+".json"))
		if err != nil {
			t.Fatal(err)
		}
		part := map[string]string{}
		if err := json.Unmarshal(data, &part); err != nil {
			t.Fatal(err)
		}
		for key, value := range part {
			words[key] = value
		}
	}
	return words
}

func smappRead(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "ui", filepath.FromSlash(name)))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func smappJS(s string) string {
	data, _ := json.Marshal(s)
	return string(data)
}

// smappPage serves the desktop shell, the ui tree and the real Storymaker handlers, opens the page in the installed Chrome and
// opens the Storymaker app in it.
func smappPage(t *testing.T, s *Server) (*rod.Page, string) {
	t.Helper()
	return smappPageWith(t, s, nil)
}

// smappPageWith is smappPage with a step that runs against the server's origin before the app opens.
func smappPageWith(t *testing.T, s *Server, prepare func(origin string)) (*rod.Page, string) {
	t.Helper()
	html := regexp.MustCompile(`(?s)<script\b[^>]*>.*?</script>`).ReplaceAllString(smappRead(t, "desktop.html"), "")
	html = regexp.MustCompile(`\{\{[^}]*\}\}`).ReplaceAllString(html, "")
	html = strings.Replace(html, "</body>", `<script>window.fixtureWords=`+smappRead(t, "lang/desktop/de.json")+`;window.t=(k,v)=>{let s=fixtureWords[k]||k;for(const [key,value] of Object.entries(v||{}))s=s.replaceAll('{{'+key+'}}',value).replaceAll('{'+key+'}',value);return s;};window.fixtureErrors=[];addEventListener('error',e=>fixtureErrors.push(e.message));addEventListener('unhandledrejection',e=>fixtureErrors.push(String(e.reason)));window.WebSocket=class extends EventTarget{close(){}};</script><script src="/js/shared/lazy-assets.js"></script><script src="/js/desktop/core/module-loader.js"></script><script src="/audit-shell.js"></script></body>`, 1)
	shell := smappRead(t, "js/desktop/bundles/main.bundle.js")
	cut := strings.LastIndex(shell, "    ensureDesktopRadialMenuAnchor();")
	if cut < 0 {
		t.Fatal("startup seam missing")
	}
	shell = shell[:cut] + `window.auditTest={state,openApp,closeWindow};` + shell[cut:]
	settings := desktop.DesktopSettingDefaults()
	for key, value := range map[string]string{"windows.restore_session": "false", "windows.animations": "false", "pet.enabled": "false", "phone_gadget.enabled": "false", "desktop.show_widgets": "false"} {
		settings[key] = value
	}

	ui := http.FileServer(http.Dir(filepath.Join("..", "..", "ui")))
	mux := http.NewServeMux()
	mux.Handle("/", ui)
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
	mux.Handle(storymakerAPIBase+"/", handleStorymaker(s))
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	chrome, ok := launcher.LookPath()
	if !ok {
		t.Skip("no Chrome or Chromium is installed")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 14*time.Minute)
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
	page := browser.MustPage().Timeout(10 * time.Minute)
	t.Cleanup(func() { _ = page.Close() })
	page.MustSetViewport(1280, 900, 1, false)
	page.MustNavigate(srv.URL + "/fixture").MustWaitLoad()
	page.MustWait(`()=>!!window.auditTest?.state.ws`)
	page.MustEval(`(words)=>Object.assign(fixtureWords, words)`, smappWords(t, "en"))
	if prepare != nil {
		prepare(srv.URL)
	}
	page.MustEval(`()=>auditTest.openApp('storymaker')`)
	return page, srv.URL
}

// smappWait waits for a condition in the page; one wait gets 90 seconds (the export waits longer, see smappWaitFor).
func smappWait(t *testing.T, page *rod.Page, expr string) {
	t.Helper()
	smappWaitFor(t, page, expr, 90*time.Second)
}

func smappWaitFor(t *testing.T, page *rod.Page, expr string, limit time.Duration) {
	t.Helper()
	if err := rod.Try(func() { page.Timeout(limit).MustWait(expr) }); err != nil {
		var state string
		_ = rod.Try(func() {
			state = page.Timeout(10 * time.Second).MustEval(`()=>JSON.stringify({errors: window.fixtureErrors, app: (document.querySelector('[data-stm="app"]')||{}).innerText||''}).slice(0, 1500)`).String()
		})
		t.Fatalf("timed out waiting for %s: %v\n%s", expr, err, state)
	}
}

func smappText(page *rod.Page, selector string) string {
	return page.MustEval(`(s)=>{const e=document.querySelector(s);return e?e.textContent:'';}`, selector).String()
}

func smappClick(t *testing.T, page *rod.Page, selector string) {
	t.Helper()
	smappWait(t, page, `()=>!!document.querySelector(`+smappJS(selector)+`)`)
	page.MustEval(`(s)=>document.querySelector(s).click()`, selector)
}

func smappNoErrors(t *testing.T, page *rod.Page) {
	t.Helper()
	if got := page.MustEval(`()=>JSON.stringify(window.fixtureErrors)`).String(); got != "[]" {
		t.Fatalf("script errors in the page: %s", got)
	}
}

// smappEditorReplace swaps text in the open CodeMirror editor through the public EditorView API of the vendored bundle.
func smappEditorReplace(page *rod.Page, find, insert string) {
	page.MustEval(`async (find, insert) => {
		const cm = await import('/js/vendor/codemirror-bundle.esm.js');
		const view = cm.EditorView.findFromDOM(document.querySelector('[data-stm="code-editor"] .cm-editor'));
		const text = view.state.doc.toString(), at = text.indexOf(find);
		if (at < 0) throw new Error('not in the editor: ' + find);
		view.dispatch({changes: {from: at, to: at + find.length, insert}});
	}`, find, insert)
}

// smappGet reads a JSON document from the real API.
func smappGet(t *testing.T, origin, path string, into any) {
	t.Helper()
	res, err := http.Get(origin + storymakerAPIBase + path)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("GET %s = %d %s", path, res.StatusCode, body)
	}
	if err := json.Unmarshal(body, into); err != nil {
		t.Fatalf("GET %s: %v\n%s", path, err, body)
	}
}

type smappProject struct {
	Story map[string]any `json:"story"`
	ETag  string         `json:"etag"`
}

func smappStored(t *testing.T, origin, id string) map[string]any {
	t.Helper()
	var p smappProject
	smappGet(t, origin, "/projects/"+id, &p)
	return p.Story
}

func smappProjectIDs(t *testing.T, origin string) []string {
	t.Helper()
	var list struct {
		Projects []struct {
			ID string `json:"id"`
		} `json:"projects"`
	}
	smappGet(t, origin, "/projects", &list)
	ids := []string{}
	for _, p := range list.Projects {
		ids = append(ids, p.ID)
	}
	return ids
}

func TestStorymakerAppRealServerBrowser(t *testing.T) {
	s := storymakerRealServer(t)
	page, origin := smappPage(t, s)
	started := time.Now()

	// Start screen: the first status call launches the browser once (a cached probe), so the app first shows a neutral
	// loading state; then the empty list, with no capability banner because FFmpeg and a browser are ready.
	smappWait(t, page, `()=>!!document.querySelector('.stm-projects') && !!document.querySelector('[data-stm="projects-empty"]') && !document.querySelector('[data-stm="projects-empty"]').hidden`)
	t.Logf("start screen after %v", time.Since(started).Round(100*time.Millisecond))
	if page.MustEval(`()=>!!document.querySelector('[data-banner="export_unavailable"]')`).Bool() {
		t.Fatal("the export-unavailable banner shows although FFmpeg and a browser are ready")
	}

	// Create a project: the editor opens on one scene, the real player page compiles it and the strip shows a drawn thumbnail.
	page.MustElement(`[data-stm="new-title"]`).MustInput("Real server tour")
	page.MustElement(`[data-stm="aspect"][data-aspect="16:9"]`).MustClick()
	page.MustElement(`[data-stm="create-project"]`).MustClick()
	smappWait(t, page, `()=>document.querySelector('[data-stm="project-title"]')?.textContent==='Real server tour'`)
	smappWait(t, page, `()=>document.querySelectorAll('[data-stm="scene"]').length===1 && document.querySelector('[data-stm="clock-total"]').textContent!=='0:00.0'`)
	ids := smappProjectIDs(t, origin)
	if len(ids) != 1 {
		t.Fatalf("projects after create = %v", ids)
	}
	id := ids[0]
	if got := *page.MustElement(`.stm-player`).MustAttribute("sandbox"); got != "allow-scripts" {
		t.Fatalf("player sandbox = %q, want exactly allow-scripts", got)
	}
	smappWait(t, page, `()=>document.querySelector('canvas[data-scene="s1"]')?.dataset.drawn==='true'`)
	if !page.MustEval(`()=>{const c=document.querySelector('canvas[data-scene="s1"]'),d=c.getContext('2d').getImageData(0,0,c.width,c.height).data,seen=new Set();for(let i=0;i<d.length;i+=4*97)seen.add(d[i]+','+d[i+1]+','+d[i+2]);return seen.size>1;}`).Bool() {
		t.Fatal("the thumbnail of the first scene is a single colour: the preview rendered a blank frame")
	}
	smappNoErrors(t, page)

	// Play and pause through the real player page.
	page.MustEval(`()=>document.querySelector('[data-stm="play"]').click()`)
	smappWait(t, page, `()=>document.querySelector('[data-stm="play"]').dataset.playing==='true'`)
	smappWait(t, page, `()=>document.querySelector('[data-stm="clock"]').textContent!=='0:00.0'`)
	page.MustEval(`()=>document.querySelector('[data-stm="play"]').click()`)
	smappWait(t, page, `()=>document.querySelector('[data-stm="play"]').dataset.playing==='false'`)

	// Narration: typing saves with If-Match and the real server keeps the text.
	before := smappText(page, `[data-stm="clock-total"]`)
	page.MustElement(`[data-stm="narration"]`).MustInput("Anna works from home. Suddenly somebody knocks at the door.")
	smappWait(t, page, `()=>document.querySelector('[data-stm="save-state"]')?.dataset.state==='saved'`)
	scene := smappStored(t, origin, id)["scenes"].([]any)[0].(map[string]any)
	if narration, _ := scene["narration"].(string); !strings.Contains(narration, "knocks at the door") {
		t.Fatalf("stored narration = %q", narration)
	}
	smappWait(t, page, `()=>document.querySelector('[data-stm="clock-total"]').textContent!==`+smappJS(before))

	// Library: a person, a prop and an environment go into the selected scene.
	smappClick(t, page, `[data-stm="side-tab"][data-tab="library"]`)
	smappClick(t, page, `[data-stm="library-card"][data-id="char.office-woman-1"]`)
	smappWait(t, page, `()=>document.querySelectorAll('[data-stm="scene-items"] [data-kind="actor"]').length===1`)
	smappClick(t, page, `[data-stm="library-tab"][data-kind="prop"]`)
	smappClick(t, page, `[data-stm="library-card"][data-id="prop.laptop"]`)
	smappClick(t, page, `[data-stm="library-tab"][data-kind="environment"]`)
	smappClick(t, page, `[data-stm="library-card"][data-id="env.office"]`)
	smappWait(t, page, `()=>document.querySelector('[data-stm="library-card"][data-id="env.office"]')?.dataset.present==='true'`)
	smappWait(t, page, `()=>document.querySelector('[data-stm="save-state"]')?.dataset.state==='saved'`)
	scene = smappStored(t, origin, id)["scenes"].([]any)[0].(map[string]any)
	if scene["environment"] != "env.office" || len(scene["actors"].([]any)) != 1 || len(scene["props"].([]any)) != 1 {
		t.Fatalf("stored scene after the library clicks = %v", scene)
	}
	smappClick(t, page, `[data-stm="side-tab"][data-tab="scene"]`)

	// Code tab: an invalid value is saved, reported by the real validator with a JSON Pointer path and marked at its line.
	smappClick(t, page, `[data-stm="tab"][data-tab="code"]`)
	smappWait(t, page, `()=>!!document.querySelector('[data-stm="code-editor"] .cm-editor') && document.querySelector('[data-stm="code-editor"] .cm-content').textContent.includes('storymaker/1')`)
	smappEditorReplace(page, `"variant": "day"`, `"variant": "dusk"`)
	smappWait(t, page, `()=>document.querySelector('[data-stm="code-state"]')?.dataset.state==='dirty'`)
	smappClick(t, page, `[data-stm="code-save"]`)
	smappWait(t, page, `()=>document.querySelector('[data-stm="code-problems"]')?.dataset.count!=='0' && !!document.querySelector('[data-stm="code-editor"] .cm-lint-marker')`)
	if !strings.Contains(smappText(page, `[data-stm="code-problems"]`), "Invalid value") {
		t.Fatalf("the problem list does not name the issue: %q", smappText(page, `[data-stm="code-problems"]`))
	}
	smappNoErrors(t, page)
	smappEditorReplace(page, `"variant": "dusk"`, `"variant": "night"`)
	smappClick(t, page, `[data-stm="code-save"]`)
	smappWait(t, page, `()=>document.querySelector('[data-stm="code-problems"]')?.dataset.count==='0' && document.querySelector('[data-stm="code-state"]')?.dataset.state==='saved'`)
	if smappStored(t, origin, id)["scenes"].([]any)[0].(map[string]any)["variant"] != "night" {
		t.Fatalf("variant after the code save = %v", smappStored(t, origin, id)["scenes"].([]any)[0])
	}
	smappClick(t, page, `[data-stm="tab"][data-tab="stage"]`)

	// A code scene: the starter program is saved as a new file (If-None-Match), bundled by the real esbuild, a mistake is
	// shown at its line, and the fixed program makes the bundle available to the player.
	smappClick(t, page, `[data-stm="add-code-scene"]`)
	smappWait(t, page, `()=>document.querySelector('[data-stm="code-file"]')?.textContent==='code2.js' && document.querySelector('[data-stm="code-editor"] .cm-content')?.textContent.includes('export default function scene')`)
	smappEditorReplace(page, "api.duration(5);", "api.duration(5;")
	smappClick(t, page, `[data-stm="code-save"]`)
	smappWait(t, page, `()=>document.querySelector('[data-stm="code-problems"]')?.dataset.count!=='0' && document.querySelector('[data-stm="code-file-btn"][data-key="scene:code2"]')?.dataset.failed==='true'`)
	smappEditorReplace(page, "api.duration(5;", "api.duration(5);")
	smappClick(t, page, `[data-stm="code-save"]`)
	smappWait(t, page, `()=>document.querySelector('[data-stm="code-problems"]')?.dataset.count==='0' && document.querySelector('[data-stm="code-file-btn"][data-key="scene:code2"]')?.dataset.failed==='false'`)
	smappClick(t, page, `[data-stm="tab"][data-tab="stage"]`)
	smappWait(t, page, `()=>document.querySelectorAll('[data-stm="scene"]').length===2 && document.querySelector('[data-stm="scene"][data-scene="code2"] .stm-scene-kind')!==null`)

	// Inspector: transition and duration of the selected (code) scene are saved by the real validator.
	const setSelect = `(sel,value)=>{const s=document.querySelector(sel);s.value=value;s.dispatchEvent(new Event('change',{bubbles:true}));}`
	page.MustEval(setSelect, `[data-stm="transition"]`, "wipe")
	page.MustEval(setSelect, `[data-stm="duration-mode"]`, "fixed")
	page.MustEval(`()=>{const i=document.querySelector('[data-stm="duration-seconds"]');i.value='3.5';i.dispatchEvent(new Event('change',{bubbles:true}));}`)
	smappWait(t, page, `()=>document.querySelector('[data-stm="save-state"]')?.dataset.state==='saved'`)
	second := smappStored(t, origin, id)["scenes"].([]any)[1].(map[string]any)
	if second["transition"] != "wipe" || second["duration"] != 3.5 {
		t.Fatalf("stored transition and duration = %v, %v", second["transition"], second["duration"])
	}
	smappClick(t, page, `[data-stm="scene-up"]`)
	smappWait(t, page, `()=>document.querySelector('[data-stm="scene"][data-index="0"]')?.dataset.scene==='code2' && document.querySelector('[data-stm="save-state"]')?.dataset.state==='saved'`)
	smappClick(t, page, `[data-stm="scene-down"]`)
	smappWait(t, page, `()=>document.querySelector('[data-stm="scene"][data-index="1"]')?.dataset.scene==='code2' && document.querySelector('[data-stm="save-state"]')?.dataset.state==='saved'`)

	// Export: the real render runs (FFmpeg and Chrome), progress arrives on the project's event stream and the panel ends in
	// the output path of an MP4 that exists in the project folder.
	smappClick(t, page, `[data-stm="export"]`)
	smappWait(t, page, `()=>!!document.querySelector('[data-stm="export-start"]') && !document.querySelector('[data-stm="export-start"]').disabled`)
	if !page.MustEval(`()=>document.querySelectorAll('[data-stm="export-caps"] [data-ok="true"]').length===2`).Bool() {
		t.Fatal("FFmpeg and the browser should both be listed as ready")
	}
	smappClick(t, page, `[data-stm="export-start"]`)
	smappWait(t, page, `()=>!!document.querySelector('[data-stm="export-progress"]')`)
	renderStart := time.Now()
	smappWaitFor(t, page, `()=>!!document.querySelector('[data-stm="export-path"]') || !!document.querySelector('[data-stm="export-failed"]')`, 6*time.Minute)
	if page.MustEval(`()=>!!document.querySelector('[data-stm="export-failed"]')`).Bool() {
		t.Fatalf("the export failed: %s", smappText(page, `[data-stm="export-panel"]`))
	}
	t.Logf("export took %v", time.Since(renderStart).Round(100*time.Millisecond))
	path := smappText(page, `[data-stm="export-path"]`)
	if !strings.HasPrefix(path, "Documents/Storymaker/"+id+"/exports/Real-server-tour-") || !strings.HasSuffix(path, ".mp4") {
		t.Fatalf("export path = %q", path)
	}
	mp4 := filepath.Join(s.Cfg.VirtualDesktop.WorkspaceDir, filepath.FromSlash(path))
	doc := storymakerFFprobe(t, s.Cfg.VideoStudio.FFmpegPath, mp4)
	var video bool
	for _, st := range doc.Streams {
		video = video || st.CodecType == "video" && st.Width == 1920 && st.Height == 1080 && st.PixFmt == "yuv420p"
	}
	if !video {
		t.Fatalf("the MP4 is not 1920x1080 yuv420p: %+v", doc.Streams)
	}
	windows := page.MustEval(`()=>auditTest.state.windows.size`).Int()
	page.MustEval(`()=>document.querySelector('[data-stm="export-show"]').click()`)
	smappWait(t, page, fmt.Sprintf(`()=>auditTest.state.windows.size===%d`, windows+1))
	smappNoErrors(t, page)

	// Leave the project and open it again: the stored story comes back through the real GET.
	smappClick(t, page, `[data-stm="export-close"]`)
	smappClick(t, page, `[data-stm="back"]`)
	smappWait(t, page, `()=>document.querySelectorAll('[data-stm="open-project"]').length===1 && !document.querySelector('.stm-player')`)
	smappClick(t, page, `[data-stm="open-project"]`)
	smappWait(t, page, `()=>document.querySelector('[data-stm="narration"]')?.value.includes('knocks at the door') && document.querySelectorAll('[data-stm="scene"]').length===2`)

	// Delete asks the desktop dialog first: refusing keeps the project, confirming removes the folder.
	smappClick(t, page, `[data-stm="back"]`)
	smappClick(t, page, `[data-stm="delete-project"]`)
	smappWait(t, page, `()=>!!document.querySelector('.vd-modal [data-cancel]')`)
	page.MustEval(`()=>document.querySelector('.vd-modal [data-cancel]').click()`)
	smappWait(t, page, `()=>!document.querySelector('.vd-modal')`)
	if len(smappProjectIDs(t, origin)) != 1 {
		t.Fatal("cancelling the dialog must keep the project")
	}
	smappClick(t, page, `[data-stm="delete-project"]`)
	smappWait(t, page, `()=>!!document.querySelector('.vd-modal [type="submit"]')`)
	page.MustEval(`()=>document.querySelector('.vd-modal [type="submit"]').click()`)
	smappWait(t, page, `()=>document.querySelectorAll('[data-stm="open-project"]').length===0 && !document.querySelector('[data-stm="projects-empty"]').hidden`)
	if left := smappProjectIDs(t, origin); len(left) != 0 {
		t.Fatalf("projects after the delete = %v", left)
	}
	if _, err := os.Stat(filepath.Join(s.Cfg.VirtualDesktop.WorkspaceDir, "Documents", "Storymaker", id)); !os.IsNotExist(err) {
		t.Errorf("the project folder is still there after the delete: %v", err)
	}

	// Closing the window tears the app down without a script error.
	page.MustEval(`()=>auditTest.closeWindow([...auditTest.state.windows.values()].find(w=>w.appId==='storymaker').id)`)
	smappWait(t, page, `()=>!document.querySelector('[data-stm="app"]')`)
	smappNoErrors(t, page)
}
