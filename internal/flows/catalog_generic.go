package flows

import (
	"context"
	"encoding/json"
	"sort"
	"strings"
	"unicode/utf8"
)

// GenericTool describes an AuraGo tool for an automatic "more tools" node.
// Schema is the tool's JSON schema for its parameters; Category is the agent tool category.
type GenericTool struct {
	Name        string
	Description string
	Category    string
	Schema      map[string]any
}

// GenericTypePrefix prefixes the node types of generic tool nodes.
const GenericTypePrefix = "tool."

// Bounds on what a tool schema puts into a node definition. The schemas come from
// AuraGo's own catalog (real tools have up to about 30 parameters and 30
// operations), but a definition goes to the editor and through every hook.
const (
	maxGenericParams      = 100
	maxGenericOptions     = 200
	maxGenericOptionBytes = 200
	maxGenericHelpRunes   = 500
	maxGenericDescRunes   = 2000
	maxGenericToolName    = 128
	maxGenericParamName   = 64
	maxGenericCategory    = 40
	// maxGenericArgsBytes bounds the encoded arguments of one call.
	maxGenericArgsBytes = 4 << 20
)

var genericExcluded = map[string]bool{
	// meta and control-flow tools
	"discover_tools": true, "invoke_tool": true, "read_tool_output": true, "retrieve_original_output": true,
	"wait_for_event": true, "question_user": true, "follow_up": true, "co_agent": true,
	"context_manager": true, "context_memory": true,
	// bound to an agent session
	"manage_plan": true, "cron_scheduler": true, "manage_missions": true, "manage_daemon": true,
	// skill and tool management
	"create_skill_from_template": true, "save_tool": true, "run_tool": true, "execute_skill": true,
	"activate_agent_skill": true, "run_agent_skill_script": true, "list_agent_skills": true, "list_skills": true,
	"list_skill_templates": true, "get_skill_documentation": true, "set_skill_documentation": true,
	// secret access
	"request_vault_secret": true, "secrets_vault": true,
	// gateways to external tool catalogs
	"mcp_call": true, "composio_call": true,
	// fully covered by curated nodes
	"ddg_search": true, "brave_search": true, "web_scraper": true, "api_request": true, "send_telegram": true,
	"send_email": true, "send_notification": true, "send_discord": true, "mqtt_publish": true,
	// curated nodes whose rules a generic node would bypass
	"home_assistant":   true, // home.assistant: the service is a literal; a template could pick shell_command.*
	"document_creator": true, // doc.pdf_create: no URL fetch; url_to_pdf renders any URL in Gotenberg without an SSRF guard
	PDFExtractorTool:   true, // doc.pdf_read: bounded path, success check; summary mode spends tokens outside the budget
}

// genericCuratedAllowed lists the tools a curated node calls that stay generic nodes
// as well, each with the reason no rule of the curated node is lost. Every other tool
// of a curated node is excluded: by genericExcluded, and at refresh time by the Tool of
// any registered curated definition. A test fails for a curated tool in neither list.
var genericCuratedAllowed = map[string]string{
	"filesystem": "file.read and file.write add path bounds, the if_exists probe and a success check to their own " +
		"options; delete, move, copy and list exist only here, and file_path and destination are sinks by name",
	"manage_appointments": "list, update and delete exist only here; the title (it reaches the agent's prompt) is a sink by " +
		"name, and agent_instruction and wake_agent are dropped, as planner.appointment_add offers neither",
	"manage_todos": "list, update and complete exist only here; the title (it reaches the agent's prompt) is a sink by name",
}

// genericDroppedParams never become parameters of a generic node: the agent's session
// task list; secret and credential injection (a flow reads secrets only through a
// secret_ref parameter whose value is scrubbed from the run); the Python tool bridge
// (code that calls other tools, while callTool allows a node exactly one); and the
// appointment wake-up, which hands an instruction to the main agent.
var genericDroppedParams = map[string]bool{"_todo": true, "vault_keys": true, "credential_ids": true,
	"enable_tool_bridge": true, "tool_bridge_call_limit": true, "inject_token": true,
	"agent_instruction": true, "wake_agent": true}

// genericSinkNames are parameter names whose value decides what a tool runs, which
// file it reads or writes, where data goes or which device it drives. They are sinks
// (SensitiveSink) for every generic tool, read-only ones included: a path or address
// that untrusted data picks leaks data even through a read.
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
	// what the main agent reads in its prompt (planner titles)
	"title": true,
}

var genericSinkSuffixes = []string{"_path", "_paths", "_dir", "_file", "_files", "_url", "_urls"}

