package gamemaker

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
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
		{"floor_movement", "platformer", "passed"}, {"floor_pickup", "platformer", "passed"},
		{"scene_pickup", "platformer", "passed"}, {"pickup_counter_only", "platformer", "unavailable"},
		{"floor_disabled", "platformer", "failed"}, {"floor_embedded", "platformer", "unavailable"},
		{"catalog_coin", "platformer", "passed"}, {"unknown_coin", "platformer", "unavailable"}, {"explicit_coin", "platformer", "unavailable"},
		{"fps_offset", "three", "passed"}, {"fps_cover", "three", "passed"}, {"fps_sealed", "three", "unavailable"},
		{"space_fixed", "three", "passed"}, {"space_camera", "three", "passed"}, {"space_camera_moving", "three", "passed"},
		{"space_legacy_fixed", "three", "passed"}, {"space_legacy_camera", "three", "passed"},
		{"space_no_fire", "three", "unavailable"}, {"space_counter_only", "three", "unavailable"},
		{"space_occluded", "three", "unavailable"}, {"space_invalid_ray", "three", "unavailable"},
		{"ground_route", "three", "passed"},
		{"model_asset_target", "three", "passed"},
		{"move_blocked_right", "topdown", "passed"}, {"move_disabled", "topdown", "failed"},
		{"paused", "topdown", "unavailable"},
		{"manual_player", "minimal", "passed"},
		{"terminal_goal_actions", "topdown", "passed"}, {"terminal_goal_outcome", "topdown", "passed"},
		{"terminal_near_goal", "topdown", "passed"}, {"terminal_far_goal", "topdown", "unavailable"},
		{"terminal_lost_goal", "topdown", "unavailable"}, {"terminal_preended", "topdown", "unavailable"},
		{"terminal_three_goal", "three", "passed"}, {"terminal_three_elevated", "three", "unavailable"},
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
			if tc.name == "manual_player" {
				plan.Assets = []PlanAsset{{Role: "player", PackID: "blocks-and-balls", Version: "2", AssetID: "paddle_01"}}
			}
			scenario := requiredScenarios(tc.base)[2]
			if tc.name == "manual_player" {
				scenario = requiredScenarios(tc.base)[0]
			}
			if tc.name == "paused" {
				scenario.Steps = append([]GameTestStep{{Action: "key", Key: "P", MS: 100}}, scenario.Steps...)
			}
			if tc.name == "move_blocked_right" || tc.name == "move_disabled" || tc.name == "floor_movement" || tc.name == "floor_disabled" || tc.name == "floor_embedded" {
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
				if strings.HasPrefix(tc.name, "terminal_three_") {
					scene.Nodes = scene.Nodes[:2]
					scene.Nodes[1].Position = Vec3{0, .5, 4}
					if tc.name == "terminal_three_elevated" {
						scene.Nodes[1].Position[1] = 4
					}
					scene.Colliders, scene.Placements = nil, nil
				}
				if strings.HasPrefix(tc.name, "space_") {
					scene.Nodes = scene.Nodes[:2]
					scene.Colliders = nil
					scene.WorldBounds = SceneBounds{Min: Vec3{-25, 0, -20}, Max: Vec3{25, 30, 80}}
					scene.Nodes[0].Position = Vec3{0, 4, 0}
					scene.Nodes[1].Position = Vec3{-8, 6, 26}
					scene.Nodes[1].Size = Vec3{2, .7, 3}
					scene.Placements[0].Position = scene.Nodes[1].Position
					if tc.name == "space_occluded" {
						scene.Nodes = append(scene.Nodes, SceneNode{ID: "wall", Kind: "obstacle", Position: Vec3{0, 15, 15}, Size: Vec3{50, 30, 1}})
						scene.Colliders = []SceneCollider{{ID: "wall-body", NodeID: "wall", Shape: "box", Extents: Vec3{25, 15, .5}}}
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
			if tc.name == "manual_player" {
				// The public manual example must not create a second sprite when
				// common.ts already contains an accepted player asset binding.
				for _, name := range []string{"sheet.png", "sheet.json"} {
					data, err := bundledAssetPackFile("blocks-and-balls", name)
					if err != nil {
						t.Fatal(err)
					}
					path := filepath.Join(root, "assets/builtin/blocks-and-balls/2", name)
					if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(path, data, 0600); err != nil {
						t.Fatal(err)
					}
				}
				detail, err := (&Service{}).describeAsset("blocks-and-balls", "paddle_01", "")
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(root, "src/main.ts"), []byte(detail.Example), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if tc.name == "scene_pickup" {
				// Scene tools can add nodes after plan acceptance. Collection uses
				// the builder's pickup counter, while combat hits must remain zero.
				scene := Scene{SchemaVersion: 1, Dimension: "2d", Seed: 41, Levels: []SceneLevel{{ID: "main", Active: true}}, WorldBounds: SceneBounds{Min: Vec3{0, 0, 0}, Max: Vec3{960, 540, 0}},
					Nodes: []SceneNode{
						{ID: "player", Kind: "player", Position: Vec3{160, 450, 0}, Size: Vec3{28, 40, 0}},
						{ID: "ground", Kind: "obstacle", Position: Vec3{480, 520, 0}, Size: Vec3{960, 40, 0}},
						{ID: "coin", Kind: "entity", Position: Vec3{280, 470, 0}, Size: Vec3{20, 20, 0}},
					},
					Colliders:  []SceneCollider{{ID: "floor", NodeID: "ground", Shape: "box", Extents: Vec3{480, 20, 0}}},
					Placements: []ScenePlacement{{ID: "coin-art", NodeID: "coin", AssetRole: "item", Behavior: "collect", Position: Vec3{280, 470, 0}}},
				}
				data, err := MarshalSceneJSON(scene)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(root, SceneFilePath), data, 0600); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(root, "src/mechanics.json"), []byte(`{"blocks":[{"id":"walk","kind":"movement","target":"player","params":{"mode":"platformer","speed":240,"gravity":900}}]}`), 0600); err != nil {
					t.Fatal(err)
				}
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
			case "pickup_counter_only":
				replace("this.physics.add.overlap(this.player,coin,", "this.physics.add.overlap(this.player,this.physics.add.group(),")
				replace("this.state.actions++;", "this.state.actions++;this.feedback('pickup',this.player);")
			case "floor_movement", "floor_pickup", "floor_disabled", "floor_embedded":
				replace("160, 450, 28, 40", "80, 450, 28, 40")
				replace("480, 520, 960, 40", "760, 500, 1600, 40")
				replace("this.physics.add.collider(this.player, ground);", "const solids=this.physics.add.staticGroup();solids.add(ground);this.physics.add.collider(this.player, solids);")
				if tc.name == "floor_disabled" {
					replace("this.inputKeys.vector().x * 240", "0")
				}
				if tc.name == "floor_embedded" {
					replace("80, 450, 28, 40", "80, 470, 28, 40")
				}
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
			case "occupied_cell":
				replace("this.marks=Array(9).fill(0);", "this.marks=Array(9).fill(0);this.marks[0]=1;")
			}
			if dimension == "3d" {
				mode := "fps"
				if strings.HasPrefix(tc.name, "space_") {
					mode = "space"
					commonPath := filepath.Join(root, "src", "common.ts")
					common, err := os.ReadFile(commonPath)
					if err != nil {
						t.Fatal(err)
					}
					if strings.Contains(tc.name, "legacy") {
						legacy, err := os.ReadFile("testdata/legacy-three-observer.ts")
						if err != nil {
							t.Fatal(err)
						}
						start, end := bytes.Index(common, []byte("  function observeTargets(){")), bytes.Index(common, []byte("  function draw("))
						common = append(append(append([]byte{}, common[:start]...), legacy...), common[end:]...)
						if strings.Contains(tc.name, "camera") {
							common = bytes.Replace(common, []byte("if(config.mode==='fps')look.setFromCamera({x:0,y:0},camera);"), []byte("if(config.mode==='fps'||config.mode==='space'){camera.updateMatrixWorld();look.setFromCamera({x:0,y:0},camera)}"), 1)
						}
					}
					if tc.name != "space_fixed" && tc.name != "space_legacy_fixed" {
						// Custom camera-directed shooting; both fire and the observer use it.
						common = bytes.Replace(common, []byte("if(config.mode==='fps'){camera.updateMatrixWorld();caster.setFromCamera"), []byte("if(config.mode==='fps'||config.mode==='space'){camera.updateMatrixWorld();caster.setFromCamera"), 1)
					}
					if tc.name == "space_no_fire" {
						common = bytes.Replace(common, []byte("function fire(){"), []byte("function fire(){return;"), 1)
					}
					if tc.name == "space_counter_only" {
						common = bytes.Replace(common, []byte("function fire(){"), []byte("function fire(){sceneState.hits++;return;"), 1)
					}
					if tc.name == "space_invalid_ray" {
						common = bytes.Replace(common, []byte("},aim_ray,targets,"), []byte("},aim_ray:{origin:{x:0,y:0,z:0},direction:{x:0,y:0,z:0}},targets,"), 1)
					}
					if err := os.WriteFile(commonPath, common, 0600); err != nil {
						t.Fatal(err)
					}
				}
				if tc.name == "ground_route" || tc.name == "model_asset_target" {
					mode = "exploration"
				}
				source = []byte("import {startGame} from './common';startGame({mode:'" + mode + "',objective:'Reach the target',speed:6,goal:1,duration:0,objects:[]});")
				if strings.HasPrefix(tc.name, "space_") {
					source = bytes.Replace(source, []byte("speed:6"), []byte("speed:14"), 1)
				}
				if tc.name == "space_camera_moving" {
					source = bytes.Replace(source, []byte("startGame({"), []byte("let elapsed=0;startGame({"), 1)
					source = bytes.Replace(source, []byte("objects:[]"), []byte("objects:[],reset(){elapsed=0},step(dt,api){elapsed+=dt;api.builder.nodes.get('target').object.position.x=-8+Math.sin(elapsed*3)*2;}"), 1)
				}
				if tc.name == "model_asset_target" {
					source = bytes.Replace(source, []byte("objects:[]"), []byte("objects:[],setup(api){api.builder.nodes.get('target').object.userData.assetID='crystal';}"), 1)
				}
			}
			if strings.HasPrefix(tc.name, "terminal_") {
				// A custom game can end on contact without marking or removing its
				// goal. The driver must observe that final contact before stopping.
				scenario = GameScenario{ID: "open_chest", Metric: "actions", Compare: "increased", Steps: []GameTestStep{{Action: "target", Target: "goal", Mode: "reach", MS: 2500}}}
				source = []byte(`import {GameScene,start} from './common';
class ContactGoal extends GameScene {
  chest:any;
  setup(){
    this.player=this.body(240,270,28,32,0x5eead4,false,'player');
    this.chest=this.body(420,270,32,24,0xd97706,true,'goal');
    this.physics.add.overlap(this.player,this.chest,()=>{this.state.actions++;this.end(true)});
  }
}
start(ContactGoal);`)
				switch tc.name {
				case "terminal_goal_outcome":
					scenario.Metric, scenario.Compare, scenario.Value = "outcome", "equals", 1
				case "terminal_near_goal":
					scenario.Steps[0].Mode = "interact"
					// A solid chest stops movement at its edge. Only the normal action
					// can open it; the final bodies never overlap.
					source = bytes.ReplaceAll(source, []byte("this.physics.add.overlap(this.player,this.chest,()=>{this.state.actions++;this.end(true)});"), []byte("this.physics.add.collider(this.player,this.chest);"))
					source = bytes.Replace(source, []byte("chest:any;"), []byte("chest:any; action(){if(Math.abs(this.player.x-this.chest.x)<36){this.state.actions++;this.end(true)}}"), 1)
				case "terminal_far_goal":
					// A global counter and victory while moving far from the goal
					// must not substitute for evidence of the targeted interaction.
					source = bytes.Replace(source, []byte("chest:any;"), []byte("chest:any; step(dt:number){super.step(dt);if(this.player.x>250){this.state.actions++;this.end(true)}}"), 1)
				case "terminal_lost_goal":
					source = bytes.ReplaceAll(source, []byte("this.end(true)"), []byte("this.end(false)"))
				case "terminal_preended":
					scenario.Metric, scenario.Compare, scenario.Value = "outcome", "equals", 1
					source = bytes.Replace(source, []byte("this.physics.add.overlap"), []byte("this.player.setPosition(420,270);this.end(true);this.physics.add.overlap"), 1)
				case "terminal_three_goal", "terminal_three_elevated":
					scenario.Metric, scenario.Compare, scenario.Value = "outcome", "equals", 1
					scenario.Steps[0].Target = "target"
					source = []byte(`import {startGame} from './common';
startGame({mode:'exploration',speed:6,goal:1,duration:0,objects:[],
  step(dt,api){const goal=api.builder.nodes.get('target').object;
    if(Math.abs(api.player.position.z-goal.position.z)<.7)api.win();
  }
});`)
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
			if tc.name == "manual_player" {
				count := page.MustEval(`()=>window.__AURAGO_GAME_TEST__.scene.children.list.filter(o=>o.active&&o.type==='Sprite'&&o.texture.key==='blocks-and-balls@2').length`).Int()
				if count != 1 {
					t.Fatalf("manual example drew %d player sprites, want one", count)
				}
			}
			if checks[0].Status != tc.want {
				t.Fatalf("want %s: %+v", tc.want, checks)
			}
			if tc.name == "scene_pickup" && (report.Observations[0].After["hits"] != 0 || report.Observations[0].After["pickup_events"] != 1) {
				t.Fatalf("collection must not count as combat: %+v", report.Observations[0])
			}
			t.Logf("%s: %+v", tc.name, checks)
		})
	}
}
