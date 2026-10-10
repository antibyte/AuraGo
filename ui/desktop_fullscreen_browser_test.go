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
	"github.com/go-rod/rod/lib/input"
	"github.com/go-rod/rod/lib/proto"
)

func TestDesktopFullscreenTranslations(t *testing.T) {
	for _, locale := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		var words map[string]string
		if err := json.Unmarshal([]byte(readDesktopAssetText(t, "lang/desktop/"+locale+".json")), &words); err != nil {
			t.Fatal(err)
		}
		for _, key := range []string{"desktop.fullscreen", "desktop.exit_fullscreen", "desktop.fullscreen_unavailable", "desktop.fullscreen_error"} {
			if strings.TrimSpace(words[key]) == "" {
				t.Errorf("%s: missing %s", locale, key)
			}
		}
	}
}

func TestDesktopFullscreenBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	html := regexp.MustCompile(`(?s)<script\b[^>]*>.*?</script>`).ReplaceAllString(readDesktopAssetText(t, "desktop.html"), "")
	html = regexp.MustCompile(`\{\{[^}]*\}\}`).ReplaceAllString(html, "")
	html = strings.Replace(html, "</body>", `<script src="/js/shared/lazy-assets.js"></script><script src="/js/desktop/core/module-loader.js"></script><script src="/fullscreen-shell.js"></script><script src="/testdata/aurora-fixture.js"></script></body>`, 1)
	shell := readDesktopAssetText(t, "js/desktop/bundles/main.bundle.js")
	cut := strings.LastIndex(shell, "    ensureDesktopRadialMenuAnchor();")
	if cut < 0 {
		t.Fatal("desktop startup seam missing")
	}
	shell = shell[:cut] + `window.aurora={state,loadIconManifest,applyDesktopSettings,renderIcons,renderStartApps,renderStartButtonIcon,wireShellChromeControls,bindViewportMetrics,handleDesktopKeydown,closeContextMenu};})();`
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(Content)))
	mux.HandleFunc("/fixture", func(w http.ResponseWriter, _ *http.Request) { fmt.Fprint(w, html) })
	mux.HandleFunc("/fullscreen-shell.js", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/javascript")
		fmt.Fprint(w, shell)
	})
	mux.HandleFunc("/fixture-words", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, readDesktopAssetText(t, "lang/desktop/de.json"))
	})
	mux.HandleFunc("/fixture-apps", func(w http.ResponseWriter, _ *http.Request) { json.NewEncoder(w).Encode(desktop.BuiltinApps()) })
	server := httptest.NewServer(mux)
	defer server.Close()
	page := newSmokeBrowser(t).MustPage(server.URL + "/fixture").Timeout(90 * time.Second)
	defer page.Close()
	page.MustWaitLoad()
	page.MustEval(`async()=>{await fixtureReady;aurora.wireShellChromeControls();}`)

	for _, width := range []int{1366, 820, 390} {
		page.MustSetViewport(width, 900, 1, width == 390)
		if err := (proto.EmulationSetTouchEmulationEnabled{Enabled: width == 390}).Call(page); err != nil {
			t.Fatal(err)
		}
		for _, theme := range []string{"standard", "fruity-light", "fruity-dark"} {
			page.MustEval(`theme=>fixtureTheme(theme)`, theme)
			page.MustEval(`async()=>{
                await new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r)));
                const b=document.getElementById('vd-fullscreen-button'),r=b.getBoundingClientRect();
                if(b.disabled||b.title!=='Vollbild'||b.getAttribute('aria-pressed')!=='false')throw Error('Initial button state');
                if(r.width<22||r.height<22||r.left<0||r.top<0||r.right>innerWidth||r.bottom>innerHeight)throw Error('Button outside viewport: '+JSON.stringify(r));
                if(!b.contains(document.elementFromPoint(r.x+r.width/2,r.y+r.height/2)))throw Error('Button is covered');
                if(matchMedia('(pointer: coarse)').matches&&(r.width<44||r.height<44))throw Error('Touch target too small');
            }`)
			if dir := os.Getenv("AURAGO_BROWSER_ARTIFACT_DIR"); dir != "" {
				if err := os.MkdirAll(dir, 0755); err != nil {
					t.Fatal(err)
				}
				page.MustScreenshot(filepath.Join(dir, fmt.Sprintf("fullscreen-%s-%d.png", theme, width)))
			}
			page.MustElement("#vd-fullscreen-button").MustClick()
			waitForJSBool(t, page, `()=>document.fullscreenElement===document.documentElement && document.getElementById('vd-fullscreen-button').getAttribute('aria-pressed')==='true' && !document.getElementById('vd-fullscreen-button').disabled`)
			page.MustEval(`()=>{const b=document.getElementById('vd-fullscreen-button');if(b.title!=='Vollbild beenden'||b.getAttribute('aria-label')!==b.title)throw Error('Exit label');}`)
			page.MustElement("#vd-fullscreen-button").MustClick()
			waitForJSBool(t, page, `()=>!document.fullscreenElement && document.getElementById('vd-fullscreen-button').getAttribute('aria-pressed')==='false' && !document.getElementById('vd-fullscreen-button').disabled`)
		}
	}
	(proto.EmulationSetTouchEmulationEnabled{Enabled: false}).Call(page)
	page.MustSetViewport(1366, 900, 1, false)
	page.MustEval(`()=>{fixtureTheme('standard');document.getElementById('vd-fullscreen-button').focus();}`)
	page.Keyboard.MustType(input.Space)
	waitForJSBool(t, page, `()=>!!document.fullscreenElement && !document.getElementById('vd-fullscreen-button').disabled`)
	// Browser UI/Esc exits emit the same native fullscreenchange event.
	page.MustEval(`async()=>{await document.exitFullscreen();}`)
	waitForJSBool(t, page, `()=>document.getElementById('vd-fullscreen-button').getAttribute('aria-pressed')==='false'`)
	page.MustEval(`async()=>{
        const b=document.getElementById('vd-fullscreen-button'),root=document.documentElement;
        const native=root.requestFullscreen;
        let calls=0,fail;
        root.requestFullscreen=()=>{calls++;return new Promise((resolve,reject)=>{fail=reject;});};
        b.click();b.click();
        if(calls!==1||!b.disabled||b.getAttribute('aria-busy')!=='true')throw Error('Duplicate request or missing pending state');
        fail(Error('blocked'));await new Promise(r=>setTimeout(r,0));
        if(b.disabled||b.getAttribute('aria-pressed')!=='false')throw Error('Failure did not restore the button');
        if(!document.querySelector('.vd-toast-message')?.textContent.includes('Vollbildmodus'))throw Error('Missing failure feedback');
        root.requestFullscreen=undefined;
        window.dispatchEvent(new Event('aurago:language-changed'));
        if(!b.disabled||!b.title.includes('nicht verfügbar'))throw Error('Unsupported browser not explained');
        root.requestFullscreen=native;
        window.dispatchEvent(new Event('aurago:language-changed'));
        if(b.disabled||b.title!=='Vollbild')throw Error('Refresh failed');
        if(fixtureErrors.length)throw Error(JSON.stringify(fixtureErrors));
    }`)
}
