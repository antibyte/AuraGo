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
	"github.com/go-rod/rod/lib/proto"
)

// TestDesktopEasyDragBrowser drives the EasyDrag app inside the real desktop shell against the
// in-memory backend of testdata/easydrag-fixture.js and saves screenshots of every screen.
func TestDesktopEasyDragBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	s := startEasyDragSmoke(t)
	page := s.page

	// Start page: the seeded flow and both templates.
	s.wait(`()=>document.querySelectorAll('.ed-flow-card').length===1 && document.querySelectorAll('.ed-template-card').length===2`)
	s.shot("home-standard")
	s.noRawKeys("home")

	// Flows switched off while the start page is open: the next list request gets 503
	// FLOWS_DISABLED and the grid shows the calm lock card instead of an error.
	s.homeDisabled()

	// Editor with the seeded four-step flow.
	page.MustElement(".ed-flow-card h3").MustClick()
	s.wait(`()=>document.querySelectorAll('.ed-node').length===4 && document.querySelectorAll('.ed-edge').length===3`)
	s.wait(`()=>document.querySelector('[data-ed-state]').textContent.length>0 && getComputedStyle(document.querySelector('.ed-empty')).display==='none'`)
	s.shot("editor-standard")
	s.failOnPageErrors("open editor")
	s.noRawKeys("editor")
	// The state chip follows the flow name, and no card repeats its label on the second line.
	s.wait(`()=>{const name=document.querySelector('[data-ed-name]').getBoundingClientRect();const chip=document.querySelector('[data-ed-state] .ed-chip').getBoundingClientRect();return chip.left-name.right>=0 && chip.left-name.right<=16}`)
	s.wait(`()=>[...document.querySelectorAll('.ed-node')].every(c=>{const sub=c.querySelector('.ed-node-summary');return !sub || sub.textContent!==c.querySelector('.ed-node-label').textContent})`)
	// The flow opens at a readable zoom (no stored view yet), and the name field keeps its own font.
	s.wait(`()=>edFixture.editor().view.zoom>=0.8`)
	s.wait(`()=>{const cs=getComputedStyle(document.querySelector('[data-ed-name]'));return cs.fontSize==='16px' && cs.fontWeight==='700'}`)
	// The schedule trigger's summary ({mode}) shows the label of its mode.
	s.wait(`()=>document.querySelector('.ed-node[data-node-id="n_zeitplan"] .ed-node-summary').textContent==='Werktags (Mo–Fr)'`)

	// Keyboard focus: an arrow key on the focused canvas selects the first step; the canvas shows
	// its focus ring and the step its selection ring.
	page.MustEval(`()=>document.querySelector('.ed-canvas').focus()`)
	page.Keyboard.MustType(input.ArrowRight)
	s.wait(`()=>document.querySelector('.ed-canvas').matches(':focus-visible') && document.querySelectorAll('.ed-node.is-selected').length===1`)
	s.settle()
	s.shot("editor-focus")

	// Keyboard: select the PDF step, Tab, search, Enter appends a connected Telegram step whose
	// file parameter is prefilled with the PDF.
	page.MustEval(`()=>{const ed=edFixture.editor();ed.bus.emit('focus-node',ed.model.doc.nodes.find(n=>n.key==='pdf').id);document.querySelector('.ed-canvas').focus();}`)
	page.Keyboard.MustType(input.Tab)
	s.wait(`()=>document.activeElement && document.activeElement.classList.contains('ed-quick-search')`)
	page.MustElement(".ed-quick-search").MustInput("Telegram")
	s.wait(`()=>document.querySelectorAll('.ed-quick .ed-palette-item').length===1`)
	page.Keyboard.MustType(input.Enter)
	s.wait(`()=>{const ed=edFixture.editor();const tg=ed.model.doc.nodes.find(n=>n.type==='notify.telegram');return !!tg && ed.model.doc.edges.some(e=>e.target.node===tg.id) && tg.params.file==='{{pdf.file}}'}`)

	// Undo removes step and wire together, redo brings both back. The edit menu only shows the
	// keys (shortcutHint); the editor's own handler runs them with focus on the canvas.
	page.MustEval(`()=>document.querySelector('.ed-canvas').focus()`)
	page.KeyActions().Press(input.ControlLeft).Type('z').MustDo()
	s.wait(`()=>edFixture.editor().model.doc.nodes.length===4 && edFixture.editor().model.doc.edges.length===3`)
	page.KeyActions().Press(input.ControlLeft).Press(input.ShiftLeft).Type('z').MustDo()
	s.wait(`()=>edFixture.editor().model.doc.nodes.length===5 && edFixture.editor().model.doc.edges.length===4`)

	// Autosave reaches the backend.
	s.wait(`()=>{const s=edFixture.lastSave();return !!s && s.nodes.length===5}`)

	// A save the server answers with 500 leaves the draft offline (retried by itself); a refused
	// one (403 FLOW_PERMISSION_DENIED, flows are read-only) fails, and its chip becomes a retry button that saves.
	s.saveFailures()

	// Detail view: type into the message and click the AI text to insert it at the caret.
	page.MustEval(`()=>{const ed=edFixture.editor();ed.bus.emit('open-detail',{nodeId:ed.model.doc.nodes.find(n=>n.type==='notify.telegram').id})}`)
	s.wait(`()=>!!document.querySelector('.ed-detail .ed-field[data-param="message"] .ed-tpl-view')`)
	page.MustElement(`.ed-detail .ed-field[data-param="message"] .ed-tpl-view`).MustClick()
	page.MustElement(`.ed-detail .ed-field[data-param="message"] textarea`).MustInput("Neu: ")
	// Typing into the required message removes its "required" marker at once.
	s.wait(`()=>{const f=document.querySelector('.ed-detail .ed-field[data-param="message"]');return !f.classList.contains('has-issue') && !f.querySelector('.ed-field-issue')}`)
	// A step that keeps its type's label names only its key under it.
	s.wait(`()=>{const sub=document.querySelector('.ed-detail .ed-detail-type');return sub.textContent==='telegram_senden' && !!sub.querySelector('.ed-code')}`)
	// One focus ring: the field's own; the text area inside draws none.
	s.wait(`()=>{const ta=document.querySelector('.ed-detail .ed-field[data-param="message"] textarea');const cs=getComputedStyle(ta);return document.activeElement===ta && !!ta.closest('.ed-tpl.is-editing') && cs.boxShadow==='none' && cs.outlineStyle==='none'}`)
	s.shot("detail-editing")
	page.MustElement(`.ed-detail .ed-tree-row[data-ref="ki.text"]`).MustClick()
	s.wait(`()=>edFixture.editor().model.doc.nodes.find(n=>n.type==='notify.telegram').params.message==='Neu: {{ki.text}}'`)
	s.wait(`()=>document.querySelectorAll('.ed-detail .ed-field[data-param="message"] .ed-ref').length===1`)
	// The live hints follow the edit: the filled message is no longer reported as empty.
	s.wait(`()=>!document.querySelector('.ed-detail .ed-field[data-param="message"] .ed-field-issue')`)
	s.shot("detail-standard")
	s.noRawKeys("detail")
	page.Keyboard.MustType(input.Escape)
	s.wait(`()=>!document.querySelector('.ed-detail-backdrop')`)

	// A step test runs the steps before it too: testing the Telegram step also asks about the
	// files of the PDF step in front of it. Nothing is confirmed while the dialog is cancelled.
	page.MustEval(`()=>{const ed=edFixture.editor();ed.bus.emit('node-test',{nodeId:ed.model.doc.nodes.find(n=>n.type==='notify.telegram').id})}`)
	s.wait(`()=>[...document.querySelectorAll('.ed-modal [data-ed-test-effects] [data-ed-effect]')].map(li=>li.dataset.edEffect).sort().join()==='sends_message,writes_files'`)
	s.shot("test-step-dialog")
	page.MustElement(`.ed-modal [data-ed-action="cancel"]`).MustClick()
	s.wait(`()=>!document.querySelector('.ed-modal-backdrop') && edFixture.editor().effectsConfirmed.size===0`)

	// Test run: the dialog warns about real effects, and running confirms each of them; the
	// stream animates every step. The stream is dropped once on the way (resync), and the editor
	// reconnects after the last event it applied.
	s.fit()
	page.KeyActions().Press(input.ControlLeft).Type(input.Enter).MustDo()
	s.wait(`()=>!!document.querySelector('.ed-modal [data-ed-action="run"]') && document.querySelectorAll('.ed-modal .ed-callout--warn .ed-effects li').length===2`)
	s.shot("test-dialog")
	s.noRawKeys("test dialog")
	page.MustEval(`()=>{edFixture.state.delay=450;edFixture.state.resyncAfter=3}`)
	page.MustElement(`.ed-modal [data-ed-action="run"]`).MustClick()
	s.wait(`()=>{const c=edFixture.editor().effectsConfirmed;return c instanceof Set && c.has('writes_files') && c.has('sends_message')}`)
	s.wait(`()=>document.querySelectorAll('.ed-node.status-running').length===1`)
	// The explicit fit shows the whole flow; below 70 % the cards show their labels only.
	s.wait(`()=>{const z=edFixture.editor().view.zoom;const c=document.querySelector('.ed-canvas');return (z<0.7)===c.classList.contains('is-zoomed-out') && (z>=0.7 || getComputedStyle(document.querySelector('.ed-node-summary')).display==='none')}`)
	s.shot("run-live")
	page.MustEval(`()=>{edFixture.state.delay=60}`)
	s.wait(`()=>{const ed=edFixture.editor();return !!ed.run && ed.run.status==='success' && document.querySelectorAll('.ed-node.status-success').length===5 && document.querySelectorAll('.ed-pill').length>=3}`)
	s.wait(`()=>edFixture.state.streams.some(es=>/[?&]after=\d+/.test(es.url))`)
	s.shot("run-done")
	s.failOnPageErrors("test run")

	// Publish (no blocking problems left). The first answer is partial: the revision is live but
	// Mission Control was not updated, so the state chip warns and Publish stays offered.
	// Publishing again finishes it.
	page.MustElement(`[data-ed-cmd="publish"]`).MustClick()
	s.wait(`()=>!!document.querySelector('.ed-modal [data-ed-action="publish"]')`)
	s.shot("publish-dialog")
	s.noRawKeys("publish dialog")
	s.publishPartial()
	page.MustElement(`[data-ed-cmd="publish"]`).MustClick()
	s.wait(`()=>!!document.querySelector('.ed-modal [data-ed-action="publish"]') && !!document.querySelector('.ed-modal .ed-callout--warn')`)
	page.MustElement(`.ed-modal [data-ed-action="publish"]`).MustClick()
	s.wait(`()=>{const r=edFixture.flow().rec;const ed=edFixture.editor();return r.published_draft_revision===r.draft_revision && !document.querySelector('.ed-modal-backdrop') && !ed.publishIncomplete && !!document.querySelector('[data-ed-state] .ed-chip--ok')}`)
	s.clearToasts()

	// Run drawer and read-only run view; Esc returns to the draft.
	page.MustElement(`[data-ed-cmd="runs"]`).MustClick()
	s.wait(`()=>document.querySelectorAll('.ed-run-row').length===1`)
	page.MustElement(`.ed-run-row`).MustClick()
	s.wait(`()=>!document.querySelector('.ed-runview-banner').hidden && document.querySelectorAll('.ed-node.is-readonly').length===5`)
	s.settle()
	// The run view hides the palette and shows the flow at a readable zoom from its trigger, in the
	// part of the canvas the drawer leaves free.
	s.wait(`()=>{const ed=edFixture.editor();const trigger=document.querySelector('.ed-node[data-node-id="n_zeitplan"]').getBoundingClientRect();const canvas=document.querySelector('.ed-canvas').getBoundingClientRect();const drawer=document.querySelector('.ed-drawer').getBoundingClientRect();return document.querySelector('.ed-palette').classList.contains('is-collapsed') && ed.view.zoom>=0.8 && trigger.left>=canvas.left && trigger.right<=drawer.left}`)
	// The hidden palette is inert, and its rail toggle waits until the run view ends.
	s.wait(`()=>document.querySelector('.ed-palette').inert===true && document.querySelector('.ed-rail [data-ed-cmd="palette"]').getAttribute('aria-disabled')==='true'`)
	s.shot("run-view")
	s.noRawKeys("run view")
	page.MustEval(`()=>document.querySelector('.ed-canvas').focus()`)
	page.Keyboard.MustType(input.Escape)
	s.wait(`()=>document.querySelector('.ed-runview-banner').hidden && !edFixture.editor().runView`)
	// The palette is back, and the footer still names the last run once the run view is left.
	s.wait(`()=>!document.querySelector('.ed-palette').classList.contains('is-collapsed') && document.querySelector('.ed-palette').inert===false && document.querySelector('.ed-rail [data-ed-cmd="palette"]').getAttribute('aria-disabled')==='false'`)
	s.wait(`()=>!document.querySelector('[data-ed-last-run]').textContent.includes(edFixture.editor().t('easydrag.ui.home_never_ran'))`)
	page.MustElement(`[data-ed-cmd="runs"]`).MustClick()
	s.wait(`()=>!document.querySelector('.ed-drawer')`)

	// flows_changed desktop events reach open editors.
	page.MustEval(`()=>{window.__flowsChanged=0;document.addEventListener('aurago:flows-changed',e=>{if(e.detail&&e.detail.reason==='enabled'&&e.detail.flow_id===edFixture.seededID)window.__flowsChanged++;});return aurora.handleDesktopEvent({type:'flows_changed',payload:{flow_id:edFixture.seededID,reason:'enabled'}});}`)
	s.wait(`()=>window.__flowsChanged===1`)
	// Mission Control pauses the flow: the editor says Paused and offers no Run now.
	s.pausedElsewhere()

	// A test run that fails at its last step, a selected step, and the three themes: success and
	// error states, delivered wires with item counts, categories and the selection ring.
	s.failingRun()
	for _, theme := range []string{"fruity-light", "fruity-dark"} {
		page.MustEval(`theme=>fixtureTheme(theme)`, theme)
		s.shot("editor-" + theme)
	}
	page.MustEval(`()=>fixtureTheme('standard')`)
	s.failedRunView()
	// Live runs that a trigger started and that wait for a slot can be stopped here.
	s.stopWaitingRuns()
	// A step test asks about the effects of the real settings, such as an HTTP POST.
	s.realEffects()
	// The HTTP step's secret field: deleting the secret, and a secret that is gone.
	s.secretField()
	// A narrow window: the palette floats over the canvas, so it closes; wide again, it is back.
	page.MustSetViewport(820, 900, 1, false)
	s.wait(`()=>getComputedStyle(document.querySelector('.ed-palette')).position==='absolute' && document.querySelector('.ed-palette').classList.contains('is-collapsed')`)
	s.shot("editor-narrow")
	s.failOnPageErrors("narrow")
	page.MustSetViewport(1440, 900, 1, false)
	s.wait(`()=>getComputedStyle(document.querySelector('.ed-palette')).position!=='absolute' && !document.querySelector('.ed-palette').classList.contains('is-collapsed')`)

	// A phone: EasyDrag opens over the whole width and the header stays usable.
	s.phone()

	// Mission Control lists the flow missions with the EasyDrag badge, the trigger summary or
	// "Not published yet", and opens them in EasyDrag.
	page.MustSetViewport(1440, 900, 1, false)
	s.missionControl()
	// A flow deleted elsewhere while it is open: the editor says so and shows the start page.
	s.deletedElsewhere()
	s.failOnPageErrors("final")
}

