package agent

import (
	"strings"
	"sync"

	"aurago/internal/gamemaker"
)

// canonicalNativeAlias covers equivalent capabilities only. Fixed git aliases
// deliberately do not belong here: allowing git_status must not grant a shell.
func canonicalNativeAlias(name string) string {
	switch name {
	case "register_server":
		return "register_device"
	case "remote_execution":
		return "execute_remote_shell"
	case "get_system_metrics":
		return "system_metrics"
	case "s3":
		return "s3_storage"
	case "paperless":
		return "paperless_ngx"
	default:
		return name
	}
}

func canonicalActionName(name string) string {
	switch name {
	case "git_status", "git_log", "git_diff", "git_push", "git_pull", "git_rollback", "git_commit":
		return "execute_shell"
	default:
		return canonicalNativeAlias(name)
	}
}

// preparationCatalogEntry supplements an adaptive snapshot with the complete
// enabled, scoped catalog. Presentation filtering is never an execution gate.
// Existing entries retain package/MCP/custom routing supplied by their run.
func preparationCatalogEntry(name string, dc *DispatchContext) (*ToolCatalogEntry, bool) {
	if dc == nil {
		return nil, false
	}
	var previous *ToolCatalogEntry
	if catalog := GetToolCatalogState(dc.discoveryKey()); catalog != nil {
		if entry, ok := canonicalCatalogEntry(catalog, name); ok {
			previous = entry
			if entry.Enabled || entry.HiddenReason != "config_disabled" {
				return entry, true
			}
		}
	}
	if dc.Cfg == nil {
		return previous, previous != nil
	}
	catalog := BuildToolCatalog(dispatchCatalogSchemas(dc), nil, dc.Cfg.Directories.PromptsDir)
	applyCatalogScope(catalog, dc)
	if entry, ok := canonicalCatalogEntry(catalog, name); ok {
		return entry, true
	}
	return previous, previous != nil
}

func canonicalCatalogEntry(catalog *ToolCatalog, name string) (*ToolCatalogEntry, bool) {
	if entry, ok := catalog.Get(name); ok {
		return entry, true
	}
	canonical := canonicalNativeAlias(name)
	if entry, ok := catalog.Get(canonical); ok {
		return entry, true
	}
	// The SSH schema retains its published wire name; its dispatch leaf has a
	// different historical spelling. Both represent exactly the same capability.
	if canonical == "execute_remote_shell" {
		return catalog.Get("remote_execution")
	}
	return nil, false
}

// discoverOperationNames are discover_tools operations that models sometimes
// call as if each were a tool of its own. Composio's search_tools is left out on
// purpose: it is a composio_call operation, so rewriting it would be ambiguous.
var discoverOperationNames = map[string]struct{}{
	"get_tool_info": {}, "search": {}, "list_categories": {}, "list_family": {}, "get_manual": {},
}

// canonicalizeDiscoverOperationCall turns a call to a discover_tools operation
// name into a discover_tools call carrying that operation. It leaves real tools
// alone (a builtin or an exact catalog entry of the run with that name). It never
// widens a scope: TransportAction stays empty, so a run without discover_tools
// still denies the rewritten call in the policy and scope checks that follow.
func canonicalizeDiscoverOperationCall(tc ToolCall, dc *DispatchContext) ToolCall {
	if _, operation := discoverOperationNames[tc.Action]; !operation {
		return tc
	}
	if _, builtin := allBuiltinToolNameSet()[tc.Action]; builtin {
		return tc
	}
	if dc != nil {
		if entry, ok := GetToolCatalogState(dc.discoveryKey()).Get(tc.Action); ok && entry.Name == tc.Action {
			return tc
		}
	}
	args := map[string]interface{}{}
	if tc.Params != nil {
		args = cloneJSONSchemaValue(tc.Params).(map[string]interface{})
	}
	args["operation"] = tc.Action
	if stringValueFromMap(args, "tool_name", "name", "tool") == "" {
		if name := firstNonEmptyToolString(tc.ToolName, tc.Name, tc.Tool); name != "" {
			args["tool_name"] = name
		}
	}
	if stringValueFromMap(args, "query", "q", "search_query", "keyword") == "" && tc.Query != "" {
		args["query"] = tc.Query
	}
	tc.Params, tc.Operation, tc.Action = args, tc.Action, "discover_tools"
	return tc
}

// gameMakerToolOperations lists the operations of each Game Maker tool, read
// from the tool schemas so the list cannot drift. game_maker_validate has no
// operation enum; its single action is called validate.
var gameMakerToolOperations = sync.OnceValue(func() map[string][]string {
	operations := map[string][]string{"game_maker_validate": {"validate"}}
	for _, definition := range appendGameMakerToolSchemas(nil, ToolFeatureFlags{GameMakerEnabled: true}) {
		params, _ := definition.Function.Parameters.(map[string]interface{})
		properties, _ := params["properties"].(map[string]interface{})
		operation, _ := properties["operation"].(map[string]interface{})
		if enum, _ := operation["enum"].([]string); len(enum) > 0 {
			operations[definition.Function.Name] = enum
		}
	}
	return operations
})

