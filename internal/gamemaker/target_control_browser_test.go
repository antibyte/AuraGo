package gamemaker

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
)

func TestTargetControlBrowser(t *testing.T) {
	if os.Getenv("GAMEMAKER_TARGET_BROWSER") != "1" {
		t.Skip("set GAMEMAKER_TARGET_BROWSER=1 for real targeted inputs")
	}
	bin := os.Getenv("CHROME_BIN")
	if bin == "" {
		bin = "C:/Program Files/Google/Chrome/Application/chrome.exe"
	}
	l := launcher.New().Bin(bin).Headless(true).NoSandbox(true).Set("enable-unsafe-swiftshader")
	browser := rod.New().ControlURL(l.MustLaunch()).MustConnect()
	defer l.Cleanup()
	defer browser.Close()
	for _, tc := range []struct{ name, base, want string }{
		{"moving_enemy", "shooter", "passed"}, {"missing_enemy", "shooter", "unavailable"}, {"counter_only", "shooter", "unavailable"},
		{"around_wall", "topdown", "passed"}, {"sealed_wall", "topdown", "unavailable"}, {"interaction", "topdown", "passed"},
		{"raised_collectible", "platformer", "passed"}, {"occupied_cell", "board", "passed"}, {"natural_miss", "blocks", "passed"},
		{"catalog_coin", "platformer", "passed"}, {"unknown_coin", "platformer", "unavailable"}, {"explicit_coin", "platformer", "unavailable"},
		{"fps_offset", "three", "passed"}, {"fps_cover", "three", "passed"}, {"fps_sealed", "three", "unavailable"},
		{"ground_route", "three", "passed"},
		{"model_asset_target", "three", "passed"},
		{"move_blocked_right", "topdown", "passed"}, {"move_disabled", "topdown", "failed"},
		{"paused", "topdown", "unavailable"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			dimension := "2d"
			if tc.base == "three" {
				dimension = "3d"
			}
			project := Project{Name: tc.name, Dimension: dimension}
			if err := WriteScaffold(root, project); err != nil {
				t.Fatal(err)
			}
			plan := ExampleGamePlan(project)
			plan.Template = tc.base
			plan.Assets = nil
			scenario := requiredScenarios(tc.base)[2]
			if tc.name == "paused" {
				scenario.Steps = append([]GameTestStep{{Action: "key", Key: "P", MS: 100}}, scenario.Steps...)
			}
			if tc.name == "move_blocked_right" || tc.name == "move_disabled" {
				scenario = requiredScenarios(tc.base)[0]
			}
			if tc.name == "interaction" {
				scenario = requiredScenarios(tc.base)[1]
			}
			if tc.name == "natural_miss" {
				scenario = GameScenario{ID: "natural_loss", Metric: "lives", Compare: "decreased", Steps: []GameTestStep{{Action: "target", Target: "ball", Mode: "avoid", MS: 4000}}}
			}
			if tc.name == "catalog_coin" || tc.name == "unknown_coin" || tc.name == "explicit_coin" {
				scenario = GameScenario{ID: "collect_coin_check", Metric: "pickup_events", Compare: "increased", Steps: []GameTestStep{{Action: "target", Target: "coin", Mode: "reach", MS: 4000}}}
			}
			if dimension == "3d" {
				at := Vec3{3, 1, 7}
				behavior := "destroy"
				mode := "aim"
				if tc.name == "ground_route" || tc.name == "model_asset_target" {
					at = Vec3{0, .5, 6}
					behavior = "collect"
					mode = "reach"
				}
				scene := Scene{SchemaVersion: 1, Dimension: "3d", Seed: 41, Levels: []SceneLevel{{ID: "main", Active: true}}, WorldBounds: SceneBounds{Min: Vec3{-12, -1, -8}, Max: Vec3{12, 15, 30}},
					Nodes:      []SceneNode{{ID: "player", Kind: "player", Position: Vec3{0, 0, 0}, Size: Vec3{.6, 1.7, .6}}, {ID: "target", Kind: "entity", Position: at, Size: Vec3{1, 1, 1}, Properties: map[string]any{"color": 0xff9922}}},
					Placements: []ScenePlacement{{ID: "target-art", NodeID: "target", AssetRole: "enemy", Behavior: behavior, Position: at}},
				}
				if tc.name != "fps_offset" {
					width := 2.0
					if tc.name == "fps_sealed" {
						width = 24
					}
					scene.Nodes = append(scene.Nodes, SceneNode{ID: "wall", Kind: "obstacle", Position: Vec3{0, 2, 3}, Size: Vec3{width, 4, .8}})
					scene.Colliders = []SceneCollider{{ID: "wall-body", NodeID: "wall", Shape: "box", Extents: Vec3{width / 2, 2, .4}}}
					if tc.name != "ground_route" {
						scene.Nodes[1].Position = Vec3{0, 1, 7}
						scene.Placements[0].Position = scene.Nodes[1].Position
					}
				}
				plan.Scene = &scene
				scenario = GameScenario{ID: "target_rule", Metric: "hits", Compare: "increased", Steps: []GameTestStep{{Action: "target", Target: "target", Mode: mode, MS: 4000}}}
				if tc.name == "ground_route" || tc.name == "model_asset_target" {
					scenario.Metric = "pickup_events"
				}
				if tc.name == "model_asset_target" {
					scenario.Steps[0].Target = "crystal"
				}
			}
			if err := installGameTemplate(root, plan); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(root, "src/main.ts")
			source, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			replace := func(old, next string) {
				if !bytes.Contains(source, []byte(old)) {
					t.Fatalf("fixture source missing %q", old)
				}
				source = bytes.ReplaceAll(source, []byte(old), []byte(next))
			}
			switch tc.name {
			case "move_blocked_right":
				replace("500,270,70,140", "274,270,40,140")
			case "move_disabled":
				replace("super.step(delta);this.player.setDepth", "this.player.body.setVelocity(0,0);this.player.setDepth")
			case "moving_enemy":
				replace("this.player.x, 100", "650, 100")
				replace("setVelocityY(70)", "setVelocity(-60,70)")
			case "missing_enemy":
				replace("this.spawn();", "")
			case "counter_only":
				replace("this.physics.add.overlap(this.shots, this.enemies,", "this.physics.add.overlap(this.shots, this.physics.add.group(),")
				replace("this.state.actions++;", "this.state.actions++;this.state.hits++;")
			case "around_wall":
				replace("500,270,70,140", "300,270,70,140")
			case "sealed_wall":
				replace("500,270,70,140", "300,270,70,540")
			case "raised_collectible":
				replace("280,470,20,20", "350,390,20,20")
			case "catalog_coin", "unknown_coin", "explicit_coin":
				// Reproduce the metadata attached by body(...,'item') for planned coin art.
				id := "coin"
				if tc.name == "unknown_coin" {
					id = "other-coin"
				}
				replace("const danger =", "coin.__gmAssets=[{role:'item',asset_id:'"+id+"'}]; const danger =")
				if tc.name == "explicit_coin" {
					// An exact but offscreen object must not be replaced by a nearby art alias.
					replace("const danger =", "this.body(2000,470,20,20,0xff0000,true,'coin'); const danger =")
				}
				replace("coin.destroy();", "coin.destroy();this.state.pickup_events++;")
			case "occupied_cell":
				replace("this.marks=Array(9).fill(0);", "this.marks=Array(9).fill(0);this.marks[0]=1;")
			}
			if dimension == "3d" {
				mode := "fps"
				if tc.name == "ground_route" || tc.name == "model_asset_target" {
					mode = "exploration"
				}
				source = []byte("import {startGame} from './common';startGame({mode:'" + mode + "',objective:'Reach the target',speed:6,goal:1,duration:0,objects:[]});")
				if tc.name == "model_asset_target" {
					source = bytes.Replace(source, []byte("objects:[]"), []byte("objects:[],setup(api){api.builder.nodes.get('target').object.userData.assetID='crystal';}"), 1)
				}
			}
			if err := os.WriteFile(path, source, 0600); err != nil {
				t.Fatal(err)
			}
			if build := buildDirectory(context.Background(), root, 150, 64<<20); !build.OK {
				t.Fatal(build.Diagnostics)
			}
			server := httptest.NewServer(http.FileServer(http.Dir(root)))
			defer server.Close()
			page := browser.MustPage("about:blank")
			page.MustEvalOnNewDocument("window.__targetErrors=[];addEventListener('error',e=>window.__targetErrors.push(e.message));addEventListener('unhandledrejection',e=>window.__targetErrors.push(String(e.reason)))")
			page.MustNavigate(server.URL + "/#gm-channel=target-fixture").MustWaitLoad()
			defer page.Close()
			page.MustSetViewport(960, 540, 1, false)
			if err := page.Timeout(10 * time.Second).Wait(rod.Eval("()=>!!window.__AURAGO_GAME_TEST__")); err != nil {
				t.Fatal(err)
			}
			driver, err := runtimeFS.ReadFile("runtime/preview-tests.js")
			if err != nil {
				t.Fatal(err)
			}
			if err := page.AddScriptTag("", string(driver)); err != nil {
				t.Fatal(err)
			}
			page.MustEval(`scenario=>{window.__targetHeld=new Set();addEventListener('keydown',e=>window.__targetHeld.add(e.code));addEventListener('keyup',e=>window.__targetHeld.delete(e.code));window.__targetReport=null;addEventListener('message',e=>{if(e.data?.type==='gameplay')window.__targetReport=e.data});postMessage({source:'aurago-studio',channel:'target-fixture',type:'run-tests',scenarios:[scenario]},'*')}`, scenario)
			if err := page.Timeout(20 * time.Second).Wait(rod.Eval("()=>!!window.__targetReport")); err != nil {
				t.Fatal(err)
			}
			var report PreviewReport
			if err := json.Unmarshal([]byte(page.MustEval("()=>JSON.stringify(window.__targetReport)").Str()), &report); err != nil {
				t.Fatal(err)
			}
			if err := validateGameReport(report); err != nil {
				t.Fatal(err)
			}
			if held := page.MustEval("()=>Array.from(window.__targetHeld).join(',')").Str(); held != "" {
				t.Fatalf("test left keys held: %s", held)
			}
			checks := compareGameObservations([]GameScenario{scenario}, report.Observations)
			if checks[0].Status != tc.want {
				t.Fatalf("want %s: %+v", tc.want, checks)
			}
			t.Logf("%s: %+v", tc.name, checks)
		})
	}
}
