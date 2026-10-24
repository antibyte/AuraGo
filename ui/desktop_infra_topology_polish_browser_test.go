package ui

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/input"
	"github.com/go-rod/rod/lib/proto"
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
	fixture.world(25) // ships saved topology placements with pinned:false on a 252x160 grid: they must not matter
	page := infraTopologyWorld(t, fixture, 1500, 970)
	defer page.Close()
	zoom := func() float64 { return page.MustEval(`()=>infraZoomOf()`).Num() }
	// A22: every card sits in a lane row (here one row per lane, centres at lane top + 96), none at its legacy pinned:false position.
	if got := page.MustEval(`()=>JSON.stringify([...new Set([...document.querySelectorAll('[data-nodes] .infra-card')].map(card=>parseFloat(card.style.top)))].sort((a,b)=>a-b))`).Str(); got != `[96,480,864,1248]` {
		t.Fatalf("the cards must sit in the lane rows of the automatic layout, not at the legacy placements: %s", got)
	}

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

// TestDesktopInfraTopologyCurvesBrowser covers amendment A19: a connection between two lanes is a cubic curve from the
// bottom edge of the upper card to the top edge of the lower card, connections that share a card edge take separate
// slots on it, and a connection inside a lane stays straight. The hit area and the selection highlight use the same
// path, and the curves follow a card that is being dragged. The Map view keeps straight lines.
//
// The cards sit at the automatic slots: inet -120,96  ont 120,96  switch -240,480  router 0,480  ap 240,480
// nas -120,864  pve 120,864  tv -120,1248  laptop 120,1248 (see TestDesktopInfraTopologyBrowser).
func TestDesktopInfraTopologyCurvesBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	fixture := infraTopologyFixture()
	page := infraPanelsPage(t, fixture)
	defer page.Close()
	page.MustEval(`()=>{
		window.infraPaths=()=>JSON.stringify(Object.fromEntries([...document.querySelectorAll('.infra-edge')].map(g=>[g.dataset.edge,[g.querySelector('.infra-edge-line').getAttribute('d'),g.querySelector('.infra-edge-hit').getAttribute('d')]])));
	}`)
	// The Map view keeps straight lines.
	if got := page.MustEval(`()=>[...document.querySelectorAll('.infra-edge-line')].filter(path=>/C/.test(path.getAttribute('d'))).length`).Int(); got != 0 {
		t.Fatalf("the Map view must draw straight connections; %d are curves", got)
	}
	page.MustEval(`()=>document.querySelector('[data-view="topology"]').click()`)
	page.MustWait(`()=>document.querySelectorAll('[data-lanes] [data-lane]').length===4&&!!document.querySelector('[data-node="tv"]')`)
	page.MustEval(`()=>document.querySelector('[data-action="fit"]').click()`)
	infraPolishSettle(page)

	// Hand computed: ont (120,96) to router (0,480): bottom centre 120,128 to top centre 0,448, offset 160; the switch (-240,480)
	// has three connections on its bottom edge in slots -43.2 (nas), 0 (tv, same x, later id) and 43.2 (pve); tv has two on its
	// top edge, -32.4 (switch) and 32.4 (nas); lane-internal ones are straight between the facing sides.
	want := map[string]string{
		"e1": "M-12 96L12 96",
		"e2": "M120 128C120 288 0 288 0 448",
		"e3": "M-108 480L-132 480",
		"e4": "M-132 480L132 480",
		"e5": "M-283.2 512C-283.2 672 -120 672 -120 832",
		"e6": "M-196.8 512C-196.8 672 120 672 120 832",
		"e7": "M240 512C240 864 120 864 120 1216",
		"e8": "M-240 512C-240 864 -152.4 864 -152.4 1216",
		"e9": "M-120 896C-120 1056 -87.6 1056 -87.6 1216",
	}
	paths := func() map[string][2]string {
		out := map[string][2]string{}
		if err := json.Unmarshal([]byte(page.MustEval(`()=>infraPaths()`).Str()), &out); err != nil {
			t.Fatal(err)
		}
		return out
	}
	got := paths()
	for id, d := range want {
		if got[id][0] != d {
			t.Errorf("connection %s is drawn as %q; want %q", id, got[id][0], d)
		}
		if got[id][1] != got[id][0] {
			t.Errorf("connection %s: the hit area %q must follow the visible line %q", id, got[id][1], got[id][0])
		}
	}
	if t.Failed() {
		t.FailNow()
	}

	// Clicking the curve selects it, and the highlighted line is that same path. e5 passes through free space at its midpoint.
	point := page.MustEval(`()=>{const hit=document.querySelector('[data-edge="e5"] .infra-edge-hit'),at=hit.getPointAtLength(hit.getTotalLength()/2),m=hit.getScreenCTM();return [m.a*at.x+m.c*at.y+m.e,m.b*at.x+m.d*at.y+m.f];}`).Arr()
	page.Mouse.MustMoveTo(point[0].Num(), point[1].Num()).MustClick(proto.InputMouseButtonLeft)
	page.MustWait(`()=>document.querySelector('[data-edge="e5"]').classList.contains('is-selected')`)
	if got := paths()["e5"]; got[0] != want["e5"] {
		t.Fatalf("the selected connection changed its path: %v", got)
	}

	// A card that is dragged takes its curves along; cancelling the drag puts them back.
	cx, cy := infraPanelsCenter(page, `[data-node="ap"]`)
	before := paths()["e7"][0]
	page.Mouse.MustMoveTo(cx, cy).MustDown(proto.InputMouseButtonLeft)
	page.Mouse.MustMoveTo(cx-30, cy+10)
	page.Mouse.MustMoveTo(cx-120, cy+40)
	page.MustWait(`(before)=>document.querySelector('[data-edge="e7"] .infra-edge-line').getAttribute('d')!==before`, before)
	if moved := paths()["e7"]; moved[0] == before || moved[1] != moved[0] || !strings.HasPrefix(moved[0], "M") || !strings.Contains(moved[0], "C") {
		t.Fatalf("while dragging, the connection of the dragged card must stay a curve that follows it: %v", moved)
	}
	page.Keyboard.MustType(input.Escape)
	page.Mouse.MustUp(proto.InputMouseButtonLeft)
	page.MustWait(`(before)=>document.querySelector('[data-edge="e7"] .infra-edge-line').getAttribute('d')===before`, before)
	closeInfraBrowserWindow(t, page)
	requireInfraBrowserClean(t, page)
}

