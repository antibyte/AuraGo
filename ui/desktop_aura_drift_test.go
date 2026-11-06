package ui

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"aurago/internal/desktop"
)

var auraDriftModules = []string{
	"aura-drift-core.js",
	"aura-drift-physics.js",
	"aura-drift-track.js",
	"aura-drift-style.js",
	"aura-drift-conditions.js",
	"aura-drift-tricks.js",
	"aura-drift-features.js",
	"aura-drift-weapons.js",
	"aura-drift-projectiles.js",
	"aura-drift-modes.js",
	"aura-drift-ai.js",
	"aura-drift-race.js",
	"aura-drift-banter.js",
	"aura-drift-camera.js",
	"aura-drift-look.js",
	"aura-drift-assets.js",
	"aura-drift-cars.js",
	"aura-drift-scenery.js",
	"aura-drift-render.js",
	"aura-drift-terrain.js",
	"aura-drift-road.js",
	"aura-drift-sky.js",
	"aura-drift-props.js",
	"aura-drift-actors.js",
	"aura-drift-carlight.js",
	"aura-drift-fx.js",
	"aura-drift-post.js",
	"aura-drift-render3d.js",
	"aura-drift-hud.js",
	"aura-drift-hud-fx.js",
	"aura-drift-audio-bank.js",
	"aura-drift-engine.js",
	"aura-drift-audio.js",
	"aura-drift-music.js",
	"aura-drift-ui.js",
	"aura-drift-ui-cards.js",
	"aura-drift-woof-lines.js",
	"aura-drift-woof.js",
	"aura-drift.js",
}

