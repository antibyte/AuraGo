package ui

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"aurago/internal/desktop"
	"aurago/internal/ecalc"
	"github.com/go-rod/rod/lib/input"
)

// TestDesktopECalcBrowser drives ECalc inside the real desktop shell with the catalog of the server route: live solving
// on the voltage divider (the oldest input yields, a lock pins a field), the star-delta set switch, the field chips on
// every schematic, the compact stack and the Ctrl+K shortcut that must not open the Spotlight. The shell runs in
// German: numbers appear with a decimal comma and the app must show the German catalog (names and categories), while
// the search still finds items by their English name. Set AURAGO_BROWSER_ARTIFACT_DIR to keep screenshots.
func TestDesktopECalcBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	catalog, err := ecalc.Default()
	if err != nil {
		t.Fatal(err)
	}
	// names reads the item and category names of one language from the same JSON the route serves.
	names := func(lang string) (items, categories map[string]string, categoryNames []string) {
		t.Helper()
		body, _, err := catalog.LocalizedJSON(lang)
		if err != nil {
			t.Fatal(err)
		}
		var parsed struct {
			Items      []struct{ ID, Name string } `json:"items"`
			Categories []struct{ ID, Name string } `json:"categories"`
		}
		if err := json.Unmarshal(body, &parsed); err != nil {
			t.Fatal(err)
		}
		items, categories = map[string]string{}, map[string]string{}
		for _, item := range parsed.Items {
			items[item.ID] = item.Name
		}
		for _, category := range parsed.Categories {
			categories[category.ID] = category.Name
			categoryNames = append(categoryNames, category.Name)
		}
		return items, categories, categoryNames
	}
	deItems, deCategories, _ := names("de")
	enItems, enCategories, _ := names("en")
	if deItems["voltage_divider"] == "" || deItems["voltage_divider"] == enItems["voltage_divider"] || deCategories["basics"] == enCategories["basics"] {
		t.Fatalf("the German and the English catalog must differ: %q / %q and %q / %q", deItems["voltage_divider"], enItems["voltage_divider"], deCategories["basics"], enCategories["basics"])
	}
	html := regexp.MustCompile(`(?s)<script\b[^>]*>.*?</script>`).ReplaceAllString(readDesktopAssetText(t, "desktop.html"), "")
	html = regexp.MustCompile(`\{\{[^}]*\}\}`).ReplaceAllString(html, "")
	html = strings.Replace(html, "</body>", `<script src="/js/shared/lazy-assets.js"></script><script src="/js/desktop/core/module-loader.js"></script><script src="/ecalc-shell.js"></script><script src="/testdata/aurora-fixture.js"></script></body>`, 1)
	shell := readDesktopAssetText(t, "js/desktop/bundles/main.bundle.js")
	cut := strings.LastIndex(shell, "    ensureDesktopRadialMenuAnchor();")
	if cut < 0 {
		t.Fatal("desktop startup seam missing")
	}
	shell = shell[:cut] + `window.aurora={state,loadIconManifest,applyDesktopSettings,renderIcons,renderStartApps,renderStartButtonIcon,wireShellChromeControls,bindViewportMetrics,handleDesktopKeydown,closeContextMenu,openApp,closeWindow};})();`

	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(Content)))
	mux.HandleFunc("/fixture", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, html) })
	mux.HandleFunc("/ecalc-shell.js", func(w http.ResponseWriter, r *http.Request) {
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
	// The fixture answers every /api/ request with an empty object; the catalog route gets the real catalog of the
	// requested language, or a failure while window.ecalcCatalogDown is set. The served German catalog names the voltage
	// divider "Zzyzx divider" in its name_en field, a word no German name or tag contains. The cache mode of every
	// catalog request is recorded: the app must ask with 'no-cache' so the browser revalidates the ETag of the route.
	page.MustEval(`async()=>{
		await fixtureReady;
		window.ecalcCatalogRequests=[];
		window.ecalcCatalogCaches=[];
		const base=window.fetch;
		window.fetch=(url,options)=>{
			const text=String(url);
			if(!text.startsWith('/api/ecalc/catalog'))return base(url,options);
			window.ecalcCatalogRequests.push(text);
			window.ecalcCatalogCaches.push(String(options&&options.cache));
			if(window.ecalcCatalogDown)return Promise.resolve(new Response('{"error":"down"}',{status:500,headers:{'Content-Type':'application/json'}}));
			return base('/ecalc-catalog.json'+text.slice('/api/ecalc/catalog'.length)).then(r=>r.json()).then(j=>{
				j.items.find(i=>i.id==='voltage_divider').name_en='Zzyzx divider';
				return new Response(JSON.stringify(j),{headers:{'Content-Type':'application/json'}});
			});
		};
	}`)

	shot := func(name string) {
		t.Helper()
		dir := os.Getenv("AURAGO_BROWSER_ARTIFACT_DIR")
		if dir == "" {
			return
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		page.MustScreenshot(filepath.Join(dir, name+".png"))
	}
	wait := func(expr string) {
		t.Helper()
		waitForJSBool(t, page, expr)
	}
	field := func(name string) string { return `.ec-field[data-var="` + name + `"]` }
	value := func(name string) string {
		return page.MustEval(`n=>document.querySelector('.ec-field[data-var="'+n+'"] .ec-input').value`, name).Str()
	}
	state := func(name string) string {
		return page.MustEval(`n=>document.querySelector('.ec-field[data-var="'+n+'"]').dataset.state`, name).Str()
	}
	typeInto := func(name, text string) {
		t.Helper()
		el := page.MustElement(field(name) + " .ec-input")
		el.MustClick()
		el.MustSelectAllText()
		el.MustInput(text)
	}
	setUnit := func(name, unit string) {
		t.Helper()
		page.MustEval(`(n,u)=>{const s=document.querySelector('.ec-field[data-var="'+n+'"] .ec-unit');s.value=u;s.dispatchEvent(new Event('change',{bubbles:true}));}`, name, unit)
	}
	visible := func(selector string) bool {
		return page.MustEval(`s=>{const e=document.querySelector(s);return !!e&&getComputedStyle(e).display!=='none'&&e.getClientRects().length>0}`, selector).Bool()
	}
	openApp := func() {
		t.Helper()
		page.MustEval(`async()=>{await fixtureOpen('ecalc');}`)
		wait(`()=>!!document.querySelector('.ec-page[data-calc]')`)
	}
	openCalc := func(id string) {
		t.Helper()
		page.MustElement(`.ec-item[data-id="` + id + `"]`).MustClick()
		wait(`()=>document.querySelector('.ec-page')?.dataset.calc==='` + id + `'`)
	}
	// The field chips of a schematic must stay inside the drawing and apart from each other.
	chipIssues := `()=>{
		const wrap=document.querySelector('.ec-sch-wrap');
		if(!wrap)return '';
		const box=wrap.getBoundingClientRect(),issues=[];
		const chips=[...wrap.querySelectorAll('.ec-chips > .ec-field')].map(c=>({id:c.dataset.var,r:c.getBoundingClientRect()}));
		chips.forEach((a,i)=>{
			if(a.r.left<box.left-1||a.r.right>box.right+1||a.r.top<box.top-1||a.r.bottom>box.bottom+1)issues.push(a.id+' sticks out of the drawing');
			chips.slice(i+1).forEach(b=>{if(a.r.left<b.r.right&&b.r.left<a.r.right&&a.r.top<b.r.bottom&&b.r.top<a.r.bottom)issues.push(a.id+' overlaps '+b.id);});
		});
		return issues.join('; ');
	}`

	// ---- opening: the rail, the list and the first calculators -------------------------------------------------
	page.MustEval(`()=>fixtureTheme('standard')`)
	// A failing catalog shows the failure and a retry button that loads it.
	page.MustEval(`async()=>{window.ecalcCatalogDown=true;await fixtureOpen('ecalc');}`)
	wait(`()=>!!document.querySelector('.ec-fail [data-action="retry"]') && document.querySelector('.ec-fail p').textContent.length>0`)
	page.MustEval(`()=>{window.ecalcCatalogDown=false;}`)
	page.MustElement(`.ec-fail [data-action="retry"]`).MustClick()
	wait(`()=>!!document.querySelector('.ec-page[data-calc]')`)
	wait(`()=>!!document.querySelector('.ec-rail-btn[data-rail="favorites"]') && !!document.querySelector('.ec-rail-btn[data-rail="basics"]')`)
	if got := page.MustEval(`()=>document.querySelectorAll('.ec-items .ec-item').length`).Int(); got != 11 {
		t.Fatalf("the basics category lists %d calculators, want 11", got)
	}
	openCalc("voltage_divider")
	wait(`()=>document.querySelector('.ec-field[data-var="vout"] .ec-input').value!==''`)
	// The desktop language is German: the page, the list and the rail show the German catalog.
	if got := page.MustEval(`()=>document.querySelector('.ec-title').textContent`).Str(); got != deItems["voltage_divider"] {
		t.Fatalf("the page title is %q, want the German name %q", got, deItems["voltage_divider"])
	}
	if got := page.MustEval(`()=>document.querySelector('.ec-item[data-id="voltage_divider"] .ec-item-name').textContent`).Str(); got != deItems["voltage_divider"] {
		t.Fatalf("the list shows %q, want the German name %q", got, deItems["voltage_divider"])
	}
	if got := page.MustEval(`()=>document.querySelector('.ec-rail-btn[data-rail="basics"] .ec-rail-label').textContent`).Str(); got != deCategories["basics"] {
		t.Fatalf("the rail shows %q, want the German category %q", got, deCategories["basics"])
	}
	// One request per open (the failing one and the retry so far), all in the desktop's language, all revalidating.
	if got := page.MustEval(`()=>JSON.stringify(window.ecalcCatalogRequests)`).Str(); got != `["/api/ecalc/catalog?lang=de","/api/ecalc/catalog?lang=de"]` {
		t.Fatalf("the failed open and its retry asked for %s, want exactly two German catalog requests", got)
	}
	if got := page.MustEval(`()=>JSON.stringify(window.ecalcCatalogCaches)`).Str(); got != `["no-cache","no-cache"]` {
		t.Fatalf("the catalog requests use the cache modes %s, want no-cache so that the ETag is revalidated", got)
	}
	// Every rail label of every language wraps instead of being cut off.
	labels := map[string][]string{}
	for _, lang := range ecalc.Locales {
		words := readMergedLangMap(t, lang)
		_, _, categoryNames := names(lang)
		labels[lang] = append([]string{words["desktop.ecalc_favorites"], words["desktop.ecalc_recent"]}, categoryNames...)
	}
	if got := page.MustEval(`labels=>{
		const rail=document.querySelector('.ec-rail'),button=rail.querySelector('.ec-rail-btn[data-rail="recent"]').cloneNode(true);
		rail.appendChild(button);
		const label=button.querySelector('.ec-rail-label'),issues=[],line=parseFloat(getComputedStyle(label).lineHeight)||18;
		for(const [lang,texts] of Object.entries(labels)){
			button.lang=lang;
			for(const text of texts){
				label.textContent=text;
				if(label.scrollWidth>label.clientWidth+1)issues.push(lang+': "'+text+'" is cut off');
				if(label.clientHeight>line*3+2)issues.push(lang+': "'+text+'" needs more than three lines');
			}
		}
		button.remove();
		return issues.join('; ');
	}`, labels).Str(); got != "" {
		t.Fatalf("rail labels: %s", got)
	}
	if got := []string{value("vin"), value("r1"), value("r2"), value("vout")}; got[0] != "12 V" || got[1] != "4,7 kΩ" || got[2] != "2,2 kΩ" || got[3] != "3,826 V" {
		t.Fatalf("divider defaults %v, want 12 V, 4,7 kΩ, 2,2 kΩ and 3,826 V", got)
	}
	if state("vin") != "input" || state("vout") != "computed" {
		t.Fatalf("states vin=%s vout=%s, want input and computed", state("vin"), state("vout"))
	}
	if got := page.MustEval(chipIssues).Str(); got != "" {
		t.Fatalf("divider chips: %s", got)
	}
	if !page.MustEval(`()=>!!document.querySelector('.ec-frac') && !!document.querySelector('.ec-live .ec-v') && !!document.querySelector('.ec-sch path')`).Bool() {
		t.Fatal("the divider page must show the typeset fraction, the live line and the schematic")
	}
	shot("voltage-divider-standard")

	// ---- every desktop skin keeps the chips apart; three are kept as screenshots ---------------------------------------
	for _, theme := range parseDesktopThemeRegistry(t) {
		names := []string{theme.ID}
		if theme.ID == "fruity" {
			names = []string{"fruity-light", "fruity-dark"}
		}
		for _, name := range names {
			page.MustEval(`id=>fixtureTheme(id)`, name)
			if got := page.MustEval(chipIssues).Str(); got != "" {
				t.Fatalf("skin %s: divider chips: %s", name, got)
			}
			if name == "fruity-light" || name == "minimal" || name == "cyberpunk" {
				shot("voltage-divider-" + name)
			}
		}
	}
	page.MustEval(`()=>fixtureTheme('standard')`)

	// ---- live solving: R2 changes V_out; typing V_out makes the oldest input (R2) computed ----------------------
	typeInto("r2", "3k3")
	wait(`()=>document.querySelector('.ec-field[data-var="vout"] .ec-input').value==='4,95 V'`)
	typeInto("vin", "10")
	wait(`()=>document.querySelector('.ec-field[data-var="vout"] .ec-input').value==='4,125 V'`)
	typeInto("r1", "5k")
	wait(`()=>document.querySelector('.ec-field[data-var="vout"] .ec-input').value==='3,976 V'`)
	typeInto("vout", "3")
	wait(`()=>document.querySelector('.ec-field[data-var="r2"]').dataset.state==='computed' && document.querySelector('.ec-field[data-var="r2"] .ec-input').value==='2,143 kΩ'`)
	if state("vout") != "input" || state("vin") != "input" || state("r1") != "input" {
		t.Fatalf("after typing V_out the states are vin=%s r1=%s vout=%s, want inputs", state("vin"), state("r1"), state("vout"))
	}
	// An unreadable text changes nothing and says so; leaving the field restores the last good value.
	typeInto("r1", "abc")
	wait(`()=>document.querySelector('.ec-field[data-var="r1"]').dataset.state==='error' && document.querySelector('.ec-field[data-var="r1"] .ec-field-msg').textContent.length>0`)
	page.MustElement(field("vin") + " .ec-input").MustClick()
	wait(`()=>document.querySelector('.ec-field[data-var="r1"]').dataset.state==='input' && document.querySelector('.ec-field[data-var="r1"] .ec-input').value==='5 kΩ'`)

	// ---- lock: a locked input never yields -----------------------------------------------------------------------
	page.MustElement(field("vin") + " .ec-lock").MustClick()
	if page.MustEval(`()=>document.querySelector('.ec-field[data-var="vin"] .ec-lock').getAttribute('aria-pressed')`).Str() != "true" {
		t.Fatal("the lock button must be pressed")
	}
	typeInto("r2", "2k")
	wait(`()=>document.querySelector('.ec-field[data-var="r1"]').dataset.state==='computed'`)
	if state("vin") != "input" || state("r2") != "input" || state("vout") != "input" {
		t.Fatalf("with V_in locked the states are vin=%s r2=%s vout=%s, want inputs", state("vin"), state("r2"), state("vout"))
	}
	// With every input locked, typing into the computed R1 would have to unlock one: the edit is refused and says so,
	// and leaving the field shows the computed value again.
	page.MustElement(field("r2") + " .ec-lock").MustClick()
	page.MustElement(field("vout") + " .ec-lock").MustClick()
	lockedOut := value("vout")
	typeInto("r1", "1k")
	wait(fmt.Sprintf(`()=>{const f=document.querySelector('.ec-field[data-var="r1"]');return f.dataset.state==='error'&&f.querySelector('.ec-field-msg').textContent===%q}`, readMergedLangMap(t, "de")["desktop.ecalc_error_locked"]))
	page.MustElement(field("vin") + " .ec-input").MustClick()
	wait(`()=>document.querySelector('.ec-field[data-var="r1"]').dataset.state==='computed'`)
	if state("vin") != "input" || state("r2") != "input" || state("vout") != "input" || value("vout") != lockedOut || value("r1") == "1k" {
		t.Fatalf("after the refused edit the states are vin=%s r2=%s vout=%s, V_out %q (was %q), R1 %q; want the locked inputs unchanged and R1 computed again", state("vin"), state("r2"), state("vout"), value("vout"), lockedOut, value("r1"))
	}
	page.MustElement(`[data-action="reset"]`).MustClick()
	wait(`()=>document.querySelector('.ec-field[data-var="r2"] .ec-input').value==='2,2 kΩ' && document.querySelector('.ec-field[data-var="vin"] .ec-lock').getAttribute('aria-pressed')==='false'`)

	// ---- unit dropdown and the favorite -------------------------------------------------------------------------
	setUnit("vout", "prefix:m")
	wait(`()=>document.querySelector('.ec-field[data-var="vout"] .ec-input').value==='3826 mV'`)
	setUnit("vout", "")
	wait(`()=>document.querySelector('.ec-field[data-var="vout"] .ec-input').value==='3,826 V'`)
	page.MustElement(`[data-action="favorite"]`).MustClick()
	wait(`()=>document.querySelector('.ec-rail-btn[data-rail="favorites"] .ec-count').textContent==='1'`)

	// ---- star-delta: typing a star value switches the whole set -----------------------------------------------
	openCalc("star_delta")
	wait(`()=>document.querySelector('.ec-field[data-var="ra"] .ec-input').value==='15 Ω'`)
	if state("rab") != "input" || state("ra") != "computed" {
		t.Fatalf("star-delta starts with rab=%s ra=%s, want input and computed", state("rab"), state("ra"))
	}
	shot("star-delta-standard")
	typeInto("ra", "12")
	wait(`()=>document.querySelector('.ec-field[data-var="rab"]').dataset.state==='computed' && document.querySelector('.ec-field[data-var="ra"]').dataset.state==='input'`)
	if state("rb") != "input" || state("rc") != "input" || value("rb") != "10 Ω" || value("rc") != "30 Ω" {
		t.Fatalf("after the switch rb=%s %q rc=%s %q, want the computed values kept as inputs", state("rb"), value("rb"), state("rc"), value("rc"))
	}
	wait(`()=>document.querySelector('.ec-field[data-var="rab"] .ec-input').value==='26 Ω'`)

	// ---- every schematic keeps its field chips apart and inside the drawing ---------------------------------------------
	for _, id := range []string{"ohms_law", "voltage_divider", "loaded_divider", "current_divider", "series_parallel", "wheatstone", "star_delta"} {
		openCalc(id)
		shot("schematic-" + id)
		if got := page.MustEval(`()=>document.querySelector('.ec-page').dataset.chips`).Str(); got != "anchored" {
			t.Fatalf("%s: the fields are %q at the default window size, want anchored on the schematic", id, got)
		}
		if got := page.MustEval(chipIssues).Str(); got != "" {
			t.Fatalf("%s chips: %s", id, got)
		}
		if id == "series_parallel" {
			if got := value("rs"); got != "4.7kΩ, 10kΩ, 2.2kΩ" {
				t.Fatalf("the resistor list shows %q, want decimal points (the comma separates the items)", got)
			}
		}
	}
	// A unit that cannot show a value does not blank the field: the bridge voltage of the defaults is negative, dBV
	// cannot show it, so the automatic text stays (and the field reads back under the dBV selection).
	openCalc("wheatstone")
	setUnit("vd", "dBV")
	wait(`()=>document.querySelector('.ec-field[data-var="vd"] .ec-input').value==='-119 mV'`)
	setUnit("vd", "")
	openCalc("decibel_ratio")
	page.MustElement(`.ec-choice input[value="power"]`).MustClick()
	wait(`()=>document.querySelector('.ec-field[data-var="g"] .ec-input').value==='10 dB'`)

	// ---- Ctrl+K belongs to the app: the search gets the focus, the Spotlight stays closed ----------------------------
	// The desktop ignores keys typed into fields, so the focus sits on a button when the shortcut is pressed.
	page.MustElement(`.ec-rail-btn[data-rail="basics"]`).MustFocus()
	page.KeyActions().Press(input.ControlLeft).Type('k').MustDo()
	wait(`()=>document.activeElement&&document.activeElement.classList.contains('ec-search-input')`)
	if page.MustEval(`()=>!!document.getElementById('vd-spotlight-backdrop')`).Bool() {
		t.Fatal("Ctrl+K inside ECalc must not open the desktop Spotlight")
	}
	search := page.MustElement(".ec-search-input")
	search.MustInput("teiler")
	wait(`()=>[...document.querySelectorAll('.ec-items .ec-item')].some(b=>b.dataset.id==='voltage_divider')`)
	// The English name (name_en of the German catalog) finds the item although the catalog and the page are German.
	search.MustSelectAllText().MustInput("zzyzx")
	wait(`()=>document.querySelectorAll('.ec-items .ec-item').length===1 && document.querySelector('.ec-items .ec-item').dataset.id==='voltage_divider'`)
	if got := page.MustEval(`()=>document.querySelector('.ec-items .ec-item .ec-item-name').textContent`).Str(); got != deItems["voltage_divider"] {
		t.Fatalf("the English-name hit shows %q, want the German name %q", got, deItems["voltage_divider"])
	}
	if got := page.MustEval(`()=>window.ecalcCatalogRequests.some(u=>!u.includes('lang=de'))`).Bool(); got {
		t.Fatal("the app asked for a catalog in a language other than the desktop's (there is no English feed any more)")
	}
	search.MustSelectAllText().MustInput("star delta")
	wait(`()=>document.querySelectorAll('.ec-items .ec-item').length===1 && document.querySelector('.ec-items .ec-item').dataset.id==='star_delta'`)
	search.MustSelectAllText().MustInput("zzzz")
	wait(`()=>document.querySelectorAll('.ec-items .ec-item').length===0 && !document.querySelector('.ec-empty').hidden`)
	search.MustSelectAllText().MustInput(" ")
	wait(`()=>document.querySelectorAll('.ec-items .ec-item').length===11`)
	openCalc("voltage_divider")

	// ---- compact: one stack with a back button -------------------------------------------------------------------
	page.MustEval(`()=>[...aurora.state.windows.keys()].forEach(id=>aurora.closeWindow(id))`)
	page.MustSetViewport(390, 667, 1, false)
	page.MustEval(`()=>fixtureTheme('fruity-dark')`)
	openApp()
	wait(`()=>document.querySelector('.ec-app').dataset.compact==='true'`)
	if !visible(".ec-page-host") || visible(".ec-rail") || visible(".ec-list") || !visible(".ec-back") {
		t.Fatal("compact mode must start on the page of the last calculator with the back button shown")
	}
	if got := page.MustEval(`()=>document.querySelector('.ec-page').dataset.chips`).Str(); got != "listed" {
		t.Fatalf("on a phone the fields are %q, want listed under the schematic", got)
	}
	shot("compact-page")
	if got := page.MustEval(`()=>{const p=document.querySelector('.ec-page-host');return p.scrollWidth>p.clientWidth+1?'the page scrolls sideways':'';}`).Str(); got != "" {
		t.Fatal(got)
	}
	// Searching from the page view moves to the list with the results; Ctrl+K with a query still in the box does too.
	page.MustElement(".ec-search-input").MustInput("teiler")
	wait(`()=>document.querySelector('.ec-app').dataset.view==='list' && [...document.querySelectorAll('.ec-items .ec-item')].some(b=>b.dataset.id==='voltage_divider')`)
	if !visible(".ec-list") || visible(".ec-page-host") || visible(".ec-rail") {
		t.Fatal("typing a query on the page must show the list with the results")
	}
	page.MustElement(".ec-search-input").MustType(input.Enter)
	wait(`()=>document.querySelector('.ec-app').dataset.view==='page'`)
	page.MustElement(".ec-back").MustFocus()
	page.KeyActions().Press(input.ControlLeft).Type('k').MustDo()
	wait(`()=>document.querySelector('.ec-app').dataset.view==='list' && document.activeElement&&document.activeElement.classList.contains('ec-search-input')`)
	if !visible(".ec-list") || visible(".ec-page-host") {
		t.Fatal("Ctrl+K with a query must show the list with the results")
	}
	page.MustElement(".ec-search-input").MustType(input.Enter)
	wait(`()=>document.querySelector('.ec-app').dataset.view==='page'`)
	page.MustElement(".ec-back").MustClick()
	if !visible(".ec-list") || visible(".ec-page-host") || visible(".ec-rail") {
		t.Fatal("back from the page must show the list")
	}
	shot("compact-list")
	page.MustElement(".ec-back").MustClick()
	if !visible(".ec-rail") || visible(".ec-list") || visible(".ec-back") {
		t.Fatal("back from the list must show the rail and hide the back button")
	}
	page.MustElement(`.ec-rail-btn[data-rail="basics"]`).MustClick()
	page.MustElement(`.ec-item[data-id="voltage_divider"]`).MustClick()
	wait(`()=>document.querySelector('.ec-page')?.dataset.calc==='voltage_divider'`)
	if !visible(".ec-page-host") {
		t.Fatal("opening a calculator must show its page")
	}
	typeInto("r2", "10k")
	wait(`()=>document.querySelector('.ec-field[data-var="vout"] .ec-input').value==='8,163 V'`)

	// Three opens (the failing one, its retry and the compact window) made exactly three German catalog requests.
	const german = `"/api/ecalc/catalog?lang=de"`
	if got := page.MustEval(`()=>JSON.stringify(window.ecalcCatalogRequests)`).Str(); got != "["+german+","+german+","+german+"]" {
		t.Fatalf("the catalog requests were %s, want one German request per open", got)
	}
	if errors := page.MustEval(`()=>JSON.stringify(fixtureErrors)`).Str(); errors != "[]" {
		t.Fatal(errors)
	}
}
