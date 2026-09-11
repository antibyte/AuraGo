package gamemaker

import (
	"encoding/json"
	"fmt"
	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPresentationBrowser(t *testing.T) {
	if os.Getenv("GAMEMAKER_PRESENTATION_BROWSER") != "1" {
		t.Skip("set GAMEMAKER_PRESENTATION_BROWSER=1 for rendered effects and audio checks")
	}
	bin := os.Getenv("CHROME_BIN")
	if bin == "" {
		bin = "C:/Program Files/Google/Chrome/Application/chrome.exe"
	}
	launch := launcher.New().Bin(bin).Headless(true).NoSandbox(true).Set("enable-unsafe-swiftshader")
	browser := rod.New().ControlURL(launch.MustLaunch()).MustConnect()
	defer launch.Cleanup()
	defer browser.Close()
	fx, err := readPresentationPack(EffectsPackID)
	if err != nil {
		t.Fatal(err)
	}
	audio, err := readPresentationPack(SoundsPackID)
	if err != nil {
		t.Fatal(err)
	}
	config, _ := json.Marshal(map[string]any{"effects": fx.Assets, "sounds": audio.Assets, "base": "/packs/aurago-sounds/", "bindings": []SoundBinding{{"shot", "rifle"}, {"hit", "impact-metal"}}, "quality": "high"})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if presentationStudioRoute(w, r) {
			return
		}
		switch {
		case r.URL.Path == "/":
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprint(w, presentationFixture)
		case r.URL.Path == "/config.json":
			w.Header().Set("Content-Type", "application/json")
			w.Write(config)
		case strings.HasPrefix(r.URL.Path, "/runtime/"):
			name := strings.TrimPrefix(r.URL.Path, "/runtime/")
			b, e := bundledAssetPackFile("runtime", name)
			if e != nil {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", "text/javascript")
			w.Write(b)
		case strings.HasPrefix(r.URL.Path, "/packs/"):
			parts := strings.SplitN(strings.TrimPrefix(r.URL.Path, "/packs/"), "/", 2)
			if len(parts) != 2 {
				http.NotFound(w, r)
				return
			}
			b, e := bundledPresentationFile(parts[0], parts[1])
			if e != nil {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", "audio/wav")
			w.Write(b)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	report := filepath.Join("..", "..", "reports", "game-maker-presentation")
	os.MkdirAll(report, 0750)
	for _, dim := range []string{"2d", "3d"} {
		t.Run(dim, func(t *testing.T) {
			page := browser.MustPage(server.URL + "/?dimension=" + dim).Timeout(90 * time.Second)
			defer page.Close()
			page.MustSetViewport(1920, 1080, 1, false)
			page.MustWaitLoad()
			page.MustWait(`() => window.fixtureReady || window.fixtureError || window.errors?.length`)
			if e := page.MustEval(`()=>JSON.stringify(window.errors)`).String(); e != "[]" {
				t.Fatal(e)
			}
			if e := page.MustEval(`()=>window.fixtureError||''`).String(); e != "" {
				t.Fatal(e)
			}
			page.MustElement("#test-audio").MustClick()
			if err := page.Timeout(20 * time.Second).Wait(rod.Eval(`()=>window.fx?.stats().buffers===40||window.fixtureError||window.errors?.length`)); err != nil {
				t.Fatal("audio timeout", page.MustEval(`()=>({state:window.fx?.stats(),errors:window.errors})`).String())
			}
			if e := page.MustEval(`()=>window.fixtureError||''`).String(); e != "" {
				t.Fatal(e)
			}
			if n := page.MustEval(`()=>window.fx.stats().buffers`).Int(); n != 40 {
				t.Fatalf("decoded %d sounds", n)
			}
			if !page.MustEval(`async()=>{const voice=await window.fx.audio.play('rifle',{position:[500,0,0],cooldown:0});window.fx.audio.listener([1000,0,0]);return voice&&(voice.pan.pan?voice.pan.pan.value<0:voice.pan.positionX.value===500)}`).Bool() {
				t.Fatal("spatial sound did not follow listener")
			}
			if !page.MustEval(`async()=>{await Promise.all(Array.from({length:50},()=>window.fx.audio.play('rifle',{cooldown:0})));return window.fx.stats().voices<=32}`).Bool() {
				t.Fatal("audio voice limit exceeded")
			}
			for _, env := range []string{"forest-rain", "coast", "space"} {
				page.MustEval(`id=>window.choose(id)`, env)
				page.MustWait(`()=>window.environmentFrames>45`)
				if dim == "2d" && env == "coast" {
					page.MustEval(`()=>{window.waterProbe=window.fixtureObject.scene.add.rectangle(960,900,80,40,0xff00ff);window.environmentFrames=0}`)
					page.MustWait(`()=>window.environmentFrames>3`)
					if !page.MustEval(`()=>new Promise(resolve=>window.fixtureObject.scene.game.renderer.snapshotPixel(960,900,c=>resolve(c.r>180&&c.g<60&&c.b>180)))`).Bool() {
						t.Fatal("coast water obscures default-depth game objects")
					}
					page.MustEval(`()=>window.waterProbe.destroy()`)
				}
				page.MustScreenshot(filepath.Join(report, dim+"-"+env+".png"))
			}
			for _, quality := range []string{"low", "medium", "high", "auto"} {
				page.MustEval(`q=>{window.choose('forest-rain');window.fx.setQuality(q)}`, quality)
				page.MustWait(`()=>window.environmentFrames>10`)
				page.MustScreenshot(filepath.Join(report, dim+"-quality-"+quality+".png"))
			}
			if !page.MustEval(`()=>{let release;for(let i=0;i<20;i++)release=window.fx.applyObject(window.fixtureObject,'hologram');const count=window.fx.stats().objects;release();return count===1&&window.fx.stats().objects===0}`).Bool() {
				t.Fatal("object effect replacement leaked")
			}
			page.MustEval(`()=>{window.fx.audio.setMuted(true);window.fx.event('shot');window.fx.audio.setMuted(false);window.fx.audio.play('not-imported')}`)
			if !page.MustEval(`()=>window.errors.pop().includes('unknown imported sound not-imported')`).Bool() {
				t.Fatal("missing sound diagnostic")
			}
			page.MustEval(`()=>{window.fx.reset();window.fx.setPaused(true);window.beforePause=window.fx.stats().time}`)
			page.MustWait(`()=>window.environmentFrames>70`)
			if !page.MustEval(`()=>window.fx.stats().time===window.beforePause&&window.fx.stats().voices===0`).Bool() {
				t.Fatal("pause did not freeze presentation")
			}
			page.MustEval(`()=>{window.fx.setPaused(false);for(let i=0;i<500;i++)window.fx.emit('blood-decal',{position:[0,0,0]})}`)
			if page.MustEval(`()=>window.fx.stats().decals`).Int() > 96 {
				t.Fatal("unbounded decals")
			}
			if !page.MustEval(`()=>{for(let i=0;i<8;i++)window.fx.reset();return window.fx.stats().decals===0&&window.fx.stats().particles===0}`).Bool() {
				t.Fatal("restart leaked effects")
			}
			errors := page.MustEval(`()=>window.errors`).String()
			if errors != "[]" && errors != "" {
				t.Fatal(errors)
			}
			b := []byte(page.MustEval(`()=>JSON.stringify({audio:window.audioChecked,stats:window.fx.stats(),frames:window.frameTimes})`).String())
			os.WriteFile(filepath.Join(report, dim+".json"), b, 0640)
			page.MustEval(`()=>{window.fixtureStopped=true;window.fx.dispose()}`)
			page.MustWait(`()=>window.fx.audio.context.state==='closed'`)
			if page.MustEval(`()=>window.fx.stats().voices`).Int() != 0 {
				t.Fatal("voices leaked on dispose")
			}
		})
	}
	t.Run("studio", func(t *testing.T) { checkPresentationStudio(t, browser, server.URL, report) })
}

const presentationFixture = `<!doctype html><html><head><meta charset="utf-8"><style>html,body{margin:0;overflow:hidden;background:#09121c;color:#effaff;font:14px system-ui}#game-root{position:absolute;inset:0}#test-audio{position:absolute;bottom:10px;right:10px;z-index:2000;padding:12px}#hud{position:absolute;left:24px;top:20px;z-index:1500;background:#102030cc;padding:14px;pointer-events:none}</style></head><body><div id="game-root"></div><div id="hud">AuraGo · Atmosphere laboratory<br>HUD stays outside postprocessing</div><button id="test-audio">Unlock and decode 40 WAV sounds</button><script>window.errors=[];window.addEventListener('error',e=>window.errors.push(e.message));window.addEventListener('unhandledrejection',e=>window.fixtureError=String(e.reason));const oldError=console.error;console.error=(...a)=>{window.errors.push(a.map(String).join(' '));oldError(...a)}</script><script type="module">
const dim=new URLSearchParams(location.search).get('dimension'),root=document.getElementById('game-root');
const config=await (await fetch('/config.json')).json();const runtime=await import('/runtime/aurago-effects-'+dim+'-1.js');
let clock=0,last=0,render,adapter,onEnvironment=()=>{};window.frameTimes=[];window.environmentFrames=0;
const report=e=>window.errors.push(String(e));
function setup(){window.fx=runtime.createPresentation({adapter,config,root,controls:true,report});window.choose=id=>{window.fx.setEnvironment(id);onEnvironment(id);window.environmentFrames=0};window.choose('forest-rain');window.fixtureReady=true;requestAnimationFrame(frame)}
function frame(now){if(window.fixtureStopped)return;const dt=last?Math.min(.1,(now-last)/1000):0;last=now;clock+=dt;window.environmentFrames++;window.frameTimes.push(dt*1000);if(window.frameTimes.length>240)window.frameTimes.shift();window.fx.update(dt);render?.();requestAnimationFrame(frame)}
if(dim==='3d'){
 const T=await import('/runtime/three-0.185.1.module.min.js');const scene=new T.Scene(),camera=new T.PerspectiveCamera(55,innerWidth/innerHeight,.1,450),renderer=new T.WebGLRenderer({antialias:true});renderer.setSize(innerWidth,innerHeight);renderer.setPixelRatio(1);renderer.toneMapping=T.ACESFilmicToneMapping;root.append(renderer.domElement);camera.position.set(12,9,18);camera.lookAt(0,1,0);const sun=new T.DirectionalLight(0xffecd0,3),ambient=new T.HemisphereLight(0xceefff,0x243523,2);sun.position.set(5,20,8);scene.add(sun,ambient);adapter=runtime.createThreeAdapter({scene,camera,renderer,sun,ambient});
 const ground=new T.Mesh(new T.PlaneGeometry(150,150),new T.MeshStandardMaterial({color:0x334e3c}));ground.rotation.x=-Math.PI/2;scene.add(ground);adapter.registerSurface(ground,{kind:'ground'});onEnvironment=id=>{ground.scale.setScalar(id==='coast'?.12:1);ground.visible=id!=='space'};
 for(let i=0;i<14;i++){const tree=new T.Mesh(new T.ConeGeometry(1.5,6,7),new T.MeshStandardMaterial({color:i%2?0x294938:0x476d43}));tree.position.set((i%7-3)*5,3,-5-Math.floor(i/7)*8);scene.add(tree)}
 const roof=new T.Mesh(new T.BoxGeometry(7,.3,5),new T.MeshStandardMaterial({color:0x956f4a}));roof.position.set(-5,3,0);scene.add(roof);window.fixtureObject=roof;adapter.registerSurface(roof,{kind:'roof'});
 render=()=>window.fx.render();setup();
}else{
 const script=document.createElement('script');script.src='/runtime/phaser-4.2.1.min.js';await new Promise((r,j)=>{script.onload=r;script.onerror=j;document.head.append(script)});
 new Phaser.Game({type:Phaser.AUTO,parent:root,width:1920,height:1080,backgroundColor:'#10202a',scene:{create(){adapter=runtime.createPhaserAdapter({scene:this,view:'side',report});this.add.rectangle(960,960,1920,240,0x344b3a);const roof=this.add.rectangle(520,580,460,24,0x83684c);window.fixtureObject=roof;adapter.registerSurface(roof,{kind:'roof'});for(let i=0;i<9;i++)this.add.triangle(i*235+50,720,0,160,70,0,140,160,0x326448);setup()}}});
}
document.getElementById('test-audio').onclick=async()=>{try{await window.fx.audio.unlock({isTrusted:true});const ctx=window.fx.audio.context;let count=0;for(const s of config.sounds){const data=await(await fetch('/packs/aurago-sounds/'+s.files[0].file)).arrayBuffer();const b=await ctx.decodeAudioData(data);if(b.sampleRate!==48000||b.duration<=0)throw Error(s.id+' decode failed');count++}window.audioChecked=count;window.fx.audio.play('rifle');}catch(e){window.fixtureError=String(e)}};
</script></body></html>`
