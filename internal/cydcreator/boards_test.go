package cydcreator

import (
	"encoding/json"
	"io/fs"
	"reflect"
	"sort"
	"strings"
	"testing"
	"testing/fstest"
)

// TestEveryEmbeddedBoardLoads covers every boards/*.json, including the
// community profiles that later plans add.
func TestEveryEmbeddedBoardLoads(t *testing.T) {
	boards, err := Boards()
	if err != nil {
		t.Fatalf("Boards(): %v", err)
	}
	files, err := fs.Glob(boardFS, "boards/*.json")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 || len(boards) != len(files) {
		t.Fatalf("%d boards loaded from %d files", len(boards), len(files))
	}
	ids := make([]string, len(boards))
	for i, b := range boards {
		ids[i] = b.ID
		if b.PIOEnv() != b.ID {
			t.Errorf("%s: PIOEnv() = %q", b.ID, b.PIOEnv())
		}
		if _, err := PartitionTable(b.FlashMB); err != nil {
			t.Errorf("%s: no partition table for %d MB", b.ID, b.FlashMB)
		}
		cfg, err := ConfigPartitionOffset(b.FlashMB)
		if err != nil || cfg+ConfigSize > b.FlashMB<<20 {
			t.Errorf("%s: config partition 0x%X does not fit %d MB", b.ID, cfg, b.FlashMB)
		}
		if err := validateBoard(b); err != nil {
			t.Errorf("%s: %v", b.ID, err)
		}
	}
	if !sort.StringsAreSorted(ids) {
		t.Errorf("boards not sorted by id: %v", ids)
	}
}

func TestVerifiedCYDProfile(t *testing.T) {
	b, err := BoardByID("esp32-2432s028r")
	if err != nil {
		t.Fatal(err)
	}
	ldr, spk, sd := 34, 26, 5
	want := Board{
		ID:      "esp32-2432s028r",
		Label:   `CYD 2.8" (ESP32-2432S028R, micro-USB)`,
		Status:  BoardVerified,
		Source:  "https://github.com/witnessmenow/ESP32-Cheap-Yellow-Display",
		Chip:    ChipESP32,
		FlashMB: 4,
		PSRAM:   false,
		PIO:     BoardPIO{Board: "esp32dev"},
		Display: BoardDisplay{
			Driver: "ILI9341", Bus: "spi", Width: 320, Height: 240, NativeOrientation: "landscape",
			Invert: true, BGR: false,
			Pins: map[string]int{"sclk": 14, "mosi": 13, "miso": 12, "cs": 15, "dc": 2, "rst": -1, "bl": 21},
			Hz:   40000000, BacklightPWM: true, OffsetRotation: 5,
		},
		Touch: BoardTouch{
			Driver: "XPT2046", Bus: "spi",
			Pins:             map[string]int{"sclk": 25, "mosi": 32, "miso": 39, "cs": 33, "irq": 36},
			Calibration:      &TouchCalibration{XMin: 290, XMax: 3670, YMin: 230, YMax: 3860},
			NeedsCalibration: true,
		},
		Extras: BoardExtras{
			RGBLED: &RGBLED{R: 4, G: 16, B: 17, ActiveLow: true},
			LDR:    &ldr, Speaker: &spk, SDCS: &sd, BootButton: 0,
		},
		LVGL: BoardLVGL{DrawBufferLines: 24, DoubleBuffer: false},
	}
	if !reflect.DeepEqual(b, want) {
		got, _ := json.MarshalIndent(b, "", " ")
		t.Fatalf("profile mismatch:\n%s", got)
	}
}

func TestBoardByIDUnknown(t *testing.T) {
	_, err := BoardByID("esp32-nope")
	if ErrorCode(err) != CodeBadRequest {
		t.Fatalf("err = %v, want CYD_BAD_REQUEST", err)
	}
}

func TestBoardsReturnDeepCopies(t *testing.T) {
	a, _ := BoardByID("esp32-2432s028r")
	a.Display.Pins["cs"] = 99
	a.Touch.Calibration.XMin = 1
	*a.Extras.LDR = 1
	b, _ := BoardByID("esp32-2432s028r")
	if b.Display.Pins["cs"] != 15 || b.Touch.Calibration.XMin != 290 || *b.Extras.LDR != 34 {
		t.Fatal("Boards() leaked its cache to callers")
	}
}

