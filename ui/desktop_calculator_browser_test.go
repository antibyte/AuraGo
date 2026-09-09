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

func TestDesktopCalculatorLayoutBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	html := regexp.MustCompile(`(?s)<script\b[^>]*>.*?</script>`).ReplaceAllString(readDesktopAssetText(t, "desktop.html"), "")
	html = regexp.MustCompile(`\{\{[^}]*\}\}`).ReplaceAllString(html, "")
	html = strings.Replace(html, "</body>", `<script src="/js/shared/lazy-assets.js"></script><script src="/js/desktop/core/module-loader.js"></script><script src="/calculator-shell.js"></script><script src="/testdata/aurora-fixture.js"></script></body>`, 1)
	shell := readDesktopAssetText(t, "js/desktop/bundles/main.bundle.js")
	cut := strings.LastIndex(shell, "    ensureDesktopRadialMenuAnchor();")
	if cut < 0 {
		t.Fatal("desktop startup seam missing")
	}
	shell = shell[:cut] + `window.aurora={state,loadIconManifest,applyDesktopSettings,renderIcons,renderStartApps,renderStartButtonIcon,wireShellChromeControls,bindViewportMetrics,handleDesktopKeydown,closeContextMenu,openApp,closeWindow};})();`
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(Content)))
	mux.HandleFunc("/fixture", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, html) })
	mux.HandleFunc("/calculator-shell.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/javascript")
		fmt.Fprint(w, shell)
	})
	mux.HandleFunc("/fixture-words", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, readDesktopAssetText(t, "lang/desktop/de.json"))
	})
	mux.HandleFunc("/fixture-apps", func(w http.ResponseWriter, r *http.Request) { json.NewEncoder(w).Encode(desktop.BuiltinApps()) })
	server := httptest.NewServer(mux)
	defer server.Close()
	page := newSmokeBrowser(t).MustPage(server.URL + "/fixture").Timeout(40 * time.Second)
	defer page.Close()
	page.MustWaitLoad()
	page.MustEval(`async()=>{await fixtureReady;await AuraDesktopModules.loadAppAssets('calculator');}`)
	click := func(selector string) {
		t.Helper()
		button := page.MustElement(selector)
		point := button.MustEval(`function(){const r=this.getBoundingClientRect();return [r.x+r.width/2,r.y+r.height/2];}`).Arr()
		if !button.MustEval(`function(){const r=this.getBoundingClientRect();return this.contains(document.elementFromPoint(r.x+r.width/2,r.y+r.height/2));}`).Bool() {
			t.Fatalf("calculator control is covered: %s", button.MustEval(`function(){const r=this.getBoundingClientRect();return JSON.stringify({rect:r,hit:document.elementFromPoint(r.x+r.width/2,r.y+r.height/2)?.outerHTML});}`).Str())
		}
		page.Mouse.MustMoveTo(point[0].Num(), point[1].Num()).MustClick(proto.InputMouseButtonLeft)
	}
	for _, viewport := range [][2]int{{1366, 900}, {390, 667}} {
		page.MustSetViewport(viewport[0], viewport[1], 1, false)
		for _, theme := range []string{"fruity-dark", "fruity-light", "standard"} {
			page.MustEval(`async theme=>{fixtureTheme(theme);await fixtureOpen('calculator');}`, theme)
			if !page.MustEval(`()=>document.querySelector('[data-prog-section]').hidden`).Bool() {
				t.Fatal("programmer controls must be hidden when opening in standard mode")
			}
			for _, mode := range []string{"standard", "scientific", "programmer", "standard"} {
				click(`[data-mode="` + mode + `"]`)
				if issues := page.MustEval(`()=>{
                    const root=document.querySelector('.vd-calc'),r=root.getBoundingClientRect(),issues=[];
                    if(root.scrollWidth>root.clientWidth+1 || root.scrollHeight>root.clientHeight+1)issues.push('calculator overflows '+root.clientWidth+'x'+root.clientHeight+' with '+root.scrollWidth+'x'+root.scrollHeight);
                    for(const b of root.querySelectorAll('button')){
                        if(!b.getClientRects().length)continue;
                        const k=b.getBoundingClientRect();
                        if(k.left<r.left || k.right>r.right || k.top<r.top || k.bottom>r.bottom || k.height<24 || b.scrollWidth>b.clientWidth+1)issues.push('clipped key: '+b.textContent);
                    }
                    return issues.join('\n');
                }`).Str(); issues != "" {
					t.Fatalf("%s %s at %dx%d: %s", theme, mode, viewport[0], viewport[1], issues)
				}
				for _, key := range []string{"7", "+", "8", "="} {
					click(`[data-key="` + key + `"]`)
				}
				if page.MustElement("[data-result]").MustText() != "15" {
					t.Fatal("calculator keys must remain usable in every mode")
				}
				if dir := os.Getenv("AURAGO_BROWSER_ARTIFACT_DIR"); dir != "" {
					if err := os.MkdirAll(dir, 0755); err != nil {
						t.Fatal(err)
					}
					page.MustScreenshot(filepath.Join(dir, fmt.Sprintf("calculator-%s-%s-%d.png", theme, mode, viewport[0])))
				}
			}
			page.MustEval(`()=>[...aurora.state.windows.keys()].forEach(id=>aurora.closeWindow(id))`)
		}
	}
	if errors := page.MustEval(`()=>JSON.stringify(fixtureErrors)`).Str(); errors != "[]" {
		t.Fatal(errors)
	}
}
