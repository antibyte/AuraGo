package ui

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"aurago/internal/desktop"
	"aurago/internal/ecalc"
)

// TestDesktopECalcLaunchBrowser sends the open_app events the agent tool ecalc emits through the real desktop core and
// follows the launch payload: the first event opens a window and the payload reaches ECalcApp.render as ctx.launch, a
// second event for the open window reaches ECalcApp.launch (no second window), everything the sanitizer rejects never
// arrives, a repeated event id is handled once. The fixture shell of TestDesktopECalcBrowser is reused with
// handleDesktopEvent exported; the app's entry points are wrapped to record what they receive.
func TestDesktopECalcLaunchBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	catalog, err := ecalc.Default()
	if err != nil {
		t.Fatal(err)
	}
	html := regexp.MustCompile(`(?s)<script\b[^>]*>.*?</script>`).ReplaceAllString(readDesktopAssetText(t, "desktop.html"), "")
	html = regexp.MustCompile(`\{\{[^}]*\}\}`).ReplaceAllString(html, "")
	html = strings.Replace(html, "</body>", `<script src="/js/shared/lazy-assets.js"></script><script src="/js/desktop/core/module-loader.js"></script><script src="/ecalc-launch-shell.js"></script><script src="/testdata/aurora-fixture.js"></script></body>`, 1)
	shell := readDesktopAssetText(t, "js/desktop/bundles/main.bundle.js")
	cut := strings.LastIndex(shell, "    ensureDesktopRadialMenuAnchor();")
	if cut < 0 {
		t.Fatal("desktop startup seam missing")
	}
	shell = shell[:cut] + `window.aurora={state,loadIconManifest,applyDesktopSettings,renderIcons,renderStartApps,renderStartButtonIcon,wireShellChromeControls,bindViewportMetrics,handleDesktopKeydown,closeContextMenu,openApp,closeWindow,handleDesktopEvent};})();`

	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(Content)))
	mux.HandleFunc("/fixture", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, html) })
	mux.HandleFunc("/ecalc-launch-shell.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/javascript")
		fmt.Fprint(w, shell)
	})
	mux.HandleFunc("/ecalc-catalog.json", func(w http.ResponseWriter, r *http.Request) {
		body, _, err := catalog.LocalizedJSON(r.URL.Query().Get("lang"))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	})
	mux.HandleFunc("/fixture-words", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, readDesktopAssetText(t, "lang/desktop/de.json"))
	})
	mux.HandleFunc("/fixture-apps", func(w http.ResponseWriter, r *http.Request) { json.NewEncoder(w).Encode(desktop.BuiltinApps()) })
	server := httptest.NewServer(mux)
	defer server.Close()
	page := newSmokeBrowser(t).MustPage(server.URL + "/fixture").Timeout(90 * time.Second)
	defer page.Close()
	page.MustWaitLoad()
	page.MustSetViewport(1366, 900, 1, false)
	page.MustEval(`async()=>{
		await fixtureReady;
		fixtureTheme('standard');
		const base=window.fetch;
		window.fetch=(url,options)=>String(url).startsWith('/api/ecalc/catalog')
			?base('/ecalc-catalog.json'+String(url).slice('/api/ecalc/catalog'.length))
			:base(url,options);
		await AuraDesktopModules.loadAppAssets('ecalc');
		// Record what the shell hands to the app. A window that is already open reaches launch(windowId, payload),
		// a new one render(host, windowId, ctx) with ctx.launch.
		window.launchLog=[];
		const copy=v=>v===undefined?null:JSON.parse(JSON.stringify(v));
		const app=window.ECalcApp;
		const render=app.render.bind(app);
		app.render=(host,id,ctx)=>{window.launchLog.push({kind:'render',id,launch:copy(ctx.launch),hasLaunchKey:'launch' in ctx});return render(host,id,ctx);};
		const launch=typeof app.launch==='function'?app.launch.bind(app):()=>{};
		app.launch=(id,payload)=>{window.launchLog.push({kind:'launch',id,payload:copy(payload)});return launch(id,payload);};
		window.sendDesktopEvent=json=>aurora.handleDesktopEvent(JSON.parse(json));
	}`)

	wait := func(expr string) {
		t.Helper()
		waitForJSBool(t, page, expr)
	}
	send := func(event map[string]any) {
		t.Helper()
		raw, err := json.Marshal(event)
		if err != nil {
			t.Fatal(err)
		}
		page.MustEval(`json=>sendDesktopEvent(json)`, string(raw))
	}
	sendRaw := func(raw string) {
		t.Helper()
		page.MustEval(`json=>sendDesktopEvent(json)`, raw)
	}
	log := func() string { return page.MustEval(`()=>JSON.stringify(window.launchLog)`).Str() }
	windows := func() int {
		return page.MustEval(`()=>document.querySelectorAll('.vd-window[data-app-id="ecalc"]').length`).Int()
	}
	windowID := func() string {
		return page.MustEval(`()=>document.querySelector('.vd-window[data-app-id="ecalc"]').dataset.windowId`).Str()
	}
	// fields reads the fields of the open page as var=text(state) in page order, title the name of the page.
	fields := func() string {
		return page.MustEval(`()=>[...document.querySelectorAll('.ec-field')].map(f=>f.dataset.var+'='+f.querySelector('.ec-input').value+'('+f.dataset.state+')').join(' ')`).Str()
	}
	// derived reads the values of the derived quantities of the page.
	derived := func() string {
		return page.MustEval(`()=>[...document.querySelectorAll('[data-rows="derived"] .ec-row-val')].map(e=>e.textContent).join(' ')`).Str()
	}
	title := func() string { return page.MustEval(`()=>document.querySelector('.ec-title').textContent`).Str() }
	deName := func(calc string) string { return catalog.Calculator(calc).Name["de"] }
	activeWindow := func() string { return page.MustEval(`()=>aurora.state.activeWindowId`).Str() }

	// 1. No window yet: the event opens one and the payload arrives as ctx.launch, values in the order they were sent.
	// The events of the ordered cases are written as text: a Go map would sort the keys, the tool does not.
	sendRaw(`{"id":"evt-1","type":"open_app","payload":{"app_id":"ecalc","ecalc":{"page":"voltage_divider","values":{"vin":"12 V","r1":"4k7"},"choices":{}}}}`)
	wait(`()=>document.querySelectorAll('.vd-window[data-app-id="ecalc"]').length===1 && !!document.querySelector('.ec-page[data-calc]')`)
	id := windowID()
	// The page opens pre-filled. The values are what the agent typed (the shell is German, so the fields show a decimal
	// comma); r2 keeps its default because two values are fewer than the three the divider needs; vout is computed.
	if got := title(); got != deName("voltage_divider") {
		t.Fatalf("the page is %q, want %q", got, deName("voltage_divider"))
	}
	if got := fields(); got != "vin=12 V(input) r1=4,7 kΩ(input) r2=2,2 kΩ(input) vout=3,826 V(computed)" {
		t.Fatalf("the first open shows %s", got)
	}
	if got := log(); !strings.HasPrefix(got, `[{"kind":"render","id":"`+id+`","launch":{"page":"voltage_divider","values":{"vin":"12 V","r1":"4k7"},"choices":{}},"hasLaunchKey":true}`) {
		t.Fatalf("the first open handed over %s", got)
	}

	// 2. The window is open: the next event goes to ECalcApp.launch of that window; no second window opens.
	before := page.MustEval(`()=>window.launchLog.length`).Int()
	sendRaw(`{"id":"evt-2","type":"open_app","payload":{"app_id":"ecalc","ecalc":{"page":"ohms_law","values":{"u":"5 V","i":"10 mA"},"choices":{"wave":"triangle"}}}}`)
	wait(fmt.Sprintf(`()=>window.launchLog.length===%d`, before+1))
	want := `{"kind":"launch","id":"` + id + `","payload":{"page":"ohms_law","values":{"u":"5 V","i":"10 mA"},"choices":{"wave":"triangle"}}}`
	if got := page.MustEval(`()=>JSON.stringify(window.launchLog[window.launchLog.length-1])`).Str(); got != want {
		t.Fatalf("the second event handed over %s, want %s", got, want)
	}
	if n := windows(); n != 1 {
		t.Fatalf("ECalc is single-instance, %d windows are open", n)
	}
	// The open window switched to the calculator of the event with its values: 5 V / 10 mA = 500 ohm, 5 V * 10 mA = 50 mW.
	wait(`()=>document.querySelector('.ec-page')?.dataset.calc==='ohms_law'`)
	if got := title(); got != deName("ohms_law") {
		t.Fatalf("the page is %q, want %q", got, deName("ohms_law"))
	}
	if got := fields(); got != "u=5 V(input) i=10 mA(input) r=500 Ω(computed) p=50 mW(computed)" {
		t.Fatalf("the second event shows %s", got)
	}

	// 3. A repeated event id is handled once.
	send(map[string]any{"id": "evt-2", "type": "open_app", "payload": map[string]any{"app_id": "ecalc", "ecalc": map[string]any{"page": "ohms_law", "values": map[string]string{"u": "9 V"}}}})
	// 4. What the sanitizer rejects never reaches the app: a page id that is no catalog id drops the whole launch ...
	send(map[string]any{"id": "evt-3", "type": "open_app", "payload": map[string]any{"app_id": "ecalc", "ecalc": map[string]any{"page": "Voltage Divider", "values": map[string]string{"vin": "1"}}}})
	send(map[string]any{"id": "evt-4", "type": "open_app", "payload": map[string]any{"app_id": "ecalc"}})
	send(map[string]any{"id": "evt-5", "type": "open_app", "payload": map[string]any{"app_id": "ecalc", "ecalc": "voltage_divider"}})
	// ... and bad names, non-string values, over-long text and a hostile key are cut out of an otherwise good one.
	sendRaw(`{"id":"evt-6","type":"open_app","payload":{"app_id":"ecalc","ecalc":{"page":"ohms_law","values":{"__proto__":"x","Vin":"1","r":"` + strings.Repeat("9", 257) + `","u":5,"i":"2 mA","p":null},"choices":{"wave":"Triangle","mode":"fast_plus"}}}}`)
	// The launch is only taken from an ECalc event, never from another app's.
	send(map[string]any{"id": "evt-7", "type": "open_app", "payload": map[string]any{"app_id": "calculator", "ecalc": map[string]any{"page": "ohms_law"}}})
	wait(fmt.Sprintf(`()=>window.launchLog.length===%d`, before+2))
	want = `{"kind":"launch","id":"` + id + `","payload":{"page":"ohms_law","values":{"i":"2 mA"},"choices":{"mode":"fast_plus"}}}`
	if got := page.MustEval(`()=>JSON.stringify(window.launchLog[window.launchLog.length-1])`).Str(); got != want {
		t.Fatalf("the sanitized event handed over %s, want %s", got, want)
	}
	if n := page.MustEval(`()=>window.launchLog.length`).Int(); n != before+2 {
		t.Fatalf("only the two good events may reach the app, the log is %s", log())
	}
	if n := page.MustEval(`()=>document.querySelectorAll('.vd-window[data-app-id="calculator"]').length`).Int(); n != 1 {
		t.Fatalf("the calculator event must still open the calculator, %d windows", n)
	}
	// Only the given value is taken; the page starts from its defaults for the rest: 12 V / 2 mA = 6 kohm, 12 V * 2 mA = 24 mW.
	if got := fields(); got != "u=12 V(input) i=2 mA(input) r=6 kΩ(computed) p=24 mW(computed)" {
		t.Fatalf("the sanitized event shows %s", got)
	}

	// 5. A launch for the open window brings it to the front: the calculator that evt-7 opened is on top now.
	if activeWindow() == id {
		t.Fatal("the calculator window must be the active one before the next launch")
	}
	sendRaw(`{"id":"evt-9","type":"open_app","payload":{"app_id":"ecalc","ecalc":{"page":"series_parallel","values":{"rs":"4k7, 10k"}}}}`)
	wait(fmt.Sprintf(`()=>window.launchLog.length===%d`, before+3))
	wait(`()=>document.querySelector('.ec-page')?.dataset.calc==='series_parallel'`)
	if activeWindow() != id || windows() != 1 {
		t.Fatalf("the launch must focus the open ECalc window %s, the active one is %s (%d ECalc windows)", id, activeWindow(), windows())
	}
	// 4.7k + 10k = 14.7k; 4.7k * 10k / 14.7k = 3197.3 ohm.
	if got, sums := fields(), derived(); got != "rs=4.7kΩ, 10kΩ(input)" || sums != "14,7 kΩ 3,197 kΩ" {
		t.Fatalf("the list launch shows %s and the totals %s", got, sums)
	}

	// 6. A closed window opens again with a fresh launch: 9 V * 2k / (1k + 2k) = 6 V.
	page.MustEval(`id=>aurora.closeWindow(id)`, id)
	wait(`()=>document.querySelectorAll('.vd-window[data-app-id="ecalc"]').length===0`)
	sendRaw(`{"id":"evt-8","type":"open_app","payload":{"app_id":"ecalc","ecalc":{"page":"voltage_divider","values":{"vin":"9 V","r1":"1k","r2":"2k"}}}}`)
	wait(`()=>document.querySelectorAll('.vd-window[data-app-id="ecalc"]').length===1 && document.querySelector('.ec-page')?.dataset.calc==='voltage_divider'`)
	if got := page.MustEval(`()=>JSON.stringify(window.launchLog[window.launchLog.length-1].launch)`).Str(); got != `{"page":"voltage_divider","values":{"vin":"9 V","r1":"1k","r2":"2k"},"choices":{}}` {
		t.Fatalf("the reopened window got %s", got)
	}
	if got := fields(); got != "vin=9 V(input) r1=1 kΩ(input) r2=2 kΩ(input) vout=6 V(computed)" {
		t.Fatalf("the reopened window shows %s", got)
	}

	// 7. A launched bare number is read in the catalog unit of its variable, as the tool's Check reads it, not in the display
	// unit the user picked last: R1 is set to fixed kilo-ohms, the agent sends 4700 (ohm) for it, and the field shows 4.7 kOhm in
	// its automatic unit again (not 4700 kOhm = 4.7 MOhm).
	id = windowID()
	page.MustEval(`()=>{const s=document.querySelector('.ec-field[data-var="r1"] .ec-unit');s.value='prefix:k';s.dispatchEvent(new Event('change',{bubbles:true}));}`)
	wait(`()=>document.querySelector('.ec-field[data-var="r1"] .ec-unit').value==='prefix:k'`)
	n := page.MustEval(`()=>window.launchLog.length`).Int()
	sendRaw(`{"id":"evt-10","type":"open_app","payload":{"app_id":"ecalc","ecalc":{"page":"voltage_divider","values":{"r1":"4700"}}}}`)
	wait(fmt.Sprintf(`()=>window.launchLog.length===%d`, n+1))
	if got := fields(); got != "vin=12 V(input) r1=4,7 kΩ(input) r2=2,2 kΩ(input) vout=3,826 V(computed)" {
		t.Fatalf("a launched 4700 on a field picked to kilo-ohms shows %s", got)
	}
	if got := page.MustEval(`()=>document.querySelector('.ec-field[data-var="r1"] .ec-unit').value`).Str(); got != "" {
		t.Fatalf("the unit of R1 shows %q after the launch, want the catalog default (automatic)", got)
	}
	if got := page.MustEval(`()=>JSON.stringify(window.launchLog[window.launchLog.length-1])`).Str(); !strings.HasPrefix(got, `{"kind":"launch","id":"`+id+`"`) {
		t.Fatalf("the launch for the open window was handed over as %s", got)
	}
	if errors := page.MustEval(`()=>JSON.stringify(fixtureErrors)`).Str(); errors != "[]" {
		t.Fatal(errors)
	}
}
