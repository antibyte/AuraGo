package ui

import (
	"strings"
	"testing"
)

func TestGalaxaJuiceCombatSFXMarkers(t *testing.T) {
	t.Parallel()
	sfx := readEmbeddedText(t, "js/desktop/apps/galaxa-audio-sfx.js")
	for _, marker := range []string{
		"shootTyped(kind, panX)",
		"playerHurt(panX)",
		"enemyHitSfx(type, panX)",
	} {
		if !strings.Contains(sfx, marker) {
			t.Fatalf("galaxa juice combat sfx missing %q", marker)
		}
	}
	fx := readEmbeddedText(t, "js/desktop/apps/galaxa-fx.js")
	if !strings.Contains(fx, "fxMuzzleSparks") {
		t.Fatal("galaxa-fx missing fxMuzzleSparks")
	}
	combat := readEmbeddedText(t, "js/desktop/apps/galaxa-entities-combat.js")
	if !strings.Contains(combat, "fxMuzzleSparks") {
		t.Fatal("combat fire path must call fxMuzzleSparks")
	}
}

func TestGalaxaJuicePlayerFeedbackMarkers(t *testing.T) {
	t.Parallel()
	sfx := readEmbeddedText(t, "js/desktop/apps/galaxa-audio-sfx.js")
	for _, marker := range []string{
		"graze(panX) { if (ctx.G.muted) return;",
		"parrySuccess(panX) { if (ctx.G.muted) return;",
		"parryMiss(panX) { if (ctx.G.muted) return;",
	} {
		if !strings.Contains(sfx, marker) {
			t.Fatalf("missing mute-guarded sfx %q", marker)
		}
	}
	fx := readEmbeddedText(t, "js/desktop/apps/galaxa-fx.js")
	if !strings.Contains(fx, "fxParryRing") {
		t.Fatal("missing fxParryRing")
	}
}
