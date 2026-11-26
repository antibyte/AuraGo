package ui

import (
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/input"
	"github.com/go-rod/rod/lib/proto"
	xdraw "golang.org/x/image/draw"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
)

// hdDesigner drives one HomeDash window in the desktop shell against the M4 fixture (contract C18); shared by the designer and gate tests.
type hdDesigner struct {
	t       *testing.T
	page    *rod.Page
	fixture *homeDashFixture
	words   map[string]string
	winSel  string // the window that capture and probe look at (hdWinSel unless a state tags another window with data-hd-shot)
	id      string // the dashboard opened by the last freshDoc
}

// hdWinSel is the default target window of capture and probe.
const hdWinSel = `.vd-window[data-app-id="homedash"]`

// hdDesignerSetupJS installs the words, an error log, an export capture (blob text of every application/json object URL) and a no-op
// click for download anchors, so exports never touch the disk.
const hdDesignerSetupJS = `(w)=>{Object.assign(window.fixtureWords,JSON.parse(w));window.hdErrs=[];
	addEventListener('error',e=>hdErrs.push(String(e.message)));addEventListener('unhandledrejection',e=>hdErrs.push(String(e.reason&&e.reason.message||e.reason)));
	window.__exports=[];const make=URL.createObjectURL.bind(URL);URL.createObjectURL=b=>{if(b&&b.type==='application/json')b.text().then(x=>__exports.push(x));return make(b);};
	const click=HTMLElement.prototype.click;HTMLAnchorElement.prototype.click=function(){if(this.hasAttribute('download'))return;click.call(this);};}`

// hdSetupExtraJS is evaluated after the setup script; empty in the committed tests (a scratch file can set it to try a CSS or JS fix before it is made).
var hdSetupExtraJS string

func newHDDesigner(t *testing.T, lang string) *hdDesigner {
	t.Helper()
	fixture := newHomeDashFixture(t)
	fixture.enableM3(t)
	page := desktopAuditBrowserWithAPI(t, map[string]http.Handler{"/api/desktop/homedash/": fixture}).CancelTimeout().Timeout(40 * time.Minute)
	t.Cleanup(fixture.shutdown)
	raw := readDesktopAssetText(t, "lang/homedash/"+lang+".json")
	words := map[string]string{}
	if err := json.Unmarshal([]byte(raw), &words); err != nil {
		t.Fatal(err)
	}
	page.MustEval(hdDesignerSetupJS, raw)
	if hdSetupExtraJS != "" {
		page.MustEval(hdSetupExtraJS)
	}
	page.MustEval(`(l)=>{document.documentElement.lang=l;}`, lang)
	return &hdDesigner{t: t, page: page, fixture: fixture, words: words, winSel: hdWinSel}
}

