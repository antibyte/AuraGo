package retronet

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"
)

// petsciiHarness decodes chunks with one engine, replays every output on the
// reference terminal and requires terminal and model to agree after each
// Decode call.
type petsciiHarness struct {
	t  testing.TB
	c  *petsciiCodec
	vt *vtScreen
}

func newPetsciiHarness(t testing.TB) *petsciiHarness {
	return &petsciiHarness{t: t, c: newPetsciiCodec(), vt: newVTScreen()}
}

// feed decodes each chunk in its own Decode call and returns all output.
func (h *petsciiHarness) feed(chunks ...string) []byte {
	h.t.Helper()
	var all []byte
	for _, chunk := range chunks {
		out := h.c.Decode(nil, []byte(chunk))
		h.check(out)
		all = append(all, out...)
	}
	return all
}

// check validates one Decode output against the reference terminal. Until the
// engine's first output the terminal shows the browser's own lines, so the
// model comparison starts with that output.
func (h *petsciiHarness) check(out []byte) {
	h.t.Helper()
	if !utf8.Valid(out) {
		h.t.Fatalf("output is not valid UTF-8: %q", out)
	}
	if err := h.vt.write(out); err != nil {
		h.t.Fatalf("reference terminal: %v\noutput %q", err, out)
	}
	if !h.c.started {
		return
	}
	if d := vtDiff(h.vt, &h.c.scr); d != "" {
		h.t.Fatalf("terminal and model differ: %s\noutput %q", d, out)
	}
}

// vtDiff compares the reference terminal with the engine's model and returns
// the first difference, or "". A blank model cell (non-reversed space) must
// look blank: a space glyph of either set or an erased cell, on black.
func vtDiff(v *vtScreen, s *c64Screen) string {
	for r := 0; r < c64Rows; r++ {
		for c := 0; c < c64Cols; c++ {
			m, got := s.cells[r][c], v.cells[r][c]
			if m.blank() {
				if !got.blank() {
					return fmt.Sprintf("cell %d,%d = %U fg %06X bg %06X, want blank", r, c, got.ch, got.fg, got.bg)
				}
				continue
			}
			fg, bg := int32(c64Palette[m.colour]), int32(c64Background)
			if m.reverse {
				fg, bg = bg, fg
			}
			want := vtCell{ch: C64Glyph(!s.upper, m.code), fg: fg, bg: bg}
			if got != want {
				return fmt.Sprintf("cell %d,%d = %U fg %06X bg %06X, want %U fg %06X bg %06X", r, c, got.ch, got.fg, got.bg, want.ch, want.fg, want.bg)
			}
		}
	}
	if v.pending || v.row != s.row || v.col != s.col {
		return fmt.Sprintf("terminal cursor %d,%d (pending %v), model cursor %d,%d", v.row, v.col, v.pending, s.row, s.col)
	}
	return ""
}

// rowText renders a model row with lower/upper-set letters, ASCII digits and
// punctuation, '@' for screen code 0 and '#' for everything else; trailing
// blanks are trimmed.
func rowText(s *c64Screen, row int) string {
	var b strings.Builder
	for _, cell := range s.cells[row] {
		sc := cell.code
		switch {
		case sc == 0x00:
			b.WriteByte('@')
		case sc >= 0x01 && sc <= 0x1A:
			b.WriteByte('a' + sc - 1)
		case sc >= 0x20 && sc <= 0x3F:
			b.WriteByte(sc)
		case sc >= 0x41 && sc <= 0x5A:
			b.WriteByte('A' + sc - 0x41)
		default:
			b.WriteByte('#')
		}
	}
	return strings.TrimRight(b.String(), " ")
}

func TestPetsciiFirstOutput(t *testing.T) {
	c := newPetsciiCodec()
	if got := c.Decode(nil, nil); len(got) != 0 {
		t.Fatalf("Decode(nil) = %q, want nothing", got)
	}
	if c.scr.row != 24 || c.scr.col != 0 || c.scr.colour != 14 || c.scr.reverse || c.scr.upper {
		t.Fatalf("initial state = %+v", c.scr)
	}
	a := string(C64Glyph(true, 0x01))
	got := c.Decode(nil, []byte{0x41})
	want := "\x1b[0m" + // reset the attributes the browser's own lines left
		"\x1b[25;1H" + // bottom-left corner
		"\x1b[38;2;112;109;235;48;2;0;0;0m" + a // light blue on black
	if string(got) != want {
		t.Fatalf("first Decode = %q, want %q", got, want)
	}
	if got := c.Decode(nil, []byte{0x41}); string(got) != a {
		t.Fatalf("second Decode = %q, want only the glyph", got)
	}
}

