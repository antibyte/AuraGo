package ui

import (
	"math"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// W1a A (FD-08): the status disc colours. The four coloured status tokens keep the hue and OKLab distance checks of
// TestEasyDragCategoryColourTokens. The grey token (--ed-status-neutral) is checked for contrast only: the logic and tool
// tiles are grey by design and grey statuses carry their meaning through the glyph, so no mix could keep 0.20 OKLab from them.

var easyDragStatusNeutralMix = regexp.MustCompile(`^color-mix\(in srgb, var\(--ed-text\) (\d+)%, var\(--ed-panel-strong\)\)$`)

func TestEasyDragStatusColourTokens(t *testing.T) {
	sheets := map[string]cssSheet{}
	ed := easyDragS0Sheet(t, sheets, "css/desktop-app-easydrag.css")
	root := easyDragS0RuleDecls(ed, easyDragS0DarkBand)
	match := easyDragStatusNeutralMix.FindStringSubmatch(root["--ed-status-neutral"])
	if match == nil {
		t.Fatalf(".ed-app --ed-status-neutral = %q, want color-mix(in srgb, var(--ed-text) N%%, var(--ed-panel-strong))", root["--ed-status-neutral"])
	}
	percent, _ := strconv.ParseFloat(match[1], 64)
	for token, want := range map[string]string{"--ed-status-neutral-ink": "var(--ed-panel-strong)", "--ed-status-edge": "var(--ed-status-neutral)"} {
		if root[token] != want {
			t.Errorf(".ed-app %s = %q, want %s", token, root[token], want)
		}
	}
	if disc := easyDragS0RuleDecls(ed, ".ed-status-disc"); !strings.Contains(disc["box-shadow"], "var(--ed-status-edge)") {
		t.Errorf(".ed-status-disc box-shadow = %q, want a ring in var(--ed-status-edge): the coloured discs fail 3:1 on light surfaces without it", disc["box-shadow"])
	}
	worstGrey, worstGlyph, worstInk := math.MaxFloat64, math.MaxFloat64, math.MaxFloat64
	for _, ctx := range easyDragS0ThemeContexts(t) {
		surfaces := easyDragS0Surfaces(t, sheets, ctx)
		text := easyDragS0HexToken(t, sheets, ctx, "--vd-text")
		card := surfaces["card"]
		var neutral [3]float64
		for i := range neutral {
			neutral[i] = math.Round(text[i]*percent/100 + card[i]*(100-percent)/100)
		}
		// the grey disc and its ring reach 3:1 on the canvas, card and well; its glyph (the panel colour) reaches 3:1 on it
		ratio, surface := easyDragS0MinContrast(neutral, surfaces)
		worstGrey = math.Min(worstGrey, ratio)
		if ratio < 3 {
			t.Errorf("%s: the grey status disc %v is %.2f:1 on the %s, want at least 3:1", ctx.Name, neutral, ratio, surface)
		}
		glyph := cssContrastRatio(card, neutral)
		worstGlyph = math.Min(worstGlyph, glyph)
		if glyph < 3 {
			t.Errorf("%s: the glyph on the grey disc is %.2f:1, want at least 3:1", ctx.Name, glyph)
		}
		// the glyph ink on the four coloured discs reaches 4.5:1 (this context's ink: dark by default, white where a band block says so)
		ink, ok := parseCSSHexColor(easyDragS0EdDecls(ed, ctx)["--ed-status-ink"])
		if !ok {
			t.Errorf("%s: --ed-status-ink is not a six-digit hex colour", ctx.Name)
			continue
		}
		for _, token := range easyDragS0StatusTokens {
			inkRatio := cssContrastRatio(ink, easyDragS0HexToken(t, sheets, ctx, token))
			worstInk = math.Min(worstInk, inkRatio)
			if inkRatio < 4.5 {
				t.Errorf("%s: the glyph ink on %s is %.2f:1, want at least 4.5:1", ctx.Name, token, inkRatio)
			}
		}
	}
	t.Logf("worst grey disc on a surface %.2f:1, worst glyph on the grey %.2f:1, worst ink on a coloured disc %.2f:1", worstGrey, worstGlyph, worstInk)
}
