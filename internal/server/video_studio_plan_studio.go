package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"sync"

	"aurago/internal/agent"
	"aurago/internal/config"
	"aurago/internal/desktop"
	"aurago/internal/videostudio/storyplan"
)

// Fields a model may put beside the arguments instead of inside them. The studio gathers them when the wrapper
// ("plan" of a write, "scene" of a patch) is missing.
var (
	videoStudioPlanStudioPlanKeys  = []string{"style", "characters", "scenes", "audio"}
	videoStudioPlanStudioSceneKeys = []string{"title", "video_prompt", "negative_prompt", "generate_seconds", "use_seconds", "transition_in", "keyframe", "narration"}
)

// videoStudioPlanStudio handles the tool calls of one planning job. It is bound to one project, one mode and, in
// revise mode, one scene: a tool call never names them. Every write goes through writeVideoStudioPlan under the
// project lock and expects the version this studio last read or wrote, so a change made outside the job is never
// overwritten (plan_conflict).
type videoStudioPlanStudio struct {
	svc       *desktop.Service
	manager   *videoStudioManager
	cfg       *config.Config
	projectID string
	mode      string // agent.VideoStudioPlanModeCreate or agent.VideoStudioPlanModeRevise
	sceneID   string // revise: the scene this job may change

	mu       sync.Mutex
	env      storyplan.Env
	plan     storyplan.Plan
	version  string
	phase    string
	finished bool // vs_plan_finish was accepted in this phase
	conflict bool // a write found a plan.json this studio did not know
	writes   int  // saved writes of this job
	calls    int  // tool calls of this job
}

// newVideoStudioPlanStudio reads the plan and the validator environment of the project. A revise job needs its
// scene in the plan.
func newVideoStudioPlanStudio(ctx context.Context, s *Server, svc *desktop.Service, manager *videoStudioManager, cfg *config.Config, projectID, mode, sceneID string) (*videoStudioPlanStudio, error) {
	plan, _, version, err := readVideoStudioPlan(ctx, svc, projectID)
	if err != nil {
		return nil, err
	}
	if mode == agent.VideoStudioPlanModeRevise && videoStudioPlanStudioSceneIndex(plan, sceneID) < 0 {
		return nil, fmt.Errorf("video studio plan: scene %q is not in the plan", sceneID)
	}
	return &videoStudioPlanStudio{
		svc: svc, manager: manager, cfg: cfg, projectID: projectID, mode: mode, sceneID: strings.TrimSpace(sceneID),
		env: videoStudioPlanEnvFor(ctx, cfg, manager, projectID, plan.Profile.Resolution), plan: plan, version: version,
	}, nil
}

// SetPhase binds the studio to the next phase of the job.
func (st *videoStudioPlanStudio) SetPhase(phase string) {
	st.mu.Lock()
	st.phase, st.finished = phase, false
	st.mu.Unlock()
}

// PhaseDone is RunConfig.RunComplete. Outline and details end when the saved plan has what the phase needs; check
// and revise end when vs_plan_finish was accepted. A conflict ends every phase: no later write can succeed.
func (st *videoStudioPlanStudio) PhaseDone() bool {
	st.mu.Lock()
	defer st.mu.Unlock()
	switch {
	case st.conflict:
		return true
	case st.phase == agent.VideoStudioPlanPhaseOutline || st.phase == agent.VideoStudioPlanPhaseDetails:
		return storyplan.PhaseComplete(st.plan, st.phase, st.env)
	}
	return st.finished
}

// Conflict reports whether a write found a plan.json changed outside this job.
func (st *videoStudioPlanStudio) Conflict() bool {
	st.mu.Lock()
	defer st.mu.Unlock()
	return st.conflict
}

// Writes is the number of saved writes of this job.
func (st *videoStudioPlanStudio) Writes() int {
	st.mu.Lock()
	defer st.mu.Unlock()
	return st.writes
}

// Calls is the number of tool calls of this job.
func (st *videoStudioPlanStudio) Calls() int {
	st.mu.Lock()
	defer st.mu.Unlock()
	return st.calls
}

