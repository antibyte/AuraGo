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
			if strings.Contains(line, "'/js/desktop/apps/"+script+"'") {
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

// Opens Aura Drift in the real Desktop shell, starts a race, drives with
// synthetic key events and verifies that no runtime error escapes.
func TestDesktopAuraDriftBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	page := auraDriftFixture(t)
	page.MustEval(`async()=>{await fixtureOpen('aura-drift');}`)

	if !page.MustEval(`()=>!!document.querySelector('.agdrift-root .agdrift-menu:not([hidden])')`).Bool() {
		t.Fatal("Aura Drift menu did not mount in the desktop window")
	}
	if !page.MustEval(`()=>{const c=document.querySelector('.agdrift-world');return !!c && c.width>0 && c.height>0;}`).Bool() {
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
	page.MustEval(`()=>new Promise(resolve=>setTimeout(resolve,3600))`)
	if !page.MustEval(`()=>document.querySelector('.agdrift-overlay').hidden`).Bool() {
		t.Fatal("race did not start: menu overlay is still visible")
	}

	// Drive with the same event codes the real keyboard produces.
	page.MustEval(`()=>{
        window.dispatchEvent(new KeyboardEvent('keydown',{code:'ArrowUp',bubbles:true}));
        window.dispatchEvent(new KeyboardEvent('keydown',{code:'ArrowRight',bubbles:true}));
    }`)
	page.MustEval(`()=>new Promise(resolve=>setTimeout(resolve,1200))`)
	page.MustEval(`()=>{
        window.dispatchEvent(new KeyboardEvent('keyup',{code:'ArrowUp',bubbles:true}));
        window.dispatchEvent(new KeyboardEvent('keyup',{code:'ArrowRight',bubbles:true}));
        window.dispatchEvent(new KeyboardEvent('keydown',{code:'Space',bubbles:true}));
        window.dispatchEvent(new KeyboardEvent('keyup',{code:'Space',bubbles:true}));
    }`)
	page.MustEval(`()=>new Promise(resolve=>setTimeout(resolve,400))`)

	if !page.MustEval(`()=>!!window.AuraDriftCore && !!window.AuraDriftApp`).Bool() {
		t.Fatal("game core or app shell was not registered")
	}
	stats := page.MustEval(`()=>JSON.stringify(window.AuraDriftApp.stats()[0]||{})`).Str()
	t.Logf("aura drift runtime stats: %s", stats)
	if !strings.Contains(stats, `"state":"running"`) {
		t.Fatalf("race did not reach the running state: %s", stats)
	}
	if !page.MustEval(`()=>{
        const s=window.AuraDriftApp.stats()[0];
        return s && s.time>0 && s.speedKmh>5 && !s.paused;
    }`).Bool() {
		t.Fatalf("car did not accelerate during the smoke drive: %s", stats)
	}
	if issues := page.MustEval(`()=>{
        const root=document.querySelector('.agdrift-root'),issues=[];
        if(!root)return 'app root disappeared';
        const r=root.getBoundingClientRect();
        if(r.width<200||r.height<200)issues.push('window too small');
        const world=document.querySelector('.agdrift-world');
        const gl=document.querySelector('.agdrift-gl');
        const hud=document.querySelector('.agdrift-hud');
        if(!hud||hud.width!==Math.round(r.width*Math.min(2,devicePixelRatio||1)))issues.push('hud canvas width mismatch');
        if(document.querySelector('.agdrift-post'))issues.push('old post canvas still present');
        if(gl&&!gl.hidden&&gl.width<=0)issues.push('gl canvas empty');
        return issues.join('; ');
    }`).Str(); issues != "" {
		t.Fatalf("render surface: %s", issues)
	}
	if dir := auraDriftArtifactDir(t, "race"); dir != "" {
		page.MustScreenshot(filepath.Join(dir, "aura-drift-race.png"))
	}

	auraDriftCloseWindows(page)
	if errors := page.MustEval(`()=>JSON.stringify(fixtureErrors)`).Str(); errors != "[]" {
		t.Fatal(errors)
	}
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
	if !page.MustEval(`()=>document.querySelector('.agdrift-overlay').hidden && AuraDriftApp.stats()[0].state==='countdown'`).Bool() {
		t.Fatal("race did not start with cosmetic filters active")
	}
	auraDriftCloseWindows(page)
	if errors := page.MustEval(`()=>JSON.stringify(fixtureErrors)`).Str(); errors != "[]" {
		t.Fatal(errors)
	}
}

// Selects the night harbour and verifies that the WebGL2 wet-reflection pass
// is active there, with the track's recommended lap count applied.
func TestDesktopAuraDriftWetReflectionsBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	page := auraDriftFixture(t)
	page.MustEval(`()=>{
        localStorage.setItem('aurago.desktop.aura-drift.v1', JSON.stringify({
            quality:'ultra', volume:0, muted:true, reducedMotion:false,
            weapons:true, showGhost:false}));
    }`)
	page.MustReload()
	page.MustWaitLoad()
	page.MustEval(`async()=>{await fixtureReady;await fixtureOpen('aura-drift');}`)
	page.MustEval(`()=>{
        const track=document.querySelector('.agdrift-menu select');
        track.value='neon-hafen';
        track.dispatchEvent(new Event('change',{bubbles:true}));
    }`)
	if !page.MustEval(`()=>document.querySelectorAll('.agdrift-menu select')[1].value==='2'`).Bool() {
		t.Fatal("selecting a track did not apply its recommended lap count")
	}
	page.MustElement(".agdrift-menu .agdrift-primary").MustClick()
	page.MustEval(`()=>new Promise(resolve=>setTimeout(resolve,1400))`)
	stats := page.MustEval(`()=>JSON.stringify(window.AuraDriftApp.stats()[0]||{})`).Str()
	t.Logf("harbour stats: %s", stats)
	if !strings.Contains(stats, `"biome":"harbour"`) || !strings.Contains(stats, `"enabled":true`) {
		t.Fatalf("wet-reflection pass not active on the night harbour: %s", stats)
	}
	if !page.MustEval(`()=>{
        const gl=document.querySelector('.agdrift-gl');
        return !!gl && !gl.hidden && gl.width>0 && !document.querySelector('.agdrift-post');
    }`).Bool() {
		t.Fatal("the WebGL canvas is not presenting a frame")
	}
	if dir := auraDriftArtifactDir(t, "harbour"); dir != "" {
		page.MustScreenshot(filepath.Join(dir, "aura-drift-harbour.png"))
	}
	auraDriftCloseWindows(page)
	if errors := page.MustEval(`()=>JSON.stringify(fixtureErrors)`).Str(); errors != "[]" {
		t.Fatal(errors)
	}
}

