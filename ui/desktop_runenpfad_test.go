package ui

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"aurago/internal/desktop"
)

var runenpfadLocales = []string{
	"cs", "da", "de", "el", "en", "es", "fr", "hi",
	"it", "ja", "nl", "no", "pl", "pt", "sv", "zh",
}

func TestRunenpfadRegistration(t *testing.T) {
	foundIdx := -1
	apps := desktop.BuiltinApps()
	for i := range apps {
		if apps[i].ID == "runenpfad" {
			foundIdx = i
			break
		}
	}
	if foundIdx < 0 {
		t.Fatal("builtin app runenpfad is not registered")
	}
	found := apps[foundIdx]
	if found.Category != "games" {
		t.Fatalf("expected runenpfad category games, got %q", found.Category)
	}
	if found.Runtime != desktop.BuiltinRuntime {
		t.Fatalf("expected runenpfad runtime builtin, got %q", found.Runtime)
	}
	if found.Metadata["open_maximized"] != "true" {
		t.Fatalf("expected runenpfad open_maximized=true, got %q", found.Metadata["open_maximized"])
	}
	if found.Metadata["logo_path"] != "/img/runenpfad/icon.svg" {
		t.Fatalf("expected runenpfad logo_path /img/runenpfad/icon.svg, got %q", found.Metadata["logo_path"])
	}

	iconBytes, err := fs.ReadFile(Content, "img/runenpfad/icon.svg")
	if err != nil {
		t.Fatalf("read icon.svg: %v", err)
	}
	if len(iconBytes) == 0 || len(iconBytes) >= 8192 {
		t.Fatalf("expected self-contained icon size < 8192 bytes, got %d", len(iconBytes))
	}
	iconText := strings.TrimSpace(string(iconBytes))
	if !strings.HasPrefix(iconText, "<svg") || !strings.Contains(iconText, `viewBox="0 0 64 64"`) {
		t.Fatal("icon.svg must be an inline 64x64 svg document")
	}
	lowerIcon := strings.ToLower(iconText)
	if strings.Contains(lowerIcon, "<script") || strings.Contains(lowerIcon, "href=") {
		t.Fatal("icon.svg must not contain scripts or external href references")
	}

	checks := []struct {
		path string
		want []string
	}{
		{"js/desktop/core/module-loader.js", []string{"runenpfad", "/css/desktop-app-runenpfad.css", "/js/desktop/apps/runenpfad/levels.js", "/js/desktop/apps/runenpfad/app.js"}},
		{"js/desktop/core/menus-and-routing.js", []string{"if (appId === 'runenpfad')", "RunenpfadApp.render"}},
		{"js/desktop/core/desktop-foundation.js", []string{"runenpfad: '/img/runenpfad/icon.svg'", "runenpfad: 'RunenpfadApp'"}},
		{"js/desktop/core/window-shell-runtime.js", []string{"runenpfad: { width: 1180, height: 740 }"}},
		{"js/desktop/bundles/main.bundle.js", []string{"if (appId === 'runenpfad')", "runenpfad: 'RunenpfadApp'"}},
		{"css/desktop-app-runenpfad.css", []string{".rp-app", ".rp-canvas", ".rp-hud"}},
		{"js/desktop/apps/runenpfad/levels.js", []string{"RunenpfadLevels", "CAMPAIGN_LEVELS", "validateSpriteManifests"}},
		{"js/desktop/apps/runenpfad/sim.js", []string{"RunenpfadSim", "aurago.runenpfad.v1", "stepSimulation"}},
		{"js/desktop/apps/runenpfad/audio.js", []string{"RunenpfadAudio", "validateSoundManifest", "/api/game-maker/asset-packs/aurago-sounds/"}},
		{"js/desktop/apps/runenpfad/view.js", []string{"RunenpfadView", "createRenderer"}},
		{"js/desktop/apps/runenpfad/app.js", []string{"RunenpfadApp", "/api/game-maker/asset-packs/"}},
	}
	for _, tc := range checks {
		text := readDesktopAssetText(t, tc.path)
		for _, needle := range tc.want {
			if !strings.Contains(text, needle) {
				t.Fatalf("%s missing expected marker %q", tc.path, needle)
			}
		}
		if strings.HasPrefix(tc.path, "js/desktop/apps/runenpfad/") {
			if strings.Contains(text, ".innerHTML") || strings.Contains(text, "http://") || strings.Contains(text, "https://") {
				t.Fatalf("%s must not use innerHTML or external URLs", tc.path)
			}
		}
	}

	requiredKeys := []string{
		"runenpfad.title",
		"runenpfad.hud_level",
		"runenpfad.shop_title",
		"runenpfad.win_title",
		"runenpfad.lose_title",
		"runenpfad.error_assets",
	}
	for _, lang := range runenpfadLocales {
		rpPath := path.Join("lang", "runenpfad", lang+".json")
		rpBytes, err := fs.ReadFile(Content, rpPath)
		if err != nil {
			t.Fatalf("read %s: %v", rpPath, err)
		}
		var dict map[string]string
		if err := json.Unmarshal(rpBytes, &dict); err != nil {
			t.Fatalf("unmarshal %s: %v", rpPath, err)
		}
		for _, k := range requiredKeys {
			if strings.TrimSpace(dict[k]) == "" {
				t.Fatalf("%s missing non-empty key %q", rpPath, k)
			}
		}

		deskPath := path.Join("lang", "desktop", lang+".json")
		deskBytes, err := fs.ReadFile(Content, deskPath)
		if err != nil {
			t.Fatalf("read %s: %v", deskPath, err)
		}
		var deskDict map[string]string
		if err := json.Unmarshal(deskBytes, &deskDict); err != nil {
			t.Fatalf("unmarshal %s: %v", deskPath, err)
		}
		if deskDict["desktop.app_runenpfad"] != "Runenpfad" {
			t.Fatalf("%s missing desktop.app_runenpfad", deskPath)
		}
	}
}

