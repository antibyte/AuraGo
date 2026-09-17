package gamemaker

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/input"
	"github.com/go-rod/rod/lib/launcher"
)

// Exercise real rendering while normal keyboard input crosses the former
// depth-1000 HUD boundary. High world depths are valid in Y-sorted games.
func TestPresentationScrollingBrowser(t *testing.T) {
	if os.Getenv("GAMEMAKER_PRESENTATION_BROWSER") != "1" {
		t.Skip("set GAMEMAKER_PRESENTATION_BROWSER=1 for scrolling camera checks")
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprint(w, presentationScrollingFixture)
			return
		}
		data, err := bundledAssetPackFile("runtime", strings.TrimPrefix(r.URL.Path, "/"))
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/javascript")
		w.Write(data)
	}))
	defer server.Close()
	bin := os.Getenv("CHROME_BIN")
	if bin == "" {
		bin = "C:/Program Files/Google/Chrome/Application/chrome.exe"
	}
	l := launcher.New().Bin(bin).Headless(true).NoSandbox(true).Set("enable-unsafe-swiftshader")
	browser := rod.New().ControlURL(l.MustLaunch()).MustConnect()
	defer l.Cleanup()
	defer browser.Close()
	page := browser.MustPage("about:blank").Timeout(30 * time.Second)
	defer page.Close()
	page.MustSetViewport(640, 360, 1, false)
	page.MustNavigate(server.URL).MustWaitLoad()
	page.MustWait(`()=>window.fixture?.frames>3`)
	page.MustActivate()
	check := func() {
		t.Helper()
		if !page.MustEval(`()=>{const s=fixture,m=s.cameras.main,h=s.cameras.cameras[1];return m.renderList.includes(s.player)&&!h.renderList.includes(s.player)&&h.renderList.includes(s.hud)&&h.renderList.includes(s.legacy)&&m.renderList.includes(s.fixedWorld)&&!m.renderList.includes(s.excluded)&&s.hud.cameraFilter===m.id&&s.excluded.cameraFilter===(m.id|h.id)}`).Bool() {
			t.Fatal("world/HUD camera routing is incorrect", page.MustEval(`()=>({y:fixture.player.y,depth:fixture.player.depth,scroll:fixture.cameras.main.scrollY})`))
		}
		// Render-list membership alone does not prove a visible pixel.
		if !page.MustEval(`()=>new Promise(resolve=>{const s=fixture,c=s.cameras.main;s.game.renderer.snapshotPixel(Math.round(s.player.x-c.scrollX),Math.round(s.player.y-c.scrollY),p=>resolve(p.r>220&&p.b>220&&p.g<30))})`).Bool() {
			t.Fatal("the scrolling player is not visibly rendered")
		}
		if !page.MustEval(`()=>new Promise(resolve=>fixture.game.renderer.snapshotPixel(20,20,p=>resolve(p.g>220&&p.b>220&&p.r<30)))`).Bool() {
			t.Fatal("explicit low-depth HUD is not visible")
		}
	}
	check()
	for _, key := range []input.Key{input.KeyW, input.KeyS} {
		if err := page.Keyboard.Press(key); err != nil {
			t.Fatal(err)
		}
		if key == input.KeyW {
			page.MustWait(`()=>fixture.player.y<900`)
		} else {
			page.MustWait(`()=>fixture.player.y>1200`)
		}
		if err := page.Keyboard.Release(key); err != nil {
			t.Fatal(err)
		}
		check()
	}
	if !page.MustEval(`()=>{const s=fixture;s.stopped=true;s.adapter.dispose();s.adapter.dispose();return s.cameras.cameras.length===1&&s.player.cameraFilter===0&&s.hud.cameraFilter===0&&s.excluded.cameraFilter===s.cameras.main.id}`).Bool() {
		t.Fatal("dispose failed to restore original camera exclusions")
	}
	if errors := page.MustEval(`()=>window.errors`).String(); errors != "[]" {
		t.Fatal(errors)
	}
}

const presentationScrollingFixture = `<!doctype html><style>body{margin:0}</style>
<script>window.errors=[];addEventListener('error',e=>errors.push(e.message));addEventListener('unhandledrejection',e=>errors.push(String(e.reason)))</script>
<script src="/phaser-4.2.1.min.js"></script><script type="module">
import {createPhaserAdapter} from '/aurago-effects-2d-1.js';
new Phaser.Game({type:Phaser.WEBGL,width:640,height:360,backgroundColor:'#102030',scene:{
 create(){window.fixture=this;this.frames=0;this.keys=this.input.keyboard.addKeys('W,S');
  this.player=this.add.rectangle(320,1250,40,40,0xff00ff).setDepth(1250);
  this.hud=this.add.rectangle(20,20,24,24,0x00ffff).setScrollFactor(0).setDepth(5).setData('auragoHUD',true);
  this.legacy=this.add.rectangle(60,20,24,24,0xffffff).setScrollFactor(0).setDepth(1000);
  this.fixedWorld=this.add.rectangle(100,20,24,24,0x00ff00).setScrollFactor(0).setDepth(1500).setData('auragoHUD',false);
  this.excluded=this.add.rectangle(320,1250,80,80,0xffffff).setDepth(10000);this.cameras.main.ignore(this.excluded);
  this.cameras.main.setBounds(0,0,640,1800).startFollow(this.player);
  this.adapter=createPhaserAdapter({scene:this});this.adapter.set('color-grade',{intensity:0});
 },
 update(time,delta){if(this.stopped)return;this.player.y+=(Number(this.keys.S.isDown)-Number(this.keys.W.isDown))*delta*.6;this.player.setDepth(this.player.y);this.adapter.update(delta/1000,time/1000);this.frames++}
}});
</script>`
