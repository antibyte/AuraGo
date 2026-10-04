package flows

import (
	"fmt"
	"reflect"
	"slices"
	"testing"
)

// genericArgs is genericToolArgs for a caller that needs no error, in the form the
// plan's test uses: nil when the arguments cannot be built.
func genericArgs(params []ParamSpec, jsonStrings map[string]bool, values map[string]any) map[string]any {
	args, err := genericToolArgs(params, jsonStrings, values)
	if err != nil {
		return nil
	}
	return args
}

// genericTextProps returns string properties with the given names.
func genericTextProps(names ...string) map[string]any {
	props := map[string]any{}
	for _, n := range names {
		props[n] = genericProp("string", "")
	}
	return props
}

func genericSchemaOf(props map[string]any) map[string]any {
	return map[string]any{"type": "object", "properties": props}
}

// Credentials never become parameters: a typed-in value would be stored in clear in
// the flow, its versions and the run records.
func TestGenericDropsCredentialParams(t *testing.T) {
	with := func(props map[string]any, extra map[string]any) map[string]any {
		for k, v := range extra {
			props[k] = v
		}
		return genericSchemaOf(props)
	}
	op := func(values ...string) map[string]any { return map[string]any{"operation": genericEnumProp(values...)} }
	tools := []GenericTool{
		{Name: "ldap", Category: "infrastructure", Schema: with(genericTextProps("password", "user_dn", "filter"), op("search", "authenticate"))},
		{Name: "manage_sql_connections", Category: "infrastructure", Schema: with(genericTextProps("password", "username", "host", "credential_action"), op("list", "create"))},
		{Name: "pdf_operations", Category: "files", Schema: with(genericTextProps("password", "file_path", "watermark_text"), op("encrypt", "decrypt"))},
		{Name: "register_device", Category: "infrastructure", Schema: with(genericTextProps("password", "hostname", "device_type", "username"),
			map[string]any{"port": genericProp("integer", "")})},
		// Not credentials: names of keys and tokens, limits, ids.
		{Name: "control_tool", Category: "infrastructure", Schema: with(genericTextProps("key", "token_id", "max_tokens", "env_key",
			"destination_key", "node_key", "credential_id", "keywords", "password_hint"), map[string]any{"private": genericProp("boolean", "")})},
	}
	reg := NewRegistry()
	if n := RefreshGenericTools(reg, tools, nil); n != len(tools) {
		t.Fatalf("registered %d", n)
	}
	for _, tool := range tools[:4] {
		def := lookupDef(t, reg, GenericTypePrefix+tool.Name)
		if names := genericParamNames(def); slices.Contains(names, "password") {
			t.Errorf("%s keeps password: %v", tool.Name, names)
		}
		fake := &fakeTools{}
		if _, err := execDef(def, map[string]any{"password": "hunter2", "hostname": "h"}, &Services{Tools: fake}); err != nil {
			t.Errorf("%s: %v", tool.Name, err)
		} else if _, sent := fake.last(t).Args["password"]; sent {
			t.Errorf("%s sent the password: %v", tool.Name, fake.last(t).Args)
		}
	}
	want := []string{"credential_id", "destination_key", "env_key", "key", "keywords", "max_tokens", "node_key", "password_hint", "private", "token_id"}
	if got := genericParamNames(lookupDef(t, reg, "tool.control_tool")); !reflect.DeepEqual(got, want) {
		t.Errorf("control params = %v, want %v", got, want)
	}
	for _, name := range []string{"password", "Password", "passwd", "secret", "token", "api_key", "apikey", "passphrase", "private_key",
		"client_secret", "access_token", "refresh_token", "db_password", "bot_token", "ssh_private_key", "aws_api_key", "webhook_secret"} {
		if !isGenericCredentialName(name) {
			t.Errorf("%s is a credential", name)
		}
	}
	for _, name := range []string{"key", "env_key", "destination_key", "node_key", "token_id", "max_tokens", "tokens", "credential_id",
		"keywords", "password_hint", "secrets_count", "tokenizer"} {
		if isGenericCredentialName(name) {
			t.Errorf("%s is no credential", name)
		}
	}
}

