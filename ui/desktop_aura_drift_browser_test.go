package ui

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"aurago/internal/desktop"
	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/input"
)

// auraDriftFixture serves the real Desktop shell with the in-memory browser
// fixture and returns an initialised page.
func auraDriftFixture(t *testing.T) *rod.Page {
	t.Helper()
	return auraDriftFixtureWithout(t, "")
}

// auraDriftFixtureWithout optionally removes one script from the served
// module-loader.js to reproduce a stale service-worker cache.
func auraDriftFixtureWithout(t *testing.T, script string) *rod.Page {
	t.Helper()
	return auraDriftFixtureServed(t, script, "")
}

// auraDriftFixtureFailing answers one URL path with a 404 (a missing GLB).
func auraDriftFixtureFailing(t *testing.T, path string) *rod.Page {
	t.Helper()
	return auraDriftFixtureServed(t, "", path)
}

func auraDriftFixtureServed(t *testing.T, script, failPath string) *rod.Page {
	t.Helper()
	html := regexp.MustCompile(`(?s)<script\b[^>]*>.*?</script>`).ReplaceAllString(readDesktopAssetText(t, "desktop.html"), "")
	html = regexp.MustCompile(`\{\{[^}]*\}\}`).ReplaceAllString(html, "")
	html = strings.Replace(html, "</body>", `<script src="/js/shared/lazy-assets.js"></script><script src="/js/desktop/core/module-loader.js"></script><script src="/racer-shell.js"></script><script src="/testdata/aurora-fixture.js"></script></body>`, 1)
	shell := readDesktopAssetText(t, "js/desktop/bundles/main.bundle.js")
	cut := strings.LastIndex(shell, "    ensureDesktopRadialMenuAnchor();")
	if cut < 0 {
		t.Fatal("desktop startup seam missing")
	}
	shell = shell[:cut] + `window.aurora={state,loadIconManifest,applyDesktopSettings,renderIcons,renderStartApps,renderStartButtonIcon,wireShellChromeControls,bindViewportMetrics,handleDesktopKeydown,closeContextMenu,openApp,closeWindow};})();`
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(Content)))
	if script != "" {
		loader := readDesktopAssetText(t, "js/desktop/core/module-loader.js")
		lines := strings.Split(loader, "\n")
		kept := make([]string, 0, len(lines))
		removed := 0
		for _, line := range lines {
			needle := "'/js/desktop/apps/" + script + "'"
			if strings.HasPrefix(script, "/") {
				needle = "'" + script + "'"
			}
			if strings.Contains(line, needle) {
				removed++
				continue
			}
			kept = append(kept, line)
		}
		if removed != 1 {
			t.Fatalf("expected exactly one loader reference to %s, found %d", script, removed)
		}
		stale := strings.Join(kept, "\n")
		mux.HandleFunc("/js/desktop/core/module-loader.js", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/javascript")
			fmt.Fprint(w, stale)
		})
	}
	if failPath != "" {
		mux.HandleFunc(failPath, func(w http.ResponseWriter, r *http.Request) { http.NotFound(w, r) })
	}
	mux.HandleFunc("/fixture", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, html) })
	mux.HandleFunc("/racer-shell.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/javascript")
		fmt.Fprint(w, shell)
	})
	mux.HandleFunc("/fixture-words", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, readDesktopAssetText(t, "lang/desktop/de.json"))
	})
	mux.HandleFunc("/fixture-apps", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(desktop.BuiltinApps())
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	// The page context is shared by every operation in a test; a short timeout
	// here would expire midway through the multi-case quality matrix.
	page := newSmokeBrowser(t).MustPage(server.URL + "/fixture").Timeout(3 * time.Minute)
	t.Cleanup(func() { page.Close() })
	page.MustSetViewport(1280, 800, 1, false)
	page.MustWaitLoad()
	page.MustEval(`async()=>{await fixtureReady;}`)
	return page
}

func auraDriftArtifactDir(t *testing.T, name string) string {
	t.Helper()
	base := os.Getenv("AURAGO_BROWSER_ARTIFACT_DIR")
	if base == "" {
		return ""
	}
	dir := filepath.Join(base, fmt.Sprintf("%s-%d", name, time.Now().UnixNano()))
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	return dir
}

func auraDriftCloseWindows(page *rod.Page) {
	page.MustEval(`()=>[...aurora.state.windows.keys()].forEach(id=>aurora.closeWindow(id))`)
	page.MustEval(`()=>new Promise(resolve=>setTimeout(resolve,150))`)
}

