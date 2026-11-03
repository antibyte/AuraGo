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
	"aurago/internal/ecalc"

	"github.com/go-rod/rod"
)

// ecalcBrowserFixture is the fixture desktop of the ECalc browser tests of phase 3 (tables, code pages): the real desktop
// shell of the bundle with the fixture API of testdata/aurora-fixture.js, the German words of the desktop, and the
// catalog of the server route (ecalc.Default, localized to the requested language) with a patch applied to the JSON
// before the app sees it. The patch is the source of a JavaScript function `catalogJSON => catalogJSON` that adds the
// content later parts write (fixture tables, a finder calculator, a code item) in exactly the shape of the served
// catalog. The shell runs in German; set AURAGO_BROWSER_ARTIFACT_DIR to keep the screenshots.
type ecalcBrowserFixture struct {
	T       *testing.T
	Page    *rod.Page
	Words   map[string]string // the merged German desktop words
	Catalog *ecalc.Catalog
}

func newECalcBrowserFixture(t *testing.T, patch string) *ecalcBrowserFixture {
	t.Helper()
	requirePrecisionBrowserSmoke(t)
	catalog, err := ecalc.Default()
	if err != nil {
		t.Fatal(err)
	}
	html := regexp.MustCompile(`(?s)<script\b[^>]*>.*?</script>`).ReplaceAllString(readDesktopAssetText(t, "desktop.html"), "")
	html = regexp.MustCompile(`\{\{[^}]*\}\}`).ReplaceAllString(html, "")
	html = strings.Replace(html, "</body>", `<script src="/js/shared/lazy-assets.js"></script><script src="/js/desktop/core/module-loader.js"></script><script src="/ecalc-fixture-shell.js"></script><script src="/testdata/aurora-fixture.js"></script></body>`, 1)
	shell := readDesktopAssetText(t, "js/desktop/bundles/main.bundle.js")
	cut := strings.LastIndex(shell, "    ensureDesktopRadialMenuAnchor();")
	if cut < 0 {
		t.Fatal("desktop startup seam missing")
	}
	shell = shell[:cut] + `window.aurora={state,loadIconManifest,applyDesktopSettings,renderIcons,renderStartApps,renderStartButtonIcon,wireShellChromeControls,bindViewportMetrics,handleDesktopKeydown,closeContextMenu,openApp,closeWindow};})();`

	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(Content)))
	mux.HandleFunc("/fixture", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, html) })
	mux.HandleFunc("/ecalc-fixture-shell.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/javascript")
		fmt.Fprint(w, shell)
	})
	mux.HandleFunc("/ecalc-catalog.json", func(w http.ResponseWriter, r *http.Request) {
		body, _, err := catalog.LocalizedJSON(r.URL.Query().Get("lang"))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	})
	mux.HandleFunc("/fixture-words", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, readDesktopAssetText(t, "lang/desktop/de.json"))
	})
	mux.HandleFunc("/fixture-apps", func(w http.ResponseWriter, r *http.Request) { json.NewEncoder(w).Encode(desktop.BuiltinApps()) })
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	// The page budget is 170 seconds (a loaded machine needs far more than the 90 the first drafts had); the browser itself is
	// stopped after 180 (newSmokeBrowserWith).
	page := newSmokeBrowser(t).MustPage(server.URL + "/fixture").Timeout(170 * time.Second)
	t.Cleanup(func() { _ = page.Close() })
	page.MustWaitLoad()
	page.MustSetViewport(1366, 900, 1, false)
	// The fixture answers every /api/ request with an empty object; the catalog route gets the real catalog of the
	// requested language with the patch applied.
	page.MustEval(`async()=>{
		await fixtureReady;
		window.ecalcPatchCatalog=` + patch + `;
		const base=window.fetch;
		window.fetch=(url,options)=>{
			const text=String(url);
			if(!text.startsWith('/api/ecalc/catalog'))return base(url,options);
			return base('/ecalc-catalog.json'+text.slice('/api/ecalc/catalog'.length)).then(r=>r.json()).then(j=>{
				window.ecalcPatchCatalog(j);
				return new Response(JSON.stringify(j),{headers:{'Content-Type':'application/json'}});
			});
		};
		fixtureTheme('standard');
	}`)
	return &ecalcBrowserFixture{T: t, Page: page, Words: readMergedLangMap(t, "de"), Catalog: catalog}
}

// Wait polls a JavaScript predicate (a function source) for up to five seconds; a timeout reports the errors the page
// logged meanwhile.
func (f *ecalcBrowserFixture) Wait(expr string) {
	f.T.Helper()
	begin := time.Now()
	deadline := begin.Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if f.Page.MustEval(expr).Bool() {
			if took := time.Since(begin); took > time.Second {
				f.T.Logf("slow wait (%v): %s", took.Round(time.Millisecond), expr)
			}
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	f.T.Fatalf("condition did not become true: %s\npage errors: %s", expr, f.Page.MustEval(`()=>JSON.stringify(fixtureErrors)`).Str())
}

// Shot saves a screenshot when AURAGO_BROWSER_ARTIFACT_DIR is set.
func (f *ecalcBrowserFixture) Shot(name string) {
	f.T.Helper()
	dir := os.Getenv("AURAGO_BROWSER_ARTIFACT_DIR")
	if dir == "" {
		return
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		f.T.Fatal(err)
	}
	f.Page.MustScreenshot(filepath.Join(dir, name+".png"))
}

// OpenApp opens the ECalc window and waits until `ready` (a selector) exists.
func (f *ecalcBrowserFixture) OpenApp(ready string) {
	f.T.Helper()
	f.Page.MustEval(`async()=>{await fixtureOpen('ecalc');}`)
	f.Wait(`()=>!!document.querySelector('` + ready + `')`)
}

// CloseWindows closes every desktop window (and with it every ECalc instance).
func (f *ecalcBrowserFixture) CloseWindows() {
	f.Page.MustEval(`()=>[...aurora.state.windows.keys()].forEach(id=>aurora.closeWindow(id))`)
}
