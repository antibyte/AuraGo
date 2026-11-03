package ui

import (
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"aurago/internal/ecalc"

	"github.com/go-rod/rod/lib/input"
)

// The code pages of ECalc (phase 3, part UK): resistor and inductor colour bands, SMD resistor, capacitor and ceramic
// temperature codes. The tests at the top and at the end are source tests (they always run): the 45 translation keys in 16
// languages, the DOM-free drawing builders, the stylesheet section, the loader order, the use of the codes engine and of the
// store. The tests in the middle drive the pages inside the real desktop shell (AURAGO_RUN_BROWSER_SMOKE=1).

// ecalcCodeKeys are the strings of the code pages (desktop.ecalc_code_*); the page names every one of them as a literal.
var ecalcCodeKeys = []string{
	"desktop.ecalc_code_auto", "desktop.ecalc_code_band_aria", "desktop.ecalc_code_bands", "desktop.ecalc_code_class_1", "desktop.ecalc_code_class_2",
	"desktop.ecalc_code_colors", "desktop.ecalc_code_common", "desktop.ecalc_code_empty", "desktop.ecalc_code_error_invalid_code",
	"desktop.ecalc_code_error_invalid_value", "desktop.ecalc_code_error_unsupported", "desktop.ecalc_code_field_change",
	"desktop.ecalc_code_field_change_minus", "desktop.ecalc_code_field_change_plus", "desktop.ecalc_code_field_class", "desktop.ecalc_code_field_code_3digit",
	"desktop.ecalc_code_field_code_4digit", "desktop.ecalc_code_field_code_eia96", "desktop.ecalc_code_field_family", "desktop.ecalc_code_field_industry_code",
	"desktop.ecalc_code_field_layout", "desktop.ecalc_code_field_rkm", "desktop.ecalc_code_field_tc", "desktop.ecalc_code_field_tcr",
	"desktop.ecalc_code_field_temp_high", "desktop.ecalc_code_field_temp_low", "desktop.ecalc_code_field_tolerance", "desktop.ecalc_code_field_tolerance_minus",
	"desktop.ecalc_code_field_tolerance_plus", "desktop.ecalc_code_field_value_code", "desktop.ecalc_code_field_voltage", "desktop.ecalc_code_hint",
	"desktop.ecalc_code_marking", "desktop.ecalc_code_menu", "desktop.ecalc_code_none", "desktop.ecalc_code_notes", "desktop.ecalc_code_print",
	"desktop.ecalc_code_result", "desktop.ecalc_code_role_digit", "desktop.ecalc_code_role_identifier", "desktop.ecalc_code_role_multiplier",
	"desktop.ecalc_code_role_point", "desktop.ecalc_code_role_tcr", "desktop.ecalc_code_role_tolerance", "desktop.ecalc_code_role_value",
}

var ecalcCodePlaceholders = map[string][]string{
	"desktop.ecalc_code_band_aria": {"{n}", "{role}", "{color}"},
	"desktop.ecalc_code_menu":      {"{n}"},
}

func TestECalcCodeLocaleKeys(t *testing.T) {
	t.Parallel()
	if len(ecalcCodeKeys) != 45 {
		t.Errorf("%d code keys are listed, want the 45 of the plan", len(ecalcCodeKeys))
	}
	for _, lang := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		words := readMergedLangMap(t, lang)
		for _, key := range ecalcCodeKeys {
			if strings.TrimSpace(words[key]) == "" {
				t.Errorf("%s misses %s", lang, key)
			}
		}
		for key, placeholders := range ecalcCodePlaceholders {
			for _, placeholder := range placeholders {
				if !strings.Contains(words[key], placeholder) {
					t.Errorf("%s: %s must keep the %s placeholder: %q", lang, key, placeholder, words[key])
				}
			}
		}
	}
}

// The drawings are string builders like the chart: no document access, and no colour in the code except the hex that the
// caller passes in (the hex of the data) after the check in safeColor.
func TestECalcCodeDrawIsADomFreeStringBuilder(t *testing.T) {
	t.Parallel()
	draw := ecalcScripts(t)["ecalc-code-draw.js"]
	for _, marker := range []string{
		"window.ECalcCodeDraw = { bandsSVG, chipSVG, safeColor, POSITIONS };", "function bandsSVG(spec)", "function chipSVG(spec)", "const safeColor =",
		`class="ec-code-art"`, `class="ec-band"`, `class="ec-band-hit"`, `class="ec-band-fill`, `class="ec-band-ring"`, `class="ec-draw-body"`, `class="ec-chip-body"`, `class="ec-chip-text"`,
		`aria-haspopup="listbox"`, `aria-expanded=`, "tabindex=\"0\" role=\"button\"", "resistor: { 3:", "inductor: { 4:", "const colour = safeColor(band.color)",
	} {
		if !strings.Contains(draw, marker) {
			t.Errorf("ecalc-code-draw.js is missing %q", marker)
		}
	}
	body := regexp.MustCompile(`(?s)/\*.*?\*/`).ReplaceAllString(draw, "")
	for label, pattern := range map[string]string{
		"document access": `\bdocument\b`,
		"window access":   `\bwindow\.(?:document|ECalcCore|localStorage)\b`,
		"DOM write":       `\b(?:innerHTML|createElement|appendChild|setAttribute)\b`,
		"hex colour":      `#[0-9a-fA-F]{3,8}\b`,
		"colour function": `\b(?:rgba?|hsla?)\(`,
		"inline style":    `\bstyle=`,
	} {
		if match := regexp.MustCompile(pattern).FindString(body); match != "" {
			t.Errorf("ecalc-code-draw.js must not contain a %s (%q)", label, match)
		}
	}
}

func TestECalcCodeStylesheetSection(t *testing.T) {
	t.Parallel()
	css := readDesktopAssetText(t, "css/desktop-app-ecalc.css")
	start, end := strings.Index(css, "/* code pages */"), strings.LastIndex(css, "/* end of code pages */")
	if start < 0 || end < start {
		t.Fatal("css/desktop-app-ecalc.css has no /* code pages */ ... /* end of code pages */ section")
	}
	if strings.TrimSpace(css[end+len("/* end of code pages */"):]) != "" {
		t.Error("the code pages section is the last section of css/desktop-app-ecalc.css")
	}
	section := css[start:end]
	for _, marker := range []string{
		".ec-code-menu {", ".ec-code-opt[aria-selected=\"true\"]", ".ec-band:focus-visible .ec-band-ring", `.ec-band[aria-expanded="true"] .ec-band-ring`, ".ec-code-error {",
		".ec-chip-text { fill: var(--vd-theme-panel-bg-strong", ".ec-chip-body { fill: var(--vd-text)", ".ec-draw-body {", ".ec-code-row dt", "@media (prefers-reduced-motion: no-preference)",
	} {
		if !strings.Contains(section, marker) {
			t.Errorf("the code pages section is missing %q", marker)
		}
	}
	// The colour of a band is the fill attribute of the drawing, never a rule of the stylesheet.
	if regexp.MustCompile(`\.ec-band-fill\s*\{[^}]*\bfill\s*:`).MatchString(section) {
		t.Error("the stylesheet must not set the fill of a band: the colour comes from the data")
	}
}

// ---- the pages in the desktop shell ------------------------------------------------------------------------------------------