// auraDriftStats is the slice of AuraDriftApp.stats() the tests read.
type auraDriftStats struct {
	State    string  `json:"state"`
	Time     float64 `json:"time"`
	SpeedKmh float64 `json:"speedKmh"`
	Paused   bool    `json:"paused"`
	Quality  string  `json:"quality"`
	Renderer struct {
		Kind       string `json:"kind"`
		Calls      int    `json:"calls"`
		Triangles  int    `json:"triangles"`
		Geometries int    `json:"geometries"`
		StandIns   int    `json:"standIns"`
	} `json:"renderer"`
	Post struct {
		Available bool   `json:"available"`
		Enabled   bool   `json:"enabled"`
		Degraded  bool   `json:"degraded"`
		Biome     string `json:"biome"`
	} `json:"post"`
}

func auraDriftReadStats(t *testing.T, page *rod.Page) (auraDriftStats, string) {
	t.Helper()
	raw := page.MustEval(`()=>JSON.stringify(window.AuraDriftApp.stats()[0]||{})`).Str()
	var stats auraDriftStats
	if err := json.Unmarshal([]byte(raw), &stats); err != nil {
		t.Fatalf("bad stats JSON %q: %v", raw, err)
	}
	return stats, raw
}

// auraDriftWaitState polls until the shell reports the wanted state. The loading
// step is variable, so no test sleeps a fixed time before the race exists.
func auraDriftWaitState(t *testing.T, page *rod.Page, want string, limit time.Duration) {
	t.Helper()
	deadline := time.Now().Add(limit)
	last := ""
	for time.Now().Before(deadline) {
		last = page.MustEval(`()=>{const s=window.AuraDriftApp.stats()[0];return s?s.state:'none';}`).Str()
		if last == want {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("Aura Drift never reached state %q within %s (last %q)", want, limit, last)
}

// auraDriftSkipCountdown steps the simulation through the start countdown, so
// slow software GL cannot stretch it (the sim advances one step per fixed tick,
// not per rendered frame, and rendering is skipped).
func auraDriftSkipCountdown(t *testing.T, page *rod.Page) {
	t.Helper()
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		state := page.MustEval(`()=>{const s=window.AuraDriftApp.stats()[0];return s?s.state:'none';}`).Str()
		if state == "countdown" {
			page.MustEval(`()=>AuraDriftApp.debug.fastForward(4,{renderEvery:100000})`)
			break
		}
		if state == "running" {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	auraDriftWaitState(t, page, "running", 10*time.Second)
}

func auraDriftKeyDown(t *testing.T, page *rod.Page, key input.Key) {
	t.Helper()
	if err := page.Keyboard.Press(key); err != nil {
		t.Fatal(err)
	}
}

func auraDriftKeyUp(t *testing.T, page *rod.Page, key input.Key) {
	t.Helper()
	if err := page.Keyboard.Release(key); err != nil {
		t.Fatal(err)
	}
}

func auraDriftSleep(page *rod.Page, ms int) {
	page.MustEval(`ms=>new Promise(resolve=>setTimeout(resolve,ms))`, ms)
}

func auraDriftHasWebGL(page *rod.Page) bool {
	return page.MustEval(`()=>!!document.createElement('canvas').getContext('webgl2')`).Bool()
}

// auraDriftReopen stores the settings, reloads a fresh document and opens the app.
func auraDriftReopen(page *rod.Page, settings map[string]interface{}, theme string) {
	base := map[string]interface{}{"quality": "low", "volume": 0, "muted": true, "reducedMotion": false, "weapons": true, "showGhost": false}
	for k, v := range settings {
		base[k] = v
	}
	page.MustEval(`(cfg)=>{localStorage.setItem('aurago.desktop.aura-drift.v1', JSON.stringify(cfg));}`, base)
	page.MustReload()
	page.MustWaitLoad()
	page.MustEval(`async theme=>{await fixtureReady;if(theme)fixtureTheme(theme);await fixtureOpen('aura-drift');}`, theme)
}

func auraDriftNoErrors(t *testing.T, page *rod.Page) {
	t.Helper()
	if errors := page.MustEval(`()=>JSON.stringify(fixtureErrors)`).Str(); errors != "[]" {
		t.Fatal(errors)
	}
}

// auraDriftSurfaceIssues checks the active canvas and the HUD against the root rect.
const auraDriftSurfaceJS = `()=>{
    const root=document.querySelector('.agdrift-root'),issues=[];
    if(!root)return 'app root missing';
    const r=root.getBoundingClientRect();
    const s=window.AuraDriftApp.stats()[0];
    const three=s.renderer.kind==='three';
    const canvas=document.querySelector(three?'canvas.agdrift-gl':'canvas.agdrift-world');
    if(!canvas)return 'no active canvas for '+s.renderer.kind;
    const dpr=window.devicePixelRatio||1;
    const tierDpr=three?AuraDriftCore.QUALITY[s.quality].gl.dpr:2;
    const expected=Math.round(r.width*Math.min(tierDpr,dpr));
    if(canvas.width!==expected)issues.push('canvas '+canvas.width+' != '+expected);
    const hud=document.querySelector('.agdrift-hud');
    if(!hud||hud.width!==Math.round(r.width*Math.min(2,dpr)))issues.push('hud canvas width mismatch');
    if(document.querySelector('.agdrift-post'))issues.push('old post canvas still present');
    return issues.join('; ');
}`

// Opens Aura Drift in the real Desktop shell, starts a race, drives with
// synthetic key events and verifies that no runtime error escapes.
func TestDesktopAuraDriftBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	page := auraDriftFixture(t)
	auraDriftReopen(page, map[string]interface{}{"quality": "medium"}, "")

	if !page.MustEval(`()=>!!document.querySelector('.agdrift-root .agdrift-menu:not([hidden])')`).Bool() {
		t.Fatal("Aura Drift menu did not mount in the desktop window")
	}
	if !page.MustEval(`()=>{
        const c=document.querySelector('canvas.agdrift-gl:not([hidden])')||document.querySelector('canvas.agdrift-world');
        return !!c && c.width>0 && c.height>0;
    }`).Bool() {
		t.Fatal("world canvas was not sized")
	}
	if !page.MustEval(`()=>{
        const note=document.querySelector('.agdrift-disclosure');
        const touch=document.querySelector('.agdrift-touch');
        return !!note && note.textContent.length>20 && touch && touch.getAttribute('aria-hidden')==='true';
    }`).Bool() {
		t.Fatal("catch-up disclosure or touch-control accessibility marker is missing")
	}

	page.MustElement(".agdrift-menu .agdrift-primary").MustClick()
	auraDriftWaitState(t, page, "running", 20*time.Second)
	if !page.MustEval(`()=>document.querySelector('.agdrift-overlay').hidden`).Bool() {
		t.Fatal("race did not start: menu overlay is still visible")
	}

	// Drive with the same event codes the real keyboard produces.
	page.MustEval(`()=>{
        window.dispatchEvent(new KeyboardEvent('keydown',{code:'ArrowUp',bubbles:true}));
        window.dispatchEvent(new KeyboardEvent('keydown',{code:'ArrowRight',bubbles:true}));
    }`)
	auraDriftSleep(page, 1200)
	page.MustEval(`()=>{
        window.dispatchEvent(new KeyboardEvent('keyup',{code:'ArrowUp',bubbles:true}));
        window.dispatchEvent(new KeyboardEvent('keyup',{code:'ArrowRight',bubbles:true}));
        window.dispatchEvent(new KeyboardEvent('keydown',{code:'Space',bubbles:true}));
        window.dispatchEvent(new KeyboardEvent('keyup',{code:'Space',bubbles:true}));
    }`)
	auraDriftSleep(page, 400)

	if !page.MustEval(`()=>!!window.AuraDriftCore && !!window.AuraDriftApp`).Bool() {
		t.Fatal("game core or app shell was not registered")
	}
	stats, raw := auraDriftReadStats(t, page)
	t.Logf("aura drift runtime stats: %s", raw)
	if stats.State != "running" || stats.Time <= 0 || stats.SpeedKmh <= 5 || stats.Paused {
		t.Fatalf("car did not accelerate during the smoke drive: %s", raw)
	}
	if issues := page.MustEval(auraDriftSurfaceJS).Str(); issues != "" {
		t.Fatalf("render surface: %s", issues)
	}
	if dir := auraDriftArtifactDir(t, "race"); dir != "" {
		page.MustScreenshot(filepath.Join(dir, "aura-drift-race.png"))
	}

	auraDriftCloseWindows(page)
	auraDriftNoErrors(t, page)
}

// The keyboard-only journey: focus the start button, press Enter, drive with
// real key events, then pause with Escape. Window blur must pause as well.
func TestDesktopAuraDriftKeyboardJourneyBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	page := auraDriftFixture(t)
	auraDriftReopen(page, map[string]interface{}{"quality": "low"}, "")

	page.MustElement(".agdrift-menu .agdrift-primary").MustFocus()
	page.Keyboard.MustType(input.Enter)
	auraDriftWaitState(t, page, "running", 20*time.Second)

	auraDriftKeyDown(t, page, input.ArrowUp)
	auraDriftSleep(page, 1000)
	auraDriftKeyUp(t, page, input.ArrowUp)
	if stats, raw := auraDriftReadStats(t, page); stats.SpeedKmh <= 5 {
		t.Fatalf("real ArrowUp did not accelerate the car: %s", raw)
	}

	page.Keyboard.MustType(input.Escape)
	auraDriftSleep(page, 150)
	if stats, raw := auraDriftReadStats(t, page); !stats.Paused {
		t.Fatalf("Escape did not pause: %s", raw)
	}
	page.Keyboard.MustType(input.Escape)
	auraDriftSleep(page, 150)
	if stats, raw := auraDriftReadStats(t, page); stats.Paused {
		t.Fatalf("second Escape did not resume: %s", raw)
	}

	// Focus loss pauses the race.
	page.MustEval(`()=>window.dispatchEvent(new Event('blur'))`)
	auraDriftSleep(page, 150)
	if stats, raw := auraDriftReadStats(t, page); !stats.Paused {
		t.Fatalf("window blur did not pause the race: %s", raw)
	}
	auraDriftCloseWindows(page)
	auraDriftNoErrors(t, page)
}

// Cosmetic filters must not mistake the game menu for advertising.
func TestDesktopAuraDriftContentFiltersBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	page := auraDriftFixture(t)
	page.MustEval(`()=>{
        const filters=document.createElement('style');
        // Legacy cosmetic rules from EasyList/AdGuard. These must target the
        // OLD ad-* class names: the app now ships agdrift-* and must survive.
        filters.textContent='.ad-overlay,.ad-panel,.ad-title { display:none !important; }';
        document.head.appendChild(filters);
    }`)
	page.MustEval(`async()=>{await fixtureOpen('aura-drift');}`)
	if dir := auraDriftArtifactDir(t, "content-filters"); dir != "" {
		page.MustScreenshot(filepath.Join(dir, "aura-drift-menu.png"))
	}
	if !page.MustEval(`()=>{
        const menu=document.querySelector('.agdrift-menu');
        return !!menu && menu.getBoundingClientRect().height>0;
    }`).Bool() {
		t.Fatal("content filters hid the Aura Drift menu, leaving an empty window")
	}
	page.MustElement(".agdrift-menu .agdrift-primary").MustClick()
	auraDriftWaitState(t, page, "running", 30*time.Second)
	if !page.MustEval(`()=>document.querySelector('.agdrift-overlay').hidden`).Bool() {
		t.Fatal("race did not start with cosmetic filters active")
	}
	auraDriftCloseWindows(page)
	auraDriftNoErrors(t, page)
}

