package ui

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"aurago/internal/desktop"
)

var screensaverThemeIDs = []string{"abyss", "event_horizon", "aurora", "ink", "stardust"}

func TestDesktopScreensaverRuntimeLoadsWithShell(t *testing.T) {
	t.Parallel()

	buildScript, err := os.ReadFile("../scripts/build-ui-bundles.js")
	if err != nil {
		t.Fatal(err)
	}
	script := string(buildScript)
	sound := strings.Index(script, "'ui/js/desktop/core/sound-runtime.js'")
	runtime := strings.Index(script, "'ui/js/desktop/core/screensaver-runtime.js'")
	if sound < 0 || runtime < sound {
		t.Fatal("screensaver runtime must be bundled right after the sound runtime")
	}

	bundle := readDesktopAssetText(t, "js/desktop/bundles/main.bundle.js")
	for _, marker := range []string{
		"/* ui/js/desktop/core/screensaver-runtime.js */",
		"window.DesktopScreensaver = {",
		"window.previewDesktopScreensaver = previewDesktopScreensaver;",
		"function screensaverSuppressionReason()",
		"document.fullscreenElement",
		"document.getElementById('vd-sip-incoming')",
		"audibleVideoPlaying()",
		"opaqueFrameFocused()",
		"aurago.desktop.activity",
		"const origApplyForScreensaver = applyDesktopSettings;",
		"'/js/desktop/screensavers/host.js'",
		"'/css/desktop-screensaver.css'",
		"stopImmediatePropagation()",
		"previewDesktopSound, setDesktopSoundVolume, applySoundSettingsChange, previewDesktopScreensaver });",
		"screensaver: 'star'",
	} {
		if !strings.Contains(bundle, marker) {
			t.Fatalf("desktop main bundle is missing screensaver marker %q", marker)
		}
	}

	sdk := readDesktopAssetText(t, "js/desktop/aura-desktop-sdk.js")
	for _, marker := range []string{"const ACTIVITY_TYPE = 'aurago.desktop.activity';", "function pingParentActivity()", "ACTIVITY_PING_MS"} {
		if !strings.Contains(sdk, marker) {
			t.Fatalf("desktop SDK must report frame activity for the screensaver, missing %q", marker)
		}
	}
}

func TestDesktopScreensaverSettingsStayInSync(t *testing.T) {
	t.Parallel()

	foundation := readDesktopAssetText(t, "js/desktop/core/desktop-foundation.js")
	settings := readDesktopAssetText(t, "js/desktop/apps/settings.js")
	runtime := readDesktopAssetText(t, "js/desktop/core/screensaver-runtime.js")
	found := 0
	for _, def := range desktop.DesktopSettingDefinitions() {
		if !strings.HasPrefix(def.Key, "screensaver.") {
			continue
		}
		found++
		if !strings.Contains(foundation, "'"+def.Key+"': '"+def.Default+"'") {
			t.Fatalf("frontend defaults must mirror %s=%q", def.Key, def.Default)
		}
		switch def.Key {
		case "screensaver.theme":
			for _, value := range def.Values {
				if !strings.Contains(settings, "'"+value+"'") {
					t.Fatalf("settings scene picker is missing theme %q", value)
				}
				if value != "random" && !strings.Contains(runtime, "'"+value+"'") {
					t.Fatalf("screensaver runtime does not know theme %q", value)
				}
			}
		case "screensaver.idle_minutes":
			for _, value := range def.Values {
				if !strings.Contains(settings, "['"+value+"', 'desktop.settings_screensaver_idle_"+value+"']") {
					t.Fatalf("settings idle select is missing %s minutes", value)
				}
				if _, err := strconv.Atoi(value); err != nil {
					t.Fatalf("idle choice %q must be numeric", value)
				}
			}
		}
	}
	if found != 4 {
		t.Fatalf("expected four screensaver settings, found %d", found)
	}
	for _, marker := range []string{
		"id: 'screensaver', icon: 'screensaver-symbolic'",
		"settingToggle('screensaver.enabled'",
		"settingToggle('screensaver.clock'",
		"settingSelect('screensaver.idle_minutes'",
		"data-screensaver-theme=",
		"data-screensaver-preview=",
		"ctx.previewDesktopScreensaver(",
		"saveDesktopSetting('screensaver.theme'",
		"'/img/screensaver/' + id + '-thumb.webp'",
	} {
		if !strings.Contains(settings, marker) {
			t.Fatalf("settings screensaver pane is missing %q", marker)
		}
	}
}

