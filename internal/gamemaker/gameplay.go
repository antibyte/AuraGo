package gamemaker

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

type CheckResult struct {
	ID       string         `json:"id"`
	Status   string         `json:"status"`
	Expected string         `json:"expected"`
	Observed string         `json:"observed"`
	Steps    []GameTestStep `json:"steps,omitempty"`
	// Repairable marks an unavailable check whose evidence the game source can
	// supply (missing/blocked target, contact without the measured effect).
	Repairable bool `json:"repairable,omitempty"`
}
type GameObservation struct {
	ID             string             `json:"id"`
	Before         map[string]float64 `json:"before"`
	After          map[string]float64 `json:"after"`
	EvidenceBefore *GameplayEvidence  `json:"evidence_before,omitempty"`
	EvidenceAfter  *GameplayEvidence  `json:"evidence_after,omitempty"`
	TargetRuns     []TargetRun        `json:"target_runs,omitempty"`
	VoxelBefore    *VoxelEvidence     `json:"voxel_before,omitempty"`
	VoxelAfter     *VoxelEvidence     `json:"voxel_after,omitempty"`
}

// TargetRun records bounded input/geometry evidence, never a client pass verdict.
type TargetRun struct {
	Target   string `json:"target"`
	Mode     string `json:"mode"`
	Samples  int    `json:"samples"`
	Inputs   int    `json:"inputs"`
	Contacts int    `json:"contacts"`
	Effects  int    `json:"effects"`
	Reason   string `json:"reason"`
}

func validateGameReport(report PreviewReport) error {
	if len(report.Observations) > 16 || len(report.Images) > 2 {
		return fmt.Errorf("gameplay report exceeds observation/image limit")
	}
	for _, o := range report.Observations {
		if err := validateVoxelEvidence(o.VoxelBefore); err != nil {
			return err
		}
		if err := validateVoxelEvidence(o.VoxelAfter); err != nil {
			return err
		}
		if len(o.TargetRuns) > 8 {
			return fmt.Errorf("too many targeted observations")
		}
		for _, run := range o.TargetRuns {
			if len(run.Target) == 0 || len(run.Target) > 96 || !slices.Contains(targetModes, run.Mode) ||
				!slices.Contains([]string{"complete", "no_target", "blocked", "timeout", "unsupported", "inactive", "host_hidden"}, run.Reason) ||
				run.Samples < 0 || run.Samples > 160 || run.Inputs < 0 || run.Inputs > 1000 || run.Contacts < 0 || run.Contacts > run.Samples || run.Effects < 0 || run.Effects > run.Samples {
				return fmt.Errorf("invalid targeted observation")
			}
		}
		if err := validateGameplayEvidence(o.EvidenceBefore); err != nil {
			return err
		}
		if err := validateGameplayEvidence(o.EvidenceAfter); err != nil {
			return err
		}
		if len(o.ID) > 64 || len(o.Before) > 32 || len(o.After) > 64 {
			return fmt.Errorf("oversized gameplay observation")
		}
		for _, values := range []map[string]float64{o.Before, o.After} {
			for key, value := range values {
				if len(key) > 64 || !finite(value) || value > 1e12 || value < -1e12 {
					return fmt.Errorf("invalid gameplay observation")
				}
			}
		}
	}
	for _, image := range report.Images {
		if len(image) > 700000 {
			return fmt.Errorf("preview image exceeds size limit")
		}
	}
	return nil
}

// A failed optional capture must not discard valid technical observations.
func validPreviewImage(image string) bool {
	if !strings.HasPrefix(image, "data:image/png;base64,") {
		return false
	}
	data, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(image, "data:image/png;base64,"))
	if err != nil {
		return false
	}
	config, err := png.DecodeConfig(bytes.NewReader(data))
	return err == nil && config.Width > 0 && config.Height > 0 && config.Width <= 1920 && config.Height <= 1280
}

