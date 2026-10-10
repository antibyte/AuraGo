package retronet

import (
	"fmt"
	"testing"
)

func TestPetsciiScreenCodeTable(t *testing.T) {
	// Oracle: the CHROUT conversion ranges, written as data.
	ranges := []struct{ lo, hi, first int }{
		{0x20, 0x3F, 0x20},
		{0x40, 0x5F, 0x00},
		{0x60, 0x7F, 0x40},
		{0xA0, 0xBF, 0x60},
		{0xC0, 0xFE, 0x40},
		{0xFF, 0xFF, 0x5E},
	}
	for b := 0; b < 256; b++ {
		want, printable := -1, false
		for _, r := range ranges {
			if b >= r.lo && b <= r.hi {
				want, printable = r.first+b-r.lo, true
			}
		}
		sc, ok := petsciiScreenCode(byte(b))
		if ok != printable || (ok && int(sc) != want) {
			t.Errorf("petsciiScreenCode(0x%02X) = 0x%02X, %v; want 0x%02X, %v", b, sc, ok, want, printable)
		}
	}
	spot := map[byte]byte{0x41: 0x01, 0x61: 0x41, 0xC1: 0x41, 0xA0: 0x60, 0xE0: 0x60, 0xDE: 0x5E, 0xFF: 0x5E, 0x5C: 0x1C}
	for b, want := range spot {
		if sc, _ := petsciiScreenCode(b); sc != want {
			t.Errorf("petsciiScreenCode(0x%02X) = 0x%02X, want 0x%02X", b, sc, want)
		}
	}
}

func TestC64Glyph(t *testing.T) {
	tests := []struct {
		lower bool
		sc    byte
		want  rune
	}{
		{true, 0x00, 0xEF00},
		{true, 0x01, 0xEF01},
		{true, 0x41, 0xEF41},
		{true, 0x7F, 0xEF7F},
		{false, 0x00, 0xEE00},
		{false, 0x41, 0xEE41},
		{false, 0x7F, 0xEE7F},
		{true, 0x81, 0xEF01},  // the reverse bit is ignored
		{false, 0xA0, 0xEE20}, // reversed space -> space
	}
	for _, tc := range tests {
		if got := C64Glyph(tc.lower, tc.sc); got != tc.want {
			t.Errorf("C64Glyph(%v, 0x%02X) = %U, want %U", tc.lower, tc.sc, got, tc.want)
		}
	}
}

func TestC64PaletteAndColourCodes(t *testing.T) {
	want := []struct {
		code byte
		hex  string
	}{
		{0x90, "#000000"}, {0x05, "#FFFFFF"}, {0x1C, "#813338"}, {0x9F, "#75CEC8"},
		{0x9C, "#8E3C97"}, {0x1E, "#56AC4D"}, {0x1F, "#2E2C9B"}, {0x9E, "#EDF171"},
		{0x81, "#8E5029"}, {0x95, "#553800"}, {0x96, "#C46C71"}, {0x97, "#4A4A4A"},
		{0x98, "#7B7B7B"}, {0x99, "#A9FF9F"}, {0x9A, "#706DEB"}, {0x9B, "#B2B2B2"},
	}
	for i, w := range want {
		if got := fmt.Sprintf("#%06X", c64Palette[i]); got != w.hex {
			t.Errorf("c64Palette[%d] = %s, want %s", i, got, w.hex)
		}
		if petsciiColourCodes[i] != w.code {
			t.Errorf("petsciiColourCodes[%d] = 0x%02X, want 0x%02X", i, petsciiColourCodes[i], w.code)
		}
	}
	colour := map[byte]int{}
	for i, w := range want {
		colour[w.code] = i
	}
	for b := 0; b < 256; b++ {
		i, isColour := colour[byte(b)]
		if !isColour {
			i = -1
		}
		if got := petsciiColourIndex[b]; int(got) != i {
			t.Errorf("petsciiColourIndex[0x%02X] = %d, want %d", b, got, i)
		}
	}
	if c64InitialColour != 14 {
		t.Errorf("c64InitialColour = %d, want 14 (light blue)", c64InitialColour)
	}
}