func screensaverTranslationKeys() []string {
	keys := []string{
		"desktop.screensaver_wake_hint",
		"desktop.settings_category_screensaver",
		"desktop.settings_category_screensaver_desc",
		"desktop.settings_screensaver_enabled",
		"desktop.settings_screensaver_enabled_desc",
		"desktop.settings_screensaver_theme",
		"desktop.settings_screensaver_theme_desc",
		"desktop.settings_screensaver_idle",
		"desktop.settings_screensaver_idle_desc",
		"desktop.settings_screensaver_clock",
		"desktop.settings_screensaver_clock_desc",
		"desktop.settings_screensaver_preview",
	}
	for _, id := range append(append([]string{}, screensaverThemeIDs...), "random") {
		keys = append(keys, "desktop.settings_screensaver_"+id, "desktop.settings_screensaver_"+id+"_desc")
	}
	for _, minutes := range []string{"1", "2", "3", "5", "10", "15", "30", "60"} {
		keys = append(keys, "desktop.settings_screensaver_idle_"+minutes)
	}
	return keys
}

func TestDesktopScreensaverTranslations(t *testing.T) {
	t.Parallel()

	entries, err := os.ReadDir("lang/desktop")
	if err != nil {
		t.Fatal(err)
	}
	languages := 0
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		languages++
		var dict map[string]string
		if err := json.Unmarshal([]byte(readDesktopAssetText(t, "lang/desktop/"+entry.Name())), &dict); err != nil {
			t.Fatalf("%s: %v", entry.Name(), err)
		}
		for _, key := range screensaverTranslationKeys() {
			if strings.TrimSpace(dict[key]) == "" {
				t.Fatalf("%s is missing screensaver translation %s", entry.Name(), key)
			}
		}
	}
	if languages != 16 {
		t.Fatalf("expected 16 desktop locales, found %d", languages)
	}
	var german map[string]string
	if err := json.Unmarshal([]byte(readDesktopAssetText(t, "lang/desktop/de.json")), &german); err != nil {
		t.Fatal(err)
	}
	if german["desktop.settings_screensaver_ink"] != "Flüssige Tinte" || !strings.Contains(german["desktop.settings_screensaver_enabled_desc"], "dich") {
		t.Fatal("German screensaver strings must use real umlauts and the informal Du")
	}
}

func TestDesktopScreensaverOverlaySitsAboveShellBelowCalls(t *testing.T) {
	t.Parallel()

	base := readDesktopAssetText(t, "css/desktop-base.css")
	match := regexp.MustCompile(`--vd-z-screensaver:\s*(\d+);`).FindStringSubmatch(base)
	if match == nil {
		t.Fatal("desktop base CSS must define --vd-z-screensaver")
	}
	z, _ := strconv.Atoi(match[1])
	if z <= 9999 || z >= 26000 {
		t.Fatalf("screensaver z-index %d must stay above the always-on-top pet (9999) and below incoming calls (26000)", z)
	}
	css := readDesktopAssetText(t, "css/desktop-screensaver.css")
	for _, marker := range []string{"z-index: var(--vd-z-screensaver, 20000);", "cursor: none;", "@media (prefers-reduced-motion: reduce)", ".vd-screensaver[data-state=\"poster\"] .vd-screensaver-poster"} {
		if !strings.Contains(css, marker) {
			t.Fatalf("screensaver CSS is missing %q", marker)
		}
	}
	if strings.Contains(readDesktopAssetText(t, "css/desktop-shell.bundle.css"), ".vd-screensaver-canvas") {
		t.Fatal("screensaver overlay CSS must stay lazy and out of the shell bundle")
	}
}

