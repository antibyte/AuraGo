package ui

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-rod/rod"

	"aurago/internal/desktop"
)

func TestLeuchtfeuerRegistration(t *testing.T) {
	t.Parallel()

	var app *desktop.AppManifest
	for _, candidate := range desktop.BuiltinApps() {
		if candidate.ID == "leuchtfeuer" {
			found := candidate
			app = &found
		}
	}
	if app == nil {
		t.Fatal("leuchtfeuer is not a builtin app")
	}
	if app.Name != "Leuchtfeuer" || app.Category != "games" || app.Entry != "builtin://leuchtfeuer" || app.Icon != "leuchtfeuer" || len(app.Requires) != 0 {
		t.Fatalf("manifest = %+v", *app)
	}
	if app.Metadata["logo_path"] != "/img/leuchtfeuer/icon.svg" {
		t.Fatalf("logo_path = %q", app.Metadata["logo_path"])
	}
	svg, err := Content.ReadFile("img/leuchtfeuer/icon.svg")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(svg), "<svg") || strings.Contains(string(svg), "<script") || strings.Contains(string(svg), "href") {
		t.Fatalf("icon must be a self-contained SVG: %.80q", svg)
	}

	loader := readDesktopAssetText(t, "js/desktop/core/module-loader.js")
	entryAt := strings.Index(loader, "leuchtfeuer: {")
	if entryAt < 0 {
		t.Fatal("module loader has no leuchtfeuer entry")
	}
	entry := loader[entryAt:]
	if end := strings.Index(entry, "},"); end > 0 {
		entry = entry[:end]
	}
	previous := 0
	for _, script := range []string{
		"/css/desktop-app-leuchtfeuer.css",
		"/js/desktop/apps/leuchtfeuer/world.js",
		"/js/desktop/apps/leuchtfeuer/sim.js",
		"/js/desktop/apps/leuchtfeuer/view.js",
		"/js/desktop/apps/leuchtfeuer/audio.js",
		"/js/desktop/apps/leuchtfeuer/app.js",
	} {
		index := strings.Index(entry, script)
		if index < 0 || index < previous {
			t.Fatalf("loader entry missing or reordered %s", script)
		}
		previous = index
	}
	routing := readDesktopAssetText(t, "js/desktop/core/menus-and-routing.js")
	if !strings.Contains(routing, "appId === 'leuchtfeuer'") || !strings.Contains(routing, "window.LeuchtfeuerApp.render") {
		t.Fatal("desktop routing does not mount Leuchtfeuer")
	}
	foundation := readDesktopAssetText(t, "js/desktop/core/desktop-foundation.js")
	for _, marker := range []string{"leuchtfeuer: 'LeuchtfeuerApp'", "leuchtfeuer: '/img/leuchtfeuer/icon.svg'"} {
		if !strings.Contains(foundation, marker) {
			t.Fatalf("desktop foundation missing %s", marker)
		}
	}
	shell := readDesktopAssetText(t, "js/desktop/core/window-shell-runtime.js")
	if !strings.Contains(shell, "leuchtfeuer: { width: 1100, height: 720 }") {
		t.Fatal("window size is not 1100 by 720")
	}
	bundle := readDesktopAssetText(t, "js/desktop/bundles/main.bundle.js")
	if !strings.Contains(bundle, "appId === 'leuchtfeuer'") || !strings.Contains(bundle, "leuchtfeuer: 'LeuchtfeuerApp'") {
		t.Fatal("main bundle is stale; run npm run build:ui")
	}
	css := readDesktopAssetText(t, "css/desktop-app-leuchtfeuer.css")
	if strings.Contains(css, "--vd-mat-") || strings.Contains(css, "!important") || strings.Contains(css, "animation:") {
		t.Fatal("Leuchtfeuer CSS must stay a theme-independent skin without animation or material tokens")
	}

	locales := []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"}
	var keys map[string]struct{}
	goals := map[string]string{}
	for _, locale := range locales {
		data, err := os.ReadFile(filepath.Join("lang", "leuchtfeuer", locale+".json"))
		if err != nil {
			t.Fatal(err)
		}
		var parsed map[string]string
		if err := json.Unmarshal(data, &parsed); err != nil {
			t.Fatal(err)
		}
		if keys == nil {
			keys = map[string]struct{}{}
			for key := range parsed {
				keys[key] = struct{}{}
			}
		}
		if len(parsed) != len(keys) {
			t.Fatalf("%s has %d keys, want %d", locale, len(parsed), len(keys))
		}
		for key := range keys {
			if strings.TrimSpace(parsed[key]) == "" || !strings.HasPrefix(key, "leuchtfeuer.") {
				t.Fatalf("%s missing %s", locale, key)
			}
		}
		goals[locale] = parsed["leuchtfeuer.goal"]
		desktopData, err := os.ReadFile(filepath.Join("lang", "desktop", locale+".json"))
		if err != nil {
			t.Fatal(err)
		}
		var labels map[string]string
		if err := json.Unmarshal(desktopData, &labels); err != nil {
			t.Fatal(err)
		}
		if labels["desktop.app_leuchtfeuer"] != "Leuchtfeuer" {
			t.Fatalf("%s desktop title = %q", locale, labels["desktop.app_leuchtfeuer"])
		}
	}
	if goals["de"] == goals["en"] || !strings.ContainsAny(goals["de"], "äöüÄÖÜ") {
		t.Fatalf("german goal = %q", goals["de"])
	}
	for locale, goal := range goals {
		if locale != "en" && goal == goals["en"] {
			t.Fatalf("%s goal copies English", locale)
		}
	}
}

