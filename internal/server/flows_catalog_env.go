package server

import (
	"encoding/json"
	"strings"
	"sync"

	"github.com/sashabaranov/go-openai"

	"aurago/internal/agent"
	"aurago/internal/config"
	"aurago/internal/flows"
)

// flowCatalogEnv answers tool availability for the node catalog and keeps the generic
// "tool.*" nodes in sync with the configured agent tools. Results are cached per
// configuration snapshot; saving the configuration swaps the pointer.
type flowCatalogEnv struct {
	schemas func(cfg *config.Config) []openai.Tool
	current func() *config.Config

	mu       sync.Mutex
	namesCfg *config.Config
	names    map[string]bool
	generic  []flows.GenericTool
	regCfg   *config.Config
}

func newFlowCatalogEnv(s *Server) *flowCatalogEnv {
	return &flowCatalogEnv{
		schemas: func(cfg *config.Config) []openai.Tool { return agent.ConfiguredToolSchemas(cfg, s.Logger) },
		current: s.ConfigSnapshot,
	}
}

// toolNames returns the names of the tools the agent may use with cfg.
func (e *flowCatalogEnv) toolNames(cfg *config.Config) map[string]bool {
	names, _ := e.snapshot(cfg)
	return names
}

func (e *flowCatalogEnv) snapshot(cfg *config.Config) (map[string]bool, []flows.GenericTool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if cfg == nil {
		return map[string]bool{}, nil
	}
	if cfg == e.namesCfg && e.names != nil {
		return e.names, e.generic
	}
	names := map[string]bool{}
	var generic []flows.GenericTool
	for _, tool := range e.schemas(cfg) {
		if tool.Function == nil || tool.Function.Name == "" {
			continue
		}
		name := tool.Function.Name
		names[name] = true
		generic = append(generic, flows.GenericTool{Name: name, Description: tool.Function.Description,
			Category: flowToolCategory(name), Schema: flowSchemaMap(tool.Function.Parameters)})
	}
	e.namesCfg, e.names, e.generic = cfg, names, generic
	return names, generic
}

// refreshRegistry rebuilds the generic tool nodes when the configuration changed.
func (e *flowCatalogEnv) refreshRegistry(reg *flows.Registry, cfg *config.Config) {
	if reg == nil || cfg == nil {
		return
	}
	_, generic := e.snapshot(cfg)
	e.mu.Lock()
	if e.regCfg == cfg {
		e.mu.Unlock()
		return
	}
	e.regCfg = cfg
	e.mu.Unlock()
	flows.RefreshGenericTools(reg, generic, e)
}

// ToolAvailability implements flows.CatalogEnv.
func (e *flowCatalogEnv) ToolAvailability(tool string) flows.Availability {
	cfg := e.current()
	return flowToolAvailability(cfg, e.toolNames(cfg), tool)
}

// flowToolAvailability decides whether a node's tool can run with cfg. names holds the
// configured agent tools. The logical names brave_search, pdf_extractor and
// document_creator:gotenberg have no schema of their own.
func flowToolAvailability(cfg *config.Config, names map[string]bool, tool string) flows.Availability {
	available := flows.Availability{State: flows.AvailableState}
	needsSetup := flows.Availability{State: flows.NeedsSetupState, Reason: "not_configured", ConfigSection: flowToolConfigSection(tool)}
	if cfg == nil {
		return needsSetup
	}
	switch tool {
	case flows.BraveSearchTool:
		if cfg.BraveSearch.Enabled && cfg.BraveSearch.APIKey != "" {
			return available
		}
		return needsSetup
	case flows.PDFExtractorTool:
		if cfg.Tools.PDFExtractor.Enabled && names["execute_skill"] {
			return available
		}
		return needsSetup
	case flows.GotenbergTool:
		if names["document_creator"] && strings.EqualFold(strings.TrimSpace(cfg.Tools.DocumentCreator.Backend), "gotenberg") {
			return available
		}
		return needsSetup
	}
	if names[tool] {
		return available
	}
	return needsSetup
}

// flowToolConfigSection points the editor's "Set up" link at a config UI section.
func flowToolConfigSection(tool string) string {
	switch tool {
	case flows.BraveSearchTool:
		return "brave_search"
	case flows.GotenbergTool, "document_creator":
		return "document_creator"
	case "send_telegram":
		return "telegram"
	case "send_email":
		return "email"
	case "send_discord":
		return "discord"
	case "send_notification":
		return "notifications"
	case "mqtt_publish":
		return "mqtt"
	case "home_assistant":
		return "home_assistant"
	case "web_scraper":
		return "web_scraper"
	default:
		return "tools"
	}
}

func flowToolCategory(name string) string {
	if c := agent.ToolCategoryForTool(name); c != "" {
		return c
	}
	return "other"
}

// flowSchemaMap turns a tool's JSON schema (map or raw JSON) into a map.
func flowSchemaMap(params any) map[string]any {
	if m, ok := params.(map[string]any); ok {
		return m
	}
	data, err := json.Marshal(params)
	if err != nil {
		return map[string]any{}
	}
	var out map[string]any
	if json.Unmarshal(data, &out) != nil || out == nil {
		return map[string]any{}
	}
	return out
}
