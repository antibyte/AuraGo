package ui

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/input"
)

// The Storymaker app browser tour (spec "Testing": one browser smoke tour of the app). It runs the real desktop shell with the
// real app modules, the real player iframe (host and player bundles under the exact player CSP) and the real catalog, against
// a handler for /api/desktop/storymaker. The handler here is storymakerFixture, a stand-in on top of the real internal/storymaker
// package. The same app runs against the real handlers (FFmpeg, Chrome, an export that ends in an MP4) in
// internal/server/storymaker_app_browser_test.go (package ui cannot build a configured Server). Gated by AURAGO_RUN_BROWSER_SMOKE=1:
//
//	AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui -run TestStorymakerApp -count=1 -timeout 10m

func storymakerWords(t *testing.T, lang string) map[string]string {
	t.Helper()
	words := map[string]string{}
	for _, dir := range []string{"storymaker", "storymaker_catalog"} {
		data, err := os.ReadFile(filepath.Join("lang", dir, lang+".json"))
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

// storymakerBrowserPage opens the desktop with the Storymaker app on the given API handler and returns the page.
func storymakerBrowserPage(t *testing.T, api http.Handler) *rod.Page {
	t.Helper()
	// The desktop harness gives its page 30 seconds; a tour of the whole app needs more.
	page := desktopAuditBrowserWithAPI(t, map[string]http.Handler{"/api/desktop/storymaker/": api}).CancelTimeout().Timeout(3 * time.Minute)
	page.MustEval(`(words)=>Object.assign(fixtureWords, words)`, storymakerWords(t, "en"))
	page.MustEval(`()=>auditTest.openApp('storymaker')`)
	return page
}

// storymakerWait waits for a condition in the page. One wait gets 90 seconds, so a condition that never comes true fails with a dump
// of the page instead of using up the budget of the whole tour (and the dump is read with a limit of its own).
func storymakerWait(t *testing.T, page *rod.Page, expr string) {
	t.Helper()
	if err := rod.Try(func() { page.Timeout(90 * time.Second).MustWait(expr) }); err != nil {
		var state string
		_ = rod.Try(func() {
			state = page.Timeout(10 * time.Second).MustEval(`()=>JSON.stringify({errors: window.fixtureErrors, scrubber: [...document.querySelectorAll('[data-stm="scrubber"]')].map(e=>[e.getAttribute('aria-valuenow'),e.getAttribute('aria-valuemax')]), app: (document.querySelector('[data-stm="app"]')||{}).innerText||''}).slice(0, 2500)`).String()
		})
		t.Fatalf("timed out waiting for %s: %v\n%s", expr, err, state)
	}
}

func storymakerText(page *rod.Page, selector string) string {
	return page.MustEval(`(s)=>{const e=document.querySelector(s);return e?e.textContent:'';}`, selector).String()
}

func storymakerClick(t *testing.T, page *rod.Page, selector string) {
	t.Helper()
	storymakerWait(t, page, `()=>!!document.querySelector(`+jsString(selector)+`)`)
	page.MustEval(`(s)=>document.querySelector(s).click()`, selector)
}

func jsString(s string) string {
	data, _ := json.Marshal(s)
	return string(data)
}

// storymakerEditorReplace swaps text in the open CodeMirror editor through the public EditorView API of the vendored bundle.
func storymakerEditorReplace(page *rod.Page, find, insert string) {
	page.MustEval(`async (find, insert) => {
		const cm = await import('/js/vendor/codemirror-bundle.esm.js');
		const view = cm.EditorView.findFromDOM(document.querySelector('[data-stm="code-editor"] .cm-editor'));
		const text = view.state.doc.toString(), at = text.indexOf(find);
		if (at < 0) throw new Error('not in the editor: ' + find);
		view.dispatch({changes: {from: at, to: at + find.length, insert}});
	}`, find, insert)
}

// storymakerShot writes a screenshot when AURAGO_BROWSER_ARTIFACT_DIR is set (a visual check by hand, not an assertion).
func storymakerShot(t *testing.T, page *rod.Page, name string) {
	t.Helper()
	dir := os.Getenv("AURAGO_BROWSER_ARTIFACT_DIR")
	if dir == "" {
		return
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name+".png"), page.MustScreenshot(), 0o644); err != nil {
		t.Fatal(err)
	}
}

func storymakerNoErrors(t *testing.T, page *rod.Page) {
	t.Helper()
	if got := page.MustEval(`()=>JSON.stringify(window.fixtureErrors)`).String(); got != "[]" {
		t.Fatalf("script errors in the page: %s", got)
	}
}

// storymakerBackend is what the tour needs from the thing behind /api/desktop/storymaker: the handler, a view of the stored projects
// and a way to move the render job along. storymakerFixture implements it.
type storymakerBackend interface {
	http.Handler
	projectIDs() []string
	storedStory(id string) (map[string]any, string)
	advance()
	watcherCount(id string) int
	joined() string
	body(key string) string
}

func TestStorymakerAppBrowser(t *testing.T) {
	storymakerTour(t, newStorymakerFixture(t))
}

// storymakerTour opens the app, creates a project, plays it, edits narration, fills a scene from the library, works in the code tab
// (an invalid value, a code scene with a syntax error), exports with a stepped job, leaves, deletes and closes the window.
func storymakerTour(t *testing.T, fixture storymakerBackend) {
	t.Helper()
	page := storymakerBrowserPage(t, fixture)

	// Start screen: no projects yet, nothing is missing, the capability banner stays away.
	storymakerWait(t, page, `()=>!!document.querySelector('.stm-projects') && !!document.querySelector('[data-stm="projects-empty"]') && !document.querySelector('[data-stm="projects-empty"]').hidden`)
	storymakerShot(t, page, "01-start")
	if page.MustEval(`()=>!!document.querySelector('[data-banner="export_unavailable"]')`).Bool() {
		t.Fatal("the export-unavailable banner shows although FFmpeg and a browser are ready")
	}

	// Create a project: the editor opens on one scene, the player compiles it and the strip shows a drawn thumbnail.
	page.MustElement(`[data-stm="new-title"]`).MustInput("Smoke test")
	page.MustElement(`[data-stm="aspect"][data-aspect="16:9"]`).MustClick()
	page.MustElement(`[data-stm="create-project"]`).MustClick()
	storymakerWait(t, page, `()=>document.querySelector('[data-stm="project-title"]')?.textContent==='Smoke test'`)
	storymakerWait(t, page, `()=>document.querySelectorAll('[data-stm="scene"]').length===1 && document.querySelector('[data-stm="clock-total"]').textContent!=='0:00.0'`)
	ids := fixture.projectIDs()
	if len(ids) != 1 {
		t.Fatalf("projects after create = %v", ids)
	}
	id := ids[0]
	frame := page.MustElement(`.stm-player`)
	if got := *frame.MustAttribute("sandbox"); got != "allow-scripts" {
		t.Fatalf("player sandbox = %q, want exactly allow-scripts", got)
	}
	if page.MustEval(`()=>!!document.querySelector('.stm-player').hasAttribute('allow')`).Bool() {
		t.Fatal("the player iframe must not delegate any permission")
	}
	storymakerWait(t, page, `()=>document.querySelector('canvas[data-scene="s1"]')?.dataset.drawn==='true'`)
	if !page.MustEval(`()=>{const c=document.querySelector('canvas[data-scene="s1"]'),d=c.getContext('2d').getImageData(0,0,c.width,c.height).data,seen=new Set();for(let i=0;i<d.length;i+=4*97)seen.add(d[i]+','+d[i+1]+','+d[i+2]);return seen.size>1;}`).Bool() {
		t.Fatal("the thumbnail of the first scene is a single colour: the preview rendered a blank frame")
	}
	storymakerShot(t, page, "02-editor")
	storymakerNoErrors(t, page)

	// A message that does not come from the iframe is dropped, even with a valid shape: the clock keeps its value.
	before := storymakerText(page, `[data-stm="clock-total"]`)
	page.MustEval(`()=>window.postMessage({sm:1,type:'compiled',report:{frame_count:99999,fps:30,scenes:[{id:'s1',start_frame:0,end_frame:99999,kind:'storyboard'}],issues:[]}},'*')`)
	time.Sleep(300 * time.Millisecond)
	if after := storymakerText(page, `[data-stm="clock-total"]`); after != before {
		t.Fatalf("a message from another window changed the timeline: %q -> %q", before, after)
	}

	// Play: the player host runs in real time, the clock moves and the button turns into pause; pause stops it again.
	page.MustEval(`()=>document.querySelector('[data-stm="play"]').click()`)
	storymakerWait(t, page, `()=>document.querySelector('[data-stm="play"]').dataset.playing==='true'`)
	storymakerWait(t, page, `()=>document.querySelector('[data-stm="clock"]').textContent!=='0:00.0'`)
	page.MustEval(`()=>document.querySelector('[data-stm="play"]').click()`)
	storymakerWait(t, page, `()=>document.querySelector('[data-stm="play"]').dataset.playing==='false'`)

	// Narration: typing saves with If-Match, the server keeps the text and the compiled length follows the text.
	page.MustElement(`[data-stm="narration"]`).MustInput("Anna works from home. Suddenly somebody knocks at the door and the whole office holds its breath for a moment.")
	storymakerWait(t, page, `()=>document.querySelector('[data-stm="save-state"]')?.dataset.state==='saved'`)
	story, _ := fixture.storedStory(id)
	scenes := story["scenes"].([]any)
	if narration, _ := scenes[0].(map[string]any)["narration"].(string); !strings.Contains(narration, "knocks at the door") {
		t.Fatalf("stored narration = %q", narration)
	}
	storymakerWait(t, page, `()=>document.querySelector('[data-stm="clock-total"]').textContent!==`+jsString(before))

	// Library: a person and a prop go into the selected scene; the environment follows the card.
	storymakerClick(t, page, `[data-stm="side-tab"][data-tab="library"]`)
	storymakerClick(t, page, `[data-stm="library-card"][data-id="char.office-woman-1"]`)
	storymakerWait(t, page, `()=>document.querySelectorAll('[data-stm="scene-items"] [data-kind="actor"]').length===1`)
	storymakerClick(t, page, `[data-stm="library-tab"][data-kind="prop"]`)
	storymakerClick(t, page, `[data-stm="library-card"][data-id="prop.laptop"]`)
	storymakerClick(t, page, `[data-stm="library-tab"][data-kind="environment"]`)
	storymakerClick(t, page, `[data-stm="library-card"][data-id="env.office"]`)
	storymakerWait(t, page, `()=>document.querySelector('[data-stm="library-card"][data-id="env.office"]')?.dataset.present==='true'`)
	storymakerWait(t, page, `()=>document.querySelector('[data-stm="save-state"]')?.dataset.state==='saved'`)
	story, _ = fixture.storedStory(id)
	scene := story["scenes"].([]any)[0].(map[string]any)
	if scene["environment"] != "env.office" || len(scene["actors"].([]any)) != 1 || len(scene["props"].([]any)) != 1 {
		t.Fatalf("stored scene after the library clicks = %v", scene)
	}
	storymakerShot(t, page, "03-library")
	storymakerClick(t, page, `[data-stm="side-tab"][data-tab="scene"]`)
	storymakerShot(t, page, "04-scene-settings")

	// Code tab: story.json in CodeMirror; an invalid value is saved, reported by the server and marked at its line.
	storymakerClick(t, page, `[data-stm="tab"][data-tab="code"]`)
	storymakerWait(t, page, `()=>!!document.querySelector('[data-stm="code-editor"] .cm-editor') && document.querySelector('[data-stm="code-editor"] .cm-content').textContent.includes('storymaker/1')`)
	storymakerEditorReplace(page, `"variant": "day"`, `"variant": "dusk"`)
	storymakerWait(t, page, `()=>document.querySelector('[data-stm="code-state"]')?.dataset.state==='dirty' && !document.querySelector('[data-stm="code-save"]').disabled`)
	storymakerClick(t, page, `[data-stm="code-save"]`)
	storymakerWait(t, page, `()=>document.querySelector('[data-stm="code-problems"]')?.dataset.count!=='0' && !!document.querySelector('[data-stm="code-editor"] .cm-lint-marker')`)
	if !strings.Contains(storymakerText(page, `[data-stm="code-problems"]`), "Invalid value") {
		t.Fatalf("the problem list does not name the issue: %q", storymakerText(page, `[data-stm="code-problems"]`))
	}
	storymakerShot(t, page, "05-code-issue")
	storymakerNoErrors(t, page)
	// Put it right again and save: the server's story comes back into the editor and the problem list is empty.
	storymakerEditorReplace(page, `"variant": "dusk"`, `"variant": "night"`)
	storymakerWait(t, page, `()=>document.querySelector('[data-stm="code-state"]')?.dataset.state==='dirty' && !document.querySelector('[data-stm="code-save"]').disabled`)
	storymakerClick(t, page, `[data-stm="code-save"]`)
	storymakerWait(t, page, `()=>document.querySelector('[data-stm="code-problems"]')?.dataset.count==='0' && document.querySelector('[data-stm="code-state"]')?.dataset.state==='saved'`)
	story, _ = fixture.storedStory(id)
	if story["scenes"].([]any)[0].(map[string]any)["variant"] != "night" {
		t.Fatalf("variant after the code save = %v", story["scenes"].([]any)[0])
	}
	storymakerClick(t, page, `[data-stm="tab"][data-tab="stage"]`)

	// A code scene: the starter program is saved as a new file (If-None-Match), bundled, and a mistake is shown at its line.
	storymakerClick(t, page, `[data-stm="add-code-scene"]`)
	storymakerWait(t, page, `()=>document.querySelector('[data-stm="code-file"]')?.textContent==='code2.js' && document.querySelector('[data-stm="code-editor"] .cm-content')?.textContent.includes('export default function scene')`)
	if !strings.Contains(fixture.body("PUT /projects/"+id+"/scenes/code2/code"), "api.duration(5)") {
		t.Fatalf("the starter code was not written: %q", fixture.body("PUT /projects/"+id+"/scenes/code2/code"))
	}
	storymakerEditorReplace(page, "api.duration(5);", "api.duration(5;")
	storymakerClick(t, page, `[data-stm="code-save"]`)
	storymakerWait(t, page, `()=>document.querySelector('[data-stm="code-problems"]')?.dataset.count!=='0' && document.querySelector('[data-stm="code-file-btn"][data-key="scene:code2"]')?.dataset.failed==='true'`)
	storymakerEditorReplace(page, "api.duration(5;", "api.duration(5);")
	storymakerClick(t, page, `[data-stm="code-save"]`)
	storymakerWait(t, page, `()=>document.querySelector('[data-stm="code-problems"]')?.dataset.count==='0' && document.querySelector('[data-stm="code-file-btn"][data-key="scene:code2"]')?.dataset.failed==='false'`)
	storymakerClick(t, page, `[data-stm="tab"][data-tab="stage"]`)
	storymakerWait(t, page, `()=>document.querySelectorAll('[data-stm="scene"]').length===2 && document.querySelector('[data-stm="scene"][data-scene="code2"] .stm-scene-kind')!==null`)

	// The preview reload that follows the last save of the code scene must be over first (it starts 250 ms after the save): a seek during
	// it is clamped to the report of that moment, and the playhead would not be at the end of the new one.
	time.Sleep(600 * time.Millisecond)
	storymakerWait(t, page, `()=>document.querySelector('[data-stm="stage-note"]').hidden && document.querySelector('[data-stm="save-state"]')?.dataset.state==='saved'`)

	// Scrubber: Home goes to the start, a click lands in the middle, End goes to the last frame.
	page.MustEval(`()=>document.querySelector('[data-stm="scrubber"]').focus()`)
	if err := page.Keyboard.Type(input.Home); err != nil {
		t.Fatal(err)
	}
	storymakerWait(t, page, `()=>document.querySelector('[data-stm="scrubber"]').getAttribute('aria-valuenow')==='0'`)
	page.MustElement(`[data-stm="scrubber"]`).MustClick()
	storymakerWait(t, page, `()=>Number(document.querySelector('[data-stm="scrubber"]').getAttribute('aria-valuenow'))>0`)
	page.MustEval(`()=>document.querySelector('[data-stm="scrubber"]').focus()`)
	if err := page.Keyboard.Type(input.End); err != nil {
		t.Fatal(err)
	}
	storymakerWait(t, page, `()=>{const s=document.querySelector('[data-stm="scrubber"]');return s.getAttribute('aria-valuenow')===s.getAttribute('aria-valuemax')&&s.getAttribute('aria-valuemax')!=='0'}`)

	// Inspector: the transition and the duration of the selected (code) scene, then the scene moves up and down.
	const setSelect = `(sel,value)=>{const s=document.querySelector(sel);s.value=value;s.dispatchEvent(new Event('change',{bubbles:true}));}`
	page.MustEval(setSelect, `[data-stm="transition"]`, "wipe")
	page.MustEval(setSelect, `[data-stm="duration-mode"]`, "fixed")
	page.MustEval(`()=>{const i=document.querySelector('[data-stm="duration-seconds"]');i.value='3.5';i.dispatchEvent(new Event('change',{bubbles:true}));}`)
	storymakerWait(t, page, `()=>document.querySelector('[data-stm="save-state"]')?.dataset.state==='saved'`)
	story, _ = fixture.storedStory(id)
	second := story["scenes"].([]any)[1].(map[string]any)
	if second["transition"] != "wipe" || second["duration"] != 3.5 {
		t.Fatalf("stored transition and duration = %v, %v", second["transition"], second["duration"])
	}
	storymakerClick(t, page, `[data-stm="scene-up"]`)
	storymakerWait(t, page, `()=>document.querySelector('[data-stm="scene"][data-index="0"]')?.dataset.scene==='code2' && document.querySelector('[data-stm="save-state"]')?.dataset.state==='saved'`)
	story, _ = fixture.storedStory(id)
	if first := story["scenes"].([]any)[0].(map[string]any)["id"]; first != "code2" {
		t.Fatalf("the first stored scene after the move = %v", first)
	}
	storymakerClick(t, page, `[data-stm="scene-down"]`)
	storymakerWait(t, page, `()=>document.querySelector('[data-stm="scene"][data-index="1"]')?.dataset.scene==='code2' && document.querySelector('[data-stm="save-state"]')?.dataset.state==='saved'`)

	// A seek that comes while the preview compiles after an edit is kept. The End key is pressed in the very task that shows the
	// "preparing" note, so it lands in the loading phase every time; the host drops or misorders seeks then, and the playhead used to
	// fall back to frame 0 (or to the frame of the edit), which made the tour fail by chance.
	storymakerWait(t, page, `()=>document.querySelector('[data-stm="stage-note"]').hidden && document.querySelector('[data-stm="save-state"]')?.dataset.state==='saved'`)
	page.MustEval(`()=>document.querySelector('[data-stm="scrubber"]').focus()`)
	if err := page.Keyboard.Type(input.Home); err != nil {
		t.Fatal(err)
	}
	storymakerWait(t, page, `()=>document.querySelector('[data-stm="scrubber"]').getAttribute('aria-valuenow')==='0'`)
	page.MustEval(`()=>{
		window.stmHostFrames = [];
		window.addEventListener('message', event => { const m = event.data; if (m && m.sm === 1 && m.type === 'time') window.stmHostFrames.push(m.frame); });
		const note = document.querySelector('[data-stm="stage-note"]');
		window.stmSeekWhileCompiling = new Promise(resolve => {
			const watch = new MutationObserver(() => {
				if (note.hidden) return;
				watch.disconnect();
				document.querySelector('[data-stm="scrubber"]').dispatchEvent(new KeyboardEvent('keydown', {key: 'End', bubbles: true, cancelable: true}));
				resolve(true);
			});
			watch.observe(note, {attributes: true, attributeFilter: ['hidden']});
		});
	}`)
	page.MustEval(setSelect, `[data-stm="transition"]`, "fade")
	page.MustEval(`()=>window.stmSeekWhileCompiling`)
	storymakerWait(t, page, `()=>document.querySelector('[data-stm="stage-note"]').hidden && (()=>{const s=document.querySelector('[data-stm="scrubber"]');return s.getAttribute('aria-valuenow')===s.getAttribute('aria-valuemax')&&s.getAttribute('aria-valuemax')!=='0'})()`)
	storymakerWait(t, page, `()=>{const s=document.querySelector('[data-stm="scrubber"]'),f=window.stmHostFrames;return f.length>0&&String(f[f.length-1])===s.getAttribute('aria-valuemax')}`)
	if !page.MustEval(`()=>{const s=document.querySelector('[data-stm="scrubber"]');return s.getAttribute('aria-valuenow')===s.getAttribute('aria-valuemax')}`).Bool() {
		t.Fatal("the playhead left the end after the preview reloaded")
	}
	// and the host shows that frame too: its last time message is the end, not the frame 0 it starts at after a load
	if !page.MustEval(`()=>{const s=document.querySelector('[data-stm="scrubber"]'),f=window.stmHostFrames;return f.length>0&&String(f[f.length-1])===s.getAttribute('aria-valuemax')}`).Bool() {
		t.Fatalf("the host stayed on another frame than the playhead: %v", page.MustEval(`()=>JSON.stringify(window.stmHostFrames)`).String())
	}

	// Export: the panel steps through the fake job and ends in the output path with "show in Files".
	storymakerClick(t, page, `[data-stm="export"]`)
	storymakerWait(t, page, `()=>!!document.querySelector('[data-stm="export-start"]') && !document.querySelector('[data-stm="export-start"]').disabled`)
	if !page.MustEval(`()=>document.querySelectorAll('[data-stm="export-caps"] [data-ok="true"]').length===2`).Bool() {
		t.Fatal("FFmpeg and the browser should both be listed as ready")
	}
	storymakerClick(t, page, `[data-stm="export-start"]`)
	storymakerClick(t, page, `[data-stm="export-confirm"]`)
	storymakerWait(t, page, `()=>!!document.querySelector('[data-stm="export-progress"]')`)
	if fixture.watcherCount(id) == 0 {
		t.Fatal("the app should follow the job on the project's event stream")
	}
	if !strings.Contains(fixture.joined(), "POST /projects/"+id+"/render") {
		t.Fatalf("no render request: %s", fixture.joined())
	}
	fixture.advance()
	fixture.advance()
	storymakerShot(t, page, "06-export-progress")
	storymakerWait(t, page, `()=>document.querySelector('.stm-export-label')?.textContent.includes('50') && document.querySelector('progress.stm-progress')?.value===0.5`)
	fixture.advance()
	fixture.advance()
	storymakerWait(t, page, `()=>!!document.querySelector('[data-stm="export-path"]')`)
	if path := storymakerText(page, `[data-stm="export-path"]`); !strings.HasPrefix(path, "Documents/Storymaker/"+id+"/exports/Smoke-test-") || !strings.HasSuffix(path, ".mp4") {
		t.Fatalf("export path = %q", path)
	}
	storymakerShot(t, page, "07-export-done")
	windows := page.MustEval(`()=>auditTest.state.windows.size`).Int()
	page.MustEval(`()=>document.querySelector('[data-stm="export-show"]').click()`)
	storymakerWait(t, page, fmt.Sprintf(`()=>auditTest.state.windows.size===%d`, windows+1))
	storymakerNoErrors(t, page)

	// Leave the project: the player iframe goes with it, the list shows the project and reopening restores the story.
	storymakerClick(t, page, `[data-stm="export-close"]`)
	storymakerClick(t, page, `[data-stm="back"]`)
	storymakerWait(t, page, `()=>document.querySelectorAll('[data-stm="open-project"]').length===1 && !document.querySelector('.stm-player')`)
	storymakerClick(t, page, `[data-stm="open-project"]`)
	storymakerWait(t, page, `()=>document.querySelector('[data-stm="narration"]')?.value.includes('knocks at the door') && document.querySelectorAll('[data-stm="scene"]').length===2`)

	// The way back to a finished export: the reopened panel offers the newest one, also one that ended while the project was closed.
	storymakerClick(t, page, `[data-stm="export"]`)
	storymakerClick(t, page, `[data-stm="export-last"]`)
	storymakerWait(t, page, `()=>!!document.querySelector('[data-stm="export-path"]')`)
	firstPath := storymakerText(page, `[data-stm="export-path"]`)
	storymakerClick(t, page, `[data-stm="export-again"]`)
	storymakerWait(t, page, `()=>!!document.querySelector('[data-stm="export-start"]') && !document.querySelector('[data-stm="export-start"]').disabled`)
	storymakerClick(t, page, `[data-stm="export-start"]`)
	storymakerClick(t, page, `[data-stm="export-confirm"]`)
	storymakerWait(t, page, `()=>!!document.querySelector('[data-stm="export-progress"]')`)
	storymakerClick(t, page, `[data-stm="export-close"]`)
	storymakerClick(t, page, `[data-stm="back"]`)
	storymakerWait(t, page, `()=>document.querySelectorAll('[data-stm="open-project"]').length===1 && !document.querySelector('.stm-player')`)
	for i := 0; i < 4; i++ {
		fixture.advance()
	}
	storymakerClick(t, page, `[data-stm="open-project"]`)
	storymakerWait(t, page, `()=>document.querySelector('[data-stm="narration"]')?.value.includes('knocks at the door') && document.querySelectorAll('[data-stm="scene"]').length===2`)
	storymakerClick(t, page, `[data-stm="export"]`)
	storymakerClick(t, page, `[data-stm="export-last"]`)
	storymakerWait(t, page, `()=>!!document.querySelector('[data-stm="export-path"]')`)
	if path := storymakerText(page, `[data-stm="export-path"]`); path == firstPath || !strings.HasSuffix(path, ".mp4") {
		t.Fatalf("the way back shows %q, the first export was %q", path, firstPath)
	}
	storymakerNoErrors(t, page)
	storymakerClick(t, page, `[data-stm="export-close"]`)

	// Delete asks the desktop dialog first: refusing keeps the project, confirming removes the folder.
	storymakerClick(t, page, `[data-stm="back"]`)
	storymakerClick(t, page, `[data-stm="delete-project"]`)
	storymakerWait(t, page, `()=>!!document.querySelector('.vd-modal [data-cancel]')`)
	page.MustEval(`()=>document.querySelector('.vd-modal [data-cancel]').click()`)
	storymakerWait(t, page, `()=>!document.querySelector('.vd-modal')`)
	if len(fixture.projectIDs()) != 1 {
		t.Fatal("cancelling the dialog must keep the project")
	}
	storymakerClick(t, page, `[data-stm="delete-project"]`)
	storymakerWait(t, page, `()=>!!document.querySelector('.vd-modal [type="submit"]')`)
	page.MustEval(`()=>document.querySelector('.vd-modal [type="submit"]').click()`)
	storymakerWait(t, page, `()=>document.querySelectorAll('[data-stm="open-project"]').length===0 && !document.querySelector('[data-stm="projects-empty"]').hidden`)
	if len(fixture.projectIDs()) != 0 {
		t.Fatalf("projects after the delete = %v", fixture.projectIDs())
	}

	// Closing the window tears the app down without a script error.
	page.MustEval(`()=>auditTest.closeWindow([...auditTest.state.windows.values()].find(w=>w.appId==='storymaker').id)`)
	storymakerWait(t, page, `()=>!document.querySelector('[data-stm="app"]')`)
	storymakerNoErrors(t, page)
}

func TestStorymakerAppStatesBrowser(t *testing.T) {
	t.Run("no ffmpeg and no browser", func(t *testing.T) {
		fixture := newStorymakerFixture(t)
		fixture.set(func() { fixture.ffmpegReady, fixture.browserReady = false, false })
		page := storymakerBrowserPage(t, fixture)
		storymakerWait(t, page, `()=>!!document.querySelector('[data-banner="export_unavailable"]')`)
		page.MustElement(`[data-stm="new-title"]`).MustInput("Offline")
		page.MustElement(`[data-stm="create-project"]`).MustClick()
		storymakerWait(t, page, `()=>!!document.querySelector('[data-stm="export"]')`)
		storymakerClick(t, page, `[data-stm="export"]`)
		storymakerWait(t, page, `()=>!!document.querySelector('[data-stm="export-blockers"]')`)
		text := storymakerText(page, `[data-stm="export-blockers"]`)
		if !strings.Contains(text, "FFmpeg is missing") || !strings.Contains(text, "No browser is available") {
			t.Fatalf("blockers = %q", text)
		}
		if !page.MustEval(`()=>document.querySelector('[data-stm="export-start"]').disabled`).Bool() {
			t.Fatal("the start button must be disabled")
		}
		if !strings.Contains(storymakerText(page, `[data-stm="export-caps"]`), "ffmpeg: executable file not found") {
			t.Fatal("the capability list should show the reason the status gave")
		}
		// the editor itself still works
		if page.MustEval(`()=>document.querySelector('[data-stm="narration"]').disabled`).Bool() {
			t.Fatal("editing must stay possible without FFmpeg")
		}
		storymakerNoErrors(t, page)
	})

	t.Run("read-only", func(t *testing.T) {
		fixture := newStorymakerFixture(t)
		page := storymakerBrowserPage(t, fixture)
		storymakerWait(t, page, `()=>!!document.querySelector('.stm-projects')`)
		page.MustElement(`[data-stm="new-title"]`).MustInput("Before")
		page.MustElement(`[data-stm="create-project"]`).MustClick()
		storymakerWait(t, page, `()=>!!document.querySelector('[data-stm="narration"]')`)
		fixture.set(func() { fixture.readOnly = true })
		// the desktop policy flips while the window is open
		page.MustEval(`()=>document.dispatchEvent(new CustomEvent('aurago:desktop-policy',{detail:{readonly:true}}))`)
		storymakerWait(t, page, `()=>!!document.querySelector('[data-banner="readonly"]') && document.querySelector('[data-stm="narration"]').disabled && document.querySelector('[data-stm="add-scene"]').disabled`)
		storymakerClick(t, page, `[data-stm="export"]`)
		storymakerWait(t, page, `()=>document.querySelector('[data-stm="export-blockers"] [data-reason="readonly"]')!==null && document.querySelector('[data-stm="export-start"]').disabled`)
		storymakerNoErrors(t, page)
	})

	t.Run("switched off", func(t *testing.T) {
		fixture := newStorymakerFixture(t)
		fixture.set(func() { fixture.disabled = true })
		page := storymakerBrowserPage(t, fixture)
		storymakerWait(t, page, `()=>document.querySelector('[data-stm="state"]')?.dataset.state==='storymaker.state.disabled_title'`)
		if !strings.Contains(storymakerText(page, `[data-stm="state"]`), "Storymaker is switched off") {
			t.Fatalf("state card = %q", storymakerText(page, `[data-stm="state"]`))
		}
		storymakerNoErrors(t, page)
	})

	t.Run("signed out", func(t *testing.T) {
		fixture := newStorymakerFixture(t)
		fixture.set(func() { fixture.unauthorized = true })
		page := storymakerBrowserPage(t, fixture)
		storymakerWait(t, page, `()=>document.querySelector('[data-stm="state"]')?.dataset.state==='storymaker.state.unauthorized_title'`)
		storymakerNoErrors(t, page)
	})

	t.Run("conflict", func(t *testing.T) {
		fixture := newStorymakerFixture(t)
		page := storymakerBrowserPage(t, fixture)
		storymakerWait(t, page, `()=>!!document.querySelector('.stm-projects')`)
		page.MustElement(`[data-stm="new-title"]`).MustInput("Conflict")
		page.MustElement(`[data-stm="create-project"]`).MustClick()
		storymakerWait(t, page, `()=>!!document.querySelector('[data-stm="narration"]')`)
		id := fixture.projectIDs()[0]
		// somebody else saves the story first
		story, etag := fixture.storedStory(id)
		story["title"] = "Changed elsewhere"
		data, _ := json.Marshal(story)
		if _, err := fixture.projects.SaveStory(t.Context(), id, data, etag); err != nil {
			t.Fatal(err)
		}
		page.MustElement(`[data-stm="narration"]`).MustInput("My text")
		storymakerWait(t, page, `()=>!!document.querySelector('[data-banner="conflict"]')`)
		storymakerClick(t, page, `[data-stm="conflict-overwrite"]`)
		storymakerWait(t, page, `()=>!document.querySelector('[data-banner="conflict"]') && document.querySelector('[data-stm="save-state"]')?.dataset.state==='saved'`)
		stored, _ := fixture.storedStory(id)
		if narration, _ := stored["scenes"].([]any)[0].(map[string]any)["narration"].(string); narration != "My text" {
			t.Fatalf("stored narration after overwrite = %q", narration)
		}
		storymakerNoErrors(t, page)
	})
}
