package ui

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync/atomic"
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
	var gatedRequests atomic.Int32
	var studioEnabled atomic.Bool
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
			if studioEnabled.Load() {
				fmt.Fprint(w, `{"enabled":true,"desktop_enabled":true,"ffmpeg_ready":true,"generation":{"enabled":false}}`)
				return
			}
			fmt.Fprint(w, `{"enabled":false,"desktop_enabled":true,"ffmpeg_ready":false,"issue":"video_studio_disabled","generation":{"enabled":false}}`)
			return
		}
		if studioEnabled.Load() && r.URL.Path == "/api/desktop/video-studio/projects" && r.Method == http.MethodGet {
			fmt.Fprint(w, `{"projects":[]}`)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/desktop/video-studio/") {
			// Mirror the real server: every gated endpoint refuses while the feature is off.
			gatedRequests.Add(1)
			w.WriteHeader(http.StatusServiceUnavailable)
			fmt.Fprint(w, `{"error":"video_studio_disabled","code":"video_studio_disabled","message":"Video Studio is disabled in configuration."}`)
			return
		}
		fmt.Fprint(w, `{}`)
	})
	server := httptest.NewServer(mux)
	defer server.Close()
	page := newSmokeBrowser(t).MustPage().Timeout(45 * time.Second)
	defer page.MustClose()
	// The empty-project card must be visible, not merely present: 1910x760 leaves the preview stage only ~190px tall.
	cardVisible := `()=>{const app=document.querySelector('.vs-app'),stage=app?.querySelector('.vs-preview-stage'),button=app?.querySelector('[data-no-project] [data-action="new-project"]'),heading=app?.querySelector('[data-no-project] h2');if(!stage||!button||!heading||!stage.classList.contains('vs-no-project'))return false;const s=stage.getBoundingClientRect(),inside=r=>r.width>0&&r.height>0&&r.left>=Math.max(0,s.left)&&r.top>=Math.max(0,s.top)&&r.right<=Math.min(innerWidth,s.right)&&r.bottom<=Math.min(innerHeight,s.bottom),hits=el=>{const r=el.getBoundingClientRect(),hit=document.elementFromPoint(r.x+r.width/2,r.y+r.height/2);return !!hit&&el.contains(hit)};return inside(button.getBoundingClientRect())&&inside(heading.getBoundingClientRect())&&hits(button)&&hits(heading)&&getComputedStyle(app.querySelector('[data-preview]')).display==='none'&&getComputedStyle(app.querySelector('[data-preview-empty]')).display==='none'}`
	cardGeometry := `()=>{const app=document.querySelector('.vs-app'),rect=sel=>app?.querySelector(sel)?.getBoundingClientRect().toJSON();return JSON.stringify({viewport:[innerWidth,innerHeight],window:document.querySelector('[data-app-id="video-studio"]')?.getBoundingClientRect().toJSON(),stage:rect('.vs-preview-stage'),mat:rect('.vs-preview-mat'),card:rect('[data-no-project]'),button:rect('[data-no-project] [data-action="new-project"]'),stageClass:app?.querySelector('.vs-preview-stage')?.className})}`
	for _, scenario := range []string{"disabled", "empty"} {
		for _, viewport := range [][2]int{{1280, 800}, {1910, 760}} {
			for _, theme := range []string{"standard", "fruity"} {
				t.Run(fmt.Sprintf("%s/%dx%d/%s", scenario, viewport[0], viewport[1], theme), func(t *testing.T) {
					studioEnabled.Store(scenario == "empty")
					gatedRequests.Store(0)
					page.MustSetViewport(viewport[0], viewport[1], 1, false)
					page.MustNavigate(server.URL + "/fixture").MustWaitLoad()
					page.MustEval(`async theme=>{await launcherReady;document.body.dataset.theme=theme;launcherFixture.openStartMenu();launcherFixture.selectStartCategory('creative');}`, theme)
					page.MustElement(`#vd-start-apps [data-app-id="video-studio"]`).MustClick()
					waitForJSBool(t, page, `()=>!!document.querySelector('.vs-app [data-no-project]')`)
					if scenario == "disabled" {
						waitForJSBool(t, page, `()=>document.querySelector('[data-app-id="video-studio"] .vs-app [data-disabled]')?.textContent.includes('Video Studio is disabled')`)
						if !page.MustEval(`()=>document.querySelector('.vs-app [data-disabled]').textContent.includes('Video Studio is disabled') && fixtureErrors.length===0`).Bool() {
							t.Fatalf("launcher did not reach disabled-feature guidance: %s", page.MustEval(`()=>JSON.stringify({errors:fixtureErrors,text:document.querySelector('.vs-app')?.textContent})`).Str())
						}
						// A switched-off feature is not a load failure: no error notice, a calm empty state, and no gated calls.
						if !page.MustEval(`()=>{const app=document.querySelector('.vs-app');return app.querySelector('[data-notice]').hidden && app.querySelector('[data-no-project] [data-action="new-project"]').disabled && app.querySelector('[data-canvas]').options.length>0 && ['new-project','save','export','upload','browse','add-media','add-title'].every(a=>[...app.querySelectorAll('[data-action="'+a+'"]')].every(b=>b.disabled))}`).Bool() || gatedRequests.Load() != 0 {
							t.Fatalf("disabled studio rendered as a failure: gated=%d %s", gatedRequests.Load(), page.MustEval(`()=>JSON.stringify({notice:document.querySelector('.vs-app [data-notice]')?.textContent,canvas:document.querySelector('.vs-app [data-canvas]')?.options.length,empty:!!document.querySelector('.vs-app [data-no-project]')})`).Str())
						}
					} else if !page.MustEval(`()=>{const app=document.querySelector('.vs-app');return app.querySelector('[data-notice]').hidden && app.querySelector('[data-disabled]').hidden && getComputedStyle(app.querySelector('[data-disabled]')).display==='none' && !app.querySelector('[data-no-project] [data-action="new-project"]').disabled && fixtureErrors.length===0}`).Bool() || gatedRequests.Load() != 0 {
						t.Fatalf("enabled studio without projects did not offer an active New project action: gated=%d %s", gatedRequests.Load(), page.MustEval(`()=>JSON.stringify({errors:fixtureErrors,notice:document.querySelector('.vs-app [data-notice]')?.textContent})`).Str())
					}
					if !page.MustEval(cardVisible).Bool() {
						t.Fatalf("empty-project card is not visible: %s", page.MustEval(cardGeometry).Str())
					}
				})
			}
		}
	}
}
