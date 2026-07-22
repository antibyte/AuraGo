package agent

import (
	"sort"

	"aurago/internal/config"
	"aurago/internal/toolmeta"
)

type ToolVisibilityClass string

const (
	ToolVisibilityHardAlways ToolVisibilityClass = "hard_always"
	ToolVisibilityCommon     ToolVisibilityClass = "common"
	ToolVisibilityAdaptive   ToolVisibilityClass = "adaptive"
	ToolVisibilityHidden     ToolVisibilityClass = "hidden"
)

type ToolRegistryMetadata struct {
	Name             string
	Family           string
	VisibilityClass  ToolVisibilityClass
	RequiresWrite    bool
	Dispatcher       string
	CompressionClass toolmeta.CompressionClass
}

var builtinToolMetadata = map[string]ToolRegistryMetadata{
	"discover_tools":         {Name: "discover_tools", Family: "tooling", VisibilityClass: ToolVisibilityHardAlways, Dispatcher: "agent"},
	"invoke_tool":            {Name: "invoke_tool", Family: "tooling", VisibilityClass: ToolVisibilityHardAlways, Dispatcher: "agent"},
	"execute_skill":          {Name: "execute_skill", Family: "skills", VisibilityClass: ToolVisibilityHardAlways, Dispatcher: "exec", RequiresWrite: true},
	"list_agent_skills":      {Name: "list_agent_skills", Family: "agent_skills", VisibilityClass: ToolVisibilityAdaptive, Dispatcher: "exec"},
	"activate_agent_skill":   {Name: "activate_agent_skill", Family: "agent_skills", VisibilityClass: ToolVisibilityAdaptive, Dispatcher: "exec"},
	"run_agent_skill_script": {Name: "run_agent_skill_script", Family: "agent_skills", VisibilityClass: ToolVisibilityAdaptive, Dispatcher: "exec", RequiresWrite: true},
	"run_tool":               {Name: "run_tool", Family: "custom_tools", VisibilityClass: ToolVisibilityHardAlways, Dispatcher: "exec", RequiresWrite: true},
	"filesystem":             {Name: "filesystem", Family: "filesystem", VisibilityClass: ToolVisibilityCommon, Dispatcher: "exec"},
	"file_editor":            {Name: "file_editor", Family: "filesystem", VisibilityClass: ToolVisibilityCommon, Dispatcher: "exec", RequiresWrite: true},
	"docker":                 {Name: "docker", Family: "infra", VisibilityClass: ToolVisibilityAdaptive, Dispatcher: "infra"},
	"homepage":               {Name: "homepage", Family: "desktop", VisibilityClass: ToolVisibilityAdaptive, Dispatcher: "services", RequiresWrite: true},
	"homepage_project":       {Name: "homepage_project", Family: "homepage", VisibilityClass: ToolVisibilityAdaptive, Dispatcher: "services", RequiresWrite: true},
	"homepage_file":          {Name: "homepage_file", Family: "homepage", VisibilityClass: ToolVisibilityAdaptive, Dispatcher: "services", RequiresWrite: true},
	"homepage_quality":       {Name: "homepage_quality", Family: "homepage", VisibilityClass: ToolVisibilityAdaptive, Dispatcher: "services"},
	"homepage_deploy":        {Name: "homepage_deploy", Family: "homepage", VisibilityClass: ToolVisibilityAdaptive, Dispatcher: "services", RequiresWrite: true},
	"homepage_git":           {Name: "homepage_git", Family: "homepage", VisibilityClass: ToolVisibilityAdaptive, Dispatcher: "services", RequiresWrite: true},
}

func lookupToolMetadata(name string) (ToolRegistryMetadata, bool) {
	meta, ok := builtinToolMetadata[name]
	if shared, sharedOK := toolmeta.Lookup(name); sharedOK {
		meta.Name = shared.Name
		if meta.Family == "" {
			meta.Family = shared.Family
		}
		meta.CompressionClass = shared.CompressionClass
		ok = true
	}
	return meta, ok
}

func hardAlwaysToolNames(cfg *config.Config) []string {
	hard := make([]string, 0, len(builtinToolMetadata))
	for name, meta := range builtinToolMetadata {
		if meta.VisibilityClass == ToolVisibilityHardAlways {
			hard = append(hard, name)
		}
	}
	sort.Strings(hard)
	return hard
}
