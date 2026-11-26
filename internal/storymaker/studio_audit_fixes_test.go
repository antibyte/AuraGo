package storymaker

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

// remove_scene + put_scene of the same id must not reset the per-scene budget.
func TestStudioRemoveSceneKeepsWriteBudget(t *testing.T) {
	studio, _, _ := newStudioWriteRig(t, nil)
	const bad = `"data":{"environment":"env.office","actors":[{"cast":"ghost"}],"beats":[{"do":"show","item":"prop.nope","at":"right","entry":"pop"}]}`
	studio.SetPhase(PhaseRevise, "")
	put := func() map[string]any {
		t.Helper()
		return studioWriteCall(t, studio, ToolStory, "put_scene", `{"scene_id":"s3",`+bad+`}`)
	}
	limit := StudioSceneChargedLimit
	if a := put(); a["corrections_left"] != float64(limit) {
		t.Fatalf("first write = %v", a)
	}
	studio.charged["s3"] = limit
	if a := studioWriteCall(t, studio, ToolStory, "remove_scene", `{"scene_id":"s3"}`); a["status"] != "ok" {
		t.Fatalf("remove = %v", a)
	}
	if studio.writes["s3"] == 0 || studio.charged["s3"] != limit {
		t.Fatalf("remove reset the budget: writes=%d charged=%d", studio.writes["s3"], studio.charged["s3"])
	}
	if a := put(); a["code"] != "corrections_exhausted" {
		t.Fatalf("put after remove = %v, want corrections_exhausted", a)
	}
}

// remove_scene points to put_scene, and a failing put of a removed id warns that the scene is not saved.
func TestStudioRemoveSceneWarnsAboutRewrite(t *testing.T) {
	studio, _, _ := newStudioWriteRig(t, nil)
	studio.SetPhase(PhaseRevise, "")
	good := studioWriteCall(t, studio, ToolStory, "put_scene", `{"scene_id":"s3","data":{"environment":"env.office"}}`)
	if good["status"] != "ok" {
		t.Fatalf("put = %v", good)
	}
	a := studioWriteCall(t, studio, ToolStory, "remove_scene", `{"scene_id":"s3"}`)
	if a["status"] != "ok" || !strings.Contains(fmt.Sprint(a["next_action"]), "put_scene") {
		t.Fatalf("remove = %v", a)
	}
	const bad = `"data":{"environment":"env.office","actors":[{"cast":"ghost"}],"beats":[{"do":"show","item":"prop.nope","at":"right","entry":"pop"}]}`
	d := studioWriteCall(t, studio, ToolStory, "put_scene", `{"scene_id":"s3",`+bad+`}`)
	if d["draft"] != true || !strings.Contains(fmt.Sprint(d["next_action"]), "not in the saved story") {
		t.Fatalf("draft = %v", d)
	}
}

// Code writes and scene writes count issues in separate maps.
func TestStudioCodeIssueCountIsSeparate(t *testing.T) {
	s, _, _ := newStudioWriteRig(t, nil)
	s.SetPhase(PhaseFill, "s1")
	s.lastIssueCount["s1"] = 7
	read := codeCall(t, s, "get_code", "s1", nil, "")
	codeCall(t, s, "put_code", "s1", map[string]any{"source": "export default function ("}, read["rev"].(string))
	if s.lastIssueCount["s1"] != 7 || s.lastCodeIssueCount["s1"] == 0 {
		t.Fatalf("scene=%d code=%d", s.lastIssueCount["s1"], s.lastCodeIssueCount["s1"])
	}
}

// get_code returns source with literal angle brackets, yet never a closing tag start.
func TestStudioGetCodeKeepsAngleBrackets(t *testing.T) {
	ctx := context.Background()
	s, ps, id := newStudioWriteRig(t, nil)
	src := "export default function scene(api) { api.duration(5); for (let i=0;i<3&&i>-1;i++){} } // </x>"
	if _, _, err := ps.SaveCodeScene(ctx, id, "s1", []byte(src), MustNotExist); err != nil {
		t.Fatal(err)
	}
	s.SetPhase(PhaseRevise, "")
	res := s.Call(ctx, StudioCall{Tool: ToolStory, Operation: "get_code", Args: map[string]any{"scene_id": "s1"}})
	raw := string(res.JSON)
	if !strings.Contains(raw, "i<3&&i>-1") || strings.Contains(raw, "<"+"/") || strings.Contains(raw, string(rune(92))+"u003c") {
		t.Fatalf("get_code = %s", raw)
	}
	if got := answerOf(t, res)["source"]; got != src {
		t.Fatalf("source did not round-trip: %v", got)
	}
}

func TestStudioArgsRepairNumericStrings(t *testing.T) {
	got := repairStoryArgs(map[string]any{
		"scene_id": "s1",
		"actors":   []any{map[string]any{"cast": "anna", "scale": "1.5"}},
		"props":    []any{map[string]any{"item": "prop.lock", "scale": "0.75"}},
		"beats":    []any{map[string]any{"do": "wait", "duration": "2.5", "scale": "1.2"}},
	})
	data := got["data"].(map[string]any)
	actor := data["actors"].([]any)[0].(map[string]any)
	prop := data["props"].([]any)[0].(map[string]any)
	beat := data["beats"].([]any)[0].(map[string]any)
	if actor["scale"] != 1.5 || prop["scale"] != 0.75 || beat["duration"] != 2.5 || beat["scale"] != 1.2 {
		t.Fatalf("not repaired: %v %v %v", actor, prop, beat)
	}
}
