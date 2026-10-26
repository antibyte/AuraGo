package ui

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"aurago/internal/desktop"
	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
)

// This opt-in check compares Standard and Fruity against the committed Desktop
// bundles before a theme change. It intentionally depends on git and a browser,
// so normal CI does not run it.
//
//	AURAGO_RUN_DESKTOP_THEME_NEUTRALITY=1 go test ./ui -run TestDesktopThemeNeutralityAgainstHeadBrowser -count=1
func TestDesktopThemeNeutralityAgainstHeadBrowser(t *testing.T) {
	if os.Getenv("AURAGO_RUN_DESKTOP_THEME_NEUTRALITY") != "1" {
		t.Skip("set AURAGO_RUN_DESKTOP_THEME_NEUTRALITY=1 to compare Standard and Fruity with HEAD")
	}
	requirePrecisionBrowserSmoke(t)

	headShell := desktopThemeNeutralityGitShow(t, "HEAD:ui/js/desktop/bundles/main.bundle.js")
	headCSS := desktopThemeNeutralityGitShow(t, "HEAD:ui/css/desktop-shell.bundle.css")
	currentShell := readDesktopAssetText(t, "js/desktop/bundles/main.bundle.js")
	currentCSS := readDesktopAssetText(t, "css/desktop-shell.bundle.css")
	for label, shell := range map[string]string{"HEAD": headShell, "current": currentShell} {
		spliced, err := themeSweepSpliceSeam(shell)
		if err != nil {
			t.Fatalf("expose %s shell test seam: %v", label, err)
		}
		if label == "HEAD" {
			headShell = spliced
		} else {
			currentShell = spliced
		}
	}

	html := regexp.MustCompile(`(?s)<script\b[^>]*>.*?</script>`).ReplaceAllString(readDesktopAssetText(t, "desktop.html"), "")
	html = regexp.MustCompile(`\{\{[^}]*\}\}`).ReplaceAllString(html, "")
	words := readDesktopAssetText(t, "lang/desktop/de.json")
	fixedDate := `<script>(()=>{const NativeDate=Date;const fixed=Date.UTC(2025,0,15,12,34,0);window.Date=class extends NativeDate{constructor(...args){if(args.length)super(...args);else super(fixed)}static now(){return fixed}}})();window.fixtureWords=` + words + `;window.t=(k,v)=>{let s=fixtureWords[k]||k;for(const [key,value] of Object.entries(v||{}))s=s.replaceAll('{{'+key+'}}',value).replaceAll('{'+key+'}',value);return s;};window.fixtureErrors=[];addEventListener('error',e=>fixtureErrors.push(e.message));addEventListener('unhandledrejection',e=>fixtureErrors.push(String(e.reason)));window.WebSocket=class extends EventTarget{close(){}};</script><script src="/js/shared/lazy-assets.js"></script><script src="/js/desktop/core/module-loader.js"></script><script src="/neutrality-shell.js"></script>`
	html = strings.Replace(html, "</body>", fixedDate+"</body>", 1)

	store := newThemeSweepStore()
	store.set(map[string]string{
		"appearance.theme": "standard", "appearance.fruity_mode": "light",
		"windows.restore_session": "false", "windows.animations": "false",
		"pet.enabled": "false", "phone_gadget.enabled": "false", "desktop.show_widgets": "false",
	})
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(Content)))
	mux.HandleFunc("/fixture", func(w http.ResponseWriter, r *http.Request) {
		baseline := "0"
		if r.URL.Query().Get("baseline") == "1" {
			baseline = "1"
		}
		fixtureHTML := strings.ReplaceAll(html, "/css/desktop-shell.bundle.css?v=", "/css/desktop-shell.bundle.css?baseline="+baseline+"&v=")
		fixtureHTML = strings.Replace(fixtureHTML, `src="/neutrality-shell.js"`, `src="/neutrality-shell.js?baseline=`+baseline+`"`, 1)
		w.Header().Set("Cache-Control", "no-store")
		fmt.Fprint(w, fixtureHTML)
	})
	mux.HandleFunc("/neutrality-shell.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/javascript")
		w.Header().Set("Cache-Control", "no-store")
		if r.URL.Query().Get("baseline") == "1" {
			fmt.Fprint(w, headShell)
			return
		}
		fmt.Fprint(w, currentShell)
	})
	mux.HandleFunc("/css/desktop-shell.bundle.css", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/css")
		w.Header().Set("Cache-Control", "no-store")
		if r.URL.Query().Get("baseline") == "1" {
			fmt.Fprint(w, headCSS)
			return
		}
		fmt.Fprint(w, currentCSS)
	})
	mux.Handle("/api/desktop/settings", store)
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api/desktop/bootstrap" {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"enabled": true, "builtin_apps": desktop.BuiltinApps(), "installed_apps": []interface{}{},
				"widgets": []interface{}{}, "shortcuts": []interface{}{}, "desktop_files": []interface{}{},
				"workspace": map[string]interface{}{"readonly": false}, "settings": store.snapshot(),
			})
			return
		}
		fmt.Fprint(w, `{"status":"ok","files":[],"pets":[],"settings":{},"enabled":false}`)
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	page := newSmokeBrowserWith(t, nil).MustPage()
	t.Cleanup(func() { _ = page.Close() })
	if err := (proto.EmulationSetEmulatedMedia{Features: []*proto.EmulationMediaFeature{{Name: "prefers-reduced-motion", Value: "reduce"}}}).Call(page); err != nil {
		t.Fatalf("pin reduced motion: %v", err)
	}

	type scenario struct{ theme, mode string }
	var scenarios []scenario
	for _, theme := range []string{"standard", "fruity"} {
		modes := []string{"light"}
		if theme == "fruity" {
			modes = []string{"light", "dark"}
		}
		for _, mode := range modes {
			scenarios = append(scenarios, scenario{theme: theme, mode: mode})
		}
	}

	outputDir := filepath.Join("..", "reports", "desktop-theme-neutrality")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatalf("create screenshot directory: %v", err)
	}
	for _, testCase := range scenarios {
		for _, width := range []int{1280, 420} {
			t.Run(fmt.Sprintf("%s-%s-%d", testCase.theme, testCase.mode, width), func(t *testing.T) {
				store.set(map[string]string{
					"appearance.theme": testCase.theme, "appearance.fruity_mode": testCase.mode,
					"windows.restore_session": "false", "windows.animations": "false",
					"pet.enabled": "false", "phone_gadget.enabled": "false", "desktop.show_widgets": "false",
				})
				if err := page.SetViewport(&proto.EmulationSetDeviceMetricsOverride{Width: width, Height: 900, DeviceScaleFactor: 1, Mobile: false}); err != nil {
					t.Fatalf("set %d px viewport: %v", width, err)
				}
				baseline := desktopThemeNeutralityScreenshot(t, page, server.URL+"/fixture?baseline=1")
				current := desktopThemeNeutralityScreenshot(t, page, server.URL+"/fixture?baseline=0")
				name := fmt.Sprintf("%s-%s-%d", testCase.theme, testCase.mode, width)
				for suffix, data := range map[string][]byte{"head": baseline, "current": current} {
					if err := os.WriteFile(filepath.Join(outputDir, name+"-"+suffix+".png"), data, 0644); err != nil {
						t.Fatalf("save %s screenshot: %v", suffix, err)
					}
				}
				pixels, err := desktopThemeNeutralityDiff(baseline, current)
				if err != nil {
					t.Fatal(err)
				}
				if pixels != 0 {
					t.Fatalf("Standard/Fruity changed %d pixels compared with HEAD (screenshots in %s)", pixels, outputDir)
				}
			})
		}
	}
}

