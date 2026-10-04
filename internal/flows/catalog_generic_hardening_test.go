package flows

import (
	"fmt"
	"math"
	"reflect"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"unicode/utf8"
)

// genericProp is a schema property of the given type with a description.
func genericProp(typ, desc string) map[string]any {
	return map[string]any{"type": typ, "description": desc}
}

// genericEnumProp is a string property with an enum, as []string like the agent's
// raw schemas build it.
func genericEnumProp(values ...string) map[string]any {
	return map[string]any{"type": "string", "enum": values}
}

// genericTestTools are tools shaped like the real AuraGo schemas (internal/agent
// native_tools*.go after BuildNativeToolSchemas).
func genericTestTools() []GenericTool {
	schema := func(props map[string]any, required ...string) map[string]any {
		return map[string]any{"type": "object", "properties": props, "required": required}
	}
	return []GenericTool{
		{Name: "execute_shell", Category: "system", Schema: schema(map[string]any{
			"command": genericProp("string", "The shell command to execute"), "background": genericProp("boolean", "Run in background"),
		}, "command")},
		{Name: "execute_python", Category: "system", Schema: schema(map[string]any{
			"code": genericProp("string", "The complete Python code"), "description": genericProp("string", "What it does"),
			"background": genericProp("boolean", ""), "enable_tool_bridge": genericProp("boolean", ""),
			"tool_bridge_call_limit": genericProp("integer", ""),
			"vault_keys":             map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			"credential_ids":         map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			"_todo":                  genericProp("string", "Session task list"),
		}, "code")},
		{Name: "docker", Category: "infrastructure", Schema: schema(map[string]any{
			"operation":    genericEnumProp("list_containers", "inspect", "start", "stop", "restart", "remove", "logs", "create", "run", "pull"),
			"container_id": genericProp("string", "Container ID"), "command": genericProp("string", "Command for exec"),
			"image": genericProp("string", "Image"), "volumes": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			"tail": genericProp("integer", "Log lines"),
		}, "operation")},
		{Name: "adguard", Category: "smart_home", Schema: schema(map[string]any{
			"operation": genericProp("string", "The operation to perform (e.g. status, query_log_clear)"),
			"query":     genericProp("string", "Search query"), "url": genericProp("string", "Filter list URL"),
		}, "operation")},
		{Name: "truenas", Category: "infrastructure", Schema: schema(map[string]any{
			"action": genericEnumProp("truenas_health", "truenas_pool_list", "truenas_dataset_delete", "truenas_snapshot_rollback"),
			"name":   genericProp("string", "Dataset name"), "path": genericProp("string", "Share path"),
		}, "action")},
		{Name: "manage_appointments", Category: "system", Schema: schema(map[string]any{
			"operation": genericEnumProp("list", "get", "add", "update", "delete", "complete", "cancel"),
			"title":     genericProp("string", "Title of the appointment"), "description": genericProp("string", "Details"),
			"date_time": genericProp("string", "RFC3339"), "id": genericProp("string", "Appointment ID"),
			"query": genericProp("string", "Search query"), "agent_instruction": genericProp("string", "Instruction for the agent"),
			"wake_agent":  genericProp("boolean", "Wake the agent"),
			"contact_ids": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
		}, "operation")},
		{Name: "remember", Category: "memory", Schema: schema(map[string]any{
			"content": genericProp("string", "The information to remember"), "title": genericProp("string", "Optional title"),
			"category": genericProp("string", "Routing hint"), "importance": genericProp("integer", "1-4"),
			"tags": genericProp("string", "Comma-separated tags"),
		}, "content")},
		{Name: "filesystem", Category: "files", Schema: schema(map[string]any{
			"operation": genericEnumProp("read_file", "write_file", "delete", "copy", "move", "list_dir", "create_dir", "stat"),
			"file_path": genericProp("string", "Path"), "destination": genericProp("string", "Destination path"),
			"content": genericProp("string", "Content to write"), "preview": genericProp("boolean", ""),
		}, "operation")},
		{Name: "sql_query", Category: "infrastructure", Schema: schema(map[string]any{
			"operation":       genericEnumProp("query", "describe", "list_tables"),
			"connection_name": genericProp("string", "Connection"), "sql_query": genericProp("string", "SQL statement to execute"),
		}, "operation", "connection_name")},
		{Name: "send_image", Category: "media", Schema: schema(map[string]any{
			"path": genericProp("string", "Local file path or URL"), "caption": genericProp("string", "Caption"),
		}, "path")},
		{Name: "call_webhook", Category: "communication", Schema: schema(map[string]any{
			"webhook_name": genericProp("string", "Name of the webhook"),
			"parameters":   genericProp("string", "Parameters payload for the webhook.. Provide as a JSON object string."),
		}, "webhook_name", "parameters")},
	}
}