// Aura Drift is a first-party offline racing app. These checks keep its shell
// registration, module order, assets and sixteen locale bundles aligned.
func TestAuraDriftAppRegistration(t *testing.T) {
	t.Parallel()

	loader := readDesktopAssetText(t, "js/desktop/core/module-loader.js")
	modules := auraDriftModules
	previous := strings.Index(loader, "'aura-drift': {")
	if previous < 0 {
		t.Fatal("module loader has no aura-drift entry")
	}
	runtimeIndex := strings.Index(loader[previous:], "'/js/vendor/aura-drift/three-runtime.js'")
	coreIndex := strings.Index(loader[previous:], "'/js/desktop/apps/aura-drift-core.js'")
	if runtimeIndex < 0 || coreIndex < 0 || runtimeIndex > coreIndex {
		t.Fatal("the three.js runtime must load before aura-drift-core.js")
	}
	for _, name := range modules {
		index := strings.Index(loader, "/js/desktop/apps/"+name)
		if index <= previous {
			t.Fatalf("module %s is missing or out of order in the loader", name)
		}
		previous = index
		if _, err := os.Stat(filepath.Join("js", "desktop", "apps", name)); err != nil {
			t.Fatalf("module %s does not exist: %v", name, err)
		}
	}
	if !strings.Contains(loader, "/css/desktop-app-aura-drift.css") {
		t.Fatal("module loader misses the Aura Drift stylesheet")
	}
	if !strings.Contains(loader, "'aura-drift': ['aura_drift']") {
		t.Fatal("module loader misses the aura_drift i18n section")
	}
	for _, path := range []string{
		filepath.Join("css", "desktop-app-aura-drift.css"),
		filepath.Join("img", "desktop-icons", "aura-drift.svg"),
		// The voice commentary runs in a worker of its own.
		filepath.Join("js", "desktop", "apps", "aura-drift-woof-worker.js"),
		filepath.Join("js", "desktop", "apps", "aura-drift-woof-runtime.js"),
		filepath.Join("js", "desktop", "apps", "aura-drift-woof-text.js"),
		// The weights of the voice ship with the game: the racer stays offline.
		filepath.Join("3d", "aura-drift", "voice", "woof_int8.npz"),
		filepath.Join("3d", "aura-drift", "voice", "LICENSE.txt"),
		filepath.Join("3d", "aura-drift", "voice", "NOTICE.txt"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("missing Aura Drift asset %s: %v", path, err)
		}
	}

	for _, manifest := range []string{
		filepath.Join("3d", "aura-drift", "v1", "manifest.json"),
		filepath.Join("3d", "aura-drift", "audio", "manifest.json"),
	} {
		if info, err := os.Stat(manifest); err != nil || info.Size() < 20 {
			t.Fatalf("missing or empty Aura Drift manifest %s: %v", manifest, err)
		}
	}

	css := readDesktopAssetText(t, "css/desktop-app-aura-drift.css")
	if !strings.Contains(css, ":focus-visible") || !strings.Contains(css, ".agdrift-sr-only") {
		t.Fatal("Aura Drift stylesheet must keep keyboard focus and screen-reader affordances")
	}
	if !strings.Contains(css, ".agdrift-load-error") {
		t.Fatal("Aura Drift stylesheet must style the visible stale-assets error panel")
	}
	uiSource := readDesktopAssetText(t, "js/desktop/apps/aura-drift-ui.js")
	if !strings.Contains(uiSource, "aria-hidden") {
		t.Fatal("pointer-only touch controls must be hidden from screen readers")
	}
	shell := readDesktopAssetText(t, "js/desktop/apps/aura-drift.js")
	for _, marker := range []string{"missingModules", "showLoadError", "createNoopPost"} {
		if !strings.Contains(shell, marker) {
			t.Fatalf("Aura Drift shell lost its stale-loader guard %s", marker)
		}
	}

	routing := readDesktopAssetText(t, "js/desktop/core/menus-and-routing.js")
	if !strings.Contains(routing, "appId === 'aura-drift'") || !strings.Contains(routing, "window.AuraDriftApp.render") {
		t.Fatal("desktop routing does not mount Aura Drift")
	}
	bundle := readDesktopAssetText(t, "js/desktop/bundles/main.bundle.js")
	if !strings.Contains(bundle, "'aura-drift': 'AuraDriftApp'") {
		t.Fatal("main bundle is stale; run npm run build:ui")
	}

	var found *desktop.AppManifest
	apps := desktop.BuiltinApps()
	for i := range apps {
		if apps[i].ID == "aura-drift" {
			found = &apps[i]
			break
		}
	}
	if found == nil {
		t.Fatal("builtin app catalog has no aura-drift entry")
	}
	if found.Category != "games" {
		t.Fatalf("aura-drift category = %q, want games", found.Category)
	}
	if found.Metadata["logo_path"] != "/img/desktop-icons/aura-drift.svg" {
		t.Fatalf("aura-drift logo_path = %q", found.Metadata["logo_path"])
	}
}

