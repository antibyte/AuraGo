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
)

// The camera viewer keeps the stream muted until the Desktop app asks for
// sound, and it accepts that request from the app window.
func TestGo2RTCViewerSoundBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	mux := http.NewServeMux()
	mux.Handle("/js/", http.FileServer(http.FS(Content)))
	mux.HandleFunc("/api/go2rtc/proxy/", func(w http.ResponseWriter, r *http.Request) {
		file := filepath.Base(r.URL.Path)
		if file != "video-stream.js" && file != "video-rtc.js" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/javascript")
		data, _ := os.ReadFile(filepath.Join("testdata", "go2rtc-1.9.14", file))
		w.Write(data)
	})
	mux.HandleFunc("/fixture", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, `<!doctype html><meta charset="utf-8"><script>
        window.fixtureErrors=[];addEventListener('error',e=>fixtureErrors.push(e.message));addEventListener('unhandledrejection',e=>fixtureErrors.push(String(e.reason)));
        window.WebSocket=class extends EventTarget{static CONNECTING=0;static OPEN=1;static CLOSED=3;constructor(url){super();this.url=url;this.readyState=1;setTimeout(()=>this.dispatchEvent(new Event('open')),0)}send(){}close(){this.readyState=3}};
        </script><script type="module">import '/api/go2rtc/proxy/video-stream.js';import{mountGo2RTCViewer}from'/js/go2rtc-viewer.js';window.player=mountGo2RTCViewer({stream:'one',role:'sub',mode:'mse',muted:new URL(location).searchParams.get('muted')!=='0'});</script>`)
	})
	server := httptest.NewServer(mux)
	defer server.Close()
	// The Desktop app grants its preview frames the autoplay permission, so the
	// fixture does the same: without it the browser mutes an audible element again.
	browser := newSmokeBrowserWith(t, func(l *launcher.Launcher) {
		l.Set("autoplay-policy", "no-user-gesture-required")
	})

	page := browser.MustPage(server.URL + "/fixture?muted=1")
	defer page.MustClose()
	state := `()=>JSON.stringify({player:!!window.player,video:!!(window.player&&window.player.video),muted:window.player&&window.player.video?window.player.video.muted:null,errors:fixtureErrors,html:document.body.innerHTML.slice(-600)})`
	wait := func(expression string) {
		t.Helper()
		if err := page.Timeout(10 * time.Second).Wait(rod.Eval(expression)); err != nil {
			t.Fatalf("viewer wait failed: %v; state=%s", err, page.MustEval(state).Str())
		}
	}
	wait(`()=>!!window.player&&!!window.player.video`)
	if !page.MustEval(`()=>player.video.muted===true`).Bool() {
		t.Fatal("viewer did not start muted")
	}
	page.MustEval(`()=>window.postMessage({type:'aurago:camera-muted',muted:false},location.origin)`)
	wait(`()=>player.video.muted===false`)
	page.MustEval(`()=>window.postMessage({type:'aurago:camera-something-else',muted:true},location.origin)`)
	if !page.MustEval(`()=>player.video.muted===false`).Bool() {
		t.Fatal("viewer followed a foreign message type")
	}
	page.MustEval(`()=>window.postMessage({type:'aurago:camera-muted',muted:true},location.origin)`)
	wait(`()=>player.video.muted===true`)
	if errors := page.MustEval(`()=>JSON.stringify(fixtureErrors)`).Str(); errors != "[]" {
		t.Fatal("browser errors: ", errors)
	}

	soundPage := browser.MustPage(server.URL + "/fixture?muted=0")
	defer soundPage.MustClose()
	if err := soundPage.Timeout(10 * time.Second).Wait(rod.Eval(`()=>!!window.player&&!!window.player.video`)); err != nil {
		t.Fatalf("sound viewer wait failed: %v; state=%s", err, soundPage.MustEval(state).Str())
	}
	if !soundPage.MustEval(`()=>player.video.muted===false`).Bool() {
		t.Fatal("viewer ignored the requested sound")
	}
}
