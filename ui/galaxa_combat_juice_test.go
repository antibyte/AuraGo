package ui

import (
	"strings"
	"testing"
)

// Galaxa Deluxe combat juice pack (super-ready cue, last-life alarm, respawn
// teleport ring, multi-kill cluster) marker tests. Same style as
// galaxa_juice_test.go; new effect names get their own assertions here and the
// fixed name lists in TestGalaxaJuiceMuteAndCaps stay untouched.

func TestGalaxaCombatJuiceSFXMarkers(t *testing.T) {
	t.Parallel()
	sfx := readEmbeddedText(t, "js/desktop/apps/galaxa-audio-sfx.js")
	for _, marker := range []string{
		"superReady(panX) { if (ctx.G.muted) return;",
		"heartbeat() { if (ctx.G.muted) return;",
		"multiKill(panX) { if (ctx.G.muted) return;",
	} {
		if !strings.Contains(sfx, marker) {
			t.Fatalf("galaxa combat juice sfx missing %q", marker)
		}
	}
}

func TestGalaxaCombatJuiceFXMarkers(t *testing.T) {
	t.Parallel()
	fx := readEmbeddedText(t, "js/desktop/apps/galaxa-fx.js")
	for _, marker := range []string{
		"fxRespawnTeleport",
		"fxMultiKill",
		"fxSuperReady",
		"prefers-reduced-motion",
	} {
		if !strings.Contains(fx, marker) {
			t.Fatalf("galaxa-fx missing %q", marker)
		}
	}
}

func TestGalaxaCombatJuiceWiringMarkers(t *testing.T) {
	t.Parallel()
	consts := readEmbeddedText(t, "js/desktop/apps/galaxa-constants.js")
	if !strings.Contains(consts, "respawn:") {
		t.Fatal("FX_CAPS must include respawn")
	}
	combat := readEmbeddedText(t, "js/desktop/apps/galaxa-entities-combat.js")
	if !strings.Contains(combat, "fxRespawnTeleport") {
		t.Fatal("respawn site must call fxRespawnTeleport")
	}
	if !strings.Contains(combat, "fxMultiKill") {
		t.Fatal("registerKill must trigger fxMultiKill")
	}
	supers := readEmbeddedText(t, "js/desktop/apps/galaxa-supers.js")
	if !strings.Contains(supers, "superReadyFired = false") {
		t.Fatal("startSuper must reset superReadyFired")
	}
}
