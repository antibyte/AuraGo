package ui

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-rod/rod"
)

// localWikipediaLowContrastJS returns "selector=ratio" for every selector whose
// text color has less than 4.5:1 contrast against its composited background
// (WCAG AA for normal text); missing elements and backgrounds without an
// opaque ancestor are reported too.
const localWikipediaLowContrastJS = `(selectors)=>{
const parse=value=>{let m=/^rgba?\(([^)]+)\)$/.exec(value);if(m){const p=m[1].split(/[\s,\/]+/).filter(Boolean).map(Number);return {r:p[0],g:p[1],b:p[2],a:p.length>3?p[3]:1}}
m=/^color\(srgb ([^)]+)\)$/.exec(value);if(m){const p=m[1].split(/[\s\/]+/).filter(Boolean).map(Number);return {r:p[0]*255,g:p[1]*255,b:p[2]*255,a:p.length>3?p[3]:1}}return null};
const over=(top,bottom)=>({r:top.r*top.a+bottom.r*(1-top.a),g:top.g*top.a+bottom.g*(1-top.a),b:top.b*top.a+bottom.b*(1-top.a),a:1});
const background=el=>{const layers=[];for(let n=el;n;n=n.parentElement){const c=parse(getComputedStyle(n).backgroundColor);if(c&&c.a>0){layers.push(c);if(c.a>=1)break}}
if(!layers.length||layers[layers.length-1].a<1)return null;return layers.reduceRight((acc,c)=>over(c,acc),{r:0,g:0,b:0,a:1})};
const lum=c=>{const f=v=>{v/=255;return v<=0.03928?v/12.92:Math.pow((v+0.055)/1.055,2.4)};return 0.2126*f(c.r)+0.7152*f(c.g)+0.0722*f(c.b)};
const out=[];
for(const selector of selectors){const el=document.querySelector(selector);if(!el){out.push(selector+'=missing');continue}
const bg=background(el);const fg=parse(getComputedStyle(el).color);if(!bg||!fg){out.push(selector+'=unknown '+getComputedStyle(el).color);continue}
const text=over(fg,bg);const a=lum(text),b=lum(bg);const ratio=(Math.max(a,b)+0.05)/(Math.min(a,b)+0.05);if(ratio<4.5)out.push(selector+'='+ratio.toFixed(2))}
return out.join(', ')}`

