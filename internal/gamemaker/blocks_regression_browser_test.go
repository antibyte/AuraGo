package gamemaker

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/input"
	"github.com/go-rod/rod/lib/launcher"
)

// This is deliberately opt-in: unlike the normal unit suite it runs the
// compiled Phaser game and drives it only through real keyboard actions.
func TestBlocksBrowserNaturalOutcomes(t *testing.T) {
	if os.Getenv("GAMEMAKER_BLOCKS_BROWSER") != "1" {
		t.Skip("set GAMEMAKER_BLOCKS_BROWSER=1 for real Blocks gameplay")
	}
	root := t.TempDir()
	project := Project{Name: "Blocks regression", Dimension: "2d"}
	if err := WriteScaffold(root, project); err != nil {
		t.Fatal(err)
	}
	plan := ExampleGamePlan(project)
	plan.Template, plan.Width, plan.Height = "blocks", 320, 240
	plan.Assets = nil
	if err := installGameTemplate(root, plan); err != nil {
		t.Fatal(err)
	}
	if result := buildDirectory(context.Background(), root, 150, 64<<20); !result.OK {
		t.Fatalf("build: %+v", result.Diagnostics)
	}
	server := httptest.NewServer(http.FileServer(http.Dir(root)))
	defer server.Close()
	bin := os.Getenv("CHROME_BIN")
	if bin == "" {
		bin = "C:/Program Files/Google/Chrome/Application/chrome.exe"
	}
	launch := launcher.New().Bin(bin).Headless(true).NoSandbox(true).Set("enable-unsafe-swiftshader")
	browser := rod.New().ControlURL(launch.MustLaunch()).MustConnect()
	defer browser.Close()
	page := browser.MustPage("about:blank")
	defer page.Close()
	page.MustEvalOnNewDocument("window.__blocksErrors=[];addEventListener('error',e=>window.__blocksErrors.push(e.message));addEventListener('unhandledrejection',e=>window.__blocksErrors.push(String(e.reason)))")
	page.MustNavigate(server.URL).MustWaitLoad()
	page.MustSetViewport(640, 480, 1, false)
	if err := page.Timeout(10 * time.Second).Wait(rod.Eval(`()=>!!window.__AURAGO_GAME_TEST__`)); err != nil {
		t.Fatalf("Blocks did not start: %v; errors=%s body=%s", err, page.MustEval("()=>JSON.stringify(window.__blocksErrors||[])").Str(), page.MustEval("()=>document.body.innerText").Str())
	}
	page.MustActivate()
	page.MustElement("canvas").MustClick()

	// Move away from the incoming ball and relaunch after each real miss. The
	// ball crosses the bottom in about 1.7s at this compact test resolution.
	// Keep the paddle away from the ball as it approaches the bottom edge.
	// These are ordinary held key actions; only read-only observations guide
	// which direction is pressed.
	for attempt := 0; attempt < 3; attempt++ {
		if err := page.Keyboard.Press(input.Space); err != nil {
			t.Fatal(err)
		}
		time.Sleep(120 * time.Millisecond)
		if err := page.Keyboard.Release(input.Space); err != nil {
			t.Fatal(err)
		}
		for sample := 0; sample < 90; sample++ {
			state := page.MustEval("()=>{const b=__AURAGO_GAME_TEST__;return {lives:b.state.lives,y:b.scene.ball.y,x:b.scene.ball.x,width:b.scene.boardWidth}}")
			if state.Get("lives").Int() < 3-attempt || state.Get("lives").Int() == 0 {
				break
			}
			key := input.ArrowLeft
			if state.Get("y").Num() > state.Get("width").Num()*.45 {
				if state.Get("x").Num() < state.Get("width").Num()/2 {
					key = input.ArrowRight
				}
			}
			if err := page.Keyboard.Press(key); err != nil {
				t.Fatal(err)
			}
			time.Sleep(80 * time.Millisecond)
			if err := page.Keyboard.Release(key); err != nil {
				t.Fatal(err)
			}
		}
		want := 2 - attempt
		if err := page.Timeout(2 * time.Second).Wait(rod.Eval(fmt.Sprintf("()=>__AURAGO_GAME_TEST__.state.lives <= %d", want))); err != nil {
			t.Fatalf("natural life loss %d was not observed: %v; state=%s", attempt+1, err, page.MustEval("()=>JSON.stringify({state:__AURAGO_GAME_TEST__.state,audit:__AURAGO_GAME_TEST__.scene.auditGame(),ball:{x:__AURAGO_GAME_TEST__.scene.ball.x,y:__AURAGO_GAME_TEST__.scene.ball.y,vx:__AURAGO_GAME_TEST__.scene.ball.body.velocity.x,vy:__AURAGO_GAME_TEST__.scene.ball.body.velocity.y},paddle:__AURAGO_GAME_TEST__.scene.player.x})").Str())
		}
	}
	if outcome := page.MustEval(`()=>__AURAGO_GAME_TEST__.scene.auditGame().outcome`).Str(); outcome != "lost" {
		t.Fatalf("natural loss outcome=%q", outcome)
	}
	if loseEvents := page.MustEval(`()=>__AURAGO_GAME_TEST__.scene.auditGame().events.lose`).Int(); loseEvents < 1 {
		t.Fatalf("natural loss event missing: %d", loseEvents)
	}

	// Restart, launch, and follow the live ball with held keyboard actions.
	// This proves brick progress through actual collisions rather than mutating
	// the scene state from the test page.
	if err := page.Keyboard.Press(input.KeyR); err != nil {
		t.Fatal(err)
	}
	time.Sleep(300 * time.Millisecond)
	if err := page.Keyboard.Release(input.KeyR); err != nil {
		t.Fatal(err)
	}
	if err := page.Timeout(2 * time.Second).Wait(rod.Eval(`()=>__AURAGO_GAME_TEST__.state.lives===3`)); err != nil {
		t.Fatal("restart did not restore lives")
	}
	if err := page.Keyboard.Press(input.Space); err != nil {
		t.Fatal(err)
	}
	time.Sleep(120 * time.Millisecond)
	if err := page.Keyboard.Release(input.Space); err != nil {
		t.Fatal(err)
	}
	lastLives := 3
	for i := 0; i < 360; i++ {
		state := page.MustEval(`()=>{const b=__AURAGO_GAME_TEST__;return {ended:b.state.ended,lives:b.state.lives,outcome:b.scene.auditGame().outcome,ball:b.scene.ball.x,paddle:b.scene.player.x}}`)
		if state.Get("ended").Int() != 0 {
			break
		}
		if state.Get("lives").Int() < lastLives {
			lastLives = state.Get("lives").Int()
			if lastLives > 0 {
				if err := page.Keyboard.Press(input.Space); err != nil {
					t.Fatal(err)
				}
				time.Sleep(120 * time.Millisecond)
				if err := page.Keyboard.Release(input.Space); err != nil {
					t.Fatal(err)
				}
			}
		}
		key := input.ArrowRight
		if state.Get("ball").Num() < state.Get("paddle").Num() {
			key = input.ArrowLeft
		}
		if err := page.Keyboard.Press(key); err != nil {
			t.Fatal(err)
		}
		time.Sleep(80 * time.Millisecond)
		if err := page.Keyboard.Release(key); err != nil {
			t.Fatal(err)
		}
	}
	if err := page.Timeout(3 * time.Second).Wait(rod.Eval(`()=>__AURAGO_GAME_TEST__.scene.auditGame().outcome==='won'`)); err != nil {
		t.Fatalf("natural win was not observed: %v; state=%s", err, page.MustEval("()=>JSON.stringify({state:__AURAGO_GAME_TEST__.state,audit:__AURAGO_GAME_TEST__.scene.auditGame(),ball:{x:__AURAGO_GAME_TEST__.scene.ball.x,y:__AURAGO_GAME_TEST__.scene.ball.y,vx:__AURAGO_GAME_TEST__.scene.ball.body.velocity.x,vy:__AURAGO_GAME_TEST__.scene.ball.body.velocity.y},paddle:__AURAGO_GAME_TEST__.scene.player.x})").Str())
	}
	if remaining := page.MustEval(`()=>__AURAGO_GAME_TEST__.scene.auditGame().remaining_bricks`).Int(); remaining != 0 {
		t.Fatalf("natural win left %d bricks", remaining)
	}
}
