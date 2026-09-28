package gamemaker

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
)

// Declared stages must exist in the running game: distinct 3D level layouts or
// 2D levels whose layout branches on levelIndex. Promised but missing stages fail.
func TestStageEvidenceBrowser(t *testing.T) {
	if os.Getenv("GAMEMAKER_GUIDED_BROWSER") != "1" {
		t.Skip("set GAMEMAKER_GUIDED_BROWSER=1 for real browser acceptance")
	}
	bin := "C:/Program Files/Google/Chrome/Application/chrome.exe"
	if b := os.Getenv("CHROME_BIN"); b != "" {
		bin = b
	}
	launch := launcher.New().Bin(bin).Headless(true).NoSandbox(true).Set("enable-unsafe-swiftshader")
	browser := rod.New().ControlURL(launch.MustLaunch()).MustConnect()
	defer launch.Cleanup()
	defer browser.Close()
	page := browser.MustPage("about:blank")
	defer page.Close()
	page.MustSetViewport(1366, 768, 1, false)
	handlers, err := os.ReadFile("../server/game_maker_handlers.go")
	if err != nil {
		t.Fatal(err)
	}
	_, declaration, _ := strings.Cut(string(handlers), "const gameMakerPreviewCSP = ")
	declaration, _, _ = strings.Cut(declaration, "\n")
	csp, err := strconv.Unquote(strings.TrimSpace(declaration))
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, base string
		levels     string
		publish    bool
		stageCount string
	}{
		{name: "3d_distinct_levels", base: "exploration", levels: "distinct", publish: true, stageCount: `()=>__AURAGO_GAME_TEST__.snapshot().stage_count`},
		{name: "3d_missing_levels", base: "exploration"},
		{name: "3d_cloned_levels", base: "exploration", levels: "cloned"},
		{name: "2d_platformer_levels", base: "platformer", publish: true, stageCount: `()=>__AURAGO_GAME_TEST__.scene.levels.length`},
		{name: "2d_minimal_missing_levels", base: "minimal"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := newTestService(t)
			s.opts.JobTimeout = 2 * time.Minute
			dimension := "2d"
			if is3DTemplate(tc.base) {
				dimension = "3d"
			}
			p := createTestProject(t, s, dimension)
			s.SetRunner(planningRunner(func(ctx context.Context, run JobRun) error {
				switch run.Stage {
				case "planning":
					d := ExampleGameDesign(p)
					d.Base = tc.base
					d.Stages = []string{"First area: learn the controls among patrols", "Second area: new hazards guard the exit"}
					data, _ := json.Marshal(d)
					return s.SetDesignJSON(ctx, run.Job.ID, data)
				case "building":
					source, err := s.ReadJobFile(ctx, run.Job.ID, "src/main.ts")
					if err != nil {
						return err
					}
					old, replacement := "this.state.score++", "this.state.score += 2"
					if dimension == "3d" {
						old, replacement = stageConfigEdit(t, source, tc.levels)
					}
					if !strings.Contains(source, old) {
						return fmt.Errorf("%s starter lacks %q", tc.base, old)
					}
					_, err = s.ReplaceJobFile(ctx, run.Job.ID, "src/main.ts", old, replacement, sourceHash(source))
					return err
				case "repair":
					return fmt.Errorf("browser checks failed: %+v", run.Diagnostics)
				}
				return nil
			}))
			job, err := s.StartJob(context.Background(), p.ID, StartJobRequest{})
			if err != nil {
				t.Fatal(err)
			}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/":
					w.Header().Set("Content-Type", "text/html")
					http.ServeFile(w, r, "testdata/studio-parent.html")
					return
				case "/state":
					job, _ := s.GetJob(r.Context(), job.ID)
					grant, _ := s.CreatePreviewGrant(p.ID)
					json.NewEncoder(w).Encode(map[string]any{"job": job, "grant": grant})
					return
				case "/report":
					var report PreviewReport
					if err := json.NewDecoder(r.Body).Decode(&report); err != nil {
						http.Error(w, err.Error(), 400)
						return
					}
					if err := s.ReportPreview(p.ID, report); err != nil {
						http.Error(w, err.Error(), 400)
					}
					return
				}
				parts := strings.SplitN(strings.TrimPrefix(r.URL.Path, "/api/game-maker/preview/"), "/", 2)
				if len(parts) != 2 {
					http.NotFound(w, r)
					return
				}
				data, kind, err := s.PreviewFile(parts[0], parts[1])
				if err != nil {
					http.Error(w, err.Error(), 404)
					return
				}
				w.Header().Set("Access-Control-Allow-Origin", "*")
				w.Header().Set("Content-Type", kind)
				w.Header().Set("Content-Security-Policy", csp)
				w.Write(data)
			}))
			defer server.Close()
			page.MustNavigate(server.URL).MustWaitLoad()
			if err := page.Timeout(100 * time.Second).Wait(rod.Eval(`()=>window.done`)); err != nil {
				t.Fatalf("browser timeout: %s", page.MustEval(`()=>document.body.innerText`).Str())
			}
			result := page.MustEval(`()=>window.result`)
			published := result.Get("status").Str() == "ready"
			if published != tc.publish {
				t.Fatalf("published=%v, want %v: %s", published, tc.publish, result.String())
			}
			if !tc.publish {
				final, _ := s.GetJob(context.Background(), job.ID)
				if !strings.Contains(final.Error, stagesCheckID) || !strings.Contains(final.Error, "levelIndex") {
					t.Fatalf("missing stages were not reported with repair guidance: %s", final.Error)
				}
				return
			}
			frame := page.MustElement("iframe").MustFrame()
			frame.MustWaitLoad()
			frame.MustWait(`()=>!!window.__AURAGO_GAME_TEST__`)
			if got := frame.MustEval(tc.stageCount).Int(); got < 2 {
				t.Fatalf("stage evidence = %d, want at least 2", got)
			}
		})
	}
}

// stageConfigEdit rewrites the guided 3D starter config through the normal edit
// primitive. Distinct levels add a landmark to the second stage; cloned levels
// only rename the same layout and must not count as another stage; no levels
// removes the starter's own stages.
func stageConfigEdit(t *testing.T, source string, levels string) (string, string) {
	t.Helper()
	match := regexp.MustCompile(`startGame\((\{.*\})\);`).FindStringSubmatch(source)
	if match == nil {
		t.Fatal("guided starter lacks a JSON startGame config")
	}
	var config map[string]any
	if err := json.Unmarshal([]byte(match[1]), &config); err != nil {
		t.Fatal(err)
	}
	config["speed"] = 6
	delete(config, "levels")
	if levels != "" {
		objects, _ := config["objects"].([]any)
		second := append([]any{}, objects...)
		if levels == "distinct" {
			second = append(second, map[string]any{"role": "tree", "at": []float64{6, 0, 6}})
		}
		config["levels"] = []any{
			map[string]any{"id": "first", "title": "First area", "objects": objects},
			map[string]any{"id": "second", "title": "Second area", "objects": second},
		}
	}
	data, err := json.Marshal(config)
	if err != nil {
		t.Fatal(err)
	}
	return match[0], "startGame(" + string(data) + ");"
}
