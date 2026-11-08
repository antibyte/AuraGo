package storymaker

import (
	"context"
	"fmt"
	"math"
	"time"
)

const (
	packetNoVoiceNote = "No voice is ready, so viewers may read instead of listen. Still write the narration, and add one short show beat with text in each scene."

	packetWordsPerSecond = 2.3
	packetWordsPerScene  = 19
	packetMinScenes      = 2
	packetMaxScenes      = 24
	packetAdviceLimit    = 3
	packetNarrationRunes = 120
)

var packetTemplates = []string{"product_explainer", "how_to", "concept_explainer", "dialogue_story", "data_story", "onboarding"}

// packetState reads the session fields of the packet. It takes s.mu and releases it before it returns,
// so Packet can call Check and Progress, which lock.
func (s *Studio) packetState() (phase, sceneID string, job StudioJob, deps StudioDeps) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.phase, s.sceneID, s.job, s.deps
}

// Packet builds the context packet of the current phase from the saved story. The server adds request and instruction,
// because the session does not hold the user's text. Normal fill and check packets add the story summary, which compact
// ones leave out.
func (s *Studio) Packet(ctx context.Context, compact bool) (map[string]any, error) {
	phase, sceneID, job, deps := s.packetState()
	story, err := packetStory(ctx, deps, job.ProjectID)
	if err != nil {
		return nil, err
	}
	packet := map[string]any{
		"phase":           phase,
		"language":        job.Language,
		"aspect":          story.Format.Aspect,
		"voice_available": job.VoiceAvailable,
		"budget":          packetBudget(ctx, phase),
	}
	switch phase {
	case PhaseOutline:
		packet["audience"] = job.Audience
		packet["target_seconds"] = job.TargetSeconds
		packet["target_words"] = int(math.Round(float64(job.TargetSeconds) * packetWordsPerSecond))
		packet["suggested_scenes"] = packetSuggestedScenes(job.TargetSeconds)
		packet["templates"] = packetTemplates
		packet["environments"] = deps.Catalog.Names("environment")
	case PhaseCast:
		packet["story"] = packetSummary(story)
		packet["characters"] = deps.Catalog.Names("character")
		packet["environments"] = deps.Catalog.Names("environment")
		packet["style_options"] = map[string]any{
			"rhythms":  []string{"calm", "standard", "lively"},
			"palettes": deps.Catalog.Names("palette"),
			"auto_sfx": true,
		}
		if !compact {
			packet["character_looks"] = packetCharacterLooks(deps.Catalog)
		}
	case PhaseFill:
		if err := packetFill(ctx, packet, story, sceneID, job, deps); err != nil {
			return nil, err
		}
		if !compact {
			packet["story"] = packetSummary(story)
		}
	case PhaseCheck:
		report, err := s.Check(ctx)
		if err != nil {
			return nil, err
		}
		packet["issues"] = append([]Issue{}, CapIssues(report.Issues, MaxModelIssues)...)
		packet["advice"] = append([]Issue{}, CapIssues(report.Advice, packetAdviceLimit)...)
		packet["cast"] = packetCast(story)
		packet["environments"] = packetEnvironments(deps.Catalog)
		if !compact {
			packet["story"] = packetSummary(story)
		}
	case PhaseRevise:
		progress, err := s.Progress(ctx)
		if err != nil {
			return nil, err
		}
		packet["story"] = packetSummary(story)
		packet["unfilled"] = append([]string{}, progress.Unfilled...)
		packet["issues"] = packetIssues(story, deps, "")
		packet["cast"] = packetCast(story)
		packet["environments"] = packetEnvironments(deps.Catalog)
	default:
		return nil, fmt.Errorf("storymaker: a packet needs a known phase, not %q", phase)
	}
	return packet, nil
}

// packetFillExample is the worked example of a fill scene. It names the first cast member of the story and
// leaves the actor and the gesture out when there is no cast, so it never names a member that does not exist.
func packetFillExample(cast []CastMember) map[string]any {
	example := map[string]any{
		"narration":   "Eine {1}Firewall prüft jedes Paket am Eingang.",
		"environment": "env.office",
		"transition":  "wipe",
	}
	beats := []any{map[string]any{"do": "show", "item": "prop.lock", "at": "right", "entry": "pop", "cue": 1}}
	if len(cast) > 0 {
		who := cast[0].ID
		example["speaker"] = who
		example["actors"] = []any{map[string]any{"cast": who, "at": "left", "pose": "present", "expression": "neutral"}}
		beats = append(beats, map[string]any{"do": "gesture", "who": who, "gesture": "point", "with": "prev"})
	}
	example["beats"] = beats
	return example
}

