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
	"github.com/go-rod/rod/lib/proto"
)

// Maximized windows must end above the taskbar or dock at every width on
// fine-pointer devices; the touch phone layout keeps its own reserve.
func TestDesktopMaximizedWindowClearsTaskbarBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	html := regexp.MustCompile(`(?s)<script\b[^>]*>.*?</script>`).ReplaceAllString(readDesktopAssetText(t, "desktop.html"), "")
	html = regexp.MustCompile(`\{\{[^}]*\}\}`).ReplaceAllString(html, "")
	html = strings.Replace(html, "</body>", `<script src="/js/shared/lazy-assets.js"></script><script src="/js/desktop/core/module-loader.js"></script><script src="/maximize-shell.js"></script><script src="/testdata/aurora-fixture.js"></script></body>`, 1)
	shell := readDesktopAssetText(t, "js/desktop/bundles/main.bundle.js")
	cut := strings.LastIndex(shell, "    ensureDesktopRadialMenuAnchor();")
	if cut < 0 {
		t.Fatal("desktop startup seam missing")
	}
	shell = shell[:cut] + `window.aurora={state,loadIconManifest,applyDesktopSettings,renderIcons,renderStartApps,renderStartButtonIcon,wireShellChromeControls,bindViewportMetrics,handleDesktopKeydown,closeContextMenu,openApp,closeWindow,toggleMaximizeWindow};})();`
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(Content)))
	mux.HandleFunc("/fixture", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, html) })
	mux.HandleFunc("/maximize-shell.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/javascript")
		fmt.Fprint(w, shell)
	})
	mux.HandleFunc("/fixture-words", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, readDesktopAssetText(t, "lang/desktop/de.json"))
	})
	mux.HandleFunc("/fixture-apps", func(w http.ResponseWriter, r *http.Request) { json.NewEncoder(w).Encode(desktop.BuiltinApps()) })
	server := httptest.NewServer(mux)
	defer server.Close()
	page := newSmokeBrowser(t).MustPage(server.URL + "/fixture").Timeout(90 * time.Second)
	defer page.Close()
	page.MustWaitLoad()
	page.MustEval(`async()=>{await fixtureReady;await AuraDesktopModules.loadAppAssets('settings');}`)

	// measure maximizes a fresh Settings window and reports its bottom edge
	// against the top of the visible bottom taskbar (Standard) or dock
	// (Fruity). Wide Fruity layouts move the status bar into the top menubar.
	const measure = `async([theme,density])=>{
        [...aurora.state.windows.keys()].forEach(id=>aurora.closeWindow(id));
        aurora.state.bootstrap.settings['appearance.density']=density;
        fixtureTheme(theme);
        await fixtureOpen('settings');
        const item=[...aurora.state.windows.values()].at(-1);
        if(!item.element.classList.contains('maximized'))aurora.toggleMaximizeWindow(item.id);
        await new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r)));
        const bar=[...document.querySelectorAll('.vd-taskbar,.vd-taskbar-apps,.vd-taskbar-system')]
            .map(e=>e.getBoundingClientRect()).filter(r=>r.width>0&&r.height>0&&r.bottom>innerHeight/2);
        const win=item.element.getBoundingClientRect();
        return {
            winTop:win.top,winBottom:win.bottom,winHeight:win.height,
            layerTop:document.getElementById('vd-window-layer').getBoundingClientRect().top,
            barTop:Math.min(...bar.map(r=>r.top)),
            viewport:innerHeight,visual:Math.round(visualViewport.height),
            collapsed:document.body.classList.contains('fruity-dock-collapsed'),
            forced:item.element.classList.contains('vd-mobile-forced-maximized'),
            height:getComputedStyle(item.element).height
        };
    }`
	artifacts := os.Getenv("AURAGO_BROWSER_ARTIFACT_DIR")
	if artifacts != "" {
		if err := os.MkdirAll(artifacts, 0755); err != nil {
			t.Fatal(err)
		}
	}
	type geometry struct {
		WinTop, WinBottom, WinHeight float64
		LayerTop, BarTop             float64
		Viewport, Visual             float64
		Collapsed, Forced            bool
		Height                       string
	}
	run := func(theme, density string) geometry {
		t.Helper()
		var g geometry
		if err := page.MustEval(measure, []string{theme, density}).Unmarshal(&g); err != nil {
			t.Fatal(err)
		}
		return g
	}

	for _, size := range [][2]int{{1366, 900}, {821, 900}, {820, 900}, {640, 900}} {
		page.MustSetViewport(size[0], size[1], 1, false)
		for _, theme := range []string{"standard", "fruity-light", "fruity-dark"} {
			for _, density := range []string{"comfortable", "compact"} {
				g := run(theme, density)
				label := fmt.Sprintf("%s/%s at %dx%d", theme, density, size[0], size[1])
				t.Logf("%s: %+v", label, g)
				if artifacts != "" {
					page.MustScreenshot(filepath.Join(artifacts, fmt.Sprintf("maximized-%s-%s-%dx%d.png", theme, density, size[0], size[1])))
				}
				if g.Forced {
					t.Fatalf("%s: a fine-pointer viewport must not use the touch window layout", label)
				}
				if g.WinTop != g.LayerTop {
					t.Errorf("%s: maximized window must start at the workspace top %.1f, got %.1f", label, g.LayerTop, g.WinTop)
				}
				if g.WinBottom > g.BarTop+0.5 {
					t.Errorf("%s: maximized window bottom %.1f reaches under the taskbar top %.1f", label, g.WinBottom, g.BarTop)
				}
				if gap := g.BarTop - g.WinBottom; gap > 16 {
					t.Errorf("%s: maximized window leaves %.1fpx unused above the taskbar", label, gap)
				}
				if g.Collapsed {
					t.Errorf("%s: a maximized window that clears the dock must not collapse it", label)
				}
			}
		}
	}

	// Phone layout: touch input keeps the forced single-window layout and its
	// own taskbar reserve below the visual viewport height.
	page.MustSetViewport(390, 844, 1, true)
	if err := (proto.EmulationSetTouchEmulationEnabled{Enabled: true}).Call(page); err != nil {
		t.Fatal(err)
	}
	for _, theme := range []string{"standard", "fruity-light"} {
		g := run(theme, "comfortable")
		t.Logf("%s touch at 390x844: %+v", theme, g)
		if artifacts != "" {
			page.MustScreenshot(filepath.Join(artifacts, fmt.Sprintf("maximized-%s-touch-390x844.png", theme)))
		}
		reserve := 64.0
		if strings.HasPrefix(theme, "fruity") {
			reserve = 116
		}
		if !g.Forced {
			t.Errorf("%s touch: phone layout must force a maximized single window", theme)
		}
		if want := g.Visual - reserve; g.WinTop != 0 || g.WinHeight < want-0.5 || g.WinHeight > want+0.5 {
			t.Errorf("%s touch: phone window must fill the visual viewport minus its %.0fpx reserve (%.1f), got top %.1f height %.1f", theme, reserve, want, g.WinTop, g.WinHeight)
		}
	}
	(proto.EmulationSetTouchEmulationEnabled{Enabled: false}).Call(page)
	page.MustSetViewport(1366, 900, 1, false)
	if errors := page.MustEval(`()=>JSON.stringify(fixtureErrors)`).Str(); errors != "[]" {
		t.Fatal(errors)
	}
}