// Selects the night harbour and verifies the post pass is configured for it,
// with the track's recommended lap count applied.
func TestDesktopAuraDriftNightHarbourBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	page := auraDriftFixture(t)
	auraDriftReopen(page, map[string]interface{}{"quality": "ultra"}, "")
	page.MustElement(`.agdrift-tab[data-tab="track"]`).MustClick()
	page.MustElement(`.agdrift-track-card[data-track="neon-hafen"]`).MustClick()
	if !page.MustEval(`()=>document.querySelector('select.agdrift-laps').value==='2'`).Bool() {
		t.Fatal("selecting a track did not apply its recommended lap count")
	}
	page.MustElement(".agdrift-menu .agdrift-primary").MustClick()
	auraDriftSkipCountdown(t, page)
	auraDriftSleep(page, 800)
	stats, raw := auraDriftReadStats(t, page)
	t.Logf("harbour stats: %s", raw)
	if stats.Quality != "ultra" {
		t.Fatalf("quality = %q, want ultra", stats.Quality)
	}
	if stats.Renderer.Kind == "three" {
		if !stats.Post.Enabled || stats.Post.Biome != "harbour" {
			t.Fatalf("post pass not active on the night harbour: %s", raw)
		}
		if !page.MustEval(`()=>{const gl=document.querySelector('canvas.agdrift-gl');return !!gl && !gl.hidden && gl.width>0;}`).Bool() {
			t.Fatal("the WebGL canvas is not presenting a frame")
		}
	} else {
		t.Logf("renderer fell back to %s; harbour post checks skipped", stats.Renderer.Kind)
	}
	if dir := auraDriftArtifactDir(t, "harbour"); dir != "" {
		page.MustScreenshot(filepath.Join(dir, "aura-drift-harbour.png"))
	}
	auraDriftCloseWindows(page)
	auraDriftNoErrors(t, page)
}