// packetFill adds the fields of one scene's fill loop.
func packetFill(ctx context.Context, packet map[string]any, story *Story, sceneID string, job StudioJob, deps StudioDeps) error {
	index := -1
	for i := range story.Scenes {
		if story.Scenes[i].ID == sceneID {
			index = i
			break
		}
	}
	if index < 0 {
		return fmt.Errorf("storymaker: the scene %q is not in the story", clip(sceneID, 40))
	}
	project, err := deps.Projects.Get(ctx, job.ProjectID)
	if err != nil {
		return err
	}
	sc := &story.Scenes[index]
	packet["scene"] = map[string]any{
		"id":          sc.ID,
		"index":       index + 1,
		"total":       len(story.Scenes),
		"narration":   sc.Narration,
		"words":       WordCount(StripMarkers(sc.Narration)),
		"word_budget": packetWordBudget(project.Template, job.TargetSeconds, len(story.Scenes)),
	}
	if index > 0 {
		packet["previous_environment"] = story.Scenes[index-1].Environment
		packet["previous_scene"] = packetPreviousScene(&story.Scenes[index-1])
		if entries := packetUsedEntries(story.Scenes[:index]); len(entries) > 0 {
			packet["used_entries"] = entries
		}
	}
	if index+1 < len(story.Scenes) {
		next := &story.Scenes[index+1]
		packet["next_scene"] = map[string]any{
			"id":        next.ID,
			"narration": clip(StripMarkers(next.Narration), packetNarrationRunes),
		}
	}
	if props := packetSuggestedProps(deps.Catalog, sc.Narration); len(props) > 0 {
		packet["suggested_props"] = props
	}
	packet["cast"] = packetCast(story)
	packet["environments"] = packetEnvironments(deps.Catalog)
	packet["open_issues"] = packetIssues(story, deps, sc.ID)
	packet["example"] = packetFillExample(story.Cast)
	if !job.VoiceAvailable {
		packet["note"] = packetNoVoiceNote
	}
	return nil
}

func packetCharacterLooks(cat *Catalog) []map[string]any {
	if cat == nil {
		return nil
	}
	ids := cat.Names("character")
	out := make([]map[string]any, 0, len(ids))
	for _, id := range ids {
		e, ok := cat.Lookup(id)
		if !ok {
			continue
		}
		row := map[string]any{"id": e.ID, "name": e.Name}
		for _, k := range []string{"body", "hair", "outfit", "glasses"} {
			if v := e.Params[k]; v != "" && v != "none" {
				row[k] = v
			}
		}
		out = append(out, row)
	}
	return out
}

func packetPreviousScene(sc *Scene) map[string]any {
	if sc == nil {
		return nil
	}
	actors := make([]map[string]any, 0, len(sc.Actors))
	for _, a := range sc.Actors {
		row := map[string]any{"cast": a.Cast}
		if a.At != nil {
			row["at"] = a.At.String()
		}
		if a.Pose != "" {
			row["pose"] = a.Pose
		}
		if a.Expression != "" {
			row["expression"] = a.Expression
		}
		actors = append(actors, row)
	}
	var shown []string
	for _, p := range sc.Props {
		if p.Item != "" {
			shown = append(shown, p.Item)
		}
	}
	for _, b := range sc.Beats {
		if b.Do == "show" {
			if b.Item != "" {
				shown = append(shown, b.Item)
			} else if b.Text != "" {
				shown = append(shown, "text:"+clip(b.Text, 24))
			}
		}
	}
	out := map[string]any{
		"id":          sc.ID,
		"environment": sc.Environment,
		"narration":   clip(StripMarkers(sc.Narration), packetNarrationRunes),
		"transition":  sc.Transition,
		"actors":      actors,
	}
	if sc.Speaker != "" {
		out["speaker"] = sc.Speaker
	}
	if len(shown) > 0 {
		out["shown_items"] = shown
	}
	return out
}

func packetUsedEntries(scenes []Scene) []string {
	var out []string
	for i := range scenes {
		for _, b := range scenes[i].Beats {
			if b.Do == "show" && b.Entry != "" && b.Entry != "none" {
				out = append(out, b.Entry)
			}
		}
	}
	if len(out) > 6 {
		out = out[len(out)-6:]
	}
	return out
}