// TestLeuchtfeuerBrowser plays one harbour round through the Desktop shell.
// Asset requests stay on the published Game Maker route and the lod0 allowlist.
func TestLeuchtfeuerBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	allowed, waypoints := leuchtfeuerAllowlist(t)
	var mu sync.Mutex
	var seen []string
	denied := 0
	glb := 0

	html := regexp.MustCompile(`(?s)<script\b[^>]*>.*?</script>`).ReplaceAllString(readDesktopAssetText(t, "desktop.html"), "")
	html = regexp.MustCompile(`\{\{[^}]*\}\}`).ReplaceAllString(html, "")
	html = strings.Replace(html, "</body>", `<script src="/js/shared/lazy-assets.js"></script><script src="/js/desktop/core/module-loader.js"></script><script src="/racer-shell.js"></script><script src="/testdata/aurora-fixture.js"></script></body>`, 1)
	shell := readDesktopAssetText(t, "js/desktop/bundles/main.bundle.js")
	cut := strings.LastIndex(shell, "    ensureDesktopRadialMenuAnchor();")
	if cut < 0 {
		t.Fatal("desktop startup seam missing")
	}
	shell = shell[:cut] + `window.aurora={state,loadIconManifest,applyDesktopSettings,renderIcons,renderStartApps,renderStartButtonIcon,wireShellChromeControls,bindViewportMetrics,handleDesktopKeydown,closeContextMenu,openApp,closeWindow};})();`
	fixture := readDesktopAssetText(t, "testdata/aurora-fixture.js")
	const fetchGate = "if(!path.startsWith('/api/')) return nativeFetch(url,options);"
	if !strings.Contains(fixture, fetchGate) {
		t.Fatal("aurora fixture fetch gate moved")
	}
	fixture = strings.Replace(fixture, fetchGate, fetchGate+"\n    if(path.startsWith('/api/game-maker/asset-packs/')||path.includes('/api/game-maker/asset-packs/')) return nativeFetch(url, options);", 1)

	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(Content)))
	mux.HandleFunc("/testdata/aurora-fixture.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
		fmt.Fprint(w, fixture)
	})
	mux.HandleFunc("/api/game-maker/asset-packs/", func(w http.ResponseWriter, r *http.Request) {
		rel := strings.TrimPrefix(r.URL.Path, "/api/game-maker/asset-packs/")
		rel = strings.TrimPrefix(rel, "/")
		mu.Lock()
		seen = append(seen, rel)
		_, ok := allowed[rel]
		if !ok || strings.Contains(rel, "..") {
			denied++
			mu.Unlock()
			http.NotFound(w, r)
			return
		}
		if strings.HasSuffix(rel, ".glb") {
			glb++
		}
		mu.Unlock()
		leuchtfeuerServePack(w, r, rel)
	})
	mux.HandleFunc("/fixture", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, html) })
	mux.HandleFunc("/racer-shell.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/javascript")
		fmt.Fprint(w, shell)
	})
	mux.HandleFunc("/fixture-words", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, readDesktopAssetText(t, "lang/desktop/de.json"))
	})
	mux.HandleFunc("/fixture-apps", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(desktop.BuiltinApps())
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	page := newSmokeBrowser(t).MustPage(server.URL + "/fixture")
	t.Cleanup(func() { page.Close() })
	page.MustSetViewport(1280, 800, 1, false)
	page.MustWaitLoad()
	page.MustEval(`async()=>{await fixtureReady;}`)
	page.MustEval(`async()=>{await fixtureOpen('leuchtfeuer');}`)

	var ready string
	deadline := time.Now().Add(45 * time.Second)
	for time.Now().Before(deadline) {
		ready = page.MustEval(`()=>{
			const err=document.querySelector('.lf-error');
			const goal=document.querySelector('.lf-goal');
			const loading=document.querySelector('.lf-loading');
			if(err&&!err.hidden)return 'error:'+err.textContent;
			if(goal&&loading&&loading.hidden&&document.querySelector('.lf-stage canvas')&&goal.textContent.includes('Die Lampe ist aus'))return 'ready';
			return 'wait';
		}`).String()
		if ready == "ready" {
			break
		}
		if strings.HasPrefix(ready, "error:") {
			detail := page.MustEval(`()=>document.querySelector('.lf-app')?.dataset.lfBoot||''`).String()
			console := page.MustEval(`()=> (window.fixtureErrors||[]).join('\n')`).String()
			mu.Lock()
			seenCopy := append([]string(nil), seen...)
			mu.Unlock()
			t.Fatalf("%s boot=%s console=%s seen=%s", ready, detail, console, strings.Join(seenCopy, " | "))
		}
		time.Sleep(200 * time.Millisecond)
	}
	if ready != "ready" {
		t.Fatalf("harbour did not finish loading: %s", ready)
	}

	probe := page.Timeout(20 * time.Second).MustEval(leuchtfeuerProbeJS).String()
	if probe != "ok" {
		t.Fatal(probe)
	}
	page.MustElement(".lf-pause-button").MustClick()
	sound := page.MustElement(".lf-dialog button[aria-pressed]")
	if sound.MustText() != "Ton aus" || lfAttr(sound, "aria-pressed") != "false" {
		t.Fatalf("sound toggle = %q pressed %q", sound.MustText(), lfAttr(sound, "aria-pressed"))
	}
	sound.MustClick()
	if sound.MustText() != "Ton an" || lfAttr(sound, "aria-pressed") != "true" {
		t.Fatalf("sound on = %q pressed %q", sound.MustText(), lfAttr(sound, "aria-pressed"))
	}
	page.MustElementR(".lf-dialog button", "^Weiter$").MustClick()

	played := page.Timeout(120*time.Second).MustEval(leuchtfeuerPlayJS, waypoints).String()
	if played != "ok" {
		t.Fatal(played)
	}
	win := page.MustElement(".lf-win")
	if win.MustProperty("hidden").Bool() {
		t.Fatal("win card stayed hidden")
	}
	if !strings.Contains(win.MustText(), "Die Lampe brennt") {
		t.Fatalf("win card = %q", win.MustText())
	}
	page.MustElement(".lf-win button").MustClick()
	cleared := page.Timeout(5 * time.Second).MustEval(`()=>{
		const shell=document.querySelector('.lf-app');
		const win=document.querySelector('.lf-win');
		if(!shell||shell.dataset.lfLit!=='0'||shell.dataset.lfParts!=='')return 'still '+shell.dataset.lfLit+' '+shell.dataset.lfParts;
		if(win&&!win.hidden)return 'win visible';
		if(sessionStorage.getItem('aurago.leuchtfeuer.v1'))return 'session kept';
		return 'ok';
	}`).String()
	if cleared != "ok" {
		t.Fatal(cleared)
	}
	page.MustEval(`()=>{aurora.closeContextMenu(true);[...aurora.state.windows.keys()].forEach(id=>aurora.closeWindow(id));}`)
	errors := page.MustEval(`()=> (window.fixtureErrors||[]).join('\n')`).String()
	if errors != "" {
		t.Fatal(errors)
	}
	mu.Lock()
	defer mu.Unlock()
	if denied > 0 {
		t.Fatalf("disallowed asset requests: %s", strings.Join(seen, "\n"))
	}
	if glb < 1 {
		t.Fatalf("no lod0 model was requested: %s", strings.Join(seen, "\n"))
	}
}

