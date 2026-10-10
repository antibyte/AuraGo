package retronet

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

// petsciiSample is a board-like stream exercising every engine path.
var petsciiSample = func() []byte {
	var b bytes.Buffer
	b.WriteString("\x93\x0e\x05WELCOME TO \x9eTHE \x1c\xc2\xc2\xd3\x0d")
	b.WriteString("\x12\x9f \xc3\x9aO\x1fM\x9cM\x1cO\x81D\x9eO\x99R\x9fE \x92\x0d")
	b.WriteString("\x1b[6n")
	b.WriteString(strings.Repeat("\xa6", 40) + "\x0d")
	b.WriteString("\x11\x11\x1d\x1d\x1dMENU\x9d\x9d\x14\x14\x94X\x07")
	b.WriteString("\x8e\xb0\xc0\xc0\xae\x8d\xdd  \xdd\x0d\xad\xc0\xc0\xbd\x0e")
	b.WriteString("\x08\x8e\x09\x13\x91\x9d\x14\xff\x1b\x1bZ\x1b[1;\x0d")
	for i := 0; i < 30; i++ {
		b.WriteString(fmt.Sprintf("\x9a%02d \x12LINE\x92 \x99%s\x0d", i, strings.Repeat("=", i%40)))
	}
	b.WriteString("\x13" + strings.Repeat("\x1d", 39) + "\x1d\x1d" + strings.Repeat("\x9d", 3) + "\x94\x94Q\x93\x8eDONE")
	return b.Bytes()
}()

func TestPetsciiSplitReadsAtEveryBoundary(t *testing.T) {
	whole := newPetsciiHarness(t)
	whole.feed(string(petsciiSample))
	for i := 0; i <= len(petsciiSample); i++ {
		h := newPetsciiHarness(t)
		h.feed(string(petsciiSample[:i]), string(petsciiSample[i:]))
		if h.c.scr != whole.c.scr {
			t.Fatalf("split at %d: model differs from the unsplit run", i)
		}
		if h.vt.visible() != whole.vt.visible() || h.vt.row != whole.vt.row || h.vt.col != whole.vt.col || h.vt.bells != whole.vt.bells {
			t.Fatalf("split at %d: rendered output differs from the unsplit run", i)
		}
	}
	h := newPetsciiHarness(t)
	for _, b := range petsciiSample {
		h.feed(string([]byte{b}))
	}
	if h.c.scr != whole.c.scr || h.vt.visible() != whole.vt.visible() {
		t.Fatal("byte-by-byte decoding differs from the unsplit run")
	}
}

// petsciiOutputBound is the most output one Decode call may produce for n
// input bytes: one deferred full-screen redraw plus 64 bytes per byte.
func petsciiOutputBound(n int) int {
	return 48<<10 + 64*n
}

// petsciiBusyScreen fills the screen so that every cell needs its own SGR:
// reverse and colour change from cell to cell (the review's worst case).
func petsciiBusyScreen() []byte {
	b := []byte{0x93}
	for i := 0; i < c64Rows*c64Cols-1; i++ { // the last cell would scroll
		if i%2 == 0 {
			b = append(b, 0x12, petsciiColourCodes[(i/2)%16], 0xA6)
		} else {
			b = append(b, 0x92, petsciiColourCodes[(i/2+7)%16], 0xA6)
		}
	}
	return append(b, 0x13)
}

func TestPetsciiOutputIsBounded(t *testing.T) {
	busy := petsciiBusyScreen()
	tests := []struct {
		name  string
		setup []byte
		in    []byte
	}{
		{"INST and alternating reverse prints", busy, bytes.Repeat([]byte{0x94, 0x12, 0x1C, 0xA6, 0x9D, 0x94, 0x92, 0x05, 0xA6, 0x9D}, 3276)},
		{"DEL and alternating reverse prints", busy, bytes.Repeat([]byte{0x1D, 0x14, 0x12, 0x1C, 0xA6, 0x9D, 0x1D, 0x14, 0x92, 0x05, 0xA6, 0x9D}, 2730)},
		{"cursor right and DEL", busy, bytes.Repeat([]byte{0x1D, 0x14}, 16<<10)},
		{"INST in column 0", busy, bytes.Repeat([]byte{0x94}, 32<<10)},
		{"one charset switch", busy, []byte{0x8E}},
		{"charset toggles", busy, bytes.Repeat([]byte{0x8E, 0x0E}, 16<<10)},
		{"colour-alternating print", nil, bytes.Repeat([]byte{0x12, 0x1C, 0xA6, 0x92, 0x05, 0xA6}, 5<<10)},
		{"reverse print and scroll", nil, bytes.Repeat([]byte{0x12, 0x1C, 0xA6, 0x0D}, 8<<10)},
	}
	// No subtests: the first case over the bound stops the test before an
	// unbounded engine reaches the gigabyte-sized charset-toggle case.
	for _, tc := range tests {
		h := newPetsciiHarness(t)
		h.feed(string(tc.setup))
		out := h.c.Decode(nil, tc.in)
		if bound := petsciiOutputBound(len(tc.in)); len(out) > bound {
			t.Fatalf("%s: %d bytes in produced %d bytes out, bound %d", tc.name, len(tc.in), len(out), bound)
		}
		h.check(out)
		t.Logf("%s: %d bytes in, %d bytes out", tc.name, len(tc.in), len(out))
	}
}

