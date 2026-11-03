package ui

import (
	"encoding/json"
	"testing"
)

// TestDesktopECalcTranscribedCellsBrowser opens real tables of the catalog in the table page. A transcribed cell is a
// published number (a band edge, a limit) and the page never rounds it below the digits it was written with, whatever the
// significant-digit setting says: 434.79 MHz is not 434.8 MHz (outside the band), 2483.5 MHz is not 2.483 GHz, and at three
// digits 868.7 to 869.2 MHz is not 869 to 869 MHz. The cells of a generated table (awg) follow the setting. The filter
// reads the shown text, and the sticky header and the scroll area still fit. Set AURAGO_RUN_BROWSER_SMOKE=1.
func TestDesktopECalcTranscribedCellsBrowser(t *testing.T) {
	f := newECalcBrowserFixture(t, `j => j`)
	page := f.Page
	f.OpenApp(`.ec-page[data-calc]`)
	openTable := func(id string) {
		t.Helper()
		page.MustEval(`id=>{document.querySelector('.ec-rail-btn[data-rail="tables"]').click();document.querySelector('.ec-item[data-id="'+id+'"]').click();}`, id)
		f.Wait(`()=>document.querySelector('.ec-page')?.dataset.table==='` + id + `'`)
	}
	// The texts of the cells of the rows that pass the filter, one string array per row (the catalog is served in German).
	cells := func() [][]string {
		t.Helper()
		var rows [][]string
		raw := page.MustEval(`()=>JSON.stringify([...document.querySelectorAll('.ec-table-page tbody tr[data-row]')].map(tr=>[...tr.children].map(c=>c.textContent)))`).Str()
		if err := json.Unmarshal([]byte(raw), &rows); err != nil {
			t.Fatal(err)
		}
		return rows
	}
	has := func(rows [][]string, want string) bool {
		for _, row := range rows {
			for _, cell := range row {
				if cell == want {
					return true
				}
			}
		}
		return false
	}
	need := func(label string, rows [][]string, wants ...string) {
		t.Helper()
		for _, want := range wants {
			if !has(rows, want) {
				t.Errorf("%s: no cell reads %q; the table shows %v", label, want, rows)
			}
		}
	}
	setDigits := func(digits string) {
		t.Helper()
		page.MustElement(`[data-action="settings"]`).MustClick()
		page.MustEval(`d=>{const s=document.querySelector('[data-set="sig"]');s.value=d;s.dispatchEvent(new Event('change',{bubbles:true}));}`, digits)
		page.MustElement(`[data-action="settings"]`).MustClick()
	}

	// ---- the default digits (4): the edges are the published numbers ---------------------------------------------------------
	openTable("ism_bands")
	f.Wait(`()=>document.querySelectorAll('.ec-table-page tbody tr[data-row]').length>10`)
	rows := cells()
	need("ism_bands at the default digits", rows, "433,05 MHz", "434,79 MHz", "2,4835 GHz", "868,7 MHz", "869,2 MHz")
	f.Shot("table-transcribed")

	// ---- the filter reads the full text -------------------------------------------------------------------------------------
	filter := page.MustElement(".ec-filter-input")
	filter.MustInput("434,79")
	f.Wait(`()=>{const n=document.querySelectorAll('.ec-table-page tbody tr[data-row]').length;return n>0&&n<10;}`)
	filtered := cells()
	need("the filter 434,79", filtered, "434,79 MHz")
	filter.MustSelectAllText().MustInput("2,4835 GHz")
	f.Wait(`()=>[...document.querySelectorAll('.ec-table-page tbody tr[data-row]')].every(tr=>tr.textContent.includes('2,4835 GHz'))&&document.querySelectorAll('.ec-table-page tbody tr[data-row]').length>0`)
	filter.MustSelectAllText().MustInput("")

	// ---- three digits: a transcribed cell keeps its digits, a generated cell follows the setting ------------------------------
	setDigits("3")
	f.Wait(`()=>document.querySelectorAll('.ec-table-page tbody tr[data-row]').length>10`)
	rows = cells()
	need("ism_bands at three digits", rows, "434,79 MHz", "2,4835 GHz", "868,7 MHz", "869,2 MHz")
	if has(rows, "869 MHz") {
		t.Errorf("a band edge was rounded to 869 MHz at three digits: %v", rows)
	}
	openTable("amateur_bands_de")
	f.Wait(`()=>document.querySelectorAll('.ec-table-page tbody tr[data-row]').length>10`)
	need("amateur_bands_de at three digits", cells(), "18,168 MHz")
	openTable("awg")
	f.Wait(`()=>document.querySelectorAll('.ec-table-page tbody tr[data-row]').length===44`)
	need("awg at three digits (generated: 4/0 is 11.684 mm)", cells(), "11,7 mm")
	setDigits("4")
	f.Wait(`()=>[...document.querySelectorAll('.ec-table-page tbody td')].some(td=>td.textContent==='11,68 mm')`)
	need("awg at four digits", cells(), "11,68 mm")

	// ---- the sticky header and the scroll area still fit ---------------------------------------------------------------------
	openTable("ism_bands")
	f.Wait(`()=>document.querySelectorAll('.ec-table-page tbody tr[data-row]').length>10`)
	layout := page.MustEval(`()=>{
		const h=document.querySelector('.ec-page-host'),s=document.querySelector('.ec-table-scroll');
		s.scrollTop=100000;
		const a=s.getBoundingClientRect(),th=document.querySelector('.ec-table thead th').getBoundingClientRect();
		return JSON.stringify({pageSideways:h.scrollWidth>h.clientWidth+1,areaBounded:a.height<=h.clientHeight,headAtTop:Math.abs(th.top-a.top)<=1,areaScrolls:s.scrollWidth>=s.clientWidth});}`).Str()
	if layout != `{"pageSideways":false,"areaBounded":true,"headAtTop":true,"areaScrolls":true}` {
		t.Errorf("ism_bands layout = %s", layout)
	}
	if errors := page.MustEval(`()=>JSON.stringify(fixtureErrors)`).Str(); errors != "[]" {
		t.Fatal(errors)
	}
}