func TestPetsciiDecodeExactOutput(t *testing.T) {
	c := newPetsciiCodec()
	got := c.Decode(nil, []byte{0x93, 0x1C, 0x41, 0x12, 0x05, 0x42, 0x0D})
	want := "\x1b[0m" +
		"\x1b[48;2;0;0;0m\x1b[2J" + // CLR on black
		"\x1b[1;1H\x1b[38;2;129;51;56m" + string(C64Glyph(true, 0x01)) + // red "a"
		"\x1b[38;2;0;0;0;48;2;255;255;255m" + string(C64Glyph(true, 0x02)) + // reversed white "b": colours swapped
		"\x1b[2;1H" // CR: cursor restored to row 2
	if string(got) != want {
		t.Fatalf("Decode = %q\nwant     %q", got, want)
	}
	if bytes.Contains(got, []byte("\x1b[7m")) {
		t.Fatal("reverse video must not use SGR 7")
	}
}

func TestPetsciiDecodeAppendsToDst(t *testing.T) {
	c := newPetsciiCodec()
	got := c.Decode([]byte("pre"), []byte{0x93})
	if !bytes.HasPrefix(got, []byte("pre\x1b[0m")) {
		t.Fatalf("Decode = %q, want the prefix kept", got)
	}
}

func TestPetsciiGlyphsForEveryPrintableByte(t *testing.T) {
	for b := 0; b < 256; b++ {
		sc, ok := petsciiScreenCode(byte(b))
		if !ok {
			continue
		}
		h := newPetsciiHarness(t)
		out := h.feed("\x93", string([]byte{byte(b)}))
		if cell := h.c.scr.cells[0][0]; cell.code != sc || cell.colour != 14 || cell.reverse {
			t.Fatalf("byte 0x%02X stored %+v, want screen code 0x%02X", b, cell, sc)
		}
		if !bytes.Contains(out, []byte(string(C64Glyph(true, sc)))) {
			t.Fatalf("byte 0x%02X: output %q lacks %U", b, out, C64Glyph(true, sc))
		}
	}
}

func TestPetsciiColourCodesSetTextColour(t *testing.T) {
	for i, code := range petsciiColourCodes {
		h := newPetsciiHarness(t)
		h.feed("\x93", string([]byte{code, 0x41, 0x12, 0x42}))
		plain, rev := h.c.scr.cells[0][0], h.c.scr.cells[0][1]
		if plain.colour != byte(i) || plain.reverse || rev.colour != byte(i) || !rev.reverse {
			t.Fatalf("code 0x%02X: cells %+v %+v, want colour %d plain then reversed", code, plain, rev, i)
		}
		if got := h.vt.cells[0][1]; got.fg != int32(c64Background) || got.bg != int32(c64Palette[i]) {
			t.Fatalf("code 0x%02X: reversed cell fg %06X bg %06X, want swapped colours", code, got.fg, got.bg)
		}
	}
}

func TestPetsciiReverseAndReturn(t *testing.T) {
	h := newPetsciiHarness(t)
	h.feed("\x93\x12A\x0dB\x12C\x8dD\x12E\x92F")
	s := &h.c.scr
	if got := rowText(s, 2); got != "def" {
		t.Fatalf("row 2 = %q, want shifted CR to start a new row", got)
	}
	want := map[[2]int]bool{{0, 0}: true, {1, 0}: false, {1, 1}: true, {2, 0}: false, {2, 1}: true, {2, 2}: false}
	for pos, rev := range want {
		if s.cells[pos[0]][pos[1]].reverse != rev {
			t.Errorf("cell %d,%d reverse = %v, want %v (RVS ON, CR and shifted CR switch it off, RVS OFF)", pos[0], pos[1], !rev, rev)
		}
	}
}

func TestPetsciiCursorSemantics(t *testing.T) {
	const (
		clr, home, down, up, right, left, cr = "\x93", "\x13", "\x11", "\x91", "\x1d", "\x9d", "\x0d"
	)
	n := strings.Repeat
	tests := []struct {
		name     string
		in       string
		row, col int
		rows     map[int]string
	}{
		{"40 characters and CR leave a blank row", clr + n("A", 40) + cr + "B", 2, 1,
			map[int]string{0: n("a", 40), 1: "", 2: "b"}},
		{"printing in column 39 wraps", clr + n("A", 41), 1, 1, map[int]string{0: n("a", 40), 1: "a"}},
		{"printing past the bottom scrolls", clr + "A" + cr + "B" + home + n(down, 24) + n(right, 39) + "Z", 24, 0,
			map[int]string{0: "b", 23: n(" ", 39) + "z", 24: ""}},
		{"down at the bottom scrolls and keeps the column", clr + "A" + cr + "B" + home + n(down, 24) + n(right, 5) + down, 24, 5,
			map[int]string{0: "b", 1: ""}},
		{"CR at the bottom scrolls", clr + "A" + cr + "B" + home + n(down, 24) + "C" + cr, 24, 0,
			map[int]string{0: "b", 23: "c", 24: ""}},
		{"up stays in row 0", clr + "AB" + up, 0, 2, map[int]string{0: "ab"}},
		{"up moves one row", clr + cr + cr + "A" + up, 1, 1, map[int]string{2: "a"}},
		{"left wraps to the end of the previous row", clr + cr + left, 0, 39, nil},
		{"left stays in the top-left corner", clr + left, 0, 0, nil},
		{"right wraps to the next row", clr + n(right, 40), 1, 0, nil},
		{"right in the bottom-right corner scrolls", clr + "A" + cr + "B" + home + n(down, 24) + n(right, 40), 24, 0,
			map[int]string{0: "b"}},
		{"home keeps the screen", clr + "AB" + cr + "C" + home, 0, 0, map[int]string{0: "ab", 1: "c"}},
		{"clear blanks the screen and homes", clr + "AB" + cr + "C" + clr, 0, 0, map[int]string{0: "", 1: ""}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := newPetsciiHarness(t)
			h.feed(tc.in)
			s := &h.c.scr
			if s.row != tc.row || s.col != tc.col {
				t.Fatalf("cursor = %d,%d, want %d,%d", s.row, s.col, tc.row, tc.col)
			}
			for row, want := range tc.rows {
				if got := rowText(s, row); got != want {
					t.Errorf("row %d = %q, want %q", row, got, want)
				}
			}
		})
	}
}

