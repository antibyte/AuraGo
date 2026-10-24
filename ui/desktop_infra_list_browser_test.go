package ui

import (
	"testing"
	"time"

	"github.com/go-rod/rod/lib/input"
)

// TestDesktopInfraListViewBrowser covers the List view: rows keep the 54 px height the virtualization relies
// on, columns sort (one aria-sort, remembered), hosts open to show their guests one level down, checkboxes
// select (Shift selects a range, the keyboard keeps the focus on the checkbox), the bulk bar moves a
// selection into a place and removes assets after a confirmation.
func TestDesktopInfraListViewBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	fixture := newInfraAppBrowserFixture()
	fixture.world(25)
	fixture.nodes = append(fixture.nodes, infraCanvasFixtureNode("room-b", "room", "Room B", "room", map[string]any{"x": 2400, "y": 100, "w": 480, "h": 320, "pinned": true}, ""))
	page := infraBrowserPageWithFixture(t, fixture)
	defer page.Close()
	page.MustSetViewport(1920, 1200, 1, false)
	openInfraBrowserApp(page)
	page.MustWait(`()=>!!document.querySelector('[data-node="fixture-node-0003"]')`)
	page.MustEval(`()=>{const win=document.querySelector('.vd-window[data-app-id="infra"]');win.classList.remove('maximized','vd-mobile-forced-maximized');win.style.right='auto';win.style.width='1500px';win.style.height='970px';win.style.left='210px';win.style.top='70px';window.w=(key,vars={})=>Object.entries(vars).reduce((s,[k,v])=>s.replaceAll('{'+k+'}',v),window.fixtureWords[key]);}`)
	page.MustWait(`()=>document.querySelector('.infra-app').clientWidth>1000`)
	page.MustEval(`()=>document.querySelector('[data-view="table"]').click()`)
	page.MustWait(`()=>document.querySelectorAll('[data-table-body] [data-row]').length>10`)

	// The 54 px row height, one sorted column and the default order (name, ascending).
	got := page.MustEval(`()=>JSON.stringify({
		height:Math.round(document.querySelector('[data-table-body] [data-row]').getBoundingClientRect().height),
		sorted:[...document.querySelectorAll('[data-sort-col][aria-sort]')].map(th=>th.dataset.sortCol+':'+th.getAttribute('aria-sort')),
		total:document.querySelector('[data-table-total]').textContent===window.w('desktop.infra_asset_count',{count:26}),
	})`).Str()
	if want := `{"height":54,"sorted":["name:ascending"],"total":true}`; got != want {
		t.Fatalf("list view starts as %s; want %s", got, want)
	}
	page.MustEval(`()=>document.querySelector('[data-sort="name"]').click()`)
	page.MustWait(`()=>document.querySelector('[data-sort-col="name"]').getAttribute('aria-sort')==='descending'`)
	descending := `()=>{const names=[...document.querySelectorAll('[data-table-body] [data-row][data-depth="0"] .infra-table-name span')].map(item=>item.textContent);const cmp=(a,b)=>a.localeCompare(b,undefined,{numeric:true,sensitivity:'base'});return names.length>5&&names.every((name,index)=>index===0||cmp(names[index-1],name)>=0);}`
	if !page.MustEval(descending).Bool() {
		t.Fatal("sorting the name column twice must list the names from Z to A")
	}

	// A second column replaces the first; the choice is remembered in the browser.
	page.MustEval(`()=>document.querySelector('[data-sort="address"]').click()`)
	page.MustWait(`()=>document.querySelector('[data-sort-col="address"]').getAttribute('aria-sort')==='ascending'&&!document.querySelector('[data-sort-col="name"]').hasAttribute('aria-sort')`)
	if !page.MustEval(`()=>{const saved=JSON.parse(localStorage.getItem('infra.table.v1')||'{}');const ips=[...document.querySelectorAll('[data-table-body] [data-row] .infra-col-address')].map(cell=>cell.textContent).filter(text=>/^\d/.test(text)).map(text=>text.split('.').reduce((sum,part)=>sum*256+Number(part),0));return saved.sort==='address'&&saved.dir==='asc'&&ips.length>5&&ips.every((ip,index)=>index===0||ips[index-1]<=ip);}`).Bool() {
		t.Fatal("sorting by address must order the addresses numerically and be remembered")
	}

	// A host opens to show its guest one level down.
	page.MustEval(`()=>document.querySelector('[data-row-expand="fixture-node-0002"]').click()`)
	page.MustWait(`()=>document.querySelector('[data-row="fixture-node-0001"]')?.dataset.depth==='1'&&document.querySelector('[data-row-expand="fixture-node-0002"]').getAttribute('aria-expanded')==='true'`)

	// A click selects one row, Shift extends to a range; the bulk bar counts the selection.
	page.MustEval(`()=>document.querySelector('[data-row-check="fixture-node-0003"]').click()`)
	page.MustWait(`()=>document.querySelector('[data-row="fixture-node-0003"]').classList.contains('is-selected')`)
	page.MustEval(`()=>document.querySelector('[data-row-check="fixture-node-0006"]').dispatchEvent(new MouseEvent('click',{bubbles:true,cancelable:true,shiftKey:true}))`)
	page.MustWait(`()=>!document.querySelector('[data-bulkbar]').hidden&&document.querySelector('[data-bulk-count]').textContent===window.w('desktop.infra_selected_count',{count:4})`)
	if !page.MustEval(`()=>['0003','0004','0005','0006'].every(id=>document.querySelector('[data-row="fixture-node-'+id+'"]').classList.contains('is-selected'))&&document.querySelector('[data-table-check-all]').indeterminate`).Bool() {
		t.Fatal("Shift+click must select the whole range between the two rows and leave the header check indeterminate")
	}

	// The keyboard keeps working across the repaint a selection causes.
	page.MustEval(`()=>document.querySelector('[data-row="fixture-node-0010"] [data-row-check]').focus()`)
	page.Keyboard.MustType(input.Space)
	page.MustWait(`()=>document.querySelector('[data-bulk-count]').textContent===window.w('desktop.infra_selected_count',{count:5})`)
	if !page.MustEval(`()=>document.activeElement?.dataset?.rowCheck==='fixture-node-0010'`).Bool() {
		t.Fatal("selecting a row with the keyboard must leave the focus on its checkbox")
	}

	// Move to place: the selection joins Room B (layout save or asset updates, depending on the map state).
	page.MustEval(`()=>document.querySelector('[data-row="fixture-node-0010"] [data-row-check]').click()`)
	page.MustWait(`()=>document.querySelector('[data-bulk-count]').textContent===window.w('desktop.infra_selected_count',{count:4})`)
	page.MustEval(`()=>document.querySelector('[data-action="bulk-move"]').click()`)
	page.MustWait(`()=>!!document.querySelector('[data-picker] [data-place-option="room-b"]')`)
	page.MustEval(`()=>document.querySelector('[data-picker] [data-place-option="room-b"]').click()`)
	parentOf := func(id string) any {
		fixture.mu.Lock()
		defer fixture.mu.Unlock()
		for _, node := range fixture.nodes {
			if node["id"] == id {
				return node["parent_id"]
			}
		}
		return nil
	}
	deadline := time.Now().Add(8 * time.Second)
	for _, id := range []string{"fixture-node-0003", "fixture-node-0004", "fixture-node-0005", "fixture-node-0006"} {
		for parentOf(id) != "room-b" {
			if time.Now().After(deadline) {
				t.Fatalf("bulk move left %s in %v; want room-b", id, parentOf(id))
			}
			time.Sleep(50 * time.Millisecond)
		}
	}
	page.MustWait(`()=>[...document.querySelectorAll('[data-row="fixture-node-0003"] td')][3]?.textContent.trim()==='Room B'`)

	// Clearing the selection hides the bar; check-all selects every row and a second click clears it again.
	page.MustEval(`()=>document.querySelector('[data-action="bulk-clear"]').click()`)
	page.MustWait(`()=>document.querySelector('[data-bulkbar]').hidden`)
	page.MustEval(`()=>document.querySelector('[data-table-check-all]').click()`)
	page.MustWait(`()=>!document.querySelector('[data-bulkbar]').hidden&&document.querySelector('[data-table-check-all]').checked`)
	page.MustEval(`()=>document.querySelector('[data-table-check-all]').click()`)
	page.MustWait(`()=>document.querySelector('[data-bulkbar]').hidden`)

	// Remove: two rows go after the confirmation, one delete request each.
	page.MustEval(`()=>{document.querySelector('[data-row-check="fixture-node-0007"]').click();}`)
	page.MustEval(`()=>document.querySelector('[data-row-check="fixture-node-0008"]').click()`)
	page.MustWait(`()=>document.querySelector('[data-bulk-count]').textContent===window.w('desktop.infra_selected_count',{count:2})`)
	page.MustEval(`()=>document.querySelector('[data-action="bulk-remove"]').click()`)
	page.MustWait(`()=>!!document.querySelector('.vd-modal [data-choice="discard"]')`)
	page.MustEval(`()=>document.querySelector('.vd-modal [data-choice="discard"]').click()`)
	page.MustWait(`()=>!document.querySelector('[data-row="fixture-node-0007"]')&&!document.querySelector('[data-row="fixture-node-0008"]')`)
	fixture.mu.Lock()
	deletes := len(fixture.nodeDeletes)
	fixture.mu.Unlock()
	if deletes != 2 {
		t.Fatalf("bulk remove sent %d delete requests, want 2", deletes)
	}
	closeInfraBrowserWindow(t, page)
	requireInfraBrowserClean(t, page)
}
