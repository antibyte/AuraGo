package flows

import (
	"encoding/json"
	"sort"
	"strings"
	"unicode/utf8"
)

// Parameters of generic tool nodes: which schema properties become parameters, which
// of them are sensitive sinks, and how resolved values become tool arguments.

// genericDroppedParams never become parameters of a generic node: the agent's session
// task list; secret and credential injection (a flow reads secrets only through a
// secret_ref parameter whose value is scrubbed from the run); the Python tool bridge
// (code that calls other tools, while callTool allows a node exactly one); and the
// appointment wake-up, which hands an instruction to the main agent.
var genericDroppedParams = map[string]bool{"_todo": true, "vault_keys": true, "credential_ids": true,
	"enable_tool_bridge": true, "tool_bridge_call_limit": true, "inject_token": true,
	"agent_instruction": true, "wake_agent": true}

// Credential parameters (ldap, manage_sql_connections, pdf_operations and
// register_device take a "password") are dropped too: a value typed into a node is
// stored in clear in the flow document, its versions and the run's step params. Exact
// names and suffixes only; "_key" is no suffix here, because key, env_key,
// destination_key and node_key name things rather than hold secrets.
var (
	genericCredentialNames = map[string]bool{"password": true, "passwd": true, "secret": true, "token": true,
		"api_key": true, "apikey": true, "passphrase": true, "private_key": true, "client_secret": true,
		"access_token": true, "refresh_token": true}
	genericCredentialSuffixes = []string{"_password", "_passwd", "_secret", "_token", "_passphrase", "_api_key",
		"_apikey", "_private_key"}
)

func isGenericCredentialName(name string) bool {
	name = strings.ToLower(name)
	if genericCredentialNames[name] {
		return true
	}
	for _, s := range genericCredentialSuffixes {
		if strings.HasSuffix(name, s) {
			return true
		}
	}
	return false
}

// genericSinkNames are parameter names whose value decides what a tool runs, which
// file it reads or writes, where data goes or which device it drives. They are sinks
// (SensitiveSink) for every generic tool, read-only ones included: a path or address
// that untrusted data picks leaks data even through a read. A JSON parameter whose
// items or properties carry such a name is a sink as well (filesystem "items" holds
// the paths of delete_batch).
var genericSinkNames = map[string]bool{
	// what runs
	"command": true, "cmd": true, "code": true, "script": true, "sql": true, "sql_query": true, "query": true,
	"operation": true, "action": true, "command_args": true, "arguments": true, "image": true, "env": true, "volumes": true,
	"package": true, "packages": true, "libraries": true, "dependencies": true,
	// which file
	"path": true, "paths": true, "file": true, "file_path": true, "filepath": true, "filename": true,
	"directory": true, "folder": true, "destination": true,
	// where data goes and who receives it
	"url": true, "uri": true, "urls": true, "endpoint": true, "host": true, "hostname": true, "ip": true,
	"ip_address": true, "domain": true, "target": true, "to": true, "cc": true, "bcc": true, "recipient": true,
	"recipients": true, "phone": true, "channel": true, "channel_id": true, "node_key": true, "webhook_name": true,
	// which device or machine, and what it is sent
	"entity_id": true, "device": true, "device_id": true, "device_name": true, "device_addr": true, "mac": true,
	"mac_address": true, "server_id": true, "node_id": true, "container_id": true, "machine_id": true,
	"workspace_id": true, "vmid": true, "topic": true, "payload": true, "service_data": true, "headers": true,
	// what a container exposes on the host (docker "ports" maps container to host ports;
	// no other tool has a parameter of this name, "port" is no sink)
	"ports": true,
	// what the main agent reads in its prompt (planner titles)
	"title": true,
}

var genericSinkSuffixes = []string{"_path", "_paths", "_dir", "_file", "_files", "_url", "_urls"}

func isGenericSinkName(name string) bool {
	name = strings.ToLower(name)
	if genericSinkNames[name] {
		return true
	}
	for _, s := range genericSinkSuffixes {
		if strings.HasSuffix(name, s) {
			return true
		}
	}
	return false
}

