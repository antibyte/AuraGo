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

	// dirty holds, per row, the first column whose xterm content is stale
	// (DEL, INST, charset switch), c64Cols when the row is current. Decode
	// redraws stale cells once, at the end of the call.
	dirty [c64Rows]uint8

	// xterm's state as far as the emitted output determines it.
	vtRow, vtCol int
	vtKnown      bool   // false until the first CUP and after a glyph in column 39 (pending wrap)
	fg, bg       uint32 // current SGR colours, rgbUnknown if not known
}

// Synchronized-output block (DEC private mode 2026) around deferred redraws:
// xterm renders it at once and the browser throttle counts it as one unit.
const (
	syncOutputBegin = "\x1b[?2026h"
	syncOutputEnd   = "\x1b[?2026l"
)

// newPetsciiCodec returns an engine with a blank screen, the cursor in the
// bottom-left corner (board output scrolls the browser's dial lines away),
// light blue text and the lower/upper set.
func newPetsciiCodec() *petsciiCodec {
	c := &petsciiCodec{fg: rgbUnknown, bg: rgbUnknown}
	c.scr.colour = c64InitialColour
	c.scr.clear()
	c.scr.row = c64Rows - 1
	c.markClean()
	return c
}

// Decode feeds server bytes into the screen model and appends the xterm
// output to dst. Glyphs, scrolling, CLR and BEL are emitted as they come;
// rows changed by DEL, INST or a charset switch are redrawn once at the end
// of the call in one synchronized-output block. Every call emits complete
// sequences only, at most 48 KiB plus 64 bytes per input byte, and ends with
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
	dst = c.flush(dst)
	return c.moveVT(dst, c.scr.row, c.scr.col)
}

// markClean marks every row as current.
func (c *petsciiCodec) markClean() {
	for row := range c.dirty {
		c.dirty[row] = c64Cols
	}
}

// markDirty records that xterm's cells from col to the end of row are stale.
func (c *petsciiCodec) markDirty(row, col int) {
	if uint8(col) < c.dirty[row] {
		c.dirty[row] = uint8(col)
	}
}

// flush redraws the stale part of every dirty row inside one
// synchronized-output block and marks the screen current.
func (c *petsciiCodec) flush(dst []byte) []byte {
	open := false
	for row, from := range c.dirty {
		if from >= c64Cols {
			continue
		}
		if !open {
			dst = append(dst, syncOutputBegin...)
			open = true
		}
		dst = c.drawRow(dst, row, int(from))
		c.dirty[row] = c64Cols
	}
	if open {
		dst = append(dst, syncOutputEnd...)
	}
	return dst
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
		c.markClean() // ED leaves nothing stale
		dst = c.setBackground(dst)
		return append(dst, "\x1b[2J"...)
	case petsciiRvsOn:
		s.reverse = true
	case petsciiRvsOff:
		s.reverse = false
	case petsciiBell:
		return append(dst, 0x07)
	case petsciiDel:
		c.del()
	case petsciiInsert:
		c.insert()
	case petsciiLowerSet:
		c.switchSet(false)
	case petsciiUpperSet:
		c.switchSet(true)
	}
	return dst
}

// print stores sc at the cursor in the current colour and reverse mode, draws
// it (unless the cell is stale anyway and the flush will draw it) and
// advances the cursor.
func (c *petsciiCodec) print(dst []byte, sc byte) []byte {
	s := &c.scr
	s.cells[s.row][s.col] = c64Cell{code: sc, colour: s.colour, reverse: s.reverse}
	if s.col < int(c.dirty[s.row]) {
		dst = c.drawCell(dst, s.row, s.col)
	}
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
// background) and keeps its cursor. Stale rows move up with their content.
func (c *petsciiCodec) scroll(dst []byte) []byte {
	s := &c.scr
	copy(s.cells[:c64Rows-1], s.cells[1:])
	blank := s.blankCell()
	for col := range s.cells[c64Rows-1] {
		s.cells[c64Rows-1][col] = blank
	}
	copy(c.dirty[:c64Rows-1], c.dirty[1:])
	c.dirty[c64Rows-1] = c64Cols
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

// del is the C64 DEL key: the cursor moves left (wrapping to the end of the
// previous row), the rest of the row shifts left onto it and the last cell
// becomes blank. Nothing happens in the top-left corner. The row tail is
// redrawn by the flush.
func (c *petsciiCodec) del() {
	s := &c.scr
	if s.row == 0 && s.col == 0 {
		return
	}
	s.left()
	row := &s.cells[s.row]
	copy(row[s.col:], row[s.col+1:])
	row[c64Cols-1] = s.blankCell()
	c.markDirty(s.row, s.col)
}

// insert is the C64 INST key: a blank cell is inserted at the cursor, the
// last cell of the row is lost and the cursor stays. The row tail is redrawn
// by the flush.
func (c *petsciiCodec) insert() {
	s := &c.scr
	row := &s.cells[s.row]
	copy(row[s.col+1:], row[s.col:c64Cols-1])
	row[s.col] = s.blankCell()
	c.markDirty(s.row, s.col)
}

// switchSet selects the lower/upper (upper=false) or upper/graphics set; the
// flush repaints all 25 rows. Nothing happens when the set does not change.
func (c *petsciiCodec) switchSet(upper bool) {
	if c.scr.upper == upper {
		return
	}
	c.scr.upper = upper
	for row := range c.dirty {
		c.dirty[row] = 0
	}
}

// drawRow re-emits cells from..39 of row: glyphs up to the last non-blank
// cell, then one erase to the end of the line (EL on black) for the blank
// rest.
func (c *petsciiCodec) drawRow(dst []byte, row, from int) []byte {
	cells := &c.scr.cells[row]
	last := c64Cols - 1
	for last >= from && cells[last].blank() {
		last--
	}
	for col := from; col <= last; col++ {
		dst = c.drawCell(dst, row, col)
	}
	if last == c64Cols-1 {
		return dst
	}
	dst = c.moveVT(dst, row, last+1)
	dst = c.setBackground(dst)
	return append(dst, "\x1b[K"...)
}