func (h *hdDesigner) str(js string, args ...any) string  { return h.page.MustEval(js, args...).Str() }
func (h *hdDesigner) yes(js string, args ...any) bool    { return h.page.MustEval(js, args...).Bool() }
func (h *hdDesigner) num(js string, args ...any) float64 { return h.page.MustEval(js, args...).Num() }
func (h *hdDesigner) word(key string) string             { return h.words[key] }
func (h *hdDesigner) fill(key string, vars map[string]string) string {
	s := h.words[key]
	for k, v := range vars {
		s = strings.ReplaceAll(s, "{{"+k+"}}", v)
	}
	return s
}
func (h *hdDesigner) diag() string {
	return h.str(`()=>JSON.stringify({errs:window.hdErrs,mode:(document.querySelector('.hd-root')||{dataset:{}}).dataset.hdMode,save:(document.querySelector('.hd-save')||{dataset:{}}).dataset.phase,
		live:(document.querySelector('.hd-root .hd-live')||{}).textContent,sel:[...document.querySelectorAll('.hd-widget[aria-selected="true"]')].map(x=>x.dataset.hdWid),
		sheet:(document.querySelector('.hd-root > .hd-sheet-layer:not([hidden]) .hd-sheet-title')||{}).textContent,
		app:(document.querySelector('.hdx-app')||{dataset:{}}).dataset.hdxView,appState:(document.querySelector('.hdx-state:not([hidden])')||{}).textContent,
		views:[...document.querySelectorAll('.hdx-view')].map(v=>v.hidden+':'+v.querySelectorAll('.hd-page').length),wins:document.querySelectorAll('.vd-window[data-app-id="homedash"]').length})`)
}
func (h *hdDesigner) until(within time.Duration, what, js string, args ...any) {
	h.t.Helper()
	deadline := time.Now().Add(within)
	for !h.page.MustEval(js, args...).Bool() {
		if time.Now().After(deadline) {
			h.t.Fatalf("timed out after %v waiting for %s\n%s", within, what, h.diag())
		}
		time.Sleep(20 * time.Millisecond)
	}
}
func hdFrame(id string) string {
	return `.hd-root[data-hd-mode="design"] .hd-page > [data-hd-wid="` + id + `"]`
}
func (h *hdDesigner) center(sel string) (float64, float64) {
	h.t.Helper()
	r := h.page.MustEval(`(s)=>{const e=document.querySelector(s);if(!e)return null;e.scrollIntoView({block:'nearest',inline:'nearest'});const r=e.getBoundingClientRect();return [r.left+r.width/2,r.top+r.height/2];}`, sel)
	if r.Nil() {
		h.t.Fatalf("no element %s\n%s", sel, h.diag())
	}
	return r.Arr()[0].Num(), r.Arr()[1].Num()
}

// drag presses at (x0,y0), moves in `steps` frames to (x1,y1), runs hold (a screenshot or a check) and releases unless hold returns false.
func (h *hdDesigner) drag(x0, y0, x1, y1 float64, steps int, hold func() bool) {
	m := h.page.Mouse
	m.MustMoveTo(x0, y0)
	m.MustDown(proto.InputMouseButtonLeft)
	for i := 1; i <= steps; i++ {
		f := float64(i) / float64(steps)
		m.MustMoveTo(x0+(x1-x0)*f, y0+(y1-y0)*f)
		time.Sleep(16 * time.Millisecond)
	}
	if hold == nil || hold() {
		m.MustUp(proto.InputMouseButtonLeft)
	}
}

// openDashboard opens the dashboard in a new window and waits for its page. The desktop's HomeDash realm outlives a window: a list answer that was
// fetched before the seed and arrives while the next window opens does not contain the new dashboard, so the engine reconciles to "deleted" and the
// window lands on the home screen (the page never shows). That is the state the previous run of a long gate session sometimes left behind; the open
// recognises the home screen at once (no 20 s wait) and opens again, at most three times, after the stale answer has had its turn.
func (h *hdDesigner) openDashboard(id string) {
	h.t.Helper()
	const pageJS = `()=>!!document.querySelector('.hdx-view:not([hidden]) .hd-page')`
	for attempt := 1; ; attempt++ {
		h.page.MustEval(`(id)=>auditTest.openApp('homedash',{dashboard_id:id})`, id)
		deadline := time.Now().Add(20 * time.Second)
		for !h.page.MustEval(pageJS).Bool() {
			if h.yes(`()=>{const a=document.querySelector('.hdx-app');return !!a&&a.dataset.hdxView==='home';}`) {
				break
			}
			if time.Now().After(deadline) {
				h.t.Fatalf("timed out after 20s waiting for dashboard %s\n%s", id, h.diag())
			}
			time.Sleep(20 * time.Millisecond)
		}
		if h.page.MustEval(pageJS).Bool() {
			return
		}
		if attempt == 3 {
			h.t.Fatalf("dashboard %s opened on the home screen three times\n%s", id, h.diag())
		}
		h.t.Logf("dashboard %s opened on the home screen (notice %q), opening again", id, h.str(`()=>(document.querySelector('.hdx-notice')||{}).textContent||''`))
		h.hardReset()
	}
}
func (h *hdDesigner) enterEdit() {
	h.mouseClick(`.hdx-edit`)
	h.until(10*time.Second, "design mode", `()=>!!document.querySelector('.hd-root[data-hd-mode="design"] > .hd-editbar')&&!!document.querySelector('.hd-pal-card')`)
}

