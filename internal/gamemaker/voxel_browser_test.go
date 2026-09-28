package gamemaker

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/input"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
)

func TestVoxelBrowser(t *testing.T) {
	if os.Getenv("GAMEMAKER_VOXEL_BROWSER") != "1" {
		t.Skip("set GAMEMAKER_VOXEL_BROWSER=1")
	}
	bin := os.Getenv("CHROME_BIN")
	if bin == "" {
		bin = "C:/Program Files/Google/Chrome/Application/chrome.exe"
	}
	l := launcher.New().Bin(bin).Headless(true).NoSandbox(true).Set("enable-unsafe-swiftshader")
	browser := rod.New().ControlURL(l.MustLaunch()).MustConnect()
	defer l.Cleanup()
	defer browser.Close()
	s := newTestService(t)
	p, dir, v := voxelFixture(t, s)
	v.Terrain = "flat"
	v.Enemies = nil
	raw, _ := json.Marshal(v)
	if err := os.WriteFile(filepath.Join(dir, "src", "voxel.json"), raw, 0600); err != nil {
		t.Fatal(err)
	}
	if result := buildDirectory(context.Background(), dir, 100, 32<<20); !result.OK {
		t.Fatal(result.Diagnostics)
	}
	publishExportFixture(t, s, p, dir)
	files := readExportFixture(t, s, p)
	extracted := t.TempDir()
	for path, data := range files {
		dest, _, err := secureJoin(extracted, path, true)
		if err != nil {
			t.Fatal(err)
		}
		os.MkdirAll(filepath.Dir(dest), 0750)
		if err := os.WriteFile(dest, data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	server := httptest.NewServer(http.StripPrefix("/nested/game/", http.FileServer(http.Dir(extracted))))
	defer server.Close()
	page := browser.MustPage("about:blank").Timeout(45 * time.Second)
	defer page.Close()
	page.MustSetViewport(1280, 720, 1, false)
	page.MustEvalOnNewDocument(`window.errors=[];addEventListener('error',e=>errors.push(e.message));addEventListener('unhandledrejection',e=>errors.push(String(e.reason)));window.voxel=()=>window.__AURAGO_GAME_TEST__?.observeVoxel()`)
	page.MustNavigate(server.URL + "/nested/game/").MustWaitLoad()
	defer func() {
		r, err := page.Context(context.Background()).Timeout(3 * time.Second).Eval(`()=>({errors:window.errors,body:document.body.innerText,game:!!window.__AURAGO_GAME_TEST__})`)
		if err == nil {
			t.Log("browser diagnostics:", r.Value.String())
		}
	}()
	page.MustWait(`()=>voxel()?.ready`)
	if !page.MustEval(`()=>voxel().paused && voxel().metrics.distance===0`).Bool() {
		t.Fatal("intro did not freeze play")
	}
	page.Keyboard.MustType(input.Enter)
	page.MustWait(`()=>!voxel().paused`)
	page.MustEval(`()=>{window.startX=voxel().player.position[0];}`)
	if err := page.Keyboard.Press(input.KeyD); err != nil {
		t.Fatal(err)
	}
	page.MustEval(`()=>new Promise(r=>setTimeout(r,250))`)
	page.Keyboard.Release(input.KeyD)
	if !page.MustEval(`()=>voxel().metrics.distance>0.3`).Bool() {
		t.Fatal("movement failed")
	}
	page.Keyboard.Press(input.KeyA)
	page.MustEval(`()=>new Promise(resolve=>{function align(){if(voxel().player.position[0]<=startX+.1){dispatchEvent(new KeyboardEvent('keyup',{key:'a',code:'KeyA',bubbles:true}));resolve();}else requestAnimationFrame(align);}align();})`)
	page.Keyboard.Release(input.KeyA)
	page.Keyboard.MustType(input.Space)
	page.MustWait(`()=>voxel().metrics.jumps>0`)
	page.MustWait(`()=>voxel().grounded`)
	page.Keyboard.Press(input.KeyE)
	page.MustEval(`()=>new Promise(r=>setTimeout(r,950))`)
	page.Keyboard.Release(input.KeyE)
	if !page.MustEval(`()=>voxel().metrics.mined>=1&&voxel().inventory.some(s=>s?.item==='wood')`).Bool() {
		t.Fatalf("mining failed: %s", page.MustEval(`()=>voxel()`).String())
	}
	page.Keyboard.MustType(input.KeyI)
	page.MustWait(`()=>!document.querySelector('[data-voxel-inventory]').hidden`)
	page.MustElement(`[data-recipe="planks"]`).MustClick()
	if !page.MustEval(`()=>voxel().metrics.crafted===1&&voxel().inventory.some(s=>s?.item==='planks'&&s.count===4)`).Bool() {
		t.Fatal("recipe did not consume wood and create planks")
	}
	page.Keyboard.MustType(input.KeyI)
	page.MustWait(`()=>!voxel().paused`)
	// Look down at the actual floor and place using the public secondary key.
	page.Keyboard.Press(input.ArrowDown)
	page.MustEval(`()=>new Promise(r=>setTimeout(r,650))`)
	page.Keyboard.Release(input.ArrowDown)
	page.Keyboard.MustType(input.Digit1, input.KeyF)
	page.MustWait(`()=>voxel().metrics.placed>0`)
	page.Keyboard.MustType(input.Escape)
	page.MustWait(`()=>voxel().paused`)
	page.MustEval(`()=>new Promise(r=>setTimeout(r,500))`)
	page.MustWait(`()=>document.querySelector('[data-voxel-save]').textContent.includes('device')`)
	page.MustReload().MustWaitLoad()
	page.MustWait(`()=>voxel()?.ready`)
	if !page.MustEval(`()=>voxel().inventory.some(s=>s?.item==='planks'&&s.count===3)`).Bool() {
		t.Fatal("export lost saved inventory after reload")
	}
	if !page.MustEval(`()=>errors.length===0`).Bool() {
		t.Fatal(page.MustEval(`()=>errors`).String())
	}
	page.Keyboard.MustType(input.Enter)
	page.MustWait(`()=>!voxel().paused`)
	if !page.MustEval(`()=>document.querySelector('[data-player-touch]').hidden&&document.querySelector('[data-voxel-actions]').hidden`).Bool() {
		t.Fatal("desktop has redundant touch controls")
	}
	page.Keyboard.Press(input.KeyD)
	page.MustEval(`()=>dispatchEvent(new Event('blur'))`)
	page.MustWait(`()=>voxel().paused`)
	page.MustEval(`()=>{window.frozen=voxel().player.position;}`)
	page.MustEval(`()=>new Promise(r=>setTimeout(r,180))`)
	if !page.MustEval(`()=>JSON.stringify(voxel().player.position)===JSON.stringify(frozen)`).Bool() {
		t.Fatal("focus loss did not freeze held movement")
	}
	page.Keyboard.Release(input.KeyD)
	page.MustElement("[data-player-start]").MustClick()
	page.MustSetViewport(390, 844, 1, false)
	maxTouch := 3
	if err := (proto.EmulationSetTouchEmulationEnabled{Enabled: true, MaxTouchPoints: &maxTouch}).Call(page); err != nil {
		t.Fatal(err)
	}
	// The first real touch selects the touch input affordances.
	page.Touch.MustTap(250, 300)
	page.MustWait(`()=>!document.querySelector('[data-voxel-actions]').hidden`)
	if !page.MustEval(`()=>document.querySelector('[data-voxel-hud]').getBoundingClientRect().top>document.querySelector('.aurago-game-presentation').getBoundingClientRect().bottom`).Bool() {
		t.Fatal("mobile status overlaps the audio controls")
	}
	if !page.MustEval(`()=>{const stick=document.querySelector('[data-player-stick]').getBoundingClientRect(),bar=document.querySelector('[data-voxel-hotbar]').getBoundingClientRect();return stick.bottom<bar.top&&bar.left>=0&&bar.right<=innerWidth&&[...document.querySelectorAll('[data-voxel-actions] button')].every(b=>{const r=b.getBoundingClientRect();return r.width>=44&&r.height>=44&&r.right<=innerWidth})}`).Bool() {
		t.Fatal("touch controls overlap or escape viewport")
	}
	point := page.MustEval(`()=>{const s=document.querySelector('[data-player-stick]').getBoundingClientRect(),a=document.querySelector('[data-voxel-action="jump"]').getBoundingClientRect();window.beforeTouch=voxel();return {x:s.x+s.width*.85,y:s.y+s.height/2,ax:a.x+a.width/2,ay:a.y+a.height/2}}`)
	one, two := float64(1), float64(2)
	page.Touch.MustStart(&proto.InputTouchPoint{X: point.Get("x").Num(), Y: point.Get("y").Num(), ID: &one}, &proto.InputTouchPoint{X: point.Get("ax").Num(), Y: point.Get("ay").Num(), ID: &two})
	page.MustWait(`()=>voxel().metrics.distance>beforeTouch.metrics.distance+.3&&voxel().metrics.jumps>beforeTouch.metrics.jumps`)
	page.Touch.MustCancel()
	page.MustEval(`()=>new Promise(r=>setTimeout(r,1100))`)
	page.MustEval(`()=>{window.released=voxel();}`)
	page.MustEval(`()=>new Promise(r=>setTimeout(r,220))`)
	if !page.MustEval(`()=>Math.abs(voxel().player.position[0]-released.player.position[0])<.01&&voxel().metrics.jumps===released.metrics.jumps`).Bool() {
		t.Fatal("touch cancellation left movement or jump held")
	}
	page.MustEval(`()=>{window.beforeLook=voxel();}`)
	page.Touch.MustStart(&proto.InputTouchPoint{X: 240, Y: 330, ID: &one})
	page.Touch.MustMove(&proto.InputTouchPoint{X: 290, Y: 350, ID: &one})
	page.Touch.MustEnd()
	if !page.MustEval(`()=>Math.abs(voxel().player.yaw-beforeLook.player.yaw)>.05&&voxel().metrics.mined===beforeLook.metrics.mined`).Bool() {
		t.Fatal("touch look must be independent from mining")
	}
	bar := page.MustEval(`()=>{const r=document.querySelector('[data-voxel-hotbar]').getBoundingClientRect();return {x:r.right-30,y:r.top+24};}`)
	page.Touch.MustStart(&proto.InputTouchPoint{X: bar.Get("x").Num(), Y: bar.Get("y").Num(), ID: &one})
	page.Touch.MustMove(&proto.InputTouchPoint{X: bar.Get("x").Num() - 140, Y: bar.Get("y").Num(), ID: &one})
	page.Touch.MustEnd()
	page.MustWait(`()=>document.querySelector('[data-voxel-hotbar]').scrollLeft>0`)
	page.Keyboard.MustType(input.KeyI)
	page.MustWait(`()=>!document.querySelector('[data-voxel-inventory]').hidden`)
	if !page.MustEval(`()=>[...document.querySelectorAll('[data-voxel-slots] button')].every(b=>b.getBoundingClientRect().width>=44)`).Bool() {
		t.Fatal("mobile inventory slots are too narrow")
	}
	page.Keyboard.MustType(input.KeyI)
	if reports := os.Getenv("VOXEL_SCREENSHOTS"); reports != "" {
		os.MkdirAll(reports, 0750)
		page.MustScreenshot(filepath.Join(reports, "voxel-touch.png"))
	}
	for range 2 {
		page.MustEval(`()=>window.__AURAGO_VOXEL_DISPOSE__()`)
		if !page.MustEval(`()=>!window.__AURAGO_GAME_TEST__&&!document.querySelector('canvas,[data-voxel-ui],[data-player-ui]')`).Bool() {
			t.Fatal("disposed game retained graphics or input UI")
		}
		page.MustReload().MustWaitLoad()
		page.MustWait(`()=>voxel()?.ready`)
		if page.MustEval(`()=>document.querySelectorAll('canvas').length`).Int() != 1 {
			t.Fatal("reopening leaked renderers")
		}
	}
}

func TestVoxelSurvivalAndCreativeBrowser(t *testing.T) {
	if os.Getenv("GAMEMAKER_VOXEL_BROWSER") != "1" {
		t.Skip("set GAMEMAKER_VOXEL_BROWSER=1")
	}
	bin := os.Getenv("CHROME_BIN")
	if bin == "" {
		bin = "C:/Program Files/Google/Chrome/Application/chrome.exe"
	}
	l := launcher.New().Bin(bin).Headless(true).NoSandbox(true).Set("enable-unsafe-swiftshader")
	browser := rod.New().ControlURL(l.MustLaunch()).MustConnect()
	defer l.Cleanup()
	defer browser.Close()
	for _, mode := range []string{"survival", "creative"} {
		t.Run(mode, func(t *testing.T) {
			s := newTestService(t)
			_, dir, v := voxelFixture(t, s)
			v.Mode = mode
			v.Terrain = "flat"
			v.Enemies[0].Count = 12
			v.Enemies[1].Count = 12
			// Lethal authored damage makes a real enemy-driven respawn check finite.
			if mode == "survival" {
				v.Enemies[0].Damage = 100
				v.Enemies[1].Damage = 100
			}
			data, _ := json.Marshal(v)
			os.WriteFile(filepath.Join(dir, "src", "voxel.json"), data, 0600)
			if build := buildDirectory(context.Background(), dir, 100, 32<<20); !build.OK {
				t.Fatal(build.Diagnostics)
			}
			server := httptest.NewServer(http.FileServer(http.Dir(dir)))
			defer server.Close()
			page := browser.MustPage("about:blank").Timeout(30 * time.Second)
			defer page.Close()
			page.MustSetViewport(1280, 720, 1, false)
			page.MustEvalOnNewDocument(`window.voxel=()=>window.__AURAGO_GAME_TEST__?.observeVoxel();window.errors=[];addEventListener('error',e=>errors.push(e.message))`)
			page.MustNavigate(server.URL).MustWaitLoad()
			page.MustWait(`()=>voxel()?.ready`)
			page.Keyboard.MustType(input.Enter)
			page.MustWait(`()=>!voxel().paused`)
			if page.MustEval(`()=>voxel().enemies.length`).Int() != 24 {
				t.Fatal("performance fixture must use 24 enemies")
			}
			if mode == "survival" {
				page.MustEval(`()=>new Promise(r=>setTimeout(r,1000))`)
				t.Log("24-active-enemy software-rendered smoke (not hardware acceptance):", page.MustEval(`()=>({fps:window.__AURAGO_GAME_TEST__.snapshot().fps,active:!voxel().paused,health:voxel().player.health,renderer:document.querySelector('canvas').getContext('webgl2')?.getParameter(7937),viewport:[innerWidth,innerHeight]})`).String())
				page.MustWait(`()=>voxel().player.health===0`)
				page.Keyboard.MustType(input.KeyR)
				page.MustWait(`()=>voxel().player.health===100&&voxel().metrics.respawns===1`)
			} else {
				page.Keyboard.MustType(input.KeyI)
				page.MustWait(`()=>!document.querySelector('[data-voxel-inventory]').hidden`)
				page.MustEval(`()=>[...document.querySelectorAll('[data-voxel-palette] button')].find(b=>b.textContent==='Metal tool').click()`)
				if !page.MustEval(`()=>voxel().inventory[voxel().selected]?.item==='metal_tool'`).Bool() {
					t.Fatal("creative palette cannot select every material/tool")
				}
				page.Keyboard.MustType(input.KeyI)
				page.MustEval(`()=>new Promise(r=>setTimeout(r,2300))`)
				t.Log("24-enemy software-rendered smoke (not hardware acceptance):", page.MustEval(`()=>({fps:window.__AURAGO_GAME_TEST__.snapshot().fps,renderer:document.querySelector('canvas').getContext('webgl2')?.getParameter(7937),viewport:[innerWidth,innerHeight]})`).String())
				if reports := os.Getenv("VOXEL_SCREENSHOTS"); reports != "" {
					os.MkdirAll(reports, 0750)
					page.MustScreenshot(filepath.Join(reports, "voxel-desktop.png"))
				}
			}
			if page.MustEval(`()=>errors.length`).Int() != 0 {
				t.Fatal(page.MustEval(`()=>errors`).String())
			}
		})
	}
}

func TestVoxelValidationBrowser(t *testing.T) {
	if os.Getenv("GAMEMAKER_VOXEL_BROWSER") != "1" {
		t.Skip("set GAMEMAKER_VOXEL_BROWSER=1")
	}
	bin := os.Getenv("CHROME_BIN")
	if bin == "" {
		bin = "C:/Program Files/Google/Chrome/Application/chrome.exe"
	}
	l := launcher.New().Bin(bin).Headless(true).NoSandbox(true).Set("enable-unsafe-swiftshader")
	browser := rod.New().ControlURL(l.MustLaunch()).MustConnect()
	defer l.Cleanup()
	defer browser.Close()
	s := newTestService(t)
	p, dir, _ := voxelFixture(t, s)
	publishExportFixture(t, s, p, dir)
	grant, err := s.CreatePreviewGrant(p.ID)
	if err != nil {
		t.Fatal(err)
	}
	plan := ExampleGamePlan(p)
	scenarios, _ := json.Marshal(gameScenarios(&plan))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprintf(w, `<html><body><iframe sandbox="allow-scripts allow-pointer-lock" style="width:960px;height:540px" src="%s#gm-channel=test"></iframe><script>
		window.reports=[];window.errors=[];addEventListener('message',e=>{if(e.source!==document.querySelector('iframe').contentWindow)return;const d=e.data;
		if(d.source==='aurago-voxel'&&d.type==='play_state')e.source.postMessage({source:'aurago-voxel-host',type:'play_state',channel:'test',request:d.request,result:{temporary:true,version:0,state:null}},'*');
		if(d.type==='ready')e.source.postMessage({source:'aurago-studio',channel:'test',type:'run-tests',scenarios:%s},'*');
		if(d.type==='gameplay')reports.push(d);if(d.type==='diagnostic'||d.type==='runtime_error')errors.push(d.message);});</script></body></html>`, grant.URL, scenarios)
			return
		}
		path := strings.TrimPrefix(r.URL.Path, "/api/game-maker/preview/"+grant.Token+"/")
		data, ct, err := s.PreviewFile(grant.Token, path)
		if err != nil {
			http.Error(w, err.Error(), 404)
			return
		}
		w.Header().Set("Content-Type", ct)
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Write(data)
	}))
	defer server.Close()
	page := browser.MustPage(server.URL).Timeout(65 * time.Second)
	defer page.Close()
	page.MustWait(`()=>reports.length>0`)
	raw := page.MustEval(`()=>JSON.stringify(reports[0])`).String()
	var report PreviewReport
	if err := json.Unmarshal([]byte(raw), &report); err != nil {
		t.Fatal(err)
	}
	if err := validateGameReport(report); err != nil {
		t.Fatal(err)
	}
	for _, check := range compareVoxelObservations(&plan, gameScenarios(&plan), report.Observations) {
		if check.Status != "passed" {
			t.Errorf("%+v", check)
		}
	}
	if t.Failed() {
		for _, o := range report.Observations {
			if o.ID == "required_voxel_place" {
				data, _ := json.Marshal(o)
				t.Log(string(data))
			}
		}
		t.Log("errors:", page.MustEval(`()=>errors`).String())
	}
}
