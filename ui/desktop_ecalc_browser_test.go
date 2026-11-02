package ui

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"sort"
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
// the search still finds items by their English name. Phase 2 adds: the units list of a variable, a chart (patched into
// the voltage divider), the dimmed fields of an `ignore` list (patched into the decibel calculator), the standard-value
// combiner (the real eseries_combiner), radix fields (a fixture calculator), the new symbols, the split schematic files and
// the launch hook (ECalcApp.launch on an open window, ctx.launch at render time, a payload that arrives while loading).
// Set AURAGO_BROWSER_ARTIFACT_DIR to keep screenshots.
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
	// Counts and drawings come from the catalog the route serves, so the test follows the content that later parts add.
	var served struct {
		Items []struct{ ID, Kind, Category, Schematic string } `json:"items"`
		Units struct {
			Dims []struct {
				ID       string
				Prefixes []string
				Units    []struct{ ID string }
			} `json:"dims"`
		} `json:"units"`
	}
	enBody, _, err := catalog.LocalizedJSON("en")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(enBody, &served); err != nil {
		t.Fatal(err)
	}
	type drawing struct{ ID, Category string }
	var drawings []drawing
	schematicIDs := map[string]bool{}
	basicsCount, voltageChoices := 0, 0
	for _, dim := range served.Units.Dims {
		if dim.ID == "voltage" {
			voltageChoices = 1 + len(dim.Prefixes) + len(dim.Units) // automatic, the fixed prefixes, the explicit units
		}
	}
	for _, item := range served.Items {
		if item.Category == "basics" {
			basicsCount++
		}
		if item.Kind == "calculator" && item.Schematic != "" {
			drawings = append(drawings, drawing{item.ID, item.Category})
			schematicIDs[item.Schematic] = true
		}
	}
	if !schematicIDs["voltage_divider"] || deItems["eseries_combiner"] == "" {
		t.Fatalf("the catalog lacks the voltage divider schematic or the eseries_combiner (Part A of phase 2)")
	}
	germanWords := readMergedLangMap(t, "de")
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
		// The patches that stand in for content the later parts write: a units list, a chart, an ignore list and a
		// calculator with radix fields (category passive, which holds no calculator yet).
		window.ecalcPatchCatalog=j=>{
			const item=id=>j.items.find(i=>i.id===id);
			item('voltage_divider').vars.find(v=>v.id==='vout').units=['','dBV'];
			item('voltage_divider').chart={x:'r2',y:'vout',to:'10*r2',samples:101};
			item('decibel_ratio').choices[0].options.find(o=>o.id==='power').ignore=['a2'];
			j.items.push({kind:'calculator',id:'fx_radix',category:'passive',name:'Radix fixture',name_en:'Radix fixture',doc:'',tags:[],source:'fixture',
				vars:[
					{id:'n',dim:'integer',label:'N',name:'N',default:255,default_list:null,unit:'hex',units:['dec','hex','bin','oct'],eseries:false,list:false,input:true},
					{id:'m',dim:'integer',label:'M',name:'M',default:null,default_list:null,unit:'dec',units:['dec','hex','bin','oct'],eseries:false,list:false,input:true}],
				choices:[],dof:1,rules:[{target:'m',alts:['n']},{target:'n',alts:['m']}],derived:[],checks:[],formula:['m = n'],schematic:'',sets:[['n'],['m']],finder:'',chart:null});
			return j;
		};
		const base=window.fetch;
		window.fetch=(url,options)=>{
			const text=String(url);
			if(!text.startsWith('/api/ecalc/catalog'))return base(url,options);
			window.ecalcCatalogRequests.push(text);
			window.ecalcCatalogCaches.push(String(options&&options.cache));
			if(window.ecalcCatalogDown)return Promise.resolve(new Response('{"error":"down"}',{status:500,headers:{'Content-Type':'application/json'}}));
			return base('/ecalc-catalog.json'+text.slice('/api/ecalc/catalog'.length)).then(r=>r.json()).then(j=>{
					j.items.find(i=>i.id==='voltage_divider').name_en='Zzyzx divider';
				window.ecalcPatchCatalog(j);
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
	// openIn picks the category on the rail first (the list shows one category), then the calculator.
	openIn := func(category, id string) {
		t.Helper()
		page.MustElement(`.ec-rail-btn[data-rail="` + category + `"]`).MustClick()
		openCalc(id)
	}
	// The chart of the open page as JSON: tick and title texts, the number of curve points, the dot, the heading, the name.
	chartInfo := `()=>{
		const block=document.querySelector('.ec-chart');
		if(!block||block.hidden)return 'hidden';
		const svg=block.querySelector('svg.ec-chart-svg');
		if(!svg)return 'no svg';
		const dot=svg.querySelector('.ec-chart-dot');
		return JSON.stringify({texts:[...svg.querySelectorAll('text')].map(e=>e.textContent),curve:(svg.querySelector('.ec-chart-curve').getAttribute('d').match(/[ML]/g)||[]).length,dot:dot?[dot.getAttribute('cx'),dot.getAttribute('cy')]:null,heading:block.querySelector('h3').textContent,label:svg.getAttribute('aria-label')});
	}`
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
	if got := page.MustEval(`()=>document.querySelectorAll('.ec-items .ec-item').length`).Int(); got != basicsCount {
		t.Fatalf("the basics category lists %d calculators, want the %d of the catalog", got, basicsCount)
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
	// The chart under the formula (patched into the divider): V_out over R_2 from 0 to ten times R_2, five ticks per axis
	// formatted by the units formatter, the curve of 101 points, the dot at the current point.
	wait(`()=>{const b=document.querySelector('.ec-chart');return !!b&&!b.hidden}`)
	wantChart := `{"texts":["0 Ω","0 V","5,5 kΩ","2,5 V","11 kΩ","5 V","16,5 kΩ","7,5 V","22 kΩ","10 V","R2","Vout"],"curve":101,"dot":["83","101.97"],"heading":"` + germanWords["desktop.ecalc_chart"] + `","label":"` + germanWords["desktop.ecalc_chart"] + `: Vout(R2)"}`
	if got := page.MustEval(chartInfo).Str(); got != wantChart {
		t.Fatalf("divider chart %s, want %s", got, wantChart)
	}
	if !page.MustEval(`()=>{const m=document.querySelector('.ec-math-card'),c=document.querySelector('.ec-chart'),f=document.querySelector('.ec-formulas');return m.contains(c)&&c.getBoundingClientRect().top>=f.getBoundingClientRect().bottom-1}`).Bool() {
		t.Fatal("the chart must sit under the formula, inside the formula card")
	}
	page.MustEval(`()=>document.querySelector('.ec-chart').scrollIntoView({block:'center'})`)
	shot("voltage-divider-chart")
	page.MustEval(`()=>{document.querySelector('.ec-page-host').scrollTop=0}`)
	typeInto("r2", "10k")
	wait(`()=>{const b=document.querySelector('.ec-chart svg');return !!b&&[...b.querySelectorAll('text')][8].textContent==='100 kΩ'}`)
	// The end of the range 10 x R_2 must be above zero: with R_2 = 0 there is nothing to draw, and the chart returns with a value.
	typeInto("r2", "0")
	wait(`()=>document.querySelector('.ec-chart').hidden`)
	page.MustElement(`[data-action="reset"]`).MustClick()
	wait(`()=>document.querySelector('.ec-field[data-var="r2"] .ec-input').value==='2,2 kΩ' && !document.querySelector('.ec-chart').hidden && [...document.querySelectorAll('.ec-chart svg text')][8].textContent==='22 kΩ'`)

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
	// The units list of V_out (patched: automatic and dBV) drops dBu and keeps the fixed prefixes; V_in has no list and offers both.
	options := `n=>{const o=[...document.querySelectorAll('.ec-field[data-var="'+n+'"] .ec-unit option')].map(e=>e.value);return o.length+':'+o.includes('')+o.includes('prefix:m')+o.includes('dBV')+o.includes('dBu');}`
	if got := page.MustEval(options, "vout").Str(); got != "8:truetruetruefalse" {
		t.Fatalf("the V_out dropdown is %s, want 8 choices with dBV and without dBu", got)
	}
	if got, want := page.MustEval(options, "vin").Str(), fmt.Sprintf("%d:truetruetruetrue", voltageChoices); got != want {
		t.Fatalf("the V_in dropdown is %s, want every choice of the voltage dimension (%s: with dBV and dBu)", got, want)
	}
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
	// The ten schematics files merged into one object, one entry per drawing the catalog names.
	wantDrawings := make([]string, 0, len(schematicIDs))
	for id := range schematicIDs {
		wantDrawings = append(wantDrawings, id)
	}
	sort.Strings(wantDrawings)
	if got := page.MustEval(`()=>JSON.stringify(Object.keys(window.ECalcSchematics).sort())`).Str(); got != `["`+strings.Join(wantDrawings, `","`)+`"]` {
		t.Fatalf("window.ECalcSchematics holds %s, want the drawings %v of the catalog", got, wantDrawings)
	}
	for _, d := range drawings {
		id := d.ID
		openIn(d.Category, id)
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
	openIn("basics", "wheatstone")
	setUnit("vd", "dBV")
	wait(`()=>document.querySelector('.ec-field[data-var="vd"] .ec-input').value==='-119 mV'`)
	setUnit("vd", "")
	openCalc("decibel_ratio")
	page.MustElement(`.ec-choice input[value="power"]`).MustClick()
	wait(`()=>document.querySelector('.ec-field[data-var="g"] .ec-input').value==='10 dB'`)

	// ---- ignore: the option "power" (patched) does not use A_2 ---------------------------------------------------------------------
	// The field is dimmed with the hint as its title, never takes the computed tint although A_2 is the computed value of
	// this set, and stays out of the copied results.
	wait(`()=>document.querySelector('.ec-field[data-var="a2"]').dataset.unused==='true'`)
	if got := page.MustEval(`()=>document.querySelector('.ec-field[data-var="a2"]').title`).Str(); got != germanWords["desktop.ecalc_unused_field"] {
		t.Fatalf("the unused field has the title %q, want %q", got, germanWords["desktop.ecalc_unused_field"])
	}
	if !page.MustEval(`()=>Number(getComputedStyle(document.querySelector('.ec-field[data-var="a2"]')).opacity)<0.9&&Number(getComputedStyle(document.querySelector('.ec-field[data-var="a1"]')).opacity)===1`).Bool() {
		t.Fatal("the unused field must be dimmed and the used ones not")
	}
	typeInto("a1", "2")
	wait(`()=>document.querySelector('.ec-field[data-var="a1"] .ec-input').value==='2'`)
	typeInto("g", "20")
	wait(`()=>document.querySelector('.ec-field[data-var="a2"] .ec-input').value==='200'`)
	if state("a2") != "input" || state("a1") != "input" || state("g") != "input" {
		t.Fatalf("with A_2 unused the states are a1=%s a2=%s g=%s, want inputs only (A_2 is computed but never tinted)", state("a1"), state("a2"), state("g"))
	}
	shot("decibel-unused-field")
	page.MustEval(`()=>{window.copiedText='';Object.defineProperty(navigator,'clipboard',{configurable:true,value:{writeText:text=>{window.copiedText=text;return Promise.resolve();}}});}`)
	page.MustElement(`[data-action="copy"]`).MustClick()
	wait(`()=>window.copiedText.length>0`)
	if got := page.MustEval(`()=>window.copiedText`).Str(); !strings.Contains(got, "A_1 = ") || !strings.Contains(got, "G = ") || strings.Contains(got, "A_2") {
		t.Fatalf("the copied results are %q, want A_1 and G but not the unused A_2", got)
	}
	page.MustElement(`.ec-choice input[value="voltage"]`).MustClick()
	wait(`()=>document.querySelector('.ec-field[data-var="a2"]').dataset.unused==='false' && document.querySelector('.ec-field[data-var="a2"]').dataset.state==='computed' && !document.querySelector('.ec-field[data-var="a2"]').hasAttribute('title')`)

	// ---- the standard-value combiner (finder: combine) -----------------------------------------------------------------------------
	openIn("basics", "eseries_combiner")
	wait(`()=>!!document.querySelector('.ec-combos:not([hidden]) .ec-combo-table')`)
	combos := `()=>{
		const b=document.querySelector('.ec-combos');
		const rows=k=>[...b.querySelectorAll('tbody[data-kind="'+k+'"] tr:not(.ec-combo-group)')].map(r=>[...r.children].map(c=>c.textContent));
		return JSON.stringify({title:b.querySelector('h3').textContent,heads:[...b.querySelectorAll('thead th')].map(c=>c.textContent),groups:[...b.querySelectorAll('tbody')].map(x=>x.querySelector('.ec-combo-group th').textContent),single:rows('single'),series:rows('series'),parallel:rows('parallel')});
	}`
	var shown struct {
		Title    string
		Heads    []string
		Groups   []string
		Single   [][]string
		Series   [][]string
		Parallel [][]string
	}
	readCombos := func() {
		t.Helper()
		if err := json.Unmarshal([]byte(page.MustEval(combos).Str()), &shown); err != nil {
			t.Fatal(err)
		}
	}
	readCombos()
	wantTitle := strings.ReplaceAll(germanWords["desktop.ecalc_combos"], "{series}", "E24")
	if shown.Title != wantTitle || len(shown.Heads) != 3 || shown.Heads[0] != wantTitle || shown.Heads[1] != germanWords["desktop.ecalc_combo_value"] || shown.Heads[2] != germanWords["desktop.ecalc_combo_deviation"] {
		t.Fatalf("combiner heading and columns are %q %q, want %q, %q, %q", shown.Title, shown.Heads, wantTitle, germanWords["desktop.ecalc_combo_value"], germanWords["desktop.ecalc_combo_deviation"])
	}
	if strings.Join(shown.Groups, "|") != germanWords["desktop.ecalc_combo_single"]+"|"+germanWords["desktop.ecalc_combo_series"]+"|"+germanWords["desktop.ecalc_combo_parallel"] {
		t.Fatalf("combiner groups are %q", shown.Groups)
	}
	// The default target is 5 kΩ; the nearest E24 value is 5.1 kΩ (+2.0 %). Five series and five parallel combinations follow,
	// each group ordered by the size of its deviation.
	if len(shown.Single) != 1 || shown.Single[0][1] != "5,1 kΩ" || !strings.HasPrefix(shown.Single[0][2], "+2,0") || len(shown.Series) != 5 || len(shown.Parallel) != 5 {
		t.Fatalf("combiner rows: single %q, %d series, %d parallel; want 5,1 kΩ +2,0 %% and five of each", shown.Single, len(shown.Series), len(shown.Parallel))
	}
	size := func(text string) float64 {
		var number float64
		if _, err := fmt.Sscanf(strings.NewReplacer(",", ".", "−", "-", "+", "").Replace(text), "%f", &number); err != nil {
			t.Fatalf("cannot read the deviation %q", text)
		}
		return max(number, -number)
	}
	for _, rows := range [][][]string{shown.Series, shown.Parallel} {
		for i := 1; i < len(rows); i++ {
			if size(rows[i][2]) < size(rows[i-1][2]) {
				t.Fatalf("combinations %q must be ordered by the size of the deviation", rows)
			}
		}
	}
	if !page.MustEval(`()=>document.querySelector('.ec-combos').getBoundingClientRect().top>=document.querySelector('.ec-stage').getBoundingClientRect().bottom-1`).Bool() {
		t.Fatal("the combinations belong below the fields")
	}
	shot("combiner")
	// The E series of the settings decides: E12 has no 5.1 kΩ, so 4.7 kΩ (-6.0 %) is the single value.
	page.MustElement(`[data-action="settings"]`).MustClick()
	page.MustEval(`()=>{const s=document.querySelector('.ec-pop [data-set="eseries"]');s.value='E12';s.dispatchEvent(new Event('change',{bubbles:true}));}`)
	wait(`()=>document.querySelector('.ec-combos h3').textContent.includes('E12')`)
	readCombos()
	if len(shown.Single) != 1 || shown.Single[0][1] != "4,7 kΩ" {
		t.Fatalf("with E12 the single value is %q, want 4,7 kΩ", shown.Single)
	}
	// The finest series costs the longest search (a few hundred ms): the block follows the series after a pause.
	page.MustEval(`()=>{const s=document.querySelector('.ec-pop [data-set="eseries"]');s.value='E192';s.dispatchEvent(new Event('change',{bubbles:true}));}`)
	wait(`()=>document.querySelector('.ec-combos h3').textContent.includes('E192')`)
	readCombos()
	if len(shown.Single) != 1 || len(shown.Series) != 5 || len(shown.Parallel) != 5 {
		t.Fatalf("with E192 the combiner lists %d single, %d series and %d parallel rows, want 1, 5 and 5", len(shown.Single), len(shown.Series), len(shown.Parallel))
	}
	page.MustEval(`()=>{const s=document.querySelector('.ec-pop [data-set="eseries"]');s.value='E24';s.dispatchEvent(new Event('change',{bubbles:true}));}`)
	wait(`()=>document.querySelector('.ec-combos h3').textContent.includes('E24')`)
	page.MustElement(`[data-action="settings"]`).MustClick()
	// A target that is a standard value: single and best series pair are exact (2.7 kΩ + 2 kΩ).
	typeInto("target", "4k7")
	wait(`()=>document.querySelector('.ec-combos tbody[data-kind="single"] .ec-row-val').textContent==='4,7 kΩ'`)
	readCombos()
	if !strings.HasPrefix(shown.Single[0][2], "0,0") || !strings.HasPrefix(shown.Series[0][2], "0,0") {
		t.Fatalf("for 4k7 the single value %q and the best series pair %q should be exact", shown.Single[0], shown.Series[0])
	}

	// ---- radix fields: the fixture calculator with integer fields ---------------------------------------------------------------------
	openIn("passive", "fx_radix")
	wait(`()=>document.querySelector('.ec-field[data-var="n"] .ec-input').value==='0xFF'`)
	if value("m") != "255" || state("m") != "computed" || state("n") != "input" {
		t.Fatalf("radix defaults: n %q (%s), m %q (%s); want 0xFF as input and 255 computed", value("n"), state("n"), value("m"), state("m"))
	}
	if got := page.MustEval(`()=>document.querySelector('.ec-field[data-var="n"] .ec-input').getAttribute('inputmode')`).Str(); got != "text" {
		t.Fatalf("a radix field has inputmode %q, want text (hex digits need the letter keyboard)", got)
	}
	typeInto("n", "0x10")
	wait(`()=>document.querySelector('.ec-field[data-var="m"] .ec-input').value==='16'`)
	typeInto("m", "300")
	wait(`()=>document.querySelector('.ec-field[data-var="n"] .ec-input').value==='0x12C' && document.querySelector('.ec-field[data-var="n"]').dataset.state==='computed'`)
	setUnit("n", "bin")
	wait(`()=>document.querySelector('.ec-field[data-var="n"] .ec-input').value==='0b1_00101100'`)
	setUnit("n", "oct")
	wait(`()=>document.querySelector('.ec-field[data-var="n"] .ec-input').value==='0o454'`)
	setUnit("n", "hex")
	typeInto("n", "dead_beef")
	wait(`()=>document.querySelector('.ec-field[data-var="m"] .ec-input').value==='3735928559'`)
	page.MustElement(field("m") + " .ec-input").MustClick()
	wait(`()=>document.querySelector('.ec-field[data-var="n"] .ec-input').value==='0xDEAD_BEEF'`)
	// Switching the unit of an input rewrites its text, so the new unit reads it the same way.
	setUnit("n", "dec")
	wait(`()=>document.querySelector('.ec-field[data-var="n"] .ec-input').value==='3735928559'`)
	typeInto("n", "0xZZ")
	wait(`()=>document.querySelector('.ec-field[data-var="n"]').dataset.state==='error'`)
	page.MustElement(field("m") + " .ec-input").MustClick()
	wait(`()=>document.querySelector('.ec-field[data-var="n"]').dataset.state!=='error'`)

	// ---- the new symbols: pins, both symbol standards, and a drawing for the eye ---------------------------------------------------------
	wantSymbols := `[{"a":[0,0],"b":[3,0]},{"a":[0,0],"b":[4,0],"w":[2,-2]},[4,3],[1,4],true,true,true,true]`
	if got := page.MustEval(`()=>{const S=window.ECalcSymbols;
		const iec=S.svg({w:12,h:8,parts:[{sym:'pot',x:2,y:5},{sym:'switch',x:7,y:5}]},{standard:'iec'});
		const ansi=S.svg({w:12,h:8,parts:[{sym:'pot',x:2,y:5}]},{standard:'ansi'});
		return JSON.stringify([S.pins('switch'),S.pins('pot'),S.pinPoint({sym:'pot',x:2,y:5},'w'),S.pinPoint({sym:'switch',x:1,y:1,rot:90},'b'),S.names().includes('switch'),S.names().includes('pot'),iec.includes('<rect x="10" y="-7" width="60"')&&iec.includes('<circle cx="15"'),ansi.includes('L16 -8L28 8L40 -8')]);}`).Str(); got != wantSymbols {
		t.Fatalf("symbols: %s, want %s", got, wantSymbols)
	}
	page.MustEval(`()=>{const h=document.createElement('div');h.id='fx-symbols';h.style.cssText='position:fixed;right:8px;bottom:8px;width:360px;z-index:99999;color:CanvasText;background:Canvas';
		h.innerHTML=window.ECalcSymbols.svg({w:12,h:8,parts:[{sym:'pot',x:2,y:5},{sym:'switch',x:7,y:5}],wires:[[[0,5],[2,5]],[[6,5],[7,5]],[[10,5],[12,5]],[[4,3],[4,1]]]},{standard:'iec'})+window.ECalcSymbols.svg({w:12,h:8,parts:[{sym:'pot',x:2,y:5}],wires:[[[0,5],[2,5]]]},{standard:'ansi'});
		document.body.appendChild(h);}`)
	shot("symbols-pot-switch")
	page.MustEval(`()=>document.getElementById('fx-symbols').remove()`)
	page.MustElement(`.ec-rail-btn[data-rail="basics"]`).MustClick()

	// ---- the launch hook: ECalcApp.launch applies a payload to the open window ------------------------------------------------------------
	winID := page.MustEval(`()=>[...aurora.state.windows.keys()][0]`).Str()
	launch := func(payload string) bool {
		t.Helper()
		return page.MustEval(`(id,payload)=>window.ECalcApp.launch(id,JSON.parse(payload))`, winID, payload).Bool()
	}
	// Values are entered as if typed, in the given order: V_out after V_in leaves R_2 at its default and makes R_1 the computed value.
	if !launch(`{"page":"voltage_divider","values":{"vin":"9V","vout":"3V"}}`) {
		t.Fatal("launch refused a payload for the open window")
	}
	wait(`()=>document.querySelector('.ec-page')?.dataset.calc==='voltage_divider' && document.querySelector('.ec-field[data-var="r1"]').dataset.state==='computed'`)
	if got := []string{value("vin"), value("vout"), value("r2"), value("r1")}; got[0] != "9 V" || got[1] != "3 V" || got[2] != "2,2 kΩ" || got[3] != "4,4 kΩ" {
		t.Fatalf("launched divider %v, want 9 V, 3 V, 2,2 kΩ and a computed 4,4 kΩ", got)
	}
	stored := page.MustEval(`()=>{const e=JSON.parse(localStorage.getItem('aurago.ecalc.v1')).values.voltage_divider;return JSON.stringify({vin:e.inputs.vin,vout:e.inputs.vout,keys:Object.keys(e.inputs).sort(),tail:e.recency.slice(-2),locked:e.locked});}`).Str()
	if stored != `{"vin":"9V","vout":"3V","keys":["r2","vin","vout"],"tail":["vin","vout"],"locked":[]}` {
		t.Fatalf("the launched inputs are stored as %s, want the raw texts, the given order and no locks", stored)
	}
	// Unknown variables, choices and unreadable texts are skipped; the inputs that are not given start from their defaults.
	if !launch(`{"page":"voltage_divider","values":{"vin":"5V","bogus":"1","r1":"abc"},"choices":{"zz":"x"}}`) {
		t.Fatal("launch refused a payload with unknown names")
	}
	wait(`()=>document.querySelector('.ec-field[data-var="vin"] .ec-input').value==='5 V'`)
	if got := []string{value("r1"), value("r2"), value("vout")}; got[0] != "4,7 kΩ" || got[1] != "2,2 kΩ" || got[2] != "1,594 V" {
		t.Fatalf("launch with unknown names: %v, want the defaults 4,7 kΩ and 2,2 kΩ with 1,594 V", got)
	}
	// An unknown page changes nothing; a choice option that does not exist keeps the default option.
	launch(`{"page":"no_such_page","values":{"vin":"1V"}}`)
	if got := page.MustEval(`()=>document.querySelector('.ec-page').dataset.calc`).Str(); got != "voltage_divider" {
		t.Fatalf("an unknown page id opened %q", got)
	}
	launch(`{"page":"decibel_ratio","choices":{"qty":"bogus"},"values":{"a1":"1","a2":"100"}}`)
	wait(`()=>document.querySelector('.ec-page')?.dataset.calc==='decibel_ratio' && document.querySelector('.ec-field[data-var="g"] .ec-input').value==='40 dB'`)
	launch(`{"page":"decibel_ratio","choices":{"qty":"power"},"values":{"a1":"1","a2":"100"}}`)
	wait(`()=>document.querySelector('.ec-field[data-var="g"] .ec-input').value==='20 dB' && document.querySelector('.ec-choice input[value="power"]').checked`)
	// A page id alone just opens the page and keeps what it holds (star-delta still has the R_a typed earlier).
	launch(`{"page":"star_delta"}`)
	wait(`()=>document.querySelector('.ec-page')?.dataset.calc==='star_delta' && document.querySelector('.ec-field[data-var="ra"] .ec-input').value==='12 Ω'`)
	if page.MustEval(`()=>window.ECalcApp.launch('no-such-window',{page:'ohms_law'})`).Bool() || page.MustEval(`(id)=>window.ECalcApp.launch(id,null)`, winID).Bool() {
		t.Fatal("launch must refuse an unknown window and an empty payload")
	}
	page.MustElement(`.ec-rail-btn[data-rail="basics"]`).MustClick()
	openCalc("voltage_divider")
	page.MustElement(`[data-action="reset"]`).MustClick()
	wait(`()=>document.querySelector('.ec-field[data-var="vin"] .ec-input').value==='12 V'`)

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
	page.MustElement(`.ec-rail-btn[data-rail="basics"]`).MustClick()
	wait(fmt.Sprintf(`()=>document.querySelectorAll('.ec-items .ec-item').length===%d`, basicsCount))
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
	if !visible(".ec-chart svg") {
		t.Fatal("the chart must show on the phone-sized page")
	}
	page.MustEval(`()=>document.querySelector('.ec-chart').scrollIntoView({block:'center'})`)
	shot("compact-chart")
	// The combiner on the phone: the table stays inside the page.
	page.MustElement(".ec-back").MustClick()
	page.MustElement(`.ec-item[data-id="eseries_combiner"]`).MustClick()
	wait(`()=>document.querySelector('.ec-page')?.dataset.calc==='eseries_combiner' && !!document.querySelector('.ec-combos:not([hidden]) .ec-combo-table')`)
	if got := page.MustEval(`()=>{const p=document.querySelector('.ec-page-host'),t=document.querySelector('.ec-combo-wrap');return (p.scrollWidth>p.clientWidth+1?'the page scrolls sideways; ':'')+(t.scrollWidth>t.clientWidth+1?'the table scrolls sideways':'');}`).Str(); got != "" {
		t.Fatal(got)
	}
	shot("compact-combiner")

	// Three opens (the failing one, its retry and the compact window) made exactly three German catalog requests.
	const german = `"/api/ecalc/catalog?lang=de"`
	if got := page.MustEval(`()=>JSON.stringify(window.ecalcCatalogRequests)`).Str(); got != "["+german+","+german+","+german+"]" {
		t.Fatalf("the catalog requests were %s, want one German request per open", got)
	}

	// ---- ctx.launch at render time, and a payload that arrives while the catalog is still loading ---------------------------------------------------
	page.MustSetViewport(1366, 900, 1, false)
	page.MustEval(`()=>[...aurora.state.windows.keys()].forEach(id=>aurora.closeWindow(id))`)
	page.MustEval(`async()=>{
		const api=(url,options)=>fetch(url,options).then(r=>r.json());
		const mount=id=>{const h=document.createElement('div');h.id=id;h.style.cssText='position:fixed;left:0;top:0;width:1000px;height:640px;z-index:99999;color:CanvasText;background:Canvas';document.body.appendChild(h);return h;};
		window.ECalcApp.render(mount('fx-launch'),'fx-launch',{lang:'de',t:window.t,api,launch:{page:'decibel_ratio',values:{a1:'2',a2:'200'},choices:{qty:'power'}}});
		let release;
		const gate=new Promise(resolve=>{release=resolve;});
		window.ECalcApp.render(mount('fx-queued'),'fx-queued',{lang:'de',t:window.t,api:(url,options)=>gate.then(()=>api(url,options))});
		window.fixtureQueued=window.ECalcApp.launch('fx-queued',{page:'voltage_divider',values:{vin:'5V'}});
		setTimeout(release,250);
	}`)
	wait(`()=>{const p=document.querySelector('#fx-launch .ec-page');return !!p&&p.dataset.calc==='decibel_ratio'&&p.querySelector('.ec-field[data-var="g"] .ec-input').value==='20 dB'&&p.querySelector('.ec-choice input[value="power"]').checked}`)
	wait(`()=>{const p=document.querySelector('#fx-queued .ec-page');return !!p&&p.dataset.calc==='voltage_divider'&&p.querySelector('.ec-field[data-var="vin"] .ec-input').value==='5 V'}`)
	if !page.MustEval(`()=>window.fixtureQueued`).Bool() {
		t.Fatal("launch must accept a payload for a window whose catalog is still loading")
	}
	page.MustEval(`()=>{for(const id of ['fx-launch','fx-queued']){window.ECalcApp.dispose(id);document.getElementById(id).remove();}}`)

	if errors := page.MustEval(`()=>JSON.stringify(fixtureErrors)`).Str(); errors != "[]" {
		t.Fatal(errors)
	}
}