// Exercises all four quality tiers, both themes, two window sizes and reduced
// motion. Either renderer kind is accepted here: the 3D-only checks run only
// when the page really renders in three.js.
func TestDesktopAuraDriftQualityMatrixBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	page := auraDriftFixture(t)
	cases := []struct {
		quality string
		width   int
		height  int
		theme   string
		reduced bool
		post    bool
	}{
		{"low", 1280, 800, "standard", false, false},
		{"medium", 1280, 800, "fruity-light", false, true},
		{"high", 1280, 800, "standard", true, true},
		{"ultra", 1440, 900, "fruity-dark", false, true},
		{"ultra", 900, 600, "standard", false, true},
	}
	dir := auraDriftArtifactDir(t, "matrix")
	if limit := os.Getenv("AURAGO_AURA_DRIFT_MATRIX_LIMIT"); limit != "" {
		var n int
		if _, err := fmt.Sscanf(limit, "%d", &n); err == nil && n > 0 && n < len(cases) {
			cases = cases[:n]
		}
	}

	for i, item := range cases {
		page.MustSetViewport(item.width, item.height, 1, false)
		// A fresh document per case avoids reusing a stale window DOM.
		auraDriftReopen(page, map[string]interface{}{"quality": item.quality, "reducedMotion": item.reduced}, item.theme)

		if !page.MustEval(`()=>{
            const overlay=document.querySelector('.agdrift-overlay');
            const menu=document.querySelector('.agdrift-menu');
            return !!overlay && !overlay.hidden && !!menu && !menu.hidden && overlay.getBoundingClientRect().height>0;
        }`).Bool() {
			t.Fatalf("case %d (%s): menu did not mount", i, item.quality)
		}
		// Keyboard-only journey: every primary control must be focusable.
		page.MustElement(".agdrift-menu .agdrift-primary").MustFocus()
		if !page.MustEval(`()=>document.activeElement && document.activeElement.classList.contains('agdrift-primary')`).Bool() {
			t.Fatalf("case %d: start button is not keyboard focusable", i)
		}
		if !page.MustEval(`()=>{
            const buttons=[...document.querySelectorAll('.agdrift-touch-btn')];
            return buttons.length===8 && buttons.every(b=>parseFloat(getComputedStyle(b).width)>=56);
        }`).Bool() {
			t.Fatalf("case %d: touch controls are not eight buttons of at least 56px", i)
		}

		page.MustElement(".agdrift-menu .agdrift-primary").MustClick()
		auraDriftSkipCountdown(t, page)
		auraDriftSleep(page, 700)
		state, raw := auraDriftReadStats(t, page)
		t.Logf("matrix %d %s %dx%d reduced=%v: %s", i, item.quality, item.width, item.height, item.reduced, raw)

		if state.Quality != item.quality {
			t.Fatalf("case %d: quality=%s, want %s", i, state.Quality, item.quality)
		}
		if state.Renderer.Kind == "three" {
			if state.Post.Enabled != item.post {
				t.Fatalf("case %d: post enabled=%v, want %v at %s", i, state.Post.Enabled, item.post, item.quality)
			}
			budget := page.MustEval(`q=>JSON.stringify(AuraDriftCore.QUALITY[q].gl)`, item.quality).Str()
			var gl struct {
				Calls     float64 `json:"calls"`
				Triangles float64 `json:"triangles"`
			}
			if err := json.Unmarshal([]byte(budget), &gl); err != nil {
				t.Fatal(err)
			}
			if float64(state.Renderer.Calls) > gl.Calls*1.5 || float64(state.Renderer.Triangles) > gl.Triangles*1.5 {
				t.Fatalf("case %d: %s over budget (calls %d/%v, triangles %d/%v)", i, item.quality,
					state.Renderer.Calls, gl.Calls, state.Renderer.Triangles, gl.Triangles)
			}
		}
		if issues := page.MustEval(auraDriftSurfaceJS).Str(); issues != "" {
			t.Fatalf("case %d render surface: %s", i, issues)
		}
		if dir != "" {
			page.MustScreenshot(filepath.Join(dir, fmt.Sprintf("aura-drift-%d-%s-%dx%d.png", i, item.quality, item.width, item.height)))
		}
	}

	auraDriftCloseWindows(page)
	auraDriftNoErrors(t, page)
}

