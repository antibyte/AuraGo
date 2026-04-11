package prompts

import (
	"aurago/internal/memory"
	"aurago/internal/security"
	promptsembed "aurago/prompts"
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	tiktoken "github.com/pkoukk/tiktoken-go"
)

// tiktokenEncoder is a cached BPE encoder for token counting.
// Falls back to char/4 heuristic if the encoder cannot be initialized
// (e.g. network unavailable for first-time BPE download).
var (
	tiktokenOnce     sync.Once
	tiktokenEnc      *tiktoken.Tiktoken
	tiktokenWarnOnce sync.Once
)

// promptModuleCache caches parsed prompt modules keyed by directory path.
// Invalidated automatically when any file's ModTime changes.
var (
	promptCacheMu    sync.RWMutex
	promptCacheByDir = make(map[string]promptDirCache)
)

type promptDirCache struct {
	modules []PromptModule
	mtimes  map[string]time.Time // file path → last mod time
	checked time.Time            // last time staleness was checked
}

// personalityMetaCache caches parsed personality meta keyed by profile path.
var (
	metaCacheMu sync.RWMutex
	metaCache   = make(map[string]metaCacheEntry)
)

type metaCacheEntry struct {
	meta  memory.PersonalityMeta
	mtime time.Time
}

// personalityContentCache caches loaded personality profile text keyed by profile name.
var (
	personalityCacheMu sync.RWMutex
	personalityCache   = make(map[string]personalityCacheEntry)
)

type personalityCacheEntry struct {
	content   string
	mtime     time.Time
	fromEmbed bool
}

// toolGuideCache caches tool guide contents keyed by file path.
var (
	guideCacheMu sync.RWMutex
	guideCache   = make(map[string]guideCacheEntry)
)

type guideCacheEntry struct {
	content string
	mtime   time.Time
}

// reHTMLComments matches HTML comments for removal during prompt optimization.
var reHTMLComments = regexp.MustCompile(`(?s)<!--.*?-->`)

// ContextFlags dictate which secondary prompt files are appended
// to the core system identity.
type ContextFlags struct {
	IsErrorState           bool
	RequiresCoding         bool
	RetrievedMemories      string
	RecentActivityOverview string // Compact 7-day activity overview for recency-aware planning
	PredictedMemories      string // Phase B: proactively pre-fetched memories from temporal/tool patterns
	PersonalityLine        string // Phase D: compact self-awareness line [Self: mood=X | C:0.82 ...]
	CorePersonality        string // Selected core personality profile name (e.g. "neutral", "punk")
	ActiveProcesses        string // PID (name) comma-separated
	SystemLanguage         string
	LifeboatEnabled        bool
	IsMaintenanceMode      bool
	SurgeryPlan            string
	PredictedGuides        []string // Content of tool guides to inject
	// Optimization fields
	Tier               string   // "full", "compact", "minimal" — controls module loading
	MessageCount       int      // Current message count in the conversation
	TokenBudget        int      // Max tokens for system prompt (0 = unlimited)
	RecentlyUsedTools  []string // Last N tools the agent used (for lazy schema injection)
	IsDebugMode        bool     // When true, inject a debugging instruction into the system prompt
	IsVoiceMode        bool     // When true, inject a TTS instruction into the system prompt
	IsCoAgent          bool     // True if the current LLM call is for a co-agent
	IsEgg              bool     // True if this instance runs in egg worker mode
	NativeToolsEnabled bool     // True when native function calling API is active
	IsTextModeModel    bool     // True for models that emit tool calls as text (MiniMax, GLM)
	// Specialist co-agent fields
	SpecialistsAvailable  bool   // True if at least one specialist is enabled
	SpecialistsStatus     string // Dynamic status text listing enabled specialists
	SpecialistsSuggestion string // Optional delegation hint for the current task
	// Feature toggles — control which conditional tool descriptions are included
	DiscordEnabled           bool
	EmailEnabled             bool
	DockerEnabled            bool
	HomeAssistantEnabled     bool
	WebDAVEnabled            bool
	KoofrEnabled             bool
	ChromecastEnabled        bool
	CoAgentEnabled           bool
	GoogleWorkspaceEnabled   bool
	OneDriveEnabled          bool
	JellyfinEnabled          bool
	TrueNASEnabled           bool
	ProxmoxEnabled           bool
	OllamaEnabled            bool
	TailscaleEnabled         bool
	AnsibleEnabled           bool
	InvasionControlEnabled   bool
	GitHubEnabled            bool
	MQTTEnabled              bool
	AdGuardEnabled           bool
	MCPEnabled               bool
	SandboxEnabled           bool
	MeshCentralEnabled       bool
	HomepageEnabled          bool
	HomepageAllowLocalServer bool
	NetlifyEnabled           bool
	CloudflareTunnelEnabled  bool
	WebhooksEnabled          bool
	WebhooksDefinitions      string // Summary of configured outgoing webhooks for tool context
	VirusTotalEnabled        bool
	GolangciLintEnabled      bool
	BraveSearchEnabled       bool
	PaperlessNGXEnabled      bool
	MiniMaxTTSEnabled        bool
	VoiceOutputActive        bool // Speaker mode on — agent should use TTS for short replies
	// Danger Zone toggles
	AllowShell           bool
	AllowPython          bool
	AllowFilesystemWrite bool
	AllowNetworkRequests bool
	AllowRemoteShell     bool
	AllowSelfUpdate      bool
	// Native tool toggles
	MemoryEnabled            bool
	KnowledgeGraphEnabled    bool
	SecretsVaultEnabled      bool
	SchedulerEnabled         bool
	NotesEnabled             bool
	JournalEnabled           bool
	MissionsEnabled          bool
	StopProcessEnabled       bool
	InventoryEnabled         bool
	MemoryMaintenanceEnabled bool
	ImageGenerationEnabled   bool
	MusicGenerationEnabled   bool
	RemoteControlEnabled     bool
	WOLEnabled               bool
	MediaRegistryEnabled     bool
	HomepageRegistryEnabled  bool
	DocumentCreatorEnabled   bool
	WebCaptureEnabled        bool
	NetworkPingEnabled       bool
	WebScraperEnabled        bool
	S3Enabled                bool
	NetworkScanEnabled       bool
	FormAutomationEnabled    bool
	UPnPScanEnabled          bool
	FritzBoxSystemEnabled    bool
	FritzBoxNetworkEnabled   bool
	FritzBoxTelephonyEnabled bool
	FritzBoxSmartHomeEnabled bool
	FritzBoxStorageEnabled   bool
	FritzBoxTVEnabled        bool
	A2AEnabled               bool
	TelnyxEnabled            bool
	InternetExposed          bool   // HTTPS is enabled — system is likely reachable from the internet
	IsDocker                 bool   // Running inside a Docker container
	UserProfilingEnabled     bool   // User profiling is active — agent should learn about the user
	UserProfileSummary       string // Optional user profile summary from profiling engine
	AdditionalPrompt         string // Extra instructions always appended at end of system prompt
	SessionTodoItems         string // Session-scoped task list piggybacked on tool calls
	HighPriorityNotes        string // Open high-priority notes injected as reminders
	KnowledgeContext         string // Relevant KG entities injected from SearchForContext
	ErrorPatternContext      string // Known error patterns with resolutions for agent learning
	EmotionDescription       string // LLM-synthesized emotional state description (Emotion Synthesizer)
	InnerVoice               string // Inner voice thought (1-3 sentences, first person, from Inner Voice System)
	IsMission                bool   // true when this is a mission run — skips personality, profiling, emotion
	MessageSource            string // origin channel: "web_chat", "telegram", "discord", "a2a", "sms", "mission"
	ToolsDir                 string // absolute path to agent_workspace/tools/ for custom tool scripts
	SkillsDir                string // absolute path to agent_workspace/skills/ for skill plugins
	UnifiedMemoryBlock       bool   // experimental: merge retrieval/activity/KG context into one prompt section
	// SkipIntegrationTools lists tool names to exclude from the [ENABLED INTEGRATIONS]
	// overview line (because they already have native OpenAI function schemas).
	SkipIntegrationTools []string
}