func TestAuraDriftLocaleKeysCoverAllLanguages(t *testing.T) {
	t.Parallel()

	required := []string{
		"desktop.app_aura_drift", "aura_drift.title", "aura_drift.subtitle",
		"aura_drift.quick_race", "aura_drift.time_trial", "aura_drift.cup",
		"aura_drift.daily", "aura_drift.track", "aura_drift.laps",
		"aura_drift.quality", "aura_drift.auto", "aura_drift.low",
		"aura_drift.medium", "aura_drift.high", "aura_drift.ultra",
		"aura_drift.sound", "aura_drift.muted", "aura_drift.weapons_off",
		"aura_drift.reduced_motion", "aura_drift.show_ghost",
		"aura_drift.voice", "aura_drift.voice_note",
		"aura_drift.voice_credit", "aura_drift.voice_credit_url",
		"aura_drift.controls_hint", "aura_drift.best_lap", "aura_drift.record",
		"aura_drift.start", "aura_drift.pause", "aura_drift.resume",
		"aura_drift.restart", "aura_drift.menu", "aura_drift.results",
		"aura_drift.finish", "aura_drift.position", "aura_drift.new_record",
		"aura_drift.next_track", "aura_drift.ready", "aura_drift.lap",
		"aura_drift.weapon", "aura_drift.loading", "aura_drift.catchup",
		"aura_drift.load_error_hint",
		"aura_drift.style", "aura_drift.combo", "aura_drift.sector",
		"aura_drift.best_combo", "aura_drift.conditions",
		"aura_drift.daily_route", "aura_drift.rivals",
	}
	// Every id-based key the shell, HUD, banter and props build at run time.
	ids := func(prefix, suffix string, list ...string) {
		for _, id := range list {
			required = append(required, "aura_drift."+prefix+id+suffix)
		}
	}
	drivers := []string{"viktor", "rosi", "bruno", "heidi", "ilse", "gustav", "kurt", "dolores"}
	cars := []string{"vektor", "kobalt", "lumen", "tundra", "softeis", "maehkoenig", "couch", "bertha"}
	biomes := []string{"alpine", "desert", "harbour", "salt", "forest", "industry", "glacier", "metro"}
	tracks := []string{"alpenpass", "wuesten-canyon", "neon-hafen", "salzsee", "wald-rallye", "industrie-ring", "gletscherpass", "metropolis"}
	weapons := []string{
		"oil", "gnome_mine", "emp", "rocket", "mg", "shield", "ducks", "railgun", "flamethrower",
		"frost", "air_horn", "black_hole", "disco", "glove", "harpoon", "cow", "cloak", "overdrive",
	}
	modes := []string{"quick", "time_trial", "cup", "daily", "drift_attack", "elimination", "hot_potato"}
	ids("mode.", ".desc", modes...)
	ids("mode.", ".name", "drift_attack", "elimination", "hot_potato")
	ids("car.", ".tag", cars...)
	ids("car.", ".quirk", cars...)
	ids("driver.", ".bio", drivers...)
	ids("biome.", "", biomes...)
	ids("track.", ".blurb", tracks...)
	ids("weapon.", ".name", weapons...)
	ids("weapon.", ".hint", weapons...)
	ids("stat.", "", "power", "grip", "mass", "top", "air")
	ids("trick.", "",
		"air_time", "barrel_roll", "double_barrel", "front_flip", "back_flip", "flat_spin", "spin_720", "corkscrew",
		"cow_vault", "perfect_start", "drift_blue", "drift_orange", "drift_pink", "slingshot", "reverse_hero", "j_turn")
	ids("landing.", "", "perfect", "good", "wobble", "pancake")
	ids("title.", "",
		"pacifist", "brake_coward", "cow_bowler", "air_force", "pancake_chef", "wall_lover", "drift_king", "reverse_hero",
		"sausage_baron", "disco_fever", "gardener", "ice_man", "sofa_king", "big_bertha", "last_but_loud", "solid_driver")
	ids("medal.", "", "bronze", "silver", "gold", "none")
	ids("hud.", "",
		"final_lap", "lap_of", "eliminated", "next_out", "bomb", "have_bomb", "pass_it", "clock", "score", "perfect_start",
		"wheelspin", "slingshot", "drift1", "drift2", "drift3", "near_miss", "boost", "strike", "cow_bowling", "respawn",
		"shield", "frozen", "toast", "disco", "cloak", "overdrive", "wrong_way", "sweeper")
	ids("settings.", "",
		"camera", "camera_chase", "camera_high", "camera_drone", "camera_bumper", "car", "livery", "tab_mode", "tab_car",
		"tab_track", "tab_settings", "tab_controls", "loading_assets", "fallback_note", "credits", "reset_bests")
	ids("controls.", "",
		"steer", "throttle", "brake_reverse", "handbrake", "drift_turbo", "nitro", "fire", "camera", "horn", "rescue",
		"pause", "restart", "air_roll", "air_flip", "air_spin")
	ids("weather.", "", "clear", "fog", "snow", "heat", "glare", "rain", "mist", "haze", "sparks")
	ids("surface.", "", "asphalt", "salt", "wet", "gravel", "mud", "ice", "oil", "sand", "snow")
	ids("sign.", "",
		"cow_priority", "brake_is_cowardly", "sofa_lane", "gnomes_watching", "no_wheelies", "speed_limit_ish", "caution_ducks",
		"free_hugs_ahead", "last_exit_pizza", "mind_the_gap", "drift_zone", "you_are_here", "turn_left_maybe", "wrong_way_fun",
		"hold_my_nitro", "honk_for_cow")
	ids("brand.", "", "wurstwerk", "muhmilch", "turbotoast", "sofaspeed", "gnombau", "nebelhorn", "entenbank", "plueschreifen")
	ids("result.", "", "score", "tricks", "air_time", "animals", "smashed", "cup_total", "eliminated_at", "survived")
	for _, driver := range drivers {
		ids("banter."+driver+".", "", "pass", "passed", "hit", "got_you", "wreck", "win", "style")
	}
	for n := 1; n <= 16; n++ {
		required = append(required, "aura_drift.marshal."+strconv.Itoa(n))
	}

	placeholder := regexp.MustCompile(`\{\w+\}`)
	english := readMergedLangMap(t, "en")
	var driftKeys []string
	for key := range english {
		if strings.HasPrefix(key, "aura_drift.") {
			driftKeys = append(driftKeys, key)
		}
	}
	if len(driftKeys) < 330 {
		t.Fatalf("en has %d aura_drift keys, want at least 330", len(driftKeys))
	}
	// The explicit list must be part of en, and every en key must stay in the list's family.
	for _, key := range required {
		if strings.TrimSpace(english[key]) == "" {
			t.Fatalf("en missing required Aura Drift key %s", key)
		}
	}
	driftKeys = append(driftKeys, "desktop.app_aura_drift")

	for _, lang := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		values := readMergedLangMap(t, lang)
		for _, key := range required {
			if strings.TrimSpace(values[key]) == "" {
				t.Fatalf("%s missing Aura Drift translation key %s", lang, key)
			}
		}
		for _, key := range driftKeys {
			value := values[key]
			if strings.TrimSpace(value) == "" {
				t.Fatalf("%s missing Aura Drift translation key %s", lang, key)
			}
			if strings.ContainsRune(value, rune(0xFFFD)) {
				t.Fatalf("%s key %s contains U+FFFD", lang, key)
			}
			for _, token := range placeholder.FindAllString(english[key], -1) {
				if !strings.Contains(value, token) {
					t.Fatalf("%s key %s dropped placeholder %s", lang, key, token)
				}
			}
		}
		if lang == "en" {
			continue
		}
		differ, total := 0, 0
		for _, key := range driftKeys {
			if strings.HasPrefix(key, "aura_drift.banter.") {
				total++
				if values[key] != english[key] {
					differ++
				}
			}
		}
		if total == 0 || differ*100 < total*60 {
			t.Fatalf("%s translates only %d of %d banter lines (need 60 percent)", lang, differ, total)
		}
	}
}

