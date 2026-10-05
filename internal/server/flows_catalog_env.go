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
//
// Locking. refreshMu is the outer lock and mu the inner one:
//   - refreshMu serialises refreshRegistry from the snapshot through the check to the
//     install, and guards regCfg. It is held across the schema build and across
//     flows.RefreshGenericTools, which is safe because ToolAvailability never takes it.
//   - mu guards the cache (namesCfg, names, generic). It is held only to read or swap
//     the cache, never across a call out of this type: schemas runs without it, so a
//     slow build never blocks a ToolAvailability that hits the cache. Two callers that
//     miss the cache at the same time may both build it (the agent caches the schemas
//     per tool flag set, so this is cheap); the one for the current configuration is
//     kept.
//
// ToolAvailability takes only mu, and only briefly. It is therefore safe to call from
// the availability hooks that flows.DescribeNodeTypes runs (every generic node calls it
// lazily), also while a refresh is in progress.
//
// The cached names and generic tools are shared by all callers and read-only. The
// schemas inside them are the agent's cached schema maps, also read-only.
type flowCatalogEnv struct {
	schemas func(cfg *config.Config) []openai.Tool
	current func() *config.Config

	refreshMu sync.Mutex
	regCfg    *config.Config // the configuration whose tools the registry holds; guarded by refreshMu

	mu       sync.Mutex
	namesCfg *config.Config
	names    map[string]bool
	generic  []flows.GenericTool
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

// snapshot returns the tool names and the generic tools of cfg, from the cache when it
// holds cfg. A new snapshot is built without a lock held and cached when cfg is the
// current configuration (or nothing is cached yet).
func (e *flowCatalogEnv) snapshot(cfg *config.Config) (map[string]bool, []flows.GenericTool) {
	if cfg == nil {
		return map[string]bool{}, nil
	}
	e.mu.Lock()
	if cfg == e.namesCfg && e.names != nil {
		names, generic := e.names, e.generic
		e.mu.Unlock()
		return names, generic
	}
	e.mu.Unlock()

	names := map[string]bool{}
	var generic []flows.GenericTool
	for _, tool := range e.schemas(cfg) {
		if tool.Function == nil || tool.Function.Name == "" {
			continue
		}
		name := tool.Function.Name
		names[name] = true
		if flowToolSpends(name) {
			continue
		}
		generic = append(generic, flows.GenericTool{Name: name, Description: tool.Function.Description,
			Category: flowToolCategory(name), Schema: flowSchemaMap(tool.Function.Parameters)})
	}

	current := e.current()
	e.mu.Lock()
	if cfg == current || e.names == nil {
		e.namesCfg, e.names, e.generic = cfg, names, generic
	}
	e.mu.Unlock()
	return names, generic
}

// refreshRegistry rebuilds the generic tool nodes when the configuration changed. It
// installs only the tools of the current configuration: when cfg was replaced while its
// snapshot was built, it installs nothing, and the refresh for the newer configuration
// (serialised after this one) installs its own. regCfg records what was installed.
func (e *flowCatalogEnv) refreshRegistry(reg *flows.Registry, cfg *config.Config) {
	if reg == nil || cfg == nil {
		return
	}
	e.refreshMu.Lock()
	defer e.refreshMu.Unlock()
	if e.regCfg == cfg {
		return
	}
	_, generic := e.snapshot(cfg)
	if cfg != e.current() {
		return
	}
	flows.RefreshGenericTools(reg, generic, e)
	e.regCfg = cfg
}

// flowSpendingTools are agent tools whose every call spends model tokens or money that
// the flow budget does not see; in phase 1 they never become generic flow nodes (the
// value says why). flowSpendingToolPrefixes do the same for tool families. Tools with
// cheap operations next to spending ones keep their node without the spending ones
// (genericDroppedOperations in internal/flows). The summary modes of web_scraper,
// ddg_search, wikipedia_search and pdf_extractor would spend as well, but flow tool calls
// always run with them off (flowToolInvoker.flowDispatchConfig).
//
// Accepted on purpose: tts and the bluetooth/chromecast speak operations (no model
// tokens; the agent's budget does not track them either).
//
// Finding spenders when tools change: the budgetTracker.Record* and IsBlocked call sites
// in internal/agent (vision, stt, image/music/video generation, yepapi, coagent), the
// callers of the summary and helper models (tools.SummariseContent and
// SummariseScrapedContent, resolveHelperBackedLLM, llm.ExecuteWithRetry and
// CreateChatCompletion outside the agent loop), and operations that start another
// agent: a co-agent, an invasion egg task (invasion_tasks send_task), the telephone
// agent (sip_phone dial; answer stays, because the tool answers only manual-route calls,
// which run without the agent pipeline) or a sidecar (space_agent).
var (
	flowSpendingTools = map[string]string{
		"analyze_image":    "a vision model call (budget category vision)",
		"manus":            "Manus tasks use Manus credits",
		"huggingface":      "the job_run operations run paid Hugging Face compute",
		"memory_reflect":   "the reflection is a model call",
		"space_agent":      "a sidecar agent that spends model tokens on every instruction",
		"treg_call":        "calls paid treg endpoints",
		"transcribe_audio": "speech-to-text (budget category stt)",
	}
	// The prefixes are fail-closed by design: a new tool of one of these families is no
	// generic node until someone decides otherwise (telnyx_manage, which only reads,
	// is left out for that reason).
	flowSpendingToolPrefixes = map[string]string{
		"generate_": "image, music and video generation (budget categories image_generation, music_generation, " +
			"video_generation); generate_image's enhance_prompt is a model call too",
		"yepapi_": "every YepAPI call is billed (budget category yepapi)",
		"telnyx_": "SMS and calls are billed by Telnyx; telnyx_manage only reads, but goes with its family",
	}
)

// flowToolSpends reports whether a tool spends outside the flow budget: a
// flowSpendingTools entry or a flowSpendingToolPrefixes family. The summary modes of
// web_scraper, ddg_search, wikipedia_search and pdf_extractor do not count: flow tool
// calls always run with them off (flowToolInvoker.flowDispatchConfig).
func flowToolSpends(name string) bool {
	if _, ok := flowSpendingTools[name]; ok {
		return true
	}
	for prefix := range flowSpendingToolPrefixes {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
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
