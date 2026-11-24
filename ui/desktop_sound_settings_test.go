package ui

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"testing"
)

var desktopSoundEvents = []string{
	"session.start", "notify.message", "notify.info", "notify.success", "notify.warning", "notify.error",
	"dialog.open", "dialog.confirm", "dialog.cancel", "window.open", "window.close", "window.maximize", "window.minimize",
	"window.restore", "window.snap", "window.deny", "file.drop", "file.trash", "file.delete", "space.switch",
	"menu.open", "menu.close", "ui.select", "ui.toggle.on", "ui.toggle.off", "ui.tick",
}

// desktopSoundEventSpec mirrors EVENT_CATEGORY / EVENT_VARIANTS in sound-runtime.js and ROLES in sound-engine.js.
var desktopSoundEventSpec = map[string]struct {
	category string
	variants int
}{
	"session.start": {"startup", 1}, "notify.message": {"notifications", 1}, "notify.info": {"notifications", 1},
	"notify.success": {"notifications", 1}, "notify.warning": {"notifications", 1}, "notify.error": {"notifications", 1},
	"dialog.open": {"files", 1}, "dialog.confirm": {"files", 1}, "dialog.cancel": {"files", 1},
	"window.open": {"windows", 3}, "window.close": {"windows", 3}, "window.maximize": {"windows", 3},
	"window.minimize": {"windows", 3}, "window.restore": {"windows", 3}, "window.snap": {"windows", 3},
	"window.deny": {"windows", 2}, "file.drop": {"files", 3}, "file.trash": {"files", 2}, "file.delete": {"files", 2},
	"space.switch": {"navigation", 2}, "menu.open": {"navigation", 3}, "menu.close": {"navigation", 3},
	"ui.select": {"interface", 3}, "ui.toggle.on": {"interface", 3}, "ui.toggle.off": {"interface", 3}, "ui.tick": {"interface", 3},
}

var desktopSoundThemeIDs = []string{
	"crystal", "wood", "analog", "workshop", "water", "amiga", "cyberpunk", "black_gloss", "crazy", "lollipop",
	"organic", "electronic_dreams", "windows_xp", "green_slime", "voxel_world", "orbit",
}

var desktopSoundThemeFiles = []string{
	"crystal", "wood", "analog", "workshop", "water", "amiga", "cyberpunk", "black-gloss", "crazy", "lollipop",
	"organic", "electronic-dreams", "windows-xp", "green-slime", "voxel-world", "orbit",
}

var desktopSoundLocaleKeys = []string{
	"desktop.settings_theme_cursors",
	"desktop.settings_theme_cursors_desc",
	"desktop.settings_cursor_preview",
	"desktop.settings_sound_theme_desktop",
	"desktop.settings_sound_theme_desktop_desc",
	"desktop.settings_sound_preview_playing",
	"desktop.settings_sound_preview_failed",
	"desktop.settings_sound_theme_amiga_desc",
	"desktop.settings_sound_theme_cyberpunk_desc",
	"desktop.settings_sound_theme_black_gloss_desc",
	"desktop.settings_sound_theme_crazy_desc",
	"desktop.settings_sound_theme_lollipop_desc",
	"desktop.settings_sound_theme_organic_desc",
	"desktop.settings_sound_theme_electronic_dreams",
	"desktop.settings_sound_theme_electronic_dreams_desc",
	"desktop.settings_sound_theme_windows_xp",
	"desktop.settings_sound_theme_windows_xp_desc",
	"desktop.settings_sound_theme_green_slime",
	"desktop.settings_sound_theme_green_slime_desc",
	"desktop.settings_sound_theme_voxel_world",
	"desktop.settings_sound_theme_voxel_world_desc",
	"desktop.settings_sound_theme_orbit",
	"desktop.settings_sound_theme_orbit_desc",
	"desktop.settings_category_sound",
	"desktop.settings_category_sound_desc",
	"desktop.settings_sound_enabled",
	"desktop.settings_sound_enabled_desc",
	"desktop.settings_sound_theme",
	"desktop.settings_sound_theme_desc",
	"desktop.settings_sound_theme_crystal",
	"desktop.settings_sound_theme_crystal_desc",
	"desktop.settings_sound_theme_wood",
	"desktop.settings_sound_theme_wood_desc",
	"desktop.settings_sound_theme_analog",
	"desktop.settings_sound_theme_analog_desc",
	"desktop.settings_sound_theme_workshop",
	"desktop.settings_sound_theme_workshop_desc",
	"desktop.settings_sound_theme_water",
	"desktop.settings_sound_theme_water_desc",
	"desktop.settings_sound_preview",
	"desktop.settings_sound_volume",
	"desktop.settings_sound_volume_desc",
	"desktop.settings_sound_windows",
	"desktop.settings_sound_windows_desc",
	"desktop.settings_sound_notifications",
	"desktop.settings_sound_notifications_desc",
	"desktop.settings_sound_navigation",
	"desktop.settings_sound_navigation_desc",
	"desktop.settings_sound_files",
	"desktop.settings_sound_files_desc",
	"desktop.settings_sound_interface",
	"desktop.settings_sound_interface_desc",
	"desktop.settings_sound_startup",
	"desktop.settings_sound_startup_desc",
	"desktop.settings_sound_preview_category",
}