func TestScreenSize(t *testing.T) {
	landscape := Board{Display: BoardDisplay{Width: 320, Height: 240}}
	portrait := Board{Display: BoardDisplay{Width: 480, Height: 800}}
	cases := []struct {
		b           Board
		orientation string
		w, h        int
	}{
		{landscape, "landscape", 320, 240},
		{landscape, "landscape_flipped", 320, 240},
		{landscape, "portrait", 240, 320},
		{landscape, "portrait_flipped", 240, 320},
		{landscape, "sideways", 320, 240},
		{portrait, "landscape", 800, 480},
		{portrait, "portrait", 480, 800},
	}
	for _, tc := range cases {
		w, h := tc.b.ScreenSize(tc.orientation)
		if w != tc.w || h != tc.h {
			t.Errorf("ScreenSize(%q) on %dx%d = %dx%d, want %dx%d", tc.orientation,
				tc.b.Display.Width, tc.b.Display.Height, w, h, tc.w, tc.h)
		}
	}
}

// mutatedProfile decodes the verified profile into a generic map, applies fn and
// returns the re-encoded JSON.
func mutatedProfile(t *testing.T, fn func(m map[string]any)) []byte {
	t.Helper()
	raw, err := fs.ReadFile(boardFS, "boards/esp32-2432s028r.json")
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	fn(m)
	out, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func profileSection(m map[string]any, keys ...string) map[string]any {
	for _, k := range keys {
		m = m[k].(map[string]any)
	}
	return m
}

// capacitiveTouch turns the verified profile's resistive touch into a GT911 on
// pins that no other role uses, so offset_rotation rules can be checked on a
// capacitive controller.
func capacitiveTouch(offsetRotation int) func(m map[string]any) {
	return func(m map[string]any) {
		touch := profileSection(m, "touch")
		touch["driver"], touch["bus"] = "GT911", "i2c"
		touch["pins"] = map[string]any{"sda": 33, "scl": 32}
		touch["i2c_addr"] = 93
		touch["needs_calibration"] = false
		touch["offset_rotation"] = offsetRotation
		delete(touch, "calibration")
	}
}

func TestValidateBoardRejects(t *testing.T) {
	cases := []struct {
		name string
		fn   func(m map[string]any)
		want string
	}{
		{"unknown top-level key", func(m map[string]any) { m["colour"] = "yellow" }, `unknown field "colour"`},
		{"unknown nested key", func(m map[string]any) { profileSection(m, "display")["spi_hz"] = 1 }, `unknown field "spi_hz"`},
		{"bad id", func(m map[string]any) { m["id"] = "ESP32_CYD" }, "id \"ESP32_CYD\""},
		{"empty label", func(m map[string]any) { m["label"] = " " }, "label must be"},
		{"bad status", func(m map[string]any) { m["status"] = "tested" }, "status \"tested\""},
		{"http source", func(m map[string]any) { m["source"] = "http://x" }, "source must be"},
		{"bad chip", func(m map[string]any) { m["chip"] = "esp32c3" }, "chip \"esp32c3\""},
		{"bad flash", func(m map[string]any) { m["flash_mb"] = 2 }, "flash_mb 2"},
		{"empty pio board", func(m map[string]any) { profileSection(m, "pio")["board"] = "" }, "pio.board"},
		{"quoted build flag", func(m map[string]any) { profileSection(m, "pio")["build_flags"] = []any{`-DX="y"`} }, "build_flags entry"},
		{"partitions board_build", func(m map[string]any) {
			profileSection(m, "pio")["board_build"] = map[string]any{"partitions": "x.csv"}
		}, "board_build.partitions"},
		{"board_build option outside the list", func(m map[string]any) {
			profileSection(m, "pio")["board_build"] = map[string]any{"embed_files": "x.bin"}
		}, `board_build key "embed_files" is not allowed`},
		{"board_build bad value", func(m map[string]any) {
			profileSection(m, "pio")["board_build"] = map[string]any{"flash_mode": "q io"}
		}, "pio.board_build"},
		{"driver bus mismatch", func(m map[string]any) { profileSection(m, "display")["driver"] = "NV3041A" }, "needs bus qspi"},
		{"ILI9341_2 on another bus", func(m map[string]any) {
			profileSection(m, "display")["driver"] = "ILI9341_2"
			profileSection(m, "display")["bus"] = "qspi"
		}, "display.driver ILI9341_2 needs bus spi"},
		{"driver name in lower case", func(m map[string]any) { profileSection(m, "display")["driver"] = "ili9341_2" }, `display.driver "ili9341_2"`},
		{"unknown driver", func(m map[string]any) { profileSection(m, "display")["driver"] = "SSD1306" }, "display.driver \"SSD1306\""},
		{"portrait mismatch", func(m map[string]any) { profileSection(m, "display")["native_orientation"] = "portrait" }, "portrait display"},
		{"hz too high", func(m map[string]any) { profileSection(m, "display")["hz"] = 100000000 }, "display.hz"},
		{"timing on spi", func(m map[string]any) { profileSection(m, "display")["timing"] = map[string]any{"pclk_hz": 1} }, "only allowed for rgb"},
		{"missing display pin", func(m map[string]any) { delete(profileSection(m, "display", "pins"), "dc") }, "display.pins.dc is required"},
		{"unknown display pin", func(m map[string]any) { profileSection(m, "display", "pins")["d0"] = 22 }, "display.pins.d0 is not a pin"},
		{"flash pin", func(m map[string]any) { profileSection(m, "display", "pins")["cs"] = 6 }, "GPIO 6 is wired to the SPI flash"},
		{"missing gpio", func(m map[string]any) { profileSection(m, "display", "pins")["cs"] = 20 }, "GPIO 20 does not exist"},
		{"input-only output", func(m map[string]any) { profileSection(m, "display", "pins")["cs"] = 35 }, "GPIO 35 is input-only"},
		{"duplicate pin", func(m map[string]any) { profileSection(m, "touch", "pins")["cs"] = 2 }, "GPIO 2 is used twice (display.dc, touch.cs)"},
		{"duplicate extra", func(m map[string]any) { profileSection(m, "extras")["speaker"] = 4 }, "GPIO 4 is used twice"},
		{"ldr on an ADC2 pin", func(m map[string]any) { profileSection(m, "extras")["ldr"] = 27 }, "GPIO 27 is not an ADC1 pin"},
		{"touch shares sclk only", func(m map[string]any) { profileSection(m, "touch", "pins")["sclk"] = 14 },
			"touch must use the display's sclk, mosi and miso exactly or none of them"},
		{"touch shares mosi only", func(m map[string]any) { profileSection(m, "touch", "pins")["mosi"] = 13 },
			"touch must use the display's sclk, mosi and miso exactly or none of them"},
		{"touch shares sclk and mosi, display has no miso", func(m map[string]any) {
			p := profileSection(m, "touch", "pins")
			p["sclk"], p["mosi"] = 14, 13
			delete(profileSection(m, "display", "pins"), "miso")
		}, "touch must use the display's sclk, mosi and miso exactly or none of them"},
		{"touch bus mismatch", func(m map[string]any) { profileSection(m, "touch")["driver"] = "GT911" }, "needs bus i2c"},
		{"missing calibration", func(m map[string]any) { delete(profileSection(m, "touch"), "calibration") }, "default calibration"},
		{"inverted calibration", func(m map[string]any) { profileSection(m, "touch", "calibration")["x_min"] = 4000 }, "touch.calibration must"},
		{"i2c addr on spi touch", func(m map[string]any) { profileSection(m, "touch")["i2c_addr"] = 93 }, "i2c_addr is only allowed"},
		{"display offset rotation", func(m map[string]any) { profileSection(m, "display")["offset_rotation"] = 8 }, "display.offset_rotation 8"},
		{"display offset rotation negative", func(m map[string]any) { profileSection(m, "display")["offset_rotation"] = -1 }, "display.offset_rotation -1"},
		{"xpt2046 offset rotation", func(m map[string]any) { profileSection(m, "touch")["offset_rotation"] = 2 }, "must be 0 for XPT2046"},
		{"capacitive touch offset rotation", capacitiveTouch(8), "touch.offset_rotation 8"},
		{"draw buffer zero", func(m map[string]any) { profileSection(m, "lvgl")["draw_buffer_lines"] = 0 }, "draw_buffer_lines 0"},
		{"draw buffer too big", func(m map[string]any) { profileSection(m, "lvgl")["draw_buffer_lines"] = 80 }, "boards without PSRAM allow"},
		{"double buffer too big", func(m map[string]any) {
			profileSection(m, "lvgl")["draw_buffer_lines"] = 40
			profileSection(m, "lvgl")["double_buffer"] = true
		}, "boards without PSRAM allow"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			data := mutatedProfile(t, tc.fn)
			b, err := parseBoard(data)
			if err == nil {
				err = validateBoard(b)
			}
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want it to contain %q", err, tc.want)
			}
		})
	}
}

