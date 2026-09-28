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
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"aurago/internal/desktop"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/input"
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
	if fake := os.Getenv("AURAGO_SCREENSAVER_FAKE_CLOCK"); fake != "" {
		page.MustEval(`iso=>{const offset=new Date(iso).getTime()-Date.now();const Real=Date;class Shifted extends Real{constructor(...a){super(...(a.length?a:[Real.now()+offset]))}static now(){return Real.now()+offset}};window.Date=Shifted}`, fake)
	}

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
			page.Timeout(90 * time.Second).MustWait(`()=>{const c=AuraScreensaverHost.inspect().current;return !!c && (c.frames>=3 || c.state==='poster')}`)
			report := page.MustEval(`()=>JSON.stringify(AuraScreensaverHost.inspect().current)`).Str()
			if strings.Contains(report, `"state":"poster"`) {
				t.Fatalf("%s fell back to the poster: %s", theme, report)
			}
			for i, at := range shots {
				page.Timeout(3*time.Minute).MustWait(`at=>{const c=AuraScreensaverHost.inspect().current;return !!c && c.time>=at}`, at)
				raw := page.Timeout(60 * time.Second).MustEval(`async()=>JSON.stringify(await AuraScreensaverHost.probe())`).Str()
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
	if fake := os.Getenv("AURAGO_SCREENSAVER_FAKE_CLOCK"); fake != "" {
		page.MustEval(`iso=>{const offset=new Date(iso).getTime()-Date.now();const Real=Date;class Shifted extends Real{constructor(...a){super(...(a.length?a:[Real.now()+offset]))}static now(){return Real.now()+offset}};window.Date=Shifted}`, fake)
	}
	if err := os.MkdirAll(out, 0o755); err != nil {
		t.Fatal(err)
	}
	moments := map[string]float64{"aurora": 14, "event_horizon": 9, "ink": 22, "stardust": 16, "abyss": 12}
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

func TestDesktopScreensaverShellBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)

	html := regexp.MustCompile(`(?s)<script\b[^>]*>.*?</script>`).ReplaceAllString(readDesktopAssetText(t, "desktop.html"), "")
	html = regexp.MustCompile(`\{\{[^}]*\}\}`).ReplaceAllString(html, "")
	html = strings.Replace(html, "</body>", `<script>
window.fixtureErrors=[];
addEventListener('error', e=>fixtureErrors.push(e.message));
addEventListener('unhandledrejection', e=>fixtureErrors.push(String(e.reason)));
window.t=key=>key;
window.WebSocket=class extends EventTarget { close(){} };
</script><script src="/js/shared/lazy-assets.js"></script><script src="/js/desktop/core/module-loader.js"></script><script src="/screensaver-shell.js"></script></body>`, 1)

	shell := readDesktopAssetText(t, "js/desktop/bundles/main.bundle.js")
	cut := strings.LastIndex(shell, "    ensureDesktopRadialMenuAnchor();")
	if cut < 0 {
		t.Fatal("desktop startup seam missing")
	}
	shell = shell[:cut] + `window.ssTest={state,openApp};` + shell[cut:]

	settings := desktop.DesktopSettingDefaults()
	for key, value := range map[string]string{
		"windows.restore_session":  "false",
		"pet.enabled":              "false",
		"phone_gadget.enabled":     "false",
		"desktop.show_widgets":     "false",
		"screensaver.enabled":      "true",
		"screensaver.theme":        "aurora",
		"screensaver.idle_minutes": "1",
	} {
		settings[key] = value
	}
	var mu sync.Mutex
	puts := []string{}
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(Content)))
	mux.HandleFunc("/fixture", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, html)
	})
	mux.HandleFunc("/screensaver-shell.js", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/javascript")
		fmt.Fprint(w, shell)
	})
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/desktop/bootstrap":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"enabled": true, "builtin_apps": desktop.BuiltinApps(), "installed_apps": []interface{}{},
				"widgets": []interface{}{}, "shortcuts": []interface{}{}, "desktop_files": []interface{}{},
				"workspace": map[string]interface{}{"readonly": false}, "settings": settings,
			})
		case "/api/desktop/settings":
			if r.Method == http.MethodPut {
				var update struct{ Key, Value string }
				if err := json.NewDecoder(r.Body).Decode(&update); err != nil || update.Key == "" {
					http.Error(w, `{"error":"invalid setting"}`, http.StatusBadRequest)
					return
				}
				settings[update.Key] = update.Value
				puts = append(puts, update.Key+"="+update.Value)
			}
			json.NewEncoder(w).Encode(map[string]interface{}{"settings": settings})
		default:
			fmt.Fprint(w, `{"status":"ok","files":[],"pets":[],"settings":{},"enabled":false}`)
		}
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	browser, closeBrowser := newScreensaverBrowser(t)
	defer closeBrowser()
	page := browser.MustPage("about:blank").Timeout(4 * time.Minute)
	defer page.MustClose()
	page.MustSetViewport(1280, 720, 1, false)
	setReducedMotion(t, page, "no-preference")
	page.MustNavigate(server.URL + "/fixture")
	page.MustWaitLoad()
	page.MustWait(`()=>!!(window.ssTest?.state?.bootstrap && window.DesktopScreensaver) || window.fixtureErrors?.length`)
	page.MustEval(`()=>{
        Object.defineProperty(document,'hidden',{configurable:true,get:()=>false});
        window.desktopClicks=0; window.desktopKeys=0;
        document.addEventListener('click',()=>{window.desktopClicks++;});
        document.addEventListener('keydown',()=>{window.desktopKeys++;});
    }`)
	check := func(js, message string) {
		t.Helper()
		if !page.MustEval(js).Bool() {
			t.Fatalf("%s: %s", message, page.MustEval(`()=>JSON.stringify(DesktopScreensaver.inspect())`).Str())
		}
	}
	artifacts := screensaverArtifactDir(t)
	shot := func(name string) {
		if artifacts != "" {
			page.MustScreenshot(filepath.Join(artifacts, "shell-"+name+".png"))
		}
	}

	check(`()=>DesktopScreensaver.inspect().enabled && DesktopScreensaver.inspect().wired`, "enabled setting must arm idle detection")
	check(`()=>DesktopScreensaver.inspect().idleMs===60000`, "idle minutes must map to milliseconds")

	// Idle activation.
	page.MustEval(`()=>DesktopScreensaver.setIdleOverrideMs(1200)`)
	page.Timeout(60 * time.Second).MustWait(`()=>{const h=DesktopScreensaver.inspect().host;return DesktopScreensaver.inspect().active && h && h.current && h.current.frames>2}`)
	check(`()=>DesktopScreensaver.inspect().theme==='aurora' && !DesktopScreensaver.inspect().preview`, "idle activation must use the configured theme")
	check(`()=>getComputedStyle(document.getElementById('vd-screensaver')).zIndex==='20000'`, "overlay must use the screensaver layer")
	shot("active")

	// A real click wakes the desktop but never reaches it.
	page.Mouse.MustMoveTo(640, 360)
	page.Mouse.MustClick(proto.InputMouseButtonLeft)
	page.Timeout(10 * time.Second).MustWait(`()=>!DesktopScreensaver.inspect().active && !document.getElementById('vd-screensaver')`)
	check(`()=>window.desktopClicks===0`, "the waking click must be swallowed")
	check(`()=>DesktopScreensaver.inspect().host.last.stoppedBy==='input'`, "wake must report input")
	time.Sleep(600 * time.Millisecond)
	check(`()=>!DesktopScreensaver.inspect().active`, "activity after wake must restart the idle timer")

	// Fullscreen suppresses activation.
	page.MustEval(`()=>Object.defineProperty(document,'fullscreenElement',{configurable:true,get:()=>document.body})`)
	time.Sleep(3500 * time.Millisecond)
	check(`()=>!DesktopScreensaver.inspect().active && DesktopScreensaver.inspect().suppressedReason==='fullscreen'`, "fullscreen must suppress the screensaver")
	page.MustEval(`()=>Object.defineProperty(document,'fullscreenElement',{configurable:true,get:()=>null})`)
	page.Timeout(60 * time.Second).MustWait(`()=>DesktopScreensaver.inspect().active`)

	// An incoming call wakes the desktop immediately and keeps it awake while ringing.
	page.MustEval(`()=>{const n=document.createElement('section');n.id='vd-sip-incoming';document.body.appendChild(n);}`)
	page.Timeout(10 * time.Second).MustWait(`()=>!DesktopScreensaver.inspect().active`)
	check(`()=>DesktopScreensaver.inspect().host.last.stoppedBy==='call'`, "incoming calls must stop the screensaver")
	time.Sleep(3 * time.Second)
	check(`()=>!DesktopScreensaver.inspect().active && DesktopScreensaver.inspect().suppressedReason==='call'`, "ringing calls must suppress activation")
	page.MustEval(`()=>{document.getElementById('vd-sip-incoming').remove();DesktopScreensaver.setIdleOverrideMs(0);}`)

	// Settings: preview, scene choice, random and disable.
	page.MustEval(`async()=>{ await AuraDesktopModules.loadAppAssets('settings'); ssTest.openApp('settings',{category:'screensaver'}); }`)
	page.Timeout(20 * time.Second).MustWait(`()=>document.querySelectorAll('[data-screensaver-theme]').length===6`)
	shot("settings")
	preview := page.MustElement(`[data-screensaver-preview="event_horizon"]`)
	preview.MustScrollIntoView()
	preview.MustClick()
	page.Timeout(60 * time.Second).MustWait(`()=>{const i=DesktopScreensaver.inspect();return i.active && i.preview && i.theme==='event_horizon' && i.host?.current?.frames>2}`)
	shot("preview")
	keysBefore := page.MustEval(`()=>window.desktopKeys`).Int()
	page.Keyboard.MustType(input.Escape)
	page.Timeout(10 * time.Second).MustWait(`()=>!DesktopScreensaver.inspect().active`)
	if page.MustEval(`()=>window.desktopKeys`).Int() != keysBefore {
		t.Fatal("the waking key press must not reach the desktop")
	}

	time.Sleep(700 * time.Millisecond)
	page.MustElement(`[data-screensaver-theme="random"]`).MustClick()
	page.Timeout(10 * time.Second).MustWait(`()=>document.querySelector('[data-screensaver-theme="random"]')?.getAttribute('aria-pressed')==='true'`)
	seen := page.MustEval(`async()=>{
        const themes=[];
        for (let i=0;i<4;i++) {
            await DesktopScreensaver.start('random');
            themes.push(DesktopScreensaver.inspect().theme);
            DesktopScreensaver.stop('test');
        }
        return JSON.stringify(themes);
    }`).Str()
	var themes []string
	if err := json.Unmarshal([]byte(seen), &themes); err != nil {
		t.Fatal(err)
	}
	for i := 1; i < len(themes); i++ {
		if themes[i] == themes[i-1] || !containsString(screensaverThemeIDs, themes[i]) {
			t.Fatalf("random mode must pick a different valid scene each time: %v", themes)
		}
	}

	time.Sleep(700 * time.Millisecond)
	page.MustEval(`()=>document.querySelector('[data-setting-key="screensaver.enabled"]').closest('label').click()`)
	page.Timeout(10 * time.Second).MustWait(`()=>!DesktopScreensaver.inspect().wired`)
	mu.Lock()
	joined := strings.Join(puts, ",")
	mu.Unlock()
	for _, want := range []string{"screensaver.theme=random", "screensaver.enabled=false"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("settings did not persist %s (saw %s)", want, joined)
		}
	}
	if errs := page.MustEval(`()=>JSON.stringify(fixtureErrors)`).Str(); errs != "[]" {
		t.Fatalf("page errors: %s", errs)
	}
}
