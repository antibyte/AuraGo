package ui

import (
	"strings"
	"testing"
)

// TestDesktopECalcSpacingTableFinderBrowser opens the IPC-2221 spacing tables of the real catalog with their finder. The
// per-volt row (millimetres per volt in the length columns, no voltage band) is never the highlighted row: 400 V
// highlights the 301 to 500 V band, 600 V highlights nothing and the finder still computes the spacing from the
// increment of that row.
func TestDesktopECalcSpacingTableFinderBrowser(t *testing.T) {
	f := newECalcBrowserFixture(t, `j => j`)
	page := f.Page
	f.OpenApp(`.ec-page[data-calc]`)
	for _, c := range []struct{ id, spacing600 string }{{"ipc2221c_conductor_spacing", "3"}, {"ipc2221b_conductor_spacing", "3"}} {
		page.MustEval(`id=>{document.querySelector('.ec-rail-btn[data-rail="tables"]').click();document.querySelector('.ec-item[data-id="'+id+'"]').click();}`, c.id)
		f.Wait(`()=>document.querySelector('.ec-page')?.dataset.table==='` + c.id + `'`)
		f.Wait(`()=>document.querySelectorAll('.ec-table-page tbody tr[data-row]').length===10 && !!document.querySelector('.ec-finder .ec-field[data-var="v"] .ec-input')`)
		matched := func() string {
			return page.MustEval(`()=>JSON.stringify([...document.querySelectorAll('.ec-table-page tbody tr[data-match]')].map(tr=>tr.firstElementChild.textContent))`).Str()
		}
		// The default 400 V belongs to the 301 to 500 V band.
		f.Wait(`()=>document.querySelectorAll('.ec-table-page tbody tr[data-match]').length===1`)
		if got := matched(); got != `["301–500 V"]` {
			t.Fatalf("%s at the default voltage highlights %s, want the 301 to 500 V band", c.id, got)
		}
		// 600 V is above the table: no band matches (and in particular not the per-volt row), the spacing comes from the increment.
		field := page.MustElement(`.ec-finder .ec-field[data-var="v"] .ec-input`)
		field.MustClick()
		field.MustSelectAllText()
		field.MustInput("600")
		f.Wait(`()=>document.querySelectorAll('.ec-table-page tbody tr[data-match]').length===0 && document.querySelector('.ec-table-hint').textContent.length>0`)
		if got := matched(); got != `[]` {
			t.Fatalf("%s at 600 V highlights %s, want no row (the per-volt row is no band)", c.id, got)
		}
		if got := page.MustEval(`()=>document.querySelector('.ec-finder [data-rows="derived"] li:last-child .ec-row-val').textContent`).Str(); !strings.HasPrefix(strings.ReplaceAll(got, ",", "."), c.spacing600+" mm") {
			t.Errorf("%s at 600 V: the finder computes the spacing %q, want %s mm", c.id, got, c.spacing600)
		}
		field.MustSelectAllText().MustInput("250")
		f.Wait(`()=>document.querySelector('.ec-table-page tbody tr[data-match] th')?.textContent==='171–250 V'`)
	}
	if errors := page.MustEval(`()=>JSON.stringify(fixtureErrors)`).Str(); errors != "[]" {
		t.Fatal(errors)
	}
}