var gameMetrics = []string{"player_x", "player_y", "player_distance", "player_right", "stage_count", "actions", "score", "hits", "spawns", "turns", "ticks", "ended", "object_count", "timer_count", "listener_count", "invalid_assets", "assets_used", "elapsed_ms", "aim", "ammo", "reloads", "health", "lives", "goal_remaining", "outcome", "hit_events", "pickup_events", "win_events", "lose_events"}
var gameKeys = []string{"LEFT", "RIGHT", "UP", "DOWN", "W", "A", "S", "D", "SPACE", "R", "P", "ESC", "ENTER", "F", "Q", "E"}
var targetModes = []string{"move", "aim", "reach", "interact", "catch", "avoid", "select"}

// GameMetrics and GameKeys expose the accepted scenario vocabulary so tool
// schemas advertise exactly what validateScenario accepts.
func GameMetrics() []string { return slices.Clone(gameMetrics) }
func GameKeys() []string    { return slices.Clone(gameKeys) }

func validateScenario(s GameScenario) error {
	if len(s.ID) < 1 || len(s.ID) > 64 || strings.HasPrefix(s.ID, "required_") {
		return fmt.Errorf("id must be 1–64 characters and cannot start with required_")
	}
	if !slices.Contains(gameMetrics, s.Metric) {
		return fmt.Errorf("metric must be one of %s", strings.Join(gameMetrics, ", "))
	}
	if !slices.Contains([]string{"increased", "decreased", "changed", "equals", "at_least"}, s.Compare) || !finite(s.Value) {
		return fmt.Errorf("compare must be increased, decreased, changed, equals or at_least with a finite value")
	}
	if len(s.Steps) < 1 || len(s.Steps) > 8 {
		return fmt.Errorf("provide 1–8 steps")
	}
	duration := 0
	for _, step := range s.Steps {
		if !slices.Contains([]string{"key", "pointer", "wait", "observe", "target"}, step.Action) {
			return fmt.Errorf("step action must be key, pointer, wait, observe or target; JavaScript is not accepted")
		}
		if step.Action == "target" {
			if len(step.Target) == 0 || len(step.Target) > 96 || strings.ContainsAny(step.Target, "\x00\r\n") || !slices.Contains(targetModes, step.Mode) || step.MS < 100 {
				return fmt.Errorf("target steps require a bounded node ID or role, mode move/aim/reach/interact/catch/avoid/select and 100–4000 ms")
			}
		} else if step.Target != "" || step.Mode != "" {
			return fmt.Errorf("target and mode require action target")
		}
		if step.Action == "key" && !slices.Contains(gameKeys, step.Key) {
			return fmt.Errorf("unsupported key %q", step.Key)
		}
		if step.MS < 0 || step.MS > 4000 {
			return fmt.Errorf("step ms must be 0–4000")
		}
		duration += step.MS
		if !finite(step.X) || !finite(step.Y) || step.X < 0 || step.X > 1920 || step.Y < 0 || step.Y > 1080 {
			return fmt.Errorf("pointer coordinates must be within 1920×1080 logical pixels")
		}
	}
	if duration > 6000 {
		return fmt.Errorf("scenario exceeds six seconds")
	}
	if hint := scenarioMetricHint(s); hint != "" {
		return errors.New(hint)
	}
	return nil
}

// Targeted movement only establishes position changes. Catch this common
// model-authored mismatch before a browser run, without rewriting the test or
// fabricating primary actions in the game. Other custom input contracts remain
// available; this guard is specific to the built-in move driver.
func scenarioMetricHint(s GameScenario) string {
	if s.Metric != "actions" || (s.Compare != "increased" && s.Compare != "changed") {
		return ""
	}
	move := false
	for _, step := range s.Steps {
		switch step.Action {
		case "target":
			if step.Mode != "move" {
				return ""
			}
			move = true
		case "pointer":
			return ""
		case "key":
			if !slices.Contains([]string{"W", "A", "S", "D", "LEFT", "RIGHT", "UP", "DOWN", "Q", "E"}, step.Key) {
				return ""
			}
		}
	}
	if move {
		return "movement-only target steps do not prove primary actions; measure player_distance/player_x/player_y for movement or send the actual action input. Never increment actions merely to satisfy this check. A position delta alone does not prove steering direction or smoothness"
	}
	return ""
}

