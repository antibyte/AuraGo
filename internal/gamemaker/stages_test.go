package gamemaker

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestDesignStagesBecomePlanCommitments(t *testing.T) {
	t.Parallel()
	s := newTestService(t)
	project := Project{Dimension: "2d"}
	design := ExampleGameDesign(project)
	if len(design.Stages) < 2 {
		t.Fatal("the design example must demonstrate stages")
	}
	plan, err := s.planFromDesign(context.Background(), "", project, design)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(plan.Stages, "|") != strings.Join(design.Stages, "|") {
		t.Fatalf("plan stages = %v", plan.Stages)
	}
	for name, stages := range map[string][]string{
		"too many":  {"a1 x", "b2 x", "c3 x", "d4 x", "e5 x", "f6 x", "g7 x"},
		"duplicate": {"Grove path", " grove path "},
		"too short": {"ok"},
	} {
		d := design
		d.Stages = stages
		issues := s.designIssues(project, d)
		if len(issues) == 0 || issues[0].Path != "design.stages" {
			t.Errorf("%s: issues = %+v", name, issues)
		}
	}
}

func TestStageCheckNeedsTwoStagesAndCapableHelper(t *testing.T) {
	t.Parallel()
	two := []string{"First area", "Second area"}
	for _, tc := range []struct {
		template, version string
		stages            []string
		ok                bool
	}{
		{"platformer", "phaser-2", two, true},
		{"platformer", "phaser-2", two[:1], false},
		{"platformer", "", two, false},
		{"exploration", "three-4", two, true},
		{"exploration", "three-3", two, false},
		{"three", "three-4", two, false},
	} {
		scenario, ok := stagesScenario(&GamePlan{Template: tc.template, Stages: tc.stages}, tc.version)
		if ok != tc.ok || ok && (scenario.Value != 2 || scenario.Metric != "stage_count") {
			t.Errorf("%s/%s/%d: ok=%v %+v", tc.template, tc.version, len(tc.stages), ok, scenario)
		}
	}
}

func TestStageLayoutsNeedLevelSpecificSource(t *testing.T) {
	t.Parallel()
	plan := &GamePlan{Template: "blocks", Stages: []string{"First area", "Second area"}}
	passed := func() []CheckResult { return []CheckResult{{ID: stagesCheckID, Status: "passed"}} }
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "src"), 0o750); err != nil {
		t.Fatal(err)
	}
	write := func(name, content string) {
		if err := os.WriteFile(filepath.Join(dir, "src", name), []byte(content), 0o640); err != nil {
			t.Fatal(err)
		}
	}
	write("common.ts", "levelIndex=0") // the helper itself never proves authored layouts
	write("main.ts", "this.configureLevels([{id:'a'},{id:'b'}]);")
	checks := passed()
	reviewStageLayouts(dir, plan, checks)
	if checks[0].Status != "failed" || !strings.Contains(checks[0].Observed, "levelIndex") {
		t.Fatalf("cloned 2D levels passed: %+v", checks[0])
	}
	write("scene.json", `{"nodes":[{"id":"a","level_id":"one"},{"id":"b","level_id":"two"}]}`)
	checks = passed()
	reviewStageLayouts(dir, plan, checks)
	if checks[0].Status != "passed" {
		t.Fatalf("scene levels with their own nodes failed: %+v", checks[0])
	}
	if err := os.Remove(filepath.Join(dir, "src", "scene.json")); err != nil {
		t.Fatal(err)
	}
	write("main.ts", "const rows=this.levelIndex===0?[1]:[1,2];")
	checks = passed()
	reviewStageLayouts(dir, plan, checks)
	if checks[0].Status != "passed" {
		t.Fatalf("level-specific source failed: %+v", checks[0])
	}
	three := []CheckResult{{ID: stagesCheckID, Status: "passed"}}
	reviewStageLayouts(t.TempDir(), &GamePlan{Template: "exploration", Stages: plan.Stages}, three)
	if three[0].Status != "passed" {
		t.Fatal("3D stage evidence is counted by the helper and needs no source review")
	}
}

// New role-based guided 3D starters ship three distinct stages whose later layouts
// scatter the objective instead of lining it up along one axis.
func TestGuided3DStartersShipDistinctStages(t *testing.T) {
	t.Parallel()
	objective := map[string]string{"exploration": "item", "fps": "enemy", "space": "enemy", "flight": "goal", "transport": "cargo"}
	for base, role := range objective {
		plan := ExampleGamePlan(Project{Dimension: "3d"})
		plan.Template, plan.Gameplay = base, &GameSettings{Goal: 5, Speed: 5, Duration: 0}
		plan.Assets = nil
		for _, a := range defaultModelRoles(base) {
			plan.Assets = append(plan.Assets, PlanAsset{Role: a.Role, PackID: a.PackID, Version: "1", AssetID: a.AssetID, Scale: 1})
		}
		sources, err := threeTemplateSources(plan)
		if err != nil {
			t.Fatal(base, err)
		}
		match := regexp.MustCompile(`startGame\((\{.*\})\);`).FindSubmatch(sources["main.ts"])
		var config struct {
			Levels []struct {
				Objects []struct {
					Role string    `json:"role"`
					At   []float64 `json:"at"`
				} `json:"objects"`
				Goal int `json:"goal"`
			} `json:"levels"`
		}
		if match == nil || json.Unmarshal(match[1], &config) != nil || len(config.Levels) != 3 {
			t.Fatalf("%s: starter lacks three stages", base)
		}
		first, _ := json.Marshal(config.Levels[1].Objects)
		second, _ := json.Marshal(config.Levels[2].Objects)
		if string(first) == string(second) {
			t.Fatalf("%s: later stages repeat one layout", base)
		}
		for i, level := range config.Levels[1:] {
			xs, found := map[float64]bool{}, 0
			for _, o := range level.Objects {
				if o.Role == role {
					found++
					xs[o.At[0]] = true
				}
			}
			if found == 0 || base != "transport" && (level.Goal != found || len(xs) < 3) {
				t.Errorf("%s stage %d: %d %s objects across %d x positions, goal %d", base, i+2, found, role, len(xs), level.Goal)
			}
		}
	}
}
