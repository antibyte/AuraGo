package ui

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
)

const screensaverSceneHarness = `<!doctype html>
<html lang="de"><head><meta charset="utf-8"><title>Screensaver scenes</title>
<link rel="stylesheet" href="/fonts/fonts.css"><link rel="stylesheet" href="/css/desktop-screensaver.css"></head>
<body style="margin:0;background:#223">
<script>
window.BUILD_VERSION='screensaver-test';
window.fixtureErrors=[];
addEventListener('error', e=>fixtureErrors.push(String(e.message)));
addEventListener('unhandledrejection', e=>fixtureErrors.push(String(e.reason)));
window.t=key=>key;
</script>
<script src="/js/shared/lazy-assets.js"></script>
<script src="/js/desktop/screensavers/host.js"></script>
</body></html>`

func newScreensaverBrowser(t *testing.T) (*rod.Browser, func()) {
	t.Helper()
	bin, ok := browserExecutable()
	if !ok {
		t.Skip("screensaver browser test requires Chrome or Edge")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	l := launcher.New().Context(ctx).Bin(bin).Headless(true).NoSandbox(true)
	if os.Getenv("AURAGO_SCREENSAVER_GPU") == "1" {
		l = l.Set("use-angle", "d3d11").Set("enable-gpu").Set("ignore-gpu-blocklist")
	} else {
		l = l.Set("enable-unsafe-swiftshader")
	}
	url, err := l.Launch()
	if err != nil {
		cancel()
		t.Skipf("screensaver browser launch failed: %v", err)
	}
	b := rod.New().Context(ctx).ControlURL(url).MustConnect()
	return b, func() {
		_ = b.Close()
		l.Kill()
		l.Cleanup()
		cancel()
	}
}

func newScreensaverSceneServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/fixture", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, screensaverSceneHarness)
	})
	mux.Handle("/", http.FileServer(http.FS(Content)))
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return server
}

func screensaverArtifactDir(t *testing.T) string {
	t.Helper()
	dir := os.Getenv("AURAGO_BROWSER_ARTIFACT_DIR")
	if dir == "" {
		return ""
	}
	dir = filepath.Join(dir, "screensaver")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

func setReducedMotion(t *testing.T, page *rod.Page, value string) {
	t.Helper()
	if err := (proto.EmulationSetEmulatedMedia{Features: []*proto.EmulationMediaFeature{{Name: "prefers-reduced-motion", Value: value}}}).Call(page); err != nil {
		t.Fatal(err)
	}
}

type screensaverProbe struct {
	Mean        float64 `json:"mean"`
	Max         float64 `json:"max"`
	LitFraction float64 `json:"litFraction"`
	Width       int     `json:"width"`
	Height      int     `json:"height"`
	Error       int     `json:"error"`
}

func TestDesktopScreensaverScenesBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)

	server := newScreensaverSceneServer(t)
	browser, closeBrowser := newScreensaverBrowser(t)
	defer closeBrowser()
	page := browser.MustPage("about:blank").Timeout(8 * time.Minute)
	defer page.MustClose()
	width, height := 1280, 720
	if os.Getenv("AURAGO_SCREENSAVER_FULLHD") == "1" {
		width, height = 1920, 1080
	}
	page.MustSetViewport(width, height, 1, false)
	setReducedMotion(t, page, "no-preference")
	page.MustNavigate(server.URL + "/fixture")
	page.MustWaitLoad()
	page.MustWait(`()=>!!window.AuraScreensaverHost`)

	artifacts := screensaverArtifactDir(t)
	themes := []string{"aurora", "event_horizon", "ink", "stardust", "abyss"}
	if only := os.Getenv("AURAGO_SCREENSAVER_THEMES"); only != "" {
		themes = strings.Split(only, ",")
	}
	shots := []float64{2.5, 7}
	if os.Getenv("AURAGO_SCREENSAVER_LONG") == "1" {
		shots = []float64{2.5, 8, 16, 28}
	}
	for _, theme := range themes {
		theme := strings.TrimSpace(theme)
		t.Run(theme, func(t *testing.T) {
			page.MustEval(`theme=>AuraScreensaverHost.start({theme, clock:true, reducedMotion:false, lang:'de', t:window.t})`, theme)
			page.Timeout(90*time.Second).MustWait(`()=>{const c=AuraScreensaverHost.inspect().current;return !!c && (c.frames>=3 || c.state==='poster')}`)
			report := page.MustEval(`()=>JSON.stringify(AuraScreensaverHost.inspect().current)`).Str()
			if strings.Contains(report, `"state":"poster"`) {
				t.Fatalf("%s fell back to the poster: %s", theme, report)
			}
			for i, at := range shots {
				page.Timeout(3*time.Minute).MustWait(`at=>{const c=AuraScreensaverHost.inspect().current;return !!c && c.time>=at}`, at)
				raw := page.Timeout(60*time.Second).MustEval(`async()=>JSON.stringify(await AuraScreensaverHost.probe())`).Str()
				var probe screensaverProbe
				if err := json.Unmarshal([]byte(raw), &probe); err != nil {
					t.Fatalf("%s probe decode %q: %v", theme, raw, err)
				}
				t.Logf("%s t=%.1fs probe=%s", theme, at, raw)
				if probe.Error != 0 {
					t.Fatalf("%s reported GL error %d", theme, probe.Error)
				}
				if probe.Mean < 2 || probe.Max < 60 || probe.LitFraction < 0.02 {
					t.Fatalf("%s rendered an (almost) black frame: %s", theme, raw)
				}
				if artifacts != "" {
					page.MustScreenshot(filepath.Join(artifacts, fmt.Sprintf("%s-%d.png", theme, i)))
				}
			}
			page.MustEval(`()=>AuraScreensaverHost.stop({reason:'test'})`)
			page.Timeout(10 * time.Second).MustWait(`()=>!document.getElementById('vd-screensaver')`)
			last := page.MustEval(`()=>JSON.stringify(AuraScreensaverHost.inspect().last)`).Str()
			t.Logf("%s final=%s", theme, last)
			if errs := page.MustEval(`()=>JSON.stringify(fixtureErrors)`).Str(); errs != "[]" {
				t.Fatalf("%s page errors: %s", theme, errs)
			}
		})
	}

	t.Run("reduced-motion-poster", func(t *testing.T) {
		setReducedMotion(t, page, "reduce")
		defer setReducedMotion(t, page, "no-preference")
		page.MustEval(`()=>AuraScreensaverHost.start({theme:'aurora', clock:true, reducedMotion:true, lang:'de', t:window.t})`)
		page.Timeout(10 * time.Second).MustWait(`()=>AuraScreensaverHost.inspect().current?.state==='poster'`)
		if page.MustEval(`()=>!!document.querySelector('#vd-screensaver canvas') && AuraScreensaverHost.inspect().current.frames===0`).Bool() == false {
			t.Fatal("reduced motion must not render frames")
		}
		clock := page.MustEval(`()=>AuraScreensaverHost.inspect().current.clockText`).Str()
		if strings.TrimSpace(clock) == "" {
			t.Fatal("reduced motion poster must keep the clock")
		}
		page.MustEval(`()=>AuraScreensaverHost.stop({reason:'test'})`)
		page.Timeout(10 * time.Second).MustWait(`()=>!document.getElementById('vd-screensaver')`)
	})
}