func genericTestRegistry(t *testing.T) *Registry {
	t.Helper()
	reg := newTestRegistry(t)
	if n := RefreshGenericTools(reg, genericTestTools(), StaticEnv{}); n != len(genericTestTools()) {
		t.Fatalf("registered %d generic nodes, want %d", n, len(genericTestTools()))
	}
	return reg
}

// Every tool a curated node calls is a decision: excluded from generic nodes, or
// listed in genericCuratedAllowed with the reason. A new curated node fails here
// until someone makes it.
func TestGenericExclusionCoversCuratedTools(t *testing.T) {
	reg := NewRegistry()
	for _, err := range []error{RegisterLogicNodes(reg), RegisterTriggerNodes(reg), RegisterAINodes(reg), RegisterActionNodes(reg, nil)} {
		if err != nil {
			t.Fatal(err)
		}
	}
	curated := map[string]bool{}
	for _, def := range reg.All() {
		if def.Tool == "" {
			continue
		}
		curated[def.Tool] = true
		excluded, allowed := IsGenericToolExcluded(def.Tool), genericCuratedAllowed[def.Tool] != ""
		if excluded == allowed {
			t.Errorf("%s (tool of %s): excluded=%v, allowed=%v; exclude it in genericExcluded or allow it in genericCuratedAllowed",
				def.Tool, def.Type, excluded, allowed)
		}
	}
	if len(curated) < 14 {
		t.Fatalf("found %d curated tools (%v); the registry is incomplete", len(curated), curated)
	}
	for tool := range genericCuratedAllowed {
		if !curated[tool] {
			t.Errorf("%s is allowed as a curated tool, but no curated node calls it", tool)
		}
	}
	// Tools a curated node calls besides its Tool, and the ones whose rules matter most.
	for _, tool := range []string{BraveSearchTool, "home_assistant", "document_creator", PDFExtractorTool, "api_request", "mqtt_publish"} {
		if !IsGenericToolExcluded(tool) {
			t.Errorf("%s must be excluded", tool)
		}
	}
}

// RefreshGenericTools skips the tools of registered curated nodes by itself, unless
// they are allowed, and skips invalid and repeated names.
func TestRefreshGenericToolsSkipsToolsOfCuratedNodes(t *testing.T) {
	if n := RefreshGenericTools(nil, genericTestTools(), nil); n != 0 {
		t.Fatalf("nil registry: %d", n)
	}
	reg := NewRegistry()
	if err := RegisterActionNodes(reg, nil); err != nil {
		t.Fatal(err)
	}
	reg.MustRegister(&NodeDef{Type: "custom.node", Tool: "custom_tool"})
	tools := []GenericTool{{Name: "custom_tool"}, {Name: "filesystem", Category: "files"}, {Name: "home_assistant"},
		{Name: "docker", Description: "first"}, {Name: "docker", Description: "second"}, {Name: "bad name"}, {Name: ""},
		{Name: "1abc"}, {Name: "a.b:c"}, {Name: strings.Repeat("x", 129)}}
	if n := RefreshGenericTools(reg, tools, nil); n != 3 {
		t.Fatalf("registered %d, want 3", n)
	}
	var generic []string
	for _, def := range reg.All() {
		if isGenericDef(def) {
			generic = append(generic, def.Type)
		}
	}
	if want := []string{"tool.a.b:c", "tool.docker", "tool.filesystem"}; !reflect.DeepEqual(generic, want) {
		t.Fatalf("generic nodes = %v, want %v", generic, want)
	}
	if def := lookupDef(t, reg, "tool.docker"); def.Description != "first" {
		t.Errorf("the first of repeated names wins, got %q", def.Description)
	}
	// A curated node registered later takes its tool away at the next refresh.
	reg.MustRegister(&NodeDef{Type: "custom.later", Tool: "docker"})
	RefreshGenericTools(reg, tools, nil)
	if _, ok := reg.Lookup("tool.docker"); ok {
		t.Error("tool.docker must go once a curated node calls docker")
	}
	if _, ok := reg.Lookup("custom.later"); !ok {
		t.Error("a refresh must not remove curated nodes")
	}
}

