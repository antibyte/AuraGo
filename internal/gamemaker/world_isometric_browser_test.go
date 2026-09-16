package gamemaker

import (
	"context"
	"encoding/json"
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
)

func TestWorldIsometricExportBrowser(t *testing.T) {
	if os.Getenv("GAMEMAKER_WORLD_BROWSER") != "1" {
		t.Skip("set GAMEMAKER_WORLD_BROWSER=1")
	}
	enableWorldReviewPacks(t)
	s := newTestService(t)
	p := createTestProject(t, s, "2d")
	dir := filepath.Join(s.opts.WorkspacePath, p.ProjectKey)
	if err := WriteScaffold(dir, p); err != nil {
		t.Fatal(err)
	}
	write := func(path string, b []byte) {
		t.Helper()
		name := filepath.Join(dir, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(name), 0750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(name, b, 0644); err != nil {
			t.Fatal(err)
		}
	}
	plan := ExampleGamePlan(p)
	plan.Template = "minimal"
	plan.Width = 960
	plan.Height = 540
	plan.Perspective = "isometric"
	plan.SchemaVersion = 4
	scene := isometricFixture()
	scene.Nodes = nil
	scene.Routes = nil
	for y := 0; y < 4; y++ {
		for x := 0; x < 9; x++ {
			z := 0.0
			if x >= 3 {
				z = 1
			}
			id := fmt.Sprintf("floor-%d-%d", x, y)
			scene.Nodes = append(scene.Nodes, SceneNode{ID: id, Kind: "floor", LevelID: "main", Position: Vec3{float64(x), float64(y), z}, Properties: map[string]any{"walkable": true}})
			scene.Placements = append(scene.Placements, ScenePlacement{ID: "art-" + id, NodeID: id, AssetRole: "floor", Behavior: "decorative", Position: Vec3{float64(x), float64(y), z}})
		}
	}
	scene.Routes = []SceneRoute{{ID: "steps", From: "floor-2-1", To: "floor-3-1", Kind: "stairs", LevelID: "main"}}
	scene.Nodes = append(scene.Nodes, SceneNode{ID: "player", Kind: "player", LevelID: "main", Position: Vec3{1.5, 1.5, 0}}, SceneNode{ID: "coin", Kind: "item", LevelID: "main", Position: Vec3{4.5, 1.5, 1}, Properties: map[string]any{"pickup": true}}, SceneNode{ID: "door", Kind: "wall", LevelID: "main", Position: Vec3{5.5, 1.5, 1}, Properties: map[string]any{"solid": true}}, SceneNode{ID: "goal", Kind: "goal", LevelID: "main", Position: Vec3{7.5, 1.5, 1}, Properties: map[string]any{"goal": true}})
	for _, binding := range []struct{ node, role string }{{"player", "player"}, {"door", "door"}} {
		for _, n := range scene.Nodes {
			if n.ID == binding.node {
				scene.Placements = append(scene.Placements, ScenePlacement{ID: "art-" + n.ID, NodeID: n.ID, AssetRole: binding.role, Behavior: "decorative", Position: n.Position})
			}
		}
	}
	plan.Scene = &scene
	// A portal transitions to a separate indoor level, never to overlapping
	// walkable planes. The entrance is normal game contact, not a test teleport.
	for i := range scene.Nodes {
		if scene.Nodes[i].ID == "goal" {
			scene.Nodes[i].Properties = map[string]any{"portal": "inside"}
		}
	}
	for y := 0; y < 3; y++ {
		for x := 0; x < 3; x++ {
			id := fmt.Sprintf("inside-floor-%d-%d", x, y)
			scene.Nodes = append(scene.Nodes, SceneNode{ID: id, Kind: "floor", LevelID: "inside", Position: Vec3{float64(x), float64(y), 0}, Properties: map[string]any{"walkable": true}})
			scene.Placements = append(scene.Placements, ScenePlacement{ID: "art-" + id, NodeID: id, AssetRole: "floor", Behavior: "decorative", Position: Vec3{float64(x), float64(y), 0}})
		}
	}
	scene.Nodes = append(scene.Nodes, SceneNode{ID: "inside-player", Kind: "player", LevelID: "inside", Position: Vec3{.5, 1.5, 0}}, SceneNode{ID: "beacon", Kind: "goal", LevelID: "inside", Position: Vec3{2.5, 1.5, 0}, Properties: map[string]any{"goal": true}})
	scene.Placements = append(scene.Placements, ScenePlacement{ID: "art-inside-player", NodeID: "inside-player", AssetRole: "player", Behavior: "decorative", Position: Vec3{.5, 1.5, 0}})
	plan.Assets = nil
	atlas, err := readAtlasManifest("aurago-isometric")
	if err != nil {
		t.Fatal(err)
	}
	for _, binding := range []struct{ role, id string }{{"floor", "terrain-grass-flat"}, {"player", "people-adventurer"}, {"door", "architecture-village-door"}} {
		selected, err := selectAtlasAssets(atlas, []string{binding.id})
		if err != nil {
			t.Fatal(err)
		}
		base := "assets/builtin/" + atlas.ID + "/" + atlas.Version + "/"
		data, _ := json.Marshal(selected)
		write(base+"assets/"+binding.id+".json", data)
		for _, page := range selected.Atlases {
			data, err := bundledAtlasFile(atlas.ID, page.File)
			if err != nil {
				t.Fatal(err)
			}
			write(base+page.File, data)
		}
		a := selected.Assets[0]
		plan.Assets = append(plan.Assets, PlanAsset{Role: binding.role, PackID: atlas.ID, Version: atlas.Version, AssetID: binding.id, Direction: a.Direction, DisplayHeight: 48, Origin: a.Origin, Collider: "rectangle"})
	}
	data, _ := json.Marshal(plan)
	write(gamePlanPath, data)
	sources, err := gameTemplateSources(plan)
	if err != nil {
		t.Fatal(err)
	}
	for name, data := range sources {
		write("src/"+name, data)
	}
	write("src/main.ts", []byte(`import {GameScene,start} from './common';
import {playAction} from '../vendor/aurago-game-1.js';
class Island extends GameScene {
  isometricContact(node:any){if(node.properties?.portal){this.changeIsometricLevel(node.properties.portal);this.state.rooms_entered=(this.state.rooms_entered||0)+1;}}
  isometricAction(){const player=this.isometric.records.get(this.isoPlayerID),door=this.isometric.records.get('door');if(door&&Math.hypot(player.at[0]-door.at[0],player.at[1]-door.at[1])<1.5){this.isometric.setSolid('door',false);playAction(door.object,'open');this.state.doors_opened=(this.state.doors_opened||0)+1;}}
  paintHUD(){this.hud.setText('ISOMETRIC EXPLORATION\nArrows/WASD: move · Space: open nearby door · P: pause · R: restart\nClimb the terrace, collect the token, open the door and reach the beacon.\nTokens '+this.state.score+' · '+(this.state.outcome===1?'COMPLETE':'Explore'));}
}
start(Island);`))
	built := buildDirectory(context.Background(), dir, 1000, 100<<20)
	if !built.OK {
		t.Fatalf("build: %+v", built)
	}
	publishExportFixture(t, s, p, dir)
	files := readExportFixture(t, s, p)
	extracted := t.TempDir()
	for name, data := range files {
		file := filepath.Join(extracted, filepath.FromSlash(name))
		os.MkdirAll(filepath.Dir(file), 0750)
		if err := os.WriteFile(file, data, 0644); err != nil {
			t.Fatal(err)
		}
	}
	server := httptest.NewServer(http.StripPrefix("/examples/island/", http.FileServer(http.Dir(extracted))))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	bin := os.Getenv("CHROME_BIN")
	if bin == "" {
		bin = "C:/Program Files/Google/Chrome/Application/chrome.exe"
	}
	launch := launcher.New().Context(ctx).Bin(bin).Headless(true).NoSandbox(true).Set("enable-unsafe-swiftshader")
	browser := rod.New().Context(ctx).ControlURL(launch.MustLaunch()).MustConnect()
	defer launch.Cleanup()
	defer browser.Close()
	page := browser.MustPage()
	page.MustEvalOnNewDocument(`globalThis.worldErrors=[];addEventListener('error',e=>worldErrors.push(String(e.message)));addEventListener('unhandledrejection',e=>worldErrors.push(String(e.reason)));`)
	page.MustNavigate(server.URL + "/examples/island/").MustWaitLoad()
	page.MustSetViewport(1366, 768, 1, false)
	page.MustEval(`()=>new Promise(resolve=>setTimeout(resolve,2000))`)
	t.Log(page.MustEval(`()=>({binding:!!globalThis.__AURAGO_GAME_TEST__,errors:worldErrors,html:document.body.innerText,globals:Object.keys(globalThis).filter(k=>/aurago|phaser/i.test(k))})`).JSON("", "  "))
	if root := os.Getenv("GAMEMAKER_WORLD_REPORTS"); root != "" {
		os.MkdirAll(root, 0750)
		page.MustScreenshot(filepath.Join(root, "isometric-start.png"))
	}
	page.MustWait(`()=>Boolean(globalThis.__AURAGO_GAME_TEST__?.scene?.isometric || worldErrors.length)`)
	if errors := page.MustEval(`()=>worldErrors`).JSON("", " "); errors != "[]" {
		t.Fatalf("runtime errors: %s", errors)
	}
	page.MustElement("canvas").MustClick()
	down := func(keys ...input.Key) {
		for _, key := range keys {
			if err := page.Keyboard.Press(key); err != nil {
				t.Fatal(err)
			}
		}
	}
	up := func(keys ...input.Key) {
		for _, key := range keys {
			if err := page.Keyboard.Release(key); err != nil {
				t.Fatal(err)
			}
		}
	}
	down(input.KeyP)
	page.MustWait(`()=>__AURAGO_GAME_TEST__.scene.manualPause`)
	up(input.KeyP)
	beforePause := page.MustEval(`()=>JSON.stringify(__AURAGO_GAME_TEST__.scene.isometric.records.get('player').at)`).Str()
	down(input.ArrowDown, input.ArrowRight)
	time.Sleep(250 * time.Millisecond)
	up(input.ArrowDown, input.ArrowRight)
	if after := page.MustEval(`()=>JSON.stringify(__AURAGO_GAME_TEST__.scene.isometric.records.get('player').at)`).Str(); after != beforePause {
		t.Fatal("paused isometric player moved")
	}
	down(input.KeyP)
	page.MustWait(`()=>!__AURAGO_GAME_TEST__.scene.manualPause`)
	up(input.KeyP)
	down(input.ArrowDown, input.ArrowRight)
	page.MustWait(`()=>__AURAGO_GAME_TEST__.scene.isometric.records.get('player').at[0]>4.5`)
	up(input.ArrowDown, input.ArrowRight, input.Space)
	if !page.MustEval(`()=>__AURAGO_GAME_TEST__.state.score===1&&__AURAGO_GAME_TEST__.scene.isometric.records.get('player').at[2]===1`).Bool() {
		t.Fatal("real input did not collect token through stairs")
	}
	down(input.Space)
	page.MustWait(`()=>__AURAGO_GAME_TEST__.state.doors_opened===1`)
	down(input.ArrowDown, input.ArrowRight)
	page.MustWait(`()=>__AURAGO_GAME_TEST__.state.outcome===1`)
	if !page.MustEval(`()=>__AURAGO_GAME_TEST__.scene.isometric.inspect().level==='inside'&&__AURAGO_GAME_TEST__.state.rooms_entered===1`).Bool() {
		t.Fatal("interior was not reached through the portal")
	}
	up(input.ArrowDown, input.ArrowRight)
	if root := os.Getenv("GAMEMAKER_WORLD_REPORTS"); root != "" {
		os.MkdirAll(root, 0750)
		page.MustScreenshot(filepath.Join(root, "isometric-played.png"))
		data := page.MustEval(`()=>({state:__AURAGO_GAME_TEST__.state,world:__AURAGO_GAME_TEST__.scene.isometric.inspect()})`).JSON("", "  ")
		os.WriteFile(filepath.Join(root, "isometric-played.json"), []byte(data), 0644)
	}
	down(input.KeyR)
	page.MustWait(`()=>__AURAGO_GAME_TEST__.state.score===0&&__AURAGO_GAME_TEST__.state.outcome===0`)
	up(input.KeyR)
	resources := `()=>{const s=__AURAGO_GAME_TEST__.scene;return JSON.stringify({objects:s.isometric.inspect().objects,children:s.children.list.length,postupdate:s.events.listenerCount('postupdate'),hidden:s.game.events.listenerCount('hidden')})}`
	baseline := page.MustEval(resources).Str()
	if page.MustEval(`()=>__AURAGO_GAME_TEST__.scene.isometric.inspect().objects`).Int() != 40 {
		t.Fatal("restart omitted objects")
	}
	for i := 0; i < 3; i++ {
		down(input.ArrowDown, input.ArrowRight)
		page.MustWait(`()=>__AURAGO_GAME_TEST__.scene.isometric.records.get('player').at[0]>1.7`)
		up(input.ArrowDown, input.ArrowRight)
		down(input.KeyR)
		page.MustWait(`()=>__AURAGO_GAME_TEST__.scene.isometric.records.get('player').at[0]===1.5`)
		up(input.KeyR)
		if got := page.MustEval(resources).Str(); got != baseline {
			t.Fatalf("restart leaked resources: before=%s after=%s", baseline, got)
		}
	}
	other := browser.MustPage("about:blank").MustActivate()
	page.MustWait(`()=>document.hidden`)
	other.Close()
	page.MustActivate()
	page.MustWait(`()=>!document.hidden`)
	down(input.ArrowDown, input.ArrowRight)
	page.MustWait(`()=>__AURAGO_GAME_TEST__.scene.isometric.records.get('player').at[0]>1.7`)
	up(input.ArrowDown, input.ArrowRight)
	if got := page.MustEval(resources).Str(); got != baseline {
		t.Fatalf("focus change leaked resources: before=%s after=%s", baseline, got)
	}
}
