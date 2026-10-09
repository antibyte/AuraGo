package retronet

import (
	"bytes"
	"testing"
	"unicode/utf8"
)

// cp437Glyphs is an independent oracle for CP437 bytes 0x80-0xFF, written as
// glyphs (16 per line) instead of code points.
const cp437Glyphs = "ÇüéâäàåçêëèïîìÄÅ" +
	"ÉæÆôöòûùÿÖÜ¢£¥₧ƒ" +
	"áíóúñÑªº¿⌐¬½¼¡«»" +
	"░▒▓│┤╡╢╖╕╣║╗╝╜╛┐" +
	"└┴┬├─┼╞╟╚╔╩╦╠═╬╧" +
	"╨╤╥╙╘╒╓╫╪┘┌█▄▌▐▀" +
	"αßΓπΣσµτΦΘΩδ∞φε∩" +
	"≡±≥≤⌠⌡÷≈°∙·√ⁿ²■ "

func TestCP437DecodeMatchesGlyphOracle(t *testing.T) {
	glyphs := []rune(cp437Glyphs)
	if len(glyphs) != 128 {
		t.Fatalf("oracle has %d glyphs, want 128", len(glyphs))
	}
	codec := NewCodec(CharsetCP437)
	for i, want := range glyphs {
		b := byte(0x80 + i)
		if got := codec.Decode(nil, []byte{b}); string(got) != string(want) {
			t.Errorf("Decode(0x%02X) = %q, want %q", b, got, string(want))
		}
	}
	if got := codec.Decode(nil, []byte{0x7F}); string(got) != "⌂" {
		t.Errorf("Decode(0x7F) = %q, want %q", got, "⌂")
	}
}

func TestCP437PassesControlsAndASCII(t *testing.T) {
	codec := NewCodec(CharsetCP437)
	for b := 0; b < 0x7F; b++ {
		if got := codec.Decode(nil, []byte{byte(b)}); !bytes.Equal(got, []byte{byte(b)}) {
			t.Errorf("Decode(0x%02X) = % X, want unchanged", b, got)
		}
	}
	ansi := []byte("\x1b[1;33mHello\x1b[0m\r\n\a\b\t")
	if got := codec.Decode(nil, ansi); !bytes.Equal(got, ansi) {
		t.Fatalf("Decode(ANSI) = %q, want %q", got, ansi)
	}
}

func TestCP437RoundTripAllBytes(t *testing.T) {
	codec := NewCodec(CharsetCP437)
	seen := make(map[string]int, 256)
	for b := 0; b < 256; b++ {
		decoded := codec.Decode(nil, []byte{byte(b)})
		if !utf8.Valid(decoded) {
			t.Fatalf("Decode(0x%02X) = % X is not valid UTF-8", b, decoded)
		}
		if prev, dup := seen[string(decoded)]; dup {
			t.Fatalf("bytes 0x%02X and 0x%02X both decode to %q", prev, b, decoded)
		}
		seen[string(decoded)] = b
		if got := codec.Encode(nil, decoded); !bytes.Equal(got, []byte{byte(b)}) {
			t.Errorf("Encode(Decode(0x%02X)) = % X, want %02X", b, got, b)
		}
	}
	all := make([]byte, 256)
	for i := range all {
		all[i] = byte(i)
	}
	if got := codec.Encode(nil, codec.Decode(nil, all)); !bytes.Equal(got, all) {
		t.Fatalf("whole-buffer round trip = % X, want all 256 bytes in order", got)
	}
}

