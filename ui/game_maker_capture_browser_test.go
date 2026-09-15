package ui

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

func TestGameMakerIndependentCaptureBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	source, err := os.ReadFile("../internal/gamemaker/runtime/preview-tests.js")
	if err != nil {
		t.Fatal(err)
	}
	assets := httptest.NewServer(http.FileServer(http.Dir("../internal/gamemaker/runtime")))
	defer assets.Close()
	for _, kind := range []string{"2d", "webgl", "phaser", "three"} {
		t.Run(kind, func(t *testing.T) {
			page := newSmokeBrowser(t).MustPage(assets.URL + "/#gm-channel=visual-fixture").Timeout(15 * time.Second)
			page.MustWaitLoad()
			page.MustEval(`kind=>{
   document.body.innerHTML='<canvas width="1920" height="1080"></canvas><div id="hud">Lives: 3</div>';
   const canvas=document.querySelector('canvas');
   if(kind==='2d'){const ctx=canvas.getContext('2d');ctx.fillStyle='#395c90';ctx.fillRect(0,0,1920,1080);ctx.fillStyle='#ffb530';ctx.fillRect(80,600,100,100)}
	   else if(kind==='webgl'){const gl=canvas.getContext('webgl');if(!gl)throw Error('WebGL unavailable');const render=()=>{gl.clearColor(.2,.4,.8,1);gl.clear(gl.COLOR_BUFFER_BIT);requestAnimationFrame(render)};render()}
   window.visualResult=null;window.addEventListener('message',e=>{if(e.data.type==='capture'&&e.data.source==='aurago-game')window.visualResult=e.data});
	  }`, kind)
			if kind == "phaser" {
				phaser, e := os.ReadFile("../internal/gamemaker/runtime/phaser-4.2.1.min.js")
				if e != nil {
					t.Fatal(e)
				}
				page.MustEval(`source=>{(0,eval)(source);document.querySelector('canvas').remove();window.fixtureGame=new Phaser.Game({type:Phaser.WEBGL,width:1920,height:1080,banner:false,audio:{noAudio:true},scene:{create(){this.add.rectangle(960,540,1800,900,0x33cc55);window.__AURAGO_GAME_TEST__={scene:this};}}})}`, string(phaser))
				page.MustWait(`()=>!!window.__AURAGO_GAME_TEST__`)
			}
			if kind == "three" {
				page.MustEval(`async url=>{const THREE=await import(url);const canvas=document.querySelector('canvas'),renderer=new THREE.WebGLRenderer({canvas});renderer.setSize(1920,1080,false);const scene=new THREE.Scene();scene.background=new THREE.Color(0x33cc55);const camera=new THREE.PerspectiveCamera();camera.position.z=3;const tick=()=>{renderer.render(scene,camera);requestAnimationFrame(tick)};tick();}`, assets.URL+"/three-0.185.1.module.min.js")
			}
			page.MustEval(`source=>{(0,eval)(source)}`, string(source))
			page.MustEval(`()=>window.postMessage({source:'aurago-studio',channel:'visual-fixture',type:'capture',request_id:'manual'},'*')`)
			page.MustWait(`()=>window.visualResult!==null`)
			if !page.MustEval(`()=>{const c=window.visualResult.captures[0];return c&&c.width===1280&&c.height===720&&c.image.startsWith('data:image/png;base64,')&&c.image.length<=700000&&c.hud.includes('Lives: 3')}`).Bool() {
				t.Fatal("capture missing, oversized or lost HTML HUD context")
			}
			if kind == "phaser" || kind == "three" {
				if !page.MustEval(`async()=>{const img=new Image();img.src=window.visualResult.captures[0].image;await img.decode();const c=document.createElement('canvas');c.width=img.width;c.height=img.height;const ctx=c.getContext('2d');ctx.drawImage(img,0,0);const p=ctx.getImageData(640,360,1,1).data;return p[1]>p[0]&&p[1]>p[2]&&p[3]===255}`).Bool() {
					t.Fatal("capture does not contain the engine's rendered green scene")
				}
			}
		})
	}
}

func TestGameMakerManualVisualLifecycleBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	page := newSmokeBrowser(t).MustPage("about:blank").Timeout(15 * time.Second)
	page.MustWaitLoad()
	page.MustEval(`()=>{
  document.body.innerHTML='<main style="background:#19212e;color:#eee;padding:24px;font:15px sans-serif"><div data-gm-visual hidden><span data-gm-visual-status></span></div></main>';
  window.visualDiagnostics=[];window.visualRequest=null;
  window.fixtureState={container:document.querySelector('main'),context:{t:k=>k},frame:{contentWindow:window},channelID:'manual',project:{id:'project'},previewProjectID:'project',previewGrant:{token:'current',expires_at:new Date(Date.now()+120000).toISOString()},previewReported:new Set(),addDiagnostic:d=>visualDiagnostics.push(d),api:{reviewVisual:(id,body,signal)=>{window.visualRequest={id,body,signal};return new Promise(resolve=>window.resolveVisual=resolve)}}};
 }`)
	page.MustEval(`source=>{(0,eval)(source)}`, string(mustReadUIFile(t, "js/desktop/apps/game-maker-studio-preview.js")))
	page.MustEval(`()=>{
  const api=window.GameMakerStudioPreview,state=window.fixtureState;api.requestCapture(state);
  const canvas=document.createElement('canvas');canvas.width=320;canvas.height=180;canvas.getContext('2d').fillRect(0,0,320,180);
  window.capturePayload={source:'aurago-game',channel:'manual',type:'capture',request_id:state.visualCapture.id,captures:[{image:canvas.toDataURL(),width:320,height:180,scenario:'current_view',at:new Date().toISOString()}]};
  api.handleMessage(state,{source:window,data:window.capturePayload});
 }`)
	if !page.MustEval(`()=>visualRequest.body.token==='current'&&fixtureState.container.querySelectorAll('img').length===1`).Bool() {
		t.Fatal("manual analysis did not bind the current capture")
	}
	page.MustEval(`()=>{GameMakerStudioPreview.cancelVisual(fixtureState);fixtureState.previewGrant={token:'next'};resolveVisual({status:'reviewed',findings:[{observation:'obsolete finding'}]})}`)
	page.MustEval(`async()=>{await Promise.resolve();await Promise.resolve()}`)
	if !page.MustEval(`()=>visualRequest.signal.aborted&&visualDiagnostics.length===0&&fixtureState.container.querySelectorAll('img').length===0&&!fixtureState.visualBusy`).Bool() {
		t.Fatal("closed/replaced preview retained its image, request or stale findings")
	}
}