// ecalcCodeColors returns {code item id: {colour id: hex}} of the served catalog for the band kinds.
func ecalcCodeColors(t testing.TB, lang string) map[string]map[string]string {
	t.Helper()
	catalog, err := ecalc.Default()
	if err != nil {
		t.Fatal(err)
	}
	body, _, err := catalog.LocalizedJSON(lang)
	if err != nil {
		t.Fatal(err)
	}
	var parsed struct {
		Items []struct {
			ID, Kind, Code string
			Data           struct {
				Colors []struct{ ID, Name, Hex string } `json:"colors"`
			} `json:"data"`
		} `json:"items"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		t.Fatal(err)
	}
	out := map[string]map[string]string{}
	for _, item := range parsed.Items {
		if item.Kind != "code" || len(item.Data.Colors) == 0 {
			continue
		}
		out[item.ID] = map[string]string{}
		for _, color := range item.Data.Colors {
			out[item.ID][color.ID] = color.Hex
		}
	}
	if len(out) != 2 {
		t.Fatalf("the catalog has %d code items with colours, want the resistor and the inductor", len(out))
	}
	return out
}

// ---- the pages in the desktop shell ------------------------------------------------------------------------------------------

// codePageState is the code page of the window as the tests read it.
type codePageState struct {
	ID        string            `json:"id"`
	Kind      string            `json:"kind"`
	State     string            `json:"state"`
	Art       string            `json:"art"`
	Count     string            `json:"count"`
	Valid     string            `json:"valid"`
	Bands     []codeBandState   `json:"bands"`
	Chip      []string          `json:"chip"`
	Inputs    map[string]string `json:"inputs"`
	Rows      [][]string        `json:"rows"`
	Prints    []string          `json:"prints"`
	Warnings  []string          `json:"warnings"`
	Error     string            `json:"error"`
	ErrorCode string            `json:"errorCode"`
	Invalid   []string          `json:"invalid"`
	Panels    []string          `json:"panels"`
	Classes   string            `json:"classes"`
}

type codeBandState struct {
	Role     string `json:"role"`
	Color    string `json:"color"`
	Fill     string `json:"fill"`
	Caption  string `json:"caption"`
	Expanded string `json:"expanded"`
	Label    string `json:"label"`
}

const codePageStateJS = `()=>{
	const p=document.querySelector('.ec-code-page');
	if(!p)return 'null';
	const q=s=>p.querySelector(s),art=q('.ec-code-art');
	const radio=p.querySelector('input[name$="-class"]:checked');
	return JSON.stringify({
		id:p.dataset.code,kind:p.dataset.kind,state:p.dataset.state,art:art?art.dataset.art:'',count:art?art.dataset.bands||'':'',valid:art?art.dataset.valid||'':'',
		bands:[...p.querySelectorAll('.ec-band')].map(b=>({role:b.dataset.role,color:b.dataset.color,fill:b.querySelector('.ec-band-fill').getAttribute('fill'),caption:(b.querySelector('.ec-draw-cap')||{}).textContent||'',expanded:b.getAttribute('aria-expanded'),label:b.getAttribute('aria-label')})),
		chip:[...p.querySelectorAll('.ec-chip-text,.ec-disc-text')].map(x=>x.textContent),
		inputs:Object.fromEntries([...p.querySelectorAll('[data-input]')].map(i=>[i.dataset.input,i.value])),
		rows:[...p.querySelectorAll('.ec-code-row')].map(r=>[r.querySelector('dt').textContent,r.querySelector('dd').textContent]),
		prints:[...p.querySelectorAll('.ec-code-print')].map(b=>b.textContent+':'+b.getAttribute('aria-pressed')),
		warnings:[...p.querySelectorAll('.ec-warnings li')].map(l=>l.textContent),
		error:q('.ec-code-error').hidden?'':q('.ec-code-error').textContent,errorCode:q('.ec-code-error').dataset.code,
		invalid:[...p.querySelectorAll('.ec-code-field[data-state="error"]')].map(f=>f.dataset.field),
		panels:[...p.querySelectorAll('.ec-code-class')].filter(x=>!x.hidden).map(x=>x.dataset.class),classes:radio?radio.value:''
	});
}`

type codesHarness struct {
	*ecalcBrowserFixture
	t      *testing.T
	colors map[string]map[string]string // code item id -> colour id -> hex
	names  map[string]map[string]string // code item id -> colour id -> name in the language of the shell
}

func newCodesHarness(t *testing.T, patch string) *codesHarness {
	t.Helper()
	f := newECalcBrowserFixture(t, patch)
	h := &codesHarness{ecalcBrowserFixture: f, t: t, colors: ecalcCodeColors(t, "de"), names: map[string]map[string]string{}}
	body, _, err := f.Catalog.LocalizedJSON("de")
	if err != nil {
		t.Fatal(err)
	}
	var parsed struct {
		Items []struct {
			ID   string
			Data struct {
				Colors []struct{ ID, Name string } `json:"colors"`
			} `json:"data"`
		} `json:"items"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		t.Fatal(err)
	}
	for _, item := range parsed.Items {
		h.names[item.ID] = map[string]string{}
		for _, color := range item.Data.Colors {
			h.names[item.ID][color.ID] = color.Name
		}
	}
	f.OpenApp(`.ec-page[data-calc]`)
	f.Wait(`()=>!!document.querySelector('.ec-rail-btn[data-rail="codes"]')`)
	return h
}

func (h *codesHarness) read() codePageState {
	h.t.Helper()
	raw := h.Page.MustEval(codePageStateJS).Str()
	if raw == "null" {
		h.t.Fatal("no code page is open")
	}
	var state codePageState
	if err := json.Unmarshal([]byte(raw), &state); err != nil {
		h.t.Fatalf("code page state %s: %v", raw, err)
	}
	return state
}

func (h *codesHarness) open(id string) {
	h.t.Helper()
	h.Page.MustEval(`()=>document.querySelector('.ec-rail-btn[data-rail="codes"]').click()`)
	h.Page.MustElement(`.ec-item[data-id="` + id + `"]`).MustClick()
	h.Wait(`()=>document.querySelector('.ec-page')?.dataset.code==='` + id + `'`)
}

func (h *codesHarness) typeInto(selector, text string) {
	h.t.Helper()
	el := h.Page.MustElement(selector)
	el.MustClick()
	el.MustSelectAllText()
	el.MustInput(text)
}

func (h *codesHarness) choose(selector, value string) {
	h.t.Helper()
	h.Page.MustEval(`(s,v)=>{const e=document.querySelector(s);e.value=v;e.dispatchEvent(new Event('change',{bubbles:true}));}`, selector, value)
}

func (h *codesHarness) radio(name, value string) {
	h.t.Helper()
	h.Page.MustEval(`(n,v)=>{const r=document.querySelector('.ec-code-page input[type="radio"][name$="-'+n+'"][value="'+v+'"]');r.checked=true;r.dispatchEvent(new Event('change',{bubbles:true}));}`, name, value)
}

// colorsOf lists the colour ids of the drawn bands (the hex of every band is looked up in the data).
func (h *codesHarness) colorsOf(id string, state codePageState) []string {
	h.t.Helper()
	var out []string
	for _, band := range state.Bands {
		found := ""
		for color, name := range h.names[id] {
			if name == band.Color {
				found = color
			}
		}
		if found == "" {
			h.t.Fatalf("band colour %q is no colour of %s", band.Color, id)
		}
		if h.colors[id][found] != band.Fill {
			h.t.Fatalf("band %q is drawn with %q, the data says %q", band.Color, band.Fill, h.colors[id][found])
		}
		out = append(out, found)
	}
	return out
}

