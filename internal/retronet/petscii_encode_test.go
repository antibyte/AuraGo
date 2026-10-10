package retronet

import (
	"bytes"
	"fmt"
	"testing"
)

func TestPetsciiEncodeKeys(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []byte
	}{
		{"lower-case letters", "az", []byte{0x41, 0x5A}},
		{"capital letters", "AZ", []byte{0xC1, 0xDA}},
		{"digits and space", "0 9", []byte("0 9")},
		{"punctuation", "!\"#$%&'()*+,-./:;<=>?", []byte("!\"#$%&'()*+,-./:;<=>?")},
		{"at and brackets", "@[]", []byte{0x40, 0x5B, 0x5D}},
		{"pound", "\U000000A3", []byte{0x5C}},
		{"up arrow", "^\U00002191", []byte{0x5E, 0x5E}},
		{"left arrow", "_\U00002190", []byte{0x5F, 0x5F}},
		{"return", "\r", []byte{0x0D}},
		{"backspace", "\x7f\x08", []byte{0x14, 0x14}},
		{"delete", "\x1b[3~", []byte{0x14}},
		{"cursor keys", "\x1b[A\x1b[B\x1b[C\x1b[D", []byte{0x91, 0x11, 0x1D, 0x9D}},
		{"application cursor keys", "\x1bOA\x1bOB\x1bOC\x1bOD", []byte{0x91, 0x11, 0x1D, 0x9D}},
		{"home", "\x1b[H\x1bOH\x1b[1~", []byte{0x13, 0x13, 0x13}},
		{"shift home is CLR", "\x1b[1;2H", []byte{0x93}},
		{"insert", "\x1b[2~", []byte{0x94}},
		{"F1 F3 F5 F7", "\x1bOP\x1bOR\x1b[15~\x1b[18~", []byte{0x85, 0x86, 0x87, 0x88}},
		{"F2 F4 F6 F8", "\x1bOQ\x1bOS\x1b[17~\x1b[19~", []byte{0x89, 0x8A, 0x8B, 0x8C}},
		{"lone escape is RUN/STOP", "\x1b", []byte{0x03}},
		{"ctrl-c is RUN/STOP", "\x03", []byte{0x03}},
		{"pasted word", "Hello", []byte{0xC8, 0x45, 0x4C, 0x4C, 0x4F}},
		{"keys and sequences mixed", "a\x1b[Cb\r", []byte{0x41, 0x1D, 0x42, 0x0D}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := newPetsciiCodec().Encode(nil, []byte(tc.in)); !bytes.Equal(got, tc.want) {
				t.Fatalf("Encode(%q) = % X, want % X", tc.in, got, tc.want)
			}
		})
	}
}

func TestPetsciiEncodeEverySingleASCIIByte(t *testing.T) {
	// Oracle written from the spec's key list.
	want := map[byte]byte{'\r': 0x0D, 0x7F: 0x14, 0x08: 0x14, 0x03: 0x03, '@': 0x40, '[': 0x5B, ']': 0x5D, '^': 0x5E, '_': 0x5F}
	for b := 0x20; b <= 0x3F; b++ {
		want[byte(b)] = byte(b)
	}
	for b := 'a'; b <= 'z'; b++ {
		want[byte(b)] = byte(b) - 'a' + 0x41
	}
	for b := 'A'; b <= 'Z'; b++ {
		want[byte(b)] = byte(b) - 'A' + 0xC1
	}
	for b := 0; b < 0x80; b++ {
		if b == 0x1B {
			continue // lone ESC: TestPetsciiEncodeKeys
		}
		got := newPetsciiCodec().Encode(nil, []byte{byte(b)})
		w, mapped := want[byte(b)]
		switch {
		case mapped && !bytes.Equal(got, []byte{w}):
			t.Errorf("Encode(0x%02X) = % X, want %02X", b, got, w)
		case !mapped && len(got) != 0:
			t.Errorf("Encode(0x%02X) = % X, want dropped", b, got)
		}
	}
}