func TestAuraDriftBanterLinesCarrySeveralWordings(t *testing.T) {
	t.Parallel()

	// Every spoken line holds its wordings, one per line of its value. The radio
	// shows one of them and the voice speaks the english original of the same one,
	// so a line never sounds the same twice in a row.
	english := readMergedLangMap(t, "en")
	lines := 0
	for key, value := range english {
		if !strings.HasPrefix(key, "aura_drift.banter.") && !strings.HasPrefix(key, "aura_drift.marshal.") {
			continue
		}
		lines++
		words := strings.Split(value, "\n")
		if len(words) < 5 {
			t.Fatalf("en %s has %d wordings, want at least 5", key, len(words))
		}
		seen := map[string]bool{}
		for _, word := range words {
			word = strings.TrimSpace(word)
			if word == "" {
				t.Fatalf("en %s has an empty wording", key)
			}
			if seen[word] {
				t.Fatalf("en %s repeats the wording %q", key, word)
			}
			seen[word] = true
		}
	}
	if lines < 70 {
		t.Fatalf("en has %d banter lines, want at least 70", lines)
	}

	for _, lang := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		values := readMergedLangMap(t, lang)
		for key, value := range english {
			if !strings.HasPrefix(key, "aura_drift.banter.") && !strings.HasPrefix(key, "aura_drift.marshal.") {
				continue
			}
			words := strings.Split(value, "\n")
			local := strings.Split(values[key], "\n")
			if len(local) != len(words) {
				t.Fatalf("%s %s has %d wordings, want %d", lang, key, len(local), len(words))
			}
			seen := map[string]bool{}
			for _, word := range local {
				word = strings.TrimSpace(word)
				if word == "" {
					t.Fatalf("%s %s has an empty wording", lang, key)
				}
				if seen[word] {
					t.Fatalf("%s %s repeats the wording %q", lang, key, word)
				}
				seen[word] = true
			}
		}
	}
}

