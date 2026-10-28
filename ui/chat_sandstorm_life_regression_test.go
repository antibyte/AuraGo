package ui

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestSandstormDesertLifeCompanionStaysWired pins the static contract between the
// Sandstorm engine and its lazily loaded desert-life companion: the loader and the
// hooks in the engine's single render loop, the geometry the companion stands on, and
// the companion's rule that it never schedules anything itself.
func TestSandstormDesertLifeCompanionStaysWired(t *testing.T) {
	t.Parallel()

	engineContent, err := os.ReadFile(filepath.Join("js", "chat", "sandstorm-particles.js"))
	if err != nil {
		t.Fatalf("read sandstorm-particles.js: %v", err)
	}
	lifeContent, err := os.ReadFile(filepath.Join("js", "chat", "sandstorm-life.js"))
	if err != nil {
		t.Fatalf("read sandstorm-life.js: %v", err)
	}
	engine := normalizeAssetText(engineContent)
	life := normalizeAssetText(lifeContent)

	for _, marker := range []string{
		// the loader: modelled on the Ocean theme's loadSeaLife
		"/js/chat/sandstorm-life.js",
		"const LIFE_SCRIPT = '/js/chat/sandstorm-life.js';",
		"function loadDesertLife()",
		"function lifeLayer()",
		"script[data-aurago-sandstorm-life]",
		"window.AuraLazyAssets.loadScript(LIFE_SCRIPT)",
		"script.dataset.auragoSandstormLife = 'true';",
		// the hooks in the render loop and the lifecycle
		"life.update(dt, time, lifeGeom)",
		"life.draw(sctx, lifeGeom, -1)",
		"life.draw(sctx, lifeGeom, i)",
		"life.reset(performance.now())",
		"life.reset();",
		"life.resize(lifeGeom)",
		// the geometry: the companion must stand exactly on what drawDunes paints
		"d.dx = dx;",
		"d.dy = dy;",
		"d.crest + d.dy + duneProfile(i, x - d.dx - DUNE_MARGIN)",
		"g.calmLeft = stormActive ? 0 : nextStormTime - time;",
		"baseWind: BASE_WIND,",
		"palette: DUNE_PALETTE",
		// the scene canvas: the companion measures chat bubbles against it to place the worm
		"canvas: null,",
		"g.canvas = sceneCanvas;",
	} {
		if !strings.Contains(engine, marker) {
			t.Fatalf("js/chat/sandstorm-particles.js missing desert-life marker %q", marker)
		}
	}

	// Painter's order in the render loop: update before any drawing, the sky stage
	// before the dunes, and each ridge stage right after its dune was painted.
	assertSandstormLifeOrder(t, "render()", engine,
		"updateBubbleSandPhysics(dt);",
		"life.update(dt, time, lifeGeom)",
		"renderFog(time);",
		"drawDustRolls(height);",
		"life.draw(sctx, lifeGeom, -1)",
		"drawDunes(width, height);",
	)
	assertSandstormLifeOrder(t, "drawDunes()", engine,
		"sctx.drawImage(d.image, dx, d.top + dy, d.width, d.height);",
		"d.dx = dx;",
		"d.dy = dy;",
		"life.draw(sctx, lifeGeom, i)",
		"// Storm dust settles over the ridges",
	)

	if strings.Count(engine, "requestAnimationFrame(render)") != 2 {
		t.Fatal("the desert life must not add an animation loop to the engine")
	}
	if !strings.Contains(engine, "window.AuraGoSandstorm = { start, stop, sync };") {
		t.Fatal("the engine's public surface must stay { start, stop, sync }")
	}

	for _, marker := range []string{
		"window.AuraGoSandstormLife = {",
		"window.AuraGoSandstormLife = { update, draw, resize, reset, forceWorm, forceTumbleweed, forceBeetle, forceCaravan, stats, debug };",
		"'use strict';",
		"function forceWorm(",
		"function stats()",
		"'tremor'",
		"'breach'",
		"'settle'",
		"const MAX_SPRAY = 320;",
		"const MAX_PUFFS = 24;",
		"const MAX_WISPS = 140;",
		// the critters: one force function each (the harness starts them with these) ...
		"function forceTumbleweed(",
		"function forceBeetle(",
		"function forceCaravan(",
		// ... and the occluder lookup that keeps the worm out from behind the chat bubbles
		"#chat-content .msg-row .bubble",
		"#chat-content .msg-row .avatar",
		"'chat-robot-mascot'",
		"windExcess * 0.12 * dtFrames",
		"attached: true,",
		"finite: allFinite()",
	} {
		if !strings.Contains(life, marker) {
			t.Fatalf("js/chat/sandstorm-life.js missing marker %q", marker)
		}
	}
	// stats() reports live counts for the four critter families (the browser test reads them).
	statsStart := strings.Index(life, "function stats()")
	statsEnd := strings.Index(life, "finite: allFinite()")
	if statsStart < 0 || statsEnd < statsStart {
		t.Fatal("js/chat/sandstorm-life.js: stats() not found")
	}
	statsBody := life[statsStart:statsEnd]
	for _, key := range []string{"vultures:", "tumbleweeds:", "beetles:", "caravan:", "tumbleweedX:"} {
		if !strings.Contains(statsBody, key) {
			t.Fatalf("stats() must report %q", key)
		}
	}
	if !strings.HasPrefix(strings.TrimSpace(life), "(function () {") {
		t.Fatal("the companion must stay a classic IIFE script")
	}
	if strings.Count(life, "window.AuraGoSandstormLife =") != 1 {
		t.Fatal("the companion must export exactly one global")
	}
	if regexp.MustCompile(`(?m)^\s*(import|export)\s`).MatchString(life) {
		t.Fatal("the companion is loaded as a classic script: no ES module syntax")
	}

	// The engine owns the only loop and the only clock: the companion reacts to
	// update()/draw() with the engine's time and dt and schedules nothing itself.
	for _, forbidden := range []string{
		"requestAnimationFrame",
		"setInterval",
		"setTimeout(",
		"performance.now",
		"Date.now",
	} {
		if strings.Contains(life, forbidden) {
			t.Fatalf("js/chat/sandstorm-life.js must not use %q", forbidden)
		}
	}
}

