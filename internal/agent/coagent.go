package agent

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"aurago/internal/budget"
	"aurago/internal/config"
	"aurago/internal/llm"
	"aurago/internal/memory"
	"aurago/internal/security"
	"aurago/internal/tools"

	"github.com/sashabaranov/go-openai"
)

// CoAgentRequest describes a task to be given to a co-agent.
type CoAgentRequest struct {
	Task         string   // Task description for the co-agent
	ContextHints []string // Optional additional context strings
	Specialist   string   // Specialist role ("researcher","coder","designer","security","writer") or empty for generic
}

// SpawnCoAgent starts a co-agent goroutine and returns its ID.
// Returns an error when the system is disabled or all slots are occupied.
func SpawnCoAgent(
	cfg *config.Config,
	parentCtx context.Context,
	logger *slog.Logger,
	coRegistry *CoAgentRegistry,

	// Shared resources (thread-safe, read-only for co-agent where enforced by blacklist)
	shortTermMem *memory.SQLiteMemory,
	longTermMem memory.VectorDB,
	vault *security.Vault,
	procRegistry *tools.ProcessRegistry,
	manifest *tools.Manifest,
	kg *memory.KnowledgeGraph,
	inventoryDB *sql.DB,

	req CoAgentRequest,
	budgetTracker *budget.Tracker,
) (string, error) {
	if !cfg.CoAgents.Enabled {
		return "", fmt.Errorf("co-agent system is disabled — set co_agents.enabled=true in config.yaml")
	}

	// Validate and check specialist enablement
	if req.Specialist != "" {
		if !config.ValidSpecialistRoles[req.Specialist] {
			return "", fmt.Errorf("unknown specialist role: %q", req.Specialist)
		}
		spec := cfg.GetSpecialist(req.Specialist)
		if spec == nil || !spec.Enabled {
			return "", fmt.Errorf("specialist %q is not enabled — enable co_agents.specialists.%s.enabled in config.yaml", req.Specialist, req.Specialist)
		}
	}

	// 1. Create a timeout context for this co-agent — use Background() so the
	// co-agent survives after the parent HTTP request/main-agent turn ends.
	timeoutSec := cfg.CoAgents.CircuitBreaker.TimeoutSeconds
	if req.Specialist != "" {
		if spec := cfg.GetSpecialist(req.Specialist); spec != nil && spec.CircuitBreaker.TimeoutSeconds > 0 {
			timeoutSec = spec.CircuitBreaker.TimeoutSeconds
		}
	}
	timeout := time.Duration(timeoutSec) * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)

	// 2. Register — checks slot availability. Specialist IDs use "specialist-<role>-N" prefix.
	idPrefix := "coagent"
	if req.Specialist != "" {
		idPrefix = "specialist-" + req.Specialist
	}
	coID, err := coRegistry.RegisterWithPrefix(idPrefix, req.Task, cancel)
	if err != nil {
		cancel()
		return "", err
	}

	// 3. Build co-agent LLM client (specialist may use a different provider)
	coModel := coAgentModelForRole(cfg, req.Specialist)
	coClient := newCoAgentLLMClientForRole(cfg, logger, req.Specialist)

	// 4. Build system prompt (specialist gets its own template)
	var systemPrompt string
	if req.Specialist != "" {
		systemPrompt = buildSpecialistSystemPrompt(cfg, req.Specialist, req, longTermMem, shortTermMem)
	} else {
		systemPrompt = buildCoAgentSystemPrompt(cfg, req, longTermMem, shortTermMem)
	}

	// 5. Ephemeral history manager (in-memory only)
	coHistoryMgr := memory.NewEphemeralHistoryManager()

	// 6. Launch goroutine
	go func() {
		defer cancel()

		component := "co-agent"
		if req.Specialist != "" {
			component = "specialist-" + req.Specialist
		}
		coLogger := logger.With("component", component, "co_id", coID)
		coLogger.Info("Co-Agent started", "task", truncateStr(req.Task, 100), "model", coModel, "timeout", timeout, "specialist", req.Specialist)

		// Deep-copy config with co-agent overrides
		coCfg := *cfg
		// Deep-copy slice fields to avoid shared references with the main config
		if len(cfg.CircuitBreaker.RetryIntervals) > 0 {
			coCfg.CircuitBreaker.RetryIntervals = make([]string, len(cfg.CircuitBreaker.RetryIntervals))
			copy(coCfg.CircuitBreaker.RetryIntervals, cfg.CircuitBreaker.RetryIntervals)
		}
		if len(cfg.Budget.Models) > 0 {
			coCfg.Budget.Models = make([]config.ModelCost, len(cfg.Budget.Models))
			copy(coCfg.Budget.Models, cfg.Budget.Models)
		}

		// Apply circuit breaker: specialist overrides → co_agents defaults
		maxToolCalls := cfg.CoAgents.CircuitBreaker.MaxToolCalls
		maxTokensBudget := cfg.CoAgents.CircuitBreaker.MaxTokens
		if req.Specialist != "" {
			if spec := cfg.GetSpecialist(req.Specialist); spec != nil {
				if spec.CircuitBreaker.MaxToolCalls > 0 {
					maxToolCalls = spec.CircuitBreaker.MaxToolCalls
				}
				if spec.CircuitBreaker.MaxTokens > 0 {
					maxTokensBudget = spec.CircuitBreaker.MaxTokens
				}
			}
		}
		coCfg.CircuitBreaker.MaxToolCalls = maxToolCalls
		coCfg.Agent.PersonalityEngine = false // No personality influence
		coCfg.LLM.Model = coModel             // Use co-agent model for loop
		if maxTokensBudget > 0 {
			coCfg.Agent.SystemPromptTokenBudget = maxTokensBudget
		} else {
			coCfg.Agent.SystemPromptTokenBudget = 6000
		}

		llmReq := openai.ChatCompletionRequest{
			Model: coModel,
			Messages: []openai.ChatCompletionMessage{
				{Role: openai.ChatMessageRoleSystem, Content: systemPrompt},
				{Role: openai.ChatMessageRoleUser, Content: req.Task},
			},
		}

		// NoopBroker — co-agent sends no events to UI
		broker := &NoopBroker{}
		sessionID := coID // Prefix "coagent-" enables blacklist in DispatchToolCall

		runCfg := RunConfig{
			Config:          &coCfg,
			Logger:          coLogger,
			LLMClient:       coClient,
			ShortTermMem:    shortTermMem,
			HistoryManager:  coHistoryMgr, // Ephemeral history
			LongTermMem:     longTermMem,
			KG:              kg,
			InventoryDB:     inventoryDB,
			Vault:           vault,
			Registry:        procRegistry,
			Manifest:        manifest,
			CronManager:     nil, // cron_scheduler will be rejected
			CoAgentRegistry: nil, // co-agents cannot spawn sub-agents
			BudgetTracker:   budgetTracker,
			SessionID:       sessionID,
			IsMaintenance:   false,
			SurgeryPlan:     "",
		}

		resp, err := ExecuteAgentLoop(ctx, llmReq, runCfg, false, broker)

		if err != nil {
			coLogger.Error("Co-Agent failed", "error", err)
			coRegistry.Fail(coID, err.Error(), 0, 0)
			return
		}

		result := ""
		if len(resp.Choices) > 0 {
			result = resp.Choices[0].Message.Content
		}
		tokensUsed := resp.Usage.TotalTokens

		// Limit result size to prevent memory exhaustion from unexpectedly large LLM outputs.
		const maxCoAgentResultBytes = 100_000
		if len(result) > maxCoAgentResultBytes {
			coLogger.Warn("Co-Agent result truncated", "original_len", len(result))
			result = result[:maxCoAgentResultBytes] + "\n\n[Result truncated — exceeded 100 KB]"
		}

		coLogger.Info("Co-Agent completed", "tokens", tokensUsed, "result_len", len(result))
		coRegistry.Complete(coID, result, tokensUsed, 0)
	}()

	return coID, nil
}

