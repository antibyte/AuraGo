package ui

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
)

// TestDesktopPinballSurvey screenshots AuraPinball harness scenes. Opt-in:
//
//	AURAGO_RUN_BROWSER_SMOKE=1 AURAGO_PINBALL_SURVEY=1 [AURAGO_PINBALL_SCENES="neon:2d:low,neon:top:low"] go test -run TestDesktopPinballSurvey -count=1 .
//
// Scene format table:scene:quality; scenes: follow:<seconds>[:narrow|:reduced] (real bot game with the ball cam, shots at the ball low / high or on a ramp / in the shooter lane / multiball), 2d, play, top, showcase, attract, action, closeup:x,y,h, preview[:x,y,h] (1280x640 lobby card shot, no ball), dmd:<scene>.
// PNGs go to AURAGO_BROWSER_ARTIFACT_DIR, else ../reports/pinball-survey (git-ignored).
func TestDesktopPinballSurvey(t *testing.T) {
	if os.Getenv("AURAGO_PINBALL_SURVEY") != "1" {
		t.Skip("set AURAGO_PINBALL_SURVEY=1 to run the AuraPinball survey")
	}
	requirePrecisionBrowserSmoke(t)
	scenes := os.Getenv("AURAGO_PINBALL_SCENES")
	if scenes == "" {
		scenes = "neon:2d:low"
	}
	dir := os.Getenv("AURAGO_BROWSER_ARTIFACT_DIR")
	if dir == "" {
		dir = filepath.Join("..", "reports", "pinball-survey")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	mux := http.NewServeMux()
	mux.Handle("/", jsTyped(http.FileServer(http.Dir("."))))
	mux.Handle("/node_modules/three/", http.StripPrefix("/node_modules/three/", jsTyped(http.FileServer(http.Dir(filepath.Join("..", "node_modules", "three"))))))
	mux.HandleFunc("/harness", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join("testdata", "pinball-harness.html"))
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	browser := newSmokeBrowserWith(t, func(l *launcher.Launcher) {
		l.Set("mute-audio")
		l.Set("enable-unsafe-swiftshader")
	})
	for _, spec := range strings.Split(scenes, ",") {
		spec = strings.TrimSpace(spec)
		if spec == "" {
			continue
		}
		table, scene, quality, ok := parsePinballSurveySpec(spec)
		if !ok {
			t.Fatalf("bad scene spec %q (want table:scene:quality)", spec)
		}
		t.Run(table+"-"+scene+"-"+quality, func(t *testing.T) {
			limit := 90 * time.Second
			if strings.HasPrefix(scene, "follow") {
				limit = 6 * time.Minute
			}
			page := browser.MustPage("about:blank").Timeout(limit)
			t.Cleanup(func() { _ = page.Close() })
			// Windows' animation setting leaks into headless Chrome as reduced motion; pin it so the ball cam really chases (":reduced" keeps the static framing).
			motion := "no-preference"
			if strings.HasSuffix(scene, ":reduced") {
				motion = "reduce"
			}
			if err := (proto.EmulationSetEmulatedMedia{Features: []*proto.EmulationMediaFeature{{Name: "prefers-reduced-motion", Value: motion}}}).Call(page); err != nil {
				t.Fatal(err)
			}
			if strings.HasSuffix(scene, ":narrow") {
				page.MustSetViewport(720, 1000, 1, false)
			} else if scene == "preview" || strings.HasPrefix(scene, "preview:") {
				page.MustSetViewport(1280, 640, 1, false) // 2:1 lobby card frame at twice the 640x320 delivery size
			} else {
				page.MustSetViewport(1440, 960, 1, false)
			}
			page.MustNavigate(server.URL + pinballSurveyURL(table, scene, quality))
			page.MustWait(`()=>window.pinballHarness?.ready===true||!!window.pinballHarnessError`)
			if msg := page.MustEval(`()=>window.pinballHarnessError||''`).Str(); msg != "" {
				t.Fatalf("harness failed: %s", msg)
			}
			time.Sleep(400 * time.Millisecond)
			if js := os.Getenv("AURAGO_PINBALL_EVAL"); js != "" {
				page.MustEval(js)
				time.Sleep(500 * time.Millisecond)
			}
			base := table + "-" + strings.NewReplacer(":", "-", ",", "_", ".", "p", ";", "_").Replace(scene) + "-" + quality
			shot := func(suffix string) {
				path := filepath.Join(dir, base+suffix+".png")
				if err := os.WriteFile(path, page.MustScreenshot(), 0o644); err != nil {
					t.Fatal(err)
				}
				t.Logf("wrote %s", path)
			}
			shot("")
			t.Logf("graphics %s", page.MustEval(`()=>{const h=window.pinballHarness;const g=h.game?h.game.inspect().graphics:(h.engine?h.engine.scene.stats():null);return g?JSON.stringify({q:g.quality,calls:g.drawCalls,tris:g.triangles,lights:g.lights,tex:g.textures}):""}`).Str())
			if strings.HasPrefix(scene, "follow") {
				surveyFollow(t, page, scene, shot)
			}
			if scene == "2d" {
				checkPinball2D(t, page)
				time.Sleep(200 * time.Millisecond)
				shot("-after")
			}
		})
	}
}

func jsTyped(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if strings.HasSuffix(r.URL.Path, ".js") || strings.HasSuffix(r.URL.Path, ".mjs") {
			w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
		}
		next.ServeHTTP(w, r)
	})
}

