package ui

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/input"
	"github.com/go-rod/rod/lib/proto"
)

// Design QA batch D-fix-1 (amendments A1, A3, A5, A8, A13). Every scenario is its own test function so the page
// time of TestDesktopInfraAppBrowser stays untouched.

// infraPolishOpen opens Infra on a 25-asset fixture in a window of the given size, fitted to the map.
func infraPolishOpen(t *testing.T, fixture *infraAppBrowserFixture, width, height int) *rod.Page {
	t.Helper()
	page := infraBrowserPageWithFixture(t, fixture)
	page.MustSetViewport(1920, 1200, 1, false)
	openInfraBrowserApp(page)
	page.MustWait(`()=>document.querySelectorAll('[data-nodes] [data-node]').length>0`)
	infraPolishResize(page, width, height)
	return page
}

// infraPolishResize sets the Infra window size and fits the map once the layout has settled.
func infraPolishResize(page *rod.Page, width, height int) {
	page.MustEval(`(w,h)=>{const win=document.querySelector('.vd-window[data-app-id="infra"]');win.classList.remove('maximized','vd-mobile-forced-maximized');win.style.right='auto';win.style.minWidth='360px';win.style.width=w+'px';win.style.height=h+'px';win.style.left='210px';win.style.top='70px';}`, width, height)
	page.MustWait(`(w)=>Math.abs(document.querySelector('.infra-app').clientWidth-w)<40`, width)
	page.MustEval(`()=>document.querySelector('[data-action="fit"]').click()`)
	infraPolishSettle(page)
}

// infraPolishSettle waits for pans, form switches and layout to finish.
func infraPolishSettle(page *rod.Page) {
	page.MustWait(`()=>!document.querySelector('[data-scene]').classList.contains('is-animating')&&!document.querySelector('[data-stage]').classList.contains('is-form-switching')`)
	page.MustEval(`()=>new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve)))`)
}

// TestDesktopInfraSemanticZoomBrowser covers amendment A1: below zoom 0.65 the cards draw in a compact form
// (no meta line, the geometry unchanged), names and region headers are counter-scaled to at least 12 and 13 px
// on screen, connection handles go below 0.5, and above 1.0 everything scales with the scene.
func TestDesktopInfraSemanticZoomBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	fixture := newInfraAppBrowserFixture()
	fixture.world(25)
	// Names of 20 characters (A2) must fit at 100 %.
	for _, node := range fixture.nodes {
		switch node["id"] {
		case "fixture-node-0010":
			node["name"] = "Camera recorder rack"
		case "fixture-node-0011":
			node["name"] = "Living room speakers"
		}
	}
	page := infraPolishOpen(t, fixture, 1500, 970)
	defer page.Close()
	page.MustEval(`()=>{
		window.infraZoomOf=()=>Number(/scale\(([^)]+)\)/.exec(document.querySelector('[data-scene]').style.transform)[1]);
		// The on-screen size of a label: its font size times its own counter-scale times the scene zoom.
		window.infraOnScreen=el=>parseFloat(getComputedStyle(el).fontSize)*new DOMMatrix(getComputedStyle(el).transform).a*infraZoomOf();
		// A24: the on-screen width of a compact card's icon image.
		window.infraIconPx=card=>card.querySelector('.infra-card-icon img').getBoundingClientRect().width;
	}`)

	page.MustEval(`()=>document.querySelector('[data-action="zoom-reset"]').click()`)
	page.MustWait(`()=>infraZoomOf()===1`)
	infraPolishSettle(page)
	if got := page.MustEval(`()=>{
		const stage=document.querySelector('[data-stage]'),card=document.querySelector('[data-node="fixture-node-0002"]'),name=card.querySelector('.infra-card-name');
		return JSON.stringify({compact:stage.classList.contains('is-compact'),name:Math.round(infraOnScreen(name)*10)/10,meta:getComputedStyle(card.querySelector('.infra-card-meta')).display,box:[card.offsetWidth,card.offsetHeight],truncated:[...document.querySelectorAll('.infra-card-name')].filter(el=>el.scrollWidth>el.clientWidth).map(el=>el.textContent+':'+el.scrollWidth+'/'+el.clientWidth)});
	}`).Str(); got != `{"compact":false,"name":16,"meta":"block","box":[216,64],"truncated":[]}` {
		t.Fatalf("at 100%% the card must be the full 216x64 form with 16 px names and no truncated name: %s", got)
	}

	// Zoom out to about 0.28: compact form, names still at least 12 px on screen, headers at least 13 px.
	for i := 0; i < 7; i++ {
		page.MustEval(`()=>document.querySelector('[data-action="zoom-out"]').click()`)
	}
	page.MustWait(`()=>infraZoomOf()<0.3`)
	infraPolishSettle(page)
	if got := page.MustEval(`()=>{
		const stage=document.querySelector('[data-stage]'),card=document.querySelector('[data-node="fixture-node-0002"]'),names=[...document.querySelectorAll('.infra-card-name')],header=document.querySelector('.infra-region-header');
		const rects=names.map(el=>el.getBoundingClientRect());
		let overlaps=0;
		for(let i=0;i<rects.length;i++)for(let j=i+1;j<rects.length;j++){const a=rects[i],b=rects[j];if(a.left<b.right-1&&a.right>b.left+1&&a.top<b.bottom-1&&a.bottom>b.top+1)overlaps++;}
		return JSON.stringify({compact:stage.classList.contains('is-compact'),tiny:stage.classList.contains('is-tiny'),handles:getComputedStyle(card.querySelector('.infra-card-handle')).display,
			name:Math.min(...names.map(infraOnScreen))>=12,header:infraOnScreen(header)>=13,meta:getComputedStyle(card.querySelector('.infra-card-meta')).display,box:[card.offsetWidth,card.offsetHeight],overlaps,icon:infraIconPx(card)>=0.75*card.getBoundingClientRect().height-0.5});
	}`).Str(); got != `{"compact":true,"tiny":true,"handles":"none","name":true,"header":true,"meta":"none","box":[216,64],"overlaps":0,"icon":true}` {
		t.Fatalf("at about 28%% the cards must be compact with legible, non-overlapping labels and the same geometry: %s", got)
	}
	// Zoom back in: 0.58 is still compact (below 0.65), the full form returns from 0.68 on.
	page.MustEval(`()=>{for(let i=0;i<3;i++)document.querySelector('[data-action="zoom-in"]').click();}`)
	page.MustWait(`()=>infraZoomOf()>0.45`)
	infraPolishSettle(page)
	if got := page.MustEval(`()=>{
		const stage=document.querySelector('[data-stage]'),card=document.querySelector('[data-node="fixture-node-0002"]');
		return JSON.stringify({compact:stage.classList.contains('is-compact'),tiny:stage.classList.contains('is-tiny'),handles:getComputedStyle(card.querySelector('.infra-card-handle')).display!=='none',lines:getComputedStyle(card.querySelector('.infra-card-name')).webkitLineClamp,icon:infraIconPx(card)>=18,ring:infraIconPx(card)<=21});
	}`).Str(); got != `{"compact":true,"tiny":false,"handles":false,"lines":"2","icon":true,"ring":true}` {
		// 0.28 * 1.2^3 = 0.48: below 0.5 the handles are still hidden.
		t.Fatalf("at about 48%% the compact form shows two-line names, still no handles and an icon of 18 to 21 px: %s", got)
	}
	page.MustEval(`()=>{for(let i=0;i<3;i++)document.querySelector('[data-action="zoom-in"]').click();}`)
	page.MustWait(`()=>infraZoomOf()>0.8`)
	infraPolishSettle(page)
	if got := page.MustEval(`()=>{
		const stage=document.querySelector('[data-stage]'),card=document.querySelector('[data-node="fixture-node-0002"]');
		return JSON.stringify({compact:stage.classList.contains('is-compact'),lowLabels:stage.classList.contains('is-zoom-low'),meta:getComputedStyle(card.querySelector('.infra-card-meta')).display});
	}`).Str(); got != `{"compact":false,"lowLabels":false,"meta":"block"}` {
		t.Fatalf("above 0.75 the full card and the edge labels return: %s", got)
	}
	// Above 100 % names scale with the scene.
	page.MustEval(`()=>{for(let i=0;i<3;i++)document.querySelector('[data-action="zoom-in"]').click();}`)
	page.MustWait(`()=>infraZoomOf()>1.3`)
	if !page.MustEval(`()=>{const name=document.querySelector('.infra-card-name');return new DOMMatrix(getComputedStyle(name).transform).a===1&&infraOnScreen(name)>20;}`).Bool() {
		t.Fatal("above 100% a name must scale with the scene instead of being counter-scaled")
	}
	// Crossing 0.65 fades the card content in, but only while the desktop animations are on and the user has not asked
	// for reduced motion. Both settings are set here, because the host's own motion setting flips headless Chrome's
	// preference and the desktop's animations switch.
	for _, motion := range []struct {
		preference, animations string
		fade                   bool
	}{{"reduce", "true", false}, {"no-preference", "false", false}, {"no-preference", "true", true}} {
		if err := (proto.EmulationSetEmulatedMedia{Features: []*proto.EmulationMediaFeature{{Name: "prefers-reduced-motion", Value: motion.preference}}}).Call(page); err != nil {
			t.Fatal(err)
		}
		page.MustEval(`(value)=>{document.body.dataset.animations=value;}`, motion.animations)
		page.MustEval(`()=>document.querySelector('[data-action="zoom-reset"]').click()`)
		page.MustWait(`()=>infraZoomOf()===1&&!document.querySelector('[data-stage]').classList.contains('is-compact')`)
		infraPolishSettle(page)
		want := fmt.Sprintf(`{"compact":true,"fading":%v}`, motion.fade)
		if got := page.MustEval(`()=>new Promise(resolve=>{
			for(let i=0;i<3;i++)document.querySelector('[data-action="zoom-out"]').click();
			requestAnimationFrame(()=>requestAnimationFrame(()=>{
				const stage=document.querySelector('[data-stage]');
				resolve(JSON.stringify({compact:stage.classList.contains('is-compact'),fading:stage.classList.contains('is-form-switching')}));
			}));
		})`).Str(); got != want {
			t.Fatalf("with prefers-reduced-motion %s and animations %s the form switch gave %s; want %s", motion.preference, motion.animations, got, want)
		}
	}
	requireInfraBrowserClean(t, page)
}

