package storymaker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
)

// sceneWriteDryRunID names the scene of a dry run without a scene_id. It is never saved.
const sceneWriteDryRunID = "dry-run"

// Each scene operation is one handler in the routing table of studio.go. Handlers run with s.mu held.
func init() {
	registerStudioOp(ToolStory, "put_scene", studioSceneOp("put_scene"))
	registerStudioOp(ToolStory, "patch_scene", studioSceneOp("patch_scene"))
	registerStudioOp(ToolStory, "dry_run", studioSceneOp("dry_run"))
	registerStudioOp(ToolStory, "remove_scene", studioSceneOp("remove_scene"))
	registerStudioOp(ToolStory, "move_scene", studioSceneOp("move_scene"))
}

// studioSceneOp returns the handler of one scene operation.
func studioSceneOp(op string) studioOp {
	return func(ctx context.Context, s *Studio, args map[string]any) StudioResult {
		return s.runSceneOp(ctx, op, args)
	}
}

// runSceneOp runs put_scene, patch_scene, remove_scene, move_scene or dry_run. Studio.Call has
// checked the phase and the read-only gate, and has repaired the arguments.
func (s *Studio) runSceneOp(ctx context.Context, op string, args map[string]any) StudioResult {
	sceneID, refused := s.sceneWriteTarget(op, args)
	if refused != nil {
		return *refused
	}
	switch op {
	case "put_scene", "patch_scene":
		s.pendingRepairs = nil // repairs left by a refused outline call must not be reported by this write
		res := s.writeScene(ctx, op, sceneID, args)
		fixes := s.takeRepairs()
		if res.Status != "ok" && res.Status != "draft" {
			return res // refused after the repair step: nothing was stored, so nothing is counted or reported
		}
		s.countRepairs(fixes)
		return withRepairs(res, fixes)
	case "dry_run":
		return s.dryRunScene(ctx, sceneID, args)
	case "remove_scene":
		return s.removeScene(ctx, sceneID, args)
	case "move_scene":
		return s.moveScene(ctx, sceneID, args)
	}
	return sceneWriteRefused(op, "", "invalid_data", "This operation does not write a scene.", "Use put_scene or patch_scene.", nil)
}

// DropDrafts discards every draft. Saved scenes stay.
func (s *Studio) DropDrafts() {
	s.mu.Lock()
	defer s.mu.Unlock()
	clear(s.drafts)
}

// draftIDs returns the scene ids with a draft, sorted. The caller holds s.mu.
func (s *Studio) draftIDs() []string {
	return slices.Sorted(maps.Keys(s.drafts))
}

func (s *Studio) sceneWriteTarget(op string, args map[string]any) (string, *StudioResult) {
	id := sceneWriteArg(args, "scene_id")
	if s.phase == PhaseFill {
		if id == "" {
			id = s.sceneID
		}
		if id != s.sceneID {
			r := sceneWriteRefused(op, "", "scene_not_found", fmt.Sprintf("This phase fills only scene %s.", s.sceneID),
				"Use scene_id "+s.sceneID+".", nil)
			return "", &r
		}
	}
	if id == "" {
		if op == "dry_run" {
			return "", nil
		}
		r := sceneWriteRefused(op, "", "invalid_data", "scene_id is required.", "Send the scene_id of the scene.", nil)
		return "", &r
	}
	if !validSlug(id) {
		r := sceneWriteRefused(op, "", "invalid_data", "scene_id is not a valid scene id.", "Use a short id such as s1.", nil)
		return "", &r
	}
	return id, nil
}

// sceneDataExample is the example scene data the refusals show: a free project writes code scenes, the catalog mode an environment.
func (s *Studio) sceneDataExample() string {
	if s.job.Mode == ModeFree {
		return `{"kind":"code"}`
	}
	return `{"environment":"env.office"}`
}

