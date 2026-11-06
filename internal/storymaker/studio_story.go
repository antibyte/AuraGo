package storymaker

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"unicode"
)

func init() {
	registerStudioOp(ToolStory, "get", studioGet)
	registerStudioOp(ToolStory, "outline", studioOutline)
	registerStudioOp(ToolStory, "set_meta", studioSetMeta)
}

// Word budgets of an outline.
const (
	outlineWordsPerSecond = 2.3
	outlineWordsPerScene  = 19
	outlineMinScenes      = 2
	outlineMaxScenes      = 24
	outlineOverLimit      = 1.15 // share of the target words above which an outline is too long
	outlineRefusalLimit   = 2    // refusals of a long outline; the next attempt is accepted
)

const (
	outlineExample = `{"template":"concept_explainer","target_seconds":60,"scenes":[{"id":"s1","narration":"Ein Passwort allein schützt dein Konto nicht."},{"id":"s2","narration":"Zwei Faktoren machen den Login sicherer."}]}`
	metaExample    = `{"title":"Sicher im Netz","cast":[{"id":"anna","name":"Anna","character":"char.office-woman-1"}]}`
)

var studioTemplates = []string{"product_explainer", "how_to", "concept_explainer", "dialogue_story", "data_story", "onboarding"}

func outlineTargetWords(target int) int {
	return int(math.Round(float64(target) * outlineWordsPerSecond))
}

func outlineSuggestedScenes(target int) int {
	n := int(math.Round(float64(target) * outlineWordsPerSecond / outlineWordsPerScene))
	return max(outlineMinScenes, min(n, outlineMaxScenes))
}

func outlineWordBudget(template string, target, scenes int) int {
	if template == "product_explainer" || template == "concept_explainer" || scenes <= 0 {
		return outlineWordsPerScene
	}
	return int(math.Round(float64(target) * outlineWordsPerSecond / float64(scenes)))
}

// hasLetterOrDigit says whether text has a letter or a digit. Placeholder text such as "..." has neither.
func hasLetterOrDigit(text string) bool {
	return strings.IndexFunc(text, func(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) }) >= 0
}

// studioDataJSON returns the data object of a call as JSON, or the refusal for a missing one.
func studioDataJSON(args map[string]any, example string) ([]byte, *StudioResult) {
	data, isObject := args["data"].(map[string]any)
	if !isObject {
		res := studioRefused("invalid_data", "The data argument must be a JSON object.", "Send data as an object. Example: "+example)
		return nil, &res
	}
	raw, err := json.Marshal(data)
	if err != nil {
		res := studioFailed()
		return nil, &res
	}
	return raw, nil
}

// studioRevMismatch refuses a write whose expected_rev is stale. The answer carries the current
// rev and, when the call names a scene, the current scene.
func studioRevMismatch(args map[string]any, st *Story, etag string) (StudioResult, bool) {
	want := studioArgString(args, "expected_rev")
	rev := studioRev(etag)
	if want == "" || studioRev(want) == rev {
		return StudioResult{}, false
	}
	extra := map[string]any{"rev": rev}
	if sc := studioFindScene(st, studioArgString(args, "scene_id")); sc != nil {
		extra["scene_id"] = sc.ID
		extra["scene"] = sc
	}
	return studioRefusedWith("rev_mismatch", "The story has changed since that rev.", "Use the rev of this answer and repeat the change.", extra), true
}

// studioSaveFailed is the answer to a failed save.
func studioSaveFailed(err error) StudioResult {
	if errors.Is(err, ErrConflict) {
		return studioRefused("rev_mismatch", "The story changed while this call ran.", "Call get, then repeat the change.")
	}
	return studioFailed()
}

// studioIssuesAt keeps the issues whose path starts with one of the prefixes.
func studioIssuesAt(issues []Issue, prefixes ...string) []Issue {
	out := []Issue{}
	for _, is := range issues {
		for _, prefix := range prefixes {
			if strings.HasPrefix(is.Path, prefix) {
				out = append(out, is)
				break
			}
		}
	}
	return out
}

// studioNonErrors keeps warnings and minor notes. The result is never nil, so it encodes as [].
func studioNonErrors(issues []Issue) []Issue {
	out := []Issue{}
	for _, is := range issues {
		if is.Severity != SeverityError {
			out = append(out, is)
		}
	}
	return out
}

func studioSceneIDs(st *Story) []string {
	ids := make([]string, len(st.Scenes))
	for i := range st.Scenes {
		ids[i] = st.Scenes[i].ID
	}
	return ids
}