// waitColors waits until the drawn bands have the hex colours of the data for the given colour ids.
func (h *codesHarness) waitColors(id string, colors ...string) {
	h.t.Helper()
	var hex []string
	for _, color := range colors {
		hex = append(hex, h.colors[id][color])
	}
	want, _ := json.Marshal(hex)
	h.Wait(`()=>JSON.stringify([...document.querySelectorAll('.ec-band-fill')].map(b=>b.getAttribute('fill')))==='` + string(want) + `'`)
}

func (h *codesHarness) same(label string, got, want any) {
	h.t.Helper()
	if !reflect.DeepEqual(got, want) {
		h.t.Fatalf("%s: got %v, want %v", label, got, want)
	}
}

func (h *codesHarness) row(state codePageState, name string) string {
	for _, row := range state.Rows {
		if row[0] == name {
			return row[1]
		}
	}
	return "<no row " + name + ">"
}

// setDigits changes the significant digits in the settings of the app (the shell then asks the open page to draw again).
func (h *codesHarness) setDigits(digits string) {
	h.t.Helper()
	h.Page.MustEval(`n=>{const s=document.querySelector('.ec-pop [data-set="sig"]');s.value=n;s.dispatchEvent(new Event('change',{bubbles:true}));}`, digits)
}

func (h *codesHarness) noErrors() {
	h.t.Helper()
	if errors := h.Page.MustEval(`()=>JSON.stringify(fixtureErrors)`).Str(); errors != "[]" {
		h.t.Fatal(errors)
	}
}

// ecalcCodesBrokenPatch adds two code items the page cannot use to the served catalog: a code kind that has no controller and a
// resistor item whose data is empty. They must show the message of an unavailable page instead of breaking the app.
const ecalcCodesBrokenPatch = `j => {
	const item = (id, code) => ({ kind: 'code', id, category: 'codes', code, name: 'Fixture: ' + id, name_en: 'Fixture: ' + id, doc: '', tags: [], source: 'Fixture', data: {} });
	j.items.push(item('fx_broken', 'resistor_bands'), item('fx_unknown', 'mystery'));
	return j;
}`

