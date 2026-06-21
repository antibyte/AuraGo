package ui

import (
	"strings"
	"testing"
)

func TestGalaxaSpriteUpgradeRenderingHooks(t *testing.T) {
	t.Parallel()

	sprites := readEmbeddedText(t, "js/desktop/apps/galaxa-sprites.js")
	render := readEmbeddedText(t, "js/desktop/apps/galaxa-render.js")
	entities := readEmbeddedText(t, "js/desktop/apps/galaxa-entities.js")

	for _, marker := range []string{
		"playerFrames",
		"function getPlayerSpriteFrame()",
		"const ENEMY_SPRITE_KEYS =",
		"function enemySpriteFor(e)",
		"ctx.enemySpriteFor = enemySpriteFor",
	} {
		if !strings.Contains(sprites, marker) {
			t.Fatalf("galaxa sprite upgrade missing sprite marker %q", marker)
		}
	}

	for _, marker := range []string{
		"function drawPlayerSpriteDetails(",
		"function drawEnemySpriteDetails(",
		"ctx.getPlayerSpriteFrame()",
		"ctx.enemySpriteFor(e)",
	} {
		if !strings.Contains(render, marker) {
			t.Fatalf("galaxa sprite upgrade missing render marker %q", marker)
		}
	}

	for _, stale := range []string{
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
		if strings.Contains(render, stale) {
			t.Fatalf("galaxa renderer still uses direct enemy frame lookup %q", stale)
		}
	}

	if !strings.Contains(entities, "const animFramesMap =") || !strings.Contains(entities, "animFrames: animFramesMap[type] || 3") {
		t.Fatal("galaxa enemies must use per-type animation frame counts")
	}

	detailBody := sectionBetween(t, render, "function drawEnemySpriteDetails", "function renderFrame")
	for _, forbidden := range []string{"Math.random(", ".push(", "new Map", "new WeakMap", "new OffscreenCanvas"} {
		if strings.Contains(detailBody, forbidden) {
			t.Fatalf("sprite detail rendering must stay deterministic and allocation-free; found %q", forbidden)
		}
	}
}