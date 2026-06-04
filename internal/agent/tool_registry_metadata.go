package agent

import (
	"sort"

	"aurago/internal/config"
)

type ToolVisibilityClass string

const (
	ToolVisibilityHardAlways ToolVisibilityClass = "hard_always"
	ToolVisibilityCommon     ToolVisibilityClass = "common"
	ToolVisibilityAdaptive   ToolVisibilityClass = "adaptive"
	ToolVisibilityHidden     ToolVisibilityClass = "hidden"
)

type ToolRegistryMetadata struct {
	Name            string
	Family          string
	VisibilityClass ToolVisibilityClass
	RequiresWrite   bool
	Dispatcher      string
}

var builtinToolMetadata = map[string]ToolRegistryMetadata{
	"discover_tools":         {Name: "discover_tools", Family: "tooling", VisibilityClass: ToolVisibilityHardAlways, Dispatcher: "agent"},
	"invoke_tool":            {Name: "invoke_tool", Family: "tooling", VisibilityClass: ToolVisibilityHardAlways, Dispatcher: "agent"},
	"execute_skill":          {Name: "execute_skill", Family: "skills", VisibilityClass: ToolVisibilityHardAlways, Dispatcher: "exec", RequiresWrite: true},
	"list_agent_skills":      {Name: "list_agent_skills", Family: "agent_skills", VisibilityClass: ToolVisibilityHardAlways, Dispatcher: "exec"},
	"activate_agent_skill":   {Name: "activate_agent_skill", Family: "agent_skills", VisibilityClass: ToolVisibilityHardAlways, Dispatcher: "exec"},
	"run_agent_skill_script": {Name: "run_agent_skill_script", Family: "agent_skills", VisibilityClass: ToolVisibilityHardAlways, Dispatcher: "exec", RequiresWrite: true},
	"run_tool":               {Name: "run_tool", Family: "custom_tools", VisibilityClass: ToolVisibilityHardAlways, Dispatcher: "exec", RequiresWrite: true},
	"filesystem":             {Name: "filesystem", Family: "filesystem", VisibilityClass: ToolVisibilityCommon, Dispatcher: "exec"},
	"file_editor":            {Name: "file_editor", Family: "filesystem", VisibilityClass: ToolVisibilityCommon, Dispatcher: "exec", RequiresWrite: true},
	"docker":                 {Name: "docker", Family: "infra", VisibilityClass: ToolVisibilityAdaptive, Dispatcher: "infra"},
	"homepage":               {Name: "homepage", Family: "desktop", VisibilityClass: ToolVisibilityAdaptive, Dispatcher: "services", RequiresWrite: true},
}

func lookupToolMetadata(name string) (ToolRegistryMetadata, bool) {
	meta, ok := builtinToolMetadata[name]
	return meta, ok
}

func hardAlwaysToolNames(cfg *config.Config) []string {
	hard := make([]string, 0, len(builtinToolMetadata)+1)
	for name, meta := range builtinToolMetadata {
		if meta.VisibilityClass == ToolVisibilityHardAlways {
			hard = append(hard, name)
		}
	}
	sort.Strings(hard)
	if cfg != nil && cfg.MCP.Enabled && cfg.Agent.AllowMCP {
		hard = append(hard, "mcp_call")
	}
	return hard
}