func TestPetsciiDecodeDoesNotAllocate(t *testing.T) {
	c := newPetsciiCodec()
	dst := c.Decode(make([]byte, 0, 1<<16), petsciiSample)
	allocs := testing.AllocsPerRun(50, func() {
		dst = c.Decode(dst[:0], petsciiSample)
	})
	if allocs != 0 {
		t.Fatalf("Decode allocates %.1f times per call with a reused buffer, want 0", allocs)
	}
}

func BenchmarkPetsciiDecode(b *testing.B) {
	c := newPetsciiCodec()
	dst := make([]byte, 0, 1<<16)
	b.SetBytes(int64(len(petsciiSample)))
	b.ReportAllocs()
	for b.Loop() {
		dst = c.Decode(dst[:0], petsciiSample)
	}
}

// FuzzPetsciiDecode feeds arbitrary bytes in one call and in chunks of 1-16
// bytes: Decode must not panic, must keep the cursor inside 40x25, must emit
// valid UTF-8 in the engine's output alphabet within petsciiOutputBound, and
// the reference terminal must show the model after every call.
func FuzzPetsciiDecode(f *testing.F) {
	f.Add(petsciiSample, uint8(0))
	f.Add(petsciiSample, uint8(6))
	f.Add([]byte("\x93\x8e\xb0\xc0\xae\x0e\x14\x94\x1b[6n\x07\xff"), uint8(2))
	f.Add([]byte("\x1b[1;2;3;4;5;6;7;8;9\x1b\x1b[\x0d\x8d\x12\x92"), uint8(1))
	f.Add([]byte("\x1b[11111111111111111111A\x8e\x0e\x8e\x14\x94\x94"), uint8(15))
	f.Fuzz(func(t *testing.T, data []byte, chunk uint8) {
		whole := newPetsciiHarness(t)
		out := whole.c.Decode(nil, data)
		if bound := petsciiOutputBound(len(data)); len(out) > bound {
			t.Fatalf("%d bytes in produced %d bytes out, bound %d", len(data), len(out), bound)
		}
		whole.check(out)
		size := int(chunk%16) + 1
		h := newPetsciiHarness(t)
		for len(data) > 0 {
			n := min(size, len(data))
			out := h.c.Decode(nil, data[:n])
			if bound := petsciiOutputBound(n); len(out) > bound {
				t.Fatalf("%d bytes in produced %d bytes out, bound %d", n, len(out), bound)
			}
			h.check(out)
			data = data[n:]
			if s := &h.c.scr; s.row < 0 || s.row >= c64Rows || s.col < 0 || s.col >= c64Cols {
				t.Fatalf("cursor %d,%d outside 40x25", s.row, s.col)
			}
		}
		if h.c.scr != whole.c.scr || h.vt.visible() != whole.vt.visible() {
			t.Fatal("chunked decoding differs from one call")
		}
	})
}

// FuzzPetsciiEncode: Encode must not panic, must never emit more bytes than it
// read, and must emit only bytes of the spec's key list.
func FuzzPetsciiEncode(f *testing.F) {
	f.Add([]byte("Hello\r\x1b[A\x1bOP\x1b[64;5~\x1b[3~\U000000A3\x1b"))
	f.Add([]byte("\x1b[64;999~\x1b[\x1bO\x1b\xff"))
	producible := map[byte]bool{}
	for b := 0x20; b <= 0x5F; b++ {
		producible[byte(b)] = true // space to left arrow, lower-case letters
	}
	for b := 0xC1; b <= 0xDA; b++ {
		producible[byte(b)] = true // capitals
	}
	for b := 0x85; b <= 0x8C; b++ {
		producible[byte(b)] = true // F1-F8
	}
	for _, b := range []byte{0x03, 0x0D, 0x11, 0x12, 0x13, 0x14, 0x1D, 0x91, 0x92, 0x93, 0x94, 0x9D} {
		producible[b] = true // RUN/STOP, RETURN, cursor, RVS, HOME/CLR, DEL/INST
	}
	for _, b := range petsciiColourCodes {
		producible[b] = true
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		got := newPetsciiCodec().Encode(nil, data)
		if len(got) > len(data) {
			t.Fatalf("Encode(%q) = % X: more bytes than input", data, got)
		}
		for _, b := range got {
			if !producible[b] {
				t.Fatalf("Encode(%q) emitted 0x%02X, which no key produces", data, b)
			}
		}
	})
}
