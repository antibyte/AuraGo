package ui

import (
	"strings"
	"testing"
)

// The shop and evolution-choice screens draw menu panels over the frozen game
// canvas while fxDraw{Overlay,Confetti,PullLines,RumbleOverlay} keep rendering
// on top of them. Leftover gameplay FX (confetti, boss-enter/rumble rings,
// magnet pull-lines, warp streaks) must keep decaying in those states, or they
// appear as frozen "alien ship" artifacts over the menu panels.
func TestGalaxaMenuStatesDecayFX(t *testing.T) {
	t.Parallel()
	game := readEmbeddedText(t, "js/desktop/apps/galaxa-game.js")
	if !strings.Contains(game, "if (ctx.G.st === 'SHOP') { if (ctx.updateFX) ctx.updateFX(dt); ctx.updateShop(); return; }") {
		t.Fatal("SHOP state must tick updateFX so leftover gameplay FX decay")
	}
	if !strings.Contains(game, "if (ctx.G.evoChoiceOpen) { if (ctx.updateFX) ctx.updateFX(dt); ctx.updateEvoChoice(); return; }") {
		t.Fatal("evo-choice overlay must tick updateFX so leftover gameplay FX decay")
	}
}
