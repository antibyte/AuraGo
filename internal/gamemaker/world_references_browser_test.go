package gamemaker

import (
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
	"github.com/go-rod/rod/lib/input"
	"github.com/go-rod/rod/lib/launcher"
)

func TestWorldMaritimeReferences(t *testing.T) {
	if os.Getenv("GAMEMAKER_WORLD_BROWSER") != "1" {
		t.Skip("set GAMEMAKER_WORLD_BROWSER=1")
	}
	enableWorldReviewPacks(t)
	for _, dimension := range []string{"2d", "3d"} {
		for _, mode := range []string{"naval", "diving"} {
			t.Run(dimension+"-"+mode, func(t *testing.T) {
				s := newTestService(t)
				p := createTestProject(t, s, dimension)
				dir := filepath.Join(s.opts.WorkspacePath, p.ProjectKey)
				if err := WriteScaffold(dir, p); err != nil {
					t.Fatal(err)
				}
				write := func(name string, b []byte) {
					t.Helper()
					path := filepath.Join(dir, filepath.FromSlash(name))
					if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(path, b, 0644); err != nil {
						t.Fatal(err)
					}
				}
				for source, target := range map[string]string{"reference.html": "index.html", "reference-input.js": "src/reference-input.js", "reference-" + dimension + ".js": "src/main.ts"} {
					b, err := os.ReadFile("../../assets/game-maker-worlds/" + source)
					if err != nil {
						t.Fatal(err)
					}
					if source == "reference.html" && dimension == "3d" {
						b = []byte(strings.ReplaceAll(string(b), `<script src="vendor/phaser-4.2.1.min.js"></script>`, ""))
					}
					write(target, b)
				}
				config, _ := json.Marshal(map[string]string{"mode": mode})
				write("src/reference.json", config)
				ids := []string{"ships-sloop", "ships-dinghy", "ships-frigate"}
				pack := "aurago-pirates-topdown"
				if mode == "diving" {
					ids = []string{"people-diver-brass", "equipment-treasure-chest", "animals-reef-shark"}
					pack = "aurago-pirates-side"
				}
				if dimension == "3d" {
					pack = "aurago-pirates-3d"
				}
				base := "assets/builtin/" + pack + "/1.0.0/"
				for _, id := range ids {
					if dimension == "3d" {
						selected, m, err := validateModelSelection([]string{id}, pack)
						if err != nil {
							t.Fatal(err)
						}
						m.Assets = selected
						b, _ := json.Marshal(m)
						write(base+"assets/"+id+".json", b)
						for _, f := range modelFiles(selected[0]) {
							b, err := bundledModelFile(f.File, pack)
							if err != nil {
								t.Fatal(err)
							}
							write(base+f.File, b)
						}
					} else {
						m, err := readAtlasManifest(pack)
						if err != nil {
							t.Fatal(err)
						}
						m, err = selectAtlasAssets(m, []string{id})
						if err != nil {
							t.Fatal(err)
						}
						b, _ := json.Marshal(m)
						write(base+"assets/"+id+".json", b)
						for _, f := range m.Atlases {
							b, err := bundledAtlasFile(pack, f.File)
							if err != nil {
								t.Fatal(err)
							}
							write(base+f.File, b)
						}
					}
				}
				if built := buildDirectory(context.Background(), dir, 1000, 100<<20); !built.OK {
					t.Fatalf("build: %+v", built)
				}
				publishExportFixture(t, s, p, dir)
				files := readExportFixture(t, s, p)
				extracted := t.TempDir()
				for name, b := range files {
					path, _, err := secureJoin(extracted, name, true)
					if err != nil {
						t.Fatal(err)
					}
					os.MkdirAll(filepath.Dir(path), 0750)
					if err := os.WriteFile(path, b, 0644); err != nil {
						t.Fatal(err)
					}
				}
				if reports := os.Getenv("GAMEMAKER_WORLD_REPORTS"); reports != "" {
					for name, b := range files {
						path := filepath.Join(reports, dimension+"-"+mode, filepath.FromSlash(name))
						os.MkdirAll(filepath.Dir(path), 0750)
						if err := os.WriteFile(path, b, 0644); err != nil {
							t.Fatal(err)
						}
					}
				}
				server := httptest.NewServer(http.StripPrefix("/games/pirates/", http.FileServer(http.Dir(extracted))))
				defer server.Close()
				ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
				defer cancel()
				bin := os.Getenv("CHROME_BIN")
				if bin == "" {
					bin = "C:/Program Files/Google/Chrome/Application/chrome.exe"
				}
				launch := launcher.New().Context(ctx).Bin(bin).Headless(true).NoSandbox(true).Set("enable-unsafe-swiftshader")
				browser := rod.New().Context(ctx).ControlURL(launch.MustLaunch()).MustConnect()
				defer launch.Cleanup()
				defer browser.Close()
				page := browser.MustPage(server.URL + "/games/pirates/").MustWaitLoad()
				page.MustSetViewport(1366, 768, 1, false)
				page.MustWait(`()=>Boolean(globalThis.referenceState?.ready||globalThis.referenceState?.errors.length)`)
				if errors := page.MustEval(`()=>referenceState.errors`).JSON("", ""); errors != "[]" {
					t.Fatal(errors)
				}
				key := input.Space
				if mode == "diving" {
					key = input.ArrowRight
				}
				if err := page.Keyboard.Press(key); err != nil {
					t.Fatal(err)
				}
				page.MustWait(`()=>referenceState.outcome!=='running'`)
				page.Keyboard.Release(key)
				if !page.MustEval(`()=>referenceState.outcome==='won'&&referenceState.contacts>=3&&referenceState.score===3`).Bool() {
					t.Fatal(page.MustEval(`()=>referenceState`).JSON("", "  "))
				}
				if reports := os.Getenv("GAMEMAKER_WORLD_REPORTS"); reports != "" {
					page.MustScreenshot(filepath.Join(reports, dimension+"-"+mode+".png"))
					observed := page.MustEval(`()=>referenceState`).JSON("", "  ")
					os.WriteFile(filepath.Join(reports, dimension+"-"+mode+".json"), []byte(observed), 0644)
				}
				for i := 0; i < 3; i++ {
					page.Keyboard.Press(input.KeyR)
					page.Keyboard.Release(input.KeyR)
					page.MustWait(`()=>referenceState.score===0&&referenceState.outcome==='running'`)
				}
				page.MustEval(`()=>disposeReference()`)
				page.MustWait(`()=>referenceState.disposed===true`)
				if dimension == "3d" && !page.MustEval(`()=>referenceState.cache===0`).Bool() {
					t.Fatal("model cache was not released")
				}
			})
		}
	}
}
