package gamemaker

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"image/png"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

type CheckResult struct {
	ID       string         `json:"id"`
	Status   string         `json:"status"`
	Expected string         `json:"expected"`
	Observed string         `json:"observed"`
	Steps    []GameTestStep `json:"steps,omitempty"`
}
type GameObservation struct {
	ID             string             `json:"id"`
	Before         map[string]float64 `json:"before"`
	After          map[string]float64 `json:"after"`
	EvidenceBefore *GameplayEvidence  `json:"evidence_before,omitempty"`
	EvidenceAfter  *GameplayEvidence  `json:"evidence_after,omitempty"`
	TargetRuns     []TargetRun        `json:"target_runs,omitempty"`
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
		if len(o.TargetRuns) > 8 {
			return fmt.Errorf("too many targeted observations")
		}
		for _, run := range o.TargetRuns {
			if len(run.Target) == 0 || len(run.Target) > 96 || !slices.Contains(targetModes, run.Mode) ||
				!slices.Contains([]string{"complete", "no_target", "blocked", "timeout", "unsupported", "inactive"}, run.Reason) ||
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
	return err == nil && config.Width > 0 && config.Height > 0 && config.Width <= 1920 && config.Height <= 1080
}

var gameMetrics = []string{"player_x", "player_y", "player_distance", "actions", "score", "hits", "spawns", "turns", "ticks", "ended", "object_count", "timer_count", "listener_count", "invalid_assets", "assets_used", "elapsed_ms", "aim", "ammo", "reloads", "health", "lives", "goal_remaining", "outcome", "hit_events", "pickup_events", "win_events", "lose_events"}
var gameKeys = []string{"LEFT", "RIGHT", "UP", "DOWN", "W", "A", "S", "D", "SPACE", "R", "P", "ESC", "ENTER", "F", "Q", "E"}
var targetModes = []string{"move", "aim", "reach", "interact", "catch", "avoid", "select"}

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
	return nil
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
	out := requiredScenarios(plan.Template)
	// Schema 4 compositions use their declared scenarios for special mechanics.
	// The starter's genre loop must not become a requirement of the new game.
	if plan.SchemaVersion >= 4 && (plan.Scene != nil || plan.Template == "minimal" || plan.Template == "three") {
		out = slices.DeleteFunc(out, func(s GameScenario) bool {
			return s.ID == "required_input" || s.ID == "required_rules" || s.ID == "required_primary" || s.ID == "required_aim" || s.ID == "required_reload" || s.ID == "required_models"
		})
	}
	for i := range out {
		if out[i].ID == "required_end" {
			out[i].Value = 1
		}
	}
	return append(out, plan.Scenarios...)
}

func compareGameObservations(scenarios []GameScenario, observations []GameObservation) []CheckResult {
	out := make([]CheckResult, 0, len(scenarios))
	for _, scenario := range scenarios {
		check := CheckResult{ID: scenario.ID, Status: "unavailable", Expected: fmt.Sprintf("%s %s %g", scenario.Metric, scenario.Compare, scenario.Value), Observed: "No complete observation", Steps: scenario.Steps}
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
			if bok && aok && finite(before) && finite(after) {
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
				}
				check.Status = "failed"
				if passed {
					check.Status = "passed"
				}
				check.Observed = fmt.Sprintf("before=%g, after=%g", before, after)
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
					for i, run := range found.TargetRuns {
						if run.Target != targetSteps[i].Target || run.Mode != targetSteps[i].Mode {
							check.Status = "unavailable"
							check.Observed = "Mismatched target evidence"
							effects = -1
							break
						}
						contacts += run.Contacts
						if run.Reason == "unsupported" || run.Reason == "inactive" || run.Reason == "no_target" || run.Reason == "blocked" {
							check.Status = "unavailable"
						}
						if run.Inputs == 0 || run.Samples < 2 || check.Status == "passed" && run.Effects == 0 {
							check.Status = "unavailable"
						}
						if run.Inputs > 0 && run.Samples >= 2 {
							effects += run.Effects
						}
						check.Observed += fmt.Sprintf("; target=%s %s (inputs=%d, contacts=%d, effects=%d)", run.Target, run.Reason, run.Inputs, run.Contacts, run.Effects)
					}
					if effects >= 0 && (check.Status == "passed" && effects == 0 || check.Status == "failed" && (contacts == 0 || scenario.Metric != "hits" && scenario.Metric != "actions" && scenario.Metric != "player_distance")) {
						check.Status = "unavailable"
					}
				}
			} else if check.Status == "failed" && slices.Contains([]string{"hits", "hit_events", "pickup_events", "health", "lives", "goal_remaining", "outcome", "win_events", "lose_events"}, scenario.Metric) && scenario.ID != "required_end" {
				check.Status = "unavailable"
				check.Observed += "; blind input did not establish the required gameplay opportunity; use an observed target and the metric for the actual action"
			}
		}
		out = append(out, check)
	}
	return out
}