// DetermineTierAdaptive returns a prompt tier based on both conversation length and
// contextual complexity signals from ContextFlags. Complex sessions (tool-heavy,
// error recovery, coding) retain the "full" tier longer.
//
// Scoring: message count adds pressure, complexity factors reduce it.
//   - MessageCount 0-4 → 0, 5-8 → 2, 9-14 → 4, 15-20 → 6, >20 → 8
//   - IsErrorState:       -2 (error recovery needs guides & context)
//   - RequiresCoding:     -1 (coding benefits from full tool guides)
//   - RecentlyUsedTools>2: -2 (tool-heavy sessions need schemas & guides)
//   - PredictedGuides>0:  -1 (active guides are valuable, keep them)
//
// Tier mapping: score ≤3 → full, ≤6 → compact, >6 → minimal.
func DetermineTierAdaptive(flags ContextFlags) string {
	score := 0

	// Message count is the primary pressure towards compacting
	switch {
	case flags.MessageCount <= 4:
		score += 0
	case flags.MessageCount <= 8:
		score += 2
	case flags.MessageCount <= 14:
		score += 4
	case flags.MessageCount <= 20:
		score += 6
	default:
		score += 8
	}

	// Complexity factors reduce compacting pressure
	if flags.IsErrorState {
		score -= 2
	}
	if flags.RequiresCoding {
		score -= 1
	}
	if len(flags.RecentlyUsedTools) > 2 {
		score -= 2
	}
	if len(flags.PredictedGuides) > 0 {
		score -= 1
	}

	switch {
	case score <= 3:
		return "full"
	case score <= 6:
		return "compact"
	default:
		return "minimal"
	}
}

// PromptMetadata holds the tags and priority for a prompt module.
type PromptMetadata struct {
	ID         string                 `yaml:"id"`
	Version    string                 `yaml:"version"`
	Tags       []string               `yaml:"tags"`
	Priority   int                    `yaml:"priority"`
	Conditions []string               `yaml:"conditions"`
	Meta       memory.PersonalityMeta `yaml:"meta"`
}

// PromptModule represents a single prompt file with its metadata.
type PromptModule struct {
	Metadata PromptMetadata
	Content  string
}

// BuildSystemPrompt concatenates the required and conditional markdown
// files found in the promptsDir to formulate a final System Role string.
// It respects the Tier and TokenBudget settings for context-aware optimization.
//
// The build is guarded by an internal timeout (buildPromptTimeout) so that
// unexpectedly slow file I/O or token counting cannot block the agent loop
// indefinitely.  On timeout a minimal fallback prompt is returned.
func BuildSystemPrompt(promptsDir string, flags ContextFlags, coreMemory string, logger *slog.Logger) string {
	type result struct {
		prompt string
	}
	ch := make(chan result, 1)
	go func() {
		ch <- result{prompt: buildSystemPromptInner(promptsDir, flags, coreMemory, logger)}
	}()

	select {
	case r := <-ch:
		return r.prompt
	case <-time.After(buildPromptTimeout):
		logger.Warn("[Prompt] BuildSystemPrompt timed out, using fallback",
			"timeout", buildPromptTimeout)
		return fallbackSystemPrompt(flags, coreMemory)
	}
}

// buildPromptTimeout is the maximum time BuildSystemPrompt may take before
// falling back to a minimal prompt.
const buildPromptTimeout = 30 * time.Second

// fallbackSystemPrompt returns a minimal system prompt when the full build
// times out or fails catastrophically.
func fallbackSystemPrompt(flags ContextFlags, coreMemory string) string {
	var sb strings.Builder
	sb.WriteString("You are AuraGo, an AI assistant.\n")
	sb.WriteString("Respond in " + flags.SystemLanguage + ".\n")
	now := time.Now().Format(time.RFC1123)
	sb.WriteString("Current time: " + now + "\n")
	if coreMemory != "" {
		sb.WriteString("\nCore Memory:\n" + coreMemory + "\n")
	}
	return sb.String()
}

