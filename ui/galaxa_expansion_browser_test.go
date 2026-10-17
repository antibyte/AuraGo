package ui

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
)

func newGalaxaSmokeBrowser(t *testing.T) *rod.Browser {
	t.Helper()
	return newSmokeBrowserWith(t, func(l *launcher.Launcher) {
		// Rod's leakless helper can hang before publishing its PID on Windows.
		l.Leakless(false)
		t.Cleanup(l.Kill)
	})
}

func TestGalaxaPlanetLifecycleBrowser(t *testing.T) {
	if os.Getenv("AURAGO_RUN_BROWSER_SMOKE") != "1" {
		t.Skip("set AURAGO_RUN_BROWSER_SMOKE=1")
	}
	var fail atomic.Bool
	fail.Store(true)
	server := galaxaArcadeServer(t, &fail)
	page := newGalaxaSmokeBrowser(t).MustPage(server.URL + "/fixture").Timeout(90 * time.Second)
	defer page.Close()
	page.MustWaitLoad()
	waitForJSBool(t, page, `()=>game?.G.st==='TITLE' && !!document.querySelector('.galaxa-planet-status button')`)
	if !page.MustEval(`()=>document.querySelector('.galaxa-planet-status').textContent.includes(words['galaxa.planet_load_failed'])`).Bool() {
		t.Fatal("planet error is not localized")
	}
	fail.Store(false)
	page.MustElement(".galaxa-planet-status button").MustClick()
	waitForJSBool(t, page, `()=>!document.querySelector('.galaxa-planet-status') && [...game.planetCache.values()].some(s=>s.ready)`)
	result, err := page.Eval(`async()=>{
 const c=game,g=c.G,assert=(v,m)=>{if(!v)throw Error(m)};cancelAnimationFrame(c.rafId);c.GalagaMusic.stop();c.settings.mute=true;
 c.resetRun(21);c.startStage();g.st='PLAYING';c.updatePlanets(0);
 const ready=async()=>{for(let i=0;i<200;i++){c.updatePlanets(0);if([...c.planetCache.values()].every(s=>s.ready))return;await new Promise(r=>setTimeout(r,10));}throw Error('planet load timeout')};await ready();
 const capture=()=>{const calls=[],draw=CanvasRenderingContext2D.prototype.drawImage;
  CanvasRenderingContext2D.prototype.drawImage=function(...a){if(a[0].src?.includes('exp-planet-'))calls.push({src:a[0].src,x:a[5],y:a[6],w:a[7],h:a[8],alpha:this.globalAlpha});return draw.apply(this,a)};
  try{c.drawPlanets(c.c)}finally{CanvasRenderingContext2D.prototype.drawImage=draw}return calls};
 const before=capture()[0];c.updatePlanets(1);const after=capture()[0];assert(after.y-before.y>=0.79&&after.y-before.y<=1.61,'planet drift');
 g.warpT=1000;g.shkT=1000;g.shkM=20;c.updateBackground(0.5);const warp=capture()[0];assert(Math.abs(warp.y-after.y-(after.y-before.y)*0.5)<0.001,'warp accelerated planet');
 c.pauseForFocus();const paused=JSON.stringify(capture());for(let i=0;i<60;i++)c.stepFrame(1/60);assert(JSON.stringify(capture())===paused,'focus pause moved planet');
 g.st='PLAYING';c.settings.reducedMotion=true;const reduced=JSON.stringify(capture());for(let i=0;i<60;i++)c.updatePlanets(1/60);assert(JSON.stringify(capture())===reduced,'reduced motion moved planet');
 c.settings.reducedMotion=false;c.updatePlanets(17);await ready();assert(c.planetCache.size<=2,'frame boundary exceeded cache');
 g.stage=2;c.updatePlanets(0);await ready();c.updatePlanets(1);assert(capture().length===2,'planet crossfade missing');assert(c.planetCache.size<=2,'transition exceeded cache');c.updatePlanets(3);await ready();
 const cacheSize=c.spriteAtlasCache.size;c.drawPlanets(c.c);assert(c.spriteAtlasCache.size===cacheSize,'planet entered sprite cache');
 GalaxaDeluxe.dispose('test');assert(c.planetCache.size===0,'planet images retained on dispose');assert(!errors.length,errors.join('\n'));return 'localized failure/retry, drift, warp, focus/pause, reduced motion, sheet boundary, crossfade and disposal';
}`)
	if err != nil {
		t.Fatal(err)
	}
	t.Log(result.Value.String())
}