// Template minimums are server-owned. The plan can add checks, never remove them.
func requiredScenarios(template string) []GameScenario {
	key := func(name string, ms int) GameTestStep { return GameTestStep{Action: "key", Key: name, MS: ms} }
	check := func(id, metric, compare string, steps ...GameTestStep) GameScenario {
		return GameScenario{ID: "required_" + id, Metric: metric, Compare: compare, Steps: steps}
	}
	target := func(mode, id string) GameTestStep {
		return GameTestStep{Action: "target", Mode: mode, Target: id, MS: 4000}
	}
	if guided3D(template) || template == "three" {
		role := "item"
		if template == "transport" {
			role = "cargo"
		}
		if template == "flight" {
			role = "goal"
		}
		rules := check("rules", "hits", "increased", target("reach", role))
		primary := check("primary", "actions", "increased", key("SPACE", 300))
		if template == "fps" || template == "space" {
			rules.Steps = []GameTestStep{target("aim", "enemy")}
			primary.Steps = []GameTestStep{target("aim", "enemy")}
		}
		checks := []GameScenario{
			check("input", "player_distance", "increased", target("move", "player")), primary, rules,
			check("timed", "ticks", "increased", GameTestStep{Action: "wait", MS: 2100}),
			check("assets", "invalid_assets", "equals", key("A", 150)),
			check("end", "ended", "equals", key("ESC", 150)),
			check("restart", "ended", "equals", key("ESC", 150), key("R", 150), GameTestStep{Action: "wait", MS: 400}, key("R", 150), GameTestStep{Action: "wait", MS: 400}),
		}
		models := check("models", "assets_used", "at_least", GameTestStep{Action: "observe"})
		models.Value = 1
		checks = append(checks, models)
		if template == "fps" {
			checks = append(checks, check("aim", "aim", "changed", key("RIGHT", 350)), check("reload", "reloads", "increased", key("SPACE", 300), key("F", 50), GameTestStep{Action: "wait", MS: 3000}))
		}
		return checks
	}
	input := check("input", "player_distance", "increased", target("move", "player"))
	primary := check("primary", "actions", "increased", key("SPACE", 300))
	rules := check("rules", "hits", "increased", target("reach", "item"))
	if template == "platformer" || template == "topdown" || template == "minimal" {
		rules.Metric = "pickup_events"
	}
	if template == "blocks" {
		rules.Steps = []GameTestStep{target("catch", "ball")}
	}
	if template == "shooter" {
		rules.Steps = []GameTestStep{target("aim", "enemy")}
		primary.Steps = []GameTestStep{target("aim", "enemy")}
	}
	if template == "board" {
		rules.Steps = []GameTestStep{target("select", "cell")}
		input = check("input", "actions", "increased", target("select", "cell"))
		primary.Steps = []GameTestStep{target("select", "cell")}
	}
	if template == "topdown" {
		primary.Steps = []GameTestStep{target("interact", "goal")}
	}
	return []GameScenario{input, primary, rules,
		check("timed", "ticks", "increased", GameTestStep{Action: "wait", MS: 2100}),
		check("late_events", "elapsed_ms", "increased", GameTestStep{Action: "wait", MS: 4000}, GameTestStep{Action: "wait", MS: 2000}),
		check("end", "ended", "equals", key("ESC", 150)),
		check("assets", "invalid_assets", "equals", key("LEFT", 150), key("UP", 150), key("DOWN", 150), key("SPACE", 150)),
		check("restart", "ended", "equals", key("ESC", 150), key("R", 150), GameTestStep{Action: "wait", MS: 400}, key("R", 150), GameTestStep{Action: "wait", MS: 400}),
	}
}

