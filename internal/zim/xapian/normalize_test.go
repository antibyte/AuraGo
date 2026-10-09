package xapian

import "testing"

func TestNormalizeGolden(t *testing.T) {
	var g goldenAnalysis
	loadJSON(t, "analysis.json", &g)
	for _, c := range g.Normalize {
		if got := Normalize(c[0]); got != c[1] {
			t.Errorf("Normalize(%q) = %q, want %q", c[0], got, c[1])
		}
	}
}

// libzim strips every combining mark (Mn, Mc and Me), e.g. Devanagari vowel
// signs (Mc) and the virama (Mn), not only non-spacing marks.
func TestNormalizeStripsAllMarks(t *testing.T) {
	cases := map[string]string{
		"हिन्दी": "हनद",  // ि U+093F Mc, ् U+094D Mn, ी U+0940 Mc
		"नमस्ते": "नमसत", // ् Mn, े U+0947 Mn
		"a⃝":     "a",    // U+20DD COMBINING ENCLOSING CIRCLE (Me)
		"Café":   "cafe",
	}
	for in, want := range cases {
		if got := Normalize(in); got != want {
			t.Errorf("Normalize(%q) = %q, want %q", in, got, want)
		}
	}
}