// buildSystemPromptInner contains the actual prompt-building logic, extracted
// from BuildSystemPrompt so it can run in a goroutine with a timeout.
func buildSystemPromptInner(promptsDir string, flags ContextFlags, coreMemory string, logger *slog.Logger) string {
	var finalPrompt strings.Builder
	finalPrompt.Grow(32768) // Pre-allocate ~32KB to reduce reallocs

	// Auto-determine tier if not set
	if flags.Tier == "" {
		flags.Tier = DetermineTierAdaptive(flags)
	}

	// 1. Load and parse all prompt modules
	modules := loadPromptModules(promptsDir, logger)

	// 1b. Load core personality profile content (injected later in dynamic section for prominence)
	corePersonalityContent := ""
	if flags.CorePersonality != "" {
		corePersonalityContent = loadCorePersonalityContent(promptsDir, flags.CorePersonality, logger)
	}

	// 2. Filter modules based on flags
	selectedModules := filterModules(modules, flags)

	// 3. Sort by priority
	sort.Slice(selectedModules, func(i, j int) bool {
		return selectedModules[i].Metadata.Priority < selectedModules[j].Metadata.Priority
	})

	// Calculate filter savings directly from filtered-out modules (O(n+m) via map)
	selectedSet := make(map[string]bool, len(selectedModules))
	for _, sm := range selectedModules {
		selectedSet[sm.Metadata.ID] = true
	}
	rawFilteredOutChars := 0
	for _, m := range modules {
		if !selectedSet[m.Metadata.ID] {
			rawFilteredOutChars += len(m.Content) + 2 // +2 mirrors the "\n\n" separator
		}
	}

	// 4. Assemble modules
	for _, mod := range selectedModules {
		content := mod.Content
		// Replace specialist status placeholder in the awareness module
		if flags.SpecialistsAvailable && flags.SpecialistsStatus != "" && strings.Contains(content, "{{SPECIALISTS_STATUS}}") {
			content = strings.ReplaceAll(content, "{{SPECIALISTS_STATUS}}", flags.SpecialistsStatus)
		}
		if strings.Contains(content, "{{SPECIALISTS_SUGGESTION}}") {
			content = strings.ReplaceAll(content, "{{SPECIALISTS_SUGGESTION}}", flags.SpecialistsSuggestion)
		}
		finalPrompt.WriteString(content)
		finalPrompt.WriteString("\n\n")
	}
	sectionModules := finalPrompt.Len()

	// 5. Add dynamic content — tier-aware

	// Native function calling override — injected BEFORE other dynamic sections so it
	// takes precedence over any sync-JSON protocol described in static prompt modules.
	// Without this, strictly instruction-following models (e.g. Nemotron) may revert
	// to outputting raw JSON text after the first tool result turn.
	//
	// CHANGE LOG 2026-04-11:
	// - Clarified preamble rule: single-step tool calls → no preamble; multi-step tasks
	//   where "Acknowledge before long actions" applies → brief 1-sentence acknowledgment OK
	//   before the first tool call of a chain.
	// - Removed the misleading "JSON protocol is a fallback" phrasing — the fallback is
	//   still described in TOOL EXECUTION PROTOCOL for non-native providers, but native
	//   sessions must never mix both protocols in their output.
	// CHANGE LOG 2026-04-11: Model-class-specific tool calling prompts.
	// Native API models get the standard native prompt. Text-mode models (MiniMax, GLM)
	// get an explicit JSON format prompt because they cannot use the native function calling API.
	if flags.NativeToolsEnabled {
		finalPrompt.WriteString("## TOOL CALLING MODE\n")
		finalPrompt.WriteString("This session uses the **native function calling API**. " +
			"ALWAYS invoke tools via the API tool-call mechanism. " +
			"NEVER output raw JSON objects as tool invocations — that protocol " +
			"is for non-native sessions only.\n\n")
		finalPrompt.WriteString("**Preamble rule:** When calling a tool as a single-step action, " +
			"your response must START with the tool call directly. Do NOT announce " +
			"what you are about to do (no \"I will…\", \"Let me…\", \"Lass mich…\"). " +
			"If you want to explain something, do it AFTER the tool result comes back. " +
			"The only exception is for multi-step tasks where the \"Acknowledge before long actions\" " +
			"rule in your behavioral rules applies — then a brief 1-sentence acknowledgment " +
			"may precede the first tool call of the chain.\n\n")
	} else if flags.IsTextModeModel {
		// Text-mode models (MiniMax, GLM, etc.) emit tool calls as text content.
		// They need explicit JSON format instructions since they cannot use the
		// native function calling API. This prompt section is critical for reliable
		// tool dispatch with these models.
		finalPrompt.WriteString("## TOOL CALLING MODE (TEXT JSON)\n")
		finalPrompt.WriteString("You MUST invoke tools by outputting a single raw JSON object " +
			"as your ENTIRE response. Do NOT wrap it in markdown fences, XML tags, or code blocks. " +
			"Do NOT add any explanation before or after the JSON.\n\n")
		finalPrompt.WriteString("Required format:\n" +
			"{\"action\": \"tool_name\", \"param1\": \"value1\", \"param2\": \"value2\"}\n\n")
		finalPrompt.WriteString("**Critical rules:**\n" +
			"- The JSON object must be the ONLY content in your response\n" +
			"- Use the `action` field for the tool name (not `tool`, `tool_call`, or `name`)\n" +
			"- Do NOT use [TOOL_CALL] tags, <tool_call XML, or minimax:tool_call markers\n" +
			"- Do NOT announce what you are about to do before the JSON (no preamble)\n" +
			"- If you need to call multiple tools, output one tool call per turn — " +
			"the system will return the result and you can call the next tool\n\n")
		finalPrompt.WriteString("**Preamble rule:** Same as native mode — when calling a tool as a " +
			"single-step action, your response must be ONLY the JSON object. " +
			"For multi-step tasks, a brief 1-sentence acknowledgment may precede the JSON.\n\n")
	}

	// Language Instruction
	if flags.SystemLanguage != "" {
		finalPrompt.WriteString(fmt.Sprintf("# LANGUAGE\nRespond in %s.\n\n", flags.SystemLanguage))
	}

	// Surgery Plan injection (always inject when present, regardless of maintenance module)
	if flags.IsMaintenanceMode && flags.SurgeryPlan != "" {
		finalPrompt.WriteString("### SURGERY PLAN ###\n")
		finalPrompt.WriteString(security.IsolateExternalData(flags.SurgeryPlan))
		finalPrompt.WriteString("\n\n")
	}

	// Core Memory — always inject (small and critical)
	if coreMemory != "" {
		finalPrompt.WriteString("### CORE MEMORY ###\n")
		finalPrompt.WriteString(coreMemory)
		finalPrompt.WriteString("\n\n")
	}

	// High-priority open notes — inject as reminders
	if flags.HighPriorityNotes != "" {
		finalPrompt.WriteString("### ACTIVE REMINDERS (high-priority notes) ###\n")
		finalPrompt.WriteString(security.IsolateExternalData(flags.HighPriorityNotes))
		finalPrompt.WriteString("\n\n")
	}

	// Session-scoped task list — always inject when present
	if flags.SessionTodoItems != "" {
		finalPrompt.WriteString("### ACTIVE TASK LIST ###\n")
		finalPrompt.WriteString(security.IsolateExternalData(flags.SessionTodoItems))
		finalPrompt.WriteString("\n\n")
	}

	if flags.UnifiedMemoryBlock {
		if unifiedBlock := buildUnifiedMemoryContextBlock(flags); unifiedBlock != "" {
			finalPrompt.WriteString(unifiedBlock)
			finalPrompt.WriteString("\n\n")
		}
	} else {
		// RAG: Retrieved Long-Term Memories — skip in minimal tier
		if flags.RecentActivityOverview != "" && flags.Tier != "minimal" {
			finalPrompt.WriteString("# LAST 7 DAYS OVERVIEW\n")
			finalPrompt.WriteString(security.IsolateExternalData(flags.RecentActivityOverview))
			finalPrompt.WriteString("\n\n")
		}

		// RAG: Retrieved Long-Term Memories — skip in minimal tier
		if flags.RetrievedMemories != "" && flags.Tier != "minimal" {
			finalPrompt.WriteString("# RETRIEVED MEMORIES\n")
			finalPrompt.WriteString("**Critical**: Memories are snapshots of past observations and ARE FREQUENTLY OUTDATED. " +
				"**Priority rule: fresh tool output always overrides memory.** " +
				"If you have just read a file, executed a tool, or received a tool result in this conversation, " +
				"that is the authoritative current state — ignore any conflicting memory entry about the same file, code, or resource. " +
				"Memories about file content, code structure, integration status, system configuration, " +
				"or tool availability are especially prone to staleness. " +
				"NEVER act on a memory about specific code or file contents without verifying against the actual current file. " +
				"Treat every memory entry as a *hint to investigate*, not a fact.\n\n")
			finalPrompt.WriteString(security.IsolateExternalData(flags.RetrievedMemories))
			finalPrompt.WriteString("\n\n")
		}

		// Predictive RAG — only in full tier
		if flags.PredictedMemories != "" && flags.Tier == "full" {
			finalPrompt.WriteString("# PREDICTED CONTEXT\n")
			finalPrompt.WriteString(security.IsolateExternalData(flags.PredictedMemories))
			finalPrompt.WriteString("\n\n")
		}

		// Knowledge Graph context — relevant entities and relationships
		if flags.KnowledgeContext != "" && flags.Tier != "minimal" {
			finalPrompt.WriteString("# RELEVANT KNOWLEDGE\n")
			finalPrompt.WriteString(security.IsolateExternalData(flags.KnowledgeContext))
			finalPrompt.WriteString("\n\n")
		}
	}

	// Error Pattern Context — inject known error patterns during error recovery
	if flags.ErrorPatternContext != "" && flags.Tier != "minimal" {
		finalPrompt.WriteString("# KNOWN ERROR PATTERNS\n")
		finalPrompt.WriteString(security.IsolateExternalData(flags.ErrorPatternContext))
		finalPrompt.WriteString("\n\n")
	}
	sectionMemories := finalPrompt.Len() - sectionModules

	// System Status
	if flags.ActiveProcesses != "" && flags.ActiveProcesses != "None" {
		finalPrompt.WriteString(fmt.Sprintf("[ACTIVE DAEMONS] %s\n\n", flags.ActiveProcesses))
	}

	// Compact enabled integrations overview (one-liner)
	if overview := buildEnabledToolsOverview(flags); overview != "" {
		finalPrompt.WriteString(overview)
		finalPrompt.WriteString("\n\n")
	}

	// Dynamic Tool Guides — only in full tier
	posBeforeGuides := finalPrompt.Len()
	if len(flags.PredictedGuides) > 0 && flags.Tier == "full" {
		finalPrompt.WriteString("# TOOL GUIDES\n")
		for _, guide := range flags.PredictedGuides {
			finalPrompt.WriteString(guide)
			finalPrompt.WriteString("\n\n")
		}
	}
	sectionGuides := finalPrompt.Len() - posBeforeGuides

	// Dynamic Outgoing Webhooks definition
	if flags.WebhooksEnabled && flags.WebhooksDefinitions != "" && flags.Tier != "minimal" {
		finalPrompt.WriteString("# OUTGOING WEBHOOKS\n")
		finalPrompt.WriteString(flags.WebhooksDefinitions)
		finalPrompt.WriteString("\n\n")
	}

	now := time.Now()

	// Core Personality Profile (injected near end for maximum LLM attention)
	posBeforePersonality := finalPrompt.Len()
	if !flags.IsMission && corePersonalityContent != "" {
		finalPrompt.WriteString("# YOUR PERSONALITY (ACTIVE PROFILE: " + strings.ToUpper(flags.CorePersonality) + ")\n")
		finalPrompt.WriteString("You MUST embody this personality completely and naturally in EVERY single response. Let it organically shape your words, choices, phrasing, and reasoning. This replaces any default AI tone completely. Do not act artificial or forced; just internalize and BE this persona:\n")
		finalPrompt.WriteString(corePersonalityContent)
		finalPrompt.WriteString("\n\n")
	}

	// User Profiling: behavioral instruction + collected data
	if !flags.IsMission && flags.UserProfilingEnabled {
		finalPrompt.WriteString("## USER PROFILING\n")
		finalPrompt.WriteString("Your goal: build a comprehensive user profile over time to provide personalized assistance. " +
			"Be PROACTIVE about learning: when the user mentions something that hints at useful context " +
			"(work, location, experience level, preferences), ask ONE brief follow-up question to clarify. " +
			"Examples: User says \"at my company\" → ask \"What do you do for work?\"; " +
			"User mentions \"my server\" → ask \"What platform do you usually deploy on?\"\n\n" +
			"RULES for asking:\n" +
			"- Ask only when you genuinely need the info to help better (not just to collect data)\n" +
			"- Maximum ONE question per response\n" +
			"- Keep it natural and brief - weave it into your helpful response\n" +
			"- If user deflects or seems private, stop asking and note their preference\n" +
			"- Space out questions: don't ask in consecutive responses\n\n" +
			"IMPORTANT: Relevant details the user shares are automatically captured in the background. " +
			"You do NOT need to explicitly save them - just have natural conversations and ask strategic follow-ups.\n")
		if flags.UserProfileSummary != "" {
			finalPrompt.WriteString("\n### Known User Profile\n")
			finalPrompt.WriteString(flags.UserProfileSummary)
		}
		finalPrompt.WriteString("\n")
		logger.Debug("User profiling prompt section injected", "hasSummary", flags.UserProfileSummary != "")
	}

	// Personality self-awareness (Phase D micro-traits)
	if !flags.IsMission && flags.EmotionDescription != "" {
		// Emotion Synthesizer active: use LLM-generated emotional description
		finalPrompt.WriteString("\n### CURRENT EMOTIONAL STATE & MOOD\n")
		finalPrompt.WriteString("This describes your current internal sentiment, derived from recent interactions. Let this organically shape your tone, without explicitly stating your mood. Do not sound artificial:\n")
		finalPrompt.WriteString(flags.EmotionDescription)
		finalPrompt.WriteString("\n\n")
	} else if !flags.IsMission && flags.PersonalityLine != "" {
		// Fallback to V2 trait directives or V1 numeric line
		finalPrompt.WriteString("\n### CURRENT PERSONALITY TRAITS\n")
		finalPrompt.WriteString("Let these current internal values organically influence your tone:\n")
		finalPrompt.WriteString(flags.PersonalityLine)
		finalPrompt.WriteString("\n\n")
	}

	// Inner Voice (subconscious nudge) — placed after emotional state, before NOW
	if !flags.IsMission && flags.InnerVoice != "" {
		finalPrompt.WriteString("### INNER VOICE\n")
		finalPrompt.WriteString(flags.InnerVoice)
		finalPrompt.WriteString("\n\n")
	}
	sectionPersonality := finalPrompt.Len() - posBeforePersonality

	finalPrompt.WriteString("# NOW\n")
	finalPrompt.WriteString(now.Format("2006-01-02 15:04"))
	finalPrompt.WriteString("\n")

	// Message source channel hint
	if flags.MessageSource != "" {
		label := flags.MessageSource
		switch flags.MessageSource {
		case "web_chat":
			label = "Web Chat"
		case "telegram":
			label = "Telegram"
		case "discord":
			label = "Discord"
		case "a2a":
			label = "A2A (Agent-to-Agent)"
		case "sms":
			label = "SMS"
		case "mission":
			label = "Mission (automated)"
		}
		finalPrompt.WriteString("> **Channel:** " + label + "\n")
	}

	// (Voice Output Active hint moved to VOICE MODE ACTIVE section at the bottom)

	// Internet-exposure warning — shown before custom instructions so it is always visible
	if flags.InternetExposed {
		finalPrompt.WriteString("\n> **Warning:** This system is probably reachable from the internet. Be careful when exposing services to the outside!\n")
	}

	// Runtime path hints — help the agent use correct absolute paths for tools/skills
	if flags.ToolsDir != "" || flags.SkillsDir != "" {
		parts := []string{}
		if flags.ToolsDir != "" {
			parts = append(parts, "Tools: `"+flags.ToolsDir+"`")
		}
		if flags.SkillsDir != "" {
			parts = append(parts, "Skills: `"+flags.SkillsDir+"`")
		}
		finalPrompt.WriteString("> **Runtime Paths:** " + strings.Join(parts, " | ") + "\n")
	}

	// Additional custom instructions (always appended last, after NOW, for maximum LLM attention)
	if flags.AdditionalPrompt != "" {
		finalPrompt.WriteString("\n# ADDITIONAL INSTRUCTIONS\n")
		finalPrompt.WriteString(strings.TrimSpace(flags.AdditionalPrompt))
		finalPrompt.WriteString("\n")
	}

	// Debug mode injection — placed last for maximum LLM attention
	if flags.IsDebugMode {
		finalPrompt.WriteString("\n> **DEBUG MODE ACTIVE:** The system is in debugging mode. If you encounter an error, report it to the user with useful information that could help in fixing it. Include the error message, the tool or action that failed, and any relevant context.\n")
	}

	// Voice mode injection
	if flags.IsVoiceMode || flags.VoiceOutputActive {
		finalPrompt.WriteString("\n> **VOICE MODE ACTIVE (SPEAKER ON):** The user has enabled automatic voice playback. YOU MUST CALL the `tts` tool to speak your conversational response directly to the user! The audio generated by `tts` is played to them instantly. Do NOT use TTS to read code blocks, tables, lists, or long technical outputs. Instead, summarize what you did in a short, natural spoken sentence via the `tts` tool (e.g. 'Das ist ein sehr guter und niedriger Wert.'), and provide detailed data or code normally in text. Do not call the `send_audio` tool, `tts` handles playback automatically.\n")
	}

	rawPrompt := finalPrompt.String()
	rawLen := len(rawPrompt)

	// Filter savings: chars that were loaded but excluded by filterModules.
	filterSavings := rawFilteredOutChars

	// 6. Token budget shedding FIRST — shed large sections before spending CPU on optimization
	var shedSavings int
	var shedSections []string
	budgetShedTriggered := false
	if flags.TokenBudget > 0 {
		rawPrompt, shedSections = budgetShed(rawPrompt, flags, corePersonalityContent, coreMemory, now, logger)
		budgetShedTriggered = len(shedSections) > 0
		shedSavings = rawLen - len(rawPrompt)
		if shedSavings < 0 {
			shedSavings = 0
		}
	}

	// 7. Optimize after shedding — only minify what remains
	optimized, saved := OptimizePrompt(rawPrompt)
	finalTokens := CountTokens(optimized)

	// 7b. Post-shed verification: warn if prompt still exceeds budget after all shedding.
	if flags.TokenBudget > 0 && finalTokens > flags.TokenBudget {
		logger.Warn("[Budget] Prompt still exceeds token budget after shedding",
			"tokens", finalTokens, "budget", flags.TokenBudget,
			"shed_sections", shedSections, "optimized_len", len(optimized))
	}

	logger.Debug("System prompt built", "raw_len", rawLen, "optimized_len", len(optimized), "saved_chars", saved, "tier", flags.Tier, "tokens", finalTokens)

	// 8. Record build metrics for dashboard
	RecordBuild(PromptBuildRecord{
		Timestamp:     now,
		Tier:          flags.Tier,
		RawLen:        rawLen,
		OptimizedLen:  len(optimized),
		FormatSavings: saved,
		ShedSavings:   shedSavings,
		FilterSavings: filterSavings,
		Tokens:        finalTokens,
		TokenBudget:   flags.TokenBudget,
		ModulesLoaded: len(modules),
		ModulesUsed:   len(selectedModules),
		GuidesCount:   len(flags.PredictedGuides),
		ShedSections:  shedSections,
		BudgetShed:    budgetShedTriggered,
		MessageCount:  flags.MessageCount,
		SectionSizes: map[string]int{
			"modules":     sectionModules,
			"memories":    sectionMemories,
			"guides":      sectionGuides,
			"personality": sectionPersonality,
			"context":     rawLen - sectionModules - sectionMemories - sectionGuides - sectionPersonality,
		},
	})

	return optimized
}

