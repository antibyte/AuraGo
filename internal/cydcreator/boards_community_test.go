package cydcreator

import (
	"slices"
	"testing"
)

// TestBoardSetMatchesSpec pins the launch profile set of spec §2: the classic
// CYD is the only verified board, the other eight are community profiles.
func TestBoardSetMatchesSpec(t *testing.T) {
	want := map[string]struct {
		chip, driver, touch string
		flashMB             int
	}{
		"esp32-2432s024c":     {ChipESP32, "ILI9341", "CST820", 4},
		"esp32-2432s024r":     {ChipESP32, "ILI9341", "XPT2046", 4},
		"esp32-2432s028-usbc": {ChipESP32, "ST7789", "XPT2046", 4},
		"esp32-2432s028r":     {ChipESP32, "ILI9341_2", "XPT2046", 4},
		"esp32-3248s035c":     {ChipESP32, "ST7796", "GT911", 4},
		"esp32-3248s035r":     {ChipESP32, "ST7796", "XPT2046", 4},
		"esp32-8048s043":      {ChipESP32S3, "ST7262", "GT911", 16},
		"esp32-8048s070":      {ChipESP32S3, "EK9716", "GT911", 16},
		"jc4827w543":          {ChipESP32S3, "NV3041A", "GT911", 4},
	}
	boards, err := Boards()
	if err != nil {
		t.Fatal(err)
	}
	var got, wantIDs []string
	for id := range want {
		wantIDs = append(wantIDs, id)
	}
	slices.Sort(wantIDs)
	for _, b := range boards {
		got = append(got, b.ID)
	}
	if !slices.Equal(got, wantIDs) {
		t.Fatalf("got board ids %v, want %v", got, wantIDs)
	}
	for _, b := range boards {
		w := want[b.ID]
		if b.Chip != w.chip || b.Display.Driver != w.driver || b.Touch.Driver != w.touch || b.FlashMB != w.flashMB {
			t.Errorf("%s: chip/display/touch/flash = %s/%s/%s/%d, want %s/%s/%s/%d", b.ID, b.Chip, b.Display.Driver,
				b.Touch.Driver, b.FlashMB, w.chip, w.driver, w.touch, w.flashMB)
		}
		wantStatus := BoardCommunity
		if b.ID == "esp32-2432s028r" {
			wantStatus = BoardVerified
		}
		if b.Status != wantStatus {
			t.Errorf("%s: status %q, want %q", b.ID, b.Status, wantStatus)
		}
		if b.Chip == ChipESP32S3 && !b.PSRAM {
			t.Errorf("%s: ESP32-S3 profiles in v1 all have PSRAM", b.ID)
		}
	}
}
