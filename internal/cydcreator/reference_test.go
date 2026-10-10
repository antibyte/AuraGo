package cydcreator

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The hand-written reference sample (testdata/reference/basic) is what the
// builder image compiles for every board profile and what plan 1c's
// generator must reproduce byte for byte. These tests keep it consistent with
// the domain code until the generator exists.
const referenceDir = "testdata/reference/basic"

func TestReferenceProjectIsCanonicalAndValid(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join(referenceDir, "cyd.project.json"))
	if err != nil {
		t.Fatal(err)
	}
	p, err := ParseProject(raw)
	if err != nil {
		t.Fatal(err)
	}
	if issues := Validate(p, ValidateOptions{AssetSizes: map[string]int64{}}); len(issues) != 0 {
		t.Fatalf("reference project has issues: %+v", issues)
	}
	canonical, err := MarshalProject(p)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(canonical, raw) {
		t.Fatal("cyd.project.json is not in MarshalProject's canonical form")
	}
	if p.TemplateVersion != TemplateVersion {
		t.Fatalf("template_version %q, want %q", p.TemplateVersion, TemplateVersion)
	}
}

func TestReferenceProfilesCoverEveryBoard(t *testing.T) {
	boards, err := Boards()
	if err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(filepath.Join(referenceDir, "profiles"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != len(boards) {
		t.Fatalf("%d profile directories for %d boards", len(entries), len(boards))
	}
	update := os.Getenv("CYD_UPDATE_REFERENCE") == "1"
	for _, b := range boards {
		dir := filepath.Join(referenceDir, "profiles", b.ID)
		wantCSV, err := PartitionCSV(b.FlashMB)
		if err != nil {
			t.Fatal(err)
		}
		csvPath := filepath.Join(dir, "partitions.csv")
		if update {
			if err := os.WriteFile(csvPath, []byte(wantCSV), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		gotCSV, err := os.ReadFile(csvPath)
		if err != nil {
			t.Fatal(err)
		}
		if string(gotCSV) != wantCSV {
			t.Errorf("%s: partitions.csv differs from PartitionCSV(%d); run CYD_UPDATE_REFERENCE=1 go test ./internal/cydcreator/ -run TestReferenceProfilesCoverEveryBoard", b.ID, b.FlashMB)
		}
		cfg, err := os.ReadFile(filepath.Join(dir, "include", "board_config.h"))
		if err != nil {
			t.Fatal(err)
		}
		for _, want := range []string{
			fmt.Sprintf("#define CYD_BOARD_ID %q\n", b.ID),
			fmt.Sprintf("#define CYD_FLASH_MB %d\n", b.FlashMB),
			fmt.Sprintf("#define CYD_PANEL_DRIVER_%s 1\n", b.Display.Driver),
			fmt.Sprintf("#define CYD_TOUCH_DRIVER_%s 1\n", b.Touch.Driver),
			fmt.Sprintf("#define CYD_PANEL_OFFSET_ROTATION %d\n", b.Display.OffsetRotation),
			fmt.Sprintf("#define CYD_DRAW_BUF_LINES %d\n", b.LVGL.DrawBufferLines),
			fmt.Sprintf("#define CYD_TEMPLATE_VERSION %q\n", TemplateVersion),
		} {
			if !strings.Contains(string(cfg), want) {
				t.Errorf("%s: board_config.h lacks %q", b.ID, strings.TrimSpace(want))
			}
		}
		ini, err := os.ReadFile(filepath.Join(dir, "platformio.ini"))
		if err != nil {
			t.Fatal(err)
		}
		for _, want := range []string{
			"[env:" + b.ID + "]\n",
			"board = " + b.PIO.Board + "\n",
			fmt.Sprintf("board_upload.flash_size = %dMB\n", b.FlashMB),
			"  bblanchon/ArduinoJson@7.4.3\n",
		} {
			if !strings.Contains(string(ini), want) {
				t.Errorf("%s: platformio.ini lacks %q", b.ID, strings.TrimSpace(want))
			}
		}
	}
}

func TestReferenceFontsJSONShape(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join(referenceDir, "src", "generated", "fonts.json"))
	if err != nil {
		t.Fatal(err)
	}
	var spec struct {
		Fonts []struct {
			Symbol    string   `json:"symbol"`
			Family    string   `json:"family"`
			Size      int      `json:"size"`
			Ranges    []string `json:"ranges"`
			Chars     string   `json:"chars"`
			LVSymbols bool     `json:"lv_symbols"`
		} `json:"fonts"`
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&spec); err != nil {
		t.Fatal(err)
	}
	if len(spec.Fonts) == 0 {
		t.Fatal("no fonts")
	}
	prev := ""
	for _, f := range spec.Fonts {
		if f.Symbol != fmt.Sprintf("ui_font_%s_%d", f.Family, f.Size) || f.Symbol <= prev {
			t.Errorf("font %q: symbol must be ui_font_<family>_<size>, sorted", f.Symbol)
		}
		prev = f.Symbol
		if strings.Join(f.Ranges, ",") != "0x20-0x7E,0xA0-0x17F" || !f.LVSymbols {
			t.Errorf("font %q: ranges/lv_symbols differ from the contract", f.Symbol)
		}
	}
}
