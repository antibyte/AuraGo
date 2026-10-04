package flows

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

// genericSubset reports whether every effect in a is in b.
func genericSubset(a, b []Effect) bool {
	for _, e := range a {
		found := false
		for _, f := range b {
			found = found || e == f
		}
		if !found {
			return false
		}
	}
	return true
}

// An operation that is not a literal gets the worst case, never "no effects": the
// union over the listed operations, or every effect without a list.
func TestGenericEffectsWorstCase(t *testing.T) {
	reg := genericTestRegistry(t)
	docker := lookupDef(t, reg, "tool.docker")
	dockerWorst := []Effect{EffectRunsCode, EffectDeletes, EffectSystemChange}
	for _, op := range []any{"{{trigger.data.op}}", "get_{{x}}", "list_containers{{x}}", `\{{x}}`, "", "  ", nil, 3.0, true,
		[]any{"list_containers"}, map[string]any{}, "drop_everything", strings.Repeat("l", 101), "list containers"} {
		if got := docker.EffectsOf(&Node{Params: map[string]any{"operation": op}}); !reflect.DeepEqual(got, dockerWorst) {
			t.Errorf("docker operation %#v: effects %v, want %v", op, got, dockerWorst)
		}
	}
	if got := docker.EffectsOf(&Node{}); !reflect.DeepEqual(got, dockerWorst) {
		t.Errorf("docker without operation: %v", got)
	}
	if got := docker.EffectsOf(nil); !reflect.DeepEqual(got, dockerWorst) {
		t.Errorf("docker nil node: %v", got)
	}
	// The returned slice is the caller's.
	got := docker.EffectsOf(nil)
	got[0] = EffectSendsMessage
	if again := docker.EffectsOf(nil); !reflect.DeepEqual(again, dockerWorst) {
		t.Errorf("a caller changed the worst case: %v", again)
	}
	// Literal operations, in any letter case, are classified.
	for op, want := range map[string][]Effect{"list_containers": nil, " LIST_CONTAINERS ": nil, "Run": {EffectRunsCode, EffectSystemChange},
		"remove": {EffectDeletes, EffectSystemChange}} {
		if got := docker.EffectsOf(&Node{Params: map[string]any{"operation": op}}); !reflect.DeepEqual(got, want) {
			t.Errorf("docker %q: %v, want %v", op, got, want)
		}
	}
	// Free-text operations: a template or other text gets everything.
	adguard := lookupDef(t, reg, "tool.adguard")
	all := []Effect{EffectControlsDevices, EffectRunsCode, EffectDeletes, EffectSystemChange}
	for op, want := range map[any][]Effect{"{{x}}": all, "get_{{x}}": all, "status": nil, "query_log_clear": {EffectControlsDevices, EffectDeletes, EffectSystemChange},
		"filtering toggle": all, 7.0: all, "": all} {
		if got := adguard.EffectsOf(&Node{Params: map[string]any{"operation": op}}); !reflect.DeepEqual(got, want) {
			t.Errorf("adguard %#v: %v, want %v", op, got, want)
		}
	}
	// "action" selects the operation when there is no "operation".
	truenas := lookupDef(t, reg, "tool.truenas")
	for op, want := range map[string][]Effect{"truenas_dataset_delete": {EffectDeletes, EffectSystemChange},
		"truenas_health": {EffectSystemChange}, "{{x}}": {EffectDeletes, EffectSystemChange}} {
		if got := truenas.EffectsOf(&Node{Params: map[string]any{"action": op}}); !reflect.DeepEqual(got, want) {
			t.Errorf("truenas %q: %v, want %v", op, got, want)
		}
	}
	// A tool without an operation has fixed effects, nil node included.
	if got := lookupDef(t, reg, "tool.execute_shell").EffectsOf(nil); !reflect.DeepEqual(got, []Effect{EffectRunsCode}) {
		t.Errorf("execute_shell: %v", got)
	}
	// Every listed operation stays within the union, and every operation within everything.
	for _, tool := range genericTestTools() {
		def := lookupDef(t, reg, GenericTypePrefix+tool.Name)
		opName, choices := genericOperation(def.Params)
		worst := def.EffectsOf(nil)
		for _, c := range choices {
			if got := def.EffectsOf(&Node{Params: map[string]any{opName: c}}); !genericSubset(got, worst) {
				t.Errorf("%s %s: %v not within %v", tool.Name, c, got, worst)
			}
		}
		for _, op := range []string{"", "run", "delete_all", "send", "write", "list", "{{x}}", "x y"} {
			if got := genericEffects(tool.Name, tool.Category, op); !genericSubset(got, genericAllEffects(tool.Name, tool.Category)) {
				t.Errorf("%s %q: %v not within everything", tool.Name, op, got)
			}
		}
	}
}

