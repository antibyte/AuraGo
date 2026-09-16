package gamemaker

import (
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

func TestWorldPerformanceBrowser(t *testing.T) {
	if os.Getenv("GAMEMAKER_WORLD_BROWSER") != "1" {
		t.Skip("set GAMEMAKER_WORLD_BROWSER=1")
	}
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.Dir("../../assets/game-maker-worlds")))
	mux.Handle("/packs/", http.StripPrefix("/packs/", http.FileServer(http.Dir("asset_packs"))))
	mux.Handle("/runtime/", http.StripPrefix("/runtime/", http.FileServer(http.Dir("runtime"))))
	server := httptest.NewServer(mux)
	defer server.Close()
	bin := os.Getenv("CHROME_BIN")
	if bin == "" {
		bin = "C:/Program Files/Google/Chrome/Application/chrome.exe"
	}
	visible := os.Getenv("GAMEMAKER_WORLD_VISIBLE") == "1"
	mode := "headless frame pacing"
	if visible {
		mode = "visible Chrome frame pacing"
	}
	launch := launcher.New().Bin(bin).Headless(!visible).NoSandbox(true).Set("enable-unsafe-swiftshader")
	if os.Getenv("GAMEMAKER_WORLD_UNTHROTTLED") == "1" {
		launch.Set("disable-frame-rate-limit")
		mode = "unthrottled throughput; not display frame pacing"
	}
	browser := rod.New().ControlURL(launch.MustLaunch()).MustConnect()
	defer launch.Cleanup()
	defer browser.Close()
	for _, dimension := range []string{"2d", "3d"} {
		t.Run(dimension, func(t *testing.T) {
			page := browser.MustPage().Timeout(90 * time.Second)
			defer page.Close()
			page.MustSetViewport(1920, 1080, 1, false)
			page.MustNavigate(server.URL + "/benchmark.html?dimension=" + dimension).MustWaitLoad()
			page.MustActivate()
			page.MustWait(`()=>Boolean(globalThis.benchmark?.complete||globalThis.benchmark?.error)`)
			page.MustEval(`mode=>benchmark.mode=mode`, mode)
			if err := page.MustEval(`()=>benchmark.error||''`).Str(); err != "" {
				t.Fatal(err)
			}
			result := page.MustEval(`()=>benchmark`).JSON("", "  ")
			t.Log(result)
			if root := os.Getenv("GAMEMAKER_WORLD_REPORTS"); root != "" {
				os.MkdirAll(root, 0750)
				os.WriteFile(filepath.Join(root, "performance-"+dimension+".json"), []byte(result), 0644)
				page.MustScreenshot(filepath.Join(root, "performance-"+dimension+".png"))
			}
			var state struct {
				Frames int `json:"frames"`
			}
			json.Unmarshal([]byte(result), &state)
			if state.Frames < 1 {
				t.Fatal("no real frames")
			}
			if dimension == "3d" && page.MustEval(`()=>disposeBenchmark()`).Int() != 0 {
				t.Fatal("model cache leaked")
			}
		})
	}
}
