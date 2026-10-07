package gamemaker

import "testing"

func TestCustomScenarioCoverageRequiresAChangeAssertion(t *testing.T) {
	scenario := func(metric, compare string, step GameTestStep) GameScenario {
		return GameScenario{Metric: metric, Compare: compare, Steps: []GameTestStep{step}}
	}
	key := GameTestStep{Action: "key", Key: "RIGHT", MS: 300}
	target := GameTestStep{Action: "target", Target: "enemy", Mode: "interact", MS: 1000}
	tests := []struct {
		name                  string
		scenario              GameScenario
		input, primary, rules bool
	}{
		{name: "score equals baseline", scenario: scenario("score", "equals", key)},
		{name: "health threshold", scenario: scenario("health", "at_least", target)},
		{name: "actions equals baseline", scenario: scenario("actions", "equals", key)},
		{name: "position threshold", scenario: scenario("player_distance", "at_least", key)},
		{name: "passive score change", scenario: GameScenario{Metric: "score", Compare: "increased", Steps: []GameTestStep{{Action: "wait", MS: 500}}}},
		{name: "score increased", scenario: scenario("score", "increased", target), rules: true},
		{name: "score changed", scenario: scenario("score", "changed", target), rules: true},
		{name: "hits changed", scenario: scenario("hits", "changed", target), rules: true},
		{name: "health decreased", scenario: scenario("health", "decreased", target), rules: true},
		{name: "lives decreased", scenario: scenario("lives", "decreased", target), rules: true},
		{name: "position decreased", scenario: scenario("player_x", "decreased", key), input: true},
		{name: "distance changed", scenario: scenario("player_distance", "changed", key), input: true},
		{name: "actions changed", scenario: scenario("actions", "changed", target), primary: true},
		{name: "actions decreased", scenario: scenario("actions", "decreased", target), primary: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			input, primary, rules := customScenarioCoverage([]GameScenario{tc.scenario})
			if input != tc.input || primary != tc.primary || rules != tc.rules {
				t.Fatalf("customScenarioCoverage() = (%t, %t, %t), want (%t, %t, %t)", input, primary, rules, tc.input, tc.primary, tc.rules)
			}
		})
	}
}

func TestGameScenariosKeepBaselineAssertionsAdditive(t *testing.T) {
	key := GameTestStep{Action: "key", Key: "RIGHT", MS: 300}
	target := GameTestStep{Action: "target", Target: "switch", Mode: "interact", MS: 1000}
	plan := &GamePlan{SchemaVersion: 4, Template: "topdown", Scenarios: []GameScenario{
		{ID: "score_starts_at_zero", Metric: "score", Compare: "equals", Value: 0, Steps: []GameTestStep{key}},
		{ID: "health_is_full", Metric: "health", Compare: "at_least", Value: 100, Steps: []GameTestStep{target}},
		{ID: "actions_start_at_zero", Metric: "actions", Compare: "equals", Value: 0, Steps: []GameTestStep{key}},
		{ID: "position_threshold", Metric: "player_distance", Compare: "at_least", Value: 0, Steps: []GameTestStep{key}},
	}}
	seen := map[string]bool{}
	for _, check := range gameScenarios(plan) {
		seen[check.ID] = true
	}
	for _, id := range []string{"required_input", "required_primary", "required_rules", "required_end", "required_restart", "score_starts_at_zero", "health_is_full", "actions_start_at_zero", "position_threshold"} {
		if !seen[id] {
			t.Errorf("baseline assertions must remain additive; missing %s", id)
		}
	}
}

func TestGameScenariosReplaceChecksWithDrivenChanges(t *testing.T) {
	plan := &GamePlan{SchemaVersion: 4, Template: "topdown", Scenarios: []GameScenario{
		{ID: "walk_to_switch", Metric: "player_distance", Compare: "increased", Steps: []GameTestStep{{Action: "target", Target: "switch", Mode: "move", MS: 1000}}},
		{ID: "toggle_switch", Metric: "actions", Compare: "changed", Steps: []GameTestStep{{Action: "target", Target: "switch", Mode: "interact", MS: 1000}}},
		{ID: "open_door", Metric: "goal_remaining", Compare: "decreased", Steps: []GameTestStep{{Action: "target", Target: "door", Mode: "interact", MS: 1000}}},
	}}
	seen := map[string]bool{}
	for _, check := range gameScenarios(plan) {
		seen[check.ID] = true
	}
	for _, id := range []string{"required_input", "required_primary", "required_rules"} {
		if seen[id] {
			t.Errorf("meaningful custom change should replace %s", id)
		}
	}
	for _, id := range []string{"required_end", "required_restart", "walk_to_switch", "toggle_switch", "open_door"} {
		if !seen[id] {
			t.Errorf("required or custom check %s was lost", id)
		}
	}
}
