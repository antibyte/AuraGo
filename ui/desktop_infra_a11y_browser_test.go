package ui

import (
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/input"
	"github.com/go-rod/rod/lib/proto"
)

// infraThemeConfig is one desktop theme setup for the Infra theme checks.
type infraThemeConfig struct {
	name, theme, setting, mode, scheme string
}

// infraReviewThemes are the themes of the visual review (spec section 13, design QA A25).
var infraReviewThemes = []infraThemeConfig{
	{"standard", "standard", "", "", "dark"},
	{"fruity-light", "fruity", "appearance.fruity_mode", "light", "light"},
	{"fruity-dark", "fruity", "appearance.fruity_mode", "dark", "dark"},
	{"minimal-light", "minimal", "appearance.minimal_mode", "light", "light"},
	{"minimal-dark", "minimal", "appearance.minimal_mode", "dark", "dark"},
	{"orbit", "orbit", "", "", "dark"},
	{"windows-xp", "windows_xp", "", "", "light"},
	{"amiga", "amiga", "", "", "light"},
}

// infraContrastAllow lowers the contrast floor of one named check in one theme, with the reason. Keep it short: an entry is a
// theme-token problem outside Infra (this test must not edit a theme), pinned at today's value so it cannot get worse.
//
//   - Standard, Fruity light and Fruity dark: the primary button (Add) is white on the shell accent, #E95420 and #0a84ff. Both
//     accent and accent-contrast come from the theme (ui/css/desktop-base.css) and every app's primary button uses the pair;
//     white on either accent is 3.65:1, below 4.5:1 for 14 px bold text. Infra cannot darken the accent per theme without a
//     theme-id selector, so the pair has to be fixed in the theme.
var infraContrastAllow = map[string]map[string]float64{
	"standard":     {"primary button": 3.6},
	"fruity-light": {"primary button": 3.6},
	"fruity-dark":  {"primary button": 3.6},
}

// infraThemePairs copies the contrast pairs with the floors this theme is allowed (see infraContrastAllow).
func infraThemePairs(theme infraThemeConfig, pairs [][]any) [][]any {
	out := make([][]any, 0, len(pairs))
	for _, pair := range pairs {
		next := append([]any(nil), pair...)
		if min, ok := infraContrastAllow[theme.name][pair[0].(string)]; ok {
			next[2] = min
		}
		out = append(out, next)
	}
	return out
}

func applyInfraTheme(page *rod.Page, c infraThemeConfig) {
	page.MustEval(`(theme,setting,mode,scheme)=>{const s=auditTest.state.bootstrap.settings;s['appearance.theme']=theme;if(setting)s[setting]=mode;document.documentElement.dataset.theme=scheme;auditTest.applyDesktopSettings();}`, c.theme, c.setting, c.mode, c.scheme)
	page.MustWait(`(theme,scheme)=>document.body.dataset.theme===theme&&document.body.dataset.scheme===scheme`, c.theme, c.scheme)
}

