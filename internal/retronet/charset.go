package retronet

import "unicode/utf8"

// Codec converts between a service charset and the UTF-8 the browser uses.
type Codec interface {
	// Decode converts server bytes to UTF-8, appending to dst.
	Decode(dst, src []byte) []byte
	// Encode converts UTF-8 keyboard input to the server charset, appending to dst.
	Encode(dst, src []byte) []byte
}

// NewCodec returns the codec for c. Empty and unknown charsets get the UTF-8
// pass-through codec.
func NewCodec(c Charset) Codec {
	switch c {
	case CharsetCP437:
		return cp437Codec{}
	case CharsetLatin1:
		return latin1Codec{}
	default:
		return utf8Codec{}
	}
}

// unmappable replaces runes the server charset cannot represent and invalid
// UTF-8 input bytes.
const unmappable = '?'

// utf8Codec passes bytes through unchanged in both directions.
type utf8Codec struct{}

func (utf8Codec) Decode(dst, src []byte) []byte { return append(dst, src...) }

func (utf8Codec) Encode(dst, src []byte) []byte { return append(dst, src...) }

// cp437Codec implements IBM code page 437. Bytes 0x00-0x7E pass unchanged so
// C0 controls (ANSI escape sequences) survive; 0x7F-0xFF map to the glyphs.
type cp437Codec struct{}

func (cp437Codec) Decode(dst, src []byte) []byte {
	for _, b := range src {
		if b < 0x7F {
			dst = append(dst, b)
			continue
		}
		dst = utf8.AppendRune(dst, cp437High[b-0x7F])
	}
	return dst
}

func (cp437Codec) Encode(dst, src []byte) []byte {
	return encodeRunes(dst, src, func(r rune) (byte, bool) {
		b, ok := cp437Reverse[r]
		return b, ok
	})
}

// latin1Codec implements ISO 8859-1 for MUDs and Amiga boards: 0xA0-0xFF map
// to U+00A0-U+00FF, the Amiga CSI 0x9B becomes "ESC [" and every other C1
// byte (0x80-0x9F) is dropped.
type latin1Codec struct{}

func (latin1Codec) Decode(dst, src []byte) []byte {
	for _, b := range src {
		switch {
		case b < 0x80:
			dst = append(dst, b)
		case b == 0x9B:
			dst = append(dst, 0x1B, '[')
		case b < 0xA0:
			// Other C1 controls have no meaning for the browser terminal.
		default:
			dst = utf8.AppendRune(dst, rune(b))
		}
	}
	return dst
}

func (latin1Codec) Encode(dst, src []byte) []byte {
	return encodeRunes(dst, src, func(r rune) (byte, bool) {
		if r >= 0xA0 && r <= 0xFF {
			return byte(r), true
		}
		return 0, false
	})
}

// encodeRunes passes ASCII through, maps every other rune with lookup and
// writes '?' for unmappable runes and for each invalid UTF-8 byte. Input is
// expected to hold complete UTF-8 sequences per call (the browser sends whole
// strings).
func encodeRunes(dst, src []byte, lookup func(rune) (byte, bool)) []byte {
	for len(src) > 0 {
		if src[0] < utf8.RuneSelf {
			dst = append(dst, src[0])
			src = src[1:]
			continue
		}
		r, size := utf8.DecodeRune(src)
		src = src[size:]
		if b, ok := lookup(r); ok {
			dst = append(dst, b)
		} else {
			dst = append(dst, unmappable)
		}
	}
	return dst
}

// cp437High maps CP437 bytes 0x7F-0xFF (index b-0x7F) to Unicode. 0x7F is the
// house glyph; 0x80-0xFF follow the IBM PC character set.
var cp437High = [129]rune{
	0x2302, // 0x7F

	0x00C7, 0x00FC, 0x00E9, 0x00E2, 0x00E4, 0x00E0, 0x00E5, 0x00E7, // 0x80
	0x00EA, 0x00EB, 0x00E8, 0x00EF, 0x00EE, 0x00EC, 0x00C4, 0x00C5, // 0x88
	0x00C9, 0x00E6, 0x00C6, 0x00F4, 0x00F6, 0x00F2, 0x00FB, 0x00F9, // 0x90
	0x00FF, 0x00D6, 0x00DC, 0x00A2, 0x00A3, 0x00A5, 0x20A7, 0x0192, // 0x98
	0x00E1, 0x00ED, 0x00F3, 0x00FA, 0x00F1, 0x00D1, 0x00AA, 0x00BA, // 0xA0
	0x00BF, 0x2310, 0x00AC, 0x00BD, 0x00BC, 0x00A1, 0x00AB, 0x00BB, // 0xA8
	0x2591, 0x2592, 0x2593, 0x2502, 0x2524, 0x2561, 0x2562, 0x2556, // 0xB0
	0x2555, 0x2563, 0x2551, 0x2557, 0x255D, 0x255C, 0x255B, 0x2510, // 0xB8
	0x2514, 0x2534, 0x252C, 0x251C, 0x2500, 0x253C, 0x255E, 0x255F, // 0xC0
	0x255A, 0x2554, 0x2569, 0x2566, 0x2560, 0x2550, 0x256C, 0x2567, // 0xC8
	0x2568, 0x2564, 0x2565, 0x2559, 0x2558, 0x2552, 0x2553, 0x256B, // 0xD0
	0x256A, 0x2518, 0x250C, 0x2588, 0x2584, 0x258C, 0x2590, 0x2580, // 0xD8
	0x03B1, 0x00DF, 0x0393, 0x03C0, 0x03A3, 0x03C3, 0x00B5, 0x03C4, // 0xE0
	0x03A6, 0x0398, 0x03A9, 0x03B4, 0x221E, 0x03C6, 0x03B5, 0x2229, // 0xE8
	0x2261, 0x00B1, 0x2265, 0x2264, 0x2320, 0x2321, 0x00F7, 0x2248, // 0xF0
	0x00B0, 0x2219, 0x00B7, 0x221A, 0x207F, 0x00B2, 0x25A0, 0x00A0, // 0xF8
}

// cp437Reverse is the encode direction of cp437High.
var cp437Reverse = func() map[rune]byte {
	m := make(map[rune]byte, len(cp437High))
	for i, r := range cp437High {
		m[r] = byte(0x7F + i)
	}
	return m
}()