// studioGet answers with one scene, or with a summary of the story.
func studioGet(ctx context.Context, s *Studio, args map[string]any) StudioResult {
	st, etag, err := s.loadStory(ctx)
	if err != nil {
		return studioFailed()
	}
	byScene := map[string][]Issue{}
	for _, is := range Validate(st, s.deps.Catalog, s.deps.Limits) {
		if is.Severity != SeverityMinor && is.SceneID != "" {
			byScene[is.SceneID] = append(byScene[is.SceneID], is)
		}
	}
	rev := studioRev(etag)
	if id := studioArgString(args, "scene_id"); id != "" {
		sc := studioFindScene(st, id)
		if sc == nil {
			return studioRefused("scene_not_found", fmt.Sprintf("The story has no scene %q.", clip(id, 40)),
				"Use one of these scene ids: "+listOf(studioSceneIDs(st))+".")
		}
		issues := append([]Issue{}, CapIssues(byScene[sc.ID], MaxModelIssues)...)
		res := studioOK(map[string]any{"rev": rev, "scene_id": sc.ID, "scene": sc, "issues": issues})
		res.SceneID = sc.ID
		return res
	}
	rows := make([]map[string]any, 0, len(st.Scenes))
	for i := range st.Scenes {
		sc := &st.Scenes[i]
		codes := []string{}
		if !studioSceneFilled(sc) {
			codes = append(codes, CodeSceneUnfilled)
		}
		for _, is := range byScene[sc.ID] {
			if !containsName(codes, is.Code) {
				codes = append(codes, is.Code)
			}
		}
		rows = append(rows, map[string]any{"id": sc.ID, "environment": sc.Environment, "narration": clip(sc.Narration, 80), "beats": len(sc.Beats), "issues": codes})
	}
	meta := map[string]any{"title": st.Title, "aspect": st.Format.Aspect, "palette": st.Style.Palette, "voice": st.Voice}
	return studioOK(map[string]any{"rev": rev, "meta": meta, "cast": st.Cast, "scenes": rows})
}

// outlineInput ignores unknown fields on purpose: a stray title on an outline row loses nothing, and
// a weak model should not spend a round on it. target_seconds is not read, because the length of
// the job binds and the model cannot stretch the film.
type outlineInput struct {
	Template string         `json:"template"`
	Scenes   []outlineScene `json:"scenes"`
}

type outlineScene struct {
	ID        string `json:"id"`
	Narration string `json:"narration"`
}

type outlineRow struct {
	ID         string `json:"id"`
	Words      int    `json:"words"`
	WordBudget int    `json:"word_budget"`
}

// studioOutline replaces the scenes of a story that has no filled scene yet.
func studioOutline(ctx context.Context, s *Studio, args map[string]any) StudioResult {
	raw, refused := studioDataJSON(args, outlineExample)
	if refused != nil {
		return *refused
	}
	var in outlineInput
	if err := json.Unmarshal(raw, &in); err != nil {
		return studioRefused("invalid_data", "The outline data could not be read: "+clip(err.Error(), 200), "Send data in this form: "+outlineExample)
	}
	template := strings.TrimSpace(in.Template)
	if !containsName(studioTemplates, template) {
		return studioRefused("invalid_data", fmt.Sprintf("The template %q is not known.", clip(template, 40)), "Use one of: "+listOf(studioTemplates)+".")
	}
	if n := len(in.Scenes); n < outlineMinScenes || n > outlineMaxScenes {
		return studioRefused("invalid_data", fmt.Sprintf("The outline has %d scenes; it needs %d to %d.", n, outlineMinScenes, outlineMaxScenes),
			"Send data in this form: "+outlineExample)
	}
	scenes := make([]Scene, 0, len(in.Scenes))
	seen := map[string]bool{}
	for _, row := range in.Scenes {
		id := strings.TrimSpace(row.ID)
		switch {
		case !validSlug(id):
			return studioRefused("invalid_data", fmt.Sprintf("The scene id %q is not a slug.", clip(id, 40)), "Use short ids such as s1 and s2.")
		case seen[id]:
			return studioRefused("invalid_data", fmt.Sprintf("The scene id %q is used twice.", id), "Give each scene its own id.")
		case !hasLetterOrDigit(StripMarkers(row.Narration)):
			return studioRefused("invalid_data", fmt.Sprintf("The scene %q has no narration in letters or digits.", id), "Write the narration of every scene in real words, not dots or placeholders.")
		}
		seen[id] = true
		scenes = append(scenes, Scene{ID: id, Narration: strings.TrimSpace(row.Narration)})
	}

	st, etag, err := s.loadStory(ctx)
	if err != nil {
		return studioFailed()
	}
	if res, mismatch := studioRevMismatch(args, st, etag); mismatch {
		return res
	}
	for i := range st.Scenes {
		if studioSceneFilled(&st.Scenes[i]) {
			return studioRefused("outline_locked", "The story already has filled scenes, so the outline cannot change.",
				"Use put_scene or patch_scene for one scene.")
		}
	}

	target := s.job.TargetSeconds
	budget := outlineWordBudget(template, target, len(scenes))
	rows := make([]outlineRow, len(scenes))
	total := 0
	for i := range scenes {
		words := WordCount(StripMarkers(scenes[i].Narration))
		rows[i] = outlineRow{ID: scenes[i].ID, Words: words, WordBudget: budget}
		total += words
	}
	targetWords := outlineTargetWords(target)
	answer := map[string]any{"scenes": rows, "total_words": total, "target_words": targetWords, "suggested_scenes": outlineSuggestedScenes(target)}
	over := float64(total) > float64(targetWords)*outlineOverLimit
	if over && s.outlineRefusals < outlineRefusalLimit {
		s.outlineRefusals++
		return studioRefusedWith(CodeOverBudget,
			fmt.Sprintf("The narration has %d words; the target is %d. The text is too long.", total, targetWords),
			"Shorten each scene to its word_budget and call outline again.", answer)
	}

	st.Scenes = scenes
	ApplyDefaults(st)
	found := studioIssuesAt(Validate(st, s.deps.Catalog, s.deps.Limits), "/scenes")
	if HasErrors(found) {
		return studioRefusedWith("invalid_data", "The outline has problems.", "Fix the listed issues and call outline again.",
			map[string]any{"issues": CapIssues(found, MaxModelIssues)})
	}
	newTag, err := s.saveStory(ctx, st, etag)
	if err != nil {
		return studioSaveFailed(err)
	}
	s.outlineSaved = true
	s.template = template
	// The story is the checkpoint and this is only a record, so like touch a failure is not reported.
	_ = s.deps.Projects.SetAgentMeta(ctx, s.job.ProjectID, target, s.job.Audience, template)

	warnings := studioNonErrors(found)
	if over {
		warnings = append(warnings, Issue{
			Severity: SeverityWarning, Path: "/scenes", Code: CodeOverTarget,
			Message: fmt.Sprintf("The narration has %d words; the target is %d.", total, targetWords),
			Fix:     "Send shorter narration with put_scene when you fill a scene.",
		})
	}
	if len(warnings) > 0 {
		answer["issues"] = CapIssues(warnings, MaxModelIssues)
	}
	answer["rev"] = studioRev(newTag)
	answer["next_action"] = "The outline is saved. This phase is done: stop."
	return studioOK(answer)
}