// Issues validates the plan as the studio last read or wrote it.
func (st *videoStudioPlanStudio) Issues() []storyplan.Issue {
	st.mu.Lock()
	defer st.mu.Unlock()
	return videoStudioPlanStudioIssues(st.plan, st.env)
}

// Snapshot returns the plan, its issues and the validator environment for the data packet of a loop.
func (st *videoStudioPlanStudio) Snapshot() (storyplan.Plan, []storyplan.Issue, storyplan.Env) {
	st.mu.Lock()
	defer st.mu.Unlock()
	return st.plan, videoStudioPlanStudioIssues(st.plan, st.env), st.env
}

// SetStatus writes plan.status from a fresh read under the project lock, so the content stays as saved. It writes
// nothing when the status is already right.
func (st *videoStudioPlanStudio) SetStatus(ctx context.Context, status string) error {
	st.mu.Lock()
	defer st.mu.Unlock()
	lock := st.manager.projectLock(st.projectID)
	lock.Lock()
	defer lock.Unlock()
	plan, _, version, err := readVideoStudioPlan(ctx, st.svc, st.projectID)
	if err != nil {
		return err
	}
	if plan.Status != status {
		plan.Status = status
		if version, err = writeVideoStudioPlan(st.manager.publicationContext(ctx, ""), st.svc, st.manager, st.cfg, st.projectID, plan, version); err != nil {
			return err
		}
	}
	st.plan, st.version = plan, version
	return nil
}

// Call runs one tool call of the job and returns its JSON result. The tool must belong to the current phase.
func (st *videoStudioPlanStudio) Call(ctx context.Context, tc agent.ToolCall) []byte {
	st.mu.Lock()
	defer st.mu.Unlock()
	st.calls++
	name := strings.TrimSpace(tc.Action)
	allowed := agent.VideoStudioPlanPhaseTools(st.phase)
	if !slices.Contains(allowed, name) {
		return videoStudioPlanStudioError("tool_not_in_phase", fmt.Sprintf("%s is not available in this phase. Use one of: %s.", name, strings.Join(allowed, ", ")))
	}
	switch name {
	case agent.VideoStudioPlanToolRead:
		return st.read(ctx)
	case agent.VideoStudioPlanToolWrite:
		return st.write(ctx, tc.Params)
	case agent.VideoStudioPlanToolPatchScene:
		return st.patchScene(ctx, tc.Params)
	case agent.VideoStudioPlanToolCheck:
		issues := videoStudioPlanStudioIssues(st.plan, st.env)
		return videoStudioPlanStudioJSON(map[string]any{"status": "ok", "issues": issues, "blocking": storyplan.HasBlocking(issues)})
	}
	issues := st.finishIssues()
	if storyplan.HasBlocking(issues) {
		return videoStudioPlanStudioJSON(map[string]any{"status": "error", "code": "blocking_issues", "message": "Fix the blocking issues first, then call vs_plan_finish again.", "issues": issues})
	}
	st.finished = true
	return videoStudioPlanStudioJSON(map[string]any{"status": "ok"})
}

// finishIssues are the issues that decide vs_plan_finish. A create job answers for the whole plan. A revise job can
// change only its scene and the transition_in of the next one, so only issues of those two scenes count; the rest of
// the plan (including plan-level issues) is the user's. vs_plan_check still lists everything.
func (st *videoStudioPlanStudio) finishIssues() []storyplan.Issue {
	issues := videoStudioPlanStudioIssues(st.plan, st.env)
	if st.mode != agent.VideoStudioPlanModeRevise {
		return issues
	}
	own := videoStudioPlanStudioSceneIndex(st.plan, st.sceneID)
	scoped := []storyplan.Issue{}
	for _, issue := range issues {
		if issue.SceneID == "" {
			continue
		}
		index := videoStudioPlanStudioSceneIndex(st.plan, issue.SceneID)
		if index >= 0 && own >= 0 && (index == own || index == own+1) {
			scoped = append(scoped, issue)
		}
	}
	return scoped
}