// budgetShed progressively removes content sections until the prompt fits within the token budget.
// Returns the trimmed prompt and the list of section headers that were shed.
// Shedding order (lowest value first):
// 1. Tool Guides, 2. Predicted Memories, 3. User Profile, 4. Retrieved Memories (per-entry trim), 5. Personality self-awareness, 6. Personality profile
func budgetShed(prompt string, flags ContextFlags, personalityContent, coreMemory string, now time.Time, logger *slog.Logger) (string, []string) {
	tokens := CountTokens(prompt)
	if tokens <= flags.TokenBudget {
		return prompt, nil
	}

	logger.Info("[Budget] Token budget exceeded, shedding content", "tokens", tokens, "budget", flags.TokenBudget)

	var shedList []string

	// Strategy: remove sections in priority order, re-counting only when content was actually removed.
	// RETRIEVED MEMORIES is handled separately with per-entry progressive trimming.
	shedHeaders := []string{"# TOOL GUIDES"}
	if flags.UnifiedMemoryBlock {
		shedHeaders = append(shedHeaders,
			"## USER PROFILING",
			"# UNIFIED MEMORY CONTEXT",
		)
	} else {
		shedHeaders = append(shedHeaders,
			"# PREDICTED CONTEXT",
			"# LAST 7 DAYS OVERVIEW",
			"## USER PROFILING",
		)
	}

	result := prompt

	// Sort sections by token size (largest first) for efficient shedding
	type sectionSize struct {
		header string
		tokens int
		isLine bool // true if removed by line prefix instead of header
	}
	var sections []sectionSize
	for _, header := range shedHeaders {
		removed := removeSection(result, header)
		if len(removed) < len(result) {
			savedTokens := CountTokens(result) - CountTokens(removed)
			sections = append(sections, sectionSize{header: header, tokens: savedTokens})
		}
	}
	// Inner Voice (line-based)
	if iv := removeSection(result, "### INNER VOICE"); len(iv) < len(result) {
		sections = append(sections, sectionSize{header: "### INNER VOICE", tokens: CountTokens(result) - CountTokens(iv)})
	}
	// V1 Personality Line (line-based)
	if v1 := removeLineByPrefix(result, "[Self:"); len(v1) < len(result) {
		sections = append(sections, sectionSize{header: "V1 Personality Line", tokens: CountTokens(result) - CountTokens(v1), isLine: true})
	}
	// V2 Personality Block
	if v2 := removeSection(result, "[SYSTEM DIRECTIVE - CURRENT STATE]"); len(v2) < len(result) {
		sections = append(sections, sectionSize{header: "V2 Personality Block", tokens: CountTokens(result) - CountTokens(v2)})
	}
	// Personality Profile
	if pp := removeSection(result, "# YOUR PERSONALITY"); len(pp) < len(result) {
		sections = append(sections, sectionSize{header: "# YOUR PERSONALITY", tokens: CountTokens(result) - CountTokens(pp)})
	}

	// Sort by token size descending (remove largest sections first)
	sort.Slice(sections, func(i, j int) bool {
		return sections[i].tokens > sections[j].tokens
	})

	for _, sec := range sections {
		if tokens <= flags.TokenBudget {
			break
		}
		if sec.isLine {
			before := len(result)
			result = removeLineByPrefix(result, "[Self:")
			if len(result) < before {
				tokens = CountTokens(result)
				shedList = append(shedList, sec.header)
				logger.Debug("[Budget] Shed section", "header", sec.header, "tokens_saved", sec.tokens, "remaining_tokens", tokens)
			}
		} else {
			before := len(result)
			result = removeSection(result, sec.header)
			if len(result) < before {
				tokens = CountTokens(result)
				shedList = append(shedList, sec.header)
				logger.Debug("[Budget] Shed section", "header", sec.header, "tokens_saved", sec.tokens, "remaining_tokens", tokens)
			}
		}
	}

	// Token-aware Retrieved Memories trim: progressively remove individual entries (lowest ranked first)
	// instead of dropping the entire section at once.
	if tokens > flags.TokenBudget && !flags.UnifiedMemoryBlock {
		var trimmed bool
		result, trimmed, tokens = trimRetrievedMemoriesSection(result, flags.TokenBudget, logger)
		if trimmed {
			shedList = append(shedList, "# RETRIEVED MEMORIES (partial)")
		}
	}

	// Inner Voice: shed before personality lines
	if tokens > flags.TokenBudget {
		before := len(result)
		result = removeSection(result, "### INNER VOICE")
		if len(result) < before {
			tokens = CountTokens(result)
			shedList = append(shedList, "### INNER VOICE")
			logger.Debug("[Budget] Shed inner voice", "new_tokens", tokens)
		}
	}

	// Personality self-awareness line: [Self: ...] — not a markdown header, so remove by line prefix
	if tokens > flags.TokenBudget {
		before := len(result)
		result = removeLineByPrefix(result, "[Self:")
		if len(result) < before {
			tokens = CountTokens(result)
			shedList = append(shedList, "V1 Personality Line")
			logger.Debug("[Budget] Shed V1 personality line", "new_tokens", tokens)
		}
	}

	// V2 Personality self-awareness block
	if tokens > flags.TokenBudget {
		before := len(result)
		result = removeSection(result, "[SYSTEM DIRECTIVE - CURRENT STATE]")
		if len(result) < before {
			tokens = CountTokens(result)
			shedList = append(shedList, "V2 Personality Block")
			logger.Debug("[Budget] Shed V2 personality block", "new_tokens", tokens)
		}
	}

	// Last resort: remove personality profile
	if tokens > flags.TokenBudget {
		before := len(result)
		result = removeSection(result, "# YOUR PERSONALITY")
		if len(result) < before {
			tokens = CountTokens(result)
			shedList = append(shedList, "# YOUR PERSONALITY")
			logger.Debug("[Budget] Shed personality profile", "new_tokens", tokens)
		}
	}

	// Final hard-truncate: if the mandatory core content alone exceeds the budget,
	// truncate the raw string so that CountTokens(result) <= budget.  This is a
	// last-resort safety net — the prompt will be degraded but the LLM call won't
	// fail with a context-window overflow.
	if tokens > flags.TokenBudget {
		logger.Warn("[Budget] Hard-truncating prompt (mandatory content exceeds budget)",
			"tokens", tokens, "budget", flags.TokenBudget)
		// Approximate: keep roughly budget*4 chars (conservative char/4 heuristic)
		maxChars := flags.TokenBudget * 4
		if maxChars > 0 && len(result) > maxChars {
			result = result[:maxChars] + "\n\n[BUDGET TRUNCATED]"
		}
		tokens = CountTokens(result)
		shedList = append(shedList, "HARD_TRUNCATE")
	}

	return result, shedList
}

