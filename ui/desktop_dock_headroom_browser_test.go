package ui

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"aurago/internal/desktop"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
)

// The dock's scroll region reaches 64 px above the visible bar, so lifted icons and their labels are
// not cut off by its horizontal scroll clip. That headroom is transparent: a click, hover or wheel at
// the bottom edge of a window just above the dock belongs to the window. The dock keeps its own
// pointer behaviour: hover lift and label, wheel scrolling anywhere on the bar, the overflow arrows
// and the collapsed dock that unfurls on hover.
func TestDesktopDockHeadroomPassesThePointerToWindowsBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	html := regexp.MustCompile(`(?s)<script\b[^>]*>.*?</script>`).ReplaceAllString(readDesktopAssetText(t, "desktop.html"), "")
	html = regexp.MustCompile(`\{\{[^}]*\}\}`).ReplaceAllString(html, "")
	html = strings.Replace(html, "</body>", `<script>window._auragoSharedInitialized=true;</script><script src="/js/shared/shared-core.js"></script><script src="/js/shared/lazy-assets.js"></script><script src="/js/desktop/core/module-loader.js"></script><script src="/dock-shell.js"></script><script src="/testdata/aurora-fixture.js"></script></body>`, 1)
	shell := readDesktopAssetText(t, "js/desktop/bundles/main.bundle.js")
	cut := strings.LastIndex(shell, "    ensureDesktopRadialMenuAnchor();")
	if cut < 0 {
		t.Fatal("shell startup seam missing")
	}
	shell = shell[:cut] + `window.aurora={state,loadIconManifest,applyDesktopSettings,renderIcons,renderStartApps,renderStartButtonIcon,wireShellChromeControls,bindViewportMetrics,handleDesktopKeydown,closeContextMenu,renderTaskbar,openApp,closeWindow,focusWindow,toggleMaximizeWindow,updateFruityDockOcclusion};})();`
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(Content)))
	mux.HandleFunc("/fixture", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, html) })
	mux.HandleFunc("/dock-shell.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/javascript")
		fmt.Fprint(w, shell)
	})
	mux.HandleFunc("/fixture-words", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, readDesktopAssetText(t, "lang/desktop/de.json"))
	})
	mux.HandleFunc("/fixture-apps", func(w http.ResponseWriter, r *http.Request) { json.NewEncoder(w).Encode(desktop.BuiltinApps()) })
	server := httptest.NewServer(mux)
	defer server.Close()
	// A mouse, not a touch screen: only hover-capable pointers get the hover headroom.
	browser := newSmokeBrowserWith(t, func(l *launcher.Launcher) {
		l.Set("blink-settings", "primaryHoverType=2,availableHoverTypes=2,primaryPointerType=4,availablePointerTypes=4")
	})
	page := browser.MustPage(server.URL + "/fixture").Timeout(90 * time.Second)
	defer page.Close()
	page.MustSetViewport(1366, 900, 1, false)
	page.MustWaitLoad()
	page.MustEval(`async()=>{
        await fixtureReady;
        window.frames2=()=>new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r)));
        window.describeHit=el=>el?(el.tagName.toLowerCase()+(el.id?'#'+el.id:'')+(el.classList.length?'.'+[...el.classList].join('.'):'')):'none';
        // Places the probe window: "floating" ends 3 px above the bar, "maximized" uses the shell's
        // own maximize, "overlap" reaches into the dock so the dock collapses.
        window.placeProbeWindow=async mode=>{
            let item=[...aurora.state.windows.values()].find(w=>w.appId==='files');
            if(!item){await fixtureOpen('files');item=[...aurora.state.windows.values()].find(w=>w.appId==='files');}
            aurora.focusWindow(item.id);
            if(!!item.maximized!==(mode==='maximized'))aurora.toggleMaximizeWindow(item.id);
            await frames2();
            const win=item.element,host=document.getElementById('vd-taskbar-apps');
            document.body.classList.remove('fruity-dock-collapsed');
            const bar=host.getBoundingClientRect();
            if(mode!=='maximized'){
                const width=Math.min(innerWidth-20,bar.width+80);
                win.style.width=width+'px';win.style.height='360px';
                let rect=win.getBoundingClientRect();
                win.style.left=(parseFloat(getComputedStyle(win).left)+(bar.left+bar.width/2-width/2)-rect.left)+'px';
                const bottom=mode==='overlap'?bar.top+30:bar.top-3;
                win.style.top=(parseFloat(getComputedStyle(win).top)+bottom-rect.bottom)+'px';
            }
            aurora.updateFruityDockOcclusion();
            await frames2();
            let probe=win.querySelector('[data-dock-headroom-probe]');
            if(!probe){
                probe=document.createElement('button');
                probe.type='button';probe.dataset.dockHeadroomProbe='';probe.textContent='Widget entfernen';
                probe.style.cssText='position:absolute;left:0;right:0;bottom:0;height:72px;z-index:2147483647;margin:0;border:0;border-radius:0;background:rgba(220,40,60,.28);color:#000;font:600 14px system-ui;cursor:pointer';
                probe.addEventListener('click',event=>window.probeClicks.push([event.clientX,event.clientY]));
                win.append(probe);
            }
            window.probeClicks=[];
            return JSON.stringify({window:win.getBoundingClientRect(),probe:probe.getBoundingClientRect(),bar,collapsed:document.body.classList.contains('fruity-dock-collapsed')});
        };
        // Every point of the probe that lies in the 64 px above the bar and within the dock's width
        // must hit the probe, not a dock layer.
        window.headroomHits=()=>{
            const probe=document.querySelector('[data-dock-headroom-probe]').getBoundingClientRect();
            const bar=document.getElementById('vd-taskbar-apps').getBoundingClientRect();
            const blocked=[];let checked=0;
            for(let y=Math.ceil(Math.max(probe.top,bar.top-64))+1;y<Math.min(probe.bottom,bar.top);y+=3){
                for(let x=Math.ceil(bar.left)+2;x<bar.right-2;x+=6){
                    if(x<probe.left||x>probe.right)continue;
                    checked++;
                    const hit=document.elementFromPoint(x,y);
                    if(!hit||!hit.closest('[data-dock-headroom-probe]'))blocked.push(x+','+y+' '+describeHit(hit));
                }
            }
            return JSON.stringify({checked,blocked:blocked.slice(0,6),blockedCount:blocked.length});
        };
    }`)

	type rect struct{ Left, Top, Right, Bottom, Width, Height, X, Y float64 }
	type placement struct {
		Window, Probe, Bar rect
		Collapsed          bool
	}
	place := func(mode string) placement {
		var p placement
		if err := json.Unmarshal([]byte(page.MustEval(`mode=>placeProbeWindow(mode)`, mode).Str()), &p); err != nil {
			t.Fatal(err)
		}
		return p
	}
	park := func() { page.Mouse.MustMoveTo(24, 140) }
	shot := func(name string) {
		if dir := os.Getenv("AURAGO_BROWSER_ARTIFACT_DIR"); dir != "" {
			if err := os.MkdirAll(dir, 0755); err != nil {
				t.Fatal(err)
			}
			page.MustScreenshot(filepath.Join(dir, "dock-headroom-"+name+".png"))
		}
	}

	// windowTakesTheHeadroom places the probe window and checks that its bottom edge, over the
	// dock's whole width, takes real clicks and every hit test in the 64 px above the bar.
	windowTakesTheHeadroom := func(label, mode string) {
		t.Helper()
		park()
		p := place(mode)
		if p.Collapsed {
			t.Fatalf("%s %s: a window above the dock must not collapse it: %+v", label, mode, p)
		}
		if mode == "maximized" && p.Window.Bottom > p.Bar.Top {
			t.Fatalf("%s: maximized window must end above the dock: window %+v, bar %+v", label, p.Window, p.Bar)
		}
		clicks := 0
		for _, fx := range []float64{0.12, 0.5, 0.88} {
			for _, y := range []float64{p.Probe.Bottom - 2, p.Bar.Top - 24, p.Bar.Top - 50} {
				if y <= p.Probe.Top || y >= p.Probe.Bottom || y >= p.Bar.Top {
					continue
				}
				x := p.Bar.Left + p.Bar.Width*fx
				page.Mouse.MustMoveTo(x, y).MustClick(proto.InputMouseButtonLeft)
				clicks++
				if got := page.MustEval(`()=>probeClicks.length`).Int(); got != clicks {
					shot(label + "-" + mode + "-click")
					t.Fatalf("%s %s: click at %.0f,%.0f above the dock did not reach the window (top element %s)", label, mode, x, y, page.MustEval(`(x,y)=>describeHit(document.elementFromPoint(x,y))`, x, y).Str())
				}
				if page.MustEval(`()=>!!document.querySelector('.vd-dock-button:hover')`).Bool() {
					t.Fatalf("%s %s: pointer at %.0f,%.0f over the window lifts a dock icon", label, mode, x, y)
				}
			}
		}
		if clicks < 6 {
			t.Fatalf("%s %s: only %d click points lay in the headroom", label, mode, clicks)
		}
		park()
		var hits struct {
			Checked, BlockedCount int
			Blocked               []string
		}
		if err := json.Unmarshal([]byte(page.MustEval(`()=>headroomHits()`).Str()), &hits); err != nil {
			t.Fatal(err)
		}
		if hits.Checked < 100 {
			t.Fatalf("%s %s: the probe must lie over the dock headroom (checked %d points): %+v", label, mode, hits.Checked, p)
		}
		if hits.BlockedCount > 0 {
			shot(label + "-" + mode + "-blocked")
			t.Fatalf("%s %s: %d of %d points above the dock do not reach the window, e.g. %v", label, mode, hits.BlockedCount, hits.Checked, hits.Blocked)
		}
		shot(label + "-" + mode)
	}

	page.MustEval(`()=>fixtureTheme('fruity-light')`)
	for _, theme := range []string{"fruity-light", "fruity-dark", "lollipop"} {
		page.MustEval(`theme=>{fixtureTheme(theme);aurora.renderTaskbar();}`, theme)
		windowTakesTheHeadroom(theme, "floating")
		windowTakesTheHeadroom(theme, "maximized")

		// Hover lift and label: the lifted icon keeps taking the pointer where it now sits.
		park()
		place("floating")
		button := `document.querySelectorAll('.vd-dock-track .vd-dock-button')[3]`
		center := page.MustEval(`()=>{const r=` + button + `.getBoundingClientRect();return [r.x+r.width/2,r.y+r.height/2];}`).Arr()
		page.Mouse.MustMoveTo(center[0].Num(), center[1].Num())
		waitForJSBool(t, page, `()=>getComputedStyle(`+button+`.querySelector('.vd-dock-label')).opacity==='1'`)
		page.MustEval(`async()=>new Promise(r=>setTimeout(r,260))`)
		if !page.MustEval(`()=>{const b=` + button + `,r=b.getBoundingClientRect();return b.matches(':hover') && b.contains(document.elementFromPoint(r.x+r.width/2,r.top+3));}`).Bool() {
			t.Fatalf("%s: the lifted dock icon must take the pointer at its lifted top edge: %s", theme, page.MustEval(`()=>{const b=`+button+`,r=b.getBoundingClientRect();return JSON.stringify({rect:r,hit:describeHit(document.elementFromPoint(r.x+r.width/2,r.top+3))});}`).Str())
		}
		if !page.MustEval(`()=>{const b=` + button + `,l=b.querySelector('.vd-dock-label').getBoundingClientRect(),s=b.closest('.vd-dock-scroll').getBoundingClientRect();return l.top>=s.top && l.bottom<=s.bottom && l.left>=s.left && l.right<=s.right;}`).Bool() {
			t.Fatalf("%s: hover label is clipped", theme)
		}
		shot(theme + "-hover")

		// Wheel and trackpad scrolling anywhere on the visible bar: over an icon, between icons and
		// in the bar's own padding above and below the icons.
		park()
		waitForJSBool(t, page, `()=>document.getElementById('vd-taskbar-apps').classList.contains('vd-dock-overflowing')`)
		points := page.MustEval(`()=>{
            const buttons=[...document.querySelectorAll('.vd-dock-track .vd-dock-button')],bar=document.getElementById('vd-taskbar-apps').getBoundingClientRect();
            const a=buttons[2].getBoundingClientRect(),b=buttons[3].getBoundingClientRect(),cy=a.y+a.height/2;
            return {icon:[a.x+a.width/2,cy],gap:[(a.right+b.left)/2,cy],top:[a.x+a.width/2,bar.top+5],bottom:[a.x+a.width/2,bar.bottom-3]};
        }`)
		for _, where := range []string{"icon", "gap", "top", "bottom"} {
			page.MustEval(`()=>{const s=document.querySelector('.vd-dock-scroll');s.style.scrollBehavior='auto';s.scrollLeft=0;}`)
			point := points.Get(where).Arr()
			page.Mouse.MustMoveTo(point[0].Num(), point[1].Num())
			if err := page.Mouse.Scroll(160, 0, 1); err != nil {
				t.Fatal(err)
			}
			deadline := time.Now().Add(3 * time.Second)
			for page.MustEval(`()=>document.querySelector('.vd-dock-scroll').scrollLeft`).Num() < 20 {
				if time.Now().After(deadline) {
					t.Fatalf("%s: a horizontal wheel over the dock (%s, %v) must scroll it; it hit %s", theme, where, point, page.MustEval(`(x,y)=>describeHit(document.elementFromPoint(x,y))`, point[0].Num(), point[1].Num()).Str())
				}
				time.Sleep(25 * time.Millisecond)
			}
		}
		page.MustEval(`()=>{const s=document.querySelector('.vd-dock-scroll');s.scrollLeft=0;s.style.scrollBehavior='';}`)
		park()

		// The overflow arrows still scroll.
		waitForJSBool(t, page, `()=>!document.querySelector('[data-fruity-dock-scroll-button="right"]').disabled`)
		page.MustElement(`[data-fruity-dock-scroll-button="right"]`).MustClick()
		waitForJSBool(t, page, `()=>document.querySelector('.vd-dock-scroll').scrollLeft>20`)
		page.MustElement(`[data-fruity-dock-scroll-button="left"]`).MustClick()
		waitForJSBool(t, page, `()=>document.querySelector('.vd-dock-scroll').scrollLeft<=2`)
		park()

		// A window over the dock collapses it to the orb; hovering the orb unfurls the dock and its
		// icons take the pointer, leaving collapses it again.
		if p := place("overlap"); !p.Collapsed {
			t.Fatalf("%s: a window over the dock must collapse it: %+v", theme, p)
		}
		waitForJSBool(t, page, `()=>getComputedStyle(document.querySelector('.vd-dock-orb')).opacity==='1'`)
		orb := page.MustEval(`()=>{const r=document.querySelector('.vd-dock-orb').getBoundingClientRect();return [r.x+r.width/2,r.y+r.height/2];}`).Arr()
		page.Mouse.MustMoveTo(orb[0].Num(), orb[1].Num())
		waitForJSBool(t, page, `()=>document.getElementById('vd-taskbar-apps').matches(':hover') && getComputedStyle(document.querySelector('.vd-dock-track .vd-dock-button')).opacity==='1'`)
		page.MustEval(`async()=>new Promise(r=>setTimeout(r,700))`)
		if !page.MustEval(`()=>{const b=document.querySelectorAll('.vd-dock-track .vd-dock-button')[1],r=b.getBoundingClientRect();return r.width>40 && b.contains(document.elementFromPoint(r.x+r.width/2,r.y+r.height/2));}`).Bool() {
			t.Fatalf("%s: the unfurled dock's icons must take the pointer", theme)
		}
		shot(theme + "-unfurled")
		park()
		waitForJSBool(t, page, `()=>!document.getElementById('vd-taskbar-apps').matches(':hover') && getComputedStyle(document.querySelector('.vd-dock-track .vd-dock-button')).opacity==='0'`)
	}

	// The compact layout below 821 px keeps the hover headroom for a mouse.
	page.MustSetViewport(790, 667, 1, false)
	page.MustEval(`async()=>{fixtureTheme('fruity-light');aurora.renderTaskbar();await frames2();}`)
	windowTakesTheHeadroom("compact", "maximized")
	if errors := page.MustEval(`()=>JSON.stringify(fixtureErrors)`).Str(); errors != "[]" {
		t.Fatal(errors)
	}
}