// TestDesktopECalcCodesBandsBrowser drives the two colour-band pages: the colour menu of a band (mouse and keyboard),
// both directions (colours to value, value to colours), the number of bands, the warnings and the errors of the engine, the
// values that persist per item, and the reset.
func TestDesktopECalcCodesBandsBrowser(t *testing.T) {
	h := newCodesHarness(t, "j => j")
	page, words := h.Page, h.Words
	resistor, inductor := "resistor_color_code", "inductor_color_code"
	bandCenter := func(i int) string { return fmt.Sprintf(`.ec-band[data-index="%d"]`, i) }
	colorsNow := func(id string) []string { return h.colorsOf(id, h.read()) }

	// ---- the rail lists the five codes; the page opens with the 1 kOhm / 5 % example drawn from the data ----------------------------
	h.same("the Codes rail counts the five code items", page.MustEval(`()=>document.querySelector('.ec-rail-btn[data-rail="codes"] .ec-count').textContent`).Str(), "5")
	h.open(resistor)
	state := h.read()
	h.same("page", []string{state.ID, state.Kind, state.Art, state.Count}, []string{resistor, "resistor_bands", "resistor", "4"})
	h.same("the default is brown black red gold", colorsNow(resistor), []string{"brown", "black", "red", "gold"})
	h.same("roles of the four bands", []string{state.Bands[0].Role, state.Bands[1].Role, state.Bands[2].Role, state.Bands[3].Role}, []string{"digit", "digit", "multiplier", "tolerance"})
	h.same("captions under the bands", []string{state.Bands[0].Caption, state.Bands[1].Caption, state.Bands[2].Caption, state.Bands[3].Caption}, []string{"1", "0", "×10²", "±5 %"})
	h.same("value field", state.Inputs["value"], "1 kΩ")
	h.same("the reading names the value", h.row(state, words["desktop.ecalc_dim_resistance"]), "1 kΩ")
	h.same("the reading lists the colours by their German names", h.row(state, words["desktop.ecalc_code_colors"]), strings.Join([]string{h.names[resistor]["brown"], h.names[resistor]["black"], h.names[resistor]["red"], h.names[resistor]["gold"]}, " "))
	if len(state.Warnings) != 0 || state.Error != "" || state.State != "ok" {
		t.Fatalf("the default shows warnings or an error: %+v", state)
	}
	if aria := state.Bands[0].Label; !strings.Contains(aria, h.names[resistor]["brown"]) || !strings.Contains(aria, words["desktop.ecalc_code_role_digit"]) {
		t.Fatalf("the band has no useful accessible name: %q", aria)
	}
	h.Shot("code-resistor")

	// ---- the bands are tab stops: the Tab key after the band selector lands on the first band, which then shows its focus ring -------------------------------
	page.MustEval(`()=>document.querySelector('input[name$="-bands"]:checked').focus()`)
	page.Keyboard.MustType(input.Tab)
	h.Wait(`()=>document.activeElement.matches('.ec-band[data-index="0"]')`)
	if ring := page.MustEval(`()=>getComputedStyle(document.activeElement.querySelector('.ec-band-ring')).stroke`).Str(); ring == "none" || strings.Contains(ring, "rgba(0, 0, 0, 0)") {
		t.Fatalf("the focused band has no focus ring: stroke %q", ring)
	}

	// ---- a click on a band opens the menu of the colours that fit its role, with the colour of the data -----------------------------------
	page.MustElement(bandCenter(0)).MustClick()
	h.Wait(`()=>!document.querySelector('.ec-code-menu').hidden`)
	menu := page.MustEval(`()=>JSON.stringify({n:document.querySelectorAll('.ec-code-opt').length,label:document.querySelector('.ec-code-menu').getAttribute('aria-label'),selected:document.querySelector('.ec-code-opt[aria-selected="true"]').dataset.color,
		expanded:document.querySelector('.ec-band[data-index="0"]').getAttribute('aria-expanded'),focus:document.activeElement.classList.contains('ec-code-opt'),
		swatches:[...document.querySelectorAll('.ec-code-opt')].map(o=>o.dataset.color+'='+o.querySelector('.ec-swatch rect').getAttribute('fill')),
		ring:getComputedStyle(document.querySelector('.ec-band[data-index="0"] .ec-band-ring')).stroke,
		inside:(()=>{const s=document.querySelector('.ec-code-stage').getBoundingClientRect(),m=document.querySelector('.ec-code-menu').getBoundingClientRect();return m.left>=s.left-1&&m.right<=s.right+1&&m.top>=s.top;})()})`).Str()
	var opened struct {
		N        int
		Label    string
		Selected string
		Expanded string
		Focus    bool
		Swatches []string
		Ring     string
		Inside   bool
	}
	if err := json.Unmarshal([]byte(menu), &opened); err != nil {
		t.Fatal(err)
	}
	if opened.N != 10 || opened.Selected != "brown" || opened.Expanded != "true" || !opened.Focus || !opened.Inside || opened.Label != strings.ReplaceAll(words["desktop.ecalc_code_menu"], "{n}", "1") {
		t.Fatalf("the digit menu: %+v", opened)
	}
	if opened.Ring == "none" || strings.Contains(opened.Ring, "rgba(0, 0, 0, 0)") {
		t.Fatalf("the open band has no focus ring: stroke %q", opened.Ring)
	}
	for _, swatch := range opened.Swatches {
		pair := strings.SplitN(swatch, "=", 2)
		if h.colors[resistor][pair[0]] != pair[1] {
			t.Fatalf("menu swatch %s is not the hex of the data (%s)", swatch, h.colors[resistor][pair[0]])
		}
	}
	h.Shot("code-resistor-menu")
	page.MustElement(`.ec-code-opt[data-color="green"]`).MustClick()
	h.waitColors(resistor, "green", "black", "red", "gold")
	state = h.read()
	h.same("green is band 1", colorsNow(resistor), []string{"green", "black", "red", "gold"})
	h.same("the value follows the colours", state.Inputs["value"], "5 kΩ")
	h.same("the reading follows", h.row(state, words["desktop.ecalc_dim_resistance"]), "5 kΩ")
	h.same("focus returns to the band", page.MustEval(`()=>document.activeElement.matches('.ec-band[data-index="0"]')`).Bool(), true)

	// ---- the keyboard reaches the menu: Enter opens it, the arrows move, Enter picks, Escape closes ------------------------------------------
	page.MustElement(bandCenter(2)).MustFocus()
	page.Keyboard.MustType(input.Enter)
	h.Wait(`()=>!document.querySelector('.ec-code-menu').hidden && document.activeElement.classList.contains('ec-code-opt')`)
	h.same("the menu opens on the colour of the band", page.MustEval(`()=>document.activeElement.dataset.color`).Str(), "red")
	page.Keyboard.MustType(input.ArrowDown)
	h.same("ArrowDown moves to the next colour", page.MustEval(`()=>document.activeElement.dataset.color`).Str(), "orange")
	page.Keyboard.MustType(input.Enter)
	h.Wait(`()=>document.querySelector('.ec-code-menu').hidden && document.activeElement.matches('.ec-band[data-index="2"]')`)
	h.same("the multiplier is orange", colorsNow(resistor), []string{"green", "black", "orange", "gold"})
	h.same("the value follows the multiplier", h.read().Inputs["value"], "50 kΩ")
	page.Keyboard.MustType(input.Enter)
	h.Wait(`()=>!document.querySelector('.ec-code-menu').hidden`)
	page.Keyboard.MustType(input.Escape)
	h.Wait(`()=>document.querySelector('.ec-code-menu').hidden && document.activeElement.matches('.ec-band[data-index="2"]')`)
	h.same("Escape changed nothing", colorsNow(resistor), []string{"green", "black", "orange", "gold"})
	page.MustElement(bandCenter(2)).MustFocus()
	page.Keyboard.MustType(input.ArrowLeft)
	h.same("ArrowLeft moves to the previous band", page.MustEval(`()=>document.activeElement.dataset.index`).Str(), "1")
	// A click outside closes the menu.
	page.Keyboard.MustType(input.ArrowDown)
	h.Wait(`()=>!document.querySelector('.ec-code-menu').hidden`)
	page.MustElement(".ec-code-page .ec-title").MustClick()
	h.Wait(`()=>document.querySelector('.ec-code-menu').hidden`)

	// ---- the tolerance menu lists the tolerance colours and no more ----------------------------------------------------------------------------
	page.MustElement(bandCenter(3)).MustClick()
	h.Wait(`()=>!document.querySelector('.ec-code-menu').hidden`)
	tolerances := page.MustEval(`()=>JSON.stringify([...document.querySelectorAll('.ec-code-opt')].map(o=>o.dataset.color))`).Str()
	h.same("the tolerance colours", tolerances, `["brown","red","orange","yellow","green","blue","violet","grey","gold","silver"]`)
	page.MustElement(`.ec-code-opt[data-color="silver"]`).MustClick()
	h.Wait(`()=>document.querySelector('.ec-code-menu').hidden`)
	h.same("silver is the tolerance", h.read().Inputs["tolerance"], "10")

	// ---- value to colours: with the tolerance on automatic the engine assumes one and says so ----------------------------------------------------
	h.choose(`[data-input="tolerance"]`, "")
	h.typeInto(`[data-input="value"]`, "4k7")
	h.waitColors(resistor, "yellow", "violet", "red", "gold")
	state = h.read()
	h.same("4k7 is yellow violet red gold", colorsNow(resistor), []string{"yellow", "violet", "red", "gold"})
	h.same("the warning of the engine is visible", state.Warnings, []string{"no tolerance given; ±5 % assumed"})
	h.same("the drawn captions", []string{state.Bands[0].Caption, state.Bands[1].Caption, state.Bands[2].Caption}, []string{"4", "7", "×10²"})
	h.choose(`[data-input="tolerance"]`, "1")
	h.waitColors(resistor, "yellow", "violet", "red", "brown")
	h.same("1 % is brown", colorsNow(resistor), []string{"yellow", "violet", "red", "brown"})

	// ---- the number of bands: 5 bands hold three digits, 6 bands add the temperature coefficient ---------------------------------------------------------
	h.radio("bands", "5")
	h.Wait(`()=>document.querySelector('.ec-code-art').dataset.bands==='5'`)
	h.same("4.7k in five bands", colorsNow(resistor), []string{"yellow", "violet", "black", "brown", "brown"})
	h.same("the tempco field is off below six bands", page.MustEval(`()=>document.querySelector('[data-input="tempco"]').disabled`).Bool(), true)
	h.radio("bands", "6")
	h.Wait(`()=>document.querySelector('.ec-code-art').dataset.bands==='6'`)
	state = h.read()
	h.same("six bands end with the temperature coefficient", colorsNow(resistor), []string{"yellow", "violet", "black", "brown", "brown", "brown"})
	h.same("the coefficient defaults to 100 ppm/K", state.Inputs["tempco"], "100")
	h.same("the roles of the six bands", state.Bands[5].Role, "tcr")
	h.choose(`[data-input="tempco"]`, "50")
	h.waitColors(resistor, "yellow", "violet", "black", "brown", "brown", "red")
	h.same("50 ppm/K is red", colorsNow(resistor)[5], "red")
	h.same("the reading lists the coefficient", h.row(h.read(), words["desktop.ecalc_code_field_tcr"]), "±50 ppm/K")
	h.Shot("code-resistor-six-bands")
	h.radio("bands", "3")
	h.Wait(`()=>document.querySelector('.ec-code-art').dataset.bands==='3'`)
	h.same("three bands have no tolerance band", colorsNow(resistor), []string{"yellow", "violet", "red"})
	h.same("the tolerance is fixed at 20 %", page.MustEval(`()=>document.querySelector('[data-input="tolerance"]').disabled+'|'+document.querySelector('[data-input="tolerance"]').value`).Str(), "true|20")
	h.radio("bands", "4")
	h.Wait(`()=>document.querySelector('.ec-code-art').dataset.bands==='4'`)

	// ---- errors of the engine stay inline: nothing reaches the console -----------------------------------------------------------------------------------------------
	h.choose(`[data-input="tolerance"]`, "1")
	h.typeInto(`[data-input="value"]`, "4.75k")
	h.Wait(`()=>document.querySelector('.ec-code-error').dataset.code==='unsupported'`)
	state = h.read()
	if !strings.Contains(state.Error, "3 significant digits") || !strings.HasPrefix(state.Error, words["desktop.ecalc_code_error_unsupported"]) || !reflect.DeepEqual(state.Invalid, []string{"value"}) || state.State != "error" {
		t.Fatalf("4.75k in four bands: %+v", state)
	}
	h.same("the drawing stays", len(state.Bands), 4)
	h.Shot("code-resistor-error")
	h.typeInto(`[data-input="value"]`, "abc")
	h.Wait(`()=>document.querySelector('.ec-code-error').dataset.code==='invalid_value'`)
	if state = h.read(); !strings.Contains(state.Error, `cannot read "abc"`) || !strings.HasPrefix(state.Error, words["desktop.ecalc_code_error_invalid_value"]) {
		t.Fatalf("abc: %+v", state)
	}
	h.typeInto(`[data-input="value"]`, "10k")
	h.waitColors(resistor, "brown", "black", "orange", "brown")
	h.same("10k is brown black orange brown", colorsNow(resistor), []string{"brown", "black", "orange", "brown"})

	// ---- the entered values persist per item ------------------------------------------------------------------------------------------------------------------------------------
	h.radio("bands", "5")
	h.Wait(`()=>document.querySelector('.ec-code-art').dataset.bands==='5'`)
	want := colorsNow(resistor)
	h.CloseWindows()
	h.OpenApp(`.ec-page`)
	h.Wait(`()=>document.querySelector('.ec-page')?.dataset.code==='` + resistor + `'`)
	state = h.read()
	h.same("the bands persist", []string{state.Count, state.Inputs["value"], state.Inputs["tolerance"]}, []string{"5", "10 kΩ", "1"})
	h.same("the colours persist", colorsNow(resistor), want)
	h.setDigits("6")
	state = h.read()
	h.same("a change of the digits keeps the page as it was", []string{state.Count, state.Inputs["value"], state.State}, []string{"5", "10 kΩ", "ok"})
	h.same("and the colours", colorsNow(resistor), want)
	h.setDigits("4")
	h.same("a favorite is a click away", page.MustEval(`()=>document.querySelector('.ec-code-page [data-action="favorite"]').getAttribute('aria-pressed')`).Str(), "false")
	page.MustElement(`.ec-code-page [data-action="favorite"]`).MustClick()
	h.Wait(`()=>document.querySelector('.ec-rail-btn[data-rail="favorites"] .ec-count').textContent==='1'`)
	// The reset button brings the example back.
	page.MustElement(`.ec-code-page [data-action="reset"]`).MustClick()
	h.Wait(`()=>document.querySelector('.ec-code-art').dataset.bands==='4'`)
	h.same("reset", colorsNow(resistor), []string{"brown", "black", "red", "gold"})

	// ---- inductor: EIA (four bands) and the military code (a wide silver band first, gold is the decimal point) ----------------------------------------------------------------
	h.open(inductor)
	state = h.read()
	h.same("inductor page", []string{state.Art, state.Count, state.Inputs["value"]}, []string{"inductor", "4", "100 µH"})
	h.same("EIA colours", colorsNow(inductor), []string{"brown", "black", "brown", "silver"})
	h.same("EIA roles", []string{state.Bands[2].Role, state.Bands[3].Role}, []string{"multiplier", "tolerance"})
	h.radio("bands", "5")
	h.Wait(`()=>document.querySelector('.ec-code-art').dataset.bands==='5'`)
	state = h.read()
	h.same("military code starts with silver", state.Bands[0].Role+"|"+state.Bands[0].Color, "identifier|"+h.names[inductor]["silver"])
	h.typeInto(`[data-input="value"]`, "6.8u")
	h.waitColors(inductor, "silver", "blue", "gold", "grey", "silver")
	h.same("6.8 µH is silver blue gold grey silver", colorsNow(inductor), []string{"silver", "blue", "gold", "grey", "silver"})
	h.same("the gold band is the decimal point", h.read().Bands[2].Caption, ".")
	page.MustElement(bandCenter(2)).MustClick()
	h.Wait(`()=>!document.querySelector('.ec-code-menu').hidden`)
	h.same("gold is offered as the decimal point", page.MustEval(`()=>document.querySelector('.ec-code-opt[data-color="gold"] .ec-opt-meaning').textContent`).Str(), words["desktop.ecalc_code_role_point"])
	page.MustElement(`.ec-code-opt[data-color="red"]`).MustClick()
	h.Wait(`()=>document.querySelector('.ec-code-menu').hidden`)
	// silver blue red grey silver: 68 x 100 (the fourth band is a multiplier only up to orange): the engine says why.
	state = h.read()
	if state.ErrorCode != "invalid_code" || !strings.Contains(state.Error, "multiplier") {
		t.Fatalf("a digit that is no multiplier in band 4: %+v", state)
	}
	h.Shot("code-inductor-error")
	h.typeInto(`[data-input="value"]`, "22u")
	h.waitColors(inductor, "silver", "red", "red", "black", "silver")
	h.same("22 µH military", colorsNow(inductor), []string{"silver", "red", "red", "black", "silver"})

	h.noErrors()
}