// coAgentModelForRole returns the model name to use for a co-agent/specialist.
// Cascade: specialist LLM → co_agents LLM → main LLM (resolved by ResolveProviders).
func coAgentModelForRole(cfg *config.Config, role string) string {
	if role != "" {
		if spec := cfg.GetSpecialist(role); spec != nil && spec.LLM.Model != "" {
			return spec.LLM.Model
		}
	}
	return cfg.CoAgents.LLM.Model
}

// newCoAgentLLMClientForRole creates an LLM client for a co-agent or specialist.
func newCoAgentLLMClientForRole(cfg *config.Config, logger *slog.Logger, role string) llm.ChatClient {
	apiKey := cfg.CoAgents.LLM.APIKey
	baseURL := cfg.CoAgents.LLM.BaseURL
	model := coAgentModelForRole(cfg, role)

	if role != "" {
		if spec := cfg.GetSpecialist(role); spec != nil {
			if spec.LLM.APIKey != "" {
				apiKey = spec.LLM.APIKey
			}
			if spec.LLM.BaseURL != "" {
				baseURL = spec.LLM.BaseURL
			}
		}
	}

	coCfg := *cfg
	coCfg.LLM.APIKey = apiKey
	coCfg.LLM.BaseURL = baseURL
	coCfg.LLM.Model = model
	coCfg.FallbackLLM.Enabled = false

	component := "co-agent-llm"
	if role != "" {
		component = "specialist-" + role + "-llm"
	}
	return llm.NewFailoverManager(&coCfg, logger.With("component", component))
}

