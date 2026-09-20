package gamemaker

import "testing"

func TestRequiredRulesUseTheMechanicsCounter(t *testing.T) {
	for template, metric := range map[string]string{"platformer": "pickup_events", "topdown": "pickup_events", "minimal": "pickup_events", "blocks": "hits", "shooter": "hits", "board": "hits", "fps": "hits"} {
		for _, scenario := range requiredScenarios(template) {
			if scenario.ID == "required_rules" && scenario.Metric != metric {
				t.Errorf("%s rules use %s, want %s", template, scenario.Metric, metric)
			}
		}
	}
}

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

func TestCustomScenariosReplaceOnlyCoveredStarterBehavior(t *testing.T) {
	plan := &GamePlan{SchemaVersion: 4, Template: "topdown", Scenarios: []GameScenario{
		{ID: "walk_to_switch", Metric: "player_distance", Compare: "increased", Steps: []GameTestStep{{Action: "target", Target: "switch", Mode: "move", MS: 1000}}},
		{ID: "toggle_switch", Metric: "actions", Compare: "increased", Steps: []GameTestStep{{Action: "target", Target: "switch", Mode: "interact", MS: 1000}}},
		{ID: "open_door", Metric: "goal_remaining", Compare: "decreased", Steps: []GameTestStep{{Action: "target", Target: "door", Mode: "interact", MS: 1000}}},
	}}
	seen := map[string]bool{}
	for _, scenario := range gameScenarios(plan) {
		seen[scenario.ID] = true
	}
	for _, id := range []string{"required_input", "required_primary", "required_rules"} {
		if seen[id] {
			t.Fatalf("covered starter check %s was retained", id)
		}
	}
	for _, id := range []string{"required_assets", "required_end", "required_restart", "walk_to_switch", "toggle_switch", "open_door"} {
		if !seen[id] {
			t.Fatalf("required or custom check %s was lost", id)
		}
	}

	plan.Scenarios = plan.Scenarios[:1]
	seen = map[string]bool{}
	for _, scenario := range gameScenarios(plan) {
		seen[scenario.ID] = true
	}
	if seen["required_input"] || !seen["required_primary"] || !seen["required_rules"] {
		t.Fatalf("partial coverage removed unrelated checks: %+v", seen)
	}

	plan.Scenarios = []GameScenario{{ID: "ambient_score", Metric: "score", Compare: "increased", Steps: []GameTestStep{{Action: "wait", MS: 1000}}}}
	seen = map[string]bool{}
	for _, scenario := range gameScenarios(plan) {
		seen[scenario.ID] = true
	}
	if !seen["required_rules"] {
		t.Fatalf("passive observation replaced a driven gameplay check: %+v", seen)
	}
}
