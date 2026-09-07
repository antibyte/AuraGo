package gamemaker

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
)

// Opt-in local browser acceptance: GAMEMAKER_BROWSER_TEST=1 go test
// ./internal/gamemaker -run TestGameMakerBrowserFixtures -v -timeout 15m.
// Open the printed loopback URL, then Run all. No provider or network needed.
func TestGameMakerBrowserFixtures(t *testing.T) {
	if os.Getenv("GAMEMAKER_BROWSER_TEST") != "1" {
		t.Skip("requires a real browser")
	}
	root := t.TempDir()
	positive := append(templateNames()[:6], "planned_blocks", "multiball", "sprite_example", "all_packs", "asset_detail", "assembly_detail")
	names := append(append([]string{}, positive...), "overridden_update", "wrapped_bodies", "metadata_texture", "broken_input", "broken_restart", "late_error", "whole_sheet", "wrong_direction", "bad_animation", "shifted_assembly", "missing_preload", "missing_player", "missing_texture")
	if os.Getenv("GAMEMAKER_BROWSER_EXPORTS_ONLY") == "1" {
		names = positive
	}
	service := newTestService(t)
	service.opts.MaxProjects = len(names)
	templateFor := func(name string) string {
		if name == "planned_blocks" || name == "multiball" {
			return "blocks"
		}
		if slices.Contains(templateNames()[:6], name) {
			return name
		}
		return "minimal"
	}
	for _, name := range names {
		project := createTestProject(t, service, "2d")
		dir := filepath.Join(service.opts.WorkspacePath, project.ProjectKey)
		if err := WriteScaffold(dir, project); err != nil {
			t.Fatal(err)
		}
		plan := ExampleGamePlan(project)
		plan.Template = templateFor(name)
		if name == "planned_blocks" {
			for _, file := range []string{"sheet.png", "sheet.json"} {
				data, err := bundledAssetPackFile("blocks-and-balls", file)
				if err != nil {
					t.Fatal(err)
				}
				path := filepath.Join(dir, "assets", "builtin", "blocks-and-balls", "2", file)
				if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, data, 0o640); err != nil {
					t.Fatal(err)
				}
			}
			plan.Assets = nil
			for _, selection := range [][2]string{{"player", "paddle_01"}, {"ball", "ball_01"}, {"block_red", "colored_block_01"}, {"block_blue", "colored_block_06"}} {
				plan.Assets = append(plan.Assets, PlanAsset{Role: selection[0], PackID: "blocks-and-balls", Version: "2", AssetID: selection[1]})
			}
		}
		if err := installGameTemplate(dir, plan); err != nil {
			t.Fatal(err)
		}
		if name == "overridden_update" {
			path := filepath.Join(dir, "src/main.ts")
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			data = bytes.Replace(data, []byte("extends GameScene {"), []byte("extends GameScene { update() {}"), 1)
			if err := os.WriteFile(path, data, 0o640); err != nil {
				t.Fatal(err)
			}
		}
		if name == "multiball" {
			data, err := bundledSkills.ReadFile("skills/aurago-phaser4-gameplay/SKILL.md")
			if err != nil {
				t.Fatal(err)
			}
			_, example, _ := strings.Cut(strings.ReplaceAll(string(data), "\r\n", "\n"), "## Spawned collision objects")
			_, example, _ = strings.Cut(example, "```typescript\n")
			example, _, _ = strings.Cut(example, "```")
			if !strings.Contains(example, "start(Multiball)") {
				t.Fatal("documented multiball example missing")
			}
			if err := os.WriteFile(filepath.Join(dir, "src/main.ts"), []byte(example), 0o640); err != nil {
				t.Fatal(err)
			}
		}
		if name == "wrapped_bodies" {
			path := filepath.Join(dir, "src/common.ts")
			data, _ := os.ReadFile(path)
			data = bytes.Replace(data, []byte("this.setup();"), []byte("this.setup(); this.physics.add.collider([{body:this.player,art:null}],this.player);"), 1)
			if err := os.WriteFile(path, data, 0o640); err != nil {
				t.Fatal(err)
			}
		}
		if name == "broken_input" || name == "broken_restart" || name == "late_error" {
			path := filepath.Join(dir, "src", "common.ts")
			data, _ := os.ReadFile(path)
			source := string(data)
			switch name {
			case "broken_input":
				source = strings.ReplaceAll(source, "v.x * 240, v.y * 240", "0, 0")
			case "broken_restart":
				source = strings.ReplaceAll(source, "this.scene.restart()", "this.state.ended = 0")
			case "late_error":
				source = strings.Replace(source, "this.setup();", "this.setup(); this.time.delayedCall(5000,()=>{throw Error('seeded delayed spawn failure');});", 1)
			}
			if err := os.WriteFile(path, []byte(source), 0o640); err != nil {
				t.Fatal(err)
			}
		}
		if slices.Contains([]string{"metadata_texture", "sprite_example", "all_packs", "asset_detail", "assembly_detail", "whole_sheet", "wrong_direction", "bad_animation", "shifted_assembly", "missing_preload", "missing_player", "missing_texture"}, name) {
			prepareAssetBrowserFixture(t, dir, name)
		}
		if result := buildDirectory(context.Background(), dir, 100, 30<<20); !result.OK {
			t.Fatal(result.Diagnostics)
		}
		// Exercise the real ZIP writer; revision admission is tested separately.
		if _, err := service.db.Exec(`UPDATE gm_projects SET current_revision=1 WHERE id=?`, project.ID); err != nil {
			t.Fatal(err)
		}
		var output bytes.Buffer
		if _, err := service.WriteExport(context.Background(), project.ID, &output); err != nil {
			t.Fatal(err)
		}
		archive, err := zip.NewReader(bytes.NewReader(output.Bytes()), int64(output.Len()))
		if err != nil {
			t.Fatal(err)
		}
		for _, file := range archive.File {
			if strings.HasPrefix(file.Name, ".") || strings.Contains(file.Name, "preview-tests") {
				t.Fatalf("internal file exported: %s", file.Name)
			}
			path, _, err := secureJoin(filepath.Join(root, name), file.Name, true)
			if err != nil {
				t.Fatal(err)
			}
			reader, err := file.Open()
			if err != nil {
				t.Fatal(err)
			}
			data, err := io.ReadAll(reader)
			reader.Close()
			if err != nil {
				t.Fatal(err)
			}
			if file.Name == "index.html" && bytes.Contains(data, []byte("run-tests")) {
				t.Fatal("test driver leaked into export")
			}
			if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, data, 0o640); err != nil {
				t.Fatal(err)
			}
		}
	}
	// Use the production policy verbatim, including its external-network block.
	handler, err := os.ReadFile("../server/game_maker_handlers.go")
	if err != nil {
		t.Fatal(err)
	}
	_, declaration, ok := strings.Cut(string(handler), "const gameMakerPreviewCSP = ")
	if !ok {
		t.Fatal("preview CSP declaration moved; update the browser fixture")
	}
	declaration, _, _ = strings.Cut(declaration, "\n")
	csp, err := strconv.Unquote(strings.TrimSpace(declaration))
	if err != nil {
		t.Fatal(err)
	}
	type result struct {
		Name         string            `json:"name"`
		Observations []GameObservation `json:"observations"`
		Errors       []string          `json:"errors"`
	}
	done := make(chan result, len(names))
	mux := http.NewServeMux()
	mux.HandleFunc("/report", func(w http.ResponseWriter, r *http.Request) {
		var report result
		r.Body = http.MaxBytesReader(w, r.Body, 200000)
		if json.NewDecoder(r.Body).Decode(&report) != nil {
			http.Error(w, "invalid", 400)
			return
		}
		if !slices.Contains(names, report.Name) {
			http.Error(w, "unknown", 400)
			return
		}
		checks := compareGameObservations(gameScenarios(&GamePlan{Template: templateFor(report.Name)}), report.Observations)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(checks)
		done <- report
	})
	mux.HandleFunc("/scenarios", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(gameScenarios(&GamePlan{Template: templateFor(r.URL.Query().Get("name"))}))
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Cache-Control", "no-store")
		if r.URL.Path != "/" {
			w.Header().Set("Content-Security-Policy", csp)
			if strings.HasSuffix(r.URL.Path, "/index.html") || strings.HasSuffix(r.URL.Path, "/") {
				rel := strings.TrimPrefix(r.URL.Path, "/")
				if strings.HasSuffix(rel, "/") {
					rel += "index.html"
				}
				path, _, err := secureJoin(root, rel, false)
				if err != nil {
					http.NotFound(w, r)
					return
				}
				data, err := os.ReadFile(path)
				if err != nil {
					http.NotFound(w, r)
					return
				}
				data = injectPreviewBoot(data)
				driver, _ := runtimeFS.ReadFile("runtime/preview-tests.js")
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				_, _ = w.Write(append(data, []byte("<script>"+string(driver)+"</script>")...))
				return
			}
			http.FileServer(http.Dir(root)).ServeHTTP(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		encodedNames, _ := json.Marshal(names)
		fmt.Fprint(w, `<!doctype html><html><body style="background:#17202b;color:white;font:16px system-ui"><h1>Game Maker browser acceptance</h1><button id="run">Run browser checks</button><pre id="results"></pre><div id="preview"></div><script>
const names=`, string(encodedNames), `;let active='',frame,channel,scenarios,errors=[];
async function next(){if(!names.length){document.querySelector('#results').textContent+='COMPLETE\n';return;}active=names.shift();errors=[];channel=crypto.randomUUID();scenarios=await fetch('/scenarios?name='+active).then(r=>r.json());frame=document.createElement('iframe');frame.width=960;frame.height=540;frame.sandbox='allow-scripts';document.querySelector('#preview').replaceChildren(frame);frame.src='/'+active+'/index.html#gm-channel='+channel;}
document.querySelector('#run').onclick=()=>{document.querySelector('#run').disabled=true;next();};
addEventListener('message',async e=>{const d=e.data;if(e.source!==frame?.contentWindow||d?.channel!==channel||d.source!=='aurago-game')return;if(d.type==='ready'&&d.boot)frame.contentWindow.postMessage({source:'aurago-studio',channel,type:'run-tests',scenarios},'*');if(['runtime_error','resource_error','diagnostic'].includes(d.type)){errors.push(d.message);document.querySelector('#results').textContent+=active+' ERROR '+d.message+'\n';}if(d.type==='gameplay'){const checks=await fetch('/report',{method:'POST',body:JSON.stringify({name:active,observations:d.observations,errors})}).then(r=>r.json());document.querySelector('#results').textContent+=active+' '+JSON.stringify(checks)+'\n';setTimeout(next,300);}});
</script></body></html>`)
	})
	listener, err := net.Listen("tcp", "127.0.0.1:8977")
	if err != nil {
		t.Fatal(err)
	}
	server := &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	defer server.Close()
	go server.Serve(listener)
	t.Log("Browser acceptance: http://127.0.0.1:8977")
	for range names {
		select {
		case report := <-done:
			negative := !slices.Contains(positive, report.Name)
			failed := len(report.Errors) > 0
			if report.Name == "planned_blocks" {
				if len(report.Observations) == 0 || report.Observations[0].Before["assets_used"] < 26 {
					t.Error("planned blocks did not use the selected library sprites")
				}
			}
			if report.Name == "metadata_texture" && !strings.Contains(strings.Join(report.Errors, " "), "pack JSON is not a Phaser texture key") {
				t.Error("metadata misuse did not produce the concrete correction")
			}
			if report.Name == "wrapped_bodies" && !strings.Contains(strings.Join(report.Errors, " "), "received a wrapper whose body is a GameObject") {
				t.Error("collider wrapper misuse did not produce the concrete correction")
			}
			if report.Name == "overridden_update" && !strings.Contains(strings.Join(report.Errors, " "), "GameScene.update must be inherited") {
				t.Error("lifecycle override did not produce the concrete correction")
			}
			for _, check := range compareGameObservations(gameScenarios(&GamePlan{Template: templateFor(report.Name)}), report.Observations) {
				if check.Status != "passed" {
					failed = true
					if !negative {
						t.Errorf("%s: %+v", report.Name, check)
					}
				}
			}
			if !negative && len(report.Errors) > 0 {
				t.Errorf("%s runtime errors: %v", report.Name, report.Errors)
			}
			if negative && !failed {
				t.Errorf("seeded failure passed: %s", report.Name)
			}
			t.Log("observed", report.Name)
		case <-time.After(10 * time.Minute):
			t.Fatal("browser acceptance timed out")
		}
	}
}