// What a file tool writes is a sink like file.write's content, also inside a JSON
// parameter; paths inside items are sinks for every tool.
func TestGenericFileContentSinks(t *testing.T) {
	patches := map[string]any{"type": "array", "items": map[string]any{"type": "object", "properties": map[string]any{
		"new_text": genericProp("string", ""), "old_text": genericProp("string", ""), "expected_occurrences": genericProp("integer", "")}}}
	todoItems := map[string]any{"type": "array", "items": map[string]any{"type": "object", "properties": map[string]any{
		"title": genericProp("string", ""), "is_done": genericProp("boolean", "")}}}
	withOp := func(props map[string]any) map[string]any {
		props["operation"] = genericEnumProp("read", "write")
		return genericSchemaOf(props)
	}
	cases := []struct {
		tool     GenericTool
		sinks    []string
		nonSinks []string
	}{
		{GenericTool{Name: "file_editor", Category: "files", Schema: withOp(genericTextProps("file_path", "content", "new", "old", "marker"))},
			[]string{"content", "file_path", "new", "operation"}, []string{"marker", "old"}},
		{GenericTool{Name: "homepage_file", Category: "infrastructure", Schema: withOp(map[string]any{"path": genericProp("string", ""),
			"content": genericProp("string", ""), "set_value": map[string]any{}, "json_path": genericProp("string", ""), "sub_operation": genericProp("string", "")})},
			[]string{"content", "json_path", "operation", "path", "set_value"}, []string{"sub_operation"}},
		{GenericTool{Name: "remote_control_files", Category: "infrastructure", Schema: withOp(map[string]any{"device_id": genericProp("string", ""),
			"path": genericProp("string", ""), "content": genericProp("string", ""), "patches": patches, "glob": genericProp("string", "")})},
			[]string{"content", "device_id", "operation", "patches", "path"}, []string{"glob"}},
		{GenericTool{Name: "office_document", Category: "infrastructure", Schema: withOp(genericTextProps("path", "append_text", "prepend_text",
			"text", "html", "document", "title", "format"))},
			[]string{"append_text", "document", "html", "operation", "path", "prepend_text", "text", "title"}, []string{"format"}},
		{GenericTool{Name: "tts", Category: "media", Schema: genericSchemaOf(genericTextProps("text", "language"))},
			[]string{"text"}, []string{"language"}},
		{GenericTool{Name: "manage_todos", Category: "system", Schema: withOp(map[string]any{"items": todoItems, "item_description": genericProp("string", "")})},
			[]string{"items", "operation"}, []string{"item_description"}},
		// Not a file tool: its content stays no sink.
		{GenericTool{Name: "send_image", Category: "media", Schema: genericSchemaOf(genericTextProps("path", "caption", "content"))},
			[]string{"path"}, []string{"caption", "content"}},
	}
	for _, c := range cases {
		reg := NewRegistry()
		RefreshGenericTools(reg, []GenericTool{c.tool}, nil)
		def := lookupDef(t, reg, GenericTypePrefix+c.tool.Name)
		if got := genericSinks(def); !reflect.DeepEqual(got, c.sinks) {
			t.Errorf("%s sinks = %v, want %v", c.tool.Name, got, c.sinks)
		}
		for _, p := range def.Params {
			if slices.Contains(c.nonSinks, p.Name) && p.SensitiveSink {
				t.Errorf("%s.%s must not be a sink", c.tool.Name, p.Name)
			}
		}
	}
}