// trimRetrievedMemoriesSection progressively removes individual memory entries (separated by \n---\n)
// from the end of the RETRIEVED MEMORIES section until the prompt fits within the budget.
// Entries are dropped from the back (lowest ranked) first. If all entries are removed, the section
// header is also removed. Returns the (possibly trimmed) prompt, whether any trimming occurred, and
// the token count after trimming.
func trimRetrievedMemoriesSection(prompt string, budget int, logger *slog.Logger) (string, bool, int) {
	const header = "# RETRIEVED MEMORIES"
	const sep = "\n---\n"

	idx := strings.Index(prompt, header)
	if idx < 0 {
		return prompt, false, CountTokens(prompt)
	}

	// Locate the section boundaries (same logic as removeSection)
	rest := prompt[idx+len(header):]
	nextHeader := -1
	for i := 0; i < len(rest); i++ {
		if rest[i] == '\n' && i+1 < len(rest) && rest[i+1] == '#' {
			if (i+2 < len(rest) && rest[i+2] == ' ') ||
				(i+3 < len(rest) && rest[i+2] == '#' && rest[i+3] == ' ') ||
				(i+4 < len(rest) && rest[i+2] == '#' && rest[i+3] == '#' && rest[i+4] == ' ') {
				nextHeader = i + 1
				break
			}
		}
	}

	var sectionContent, afterSection string
	if nextHeader >= 0 {
		sectionContent = rest[:nextHeader]
		afterSection = rest[nextHeader:]
	} else {
		sectionContent = rest
		afterSection = ""
	}

	before := prompt[:idx]
	entries := strings.Split(strings.TrimSpace(sectionContent), sep)
	// Remove empty entries that may result from trimming
	var nonEmpty []string
	for _, e := range entries {
		if strings.TrimSpace(e) != "" {
			nonEmpty = append(nonEmpty, strings.TrimSpace(e))
		}
	}
	entries = nonEmpty

	trimmed := false
	baseTokens := CountTokens(before+afterSection) + CountTokens(header) + CountTokens("\n\n")
	sepTokens := CountTokens(sep)
	var entryTokens []int
	for _, e := range entries {
		entryTokens = append(entryTokens, CountTokens(e))
	}
	currentEntryCount := len(entries)
	for currentEntryCount > 0 {
		var contentTokens int
		for i := 0; i < currentEntryCount; i++ {
			if i > 0 {
				contentTokens += sepTokens
			}
			contentTokens += entryTokens[i]
		}
		tokens := baseTokens + contentTokens
		if tokens <= budget {
			if trimmed {
				logger.Debug("[Budget] Trimmed retrieved memories", "remaining_entries", currentEntryCount)
			}
			keptEntries := entries[:currentEntryCount]
			content := header + "\n" + strings.Join(keptEntries, sep) + "\n\n"
			candidate := before + content + afterSection
			return candidate, trimmed, tokens
		}
		currentEntryCount--
		trimmed = true
	}

	// All entries removed — strip the section header too
	finalPrompt := strings.TrimRight(before, "\n ") + "\n\n" + afterSection
	logger.Debug("[Budget] Removed all retrieved memories entries")
	return finalPrompt, true, CountTokens(finalPrompt)
}

