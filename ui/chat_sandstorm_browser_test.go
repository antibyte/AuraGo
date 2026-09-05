package ui

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
)

func TestSandstormWeatherBrowserSmoke(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	source := readDesktopAssetText(t, "js/chat/sandstorm-particles.js")
	source = strings.Replace(source, "    window.AuraGoSandstorm =", `
    window.__sand = {
        resize, weather: updateWeather,
        forceStorm(time) { nextStormTime=time; stormActive=false; },
        stats() { return { active, fogActive, stormIntensity, windSpeed,
            flying:fCount, ground:gCount, trails:tCount, clouds:cCount,
            width:fogCanvas?.width || 0, height:fogCanvas?.height || 0,
            finite: !fx || Array.from(fx).every(Number.isFinite) && Array.from(fy).every(Number.isFinite),
            visible: !fy ? 0 : Array.from(fy.slice(0,fCount)).filter(y=>y>0 && y<canvas.clientHeight).length,
            gpuError:gl ? gl.getError() : 0 }; },
        fogPixel() {
            if(!gl || !fogActive)return [];
            const pixels=new Uint8Array(4); renderFog(performance.now());
            gl.readPixels(Math.floor(fogCanvas.width/2),Math.floor(fogCanvas.height/2),1,1,gl.RGBA,gl.UNSIGNED_BYTE,pixels);
            return Array.from(pixels);
        }
    };
    window.AuraGoSandstorm =`, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte(`<!doctype html><html data-theme="sandstorm"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<link rel="stylesheet" href="/css/chat.bundle.css"><style>
body{height:100vh;overflow:hidden;display:flex;flex-direction:column;margin:0}
.app-header{position:relative;flex:0 0 64px;padding:0 32px}
#chat-box{flex:1;min-height:0;width:100%;padding:56px max(24px,calc((100vw - 820px)/2));box-sizing:border-box;overflow-y:auto}
.app-footer{position:relative;flex:0 0 110px;padding:22px max(24px,calc((100vw - 820px)/2))}
.msg-row{margin-bottom:36px}.bubble{padding:18px 24px;max-width:600px}.msg-row.user{justify-content:flex-end}
input{width:100%;padding:16px;color:var(--text-primary);background:var(--input-bg);border:1px solid var(--input-border);border-radius:16px}
</style></head><body><header class="app-header"><strong>AuraGo</strong></header><main id="chat-box">
<div class="msg-row"><div class="bubble bot">Der Wind zieht über die Dünen. Feiner Staub wirbelt durch die warme Abendluft.</div></div>
<div class="msg-row user"><div class="bubble user">Wie entwickelt sich der Sandsturm?</div></div>
<div class="msg-row"><div class="bubble bot">Eine Böe kommt näher. Die Sandkörner tanzen über den Boden, während dünne Staubschleier vorbeiziehen.</div></div>
</main><footer class="app-footer"><input id="composer" aria-label="Nachricht" placeholder="Nachricht schreiben …"></footer>
<script>
window.__errors=[];window.addEventListener('error',e=>__errors.push(e.message));console.error=(...args)=>__errors.push(args.join(' '));
window.__reduce=false;window.matchMedia=()=>({get matches(){return window.__reduce},addEventListener(){}});
window.__nativeRAF=window.requestAnimationFrame.bind(window);window.__pending=new Map();let raf=0;window.requestAnimationFrame=fn=>{__pending.set(++raf,fn);return raf};window.cancelAnimationFrame=id=>__pending.delete(id);
window.__frame=time=>{const jobs=[...__pending.values()];__pending.clear();jobs.forEach(fn=>fn(time))};
if(location.search.includes('fallback')){const get=HTMLCanvasElement.prototype.getContext;HTMLCanvasElement.prototype.getContext=function(type,...args){return type==='webgl'?null:get.call(this,type,...args)}}
let seed=17;Math.random=()=>((seed=(Math.imul(seed,1664525)+1013904223)>>>0)/4294967296);
</script><script src="/sandstorm.js"></script></body></html>`))
			return
		}
		if r.URL.Path == "/sandstorm.js" {
			w.Header().Set("Content-Type", "application/javascript")
			_, _ = w.Write([]byte(source))
			return
		}
		http.FileServer(http.FS(Content)).ServeHTTP(w, r)
	}))
	defer server.Close()
	bin, ok := browserExecutable()
	if !ok {
		t.Skip("Chrome or Edge required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	launch := launcher.New().Context(ctx).Bin(bin).Headless(true).NoSandbox(true).Set("enable-unsafe-swiftshader")
	url := launch.MustLaunch()
	defer func() { launch.Kill(); launch.Cleanup() }()
	browser := rod.New().Context(ctx).ControlURL(url).MustConnect()
	defer browser.Close()
	for _, mode := range []string{"webgl", "fallback"} {
		t.Run(mode, func(t *testing.T) {
			page := browser.MustPage("about:blank")
			defer page.Close()
			page.MustSetViewport(1280, 800, 1, false)
			page.MustNavigate(server.URL + "/?" + mode).MustWaitLoad()
			result := page.MustEval(`() => {
                const s=__sand, now=performance.now();
                s.resize(); const initial=s.stats();
                s.forceStorm(now+2000);
                const samples=[];
                for(let i=0;i<360;i++) {const start=performance.now();__frame(now+i*1000/60);samples.push(performance.now()-start)}
                const peak=s.stats(), pixel=s.fogPixel();
                s.weather(1,now+11001);const after=s.stats();
                window.__sandTestTime=now+11001;
                samples.sort((a,b)=>a-b);
                return {initial,peak,after,pixel,pending:__pending.size,errors:__errors,p50:samples[180],p95:samples[342]};
            }`).Map()
			t.Logf("%s: %v", mode, result)
			initial, peak, after := result["initial"].Map(), result["peak"].Map(), result["after"].Map()
			if !initial["active"].Bool() || initial["visible"].Int() < initial["flying"].Int()/2 {
				t.Error("sand should be visible immediately across the scene")
			}
			if peak["stormIntensity"].Num() < 0.9 || peak["windSpeed"].Num() <= initial["windSpeed"].Num()*2 || after["stormIntensity"].Num() != 0 {
				t.Error("gust did not build and release")
			}
			if !peak["finite"].Bool() || peak["flying"].Int() > 320 || peak["ground"].Int() > 2500 || result["pending"].Int() != 1 || len(result["errors"].Arr()) > 0 {
				t.Error("particle, scheduler or browser error contract failed")
			}
			if mode == "webgl" && (!peak["fogActive"].Bool() || peak["gpuError"].Int() != 0 || result["pixel"].Arr()[3].Int() == 0) {
				t.Error("WebGL dust shader did not render")
			}
			if mode == "fallback" && peak["fogActive"].Bool() {
				t.Error("failed WebGL must use 2D fallback")
			}
			if !page.MustEval(`() => {
                const input=document.getElementById('composer'), r=input.getBoundingClientRect();
                input.focus(); input.value='Sand bleibt bedienbar';
                return document.activeElement===input && document.elementFromPoint(r.x+r.width/2,r.y+r.height/2)===input;
            }`).Bool() {
				t.Error("effect layers blocked the composer")
			}
			if dir := os.Getenv("AURAGO_BROWSER_ARTIFACT_DIR"); dir != "" {
				page.MustEval(`async () => {
                    const t=__sandTestTime, start=performance.now(); __sand.forceStorm(t); __sand.weather(1,t);
                    for(let i=0;i<60;i++) { await new Promise(__nativeRAF); __frame(t+4000+performance.now()-start); }
                }`)
				if err := os.MkdirAll(dir, 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, "sandstorm-"+mode+".png"), page.MustScreenshot(), 0644); err != nil {
					t.Fatal(err)
				}
			}
			page.MustSetViewport(2560, 1440, 2, false)
			page.MustEval(`() => __sand.resize()`)
			size := page.MustEval(`() => __sand.stats()`).Map()
			if mode == "webgl" && (size["width"].Int() > 960 || size["height"].Int() > 540) {
				t.Error("fog exceeded pixel budget")
			}
			page.MustEval(`() => {document.documentElement.dataset.theme='dark';AuraGoSandstorm.sync()}`)
			if page.MustEval(`() => __sand.stats().active || __pending.size!==0`).Bool() {
				t.Error("theme exit did not stop animation")
			}
			page.MustEval(`() => {document.documentElement.dataset.theme='sandstorm';AuraGoSandstorm.sync();AuraGoSandstorm.sync()}`)
			if !page.MustEval(`() => __sand.stats().active && __pending.size===1 && document.querySelectorAll('#sandstorm-overlay').length===1`).Bool() {
				t.Error("theme restart duplicated resources")
			}
			page.MustEval(`() => {Object.defineProperty(document,'hidden',{configurable:true,value:true});document.dispatchEvent(new Event('visibilitychange'));AuraGoSandstorm.sync()}`)
			if page.MustEval(`() => __sand.stats().active || __pending.size!==0`).Bool() {
				t.Error("hidden tab did not stop animation")
			}
			page.MustEval(`() => {delete document.hidden;document.dispatchEvent(new Event('visibilitychange'))}`)
			if !page.MustEval(`() => __sand.stats().active && __pending.size===1`).Bool() {
				t.Error("visible tab did not resume animation")
			}
			page.MustEval(`() => {window.__reduce=true;AuraGoSandstorm.sync()}`)
			if page.MustEval(`() => __sand.stats().active || __pending.size!==0`).Bool() {
				t.Error("reduced motion did not stop animation")
			}
			page.MustSetViewport(390, 844, 1, true)
			page.MustEval(`() => {window.__reduce=false;AuraGoSandstorm.sync()}`)
			if page.MustEval(`() => __sand.stats().active`).Bool() {
				t.Error("narrow-screen gate was lost")
			}
		})
	}
}
