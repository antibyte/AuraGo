package storymaker

import (
	"encoding/json"
	"testing"
)

func TestStudioArgsRepairFlatAndStringForms(t *testing.T) {
	got := repairStoryArgs(map[string]any{
		"operation": "put_scene", "action": ToolNameForArgsTest, "project_id": "someone-elses",
		"id": "s2", "environment": "env.office", "duration": "6s",
		"actors": []any{map[string]any{"cast": "anna", "at": "30,88"}},
		"props":  []any{map[string]any{"item": "prop.lock", "at": "right"}},
		"beats":  []any{map[string]any{"do": "walk", "who": "anna", "to": "70, 88"}},
	})
	if got["scene_id"] != "s2" {
		t.Fatalf("scene_id = %v", got["scene_id"])
	}
	for _, key := range []string{"operation", "action", "project_id", "id", "environment"} {
		if _, present := got[key]; present {
			t.Errorf("%s stayed at the top level: %v", key, got)
		}
	}
	raw, err := json.Marshal(got["data"])
	if err != nil {
		t.Fatal(err)
	}
	sc, issues, err := DecodeScene(raw)
	if err != nil {
		t.Fatalf("DecodeScene: %v (%+v)", err, issues)
	}
	if sc.Environment != "env.office" || sc.Duration == nil || sc.Duration.Seconds != 6 {
		t.Errorf("flat fields: %+v", sc)
	}
	if a := sc.Actors[0].At; !a.Point || a.X != 30 || a.Y != 88 {
		t.Errorf("actor at = %+v", a)
	}
	if p := sc.Props[0].At; p.Point || p.Spot != "right" {
		t.Errorf("prop at = %+v", p)
	}
	if b := sc.Beats[0].To; !b.Point || b.X != 70 || b.Y != 88 {
		t.Errorf("beat to = %+v", b)
	}

	got = repairStoryArgs(map[string]any{"scene": "s3", "data": `{"duration":"auto","narration":"Hallo"}`})
	data, isObject := got["data"].(map[string]any)
	if got["scene_id"] != "s3" || !isObject || data["duration"] != "auto" || data["narration"] != "Hallo" {
		t.Errorf("JSON-string data: %v", got)
	}

	got = repairStoryArgs(map[string]any{"scene_id": "s4", "narration": "flat", "data": map[string]any{"narration": "kept", "id": "s9"}})
	data = got["data"].(map[string]any)
	if got["scene_id"] != "s4" || data["narration"] != "kept" || data["id"] != nil {
		t.Errorf("data wins over flat fields, and the bound id stays: %v", got)
	}

	got = repairStoryArgs(map[string]any{"id": "s5"})
	if _, present := got["data"]; present || got["scene_id"] != "s5" {
		t.Errorf("a get must stay without data: %v", got)
	}

	if got = repairStoryArgs(map[string]any{"data": "no json"}); got["data"] != "no json" {
		t.Errorf("unreadable data must reach the handler: %v", got)
	}
}

// ToolNameForArgsTest is the action field a text-mode call carries.
const ToolNameForArgsTest = "storymaker_story"