func (s *Studio) writeScene(ctx context.Context, op, sceneID string, args map[string]any) StudioResult {
	data, ok := args["data"].(map[string]any)
	if !ok {
		return sceneWriteRefused(op, sceneID, "invalid_data", "data must be an object with the scene fields.",
			"Send data as an object, for example "+s.sceneDataExample()+".", nil)
	}
	story, etag, refused := s.loadSceneStory(ctx, op, sceneID, args)
	if refused != nil {
		return *refused
	}
	if s.writes[sceneID] >= StudioSceneWriteLimit {
		return sceneWriteRefused(op, sceneID, "corrections_exhausted", fmt.Sprintf("Scene %s used all %d writes.", sceneID, StudioSceneWriteLimit),
			"Stop working on this scene. Its last saved version stays.", nil)
	}
	fields, refused := s.sceneWriteFields(op, op, sceneID, data, &story)
	if refused != nil {
		return *refused
	}
	s.repairSceneFields(fields)
	cueNotes, cueRefusal := resolveCuePhrases(fields)
	if cueRefusal != nil {
		s.writes[sceneID]++
		return sceneWriteRefused(op, sceneID, cueRefusal.Code, cueRefusal.Message, cueRefusal.Next, nil)
	}
	ev := s.evaluateSceneWrite(story, sceneID, fields)
	if ev.errors == 0 {
		if ev.scene.Kind == "code" {
			if _, err := s.deps.Projects.LoadCodeBundle(ctx, s.job.ProjectID, sceneID); err != nil {
				return sceneWriteRefused(op, sceneID, CodeCodeError, "A code scene needs a successfully bundled source first.", "Call get_code, then put_code, then repeat this scene write.", nil)
			}
		}
		s.writes[sceneID]++
		newETag, err := s.saveSceneStory(ctx, &ev.story, etag)
		if err != nil {
			return sceneWriteSaveRefused(op, sceneID, err)
		}
		s.lastIssueCount[sceneID] = 0
		delete(s.drafts, sceneID)
		empty := !studioSceneFilled(&ev.scene)
		next := ""
		if s.phase == PhaseFill {
			next = "This scene is saved. Stop."
			if empty {
				next = "The scene is saved but nothing is on stage. Add actors and beats with patch_scene."
			}
		}
		return sceneWriteOK(op, sceneID, newETag, ev.issues, withCueNotes(next, cueNotes))
	}
	free := s.writes[sceneID] == 0 || ev.errors < s.lastIssueCount[sceneID]
	if !free && s.charged[sceneID] >= StudioSceneChargedLimit {
		return sceneWriteRefused(op, sceneID, "corrections_exhausted", fmt.Sprintf("Scene %s used all %d corrections.", sceneID, StudioSceneChargedLimit),
			"Send a patch_scene that removes at least one listed issue, or stop. The last saved version stays.", nil)
	}
	if !free {
		s.charged[sceneID]++
	}
	s.writes[sceneID]++
	s.lastIssueCount[sceneID] = ev.errors
	s.drafts[sceneID] = fields
	left := StudioSceneChargedLimit - s.charged[sceneID]
	next := "Fix the listed issues with patch_scene and the same scene_id."
	if left == 0 {
		next = "Fix the listed issues with patch_scene and the same scene_id. Only a patch that removes an issue is still accepted."
	}
	return sceneWriteAnswer(op, "draft", sceneID, map[string]any{
		"draft": true, "scene_id": sceneID, "issues": sceneWriteIssues(ev.issues, false), "corrections_left": left, "next_action": withCueNotes(next, cueNotes),
	})
}

func (s *Studio) dryRunScene(ctx context.Context, sceneID string, args map[string]any) StudioResult {
	const op = "dry_run"
	data, ok := args["data"].(map[string]any)
	if !ok {
		return sceneWriteRefused(op, sceneID, "invalid_data", "data must be an object with the scene fields.",
			"Send data as an object, for example "+s.sceneDataExample()+".", nil)
	}
	mode := sceneWriteArg(args, "op")
	if mode == "" {
		mode = "put_scene"
	}
	if mode != "put_scene" && mode != "patch_scene" {
		return sceneWriteRefused(op, sceneID, "invalid_data", "op must be put_scene or patch_scene.", "Send op put_scene or patch_scene.", nil)
	}
	target := sceneID
	if target == "" {
		target = sceneWriteDryRunID
	}
	if s.dryRunsJob >= StudioDryRunsPerJob || s.dryRunsScene[target] >= StudioDryRunsPerScene {
		return sceneWriteRefused(op, sceneID, "dry_runs_exhausted", "No dry run is left.", "Call put_scene or patch_scene directly.", nil)
	}
	story, _, refused := s.loadSceneStory(ctx, op, sceneID, args)
	if refused != nil {
		return *refused
	}
	fields, refused := s.sceneWriteFields(op, mode, target, data, &story)
	if refused != nil {
		return *refused
	}
	s.dryRunsJob++
	s.dryRunsScene[target]++
	cueNotes, cueRefusal := resolveCuePhrases(fields)
	if cueRefusal != nil {
		return sceneWriteRefused(op, sceneID, cueRefusal.Code, cueRefusal.Message, cueRefusal.Next, nil)
	}
	ev := s.evaluateSceneWrite(story, target, fields)
	body := map[string]any{
		"issues":           sceneWriteIssues(ev.issues, false),
		"defaults_applied": append([]DefaultApplied{}, ev.defaults...),
	}
	if next := withCueNotes("", cueNotes); next != "" {
		body["next_action"] = next
	}
	if sceneID != "" {
		body["scene_id"] = sceneID
	}
	return sceneWriteAnswer(op, "ok", sceneID, body)
}