// pausedElsewhere switches the seeded flow off as Mission Control would (the broadcast names the
// flow): the state chip says Paused, the switch is off, and Run now is off in the ⋯ menu and the
// Flow menu with a hint. Switched on again, Run now is back.
func (s *easyDragSmoke) pausedElsewhere() {
	s.t.Helper()
	page := s.page
	broadcast := `()=>aurora.handleDesktopEvent({type:'flows_changed',payload:{flow_id:edFixture.seededID,reason:'enabled'}})`
	runItem := `const ed=edFixture.editor();const item=aurora.state.windowMenus.get(ed.windowId).renderedMenus.find(m=>m.id==='flow').items.find(i=>i.id==='run');`
	page.MustEval(`()=>{edFixture.flow().enabled=false}`)
	page.MustEval(broadcast)
	s.wait(`()=>{` + runItem + `const chips=[...document.querySelectorAll('[data-ed-state] .ed-chip')].map(c=>c.textContent);return ed.flowEnabled===false && chips.includes(ed.t('easydrag.ui.state_inactive')) && document.querySelector('[data-ed-cmd="active"]').getAttribute('aria-checked')==='false' && item.disabled && item.disabledHint===ed.t('easydrag.ui.error_flow_disabled')}`)
	page.MustElement(`[data-ed-cmd="more"]`).MustClick()
	s.wait(`()=>{const ed=edFixture.editor();const b=[...document.querySelectorAll('.vd-context-menu .vd-context-item')].find(x=>x.textContent.includes(ed.t('easydrag.ui.run_now')));return !!b && b.disabled && b.title===ed.t('easydrag.ui.error_flow_disabled')}`)
	s.shot("editor-paused")
	s.noRawKeys("paused")
	page.MustEval(`()=>aurora.closeContextMenu()`)
	page.MustEval(`()=>{edFixture.flow().enabled=true}`)
	page.MustEval(broadcast)
	s.wait(`()=>{` + runItem + `return ed.flowEnabled===true && document.querySelectorAll('[data-ed-state] .ed-chip').length===1 && !item.disabled}`)
	s.failOnPageErrors("paused elsewhere")
}