// infraContrastProbeJS installs window.infraProbe: WCAG contrast of an
// element's text against its composited background (or against the background
// of a named stand-in, for text that sits on the canvas although it is a
// descendant of a card), contrast of a border against the canvas, font-size
// floors and the focus ring of the focused element. Any CSS colour syntax is
// resolved by painting it on a 1x1 canvas.
const infraContrastProbeJS = `()=>{
	const canvas=document.createElement('canvas');canvas.width=canvas.height=1;
	const ctx=canvas.getContext('2d',{willReadFrequently:true});
	const rgba=value=>{ctx.clearRect(0,0,1,1);ctx.fillStyle='rgba(0,0,0,0)';ctx.fillStyle=value;ctx.fillRect(0,0,1,1);const d=ctx.getImageData(0,0,1,1).data;return [d[0],d[1],d[2],d[3]/255];};
	const over=(top,bottom)=>{const a=top[3]+bottom[3]*(1-top[3]);if(!a)return [0,0,0,0];return [0,1,2].map(i=>(top[i]*top[3]+bottom[i]*bottom[3]*(1-top[3]))/a).concat([a]);};
	const background=el=>{const layers=[];for(let node=el;node&&node.nodeType===1;node=node.parentElement){const c=rgba(getComputedStyle(node).backgroundColor);if(c[3]>0){layers.push(c);if(c[3]>=0.99)break;}}let out=[255,255,255,1];for(let i=layers.length-1;i>=0;i-=1)out=over(layers[i],out);return out;};
	const lum=c=>{const f=v=>{v/=255;return v<=0.03928?v/12.92:Math.pow((v+0.055)/1.055,2.4);};return 0.2126*f(c[0])+0.7152*f(c[1])+0.0722*f(c[2]);};
	const ratio=(a,b)=>{const x=lum(a),y=lum(b);return (Math.max(x,y)+0.05)/(Math.min(x,y)+0.05);};
	const opacity=el=>{let o=1;for(let node=el;node&&node.nodeType===1;node=node.parentElement)o*=Number(getComputedStyle(node).opacity);return o;};
	const contrast=(el,bgEl)=>{const bg=background(bgEl||el);return Math.round(ratio(over(rgba(getComputedStyle(el).color),bg),bg)*100)/100;};
	window.infraProbe={
		contrast,
		check(pairs){return pairs.flatMap(([label,selector,min,bgSelector])=>{const el=document.querySelector(selector);if(!el||!el.getClientRects().length)return [label+': missing'];if(opacity(el)<0.99)return [label+': faded'];const bgEl=bgSelector?document.querySelector(bgSelector):null;if(bgSelector&&!bgEl)return [label+': missing background '+bgSelector];const value=contrast(el,bgEl);return value>=min?[]:[label+': '+value];});},
		edges(list){const bg=background(document.querySelector('.infra-canvas'));return list.flatMap(([label,selector,min])=>{const el=document.querySelector(selector);if(!el||!el.getClientRects().length)return [label+': missing'];const style=getComputedStyle(el);if(style.borderTopStyle==='none'||!(parseFloat(style.borderTopWidth)>0))return [label+': no border'];const value=Math.round(ratio(over(rgba(style.borderTopColor),bg),bg)*100)/100;return value>=min?[]:[label+': '+value];});},
		floors(list){return list.flatMap(([label,selector,min])=>{const el=document.querySelector(selector);if(!el)return [label+': missing'];const size=parseFloat(getComputedStyle(el).fontSize);return size>=min?[]:[label+': '+size+'px'];});},
		focusRing(el){if(!el||el===document.body)return {ok:false,reason:'nothing focused'};const style=getComputedStyle(el);const width=parseFloat(style.outlineWidth)||0;if(style.outlineStyle==='none'||width<1)return {ok:false,reason:'outline '+style.outlineStyle+' '+style.outlineWidth,tag:el.tagName};const bg=background(el.parentElement);const value=Math.round(ratio(over(rgba(style.outlineColor),bg),bg)*100)/100;return {ok:value>=3,ratio:value,tag:el.tagName};},
	};
	return true;
}`

// infraFlakySnapshot answers GET /snapshot with 503 while fail is set.
type infraFlakySnapshot struct {
	base http.Handler
	fail atomic.Bool
}

