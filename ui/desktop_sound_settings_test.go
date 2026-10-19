package ui

import (
	"encoding/json"
	"regexp"
	"strings"
	"testing"
)

var desktopSoundEvents = []string{
	"window.open", "window.close", "window.minimize", "window.restore", "window.maximize",
	"window.snap", "window.deny", "notify.info", "notify.message", "notify.error",
	"menu.open", "menu.close", "space.switch", "dialog.open", "dialog.confirm", "dialog.cancel",
	"file.trash", "file.delete", "file.drop",
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

	loader := readDesktopAssetText(t, "js/desktop/core/module-loader.js")
	if !strings.Contains(loader, "desktop-sounds") {
		t.Fatal("module-loader missing desktop-sounds bundle path")
	}

	bundle := readDesktopAssetText(t, "js/desktop/bundles/desktop-sounds.bundle.js")
	for _, marker := range []string{
		"window.DesktopSoundSynth",
		"window.DesktopSoundThemes",
		"DesktopSoundThemes.crystal",
		"DesktopSoundThemes.wood",
		"DesktopSoundThemes.analog",
		"DesktopSoundThemes.workshop",
		"DesktopSoundThemes.water",
		"DesktopSoundThemes.electronic_dreams",
		"DesktopSoundThemes.windows_xp",
		"DesktopSoundThemes.green_slime",
		"DesktopSoundThemes.voxel_world",
	} {
		if !strings.Contains(bundle, marker) {
			t.Fatalf("desktop-sounds bundle missing marker %q", marker)
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
			"desktopSound('notify.message')",
			"desktopSound('notify.error')",
			"desktopSound('notify.info')",
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
	for _, theme := range []string{"crystal", "wood", "analog", "workshop", "water", "desktop"} {
		source := readDesktopAssetText(t, "js/desktop/sound/theme-"+theme+".js")
		found := map[string]bool{}
		for _, match := range eventRe.FindAllStringSubmatch(source, -1) {
			found[match[1]] = true
		}
		for _, event := range desktopSoundEvents {
			if !found[event] {
				t.Fatalf("theme %s missing event %q", theme, event)
			}
		}
	}
	for _, theme := range []string{"electronic-dreams", "windows-xp", "green-slime", "voxel-world"} {
		source := readDesktopAssetText(t, "js/desktop/sound/theme-"+theme+".js")
		if !strings.Contains(source, "composeDesktopEvents") {
			t.Errorf("theme %s must compose a complete event bank from its authored profile", theme)
		}
	}
	voxel := readDesktopAssetText(t, "js/desktop/sound/theme-voxel-world.js")
	for _, marker := range []string{"specialFx", "'window.fracture'", "'window.impact'", "'wood'", "'stone'", "'earth'"} {
		if !strings.Contains(voxel, marker) {
			t.Errorf("voxel-world bank missing material sound marker %q", marker)
		}
	}
}

func TestDesktopVoxelSoundRuntimeContract(t *testing.T) {
	t.Parallel()

	runtime := readDesktopAssetText(t, "js/desktop/core/sound-runtime.js")
	for _, marker := range []string{
		"'window.fracture': 'windows'", "'window.impact': 'windows'",
		"function queueVoxelFx(", "function ensureVoxelFxBuffer(", "currentThemeId() !== 'voxel_world'",
		"cancelVoxelFx()", "cleanVoxelFxOptions", "options.pan", "options.strength",
	} {
		if !strings.Contains(runtime, marker) {
			t.Errorf("sound-runtime missing voxel FX contract marker %q", marker)
		}
	}
}

func TestDesktopSoundSettingsUIMarkers(t *testing.T) {
	t.Parallel()

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
		"data-sound-preview",
		"nextPane.scrollTop = paneScroll",
		"key === 'sound.volume'",
	} {
		if !strings.Contains(settings, marker) {
			t.Fatalf("settings.js missing marker %q", marker)
		}
	}

	css := readDesktopAssetText(t, "css/desktop-app-settings.css")
	for _, marker := range []string{
		".vd-sound-theme-grid",
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