func TestPetsciiEncodePrivateColourKeys(t *testing.T) {
	allowed := append(petsciiColourCodes[:], 0x12, 0x92)
	for _, b := range allowed {
		in := fmt.Sprintf("\x1b[64;%d~", b)
		if got := newPetsciiCodec().Encode(nil, []byte(in)); !bytes.Equal(got, []byte{b}) {
			t.Errorf("Encode(%q) = % X, want %02X", in, got, b)
		}
	}
	refused := []string{
		"\x1b[64;65~",   // a letter
		"\x1b[64;13~",   // RETURN
		"\x1b[64;147~",  // CLR
		"\x1b[64;0~",    // NUL
		"\x1b[64;256~",  // out of range
		"\x1b[64;1440~", // four digits
		"\x1b[64;~",     // no number
		"\x1b[64;5;5~",  // extra parameter
		"\x1b[64;5:~",   // not decimal
		"\x1b[63;5~",    // wrong selector
		"\x1b[64;5$",    // wrong final byte
	}
	for _, in := range refused {
		if got := newPetsciiCodec().Encode(nil, []byte(in)); len(got) != 0 {
			t.Errorf("Encode(%q) = % X, want dropped", in, got)
		}
	}
}

func TestPetsciiEncodeDropsEverythingElse(t *testing.T) {
	tests := []string{
		"\\`{|}~",                        // ASCII without a C64 key
		"\n\t\x00\x1f",                   // other controls
		"\U000000E9\U000020AC\U0001F600", // other runes
		"\xff\xc3",                       // invalid UTF-8
		"\x1b[5~\x1b[6~",                 // PageUp, PageDown
		"\x1b[1;5A",                      // Ctrl+Up
		"\x1b[200~",                      // bracketed paste start
		"\x1bOx",                         // unknown SS3
		"\x1bx\x1b\x1b",                  // Alt chords
		"\x1b\U000000E9",                 // Alt + non-ASCII
		"\x1b[",                          // incomplete
		"\x1bO",                          // incomplete
		"\x1b[12",                        // incomplete with parameters
	}
	for _, in := range tests {
		if got := newPetsciiCodec().Encode(nil, []byte(in)); len(got) != 0 {
			t.Errorf("Encode(%q) = % X, want dropped", in, got)
		}
	}
	if got := newPetsciiCodec().Encode(nil, []byte("\x1b[1\r")); !bytes.Equal(got, []byte{0x0D}) {
		t.Errorf("malformed sequence = % X, want only the RETURN after it (0D)", got)
	}
	if got := newPetsciiCodec().Encode(nil, []byte("a\x1b[5~b\x1b[")); !bytes.Equal(got, []byte{0x41, 0x42}) {
		t.Errorf("unknown sequences inside a paste = % X, want 41 42", got)
	}
}

func TestPetsciiEncodeAppendsAndIgnoresScreenState(t *testing.T) {
	c := newPetsciiCodec()
	c.Decode(nil, []byte{0x8E, 0x08, 0x12})
	if got := c.Encode([]byte("pre"), []byte("a")); string(got) != "pre\x41" {
		t.Fatalf("Encode = %q, want \"preA\"", got)
	}
}

func TestPetsciiEncodeDoesNotAllocate(t *testing.T) {
	c := newPetsciiCodec()
	in := []byte("Hello \x1b[A\x1bOP\x1b[64;5~\x1b[3~\U000000A3\x1b")
	dst := make([]byte, 0, 64)
	allocs := testing.AllocsPerRun(100, func() {
		dst = c.Encode(dst[:0], in)
	})
	if allocs != 0 {
		t.Fatalf("Encode allocates %.1f times per call, want 0", allocs)
	}
}

func TestNewCodecPETSCII(t *testing.T) {
	a, ok := NewCodec(CharsetPETSCII).(*petsciiCodec)
	if !ok {
		t.Fatalf("NewCodec(petscii) = %T, want *petsciiCodec", NewCodec(CharsetPETSCII))
	}
	b := NewCodec(CharsetPETSCII).(*petsciiCodec)
	if a == b {
		t.Fatal("NewCodec must return a new engine per call")
	}
	a.Decode(nil, []byte{0x93, 0x41})
	if b.scr.cells[0][0].code != c64Space || b.started {
		t.Fatal("engines must not share state")
	}
	if CharsetPETSCII != "petscii" {
		t.Fatalf("CharsetPETSCII = %q", CharsetPETSCII)
	}
}
