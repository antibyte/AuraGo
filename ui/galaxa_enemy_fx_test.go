package ui

import (
	"strings"
	"testing"
)

// Galaxa Deluxe enemy-behaviour + enemy-effect pass (quirks, feints, squad and
// revenge dives, dive path profiles, enemy-side FX). Same marker style as
// galaxa_juice_test.go / galaxa_combat_juice_test.go: these assertions pin the
// wiring that is easy to sever during a later refactor.

func TestGalaxaEnemyFxModuleRegistered(t *testing.T) {
	t.Parallel()
	loader := readEmbeddedText(t, "js/desktop/core/module-loader.js")
	if !strings.Contains(loader, "/js/desktop/apps/galaxa-enemy-fx.js") {
		t.Fatal("module loader must register galaxa-enemy-fx.js")
	}
	if !strings.Contains(loader, "/js/desktop/apps/galaxa-enemy-motion.js") {
		t.Fatal("enemy motion module must stay registered next to enemy-fx")
	}
	entities := readEmbeddedText(t, "js/desktop/apps/galaxa-entities.js")
	if !strings.Contains(entities, "GC.createEnemyFx(ctx)") {
		t.Fatal("createEntities must wire GC.createEnemyFx(ctx)")
	}
}

func TestGalaxaEnemyFxConstantsPresent(t *testing.T) {
	t.Parallel()
	consts := readEmbeddedText(t, "js/desktop/apps/galaxa-constants.js")
	for _, marker := range []string{
		"GC.ENEMY_QUIRKS",
		"GC.QUIRK_CHANCE",
		"GC.FEINT",
		"GC.SQUAD_DIVE",
		"GC.REVENGE_DIVE",
		"GC.FORMATION_SHUFFLE",
		"GC.DIVE_PATHS",
		"GC.ENEMY_RAGE_COL",
		"GC.ENEMY_BIOME_TINT",
		"GC.SMOLDER",
	} {
		if !strings.Contains(consts, marker) {
			t.Fatalf("galaxa-constants missing %q", marker)
		}
	}
	for _, cap := range []string{"telegraph:", "enemyMarks:", "chain:", "smolder:"} {
		if !strings.Contains(consts, cap) {
			t.Fatalf("FX_CAPS must include %q", cap)
		}
	}
}

func TestGalaxaEnemyFxHonorsParticlesAndReducedMotion(t *testing.T) {
	t.Parallel()
	fx := readEmbeddedText(t, "js/desktop/apps/galaxa-enemy-fx.js")
	for _, marker := range []string{
		"function caps() { return GC.FX_CAPS[ctx.settings.particles]",
		"function reducedMotion()",
		"prefers-reduced-motion",
		"ctx.getParticle",
		"function updateEnemyFx(dt)",
		"function drawEnemyFxWorld(c)",
		"ctx.updateEnemyFx = updateEnemyFx",
		"ctx.drawEnemyFxWorld = drawEnemyFxWorld",
		"function igniteSmolder(e, prep)",
		"function updateSmolder(dt, dtMs)",
		"function smolderDetonate(e, defused, playerHit)",
		"e.st = 'SMOLDER'",
		"ctx.igniteSmolder = igniteSmolder",
		"ctx.smolderDetonate = smolderDetonate",
	} {
		if !strings.Contains(fx, marker) {
			t.Fatalf("galaxa-enemy-fx missing %q", marker)
		}
	}
}

