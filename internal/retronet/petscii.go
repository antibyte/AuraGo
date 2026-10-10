package retronet

import (
	"strconv"
	"unicode/utf8"
)

// c64Cell is one character cell of the C64 screen model.
type c64Cell struct {
	code    byte // non-reversed screen code 0x00-0x7F
	colour  byte // palette index 0-15
	reverse bool
}

// blank reports whether the cell shows nothing: a non-reversed space.
func (c c64Cell) blank() bool {
	return c.code == c64Space && !c.reverse
}

// c64Screen is the authoritative C64 screen: 40x25 cells, the cursor and the
// screen editor's modes.
type c64Screen struct {
	cells    [c64Rows][c64Cols]c64Cell
	row, col int
	colour   byte // current text colour (palette index)
	reverse  bool // RVS ON
	upper    bool // upper/graphics set active; false: lower/upper set
}

// blankCell is the cell CLR, scrolling, DEL and INST leave behind.
func (s *c64Screen) blankCell() c64Cell {
	return c64Cell{code: c64Space, colour: s.colour}
}

// clear blanks every cell and homes the cursor.
func (s *c64Screen) clear() {
	blank := s.blankCell()
	for r := range s.cells {
		for c := range s.cells[r] {
			s.cells[r][c] = blank
		}
	}
	s.row, s.col = 0, 0
}

// left moves the cursor one cell left, wrapping to column 39 of the previous
// row; it stays in the top-left corner.
func (s *c64Screen) left() {
	switch {
	case s.col > 0:
		s.col--
	case s.row > 0:
		s.row--
		s.col = c64Cols - 1
	}
}

// petsciiEscape is the state of the ANSI-probe swallower.
type petsciiEscape uint8

const (
	escNone petsciiEscape = iota // ordinary bytes
	escSeen                      // after ESC
	escCSI                       // inside ESC [ (parameter and intermediate bytes)
)

// rgbUnknown is no 24-bit colour; it forces the next SGR.
const rgbUnknown uint32 = 1 << 24

// petsciiCodec is the stateful PETSCII engine. Decode keeps the C64 screen
// model and draws it on xterm with C64 Pro Mono glyphs, 24-bit SGR colours
// and explicit cursor positioning; Encode maps browser keys to PETSCII. Both
// run on the session goroutine; there is no locking.
type petsciiCodec struct {
	scr     c64Screen
	esc     petsciiEscape
	started bool // the first output reset xterm's attributes

	// xterm's state as far as the emitted output determines it.
	vtRow, vtCol int
	vtKnown      bool   // false until the first CUP and after a glyph in column 39 (pending wrap)
	fg, bg       uint32 // current SGR colours, rgbUnknown if not known
}

// newPetsciiCodec returns an engine with a blank screen, the cursor in the
// bottom-left corner (board output scrolls the browser's dial lines away),
// light blue text and the lower/upper set.
func newPetsciiCodec() *petsciiCodec {
	c := &petsciiCodec{fg: rgbUnknown, bg: rgbUnknown}
	c.scr.colour = c64InitialColour
	c.scr.clear()
	c.scr.row = c64Rows - 1
	return c
}

// Decode feeds server bytes into the screen model and appends the xterm
// output to dst. Every call emits complete sequences only and ends with
// xterm's cursor on the model cursor.
func (c *petsciiCodec) Decode(dst, src []byte) []byte {
	if len(src) == 0 {
		return dst
	}
	if !c.started {
		c.started = true
		dst = append(dst, "\x1b[0m"...)
	}
	for _, b := range src {
		dst = c.decodeByte(dst, b)
	}
	return c.moveVT(dst, c.scr.row, c.scr.col)
}

// decodeByte handles one byte: the ANSI-probe swallower first, then printable
// characters, then control codes.
func (c *petsciiCodec) decodeByte(dst []byte, b byte) []byte {
	switch c.esc {
	case escSeen:
		c.esc = escNone
		if b == '[' {
			c.esc = escCSI
			return dst
		}
		// A lone ESC is ignored; b is an ordinary byte.
	case escCSI:
		switch {
		case b >= 0x20 && b <= 0x3F: // parameter and intermediate bytes
			return dst
		case b >= 0x40 && b <= 0x7E: // final byte
			c.esc = escNone
			return dst
		}
		c.esc = escNone // not part of a CSI: the sequence ends, b is ordinary
	}
	if b == petsciiEsc {
		c.esc = escSeen
		return dst
	}
	if sc, ok := petsciiScreenCode(b); ok {
		return c.print(dst, sc)
	}
	return c.control(dst, b)
}

