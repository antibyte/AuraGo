package flows

import (
	"context"
	"strings"
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

// genericExcludedPrefixes name dynamic tools: skills, custom tools, Game Maker job
// tools, and MCP and package tools whose names a server or a skill package chooses.
var genericExcludedPrefixes = []string{"skill__", "tool__", "game_maker_", "mcp__", "package__"}

// genericCuratedAllowed lists the tools a curated node calls that stay generic nodes
// as well, each with the reason no rule of the curated node is lost. Every other tool
// of a curated node is excluded: by genericExcluded, and at refresh time by the Tool of
// any registered curated definition. A test fails for a curated tool in neither list.
var genericCuratedAllowed = map[string]string{
	"filesystem": "file.read and file.write add path bounds, the if_exists probe and a success check to their own " +
		"options; delete, move, copy, list and the batch operations exist only here. Their taint rules hold here too: " +
		"file_path, destination and the paths inside items are sinks by name, and content is a file content sink",
	"manage_appointments": "list, update and delete exist only here; the title (it reaches the agent's prompt) is a sink by " +
		"name, and agent_instruction and wake_agent are dropped, as planner.appointment_add offers neither",
	"manage_todos": "list, update and complete exist only here; the title (it reaches the agent's prompt) is a sink by " +
		"name, also inside items",
}

// IsGenericToolExcluded reports whether a tool never becomes a generic node.
func IsGenericToolExcluded(name string) bool {
	if genericExcluded[name] {
		return true
	}
	for _, p := range genericExcludedPrefixes {
		if strings.HasPrefix(name, p) {
			return true
		}
	}
	return false
}

func isGenericDef(d *NodeDef) bool { return strings.HasPrefix(d.Type, GenericTypePrefix) }

// RefreshGenericTools replaces all generic tool nodes with nodes for tools in one
// atomic step (Registry.ReplaceWhere) and returns how many were registered. Skipped:
// excluded tools (IsGenericToolExcluded), tools a registered curated definition calls
// (its Tool) unless genericCuratedAllowed lists them, tools that genericDroppedOperations
// leave without an operation, invalid names and repeated names (the first one wins).
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
		if def := genericToolDef(tool, env); def != nil {
			defs = append(defs, def)
		}
	}
	reg.ReplaceWhere(isGenericDef, defs)
	return len(defs)
}

// genericToolDef builds the node of one tool, or returns nil when genericToolParams
// leaves it without an operation. The output is untrusted; parameters and sinks come
// from genericToolParams. The operation parameter ("operation", else
// "action") decides the effects: a literal one is classified (genericEffects);
// anything else gets the worst case, the union over the operations the schema lists,
// which Execute enforces, or every effect when there is no list (genericWorstEffects).
func genericToolDef(tool GenericTool, env CatalogEnv) *NodeDef {
	params, jsonStrings, ok := genericToolParams(tool)
	if !ok {
		return nil
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
	opName, choices := genericOperation(params)
	for _, p := range params {
		if (p.Kind == ParamText || p.Kind == ParamTextarea) && p.Name != opName {
			def.PrimaryInput = p.Name
			break
		}
	}
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