// auraDriftSteer holds the throttle, then one steer key, through real key events
// and checks that the car's aim point moves to the matching side of the screen.
// kind is the renderer the page must be using ("three" or "canvas2d").
func auraDriftSteer(t *testing.T, page *rod.Page, trackID string, key input.Key, wantRight bool, kind string) {
	t.Helper()
	page.MustEval(`async id=>{await AuraDriftApp.debug.start({mode:'time_trial',trackId:id,laps:1});}`, trackID)
	auraDriftWaitState(t, page, "running", 30*time.Second)
	if stats, raw := auraDriftReadStats(t, page); stats.Renderer.Kind != kind {
		t.Fatalf("renderer kind = %q, want %q: %s", stats.Renderer.Kind, kind, raw)
	}
	auraDriftKeyDown(t, page, input.ArrowUp)
	auraDriftSleep(page, 1200)
	page.MustEval(`()=>{window.__ad0=AuraDriftApp.debug.snapshot();}`)
	auraDriftKeyDown(t, page, key)
	auraDriftSleep(page, 500)
	// While still turning: the real three.js camera must agree with AD.projectToScreen.
	var live string
	if kind == "three" {
		live = page.MustEval(`()=>{
            const AD=AuraDriftCore,snap=AuraDriftApp.debug.snapshot(),three=AuraDriftApp.debug.three();
            const proj=AuraDriftApp.stats()[0].renderer.projection;
            const v=new three.T.Vector3(snap.aim[0],snap.aim[1],snap.aim[2]).project(three.camera);
            const p=AD.projectToScreen(snap.pose,snap.aim[0],snap.aim[1],snap.aim[2],snap.aspect);
            return JSON.stringify({noseX:proj.noseX,tailX:proj.tailX,dx:Math.abs(v.x-p.x),dy:Math.abs(v.y-p.y),visible:p.visible});
        }`).Str()
	}
	page.MustEval(`()=>{window.__ad1=AuraDriftApp.debug.snapshot();}`)
	auraDriftKeyUp(t, page, key)
	auraDriftKeyUp(t, page, input.ArrowUp)

	xs := page.MustEval(`kind=>{
        const AD=AuraDriftCore,a=window.__ad0,b=window.__ad1;
        if(a.body.vx<3)return JSON.stringify({slow:true});
        const f=kind==='three'
            ?s=>AD.projectToScreen(a.pose,s.aim[0],s.aim[1],s.aim[2],a.aspect).x
            :s=>AD.project2D(a.pose,s.aim[0],s.aim[2],800,600)[0]/400-1;
        return JSON.stringify({x0:f(a),x1:f(b),psi0:a.psi,psi1:b.psi,steer:b.body.steer});
    }`, kind).Str()
	var res struct {
		Slow  bool    `json:"slow"`
		X0    float64 `json:"x0"`
		X1    float64 `json:"x1"`
		Psi0  float64 `json:"psi0"`
		Psi1  float64 `json:"psi1"`
		Steer float64 `json:"steer"`
	}
	if err := json.Unmarshal([]byte(xs), &res); err != nil {
		t.Fatal(err)
	}
	if res.Slow {
		t.Fatalf("%s: the car did not accelerate before steering", trackID)
	}
	delta := res.X1 - res.X0
	if wantRight && delta <= 0.005 || !wantRight && delta >= -0.005 {
		t.Fatalf("%s %s right=%v: projected aim x moved %.4f (%s)", trackID, kind, wantRight, delta, xs)
	}
	if kind == "three" {
		var l struct {
			NoseX   float64 `json:"noseX"`
			TailX   float64 `json:"tailX"`
			DX      float64 `json:"dx"`
			DY      float64 `json:"dy"`
			Visible bool    `json:"visible"`
		}
		if err := json.Unmarshal([]byte(live), &l); err != nil {
			t.Fatal(err)
		}
		if wantRight && l.NoseX <= l.TailX || !wantRight && l.NoseX >= l.TailX {
			t.Fatalf("%s: nose/tail on the real camera mirror the steering (right=%v): %s", trackID, wantRight, live)
		}
		if l.DX > 0.02 || l.DY > 0.02 {
			t.Fatalf("%s: AD.projectToScreen differs from the three.js camera: %s", trackID, live)
		}
	}
}

