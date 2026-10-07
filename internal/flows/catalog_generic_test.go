package flows

import (
	"reflect"
	"testing"
)

func sampleToolSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"operation": map[string]any{"type": "string", "enum": []any{"list", "create", "delete"}, "description": "Operation"},
			"name":      map[string]any{"type": "string", "description": "Name"},
			"force":     map[string]any{"type": "boolean"},
			"limit":     map[string]any{"type": "integer"},
			"labels":    map[string]any{"type": "object", "additionalProperties": map[string]any{"type": "string"}},
			"config":    map[string]any{"type": "object"},
			"tags":      map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			"sections":  map[string]any{"type": "string", "description": "JSON array of sections"},
			"content":   map[string]any{"type": "string"},
			"_todo":     map[string]any{"type": "string"},
		},
		"required": []any{"operation", "name"},
	}
}

func TestParamsFromSchema(t *testing.T) {
	params, jsonStrings := paramsFromSchema(sampleToolSchema())
	var names, kinds []string
	for _, p := range params {
		names = append(names, p.Name)
		kinds = append(kinds, p.Kind)
	}
	wantNames := []string{"operation", "name", "config", "content", "force", "labels", "limit", "sections", "tags"}
	wantKinds := []string{ParamSegmented, ParamText, ParamJSON, ParamTextarea, ParamBool, ParamKeyValue, ParamNumber, ParamJSON, ParamTags}
	if !reflect.DeepEqual(names, wantNames) || !reflect.DeepEqual(kinds, wantKinds) {
		t.Fatalf("params = %v %v", names, kinds)
	}
	if !params[0].Required || !params[1].Required || params[2].Required {
		t.Fatal("required flags mismatch")
	}
	if len(params[0].Options) != 3 || params[0].Options[2].Value != "delete" || params[1].Label != "Name" || params[1].Help != "Name" {
		t.Fatalf("operation/name = %+v %+v", params[0], params[1])
	}
	if !jsonStrings["sections"] || jsonStrings["config"] {
		t.Fatalf("json strings = %v", jsonStrings)
	}
}

func TestGenericArgs(t *testing.T) {
	params, jsonStrings := paramsFromSchema(sampleToolSchema())
	args := genericArgs(params, jsonStrings, map[string]any{
		"operation": "create", "name": "x", "force": "ja", "limit": "5", "config": `{"a":1}`,
		"sections": []any{map[string]any{"type": "text"}}, "tags": "a, b,", "content": "", "unknown": "y",
	})
	want := map[string]any{"operation": "create", "name": "x", "force": true, "limit": 5.0, "config": map[string]any{"a": 1.0},
		"sections": `[{"type":"text"}]`, "tags": []any{"a", "b"}}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("args = %#v", args)
	}
}

func TestRefreshGenericTools(t *testing.T) {
	reg := NewRegistry()
	reg.MustRegister(&NodeDef{Type: GenericTypePrefix + "stale"})
	tools := []GenericTool{
		{Name: "docker", Category: "infrastructure", Description: "Manage containers", Schema: sampleToolSchema()},
		{Name: "ddg_search", Category: "network"},
		{Name: "skill__weather", Category: "skills"},
		{Name: "proxmox", Category: "infrastructure"},
	}
	if n := RefreshGenericTools(reg, tools, StaticEnv{"docker": {}}); n != 2 {
		t.Fatalf("registered %d generic nodes, want 2", n)
	}
	if _, ok := reg.Lookup(GenericTypePrefix + "stale"); ok {
		t.Fatal("old generic nodes must be removed")
	}
	def := lookupDef(t, reg, GenericTypePrefix+"docker")
	if def.Category != "tool:infrastructure" || def.Label != "Docker" || def.Description != "Manage containers" || def.Tool != "docker" || !def.UntrustedOutput {
		t.Fatalf("docker def = %+v", def)
	}
	if def.Availability().State != AvailableState || lookupDef(t, reg, GenericTypePrefix+"proxmox").Availability().State != NeedsSetupState {
		t.Fatal("availability must come from the env")
	}
	tools0 := &fakeTools{respond: toolReply(`Tool Output: {"status":"success","items":[1,2,3]}`)}
	res, err := execDef(def, map[string]any{"operation": "list", "name": "web"}, &Services{Tools: tools0})
	if err != nil || res.ItemCount != 3 {
		t.Fatalf("execute = %#v, %v", res, err)
	}
	if req := tools0.last(t); req.Tool != "docker" || !reflect.DeepEqual(req.Args, map[string]any{"operation": "list", "name": "web"}) {
		t.Fatalf("request = %+v", req)
	}
	if got := def.EffectsOf(&Node{Params: map[string]any{"operation": "delete"}}); !reflect.DeepEqual(got, []Effect{EffectDeletes, EffectSystemChange}) {
		t.Fatalf("docker delete effects = %v", got)
	}
}

func TestGenericEffects(t *testing.T) {
	cases := []struct {
		tool, category, op string
		want               []Effect
	}{
		{"execute_shell", "system", "", []Effect{EffectRunsCode}},
		{"execute_sudo", "system", "", []Effect{EffectRunsCode, EffectSystemChange}},
		{"docker", "infrastructure", "list", nil},
		{"docker", "infrastructure", "restart", []Effect{EffectSystemChange}},
		{"filesystem", "files", "write_file", []Effect{EffectWritesFiles}},
		{"filesystem", "files", "delete", []Effect{EffectDeletes}},
		{"filesystem", "files", "read_file", nil},
		{"send_youtube_video", "communication", "", []Effect{EffectSendsMessage}},
		{"fetch_email", "communication", "", nil},
		{"wake_on_lan", "smart_home", "", []Effect{EffectControlsDevices}},
		{"fritzbox_smarthome", "smart_home", "get_devices", nil},
		{"wikipedia_search", "network", "", nil},
	}
	for _, tc := range cases {
		if got := genericEffects(tc.tool, tc.category, tc.op); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("genericEffects(%s, %s, %s) = %v, want %v", tc.tool, tc.category, tc.op, got, tc.want)
		}
	}
	for _, name := range []string{"discover_tools", "skill__x", "tool__y", "game_maker_file", "send_email"} {
		if !IsGenericToolExcluded(name) {
			t.Errorf("%s must be excluded", name)
		}
	}
	if IsGenericToolExcluded("docker") || IsGenericToolExcluded("filesystem") {
		t.Fatal("docker and filesystem stay available as generic nodes")
	}
}