func (s *Studio) removeScene(ctx context.Context, sceneID string, args map[string]any) StudioResult {
	const op = "remove_scene"
	story, etag, refused := s.loadSceneStory(ctx, op, sceneID, args)
	if refused != nil {
		return *refused
	}
	index := sceneWriteIndex(&story, sceneID)
	_, hadDraft := s.drafts[sceneID]
	if index >= 0 && len(story.Scenes) == 1 {
		return sceneWriteRefused(op, sceneID, "invalid_data", "A story needs at least one scene.", "Change this scene instead of removing it.", nil)
	}
	delete(s.drafts, sceneID)
	delete(s.writes, sceneID)
	delete(s.charged, sceneID)
	delete(s.lastIssueCount, sceneID)
	delete(s.dryRunsScene, sceneID)
	if index < 0 {
		if hadDraft {
			return sceneWriteOK(op, sceneID, etag, nil, "")
		}
		return sceneWriteNotFound(op, sceneID, &story)
	}
	story.Scenes = slices.Delete(story.Scenes, index, index+1)
	newETag, err := s.saveSceneStory(ctx, &story, etag)
	if err != nil {
		return sceneWriteSaveRefused(op, sceneID, err)
	}
	return sceneWriteOK(op, sceneID, newETag, nil, "")
}

func (s *Studio) moveScene(ctx context.Context, sceneID string, args map[string]any) StudioResult {
	const op = "move_scene"
	story, etag, refused := s.loadSceneStory(ctx, op, sceneID, args)
	if refused != nil {
		return *refused
	}
	from := sceneWriteIndex(&story, sceneID)
	if from < 0 {
		return sceneWriteNotFound(op, sceneID, &story)
	}
	after := sceneWriteArg(args, "after")
	if after == sceneID {
		return sceneWriteOK(op, sceneID, etag, nil, "")
	}
	if after != "" && sceneWriteIndex(&story, after) < 0 {
		return sceneWriteNotFound(op, after, &story)
	}
	scene := story.Scenes[from]
	rest := slices.Delete(story.Scenes, from, from+1)
	to := 0
	if after != "" {
		to = slices.IndexFunc(rest, func(sc Scene) bool { return sc.ID == after }) + 1
	}
	story.Scenes = slices.Insert(rest, to, scene)
	newETag, err := s.saveSceneStory(ctx, &story, etag)
	if err != nil {
		return sceneWriteSaveRefused(op, sceneID, err)
	}
	return sceneWriteOK(op, sceneID, newETag, nil, "")
}

// loadSceneStory reads the current story with defaults applied and checks expected_rev.
func (s *Studio) loadSceneStory(ctx context.Context, op, sceneID string, args map[string]any) (Story, string, *StudioResult) {
	unreadable := func() (Story, string, *StudioResult) {
		r := sceneWriteRefused(op, sceneID, "invalid_data", "The story could not be read.", "Stop. The server reports this to the user.", nil)
		return Story{}, "", &r
	}
	raw, etag, err := s.deps.Projects.LoadStory(ctx, s.job.ProjectID)
	if err != nil {
		return unreadable()
	}
	story, _, err := DecodeStory(raw, s.deps.Limits)
	if err != nil {
		return unreadable()
	}
	ApplyDefaults(&story)
	if res, mismatch := studioRevMismatch(args, &story, etag); mismatch {
		res.Operation, res.SceneID = op, sceneID
		return Story{}, "", &res
	}
	return story, etag, nil
}