func leuchtfeuerServePack(w http.ResponseWriter, r *http.Request, rel string) {
	var disk string
	if strings.HasPrefix(rel, "runtime/") {
		disk = filepath.Join("..", "internal", "gamemaker", "runtime", filepath.FromSlash(strings.TrimPrefix(rel, "runtime/")))
	} else {
		disk = filepath.Join("..", "internal", "gamemaker", "asset_packs", filepath.FromSlash(rel))
	}
	file, err := os.Open(disk)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || info.IsDir() {
		http.NotFound(w, r)
		return
	}
	switch strings.ToLower(filepath.Ext(disk)) {
	case ".js":
		w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
	case ".json":
		w.Header().Set("Content-Type", "application/json")
	case ".glb":
		w.Header().Set("Content-Type", "model/gltf-binary")
	case ".wav":
		w.Header().Set("Content-Type", "audio/wav")
	case ".webp":
		w.Header().Set("Content-Type", "image/webp")
	}
	http.ServeContent(w, r, info.Name(), info.ModTime(), file)
}

func leuchtfeuerAllowlist(t *testing.T) (map[string]struct{}, string) {
	t.Helper()
	root := filepath.Join("..", "internal", "gamemaker", "asset_packs")
	models := lfReadManifest(t, filepath.Join(root, "aurago-pirates-3d", "manifest.json"))
	effects := lfReadManifest(t, filepath.Join(root, "aurago-effects", "manifest.json"))
	sounds := lfReadManifest(t, filepath.Join(root, "aurago-sounds", "manifest.json"))
	allowed := map[string]struct{}{
		"aurago-pirates-3d/manifest.json": {},
		"aurago-effects/manifest.json":    {},
		"aurago-sounds/manifest.json":     {},
	}
	for _, name := range []string{
		"three-0.186.1.module.min.js",
		"three-0.186.1.core.min.js",
		"aurago-three-assets-1.js",
		"aurago-effects-3d-1.js",
	} {
		allowed["runtime/"+name] = struct{}{}
	}
	ids := []string{
		"coast-island", "coast-beach-straight", "harbor-pier", "harbor-pier-corner",
		"harbor-lighthouse", "harbor-tavern", "ships-sloop", "equipment-treasure-chest",
		"equipment-lantern", "equipment-barrel", "nature-coconut-palm", "people-naval-sailor",
		"people-fisherman", "people-pirate-captain", "people-pirate-gunner", "animals-seagull",
	}
	byID := lfAssets(t, models)
	for _, id := range ids {
		asset := byID[id]
		lods, _ := asset["lods"].([]any)
		if len(lods) == 0 {
			t.Fatalf("%s has no lod", id)
		}
		first, _ := lods[0].(map[string]any)
		file, _ := first["file"].(string)
		lfAllowFile(t, allowed, "aurago-pirates-3d", file)
		lfAllowLibrary(t, allowed, "aurago-pirates-3d", asset["animation_library"])
	}
	effectIDs := []string{"coast", "sky-clear", "water-ocean", "color-grade", "fog-ground", "water-splash", "water-ripple", "fire", "magic", "pickup-glow"}
	soundIDs := []string{"ocean", "wind-gentle", "step-wood", "step-gravel", "pickup", "water-splash", "door-wood", "victory", "fire"}
	lfAllowAssetFiles(t, allowed, "aurago-effects", lfAssets(t, effects), effectIDs)
	lfAllowAssetFiles(t, allowed, "aurago-sounds", lfAssets(t, sounds), soundIDs)

	ship := byID["ships-sloop"]
	shapes, _ := lfMap(ship["collider"])["shapes"].([]any)
	if len(shapes) == 0 {
		t.Fatal("sloop has no collider shape")
	}
	deck := lfBox(t, shapes[0])
	const sloopX, sloopZ = 4.2, 19.0
	deckMinX := sloopX + deck.cx - deck.sx/2
	deckMaxX := sloopX + deck.cx + deck.sx/2
	deckMinZ := sloopZ + deck.cz - deck.sz/2
	deckMaxZ := sloopZ + deck.cz + deck.sz/2
	chest := lfBox(t, byID["equipment-treasure-chest"]["collider"])
	barrel := lfBox(t, byID["equipment-barrel"]["collider"])
	light := lfBox(t, byID["harbor-lighthouse"]["collider"])
	lampX := -5 + light.cx + light.sx/2 + 0.55
	const chestX, chestZ = -0.4, 14.5
	glassX, glassZ := 0.92, lfStandOff(chestX, chestZ, chest, 0.92, -1)
	oilX, oilZ := sloopX, lfStandOff(sloopX, 21, barrel, sloopX, -1)
	if glassX < 0.8 || glassX > 0.98 || glassZ < 11 || glassZ > 24 || math.Hypot(glassX-chestX, glassZ-chestZ) > 1.35 {
		t.Fatalf("glass approach (%v,%v) leaves reach or the safe pier lane", glassX, glassZ)
	}
	if oilX < deckMinX || oilX > deckMaxX || oilZ < deckMinZ || oilZ > deckMaxZ {
		t.Fatalf("oil approach (%v,%v) is off the deck", oilX, oilZ)
	}
	points := [][]float64{
		{0, 7.4}, {2.2, 2.2}, {4.2, 0.4},
		{2.2, 2.2}, {0.85, 7.4}, {0.85, 12.2}, {glassX, glassZ},
		{0.85, 16}, {0.85, 19}, {deckMinX + 0.35, 19}, {oilX, oilZ},
		{sloopX, 19}, {deckMinX + 0.35, 19}, {0.85, 19}, {0.85, 12}, {0.85, 7.4}, {0, 7.4}, {0, 2}, {lampX + 0.75, 2}, {lampX + 0.75, -1},
	}
	raw, err := json.Marshal(map[string]any{
		"approach": [][]float64{{0, 7.4}, {0.85, 7.4}, {0.85, 12}, {0.85, 19}},
		"wick":     points[0:3],
		"glass":    points[3:7],
		"oil":      points[7:11],
		"lamp":     points[11:],
		"parts":    []string{"wick", "glass", "oil", "lamp"},
	})
	if err != nil {
		t.Fatal(err)
	}
	return allowed, string(raw)
}

