package ui

import (
	"testing"

	"github.com/go-rod/rod"
)

// Design QA batch D-fix-2 (amendments A17 to A24 for the Topology view). Each scenario is its own test function.

// infraTopologyWorld opens the 25-asset fixture in Topology in a window of the given size, fitted to the cards. The
// fixture ships saved topology placements with pinned:false, the leftovers of the retired layered Arrange; Topology
// ignores them and draws the 22 placeable assets at their automatic slots (A22).
func infraTopologyWorld(t *testing.T, fixture *infraAppBrowserFixture, width, height int) *rod.Page {
	t.Helper()
	page := infraPolishOpen(t, fixture, width, height)
	page.MustEval(`()=>{
		window.infraZoomOf=()=>Number(/scale\(([^)]+)\)/.exec(document.querySelector('[data-scene]').style.transform)[1]);
		document.querySelector('[data-view="topology"]').click();
	}`)
	page.MustWait(`()=>document.querySelectorAll('[data-nodes] [data-node]').length===22&&document.querySelectorAll('[data-lanes] [data-lane]').length===4`)
	infraPolishResize(page, width, height)
	return page
}

// TestDesktopInfraTopologyLanesBrowser covers amendments A17 and A18. Lanes are 384 px tall with two auto rows, so fit
// is large and no compact name touches another card; the lanes are swimlane stripes far wider than the content that
// do not take part in fit, and their labels stay at the left edge of the viewport while panning and zooming.
func TestDesktopInfraTopologyLanesBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	fixture := newInfraAppBrowserFixture()
	fixture.world(25)
	for _, node := range fixture.nodes { // removed with A22: the first scenarios run on a fixture without the legacy placements
		if placements, ok := node["placements"].(map[string]any); ok {
			delete(placements, "topology")
		}
	}
	page := infraTopologyWorld(t, fixture, 1500, 970)
	defer page.Close()
	zoom := func() float64 { return page.MustEval(`()=>infraZoomOf()`).Num() }

	// A17: fit at 1500x970 shows the lanes at 45 % or more.
	if z := zoom(); z < 0.45 {
		t.Fatalf("fit at 1500x970 is %.0f%%; want at least 45%%", z*100)
	} else {
		t.Logf("fit at 1500x970: %.0f%%", z*100)
	}
	// A18: four stripes, far wider than the cards, behind everything; fit hugs the cards (48 px of padding on the
	// limiting axis), so the stripes are not part of the scene bounds.
	if got := page.MustEval(`()=>{
		const vp=document.querySelector('[data-stage]').getBoundingClientRect(),cards=[...document.querySelectorAll('[data-nodes] .infra-card')].map(el=>el.getBoundingClientRect());
		const gap=Math.min(Math.min(...cards.map(r=>r.left))-vp.left,Math.min(...cards.map(r=>r.top))-vp.top,vp.right-Math.max(...cards.map(r=>r.right)),vp.bottom-Math.max(...cards.map(r=>r.bottom)));
		const stripes=[...document.querySelectorAll('[data-lanes] [data-lane]')];
		return JSON.stringify({gap:Math.round(gap/4),widths:stripes.map(el=>el.offsetWidth>=100000),heights:stripes.map(el=>el.offsetHeight),events:stripes.map(el=>getComputedStyle(el).pointerEvents),minimap:document.querySelector('[data-minimap]').hidden});
	}`).Str(); got != `{"gap":12,"widths":[true,true,true,true],"heights":[384,384,384,384],"events":["none","none","none","none"],"minimap":true}` {
		t.Fatalf("the stripes must be 100000x384 px, take no pointer events and stay out of fit and the minimap (gap/4 = 12 means 48 px): %s", got)
	}

	// A18: at 100 % every lane label in view is visible and sits 12 px from the left edge, also after panning.
	page.MustEval(`()=>document.querySelector('[data-action="zoom-reset"]').click()`)
	page.MustWait(`()=>infraZoomOf()===1`)
	infraPolishSettle(page)
	const labelsInView = `()=>{
		const vp=document.querySelector('[data-stage]').getBoundingClientRect(),rows=[];
		for(const lane of document.querySelectorAll('[data-lanes] [data-lane]')){
			const band=lane.getBoundingClientRect(),label=lane.querySelector('.infra-lane-label').getBoundingClientRect();
			if(band.bottom<=vp.top||band.top>=vp.bottom)continue;
			const labelInView=label.bottom>vp.top&&label.top<vp.bottom;
			rows.push({lane:lane.dataset.lane,left:Math.round(label.left-vp.left),inside:label.left>=vp.left&&label.right<=vp.right,labelInView});
		}
		return JSON.stringify(rows.filter(row=>row.labelInView));
	}`
	check := func(step string) {
		t.Helper()
		got := page.MustEval(labelsInView).Str()
		if !page.MustEval(`(raw)=>{const rows=JSON.parse(raw);return rows.length>=2&&rows.every(row=>row.inside&&row.left>=11&&row.left<=13);}`, got).Bool() {
			t.Fatalf("%s: every lane label in view must be whole and 12 px from the left edge: %s", step, got)
		}
	}
	check("100 %")
	page.MustEval(`()=>document.querySelector('[data-stage]').dispatchEvent(new WheelEvent('wheel',{deltaX:700,deltaY:260,bubbles:true,cancelable:true}))`)
	infraPolishSettle(page)
	check("panned right and down")
	page.MustEval(`()=>document.querySelector('[data-stage]').dispatchEvent(new WheelEvent('wheel',{deltaX:-2400,deltaY:0,bubbles:true,cancelable:true}))`)
	infraPolishSettle(page)
	check("panned far left")

	// A17: at 1000x700 fit no compact name touches another card or another name; A18: at 430x700 no label is cut.
	infraPolishResize(page, 1000, 700)
	if got := page.MustEval(`()=>{
		const boxes=[...document.querySelectorAll('[data-nodes] .infra-card')].map(el=>({id:el.dataset.node,r:el.getBoundingClientRect()}));
		const names=[...document.querySelectorAll('.infra-card-name')].map(el=>({id:el.closest('.infra-card').dataset.node,r:el.getBoundingClientRect()}));
		const hit=(a,b)=>a.left<b.right-1&&a.right>b.left+1&&a.top<b.bottom-1&&a.bottom>b.top+1;
		const bad=[];
		for(const n of names){for(const b of boxes)if(b.id!==n.id&&hit(n.r,b.r))bad.push(n.id+'>'+b.id);for(const m of names)if(m.id<n.id&&hit(n.r,m.r))bad.push(n.id+'~'+m.id);}
		return JSON.stringify({compact:document.querySelector('[data-stage]').classList.contains('is-compact'),cards:boxes.length,bad});
	}`).Str(); got != `{"compact":true,"cards":22,"bad":[]}` {
		t.Fatalf("at 1000x700 fit no compact name may touch another card box or name: %s", got)
	}
	infraPolishResize(page, 430, 700)
	page.MustEval(`()=>document.querySelector('[data-action="close-inspector"]')?.click()`)
	infraPolishSettle(page)
	if got := page.MustEval(`()=>{
		const vp=document.querySelector('[data-stage]').getBoundingClientRect();
		return JSON.stringify([...document.querySelectorAll('[data-lanes] .infra-lane-label')].map(el=>{const r=el.getBoundingClientRect();return r.left>=vp.left&&r.right<=vp.right&&r.top>=vp.top&&r.bottom<=vp.bottom;}));
	}`).Str(); got != `[true,true,true,true]` {
		t.Fatalf("at 430x700 fit no lane label may be cut: %s", got)
	}
	requireInfraBrowserClean(t, page)
}
