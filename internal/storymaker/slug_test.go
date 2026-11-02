package storymaker

import (
	"strings"
	"testing"
)

func TestSlugify(t *testing.T) {
	const id = "00000000-0000-4000-8000-000000000001"
	cases := []struct{ title, want string }{
		{"Was ist eine Firewall?", "Was-ist-eine-Firewall"},
		{"Größe & Übung: Äpfel, Öl, Straße", "Groesse-Uebung-Aepfel-Oel-Strasse"},
		{"ÄÖÜ äöü ß", "AeOeUe-aeoeue-ss"},
		{"  --Hallo   Welt--  ", "Hallo-Welt"},
		{"Café Crème brûlée, señor", "Cafe-Creme-brulee-senor"},
		{"Zażółć gęślą jaźń", "Zazolc-gesla-jazn"},
		{"a/b\\c:d*e?f\"g<h>i|j", "a-b-c-d-e-f-g-h-i-j"},
		{"../../etc/passwd", "etc-passwd"},
		{"v1.2 final.mp4", "v1-2-final-mp4"},
		{"日本語のタイトル", id},
		{"???", id},
		{"", id},
		// Windows reserved device names fall back (any case), their look-alikes stay.
		{"CON", id}, {"nul", id}, {"Aux", id}, {"PRN", id}, {"com1", id}, {"LPT9", id},
		{"COM0", "COM0"}, {"console", "console"}, {"CON-1", "CON-1"}, {"COM10", "COM10"},
	}
	for _, tc := range cases {
		if got := Slugify(tc.title, id); got != tc.want {
			t.Errorf("Slugify(%q) = %q, want %q", tc.title, got, tc.want)
		}
	}
	if got := Slugify("???", ""); got != "storymaker" {
		t.Errorf("empty fallback: %q", got)
	}
	long := Slugify(strings.Repeat("abcdefghi ", 20), id)
	if len(long) > 80 || strings.HasSuffix(long, "-") || !strings.HasPrefix(long, "abcdefghi-abcdefghi") {
		t.Errorf("long title: %q (%d)", long, len(long))
	}
	// Whatever the input, the result is a plain file-name stem.
	for _, in := range []string{"x\x00y", "a\nb", "tab\there", "emoji " + string(rune(0x1F600)) + " title", string(rune(0x202e)) + "evil"} {
		got := Slugify(in, id)
		for _, r := range got {
			if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-') {
				t.Errorf("Slugify(%q) = %q has %q", in, got, r)
			}
		}
	}
}

func TestValidProjectID(t *testing.T) {
	ok := []string{"00000000-0000-4000-8000-000000000001", "9b2f1c3e-7d44-4a5b-8c1d-0123456789ab"}
	bad := []string{"", "x", "9B2F1C3E-7D44-4A5B-8C1D-0123456789AB", "9b2f1c3e-7d44-1a5b-8c1d-0123456789ab", "9b2f1c3e-7d44-4a5b-cc1d-0123456789ab",
		"9b2f1c3e7d444a5b8c1d0123456789ab", "../9b2f1c3e-7d44-4a5b-8c1d-0123456789ab", "9b2f1c3e-7d44-4a5b-8c1d-0123456789ab/",
		"9b2f1c3e-7d44-4a5b-8c1d-0123456789ab\n"}
	for _, id := range ok {
		if !ValidProjectID(id) {
			t.Errorf("%q rejected", id)
		}
	}
	for _, id := range bad {
		if ValidProjectID(id) {
			t.Errorf("%q accepted", id)
		}
	}
}