// ---- pointer and keyboard like a user (the gate states drive the real UI through these) ----

func (h *hdDesigner) mouseClickAt(x, y float64) {
	h.page.Mouse.MustMoveTo(x, y)
	h.page.Mouse.MustClick(proto.InputMouseButtonLeft)
}
func (h *hdDesigner) mouseClick(sel string) { x, y := h.center(sel); h.mouseClickAt(x, y) }
func (h *hdDesigner) mouseRightAt(x, y float64) {
	h.page.Mouse.MustMoveTo(x, y)
	h.page.Mouse.MustClick(proto.InputMouseButtonRight)
}
func (h *hdDesigner) mouseRight(sel string) { x, y := h.center(sel); h.mouseRightAt(x, y) }

// textCenter finds the visible element of `scope` whose text starts with `text` (the shortest one when several do) and returns its centre.
func (h *hdDesigner) textCenter(scope, text string) (float64, float64) {
	h.t.Helper()
	r := h.page.MustEval(`(s,tx)=>{const l=[...document.querySelectorAll(s)].filter(e=>e.innerText.trim().startsWith(tx)&&!e.closest('[hidden]')).sort((a,b)=>a.innerText.length-b.innerText.length);
		const e=l[0];if(!e)return null;e.scrollIntoView({block:'nearest',inline:'nearest'});const r=e.getBoundingClientRect();return [r.left+r.width/2,r.top+r.height/2];}`, scope, text)
	if r.Nil() {
		h.t.Fatalf("no element %s with text %q\n%s", scope, text, h.diag())
	}
	return r.Arr()[0].Num(), r.Arr()[1].Num()
}
func (h *hdDesigner) mouseText(scope, text string) {
	x, y := h.textCenter(scope, text)
	h.mouseClickAt(x, y)
}

// rect returns left, top, width, height of the first match without scrolling.
func (h *hdDesigner) rect(sel string) [4]float64 {
	h.t.Helper()
	r := h.page.MustEval(`(s)=>{const e=document.querySelector(s);if(!e)return null;const r=e.getBoundingClientRect();return [r.left,r.top,r.width,r.height];}`, sel)
	if r.Nil() {
		h.t.Fatalf("no element %s\n%s", sel, h.diag())
	}
	a := r.Arr()
	return [4]float64{a[0].Num(), a[1].Num(), a[2].Num(), a[3].Num()}
}

// emptyPoint finds a point of the open page that no widget covers, as far to the bottom right of the visible stage as possible.
func (h *hdDesigner) emptyPoint() (float64, float64) {
	h.t.Helper()
	r := h.page.MustEval(`()=>{const st=document.querySelector('.hd-root[data-hd-mode="design"] .hd-stage');if(!st)return null;const sr=st.getBoundingClientRect();
		for(let y=sr.bottom-30;y>sr.top+20;y-=20)for(let x=sr.right-40;x>sr.left+20;x-=20){const e=document.elementFromPoint(x,y);
			if(e&&e.closest('.hd-page')&&!e.closest('.hd-widget')&&!e.closest('.hd-handle')&&!e.closest('button,input,select'))return [x,y];}return null;}`)
	if r.Nil() {
		h.t.Fatalf("no empty point on the page\n%s", h.diag())
	}
	return r.Arr()[0].Num(), r.Arr()[1].Num()
}

// pause lets a frame or two and the transitions settle (motion is reduced in the fixture, so this is short).
func (h *hdDesigner) pause(ms int) {
	h.page.MustEval(`()=>new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(()=>r(true))))`)
	time.Sleep(time.Duration(ms) * time.Millisecond)
}