func TestGenericReadOperations(t *testing.T) {
	for _, op := range []string{"list", "LIST", " list ", "get_devices", "read_file", "stats_top", "status", "query", "check",
		"diff_files", "list_downloads"} {
		if !isReadOperation(op) {
			t.Errorf("%q must be a read", op)
		}
	}
	for _, op := range []string{"", "get_{{x}}", "{{x}}", `get_\{{x}}`, "list x", "query_log_clear", "get_and_delete", "listen",
		"show", "show_notification", "log_problem", "mark_tam_message_read", "checkpoint", "counter_reset", "get_download",
		strings.Repeat("get_", 30), "get\x00", "lïst"} {
		if isReadOperation(op) {
			t.Errorf("%q must not be a read", op)
		}
	}
}

// The tool lists cover the real AuraGo tools; operations from their schemas.
func TestGenericEffectsOfRealTools(t *testing.T) {
	var (
		msg = EffectSendsMessage
		wr  = EffectWritesFiles
		dev = EffectControlsDevices
		run = EffectRunsCode
		del = EffectDeletes
		sys = EffectSystemChange
	)
	cases := []struct {
		tool, category, op string
		want               []Effect
	}{
		{"docker", "infrastructure", "run", []Effect{run, sys}},
		{"homepage_project", "infrastructure", "exec", []Effect{run, sys}},
		{"meshcentral", "infrastructure", "run_command", []Effect{dev, run, sys}},
		{"huggingface", "data_apis", "job_run_python", []Effect{run}},
		{"ansible", "infrastructure", "playbook", []Effect{run, sys}},
		{"ansible", "infrastructure", "check", nil},
		{"remote_control_shell", "infrastructure", "shell_session_read", []Effect{run}},
		{"virtual_workspace", "infrastructure", "list", nil},
		{"virtual_workspace", "infrastructure", "exec", []Effect{run, sys}},
		{"sql_query", "infrastructure", "query", []Effect{run}},
		{"grafana", "smart_home", "query", nil},
		{"package_manager", "system", "install", []Effect{sys}},
		{"process_management", "system", "kill", []Effect{sys}},
		{"ollama", "infrastructure", "pull", []Effect{sys}},
		{"fritzbox_network", "smart_home", "add_port_forward", []Effect{dev, sys}},
		{"fritzbox_system", "smart_home", "reboot", []Effect{dev, sys}},
		{"adguard", "smart_home", "query_log_clear", []Effect{dev, del, sys}},
		{"manage_webhooks", "communication", "create", []Effect{sys}},
		{"send_image", "media", "", []Effect{msg}},
		{"call_webhook", "communication", "", []Effect{msg}},
		{"sip_phone", "infrastructure", "dial", []Effect{msg}},
		{"telnyx_sms", "communication", "send", []Effect{msg}},
		{"agentmail_messages", "communication", "forward_message", []Effect{msg}},
		{"agentmail_messages", "communication", "list_messages", nil},
		{"cyd_display", "communication", "show", []Effect{dev}},
		{"fetch_discord", "communication", "", nil},
		{"list_email_accounts", "communication", "", nil},
		{"file_editor", "files", "str_replace", []Effect{wr}},
		{"obsidian", "files", "delete_note", []Effect{del}},
		{"text_diff", "files", "diff_files", nil},
		{"detect_file_type", "files", "", nil},
		{"bluetooth", "media", "play", []Effect{dev}},
		{"three_d_printer", "infrastructure", "start_print", []Effect{dev}},
		// a send word sends outside the communication category for google_workspace
		{"google_workspace", "files", "gmail_send", []Effect{msg, wr}},
		// no read verb first: the files rule lists a (not risky) write, but no message
		{"google_workspace", "files", "gmail_list", []Effect{wr}},
		{"google_workspace", "files", "calendar_create", []Effect{wr}},
		// file tools outside the files category write with a write word
		{"homepage_file", "infrastructure", "write_file", []Effect{wr}},
		{"homepage_file", "infrastructure", "json_edit", []Effect{wr}},
		{"homepage_file", "infrastructure", "read_file", nil},
		{"virtual_desktop_files", "infrastructure", "patch_file", []Effect{wr}},
		{"virtual_desktop_files", "infrastructure", "export_file", []Effect{wr}},
		{"virtual_desktop_files", "infrastructure", "delete_file", []Effect{del}},
		{"office_document", "infrastructure", "write", []Effect{wr}},
		{"office_document", "infrastructure", "read", nil},
		{"office_workbook", "infrastructure", "set_cell", []Effect{wr}},
		{"video_download", "media", "download", []Effect{wr}},
		{"video_download", "media", "search", nil},
		{"remote_control_files", "infrastructure", "write_file", []Effect{wr, sys}},
		{"s3_storage", "infrastructure", "upload", []Effect{wr}},
		// tools that write a file with every call that is no read
		{"transfer_remote_file", "infrastructure", "", []Effect{wr}},
		{"web_capture", "network", "screenshot", []Effect{wr}},
		{"media_conversion", "media", "audio_convert", []Effect{wr}},
		{"media_conversion", "media", "info", nil},
		{"tts", "media", "", []Effect{wr}},
		{"certificate_manager", "network", "generate_self_signed", []Effect{wr}},
		{"certificate_manager", "network", "check_remote", nil},
		{"send_document", "media", "", []Effect{msg}},
	}
	for _, tc := range cases {
		if got := genericEffects(tc.tool, tc.category, tc.op); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("genericEffects(%s, %s, %s) = %v, want %v", tc.tool, tc.category, tc.op, got, tc.want)
		}
	}
}

