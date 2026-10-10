package ui

import (
	"bytes"
	"compress/zlib"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/retronet"
)

// C64 Pro Mono v1.2.1 by Style, vendored unmodified under its original file name (Contract C).
const (
	c64FontFile          = "C64_Pro_Mono-STYLE.woff"
	c64FontLicenseFile   = "C64_Pro_Mono-LICENSE.txt"
	c64FontBytes         = 8300
	c64FontSHA256        = "e4f84e4046c1edededf5cbf3fc5a236de19974a6be72ad2a6fa87f046d91f50e"
	c64FontArchiveSHA256 = "b630ff545df6fed324bd9b6dc7336b7b93e8777668f45cbb3679561f9960ef2d"
)

func readC64Font(t *testing.T) []byte {
	t.Helper()
	font, err := os.ReadFile(filepath.Join("fonts", c64FontFile))
	if err != nil {
		t.Fatalf("read C64 font: %v", err)
	}
	return font
}

func TestDesktopTerminalC64FontIsVendoredWithLicense(t *testing.T) {
	t.Parallel()

	font := readC64Font(t)
	if len(font) != c64FontBytes || string(font[:4]) != "wOFF" {
		t.Fatalf("%s must be the original %d-byte WOFF, got %d bytes starting %q", c64FontFile, c64FontBytes, len(font), font[:4])
	}
	sum := sha256.Sum256(font)
	if got := hex.EncodeToString(sum[:]); got != c64FontSHA256 {
		t.Fatalf("%s was modified: SHA-256 %s, want %s", c64FontFile, got, c64FontSHA256)
	}
	raw, err := os.ReadFile(filepath.Join("fonts", c64FontLicenseFile))
	if err != nil {
		t.Fatalf("read C64 font license: %v", err)
	}
	license := strings.ReplaceAll(string(raw), "\r\n", "\n")
	for _, want := range []string{
		"C64 Pro Mono",
		"Fonts in this package are (c) 2010-2019 Style.",
		"http://style64.org/c64-truetype",
		"Source archive: C64_TrueType_v1.2.1-STYLE.zip",
		"Source archive SHA-256: " + c64FontArchiveSHA256,
		"Source archive entry: C64_TrueType_v1.2.1-STYLE/fonts/C64_Pro_Mono-STYLE.woff",
		"SHA-256: " + c64FontSHA256,
		"unmodified, under its original file name",
		"free of charge",
		// license.txt verbatim: the clauses AuraGo relies on.
		"embed the font (without any modification or file renaming) for display on any web site using @font-face rules",
		"ONLY if said software package is freely provided to end users",
		"http://style64.org/c64-truetype/license",
	} {
		if !strings.Contains(license, want) {
			t.Fatalf("%s missing %q", c64FontLicenseFile, want)
		}
	}
	css := strings.ReplaceAll(readDesktopAssetText(t, "css/desktop-app-terminal.css"), "\r\n", "\n")
	start := strings.Index(css, `font-family: "Aura C64";`)
	if start < 0 {
		t.Fatal(`desktop-app-terminal.css has no @font-face for "Aura C64"`)
	}
	face := css[strings.LastIndex(css[:start], "@font-face {"):]
	face = face[:strings.Index(face, "}")]
	for _, want := range []string{`src: url('/fonts/C64_Pro_Mono-STYLE.woff') format('woff');`, "font-display: block;"} {
		if !strings.Contains(face, want) {
			t.Fatalf(`the "Aura C64" @font-face misses %q: %s`, want, face)
		}
	}
	thirdParty, err := os.ReadFile(filepath.Join("..", "THIRD_PARTY_LIBRARIES.md"))
	if err != nil {
		t.Fatalf("read THIRD_PARTY_LIBRARIES.md: %v", err)
	}
	if got := strings.Count(string(thirdParty), "`ui/fonts/C64_Pro_Mono-STYLE.woff`"); got != 2 || !strings.Contains(string(thirdParty), "`ui/fonts/C64_Pro_Mono-LICENSE.txt`") {
		t.Fatalf("THIRD_PARTY_LIBRARIES.md must list the C64 font in the fonts table and the license-attention table (found %d rows) and name its license file", got)
	}
}