// genericFileContentNames carry what a file tool writes into a file. For tools that
// work on files (genericFileTool) they are sinks, like file.write's content: untrusted
// text must not become a script, a ~/.bashrc or a spreadsheet formula.
var genericFileContentNames = map[string]bool{"content": true, "content_base64": true, "new": true, "new_text": true,
	"set_value": true, "append_text": true, "prepend_text": true, "text": true, "html": true, "document": true,
	"workbook": true, "value": true, "values": true, "formula": true, "replacements": true, "patches": true}

func isGenericFileContentName(name string) bool {
	return genericFileContentNames[strings.ToLower(name)]
}

// genericPageInputParams type into a live web page: a browser or form automation tool
// fills fields with them, and a submitted form sends the data away. They are sinks for
// these tools only; "text" and "value" are no sinks by name elsewhere (outside the file
// tools). remote_control_desktop types too, but as a code-running tool every text
// parameter of it is a sink already.
var genericPageInputParams = map[string]map[string]bool{
	"virtual_browser":    {"text": true, "value": true},
	"browser_automation": {"text": true, "value": true},
	"form_automation":    {"fields": true},
}

// genericFileTool reports whether a tool works on or writes files: the files category,
// tools whose name says so or that fileTools lists (genericFileish), and fileWritingTools.
func genericFileTool(tool GenericTool) bool {
	return tool.Category == "files" || genericFileish(tool.Name) || fileWritingTools[tool.Name]
}

// genericPromptCategory is the agent category whose tools store what they get where
// the main agent's prompts read it (memories, notes, journal, knowledge graph): every
// text parameter of them is a sink, like a planner title.
const genericPromptCategory = "memory"

// genericProgramTools carry a program or a task in parameters whose names do not say
// so (an instruction, a module, extra variables), like every codeRunningTools entry: a
// virtual computer that runs shell and desktop tasks, and ansible.
var genericProgramTools = map[string]bool{"virtual_computers": true, "ansible": true}

// genericAllTextSinks reports whether every parameter of tool that is no bool or
// number is a sink: memory tools (the agent's prompts read what they store) and tools
// whose text runs or steers something (codeRunningTools, genericProgramTools).
func genericAllTextSinks(tool GenericTool) bool {
	return tool.Category == genericPromptCategory || codeRunningTools[tool.Name] || genericProgramTools[tool.Name]
}

// genericToolParams is paramsFromSchema plus the rules of the tool: a code-running
// tool loses "background" (a detached process outlives the node's timeout and the
// run's cancellation); every text parameter of a genericAllTextSinks tool is a sink,
// and so is every file content parameter of a file tool, also one level down a JSON
// parameter (remote_control_files "patches" holds new_text), and every page input of
// a browser or form automation tool (genericPageInputParams).
func genericToolParams(tool GenericTool) ([]ParamSpec, map[string]bool) {
	params, jsonStrings := paramsFromSchema(tool.Schema)
	if codeRunningTools[tool.Name] {
		kept := params[:0]
		for _, p := range params {
			if p.Name != "background" {
				kept = append(kept, p)
			}
		}
		params = kept
	}
	allText, fileTool := genericAllTextSinks(tool), genericFileTool(tool)
	props, _ := tool.Schema["properties"].(map[string]any)
	for i := range params {
		p := &params[i]
		if (allText && p.Kind != ParamBool && p.Kind != ParamNumber) || genericPageInputParams[tool.Name][p.Name] {
			p.SensitiveSink = true
		}
		if fileTool && !p.SensitiveSink {
			prop, _ := props[p.Name].(map[string]any)
			p.SensitiveSink = isGenericFileContentName(p.Name) || anyName(schemaNestedNames(prop), isGenericFileContentName)
		}
	}
	return params, jsonStrings
}

// schemaNestedNames returns the property names one level down a schema property: of
// an object's "properties" and of an array's "items" "properties", at most
// maxGenericParams of each.
func schemaNestedNames(p map[string]any) []string {
	var names []string
	add := func(props map[string]any) {
		n := 0
		for name := range props {
			if n++; n > maxGenericParams {
				return
			}
			names = append(names, name)
		}
	}
	if props, ok := p["properties"].(map[string]any); ok {
		add(props)
	}
	if items, ok := p["items"].(map[string]any); ok {
		if props, ok := items["properties"].(map[string]any); ok {
			add(props)
		}
	}
	return names
}

