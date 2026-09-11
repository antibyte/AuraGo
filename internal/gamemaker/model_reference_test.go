package gamemaker

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
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

var modelReferenceSelections = map[string][]string{
	"transport":   {"road-pickup", "architecture-warehouse", "landscape-road-straight", "props-crate-wood", "props-checkpoint", "humans-mechanic-a"},
	"flight":      {"aircraft-prop-plane", "props-checkpoint", "landscape-island", "architecture-hangar"},
	"space":       {"space-scout", "space-planet-earth", "space-asteroid-split", "space-asteroid-round"},
	"exploration": {"humans-explorer-a", "architecture-house", "vegetation-oak", "vegetation-pine", "props-crystal", "animals-wolf"},
	"fps":         {"fps-medkit", "architecture-floor", "architecture-wall", "humans-trooper-b", "fps-arms-modern", "fps-rifle"},
	"performance": {"vegetation-pine", "props-barrel-metal", "animals-dog", "humans-civilian-a", "road-sedan"},
}

func prepareModelReference(t *testing.T, mode, destination string) {
	t.Helper()
	source, err := os.ReadFile("../../assets/game-maker-low-poly/reference-game.ts")
	if err != nil {
		t.Fatal(err)
	}
	s := newTestService(t)
	project := createTestProject(t, s, "3d")
	ids := modelReferenceSelections[mode]
	s.SetRunner(planningRunner(func(ctx context.Context, run JobRun) error {
		if run.Stage == "visual" {
			return nil
		}
		if run.Stage == "planning" {
			plan := ExampleGamePlan(run.Project)
			plan.Assets = nil
			for _, id := range ids {
				plan.Assets = append(plan.Assets, PlanAsset{Role: id, PackID: ModelPackID, Version: "1.0.0", AssetID: id, Scale: 1, Collider: "catalog"})
			}
			return s.SetPlan(ctx, run.Job.ID, plan)
		}
		pack, err := s.ImportAssetPack(ctx, run.Job.ID, ModelPackID, ids...)
		if err != nil {
			return err
		}
		var imports, names []string
		for i, id := range ids {
			name := fmt.Sprintf("model%d", i)
			names = append(names, name)
			imports = append(imports, fmt.Sprintf("import %s from '../%s';", name, pack.Manifests[id]))
		}
		imports = append(imports, "const MODEL_DATA=["+strings.Join(names, ",")+"];")
		code := strings.Replace(string(source), "// MODEL_IMPORTS", strings.Join(imports, "\n"), 1)
		code = strings.Replace(code, "const MODE = 'transport';", "const MODE = '"+mode+"';", 1)
		return s.WriteJobFile(ctx, run.Job.ID, "src/main.ts", diagnosticsPrelude+code)
	}))
	job, err := s.StartJob(context.Background(), project.ID, StartJobRequest{ModelAssetIDs: ids})
	if err != nil {
		t.Fatal(err)
	}
	if done := waitJob(t, s, job.ID); done.Status != "ready" {
		t.Fatalf("%s job: %+v", mode, done)
	}
	var output bytes.Buffer
	if _, err := s.WriteExport(context.Background(), project.ID, &output); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(destination, 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(destination+".zip", output.Bytes(), 0640); err != nil {
		t.Fatal(err)
	}
	archive, err := zip.NewReader(bytes.NewReader(output.Bytes()), int64(output.Len()))
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range archive.File {
		path, _, err := secureJoin(destination, file.Name, true)
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
		if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data, 0640); err != nil {
			t.Fatal(err)
		}
	}
}

