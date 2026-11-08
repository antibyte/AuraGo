package storymaker

import (
	"context"
	"crypto/sha256"
	"errors"
	"io/fs"
	"sort"
	"strings"
)

// StudioCodeBytes keeps source exchanges bounded even for the compact agent profile.
const StudioCodeBytes = 8 << 10

type studioCodeChange struct{ before, after [32]byte }

func init() {
	for _, operation := range []string{"get_code", "put_code"} {
		op := operation
		registerStudioOp(ToolStory, op, func(ctx context.Context, s *Studio, args map[string]any) StudioResult {
			return s.runCode(ctx, op, args)
		})
	}
}

func (s *Studio) runCode(ctx context.Context, op string, args map[string]any) StudioResult {
	id, refused := s.sceneWriteTarget(op, args)
	if refused != nil {
		return *refused
	}
	story, _, err := s.loadStory(ctx)
	if err != nil {
		return studioFailed()
	}
	scene := studioFindScene(story, id)
	if scene == nil {
		return sceneWriteNotFound(op, id, story)
	}
	previous, etag, err := s.deps.Projects.LoadCodeSource(ctx, s.job.ProjectID, id)
	missing := errors.Is(err, fs.ErrNotExist)
	if err != nil && !missing {
		return studioFailed()
	}
	if len(previous) > StudioCodeBytes {
		return sceneWriteRefused(op, id, "invalid_data", "This source exceeds the agent's 8 KiB limit.", "Keep this source and edit it in the app.", nil)
	}
	rev := studioRev(etag)
	if missing {
		rev = MustNotExist
	}
	if op == "get_code" {
		return sceneWriteAnswer(op, "ok", id, map[string]any{"scene_id": id, "kind": scene.Kind, "rev": rev, "source": string(previous), "max_bytes": StudioCodeBytes})
	}
	data, _ := args["data"].(map[string]any)
	source, ok := data["source"].(string)
	if !ok || strings.TrimSpace(source) == "" || len(source) > StudioCodeBytes {
		return sceneWriteRefused(op, id, "invalid_data", "data.source must contain JavaScript within 8 KiB.", "Read the code reference and send data containing source.", nil)
	}
	for key, value := range data {
		if key != "source" && value != nil {
			return sceneWriteRefused(op, id, "invalid_data", "put_code changes source only.", "Write scene fields separately with put_scene or patch_scene.", nil)
		}
	}
	if expected := studioArgString(args, "expected_rev"); expected == "" || studioRev(expected) != studioRev(rev) {
		return sceneWriteRefused(op, id, "rev_mismatch", "The code revision is missing or stale.", "Call get_code and use its rev as expected_rev.", nil)
	}
	if s.writes[id] >= StudioSceneWriteLimit {
		return sceneWriteRefused(op, id, "corrections_exhausted", "This scene used its write budget.", "Stop. Its last saved version stays.", nil)
	}
	result := BundleCodeScene([]byte(source), id, s.deps.Limits.MaxCodeBytes)
	count := len(result.Errors)
	free := s.writes[id] == 0 || count < s.lastIssueCount[id]
	if count > 0 && !free && s.charged[id] >= StudioSceneChargedLimit {
		return sceneWriteRefused(op, id, "corrections_exhausted", "This scene used its correction budget.", "Stop. Its last saved version stays.", nil)
	}
	if count > 0 && !free {
		s.charged[id]++
	}
	s.writes[id]++
	s.lastIssueCount[id] = count
	if count > 0 {
		if count > 4 {
			result.Errors = result.Errors[:4]
		}
		return sceneWriteRefused(op, id, CodeCodeError, "The source did not bundle; the saved source and bundle are unchanged.", "Fix the reported lines and call put_code again with the same rev.", map[string]any{"errors": result.Errors, "corrections_left": StudioSceneChargedLimit - s.charged[id]})
	}
	match := etag
	if missing {
		match = MustNotExist
	}
	newETag, _, err := s.deps.Projects.SaveCodeScene(ctx, s.job.ProjectID, id, []byte(source), match)
	if errors.Is(err, ErrConflict) {
		return sceneWriteRefused(op, id, "rev_mismatch", "The code changed while saving.", "Call get_code before repeating the change.", nil)
	}
	if err != nil {
		return studioFailed()
	}
	if s.codeChanges == nil {
		s.codeChanges = map[string]studioCodeChange{}
	}
	change, exists := s.codeChanges[id]
	if !exists {
		change.before = sha256.Sum256(previous)
	}
	change.after = sha256.Sum256([]byte(source))
	s.codeChanges[id] = change
	s.checkClean, s.compileETag = false, ""
	next := "The source is bundled. Run storymaker_check after editing."
	if scene.Kind != "code" {
		next = "Activate it with put_scene and data.kind code. Keep the narration; omit actors, props and beats."
	}
	return sceneWriteAnswer(op, "ok", id, map[string]any{"scene_id": id, "rev": studioRev(newETag), "next_action": next})
}

// CodeChangedScenes includes code-only revisions without treating a reverted edit as changed.
func (s *Studio) CodeChangedScenes() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	ids := []string{}
	for id, change := range s.codeChanges {
		if change.before != change.after {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	return ids
}