// park moves the pointer onto the title bar of the target window so that no hover state shows in a shot.
func (h *hdDesigner) park() {
	r := h.rect(h.winSel)
	h.page.Mouse.MustMoveTo(r[0]+r[2]/2, r[1]+14)
}

// hardReset ends whatever the last state left: a held button, a desktop menu, every HomeDash window (closed through the shell with the
// close guard off, so a dirty or conflicting lane cannot hold it) and the fixture's scripted answers and delay.
func (h *hdDesigner) hardReset() {
	_ = h.page.Mouse.Up(proto.InputMouseButtonLeft, 1)
	h.page.MustEval(`async()=>{document.querySelectorAll('.vd-context-menu').forEach(m=>m.remove());auditTest.state.contextMenu=null;
		document.querySelectorAll('.vd-window-menu.open').forEach(m=>m.classList.remove('open'));
		for(const [id,w] of [...auditTest.state.windows]){if(w.appId==='homedash'){w.beforeClose=null;await auditTest.closeWindow(id);}}
		document.querySelectorAll('[data-hd-shot]').forEach(e=>e.removeAttribute('data-hd-shot'));return true;}`)
	h.until(10*time.Second, "all HomeDash windows closed", `()=>!document.querySelector('.vd-window[data-app-id="homedash"]')`)
	f := h.fixture
	f.mu.Lock()
	m := f.m4Locked()
	m.answers, m.drops, m.delay = nil, 0, 0
	f.mu.Unlock()
	h.winSel = hdWinSel
}

// freshDoc seeds doc, opens it in a new window (every other HomeDash window is closed first) and, with edit, enters design mode by the Edit button.
func (h *hdDesigner) freshDoc(doc map[string]any, edit bool) string {
	h.t.Helper()
	h.hardReset()
	h.id = h.fixture.seedDashboard(doc)
	h.openDashboard(h.id)
	if edit {
		h.enterEdit()
	}
	h.pause(200)
	return h.id
}

// selectOne selects a widget the way a user does: a click on its frame.
func (h *hdDesigner) selectOne(id string) {
	h.t.Helper()
	h.mouseClick(hdFrame(id))
	h.until(5*time.Second, "selection of "+id, `(id)=>{const f=document.querySelector('.hd-root[data-hd-mode="design"] .hd-page > [data-hd-wid="'+id+'"]');return !!f&&f.getAttribute('aria-selected')==='true';}`, id)
}
func (h *hdDesigner) selectedCount() int {
	return int(h.num(`()=>document.querySelectorAll('.hd-root[data-hd-mode="design"] .hd-page > .hd-widget[aria-selected="true"]').length`))
}

// withKey holds a modifier key while f runs.
func (h *hdDesigner) withKey(k input.Key, f func()) {
	if err := h.page.Keyboard.Press(k); err != nil {
		h.t.Fatal(err)
	}
	defer func() { _ = h.page.Keyboard.Release(k) }()
	f()
}

// resizeWindow sets the size of the target window (test-only inline style; the shell does the same while a user drags an edge).
func (h *hdDesigner) resizeWindow(w, ht int) {
	h.page.MustEval(`(sel,w,h)=>{const e=document.querySelector(sel);e.style.width=w+'px';e.style.height=h+'px';}`, h.winSel, w, ht)
	h.pause(150)
}

// closeSheet presses the named action button of the open engine sheet (E.sheet confirm buttons or form buttons).
func (h *hdDesigner) press(action string) {
	h.t.Helper()
	h.page.MustEval(`(a)=>{const b=document.querySelector('.hd-root .hd-sheet-layer:not([hidden]) [data-action="'+a+'"]');if(!b)throw new Error("no sheet button "+a);b.click();}`, action)
}
func (h *hdDesigner) waitPuts(after int, what string) homeDashPut {
	h.t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		if log := h.fixture.putLog(); len(log) > after {
			return log[len(log)-1]
		}
		if time.Now().After(deadline) {
			h.t.Fatalf("no PUT for %s\n%s", what, h.diag())
		}
		time.Sleep(25 * time.Millisecond)
	}
}

