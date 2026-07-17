package ui

import (
	"strings"
	"testing"
)

func TestThreeDeeRobotsFirePrismLanceBeam(t *testing.T) {
	t.Parallel()

	shader := readDesktopAssetText(t, "js/chat/threedee-shader.js")
	if cooldown := extractJSConstFloat(t, shader, "ROBOT_BEAM_COOLDOWN"); cooldown < 6.0 {
		t.Fatalf("ROBOT_BEAM_COOLDOWN should keep the Prism Lance rare, got %.2f", cooldown)
	}
	if duration := extractJSConstFloat(t, shader, "ROBOT_BEAM_DURATION"); duration <= 0.3 {
		t.Fatalf("ROBOT_BEAM_DURATION should make the beam visible for a while, got %.2f", duration)
	}

	for _, marker := range []string{
		"const ROBOT_BEAM_COOLDOWN =",
		"const ROBOT_BEAM_CHARGE_TIME =",
		"const ROBOT_BEAM_DURATION =",
		"const ROBOT_BEAM_HIT_RADIUS =",
		"const ROBOT_BEAM_IMPULSE_STRENGTH =",
		"const ROBOT_BEAM_MAX_LENGTH =",
		"const ROBOT_BEAM_GROUND_STEP =",
		"const BEAM_VERTEX_SHADER =",
		"const BEAM_FRAGMENT_SHADER =",
		"isBeamCharging: false",
		"lastBeamAt: -999",
		"function ensureBeamAssets",
		"function acquireBeamMeshes",
		"function releaseBeamMeshes",
		"function fireRobotBeam",
		"function updateRobotBeams",
		"function applyRobotBeamHit",
		"function spawnBeamImpactEffects",
		"function beamSurfaceStrikeLength",
		"window.beamCoreGeom",
		"window.beamHaloGeom",
		"beam.pair.core.quaternion.setFromUnitVectors(UP_AXIS, dir)",
		"beam.hitPoint = targetPoint.clone();",
		"kind: 'beamMuzzle'",
		"kind: 'beamImpactSpark'",
		"kind: 'beamHitSpark'",
		"kind: 'beamEndFlare'",
		"updateRobotBeams(dt, t);",
		"fireRobotBeam(blueRobot, redRobot, t);",
		"fireRobotBeam(redRobot, blueRobot, t);",
	} {
		if !strings.Contains(shader, marker) {
			t.Fatalf("threedee-shader.js missing Prism Lance beam marker %q", marker)
		}
	}
}

func TestThreeDeeRobotsDeflectShotsWithEnergyShield(t *testing.T) {
	t.Parallel()

	shader := readDesktopAssetText(t, "js/chat/threedee-shader.js")
	if minHits := extractJSConstFloat(t, shader, "ROBOT_SHIELD_MIN_HITS"); minHits < 2 {
		t.Fatalf("ROBOT_SHIELD_MIN_HITS should require battle damage before shields appear, got %.2f", minHits)
	}

	for _, marker := range []string{
		"const ROBOT_SHIELD_COOLDOWN =",
		"const ROBOT_SHIELD_DURATION =",
		"const ROBOT_SHIELD_MIN_HITS =",
		"const ROBOT_SHIELD_RADIUS_SCALE =",
		"const ROBOT_SHIELD_RICOCHET_SPEED =",
		"const SHIELD_VERTEX_SHADER =",
		"const SHIELD_FRAGMENT_SHADER =",
		"shieldUntil: -999",
		"shieldReadyAt: 4.0",
		"shieldHitFlash: 0",
		"shieldMesh: null",
		"function robotShieldRadius",
		"function ensureRobotShield",
		"function updateRobotShields",
		"function spawnShieldDeflectEffects",
		"updateRobotShields(dt, t);",
		"projectile.ricocheted = true;",
		"projectile.target = shooter;",
		"projectile.source = shieldBot;",
		"projectile.mesh.material.color.setHex(shieldBot.projectileHex);",
		"projectile.speed = ROBOT_SHIELD_RICOCHET_SPEED;",
		"kind: 'shieldDeflectSpark'",
	} {
		if !strings.Contains(shader, marker) {
			t.Fatalf("threedee-shader.js missing deflector shield marker %q", marker)
		}
	}
}

func TestThreeDeeRobotsBlinkAndBarrelRollToDodge(t *testing.T) {
	t.Parallel()

	shader := readDesktopAssetText(t, "js/chat/threedee-shader.js")
	if chance := extractJSConstFloat(t, shader, "ROBOT_BLINK_CHANCE"); chance <= 0 || chance > 0.9 {
		t.Fatalf("ROBOT_BLINK_CHANCE should be a defensive rarity, got %.2f", chance)
	}

	for _, marker := range []string{
		"const ROBOT_BLINK_CHANCE =",
		"const ROBOT_BLINK_DURATION =",
		"const ROBOT_BLINK_DISTANCE =",
		"const ROBOT_BARREL_ROLL_DURATION =",
		"blinkUntil: -999",
		"rollUntil: -999",
		"rollDirection: 1",
		"function startRobotBlink",
		"function updateRobotBlink",
		"function startRobotBarrelRoll",
		"updateRobotBlink(dt, t);",
		"startRobotBlink(bot, t, sideX, sideZ)",
		"bot.group.visible = false;",
		"kind: 'blinkImplode'",
		"kind: 'blinkRematerialize'",
		"barrelRollAngle",
		"const targetBlinking = projectile.target.state && projectile.target.state.blinkUntil > t;",
	} {
		if !strings.Contains(shader, marker) {
			t.Fatalf("threedee-shader.js missing blink/barrel roll marker %q", marker)
		}
	}
}