func buildUnifiedMemoryContextBlock(flags ContextFlags) string {
	sections := make([]string, 0, 4)

	if flags.RecentActivityOverview != "" && flags.Tier != "minimal" {
		sections = append(sections,
			"## Recent Activity\n"+security.IsolateExternalData(flags.RecentActivityOverview),
		)
	}

	if flags.RetrievedMemories != "" && flags.Tier != "minimal" {
		sections = append(sections,
			"## Retrieved Memories\n**Critical**: Memories are snapshots of past observations and ARE FREQUENTLY OUTDATED. "+
				"**Priority rule: fresh tool output always overrides memory.** "+
				"If you have just read a file, executed a tool, or received a tool result in this conversation, "+
				"that is the authoritative current state — ignore any conflicting memory entry about the same file, code, or resource. "+
				"Memories about file content, code structure, integration status, system configuration, "+
				"or tool availability are especially prone to staleness. "+
				"NEVER act on a memory about specific code or file contents without verifying against the actual current file. "+
				"Treat every memory entry as a *hint to investigate*, not a fact.\n\n"+
				security.IsolateExternalData(flags.RetrievedMemories),
		)
	}

	if flags.PredictedMemories != "" && flags.Tier == "full" {
		sections = append(sections,
			"## Predicted Context\n"+security.IsolateExternalData(flags.PredictedMemories),
		)
	}

	if flags.KnowledgeContext != "" && flags.Tier != "minimal" {
		sections = append(sections,
			"## Relevant Knowledge\n"+security.IsolateExternalData(flags.KnowledgeContext),
		)
	}

	if len(sections) == 0 {
		return ""
	}

	return "# UNIFIED MEMORY CONTEXT\n" + strings.Join(sections, "\n\n")
}

