package dockerutil

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestEngineErrorMessage(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
	}{
		{"engine JSON message", `{"message":"No such container: x"}` + "\n", "No such container: x"},
		{"message keeps its own text", `{"message":"  padded\n"}`, "  padded\n"},
		{"blank message falls back to the body", `{"message":"   "}`, `{"message":"   "}`},
		{"zero-width message falls back to the body", `{"message":"\u200b"}`, `{"message":"\u200b"}`},
		{"control-only message falls back to the body", `{"message":"\u001b \t"}`, `{"message":"\u001b \t"}`},
		{"JSON without message", `{"error":"x"}`, `{"error":"x"}`},
		{"HTML from a socket proxy", "<html><body><h1>403 Forbidden</h1>\n</body></html>\n", "<html><body><h1>403 Forbidden</h1>\n</body></html>\n"},
		{"empty", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := EngineErrorMessage([]byte(tc.body)); got != tc.want {
				t.Fatalf("EngineErrorMessage() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestSanitizeOneLine(t *testing.T) {
	cases := []struct {
		name     string
		text     string
		maxBytes int
		want     string
	}{
		{"printable text unchanged", "manifest unknown", 256, "manifest unknown"},
		{"trimmed", "  bad gateway \n", 256, "bad gateway"},
		{"line breaks and controls become spaces", "a\nb\r\nc\td\x1b[31me", 0, "a b  c d [31me"},
		{"unicode separators, bidi and zero width", "denied\u2028token\u2029x\u202ey\u200bz", 0, "denied token x y z"},
		{"cut at maxBytes", strings.Repeat("x", 600), 512, strings.Repeat("x", 512)},
		{"never splits a rune", strings.Repeat("x", 255) + "é tail", 256, strings.Repeat("x", 255)},
		{"no bound", strings.Repeat("x", 600), 0, strings.Repeat("x", 600)},
		{"only non-printable", "\u0001\u001b\u202e\u200b", 256, ""},
		{"invalid UTF-8 becomes U+FFFD", "a\xffb", 0, "a�b"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := SanitizeOneLine(tc.text, tc.maxBytes)
			if got != tc.want {
				t.Fatalf("SanitizeOneLine() = %q, want %q", got, tc.want)
			}
			if !utf8.ValidString(got) {
				t.Fatalf("SanitizeOneLine() = %q is not valid UTF-8", got)
			}
		})
	}
}
