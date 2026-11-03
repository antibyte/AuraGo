package ui

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/go-rod/rod/lib/input"
)

// ecalcTablesFixturePatch adds the content the later parts write to the served catalog, in the JSON shape of the contract
// (the catalog is served in German, so the names are German): six fixture tables in the groups conductors, general and an
// unknown one, two finder calculators and a code item.
//   - fx_ampacity / fx_ampacity_al: guide and mains banners, notes, a doc text, null cells in a numeric and in a text column,
//     unit lists on two columns, and the finder fx_finder with the material fixed to cu / al. The finder solves
//     smin = lookup_ge(tbl, col, ieq, out) through the real JavaScript solver: the material option defines the aliases
//     tbl / col / out (refs), a second choice (method) stays visible and divides the current.
//   - fx_awg: a text key column and the columns of the generated AWG table (ASTM B258 diameter and area, copper at 20 degC);
//     its item carries `generate`, so its cells follow the significant-digit setting, while the transcribed fixtures keep their digits.
//   - fx_long: 60 rows and the finder fx_pick (n = round(x)): the highlighted row must be scrolled into view, and a row
//     number beyond the table shows the no-match hint.
//   - fx_ampacity_al also has two columns without any value (a numeric and a text one): they are hidden, a line under the
//     table counts them, and the finder's aliases still find the columns that have data.
//   - fx_ascii: an integer column whose units list holds only radix units (dec, hex, bin, oct), no automatic prefix.
//   - fx_finder has a check that fires for a current beyond the tables: lookup_ge gives no value, the header shows the warning,
//     nothing is highlighted.
//   - fx_wide: 16 columns and a very long name: the table scrolls inside its own area, neither the page nor the list moves
//     sideways. fx_finder has a third choice with twelve long options (the segmented control wraps instead of widening the page).
//   - fx_plain: no banners, no notes, no doc, no finder; fx_misc: a group the shell does not know and a column with a fixed unit
//     (mil) but no units list: no dropdown, the unit is written once in the header.
//   - fx_code: an item of kind code (the page creator is stubbed by the test).
//
// Tables of the real catalog (the generated tables of Part A and the content of Part C) are taken out of the served JSON
// unless window.ecalcWithReal is set, so that the fixture assertions do not depend on content; their ids stay in
// window.ecalcRealTableIDs.
const ecalcTablesFixturePatch = `j => {
	const real = j.items.filter(item => item.kind === 'table');
	window.ecalcRealTableIDs = real.map(item => item.id);
	if (!window.ecalcWithReal) j.items = j.items.filter(item => !real.includes(item));
	const col = (id, name, dim, unit, units, label) => ({ id, name, dim, unit: unit || '', units: units || [], label: label || '' });
	const table = (id, group, name, extra) => Object.assign({ kind: 'table', id, category: 'tables', group, name, name_en: name + ' (en)', doc: '', notes: '', tags: [id.replace(/_/g, ' ')],
		source: 'Fixture, keine Norm', guide: false, mains: false, key: '', columns: [], rows: [], finder: null, generate: '' }, extra);
	const section = [1.5e-6, 2.5e-6, 4e-6, 6e-6, 10e-6, 16e-6];
	const amps = [15.5, 19.5, 26, 34, 46, 61];
	const diameter = [1.382e-3, 1.784e-3, null, 2.764e-3, 3.568e-3, 4.514e-3];
	const notes = [null, 'siehe Anmerkung 2', null, 'siehe Anmerkung 2', 'Mehrleiter', null];
	const ampacity = (id, name, factor, material, unverified) => table(id, 'conductors', name, {
		doc: 'Fixture-Tabelle mit Richtwerten.\n\nZweiter Absatz der Erklärung.', notes: 'Anmerkung 1: Werte für Cu.\n\nAnmerkung 2: Gehäufte Verlegung.', guide: true, mains: true, key: 'section',
		columns: [col('section', 'Querschnitt', 'area', 'mm2', ['mm2', 'AWG'], 'A'), col('iz', 'Belastbarkeit', 'current', '', [], 'I_z'), col('d', 'Außendurchmesser', 'length', '', ['', 'in', 'mil']), col('note', 'Hinweis', 'text')].concat(unverified ? [col('iz_b', 'Belastbarkeit B2', 'current'), col('note_b', 'Hinweis B2', 'text')] : []),
		rows: section.map((s, i) => [s, Math.round(amps[i] * factor * 10) / 10, diameter[i], notes[i]].concat(unverified ? [null, null] : [])),
		finder: { calc: 'fx_finder', column: 'section', var: 'smin', choices: { material } }
	});
	const awgSize = n => (n <= 0 ? ['1/0', '2/0', '3/0', '4/0'][-n] : String(n));
	const awgRows = [-3, -2, -1, 0, 1, 2, 3, 4, 6, 8, 10, 12, 14, 16, 18, 20, 24, 28, 32, 36, 40].map(n => {
		const d = 0.127e-3 * Math.pow(92, (36 - n) / 39), area = Math.PI / 4 * d * d;
		return [awgSize(n), n, d, area, 1000 / (58e6 * area)];
	});
	const wideColumns = Array.from({ length: 15 }, (_, i) => col('c' + (i + 1), 'Spalte ' + (i + 1) + ' mit einer längeren Beschriftung', ['current', 'voltage', 'number'][i % 3]));
	const longRows = Array.from({ length: 60 }, (_, i) => [i + 1, (i + 1) * (i + 1)]);
	const chooser = (id, label, options) => ({ id, name: label, default: options[0].id, options });
	const option = (id, name, consts, refs) => ({ id, name, consts, ignore: [], refs: refs || {} });
	const routing = chooser('routing', 'Verlegevariante', Array.from({ length: 12 }, (_, i) => option('r' + (i + 1), 'Verlegeart ' + (i + 1) + ' – mehradrig im Rohr in wärmegedämmter Wand, direkt verlegt', {})));
	const calc = (id, name, extra) => Object.assign({ kind: 'calculator', id, category: 'wiring', name, name_en: name, doc: 'Fixture-Rechner.', tags: [], source: 'Fixture', vars: [], choices: [], dof: 1, rules: [], derived: [], checks: [], formula: [], schematic: '', sets: [], finder: '', chart: null }, extra);
	const v = (id, dim, label, name, extra) => Object.assign({ id, dim, label, name, default: null, default_list: null, unit: '', units: [], eseries: false, list: false, input: true }, extra);
	j.items.push(
		ampacity('fx_ampacity', 'Fixture: Belastbarkeit Cu', 1, 'cu'),
		ampacity('fx_ampacity_al', 'Fixture: Belastbarkeit Al', 0.78, 'al', true),
		table('fx_awg', 'conductors', 'Fixture: AWG', {
			key: 'size', generate: 'awg', source: 'ASTM B258, IEC 60028 (Formeln)',
			columns: [col('size', 'Größe', 'text'), col('awg', 'AWG-Zahl', 'number'), col('diameter', 'Durchmesser', 'length', '', ['', 'in', 'mil']), col('area', 'Querschnitt', 'area', 'mm2', ['mm2', 'kcmil']), col('r_km', 'Widerstand je km', 'resistance')],
			rows: awgRows
		}),
		table('fx_long', 'general', 'Fixture: Quadratzahlen', { key: 'n', notes: 'Hier steht eine Anmerkung.', columns: [col('n', 'N', 'number'), col('sq', 'N²', 'number')], rows: longRows,
			finder: { calc: 'fx_pick', column: 'n', var: 'n', choices: {} } }),
		table('fx_plain', 'general', 'Fixture: Einfach', { key: 'name', columns: [col('name', 'Werkstoff', 'text'), col('rho', 'Spezifischer Widerstand', 'resistivity', '')],
			rows: [['Kupfer', 1.72e-8], ['Aluminium', 2.82e-8], ['Gold', 2.44e-8]] }),
		table('fx_ascii', 'general', 'Fixture: ASCII', { key: 'char', columns: [col('char', 'Zeichen', 'text'), col('dec', 'Code', 'integer', 'dec', ['dec', 'hex', 'bin', 'oct'])],
			rows: [['A', 65], ['B', 66], ['a', 97], ['0', 48]] }),
		table('fx_wide', 'general', 'Fixture: Strombelastbarkeitsumrechnungsfaktoren', { key: 'n', columns: [col('n', 'Zeile', 'number')].concat(wideColumns),
			rows: Array.from({ length: 5 }, (_, r) => [r + 1].concat(wideColumns.map((c, i) => (r + 1) * (i + 1)))) }),
		table('fx_misc', 'misc', 'Fixture: Sonstiges', { key: 'a', columns: [col('a', 'A', 'number'), col('b', 'B', 'length', 'mil')], rows: [[1, 2.54e-4], [2, 5.08e-5]] }),
		calc('fx_finder', 'Fixture: Querschnitt finden', {
			vars: [v('current', 'current', 'I', 'Strom', { default: 15 }), v('smin', 'area', 'A', 'Querschnitt', { unit: 'mm2', units: ['mm2'], input: false })],
			choices: [chooser('material', 'Werkstoff', [option('cu', 'Kupfer', {}, { tbl: 'fx_ampacity', col: 'iz', out: 'section' }), option('al', 'Aluminium', {}, { tbl: 'fx_ampacity_al', col: 'iz', out: 'section' })]), chooser('method', 'Verlegeart', [option('a', 'A', { m: 1 }), option('b', 'B', { m: 0.8 })]), routing],
			rules: [{ target: 'smin', alts: ['lookup_ge(tbl, col, ieq, out)'] }],
			derived: [{ id: 'ieq', dim: 'current', label: 'I_eq', name: 'Ersatzstrom', alts: ['current/m'], unit: '', units: [] }],
			formula: ['ieq = current / m', 'smin = lookup_ge(tbl, col, ieq, out)'], sets: [['current']],
			checks: [{ when: 'ieq>61', warn: 'Der Strom liegt außerhalb der geprüften Daten.' }]
		}),
		calc('fx_pick', 'Fixture: Zeile wählen', {
			vars: [v('x', 'number', 'x', 'Wert', { default: 1 }), v('n', 'number', 'n', 'Zeile', { input: false })],
			rules: [{ target: 'n', alts: ['round(x)'] }], formula: ['n = round(x)'], sets: [['x']]
		}),
		{ kind: 'code', id: 'fx_code', category: 'codes', code: 'resistor_bands', name: 'Fixture: Code', name_en: 'Fixture: Code', doc: '', tags: [], source: 'Fixture', data: {} }
	);
	return j;
}`