// TestDesktopECalcCodesMarkingsBrowser drives the three printed-code pages: SMD resistor (3-digit, 4-digit, EIA-96 and R
// notation, the chip drawing with the code printed on it), capacitor (value code, tolerance letter, voltage code) and ceramic
// temperature codes (class 1 and 2).
func TestDesktopECalcCodesMarkingsBrowser(t *testing.T) {
	h := newCodesHarness(t, ecalcCodesBrokenPatch)
	page, words := h.Page, h.Words
	field := func(id string) string { return words["desktop.ecalc_code_field_"+id] }

	// ---- SMD resistor ---------------------------------------------------------------------------------------------------------
	h.open("smd_resistor_code")
	state := h.read()
	h.same("smd page", []string{state.Kind, state.Art, state.Valid}, []string{"smd_resistor", "chip", "true"})
	h.same("the chip prints the code", state.Chip, []string{"472"})
	h.same("472 is 4,7 kOhm (decimal comma in German)", state.Inputs["value"], "4,7 kΩ")
	h.same("the notation", h.row(state, field("family")), "3-digit code")
	h.Shot("code-smd")
	h.typeInto(`[data-input="code"]`, "01C")
	h.Wait(`()=>document.querySelector('.ec-chip-text').textContent==='01C' && document.querySelector('[data-input="value"]').value==='10 kΩ'`)
	state = h.read()
	h.same("EIA-96", h.row(state, field("family")), "EIA-96")
	h.same("the code field keeps what was typed", state.Inputs["code"], "01C")
	h.typeInto(`[data-input="value"]`, "10k")
	h.typeInto(`[data-input="tolerance"]`, "1 %")
	h.Wait(`()=>document.querySelector('[data-input="code"]').value==='1002'`)
	state = h.read()
	h.same("at 1 % the 4-digit code is primary", state.Chip, []string{"1002"})
	h.same("every marking is a button", state.Prints, []string{"103:false", "1002:true", "01C:false"})
	page.MustElement(`.ec-code-print[data-print="01C"]`).MustClick()
	h.Wait(`()=>document.querySelector('.ec-chip-text').textContent==='01C'`)
	state = h.read()
	h.same("the pressed marking is on the chip", state.Prints, []string{"103:false", "1002:false", "01C:true"})
	h.same("and in the code field", state.Inputs["code"], "01C")
	h.typeInto(`[data-input="code"]`, "ABC")
	h.Wait(`()=>document.querySelector('.ec-code-error').dataset.code==='invalid_code'`)
	state = h.read()
	if state.Valid != "false" || !reflect.DeepEqual(state.Invalid, []string{"code"}) || !strings.Contains(state.Error, "SMD resistor code") {
		t.Fatalf("ABC: %+v", state)
	}
	h.same("the chip shows what was typed, dimmed", state.Chip, []string{"ABC"})
	h.Shot("code-smd-error")
	h.typeInto(`[data-input="code"]`, "4R7")
	h.Wait(`()=>document.querySelector('.ec-code-error').hidden`)
	h.same("R notation", h.read().Inputs["value"], "4,7 Ω")
	h.setDigits("6")
	h.same("a change of the digits keeps the SMD page", []string{h.read().Inputs["code"], h.read().Inputs["value"], h.read().State}, []string{"4R7", "4,7 Ω", "ok"})
	h.setDigits("4")

	// ---- capacitor ---------------------------------------------------------------------------------------------------------------
	h.open("capacitor_code")
	state = h.read()
	h.same("capacitor page", []string{state.Kind, state.Art}, []string{"capacitor", "disc"})
	h.same("the disc prints the marking on two lines", state.Chip, []string{"104K", "1H"})
	h.same("104K 1H fields", []string{state.Inputs["value"], state.Inputs["tolerance"], state.Inputs["voltage"]}, []string{"100 nF", "K", "50"})
	h.same("tolerance and voltage in the reading", []string{h.row(state, field("tolerance")), h.row(state, field("voltage"))}, []string{"±10 %", "50 V"})
	if len(state.Warnings) == 0 || !strings.Contains(state.Warnings[0], "voltage codes are a maker convention") {
		t.Fatalf("the voltage code warning of the engine is missing: %v", state.Warnings)
	}
	h.choose(`[data-input="voltage"]`, "100")
	h.Wait(`()=>document.querySelector('[data-input="code"]').value==='104K 2A'`)
	h.same("100 V is 2A", h.read().Chip, []string{"104K", "2A"})
	h.typeInto(`[data-input="value"]`, "4.7n")
	h.Wait(`()=>document.querySelector('[data-input="code"]').value==='472K 2A'`)
	h.choose(`[data-input="tolerance"]`, "J")
	h.Wait(`()=>document.querySelector('[data-input="code"]').value==='472J 2A'`)
	h.same("J is 5 %", h.row(h.read(), field("tolerance")), "±5 %")
	h.typeInto(`[data-input="code"]`, "104 K 1H 3A")
	h.Wait(`()=>document.querySelector('.ec-code-error').dataset.code==='invalid_code'`)
	if state = h.read(); !strings.Contains(state.Error, "at most three parts") || !reflect.DeepEqual(state.Invalid, []string{"code"}) {
		t.Fatalf("four parts: %+v", state)
	}
	h.typeInto(`[data-input="code"]`, "4p7C")
	h.Wait(`()=>document.querySelector('.ec-code-error').hidden`)
	state = h.read()
	h.same("4p7C is 4,7 pF, C is 0.25 pF", []string{state.Inputs["value"], state.Inputs["tolerance"], h.row(state, field("tolerance"))}, []string{"4,7 pF", "C", "±0,25 pF"})
	h.setDigits("6")
	h.same("a change of the digits keeps the capacitor page", []string{h.read().Inputs["value"], h.read().Inputs["tolerance"], h.read().State}, []string{"4,7 pF", "C", "ok"})
	h.setDigits("4")
	h.Shot("code-capacitor")

	// ---- ceramic temperature codes ----------------------------------------------------------------------------------------------------------------------
	h.open("ceramic_temp_code")
	state = h.read()
	h.same("ceramic default", []string{state.Kind, state.Classes, state.Inputs["code"], state.Inputs["low"], state.Inputs["high"], state.Inputs["change"]}, []string{"ceramic_temp", "2", "X7R", "X", "7", "R"})
	h.same("only the class 2 fields are shown", state.Panels, []string{"2"})
	h.same("X7R reading", []string{h.row(state, field("temp_low")), h.row(state, field("temp_high")), h.row(state, field("change"))}, []string{"−55 °C", "+125 °C", "±15 %"})
	h.choose(`[data-input="change"]`, "P")
	h.Wait(`()=>document.querySelector('[data-input="code"]').value==='X7P'`)
	h.same("X7P", h.row(h.read(), field("change")), "±10 %")
	h.choose(`[data-input="low"]`, "Y")
	h.choose(`[data-input="high"]`, "5")
	h.choose(`[data-input="change"]`, "V")
	h.Wait(`()=>document.querySelector('[data-input="code"]').value==='Y5V'`)
	h.same("Y5V is asymmetric", h.row(h.read(), field("change")), "+22 % / −82 %")
	h.radio("class", "1")
	h.Wait(`()=>document.querySelector('[data-input="code"]').value==='U2G'`)
	state = h.read()
	h.same("class 1 shows its own fields", []string{state.Classes, strings.Join(state.Panels, ",")}, []string{"1", "1"})
	h.same("-750 ppm/K and 30 ppm/K make U2G", h.row(state, field("tc")), "−750 ppm/K")
	h.typeInto(`[data-input="code"]`, "np0")
	h.Wait(`()=>document.querySelector('[data-input="code"]').value==='np0' && document.querySelector('.ec-code-error').hidden && document.querySelector('[data-input="tempco"]').value==='0 ppm'`)
	h.same("NP0 is C0G", h.row(h.read(), field("industry_code")), "NP0")
	page.MustElement(`.ec-chip-btn[data-pick="X7R"]`).MustClick()
	h.Wait(`()=>document.querySelector('input[name$="-class"]:checked').value==='2' && document.querySelector('[data-input="code"]').value==='X7R'`)
	h.typeInto(`[data-input="code"]`, "QQQ")
	h.Wait(`()=>document.querySelector('.ec-code-error').dataset.code==='invalid_code'`)
	if state = h.read(); !strings.Contains(state.Error, "unknown temperature coefficient letter Q") {
		t.Fatalf("QQQ: %+v", state)
	}
	h.typeInto(`[data-input="code"]`, "U2J")
	h.Wait(`()=>document.querySelector('.ec-code-error').hidden && document.querySelector('input[name$="-class"]:checked').value==='1'`)
	state = h.read()
	h.same("U2J fields", []string{state.Inputs["tempco"], state.Inputs["tol"], h.row(state, field("tolerance"))}, []string{"-750 ppm", "120", "±120 ppm/K"})
	h.setDigits("6")
	h.same("a change of the digits keeps the ceramic page", []string{h.read().Inputs["code"], h.read().Classes, h.read().State}, []string{"U2J", "1", "ok"})
	h.setDigits("4")
	h.Shot("code-ceramic")

	// ---- the printed codes persist per item as well: a new window opens on the last page with the last code ------------------------------------------------------
	h.CloseWindows()
	h.OpenApp(`.ec-page`)
	h.Wait(`()=>document.querySelector('.ec-page')?.dataset.code==='ceramic_temp_code'`)
	state = h.read()
	h.same("the ceramic page keeps its code and class", []string{state.Inputs["code"], state.Classes, h.row(state, field("tc"))}, []string{"U2J", "1", "−750 ppm/K"})

	// ---- an item the page cannot use says so: a code kind without a controller, a resistor whose data is empty ----------------------------------------------------
	for _, id := range []string{"fx_unknown", "fx_broken"} {
		h.open(id)
		h.same("the message for "+id, page.MustEval(`id=>document.querySelector('.ec-page[data-code="'+id+'"] .ec-empty-page').textContent`, id).Str(), words["desktop.ecalc_page_unavailable"])
	}

	h.noErrors()
}