func desktopThemeNeutralityGitShow(t *testing.T, revisionPath string) string {
	t.Helper()
	cmd := exec.Command("git", "-C", "..", "show", revisionPath)
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("read %s from HEAD: %v", revisionPath, err)
	}
	return string(output)
}

func desktopThemeNeutralityScreenshot(t *testing.T, page *rod.Page, url string) []byte {
	t.Helper()
	if err := page.Timeout(60 * time.Second).Navigate(url); err != nil {
		t.Fatalf("navigate to fixture: %v", err)
	}
	if err := page.Timeout(60 * time.Second).WaitLoad(); err != nil {
		t.Fatalf("wait for fixture load: %v", err)
	}
	if err := page.Timeout(30 * time.Second).Wait(rod.Eval(`()=>!!(window.auditTest?.state?.bootstrap && window.auditTest?.state?.ws)`)); err != nil {
		t.Fatalf("wait for Desktop startup: %v", err)
	}
	if _, err := page.Timeout(30 * time.Second).Eval(`async()=>{await AuraDesktopModules.loadAppAssets('calculator');auditTest.openApp('calculator');}`); err != nil {
		t.Fatalf("open calculator: %v", err)
	}
	if err := page.Timeout(20 * time.Second).Wait(rod.Eval(`()=>!!document.querySelector('.vd-window[data-app-id="calculator"] .vd-window-content > *')`)); err != nil {
		t.Fatalf("wait for calculator render: %v", err)
	}
	if _, err := page.Timeout(20 * time.Second).Eval(`async()=>{await document.fonts.ready;await new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r)));await new Promise(r=>setTimeout(r,200));}`); err != nil {
		t.Fatalf("settle screenshot: %v", err)
	}
	if got, err := page.Timeout(10 * time.Second).Eval(`()=>JSON.stringify(window.fixtureErrors||[])`); err != nil || got.Value.Str() != "[]" {
		t.Fatalf("fixture browser errors: %v %v", got, err)
	}
	png, err := page.Screenshot(false, &proto.PageCaptureScreenshot{Format: proto.PageCaptureScreenshotFormatPng})
	if err != nil {
		t.Fatalf("capture screenshot: %v", err)
	}
	return png
}

func desktopThemeNeutralityDiff(before, after []byte) (int, error) {
	a, err := png.Decode(bytes.NewReader(before))
	if err != nil {
		return 0, fmt.Errorf("decode HEAD screenshot: %w", err)
	}
	b, err := png.Decode(bytes.NewReader(after))
	if err != nil {
		return 0, fmt.Errorf("decode current screenshot: %w", err)
	}
	if a.Bounds() != b.Bounds() {
		return 0, fmt.Errorf("screenshot dimensions differ: HEAD %v, current %v", a.Bounds(), b.Bounds())
	}
	changed := 0
	for y := a.Bounds().Min.Y; y < a.Bounds().Max.Y; y++ {
		for x := a.Bounds().Min.X; x < a.Bounds().Max.X; x++ {
			if a.At(x, y) != b.At(x, y) {
				changed++
			}
		}
	}
	return changed, nil
}