type lfBox3 struct{ cx, cy, cz, sx, sy, sz float64 }

func lfBox(t *testing.T, raw any) lfBox3 {
	t.Helper()
	box := lfMap(raw)
	cx, cy, cz := lfVec(t, box["center"])
	sx, sy, sz := lfVec(t, box["size"])
	if sx <= 0 || sy <= 0 || sz <= 0 {
		t.Fatalf("bad box %#v", raw)
	}
	return lfBox3{cx, cy, cz, sx, sy, sz}
}

// lfStandOff walks from the spot along Z until the point is outside the box and still in reach.
func lfStandOff(px, pz float64, box lfBox3, x, direction float64) float64 {
	minX := px + box.cx - box.sx/2 - 0.36
	maxX := px + box.cx + box.sx/2 + 0.36
	minZ := pz + box.cz - box.sz/2 - 0.36
	maxZ := pz + box.cz + box.sz/2 + 0.36
	for step := 0.0; step <= 1.25; step += 0.05 {
		z := pz + direction*step
		hit := x > minX && x < maxX && z > minZ && z < maxZ
		if !hit && math.Hypot(x-px, z-pz) <= 1.35 {
			return z
		}
	}
	return pz + direction*1.25
}

func lfAllowAssetFiles(t *testing.T, allowed map[string]struct{}, pack string, assets map[string]map[string]any, ids []string) {
	t.Helper()
	for _, id := range ids {
		asset, ok := assets[id]
		if !ok {
			t.Fatalf("missing %s asset %s", pack, id)
		}
		lfCollectFiles(t, allowed, pack, asset["files"])
	}
}

