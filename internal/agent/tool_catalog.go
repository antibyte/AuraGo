package agent

import (
	"aurago/internal/prompts"
	"encoding/json"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	openai "github.com/sashabaranov/go-openai"
)

type ToolKind string

const (
	ToolKindNative  ToolKind = "native"
	ToolKindSkill   ToolKind = "skill"
	ToolKindCustom  ToolKind = "custom"
	ToolKindMCP     ToolKind = "mcp"
	ToolKindPackage ToolKind = "agent_skill"
)

type ToolStatus string

const (
	ToolStatusActive     ToolStatus = "active"
	ToolStatusHidden     ToolStatus = "hidden"
	ToolStatusDisabled   ToolStatus = "disabled"
	ToolStatusNeedsSetup ToolStatus = "needs_setup"
)

type ToolRouting struct {
	NativeAction string `json:"native_action,omitempty"`
	SkillName    string `json:"skill_name,omitempty"`
	CustomName   string `json:"custom_name,omitempty"`
	MCPServer    string `json:"mcp_server,omitempty"`
	MCPTool      string `json:"mcp_tool,omitempty"`
}

type ToolCatalogEntry struct {
	ReadOnly bool     `json:"read_only,omitempty"`
	Name     string   `json:"name"`
	Kind     ToolKind `json:"kind"`
	Aliases  []string `json:"aliases,omitempty"`
	// SearchTerms are the curated phrases (English and German) a user would
	// type to find the tool. They only feed the search ranking: unlike Aliases
	// they never resolve a tool name, and they stay out of the JSON so that
	// they cannot change a catalog revision or a discovery result.
	SearchTerms  []string    `json:"-"`
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

	searchMu sync.Mutex
	search   *toolSearchIndex // built at the first search, dropped by add
}

func BuildToolCatalog(allSchemas, activeSchemas []openai.Tool, promptsDir string) *ToolCatalog {
	return newToolCatalog(allSchemas, activeSchemas, promptsDir, true)
}

// newToolCatalog builds the catalog of the given schemas. With withDisabled it
// also adds the categorized tools that no schema backs as disabled entries;
// the adaptive selection ranks only what it can offer and leaves them out.
func newToolCatalog(allSchemas, activeSchemas []openai.Tool, promptsDir string, withDisabled bool) *ToolCatalog {
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

	if !withDisabled {
		return c
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
				SearchTerms:  toolSearchTermsFor(def.Name),
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
		SearchTerms: toolSearchTermsFor(name),
	}
	if active {
		entry.Status = ToolStatusActive
	} else {
		entry.HiddenReason = "adaptive_filter"
	}

	switch {
	case strings.HasPrefix(name, "skill__"):
		skillName := strings.TrimPrefix(name, "skill__")
		entry.Name = name
		entry.Kind = ToolKindSkill
		entry.Aliases = []string{skillName}
		entry.Category = "skills"
		entry.ManualPath = manualPathFor(promptsDir, skillName)
		entry.Routing = ToolRouting{SkillName: skillName}
	case strings.HasPrefix(name, "tool__"):
		toolName := strings.TrimPrefix(name, "tool__")
		entry.Name = name
		entry.Kind = ToolKindCustom
		entry.Aliases = []string{toolName}
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
	c.searchMu.Lock()
	c.search = nil
	c.searchMu.Unlock()
	for _, alias := range entry.Aliases {
		if alias != "" {
			if previous, exists := c.aliases[alias]; exists && previous != entry.Name {
				c.aliases[alias] = ""
			} else if !exists {
				c.aliases[alias] = entry.Name
			}
		}
	}
}

func (c *ToolCatalog) Get(name string) (*ToolCatalogEntry, bool) {
	if c == nil {
		return nil, false
	}
	name = strings.TrimSpace(name)
	if entry, ok := c.entries[name]; ok {
		return entry, true
	}
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

// Search returns the entries that match a query, best first; rankedSearch has
// the ranking rules.
func (c *ToolCatalog) Search(query string) []*ToolCatalogEntry {
	return hitEntries(c.rankedSearch(query))
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
	return filepath.Join(promptsDir, "tools_manuals", prompts.ToolManualID(name)+".md")
}

func callMethodForEntry(entry *ToolCatalogEntry) string {
	if entry == nil {
		return ""
	}
	if entry.Status == ToolStatusNeedsSetup {
		return "needs_setup"
	}
	if entry.Status == ToolStatusDisabled || !entry.Enabled {
		return "disabled"
	}
	switch entry.Kind {
	case ToolKindPackage:
		return "activate_agent_skill"
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

func toolCatalogOperations(entry *ToolCatalogEntry) []string {
	params := schemaParameters(entry.Schema)
	props, _ := params["properties"].(map[string]interface{})
	var result []string
	for _, key := range []string{"operation", "action", "mode", "sub_operation", "resource", "metric", "target"} {
		property, _ := props[key].(map[string]interface{})
		switch enum := property["enum"].(type) {
		case nil:
		case []string:
			result = append(result, enum...)
		case []interface{}:
			// An enum with a non-string value names no operations at all.
			names := make([]string, 0, len(enum))
			for _, value := range enum {
				if name, ok := value.(string); ok {
					names = append(names, name)
				}
			}
			if len(names) == len(enum) {
				result = append(result, names...)
			}
		default:
			data, _ := json.Marshal(enum)
			var names []string
			if json.Unmarshal(data, &names) == nil {
				result = append(result, names...)
			}
		}
	}
	return result
}