// Exercises all four quality tiers, both themes, two window sizes and reduced
// motion. Verifies the WebGL2 pass is gated by tier, cache caps hold, keyboard
// focus works, touch targets meet the product minimum and nothing errors.
func TestDesktopAuraDriftQualityMatrixBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	page := auraDriftFixture(t)
	cases := []struct {
		quality  string
		width    int
		height   int
		theme    string
		reduced  bool
		cacheCap int
		post     bool
	}{
		{"low", 1280, 800, "standard", false, 24, false},
		{"medium", 1280, 800, "fruity-light", false, 40, false},
		{"high", 1280, 800, "standard", true, 64, true},
		{"ultra", 1440, 900, "fruity-dark", false, 96, true},
		{"ultra", 900, 600, "standard", false, 96, true},
	}
	dir := auraDriftArtifactDir(t, "matrix")
	if limit := os.Getenv("AURAGO_AURA_DRIFT_MATRIX_LIMIT"); limit != "" {
		var n int
		if _, err := fmt.Sscanf(limit, "%d", &n); err == nil && n > 0 && n < len(cases) {
			cases = cases[:n]
		}
	}

	for i, item := range cases {
		page.MustEval(`(cfg)=>{
            localStorage.setItem('aurago.desktop.aura-drift.v1', JSON.stringify(cfg));
        }`, map[string]interface{}{
			"quality": item.quality, "volume": 0, "muted": true,
			"reducedMotion": item.reduced, "weapons": true, "showGhost": false,
		})
		page.MustSetViewport(item.width, item.height, 1, false)
		// A fresh document per case avoids reusing a stale window DOM.
		page.MustReload()
		page.MustWaitLoad()
		page.MustEval(`async()=>{await fixtureReady;}`)
		page.MustEval(`async theme=>{fixtureTheme(theme);await fixtureOpen('aura-drift');}`, item.theme)

		mounted := page.MustEval(`()=>{
            const overlay=document.querySelector('.agdrift-overlay');
            const menu=document.querySelector('.agdrift-menu');
            const rect=overlay?overlay.getBoundingClientRect():null;
            return JSON.stringify({overlay:!!overlay,hidden:overlay?overlay.hidden:null,
                menu:!!menu,menuHidden:menu?menu.hidden:null,height:rect?rect.height:0,
                windows:aurora.state.windows.size});
        }`).Str()
		if !page.MustEval(`()=>{
            const overlay=document.querySelector('.agdrift-overlay');
            const menu=document.querySelector('.agdrift-menu');
            return !!overlay && !overlay.hidden && !!menu && !menu.hidden && overlay.getBoundingClientRect().height>0;
        }`).Bool() {
			t.Fatalf("case %d (%s): menu did not mount: %s", i, item.quality, mounted)
		}
		// Keyboard-only journey: every primary control must be focusable.
		page.MustElement(".agdrift-menu .agdrift-primary").MustFocus()
		if !page.MustEval(`()=>document.activeElement && document.activeElement.classList.contains('agdrift-primary')`).Bool() {
			t.Fatalf("case %d: start button is not keyboard focusable", i)
		}
		if !page.MustEval(`()=>{
            const buttons=[...document.querySelectorAll('.agdrift-touch-btn')];
            return buttons.length===6 && buttons.every(b=>parseFloat(getComputedStyle(b).width)>=44);
        }`).Bool() {
			t.Fatalf("case %d: touch targets are below the 44px product minimum", i)
		}

		page.MustElement(".agdrift-menu .agdrift-primary").MustClick()
		page.MustEval(`()=>new Promise(resolve=>setTimeout(resolve,1400))`)
		result := page.MustEval(`()=>JSON.stringify(window.AuraDriftApp.stats()[0]||{})`).Str()
		t.Logf("matrix %d %s %dx%d reduced=%v: %s", i, item.quality, item.width, item.height, item.reduced, result)

		var state struct {
			Quality  string `json:"quality"`
			Renderer struct {
				Chunks int `json:"chunks"`
			} `json:"renderer"`
			Post struct {
				Available bool `json:"available"`
				Enabled   bool `json:"enabled"`
			} `json:"post"`
		}
		if err := json.Unmarshal([]byte(result), &state); err != nil {
			t.Fatalf("case %d: bad stats JSON: %v", i, err)
		}
		if state.Quality != item.quality {
			t.Fatalf("case %d: quality=%s, want %s", i, state.Quality, item.quality)
		}
		if state.Renderer.Chunks > item.cacheCap+2 {
			t.Fatalf("case %d: %s chunk cache exceeded cap %d: %d", i, item.quality, item.cacheCap, state.Renderer.Chunks)
		}
		if item.post {
			if state.Post.Available && !state.Post.Enabled {
				t.Fatalf("case %d: WebGL2 available but post-processing disabled", i)
			}
		} else if state.Post.Enabled {
			t.Fatalf("case %d: post-processing must stay off below the high tier", i)
		}
		if issues := page.MustEval(`()=>{
            const root=document.querySelector('.agdrift-root'),issues=[];
            if(!root)return 'app root missing';
            const r=root.getBoundingClientRect();
            const world=document.querySelector('.agdrift-world');
            const expected=Math.round(r.width*Math.min(2,devicePixelRatio||1));
            if(world.width!==expected)issues.push('canvas '+world.width+' != '+expected);
            if(document.querySelector('.agdrift-hud').width!==world.width)issues.push('hud mismatch');
            return issues.join('; ');
        }`).Str(); issues != "" {
			t.Fatalf("case %d render surface: %s", i, issues)
		}
		if dir != "" {
			page.MustScreenshot(filepath.Join(dir, fmt.Sprintf("aura-drift-%d-%s-%dx%d.png", i, item.quality, item.width, item.height)))
		}
	}

	auraDriftCloseWindows(page)
	if errors := page.MustEval(`()=>JSON.stringify(fixtureErrors)`).Str(); errors != "[]" {
		t.Fatal(errors)
	}
}