// TestDesktopInfraMinimapBrowser covers amendment A3: a 168x112 overview with no text inside, filled with the theme
// panel colour (the tools surface) and blur, that hides itself while the whole scene fits and never covers a card at fit. The shown
// count lives in the toggle's tooltip and the cluster's description, and the toggle works in every state.
func TestDesktopInfraMinimapBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	fixture := newInfraAppBrowserFixture()
	fixture.world(25)
	page := infraPolishOpen(t, fixture, 1000, 700)
	defer page.Close()
	page.MustEval(`()=>{
		window.infraZoomOf=()=>Number(/scale\(([^)]+)\)/.exec(document.querySelector('[data-scene]').style.transform)[1]);
		window.infraMinimapState=()=>{const map=document.querySelector('[data-minimap]'),toggle=document.querySelector('[data-action="toggle-minimap"]');return JSON.stringify({shown:!map.hidden&&map.offsetParent!==null,pressed:toggle.getAttribute('aria-pressed')});};
	}`)

	// At fit everything is in view: the overview is hidden, so it cannot cover a card.
	if got := page.MustEval(`()=>infraMinimapState()`).Str(); got != `{"shown":false,"pressed":"true"}` {
		t.Fatalf("the minimap must hide itself while the whole scene fits: %s", got)
	}
	// The count is in the tooltip and the description, never as text on the map.
	if got := page.MustEval(`()=>{const toggle=document.querySelector('[data-action="toggle-minimap"]'),tools=document.querySelector('[data-tools]'),words=window.fixtureWords;
		const shown=words['desktop.infra_visible_count'].replace('{count}','23');
		return JSON.stringify({title:toggle.title===words['desktop.infra_minimap']+' · '+shown,description:tools.getAttribute('aria-description')===shown,inCluster:!!toggle.closest('[data-tools]'),legacyCount:!!document.querySelector('[data-visible-count]'),text:document.querySelector('[data-minimap]').textContent.trim()});}`).Str(); got != `{"title":true,"description":true,"inCluster":true,"legacyCount":false,"text":""}` {
		t.Fatalf("the shown count must move into the toggle tooltip and the cluster description: %s", got)
	}

	// Zoomed in the scene overflows: the overview appears at 168x112 on the strong panel surface (the tools surface, not the modal scrim) with the theme blur.
	page.MustEval(`()=>document.querySelector('[data-action="zoom-reset"]').click()`)
	page.MustWait(`()=>infraZoomOf()===1&&!document.querySelector('[data-minimap]').hidden`)
	if got := page.MustEval(`()=>{
		const map=document.querySelector('[data-minimap]'),r=map.getBoundingClientRect(),cs=getComputedStyle(map);
		const probe=document.createElement('i');probe.style.cssText='background:var(--vd-theme-panel-bg-strong);backdrop-filter:var(--vd-theme-blur, blur(12px))';document.body.appendChild(probe);
		const want=getComputedStyle(probe);const result={size:[Math.round(r.width),Math.round(r.height)],background:cs.backgroundColor===want.backgroundColor,blur:cs.backdropFilter===want.backdropFilter,text:map.textContent.trim()};probe.remove();
		return JSON.stringify(result);
	}`).Str(); got != `{"size":[168,112],"background":true,"blur":true,"text":""}` {
		t.Fatalf("the minimap must be 168x112, filled with the strong panel colour and the blur and hold no text: %s", got)
	}

	// The toggle switches it off even while the scene overflows, and on again.
	page.MustEval(`()=>document.querySelector('[data-action="toggle-minimap"]').click()`)
	page.MustWait(`()=>document.querySelector('[data-action="toggle-minimap"]').getAttribute('aria-pressed')==='false'&&document.querySelector('[data-minimap]').hidden`)
	page.MustEval(`()=>document.querySelector('[data-action="toggle-minimap"]').click()`)
	page.MustWait(`()=>document.querySelector('[data-action="toggle-minimap"]').getAttribute('aria-pressed')==='true'&&!document.querySelector('[data-minimap]').hidden`)

	// Fitting hides it again; the toggle still reads "on".
	page.MustEval(`()=>document.querySelector('[data-action="fit"]').click()`)
	infraPolishSettle(page)
	if got := page.MustEval(`()=>infraMinimapState()`).Str(); got != `{"shown":false,"pressed":"true"}` {
		t.Fatalf("fitting the map must hide the minimap again: %s", got)
	}
	// Whenever it is shown it never sits on a card at fit: at fit it is hidden, at 100 % it is the user's own pan.
	requireInfraBrowserClean(t, page)
}

