package ui

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type auraDriftSurveyScene struct {
	name string
	// setup runs in the page once the harness is ready and only when
	// AuraDriftApp.debug exists. Empty means the scene is just the menu.
	setup string
}

func auraDriftSurveyScenes() []auraDriftSurveyScene {
	scenes := []auraDriftSurveyScene{}
	// custom: the JS in the file named by AURAGO_AURA_DRIFT_CUSTOM (an async function expression). It may set
	// window.__steps = [async()=>..., ...]; each step runs and is screenshot as custom-<n>.png.
	// __ARG__ in the file is replaced by AURAGO_AURA_DRIFT_ARG.
	if file := os.Getenv("AURAGO_AURA_DRIFT_CUSTOM"); file != "" {
		if data, err := os.ReadFile(file); err == nil {
			scenes = append(scenes, auraDriftSurveyScene{"custom", strings.ReplaceAll(strings.TrimSpace(string(data)), "__ARG__", os.Getenv("AURAGO_AURA_DRIFT_ARG"))})
		}
	}
	biomes := []string{"alpine", "desert", "harbour", "salt", "forest", "industry", "glacier", "metro"}
	for _, biome := range biomes {
		scenes = append(scenes, auraDriftSurveyScene{"biome-" + biome, fmt.Sprintf(
			`async()=>{const tr=AuraDriftCore.TRACKS.find(x=>x.biome===%q)||AuraDriftCore.TRACKS[0];await AuraDriftApp.debug.start({mode:'time_trial',trackId:tr.id});AuraDriftApp.debug.setCamera('chase');}`, biome)})
	}
	// run-<biome>: a quick race driven by the autopilot for 25 simulated seconds, so the shot shows the HUD, rivals and effects mid-race.
	for _, biome := range biomes {
		scenes = append(scenes, auraDriftSurveyScene{"run-" + biome, fmt.Sprintf(
			`async()=>{const tr=AuraDriftCore.TRACKS.find(x=>x.biome===%q)||AuraDriftCore.TRACKS[0];await AuraDriftApp.debug.start({mode:'quick',trackId:tr.id});AuraDriftApp.debug.autopilot(true,{skill:1});AuraDriftApp.debug.fastForward(29,{renderEvery:30});}`, biome)})
	}
	for _, mode := range []string{"chase", "high", "drone", "bumper"} {
		scenes = append(scenes, auraDriftSurveyScene{"camera-" + mode, fmt.Sprintf(
			`async()=>{await AuraDriftApp.debug.start({mode:'time_trial'});AuraDriftApp.debug.setCamera(%q);}`, mode)})
	}
	for _, car := range []string{"vektor", "kobalt", "lumen", "tundra", "softeis", "maehkoenig", "couch", "bertha"} {
		scenes = append(scenes, auraDriftSurveyScene{"car-" + car, fmt.Sprintf(
			`async()=>{AuraDriftApp.debug.showcase({carId:%q});}`, car)})
	}
	for _, weapon := range []string{"oil", "gnome_mine", "emp", "rocket", "mg", "shield", "ducks", "railgun", "flamethrower", "frost", "air_horn", "black_hole", "disco", "glove", "harpoon", "cow", "cloak", "overdrive"} {
		scenes = append(scenes, auraDriftSurveyScene{"weapon-" + weapon, fmt.Sprintf(
			`async()=>{await AuraDriftApp.debug.start({mode:'quick'});AuraDriftApp.debug.give(%q);AuraDriftApp.debug.fire();}`, weapon)})
	}
	scenes = append(scenes,
		auraDriftSurveyScene{"night-high", `async()=>{const tr=AuraDriftCore.TRACKS.find(x=>x.biome==='metro');await AuraDriftApp.debug.start({mode:'quick',trackId:tr.id});AuraDriftApp.debug.setCamera('high');}`},
		auraDriftSurveyScene{"night-bumper", `async()=>{const tr=AuraDriftCore.TRACKS.find(x=>x.biome==='harbour');await AuraDriftApp.debug.start({mode:'quick',trackId:tr.id});AuraDriftApp.debug.setCamera('drone');}`},
		auraDriftSurveyScene{"trick-air", `async()=>{await AuraDriftApp.debug.start({mode:'time_trial'});}`},
		auraDriftSurveyScene{name: "menu"},
		auraDriftSurveyScene{name: "results", setup: `async()=>{AuraDriftApp.debug.results({});}`},
		auraDriftSurveyScene{name: "hud", setup: `async()=>{await AuraDriftApp.debug.start({mode:'quick'});}`},
	)
	return scenes
}