// TestDesktopInfraTopologyArrangeBrowser covers where Arrange is available in Topology (A22): it hands cards back to the
// automatic layout, so it needs a card with a saved placement and a server that can unplace. Without unplace support
// the menu item and the inspector button are disabled with a reason; with nothing to hand back they are disabled
// without one.
func TestDesktopInfraTopologyArrangeBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	reason := infraGermanWord(t, "desktop.infra_arrange_unsupported")
	auto := infraGermanWord(t, "desktop.infra_arrange_auto")
	const arrangeState = `()=>{
		document.querySelector('[data-action="more"]').click();
		const item=document.querySelector('[data-menu="more"] [data-action="arrange"]');
		const state={disabled:item.disabled,title:item.getAttribute('title')||'',label:item.textContent.trim()};
		document.querySelector('[data-action="more"]').click();
		return JSON.stringify(state);
	}`
	const selectionState = `()=>{
		document.querySelector('[data-node="inet"]').click();
		document.querySelector('[data-node="ont"]').dispatchEvent(new MouseEvent('click',{bubbles:true,shiftKey:true}));
		return new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(()=>{
			const button=document.querySelector('[data-inspector] [data-action="arrange-selection"]');
			resolve(JSON.stringify(button?{disabled:button.disabled,title:button.getAttribute('title')||'',label:button.textContent.trim()}:null));
		})));
	}`
	for _, server := range []struct {
		name        string
		noUnplace   bool
		title, want string
	}{{"a server without unplace", true, reason, "reason"}, {"a server that can unplace but has nothing to hand back", false, "", "nothing"}} {
		fixture := infraTopologyFixture()
		fixture.noSourceHosts = server.noUnplace
		page := infraPanelsPage(t, fixture)
		page.MustEval(`()=>document.querySelector('[data-view="topology"]').click()`)
		page.MustWait(`()=>document.querySelectorAll('[data-lanes] [data-lane]').length===4&&!!document.querySelector('[data-node="tv"]')`)
		want := fmt.Sprintf(`{"disabled":true,"title":%q,"label":%q}`, server.title, auto)
		if got := page.MustEval(arrangeState).Str(); got != want {
			t.Fatalf("%s: the Arrange item is %s; want %s", server.name, got, want)
		}
		if got := page.MustEval(selectionState).Str(); got != want {
			t.Fatalf("%s: the inspector's Arrange button is %s; want %s", server.name, got, want)
		}
		page.Close()
	}

	// With a server that can unplace and a card that was placed by hand, Arrange is available and the title is gone.
	fixture := infraTopologyFixture()
	fixture.nodes[1]["placements"].(map[string]any)["topology"] = infraPanelsAt(120, 120)
	page := infraPanelsPage(t, fixture)
	defer page.Close()
	page.MustEval(`()=>document.querySelector('[data-view="topology"]').click()`)
	page.MustWait(`()=>document.querySelectorAll('[data-lanes] [data-lane]').length===4&&!!document.querySelector('[data-node="tv"]')`)
	if got, want := page.MustEval(arrangeState).Str(), fmt.Sprintf(`{"disabled":false,"title":"","label":%q}`, auto); got != want {
		t.Fatalf("with a placed card the Arrange item is %s; want %s", got, want)
	}
	if got, want := page.MustEval(selectionState).Str(), fmt.Sprintf(`{"disabled":false,"title":"","label":%q}`, auto); got != want {
		t.Fatalf("with a placed card in the selection the inspector's Arrange button is %s; want %s", got, want)
	}
	requireInfraBrowserClean(t, page)
}