// TestDesktopInfraEdgeLabelsBrowser covers amendment A5: a connection label sits at the midpoint of the visible
// segment; when its box would touch a card it stays hidden until its connection is hovered, and no visible
// label ever overlaps a card.
func TestDesktopInfraEdgeLabelsBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	fixture := newInfraAppBrowserFixture()
	fixture.world(25)
	page := infraPolishOpen(t, fixture, 1500, 970)
	defer page.Close()
	page.MustEval(`()=>document.querySelector('[data-action="zoom-reset"]').click()`)
	page.MustWait(`()=>Number(/scale\(([^)]+)\)/.exec(document.querySelector('[data-scene]').style.transform)[1])===1`)
	infraPolishSettle(page)
	// The LAN label would sit under the card between its ends, the Wi-Fi label does not fit the 36 px gap between
	// neighbours, the fibre label has a clear stretch of line.
	got := page.MustEval(`()=>{
		const rect=el=>el.getBoundingClientRect(),cards=[...document.querySelectorAll('[data-nodes] [data-node]')].map(rect);
		const rows=[...document.querySelectorAll('.infra-edge')].filter(edge=>edge.querySelector('.infra-edge-label')).map(edge=>{
			const label=edge.querySelector('.infra-edge-label'),box=rect(label),visible=getComputedStyle(label).visibility==='visible';
			return {id:edge.dataset.edge,clash:edge.classList.contains('is-label-clash'),visible,overlap:visible&&cards.some(card=>box.left<card.right&&box.right>card.left&&box.top<card.bottom&&box.bottom>card.top)};
		}).sort((a,b)=>a.id.localeCompare(b.id));
		return JSON.stringify(rows);
	}`).Str()
	if want := `[{"id":"fixture-edge-fiber","clash":false,"visible":true,"overlap":false},{"id":"fixture-edge-lan","clash":true,"visible":false,"overlap":false},{"id":"fixture-edge-wifi","clash":true,"visible":false,"overlap":false}]`; got != want {
		t.Fatalf("edge labels: got %s; want %s", got, want)
	}

	// Hovering the LAN connection shows its label.
	point := page.MustEval(`()=>{const r=document.querySelector('[data-edge="fixture-edge-lan"] .infra-edge-hit').getBoundingClientRect();return [r.left+r.width*0.12,r.top+r.height/2];}`).Arr()
	page.Mouse.MustMoveTo(point[0].Num(), point[1].Num())
	page.MustWait(`()=>getComputedStyle(document.querySelector('[data-edge="fixture-edge-lan"] .infra-edge-label')).visibility==='visible'`)
	page.Mouse.MustMoveTo(5, 5)
	page.MustWait(`()=>getComputedStyle(document.querySelector('[data-edge="fixture-edge-lan"] .infra-edge-label')).visibility==='hidden'`)

	// Dragging the card off the line brings the label back while the card is still held: the clash is re-evaluated on a move.
	centre := page.MustEval(`()=>{const r=document.querySelector('[data-node="fixture-node-0002"]').getBoundingClientRect();return [r.left+r.width/2,r.top+r.height/2];}`).Arr()
	cx, cy := centre[0].Num(), centre[1].Num()
	page.Mouse.MustMoveTo(cx, cy).MustDown(proto.InputMouseButtonLeft)
	page.Mouse.MustMoveTo(cx, cy+60)
	page.Mouse.MustMoveTo(cx, cy+200)
	page.MustWait(`()=>getComputedStyle(document.querySelector('[data-edge="fixture-edge-lan"] .infra-edge-label')).visibility==='visible'&&!document.querySelector('[data-edge="fixture-edge-lan"]').classList.contains('is-label-clash')`)
	page.Mouse.MustUp(proto.InputMouseButtonLeft)
	infraPolishSettle(page)
	page.Mouse.MustMoveTo(5, 5)
	if !page.MustEval(`()=>getComputedStyle(document.querySelector('[data-edge="fixture-edge-lan"] .infra-edge-label')).visibility==='visible'`).Bool() {
		t.Fatal("after dropping the card off the line the LAN label must stay visible")
	}
	requireInfraBrowserClean(t, page)
}

