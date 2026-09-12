package ui

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"aurago/internal/desktop"
	"github.com/go-rod/rod/lib/input"
	"github.com/go-rod/rod/lib/proto"
)

// Opt-in headless smoke test: boots the real desktop bundle, opens the
// Calendar app and creates an appointment with genuine mouse clicks. It
// guards the click delegation in wireCalendarShell: a day cell, the toolbar
// button and a time slot must open the editor, the editor must be clickable
// on top of the window, and the view tabs must keep switching views.
func TestDesktopCalendarCreateAppointmentBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	html := regexp.MustCompile(`(?s)<script\b[^>]*>.*?</script>`).ReplaceAllString(readDesktopAssetText(t, "desktop.html"), "")
	html = regexp.MustCompile(`\{\{[^}]*\}\}`).ReplaceAllString(html, "")
	html = strings.Replace(html, "</body>", `<script>
window.fixtureErrors=[];
addEventListener('error', e=>fixtureErrors.push(e.message));
addEventListener('unhandledrejection', e=>fixtureErrors.push(String(e.reason)));
window.t=key=>key;
window.WebSocket=class extends EventTarget { close(){} };
</script><script src="/js/shared/lazy-assets.js"></script><script src="/js/desktop/core/module-loader.js"></script><script src="/session-shell.js"></script></body>`, 1)
	shell := readDesktopAssetText(t, "js/desktop/bundles/main.bundle.js")
	cut := strings.LastIndex(shell, "    ensureDesktopRadialMenuAnchor();")
	if cut < 0 {
		t.Fatal("desktop startup seam missing")
	}
	shell = shell[:cut] + `window.sessionTest={state,openApp};` + shell[cut:]
	settings := desktop.DesktopSettingDefaults()
	for key, value := range map[string]string{"windows.restore_session": "false", "windows.animations": "false", "pet.enabled": "false", "phone_gadget.enabled": "false", "desktop.show_widgets": "false"} {
		settings[key] = value
	}
	var mu sync.Mutex
	var created []map[string]interface{}
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(Content)))
	mux.HandleFunc("/fixture", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, html) })
	mux.HandleFunc("/session-shell.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/javascript")
		fmt.Fprint(w, shell)
	})
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/api/desktop/bootstrap":
			json.NewEncoder(w).Encode(map[string]interface{}{"enabled": true, "builtin_apps": desktop.BuiltinApps(), "installed_apps": []interface{}{}, "widgets": []interface{}{}, "shortcuts": []interface{}{}, "desktop_files": []interface{}{}, "workspace": map[string]interface{}{"readonly": false}, "settings": settings})
		case r.URL.Path == "/api/desktop/settings":
			json.NewEncoder(w).Encode(map[string]interface{}{"settings": settings})
		case r.URL.Path == "/api/appointments" && r.Method == http.MethodPost:
			body, _ := io.ReadAll(r.Body)
			var payload map[string]interface{}
			_ = json.Unmarshal(body, &payload)
			mu.Lock()
			created = append(created, payload)
			id := fmt.Sprintf("appt-%d", len(created))
			mu.Unlock()
			payload["id"] = id
			payload["status"] = "upcoming"
			json.NewEncoder(w).Encode(payload)
		case strings.HasPrefix(r.URL.Path, "/api/appointments"):
			mu.Lock()
			list := make([]map[string]interface{}, 0, len(created))
			for index, item := range created {
				copy := map[string]interface{}{"id": fmt.Sprintf("appt-%d", index+1), "status": "upcoming"}
				for key, value := range item {
					copy[key] = value
				}
				list = append(list, copy)
			}
			mu.Unlock()
			json.NewEncoder(w).Encode(list)
		case r.URL.Path == "/api/contacts":
			fmt.Fprint(w, `[]`)
		default:
			fmt.Fprint(w, `{"status":"ok","files":[],"pets":[],"settings":{},"enabled":false}`)
		}
	})
	server := httptest.NewServer(mux)
	defer server.Close()
	page := newSmokeBrowser(t).MustPage().Timeout(60 * time.Second)
	defer page.Close()
	page.MustSetViewport(1440, 1000, 1, false)
	page.MustNavigate(server.URL + "/fixture")
	page.MustWaitLoad()
	page.MustWait(`()=>!!(window.sessionTest?.state.ws || window.fixtureErrors?.length)`)
	assertNoErrors := func(step string) {
		t.Helper()
		if errors := page.MustEval(`()=>JSON.stringify(fixtureErrors)`).Str(); errors != "[]" {
			t.Fatalf("%s: %s", step, errors)
		}
	}
	assertNoErrors("boot")
	page.MustEval(`async()=>{ await AuraDesktopModules.loadAppAssets('calendar'); sessionTest.openApp('calendar'); }`)
	page.MustWait(`()=>!!document.querySelector('.vd-calendar-shell .vd-calendar-cell[data-cal-date]') && !document.querySelector('.vd-calendar-shell').classList.contains('is-loading')`)
	assertNoErrors("open calendar")

	// Real mouse clicks: the element under the cursor must be the intended
	// target and the click must reach the calendar's delegated handler.
	clickCenter := func(selector string) {
		t.Helper()
		el := page.MustElement(selector)
		point := el.MustEval(`function(){const r=this.getBoundingClientRect();const hit=document.elementFromPoint(r.x+r.width/2,r.y+r.height/2);if(!this.contains(hit) && !(hit && hit.contains(this)))throw Error('target is covered by '+(hit?hit.tagName+'.'+hit.className:'nothing'));return [r.x+r.width/2,r.y+r.height/2];}`).Arr()
		page.Mouse.MustMoveTo(point[0].Num(), point[1].Num()).MustClick(proto.InputMouseButtonLeft)
	}
	editorOpen := `()=>{const bd=document.querySelector('.vd-calendar-editor-backdrop');if(!bd)return false;const title=bd.querySelector('form').elements.title;const r=title.getBoundingClientRect();return r.width>0 && title.contains(document.elementFromPoint(r.x+r.width/2,r.y+r.height/2));}`
	titleFocused := `()=>{const bd=document.querySelector('.vd-calendar-editor-backdrop');return !!bd && document.activeElement===bd.querySelector('form').elements.title;}`

	// 1. A month cell opens the editor for that day.
	cell := page.MustElement(`.vd-calendar-shell .vd-calendar-cell[data-cal-date]:nth-child(10)`)
	cellDate := cell.MustAttribute("data-cal-date")
	clickCenter(`.vd-calendar-shell .vd-calendar-cell[data-cal-date]:nth-child(10)`)
	page.MustWait(editorOpen)
	if got := page.MustEval(`()=>document.querySelector('.vd-calendar-editor-backdrop form').elements.date.value`).Str(); got != *cellDate {
		t.Fatalf("editor date %q does not match the clicked cell %q", got, *cellDate)
	}
	if !page.MustEval(`()=>document.querySelector('.vd-calendar-shell').dataset.calMode==='month' && !document.querySelector('.vd-calendar-shell').classList.contains('is-active')`).Bool() {
		t.Fatal("clicking a cell must not be treated as a view-tab click")
	}
	// Optional fields honour the hidden attribute despite flex/grid classes
	// and appear once their option is chosen.
	if !page.MustEval(`()=>{
		const form=document.querySelector('.vd-calendar-editor-backdrop form');
		const visible=el=>el.getClientRects().length>0;
		const custom=form.querySelector('[data-reminder-custom]'), count=form.querySelector('[data-repeat-count]'), preview=form.querySelector('[data-repeat-preview]');
		if (visible(custom) || visible(count) || visible(preview)) return false;
		form.elements.repeat.value='weekly';
		form.elements.repeat.dispatchEvent(new Event('change',{bubbles:true}));
		const shown=visible(count) && visible(preview) && preview.textContent.length>0;
		form.elements.repeat.value='none';
		form.elements.repeat.dispatchEvent(new Event('change',{bubbles:true}));
		return shown && !visible(count) && !visible(preview);
	}`).Bool() {
		t.Fatal("optional editor fields must stay hidden until their option is selected")
	}
	if dir := os.Getenv("AURAGO_BROWSER_ARTIFACT_DIR"); dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "calendar-editor.png"), page.MustScreenshot(), 0644); err != nil {
			t.Fatal(err)
		}
	}

	// 2. Typing a title and clicking Create posts the appointment and closes the editor.
	page.MustElement(`.vd-calendar-editor-backdrop input[name=title]`).MustInput("Zahnarzt")
	clickCenter(`.vd-calendar-editor-backdrop [data-submit]`)
	page.MustWait(`()=>!document.querySelector('.vd-calendar-editor-backdrop')`)
	mu.Lock()
	count := len(created)
	var first map[string]interface{}
	if count > 0 {
		first = created[0]
	}
	mu.Unlock()
	if count != 1 || first["title"] != "Zahnarzt" || !strings.HasPrefix(fmt.Sprint(first["date_time"]), *cellDate) {
		t.Fatalf("expected one created appointment for %s, got %d: %v", *cellDate, count, first)
	}
	page.MustWait(`()=>!!document.querySelector('.vd-calendar-shell .vd-calendar-event[data-appt-id="appt-1"]')`)
	assertNoErrors("create appointment")

	// 3. The toolbar button opens a fresh editor that focuses the title;
	// Escape closes the modal even when focus is not inside the form.
	clickCenter(`.vd-calendar-shell [data-cal-create]`)
	page.MustWait(editorOpen)
	page.MustWait(titleFocused)
	page.MustEval(`()=>document.activeElement.blur()`)
	page.Keyboard.MustType(input.Escape)
	page.MustWait(`()=>!document.querySelector('.vd-calendar-editor-backdrop')`)

	// 4. View tabs still switch views and a time slot opens the editor too.
	clickCenter(`.vd-calendar-shell [data-cal-view="week"]`)
	page.MustWait(`()=>document.querySelector('.vd-calendar-shell').dataset.calMode==='week' && !!document.querySelector('.vd-calendar-shell .vd-calendar-hour-slot')`)
	page.MustEval(`()=>document.querySelector('.vd-calendar-shell [data-cal-column] .vd-calendar-hour-slot[data-cal-hour="10"]').scrollIntoView({block:'center'})`)
	clickCenter(`.vd-calendar-shell [data-cal-column] .vd-calendar-hour-slot[data-cal-hour="10"]`)
	page.MustWait(editorOpen)
	page.MustWait(titleFocused)
	if got := page.MustEval(`()=>document.querySelector('.vd-calendar-editor-backdrop form').elements.time.value`).Str(); !strings.HasPrefix(got, "10:") {
		t.Fatalf("time slot click proposed %q instead of a 10 o'clock start", got)
	}
	page.Keyboard.MustType(input.Escape)
	page.MustWait(`()=>!document.querySelector('.vd-calendar-editor-backdrop')`)
	assertNoErrors("week view")
}
