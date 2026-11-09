package storymaker

import (
	"context"
	"fmt"
	"strings"
)

const (
	craftMinScenes         = 4 // a film of fewer scenes is exempt from every craft lint
	craftEnvironmentsWants = 3 // the environments a long enough film should use
)

// LintFilmCraft finds what Validate accepts but what makes a film look uniform: too few environments, no chosen framing,
// one gesture in too many scenes, a cast member who never appears. It needs a film of at least 4 storyboard scenes: a
// code scene has no environment, framing or gesture and nobody can be put in it, so it is neither counted nor named in a
// fix. The dialogue_story template is exempt from the environment rule, because its scenes follow the speaker turns. The
// fix texts name concrete scenes and ids from DirectionPlan. Every finding is a warning; see IssueCraft.
func LintFilmCraft(st *Story, template string, cat *Catalog) []Issue {
	if st == nil {
		return nil
	}
	board := storyboardScenes(st)
	if len(board) < craftMinScenes {
		return nil
	}
	plan := DirectionPlan(st, template, cat)
	var out []Issue
	if template != "dialogue_story" {
		out = append(out, lintEnvironmentMonotony(st, plan, board)...)
	}
	out = append(out, lintFramingMonotony(st, plan, board)...)
	out = append(out, lintGestureMonotony(st, board)...)
	out = append(out, lintCastUnused(st, plan, board)...)
	return out
}

// storyboardScenes returns the indexes of the scenes that play on a stage, which is every scene but the code scenes.
func storyboardScenes(st *Story) []int {
	out := make([]int, 0, len(st.Scenes))
	for i := range st.Scenes {
		if st.Scenes[i].Kind != "code" {
			out = append(out, i)
		}
	}
	return out
}

// craftIssues runs LintFilmCraft for the job's story. Only a create job gets it: a revise job makes the smallest change
// the user asked for, and a film-level finding on an old story would send the model off to rewrite scenes nobody named.
func (s *Studio) craftIssues(ctx context.Context, story *Story) []Issue {
	if s.job.Kind != AgentKindCreate {
		return nil
	}
	template := ""
	if project, err := s.deps.Projects.Get(ctx, s.job.ProjectID); err == nil {
		template = project.Template
	}
	return LintFilmCraft(story, template, s.deps.Catalog)
}

func lintEnvironmentMonotony(st *Story, plan []SceneDirection, board []int) []Issue {
	n := len(board)
	want := min(craftEnvironmentsWants, (n+2)/3) // min(3, ceil(n/3))
	have := map[string]bool{}
	for _, i := range board {
		if env := st.Scenes[i].Environment; env != "" {
			have[env] = true
		}
	}
	if len(have) >= want {
		return nil
	}
	var picks []string
	used := map[string]bool{}
	for _, i := range board {
		if len(have)+len(used) >= want {
			break
		}
		env := plan[i].Environment
		if env == "" || have[env] || used[env] {
			continue
		}
		used[env] = true
		picks = append(picks, st.Scenes[i].ID+" -> "+env)
	}
	fix := fmt.Sprintf(`Give some scenes another environment so the film uses at least %d. Send patch_scene with {"environment":"<id>","transition":"wipe"} for those scenes`, want)
	if len(picks) > 0 {
		fix += ", for example " + strings.Join(picks, ", ")
	}
	fix += ". A change of environment never fades: use wipe, slide-left or cut."
	return []Issue{{
		Severity: SeverityWarning, Path: "/scenes", Code: CodeEnvironmentMonotony,
		Message: fmt.Sprintf("All %d scenes play in %d environment(s); the film needs at least %d.", n, len(have), want),
		Fix:     fix,
	}}
}