// coAgentModel returns the model name to use for generic co-agents.
func coAgentModel(cfg *config.Config) string {
	return cfg.CoAgents.LLM.Model
}

// newCoAgentLLMClient creates an LLM client for a generic co-agent.
func newCoAgentLLMClient(cfg *config.Config, logger *slog.Logger) llm.ChatClient {
	return newCoAgentLLMClientForRole(cfg, logger, "")
}

// coAgentTemplateMissing is the sentinel value used in the template cache to signal
// that a file does not exist on disk, preventing repeated failed reads.
const coAgentTemplateMissing = "\x00MISSING"

var (
	coAgentTemplateMu    sync.RWMutex
	coAgentTemplateCache = make(map[string]string)
)

// loadPromptTemplate reads a prompt template from the cache, populating it on first
// access via os.ReadFile. If the file cannot be read, fallback is returned.
// Templates are cached for the lifetime of the process; restart to pick up changes.
func loadPromptTemplate(path, fallback string) string {
	coAgentTemplateMu.RLock()
	if cached, ok := coAgentTemplateCache[path]; ok {
		coAgentTemplateMu.RUnlock()
		if cached == coAgentTemplateMissing {
			return fallback
		}
		return cached
	}
	coAgentTemplateMu.RUnlock()

	// Read outside any lock — multiple goroutines may do this redundantly,
	// but the write is idempotent and far cheaper than holding a lock during I/O.
	b, err := os.ReadFile(path)

	coAgentTemplateMu.Lock()
	defer coAgentTemplateMu.Unlock()
	// Double-check: another goroutine may have populated the cache while we read.
	if cached, ok := coAgentTemplateCache[path]; ok {
		if cached == coAgentTemplateMissing {
			return fallback
		}
		return cached
	}
	if err != nil {
		coAgentTemplateCache[path] = coAgentTemplateMissing
		return fallback
	}
	s := string(b)
	coAgentTemplateCache[path] = s
	return s
}

// loadPromptTemplateExists reads a template from the cache.
// Returns ("", false) if the file does not exist or cannot be read.
func loadPromptTemplateExists(path string) (string, bool) {
	tmpl := loadPromptTemplate(path, coAgentTemplateMissing)
	if tmpl == coAgentTemplateMissing {
		return "", false
	}
	return tmpl, true
}

// buildCoAgentSystemPrompt assembles the system prompt for a co-agent.
func buildCoAgentSystemPrompt(cfg *config.Config, req CoAgentRequest, ltm memory.VectorDB, stm *memory.SQLiteMemory) string {
	// 1. Load template (cached after first read)
	const coAgentFallbackTmpl = "You are a Co-Agent helper. Complete the task and return the result.\nLanguage: {{LANGUAGE}}\n\n{{CONTEXT_SNAPSHOT}}\n\nTask: {{TASK}}"
	tmplPath := filepath.Join(cfg.Directories.PromptsDir, "templates", "coagent_system.md")
	tmpl := stripYAMLFrontmatter(loadPromptTemplate(tmplPath, coAgentFallbackTmpl))

	// 2. Core memory snapshot (read-only)
	var coreMem []byte
	if stm != nil {
		coreMem = []byte(stm.ReadCoreMemory())
	}

	// 3. RAG search for task context
	var ragContext string
	if ltm != nil {
		results, _, err := ltm.SearchSimilar(req.Task, 3)
		if err == nil && len(results) > 0 {
			ragContext = strings.Join(results, "\n---\n")
		}
	}

	// 4. User-provided context hints
	hintsStr := strings.Join(req.ContextHints, "\n")

	// 5. Assemble context snapshot
	var sb strings.Builder
	if len(coreMem) > 0 {
		sb.WriteString("## Core Memory\n")
		sb.Write(coreMem)
		sb.WriteString("\n\n")
	}
	if ragContext != "" {
		sb.WriteString("## Relevant Context (RAG)\n")
		sb.WriteString(ragContext)
		sb.WriteString("\n\n")
	}
	if hintsStr != "" {
		sb.WriteString("## Additional Hints\n")
		sb.WriteString(hintsStr)
		sb.WriteString("\n")
	}

	// 6. Fill template
	prompt := strings.ReplaceAll(tmpl, "{{LANGUAGE}}", cfg.Agent.SystemLanguage)
	prompt = strings.ReplaceAll(prompt, "{{CONTEXT_SNAPSHOT}}", sb.String())
	prompt = strings.ReplaceAll(prompt, "{{TASK}}", req.Task)

	return prompt
}

