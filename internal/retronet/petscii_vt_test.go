package retronet

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"
)

// vtDefaultFG is the reference terminal's default foreground (after SGR 0 and
// in erased cells). The default background is the C64 session's black.
const vtDefaultFG int32 = -1

// vtCell is one cell of the reference terminal.
type vtCell struct {
	ch     rune
	fg, bg int32 // 0xRRGGBB, or vtDefaultFG
}

// vtScreen is a minimal 40x25 xterm model for the engine's output alphabet:
// glyphs U+EE00-U+EE7F/U+EF00-U+EF7F, CUP, SGR 0 / 38;2 / 48;2, SU, ED 2,
// EL 0 and BEL. Anything else is an error, and so is a glyph written while a
// wrap is pending (the engine must never rely on autowrap). Erases and
// scrolls fill with the current background, like xterm (BCE).
type vtScreen struct {
	cells    [c64Rows][c64Cols]vtCell
	row, col int
	pending  bool // a glyph was written in column 39
	fg, bg   int32
	bells    int
}

func newVTScreen() *vtScreen {
	v := &vtScreen{fg: vtDefaultFG, bg: int32(c64Background)}
	for r := range v.cells {
		for c := range v.cells[r] {
			v.cells[r][c] = v.erased()
		}
	}
	return v
}

// erased is the cell an erase or scroll leaves: space, default foreground,
// current background.
func (v *vtScreen) erased() vtCell {
	return vtCell{ch: ' ', fg: vtDefaultFG, bg: v.bg}
}

// write interprets one Decode output. Every call must hold complete
// sequences.
func (v *vtScreen) write(p []byte) error {
	for len(p) > 0 {
		switch p[0] {
		case 0x07:
			v.bells++
			p = p[1:]
		case 0x1B:
			n, err := v.csi(p)
			if err != nil {
				return err
			}
			p = p[n:]
		default:
			r, size := utf8.DecodeRune(p)
			if r == utf8.RuneError && size <= 1 {
				return fmt.Errorf("invalid UTF-8 at % X", p[:1])
			}
			if !(r >= 0xEE00 && r <= 0xEE7F) && !(r >= 0xEF00 && r <= 0xEF7F) {
				return fmt.Errorf("unexpected output %U", r)
			}
			if v.pending {
				return fmt.Errorf("glyph %U written with a pending wrap at row %d", r, v.row+1)
			}
			v.cells[v.row][v.col] = vtCell{ch: r, fg: v.fg, bg: v.bg}
			if v.col == c64Cols-1 {
				v.pending = true
			} else {
				v.col++
			}
			p = p[size:]
		}
	}
	return nil
}

// csi interprets one ESC [ params final sequence at the start of p.
func (v *vtScreen) csi(p []byte) (int, error) {
	if len(p) < 2 || p[1] != '[' {
		return 0, fmt.Errorf("unexpected escape %q", p)
	}
	n := 2
	for n < len(p) && (p[n] >= '0' && p[n] <= '9' || p[n] == ';') {
		n++
	}
	if n == len(p) {
		return 0, fmt.Errorf("truncated sequence %q", p)
	}
	raw := string(p[2:n])
	var params []int
	if raw != "" {
		for _, f := range strings.Split(raw, ";") {
			x, err := strconv.Atoi(f)
			if err != nil {
				return 0, fmt.Errorf("bad parameter in %q", p[:n+1])
			}
			params = append(params, x)
		}
	}
	seq := string(p[:n+1])
	switch p[n] {
	case 'H':
		row, col := 1, 1
		if len(params) > 0 {
			row = params[0]
		}
		if len(params) > 1 {
			col = params[1]
		}
		if len(params) > 2 || row < 1 || row > c64Rows || col < 1 || col > c64Cols {
			return 0, fmt.Errorf("CUP out of range: %q", seq)
		}
		v.row, v.col, v.pending = row-1, col-1, false
	case 'm':
		if err := v.sgr(params); err != nil {
			return 0, fmt.Errorf("%v in %q", err, seq)
		}
	case 'S':
		count := 1
		if len(params) == 1 {
			count = params[0]
		}
		if len(params) > 1 || count < 1 {
			return 0, fmt.Errorf("bad SU %q", seq)
		}
		for ; count > 0; count-- {
			copy(v.cells[:c64Rows-1], v.cells[1:])
			for c := range v.cells[c64Rows-1] {
				v.cells[c64Rows-1][c] = v.erased()
			}
		}
	case 'J':
		if raw != "2" {
			return 0, fmt.Errorf("unsupported ED %q", seq)
		}
		for r := range v.cells {
			for c := range v.cells[r] {
				v.cells[r][c] = v.erased()
			}
		}
	case 'K':
		if raw != "" && raw != "0" {
			return 0, fmt.Errorf("unsupported EL %q", seq)
		}
		if v.pending {
			return 0, fmt.Errorf("EL with a pending wrap at row %d", v.row+1)
		}
		for c := v.col; c < c64Cols; c++ {
			v.cells[v.row][c] = v.erased()
		}
	default:
		return 0, fmt.Errorf("unsupported sequence %q", seq)
	}
	return n + 1, nil
}

// sgr accepts SGR 0 on its own and any run of 38;2;r;g;b and 48;2;r;g;b.
func (v *vtScreen) sgr(params []int) error {
	if len(params) == 0 || len(params) == 1 && params[0] == 0 {
		v.fg, v.bg = vtDefaultFG, int32(c64Background)
		return nil
	}
	for len(params) > 0 {
		if len(params) < 5 || params[1] != 2 || params[0] != 38 && params[0] != 48 {
			return fmt.Errorf("unsupported SGR %v", params)
		}
		for _, x := range params[2:5] {
			if x < 0 || x > 255 {
				return fmt.Errorf("colour component %d", x)
			}
		}
		rgb := int32(params[2]<<16 | params[3]<<8 | params[4])
		if params[0] == 38 {
			v.fg = rgb
		} else {
			v.bg = rgb
		}
		params = params[5:]
	}
	return nil
}