func TestDesktopSoundRuntimeMarkers(t *testing.T) {
	t.Parallel()

	main := readDesktopAssetText(t, "js/desktop/bundles/main.bundle.js")
	for _, marker := range []string{
		"function desktopSound(",
		"function syncDesktopSoundSettings(",
		"function previewDesktopSound(",
		"window.DesktopSounds = {",
		"loader('desktop-sounds')",
		"applySoundSettingsChange",
		"sound: 'speaker'",
		"function previewSoundCategory(",
		"renderEvent",
		"new Worker(",
		"DesktopSoundsRenderGate",
	} {
		if !strings.Contains(main, marker) {
			t.Fatalf("main bundle missing sound marker %q", marker)
		}
	}

	mini := readDesktopAssetText(t, "js/desktop/core/mini-icons-runtime.js")
	if !strings.Contains(mini, "sound: 'speaker'") {
		t.Fatal("mini-icons-runtime must map sound-symbolic to the speaker artwork")
	}

	runtime := readDesktopAssetText(t, "js/desktop/core/sound-runtime.js")
	if strings.Contains(runtime, "new AudioContext") || strings.Contains(runtime, "new (window.AudioContext") {
		t.Fatal("sound-runtime must not construct AudioContext directly; use ensureContext Ctor path")
	}
	if !strings.Contains(runtime, "actx = new Ctor()") {
		t.Fatal("sound-runtime missing guarded AudioContext construction")
	}
	for _, event := range desktopSoundEvents {
		spec := desktopSoundEventSpec[event]
		if marker := fmt.Sprintf("'%s': '%s'", event, spec.category); !strings.Contains(runtime, marker) {
			t.Errorf("sound-runtime missing category marker %q", marker)
		}
		if marker := fmt.Sprintf("'%s': %d", event, spec.variants); !strings.Contains(runtime, marker) {
			t.Errorf("sound-runtime missing variant marker %q", marker)
		}
	}

	loader := readDesktopAssetText(t, "js/desktop/core/module-loader.js")
	if !strings.Contains(loader, "desktop-sounds") {
		t.Fatal("module-loader missing desktop-sounds bundle path")
	}

	bundle := readDesktopAssetText(t, "js/desktop/bundles/desktop-sounds.bundle.js")
	bundleMarkers := []string{
		"root.DesktopSoundDSP",
		"root.DesktopSoundInstruments",
		"root.DesktopSoundEngine",
		"/* ui/js/desktop/sound/render-host.js */",
	}
	for _, id := range desktopSoundThemeIDs {
		bundleMarkers = append(bundleMarkers, "id: '"+id+"'")
	}
	for _, marker := range bundleMarkers {
		if !strings.Contains(bundle, marker) {
			t.Fatalf("desktop-sounds bundle missing marker %q", marker)
		}
	}
	for _, forbidden := range []string{"DesktopSoundSynth", "Math.random("} {
		if strings.Contains(bundle, forbidden) {
			t.Fatalf("desktop-sounds bundle must not contain %q", forbidden)
		}
	}
}