// TestDesktopScreensaverPosterCapture renders every scene and writes the settings
// thumbnails and fallback posters. It only runs on demand.
func TestDesktopScreensaverPosterCapture(t *testing.T) {
	out := os.Getenv("AURAGO_SCREENSAVER_POSTERS")
	if out == "" {
		t.Skip("set AURAGO_SCREENSAVER_POSTERS=<dir> to capture screensaver posters")
	}
	server := newScreensaverSceneServer(t)
	browser, closeBrowser := newScreensaverBrowser(t)
	defer closeBrowser()
	page := browser.MustPage("about:blank").Timeout(10 * time.Minute)
	defer page.MustClose()
	page.MustSetViewport(1920, 1080, 1, false)
	setReducedMotion(t, page, "no-preference")
	page.MustNavigate(server.URL + "/fixture")
	page.MustWaitLoad()
	page.MustWait(`()=>!!window.AuraScreensaverHost`)
	if err := os.MkdirAll(out, 0o755); err != nil {
		t.Fatal(err)
	}
	moments := map[string]float64{"aurora": 14, "event_horizon": 9, "ink": 16, "stardust": 26, "abyss": 12}
	for theme, at := range moments {
		if only := os.Getenv("AURAGO_SCREENSAVER_THEMES"); only != "" && !strings.Contains(only, theme) {
			continue
		}
		page.MustEval(`theme=>AuraScreensaverHost.start({theme, clock:false, reducedMotion:false, lang:'de', t:window.t})`, theme)
		page.Timeout(5*time.Minute).MustWait(`at=>{const c=AuraScreensaverHost.inspect().current;return !!c && c.time>=at}`, at)
		for _, size := range []struct {
			name          string
			width, height int
			quality       float64
		}{{theme, 1920, 1080, 0.82}, {theme + "-thumb", 480, 270, 0.86}} {
			dataURL := page.Timeout(60*time.Second).MustEval(`(w,h,q)=>AuraScreensaverHost.capture({width:w,height:h,type:'image/webp',quality:q})`, size.width, size.height, size.quality).Str()
			payload := strings.TrimPrefix(dataURL, "data:image/webp;base64,")
			if payload == dataURL {
				t.Fatalf("%s capture did not return webp", size.name)
			}
			bytes, err := base64.StdEncoding.DecodeString(payload)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(out, size.name+".webp"), bytes, 0o644); err != nil {
				t.Fatal(err)
			}
			t.Logf("%s: %d bytes", size.name, len(bytes))
		}
		page.MustEval(`()=>AuraScreensaverHost.stop({reason:'pagehide'})`)
	}
}
