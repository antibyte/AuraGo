package agent

import (
	"aurago/internal/tools"
	"encoding/json"
	"fmt"
	"sync/atomic"

	openai "github.com/sashabaranov/go-openai"
)

var discoveryRunCounter atomic.Uint64

func acquireDiscoveryRun() string {
	id := fmt.Sprintf("run:%d", discoveryRunCounter.Add(1))
	discoverToolsState.mu.Lock()
	if discoverToolsState.liveRuns == nil {
		discoverToolsState.liveRuns = make(map[string]bool)
	}
	discoverToolsState.liveRuns[id] = true
	discoverToolsState.mu.Unlock()
	return id
}

func releaseDiscoveryRun(id string) {
	discoverToolsState.mu.Lock()
	defer discoverToolsState.mu.Unlock()
	delete(discoverToolsState.liveRuns, id)
	delete(discoverToolsState.snapshots, id)
	delete(discoverToolsState.requested, id)
}

func (dc *DispatchContext) discoveryKey() string {
	if dc.DiscoveryRunID != "" {
		return dc.DiscoveryRunID
	}
	return dc.SessionID
}

func dispatchCatalogSchemas(dc *DispatchContext) []openai.Tool {
	if dc == nil || dc.Cfg == nil {
		return nil
	}
	policy := BuildToolingPolicy(dc.Cfg, "")
	flags := buildToolFeatureFlags(RunConfig{Config: dc.Cfg, IsCoAgent: dc.IsCoAgent}, policy)
	schemas := BuildNativeToolSchemas(dc.Cfg.Directories.SkillsDir, dc.Manifest, flags, dc.Logger)
	if dc.ToolScopeRestricted {
		names := make([]string, 0, len(dc.AllowedTools))
		for name := range dc.AllowedTools {
			names = append(names, name)
		}
		schemas = filterSchemasByAllowedTools(schemas, names)
	}
	return schemas
}

func setRunDiscoverToolsState(dc *DispatchContext, all, active []openai.Tool) {
	SetDiscoverToolsState(dc.discoveryKey(), all, active, dc.Cfg.Directories.PromptsDir, dc)
}

func applyCatalogScope(catalog *ToolCatalog, dc *DispatchContext) {
	addIntegrationCatalogEntries(catalog, dc)
	applyCatalogEntryPolicies(catalog, dc)
}

func applyCatalogEntryPolicies(catalog *ToolCatalog, dc *DispatchContext) {
	for _, entry := range catalog.Entries() {
		if !catalogEntryAllowed(dc, entry) {
			entry.Enabled, entry.Active, entry.Status, entry.HiddenReason = false, false, ToolStatusDisabled, "out_of_scope"
			continue
		}
		entry.ReadOnly = catalogReadOnly(dc.Cfg, entry)
		if entry.Enabled && (dc.ExecutionHooks == nil || dc.ExecutionHooks.HandleTool == nil) {
			if reason := toolRuntimeUnavailable(dc, entry.Name); reason != "" {
				entry.Enabled, entry.Active, entry.Status, entry.HiddenReason = false, false, ToolStatusNeedsSetup, reason
			}
		}
		if !entry.Enabled || entry.Kind != ToolKindNative {
			continue
		}
		ops := toolCatalogOperationField(entry, "operation")
		if len(ops) == 0 {
			if roleToolRestriction(ToolCall{Action: entry.Name}, dc) != "" {
				entry.Enabled, entry.Active, entry.Status, entry.HiddenReason = false, false, ToolStatusDisabled, "role_restricted"
			}
			continue
		}
		allowed := make([]string, 0, len(ops))
		for _, op := range ops {
			if roleToolRestriction(ToolCall{Action: entry.Name, Operation: op}, dc) == "" {
				allowed = append(allowed, op)
			}
		}
		if len(allowed) == len(ops) {
			continue
		}
		if len(allowed) == 0 {
			entry.Enabled, entry.Active, entry.Status, entry.HiddenReason = false, false, ToolStatusDisabled, "role_restricted"
			continue
		}
		// Never mutate cached schemas shared with another run.
		raw, _ := json.Marshal(entry.Schema)
		var cloned openai.Tool
		if json.Unmarshal(raw, &cloned) == nil {
			entry.Schema = cloned
			props, _ := schemaParameters(cloned)["properties"].(map[string]interface{})
			if op, ok := props["operation"].(map[string]interface{}); ok {
				op["enum"] = allowed
			}
		}
	}
}

func catalogEntryAllowed(dc *DispatchContext, entry *ToolCatalogEntry) bool {
	action := entry.Routing.NativeAction
	if entry.Kind == ToolKindSkill {
		action = "execute_skill"
	}
	if entry.Kind == ToolKindCustom {
		action = "run_tool"
	}
	if action == "" {
		action = entry.Name
	}
	return dispatchToolAllowed(dc, action)
}

func addIntegrationCatalogEntries(catalog *ToolCatalog, dc *DispatchContext) {
	if parent, ok := catalog.Get("mcp_call"); ok && parent.Enabled && dispatchToolAllowed(dc, "mcp_call") {
		if manager := tools.GetMCPManager(); manager != nil {
			for _, item := range manager.CachedTools("") {
				name := fmt.Sprintf("mcp__%x__%x", item.Server, item.Name)
				catalog.add(&ToolCatalogEntry{Name: name, Kind: ToolKindMCP, Aliases: []string{item.Server + "/" + item.Name}, Category: "mcp", Description: item.Description, Enabled: true, Status: ToolStatusHidden, HiddenReason: "external_tool", ManualPath: parent.ManualPath, Routing: ToolRouting{NativeAction: "mcp_call", MCPServer: item.Server, MCPTool: item.Name}, Schema: openai.Tool{Type: openai.ToolTypeFunction, Function: &openai.FunctionDefinition{Name: name, Description: item.Description, Parameters: item.InputSchema}}})
			}
		}
	}
	parent, packageEnabled := catalog.Get("activate_agent_skill")
	if manager := tools.DefaultAgentSkillManager(); manager != nil && packageEnabled && parent.Enabled && dispatchToolAllowed(dc, "activate_agent_skill") {
		if entries, err := manager.ListAgentSkills(true, ""); err == nil {
			for _, item := range entries {
				if !agentSkillAllowed(dc, item.Name) || ensureAgentSkillUsable(&item) != nil {
					continue
				}
				catalog.add(&ToolCatalogEntry{Name: "package__" + item.Name, Kind: ToolKindPackage, Aliases: []string{item.Name}, Category: "agent_skills", Description: item.Description, Enabled: true, Status: ToolStatusHidden, HiddenReason: "on_demand_package", Routing: ToolRouting{NativeAction: "activate_agent_skill", SkillName: item.Name}})
			}
		}
	}
}

func discoveryRunKey(cfg RunConfig) string {
	if cfg.DiscoveryRunID != "" {
		return cfg.DiscoveryRunID
	}
	return cfg.SessionID
}

func scopedCatalogSchemas(schemas []openai.Tool, dc *DispatchContext) []openai.Tool {
	if dc == nil {
		return schemas
	}
	catalog := BuildToolCatalog(schemas, schemas, "")
	applyCatalogEntryPolicies(catalog, dc)
	result := make([]openai.Tool, 0, len(schemas))
	for _, schema := range schemas {
		if schema.Function == nil {
			continue
		}
		if entry, ok := catalog.Get(schema.Function.Name); ok && entry.Enabled {
			result = append(result, entry.Schema)
		}
	}
	return result
}