func TestDesktopSoundHookMarkers(t *testing.T) {
	t.Parallel()

	hooks := map[string][]string{
		"js/desktop/core/window-shell-runtime.js": {
			"desktopSound('window.open')",
		},
		"js/desktop/core/window-interactions-runtime.js": {
			"desktopSound('window.minimize')",
			"desktopSound('window.snap')",
			"'window.maximize'",
			"desktopSound('window.restore')",
			"desktopSound('window.deny')",
			"desktopSound('window.close')",
		},
		"js/desktop/core/sdk-events-bootstrap.js": {
			"function notificationSoundEvent(",
			"'notify.message'",
			"'notify.error'",
			"'notify.warning'",
			"'notify.success'",
			"'notify.info'",
			"desktopSound('session.start')",
		},
		"js/desktop/core/desktop-foundation.js": {
			"desktopSound('menu.open')",
			"desktopSound('menu.close')",
		},
		"js/desktop/core/menus-and-routing.js": {
			"desktopSound('menu.open')",
			"desktopSound('dialog.open')",
			"desktopSound('dialog.confirm')",
			"desktopSound('dialog.cancel')",
			"desktopSound('file.delete')",
			"desktopSound('file.trash')",
			"previewDesktopSound",
			"setDesktopSoundVolume",
		},
		"js/desktop/core/spotlight-runtime.js": {
			"desktopSound('menu.open')",
			"desktopSound('menu.close')",
		},
		"js/desktop/core/shell-chrome-runtime.js": {
			"desktopSound('menu.open')",
			"desktopSound('menu.close')",
		},
		"js/desktop/core/spaces-runtime.js": {
			"desktopSound('space.switch')",
		},
		"js/desktop/core/desktop-file-drops.js": {
			"desktopSound('file.drop')",
		},
	}
	for path, markers := range hooks {
		source := readDesktopAssetText(t, path)
		for _, marker := range markers {
			if !strings.Contains(source, marker) {
				t.Fatalf("%s missing hook marker %q", path, marker)
			}
		}
	}
}

func TestDesktopSoundThemeEvents(t *testing.T) {
	t.Parallel()

	eventRe := regexp.MustCompile(`'([a-z]+(?:\.[a-z]+)+)':`)
	for i, theme := range desktopSoundThemeFiles {
		source := readDesktopAssetText(t, "js/desktop/sound/theme-"+theme+".js")
		for _, marker := range []string{"self.DesktopSoundEngine.defineTheme({", "id: '" + desktopSoundThemeIDs[i] + "'"} {
			if !strings.Contains(source, marker) {
				t.Errorf("theme %s missing marker %q", theme, marker)
			}
		}
		found := map[string]bool{}
		for _, match := range eventRe.FindAllStringSubmatch(source, -1) {
			found[match[1]] = true
		}
		for _, event := range desktopSoundEvents {
			if !found[event] {
				t.Errorf("theme %s missing event %q", theme, event)
			}
		}
	}
	voxel := readDesktopAssetText(t, "js/desktop/sound/theme-voxel-world.js")
	for _, marker := range []string{"fx:", "'window.fracture'", "'window.impact'", "wood:", "stone:", "earth:"} {
		if !strings.Contains(voxel, marker) {
			t.Errorf("voxel-world bank missing material sound marker %q", marker)
		}
	}
	orbit := readDesktopAssetText(t, "js/desktop/sound/theme-orbit.js")
	for _, marker := range []string{"fx:", "'orbit.ping'", "small:", "medium:", "large:"} {
		if !strings.Contains(orbit, marker) {
			t.Errorf("orbit bank missing marker %q", marker)
		}
	}
}

func TestDesktopThemeFxSoundRuntimeContract(t *testing.T) {
	t.Parallel()

	// Voxel World and Green Slime share the theme FX plumbing: one table of families, one coalescing queue
	// and one lazily rendered variant cache.
	runtime := readDesktopAssetText(t, "js/desktop/core/sound-runtime.js")
	for _, marker := range []string{
		"'window.fracture': 'windows'", "'window.impact': 'windows'", "'slime.pop': 'windows'", "'slime.snap': 'windows'",
		"theme: 'voxel_world'", "theme: 'green_slime'", "'slime.pop': { gain: 0.4, gapMs: 350 }", "'slime.snap': { gain: 0.42, gapMs: 900 }",
		"'orbit.ping': 'navigation'", "theme: 'orbit'", "'orbit.ping': { gain: 0.38, gapMs: 350 }",
		"windowMs: 2000, maxInWindow: 3", "coalesceMs: 80, expireMs: 240",
		"function queueThemeFx(", "function ensureThemeFxBuffer(", "currentThemeId() !== fx.theme",
		"cancelThemeFx()", "cleanThemeFxOptions", "options.pan", "options.strength", "themeFx: Object.fromEntries",
	} {
		if !strings.Contains(runtime, marker) {
			t.Errorf("sound-runtime missing theme FX contract marker %q", marker)
		}
	}
	if strings.Contains(runtime, "Voxel") {
		t.Error("sound-runtime still carries voxel-only FX plumbing")
	}
}