// infraBarProbe is the page-side helper of the command bar tests: it reports how the bar, the search and the
// filter chips share the width.
const infraBarProbe = `()=>{
	window.infraBar=()=>{
		const bar=document.querySelector('.infra-commandbar'),chips=document.querySelector('[data-filter-chips]'),search=document.querySelector('.infra-search'),more=chips.querySelector('[data-filter-overflow]');
		const box=chips.getBoundingClientRect(),all=[...chips.querySelectorAll('[data-remove-filter]')],shown=all.filter(chip=>!chip.hidden&&chip.offsetParent!==null);
		return {app:document.querySelector('.infra-app').clientWidth,overflow:bar.scrollWidth>bar.clientWidth+1,chipsOverflow:chips.scrollWidth>chips.clientWidth+1,search:Math.round(search.getBoundingClientRect().width),
			shown:shown.length,total:all.length,more:more&&!more.hidden&&more.offsetParent!==null?more.textContent:'',
			clipped:shown.filter(chip=>{const r=chip.getBoundingClientRect();return r.left<box.left-0.5||r.right>box.right+0.5;}).length};
	};
}`

// TestDesktopInfraFilterChipsBrowser covers amendment A13: with several active filters the command bar shows as many
// chips as fit and a "+N" chip for the rest, the search keeps at least 220 px above 760 px, and nothing overflows.
func TestDesktopInfraFilterChipsBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	fixture := newInfraAppBrowserFixture()
	fixture.world(25)
	page := infraPolishOpen(t, fixture, 1110, 800)
	defer page.Close()
	page.MustEval(infraBarProbe)
	page.MustEval(`()=>{const set=(selector,value)=>{const el=document.querySelector(selector);el.value=value;el.dispatchEvent(new Event('change',{bubbles:true}));};set('[data-kind-filter]','device');set('[data-source-filter]','docker');set('[data-power-filter]','running');}`)
	page.MustWait(`()=>document.querySelectorAll('[data-remove-filter]').length===3`)
	infraPolishSettle(page)
	state := func() string { return page.MustEval(`()=>JSON.stringify(infraBar())`).Str() }
	if !page.MustEval(`()=>{const b=infraBar();return b.app>=1100&&b.app<1200&&!b.overflow&&!b.chipsOverflow&&b.search>=220&&b.clipped===0&&b.shown<3&&b.shown+Number(b.more.slice(1))===3;}`).Bool() {
		t.Fatalf("at about 1110 px three filters must show whole chips plus a +N chip, with the search at 220 px or more and no overflow: %s", state())
	}
	if !page.MustEval(`()=>{const more=document.querySelector('[data-filter-overflow]'),words=window.fixtureWords,n=Number(more.textContent.slice(1));
		return more.getAttribute('aria-label')===(n===1?words['desktop.infra_filters_more_one']:words['desktop.infra_filters_more'].replace('{count}',n))&&more.title===more.getAttribute('aria-label');}`).Bool() {
		t.Fatal("the +N chip must say how many filters it hides")
	}
	// +N opens the filter popover.
	page.MustEval(`()=>document.querySelector('[data-filter-overflow]').click()`)
	page.MustWait(`()=>!document.querySelector('[data-popover="filters"]').hidden`)
	page.MustEval(`()=>document.querySelector('[data-filter-overflow]').click()`)
	page.MustWait(`()=>document.querySelector('[data-popover="filters"]').hidden`)

	// The search keeps 220 px and the bar never overflows, at every width from 760 px up.
	for _, width := range []int{760, 800, 860, 980, 1099, 1140, 1300, 1500} {
		infraPolishResize(page, width, 800)
		page.MustEval(`()=>new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve)))`)
		t.Logf("%d px window: %s", width, state())
		if !page.MustEval(`()=>{const b=infraBar();return !b.overflow&&!b.chipsOverflow&&b.search>=220&&b.clipped===0&&(b.app<1100||b.shown+(b.more?Number(b.more.slice(1)):0)===3);}`).Bool() {
			t.Fatalf("at a %d px window the bar overflows, clips a chip or squeezes the search below 220 px: %s", width, state())
		}
	}
	requireInfraBrowserClean(t, page)
}

