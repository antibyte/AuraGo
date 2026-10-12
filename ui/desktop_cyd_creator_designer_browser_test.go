package ui

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/input"
	"github.com/go-rod/rod/lib/proto"
)

type cydDesignerSmoke struct {
	t    *testing.T
	page *rod.Page
	dir  string
}

func (s *cydDesignerSmoke) wait(expr string) {
	s.t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if s.page.MustEval(expr).Bool() {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	s.t.Fatalf("condition did not become true: %s\nstate: %s", expr, s.page.MustEval(`()=>JSON.stringify({errors:cydFixture.errors,notices:cydFixture.notices,pageKeys:cydFixture.pageKeys,selection:cydFixture.state.selection,screen:cydFixture.state.screenId,zoom:cydFixture.state.zoom,focus:document.activeElement?document.activeElement.outerHTML.slice(0,160):'',widget:cydFixture.state.selection[0]?cydFixture.widget(cydFixture.state.selection[0]):null})`).Str())
}

// shot saves a screenshot once two frames and a short pause let the layout settle.
func (s *cydDesignerSmoke) shot(name string) {
	s.t.Helper()
	s.page.MustEval(`()=>new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(()=>setTimeout(r,160))))`)
	dest := filepath.Join(s.dir, name+".png")
	s.page.MustScreenshot(dest)
	if info, err := os.Stat(dest); err != nil || info.Size() == 0 {
		s.t.Fatalf("screenshot %s missing or empty (%v)", dest, err)
	}
	s.t.Logf("wrote %s", dest)
}

func (s *cydDesignerSmoke) noErrors(step string) {
	s.t.Helper()
	if got := s.page.MustEval(`()=>JSON.stringify(cydFixture.errors)`).Str(); got != "[]" {
		s.t.Fatalf("%s: page errors %s", step, got)
	}
}

// noRawKeys fails when the designer shows an untranslated cyd_creator.* key in its text or in a
// title, label or placeholder.
func (s *cydDesignerSmoke) noRawKeys(step string) {
	s.t.Helper()
	found := s.page.MustEval(`()=>{
		const texts=[];
		document.querySelectorAll('.cc-app').forEach(app=>{
			texts.push(app.innerText);
			app.querySelectorAll('[title],[aria-label],[placeholder],option').forEach(n=>['title','aria-label','placeholder','label'].forEach(a=>texts.push(n.getAttribute(a)||'')));
		});
		const out=new Set();
		texts.forEach(text=>(String(text).match(/cyd_creator\.[A-Za-z0-9_.]+/g)||[]).forEach(k=>out.add(k)));
		return JSON.stringify([...out]);
	}`).Str()
	if found != "[]" {
		s.t.Fatalf("%s: untranslated keys %s", step, found)
	}
}

// noPageKeys fails when Delete, Backspace, Enter or F2 reached the document, where the desktop
// deletes, opens or renames its selected icons (sdk-events-bootstrap.js handleDesktopKeydown).
func (s *cydDesignerSmoke) noPageKeys(step string) {
	s.t.Helper()
	if got := s.page.MustEval(`()=>JSON.stringify(cydFixture.pageKeys)`).Str(); got != "[]" {
		s.t.Fatalf("%s: keys reached the desktop: %s", step, got)
	}
}

// client returns the viewport position of a device pixel of the CYD screen.
func (s *cydDesignerSmoke) client(x, y float64) (float64, float64) {
	p := s.page.MustEval(fmt.Sprintf(`()=>cydFixture.client(%v,%v)`, x, y))
	return p.Get("x").Num(), p.Get("y").Num()
}

// drag presses at (x1,y1), moves in steps to (x2,y2) and, before releasing, evaluates during
// (when not empty) so a test can look at guides or the drop target while the pointer is down.
func (s *cydDesignerSmoke) drag(x1, y1, x2, y2 float64, during string) bool {
	s.t.Helper()
	m := s.page.Mouse
	m.MustMoveTo(x1, y1)
	m.MustDown(proto.InputMouseButtonLeft)
	for i := 1; i <= 8; i++ {
		m.MustMoveTo(x1+(x2-x1)*float64(i)/8, y1+(y2-y1)*float64(i)/8)
	}
	ok := true
	if during != "" {
		ok = s.page.MustEval(during).Bool()
	}
	m.MustUp(proto.InputMouseButtonLeft)
	return ok
}

func (s *cydDesignerSmoke) center(selector string) (float64, float64) {
	s.t.Helper()
	r := s.page.MustEval(fmt.Sprintf(`()=>{const r=document.querySelector(%q).getBoundingClientRect();return{x:r.x+r.width/2,y:r.y+r.height/2}}`, selector))
	return r.Get("x").Num(), r.Get("y").Num()
}

// focusCenter is the centre of the control with data-cc-focus key, scrolled into its panel's view.
func (s *cydDesignerSmoke) focusCenter(key string) (float64, float64) {
	s.t.Helper()
	r := s.page.MustEval(`k=>{const el=cydFixture.byFocus(k);if(!el)return{x:-1,y:-1};el.scrollIntoView({block:'nearest',inline:'nearest'});const r=el.getBoundingClientRect();return{x:r.x+r.width/2,y:r.y+r.height/2,w:r.width}}`, key)
	if r.Get("x").Num() < 0 || r.Get("w").Num() == 0 {
		s.t.Fatalf("control %s is missing or not shown", key)
	}
	return r.Get("x").Num(), r.Get("y").Num()
}

// press clicks the control with data-cc-focus key with the real mouse, holding the button for hold.
func (s *cydDesignerSmoke) press(key string, hold time.Duration) {
	s.t.Helper()
	x, y := s.focusCenter(key)
	m := s.page.Mouse
	m.MustMoveTo(x, y)
	m.MustDown(proto.InputMouseButtonLeft)
	time.Sleep(hold)
	m.MustUp(proto.InputMouseButtonLeft)
}

func (s *cydDesignerSmoke) focus(key string) {
	s.t.Helper()
	if !s.page.MustEval(`k=>{const el=cydFixture.byFocus(k);if(!el)return false;el.focus();return document.activeElement===el}`, key).Bool() {
		s.t.Fatalf("cannot focus %s", key)
	}
}

func (s *cydDesignerSmoke) keys(keys ...input.Key) {
	s.t.Helper()
	for _, k := range keys {
		s.page.Keyboard.MustType(k)
	}
}

func (s *cydDesignerSmoke) insertText(text string) {
	s.t.Helper()
	if err := (proto.InputInsertText{Text: text}).Call(s.page); err != nil {
		s.t.Fatal(err)
	}
}

func startCYDDesignerSmoke(t *testing.T, motion, lang, theme string) *cydDesignerSmoke {
	t.Helper()
	var sun bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 32, 32))
	for y := 0; y < 32; y++ {
		for x := 0; x < 32; x++ {
			if (x-16)*(x-16)+(y-16)*(y-16) < 140 {
				img.Set(x, y, color.RGBA{R: 255, G: 196, B: 0, A: 255})
			}
		}
	}
	if err := png.Encode(&sun, img); err != nil {
		t.Fatal(err)
	}
	var scripts strings.Builder
	for _, name := range cydDesignerScripts {
		scripts.WriteString(`<script src="/js/desktop/apps/` + name + `"></script>`)
	}
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(Content)))
	mux.HandleFunc("/fixture-asset/", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(sun.Bytes())
	})
	// The page holds the shell's workbench columns (plan 4a: .cc-left with screens and palette,
	// .cc-center-design, .cc-right .cc-panel) inside .cc-app, with the desktop theme tokens.
	mux.HandleFunc("/fixture", func(w http.ResponseWriter, r *http.Request) {
		theme := r.URL.Query().Get("theme")
		if theme == "" {
			theme = "standard"
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = fmt.Fprintf(w, `<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<link rel="stylesheet" href="/fonts/fonts.css"><link rel="stylesheet" href="/shared-variables.css"><link rel="stylesheet" href="/css/desktop-shell.bundle.css"><link rel="stylesheet" href="/css/desktop-app-cyd-creator.css">
<style>html,body{margin:0;height:100%%;overflow:hidden}</style></head>
<body class="desktop-body" data-theme="%s" data-fruity-mode="light"><div class="cc-app"><div class="cc-workbench">
<aside class="cc-left"><div class="cc-left-screens" id="screens"></div><div class="cc-left-palette" id="palette"></div></aside>
<section class="cc-center"><div class="cc-center-design" id="canvas"></div></section>
<aside class="cc-right"><div class="cc-panel" id="inspector"></div></aside></div></div>
<script src="/js/desktop/apps/cyd-creator-core.js"></script>%s<script src="/testdata/cyd-creator-designer-fixture.js"></script></body></html>`, theme, scripts.String())
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	page := newSmokeBrowser(t).MustPage().Timeout(120 * time.Second)
	t.Cleanup(func() { _ = page.Close() })
	page.MustSetViewport(1400, 900, 1, false)
	// The Windows animation setting reaches headless Chrome: both motion modes are emulated.
	if err := (proto.EmulationSetEmulatedMedia{Features: []*proto.EmulationMediaFeature{{Name: "prefers-reduced-motion", Value: motion}}}).Call(page); err != nil {
		t.Fatal(err)
	}
	page.MustNavigate(server.URL + "/fixture?theme=" + theme + "&lang=" + lang)
	page.MustWaitLoad()
	dir := os.Getenv("AURAGO_BROWSER_ARTIFACT_DIR")
	if dir == "" {
		dir = filepath.Join("..", "reports", "cyd-creator")
	}
	dir = filepath.Join(dir, "designer", motion)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	s := &cydDesignerSmoke{t: t, page: page, dir: dir}
	s.wait(`()=>window.cydFixture && (cydFixture.ready || cydFixture.errors.length>0)`)
	s.noErrors("load")
	if !page.MustEval(`m=>matchMedia('(prefers-reduced-motion: '+m+')').matches`, motion).Bool() {
		t.Fatalf("motion emulation %s not active", motion)
	}
	// Pointer maths below assume zoom 2 and snapping to 8 px.
	page.MustEval(`()=>{cydFixture.state.setZoom(2);cydFixture.state.setSnap(8);cydFixture.state.setGrid(true)}`)
	s.wait(`()=>document.querySelector('.cc-zoom-label').textContent==='2×'`)
	return s
}

// TestDesktopCYDCreatorDesignerBrowser drives the designer modules with a fake shell instance in
// both prefers-reduced-motion modes: palette drag, move with guides, handle resize, multi-select +
// align, undo/redo, reparenting, tab pages, theme switch, inspector prop/event edits, screens,
// issues, reveal, upload and the lock (plan 4b Task 14), plus the canvas, inspector, screens and
// keyboard contracts found in the reviews of Tasks 6, 9 and 10 and the German palette at its
// narrowest column.
func TestDesktopCYDCreatorDesignerBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	for _, motion := range []string{"no-preference", "reduce"} {
		t.Run(motion, func(t *testing.T) {
			s := startCYDDesignerSmoke(t, motion, "en", "standard")
			s.render()
			s.canvasContracts()
			s.planScenarios()
			s.inspectorContracts()
			s.screensContracts()
			s.keyboardStaysInDesigner()
			s.noErrors("standard")
			s.germanFruity()
		})
	}
}

// render: device frame, visible widgets (the LED sits on the hidden tab page), palette, layers.
func (s *cydDesignerSmoke) render() {
	s.t.Helper()
	s.wait(`()=>document.querySelector('.cc-device-frame .cc-screen[data-cc-screen-id="s_home"]')!==null`)
	s.wait(`()=>document.querySelectorAll('.cc-screen [data-cc-widget-id]').length===6 && !document.querySelector('[data-cc-widget-id="w_led"]')`)
	s.wait(`()=>document.querySelectorAll('[data-cc-palette]').length===26 && document.querySelectorAll('.cc-layer-select').length===7`)
	s.wait(`()=>document.querySelector('.cc-device-label').textContent.includes('320 × 240')`)
	s.noErrors("render")
	s.noRawKeys("render")
	s.shot("designer-night")
}

// canvasContracts (Task 6 review): the orientation label, the statusbar from the settings, untrusted
// colours and boxes, read-only and the fit guard of a hidden viewport.
func (s *cydDesignerSmoke) canvasContracts() {
	s.t.Helper()
	page := s.page
	// The device label names the orientation with plan 4a's cyd_creator.orientation.* strings.
	s.wait(`()=>cydFixture.t.has('cyd_creator.orientation.landscape') && document.querySelector('.cc-device-size').textContent==='320 × 240 · '+cydFixture.t('cyd_creator.orientation.landscape')`)

	// The statusbar shows its title (else the screen title), the clock and Wi-Fi as the settings say.
	const bar = `document.querySelector('[data-cc-widget-id="w_status"]')`
	s.wait(`()=>{const b=` + bar + `;return b.textContent.includes('Wetter') && b.textContent.includes('12:34') && !!b.querySelector('.cc-glyph')}`)
	page.MustElement(`[data-cc-field="statusbar_clock"]`).MustClick()
	s.wait(`()=>{const b=` + bar + `;return cydFixture.store.get().settings.statusbar_clock===false && !b.textContent.includes('12:34') && !!b.querySelector('.cc-glyph')}`)
	page.MustEval(`()=>{const s=document.querySelector('[data-cc-field="wifi"]');s.value='off';s.dispatchEvent(new Event('change',{bubbles:true}))}`)
	s.wait(`()=>{const b=` + bar + `;return cydFixture.store.get().settings.wifi==='off' && !b.querySelector('.cc-glyph')}`)
	page.MustEval(`()=>{const s=document.querySelector('[data-cc-field="wifi"]');s.value='optional';s.dispatchEvent(new Event('change',{bubbles:true}))}`)
	page.MustElement(`[data-cc-field="statusbar_clock"]`).MustClick()
	s.wait(`()=>{const b=` + bar + `;const st=cydFixture.store.get().settings;return st.wifi==='optional' && st.statusbar_clock===true && b.textContent.includes('12:34') && !!b.querySelector('.cc-glyph')}`)
	page.MustElement(`[data-cc-layer-id="w_status"]`).MustClick()
	page.MustElement(`[data-cc-field="prop:title"]`).MustSelectAllText()
	s.keys(input.Backspace)
	s.wait(`()=>{const b=` + bar + `;return !('title' in (cydFixture.widget('w_status').props||{})) && b.textContent.includes('Start') && !b.textContent.includes('Wetter')}`)
	page.MustElement(`[data-cc-field="prop:title"]`).MustInput("Wetter")
	s.wait(`()=>cydFixture.widget('w_status').props.title==='Wetter' && ` + bar + `.textContent.includes('Wetter')`)

	// Layout values are untrusted: a url() colour never reaches CSS, boxes are finite and clamped.
	page.MustEval(`()=>cydFixture.remote(p=>{p.screens[0].style={bg_color:'url(/fixture-asset/a_sun)'};p.screens[0].widgets[1].style={bg_color:'url(/fixture-asset/a_sun)'};p.screens[0].widgets.push({id:'w_huge',type:'label',name:'lbl_huge',x:1e9,y:-1e9,w:1e9,h:-5},{id:'w_str',type:'label',name:'lbl_str',x:'12',y:null,w:'40',h:20.6})})`)
	s.wait(`()=>{const scr=document.querySelector('.cc-screen');const night=cydFixture.inst.caps.themes.find(t=>t.id==='night').tokens.bg;const probe=document.createElement('i');probe.style.color=night;document.body.append(probe);const want=getComputedStyle(probe).color;probe.remove();const imgs=[scr,...scr.querySelectorAll('*')].map(n=>getComputedStyle(n).backgroundImage).filter(v=>v.includes('url('));return getComputedStyle(scr).backgroundColor===want && imgs.length===0 && !scr.getAttribute('style').includes('url(')}`)
	s.wait(`()=>{const st=id=>document.querySelector('[data-cc-widget-id="'+id+'"]').style;const h=st('w_huge'),k=st('w_str');return h.left==='4096px' && h.top==='-4096px' && h.width==='2048px' && h.height==='1px' && k.left==='0px' && k.top==='0px' && k.width==='1px' && k.height==='21px'}`)
	page.MustEval(`()=>cydFixture.remote(p=>{p.screens[0].style={};delete p.screens[0].widgets[1].style;p.screens[0].widgets=p.screens[0].widgets.filter(w=>w.id!=='w_huge' && w.id!=='w_str')})`)
	s.wait(`()=>!document.querySelector('[data-cc-widget-id="w_huge"]') && document.querySelectorAll('.cc-layer-select').length===7`)
	s.noErrors("untrusted layout")

	// Read-only (plan 4a setReadonly → readonly:changed): handles, tools, palette, screen tools and
	// fields are disabled, keys change nothing; switching back enables them again.
	page.MustEval(`()=>cydFixture.state.select(['w_btn'])`)
	s.wait(`()=>document.querySelectorAll('.cc-handle').length===8 && !document.querySelector('[data-cc-cmd="align-left"]').disabled`)
	page.MustEval(`()=>cydFixture.setReadonly(true)`)
	const disabled = `()=>document.querySelectorAll('.cc-handle').length===0 && document.querySelector('.cc-canvas').classList.contains('is-readonly') && ['align-left','lock','hide','delete','z-front'].every(c=>document.querySelector('.cc-canvas-bar [data-cc-cmd="'+c+'"]').disabled) && [...document.querySelectorAll('[data-cc-palette]')].every(b=>b.disabled) && document.querySelector('[data-cc-cmd="screen-add"]').disabled && [...document.querySelectorAll('.cc-inspector fieldset, .cc-screens fieldset')].every(f=>f.disabled) && [...document.querySelectorAll('.cc-layer-flag')].every(b=>b.disabled)`
	s.wait(disabled)
	before := page.MustEval(`()=>JSON.stringify(cydFixture.store.get())`).Str()
	page.MustEval(`()=>document.querySelector('.cc-canvas').focus()`)
	s.keys(input.Delete, input.ArrowRight)
	bx, by := s.client(40, 60)
	s.drag(bx, by, bx+30, by+30, "")
	if after := page.MustEval(`()=>JSON.stringify(cydFixture.store.get())`).Str(); after != before {
		s.t.Fatal("the read-only designer changed the layout")
	}
	s.shot("designer-readonly")
	page.MustEval(`()=>cydFixture.setReadonly(false)`)
	s.wait(`()=>document.querySelectorAll('.cc-handle').length===8 && !document.querySelector('.cc-canvas').classList.contains('is-readonly') && [...document.querySelectorAll('[data-cc-palette]')].every(b=>!b.disabled) && !document.querySelector('.cc-inspector fieldset').disabled`)
	page.MustEval(`()=>cydFixture.state.select([])`)

	// A hidden canvas (Code mode) keeps its zoom: fitting a 0×0 viewport would fall back to zoom 1.
	page.MustElement(`[data-cc-cmd="zoom-fit"]`).MustClick()
	s.wait(`()=>cydFixture.state.zoomAuto===true && cydFixture.state.zoom>1`)
	zoom := page.MustEval(`()=>cydFixture.state.zoom`).Int()
	page.MustEval(`()=>{document.getElementById('canvas').style.display='none';cydFixture.bus.emit('mode:changed','code')}`)
	page.MustEval(`()=>new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(()=>setTimeout(r,100))))`)
	if got := page.MustEval(`()=>cydFixture.state.zoom`).Int(); got != zoom {
		s.t.Fatalf("hidden canvas changed the zoom from %d to %d", zoom, got)
	}
	page.MustEval(`()=>{document.getElementById('canvas').style.display='';cydFixture.bus.emit('mode:changed','design')}`)
	s.wait(fmt.Sprintf(`()=>cydFixture.state.zoom===%d && document.querySelector('.cc-screen-host').getBoundingClientRect().width===320*%d`, zoom, zoom))
	page.MustEval(`()=>cydFixture.state.setZoom(2)`)
	s.wait(`()=>document.querySelector('.cc-zoom-label').textContent==='2×' && !cydFixture.state.zoomAuto`)
	s.noErrors("canvas contracts")
}

// planScenarios are the steps of plan 4b Task 14.
func (s *cydDesignerSmoke) planScenarios() {
	s.t.Helper()
	t := s.t
	page := s.page

	// Palette drag: a label dropped at device (232, 92) lands centred there, snapped to 8 px.
	px, py := s.center(`[data-cc-palette="label"]`)
	tx, ty := s.client(232, 92)
	if !s.drag(px, py, tx, ty, `()=>!!document.querySelector('.cc-drag-ghost')`) {
		t.Fatal("palette drag showed no ghost")
	}
	s.wait(`()=>{const w=cydFixture.store.get().screens[0].widgets.find(w=>w.name==='lbl_2');return !!w && w.x===176 && w.y===80 && cydFixture.state.selection[0]===w.id}`)
	s.wait(`()=>!document.querySelector('.cc-drag-ghost')`)

	// Keyboard "add at centre": Enter on a palette entry adds the widget in the middle of the screen.
	page.MustEval(`()=>document.querySelector('[data-cc-palette="led"]').focus()`)
	page.Keyboard.MustType(input.Enter)
	s.wait(`()=>cydFixture.store.get().screens[0].widgets.some(w=>w.type==='led' && w.x===152 && w.y===112)`)
	page.MustEval(`()=>cydFixture.store.undo()`)
	s.wait(`()=>!cydFixture.store.get().screens[0].widgets.some(w=>w.type==='led')`)

	// Move with guides (grid off): lbl_1 dragged 6 px up snaps its top to btn_1's top (y 40).
	page.MustEval(`()=>cydFixture.state.setSnap(0)`)
	lx, ly := s.client(220, 60)
	if !s.drag(lx, ly, lx, ly-12, `()=>document.querySelectorAll('.cc-guide-h').length>0`) {
		t.Fatal("no horizontal alignment guide during the move")
	}
	s.wait(`()=>{const w=cydFixture.widget('w_lbl');return w.x===160 && w.y===40}`)
	s.wait(`()=>document.querySelectorAll('.cc-guide').length===0`)
	page.MustEval(`()=>cydFixture.state.setSnap(8)`)

	// Resize via the south-east handle: +13/+5 device px snap to the 8 px grid (w 136, h 48).
	page.MustEval(`()=>cydFixture.state.select(['w_btn'])`)
	s.wait(`()=>document.querySelectorAll('.cc-handle').length===8`)
	hx, hy := s.center(`.cc-handle-se`)
	s.drag(hx, hy, hx+26, hy+10, "")
	s.wait(`()=>{const w=cydFixture.widget('w_btn');return w.w===136 && w.h===48 && w.x===16 && w.y===40}`)

	// Multi-select (click + Shift+click) and align bottom: lbl_1 moves to y 64.
	bx, by := s.client(40, 60)
	page.Mouse.MustMoveTo(bx, by).MustClick(proto.InputMouseButtonLeft)
	lx, ly = s.client(220, 52)
	if err := page.Keyboard.Press(input.ShiftLeft); err != nil {
		t.Fatal(err)
	}
	page.Mouse.MustMoveTo(lx, ly).MustClick(proto.InputMouseButtonLeft)
	if err := page.Keyboard.Release(input.ShiftLeft); err != nil {
		t.Fatal(err)
	}
	s.wait(`()=>cydFixture.state.selection.join()==='w_btn,w_lbl' && document.querySelectorAll('.cc-sel').length===2`)
	page.MustElement(`[data-cc-cmd="align-bottom"]`).MustClick()
	s.wait(`()=>cydFixture.widget('w_lbl').y===64 && cydFixture.widget('w_btn').y===40`)

	// Undo/redo on the focused canvas (Ctrl+Z, Ctrl+Y, Ctrl+Shift+Z).
	page.MustEval(`()=>document.querySelector('.cc-canvas').focus()`)
	page.KeyActions().Press(input.ControlLeft).Type('z').MustDo()
	s.wait(`()=>cydFixture.widget('w_lbl').y===40`)
	page.KeyActions().Press(input.ControlLeft).Type('y').MustDo()
	s.wait(`()=>cydFixture.widget('w_lbl').y===64`)
	page.KeyActions().Press(input.ControlLeft).Type('z').MustDo()
	page.KeyActions().Press(input.ControlLeft).Press(input.ShiftLeft).Type('z').MustDo()
	s.wait(`()=>cydFixture.widget('w_lbl').y===64`)

	// Keyboard nudge: Arrow = 1 px, Shift+Arrow = 8 px, one undo step per burst.
	page.MustEval(`()=>cydFixture.state.select(['w_lbl'])`)
	page.Keyboard.MustType(input.ArrowRight)
	page.KeyActions().Press(input.ShiftLeft).Type(input.ArrowDown).MustDo()
	s.wait(`()=>{const w=cydFixture.widget('w_lbl');return w.x===161 && w.y===72}`)
	page.MustEval(`()=>cydFixture.store.undo()`)
	s.wait(`()=>{const w=cydFixture.widget('w_lbl');return w.x===160 && w.y===64}`)

	// Reparent: lbl_2 dragged into the panel becomes its child (coordinates relative to the content box).
	sx, sy := s.client(236, 92)
	ex, ey := s.client(88, 164)
	if !s.drag(sx, sy, ex, ey, `()=>!document.querySelector('.cc-drop').hidden`) {
		t.Fatal("no drop target while dragging over the panel")
	}
	s.wait(`()=>{const w=cydFixture.store.get().screens[0].widgets.find(w=>w.id==='w_pnl').children.find(c=>c.name==='lbl_2');if(!w)return false;const a=cydFixture.abs(w.id);return a.x>=16 && a.y>=104 && a.x+a.w<=200 && w.x===a.x-16 && w.y===a.y-104}`)

	// Tab pages: clicking the tab header "B" shows the LED page.
	bx, by = s.center(`[data-cc-widget-id="w_tabs"] [data-cc-tab="1"]`)
	page.Mouse.MustMoveTo(bx, by).MustClick(proto.InputMouseButtonLeft)
	s.wait(`()=>!!document.querySelector('[data-cc-widget-id="w_led"]') && !document.querySelector('[data-cc-widget-id="w_sw"]')`)

	// Theme switch: no selection shows project settings; Terminal re-renders the preview.
	page.MustEval(`()=>document.querySelector('.cc-canvas').focus()`)
	page.Keyboard.MustType(input.Escape)
	s.wait(`()=>cydFixture.state.selection.length===0 && !!document.querySelector('[data-cc-theme="terminal"]')`)
	page.MustElement(`[data-cc-theme="terminal"]`).MustClick()
	s.wait(`()=>{const th=cydFixture.inst.caps.themes.find(t=>t.id==='terminal');const probe=document.createElement('i');probe.style.color=th.tokens.bg;document.body.append(probe);const want=getComputedStyle(probe).color;probe.remove();return cydFixture.store.get().theme==='terminal' && getComputedStyle(document.querySelector('.cc-screen')).backgroundColor===want && document.querySelector('[data-cc-widget-id="w_btn"]').textContent.includes('[')}`)
	page.Mouse.MustMoveTo(5, 5)
	s.wait(`()=>cydFixture.state.previewTheme===''`)
	s.shot("designer-terminal")

	// Inspector: edit a prop (text) and an event (goto_screen, then logic + hook).
	page.MustElement(`[data-cc-layer-id="w_btn"]`).MustClick()
	s.wait(`()=>!!document.querySelector('[data-cc-field="prop:text"]')`)
	page.MustElement(`[data-cc-field="prop:text"]`).MustSelectAllText().MustInput("Start")
	s.wait(`()=>cydFixture.widget('w_btn').props.text==='Start' && document.querySelector('[data-cc-widget-id="w_btn"]').textContent.includes('Start')`)
	page.MustElement(`[data-cc-focus="event:add"]`).MustClick()
	s.wait(`()=>(cydFixture.widget('w_btn').events||[]).length===1 && !!document.querySelector('[data-cc-field="event:0:action"]')`)
	page.MustEval(`()=>{const s=document.querySelector('[data-cc-field="event:0:action"]');s.value='goto_screen';s.dispatchEvent(new Event('change',{bubbles:true}))}`)
	s.wait(`()=>{const e=cydFixture.widget('w_btn').events[0];return e.on==='clicked' && e.action.type==='goto_screen' && e.action.screen==='s_details' && !!document.querySelector('[data-cc-field="event:0:screen"]')}`)
	page.MustEval(`()=>{const s=document.querySelector('[data-cc-field="event:0:action"]');s.value='logic';s.dispatchEvent(new Event('change',{bubbles:true}))}`)
	s.wait(`()=>!!document.querySelector('[data-cc-field="event:0:hint"]')`)
	page.MustElement(`[data-cc-field="event:0:hint"]`).MustInput("lädt das Wetter neu")
	s.wait(`()=>cydFixture.widget('w_btn').events[0].action.hint==='lädt das Wetter neu'`)
	page.MustElement(`[data-cc-focus="event:0:hook"]`).MustClick()
	s.wait(`()=>cydFixture.hooks.length===1 && cydFixture.hooks[0].name==='btn_1' && cydFixture.hooks[0].event==='clicked'`)

	// Name validation: an invalid name is not stored and shows the issue text.
	page.MustElement(`[data-cc-field="name"]`).MustSelectAllText().MustInput("Bad Name")
	s.wait(`()=>cydFixture.widget('w_btn').name==='btn_1' && document.querySelector('[data-cc-field="name"]').getAttribute('aria-invalid')==='true'`)
	page.MustElement(`[data-cc-field="name"]`).MustSelectAllText().MustInput("btn_go")
	s.wait(`()=>cydFixture.widget('w_btn').name==='btn_go'`)

	// Issues from the shell mark the widget and appear in the inspector.
	page.MustEval(`()=>cydFixture.bus.emit('layout:issues',[{severity:'error',code:'NAME_DUPLICATE',widget:'w_btn',message:'duplicate'}])`)
	s.wait(`()=>!!document.querySelector('.cc-issue-dot.is-error') && document.querySelector('.cc-issues').textContent.length>0 && document.querySelector('[data-cc-widget-id="w_btn"]').title.length>0`)

	// Screens: add, inline rename (Enter), delete with the shell confirm.
	page.MustElement(`[data-cc-cmd="screen-add"]`).MustClick()
	s.wait(`()=>cydFixture.store.get().screens.length===3 && cydFixture.state.screenId===cydFixture.store.get().screens[2].id`)
	page.MustEval(`()=>document.querySelector('.cc-screen-row.is-active [data-cc-cmd="screen-rename"]').click()`)
	s.wait(`()=>document.activeElement && document.activeElement.classList.contains('cc-screen-rename')`)
	page.MustElement(`.cc-screen-rename`).MustSelectAllText().MustInput("settings")
	page.Keyboard.MustType(input.Enter)
	s.wait(`()=>cydFixture.store.get().screens[2].name==='settings'`)
	page.MustEval(`()=>document.querySelector('.cc-screen-row.is-active [data-cc-cmd="screen-delete"]').click()`)
	s.wait(`()=>cydFixture.store.get().screens.length===2 && cydFixture.confirms.length===1 && cydFixture.confirms[0].titleKey==='cyd_creator.designer.screen_delete_title'`)
	// The Wi-Fi setup screen is a locked system entry with a themed preview.
	page.MustElement(`[data-cc-screen-id="__wifi_setup__"] .cc-screen-open`).MustClick()
	s.wait(`()=>!!document.querySelector('.cc-screen .cc-system-screen') && document.querySelector('.cc-canvas').classList.contains('is-system')`)

	// Reveal (agent/problems): switches screen, selects and flashes the widget.
	page.MustEval(`()=>cydFixture.bus.emit('designer:reveal',{widget:'gauge_1'})`)
	s.wait(`()=>cydFixture.state.screenId==='s_details' && cydFixture.state.selection[0]==='w_gauge' && !!document.querySelector('.cc-flash')`)
	s.wait(`()=>document.querySelector('[data-cc-widget-id="w_img"] img')?.getAttribute('src')==='/fixture-asset/a_sun'`)

	// Image upload: an SVG is rasterised to a PNG at the widget size on a real canvas, uploaded
	// (the window's store is replaced remotely with the asset listed), set and sized.
	page.MustEval(`()=>cydFixture.state.select(['w_img'])`)
	s.wait(`()=>!!document.querySelector('[data-cc-field="prop:asset:upload"]')`)
	page.MustEval(`()=>{const input=document.querySelector('[data-cc-field="prop:asset:upload"]');const dt=new DataTransfer();dt.items.add(new File(['<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16"><rect width="16" height="16" fill="red"/></svg>'],'Sun Icon.svg',{type:'image/svg+xml'}));input.files=dt.files;input.dispatchEvent(new Event('change',{bubbles:true}))}`)
	s.wait(`()=>{const w=cydFixture.widget('w_img');return cydFixture.uploads[0]==='Sun Icon.png:image/png' && w.props.asset==='a_up' && w.w===48 && w.h===40 && cydFixture.store.get().assets.filter(a=>a.id==='a_up').length===1 && cydFixture.events.some(e=>e[0]==='project:changed' && e[1].what==='assets')}`)
	raster := page.MustEval(`async()=>{const f=cydFixture.files[0];const head=[...new Uint8Array(await f.slice(0,8).arrayBuffer())];const b=await createImageBitmap(f);const c=document.createElement('canvas');c.width=b.width;c.height=b.height;const g=c.getContext('2d');g.drawImage(b,0,0);const px=[...g.getImageData(16,16,1,1).data];return {w:b.width,h:b.height,head:head.join(),px:px.join()}}`)
	if raster.Get("w").Int() != 32 || raster.Get("h").Int() != 32 || raster.Get("head").Str() != "137,80,78,71,13,10,26,10" || raster.Get("px").Str() != "255,0,0,255" {
		t.Fatalf("SVG upload was not rasterised to a red 32×32 PNG: %s", raster.JSON("", ""))
	}
	// The upload's remote replace keeps the undo history: one undo takes the size back.
	page.MustEval(`()=>cydFixture.store.undo()`)
	s.wait(`()=>{const w=cydFixture.widget('w_img');return w.props.asset==='a_sun' && w.w===32}`)
	page.MustEval(`()=>cydFixture.store.redo()`)
	s.wait(`()=>cydFixture.widget('w_img').props.asset==='a_up'`)

	// Duplicate (Ctrl+D), copy/paste (Ctrl+C, Ctrl+V), z-order, distribute, lock from the toolbar.
	page.MustEval(`()=>{cydFixture.state.select(['w_gauge']);document.querySelector('.cc-canvas').focus()}`)
	page.KeyActions().Press(input.ControlLeft).Type('d').MustDo()
	s.wait(`()=>{const ws=cydFixture.store.get().screens[1].widgets;const d=ws.find(w=>w.name==='gauge_2');return ws.length===3 && !!d && d.x===98 && d.y===48 && cydFixture.state.selection[0]===d.id}`)
	page.KeyActions().Press(input.ControlLeft).Type('c').MustDo()
	page.KeyActions().Press(input.ControlLeft).Type('v').MustDo()
	s.wait(`()=>{const ws=cydFixture.store.get().screens[1].widgets;const p=ws.find(w=>w.name==='gauge_3');return ws.length===4 && !!p && p.x===106 && p.y===56}`)
	page.MustEval(`()=>cydFixture.state.select(['w_img'])`)
	page.MustElement(`[data-cc-cmd="z-front"]`).MustClick()
	s.wait(`()=>{const ws=cydFixture.store.get().screens[1].widgets;return ws[ws.length-1].id==='w_img'}`)
	page.MustEval(`()=>{const ws=cydFixture.store.get().screens[1].widgets;cydFixture.state.select(ws.filter(w=>w.type==='gauge').map(w=>w.id))}`)
	page.MustElement(`[data-cc-cmd="distribute-h"]`).MustClick()
	s.wait(`()=>{const g=cydFixture.store.get().screens[1].widgets.filter(w=>w.type==='gauge').sort((a,b)=>a.x-b.x);return Math.abs((g[1].x-g[0].x)-(g[2].x-g[1].x))<=1}`)
	page.MustEval(`()=>cydFixture.state.select(['w_img'])`)
	page.MustElement(`[data-cc-cmd="lock"]`).MustClick()
	s.wait(`()=>cydFixture.widget('w_img').locked===true && document.querySelector('[data-cc-cmd="layer-lock"][data-cc-id="w_img"]').getAttribute('aria-pressed')==='true'`)
	page.MustElement(`[data-cc-cmd="layer-lock"][data-cc-id="w_img"]`).MustClick()
	s.wait(`()=>!cydFixture.widget('w_img').locked`)

	// Layers list: arrow keys move the focus between rows (roving tab stop).
	page.MustEval(`()=>document.querySelector('.cc-layer-select').focus()`)
	page.Keyboard.MustType(input.ArrowDown)
	s.wait(`()=>document.activeElement===document.querySelectorAll('.cc-layer-select')[1]`)

	// Project settings: orientation swaps the device size, Wi-Fi off hides the system screen,
	// hovering a theme previews it without changing the layout.
	page.MustEval(`()=>{document.querySelector('.cc-canvas').focus();cydFixture.state.select([])}`)
	s.wait(`()=>!!document.querySelector('[data-cc-field="orientation"]')`)
	page.MustEval(`()=>{const s=document.querySelector('[data-cc-field="orientation"]');s.value='portrait';s.dispatchEvent(new Event('change',{bubbles:true}))}`)
	s.wait(`()=>{const r=document.querySelector('.cc-screen-host').getBoundingClientRect();return cydFixture.store.get().orientation==='portrait' && Math.round(r.width)===240*cydFixture.state.zoom && Math.round(r.height)===320*cydFixture.state.zoom && document.querySelector('.cc-device-size').textContent==='240 × 320 · '+cydFixture.t('cyd_creator.orientation.portrait')}`)
	page.MustEval(`()=>{const s=document.querySelector('[data-cc-field="orientation"]');s.value='landscape';s.dispatchEvent(new Event('change',{bubbles:true}))}`)
	page.MustEval(`()=>{const s=document.querySelector('[data-cc-field="wifi"]');s.value='off';s.dispatchEvent(new Event('change',{bubbles:true}))}`)
	s.wait(`()=>cydFixture.store.get().settings.wifi==='off' && !document.querySelector('[data-cc-screen-id="__wifi_setup__"]') && cydFixture.store.get().settings.aurago_link===false`)
	page.MustEval(`()=>{const s=document.querySelector('[data-cc-field="wifi"]');s.value='optional';s.dispatchEvent(new Event('change',{bubbles:true}))}`)
	s.wait(`()=>!!document.querySelector('[data-cc-screen-id="__wifi_setup__"]')`)
	page.MustEval(`()=>document.querySelector('[data-cc-theme="clear"]').dispatchEvent(new PointerEvent('pointerenter'))`)
	s.wait(`()=>{const th=cydFixture.inst.caps.themes.find(t=>t.id==='clear');const probe=document.createElement('i');probe.style.color=th.tokens.bg;document.body.append(probe);const want=getComputedStyle(probe).color;probe.remove();return cydFixture.store.get().theme==='terminal' && getComputedStyle(document.querySelector('.cc-screen')).backgroundColor===want}`)
	page.MustEval(`()=>document.querySelector('.cc-theme-grid').dispatchEvent(new PointerEvent('pointerleave'))`)
	s.wait(`()=>cydFixture.state.previewTheme===''`)

	// Duplicate screen: the copy follows the original and becomes active; one undo removes it.
	page.MustEval(`()=>document.querySelector('[data-cc-screen-id="s_details"] [data-cc-cmd="screen-duplicate"]').click()`)
	s.wait(`()=>{const ss=cydFixture.store.get().screens;return ss.length===3 && ss[2].name==='details_1' && ss[2].widgets.length===4 && cydFixture.state.screenId===ss[2].id}`)
	page.MustEval(`()=>cydFixture.store.undo()`)
	s.wait(`()=>cydFixture.store.get().screens.length===2`)

	// Locked by an agent job (lock:changed(true, jobId)): no edits from palette, canvas, keyboard
	// or inspector.
	page.MustEval(`()=>{cydFixture.state.setScreen('s_details');cydFixture.setLocked(true);cydFixture.state.select(['w_gauge'])}`)
	s.wait(`()=>[...document.querySelectorAll('[data-cc-palette]')].every(b=>b.disabled) && document.querySelector('.cc-inspector fieldset').disabled && document.querySelectorAll('.cc-handle').length===0 && cydFixture.events.some(e=>e[0]==='lock:changed' && e[1]===true && e[2]==='ccj_fixture')`)
	before := page.MustEval(`()=>JSON.stringify(cydFixture.store.get())`).Str()
	gx, gy := s.client(160, 110)
	s.drag(gx, gy, gx+40, gy+40, "")
	page.MustEval(`()=>document.querySelector('.cc-canvas').focus()`)
	page.Keyboard.MustType(input.Delete)
	page.KeyActions().Press(input.ControlLeft).Type('z').MustDo()
	if after := page.MustEval(`()=>JSON.stringify(cydFixture.store.get())`).Str(); after != before {
		t.Fatal("the locked designer changed the layout")
	}
	s.shot("designer-locked")
	page.MustEval(`()=>cydFixture.setLocked(false)`)
	s.wait(`()=>[...document.querySelectorAll('[data-cc-palette]')].every(b=>!b.disabled)`)
	s.noErrors("plan scenarios")
}

// inspectorContracts (Task 9 review): stored values in select, textarea and range; refused entries,
// focus and caret across a remote replace; read-only; clamping; colours; issue lists; IME input.
func (s *cydDesignerSmoke) inspectorContracts() {
	s.t.Helper()
	t := s.t
	page := s.page
	page.MustEval(`()=>{cydFixture.state.setScreen('s_home');cydFixture.state.select([])}`)
	s.wait(`()=>{const f=cydFixture.byFocus;return !!f('backlight') && f('orientation').value==='landscape' && f('language').value==='de' && f('wifi').value==='optional' && f('backlight').value==='80' && document.querySelector('.cc-range-row output').textContent==='80 %' && f('timezone').value==='CET-1CEST,M3.5.0,M10.5.0/3'}`)
	page.MustEval(`()=>cydFixture.state.select(['w_lbl'])`)
	s.wait(`()=>{const f=cydFixture.byFocus;return !!f('prop:size') && f('prop:size').value==='20' && f('prop:align').value==='left' && f('prop:text').tagName==='TEXTAREA' && f('prop:text').value==='22,5 °C'}`)

	// A refused name stays in its field with focus and caret across a store replace from elsewhere.
	page.MustElement(`[data-cc-field="name"]`).MustSelectAllText().MustInput("Bad Name")
	s.wait(`()=>{const n=cydFixture.byFocus('name');return document.activeElement===n && n.getAttribute('aria-invalid')==='true' && cydFixture.widget('w_lbl').name==='lbl_1'}`)
	page.MustEval(`()=>{const n=cydFixture.byFocus('name');n.setSelectionRange(3,3);window.__ccName=n;cydFixture.remote(p=>{p.screens[0].widgets.find(w=>w.id==='w_btn').y+=8})}`)
	s.wait(`()=>{const n=cydFixture.byFocus('name');return n!==window.__ccName && n.value==='Bad Name' && document.activeElement===n && n.selectionStart===3 && n.selectionEnd===3 && n.getAttribute('aria-invalid')==='true' && cydFixture.widget('w_btn').y===48}`)
	// A new issue list replaces only the list: the field (and what it holds) stays.
	page.MustEval(`()=>{window.__ccName=cydFixture.byFocus('name');cydFixture.bus.emit('layout:issues',[{severity:'warning',code:'PARTLY_OUT_OF_BOUNDS',widget:'w_lbl',message:'partly outside'}])}`)
	s.wait(`()=>{const n=cydFixture.byFocus('name');return n===window.__ccName && n.value==='Bad Name' && document.activeElement===n && document.querySelectorAll('.cc-inspector .cc-issue').length===1}`)
	page.MustElement(`[data-cc-field="name"]`).MustSelectAllText().MustInput("lbl_1")
	s.wait(`()=>cydFixture.byFocus('name').getAttribute('aria-invalid')==='false' && cydFixture.widget('w_lbl').name==='lbl_1'`)

	// A number is clamped while typing and shows the stored value when the field is left.
	s.press("geo:w", 30*time.Millisecond)
	page.MustEval(`()=>cydFixture.byFocus('geo:w').select()`)
	s.insertText("5000")
	s.wait(`()=>cydFixture.widget('w_lbl').w===2048 && cydFixture.byFocus('geo:w').value==='5000'`)
	s.keys(input.Tab)
	s.wait(`()=>cydFixture.byFocus('geo:w').value==='2048' && document.activeElement===cydFixture.byFocus('geo:h')`)
	page.MustEval(`()=>cydFixture.store.undo()`)
	s.wait(`()=>cydFixture.widget('w_lbl').w===120 && cydFixture.byFocus('geo:w').value==='120'`)

	// Read-only: the fieldset disables every field inside it (:disabled) and a focused field hands
	// the focus to the panel, never to the page.
	s.focus("geo:x")
	page.MustEval(`()=>cydFixture.setReadonly(true)`)
	s.wait(`()=>{const f=cydFixture.byFocus;return document.querySelector('.cc-inspector fieldset').disabled && ['geo:x','prop:text','prop:size','style:bg_color','name'].every(k=>f(k).matches(':disabled')) && document.activeElement===document.querySelector('.cc-inspector')}`)
	page.MustEval(`()=>cydFixture.setReadonly(false)`)
	s.wait(`()=>!document.querySelector('.cc-inspector fieldset').disabled && !cydFixture.byFocus('geo:x').matches(':disabled')`)

	// Colours: the picker holds lower case, the layout gets upper case #RRGGBB; "Use theme" resets.
	s.wait(`()=>/^#[0-9a-f]{6}$/.test(cydFixture.byFocus('style:bg_color').value)`)
	page.MustEval(`()=>{const c=cydFixture.byFocus('style:bg_color');c.value='#a1b2c3';c.dispatchEvent(new Event('input',{bubbles:true}))}`)
	s.wait(`()=>{const c=cydFixture.byFocus('style:bg_color');return cydFixture.widget('w_lbl').style.bg_color==='#A1B2C3' && c.value==='#a1b2c3' && c.parentElement.querySelector('.cc-color-value').textContent==='#A1B2C3'}`)
	page.MustEval(`()=>cydFixture.byFocus('style:bg_color:reset').click()`)
	s.wait(`()=>!(cydFixture.widget('w_lbl').style||{}).bg_color && cydFixture.byFocus('style:bg_color:reset').getAttribute('aria-pressed')==='true'`)

	// An input method composes across a store replace from elsewhere: the field is not rebuilt
	// while it composes; the committed text is stored and the field keeps the focus.
	page.MustEval(`()=>cydFixture.state.select(['w_btn'])`)
	s.wait(`()=>!!cydFixture.byFocus('prop:text')`)
	start := page.MustEval(`()=>cydFixture.widget('w_btn').props.text`).Str()
	s.press("prop:text", 30*time.Millisecond)
	page.MustEval(`()=>{const f=cydFixture.byFocus('prop:text');f.setSelectionRange(f.value.length,f.value.length);window.__ccText=f;window.__ccComp=[];['compositionstart','compositionend'].forEach(type=>f.addEventListener(type,()=>window.__ccComp.push(type)))}`)
	if err := (proto.InputImeSetComposition{Text: "にほ", SelectionStart: 2, SelectionEnd: 2}).Call(page); err != nil {
		t.Fatal(err)
	}
	s.wait(`()=>window.__ccComp.join()==='compositionstart' && cydFixture.byFocus('prop:text').value.endsWith('にほ')`)
	page.MustEval(`()=>cydFixture.remote(p=>{p.screens[0].widgets.find(w=>w.id==='w_lbl').x+=8})`)
	s.wait(`()=>cydFixture.widget('w_lbl').x===168 && cydFixture.byFocus('prop:text')===window.__ccText && document.activeElement===window.__ccText && window.__ccComp.join()==='compositionstart'`)
	s.insertText("日本")
	s.wait(fmt.Sprintf(`()=>{const f=cydFixture.byFocus('prop:text');return window.__ccComp.join()==='compositionstart,compositionend' && cydFixture.widget('w_btn').props.text===%q && f.value===%q && document.activeElement===f && f.selectionStart===f.value.length && document.querySelector('[data-cc-widget-id="w_btn"]').textContent.includes('日本')}`, start+"日本", start+"日本"))
	s.noErrors("inspector contracts")
}

// screensContracts (Task 10 review): Tab out of an edited screen title stays in the panel, and a
// press (held 100 ms) on another screen, a tool or a layer while renaming both stores the name and
// does what the press was for.
func (s *cydDesignerSmoke) screensContracts() {
	s.t.Helper()
	page := s.page
	page.MustEval(`()=>{cydFixture.state.setScreen('s_home');cydFixture.state.select([])}`)
	s.press("screen-title", 30*time.Millisecond)
	s.keys(input.End)
	s.insertText(" Haus")
	s.keys(input.Tab)
	s.wait(`()=>{const a=document.activeElement;return cydFixture.store.get().screens[0].title==='Start Haus' && a && a.getAttribute('data-cc-focus')==='screen-transition' && document.querySelector('.cc-screens').contains(a) && document.querySelector('[data-cc-title="s_home"]').textContent==='Start Haus'}`)

	rename := func(id, name string) {
		s.t.Helper()
		s.press("screen-rename:"+id, 30*time.Millisecond)
		s.wait(fmt.Sprintf(`()=>document.activeElement===cydFixture.byFocus('rename:%s')`, id))
		page.MustEval(`()=>document.activeElement.select()`)
		s.insertText(name)
	}
	// Another screen's entry: the rename is stored and that screen opens.
	rename("s_home", "start")
	s.press("open:s_details", 100*time.Millisecond)
	s.wait(`()=>cydFixture.store.get().screens[0].name==='start' && cydFixture.state.screenId==='s_details' && document.activeElement===cydFixture.byFocus('open:s_details') && !document.querySelector('[data-cc-rename]')`)
	// A layer: stored and the widget selected.
	rename("s_details", "detail")
	s.press("layer:w_gauge", 100*time.Millisecond)
	s.wait(`()=>cydFixture.store.get().screens[1].name==='detail' && cydFixture.state.selection.join()==='w_gauge' && document.activeElement===cydFixture.byFocus('layer:w_gauge')`)
	// The rename tool of another screen (shown on hover): the first name is stored, the second rename starts.
	rename("s_details", "details")
	x, y := s.focusCenter("open:s_home")
	page.Mouse.MustMoveTo(x, y)
	s.press("screen-rename:s_home", 100*time.Millisecond)
	s.wait(`()=>cydFixture.store.get().screens[1].name==='details' && document.activeElement===cydFixture.byFocus('rename:s_home')`)
	s.keys(input.Escape)
	s.wait(`()=>!document.querySelector('[data-cc-rename]') && cydFixture.store.get().screens[0].name==='start' && document.activeElement===cydFixture.byFocus('open:s_home')`)
	// The add tool: stored and a screen added. Pointer and focus leave the s_home row first (its
	// tools show while it is hovered or focused; rowsStayPut checks what that does to the rows).
	x, y = s.center(`.cc-canvas-viewport`)
	page.Mouse.MustMoveTo(x, y)
	page.MustEval(`()=>{cydFixture.state.setScreen('s_details');document.querySelector('.cc-canvas').focus()}`)
	rename("s_details", "details_x")
	s.press("screen-add", 100*time.Millisecond)
	s.wait(`()=>{const ss=cydFixture.store.get().screens;return ss[1].name==='details_x' && ss.length===3 && cydFixture.state.screenId===ss[2].id}`)
	// The click ran first, then the rename was committed: undo takes back the rename, then the add.
	page.MustEval(`()=>cydFixture.store.undo()`)
	s.wait(`()=>{const ss=cydFixture.store.get().screens;return ss.length===3 && ss[1].name==='details'}`)
	page.MustEval(`()=>cydFixture.store.undo()`)
	s.wait(`()=>cydFixture.store.get().screens.length===2`)
	s.noErrors("screens contracts")
	s.rowsStayPut()
}

// rowsStayPut: a screen entry that shows its tools (hovered or focused) must not move the entries
// below it. In the left column the tools wrap onto a second line; when showing them changes the
// row height, a press that takes the pointer or the focus out of that row shifts every row below
// between press and release, and the click lands on another element or is lost.
func (s *cydDesignerSmoke) rowsStayPut() {
	s.t.Helper()
	page := s.page
	const top = `()=>new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(()=>r(Math.round(document.querySelector('.cc-screen-row[data-cc-screen-id="s_details"]').getBoundingClientRect().top)))))`
	page.MustEval(`()=>{cydFixture.state.setScreen('s_details');document.querySelector('.cc-canvas').focus()}`)
	x, y := s.center(`.cc-canvas-viewport`)
	page.Mouse.MustMoveTo(x, y)
	rest := page.MustEval(top).Int()
	x, y = s.focusCenter("open:s_home")
	page.Mouse.MustMoveTo(x, y)
	hovered := page.MustEval(top).Int()
	x, y = s.center(`.cc-canvas-viewport`)
	page.Mouse.MustMoveTo(x, y)
	s.focus("open:s_home")
	focused := page.MustEval(top).Int()
	page.MustEval(`()=>document.querySelector('.cc-canvas').focus()`)
	if hovered != rest || focused != rest {
		s.t.Errorf("showing the tools of screen entry s_home moves the entry below it: top %d at rest, %d with s_home hovered, %d with s_home focused", rest, hovered, focused)
	}
}

// keyboardStaysInDesigner: Delete, Enter and F2 on the canvas, a palette entry, a layer, a screen
// entry and inspector buttons never reach the document, where the desktop deletes, opens or
// renames its selected icons.
func (s *cydDesignerSmoke) keyboardStaysInDesigner() {
	s.t.Helper()
	page := s.page
	page.MustEval(`()=>{cydFixture.state.setScreen('s_details');cydFixture.state.select(['w_gauge']);cydFixture.pageKeys.length=0;document.querySelector('.cc-canvas').focus()}`)
	s.keys(input.Enter, input.F2, input.Delete)
	s.wait(`()=>!cydFixture.widget('w_gauge')`)
	page.KeyActions().Press(input.ControlLeft).Type('z').MustDo()
	s.wait(`()=>!!cydFixture.widget('w_gauge')`)
	s.noPageKeys("canvas")

	page.MustEval(`()=>document.querySelector('[data-cc-palette="label"]').focus()`)
	s.keys(input.Enter)
	s.wait(`()=>{const sel=cydFixture.state.selection;return sel.length===1 && cydFixture.widget(sel[0]).type==='label'}`)
	s.keys(input.F2, input.Delete)
	s.wait(`()=>!cydFixture.store.get().screens[1].widgets.some(w=>w.type==='label')`)
	s.noPageKeys("palette")

	page.MustEval(`()=>document.querySelector('[data-cc-layer-id="w_gauge"]').focus()`)
	s.keys(input.Enter, input.F2)
	s.wait(`()=>cydFixture.state.selection.join()==='w_gauge'`)
	s.keys(input.Delete)
	s.wait(`()=>!cydFixture.widget('w_gauge')`)
	page.MustEval(`()=>cydFixture.store.undo()`)
	s.noPageKeys("layers")

	s.focus("open:s_home")
	s.keys(input.Enter, input.Delete, input.F2)
	s.wait(`()=>cydFixture.state.screenId==='s_home' && document.activeElement===cydFixture.byFocus('rename:s_home')`)
	s.keys(input.Escape)
	s.noPageKeys("screens")

	page.MustEval(`()=>cydFixture.state.select([])`)
	s.wait(`()=>!!cydFixture.byFocus('theme:terminal')`)
	s.focus("theme:terminal")
	s.keys(input.Enter, input.F2, input.Delete)
	page.MustEval(`()=>cydFixture.state.select(['w_btn'])`)
	s.wait(`()=>!!cydFixture.byFocus('event:0:remove')`)
	s.focus("event:0:hook")
	s.keys(input.Enter, input.F2, input.Delete)
	s.focus("prop:text")
	s.keys(input.Enter)
	s.noPageKeys("inspector")
}

// germanFruity: the Fruity light desktop theme in German renders without horizontal overflow, and
// the palette at its narrowest entries (150 px) keeps whole words.
func (s *cydDesignerSmoke) germanFruity() {
	s.t.Helper()
	page := s.page
	page.MustNavigate(strings.Replace(page.MustInfo().URL, "theme=standard&lang=en", "theme=fruity&lang=de", 1))
	page.MustWaitLoad()
	s.wait(`()=>window.cydFixture && cydFixture.ready`)
	s.wait(`()=>document.querySelector('[data-cc-palette="button"]').textContent.includes('Schaltfläche')`)
	if page.MustEval(`()=>['#palette','#screens','#inspector'].some(sel=>{const el=document.querySelector(sel);return el.scrollWidth>el.clientWidth+1})`).Bool() {
		s.t.Fatal("a designer panel overflows horizontally")
	}
	s.noErrors("fruity")
	s.noRawKeys("fruity")
	s.shot("designer-fruity-de")

	// The palette column narrowed until its entries reach their 150 px minimum.
	page.MustEval(`()=>{const host=document.getElementById('palette');const bar=host.offsetWidth-host.clientWidth;document.querySelector('.cc-workbench').style.gridTemplateColumns=(170+bar)+'px minmax(0,1fr) minmax(270px,330px)'}`)
	s.wait(`()=>Math.abs(document.querySelector('.cc-palette-item').getBoundingClientRect().width-150)<0.6`)
	broken := page.MustEval(`()=>{
		const out=[];
		document.querySelectorAll('.cc-palette-name').forEach(n=>{
			const text=n.textContent;
			const r=document.createRange();r.selectNodeContents(n);
			const tops=new Set([...r.getClientRects()].filter(x=>x.width>0).map(x=>Math.round(x.top)));
			// a single-word name on more than one line was broken inside the word
			if(!/\s/.test(text.trim()) && tops.size>1) out.push(text);
		});
		return JSON.stringify(out);
	}`).Str()
	s.shot("designer-palette-150-de")
	if broken != "[]" {
		s.t.Errorf("German palette names broken inside a word at 150 px: %s", broken)
	}
	if page.MustEval(`()=>{const el=document.getElementById('palette');return el.scrollWidth>el.clientWidth+1}`).Bool() {
		s.t.Error("the palette overflows horizontally at its narrowest entries")
	}
	s.noErrors("fruity narrow")
}
