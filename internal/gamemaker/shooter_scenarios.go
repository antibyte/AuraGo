package gamemaker

// shooterScenarioInput compiles a redundant blind firing probe into the same
// observed-enemy input used by the server's shooter rule check. A hit requirement
// needs a shooting opportunity, not a lucky enemy spawn in the player's column.
// Keep the accepted plan/source, metric, comparison and total duration intact.
// Explicit targets, navigation, pointer input and scene compositions are custom
// contracts: never guess a replacement target for them.
func shooterScenarioInput(plan *GamePlan, scenario GameScenario) GameScenario {
	if plan.Template != "shooter" || plan.Scene != nil || scenario.Metric != "hits" || scenario.Compare != "increased" {
		return scenario
	}
	firstShot, duration, total := -1, 0, 0
	for i, step := range scenario.Steps {
		if step.MS < 0 || step.MS > 4000 {
			return scenario
		}
		switch step.Action {
		case "key":
			if step.Key != "SPACE" {
				return scenario
			}
			if firstShot == -1 {
				firstShot = i
			}
		case "wait", "observe":
		default:
			return scenario
		}
		total += step.MS
		if firstShot >= 0 {
			duration += step.MS
		}
	}
	if firstShot < 0 || duration < 100 || total > 6000 || len(scenario.Steps) > 8 {
		return scenario
	}
	steps := append([]GameTestStep(nil), scenario.Steps[:firstShot]...)
	active := min(duration, 4000)
	steps = append(steps, GameTestStep{Action: "target", Target: "enemy", Mode: "aim", MS: active})
	if duration > active {
		steps = append(steps, GameTestStep{Action: "wait", MS: duration - active})
	}
	scenario.Steps = steps
	scenario.inputNote = "input adapted: blind SPACE firing replaced by observed enemy aiming within the original time budget; real hits and target effects are still required"
	return scenario
}
