package ui

import "testing"

// TestDesktopInfraInspectorOverviewBrowser covers the wording of the inspector Overview: counts use the
// singular text for exactly one ("1 connection"), a placed asset reads "On the map" without coordinates (they
// live in the Details tab as read-only text) and an unplaced one says so.
func TestDesktopInfraInspectorOverviewBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	fixture := newInfraAppBrowserFixture()
	fixture.world(25)
	fixture.nodes = append(fixture.nodes, infraCanvasFixtureNode("dev-unplaced", "device", "Cold NAS", "nas", nil, ""))
	page := infraBrowserPageWithFixture(t, fixture)
	defer page.Close()
	page.MustSetViewport(1920, 1200, 1, false)
	openInfraBrowserApp(page)
	page.MustWait(`()=>!!document.querySelector('[data-node="fixture-node-0003"]')`)
	page.MustEval(`()=>{const win=document.querySelector('.vd-window[data-app-id="infra"]');win.classList.remove('maximized','vd-mobile-forced-maximized');win.style.right='auto';win.style.width='1500px';win.style.height='970px';win.style.left='210px';win.style.top='70px';window.w=(key,vars={})=>Object.entries(vars).reduce((s,[k,v])=>s.replaceAll('{'+k+'}',v),window.fixtureWords[key]);window.fact=label=>{const dt=[...document.querySelectorAll('[data-inspector-panel="overview"] dt')].find(item=>item.textContent===label);return dt?dt.nextElementSibling.textContent.trim():null;};}`)
	page.MustWait(`()=>document.querySelector('.infra-app').clientWidth>1000`)

	// One connection reads in the singular, none in the plural form.
	page.MustEval(`()=>document.querySelector('[data-node="fixture-node-0003"]').click()`)
	page.MustWait(`()=>document.querySelector('[data-inspector] h2')?.textContent==='Cold-store NAS'`)
	if !page.MustEval(`()=>fact(w('desktop.infra_tab_connections'))===w('desktop.infra_connections_summary_one')`).Bool() {
		t.Fatalf("one connection must use the singular text, got %q", page.MustEval(`()=>fact(w('desktop.infra_tab_connections'))`).Str())
	}
	page.MustEval(`()=>document.querySelector('[data-node="fixture-node-0010"]').click()`)
	page.MustWait(`()=>document.querySelector('[data-inspector] h2')?.textContent==='Admin laptop'`)
	if !page.MustEval(`()=>fact(w('desktop.infra_tab_connections'))===w('desktop.infra_connections_summary',{count:0})`).Bool() {
		t.Fatalf("no connection must use the plural text, got %q", page.MustEval(`()=>fact(w('desktop.infra_tab_connections'))`).Str())
	}

	// The Overview names the placement state without coordinates; the Details tab keeps them as read-only text.
	page.MustEval(`()=>document.querySelector('[data-node="fixture-node-0003"]').click()`)
	page.MustWait(`()=>document.querySelector('[data-inspector] h2')?.textContent==='Cold-store NAS'`)
	got := page.MustEval(`()=>JSON.stringify({
		onMap:fact(w('desktop.infra_placement'))===w('desktop.infra_on_map'),
		noCoordinates:!/\bx -?\d+, y -?\d+/.test(document.querySelector('[data-inspector-panel="overview"]').textContent),
		readout:/x -?\d+, y -?\d+/.test(document.querySelector('[data-placement-readout] dd').textContent),
	})`).Str()
	if want := `{"onMap":true,"noCoordinates":true,"readout":true}`; got != want {
		t.Fatalf("placed asset overview %s; want %s", got, want)
	}
	page.MustEval(`()=>document.querySelector('[data-library-item="dev-unplaced"] [data-library-select]').click()`)
	page.MustWait(`()=>document.querySelector('[data-inspector] h2')?.textContent==='Cold NAS'`)
	if !page.MustEval(`()=>fact(w('desktop.infra_placement'))===w('desktop.infra_unplaced')&&document.querySelector('[data-placement-readout] dd').textContent===w('desktop.infra_unplaced')`).Bool() {
		t.Fatal("an unplaced asset must say so in the Overview and in the Details placement readout")
	}
	closeInfraBrowserWindow(t, page)
	requireInfraBrowserClean(t, page)
}