func gameScenarios(plan *GamePlan) []GameScenario {
	if plan.Template == "voxel" {
		return voxelScenarios(plan)
	}
	out := requiredScenarios(plan.Template)
	// Schema 4 compositions use their declared scenarios for special mechanics.
	// An explicit scenario replaces only the starter behavior it actually proves;
	// lifecycle, resource and unrelated gameplay checks remain server-owned.
	if plan.SchemaVersion >= 4 && (plan.Scene != nil || plan.Template == "minimal" || plan.Template == "three") {
		out = slices.DeleteFunc(out, func(s GameScenario) bool {
			return s.ID == "required_input" || s.ID == "required_rules" || s.ID == "required_primary" || s.ID == "required_aim" || s.ID == "required_reload" || s.ID == "required_models"
		})
	} else if plan.SchemaVersion >= 4 && len(plan.Scenarios) > 0 {
		input, primary, rules := customScenarioCoverage(plan.Scenarios)
		out = slices.DeleteFunc(out, func(s GameScenario) bool {
			switch s.ID {
			case "required_input":
				return input
			case "required_primary":
				return primary
			case "required_rules":
				return rules
			default:
				return false
			}
		})
	}
	for i := range out {
		if out[i].ID == "required_end" {
			out[i].Value = 1
		}
	}
	for _, scenario := range plan.Scenarios {
		out = append(out, shooterScenarioInput(plan, scenario))
	}
	return out
}

// customScenarioCoverage lets only driven delta checks replace starter checks;
// equality and thresholds remain additive because they may pass from the start.
func customScenarioCoverage(scenarios []GameScenario) (input, primary, rules bool) {
	for _, scenario := range scenarios {
		driven := slices.ContainsFunc(scenario.Steps, func(step GameTestStep) bool {
			return step.Action == "key" || step.Action == "pointer" || step.Action == "target"
		})
		hasDelta := scenario.Compare == "increased" || scenario.Compare == "decreased" || scenario.Compare == "changed"
		if !driven || !hasDelta {
			continue
		}
		switch scenario.Metric {
		case "player_x", "player_y", "player_distance", "player_right":
			input = true
		case "actions":
			primary = true
		case "score", "hits", "turns", "health", "lives", "goal_remaining", "outcome", "hit_events", "pickup_events", "win_events", "lose_events":
			rules = true
		}
	}
	return input, primary, rules
}

// scenarioExpectation prints what a check expects. Comparisons against the
// before value (increased, decreased, changed) have no threshold, so no value is
// printed; "increased 0" read like a required minimum.
func scenarioExpectation(scenario GameScenario) string {
	switch scenario.Compare {
	case "increased", "decreased", "changed":
		return fmt.Sprintf("%s %s", scenario.Metric, scenario.Compare)
	}
	return fmt.Sprintf("%s %s %g", scenario.Metric, scenario.Compare, scenario.Value)
}

// targetReasonNote explains a driver reason code in plain language. It is
// appended after the unchanged raw code so parsers and tests keep their token.
func targetReasonNote(reason string) string {
	switch reason {
	case "no_target":
		return ": no object with this role was visible in the running game"
	case "timeout":
		return ": the target was seen but the test ran out of time before the effect happened"
	case "blocked":
		return ": no free path to the target was found"
	case "unsupported":
		return ": the running game does not expose this target or its geometry to the test"
	case "inactive":
		return ": the game itself was not active during the test (paused or finished)"
	case "host_hidden":
		return ": the Studio preview was hidden or in the background, so the test could not observe the game; keep the preview visible"
	}
	return ""
}

