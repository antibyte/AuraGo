package agent

import (
	"path/filepath"
	"sort"
	"strings"

	openai "github.com/sashabaranov/go-openai"
)

type ToolKind string

const (
	ToolKindNative ToolKind = "native"
	ToolKindSkill  ToolKind = "skill"
	ToolKindCustom ToolKind = "custom"
	ToolKindMCP    ToolKind = "mcp"
)

type ToolStatus string

const (
	ToolStatusActive   ToolStatus = "active"
	ToolStatusHidden   ToolStatus = "hidden"
	ToolStatusDisabled ToolStatus = "disabled"
)

type ToolRouting struct {
	NativeAction string `json:"native_action,omitempty"`
	SkillName    string `json:"skill_name,omitempty"`
	CustomName   string `json:"custom_name,omitempty"`
	MCPServer    string `json:"mcp_server,omitempty"`
}

type ToolCatalogEntry struct {
	Name         string      `json:"name"`
	Kind         ToolKind    `json:"kind"`
	Aliases      []string    `json:"aliases,omitempty"`
	Category     string      `json:"category,omitempty"`
	Schema       openai.Tool `json:"-"`
	ManualPath   string      `json:"manual_path,omitempty"`
	Routing      ToolRouting `json:"routing"`
	Status       ToolStatus  `json:"status"`
	Enabled      bool        `json:"enabled"`
	Active       bool        `json:"active"`
	HiddenReason string      `json:"hidden_reason,omitempty"`
	Description  string      `json:"description,omitempty"`
}

type ToolCatalog struct {
	entries map[string]*ToolCatalogEntry
	aliases map[string]string
}

func BuildToolCatalog(allSchemas, activeSchemas []openai.Tool, promptsDir string) *ToolCatalog {
	active := make(map[string]bool, len(activeSchemas))
	for _, schema := range activeSchemas {
		if schema.Function != nil && schema.Function.Name != "" {
			active[schema.Function.Name] = true
		}
	}

	c := &ToolCatalog{
		entries: make(map[string]*ToolCatalogEntry),
		aliases: make(map[string]string),
	}

	for _, schema := range allSchemas {
		if schema.Function == nil || schema.Function.Name == "" {
			continue
		}
		entry := catalogEntryFromSchema(schema, active[schema.Function.Name], promptsDir)
		c.add(entry)
	}

	// Add categorized tools that are absent from all enabled schemas as disabled
	// entries so discover_tools can distinguish "known but disabled" from
	// "unknown".
	for category, entries := range toolCategoryDef {
		for _, def := range entries {
			if _, ok := c.Get(def.Name); ok {
				continue
			}
			c.add(&ToolCatalogEntry{
				Name:         def.Name,
				Kind:         ToolKindNative,
				Category:     category,
				ManualPath:   manualPathFor(promptsDir, def.Name),
				Routing:      ToolRouting{NativeAction: def.Name},
				Status:       ToolStatusDisabled,
				Enabled:      false,
				Active:       false,
				HiddenReason: "config_disabled",
				Description:  def.ShortDesc,
			})
		}
	}

	return c
}

func catalogEntryFromSchema(schema openai.Tool, active bool, promptsDir string) *ToolCatalogEntry {
	name := schema.Function.Name
	description := schema.Function.Description
	entry := &ToolCatalogEntry{
		Name:        name,
		Kind:        ToolKindNative,
		Category:    ToolCategoryForTool(name),
		Schema:      schema,
		ManualPath:  manualPathFor(promptsDir, name),
		Routing:     ToolRouting{NativeAction: name},
		Status:      ToolStatusHidden,
		Enabled:     true,
		Active:      active,
		Description: description,
	}
	if active {
		entry.Status = ToolStatusActive
	} else {
		entry.HiddenReason = "adaptive_filter"
	}

	switch {
	case strings.HasPrefix(name, "skill__"):
		skillName := strings.TrimPrefix(name, "skill__")
		entry.Name = skillName
		entry.Kind = ToolKindSkill
		entry.Aliases = []string{name}
		entry.Category = "skills"
		entry.ManualPath = manualPathFor(promptsDir, skillName)
		entry.Routing = ToolRouting{SkillName: skillName}
	case strings.HasPrefix(name, "tool__"):
		toolName := strings.TrimPrefix(name, "tool__")
		entry.Name = toolName
		entry.Kind = ToolKindCustom
		entry.Aliases = []string{name}
		entry.Category = "custom"
		entry.ManualPath = manualPathFor(promptsDir, toolName)
		entry.Routing = ToolRouting{CustomName: toolName}
	case name == "mcp_call":
		entry.Kind = ToolKindMCP
		entry.Routing = ToolRouting{NativeAction: name}
	}

	return entry
}

func (c *ToolCatalog) add(entry *ToolCatalogEntry) {
	if c == nil || entry == nil || entry.Name == "" {
		return
	}
	c.entries[entry.Name] = entry
	c.aliases[entry.Name] = entry.Name
	for _, alias := range entry.Aliases {
		if alias != "" {
			c.aliases[alias] = entry.Name
		}
	}
}

func (c *ToolCatalog) Get(name string) (*ToolCatalogEntry, bool) {
	if c == nil {
		return nil, false
	}
	name = strings.TrimSpace(name)
	if canonical, ok := c.aliases[name]; ok {
		entry, ok := c.entries[canonical]
		return entry, ok
	}
	entry, ok := c.entries[name]
	return entry, ok
}

func (c *ToolCatalog) Entries() []*ToolCatalogEntry {
	if c == nil {
		return nil
	}
	names := make([]string, 0, len(c.entries))
	for name := range c.entries {
		names = append(names, name)
	}
	sort.Strings(names)
	out := make([]*ToolCatalogEntry, 0, len(names))
	for _, name := range names {
		out = append(out, c.entries[name])
	}
	return out
}

func (c *ToolCatalog) Search(query string) []*ToolCatalogEntry {
	query = strings.ToLower(strings.TrimSpace(resolveDiscoverToolName(query)))
	if query == "" {
		return nil
	}
	var out []*ToolCatalogEntry
	for _, entry := range c.Entries() {
		if strings.Contains(strings.ToLower(entry.Name), query) ||
			strings.Contains(strings.ToLower(entry.Description), query) ||
			strings.Contains(strings.ToLower(entry.Category), query) {
			out = append(out, entry)
		}
	}
	return out
}

func (c *ToolCatalog) ByCategory(category string) []*ToolCatalogEntry {
	var out []*ToolCatalogEntry
	for _, entry := range c.Entries() {
		if category == "" || entry.Category == category {
			out = append(out, entry)
		}
	}
	return out
}

func ToolCategoryForTool(name string) string {
	if strings.HasPrefix(name, "skill__") {
		return "skills"
	}
	if strings.HasPrefix(name, "tool__") {
		return "custom"
	}
	return ToolCategoryForName(name)
}

func manualPathFor(promptsDir, name string) string {
	if promptsDir == "" || name == "" {
		return ""
	}
	return filepath.Join(promptsDir, "tools_manuals", name+".md")
}

func callMethodForEntry(entry *ToolCatalogEntry) string {
	if entry == nil {
		return ""
	}
	switch entry.Kind {
	case ToolKindSkill:
		return "execute_skill"
	case ToolKindCustom:
		return "run_tool"
	case ToolKindNative, ToolKindMCP:
		if entry.Status == ToolStatusHidden {
			return "invoke_tool"
		}
		return "direct"
	default:
		return "unknown"
	}
}