// metaInput is the data of set_meta. There is no music field: music stays the user's choice in this
// slice, so the strict decode refuses it and the model can say so.
type metaInput struct {
	Title *string       `json:"title"`
	Style *metaStyle    `json:"style"`
	Voice *Voice        `json:"voice"`
	Cast  *[]CastMember `json:"cast"`
}

type metaStyle struct {
	Palette  *string `json:"palette"`
	Captions *bool   `json:"captions"`
}

// studioSetMeta changes the fields the data names. cast is replaced whole.
func studioSetMeta(ctx context.Context, s *Studio, args map[string]any) StudioResult {
	raw, refused := studioDataJSON(args, metaExample)
	if refused != nil {
		return *refused
	}
	var in metaInput
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&in); err != nil {
		return studioRefused("invalid_data", "set_meta takes only title, style, voice and cast: "+clip(err.Error(), 200),
			"Send only these fields. Music is not available to you. Example: "+metaExample)
	}
	if in.Title == nil && in.Style == nil && in.Voice == nil && in.Cast == nil {
		return studioRefused("invalid_data", "set_meta needs at least one of title, style, voice and cast.", "Example: "+metaExample)
	}
	if in.Title != nil && !hasLetterOrDigit(*in.Title) {
		return studioRefused("invalid_data", "The title has no letters or digits.", "Write a short title in real words, not dots or placeholders.")
	}
	if s.phase == PhaseCast && (in.Cast == nil || len(*in.Cast) == 0) {
		return studioRefused("invalid_data", "This phase needs a cast with at least one member.", "Example: "+metaExample)
	}

	st, etag, err := s.loadStory(ctx)
	if err != nil {
		return studioFailed()
	}
	if res, mismatch := studioRevMismatch(args, st, etag); mismatch {
		return res
	}
	if in.Title != nil {
		st.Title = strings.TrimSpace(*in.Title)
	}
	if in.Style != nil {
		if in.Style.Palette != nil {
			st.Style.Palette = strings.TrimSpace(*in.Style.Palette)
		}
		if in.Style.Captions != nil {
			st.Style.Captions = *in.Style.Captions
		}
	}
	if in.Voice != nil {
		st.Voice = in.Voice
	}
	if in.Cast != nil {
		st.Cast = *in.Cast
	}
	ApplyDefaults(st)
	issues := Validate(st, s.deps.Catalog, s.deps.Limits)
	found := studioIssuesAt(issues, "/title", "/style", "/voice", "/cast")
	blocking := append([]Issue{}, found...)
	for _, is := range issues {
		if is.Code == CodeUnknownCast && is.Severity == SeverityError && strings.HasPrefix(is.Path, "/scenes") {
			blocking = append(blocking, is) // a scene still names a cast member this call removes
		}
	}
	if HasErrors(blocking) {
		return studioRefusedWith("invalid_data", "The meta data has problems.", "Fix the listed issues and call set_meta again.",
			map[string]any{"issues": CapIssues(blocking, MaxModelIssues)})
	}
	newTag, err := s.saveStory(ctx, st, etag)
	if err != nil {
		return studioSaveFailed(err)
	}
	answer := map[string]any{"rev": studioRev(newTag), "issues": CapIssues(studioNonErrors(found), MaxModelIssues)}
	if in.Cast != nil && len(*in.Cast) > 0 {
		s.castSaved = true
		if s.phase == PhaseCast {
			answer["next_action"] = "The cast is saved. This phase is done: stop."
		}
	}
	return studioOK(answer)
}
