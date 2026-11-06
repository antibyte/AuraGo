package storymaker

import (
	"context"
	"fmt"
	"math"
	"slices"
)

const (
	checkWordsPerSecond        = 2.3
	checkSceneTailSeconds      = 0.4
	checkMinSceneSeconds       = 1.0
	checkHouseWords            = 19
	checkOverFactor            = 1.15
	checkMaxAdvice             = 3
	checkCodeCompileUnverified = "compile_unverified"
)

// StudioProgress counts the scenes of the saved story. ScenesTotal is 0 for a blank story, which
// means that no outline is saved. Rev is studioRev of ETag. Drafts lists the scenes with a draft.
type StudioProgress struct {
	ScenesTotal, ScenesDone int
	Unfilled, Drafts        []string
	Rev, ETag               string
}

// CheckReport is the result of Studio.Check and of the model's validate. Issues are model-fixable,
// Advice is optional, and Environment is not the model's to fix. Compile is "ok", "unavailable" or "failed".
type CheckReport struct {
	Issues, Advice, Environment []Issue
	Scenes                      []CheckScene
	TotalSeconds                float64
	TargetSeconds               int
	Compile                     string
	RoundsLeft                  int
}

// CheckScene is the timing of one scene in a check report.
type CheckScene struct {
	ID               string
	NarrationSeconds float64
	EndSeconds       float64
	CueTimes         []float64
}

func init() {
	registerStudioOp(ToolCheck, "validate", studioValidateOp)
}

type checkSceneAnswer struct {
	ID               string    `json:"id"`
	NarrationSeconds float64   `json:"narration_seconds"`
	EndSeconds       float64   `json:"end_seconds"`
	CueTimes         []float64 `json:"cue_times,omitempty"`
}

type checkEnvironmentAnswer struct {
	Code         string `json:"code"`
	Message      string `json:"message"`
	ModelFixable bool   `json:"model_fixable"`
}

// LintForJob adds what Validate accepts but a finished film must not have: scene_unfilled for a
// scene with nothing on stage, and over_budget for narration over the house budget of 19 words
// plus 15 percent.
func LintForJob(st *Story) []Issue {
	if st == nil {
		return nil
	}
	var out []Issue
	for i := range st.Scenes {
		sc := &st.Scenes[i]
		if !studioSceneFilled(sc) {
			out = append(out, Issue{
				Severity: SeverityError, SceneID: sc.ID, Path: jp("scenes", i), Code: CodeSceneUnfilled,
				Message: fmt.Sprintf("Scene %s has nothing on stage.", sc.ID),
				Fix:     `Add actors and beats with patch_scene. Example: {"actors":[{"cast":"anna","at":"left","pose":"present"}],"beats":[{"do":"show","text":"Kurz gesagt","at":"right","entry":"pop"}]}`,
			})
		}
		if sc.Kind == "code" || sc.Silent {
			continue
		}
		if words := WordCount(StripMarkers(sc.Narration)); float64(words) > checkHouseWords*checkOverFactor {
			out = append(out, Issue{
				Severity: SeverityWarning, SceneID: sc.ID, Path: jp("scenes", i) + "/narration", Code: CodeOverBudget,
				Message: fmt.Sprintf("The narration of scene %s has %d words; the budget is about %d.", sc.ID, words, checkHouseWords),
				Fix:     "Shorten the narration or split the scene.",
			})
		}
	}
	return out
}

// Check is the server's own check, run between loops and before narration. It clears checkClean, so
// the check phase ends only on the model's own validate. It charges no check round.
func (s *Studio) Check(ctx context.Context) (CheckReport, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.checkClean = false
	report, _, err := s.runCheck(ctx)
	return report, err
}

// Progress counts the saved story. A blank story, as ProjectStore.Create writes it, reports zero
// scenes, so ScenesTotal 0 means that no outline is saved.
func (s *Studio) Progress(ctx context.Context) (StudioProgress, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	raw, etag, err := s.deps.Projects.LoadStory(ctx, s.job.ProjectID)
	if err != nil {
		return StudioProgress{}, fmt.Errorf("storymaker: progress: %w", err)
	}
	story, _, err := DecodeStory(raw, s.deps.Limits)
	if err != nil {
		return StudioProgress{}, fmt.Errorf("storymaker: progress: %w", err)
	}
	out := StudioProgress{Drafts: s.draftIDs(), Rev: studioRev(etag), ETag: etag}
	if len(story.Scenes) == 1 && story.Scenes[0].Narration == "" && !studioSceneFilled(&story.Scenes[0]) {
		return out, nil
	}
	out.ScenesTotal = len(story.Scenes)
	for i := range story.Scenes {
		if !studioSceneFilled(&story.Scenes[i]) {
			out.Unfilled = append(out.Unfilled, story.Scenes[i].ID)
		} else {
			out.ScenesDone++
		}
	}
	return out, nil
}