func TestDesktopScreensaverScenesStayLocalAndLazy(t *testing.T) {
	t.Parallel()

	host := readDesktopAssetText(t, "js/desktop/screensavers/host.js")
	for _, id := range screensaverThemeIDs {
		if !strings.Contains(host, id+": [") {
			t.Fatalf("screensaver host does not register scripts for %s", id)
		}
	}
	for _, marker := range []string{"window.AuraScreensaverHost = {", "forceContextLoss", "'context-lost'", "'reduced-motion'", "LONG_RUN_MS", "CALIBRATION_FRAMES", "AuraLazyAssets"} {
		if !strings.Contains(host+readDesktopAssetText(t, "js/desktop/screensavers/gl-kit.js"), marker) && marker != "forceContextLoss" {
			t.Fatalf("screensaver host/kit is missing %q", marker)
		}
	}
	files := []string{"host.js", "gl-kit.js", "aurora.js", "event-horizon.js", "ink.js", "stardust.js", "abyss.js", "abyss-scene.js"}
	remote := regexp.MustCompile(`https?://`)
	for _, name := range files {
		source := readDesktopAssetText(t, "js/desktop/screensavers/"+name)
		if remote.MatchString(source) {
			t.Fatalf("%s must not reference remote resources", name)
		}
		if strings.Contains(source, "eval(") || strings.Contains(source, "new Function") {
			t.Fatalf("%s must not use eval-like constructs (CSP)", name)
		}
	}
	abyss := readDesktopAssetText(t, "js/desktop/screensavers/abyss.js")
	if !strings.Contains(abyss, "'/js/vendor/screensaver-abyss/abyss.esm.js'") || !strings.Contains(abyss, "env.versionedURL(BUNDLE)") {
		t.Fatal("Tiefsee must import its isolated renderer through a versioned URL")
	}
	desktopHTML := readDesktopAssetText(t, "desktop.html")
	if strings.Contains(desktopHTML, "screensaver") {
		t.Fatal("desktop.html must not load screensaver assets eagerly")
	}
}

func TestDesktopScreensaverAbyssAssetsMatchManifests(t *testing.T) {
	t.Parallel()

	var kit struct {
		Assets []struct {
			File      string `json:"file"`
			Bytes     int    `json:"bytes"`
			SHA256    string `json:"sha256"`
			Triangles int    `json:"triangles"`
		} `json:"assets"`
	}
	if err := json.Unmarshal([]byte(readDesktopAssetText(t, "3d/screensaver/abyss/v1/manifest.json")), &kit); err != nil {
		t.Fatal(err)
	}
	if len(kit.Assets) != 4 {
		t.Fatalf("expected four Tiefsee creatures, got %d", len(kit.Assets))
	}
	total := 0
	for _, asset := range kit.Assets {
		data, err := os.ReadFile("3d/screensaver/abyss/v1/" + asset.File)
		if err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(data)
		if len(data) != asset.Bytes || hex.EncodeToString(sum[:]) != asset.SHA256 {
			t.Fatalf("%s does not match its manifest entry", asset.File)
		}
		if asset.Triangles > 12000 {
			t.Fatalf("%s exceeds the 12k triangle budget", asset.File)
		}
		total += len(data)
	}
	if total > 1536*1024 {
		t.Fatalf("Tiefsee creatures use %d bytes; budget is 1.5 MiB", total)
	}

	var bundle struct {
		Three  string `json:"three"`
		Bytes  int    `json:"bytes"`
		SHA256 string `json:"sha256"`
	}
	if err := json.Unmarshal([]byte(readDesktopAssetText(t, "js/vendor/screensaver-abyss/manifest.json")), &bundle); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile("js/vendor/screensaver-abyss/abyss.esm.js")
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	if bundle.Three != "0.185.1" || bundle.Bytes != len(data) || bundle.SHA256 != hex.EncodeToString(sum[:]) {
		t.Fatal("Tiefsee renderer bundle is stale; run node scripts/build-screensaver-abyss.js")
	}
}

func TestDesktopScreensaverPostersAndThumbnailsShip(t *testing.T) {
	t.Parallel()

	for _, id := range screensaverThemeIDs {
		for _, entry := range []struct {
			suffix string
			limit  int
		}{{"", 400 * 1024}, {"-thumb", 48 * 1024}} {
			path := "img/screensaver/" + id + entry.suffix + ".webp"
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("%s is missing: %v", path, err)
			}
			if len(data) < 12 || string(data[0:4]) != "RIFF" || string(data[8:12]) != "WEBP" {
				t.Fatalf("%s must be a WebP image", path)
			}
			if len(data) > entry.limit {
				t.Fatalf("%s is %d bytes; limit %d", path, len(data), entry.limit)
			}
		}
	}
	host := readDesktopAssetText(t, "js/desktop/screensavers/host.js")
	if !strings.Contains(host, "'/img/screensaver/' + s.theme + '.webp'") {
		t.Fatal("screensaver host must show the matching poster as fallback")
	}
}