func TestGalaxaPlanetOcclusionBrowser(t *testing.T) {
	if os.Getenv("AURAGO_RUN_BROWSER_SMOKE") != "1" {
		t.Skip("set AURAGO_RUN_BROWSER_SMOKE=1")
	}
	server := galaxaArcadeServer(t)
	page := newGalaxaSmokeBrowser(t).MustPage(server.URL + "/fixture").Timeout(120 * time.Second)
	defer page.Close()
	page.MustWaitLoad()
	waitForJSBool(t, page, `()=>game?.G.st==='TITLE' || errors.length>0`)
	result, err := page.Eval(`async()=>{
 const c=game,g=c.G,assert=(ok,message)=>{if(!ok)throw Error(message)};
 cancelAnimationFrame(c.rafId);c.GalagaMusic.stop();c.settings.mute=true;c.settings.reducedMotion=false;
 c.resetRun(21);g.st='LOADING';c.scale=1;
 const canvas=document.createElement('canvas');canvas.width=c.W;canvas.height=c.H;
 const d=canvas.getContext('2d',{willReadFrequently:true});c.c=d;
 const ready=async()=>{for(let i=0;i<500;i++){c.updatePlanets(0);if([...c.planetCache.values()].every(s=>s.ready))return;await new Promise(r=>setTimeout(r,10));}throw Error('planet load timeout')};
 const pixels=()=>d.getImageData(0,0,c.W,c.H).data;
 let motifs=0,checks=0,minOpaque=Infinity;
 for(const [sector,pool] of Object.entries(c.PLANET_ART))for(const [index,id] of pool.entries()){
  c.resetPlanets();c.expansionSector=()=>sector;g.biome=sector;g.stage=index+1;c.updatePlanets(0);await ready();
  const a=c.spriteAssets.manifest.animations['planet.'+id];
  for(const midpoint of [false,true]){
   if(midpoint)c.updatePlanets(a.ms/2000);
   d.clearRect(0,0,c.W,c.H);c.drawPlanets(d);const mask=pixels();
   c.drawStars=()=>{};c.renderFrame();const withoutStars=pixels();
   c.drawStars=ctx=>{ctx.fillStyle='#ffffff';ctx.fillRect(0,0,c.W,c.H)};
   c.renderFrame();const withStars=pixels();let opaque=0,visibleSky=0;
   for(let i=0;i<mask.length;i+=4){
    // The original PNG bodies have alpha 252-253, so allow their small residual transparency.
    if(mask[i+3]>=250){
     opaque++;
     for(let channel=0;channel<3;channel++)assert(Math.abs(withStars[i+channel]-withoutStars[i+channel])<=6,'stars show through '+id+' midpoint='+midpoint);
    }else if(mask[i+3]===0&&withStars[i]!==withoutStars[i])visibleSky++;
   }
   assert(opaque>500,'planet body is translucent: '+id+' midpoint='+midpoint+' opaque='+opaque);
   assert(visibleSky>1000,'stars disappeared outside the planet: '+id);
   minOpaque=Math.min(minOpaque,opaque);checks++;
  }
  assert(c.planetCache.size<=2,'planet sheet budget');motifs++;
 }
 GalaxaDeluxe.dispose('test');assert(!errors.length,errors.join('\n'));
 return {motifs,checks,minOpaque};
}`)
	if err != nil {
		t.Fatal(err)
	}
	t.Log(result.Value.String())
}

func TestGalaxaExpansionBrowser(t *testing.T) {
	if os.Getenv("AURAGO_RUN_BROWSER_SMOKE") != "1" {
		t.Skip("set AURAGO_RUN_BROWSER_SMOKE=1")
	}
	server := galaxaArcadeServer(t)
	page := newGalaxaSmokeBrowser(t).MustPage(server.URL + "/fixture").Timeout(180 * time.Second)
	defer page.Close()
	page.MustWaitLoad()
	waitForJSBool(t, page, `()=>game?.G.st==='TITLE' || errors.length>0`)
	script, err := os.ReadFile("testdata/galaxa-expansion-checks.js")
	if err != nil {
		t.Fatal(err)
	}
	result, err := page.Eval(string(script))
	if err != nil {
		t.Fatal(err)
	}
	t.Log(result.Value.String())
	if dir := os.Getenv("AURAGO_BROWSER_ARTIFACT_DIR"); dir != "" {
		if err = os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		for _, kind := range []string{"asteroid", "alien", "weapon", "planet", "effect", "asteroid-frames", "alien-frames", "weapon-frames", "planet-frames", "effect-frames"} {
			data, err := base64.StdEncoding.DecodeString(page.MustEval(`kind=>expansionGalleryImages[kind]`, kind).Str())
			if err != nil {
				t.Fatal(err)
			}
			if err = os.WriteFile(filepath.Join(dir, "galaxa-expansion-"+kind+".png"), data, 0644); err != nil {
				t.Fatal(err)
			}
		}
	}
	page.MustEval(`()=>GalaxaDeluxe.dispose('test')`)
}
