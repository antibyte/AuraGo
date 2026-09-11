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

// Render the real asset browser and desktop materials; only its HTTP/context host is a fixture.
func presentationStudioRoute(w http.ResponseWriter, r *http.Request) bool {
	if r.URL.Path == "/studio" {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, presentationStudioFixture)
		return true
	}
	files := map[string]string{
		"/studio.js":  "js/desktop/apps/game-maker-studio-assets.js",
		"/studio.css": "css/desktop-app-game-maker-studio.css",
		"/shell.css":  "css/desktop-shell.bundle.css",
		"/en.json":    "lang/desktop/en.json",
	}
	if file, ok := files[r.URL.Path]; ok {
		b, err := os.ReadFile(filepath.Join("..", "..", "ui", filepath.FromSlash(file)))
		if err != nil {
			http.Error(w, err.Error(), 500)
			return true
		}
		kind := "application/json"
		if strings.HasSuffix(file, ".js") {
			kind = "text/javascript"
		}
		if strings.HasSuffix(file, ".css") {
			kind = "text/css"
		}
		w.Header().Set("Content-Type", kind)
		w.Write(b)
		return true
	}
	return false
}

func checkPresentationStudio(t *testing.T, browser *rod.Browser, url, report string) {
	page := browser.MustPage(url + "/studio").Timeout(5 * time.Minute)
	defer page.Close()
	page.MustSetViewport(1366, 768, 1, false)
	page.MustWaitLoad()
	page.MustWait(`()=>document.querySelectorAll('[data-presentation]').length===92`)
	page.MustElement(`[data-asset-kind]`).MustSelect("Effects")
	page.MustElement(`[data-presentation="forest-rain"]`).MustClick()
	page.MustWait(`()=>document.querySelector('[data-fx-stage] canvas')&&document.querySelector('[data-fx-status]').textContent!=='Loading…'`)
	for _, theme := range []string{"standard", "fruity-dark", "fruity-light"} {
		page.MustEval(`theme=>{document.body.dataset.theme=theme==='standard'?'standard':'fruity';document.body.dataset.fruityMode=theme.endsWith('light')?'light':'dark'}`, theme)
		for _, dim := range []string{"3d", "2d"} {
			page.MustElement(`[data-fx-dimension]`).MustSelect(dim)
			page.MustWait(`()=>document.querySelectorAll('[data-fx-stage] canvas').length===1`)
			page.MustScreenshot(filepath.Join(report, "studio-"+theme+"-"+dim+".png"))
		}
	}
	page.MustElement(`[data-select-presentation="forest-rain"]`).MustClick()
	if os.Getenv("GAMEMAKER_PRESENTATION_GALLERY") == "1" {
		manifest, err := readPresentationPack(EffectsPackID)
		if err != nil {
			t.Fatal(err)
		}
		for _, asset := range manifest.Assets {
			page.MustElement(`[data-presentation="` + asset.ID + `"]`).MustClick()
			for _, dim := range []string{"3d", "2d"} {
				page.MustElement(`[data-fx-dimension]`).MustSelect(dim)
				page.MustWait(`()=>document.querySelectorAll('[data-fx-stage] canvas').length===1&&document.querySelector('[data-fx-status]').textContent==='Click the scene to trigger the effect'`)
				frames := 25
				if asset.ID == "hit-flash" || asset.ID == "muzzle-flash" {
					frames = 1
				}
				page.MustEval(`n=>new Promise(resolve=>{function tick(){if(--n<=0)resolve();else requestAnimationFrame(tick)}requestAnimationFrame(tick)})`, frames)
				page.MustScreenshot(filepath.Join(report, "effect-"+dim+"-"+asset.ID+".png"))
			}
			t.Log("rendered", asset.ID)
		}
		for _, quality := range []string{"Low", "Medium", "High", "Auto"} {
			page.MustElement(`[data-fx-quality]`).MustSelect(quality)
			page.MustElement(`[data-fx-play]`).MustClick()
		}
	}
	page.MustElement(`[data-asset-kind]`).MustSelect("Sounds")
	page.MustElement(`[data-select-presentation="rifle"]`).MustClick()
	page.MustElement(`[data-presentation="rifle"]`).MustClick()
	page.MustElement(`[data-fx-play]`).MustClick()
	page.MustWait(`()=>document.querySelector('audio').readyState>=2`)
	if !page.MustEval(`()=>window.state.selectedPresentation.environment==='forest-rain'&&window.state.selectedPresentation.sounds.some(s=>s.sound==='rifle'&&s.event==='shot')`).Bool() {
		t.Fatal("studio selection lost")
	}
	page.MustEval(`()=>{window.previousAudio=document.querySelector('audio')}`)
	page.MustElement(`[data-presentation="rain-heavy"]`).MustClick()
	if !page.MustEval(`()=>window.previousAudio.paused&&!window.previousAudio.getAttribute('src')&&document.querySelectorAll('audio').length===1`).Bool() {
		t.Fatal("sound preview leaked")
	}
	page.MustSetViewport(390, 844, 2, true)
	page.MustScreenshot(filepath.Join(report, "studio-touch.png"))
	if !page.MustEval(`()=>document.documentElement.scrollWidth<=innerWidth`).Bool() {
		t.Fatal("studio horizontal overflow")
	}
	page.MustEval(`()=>window.state.assetBrowserCleanup()`)
	if e := page.MustEval(`()=>JSON.stringify(window.errors)`).String(); e != "[]" {
		t.Fatal(e)
	}
}

const presentationStudioFixture = `<!doctype html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><link rel="stylesheet" href="/shell.css"><link rel="stylesheet" href="/studio.css"><style>html,body{margin:0;width:100%;height:100%;overflow:hidden}#host{position:absolute;inset:0}</style></head><body class="desktop-body" data-theme="standard"><div id="host" class="gm-studio"></div><script>window.errors=[];addEventListener('error',e=>window.errors.push(e.message));addEventListener('unhandledrejection',e=>window.errors.push(String(e.reason)))</script><script src="/studio.js"></script><script type="module">
const dict=await(await fetch('/en.json')).json(),host=document.getElementById('host');
const t=key=>dict[key]||key,esc=value=>String(value).replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
window.state={container:host,context:{t,esc},api:{assetPacks:async()=>({packs:[{id:'aurago-effects',kind:'effect',tags:[]},{id:'aurago-sounds',kind:'audio',tags:[]}]}),modelPack:async id=>(await fetch('/packs/'+id+'/manifest.json')).json(),assetPackFileURL:(id,file)=>id==='runtime'?'/runtime/'+file:'/packs/'+id+'/'+file}};
GameMakerStudioAssets.show(window.state,{showModal(state,html,setup){host.innerHTML='<div class="gm-modal-layer">'+html+'</div>';setup(host.firstChild)},modalError(layer,message){window.errors.push(message)}});
</script></body></html>`
