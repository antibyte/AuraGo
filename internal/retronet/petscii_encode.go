package retronet

import "unicode/utf8"

// Encode maps browser keyboard input to PETSCII. One call may hold several
// keys (a paste); escape sequences must be complete within the call. Unknown
// keys and sequences are dropped. The result is not IAC-escaped.
func (c *petsciiCodec) Encode(dst, src []byte) []byte {
	for len(src) > 0 {
		b := src[0]
		switch {
		case b == petsciiEsc:
			n, key, ok := petsciiKeySequence(src)
			src = src[n:]
			if ok {
				dst = append(dst, key)
			}
		case b < utf8.RuneSelf:
			src = src[1:]
			if key := petsciiASCIIKeys[b]; key != 0 {
				dst = append(dst, key)
			}
		default:
			r, size := utf8.DecodeRune(src)
			src = src[size:]
			if key, ok := petsciiRuneKey(r); ok {
				dst = append(dst, key)
			}
		}
	}
	return dst
}

// petsciiKeySequence decodes the key that starts with the ESC at src[0] and
// returns the number of bytes it consumed. A lone ESC and an ESC followed by
// another ESC are RUN/STOP; ESC followed by another key (an Alt chord) is
// dropped together with that key; incomplete, malformed and unknown
// sequences are dropped.
func petsciiKeySequence(src []byte) (n int, key byte, ok bool) {
	if len(src) == 1 || src[1] == petsciiEsc {
		return 1, petsciiRunStop, true
	}
	switch src[1] {
	case 'O':
		if len(src) < 3 {
			return len(src), 0, false
		}
		key, ok = petsciiKeySequences[string(src[:3])]
		return 3, key, ok
	case '[':
		n = 2
		for n < len(src) && src[n] >= 0x20 && src[n] <= 0x3F {
			n++
		}
		if n == len(src) || src[n] < 0x40 || src[n] > 0x7E {
			return n, 0, false
		}
		n++
		if key, ok = petsciiKeySequences[string(src[:n])]; ok {
			return n, key, true
		}
		key, ok = petsciiPrivateKey(src[:n])
		return n, key, ok
	default:
		_, size := utf8.DecodeRune(src[1:])
		return 1 + size, 0, false
	}
}

// petsciiPrivateKey decodes the browser's colour-key sequence ESC [ 6 4 ; n ~
// (n decimal, one to three digits) and allows only the bytes
// petsciiPrivateAllowed accepts.
func petsciiPrivateKey(seq []byte) (byte, bool) {
	if len(seq) < len(petsciiPrivatePrefix)+2 || string(seq[:len(petsciiPrivatePrefix)]) != petsciiPrivatePrefix || seq[len(seq)-1] != '~' {
		return 0, false
	}
	digits := seq[len(petsciiPrivatePrefix) : len(seq)-1]
	if len(digits) > 3 {
		return 0, false
	}
	n := 0
	for _, d := range digits {
		if d < '0' || d > '9' {
			return 0, false
		}
		n = n*10 + int(d-'0')
	}
	if n > 0xFF || !petsciiPrivateAllowed(byte(n)) {
		return 0, false
	}
	return byte(n), true
}
