package ui

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/go-rod/rod"
)

// The camera preview starts silent. The sound switch turns it on for the running
// preview and the choice is remembered for the next start of the app.
func TestDesktopNetworkCamerasSoundPreferenceBrowser(t *testing.T) {
	jpeg := newNetworkCamerasDetectionFixture(t, false).jpeg
	fixture := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/api/desktop/integrations/go2rtc/")
		switch {
		case path == "app/state":
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{
				"status": "ok", "can_manage": true, "can_talk": false, "webrtc_enabled": false,
				"integration":      map[string]any{"enabled": true, "api_usable": true},
				"streams":          []any{map[string]any{"id": "one", "name": "Front door", "enabled": true, "revision": 1, "media_ready": true, "reachable": true, "state": "online", "codecs": []string{"H264"}, "consumers": 1}},
				"disabled_streams": []any{},
			})
		case strings.HasPrefix(path, "viewer/"):
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprint(w, "<!doctype html><title>synthetic viewer</title><script>window.soundMessages=[];addEventListener('message',event=>window.soundMessages.push(event.data));</script>")
		case strings.HasPrefix(path, "thumbnail/"):
			w.Header().Set("Content-Type", "image/jpeg")
			w.Write(jpeg)
		default:
			fmt.Fprint(w, `{"status":"ok"}`)
		}
	})
	page := desktopAuditBrowserWithAPI(t, map[string]http.Handler{"/api/desktop/integrations/go2rtc/": fixture})
	debug := `()=>{const frame=document.querySelector('.nc-detail-video iframe');return JSON.stringify({src:frame&&frame.src,stored:localStorage.getItem('aurago.desktop.network-cameras'),messages:frame&&frame.contentWindow&&frame.contentWindow.soundMessages,errors:fixtureErrors});}`
	wait := func(expression string) {
		t.Helper()
		if err := page.Timeout(15 * time.Second).Wait(rod.Eval(expression)); err != nil {
			t.Fatalf("camera sound wait failed: %v; state=%s", err, page.MustEval(debug).Str())
		}
	}

	page.MustEval(`()=>{localStorage.removeItem('aurago.desktop.network-cameras');auditTest.openApp('network-cameras');}`)
	wait(`()=>!!document.querySelector('.nc-detail-video iframe')`)
	wait(`()=>{const frame=document.querySelector('.nc-detail-video iframe');return !!(frame.contentWindow&&frame.contentWindow.soundMessages);}`)
	if !page.MustEval(`()=>{const frame=document.querySelector('.nc-detail-video iframe');const button=document.querySelector('[data-action="sound"]');return frame.src.includes('muted=1')&&button.getAttribute('aria-pressed')==='false';}`).Bool() {
		t.Fatal("camera preview did not start without sound")
	}

	page.MustElement(`[data-action="sound"]`).MustClick()
	wait(`()=>document.querySelector('[data-action="sound"]').getAttribute('aria-pressed')==='true'`)
	wait(`()=>{const frame=document.querySelector('.nc-detail-video iframe');return frame.contentWindow.soundMessages.some(message=>message&&message.type==='aurago:camera-muted'&&message.muted===false);}`)
	if !page.MustEval(`()=>document.querySelector('.nc-detail-video iframe').src.includes('muted=1')`).Bool() {
		t.Fatal("switching sound restarted the preview stream: ", page.MustEval(debug).Str())
	}
	if !page.MustEval(`()=>JSON.parse(localStorage.getItem('aurago.desktop.network-cameras')).sound===true`).Bool() {
		t.Fatal("sound choice was not remembered: ", page.MustEval(debug).Str())
	}

	page.MustReload()
	wait(`()=>!!window.auditTest`)
	page.MustEval(`()=>auditTest.openApp('network-cameras')`)
	wait(`()=>!!document.querySelector('.nc-detail-video iframe')`)
	if !page.MustEval(`()=>{const frame=document.querySelector('.nc-detail-video iframe');const button=document.querySelector('[data-action="sound"]');return frame.src.includes('muted=0')&&button.getAttribute('aria-pressed')==='true';}`).Bool() {
		t.Fatal("camera preview did not restore the remembered sound")
	}
	if errors := page.MustEval(`()=>JSON.stringify(fixtureErrors)`).Str(); errors != "[]" {
		t.Fatal("browser errors: ", errors)
	}
}
