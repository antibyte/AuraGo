package ui

import (
	"testing"

	"github.com/go-rod/rod"
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
			name:Math.min(...names.map(infraOnScreen))>=12,header:infraOnScreen(header)>=13,meta:getComputedStyle(card.querySelector('.infra-card-meta')).display,box:[card.offsetWidth,card.offsetHeight],overlaps});
	}`).Str(); got != `{"compact":true,"tiny":true,"handles":"none","name":true,"header":true,"meta":"none","box":[216,64],"overlaps":0}` {
		t.Fatalf("at about 28%% the cards must be compact with legible, non-overlapping labels and the same geometry: %s", got)
	}
	// Zoom back in: 0.58 is still compact (below 0.65), the full form returns from 0.68 on.
	page.MustEval(`()=>{for(let i=0;i<3;i++)document.querySelector('[data-action="zoom-in"]').click();}`)
	page.MustWait(`()=>infraZoomOf()>0.45`)
	infraPolishSettle(page)
	if got := page.MustEval(`()=>{
		const stage=document.querySelector('[data-stage]'),card=document.querySelector('[data-node="fixture-node-0002"]');
		return JSON.stringify({compact:stage.classList.contains('is-compact'),tiny:stage.classList.contains('is-tiny'),handles:getComputedStyle(card.querySelector('.infra-card-handle')).display!=='none',lines:getComputedStyle(card.querySelector('.infra-card-name')).webkitLineClamp});
	}`).Str(); got != `{"compact":true,"tiny":false,"handles":false,"lines":"2"}` {
		// 0.28 * 1.2^3 = 0.48: below 0.5 the handles are still hidden.
		t.Fatalf("at about 48%% the compact form shows two-line names and still no handles: %s", got)
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
	requireInfraBrowserClean(t, page)
}

// TestDesktopInfraMinimapBrowser covers amendment A3: a 168x112 overview with no text inside, filled with the theme
// overlay colour and blur, that hides itself while the whole scene fits and never covers a card at fit. The shown
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

	// Zoomed in the scene overflows: the overview appears at 168x112 over the theme overlay colour with the theme blur.
	page.MustEval(`()=>document.querySelector('[data-action="zoom-reset"]').click()`)
	page.MustWait(`()=>infraZoomOf()===1&&!document.querySelector('[data-minimap]').hidden`)
	if got := page.MustEval(`()=>{
		const map=document.querySelector('[data-minimap]'),r=map.getBoundingClientRect(),cs=getComputedStyle(map);
		const probe=document.createElement('i');probe.style.cssText='background:var(--vd-theme-overlay-bg);backdrop-filter:var(--vd-theme-blur, blur(12px))';document.body.appendChild(probe);
		const want=getComputedStyle(probe);const result={size:[Math.round(r.width),Math.round(r.height)],background:cs.backgroundColor===want.backgroundColor,blur:cs.backdropFilter===want.backdropFilter,text:map.textContent.trim()};probe.remove();
		return JSON.stringify(result);
	}`).Str(); got != `{"size":[168,112],"background":true,"blur":true,"text":""}` {
		t.Fatalf("the minimap must be 168x112, filled with the overlay colour and blur and hold no text: %s", got)
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