// studioValidateOp runs storymaker_check. Studio.Call has checked the phase and the read-only gate.
// The handler runs with s.mu held, so it calls runCheck and not Check.
func studioValidateOp(ctx context.Context, s *Studio, _ map[string]any) StudioResult {
	const op = "validate"
	if s.checkRounds >= StudioCheckRounds {
		return sceneWriteRefused(op, "", "check_rounds_exhausted", fmt.Sprintf("All %d check rounds are used.", StudioCheckRounds),
			"Answer with one short sentence and stop.", nil)
	}
	s.checkRounds++

	report, etag, err := s.runCheck(ctx)
	if err != nil {
		return sceneWriteRefused(op, "", "invalid_data", "The story could not be read.", "Stop. The server reports this to the user.", nil)
	}
	clean := len(report.Issues) == 0
	s.checkClean = clean

	environment := make([]checkEnvironmentAnswer, 0, len(report.Environment))
	for _, is := range report.Environment {
		environment = append(environment, checkEnvironmentAnswer{Code: is.Code, Message: is.Message})
	}
	scenes := make([]checkSceneAnswer, 0, len(report.Scenes))
	for _, sc := range report.Scenes {
		scenes = append(scenes, checkSceneAnswer{ID: sc.ID, NarrationSeconds: sc.NarrationSeconds, EndSeconds: sc.EndSeconds, CueTimes: sc.CueTimes})
	}
	next := "Fix each issue with storymaker_story operation patch_scene, then call storymaker_check again."
	if clean {
		next = "Nothing is left to fix. Answer with one short sentence and stop."
	}
	return sceneWriteAnswer(op, "ok", "", map[string]any{
		"rev":            studioRev(etag),
		"clean":          clean,
		"issues":         append([]Issue{}, CapIssues(report.Issues, MaxModelIssues)...),
		"advice":         append([]Issue{}, CapIssues(report.Advice, checkMaxAdvice)...),
		"environment":    environment,
		"scenes":         scenes,
		"total_seconds":  report.TotalSeconds,
		"target_seconds": report.TargetSeconds,
		"compile":        report.Compile,
		"rounds_left":    report.RoundsLeft,
		"next_action":    next,
	})
}

func (s *Studio) runCheck(ctx context.Context) (CheckReport, string, error) {
	raw, etag, err := s.deps.Projects.LoadStory(ctx, s.job.ProjectID)
	if err != nil {
		return CheckReport{}, "", fmt.Errorf("storymaker: check: %w", err)
	}
	story, _, err := DecodeStory(raw, s.deps.Limits)
	if err != nil {
		return CheckReport{}, "", fmt.Errorf("storymaker: check: %w", err)
	}
	ApplyDefaults(&story)
	found := Validate(&story, s.deps.Catalog, s.deps.Limits)
	if status, statusErr := s.deps.Projects.CodeSceneStatus(ctx, s.job.ProjectID); statusErr == nil {
		found = append(found, CodeSceneIssues(&story, status)...)
	}
	found = append(found, LintForJob(&story)...)
	summary, state, environment := s.compileForCheck(ctx, etag)
	found = append(found, checkCompileIssues(summary.Issues)...)
	found = append(found, LintWords(&story, s.job.Language)...)
	scenes, total := checkTiming(&story, summary)
	if over := s.checkOverTarget(total); over != nil && !slices.ContainsFunc(found, func(is Issue) bool { return is.Code == over.Code }) {
		found = append(found, *over)
	}

	report := CheckReport{
		Environment: environment, Scenes: scenes, TotalSeconds: total,
		TargetSeconds: s.job.TargetSeconds, Compile: state,
	}
	for _, is := range found {
		switch {
		case is.Code == CodeOverTarget, is.Code == CodeCompileFailed && is.SceneID == "", is.Severity == SeverityError && !IssueModelFixable(is):
			report.Environment = append(report.Environment, is)
		case IssueModelFixable(is):
			report.Issues = append(report.Issues, is)
		case IssueAdvisory(is.Code):
			report.Advice = append(report.Advice, is)
		}
	}
	report.RoundsLeft = max(0, StudioCheckRounds-s.checkRounds)
	return report, etag, nil
}