// tableState is the table page of the window as the tests read it.
type tableState struct {
	ID      string     `json:"id"`
	Title   string     `json:"title"`
	Banners []string   `json:"banners"`
	Heads   []string   `json:"heads"`
	Rows    [][]string `json:"rows"`
	Order   []string   `json:"order"`
	Match   []string   `json:"match"`
	Sort    []string   `json:"sort"`
	Count   string     `json:"count"`
	Hint    string     `json:"hint"`
	Notes   string     `json:"notes"`
	Source  string     `json:"source"`
}

const tableStateJS = `()=>{
	const p=document.querySelector('.ec-table-page');
	if(!p)return 'null';
	return JSON.stringify({
		id:p.dataset.table,title:p.querySelector('.ec-title').textContent,
		banners:[...p.querySelectorAll('.ec-banner')].map(b=>b.dataset.kind+': '+b.textContent),
		heads:[...p.querySelectorAll('thead th')].map(th=>th.querySelector('.ec-th-name').textContent),
		rows:[...p.querySelectorAll('tbody tr:not(.ec-table-empty)')].map(tr=>[...tr.children].map(c=>c.textContent)),
		order:[...p.querySelectorAll('tbody tr:not(.ec-table-empty)')].map(tr=>tr.dataset.row),
		match:[...p.querySelectorAll('tbody tr[data-match]')].map(tr=>tr.dataset.row),
		sort:[...p.querySelectorAll('thead th')].map(th=>th.getAttribute('aria-sort')),
		count:p.querySelector('.ec-table-count').textContent,hint:p.querySelector('.ec-table-hint').textContent,
		notes:p.querySelector('.ec-table-notes')?p.querySelector('.ec-table-notes').textContent:'',source:p.querySelector('.ec-source').textContent});
}`