// sceneWriteFields builds the JSON fields of the candidate scene. put_scene replaces the scene
// but keeps narration, kind and silent when data leaves them out. patch_scene merges into the
// draft, else into the saved scene: top-level fields replace and null removes.
func (s *Studio) sceneWriteFields(answerOp, mode, sceneID string, data map[string]any, story *Story) (map[string]any, *StudioResult) {
	if refused := s.freeSceneRefusal(answerOp, sceneID, data); refused != nil {
		return nil, refused
	}
	index := sceneWriteIndex(story, sceneID)
	var saved map[string]any
	if index >= 0 {
		var err error
		saved, err = sceneWriteFieldsOf(story.Scenes[index])
		if err != nil {
			r := sceneWriteRefused(answerOp, sceneID, "invalid_data", "The saved scene could not be read.", "Send the whole scene with put_scene.", nil)
			return nil, &r
		}
	}
	var fields map[string]any
	if mode == "put_scene" {
		fields = maps.Clone(data)
		for _, key := range []string{"narration", "kind", "silent"} {
			if v, present := fields[key]; present && v != nil {
				continue
			}
			if kept, has := saved[key]; has {
				fields[key] = kept
			} else {
				delete(fields, key)
			}
		}
	} else {
		switch draft, hasDraft := s.drafts[sceneID]; {
		case hasDraft:
			fields = maps.Clone(draft)
		case index >= 0:
			fields = saved
		default:
			r := sceneWriteNotFound(answerOp, sceneID, story)
			return nil, &r
		}
		for key, value := range data {
			if value == nil {
				delete(fields, key)
			} else {
				fields[key] = value
			}
		}
	}
	if s.job.Mode == ModeFree {
		fields["kind"] = "code" // every scene of a free project is a code scene, even when the model leaves kind out
	}
	fields["id"] = sceneID
	return fields, nil
}

// freeSceneFields are the fields the scene of a free project never carries: its code draws everything on screen.
var freeSceneFields = []string{"environment", "variant", "speaker", "actors", "props", "beats"}

// freeSceneRefusal refuses scene data that a free project cannot hold. A null removes a field and is allowed.
func (s *Studio) freeSceneRefusal(op, sceneID string, data map[string]any) *StudioResult {
	if s.job.Mode != ModeFree {
		return nil
	}
	const next = `Write the picture with put_code, then activate the scene with put_scene and data {"kind":"code"}. Send only kind, narration, transition and duration.`
	for _, key := range freeSceneFields {
		if value, present := data[key]; present && value != nil {
			r := sceneWriteRefused(op, sceneID, "free_mode_code_only",
				fmt.Sprintf("This project is drawn freely, so a scene has no %s: its code draws everything.", key), next, nil)
			return &r
		}
	}
	if kind, present := data["kind"]; present && kind != nil && kind != "code" {
		r := sceneWriteRefused(op, sceneID, "free_mode_code_only", "This project is drawn freely, so every scene is a code scene.", next, nil)
		return &r
	}
	return nil
}

// sceneWriteEval is the result of checking a candidate scene inside its story.
type sceneWriteEval struct {
	story    Story
	scene    Scene
	issues   []Issue
	defaults []DefaultApplied
	errors   int
}

// evaluateSceneWrite decodes the fields and validates the whole story around them. Only findings
// of this scene count, so a finding elsewhere never blocks this write. It owns story.Scenes.
func (s *Studio) evaluateSceneWrite(story Story, sceneID string, fields map[string]any) sceneWriteEval {
	var ev sceneWriteEval
	raw, err := json.Marshal(fields)
	if err != nil {
		ev.issues = []Issue{{Severity: SeverityError, SceneID: sceneID, Code: CodeInvalidJSON, Message: "The scene could not be encoded."}}
		ev.errors = 1
		return ev
	}
	scene, decodeIssues, err := DecodeScene(raw)
	if err != nil {
		for _, is := range decodeIssues {
			is.SceneID = sceneID
			ev.issues = append(ev.issues, is)
			if is.Severity == SeverityError {
				ev.errors++
			}
		}
		if ev.errors == 0 {
			ev.issues = append(ev.issues, Issue{Severity: SeverityError, SceneID: sceneID, Code: CodeInvalidJSON, Message: "The scene could not be read.",
				Fix: "Send the scene fields of the tool schema, for example " + s.sceneDataExample() + "."})
			ev.errors = 1
		}
		return ev
	}
	index := sceneWriteIndex(&story, sceneID)
	if index < 0 {
		index = len(story.Scenes)
		story.Scenes = append(story.Scenes, scene)
	} else {
		story.Scenes[index] = scene
	}
	prefix := jp("scenes", index) + "/"
	for _, d := range ApplyDefaults(&story) {
		if strings.HasPrefix(d.Path, prefix) {
			ev.defaults = append(ev.defaults, d)
		}
	}
	for _, is := range Validate(&story, s.deps.Catalog, s.deps.Limits) {
		if is.SceneID != sceneID || is.Severity == SeverityMinor {
			continue
		}
		ev.issues = append(ev.issues, is)
		if is.Severity == SeverityError {
			ev.errors++
		}
	}
	ev.issues = append(ev.issues, lintSceneStage(&story.Scenes[index], index)...)
	ev.story = story
	ev.scene = story.Scenes[index]
	return ev
}