func lintFramingMonotony(st *Story, plan []SceneDirection, board []int) []Issue {
	for _, i := range board {
		sc := &st.Scenes[i]
		// ApplyDefaults always writes a camera, so only a framing other than wide or a target is a choice.
		if sc.Camera != nil && ((sc.Camera.Framing != "" && sc.Camera.Framing != "wide") || sc.Camera.Target != "") {
			return nil
		}
		for j := range sc.Beats {
			if sc.Beats[j].Do == "camera" {
				return nil
			}
		}
	}
	var picks []string
	for _, i := range board {
		if plan[i].Framing != "wide" && len(picks) < 3 {
			picks = append(picks, st.Scenes[i].ID)
		}
	}
	fix := `Choose a framing for the scenes that carry the point: patch_scene with {"camera":{"framing":"medium"}}. Medium follows the speaker, so keep a shown prop on the spot "center" or next to the speaker`
	if len(picks) > 0 {
		fix += "; suggested scenes: " + strings.Join(picks, ", ")
	}
	return []Issue{{
		Severity: SeverityWarning, Path: "/scenes", Code: CodeFramingMonotony,
		Message: "Every scene uses the default wide framing and no scene moves the camera.",
		Fix:     fix + ".",
	}}
}

func lintGestureMonotony(st *Story, board []int) []Issue {
	n := len(board)
	limit := max(2, (n+2)/3) // max(2, ceil(n/3))
	scenesOf := map[string][]string{}
	var order []string
	for _, i := range board {
		sc := &st.Scenes[i]
		seen := map[string]bool{}
		for j := range sc.Beats {
			b := &sc.Beats[j]
			if b.Do != "gesture" || b.Gesture == "" || seen[b.Gesture] {
				continue
			}
			seen[b.Gesture] = true
			if len(scenesOf[b.Gesture]) == 0 {
				order = append(order, b.Gesture)
			}
			scenesOf[b.Gesture] = append(scenesOf[b.Gesture], sc.ID)
		}
	}
	var out []Issue
	for _, gesture := range order {
		ids := scenesOf[gesture]
		if len(ids) <= limit {
			continue
		}
		var choices []string
		for _, c := range craftGestureChoices {
			if c != gesture && len(choices) < 5 {
				choices = append(choices, c)
			}
		}
		out = append(out, Issue{
			Severity: SeverityWarning, SceneID: ids[limit], Path: "/scenes", Code: CodeGestureMonotony,
			Message: fmt.Sprintf("The gesture %q is used in %d of %d scenes (%s).", gesture, len(ids), n, strings.Join(ids, ", ")),
			Fix: fmt.Sprintf("Use %q in at most %d scenes. In %s send patch_scene with the whole beats list of the scene and another gesture, one of %s.",
				gesture, limit, strings.Join(ids[limit:], ", "), strings.Join(choices, ", ")),
		})
	}
	return out
}

// lintCastUnused names a storyboard scene for each member who is neither a speaker nor on a stage. A code scene is never the
// target of the fix; a speaker of a code scene still counts as used, because that member does speak in the film.
func lintCastUnused(st *Story, plan []SceneDirection, board []int) []Issue {
	used := map[string]bool{}
	for i := range st.Scenes {
		sc := &st.Scenes[i]
		used[sc.Speaker] = true
		for j := range sc.Actors {
			used[sc.Actors[j].Cast] = true
		}
	}
	var out []Issue
	for k, member := range st.Cast {
		if used[member.ID] {
			continue
		}
		index := board[len(board)/2]
		for pos, i := range board {
			if pos > 0 && plan[i].Actor == member.ID {
				index = i
				break
			}
		}
		sc := &st.Scenes[index]
		at := "right"
		for j := range sc.Actors {
			if sc.Actors[j].At != nil && sc.Actors[j].At.Spot == "right" {
				at = "left"
			}
		}
		out = append(out, Issue{
			Severity: SeverityWarning, SceneID: sc.ID, Path: jp("cast", k), Code: CodeCastUnused,
			Message: fmt.Sprintf("%s (%s) is in the cast but never appears.", member.Name, member.ID),
			Fix: fmt.Sprintf(`Give %s a part in scene %s with patch_scene: send the actors list with the actors that are already there plus {"cast":%q,"at":%q,"pose":"stand"}, and set "speaker":%q when %s speaks the narration.`,
				member.Name, sc.ID, member.ID, at, member.ID, member.Name),
		})
	}
	return out
}