// StopAfterValidation ends a repair round after its first check. Building may
// continue after core checks until repairs are exhausted or feedback is absent.
// Read-only new-game exploration yields to implementation recovery before the
// job deadline. Results from preceding rounds cannot end a new round.
func (s *Service) StopAfterValidation(jobID string, repairRound bool) func() bool {
	// Reserve time for implementation and validation when a new-game agent only
	// explores. This does not reduce the tool budget of productive runs.
	return s.stopAfterValidation(jobID, repairRound, time.Now().Add(min(5*time.Minute, s.opts.JobTimeout/4)))
}

func (s *Service) stopAfterValidation(jobID string, repairRound bool, explorationDeadline time.Time) func() bool {
	s.mu.RLock()
	previous := s.lastValidation[jobID]
	s.mu.RUnlock()
	var lastWrite int64 = -1
	return func() bool {
		s.mu.RLock()
		active := s.activeJobID == jobID
		current := s.lastValidation[jobID]
		validated := current != nil && current != previous &&
			(repairRound || s.validationFailures[jobID] >= 4 || current.RuntimeStatus == "unavailable" || current.GameplayStatus == "unavailable")
		s.mu.RUnlock()
		if !active || validated {
			return active && validated
		}
		if repairRound || explorationDeadline.IsZero() || time.Now().Before(explorationDeadline) {
			return false
		}
		ctx := context.Background()
		job, err := s.GetJob(ctx, jobID)
		if err != nil {
			return false
		}
		if job.BaseRevision != 0 {
			explorationDeadline = time.Time{} // Existing games keep their full editing budget.
			return false
		}
		project, err := s.GetProject(ctx, job.ProjectID)
		if err != nil {
			return false
		}
		unchanged, err := s.unchangedGameStarter(ctx, jobID, project.Dimension)
		if err != nil {
			return false
		}
		if !unchanged {
			// A first edit must not disable the stall guard for the rest of the job.
			// Check only at the existing exploration interval; invalid tool attempts
			// and repeated reads do not postpone validation of the saved implementation.
			var write int64
			if err := s.db.QueryRowContext(ctx, "SELECT COALESCE(MAX(id),0) FROM gm_events WHERE job_id=? AND event_type='file_changed'", jobID).Scan(&write); err != nil {
				return false
			}
			if write == lastWrite {
				return true
			}
			lastWrite = write
			explorationDeadline = time.Now().Add(min(5*time.Minute, s.opts.JobTimeout/4))
		}
		// Normal validation reports the unchanged starter, then the existing
		// bounded implementation repair writes code. No check is bypassed.
		return unchanged
	}
}