// Host port mappings and what a browser or form automation types into a page are
// sinks; a "text" elsewhere (outside file and code-running tools) is not.
func TestGenericPortsAndPageInputSinks(t *testing.T) {
	ops := func(props map[string]any, values ...string) map[string]any {
		props["operation"] = genericEnumProp(values...)
		return genericSchemaOf(props)
	}
	tools := []GenericTool{
		{Name: "docker", Category: "infrastructure", Schema: ops(map[string]any{"image": genericProp("string", ""), "name": genericProp("string", ""),
			"restart": genericProp("string", ""), "ports": genericProp("string", "Port mappings: {'container_port': 'host_port'}. Provide as a JSON object string.")},
			"list_containers", "run")},
		{Name: "virtual_browser", Category: "infrastructure", Schema: ops(genericTextProps("workspace_id", "url", "text", "value", "key", "selector"),
			"open", "type", "select", "press")},
		{Name: "browser_automation", Category: "network", Schema: ops(genericTextProps("session_id", "url", "text", "value", "key", "selector"),
			"navigate", "type", "select")},
		{Name: "form_automation", Category: "network", Schema: ops(map[string]any{"url": genericProp("string", ""), "selector": genericProp("string", ""),
			"fields": genericProp("string", "JSON object mapping CSS selector to value for fill_submit")}, "get_fields", "fill_submit")},
		// Controls: a radio message and spoken text are no sinks.
		{Name: "meshcore", Category: "communication", Schema: ops(genericTextProps("node_key", "text"), "status", "send_direct")},
		{Name: "bluetooth", Category: "media", Schema: ops(genericTextProps("device", "text"), "status", "speak")},
	}
	reg := newTestRegistry(t)
	if n := RefreshGenericTools(reg, tools, nil); n != len(tools) {
		t.Fatalf("registered %d", n)
	}
	want := map[string][]string{
		"docker":             {"image", "operation", "ports"},
		"virtual_browser":    {"operation", "text", "url", "value", "workspace_id"},
		"browser_automation": {"operation", "text", "url", "value"},
		"form_automation":    {"fields", "operation", "url"},
		"meshcore":           {"node_key", "operation"},
		"bluetooth":          {"device", "operation"},
	}
	for tool, sinks := range want {
		if got := genericSinks(lookupDef(t, reg, GenericTypePrefix+tool)); !reflect.DeepEqual(got, sinks) {
			t.Errorf("%s sinks = %v, want %v", tool, got, sinks)
		}
	}
	cases := []struct {
		typ    string
		params map[string]any
		warnOn string
	}{
		{"tool.docker", map[string]any{"operation": "run", "image": "nginx", "ports": "{{trigger.data.ports}}"}, "ports"},
		{"tool.virtual_browser", map[string]any{"operation": "type", "workspace_id": "w", "text": "{{trigger.data.text}}"}, "text"},
		{"tool.form_automation", map[string]any{"operation": "fill_submit", "url": "https://x.example", "fields": "{{trigger.data.fields}}"}, "fields"},
		{"tool.meshcore", map[string]any{"operation": "send_direct", "node_key": "k", "text": "{{trigger.data.text}}"}, ""},
	}
	for _, c := range cases {
		b := newFlow(c.typ)
		trg := b.node("trg", "test.untrusted_trigger", nil)
		n := b.node("gen", c.typ, c.params)
		b.edge(trg, PortOut, n)
		var params []string
		for _, is := range LintUntrustedData(b.build(), reg) {
			params = append(params, is.Param)
		}
		if c.warnOn == "" && len(params) != 0 || c.warnOn != "" && !reflect.DeepEqual(params, []string{c.warnOn}) {
			t.Errorf("%s: lint warns on %v, want %q", c.typ, params, c.warnOn)
		}
	}
}

// genericOpsSchema adds an operation enum to props.
func genericOpsSchema(props map[string]any, ops ...string) map[string]any {
	props["operation"] = genericEnumProp(ops...)
	return genericSchemaOf(props)
}

// genericLintParams returns the params the lint warns on when an untrusted trigger
// feeds a node of typ with params.
func genericLintParams(reg *Registry, typ string, params map[string]any) []string {
	b := newFlow(typ)
	trg := b.node("trg", "test.untrusted_trigger", nil)
	n := b.node("gen", typ, params)
	b.edge(trg, PortOut, n)
	var warned []string
	for _, is := range LintUntrustedData(b.build(), reg) {
		warned = append(warned, is.Param)
	}
	return warned
}