func TestDesktopRunenpfadBrowser(t *testing.T) {
	if os.Getenv("AURAGO_RUN_BROWSER_SMOKE") != "1" {
		t.Skip("set AURAGO_RUN_BROWSER_SMOKE=1 to run the Runenpfad browser test")
	}

	allowedPacks := map[string]bool{
		"jump-and-run":               true,
		"human-characters-animated":  true,
		"monsters-animals-animated":  true,
		"robots-drones-animated":     true,
		"assets-animated":            true,
		"nature-assets":              true,
		"aurago-sounds":              true,
	}
	packRoot := filepath.Clean(filepath.Join("..", "internal", "gamemaker", "asset_packs"))
	var mu sync.Mutex
	var disallowed []string

	packHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rel := strings.TrimPrefix(path.Clean(r.URL.Path), "/api/game-maker/asset-packs/")
		parts := strings.SplitN(rel, "/", 2)
		if len(parts) != 2 || !allowedPacks[parts[0]] || parts[1] == "" || strings.Contains(parts[1], "..") {
			mu.Lock()
			disallowed = append(disallowed, r.URL.Path)
			mu.Unlock()
			http.Error(w, "forbidden asset pack path", http.StatusForbidden)
			return
		}
		diskPath := filepath.Join(packRoot, parts[0], filepath.FromSlash(parts[1]))
		if info, err := os.Stat(diskPath); err != nil || info.IsDir() {
			http.NotFound(w, r)
			return
		}
		switch {
		case strings.HasSuffix(diskPath, ".json"):
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
		case strings.HasSuffix(diskPath, ".png"):
			w.Header().Set("Content-Type", "image/png")
		case strings.HasSuffix(diskPath, ".wav"):
			w.Header().Set("Content-Type", "audio/wav")
		}
		http.ServeFile(w, r, diskPath)
	})

	page := desktopAuditBrowserWithAPI(t, map[string]http.Handler{
		"/api/game-maker/asset-packs/": packHandler,
	})

	page.MustEval(`()=>{ localStorage.removeItem('aurago.runenpfad.v1'); window.auditTest.openApp('runenpfad'); }`)
	page.MustWait(`()=>{
		const app = document.querySelector('.rp-app');
		const load = document.querySelector('.rp-loading');
		const errCard = document.querySelector('.rp-error');
		return app && app.dataset.rpState === 'playing' && load && load.hidden && errCard && errCard.hidden;
	}`)

	if !page.MustEval(`()=>{
		const c = document.querySelector('.rp-canvas');
		return !!c && c.width > 300 && c.height > 200;
	}`).Bool() {
		t.Fatal("runenpfad canvas was not sized cleanly")
	}

	if got := page.MustEval(`()=>document.querySelector('.rp-app').dataset.rpHero`).Str(); got != "knight" {
		t.Fatalf("expected initial hero knight, got %q", got)
	}

	// Switch hero Knight -> Ranger -> Knight
	page.MustEval(`()=>document.querySelector('[data-rp-btn="hero"]').click()`)
	page.MustWait(`()=>document.querySelector('.rp-app')?.dataset.rpHero === 'ranger'`)
	page.MustEval(`()=>document.querySelector('[data-rp-btn="hero"]').click()`)
	page.MustWait(`()=>document.querySelector('.rp-app')?.dataset.rpHero === 'knight'`)

	// Buy Vitality upgrade in Workshop (25 starting coins >= 15 cost -> maxHp 5 -> 6)
	page.MustEval(`()=>document.querySelector('[data-rp-btn="shop"]').click()`)
	page.MustWait(`()=>document.querySelector('.rp-shop-dialog')?.hasAttribute('open')`)
	page.MustEval(`()=>document.querySelector('.rp-shop-dialog [data-rp-buy-track="vitality"]').click()`)
	page.MustWait(`()=>document.querySelector('.rp-app')?.dataset.rpMaxHp === '6'`)
	page.MustEval(`()=>document.querySelector('.rp-shop-dialog .rp-dialog-head button').click()`)
	page.MustWait(`()=>!document.querySelector('.rp-shop-dialog')?.hasAttribute('open')`)

	// Sound is enabled by default; toggle off in HUD and back on in Pause modal
	page.MustWait(`()=>document.querySelector('.rp-app')?.dataset.rpSound === '1'`)
	page.MustEval(`()=>document.querySelector('[data-rp-btn="sound"]').click()`)
	page.MustWait(`()=>document.querySelector('.rp-app')?.dataset.rpSound === '0'`)

	// Toggle sound back on via Pause modal
	page.MustEval(`()=>document.querySelector('[data-rp-btn="pause"]').click()`)
	page.MustWait(`()=>document.querySelector('.rp-pause-dialog')?.hasAttribute('open')`)
	page.MustEval(`()=>document.querySelector('.rp-pause-dialog button[aria-pressed]').click()`)
	page.MustWait(`()=>document.querySelector('.rp-app')?.dataset.rpSound === '1'`)
	page.MustEval(`()=>document.querySelector('.rp-pause-dialog .rp-dialog-head button').click()`)
	page.MustWait(`()=>!document.querySelector('.rp-pause-dialog')?.hasAttribute('open')`)

	// Verify adjacent subterranean ground blocks render without sky-colored gaps
	page.MustWait(`()=>{
		const c = document.querySelector('.rp-canvas');
		const ctx2d = c && c.getContext('2d');
		if (!ctx2d || c.width < 300 || c.height < 200) return false;
		const dpr = Math.min(2, window.devicePixelRatio || 1);
		const seamX = Math.round(144 * dpr);
		const scanY = c.height - Math.round(28 * dpr);
		const img = ctx2d.getImageData(seamX - 6, scanY, 13, 1).data;
		for (let i = 0; i < img.length; i += 4) {
			const r = img[i], b = img[i + 2];
			if (b > r) return false;
		}
		return true;
	}`)

	// Play through Stage 1 key + portal through the live RAF loop
	page.MustEval(`()=>{
		const shell = document.querySelector('.rp-app');
		const sim = shell.__runenpfad.getSim();
		shell.__runenpfad.setPlayerPosition(sim.stage.key.x, sim.stage.key.y + 20);
	}`)
	page.MustWait(`()=>document.querySelector('.rp-app')?.dataset.rpKey === '1'`)
	page.MustEval(`()=>{
		const shell = document.querySelector('.rp-app');
		const sim = shell.__runenpfad.getSim();
		shell.__runenpfad.setPlayerPosition(sim.stage.portal.x, sim.stage.portal.y + 32);
	}`)
	page.MustWait(`()=>{
		const app = document.querySelector('.rp-app');
		const win = document.querySelector('.rp-win');
		return app && app.dataset.rpState === 'won' && app.dataset.rpUnlocked === '2' && Number(app.dataset.rpStars) >= 1 && win && !win.hidden;
	}`)

	// Close window and verify clean teardown
	page.MustEval(`()=>{
		const win = [...window.auditTest.state.windows.values()].find(w => w.appId === 'runenpfad');
		if (!win) throw Error('runenpfad window missing');
		window.auditTest.closeWindow(win.id);
	}`)
	page.MustWait(`()=>!document.querySelector('.rp-app')`)

	if errs := page.MustEval(`()=>JSON.stringify(window.fixtureErrors || [])`).Str(); errs != "[]" {
		t.Fatalf("unexpected browser console/page errors: %s", errs)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(disallowed) > 0 {
		t.Fatalf("unexpected disallowed asset pack requests: %v", disallowed)
	}
	_ = fmt.Sprintf("verified")
}
