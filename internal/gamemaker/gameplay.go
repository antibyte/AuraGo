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
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

type CheckResult struct {
	ID       string `json:"id"`
	Status   string `json:"status"`
	Expected string `json:"expected"`
	Observed string `json:"observed"`
}
type GameObservation struct {
	ID     string             `json:"id"`
	Before map[string]float64 `json:"before"`
	After  map[string]float64 `json:"after"`
}

func validateGameReport(report PreviewReport) error {
	if len(report.Observations) > 16 || len(report.Images) > 2 {
		return fmt.Errorf("gameplay report exceeds observation/image limit")
	}
	for _, o := range report.Observations {
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

var gameMetrics = []string{"player_x", "player_y", "actions", "score", "hits", "spawns", "turns", "ticks", "ended", "object_count", "timer_count", "listener_count", "invalid_assets", "assets_used", "elapsed_ms"}
var gameKeys = []string{"LEFT", "RIGHT", "UP", "DOWN", "W", "A", "S", "D", "SPACE", "R", "ESC", "ENTER"}

func validateScenario(s GameScenario) error {
	if len(s.ID) < 1 || len(s.ID) > 64 || strings.HasPrefix(s.ID, "required_") {
		return fmt.Errorf("id must be 1–64 characters and cannot start with required_")
	}
	if !slices.Contains(gameMetrics, s.Metric) {
		return fmt.Errorf("metric must be one of %s", strings.Join(gameMetrics, ", "))
	}
	if !slices.Contains([]string{"increased", "decreased", "changed", "equals"}, s.Compare) || !finite(s.Value) {
		return fmt.Errorf("compare must be increased, decreased, changed or equals with a finite value")
	}
	if len(s.Steps) < 1 || len(s.Steps) > 8 {
		return fmt.Errorf("provide 1–8 steps")
	}
	duration := 0
	for _, step := range s.Steps {
		if !slices.Contains([]string{"key", "pointer", "wait", "observe"}, step.Action) {
			return fmt.Errorf("step action must be key, pointer, wait or observe; JavaScript is not accepted")
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
	input := check("input", "player_x", "changed", key("RIGHT", 350))
	primary := check("primary", "actions", "increased", key("SPACE", 300))
	rules := check("rules", "hits", "increased", key("RIGHT", 650))
	if template == "shooter" || template == "blocks" {
		rules.Steps = []GameTestStep{key("SPACE", 2400)}
	}
	if template == "board" {
		rules.Steps = []GameTestStep{key("SPACE", 150), key("RIGHT", 150), key("SPACE", 150)}
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
		check := CheckResult{ID: scenario.ID, Status: "unavailable", Expected: fmt.Sprintf("%s %s %g", scenario.Metric, scenario.Compare, scenario.Value), Observed: "No complete observation"}
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
				}
				check.Status = "failed"
				if passed {
					check.Status = "passed"
				}
				check.Observed = fmt.Sprintf("before=%g, after=%g", before, after)
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
		}
		out = append(out, check)
	}
	return out
}

// StopAfterValidation ends a repair round after its first check. Building may
// continue after core checks until repairs are exhausted or feedback is absent.
// Results from preceding rounds cannot end a new round.
func (s *Service) StopAfterValidation(jobID string, repairRound bool) func() bool {
	s.mu.RLock()
	previous := s.lastValidation[jobID]
	s.mu.RUnlock()
	return func() bool {
		s.mu.RLock()
		defer s.mu.RUnlock()
		current := s.lastValidation[jobID]
		return s.activeJobID == jobID && current != nil && current != previous &&
			(repairRound || s.validationFailures[jobID] >= 4 || current.RuntimeStatus == "unavailable" || current.GameplayStatus == "unavailable")
	}
}

// Omitted scope remains startup-compatible. Publication calls full for 2D.
func (s *Service) ValidateJobScope(ctx context.Context, jobID, scope string) (result BuildResult) {
	if scope == "" {
		scope = "startup"
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
	if project.Dimension == "3d" && scope != "startup" {
		return BuildResult{GameplayStatus: "unavailable", Diagnostics: []Diagnostic{{Level: "error", Message: "3D gameplay tests are not available; use startup scope"}}}
	}
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	result = s.buildJob(ctx, jobID, scope)
	if !result.OK {
		return result
	}
	check := result.check
	result = s.waitForPreview(ctx, check, 12*time.Second)
	result.check = check
	result.GameplayStatus = "unverified"
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