// TestDesktopLocalWikipediaShellBrowser opens the Wikipedia app through the real
// Desktop shell (desktop.html + main.bundle.js) with the fake API of
// desktop_local_wikipedia_browser_test.go and checks it in Standard dark and
// Fruity light/dark at desktop and phone widths.
func TestDesktopLocalWikipediaShellBrowser(t *testing.T) {
	status := localWikipediaFixtureWith(localWikipediaFixtureStatus("ready", true, true, true, 1), "fulltext", false, "update_available", localWikipediaFixtureUpdate())
	fixture := &localWikipediaFixture{status: status}
	page := desktopAuditBrowserWithAPI(t, map[string]http.Handler{"/api/desktop/local-wikipedia/": fixture})
	// The server fills <html lang>; the fixture strips template fields, so set the German UI language.
	page.MustEval(`()=>{document.documentElement.lang='de';auditTest.openApp('local-wikipedia')}`)
	page.MustWait(`()=>document.querySelector('.lw-title')?.textContent==='Hauptseite'`)
	dir := filepath.Join("..", "reports", "local-wikipedia-shell")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	themes := []struct{ theme, mode string }{{"standard", "dark"}, {"fruity", "light"}, {"fruity", "dark"}}
	setTheme := func(theme, mode string) {
		page.MustEval(`(theme,mode)=>{const a=auditTest;a.state.bootstrap.settings['appearance.theme']=theme;a.state.bootstrap.settings['appearance.fruity_mode']=mode;document.documentElement.dataset.theme=mode;a.applyDesktopSettings();a.refreshSpacesForViewport();}`, theme, mode)
		page.MustWait(`()=>!!document.querySelector('.lw-app')`)
	}
	checkContrast := func(name string, selectors ...string) {
		t.Helper()
		if low := page.MustEval(localWikipediaLowContrastJS, selectors).Str(); low != "" {
			t.Fatalf("text contrast below 4.5:1 in %s: %s", name, low)
		}
	}
	toolbars := map[string]string{}
	for _, width := range []int{1280, 420} {
		page.MustSetViewport(width, 900, 1, false)
		if width == 420 {
			// Phones open apps maximized; a window opened on the wide desktop keeps its size otherwise.
			page.MustEval(`()=>{const a=auditTest;a.applyWindowSnap(a.state.windows.get(a.state.activeWindowId).element,'maximize')}`)
		}
		for _, combo := range themes {
			name := fmt.Sprintf("%s-%s-%d", combo.theme, combo.mode, width)
			setTheme(combo.theme, combo.mode)
			if page.MustEval(`()=>{const app=document.querySelector('.lw-app');return app.scrollWidth>app.clientWidth+1}`).Bool() {
				t.Fatalf("horizontal overflow in %s", name)
			}
			if bg := page.MustEval(`()=>getComputedStyle(document.querySelector('.lw-frame')).backgroundColor`).Str(); bg != "rgb(255, 255, 255)" {
				t.Fatalf("article surface in %s = %s", name, bg)
			}
			// The window stays on screen and, in Standard, the taskbar never covers the footer.
			if !page.MustEval(`(theme)=>{const w=document.querySelector('.vd-window[data-app-id="local-wikipedia"]').getBoundingClientRect();const f=document.querySelector('.lw-footer').getBoundingClientRect();const bar=document.querySelector('.vd-taskbar').getBoundingClientRect();
return w.left>=-1&&w.right<=innerWidth+1&&w.top>=0&&f.bottom<=innerHeight+1&&(theme!=='standard'||f.bottom<=bar.top+1)}`, combo.theme).Bool() {
				t.Fatalf("window or footer off screen in %s", name)
			}
			if width == 420 && !page.MustEval(`()=>{const box=s=>document.querySelector(s).getBoundingClientRect();return box('.lw-search').top>=box('.lw-nav').bottom-1&&getComputedStyle(document.querySelector('.lw-submit span')).display==='none'}`).Bool() {
				t.Fatalf("narrow toolbar layout missing in %s", name)
			}
			if footer := page.MustEval(`()=>document.querySelector('.lw-footer').textContent`).Str(); !strings.Contains(footer, "Deutsch") || !strings.Contains(footer, "Oktober 2026") {
				t.Fatalf("footer in %s = %q", name, footer)
			}
			checkContrast(name, ".lw-title", ".lw-footer", ".lw-footer-label", `.lw-banner[data-kind="info"] > span`, `.lw-banner[data-kind="hint"] > span`, `.lw-banner [data-action="settings"]`, `.lw-tool[data-action="main"]`, ".lw-input", ".lw-submit")
			toolbars[combo.theme+"-"+combo.mode] = page.MustEval(`()=>getComputedStyle(document.querySelector('.lw-toolbar')).backgroundColor`).Str()
			page.MustScreenshot(filepath.Join(dir, name+".png"))
		}
	}
	if toolbars["standard-dark"] == toolbars["fruity-light"] || toolbars["fruity-light"] == toolbars["fruity-dark"] {
		t.Fatalf("toolbar ignores the theme tokens: %v", toolbars)
	}

	// Results, the empty result and a failed search keep readable text in every theme.
	page.MustSetViewport(1280, 900, 1, false)
	search := func(query, ready string) {
		page.MustEval(`(query)=>{const input=document.querySelector('.lw-input');input.value=query;document.querySelector('.lw-search').requestSubmit()}`, query)
		if err := page.Wait(rod.Eval(ready)); err != nil {
			t.Fatalf("search %q: %v", query, err)
		}
	}
	for _, view := range []struct {
		query, ready string
		selectors    []string
	}{
		{"Hauptstadt", `()=>document.querySelectorAll('.lw-result').length===2`, []string{".lw-results-title", ".lw-result strong", ".lw-result span"}},
		{"Nichts", `()=>!!document.querySelector('.lw-results .lw-muted')`, []string{".lw-results .lw-muted"}},
		{"Fehler", `()=>!!document.querySelector('.lw-results .lw-error')`, []string{".lw-results .lw-error"}},
	} {
		search(view.query, view.ready)
		for _, combo := range themes {
			name := fmt.Sprintf("%s-%s-%s", strings.ToLower(view.query), combo.theme, combo.mode)
			setTheme(combo.theme, combo.mode)
			checkContrast(name, view.selectors...)
			if view.query == "Hauptstadt" {
				page.MustScreenshot(filepath.Join(dir, "results-"+combo.theme+"-"+combo.mode+".png"))
			}
		}
	}
	// The in-app article error (a missing article) keeps readable text in every theme.
	page.MustEval(`()=>document.querySelector('.lw-tool[data-action="main"]').click()`)
	page.MustWait(`()=>document.querySelector('.lw-title')?.textContent==='Hauptseite'`)
	page.MustEval(`()=>document.querySelector('.lw-frame').contentDocument.getElementById('to-missing').click()`)
	page.MustWait(`()=>{const e=document.querySelector('.lw-frame-error');return !!e&&!e.hidden}`)
	for _, combo := range themes {
		setTheme(combo.theme, combo.mode)
		checkContrast(fmt.Sprintf("frame-error-%s-%s", combo.theme, combo.mode), ".lw-frame-error")
	}

	// State screens replace the article; each one opens in a fresh window that reads its own status.
	for _, screen := range []struct {
		name      string
		prepare   func()
		ready     string
		selectors []string
	}{
		{"not-installed", func() { fixture.setStatus(localWikipediaFixtureStatus("not_installed", true, false, false, 0)) },
			`()=>!!document.querySelector('.lw-state-card[data-state="not_installed"]')`,
			[]string{".lw-state-card h2", ".lw-state-card p", `.lw-state-card [data-action="settings"]`}},
		{"downloading", func() { fixture.setStatus(localWikipediaFixtureStatus("downloading", false, false, false, 0.42)) },
			`()=>!!document.querySelector('.lw-state-card[data-state="downloading"]')`,
			[]string{".lw-state-card h2", ".lw-state-card p"}},
		{"failed", func() {
			fixture.setFailure(http.StatusInternalServerError, map[string]any{"error": "boom", "code": "internal"})
		},
			`()=>!!document.querySelector('.lw-state-card[data-state="failed"]')`,
			[]string{".lw-state-card h2", ".lw-state-card p", `.lw-state-card [data-action="retry"]`}},
	} {
		page.MustEval(`()=>auditTest.closeWindow(auditTest.state.activeWindowId)`)
		page.MustWait(`()=>!document.querySelector('.lw-app')`)
		screen.prepare()
		page.MustEval(`()=>auditTest.openApp('local-wikipedia')`)
		if err := page.Wait(rod.Eval(screen.ready)); err != nil {
			t.Fatalf("state screen %s: %v", screen.name, err)
		}
		for _, combo := range themes {
			setTheme(combo.theme, combo.mode)
			checkContrast(fmt.Sprintf("%s-%s-%s", screen.name, combo.theme, combo.mode), screen.selectors...)
			if combo.theme == "standard" {
				page.MustScreenshot(filepath.Join(dir, "state-"+screen.name+".png"))
			}
		}
	}
	if got := page.MustEval(`()=>JSON.stringify(fixtureErrors)`).Str(); got != "[]" {
		t.Fatalf("shell errors: %s", got)
	}
}
