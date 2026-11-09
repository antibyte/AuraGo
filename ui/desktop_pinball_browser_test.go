package ui

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-rod/rod/lib/input"
	"github.com/go-rod/rod/lib/launcher"
)

func TestDesktopPinballBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(Content)))
	mux.HandleFunc("/fixture", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, `<!doctype html><html lang="de"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><link rel="stylesheet" href="/css/desktop-app-pinball.css"><style>html,body,#host{margin:0;width:100%;height:100%;overflow:hidden}</style></head><body data-theme="standard"><main id="host"></main><script type="module">
import {mountGame} from '/js/vendor/pinball/game.esm.js';
const words=await(await fetch('/lang/desktop/de.json')).json(),query=new URLSearchParams(location.search),fault=query.get('fault');
if(fault==='webgl'){const native=HTMLCanvasElement.prototype.getContext;HTMLCanvasElement.prototype.getContext=function(type,...args){return type.includes('webgl')?null:native.call(this,type,...args)}}
if(!query.has('keep'))localStorage.setItem('aurago.desktop.pinball.settings.v1',JSON.stringify({quality:query.get('quality')||'low'}));
window.fixture=mountGame(document.getElementById('host'),{t:k=>words[k]||k,assetURL:p=>fault==='assets'&&p.endsWith('/manifest.json')?'/missing-pinball.json':p,isActive:()=>window.fixtureActive!==false,testing:true});
</script></body></html>`)
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	benchmark := os.Getenv("AURAGO_PINBALL_BENCHMARK") == "1"
	browser := newSmokeBrowserWith(t, func(l *launcher.Launcher) {
		l.Set("mute-audio")
		l.Set("enable-unsafe-swiftshader")
		if os.Getenv("AURAGO_PINBALL_GPU") == "1" {
			l.Delete("disable-gpu")
		}
		if benchmark {
			l.Context(t.Context())
		}
	})
	page := browser.MustPage("about:blank").Timeout(22 * time.Minute)
	t.Cleanup(func() { _ = page.Close() })
	page.MustSetViewport(1440, 960, 1, false)
	page.MustNavigate(server.URL + "/fixture")
	if benchmark {
		page.MustSetViewport(1920, 1080, 1, false)
		page.MustNavigate(server.URL + "/fixture?quality=high")
	}
	page.MustWait(`()=>window.fixture?.inspect().panel==='lobby'`)
	if _, err := page.Timeout(20 * time.Second).Eval(`()=>new Promise((ok,no)=>{const t0=performance.now();(function poll(){const s=fixture.inspect();if(s.dmd?.base==='attract'&&s.lamps.lit>0&&s.attract===true)return ok(true);if(performance.now()-t0>15000)return no(new Error(JSON.stringify({dmd:s.dmd,lamps:s.lamps,attract:s.attract})));setTimeout(poll,100)})()})`); err != nil {
		t.Fatalf("lobby attract state: %v", err)
	}
	page.MustEval(`()=>{const e=document.querySelector('[data-setting="camera"]');e.value='showcase';e.dispatchEvent(new Event('change',{bubbles:true}))}`)
	page.MustWait(`()=>fixture.inspect().panel==='showcase'`)
	page.MustElement(`[data-action="resume"]`).MustClick()
	page.MustWait(`()=>fixture.inspect().panel==='lobby'`)
	if benchmark {
		page.MustEval(`()=>new Promise(resolve=>{let first=0,count=0;function frame(now){if(!first)first=now;if(++count<120)requestAnimationFrame(frame);else{window.pinballBaselineFPS=119000/(now-first);resolve(true)}}requestAnimationFrame(frame)})`)
		t.Logf("Unrendered requestAnimationFrame baseline: %.2f fps", page.MustEval(`()=>pinballBaselineFPS`).Num())
	}
	for _, table := range []string{"neon", "leviathan", "temple"} {
		if table != "neon" {
			page.MustElement(`[data-action="table-` + table + `"]`).MustClick()
			page.MustWait(`id=>fixture.inspect().table===id&&fixture.inspect().panel==='lobby'`, table)
		}
		if !page.MustEval(`()=>fixture.inspect().graphics.ready&&!!fixture.inspect().graphics.flipperOrder&&document.querySelectorAll('.pb-canvas').length===1&&!!document.querySelector('.pb-dmd canvas')&&fixture.inspect().audio.unlocked===false`).Bool() {
			t.Fatal("table preview or gesture audio lifecycle failed")
		}
		t.Logf("%s graphics: %s", table, page.MustEval(`()=>JSON.stringify(fixture.inspect().graphics)`).Str())
		if benchmark {
			page.MustElement(`[data-action="training"]`).MustClick()
			page.MustWait(`()=>fixture.inspect().round?.training===true`)
			page.MustEval(`()=>{fixture.stress();window.pinballFrames=[];window.pinballPauses=0;window.pinballStressBalls=0}`)
			for minute := 1; minute <= 3; minute++ {
				page.MustEval(`()=>new Promise(resolve=>{let start=performance.now(),last=start,lastFlip=false;function frame(now){pinballFrames.push(now-last);last=now;const s=fixture.inspect();if(s.paused){pinballPauses++;document.querySelector('[data-action="resume"]')?.click()}pinballStressBalls=Math.max(pinballStressBalls,s.stats.balls);const flip=Math.floor((now-start)/700)%2===0;if(flip!==lastFlip){for(const code of ['ShiftLeft','ShiftRight'])document.querySelector('.pb-app').dispatchEvent(new KeyboardEvent(flip?'keydown':'keyup',{code,bubbles:true}));lastFlip=flip}if(now-start<60000)requestAnimationFrame(frame);else resolve(true)}requestAnimationFrame(frame)})`)
				t.Logf("%s multiball minute %d/3", table, minute)
			}
			metrics := page.MustEval(`()=>{const f=pinballFrames.slice(30).sort((a,b)=>a-b);return JSON.stringify({table:fixture.inspect().table,seconds:f.reduce((a,b)=>a+b,0)/1000,fps:1000*f.length/f.reduce((a,b)=>a+b,0),p95ms:f[Math.floor(f.length*.95)],baselineRAFfps:pinballBaselineFPS,maxBalls:pinballStressBalls,pauses:pinballPauses,graphics:fixture.inspect().graphics})}`).Str()
			t.Logf("%s benchmark: %s", table, metrics)
			if dir := os.Getenv("AURAGO_BROWSER_ARTIFACT_DIR"); dir != "" {
				if err := os.MkdirAll(dir, 0755); err != nil {
					t.Fatal(err)
				}
				var document any
				if err := json.Unmarshal([]byte(metrics), &document); err != nil {
					t.Fatal(err)
				}
				data, err := json.MarshalIndent(document, "", "  ")
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, "pinball-"+table+"-metrics.json"), append(data, '\n'), 0644); err != nil {
					t.Fatal(err)
				}
			}
			page.MustElement(`.pb-toolbar [data-action="pause"]`).MustClick()
			page.MustElement(`[data-action="quit"]`).MustClick()
			page.MustElement(`[data-action="lobby"]`).MustClick()
		}
		if dir := os.Getenv("AURAGO_BROWSER_ARTIFACT_DIR"); dir != "" {
			if err := os.MkdirAll(dir, 0755); err != nil {
				t.Fatal(err)
			}
			page.MustElement(`[data-action="training"]`).MustClick()
			page.MustWait(`()=>fixture.inspect().round?.training===true`)
			page.MustEval(`()=>fixture.advance(520,{plunger:1})`)
			page.MustEval(`()=>fixture.advance(240,{left:true,right:true})`)
			page.MustEval(`()=>{const camera=document.querySelector('[data-setting="camera"]');camera.value='showcase';camera.dispatchEvent(new Event('change',{bubbles:true}))}`)
			page.MustWait(`()=>fixture.inspect().panel==='showcase'`)
			page.MustScreenshot(filepath.Join(dir, "pinball-"+table+".png"))
			page.MustElement(`.pb-toolbar [data-action="pause"]`).MustClick()
			page.MustEval(`()=>{fixtureActive=false}`)
			page.MustWait(`()=>fixture.inspect().paused===true`)
			page.MustEval(`()=>{fixtureActive=true}`)
			page.MustElement(`[data-action="quit"]`).MustClick()
			page.MustElement(`[data-action="lobby"]`).MustClick()
		}
	}
	page.MustElement(`[data-action="training"]`).MustClick()
	page.MustWait(`()=>fixture.inspect().round?.training===true`)
	page.MustEval(`()=>fixture.advance(960,{plunger:1})`)
	page.MustEval(`()=>fixture.advance(240,{})`)
	if !page.MustEval(`()=>{const s=fixture.inspect();return s.round.phase==='play'&&s.stats.balls===1&&s.physics.balls.length>0&&s.physics.balls.every(b=>[b.x,b.y,b.h,b.vx,b.vy].every(Number.isFinite))}`).Bool() {
		t.Fatalf("physical training launch failed: %s", page.MustEval(`()=>JSON.stringify(fixture.inspect().round)`).Str())
	}
	// The ball cam is the default; the camera key cycles follow -> play -> top -> follow.
	camMode := `()=>fixture.internals().scene.three.rig.mode`
	if got := page.MustEval(camMode).Str(); got != "follow" {
		t.Fatalf("default camera = %q, want follow", got)
	}
	for _, want := range []string{"play", "top", "follow"} {
		page.Keyboard.MustType(input.KeyC)
		page.MustWait(`()=>fixture.internals().scene.three.rig.mode==='` + want + `'`)
	}
	page.MustEval(`()=>new Promise(resolve=>requestAnimationFrame(()=>{const end=performance.now()+300;while(performance.now()<end);requestAnimationFrame(()=>requestAnimationFrame(resolve))}))`)
	if !page.MustEval(`()=>{const s=fixture.inspect();return s.paused===false&&s.panel===''}`).Bool() {
		t.Fatal("a 300 ms busy frame paused the game")
	}
	page.MustEval(`()=>fixtureActive=false`)
	page.MustWait(`()=>fixture.inspect().paused===true`)
	page.MustEval(`()=>fixtureActive=true`)
	if !page.MustEval(`()=>fixture.inspect().paused===true`).Bool() {
		t.Fatal("focus regain resumed without acknowledgement")
	}
	page.MustElement(`[data-action="settings"]`).MustClick()
	page.MustEval(`()=>{for(const [k,v] of [['effects','0'],['music','0.2']]){const e=document.querySelector('[data-setting="'+k+'"]');e.value=v;e.dispatchEvent(new Event('input',{bubbles:true}))}}`)
	if !page.MustEval(`()=>{const s=JSON.parse(localStorage.getItem('aurago.desktop.pinball.settings.v1'));return s.effects===0&&s.music===0.2}`).Bool() {
		t.Fatal("zero volume was lost")
	}
	page.MustElement(`[data-action="bind-keys-left"]`).MustClick()
	page.Keyboard.MustType(input.KeyK)
	page.MustWait(`()=>fixture.inspect().settings.keys.left==='KeyK'&&fixture.inspect().panel==='settings'`)
	page.MustElement(`[data-action="back"]`).MustClick()
	page.MustElement(`[data-action="resume"]`).MustClick()
	if err := page.Keyboard.Press(input.KeyK); err != nil {
		t.Fatal(err)
	}
	page.MustEval(`()=>fixture.advance(48)`)
	if !page.MustEval(`()=>fixture.inspect().physics.flippers[0].angle>0&&fixture.inspect().physics.flippers[0].up`).Bool() {
		t.Fatal("physical left flipper did not move")
	}
	if err := page.Keyboard.Release(input.KeyK); err != nil {
		t.Fatal(err)
	}
	page.MustEval(`()=>{document.body.dataset.theme='fruity';document.getElementById('host').style.cssText='width:1024px;height:768px'}`)
	page.MustWait(`()=>fixture.inspect().graphics.width===1024&&fixture.inspect().graphics.height===768`)
	page.MustElement(`[data-action="fullscreen"]`).MustClick()
	page.MustWait(`()=>!!document.fullscreenElement`)
	page.MustEval(`()=>document.exitFullscreen()`)
	page.MustEval(`()=>fixture.dispose()`)
	if !page.MustEval(`()=>{const s=fixture.inspect();return document.querySelectorAll('.pb-app,.pb-canvas').length===0&&!!s}`).Bool() {
		t.Fatal("window resources survived disposal")
	}
	page.MustNavigate(server.URL + "/fixture?keep=1")
	page.MustWait(`()=>window.fixture?.inspect().panel==='lobby'`)
	if !page.MustEval(`()=>{const s=fixture.inspect().settings;return s.effects===0&&s.music===0.2&&s.keys.left==='KeyK'}`).Bool() {
		t.Fatal("settings did not survive a reload")
	}
	page.MustEval(`()=>fixture.dispose()`)
	for _, fault := range []string{"webgl", "assets"} {
		page.MustNavigate(server.URL + "/fixture?fault=" + fault)
		page.MustWait(`()=>window.fixture?.inspect().panel==='error'`)
		if !page.MustEval(`()=>!!document.querySelector('[data-action="retry"]')&&document.querySelectorAll('.pb-canvas').length===0&&!document.querySelector('.pb-panel').textContent.includes('pinball.')&&document.querySelector('[data-action="retry"]').textContent.trim().length>0&&document.querySelector('[data-action="retry"]').textContent!=='retry'`).Bool() {
			t.Fatalf("%s error did not show a translated retry view", fault)
		}
		page.MustEval(`()=>fixture.dispose()`)
	}
}