func lfCollectFiles(t *testing.T, allowed map[string]struct{}, pack string, raw any) {
	t.Helper()
	switch value := raw.(type) {
	case []any:
		for _, item := range value {
			lfCollectFiles(t, allowed, pack, item)
		}
	case map[string]any:
		if file, ok := value["file"].(string); ok {
			lfAllowFile(t, allowed, pack, file)
		}
		for _, item := range value {
			if _, ok := item.(string); !ok {
				lfCollectFiles(t, allowed, pack, item)
			}
		}
	}
}

func lfAllowLibrary(t *testing.T, allowed map[string]struct{}, pack string, raw any) {
	t.Helper()
	switch library := raw.(type) {
	case string:
		if library != "" {
			lfAllowFile(t, allowed, pack, library)
		}
	case map[string]any:
		if file, ok := library["file"].(string); ok && file != "" {
			lfAllowFile(t, allowed, pack, file)
		}
	}
}

func lfAllowFile(t *testing.T, allowed map[string]struct{}, pack, file string) {
	t.Helper()
	file = strings.TrimPrefix(filepath.ToSlash(file), "/")
	if file == "" || strings.Contains(file, "..") || strings.Contains(file, "://") {
		t.Fatalf("bad %s file %q", pack, file)
	}
	allowed[pack+"/"+file] = struct{}{}
}