func TestLatin1Decode(t *testing.T) {
	codec := NewCodec(CharsetLatin1)
	tests := []struct {
		name string
		in   []byte
		want string
	}{
		{"ascii and ansi", []byte("\x1b[31mred\x1b[0m\r\n"), "\x1b[31mred\x1b[0m\r\n"},
		{"amiga csi", []byte{0x9B, '3', '1', 'm', 'x'}, "\x1b[31mx"},
		{"other c1 dropped", []byte{'a', 0x80, 0x85, 0x9A, 0x9C, 0x9F, 'b'}, "ab"},
		{"nbsp and umlauts", []byte{0xA0, 0xE4, 0xF6, 0xFC, 0xDF}, " äöüß"},
		{"last byte", []byte{0xFF}, "ÿ"},
		{"del passes", []byte{0x7F}, "\x7f"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := codec.Decode(nil, tc.in); string(got) != tc.want {
				t.Fatalf("Decode(% X) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestLatin1RoundTrip(t *testing.T) {
	codec := NewCodec(CharsetLatin1)
	for b := 0; b < 256; b++ {
		if b >= 0x80 && b < 0xA0 {
			continue // C1 bytes are dropped (0x9B becomes ESC [) and cannot round-trip
		}
		decoded := codec.Decode(nil, []byte{byte(b)})
		if got := codec.Encode(nil, decoded); !bytes.Equal(got, []byte{byte(b)}) {
			t.Errorf("Encode(Decode(0x%02X)) = % X, want %02X", b, got, b)
		}
	}
}

func TestEncodeUnmappableAndInvalidUTF8(t *testing.T) {
	tests := []struct {
		name    string
		charset Charset
		in      []byte
		want    []byte
	}{
		{"cp437 umlauts", CharsetCP437, []byte("Grüße"), []byte{'G', 'r', 0x81, 0xE1, 'e'}},
		{"cp437 box drawing", CharsetCP437, []byte("╔═╗"), []byte{0xC9, 0xCD, 0xBB}},
		{"cp437 euro unmappable", CharsetCP437, []byte("5€"), []byte("5?")},
		{"cp437 emoji unmappable", CharsetCP437, []byte("a😀b"), []byte("a?b")},
		{"cp437 invalid utf8", CharsetCP437, []byte{'a', 0xFF, 'b'}, []byte("a?b")},
		{"cp437 control keys pass", CharsetCP437, []byte("\x1b[A\x7f\r\x03"), []byte("\x1b[A\x7f\r\x03")},
		{"latin1 umlauts", CharsetLatin1, []byte("Grüße"), []byte{'G', 'r', 0xFC, 0xDF, 'e'}},
		{"latin1 euro unmappable", CharsetLatin1, []byte("€"), []byte("?")},
		{"latin1 c1 rune unmappable", CharsetLatin1, []byte("\u0085"), []byte("?")},
		{"latin1 lone lead byte", CharsetLatin1, []byte{0xC3}, []byte("?")},
		{"latin1 truncated sequence", CharsetLatin1, []byte{0xE2, 0x82}, []byte("??")},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := NewCodec(tc.charset).Encode(nil, tc.in); !bytes.Equal(got, tc.want) {
				t.Fatalf("Encode(%q) = % X, want % X", tc.in, got, tc.want)
			}
		})
	}
}

func TestUTF8CodecPassesThrough(t *testing.T) {
	in := []byte("café \x1b[0m \xff \x9b")
	for _, c := range []Charset{CharsetUTF8, "", "unknown"} {
		codec := NewCodec(c)
		if got := codec.Decode(nil, in); !bytes.Equal(got, in) {
			t.Errorf("%q Decode = % X, want unchanged", c, got)
		}
		if got := codec.Encode(nil, in); !bytes.Equal(got, in) {
			t.Errorf("%q Encode = % X, want unchanged", c, got)
		}
	}
}

func TestCodecsAppendToDst(t *testing.T) {
	tests := []struct {
		charset Charset
		decoded string
		encoded string
	}{
		{CharsetCP437, "preÇ", "pre\x80"},
		{CharsetLatin1, "preä", "pre\xe4"},
		{CharsetUTF8, "preä", "preä"},
	}
	for _, tc := range tests {
		codec := NewCodec(tc.charset)
		src := []byte(tc.encoded[3:])
		if got := codec.Decode([]byte("pre"), src); string(got) != tc.decoded {
			t.Errorf("%s Decode = %q, want %q", tc.charset, got, tc.decoded)
		}
		if got := codec.Encode([]byte("pre"), []byte(tc.decoded[3:])); string(got) != tc.encoded {
			t.Errorf("%s Encode = %q, want %q", tc.charset, got, tc.encoded)
		}
	}
}
