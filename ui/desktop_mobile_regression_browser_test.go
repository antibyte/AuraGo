package ui

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
)

func TestDesktopMobileRegressionsBrowser(t *testing.T) {
	css := readDesktopAssetText(t, "css/desktop-shell.bundle.css")
	codeMux := http.NewServeMux()
	newCodeStudioFixtureBackend().register(codeMux)
	page := desktopAuditBrowserWithAPI(t, map[string]http.Handler{
		"/css/desktop-shell.bundle.css": http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/css; charset=utf-8")
			_, _ = w.Write([]byte(css))
		}),
		"/api/code-studio/": codeMux,
		"/api/desktop/integrations/contacts": http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[{"id":"mobile-person","name":"Ada Lovelace","relationship":"friend","email":"ada@example.test"}]`))
		}),
		"/api/desktop/integrations/people/": http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[]`))
		}),
	}).CancelTimeout().Timeout(3 * time.Minute)
	// Exercise the real IPTV layout without any external catalog or stream requests.
	page.MustEval(`() => { const native=window.fetch.bind(window); window.fetch=(url,opts)=>String(url).startsWith('https://iptv-org.github.io/api/') ? Promise.resolve(new Response('[]',{headers:{'Content-Type':'application/json'}})) : native(url,opts); }`)
	for _, theme := range []string{"standard", "fruity"} {
		page.MustEval(`theme=>{auditTest.state.bootstrap.settings['appearance.theme']=theme;auditTest.state.bootstrap.settings['appearance.fruity_mode']='light';auditTest.applyDesktopSettings();}`, theme)
		for _, width := range []int{390, 768} {
			t.Run(fmt.Sprintf("%s/teevee/%d", theme, width), func(t *testing.T) {
				setDesktopMobileViewport(t, page, width, 844, true)
				openDesktopMobileApp(t, page, "teevee")
				defer page.MustEval(`async()=>{await auditTest.closeWindow(auditTest.state.activeWindowId);}`)
				assertDesktopMobileWindow(t, page, "teevee", width, 844, false)
				if !page.MustEval(`()=>{
					const w=auditTest.state.windows.get(auditTest.state.activeWindowId).element,c=w.querySelector('[data-window-content]'),app=c.querySelector('.teevee-app'),cr=c.getBoundingClientRect(),ar=app.getBoundingClientRect();
					return !w.classList.contains('vd-mobile-wide-window') && c.scrollWidth<=c.clientWidth+1 && ar.left>=cr.left-1 && ar.right<=cr.right+1;
				}`).Bool() {
					t.Error("TeeVee must use its responsive layout without clipped wide content")
				}
				if !page.MustEval(`()=>{
					const c=document.querySelector('.teevee-app').getBoundingClientRect();
					return ['.teevee-video-shell','.teevee-power','.teevee-list-panel'].every(s=>{const r=document.querySelector(s).getBoundingClientRect();return r.width>0 && r.left>=c.left-1 && r.right<=c.right+1;});
				}`).Bool() {
					t.Error("TeeVee player, channel list and power control must fit the mobile app")
				}
				saveDesktopMobileRegressionScreenshot(t, page, fmt.Sprintf("%s-teevee-%d", theme, width))
			})
		}
		for _, minimized := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/viewer-return/minimized-%t", theme, minimized), func(t *testing.T) {
				setDesktopMobileViewport(t, page, 390, 844, true)
				openDesktopMobileApp(t, page, "viewer")
				id := page.MustEval(`()=>auditTest.state.activeWindowId`).Str()
				defer page.MustEval(`async id=>{await auditTest.closeWindow(id);}`, id)
				if minimized {
					page.MustEval(`id=>auditTest.state.windows.get(id).element.querySelector('[data-action="minimize"]').click()`, id)
					page.Timeout(5*time.Second).MustWait(`id=>auditTest.state.windows.get(id).element.style.display==='none'`, id)
				}
				setDesktopMobileViewport(t, page, 1440, 900, false)
				page.MustEval(`id=>auditTest.focusWindow(id)`, id)
				if !page.MustEval(`id=>{const w=auditTest.state.windows.get(id);return !w.maximized && !w.element.classList.contains('maximized') && !w.element.classList.contains('vd-mobile-forced-maximized') && w.element.style.width.endsWith('px');}`, id).Bool() {
					t.Error("temporary mobile fullscreen must leave a normal desktop window")
				}
				point := page.MustEval(`id=>{const e=auditTest.state.windows.get(id).element,r=e.querySelector('.vd-window-title').getBoundingClientRect(),w=e.getBoundingClientRect();return {x:r.left+Math.min(50,r.width/2),y:r.top+r.height/2,left:w.left,top:w.top};}`, id)
				x, y := point.Get("x").Num(), point.Get("y").Num()
				page.Mouse.MustMoveTo(x, y).MustDown("left").MustMoveTo(x+100, y+60).MustUp("left")
				if !page.MustEval(`(id,left,top)=>{const r=auditTest.state.windows.get(id).element.getBoundingClientRect();return r.left>left+20 || r.top>top+20;}`, id, point.Get("left").Num(), point.Get("top").Num()).Bool() {
					t.Error("window must actually move when dragged after leaving mobile mode")
				}
				page.MustEval(`id=>{const w=auditTest.state.windows.get(id);auditTest.applyWindowSnap(w.element,'maximize');window.mobileExpectedRestore=JSON.stringify(w.restoreBounds);}`, id)
				setDesktopMobileViewport(t, page, 768, 1024, true)
				setDesktopMobileViewport(t, page, 1440, 900, false)
				if !page.MustEval(`id=>{const w=auditTest.state.windows.get(id);return w.maximized && w.element.classList.contains('maximized') && JSON.stringify(w.restoreBounds)===window.mobileExpectedRestore;}`, id).Bool() {
					t.Error("an explicitly maximized window must retain its state and restore bounds")
				}
			})
		}
		t.Run(theme+"/people-keyboard", func(t *testing.T) {
			setDesktopMobileViewport(t, page, 375, 667, true)
			openDesktopMobileApp(t, page, "people")
			defer page.MustEval(`async()=>{delete visualViewport.height;visualViewport.dispatchEvent(new Event('resize'));await auditTest.closeWindow(auditTest.state.activeWindowId);}`)
			page.Timeout(5 * time.Second).MustWait(`()=>!!document.querySelector('.vd-people-card[data-contact-id="mobile-person"]')`)
			page.MustEval(`async()=>{document.querySelector('.vd-people-search').focus();Object.defineProperty(visualViewport,'height',{configurable:true,value:360});visualViewport.dispatchEvent(new Event('resize'));await new Promise(requestAnimationFrame);await new Promise(requestAnimationFrame);}`)
			page.MustElement(`.vd-people-sidebar-item[data-filter="friend"]`).MustClick()
			if !page.MustEval(`()=>{const b=document.querySelector('.vd-people-browse'),r=b.getBoundingClientRect();return b.contains(document.elementFromPoint(r.left+r.width/2,r.bottom-5));}`).Bool() {
				t.Fatal("shell chrome must not intercept touches in the contact browser")
			}
			browse := page.MustEval(`()=>{const r=document.querySelector('.vd-people-browse').getBoundingClientRect();return {x:r.left+r.width/2,top:r.top,bottom:r.bottom};}`)
			for range 3 {
				swipeDesktopMobileRegression(page, browse.Get("x").Num(), browse.Get("bottom").Num()-5, browse.Get("x").Num(), browse.Get("top").Num()+5)
			}
			visible := page.MustEval(`()=>{
				const b=document.querySelector('.vd-people-browse'),br=b.getBoundingClientRect(),card=document.querySelector('.vd-people-card[data-contact-id="mobile-person"]'),r=card.getBoundingClientRect();
				const top=Math.max(r.top,br.top),bottom=Math.min(r.bottom,br.bottom);
				return {height:bottom-top,x:(r.left+r.right)/2,y:(top+bottom)/2,scroll:b.scrollTop};
			}`)
			if visible.Get("height").Num() < 44 || visible.Get("scroll").Num() <= 0 {
				saveDesktopMobileRegressionScreenshot(t, page, theme+"-people-keyboard-failure")
				t.Log(page.MustEval(`()=>['.vd-people','.vd-people-body','.vd-people-browse','.vd-people-sidebar','.vd-people-content'].map(s=>{const e=document.querySelector(s),r=e.getBoundingClientRect(),c=getComputedStyle(e);return {s,top:r.top,bottom:r.bottom,clientHeight:e.clientHeight,scrollHeight:e.scrollHeight,scrollTop:e.scrollTop,overflow:c.overflow,touch:c.touchAction};})`).JSON("", ""))
				t.Fatalf("contacts must remain reachable by touch with the keyboard open: %s", visible.JSON("", ""))
			}
			page.Touch.MustTap(visible.Get("x").Num(), visible.Get("y").Num())
			page.Timeout(5 * time.Second).MustWait(`()=>document.querySelector('.vd-people-detail')?.classList.contains('open')`)
			page.MustElement(`.vd-people-detail-close`).MustClick()
			page.Timeout(5 * time.Second).MustWait(`()=>!document.querySelector('.vd-people-detail')?.classList.contains('open')`)
			if !page.MustEval(`()=>document.querySelector('.vd-people-browse').scrollTop>0 && document.querySelector('.vd-people-sidebar-item[data-filter="friend"]').classList.contains('active')`).Bool() {
				t.Error("closing contact details must preserve the compact filter and list scroll position")
			}
			saveDesktopMobileRegressionScreenshot(t, page, theme+"-people-keyboard")
		})
		t.Run(theme+"/code-studio-touch", func(t *testing.T) {
			setDesktopMobileViewport(t, page, 390, 844, true)
			openDesktopMobileApp(t, page, "code-studio")
			defer page.MustEval(`async()=>{await auditTest.closeWindow(auditTest.state.activeWindowId);}`)
			swipeDesktopMobileRegression(page, 330, 230, 70, 230)
			if page.MustEval(`()=>auditTest.state.windows.get(auditTest.state.activeWindowId).element.querySelector('[data-window-content]').scrollLeft`).Num() < 10 {
				t.Error("a wide Code Studio workspace must still respond to real horizontal touch swipes")
			}
			assertDesktopMobileWindow(t, page, "code-studio", 390, 844, true)
		})
	}
}

func swipeDesktopMobileRegression(page *rod.Page, x1, y1, x2, y2 float64) {
	page.Touch.MustStart(&proto.InputTouchPoint{X: x1, Y: y1})
	for step := 1; step <= 8; step++ {
		f := float64(step) / 8
		page.Touch.MustMove(&proto.InputTouchPoint{X: x1 + (x2-x1)*f, Y: y1 + (y2-y1)*f})
		time.Sleep(25 * time.Millisecond)
	}
	page.Touch.MustEnd()
	time.Sleep(150 * time.Millisecond)
}

func saveDesktopMobileRegressionScreenshot(t *testing.T, page *rod.Page, name string) {
	t.Helper()
	dir := filepath.Join("..", "reports", "desktop-mobile-fixes")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name+".png"), page.MustScreenshot(), 0644); err != nil {
		t.Fatal(err)
	}
}