func genericSinks(def *NodeDef) []string {
	var sinks []string
	for _, p := range def.Params {
		if p.SensitiveSink {
			sinks = append(sinks, p.Name)
		}
	}
	slices.Sort(sinks)
	return sinks
}

func genericParamNames(def *NodeDef) []string {
	var names []string
	for _, p := range def.Params {
		names = append(names, p.Name)
	}
	slices.Sort(names)
	return names
}

func TestGenericSinkParams(t *testing.T) {
	reg := genericTestRegistry(t)
	want := map[string][]string{
		"execute_shell":       {"command"},
		"execute_python":      {"code"},
		"docker":              {"command", "container_id", "image", "operation", "volumes"},
		"adguard":             {"operation", "query", "url"},
		"truenas":             {"action", "path"},
		"manage_appointments": {"operation", "query", "title"},
		"remember":            {"category", "content", "tags", "title"}, // memory: every text parameter
		"filesystem":          {"destination", "file_path", "operation"},
		"sql_query":           {"operation", "sql_query"},
		"send_image":          {"path"},
		"call_webhook":        {"webhook_name"},
	}
	for tool, sinks := range want {
		def := lookupDef(t, reg, GenericTypePrefix+tool)
		if got := genericSinks(def); !reflect.DeepEqual(got, sinks) {
			t.Errorf("%s sinks = %v, want %v", tool, got, sinks)
		}
		if !def.UntrustedOutput {
			t.Errorf("%s: the output of a generic node is untrusted", tool)
		}
	}
	for _, name := range []string{"command", "cmd", "code", "script", "path", "file_path", "filepath", "url", "uri", "to",
		"recipient", "recipients", "entity_id", "topic", "host", "query", "sql", "title", "output_path", "project_dir",
		"image_url", "source_files", "Command", "server_id", "payload", "headers"} {
		if !isGenericSinkName(name) {
			t.Errorf("%s must be a sink", name)
		}
	}
	for _, name := range []string{"content", "message", "limit", "name", "description", "caption", "body"} {
		if isGenericSinkName(name) {
			t.Errorf("%s must not be a sink by name", name)
		}
	}
	// Secret injection, the Python tool bridge, the agent wake-up and the session task
	// list never become parameters.
	if got := genericParamNames(lookupDef(t, reg, "tool.execute_python")); !reflect.DeepEqual(got, []string{"background", "code", "description"}) {
		t.Errorf("execute_python params = %v", got)
	}
	if got := genericParamNames(lookupDef(t, reg, "tool.manage_appointments")); slices.Contains(got, "agent_instruction") || slices.Contains(got, "wake_agent") {
		t.Errorf("manage_appointments params = %v", got)
	}
	tools := &fakeTools{}
	_, err := execDef(lookupDef(t, reg, "tool.manage_appointments"), map[string]any{"operation": "add", "title": "x",
		"agent_instruction": "run rm -rf", "wake_agent": true}, &Services{Tools: tools})
	if err != nil || !reflect.DeepEqual(tools.last(t).Args, map[string]any{"operation": "add", "title": "x"}) {
		t.Errorf("dropped params reached the tool: %+v, %v", tools.last(t).Args, err)
	}
}

