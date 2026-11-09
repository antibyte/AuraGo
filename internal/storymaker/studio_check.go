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
// scene with nothing on stage, over_budget for narration over the house budget of 19 words
// plus 15 percent, fade_between_worlds for a fade across different environments, entry_monotony
// for three or more consecutive show beats of one scene with the same entry style, and spot_overlap when
// actors or shown items share the same named spot.
func LintForJob(st *Story) []Issue {
	if st == nil {
		return nil
	}
	var out []Issue
	for i := range st.Scenes {
		sc := &st.Scenes[i]
		// The streak of equal entries counts inside one scene: a new scene starts a new reveal sequence.
		lastEntry, entryStreak := "", 0
		if !studioSceneFilled(sc) {
			out = append(out, Issue{
				Severity: SeverityError, SceneID: sc.ID, Path: jp("scenes", i), Code: CodeSceneUnfilled,
				Message: fmt.Sprintf("Scene %s has nothing on stage.", sc.ID),
				Fix:     `Add actors and beats with patch_scene. Example: {"actors":[{"cast":"anna","at":"left","pose":"present"}],"beats":[{"do":"show","text":"Kurz gesagt","at":"right","entry":"pop"}]}`,
			})
		}
		if i > 0 && sc.Transition == "fade" && sc.Environment != "" && st.Scenes[i-1].Environment != "" && sc.Environment != st.Scenes[i-1].Environment {
			out = append(out, Issue{
				Severity: SeverityWarning, SceneID: sc.ID, Path: jp("scenes", i) + "/transition", Code: CodeFadeBetweenWorlds,
				Message: fmt.Sprintf("Scene %s uses a fade transition between %s and %s.", sc.ID, st.Scenes[i-1].Environment, sc.Environment),
				Fix:     `Use cut, wipe or slide-left when changing environments. Example: {"transition":"wipe"}`,
			})
		}
		if sc.Kind == "code" {
			continue
		}
		out = append(out, lintSceneStage(sc, i)...)
		for j := range sc.Beats {
			b := &sc.Beats[j]
			if b.Do != "show" || b.Entry == "" || b.Entry == "none" {
				continue
			}
			if b.Entry == lastEntry {
				entryStreak++
				if entryStreak == 3 {
					out = append(out, Issue{
						Severity: SeverityWarning, SceneID: sc.ID, Path: fmt.Sprintf("%s/beats/%d/entry", jp("scenes", i), j), Code: CodeEntryMonotony,
						Message: fmt.Sprintf("Three show beats in a row use entry %q.", b.Entry),
						Fix:     `Vary the entry style across reveals: pop, fade or slide.`,
					})
				}
			} else {
				lastEntry = b.Entry
				entryStreak = 1
			}
		}
		if sc.Silent {
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

// lintSceneStage checks whether a scene places two actors or an actor and a prop/text reveal on
// the exact same named spot at the same time.
func lintSceneStage(sc *Scene, sceneIndex int) []Issue {
	if sc == nil || sc.Kind == "code" {
		return nil
	}
	var out []Issue
	actorSpot := make(map[string]string, len(sc.Actors))
	spotOwner := make(map[string]string, len(sc.Actors))
	for j, a := range sc.Actors {
		if a.Cast == "" || a.At == nil || a.At.Point || a.At.Spot == "" {
			continue
		}
		if prev, busy := spotOwner[a.At.Spot]; busy && prev != a.Cast {
			out = append(out, Issue{
				Severity: SeverityWarning, SceneID: sc.ID, Path: fmt.Sprintf("%s/actors/%d/at", jp("scenes", sceneIndex), j), Code: CodeSpotOverlap,
				Message: fmt.Sprintf("Actors %s and %s both stand at spot %q in scene %s.", prev, a.Cast, a.At.Spot, sc.ID),
				Fix:     `Place each actor on a separate spot such as "left", "center" or "right".`,
			})
		} else {
			spotOwner[a.At.Spot] = a.Cast
		}
		actorSpot[a.Cast] = a.At.Spot
	}
	for j, p := range sc.Props {
		if p.At == nil || p.At.Point || p.At.Spot == "" {
			continue
		}
		if who, busy := spotOwner[p.At.Spot]; busy {
			out = append(out, Issue{
				Severity: SeverityWarning, SceneID: sc.ID, Path: fmt.Sprintf("%s/props/%d/at", jp("scenes", sceneIndex), j), Code: CodeSpotOverlap,
				Message: fmt.Sprintf("Prop %s is placed on spot %q where actor %s stands in scene %s.", p.Item, p.At.Spot, who, sc.ID),
				Fix:     `Place the prop on a free spot such as "right", "center" or "board" so it does not cover the actor.`,
			})
		}
	}
	for j, b := range sc.Beats {
		switch b.Do {
		case "enter", "walk":
			if b.Who != "" && b.To != nil && !b.To.Point && b.To.Spot != "" {
				if old := actorSpot[b.Who]; old != "" && spotOwner[old] == b.Who {
					delete(spotOwner, old)
				}
				actorSpot[b.Who] = b.To.Spot
				spotOwner[b.To.Spot] = b.Who
			}
		case "move":
			if b.Target != "" && b.To != nil && !b.To.Point && b.To.Spot != "" {
				if old, ok := actorSpot[b.Target]; ok {
					if spotOwner[old] == b.Target {
						delete(spotOwner, old)
					}
					actorSpot[b.Target] = b.To.Spot
					spotOwner[b.To.Spot] = b.Target
				}
			}
		case "exit":
			if old := actorSpot[b.Who]; old != "" && spotOwner[old] == b.Who {
				delete(spotOwner, old)
			}
			delete(actorSpot, b.Who)
		case "show":
			if b.At == nil || b.At.Point || b.At.Spot == "" {
				continue
			}
			if who, busy := spotOwner[b.At.Spot]; busy {
				label := b.Item
				if label == "" {
					label = "text"
				}
				out = append(out, Issue{
					Severity: SeverityWarning, SceneID: sc.ID, Path: fmt.Sprintf("%s/beats/%d/at", jp("scenes", sceneIndex), j), Code: CodeSpotOverlap,
					Message: fmt.Sprintf("Beat %d shows %s at spot %q where actor %s stands in scene %s.", j, label, b.At.Spot, who, sc.ID),
					Fix:     `Place shown props and text on a free spot (for example "right" when the actor is at "left") so they do not overlap.`,
				})
			}
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

// RoundsLeft is the number of check rounds the model has not used yet. Only its own validate calls use a round, so the
// number is current after any of them; the CheckReport of an earlier Check can be stale.
func (s *Studio) RoundsLeft() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return max(0, StudioCheckRounds-s.checkRounds)
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
		if len(report.Advice) > 0 {
			next = "No blocking issues remain. If any advice item can be improved with patch_scene, patch it now and call storymaker_check again; otherwise answer with one short sentence and stop."
		} else {
			next = "Nothing is left to fix. Answer with one short sentence and stop."
		}
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
	summary, state, environment := s.compileForCheck(ctx, etag)
	found = append(found, checkCompileIssues(summary.Issues)...)
	found = append(found, LintForJob(&story)...)
	found = append(found, s.craftIssues(ctx, &story)...)
	found = append(found, LintWords(&story, s.job.Language)...)
	found = dedupeCheckIssues(found)
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
		case IssueAdvisory(is.Code) || is.Severity == SeverityWarning:
			report.Advice = append(report.Advice, is)
		}
	}
	report.RoundsLeft = max(0, StudioCheckRounds-s.checkRounds)
	return report, etag, nil
}

// dedupeCheckIssues removes duplicate findings for the same scene and code (or exact path) when
// both the static job lint and the browser compile report the same rule. Two gesture_monotony findings
// can share the scene and the path (two gestures that both pass the limit), so the message, which names
// the gesture, is part of their key.
func dedupeCheckIssues(list []Issue) []Issue {
	seen := make(map[string]bool, len(list))
	out := make([]Issue, 0, len(list))
	for _, is := range list {
		key := is.SceneID + "|" + is.Code + "|" + is.Path
		switch is.Code {
		case CodeFadeBetweenWorlds, CodeEntryMonotony:
			key = is.SceneID + "|" + is.Code
		case CodeGestureMonotony:
			key += "|" + is.Message
		}
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, is)
	}
	return out
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
				row.CueTimes = append(row.CueTimes, checkRound(float64(c.StartMS+cue.AtMS)/1000))
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