// TestSandstormDesertLifeVulturesAvoidTheChat pins how the vultures keep out from behind the
// chat: the thermal they circle is chosen by pickThermal (the only vulture code that reads the
// DOM) from the chat rectangles, relocated on a timer while birds are in view, started over by
// resize and reset, and reported by debug.where(); plus the tuning that keeps the birds in view
// for roughly half of the time.
func TestSandstormDesertLifeVulturesAvoidTheChat(t *testing.T) {
	t.Parallel()

	lifeContent, err := os.ReadFile(filepath.Join("js", "chat", "sandstorm-life.js"))
	if err != nil {
		t.Fatalf("read sandstorm-life.js: %v", err)
	}
	life := normalizeAssetText(lifeContent)

	for _, marker := range []string{
		"function pickThermal(",
		"const thermal = {",
		"const VULTURE_RETURN_MIN = 6;",
		"const VULTURE_RETURN_MAX = 12;",
		"const VULTURE_FLEE_SPEED = 220;",
		"const VULTURE_FLEE_PUSH = 160;",
		"const VULTURE_RELOCATE = 15;",
		"b.size = rand(6, 10);",
		"VULTURE_FLEE_PUSH * sdt",
	} {
		if !strings.Contains(life, marker) {
			t.Fatalf("js/chat/sandstorm-life.js missing vulture marker %q", marker)
		}
	}

	// The placement reads the DOM through the shared occluder lookup, in one place only.
	pick := sandstormLifeFunction(t, life, "function pickThermal(")
	for _, want := range []string{"collectOccluders(", "thermalHidden(", "THERMAL_BIAS", "thermal.picked"} {
		if !strings.Contains(pick, want) {
			t.Errorf("pickThermal must use %q", want)
		}
	}
	for _, name := range []string{"function updateVultures(", "function driftThermal(", "function settleThermal(", "function placeOnThermal("} {
		if body := sandstormLifeFunction(t, life, name); strings.Contains(body, "collectOccluders(") || strings.Contains(body, "document.") {
			t.Errorf("%s must not read the DOM itself (pickThermal does, once per glide-in and per VULTURE_RELOCATE)", name)
		}
	}
	if !strings.Contains(sandstormLifeFunction(t, life, "function settleThermal("), "pickThermal(g)") ||
		!strings.Contains(sandstormLifeFunction(t, life, "function driftThermal("), "VULTURE_RELOCATE") ||
		!strings.Contains(sandstormLifeFunction(t, life, "function updateVultures("), "settleThermal(g)") {
		t.Error("the thermal must be chosen when the first bird glides in and again every VULTURE_RELOCATE seconds")
	}
	// A new geometry or a new run starts the search over, from the default spot.
	for _, name := range []string{"function resize(", "function reset("} {
		body := sandstormLifeFunction(t, life, name)
		if !strings.Contains(body, "thermal.due = 0;") || !strings.Contains(body, "thermal.picked = false;") {
			t.Errorf("%s must send the thermal back to the default spot on the next update", name)
		}
	}
	// The harness reads where the flock circles and where every bird is.
	whereBody := sandstormLifeFunction(t, life, "function where(")
	for _, key := range []string{"vulture:", "vultures:", "thermal:"} {
		if !strings.Contains(whereBody, key) {
			t.Errorf("debug.where() must report %q", key)
		}
	}
}

// sandstormLifeFunction returns the source of the function that starts with header, up to its
// closing brace: the functions of the companion sit directly inside the IIFE, indented by four spaces.
func sandstormLifeFunction(t *testing.T, source, header string) string {
	t.Helper()
	start := strings.Index(source, "    "+header)
	if start < 0 {
		t.Fatalf("js/chat/sandstorm-life.js: %q not found", header)
	}
	end := strings.Index(source[start:], "\n    }\n")
	if end < 0 {
		t.Fatalf("js/chat/sandstorm-life.js: the end of %q not found", header)
	}
	return source[start : start+end]
}

// assertSandstormLifeOrder fails unless every marker occurs in source, each after the previous one.
func assertSandstormLifeOrder(t *testing.T, what, source string, markers ...string) {
	t.Helper()
	from := 0
	for _, marker := range markers {
		idx := strings.Index(source[from:], marker)
		if idx < 0 {
			t.Fatalf("%s: %q is missing or out of order", what, marker)
		}
		from += idx + len(marker)
	}
}