func parsePinballSurveySpec(spec string) (table, scene, quality string, ok bool) {
	parts := strings.Split(spec, ":")
	if len(parts) < 3 {
		return "", "", "", false
	}
	return parts[0], strings.Join(parts[1:len(parts)-1], ":"), parts[len(parts)-1], true
}

func pinballSurveyURL(table, scene, quality string) string {
	q := url.Values{"table": {table}, "quality": {quality}, "seed": {"1"}}
	switch {
	case scene == "2d":
		q.Set("mode", "2d")
		q.Set("debug", "1")
	case scene == "play":
		q.Set("mode", "game")
		q.Set("autoplay", "1")
	case strings.HasPrefix(scene, "follow"):
		q.Set("mode", "game")
		q.Set("scene", "follow")
		q.Set("camera", "follow")
	default:
		q.Set("mode", "engine")
		q.Set("scene", scene)
		if strings.HasPrefix(scene, "closeup:") {
			q.Set("camera", "closeup")
		} else if scene == "top" || scene == "showcase" {
			q.Set("camera", scene)
		}
	}
	return "/harness?" + q.Encode()
}

// surveyFollow lets the bot play in real time and screenshots the ball cam the first time each moment shows up.
func surveyFollow(t *testing.T, page *rod.Page, scene string, shot func(string)) {
	t.Helper()
	secs := 40
	if parts := strings.Split(scene, ":"); len(parts) > 1 {
		if n, err := strconv.Atoi(parts[1]); err == nil && n > 0 {
			secs = n
		}
	}
	taken := map[string]bool{}
	deadline := time.Now().Add(time.Duration(secs) * time.Second)
	for time.Now().Before(deadline) && len(taken) < 4 {
		tags := page.MustEval(`()=>{const g=window.pinballHarness.game,i=g.inspect(),b=(i.physics&&i.physics.balls||[]).filter(x=>['field','path','held','grab','shooter'].includes(x.mode));
const out=[];if(b.some(x=>x.mode==='shooter'))out.push('shooter');if(b.some(x=>x.y>0.72||x.h>0.02))out.push('high');
if(b.some(x=>x.mode==='field'&&x.y<0.3&&x.y>0.06&&Math.abs(x.x)<0.2))out.push('low');if(b.length>=2)out.push('multi');return out.join(',')}`).Str()
		for _, tag := range strings.Split(tags, ",") {
			if tag != "" && !taken[tag] {
				taken[tag] = true
				page.MustEval(`()=>window.pinballHarness.game.hold(true)`) // freeze the physics so the shot shows the moment, then let the camera settle
				// Headless software GL renders only a few frames per second and each frame advances the camera by at most 50 ms: wait for ~2 s of camera time.
				page.MustEval(`()=>new Promise(resolve=>{const s=window.pinballHarness.game.internals().scene,f0=s.stats().frames;const w=()=>s.stats().frames-f0>=40?resolve():setTimeout(w,50);w()})`)
				shot("-" + tag)
				page.MustEval(`()=>window.pinballHarness.game.hold(false)`)
				t.Logf("follow %s: %s", tag, page.MustEval(`()=>{const r=window.pinballHarness.game.internals().scene.three.rig;return JSON.stringify({mode:r.mode,follow:r.follow,balls:window.pinballHarness.game.inspect().physics.balls})}`).Str())
			}
		}
		time.Sleep(60 * time.Millisecond)
	}
	t.Logf("follow moments shot: %v", taken)
}

// checkPinball2D proves plunging and flipping work: serve a ball, pull and release the plunger, then fire the flippers.
func checkPinball2D(t *testing.T, page *rod.Page) {
	t.Helper()
	got := page.MustEval(`()=>{const h=window.pinballHarness;h.clock.paused=true;h.reset();h.addBall(false);h.step(120);
h.setInput({plunger:1});h.step(480);h.setInput({plunger:0});h.step(720);h.setInput({left:true,right:true});h.step(120);h.setInput({left:false,right:false});
const types=[...new Set(h.events.map(e=>e.type))];return JSON.stringify({types,balls:h.snapshot().balls.map(b=>[+b.x.toFixed(3),+b.y.toFixed(3),b.mode])})}`).Str()
	t.Logf("2d interaction: %s", got)
	for _, want := range []string{`"shooter"`, `"launch"`, `"flipper"`} {
		if !strings.Contains(got, want) {
			t.Errorf("2d interaction missing %s event: %s", want, got)
		}
	}
}
