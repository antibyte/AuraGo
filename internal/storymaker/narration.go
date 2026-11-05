package storymaker

// IsJSSpace reports the white space set of ECMAScript. Narration collapses it to U+0020 and the player's
// glyphs.needsFor skips it, so Go and JS must agree on it exactly.
func IsJSSpace(r rune) bool {
	switch {
	case r >= 0x0009 && r <= 0x000D, r >= 0x2000 && r <= 0x200A:
		return true
	case r == 0x0020, r == 0x00A0, r == 0x1680, r == 0x2028, r == 0x2029, r == 0x202F, r == 0x205F, r == 0x3000, r == 0xFEFF:
		return true
	}
	return false
}