// TestAuraDriftSurveyBrowser renders the dev harness scene by scene and writes
// PNGs for a human to look at. It is a development tool: skipped unless
// AURAGO_AURA_DRIFT_SURVEY=1. AURAGO_AURA_DRIFT_SCENES filters by name
// (comma separated) and AURAGO_BROWSER_ARTIFACT_DIR picks the output folder.
func TestAuraDriftSurveyBrowser(t *testing.T) {
	if os.Getenv("AURAGO_AURA_DRIFT_SURVEY") != "1" {
		t.Skip("set AURAGO_AURA_DRIFT_SURVEY=1 to run the Aura Drift visual survey")
	}
	outDir := os.Getenv("AURAGO_BROWSER_ARTIFACT_DIR")
	if outDir == "" {
		outDir = filepath.Join(os.TempDir(), "agdrift-shots")
	}
	if err := os.MkdirAll(outDir, 0755); err != nil {
		t.Fatal(err)
	}
	wanted := map[string]bool{}
	for _, name := range strings.Split(os.Getenv("AURAGO_AURA_DRIFT_SCENES"), ",") {
		if name = strings.TrimSpace(name); name != "" {
			wanted[name] = true
		}
	}

	harness := readDesktopAssetText(t, "testdata/aura-drift-harness.html")
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(Content)))
	mux.HandleFunc("/harness", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		fmt.Fprint(w, harness)
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	browser := newSmokeBrowser(t)
	for _, scene := range auraDriftSurveyScenes() {
		if len(wanted) > 0 && !wanted[scene.name] {
			continue
		}
		scene := scene
		t.Run(scene.name, func(t *testing.T) {
			page := browser.MustPage("").Timeout(2 * time.Minute)
			t.Cleanup(func() { page.Close() })
			page.MustSetViewport(1280, 720, 1, false)
			url := server.URL + "/harness"
			if quality := os.Getenv("AURAGO_AURA_DRIFT_QUALITY"); quality != "" {
				url += "?quality=" + quality
			}
			page.MustNavigate(url)
			page.MustWaitLoad()
			page.MustEval(`async()=>{await window.AuraDriftHarnessReady;}`)
			page.MustEval(`async()=>{for(let i=0;i<100&&!document.querySelector('.agdrift-root');i++)await new Promise(r=>setTimeout(r,50));}`)
			hasDebug := page.MustEval(`()=>!!(window.AuraDriftApp&&window.AuraDriftApp.debug)`).Bool()
			if hasDebug && scene.setup != "" {
				page.MustEval(scene.setup)
			}
			page.MustEval(`()=>new Promise(r=>setTimeout(r,1500))`)
			stats := page.MustEval(`()=>JSON.stringify(window.AuraDriftApp?AuraDriftApp.stats():null)`).String()
			errs := page.MustEval(`()=>JSON.stringify(window.fixtureErrors||[])`).String()
			t.Logf("scene %s debug=%v stats=%s", scene.name, hasDebug, stats)
			png := filepath.Join(outDir, scene.name+".png")
			page.MustScreenshot(png)
			t.Logf("wrote %s", png)
			if scene.name == "custom" {
				steps := page.MustEval(`()=>(window.__steps||[]).length`).Int()
				for i := 0; i < steps; i++ {
					page.MustEval(fmt.Sprintf(`async()=>{await window.__steps[%d]();await new Promise(r=>setTimeout(r,300));}`, i))
					png := filepath.Join(outDir, fmt.Sprintf("custom-%d.png", i))
					page.MustScreenshot(png)
					t.Logf("wrote %s log=%s", png, page.MustEval(`()=>JSON.stringify(window.__log||null)`).String())
				}
			}
			if errs != "[]" {
				t.Fatalf("uncaught errors in scene %s: %s", scene.name, errs)
			}
		})
	}
}