// genericPromptCategory is the agent category whose tools store what they get where
// the main agent's prompts read it (memories, notes, journal, knowledge graph): every
// text parameter of them is a sink, like a planner title.
const genericPromptCategory = "memory"

// genericProgramTools carry a program, a task for an agent or keystrokes in parameters
// whose names do not say so (an instruction, a task, an input, app files), like every
// codeRunningTools entry: a virtual computer that runs shell and desktop tasks, ansible
// (module, playbook, extra variables) and the Manus agent, which acts with the user's
// connected accounts.
var genericProgramTools = map[string]bool{"virtual_computers": true, "ansible": true, "manus": true}

// genericAllTextSinks reports whether every parameter of tool that is no bool or
// number is a sink: memory tools (the agent's prompts read what they store) and tools
// whose text runs or steers something (codeRunningTools, genericProgramTools).
func genericAllTextSinks(tool GenericTool) bool {
	return tool.Category == genericPromptCategory || codeRunningTools[tool.Name] || genericProgramTools[tool.Name]
}

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

// IsGenericToolExcluded reports whether a tool never becomes a generic node.
func IsGenericToolExcluded(name string) bool {
	return genericExcluded[name] || strings.HasPrefix(name, "skill__") || strings.HasPrefix(name, "tool__") ||
		strings.HasPrefix(name, "game_maker_")
}

func isGenericDef(d *NodeDef) bool { return strings.HasPrefix(d.Type, GenericTypePrefix) }

// RefreshGenericTools replaces all generic tool nodes with nodes for tools in one
// atomic step (Registry.ReplaceWhere) and returns how many were registered. Skipped:
// excluded tools (IsGenericToolExcluded), tools a registered curated definition calls
// (its Tool) unless genericCuratedAllowed lists them, invalid names and repeated names
// (the first one wins).
func RefreshGenericTools(reg *Registry, tools []GenericTool, env CatalogEnv) int {
	if reg == nil {
		return 0
	}
	curated := map[string]bool{}
	for _, d := range reg.All() {
		if d.Tool != "" && !isGenericDef(d) {
			curated[d.Tool] = true
		}
	}
	defs := make([]*NodeDef, 0, len(tools))
	seen := map[string]bool{}
	for _, tool := range tools {
		if !genericNameOK(tool.Name, maxGenericToolName, false) || IsGenericToolExcluded(tool.Name) || seen[tool.Name] ||
			(curated[tool.Name] && genericCuratedAllowed[tool.Name] == "") {
			continue
		}
		seen[tool.Name] = true
		defs = append(defs, genericToolDef(tool, env))
	}
	reg.ReplaceWhere(isGenericDef, defs)
	return len(defs)
}

// genericToolDef builds the node of one tool. The output is untrusted. Sinks are the
// parameters isGenericSinkName names, and every text parameter of a tool for which
// genericAllTextSinks holds. The operation
// parameter ("operation", else "action") decides the effects: a literal one is
// classified (genericEffects); anything else gets the worst case, the union over the
// operations the schema lists, which Execute enforces, or every effect when there is
// no list (genericWorstEffects).
func genericToolDef(tool GenericTool, env CatalogEnv) *NodeDef {
	params, jsonStrings := paramsFromSchema(tool.Schema)
	if genericAllTextSinks(tool) {
		for i := range params {
			if params[i].Kind != ParamBool && params[i].Kind != ParamNumber {
				params[i].SensitiveSink = true
			}
		}
	}
	category := tool.Category
	if !genericNameOK(category, maxGenericCategory, true) {
		category = "other"
	}
	def := &NodeDef{
		Type: GenericTypePrefix + tool.Name, Version: 1, Category: "tool:" + category, Icon: "tool", Color: "tool",
		Label: humanizeToolName(tool.Name), Description: truncateRunes(validUTF8(tool.Description), maxGenericDescRunes),
		Params: params, Tool: tool.Name, UntrustedOutput: true, AvailabilityFunc: availabilityOf(env, tool.Name),
	}
	for _, p := range params {
		if p.Kind == ParamText || p.Kind == ParamTextarea {
			def.PrimaryInput = p.Name
			break
		}
	}
	opName, choices := genericOperation(params)
	worst := genericWorstEffects(tool.Name, tool.Category, choices)
	def.EffectsFunc = func(n *Node) []Effect {
		if opName == "" {
			return genericEffects(tool.Name, tool.Category, "")
		}
		op, ok := genericLiteralOperation(n, opName, choices)
		if !ok {
			return append([]Effect(nil), worst...)
		}
		return genericEffects(tool.Name, tool.Category, op)
	}
	if len(choices) > 0 {
		def.Validate = func(n *Node, _ ValidateContext) []Issue {
			if n == nil {
				return nil
			}
			v, present := n.Params[opName]
			if !present || v == nil {
				return nil
			}
			if s, isText := v.(string); isText {
				// A template is checked by Execute once it is resolved; a blank one is
				// the required-parameter check's business.
				if HasTemplate(s) || strings.TrimSpace(s) == "" {
					return nil
				}
				if len(s) <= maxOperationBytes {
					if _, ok := choiceParam(s, "", choices...); ok {
						return nil
					}
				}
			}
			return []Issue{paramIssue(n, IssueParamInvalid, SeverityError, opName, opName+" is not one of the tool's operations")}
		}
	}
	def.Execute = func(ctx context.Context, in ExecInput) (ExecResult, error) {
		args, err := genericToolArgs(params, jsonStrings, in.Params)
		if err != nil {
			return ExecResult{}, err
		}
		if err := genericCheckOperation(args, opName, choices); err != nil {
			return ExecResult{}, err
		}
		out, err := callTool(ctx, in, tool.Name, args)
		if err != nil {
			return ExecResult{}, err
		}
		items := 0
		if list, ok := out["items"].([]any); ok {
			items = len(list)
		}
		return ExecResult{Output: out, ItemCount: items}, nil
	}
	return def
}