// Holding ArrowRight through real key events moves the car right on screen, in
// the 3D scene and in the Canvas 2D fallback, on two tracks each (and the mirror).
func TestDesktopAuraDriftSteeringBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	tracks := []string{"alpenpass", "wuesten-canyon"}

	t.Run("three", func(t *testing.T) {
		page := auraDriftFixture(t)
		if !auraDriftHasWebGL(page) {
			t.Skip("this browser has no WebGL2")
		}
		auraDriftReopen(page, map[string]interface{}{"quality": "medium"}, "")
		for _, track := range tracks {
			auraDriftSteer(t, page, track, input.ArrowRight, true, "three")
			auraDriftSteer(t, page, track, input.ArrowLeft, false, "three")
		}
		auraDriftCloseWindows(page)
		auraDriftNoErrors(t, page)
	})

	t.Run("canvas2d", func(t *testing.T) {
		page := auraDriftFixtureWithout(t, "aura-drift-render3d.js")
		page.MustEval(`async()=>{await fixtureOpen('aura-drift');}`)
		for _, track := range tracks {
			auraDriftSteer(t, page, track, input.ArrowRight, true, "canvas2d")
			auraDriftSteer(t, page, track, input.ArrowLeft, false, "canvas2d")
		}
		auraDriftCloseWindows(page)
		auraDriftNoErrors(t, page)
	})
}

// A short run of every mode through debug.fastForward: each reaches the results
// panel without a single page error.
func TestDesktopAuraDriftModesSmokeBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	page := auraDriftFixture(t)
	auraDriftReopen(page, map[string]interface{}{"quality": "low"}, "")
	for _, mode := range []string{"quick", "time_trial", "cup", "daily", "drift_attack", "elimination", "hot_potato"} {
		mode := mode
		t.Run(mode, func(t *testing.T) {
			page.MustEval(`async m=>{
                await AuraDriftApp.debug.start({mode:m,trackId:'alpenpass',laps:1});
                AuraDriftApp.debug.autopilot(true,{skill:1,ramps:true});
            }`, mode)
			finished := false
			for i := 0; i < 24 && !finished; i++ {
				finished = page.MustEval(`()=>{
                    const r=AuraDriftApp.debug.fastForward(30,{renderEvery:600});
                    return !!r && (r.state==='finished'||r.results);
                }`).Bool()
			}
			if !finished {
				t.Fatalf("%s: no result after 720 simulated seconds", mode)
			}
			deadline := time.Now().Add(15 * time.Second)
			for !page.MustEval(`()=>!!AuraDriftApp.debug.view().results`).Bool() {
				if time.Now().After(deadline) {
					t.Fatalf("%s: the results panel never appeared", mode)
				}
				time.Sleep(100 * time.Millisecond)
			}
			page.MustEval(`()=>AuraDriftApp.debug.autopilot(false)`)
			auraDriftNoErrors(t, page)
		})
	}
	auraDriftCloseWindows(page)
}

