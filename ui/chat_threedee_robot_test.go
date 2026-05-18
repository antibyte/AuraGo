package ui

import (
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
	} {
		if !strings.Contains(shader, marker) {
			t.Fatalf("threedee-shader.js must use local DRACO GLB loading marker %q", marker)
		}
	}
	if strings.Contains(shader, "https://") || strings.Contains(shader, "unpkg.com") || strings.Contains(shader, "cdn.jsdelivr") {
		t.Fatal("ThreeDee robot loader must not depend on remote libraries or assets")
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
