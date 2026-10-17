package ui

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestGalaxaAsteroidAnimationBrowser(t *testing.T) {
	if os.Getenv("AURAGO_RUN_BROWSER_SMOKE") != "1" {
		t.Skip("set AURAGO_RUN_BROWSER_SMOKE=1")
	}
	server := galaxaArcadeServer(t)
	page := newGalaxaSmokeBrowser(t).MustPage(server.URL + "/fixture").Timeout(90 * time.Second)
	defer page.Close()
	page.MustWaitLoad()
	waitForJSBool(t, page, `()=>game?.G.st==='TITLE' || errors.length>0`)
	dir := os.Getenv("AURAGO_BROWSER_ARTIFACT_DIR")
	result, err := page.Eval(`async capture=>{
 const c=game,g=c.G,assert=(ok,message)=>{if(!ok)throw Error(message)};
 cancelAnimationFrame(c.rafId);c.GalagaMusic.stop();g.muted=true;c.settings.mute=true;
 const canvas=document.createElement('canvas');canvas.width=canvas.height=128;
 const d=canvas.getContext('2d',{willReadFrequently:true});
 const pixels=()=>d.getImageData(0,0,128,128).data;
 const frame=(key,time,size,loop)=>{d.clearRect(0,0,128,128);c.drawAnimation(d,key,64,64,time,size,null,loop);return pixels()};
 const difference=(a,b)=>{let sum=0;for(let i=0;i<a.length;i+=4){sum+=Math.abs(a[i+3]-b[i+3]);for(let ch=0;ch<3;ch++)sum+=Math.abs(a[i+ch]*a[i+3]-b[i+ch]*b[i+3])/255}return sum/a.length/255};
 const keys=Object.keys(c.spriteAssets.manifest.animations).filter(k=>k.startsWith('asteroid.'));
 c.clearSpriteAtlasCache();let worstBoundary=0,worstMidpoint=0;
 for(const key of keys){
  const a=c.spriteAssets.manifest.animations[key],cycle=a.ms*a.frames.length;
  for(const size of [35,55,80]){
   for(let boundary=1;boundary<=a.frames.length;boundary++){
    const delta=difference(frame(key,boundary*a.ms-.5,size),frame(key,boundary*a.ms+.5,size));
    worstBoundary=Math.max(worstBoundary,delta);
    assert(delta<.003,'asteroid frame discontinuity: '+key+' frame '+boundary+' size '+size+' delta '+delta);
   }
   const before=frame(key,cycle-a.ms,size),after=frame(key,cycle,size),mid=frame(key,cycle-a.ms/2,size);
   let error=0;
   for(let i=0;i<mid.length;i+=4){
    error+=Math.abs(mid[i+3]-(before[i+3]+after[i+3])/2);
    for(let ch=0;ch<3;ch++)error+=Math.abs(mid[i+ch]*mid[i+3]/255-(before[i+ch]*before[i+3]+after[i+ch]*after[i+3])/510);
   }
   error/=mid.length*255;worstMidpoint=Math.max(worstMidpoint,error);
   assert(error<.003,'asteroid crossfade changed opacity/brightness: '+key+' '+error);
   assert(difference(frame(key,cycle+500,size,false),frame(key,cycle-a.ms,size,false))===0,'non-looping override wrapped: '+key);
  }
 }
 assert(c.spriteAtlasCache.size<=keys.length*16*3,'intermediate frames entered sprite cache');
 const ordinary=c.spriteAssets.manifest.animations['bee.idle'];
 assert(difference(frame('bee.idle',0,48),frame('bee.idle',ordinary.ms/2,48))===0,'unmarked animation changed');
 c.settings.reducedMotion=true;g.asteroids=[{art:{id:'hollow-asteroid'},x:64,y:64,r:32,warning:0,age:0}];
 d.clearRect(0,0,128,128);c.drawAsteroids(d);const still=pixels();
 g.asteroids[0].age=1590;d.clearRect(0,0,128,128);c.drawAsteroids(d);
 assert(difference(still,pixels())===0,'reduced motion animated asteroids');
 c.settings.reducedMotion=false;c.resetRun(71);g.stage=7;c.startStage();g.st='PLAYING';g.p.inv=1e6;g.asteroidSpawnT=1e9;
 g.asteroids=Array.from({length:20},(_,i)=>({art:c.ASTEROID_ART[i%keys.length],x:50+i%4*140,y:120+Math.floor(i/4)*100,
  r:32,size:2,hp:6,vy:0,warning:0,age:i*39,root:{rewarded:false}}));
 const started=performance.now(),times=[];
 await new Promise(resolve=>{const tick=()=>{const before=performance.now();c.stepFrame(1/60);c.renderFrame();times.push(performance.now()-before);
  if(performance.now()-started>=3200)resolve();else requestAnimationFrame(tick)};requestAnimationFrame(tick)});
 const fps=times.length*1000/(performance.now()-started),meanMs=times.reduce((a,b)=>a+b,0)/times.length;
 assert(g.asteroids.length===20&&c.spriteAtlasCache.size<=768,'asteroid stress fixture or cache limit');
 if(capture){
  const review=document.createElement('canvas');review.width=896;review.height=keys.length*100;
  const rd=review.getContext('2d');rd.fillStyle='#071020';rd.fillRect(0,0,review.width,review.height);
  for(const [row,key] of keys.entries()){
   const a=c.spriteAssets.manifest.animations[key],end=a.ms*a.frames.length;
   rd.fillStyle='#dbeeff';rd.font='12px monospace';rd.fillText(key,8,row*100+14);
   for(const [column,offset] of [-100,-75,-50,-25,0,25,50].entries())c.drawAnimation(rd,key,column*128+64,row*100+60,end+offset,64);
  }
  window.asteroidLoopReview=review.toDataURL('image/png').split(',')[1];
 }
 GalaxaDeluxe.dispose('test');assert(c.spriteAtlasCache.size===0,'disposal retained sprite cache');
 assert(!errors.length,errors.join('\n'));
 return {motifs:keys.length,sizes:3,boundaries:keys.length*16*3,worstBoundary,worstMidpoint,asteroids:20,fps,meanMs};
}`, dir != "")
	if err != nil {
		t.Fatal(err)
	}
	t.Log(result.Value.String())
	if dir != "" {
		data, err := base64.StdEncoding.DecodeString(page.MustEval(`()=>asteroidLoopReview`).Str())
		if err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "galaxa-asteroid-loop-boundaries.png"), data, 0644); err != nil {
			t.Fatal(err)
		}
	}
}