// TestDesktopInfraSearchCollapseBrowser covers amendment A8: below 560 px the search is an icon button that opens
// the field over the command bar until the focus leaves it or Escape is pressed; Ctrl K opens it too, a query
// or filter that is still applied shows as a dot on the icon, and nothing overflows down to the 360 px minimum.
func TestDesktopInfraSearchCollapseBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	fixture := newInfraAppBrowserFixture()
	fixture.world(25)
	page := infraPolishOpen(t, fixture, 1500, 970)
	defer page.Close()
	page.MustEval(`()=>{
		window.infraSearch=()=>{
			const app=document.querySelector('.infra-app'),bar=document.querySelector('.infra-commandbar'),input=document.querySelector('[data-search]'),open=document.querySelector('[data-action="open-search"]'),group=document.querySelector('.infra-search');
			return {app:app.clientWidth,overflow:bar.scrollWidth>bar.clientWidth+1,field:input.offsetParent!==null,button:open.offsetParent!==null,opened:app.dataset.searchOpen,expanded:open.getAttribute('aria-expanded'),
				focus:document.activeElement===input?'input':document.activeElement===open?'button':'other',dot:!document.querySelector('[data-search-dot]').hidden,width:Math.round(group.getBoundingClientRect().width),
				covers:group.getBoundingClientRect().width>=bar.clientWidth-16};
		};
	}`)
	state := func() string { return page.MustEval(`()=>JSON.stringify(infraSearch())`).Str() }
	if !page.MustEval(`()=>{const s=infraSearch();return s.field&&!s.button&&!s.overflow;}`).Bool() {
		t.Fatalf("a wide window keeps the search field and no icon button: %s", state())
	}

	infraPolishResize(page, 430, 700)
	if !page.MustEval(`()=>{const s=infraSearch();return s.app<560&&!s.field&&s.button&&!s.overflow&&s.width<110&&s.opened==='false';}`).Bool() {
		t.Fatalf("at 430 px the search must be an icon button and nothing may overflow: %s", state())
	}
	page.MustElement(`[data-action="open-search"]`).MustClick()
	page.MustWait(`()=>{const s=infraSearch();return s.opened==='true'&&s.field&&s.focus==='input'&&s.covers&&!s.button&&s.expanded==='true';}`)
	if !page.MustEval(`()=>{const s=infraSearch(),bar=document.querySelector('.infra-commandbar').getBoundingClientRect(),group=document.querySelector('.infra-search').getBoundingClientRect(),filters=document.querySelector('[data-action="filters"]').getBoundingClientRect();
		return !s.overflow&&group.left>=bar.left&&group.right<=bar.right&&filters.right<=group.right&&filters.left>=group.left&&getComputedStyle(document.querySelector('.infra-view-switch')).visibility==='hidden';}`).Bool() {
		t.Fatalf("the open field must sit over the command bar, keep the filter button and overflow nothing: %s", state())
	}
	// The filters popover opens under the button at the right edge: it must stay inside the window and must not scroll the
	// overflow-hidden app sideways (the command bar stayed shifted before).
	page.MustElement(`[data-action="filters"]`).MustClick()
	page.MustWait(`()=>!document.querySelector('[data-popover="filters"]').hidden`)
	if got := page.MustEval(`()=>{const app=document.querySelector('.infra-app'),a=app.getBoundingClientRect(),r=document.querySelector('[data-popover="filters"]').getBoundingClientRect();return JSON.stringify({scrolled:app.scrollLeft!==0,inside:r.left>=a.left&&r.right<=a.right});}`).Str(); got != `{"scrolled":false,"inside":true}` {
		t.Fatalf("the filters popover must stay inside a 430 px window without scrolling the app: %s", got)
	}
	page.Keyboard.MustType(input.Escape)
	page.MustWait(`()=>document.querySelector('[data-popover="filters"]').hidden`)
	page.MustElement(`[data-search]`).MustInput("Cold-store")
	page.MustWait(`()=>document.querySelector('[data-search]').value==="Cold-store"&&document.querySelectorAll('[data-nodes] [data-node]:not(.is-dimmed)').length>0&&document.querySelectorAll('[data-nodes] .is-dimmed').length>0`)

	// Escape closes it, hands the focus to the icon and leaves a dot because the query is still applied.
	page.Keyboard.MustType(input.Escape)
	page.MustWait(`()=>{const s=infraSearch();return s.opened==='false'&&!s.field&&s.button&&s.focus==='button'&&s.dot&&s.expanded==='false';}`)
	if got := page.MustEval(`()=>document.querySelector('[data-search]').value`).Str(); got != "Cold-store" {
		t.Fatalf("Escape must keep the query that is applied; the field says %q", got)
	}

	// Ctrl K opens it from anywhere in the window; moving the focus away closes it.
	page.MustEval(`()=>document.querySelector('[data-stage]').focus()`)
	page.KeyActions().Press(input.ControlLeft).Type(input.KeyK).Release(input.ControlLeft).MustDo()
	page.MustWait(`()=>{const s=infraSearch();return s.opened==='true'&&s.focus==='input';}`)
	page.MustEval(`()=>document.querySelector('[data-stage]').focus()`)
	page.MustWait(`()=>infraSearch().opened==='false'`)

	// At 560 px and wider the field is back, and the bar still holds everything.
	for _, width := range []int{620, 360} {
		infraPolishResize(page, width, 700)
		want := width >= 560
		if !page.MustEval(`(want)=>{const s=infraSearch();return s.field===want&&s.button===!want&&!s.overflow;}`, want).Bool() {
			t.Fatalf("at %d px the search must be %v and the bar must not overflow: %s", width, map[bool]string{true: "a field", false: "an icon button"}[want], state())
		}
	}
	requireInfraBrowserClean(t, page)
}

// TestDesktopInfraEmptyStatesBrowser covers amendment A26: when a query or filters hide every asset, the action of
// the empty map matches the cause (search, filters or both), and the library sections say "No matches" instead of
// leaving a bare header with a zero.
func TestDesktopInfraEmptyStatesBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	fixture := newInfraAppBrowserFixture()
	fixture.world(25)
	page := infraPolishOpen(t, fixture, 1110, 800)
	defer page.Close()
	page.MustEval(`()=>{
		window.infraSet=(selector,value,type)=>{const el=document.querySelector(selector);el.value=value;el.dispatchEvent(new Event(type,{bubbles:true}));};
		window.infraEmpty=()=>{
			const box=document.querySelector('[data-no-match]'),button=box.querySelector('button'),words=window.fixtureWords;
			return JSON.stringify({shown:!box.hidden,action:button.dataset.action,label:button.textContent.trim(),
				query:document.querySelector('[data-search]').value,kind:document.querySelector('[data-kind-filter]').value});
		};
		window.infraWord=key=>window.fixtureWords[key];
	}`)
	empty := func() string { return page.MustEval(`()=>infraEmpty()`).Str() }
	word := func(key string) string { return page.MustEval(`(key)=>infraWord(key)`, key).Str() }
	waitShown := func(action string) {
		t.Helper()
		page.MustWait(`(action)=>{const box=document.querySelector('[data-no-match]');return !box.hidden&&box.querySelector('button').dataset.action===action;}`, action)
	}

	// Filters only: the action resets the filters, it does not offer to clear a search that is empty.
	page.MustEval(`()=>infraSet('[data-kind-filter]','network','change')`)
	waitShown("reset-filters")
	if got, want := empty(), fmt.Sprintf(`{"shown":true,"action":"reset-filters","label":%q,"query":"","kind":"network"}`, word("desktop.infra_clear_filters")); got != want {
		t.Fatalf("with only a filter the empty map shows %s; want %s", got, want)
	}
	// A30: the message has a surface of its own (the strong panel token with the minimap blur) above the connection layer, so the
	// connections of the dimmed cards never run through its text.
	if got := page.MustEval(`()=>{
		const box=getComputedStyle(document.querySelector('[data-no-match]')),map=getComputedStyle(document.querySelector('[data-minimap]')),scene=getComputedStyle(document.querySelector('[data-scene]'));
		const filled=box.backgroundColor!=='rgba(0, 0, 0, 0)'&&box.backgroundColor!=='transparent';
		return JSON.stringify({filled,sameSurface:box.backgroundColor===map.backgroundColor&&box.backdropFilter===map.backdropFilter,above:Number(box.zIndex)>=1&&!(Number(scene.zIndex)>=1)});
	}`).Str(); got != `{"filled":true,"sameSurface":true,"above":true}` {
		t.Fatalf("the empty-result message must sit on the strong panel surface above the connections: %s", got)
	}
	// The library sections say so too: one "No matches" line instead of a bare header with a zero.
	if got, want := page.MustEval(`()=>JSON.stringify(['to-place','on-map'].map(id=>{const line=document.querySelector('[data-section-empty="'+id+'"]');return [line.hidden,line.textContent.trim()];}))`).Str(), fmt.Sprintf(`[[false,%q],[false,%q]]`, word("desktop.infra_no_matches"), word("desktop.infra_no_matches")); got != want {
		t.Fatalf("with a filter that hides every asset the library sections show %s; want %s", got, want)
	}
	page.MustEval(`()=>document.querySelector('[data-no-match] button').click()`)
	page.MustWait(`()=>document.querySelector('[data-no-match]').hidden&&document.querySelector('[data-kind-filter]').value===''`)

	// A query only: the action clears the search.
	page.MustEval(`()=>infraSet('[data-search]','zzzz-no-such-asset','input')`)
	waitShown("clear-search")
	if got, want := empty(), fmt.Sprintf(`{"shown":true,"action":"clear-search","label":%q,"query":"zzzz-no-such-asset","kind":""}`, word("desktop.infra_clear_search")); got != want {
		t.Fatalf("with only a query the empty map shows %s; want %s", got, want)
	}
	// Under the query every library section is empty and says "No matches".
	page.MustWait(`()=>['to-place','on-map'].every(id=>{const line=document.querySelector('[data-section-empty="'+id+'"]');return !line.hidden&&line.textContent.trim()===infraWord('desktop.infra_no_matches');})`)
	// Both: one action clears both.
	page.MustEval(`()=>infraSet('[data-kind-filter]','network','change')`)
	waitShown("clear-search-filters")
	if got, want := empty(), fmt.Sprintf(`{"shown":true,"action":"clear-search-filters","label":%q,"query":"zzzz-no-such-asset","kind":"network"}`, word("desktop.infra_clear_search_filters")); got != want {
		t.Fatalf("with a query and a filter the empty map shows %s; want %s", got, want)
	}
	page.MustEval(`()=>document.querySelector('[data-no-match] button').click()`)
	page.MustWait(`()=>document.querySelector('[data-no-match]').hidden&&document.querySelector('[data-kind-filter]').value===''&&document.querySelector('[data-search]').value===''`)
	// Without a query or filter the sections keep their own empty texts (everything is placed here).
	if got := page.MustEval(`()=>JSON.stringify(['to-place','on-map'].map(id=>document.querySelector('[data-section-empty="'+id+'"]').hidden))`).Str(); got != `[false,true]` {
		t.Fatalf("with nothing filtered the To place section is empty and On the map is not: %s", got)
	}
	if got, want := page.MustEval(`()=>document.querySelector('[data-section-empty="to-place"]').textContent.trim()`).Str(), word("desktop.infra_everything_placed"); got != want {
		t.Fatalf("the To place section says %q; want %q", got, want)
	}
	requireInfraBrowserClean(t, page)
}

