package storymaker

import (
	"hash/fnv"
	"sort"
	"strings"
	"unicode"
)

// The roles of the arc the outline asks for.
const (
	RoleHook     = "hook"
	RoleProblem  = "problem"
	RoleSolution = "solution"
	RoleHow      = "how"
	RoleBenefit  = "benefit"
	RoleClose    = "close"
)

// directionNote is the sentence that travels with every direction.
const directionNote = "A suggestion for staging this scene. Keep it unless the narration needs something else. Scenes with different roles need different looks."

// directionStyle is what a role asks for: a framing, a pose and a gesture. The values are members of the closed name sets
// (a test checks them). Medium follows the speaker; wide shows the whole stage.
var directionStyles = map[string]struct{ framing, pose, gesture string }{
	RoleHook:     {"medium", "present", "wave"},
	RoleProblem:  {"medium", "think", "shrug"},
	RoleSolution: {"wide", "stand", "present"},
	RoleHow:      {"medium", "present", "point"},
	RoleBenefit:  {"wide", "stand", "thumbs-up"},
	RoleClose:    {"medium", "stand", "celebrate"},
}

// directionEntries are the show entries the validator accepts, rotated by scene.
var directionEntries = []string{"pop", "fade", "slide"}

// craftGestureChoices are the gestures named in a fix text of the craft lint, all members of GestureNames.
var craftGestureChoices = []string{"present", "thumbs-up", "shrug", "wave", "think", "celebrate", "count", "point"}

// SceneDirection is the staging suggestion of one scene. It is computed from the story, never stored in it.
type SceneDirection struct {
	Role        string
	Environment string
	Framing     string
	Pose        string
	Gesture     string
	Entry       string
	Actor       string // cast id; "" without a cast and in a dialogue_story
	ActorAt     string
	HeroProp    string // catalog id of the best prop for the narration; "" when none matches
	PropAt      string
}

// packet is the direction as the fill packet shows it.
func (d SceneDirection) packet() map[string]any {
	out := map[string]any{
		"role": d.Role, "environment": d.Environment, "framing": d.Framing, "pose": d.Pose, "gesture": d.Gesture,
		"entry": d.Entry, "actor_at": d.ActorAt, "prop_at": d.PropAt, "note": directionNote,
	}
	if about := environmentAbout(d.Environment); about != "" {
		out["environment_about"] = about
	}
	if d.Actor != "" {
		out["actor"] = d.Actor
	}
	if d.HeroProp != "" {
		out["hero_prop"] = d.HeroProp
	}
	return out
}

// DirectionPlan computes the direction of every scene of the story. It is a pure function of the title, the narrations, the
// cast, the template and the catalog, so every fill packet of one film sees the same plan. A film of at least 3 environments
// gets three: the best match goes to the middle act (solution and how), the second best to the first act (hook and
// problem), the third to the last act (benefit and close). plain-stage is used only when nothing else exists.
func DirectionPlan(story *Story, template string, cat *Catalog) []SceneDirection {
	if story == nil || len(story.Scenes) == 0 {
		return nil
	}
	n := len(story.Scenes)
	ranked := rankEnvironments(story, cat)
	roles := directionRoles(n)
	plan := make([]SceneDirection, n)
	for i := range plan {
		role := roles[i]
		style := directionStyles[role]
		d := SceneDirection{
			Role: role, Environment: actEnvironment(ranked, roleAct(role)),
			Framing: style.framing, Pose: style.pose, Gesture: style.gesture,
			Entry:   directionEntries[i%len(directionEntries)],
			ActorAt: "left", PropAt: "right",
		}
		if i%2 == 1 {
			d.ActorAt, d.PropAt = "right", "left"
		}
		if style.framing == "medium" {
			d.PropAt = "center" // the camera follows the speaker, so the prop stays within reach
		}
		if template != "dialogue_story" && len(story.Cast) > 0 {
			d.Actor = story.Cast[i%len(story.Cast)].ID
		}
		if props := packetSuggestedProps(cat, story.Scenes[i].Narration); len(props) > 0 {
			d.HeroProp = props[0]
		}
		plan[i] = d
	}
	return plan
}