// deletedElsewhere opens a second flow, edits it and deletes it behind the editor's back (as
// Mission Control does): the editor names the delete, shows the start page and keeps no
// emergency copy of the dead flow.
func (s *easyDragSmoke) deletedElsewhere() {
	s.t.Helper()
	page := s.page
	id := page.MustEval(`()=>{const id=edFixture.addDraft('Kurzlebig');const ed=edFixture.editor();EasyDragApp.open(ed.windowId,{flowId:id});return id;}`).Str()
	s.wait(fmt.Sprintf(`()=>{const ed=edFixture.editor();return !!ed && ed.flow.id==='%s' && document.querySelectorAll('.ed-node').length>0}`, id))
	page.MustEval(`()=>edFixture.editor().model.setFlow({description:'unsaved'})`)
	s.wait(fmt.Sprintf(`()=>localStorage.getItem('aurago.easydrag.draft.%s')!==null`, id))
	page.MustEval(fmt.Sprintf(`()=>{edFixture.state.flows.delete('%s');return aurora.handleDesktopEvent({type:'flows_changed',payload:{flow_id:'%s',reason:'deleted'}});}`, id, id))
	s.wait(fmt.Sprintf(`()=>!edFixture.editor() && !!document.querySelector('.ed-home') && localStorage.getItem('aurago.easydrag.draft.%s')===null && [...document.querySelectorAll('.vd-toast')].some(x=>x.textContent.includes(t('easydrag.ui.flow_deleted_elsewhere')))`, id))
	s.clearToasts()
	s.failOnPageErrors("deleted elsewhere")
}