// TestDesktopInfraTopologyMotionBrowser covers the motion rule of A22: a card that Topology places automatically
// glides to its new slot, but only while the desktop animations are on and reduced motion is not asked for.
func TestDesktopInfraTopologyMotionBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	fixture := infraTopologyFixture()
	fixture.nodes[1]["placements"].(map[string]any)["topology"] = infraPanelsAt(120, 120)
	page := infraPanelsPage(t, fixture)
	defer page.Close()
	page.MustEval(`()=>document.querySelector('[data-view="topology"]').click()`)
	page.MustWait(`()=>document.querySelectorAll('[data-lanes] [data-lane]').length===4&&!!document.querySelector('[data-node="inet"]')`)
	saves := func() int {
		fixture.mu.Lock()
		defer fixture.mu.Unlock()
		return len(fixture.layoutCalls)
	}
	waitSaves := func(want int) {
		t.Helper()
		deadline := time.Now().Add(8 * time.Second)
		for saves() < want {
			if time.Now().After(deadline) {
				t.Fatalf("waited for %d layout saves, got %d", want, saves())
			}
			time.Sleep(50 * time.Millisecond)
		}
		time.Sleep(450 * time.Millisecond) // the glide class is gone after 400 ms
	}
	// inet starts as a card placed by hand at 120,120; each Arrange hands it back to its automatic slot, each undo places it again.
	const arrangeAndLook = `()=>new Promise(resolve=>{
		document.querySelector('[data-action="more"]').click();
		document.querySelector('[data-menu="more"] [data-action="arrange"]').click();
		requestAnimationFrame(()=>requestAnimationFrame(()=>{
			const card=document.querySelector('[data-node="inet"]');
			resolve(JSON.stringify({glide:card.classList.contains('is-gliding'),transition:getComputedStyle(card).transitionProperty.includes('left')}));
		}));
	})`
	n := 0
	for _, motion := range []struct {
		preference, animations string
		glide                  bool
	}{{"reduce", "true", false}, {"no-preference", "false", false}, {"no-preference", "true", true}} {
		if err := (proto.EmulationSetEmulatedMedia{Features: []*proto.EmulationMediaFeature{{Name: "prefers-reduced-motion", Value: motion.preference}}}).Call(page); err != nil {
			t.Fatal(err)
		}
		page.MustEval(`(value)=>{document.body.dataset.animations=value;}`, motion.animations)
		page.MustWait(`()=>{const item=document.querySelector('[data-action="more"]');return !!item&&!item.disabled;}`)
		page.MustWait(`()=>{document.querySelector('[data-action="more"]').click();const item=document.querySelector('[data-menu="more"] [data-action="arrange"]'),ok=!item.disabled;document.querySelector('[data-action="more"]').click();return ok;}`)
		want := fmt.Sprintf(`{"glide":%v,"transition":%v}`, motion.glide, motion.glide)
		if got := page.MustEval(arrangeAndLook).Str(); got != want {
			t.Fatalf("with prefers-reduced-motion %s and animations %s an automatic card gave %s; want %s", motion.preference, motion.animations, got, want)
		}
		n++
		waitSaves(n)
		page.MustEval(`()=>document.querySelector('[data-action="undo"]').click()`)
		n++
		waitSaves(n)
	}
	requireInfraBrowserClean(t, page)
}
