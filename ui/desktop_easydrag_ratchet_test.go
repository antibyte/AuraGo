package ui

import (
	"regexp"
	"strings"
	"testing"
)

// The ceilings only fall. A change that lowers a count lowers its ceiling in the same commit; a change that
// raises one is refused (use a --ed-radius-* token or a status/theme colour token instead of a literal).
const (
	easyDragRadiusLiteralCeiling = 79
	easyDragColourLiteralCeiling = 14
)

var (
	easyDragRadiusProp = regexp.MustCompile(`^border(-(top|bottom)-(left|right))?-radius$`)
	easyDragRadiusLit  = regexp.MustCompile(`(^|[^\w.-])(\d*\.)?[1-9]\d*(\.\d+)?((px|em|rem)\b|%)|(^|[^\w.-])0\.\d+(px|em|rem)\b`)
	easyDragColourLit  = regexp.MustCompile(`#[0-9a-fA-F]{3,8}\b|\b(?:rgba?|hsla?)\(|\b(?:white|black)\b`)
)

// easyDragRatchetCounts counts the literal radii and the literal colours outside the token definitions
// (every declaration of a --ed-* custom property is a token; the token bands are spread over four rules).
func easyDragRatchetCounts(css string) (radius, colour int) {
	for _, d := range easyDragCSSDeclarations(css) {
		if strings.HasPrefix(d.Prop, "--ed-") {
			continue
		}
		if easyDragRadiusProp.MatchString(d.Prop) && easyDragRadiusLit.MatchString(d.Value) {
			radius++
		}
		colour += len(easyDragColourLit.FindAllString(d.Value, -1))
	}
	return radius, colour
}

func TestEasyDragLiteralRatchets(t *testing.T) {
	css := strings.ReplaceAll(readDesktopAssetText(t, "css/desktop-app-easydrag.css"), "\r\n", "\n")
	radius, colour := easyDragRatchetCounts(css)
	check := func(name string, have, ceiling int, instead string) {
		switch {
		case have > ceiling:
			t.Errorf("%s: %d, ceiling %d: a new literal; use %s", name, have, ceiling, instead)
		case have < ceiling:
			t.Errorf("%s: %d, ceiling %d: lower the ceiling to %d in this commit", name, have, ceiling, have)
		}
	}
	check("literal border-radius declarations", radius, easyDragRadiusLiteralCeiling, "var(--ed-radius-sm|md|lg|pill) (a theme material radius keeps its var(--vd-mat-*-radius, var(--ed-radius-*)) form)")
	check("colour literals outside the --ed-* tokens", colour, easyDragColourLiteralCeiling, "a token (--ed-*) or a colour-mix() of tokens")
	// the counter must see the real stylesheet: guard against a scan that finds nothing
	if radius+colour == 0 {
		t.Fatal("the ratchet scan found no literal at all: easyDragCSSDeclarations is broken")
	}
}

// TestEasyDragRatchetCountsOnSamples pins the counting rules.
func TestEasyDragRatchetCountsOnSamples(t *testing.T) {
	r, c := easyDragRatchetCounts(`.a { border-radius: 8px; color: #fff; } .b { border-radius: var(--ed-radius-md); border-radius: 0; }
.c { --ed-x: #123456; border-top-left-radius: var(--vd-mat-btn-radius, 999px); background: color-mix(in srgb, var(--ed-ok) 12%, white); }
@media (pointer: coarse) { .d { border-radius: 50%; box-shadow: 0 0 0 1px rgba(0,0,0,.3), inset 0 0 black; } }`)
	if r != 3 || c != 4 {
		t.Fatalf("samples: radius %d (want 3: 8px, the 999px fallback, 50%%), colour %d (want 4: #fff, white, rgba(, black)", r, c)
	}
}
