package ui

import (
	"encoding/json"
	"fmt"
	"io/fs"
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

// TestDesktopCYDCreatorShellBrowser drives the CYD Creator shell (plan 4a) inside the real desktop
// shell against the in-memory backend of testdata/cyd-creator-fixture.js: start screen, new project,
// toolbar, Design/Code switch, code edit + autosave + conflict, the publication-review fence, build
// with streamed log and problems, the agent job with plan confirmation over the fake SSE stream, the
// locked state, revisions and the event stream's give-up after failed reconnects.
// Without plan 4b the Design mode shows its placeholder.
func TestDesktopCYDCreatorShellBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	s := startCYDCreatorSmoke(t)
	page := s.page

	// Start screen: describe / designer entry points, no Import without plan 5, the seeded project.
	s.wait(`()=>!!document.querySelector('.cc-home') && document.querySelectorAll('.cc-project-row').length===1`)
	s.wait(`()=>!!document.querySelector('[data-cc-action="new-agent"]') && !!document.querySelector('[data-cc-action="new-designer"]') && document.querySelector('[data-cc-action="import"]').hidden`)
	s.noRawKeys("home")
	s.shot("home")

	// New project in the designer: two boards with their badge, seven theme swatches, portrait.
	page.MustElement(`[data-cc-action="new-designer"]`).MustClick()
	s.wait(`()=>document.querySelectorAll('.cc-dialog [data-cc-board]').length===2 && document.querySelectorAll('.cc-dialog [data-cc-theme]').length===7`)
	// Board states and theme names are plan-4b strings: without them the shell shows its fallbacks.
	s.wait(`()=>{const tag=r=>document.querySelector('.cc-dialog [data-cc-board="'+r+'"]').closest('label').querySelector('.cc-tag');return tag('esp32-2432s028r').dataset.tone==='ok' && tag('esp32-2432s028r').textContent.trim()!=='' && tag('esp32-3248s035c').dataset.tone!=='ok'}`)
	s.wait(`()=>document.querySelector('.cc-dialog [data-cc-theme="dark_sun"]').closest('label').textContent.trim()===(I18N['cyd_creator.theme.dark_sun']||'Dark sun')`)
	s.noRawKeys("new project dialog")
	s.shot("new-project-dialog")
	page.MustElement(`.cc-dialog [data-cc-field="name"]`).MustInput("Thermometer")
	page.MustEval(`()=>{document.querySelector('.cc-dialog [data-cc-field="orientation"]').value='portrait';}`)
	page.MustElement(`.cc-dialog [data-cc-dialog-action="create"]`).MustClick()
	// Plan 4b replaces the placeholder with the designer canvas; accept either so the smoke passes before and after 4b.
	s.wait(`()=>document.querySelector('[data-cc-project-title]')?.textContent==='Thermometer' && !!document.querySelector('[data-cc-designer-placeholder], .cc-canvas')`)
	// The orientation is not part of POST /projects: the layout is saved once with If-Match 1.
	s.wait(`()=>cydFixture.state.layoutPuts.some(p=>p.version===2 && p.project.orientation==='portrait')`)
	// Plan 5 modules are absent: Flash, OTA and Export are hidden; Build is ready; Design is active.
	s.wait(`()=>{const q=a=>document.querySelector('[data-cc-action="'+a+'"]');return q('flash').hidden && q('ota').hidden && q('export').hidden && !q('build').disabled && q('mode-design').getAttribute('aria-pressed')==='true'}`)
	// The layout warning of the save answer reaches Problems through layout:issues.
	s.wait(`()=>document.querySelector('[data-cc-dock-tab="problems"] .cc-dock-badge')?.textContent==='1'`)
	s.noRawKeys("project")
	s.shot("project-design")
	s.failOnPageErrors("new project")

	// Code mode: your files and the generated group; edit app_main.cpp, autosave with If-Match.
	page.MustElement(`[data-cc-action="mode-code"]`).MustClick()
	s.wait(`()=>!!document.querySelector('[data-cc-file="src/app/app_main.cpp"]') && !!document.querySelector('.cc-tree-group[data-group="generated"]')`)
	page.MustElement(`[data-cc-file="src/app/app_main.cpp"]`).MustClick()
	s.wait(`()=>!!document.querySelector('.cc-code-editor .cm-editor') && !!document.querySelector('.cc-code-editor .cm-content')?.textContent.includes('app_setup')`)
	page.MustEval(`()=>{window.__ccSha=cydFixture.sha(cydFixture.state.files.get(cydFixture.inst().projectId).get('src/app/app_main.cpp').text);window.__ccPuts=cydFixture.state.filePuts.length;}`)
	page.MustElement(`.cc-code-editor .cm-content`).MustClick()
	page.MustInsertText("// Temperatur")
	s.wait(`()=>{const p=cydFixture.state.filePuts.at(-1);return cydFixture.state.filePuts.length===window.__ccPuts+1 && p.path==='src/app/app_main.cpp' && p.text.includes('// Temperatur') && p.ifMatch===window.__ccSha}`)
	s.wait(`()=>document.querySelector('[data-cc-code-state]')?.dataset.state==='saved'`)
	s.shot("code-saved")

	// Generated files are read-only and say where to change them.
	page.MustElement(`.cc-tree-group[data-group="generated"] summary`).MustClick()
	page.MustElement(`[data-cc-file="src/generated/ui.h"]`).MustClick()
	s.wait(`()=>{const h=document.querySelector('[data-cc-code-hint]');return !h.hidden && h.textContent===t('cyd_creator.code.generated_hint') && document.querySelector('.cc-code-editor .cm-content').getAttribute('contenteditable')==='false'}`)
	s.shot("code-generated")

	// A file changed elsewhere: the save conflicts, Overwrite saves with the server's sha.
	page.MustElement(`[data-cc-file="src/app/app_main.cpp"]`).MustClick()
	s.wait(`()=>document.querySelector('.cc-code-editor .cm-content').textContent.includes('// Temperatur')`)
	page.MustEval(`()=>cydFixture.touchFile(cydFixture.inst().projectId,'src/app/app_main.cpp','// changed elsewhere\n')`)
	page.MustElement(`.cc-code-editor .cm-content`).MustClick()
	page.MustInsertText(" mine")
	s.wait(`()=>!!document.querySelector('.cc-dialog [data-cc-dialog-action="overwrite"]')`)
	s.noRawKeys("conflict")
	s.shot("code-conflict")
	page.MustElement(`.cc-dialog [data-cc-dialog-action="overwrite"]`).MustClick()
	s.wait(`()=>{const p=cydFixture.state.filePuts.at(-1);return p.text.includes(' mine') && p.ifMatch===cydFixture.sha('// changed elsewhere\n') && document.querySelector('[data-cc-code-state]').dataset.state==='saved'}`)
	s.failOnPageErrors("code")

	// A project fenced after an interrupted publication refuses saves with CYD_BUSY publication_review:
	// the save fails (no offline retry), the chip names the reason and the window shows its banner
	// until the summary no longer reports publish_review; Save now then lands.
	page.MustEval(`()=>cydFixture.fence(cydFixture.inst().projectId,true)`)
	page.MustElement(`.cc-code-editor .cm-content`).MustClick()
	page.MustInsertText(" fenced")
	s.wait(`()=>{const c=document.querySelector('[data-cc-code-state]');return c.dataset.state==='failed' && c.title===t('cyd_creator.error.publication_review') && document.querySelector('[data-cc-save-state]').dataset.state==='failed'}`)
	s.wait(`()=>{const b=document.querySelector('[data-cc-publish-review]');return !b.hidden && b.textContent===t('cyd_creator.error.publication_review')}`)
	s.noRawKeys("publication review")
	s.shot("publication-review")
	page.MustEval(`()=>cydFixture.fence(cydFixture.inst().projectId,false)`)
	s.wait(`()=>document.querySelector('[data-cc-publish-review]').hidden`)
	page.MustElement(`[data-cc-code-action="save"]`).MustClick()
	s.wait(`()=>{const p=cydFixture.state.filePuts.at(-1);return p.text.includes(' fenced') && document.querySelector('[data-cc-code-state]').dataset.state==='saved'}`)
	s.failOnPageErrors("publication review")

	// Build: streamed log lines, failed status, the diagnostic in Problems opens the file at the line.
	page.MustElement(`[data-cc-action="build"]`).MustClick()
	s.wait(`()=>document.querySelectorAll('[data-cc-log] .cc-log-line').length>=4 && document.querySelector('[data-cc-build-status]')?.dataset.status==='failed'`)
	s.wait(`()=>document.querySelector('[data-cc-build-status]').textContent.includes(t('cyd_creator.build.status_failed'))`)
	s.noRawKeys("build")
	s.shot("build-failed")
	page.MustElement(`[data-cc-dock-tab="problems"]`).MustClick()
	s.wait(`()=>document.querySelectorAll('[data-cc-problems] .cc-problem[data-source="build"]').length===1 && document.querySelectorAll('[data-cc-problems] .cc-problem[data-source="layout"]').length===1`)
	page.MustElement(`[data-cc-problems] .cc-problem[data-source="build"]`).MustClick()
	s.wait(`()=>cydFixture.inst().code.openPath()==='src/app/app_main.cpp' && document.querySelectorAll('.cc-code-marker').length===1`)
	s.shot("problem-marker")
	s.failOnPageErrors("build")

	// Agent: a new app from a description plans first; the plan waits for a confirmation.
	page.MustElement(`[data-cc-action="home"]`).MustClick()
	s.wait(`()=>document.querySelectorAll('.cc-project-row').length===2`)
	page.MustElement(`[data-cc-action="new-agent"]`).MustClick()
	page.MustElement(`.cc-dialog [data-cc-field="name"]`).MustInput("Wetteruhr")
	page.MustElement(`.cc-dialog [data-cc-field="description"]`).MustInput("Zeige die Temperatur groß und den Verlauf der letzten Stunden.")
	page.MustElement(`.cc-dialog [data-cc-dialog-action="create"]`).MustClick()
	s.wait(`()=>document.querySelector('[data-cc-panel="agent"]')?.getAttribute('aria-selected')==='true' && !!document.querySelector('[data-cc-plan]')`)
	s.wait(`()=>{const b=cydFixture.state.jobBodies.at(-1);return b.mode==='new' && b.prompt.startsWith('Zeige')}`)
	s.wait(`()=>{const p=document.querySelector('[data-cc-plan]')?.textContent||'';return p.includes('verlauf') && p.includes('ArduinoJson')}`)
	s.noRawKeys("plan")
	s.shot("agent-plan")

	// Confirm, then look at Properties: the job locks the project, shows its dot, never switches tabs.
	page.MustElement(`[data-cc-agent-action="confirm"]`).MustClick()
	page.MustElement(`[data-cc-panel="properties"]`).MustClick()
	s.wait(`()=>cydFixture.state.heldJob!==null && !document.querySelector('[data-cc-lock]').hidden && document.querySelector('[data-cc-action="build"]').disabled && !document.querySelector('[data-cc-agent-dot]').hidden`)
	s.wait(`()=>document.querySelector('[data-cc-panel="properties"]').getAttribute('aria-selected')==='true'`)
	page.MustElement(`[data-cc-action="mode-code"]`).MustClick()
	page.MustElement(`[data-cc-file="src/app/app_main.cpp"]`).MustClick()
	s.wait(`()=>document.querySelector('.cc-code-editor .cm-content')?.getAttribute('contenteditable')==='false' && document.querySelector('[data-cc-code-action="new"]').disabled`)
	s.noRawKeys("locked")
	s.shot("locked")
	// A read-only viewer can still cancel the running job (plan 2 allows the cancel routes in read-only mode).
	page.MustEval(`()=>{const i=cydFixture.inst();i.readonly=true;cydFixture.emit(i.projectId,'job_status',{job:cydFixture.state.heldJob},false)}`)
	s.wait(`()=>{const c=document.querySelector('.cc-job [data-cc-agent-action="cancel"]');return !!c && !c.disabled && document.querySelector('[data-cc-agent-input]').disabled}`)
	page.MustEval(`()=>{const i=cydFixture.inst();i.readonly=false;cydFixture.emit(i.projectId,'job_status',{job:cydFixture.state.heldJob},false)}`)
	s.wait(`()=>!document.querySelector('[data-cc-agent-input]').disabled`)
	page.MustEval(`()=>cydFixture.finishJob()`)
	s.wait(`()=>document.querySelector('[data-cc-lock]').hidden && document.querySelector('[data-cc-agent-dot]').hidden && document.querySelector('.cc-code-editor .cm-content').getAttribute('contenteditable')==='true'`)
	s.wait(`()=>document.querySelector('[data-cc-panel="properties"]').getAttribute('aria-selected')==='true'`)
	// The job's layout change reached the window (project_changed with origin job:<id>).
	s.wait(`()=>{const i=cydFixture.inst();return i.layout().version===cydFixture.state.layouts.get(i.projectId).version && i.layout().project.screens[0].title==='Thermometer'}`)
	page.MustElement(`[data-cc-panel="agent"]`).MustClick()
	s.wait(`()=>[...document.querySelectorAll('[data-cc-message][data-role="agent"]')].some(m=>m.textContent.includes('Fertig')) && document.querySelectorAll('[data-cc-message][data-role="user"]').length===1`)
	s.shot("agent-done")
	s.failOnPageErrors("agent")

	// Revisions: list, diff and an enabled Restore once a version is chosen.
	page.MustElement(`[data-cc-action="revisions"]`).MustClick()
	s.wait(`()=>document.querySelectorAll('[data-cc-revision]').length===2`)
	page.MustElement(`[data-cc-revision]`).MustClick()
	s.wait(`()=>!!document.querySelector('[data-cc-revision-diff]')?.textContent.includes('cyd::log::info') && document.querySelector('.cc-dialog [data-cc-dialog-action="restore"]')?.disabled===false`)
	s.noRawKeys("revisions")
	s.shot("revisions")
	page.Keyboard.MustType(input.Escape)
	s.wait(`()=>!document.querySelector('.cc-dialog')`)

	// The stream gives up: after three failed connects in a row GET /projects/{id} answers 401, the
	// window stops reconnecting and shows why; Retry resumes after the last applied event.
	page.MustEval(`()=>cydFixture.breakStream('unauthorized')`)
	s.wait(`()=>cydFixture.state.streamErrors>=1 && !document.querySelector('[data-cc-stream-state]').hidden`)
	s.wait(`()=>{const b=document.querySelector('[data-cc-stream-stopped]');return !b.hidden && b.textContent.includes(t('cyd_creator.error.unauthorized')) && !!b.querySelector('[data-cc-action="reconnect"]') && cydFixture.state.streams.every(es=>es.closed)}`)
	s.noRawKeys("stream stopped")
	s.shot("stream-stopped")
	page.MustEval(`()=>{cydFixture.state.streamDown='';document.querySelector('[data-cc-stream-stopped] [data-cc-action="reconnect"]').click()}`)
	s.wait(`()=>{const es=cydFixture.state.streams.at(-1);return es.ready && !es.closed && Number(new URL(es.url,location.origin).searchParams.get('after'))>0 && document.querySelector('[data-cc-stream-stopped]').hidden && document.querySelector('[data-cc-stream-state]').hidden}`)
	s.failOnPageErrors("stream")

	// Both desktop themes keep the chrome readable; the editor follows a light theme (no One Dark).
	const oneDark = `getComputedStyle(document.querySelector('.cc-code-editor .cm-editor')).backgroundColor==='rgb(40, 44, 52)'`
	page.MustEval(`()=>fixtureTheme('fruity-light')`)
	s.wait(`()=>!(` + oneDark + `)`)
	s.shot("project-fruity-light")
	page.MustEval(`()=>fixtureTheme('fruity-dark')`)
	s.wait(`()=>` + oneDark)
	s.shot("project-fruity-dark")
	page.MustEval(`()=>fixtureTheme('standard')`)

	// A narrow window stacks the side panel under the editor without horizontal scrolling.
	page.MustSetViewport(820, 900, 1, false)
	s.wait(`()=>{const a=document.querySelector('.cc-app');return a.scrollWidth<=a.clientWidth+1 && getComputedStyle(document.querySelector('.cc-workbench')).gridTemplateRows.split(' ').length===2}`)
	s.shot("project-narrow")
	page.MustSetViewport(1440, 900, 1, false)

	// Closing the window disposes the instance and its event stream.
	page.MustEval(`()=>fixtureCloseAll()`)
	s.wait(`()=>window.CYDCreatorApp.instances.size===0 && cydFixture.state.streams.every(es=>es.closed)`)
	s.failOnPageErrors("final")
}