// easyDragSmoke holds the page and the screenshot folder of TestDesktopEasyDragBrowser.
type easyDragSmoke struct {
	t    *testing.T
	page *rod.Page
	dir  string
}

// startEasyDragSmoke serves the desktop shell with the aurora and EasyDrag fixtures and opens
// EasyDrag in a 1440×900 page.
func startEasyDragSmoke(t *testing.T) *easyDragSmoke {
	t.Helper()
	html := regexp.MustCompile(`(?s)<script\b[^>]*>.*?</script>`).ReplaceAllString(readDesktopAssetText(t, "desktop.html"), "")
	html = regexp.MustCompile(`\{\{[^}]*\}\}`).ReplaceAllString(html, "")
	html = strings.Replace(html, "</body>", `<script src="/js/shared/lazy-assets.js"></script><script src="/js/desktop/core/module-loader.js"></script><script src="/easydrag-shell.js"></script><script src="/testdata/aurora-fixture.js"></script><script src="/testdata/easydrag-fixture.js"></script></body>`, 1)
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
	mux.HandleFunc("/easydrag-shell.js", func(w http.ResponseWriter, r *http.Request) {
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
		dir = filepath.Join("..", "reports", "easydrag")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	page := newSmokeBrowser(t).MustPage().Timeout(150 * time.Second)
	t.Cleanup(func() { _ = page.Close() })
	page.MustSetViewport(1440, 900, 1, false)
	page.MustNavigate(srv.URL + "/fixture").MustWaitLoad()
	page.MustEval(`async()=>{await fixtureReady;await fixtureOpen('easydrag');}`)
	return &easyDragSmoke{t: t, page: page, dir: dir}
}

func (s *easyDragSmoke) wait(expr string) {
	s.t.Helper()
	waitForJSBool(s.t, s.page, expr)
}

// settle waits until the canvas ends an animated pan or zoom.
func (s *easyDragSmoke) settle() {
	s.t.Helper()
	s.wait(`()=>![...document.querySelectorAll('.ed-canvas')].some(c=>c.classList.contains('is-animating'))`)
}

// fit shows the whole flow (Shift+1 on the canvas). The editor's "view" event says the key was
// handled; the animation class is no signal (it is short, and never set with reduced motion).
func (s *easyDragSmoke) fit() {
	s.t.Helper()
	s.page.MustEval(`()=>{const ed=edFixture.editor();window.__edViewed=false;const off=ed.bus.on('view',()=>{window.__edViewed=true;off();});document.querySelector('.ed-canvas').focus();}`)
	s.page.KeyActions().Press(input.ShiftLeft).Type(input.Digit1).MustDo()
	s.wait(`()=>window.__edViewed===true`)
	s.settle()
}

// shot saves a screenshot once two frames and a short pause let the layout settle.
func (s *easyDragSmoke) shot(name string) {
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

// clearToasts closes the desktop notifications, which cover the window's header.
func (s *easyDragSmoke) clearToasts() {
	s.t.Helper()
	s.page.MustEval(`()=>document.querySelectorAll('.vd-toast .vd-toast-close').forEach(b=>b.click())`)
	s.wait(`()=>!document.querySelector('.vd-toast')`)
}

func (s *easyDragSmoke) failOnPageErrors(step string) {
	s.t.Helper()
	if errs := s.page.MustEval(`()=>JSON.stringify(fixtureErrors)`).Str(); errs != "[]" {
		s.t.Fatalf("%s: page errors %s", step, errs)
	}
}

// noRawKeys fails when an EasyDrag window shows an untranslated key: in its text, or in a title,
// label or placeholder of its markup.
func (s *easyDragSmoke) noRawKeys(step string) {
	s.t.Helper()
	found := s.page.MustEval(`()=>{
		const out=new Set();
		document.querySelectorAll('.ed-app').forEach(app=>{
			const win=app.closest('.vd-window')||app;
			const texts=[win.innerText];
			win.querySelectorAll('[title],[aria-label],[placeholder]').forEach(n=>['title','aria-label','placeholder'].forEach(a=>texts.push(n.getAttribute(a)||'')));
			texts.forEach(text=>(String(text).match(/easydrag\.(ui|node)\.[A-Za-z0-9_.]+/g)||[]).forEach(k=>out.add(k)));
		});
		return JSON.stringify([...out]);
	}`).Str()
	if found != "[]" {
		s.t.Fatalf("%s: untranslated keys %s", step, found)
	}
}

func (s *easyDragSmoke) homeDisabled() {
	s.t.Helper()
	broadcast := `()=>aurora.handleDesktopEvent({type:'flows_changed',payload:{flow_id:'',reason:'enabled'}})`
	s.page.MustEval(`()=>{edFixture.state.disabled=true}`)
	s.page.MustEval(broadcast)
	s.wait(`()=>{const card=document.querySelector('.ed-flow-grid .ed-home-empty');return !!card && !!card.querySelector('h3') && !document.querySelector('.ed-flow-card') && document.querySelector('[data-ed-home-search]').disabled}`)
	// Nothing can be created meanwhile: New flow, Import and the templates wait.
	s.wait(`()=>document.querySelector('.ed-home-hero [data-ed-new]').disabled && document.querySelector('[data-ed-import]').disabled && document.querySelector('.ed-template-grid').classList.contains('is-disabled') && [...document.querySelectorAll('.ed-template-card')].every(c=>c.getAttribute('aria-disabled')==='true' && c.tabIndex===-1)`)
	// The card offers the window lock card's actions: Open settings and Try again.
	s.wait(`()=>{const card=document.querySelector('.ed-flow-grid .ed-home-empty');return !!card.querySelector('[data-ed-home-settings]') && !!card.querySelector('[data-ed-home-retry]')}`)
	s.shot("home-disabled")
	s.noRawKeys("home disabled")
	s.failOnPageErrors("home disabled")
	// Try again asks for the list once more; with flows back on, the cards return.
	s.page.MustEval(`()=>{edFixture.state.disabled=false}`)
	s.page.MustElement(`.ed-flow-grid [data-ed-home-retry]`).MustClick()
	s.wait(`()=>document.querySelectorAll('.ed-flow-card').length===1 && !document.querySelector('[data-ed-home-search]').disabled && !document.querySelector('.ed-home-hero [data-ed-new]').disabled && !document.querySelector('.ed-template-grid').classList.contains('is-disabled')`)
}

// realEffects adds an HTTP request with POST behind the PDF step. Its catalog entry (default GET)
// names no effect, yet a step test on it asks about the message it sends: the dialog reads the
// effects of the real settings from the publish preview. secretField removes the step again.
func (s *easyDragSmoke) realEffects() {
	s.t.Helper()
	page := s.page
	page.MustEval(`()=>{const ed=edFixture.editor();ed.effectsConfirmed.clear();const pdf=ed.model.doc.nodes.find(n=>n.key==='pdf').id;window.__httpStep=ed.model.addNode('http.request',{x:1280,y:360},{method:'POST',url:'https://example.test/hook',auth_secret:'wetter_api'},{node:pdf,port:'out'});}`)
	s.wait(`()=>{const last=edFixture.lastSave();return !!last && last.nodes.some(n=>n.type==='http.request')}`)
	page.MustEval(`()=>edFixture.editor().bus.emit('node-test',{nodeId:window.__httpStep})`)
	s.wait(`()=>{const li=[...document.querySelectorAll('.ed-modal [data-ed-test-effects] [data-ed-effect]')];const sends=li.find(x=>x.dataset.edEffect==='sends_message');return li.map(x=>x.dataset.edEffect).sort().join()==='sends_message,writes_files' && !!sends && sends.textContent.includes('HTTP') && !document.querySelector('.ed-modal [data-ed-effects-unchecked]')}`)
	s.shot("test-step-real-effects")
	s.noRawKeys("real effects")
	page.MustElement(`.ed-modal [data-ed-action="cancel"]`).MustClick()
	s.wait(`()=>!document.querySelector('.ed-modal-backdrop')`)
	// Without the preview the dialog says that the effects could not be fully checked.
	page.MustEval(`()=>{edFixture.state.failPreview=true;edFixture.editor().bus.emit('node-test',{nodeId:window.__httpStep});}`)
	s.wait(`()=>!!document.querySelector('.ed-modal [data-ed-effects-unchecked]') && !!document.querySelector('.ed-modal [data-ed-effect="writes_files"]')`)
	s.shot("test-step-unchecked")
	s.noRawKeys("unchecked effects")
	page.MustElement(`.ed-modal [data-ed-action="cancel"]`).MustClick()
	s.wait(`()=>!document.querySelector('.ed-modal-backdrop')`)
	page.MustEval(`()=>{edFixture.state.failPreview=false;}`)
	s.clearToasts()
	s.failOnPageErrors("real effects")
}

// secretField deletes the HTTP step's secret in its detail view. The change rebuilds the form, and
// the new secret list takes the focus. A secret the Vault no longer has is marked as missing. The
// step is removed at the end.
func (s *easyDragSmoke) secretField() {
	s.t.Helper()
	page := s.page
	field := `.ed-detail .ed-field[data-param="auth_secret"]`
	page.MustEval(`()=>edFixture.editor().bus.emit('open-detail',{nodeId:window.__httpStep})`)
	s.wait(`()=>{const b=document.querySelector('` + field + ` [data-ed-secret-delete]');return !!b && !b.disabled && document.querySelector('` + field + ` select').value==='wetter_api'}`)
	page.MustElement(field + ` [data-ed-secret-delete]`).MustClick()
	s.wait(`()=>!!document.querySelector('.ed-modal [data-ed-action="delete"]')`)
	page.MustElement(`.ed-modal [data-ed-action="delete"]`).MustClick()
	s.wait(`()=>{const sel=document.querySelector('` + field + ` select');const node=edFixture.editor().model.node(window.__httpStep);return !document.querySelector('.ed-modal-backdrop') && !!sel && document.activeElement===sel && !sel.value && !('auth_secret' in node.params) && !edFixture.state.secrets.includes('wetter_api')}`)
	s.shot("detail-secret-deleted")
	s.noRawKeys("secret deleted")
	page.Keyboard.MustType(input.Escape)
	s.wait(`()=>!document.querySelector('.ed-detail-backdrop')`)
	// A secret that is gone: the list names it as missing.
	page.MustEval(`()=>{const ed=edFixture.editor();ed.model.setParam(window.__httpStep,'auth_secret','wetter_alt');ed.bus.emit('open-detail',{nodeId:window.__httpStep});}`)
	s.wait(`()=>{const sel=document.querySelector('` + field + ` select');return !!sel && sel.value==='wetter_alt' && sel.options[sel.selectedIndex].textContent===edFixture.editor().t('easydrag.ui.secret_missing',{name:'wetter_alt'}) && sel.dataset.error===edFixture.editor().t('easydrag.ui.error_flow_secret_unavailable')}`)
	s.shot("detail-secret-missing")
	s.noRawKeys("secret missing")
	page.Keyboard.MustType(input.Escape)
	s.wait(`()=>!document.querySelector('.ed-detail-backdrop')`)
	page.MustEval(`()=>{edFixture.state.secrets.push('wetter_api');edFixture.editor().model.removeNodes([window.__httpStep]);}`)
	s.wait(`()=>{const last=edFixture.lastSave();return !!last && last.nodes.length===5 && !last.nodes.some(n=>n.type==='http.request')}`)
	s.clearToasts()
	s.failOnPageErrors("secret field")
}

// stopWaitingRuns adds two live runs that wait for a slot. One is stopped from its row in the runs
// drawer, the other from the banner of its run view; both ask first.
func (s *easyDragSmoke) stopWaitingRuns() {
	s.t.Helper()
	page := s.page
	ids := page.MustEval(`()=>[edFixture.addWaitingRun(),edFixture.addWaitingRun()]`).Arr()
	first, second := ids[0].Str(), ids[1].Str()
	page.MustElement(`[data-ed-cmd="runs"]`).MustClick()
	s.wait(`()=>document.querySelectorAll('.ed-drawer .ed-run-item [data-ed-run-stop]').length===2`)
	// The Stop button's name contains its visible label and names the run's start time.
	s.wait(fmt.Sprintf(`()=>{const b=document.querySelector('[data-ed-run-stop="%s"]');const name=b.getAttribute('aria-label').toLowerCase();return name.includes(b.textContent.trim().toLowerCase()) && /\d/.test(name)}`, first))
	s.shot("runs-drawer-stop")
	s.noRawKeys("runs drawer stop")
	page.MustElement(fmt.Sprintf(`[data-ed-run-stop="%s"]`, first)).MustClick()
	s.wait(`()=>!!document.querySelector('.ed-modal [data-ed-action="stop"]')`)
	s.shot("run-stop-confirm")
	s.noRawKeys("run stop confirm")
	page.MustElement(`.ed-modal [data-ed-action="stop"]`).MustClick()
	s.wait(fmt.Sprintf(`()=>edFixture.state.runs.get('%s').run.status==='cancelled' && !document.querySelector('.ed-modal-backdrop') && !document.querySelector('[data-ed-run-stop="%s"]') && document.activeElement===document.querySelector('[data-ed-run="%s"]')`, first, first, first))
	// The run view of the other waiting run: Stop in the banner, then the banner shows it stopped.
	// The stopped run's run_finished broadcast redraws the drawer about 250 ms later, so the row
	// is found and clicked in one step (a found row can be detached before a separate click).
	page.MustEval(fmt.Sprintf(`()=>document.querySelector('.ed-run-row[data-ed-run="%s"]').click()`, second))
	s.wait(`()=>{const b=document.querySelector('.ed-runview-banner');return !b.hidden && !b.querySelector('[data-ed-cmd="stop-viewed"]').hidden}`)
	s.settle()
	s.shot("run-view-stop")
	page.MustElement(`.ed-runview-banner [data-ed-cmd="stop-viewed"]`).MustClick()
	s.wait(`()=>!!document.querySelector('.ed-modal [data-ed-action="stop"]')`)
	page.MustElement(`.ed-modal [data-ed-action="stop"]`).MustClick()
	s.wait(fmt.Sprintf(`()=>{const b=document.querySelector('.ed-runview-banner');return edFixture.state.runs.get('%s').run.status==='cancelled' && b.querySelector('[data-ed-cmd="stop-viewed"]').hidden && b.textContent.includes(edFixture.editor().t('easydrag.ui.status_cancelled')) && document.activeElement===b.querySelector('[data-ed-cmd="exit-run-view"]')}`, second))
	s.shot("run-view-stopped")
	s.noRawKeys("run view stopped")
	page.MustEval(`()=>document.querySelector('.ed-canvas').focus()`)
	page.Keyboard.MustType(input.Escape)
	s.wait(`()=>document.querySelector('.ed-runview-banner').hidden`)
	page.MustElement(`[data-ed-cmd="runs"]`).MustClick()
	s.wait(`()=>!document.querySelector('.ed-drawer')`)
	s.clearToasts()
	s.failOnPageErrors("stop waiting runs")
}

// saveFailures moves the selected step down by keyboard: the save gets 500 (offline, retried by
// itself), Ctrl+S then gets 403 FLOW_PERMISSION_DENIED (failed, no retry by itself), and the chip's retry
// button saves.
func (s *easyDragSmoke) saveFailures() {
	s.t.Helper()
	page := s.page
	saves := page.MustEval(`()=>edFixture.state.saves.length`).Int()
	page.MustEval(`()=>{
		edFixture.state.failSaves.push({status:500,code:'FLOW_INTERNAL',error:'the flow service failed; see the server log'},{status:403,code:'FLOW_PERMISSION_DENIED',error:'flows are read-only'});
		const ed=edFixture.editor();
		ed.bus.emit('focus-node',ed.model.doc.nodes.find(n=>n.type==='notify.telegram').id);
		document.querySelector('.ed-canvas').focus();
	}`)
	page.KeyActions().Press(input.ShiftLeft).Type(input.ArrowDown).MustDo()
	s.wait(`()=>!!document.querySelector('[data-ed-save].ed-save--offline')`)
	s.settle()
	s.shot("editor-save-offline")
	page.MustEval(`()=>document.querySelector('.ed-canvas').focus()`)
	page.KeyActions().Press(input.ControlLeft).Type('s').MustDo()
	s.wait(`()=>!!document.querySelector('[data-ed-save].ed-save--failed .ed-save-retry') && edFixture.state.failSaves.length===0`)
	s.shot("editor-save-failed")
	s.noRawKeys("save failed")
	page.MustElement(`[data-ed-save] .ed-save-retry`).MustClick()
	s.wait(`()=>!!document.querySelector('[data-ed-save].ed-save--saved')`)
	if got := page.MustEval(`()=>edFixture.state.saves.length`).Int(); got != saves+1 {
		s.t.Fatalf("the retry must save once, saves went from %d to %d", saves, got)
	}
	s.failOnPageErrors("save failures")
}

// publishPartial publishes from the open dialog while the server answers partial.
func (s *easyDragSmoke) publishPartial() {
	s.t.Helper()
	page := s.page
	// reads counts the editor's GET /api/desktop/flows/<id>: the "published" broadcast makes it re-read the flow.
	reads := `()=>edFixture.state.requests.filter(r=>r==='GET /api/desktop/flows/'+edFixture.seededID).length`
	before := page.MustEval(reads).Int()
	page.MustEval(`()=>{edFixture.state.publishPartial='FLOW_PUBLISH_INCOMPLETE'}`)
	page.MustElement(`.ed-modal [data-ed-action="publish"]`).MustClick()
	s.wait(`()=>{const ed=edFixture.editor();const r=edFixture.flow().rec;return ed.publishIncomplete==='FLOW_PUBLISH_INCOMPLETE' && r.published_draft_revision===r.draft_revision && !document.querySelector('.ed-modal-backdrop') && !!document.querySelector('.vd-toast')}`)
	// The re-read flow has the same live revision, which must not end the warning.
	s.wait(fmt.Sprintf(`()=>(%s)()>%d`, reads, before))
	s.wait(`()=>{const ed=edFixture.editor();const pub=document.querySelector('[data-ed-cmd="publish"]');return ed.publishIncomplete==='FLOW_PUBLISH_INCOMPLETE' && !!document.querySelector('[data-ed-state] .ed-chip--warn') && !pub.disabled && pub.classList.contains('has-changes')}`)
	s.clearToasts()
	s.shot("publish-partial")
	s.noRawKeys("publish partial")
	s.failOnPageErrors("publish partial")
	page.MustEval(`()=>{edFixture.state.publishPartial=''}`)
}

// failingRun runs a test whose Telegram step fails, then selects the AI step and shows the whole
// flow.
func (s *easyDragSmoke) failingRun() {
	s.t.Helper()
	page := s.page
	page.MustEval(`()=>{edFixture.state.failType='notify.telegram'}`)
	s.fit()
	page.KeyActions().Press(input.ControlLeft).Type(input.Enter).MustDo()
	s.wait(`()=>!!document.querySelector('.ed-modal [data-ed-action="run"]') && !document.querySelector('.ed-modal .ed-callout--warn')`)
	page.MustElement(`.ed-modal [data-ed-action="run"]`).MustClick()
	s.wait(`()=>{const ed=edFixture.editor();return !!ed.run && ed.run.status==='error' && document.querySelectorAll('.ed-node.status-error').length===1 && document.querySelectorAll('.ed-node.status-success').length===4 && !!document.querySelector('.ed-node-error')}`)
	page.MustEval(`()=>{edFixture.state.failType='';const ed=edFixture.editor();ed.bus.emit('focus-node',ed.model.doc.nodes.find(n=>n.key==='ki').id);}`)
	s.settle()
	s.fit()
	s.wait(`()=>document.querySelectorAll('.ed-node.is-selected').length===1`)
	// The step's error reads as one sentence, and the selected step's toolbar leaves its status badge free.
	s.wait(`()=>!/[.。।]:/.test(document.querySelector('.ed-node-error').textContent)`)
	s.wait(`()=>{const sel=document.querySelector('.ed-node.is-selected');const a=sel.querySelector('.ed-node-tools').getBoundingClientRect();const b=sel.querySelector('.ed-status').getBoundingClientRect();return a.right<=b.left || b.right<=a.left || a.bottom<=b.top || b.bottom<=a.top}`)
	s.shot("run-error")
	s.noRawKeys("run error")
	s.failOnPageErrors("failing run")
}

// phoneKeyboardAdd adds a step from the floating palette by keyboard (Ctrl+K, Enter on an item):
// the palette closes, turns inert and gives the focus to the canvas. Undo takes the step back.
func (s *easyDragSmoke) phoneKeyboardAdd() {
	s.t.Helper()
	page := s.page
	s.wait(`()=>document.querySelector('.ed-palette').inert===true`)
	page.MustEval(`()=>document.querySelector('.ed-canvas').focus()`)
	page.KeyActions().Press(input.ControlLeft).Type('k').MustDo()
	s.wait(`()=>!document.querySelector('.ed-palette').classList.contains('is-collapsed') && document.querySelector('.ed-palette').inert===false && document.activeElement.classList.contains('ed-palette-search')`)
	page.MustEval(`()=>document.querySelector('.ed-palette .ed-palette-item[data-ed-type="web.search"]').focus()`)
	page.Keyboard.MustType(input.Enter)
	s.wait(`()=>{const p=document.querySelector('.ed-palette');return edFixture.editor().model.doc.nodes.length===6 && p.classList.contains('is-collapsed') && p.inert===true && document.activeElement===document.querySelector('.ed-canvas') && !document.querySelector('.ed-detail-backdrop')}`)
	page.KeyActions().Press(input.ControlLeft).Type('z').MustDo()
	s.wait(`()=>edFixture.editor().model.doc.nodes.length===5`)
	s.wait(`()=>{const last=edFixture.lastSave();return !!last && last.nodes.length===5}`)
	s.failOnPageErrors("phone keyboard add")
}

// failedRunView opens the failed run from the drawer: the run view centres the failed step in the
// part of the canvas the drawer leaves free.
func (s *easyDragSmoke) failedRunView() {
	s.t.Helper()
	page := s.page
	page.MustElement(`[data-ed-cmd="runs"]`).MustClick()
	s.wait(`()=>document.querySelectorAll('.ed-run-row').length===2`)
	page.MustElement(`.ed-run-row .ed-run-dot--error`).MustClick()
	s.wait(`()=>!document.querySelector('.ed-runview-banner').hidden && document.querySelectorAll('.ed-node.status-error').length===1`)
	s.settle()
	s.wait(`()=>{const canvas=document.querySelector('.ed-canvas').getBoundingClientRect();const drawer=document.querySelector('.ed-drawer');const free=canvas.width-drawer.offsetWidth;const card=document.querySelector('.ed-node.status-error').getBoundingClientRect();const mid=card.left+card.width/2-canvas.left;return Math.abs(mid-free/2)<=2 && edFixture.editor().view.zoom>=0.8}`)
	s.shot("run-view-failed")
	s.noRawKeys("failed run view")
	page.MustEval(`()=>document.querySelector('.ed-canvas').focus()`)
	page.Keyboard.MustType(input.Escape)
	s.wait(`()=>document.querySelector('.ed-runview-banner').hidden`)
	page.MustElement(`[data-ed-cmd="runs"]`).MustClick()
	s.wait(`()=>!document.querySelector('.ed-drawer')`)
	s.failOnPageErrors("failed run view")
}

// phone opens EasyDrag afresh in a 390×844 touch viewport, as a phone would: the start page and
// the editor fill the width, and the header keeps every action on screen.
func (s *easyDragSmoke) phone() {
	s.t.Helper()
	page := s.page
	page.MustSetViewport(390, 844, 1, true)
	if err := (proto.EmulationSetTouchEmulationEnabled{Enabled: true}).Call(page); err != nil {
		s.t.Fatal(err)
	}
	defer func() {
		_ = (proto.EmulationSetTouchEmulationEnabled{Enabled: false}).Call(page)
	}()
	page.MustEval(`()=>fixtureCloseAll()`)
	s.wait(`()=>!document.querySelector('.ed-app')`)
	page.MustEval(`async()=>{await fixtureOpen('easydrag')}`)
	s.wait(`()=>document.querySelectorAll('.ed-flow-card').length===1 && document.querySelectorAll('.ed-template-card').length===2`)
	s.wait(`()=>{const r=document.querySelector('.ed-app').getBoundingClientRect();const home=document.querySelector('.ed-home');return r.width>0 && r.left>=0 && r.right<=innerWidth+1 && home.scrollWidth<=home.clientWidth+1}`)
	s.shot("home-phone")
	s.noRawKeys("phone home")
	page.MustElement(".ed-flow-card h3").MustClick()
	s.wait(`()=>document.querySelectorAll('.ed-node').length===5`)
	// The palette floats over the canvas here, so it starts closed and leaves the flow in view. The
	// flow opens at a readable zoom from its trigger, 48 px from the edge.
	s.wait(`()=>document.querySelector('.ed-palette').classList.contains('is-collapsed')`)
	s.wait(`()=>{const ed=edFixture.editor();const canvas=document.querySelector('.ed-canvas').getBoundingClientRect();const trigger=document.querySelector('.ed-node[data-node-id="n_zeitplan"]').getBoundingClientRect();return Math.abs(ed.view.zoom-0.8)<0.001 && Math.abs(trigger.left-canvas.left-48)<=1 && trigger.top>=canvas.top && trigger.bottom<=canvas.bottom}`)
	// The footer keeps a short run status; the other items show icons, named for screen readers.
	s.wait(`()=>{const run=document.querySelector('[data-ed-last-run]');const btn=run.parentElement;const texts=[...document.querySelectorAll('.ed-foot .ed-foot-text')];return getComputedStyle(run).display!=='none' && run.getBoundingClientRect().width>40 && btn.getAttribute('aria-label')===run.textContent && texts.length>=2 && texts.every(x=>x.getBoundingClientRect().width<=1) && !!document.querySelector('[data-ed-cmd="issues"]').getAttribute('aria-label')}`)
	s.wait(`()=>{
		const head=document.querySelector('.ed-head');
		const box=head.getBoundingClientRect();
		const buttons=[...head.querySelectorAll('button')].filter(b=>!b.hidden && b.getClientRects().length);
		return buttons.length>=6 && box.right<=innerWidth+1 && head.scrollWidth<=head.clientWidth+1 &&
			buttons.every(b=>{const r=b.getBoundingClientRect();return r.left>=box.left-1 && r.right<=box.right+1 && r.width>=24;});
	}`)
	s.settle()
	s.shot("editor-phone")
	s.noRawKeys("phone editor")
	s.phoneKeyboardAdd()
	page.MustElement(`.ed-head [data-ed-cmd="home"]`).MustClick()
	s.wait(`()=>document.querySelectorAll('.ed-flow-card').length===1`)
	s.failOnPageErrors("phone")
}

// missionControl opens Mission Control with the published flow and an unpublished one, and opens
// the published flow in EasyDrag from there.
func (s *easyDragSmoke) missionControl() {
	s.t.Helper()
	page := s.page
	page.MustEval(`async()=>{edFixture.addDraft('Wetterwarnung');await fixtureOpen('mission-control');}`)
	s.wait(`()=>document.querySelectorAll('.vd-mc-row').length===2 && document.querySelectorAll('.vd-mc-row .vd-mc-row-badge--flow').length===2`)
	page.MustEval(`()=>{const id=edFixture.flow().rec.mission_id;document.querySelector('.vd-mc-row[data-mc-id="'+id+'"]').click();}`)
	s.wait(`()=>!!document.querySelector('.vd-mc [data-mc-action="openFlow"]') && document.querySelector('.vd-mc').textContent.includes('Noch nicht veröffentlicht')`)
	// The weekday schedule (0 7 * * 1-5) reads as such, in the row and in the detail.
	s.wait(`()=>{const row=document.querySelector('.vd-mc-row[data-mc-id="'+edFixture.flow().rec.mission_id+'"]');return row.textContent.includes('Mo–Fr um 07:00') && document.querySelector('.vd-mc-detail-body').textContent.includes('Mo–Fr um 07:00')}`)
	// Flows that never ran say so: the server's zero time ("0001-01-01T00:00:00Z") is no last run.
	s.wait(`()=>{const never=t('desktop.mc_state_never_run');const draft=[...document.querySelectorAll('.vd-mc-row')].find(r=>r.textContent.includes('Wetterwarnung'));return !!draft && draft.textContent.includes(never) && document.querySelector('.vd-mc-detail-body').textContent.includes(never)}`)
	s.shot("mission-control-flow")
	page.MustElement(`.vd-mc [data-mc-action="openFlow"]`).MustClick()
	s.wait(`()=>{const ed=edFixture.editor();return !!ed && ed.flow.id===edFixture.seededID && document.querySelectorAll('.ed-node').length===5}`)
}