func prepareAssetBrowserFixture(t *testing.T, dir, name string) {
	t.Helper()
	catalogData, _ := assetPackFS.ReadFile("asset_packs/catalog.json")
	var packs []AssetPackSummary
	if err := json.Unmarshal(catalogData, &packs); err != nil {
		t.Fatal(err)
	}
	var imports, metas []string
	for i, p := range packs {
		for _, file := range []string{"sheet.png", "sheet.json"} {
			data, err := bundledAssetPackFile(p.ID, file)
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(dir, "assets", "builtin", p.ID, p.Version, file)
			if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, data, 0o640); err != nil {
				t.Fatal(err)
			}
		}
		imports = append(imports, fmt.Sprintf("import m%d from '../assets/builtin/%s/%s/sheet.json';", i, p.ID, p.Version))
		metas = append(metas, fmt.Sprintf("m%d", i))
	}
	source := strings.Join(imports, "\n") + "\nimport {GameScene,start} from './common';\nimport {preloadPack,registerAnimations,createAsset,createAssembly,playAction,setFacing} from '../vendor/aurago-game-1.js';\nconst packs=[" + strings.Join(metas, ",") + "];\n" + `
class Assets extends GameScene {
  preload(){for(const m of packs)preloadPack(this,m,'assets/builtin/'+m.id+'/'+m.version+'/sheet.png');}
  setup(){
    super.setup();const coin=this.body(360,270,20,20,0xfacc15,true);
    this.physics.add.overlap(this.player,coin,()=>{coin.destroy();this.state.hits++;this.state.score++;});
    let index=0;const animated:any[]=[];
    for(const m of packs){registerAnimations(this,m);registerAnimations(this,m);
      for(const a of m.assets){if(a.assembly_part)continue;const o=createAsset(this,m,a.id,30+(index%22)*42,90+Math.floor(index/22)*34).setScale(.5);index++;
        const anim=m.animations.find(anim=>anim.asset_id===a.id);if(anim){o.play(m.id+'@'+m.version+':'+anim.id);animated.push(o);}
      }
      for(const a of m.assemblies||[]){const o=createAssembly(this,m,a.id,80+(index%10)*85,420).setScale(.3);index++;if(a.transform.mode==='rotate')setFacing(o,1,0);}
    }
    const seen=new Map(animated.map(o=>[o,new Set([o.anims.currentFrame?.index])]));
    for(const o of animated)o.on('animationupdate',(_animation:any,frame:any)=>seen.get(o).add(frame.index));
    this.time.delayedCall(950,()=>{for(const o of animated){if(o.anims.currentAnim.frames.length>1&&seen.get(o).size<2)throw Error('Animation did not advance: '+o.anims.currentAnim.key);}});
    // Seed mutations are injected here by the fixture, not by model commands.
    SEED
  }
  action(){super.action();this.player.setFillStyle(this.state.actions%2?0xa78bfa:0x5eead4);}
}
start(Assets);`
	seed := ""
	switch name {
	case "asset_detail", "assembly_detail":
		s := &Service{}
		packID, assetID, assemblyID := "blocks-and-balls", "paddle_01", ""
		if name == "assembly_detail" {
			packID, assetID, assemblyID = "vehicles-planes", "", "tank"
		}
		detail, err := s.describeAsset(packID, assetID, assemblyID)
		if err != nil {
			t.Fatal(err)
		}
		source = detail.Example
	case "missing_preload":
		source = strings.Replace(source, "preload(){for(const m of packs)preloadPack(this,m,'assets/builtin/'+m.id+'/'+m.version+'/sheet.png');}", "preload(){}", 1)
		source = strings.ReplaceAll(source, "registerAnimations(this,m);", "")
	case "missing_player":
		seed = "this.player=undefined;"
	case "missing_texture":
		seed = "this.add.sprite(480,270,'nonexistent-texture');"
	case "metadata_texture":
		seed = "this.add.sprite(480,270,packs.find(m=>m.id==='blocks-and-balls'),'paddle_01');"
	case "sprite_example":
		body, err := bundledSkills.ReadFile("skills/aurago-phaser4-gameplay/SKILL.md")
		if err != nil {
			t.Fatal(err)
		}
		parts := strings.SplitN(strings.ReplaceAll(string(body), "\r\n", "\n"), "```typescript\n", 2)
		if len(parts) != 2 {
			t.Fatal("missing Phaser example")
		}
		source = strings.Split(parts[1], "```")[0]
	case "whole_sheet":
		source = strings.Replace(source, "preloadPack(this,m,'assets/builtin/'+m.id+'/'+m.version+'/sheet.png')", "this.load.image(m.id+'@'+m.version,'assets/builtin/'+m.id+'/'+m.version+'/sheet.png')", 1)
	case "wrong_direction":
		seed = "const m=packs.find(m=>m.id==='human-characters-animated');createAsset(this,m,'ranger_idle',100,100).setRotation(Math.PI/2);"
	case "bad_animation":
		seed = "const m=packs.find(m=>m.id==='human-characters-animated');playAction(createAsset(this,m,'ranger_idle',100,100),'invented');"
	case "shifted_assembly":
		seed = "const m=packs.find(m=>m.id==='vehicles-planes');const o=createAssembly(this,m,'tank',100,100);o.list[0].x+=20;"
	}
	source = strings.Replace(source, "SEED", seed, 1)
	if err := os.WriteFile(filepath.Join(dir, "src", "main.ts"), []byte(source), 0o640); err != nil {
		t.Fatal(err)
	}
}
