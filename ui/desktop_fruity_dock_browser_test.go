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
	"github.com/go-rod/rod/lib/input"
	"github.com/go-rod/rod/lib/proto"
)

func TestDesktopFruityDockBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	html := regexp.MustCompile(`(?s)<script\b[^>]*>.*?</script>`).ReplaceAllString(readDesktopAssetText(t, "desktop.html"), "")
	html = regexp.MustCompile(`\{\{[^}]*\}\}`).ReplaceAllString(html, "")
	html = strings.Replace(html, "</body>", `<script>window._auragoSharedInitialized=true;</script><script src="/js/shared/shared-core.js"></script><script src="/js/shared/lazy-assets.js"></script><script src="/js/desktop/core/module-loader.js"></script><script src="/dock-shell.js"></script><script src="/testdata/aurora-fixture.js"></script></body>`, 1)
	shell := readDesktopAssetText(t, "js/desktop/bundles/main.bundle.js")
	cut := strings.LastIndex(shell, "    ensureDesktopRadialMenuAnchor();")
	if cut < 0 {
		t.Fatal("shell startup seam missing")
	}
	shell = shell[:cut] + `window.aurora={state,loadIconManifest,applyDesktopSettings,renderIcons,renderStartApps,renderStartButtonIcon,wireShellChromeControls,bindViewportMetrics,handleDesktopKeydown,closeContextMenu,renderTaskbar,dockApps,ensureDesktopRadialMenuAnchor};})();`
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
	page := newSmokeBrowser(t).MustPage(server.URL + "/fixture").Timeout(30 * time.Second)
	defer page.Close()
	page.MustSetViewport(1366, 900, 1, false)
	page.MustWaitLoad()
	page.MustEval(`async()=>{
        await fixtureReady;
        Object.assign(window.I18N, await (await fetch('/lang/common/de.json')).json());
        aurora.ensureDesktopRadialMenuAnchor();
        document.querySelectorAll('.radial-item[href]').forEach(link=>link.addEventListener('click',event=>{
            event.preventDefault();window.navigationClicked=link.getAttribute('href');
        }));
    }`)
	for _, size := range [][2]int{{1366, 900}, {900, 360}, {790, 667}, {390, 667}} {
		page.MustSetViewport(size[0], size[1], 1, false)
		for _, theme := range []string{"fruity-light", "fruity-dark"} {
			page.MustEval(`theme=>fixtureTheme(theme)`, theme)
			page.MustElement("#radialTrigger").MustClick()
			if dir := os.Getenv("AURAGO_BROWSER_ARTIFACT_DIR"); dir != "" {
				if err := os.MkdirAll(dir, 0755); err != nil {
					t.Fatal(err)
				}
				page.MustScreenshot(filepath.Join(dir, fmt.Sprintf("navigation-%s-%dx%d.png", theme, size[0], size[1])))
			}
			links := page.MustElements(".radial-item[href]")
			for _, link := range links {
				link.MustScrollIntoView()
				if !link.MustEval(`function(){const r=this.getBoundingClientRect(),trigger=document.getElementById('radialTrigger').getBoundingClientRect();return (innerWidth<=820?r.bottom<=trigger.top:r.top>=trigger.bottom) && r.top>=0 && r.bottom<=innerHeight && r.left>=0 && r.right<=innerWidth && this.contains(document.elementFromPoint(r.x+r.width/2,r.y+r.height/2));}`).Bool() {
					t.Fatalf("%s %dx%d: navigation link is offscreen or covered: %s", theme, size[0], size[1], link.MustEval(`function(){const r=this.getBoundingClientRect();return JSON.stringify({href:this.getAttribute('href'),rect:r,list:this.parentElement.getBoundingClientRect(),hit:document.elementFromPoint(r.x+r.width/2,r.y+r.height/2)?.outerHTML});}`).Str())
				}
				point := link.MustEval(`function(){const r=this.getBoundingClientRect();return [r.x+r.width/2,r.y+r.height/2];}`).Arr()
				page.Mouse.MustMoveTo(point[0].Num(), point[1].Num()).MustClick(proto.InputMouseButtonLeft)
				if page.MustEval(`()=>window.navigationClicked`).Str() != *link.MustAttribute("href") {
					t.Fatal("navigation link did not receive the click")
				}
			}
			page.Keyboard.MustType(input.Escape)
			if page.MustEval(`()=>document.getElementById('radialMenu').classList.contains('open')`).Bool() {
				t.Fatal("Escape did not close navigation")
			}
			page.MustElement("#radialTrigger").MustClick()
			page.Mouse.MustMoveTo(8, 100).MustClick(proto.InputMouseButtonLeft)
			if page.MustEval(`()=>document.getElementById('radialMenu').classList.contains('open')`).Bool() {
				t.Fatal("outside click did not close navigation")
			}
			page.MustElement("#radialTrigger").MustClick()
			page.MustElement("#radialTrigger").MustClick()
			if page.MustEval(`()=>document.getElementById('radialMenu').classList.contains('open')`).Bool() {
				t.Fatal("trigger did not close navigation")
			}
		}
	}
	page.MustSetViewport(1366, 900, 1, false)
	page.MustEval(`async()=>{
        await fixtureReady;
        aurora.state.bootstrap.installed_apps=[{id:'custom-dock',name:'Custom dock app',icon:'apps',dock_visible:true}];
        aurora.state.appsCacheBootstrap=null;
        fixtureTheme('fruity-light');
        aurora.renderTaskbar();
    }`)
	if !page.MustEval(`()=>{const apps=[...aurora.state.bootstrap.builtin_apps,...aurora.state.bootstrap.installed_apps].filter(a=>!a.internal && a.dock_visible!==false);const ids=[...document.querySelectorAll('.vd-dock-button')].map(b=>b.dataset.appId);return apps.length===ids.length && apps.every(a=>ids.includes(a.id));}`).Bool() {
		t.Error("dock must include every app enabled for the dock, beyond the default pins")
	}
	for _, theme := range []string{"fruity-light", "fruity-dark"} {
		page.MustEval(`theme=>{fixtureTheme(theme);aurora.renderTaskbar();}`, theme)
		page.MustElement(`.vd-dock-button[data-app-id="writer"]`).MustHover()
		waitForJSBool(t, page, `()=>getComputedStyle(document.querySelector('[data-app-id="writer"] .vd-dock-label')).opacity==='1'`)
		page.MustEval(`async()=>new Promise(r=>setTimeout(r,200))`)
		if dir := os.Getenv("AURAGO_BROWSER_ARTIFACT_DIR"); dir != "" {
			if err := os.MkdirAll(dir, 0755); err != nil {
				t.Fatal(err)
			}
			page.MustScreenshot(filepath.Join(dir, "dock-hover-"+theme+".png"))
		}
		if !page.MustEval(`()=>{const b=document.querySelector('.vd-dock-button[data-app-id="writer"]'),l=b.querySelector('.vd-dock-label').getBoundingClientRect(),s=b.closest('.vd-dock-scroll').getBoundingClientRect();return l.top>=s.top && l.bottom<=s.bottom && l.left>=s.left && l.right<=s.right;}`).Bool() {
			t.Errorf("%s hover label is clipped: %s", theme, page.MustEval(`()=>{const b=document.querySelector('.vd-dock-button[data-app-id="writer"]');return JSON.stringify({label:b.querySelector('.vd-dock-label').getBoundingClientRect(),scroll:b.closest('.vd-dock-scroll').getBoundingClientRect(),buttonOverflow:getComputedStyle(b).overflow});}`).Str())
		}
		page.Mouse.MustMoveTo(20, 100)
	}
	for _, width := range []int{1920, 900, 390} {
		page.MustSetViewport(width, 900, 1, false)
		page.MustEval(`()=>{aurora.renderTaskbar();document.querySelector('.vd-dock-scroll').scrollTo({left:0,behavior:'instant'});}`)
		waitForJSBool(t, page, `()=>document.querySelector('.vd-taskbar-apps').classList.contains('vd-dock-overflowing') && !document.querySelector('[data-fruity-dock-scroll-button="right"]').disabled`)
		page.MustElement(`[data-fruity-dock-scroll-button="right"]`).MustClick()
		waitForJSBool(t, page, `()=>document.querySelector('.vd-dock-scroll').scrollLeft>20`)
		page.MustElement(`.vd-dock-button[data-app-id="custom-dock"]`).MustFocus()
		waitForJSBool(t, page, `()=>document.querySelector('[data-fruity-dock-scroll-button="right"]').disabled`)
		if !page.MustEval(`()=>{const b=document.querySelector('.vd-dock-button[data-app-id="custom-dock"]'),r=b.getBoundingClientRect(),s=b.closest('.vd-dock-scroll').getBoundingClientRect();return r.left>=s.left-1 && r.right<=s.right+1 && s.left>=0 && s.right<=innerWidth;}`).Bool() {
			t.Fatalf("last dock app must be reachable within a %dpx viewport", width)
		}
		page.MustEval(`()=>document.activeElement.blur()`)
	}
	page.MustSetViewport(1366, 900, 1, false)
	if !page.MustEval(`()=>{
        [...aurora.state.bootstrap.builtin_apps,...aurora.state.bootstrap.installed_apps].forEach(a=>a.dock_visible=false);
        aurora.renderTaskbar();
        return document.querySelectorAll('.vd-dock-button').length===0;
    }`).Bool() {
		t.Fatal("explicitly hidden apps must stay hidden, including default pins")
	}
	if !page.MustEval(`()=>{
        const element=document.createElement('section');
        aurora.state.windows.set('active-terminal',{id:'active-terminal',appId:'terminal',spaceId:'1',element});
        aurora.state.windows.set('other-space',{id:'other-space',appId:'writer',spaceId:'2',element});
        aurora.renderTaskbar();
        const buttons=[...document.querySelectorAll('.vd-dock-button')];
        return buttons.length===1 && buttons[0].dataset.appId==='terminal';
    }`).Bool() {
		t.Fatal("hidden launchers must appear only for apps running on the active space")
	}
	if errors := page.MustEval(`()=>JSON.stringify(fixtureErrors)`).Str(); errors != "[]" {
		t.Fatal(errors)
	}
}
