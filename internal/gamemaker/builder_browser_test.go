package gamemaker

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"io"
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
)

// Exercise compiled helpers and the actual ZIP writer, with trusted Chrome
// input. These checks do not replace the independent model comparison.
func TestBuilderBrowser(t *testing.T) {
	if os.Getenv("GAMEMAKER_BUILDER_BROWSER") != "1" {
		t.Skip("set GAMEMAKER_BUILDER_BROWSER=1 for real Chrome scene acceptance")
	}
	bin := os.Getenv("CHROME_BIN")
	if bin == "" {
		bin = "C:/Program Files/Google/Chrome/Application/chrome.exe"
	}
	l := launcher.New().Bin(bin).Headless(true).NoSandbox(true).Set("enable-unsafe-swiftshader")
	browser := rod.New().ControlURL(l.MustLaunch()).MustConnect()
	defer l.Cleanup()
	defer browser.Close()
	for _, dimension := range []string{"2d", "3d"} {
		t.Run(dimension, func(t *testing.T) {
			s := newTestService(t)
			project := createTestProject(t, s, dimension)
			dir := filepath.Join(s.opts.WorkspacePath, project.ProjectKey)
			if err := WriteScaffold(dir, project); err != nil {
				t.Fatal(err)
			}
			plan := ExampleGamePlan(project)
			plan.SchemaVersion = 4
			plan.Objective = "Parcel route — deliver the gold parcel around the stone barrier"
			plan.Template = "minimal"
			plan.Assets = nil
			plan.Mechanics = map[string]any{"outcomes": []string{"won"}}
			scene := Scene{SchemaVersion: 1, Dimension: dimension, Seed: 1729,
				Levels:      []SceneLevel{{ID: "main", Active: true}},
				WorldBounds: SceneBounds{Min: Vec3{0, 0, 0}, Max: Vec3{960, 540, 0}},
				Nodes: []SceneNode{
					{ID: "courier", Kind: "player", Position: Vec3{180, 270, 0}, Size: Vec3{24, 24, 0}, Properties: map[string]any{"color": 0x5eead4}},
					{ID: "parcel", Kind: "entity", Position: Vec3{400, 270, 0}, Size: Vec3{28, 28, 0}, Properties: map[string]any{"color": 0xfacc15, "win_when_cleared": true}},
				},
				Placements: []ScenePlacement{{ID: "parcel-art", NodeID: "parcel", AssetRole: "parcel-art", Behavior: "collect", Position: Vec3{400, 270, 0}}},
			}
			if dimension == "3d" {
				plan.Template = "three"
				scene.WorldBounds = SceneBounds{Min: Vec3{-15, 0, -10}, Max: Vec3{15, 15, 35}}
				scene.Nodes[0].Position = Vec3{0, .5, 0}
				scene.Nodes[0].Size = Vec3{1, 1, 1}
				scene.Nodes[1].Position = Vec3{0, .5, 4}
				scene.Nodes[1].Size = Vec3{1, 1, 1}
				scene.Placements[0].Position = scene.Nodes[1].Position
			}
			obstacle := SceneNode{ID: "stone-barrier", Kind: "obstacle", Position: Vec3{300, 270, 0}, Size: Vec3{80, 140, 0}, Properties: map[string]any{"color": 0x475569}}
			extents := Vec3{40, 70, 0}
			if dimension == "3d" {
				obstacle.Position = Vec3{0, 1, 2}
				obstacle.Size = Vec3{1.4, 2, .6}
				extents = Vec3{.7, 1, .3}
			}
			scene.Nodes = append(scene.Nodes, obstacle)
			scene.Colliders = []SceneCollider{{ID: "stone-body", NodeID: obstacle.ID, Shape: "box", Extents: extents}}
			plan.Scene = &scene
			if err := installGameTemplate(dir, plan); err != nil {
				t.Fatal(err)
			}
			if dimension == "2d" {
				// A genuine custom mechanic remains callable beside the mapbuilder.
				code := "import {GameScene,start} from './common'; class Courier extends GameScene { action(){super.action();this.state.score+=7;this.player.setFillStyle(0xa78bfa);} } start(Courier);"
				if err := os.WriteFile(filepath.Join(dir, "src/main.ts"), []byte(code), 0640); err != nil {
					t.Fatal(err)
				}
			}
			if dimension == "3d" {
				code := "import {startGame} from './common';let charge=0;startGame({mode:'exploration',objective:'Deliver the parcel',speed:5,goal:1,duration:0,step(dt,api){if(api.input.isDown('c'))charge+=dt;document.getElementById('game-root').dataset.charge=String(charge)},reset(){charge=0}});"
				if err := os.WriteFile(filepath.Join(dir, "src/main.ts"), []byte(code), 0640); err != nil {
					t.Fatal(err)
				}
			}
			data, _ := json.Marshal(plan)
			if err := os.MkdirAll(filepath.Join(dir, ".aurago"), 0750); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, filepath.FromSlash(gamePlanPath)), data, 0640); err != nil {
				t.Fatal(err)
			}
			result := buildDirectory(context.Background(), dir, 150, 64<<20)
			if !result.OK {
				t.Fatalf("build: %+v", result.Diagnostics)
			}
			if _, err := s.db.Exec(`UPDATE gm_projects SET current_revision=1 WHERE id=?`, project.ID); err != nil {
				t.Fatal(err)
			}
			var output bytes.Buffer
			if _, err := s.WriteExport(context.Background(), project.ID, &output); err != nil {
				t.Fatal(err)
			}
			archive, err := zip.NewReader(bytes.NewReader(output.Bytes()), int64(output.Len()))
			if err != nil {
				t.Fatal(err)
			}
			exportDir := t.TempDir()
			for _, file := range archive.File {
				if strings.Contains(file.Name, "preview-tests") {
					t.Fatal("preview tests leaked into export")
				}
				if file.FileInfo().IsDir() {
					continue
				}
				path := filepath.Join(exportDir, filepath.FromSlash(file.Name))
				if !strings.HasPrefix(path, exportDir+string(os.PathSeparator)) {
					t.Fatal("unsafe ZIP path")
				}
				if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil {
					t.Fatal(err)
				}
				r, err := file.Open()
				if err != nil {
					t.Fatal(err)
				}
				content, err := io.ReadAll(r)
				r.Close()
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, content, 0640); err != nil {
					t.Fatal(err)
				}
			}
			for _, target := range []struct{ name, dir string }{{"preview", dir}, {"export", exportDir}} {
				files := http.FileServer(http.Dir(target.dir))
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if !builderStudioRoute(w, r) {
						files.ServeHTTP(w, r)
					}
				}))
				page := browser.MustPage("about:blank").Timeout(60*time.Second).MustSetViewport(1366, 768, 1, false)
				page.MustEvalOnNewDocument(`window.builderErrors=[];addEventListener('error',e=>builderErrors.push(e.message));addEventListener('unhandledrejection',e=>builderErrors.push(String(e.reason)))`)
				page.MustNavigate(server.URL).MustWaitLoad()
				if err := page.Timeout(15 * time.Second).Wait(rod.Eval(`()=>!!window.__AURAGO_GAME_TEST__`)); err != nil {
					errors := page.MustEval(`()=>window.builderErrors`).String()
					page.Close()
					server.Close()
					t.Fatalf("scene startup: %v; errors=%s", err, errors)
				}
				page.MustActivate()
				time.Sleep(350 * time.Millisecond)
				page.MustElement("canvas").MustClick()
				if dimension == "3d" {
					if err := page.Keyboard.Press(input.KeyC); err != nil {
						t.Fatal(err)
					}
					time.Sleep(450 * time.Millisecond)
					if err := page.Keyboard.Release(input.KeyC); err != nil {
						t.Fatal(err)
					}
					charge := page.MustEval(`()=>Number(document.getElementById('game-root').dataset.charge)`).Num()
					if charge < .15 {
						t.Fatalf("custom charge did not react to real input: %g", charge)
					}
					time.Sleep(150 * time.Millisecond)
					if after := page.MustEval(`()=>Number(document.getElementById('game-root').dataset.charge)`).Num(); after > charge+.06 {
						t.Fatalf("released input kept charging: %g -> %g", charge, after)
					}
				}
				if dimension == "2d" {
					if err := page.Keyboard.Press(input.Space); err != nil {
						t.Fatal(err)
					}
					time.Sleep(400 * time.Millisecond)
					if err := page.Keyboard.Release(input.Space); err != nil {
						t.Fatal(err)
					}
					if err := page.Timeout(3 * time.Second).Wait(rod.Eval(`()=>__AURAGO_GAME_TEST__.state.score===7`)); err != nil {
						t.Fatalf("custom action beside scene failed: %s", page.MustEval(`()=>({state:__AURAGO_GAME_TEST__.state,errors:builderErrors,hidden:document.hidden,focus:document.hasFocus(),active:document.activeElement?.tagName,sys:__AURAGO_GAME_TEST__.scene.sys.settings,loop:{running:__AURAGO_GAME_TEST__.scene.game.loop.running,frame:__AURAGO_GAME_TEST__.scene.game.loop.frame,hasFocus:__AURAGO_GAME_TEST__.scene.game.loop.hasFocus},key:{down:__AURAGO_GAME_TEST__.scene.inputKeys.keys.SPACE.isDown,enabled:__AURAGO_GAME_TEST__.scene.input.keyboard.enabled}})`).String())
					}
				}
				reports, _ := filepath.Abs("../../reports/game-maker-builder/browser")
				os.MkdirAll(reports, 0750)
				page.MustScreenshot(filepath.Join(reports, dimension+"-"+target.name+"-start.png"))
				hold := func(key input.Key, duration time.Duration) {
					t.Helper()
					if err := page.Keyboard.Press(key); err != nil {
						t.Fatal(err)
					}
					time.Sleep(duration)
					if err := page.Keyboard.Release(key); err != nil {
						t.Fatal(err)
					}
				}
				hold(input.KeyP, 150*time.Millisecond)
				position := page.MustEval(`()=>{const b=__AURAGO_GAME_TEST__;return b.kind==='three'?b.snapshot().player_x:b.player.x}`).Num()
				hold(input.KeyD, 350*time.Millisecond)
				if after := page.MustEval(`()=>{const b=__AURAGO_GAME_TEST__;return b.kind==='three'?b.snapshot().player_x:b.player.x}`).Num(); after != position {
					t.Fatal("paused player moved", position, after)
				}
				hold(input.KeyP, 150*time.Millisecond)
				key := input.ArrowRight
				if dimension == "3d" {
					key = input.KeyW
				}
				if err := page.Keyboard.Press(key); err != nil {
					t.Fatal(err)
				}
				time.Sleep(800 * time.Millisecond)
				if err := page.Keyboard.Release(key); err != nil {
					t.Fatal(err)
				}
				blocked := page.MustEval(`()=>{const b=__AURAGO_GAME_TEST__;return b.kind==='three'?b.snapshot().player_y:b.player.x}`).Num()
				if dimension == "2d" && (blocked < 230 || blocked > 251) || dimension == "3d" && (blocked < .5 || blocked > 1.5) {
					t.Fatalf("solid obstacle was not respected: %g", blocked)
				}
				if dimension == "2d" {
					// Wait for actual positions: headless software rendering may run below 60 FPS.
					for _, leg := range []struct {
						key       input.Key
						condition string
					}{
						{input.ArrowDown, "()=>__AURAGO_GAME_TEST__.player.y>=380"},
						{input.ArrowRight, "()=>__AURAGO_GAME_TEST__.player.x>=395"},
					} {
						if err := page.Keyboard.Press(leg.key); err != nil {
							t.Fatal(err)
						}
						deadline := time.Now().Add(5 * time.Second)
						for time.Now().Before(deadline) && !page.MustEval(leg.condition).Bool() {
							time.Sleep(10 * time.Millisecond)
						}
						page.Keyboard.Release(leg.key)
						if !page.MustEval(leg.condition).Bool() {
							t.Fatal("could not follow open route")
						}
					}
					key = input.ArrowUp
				} else {
					hold(input.KeyD, 500*time.Millisecond)
					hold(input.KeyW, 600*time.Millisecond)
					key = input.KeyA
				}
				if err := page.Keyboard.Press(key); err != nil {
					t.Fatal(err)
				}
				waitErr := page.Timeout(6 * time.Second).Wait(rod.Eval(`()=>{const b=__AURAGO_GAME_TEST__;return (b.kind==='three'?b.audit?.():b.scene.builder?.audit?.())?.outcome==='won'}`))
				if err := page.Keyboard.Release(key); err != nil {
					t.Fatal(err)
				}
				if waitErr != nil {
					t.Errorf("%s input did not reach the scene goal: %v", target.name, waitErr)
				}
				page.MustScreenshot(filepath.Join(reports, dimension+"-"+target.name+".png"))
				if err := page.Keyboard.Press(input.KeyR); err != nil {
					t.Fatal(err)
				}
				time.Sleep(400 * time.Millisecond)
				if err := page.Keyboard.Release(input.KeyR); err != nil {
					t.Fatal(err)
				}
				page.MustWait(`()=>{const b=__AURAGO_GAME_TEST__;return (b.kind==='three'?b.audit?.():b.scene.builder?.audit?.())?.outcome==='playing'}`)
				resourceSnapshot := `()=>{const b=__AURAGO_GAME_TEST__,a=b.kind==='three'?b.audit():b.scene.builder.audit();return JSON.stringify(a.resource_counts)}`
				resources := page.MustEval(resourceSnapshot).String()
				for restart := 0; restart < 3; restart++ {
					hold(input.KeyR, 120*time.Millisecond)
					time.Sleep(180 * time.Millisecond)
					if got := page.MustEval(resourceSnapshot).String(); got != resources {
						t.Fatalf("restart leaked resources: before=%s after=%s", resources, got)
					}
				}
				page.MustSetViewport(390, 844, 1, false)
				page.MustScreenshot(filepath.Join(reports, dimension+"-"+target.name+"-touch.png"))
				page.MustClose()
				if dimension == "2d" && target.name == "preview" {
					checkBuilderStudio(t, browser, server.URL, reports)
				}
				server.Close()
			}
		})
	}
}