// directionRoles gives the role of each of n scenes: the first is the hook, the last the close, and the scenes between
// follow t = i/(n-1): problem below 0.25, solution below 0.5, how below 0.75, else benefit.
func directionRoles(n int) []string {
	out := make([]string, n)
	for i := range out {
		switch {
		case i == 0:
			out[i] = RoleHook
		case i == n-1:
			out[i] = RoleClose
		default:
			switch t := float64(i) / float64(n-1); {
			case t < 0.25:
				out[i] = RoleProblem
			case t < 0.5:
				out[i] = RoleSolution
			case t < 0.75:
				out[i] = RoleHow
			default:
				out[i] = RoleBenefit
			}
		}
	}
	return out
}

// roleAct is 0 for hook and problem, 1 for solution and how, 2 for benefit and close.
func roleAct(role string) int {
	switch role {
	case RoleHook, RoleProblem:
		return 0
	case RoleSolution, RoleHow:
		return 1
	}
	return 2
}

// actEnvironment picks the environment of an act from the ranked list: the best match plays the middle act.
func actEnvironment(ranked []string, act int) string {
	switch len(ranked) {
	case 0:
		return ""
	case 1:
		return ranked[0]
	case 2:
		return [3]string{ranked[1], ranked[0], ranked[1]}[act]
	}
	return [3]string{ranked[1], ranked[0], ranked[2]}[act]
}

// rankEnvironments orders the catalog environments for this film: each tag that matches a word of the title or of the
// narrations scores 2, a base environment scores 1 more, plain-stage comes last, and ties rotate by a hash of the title so
// that two films about the same thing but with different titles do not both start on the same environment.
func rankEnvironments(story *Story, cat *Catalog) []string {
	ids := cat.Names("environment")
	if len(ids) == 0 {
		return nil
	}
	guide := EnvironmentGuide()
	words := directionWords(story)
	seed := fnv.New32a()
	_, _ = seed.Write([]byte(story.Title))
	start := int(seed.Sum32() % uint32(len(ids)))
	type row struct {
		id            string
		score, rotate int
	}
	rows := make([]row, 0, len(ids))
	for k, id := range ids {
		g := guide[id]
		score := 0
		for _, tag := range g.Tags {
			if tagMatches(words, tag) {
				score += 2
			}
		}
		if g.Base {
			score++
		}
		rows = append(rows, row{id: id, score: score, rotate: (k - start + len(ids)) % len(ids)})
	}
	sort.SliceStable(rows, func(a, b int) bool {
		plainA, plainB := rows[a].id == plainStageID, rows[b].id == plainStageID
		if plainA != plainB {
			return plainB
		}
		if rows[a].score != rows[b].score {
			return rows[a].score > rows[b].score
		}
		return rows[a].rotate < rows[b].rotate
	})
	out := make([]string, len(rows))
	for i, r := range rows {
		out[i] = r.id
	}
	return out
}

// directionWords are the lower-case words of at least 3 letters of the title and the narrations, markers removed.
func directionWords(story *Story) []string {
	var b strings.Builder
	b.WriteString(story.Title)
	for i := range story.Scenes {
		b.WriteByte(' ')
		b.WriteString(StripMarkers(story.Scenes[i].Narration))
	}
	fields := strings.FieldsFunc(strings.ToLower(b.String()), func(r rune) bool { return !unicode.IsLetter(r) })
	out := fields[:0]
	for _, f := range fields {
		if len([]rune(f)) >= 3 {
			out = append(out, f)
		}
	}
	return out
}

// tagMatches is true when a word equals the tag, or starts with it and the tag has at least 4 letters ("daten" matches
// "datenpakete").
func tagMatches(words []string, tag string) bool {
	tag = strings.ToLower(tag)
	long := len([]rune(tag)) >= 4
	for _, w := range words {
		if w == tag || (long && strings.HasPrefix(w, tag)) {
			return true
		}
	}
	return false
}
