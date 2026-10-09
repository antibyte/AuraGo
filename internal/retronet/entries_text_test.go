package retronet

import (
	"fmt"
	"testing"
	"unicode/utf16"
)

// The escapes below are built from code points on purpose: invisible and
// bidirectional characters must not sit in the test source as literals.

// zwj is U+200D ZERO WIDTH JOINER, the one invisible rune names may contain.
const zwj rune = 0x200d

// invisibleRunes covers the rejected categories Cc, Cf, Co, Zl, Zp and U+FFFD.
var invisibleRunes = []rune{
	0x0000, 0x0007, 0x001b, 0x007f, 0x0085, 0x009b, // Cc
	0x00ad, 0x061c, 0x200b, 0x200c, 0x200e, 0x200f, // Cf: soft hyphen, ALM, ZWSP, ZWNJ, LRM, RLM
	0x202a, 0x202b, 0x202c, 0x202d, 0x202e, // Cf: bidi embeddings and overrides
	0x2060, 0x2066, 0x2067, 0x2068, 0x2069, 0xfeff, // Cf: word joiner, isolates, BOM
	0xe0001, 0xe0020, 0xe0041, 0xe007f, // Cf: tag characters
	0x2028, 0x2029, // Zl, Zp
	0xe000, 0xf8ff, 0xf0000, 0x10fffd, // Co: private use
	0xfffd, // replacement character
}

// jsonEscapeRune returns the JSON escape of r, as a surrogate pair above U+FFFF.
func jsonEscapeRune(r rune) string {
	if r > 0xffff {
		hi, lo := utf16.EncodeRune(r)
		return jsonEscape(hi) + jsonEscape(lo)
	}
	return jsonEscape(r)
}

// textForms renders r once as the character itself and once as a JSON escape.
var textForms = []struct {
	name   string
	render func(rune) string
	ok     func(rune) bool // the form is valid JSON for r
}{
	{"raw", func(r rune) string { return string(r) }, func(r rune) bool { return r >= 0x20 }},
	{"escaped", jsonEscapeRune, func(rune) bool { return true }},
}

func TestValidateEntriesDocumentRejectsInvisibleText(t *testing.T) {
	for _, r := range invisibleRunes {
		for _, field := range []string{"name", "description"} {
			for _, form := range textForms {
				if !form.ok(r) {
					continue
				}
				t.Run(fmt.Sprintf("%s U+%04X %s", field, r, form.name), func(t *testing.T) {
					for _, text := range []string{"My" + form.render(r) + "BBS", form.render(r) + "BBS", "BBS" + form.render(r)} {
						if err := ValidateEntriesDocument(entriesDoc(telnetJSON(field, `"`+text+`"`))); err == nil {
							t.Fatalf("accepted %s %q", field, text)
						}
					}
				})
			}
		}
	}
}

func TestValidateEntriesDocumentRejectsLoneSurrogateEscapes(t *testing.T) {
	// encoding/json turns these into U+FFFD, which is rejected like the raw character.
	escapes := map[string]string{
		"high":          jsonEscape(0xd800),
		"low":           jsonEscape(0xdc00),
		"high then BMP": jsonEscape(0xd800) + "x",
		"reversed pair": jsonEscape(0xdc00) + jsonEscape(0xd800),
		"max high":      jsonEscape(0xdbff),
	}
	for name, escape := range escapes {
		for _, field := range []string{"name", "description"} {
			t.Run(field+" "+name, func(t *testing.T) {
				if err := ValidateEntriesDocument(entriesDoc(telnetJSON(field, `"My`+escape+`BBS"`))); err == nil {
					t.Fatalf("accepted a lone surrogate escape in %s", field)
				}
			})
		}
	}
}

func TestValidateEntriesDocumentRejectsBlankNames(t *testing.T) {
	joiner := string(zwj)
	blank := map[string]string{
		"empty":                "",
		"spaces":               "   ",
		"joiner":               joiner,
		"joiners":              joiner + joiner + joiner,
		"spaces and joiners":   " " + joiner + " " + joiner,
		"joiner between space": " " + joiner + " ",
	}
	for name, text := range blank {
		t.Run(name, func(t *testing.T) {
			if err := ValidateEntriesDocument(entriesDoc(telnetJSON("name", `"`+text+`"`))); err == nil {
				t.Fatalf("accepted the blank name %q", text)
			}
			if err := ValidateEntriesDocument(entriesDoc(telnetJSON("name", `"`+jsonEscape(zwj)+text+`"`))); err == nil {
				t.Fatalf("accepted the blank name %q behind an escaped joiner", text)
			}
		})
	}
}

func TestValidateEntriesDocumentAcceptsJoinerSequences(t *testing.T) {
	technologist := string([]rune{0x1f469, zwj, 0x1f4bb})
	family := string([]rune{0x1f468, zwj, 0x1f469, zwj, 0x1f467})
	texts := map[string]string{
		"emoji with text":     technologist + " BBS",
		"emoji sequence only": family,
		"joiner in text":      "Zero" + string(zwj) + "Cool",
	}
	for name, text := range texts {
		t.Run(name, func(t *testing.T) {
			doc := entriesDoc(telnetJSON("name", `"`+text+`"`))
			entries, err := ParseEntriesDocument(doc)
			if err != nil {
				t.Fatalf("ParseEntriesDocument() error = %v", err)
			}
			if entries[0].Name != text {
				t.Fatalf("name = %q, want %q", entries[0].Name, text)
			}
			if err := ValidateEntriesDocument(entriesDoc(telnetJSON("description", `"`+text+`"`))); err != nil {
				t.Fatalf("description with %q: %v", text, err)
			}
			if err := ValidateEntriesDocument(entriesDoc(telnetJSON("name", `"`+jsonEscape(zwj)+"x"+`"`))); err != nil {
				t.Fatalf("escaped joiner: %v", err)
			}
		})
	}
}
