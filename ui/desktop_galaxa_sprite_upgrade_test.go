package ui

import (
	"strings"
	"testing"
)

func TestGalaxaPremiumCodeDefinedSprites(t *testing.T) {
	t.Parallel()

	sprites := readEmbeddedText(t, "js/desktop/apps/galaxa-sprites.js")
	render := readEmbeddedText(t, "js/desktop/apps/galaxa-render.js")
	entities := readEmbeddedText(t, "js/desktop/apps/galaxa-entities.js")

	for _, marker := range []string{
		"const PREMIUM_PIXEL_ART_VERSION = 'galaxa-premium-v5'",
		"const PLAYER_FRAME = Object.freeze({ idleA: 0, idleB: 1, bankLeft: 2, bankRight: 3, boost: 4, fire: 5, super: 6 })",
		"const ENEMY_FRAME_COUNTS = Object.freeze({ bee: 4, butterfly: 4, stalker: 4, sniper: 4, hunter: 4, spinner: 4, bomber: 4, lasher: 4, weaver: 4, splitter: 4, shield_bee: 4, kamikaze: 4, carrier: 4, teleporter: 4, boss: 3, miniboss: 3 })",
		"PREMIUM_PIXEL_ART_VERSION: 'galaxa-premium-v5'",
		"playerIcon: expandedPF[_rawSP.PLAYER_FRAME.idleA]",
		"function validateSpritePalette(",
		"function validateSpriteSet(",
		"function validateSpriteFrameCount(",
		"ENEMY_FRAME_COUNTS,",
		"function getPlayerSpriteFrame()",
		"function enemySpriteFor(e)",
		"ctx.enemySpriteFor = enemySpriteFor",
		"SHEET_FRAMES",
		"SHEET_STRIDE",
		"ctx.SHEET_FRAMES = SHEET_FRAMES",
		"typeof sp === 'string' && ctx.spriteSheet",
	} {
		if !strings.Contains(sprites, marker) {
			t.Fatalf("galaxa premium sprites missing marker %q", marker)
		}
	}

	for _, marker := range []string{
		"ctx.getPlayerSpriteFrame()",
		"ctx.enemySpriteFor(e)",
		"ctx.SP.playerIcon",
	} {
		if !strings.Contains(render, marker) {
			t.Fatalf("galaxa premium render missing marker %q", marker)
		}
	}

	for _, marker := range []string{
		"data:image/png;base64,",
		"new Image()",
		"_preloadedSheet",
	} {
		if !strings.Contains(sprites, marker) {
			t.Fatalf("galaxa sprites missing sprite sheet marker %q", marker)
		}
	}

	for _, stale := range []string{
		"SP.shield_bee = SP.bee",
		"function drawPlayerSpriteDetails(",
		"function drawEnemySpriteDetails(",
		"drawPlayerSpriteDetails(ctx.c",
		"drawEnemySpriteDetails(ctx.c",
		"ctx.SP.bee[e.fr]",
		"ctx.SP.bf[e.fr]",
		"ctx.SP.stalker[e.fr]",
		"ctx.SP.sniper[e.fr]",
		"ctx.SP.hunter[e.fr]",
		"ctx.SP.spinner[e.fr]",
		"ctx.SP.bomber[e.fr]",
		"ctx.SP.lasher[e.fr]",
		"ctx.SP.weaver[e.fr]",
		"ctx.SP.splitter[e.fr]",
		"ctx.SP.shield_bee[e.fr]",
		"ctx.SP.kamikaze[e.fr]",
		"ctx.SP.carrier[e.fr]",
		"ctx.SP.teleporter[e.fr]",
	} {
		if strings.Contains(render+sprites, stale) {
			t.Fatalf("galaxa premium sprites still contain stale pattern %q", stale)
		}
	}

	for _, marker := range []string{
		"const animSpeed = GC.ENEMY_ANIM_SPEED[type] || 120",
		"const animFrames = GC.ENEMY_FRAME_COUNT[type] || 3",
		"animFrame: 0, animTimer: 0, animSpeed, animFrames",
		"spawnAnim: 0, spawnDur: GC.ENEMY_SPAWN_DURATION",
	} {
		if !strings.Contains(entities, marker) {
			t.Fatalf("galaxa enemies must use centralized animation constants, missing %q", marker)
		}
	}
	for _, stale := range []string{
		"const animSpeedMap =",
		"const animFramesMap =",
		"animFrames: animFramesMap[type] || 3",
		"spawnDur: 400",
	} {
		if strings.Contains(entities, stale) {
			t.Fatalf("galaxa enemies still contain stale local animation constant %q", stale)
		}
	}
}
