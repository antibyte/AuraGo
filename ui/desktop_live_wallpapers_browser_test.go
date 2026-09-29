package ui

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
)

const liveWallpaperHarness = `<!doctype html><html><head><meta charset="utf-8">
<link rel="stylesheet" href="/css/desktop-shell.bundle.css">
<script>window.liveLogs=[];for(const k of ['warn','error']){const o=console[k];console[k]=(...a)=>{liveLogs.push(a.join(' '));o(...a)}};addEventListener('error',e=>liveLogs.push(e.message));</script>
</head><body class="desktop-body" data-theme="standard" data-animations="true">
<div id="vd-wallpaper-live" class="vd-wallpaper-live" aria-hidden="true" hidden></div>
<main id="main-content" class="vd-shell"><section class="vd-workspace" id="vd-workspace"></section></main>
<script type="module" src="/js/desktop/live-wallpapers.js"></script></body></html>`

// Render every animated wallpaper in a real browser: frames arrive with real colour, a
// covering maximized window pauses the loop, reduced motion leaves a still frame and a photo
// wallpaper releases the WebGL canvas.
func TestDesktopLiveWallpapersBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	bin, ok := browserExecutable()
	if !ok {
		t.Fatal("Chrome or Edge required")
	}
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(Content)))
	mux.HandleFunc("/live-harness", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, liveWallpaperHarness)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	launch := launcher.New().Bin(bin).Headless(true).NoSandbox(true).Set("enable-unsafe-swiftshader")
	browser := rod.New().ControlURL(launch.MustLaunch()).MustConnect()
	defer launch.Cleanup()
	defer browser.Close()
	page := browser.MustPage("")
	defer page.Close()
	page.MustSetViewport(1600, 900, 1, false)
	if err := (proto.EmulationSetEmulatedMedia{Features: []*proto.EmulationMediaFeature{{Name: "prefers-reduced-motion", Value: "no-preference"}}}).Call(page); err != nil {
		t.Fatal(err)
	}
	page.MustNavigate(srv.URL + "/live-harness").MustWaitLoad()
	page.Timeout(20 * time.Second).MustWait(`()=>!!window.AuraLiveWallpaper`)
	dir := os.Getenv("AURAGO_BROWSER_ARTIFACT_DIR")

	for _, id := range desktopLiveWallpapers {
		page.MustEval(`id=>{document.body.dataset.wallpaper=id}`, id)
		if err := page.Timeout(15 * time.Second).Wait(rod.Eval(`()=>{const s=AuraLiveWallpaper.inspect();return s.state==='live-wallpaper:running'&&s.frames>3}`)); err != nil {
			t.Fatalf("%s never ran: %s", id, page.MustEval(`()=>JSON.stringify(AuraLiveWallpaper.inspect())`).Str())
		}
		sample := page.MustEval(`()=>AuraLiveWallpaper.sample()`)
		lum, spread := sample.Get("luminance").Int(), sample.Get("spread").Int()
		if lum < 8 || lum > 200 || spread < 4 {
			t.Errorf("%s renders an implausible picture: %s", id, sample.JSON("", ""))
		}
		if page.MustEval(`()=>document.getElementById('vd-wallpaper-live').hidden||!document.querySelector('.vd-wallpaper-live-canvas.is-ready')`).Bool() {
			t.Errorf("%s canvas is not shown", id)
		}
		page.MustEval(`()=>AuraLiveWallpaper.pulse(0.5,0.5)`)
		if dir != "" {
			time.Sleep(250 * time.Millisecond)
			if shot, err := page.Screenshot(false, &proto.PageCaptureScreenshot{Format: proto.PageCaptureScreenshotFormatPng}); err == nil {
				os.WriteFile(filepath.Join(dir, "live-"+id+".png"), shot, 0644)
			}
		}
	}

	page.MustEval(`()=>{const w=document.createElement('div');w.id='live-cover';w.className='vd-window maximized';w.style.cssText='position:fixed;inset:0;background:#111';document.body.appendChild(w)}`)
	if err := page.Timeout(5 * time.Second).Wait(rod.Eval(`()=>AuraLiveWallpaper.inspect().state==='live-wallpaper:paused'`)); err != nil {
		t.Fatal("a covering maximized window must pause the wallpaper")
	}
	before := page.MustEval(`()=>AuraLiveWallpaper.inspect().frames`).Int()
	time.Sleep(600 * time.Millisecond)
	if after := page.MustEval(`()=>AuraLiveWallpaper.inspect().frames`).Int(); after != before {
		t.Errorf("paused wallpaper kept drawing: %d -> %d frames", before, after)
	}
	page.MustEval(`()=>document.getElementById('live-cover').remove()`)
	if err := page.Timeout(5 * time.Second).Wait(rod.Eval(`()=>AuraLiveWallpaper.inspect().state==='live-wallpaper:running'`)); err != nil {
		t.Fatal("the wallpaper must resume once the window is gone")
	}

	page.MustEval(`()=>{document.body.dataset.animations='false'}`)
	if err := page.Timeout(5 * time.Second).Wait(rod.Eval(`()=>AuraLiveWallpaper.inspect().state==='live-wallpaper:still'`)); err != nil {
		t.Fatal("animations off must leave a still frame")
	}
	before = page.MustEval(`()=>AuraLiveWallpaper.inspect().frames`).Int()
	time.Sleep(600 * time.Millisecond)
	if after := page.MustEval(`()=>AuraLiveWallpaper.inspect().frames`).Int(); after != before {
		t.Errorf("still wallpaper kept drawing: %d -> %d frames", before, after)
	}

	page.MustEval(`()=>{document.body.dataset.wallpaper='groupshoot'}`)
	if err := page.Timeout(5 * time.Second).Wait(rod.Eval(`()=>AuraLiveWallpaper.inspect().state==='live-wallpaper:off'&&!document.querySelector('.vd-wallpaper-live-canvas')&&document.getElementById('vd-wallpaper-live').hidden`)); err != nil {
		t.Fatal("a photo wallpaper must release the WebGL canvas")
	}
	if logs := page.MustEval(`()=>JSON.stringify(liveLogs)`).Str(); logs != "[]" {
		t.Fatalf("console reported problems: %s", logs)
	}
}