func anyName(names []string, pred func(string) bool) bool {
	for _, n := range names {
		if pred(n) {
			return true
		}
	}
	return false
}

// genericNameOK reports whether s can name a tool (dots and colons allowed, as in
// provider function names) or, with param, a parameter: a parameter name is part of
// document paths and template references, where "." and "[" would split it.
func genericNameOK(s string, limit int, param bool) bool {
	if s == "" || len(s) > limit {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c == '_':
		case i > 0 && (c >= '0' && c <= '9' || c == '-'):
		case i > 0 && !param && (c == '.' || c == ':'):
		default:
			return false
		}
	}
	return true
}

var textareaParamNames = map[string]bool{"content": true, "body": true, "message": true, "prompt": true,
	"text": true, "html": true, "markdown": true, "code": true, "script": true}

// jsonStringPhrases in a description say that a text parameter holds JSON
// ("Provide as a JSON object string.", "Extra variables as JSON string", "Raw JSON config").
var jsonStringPhrases = []string{"json array", "json object", "json string", "raw json"}

// paramsFromSchema derives form parameters from a JSON schema: operation first, then
// required, then optional parameters (alphabetical). It also returns the parameters
// whose schema type is a string holding JSON.
//
// Any schema is accepted. A property that is not an object becomes a text parameter;
// a type list such as ["string","null"] counts as its first non-null type; nested
// objects become JSON parameters (only the names one level down are looked at, for
// sinks); an enum ([]any or []string) longer than maxGenericOptions or without a
// usable entry is no enum. Invalid names (genericNameOK), genericDroppedParams and
// credentials (isGenericCredentialName) are left out, the list is cut to
// maxGenericParams and help texts to maxGenericHelpRunes. A parameter is a
// SensitiveSink when its name, or a name one level down, says so (isGenericSinkName).
func paramsFromSchema(schema map[string]any) ([]ParamSpec, map[string]bool) {
	props, _ := schema["properties"].(map[string]any)
	required := map[string]bool{}
	switch r := schema["required"].(type) {
	case []any:
		for _, v := range r {
			if s, ok := v.(string); ok {
				required[s] = true
			}
		}
	case []string:
		for _, v := range r {
			required[v] = true
		}
	}
	rank := func(name string) int {
		switch {
		case name == "operation":
			return 0
		case required[name]:
			return 1
		}
		return 2
	}
	names := make([]string, 0, len(props))
	for name := range props {
		if !genericDroppedParams[name] && !isGenericCredentialName(name) && genericNameOK(name, maxGenericParamName, true) {
			names = append(names, name)
		}
	}
	sort.Slice(names, func(i, j int) bool {
		if ri, rj := rank(names[i]), rank(names[j]); ri != rj {
			return ri < rj
		}
		return names[i] < names[j]
	})
	if len(names) > maxGenericParams {
		names = names[:maxGenericParams]
	}
	jsonStrings := map[string]bool{}
	params := make([]ParamSpec, 0, len(names))
	for _, name := range names {
		p, _ := props[name].(map[string]any)
		desc, _ := p["description"].(string)
		desc = strings.TrimSpace(validUTF8(desc))
		spec := ParamSpec{Name: name, Label: humanizeToolName(name), Help: truncateRunes(desc, maxGenericHelpRunes),
			Required: required[name], Templatable: true,
			SensitiveSink: isGenericSinkName(name) || anyName(schemaNestedNames(p), isGenericSinkName)}
		typ := schemaType(p["type"])
		enum := schemaEnum(p["enum"])
		lowerDesc := strings.ToLower(desc)
		switch {
		case len(enum) > 0:
			spec.Kind = ParamSelect
			if name == "operation" && len(enum) <= 5 {
				spec.Kind = ParamSegmented
			}
			for _, s := range enum {
				spec.Options = append(spec.Options, Option{Value: s, Label: s})
			}
		case typ == "boolean":
			spec.Kind = ParamBool
		case typ == "integer" || typ == "number":
			spec.Kind = ParamNumber
		case typ == "object":
			spec.Kind = ParamJSON
			if ap, ok := p["additionalProperties"].(map[string]any); ok && schemaType(ap["type"]) == "string" {
				spec.Kind = ParamKeyValue
			}
		case typ == "array":
			spec.Kind = ParamJSON
			if items, ok := p["items"].(map[string]any); ok && schemaType(items["type"]) == "string" {
				spec.Kind = ParamTags
			}
		case containsAny(lowerDesc, jsonStringPhrases...):
			spec.Kind = ParamJSON
			jsonStrings[name] = true
		case textareaParamNames[name]:
			spec.Kind = ParamTextarea
		default:
			spec.Kind = ParamText
		}
		params = append(params, spec)
	}
	return params, jsonStrings
}

