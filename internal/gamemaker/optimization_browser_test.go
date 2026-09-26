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

// Two authored arrangements exercise real ray contacts and enemy damage with
// normal input. The observer reads state; it cannot change targets or counters.
func TestFPSCoverBrowser(t *testing.T) {
	if os.Getenv("GAMEMAKER_OPTIMIZATION_BROWSER") != "1" {
		t.Skip("set GAMEMAKER_OPTIMIZATION_BROWSER=1")
	}
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
			data, _ := json.Marshal(d)
			return s.SetDesignJSON(ctx, run.Job.ID, data)
		}
		if run.Stage != "building" {
			return errors.New("unexpected repair")
		}
		source := `import {startGame} from './common';
const cover = new URLSearchParams(location.search).has('cover');
startGame({mode:'fps',objective:'Cover test',goal:2,speed:5,duration:0,
 combat:{enemyRange:18,enemyDamage:7,enemyCooldown:1.5},
 objects:[{role:'enemy',at:[0,0,8]},...(cover?[{role:'tree',at:[0,0,4],scale:3}]:[])]});`
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
		return errors.New("browser fixture completed")
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
	for _, covered := range []bool{true, false} {
		name := "exposed"
		url := server.URL
		if covered {
			name = "covered"
			url += "?cover=1"
		}
		t.Run(name, func(t *testing.T) {
			page := browser.MustPage("about:blank").Timeout(35 * time.Second)
			defer page.Close()
			page.MustSetViewport(1280, 720, 1, false)
			page.MustNavigate(url).MustWaitLoad()
			page.MustElement("[data-player-start]").MustClick()
			page.MustWait(`()=>window.__AURAGO_GAME_TEST__?.snapshot().elapsed_ms > 6500`)
			state := page.MustEval(`()=>window.__AURAGO_GAME_TEST__.snapshot()`)
			if covered && state.Get("health").Int() != 100 {
				t.Fatalf("cover did not block enemy fire: %s", state.JSON("", ""))
			}
			if !covered && (state.Get("health").Int() >= 100 || state.Get("health").Int() < 86) {
				t.Fatalf("enemy fire/cooldown incorrect: %s", state.JSON("", ""))
			}
			page.MustActivate()
			page.Keyboard.MustType(input.Space)
			page.MustWait(`()=>window.__AURAGO_GAME_TEST__.snapshot().actions >= 1`)
			state = page.MustEval(`()=>window.__AURAGO_GAME_TEST__.snapshot()`)
			if covered && state.Get("hits").Int() != 0 {
				t.Fatal("player shot crossed cover")
			}
			if !covered && state.Get("hits").Int() != 1 {
				t.Fatalf("clear player shot missed enemy: %s", state.JSON("", ""))
			}
		})
	}
}
