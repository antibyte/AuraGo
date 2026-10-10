package cydcreator

import (
	"fmt"
	"strconv"
	"strings"
	"testing"
)

func TestCHexColor(t *testing.T) {
	if got := cHexColor("#3fc6e0"); got != "lv_color_hex(0x3FC6E0)" {
		t.Errorf("cHexColor = %s", got)
	}
	if cBool(true) != "1" || cBool(false) != "0" {
		t.Error("cBool")
	}
}

func TestCString(t *testing.T) {
	cases := map[string]string{
		"":                        `""`,
		"Start":                   `"Start"`,
		`say "hi"`:                `"say \"hi\""`,
		`C:\temp`:                 `"C:\\temp"`,
		"two\nlines":              `"two\nlines"`,
		"tab\there":               `"tab\011here"`,
		"cr\r\x01\x1f\x7f":        `"cr\015\001\037\177"`,
		"\x01" + "7":              `"\0017"`, // three octal digits: the 7 stays a character
		"why??=":                  `"why?\?="`,
		"???":                     `"?\?\?"`,
		"22,5 °C – Zurück € ✓ 你好": `"22,5 °C – Zurück € ✓ 你好"`, // UTF-8 stays raw
		"100 %":                   `"100 %"`,
	}
	for in, want := range cases {
		if got := cString(in); got != want {
			t.Errorf("cString(%q) = %s, want %s", in, got, want)
		}
	}
}

// TestCStringMatchesTemplateEscaper pins that the generator and the device
// strings header (plan 1b, cStringLiteral) escape control bytes and trigraphs
// identically: the 1b reference must stay reproducible byte for byte, and the
// two escapers must never drift apart.
func TestCStringMatchesTemplateEscaper(t *testing.T) {
	var inputs []string
	for c := 0; c < 0x80; c++ {
		inputs = append(inputs, string(rune(c)), "a"+string(rune(c))+"7")
	}
	inputs = append(inputs, "??", "???", "????", "?a?", "??=", "??/", "??'", "??(", "??)", "??!", "??<", "??>", "??-",
		"What?? Really???", "ü??ü", "\\??", "\"??\"", "\n??\n", "22,5 °C – Zurück € ✓ 你好")
	for _, in := range inputs {
		if got, want := cString(in), cStringLiteral(in); got != want {
			t.Errorf("cString(%q) = %s, cStringLiteral = %s", in, got, want)
		}
	}
}

// TestCStringControlBytesAndTrigraphs checks the two properties generated
// sources rely on, for every control byte and every trigraph introducer.
func TestCStringControlBytesAndTrigraphs(t *testing.T) {
	for c := 0; c < 0x20; c++ {
		in := string(rune(c))
		want := fmt.Sprintf(`"\%03o"`, c)
		if c == '\n' {
			want = `"\n"`
		}
		if got := cString(in); got != want {
			t.Errorf("cString(%#x) = %s, want %s", c, got, want)
		}
		// Octal escapes carry three digits, so a following digit stays literal.
		if got := cString(in + "7"); got != strings.TrimSuffix(want, `"`)+`7"` {
			t.Errorf("cString(%#x+7) = %s", c, got)
		}
	}
	if got := cString("\x7f"); got != `"\177"` {
		t.Errorf("cString(DEL) = %s", got)
	}
	for _, third := range "=/'()!<>-" {
		in := "??" + string(third)
		got := cString(in)
		if strings.Contains(got, "??") {
			t.Errorf("cString(%q) = %s still contains a trigraph", in, got)
		}
		if got != `"?\?`+string(third)+`"` {
			t.Errorf("cString(%q) = %s", in, got)
		}
	}
	for n := 2; n <= 6; n++ {
		if got := cString(strings.Repeat("?", n)); strings.Contains(got, "??") {
			t.Errorf("cString(%d question marks) = %s still contains a trigraph", n, got)
		}
	}
}

func TestCComment(t *testing.T) {
	cases := map[string]string{
		"Sets the backlight.":           "Sets the backlight.",
		"line one\nline two\r\tend":     "line one line two  end",
		`ends with a backslash \`:       "ends with a backslash",
		"trailing spaces and \\ \\   ":  "trailing spaces and",
		"no trigraph ??/ here ???":      "no trigraph ? ?/ here ? ? ?",
		"lädt das Wetter neu \u2028 ok": "lädt das Wetter neu   ok",
		"":                              "",
	}
	for in, want := range cases {
		if got := cComment(in); got != want {
			t.Errorf("cComment(%q) = %q, want %q", in, got, want)
		}
	}
}

// firstDiff shows the first differing line of two texts.
func firstDiff(want, got string) string {
	wl, gl := strings.Split(want, "\n"), strings.Split(got, "\n")
	for i := 0; i < max(len(wl), len(gl)); i++ {
		var w, g string
		if i < len(wl) {
			w = wl[i]
		}
		if i < len(gl) {
			g = gl[i]
		}
		if w != g {
			return "line " + strconv.Itoa(i+1) + "\n want: " + w + "\n  got: " + g
		}
	}
	return "(same lines, different line endings or trailing bytes)"
}