func (st *videoStudioPlanStudio) read(ctx context.Context) []byte {
	plan, _, version, err := readVideoStudioPlan(ctx, st.svc, st.projectID)
	if err != nil {
		return videoStudioPlanStudioError("plan_unreadable", "The plan could not be read. Try once more.")
	}
	st.plan, st.version = plan, version
	return videoStudioPlanStudioJSON(map[string]any{
		"status":       "ok",
		"plan":         plan,
		"issues":       videoStudioPlanStudioIssues(plan, st.env),
		"profile":      st.env.Profile,
		"availability": videoStudioPlanAvailabilityFor(st.cfg, st.env.Profile),
	})
}

// write replaces the parts of the plan the call names: style, characters, scenes, audio. Idea, target, language,
// profile, status, cost and every server-owned scene field stay as stored.
func (st *videoStudioPlanStudio) write(ctx context.Context, params map[string]any) []byte {
	const usage = `vs_plan_write needs "plan": an object with style, characters, scenes or audio.`
	raw, ok := videoStudioPlanStudioObject(params, "plan", videoStudioPlanStudioPlanKeys)
	if !ok {
		return videoStudioPlanStudioError("invalid_arguments", usage)
	}
	var parts struct {
		Style      *string                `json:"style"`
		Characters *[]storyplan.Character `json:"characters"`
		Scenes     *[]storyplan.Scene     `json:"scenes"`
		Audio      json.RawMessage        `json:"audio"`
	}
	if err := json.Unmarshal(raw, &parts); err != nil {
		return videoStudioPlanStudioError("invalid_arguments", "The plan could not be read: "+videoStudioPlanStudioReason(err))
	}
	audio := len(parts.Audio) > 0 && string(parts.Audio) != "null"
	if parts.Style == nil && parts.Characters == nil && parts.Scenes == nil && !audio {
		return videoStudioPlanStudioError("invalid_arguments", usage)
	}
	next := st.plan
	if parts.Style != nil {
		next.Style = *parts.Style
	}
	if parts.Characters != nil {
		next.Characters = videoStudioPlanStudioCharacters(st.plan.Characters, *parts.Characters)
	}
	if parts.Scenes != nil {
		next.Scenes = videoStudioPlanStudioScenes(st.plan.Scenes, *parts.Scenes)
	}
	if audio {
		merged := st.plan.Audio
		if err := json.Unmarshal(parts.Audio, &merged); err != nil {
			return videoStudioPlanStudioError("invalid_arguments", "The audio could not be read: "+videoStudioPlanStudioReason(err))
		}
		merged.Music.AssetID = st.plan.Audio.Music.AssetID
		next.Audio = merged
	}
	return st.save(ctx, next)
}

// patchScene merges the given fields into one scene. Nested objects merge field by field, because the decoder
// fills the stored scene; the id and the server-owned fields are put back afterwards.
func (st *videoStudioPlanStudio) patchScene(ctx context.Context, params map[string]any) []byte {
	sceneID, _ := params["scene_id"].(string)
	sceneID = strings.TrimSpace(sceneID)
	raw, ok := videoStudioPlanStudioObject(params, "scene", videoStudioPlanStudioSceneKeys)
	var fields map[string]json.RawMessage
	if !ok || sceneID == "" || json.Unmarshal(raw, &fields) != nil || len(fields) == 0 {
		return videoStudioPlanStudioError("invalid_arguments", `vs_plan_patch_scene needs "scene_id" and "scene" with the fields to change.`)
	}
	index := videoStudioPlanStudioSceneIndex(st.plan, sceneID)
	if index < 0 {
		return videoStudioPlanStudioError("scene_not_found", fmt.Sprintf("There is no scene %q. The scenes are: %s.", sceneID, videoStudioPlanStudioSceneIDs(st.plan)))
	}
	if refusal := st.reviseRefusal(index, fields); refusal != nil {
		return refusal
	}
	stored := st.plan.Scenes[index]
	scene := stored
	if err := json.Unmarshal(raw, &scene); err != nil {
		return videoStudioPlanStudioError("invalid_arguments", "The scene could not be read: "+videoStudioPlanStudioReason(err))
	}
	scene.ID = stored.ID
	videoStudioPlanStudioAdopt(&scene, &stored)
	next := st.plan
	next.Scenes = slices.Clone(st.plan.Scenes)
	next.Scenes[index] = scene
	return st.save(ctx, next)
}

