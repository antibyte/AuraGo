package ui

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
)

func galaxaArcadeServer(t *testing.T) *httptest.Server {
	t.Helper()
	loader := readDesktopAssetText(t, "js/desktop/core/module-loader.js")
	section := strings.Split(strings.Split(loader, "'galaxa-deluxe': {")[1], "prefetch:")[0]
	var scripts strings.Builder
	for _, path := range regexp.MustCompile(`/js/desktop/apps/galaxa-[a-z-]+\.js`).FindAllString(section, -1) {
		fmt.Fprintf(&scripts, `<script src="%s"></script>`, path)
	}
	words := readDesktopAssetText(t, "lang/desktop/en.json")
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(Content)))
	mux.HandleFunc("/fixture", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<html><meta charset="utf-8"><link rel="stylesheet" href="/css/galaxa-deluxe.css"><style>body{margin:0;background:#030817}#host{width:600px;height:800px;margin:auto}</style><div id="host"></div><script>window.errors=[];onerror=(m,s,l,c,e)=>errors.push(e?.stack||m);onunhandledrejection=e=>errors.push(String(e.reason));</script>`+scripts.String()+`<script>
const words=`+words+`;window.active=true;window.mount=()=>GalaxaDeluxe.render(document.querySelector('#host'),'test',{t:k=>words[k]||k,api:async()=>[],isActive:()=>active,onReady:c=>window.game=c});mount();
</script></html>`)
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return server
}

func TestGalaxaArcadeSoak(t *testing.T) {
	seconds, _ := strconv.Atoi(os.Getenv("AURAGO_GALAXA_SOAK_SECONDS"))
	if seconds <= 0 {
		t.Skip("set AURAGO_GALAXA_SOAK_SECONDS=1800 for acceptance")
	}
	bin, ok := browserExecutable()
	if !ok {
		t.Fatal("browser unavailable")
	}
	deadline, cancel := context.WithTimeout(context.Background(), time.Duration(seconds+120)*time.Second)
	defer cancel()
	u, err := launcher.New().Context(deadline).Bin(bin).Headless(true).NoSandbox(true).Set("disable-dev-shm-usage").Set("disable-gpu").Launch()
	if err != nil {
		t.Fatal(err)
	}
	browser := rod.New().ControlURL(u).NoDefaultDevice().MustConnect()
	defer browser.Close()
	server := galaxaArcadeServer(t)
	page := browser.MustPage(server.URL + "/fixture")
	defer page.Close()
	page.MustWaitLoad()
	waitForJSBool(t, page, `()=>game?.G.st==='TITLE' || errors.length>0`)
	if errs := page.MustEval(`()=>errors`).String(); errs != "[]" {
		t.Fatal(errs)
	}
	page.MustElement("canvas").MustClick()
	page.MustEval(`()=>{
 const c=game,g=c.G;c.settings.mode='classic';g.kb.s=true;
 window.soak={frames:0,started:performance.now(),maxParticles:0,maxBullets:0,maxSprites:0,stages:new Set(),samples:[],last:performance.now()};
 window.soakTick=()=>{
  const q=soak;q.frames++;q.stages.add(g.stage);q.maxParticles=Math.max(q.maxParticles,g.part.length);q.maxBullets=Math.max(q.maxBullets,g.bul.length+g.ebul.length);q.maxSprites=Math.max(q.maxSprites,c.spriteAtlasCache.size);
  g.p.inv=10000;g.kb.s=g.st==='TITLE';g.kb.f=true;
  if(g.evoChoiceOpen)c.selectEvolution(0);
  if(g.st==='SHOP')c.closeShop();if(g.st==='LOOP_CLEAR')c.openShop();
  const target=g.encounterBoss?.st!=='DEAD'&&g.encounterBoss || g.enemies.filter(e=>e.st!=='DEAD'&&e.y>40&&e.y<560).sort((a,b)=>b.y-a.y)[0];
  g.kb.l=!!target&&target.x<g.p.x-3;g.kb.r=!!target&&target.x>g.p.x+3;
  q.id=requestAnimationFrame(soakTick);
 };requestAnimationFrame(soakTick);
}`)
	started := time.Now()
	for time.Since(started) < time.Duration(seconds)*time.Second {
		time.Sleep(10 * time.Second)
		value := page.MustEval(`()=>{const now=performance.now(),q=soak;const result={elapsed:(now-q.started)/1000,fps:q.frames/((now-q.last)/1000),stage:game.G.stage,state:game.G.st,particles:q.maxParticles,bullets:q.maxBullets,sprites:q.maxSprites,audio:game.audioStats(),heap:performance.memory?.usedJSHeapSize,dom:document.querySelectorAll('*').length,errors};q.frames=0;q.last=now;q.samples.push(result);return result}`)
		if time.Since(started)%time.Minute < 11*time.Second {
			t.Log(value.String())
		}
		if page.MustEval(`()=>errors.length>0 || game.audioStats().effects>32 || game.spriteAtlasCache.size>768`).Bool() {
			t.Fatal(value.String())
		}
	}
	data := page.MustEval(`()=>JSON.stringify({samples:soak.samples,stages:[...soak.stages],browser:navigator.userAgent,renderer:(()=>{const gl=document.createElement('canvas').getContext('webgl');return gl?.getParameter(gl.RENDERER)})()})`).Str()
	if dir := os.Getenv("AURAGO_BROWSER_ARTIFACT_DIR"); dir != "" {
		os.MkdirAll(dir, 0755)
		os.WriteFile(filepath.Join(dir, "galaxa-soak.json"), []byte(data), 0644)
		os.WriteFile(filepath.Join(dir, "galaxa-soak.png"), page.MustScreenshot(), 0644)
	}
	page.MustEval(`()=>{cancelAnimationFrame(soak.id);GalaxaDeluxe.dispose('test')}`)
	if !page.MustEval(`()=>game.state.disposed && game.audioStats().effects===0 && game.audioStats().music===0`).Bool() {
		t.Fatal("soak cleanup failed")
	}
	t.Logf("Soak completed: %s", time.Since(started))
	if seconds >= 1800 && page.MustEval(`()=>{const fps=soak.samples.map(s=>s.fps).sort((a,b)=>a-b);return fps[Math.floor(fps.length/2)]}`).Num() < 55 {
		t.Fatal("30 minute median frame rate below 55 FPS")
	}
}

func TestGalaxaArcadeBrowser(t *testing.T) {
	if os.Getenv("AURAGO_RUN_BROWSER_SMOKE") != "1" {
		t.Skip("set AURAGO_RUN_BROWSER_SMOKE=1")
	}
	server := galaxaArcadeServer(t)
	page := newSmokeBrowser(t).MustPage(server.URL + "/fixture").Timeout(60 * time.Second)
	defer page.Close()
	page.MustWaitLoad()
	waitForJSBool(t, page, `()=>window.errors.length>0 || window.game?.G.st==='TITLE'`)
	if errs := page.MustEval(`()=>errors`).String(); errs != "[]" {
		t.Fatal(errs)
	}
	page.MustEval(`()=>{cancelAnimationFrame(game.rafId); game.settings.mute=true; game.G.muted=true; game.GalagaMusic.stop(); game.G.kb.s=true; game.stepFrame(1/60);game.G.kb.s=false;game.stepFrame(1/60);}`)
	if state := page.MustEval(`()=>game.G.st`).Str(); state != "STAGE_INTRO" {
		t.Fatalf("start state: %s", state)
	}
	page.MustEval(`()=>{for(let i=0;i<180;i++)game.stepFrame(1/60);game.renderFrame();}`)
	if errs := page.MustEval(`()=>errors`).String(); errs != "[]" {
		t.Fatal(errs)
	}
	if dir := os.Getenv("AURAGO_BROWSER_ARTIFACT_DIR"); dir != "" {
		os.MkdirAll(dir, 0755)
		os.WriteFile(filepath.Join(dir, "galaxa-nebula.png"), page.MustScreenshot(), 0644)
	}
	t.Log(page.MustEval(`()=>({state:game.G.st,enemies:game.G.enemies.length,assets:Object.keys(game.spriteAssets.manifest.animations).length,browser:navigator.userAgent})`).String())
	result := page.MustEval(`()=>{
 const c=game,g=c.G, stages=[],seen=new Set();c.seedRun(91421);g.kb.f=true;
 for(let i=0;i<60*4000;i++){
  if(!seen.has(g.stage)){seen.add(g.stage);stages.push({stage:g.stage,time:g.simTime});}
  if(g.stage===31)return {ok:true,stages,score:g.score};
  if(g.st==='SHOP'){c.closeShop();continue;}
  if(g.st==='LOOP_CLEAR'){c.openShop();continue;}
  if(g.evoChoiceOpen)c.selectEvolution(0);
  g.p.inv=10000;
  const targets=g.enemies.filter(e=>e.st!=='DEAD'&&e.y>40&&e.y<560);
  const target=g.encounterBoss?.st!=='DEAD'&&g.encounterBoss || targets.sort((a,b)=>b.y-a.y)[0];
  if(target){g.kb.l=target.x<g.p.x-3;g.kb.r=target.x>g.p.x+3;}else{g.kb.l=false;g.kb.r=false;}
  g.kb.f=true;c.stepFrame(1/60);
  if(i%60===0)c.renderFrame();
 }
 return {ok:false,stages,state:g.st,stage:g.stage,alive:g.enemies.filter(e=>e.st!=='DEAD'),time:g.simTime,bul:g.bul.length,waves:g.waveIndex};
}`).String()
	t.Log(result)
	if !strings.Contains(result, "ok:true") {
		t.Fatal("campaign did not reach loop 2")
	}
}

func TestGalaxaArcadeBehaviorBrowser(t *testing.T) {
	if os.Getenv("AURAGO_RUN_BROWSER_SMOKE") != "1" {
		t.Skip("set AURAGO_RUN_BROWSER_SMOKE=1")
	}
	server := galaxaArcadeServer(t)
	page := newSmokeBrowser(t).MustPage(server.URL + "/fixture").Timeout(120 * time.Second)
	defer page.Close()
	page.MustWaitLoad()
	waitForJSBool(t, page, `()=>game?.G.st==='TITLE' || errors.length>0`)
	script, err := os.ReadFile("testdata/galaxa-arcade-checks.js")
	if err != nil {
		t.Fatal(err)
	}
	result, err := page.Eval(string(script))
	if err != nil {
		t.Fatal(err)
	}
	t.Log(result.Value.String())
}

func TestGalaxaArcadeAudioBrowser(t *testing.T) {
	if os.Getenv("AURAGO_RUN_BROWSER_SMOKE") != "1" {
		t.Skip("set AURAGO_RUN_BROWSER_SMOKE=1")
	}
	server := galaxaArcadeServer(t)
	page := newSmokeBrowser(t).MustPage(server.URL + "/fixture").Timeout(120 * time.Second)
	defer page.Close()
	page.MustWaitLoad()
	waitForJSBool(t, page, `()=>game?.G.st==='TITLE' || errors.length>0`)
	page.MustEval(`()=>{cancelAnimationFrame(game.rafId);game.GalagaMusic.stop();}`)
	script, err := os.ReadFile("testdata/galaxa-audio-checks.js")
	if err != nil {
		t.Fatal(err)
	}
	result, err := page.Eval(string(script))
	if err != nil {
		t.Fatal(err)
	}
	t.Log(result.Value.String())
	if dir := os.Getenv("AURAGO_BROWSER_ARTIFACT_DIR"); dir != "" {
		os.MkdirAll(dir, 0755)
		data, err := base64.StdEncoding.DecodeString(page.MustEval(`()=>audioWav`).Str())
		if err != nil {
			t.Fatal(err)
		}
		os.WriteFile(filepath.Join(dir, "galaxa-sfx-review.wav"), data, 0644)
	}
	page.MustEval(`async()=>{game.resumeAudio();await game.actx.suspend()}`)
	page.MustElement("canvas").MustClick()
	waitForJSBool(t, page, `()=>game.actx.state==='running'`)
	page.MustEval(`()=>{game.settings.mode='classic';game.G.demoMode=false;game.MusicEngine.play('nebula')}`)
	waitForJSBool(t, page, `()=>game.audioStats().music>0`)
	page.MustEval(`()=>game.MusicEngine.setPaused(true)`)
	if page.MustEval(`()=>game.audioStats().music`).Int() != 0 {
		t.Fatal("pause retained music voices")
	}
	page.MustEval(`()=>GalaxaDeluxe.dispose('test')`)
}

func TestGalaxaArcadeVisualBrowser(t *testing.T) {
	if os.Getenv("AURAGO_RUN_BROWSER_SMOKE") != "1" {
		t.Skip("set AURAGO_RUN_BROWSER_SMOKE=1")
	}
	server := galaxaArcadeServer(t)
	browser := newSmokeBrowser(t).NoDefaultDevice()
	page := browser.MustPage(server.URL + "/fixture").Timeout(90 * time.Second)
	defer page.Close()
	version, _ := browser.Version()
	t.Logf("Browser: %+v", version)
	page.MustWaitLoad()
	waitForJSBool(t, page, `()=>game?.G.st==='TITLE' || errors.length>0`)
	page.MustEval(`()=>{cancelAnimationFrame(game.rafId);game.GalagaMusic.stop();game.settings.crt=false;game.wrapEl.classList.remove('galaxa-crt');game.G.muted=true;}`)
	dir := os.Getenv("AURAGO_BROWSER_ARTIFACT_DIR")
	if dir != "" {
		os.MkdirAll(dir, 0755)
	}
	for _, view := range []struct {
		name string
		w, h int
		dpr  float64
	}{{"desktop", 1140, 900, 1}, {"fullscreen", 1920, 1080, 2}, {"compact", 360, 520, 1}, {"compact-dpr2", 360, 520, 2}} {
		if err := (proto.EmulationSetDeviceMetricsOverride{Width: view.w, Height: view.h, DeviceScaleFactor: view.dpr, Mobile: false}).Call(page); err != nil {
			t.Fatal(err)
		}
		page.MustEval(`()=>{const host=document.querySelector('#host');host.style.width='100vw';host.style.height='100vh';game.resize();game.G.st='TITLE';game.renderFrame()}`)
		if !page.MustEval(`()=>{const r=game.canvas.getBoundingClientRect();return r.width>0&&r.height>0&&r.left>=0&&r.top>=0&&r.right<=innerWidth+.5&&r.bottom<=innerHeight+.5&&game.canvas.width===Math.round(r.width*devicePixelRatio)}`).Bool() {
			t.Fatal("canvas clipping in " + view.name)
		}
		if dir != "" {
			os.WriteFile(filepath.Join(dir, "galaxa-title-"+view.name+".png"), page.MustScreenshot(), 0644)
		}
	}
	(proto.EmulationSetDeviceMetricsOverride{Width: 1140, Height: 900, DeviceScaleFactor: 1}).Call(page)
	for _, state := range []string{"settings", "shop", "nebula", "asteroid", "crystal", "storm", "blackhole", "void"} {
		page.MustEval(`name=>{const c=game,g=c.G;c.settings.mode='classic';c.resize();c.resetRun(99);g.stage=Math.max(1,(GalaxaCore.SECTORS.findIndex(s=>s.id===name)+1)*5);c.startStage();g.st='PLAYING';if(g.encounterBoss){const e=g.encounterBoss;e.x=270;e.y=165;e.bossState='windup';e.bossPhase=3;e.targetX=270;e.safeLane=3;e.telegraph={x:270};g.animTime=320;g.superMeter=100;g.ebul=Array.from({length:15},(_,i)=>({x:65+i*29,y:350+(i%3)*60,kind:'ion'}));}if(name==='settings')g.st='SETTINGS';if(name==='shop')c.openShop();c.renderFrame()}`, state)
		if dir != "" {
			os.WriteFile(filepath.Join(dir, "galaxa-"+state+".png"), page.MustScreenshot(), 0644)
		}
	}
	page.MustEval(`()=>{const c=game,canvas=document.createElement('canvas');canvas.width=960;canvas.height=2040;const d=canvas.getContext('2d');d.fillStyle='#081322';d.fillRect(0,0,960,2040);d.imageSmoothingEnabled=false;const names=['player.classic.idle','player.interceptor.idle','player.heavy.idle','player.stealth.idle','bee.idle','butterfly.idle','stalker.idle','sniper.idle','hunter.idle','spinner.idle','bomber.idle','lasher.idle','weaver.idle','splitter.idle','shield_bee.idle','kamikaze.idle','carrier.idle','teleporter.idle','boss.idle','miniboss.idle'];names.forEach((key,row)=>{d.fillStyle='#cee3f5';d.font='14px monospace';d.fillText(key,10,row*100+18);for(let f=0;f<8;f++)c.drawAnimation(d,key,70+f*116,row*100+62,f*c.spriteAssets.manifest.animations[key].ms,64)});window.contactSheet=canvas.toDataURL('image/png').split(',')[1]}`)
	if dir != "" {
		data, _ := base64.StdEncoding.DecodeString(page.MustEval(`()=>contactSheet`).Str())
		os.WriteFile(filepath.Join(dir, "galaxa-animation-contact.png"), data, 0644)
	}
	if errs := page.MustEval(`()=>errors`).String(); errs != "[]" {
		t.Fatal(errs)
	}
}

func TestGalaxaArcadeInputBrowser(t *testing.T) {
	if os.Getenv("AURAGO_RUN_BROWSER_SMOKE") != "1" {
		t.Skip("set AURAGO_RUN_BROWSER_SMOKE=1")
	}
	server := galaxaArcadeServer(t)
	page := newSmokeBrowser(t).NoDefaultDevice().MustPage()
	defer page.Close()
	page.MustEvalOnNewDocument(`
 Object.defineProperty(navigator,'maxTouchPoints',{get:()=>2});
 const nativeFetch=window.fetch;let failed=false;window.fetch=(url,...args)=>{if(String(url).includes('atlas.json')&&!failed){failed=true;return Promise.resolve(new Response('',{status:503}));}return nativeFetch(url,...args)};
 `)
	page.MustNavigate(server.URL + "/fixture").MustWaitLoad()
	waitForJSBool(t, page, `()=>document.querySelector('.galaxa-overlay button')!==null`)
	page.MustElement(".galaxa-overlay button").MustClick()
	waitForJSBool(t, page, `()=>game.G.st==='TITLE'`)
	page.MustEval(`()=>{cancelAnimationFrame(game.rafId);game.GalagaMusic.stop();game.renderFrame()}`)
	result, err := page.Evaluate(rod.Eval(`() => {
 const assert=(ok,msg)=>{if(!ok)throw Error(msg)},c=game,g=c.G;
 assert(document.querySelectorAll('.galaxa-touch-actions button').length===2,'missing touch actions');
 assert(c.touchActions.hidden,'touch controls cover title');
 const point=(x,y)=>{const r=c.canvas.getBoundingClientRect();c.canvas.dispatchEvent(new PointerEvent('pointerup',{bubbles:true,clientX:r.left+x*r.width/540,clientY:r.top+y*r.height/720}))};
 point(210,230);assert(c.settings.ship==='interceptor','ship card selection');
 point(270,420);assert(g.st==='STAGE_INTRO','touch start');
 g.st='PLAYING';g.p.inv=1e6;c.renderFrame();assert(!c.touchActions.hidden,'touch actions hidden in play');
 const r=c.canvas.getBoundingClientRect(),touch=(type,x,y)=>{const item=new Touch({identifier:1,target:c.canvas,clientX:r.left+x*r.width/540,clientY:r.top+y*r.height/720});c.canvas.dispatchEvent(new TouchEvent(type,{bubbles:true,cancelable:true,changedTouches:[item],touches:type==='touchend'?[]:[item]}))};
 const before=g.p.x;touch('touchstart',100,600);touch('touchmove',180,600);c.stepFrame(.1);assert(g.p.x>before,'touch joystick');touch('touchend',180,600);assert(!g.kb.r,'touch release');
 const pads=[{axes:[-1,0],buttons:Array.from({length:16},()=>({pressed:false}))}];Object.defineProperty(navigator,'getGamepads',{configurable:true,value:()=>pads});
 pads[0].buttons[5].pressed=true;c.stepFrame(1/60);assert(g.parryActive>0&&g.inp.l,'gamepad move/parry');pads[0].buttons[5].pressed=false;g.superMeter=100;pads[0].buttons[3].pressed=true;c.stepFrame(1/60);assert(g.superPhase!=='idle','gamepad super');
 window.dispatchEvent(new Event('blur'));assert(g.st==='PAUSED'&&!g.kb.r,'focus pause');
 window.active=false;c.pollGP();assert(Object.values(g.gp).every(v=>!v),'inactive gamepad consumed input');window.active=true;
 Object.defineProperty(navigator,'getGamepads',{value:()=>[]});
 g.st='PLAYING';g.evoChoiceOpen=true;const clock=g.simTime;c.stepFrame(.2);assert(g.simTime===clock,'evolution advanced game clock');g.evoChoiceOpen=false;g.hitstopT=100;c.stepFrame(.05);assert(g.simTime===clock,'hitstop advanced game clock');
 c.settings.adaptiveMusic=false;c.modulateMusic('combo',40);c.modulateMusic('bossPhase',3);assert(c.MusicEngine.tempoMult===1&&c.MusicEngine.semitoneOffset===0,'adaptive music off ignored');
 assert(errors.length===0,errors.join('\n'));return 'asset retry, pointer menus, touch joystick, gamepad, focus and frozen clock';
 }`))
	if err != nil {
		t.Fatal(err)
	}
	t.Log(result.Value.String())
}

func TestGalaxaArcadePerformanceBrowser(t *testing.T) {

	if os.Getenv("AURAGO_RUN_BROWSER_SMOKE") != "1" {
		t.Skip("set AURAGO_RUN_BROWSER_SMOKE=1")
	}
	server := galaxaArcadeServer(t)
	page := newSmokeBrowser(t).NoDefaultDevice().MustPage(server.URL + "/fixture")
	defer page.Close()
	page.MustWaitLoad()
	waitForJSBool(t, page, `()=>game.G.st==='TITLE'`)
	result, err := page.Evaluate(rod.Eval(`async()=>{
 cancelAnimationFrame(game.rafId);game.GalagaMusic.stop();game.resetRun(91);game.G.stage=30;game.startStage();game.G.st='PLAYING';game.G.p.inv=1e8;
 const times=[];for(let n=0;n<3600;n++){const start=performance.now();game.G.kb.f=true;game.stepFrame(1/60);game.renderFrame();times.push(performance.now()-start)}
 times.sort((a,b)=>a-b);
 let start=performance.now(),frames=0;await new Promise(resolve=>{const tick=()=>{frames++;if(performance.now()-start>=5000)resolve();else requestAnimationFrame(tick)};requestAnimationFrame(tick)});
 const idleFps=frames*1000/(performance.now()-start);
 start=performance.now();frames=0;await new Promise(resolve=>{const tick=()=>{frames++;game.stepFrame(1/60);game.renderFrame();if(performance.now()-start>=10000)resolve();else requestAnimationFrame(tick)};requestAnimationFrame(tick)});
 return {mean:times.reduce((a,b)=>a+b)/times.length,p95:times[Math.floor(times.length*.95)],p99:times[Math.floor(times.length*.99)],idleFps,gameFps:frames*1000/(performance.now()-start),browser:navigator.userAgent}
}`).ByPromise())
	if err != nil {
		t.Fatal(err)
	}
	t.Log(result.Value.String())
	if dir := os.Getenv("AURAGO_BROWSER_ARTIFACT_DIR"); dir != "" {
		os.WriteFile(filepath.Join(dir, "galaxa-performance.json"), []byte(result.Value.JSON("", "  ")), 0644)
	}
}

func TestDesktopGalaxaArcadeBrowser(t *testing.T) {
	if os.Getenv("AURAGO_RUN_BROWSER_SMOKE") != "1" {
		t.Skip("set AURAGO_RUN_BROWSER_SMOKE=1")
	}
	html := readDesktopAssetText(t, "desktop.html")
	html = regexp.MustCompile(`(?s)<script\b[^>]*>.*?</script>`).ReplaceAllString(html, "")
	html = regexp.MustCompile(`\{\{[^}]*\}\}`).ReplaceAllString(html, "")
	html = strings.Replace(html, "</head>", `<link rel="stylesheet" href="/css/galaxa-deluxe.css"><style>body{margin:0}.vd-shell{height:100vh}</style></head>`, 1)
	loader := readDesktopAssetText(t, "js/desktop/core/module-loader.js")
	section := strings.Split(strings.Split(loader, "'galaxa-deluxe': {")[1], "prefetch:")[0]
	var scripts strings.Builder
	for _, path := range regexp.MustCompile(`/js/desktop/apps/galaxa-[a-z-]+\.js`).FindAllString(section, -1) {
		fmt.Fprintf(&scripts, `<script src="%s"></script>`, path)
	}
	html = strings.Replace(html, "</body>", `<script src="/galaxa-shell.js"></script>`+scripts.String()+`<script src="/js/desktop/apps/calculator.js"></script><script src="/galaxa-fixture.js"></script></body>`, 1)
	shell := readDesktopAssetText(t, "js/desktop/bundles/main.bundle.js")
	cut := strings.LastIndex(shell, "    ensureDesktopRadialMenuAnchor();")
	if cut < 0 {
		t.Fatal("desktop startup seam missing")
	}
	shell = shell[:cut] + `window.galaxaShell={state,openApp,loadIconManifest,closeWindow,minimizeWindow,focusWindow,toggleMaximizeWindow};})();`
	fixture := `
 window.fixtureErrors=[];window.contexts=[];
 addEventListener('error',e=>fixtureErrors.push(e.message));addEventListener('unhandledrejection',e=>fixtureErrors.push(String(e.reason)));
 const nativeFetch=window.fetch.bind(window);window.fetch=(url,options)=>String(url).includes('/api/')?Promise.resolve(new Response(JSON.stringify(String(url).includes('highscore')?[]:{}),{headers:{'Content-Type':'application/json'}})):nativeFetch(url,options);
 const originalRender=GalaxaDeluxe.render;GalaxaDeluxe.render=(host,id,context)=>originalRender(host,id,{...context,onReady:c=>{window.game=c;contexts.push(c)}});
 window.fixtureReady=(async()=>{
 const words=await(await nativeFetch('/lang/desktop/de.json')).json();window.i18n={t:key=>words[key]||key};window.t=key=>words[key]||key;
 galaxaShell.state.bootstrap={enabled:true,builtin_apps:[{id:'galaxa-deluxe',name:'Galaxa Deluxe',icon:'galaxa'},{id:'calculator',name:'Calculator',icon:'calculator'}],apps:[],widgets:[],shortcuts:[],desktop_files:[],settings:{'appearance.theme':'standard','windows.restore_session':false}};
 document.body.dataset.theme='standard';document.body.dataset.animations='false';document.getElementById('vd-disabled').hidden=true;
 await galaxaShell.loadIconManifest();galaxaShell.openApp('galaxa-deluxe');
 })();
 `
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(Content)))
	for name, value := range map[string]string{"/fixture": html, "/galaxa-shell.js": shell, "/galaxa-fixture.js": fixture} {
		mux.HandleFunc(name, func(w http.ResponseWriter, r *http.Request) {
			if name == "/fixture" {
				w.Header().Set("Content-Type", "text/html")
			} else {
				w.Header().Set("Content-Type", "text/javascript")
			}
			fmt.Fprint(w, value)
		})
	}
	server := httptest.NewServer(mux)
	defer server.Close()
	page := newSmokeBrowser(t).NoDefaultDevice().MustPage(server.URL + "/fixture").Timeout(90 * time.Second)
	defer page.Close()
	page.MustSetViewport(1140, 900, 1, false)
	page.MustWaitLoad()
	waitForJSBool(t, page, `()=>window.game?.G.st==='TITLE'||fixtureErrors.length>0`)
	if errors := page.MustEval(`()=>fixtureErrors`).String(); errors != "[]" {
		t.Fatal(errors)
	}
	page.MustEval(`()=>{game.G.kb.s=true;game.stepFrame(1/60);game.G.kb.s=false;game.G.p.inv=1e8}`)
	waitForJSBool(t, page, `()=>game.G.st==='PLAYING'`)
	if dir := os.Getenv("AURAGO_BROWSER_ARTIFACT_DIR"); dir != "" {
		os.MkdirAll(dir, 0755)
		os.WriteFile(filepath.Join(dir, "galaxa-real-desktop.png"), page.MustScreenshot(), 0644)
	}
	page.MustEval(`()=>{window.galaxaID=galaxaShell.state.activeWindowId;galaxaShell.openApp('calculator')}`)
	waitForJSBool(t, page, `()=>game.G.st==='PAUSED'`)
	if !page.MustEval(`()=>{const x=game.G.p.x;document.dispatchEvent(new KeyboardEvent('keydown',{key:'ArrowRight',bubbles:true}));game.stepFrame(.1);return game.G.p.x===x&&!game.G.kb.r}`).Bool() {
		t.Fatal("background game consumed keyboard")
	}
	page.MustEval(`()=>{galaxaShell.focusWindow(galaxaID);game.G.kb.p=true;game.stepFrame(1/60);game.G.kb.p=false;galaxaShell.toggleMaximizeWindow(galaxaID)}`)
	waitForJSBool(t, page, `()=>game.G.st==='PLAYING'`)
	if !page.MustEval(`()=>{const r=game.canvas.getBoundingClientRect(),h=game.wrapEl.getBoundingClientRect();return r.left>=h.left-.5&&r.top>=h.top-.5&&r.right<=h.right+.5&&r.bottom<=h.bottom+.5}`).Bool() {
		t.Fatal("maximized game clipped")
	}
	for i := 0; i < 5; i++ {
		page.MustEval(`()=>{galaxaShell.closeWindow(galaxaID);galaxaShell.openApp('galaxa-deluxe');window.galaxaID=galaxaShell.state.activeWindowId}`)
		waitForJSBool(t, page, `()=>game.G.st==='TITLE'`)
	}
	if !page.MustEval(`()=>contexts.slice(0,-1).every(c=>c.state.disposed&&c.audioStats().music===0&&c.audioStats().effects===0)`).Bool() {
		t.Fatal("repeated desktop close leaked a game")
	}
	if errors := page.MustEval(`()=>fixtureErrors`).String(); errors != "[]" {
		t.Fatal(errors)
	}
}

func TestGalaxaArcadeMusicReview(t *testing.T) {
	if os.Getenv("AURAGO_GALAXA_AUDIO_REVIEW") != "1" {
		t.Skip("set AURAGO_GALAXA_AUDIO_REVIEW=1 for synthesized music review")
	}
	server := galaxaArcadeServer(t)
	page := newSmokeBrowser(t).NoDefaultDevice().MustPage(server.URL + "/fixture").Timeout(140 * time.Second)
	defer page.Close()
	page.MustWaitLoad()
	waitForJSBool(t, page, `()=>game.G.st==='TITLE'`)
	page.MustElement("canvas").MustClick()
	result, err := page.Eval(`async()=>{
 const c=game;cancelAnimationFrame(c.rafId);c.GalagaMusic.stop();c.G.st='PLAYING';c.G.demoMode=false;c.G.muted=false;
 c.settings.vol=100;c.settings.musicVol=100;c.settings.sfxVol=100;c.G.vol=1;c.resumeAudio();await c.actx.resume();c.applyAudioSettings();
 const stream=c.actx.createMediaStreamDestination();c.masterBus.connect(stream);
 const analyzer=c.actx.createAnalyser();analyzer.fftSize=2048;c.masterBus.connect(analyzer);const samples=new Float32Array(analyzer.fftSize);let peak=0,clipped=0,frames=0;
 const measure=setInterval(()=>{analyzer.getFloatTimeDomainData(samples);for(const n of samples){peak=Math.max(peak,Math.abs(n));if(Math.abs(n)>=.999)clipped++;}frames++},20);
 const chunks=[],recorder=new MediaRecorder(stream.stream,{mimeType:'audio/webm;codecs=opus'});recorder.ondataavailable=e=>chunks.push(e.data);recorder.start();
 const timeline=[];
 for(const sector of GalaxaCore.SECTORS){
  for(const boss of [false,true]){const theme=sector.id+(boss?'_boss':'');timeline.push({theme,at:c.actx.currentTime});c.MusicEngine.stop();c.G.biome=sector.id;c.MusicEngine.play(theme);c.MusicEngine.setIntensity(boss?8:3);await new Promise(r=>setTimeout(r,6000));}
 }
 for(const theme of ['shop','challenge','victory','gameover']){timeline.push({theme,at:c.actx.currentTime});c.MusicEngine.stop();c.MusicEngine.play(theme);await new Promise(r=>setTimeout(r,4000));}
 c.MusicEngine.stop();clearInterval(measure);await new Promise(resolve=>{recorder.onstop=resolve;recorder.stop()});
 const bytes=new Uint8Array(await new Blob(chunks,{type:'audio/webm'}).arrayBuffer());let binary='';for(let i=0;i<bytes.length;i+=16384)binary+=String.fromCharCode(...bytes.subarray(i,i+16384));window.musicAudio=btoa(binary);
 analyzer.disconnect();c.masterBus.disconnect(analyzer);c.masterBus.disconnect(stream);stream.stream.getTracks().forEach(t=>t.stop());
 if(peak<=.005||peak>=.98||clipped)throw Error('invalid music mix '+peak+'/'+clipped);
 return {peak,clipped,frames,timeline};
}`)
	if err != nil {
		t.Fatal(err)
	}
	t.Log(result.Value.String())
	if dir := os.Getenv("AURAGO_BROWSER_ARTIFACT_DIR"); dir != "" {
		os.MkdirAll(dir, 0755)
		data, err := base64.StdEncoding.DecodeString(page.MustEval(`()=>musicAudio`).Str())
		if err != nil {
			t.Fatal(err)
		}
		os.WriteFile(filepath.Join(dir, "galaxa-music-review.webm"), data, 0644)
		os.WriteFile(filepath.Join(dir, "galaxa-music-review.json"), []byte(result.Value.JSON("", "  ")), 0644)
	}
}

func TestGalaxaArcadeAllFramesBrowser(t *testing.T) {
	dir := os.Getenv("AURAGO_BROWSER_ARTIFACT_DIR")
	if os.Getenv("AURAGO_RUN_BROWSER_SMOKE") != "1" || dir == "" {
		t.Skip("set AURAGO_RUN_BROWSER_SMOKE and AURAGO_BROWSER_ARTIFACT_DIR")
	}
	server := galaxaArcadeServer(t)
	page := newSmokeBrowser(t).NoDefaultDevice().MustPage(server.URL + "/fixture")
	defer page.Close()
	page.MustWaitLoad()
	waitForJSBool(t, page, `()=>game.G.st==='TITLE'`)
	page.MustEval(`()=>{cancelAnimationFrame(game.rafId);game.GalagaMusic.stop()}`)
	for _, sheet := range []string{"player-classic", "player-interceptor", "player-heavy", "player-stealth", "enemies-a", "enemies-b", "enemies-c", "enemies-d", "boss-nebula", "boss-asteroid", "boss-crystal", "boss-storm", "boss-blackhole", "boss-void", "effects", "items"} {
		encoded := page.MustEval(`sheet=>{
 const c=game,seen=new Set(),rows=[];
 for(const [key,a]of Object.entries(c.spriteAssets.manifest.animations)){
  if(a.sheet!==sheet||key.startsWith('icon.'))continue;const unique=JSON.stringify(a.frames);if(seen.has(unique))continue;seen.add(unique);
  for(let start=0;start<a.frames.length;start+=8)rows.push({key,a,start});
 }
 const canvas=document.createElement('canvas');canvas.width=1200;canvas.height=rows.length*150;const d=canvas.getContext('2d');d.fillStyle='#081322';d.fillRect(0,0,canvas.width,canvas.height);d.imageSmoothingEnabled=false;
 rows.forEach(({key,a,start},row)=>{d.fillStyle='#cee3f5';d.font='14px monospace';d.fillText(key+' / '+start,10,row*150+18);for(let f=start;f<Math.min(start+8,a.frames.length);f++)c.drawAnimation(d,key,75+(f-start)*150,row*150+86,f*a.ms,112)});
 return canvas.toDataURL('image/png').split(',')[1];
}`, sheet).Str()
		data, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(filepath.Join(dir, "frames-"+sheet+".png"), data, 0644); err != nil {
			t.Fatal(err)
		}
	}
}
