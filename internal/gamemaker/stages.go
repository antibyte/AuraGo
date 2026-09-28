package gamemaker

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Plans often promise several areas while the build ships one layout. Declared
// stages become a bounded, observable commitment: the running game must provide
// that many distinct stage layouts. Waves inside one arena remain features.
const (
	stagesCheckID    = "required_stages"
	maxDesignStages  = 6
	stagesRepairHint = "the accepted plan declares more stages than the game provides. Give every planned stage its own layout and challenge: 2D configureLevels([{id,title},...]) and build each layout in setup() from this.levelIndex (or scene levels whose nodes carry level_id); 3D config.levels:[{id,title,objects:[...],goal}] with different objects per level. end(true)/api.win() then offers Continue to the next stage. Never clone a layout or rename a level to add a stage"
)

func validateStages(stages []string) error {
	if len(stages) > maxDesignStages {
		return fmt.Errorf("list at most %d stages; put waves or escalating rules inside one arena in features", maxDesignStages)
	}
	seen := map[string]bool{}
	for _, stage := range stages {
		name := strings.ToLower(strings.TrimSpace(stage))
		if len(name) < 3 || len(stage) > 200 || seen[name] {
			return fmt.Errorf("each stage needs a unique 3–200 character description of its distinct layout and new challenge")
		}
		seen[name] = true
	}
	return nil
}

// stagesScenario observes the running game's distinct stage count when the plan
// declares at least two stages and the installed helper can report them.
func stagesScenario(plan *GamePlan, runtimeVersion string) (GameScenario, bool) {
	if plan == nil || len(plan.Stages) < 2 {
		return GameScenario{}, false
	}
	if is3DTemplate(plan.Template) {
		if !guided3D(plan.Template) || !threeRuntimeAtLeast(runtimeVersion, 4) {
			return GameScenario{}, false
		}
	} else if !phaserRuntimeAtLeast(runtimeVersion, 2) {
		return GameScenario{}, false
	}
	return GameScenario{ID: stagesCheckID, Metric: "stage_count", Compare: "at_least", Value: float64(len(plan.Stages)),
		Steps: []GameTestStep{{Action: "observe"}}}, true
}

func phaserRuntimeAtLeast(version string, minimum int) bool {
	n, err := strconv.Atoi(strings.TrimPrefix(version, "phaser-"))
	return strings.HasPrefix(version, "phaser-") && err == nil && n >= minimum
}

// reviewStageLayouts completes the 2D stage check. Phaser builds one layout at a
// time, so the runtime can only count configured levels; they are distinct only
// when source branches on the level or the scene assigns nodes to levels. The 3D
// helper already counts distinct layouts itself.
func reviewStageLayouts(stage string, plan *GamePlan, checks []CheckResult) {
	if plan == nil || is3DTemplate(plan.Template) {
		return
	}
	for i := range checks {
		if checks[i].ID != stagesCheckID || checks[i].Status != "passed" || stageLayoutsVary(stage, len(plan.Stages)) {
			continue
		}
		checks[i].Status = "failed"
		checks[i].Observed += "; levels are configured, but no source reads this.levelIndex and no scene nodes carry level_id, so every stage repeats one layout; " + stagesRepairHint
	}
}

func stageLayoutsVary(stage string, stages int) bool {
	if data, err := os.ReadFile(filepath.Join(stage, "src", "scene.json")); err == nil {
		var scene struct {
			Nodes []struct {
				LevelID string `json:"level_id"`
			} `json:"nodes"`
		}
		if json.Unmarshal(data, &scene) == nil {
			levels := map[string]bool{}
			for _, node := range scene.Nodes {
				if node.LevelID != "" {
					levels[node.LevelID] = true
				}
			}
			if len(levels) >= stages {
				return true
			}
		}
	}
	entries, err := os.ReadDir(filepath.Join(stage, "src"))
	if err != nil {
		return false
	}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || name == "common.ts" || !strings.HasSuffix(name, ".ts") {
			continue
		}
		if data, err := os.ReadFile(filepath.Join(stage, "src", name)); err == nil && strings.Contains(string(data), "levelIndex") {
			return true
		}
	}
	return false
}