// Scene-backed Three games expose the same bounded observation contract.
func sceneBackedGame(plan GamePlan) bool {
	return guided3D(plan.Template) || plan.Template == "three" && plan.SchemaVersion >= 4 && plan.Scene != nil
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

// Omitted scope remains startup-compatible. Publication calls full for 2D.
func (s *Service) ValidateJobScope(ctx context.Context, jobID, scope string, requestedCheckIDs ...string) (result BuildResult) {
	if scope == "" {
		scope = "startup"
	}
	targetedChecks, checkErr := normalizeTargetedGameMakerChecks(requestedCheckIDs)
	if checkErr != nil {
		return previewUnavailable(checkErr.Error())
	}
	if len(targetedChecks) > 0 && scope == "startup" {
		return previewUnavailable("check_ids require gameplay or full scope")
	}
	if !slices.Contains([]string{"startup", "gameplay", "full"}, scope) {
		return previewUnavailable("scope must be startup, gameplay or full")
	}
	if err := s.CheckJobMutation(ctx, jobID); err != nil {
		if errors.Is(err, ErrRepairLimit) {
			s.mu.RLock()
			last := s.lastValidation[jobID]
			s.mu.RUnlock()
			if last != nil && !last.OK {
				return *last
			}
		}
		return previewUnavailable(err.Error())
	}
	defer func(requestCtx context.Context) {
		if !result.OK && result.RuntimeStatus != "unavailable" && result.GameplayStatus != "unavailable" && requestCtx.Err() == nil {
			s.recordValidationFailure(jobID, result)
		}
		if requestCtx.Err() == nil {
			completed := result
			s.mu.Lock()
			s.lastValidation[jobID] = &completed
			s.mu.Unlock()
		}
	}(ctx)
	project, _, err := s.ProjectForJob(ctx, jobID)
	if err != nil {
		return previewUnavailable(err.Error())
	}
	plan, planErr := s.GetPlan(ctx, jobID)
	if planErr != nil {
		return previewUnavailable(planErr.Error())
	}
	sceneBacked := s.sceneBackedCurrent(ctx, jobID, plan)
	if project.Dimension == "3d" && !sceneBacked && scope != "startup" {
		return BuildResult{GameplayStatus: "unavailable", Diagnostics: []Diagnostic{{Level: "error", Message: "3D gameplay tests are not available; use startup scope"}}}
	}
	missing, err := s.unchangedGameStarter(ctx, jobID, project.Dimension)
	if err != nil {
		return BuildResult{RuntimeStatus: "failed", Diagnostics: []Diagnostic{{Level: "error", File: "src/main.ts", Message: err.Error()}}}
	}
	if missing {
		return BuildResult{RuntimeStatus: "failed", Diagnostics: []Diagnostic{{Level: "implementation", File: "src/main.ts", Message: "The unchanged starter template is not the requested game. No implementation was written. Implement the accepted plan in the project source and use the supplied asset examples, then validate. Imported assets and passing starter gameplay checks do not implement the game."}}}
	}
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	result = s.buildJob(ctx, jobID, scope)
	if !result.OK {
		return result
	}
	check := result.check
	if len(targetedChecks) > 0 {
		selected, selectErr := selectTargetedGameMakerScenarios(check.Scenarios, targetedChecks)
		if selectErr != nil {
			return previewUnavailable(selectErr.Error())
		}
		s.mu.Lock()
		if s.previewCheck == check {
			check.Scenarios = selected
		}
		s.mu.Unlock()
	}
	result = s.waitForPreview(ctx, check, 12*time.Second)
	result.check = check
	result.TargetedChecks = len(targetedChecks) > 0
	result.GameplayStatus = "unverified"
	result.RulesStatus = "unverified"
	result.VisualStatus = "skipped"
	if !result.OK || scope == "startup" {
		return result
	}
	tick := time.NewTicker(25 * time.Millisecond)
	defer tick.Stop()
	for {
		s.mu.RLock()
		current := s.previewCheck == check
		done := check.GameplayReceived
		observations := append([]GameObservation(nil), check.Observations...)
		diagnostics := append([]Diagnostic(nil), check.Diagnostics...)
		images := append([]string(nil), check.Images...)
		s.mu.RUnlock()
		if !current {
			return previewUnavailable("Preview build changed during gameplay validation")
		}
		if len(diagnostics) > 0 {
			return BuildResult{RuntimeStatus: "failed", GameplayStatus: "failed", VisualStatus: "skipped", Diagnostics: diagnostics}
		}
		if done {
			result.Checks = compareGameObservations(check.Scenarios, observations)
			result.GameplayStatus = "passed"
			for _, c := range result.Checks {
				if c.Status != "passed" {
					result.OK = false
					if c.Status == "failed" {
						result.GameplayStatus = "failed"
					} else if result.GameplayStatus != "failed" {
						result.GameplayStatus = "unavailable"
					}
					result.Diagnostics = append(result.Diagnostics, Diagnostic{Level: "gameplay", Message: c.ID + ": expected " + c.Expected + "; observed " + c.Observed})
				}
			}
			ruleStatus, ruleChecks := compareGameplayEvidence(plan, observations)
			result.RulesStatus = ruleStatus
			result.Checks = append(result.Checks, ruleChecks...)
			for _, c := range ruleChecks {
				if c.Status == "failed" {
					result.OK = false
					result.GameplayStatus = "failed"
					result.Diagnostics = append(result.Diagnostics, Diagnostic{Level: "gameplay", Message: c.ID + ": expected " + c.Expected + "; observed " + c.Observed})
				}
			}
			result.Images = images
			data, _ := json.Marshal(result)
			_, _ = s.emit(context.Background(), project.ID, jobID, "validation_result", map[string]any{"result": json.RawMessage(data)})
			return result
		}
		select {
		case <-ctx.Done():
			result.OK = false
			result.GameplayStatus = "unavailable"
			result.Diagnostics = []Diagnostic{{Level: "error", Message: "Gameplay observations were not received within the 60 second full-run limit; keep the Studio preview open"}}
			return result
		case <-tick.C:
		}
	}
}

func (s *Service) recordValidationFailure(jobID string, result BuildResult) {
	stage, err := s.JobDirectory(jobID)
	if err != nil {
		return
	}
	hash := sha256.New()
	err = filepath.WalkDir(stage, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if entry.Name() == "vendor" || entry.Name() == "dist" {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return ErrInvalidPath
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(stage, path)
		fmt.Fprint(hash, rel, "\x00")
		_, _ = hash.Write(data)
		return nil
	})
	if err != nil {
		return
	}
	signature := fmt.Sprintf("%x", hash.Sum(nil))
	s.mu.Lock()
	defer s.mu.Unlock()
	// Revalidating an unchanged failure consumes no additional repair pass.
	if s.lastFailedBuild[jobID] != signature {
		s.validationFailures[jobID]++
		s.lastFailedBuild[jobID] = signature
	}
}

func writeValidationReport(stage string, result BuildResult, maxFiles int, maxFileBytes, maxBytes int64) error {
	bundle, err := os.ReadFile(filepath.Join(stage, "dist", "game.js"))
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(map[string]any{"schema_version": 1, "build_sha256": sha256Bytes(bundle), "result": result}, "", "  ")
	if err != nil {
		return err
	}
	if int64(len(data)) > maxFileBytes {
		return fmt.Errorf("validation report exceeds file size limit")
	}
	path, _, err := secureJoin(stage, ".aurago/validation-report.json", true)
	if err != nil {
		return err
	}
	extra := 1
	var oldBytes int64
	if info, err := os.Stat(path); err == nil {
		extra = 0
		oldBytes = info.Size()
	}
	if err := validateTreeLimits(stage, maxFiles-extra, maxBytes-int64(len(data))+oldBytes); err != nil {
		return fmt.Errorf("validation report exceeds project limits: %w", err)
	}
	return os.WriteFile(path, data, 0o640)
}
