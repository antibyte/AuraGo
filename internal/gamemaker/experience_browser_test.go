package gamemaker

import (
	"context"
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

// Normal browser inputs exercise contact, recovery and stage transitions in both
// engines. Observers never change health, position, outcome or test verdicts.
func TestExperienceBrowser(t *testing.T) {
	if os.Getenv("GAMEMAKER_EXPERIENCE_BROWSER") != "1" {
		t.Skip("set GAMEMAKER_EXPERIENCE_BROWSER=1")
	}
	bin := os.Getenv("CHROME_BIN")
	if bin == "" {
		bin = "C:/Program Files/Google/Chrome/Application/chrome.exe"
	}
	launch := launcher.New().Bin(bin).Headless(true).NoSandbox(true).Set("enable-unsafe-swiftshader")
	browser := rod.New().ControlURL(launch.MustLaunch()).MustConnect()
	defer launch.Cleanup()
	defer browser.Close()
	for _, dimension := range []string{"2d", "3d"} {
		t.Run(dimension, func(t *testing.T) {
			service := newTestService(t)
			project := createTestProject(t, service, dimension)
			root := filepath.Join(service.opts.WorkspacePath, project.ProjectKey)
			if err := WriteScaffold(root, project); err != nil {
				t.Fatal(err)
			}
			plan := ExampleGamePlan(project)
			plan.Assets = nil
			plan.Template = "topdown"
			if dimension == "3d" {
				plan.Template = "exploration"
			}
			if err := installGameTemplate(root, plan); err != nil {
				t.Fatal(err)
			}
			source, err := os.ReadFile("testdata/experience-" + dimension + ".ts")
			if err != nil {
				t.Fatal(err)
			}
			if err = os.WriteFile(filepath.Join(root, "src/main.ts"), source, 0600); err != nil {
				t.Fatal(err)
			}
			if build := buildDirectory(context.Background(), root, 150, 64<<20); !build.OK {
				t.Fatal(build.Diagnostics)
			}
			publishExportFixture(t, service, project, root)
			exported := readExportFixture(t, service, project)
			if len(exported["vendor/game-flow.js"]) == 0 {
				t.Fatal("export missing game flow")
			}
			extracted := t.TempDir()
			for path, data := range exported {
				dest, _, err := secureJoin(extracted, path, true)
				if err != nil {
					t.Fatal(err)
				}
				if err = os.MkdirAll(filepath.Dir(dest), 0750); err != nil {
					t.Fatal(err)
				}
				if err = os.WriteFile(dest, data, 0600); err != nil {
					t.Fatal(err)
				}
			}
			server := httptest.NewServer(http.StripPrefix("/games/journey/", http.FileServer(http.Dir(extracted))))
			defer server.Close()
			page := browser.MustPage("about:blank").Timeout(45 * time.Second)
			defer page.Close()
			page.MustSetViewport(1280, 720, 1, false)
			page.MustEvalOnNewDocument(`window.errors=[];addEventListener('error',e=>errors.push(e.message));addEventListener('unhandledrejection',e=>errors.push(String(e.reason)))`)
			page.MustNavigate(server.URL + "/games/journey/").MustWaitLoad()
			page.MustWait(`()=>!!window.fixture&&!!document.querySelector('[data-game-flow]')`)
			if page.MustEval(`()=>fixture.presentation.audio.stats().state`).Str() != "locked" {
				t.Fatal("audio started before interaction")
			}
			page.MustActivate()
			key := input.KeyD
			if dimension == "3d" {
				key = input.KeyW
			}
			audit := `const a=fixture.flow?.audit()||fixture.feedback.audit();`
			if err := page.Keyboard.Press(key); err != nil {
				t.Fatal(err)
			}
			page.MustEval(`()=>new Promise(resolve=>{const poll=()=>{` + audit + `if(a.feedback.death===1&&a.feedback.hit===1)resolve(true);else requestAnimationFrame(poll)};poll()})`)
			if !page.MustEval(`()=>!!document.querySelector('[data-game-feedback="hit"]')`).Bool() {
				t.Fatal("contact lacked visible marker")
			}
			if err := page.Keyboard.Release(key); err != nil {
				t.Fatal(err)
			}
			// Pause must freeze the recovery delay and pending feedback, with no replay.
			page.Keyboard.MustType(input.KeyP)
			page.MustEval(`()=>new Promise(resolve=>setTimeout(resolve,800))`)
			if page.MustEval(`()=>{` + audit + `return !!a.feedback.respawn}`).Bool() {
				t.Fatal("recovery advanced while paused")
			}
			page.Keyboard.MustType(input.KeyP)
			page.MustWait(`()=>{` + audit + `return a.feedback.respawn===1}`)
			if dimension == "2d" {
				if x := page.MustEval(`()=>fixture.player.x`).Int(); x < 99 || x > 101 {
					t.Fatalf("checkpoint x=%d", x)
				}
			} else {
				if z := page.MustEval(`()=>fixture.player.position.z`).Int(); z != 0 {
					t.Fatalf("checkpoint z=%d", z)
				}
			}
			if err := page.Keyboard.Press(key); err != nil {
				t.Fatal(err)
			}
			page.MustWait(`()=>!!document.querySelector('[data-game-result="won"]')`)
			if err := page.Keyboard.Release(key); err != nil {
				t.Fatal(err)
			}
			if dimension == "2d" && page.MustEval(`()=>fixture.cameras.main.scrollX`).Int() < 300 {
				t.Fatal("camera never traversed world")
			}
			if n := page.MustEval(`()=>document.querySelectorAll('[data-game-result] button').length`).Int(); n != 2 {
				t.Fatalf("win buttons=%d", n)
			}
			if page.MustEval(`()=>fixture.presentation.audio.stats().cue_buffers`).Int() < 3 {
				t.Fatal("feedback did not use unlocked mixer")
			}
			reports := filepath.Join("..", "..", "reports", "game-experience")
			os.MkdirAll(reports, 0750)
			page.MustScreenshot(filepath.Join(reports, dimension+"-won.png"))
			page.MustElement("[aria-label='Mute sound']").MustClick()
			page.MustElement("[data-game-result] button").MustClick()
			page.MustWait(`()=>window.fixture.levelIndex===1&&!document.querySelector('[data-game-result]')`)
			if !page.MustEval(`()=>fixture.presentation.audio.preferences.muted&&document.querySelector('[aria-label="Mute sound"]').getAttribute('aria-pressed')==='true'`).Bool() {
				t.Fatal("mute lost on stage change")
			}
			if n := page.MustEval(`()=>document.querySelectorAll('[data-game-flow]').length`).Int(); n != 1 {
				t.Fatalf("flow leak after advance: %d", n)
			}
			if err := page.Keyboard.Press(key); err != nil {
				t.Fatal(err)
			}
			if dimension == "3d" {
				page.MustWait(`()=>fixture.feedback.audit().feedback.respawn===1`)
				if err := page.Keyboard.Release(key); err != nil {
					t.Fatal(err)
				}
				// Let the deliberate post-respawn protection expire before
				// approaching the same hazard again with normal input.
				page.MustEval(`()=>new Promise(resolve=>setTimeout(resolve,1200))`)
				if err := page.Keyboard.Press(key); err != nil {
					t.Fatal(err)
				}
			}
			page.MustWait(`()=>!!document.querySelector('[data-game-result="lost"]')`)
			if err := page.Keyboard.Release(key); err != nil {
				t.Fatal(err)
			}
			page.MustScreenshot(filepath.Join(reports, dimension+"-lost.png"))
			for i := 0; i < 3; i++ {
				page.MustEval(`()=>{window.previousFlow=fixture.flow}`)
				page.Keyboard.MustType(input.KeyR)
				if err := page.Timeout(5 * time.Second).Wait(rod.Eval(`()=>fixture.levelIndex===0&&!fixture.state.ended&&!!document.querySelector('[data-game-flow]')&&!document.querySelector('[data-game-result]')&&(!fixture.flow||fixture.flow!==window.previousFlow)`)); err != nil {
					t.Fatalf("restart %d: %s", i, page.MustEval(`()=>({index:fixture.levelIndex,state:fixture.state,flow:fixture.flow?.audit(),errors})`).String())
				}
			}
			if n := page.MustEval(`()=>document.querySelectorAll('canvas').length`).Int(); n != 1 {
				t.Fatalf("canvas leak: %d", n)
			}
			if n := page.MustEval(`()=>document.querySelectorAll('.aurago-game-presentation').length`).Int(); n != 1 {
				t.Fatalf("mixer controls leak: %d", n)
			}
			page.MustSetViewport(390, 844, 1, false)
			page.Keyboard.MustType(input.Escape)
			page.MustWait(`()=>!!document.querySelector('[data-game-result]')`)
			if page.MustEval(`()=>{const r=document.querySelector('[data-game-result] button').getBoundingClientRect();return r.height<44||r.right>innerWidth||r.bottom>innerHeight}`).Bool() {
				t.Fatal("result controls outside touch viewport")
			}
			page.MustScreenshot(filepath.Join(reports, dimension+"-touch.png"))
			if errors := page.MustEval(`()=>errors`).String(); errors != "[]" {
				t.Fatal(errors)
			}
			if !page.MustEval(`()=>performance.getEntriesByType('resource').every(r=>new URL(r.name).origin===location.origin)&&!window.__AURAGO_PREVIEW_BOOT__`).Bool() {
				t.Fatal("export used external or Studio runtime")
			}
		})
	}
}