// genericOperation returns the parameter that selects the operation and the
// operations the schema lists for it (nil when it lists none).
func genericOperation(params []ParamSpec) (string, []string) {
	for _, name := range []string{"operation", "action"} {
		for _, p := range params {
			if p.Name != name {
				continue
			}
			var choices []string
			for _, o := range p.Options {
				choices = append(choices, o.Value)
			}
			return name, choices
		}
	}
	return "", nil
}

// genericLiteralOperation returns the operation of a raw node when it is a literal
// one: text, no template, and one of choices when there are any (in their spelling).
func genericLiteralOperation(n *Node, opName string, choices []string) (string, bool) {
	if n == nil {
		return "", false
	}
	s, ok := n.Params[opName].(string)
	if !ok || len(s) > maxOperationBytes || HasTemplate(s) {
		return "", false
	}
	if len(choices) == 0 {
		s = strings.TrimSpace(s)
		return s, s != ""
	}
	canonical, ok := choiceParam(s, "", choices...)
	return canonical, ok && canonical != ""
}

// genericCheckOperation refuses a resolved operation the schema does not list, so the
// effects shown before publishing (at worst the union over the list) hold at run time,
// and passes the listed spelling on.
func genericCheckOperation(args map[string]any, opName string, choices []string) error {
	v, ok := args[opName]
	if len(choices) == 0 || !ok {
		return nil
	}
	if s, isText := v.(string); isText && len(s) <= maxOperationBytes {
		if canonical, ok := choiceParam(s, "", choices...); ok {
			args[opName] = canonical
			return nil
		}
		return NewNodeError("FLOW_PARAM_INVALID", "%s %s is not one of the tool's operations", opName, quoteForError(s))
	}
	return NewNodeError("FLOW_PARAM_INVALID", "%s must be one of the tool's operations", opName)
}

var textareaParamNames = map[string]bool{"content": true, "body": true, "message": true, "prompt": true,
	"text": true, "html": true, "markdown": true, "code": true, "script": true}

// paramsFromSchema derives form parameters from a JSON schema: operation first, then
// required, then optional parameters (alphabetical). It also returns the parameters
// whose schema type is a string holding JSON.
//
// Any schema is accepted. A property that is not an object becomes a text parameter;
// a type list such as ["string","null"] counts as its first non-null type; nested
// objects become JSON parameters without being looked into; an enum ([]any or
// []string) longer than maxGenericOptions or without a usable entry is no enum.
// Invalid names (genericNameOK) and genericDroppedParams are left out, the list is cut
// to maxGenericParams and help texts to maxGenericHelpRunes. A parameter is a
// SensitiveSink when its name says so (isGenericSinkName).
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
		if !genericDroppedParams[name] && genericNameOK(name, maxGenericParamName, true) {
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
			Required: required[name], Templatable: true, SensitiveSink: isGenericSinkName(name)}
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
		case strings.Contains(lowerDesc, "json array") || strings.Contains(lowerDesc, "json object"):
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

// genericArgs is genericToolArgs for a caller that needs no error: nil when the
// arguments cannot be built.
func genericArgs(params []ParamSpec, jsonStrings map[string]bool, values map[string]any) map[string]any {
	args, err := genericToolArgs(params, jsonStrings, values)
	if err != nil {
		return nil
	}
	return args
}

func humanizeToolName(name string) string {
	s := strings.ReplaceAll(name, "_", " ")
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
