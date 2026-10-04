package flows

import (
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