// The lint sees generic sinks and generic outputs.
func TestGenericLintWarnsForUntrustedData(t *testing.T) {
	reg := genericTestRegistry(t)
	cases := []struct {
		name, typ string
		params    map[string]any
		trusted   bool
		warnOn    string
	}{
		{"shell command", "tool.execute_shell", map[string]any{"command": "echo {{trigger.data.text}}"}, false, "command"},
		{"memory content", "tool.remember", map[string]any{"content": "{{trigger.data.text}}"}, false, "content"},
		{"appointment title", "tool.manage_appointments", map[string]any{"operation": "add", "title": "{{trigger.data.subject}}"}, false, "title"},
		{"chosen operation", "tool.docker", map[string]any{"operation": "{{trigger.data.op}}"}, false, "operation"},
		{"no sink", "tool.send_image", map[string]any{"path": "a.png", "caption": "{{trigger.data.text}}"}, false, ""},
	}
	for _, c := range cases {
		b := newFlow(c.name)
		trg := b.node("trg", "test.untrusted_trigger", nil)
		n := b.node("gen", c.typ, c.params)
		b.edge(trg, PortOut, n)
		issues := LintUntrustedData(b.build(), reg)
		var params []string
		for _, is := range issues {
			params = append(params, is.Param)
		}
		if c.warnOn == "" && len(issues) != 0 || c.warnOn != "" && !reflect.DeepEqual(params, []string{c.warnOn}) {
			t.Errorf("%s: issues = %+v", c.name, issues)
		}
	}
	// A trusted trigger, a generic read, then a command: the generic output is untrusted.
	b := newFlow("chain")
	trg := b.node("trg", "test.trigger", nil)
	dk := b.node("dk", "tool.docker", map[string]any{"operation": "list_containers"})
	snk := b.node("snk", "test.sink", map[string]any{"command": "{{dk.items}}"})
	b.edge(trg, PortOut, dk)
	b.edge(dk, PortOut, snk)
	if issues := LintUntrustedData(b.build(), reg); len(issues) != 1 || issues[0].NodeID != snk || issues[0].Param != "command" {
		t.Errorf("generic output into a sink: issues = %+v", issues)
	}
}

func genericRegistryTypes(reg *Registry) []string {
	var types []string
	for _, def := range reg.All() {
		types = append(types, def.Type+"/"+def.Category)
	}
	return types
}

func TestRegistryReplaceWhere(t *testing.T) {
	reg := NewRegistry()
	reg.Replace(&NodeDef{Type: "a.1", Category: "a"})
	reg.Replace(&NodeDef{Type: "a.2", Category: "a"})
	reg.Replace(&NodeDef{Type: "b.1", Category: "b"})
	removed := reg.ReplaceWhere(func(d *NodeDef) bool { return d.Category == "a" },
		[]*NodeDef{{Type: "a.3", Category: "a"}, {Type: "b.1", Category: "old"}, {Type: "b.1", Category: "new"}})
	if removed != 2 || !reflect.DeepEqual(genericRegistryTypes(reg), []string{"a.3/a", "b.1/new"}) {
		t.Fatalf("removed %d, registry %v", removed, genericRegistryTypes(reg))
	}
	if def, _ := reg.Lookup("a.3"); def.Version != 1 {
		t.Errorf("version = %d, want 1", def.Version)
	}
	if removed := reg.ReplaceWhere(nil, []*NodeDef{{Type: "c.1", Category: "c"}}); removed != 0 || len(reg.All()) != 3 {
		t.Fatalf("nil pred: removed %d, registry %v", removed, genericRegistryTypes(reg))
	}
	if removed := reg.ReplaceWhere(func(d *NodeDef) bool { return d.Type == "c.1" }, nil); removed != 1 || len(reg.All()) != 2 {
		t.Fatalf("no defs: removed %d, registry %v", removed, genericRegistryTypes(reg))
	}
	before := genericRegistryTypes(reg)
	for name, defs := range map[string][]*NodeDef{"nil": {nil}, "no type": {{}}, "valid then nil": {{Type: "d.1"}, nil}} {
		func() {
			defer func() {
				if r := recover(); r != "flows: ReplaceWhere needs definitions with a type" {
					t.Errorf("%s: recovered %v", name, r)
				}
			}()
			reg.ReplaceWhere(func(*NodeDef) bool { return true }, defs)
		}()
		if got := genericRegistryTypes(reg); !reflect.DeepEqual(got, before) {
			t.Errorf("%s: a rejected ReplaceWhere changed the registry to %v", name, got)
		}
	}
	// A crashing pred leaves the registry as it was and does not keep it locked.
	func() {
		defer func() { _ = recover() }()
		reg.ReplaceWhere(func(d *NodeDef) bool {
			if d.Type == "b.1" {
				panic("pred")
			}
			return true
		}, []*NodeDef{{Type: "e.1"}})
	}()
	if got := genericRegistryTypes(reg); !reflect.DeepEqual(got, before) {
		t.Errorf("a crashing pred changed the registry to %v", got)
	}
}

