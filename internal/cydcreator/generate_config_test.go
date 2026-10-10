package cydcreator

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// loadReferenceProject parses plan 1b's reference layout (a fresh copy per call).
func loadReferenceProject(t *testing.T) Project {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(referenceDir, "cyd.project.json"))
	if err != nil {
		t.Fatal(err)
	}
	p, err := ParseProject(raw)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// The hand-written profiles of plan 1b are the board_config.h and
// platformio.ini contract; the renderers must reproduce them exactly.
func TestRenderConfigMatchesReferenceProfiles(t *testing.T) {
	boards, err := Boards()
	if err != nil {
		t.Fatal(err)
	}
	p := loadReferenceProject(t)
	for _, b := range boards {
		dir := filepath.Join(referenceDir, "profiles", b.ID)
		wantCfg, err := os.ReadFile(filepath.Join(dir, "include", "board_config.h"))
		if err != nil {
			t.Fatal(err)
		}
		if got := renderBoardConfig(p, b, "ccp_000000000001"); got != string(wantCfg) {
			t.Errorf("%s board_config.h:\n%s", b.ID, firstDiff(string(wantCfg), got))
		}
		wantIni, err := os.ReadFile(filepath.Join(dir, "platformio.ini"))
		if err != nil {
			t.Fatal(err)
		}
		if got := renderPlatformIOIni(p, b); got != string(wantIni) {
			t.Errorf("%s platformio.ini:\n%s", b.ID, firstDiff(string(wantIni), got))
		}
	}
}

func TestRenderBoardConfigProjectMacros(t *testing.T) {
	p := loadReferenceProject(t)
	p.Name = `Say "hi" \ to Ünï` + "\t!"
	p.Timezone = "<+03>-3"
	p.Orientation = "portrait_flipped"
	p.Language = "zh"
	p.Settings.WiFi = "required"
	p.Settings.AuraGoLink = false
	p.Settings.Backlight = 40
	p.Settings.StatusbarClock = false
	b, err := BoardByID("esp32-2432s028r")
	if err != nil {
		t.Fatal(err)
	}
	cfg := renderBoardConfig(p, b, `ccp_"x"`)
	for _, want := range []string{
		"#define CYD_PROJECT_ID \"ccp_\\\"x\\\"\"\n",
		"#define CYD_PROJECT_NAME \"Say \\\"hi\\\" \\\\ to Ünï\\011!\"\n",
		"#define CYD_SCREEN_W 240\n#define CYD_SCREEN_H 320\n#define CYD_ROTATION 3\n",
		"#define CYD_WIFI_MODE 2\n#define CYD_AURAGO_LINK 0\n",
		"#define CYD_LANG \"en\"\n#define CYD_LANG_EN 1\n",
		"#define CYD_TZ \"<+03>-3\"\n",
		"#define CYD_BACKLIGHT_DEFAULT 40\n#define CYD_STATUSBAR_CLOCK 0\n",
	} {
		if !strings.Contains(cfg, want) {
			t.Errorf("board_config.h lacks %q", want)
		}
	}
}

func TestGenLibDeps(t *testing.T) {
	got := genLibDeps([]string{"bme280", "arduinojson", "dht", "dallastemperature", "onewire", "unknown"})
	want := []string{
		"lvgl/lvgl@9.5.0", "lovyan03/LovyanGFX@1.2.30", "tzapu/WiFiManager@2.0.17", "bblanchon/ArduinoJson@7.4.3",
		"adafruit/Adafruit BME280 Library@2.3.0", "adafruit/Adafruit BusIO@1.17.4", "adafruit/Adafruit Unified Sensor@1.1.15",
		"adafruit/DHT sensor library@1.4.7", "milesburton/DallasTemperature@4.0.6", "paulstoffregen/OneWire@2.3.8",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("genLibDeps =\n%q\nwant\n%q", got, want)
	}
	// Every curated spec must be one the builder image pre-installs.
	for _, lib := range Libraries() {
		for _, spec := range append([]string{lib.PIOSpec}, lib.Depends...) {
			if !slices.Contains(genLibDeps([]string{lib.ID}), spec) {
				t.Errorf("%s: %s missing from lib_deps", lib.ID, spec)
			}
		}
	}
}

func TestGenRotation(t *testing.T) {
	landscape := map[string]int{"landscape": 0, "portrait": 1, "landscape_flipped": 2, "portrait_flipped": 3}
	for o, want := range landscape {
		if got := genRotation("landscape", o); got != want {
			t.Errorf("native landscape %s = %d, want %d", o, got, want)
		}
	}
	portrait := map[string]int{"portrait": 0, "landscape": 1, "portrait_flipped": 2, "landscape_flipped": 3}
	for o, want := range portrait {
		if got := genRotation("portrait", o); got != want {
			t.Errorf("native portrait %s = %d, want %d", o, got, want)
		}
	}
}