// The PETSCII engine draws C64 glyphs as retronet.C64Glyph code points (U+EE00/U+EF00 + screen code); every one
// of them, for both character sets, must exist in the vendored font, or the browser falls back to another font.
func TestDesktopTerminalC64FontCoversEngineGlyphs(t *testing.T) {
	t.Parallel()

	glyphs := cmapGlyphs(t, woffTable(t, readC64Font(t), "cmap"))
	if len(glyphs) < 512 {
		t.Fatalf("C64 Pro Mono cmap maps only %d code points", len(glyphs))
	}
	for _, lower := range []bool{false, true} {
		base := rune(0xEE00)
		if lower {
			base = 0xEF00
		}
		for sc := 0; sc < 0x80; sc++ {
			code := retronet.C64Glyph(lower, byte(sc))
			if code != base+rune(sc) {
				t.Fatalf("retronet.C64Glyph(%v, %#02x) = U+%04X, want U+%04X (Contract A)", lower, sc, code, base+rune(sc))
			}
			if code >= 0xE0A0 && code <= 0xE0BF {
				t.Fatalf("U+%04X lies in the Powerline range the xterm WebGL renderer replaces", code)
			}
			if glyphs[code] == 0 {
				t.Errorf("C64 Pro Mono has no glyph for U+%04X (screen code %#02x, lower/upper set %v)", code, sc, lower)
			}
		}
	}
	for sc := byte(0x01); sc <= 0x1A; sc++ {
		if glyphs[retronet.C64Glyph(false, sc)] == glyphs[retronet.C64Glyph(true, sc)] {
			t.Errorf("screen code %#02x has the same glyph in both sets: U+EE00/U+EF00 are not the two character ROMs", sc)
		}
	}
}

// woffTable returns one table of a WOFF 1.0 font, inflated when it is stored zlib-compressed.
func woffTable(t *testing.T, font []byte, tag string) []byte {
	t.Helper()
	if len(font) < 44 || string(font[:4]) != "wOFF" {
		t.Fatalf("not a WOFF 1.0 font (%d bytes)", len(font))
	}
	count := int(binary.BigEndian.Uint16(font[12:]))
	for i := 0; i < count; i++ {
		entry := 44 + i*20
		if entry+20 > len(font) {
			t.Fatalf("WOFF table directory truncated at entry %d", i)
		}
		if string(font[entry:entry+4]) != tag {
			continue
		}
		offset := int(binary.BigEndian.Uint32(font[entry+4:]))
		stored := int(binary.BigEndian.Uint32(font[entry+8:]))
		original := int(binary.BigEndian.Uint32(font[entry+12:]))
		if offset < 44 || stored <= 0 || offset+stored > len(font) || original < stored {
			t.Fatalf("WOFF %s table out of bounds (offset %d, %d of %d bytes)", tag, offset, stored, len(font))
		}
		data := font[offset : offset+stored]
		if stored == original {
			return data
		}
		reader, err := zlib.NewReader(bytes.NewReader(data))
		if err != nil {
			t.Fatalf("inflate WOFF %s table: %v", tag, err)
		}
		out, err := io.ReadAll(io.LimitReader(reader, int64(original)+1))
		if err != nil || len(out) != original {
			t.Fatalf("inflate WOFF %s table: %d of %d bytes (%v)", tag, len(out), original, err)
		}
		return out
	}
	t.Fatalf("WOFF has no %s table", tag)
	return nil
}

// cmapGlyphs maps every code point of a cmap's format 4 and 12 subtables to its glyph ID (0 is .notdef, left out).
func cmapGlyphs(t *testing.T, cmap []byte) map[rune]int {
	t.Helper()
	u16 := func(at int) int {
		if at < 0 || at+2 > len(cmap) {
			t.Fatalf("cmap truncated at byte %d", at)
		}
		return int(binary.BigEndian.Uint16(cmap[at:]))
	}
	u32 := func(at int) int {
		if at < 0 || at+4 > len(cmap) {
			t.Fatalf("cmap truncated at byte %d", at)
		}
		return int(binary.BigEndian.Uint32(cmap[at:]))
	}
	glyphs := map[rune]int{}
	for i, subtables := 0, u16(2); i < subtables; i++ {
		offset := u32(4 + i*8 + 4)
		switch u16(offset) {
		case 4:
			segments := u16(offset+6) / 2
			ends := offset + 14
			starts := ends + segments*2 + 2
			deltas := starts + segments*2
			ranges := deltas + segments*2
			for s := 0; s < segments; s++ {
				first, last := u16(starts+2*s), u16(ends+2*s)
				delta, rangeOffset := u16(deltas+2*s), u16(ranges+2*s)
				for code := first; code <= last && code != 0xFFFF; code++ {
					glyph := (code + delta) & 0xFFFF
					if rangeOffset != 0 {
						if glyph = u16(ranges + 2*s + rangeOffset + 2*(code-first)); glyph != 0 {
							glyph = (glyph + delta) & 0xFFFF
						}
					}
					if glyph != 0 {
						glyphs[rune(code)] = glyph
					}
				}
			}
		case 12:
			for g, groups := 0, u32(offset+12); g < groups; g++ {
				at := offset + 16 + g*12
				first, last, glyph := u32(at), u32(at+4), u32(at+8)
				if last < first || last > 0x10FFFF {
					t.Fatalf("cmap format 12 group %d has the range %X-%X", g, first, last)
				}
				for code := first; code <= last; code++ {
					if id := glyph + code - first; id != 0 {
						glyphs[rune(code)] = id
					}
				}
			}
		}
	}
	return glyphs
}
