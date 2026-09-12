package gamemaker

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-rod/rod"
)

func builderStudioRoute(w http.ResponseWriter, r *http.Request) bool {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	if r.URL.Path == "/_studio" {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, builderStudioFixture)
		return true
	}
	if strings.HasPrefix(r.URL.Path, "/img/") || strings.HasPrefix(r.URL.Path, "/fonts/") {
		http.ServeFile(w, r, filepath.Join("../../ui", filepath.FromSlash(strings.TrimPrefix(r.URL.Path, "/"))))
		return true
	}
	if strings.HasPrefix(r.URL.Path, "/_ui/") {
		path := strings.TrimPrefix(r.URL.Path, "/_ui/")
		if !filepath.IsLocal(filepath.FromSlash(path)) {
			http.Error(w, "invalid path", 400)
			return true
		}
		http.ServeFile(w, r, filepath.Join("../../ui", filepath.FromSlash(path)))
		return true
	}
	return false
}

func checkBuilderStudio(t *testing.T, browser *rod.Browser, url, report string) {
	t.Helper()
	page := browser.MustPage(url + "/_studio").Timeout(time.Minute)
	defer page.Close()
	page.MustSetViewport(1920, 1080, 1, false).MustWaitLoad()
	if err := page.Timeout(8 * time.Second).Wait(rod.Eval(`()=>!!window.GameMakerStudioApp?.instances.get('fixture')?.frame`)); err != nil {
		t.Fatalf("Studio failed to initialize: %s", page.MustEval(`()=>({errors:window.errors,body:document.body.innerText})`).String())
	}
	frame := page.MustElement("iframe").MustFrame()
	frame.MustWait(`()=>!!window.__AURAGO_GAME_TEST__?.scene?.builder`)
	page.MustWait(`()=>!document.querySelector('[data-gm-preview-loading]')`)
	page.MustElement(`[data-gm-action="scene_debug"]`).MustClick()
	frame.MustWait(`()=>__AURAGO_GAME_TEST__.scene.sceneDebugGraphics?.visible`)
	if !page.MustEval(`()=>document.querySelector('[data-gm-action="scene_debug"]').getAttribute('aria-pressed')==='true'`).Bool() {
		t.Fatal("debug control not pressed")
	}
	for _, theme := range []string{"standard", "fruity-dark", "fruity-light"} {
		page.MustEval(`theme=>{document.body.dataset.theme=theme==='standard'?'standard':'fruity';document.body.dataset.fruityMode=theme.endsWith('light')?'light':'dark'}`, theme)
		for _, density := range []string{"normal", "compact"} {
			page.MustEval(`density=>document.body.dataset.density=density`, density)
			for _, width := range []int{1920, 1366, 390} {
				height := 768
				if width == 1920 {
					height = 1080
				}
				if width == 390 {
					height = 844
				}
				page.MustSetViewport(width, height, 1, false)
				page.MustScreenshot(filepath.Join(report, fmt.Sprintf("studio-%s-%s-%d.png", theme, density, width)))
				if !page.MustEval(`()=>document.documentElement.scrollWidth<=innerWidth&&[...document.querySelectorAll('.gm-workspace,.gm-preview-pane')].every(e=>e.scrollWidth<=e.clientWidth+1)`).Bool() {
					t.Fatalf("Studio overflow %s %s %d", theme, density, width)
				}
			}
		}
	}
	page.MustSetViewport(1366, 768, 1, false)
	page.MustElement(`[data-gm-action="scene_debug"]`).MustClick()
	frame.MustWait(`()=>__AURAGO_GAME_TEST__.scene.sceneDebugGraphics?.visible===false`)
	// A wrong channel from the genuine parent must still be rejected.
	page.MustEval(`()=>{const s=GameMakerStudioApp.instances.get('fixture');s.frame.contentWindow.postMessage({source:'aurago-studio',type:'scene-debug',channel:'wrong',enabled:true},'*')}`)
	time.Sleep(100 * time.Millisecond)
	if frame.MustEval(`()=>__AURAGO_GAME_TEST__.scene.sceneDebugGraphics.visible`).Bool() {
		t.Fatal("wrong debug channel accepted")
	}
	page.MustEval(`()=>GameMakerStudioApp.dispose('fixture')`)
	if page.MustEval(`()=>GameMakerStudioApp.instances.has('fixture')`).Bool() {
		t.Fatal("Studio instance retained")
	}
	if errors := page.MustEval(`()=>JSON.stringify(window.errors)`).String(); errors != "[]" {
		t.Fatal(errors)
	}
	os.WriteFile(filepath.Join(report, "studio-acceptance.txt"), []byte("Real Studio modules; Standard/Fruity dark/light, normal/compact, 1920/1366/390; debug toggle and wrong-channel rejection passed.\n"), 0640)
}

const builderStudioFixture = `<!doctype html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><link rel="stylesheet" href="/_ui/css/desktop-shell.bundle.css"><link rel="stylesheet" href="/_ui/css/desktop-app-game-maker-studio.css"><style>html,body{margin:0;width:100%;height:100%;overflow:hidden}#host{position:absolute;inset:0}</style></head><body class="desktop-body" data-theme="standard"><div id="host"></div><script>window.errors=[];addEventListener('error',e=>errors.push(e.message));addEventListener('unhandledrejection',e=>errors.push(String(e.reason)));</script><script src="/_ui/js/desktop/apps/game-maker-studio-modals.js"></script><script src="/_ui/js/desktop/apps/game-maker-studio-api.js"></script><script src="/_ui/js/desktop/apps/game-maker-studio-preview.js"></script><script src="/_ui/js/desktop/apps/game-maker-studio.js"></script><script type="module">
const dict=await(await fetch('/_ui/lang/desktop/en.json')).json(),project={id:'fixture',name:'Parcel route',description:'Free code and generated map together',dimension:'2d',status:'ready',current_revision:1};
const t=key=>dict[key]||key,esc=value=>String(value??'').replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
// Only HTTP data and SSE are fixtures; rendering and preview lifecycle are real modules.
window.EventSource=class{addEventListener(){}close(){}};
const api=async path=>path.endsWith('/capabilities')?{enabled:true,skills_ready:true,allow_create:true,allow_edit:true,phaser_version:'4.2.1',three_version:'0.185.1'}:path.endsWith('/preview-token')?{url:'/',expires_at:new Date(Date.now()+600000).toISOString()}:path.endsWith('/projects')?{projects:[project]}:{project,messages:[]};
GameMakerStudioApp.render(document.getElementById('host'),'fixture',{api,t,esc});
</script></body></html>`