// Reduced motion shortens the finish-to-results delay to well under a second
// and keeps the camera shake at zero.
func TestDesktopAuraDriftReducedMotionBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	page := auraDriftFixture(t)
	auraDriftReopen(page, map[string]interface{}{"quality": "low", "reducedMotion": true}, "")
	page.MustEval(`async()=>{
        await AuraDriftApp.debug.start({mode:'quick',trackId:'alpenpass',laps:1,opponents:1});
        AuraDriftApp.debug.autopilot(true,{skill:1,ramps:false});
    }`)
	for i := 0; i < 24; i++ {
		if page.MustEval(`()=>{const r=AuraDriftApp.debug.fastForward(30,{renderEvery:600});return !!r&&r.state==='finished';}`).Bool() {
			break
		}
	}
	start := time.Now()
	for !page.MustEval(`()=>!!AuraDriftApp.debug.view().results`).Bool() {
		if time.Since(start) > 6*time.Second {
			t.Fatal("results did not appear under reduced motion")
		}
		time.Sleep(50 * time.Millisecond)
	}
	if waited := time.Since(start); waited > 1500*time.Millisecond {
		t.Fatalf("reduced motion should show results in about 0.4 s, took %s", waited)
	}
	auraDriftCloseWindows(page)
	auraDriftNoErrors(t, page)
}

// A stale service-worker cache can serve an older module-loader.js. The game
// must degrade to Canvas 2D when an optional 3D module is missing, and must
// show a visible, actionable error when a required module is missing.
func TestDesktopAuraDriftStaleLoaderBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)

	degraded := func(t *testing.T, page *rod.Page, steering bool) {
		t.Helper()
		page.MustEval(`async()=>{await fixtureOpen('aura-drift');}`)
		if !page.MustEval(`()=>{
            const o=document.querySelector('.agdrift-overlay');
            return !!o && !o.hidden && o.getBoundingClientRect().height>0;
        }`).Bool() {
			t.Fatal("stale loader without the 3D module left an empty window")
		}
		if page.MustEval(`()=>!!document.querySelector('.agdrift-load-error')`).Bool() {
			t.Fatal("an optional 3D module must not trigger the load-error panel")
		}
		page.MustElement(".agdrift-menu .agdrift-primary").MustClick()
		auraDriftWaitState(t, page, "running", 30*time.Second)
		auraDriftKeyDown(t, page, input.ArrowUp)
		auraDriftSleep(page, 1100)
		auraDriftKeyUp(t, page, input.ArrowUp)
		stats, raw := auraDriftReadStats(t, page)
		t.Logf("degraded stats: %s", raw)
		if stats.Renderer.Kind != "canvas2d" || !strings.Contains(raw, `"degraded":true`) {
			t.Fatalf("the 2D fallback was not reported: %s", raw)
		}
		if stats.State != "running" || stats.SpeedKmh <= 5 {
			t.Fatalf("game was not playable without the 3D module: %s", raw)
		}
		if steering {
			auraDriftSteer(t, page, "alpenpass", input.ArrowRight, true, "canvas2d")
		}
		auraDriftCloseWindows(page)
		auraDriftNoErrors(t, page)
	}

	t.Run("optional_webgl2_module_missing", func(t *testing.T) {
		degraded(t, auraDriftFixtureWithout(t, "aura-drift-render3d.js"), true)
	})

	t.Run("runtime_missing", func(t *testing.T) {
		degraded(t, auraDriftFixtureWithout(t, "/js/vendor/aura-drift/three-runtime.js"), false)
	})

	t.Run("required_module_missing", func(t *testing.T) {
		page := auraDriftFixtureWithout(t, "aura-drift-core.js")
		page.MustEval(`async()=>{await fixtureOpen('aura-drift');}`)
		panel := page.MustEval(`()=>{
            const p=document.querySelector('.agdrift-load-error');
            return p ? JSON.stringify({text:p.textContent, visible:p.getBoundingClientRect().height>0}) : 'null';
        }`).Str()
		if panel == "null" {
			t.Fatal("missing core module produced a silent empty window")
		}
		if !strings.Contains(panel, "aura-drift-core.js") || !strings.Contains(panel, `"visible":true`) {
			t.Fatalf("load-error panel lacks the missing module or is invisible: %s", panel)
		}
		if !page.MustEval(`()=>!!document.querySelector('.agdrift-load-error .agdrift-primary')`).Bool() {
			t.Fatal("load-error panel has no reload action")
		}
		auraDriftCloseWindows(page)
	})
}