func compareGameObservations(scenarios []GameScenario, observations []GameObservation) []CheckResult {
	out := make([]CheckResult, 0, len(scenarios))
	for _, scenario := range scenarios {
		check := CheckResult{ID: scenario.ID, Status: "unavailable", Expected: scenarioExpectation(scenario), Observed: "No complete observation", Steps: scenario.Steps}
		var found *GameObservation
		for i := range observations {
			if observations[i].ID == scenario.ID {
				if found != nil {
					found = nil
					break
				}
				found = &observations[i]
			}
		}
		if found != nil {
			before, bok := found.Before[scenario.Metric]
			after, aok := found.After[scenario.Metric]
			if scenario.Metric == "player_distance" {
				x1, bx := found.Before["player_x"]
				y1, by := found.Before["player_y"]
				x2, ax := found.After["player_x"]
				y2, ay := found.After["player_y"]
				before, after = 0, math.Hypot(x2-x1, y2-y1)
				bok, aok = bx && by && finite(x1) && finite(y1), ax && ay && finite(x2) && finite(y2)
			}
			if scenario.Metric == "player_right" {
				// Displacement along the screen-right ground axis observed at the start,
				// so a following camera cannot change the sign during the move.
				x1, bx := found.Before["player_x"]
				y1, by := found.Before["player_y"]
				rx, brx := found.Before["view_right_x"]
				ry, bry := found.Before["view_right_y"]
				x2, ax := found.After["player_x"]
				y2, ay := found.After["player_y"]
				before, after = 0, (x2-x1)*rx+(y2-y1)*ry
				bok = bx && by && brx && bry && finite(x1) && finite(y1) && finite(rx) && finite(ry)
				aok = ax && ay && finite(x2) && finite(y2)
			}
			measured := bok && aok && finite(before) && finite(after)
			if measured {
				passed := false
				switch scenario.Compare {
				case "increased":
					passed = after > before
				case "decreased":
					passed = after < before
				case "changed":
					passed = after != before
				case "equals":
					passed = after == scenario.Value
				case "at_least":
					passed = after >= scenario.Value
				case "not_decreased":
					passed = after >= before-controlsTolerance
				}
				check.Status = "failed"
				if passed {
					check.Status = "passed"
				}
				check.Observed = fmt.Sprintf("before=%g, after=%g", before, after)
				if !passed && scenario.ID == controlsCheckID {
					check.Observed += "; " + controlsRepairHint
				}
				if !passed && scenario.ID == stagesCheckID {
					check.Observed += "; " + stagesRepairHint
				}
				if !passed {
					if hint := scenarioMetricHint(scenario); hint != "" {
						check.Observed += "; check definition: " + hint
					}
				}
				if !passed && scenario.Metric == "hits" {
					for _, metric := range []string{"actions", "spawns", "hit_events", "ended"} {
						first, a := found.Before[metric]
						last, b := found.After[metric]
						if a && b && finite(first) && finite(last) {
							check.Observed += fmt.Sprintf("; %s=%g->%g", metric, first, last)
						}
					}
				}
			}
			if scenario.ID == "required_restart" {
				for _, metric := range []string{"score", "actions", "hits", "turns", "object_count", "timer_count", "listener_count"} {
					first, a := found.Before[metric]
					last, b := found.After[metric]
					one, c := found.After["restart1_"+metric]
					if !a || !b || !c {
						check.Status = "unavailable"
						check.Observed = "Missing restart observation for " + metric
						break
					}
					if first != last || first != one {
						check.Status = "failed"
						check.Observed = fmt.Sprintf("%s initial=%g, restart1=%g, restart2=%g", metric, first, one, last)
						break
					}
				}
			}
			targetSteps := []GameTestStep{}
			for _, step := range scenario.Steps {
				if step.Action == "target" {
					targetSteps = append(targetSteps, step)
				}
			}
			if len(targetSteps) > 0 {
				if len(found.TargetRuns) != len(targetSteps) {
					check.Status = "unavailable"
					check.Observed = "Missing targeted input/geometry evidence"
				} else {
					contacts, effects := 0, 0
					// A missing measurement or an undriven run is a harness gap unless
					// the driver names a reason the running game itself caused.
					harness, sourceDefect, hostHidden := !measured, false, false
					for i, run := range found.TargetRuns {
						if run.Target != targetSteps[i].Target || run.Mode != targetSteps[i].Mode {
							check.Status = "unavailable"
							check.Observed = "Mismatched target evidence"
							effects = -1
							break
						}
						contacts += run.Contacts
						if run.Reason == "host_hidden" {
							// The Studio window, not the game, stopped the run: the evidence is
							// incomplete and no source edit can supply it.
							check.Status = "unavailable"
							hostHidden = true
						}
						gameReason := run.Reason == "unsupported" || run.Reason == "inactive" || run.Reason == "no_target" || run.Reason == "blocked"
						if gameReason {
							check.Status = "unavailable"
							sourceDefect = true
						}
						if run.Inputs == 0 || run.Samples < 2 {
							check.Status = "unavailable"
							harness = harness || !gameReason
						} else if check.Status == "passed" && run.Effects == 0 {
							check.Status = "unavailable"
						}
						if run.Inputs > 0 && run.Samples >= 2 {
							effects += run.Effects
						}
						check.Observed += fmt.Sprintf("; target=%s %s (inputs=%d, contacts=%d, effects=%d)", run.Target, run.Reason, run.Inputs, run.Contacts, run.Effects) + targetReasonNote(run.Reason)
					}
					if effects >= 0 && (check.Status == "passed" && effects == 0 || check.Status == "failed" && (contacts == 0 || scenario.Metric != "hits" && scenario.Metric != "actions" && scenario.Metric != "player_distance")) {
						check.Status = "unavailable"
					}
					check.Repairable = check.Status == "unavailable" && effects >= 0 && !hostHidden && (sourceDefect || !harness)
				}
			} else if check.Status == "failed" && slices.Contains([]string{"hits", "hit_events", "pickup_events", "health", "lives", "goal_remaining", "outcome", "win_events", "lose_events"}, scenario.Metric) && scenario.ID != "required_end" {
				check.Status = "unavailable"
				check.Repairable = true
				check.Observed += "; blind input did not establish the required gameplay opportunity; use an observed target and the metric for the actual action; blind input means the test only pressed generic controls and never saw a target to act on"
			}
		}
		if scenario.inputNote != "" {
			check.Observed += "; " + scenario.inputNote
		}
		out = append(out, check)
	}
	return out
}