func TestThreeDeeNovaClashTriggersSlowMotionBlast(t *testing.T) {
	t.Parallel()

	shader := readDesktopAssetText(t, "js/chat/threedee-shader.js")
	if cooldown := extractJSConstFloat(t, shader, "ROBOT_CLASH_COOLDOWN"); cooldown < 20.0 {
		t.Fatalf("ROBOT_CLASH_COOLDOWN should keep the nova clash a rare highlight, got %.2f", cooldown)
	}
	if slowmo := extractJSConstFloat(t, shader, "ROBOT_CLASH_SLOWMO_SCALE"); slowmo <= 0 || slowmo >= 1 {
		t.Fatalf("ROBOT_CLASH_SLOWMO_SCALE must slow the world down, got %.2f", slowmo)
	}

	for _, marker := range []string{
		"const ROBOT_CLASH_WINDOW =",
		"const ROBOT_CLASH_MIN_HITS =",
		"const ROBOT_CLASH_COOLDOWN =",
		"const ROBOT_CLASH_COLLIDE_RANGE =",
		"const ROBOT_CLASH_SLOWMO_SCALE =",
		"const ROBOT_CLASH_SLOWMO_DURATION =",
		"let worldTimeScale = 1;",
		"let worldTimeSlowUntil = -999;",
		"let novaClashReadyAt = 0;",
		"function updateWorldTimeScale",
		"function updateNovaClash",
		"function spawnNovaClashExplosion",
		"function detonateNovaClash",
		"updateNovaClash(dt, t);",
		"updateWorldTimeScale(rawDt);",
		"const dt = rawDt * worldTimeScale;",
		"globalTime += dt;",
		"projA.clashTarget = clashPoint;",
		"projB.clashTarget = clashPoint;",
		"worldTimeSlowUntil = t + ROBOT_CLASH_SLOWMO_DURATION;",
		"kind: 'clashCore'",
		"kind: 'clashSpark'",
		"kind: 'clashRing'",
	} {
		if !strings.Contains(shader, marker) {
			t.Fatalf("threedee-shader.js missing nova clash marker %q", marker)
		}
	}
}

func TestThreeDeeRobotsVolleyBurstsAndRammingDash(t *testing.T) {
	t.Parallel()

	shader := readDesktopAssetText(t, "js/chat/threedee-shader.js")
	if burstChance := extractJSConstFloat(t, shader, "ROBOT_BURST_CHANCE"); burstChance <= 0 || burstChance >= 0.9 {
		t.Fatalf("ROBOT_BURST_CHANCE should keep single shots the default, got %.2f", burstChance)
	}

	for _, marker := range []string{
		"const ROBOT_BURST_CHANCE =",
		"const ROBOT_BURST_SPREAD =",
		"const ROBOT_DASH_COOLDOWN =",
		"const ROBOT_DASH_RANGE =",
		"const ROBOT_DASH_FORCE =",
		"const ROBOT_DASH_COLLIDE_RANGE =",
		"dashUntil: -999",
		"dashReadyAt: 3.0",
		"function spawnRobotVolley",
		"function resolveRammingCollision",
		"spawnRobotVolley(blueRobot, redRobot, t, blueRobot.state.isSuperweaponCharging);",
		"spawnRobotVolley(redRobot, blueRobot, t, redRobot.state.isSuperweaponCharging);",
		"extra.direction.applyAxisAngle(UP_AXIS, side * ROBOT_BURST_SPREAD);",
		"extra.burstWeak = true;",
		"projectile.burstWeak ? clamp(dt * 1.4, 0, 0.2) : clamp(dt * 2.2, 0, 0.32)",
		"state.dashVector.set(",
		"resolveRammingCollision(blueRobot, redRobot, t);",
	} {
		if !strings.Contains(shader, marker) {
			t.Fatalf("threedee-shader.js missing burst/dash marker %q", marker)
		}
	}
}

func TestThreeDeeBattleOverdriveResourcesAreDisposed(t *testing.T) {
	t.Parallel()

	shader := readDesktopAssetText(t, "js/chat/threedee-shader.js")
	for _, marker := range []string{
		"activeBeams.length = 0;",
		"while (beamMeshPool.length) {",
		"'beamCoreGeom',",
		"'beamHaloGeom'",
		"worldTimeScale = 1;",
		"worldTimeSlowUntil = -999;",
		"novaClashReadyAt = 0;",
		"bot.shieldMesh = null;",
		"worldTimeScale: worldTimeScale",
		"activeBeams: activeBeams.length",
		"novaClashReadyIn: Math.max(0, novaClashReadyAt - globalTime)",
	} {
		if !strings.Contains(shader, marker) {
			t.Fatalf("threedee-shader.js missing battle overdrive cleanup/debug marker %q", marker)
		}
	}
}
