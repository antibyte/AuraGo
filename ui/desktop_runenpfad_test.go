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
	"time"

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
		{"js/desktop/core/module-loader.js", []string{"runenpfad", "/css/desktop-app-runenpfad.css", "/js/desktop/apps/runenpfad/levels.js", "/js/desktop/apps/runenpfad/app.js", "/js/desktop/apps/runenpfad/fx.js", "/js/desktop/apps/runenpfad/biomes.js", "/js/desktop/apps/runenpfad/backdrops.js", "/js/desktop/apps/runenpfad/backdrops-biomes.js", "/js/desktop/apps/runenpfad/parallax-art.js", "/js/desktop/apps/runenpfad/atmosphere.js", "/js/desktop/apps/runenpfad/behaviors.js", "/js/desktop/apps/runenpfad/variety.js", "/js/desktop/apps/runenpfad/profile.js", "/js/desktop/apps/runenpfad/physics.js", "/js/desktop/apps/runenpfad/combat.js", "/js/desktop/apps/runenpfad/bosses.js", "/js/desktop/apps/runenpfad/zones.js", "/js/desktop/apps/runenpfad/music.js", "/js/desktop/apps/runenpfad/decor.js", "/js/desktop/apps/runenpfad/actors.js", "/js/desktop/apps/runenpfad/stage-art.js", "/js/desktop/apps/runenpfad/postfx.js", "/js/desktop/apps/runenpfad/hud.js", "/js/desktop/apps/runenpfad/npcs.js", "/js/desktop/apps/runenpfad/npc-story.js", "/js/desktop/apps/runenpfad/npc-story-late.js", "/js/desktop/apps/runenpfad/dialogue.js", "/js/desktop/apps/runenpfad/trade.js", "/js/desktop/apps/runenpfad/duel-patterns.js", "/js/desktop/apps/runenpfad/duel.js", "/js/desktop/apps/runenpfad/npc-view.js", "/js/desktop/apps/runenpfad/duel-view.js", "/js/desktop/apps/runenpfad/dialogue-ui.js", "/js/desktop/apps/runenpfad/place-rooms.js", "/js/desktop/apps/runenpfad/interior-mobs.js", "/js/desktop/apps/runenpfad/interior.js", "/js/desktop/apps/runenpfad/places.js", "/js/desktop/apps/runenpfad/folk.js", "/js/desktop/apps/runenpfad/folk-story.js", "/js/desktop/apps/runenpfad/village-rooms.js", "/js/desktop/apps/runenpfad/village.js", "/js/desktop/apps/runenpfad/cutscenes.js", "/js/desktop/apps/runenpfad/place-music.js", "/js/desktop/apps/runenpfad/story-music.js", "/js/desktop/apps/runenpfad/place-view.js", "/js/desktop/apps/runenpfad/interior-bake.js", "/js/desktop/apps/runenpfad/interior-view.js", "/js/desktop/apps/runenpfad/cutscene-view.js", "/js/desktop/apps/runenpfad/worldmap-ui.js", "/js/desktop/apps/runenpfad/cutscene-ui.js"}},
		{"js/desktop/core/menus-and-routing.js", []string{"if (appId === 'runenpfad')", "RunenpfadApp.render"}},
		{"js/desktop/core/desktop-foundation.js", []string{"runenpfad: '/img/runenpfad/icon.svg'", "runenpfad: 'RunenpfadApp'"}},
		{"js/desktop/core/window-shell-runtime.js", []string{"runenpfad: { width: 1180, height: 740 }"}},
		{"js/desktop/bundles/main.bundle.js", []string{"if (appId === 'runenpfad')", "runenpfad: 'RunenpfadApp'"}},
		{"css/desktop-app-runenpfad.css", []string{".rp-app", ".rp-canvas", ".rp-hud"}},
		{"js/desktop/apps/runenpfad/levels.js", []string{"RunenpfadLevels", "CAMPAIGN_LEVELS", "validateSpriteManifests"}},
		{"js/desktop/apps/runenpfad/sim.js", []string{"RunenpfadSim", "aurago.runenpfad.v1", "stepSimulation"}},
		{"js/desktop/apps/runenpfad/profile.js", []string{"RunenpfadProfile", "aurago.runenpfad.v1", "importProfile"}},
		{"js/desktop/apps/runenpfad/audio.js", []string{"RunenpfadAudio", "validateSoundManifest", "/api/game-maker/asset-packs/aurago-sounds/"}},
		{"js/desktop/apps/runenpfad/fx.js", []string{"RunenpfadFx", "createSystem", "hitstop"}},
		{"js/desktop/apps/runenpfad/biomes.js", []string{"RunenpfadBiomes", "BIOMES", "biomeFor"}},
		{"js/desktop/apps/runenpfad/backdrops.js", []string{"RunenpfadBackdrops", "buildCrest"}},
		{"js/desktop/apps/runenpfad/backdrops-biomes.js", []string{"RunenpfadBackdrops", "decoratePeaks"}},
		{"js/desktop/apps/runenpfad/parallax-art.js", []string{"RunenpfadParallaxArt", "createParallax"}},
		{"js/desktop/apps/runenpfad/atmosphere.js", []string{"RunenpfadAtmosphere", "createAtmosphere", "paintLighting"}},
		{"js/desktop/apps/runenpfad/behaviors.js", []string{"RunenpfadBehaviors", "ambusher", "turret"}},
		{"js/desktop/apps/runenpfad/variety.js", []string{"RunenpfadVariety", "MODS", "pickMod"}},
		{"js/desktop/apps/runenpfad/view.js", []string{"RunenpfadView", "createRenderer"}},
		{"js/desktop/apps/runenpfad/physics.js", []string{"RunenpfadPhysics"}},
		{"js/desktop/apps/runenpfad/combat.js", []string{"RunenpfadCombat"}},
		{"js/desktop/apps/runenpfad/bosses.js", []string{"RunenpfadBosses"}},
		{"js/desktop/apps/runenpfad/zones.js", []string{"RunenpfadZones"}},
		{"js/desktop/apps/runenpfad/music.js", []string{"RunenpfadMusic"}},
		{"js/desktop/apps/runenpfad/decor.js", []string{"RunenpfadDecor"}},
		{"js/desktop/apps/runenpfad/actors.js", []string{"RunenpfadActors"}},
		{"js/desktop/apps/runenpfad/stage-art.js", []string{"RunenpfadStageArt"}},
		{"js/desktop/apps/runenpfad/npc-view.js", []string{"RunenpfadNpcView"}},
		{"js/desktop/apps/runenpfad/duel-view.js", []string{"RunenpfadDuelView", "paintBackdrop", "drawWipe"}},
		{"js/desktop/apps/runenpfad/postfx.js", []string{"RunenpfadPostfx"}},
		{"js/desktop/apps/runenpfad/hud.js", []string{"RunenpfadHud", "createElement", "rp-topbar"}},
		{"js/desktop/apps/runenpfad/npcs.js", []string{"RunenpfadNpcs", "npc_camp"}},
		{"js/desktop/apps/runenpfad/dialogue.js", []string{"RunenpfadDialogue", "validateGraphs"}},
		{"js/desktop/apps/runenpfad/npc-story.js", []string{"RunenpfadNpcStory"}},
		{"js/desktop/apps/runenpfad/npc-story-late.js", []string{"RunenpfadNpcStory", "Saida"}},
		{"js/desktop/apps/runenpfad/trade.js", []string{"RunenpfadTrade"}},
		{"js/desktop/apps/runenpfad/duel-patterns.js", []string{"RunenpfadDuelPatterns", "tellTime"}},
		{"js/desktop/apps/runenpfad/duel.js", []string{"RunenpfadDuel", "hurtHero"}},
		{"js/desktop/apps/runenpfad/dialogue-ui.js", []string{"RunenpfadDialogueUi", "createElement", "rp-talk"}},
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

	// readProfile() assigns rejectedRaw, so a declaration after the first call throws and discards every saved profile.
	appJS := readDesktopAssetText(t, "js/desktop/apps/runenpfad/app.js")
	rejectedDecl := strings.Index(appJS, "let rejectedRaw")
	firstRead := strings.Index(appJS, "= readProfile()")
	if rejectedDecl < 0 || firstRead < 0 || rejectedDecl > firstRead {
		t.Fatal("runenpfad app.js must declare rejectedRaw before the first readProfile() call")
	}

	requiredKeys := []string{
		"runenpfad.title",
		"runenpfad.hud_level",
		"runenpfad.shop_title",
		"runenpfad.win_title",
		"runenpfad.lose_title",
		"runenpfad.error_assets",
		"runenpfad.float_block",
		"runenpfad.float_open",
		"runenpfad.level_locked",
		"runenpfad.hero_selected",
		"runenpfad.hero_choose",
		"runenpfad.hero_switch_confirm",
		"runenpfad.win_new_best",
		"runenpfad.win_goal_portal",
		"runenpfad.win_goal_gems",
		"runenpfad.win_goal_swift",
		"runenpfad.ctl_move",
		"runenpfad.resonance_hint",
		"runenpfad.sky_ray_hint",
		"runenpfad.resonance_progress",
		"runenpfad.notice.resonance_complete",
		"runenpfad.notice.resonance_expire",
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
		if !strings.Contains(dict["runenpfad.win_goal_swift"], "{time}") {
			t.Fatalf("%s runenpfad.win_goal_swift must keep the {time} placeholder", rpPath)
		}
		for _, placeholder := range []string{"{count}", "{total}", "{seconds}"} {
			if !strings.Contains(dict["runenpfad.resonance_progress"], placeholder) {
				t.Errorf("%s resonance progress is missing %s", rpPath, placeholder)
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
		"jump-and-run":              true,
		"human-characters-animated": true,
		"monsters-animals-animated": true,
		"robots-drones-animated":    true,
		"assets-animated":           true,
		"nature-assets":             true,
		"aurago-sounds":             true,
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
	}).CancelTimeout().Timeout(90 * time.Second)

	page.MustEval(`()=>{ localStorage.removeItem('aurago.runenpfad.v1'); window.auditTest.openApp('runenpfad'); }`)
	page.MustWait(`()=>{
		const app = document.querySelector('.rp-app');
		const load = document.querySelector('.rp-loading');
		const errCard = document.querySelector('.rp-error');
		return app && app.dataset.rpState === 'playing' && load && load.hidden && errCard && errCard.hidden;
	}`)

	// A fresh profile opens the prologue over the paused stage; Skip closes it and the stage runs.
	page.MustWait(`()=>!!document.querySelector('.rp-cut') && !!document.querySelector('.rp-app').__runenpfad.getCutscene()`)
	page.MustEval(`()=>document.querySelector('.rp-cut-skip').click()`)
	page.MustWait(`()=>!document.querySelector('.rp-cut') && !document.querySelector('.rp-app').__runenpfad.getCutscene()`)

	if !page.MustEval(`()=>{
		const c = document.querySelector('.rp-canvas');
		return !!c && c.width > 300 && c.height > 200;
	}`).Bool() {
		t.Fatal("runenpfad canvas was not sized cleanly")
	}

	if got := page.MustEval(`()=>document.querySelector('.rp-app').dataset.rpHero`).Str(); got != "knight" {
		t.Fatalf("expected initial hero knight, got %q", got)
	}

	if !page.MustEval(`() => {
		const canvas = document.createElement('canvas');
		canvas.width = 320; canvas.height = 180;
		const sky = RunenpfadAtmosphere.createAtmosphere(canvas.getContext('2d'));
		const biome = RunenpfadBiomes.biomeFor(3);
		const paint = (t, reduced) => {
			sky.paintBackdrop(biome, 0, 320, 180, 150, t, reduced);
			return canvas.toDataURL();
		};
		try {
			paint(1, true);
			return paint(5, true) === paint(15, true) && paint(5, false) !== paint(15, false);
		} finally { sky.dispose(); }
	}`).Bool() {
		t.Fatal("the glacier sky must animate normally and hold still with reduced motion")
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

	// Post-processing resolves to either the GPU compositor or the plain canvas
	page.MustWait(`()=>['gpu','canvas'].includes(document.querySelector('.rp-app')?.dataset.rpGfx)`)

	// Mid-stage coin pickups reach the HUD data marker before the stage ends
	coinsBefore := page.MustEval(`()=>Number(document.querySelector('.rp-app').dataset.rpCoins)`).Int()
	page.MustEval(`()=>{
		const shell = document.querySelector('.rp-app');
		const sim = shell.__runenpfad.getSim();
		const coin = sim.stage.collectibles.find(c => c.kind === 'coin' && !c.collected);
		if (!coin) throw Error('no coin in stage');
		shell.__runenpfad.setPlayerPosition(coin.x, coin.y + 10);
	}`)
	page.MustWait(fmt.Sprintf(`()=>Number(document.querySelector('.rp-app').dataset.rpCoins) > %d`, coinsBefore))

	// Each pickup passes through the live simulation, world renderer and translated HUD.
	total := page.MustEval(`()=>document.querySelector('.rp-app').__runenpfad.getSim().stage.resonancePath.runes.length`).Int()
	for i := 0; i < total; i++ {
		page.MustEval(`(i)=>{
			const game = document.querySelector('.rp-app').__runenpfad;
			const rune = game.getSim().stage.resonancePath.runes[i];
			game.setPlayerPosition(rune.x, rune.y + 20);
		}`, i)
		if i+1 < total {
			page.MustWait(fmt.Sprintf(`()=>{const tag = document.querySelector('.rp-tag-path'); return !tag.hidden && tag.textContent.includes('%d/%d');}`, i+1, total))
			if dir := os.Getenv("AURAGO_BROWSER_ARTIFACT_DIR"); dir != "" && i == 0 {
				if err := os.MkdirAll(dir, 0755); err != nil {
					t.Fatal(err)
				}
				page.MustElement(".rp-app").MustScreenshot(filepath.Join(dir, "runenspur.png"))
			}
		}
	}
	page.MustWait(`()=>document.querySelector('.rp-app').__runenpfad.getSim().stage.resonancePath.complete && document.querySelector('.rp-tag-path').hidden`)

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

	// The win card is keyboard-operable: the primary button is focused, and a fresh Enter keydown
	// (after the short guard window that swallows the key which ended the round) starts the next stage.
	page.MustWait(`()=>{
		const win = document.querySelector('.rp-win');
		return !!win && win.contains(document.activeElement);
	}`)
	page.MustEval(`()=>new Promise(r => setTimeout(r, 450))`)
	page.MustEval(`()=>window.dispatchEvent(new KeyboardEvent('keydown', { code: 'Enter', key: 'Enter', bubbles: true }))`)
	page.MustWait(`()=>{
		const app = document.querySelector('.rp-app');
		const win = document.querySelector('.rp-win');
		return app && app.dataset.rpState === 'playing' && app.dataset.rpLevel === '2' && win && win.hidden;
	}`)

	page.MustWait(`()=>RunenpfadArt.has(RunenpfadArt.current(), "creature-sky-ray.glide")`)
	if !page.MustEval(`()=>{
		const canvas = document.createElement('canvas'); canvas.width = 160; canvas.height = 140;
		const ctx = canvas.getContext('2d');
		const painter = RunenpfadStageArt.create({ctx, art: RunenpfadArt.current(), drawGlow() {}});
		const stage = {portal: {x: 80, y: 74}, tileSize: 48};
		const paint = (hasKey, nowSec, reducedMotion) => {
			ctx.clearRect(0, 0, 160, 140);
			painter.drawPortal({stage, hasKey}, {sx: 0, viewW: 160, nowSec, reducedMotion});
			return canvas.toDataURL();
		};
		const locked = paint(false, 0, false), open = paint(true, 0, true);
		return locked !== open && locked === paint(false, 1.5, false) &&
			open === paint(true, 1.5, true) && open !== paint(true, 1.5, false);
	}`).Bool() {
		t.Fatal("the portal must show its lock, animate when open and hold still with reduced motion")
	}

	// Board from the ground, travel on the ray and jump off using the regular control.
	page.MustEval(`()=>{
		const game = document.querySelector('.rp-app').__runenpfad;
		const sim = game.getSim(), ray = sim.stage.skyRays[0];
		game.setPlayerPosition(ray.x + ray.w / 2 + ray.dir * 30, sim.stage.groundY);
	}`)
	page.MustWait(`()=>{
		const tag = document.querySelector('.rp-tag-path');
		const sim = document.querySelector('.rp-app').__runenpfad.getSim();
		return sim.player.onGround && !tag.hidden && tag.textContent.trim() && !tag.textContent.includes('runenpfad.');
	}`)
	page.MustEval(`()=>window.dispatchEvent(new KeyboardEvent("keydown", {code: "Space", key: " ", bubbles: true}))`)
	page.MustWait(`()=>document.querySelector('.rp-app').__runenpfad.getSim().stage.skyRays[0].ridden`)
	page.MustEval(`()=>window.dispatchEvent(new KeyboardEvent("keyup", {code: "Space", key: " ", bubbles: true}))`)
	startX := page.MustEval(`()=>document.querySelector('.rp-app').__runenpfad.getSim().player.x`).Int()
	page.MustWait(fmt.Sprintf(`()=>{
		const sim = document.querySelector('.rp-app').__runenpfad.getSim();
		return sim.stage.skyRays[0].ridden && Math.abs(sim.player.x - %d) > 36;
	}`, startX))
	if dir := os.Getenv("AURAGO_BROWSER_ARTIFACT_DIR"); dir != "" {
		page.MustElement(".rp-app").MustScreenshot(filepath.Join(dir, "windrochen.png"))
	}
	page.MustEval(`()=>window.dispatchEvent(new KeyboardEvent("keydown", {code: "Space", key: " ", bubbles: true}))`)
	page.MustWait(`()=>{
		const sim = document.querySelector('.rp-app').__runenpfad.getSim();
		return !sim.stage.skyRays[0].ridden && sim.player.y < sim.stage.skyRays[0].y - 15;
	}`)
	page.MustEval(`()=>window.dispatchEvent(new KeyboardEvent("keyup", {code: "Space", key: " ", bubbles: true}))`)

	// NPC encounter smoke on stage 2 (witch camp): talk, answer, leave, then a duel that ends quickly.
	page.MustEval(`()=>{
		const shell = document.querySelector('.rp-app');
		const sim = shell.__runenpfad.getSim();
		const npc = (sim.stage.npcs || []).find(n => n.npcId === 'witch');
		if (!npc) throw Error('no witch camp on stage 2');
		shell.__runenpfad.setPlayerPosition(npc.x - 40, npc.y);
	}`)
	page.MustWait(`()=>document.querySelector('.rp-app')?.dataset.rpNpcNear === '1'`)
	page.MustEval(`()=>new Promise(r => setTimeout(r, 450))`)
	page.MustEval(`()=>window.dispatchEvent(new KeyboardEvent('keydown', { code: 'KeyE', key: 'e', bubbles: true }))`)
	page.MustWait(`()=>{
		const app = document.querySelector('.rp-app');
		const sim = app && app.__runenpfad.getSim();
		return app.dataset.rpEncounter === 'dialogue' && sim.encounter && sim.encounter.kind === 'dialogue' && !!document.querySelector('.rp-talk.is-in');
	}`)
	page.MustEval(`()=>new Promise(r => setTimeout(r, 600))`)
	page.MustEval(`()=>window.dispatchEvent(new KeyboardEvent('keydown', { code: 'Digit1', key: '1', bubbles: true }))`)
	// Leave with Esc (repeated: the first press may only advance the typewriter or a node).
	page.MustWait(`()=>{
		const app = document.querySelector('.rp-app');
		const sim = app.__runenpfad.getSim();
		if (!sim.encounter) return true;
		const now = performance.now();
		if (!window.__rpEscAt || now - window.__rpEscAt > 500) {
			window.__rpEscAt = now;
			window.dispatchEvent(new KeyboardEvent('keydown', { code: 'Escape', key: 'Escape', bubbles: true }));
		}
		return false;
	}`)
	page.MustWait(`()=>document.querySelector('.rp-app')?.dataset.rpEncounter === '' && !document.querySelector('.rp-pause-dialog')?.hasAttribute('open')`)

	page.MustEval(`()=>{
		const shell = document.querySelector('.rp-app');
		const sim = shell.__runenpfad.getSim();
		const npc = sim.stage.npcs.find(n => n.npcId === 'witch');
		if (!window.RunenpfadNpcs.startDuel(sim, npc, { provoked: true })) throw Error('duel did not start');
	}`)
	page.MustWait(`()=>{
		const app = document.querySelector('.rp-app');
		const sim = app.__runenpfad.getSim();
		return app.dataset.rpEncounter === 'duel' && sim.encounter && sim.encounter.kind === 'duel';
	}`)
	// Hit the NPC for lethal damage as soon as the duel is in its fight phase; the outro and finishDuel follow.
	page.MustWait(`()=>{
		const sim = document.querySelector('.rp-app').__runenpfad.getSim();
		const d = sim.encounter && sim.encounter.duel;
		if (!d) return true;
		if (d.phase === 'fight') window.RunenpfadDuel.hurtNpc(sim, d, 999, { bypass: true });
		return d.phase === 'outro';
	}`)
	page.MustWait(`()=>{
		const app = document.querySelector('.rp-app');
		const sim = app.__runenpfad.getSim();
		if (sim.encounter && sim.encounter.kind === 'dialogue') {
			// A won duel may open a reaction dialogue; leave it the same way.
			const now = performance.now();
			if (!window.__rpEscAt || now - window.__rpEscAt > 500) {
				window.__rpEscAt = now;
				window.dispatchEvent(new KeyboardEvent('keydown', { code: 'Escape', key: 'Escape', bubbles: true }));
			}
			return false;
		}
		return !sim.encounter && app.dataset.rpState === 'playing' && app.dataset.rpEncounter === '';
	}`)
	page.MustEval(`()=>{
		const sim = document.querySelector('.rp-app').__runenpfad.getSim();
		const npc = sim.stage.npcs.find(n => n.npcId === 'witch');
		if (sim.player.hp <= 0 || sim.state !== 'playing') throw Error('hero not playing after the duel');
		if (!npc) throw Error('witch vanished');
	}`)

	// The village hub, a building interior and a chronicle replay.
	page.MustEval(`()=>document.querySelector('.rp-app').__runenpfad.startHub()`)
	page.MustWait(`()=>document.querySelector('.rp-app')?.dataset.rpScene === 'hub'`)
	if !page.MustEval(`()=>document.querySelector('.rp-app').__runenpfad.enterPlace('tavern')`).Bool() {
		t.Fatal("enterPlace('tavern') did not enter the tavern")
	}
	page.MustWait(`()=>document.querySelector('.rp-app')?.dataset.rpScene === 'interior'`)
	page.MustEval(`()=>document.querySelector('.rp-app').__runenpfad.leavePlace()`)
	page.MustWait(`()=>document.querySelector('.rp-app')?.dataset.rpScene === 'hub'`)
	page.MustEval(`()=>document.querySelector('.rp-app').__runenpfad.playCutscene('after_1')`)
	page.MustWait(`()=>{ const c = document.querySelector('.rp-app').__runenpfad.getCutscene(); return !!c && c.open && !!document.querySelector('.rp-cut'); }`)
	page.MustEval(`()=>document.querySelector('.rp-cut-skip').click()`)
	page.MustWait(`()=>!document.querySelector('.rp-cut')`)

	// Close window and verify clean teardown
	page.MustEval(`()=>{
		const win = [...window.auditTest.state.windows.values()].find(w => w.appId === 'runenpfad');
		if (!win) throw Error('runenpfad window missing');
		window.auditTest.closeWindow(win.id);
	}`)
	page.MustWait(`()=>!document.querySelector('.rp-app')`)

	// Reopening restores the saved profile: the unlocked stage and the bought upgrade survive the window.
	page.MustEval(`()=>window.auditTest.openApp('runenpfad')`)
	page.MustWait(`()=>{
		const app = document.querySelector('.rp-app');
		return app && app.dataset.rpState === 'playing' && app.dataset.rpUnlocked === '2' && app.dataset.rpLevel === '2' && app.dataset.rpMaxHp === '6';
	}`)
	if page.MustEval(`()=>!!document.querySelector('.rp-cut')`).Bool() {
		t.Fatal("the prologue must not open again once it was seen")
	}

	// Reset (two clicks) closes the pause dialog, clears the stored progress and starts level 1 with the prologue again.
	page.MustEval(`()=>document.querySelector('[data-rp-btn="pause"]').click()`)
	page.MustWait(`()=>document.querySelector('.rp-pause-dialog')?.hasAttribute('open')`)
	page.MustEval(`()=>document.querySelector('.rp-pause-dialog .rp-btn-danger').click()`)
	page.MustEval(`()=>document.querySelector('.rp-pause-dialog .rp-btn-danger').click()`)
	page.MustWait(`()=>!document.querySelector('.rp-pause-dialog')?.hasAttribute('open') && !!document.querySelector('.rp-cut')`)
	page.MustEval(`()=>document.querySelector('.rp-cut-skip').click()`)
	page.MustWait(`()=>{
		const app = document.querySelector('.rp-app');
		const saved = JSON.parse(localStorage.getItem('aurago.runenpfad.v1') || '{}');
		return app && app.dataset.rpLevel === '1' && app.dataset.rpUnlocked === '1' && app.dataset.rpMaxHp === '5' && saved.unlockedLevel === 1 && !document.querySelector('.rp-cut');
	}`)
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