func (h *infraFlakySnapshot) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h.fail.Load() && r.URL.Path == "/api/desktop/infra/snapshot" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"error":"provider_unavailable"}`))
		return
	}
	h.base.ServeHTTP(w, r)
}

func infraA11yFixture() *infraAppBrowserFixture {
	fixture := newInfraAppBrowserFixture()
	fixture.nodes = []map[string]any{
		infraCanvasFixtureNode("room-1", "room", "Living room", "room", map[string]any{"x": 400, "y": 120, "w": 480, "h": 320, "pinned": true}, ""),
		infraCanvasFixtureNode("dev-a", "device", "Edge router", "router", map[string]any{"x": 160, "y": 200, "pinned": true}, ""),
		infraCanvasFixtureNode("dev-b", "device", "Core switch", "managed-switch", map[string]any{"x": 160, "y": 420, "pinned": true}, ""),
		infraCanvasFixtureNode("dev-c", "device", "Media box", "mini-pc", map[string]any{"x": 640, "y": 300, "pinned": true}, "room-1"),
		infraCanvasFixtureNode("dev-d", "device", "Cold NAS", "nas", nil, ""),
	}
	fixture.edges = []map[string]any{{"id": "edge-ab", "version": 1, "from": "dev-a", "to": "dev-b", "kind": "ethernet"}}
	return fixture
}

// infraA11yPage opens Infra at 1500x970 and records every scheduled 30 s poll
// in window.infraPolls, so a test can run a poll without waiting for it.
func infraA11yPage(t *testing.T, handler http.Handler) *rod.Page {
	t.Helper()
	page := infraBrowserPageWithFixture(t, handler)
	page.MustSetViewport(1920, 1200, 1, false)
	page.MustEval(`()=>{
		Object.keys(localStorage).filter(key=>key.startsWith('infra.')).forEach(key=>localStorage.removeItem(key));
		window.infraPolls=[];
		const set=window.setTimeout.bind(window);
		window.setTimeout=(fn,delay,...args)=>{if(Number(delay)===30000)window.infraPolls.push(fn);return set(fn,delay,...args);};
	}`)
	openInfraBrowserApp(page)
	page.MustWait(`()=>!!document.querySelector('[data-nodes] [data-node]')`)
	page.MustEval(`()=>{const win=document.querySelector('.vd-window[data-app-id="infra"]');win.classList.remove('maximized','vd-mobile-forced-maximized');win.style.right='auto';win.style.width='1500px';win.style.height='970px';win.style.left='210px';win.style.top='70px';}`)
	page.MustWait(`()=>document.querySelector('.infra-app').clientWidth>1000`)
	page.MustEval(`()=>document.querySelector('[data-action="fit"]').click()`)
	return page
}

func runInfraPoll(page *rod.Page) {
	page.MustEval(`async()=>{const poll=window.infraPolls.at(-1);if(!poll)throw new Error('no 30 s poll was scheduled');await poll();}`)
}

// TestDesktopInfraAccessibilityBrowser covers spec section 5.3:
//   - the map is one tab stop and toolbars rove;
//   - tablist structure, and the combobox pattern with focus return;
//   - focus returns after the inspector, the shortcut dialog, the import
//     sheet and the narrow library sheet;
//   - routine polls never announce, while a lost and recovered sync is
//     announced once each;
//   - reduced motion and Window animations off stop every Infra motion.
func TestDesktopInfraAccessibilityBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	fixture := infraA11yFixture()
	flaky := &infraFlakySnapshot{base: fixture}
	page := infraA11yPage(t, flaky)
	defer page.Close()
	focused := func(selector string) bool {
		return page.MustEval(`selector=>document.activeElement===document.querySelector(selector)`, selector).Bool()
	}

	// One tab stop for the map, one per toolbar, and arrows rove.
	if got := page.MustEval(`()=>{const stage=document.querySelector('[data-stage]');return [stage,...stage.querySelectorAll('button,[tabindex]')].filter(el=>el.tabIndex>=0&&!el.disabled).length;}`).Int(); got != 1 {
		t.Fatalf("the map must be a single tab stop, found %d", got)
	}
	if got := page.MustEval(`()=>JSON.stringify([...document.querySelectorAll('.infra-app [role="toolbar"]')].map(bar=>[...bar.querySelectorAll('button')].filter(button=>!button.disabled&&button.getClientRects().length&&button.tabIndex===0).length).filter(count=>count!==1))`).Str(); got != "[]" {
		t.Fatalf("every toolbar needs exactly one tab stop: %s", got)
	}
	page.MustEval(`()=>document.querySelector('[data-tools] button[tabindex="0"]').focus()`)
	first := page.MustEval(`()=>{const el=document.activeElement;return el.dataset.tool||el.dataset.action;}`).Str()
	page.Keyboard.MustType(input.ArrowRight)
	if !page.MustEval(`(first)=>{const el=document.activeElement;return (el.dataset.tool||el.dataset.action)!==first&&el.tabIndex===0&&!!el.closest('[data-tools]')&&[...document.querySelectorAll('[data-tools] button')].filter(button=>button.tabIndex===0).length===1;}`, first).Bool() {
		t.Fatal("ArrowRight must move the toolbar's single tab stop to the next tool")
	}
	page.Keyboard.MustType(input.End)
	if !page.MustEval(`()=>document.activeElement===[...document.querySelectorAll('[data-tools] button:not(:disabled)')].at(-1)`).Bool() {
		t.Fatal("End must reach the last tool")
	}
	page.Keyboard.MustType(input.Home)
	if !page.MustEval(`()=>document.activeElement===document.querySelector('[data-tools] button:not(:disabled)')`).Bool() {
		t.Fatal("Home must reach the first tool")
	}

	// Tablist structure.
	page.MustEval(`()=>{const card=document.querySelector('[data-node="dev-a"]');card.click();card.focus();}`)
	page.MustWait(`()=>!document.querySelector('[data-inspector]').hidden&&!!document.querySelector('[data-inspector] [role="tablist"]')`)
	if got := page.MustEval(`()=>{const problems=[];for(const list of document.querySelectorAll('.infra-app [role="tablist"]')){const tabs=[...list.querySelectorAll('[role="tab"]')];const stops=tabs.filter(tab=>tab.tabIndex===0),chosen=tabs.filter(tab=>tab.getAttribute('aria-selected')==='true');if(stops.length!==1||chosen.length!==1||stops[0]!==chosen[0])problems.push('roving');for(const tab of tabs){const panel=document.getElementById(tab.getAttribute('aria-controls'));if(!panel||panel.getAttribute('role')!=='tabpanel'||panel.getAttribute('aria-labelledby')!==tab.id)problems.push('link '+tab.id);else if(panel.hidden===(tab.getAttribute('aria-selected')==='true'))problems.push('visibility '+tab.id);}}return JSON.stringify(problems);}`).Str(); got != "[]" {
		t.Fatalf("inspector tabs break the tab pattern: %s", got)
	}

	// Combobox: ARIA 1.2 structure, active option moves, Escape gives focus back.
	page.MustEval(`()=>document.querySelector('[data-inspector-tab="connections"]').click()`)
	page.MustEval(`()=>{const add=document.querySelector('[data-inspector-panel="connections"] [data-action="add-edge"]');add.focus();add.click();}`)
	page.MustWait(`()=>!!document.querySelector('[data-picker] [data-picker-input]')`)
	if got := page.MustEval(`()=>{const input=document.querySelector('[data-picker] [data-picker-input]'),list=document.getElementById(input.getAttribute('aria-controls')),active=document.getElementById(input.getAttribute('aria-activedescendant'));return JSON.stringify({role:input.getAttribute('role'),expanded:input.getAttribute('aria-expanded'),list:list&&list.getAttribute('role'),active:active&&active.getAttribute('role'),selected:active&&active.getAttribute('aria-selected'),focused:document.activeElement===input});}`).Str(); got != `{"role":"combobox","expanded":"true","list":"listbox","active":"option","selected":"true","focused":true}` {
		t.Fatalf("the asset picker is not an ARIA combobox: %s", got)
	}
	before := page.MustEval(`()=>document.querySelector('[data-picker] [data-picker-input]').getAttribute('aria-activedescendant')`).Str()
	page.Keyboard.MustType(input.ArrowDown)
	if after := page.MustEval(`()=>document.querySelector('[data-picker] [data-picker-input]').getAttribute('aria-activedescendant')`).Str(); after == before || after == "" {
		t.Fatalf("ArrowDown must move the active option (before %q, after %q)", before, after)
	}
	page.Keyboard.MustType(input.Escape)
	page.MustWait(`()=>!document.querySelector('[data-picker]')`)
	if !focused(`[data-inspector-panel="connections"] [data-action="add-edge"]`) {
		t.Fatal("closing the picker must give focus back to Add connection")
	}

	// The inspector gives focus back to the card.
	page.MustEval(`()=>document.querySelector('[data-inspector-tab="overview"]').focus()`)
	page.Keyboard.MustType(input.Escape)
	page.MustWait(`()=>document.querySelector('[data-inspector]').hidden`)
	if !focused(`[data-node="dev-a"]`) {
		t.Fatal("closing the inspector must give focus back to the asset card")
	}

	// The shortcut dialog gives focus back to where it was opened.
	page.MustEval(`()=>document.querySelector('[data-node="dev-a"]').dispatchEvent(new KeyboardEvent('keydown',{key:'?',bubbles:true}))`)
	page.MustWait(`()=>document.querySelector('[data-dialog]').open`)
	page.Keyboard.MustType(input.Escape)
	page.MustWait(`()=>!document.querySelector('[data-dialog]').open`)
	if !focused(`[data-node="dev-a"]`) {
		t.Fatal("closing the shortcut sheet must give focus back to where it was opened")
	}

	// The import sheet, opened from the Add menu, gives focus back to the menu button.
	page.MustEval(`()=>{const trigger=document.querySelector('[data-action="add-menu"]');trigger.focus();trigger.click();}`)
	page.MustWait(`()=>!document.querySelector('[data-menu="add"]').hidden`)
	page.MustEval(`()=>document.querySelector('[data-menu="add"] [data-action="import"]').click()`)
	page.MustWait(`()=>!document.querySelector('[data-import-sheet]').hidden`)
	page.MustEval(`()=>document.querySelector('[data-import-search]').focus()`)
	page.Keyboard.MustType(input.Escape)
	page.MustWait(`()=>document.querySelector('[data-import-sheet]').hidden`)
	if !focused(`[data-action="add-menu"]`) {
		t.Fatal("closing the import sheet must give focus back to the Add menu button")
	}

	// The narrow library sheet takes focus and gives it back on Escape.
	page.MustEval(`()=>{const win=document.querySelector('.vd-window[data-app-id="infra"]');win.style.width='430px';win.style.height='700px';}`)
	page.MustWait(`()=>document.querySelector('.infra-app').clientWidth<500&&getComputedStyle(document.querySelector('.infra-library')).display==='none'`)
	page.MustEval(`()=>{const toggle=document.querySelector('[data-action="toggle-library"]');toggle.focus();toggle.click();}`)
	page.MustWait(`()=>document.querySelector('.infra-app').dataset.librarySheet==='open'&&document.querySelector('[data-library]').contains(document.activeElement)`)
	page.Keyboard.MustType(input.Escape)
	page.MustWait(`()=>document.querySelector('.infra-app').dataset.librarySheet==='closed'`)
	if !focused(`[data-action="toggle-library"]`) || page.MustEval(`()=>document.querySelector('[data-action="toggle-library"]').getAttribute('aria-expanded')`).Str() != "false" {
		t.Fatal("Escape must close the library sheet, report it collapsed and give focus back to the library button")
	}
	page.MustEval(`()=>{const win=document.querySelector('.vd-window[data-app-id="infra"]');win.style.width='1500px';win.style.height='970px';}`)
	page.MustWait(`()=>document.querySelector('.infra-app').clientWidth>1000`)

	// Live regions: routine polls stay silent; losing and regaining sync speak once each. The desktop shell itself is a polite live
	// region (main.vd-shell), so only a region inside the Infra window counts against the indicator.
	if got := page.MustEval(`()=>{const region=document.querySelector('[data-live]').closest('[aria-live]:not([aria-live="off"]),[role="status"],[role="alert"]');return region&&document.querySelector('.infra-app').contains(region)?region.tagName+'.'+region.className+' role='+region.getAttribute('role')+' live='+region.getAttribute('aria-live'):'';}`).Str(); got != "" {
		t.Fatalf("the sync indicator must not be inside a live region: %s", got)
	}
	page.MustEval(`()=>{
		const app=document.querySelector('.infra-app');window.infraLive=[];
		const regionOf=node=>{const el=node&&node.nodeType===1?node:node&&node.parentElement;return el&&el.closest('[aria-live]:not([aria-live="off"]),[role="status"],[role="alert"],[role="log"]');};
		new MutationObserver(records=>{const touched=new Set();for(const record of records){const region=regionOf(record.target);if(region&&app.contains(region)&&!region.closest('[hidden]'))touched.add(region);}for(const region of touched){const text=region.textContent.trim();if(text)window.infraLive.push(text);}}).observe(app,{subtree:true,childList:true,characterData:true,attributes:true,attributeFilter:['hidden']});
	}`)
	page.MustEval(`()=>document.querySelector('[data-node="dev-a"]').click()`)
	page.MustWait(`()=>!document.querySelector('[data-inspector]').hidden`)
	page.MustEval(`()=>document.querySelector('[data-stage]').focus()`) // focus outside the inspector, so polls repaint it
	time.Sleep(300 * time.Millisecond)
	page.MustEval(`()=>{window.infraLive.length=0;}`)
	runInfraPoll(page)
	runInfraPoll(page)
	fixture.mu.Lock()
	for _, node := range fixture.nodes {
		if node["id"] == "dev-a" {
			node["name"] = "Edge router (renamed elsewhere)"
			node["version"] = infraFixtureNumber(node["version"]) + 1
		}
	}
	fixture.mu.Unlock()
	runInfraPoll(page)
	page.MustWait(`()=>document.querySelector('[data-inspector] h2').textContent.includes('renamed elsewhere')`)
	time.Sleep(300 * time.Millisecond)
	if got := page.MustEval(`()=>JSON.stringify(window.infraLive)`).Str(); got != "[]" {
		t.Fatalf("routine polls must not announce anything: %s", got)
	}
	stale, recovered := infraGermanWord(t, "desktop.infra_sync_stale"), infraGermanWord(t, "desktop.infra_sync_recovered")
	flaky.fail.Store(true)
	runInfraPoll(page)
	page.MustWait(`()=>document.querySelector('[data-live]').dataset.sync==='error'`)
	page.MustWait(`(text)=>window.infraLive.includes(text)`, stale)
	time.Sleep(300 * time.Millisecond)
	spoken := page.MustEval(`()=>window.infraLive.length`).Int()
	runInfraPoll(page)
	time.Sleep(400 * time.Millisecond)
	if got := page.MustEval(`()=>window.infraLive.length`).Int(); got != spoken {
		t.Fatalf("a poll that keeps failing must not announce again: %s", page.MustEval(`()=>JSON.stringify(window.infraLive)`).Str())
	}
	flaky.fail.Store(false)
	runInfraPoll(page)
	page.MustWait(`(text)=>window.infraLive.at(-1)===text`, recovered)

	// Motion: reduced motion and Window animations off stop every Infra motion.
	setMotion := func(media string, animations bool) {
		t.Helper()
		if err := (proto.EmulationSetEmulatedMedia{Features: []*proto.EmulationMediaFeature{{Name: "prefers-reduced-motion", Value: media}}}).Call(page); err != nil {
			t.Fatal(err)
		}
		page.MustEval(`(on)=>{auditTest.state.bootstrap.settings['windows.animations']=on?'true':'false';auditTest.applyDesktopSettings();}`, animations)
		page.MustWait(`(on)=>document.body.dataset.animations===(on?'true':'false')`, animations)
	}
	motion := func() string {
		return page.MustEval(`async()=>{
			const scene=document.querySelector('[data-scene]');let animated=false;
			const watch=new MutationObserver(()=>{if(scene.classList.contains('is-animating'))animated=true;});
			watch.observe(scene,{attributes:true,attributeFilter:['class']});
			document.querySelector('[data-node="dev-b"]').click();
			document.querySelector('[data-action="zoom-in"]').click();
			document.querySelector('[data-action="fit"]').click();
			await new Promise(resolve=>setTimeout(resolve,120));
			watch.disconnect();
			const any=value=>value.split(',').some(part=>parseFloat(part)>0);
			const moving=['.infra-inspector','.infra-library','.infra-scrim','.infra-card','.infra-scene'].filter(selector=>{const el=document.querySelector(selector);if(!el)return false;const style=getComputedStyle(el);return any(style.transitionDuration)||(style.animationName!=='none'&&any(style.animationDuration));});
			return JSON.stringify({animated,moving});
		}`).Str()
	}
	setMotion("no-preference", true)
	if got := motion(); !strings.Contains(got, `"animated":true`) || !strings.Contains(got, ".infra-card") {
		t.Fatalf("test control: with motion allowed, Fit should animate and cards should transition: %s", got)
	}
	setMotion("reduce", true)
	if got := motion(); got != `{"animated":false,"moving":[]}` {
		t.Fatalf("prefers-reduced-motion must stop every Infra transition and animation: %s", got)
	}
	setMotion("no-preference", false)
	if got := motion(); got != `{"animated":false,"moving":[]}` {
		t.Fatalf("Window animations off must stop every Infra transition and animation: %s", got)
	}
	closeInfraBrowserWindow(t, page)
	requireInfraBrowserClean(t, page)
}

// TestDesktopInfraThemeContrastBrowser checks the review themes at 1500x970:
// text contrast of every key surface, the text floors, no overflow in the
// command bar, library and inspector, a visible focus ring on a card, a
// readable lane label in Topology, and (design QA A25) a place outline and a
// lane separator that keep at least 1.4:1 against the canvas.
func TestDesktopInfraThemeContrastBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	page := infraA11yPage(t, infraA11yFixture())
	defer page.Close()
	page.MustEval(infraContrastProbeJS)
	page.MustEval(`()=>document.querySelector('[data-node="dev-a"]').click()`)
	page.MustWait(`()=>!document.querySelector('[data-inspector]').hidden&&!!document.querySelector('[data-inspector] .infra-facts dt')`)
	atlasPairs := [][]any{
		{"card name", `[data-node="dev-a"] .infra-card-name`, 4.5},
		{"card meta", `[data-node="dev-a"] .infra-card-meta`, 4.5},
		{"region name", `[data-region="room-1"] .infra-region-name`, 4.5},
		{"library row", `.infra-library-row-name`, 4.5},
		{"library meta", `.infra-library-row-meta`, 4.5},
		{"library section", `.infra-library-title`, 4.5},
		{"view switch", `.infra-view-switch button[aria-pressed="false"]`, 4.5},
		{"primary button", `.infra-split .infra-button-primary`, 4.5},
		{"inspector title", `[data-inspector] h2`, 4.5},
		{"inspector tab", `[data-inspector] [role="tab"][aria-selected="false"]`, 4.5},
		{"inspector fact", `[data-inspector] .infra-facts dt`, 4.5},
	}
	floors := [][]any{
		{"card name", `.infra-card-name`, 16}, {"card meta", `.infra-card-meta`, 14}, {"library row", `.infra-library-row-name`, 14},
		{"library section", `.infra-library-title`, 13}, {"view switch", `.infra-view-switch button`, 14}, {"inspector tab", `[data-inspector] [role="tab"]`, 14},
	}
	for _, theme := range infraReviewThemes {
		applyInfraTheme(page, theme)
		page.MustEval(`()=>document.querySelector('[data-view="atlas"]').click()`)
		page.MustWait(`()=>!!document.querySelector('[data-region="room-1"] .infra-region-name')`)
		page.MustEval(`()=>document.querySelector('[data-action="fit"]').click()`)
		infraPolishSettle(page)
		if got := page.MustEval(`(pairs,floors)=>JSON.stringify({contrast:infraProbe.check(pairs),floors:infraProbe.floors(floors),outline:infraProbe.edges([['place outline','[data-region="room-1"]',1.4]]),overflow:['.infra-commandbar','.infra-library','.infra-inspector'].filter(selector=>{const el=document.querySelector(selector);return el&&el.getClientRects().length&&el.scrollWidth>el.clientWidth+1;})})`, infraThemePairs(theme, atlasPairs), floors).Str(); got != `{"contrast":[],"floors":[],"outline":[],"overflow":[]}` {
			t.Fatalf("%s: %s", theme.name, got)
		}
		page.MustEval(`()=>document.querySelector('[data-stage]').focus()`)
		page.Keyboard.MustType(input.ArrowRight)
		if ring := page.MustEval(`()=>JSON.stringify(infraProbe.focusRing(document.activeElement))`).Str(); !strings.Contains(ring, `"ok":true`) {
			t.Fatalf("%s: the focused card has no visible focus ring: %s", theme.name, ring)
		}
		page.MustEval(`()=>document.querySelector('[data-view="topology"]').click()`)
		page.MustWait(`()=>!document.querySelector('[data-lanes]').hidden&&!!document.querySelector('[data-lane="network"] .infra-lane-label')&&!!document.querySelector('[data-node="dev-a"]')`)
		page.MustEval(`()=>document.querySelector('[data-action="fit"]').click()`)
		infraPolishSettle(page)
		lanePairs := [][]any{{"lane label", `[data-lane="network"] .infra-lane-label`, 4.5}, {"card in a lane", `[data-node="dev-a"] .infra-card-name`, 4.5}}
		if got := page.MustEval(`(pairs,floors)=>JSON.stringify(infraProbe.check(pairs).concat(infraProbe.floors(floors),infraProbe.edges([['lane separator','[data-lane="network"]',1.4],['lane separator (first lane)','[data-lane="internet"]',1.4]])))`, lanePairs, [][]any{{"lane label", `.infra-lane-label`, 14}}).Str(); got != "[]" {
			t.Fatalf("%s topology: %s", theme.name, got)
		}
		// Zoomed out the cards are compact and their names sit on the canvas under the card, not on it.
		page.MustEval(`()=>{for(let i=0;i<8;i++)document.querySelector('[data-action="zoom-out"]').click();}`)
		page.MustWait(`()=>document.querySelector('[data-stage]').classList.contains('is-compact')`)
		infraPolishSettle(page)
		if got := page.MustEval(`(pairs)=>JSON.stringify(infraProbe.check(pairs))`, [][]any{{"compact card name", `[data-node="dev-a"] .infra-card-name`, 4.5, `.infra-canvas`}}).Str(); got != "[]" {
			t.Fatalf("%s compact topology: %s", theme.name, got)
		}
	}
	closeInfraBrowserWindow(t, page)
	requireInfraBrowserClean(t, page)
}