// Parameters that expose a service to the network, and content that ends up in a
// repository, a model or a note, are sinks of their tools (shapes of the real schemas).
func TestGenericExposureAndContentSinks(t *testing.T) {
	acl := map[string]any{"type": "array", "items": map[string]any{"type": "object", "properties": map[string]any{
		"level": genericProp("string", ""), "principal": genericProp("string", "")}}}
	tools := []GenericTool{
		{Name: "fritzbox_network", Category: "smart_home", Schema: genericOpsSchema(map[string]any{
			"external_port": genericProp("string", ""), "internal_port": genericProp("string", ""), "internal_client": genericProp("string", ""),
			"hostname": genericProp("string", ""), "protocol": genericProp("string", ""), "description": genericProp("string", ""),
			"enabled": genericProp("boolean", "")}, "get_port_forwards", "add_port_forward", "delete_port_forward")},
		{Name: "cloudflare_tunnel", Category: "network", Schema: genericOpsSchema(map[string]any{"port": genericProp("integer", "")}, "status", "quick_tunnel")},
		{Name: "tailscale", Category: "infrastructure", Schema: genericOpsSchema(genericTextProps("query", "value"), "routes", "enable_routes")},
		{Name: "network_shares", Category: "infrastructure", Schema: genericOpsSchema(map[string]any{"acl": acl,
			"clients": map[string]any{"type": "array", "items": map[string]any{"type": "string"}}, "path": genericProp("string", ""),
			"name": genericProp("string", ""), "comment": genericProp("string", ""), "guest": genericProp("boolean", "")}, "list", "create")},
		{Name: "github", Category: "infrastructure", Schema: genericOpsSchema(genericTextProps("content", "value", "path", "title", "body", "owner"),
			"list_repos", "create_or_update_file", "list_pull_requests")},
		{Name: "openscad_render", Category: "infrastructure", Schema: genericSchemaOf(map[string]any{"source_scad": genericProp("string", ""),
			"model_name": genericProp("string", ""), "window_id": genericProp("string", ""),
			"exports": map[string]any{"type": "array", "items": map[string]any{"type": "string"}}})},
		{Name: "desktop_notes", Category: "infrastructure", Schema: genericOpsSchema(genericTextProps("content", "title", "folder", "path", "query", "tag"),
			"list", "create")},
	}
	reg := newTestRegistry(t)
	if n := RefreshGenericTools(reg, tools, nil); n != len(tools) {
		t.Fatalf("registered %d", n)
	}
	want := map[string][]string{
		"fritzbox_network":  {"external_port", "hostname", "internal_client", "internal_port", "operation"},
		"cloudflare_tunnel": {"operation", "port"},
		"tailscale":         {"operation", "query", "value"},
		"network_shares":    {"acl", "clients", "operation", "path"},
		"github":            {"content", "operation", "path", "title"}, // not value: a SHA or state filter
		"openscad_render":   {"source_scad"},
		"desktop_notes":     {"content", "folder", "operation", "path", "query", "title"},
	}
	for tool, sinks := range want {
		if got := genericSinks(lookupDef(t, reg, GenericTypePrefix+tool)); !reflect.DeepEqual(got, sinks) {
			t.Errorf("%s sinks = %v, want %v", tool, got, sinks)
		}
	}
	cases := []struct {
		typ    string
		params map[string]any
		warnOn string
	}{
		{"tool.fritzbox_network", map[string]any{"operation": "add_port_forward", "external_port": "{{trigger.data.port}}",
			"internal_port": "22", "internal_client": "192.168.1.5", "protocol": "TCP"}, "external_port"},
		{"tool.fritzbox_network", map[string]any{"operation": "add_port_forward", "external_port": "2222", "internal_port": "22",
			"internal_client": "{{trigger.data.ip}}"}, "internal_client"},
		{"tool.cloudflare_tunnel", map[string]any{"operation": "quick_tunnel", "port": "{{trigger.data.port}}"}, "port"},
		{"tool.tailscale", map[string]any{"operation": "enable_routes", "query": "node1", "value": "{{trigger.data.routes}}"}, "value"},
		{"tool.github", map[string]any{"operation": "create_or_update_file", "path": ".github/workflows/ci.yml",
			"content": "{{trigger.data.b64}}"}, "content"},
	}
	for _, c := range cases {
		if got := genericLintParams(reg, c.typ, c.params); !reflect.DeepEqual(got, []string{c.warnOn}) {
			t.Errorf("%s: lint warns on %v, want %q", c.typ, got, c.warnOn)
		}
	}
	// Reads of github are no system change, "pull" or not.
	github := lookupDef(t, reg, "tool.github")
	if got := github.EffectsOf(&Node{Params: map[string]any{"operation": "list_pull_requests"}}); got != nil {
		t.Errorf("list_pull_requests effects = %v", got)
	}
	if got := github.EffectsOf(&Node{Params: map[string]any{"operation": "create_or_update_file"}}); !reflect.DeepEqual(got, []Effect{EffectSystemChange}) {
		t.Errorf("create_or_update_file effects = %v", got)
	}
}