// reviseRefusal keeps a revise job on its scene: it may change that scene and the transition_in of the next one.
func (st *videoStudioPlanStudio) reviseRefusal(index int, fields map[string]json.RawMessage) []byte {
	if st.mode != agent.VideoStudioPlanModeRevise {
		return nil
	}
	own := videoStudioPlanStudioSceneIndex(st.plan, st.sceneID)
	if index == own {
		return nil
	}
	if _, transition := fields["transition_in"]; index == own+1 && transition && len(fields) == 1 {
		return nil
	}
	next := ""
	if own >= 0 && own+1 < len(st.plan.Scenes) {
		next = " and the transition_in of scene " + st.plan.Scenes[own+1].ID
	}
	return videoStudioPlanStudioError("scene_not_allowed", fmt.Sprintf("This revision may change only scene %s%s.", st.sceneID, next))
}

// save writes next with the version this studio knows and answers with the issues of the saved plan.
func (st *videoStudioPlanStudio) save(ctx context.Context, next storyplan.Plan) []byte {
	lock := st.manager.projectLock(st.projectID)
	lock.Lock()
	defer lock.Unlock()
	version, err := writeVideoStudioPlan(st.manager.publicationContext(ctx, ""), st.svc, st.manager, st.cfg, st.projectID, next, st.version)
	if err != nil {
		return st.saveError(err)
	}
	st.version, st.writes = version, st.writes+1
	if plan, _, readVersion, readErr := readVideoStudioPlan(ctx, st.svc, st.projectID); readErr == nil && readVersion == version {
		st.plan = plan
	} else {
		storyplan.Normalize(&next)
		st.plan = next
	}
	return videoStudioPlanStudioJSON(map[string]any{
		"status":         "ok",
		"issues":         videoStudioPlanStudioIssues(st.plan, st.env),
		"phase_complete": storyplan.PhaseComplete(st.plan, st.completionPhase(), st.env),
	})
}

func (st *videoStudioPlanStudio) saveError(err error) []byte {
	var conflict *desktopFileConflict
	switch {
	case errors.As(err, &conflict) && conflict.Status == http.StatusPreconditionFailed, errors.Is(err, errVideoStudioPlanNotFound):
		st.conflict = true
		return videoStudioPlanStudioError("plan_conflict", "The plan changed outside this job, so the job ends. Do not call more tools.")
	case errors.Is(err, errVideoStudioProjectSizeLimit):
		return videoStudioPlanStudioError("project_size_limit", "The project has no room for this plan. Shorten the texts.")
	}
	return videoStudioPlanStudioError("save_failed", "The plan could not be saved. Shorten long texts and try once more.")
}

// completionPhase is the validator phase a write reports as phase_complete. A revision is complete without
// blocking issues.
func (st *videoStudioPlanStudio) completionPhase() string {
	if st.phase == agent.VideoStudioPlanPhaseRevise {
		return agent.VideoStudioPlanPhaseCheck
	}
	return st.phase
}

func videoStudioPlanStudioIssues(plan storyplan.Plan, env storyplan.Env) []storyplan.Issue {
	if issues := storyplan.Validate(plan, env); issues != nil {
		return issues
	}
	return []storyplan.Issue{}
}

func videoStudioPlanStudioSceneIndex(plan storyplan.Plan, id string) int {
	id = strings.TrimSpace(id)
	return slices.IndexFunc(plan.Scenes, func(scene storyplan.Scene) bool { return scene.ID == id })
}

