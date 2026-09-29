package gamemaker

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// ValidationCheck discloses one check that a full validation runs. It describes
// the measurement only; verdicts always come from observed browser evidence.
type ValidationCheck struct {
	ID      string         `json:"id"`
	Metric  string         `json:"metric"`
	Compare string         `json:"compare"`
	Value   float64        `json:"value,omitempty"`
	Steps   []GameTestStep `json:"steps"`
	Owner   string         `json:"owner"`
}

const validationPlanNote = "Checks use normal keyboard/pointer input only. Each target step needs a real object carrying that exact role or ID, and the real contact must change the measured metric. A check stays unavailable when the running game offers no such target or effect; counters changed without a physical effect never pass."

// validationScenarios is the single source for the checks of a staged build. The
// browser run and the plan disclosed to the agent must never diverge.
func validationScenarios(stage string, plan *GamePlan) []GameScenario {
	if plan == nil {
		return nil
	}
	current := *plan
	if current.Template == "voxel" {
		if data, err := os.ReadFile(filepath.Join(stage, "src", "voxel.json")); err == nil {
			if definition, err := ParseVoxelDefinition(data); err == nil {
				current.Voxel = definition
			}
		}
	}
	scenarios := gameScenarios(&current)
	// The driver accepts at most 16 checks; accepted plans may already fill them.
	runtimeVersion := installedRuntimeVersion(stage)
	if controls, ok := controlsScenario(current.Template, runtimeVersion); ok && len(scenarios) < 16 {
		scenarios = withRequiredScenario(scenarios, controls)
	}
	if stages, ok := stagesScenario(&current, runtimeVersion); ok && len(scenarios) < 16 {
		scenarios = withRequiredScenario(scenarios, stages)
	}
	return scenarios
}

func describeScenarios(scenarios []GameScenario) ([]ValidationCheck, []string) {
	checks := make([]ValidationCheck, 0, len(scenarios))
	targets := []string{}
	for _, scenario := range scenarios {
		owner := "plan"
		if strings.HasPrefix(scenario.ID, "required_") {
			owner = "server"
		}
		checks = append(checks, ValidationCheck{ID: scenario.ID, Metric: scenario.Metric, Compare: scenario.Compare, Value: scenario.Value, Steps: scenario.Steps, Owner: owner})
		for _, step := range scenario.Steps {
			if step.Action == "target" && !slices.Contains(targets, step.Target) {
				targets = append(targets, step.Target)
			}
		}
	}
	return checks, targets
}

// ValidationPlan lists the checks publication of this job requires, so the
// agent can build toward them instead of discovering them through failures.
func (s *Service) ValidationPlan(ctx context.Context, jobID string) map[string]any {
	stage, err := s.JobDirectory(jobID)
	if err != nil {
		return nil
	}
	plan, err := s.GetPlan(ctx, jobID)
	if err != nil || plan == nil {
		return nil
	}
	project, _, err := s.ProjectForJob(ctx, jobID)
	if err != nil {
		return nil
	}
	if project.Dimension == "3d" && !sceneBackedCurrentAt(stage, plan) {
		return map[string]any{"scope": "startup", "checks": []ValidationCheck{}, "note": "Free-code three has startup checks only: a visible canvas and three seconds without runtime errors. Gameplay stays unverified."}
	}
	checks, targets := describeScenarios(validationScenarios(stage, plan))
	return map[string]any{"scope": "full", "checks": checks, "target_roles": targets, "note": validationPlanNote}
}

// BaseChecks summarises what each base's server-owned gameplay checks drive and
// measure. Lifecycle checks (timer, late events, end, assets, restart) apply to
// every base and are omitted here.
func BaseChecks(dimension string) map[string]map[string]string {
	out := map[string]map[string]string{}
	for _, base := range templateNames() {
		if base == "voxel" || is3DTemplate(base) != (dimension == "3d") {
			continue
		}
		if base == "three" {
			out[base] = map[string]string{"startup": "visible canvas without runtime errors; gameplay checks need scene data or declared scenarios"}
			continue
		}
		entry := map[string]string{}
		for _, scenario := range requiredScenarios(base) {
			name := strings.TrimPrefix(scenario.ID, "required_")
			if !slices.Contains([]string{"input", "primary", "rules", "aim", "reload"}, name) {
				continue
			}
			steps := make([]string, 0, len(scenario.Steps))
			for _, step := range scenario.Steps {
				switch step.Action {
				case "target":
					steps = append(steps, step.Mode+" "+step.Target)
				case "key":
					steps = append(steps, "key "+step.Key)
				default:
					steps = append(steps, step.Action)
				}
			}
			entry[name] = fmt.Sprintf("%s => %s %s", strings.Join(steps, ", "), scenario.Metric, scenario.Compare)
		}
		out[base] = entry
	}
	return out
}

// repairableChecks reports whether every open check is one the game source can
// satisfy. Missing harness evidence is never repairable by a model call.
func repairableChecks(checks []CheckResult) bool {
	found := false
	for _, check := range checks {
		if check.Status == "passed" {
			continue
		}
		if check.Status != "unavailable" || !check.Repairable {
			return false
		}
		found = true
	}
	return found
}

// ValidationNextAction turns a result into one bounded instruction. It never
// reports success for checks that were not observed.
func ValidationNextAction(result BuildResult) string {
	switch {
	case result.OK:
		return "This scope passed. Finish the remaining accepted features, then end with the player-facing summary; the server validates again before publication."
	case result.Repairable:
		return "The running game offered no target or physical effect for the open checks. Give each listed target role/ID to a real object (2D body(...,role) or object.__gmRole; 3D a config.objects role or scene node) and let the real contact change the measured metric, then validate again. Never change counters or observers to satisfy a check."
	case result.RuntimeStatus == "unavailable" || result.GameplayStatus == "unavailable":
		return "Browser evidence is unavailable, which source edits cannot repair. The Studio preview must stay open; end this turn."
	case result.RuntimeStatus == "failed":
		return "Fix the reported build or runtime error at its source location before any gameplay change, then validate again."
	default:
		return "Fix the first failed check using its expected/observed values and executed steps. Keep every passed check passing, then validate again."
	}
}
