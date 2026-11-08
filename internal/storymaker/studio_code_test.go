package storymaker

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"reflect"
	"strings"
	"testing"
)

func codeCall(t *testing.T, s *Studio, op, id string, data map[string]any, rev string) map[string]any {
	t.Helper()
	return answerOf(t, s.Call(context.Background(), StudioCall{Tool: ToolStory, Operation: op, Args: map[string]any{"scene_id": id, "data": data, "expected_rev": rev}}))
}

func TestStudioCodeWritesKeepSavedSourceOnFailureAndUseRevisions(t *testing.T) {
	ctx := context.Background()
	s, ps, id := newStudioWriteRig(t, nil)
	s.SetPhase(PhaseFill, "s1")
	if got := codeCall(t, s, "put_scene", "s1", map[string]any{"kind": "code"}, ""); got["code"] != CodeCodeError {
		t.Fatalf("activated missing source: %v", got)
	}
	read := codeCall(t, s, "get_code", "s1", nil, "")
	if read["rev"] != MustNotExist {
		t.Fatal(read)
	}
	write := codeCall(t, s, "put_code", "s1", map[string]any{"source": codeReferenceExample}, read["rev"].(string))
	if write["status"] != "ok" || s.PhaseDone() {
		t.Fatalf("source write prematurely completed scene: %v", write)
	}
	saved, _, err := ps.LoadCodeSource(ctx, id, "s1")
	if err != nil {
		t.Fatal(err)
	}
	bundle, err := ps.LoadCodeBundle(ctx, id, "s1")
	if err != nil {
		t.Fatal(err)
	}
	for _, source := range []string{"export default (", "export default function scene(api) { fetch('https://example.test'); }", strings.Repeat("x", StudioCodeBytes+1)} {
		bad := codeCall(t, s, "put_code", "s1", map[string]any{"source": source}, write["rev"].(string))
		if bad["status"] != "refused" {
			t.Fatalf("accepted bad source: %v", bad)
		}
		current, _, _ := ps.LoadCodeSource(ctx, id, "s1")
		currentBundle, _ := ps.LoadCodeBundle(ctx, id, "s1")
		if !bytes.Equal(saved, current) || !bytes.Equal(bundle, currentBundle) {
			t.Fatal("bad source replaced saved files")
		}
	}
	if stale := codeCall(t, s, "put_code", "s1", map[string]any{"source": codeReferenceExample}, MustNotExist); stale["code"] != "rev_mismatch" {
		t.Fatal(stale)
	}
	if activation := codeCall(t, s, "put_scene", "s1", map[string]any{"kind": "code"}, ""); activation["status"] != "ok" || !s.PhaseDone() {
		t.Fatal(activation)
	}
	if !reflect.DeepEqual(s.CodeChangedScenes(), []string{"s1"}) {
		t.Fatal(s.CodeChangedScenes())
	}
	if foreign := codeCall(t, s, "get_code", "s2", nil, ""); foreign["status"] != "refused" {
		t.Fatal("fill read a different scene")
	}
	if escaped := codeCall(t, s, "get_code", "../s1", nil, ""); escaped["status"] != "refused" {
		t.Fatal("read a path")
	}
	s.SetPhase(PhaseOutline, "")
	if unavailable := codeCall(t, s, "get_code", "s1", nil, ""); unavailable["code"] != "operation_not_in_phase" {
		t.Fatal(unavailable)
	}
}