func TestGreenSlimeSoundFxContract(t *testing.T) {
	t.Parallel()

	bank := readDesktopAssetText(t, "js/desktop/sound/theme-green-slime.js")
	for _, marker := range []string{"fx:", "'slime.pop'", "'slime.snap'", "small:", "medium:", "large:"} {
		if !strings.Contains(bank, marker) {
			t.Errorf("green-slime bank missing FX marker %q", marker)
		}
	}

	// The effect only reports; the sound runtime decides whether anything is heard.
	bridge := readDesktopAssetText(t, "js/desktop/theme-fx/green_slime.js")
	for _, marker := range []string{
		"function sound(eventName, variant, strength, x)", "!motion || document.hidden", "window.DesktopSounds.play(eventName",
		"slime.soundCue(event)", "sim.takeEvents()", "soundEvents",
	} {
		if !strings.Contains(bridge, marker) {
			t.Errorf("green_slime.js missing sound bridge marker %q", marker)
		}
	}
	sim := readDesktopAssetText(t, "js/desktop/theme-fx/green_slime_sim.js")
	bubbles := readDesktopAssetText(t, "js/desktop/theme-fx/green_slime_bubbles.js")
	touch := readDesktopAssetText(t, "js/desktop/theme-fx/green_slime_touch.js")
	for source, marker := range map[string]string{
		sim: "function soundCue(event)", bubbles: "core.emit({ type: 'burst'", touch: "core.emit({ type: 'snap'",
	} {
		if !strings.Contains(source, marker) {
			t.Errorf("Green Slime simulation missing event marker %q", marker)
		}
	}
}

func TestDesktopSoundSettingsUIMarkers(t *testing.T) {
	t.Parallel()

	catalog := readDesktopAssetText(t, "js/desktop/core/settings-catalog-runtime.js")
	settings := readDesktopAssetText(t, "js/desktop/apps/settings.js")
	for _, marker := range []string{
		"id: 'sound', icon: 'sound-symbolic'",
		"settingToggle('sound.enabled'",
		"type: 'sound_theme'",
		"type: 'range'",
		"settingToggle('sound.windows'",
		"settingToggle('sound.notifications'",
		"settingToggle('sound.navigation'",
		"settingToggle('sound.files'",
		"settingToggle('sound.interface'",
		"settingToggle('sound.startup'",
	} {
		if !strings.Contains(catalog, marker) {
			t.Fatalf("settings catalog missing marker %q", marker)
		}
	}
	for _, marker := range []string{
		"data-sound-preview",
		"data-sound-category-preview",
		"nextPane.scrollTop = paneScroll",
		"key !== 'sound.volume'",
	} {
		if !strings.Contains(settings, marker) {
			t.Fatalf("settings.js missing marker %q", marker)
		}
	}
	css := readDesktopAssetText(t, "css/desktop-app-settings.css")
	for _, marker := range []string{
		".vd-sound-theme-grid",
		".vd-sound-category-actions",
		".vd-sound-theme-card",
		".vd-setting-range-wrap",
		".vd-settings-pane-icon .vd-theme-icon",
		"mask: var(--vd-theme-icon-url)",
	} {
		if !strings.Contains(css, marker) {
			t.Fatalf("desktop-app-settings.css missing marker %q", marker)
		}
	}
}

func TestDesktopSoundTranslations(t *testing.T) {
	t.Parallel()

	for _, lang := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		var values map[string]string
		if err := json.Unmarshal([]byte(readDesktopAssetText(t, "lang/desktop/"+lang+".json")), &values); err != nil {
			t.Fatalf("%s: %v", lang, err)
		}
		for _, key := range desktopSoundLocaleKeys {
			if strings.TrimSpace(values[key]) == "" {
				t.Errorf("%s missing or empty %s", lang, key)
			}
		}
	}
}