// hdThemeVariant is one of the 16 desktop theme variants (14 themes plus the dark schemes of Fruity and Minimal).
type hdThemeVariant struct{ Name, Theme, Setting, Mode, Scheme string }

func hdThemeVariants(t *testing.T) []hdThemeVariant {
	t.Helper()
	var out []hdThemeVariant
	for _, th := range parseDesktopThemeRegistry(t) {
		if th.SchemeSetting != "" {
			for _, m := range []string{"light", "dark"} {
				out = append(out, hdThemeVariant{th.ID + "-" + m, th.ID, th.SchemeSetting, m, m})
			}
			continue
		}
		out = append(out, hdThemeVariant{th.ID, th.ID, "", "", th.Scheme})
	}
	if len(out) != 16 {
		t.Fatalf("%d theme variants, want 16", len(out))
	}
	return out
}
func (h *hdDesigner) theme(v hdThemeVariant) {
	h.page.MustEval(`(theme,setting,mode,scheme)=>{const s=auditTest.state.bootstrap.settings;s['appearance.theme']=theme;if(setting)s[setting]=mode;document.documentElement.dataset.theme=scheme;auditTest.applyDesktopSettings();}`, v.Theme, v.Setting, v.Mode, v.Scheme)
	h.page.MustWait(`(theme,scheme)=>document.body.dataset.theme===theme&&document.body.dataset.scheme===scheme`, v.Theme, v.Scheme)
	h.page.MustEval(`()=>new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(()=>r(true))))`)
}
func (h *hdDesigner) capture(dir, name string) string {
	h.t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		h.t.Fatal(err)
	}
	path := filepath.Join(dir, name+".png")
	if err := os.WriteFile(path, h.page.MustElement(h.winSel).MustScreenshot(), 0o644); err != nil {
		h.t.Fatal(err)
	}
	return path
}

type hdTile struct{ Label, File string }

// writeHDSheet lays tiles out in `cols` columns, each drawn with its Label (the callers pass "NN state"), scaled by `scale` (2x shots at 0.5 keep sheets comparable).
func writeHDSheet(t *testing.T, out string, tiles []hdTile, cols int, scale float64) {
	t.Helper()
	if len(tiles) == 0 {
		return
	}
	imgs, tw, th := make([]image.Image, len(tiles)), 0, 0
	for i, tile := range tiles {
		f, err := os.Open(tile.File)
		if err != nil {
			t.Fatal(err)
		}
		img, err := png.Decode(f)
		f.Close()
		if err != nil {
			t.Fatal(err)
		}
		imgs[i] = img
		b := img.Bounds()
		tw, th = max(tw, int(float64(b.Dx())*scale)), max(th, int(float64(b.Dy())*scale))
	}
	const label, pad = 20, 12
	rows := (len(tiles) + cols - 1) / cols
	sheet := image.NewRGBA(image.Rect(0, 0, pad+cols*(tw+pad), pad+rows*(th+label+pad)))
	xdraw.Draw(sheet, sheet.Bounds(), &image.Uniform{color.RGBA{42, 42, 46, 255}}, image.Point{}, xdraw.Src)
	for i, img := range imgs {
		x, y := pad+(i%cols)*(tw+pad), pad+(i/cols)*(th+label+pad)
		d := font.Drawer{Dst: sheet, Src: image.NewUniform(color.RGBA{238, 238, 238, 255}), Face: basicfont.Face7x13, Dot: fixed.P(x, y+14)}
		d.DrawString(tiles[i].Label)
		b := img.Bounds()
		xdraw.CatmullRom.Scale(sheet, image.Rect(x, y+label, x+int(float64(b.Dx())*scale), y+label+int(float64(b.Dy())*scale)), img, b, xdraw.Over, nil)
	}
	f, err := os.Create(out)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, sheet); err != nil {
		t.Fatal(err)
	}
}