func lfReadManifest(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatal(err)
	}
	return parsed
}

func lfAssets(t *testing.T, manifest map[string]any) map[string]map[string]any {
	t.Helper()
	items, _ := manifest["assets"].([]any)
	out := map[string]map[string]any{}
	for _, item := range items {
		asset, ok := item.(map[string]any)
		if !ok {
			continue
		}
		id, _ := asset["id"].(string)
		if id != "" {
			out[id] = asset
		}
	}
	if len(out) == 0 {
		t.Fatal("manifest has no assets")
	}
	return out
}

func lfMap(raw any) map[string]any {
	value, _ := raw.(map[string]any)
	if value == nil {
		return map[string]any{}
	}
	return value
}

func lfVec(t *testing.T, raw any) (float64, float64, float64) {
	t.Helper()
	items, ok := raw.([]any)
	if !ok || len(items) < 3 {
		t.Fatalf("vector = %#v", raw)
	}
	return lfFloat(items[0]), lfFloat(items[1]), lfFloat(items[2])
}

func lfFloat(raw any) float64 {
	value, _ := raw.(float64)
	return value
}

func lfAttr(element *rod.Element, name string) string {
	value := element.MustAttribute(name)
	if value == nil {
		return ""
	}
	return *value
}

const leuchtfeuerProbeJS = `async()=>{
	const shell=document.querySelector('.lf-app');
	const pose=()=>({x:Number(shell.dataset.lfX),z:Number(shell.dataset.lfZ),notice:shell.dataset.lfNotice||''});
	const down={};
	const setKey=(code,want)=>{
		if(!!down[code]===!!want)return;
		down[code]=!!want;
		document.dispatchEvent(new KeyboardEvent(want?'keydown':'keyup',{code,bubbles:true,cancelable:true}));
	};
	const release=()=>{for(const code of Object.keys(down))setKey(code,false);};
	const sleep=ms=>new Promise(r=>setTimeout(r,ms));
	const start=pose();
	setKey('KeyW',true);
	await sleep(450);
	release();
	await sleep(80);
	const moved=pose();
	if(!(moved.z>start.z+0.35))return 'W did not walk +Z from '+start.z+' to '+moved.z;
	setKey('KeyA',true);
	const deadline=performance.now()+4000;
	let wet=pose();
	while(performance.now()<deadline){
		await sleep(40);
		wet=pose();
		if(wet.notice==='leuchtfeuer.notice.water'&&Math.hypot(wet.x,wet.z-9)<=0.8)break;
	}
	release();
	if(wet.notice!=='leuchtfeuer.notice.water')return 'water notice '+wet.notice+' at '+wet.x+','+wet.z;
	if(Math.hypot(wet.x,wet.z-9)>0.8)return 'water spawn '+wet.x+','+wet.z;
	return 'ok';
}`

