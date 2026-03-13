package prompts

import (
	"aurago/internal/memory"
	promptsembed "aurago/prompts"
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
var (
	tiktokenOnce sync.Once
	tiktokenEnc  *tiktoken.Tiktoken
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
	IsErrorState      bool
	RequiresCoding    bool
	RetrievedMemories string
	PredictedMemories string // Phase B: proactively pre-fetched memories from temporal/tool patterns
	PersonalityLine   string // Phase D: compact self-awareness line [Self: mood=X | C:0.82 ...]
	CorePersonality   string // Selected core personality profile name (e.g. "neutral", "punk")
	ActiveProcesses   string // PID (name) comma-separated
	SystemLanguage    string
	LifeboatEnabled   bool
	IsMaintenanceMode bool
	SurgeryPlan       string
	PredictedGuides   []string // Content of tool guides to inject
	// Optimization fields
	Tier              string   // "full", "compact", "minimal" — controls module loading
	MessageCount      int      // Current message count in the conversation
	TokenBudget       int      // Max tokens for system prompt (0 = unlimited)
	RecentlyUsedTools []string // Last N tools the agent used (for lazy schema injection)
	IsDebugMode       bool     // When true, inject a debugging instruction into the system prompt
	IsCoAgent         bool     // True if the current LLM call is for a co-agent
	IsEgg             bool     // True if this instance runs in egg worker mode
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
	WebhooksEnabled          bool
	WebhooksDefinitions      string // Summary of configured outgoing webhooks for tool context
	VirusTotalEnabled        bool
	BraveSearchEnabled       bool
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
	MissionsEnabled          bool
	StopProcessEnabled       bool
	InventoryEnabled         bool
	MemoryMaintenanceEnabled bool
	WOLEnabled               bool
	InternetExposed          bool   // HTTPS is enabled — system is likely reachable from the internet
	IsDocker                 bool   // Running inside a Docker container
	UserProfileSummary       string // Optional user profile summary from profiling engine
	AdditionalPrompt         string // Extra instructions always appended at end of system prompt
	SessionTodoItems         string // Session-scoped task list piggybacked on tool calls
}

// DetermineTier returns the appropriate prompt tier based on the conversation length.
// full = all modules; compact = skip RAG/guides; minimal = identity + tools only.
func DetermineTier(messageCount int) string {
	switch {
	case messageCount <= 6:
		return "full"
	case messageCount <= 12:
		return "compact"
	default:
		return "minimal"
	}
}

// PromptMetadata holds the tags and priority for a prompt module.
type PromptMetadata struct {
	ID         string                 `yaml:"id"`
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
func BuildSystemPrompt(promptsDir string, flags ContextFlags, coreMemory string, logger *slog.Logger) string {
	var finalPrompt strings.Builder

	// Auto-determine tier if not set
	if flags.Tier == "" {
		flags.Tier = DetermineTier(flags.MessageCount)
	}

	// 1. Load and parse all prompt modules
	modules := loadPromptModules(promptsDir, logger)

	// 1b. Load core personality profile content (injected later in dynamic section for prominence)
	corePersonalityContent := ""
	if flags.CorePersonality != "" {
		// Try disk first (user-created or overridden personality), fall back to embedded default.
		profilePath := filepath.Join(promptsDir, "personalities", flags.CorePersonality+".md")
		if data, err := os.ReadFile(profilePath); err == nil {
			corePersonalityContent = strings.TrimSpace(string(data))
			logger.Debug("Loaded core personality profile from disk", "profile", flags.CorePersonality)
		} else if data, err := fs.ReadFile(promptsembed.FS, "personalities/"+flags.CorePersonality+".md"); err == nil {
			corePersonalityContent = strings.TrimSpace(string(data))
			logger.Debug("Loaded core personality profile from embed", "profile", flags.CorePersonality)
		} else {
			logger.Warn("Core personality profile not found", "profile", flags.CorePersonality)
		}
	}

	// 2. Filter modules based on flags
	selectedModules := filterModules(modules, flags)

	// 3. Sort by priority
	sort.Slice(selectedModules, func(i, j int) bool {
		return selectedModules[i].Metadata.Priority < selectedModules[j].Metadata.Priority
	})

	// 4. Assemble modules
	for _, mod := range selectedModules {
		finalPrompt.WriteString(mod.Content)
		finalPrompt.WriteString("\n\n")
	}

	// 5. Add dynamic content — tier-aware

	// Language Instruction
	if flags.SystemLanguage != "" {
		finalPrompt.WriteString(fmt.Sprintf("# LANGUAGE\nRespond in %s.\n\n", flags.SystemLanguage))
	}

	// Surgery Plan injection (always inject when present, regardless of maintenance module)
	if flags.IsMaintenanceMode && flags.SurgeryPlan != "" {
		finalPrompt.WriteString("### SURGERY PLAN ###\n")
		finalPrompt.WriteString(flags.SurgeryPlan)
		finalPrompt.WriteString("\n\n")
	}

	// Core Memory — always inject (small and critical)
	if coreMemory != "" {
		finalPrompt.WriteString("### CORE MEMORY ###\n")
		finalPrompt.WriteString(coreMemory)
		finalPrompt.WriteString("\n\n")
	}

	// Session-scoped task list — always inject when present
	if flags.SessionTodoItems != "" {
		finalPrompt.WriteString("### ACTIVE TASK LIST ###\n")
		finalPrompt.WriteString(flags.SessionTodoItems)
		finalPrompt.WriteString("\n\n")
	}

	// RAG: Retrieved Long-Term Memories — skip in minimal tier
	if flags.RetrievedMemories != "" && flags.Tier != "minimal" {
		finalPrompt.WriteString("# RETRIEVED MEMORIES\n")
		finalPrompt.WriteString(flags.RetrievedMemories)
		finalPrompt.WriteString("\n\n")
	}

	// Predictive RAG — only in full tier
	if flags.PredictedMemories != "" && flags.Tier == "full" {
		finalPrompt.WriteString("# PREDICTED CONTEXT\n")
		finalPrompt.WriteString(flags.PredictedMemories)
		finalPrompt.WriteString("\n\n")
	}

	// System Status
	if flags.ActiveProcesses != "" && flags.ActiveProcesses != "None" {
		finalPrompt.WriteString(fmt.Sprintf("[ACTIVE DAEMONS] %s\n\n", flags.ActiveProcesses))
	}

	// Dynamic Tool Guides — only in full tier
	if len(flags.PredictedGuides) > 0 && flags.Tier == "full" {
		finalPrompt.WriteString("# TOOL GUIDES\n")
		for _, guide := range flags.PredictedGuides {
			finalPrompt.WriteString(guide)
			finalPrompt.WriteString("\n\n")
		}
	}

	// Dynamic Outgoing Webhooks definition
	if flags.WebhooksEnabled && flags.WebhooksDefinitions != "" && flags.Tier != "minimal" {
		finalPrompt.WriteString("# OUTGOING WEBHOOKS\n")
		finalPrompt.WriteString(flags.WebhooksDefinitions)
		finalPrompt.WriteString("\n\n")
	}

	now := time.Now()

	// Core Personality Profile (injected near end for maximum LLM attention)
	if corePersonalityContent != "" {
		finalPrompt.WriteString("# YOUR PERSONALITY (ACTIVE PROFILE: " + strings.ToUpper(flags.CorePersonality) + ")\n")
		finalPrompt.WriteString("You MUST embody this personality in EVERY response. This overrides any default tone.\n")
		finalPrompt.WriteString(corePersonalityContent)
		finalPrompt.WriteString("\n\n")
	}

	// User Profile (optional, from profiling engine)
	if flags.UserProfileSummary != "" {
		finalPrompt.WriteString(flags.UserProfileSummary)
		finalPrompt.WriteString("\n")
	}

	// Personality self-awareness (Phase D micro-traits)
	if flags.PersonalityLine != "" {
		finalPrompt.WriteString(flags.PersonalityLine)
		finalPrompt.WriteString("\n\n")
	}

	finalPrompt.WriteString(fmt.Sprintf("# NOW\n%s %s\n",
		now.Format("2006-01-02"), now.Format("15:04")))

	// Internet-exposure warning — shown before custom instructions so it is always visible
	if flags.InternetExposed {
		finalPrompt.WriteString("\n> **Warning:** This system is probably reachable from the internet. Be careful when exposing services to the outside!\n")
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

	rawPrompt := finalPrompt.String()
	rawLen := len(rawPrompt)

	// 6. Token budget shedding FIRST — shed large sections before spending CPU on optimization
	var shedSections []string
	budgetShedTriggered := false
	if flags.TokenBudget > 0 {
		rawPrompt, shedSections = budgetShed(rawPrompt, flags, corePersonalityContent, coreMemory, now, logger)
		budgetShedTriggered = len(shedSections) > 0
	}

	// 7. Optimize after shedding — only minify what remains
	optimized, saved := OptimizePrompt(rawPrompt)
	finalTokens := CountTokens(optimized)

	logger.Debug("System prompt built", "raw_len", rawLen, "optimized_len", len(optimized), "saved_chars", saved, "tier", flags.Tier, "tokens", finalTokens)

	// 8. Record build metrics for dashboard
	RecordBuild(PromptBuildRecord{
		Timestamp:     now,
		Tier:          flags.Tier,
		RawLen:        rawLen,
		OptimizedLen:  len(optimized),
		SavedChars:    saved,
		Tokens:        finalTokens,
		TokenBudget:   flags.TokenBudget,
		ModulesLoaded: len(modules),
		ModulesUsed:   len(selectedModules),
		GuidesCount:   len(flags.PredictedGuides),
		ShedSections:  shedSections,
		BudgetShed:    budgetShedTriggered,
		MessageCount:  flags.MessageCount,
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
	shedHeaders := []string{
		"# TOOL GUIDES",
		"# PREDICTED CONTEXT",
		"### User Profile",
	}

	result := prompt
	for _, header := range shedHeaders {
		if tokens <= flags.TokenBudget {
			break
		}
		before := len(result)
		result = removeSection(result, header)
		if len(result) < before {
			tokens = CountTokens(result)
			shedList = append(shedList, header)
			logger.Debug("[Budget] Shed section", "header", header, "new_tokens", tokens)
		}
	}

	// Token-aware Retrieved Memories trim: progressively remove individual entries (lowest ranked first)
	// instead of dropping the entire section at once.
	if tokens > flags.TokenBudget {
		var trimmed bool
		result, trimmed, tokens = trimRetrievedMemoriesSection(result, flags.TokenBudget, logger)
		if trimmed {
			shedList = append(shedList, "# RETRIEVED MEMORIES (partial)")
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
	for len(entries) > 0 {
		content := header + "\n" + strings.Join(entries, sep) + "\n\n"
		candidate := before + content + afterSection
		tokens := CountTokens(candidate)
		if tokens <= budget {
			if trimmed {
				logger.Debug("[Budget] Trimmed retrieved memories", "remaining_entries", len(entries))
			}
			return candidate, trimmed, tokens
		}
		// Drop the last (lowest-ranked) entry and retry
		entries = entries[:len(entries)-1]
		trimmed = true
	}

	// All entries removed — strip the section header too
	finalPrompt := strings.TrimRight(before, "\n ") + "\n\n" + afterSection
	logger.Debug("[Budget] Removed all retrieved memories entries")
	return finalPrompt, true, CountTokens(finalPrompt)
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

// removeSection removes a section starting with the given header line up to the next section header or end of text.
func removeSection(text, header string) string {
	idx := strings.Index(text, header)
	if idx < 0 {
		return text
	}

	// Find the end of this section: next markdown header (# , ## , ### ) or end of text
	rest := text[idx+len(header):]

	// Search for next header by scanning for newline followed by #
	// This is more efficient than splitting the entire string
	nextHeader := -1
	for i := 0; i < len(rest); i++ {
		if rest[i] == '\n' && i+1 < len(rest) && rest[i+1] == '#' {
			// Check if it's a valid header (# , ## , ### )
			if i+2 < len(rest) && rest[i+2] == ' ' {
				nextHeader = i + 1
				break
			}
			if i+3 < len(rest) && rest[i+2] == '#' && rest[i+3] == ' ' {
				nextHeader = i + 1
				break
			}
			if i+4 < len(rest) && rest[i+2] == '#' && rest[i+3] == '#' && rest[i+4] == ' ' {
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
	emptyLineCount := 0

	for _, line := range lines {
		// Toggle code block state on ``` delimiters
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			inCodeBlock = !inCodeBlock
			emptyLineCount = 0
			result = append(result, line)
			continue
		}

		// Inside code blocks: keep as-is (protection)
		if inCodeBlock {
			result = append(result, line)
			continue
		}

		// Outside code blocks: trim + collapse markers + collapse blank lines
		trimmed := strings.TrimSpace(line)

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

		// Blank line collapsing: max 1 consecutive empty line
		if trimmed == "" {
			emptyLineCount++
			if emptyLineCount <= 1 {
				result = append(result, "")
			}
		} else {
			emptyLineCount = 0
			result = append(result, trimmed)
		}
	}

	optimized := strings.TrimSpace(strings.Join(result, "\n"))
	saved := len(raw) - len(optimized)

	return optimized, saved
}

// CountTokens returns the number of BPE tokens in text using the cl100k_base encoding.
// Falls back to a character-based heuristic if the encoder fails to initialize.
func CountTokens(text string) int {
	tiktokenOnce.Do(func() {
		enc, err := tiktoken.GetEncoding("cl100k_base")
		if err == nil {
			tiktokenEnc = enc
		}
	})
	if tiktokenEnc != nil {
		return len(tiktokenEnc.Encode(text, nil, nil))
	}
	// Fallback: rough estimate
	return len(text) / 4
}

// parseOrFallback parses a prompt module, falling back to a minimal struct if
// the file has no YAML frontmatter.
