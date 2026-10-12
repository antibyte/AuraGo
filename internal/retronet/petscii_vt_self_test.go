package retronet

import "testing"

// The reference terminal is the oracle of the engine tests, so it is tested
// on its own first.

func TestReferenceTerminalRendersTheEngineAlphabet(t *testing.T) {
	v := newVTScreen()
	a, g := string(C64Glyph(true, 0x01)), string(C64Glyph(false, 0x41))
	steps := "\x1b[0m" +
		"\x1b[?2026h" + // a synchronized block changes nothing by itself
		"\x1b[2;39H\x1b[38;2;255;255;255;48;2;129;51;56m" + a + g + // white on red in columns 39-40 of row 2
		"\x1b[?2026l" +
		"\x07" +
		"\x1b[1;1H\x1b[38;2;112;109;235m\x1b[48;2;0;0;0m" + a // light blue on black, home
	if err := v.write([]byte(steps)); err != nil {
		t.Fatalf("write: %v", err)
	}
	red, white, blue := int32(0x813338), int32(0xFFFFFF), int32(0x706DEB)
	if got := v.cells[1][38]; got != (vtCell{ch: C64Glyph(true, 0x01), fg: white, bg: red}) {
		t.Fatalf("cell 1,38 = %+v", got)
	}
	if got := v.cells[0][0]; got != (vtCell{ch: C64Glyph(true, 0x01), fg: blue, bg: 0}) {
		t.Fatalf("cell 0,0 = %+v", got)
	}
	if v.row != 0 || v.col != 1 || v.pending || v.bells != 1 {
		t.Fatalf("cursor %d,%d pending %v bells %d", v.row, v.col, v.pending, v.bells)
	}
	if err := v.write([]byte("\x1b[48;2;0;0;0m\x1b[S")); err != nil { // scroll up on black
		t.Fatalf("SU: %v", err)
	}
	if got := v.cells[0][38]; got.ch != C64Glyph(true, 0x01) || v.cells[24][0].ch != ' ' || v.cells[24][0].bg != 0 {
		t.Fatalf("after SU: cell 0,38 = %+v, bottom row %+v", got, v.cells[24][0])
	}
	if err := v.write([]byte("\x1b[1;40H\x1b[48;2;46;44;155m\x1b[K")); err != nil { // erase the last cell on blue
		t.Fatalf("EL: %v", err)
	}
	if got := v.cells[0][39]; got != (vtCell{ch: ' ', fg: vtDefaultFG, bg: 0x2E2C9B}) || v.cells[0][38].ch != C64Glyph(true, 0x01) {
		t.Fatalf("after EL: cells %+v %+v", v.cells[0][38], got)
	}
	if err := v.write([]byte("\x1b[48;2;0;0;0m\x1b[2J")); err != nil {
		t.Fatalf("ED: %v", err)
	}
	for r := range v.cells {
		for c, cell := range v.cells[r] {
			if cell != (vtCell{ch: ' ', fg: vtDefaultFG, bg: 0}) {
				t.Fatalf("after ED cell %d,%d = %+v", r, c, cell)
			}
		}
	}
}

func TestReferenceTerminalRejectsOtherOutput(t *testing.T) {
	g := string(C64Glyph(true, 0x01))
	bad := map[string]string{
		"ASCII text":               "A",
		"carriage return":          "\r",
		"line feed":                "\n",
		"backspace":                "\b",
		"SGR 7":                    "\x1b[7m",
		"bold":                     "\x1b[1m",
		"256-colour SGR":           "\x1b[38;5;1m",
		"short 24-bit SGR":         "\x1b[38;2;1;2m",
		"colour component":         "\x1b[38;2;1;2;256m",
		"ED 0":                     "\x1b[J",
		"EL 2":                     "\x1b[2K",
		"CUP row 0":                "\x1b[0;1H",
		"CUP row 26":               "\x1b[26;1H",
		"CUP column 41":            "\x1b[1;41H",
		"private mode":             "\x1b[?25l",
		"truncated CSI":            "\x1b[12",
		"RIS":                      "\x1bc",
		"Powerline code point":     string(rune(0xE0A0)),
		"reversed glyph":           string(rune(0xEF81)),
		"invalid UTF-8":            "\xff",
		"glyph after column 40":    "\x1b[1;40H" + g + g,
		"EL with a pending wrap":   "\x1b[1;40H" + g + "\x1b[K",
		"scroll by zero":           "\x1b[0S",
		"scroll with two operands": "\x1b[1;1S",
		"open synchronized block":  "\x1b[?2026h" + g,
		"sync end without start":   "\x1b[?2026l",
		"nested synchronized":      "\x1b[?2026h\x1b[?2026h\x1b[?2026l\x1b[?2026l",
		"other private mode 2027":  "\x1b[?2027h",
		"truncated sync marker":    "\x1b[?2026",
	}
	for name, out := range bad {
		if err := newVTScreen().write([]byte(out)); err == nil {
			t.Errorf("%s: %q accepted", name, out)
		}
	}
}