// schemaType returns a schema "type": the text itself, or the first non-null entry
// of a type list; "" for anything else.
func schemaType(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case []any:
		for _, x := range t {
			if s, ok := x.(string); ok && s != "null" {
				return s
			}
		}
	case []string:
		for _, s := range t {
			if s != "null" {
				return s
			}
		}
	}
	return ""
}

// schemaEnum returns the usable entries of a schema "enum": text, numbers and
// booleans in their natural form, non-empty, valid UTF-8, at most
// maxGenericOptionBytes and not repeated. More than maxGenericOptions entries is no enum.
func schemaEnum(v any) []string {
	var entries []any
	switch e := v.(type) {
	case []any:
		entries = e
	case []string:
		if len(e) > maxGenericOptions {
			return nil
		}
		entries = make([]any, len(e))
		for i, s := range e {
			entries[i] = s
		}
	}
	if len(entries) > maxGenericOptions {
		return nil
	}
	var values []string
	seen := map[string]bool{}
	for _, entry := range entries {
		var s string
		switch x := entry.(type) {
		case string:
			s = x
		case bool, float64, int, int64, json.Number:
			s = Stringify(x)
		default:
			continue
		}
		if s == "" || len(s) > maxGenericOptionBytes || !utf8.ValidString(s) || seen[s] {
			continue
		}
		seen[s] = true
		values = append(values, s)
	}
	return values
}

// genericToolArgs converts resolved parameter values into tool arguments; empty
// values are omitted. A JSON parameter that the tool takes as a string is encoded with
// marshalCompact; arguments that cannot be encoded as JSON (NaN, a cycle, a func) or
// whose encoding exceeds maxGenericArgsBytes fail with FLOW_PARAM_INVALID. The result
// is a new map; the values in it may alias run data and are read-only.
func genericToolArgs(params []ParamSpec, jsonStrings map[string]bool, values map[string]any) (map[string]any, error) {
	args := map[string]any{}
	for _, spec := range params {
		v, ok := values[spec.Name]
		if !ok || isEmptyValue(v) {
			continue
		}
		switch spec.Kind {
		case ParamNumber:
			if f, ok := toNumber(v); ok {
				v = f
			}
		case ParamBool:
			if _, isBool := v.(bool); !isBool {
				v = truthy(v)
			}
		case ParamJSON:
			if jsonStrings[spec.Name] {
				if _, isString := v.(string); !isString {
					s, err := marshalCompact(v)
					if err != nil {
						return nil, NewNodeError("FLOW_PARAM_INVALID", "%s holds a value that is not valid JSON", spec.Name)
					}
					v = s
				}
			} else if s, isString := v.(string); isString {
				var parsed any
				if json.Unmarshal([]byte(s), &parsed) == nil {
					v = parsed
				}
			}
		case ParamTags:
			if s, isString := v.(string); isString {
				list := []any{}
				for _, part := range strings.Split(s, ",") {
					if part = strings.TrimSpace(part); part != "" {
						list = append(list, part)
					}
				}
				v = list
			}
		}
		args[spec.Name] = v
	}
	encoded, err := marshalCompact(args)
	if err != nil {
		return nil, NewNodeError("FLOW_PARAM_INVALID", "the tool arguments hold a value that is not valid JSON")
	}
	if len(encoded) > maxGenericArgsBytes {
		return nil, NewNodeError("FLOW_PARAM_INVALID", "the tool arguments are %d bytes; the limit is %d", len(encoded), maxGenericArgsBytes)
	}
	return args, nil
}

func humanizeToolName(name string) string {
	s := strings.ReplaceAll(name, "_", " ")
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
