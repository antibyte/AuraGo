package ui

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
)

// Opt-in with the reviewed image on port 14173 and frame origin port 18099.
func TestGodsEyeDesktopBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	if os.Getenv("AURAGO_GEV_BROWSER") != "1" {
		t.Skip("requires the God's Eye View test container")
	}
	html := readDesktopAssetText(t, "desktop.html")
	html = regexp.MustCompile(`(?s)<script\b[^>]*>.*?</script>`).ReplaceAllString(html, "")
	html = regexp.MustCompile(`\{\{[^}]*\}\}`).ReplaceAllString(html, "")
	html = strings.Replace(html, "</head>", `<link rel="stylesheet" href="/css/desktop-app-common.css"><link rel="stylesheet" href="/css/desktop-app-software-store.css"></head>`, 1)
	html = strings.Replace(html, "</body>", `<script src="/gev-shell.js"></script><script src="/js/desktop/apps/software-store.js"></script><script src="/testdata/gods-eye-fixture.js"></script></body>`, 1)
	shell := readDesktopAssetText(t, "js/desktop/bundles/main.bundle.js")
	cut := strings.LastIndex(shell, "    ensureDesktopRadialMenuAnchor();")
	if cut < 0 {
		t.Fatal("desktop startup seam missing")
	}
	shell = shell[:cut] + `window.gevTest={state,openApp,loadIconManifest,closeWindow,toggleMaximizeWindow};})();`
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(Content)))
	mux.HandleFunc("/fixture", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, html)
	})
	mux.HandleFunc("/gev-shell.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/javascript")
		fmt.Fprint(w, shell)
	})
	listener, err := net.Listen("tcp", "127.0.0.1:18099")
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewUnstartedServer(mux)
	srv.Listener = listener
	srv.Start()
	defer srv.Close()
	bin, ok := browserExecutable()
	if !ok {
		t.Skip("Chrome or Edge required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()
	launch := launcher.New().Context(ctx).Bin(bin).Headless(true).NoSandbox(true).Set("enable-unsafe-swiftshader")
	controlURL := launch.MustLaunch()
	defer launch.Cleanup()
	browser := rod.New().Context(ctx).ControlURL(controlURL).MustConnect()
	defer browser.Close()
	page := browser.MustPage(srv.URL + "/fixture")
	defer page.Close()
	page.MustSetViewport(1440, 960, 1, false)
	page.MustWaitLoad()
	page.MustEval(`async()=>{await fixtureReady;await document.fonts.ready}`)
	waitForJSBool(t, page, `()=>!!document.querySelector('.vd-store-app-frame')`)
	frame := page.MustElement(".vd-store-app-frame").MustFrame()
	frame.MustElement(".cesium-widget canvas")
	frame.MustElement("#aurago-provider-notice")
	frame.MustElement(`[data-first-run-choice="explore"]`).MustClick()
	waitForJSBool(t, frame, `()=>!!window.__godsEyeView && document.getElementById('loading-screen').classList.contains('hidden')`)
	frame.MustElement("#reset-globe-view").MustClick()
	waitForJSBool(t, frame, `()=>__godsEyeView.viewer.camera.positionCartographic.height > 10000000 && !__godsEyeView.viewer.camera._currentFlight && __godsEyeView.viewer.scene.globe.tilesLoaded`)
	// Exercise the real canvas controls, and a free live feed through the app.
	height := frame.MustEval(`()=>__godsEyeView.viewer.camera.positionCartographic.height`).Num()
	if artifacts := os.Getenv("AURAGO_BROWSER_ARTIFACT_DIR"); artifacts != "" {
		page.MustScreenshot(filepath.Join(artifacts, "gods-eye-globe-before-controls.png"))
	}
	page.Mouse.MustMoveTo(720, 460).MustScroll(0, -400)
	waitForJSBool(t, frame, fmt.Sprintf(`()=>Math.abs(__godsEyeView.viewer.camera.positionCartographic.height - %f) > 10000`, height))
	frame.MustEval(`()=>__godsEyeView.dataManager.setEnabled('earthquakes',true,{origin:'user'})`)
	waitForJSBool(t, frame, `()=>__godsEyeView.dataManager.layers.get('earthquakes')?.module.getStats().count > 0`)
	if !page.MustEval(`()=>document.querySelector('.vd-store-app-frame').allow.includes('microphone')`).Bool() {
		t.Fatal("microphone permission missing")
	}
	if artifacts := os.Getenv("AURAGO_BROWSER_ARTIFACT_DIR"); artifacts != "" {
		if err := os.MkdirAll(artifacts, 0755); err != nil {
			t.Fatal(err)
		}
		page.MustScreenshot(filepath.Join(artifacts, "gods-eye-desktop.png"))
	}
	width := frame.MustEval(`()=>document.querySelector('.cesium-widget canvas').clientWidth`).Int()
	page.MustSetViewport(1024, 768, 1, false)
	waitForJSBool(t, frame, fmt.Sprintf(`()=>document.querySelector('.cesium-widget canvas').clientWidth < %d`, width))
	page.MustEval(`()=>{const id=document.querySelector('[data-app-id="store-gods-eye-view"]').dataset.windowId;gevTest.closeWindow(id)}`)
	if page.MustEval(`()=>!!document.querySelector('.vd-store-app-frame')`).Bool() {
		t.Fatal("closing the window retained its frame")
	}
	page.MustEval(`()=>gevTest.openApp('software-store')`)
	page.MustElement(`[data-action="configure-gev"]`).MustClick()
	if page.MustElement(`.vd-gev-config input[name="OPENAI_API_KEY"]`).MustProperty("value").Str() != "" {
		t.Fatal("configuration prefilled a stored key")
	}
	page.MustElement(`.vd-gev-config input[name="OPENAI_API_KEY"]`).MustInput("synthetic-browser-fixture-key")
	if artifacts := os.Getenv("AURAGO_BROWSER_ARTIFACT_DIR"); artifacts != "" {
		page.MustScreenshot(filepath.Join(artifacts, "gods-eye-setup.png"))
	}
	page.MustElement(`.vd-gev-config button[type="submit"]`).MustScrollIntoView().MustClick()
	waitForJSBool(t, page, `()=>!!window.configPayload && !document.querySelector('.vd-gev-config')`)
	if !page.MustEval(`()=>configPayload.keys.OPENAI_API_KEY==='synthetic-browser-fixture-key' && configPayload.allowed_origins[0]===location.origin`).Bool() {
		t.Fatal("configuration payload incorrect")
	}
	page.MustEval(`()=>gevTest.openApp('store-gods-eye-view')`)
	page.MustElement(".vd-store-app-frame").MustFrame().MustElement(".cesium-widget canvas")
	if errors := page.MustEval(`()=>JSON.stringify(fixtureErrors)`).Str(); errors != "[]" {
		t.Fatal(errors)
	}
	foreign := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<iframe src="http://127.0.0.1:14173/"></iframe>`)
	}))
	defer foreign.Close()
	foreignPage := browser.MustPage(foreign.URL).MustWaitLoad()
	defer foreignPage.Close()
	if location := foreignPage.MustElement("iframe").MustFrame().MustEval(`()=>location.href`).Str(); location != "chrome-error://chromewebdata/" {
		t.Fatalf("foreign origin was not blocked: %s", location)
	}
}
