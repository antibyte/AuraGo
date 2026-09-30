package server

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"aurago/internal/gamemaker"
)

func TestGameMakerPlanningContextNamesBaseChecks(t *testing.T) {
	for _, dimension := range []string{"2d", "3d"} {
		data := compactGameMakerContext(gamemaker.JobRun{Stage: "planning", Project: gamemaker.Project{Dimension: dimension}})
		encoded, err := json.Marshal(data)
		if err != nil {
			t.Fatal(err)
		}
		text := string(encoded)
		want, foreign := "reach item", `"fps"`
		if dimension == "3d" {
			want, foreign = "reach cargo", `"platformer"`
		}
		if !strings.Contains(text, `"base_checks"`) || !strings.Contains(text, want) || strings.Contains(text, foreign) {
			t.Errorf("%s planning context = %s", dimension, text)
		}
		if contract, _ := data["planning_contract"].(string); !strings.Contains(contract, "Declared scenarios replace") {
			t.Errorf("%s planning contract does not explain base checks: %q", dimension, contract)
		}
	}
	voxel := compactGameMakerContext(gamemaker.JobRun{Stage: "planning", Project: gamemaker.Project{Dimension: "3d", Variant: "voxel"}})
	if _, exists := voxel["base_checks"]; exists {
		t.Error("voxel planning received sprite/guided base checks")
	}
}

func TestGameMakerBudgetIsStableAndPhaseSpecific(t *testing.T) {
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(29*time.Minute+50*time.Second))
	defer cancel()
	first := gameMakerBudget(ctx, 50, gamemaker.JobRun{Stage: "building"})
	second := gameMakerBudget(ctx, 50, gamemaker.JobRun{Stage: "building"})
	if first["tool_calls"] != 50 || first["seconds_remaining"] != second["seconds_remaining"] {
		t.Fatalf("budget is unstable: %+v / %+v", first, second)
	}
	if seconds, _ := first["seconds_remaining"].(int); seconds%30 != 0 || seconds < 1700 || seconds > 1800 {
		t.Fatalf("seconds_remaining = %v", first["seconds_remaining"])
	}
	guidance := map[string]string{}
	for name, run := range map[string]gamemaker.JobRun{
		"planning": {Stage: "planning"},
		"repair":   {Stage: "repair"},
		"new":      {Stage: "building"},
		"edit":     {Stage: "building", Job: gamemaker.Job{BaseRevision: 3}},
	} {
		guidance[name], _ = gameMakerBudget(context.Background(), 40, run)["guidance"].(string)
	}
	if !strings.Contains(guidance["planning"], "set_design") || !strings.Contains(guidance["repair"], "validate once") ||
		!strings.Contains(guidance["new"], "unchanged") || strings.Contains(guidance["edit"], "unchanged") {
		t.Fatalf("guidance = %+v", guidance)
	}
	// The entry file already travels in current_sources; do not ask for a read.
	for _, name := range []string{"new", "edit", "repair"} {
		if !strings.Contains(guidance[name], "current_sources") {
			t.Fatalf("%s guidance does not point at current_sources: %q", name, guidance[name])
		}
	}
	if _, exists := gameMakerBudget(context.Background(), 40, gamemaker.JobRun{Stage: "building"})["seconds_remaining"]; exists {
		t.Error("a context without a deadline reported remaining time")
	}
}
