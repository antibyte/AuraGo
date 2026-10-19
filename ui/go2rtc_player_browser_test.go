package ui

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"github.com/go-rod/rod"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestGo2RTCPinnedPlayerFixtures(t *testing.T) {
	for file, expected := range map[string]string{"video-stream.js": "86b4b69001242c3c71f9aedceabff06a6bc58155d8a0fe27970834aefc670b16", "video-rtc.js": "d48ce627baf7c341a92c0f5844a3c546431f9db873ff21489671aba2ecfe64fb"} {
		data, err := os.ReadFile(filepath.Join("testdata", "go2rtc-1.9.14", file))
		if err != nil {
			t.Fatal(err)
		}
		if fmt.Sprintf("%x", sha256.Sum256(data)) != expected {
			t.Fatal("upstream player fixture changed", file)
		}
	}
}

// Exercise the real pinned player with AuraGo's actual adaptation module and
// synthetic signaling. HLS URI resolution is checked with native-HLS support
// simulated, because Chromium itself does not implement Safari's HLS decoder.
func TestGo2RTCPlayerProtocolsBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	jpeg := base64.StdEncoding.EncodeToString(newNetworkCamerasDetectionFixture(t, false).jpeg)
	mp4, err := os.ReadFile(filepath.Join("testdata", "teevee-test.mp4"))
	if err != nil {
		t.Fatal(err)
	}
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
	mux.HandleFunc("/movie", func(w http.ResponseWriter, r *http.Request) { w.Write(mp4) })
	mux.HandleFunc("/fixture", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'unsafe-inline'; img-src 'self' data: blob:; media-src 'self' data: blob:; connect-src 'self' ws: wss:")
		fmt.Fprintf(w, `<!doctype html><style>video-stream{display:block;width:640px;height:360px}</style><script>
        window.fixtureErrors=[];window.fixtureCSP=[];window.playerMessages=[];window.playerSockets=[];
        const srcProperty=Object.getOwnPropertyDescriptor(HTMLMediaElement.prototype,'src');
        Object.defineProperty(HTMLMediaElement.prototype,'src',{...srcProperty,set(value){if(String(value).startsWith('data:application/vnd.apple.mpegurl;base64,'))window.fixtureHls=value;srcProperty.set.call(this,value)}});
        addEventListener('error',e=>fixtureErrors.push(e.message));addEventListener('unhandledrejection',e=>fixtureErrors.push(String(e.reason)));addEventListener('securitypolicyviolation',e=>fixtureCSP.push(e.violatedDirective));
        const originalCanPlay=HTMLMediaElement.prototype.canPlayType;HTMLMediaElement.prototype.canPlayType=function(type){return type==='application/vnd.apple.mpegurl'?'probably':originalCanPlay.call(this,type)};
        window.WebSocket=class extends EventTarget {
            static CONNECTING=0;static OPEN=1;static CLOSED=3;
            constructor(url){super();this.url=url;this.readyState=1;this.messages=[];playerSockets.push(this);setTimeout(()=>this.dispatchEvent(new Event('open')),0)}
            send(data){const message=JSON.parse(data);this.messages.push(message);playerMessages.push(message);setTimeout(async()=>{
                const emit=value=>this.dispatchEvent(new MessageEvent('message',{data:value}));
                if(message.type==='mse')emit(JSON.stringify({type:'mse',value:'video/mp4; codecs="avc1.42E01E"'}));
                if(message.type==='hls')emit(JSON.stringify({type:'hls',value:'#EXTM3U\n#EXT-X-STREAM-INF:BANDWIDTH=1000\nhls/playlist.m3u8?id=ABCDEFGH\n'}));
                if(message.type==='mp4')emit(await (await fetch('/movie')).arrayBuffer());
                if(message.type==='mjpeg')emit(Uint8Array.from(atob('%s'),c=>c.charCodeAt(0)).buffer);
            },20)}
            close(){if(this.readyState===3)return;this.readyState=3;this.dispatchEvent(new Event('close'))}
        };
        </script><script type="module">import '/api/go2rtc/proxy/video-stream.js';import{mountGo2RTCViewer}from'/js/go2rtc-viewer.js';window.player=mountGo2RTCViewer({stream:'one',role:'sub',mode:new URL(location).searchParams.get('mode')});</script>`, jpeg)
	})
	server := httptest.NewServer(mux)
	defer server.Close()
	browser := newSmokeBrowser(t)
	for _, mode := range []string{"mse", "hls", "mp4", "mjpeg", "webrtc,mse", "webrtc,mse,mjpeg"} {
		t.Run(mode, func(t *testing.T) {
			page := browser.MustPage(server.URL + "/fixture?mode=" + mode)
			defer page.MustClose()
			wait := func(expression string, args ...interface{}) {
				if err := page.Timeout(8 * time.Second).Wait(rod.Eval(expression, args...)); err != nil {
					t.Fatalf("player wait failed: %v; state=%s", err, page.MustEval(`()=>JSON.stringify({messages:playerMessages,errors:fixtureErrors,csp:fixtureCSP,mode:window.player&&player.mode,src:window.player&&player.video.src,html:document.body.innerHTML.slice(-1000)})`).Str())
				}
			}
			protocol := mode
			if strings.Contains(mode, "webrtc") {
				protocol = "mse"
			}
			wait(`mode=>playerMessages.some(m=>m.type===mode)`, protocol)
			switch mode {
			case "webrtc,mse", "webrtc,mse,mjpeg":
				wait(`()=>player.mseCodecs.includes('avc1')&&player.pc&&playerSockets.some(s=>s.messages.some(m=>m.type==='webrtc/offer'))`)
				mseWins := mode == "webrtc,mse,mjpeg"
				page.MustEval(`mseWins=>{
                    player.pc.close();window.fixturePeerClosed=false;
                    player.pc={remoteDescription:{sdp:'H264/90000'},getSenders:()=>[],close:()=>{fixturePeerClosed=true}};
                    if(mseWins)player.mseCodecs='video/mp4; codecs="hvc1.1.6.L153.B0"';
                    const canvas=document.createElement('canvas');canvas.width=16;canvas.height=8;canvas.getContext('2d').fillRect(0,0,16,8);
                    window.fixtureTracks=canvas.captureStream(1);player.onpcvideo({srcObject:fixtureTracks});
                }`, mseWins)
				if !page.MustEval(`mseWins=>{const rtc=playerSockets.find(s=>s.messages.some(m=>m.type==='webrtc/offer'));const media=playerSockets.find(s=>s.messages.some(m=>m.type==='mse'));if(rtc===media||rtc.readyState!==1||player.ws!==rtc)return false;if(mseWins)return media.readyState===1&&player.pc===null&&fixturePeerClosed;media.dispatchEvent(new MessageEvent('message',{data:new ArrayBuffer(8)}));return media.readyState===3&&player.pcState===1&&player.ondata===null&&!fixturePeerClosed}`, mseWins).Bool() {
					t.Fatal("auto selection retained its losing media consumer or lost its winning connection")
				}
				page.MustEval(`()=>player.ws.close()`)
				if !page.MustEval(`()=>fixturePeerClosed&&player.pc===null&&player.video.srcObject===null&&playerSockets.every(s=>s.readyState===3)`).Bool() {
					t.Fatal("signaling revocation did not terminate the selected RTC peer")
				}
				page.MustEval(`()=>fixtureTracks.getTracks().forEach(track=>track.stop())`)
			case "mse":
				wait(`()=>player.mseCodecs.includes('avc1')&&typeof player.ondata==='function'`)
			case "hls":
				wait(`()=>!!window.fixtureHls`)
				if !page.MustEval(`()=>atob(fixtureHls.split(',')[1]).includes(location.origin+'/api/go2rtc/proxy/api/hls/playlist.m3u8?id=ABCDEFGH')`).Bool() {
					t.Fatal("HLS playlist escaped AuraGo's bound session path")
				}
			case "mp4", "mjpeg":
				wait(`()=>player.video.poster.startsWith('data:image/jpeg;base64,')`)
			}
			if !page.MustEval(`()=>player.pcConfig.iceServers.length===0&&!player.media.includes('microphone')&&fixtureErrors.length===0&&fixtureCSP.length===0`).Bool() {
				t.Fatalf("unsafe player or browser error: %s", page.MustEval(`()=>JSON.stringify({errors:fixtureErrors,csp:fixtureCSP})`).Str())
			}
			if !strings.Contains(mode, "webrtc") && !page.MustEval(`()=>{player.ondisconnect();return !player.reconnectTID&&playerSockets.every(s=>s.readyState===3)}`).Bool() {
				t.Fatal("intentional player stop scheduled an unwanted reconnect")
			}
		})
	}
}
