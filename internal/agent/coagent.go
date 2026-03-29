package agent

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
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
	Priority     int      // 1=low, 2=normal, 3=high
}

type coAgentPromptTemplate struct {
	Content string
	ModTime time.Time
	Exists  bool
}

type coAgentLLMSelection struct {
	Model   string
	APIKey  string
	BaseURL string
	Source  string
}

type coAgentBroker struct {
	id       string
	registry *CoAgentRegistry
}

func (b *coAgentBroker) Send(event, message string) {
	if b == nil || b.registry == nil {
		return
	}
	parts := []string{strings.TrimSpace(event), strings.TrimSpace(message)}
	b.registry.RecordEvent(b.id, strings.TrimSpace(strings.Join(parts, ": ")))
}

func (b *coAgentBroker) SendJSON(jsonStr string) {}

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
) (string, CoAgentState, error) {
	if !cfg.CoAgents.Enabled {
		return "", "", fmt.Errorf("co-agent system is disabled — set co_agents.enabled=true in config.yaml")
	}
	if budgetTracker != nil && budgetTracker.IsCategoryQuotaBlocked("coagent", cfg.CoAgents.BudgetQuotaPercent) {
		return "", "", fmt.Errorf("co-agent quota reached — co_agents.budget_quota_percent is exhausted for today")
	}
	if !cfg.CoAgents.QueueWhenBusy && coRegistry.AvailableSlots() <= 0 {
		return "", "", fmt.Errorf("all %d co-agent slots are occupied", cfg.CoAgents.MaxConcurrent)
	}
	req = normalizeCoAgentRequest(cfg, req)
	if strings.TrimSpace(req.Task) == "" {
		return "", "", fmt.Errorf("'task' is required to spawn a co-agent")
	}

	// Validate and check specialist enablement
	if req.Specialist != "" {
		if !config.ValidSpecialistRoles[req.Specialist] {
			return "", "", fmt.Errorf("unknown specialist role: %q", req.Specialist)
		}
		spec := cfg.GetSpecialist(req.Specialist)
		if spec == nil || !spec.Enabled {
			return "", "", fmt.Errorf("specialist %q is not enabled — enable co_agents.specialists.%s.enabled in config.yaml", req.Specialist, req.Specialist)
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
	coID, state, err := coRegistry.RegisterWithPriority(idPrefix, req.Task, cancel, reqPriority(req))
	if err != nil {
		cancel()
		return "", "", err
	}

	// 3. Build co-agent LLM client (specialist may use a different provider).
	// Designer specialists sometimes get configured with image-only models such as
	// Seedream/FLUX/Imagen. Those cannot run the normal chat+tool loop, so we
	// transparently fall back to a text-capable co-agent/main model for reasoning
	// while the specialist can still call the image_generation tool.
	coLLM, llmFallback := selectCoAgentLLMForRole(cfg, req.Specialist)
	coModel := coLLM.Model
	coClient := newCoAgentLLMClientForSelection(cfg, logger, req.Specialist, coLLM)

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
		if state == CoAgentQueued {
			coLogger.Info("Co-Agent queued", "task", truncateStr(req.Task, 100), "model", coModel, "timeout", timeout, "specialist", req.Specialist)
			if err := coRegistry.WaitForStart(coID, ctx); err != nil {
				coLogger.Warn("Co-Agent did not start", "error", err)
				return
			}
		}
		if llmFallback != "" {
			coLogger.Warn("Co-Agent specialist model fallback applied", "reason", llmFallback, "model", coModel, "specialist", req.Specialist)
			coRegistry.RecordEvent(coID, llmFallback)
		}
		coRegistry.RecordEvent(coID, "starting execution")
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
		coCfg.Personality.Engine = false // No personality influence
		coCfg.LLM.Model = coModel        // Use co-agent model for loop
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

		broker := &coAgentBroker{id: coID, registry: coRegistry}
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

		maxRetries := cfg.CoAgents.RetryPolicy.MaxRetries
		delay := time.Duration(cfg.CoAgents.RetryPolicy.RetryDelaySeconds) * time.Second
		var resp openai.ChatCompletionResponse
		var err error
		for attempt := 0; attempt <= maxRetries; attempt++ {
			if attempt > 0 {
				coRegistry.RecordEvent(coID, fmt.Sprintf("retry %d/%d", attempt, maxRetries))
				select {
				case <-ctx.Done():
					err = ctx.Err()
				case <-time.After(delay):
				}
				if err != nil {
					break
				}
			}
			resp, err = ExecuteAgentLoop(ctx, llmReq, runCfg, false, broker)
			if err == nil {
				break
			}
			if !isRetryableCoAgentError(cfg, err) || attempt == maxRetries {
				break
			}
			coRegistry.RecordRetry(coID, err.Error())
			coLogger.Warn("Co-Agent transient failure; retrying", "attempt", attempt+1, "max_retries", maxRetries, "error", err)
		}

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
		maxCoAgentResultBytes := cfg.CoAgents.MaxResultBytes
		if len(result) > maxCoAgentResultBytes {
			coLogger.Warn("Co-Agent result truncated", "original_len", len(result))
			result = result[:maxCoAgentResultBytes] + fmt.Sprintf("\n\n[Result truncated — exceeded %d bytes]", maxCoAgentResultBytes)
		}

		coLogger.Info("Co-Agent completed", "tokens", tokensUsed, "result_len", len(result))
		coRegistry.Complete(coID, result, tokensUsed, 0)
	}()

	return coID, state, nil
}

func selectCoAgentLLMForRole(cfg *config.Config, role string) (coAgentLLMSelection, string) {
	selection := coAgentLLMSelection{
		Model:   strings.TrimSpace(cfg.CoAgents.LLM.Model),
		APIKey:  cfg.CoAgents.LLM.APIKey,
		BaseURL: cfg.CoAgents.LLM.BaseURL,
		Source:  "co_agents",
	}
	if role != "" {
		if spec := cfg.GetSpecialist(role); spec != nil {
			if spec.LLM.APIKey != "" {
				selection.APIKey = spec.LLM.APIKey
			}
			if spec.LLM.BaseURL != "" {
				selection.BaseURL = spec.LLM.BaseURL
			}
			if spec.LLM.Model != "" {
				selection.Model = strings.TrimSpace(spec.LLM.Model)
				selection.Source = "specialist"
			}
		}
	}

	if role != "designer" || !isLikelyImageOnlyCoAgentModel(selection.Model) {
		return selection, ""
	}

	candidates := []coAgentLLMSelection{
		{
			Model:   strings.TrimSpace(cfg.CoAgents.LLM.Model),
			APIKey:  cfg.CoAgents.LLM.APIKey,
			BaseURL: cfg.CoAgents.LLM.BaseURL,
			Source:  "co_agents",
		},
		{
			Model:   strings.TrimSpace(cfg.LLM.Model),
			APIKey:  cfg.LLM.APIKey,
			BaseURL: cfg.LLM.BaseURL,
			Source:  "main",
		},
	}
	for _, candidate := range candidates {
		if candidate.Model == "" || isLikelyImageOnlyCoAgentModel(candidate.Model) {
			continue
		}
		if candidate.Model == selection.Model && candidate.APIKey == selection.APIKey && candidate.BaseURL == selection.BaseURL {
			continue
		}
		return candidate, fmt.Sprintf("designer specialist model %q is image-only; falling back to %s model %q for chat/tool execution", selection.Model, candidate.Source, candidate.Model)
	}

	return selection, ""
}

// newCoAgentLLMClientForSelection creates an LLM client for a co-agent or specialist.
func newCoAgentLLMClientForSelection(cfg *config.Config, logger *slog.Logger, role string, selection coAgentLLMSelection) llm.ChatClient {
	coCfg := *cfg
	coCfg.LLM.APIKey = selection.APIKey
	coCfg.LLM.BaseURL = selection.BaseURL
	coCfg.LLM.Model = selection.Model
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
	selection, _ := selectCoAgentLLMForRole(cfg, "")
	return newCoAgentLLMClientForSelection(cfg, logger, "", selection)
}

func isLikelyImageOnlyCoAgentModel(model string) bool {
	model = strings.ToLower(strings.TrimSpace(model))
	if model == "" {
		return false
	}
	patterns := []string{
		"seedream",
		"flux",
		"imagen",
		"dall-e",
		"gpt-image",
		"gpt-5-image",
		"recraft",
		"stable-diffusion",
		"sdxl",
		"midjourney",
	}
	for _, pattern := range patterns {
		if strings.Contains(model, pattern) {
			return true
		}
	}
	return false
}

var (
	coAgentTemplateMu    sync.RWMutex
	coAgentTemplateCache = make(map[string]coAgentPromptTemplate)
)

// loadPromptTemplate reads a prompt template from the cache, populating it on first
// access via os.ReadFile. If the file cannot be read, fallback is returned.
// The cache invalidates automatically when the file timestamp changes.
func loadPromptTemplate(path, fallback string) string {
	stat, statErr := os.Stat(path)

	coAgentTemplateMu.RLock()
	if cached, ok := coAgentTemplateCache[path]; ok {
		coAgentTemplateMu.RUnlock()
		if statErr != nil {
			if !cached.Exists {
				return fallback
			}
		} else if cached.Exists && cached.ModTime.Equal(stat.ModTime()) {
			return cached.Content
		}
	} else {
		coAgentTemplateMu.RUnlock()
	}

	// Read outside any lock — multiple goroutines may do this redundantly,
	// but the write is idempotent and far cheaper than holding a lock during I/O.
	b, err := os.ReadFile(path)

	coAgentTemplateMu.Lock()
	defer coAgentTemplateMu.Unlock()
	// Double-check: another goroutine may have populated the cache while we read.
	if cached, ok := coAgentTemplateCache[path]; ok {
		if statErr != nil && !cached.Exists {
			return fallback
		}
		if statErr == nil && cached.Exists && cached.ModTime.Equal(stat.ModTime()) {
			return cached.Content
		}
	}
	if err != nil {
		coAgentTemplateCache[path] = coAgentPromptTemplate{Exists: false}
		return fallback
	}
	s := string(b)
	modTime := time.Time{}
	if statErr == nil {
		modTime = stat.ModTime()
	}
	coAgentTemplateCache[path] = coAgentPromptTemplate{
		Content: s,
		ModTime: modTime,
		Exists:  true,
	}
	return s
}

// loadPromptTemplateExists reads a template from the cache.
// Returns ("", false) if the file does not exist or cannot be read.
func loadPromptTemplateExists(path string) (string, bool) {
	tmpl := loadPromptTemplate(path, "")
	if tmpl == "" {
		return "", false
	}
	return tmpl, true
}

// buildCoAgentSystemPrompt assembles the system prompt for a co-agent.
// buildContextSnapshot assembles the shared context block (core memory, RAG, hints)
// used in both co-agent and specialist system prompts.
func buildContextSnapshot(req CoAgentRequest, ltm memory.VectorDB, stm *memory.SQLiteMemory) string {
	var coreMem []byte
	if stm != nil {
		coreMem = []byte(stm.ReadCoreMemory())
	}

	var ragContext string
	if ltm != nil {
		results, _, err := ltm.SearchSimilar(req.Task, 3)
		if err == nil && len(results) > 0 {
			ragContext = strings.Join(results, "\n---\n")
		}
	}

	hintsStr := strings.Join(req.ContextHints, "\n")

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
	return sb.String()
}

func normalizeCoAgentRequest(cfg *config.Config, req CoAgentRequest) CoAgentRequest {
	req.Task = strings.TrimSpace(req.Task)
	req.Specialist = strings.ToLower(strings.TrimSpace(req.Specialist))
	if cfg == nil {
		return req
	}
	maxHints := cfg.CoAgents.MaxContextHints
	maxChars := cfg.CoAgents.MaxContextHintChars
	seen := make(map[string]struct{})
	filtered := make([]string, 0, min(len(req.ContextHints), maxHints))
	for _, hint := range req.ContextHints {
		hint = strings.TrimSpace(strings.ReplaceAll(hint, "\r", " "))
		hint = strings.ReplaceAll(hint, "\n", " ")
		if hint == "" {
			continue
		}
		if maxChars > 0 && len(hint) > maxChars {
			hint = hint[:maxChars]
		}
		key := strings.ToLower(hint)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		filtered = append(filtered, hint)
		if maxHints > 0 && len(filtered) >= maxHints {
			break
		}
	}
	req.ContextHints = filtered
	return req
}

func reqPriority(req CoAgentRequest) int {
	return normalizeCoAgentPriority(req.Priority)
}

func isRetryableCoAgentError(cfg *config.Config, err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	if cfg == nil {
		return strings.Contains(msg, "timeout") || strings.Contains(msg, "deadline exceeded")
	}
	for _, pattern := range cfg.CoAgents.RetryPolicy.RetryableErrorPatterns {
		if pattern != "" && strings.Contains(msg, strings.ToLower(pattern)) {
			return true
		}
	}
	return false
}

func buildCoAgentSystemPrompt(cfg *config.Config, req CoAgentRequest, ltm memory.VectorDB, stm *memory.SQLiteMemory) string {
	const coAgentFallbackTmpl = "You are a Co-Agent helper. Complete the task and return the result.\nLanguage: {{LANGUAGE}}\n\n{{CONTEXT_SNAPSHOT}}\n\nTask: {{TASK}}"
	tmplPath := filepath.Join(cfg.Directories.PromptsDir, "templates", "coagent_system.md")
	tmpl := stripYAMLFrontmatter(loadPromptTemplate(tmplPath, coAgentFallbackTmpl))

	prompt := strings.ReplaceAll(tmpl, "{{LANGUAGE}}", cfg.Agent.SystemLanguage)
	prompt = strings.ReplaceAll(prompt, "{{CONTEXT_SNAPSHOT}}", buildContextSnapshot(req, ltm, stm))
	prompt = strings.ReplaceAll(prompt, "{{TASK}}", req.Task)
	return prompt
}

// buildSpecialistSystemPrompt assembles the system prompt for a specialist co-agent.
// It loads the specialist-specific template, falling back to the generic co-agent template.
func buildSpecialistSystemPrompt(cfg *config.Config, role string, req CoAgentRequest, ltm memory.VectorDB, stm *memory.SQLiteMemory) string {
	tmplPath := filepath.Join(cfg.Directories.PromptsDir, "templates", "specialist_"+role+".md")
	rawTmpl, ok := loadPromptTemplateExists(tmplPath)
	if !ok {
		return buildCoAgentSystemPrompt(cfg, req, ltm, stm)
	}
	tmpl := stripYAMLFrontmatter(rawTmpl)

	prompt := strings.ReplaceAll(tmpl, "{{LANGUAGE}}", cfg.Agent.SystemLanguage)
	prompt = strings.ReplaceAll(prompt, "{{CONTEXT_SNAPSHOT}}", buildContextSnapshot(req, ltm, stm))
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

func buildSpecialistDelegationHint(cfg *config.Config, userQuery string) string {
	if cfg == nil || !specialistsAvailable(cfg) {
		return ""
	}
	query := strings.ToLower(strings.TrimSpace(userQuery))
	if query == "" {
		return ""
	}
	roles := make([]string, 0, 3)
	addRole := func(role string, enabled bool) {
		if !enabled {
			return
		}
		if !slices.Contains(roles, role) {
			roles = append(roles, role)
		}
	}
	if coAgentContainsAny(query, "research", "compare", "investigate", "look up", "find sources", "verify") {
		addRole("researcher", cfg.CoAgents.Specialists.Researcher.Enabled)
	}
	if coAgentContainsAny(query, "code", "implement", "refactor", "debug", "test", "fix bug") {
		addRole("coder", cfg.CoAgents.Specialists.Coder.Enabled)
	}
	if coAgentContainsAny(query, "design", "ui", "ux", "image", "logo", "layout", "visual") {
		addRole("designer", cfg.CoAgents.Specialists.Designer.Enabled)
	}
	if coAgentContainsAny(query, "security", "audit", "vulnerability", "threat", "hardening", "cve") {
		addRole("security", cfg.CoAgents.Specialists.Security.Enabled)
	}
	if coAgentContainsAny(query, "write", "document", "blog", "article", "summary", "report") {
		addRole("writer", cfg.CoAgents.Specialists.Writer.Enabled)
	}
	complex := len(roles) >= 2 ||
		strings.Contains(query, " and ") ||
		strings.Contains(query, "parallel") ||
		strings.Contains(query, "meanwhile") ||
		len(query) > 220
	if !complex || len(roles) == 0 {
		return ""
	}
	if len(roles) == 1 {
		return fmt.Sprintf("### Delegation Hint\nThis task may benefit from `spawn_specialist` with the **%s** specialist if the work becomes multi-step.", roles[0])
	}
	return fmt.Sprintf("### Delegation Hint\nThis request spans multiple domains. Consider splitting it with `spawn_specialist`, for example: **%s**.", strings.Join(roles, ", "))
}

func coAgentContainsAny(value string, patterns ...string) bool {
	for _, pattern := range patterns {
		if strings.Contains(value, pattern) {
			return true
		}
	}
	return false
}