func TestModelReferenceExports(t *testing.T) {
	if os.Getenv("GAMEMAKER_MODEL_BROWSER") != "1" {
		t.Skip("set GAMEMAKER_MODEL_BROWSER=1")
	}
	reports, err := filepath.Abs("../../reports/low-poly/references")
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"transport", "flight", "space", "exploration", "fps", "performance"} {
		prepareModelReference(t, mode, filepath.Join(reports, mode))
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		if r.URL.Path == "/sandbox" {
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprint(w, `<style>html,body,iframe{margin:0;width:100%;height:100%;border:0}</style><iframe sandbox="allow-scripts" src="/transport/index.html"></iframe>`)
			return
		}
		http.FileServer(http.Dir(reports)).ServeHTTP(w, r)
	}))
	defer server.Close()
	bin := "C:/Program Files/Google/Chrome/Application/chrome.exe"
	if value := os.Getenv("CHROME_BIN"); value != "" {
		bin = value
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	launch := launcher.New().Context(ctx).Bin(bin).Headless(true).NoSandbox(true).Set("enable-unsafe-swiftshader").
		Set("disable-frame-rate-limit").Set("disable-gpu-vsync")
	url := launch.MustLaunch()
	defer launch.Cleanup()
	browser := rod.New().Context(ctx).ControlURL(url).MustConnect()
	defer browser.Close()
	page := browser.MustPage("about:blank")
	page.MustSetViewport(1920, 1080, 1, false)
	hold := func(key input.Key, ms int) {
		if err := page.Keyboard.Press(key); err != nil {
			t.Fatal(err)
		}
		page.MustEval(`ms=>new Promise(r=>setTimeout(r,ms))`, ms)
		if err := page.Keyboard.Release(key); err != nil {
			t.Fatal(err)
		}
	}
	for _, mode := range []string{"transport", "flight", "space", "exploration", "fps", "performance"} {
		page.MustNavigate(server.URL + "/" + mode + "/index.html").MustWaitLoad()
		page.MustWait(`()=>window.__referenceReady===true||!!window.__referenceError`)
		if failure := page.MustEval(`()=>window.__referenceError||''`).Str(); failure != "" {
			t.Fatalf("%s: %s", mode, failure)
		}
		before := page.MustEval(`()=>__reference.snapshot()`)
		if mode == "performance" {
			page.MustEval(`()=>new Promise(r=>setTimeout(r,10000))`)
			stats := page.MustEval(`()=>__reference.performance()`)
			t.Logf("performance: %s", stats.String())
			if stats.Get("samples").Int() < 100 {
				t.Fatal("insufficient rendered performance samples")
			}
			os.WriteFile(filepath.Join(reports, "performance.json"), []byte(stats.JSON("", "  ")), 0640)
		} else {
			if mode == "exploration" {
				hold(input.KeyA, 900)
			}
			if mode == "space" || mode == "fps" {
				hold(input.Space, 2300)
			} else {
				hold(input.KeyW, modeDuration(mode))
			}
			after := page.MustEval(`()=>__reference.snapshot()`)
			t.Logf("%s: %s", mode, after.String())
			if after.Get("score").Int() < 1 && !after.Get("carrying").Bool() {
				t.Fatalf("%s: no gameplay interaction: %s -> %s", mode, before.String(), after.String())
			}
			for i := 0; i < 2; i++ {
				hold(input.KeyR, 30)
				reset := page.MustEval(`()=>__reference.snapshot()`)
				if reset.Get("score").Int() != 0 || reset.Get("objects").Int() != before.Get("objects").Int() {
					t.Fatalf("%s restart leaked state", mode)
				}
			}
			hold(input.KeyP, 30)
			frozen := page.MustEval(`()=>__reference.snapshot().time`).Num()
			page.MustEval(`()=>new Promise(r=>setTimeout(r,120))`)
			if page.MustEval(`()=>__reference.snapshot().time`).Num() != frozen {
				t.Fatal("pause still advances simulation")
			}
		}
		page.MustScreenshot(filepath.Join(reports, mode+".png"))
		page.MustEval(`()=>__reference.dispose()`)
		if page.MustEval(`()=>__reference.snapshot().cache`).Int() != 0 {
			t.Fatalf("%s asset leak", mode)
		}
	}
	// The actual Studio iframe has an opaque origin. GLBs must still load locally.
	page.MustNavigate(server.URL + "/sandbox").MustWaitLoad()
	frame := page.MustElement("iframe").MustFrame()
	frame.MustWaitLoad()
	if err := frame.Timeout(15 * time.Second).Wait(rod.Eval(`()=>window.__referenceReady===true||!!window.__referenceError`)); err != nil {
		t.Fatalf("sandbox startup: %v, %s", err, frame.MustEval(`()=>document.body.innerText`).Str())
	}
	if failure := frame.MustEval(`()=>window.__referenceError||''`).Str(); failure != "" {
		t.Fatal("sandbox: " + failure)
	}
	frame.MustEval(`()=>__reference.dispose()`)
}

func modeDuration(mode string) int {
	if mode == "transport" {
		return 2900
	}
	if mode == "flight" {
		return 1100
	}
	return 1900
}