func TestValidateBoardAccepts(t *testing.T) {
	cases := []struct {
		name string
		fn   func(m map[string]any)
	}{
		{"unchanged", func(m map[string]any) {}},
		// Both ILI9341 init sequences drive the same 240x320 controller on SPI.
		{"ILI9341 with the standard init", func(m map[string]any) { profileSection(m, "display")["driver"] = "ILI9341" }},
		{"ILI9341 with the alternative init", func(m map[string]any) { profileSection(m, "display")["driver"] = "ILI9341_2" }},
		{"touch shares the display SPI bus", func(m map[string]any) {
			p := profileSection(m, "touch", "pins")
			p["sclk"], p["mosi"], p["miso"] = 14, 13, 12
		}},
		{"optional pins absent", func(m map[string]any) {
			delete(profileSection(m, "display", "pins"), "rst")
			delete(profileSection(m, "display", "pins"), "miso")
			delete(profileSection(m, "touch", "pins"), "irq")
			delete(m, "extras")
			m["extras"] = map[string]any{"boot_button": 0}
		}},
		{"build flags", func(m map[string]any) {
			profileSection(m, "pio")["build_flags"] = []any{"-DBOARD_HAS_PSRAM", "-DCYD_X=1", "-mfix-esp32-psram-cache-issue"}
		}},
		{"board_build options", func(m map[string]any) {
			profileSection(m, "pio")["board_build"] = map[string]any{
				"arduino.memory_type": "qio_qspi", "flash_mode": "qio", "f_flash": "80000000L", "f_cpu": "240000000L"}
		}},
		{"ldr on another ADC1 pin", func(m map[string]any) { profileSection(m, "extras")["ldr"] = 35 }},
		{"capacitive touch with offset rotation", capacitiveTouch(2)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b, err := parseBoard(mutatedProfile(t, tc.fn))
			if err != nil {
				t.Fatal(err)
			}
			if err := validateBoard(b); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestValidateBoardS3Profiles(t *testing.T) {
	rgb := Board{
		ID: "esp32-8048s043", Label: "4.3 RGB", Status: BoardCommunity, Source: "https://example.com/x",
		Chip: ChipESP32S3, FlashMB: 16, PSRAM: true,
		PIO: BoardPIO{Board: "esp32-s3-devkitc-1", BoardBuild: map[string]string{"arduino.memory_type": "qio_opi"}},
		Display: BoardDisplay{Driver: "ST7262", Bus: "rgb", Width: 800, Height: 480, NativeOrientation: "landscape",
			Pins: map[string]int{"de": 40, "vsync": 41, "hsync": 39, "pclk": 42,
				"r0": 45, "r1": 48, "r2": 47, "r3": 21, "r4": 14,
				"g0": 5, "g1": 6, "g2": 7, "g3": 15, "g4": 16, "g5": 4,
				"b0": 8, "b1": 3, "b2": 46, "b3": 9, "b4": 1, "bl": 2},
			Timing: map[string]int{"pclk_hz": 14000000, "hsync_polarity": 0, "hsync_front_porch": 8,
				"hsync_pulse_width": 4, "hsync_back_porch": 43, "vsync_polarity": 0, "vsync_front_porch": 8,
				"vsync_pulse_width": 4, "vsync_back_porch": 12, "pclk_active_neg": 1},
		},
		Touch:  BoardTouch{Driver: "GT911", Bus: "i2c", Pins: map[string]int{"sda": 19, "scl": 20, "int": -1, "rst": 38}, I2CAddr: 0x5D},
		Extras: BoardExtras{BootButton: 0},
		LVGL:   BoardLVGL{DrawBufferLines: 480, DoubleBuffer: true},
	}
	if err := validateBoard(rgb); err != nil {
		t.Fatalf("rgb profile rejected: %v", err)
	}

	noPSRAM := cloneBoard(rgb)
	noPSRAM.PSRAM = false
	if err := validateBoard(noPSRAM); err == nil || !strings.Contains(err.Error(), "rgb panels need an esp32s3 with PSRAM") {
		t.Fatalf("rgb without PSRAM: %v", err)
	}
	octal := cloneBoard(rgb)
	octal.Touch.Pins["rst"] = 35
	if err := validateBoard(octal); err == nil || !strings.Contains(err.Error(), "used by octal PSRAM") {
		t.Fatalf("octal PSRAM pin: %v", err)
	}
	missingTiming := cloneBoard(rgb)
	delete(missingTiming.Display.Timing, "vsync_back_porch")
	missingTiming.Display.Timing["vsync_extra"] = 1
	err := validateBoard(missingTiming)
	if err == nil || !strings.Contains(err.Error(), "timing.vsync_back_porch is required") ||
		!strings.Contains(err.Error(), "timing.vsync_extra is unknown") {
		t.Fatalf("timing checks: %v", err)
	}
	widePorch := cloneBoard(rgb)
	widePorch.Display.Timing["hsync_back_porch"] = 210
	if err := validateBoard(widePorch); err == nil || !strings.Contains(err.Error(), "timing.hsync_back_porch=210 is out of range") {
		t.Fatalf("int8 porch bound: %v", err)
	}
	noMemType := cloneBoard(rgb)
	delete(noMemType.PIO.BoardBuild, "arduino.memory_type")
	if err := validateBoard(noMemType); err == nil || !strings.Contains(err.Error(), `must set pio.board_build["arduino.memory_type"]`) {
		t.Fatalf("PSRAM without memory_type: %v", err)
	}
	s3LDR := cloneBoard(rgb)
	ldr := 12
	s3LDR.Extras.LDR = &ldr
	if err := validateBoard(s3LDR); err == nil || !strings.Contains(err.Error(), "GPIO 12 is not an ADC1 pin") {
		t.Fatalf("s3 ADC2 LDR: %v", err)
	}
	ldr = 10
	if err := validateBoard(s3LDR); err != nil {
		t.Fatalf("s3 ADC1 LDR rejected: %v", err)
	}
	cst := cloneBoard(rgb)
	cst.Touch.Driver, cst.Touch.I2CAddr = "CST820", 0x5D
	if err := validateBoard(cst); err == nil || !strings.Contains(err.Error(), "not valid for CST820") {
		t.Fatalf("cst820 address: %v", err)
	}
	capCal := cloneBoard(rgb)
	capCal.Touch.NeedsCalibration = true
	if err := validateBoard(capCal); err == nil || !strings.Contains(err.Error(), "must not carry a calibration") {
		t.Fatalf("capacitive calibration: %v", err)
	}
	qspiOnESP32 := cloneBoard(rgb)
	qspiOnESP32.Chip = ChipESP32
	qspiOnESP32.Display = BoardDisplay{Driver: "NV3041A", Bus: "qspi", Width: 480, Height: 272,
		NativeOrientation: "landscape", Hz: 32000000,
		Pins: map[string]int{"cs": 15, "sclk": 14, "d0": 13, "d1": 12, "d2": 27, "d3": 26}}
	if err := validateBoard(qspiOnESP32); err == nil || !strings.Contains(err.Error(), "qspi panels need an esp32s3") {
		t.Fatalf("qspi on esp32: %v", err)
	}
}

func TestLoadBoardsFileRules(t *testing.T) {
	good, err := fs.ReadFile(boardFS, "boards/esp32-2432s028r.json")
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name  string
		files fstest.MapFS
		want  string
	}{
		{"empty", fstest.MapFS{"boards/readme.txt": {Data: []byte("x")}}, "no board profiles"},
		{"name mismatch", fstest.MapFS{"boards/other.json": {Data: good}}, "does not match the file name"},
		{"trailing data", fstest.MapFS{"boards/esp32-2432s028r.json": {Data: append(append([]byte{}, good...), []byte("{}")...)}}, "trailing data"},
	}
	for _, tc := range cases {
		if _, err := loadBoards(tc.files, "boards"); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: err = %v, want %q", tc.name, err, tc.want)
		}
	}
	ok := fstest.MapFS{"boards/esp32-2432s028r.json": {Data: good}, "boards/notes.md": {Data: []byte("ignored")}}
	if boards, err := loadBoards(ok, "boards"); err != nil || len(boards) != 1 {
		t.Fatalf("loadBoards = %d boards, %v", len(boards), err)
	}
}
