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
		"heightAt(robotState.x, robotState.z, t)",
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
		"[0, -0.58, -0.25]",
		"[0, -0.58, 0.25]",
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

func TestThreeDeeRobotsLiftAndDampenMatrixWaves(t *testing.T) {
	t.Parallel()

	shader := readDesktopAssetText(t, "js/chat/threedee-shader.js")
	for _, marker := range []string{
		"const ROBOT_FLIGHT_MIN_INTERVAL =",
		"const ROBOT_FLIGHT_MAX_INTERVAL =",
		"const ROBOT_FLIGHT_DURATION =",
		"const ROBOT_FLIGHT_HEIGHT =",
		"const ROBOT_WAVE_DAMPING_HEIGHT =",
		"flightLift",
		"nextFlightAt",
		"function scheduleNextRobotFlight",
		"function updateRobotFlight",
		"robotWaveInfluenceForFlightHeight",
		"height += (hoverDepression + hoverRipple) * flightWaveInfluence",
		"waterY * flightWaveInfluence",
		"normal.lerp(bot.up, 1 - flightWaveInfluence).normalize()",
		"bot.thrusterLight.intensity = 1.4 + (bot.state.flightLift || 0) * 0.9",
	} {
		if !strings.Contains(shader, marker) {
			t.Fatalf("threedee-shader.js missing robot flight/wave damping marker %q", marker)
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