const leuchtfeuerPlayJS = `async(raw)=>{
	const plan=JSON.parse(raw);
	const shell=document.querySelector('.lf-app');
	const pose=()=>({
		x:Number(shell.dataset.lfX),z:Number(shell.dataset.lfZ),
		lit:shell.dataset.lfLit,notice:shell.dataset.lfNotice||'',parts:shell.dataset.lfParts||''
	});
	const down={};
	const setKey=(code,want)=>{
		if(!!down[code]===!!want)return;
		down[code]=!!want;
		document.dispatchEvent(new KeyboardEvent(want?'keydown':'keyup',{code,bubbles:true,cancelable:true}));
	};
	const release=()=>{for(const code of Object.keys(down))setKey(code,false);};
	const sleep=ms=>new Promise(r=>setTimeout(r,ms));
	async function walk(points,budget){
		const deadline=performance.now()+budget;
		let route=points.slice();
		let index=0;
		let left=false;
		while(performance.now()<deadline){
			const here=pose();
			const home=Math.abs(here.x)<0.5&&Math.abs(here.z-9)<0.5;
			if(!home)left=true;
			if(left&&home){
				route=(points[0][1]>11?plan.approach:[]).concat(points);
				index=0;
				left=false;
			}
			const target=route[index];
			const dx=target[0]-here.x;
			const dz=target[1]-here.z;
			if(Math.hypot(dx,dz)<0.4){
				release();
				index++;
				if(index>=points.length)return;
				continue;
			}
			setKey('KeyA',dx>0.25);
			setKey('KeyD',dx<-0.25);
			setKey('KeyW',dz>0.25);
			setKey('KeyS',dz<-0.25);
			await sleep(40);
		}
		release();
		throw new Error('stuck at '+JSON.stringify(pose())+' going to '+JSON.stringify(route[index]));
	}
	async function take(part){
		for(let n=0;n<6;n++){
			shell.dispatchEvent(new KeyboardEvent('keydown',{code:'KeyE',bubbles:true,cancelable:true}));
			await sleep(250);
			const here=pose();
			if(part==='lamp'?here.lit==='1':here.parts.split(',').includes(part))return;
		}
		throw new Error('missed '+part+' at '+JSON.stringify(pose()));
	}
	for(const part of plan.parts){
		await walk(plan[part],30000);
		await take(part);
	}
	const done=pose();
	if(done.lit!=='1'||done.parts!=='glass,wick,oil')return 'round '+JSON.stringify(done);
	const win=document.querySelector('.lf-win');
	if(!win||win.hidden)return 'no win card';
	return 'ok';
}`