func packetSuggestedProps(cat *Catalog, narration string) []string {
	if cat == nil {
		return nil
	}
	hits := cat.Search(StripMarkers(narration), "prop", 6)
	if len(hits) == 0 {
		return nil
	}
	out := make([]string, 0, len(hits))
	for _, h := range hits {
		out = append(out, h.ID)
	}
	return out
}

func packetStory(ctx context.Context, deps StudioDeps, projectID string) (*Story, error) {
	data, _, err := deps.Projects.LoadStory(ctx, projectID)
	if err != nil {
		return nil, err
	}
	story, _, err := DecodeStory(data, deps.Limits)
	if err != nil {
		return nil, err
	}
	ApplyDefaults(&story)
	return &story, nil
}

// packetBudget gives the limits of decision 14. The time left comes from the job's deadline. corrections_left is the
// limit of a fresh scene, and each put_scene and patch_scene answer carries the exact count.
func packetBudget(ctx context.Context, phase string) map[string]any {
	budget := map[string]any{}
	switch phase {
	case PhaseOutline:
		budget["tool_calls"] = 6
	case PhaseCast:
		budget["tool_calls"] = 12
	case PhaseFill:
		budget["tool_calls"] = 10
		budget["corrections_left"] = StudioSceneChargedLimit
	case PhaseCheck:
		budget["tool_calls"] = 20
		budget["corrections_left"] = StudioSceneChargedLimit
	case PhaseRevise:
		budget["tool_calls"] = 30
		budget["corrections_left"] = StudioSceneChargedLimit
	}
	if deadline, ok := ctx.Deadline(); ok {
		budget["seconds_left"] = max(0, int(time.Until(deadline).Seconds()))
	}
	return budget
}

func packetSuggestedScenes(targetSeconds int) int {
	scenes := int(math.Round(float64(targetSeconds) * packetWordsPerSecond / packetWordsPerScene))
	return max(packetMinScenes, min(packetMaxScenes, scenes))
}

// packetWordBudget is 19 for product and concept explainers, else the target words split over the scenes.
func packetWordBudget(template string, targetSeconds, scenes int) int {
	if template == "product_explainer" || template == "concept_explainer" || targetSeconds <= 0 || scenes <= 0 {
		return packetWordsPerScene
	}
	return int(math.Round(float64(targetSeconds) * packetWordsPerSecond / float64(scenes)))
}

func packetCast(story *Story) []map[string]any {
	out := make([]map[string]any, 0, len(story.Cast))
	for _, member := range story.Cast {
		out = append(out, map[string]any{"id": member.ID, "name": member.Name, "character": member.Character})
	}
	return out
}

func packetEnvironments(cat *Catalog) []map[string]any {
	ids := cat.Names("environment")
	out := make([]map[string]any, 0, len(ids))
	for _, id := range ids {
		out = append(out, map[string]any{"id": id, "spots": cat.SpotNames(id)})
	}
	return out
}

func packetSummary(story *Story) map[string]any {
	scenes := make([]map[string]any, 0, len(story.Scenes))
	total := 0
	for i := range story.Scenes {
		sc := &story.Scenes[i]
		plain := StripMarkers(sc.Narration)
		words := WordCount(plain)
		total += words
		scenes = append(scenes, map[string]any{
			"id":          sc.ID,
			"environment": sc.Environment,
			"narration":   clip(plain, packetNarrationRunes),
			"words":       words,
			"filled":      sc.Kind == "code" || len(sc.Actors) > 0 || len(sc.Props) > 0 || len(sc.Beats) > 0,
		})
	}
	return map[string]any{"title": story.Title, "scenes": scenes, "total_words": total}
}

// packetIssues lists the validation errors and the over_budget lint, for one scene or, with sceneID "", for the story.
// It starts no browser. scene_unfilled stays out: a fill packet is that scene's task, and a revise packet has unfilled.
func packetIssues(story *Story, deps StudioDeps, sceneID string) []Issue {
	all := append(Validate(story, deps.Catalog, deps.Limits), LintForJob(story)...)
	out := []Issue{}
	for _, is := range all {
		if is.Code == CodeSceneUnfilled || (is.Severity != SeverityError && is.Code != CodeOverBudget) {
			continue
		}
		if sceneID == "" || is.SceneID == sceneID {
			out = append(out, is)
		}
	}
	return CapIssues(out, MaxModelIssues)
}
