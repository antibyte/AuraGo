package retronet

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

func TestPetsciiDelete(t *testing.T) {
	const del, left, cr = "\x14", "\x9d", "\x0d"
	tests := []struct {
		name     string
		in       string
		row, col int
		rows     map[int]string
	}{
		{"deletes the character left of the cursor", "ABC" + del, 0, 2, map[int]string{0: "ab"}},
		{"shifts the rest of the row left", "ABCD" + left + left + del, 0, 1, map[int]string{0: "acd"}},
		{"wraps to the end of the previous row", strings.Repeat("A", 40) + del, 0, 39, map[int]string{0: strings.Repeat("a", 39), 1: ""}},
		{"does not join the next row", "A" + cr + "B" + "\x13\x1d" + del, 0, 0, map[int]string{0: "", 1: "b"}},
		{"does nothing in the top-left corner", "\x13" + del, 0, 0, nil},
		{"a run of deletes", "HELLO" + del + del + del, 0, 2, map[int]string{0: "he"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := newPetsciiHarness(t)
			h.feed("\x93", tc.in)
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
	h := newPetsciiHarness(t)
	h.feed("\x93AB")
	if out := h.feed(del); string(out) != "\x1b[?2026h\x1b[1;2H\x1b[K\x1b[?2026l" {
		t.Fatalf("DEL output = %q, want the row tail redrawn once at the end of the call (CUP + EL) in a synchronized block", out)
	}
	h.feed("\x93ABCDEF\x9d\x9d")
	out := h.feed(del + del + del + "\x94\x94")
	start, end := bytes.Index(out, []byte("\x1b[?2026h")), bytes.Index(out, []byte("\x1b[?2026l"))
	if bytes.Count(out, []byte("\x1b[?2026h")) != 1 || start < 0 || end < start || bytes.Count(out[start:end], []byte("\x1b[1;")) != 1 {
		t.Fatalf("DEL/INST run output = %q, want one redraw of row 1 in one synchronized block", out)
	}
	if got := rowText(&h.c.scr, 0); got != "a  ef" {
		t.Fatalf("row 0 = %q after DEL DEL DEL INST INST, want \"a  ef\"", got)
	}
	h.feed("\x13")
	if out := h.feed(del); len(out) != 0 {
		t.Fatalf("DEL at home output = %q, want nothing", out)
	}
}

func TestPetsciiInsert(t *testing.T) {
	h := newPetsciiHarness(t)
	h.feed("\x93ABC\x9d\x9d\x94")
	if got, s := rowText(&h.c.scr, 0), &h.c.scr; got != "a bc" || s.col != 1 {
		t.Fatalf("after INST row 0 = %q, cursor col %d; want \"a bc\", col 1", got, s.col)
	}
	full := strings.Repeat("A", 39) + "Z"
	h = newPetsciiHarness(t)
	h.feed("\x93"+full, "\x13\x94")
	if got := rowText(&h.c.scr, 0); got != " "+strings.Repeat("a", 39) {
		t.Fatalf("INST in a full row = %q, want the last cell lost", got)
	}
	if got := rowText(&h.c.scr, 1); got != "" {
		t.Fatalf("INST must not spill into the next row, row 1 = %q", got)
	}
}

func TestPetsciiCharsetSwitch(t *testing.T) {
	h := newPetsciiHarness(t)
	h.feed("\x93A\x0dB")
	out := h.feed("\x8e")
	if !h.c.scr.upper {
		t.Fatal("0x8E must select the upper/graphics set")
	}
	for row := 1; row <= c64Rows; row++ {
		if !bytes.Contains(out, []byte(fmt.Sprintf("\x1b[%d;1H", row))) {
			t.Errorf("repaint lacks row %d", row)
		}
	}
	if !bytes.Contains(out, []byte(string(C64Glyph(false, 0x01)))) || !bytes.HasPrefix(out, []byte("\x1b[?2026h")) ||
		!bytes.HasSuffix(out, []byte("\x1b[?2026l\x1b[2;2H")) {
		t.Fatalf("repaint output = %q, want upper-set glyphs in one synchronized block, then the cursor restored", out)
	}
	if out := h.feed("\x8e"); len(out) != 0 {
		t.Fatalf("0x8E in the upper set = %q, want nothing", out)
	}
	// 0x08 only locks SHIFT+C= on a real keyboard: it is ignored and the
	// switch still works, as with the C64 KERNAL.
	if out := h.feed("\x08"); len(out) != 0 {
		t.Fatalf("0x08 output = %q, want nothing", out)
	}
	if out := h.feed("\x0e"); len(out) == 0 || h.c.scr.upper {
		t.Fatal("0x0E after 0x08 must switch back to the lower/upper set and repaint")
	}
	if !bytes.Contains(h.feed("C"), []byte(string(C64Glyph(true, 0x03)))) {
		t.Fatal("printing after the switch must use the lower/upper set")
	}
	if out := h.feed("\x09\x8e"); !h.c.scr.upper || !bytes.Contains(out, []byte(string(C64Glyph(false, 0x03)))) {
		t.Fatalf("0x09 then 0x8E = %q, want the upper/graphics set repainted", out)
	}
	toggles := h.feed(strings.Repeat("\x0e\x8e", 100) + "\x0e")
	if bytes.Count(toggles, []byte("\x1b[?2026h")) != 1 || bytes.Count(toggles, []byte("\x1b[1;1H")) != 1 || h.c.scr.upper {
		t.Fatalf("201 switches in one read = %d bytes, want one repaint at the end of the call in the final set", len(toggles))
	}
}