// contrastJS measures the drawing of the open code page against what is behind it: translucent layers are laid over the next
// one outward, an image (a gradient) falls back to the strong panel colour of the skin.
const contrastJS = `()=>{
	const cv=document.createElement('canvas');cv.width=cv.height=1;const cx=cv.getContext('2d',{willReadFrequently:true});
	const rgba=css=>{cx.clearRect(0,0,1,1);cx.fillStyle='#000';cx.fillStyle=css;cx.fillRect(0,0,1,1);const d=cx.getImageData(0,0,1,1).data;return [d[0],d[1],d[2],d[3]/255];};
	const over=(top,under)=>{const a=top[3];return [0,1,2].map(i=>top[i]*a+under[i]*(1-a)).concat([1]);};
	const lum=c=>{const f=v=>{v/=255;return v<=0.03928?v/12.92:Math.pow((v+0.055)/1.055,2.4);};return 0.2126*f(c[0])+0.7152*f(c[1])+0.0722*f(c[2]);};
	const ratio=(a,b)=>{const x=lum(a),y=lum(b);return (Math.max(x,y)+0.05)/(Math.min(x,y)+0.05);};
	const stage=document.querySelector('.ec-code-stage');
	const strong=over(rgba(getComputedStyle(stage).getPropertyValue('--vd-theme-panel-bg-strong')||'#fff'),[255,255,255,1]);
	const behind=el=>{const layers=[];for(let n=el;n;n=n.parentElement){const cs=getComputedStyle(n);if(cs.backgroundImage&&cs.backgroundImage!=='none')return strong;const c=rgba(cs.backgroundColor);if(c[3]>0)layers.push(c);if(c[3]>=1)break;}
		let base=strong;if(layers.length&&layers[layers.length-1][3]>=1)base=layers.pop();for(let i=layers.length-1;i>=0;i--)base=over(layers[i],base);return base;};
	const bg=behind(stage),out={bg:bg.map(Math.round).join(',')};
	const first=document.querySelector('.ec-band-fill');
	if(first){const cs=getComputedStyle(first),s=rgba(cs.stroke);s[3]*=parseFloat(cs.strokeOpacity);out.bandOutline=ratio(over(s,bg),bg);out.fills=[...document.querySelectorAll('.ec-band-fill')].map(b=>getComputedStyle(b).fill).join('|');
		const body=document.querySelector('.ec-draw-body');out.bodyOutline=ratio(rgba(getComputedStyle(body).stroke),bg);}
	const chip=document.querySelector('.ec-chip-body');
	if(chip){const body=rgba(getComputedStyle(chip).fill),ink=over(rgba(getComputedStyle(document.querySelector('.ec-chip-text')).fill),body);out.print=ratio(ink,body);out.chip=ratio(body,bg);}
	return JSON.stringify(out);
}`