// TestDesktopECalcTablesBrowser drives the table pages inside the real desktop shell with fixture tables injected into
// the served catalog: the Tables list sectioned by group, the banners, the finder header (fixed choice hidden, free choice
// shown, own stored state per table, highlight and scroll into view), the filter box and the "/" shortcut, sorting with
// empty cells last, the unit dropdowns and what they do to the copied text, the options object all page creators get, the
// message for a kind without a page, the settings, favorites, the opaque sticky header in every desktop skin and the
// phone-sized layout. The tables of the real catalog (the generated tables of Part A, the content of Part C) are opened
// one by one at the end. Set AURAGO_BROWSER_ARTIFACT_DIR to keep screenshots.
func TestDesktopECalcTablesBrowser(t *testing.T) {
	f := newECalcBrowserFixture(t, ecalcTablesFixturePatch)
	page := f.Page
	words := f.Words
	read := func() tableState {
		t.Helper()
		raw := page.MustEval(tableStateJS).Str()
		if raw == "null" {
			t.Fatal("no table page is open")
		}
		var state tableState
		if err := json.Unmarshal([]byte(raw), &state); err != nil {
			t.Fatalf("table state %s: %v", raw, err)
		}
		return state
	}
	openTable := func(id string) {
		t.Helper()
		page.MustElement(`.ec-rail-btn[data-rail="tables"]`).MustClick()
		page.MustElement(`.ec-item[data-id="` + id + `"]`).MustClick()
		f.Wait(`()=>document.querySelector('.ec-page')?.dataset.table==='` + id + `'`)
	}
	finderField := func(name string) string { return `.ec-finder .ec-field[data-var="` + name + `"] .ec-input` }
	typeInto := func(selector, text string) {
		t.Helper()
		el := page.MustElement(selector)
		el.MustClick()
		el.MustSelectAllText()
		el.MustInput(text)
	}
	chooseMethod := func(option string) {
		t.Helper()
		page.MustEval(`o=>{const r=document.querySelector('.ec-finder input[name$="-method"][value="'+o+'"]');r.checked=true;r.dispatchEvent(new Event('change',{bubbles:true}));}`, option)
	}
	same := func(label string, got, want any) {
		t.Helper()
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("%s: got %v, want %v", label, got, want)
		}
	}
	// A row is visible when it lies inside the scroll area below the sticky header.
	rowVisible := func(row string) bool {
		return page.MustEval(`r=>{const tr=document.querySelector('tbody tr[data-row="'+r+'"]');if(!tr)return false;
			const area=document.querySelector('.ec-table-scroll').getBoundingClientRect(),head=document.querySelector('.ec-table thead').getBoundingClientRect().height,box=tr.getBoundingClientRect();
			return box.top>=area.top+head-1&&box.bottom<=area.bottom+1;}`, row).Bool()
	}

	// ---- the app opens on a calculator; the rail shows Tables and Codes (they have items), Pinouts stays hidden -------------
	f.OpenApp(`.ec-page[data-calc]`)
	f.Wait(`()=>!!document.querySelector('.ec-rail-btn[data-rail="tables"]') && !!document.querySelector('.ec-rail-btn[data-rail="codes"]')`)
	if page.MustEval(`()=>!!document.querySelector('.ec-rail-btn[data-rail="pinouts"]')`).Bool() {
		t.Fatal("the Pinouts category has no items yet and must stay off the rail")
	}
	if got := page.MustEval(`()=>document.querySelector('.ec-rail-btn[data-rail="tables"] .ec-count').textContent`).Str(); got != "8" {
		t.Fatalf("the Tables rail entry counts %s tables, want the 8 of the fixture", got)
	}
	// The shell handed the tables to the registry of the table functions.
	if !page.MustEval(`()=>!!window.ECalcCore.tables&&typeof window.ECalcCore.tables.register==='function'&&!!window.ECalcCore.tables.get('fx_awg')`).Bool() {
		t.Fatal("the shell must register the catalog tables with ECalcCore.tables (the API of Part A)")
	}

	// ---- the Tables list is sectioned by group, in the order of the contract; an unknown group follows by its id ------------
	page.MustElement(`.ec-rail-btn[data-rail="tables"]`).MustClick()
	f.Wait(`()=>!!document.querySelector('.ec-item[data-id="fx_awg"]')`)
	list := page.MustEval(`()=>JSON.stringify([...document.querySelectorAll('.ec-items > li')].map(li=>li.classList.contains('ec-group')?'# '+li.textContent:li.querySelector('.ec-item').dataset.id))`).Str()
	wantList := fmt.Sprintf(`["# %s","fx_ampacity","fx_ampacity_al","fx_awg","# %s","fx_long","fx_plain","fx_ascii","fx_wide","# misc","fx_misc"]`, words["desktop.ecalc_group_conductors"], words["desktop.ecalc_group_general"])
	if list != wantList {
		t.Fatalf("the Tables list is %s, want %s", list, wantList)
	}
	if got := page.MustEval(`()=>document.querySelector('.ec-list-title').textContent`).Str(); got != "Tabellen" {
		t.Fatalf("the list heading is %q, want the German category name", got)
	}

	// ---- a table with a finder: banners, header, fixed choice hidden, the row of the default current is highlighted -------
	openTable("fx_ampacity")
	f.Wait(`()=>!!document.querySelector('.ec-finder .ec-field[data-var="current"] .ec-input')&&document.querySelector('.ec-finder .ec-field[data-var="current"] .ec-input').value!==''`)
	state := read()
	same("title", state.Title, "Fixture: Belastbarkeit Cu")
	same("banners", state.Banners, []string{"mains: " + words["desktop.ecalc_mains_note"], "guide: " + words["desktop.ecalc_guide_values"]})
	same("heads", state.Heads, []string{"Querschnitt", "Belastbarkeit", "Außendurchmesser", "Hinweis"})
	same("first row (the area in mm2 without the unit, a current with its unit, a length with the automatic prefix, a text)", state.Rows[0], []string{"1,5", "15,5 A", "1,382 mm", "—"})
	same("a null cell is a dash", state.Rows[2], []string{"4", "26 A", "—", "—"})
	same("a text cell", state.Rows[1][3], "siehe Anmerkung 2")
	same("count", state.Count, strings.ReplaceAll(words["desktop.ecalc_table_rows"], "{total}", "6"))
	same("highlighted row of 15 A", state.Match, []string{"0"})
	same("notes", strings.Contains(state.Notes, "Anmerkung 2: Gehäufte Verlegung."), true)
	same("source", state.Source, words["desktop.ecalc_source"]+": Fixture, keine Norm")
	same("a table with data in every column hides nothing", page.MustEval(`()=>!!document.querySelector('.ec-table-hidden')+'|'+document.querySelector('.ec-table-page').dataset.hiddenColumns`).Str(), "false|0")
	// The finder header holds a choice with twelve long options: the page does not scroll sideways because of it.
	same("the page does not scroll sideways", page.MustEval(`()=>{const h=document.querySelector('.ec-page-host');return h.scrollWidth>h.clientWidth+1;}`).Bool(), false)
	same("the doc is under the table", page.MustEval(`()=>{const d=document.querySelector('.ec-table-page > .ec-doc');return !!d&&d.getBoundingClientRect().top>=document.querySelector('.ec-table-wrap').getBoundingClientRect().bottom-1&&d.querySelectorAll('p').length===2;}`).Bool(), true)
	if page.MustEval(`()=>!!document.querySelector('.ec-finder [data-choice="material"]')`).Bool() || !page.MustEval(`()=>!!document.querySelector('.ec-finder [data-choice="method"]')`).Bool() {
		t.Fatal("the finder header hides the choice the table fixes (material) and keeps the free one (method)")
	}
	if page.MustEval(`()=>!!document.querySelector('.ec-finder .ec-sch-wrap, .ec-finder .ec-math-card, .ec-finder .ec-doc, .ec-finder [data-block="eseries"], .ec-finder [data-action="favorite"]')`).Bool() {
		t.Fatal("the finder header is compact: no schematic, formula, explanation, E-series block or favorite button")
	}
	if got := page.MustEval(`()=>document.querySelectorAll('.ec-page[data-calc]').length`).Int(); got != 0 {
		t.Fatalf("the finder header must not look like a calculator page: %d elements carry data-calc", got)
	}
	f.Shot("table-ampacity")

	// ---- the finder drives the highlight: typed currents move it, the free choice changes the equivalent current ------------
	typeInto(finderField("current"), "40")
	f.Wait(`()=>document.querySelector('tr[data-match]')?.dataset.row==='4'`)
	if got := page.MustEval(`()=>document.querySelector('tr[data-match]').getAttribute('aria-current')`).Str(); got != "true" {
		t.Fatalf("the highlighted row has aria-current %q", got)
	}
	chooseMethod("b")
	f.Wait(`()=>document.querySelector('tr[data-match]')?.dataset.row==='5'`) // 40 A / 0.8 = 50 A -> 16 mm2
	typeInto(finderField("current"), "100")
	f.Wait(`()=>!document.querySelector('tr[data-match]') && document.querySelector('.ec-finder .ec-field[data-var="smin"]').dataset.state==='error'`)
	same("no hint while the finder has no value", read().Hint, "")
	// A finder without a value (lookup_ge finds no row) shows the warning of its check in the header and highlights nothing.
	f.Shot("table-finder-warning")
	same("the header shows the check warning", page.MustEval(`()=>[...document.querySelectorAll('.ec-finder .ec-warnings li')].map(l=>l.textContent).join('|')`).Str(), "Der Strom liegt außerhalb der geprüften Daten.")
	typeInto(finderField("current"), "15")
	f.Wait(`()=>document.querySelector('tr[data-match]')?.dataset.row==='1' && document.querySelector('.ec-table-hint').textContent==='' && document.querySelectorAll('.ec-finder .ec-warnings li').length===0`) // 15 A / 0.8 -> 2.5 mm2

	// ---- the second table of the same finder has its own fixed choice and its own stored inputs ------------------------------
	openTable("fx_ampacity_al")
	f.Wait(`()=>document.querySelector('tr[data-match]')?.dataset.row==='1'`) // 15 A / 0.78 = 19.2 A -> 2.5 mm2
	if !page.MustEval(`()=>document.querySelector('.ec-finder input[name$="-method"][value="a"]').checked`).Bool() {
		t.Fatal("the method chosen on the copper table must not leak into the aluminium table")
	}
	typeInto(finderField("current"), "16")
	f.Wait(`()=>document.querySelector('tr[data-match]')?.dataset.row==='2'`) // 16 A / 0.78 = 20.5 A -> 4 mm2
	// The two columns without any value are not shown, and a line under the table counts them.
	state = read()
	same("the columns without data are hidden", state.Heads, []string{"Querschnitt", "Belastbarkeit", "Außendurchmesser", "Hinweis"})
	same("every row holds the shown cells only", len(state.Rows[0]), 4)
	f.Shot("table-hidden-columns")
	same("the line under the table counts the hidden columns", page.MustEval(`()=>document.querySelector('.ec-table-hidden').textContent+'|'+document.querySelector('.ec-table-page').dataset.hiddenColumns`).Str(), strings.ReplaceAll(words["desktop.ecalc_table_columns_hidden"], "{count}", "2")+"|2")
	// The finder calculator itself keeps its own state: material copper, method A, the default current.
	page.MustElement(`.ec-rail-btn[data-rail="wiring"]`).MustClick()
	page.MustElement(`.ec-item[data-id="fx_finder"]`).MustClick()
	f.Wait(`()=>document.querySelector('.ec-page')?.dataset.calc==='fx_finder'`)
	if got := page.MustEval(`()=>JSON.stringify([...document.querySelectorAll('.ec-choice')].map(c=>c.dataset.choice+'='+c.querySelector('input:checked').value))`).Str(); got != `["material=cu","method=a","routing=r1"]` {
		t.Fatalf("the finder calculator page shows %s, want both choices at their defaults", got)
	}
	stored := page.MustEval(`()=>Object.keys(JSON.parse(localStorage.getItem('aurago.ecalc.v1')).values).filter(k=>k.startsWith('fx_finder')).sort().join(',')`).Str()
	if stored != "fx_finder@fx_ampacity,fx_finder@fx_ampacity_al" {
		t.Fatalf("the headers store their state as %q, want one entry per table", stored)
	}
	// Back on the copper table the header has what it held: method B and the highlighted 2.5 mm2.
	openTable("fx_ampacity")
	f.Wait(`()=>document.querySelector('tr[data-match]')?.dataset.row==='1' && document.querySelector('.ec-finder input[name$="-method"][value="b"]').checked`)
	chooseMethod("a")
	f.Wait(`()=>document.querySelector('tr[data-match]')?.dataset.row==='0'`)

	// ---- the finder row is scrolled into view in a table longer than the scroll area ------------------------------------------
	openTable("fx_long")
	f.Wait(`()=>document.querySelector('tr[data-match]')?.dataset.row==='0'`)
	if !page.MustEval(`()=>{const s=document.querySelector('.ec-table-scroll');return s.scrollHeight>s.clientHeight+20;}`).Bool() {
		t.Fatal("the 60 rows of fx_long must scroll inside the table area")
	}
	if page.MustEval(`()=>{const h=document.querySelector('.ec-page-host');return h.scrollHeight>h.clientHeight+1;}`).Bool() {
		t.Fatal("on a tall window the page host must not scroll: the table scrolls inside its own area")
	}
	typeInto(finderField("x"), "57")
	f.Wait(`()=>document.querySelector('tr[data-match]')?.dataset.row==='56'`)
	if !rowVisible("56") {
		t.Fatal("the highlighted row 57 must be scrolled into view")
	}
	typeInto(finderField("x"), "2")
	f.Wait(`()=>document.querySelector('tr[data-match]')?.dataset.row==='1'`)
	if !rowVisible("1") {
		t.Fatal("the highlighted row 2 must be scrolled back into view")
	}
	// A solved value without a row: nothing is highlighted and the hint says so.
	typeInto(finderField("x"), "1000")
	f.Wait(`()=>!document.querySelector('tr[data-match]') && document.querySelector('.ec-table-hint').textContent.length>0`)
	same("hint when no row has the solved value", read().Hint, words["desktop.ecalc_table_no_match"])
	typeInto(finderField("x"), "2")
	f.Wait(`()=>document.querySelector('tr[data-match]')?.dataset.row==='1' && document.querySelector('.ec-table-hint').textContent===''`)
	// The header stays on top while the rows scroll.
	page.MustEval(`()=>{document.querySelector('.ec-table-scroll').scrollTop=100000;}`)
	if !page.MustEval(`()=>{const a=document.querySelector('.ec-table-scroll').getBoundingClientRect(),h=document.querySelector('.ec-table thead th').getBoundingClientRect();return Math.abs(h.top-a.top)<=1;}`).Bool() {
		t.Fatal("the header must stay at the top of the scroll area")
	}
	f.Shot("table-long")

	// ---- the filter box matches any shown cell, case-insensitively; Escape clears it; "/" focuses it ------------------------
	openTable("fx_ampacity")
	f.Wait(`()=>document.querySelectorAll('tbody tr[data-row]').length===6`)
	page.MustElement(`.ec-rail-btn[data-rail="tables"]`).MustFocus()
	page.KeyActions().Type(input.Slash).MustDo()
	f.Wait(`()=>document.activeElement&&document.activeElement.classList.contains('ec-filter-input')`)
	filter := page.MustElement(".ec-filter-input")
	filter.MustInput("ANMERKUNG")
	f.Wait(`()=>document.querySelectorAll('tbody tr[data-row]').length===2`)
	same("filtered order", read().Order, []string{"1", "3"})
	same("filtered count", read().Count, strings.NewReplacer("{shown}", "2", "{total}", "6").Replace(words["desktop.ecalc_table_rows_filtered"]))
	filter.MustSelectAllText().MustInput("1,382")
	f.Wait(`()=>document.querySelectorAll('tbody tr[data-row]').length===1`)
	same("a number matches its shown text", read().Order, []string{"0"})
	filter.MustSelectAllText().MustInput("gibt es nicht")
	f.Wait(`()=>!!document.querySelector('tbody tr.ec-table-empty')`)
	same("the empty message", page.MustEval(`()=>document.querySelector('tbody tr.ec-table-empty').textContent`).Str(), words["desktop.ecalc_table_no_rows"])
	filter.MustType(input.Escape)
	f.Wait(`()=>document.querySelectorAll('tbody tr[data-row]').length===6 && document.querySelector('.ec-filter-input').value===''`)
	// A slash typed into a field stays a character of the field.
	filter.MustInput("a/b")
	same("a slash typed into the filter", page.MustEval(`()=>document.querySelector('.ec-filter-input').value`).Str(), "a/b")
	filter.MustType(input.Escape)
	f.Wait(`()=>document.querySelectorAll('tbody tr[data-row]').length===6`)

	// ---- sorting: ascending, descending, table order; empty cells last either way --------------------------------------------
	sortBy := func(col string) { page.MustElement(`th[data-col="` + col + `"] .ec-th-sort`).MustClick() }
	sortBy("d")
	same("diameter ascending", read().Order, []string{"0", "1", "3", "4", "5", "2"})
	same("aria-sort ascending", read().Sort, []string{"none", "none", "ascending", "none"})
	sortBy("d")
	same("diameter descending", read().Order, []string{"5", "4", "3", "1", "0", "2"})
	same("aria-sort descending", read().Sort[2], "descending")
	sortBy("d")
	same("table order again", read().Order, []string{"0", "1", "2", "3", "4", "5"})
	same("aria-sort none", read().Sort, []string{"none", "none", "none", "none"})
	sortBy("note")
	same("text ascending, empty cells last", read().Order, []string{"4", "1", "3", "0", "2", "5"})
	sortBy("iz")
	same("sorting another column starts ascending", read().Sort, []string{"none", "ascending", "none", "none"})
	sortBy("iz")
	same("current descending", read().Order, []string{"5", "4", "3", "2", "1", "0"})
	sortBy("iz")
	same("table order after the third click", read().Order, []string{"0", "1", "2", "3", "4", "5"})

	// ---- the unit dropdowns: the area as AWG (bare numbers, the unit in the header), the diameter in in and mil ---------------
	setColumnUnit := func(col, unit string) {
		page.MustEval(`(c,u)=>{const s=document.querySelector('th[data-col="'+c+'"] .ec-th-unit');s.value=u;s.dispatchEvent(new Event('change',{bubbles:true}));}`, col, unit)
	}
	if got := page.MustEval(`()=>JSON.stringify([...document.querySelectorAll('th[data-col="section"] .ec-th-unit option')].map(o=>o.value+'|'+o.textContent))`).Str(); got != `["mm2|mm²","AWG|AWG"]` {
		t.Fatalf("the area dropdown offers %s, want exactly the units list of the column", got)
	}
	if page.MustEval(`()=>!!document.querySelector('th[data-col="iz"] .ec-th-unit')`).Bool() {
		t.Fatal("a column without a units list has no dropdown")
	}
	setColumnUnit("section", "AWG")
	f.Wait(`()=>document.querySelector('tbody tr[data-row="0"] th').textContent!=='1,5'`)
	if got := read().Rows[0][0]; !strings.HasPrefix(got, "15,4") {
		t.Fatalf("1.5 mm2 is %q AWG, want about 15,4", got)
	}
	setColumnUnit("d", "mil")
	f.Wait(`()=>document.querySelector('tbody tr[data-row="0"] td:nth-child(3)').textContent==='54,41'`)
	setColumnUnit("d", "in")
	f.Wait(`()=>document.querySelector('tbody tr[data-row="0"] td:nth-child(3)').textContent==='0,05441'`)
	setColumnUnit("d", "")
	f.Wait(`()=>document.querySelector('tbody tr[data-row="0"] td:nth-child(3)').textContent==='1,382 mm'`)
	setColumnUnit("d", "mil")
	f.Wait(`()=>document.querySelector('tbody tr[data-row="0"] td:nth-child(3)').textContent==='54,41'`)
	// The choice is remembered per table and column.
	if got := page.MustEval(`()=>{const u=JSON.parse(localStorage.getItem('aurago.ecalc.v1')).units;return u['fx_ampacity.section']+','+u['fx_ampacity.d'];}`).Str(); got != "AWG,mil" {
		t.Fatalf("the stored units are %q", got)
	}

	// ---- copy: the visible rows in the shown units, tab-separated -----------------------------------------------------------
	page.MustEval(`()=>{window.copied=null;Object.defineProperty(navigator,'clipboard',{configurable:true,value:{writeText:t=>{window.copied=t;return Promise.resolve();}}});}`)
	filter = page.MustElement(".ec-filter-input")
	filter.MustInput("anmerkung")
	f.Wait(`()=>document.querySelectorAll('tbody tr[data-row]').length===2`)
	sortBy("iz")
	sortBy("iz")
	same("sorted for the copy", read().Order, []string{"3", "1"})
	page.MustElement(`[data-action="copy"]`).MustClick()
	f.Wait(`()=>window.copied!==null`)
	copied := page.MustEval(`()=>window.copied`).Str()
	wantCopy := page.MustEval(`()=>{const p=document.querySelector('.ec-table-page');
		const head=[...p.querySelectorAll('thead th')].map(th=>{const s=th.querySelector('.ec-th-unit');const unit=s&&s.value?' ['+s.selectedOptions[0].textContent+']':'';return th.querySelector('.ec-th-name').textContent+unit;});
		return [head.join('\t')].concat([...p.querySelectorAll('tbody tr')].map(tr=>[...tr.children].map(c=>c.textContent).join('\t'))).join('\n');}`).Str()
	if copied != wantCopy {
		t.Fatalf("the copied text is %q, want what the table shows: %q", copied, wantCopy)
	}
	lines := strings.Split(copied, "\n")
	if len(lines) != 3 || lines[0] != "Querschnitt [AWG]\tBelastbarkeit\tAußendurchmesser [mil]\tHinweis" || !strings.Contains(lines[1], "\t34 A\t108,8\t") || !strings.Contains(lines[2], "\t19,5 A\t70,24\t") {
		t.Fatalf("the copied text %q has the wrong shape (header with the shown units, then the two visible rows, sorted)", copied)
	}
	if page.MustEval(`()=>document.querySelector('[data-action="copy"]').dataset.done`).Str() != "true" {
		t.Fatal("the copy button must confirm")
	}
	filter.MustType(input.Escape)
	f.Wait(`()=>document.querySelectorAll('tbody tr[data-row]').length===6`)
	// The columns without data are not in the copied text and not in what the filter searches.
	openTable("fx_ampacity_al")
	page.MustEval(`()=>{window.copied=null;}`)
	page.MustElement(`[data-action="copy"]`).MustClick()
	f.Wait(`()=>window.copied!==null`)
	alLines := strings.Split(page.MustEval(`()=>window.copied`).Str(), "\n")
	if len(alLines) != 7 || alLines[0] != "Querschnitt [mm²]\tBelastbarkeit\tAußendurchmesser\tHinweis" || strings.Count(alLines[1], "\t") != 3 {
		t.Fatalf("the copied text of a table with hidden columns is %q", alLines)
	}
	filter = page.MustElement(".ec-filter-input")
	filter.MustInput("—")
	f.Wait(`()=>document.querySelectorAll('tbody tr[data-row]').length===3`) // the rows with an empty shown cell; the hidden columns are empty everywhere and would match every row
	filter.MustType(input.Escape)
	f.Wait(`()=>document.querySelectorAll('tbody tr[data-row]').length===6`)
	// The remembered units survive leaving and returning to the table.
	openTable("fx_awg")
	openTable("fx_ampacity")
	f.Wait(`()=>document.querySelector('th[data-col="d"] .ec-th-unit').value==='mil'`)

	// ---- settings: three significant digits apply to a generated table (fx_awg), never to the cells of a transcribed one --------
	setColumnUnit("d", "")
	f.Wait(`()=>document.querySelector('tbody tr[data-row="0"] td:nth-child(3)').textContent==='1,382 mm'`)
	setDigits := func(digits string) {
		t.Helper()
		page.MustElement(`[data-action="settings"]`).MustClick()
		page.MustEval(`d=>{const s=document.querySelector('[data-set="sig"]');s.value=d;s.dispatchEvent(new Event('change',{bubbles:true}));}`, digits)
		page.MustElement(`[data-action="settings"]`).MustClick()
	}
	setDigits("3")
	same("a transcribed cell keeps its digits at three significant digits", read().Rows[0][2], "1,382 mm")
	openTable("fx_awg")
	f.Wait(`()=>document.querySelector('tbody tr[data-row="0"] td:nth-child(3)').textContent==='11,7 mm'`) // 4/0: 11.684 mm
	setDigits("4")
	f.Wait(`()=>document.querySelector('tbody tr[data-row="0"] td:nth-child(3)').textContent==='11,68 mm'`)
	openTable("fx_ampacity")
	f.Wait(`()=>document.querySelector('tbody tr[data-row="0"] td:nth-child(3)').textContent==='1,382 mm'`)

	// ---- favorites: the star of the table page, the rail and the list --------------------------------------------------------
	page.MustElement(`[data-action="favorite"]`).MustClick()
	f.Wait(`()=>document.querySelector('.ec-rail-btn[data-rail="favorites"] .ec-count').textContent==='1' && !!document.querySelector('.ec-item[data-id="fx_ampacity"] .ec-item-fav')`)
	page.MustElement(`[data-action="favorite"]`).MustClick()
	f.Wait(`()=>document.querySelector('.ec-rail-btn[data-rail="favorites"] .ec-count').textContent==='0'`)

	// ---- tables without banners, notes and finder; a text key column; the German decimal comma ------------------------------
	openTable("fx_plain")
	state = read()
	same("no banners", len(state.Banners), 0)
	same("no notes section", state.Notes, "")
	same("rows with a resistivity", state.Rows[0], []string{"Kupfer", "17,2 nΩ·m"})
	if page.MustEval(`()=>!!document.querySelector('.ec-finder:not([hidden])')`).Bool() || page.MustEval(`()=>!!document.querySelector('.ec-table-page > .ec-doc')`).Bool() {
		t.Fatal("a table without finder and doc shows neither")
	}
	openTable("fx_misc")
	state = read()
	same("a fixed unit without a units list: bare numbers in the cells", state.Rows, [][]string{{"1", "10"}, {"2", "2"}})
	same("the unit stands once in the header, without a dropdown", page.MustEval(`()=>JSON.stringify([[...document.querySelectorAll('.ec-th-unit-static')].map(e=>e.textContent),document.querySelectorAll('.ec-th-unit').length])`).Str(), `[["mil"],0]`)
	// A units list without "" offers only its explicit units: the number systems of an integer column, no automatic prefix.
	openTable("fx_ascii")
	same("an explicit-only units list", page.MustEval(`()=>JSON.stringify([...document.querySelectorAll('th[data-col="dec"] .ec-th-unit option')].map(o=>o.value+'|'+o.textContent))`).Str(), `["dec|dec","hex|hex","bin|bin","oct|oct"]`)
	same("decimal by default", read().Rows, [][]string{{"A", "65"}, {"B", "66"}, {"a", "97"}, {"0", "48"}})
	setColumnUnit("dec", "hex")
	f.Wait(`()=>document.querySelector('tbody tr[data-row="0"] td').textContent==='0x41'`)
	filter = page.MustElement(".ec-filter-input")
	filter.MustInput("0x42")
	f.Wait(`()=>document.querySelectorAll('tbody tr[data-row]').length===1`)
	same("the filter reads the number system shown", read().Order, []string{"1"})
	filter.MustType(input.Escape)
	f.Wait(`()=>document.querySelectorAll('tbody tr[data-row]').length===4`)
	setColumnUnit("dec", "bin")
	f.Wait(`()=>document.querySelector('tbody tr[data-row="0"] td').textContent==='0b1000001'`)
	setColumnUnit("dec", "oct")
	f.Wait(`()=>document.querySelector('tbody tr[data-row="0"] td').textContent==='0o101'`)
	sortBy("dec")
	sortBy("dec")
	same("sorted by the value, not by the text", read().Order, []string{"2", "1", "0", "3"})
	page.MustEval(`()=>{window.copied=null;}`)
	page.MustElement(`[data-action="copy"]`).MustClick()
	f.Wait(`()=>window.copied!==null`)
	same("the copied text names the number system", strings.Split(page.MustEval(`()=>window.copied`).Str(), "\n")[:2], []string{"Zeichen\tCode [oct]", "a\t0o141"})
	// A table wider than the window scrolls inside its own area: neither the page nor the (narrow) list moves sideways, and the
	// long name of the table does not widen the list.
	openTable("fx_wide")
	same("a table with sixteen columns", len(read().Heads), 16)
	same("a wide table scrolls inside its area, not the page", page.MustEval(`()=>{const h=document.querySelector('.ec-page-host'),s=document.querySelector('.ec-table-scroll'),l=document.querySelector('.ec-list');return JSON.stringify([h.scrollWidth>h.clientWidth+1,s.scrollWidth>s.clientWidth+200,l.scrollWidth>l.clientWidth+1]);}`).Str(), `[false,true,false]`)
	page.MustEval(`()=>{document.querySelector('.ec-table-scroll').scrollLeft=100000;}`)
	same("the page stays put while the area scrolls to its end", page.MustEval(`()=>{const h=document.querySelector('.ec-page-host'),s=document.querySelector('.ec-table-scroll');return JSON.stringify([h.scrollLeft,s.scrollLeft>0]);}`).Str(), `[0,true]`)
	f.Shot("table-wide")
	openTable("fx_awg")
	state = read()
	same("awg rows", len(state.Rows), 21)
	same("awg first row", state.Rows[0][:2], []string{"4/0", "-3"})
	same("awg last row", state.Rows[20][:2], []string{"40", "40"})
	same("the text key column is the row header", page.MustEval(`()=>document.querySelector('tbody tr:first-child th').scope`).Str(), "row")
	f.Shot("table-awg")

	// ---- the top search finds tables by name -----------------------------------------------------------------------------------
	search := page.MustElement(".ec-search-input")
	search.MustInput("quadratzahlen")
	f.Wait(`()=>document.querySelectorAll('.ec-items .ec-item').length===1 && document.querySelector('.ec-items .ec-item').dataset.id==='fx_long'`)
	if got := page.MustEval(`()=>document.querySelector('.ec-items .ec-item-sub').textContent`).Str(); got != "Tabellen" {
		t.Fatalf("a table hit names its category: %q", got)
	}
	search.MustType(input.Enter)
	f.Wait(`()=>document.querySelector('.ec-page')?.dataset.table==='fx_long'`)
	search.MustType(input.Escape)

	// ---- one options object for all three page creators; a kind without a page creator says so -------------------------------
	page.MustEval(`()=>{
		window.pageOptions={};
		const record=(name,Page)=>{const create=Page.create;Page.create=options=>{window.pageOptions[name]=Object.keys(options).sort().join(',');return create(options);};};
		record('calculator',window.ECalcCalcPage);
		record('table',window.ECalcTablePage);
		window.ECalcCodePage={create(options){window.pageOptions.code=Object.keys(options).sort().join(',');
			const el=document.createElement('article');el.className='ec-page';el.dataset.codeStub=options.item.id;el.textContent=options.item.name;return {el,dispose(){window.codeDisposed=true;}};}};
	}`)
	page.MustElement(`.ec-rail-btn[data-rail="codes"]`).MustClick()
	page.MustElement(`.ec-item[data-id="fx_code"]`).MustClick()
	f.Wait(`()=>!!document.querySelector('.ec-page[data-code-stub="fx_code"]')`)
	openTable("fx_plain")
	f.Wait(`()=>window.codeDisposed===true`)
	page.MustElement(`.ec-rail-btn[data-rail="wiring"]`).MustClick()
	page.MustElement(`.ec-item[data-id="fx_pick"]`).MustClick()
	f.Wait(`()=>document.querySelector('.ec-page')?.dataset.calc==='fx_pick'`)
	const optionKeys = "decimalComma,getItem,icon,item,onFavorite,schematics,store,t,uid,units"
	if got := page.MustEval(`()=>JSON.stringify(Object.fromEntries(Object.entries(window.pageOptions).sort()))`).Str(); got != fmt.Sprintf(`{"calculator":%q,"code":%q,"table":%q}`, optionKeys, optionKeys, optionKeys) {
		t.Fatalf("the page creators received %s, want the same keys %q everywhere", got, optionKeys)
	}
	page.MustEval(`()=>{delete window.ECalcCodePage;}`)
	page.MustElement(`.ec-rail-btn[data-rail="codes"]`).MustClick()
	page.MustElement(`.ec-item[data-id="fx_code"]`).MustClick()
	f.Wait(`()=>!!document.querySelector('.ec-page[data-unavailable="fx_code"]')`)
	same("the message for a kind without a page", page.MustEval(`()=>document.querySelector('.ec-empty-page').textContent`).Str(), words["desktop.ecalc_page_unavailable"])

	// ---- the sticky header is (practically) opaque in every desktop skin: rows scrolling under it must not show through --------------------------------------------------------------------
	openTable("fx_long")
	for _, theme := range parseDesktopThemeRegistry(t) {
		names := []string{theme.ID}
		if theme.ID == "fruity" {
			names = []string{"fruity-light", "fruity-dark"}
		}
		for _, name := range names {
			page.MustEval(`id=>fixtureTheme(id)`, name)
			alpha := page.MustEval(`()=>{const c=getComputedStyle(document.querySelector('.ec-table thead th')).backgroundColor;
				const m=c.match(/^rgba?\(([^)]+)\)/);if(m){const p=m[1].split(/[ ,/]+/).filter(Boolean);return p.length>3?Number(p[3]):1;}
				return /^color\(/.test(c)?(c.includes('/')?Number(c.split('/')[1].replace(')','')):1):0;}`).Num()
			if alpha < 0.9 {
				t.Fatalf("skin %s: the sticky header background is translucent (alpha %v)", name, alpha)
			}
			if name == "fruity-dark" || name == "cyberpunk" || name == "minimal" {
				f.Shot("table-long-" + name)
			}
		}
	}
	page.MustEval(`()=>fixtureTheme('standard')`)

	// ---- every table of the real catalog (Part A's generated tables, Part C's content) opens with all its rows -----------------------
	f.CloseWindows()
	page.MustEval(`()=>{window.ecalcWithReal=true;}`)
	f.OpenApp(`.ec-page`)
	// The pre-fill bridge opens a table page by its id (values and choices only apply to calculators).
	winID := page.MustEval(`()=>[...aurora.state.windows.keys()][0]`).Str()
	if !page.MustEval(`id=>window.ECalcApp.launch(id,{page:'fx_awg',values:{x:'1'},choices:{y:'z'}})`, winID).Bool() {
		t.Fatal("launch refused a payload for the open window")
	}
	f.Wait(`()=>document.querySelector('.ec-page')?.dataset.table==='fx_awg'`)
	for _, id := range strings.Split(strings.Trim(page.MustEval(`()=>window.ecalcRealTableIDs.join(',')`).Str(), ","), ",") {
		if id == "" {
			continue
		}
		// The rail and the list entry are clicked by script: the fixture window reaches under the taskbar, which covers the
		// lower entries of a list of fifty tables.
		page.MustEval(`id=>{document.querySelector('.ec-rail-btn[data-rail="tables"]').click();document.querySelector('.ec-item[data-id="'+id+'"]').click();}`, id)
		f.Wait(`()=>document.querySelector('.ec-page')?.dataset.table==='` + id + `'`)
		raw := page.MustEval(`id=>{
			const p=document.querySelector('.ec-table-page'),h=document.querySelector('.ec-page-host'),s=document.querySelector('.ec-table-scroll'),l=document.querySelector('.ec-list');
			const item=window.ECalcCore.tables.get(id).item,cols=item.columns,rows=item.rows;
			const empty=cols.filter((c,i)=>rows.length&&rows.every(r=>r[i]===null)).length;
			const selects=[...p.querySelectorAll('.ec-th-unit')];
			const unitsOk=selects.every(sel=>{const col=cols.find(c=>c.id===sel.closest('th').dataset.col);const allowed=col.units||[];return !allowed.length||allowed.includes('')||[...sel.options].every(o=>allowed.includes(o.value));});
			const warnings=p.querySelectorAll('.ec-finder .ec-warnings li').length,matches=p.querySelectorAll('tbody tr[data-match]').length;
			return JSON.stringify({rows:p.querySelectorAll('tbody tr[data-row]').length,heads:p.querySelectorAll('thead th').length,wantHeads:cols.length-empty,
				sideways:h.scrollWidth>h.clientWidth+1,listSideways:l.scrollWidth>l.clientWidth+1,areaBounded:s.getBoundingClientRect().height<=h.clientHeight,
				cells:[...p.querySelectorAll('tbody td,tbody th')].filter(c=>/NaN|undefined|Infinity/.test(c.textContent)).length,
				finder:!!item.finder,header:!!p.querySelector('.ec-finder:not([hidden])'),settled:matches===1||warnings>0||p.querySelector('.ec-table-hint').textContent!=='',matches,unitsOk});}`, id).Str()
		var real struct {
			Rows, Heads, WantHeads, Cells, Matches int
			Sideways, ListSideways, AreaBounded    bool
			Finder, Header, Settled, UnitsOk       bool
		}
		if err := json.Unmarshal([]byte(raw), &real); err != nil {
			t.Fatal(err)
		}
		// A wide table scrolls inside its own area: neither the page nor the list moves sideways, the area stays inside the window.
		if real.Rows == 0 || real.Heads == 0 || real.Sideways || real.ListSideways || !real.AreaBounded || real.Cells != 0 {
			t.Fatalf("real table %s: %s", id, raw)
		}
		// Columns without any value are the only ones that are hidden; a table with a finder shows its header, and the finder
		// either highlights exactly one row, warns, or says that no row matches; a units list never offers a unit it does not list.
		if real.Heads != real.WantHeads || (real.Finder && (!real.Header || !real.Settled || real.Matches > 1)) || !real.UnitsOk {
			t.Fatalf("real table %s: %s", id, raw)
		}
		wantRows := page.MustEval(`id=>window.ECalcCore.tables.get(id).rows.length`, id).Int()
		if wantRows != real.Rows {
			t.Fatalf("real table %s shows %d rows, the registry holds %d", id, real.Rows, wantRows)
		}
		// The number systems of the ASCII table: the column with a radix units list switches to hex.
		if id == "ascii" {
			page.MustEval(`()=>{const s=[...document.querySelectorAll('.ec-th-unit')].find(x=>[...x.options].some(o=>o.value==='hex'));s.value='hex';s.dispatchEvent(new Event('change',{bubbles:true}));}`)
			f.Wait(`()=>{const s=[...document.querySelectorAll('.ec-th-unit')].find(x=>[...x.options].some(o=>o.value==='hex'));const i=[...document.querySelectorAll('thead th')].indexOf(s.closest('th'));return /^0x[0-9A-F_]+$/.test(document.querySelector('tbody tr').children[i].textContent);}`)
		}
	}

	// ---- a phone-sized window: the table scrolls inside its own area, the page does not move sideways --------------------------------
	f.CloseWindows()
	page.MustSetViewport(390, 667, 1, false)
	page.MustEval(`()=>fixtureTheme('fruity-dark')`)
	f.OpenApp(`.ec-app[data-compact="true"]`)
	f.Wait(`()=>!!document.querySelector('.ec-page')`)
	page.MustElement(".ec-back").MustClick()
	// The fixture window reaches under the taskbar, which covers the lower rail entries: the rail button is clicked by script.
	page.MustEval(`()=>document.querySelector('.ec-rail-btn[data-rail="tables"]').click()`)
	page.MustElement(`.ec-item[data-id="fx_awg"]`).MustClick()
	f.Wait(`()=>document.querySelector('.ec-page')?.dataset.table==='fx_awg'`)
	if got := page.MustEval(`()=>{const h=document.querySelector('.ec-page-host'),s=document.querySelector('.ec-table-scroll');return (h.scrollWidth>h.clientWidth+1?'the page scrolls sideways; ':'')+(s.scrollWidth<=s.clientWidth+1?'the table does not scroll inside its area':'');}`).Str(); got != "" {
		t.Fatal(got)
	}
	f.Shot("table-compact")
	page.MustElement(".ec-back").MustClick()
	page.MustElement(`.ec-item[data-id="fx_ampacity"]`).MustClick()
	f.Wait(`()=>document.querySelector('.ec-page')?.dataset.table==='fx_ampacity' && !!document.querySelector('tr[data-match]')`)
	if got := page.MustEval(`()=>{const h=document.querySelector('.ec-page-host');return h.scrollWidth>h.clientWidth+1?'the page scrolls sideways':'';}`).Str(); got != "" {
		t.Fatal(got)
	}
	f.Shot("table-compact-finder")

	if errors := page.MustEval(`()=>JSON.stringify(fixtureErrors)`).Str(); errors != "[]" {
		t.Fatal(errors)
	}
}