// saveSceneStory saves a changed story. Every saved write clears the check state of the model's
// last validate, so the check phase cannot end on a story it has not seen.
func (s *Studio) saveSceneStory(ctx context.Context, story *Story, etag string) (string, error) {
	s.checkClean = false
	ApplyPhononVoices(story, s.deps.Catalog)
	data, err := CanonicalJSON(*story)
	if err != nil {
		return "", err
	}
	return s.deps.Projects.SaveStory(ctx, s.job.ProjectID, data, etag)
}

// withCueNotes appends the notes of resolveCuePhrases to the next_action text of an answer.
func withCueNotes(next string, notes []string) string {
	return strings.TrimSpace(next + " " + strings.Join(notes, " "))
}

func sceneWriteArg(args map[string]any, key string) string {
	value, _ := args[key].(string)
	return strings.TrimSpace(value)
}

func sceneWriteIndex(story *Story, sceneID string) int {
	return slices.IndexFunc(story.Scenes, func(sc Scene) bool { return sc.ID == sceneID })
}

func sceneWriteFieldsOf(scene Scene) (map[string]any, error) {
	raw, err := json.Marshal(scene)
	if err != nil {
		return nil, err
	}
	var fields map[string]any
	if err := json.Unmarshal(raw, &fields); err != nil {
		return nil, err
	}
	return fields, nil
}

func sceneWriteIssues(issues []Issue, onlyWarnings bool) []Issue {
	out := []Issue{}
	for _, is := range issues {
		if !onlyWarnings || is.Severity == SeverityWarning {
			out = append(out, is)
		}
	}
	return CapIssues(out, MaxModelIssues)
}

func sceneWriteAnswer(op, status, sceneID string, body map[string]any) StudioResult {
	body["status"] = status
	data, err := json.Marshal(body)
	if err != nil {
		status = "refused"
		data = []byte(`{"status":"refused","code":"invalid_data","message":"The answer could not be encoded."}`)
	}
	return StudioResult{JSON: data, Status: status, Operation: op, SceneID: sceneID}
}

func sceneWriteOK(op, sceneID, etag string, issues []Issue, next string) StudioResult {
	body := map[string]any{"rev": studioRev(etag), "issues": sceneWriteIssues(issues, true)}
	if sceneID != "" {
		body["scene_id"] = sceneID
	}
	if next != "" {
		body["next_action"] = next
	}
	return sceneWriteAnswer(op, "ok", sceneID, body)
}

func sceneWriteRefused(op, sceneID, code, message, next string, extra map[string]any) StudioResult {
	body := map[string]any{"code": code, "message": message, "next_action": next}
	maps.Copy(body, extra)
	return sceneWriteAnswer(op, "refused", sceneID, body)
}

func sceneWriteNotFound(op, sceneID string, story *Story) StudioResult {
	ids := make([]string, 0, len(story.Scenes))
	for i := range story.Scenes {
		ids = append(ids, story.Scenes[i].ID)
	}
	return sceneWriteRefused(op, "", "scene_not_found", fmt.Sprintf("The scene %s does not exist.", clip(sceneID, 48)),
		"Use one of: "+strings.Join(ids, ", ")+".", nil)
}

func sceneWriteSaveRefused(op, sceneID string, err error) StudioResult {
	if errors.Is(err, ErrConflict) {
		return sceneWriteRefused(op, sceneID, "rev_mismatch", "The story changed while this call ran.", "Send the change again.", nil)
	}
	return sceneWriteRefused(op, sceneID, "invalid_data", "The story could not be saved.", "Stop. The server reports this to the user.", nil)
}