func TestGalaxaEnemyFxWiringMarkers(t *testing.T) {
	t.Parallel()
	behaviors := readEmbeddedText(t, "js/desktop/apps/galaxa-entities-behaviors.js")
	for _, marker := range []string{
		"function assignEnemyQuirk(e)",
		"ctx.assignEnemyQuirk = assignEnemyQuirk",
		"ctx.onEnemyKilled = onEnemyKilled",
		"function moveDivePath(e, eDt)",
		"function pickDivePath(e)",
		"e.st = 'FEINT'",
		"else if (e.st === 'FEINT')",
		"GC.SQUAD_DIVE.staggerMs",
		"ctx.fxSquadDiveCue",
	} {
		if !strings.Contains(behaviors, marker) {
			t.Fatalf("galaxa-entities-behaviors missing %q", marker)
		}
	}
	// Quirks must be assigned at both spawn sites.
	spawning := readEmbeddedText(t, "js/desktop/apps/galaxa-entities-spawning.js")
	if !strings.Contains(spawning, "ctx.assignEnemyQuirk(enemy)") {
		t.Fatal("spawning must assign a quirk per enemy")
	}
	campaign := readEmbeddedText(t, "js/desktop/apps/galaxa-campaign.js")
	if !strings.Contains(campaign, "ctx.assignEnemyQuirk(e)") {
		t.Fatal("campaign newEnemy must assign a quirk")
	}
	// Kill path must notify the behaviour layer and spawn kill chains.
	combat := readEmbeddedText(t, "js/desktop/apps/galaxa-entities-combat.js")
	if !strings.Contains(combat, "ctx.onEnemyKilled(_kx, _ky)") {
		t.Fatal("registerKill must call ctx.onEnemyKilled")
	}
	if !strings.Contains(combat, "ctx.fxKillChain") {
		t.Fatal("registerKill must link rapid kills with fxKillChain")
	}
	if !strings.Contains(combat, "ctx.maybeSmolder(e, b)") {
		t.Fatal("bullet kill path must try the smolder crash dive")
	}
	fx := readEmbeddedText(t, "js/desktop/apps/galaxa-enemy-fx.js")
	for _, marker := range []string{
		"function maybeSmolder(e, b)",
		"ctx.maybeSmolder = maybeSmolder",
		"function smolderDetonate(e, defused, playerHit)",
	} {
		if !strings.Contains(fx, marker) {
			t.Fatalf("galaxa-enemy-fx missing %q", marker)
		}
	}
	if !strings.Contains(campaign, "ctx.fxEnemyHit") {
		t.Fatal("damageEnemy must spawn an enemy hit pop")
	}
	// Render + update hooks.
	render := readEmbeddedText(t, "js/desktop/apps/galaxa-render.js")
	if !strings.Contains(render, "ctx.drawEnemyFxWorld(c)") {
		t.Fatal("render pipeline must draw the enemy FX world pass")
	}
	game := readEmbeddedText(t, "js/desktop/apps/galaxa-game.js")
	if !strings.Contains(game, "ctx.updateEnemyFx(dt)") {
		t.Fatal("updateExp must tick ctx.updateEnemyFx")
	}
	// Enemy motion must treat feints like dives and tint by rage/biome.
	motion := readEmbeddedText(t, "js/desktop/apps/galaxa-enemy-motion.js")
	if !strings.Contains(motion, "e.st === 'FEINT'") {
		t.Fatal("enemy motion must cover the FEINT state")
	}
	if !strings.Contains(motion, "GC.ENEMY_BIOME_TINT") {
		t.Fatal("enemy motion trail must tint by biome")
	}
}

func TestGalaxaEnemyFxSFXRespectMute(t *testing.T) {
	t.Parallel()
	sfx := readEmbeddedText(t, "js/desktop/apps/galaxa-audio-sfx.js")
	for _, marker := range []string{
		"feintWhistle(panX) { if (ctx.G.muted) return;",
		"squadDive(panX) { if (ctx.G.muted) return;",
		"rageGrowl(panX) { if (ctx.G.muted) return;",
		"shuffleRumble(panX) { if (ctx.G.muted) return;",
		"smolderIgnite(panX) { if (ctx.G.muted) return;",
		"smolderBlast(panX) { if (ctx.G.muted) return;",
	} {
		if !strings.Contains(sfx, marker) {
			t.Fatalf("galaxa audio sfx missing mute guard %q", marker)
		}
	}
}