// TestDesktopECalcCodesSkinsBrowser checks the drawings in every desktop skin: the colour of a band is the same everywhere
// (it comes from the data), the outline of the bands and of the body is legible against the stage, the printed code of the
// SMD chip against the chip; and a phone-sized window keeps the page and the colour menu inside the screen.
func TestDesktopECalcCodesSkinsBrowser(t *testing.T) {
	h := newCodesHarness(t, "j => j")
	page := h.Page
	skins := []string{}
	for _, theme := range parseDesktopThemeRegistry(t) {
		if theme.ID == "fruity" {
			skins = append(skins, "fruity-light", "fruity-dark")
		} else {
			skins = append(skins, theme.ID)
		}
	}
	type measure struct {
		Bg          string
		BandOutline float64
		BodyOutline float64
		Fills       string
		Print       float64
		Chip        float64
	}
	measureNow := func() measure {
		var m measure
		if err := json.Unmarshal([]byte(page.MustEval(contrastJS).Str()), &m); err != nil {
			t.Fatal(err)
		}
		return m
	}
	const outlineMin, printMin = 3.0, 4.5
	h.open("resistor_color_code")
	var fills string
	for _, skin := range skins {
		page.MustEval(`id=>fixtureTheme(id)`, skin)
		m := measureNow()
		t.Logf("skin %-14s resistor: bands %.2f body %.2f (stage rgb %s)", skin, m.BandOutline, m.BodyOutline, m.Bg)
		if m.BandOutline < outlineMin || m.BodyOutline < outlineMin {
			t.Errorf("skin %s: the resistor outline is faint against the stage (bands %.2f, body %.2f, stage rgb %s, want %.1f)", skin, m.BandOutline, m.BodyOutline, m.Bg, outlineMin)
		}
		if fills == "" {
			fills = m.Fills
		} else if m.Fills != fills {
			t.Errorf("skin %s draws the bands in other colours than the standard skin: %s vs %s", skin, m.Fills, fills)
		}
		if skin == "fruity-dark" || skin == "cyberpunk" || skin == "minimal" {
			h.Shot("code-resistor-" + skin)
		}
	}
	h.open("smd_resistor_code")
	for _, skin := range skins {
		page.MustEval(`id=>fixtureTheme(id)`, skin)
		m := measureNow()
		t.Logf("skin %-14s smd chip: print %.2f chip %.2f (stage rgb %s)", skin, m.Print, m.Chip, m.Bg)
		if m.Print < printMin || m.Chip < outlineMin {
			t.Errorf("skin %s: the SMD chip is hard to read (print on chip %.2f, chip on stage %.2f, stage rgb %s, want %.1f and %.1f)", skin, m.Print, m.Chip, m.Bg, printMin, outlineMin)
		}
		if skin == "fruity-dark" || skin == "cyberpunk" || skin == "minimal" {
			h.Shot("code-smd-" + skin)
		}
	}
	page.MustEval(`()=>fixtureTheme('standard')`)

	// ---- a phone-sized window: the page does not move sideways and the menu stays on the stage -----------------------------------------------------------------
	h.CloseWindows()
	page.MustSetViewport(390, 667, 1, false)
	page.MustEval(`()=>fixtureTheme('fruity-dark')`)
	h.OpenApp(`.ec-app[data-compact="true"]`)
	h.Wait(`()=>!!document.querySelector('.ec-page')`)
	page.MustElement(".ec-back").MustClick()
	page.MustEval(`()=>document.querySelector('.ec-rail-btn[data-rail="codes"]').click()`)
	page.MustElement(`.ec-item[data-id="resistor_color_code"]`).MustClick()
	h.Wait(`()=>document.querySelector('.ec-page')?.dataset.code==='resistor_color_code'`)
	page.MustElement(`.ec-band[data-index="3"]`).MustClick()
	h.Wait(`()=>!document.querySelector('.ec-code-menu').hidden`)
	if got := page.MustEval(`()=>{const h=document.querySelector('.ec-page-host'),s=document.querySelector('.ec-code-stage').getBoundingClientRect(),m=document.querySelector('.ec-code-menu').getBoundingClientRect(),a=document.querySelector('.ec-code-art').getBoundingClientRect();
		return (h.scrollWidth>h.clientWidth+1?'the page scrolls sideways; ':'')+(m.left<s.left-1||m.right>s.right+1?'the menu leaves the stage; ':'')+(a.width<250?'the drawing is only '+a.width+' px wide; ':'');}`).Str(); got != "" {
		t.Fatal(got)
	}
	h.Shot("code-compact-menu")
	page.Keyboard.MustType(input.Escape)
	page.MustElement(".ec-back").MustClick()
	page.MustEval(`()=>document.querySelector('.ec-rail-btn[data-rail="codes"]').click()`)
	page.MustElement(`.ec-item[data-id="capacitor_code"]`).MustClick()
	h.Wait(`()=>document.querySelector('.ec-page')?.dataset.code==='capacitor_code'`)
	if got := page.MustEval(`()=>{const h=document.querySelector('.ec-page-host');return h.scrollWidth>h.clientWidth+1?'the page scrolls sideways':'';}`).Str(); got != "" {
		t.Fatal(got)
	}
	h.Shot("code-compact-capacitor")
	h.noErrors()
}

