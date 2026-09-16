package ui

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
)

func TestGameMakerWorldAssetBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	bin, ok := browserExecutable()
	if !ok {
		t.Fatal("Chrome required")
	}
	var catalog []map[string]any
	for _, id := range []string{"aurago-pirates-3d", "aurago-pirates-topdown", "aurago-pirates-side", "aurago-isometric"} {
		b, err := os.ReadFile("../internal/gamemaker/asset_packs/" + id + "/manifest.json")
		if err != nil {
			t.Fatal(err)
		}
		var p map[string]any
		if err := json.Unmarshal(b, &p); err != nil {
			t.Fatal(err)
		}
		catalog = append(catalog, map[string]any{"id": id, "kind": p["kind"], "name": p["name"], "description": p["description"], "tags": p["tags"], "manifest_schema": p["schema_version"]})
	}
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.Dir(".")))
	mux.Handle("/packs/runtime/", http.StripPrefix("/packs/runtime/", http.FileServer(http.Dir("../internal/gamemaker/runtime"))))
	mux.Handle("/packs/", http.StripPrefix("/packs/", http.FileServer(http.Dir("../internal/gamemaker/asset_packs"))))
	mux.HandleFunc("/catalog", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"packs": catalog})
	})
	mux.HandleFunc("/fixture", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<!doctype html><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<link rel="stylesheet" href="/css/desktop-shell.bundle.css"><link rel="stylesheet" href="/css/desktop-app-game-maker-studio.css">
<style>html,body,#app{margin:0;width:100%;height:100%;overflow:hidden}</style>
<body class="desktop-body" data-theme="standard"><div class="gm-studio" id="app"></div>
<script src="/js/desktop/apps/game-maker-studio-models.js"></script><script src="/js/desktop/apps/game-maker-studio-assets.js"></script>
<script>
window.errors=[];addEventListener('error',e=>errors.push(e.message));addEventListener('unhandledrejection',e=>errors.push(String(e.reason)));
window.liveURLs=new Set();const create=URL.createObjectURL.bind(URL),revoke=URL.revokeObjectURL.bind(URL);URL.createObjectURL=b=>{const url=create(b);liveURLs.add(url);return url};URL.revokeObjectURL=url=>{liveURLs.delete(url);revoke(url)};
window.liveIntervals=new Set();const interval=window.setInterval.bind(window),clear=window.clearInterval.bind(window);window.setInterval=(...a)=>{const id=interval(...a);liveIntervals.add(id);return id};window.clearInterval=id=>{liveIntervals.delete(id);clear(id)};
(async()=>{const labels=await(await fetch('/lang/desktop/de.json')).json();window.state={container:document.getElementById('app'),selectedAssetSelections:[],selectedAssetDimensions:{},context:{esc:s=>String(s??'').replace(/[&<>"']/g,c=>'&#'+c.charCodeAt(0)+';'),t:k=>labels[k]||k},api:{assetPacks:o=>fetch('/catalog',o).then(r=>r.json()),modelPack:(id,o)=>fetch('/packs/'+id+'/manifest.json',o).then(r=>r.json()),assetPackFileURL:(id,f)=>'/packs/'+id+'/'+f}};GameMakerStudioAssets.show(state,{showModal(s,html,ready){const layer=document.createElement('div');layer.className='gm-modal-layer';layer.innerHTML=html;s.container.append(layer);ready(layer)},modalError(_,message){errors.push(message)}})})();
</script>`)
	})
	server := httptest.NewServer(mux)
	defer server.Close()
	launch := launcher.New().Bin(bin).Headless(true).NoSandbox(true).Set("enable-unsafe-swiftshader")
	browser := rod.New().ControlURL(launch.MustLaunch()).MustConnect()
	defer launch.Cleanup()
	defer browser.Close()
	page := browser.MustPage(server.URL + "/fixture").MustWaitLoad().Timeout(70 * time.Second)
	page.MustSetViewport(1920, 1080, 1, false)
	page.MustWait(`()=>document.querySelectorAll('[data-entry]').length===600`)
	choose := func(selector, value string) {
		t.Helper()
		if err := page.MustElement(selector).Select([]string{`option[value="` + value + `"]`}, true, rod.SelectorTypeCSSSector); err != nil {
			t.Fatal(err)
		}
	}
	// Same model ID in two packs must remain two distinct selections.
	for _, pack := range []string{"aurago-pirates-topdown", "aurago-pirates-side"} {
		choose(`[data-asset-category]`, pack)
		page.MustElement(`[data-asset-search]`).MustSelectAllText().MustInput("diver brass")
		page.MustElement(`[data-select-entry="people-diver-brass"][data-entry-pack="` + pack + `"]`).MustClick()
	}
	if n := page.MustEval(`()=>state.selectedAssetSelections.length`).Int(); n != 2 {
		t.Fatalf("selection collision: %d", n)
	}
	page.MustElement(`[data-entry="people-diver-brass"][data-entry-pack="aurago-pirates-side"]`).MustClick()
	page.MustWait(`()=>document.querySelector('[data-status]')?.textContent.includes('96')===true`)
	page.MustElement(`[data-action]`).MustSelect("swim")
	page.MustElement(`[data-play]`).MustClick()
	page.MustWait(`()=>liveIntervals.size===1`)
	choose(`[data-asset-category]`, "aurago-isometric")
	page.MustElement(`[data-asset-search]`).MustSelectAllText().MustInput("adventurer")
	page.MustElement(`[data-entry="people-adventurer"]`).MustClick()
	page.MustWait(`()=>document.querySelector('[data-elevation]')!==null&&liveIntervals.size===0`)
	page.MustElement(`[data-action]`).MustSelect("walk")
	page.MustElement(`[data-play]`).MustClick()
	choose(`[data-asset-category]`, "aurago-pirates-3d")
	page.MustElement(`[data-asset-search]`).MustSelectAllText().MustInput("sloop")
	page.MustElement(`[data-entry="ships-sloop"]`).MustClick()
	page.MustWait(`()=>document.querySelector('[data-model-status]')?.textContent===state.context.t('game_maker.model_rotate')`)
	page.MustElement(`[data-model-animation]`).MustSelect("damaged")
	page.MustEval(`()=>new Promise(resolve=>setTimeout(resolve,600))`)
	page.MustElement(`[data-model-lod]`).MustSelect("LOD 2")
	if root := os.Getenv("GAMEMAKER_WORLD_REPORTS"); root != "" {
		page.MustScreenshot(filepath.Join(root, "studio-ship-damaged.png"))
	}
	choose(`[data-asset-category]`, "aurago-isometric")
	page.MustElement(`[data-asset-search]`).MustSelectAllText().MustInput("adventurer")
	page.MustElement(`[data-entry="people-adventurer"]`).MustClick()
	page.MustWait(`()=>document.querySelector('[data-elevation]')!==null`)
	for _, theme := range []string{"standard", "light", "dark"} {
		page.MustEval(`theme=>{document.body.dataset.theme=theme==='standard'?'standard':'fruity';document.body.dataset.fruityMode=theme==='standard'?'dark':theme}`, theme)
		for _, width := range []int{1920, 390} {
			page.MustSetViewport(width, 1080, 1, width == 390)
			if !page.MustEval(`()=>document.documentElement.scrollWidth<=innerWidth`).Bool() {
				t.Fatalf("horizontal overflow: %s %d", theme, width)
			}
			if root := os.Getenv("GAMEMAKER_WORLD_REPORTS"); root != "" {
				os.MkdirAll(root, 0750)
				page.MustScreenshot(filepath.Join(root, fmt.Sprintf("studio-%s-%d.png", theme, width)))
			}
		}
	}
	page.MustEval(`()=>state.assetBrowserCleanup()`)
	if !page.MustEval(`()=>liveURLs.size===0&&liveIntervals.size===0&&errors.length===0`).Bool() {
		t.Fatal(page.MustEval(`()=>({urls:liveURLs.size,timers:liveIntervals.size,errors})`).JSON("", " "))
	}
}
