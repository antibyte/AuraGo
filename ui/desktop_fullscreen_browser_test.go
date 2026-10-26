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
	"github.com/go-rod/rod/lib/proto"
)

func TestDesktopFullscreenTranslations(t *testing.T) {
	for _, locale := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		var words map[string]string
		if err := json.Unmarshal([]byte(readDesktopAssetText(t, "lang/desktop/"+locale+".json")), &words); err != nil {
			t.Fatal(err)
		}
		for _, key := range []string{"desktop.fullscreen", "desktop.exit_fullscreen", "desktop.fullscreen_unavailable", "desktop.fullscreen_error", "desktop.settings_open_fullscreen", "desktop.settings_open_fullscreen_desc"} {
			if strings.TrimSpace(words[key]) == "" {
				t.Errorf("%s: missing %s", locale, key)
			}
		}
	}
}

func newDesktopFullscreenPage(t *testing.T) *rod.Page {
	t.Helper()
	requirePrecisionBrowserSmoke(t)
	html := regexp.MustCompile(`(?s)<script\b[^>]*>.*?</script>`).ReplaceAllString(readDesktopAssetText(t, "desktop.html"), "")
	html = regexp.MustCompile(`\{\{[^}]*\}\}`).ReplaceAllString(html, "")
	html = strings.Replace(html, "</body>", `<script src="/js/shared/lazy-assets.js"></script><script src="/js/desktop/core/module-loader.js"></script><script src="/fullscreen-shell.js"></script><script src="/testdata/aurora-fixture.js"></script></body>`, 1)
	shell := readDesktopAssetText(t, "js/desktop/bundles/main.bundle.js")
	cut := strings.LastIndex(shell, "    ensureDesktopRadialMenuAnchor();")
	if cut < 0 {
		t.Fatal("desktop startup seam missing")
	}
	shell = shell[:cut] + `window.aurora={state,loadIconManifest,applyDesktopSettings,renderIcons,renderStartApps,renderStartButtonIcon,wireShellChromeControls,openDesktopFullscreenOnStartup,bindViewportMetrics,handleDesktopKeydown,closeContextMenu};})();`
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
	t.Cleanup(server.Close)
	page := newSmokeBrowser(t).MustPage(server.URL + "/fixture").Timeout(90 * time.Second)
	page.MustWaitLoad()
	page.MustEval(`async()=>{await fixtureReady;aurora.wireShellChromeControls();}`)
	return page
}

func TestDesktopFullscreenBrowser(t *testing.T) {
	page := newDesktopFullscreenPage(t)
	defer page.Close()

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

func TestDesktopFullscreenOnOpenBrowser(t *testing.T) {
	page := newDesktopFullscreenPage(t)
	defer page.Close()
	for _, tc := range []struct {
		name, theme, gesture string
		enabled              bool
	}{
		{"default_off", "standard", "click", false},
		{"mouse", "standard", "click", true},
		{"activation_at_startup", "standard", "active", true},
		{"touch", "standard", "touch", true},
		{"keyboard", "fruity-light", "key", true},
		{"manual_button", "fruity-dark", "button", true},
		{"disabled_before_gesture", "standard", "disable", true},
		{"unsupported", "standard", "unsupported", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			page.MustReload().MustWaitLoad()
			page.MustEval(`async(theme,enabled,gesture)=>{
                await fixtureReady;
                fixtureTheme(theme);
                if(enabled) aurora.state.bootstrap.settings['desktop.open_fullscreen']='true';
                if(gesture==='unsupported') {
                    document.documentElement.requestFullscreen=undefined;
                    window.dispatchEvent(new Event('aurago:language-changed'));
                }
                window.startupFullscreenCalls=0;
                const native=document.documentElement.requestFullscreen;
                if(native) document.documentElement.requestFullscreen=function(){startupFullscreenCalls++;return native.call(this);};
                if(gesture!=='active') aurora.openDesktopFullscreenOnStartup();
                if(startupFullscreenCalls||document.fullscreenElement) throw Error('Requested fullscreen without activation');
                document.body.dispatchEvent(new MouseEvent('click',{bubbles:true}));
                document.body.dispatchEvent(new KeyboardEvent('keydown',{key:'Enter',bubbles:true}));
                if(startupFullscreenCalls) throw Error('Synthetic input triggered fullscreen');
                if(gesture==='disable') aurora.state.bootstrap.settings['desktop.open_fullscreen']='false';
            }`, tc.theme, tc.enabled, tc.gesture)
			if tc.gesture == "key" {
				page.Keyboard.MustType(input.Tab)
			} else if tc.gesture == "touch" {
				if err := (proto.EmulationSetTouchEmulationEnabled{Enabled: true}).Call(page); err != nil {
					t.Fatal(err)
				}
				page.Touch.MustTap(600, 400)
				if err := (proto.EmulationSetTouchEmulationEnabled{Enabled: false}).Call(page); err != nil {
					t.Fatal(err)
				}
			} else if tc.gesture == "button" {
				page.MustElement("#vd-fullscreen-button").MustClick()
			} else {
				page.Mouse.MustMoveTo(600, 400).MustClick(proto.InputMouseButtonLeft)
			}
			if tc.gesture == "active" {
				page.MustEval(`()=>aurora.openDesktopFullscreenOnStartup()`)
			}
			if !tc.enabled || tc.gesture == "disable" || tc.gesture == "unsupported" {
				page.MustEval(`()=>{if(startupFullscreenCalls||document.fullscreenElement)throw Error('Fullscreen preference or support ignored');}`)
				return
			}
			waitForJSBool(t, page, `()=>document.fullscreenElement===document.documentElement && !document.getElementById('vd-fullscreen-button').disabled`)
			page.MustEval(`async()=>{if(startupFullscreenCalls!==1)throw Error('Duplicate startup request');await document.exitFullscreen();}`)
			waitForJSBool(t, page, `()=>!document.fullscreenElement && document.getElementById('vd-fullscreen-button').getAttribute('aria-pressed')==='false'`)
			page.Mouse.MustMoveTo(600, 400).MustClick(proto.InputMouseButtonLeft)
			page.MustEval(`()=>{
                if(startupFullscreenCalls!==1||document.fullscreenElement) throw Error('Manual fullscreen exit was overridden');
                if(fixtureErrors.length) throw Error(JSON.stringify(fixtureErrors));
            }`)
		})
	}
}
