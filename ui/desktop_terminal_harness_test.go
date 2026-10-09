package ui

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
)

// Prepended to every terminal fixture: browser errors are collected in fixtureErrors, every xterm instance
// lands in fixtureTerms with a steady cursor (no blink between reads), and a failing renderer addon counts
// as an error unless the fixture switched WebGL off (window.fixtureNoGL).
const terminalFixturePrelude = `
window.fixtureErrors=[];
addEventListener('error',e=>fixtureErrors.push(e.error&&e.error.stack||e.message));
addEventListener('unhandledrejection',e=>fixtureErrors.push(String(e.reason&&e.reason.stack||e.reason)));
window.fixtureTerms=[];
const FixtureXterm=Terminal;
window.Terminal=class extends FixtureXterm{
 constructor(opts){super({...opts,cursorBlink:false});fixtureTerms.push(this);}
 loadAddon(addon){
  try{return super.loadAddon(addon);}
  catch(e){if(!window.fixtureNoGL)fixtureErrors.push('addon: '+e.message);throw e;}
 }
};
`

var (
	terminalLoaderEntry   = regexp.MustCompile(`(?s)\n\s*'terminal':\s*\{(.*?)\n\s*\},`)
	terminalLoaderStyles  = regexp.MustCompile(`(?s)styles:\s*appStyles\((.*?)\)`)
	terminalLoaderScripts = regexp.MustCompile(`(?s)scripts:\s*\[(.*?)\]`)
	quotedAssetPath       = regexp.MustCompile(`'(/[^']+)'`)
)

// terminalLoaderAssets returns the Terminal's stylesheets and scripts exactly as DESKTOP_APP_ASSETS in
// module-loader.js lists them, so the browser fixtures load what the desktop loads, in the same order.
func terminalLoaderAssets(t *testing.T) (styles, scripts []string) {
	t.Helper()
	entry := terminalLoaderEntry.FindStringSubmatch(readDesktopAssetText(t, "js/desktop/core/module-loader.js"))
	if entry == nil {
		t.Fatal("module-loader.js has no 'terminal' entry")
	}
	list := func(pattern *regexp.Regexp, kind string) []string {
		block := pattern.FindStringSubmatch(entry[1])
		if block == nil {
			t.Fatalf("module-loader.js 'terminal' entry has no %s", kind)
		}
		var out []string
		for _, match := range quotedAssetPath.FindAllStringSubmatch(block[1], -1) {
			out = append(out, match[1])
		}
		if len(out) == 0 {
			t.Fatalf("module-loader.js 'terminal' entry lists no %s", kind)
		}
		return out
	}
	return list(terminalLoaderStyles, "styles"), list(terminalLoaderScripts, "scripts")
}

// newTerminalDesktopServer serves /fixture: desktop.html without its scripts, the real main bundle cut at
// the startup seam (exposing window.terminalTest), the Terminal's assets in module-loader order and then
// fixtureJS after the shared prelude (/terminal-fixture.js). extraRoutes add or replace single paths.
func newTerminalDesktopServer(t *testing.T, fixtureJS string, extraRoutes map[string]http.HandlerFunc) *httptest.Server {
	t.Helper()
	styles, scripts := terminalLoaderAssets(t)
	html := readDesktopAssetText(t, "desktop.html")
	html = regexp.MustCompile(`(?s)<script\b[^>]*>.*?</script>`).ReplaceAllString(html, "")
	html = regexp.MustCompile(`\{\{[^}]*\}\}`).ReplaceAllString(html, "")
	var links strings.Builder
	for _, href := range styles {
		fmt.Fprintf(&links, `<link rel="stylesheet" href="%s">`, href)
	}
	html = strings.Replace(html, "</head>", links.String()+"</head>", 1)
	var tags strings.Builder
	for _, src := range append(append([]string{"/terminal-shell.js"}, scripts...), "/terminal-fixture.js") {
		fmt.Fprintf(&tags, `<script src="%s"></script>`, src)
	}
	html = strings.Replace(html, "</body>", tags.String()+"</body>", 1)
	shell := readDesktopAssetText(t, "js/desktop/bundles/main.bundle.js")
	cut := strings.LastIndex(shell, "    ensureDesktopRadialMenuAnchor();")
	if cut < 0 {
		t.Fatal("desktop startup seam missing")
	}
	shell = shell[:cut] + `window.terminalTest={state,openApp,loadIconManifest,closeWindow,applyDesktopSettings,renderTaskbar};})();`
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(Content)))
	sources := map[string]string{"/fixture": html, "/terminal-shell.js": shell, "/terminal-fixture.js": terminalFixturePrelude + fixtureJS}
	for route, source := range sources {
		mux.HandleFunc(route, func(w http.ResponseWriter, r *http.Request) {
			if route == "/fixture" {
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
			} else {
				w.Header().Set("Content-Type", "text/javascript")
			}
			fmt.Fprint(w, source)
		})
	}
	for route, handler := range extraRoutes {
		mux.HandleFunc(route, handler)
	}
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// newTerminalBrowser launches headless Chrome or Edge like newSmokeBrowser (skip when none is installed or
// the launch fails) with a real device scale factor: CDP emulation alone leaves ResizeObserver's
// devicePixelContentBoxSize at the host scale and mis-sizes xterm's canvases.
func newTerminalBrowser(t *testing.T, scale string) *rod.Browser {
	t.Helper()
	bin, ok := browserExecutable()
	if !ok {
		t.Skip("headless browser smoke test requires Chrome or Edge")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	t.Cleanup(cancel)
	u, err := launcher.New().
		Context(ctx).
		Bin(bin).
		Headless(true).
		NoSandbox(true).
		Set("disable-gpu").
		Set("disable-dev-shm-usage").
		Set("force-device-scale-factor", scale).
		Launch()
	if err != nil {
		t.Skipf("headless browser smoke test skipped; browser launch failed: %v", err)
	}
	browser := rod.New().ControlURL(u)
	if err := browser.Connect(); err != nil {
		t.Fatalf("connect headless browser for the terminal test: %v", err)
	}
	t.Cleanup(func() { _ = browser.Close() })
	return browser
}
