package ui

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestDesktopNetworkCamerasMediaBrowser(t *testing.T) {
	for _, theme := range []string{"standard", "fruity"} {
		t.Run(theme, func(t *testing.T) {
			jpeg := newNetworkCamerasDetectionFixture(t, false).jpeg
			var mu sync.Mutex
			revision, calls := 1, 0
			mediaReady := true
			fixture := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				mu.Lock()
				defer mu.Unlock()
				w.Header().Set("Content-Type", "application/json")
				path := strings.TrimPrefix(r.URL.Path, "/api/desktop/integrations/go2rtc/")
				switch {
				case path == "app/state":
					calls++
					streams := []any{}
					for _, id := range []string{"one", "two"} {
						rev := 1
						if id == "one" {
							rev = revision
						}
						streams = append(streams, map[string]any{"id": id, "name": id, "enabled": true, "revision": rev, "media_ready": mediaReady, "reachable": true, "state": "online", "codecs": []string{"H264"}, "video_codecs": []string{"H264"}, "audio_codecs": []string{"PCMA"}, "consumers": calls, "has_substream": true, "media": map[string]any{"talk_enabled": true}})
					}
					json.NewEncoder(w).Encode(map[string]any{"status": "ok", "can_manage": true, "can_talk": true, "webrtc_enabled": true, "integration": map[string]any{"enabled": true, "api_usable": true}, "streams": streams, "disabled_streams": []any{}})
				case strings.HasPrefix(path, "viewer/"):
					w.Header().Set("Content-Type", "text/html")
					fmt.Fprint(w, "<!doctype html><title>synthetic viewer</title>")
				case strings.HasPrefix(path, "thumbnail/"):
					w.Header().Set("Content-Type", "image/jpeg")
					w.Write(jpeg)
				case strings.HasPrefix(path, "talk/"):
					json.NewEncoder(w).Encode(map[string]any{"session": "fixture", "revision": revision, "ws_url": "/api/go2rtc/talk/one/ws?session=fixture"})
				case strings.HasPrefix(path, "diagnostics/"):
					fmt.Fprint(w, `{"status":"ok","width":640,"height":360,"duration_ms":12}`)
				default:
					fmt.Fprint(w, `{"status":"ok"}`)
				}
			})
			page := desktopAuditBrowserWithAPI(t, map[string]http.Handler{"/api/desktop/integrations/go2rtc/": fixture, "/api/go2rtc/talk/": fixture})
			capture := func(size string) {
				if dir := os.Getenv("AURAGO_GO2RTC_SCREENSHOTS"); dir != "" {
					if err := os.MkdirAll(dir, 0755); err != nil {
						t.Fatal(err)
					}
					page.MustScreenshot(filepath.Join(dir, "camera-media-"+theme+"-"+size+".png"))
				}
			}
			page.MustEval(`theme => {
                auditTest.state.bootstrap.settings['appearance.theme']=theme;
                auditTest.applyDesktopSettings();
                localStorage.removeItem('aurago.desktop.network-cameras');
                const nativeTimeout=window.setTimeout;
                window.setTimeout=(fn,ms,...args)=>nativeTimeout(fn,ms===5000?100:ms,...args);
                window.cameraFixture={tracks:[], peers:[], sockets:[], microphoneRequests:0};
                Object.defineProperty(navigator,'mediaDevices',{configurable:true,value:{getUserMedia:async constraints=>{
                    cameraFixture.microphoneRequests++;
                    if(constraints.video || constraints.audio!==true)throw Error('unexpected microphone request');
                    const track={stopped:false,stop(){this.stopped=true}};cameraFixture.tracks.push(track);
                    return {getTracks:()=>[track],getAudioTracks:()=>[track]};
                }}});
                window.RTCPeerConnection=class {
                    constructor(config){this.config=config;this.connectionState='new';cameraFixture.peers.push(this)}
                    addTransceiver(track,options){if(options.direction!=='sendonly')throw Error('unsafe audio direction')}
                    async createOffer(){return {type:'offer',sdp:'synthetic microphone offer'}}
                    async setLocalDescription(offer){this.localDescription=offer}
                    async setRemoteDescription(answer){this.remoteDescription=answer}
                    async addIceCandidate(){}
                    close(){this.closed=true}
                };
                window.WebSocket=class extends EventTarget {
                    constructor(url){super();this.url=url;this.readyState=1;cameraFixture.sockets.push(this);nativeTimeout(()=>this.onopen&&this.onopen(),0)}
                    send(data){const msg=JSON.parse(data);if(msg.type==='webrtc/offer')nativeTimeout(()=>this.onmessage&&this.onmessage({data:JSON.stringify({type:'webrtc/answer',value:'synthetic answer'})}),0)}
                    close(){this.readyState=3;this.closed=true}
                };
                auditTest.openApp('network-cameras');
            }`, theme)
			page.MustWait(`()=>!!document.querySelector('[data-role="talk"]')`)
			page.MustEval(`()=>{window.keptCameraFrame=document.querySelector('.nc-detail-video iframe');window.keptCameraButton=document.querySelector('[data-role="talk"]');}`)
			page.MustElement(`[data-edit="one"]`).MustClick()
			page.MustElement(`[data-field="name"]`).MustSelectAllText().MustInput("Camera draft")
			page.MustEval(`()=>document.querySelector('.nc-media-options').open=true`)
			capture("settings")
			page.MustWait(`()=>document.querySelector('[data-stream-card="one"] .nc-card-meta > span').textContent.includes('3')`)
			if !page.MustEval(`()=>keptCameraFrame===document.querySelector('.nc-detail-video iframe')&&keptCameraButton===document.querySelector('[data-role="talk"]')&&document.querySelector('[data-field="name"]').value==='Camera draft'&&cameraFixture.microphoneRequests===0`).Bool() {
				t.Fatal("passive status refresh rebuilt media/form or opened the microphone")
			}
			page.MustElement(`[data-modal-action="cancel"]`).MustClick()
			page.MustEval(`()=>{const s=document.querySelector('[data-role="media-source"]');s.value='sub';s.dispatchEvent(new Event('change'));}`)
			if !page.MustEval(`()=>document.querySelector('.nc-detail-video iframe').src.includes('role=sub')`).Bool() {
				t.Fatal("source choice not applied")
			}
			for _, gesture := range []string{"mouse", "touch", "keyboard"} {
				page.MustEval(`kind=>{const b=document.querySelector('[data-role="talk"]');b.setPointerCapture=()=>{};if(kind==='keyboard')b.dispatchEvent(new KeyboardEvent('keydown',{key:' ',bubbles:true}));else b.dispatchEvent(new PointerEvent('pointerdown',{button:0,pointerId:1,pointerType:kind,bubbles:true}));}`, gesture)
				page.MustWait(`()=>document.querySelector('[data-role="talk"]').getAttribute('aria-pressed')==='true'`)
				page.MustEval(`kind=>{const b=document.querySelector('[data-role="talk"]');b.dispatchEvent(kind==='keyboard'?new KeyboardEvent('keyup',{key:' ',bubbles:true}):new PointerEvent('pointerup',{pointerId:1,pointerType:kind,bubbles:true}));}`, gesture)
				if !page.MustEval(`()=>cameraFixture.tracks.every(t=>t.stopped)&&cameraFixture.peers.every(p=>p.closed&&p.config.iceServers.length===0)&&cameraFixture.sockets.every(s=>s.closed)&&document.querySelector('[data-role="talk"]').getAttribute('aria-pressed')==='false'`).Bool() {
					t.Fatal("release retained microphone, peer, socket or public ICE server")
				}
			}
			page.MustEval(`()=>{const b=document.querySelector('[data-role="talk"]');b.dispatchEvent(new KeyboardEvent('keydown',{key:'Enter',bubbles:true}));}`)
			page.MustWait(`()=>document.querySelector('[data-role="talk"]').getAttribute('aria-pressed')==='true'`)
			page.MustEval(`()=>window.dispatchEvent(new Event('blur'))`)
			if !page.MustEval(`()=>cameraFixture.tracks.every(t=>t.stopped)`).Bool() {
				t.Fatal("focus loss retained microphone")
			}
			mu.Lock()
			revision++
			mu.Unlock()
			page.MustWait(`()=>document.querySelector('.nc-detail-video iframe').dataset.revision==='2'`)
			if !page.MustEval(`()=>keptCameraFrame===document.querySelector('.nc-detail-video iframe')&&document.querySelector('.nc-detail-video iframe').src.includes('role=sub')`).Bool() {
				t.Fatal("camera revision lost source choice or frame identity")
			}
			page.MustEval(`()=>document.querySelector('[data-role="talk"]').dispatchEvent(new KeyboardEvent('keydown',{key:'Enter',bubbles:true}))`)
			page.MustWait(`()=>document.querySelector('[data-role="talk"]').getAttribute('aria-pressed')==='true'`)
			mu.Lock()
			mediaReady = false
			mu.Unlock()
			page.MustWait(`()=>keptCameraFrame.src==='about:blank'&&cameraFixture.tracks.every(t=>t.stopped)`)
			mu.Lock()
			mediaReady = true
			mu.Unlock()
			page.MustWait(`()=>keptCameraFrame.src.includes('role=sub')`)
			if !page.MustEval(`()=>keptCameraFrame===document.querySelector('.nc-detail-video iframe')&&cameraFixture.microphoneRequests===5`).Bool() {
				t.Fatal("readiness change rebuilt the viewer or restarted capture")
			}
			capture("desktop")
			page.MustSetViewport(390, 844, 1, true)
			page.MustEval(`()=>{const a=auditTest;a.refreshSpacesForViewport();a.applyWindowSnap(a.state.windows.get(a.state.activeWindowId).element,'maximize')}`)
			if !page.MustEval(`()=>document.querySelector('.nc-talk').getBoundingClientRect().height>=44`).Bool() {
				t.Fatal("touch target too small")
			}
			if !page.MustEval(`()=>[...document.querySelectorAll('.nc-detail-actions button,.nc-primary[data-action="add"]')].every(el=>el.getAttribute('aria-label')&&el.querySelector('.nc-glyph').getBoundingClientRect().width>0)`).Bool() {
				t.Fatal("mobile action icons or accessible labels hidden")
			}
			if !page.MustEval(`()=>{const detail=document.querySelector('.nc-detail').getBoundingClientRect(),top=document.querySelector('.nc-content').getBoundingClientRect().top;return [...document.querySelectorAll('.nc-media-toolbar button,.nc-source-choice,.nc-toolbar-actions button')].every(el=>{const r=el.getBoundingClientRect();return r.left>=0&&r.right<=innerWidth&&r.top>=0})&&document.querySelector('.nc-media-toolbar').getBoundingClientRect().top>=top&&document.querySelector('[data-role="media-stats"]').getBoundingClientRect().bottom<=detail.bottom}`).Bool() {
				t.Fatal("narrow camera controls or telemetry clipped")
			}
			capture("touch")
			if errors := page.MustEval(`()=>JSON.stringify(fixtureErrors)`).Str(); errors != "[]" {
				t.Fatal("browser errors: ", errors)
			}
		})
	}
}
