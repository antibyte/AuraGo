package retronet

// C64 screen geometry of the PETSCII engine.
const (
	c64Cols = 40
	c64Rows = 25
)

// c64InitialColour is the text colour after power-on: light blue.
const c64InitialColour byte = 14

// c64Background is the fixed screen background (black) as 0xRRGGBB.
const c64Background uint32 = 0x000000

// c64Palette holds the 16 C64 colours (Colodore) as 0xRRGGBB, in palette order:
// black, white, red, cyan, purple, green, blue, yellow, orange, brown,
// light red, dark grey, grey, light green, light blue, light grey.
var c64Palette = [16]uint32{
	0x000000, 0xFFFFFF, 0x813338, 0x75CEC8,
	0x8E3C97, 0x56AC4D, 0x2E2C9B, 0xEDF171,
	0x8E5029, 0x553800, 0xC46C71, 0x4A4A4A,
	0x7B7B7B, 0xA9FF9F, 0x706DEB, 0xB2B2B2,
}

// petsciiColourCodes lists the PETSCII colour control codes in palette order.
var petsciiColourCodes = [16]byte{
	0x90, 0x05, 0x1C, 0x9F, 0x9C, 0x1E, 0x1F, 0x9E,
	0x81, 0x95, 0x96, 0x97, 0x98, 0x99, 0x9A, 0x9B,
}

// petsciiColourIndex maps a byte to its palette index, or -1 if the byte is
// not a colour control code.
var petsciiColourIndex = func() [256]int8 {
	var t [256]int8
	for i := range t {
		t[i] = -1
	}
	for i, b := range petsciiColourCodes {
		t[b] = int8(i)
	}
	return t
}()

// PETSCII control codes the engine acts on (colour codes: petsciiColourCodes).
// 0x08/0x09 only lock SHIFT+C= on a C64 keyboard and are ignored.
const (
	petsciiRunStop     byte = 0x03
	petsciiBell        byte = 0x07
	petsciiReturn      byte = 0x0D
	petsciiLowerSet    byte = 0x0E
	petsciiDown        byte = 0x11
	petsciiRvsOn       byte = 0x12
	petsciiHome        byte = 0x13
	petsciiDel         byte = 0x14
	petsciiEsc         byte = 0x1B
	petsciiRight       byte = 0x1D
	petsciiShiftReturn byte = 0x8D
	petsciiUpperSet    byte = 0x8E
	petsciiUp          byte = 0x91
	petsciiRvsOff      byte = 0x92
	petsciiClear       byte = 0x93
	petsciiInsert      byte = 0x94
	petsciiLeft        byte = 0x9D
)

// c64Space is the screen code of the blank cell.
const c64Space byte = 0x20

// petsciiScreenCode converts a printable PETSCII byte to its non-reversed
// screen code (the KERNAL's CHROUT conversion). ok is false for control bytes
// (0x00-0x1F, 0x80-0x9F).
func petsciiScreenCode(b byte) (sc byte, ok bool) {
	switch {
	case b >= 0x20 && b <= 0x3F:
		return b, true
	case b >= 0x40 && b <= 0x5F:
		return b - 0x40, true
	case b >= 0x60 && b <= 0x7F:
		return b - 0x20, true
	case b >= 0xA0 && b <= 0xBF:
		return b - 0x40, true
	case b >= 0xC0 && b <= 0xFE:
		return b - 0x80, true
	case b == 0xFF:
		return 0x5E, true // pi
	}
	return 0, false
}

// C64Glyph returns the C64 Pro Mono code point for a non-reversed screen code
// (0x00-0x7F): U+EF00+sc in the lower/upper set, U+EE00+sc in the
// upper/graphics set. The reverse bit 0x80 is ignored; the engine draws
// reverse video with swapped colours.
func C64Glyph(lowerSet bool, screenCode byte) rune {
	if lowerSet {
		return 0xEF00 + rune(screenCode&0x7F)
	}
	return 0xEE00 + rune(screenCode&0x7F)
}
