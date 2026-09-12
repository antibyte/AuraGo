package gamemaker

import "testing"

func TestBuilderScenariosPreserveLifecycleWithoutImposingGenre(t *testing.T) {
	for _, version := range []int{3, 4} {
		plan := &GamePlan{SchemaVersion: version, Template: "minimal", Scenarios: []GameScenario{{ID: "custom_lantern"}}}
		seen := map[string]bool{}
		for _, scenario := range gameScenarios(plan) {
			seen[scenario.ID] = true
		}
		for _, id := range []string{"required_assets", "required_end", "required_restart", "custom_lantern"} {
			if !seen[id] {
				t.Fatalf("schema %d lost %s", version, id)
			}
		}
		if seen["required_input"] != (version == 3) || seen["required_rules"] != (version == 3) || seen["required_primary"] != (version == 3) {
			t.Fatalf("schema %d imposes the wrong starter mechanics", version)
		}
	}
}
