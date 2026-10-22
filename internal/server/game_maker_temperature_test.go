package server

import (
	"testing"

	"aurago/internal/config"
)

func TestGameMakerPhaseTemperatureDefaultsAndOverrides(t *testing.T) {
	cfg := &config.Config{}
	if got := gameMakerPhaseTemperature(cfg, "planning"); got != gameMakerPlanningTemperature {
		t.Fatalf("planning temperature = %v, want %v", got, gameMakerPlanningTemperature)
	}
	for _, stage := range []string{"building", "repair", "visual"} {
		if got := gameMakerPhaseTemperature(cfg, stage); got != gameMakerBuildingTemperature {
			t.Fatalf("%s temperature = %v, want %v", stage, got, gameMakerBuildingTemperature)
		}
	}

	planning, building := 0.9, 0.05
	cfg.GameMaker.PlanningTemperature = &planning
	cfg.GameMaker.BuildingTemperature = &building
	if got := gameMakerPhaseTemperature(cfg, "planning"); got != 0.9 {
		t.Fatalf("configured planning temperature = %v", got)
	}
	if got := gameMakerPhaseTemperature(cfg, "building"); got != 0.05 {
		t.Fatalf("configured building temperature = %v", got)
	}

	// Values outside the agent loop's range are clamped instead of rejected.
	low, high := -3.0, 9.0
	cfg.GameMaker.PlanningTemperature = &high
	cfg.GameMaker.BuildingTemperature = &low
	if got := gameMakerPhaseTemperature(cfg, "planning"); got != 1.5 {
		t.Fatalf("high temperature was not clamped: %v", got)
	}
	if got := gameMakerPhaseTemperature(cfg, "building"); got != 0.05 {
		t.Fatalf("low temperature was not clamped: %v", got)
	}
}