// control applies a control code; codes without a meaning are ignored,
// including 0x08/0x09 (they only lock SHIFT+C= on a real keyboard).
func (c *petsciiCodec) control(dst []byte, b byte) []byte {
	s := &c.scr
	if i := petsciiColourIndex[b]; i >= 0 {
		s.colour = byte(i)
		return dst
	}
	switch b {
	case petsciiReturn, petsciiShiftReturn: // the KERNAL treats both alike
		s.reverse = false
		s.col = 0
		return c.down(dst)
	case petsciiDown:
		return c.down(dst)
	case petsciiUp:
		if s.row > 0 {
			s.row--
		}
	case petsciiRight:
		return c.advance(dst)
	case petsciiLeft:
		s.left()
	case petsciiHome:
		s.row, s.col = 0, 0
	case petsciiClear:
		s.clear()
		dst = c.setBackground(dst)
		return append(dst, "\x1b[2J"...)
	case petsciiRvsOn:
		s.reverse = true
	case petsciiRvsOff:
		s.reverse = false
	case petsciiBell:
		return append(dst, 0x07)
	}
	return dst
}

// print stores sc at the cursor in the current colour and reverse mode, draws
// it and advances the cursor.
func (c *petsciiCodec) print(dst []byte, sc byte) []byte {
	s := &c.scr
	s.cells[s.row][s.col] = c64Cell{code: sc, colour: s.colour, reverse: s.reverse}
	dst = c.drawCell(dst, s.row, s.col)
	return c.advance(dst)
}

// advance moves the cursor one cell right; column 39 wraps to column 0 of the
// next row, scrolling at the bottom.
func (c *petsciiCodec) advance(dst []byte) []byte {
	if c.scr.col < c64Cols-1 {
		c.scr.col++
		return dst
	}
	c.scr.col = 0
	return c.down(dst)
}

// down moves the cursor one row down, scrolling at the bottom.
func (c *petsciiCodec) down(dst []byte) []byte {
	if c.scr.row < c64Rows-1 {
		c.scr.row++
		return dst
	}
	return c.scroll(dst)
}

// scroll moves every row up one and blanks the bottom row; xterm scrolls with
// SU on a black background (it fills the new row with the current
// background) and keeps its cursor.
func (c *petsciiCodec) scroll(dst []byte) []byte {
	s := &c.scr
	copy(s.cells[:c64Rows-1], s.cells[1:])
	blank := s.blankCell()
	for col := range s.cells[c64Rows-1] {
		s.cells[c64Rows-1][col] = blank
	}
	dst = c.setBackground(dst)
	return append(dst, "\x1b[S"...)
}

// drawCell writes the model cell (row, col) to xterm: CUP if xterm's cursor is
// elsewhere, the colours (reverse swaps them) and the glyph of the active set.
func (c *petsciiCodec) drawCell(dst []byte, row, col int) []byte {
	cell := c.scr.cells[row][col]
	fg, bg := c64Palette[cell.colour], c64Background
	if cell.reverse {
		fg, bg = bg, fg
	}
	dst = c.moveVT(dst, row, col)
	dst = c.setColours(dst, fg, bg)
	dst = utf8.AppendRune(dst, C64Glyph(!c.scr.upper, cell.code))
	if col < c64Cols-1 {
		c.vtCol = col + 1
	} else {
		c.vtKnown = false // pending wrap: the engine never relies on autowrap
	}
	return dst
}

// moveVT emits CUP unless xterm's cursor is known to be at (row, col).
func (c *petsciiCodec) moveVT(dst []byte, row, col int) []byte {
	if c.vtKnown && c.vtRow == row && c.vtCol == col {
		return dst
	}
	dst = append(dst, 0x1B, '[')
	dst = strconv.AppendInt(dst, int64(row+1), 10)
	dst = append(dst, ';')
	dst = strconv.AppendInt(dst, int64(col+1), 10)
	dst = append(dst, 'H')
	c.vtRow, c.vtCol, c.vtKnown = row, col, true
	return dst
}

// setColours emits one 24-bit SGR for the colours that change
// (38;2;r;g;b and/or 48;2;r;g;b).
func (c *petsciiCodec) setColours(dst []byte, fg, bg uint32) []byte {
	if c.fg == fg && c.bg == bg {
		return dst
	}
	dst = append(dst, 0x1B, '[')
	if c.fg != fg {
		dst = appendRGB(append(dst, "38;2;"...), fg)
		if c.bg != bg {
			dst = append(dst, ';')
		}
	}
	if c.bg != bg {
		dst = appendRGB(append(dst, "48;2;"...), bg)
	}
	c.fg, c.bg = fg, bg
	return append(dst, 'm')
}

// setBackground selects black before an erase or scroll: xterm fills the
// erased cells with the current background.
func (c *petsciiCodec) setBackground(dst []byte) []byte {
	return c.setColours(dst, c.fg, c64Background)
}

// appendRGB appends "r;g;b" of a 0xRRGGBB colour.
func appendRGB(dst []byte, rgb uint32) []byte {
	dst = strconv.AppendUint(dst, uint64(rgb>>16&0xFF), 10)
	dst = append(dst, ';')
	dst = strconv.AppendUint(dst, uint64(rgb>>8&0xFF), 10)
	dst = append(dst, ';')
	return strconv.AppendUint(dst, uint64(rgb&0xFF), 10)
}
