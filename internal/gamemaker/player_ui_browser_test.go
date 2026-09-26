package gamemaker

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/input"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
)

// Exercise exported games with real keyboard, mouse and simultaneous touch
// inputs. Only read observations; never bypass the Start gate or mutate actors.
func TestPlayerUIBrowser(t *testing.T) {
	if os.Getenv("GAMEMAKER_EXPERIENCE_BROWSER") != "1" {
		t.Skip("set GAMEMAKER_EXPERIENCE_BROWSER=1")
	}
	bin := os.Getenv("CHROME_BIN")
	if bin == "" {
		bin = "C:/Program Files/Google/Chrome/Application/chrome.exe"
	}
	launch := launcher.New().Bin(bin).Headless(true).NoSandbox(true).Set("enable-unsafe-swiftshader")
	browser := rod.New().ControlURL(launch.MustLaunch()).MustConnect()
	defer launch.Cleanup()
	defer browser.Close()
	for _, mode := range []string{"platformer", "board", "fps", "space"} {
		t.Run(mode, func(t *testing.T) {
			dimension := "2d"
			if mode == "fps" || mode == "space" {
				dimension = "3d"
			}
			service := newTestService(t)
			project := createTestProject(t, service, dimension)
			root := filepath.Join(service.opts.WorkspacePath, project.ProjectKey)
			if err := WriteScaffold(root, project); err != nil {
				t.Fatal(err)
			}
			plan := ExampleGamePlan(project)
			plan.Template, plan.Assets, plan.Objective = mode, nil, "Find the forest outpost."
			if err := installGameTemplate(root, plan); err != nil {
				t.Fatal(err)
			}
			if dimension == "3d" {
				source := "import {startGame} from './common'; startGame({mode:'" + mode + "', objective:'Find the forest outpost.', objects:[], speed:6, goal:3, duration:120});"
				if err := os.WriteFile(filepath.Join(root, "src/main.ts"), []byte(source), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if build := buildDirectory(context.Background(), root, 150, 64<<20); !build.OK {
				t.Fatal(build.Diagnostics)
			}
			publishExportFixture(t, service, project, root)
			files := readExportFixture(t, service, project)
			if len(files["vendor/player-ui.js"]) == 0 {
				t.Fatal("export missing player UI")
			}
			extracted := t.TempDir()
			for path, data := range files {
				dest, _, err := secureJoin(extracted, path, true)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.MkdirAll(filepath.Dir(dest), 0750); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(dest, data, 0600); err != nil {
					t.Fatal(err)
				}
			}
			server := httptest.NewServer(http.StripPrefix("/play/", http.FileServer(http.Dir(extracted))))
			defer server.Close()
			page := browser.MustPage("about:blank").Timeout(40 * time.Second)
			defer page.Close()
			page.MustSetViewport(1280, 720, 1, false)
			page.MustEvalOnNewDocument(`window.errors=[];addEventListener('error',e=>errors.push(e.message));addEventListener('unhandledrejection',e=>errors.push(String(e.reason)));Object.defineProperty(navigator,'language',{value:'de-DE'});window.readState=()=>{const b=window.__AURAGO_GAME_TEST__;return b.kind==='three'?b.snapshot():{...b.state,elapsed_ms:b.scene.elapsed,player_x:b.player.x,player_y:b.player.y}}`)
			page.MustNavigate(server.URL + "/play/").MustWaitLoad()
			page.MustWait(`()=>!!window.__AURAGO_GAME_TEST__&&!!document.querySelector('[data-player-start]')`)
			page.MustActivate()
			page.Keyboard.MustType(input.KeyW, input.Space)
			page.MustEval(`()=>new Promise(r=>setTimeout(r,500))`)
			if !page.MustEval(`()=>{const s=readState();return s.elapsed_ms===0&&s.actions===0&&document.querySelector('[data-player-start]').textContent==='Spiel starten'}`).Bool() {
				t.Fatalf("game advanced before localized Start: %s", page.MustEval(`()=>({state:readState(),label:document.querySelector('[data-player-start]').textContent,lang:document.documentElement.lang})`).String())
			}
			reports := filepath.Join("..", "..", "reports", "game-player-ui")
			os.MkdirAll(reports, 0750)
			page.MustScreenshot(filepath.Join(reports, mode+"-intro.png"))
			page.Keyboard.MustType(input.Enter)
			page.MustWait(`()=>document.querySelector('[data-player-panel]').hidden&&readState().elapsed_ms>0`)
			if !page.MustEval(`()=>document.querySelector('[data-player-touch]').hidden&&!document.querySelector('[data-hud]')?.textContent.includes('forest')`).Bool() {
				t.Fatal("desktop retained instruction card or touch controls")
			}
			page.MustSetViewport(390, 844, 1, false)
			if !page.MustEval(`()=>document.querySelector('[data-player-touch]').hidden`).Bool() {
				t.Fatal("narrow desktop enabled touch controls")
			}
			// Pause through a keyboard key, then resume through the menu. No time,
			// action or timer may advance while help is being read.
			page.Keyboard.MustType(input.KeyP)
			page.MustWait(`()=>!document.querySelector('[data-player-panel]').hidden`)
			page.MustEval(`()=>{window.pausedState=readState();}`)
			page.MustEval(`()=>document.activeElement.blur()`)
			page.Keyboard.MustType(input.Space)
			page.MustEval(`()=>new Promise(r=>setTimeout(r,250))`)
			if !page.MustEval(`()=>readState().elapsed_ms===pausedState.elapsed_ms&&readState().actions===pausedState.actions`).Bool() {
				t.Fatal("pause did not freeze simulation")
			}
			page.MustElement("[data-player-start]").MustClick()
			maxTouch := 3
			if err := (proto.EmulationSetTouchEmulationEnabled{Enabled: true, MaxTouchPoints: &maxTouch}).Call(page); err != nil {
				t.Fatal(err)
			}
			page.MustWait(`()=>!document.querySelector('[data-player-touch]').hidden`)
			if !page.MustEval(`()=>[...document.querySelectorAll('[data-player-actions] button')].every(b=>{const r=b.getBoundingClientRect();return r.width>=44&&r.height>=44&&r.right<=innerWidth&&r.bottom<=innerHeight})`).Bool() {
				t.Fatal("touch actions too small or outside viewport")
			}
			if mode == "board" {
				if !page.MustEval(`()=>document.querySelector('[data-player-stick]').hidden&&document.querySelectorAll('[data-player-actions] button').length===0`).Bool() {
					t.Fatal("board received redundant movement/action controls")
				}
				point := page.MustEval(`()=>{const c=document.querySelector('canvas').getBoundingClientRect();return {x:c.x+c.width*360/960,y:c.y+c.height*180/540}}`)
				page.Touch.MustTap(point.Get("x").Num(), point.Get("y").Num())
				page.MustWait(`()=>readState().actions===1`)
			} else {
				page.MustEval(`()=>{window.beforeTouch=readState();}`)
				position := page.MustEval(`()=>{const s=document.querySelector('[data-player-stick]').getBoundingClientRect(),a=document.querySelector('[data-player-key="SPACE"]').getBoundingClientRect();return {x:s.x+s.width*.88,y:s.y+s.height/2,ax:a.x+a.width/2,ay:a.y+a.height/2}}`)
				one, two := float64(1), float64(2)
				stick := &proto.InputTouchPoint{X: position.Get("x").Num(), Y: position.Get("y").Num(), ID: &one}
				action := &proto.InputTouchPoint{X: position.Get("ax").Num(), Y: position.Get("ay").Num(), ID: &two}
				page.Touch.MustStart(stick, action)
				page.MustWait(`()=>Math.abs(readState().player_x-beforeTouch.player_x)>2&&readState().actions>beforeTouch.actions`)
				page.Touch.MustCancel()
				page.MustEval(`()=>new Promise(r=>setTimeout(r,150))`)
				page.MustEval(`()=>{window.releasedState=readState();}`)
				page.MustEval(`()=>new Promise(r=>setTimeout(r,300))`)
				if !page.MustEval(`()=>Math.abs(readState().player_x-releasedState.player_x)<.1&&readState().actions===releasedState.actions`).Bool() {
					t.Fatal("cancelled touch remained held")
				}
				if mode == "fps" {
					page.MustEval(`()=>{window.beforeLook=readState();}`)
					page.Touch.MustStart(&proto.InputTouchPoint{X: 230, Y: 300, ID: &one})
					page.Touch.MustMove(&proto.InputTouchPoint{X: 280, Y: 320, ID: &one})
					page.Touch.MustEnd()
					if !page.MustEval(`()=>Math.abs(readState().aim-beforeLook.aim)>.05&&readState().actions===beforeLook.actions`).Bool() {
						t.Fatal("touch look did not aim independently of fire")
					}
				}
			}
			page.MustScreenshot(filepath.Join(reports, mode+"-touch.png"))
			page.Keyboard.MustType(input.KeyR)
			page.MustWait(`()=>!!document.querySelector('[data-player-panel]')&&document.querySelector('[data-player-panel]').hidden&&readState().actions===0`)
			if n := page.MustEval(`()=>document.querySelectorAll('[data-player-ui]').length`).Int(); n != 1 {
				t.Fatalf("restart leaked player UI: %d", n)
			}
			if errors := page.MustEval(`()=>errors`).String(); errors != "[]" {
				t.Fatal(errors)
			}
		})
	}
}