// Validate refuses a literal operation the schema does not list.
func TestGenericValidateOperation(t *testing.T) {
	def := lookupDef(t, genericTestRegistry(t), "tool.docker")
	vc := ValidateContext{Mode: ModePublish, Now: time.Now(), Location: time.UTC}
	for _, c := range []struct {
		op  any
		bad bool
	}{
		{"drop_all", true}, {5.0, true}, {[]any{"run"}, true}, {strings.Repeat("r", 101), true},
		{"run", false}, {"RUN", false}, {"{{trigger.data.op}}", false}, {"", false}, {nil, false},
	} {
		issues := def.Validate(&Node{ID: testNodeID(1), Key: "dk", Params: map[string]any{"operation": c.op}}, vc)
		if c.bad && (len(issues) != 1 || issues[0].Code != IssueParamInvalid || issues[0].Param != "operation") || !c.bad && len(issues) != 0 {
			t.Errorf("operation %#v: issues %+v", c.op, issues)
		}
	}
	if issues := def.Validate(nil, vc); issues != nil {
		t.Errorf("nil node: %+v", issues)
	}
	if lookupDef(t, genericTestRegistry(t), "tool.adguard").Validate != nil {
		t.Error("a tool without a list of operations has nothing to validate")
	}
}

// Hooks and Execute run on raw or resolved params of any shape: no panic, bounded and
// valid messages, effects within everything the tool can do.
func TestGenericHooksSurviveOddParams(t *testing.T) {
	reg := genericTestRegistry(t)
	vc := ValidateContext{Mode: ModePublish, Now: triggerNow, Location: time.UTC}
	values := oddParamValues()
	var failures []string
	fail := func(format string, args ...any) { failures = append(failures, fmt.Sprintf(format, args...)) }
	runs := 0
	for _, tool := range genericTestTools() {
		def := lookupDef(t, reg, GenericTypePrefix+tool.Name)
		names := []string{"operation", "action", "unknown_param"}
		for _, p := range def.Params {
			names = append(names, p.Name)
		}
		everything := genericAllEffects(tool.Name, tool.Category)
		for _, name := range names {
			for _, odd := range values {
				runs++
				node := &Node{ID: testNodeID(1), Key: "gen", Type: def.Type, Params: map[string]any{name: odd.v}}
				var effects []Effect
				var issues []Issue
				if r := catchPanic(func() {
					effects = def.EffectsOf(node)
					if def.Validate != nil {
						issues = def.Validate(node, vc)
					}
					def.OutputPorts(node)
					def.FieldsOf(node)
					def.Availability()
				}); r != nil {
					fail("%s %s=%s: hook panic %v", tool.Name, name, odd.name, r)
					continue
				}
				if !genericSubset(effects, everything) {
					fail("%s %s=%s: effects %v", tool.Name, name, odd.name, effects)
				}
				for _, is := range issues {
					if len(is.Message) > maxEchoMessageBytes || !utf8.ValidString(is.Message) {
						fail("%s %s=%s: issue %q", tool.Name, name, odd.name, is.Message)
					}
				}
				var execErr error
				if r := catchPanic(func() {
					_, execErr = execDef(def, map[string]any{name: odd.v}, &Services{Tools: &fakeTools{}})
				}); r != nil {
					fail("%s %s=%s: Execute panic %v", tool.Name, name, odd.name, r)
					continue
				}
				if execErr != nil {
					var ne *NodeError
					if !errors.As(execErr, &ne) || ne == nil || ne.Code == "" || len(ne.Message) > maxEchoMessageBytes || !utf8.ValidString(ne.Message) {
						fail("%s %s=%s: error %v", tool.Name, name, odd.name, execErr)
					}
				}
			}
		}
	}
	if runs < 1000 {
		t.Fatalf("only %d runs", runs)
	}
	if len(failures) > 0 {
		t.Fatalf("%d failures, first: %s", len(failures), strings.Join(failures[:min(len(failures), 10)], "\n"))
	}
}