// ---- the source of the page and of the loader wiring -------------------------------------------------------------------------

func TestECalcCodePagesAreWired(t *testing.T) {
	t.Parallel()
	loader := readDesktopAssetText(t, "js/desktop/core/module-loader.js")
	start := strings.Index(loader, "        ecalc: {")
	if start < 0 {
		t.Fatal("module-loader.js has no ecalc entry in DESKTOP_APP_ASSETS")
	}
	entry := loader[start : start+strings.Index(loader[start:], "\n        },")]
	// The engine follows the table registry; the drawings and the page follow the table page and come before the shell, which
	// looks up window.ECalcCodePage when it mounts a page.
	order := []string{"ecalc-eseries.js", "ecalc-solve.js", "ecalc-tables.js", "ecalc-codes.js", "ecalc-typeset.js", "ecalc-calc-page.js", "ecalc-table-page.js", "ecalc-code-draw.js", "ecalc-code-page.js", "ecalc-shell.js", "/ecalc.js'"}
	offset := 0
	for _, script := range order {
		index := strings.Index(entry[offset:], script)
		if index < 0 {
			t.Fatalf("the ecalc loader entry lists %s out of order or not at all", script)
		}
		offset += index + len(script)
	}
	scripts := ecalcScripts(t)
	for _, name := range []string{"ecalc-codes.js", "ecalc-code-draw.js", "ecalc-code-page.js"} {
		if _, ok := scripts[name]; !ok {
			t.Errorf("%s/%s is missing", ecalcDir, name)
		}
	}
	shell := scripts["ecalc-shell.js"]
	for _, marker := range []string{"window.ECalcCodePage || null", "kind === 'code'"} {
		if !strings.Contains(shell, marker) {
			t.Errorf("ecalc-shell.js is missing %q", marker)
		}
	}
}

// The page calls the engine for every reading and writing, keeps what the user entered in the store under the item id,
// and never reaches into the theme: its only inline style is the position of the colour menu.
func TestECalcCodePageUsesTheEngineAndTheStore(t *testing.T) {
	t.Parallel()
	page := ecalcScripts(t)["ecalc-code-page.js"]
	for _, marker := range []string{
		"window.ECalcCodePage = { create };", "codes.decode(item, ", "codes.encode(item, ", "store.saveCalc(item.id, ", "store.calcState(item.id)", "el.dataset.code = item.id;",
		"el.dataset.kind = String(item.code);", "Draw.bandsSVG(", "Draw.chipSVG(", `role="listbox"`, `role="option"`, "event.key === 'Escape'", "event.key === 'ArrowDown'",
		"function describe(error)", "store.flush()", "ERROR_CODES = ['invalid_code', 'invalid_value', 'unsupported']", "const CONTROLLERS = {",
		"resistor_bands: resistorController, inductor_bands: inductorController, smd_resistor: smdController, capacitor: capacitorController, ceramic_temp: ceramicController",
	} {
		if !strings.Contains(page, marker) {
			t.Errorf("ecalc-code-page.js is missing %q", marker)
		}
	}
	body := regexp.MustCompile(`(?s)/\*.*?\*/`).ReplaceAllString(page, "")
	for label, pattern := range map[string]string{
		"hex colour":      `#[0-9a-fA-F]{3,8}\b`,
		"colour function": `\b(?:rgba?|hsla?)\(`,
		"console call":    `\bconsole\.`,
		"storage access":  `\b(?:localStorage|sessionStorage)\b`,
		"throw":           `\bthrow\b`,
		"dynamic key":     `['"]desktop\.ecalc_code_[a-z0-9_]*['"]\s*\+`,
	} {
		if match := regexp.MustCompile(pattern).FindString(body); match != "" {
			t.Errorf("ecalc-code-page.js must not contain a %s (%q)", label, match)
		}
	}
	for _, style := range regexp.MustCompile(`\.style\.[A-Za-z]+`).FindAllString(body, -1) {
		if style != ".style.setProperty" {
			t.Errorf("ecalc-code-page.js uses %s; only .style.setProperty for the menu position is allowed", style)
		}
	}
	if got := strings.Count(body, ".style.setProperty('--ec-menu-"); got != 2 {
		t.Errorf("ecalc-code-page.js sets %d menu position variables, want --ec-menu-x and --ec-menu-y", got)
	}
}

// The page names every string of the code pages as a literal and uses no key of this family that ecalcCodeKeys does not list
// (the list is also what TestECalcUsesOnlyItsOwnTranslationKeys lets the ECalc scripts use).
func TestECalcCodeKeysAreUsedByThePage(t *testing.T) {
	t.Parallel()
	page := ecalcScripts(t)["ecalc-code-page.js"]
	listed := map[string]bool{}
	for _, key := range ecalcCodeKeys {
		listed[key] = true
		if !strings.Contains(page, "'"+key+"'") {
			t.Errorf("ecalc-code-page.js never uses the key %s", key)
		}
	}
	for _, match := range regexp.MustCompile(`['"](desktop.ecalc_code_[a-z0-9_]+)['"]`).FindAllStringSubmatch(page, -1) {
		if !listed[match[1]] {
			t.Errorf("ecalc-code-page.js uses the key %s, which ecalcCodeKeys does not list", match[1])
		}
	}
}

// The colours of the data are what the drawing shows: every colour of the three band and chip kinds is a hex colour (or the
// empty band), written in the data of the code item.
func TestECalcCodeColoursAreHexInTheData(t *testing.T) {
	t.Parallel()
	for id, colors := range ecalcCodeColors(t, "en") {
		for color, hex := range colors {
			if !regexp.MustCompile(`^(#[0-9A-Fa-f]{6}|transparent)$`).MatchString(hex) {
				t.Errorf("%s: colour %s has the hex %q", id, color, hex)
			}
		}
	}
}