// StopAfterValidation ends a repair round after its first check. Building may
// continue after core checks until repairs are exhausted or feedback is absent.
// Read-only new-game exploration yields to implementation recovery before the
// job deadline. Results from preceding rounds cannot end a new round.

// Scene-backed Three games expose the same bounded observation contract.
func sceneBackedGame(plan GamePlan) bool {
	return plan.Template == "voxel" || guided3D(plan.Template) || plan.Template == "three" && plan.SchemaVersion >= 4 && plan.Scene != nil
}

const maxTargetedGameMakerChecks = 16

func normalizeTargetedGameMakerChecks(checkIDs []string) ([]string, error) {
	if len(checkIDs) > maxTargetedGameMakerChecks {
		return nil, fmt.Errorf("check_ids must contain at most %d existing checks", maxTargetedGameMakerChecks)
	}
	out := make([]string, 0, len(checkIDs))
	seen := make(map[string]bool, len(checkIDs))
	for _, raw := range checkIDs {
		id := strings.TrimSpace(raw)
		if id == "" || len(id) > 64 || seen[id] {
			return nil, fmt.Errorf("check_ids must contain unique nonempty check IDs")
		}
		seen[id] = true
		out = append(out, id)
	}
	return out, nil
}

func selectTargetedGameMakerScenarios(scenarios []GameScenario, checkIDs []string) ([]GameScenario, error) {
	if len(checkIDs) == 0 {
		return append([]GameScenario(nil), scenarios...), nil
	}
	requested := make(map[string]bool, len(checkIDs))
	for _, id := range checkIDs {
		requested[id] = true
	}
	selected := make([]GameScenario, 0, len(checkIDs))
	for _, scenario := range scenarios {
		if requested[scenario.ID] {
			selected = append(selected, scenario)
		}
	}
	if len(selected) != len(checkIDs) {
		return nil, fmt.Errorf("check_ids must reference existing checks")
	}
	return selected, nil
}

// sceneBackedCurrentAt accepts a plan-bound scene or the canonical staged scene
// written later by scene_set/scene_patch. The caller may already hold Service.mu,
// so this helper never resolves a job through JobDirectory.
func sceneBackedCurrentAt(stage string, plan *GamePlan) bool {
	if plan != nil && sceneBackedGame(*plan) {
		return true
	}
	data, err := os.ReadFile(filepath.Join(stage, filepath.FromSlash(SceneFilePath)))
	if err != nil || bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		return false
	}
	scene, err := DecodeSceneJSON(data)
	return err == nil && scene.Dimension == "3d"
}

func (s *Service) sceneBackedCurrent(ctx context.Context, jobID string, plan *GamePlan) bool {
	stage, err := s.JobDirectory(jobID)
	return err == nil && sceneBackedCurrentAt(stage, plan)
}

// ValidateJobScope is the legacy build-only entry point. Scope/check IDs no longer run gameplay checks.
func (s *Service) ValidateJobScope(ctx context.Context, jobID, scope string, requestedCheckIDs ...string) BuildResult {
	return s.BuildJob(ctx, jobID)
}