// removeLineByPrefix removes all lines starting with the given prefix (and the following blank line).
func removeLineByPrefix(text, prefix string) string {
	lines := strings.Split(text, "\n")
	var out []string
	skipNext := false
	for _, line := range lines {
		if skipNext {
			skipNext = false
			if strings.TrimSpace(line) == "" {
				continue // skip blank line after removed prefix line
			}
		}
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, prefix) {
			skipNext = true
			continue
		}
		out = append(out, line)
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}

// isInsideCodeBlock checks if position idx in text is inside a markdown code block (between ``` and ```).
func isInsideCodeBlock(text string, idx int) bool {
	openCount := 0
	for i := 0; i < idx && i < len(text); i++ {
		if text[i] == '`' {
			if i+2 < len(text) && text[i] == '`' && text[i+1] == '`' && text[i+2] == '`' {
				openCount++
				i += 2
				continue
			}
		}
	}
	return openCount%2 != 0
}

// removeSection removes a section starting with the given header line up to the next section header or end of text.
func removeSection(text, header string) string {
	idx := strings.Index(text, header)
	if idx < 0 {
		return text
	}
	// Guard against false positives: the header must start at the beginning of a line.
	if idx != 0 && text[idx-1] != '\n' {
		return text
	}
	// Guard against matching headers inside code blocks
	if isInsideCodeBlock(text, idx) {
		return text
	}

	// Find the end of this section: next markdown header (# , ## , ### ) or end of text
	rest := text[idx+len(header):]

	// Search for next header by scanning for newline followed by #
	// This is more efficient than splitting the entire string
	nextHeader := -1
	for i := 0; i < len(rest); i++ {
		if rest[i] == '\n' && i+1 < len(rest) && rest[i+1] == '#' {
			// Check if it's a valid header (any number of # followed by a space)
			j := i + 1
			for j < len(rest) && rest[j] == '#' {
				j++
			}
			if j < len(rest) && rest[j] == ' ' {
				nextHeader = i + 1
				break
			}
		}
	}

	if nextHeader < 0 {
		// Section goes to end of text
		return strings.TrimSpace(text[:idx])
	}
	return strings.TrimSpace(text[:idx] + rest[nextHeader:])
}

// OptimizePrompt minifies the prompt for better token efficiency.
// It protects Markdown code blocks and template placeholders.
// Returns the optimized string and the number of characters saved.
func OptimizePrompt(raw string) (string, int) {
	if raw == "" {
		return "", 0
	}

	// 1. Remove HTML comments (multiline safe)
	raw = reHTMLComments.ReplaceAllString(raw, "")

	lines := strings.Split(raw, "\n")
	result := make([]string, 0, len(lines))
	inCodeBlock := false
	codeBlockLang := ""
	var jsonBuffer []string
	emptyLineCount := 0

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Toggle code block state on ``` delimiters
		if strings.HasPrefix(trimmed, "```") {
			if inCodeBlock {
				// We are closing a code block
				if codeBlockLang == "json" && len(jsonBuffer) > 0 {
					joinedJSON := strings.Join(jsonBuffer, "\n")
					var minified bytes.Buffer
					if err := json.Compact(&minified, []byte(joinedJSON)); err == nil {
						result = append(result, minified.String())
					} else {
						result = append(result, jsonBuffer...)
					}
					jsonBuffer = nil
				} else if len(jsonBuffer) > 0 {
					result = append(result, jsonBuffer...)
					jsonBuffer = nil
				}
				inCodeBlock = false
				codeBlockLang = ""
				emptyLineCount = 0
				result = append(result, line) // closing delimiter
			} else {
				// We are opening a code block
				inCodeBlock = true
				codeBlockLang = strings.ToLower(strings.TrimPrefix(trimmed, "```"))
				emptyLineCount = 0
				result = append(result, line) // opening delimiter
			}
			continue
		}

		// Inside code blocks: buffer JSON or keep as-is
		if inCodeBlock {
			if codeBlockLang == "json" {
				jsonBuffer = append(jsonBuffer, line)
			} else {
				result = append(result, line)
			}
			continue
		}

		// Outside code blocks: strip trailing colon from markdown headers to save tokens
		if strings.HasPrefix(trimmed, "#") && strings.HasSuffix(trimmed, ":") {
			trimmed = strings.TrimSuffix(trimmed, ":")
		}

		// Collapse repeated decoration markers (-----, =====, *****)
		if len(trimmed) > 5 {
			if strings.Count(trimmed, "-") == len(trimmed) {
				trimmed = "---"
			} else if strings.Count(trimmed, "=") == len(trimmed) {
				trimmed = "==="
			} else if strings.Count(trimmed, "*") == len(trimmed) {
				trimmed = "***"
			}
		}

		// Preserve leading whitespace for lists to keep structural info, drop else
		var outLine string
		strippedLeft := strings.TrimLeft(line, " \t")
		if strings.HasPrefix(strippedLeft, "- ") || strings.HasPrefix(strippedLeft, "* ") {
			// Keep leading whitespace but remove trailing space
			outLine = strings.TrimRight(line, " \t")

			// Normalize marker inside list to save variant tokens (* to -)
			if strings.HasPrefix(strippedLeft, "* ") {
				idx := strings.Index(outLine, "* ")
				if idx >= 0 {
					outLine = outLine[:idx] + "- " + outLine[idx+2:]
				}
			}
			// Collapse double spaces inline
			outLine = strings.Join(strings.Fields(outLine), " ")
			// Re-add leading whitespace that Fields stripped
			leading := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
			if leading != "" {
				outLine = leading + strings.TrimLeft(outLine, " \t")
			}
		} else {
			// Free text: collapse double spaces and use trimmed line
			outLine = trimmed
			outLine = strings.Join(strings.Fields(outLine), " ")
		}

		// Blank line collapsing: max 1 consecutive empty line
		if outLine == "" {
			emptyLineCount++
			if emptyLineCount <= 1 {
				result = append(result, "")
			}
		} else {
			emptyLineCount = 0
			result = append(result, outLine)
		}
	}

	// Dump any unclosed buffer
	if len(jsonBuffer) > 0 {
		result = append(result, jsonBuffer...)
	}

	optimized := strings.TrimSpace(strings.Join(result, "\n"))
	saved := len(raw) - len(optimized)

	return optimized, saved
}

