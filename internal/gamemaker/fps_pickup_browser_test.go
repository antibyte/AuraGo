package gamemaker

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/input"
	"github.com/go-rod/rod/lib/launcher"
)

func TestFPSPickupBrowser(t *testing.T) {
	if os.Getenv("GAMEMAKER_TARGET_BROWSER") != "1" {
		t.Skip("set GAMEMAKER_TARGET_BROWSER=1 for real FPS pickup inputs")
	}
	for _, revision := range []string{"current", "0f5d4b125", "6040144e3"} {
		t.Run(revision, func(t *testing.T) { testFPSPickupBrowser(t, revision) })
	}
}

func testFPSPickupBrowser(t *testing.T, revision string) {
	s := newTestService(t)
	s.opts.JobTimeout = 2 * time.Minute
	project := createTestProject(t, s, "3d")
	ready := make(chan string, 1)
	finish := make(chan struct{})
	defer close(finish)
	s.SetRunner(planningRunner(func(ctx context.Context, run JobRun) error {
		if run.Stage == "planning" {
			d := ExampleGameDesign(project)
			d.Base = "fps"
			d.Assets = append(defaultModelRoles("fps"), DesignAsset{Role: "item", PackID: ModelPackID, AssetID: "props-crystal"})
			data, _ := json.Marshal(d)
			return s.SetDesignJSON(ctx, run.Job.ID, data)
		}
		if run.Stage != "building" {
			return errors.New("unexpected repair")
		}
		if revision != "current" {
			legacy, err := legacyThreePickupFixture(revision, *run.Plan)
			if err != nil {
				return err
			}
			if err := s.writeJobFile(ctx, run.Job.ID, "src/common.ts", legacy); err != nil {
				return err
			}
		}
		source := `import {startGame} from './common';
startGame({mode:'fps',objective:'Collect the item',goal:2,speed:5,duration:0,
 objects:[{role:'item',at:[0,0,6],scale:.2}]});`
		written, err := s.WriteJobFileChecked(ctx, run.Job.ID, "src/main.ts", source, "")
		if err != nil {
			return err
		}
		if !written.Build.OK {
			return errors.New(diagnosticsText(written.Build.Diagnostics))
		}
		stage, _ := s.JobDirectory(run.Job.ID)
		ready <- stage
		select {
		case <-finish:
		case <-ctx.Done():
		}
		return errors.New("pickup fixture completed")
	}))
	job, err := s.StartJob(context.Background(), project.ID, StartJobRequest{})
	if err != nil {
		t.Fatal(err)
	}
	var stage string
	select {
	case stage = <-ready:
	case <-time.After(15 * time.Second):
		state, _ := s.GetJob(context.Background(), job.ID)
		t.Fatalf("fixture not ready: %+v", state)
	}
	server := httptest.NewServer(http.FileServer(http.Dir(stage)))
	defer server.Close()
	bin := os.Getenv("CHROME_BIN")
	if bin == "" {
		bin = "C:/Program Files/Google/Chrome/Application/chrome.exe"
	}
	launch := launcher.New().Bin(bin).Headless(true).NoSandbox(true).Set("enable-unsafe-swiftshader")
	browser := rod.New().ControlURL(launch.MustLaunch()).MustConnect()
	defer launch.Cleanup()
	defer browser.Close()
	for _, mode := range []string{"normal_input", "target_driver"} {
		t.Run(mode, func(t *testing.T) {
			page := browser.MustPage("about:blank").Timeout(25 * time.Second)
			defer page.Close()
			page.MustSetViewport(960, 540, 1, false)
			page.MustNavigate(server.URL + "/#gm-channel=fps-pickup").MustWaitLoad()
			page.MustWait(`()=>window.__AURAGO_GAME_TEST__?.alive()`)
			if mode == "normal_input" {
				page.MustActivate()
				if found, _, _ := page.Has("[data-player-start]"); found {
					page.MustElement("[data-player-start]").MustClick()
				}
				if err := page.Keyboard.Press(input.KeyW); err != nil {
					t.Fatal(err)
				}
				page.MustWait(`()=>window.__AURAGO_GAME_TEST__.snapshot().hits === 1`)
				if err := page.Keyboard.Release(input.KeyW); err != nil {
					t.Fatal(err)
				}
				state := page.MustEval(`()=>window.__AURAGO_GAME_TEST__.snapshot()`)
				if state.Get("pickup_events").Int() != 1 || state.Get("score").Int() != 1 {
					t.Fatalf("actual pickup is not observable: %s", state.JSON("", ""))
				}
				page.Keyboard.MustType(input.KeyR)
				page.MustWait(`()=>window.__AURAGO_GAME_TEST__.snapshot().score === 0`)
			} else {
				driver, err := runtimeFS.ReadFile("runtime/preview-tests.js")
				if err != nil {
					t.Fatal(err)
				}
				if err := page.AddScriptTag("", string(driver)); err != nil {
					t.Fatal(err)
				}
				scenario := GameScenario{ID: "collect_ammo", Metric: "pickup_events", Compare: "increased", Steps: []GameTestStep{{Action: "target", Target: "item", Mode: "reach", MS: 4000}}}
				page.MustEval(`scenario=>{window.__pickupReport=null;addEventListener('message',e=>{if(e.data?.type==='gameplay')window.__pickupReport=e.data});postMessage({source:'aurago-studio',channel:'fps-pickup',type:'run-tests',scenarios:[scenario]},'*')}`, scenario)
				page.MustWait(`()=>!!window.__pickupReport`)
				var report PreviewReport
				if err := json.Unmarshal([]byte(page.MustEval(`()=>JSON.stringify(window.__pickupReport)`).Str()), &report); err != nil {
					t.Fatal(err)
				}
				if err := validateGameReport(report); err != nil {
					t.Fatal(err)
				}
				checks := compareGameObservations([]GameScenario{scenario}, report.Observations)
				if checks[0].Status != "passed" {
					t.Fatalf("driver did not collect the low FPS item: %+v", checks)
				}
			}
			if pickups := page.MustEval(`()=>window.__AURAGO_GAME_TEST__.snapshot().pickup_events`).Int(); pickups != 0 {
				t.Fatalf("restart retained %d pickup events", pickups)
			}
		})
	}
}
