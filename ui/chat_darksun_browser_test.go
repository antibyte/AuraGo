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

func TestDarkSunEclipseBrowserSmoke(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	source := readDesktopAssetText(t, "js/chat/dark-sun-shader.js")
	source = strings.Replace(source, "    window.AuraGoDarkSun =", `
    window.__ds = {
        resize, flare: updateFlare,
        forceFlare(time) { nextFlareTime=time; flareActive=false; },
        flush() { if(gl && skyActive)gl.finish(); },
        stats() { return { active, skyActive, flareIntensity, waveProgress, flares:flareCount,
            embers:emberCount, sparks:sparkCount, heat:heatStrength,
            weather:document.documentElement.getAttribute('data-darksun') || '',
            plain:!!plainCache, cracks:crackPoints.length,
            scene:!!(sceneCanvas && sceneCanvas.style.display !== 'none'),
            width:skyCanvas?.width || 0, height:skyCanvas?.height || 0,
            finite: !ex || Array.from(ex.slice(0,emberCount)).every(Number.isFinite) && Array.from(ey.slice(0,emberCount)).every(Number.isFinite),
            gpuError:gl ? gl.getError() : 0 }; },
        skyPixel() {
            if(!gl || !skyActive)return [];
            const pixels=new Uint8Array(4); renderSky();
            gl.readPixels(Math.floor(skyCanvas.width/2),Math.floor(skyCanvas.height/2),1,1,gl.RGBA,gl.UNSIGNED_BYTE,pixels);
            return Array.from(pixels);
        }
    };
    window.AuraGoDarkSun =`, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte(`<!doctype html><html data-theme="dark-sun"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<link rel="stylesheet" href="/css/chat.bundle.css"><style>
body{height:100vh;overflow:hidden;display:flex;flex-direction:column;margin:0}
.app-header{position:relative;flex:0 0 64px;padding:0 32px}
#chat-box{flex:1;min-height:0;width:100%;padding:56px max(24px,calc((100vw - 820px)/2));box-sizing:border-box;overflow-y:auto}
.app-footer{position:relative;flex:0 0 110px;padding:22px max(24px,calc((100vw - 820px)/2))}
.msg-row{margin-bottom:36px}.bubble{padding:18px 24px;max-width:600px}.msg-row.user{justify-content:flex-end}
input{width:100%;padding:16px;color:var(--text-primary);background:var(--input-bg);border:1px solid var(--input-border);border-radius:16px}
</style></head><body><header class="app-header"><strong>AuraGo</strong></header><main id="chat-box">
<div class="msg-row"><div class="bubble bot">Die Korona flackert über der Ebene. Glut steigt aus den Rissen auf.</div></div>
<div class="msg-row user"><div class="bubble user">Wann bricht die nächste Protuberanz aus?</div></div>
<div class="msg-row"><div class="bubble bot">Bald. Die Lichtwelle wird über das Lager laufen, dann wird es wieder still.</div></div>
<div class="msg-row"><div class="bubble bot">Funken tanzen an den Rändern der Nachrichten, wenn die Glut sie berührt.</div></div>
</main><footer class="app-footer"><input id="composer" aria-label="Nachricht" placeholder="Nachricht schreiben …"></footer>
<script>
window.__errors=[];window.addEventListener('error',e=>__errors.push(e.message));console.error=(...args)=>__errors.push(args.join(' '));
window.__reduce=false;window.matchMedia=()=>({get matches(){return window.__reduce},addEventListener(){}});
window.__nativeRAF=window.requestAnimationFrame.bind(window);window.__pending=new Map();let raf=0;window.requestAnimationFrame=fn=>{__pending.set(++raf,fn);return raf};window.cancelAnimationFrame=id=>__pending.delete(id);
window.__frame=time=>{const jobs=[...__pending.values()];__pending.clear();jobs.forEach(fn=>fn(time))};
if(location.search.includes('fallback')){const get=HTMLCanvasElement.prototype.getContext;HTMLCanvasElement.prototype.getContext=function(type,...args){return type==='webgl'?null:get.call(this,type,...args)}}
let seed=23;Math.random=()=>((seed=(Math.imul(seed,1664525)+1013904223)>>>0)/4294967296);
</script><script src="/darksun.js"></script></body></html>`))
			return
		}
		if r.URL.Path == "/darksun.js" {
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
                const s=__ds, now=performance.now();
                s.resize(); const initial=s.stats();
                s.forceFlare(now+1000);
                const samples=[];let maxFlare=0,maxWave=0,flareSeen=0,sparksSeen=0,maxEmbers=0,maxSparks=0;
                for(let i=0;i<300;i++) {
                    const start=performance.now();__frame(now+i*1000/60);samples.push(performance.now()-start);
                    const st=s.stats();
                    maxFlare=Math.max(maxFlare,st.flareIntensity);maxWave=Math.max(maxWave,st.waveProgress);
                    if(st.weather==='flare')flareSeen++;sparksSeen=Math.max(sparksSeen,st.sparks);
                    maxEmbers=Math.max(maxEmbers,st.embers);maxSparks=Math.max(maxSparks,st.sparks);
                }
                const peak=s.stats(), pixel=s.skyPixel();
                s.flare(1,now+1000+5001);const after=s.stats();
                window.__dsTestTime=now+1000+5001;
                samples.sort((a,b)=>a-b);
                return {initial,peak,after,pixel,maxFlare,maxWave,flareSeen,sparksSeen,maxEmbers,maxSparks,pending:__pending.size,errors:__errors,p50:samples[150],p95:samples[285]};
            }`).Map()
			t.Logf("%s: %v", mode, result)
			initial, peak, after := result["initial"].Map(), result["peak"].Map(), result["after"].Map()
			if !initial["active"].Bool() || !initial["plain"].Bool() || initial["cracks"].Int() < 40 || !initial["scene"].Bool() || initial["embers"].Int() < 20 {
				t.Errorf("scene must start with the plain, cracks and seeded embers: %v", initial)
			}
			if result["maxFlare"].Num() < 0.95 || result["maxWave"].Num() < 0.5 || result["flareSeen"].Int() < 60 || result["sparksSeen"].Int() < 1 {
				t.Errorf("eruption did not build, launch its wave, publish its state or throw sparks: %v", result)
			}
			if after["weather"].Str() != "calm" || after["flareIntensity"].Num() != 0 || after["waveProgress"].Num() != 0 || peak["flares"].Int() != 1 {
				t.Errorf("eruption did not release back to calm: %v", after)
			}
			if !peak["finite"].Bool() || result["maxEmbers"].Int() > 160 || result["maxSparks"].Int() > 240 || result["pending"].Int() != 1 || len(result["errors"].Arr()) > 0 {
				t.Errorf("particle, scheduler or browser error contract failed: %v", result)
			}
			if mode == "webgl" && (!peak["skyActive"].Bool() || peak["gpuError"].Int() != 0 || result["pixel"].Arr()[3].Int() == 0) {
				t.Error("WebGL sky shader did not render")
			}
			if mode == "fallback" && peak["skyActive"].Bool() {
				t.Error("failed WebGL must use the 2D fallback")
			}
			heat := page.MustEval(`() => {
                const box=document.getElementById('chat-box'), r=box.getBoundingClientRect(), t=window.__dsTestTime;
                box.dispatchEvent(new PointerEvent('pointermove',{clientX:r.x+200,clientY:r.y+300,bubbles:true}));
                box.dispatchEvent(new PointerEvent('pointermove',{clientX:r.x+262,clientY:r.y+288,bubbles:true}));
                __frame(t+100); const rising=__ds.stats().heat;
                for(let i=1;i<=150;i++)__frame(t+100+i*1000/60);
                return {rising, settled:__ds.stats().heat, pending:__pending.size};
            }`).Map()
			if heat["rising"].Num() <= 0.05 || heat["settled"].Num() >= heat["rising"].Num()*0.2 || heat["pending"].Int() != 1 {
				t.Errorf("pointer heat did not rise and decay within a single loop: %v", heat)
			}
			if !page.MustEval(`() => {
                const input=document.getElementById('composer'), r=input.getBoundingClientRect();
                input.focus(); input.value='Glut bleibt bedienbar';
                return document.activeElement===input && document.elementFromPoint(r.x+r.width/2,r.y+r.height/2)===input;
            }`).Bool() {
				t.Error("effect layers blocked the composer")
			}
			if dir := os.Getenv("AURAGO_BROWSER_ARTIFACT_DIR"); dir != "" {
				page.MustEval(`async () => {
                    const t=__dsTestTime, start=performance.now(); __ds.forceFlare(t); __ds.flare(1,t);
                    for(let i=0;i<60;i++) { await new Promise(__nativeRAF); __frame(t+1500+performance.now()-start); }
                }`)
				if err := os.MkdirAll(dir, 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, "dark-sun-"+mode+".png"), page.MustScreenshot(), 0644); err != nil {
					t.Fatal(err)
				}
			}
			if os.Getenv("AURAGO_DARKSUN_BENCHMARK") == "1" {
				page.MustSetViewport(1920, 1080, 1, false)
				page.MustEval(`() => __ds.resize()`)
				metrics := page.MustEval(`async () => {
                    const samples=[],frames=[],base=__dsTestTime+20000;let first=0,last=0;
                    __ds.forceFlare(base-1500);
                    for(let i=0;i<120;i++) {
                        const stamp=await new Promise(__nativeRAF);if(!first)first=stamp;if(last)frames.push(stamp-last);last=stamp;
                        const start=performance.now();__frame(base+stamp-first);__ds.flush();samples.push(performance.now()-start);
                    }
                    samples.sort((a,b)=>a-b);frames.sort((a,b)=>a-b);
                    return {fps:119000/(last-first),renderP95:samples[114],frameP95:frames[113],stats:__ds.stats()};
                }`).Map()
				t.Logf("1920x1080 native RAF, GPU-synchronized %s: %v", mode, metrics)
			}
			page.MustSetViewport(2560, 1440, 2, false)
			page.MustEval(`() => __ds.resize()`)
			size := page.MustEval(`() => __ds.stats()`).Map()
			if mode == "webgl" && (size["width"].Int() > 1280 || size["height"].Int() > 720) {
				t.Error("sky exceeded its pixel budget")
			}
			page.MustEval(`() => {document.documentElement.dataset.theme='dark';AuraGoDarkSun.sync()}`)
			if page.MustEval(`() => __ds.stats().active || __pending.size!==0 || document.documentElement.hasAttribute('data-darksun') || document.querySelector('.app-header').style.getPropertyValue('--darksun-flash') === '1'`).Bool() {
				t.Error("theme exit did not stop animation or clear the scene state")
			}
			page.MustEval(`() => {document.documentElement.dataset.theme='dark-sun';AuraGoDarkSun.sync();AuraGoDarkSun.sync()}`)
			if !page.MustEval(`() => __ds.stats().active && __pending.size===1 && document.querySelectorAll('#dark-sun-overlay').length===1 && document.querySelectorAll('#dark-sun-scene').length===1`).Bool() {
				t.Error("theme restart duplicated resources")
			}
			page.MustEval(`() => {Object.defineProperty(document,'hidden',{configurable:true,value:true});document.dispatchEvent(new Event('visibilitychange'));AuraGoDarkSun.sync()}`)
			if page.MustEval(`() => __ds.stats().active || __pending.size!==0`).Bool() {
				t.Error("hidden tab did not stop animation")
			}
			page.MustEval(`() => {delete document.hidden;document.dispatchEvent(new Event('visibilitychange'))}`)
			if !page.MustEval(`() => __ds.stats().active && __pending.size===1`).Bool() {
				t.Error("visible tab did not resume animation")
			}
			page.MustEval(`() => {window.__reduce=true;AuraGoDarkSun.sync()}`)
			if page.MustEval(`() => __ds.stats().active || __pending.size!==0`).Bool() {
				t.Error("reduced motion did not stop animation")
			}
			page.MustSetViewport(700, 844, 1, true)
			page.MustEval(`() => {window.__reduce=false;AuraGoDarkSun.sync()}`)
			if page.MustEval(`() => __ds.stats().active`).Bool() {
				t.Error("narrow-screen gate was lost")
			}
		})
	}
}
