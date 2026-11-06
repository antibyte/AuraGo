package storymaker

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
)

// Open-check codes that are no issue codes. render.CodeModelError has the same text as a job error code.
const (
	outcomeCheckCompileUnverified    = "compile_unverified"
	outcomeCheckModelError           = "model_error"
	outcomeCheckCorrectionsExhausted = "corrections_exhausted"
)

// silentBackupName is the copy of story.json that make-silent keeps next to it, so the speech can come back.
const silentBackupName = "story.before-silent.json"

// silentWordsPerSecond and silentPadSeconds turn the words of a silent scene into its length. Without an explicit
// length a silent scene falls to its 1 s minimum or its last beat.
const (
	silentWordsPerSecond = 2.3
	silentPadSeconds     = 0.4
)

// AgentCheck is one open item of a finished agent job.
type AgentCheck struct {
	Code    string `json:"code"`
	SceneID string `json:"scene_id,omitempty"`
}

// AgentJobInfo is the agent part of a job record. Kind is create or revise. Phase is outline, cast, fill, check,
// revise or narrating. Request holds the first 200 runes of the request, scrubbed.
type AgentJobInfo struct {
	Kind            string       `json:"kind"`
	Outcome         string       `json:"outcome,omitempty"`
	Phase           string       `json:"phase,omitempty"`
	SceneID         string       `json:"scene_id,omitempty"`
	ScenesDone      int          `json:"scenes_done"`
	ScenesTotal     int          `json:"scenes_total"`
	StoryETag       string       `json:"story_etag,omitempty"`
	OpenChecks      []AgentCheck `json:"open_checks,omitempty"`
	Summary         string       `json:"summary,omitempty"`
	Provider        string       `json:"provider,omitempty"`
	Model           string       `json:"model,omitempty"`
	Compact         bool         `json:"compact,omitempty"`
	TextMode        bool         `json:"text_mode,omitempty"`
	TargetSeconds   int          `json:"target_seconds,omitempty"`
	DurationSeconds float64      `json:"duration_seconds,omitempty"`
	Request         string       `json:"request,omitempty"`
	Tokens          int          `json:"tokens,omitempty"`
	ChangedScenes   []string     `json:"changed_scenes,omitempty"`
	UnchangedScenes []string     `json:"unchanged_scenes,omitempty"`
	RemovedScenes   []string     `json:"removed_scenes,omitempty"`
}

// OutcomeInput is what a finished job knows. Narration is "", narration_unavailable, narration_failed,
// narration_budget or narration_pending.
type OutcomeInput struct {
	Progress   StudioProgress
	Check      CheckReport
	TimedOut   bool
	ModelError bool
	Narration  string
}

// DecideOutcome applies the outcome rule. failed: no outline was saved. incomplete: a timeout, a model error, or an
// unfilled scene, a model-fixable issue, a compile failure or an environment error is left. Otherwise ready_unverified:
// layout is never verified, so this slice never answers ready. Advice is not listed. Environment items are listed.
func DecideOutcome(in OutcomeInput) (outcome string, checks []AgentCheck) {
	seen := map[AgentCheck]bool{}
	add := func(code, sceneID string) {
		check := AgentCheck{Code: code, SceneID: sceneID}
		if !seen[check] {
			seen[check] = true
			checks = append(checks, check)
		}
	}
	add(CodeLayoutUnverified, "")
	if in.Check.Compile == "unavailable" {
		add(outcomeCheckCompileUnverified, "")
	}
	if in.Narration != "" {
		add(in.Narration, "")
	}
	if in.ModelError {
		add(outcomeCheckModelError, "")
	}
	for _, sceneID := range in.Progress.Unfilled {
		add(CodeSceneUnfilled, sceneID)
	}
	// A draft was never saved, so the story keeps an older version of that scene.
	for _, sceneID := range in.Progress.Drafts {
		add(outcomeCheckCorrectionsExhausted, sceneID)
	}
	blocked := in.TimedOut || in.ModelError || len(in.Progress.Unfilled) > 0 || len(in.Progress.Drafts) > 0
	compileFailed := false
	listed := append(append([]Issue(nil), in.Check.Issues...), in.Check.Environment...)
	for _, is := range listed {
		add(is.Code, is.SceneID)
		compileFailed = compileFailed || is.Code == CodeCompileFailed
		// An error among the environment items blocks by its own code. over_target and compile_failed keep their rules.
		blocked = blocked || IssueModelFixable(is) || (is.Severity == SeverityError && is.Code != CodeOverTarget && is.Code != CodeCompileFailed)
	}
	if in.Check.Compile == "failed" && !compileFailed {
		add(CodeCompileFailed, "")
		compileFailed = true
	}
	// A blank story reports zero scenes (Progress), so zero means that no outline is saved.
	noOutline := in.Progress.ScenesTotal == 0
	switch {
	case noOutline:
		return OutcomeFailed, checks
	case blocked || compileFailed:
		return OutcomeIncomplete, checks
	}
	return OutcomeReadyUnverified, checks
}

