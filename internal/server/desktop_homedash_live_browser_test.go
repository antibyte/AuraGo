package server

import (
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"net/http/httptest"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"aurago/internal/homedash/sources/hass/hasstest"
	"aurago/internal/homedash/sourcetest"
	"aurago/internal/i18n"
	"aurago/ui"

	"github.com/go-rod/rod/lib/input"
)

// TestDesktopHomeDashLiveHABrowser (D13, M2's D9): the desktop shell with the real HomeDash handlers (handleHomeDash) and hasstest as Home
// Assistant. It opens HomeDash, creates the home_assistant dashboard, switches the coffee machine on and dims the kitchen light by keyboard,
// then checks the fake's service calls and that the UI confirmed (no pending, no error, values from the state frames). Needs
// AURAGO_RUN_BROWSER_SMOKE=1 and Chrome.
func TestDesktopHomeDashLiveHABrowser(t *testing.T) {
	browser := personalRadioBrowser(t)
	shortenHomeDashStreamTimers(t)
	demo := hasstest.DemoOptions()
	ha := hasstest.New(t, demo)
	env := newHomeDashHAEnv(t, ha)
	sourcetest.WaitFor(t, 10*time.Second, func() bool {
		var boot struct {
			Templates []string `json:"templates"`
		}
		hdDecode(t, env.do(t, env.admin, http.MethodGet, "bootstrap", nil), &boot)
		return slices.Contains(boot.Templates, "home_assistant")
	}, "bootstrap lists home_assistant")

	// The kitchen light starts at the demo's brightness (0..255) shown as a percentage; one ArrowRight is one step of the slider's range.
	var initial float64
	for _, st := range demo.States {
		if st.EntityID == "light.kitchen" {
			initial = math.Round(st.Attributes["brightness"].(float64) / 255 * 100)
		}
	}
	const keys = 2

	i18n.Load(ui.Content, slog.Default())
	read := func(p string) string {
		b, err := ui.Content.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	html := regexp.MustCompile(`(?s)<script\b[^>]*>.*?</script>`).ReplaceAllString(read("desktop.html"), "")
	html = regexp.MustCompile(`\{\{[^}]*\}\}`).ReplaceAllString(html, "")
	html = strings.Replace(html, "</body>", `<script type="application/json" id="aurago-template-data">__DATA__</script><script src="/js/shared/template-data.js"></script><script>window._auragoSharedInitialized=true;window.hdErrors=[];addEventListener('error',e=>hdErrors.push(e.message));addEventListener('unhandledrejection',e=>hdErrors.push(String(e.reason)));</script><script src="/js/shared/shared-core.js"></script><script src="/js/shared/lazy-assets.js"></script><script src="/js/desktop/core/module-loader.js"></script><script src="/hd-shell.js"></script></body>`, 1)
	shell := read("js/desktop/bundles/main.bundle.js")
	cut := strings.LastIndex(shell, "    ensureDesktopRadialMenuAnchor();")
	if cut < 0 {
		t.Fatal("shell seam missing")
	}
	shell = shell[:cut] + `window.hdTest={state,openApp};})();`
	api := handleHomeDash(env.s)
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(ui.Content)))
	mux.HandleFunc("/api/i18n", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"data":%s}`, getI18NJSONForSections(normalizeLang(r.URL.Query().Get("lang")), strings.Split(r.URL.Query().Get("sections"), ",")...))
	})
	mux.Handle(homeDashAPIPrefix, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Header.Set("Authorization", "Bearer "+env.admin)
		api.ServeHTTP(w, r)
	}))
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{}`)
	})
	mux.HandleFunc("/fixture", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, strings.Replace(html, "__DATA__", fmt.Sprint(uiTemplateData("de", "desktop")["TemplateDataJSON"]), 1))
	})
	mux.HandleFunc("/hd-shell.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/javascript")
		fmt.Fprint(w, shell)
	})
	origin := httptest.NewServer(mux)
	// Closed after the page (defers run last in, first out) so no stream handler outlives the server's wait.
	defer origin.Close()
	page := browser.MustPage().Timeout(2 * time.Minute)
	defer page.Close()
	page.MustSetViewport(1280, 900, 1, false)
	page.MustNavigate(origin.URL + "/fixture").MustWaitLoad()
	page.MustEval(`()=>{hdTest.state.bootstrap={enabled:true,builtin_apps:[{id:'homedash',name:'HomeDash',icon:'homedash'}],apps:[],widgets:[],shortcuts:[],desktop_files:[],settings:{'appearance.theme':'standard','windows.restore_session':false,'windows.animations':'false'}};document.body.dataset.animations='false';const d=document.getElementById('vd-disabled');if(d)d.hidden=true;hdTest.openApp('homedash');}`)
	// diag is only evaluated when a wait has failed.
	diag := func() string {
		return page.MustEval(`()=>JSON.stringify({errors:window.hdErrors,mode:(document.querySelector('.hdx-home')||{dataset:{}}).dataset.hdxMode,ha:[...document.querySelectorAll('[data-action=create-ha]')].map(b=>b.hidden+'/'+!!b.closest('[hidden]')),notice:(document.querySelector('.hdx-notice')||{}).textContent,conn:(document.querySelector('.hdx-view .hd-root')||{dataset:{}}).dataset.hdConn,widgets:[...document.querySelectorAll('.hdx-view .hd-widget')].map(x=>({wid:x.dataset.hdWid,status:x.dataset.hdStatus,pending:x.hasAttribute('data-hd-pending'),text:(x.querySelector('.hd-state-text')||{}).textContent})),window:(document.querySelector('.vd-window[data-app-id="homedash"]')||{textContent:'no window'}).textContent.replace(/\s+/g,' ').slice(0,300)})`).Str()
	}
	until := func(within time.Duration, what, js string, args ...any) {
		t.Helper()
		deadline := time.Now().Add(within)
		for !page.MustEval(js, args...).Bool() {
			if time.Now().After(deadline) {
				t.Fatalf("timed out after %v waiting for %s\n%s", within, what, diag())
			}
			time.Sleep(25 * time.Millisecond)
		}
	}
	waitCall := func(domain, service string, match func(map[string]any) bool) {
		t.Helper()
		deadline := time.Now().Add(10 * time.Second)
		for {
			for _, c := range ha.Calls() {
				if c.Domain == domain && c.Service == service && match(c.Data) {
					return
				}
			}
			if time.Now().After(deadline) {
				t.Fatalf("no %s.%s call at the fake Home Assistant: %+v\n%s", domain, service, ha.Calls(), diag())
			}
			time.Sleep(25 * time.Millisecond)
		}
	}

	until(30*time.Second, "the home screen with the Home Assistant button", `()=>!!document.querySelector('.hdx-home .hdx-choice[data-action="create-ha"]:not([hidden])')&&!document.querySelector('.hdx-home .hdx-choice[data-action="create-ha"]').closest('[hidden]')`)
	page.MustEval(`()=>document.querySelector('.hdx-home .hdx-choice[data-action="create-ha"]').click()`)
	until(20*time.Second, "the three pages and the coffee switch", `()=>document.querySelectorAll('.hdx-view .hd-tab').length===3&&!!document.querySelector('[data-hd-wid="w2"] [role="switch"]')`)
	until(20*time.Second, "a live stream and the first values", `()=>{const r=document.querySelector('.hdx-view .hd-root'),w=document.querySelector('[data-hd-wid="w2"]'),l=document.querySelector('[data-hd-wid="w1"] .hd-slider');
		return !!r&&r.dataset.hdConn==='live'&&!!w&&w.querySelector('[role="switch"]').getAttribute('aria-checked')==='false'&&!!l&&l.getAttribute('aria-valuenow')!==null;}`)

	// Coffee machine: switch on.
	page.MustEval(`()=>document.querySelector('[data-hd-wid="w2"] [role="switch"]').click()`)
	waitCall("switch", "turn_on", func(d map[string]any) bool { return d["entity_id"] == "switch.coffee" })
	until(15*time.Second, "the coffee machine confirmed on", `()=>{const w=document.querySelector('[data-hd-wid="w2"]');return w.querySelector('[role="switch"]').getAttribute('aria-checked')==='true'&&!w.hasAttribute('data-hd-pending')&&w.dataset.hdStatus==='ok';}`)
	if st, ok := ha.State("switch.coffee"); !ok || st.State != "on" {
		t.Fatalf("fake Home Assistant coffee machine = %+v", st)
	}

	// Kitchen light: dim by keyboard.
	if got := page.MustEval(`()=>document.querySelector('[data-hd-wid="w1"] .hd-slider').getAttribute('aria-valuenow')`).Str(); got != fmt.Sprint(initial) {
		t.Fatalf("kitchen brightness before the keys = %s, want %v (the demo's 102/255)", got, initial)
	}
	page.MustElement(`[data-hd-wid="w1"] .hd-slider`).MustFocus()
	for i := 0; i < keys; i++ {
		page.Keyboard.MustType(input.ArrowRight)
	}
	want := initial + keys
	waitCall("light", "turn_on", func(d map[string]any) bool { return d["entity_id"] == "light.kitchen" && d["brightness_pct"] == want })
	until(15*time.Second, "the kitchen light confirmed", `(v)=>{const w=document.querySelector('[data-hd-wid="w1"]');return w.querySelector('.hd-slider').getAttribute('aria-valuenow')===String(v)&&!w.hasAttribute('data-hd-pending')&&w.dataset.hdStatus==='ok';}`, want)
	var dims int
	for _, c := range ha.Calls() {
		if c.Domain == "light" && c.Service == "turn_on" && c.Data["entity_id"] == "light.kitchen" {
			dims++
		}
	}
	if dims != 1 {
		t.Fatalf("%d light.turn_on calls, want one trailing send for %d key presses", dims, keys)
	}
	if st, ok := ha.State("light.kitchen"); !ok || st.State != "on" {
		t.Fatalf("fake Home Assistant kitchen light = %+v", st)
	}
	// M4 (D13): edit and save against the real handlers. The designer's forSave body (kiosk block, theme snapshot, a widget added from the device
	// browser through pickWidgetFor) must pass the strict decoder and the validator.
	type hdDash struct {
		Dashboard map[string]any `json:"dashboard"`
	}
	var list struct {
		Dashboards []struct {
			ID       string `json:"id"`
			Revision int    `json:"revision"`
		} `json:"dashboards"`
	}
	hdDecode(t, env.do(t, env.admin, http.MethodGet, "dashboards", nil), &list)
	if len(list.Dashboards) != 1 {
		t.Fatalf("dashboards = %+v", list.Dashboards)
	}
	dashID, rev0 := list.Dashboards[0].ID, list.Dashboards[0].Revision
	server := func() map[string]any { var d hdDash; hdDecode(t, env.do(t, env.admin, http.MethodGet, "dashboard?id="+dashID, nil), &d); return d.Dashboard }
	waitRev := func(after int) map[string]any {
		var d map[string]any
		sourcetest.WaitFor(t, 15*time.Second, func() bool { d = server(); n, _ := d["revision"].(float64); return int(n) > after }, "a saved revision")
		return d
	}
	page.MustEval(`()=>document.querySelector('.hdx-edit').click()`)
	until(10*time.Second, "design mode", `()=>!!document.querySelector('.hd-root[data-hd-mode="design"] .hd-editbar')`)
	page.MustEval(`()=>document.querySelector('.hd-root[data-hd-mode="design"] [data-hd-wid="w2"]').focus()`)
	page.Keyboard.MustType(input.ArrowRight)
	d1 := waitRev(rev0)
	page.MustEval(`()=>document.querySelector('.hd-palette .hd-f-tab[data-tab="entities"]').click()`)
	until(15*time.Second, "the kitchen light row", `()=>!!document.querySelector('.hd-ent-row[data-entity="ha:light.kitchen"]')`)
	page.MustEval(`()=>document.querySelector('.hd-ent-row[data-entity="ha:light.kitchen"]').click()`)
	d2 := waitRev(int(d1["revision"].(float64)))
	found := false
	for _, p := range d2["pages"].([]any) {
		for _, w := range p.(map[string]any)["widgets"].([]any) {
			wm := w.(map[string]any)
			if b, _ := wm["bindings"].(map[string]any); b != nil && b["primary"] == "ha:light.kitchen" && wm["type"] == "light" {
				found = true
			}
		}
	}
	if !found {
		t.Fatalf("no light widget for ha:light.kitchen in %v", d2["pages"])
	}
	pal := d2["look"].(map[string]any)["palette"].(map[string]any)
	if snap, _ := pal["snapshot"].(map[string]any); pal["mode"] == "theme" && (len(snap) == 0 || len(snap) > 40) {
		t.Fatalf("theme snapshot = %v", pal["snapshot"])
	}
	page.MustEval(`()=>document.querySelector('.hd-eb-done').click()`)
	until(10*time.Second, "view mode", `()=>document.querySelector('.hdx-view .hd-root').dataset.hdMode==='view'`)

	if strings.Contains(page.MustEval(`()=>document.body.textContent`).Str(), hasstest.UpstreamSecret) {
		t.Fatal("Home Assistant text reached the page")
	}
	if errs := page.MustEval(`()=>hdErrors.join('\n')`).Str(); errs != "" {
		t.Fatalf("page errors:\n%s", errs)
	}
}
