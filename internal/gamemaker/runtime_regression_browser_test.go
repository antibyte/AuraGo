package gamemaker

import (
	"context"
	"fmt"
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

func runtimeRegressionPage(t *testing.T, browser *rod.Browser, mode string, sources map[string]string) *rod.Page {
	t.Helper()
	dimension := "3d"
	if mode == "board" || mode == "minimal" {
		dimension = "2d"
	}
	service := newTestService(t)
	project := createTestProject(t, service, dimension)
	root := filepath.Join(service.opts.WorkspacePath, project.ProjectKey)
	if err := WriteScaffold(root, project); err != nil {
		t.Fatal(err)
	}
	plan := ExampleGamePlan(project)
	plan.Template, plan.Assets = mode, nil
	if err := installGameTemplate(root, plan); err != nil {
		t.Fatal(err)
	}
	for name, source := range sources {
		if err := os.WriteFile(filepath.Join(root, "src", name), []byte(source), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if result := buildDirectory(context.Background(), root, 150, 64<<20); !result.OK {
		t.Fatal(result.Diagnostics)
	}
	publishExportFixture(t, service, project, root)
	extracted := t.TempDir()
	for path, data := range readExportFixture(t, service, project) {
		dest, _, err := secureJoin(extracted, path, true)
		if err != nil {
			t.Fatal(err)
		}
		if err = os.MkdirAll(filepath.Dir(dest), 0750); err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(dest, data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	server := httptest.NewServer(http.StripPrefix("/play/", http.FileServer(http.Dir(extracted))))
	t.Cleanup(server.Close)
	page := browser.MustPage("about:blank").Timeout(30 * time.Second)
	t.Cleanup(func() { page.Close() })
	page.MustSetViewport(1280, 720, 1, false)
	page.MustEvalOnNewDocument(`window.errors=[];addEventListener('error',e=>errors.push(e.message));addEventListener('unhandledrejection',e=>errors.push(String(e.reason)));window.readState=()=>{const b=window.__AURAGO_GAME_TEST__;return b.kind==='three'?b.snapshot():{...b.state}}`)
	page.MustNavigate(server.URL + "/play/").MustWaitLoad()
	page.MustWait(`()=>!!window.__AURAGO_GAME_TEST__&&!!document.querySelector('[data-player-start]')`)
	t.Cleanup(func() {
		if errors := page.MustEval(`()=>errors`).String(); errors != "[]" {
			t.Error(errors)
		}
	})
	return page
}

func TestRuntimeInputRegressionBrowser(t *testing.T) {
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
	start := func(page *rod.Page) {
		page.Keyboard.MustType(input.Enter)
		page.MustWait(`()=>document.querySelector('[data-player-panel]').hidden`)
	}
	t.Run("locked_mouse", func(t *testing.T) {
		page := runtimeRegressionPage(t, browser, "fps", map[string]string{"main.ts": "import {startGame} from './common';startGame({mode:'fps',objects:[],speed:6,goal:3,duration:0});"})
		start(page)
		page.MustElement("canvas").MustClick()
		page.MustWait(`()=>document.pointerLockElement===document.querySelector('canvas')`)
		for i := 0; i < 3; i++ {
			page.MustEval(`()=>{window.before=readState()}`)
			page.MustWait(`()=>readState().elapsed_ms>before.elapsed_ms+250`)
			page.Mouse.MustClick(proto.InputMouseButtonLeft)
			page.MustWait(`()=>readState().actions===before.actions+1`)
			if !page.MustEval(`()=>readState().ammo===before.ammo-1&&errors.length===0`).Bool() {
				t.Fatal("locked mouse did not fire exactly one shot")
			}
		}
	})
	t.Run("board_lifecycle", func(t *testing.T) {
		page := runtimeRegressionPage(t, browser, "board", nil)
		page.MustEval(`()=>{window.boardState=()=>{const s=__AURAGO_GAME_TEST__.scene;return JSON.stringify({selected:s.selected,marks:s.marks,actions:s.state.actions,turns:s.state.turns,score:s.state.score})}}`)
		click := func(index int) {
			point := page.MustEval(`i=>{const r=document.querySelector('canvas').getBoundingClientRect(),c=__AURAGO_GAME_TEST__.scene.cells[i];return {x:r.x+r.width*c.x/960,y:r.y+r.height*c.y/540}}`, index)
			page.Mouse.MustMoveTo(point.Get("x").Num(), point.Get("y").Num()).MustClick(proto.InputMouseButtonLeft)
		}
		blocked := func() {
			before := page.MustEval(`()=>boardState()`).Str()
			click(2)
			if after := page.MustEval(`()=>boardState()`).Str(); after != before {
				t.Fatalf("blocked board input changed state: before=%s after=%s", before, after)
			}
		}
		blocked()
		start(page)
		page.Keyboard.MustType(input.KeyP)
		page.MustWait(`()=>__AURAGO_GAME_TEST__.scene.manualPause`)
		blocked()
		page.Keyboard.MustType(input.KeyP)
		page.MustWait(`()=>!__AURAGO_GAME_TEST__.scene.manualPause`)
		page.MustEval(`()=>window.postMessage({type:'aurago:game:active',active:false},'*')`)
		page.MustWait(`()=>!__AURAGO_GAME_TEST__.scene.previewActive`)
		blocked()
		page.MustEval(`()=>window.postMessage({type:'aurago:game:active',active:true},'*')`)
		page.MustWait(`()=>__AURAGO_GAME_TEST__.scene.previewActive`)
		click(0)
		page.MustWait(`()=>readState().actions===1`)
		page.Keyboard.MustType(input.Escape)
		page.MustWait(`()=>readState().ended===1&&!!document.querySelector('[data-game-result]')`)
		blocked()
	})
	for _, mode := range []string{"fps", "space"} {
		for _, hook := range []string{"", "return false", "return true", "return api.state.actions<1"} {
			t.Run(mode+"/"+hook, func(t *testing.T) {
				action := ""
				if hook != "" {
					action = ",action(api){(window as any).actionCalls=((window as any).actionCalls||0)+1;" + hook + ";}"
				}
				page := runtimeRegressionPage(t, browser, mode, map[string]string{"main.ts": fmt.Sprintf("import {startGame} from './common';startGame({mode:%q,objects:[],speed:6,goal:3,duration:0%s});", mode, action)})
				start(page)
				page.MustEval(`()=>{window.before=readState()}`)
				if err := page.Keyboard.Press(input.Space); err != nil {
					t.Fatal(err)
				}
				page.MustWait(`()=>readState().elapsed_ms>before.elapsed_ms+650`)
				if err := page.Keyboard.Release(input.Space); err != nil {
					t.Fatal(err)
				}
				shots := page.MustEval(`()=>readState().actions`).Int()
				if (hook == "return false" && shots != 0) || (hook == "return api.state.actions<1" && shots != 1) || ((hook == "" || hook == "return true") && shots < 2) {
					t.Fatalf("held action ignored hook %q: %s", hook, page.MustEval(`()=>readState()`).String())
				}
				if hook != "" && !page.MustEval(`()=>window.actionCalls<=1+Math.ceil((readState().elapsed_ms-before.elapsed_ms)/220)`).Bool() {
					t.Fatal("held hook bypassed the firing cooldown")
				}
				if mode == "fps" && page.MustEval(`()=>readState().ammo`).Int() != 8-shots {
					t.Fatal("ammo does not match permitted shots")
				}
			})
		}
	}
	t.Run("restart_active_level", func(t *testing.T) {
		page := runtimeRegressionPage(t, browser, "space", map[string]string{
			"main.ts":    "import {startGame} from './common';startGame({mode:'space',objects:[],speed:6,goal:3,duration:0,setup(api){(window as any).activeStage=api.levelIndex;}});",
			"scene.json": `{"schema_version":1,"dimension":"3d","levels":[{"id":"inside","active":false},{"id":"main","active":true}],"nodes":[]}`,
		})
		start(page)
		page.MustWait(`()=>window.activeStage===1&&readState().elapsed_ms>500`)
		for i := 0; i < 3; i++ {
			page.Keyboard.MustType(input.KeyR)
			page.MustWait(`()=>readState().elapsed_ms<200`)
			if !page.MustEval(`()=>window.activeStage===1&&document.querySelectorAll('canvas').length===1&&document.querySelectorAll('[data-player-ui]').length===1`).Bool() {
				t.Fatal("restart selected the inactive level or leaked its renderer")
			}
			page.MustWait(`()=>readState().elapsed_ms>500`)
		}
	})
	t.Run("dynamic_cleanup_2d", func(t *testing.T) {
		page := runtimeRegressionPage(t, browser, "minimal", map[string]string{
			"main.ts":    "import {GameScene,start} from './common';class Main extends GameScene {action(){super.action();this.builder.spawnProjectile({position:[200,200,0],ttl:.01});}}start(Main);",
			"scene.json": `{"schema_version":1,"dimension":"2d","levels":[{"id":"main","active":true}],"world_bounds":{"min":[0,0,0],"max":[960,540,0]},"nodes":[{"id":"player","kind":"player","position":[100,100,0],"size":[10,10,0]}]}`,
		})
		start(page)
		resources := `()=>{const s=__AURAGO_GAME_TEST__.scene;return JSON.stringify({records:s.builder.nodes.size,objects:s.gameObjects.length,visuals:s.visuals.length,children:s.children.list.length,bodies:s.physics.world.bodies.entries.length})}`
		before := page.MustEval(resources).Str()
		for i := 1; i <= 5; i++ {
			page.Keyboard.MustType(input.Space)
			page.MustWait(`n=>readState().actions===n&&__AURAGO_GAME_TEST__.scene.builder.nodes.size===1`, i)
			if after := page.MustEval(resources).Str(); after != before {
				t.Fatalf("expired projectile retained renderer resources: before=%s after=%s", before, after)
			}
		}
	})
}