// paramsFromSchema accepts any schema and keeps the result bounded.
func TestParamsFromSchemaSurvivesOddSchemas(t *testing.T) {
	cyclic := map[string]any{}
	cyclic["self"] = cyclic
	hugeEnum := make([]any, maxGenericOptions+1)
	for i := range hugeEnum {
		hugeEnum[i] = fmt.Sprintf("op_%d", i)
	}
	many := map[string]any{}
	for i := 0; i < 1000; i++ {
		many[fmt.Sprintf("p%04d", i)] = map[string]any{"type": "string"}
	}
	odd := map[string]any{
		"properties": map[string]any{
			"": map[string]any{}, "a.b": map[string]any{}, "x[0]": map[string]any{}, "é": map[string]any{}, "1st": map[string]any{},
			"not_a_map": "text", "nil_prop": nil, "cyc": cyclic, "list_prop": []any{1.0},
			"huge_desc":   map[string]any{"type": "string", "description": strings.Repeat("ü", 100000) + " json object"},
			"num_desc":    map[string]any{"type": "integer", "description": 5.0},
			"huge_enum":   map[string]any{"type": "string", "enum": hugeEnum},
			"string_enum": map[string]any{"type": "string", "enum": []string{"a", "b", "a"}},
			"mixed_enum": map[string]any{"enum": []any{"a", 1.0, true, nil, map[string]any{}, []any{}, strings.Repeat("x", 300),
				"a", "\xff", ""}},
			"empty_enum":  map[string]any{"type": "boolean", "enum": []any{nil, map[string]any{}}},
			"nullable":    map[string]any{"type": []any{"null", "boolean"}},
			"type_list":   map[string]any{"type": []string{"string", "null"}},
			"bad_type":    map[string]any{"type": map[string]any{"x": 1.0}},
			"nested":      map[string]any{"type": "object", "properties": map[string]any{"a": cyclic}},
			"kv":          map[string]any{"type": "object", "additionalProperties": map[string]any{"type": []any{"string"}}},
			"tags":        map[string]any{"type": "array", "items": "string"},
			"vault_keys":  map[string]any{"type": "array"},
			"_todo":       map[string]any{"type": "string"},
			"wake_agent":  map[string]any{"type": "boolean"},
			"output_path": map[string]any{"type": "string"},
		},
		"required": []any{"not_a_map", 3.0, nil, map[string]any{}, "missing"},
	}
	schemas := map[string]map[string]any{
		"nil": nil, "empty": {}, "properties not a map": {"properties": []any{"a"}}, "required not a list": {"properties": map[string]any{"a": nil}, "required": "a"},
		"odd": odd, "many": {"properties": many},
	}
	for name, schema := range schemas {
		var params []ParamSpec
		var jsonStrings map[string]bool
		if r := catchPanic(func() { params, jsonStrings = paramsFromSchema(schema) }); r != nil {
			t.Fatalf("%s: panic %v", name, r)
		}
		if len(params) > maxGenericParams || jsonStrings == nil {
			t.Errorf("%s: %d params, json strings %v", name, len(params), jsonStrings)
		}
		for _, p := range params {
			if !genericNameOK(p.Name, maxGenericParamName, true) || p.Kind == "" || utf8.RuneCountInString(p.Help) > maxGenericHelpRunes+1 ||
				!utf8.ValidString(p.Help) || len(p.Options) > maxGenericOptions {
				t.Errorf("%s: param %+.200v", name, p)
			}
		}
	}
	params, jsonStrings := paramsFromSchema(odd)
	kinds := map[string]string{}
	for _, p := range params {
		kinds[p.Name] = p.Kind
	}
	wantKinds := map[string]string{"not_a_map": ParamText, "nil_prop": ParamText, "cyc": ParamText, "list_prop": ParamText,
		"huge_desc": ParamJSON, "num_desc": ParamNumber, "huge_enum": ParamText, "string_enum": ParamSelect, "mixed_enum": ParamSelect,
		"empty_enum": ParamBool, "nullable": ParamBool, "type_list": ParamText, "bad_type": ParamText, "nested": ParamJSON,
		"kv": ParamKeyValue, "tags": ParamJSON, "output_path": ParamText}
	if !reflect.DeepEqual(kinds, wantKinds) {
		t.Errorf("kinds = %v,\nwant %v", kinds, wantKinds)
	}
	if !jsonStrings["huge_desc"] || len(jsonStrings) != 1 {
		t.Errorf("json strings = %v", jsonStrings)
	}
	for _, p := range params {
		switch p.Name {
		case "not_a_map":
			if !p.Required || params[0].Name != "not_a_map" {
				t.Errorf("required not_a_map must come first: %+v", params[0])
			}
		case "string_enum":
			if len(p.Options) != 2 || p.Options[1].Value != "b" {
				t.Errorf("string enum options = %+v", p.Options)
			}
		case "mixed_enum":
			if got := fmt.Sprint(p.Options); got != "[{a  a} {1  1} {true  true}]" {
				t.Errorf("mixed enum options = %s", got)
			}
		case "output_path":
			if !p.SensitiveSink {
				t.Error("output_path is a sink")
			}
		}
	}
}

// A tool description from the catalog is bounded and valid in the definition.
func TestGenericDefinitionTextIsBounded(t *testing.T) {
	reg := NewRegistry()
	RefreshGenericTools(reg, []GenericTool{{Name: "x_tool", Category: "bad category!", Description: strings.Repeat("d", 5000) + "\xff"}}, nil)
	def := lookupDef(t, reg, "tool.x_tool")
	if def.Category != "tool:other" || utf8.RuneCountInString(def.Description) > maxGenericDescRunes+1 || !utf8.ValidString(def.Description) ||
		def.Label != "X tool" {
		t.Errorf("def = %q %q %d runes", def.Category, def.Label, utf8.RuneCountInString(def.Description))
	}
	if def.Availability().State != NeedsSetupState {
		t.Errorf("no env: %+v", def.Availability())
	}
}