// gameMakerOperationOwners maps an operation name to the one tool that owns it.
// The enums are disjoint (guarded by a test), so the owner is unambiguous.
var gameMakerOperationOwners = sync.OnceValue(func() map[string]string {
	owners := map[string]string{}
	for tool, operations := range gameMakerToolOperations() {
		for _, operation := range operations {
			owners[operation] = tool
		}
	}
	return owners
})

// gameMakerDesignFields are the keys of the design object. Only these move into
// it when set_design fields arrive flat: a text-mode call also carries transport
// residue (tool, arguments, ...) that must never reach the design.
var gameMakerDesignFields = sync.OnceValue(func() map[string]struct{} {
	fields := map[string]struct{}{}
	properties, _ := gameDesignSchema()["properties"].(map[string]interface{})
	for key := range properties {
		fields[key] = struct{}{}
	}
	return fields
})

// canonicalizeGameMakerOperationCall repairs the two ways weak models misaddress
// the Game Maker tools inside a Studio run: they call an operation name as if it
// were a tool (set_design, get_plan, search_assets), or pass an operation to the
// tool that does not own it (game_maker_file list_files). set_design fields given
// flat are wrapped into the design object. It is a strict no-op unless a Studio
// job is bound to the context, the name is no real tool, and the owning tool is
// inside the run's scope: it never widens a scope, so an out-of-scope call keeps
// the ordinary denial.
func canonicalizeGameMakerOperationCall(tc ToolCall, dc *DispatchContext) ToolCall {
	if dc == nil || dc.Ctx == nil || gamemaker.JobIDFromContext(dc.Ctx) == "" {
		return tc
	}
	owners := gameMakerOperationOwners()
	name := tc.Action
	operation := firstNonEmptyToolString(tc.Operation, toolArgString(tc.Params, "operation"))
	owner := ""
	if _, studioTool := gameMakerToolOperations()[name]; studioTool {
		// game_maker_validate ignores its operation, so it never reroutes.
		if owner = owners[operation]; owner == "" || name == "game_maker_validate" {
			owner = name
		}
	} else {
		if owner = owners[name]; owner == "" {
			return tc
		}
		if _, builtin := allBuiltinToolNameSet()[name]; builtin {
			return tc
		}
		if entry, ok := GetToolCatalogState(dc.discoveryKey()).Get(name); ok && entry.Name == name {
			return tc
		}
		if operation == "" {
			operation = name
		}
	}
	if !dispatchToolAllowed(dc, owner) {
		return tc
	}
	_, hasBase := tc.Params["base"]
	_, hasObjective := tc.Params["objective"]
	wrap := operation == "set_design" && tc.Params["design"] == nil && (hasBase || hasObjective)
	if owner == name && !wrap {
		return tc
	}
	args := map[string]interface{}{}
	if tc.Params != nil {
		args = cloneJSONSchemaValue(tc.Params).(map[string]interface{})
	}
	if wrap {
		design := map[string]interface{}{}
		for key, value := range args {
			if _, field := gameMakerDesignFields()[key]; field {
				design[key] = value
				delete(args, key)
			}
		}
		args["design"] = design
	}
	args["operation"] = operation
	if dc.Logger != nil {
		dc.Logger.Info("Game Maker call normalized", "from", name, "tool", owner, "operation", operation, "wrapped_design", wrap)
	}
	tc.normalizedFrom, tc.Params, tc.Operation, tc.Action = name, args, operation, owner
	return tc
}

