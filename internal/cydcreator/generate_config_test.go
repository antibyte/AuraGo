package cydcreator

import (
	"io/fs"
	"maps"
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

// Every display driver of the board schema becomes exactly one
// CYD_PANEL_DRIVER_<driver> macro (ILI9341_2 must not also define ILI9341), and
// the template's cyd_board.cpp selects a LovyanGFX panel class for it.
func TestRenderBoardConfigPanelDriverMacro(t *testing.T) {
	src, err := fs.ReadFile(TemplateFS(), "lib/cyd_template/src/cyd_board.cpp")
	if err != nil {
		t.Fatal(err)
	}
	chain := panelSelectionChain(t, string(src))
	p := loadReferenceProject(t)
	b, err := BoardByID("esp32-2432s028r")
	if err != nil {
		t.Fatal(err)
	}
	for _, driver := range slices.Sorted(maps.Keys(displayDriverBus)) {
		b.Display.Driver = driver
		var macros []string
		for _, line := range strings.Split(renderBoardConfig(p, b, "ccp_000000000001"), "\n") {
			if strings.HasPrefix(line, "#define CYD_PANEL_DRIVER_") {
				macros = append(macros, line)
			}
		}
		if want := []string{"#define CYD_PANEL_DRIVER_" + driver + " 1"}; !slices.Equal(macros, want) {
			t.Errorf("%s: driver macros %q, want %q", driver, macros, want)
		}
		if !strings.Contains(chain, "defined(CYD_PANEL_DRIVER_"+driver+")") {
			t.Errorf("%s: cyd_board.cpp has no CYD_PANEL_DRIVER_%s case in its panel selection", driver, driver)
		}
	}
}

// panelSelectionChain is the #if/#elif chain of cyd_board.cpp that picks the
// LovyanGFX panel class: from the last "#if defined(CYD_PANEL_DRIVER_" before
// its "#error" to that line. Include guards and later checks (the NV3041A
// rotation #error) also mention driver macros and must not count as a case.
func panelSelectionChain(t *testing.T, src string) string {
	t.Helper()
	end := strings.Index(src, `#error "board_config.h: unknown CYD_PANEL_DRIVER_*"`)
	if end < 0 {
		t.Fatal("cyd_board.cpp: the panel selection #error is missing")
	}
	start := strings.LastIndex(src[:end], "#if defined(CYD_PANEL_DRIVER_")
	if start < 0 {
		t.Fatal("cyd_board.cpp: the panel selection #if is missing")
	}
	return src[start:end]
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

// Selected libraries land in lib_deps after the template dependencies, each
// spec once (shared Depends entries are not repeated), sorted, unknown ids
// ignored.
func TestRenderPlatformIOIniWithLibraries(t *testing.T) {
	p := loadReferenceProject(t)
	p.Settings.Libraries = []string{"bme280", "dht", "bme280", "unknown"}
	b, err := BoardByID("esp32-2432s028r")
	if err != nil {
		t.Fatal(err)
	}
	ini := renderPlatformIOIni(p, b)
	const head, tail = "lib_deps =\n", "build_flags =\n"
	start, end := strings.Index(ini, head), strings.Index(ini, tail)
	if start < 0 || end < start {
		t.Fatalf("platformio.ini has no lib_deps section:\n%s", ini)
	}
	var got []string
	for _, line := range strings.Split(strings.TrimSuffix(ini[start+len(head):end], "\n"), "\n") {
		got = append(got, strings.TrimPrefix(line, "  "))
	}
	want := []string{
		"lvgl/lvgl@9.5.0", "lovyan03/LovyanGFX@1.2.30", "tzapu/WiFiManager@2.0.17", "bblanchon/ArduinoJson@7.4.3",
		"adafruit/Adafruit BME280 Library@2.3.0", "adafruit/Adafruit BusIO@1.17.4", "adafruit/Adafruit Unified Sensor@1.1.15",
		"adafruit/DHT sensor library@1.4.7",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("lib_deps =\n%q\nwant\n%q", got, want)
	}
}

// libDepConflicts lists the specs of deps that pin a second version of an
// owner/name already listed (case-insensitive; PlatformIO would not resolve
// two versions of one library).
func libDepConflicts(deps []string) []string {
	seen := map[string]string{}
	var out []string
	for _, spec := range deps {
		at := strings.LastIndex(spec, "@")
		if at <= 0 {
			out = append(out, spec+" (no @version)")
			continue
		}
		name := strings.ToLower(spec[:at])
		if prev, ok := seen[name]; ok {
			out = append(out, prev+" vs "+spec)
			continue
		}
		seen[name] = spec
	}
	return out
}

func TestLibDepConflictsDetectsSecondVersion(t *testing.T) {
	if got := libDepConflicts([]string{"a/b@1", "c/d@2", "A/B@3"}); len(got) != 1 {
		t.Fatalf("conflicts = %q, want the second a/b version", got)
	}
	if got := libDepConflicts([]string{"a/b@1", "a/b@1"}); len(got) != 1 {
		t.Fatalf("conflicts = %q, want the repeated spec", got)
	}
	if got := libDepConflicts([]string{"a/b@1", "a/c@1"}); len(got) != 0 {
		t.Fatalf("conflicts = %q, want none", got)
	}
}

// No selection of curated libraries may produce two lib_deps entries for one
// owner/name: alone, and all of them together with the template dependencies.
func TestGenLibDepsHaveNoVersionConflicts(t *testing.T) {
	var all []string
	for _, lib := range Libraries() {
		all = append(all, lib.ID)
		if c := libDepConflicts(genLibDeps([]string{lib.ID})); len(c) != 0 {
			t.Errorf("%s: conflicting lib_deps %q", lib.ID, c)
		}
	}
	if len(all) == 0 {
		t.Fatal("no curated libraries")
	}
	if c := libDepConflicts(genLibDeps(all)); len(c) != 0 {
		t.Errorf("all libraries together: conflicting lib_deps %q", c)
	}
}