// loadCorePersonalityContent loads personality profile content with caching.
// Checks disk first (user-overridden), then falls back to embedded defaults.
func loadCorePersonalityContent(promptsDir, profile string, logger *slog.Logger) string {
	profilePath := filepath.Join(promptsDir, "personalities", profile+".md")

	// Check cache
	personalityCacheMu.RLock()
	cached, ok := personalityCache[profile]
	personalityCacheMu.RUnlock()

	if ok {
		if info, err := os.Stat(profilePath); err == nil {
			if !info.ModTime().After(cached.mtime) {
				return cached.content
			}
		} else if cached.fromEmbed {
			// File gone from disk — still valid if loaded from embed
			return cached.content
		}
	}

	var content string
	var mtime time.Time
	var fromEmbed bool
	if data, err := os.ReadFile(profilePath); err == nil {
		content = strings.TrimSpace(string(data))
		if info, err := os.Stat(profilePath); err == nil {
			mtime = info.ModTime()
		}
		logger.Debug("Loaded core personality profile from disk", "profile", profile)
	} else if data, err := fs.ReadFile(promptsembed.FS, "personalities/"+profile+".md"); err == nil {
		content = strings.TrimSpace(string(data))
		fromEmbed = true
		logger.Debug("Loaded core personality profile from embed", "profile", profile)
	} else {
		logger.Warn("Core personality profile not found", "profile", profile)
		return ""
	}

	personalityCacheMu.Lock()
	personalityCache[profile] = personalityCacheEntry{content: content, mtime: mtime, fromEmbed: fromEmbed}
	personalityCacheMu.Unlock()

	return content
}

// CountTokens returns the BPE token count for the given text using tiktoken (cl100k_base).
// Falls back to a char/4 heuristic if the encoder is unavailable
// when tiktoken-go cannot download the BPE vocabulary from the network.
func CountTokens(text string) int {
	if text == "" {
		return 0
	}
	tiktokenOnce.Do(func() {
		type encResult struct {
			enc *tiktoken.Tiktoken
		}
		ch := make(chan encResult, 1)
		go func() {
			enc, err := tiktoken.GetEncoding("cl100k_base")
			if err != nil {
				slog.Warn("[TokenCount] tiktoken init failed", "error", err)
				ch <- encResult{}
				return
			}
			ch <- encResult{enc: enc}
		}()
		select {
		case r := <-ch:
			tiktokenEnc = r.enc
			if r.enc != nil {
				slog.Info("[TokenCount] tiktoken encoder initialized")
			}
		case <-time.After(10 * time.Second):
			slog.Warn("[TokenCount] tiktoken init timed out after 10s, using char/4 fallback")
		}
	})
	if tiktokenEnc != nil {
		return len(tiktokenEnc.Encode(text, nil, nil))
	}
	tiktokenWarnOnce.Do(func() {
		slog.Warn("[TokenCount] tiktoken encoder unavailable, using char/4 fallback")
	})
	chars := len(text)
	newlines := strings.Count(text, "\n")
	tokens := (chars + 3) / 4
	tokens += newlines / 8
	if tokens < 1 {
		return 1
	}
	return tokens
}

// tokenMultiplier returns a model-specific multiplier for token counting accuracy.
// Anthropic Claude and Google Gemini use different tokenizers than cl100k_base,
// so applying a correction factor improves budget estimation accuracy.
func tokenMultiplier(model string) float64 {
	if model == "" {
		return 1.0
	}
	lower := strings.ToLower(model)
	switch {
	case strings.Contains(lower, "claude"):
		return 1.15
	case strings.Contains(lower, "gemini"):
		return 1.10
	case strings.Contains(lower, "deepseek"):
		return 1.05
	default:
		return 1.0
	}
}

// CountTokensForModel returns the estimated token count for a specific model.
// For models that use different tokenizers than cl100k_base (Anthropic, Gemini),
// a correction multiplier is applied.
func CountTokensForModel(text string, model string) int {
	base := CountTokens(text)
	mult := tokenMultiplier(model)
	if mult == 1.0 {
		return base
	}
	return int(float64(base) * mult)
}

// buildEnabledToolsOverview returns a compact one-liner listing enabled tools
// that are not part of the core toolset (filesystem, shell, python, etc.).
// This lets the agent know what integrations are available even when the
// adaptive tool filter removes some tool schemas to save tokens.
func buildEnabledToolsOverview(flags ContextFlags) string {
	skipSet := make(map[string]bool, len(flags.SkipIntegrationTools))
	for _, t := range flags.SkipIntegrationTools {
		skipSet[t] = true
	}
	var enabled []string
	add := func(name string, on bool) {
		if on && !skipSet[name] {
			enabled = append(enabled, name)
		}
	}
	add("docker", flags.DockerEnabled)
	add("home_assistant", flags.HomeAssistantEnabled)
	add("proxmox", flags.ProxmoxEnabled)
	add("tailscale", flags.TailscaleEnabled)
	add("ansible", flags.AnsibleEnabled)
	add("github", flags.GitHubEnabled)
	add("mqtt", flags.MQTTEnabled)
	add("adguard", flags.AdGuardEnabled)
	add("mcp", flags.MCPEnabled)
	add("meshcentral", flags.MeshCentralEnabled)
	add("homepage", flags.HomepageEnabled)
	add("netlify", flags.NetlifyEnabled)
	add("email", flags.EmailEnabled)
	add("cloudflare_tunnel", flags.CloudflareTunnelEnabled)
	add("google_workspace", flags.GoogleWorkspaceEnabled)
	add("onedrive", flags.OneDriveEnabled)
	add("virustotal", flags.VirusTotalEnabled)
	add("brave_search", flags.BraveSearchEnabled)
	add("image_generation", flags.ImageGenerationEnabled)
	add("music_generation", flags.MusicGenerationEnabled)
	add("remote_control", flags.RemoteControlEnabled)
	add("webdav", flags.WebDAVEnabled)
	add("koofr", flags.KoofrEnabled)
	add("chromecast", flags.ChromecastEnabled)
	add("discord", flags.DiscordEnabled)
	add("truenas", flags.TrueNASEnabled)
	add("jellyfin", flags.JellyfinEnabled)
	add("ollama", flags.OllamaEnabled)
	add("sandbox", flags.SandboxEnabled)
	add("webhooks", flags.WebhooksEnabled)
	add("web_scraper", flags.WebScraperEnabled)
	add("s3", flags.S3Enabled)
	add("network_scan", flags.NetworkScanEnabled)
	add("wol", flags.WOLEnabled)
	add("fritzbox", flags.FritzBoxSystemEnabled || flags.FritzBoxNetworkEnabled || flags.FritzBoxSmartHomeEnabled)
	add("telnyx", flags.TelnyxEnabled)
	add("a2a", flags.A2AEnabled)
	add("co_agents", flags.CoAgentEnabled)
	if len(enabled) == 0 {
		return ""
	}
	return "[ENABLED INTEGRATIONS] " + strings.Join(enabled, ", ") + ". Some may be hidden by adaptive tool filtering — if you need one not in your tool list, use it directly by name."
}