// cydCreatorSmoke holds the page and the screenshot folder of TestDesktopCYDCreatorShellBrowser.
type cydCreatorSmoke struct {
	t    *testing.T
	page *rod.Page
	dir  string
}

// startCYDCreatorSmoke serves the desktop shell with the aurora and CYD Creator fixtures and opens
// CYD Creator in a 1440×900 page.
func startCYDCreatorSmoke(t *testing.T) *cydCreatorSmoke {
	t.Helper()
	html := regexp.MustCompile(`(?s)<script\b[^>]*>.*?</script>`).ReplaceAllString(readDesktopAssetText(t, "desktop.html"), "")
	html = regexp.MustCompile(`\{\{[^}]*\}\}`).ReplaceAllString(html, "")
	html = strings.Replace(html, "</body>", `<script src="/js/shared/lazy-assets.js"></script><script src="/js/desktop/core/module-loader.js"></script><script src="/cyd-creator-shell.js"></script><script src="/testdata/aurora-fixture.js"></script><script src="/testdata/cyd-creator-fixture.js"></script></body>`, 1)
	shell := readDesktopAssetText(t, "js/desktop/bundles/main.bundle.js")
	cut := strings.LastIndex(shell, "    ensureDesktopRadialMenuAnchor();")
	if cut < 0 {
		t.Fatal("desktop startup seam missing")
	}
	shell = shell[:cut] + `window.aurora={state,openApp,loadIconManifest,applyDesktopSettings,renderIcons,renderStartApps,renderStartButtonIcon,closeWindow,focusWindow,showContextMenu,closeContextMenu,wireShellChromeControls,bindViewportMetrics,handleDesktopKeydown,handleDesktopEvent,disposeWebampMusic,launchStandaloneWebamp};})();`

	apps, err := json.Marshal(desktop.BuiltinApps())
	if err != nil {
		t.Fatal(err)
	}
	words := map[string]string{}
	err = fs.WalkDir(Content, "lang", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, "/de.json") {
			return err
		}
		data, err := fs.ReadFile(Content, path)
		if err != nil {
			return err
		}
		part := map[string]string{}
		if json.Unmarshal(data, &part) == nil {
			for key, value := range part {
				words[key] = value
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(Content)))
	mux.HandleFunc("/fixture", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, html)
	})
	mux.HandleFunc("/cyd-creator-shell.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/javascript")
		fmt.Fprint(w, shell)
	})
	mux.HandleFunc("/fixture-words", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(words)
	})
	mux.HandleFunc("/fixture-apps", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(apps)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	dir := os.Getenv("AURAGO_BROWSER_ARTIFACT_DIR")
	if dir == "" {
		dir = filepath.Join("..", "reports", "cyd-creator")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	page := newSmokeBrowser(t).MustPage().Timeout(150 * time.Second)
	t.Cleanup(func() { _ = page.Close() })
	page.MustSetViewport(1440, 900, 1, false)
	page.MustNavigate(srv.URL + "/fixture").MustWaitLoad()
	page.MustEval(`async()=>{await fixtureReady;await fixtureOpen('cyd-creator');}`)
	return &cydCreatorSmoke{t: t, page: page, dir: dir}
}

func (s *cydCreatorSmoke) wait(expr string) {
	s.t.Helper()
	waitForJSBool(s.t, s.page, expr)
}

// shot saves a screenshot once two frames and a short pause let the layout settle.
func (s *cydCreatorSmoke) shot(name string) {
	s.t.Helper()
	s.page.MustEval(`()=>new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(()=>setTimeout(r,160))))`)
	dest := filepath.Join(s.dir, name+".png")
	s.page.MustScreenshot(dest)
	info, err := os.Stat(dest)
	if err != nil || info.Size() == 0 {
		s.t.Fatalf("screenshot %s missing or empty (%v)", dest, err)
	}
	s.t.Logf("wrote %s (%d bytes)", dest, info.Size())
}

func (s *cydCreatorSmoke) failOnPageErrors(step string) {
	s.t.Helper()
	if errs := s.page.MustEval(`()=>JSON.stringify(fixtureErrors)`).Str(); errs != "[]" {
		s.t.Fatalf("%s: page errors %s", step, errs)
	}
}

// noRawKeys fails when the CYD Creator window shows an untranslated cyd_creator.* key in its text or
// in a title, label or placeholder.
func (s *cydCreatorSmoke) noRawKeys(step string) {
	s.t.Helper()
	found := s.page.MustEval(`()=>{
		const out=new Set();
		document.querySelectorAll('.cc-app').forEach(app=>{
			const win=app.closest('.vd-window')||app;
			const texts=[win.innerText];
			win.querySelectorAll('[title],[aria-label],[placeholder]').forEach(n=>['title','aria-label','placeholder'].forEach(a=>texts.push(n.getAttribute(a)||'')));
			texts.forEach(text=>(String(text).match(/cyd_creator\.[A-Za-z0-9_.]+/g)||[]).forEach(k=>out.add(k)));
		});
		return JSON.stringify([...out]);
	}`).Str()
	if found != "[]" {
		s.t.Fatalf("%s: untranslated keys %s", step, found)
	}
}
