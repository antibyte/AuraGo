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

// petsciiASCIIKeys maps single ASCII bytes from the browser to PETSCII; 0
// means the byte is dropped (no key produces PETSCII 0x00).
var petsciiASCIIKeys = func() [128]byte {
	var t [128]byte
	for b := 0x20; b <= 0x3F; b++ {
		t[b] = byte(b) // space, digits and !"#$%&'()*+,-./:;<=>?
	}
	for b := 'a'; b <= 'z'; b++ {
		t[b] = byte(b-'a') + 0x41
	}
	for b := 'A'; b <= 'Z'; b++ {
		t[b] = byte(b-'A') + 0xC1
	}
	t['@'] = 0x40
	t['['] = 0x5B
	t[']'] = 0x5D
	t['^'] = 0x5E
	t['_'] = 0x5F
	t['\r'] = petsciiReturn
	t[0x7F] = petsciiDel
	t[0x08] = petsciiDel
	t[0x03] = petsciiRunStop
	return t
}()

// petsciiRuneKey maps the non-ASCII keys the browser can send.
func petsciiRuneKey(r rune) (byte, bool) {
	switch r {
	case '\U000000A3': // pound sign
		return 0x5C, true
	case '\U00002191': // up arrow
		return 0x5E, true
	case '\U00002190': // left arrow
		return 0x5F, true
	}
	return 0, false
}

// petsciiKeySequences maps complete xterm key sequences to PETSCII.
var petsciiKeySequences = map[string]byte{
	"\x1b[A": 0x91, "\x1bOA": 0x91, // up
	"\x1b[B": 0x11, "\x1bOB": 0x11, // down
	"\x1b[C": 0x1D, "\x1bOC": 0x1D, // right
	"\x1b[D": 0x9D, "\x1bOD": 0x9D, // left
	"\x1b[H": 0x13, "\x1bOH": 0x13, "\x1b[1~": 0x13, // Home -> HOME
	"\x1b[1;2H": 0x93, // Shift+Home -> CLR
	"\x1b[2~":   0x94, // Insert -> INST
	"\x1b[3~":   0x14, // Delete -> DEL
	"\x1bOP":    0x85, // F1
	"\x1bOQ":    0x89, // F2
	"\x1bOR":    0x86, // F3
	"\x1bOS":    0x8A, // F4
	"\x1b[15~":  0x87, // F5
	"\x1b[17~":  0x8B, // F6
	"\x1b[18~":  0x88, // F7
	"\x1b[19~":  0x8C, // F8
}

// petsciiPrivatePrefix starts the browser's colour-key sequence ESC [ 6 4 ; n ~.
const petsciiPrivatePrefix = "\x1b[64;"

// petsciiPrivateAllowed reports whether the colour-key sequence may send b:
// the 16 colour codes, RVS ON and RVS OFF.
func petsciiPrivateAllowed(b byte) bool {
	return petsciiColourIndex[b] >= 0 || b == petsciiRvsOn || b == petsciiRvsOff
}