func TestStudioCodeReadOnlyCacheInvalidationAndRevertedChange(t *testing.T) {
	ctx := context.Background()
	s, ps, id := newStudioWriteRig(t, nil)
	original := []byte("export default function scene(api) { api.duration(5); } // </external_data>")
	if _, res, err := ps.SaveCodeScene(ctx, id, "s1", original, MustNotExist); err != nil || len(res.Errors) != 0 {
		t.Fatalf("save: %v %+v", err, res)
	}
	s.SetPhase(PhaseRevise, "")
	read := codeCall(t, s, "get_code", "s1", nil, "")
	raw, _ := json.Marshal(read)
	if bytes.Contains(raw, []byte("</external_data>")) {
		t.Fatal("source escaped its JSON boundary")
	}
	s.deps.ReadOnly = func() bool { return true }
	if got := codeCall(t, s, "put_code", "s1", map[string]any{"source": codeReferenceExample}, read["rev"].(string)); got["code"] != "desktop_readonly" {
		t.Fatal(got)
	}
	s.deps.ReadOnly = nil
	s.checkClean = true
	s.compileETag = "cached-story"
	write := codeCall(t, s, "put_code", "s1", map[string]any{"source": codeReferenceExample}, read["rev"].(string))
	if write["status"] != "ok" {
		t.Fatal(write)
	}
	if s.checkClean || s.compileETag != "" {
		t.Fatal("code write retained a stale compile/check result")
	}
	if !reflect.DeepEqual(s.CodeChangedScenes(), []string{"s1"}) {
		t.Fatal(s.CodeChangedScenes())
	}
	revert := codeCall(t, s, "put_code", "s1", map[string]any{"source": string(original)}, write["rev"].(string))
	if revert["status"] != "ok" || len(s.CodeChangedScenes()) != 0 {
		t.Fatal(revert, s.CodeChangedScenes())
	}
	s.writes["s1"] = StudioSceneWriteLimit
	read = codeCall(t, s, "get_code", "s1", nil, "")
	if got := codeCall(t, s, "put_code", "s1", map[string]any{"source": codeReferenceExample}, read["rev"].(string)); got["code"] != "corrections_exhausted" {
		t.Fatal(got)
	}
	if _, _, err := ps.LoadCodeSource(ctx, id, "s2"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatal("unrelated source changed")
	}
}

func TestStudioReferenceAndMetaAnimationControls(t *testing.T) {
	s, ps, id := newStudioWriteRig(t, nil)
	s.SetPhase(PhaseCast, "")
	for _, topic := range []string{"animation", "code"} {
		r := s.Call(context.Background(), StudioCall{Tool: ToolCatalog, Operation: "reference", Args: map[string]any{"topic": topic}})
		a := answerOf(t, r)
		if r.Status != "ok" || len(a["guide"].(string)) > 20000 {
			t.Fatal(a)
		}
	}
	meta := s.Call(context.Background(), StudioCall{Tool: ToolStory, Operation: "set_meta", Args: map[string]any{"data": map[string]any{"cast": []any{map[string]any{"id": "anna", "name": "Anna", "character": "char.office-woman-1"}}, "style": map[string]any{"rhythm": "calm", "auto_sfx": true}}}})
	if meta.Status != "ok" {
		t.Fatal(string(meta.JSON))
	}
	data, _, _ := ps.LoadStory(context.Background(), id)
	story, _, err := DecodeStory(data, DefaultLimits())
	if err != nil || story.Style.Rhythm != "calm" || !story.Style.AutoSFX {
		t.Fatal(err, string(data))
	}
	if res := BundleCodeScene([]byte(codeReferenceExample), "s1", StudioCodeBytes); len(res.Errors) != 0 {
		t.Fatal(res.Errors)
	}
	start := strings.Index(animationReference, "```json\n") + len("```json\n")
	end := strings.Index(animationReference[start:], "\n```") + start
	var example map[string]any
	if err := json.Unmarshal([]byte(animationReference[start:end]), &example); err != nil {
		t.Fatal(err)
	}
	s.SetPhase(PhaseFill, "s1")
	if got := codeCall(t, s, "put_scene", "s1", example, ""); got["status"] != "ok" {
		t.Fatal("the embedded storyboard example is invalid", got)
	}
}

func TestStudioCodeKeepsLargeUserSourceAndReportsCodeOnlyChanges(t *testing.T) {
	s, ps, id := newStudioWriteRig(t, nil)
	ctx := context.Background()
	source := []byte("export default function scene(api) { api.duration(5); } /*" + strings.Repeat("x", StudioCodeBytes) + "*/")
	if _, result, err := ps.SaveCodeScene(ctx, id, "s1", source, MustNotExist); err != nil || len(result.Errors) != 0 {
		t.Fatal(err, result.Errors)
	}
	s.SetPhase(PhaseRevise, "")
	for _, op := range []string{"get_code", "put_code"} {
		if got := codeCall(t, s, op, "s1", map[string]any{"source": codeReferenceExample}, MustNotExist); got["status"] != "refused" {
			t.Fatal(got)
		}
	}
	stored, _, _ := ps.LoadCodeSource(ctx, id, "s1")
	if !bytes.Equal(stored, source) {
		t.Fatal("large user source was overwritten")
	}
	story, _, _ := s.loadStory(ctx)
	changed, unchanged, removed := ChangedScenes(story, story, "s1")
	if !reflect.DeepEqual(changed, []string{"s1"}) || !reflect.DeepEqual(unchanged, []string{"s2"}) || len(removed) != 0 {
		t.Fatal(changed, unchanged, removed)
	}
}
