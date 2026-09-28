package gamemaker

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Swapped horizontal controls are a recurring generated-game defect: RIGHT/D
// must never move the player toward screen-left. The check fails only on that
// contradiction, so rotation-based, vertical-only or pointer controls stay valid.
const (
	controlsCheckID    = "required_controls"
	controlsTolerance  = 0.05
	controlsRepairHint = "RIGHT/D moved the player toward screen-left. Map RIGHT/D to screen-right and LEFT/A to screen-left: in 2D RIGHT increases x; in 3D derive horizontal movement from the camera's right vector (a camera looking along +Z sees world -X on its right)"
)

// controlsScenario returns the check when the installed helper can report the
// screen-right ground axis: every 2D base, and guided 3D from helper three-4 on.
// Older 3D helpers keep their existing checks instead of an unavailable result.
func controlsScenario(template, runtimeVersion string) (GameScenario, bool) {
	key := "RIGHT"
	if is3DTemplate(template) {
		if !guided3D(template) || !threeRuntimeAtLeast(runtimeVersion, 4) {
			return GameScenario{}, false
		}
		key = "D"
	}
	return GameScenario{ID: controlsCheckID, Metric: "player_right", Compare: "not_decreased",
		Steps: []GameTestStep{{Action: "key", Key: key, MS: 400}}}, true
}

// withRequiredScenario keeps server checks ahead of the plan's own scenarios.
func withRequiredScenario(scenarios []GameScenario, required GameScenario) []GameScenario {
	at := len(scenarios)
	for i, scenario := range scenarios {
		if !strings.HasPrefix(scenario.ID, "required_") {
			at = i
			break
		}
	}
	return append(scenarios[:at:at], append([]GameScenario{required}, scenarios[at:]...)...)
}

func threeRuntimeAtLeast(version string, minimum int) bool {
	n, err := strconv.Atoi(strings.TrimPrefix(version, "three-"))
	return strings.HasPrefix(version, "three-") && err == nil && n >= minimum
}

// installedRuntimeVersion reads the descriptor of the helper installed in a job
// directory, never the bundled template. Missing or custom helpers return "".
func installedRuntimeVersion(stage string) string {
	data, err := os.ReadFile(filepath.Join(stage, "src", "common.ts"))
	if err != nil {
		return ""
	}
	for i, line := range strings.SplitN(string(data), "\n", 9) {
		line = strings.TrimSpace(line)
		if i >= 8 || !strings.HasPrefix(line, runtimeContractPrefix) || len(line) > 3000 {
			continue
		}
		var descriptor struct {
			Version string `json:"version"`
		}
		if json.Unmarshal([]byte(strings.TrimPrefix(line, runtimeContractPrefix)), &descriptor) == nil {
			return descriptor.Version
		}
	}
	return ""
}