// A GLB that 404s is replaced by a stand-in; the race still runs.
func TestDesktopAuraDriftGLBFallbackBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	page := auraDriftFixtureFailing(t, "/3d/aura-drift/v1/car-vektor.glb")
	if !auraDriftHasWebGL(page) {
		t.Skip("this browser has no WebGL2")
	}
	auraDriftReopen(page, map[string]interface{}{"quality": "medium"}, "")
	page.MustEval(`async()=>{await AuraDriftApp.debug.start({mode:'quick',trackId:'alpenpass',laps:1,carId:'vektor'});}`)
	auraDriftWaitState(t, page, "running", 30*time.Second)
	auraDriftSleep(page, 800)
	stats, raw := auraDriftReadStats(t, page)
	t.Logf("glb fallback stats: %s", raw)
	if stats.Renderer.Kind != "three" || stats.Renderer.StandIns < 1 {
		t.Fatalf("expected a three.js stand-in for the missing car: %s", raw)
	}
	auraDriftCloseWindows(page)
	auraDriftNoErrors(t, page)
}

// Opening and closing the app twice must not grow the GPU resource count.
// Either renderer kind passes; geometry counts only mean something in 3D.
func TestDesktopAuraDriftDisposeBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	page := auraDriftFixture(t)
	auraDriftReopen(page, map[string]interface{}{"quality": "low"}, "")
	run := func() auraDriftStats {
		page.MustEval(`async()=>{await AuraDriftApp.debug.start({mode:'time_trial',trackId:'alpenpass',laps:1});}`)
		auraDriftWaitState(t, page, "running", 30*time.Second)
		auraDriftSleep(page, 600)
		stats, _ := auraDriftReadStats(t, page)
		auraDriftCloseWindows(page)
		return stats
	}
	first := run()
	page.MustEval(`async()=>{await fixtureOpen('aura-drift');}`)
	second := run()
	t.Logf("geometries: first %d, second %d (%s)", first.Renderer.Geometries, second.Renderer.Geometries, second.Renderer.Kind)
	if second.Renderer.Geometries > first.Renderer.Geometries+8 {
		t.Fatalf("geometries grew from %d to %d across a reopen", first.Renderer.Geometries, second.Renderer.Geometries)
	}
	if page.MustEval(`()=>window.AuraDriftApp.stats().length`).Int() != 0 {
		t.Fatal("a closed window left a live Aura Drift instance")
	}
	auraDriftNoErrors(t, page)
}

// Regression for the actual field failure: AdGuard/EasyList hide elements whose
// classes start with "ad-" using display:none !important. The original racer
// used ad-overlay/ad-panel/ad-menu, so a perfectly running game was invisible
// with no console error. agdrift- cannot match those cosmetic filters.
func TestDesktopAuraDriftAdBlockerBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	page := auraDriftFixture(t)
	page.MustEval(`()=>{
        const style=document.createElement('style');
        style.id='agdrift-blocker-rule';
        style.textContent='[class^="ad-"],[class*=" ad-"],.ad-overlay,.ad-panel,.ad-menu{display:none !important}';
        document.head.appendChild(style);
        const probe=document.createElement('div');
        probe.id='agdrift-ad-probe';
        probe.className='ad-probe';
        probe.style.cssText='position:fixed;width:10px;height:10px';
        document.body.appendChild(probe);
    }`)
	if !page.MustEval(`()=>getComputedStyle(document.getElementById('agdrift-ad-probe')).display==='none'`).Bool() {
		t.Fatal("simulated ad blocker rule is not active; the regression test is invalid")
	}
	page.MustEval(`async()=>{await fixtureOpen('aura-drift');}`)
	if !page.MustEval(`()=>{
        const root=document.querySelector('.agdrift-root');
        const overlay=document.querySelector('.agdrift-overlay');
        if(!root||!overlay)return false;
        if(getComputedStyle(overlay).display==='none')return false;
        if(overlay.getBoundingClientRect().height<=0)return false;
        const menu=document.querySelector('.agdrift-menu');
        if(!menu||menu.hidden||getComputedStyle(menu).display==='none')return false;
        return [...root.querySelectorAll('*')].every(node=>{
            if(typeof node.className!=='string')return true;
            return !/(^|\s)ad-/.test(node.className);
        });
    }`).Bool() {
		t.Fatal("ad blocker simulation hid the Aura Drift UI or legacy ad- classes remain")
	}
	page.MustElement(".agdrift-menu .agdrift-primary").MustClick()
	auraDriftWaitState(t, page, "running", 30*time.Second)
	if !page.MustEval(`()=>document.querySelector('.agdrift-overlay').hidden`).Bool() {
		t.Fatal("race did not start with the ad blocker rule active")
	}
	auraDriftCloseWindows(page)
	auraDriftNoErrors(t, page)
}