func TestPetsciiClearKeepsColourAndReverse(t *testing.T) {
	h := newPetsciiHarness(t)
	h.feed("\x93\x1c\x12A\x93B")
	if cell := h.c.scr.cells[0][0]; cell.code != 0x02 || cell.colour != 2 || !cell.reverse {
		t.Fatalf("cell after CLR = %+v, want reversed red b", cell)
	}
}

func TestPetsciiScrollUsesBlackBackground(t *testing.T) {
	h := newPetsciiHarness(t)
	h.feed("\x93\x12\x05A")                            // the last SGR background is white
	out := h.feed("\x13" + strings.Repeat("\x11", 25)) // the 25th down scrolls
	if !bytes.Contains(out, []byte("\x1b[48;2;0;0;0m\x1b[S")) {
		t.Fatalf("scroll output = %q, want black background then SU", out)
	}
	if rowText(&h.c.scr, 0) != "" {
		t.Fatalf("row 0 = %q after the scroll, want blank", rowText(&h.c.scr, 0))
	}
}

func TestPetsciiANSIProbesAreSwallowed(t *testing.T) {
	tests := []struct {
		name   string
		chunks []string
		want   string
	}{
		{"cursor position request", []string{"\x1b[6nA"}, "a"},
		{"private mode", []string{"\x1b[?25hA"}, "a"},
		{"SGR with intermediate", []string{"\x1b[1;31 qA"}, "a"},
		{"split across reads", []string{"\x1b", "[", "6", "nA"}, "a"},
		{"lone ESC is ignored", []string{"\x1bAB"}, "ab"},
		{"ESC ESC", []string{"\x1b\x1b[6nA"}, "a"},
		{"control byte ends the sequence", []string{"\x1b[1\x0dA"}, ""},
		{"shifted letter ends the sequence", []string{"\x1b[1\xc1"}, "A"},
		{"16 parameter bytes are swallowed", []string{"\x1b[" + strings.Repeat("1", 16) + "mA"}, "a"},
		{"a 17th parameter byte ends the swallow", []string{"\x1b[" + strings.Repeat("1", 17) + "A"}, "1a"},
		{"intermediate bytes count too", []string{"\x1b[" + strings.Repeat(" ", 16) + "1A"}, "1a"},
		{"the cap counts across reads", []string{"\x1b[" + strings.Repeat("1", 9), strings.Repeat("1", 8) + "A"}, "1a"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := newPetsciiHarness(t)
			h.feed("\x93")
			h.feed(tc.chunks...)
			if got := rowText(&h.c.scr, 0); got != tc.want {
				t.Fatalf("row 0 = %q, want %q", got, tc.want)
			}
		})
	}
	h := newPetsciiHarness(t)
	h.feed("\x93\x1b[1\x0dA")
	if got := rowText(&h.c.scr, 1); got != "a" {
		t.Fatalf("CR inside an aborted sequence must act: row 1 = %q", got)
	}
}

func TestPetsciiBell(t *testing.T) {
	h := newPetsciiHarness(t)
	h.feed("\x93")
	if out := h.feed("\x07"); string(out) != "\x07" || h.vt.bells != 1 {
		t.Fatalf("BEL output = %q, bells %d", out, h.vt.bells)
	}
}

func TestPetsciiIgnoredControls(t *testing.T) {
	// 0x08 and 0x09 (SHIFT+C= lock) are deliberately absent: they must be ignored.
	handled := map[byte]bool{0x07: true, 0x0D: true, 0x0E: true, 0x11: true, 0x12: true, 0x13: true, 0x14: true,
		0x1B: true, 0x1D: true, 0x8D: true, 0x8E: true, 0x91: true, 0x92: true, 0x93: true, 0x94: true, 0x9D: true}
	for _, code := range petsciiColourCodes {
		handled[code] = true
	}
	for b := 0; b < 256; b++ {
		if _, printable := petsciiScreenCode(byte(b)); printable || handled[byte(b)] {
			continue
		}
		h := newPetsciiHarness(t)
		h.feed("\x93\x1cAB\x9d")
		before := h.c.scr
		if out := h.feed(string([]byte{byte(b)})); len(out) != 0 || h.c.scr != before {
			t.Errorf("byte 0x%02X: output %q, model changed %v; want ignored", b, out, h.c.scr != before)
		}
	}
}