// StoryIsBlank reports a story an agent may overwrite: at most one scene with nothing in it, no cast and no music.
// The title, the format and the voice do not count, because creating a project sets them.
func StoryIsBlank(st *Story) bool {
	if st == nil || len(st.Cast) > 0 || len(st.Scenes) > 1 {
		return false
	}
	if st.Music != nil && st.Music.Mode != "" && st.Music.Mode != "none" {
		return false
	}
	for i := range st.Scenes {
		sc := &st.Scenes[i]
		if sc.Kind == "code" || StripMarkers(sc.Narration) != "" || len(sc.Actors) > 0 || len(sc.Props) > 0 || len(sc.Beats) > 0 {
			return false
		}
	}
	return true
}

// ChangedScenes compares two versions of a story scene by scene, by id. changed holds the scenes that
// differ or are new, and unchanged the scenes that are equal. A moved scene that is otherwise equal is
// unchanged. removed holds the ids that only before has. changed and unchanged follow after, and removed
// follows before. A nil story counts as having no scenes.
func ChangedScenes(before, after *Story) (changed, unchanged, removed []string) {
	old := map[string]string{}
	if before != nil {
		for i := range before.Scenes {
			old[before.Scenes[i].ID] = sceneFingerprint(&before.Scenes[i])
		}
	}
	seen := map[string]bool{}
	if after != nil {
		for i := range after.Scenes {
			sc := &after.Scenes[i]
			seen[sc.ID] = true
			if prev, had := old[sc.ID]; had && prev == sceneFingerprint(sc) {
				unchanged = append(unchanged, sc.ID)
			} else {
				changed = append(changed, sc.ID)
			}
		}
	}
	if before != nil {
		for i := range before.Scenes {
			if id := before.Scenes[i].ID; !seen[id] {
				removed = append(removed, id)
			}
		}
	}
	return changed, unchanged, removed
}

// sceneFingerprint is the JSON encoding of a scene. The position of the scene in the story is not part of it.
func sceneFingerprint(sc *Scene) string {
	raw, _ := json.Marshal(sc)
	return string(raw)
}

// MakeSilent rewrites a story so that it needs no voice, and returns the number of scenes it changed. In each
// storyboard scene that still speaks, it sets silent and a length from the spoken words, removes the cues and turns
// each say with text into a show beat. Code scenes stay as they are. The caller applies the defaults again, which
// gives the new show beats their ids. A second call changes nothing.
func MakeSilent(st *Story) int {
	if st == nil {
		return 0
	}
	changed := 0
	for i := range st.Scenes {
		sc := &st.Scenes[i]
		if sc.Kind == "code" {
			continue
		}
		words := 0
		narrated := !sc.Silent && StripMarkers(sc.Narration) != ""
		if narrated {
			words = WordCount(StripMarkers(sc.Narration))
		}
		spoken := false
		for j := range sc.Beats {
			if sc.Beats[j].Do == "say" && sayHasText(sc.Beats[j].Text) {
				spoken = true
				words += WordCount(StripMarkers(sc.Beats[j].Text))
			}
		}
		if !narrated && !spoken {
			continue
		}
		for j := range sc.Beats {
			b := &sc.Beats[j]
			if b.Do == "say" && sayHasText(b.Text) {
				*b = Beat{Do: "show", Text: StripMarkers(b.Text), At: SpotAt("center"), Entry: "fade"}
				continue
			}
			b.Cue = 0
		}
		sc.Silent = true
		sc.Duration = SecondsDuration(max(1, float64(words)/silentWordsPerSecond+silentPadSeconds))
		changed++
	}
	return changed
}

// SaveSilentBackup writes the story bytes next to story.json and replaces an older copy.
func (p *ProjectStore) SaveSilentBackup(ctx context.Context, id string, data []byte) error {
	if err := p.checkID(id); err != nil {
		return err
	}
	_, err := p.st.WriteFile(ctx, p.file(id, silentBackupName), data, "")
	return err
}

// LoadSilentBackup returns the kept story bytes. A missing copy wraps fs.ErrNotExist.
func (p *ProjectStore) LoadSilentBackup(ctx context.Context, id string) ([]byte, error) {
	if err := p.checkID(id); err != nil {
		return nil, err
	}
	data, _, err := p.st.ReadFile(ctx, p.file(id, silentBackupName))
	return data, err
}

// HasSilentBackup reports whether a kept copy exists.
func (p *ProjectStore) HasSilentBackup(ctx context.Context, id string) (bool, error) {
	_, err := p.LoadSilentBackup(ctx, id)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	return err == nil, err
}

// RemoveSilentBackup deletes the kept copy. A missing copy is not an error.
func (p *ProjectStore) RemoveSilentBackup(ctx context.Context, id string) error {
	if err := p.checkID(id); err != nil {
		return err
	}
	return p.st.Remove(ctx, p.file(id, silentBackupName))
}