// Operations that take a secret as a plain value are gone, with their parameters;
// Validate and Execute refuse them like any operation the list does not hold.
func TestGenericDroppedSecretOperations(t *testing.T) {
	envProps := func(extra string) map[string]any { return genericTextProps("env_key", "env_value", extra) }
	tools := []GenericTool{
		{Name: "invasion_tasks", Category: "infrastructure", Schema: genericOpsSchema(genericTextProps("key", "value", "nest_id", "task", "body"),
			"send_task", "task_status", "send_secret")},
		{Name: "netlify", Category: "infrastructure", Schema: genericOpsSchema(envProps("site_id"), "list_env", "get_env", "set_env", "delete_env")},
		{Name: "vercel", Category: "infrastructure", Schema: genericOpsSchema(envProps("project_id"), "list_env", "get_env", "set_env", "delete_env")},
	}
	reg := NewRegistry()
	if n := RefreshGenericTools(reg, tools, nil); n != 3 {
		t.Fatalf("registered %d", n)
	}
	vc := ValidateContext{Mode: ModePublish}
	for _, c := range []struct {
		tool, dropped string
		params        []string
	}{
		{"invasion_tasks", "send_secret", []string{"body", "nest_id", "operation", "task"}},
		{"netlify", "set_env", []string{"env_key", "operation", "site_id"}},
		{"vercel", "set_env", []string{"env_key", "operation", "project_id"}},
	} {
		def := lookupDef(t, reg, GenericTypePrefix+c.tool)
		if got := genericParamNames(def); !reflect.DeepEqual(got, c.params) {
			t.Errorf("%s params = %v, want %v", c.tool, got, c.params)
		}
		_, choices := genericOperation(def.Params)
		if slices.Contains(choices, c.dropped) || len(choices) == 0 {
			t.Errorf("%s operations = %v", c.tool, choices)
		}
		node := &Node{ID: testNodeID(1), Key: "gen", Params: map[string]any{"operation": c.dropped}}
		if issues := def.Validate(node, vc); len(issues) != 1 || issues[0].Code != IssueParamInvalid {
			t.Errorf("%s: Validate(%s) = %+v", c.tool, c.dropped, issues)
		}
		fake := &fakeTools{}
		_, err := execDef(def, map[string]any{"operation": c.dropped, "key": "API", "value": "s3cr3t", "env_key": "TOKEN",
			"env_value": "s3cr3t"}, &Services{Tools: fake})
		if ne := asNodeError(err); ne == nil || ne.Code != "FLOW_PARAM_INVALID" || fake.count() != 0 {
			t.Errorf("%s: Execute(%s) = %v with %d calls", c.tool, c.dropped, err, fake.count())
		}
	}
	// The other operations still work, without the dropped parameters.
	fake := &fakeTools{}
	if _, err := execDef(lookupDef(t, reg, "tool.invasion_tasks"), map[string]any{"operation": "send_task", "task": "x", "nest_id": "n",
		"key": "API", "value": "s3cr3t"}, &Services{Tools: fake}); err != nil ||
		!reflect.DeepEqual(fake.last(t).Args, map[string]any{"operation": "send_task", "task": "x", "nest_id": "n"}) {
		t.Errorf("send_task: %v, %v", fake.last(t).Args, err)
	}
	fake = &fakeTools{}
	if _, err := execDef(lookupDef(t, reg, "tool.netlify"), map[string]any{"operation": "get_env", "env_key": "A"}, &Services{Tools: fake}); err != nil ||
		!reflect.DeepEqual(fake.last(t).Args, map[string]any{"operation": "get_env", "env_key": "A"}) {
		t.Errorf("get_env: %v, %v", fake.last(t).Args, err)
	}
	// A tool left without an operation, or whose operations are no list to filter,
	// gets no node.
	for name, schema := range map[string]map[string]any{
		"only the dropped operation": genericOpsSchema(genericTextProps("key", "value"), "send_secret"),
		"free-text operation":        genericSchemaOf(genericTextProps("operation", "key", "value")),
	} {
		reg := NewRegistry()
		if n := RefreshGenericTools(reg, []GenericTool{{Name: "invasion_tasks", Schema: schema}}, nil); n != 0 {
			t.Errorf("%s: registered %d", name, n)
		}
	}
}