func videoStudioPlanStudioSceneIDs(plan storyplan.Plan) string {
	ids := make([]string, 0, len(plan.Scenes))
	for _, scene := range plan.Scenes {
		ids = append(ids, scene.ID)
	}
	if len(ids) == 0 {
		return "none"
	}
	return strings.Join(ids, ", ")
}

// videoStudioPlanStudioAdopt copies the server-owned fields of stored into scene: state, asset ids, timeline clip,
// narration measurement, cost and error. A scene without a stored counterpart (nil) gets them empty. The agent never
// sets an asset id.
func videoStudioPlanStudioAdopt(scene *storyplan.Scene, stored *storyplan.Scene) {
	var keep storyplan.Scene
	if stored != nil {
		keep = *stored
	}
	scene.State = keep.State
	scene.ClipAssetID, scene.StillAssetID, scene.TimelineClipID = keep.ClipAssetID, keep.StillAssetID, keep.TimelineClipID
	scene.Cost, scene.Error = keep.Cost, keep.Error
	scene.Keyframe.AssetID = keep.Keyframe.AssetID
	scene.Narration.AssetID, scene.Narration.MeasuredSeconds = keep.Narration.AssetID, keep.Narration.MeasuredSeconds
}

// videoStudioPlanStudioScenes is a written scene list with the server-owned fields of the stored scene of the same
// id.
func videoStudioPlanStudioScenes(stored, next []storyplan.Scene) []storyplan.Scene {
	out := make([]storyplan.Scene, len(next))
	for i, scene := range next {
		var match *storyplan.Scene
		if id := strings.TrimSpace(scene.ID); id != "" {
			for j := range stored {
				if stored[j].ID == id {
					match = &stored[j]
					break
				}
			}
		}
		videoStudioPlanStudioAdopt(&scene, match)
		out[i] = scene
	}
	return out
}

// videoStudioPlanStudioCharacters is a written character list. A reference image is the user's choice: it stays
// with the character of the same name.
func videoStudioPlanStudioCharacters(stored, next []storyplan.Character) []storyplan.Character {
	out := make([]storyplan.Character, len(next))
	for i, character := range next {
		character.ReferenceAssetID = ""
		for _, old := range stored {
			if strings.EqualFold(strings.TrimSpace(old.Name), strings.TrimSpace(character.Name)) {
				character.ReferenceAssetID = old.ReferenceAssetID
				break
			}
		}
		out[i] = character
	}
	return out
}

// videoStudioPlanStudioObject returns the JSON object of an argument: an object as sent, or a JSON object text,
// which some models and the XML fallback parser produce. When the argument is missing, the flat keys the model put
// beside it are gathered instead.
func videoStudioPlanStudioObject(params map[string]any, key string, flat []string) ([]byte, bool) {
	value, ok := params[key]
	if !ok || value == nil {
		gathered := map[string]any{}
		for _, name := range flat {
			if field, found := params[name]; found && field != nil {
				gathered[name] = field
			}
		}
		if len(gathered) == 0 {
			return nil, false
		}
		value = gathered
	}
	switch typed := value.(type) {
	case string:
		text := strings.TrimSpace(typed)
		if !strings.HasPrefix(text, "{") || !json.Valid([]byte(text)) {
			return nil, false
		}
		return []byte(text), true
	case map[string]any:
		data, err := json.Marshal(typed)
		return data, err == nil
	}
	return nil, false
}

func videoStudioPlanStudioJSON(value any) []byte {
	data, err := json.Marshal(value)
	if err != nil {
		return []byte(`{"status":"error","code":"result_unencodable","message":"The result could not be encoded."}`)
	}
	return data
}

func videoStudioPlanStudioError(code, message string) []byte {
	return videoStudioPlanStudioJSON(map[string]any{"status": "error", "code": code, "message": message})
}

// videoStudioPlanStudioReason is the decoder's complaint, bounded: it names the field and the type a model got wrong.
func videoStudioPlanStudioReason(err error) string {
	text := strings.TrimPrefix(err.Error(), "json: ")
	if runes := []rune(text); len(runes) > 200 {
		text = string(runes[:200])
	}
	return text
}