// A lookup during refreshes never misses a generic type that exists before and after.
func TestRefreshGenericToolsIsAtomic(t *testing.T) {
	reg := NewRegistry()
	tools := []GenericTool{{Name: "docker", Category: "infrastructure", Schema: sampleToolSchema()}, {Name: "proxmox"}}
	RefreshGenericTools(reg, tools, nil)
	var misses, lookups atomic.Int64
	done := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-done:
					return
				default:
				}
				lookups.Add(1)
				if _, ok := reg.Lookup("tool.docker"); !ok {
					misses.Add(1)
				}
			}
		}()
	}
	for i := 0; i < 500; i++ {
		list := tools
		if i%2 == 1 {
			list = append([]GenericTool{{Name: fmt.Sprintf("extra_%d", i)}}, tools...)
		}
		RefreshGenericTools(reg, list, nil)
	}
	close(done)
	wg.Wait()
	if misses.Load() != 0 {
		t.Fatalf("%d of %d lookups missed tool.docker during refreshes", misses.Load(), lookups.Load())
	}
}

// Arguments are real JSON: what cannot be encoded fails the node before any call, and
// so do an operation the schema does not list and oversized arguments.
func TestGenericExecuteChecksArguments(t *testing.T) {
	reg := NewRegistry()
	RefreshGenericTools(reg, []GenericTool{{Name: "docker", Category: "infrastructure", Schema: sampleToolSchema()}}, nil)
	def := lookupDef(t, reg, "tool.docker")
	cyclic := map[string]any{}
	cyclic["self"] = cyclic
	bad := []struct {
		name   string
		params map[string]any
		inMsg  string
	}{
		{"NaN in a JSON string param", map[string]any{"operation": "list", "name": "x", "sections": []any{math.NaN()}}, "sections"},
		{"cycle in a JSON param", map[string]any{"operation": "list", "name": "x", "config": cyclic}, "not valid JSON"},
		{"func", map[string]any{"operation": "list", "name": func() {}}, "not valid JSON"},
		{"NaN number", map[string]any{"operation": "list", "name": "x", "limit": math.NaN()}, "not valid JSON"},
		{"too large", map[string]any{"operation": "list", "name": strings.Repeat("x", maxGenericArgsBytes)}, "limit"},
		{"unlisted operation", map[string]any{"operation": "drop_all", "name": "x"}, `"drop_all"`},
		{"template text as operation", map[string]any{"operation": "{{x}}", "name": "x"}, "operations"},
		{"number as operation", map[string]any{"operation": 3.0, "name": "x"}, "operations"},
		{"huge operation", map[string]any{"operation": strings.Repeat("l", 200), "name": "x"}, "operations"},
	}
	for _, c := range bad {
		tools := &fakeTools{}
		_, err := execDef(def, c.params, &Services{Tools: tools})
		ne := asNodeError(err)
		if ne == nil || ne.Code != "FLOW_PARAM_INVALID" || !strings.Contains(ne.Message, c.inMsg) || len(ne.Message) > maxEchoMessageBytes ||
			!utf8.ValidString(ne.Message) || tools.count() != 0 {
			t.Errorf("%s: error %v, %d calls", c.name, err, tools.count())
		}
	}
	// The listed spelling is sent; an absent operation is the tool's business.
	for _, c := range []struct {
		op   any
		want map[string]any
	}{
		{" LIST ", map[string]any{"operation": "list", "name": "x"}},
		{nil, map[string]any{"name": "x"}},
	} {
		tools := &fakeTools{}
		if _, err := execDef(def, map[string]any{"operation": c.op, "name": "x"}, &Services{Tools: tools}); err != nil ||
			!reflect.DeepEqual(tools.last(t).Args, c.want) {
			t.Errorf("operation %q: args %v, %v", c.op, tools.last(t).Args, err)
		}
	}
	// Without a list of operations any text goes to the tool as it is.
	reg2 := genericTestRegistry(t)
	tools := &fakeTools{}
	if _, err := execDef(lookupDef(t, reg2, "tool.adguard"), map[string]any{"operation": "query_log_clear"}, &Services{Tools: tools}); err != nil ||
		tools.last(t).Args["operation"] != "query_log_clear" {
		t.Errorf("adguard: %v, %v", tools.last(t).Args, err)
	}
	// genericArgs, the plan's form without an error, gives nil for arguments it cannot build.
	params, jsonStrings := paramsFromSchema(sampleToolSchema())
	if args := genericArgs(params, jsonStrings, map[string]any{"sections": []any{math.Inf(1)}}); args != nil {
		t.Errorf("genericArgs = %v, want nil", args)
	}
}
