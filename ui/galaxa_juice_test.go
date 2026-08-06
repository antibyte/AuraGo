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

func TestGalaxaJuicePowerupProgressionMarkers(t *testing.T) {
	t.Parallel()
	sfx := readEmbeddedText(t, "js/desktop/apps/galaxa-audio-sfx.js")
	for _, marker := range []string{
		"puCollectRarity(rarity, panX)",
		"weaponArm(panX) { if (ctx.G.muted) return;",
	} {
		if !strings.Contains(sfx, marker) {
			t.Fatalf("missing %q", marker)
		}
	}
	shop := readEmbeddedText(t, "js/desktop/apps/galaxa-shop.js")
	if !strings.Contains(shop, "shopBuy") {
		t.Fatal("shop buy must use SFX.shopBuy")
	}
}

func TestGalaxaJuiceBossKillMarkers(t *testing.T) {
	t.Parallel()
	sfx := readEmbeddedText(t, "js/desktop/apps/galaxa-audio-sfx.js")
	if !strings.Contains(sfx, "bossKillFanfare(panX) { if (ctx.G.muted) return;") {
		t.Fatal("missing bossKillFanfare")
	}
	fx := readEmbeddedText(t, "js/desktop/apps/galaxa-fx.js")
	if !strings.Contains(fx, "fxBossKillSetPiece") {
		t.Fatal("missing fxBossKillSetPiece")
	}
	if !strings.Contains(fx, "prefers-reduced-motion") {
		t.Fatal("boss kill FX must respect prefers-reduced-motion")
	}
}

func TestGalaxaJuiceSignatureComboStageMarkers(t *testing.T) {
	t.Parallel()
	sfx := readEmbeddedText(t, "js/desktop/apps/galaxa-audio-sfx.js")
	for _, marker := range []string{
		"megaComboStinger(level, panX) { if (ctx.G.muted) return;",
		"stageClearFanfare(panX) { if (ctx.G.muted) return;",
	} {
		if !strings.Contains(sfx, marker) {
			t.Fatalf("missing %q", marker)
		}
	}
	fx := readEmbeddedText(t, "js/desktop/apps/galaxa-fx.js")
	for _, marker := range []string{"fxMegaCombo", "fxStageClearSetPiece"} {
		if !strings.Contains(fx, marker) {
			t.Fatalf("missing %q", marker)
		}
	}
	game := readEmbeddedText(t, "js/desktop/apps/galaxa-game.js")
	if !strings.Contains(game, "stageClearFanfare") && !strings.Contains(game, "fxStageClearSetPiece") {
		t.Fatal("stage clear path must call fanfare or set-piece")
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
