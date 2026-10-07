package ui

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"aurago/internal/desktop"
)

func TestDesktopVideoStudioStartMenuBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	html := readDesktopAssetText(t, "desktop.html")
	html = regexp.MustCompile(`(?s)<script\b[^>]*>.*?</script>`).ReplaceAllString(html, "")
	html = regexp.MustCompile(`\{\{[^}]*\}\}`).ReplaceAllString(html, "")
	html = strings.Replace(html, "</body>", `<script src="/js/shared/lazy-assets.js"></script><script src="/js/desktop/core/module-loader.js"></script><script src="/launcher-shell.js"></script></body>`, 1)
	shell := readDesktopAssetText(t, "js/desktop/bundles/main.bundle.js")
	cut := strings.LastIndex(shell, "    ensureDesktopRadialMenuAnchor();")
	if cut < 0 {
		t.Fatal("desktop startup seam missing")
	}
	apps, err := json.Marshal(desktop.BuiltinApps())
	if err != nil {
		t.Fatal(err)
	}
	shell = shell[:cut] + `
window.fixtureErrors=[];addEventListener('error',e=>fixtureErrors.push(e.message));addEventListener('unhandledrejection',e=>fixtureErrors.push(String(e.reason)));
state.bootstrap={enabled:true,readonly:false,builtin_apps:` + string(apps) + `,apps:[],widgets:[],shortcuts:[],desktop_files:[],settings:{'windows.restore_session':false}};
document.getElementById('vd-disabled').hidden=true;document.body.dataset.animations='false';
window.launcherFixture={state,openStartMenu,selectStartCategory};
window.launcherReady=(async()=>{const words=await(await fetch('/lang/desktop/en.json')).json();window.t=key=>words[key]||key;window.i18n={t:window.t};await loadIconManifest();})();})();`
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(Content)))
	mux.HandleFunc("/fixture", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, html)
	})
	mux.HandleFunc("/launcher-shell.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/javascript")
		fmt.Fprint(w, shell)
	})
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api/desktop/video-studio/status" {
			fmt.Fprint(w, `{"enabled":false,"desktop_enabled":true,"ffmpeg_ready":false,"issue":"video_studio_disabled","generation":{"enabled":false}}`)
			return
		}
		fmt.Fprint(w, `{}`)
	})
	server := httptest.NewServer(mux)
	defer server.Close()
	page := newSmokeBrowser(t).MustPage().Timeout(45 * time.Second)
	defer page.MustClose()
	for _, theme := range []string{"standard", "fruity"} {
		t.Run(theme, func(t *testing.T) {
			page.MustNavigate(server.URL + "/fixture").MustWaitLoad()
			page.MustEval(`async theme=>{await launcherReady;document.body.dataset.theme=theme;launcherFixture.openStartMenu();launcherFixture.selectStartCategory('creative');}`, theme)
			page.MustElement(`#vd-start-apps [data-app-id="video-studio"]`).MustClick()
			waitForJSBool(t, page, `()=>document.querySelector('[data-app-id="video-studio"] .vs-app [data-disabled]')?.textContent.includes('Video Studio is disabled')`)
			if !page.MustEval(`()=>document.querySelector('.vs-app [data-disabled]').textContent.includes('Video Studio is disabled') && fixtureErrors.length===0`).Bool() {
				t.Fatalf("launcher did not reach disabled-feature guidance: %s", page.MustEval(`()=>JSON.stringify({errors:fixtureErrors,text:document.querySelector('.vs-app')?.textContent})`).Str())
			}
		})
	}
}