// checkCompileIssues keeps the compile findings the model may see. narration_missing is dropped:
// the job narrates after the check and reports missing narration as an open check.
func checkCompileIssues(list []Issue) []Issue {
	out := []Issue{}
	for _, is := range list {
		if is.Code != CodeNarrationMissing {
			out = append(out, is)
		}
	}
	return out
}

// compileForCheck returns the compile result of this story version. A result is kept per etag,
// and a compile error is not kept, so the next round tries again.
func (s *Studio) compileForCheck(ctx context.Context, etag string) (CompileSummary, string, []Issue) {
	summary, hit := s.compileSummary, s.compileETag == etag
	if !hit {
		if s.deps.Compile == nil {
			summary = CompileSummary{Reason: "no compile check is available"}
		} else {
			fresh, err := s.deps.Compile(ctx, s.job.ProjectID)
			if err != nil {
				return CompileSummary{}, "failed", []Issue{{
					Severity: SeverityError, Code: CodeCompileFailed,
					Message: "The film could not be compiled.",
					Fix:     "This is not a scene problem. The server reports it to the user.",
				}}
			}
			summary = fresh
			s.compileSummary, s.compileETag = fresh, etag
		}
	}
	if !summary.Available {
		message := "The film could not be compiled here, so its real length and motion were not checked."
		if summary.Reason != "" {
			message += " Reason: " + clip(summary.Reason, 120) + "."
		}
		return summary, "unavailable", []Issue{{Severity: SeverityWarning, Code: checkCodeCompileUnverified, Message: message}}
	}
	if slices.ContainsFunc(summary.Issues, func(is Issue) bool { return is.Code == CodeCompileFailed }) {
		return summary, "failed", nil
	}
	return summary, "ok", nil
}

// checkOverTarget reports a film over the export limit as a model-fixable error, so the check loop
// keeps the job until it is cut. A film only over its target is a warning, which is advice for the outcome.
func (s *Studio) checkOverTarget(total float64) *Issue {
	if limit := s.deps.MaxDurationSeconds; limit > 0 && total > float64(limit) {
		return &Issue{
			Severity: SeverityError, Path: "/scenes", Code: CodeOverLimit,
			Message: fmt.Sprintf("The film is about %.0f seconds; the export limit is %d seconds.", total, limit),
			Fix:     fmt.Sprintf("Cut about %.0f seconds: shorten narration or remove a scene.", total-float64(limit)),
		}
	}
	if target := float64(s.job.TargetSeconds); target > 0 && total > target*checkOverFactor {
		return &Issue{
			Severity: SeverityWarning, Path: "/scenes", Code: CodeOverTarget,
			Message: fmt.Sprintf("The film is about %.0f seconds; the target is %d seconds.", total, s.job.TargetSeconds),
			Fix:     "Shorten narration or remove a scene.",
		}
	}
	return nil
}

// checkTiming estimates 2.3 words per second and replaces the estimate with compiled times where
// the compile check ran.
func checkTiming(story *Story, summary CompileSummary) ([]CheckScene, float64) {
	compiled := map[string]CompileScene{}
	if summary.Available {
		for _, sc := range summary.Scenes {
			compiled[sc.ID] = sc
		}
	}
	clock := 0.0
	out := make([]CheckScene, 0, len(story.Scenes))
	for i := range story.Scenes {
		sc := &story.Scenes[i]
		speech := 0.0
		if !sc.Silent {
			speech = float64(WordCount(StripMarkers(sc.Narration))) / checkWordsPerSecond
		}
		length := max(checkMinSceneSeconds, speech+checkSceneTailSeconds)
		if d := sc.Duration; d != nil && !d.Auto && len(d.Invalid) == 0 && d.Seconds > 0 {
			length = d.Seconds
		}
		clock += length
		row := CheckScene{ID: sc.ID, NarrationSeconds: checkRound(speech), EndSeconds: checkRound(clock)}
		if c, ok := compiled[sc.ID]; ok {
			row.EndSeconds = checkRound(float64(c.EndMS) / 1000)
			for _, cue := range c.CuesMS {
				row.CueTimes = append(row.CueTimes, checkRound(float64(cue.AtMS)/1000))
			}
		}
		out = append(out, row)
	}
	total := clock
	if summary.Available && summary.DurationMS > 0 {
		total = float64(summary.DurationMS) / 1000
	}
	return out, checkRound(total)
}

func checkRound(seconds float64) float64 { return math.Round(seconds*100) / 100 }