// buildSpecialistSystemPrompt assembles the system prompt for a specialist co-agent.
// It loads the specialist-specific template, falling back to the generic co-agent template.
func buildSpecialistSystemPrompt(cfg *config.Config, role string, req CoAgentRequest, ltm memory.VectorDB, stm *memory.SQLiteMemory) string {
	// 1. Load specialist template (cached), fallback to generic co-agent template
	tmplPath := filepath.Join(cfg.Directories.PromptsDir, "templates", "specialist_"+role+".md")
	rawTmpl, ok := loadPromptTemplateExists(tmplPath)
	if !ok {
		return buildCoAgentSystemPrompt(cfg, req, ltm, stm)
	}
	tmpl := stripYAMLFrontmatter(rawTmpl)

	// 2. Core memory snapshot (read-only)
	var coreMem []byte
	if stm != nil {
		coreMem = []byte(stm.ReadCoreMemory())
	}

	// 3. RAG search for task context
	var ragContext string
	if ltm != nil {
		results, _, err := ltm.SearchSimilar(req.Task, 3)
		if err == nil && len(results) > 0 {
			ragContext = strings.Join(results, "\n---\n")
		}
	}

	// 4. User-provided context hints
	hintsStr := strings.Join(req.ContextHints, "\n")

	// 5. Assemble context snapshot
	var sb strings.Builder
	if len(coreMem) > 0 {
		sb.WriteString("## Core Memory\n")
		sb.Write(coreMem)
		sb.WriteString("\n\n")
	}
	if ragContext != "" {
		sb.WriteString("## Relevant Context (RAG)\n")
		sb.WriteString(ragContext)
		sb.WriteString("\n\n")
	}
	if hintsStr != "" {
		sb.WriteString("## Additional Hints\n")
		sb.WriteString(hintsStr)
		sb.WriteString("\n")
	}

	// 6. Fill template
	prompt := strings.ReplaceAll(tmpl, "{{LANGUAGE}}", cfg.Agent.SystemLanguage)
	prompt = strings.ReplaceAll(prompt, "{{CONTEXT_SNAPSHOT}}", sb.String())
	prompt = strings.ReplaceAll(prompt, "{{TASK}}", req.Task)

	return prompt
}

// stripYAMLFrontmatter removes YAML frontmatter (---...---) from the beginning of a string.
func stripYAMLFrontmatter(s string) string {
	if !strings.HasPrefix(s, "---") {
		return s
	}
	inner := s[3:]
	inner = strings.TrimLeft(inner, "\r\n")
	if idx := strings.Index(inner, "\n---"); idx >= 0 {
		end := idx + 4
		if end < len(inner) && inner[end] == '\r' {
			end++
		}
		if end < len(inner) && inner[end] == '\n' {
			end++
		}
		return strings.TrimSpace(inner[end:])
	}
	return s
}

// specialistsAvailable returns true if at least one specialist co-agent is enabled.
func specialistsAvailable(cfg *config.Config) bool {
	if !cfg.CoAgents.Enabled {
		return false
	}
	s := &cfg.CoAgents.Specialists
	return s.Researcher.Enabled || s.Coder.Enabled || s.Designer.Enabled || s.Security.Enabled || s.Writer.Enabled
}

// buildSpecialistsStatus returns a human-readable status string listing enabled specialists.
// Used for injection into the main agent's system prompt.
func buildSpecialistsStatus(cfg *config.Config) string {
	if !cfg.CoAgents.Enabled {
		return ""
	}
	type spec struct {
		role string
		desc string
		on   bool
	}
	specs := []spec{
		{"researcher", "Internet research, fact-finding, source verification", cfg.CoAgents.Specialists.Researcher.Enabled},
		{"coder", "Code planning, writing, debugging, testing", cfg.CoAgents.Specialists.Coder.Enabled},
		{"designer", "Image generation, visual design, layout concepts", cfg.CoAgents.Specialists.Designer.Enabled},
		{"security", "Security audits, vulnerability analysis, system hardening", cfg.CoAgents.Specialists.Security.Enabled},
		{"writer", "High-quality text creation, documentation, professional writing", cfg.CoAgents.Specialists.Writer.Enabled},
	}
	var sb strings.Builder
	for _, s := range specs {
		if s.on {
			sb.WriteString(fmt.Sprintf("- **%s**: %s\n", s.role, s.desc))
		}
	}
	if sb.Len() == 0 {
		return "No specialists are currently enabled."
	}
	return sb.String()
}