// A sink name one level down is found whatever the number of names and the map order.
func TestGenericNestedSinkLooksAtEveryName(t *testing.T) {
	nested := map[string]any{}
	for i := 0; i < 3*maxGenericParams; i++ {
		nested[fmt.Sprintf("field_%03d", i)] = genericProp("string", "")
	}
	nested["zz_url"] = genericProp("string", "")
	schema := genericSchemaOf(map[string]any{"rules": map[string]any{"type": "array", "items": map[string]any{"type": "object", "properties": nested}}})
	for i := 0; i < 20; i++ {
		params, _ := paramsFromSchema(schema)
		if len(params) != 1 || !params[0].SensitiveSink {
			t.Fatalf("run %d: %+v", i, params)
		}
	}
}

// A code-running tool cannot start a detached process that outlives the node.
func TestGenericCodeToolsDropBackground(t *testing.T) {
	reg := genericTestRegistry(t)
	def := lookupDef(t, reg, "tool.execute_shell")
	if got := genericParamNames(def); !reflect.DeepEqual(got, []string{"command"}) {
		t.Fatalf("execute_shell params = %v", got)
	}
	fake := &fakeTools{}
	if _, err := execDef(def, map[string]any{"command": "sleep 1000", "background": true}, &Services{Tools: fake}); err != nil ||
		!reflect.DeepEqual(fake.last(t).Args, map[string]any{"command": "sleep 1000"}) {
		t.Errorf("args = %v, %v", fake.last(t).Args, err)
	}
	// Other tools keep their background flag.
	reg2 := NewRegistry()
	RefreshGenericTools(reg2, []GenericTool{{Name: "video_download", Category: "media", Schema: genericSchemaOf(map[string]any{
		"url": genericProp("string", ""), "background": genericProp("boolean", "")})}}, nil)
	if got := genericParamNames(lookupDef(t, reg2, "tool.video_download")); !reflect.DeepEqual(got, []string{"background", "url"}) {
		t.Errorf("video_download params = %v", got)
	}
}

// Descriptions that say "JSON string" or "raw JSON" make a JSON-string parameter.
func TestGenericJSONStringPhrases(t *testing.T) {
	params, jsonStrings := paramsFromSchema(genericSchemaOf(map[string]any{
		"body":    genericProp("string", `Extra variables as JSON string (e.g. '{"env":"prod"}')`),
		"content": genericProp("string", "Raw JSON config for DHCP, client, or DNS settings operations"),
		"name":    genericProp("string", "Playbook file name"),
	}))
	kinds := map[string]string{}
	for _, p := range params {
		kinds[p.Name] = p.Kind
	}
	if !reflect.DeepEqual(kinds, map[string]string{"body": ParamJSON, "content": ParamJSON, "name": ParamText}) ||
		!reflect.DeepEqual(jsonStrings, map[string]bool{"body": true, "content": true}) {
		t.Fatalf("kinds %v, json strings %v", kinds, jsonStrings)
	}
	args, err := genericToolArgs(params, jsonStrings, map[string]any{"body": map[string]any{"env": "prod"}, "content": `{"a":1}`})
	if err != nil || !reflect.DeepEqual(args, map[string]any{"body": `{"env":"prod"}`, "content": `{"a":1}`}) {
		t.Errorf("args = %#v, %v", args, err)
	}
}

// The operation is never the node's primary input, not even as free text.
func TestGenericPrimaryInputSkipsOperation(t *testing.T) {
	reg := genericTestRegistry(t)
	for tool, want := range map[string]string{"adguard": "query", "docker": "command", "execute_shell": "command", "truenas": "name"} {
		if got := lookupDef(t, reg, GenericTypePrefix+tool).PrimaryInput; got != want {
			t.Errorf("%s primary input = %q, want %q", tool, got, want)
		}
	}
	reg2 := NewRegistry()
	RefreshGenericTools(reg2, []GenericTool{{Name: "only_op", Schema: genericSchemaOf(genericTextProps("operation"))}}, nil)
	if got := lookupDef(t, reg2, "tool.only_op").PrimaryInput; got != "" {
		t.Errorf("only_op primary input = %q", got)
	}
}
