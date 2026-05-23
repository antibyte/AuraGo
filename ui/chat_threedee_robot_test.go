package ui

import (
	"encoding/binary"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

func TestThreeDeeThemeLoadsLocalDracoRobotAssets(t *testing.T) {
	t.Parallel()

	html := readDesktopAssetText(t, "index.html")
	shader := readDesktopAssetText(t, "js/chat/threedee-shader.js")

	for _, marker := range []string{
		`/js/vendor/GLTFLoader.min.js`,
		`/js/vendor/DRACOLoader.min.js`,
		`/js/vendor/three.min.js`,
	} {
		if !strings.Contains(html, marker) {
			t.Fatalf("index.html must load local ThreeDee robot dependency %q", marker)
		}
	}
	if strings.Index(html, `/js/vendor/three.min.js`) > strings.Index(html, `/js/vendor/GLTFLoader.min.js`) {
		t.Fatal("GLTFLoader must load after three.min.js")
	}
	if strings.Index(html, `/js/vendor/GLTFLoader.min.js`) > strings.Index(html, `/js/vendor/DRACOLoader.min.js`) {
		t.Fatal("DRACOLoader must load after GLTFLoader")
	}
	for _, asset := range []string{
		"3d/robot.glb",
		"3d/redrobot.glb",
		"js/vendor/GLTFLoader.min.js",
		"js/vendor/DRACOLoader.min.js",
		"js/vendor/draco/draco_wasm_wrapper.js",
		"js/vendor/draco/draco_decoder.wasm",
	} {
		if _, err := Content.ReadFile(asset); err != nil {
			t.Fatalf("ThreeDee robot asset %s must be embedded: %v", asset, err)
		}
	}

	for _, marker := range []string{
		"new THREE.GLTFLoader()",
		"new THREE.DRACOLoader()",
		"setDecoderPath('/js/vendor/draco/')",
		"loader.setDRACOLoader(dracoLoader)",
		"loader.load('/3d/robot.glb'",
		"loader.load('/3d/redrobot.glb'",
	} {
		if !strings.Contains(shader, marker) {
			t.Fatalf("threedee-shader.js must use local DRACO GLB loading marker %q", marker)
		}
	}
	if strings.Contains(shader, "https://") || strings.Contains(shader, "unpkg.com") || strings.Contains(shader, "cdn.jsdelivr") {
		t.Fatal("ThreeDee robot loader must not depend on remote libraries or assets")
	}
}

func TestThreeDeeRobotAssetsUseDracoCompression(t *testing.T) {
	t.Parallel()

	for _, asset := range []string{
		"3d/robot.glb",
		"3d/redrobot.glb",
	} {
		data, err := Content.ReadFile(asset)
		if err != nil {
			t.Fatalf("ThreeDee robot asset %s must be embedded: %v", asset, err)
		}
		jsonChunk := readGLBJSONChunk(t, asset, data)
		if !strings.Contains(jsonChunk, `"KHR_draco_mesh_compression"`) {
			t.Fatalf("ThreeDee robot asset %s must use KHR_draco_mesh_compression", asset)
		}
	}
}

func TestThreeDeeRobotUsesWavePhysicsAndBoundaryBounce(t *testing.T) {
	t.Parallel()

	shader := readDesktopAssetText(t, "js/chat/threedee-shader.js")
	for _, marker := range []string{
		"robotVelocity",
		"robotBounds",
		"loadFloatingRobot",
		"normalizeFloatingRobot",
		"sampleSurfaceNormal",
		"updateFloatingRobot",
		"bounceFloatingRobotWithinBounds",
		"heightAt(robotState.x, robotState.z, t, sampleOptions)",
		"surface.position.z",
		"robotGroup.position.lerp",
		"targetQuaternion.slerp",
	} {
		if !strings.Contains(shader, marker) {
			t.Fatalf("threedee-shader.js missing robot wave physics marker %q", marker)
		}
	}
}

func TestThreeDeeRobotsDuelWithEnergyProjectiles(t *testing.T) {
	t.Parallel()

	shader := readDesktopAssetText(t, "js/chat/threedee-shader.js")
	for _, marker := range []string{
		"const ROBOT_DUEL_DISTANCE =",
		"const ROBOT_DUEL_COOLDOWN =",
		"const energyProjectiles = [];",
		"function createRobotConfig",
		"function loadRobotAsset",
		"function updateRobotDuel",
		"function spawnEnergyProjectile",
		"function updateEnergyProjectiles",
		"projectileLight",
		"energyProjectile",
		"spawnEnergyExplosion(",
		"cameraShake = Math.max(cameraShake",
	} {
		if !strings.Contains(shader, marker) {
			t.Fatalf("threedee-shader.js missing robot duel marker %q", marker)
		}
	}
}

func TestThreeDeeRobotsRequireCloseRangeAndReactToHits(t *testing.T) {
	t.Parallel()

	shader := readDesktopAssetText(t, "js/chat/threedee-shader.js")
	if distance := extractJSConstFloat(t, shader, "ROBOT_DUEL_DISTANCE"); distance > 4.2 {
		t.Fatalf("ROBOT_DUEL_DISTANCE should require close robot range, got %.2f", distance)
	}
	if redScale := extractJSConstFloat(t, shader, "ROBOT_RED_TARGET_SIZE"); redScale <= 1.45 {
		t.Fatalf("ROBOT_RED_TARGET_SIZE should make red robot larger than blue, got %.2f", redScale)
	}

	for _, marker := range []string{
		"const ROBOT_HIT_RECOIL =",
		"function applyRobotHitRecoil",
		"target.velocity.x += recoil.x * ROBOT_HIT_RECOIL",
		"target.state.hitFlash",
		"target.state.hits",
		"function createJetFlameSprite",
		"const RED_ROBOT_FOOT_JET_OFFSETS =",
		"[0, ROBOT_FOOT_JET_UNDERSIDE_Y, -0.25]",
		"[0, ROBOT_FOOT_JET_UNDERSIDE_Y, 0.25]",
		"RED_ROBOT_FOOT_JET_OFFSETS.map",
		"kind: 'robotJetFlame'",
		"function spawnEnergyExplosion",
		"kind: 'energyImpactCore'",
		"kind: 'energyImpactSpark'",
		"kind: 'energyImpactRing'",
	} {
		if !strings.Contains(shader, marker) {
			t.Fatalf("threedee-shader.js missing close duel/hit reaction marker %q", marker)
		}
	}
}

func TestThreeDeeRobotHitsCreateMeshDentsAndScorchMarks(t *testing.T) {
	t.Parallel()

	shader := readDesktopAssetText(t, "js/chat/threedee-shader.js")
	for _, marker := range []string{
		"const ROBOT_DAMAGE_DENT_RADIUS =",
		"const ROBOT_DAMAGE_DENT_DEPTH =",
		"const ROBOT_DAMAGE_MAX_DENT_OFFSET =",
		"const ROBOT_DAMAGE_DECAL_OFFSET =",
		"const ROBOT_DAMAGE_MAX_SCORCH_MARKS =",
		"damageMeshes: []",
		"damageScorchMarks: []",
		"node.geometry = node.geometry.clone();",
		"robotDamageBasePositions",
		"position.setUsage(THREE.DynamicDrawUsage)",
		"function robotAimPoint",
		"function resolveRobotDamageImpact",
		"function intersectRobotDamageBox",
		"function applyRobotDamage",
		"function applyRobotMeshDent",
		"function spawnRobotScorchMarks",
		"const targetPosition = robotAimPoint(target);",
		"const targetPosition = robotAimPoint(projectile.target);",
		"const damage = resolveRobotDamageImpact(target, impactPosition, impactDirection);",
		"const basePositions = mesh.geometry.userData.robotDamageBasePositions;",
		"const offsetLength = Math.hypot(currentOffsetX, currentOffsetY, currentOffsetZ);",
		"const offsetScale = Math.min(1, ROBOT_DAMAGE_MAX_DENT_OFFSET / offsetLength);",
		"new THREE.MeshBasicMaterial",
		"new THREE.PlaneGeometry(1, 1)",
		"scorch.quaternion.setFromUnitVectors",
		"scorch.position.copy(damage.position).addScaledVector(damage.normal, ROBOT_DAMAGE_DECAL_OFFSET);",
		"robot-damage-scorch",
		"target.damageScorchMarks.push(scorch);",
		"position.needsUpdate = true;",
		"mesh.geometry.computeVertexNormals();",
		"applyRobotDamage(target, impactPosition, recoil, isSuper);",
	} {
		if !strings.Contains(shader, marker) {
			t.Fatalf("threedee-shader.js missing robot damage marker %q", marker)
		}
	}
	if strings.Contains(shader, "const scorch = new THREE.Sprite") ||
		strings.Contains(shader, "new THREE.SpriteMaterial({\n                map: texture,") {
		t.Fatal("robot scorch marks should be surface-aligned decals, not camera-facing sprites")
	}
}

func TestThreeDeeRobotsLiftAndDampenMatrixWaves(t *testing.T) {
	t.Parallel()

	shader := readDesktopAssetText(t, "js/chat/threedee-shader.js")
	for _, marker := range []string{
		"const ROBOT_FLIGHT_MIN_INTERVAL =",
		"const ROBOT_FLIGHT_MAX_INTERVAL =",
		"const ROBOT_FLIGHT_DURATION =",
		"const ROBOT_FLIGHT_HEIGHT =",
		"const ROBOT_FLIGHT_MAX_HEIGHT =",
		"const ROBOT_WAVE_DAMPING_HEIGHT =",
		"flightLift",
		"nextFlightAt",
		"function scheduleNextRobotFlight",
		"function updateRobotFlight",
		"robotWaveInfluenceForFlightHeight",
		"height += hoverDepression * flightWaveInfluence",
		"waterY * flightWaveInfluence",
		"normal.lerp(bot.up, 1 - flightWaveInfluence).normalize()",
		"bot.thrusterLight.intensity = 1.4 + (bot.state.flightLift || 0) * 0.9",
	} {
		if !strings.Contains(shader, marker) {
			t.Fatalf("threedee-shader.js missing robot flight/wave damping marker %q", marker)
		}
	}
}

func TestThreeDeeRobotFlightsUseRandomHighAltitude(t *testing.T) {
	t.Parallel()

	shader := readDesktopAssetText(t, "js/chat/threedee-shader.js")
	baseHeight := extractJSConstFloat(t, shader, "ROBOT_FLIGHT_HEIGHT")
	maxHeight := extractJSConstFloat(t, shader, "ROBOT_FLIGHT_MAX_HEIGHT")
	if maxHeight < baseHeight*2.6 {
		t.Fatalf("robot max flight height should be much higher than the base height, got base %.2f max %.2f", baseHeight, maxHeight)
	}
	for _, marker := range []string{
		"const flightHeightRange = ROBOT_FLIGHT_MAX_HEIGHT - ROBOT_FLIGHT_HEIGHT;",
		"state.flightPeak = ROBOT_FLIGHT_HEIGHT + Math.random() * flightHeightRange;",
		"state.flightPeak *= bot.id === 'red' ? 1.08 : 0.96;",
	} {
		if !strings.Contains(shader, marker) {
			t.Fatalf("threedee-shader.js missing random high flight marker %q", marker)
		}
	}
}

func TestThreeDeeRobotsDodgeApproachingSuperweaponsWorseAfterHits(t *testing.T) {
	t.Parallel()

	shader := readDesktopAssetText(t, "js/chat/threedee-shader.js")
	baseRange := extractJSConstFloat(t, shader, "ROBOT_SUPERWEAPON_EVASION_RANGE")
	minRange := extractJSConstFloat(t, shader, "ROBOT_SUPERWEAPON_EVASION_MIN_RANGE")
	if minRange >= baseRange {
		t.Fatalf("damaged robots should detect superweapons later: min %.2f base %.2f", minRange, baseRange)
	}

	for _, marker := range []string{
		"const ROBOT_SUPERWEAPON_EVASION_RANGE =",
		"const ROBOT_SUPERWEAPON_EVASION_MIN_RANGE =",
		"const ROBOT_SUPERWEAPON_EVASION_HIT_PENALTY =",
		"evasionUntil: -999",
		"evasionVector: new THREE.Vector2(0, 0)",
		"evasionThreat: null",
		"function robotSuperweaponDetectionRange",
		"Math.max(ROBOT_SUPERWEAPON_EVASION_MIN_RANGE",
		"(bot.state.hits || 0) * ROBOT_SUPERWEAPON_EVASION_HIT_PENALTY",
		"function projectileClosingSpeedToRobot",
		"projectile.direction.dot(toRobot)",
		"projectile.velocity3D.dot(toRobot)",
		"function tryRobotSuperweaponEvasion",
		"if (!projectile.isSuper || projectile.target !== bot)",
		"const detectionRange = robotSuperweaponDetectionRange(bot);",
		"if (distance > detectionRange || closingSpeed <= ROBOT_SUPERWEAPON_EVASION_MIN_CLOSING_SPEED)",
		"state.evasionUntil = t + ROBOT_SUPERWEAPON_EVASION_DURATION",
		"state.evasionThreat = projectile",
		"state.evasionVector.set(sideX, sideZ).normalize();",
		"state.flightStartedAt = t - ROBOT_SUPERWEAPON_EVASION_FLIGHT_PHASE",
		"state.flightPeak = Math.max(state.flightPeak || 0, ROBOT_SUPERWEAPON_EVASION_FLIGHT_HEIGHT);",
		"bot.state.pendingThrusterRipple = Math.max(bot.state.pendingThrusterRipple || 0, 1.18);",
		"tryRobotSuperweaponEvasion(projectile, t);",
		"if (bot.state.evasionUntil > t)",
		"bot.velocity.x += bot.state.evasionVector.x * ROBOT_SUPERWEAPON_EVASION_FORCE * dt",
		"bot.velocity.y += bot.state.evasionVector.y * ROBOT_SUPERWEAPON_EVASION_FORCE * dt",
	} {
		if !strings.Contains(shader, marker) {
			t.Fatalf("threedee-shader.js missing robot superweapon evasion marker %q", marker)
		}
	}
}

func TestThreeDeeRedRocketUsesSluggishTargetCorrection(t *testing.T) {
	t.Parallel()

	shader := readDesktopAssetText(t, "js/chat/threedee-shader.js")
	if maxTurn := extractJSConstFloat(t, shader, "ROBOT_RED_ROCKET_MAX_TURN_STEP"); maxTurn > 0.18 {
		t.Fatalf("red rocket turn correction should stay sluggish, got max turn %.2f", maxTurn)
	}
	if turnRate := extractJSConstFloat(t, shader, "ROBOT_RED_ROCKET_TURN_RATE"); turnRate > 1.6 {
		t.Fatalf("red rocket turn rate should be much lower than the old aggressive homing, got %.2f", turnRate)
	}

	for _, marker := range []string{
		"const ROBOT_RED_ROCKET_TURN_RATE =",
		"const ROBOT_RED_ROCKET_MAX_TURN_STEP =",
		"const ROBOT_RED_ROCKET_ACCELERATION =",
		"const ROBOT_RED_ROCKET_MAX_SPEED =",
		"function updateRedRocketGuidance",
		"const targetDirection = toTarget.normalize();",
		"const correction = clamp(dt * ROBOT_RED_ROCKET_TURN_RATE, 0, ROBOT_RED_ROCKET_MAX_TURN_STEP);",
		"projectile.direction.lerp(targetDirection, correction).normalize();",
		"projectile.currentSpeed = Math.min(ROBOT_RED_ROCKET_MAX_SPEED, projectile.currentSpeed + dt * ROBOT_RED_ROCKET_ACCELERATION);",
		"updateRedRocketGuidance(projectile, dt);",
	} {
		if !strings.Contains(shader, marker) {
			t.Fatalf("threedee-shader.js missing sluggish red rocket marker %q", marker)
		}
	}
	if strings.Contains(shader, "projectile.direction.lerp(toTarget.normalize(), clamp(dt * 5.5, 0, 0.65)).normalize();") {
		t.Fatal("red rocket must not use the old aggressive homing correction")
	}
}

func TestThreeDeeRobotThrustersUseUndersideOffsetsAndFadingRipples(t *testing.T) {
	t.Parallel()

	shader := readDesktopAssetText(t, "js/chat/threedee-shader.js")
	for _, marker := range []string{
		"const ROBOT_FOOT_JET_UNDERSIDE_Y =",
		"const ROBOT_THRUSTER_RIPPLE_LIFETIME =",
		"const ROBOT_THRUSTER_RIPPLE_MAX_ACTIVE_PER_ROBOT =",
		"const ROBOT_THRUSTER_RIPPLE_WIDTH =",
		"const MAX_ROBOT_THRUSTER_RIPPLES =",
		"const robotThrusterRipples = [];",
		"function addRobotThrusterRipple",
		"function updateRobotThrusterRipples",
		"function robotThrusterRippleHeightAt",
		"lastThrusterRippleAt: -999",
		"thrusterRipplePrimed: false",
		"flightWasActive: false",
		"const ignoreRobotOwner = options && options.ignoreRobotOwner;",
		"const hoverDepression = -0.2 * Math.exp",
		"height += hoverDepression * flightWaveInfluence;",
		"if (ignoreRobotOwner && botOwner === ignoreRobotOwner) continue;",
		"height += robotThrusterRippleHeightAt(x, z, t, ignoreRobotOwner);",
		"if (ignoreOwner && ripple.owner === ignoreOwner) continue;",
		"const sampleOptions = bot && bot.id ? { ignoreRobotOwner: bot.id, ignoreRobotFeedbackWaves: true } : { ignoreRobotFeedbackWaves: true };",
		"updateRobotThrusterRipples(t);",
		"new THREE.Vector3(0, ROBOT_FOOT_JET_UNDERSIDE_Y, 0)",
		"const owner = bot.id || 'robot';",
		"owner,",
		"activeForRobot >= ROBOT_THRUSTER_RIPPLE_MAX_ACTIVE_PER_ROBOT",
		"const rippleScale = strengthScale == null ? 1 : clamp(strengthScale, 0.35, 1.35);",
		"addRobotThrusterRipple(bot, t, pendingThrusterRipple);",
		"const ridge = Math.exp(-(delta * delta) / ROBOT_THRUSTER_RIPPLE_WIDTH);",
		"const trailingWake = Math.exp(-(wakeDelta * wakeDelta) / (ROBOT_THRUSTER_RIPPLE_WIDTH * 1.8));",
		"const rippleAttack = smoothstep",
		"const rippleRelease = 1 - smoothstep",
		"const rippleFade = rippleAttack * rippleRelease * rippleRelease",
	} {
		if !strings.Contains(shader, marker) {
			t.Fatalf("threedee-shader.js missing thruster underside/ripple marker %q", marker)
		}
	}
	if strings.Contains(shader, "const hoverRipple = Math.sin(distToRobot * 6.8 - t * 11.5)") {
		t.Fatal("thruster ripple must not be a continuous phase-resetting heightAt sine wave")
	}
	if strings.Contains(shader, "nextThrusterRippleAt") {
		t.Fatal("thruster ripples must be event-driven, not periodically rescheduled")
	}
	if strings.Contains(shader, "Math.cos(delta") {
		t.Fatal("thruster ripple must not use a cosine carrier that aliases on the matrix grid")
	}
}

func TestThreeDeeRobotThrusterRipplesStaySparse(t *testing.T) {
	t.Parallel()

	shader := readDesktopAssetText(t, "js/chat/threedee-shader.js")
	lifetime := extractJSConstFloat(t, shader, "ROBOT_THRUSTER_RIPPLE_LIFETIME")
	if minGap := extractJSConstFloat(t, shader, "ROBOT_THRUSTER_RIPPLE_MIN_GAP"); minGap < lifetime {
		t.Fatalf("thruster ripple minimum gap should avoid dense ripple stacks, got %.2f", minGap)
	}
	if maxActive := extractJSConstFloat(t, shader, "ROBOT_THRUSTER_RIPPLE_MAX_ACTIVE_PER_ROBOT"); maxActive > 1 {
		t.Fatalf("thruster ripples should stay sparse per robot, got max %.0f", maxActive)
	}
	if maxRipples := extractJSConstFloat(t, shader, "MAX_ROBOT_THRUSTER_RIPPLES"); maxRipples > 6 {
		t.Fatalf("global thruster ripple pool should stay small enough to avoid jitter, got %.0f", maxRipples)
	}
	for _, marker := range []string{
		"t - lastRippleAt < ROBOT_THRUSTER_RIPPLE_MIN_GAP",
		"pendingThrusterRipple = Math.max(pendingThrusterRipple, 1)",
		"pendingThrusterRipple = Math.max(pendingThrusterRipple, 0.78)",
		"bot.state.flightWasActive = isFlightActive;",
		"updateFloatingRobot(dt, t);",
		"updateSurface(t);",
	} {
		if !strings.Contains(shader, marker) {
			t.Fatalf("threedee-shader.js missing sparse event-driven ripple marker %q", marker)
		}
	}
	if strings.Contains(shader, "ROBOT_THRUSTER_RIPPLE_INTERVAL") {
		t.Fatal("thruster ripple interval scheduler should not exist")
	}
	if strings.Contains(shader, "ROBOT_THRUSTER_RIPPLE_MAX_ACTIVE_PER_ROBOT = 2") {
		t.Fatal("two active thruster ripples per robot can visually jump between overlapping fronts")
	}
}

func TestThreeDeeRobotSamplingIgnoresFeedbackWaves(t *testing.T) {
	t.Parallel()

	shader := readDesktopAssetText(t, "js/chat/threedee-shader.js")
	for _, marker := range []string{
		"const ignoreRobotFeedbackWaves = options && options.ignoreRobotFeedbackWaves;",
		"if (!ignoreRobotFeedbackWaves && robotState) {",
		"if (!ignoreRobotFeedbackWaves) {",
		"height += robotThrusterRippleHeightAt(x, z, t, ignoreRobotOwner);",
		"const sampleOptions = bot && bot.id ? { ignoreRobotOwner: bot.id, ignoreRobotFeedbackWaves: true } : { ignoreRobotFeedbackWaves: true };",
		"const sampledWaterY = bot.id === 'blue' ? heightAt(robotState.x, robotState.z, t, sampleOptions) : heightAt(bot.state.x, bot.state.z, t, sampleOptions);",
		"bot.state.visualWaterY",
		"const normal = sampleSurfaceNormal(bot.state.x, bot.state.z, t, bot, sampleOptions);",
	} {
		if !strings.Contains(shader, marker) {
			t.Fatalf("threedee-shader.js missing robot feedback damping marker %q", marker)
		}
	}
	if strings.Contains(shader, "sampleSurfaceNormal(bot.state.x, bot.state.z, t, bot);") {
		t.Fatal("robot normal sampling should receive feedback-free sample options")
	}
}

func TestThreeDeeThemeDisposesSceneWhenInactive(t *testing.T) {
	t.Parallel()

	shader := readDesktopAssetText(t, "js/chat/threedee-shader.js")
	for _, marker := range []string{
		"let sceneGeneration = 0;",
		"function disposeThreeDeeScene()",
		"disposeThreeObject(scene);",
		"renderer.dispose();",
		"renderer.forceContextLoss",
		"canvas.remove();",
		"const loadGeneration = sceneGeneration;",
		"loadGeneration !== sceneGeneration",
		"disposeThreeObject(model);",
		"disposeRobotLoader(bot, dracoLoader);",
		"stop();",
		"disposeThreeDeeScene();",
	} {
		if !strings.Contains(shader, marker) {
			t.Fatalf("threedee-shader.js missing theme-exit disposal marker %q", marker)
		}
	}
}

func readGLBJSONChunk(t *testing.T, asset string, data []byte) string {
	t.Helper()

	if len(data) < 20 {
		t.Fatalf("ThreeDee robot asset %s is too small to be a GLB", asset)
	}
	if string(data[:4]) != "glTF" {
		t.Fatalf("ThreeDee robot asset %s has invalid GLB magic", asset)
	}
	jsonLen := int(binary.LittleEndian.Uint32(data[12:16]))
	if string(data[16:20]) != "JSON" {
		t.Fatalf("ThreeDee robot asset %s first GLB chunk is not JSON", asset)
	}
	if 20+jsonLen > len(data) {
		t.Fatalf("ThreeDee robot asset %s has truncated GLB JSON chunk", asset)
	}
	return strings.TrimRight(string(data[20:20+jsonLen]), "\x00 ")
}

func extractJSConstFloat(t *testing.T, source, name string) float64 {
	t.Helper()

	re := regexp.MustCompile(`const\s+` + regexp.QuoteMeta(name) + `\s*=\s*([0-9]+(?:\.[0-9]+)?)`)
	match := re.FindStringSubmatch(source)
	if len(match) != 2 {
		t.Fatalf("missing numeric JS const %s", name)
	}
	value, err := strconv.ParseFloat(match[1], 64)
	if err != nil {
		t.Fatalf("invalid numeric JS const %s=%q: %v", name, match[1], err)
	}
	return value
}