// canonicalizeLeafCall runs before Guardian, role, scope, learning and action
// policy checks. It only rewrites known routes and never performs an action.
func canonicalizeLeafCall(tc ToolCall, dc *DispatchContext) ToolCall {
	tc = canonicalizeGameMakerOperationCall(tc, dc)
	tc = canonicalizeDiscoverOperationCall(tc, dc)
	if tc.Action == "execute_skill" {
		tc = canonicalizeNativeSkillWrapper(tc, dc)
		if tc.PreparationError != "" {
			return tc
		}
	}
	original := tc.Action
	tc.Action = canonicalActionName(original)
	gitCommand := ""
	switch original {
	case "git_status":
		gitCommand = "git status"
	case "git_log":
		gitCommand = "git log --oneline -10"
	case "git_diff":
		gitCommand = "git diff"
	case "git_push":
		gitCommand = "git push"
	case "git_pull":
		gitCommand = "git pull"
	case "git_rollback":
		gitCommand = "git reset --hard HEAD~1"
	case "git_commit":
		message := firstNonEmptyToolString(tc.Message, toolArgString(tc.Params, "message"))
		gitCommand = "git add -A && git commit -m " + quoteCanonicalShellArgument(message)
	case "exec":
		if decodeShellExecutionArgs(tc).Command != "" {
			tc.Action = "execute_shell"
		}
	}
	if gitCommand != "" {
		if tc.TransportAction == "" {
			tc.TransportAction = original
		}
		tc.Operation = "execute"
		path := firstNonEmptyToolString(tc.Path, toolArgString(tc.Params, "path"))
		if path != "" {
			gitCommand = "cd " + quoteCanonicalShellArgument(path) + " && " + gitCommand
		}
		tc.Command = gitCommand
		args := map[string]interface{}{}
		if tc.Params != nil {
			args = cloneJSONSchemaValue(tc.Params).(map[string]interface{})
		}
		args["command"], args["operation"] = gitCommand, tc.Operation
		tc.Params = args
	}
	tc.Operation = toolCallSnapshotOperation(tc)
	return tc
}

func quoteCanonicalShellArgument(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'"'"'`) + "'"
}

func canonicalizeNativeSkillWrapper(tc ToolCall, dc *DispatchContext) ToolCall {
	args := tc.SkillArgs
	if args == nil {
		args = tc.Params
	}
	if len(args) == 0 {
		args = synthesizeExecuteSkillArgs(tc)
	}
	name := strings.TrimSuffix(strings.TrimSpace(tc.Skill), ".py")
	if name == "" {
		name = strings.TrimSuffix(toolArgString(args, "skill", "skill_name", "name", "tool"), ".py")
	}
	lookup := dc
	if dc != nil {
		// Resolve identity independently of scope; both the wrapper and the leaf
		// are checked afterwards. A hidden/out-of-scope leaf must not fall back
		// to executing the wrapper as if it were an ordinary Python skill.
		local := *dc
		local.ToolScopeRestricted, local.AllowedTools = false, nil
		lookup = &local
	}
	var entry *ToolCatalogEntry
	ok := false
	if lookup != nil && lookup.Cfg != nil {
		catalog := BuildToolCatalog(dispatchCatalogSchemas(lookup), nil, lookup.Cfg.Directories.PromptsDir)
		entry, ok = canonicalCatalogEntry(catalog, name)
	}
	if !ok || entry.Kind != ToolKindNative {
		entry, ok = preparationCatalogEntry(name, lookup)
	}
	if !ok || entry.Kind != ToolKindNative {
		return tc // Real Python skills keep their ordinary manager/runtime path.
	}
	if entry.Schema.Function == nil {
		if _, native := allBuiltinToolNameSet()[entry.Name]; !native {
			return tc // Category-only skills such as PDF extraction have no native leaf.
		}
	}
	if entry.Name == "invoke_tool" || entry.Name == "execute_skill" {
		tc.PreparationError = `Tool Output: {"status":"policy_denied","code":"tool_scope_denied","message":"Skill wrappers cannot recursively target transport wrappers."}`
		return tc
	}
	if !dispatchToolAllowed(dc, "execute_skill") {
		tc.PreparationError = toolScopeDeniedFor(dc, "execute_skill")
		return tc
	}
	if inner, ok := args["skill_args"].(map[string]interface{}); ok && len(inner) > 0 {
		args = inner
	} else {
		clean := make(map[string]interface{}, len(args))
		properties, _ := schemaParameters(entry.Schema)["properties"].(map[string]interface{})
		for key, value := range args {
			switch key {
			case "skill", "skill_name", "name", "tool", "action":
				if _, leafArgument := properties[key]; !leafArgument {
					continue
				}
			}
			clean[key] = value
		}
		args = clean
	}
	args = cloneJSONSchemaValue(args).(map[string]interface{})
	stripStrictNulls(args, originalToolArgumentSchema(entry.Name, dc))
	action := entry.Routing.NativeAction
	if action == "" {
		action = entry.Name
	}
	routed := toolCallFromInvokeArgs(action, args)
	routed.NativeCallID, routed.TransportAction = tc.NativeCallID, tc.TransportAction
	if routed.TransportAction == "" {
		routed.TransportAction = "execute_skill"
	}
	routed.RawJSON, routed.NativeArgsRaw = tc.RawJSON, tc.NativeArgsRaw
	routed.NativeArgsMalformed, routed.NativeArgsError = tc.NativeArgsMalformed, tc.NativeArgsError
	routed.IsTool, routed.Todo = tc.IsTool, tc.Todo
	routed.RawCodeDetected, routed.XMLFallbackDetected = tc.RawCodeDetected, tc.XMLFallbackDetected
	return routed
}