// TestDesktopInfraUnplacedSearchMatchBrowser covers a query whose matches are all missing from the Map (they wait in the
// library's To place section): the canvas says so with the count instead of "no asset matches", Show match and Enter in
// the search open the first match (by name) in the inspector without placing it, and a query that matches nothing keeps
// the plain "no match" box. A match on the map still wins: Enter then selects and focuses its card.
func TestDesktopInfraUnplacedSearchMatchBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	fixture := newInfraAppBrowserFixture()
	fixture.world(25)
	hiddenGuest := infraCanvasFixtureNode("brain-vm", "vm", "Hidden guest vm", "vm", nil, "")
	hiddenGuest["host_id"] = "brain-a" // rides in the unplaced host "TheBrain": it has no library row of its own
	staleBox := infraCanvasFixtureNode("stale-box", "container", "Stale box", "container", nil, "")
	staleSource := fixture.source("docker", "engine-a", "stale-box-key", "Stale box")
	staleSource["state"] = "stale"
	staleBox["sources"] = []any{staleSource} // a hostless guest the library lists under Needs attention
	fixture.nodes = append(fixture.nodes,
		infraCanvasFixtureNode("brain-b", "device", "TheBrain Backup", "nas", nil, ""),
		infraCanvasFixtureNode("brain-a", "device", "TheBrain", "mini-pc", nil, ""),
		infraCanvasFixtureNode("switch-spare", "device", "Core switch spare", "managed-switch", nil, ""),
		hiddenGuest, staleBox,
		infraCanvasFixtureNode("loose-box", "container", "Loose box", "container", nil, ""), // a hostless container is a Cleanup suggestion
		infraCanvasFixtureNode("annex", "room", "Brain annex", "room", nil, ""))             // an unplaced place: a row in the Map, none in Topology
	page := infraPolishOpen(t, fixture, 1500, 900)
	defer page.Close()
	page.MustEval(`()=>{
		window.infraQuery=value=>{const el=document.querySelector('[data-search]');el.value=value;el.dispatchEvent(new Event('input',{bubbles:true}));};
		window.infraBox=()=>{
			const box=document.querySelector('[data-no-match]'),show=box.querySelector('[data-action="show-match"]'),clear=box.querySelector('[data-cause]');
			return JSON.stringify({shown:!box.hidden,text:box.querySelector('[data-no-match-text]').textContent.trim(),show:!show.hidden,showLabel:show.textContent.trim(),clear:clear.dataset.action,clearLabel:clear.textContent.trim()});
		};
		window.infraWord=(key,count)=>window.fixtureWords[key].replace('{count}',count);
		window.infraOpenTitle=()=>document.querySelector('[data-inspector] h2')?.textContent||'';
	}`)
	type box struct {
		Shown, Show            bool
		Text, ShowLabel, Clear string
		ClearLabel             string
	}
	readBox := func() box {
		t.Helper()
		var got box
		if err := json.Unmarshal([]byte(page.MustEval(`()=>infraBox()`).Str()), &got); err != nil {
			t.Fatal(err)
		}
		return got
	}
	word := func(key string, count int) string {
		return page.MustEval(`(key,count)=>infraWord(key,count)`, key, count).Str()
	}
	waitText := func(want string) {
		t.Helper()
		page.MustWait(`(want)=>{const box=document.querySelector('[data-no-match]');return !box.hidden&&box.querySelector('[data-no-match-text]').textContent.trim()===want;}`, want)
	}
	search := page.MustElement(`[data-search]`)

	// Two matches, none on the map: the count, Show match as the primary action and the cause-matched clear action.
	page.MustEval(`()=>infraQuery('TheBrain')`)
	waitText(word("desktop.infra_no_match_unplaced", 2))
	if got, want := readBox(), (box{Shown: true, Show: true, Text: word("desktop.infra_no_match_unplaced", 2), ShowLabel: word("desktop.infra_show_match", 0), Clear: "clear-search", ClearLabel: word("desktop.infra_clear_search", 0)}); got != want {
		t.Fatalf("with two unplaced matches the empty map shows %+v; want %+v", got, want)
	}
	if !page.MustEval(`()=>{const show=document.querySelector('[data-no-match] [data-action="show-match"]'),clear=document.querySelector('[data-no-match] [data-cause]');return show.classList.contains('infra-button-primary')&&!clear.classList.contains('infra-button-primary');}`).Bool() {
		t.Fatal("Show match must be the primary action and the clear action the secondary one")
	}
	// Enter opens the first match by name ("TheBrain" before "TheBrain Backup") and shows its library row as selected, in view.
	search.MustFocus()
	page.Keyboard.MustType(input.Enter)
	page.MustWait(`()=>infraOpenTitle()==='TheBrain'`)
	if got := page.MustEval(`()=>{
		const row=document.querySelector('[data-library-item="brain-a"] [data-library-select]'),scroll=document.querySelector('[data-library-scroll]').getBoundingClientRect(),box=row.getBoundingClientRect();
		return JSON.stringify({selected:row.getAttribute('aria-selected')==='true',inView:box.top>=scroll.top-1&&box.bottom<=scroll.bottom+1,section:row.closest('[data-library-item]').dataset.section,
			open:document.querySelector('.infra-app').dataset.inspectorOpen,onMap:!!document.querySelector('[data-nodes] [data-node="brain-a"]'),focus:document.activeElement===document.querySelector('[data-search]')});
	}`).Str(); got != `{"selected":true,"inView":true,"section":"to-place","open":"true","onMap":false,"focus":true}` {
		t.Fatalf("Enter on an unplaced match must open it without placing it and keep the focus in the search: %s", got)
	}
	fixture.mu.Lock()
	layoutCalls := len(fixture.layoutCalls)
	fixture.mu.Unlock()
	if layoutCalls != 0 {
		t.Fatalf("revealing an unplaced match must not save a layout; got %d PUT /layout calls", layoutCalls)
	}
	// Show match does the same: select something else first, then click it.
	page.MustEval(`()=>document.querySelector('[data-node="fixture-node-0003"]').click()`)
	page.MustWait(`()=>infraOpenTitle()==='Cold-store NAS'`)
	page.MustEval(`()=>document.querySelector('[data-no-match] [data-action="show-match"]').click()`)
	page.MustWait(`()=>infraOpenTitle()==='TheBrain'`)

	// One match reads in the singular.
	page.MustEval(`()=>infraQuery('TheBrain Backup')`)
	waitText(word("desktop.infra_no_match_unplaced_one", 1))
	if got := readBox(); !got.Show || got.Text != word("desktop.infra_no_match_unplaced_one", 1) {
		t.Fatalf("with one unplaced match the empty map shows %+v", got)
	}

	// A guest of an unplaced host has no row of its own: it counts as its host's row, and Enter opens the host.
	page.MustEval(`()=>document.querySelector('[data-node="fixture-node-0003"]').click()`)
	page.MustWait(`()=>infraOpenTitle()==='Cold-store NAS'`)
	page.MustEval(`()=>infraQuery('Hidden guest')`)
	waitText(word("desktop.infra_no_match_unplaced_one", 1))
	// The library lists the same rows (it repaints on the next frame): the host of the matching guest, nothing else.
	page.MustWait(`()=>JSON.stringify([...document.querySelectorAll('[data-section="to-place"][data-library-item]')].map(row=>row.dataset.libraryItem))==='["brain-a"]'`)
	search.MustFocus()
	page.Keyboard.MustType(input.Enter)
	page.MustWait(`()=>infraOpenTitle()==='TheBrain'&&document.querySelector('[data-library-item="brain-a"] [data-library-select]').getAttribute('aria-selected')==='true'`)

	// A hostless container and a place in Topology are no To place rows: the plain "no match" box, no Show match, Enter does nothing.
	plain := func(query string) {
		t.Helper()
		page.MustEval(`(query)=>infraQuery(query)`, query)
		waitText(word("desktop.infra_no_match", 0))
		if got := readBox(); got.Show {
			t.Fatalf("%q matches nothing the library lists, but the box offers Show match: %+v", query, got)
		}
		before := page.MustEval(`()=>infraOpenTitle()`).Str()
		search.MustFocus()
		page.Keyboard.MustType(input.Enter)
		page.MustEval(`()=>new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve)))`)
		if got := page.MustEval(`()=>infraOpenTitle()`).Str(); got != before {
			t.Fatalf("%q: Enter must change nothing, the panel went from %q to %q", query, before, got)
		}
	}
	plain("Loose box")
	page.MustEval(`()=>infraQuery('Brain annex')`)
	waitText(word("desktop.infra_no_match_unplaced_one", 1)) // in the Map the unplaced place is a row
	page.MustEval(`()=>document.querySelector('.infra-view-switch [data-view="topology"]').click()`)
	page.MustWait(`()=>document.querySelector('.infra-view-switch [data-view="topology"]').getAttribute('aria-pressed')==='true'`)
	plain("Brain annex")
	page.MustEval(`()=>document.querySelector('.infra-view-switch [data-view="atlas"]').click()`)
	page.MustWait(`()=>document.querySelector('.infra-view-switch [data-view="atlas"]').getAttribute('aria-pressed')==='true'`)

	// A hostless guest that the library lists under Needs attention counts too, and Enter and Show match reveal its row there.
	page.MustEval(`()=>document.querySelector('[data-node="fixture-node-0003"]').click()`)
	page.MustWait(`()=>infraOpenTitle()==='Cold-store NAS'`)
	page.MustEval(`()=>infraQuery('Stale box')`)
	page.MustWait(`()=>!!document.querySelector('[data-library-item="stale-box"][data-section="attention"]')`)
	waitText(word("desktop.infra_no_match_unplaced_one", 1))
	search.MustFocus()
	page.Keyboard.MustType(input.Enter)
	page.MustWait(`()=>infraOpenTitle()==='Stale box'`)
	if got := page.MustEval(`()=>{
		const row=document.querySelector('[data-library-item="stale-box"] [data-library-select]'),scroll=document.querySelector('[data-library-scroll]').getBoundingClientRect(),box=row.getBoundingClientRect();
		return JSON.stringify({selected:row.getAttribute('aria-selected')==='true',inView:box.top>=scroll.top-1&&box.bottom<=scroll.bottom+1,section:row.closest('[data-library-item]').dataset.section,onMap:!!document.querySelector('[data-nodes] [data-node="stale-box"]')});
	}`).Str(); got != `{"selected":true,"inView":true,"section":"attention","onMap":false}` {
		t.Fatalf("Enter on a hostless guest must open it and show its Needs attention row selected: %s", got)
	}
	page.MustEval(`()=>document.querySelector('[data-node="fixture-node-0003"]').click()`)
	page.MustWait(`()=>infraOpenTitle()==='Cold-store NAS'`)
	page.MustEval(`()=>document.querySelector('[data-no-match] [data-action="show-match"]').click()`)
	page.MustWait(`()=>infraOpenTitle()==='Stale box'`)

	// Nothing matches at all: today's box, without Show match, and Enter does nothing.
	page.MustEval(`()=>infraQuery('zzzz-no-such-asset')`)
	waitText(word("desktop.infra_no_match", 0))
	if got, want := readBox(), (box{Shown: true, Text: word("desktop.infra_no_match", 0), ShowLabel: word("desktop.infra_show_match", 0), Clear: "clear-search", ClearLabel: word("desktop.infra_clear_search", 0)}); got != want {
		t.Fatalf("with no match at all the empty map shows %+v; want %+v", got, want)
	}
	before := page.MustEval(`()=>infraOpenTitle()`).Str()
	search.MustFocus()
	page.Keyboard.MustType(input.Enter)
	page.MustEval(`()=>new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve)))`)
	if got := page.MustEval(`()=>infraOpenTitle()`).Str(); got != before {
		t.Fatalf("Enter without a match must change nothing: the panel went from %q to %q", before, got)
	}

	// A match on the map wins: no overlay, and Enter selects and focuses its card, not the unplaced one.
	page.MustEval(`()=>infraQuery('Core switch')`)
	page.MustWait(`()=>document.querySelector('[data-no-match]').hidden&&document.querySelector('[data-node="fixture-node-0004"]')&&!document.querySelector('[data-node="fixture-node-0004"]').classList.contains('is-dimmed')`)
	search.MustFocus()
	page.Keyboard.MustType(input.Enter)
	page.MustWait(`()=>infraOpenTitle()==='Core switch'&&document.activeElement===document.querySelector('[data-node="fixture-node-0004"]')`)
	requireInfraBrowserClean(t, page)
}

