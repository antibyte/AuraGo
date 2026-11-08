package ui

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
)

// TestDesktopPinballSurvey screenshots AuraPinball harness scenes. Opt-in:
//
//	AURAGO_RUN_BROWSER_SMOKE=1 AURAGO_PINBALL_SURVEY=1 [AURAGO_PINBALL_SCENES="neon:2d:low,neon:top:low"] go test -run TestDesktopPinballSurvey -count=1 .
//
// Scene format table:scene:quality; scenes: 2d, play, top, showcase, attract, action, closeup:x,y,h, dmd:<scene>.
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
			page := browser.MustPage("about:blank").Timeout(90 * time.Second)
			t.Cleanup(func() { _ = page.Close() })
			page.MustSetViewport(1440, 960, 1, false)
			page.MustNavigate(server.URL + pinballSurveyURL(table, scene, quality))
			page.MustWait(`()=>window.pinballHarness?.ready===true||!!window.pinballHarnessError`)
			if msg := page.MustEval(`()=>window.pinballHarnessError||''`).Str(); msg != "" {
				t.Fatalf("harness failed: %s", msg)
			}
			time.Sleep(400 * time.Millisecond)
			base := table + "-" + strings.NewReplacer(":", "-", ",", "_", ".", "p").Replace(scene)
			shot := func(suffix string) {
				path := filepath.Join(dir, base+suffix+".png")
				if err := os.WriteFile(path, page.MustScreenshot(), 0o644); err != nil {
					t.Fatal(err)
				}
				t.Logf("wrote %s", path)
			}
			shot("")
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
