package gamemaker

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
)

// The real service waits for real sandboxed browser observations before publish.
func TestGuidedBrowser(t *testing.T) {
	if os.Getenv("GAMEMAKER_GUIDED_BROWSER") != "1" {
		t.Skip("set GAMEMAKER_GUIDED_BROWSER=1 for real browser acceptance")
	}
	bin := "C:/Program Files/Google/Chrome/Application/chrome.exe"
	if b := os.Getenv("CHROME_BIN"); b != "" {
		bin = b
	}
	launch := launcher.New().Bin(bin).Headless(true).NoSandbox(true).Set("enable-unsafe-swiftshader")
	url := launch.MustLaunch()
	defer launch.Cleanup()
	browser := rod.New().ControlURL(url).MustConnect()
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
	modes := []string{"fps", "exploration", "transport", "flight", "space", "blocks", "shooter", "platformer", "board", "topdown", "minimal"}
	if only := os.Getenv("GAMEMAKER_GUIDED_MODE"); only != "" {
		modes = strings.Split(only, ",")
	}
	for _, mode := range modes {
		t.Run(mode, func(t *testing.T) {
			s := newTestService(t)
			s.opts.JobTimeout = 2 * time.Minute
			dimension := "3d"
			if !is3DTemplate(mode) {
				dimension = "2d"
			}
			p := createTestProject(t, s, dimension)
			s.SetRunner(planningRunner(func(ctx context.Context, run JobRun) error {
				switch run.Stage {
				case "planning":
					d := ExampleGameDesign(p)
					if os.Getenv("GAMEMAKER_PRESENTATION") == "1" {
						d.Presentation = &Presentation{Environment: "forest-rain", Effects: []string{"metal-sparks", "blood-spray", "muzzle-flash", "pickup-glow"}, Sounds: []SoundBinding{{"step", "step-grass"}, {"shot", "rifle"}, {"hit", "impact-metal"}, {"pickup", "pickup"}, {"win", "victory"}, {"lose", "defeat"}}, Quality: "auto"}
					}
					d.Base = mode
					if d.Settings != nil {
						d.Settings.Duration = 15
						if mode == "flight" {
							d.Settings.Duration = 0
						}
					}
					d.Objective = map[string]string{"fps": "Forest patrol", "exploration": "Collect the crystals", "transport": "Deliver the cargo", "flight": "Fly through all gates", "space": "Clear the asteroids"}[mode]
					if d.Objective == "" {
						d.Objective = "Play " + mode
					}
					if mode == "topdown" {
						d.Assets = []DesignAsset{{Role: "player", PackID: "robots-drones-animated-top-down", AssetID: "service_robot_move_down"}}
					}
					if mode == "blocks" {
						if d.Presentation != nil {
							d.Presentation.Environment = "coast"
						}
						d.Assets = []DesignAsset{{Role: "player", PackID: "blocks-and-balls", AssetID: "paddle_01"}, {Role: "ball", PackID: "blocks-and-balls", AssetID: "ball_01"}, {Role: "block", PackID: "blocks-and-balls", AssetID: "colored_block_01"}}
					}
					data, _ := json.Marshal(d)
					return s.SetDesignJSON(ctx, run.Job.ID, data)
				case "building":
					source, err := s.ReadJobFile(ctx, run.Job.ID, "src/main.ts")
					if err != nil {
						return err
					}
					// A small actual rule change, using the same edit primitive advertised to LLMs.
					old, replacement := `"speed": 5`, `"speed": 6`
					if dimension == "3d" && !strings.Contains(source, old) {
						old, replacement = `"speed":5`, `"speed":6`
					}
					if dimension == "2d" {
						old, replacement = "this.state.score++", "this.state.score += 2"
						if mode == "shooter" {
							old, replacement = "this.state.score += 10", "this.state.score += 20"
						}
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
				if r.URL.Path == "/" {
					w.Header().Set("Content-Type", "text/html")
					http.ServeFile(w, r, "testdata/studio-parent.html")
					return
				}
				if r.URL.Path == "/state" {
					job, _ := s.GetJob(r.Context(), job.ID)
					grant, _ := s.CreatePreviewGrant(p.ID)
					json.NewEncoder(w).Encode(map[string]any{"job": job, "grant": grant})
					return
				}
				if r.URL.Path == "/report" {
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
			if result.Get("status").Str() != "ready" {
				t.Fatalf("%s: %s; events=%s", mode, result.String(), page.MustEval(`()=>window.events`).String())
			}
			frame := page.MustElement("iframe").MustFrame()
			frame.MustWaitLoad()
			frame.MustWait(`()=>!!window.__AURAGO_GAME_TEST__`)
			state := frame.MustEval(`async()=>{const b=__AURAGO_GAME_TEST__;if(b.kind==='three')return b.snapshot();const {inspectAssets}=await import('./vendor/aurago-game-1.js');return inspectAssets(b.scene)}`)
			if (dimension == "3d" || mode == "blocks" || mode == "topdown") && state.Get("assets_used").Int() < 1 {
				t.Fatal("planned assets do not participate")
			}
			if mode == "board" {
				turns := frame.MustEval(`async()=>{const key=(name,type)=>window.dispatchEvent(new KeyboardEvent(type,{key:name,code:'KeyR',keyCode:82,which:82,bubbles:true}));key('r','keydown');key('r','keyup');await new Promise(r=>setTimeout(r,500));const b=__AURAGO_GAME_TEST__,canvas=b.scene.game.canvas,rect=canvas.getBoundingClientRect(),before=b.state.turns,opts={bubbles:true,clientX:rect.left+360/960*rect.width,clientY:rect.top+180/540*rect.height,button:0,buttons:1};canvas.dispatchEvent(new MouseEvent('mousedown',opts));canvas.dispatchEvent(new MouseEvent('mouseup',{...opts,buttons:0}));await new Promise(r=>setTimeout(r,100));return b.state.turns-before}`)
				if turns.Int() != 1 {
					t.Fatal("board mouse selection did not place one mark")
				}
			}
			reports, _ := filepath.Abs("../../reports/low-poly/guided")
			os.MkdirAll(reports, 0750)
			page.MustScreenshot(filepath.Join(reports, mode+".png"))
			events := page.MustEval(`()=>window.events`)
			os.WriteFile(filepath.Join(reports, mode+"-checks.json"), []byte(events.JSON("", "  ")), 0640)
			if mode == "fps" {
				page.MustSetViewport(390, 844, 1, false)
				page.MustScreenshot(filepath.Join(reports, "fps-touch.png"))
				page.MustSetViewport(1366, 768, 1, false)
			}
			// Page lifecycle must clear the observable binding and release the canvas.
			if dimension == "2d" {
				return
			}
			control := frame.MustEval(`async()=>{const wait=ms=>new Promise(r=>setTimeout(r,ms)),key=(k,down)=>window.dispatchEvent(new KeyboardEvent(down?'keydown':'keyup',{key:k,bubbles:true}));key('r',true);key('r',false);key('p',true);key('p',false);const before=__AURAGO_GAME_TEST__.snapshot();await wait(450);const paused=__AURAGO_GAME_TEST__.snapshot();key('p',true);key('p',false);key('d',true);await wait(200);window.dispatchEvent(new Event('blur'));const released=__AURAGO_GAME_TEST__.snapshot();await wait(300);const after=__AURAGO_GAME_TEST__.snapshot();return {paused:before.elapsed_ms===paused.elapsed_ms,blur:released.player_x===after.player_x}}`)
			if !control.Get("paused").Bool() || !control.Get("blur").Bool() {
				t.Fatal("pause/blur drift:", control)
			}
			if mode == "fps" || mode == "space" {
				lost := frame.MustEval(`async()=>{window.dispatchEvent(new KeyboardEvent('keydown',{key:'r'}));window.dispatchEvent(new KeyboardEvent('keyup',{key:'r'}));await new Promise(r=>setTimeout(r,16500));return document.body.innerText.includes('GAME OVER')&&__AURAGO_GAME_TEST__.snapshot().ended===1}`)
				if !lost.Bool() {
					t.Fatal("time limit did not end the game")
				}
			} else {
				won := frame.MustEval(`async()=>{window.dispatchEvent(new KeyboardEvent('keydown',{key:'r'}));window.dispatchEvent(new KeyboardEvent('keyup',{key:'r'}));window.dispatchEvent(new KeyboardEvent('keydown',{key:'w'}));await new Promise(r=>setTimeout(r,9000));window.dispatchEvent(new KeyboardEvent('keyup',{key:'w'}));return document.body.innerText.includes('COMPLETE')&&__AURAGO_GAME_TEST__.snapshot().ended===1}`)
				if !won.Bool() {
					t.Fatal("reachable objective did not complete:", frame.MustEval(`()=>__AURAGO_GAME_TEST__.snapshot()`))
				}
			}
			frame.MustEval(`()=>dispatchEvent(new Event('pagehide'))`)
			if frame.MustEval(`()=>!!window.__AURAGO_GAME_TEST__||!!document.querySelector('canvas')`).Bool() {
				t.Fatal("3D disposal leaked")
			}
			t.Logf("%s: published revision %d after real browser checks; %s", mode, result.Get("result_revision").Int(), state.String())
		})
	}
}