// TestDesktopInfraSearchPlaceholderBrowser covers amendment A27: the Ctrl K badge only shows while the field still has
// room for the placeholder next to it, so the placeholder never truncates from 760 px container width up, in English
// and in German, with and without active filters.
func TestDesktopInfraSearchPlaceholderBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	var english map[string]any
	if err := json.Unmarshal([]byte(readDesktopAssetText(t, "lang/desktop/en.json")), &english); err != nil {
		t.Fatal(err)
	}
	words := map[string]any{}
	for key, value := range english {
		if strings.HasPrefix(key, "desktop.infra") || strings.HasPrefix(key, "desktop.app_infra") {
			words[key] = value
		}
	}
	const probe = `()=>{
		window.infraPlaceholder=()=>{
			const input=document.querySelector('[data-search]'),kbd=document.querySelector('.infra-search-kbd'),bar=document.querySelector('.infra-commandbar');
			const span=document.createElement('span'),style=getComputedStyle(input);
			span.style.cssText='position:absolute;visibility:hidden;white-space:pre;left:-9999px;font:'+style.fontStyle+' '+style.fontWeight+' '+style.fontSize+' '+style.fontFamily;
			span.textContent=input.placeholder;document.body.appendChild(span);
			const text=span.getBoundingClientRect().width;span.remove();
			return {app:document.querySelector('.infra-app').clientWidth,field:Math.round(input.clientWidth),text:Math.ceil(text),kbd:kbd.offsetParent!==null,overflow:bar.scrollWidth>bar.clientWidth+1,bar:bar.clientWidth+'/'+bar.scrollWidth+' '+[...bar.children].filter(c=>c.offsetParent!==null).map(c=>(c.className||c.tagName).toString().split(' ')[0]+':'+Math.round(c.getBoundingClientRect().width)).join(' ')};
		};
	}`
	for _, language := range []struct {
		name  string
		words map[string]any
	}{{"de", nil}, {"en", words}} {
		fixture := newInfraAppBrowserFixture()
		fixture.world(25)
		page := infraBrowserPageWithFixture(t, fixture)
		page.MustSetViewport(1920, 1200, 1, false)
		if language.words != nil {
			page.MustEval(`(words)=>Object.assign(window.fixtureWords,words)`, language.words)
		}
		openInfraBrowserApp(page)
		page.MustWait(`()=>document.querySelectorAll('[data-nodes] [data-node]').length>0`)
		page.MustEval(probe)
		wantPlaceholder := page.MustEval(`()=>window.fixtureWords['desktop.infra_search_jump']`).Str()
		if got := page.MustEval(`()=>document.querySelector('[data-search]').placeholder`).Str(); got != wantPlaceholder {
			t.Fatalf("%s: the placeholder is %q; want %q", language.name, got, wantPlaceholder)
		}
		for _, filtered := range []bool{false, true} {
			if filtered {
				page.MustEval(`()=>{const set=(selector,value)=>{const el=document.querySelector(selector);el.value=value;el.dispatchEvent(new Event('change',{bubbles:true}));};set('[data-kind-filter]','device');set('[data-source-filter]','docker');set('[data-power-filter]','running');}`)
				page.MustWait(`()=>document.querySelectorAll('[data-remove-filter]').length===3`)
			}
			shown, hidden := 0, 0
			for width := 760; width <= 1500; width += 20 {
				infraPolishResize(page, width, 800)
				page.MustEval(`()=>new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve)))`)
				var got struct {
					App, Field, Text int
					Kbd, Overflow    bool
					Bar              string
				}
				if err := json.Unmarshal([]byte(page.MustEval(`()=>JSON.stringify(infraPlaceholder())`).Str()), &got); err != nil {
					t.Fatal(err)
				}
				if got.Field < got.Text || got.Overflow {
					t.Fatalf("%s, filters %v, %d px window: the field is %d px wide for a %d px placeholder (overflow %v, badge %v): %s", language.name, filtered, width, got.Field, got.Text, got.Overflow, got.Kbd, got.Bar)
				}
				if got.Kbd {
					shown++
				} else {
					hidden++
				}
			}
			t.Logf("%s, filters %v: the badge shows at %d of %d widths", language.name, filtered, shown, shown+hidden)
			if !filtered && shown == 0 {
				t.Fatalf("%s: without filters the badge must show where there is room for it", language.name)
			}
		}
		requireInfraBrowserClean(t, page)
		page.Close()
	}
}