// A stale service-worker cache can serve an older module-loader.js. The game
// must degrade to Canvas 2D when the optional WebGL2 module is missing, and
// must show a visible, actionable error when a required module is missing.
func TestDesktopAuraDriftStaleLoaderBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)

	t.Run("optional_webgl2_module_missing", func(t *testing.T) {
		page := auraDriftFixtureWithout(t, "aura-drift-post.js")
		page.MustEval(`async()=>{await fixtureOpen('aura-drift');}`)
		if !page.MustEval(`()=>{
            const o=document.querySelector('.agdrift-overlay');
            return !!o && !o.hidden && o.getBoundingClientRect().height>0;
        }`).Bool() {
			t.Fatal("stale loader without WebGL2 module left an empty window")
		}
		if page.MustEval(`()=>!!document.querySelector('.agdrift-load-error')`).Bool() {
			t.Fatal("optional WebGL2 module must not trigger the load-error panel")
		}
		page.MustElement(".agdrift-menu .agdrift-primary").MustClick()
		page.MustEval(`()=>new Promise(resolve=>setTimeout(resolve,3600))`)
		page.MustEval(`()=>window.dispatchEvent(new KeyboardEvent('keydown',{code:'ArrowUp',bubbles:true}))`)
		page.MustEval(`()=>new Promise(resolve=>setTimeout(resolve,1100))`)
		stats := page.MustEval(`()=>JSON.stringify(window.AuraDriftApp.stats()[0]||{})`).Str()
		t.Logf("degraded stats: %s", stats)
		if !strings.Contains(stats, `"degraded":true`) {
			t.Fatalf("WebGL2 fallback was not reported: %s", stats)
		}
		if !page.MustEval(`()=>{
            const s=window.AuraDriftApp.stats()[0];
            return s && s.state==='running' && s.speedKmh>5;
        }`).Bool() {
			t.Fatalf("game was not playable without the WebGL2 module: %s", stats)
		}
		auraDriftCloseWindows(page)
		if errors := page.MustEval(`()=>JSON.stringify(fixtureErrors)`).Str(); errors != "[]" {
			t.Fatal(errors)
		}
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
	page.MustEval(`()=>new Promise(resolve=>setTimeout(resolve,3600))`)
	if !page.MustEval(`()=>document.querySelector('.agdrift-overlay').hidden`).Bool() {
		t.Fatal("race did not start with the ad blocker rule active")
	}
	auraDriftCloseWindows(page)
	if errors := page.MustEval(`()=>JSON.stringify(fixtureErrors)`).Str(); errors != "[]" {
		t.Fatal(errors)
	}
}
