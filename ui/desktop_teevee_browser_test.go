package ui

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/go-rod/rod/lib/input"
	"github.com/go-rod/rod/lib/proto"
)

// Real shell, HLS and decoder; network responses and video are reproducible local fixtures.
func TestDesktopTeeVeeBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	read := func(name string) []byte {
		t.Helper()
		b, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	movie := read("testdata/teevee-test.mp4")
	segment := read("testdata/teevee-test.ts")
	key := []byte("teevee-test-key!") // Synthetic AES fixture, never a credential.
	block, err := aes.NewCipher(key)
	if err != nil {
		t.Fatal(err)
	}
	pad := aes.BlockSize - len(segment)%aes.BlockSize
	encrypted := append(append([]byte{}, segment...), bytes.Repeat([]byte{byte(pad)}, pad)...)
	cipher.NewCBCEncrypter(block, make([]byte, aes.BlockSize)).CryptBlocks(encrypted, encrypted)
	serveMovie := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "video/mp4")
		http.ServeContent(w, r, "teevee-test.mp4", time.Time{}, bytes.NewReader(movie))
	}
	cross := httptest.NewTLSServer(http.HandlerFunc(serveMovie))
	defer cross.Close()
	fixture := strings.ReplaceAll(string(read("testdata/teevee-fixture.js")), "__CROSS_ORIGIN__", cross.URL)
	html := readDesktopAssetText(t, "desktop.html")
	html = regexp.MustCompile(`(?s)<script\b[^>]*>.*?</script>`).ReplaceAllString(html, "")
	html = regexp.MustCompile(`\{\{[^}]*\}\}`).ReplaceAllString(html, "")
	html = strings.Replace(html, "</head>", `<link rel="stylesheet" href="/css/teevee.css"><style>body{margin:0}.vd-shell{height:100vh}</style></head>`, 1)
	html = strings.Replace(html, "</body>", `<script src="/teevee-shell.js"></script><script src="/js/vendor/hls.min.js"></script><script src="/js/desktop/core/media-helpers.js"></script><script src="/js/desktop/apps/teevee-crt.js"></script><script src="/js/desktop/apps/teevee-catalog.js"></script><script src="/js/desktop/apps/teevee.js"></script><script src="/teevee-fixture.js"></script></body>`, 1)
	shell := readDesktopAssetText(t, "js/desktop/bundles/main.bundle.js")
	cut := strings.LastIndex(shell, "    ensureDesktopRadialMenuAnchor();")
	if cut < 0 {
		t.Fatal("shell startup seam missing")
	}
	shell = shell[:cut] + `window.teeveeTest={state,openApp,loadIconManifest,closeWindow,minimizeWindow,focusWindow,toggleMaximizeWindow,switchSpace};})();`
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(Content)))
	for name, value := range map[string]string{"/fixture": html, "/teevee-shell.js": shell, "/teevee-fixture.js": fixture} {
		mux.HandleFunc(name, func(w http.ResponseWriter, r *http.Request) {
			if name == "/fixture" {
				w.Header().Set("Content-Type", "text/html")
			} else {
				w.Header().Set("Content-Type", "text/javascript")
			}
			fmt.Fprint(w, value)
		})
	}
	mux.HandleFunc("/testdata/teevee-test.mp4", serveMovie)
	// Only local fixture media; production proxy policy has separate server tests.
	mux.HandleFunc("/api/desktop/teevee/stream", serveMovie)
	manifest := "#EXTM3U\n#EXT-X-VERSION:3\n#EXT-X-TARGETDURATION:3\n#EXT-X-MEDIA-SEQUENCE:0\n#EXTINF:3.0,\nteevee-test.ts\n#EXT-X-ENDLIST\n"
	mux.HandleFunc("/testdata/teevee-test.m3u8", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
		fmt.Fprint(w, manifest)
	})
	mux.HandleFunc("/testdata/teevee-encrypted.m3u8", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
		fmt.Fprint(w, strings.Replace(strings.Replace(manifest, "#EXTINF", `#EXT-X-KEY:METHOD=AES-128,URI="teevee-key",IV=0x00000000000000000000000000000000`+"\n#EXTINF", 1), "teevee-test.ts", "teevee-encrypted.ts", 1))
	})
	mux.HandleFunc("/testdata/teevee-test.ts", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "video/mp2t")
		w.Write(segment)
	})
	mux.HandleFunc("/testdata/teevee-encrypted.ts", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "video/mp2t")
		w.Write(encrypted)
	})
	mux.HandleFunc("/testdata/teevee-key", func(w http.ResponseWriter, r *http.Request) { w.Write(key) })
	srv := httptest.NewTLSServer(mux)
	defer srv.Close()
	page := newSmokeBrowser(t).MustIgnoreCertErrors(true).MustPage().Timeout(150 * time.Second)
	defer page.Close()
	page.MustSetViewport(1700, 1020, 1, false)
	artifacts := os.Getenv("AURAGO_BROWSER_ARTIFACT_DIR")
	if artifacts != "" {
		if err := os.MkdirAll(artifacts, 0755); err != nil {
			t.Fatal(err)
		}
	}
	screenshot := func(name string) {
		t.Helper()
		if artifacts != "" {
			if err := os.WriteFile(filepath.Join(artifacts, name+".png"), page.MustElement(`[data-app-id="teevee"]`).MustScreenshot(), 0644); err != nil {
				t.Fatal(err)
			}
		}
	}
	check := func(name, js string) {
		t.Helper()
		if !page.MustEval(js).Bool() {
			t.Fatalf("%s; errors=%s", name, page.MustEval(`()=>JSON.stringify(fixtureErrors)`).Str())
		}
	}
	open := func(query string) {
		t.Helper()
		page.MustNavigate(srv.URL + "/fixture" + query).MustWaitLoad()
		page.MustEval(`async()=>{await fixtureReady;await document.fonts.ready;}`)
		waitForJSBool(t, page, `()=>document.querySelectorAll('[data-channel-play]').length===40`)
		check("fixture loaded", `()=>fixtureErrors.length===0`)
	}
	menu := func(label string) {
		t.Helper()
		page.MustElement(`[data-window-menu="view"]`).MustClick()
		page.MustElementR(`.vd-window-menu-item`, label).MustClick()
	}
	measure := func(name string) {
		t.Helper()
		if artifacts == "" || os.Getenv("AURAGO_TEEVEE_RECORD") != "1" {
			return
		}
		data := page.MustEval(`async()=>{
            const v=document.querySelector('video'),c=document.querySelector('.teevee-crt-canvas'),gl=c.getContext('webgl');
            const before=v.getVideoPlaybackQuality(),raf=[],submit=[],synchronized=[];
            const upload=gl.texImage2D.bind(gl),draw=gl.drawArrays.bind(gl);let start=0,passes=0;
            gl.texImage2D=(...args)=>{start=performance.now();passes=0;return upload(...args)};
            gl.drawArrays=(...args)=>{const result=draw(...args);if(++passes===2){submit.push(performance.now()-start);gl.finish();synchronized.push(performance.now()-start)}return result};
            try {await new Promise(resolve=>{let previous;function frame(now){if(previous)raf.push(now-previous);previous=now;if(raf.length<120)requestAnimationFrame(frame);else resolve()}requestAnimationFrame(frame)});}
            finally {gl.texImage2D=upload;gl.drawArrays=draw;}
            const after=v.getVideoPlaybackQuality(),ext=gl.getExtension('WEBGL_debug_renderer_info');
            const p95=a=>a.length?[...a].sort((x,y)=>x-y)[Math.floor((a.length-1)*.95)]:null;
            return JSON.stringify({userAgent:navigator.userAgent,renderer:ext?gl.getParameter(ext.UNMASKED_RENDERER_WEBGL):gl.getParameter(gl.RENDERER),mode:document.querySelector('[data-video-mount]').dataset.crtMode,source:[v.videoWidth,v.videoHeight],canvas:[c.width,c.height],frames:after.totalVideoFrames-before.totalVideoFrames,dropped:after.droppedVideoFrames-before.droppedVideoFrames,rafP95Ms:p95(raf),cpuSubmitP95Ms:p95(submit),synchronizedPipelineP95Ms:p95(synchronized),note:'gl.finish diagnostic includes CPU upload and GPU completion; not a pure GPU timer. Headless software rendering is not native GPU acceptance.'},null,2);
        }`).Str()
		if err := os.WriteFile(filepath.Join(artifacts, "performance-"+name+".json"), []byte(data+"\n"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	checkTubeAspect := func() {
		t.Helper()
		check("reference tube proportions", `()=>{const r=document.querySelector('.teevee-video-shell').getBoundingClientRect();return r.height>=100 && Math.abs(r.width/r.height-750/643)<.005}`)
	}
	page.MustSetViewport(1522, 673, 1, false)
	open("?natural")
	check("opening fits the short desktop with sidebar visible", `()=>{const w=document.querySelector('.vd-window').getBoundingClientRect(),d=document.querySelector('#vd-workspace').getBoundingClientRect(),bar=document.querySelector('.vd-taskbar');return w.width>=1140 && w.right<=d.right && w.bottom<=d.bottom-bar.offsetHeight && getComputedStyle(document.querySelector('.teevee-sidebar')).display==='flex'}`)
	check("compact menus do not cover window controls", `()=>document.querySelector('.vd-window-menubar').getBoundingClientRect().right<=document.querySelector('.vd-window-actions').getBoundingClientRect().left+1`)
	checkTubeAspect()
	for _, target := range []string{".vd-window-menubar", ".teevee-model > div", ".teevee-model img"} {
		point := page.MustEval(`selector=>{const r=document.querySelector(selector).getBoundingClientRect();return {x:selector==='.vd-window-menubar'?r.right-12:r.x+r.width/2,y:r.y+r.height/2,left:document.querySelector('.vd-window').getBoundingClientRect().left};}`, target)
		page.Mouse.MustMoveTo(point.Get("x").Num(), point.Get("y").Num()).MustDown(proto.InputMouseButtonLeft).
			MustMoveTo(point.Get("x").Num()+24, point.Get("y").Num()).MustUp(proto.InputMouseButtonLeft)
		check("free header drags from "+target, fmt.Sprintf(`()=>Math.abs(document.querySelector('.vd-window').getBoundingClientRect().left-%f)<1`, point.Get("left").Num()+24))
	}
	screenshot("teevee-natural-short-desktop")
	page.MustElement(`[data-action="maximize"]`).MustClick()
	checkTubeAspect()
	screenshot("teevee-maximized-short-desktop")
	page.MustElement(`[data-action="maximize"]`).MustClick()
	page.MustEval(`()=>document.querySelector('.vd-window').style.width='360px'`)
	check("minimum width keeps sidebar visible", `()=>document.querySelector('.vd-window').getBoundingClientRect().width>=1140 && getComputedStyle(document.querySelector('.teevee-sidebar')).display==='flex'`)
	edge := page.MustEval(`()=>{const r=document.querySelector('[data-resize="w"]').getBoundingClientRect();return {x:r.x+r.width/2,y:r.y+r.height/2,left:document.querySelector('.vd-window').getBoundingClientRect().left};}`)
	page.Mouse.MustMoveTo(edge.Get("x").Num(), edge.Get("y").Num()).MustDown(proto.InputMouseButtonLeft).
		MustMoveTo(edge.Get("x").Num()+400, edge.Get("y").Num()).MustUp(proto.InputMouseButtonLeft)
	check("left resize stops at minimum without shifting the window", fmt.Sprintf(`()=>Math.abs(document.querySelector('.vd-window').getBoundingClientRect().left-%f)<1`, edge.Get("left").Num()))
	page.MustEval(`()=>{teeveeTest.closeWindow(document.querySelector('.vd-window').dataset.windowId);teeveeTest.openApp('teevee',{sessionRestore:{width:720,height:400,left:16,top:16}});}`)
	waitForJSBool(t, page, `()=>document.querySelectorAll('[data-channel-play]').length===40`)
	check("small saved bounds expand to sidebar minimum", `()=>{const w=document.querySelector('.vd-window').getBoundingClientRect();return w.width>=1140 && w.height>=540 && getComputedStyle(document.querySelector('.teevee-sidebar')).display==='flex'}`)
	checkTubeAspect()
	page.MustSetViewport(1700, 1020, 1, false)
	open("")
	checkTubeAspect()
	check("reference state", `()=>document.querySelector('[data-list-count]').textContent==='567' && document.querySelector('[data-filter="favorites"] em').textContent==='3' && !document.querySelector('video').getAttribute('src')`)
	check("all material assets decoded", `async()=>{await Promise.all([...document.querySelectorAll('.teevee-app img,.vd-window-titlebar img')].map(i=>i.decode().catch(()=>{})));return [...document.querySelectorAll('.teevee-app img,.vd-window-titlebar img')].every(i=>i.naturalWidth>0)}`)
	check("reference heading fits", `()=>{const h=document.querySelector('[data-list-title]');return h.scrollWidth<=h.clientWidth}`)
	screenshot("teevee-reference")
	page.MustElement(`[data-search]`).MustInput("3sat")
	waitForJSBool(t, page, `()=>document.querySelectorAll('[data-channel-play]').length===1 && document.querySelector('[data-list-count]').textContent==='1'`)
	page.MustElement(`[data-search]`).MustSelectAllText().MustInput("")
	waitForJSBool(t, page, `()=>document.querySelectorAll('[data-channel-play]').length===40`)
	for _, size := range [][2]int{{1500, 610}, {1440, 900}, {1140, 720}, {1120, 720}, {960, 720}, {390, 844}} {
		page.MustSetViewport(size[0]+32, max(size[1]+80, 940), 1, false)
		open(fmt.Sprintf("?width=%d&height=%d", size[0], size[1]))
		checkTubeAspect()
		check("no horizontal overflow", `()=>{const a=document.querySelector('.teevee-app');return a.scrollWidth<=a.clientWidth+1}`)
		check("power and player fit", `()=>{const w=document.querySelector('.vd-window').getBoundingClientRect(),p=document.querySelector('.teevee-power').getBoundingClientRect(),v=document.querySelector('.teevee-video-shell').getBoundingClientRect();return p.bottom<=w.bottom && p.right<=w.right && v.height>=100 && v.width>=100}`)
		check("window controls fit", `()=>{const w=document.querySelector('.vd-window').getBoundingClientRect(),a=document.querySelector('.vd-window-actions').getBoundingClientRect();return a.right<=w.right && a.left>w.left}`)
		screenshot(fmt.Sprintf("teevee-%d", size[0]))
		if size[0] == 390 {
			menu("Filter")
			check("compact filters reachable", `()=>getComputedStyle(document.querySelector('.teevee-sidebar')).display==='flex'`)
			page.MustElement(`[data-search]`).MustType(input.Escape)
			check("escape closes compact filters", `()=>getComputedStyle(document.querySelector('.teevee-sidebar')).display==='none'`)
		}
	}
	page.MustSetViewport(1700, 1020, 1, false)
	page.MustEval(`()=>{const w=document.querySelector('.vd-window');w.style.width='1672px';w.style.height='941px';document.body.dataset.theme='fruity';document.body.dataset.fruityMode='light';}`)
	check("fruity preserves receiver", `()=>getComputedStyle(document.querySelector('.teevee-sidebar')).backgroundImage.includes('/img/teevee/metal.png') && getComputedStyle(document.querySelector('.vd-window-ai-button')).display==='none'`)
	screenshot("teevee-fruity")
	page.MustEval(`()=>document.body.dataset.theme='standard'`)
	page.MustElement(`[data-action="favorite"]`).MustClick()
	check("favorite does not start video", `()=>!document.querySelector('video').getAttribute('src')`)
	page.MustElement(`[data-action="favorite"]`).MustType(input.Enter)
	check("keyboard favorite isolated", `()=>!document.querySelector('video').getAttribute('src') && document.querySelector('[data-filter="favorites"] em').textContent==='3'`)
	page.MustElement(`[data-channel-play]`).MustClick()
	waitForJSBool(t, page, `()=>document.querySelector('video').currentTime>0.05 && document.querySelector('[data-video-mount]').dataset.crtMode==='webgl'`)
	check("fullscreen button visible when stream starts", `()=>getComputedStyle(document.querySelector('[data-action="fullscreen-video"]')).opacity!=='0'`)
	check("fullscreen waits two seconds before hiding", `async()=>{await new Promise(r=>setTimeout(r,1000));return getComputedStyle(document.querySelector('[data-action="fullscreen-video"]')).opacity!=='0'}`)
	waitForJSBool(t, page, `()=>getComputedStyle(document.querySelector('[data-action="fullscreen-video"]')).opacity==='0'`)
	check("hidden fullscreen does not intercept clicks", `()=>getComputedStyle(document.querySelector('[data-action="fullscreen-video"]')).pointerEvents==='none'`)
	tube := page.MustEval(`()=>{const r=document.querySelector('.teevee-tube').getBoundingClientRect();return {x:r.x+r.width/2,y:r.y+r.height/2};}`)
	page.Mouse.MustMoveTo(tube.Get("x").Num(), tube.Get("y").Num())
	check("tube hover reveals fullscreen", `()=>getComputedStyle(document.querySelector('[data-action="fullscreen-video"]')).opacity!=='0'`)
	page.Mouse.MustMoveTo(1690, 1010)
	check("leaving tube hides fullscreen", `()=>getComputedStyle(document.querySelector('[data-action="fullscreen-video"]')).opacity==='0'`)
	page.MustElement(`[data-action="fullscreen-video"]`).MustFocus().MustType(input.Tab)
	page.KeyActions().Press(input.ShiftLeft).Type(input.Tab).Release(input.ShiftLeft).MustDo()
	check("keyboard focus reveals fullscreen", `()=>document.activeElement.matches('[data-action="fullscreen-video"]') && getComputedStyle(document.activeElement).opacity!=='0'`)
	page.Keyboard.MustType(input.Tab)
	measure("crt")
	page.MustEval(`()=>{window.tvDraws=0;window.tvMotion=-1;const gl=document.querySelector('.teevee-crt-canvas').getContext('webgl'),draw=gl.drawArrays.bind(gl),uniform=gl.uniform1f.bind(gl);gl.drawArrays=(...args)=>{tvDraws++;return draw(...args);};window.tvGL=gl;gl.uniform1f=(l,v)=>{if(v===0)tvMotion=v;return uniform(l,v);};}`)
	check("minimized renderer sleeps", `async()=>{teeveeTest.minimizeWindow([...teeveeTest.state.windows.keys()][0]);await new Promise(r=>setTimeout(r,230));const count=tvDraws;await new Promise(r=>setTimeout(r,180));return tvDraws===count && !document.querySelector('video').paused}`)
	page.MustEval(`()=>teeveeTest.focusWindow([...teeveeTest.state.windows.keys()][0])`)
	waitForJSBool(t, page, `()=>tvDraws>2`)
	check("hidden space renderer sleeps", `async()=>{teeveeTest.switchSpace('2');await new Promise(r=>setTimeout(r,80));const count=tvDraws;await new Promise(r=>setTimeout(r,180));return tvDraws===count}`)
	page.MustEval(`()=>teeveeTest.switchSpace('1')`)
	page.MustEval(`()=>{window.tvLost=tvGL.getExtension('WEBGL_lose_context');tvLost.loseContext();}`)
	waitForJSBool(t, page, `()=>document.querySelector('[data-video-mount]').dataset.crtMode==='basic'`)
	check("context loss preserves native video", `()=>!document.querySelector('video').paused && getComputedStyle(document.querySelector('video')).visibility==='visible'`)
	page.MustEval(`()=>tvLost.restoreContext()`)
	waitForJSBool(t, page, `()=>document.querySelector('[data-video-mount]').dataset.crtMode==='webgl'`)
	check("desktop animations off removes temporal noise", `()=>tvMotion===0`)
	page.MustSetViewport(1700, 1020, 2, false)
	check("DPR buffers bounded", `async()=>{await new Promise(r=>setTimeout(r,100));const c=document.querySelector('.teevee-crt-canvas');return c.width<=1920 && c.height<=1440 && c.width>1000}`)
	page.MustSetViewport(1700, 1020, 1, false)
	page.MustEval(`async()=>{const v=document.querySelector('video');v.pause();v.currentTime=1;await new Promise(r=>v.addEventListener('seeked',r,{once:true}));await new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r)));}`)
	screenshot("teevee-video-crt")
	check("paused renderer sleeps", `async()=>{const count=tvDraws;await new Promise(r=>setTimeout(r,180));return tvDraws===count}`)
	page.MustElement(`[data-window-menu="playback"]`).MustClick()
	page.MustElementR(`.vd-window-menu-item`, "Wiedergabe/Pause").MustClick()
	page.MustEval(`()=>{window.teeveeSource=document.querySelector('video').currentSrc;window.teeveeLoads=0;document.querySelector('video').addEventListener('emptied',()=>teeveeLoads++);}`)
	menu("Röhrenfilter")
	check("filter off keeps stream", `()=>document.querySelector('[data-video-mount]').dataset.crtMode==='off' && document.querySelector('.teevee-crt-canvas').hidden && document.querySelector('video').currentSrc===teeveeSource && teeveeLoads===0 && !document.querySelector('video').paused`)
	menu("Glasreflexion")
	check("clear native image", `()=>getComputedStyle(document.querySelector('.teevee-glass')).display==='none' && getComputedStyle(document.querySelector('[data-video-mount]'),'::after').display==='none'`)
	measure("native")
	page.MustEval(`async()=>{const v=document.querySelector('video');v.pause();v.currentTime=1;await new Promise(r=>v.addEventListener('seeked',r,{once:true}));await new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r)));}`)
	screenshot("teevee-video-native")
	page.MustElement(`[data-window-menu="playback"]`).MustClick()
	page.MustElementR(`.vd-window-menu-item`, "Wiedergabe/Pause").MustClick()
	menu("Röhrenfilter")
	waitForJSBool(t, page, `()=>document.querySelector('[data-video-mount]').dataset.crtMode==='webgl'`)
	check("toggle does not reload", `()=>teeveeLoads===0`)
	tube = page.MustEval(`()=>{const r=document.querySelector('.teevee-tube').getBoundingClientRect();return {x:r.x+r.width/2,y:r.y+r.height/2};}`)
	page.Mouse.MustMoveTo(tube.Get("x").Num(), tube.Get("y").Num())
	page.MustElement(`[data-action="fullscreen-video"]`).MustClick()
	waitForJSBool(t, page, `()=>document.fullscreenElement===document.querySelector('[data-video-shell]')`)
	check("fullscreen retains renderer and glass", `()=>document.fullscreenElement.contains(document.querySelector('.teevee-crt-canvas')) && document.fullscreenElement.contains(document.querySelector('.teevee-glass'))`)
	page.MustEval(`()=>document.exitFullscreen()`)
	if artifacts != "" && os.Getenv("AURAGO_TEEVEE_RECORD") == "1" {
		for i := 0; i < 30; i++ {
			screenshot(fmt.Sprintf("motion-%02d", i))
			time.Sleep(100 * time.Millisecond)
		}
	}
	page.MustElement(`[data-action="power"]`).MustClick()
	check("power stops stream", `()=>document.querySelector('video').paused && !document.querySelector('video').getAttribute('src') && document.querySelector('.teevee-crt-canvas').hidden`)
	page.MustElement(`[data-action="power"]`).MustClick()
	waitForJSBool(t, page, `()=>document.querySelector('video').currentTime>0 && document.querySelector('[data-video-mount]').dataset.crtMode==='webgl'`)
	page.MustElement(`[data-action="stop"]`).MustClick()
	page.MustElement(`[data-window-menu="playback"]`).MustClick()
	page.MustElementR(`.vd-window-menu-item`, "Wiedergabe/Pause").MustClick()
	waitForJSBool(t, page, `()=>document.querySelector('video').currentTime>0`)
	page.MustEval(`()=>{window.oldVideo=document.querySelector('video');window.oldCanvas=document.querySelector('.teevee-crt-canvas');teeveeTest.closeWindow([...teeveeTest.state.windows.keys()][0]);}`)
	check("dispose releases source and canvas", `()=>oldVideo.paused && !oldVideo.getAttribute('src') && !oldCanvas.isConnected`)
	for _, mode := range []string{"hls", "encrypted", "cors", "nogl"} {
		open("?media=" + mode)
		page.MustElement(`[data-channel-play]`).MustClick()
		expected := "webgl"
		if mode == "cors" || mode == "nogl" {
			expected = "basic"
		}
		waitForJSBool(t, page, fmt.Sprintf(`()=>document.querySelector('video').currentTime>0.05 && document.querySelector('[data-video-mount]').dataset.crtMode===%q`, expected))
		check("native playback survives source policy", `()=>!document.querySelector('video').paused && fixtureErrors.length===0`)
		screenshot("teevee-" + mode)
		if mode == "cors" {
			menu("Für Röhrenfilter neu verbinden")
			waitForJSBool(t, page, `()=>document.querySelector('[data-video-mount]').dataset.crtMode==='webgl'`)
			check("explicit reconnect uses existing proxy", `()=>document.querySelector('video').currentSrc.includes('/api/desktop/teevee/stream?')`)
			page.MustElements(`[data-channel-play]`)[1].MustClick()
			waitForJSBool(t, page, `()=>document.querySelector('video').currentTime>0`)
			check("proxy choice stays with selected station", `()=>!document.querySelector('video').currentSrc.includes('/api/desktop/teevee/stream?')`)
		}
	}
	open("")
	for i := 0; i < 20; i++ {
		page.MustEval(`()=>{teeveeTest.closeWindow([...teeveeTest.state.windows.keys()][0]);teeveeTest.openApp('teevee');}`)
	}
	check("single renderer after repeated open/close", `()=>document.querySelectorAll('.teevee-crt-canvas').length===1 && fixtureErrors.length===0`)
	page.MustEval(`()=>{window.holdCatalog=true;}`)
	menu("Aktualisieren")
	page.MustEval(`()=>{teeveeTest.closeWindow([...teeveeTest.state.windows.keys()][0]);heldCatalog.splice(0).forEach(resolve=>resolve());}`)
	check("late catalog never revives app", `async()=>{await new Promise(r=>setTimeout(r,150));return !document.querySelector('.teevee-app') && fixtureErrors.length===0}`)
}