func TestAuraDriftShipsNoExternalResources(t *testing.T) {
	t.Parallel()

	// The voice commentary runs in a worker of its own, so it is not in the module
	// list, but it belongs to the racer and must stay offline as well.
	names := append(append([]string{}, auraDriftModules...),
		"aura-drift-woof-worker.js", "aura-drift-woof-runtime.js", "aura-drift-woof-text.js")
	for _, name := range names {
		source := readDesktopAssetText(t, filepath.Join("js", "desktop", "apps", name))
		for _, forbidden := range []string{"https://", "http://", "fetch(", "XMLHttpRequest", "import("} {
			if forbidden == "fetch(" && name == "aura-drift-assets.js" {
				// The asset loader may fetch only versioned same-origin URLs.
				if strings.Count(source, "fetch(") != strings.Count(source, "fetch(AD.assetURL(") {
					t.Fatalf("%s calls fetch( without AD.assetURL(", name)
				}
				continue
			}
			if strings.Contains(source, forbidden) {
				t.Fatalf("%s contains %q; the racer must stay fully offline", name, forbidden)
			}
		}
	}
}

// AdGuard/EasyList hide [class^="ad-"] with display:none !important. The racer
// once shipped ad-overlay/ad-panel and was invisible with no console error.
// agdrift- contains no "ad" substring, so even [class*="ad"] rules miss it.
func TestAuraDriftAvoidsAdBlockerClassPrefix(t *testing.T) {
	t.Parallel()
	blockedPrefix := regexp.MustCompile(`(^|[^A-Za-z0-9_-])ad-[a-z]`)
	// Only shipped sources: this test file itself contains the pattern literal.
	paths := []string{
		"css/desktop-app-aura-drift.css",
		"js/desktop/apps/aura-drift.js",
		"js/desktop/apps/aura-drift-ui.js",
		"js/desktop/apps/aura-drift-ui-cards.js",
	}
	for _, path := range paths {
		content := readDesktopAssetText(t, path)
		if loc := blockedPrefix.FindStringIndex(content); loc != nil {
			start := loc[0]
			if start > 40 {
				start -= 40
			}
			end := loc[1] + 40
			if end > len(content) {
				end = len(content)
			}
			t.Fatalf("%s still uses an ad blocker-vulnerable class: ...%s...", path, content[start:end])
		}
	}
	css := readDesktopAssetText(t, "css/desktop-app-aura-drift.css")
	for _, required := range []string{".agdrift-root", ".agdrift-overlay", ".agdrift-panel", ".agdrift-hud"} {
		if !strings.Contains(css, required) {
			t.Fatalf("Aura Drift stylesheet lost %s", required)
		}
	}
	ui := readDesktopAssetText(t, "js/desktop/apps/aura-drift-ui.js")
	for _, required := range []string{"agdrift-panel agdrift-menu", "agdrift-primary", "agdrift-touch-btn"} {
		if !strings.Contains(ui, required) {
			t.Fatalf("Aura Drift UI lost %s", required)
		}
	}
}

// Only hud-fx.js declares TAU; the speedometer glow arc once used a bare TAU and
// threw ReferenceError as soon as boost or slipstream was active during a race.
func TestAuraDriftHUDUsesKitTAU(t *testing.T) {
	t.Parallel()
	hud := readDesktopAssetText(t, "js/desktop/apps/aura-drift-hud.js")
	if loc := regexp.MustCompile(`(^|[^.\w])TAU([^\w]|$)`).FindStringIndex(hud); loc != nil {
		t.Fatalf("aura-drift-hud.js uses a bare TAU (%q); use kit.TAU", hud[loc[0]:loc[1]])
	}
}
