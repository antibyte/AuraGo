package agent

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"os"
	"path/filepath"
	"runtime/debug"
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

type coAgentProgressBroadcaster func(payload map[string]interface{})

type coAgentBroker struct {
	id        string
	registry  *CoAgentRegistry
	broadcast coAgentProgressBroadcaster
}

func (b *coAgentBroker) Send(event, message string) {
	if b == nil || b.registry == nil {
		return
	}
	parts := []string{strings.TrimSpace(event), strings.TrimSpace(message)}
	b.registry.RecordEvent(b.id, strings.TrimSpace(strings.Join(parts, ": ")))
	if event == "tool_start" || event == "tool_end" || event == "thinking" || event == "error_recovery" {
		b.emitProgress()
	}
}

func (b *coAgentBroker) SendJSON(jsonStr string) {
	if b == nil {
		return
	}
	if b.registry != nil {
		b.registry.RecordEvent(b.id, strings.TrimSpace(jsonStr))
	}
}

func (b *coAgentBroker) SendLLMStreamDelta(content, toolName, toolID string, index int, finishReason string) {
}

func (b *coAgentBroker) SendLLMStreamDone(finishReason string) {
	b.emitProgress()
}

func (b *coAgentBroker) SendTokenUpdate(prompt, completion, total, sessionTotal, globalTotal int, isEstimated, isFinal bool, source string) {
	if b != nil && b.registry != nil && isFinal {
		b.emitProgress()
	}
}

func (b *coAgentBroker) SendThinkingBlock(provider, content, state string) {
}

func (b *coAgentBroker) emitProgress() {
	if b == nil || b.broadcast == nil || b.registry == nil {
		return
	}
	status, err := b.registry.GetStatus(b.id)
	if err != nil {
		return
	}
	b.broadcast(status)
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
	cheatsheetDB *sql.DB,

	req CoAgentRequest,
	budgetTracker *budget.Tracker,
	progressSink coAgentProgressBroadcaster,
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

	// 3. Select LLM model (lightweight — no client created yet).
	coLLM, llmFallback := selectCoAgentLLMForRole(cfg, req.Specialist)
	coModel := coLLM.Model

	// 4. Ephemeral history manager (in-memory only)
	coHistoryMgr := memory.NewEphemeralHistoryManager()

	// 5. Launch goroutine — expensive work (LLM client, config clone, prompt build)
	// is deferred until after queue promotion so queued agents consume minimal resources.
	go func() {
		component := "co-agent"
		if req.Specialist != "" {
			component = "specialist-" + req.Specialist
		}
		coLogger := logger.With("component", component, "co_id", coID)
		defer cancel()
		defer recoverCoAgentPanic(coRegistry, coID, coLogger)
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

		coClient := newCoAgentLLMClientForSelection(cfg, logger, req.Specialist, coLLM)

		var systemPrompt string
		if req.Specialist != "" {
			systemPrompt = buildSpecialistSystemPrompt(cfg, req.Specialist, req, longTermMem, shortTermMem, cheatsheetDB)
		} else {
			systemPrompt = buildCoAgentSystemPrompt(cfg, req, longTermMem, shortTermMem)
		}

		coCfg := deepClone(*cfg)

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
		coCfg.Personality.Engine = false
		coCfg.LLM.Model = coModel
		coCfg.Agent.SystemPromptTokenBudget = 6000
		if maxTokensBudget > 0 {
			coCfg.Agent.SystemPromptTokenBudget = maxTokensBudget
		}

		llmReq := openai.ChatCompletionRequest{
			Model: coModel,
			Messages: []openai.ChatCompletionMessage{
				{Role: openai.ChatMessageRoleSystem, Content: systemPrompt},
				{Role: openai.ChatMessageRoleUser, Content: req.Task},
			},
		}

		broker := &coAgentBroker{id: coID, registry: coRegistry, broadcast: progressSink}
		sessionID := coID // Prefix "coagent-" enables blacklist in DispatchToolCall

		runCfg := RunConfig{
			Config:            &coCfg,
			Logger:            coLogger,
			LLMClient:         coClient,
			ShortTermMem:      shortTermMem,
			HistoryManager:    coHistoryMgr,
			LongTermMem:       longTermMem,
			KG:                kg,
			InventoryDB:       inventoryDB,
			Vault:             vault,
			Registry:          procRegistry,
			Manifest:          manifest,
			CronManager:       nil,
			CoAgentRegistry:   nil,
			BudgetTracker:     budgetTracker,
			SessionID:         sessionID,
			IsMaintenance:     false,
			IsCoAgent:         true,
			CoAgentSpecialist: req.Specialist,
			SurgeryPlan:       "",
		}

		maxRetries := cfg.CoAgents.RetryPolicy.MaxRetries
		delay := time.Duration(cfg.CoAgents.RetryPolicy.RetryDelaySeconds) * time.Second
		var resp openai.ChatCompletionResponse
		var err error
		for attempt := 0; attempt <= maxRetries; attempt++ {
			if attempt > 0 {
				coRegistry.RecordEvent(coID, fmt.Sprintf("retry %d/%d", attempt, maxRetries))
				jitter := time.Duration(float64(delay) * (0.8 + rand.Float64()*0.4))
				select {
				case <-ctx.Done():
					err = ctx.Err()
				case <-time.After(jitter):
				}
				if err != nil {
					break
				}
			}
			resp, err = ExecuteAgentLoop(ctx, llmReq, runCfg, false, broker)
			if partial := extractCoAgentPartialResult(coHistoryMgr); partial != "" {
				coRegistry.RecordPartialResult(coID, partial)
			}
			if err == nil {
				break
			}
			if !isRetryableCoAgentError(cfg, err) || attempt == maxRetries {
				break
			}
			coRegistry.RecordRetry(coID, err.Error())
			coRegistry.RecordEvent(coID, fmt.Sprintf("transient failure before retry: %s", truncateStr(err.Error(), 160)))
			coLogger.Warn("Co-Agent transient failure; retrying", "attempt", attempt+1, "max_retries", maxRetries, "error", err)
		}

		if err != nil {
			if partial := extractCoAgentPartialResult(coHistoryMgr); partial != "" {
				coRegistry.RecordPartialResult(coID, partial)
			}
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
		if maxCoAgentResultBytes > 0 && len(result) > maxCoAgentResultBytes {
			coLogger.Warn("Co-Agent result truncated", "original_len", len(result))
			notice := fmt.Sprintf("\n\n[Result truncated — exceeded %d bytes]", maxCoAgentResultBytes)
			targetLen := maxCoAgentResultBytes - len(notice)
			if targetLen < 0 {
				targetLen = 0
			}
			result = truncateUTF8ToLimit(result, targetLen, notice)
		}

		if result != "" {
			coRegistry.RecordPartialResult(coID, result)
		}
		coLogger.Info("Co-Agent completed", "tokens", tokensUsed, "result_len", len(result))
		coRegistry.Complete(coID, result, tokensUsed, 0)
	}()

	return coID, state, nil
}

func recoverCoAgentPanic(registry *CoAgentRegistry, coID string, logger *slog.Logger) {
	if recovered := recover(); recovered != nil {
		errMsg := fmt.Sprintf("co-agent panic: %v", recovered)
		if logger != nil {
			logger.Error("Co-Agent panicked", "error", recovered, "stack", string(debug.Stack()))
		}
		if registry != nil && coID != "" {
			registry.Fail(coID, errMsg, 0, 0)
		}
	}
}

func extractCoAgentPartialResult(history *memory.HistoryManager) string {
	if history == nil {
		return ""
	}
	if summary := strings.TrimSpace(history.GetSummary()); summary != "" {
		return truncateStr(summary, 1200)
	}
	all := history.GetAll()
	for i := len(all) - 1; i >= 0; i-- {
		msg := all[i]
		if msg.IsInternal || msg.Role != openai.ChatMessageRoleAssistant {
			continue
		}
		content := strings.TrimSpace(msg.Content)
		if content == "" || content == "[Empty Response]" {
			continue
		}
		return truncateStr(content, 1200)
	}
	return ""
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
	coCfg := deepClone(*cfg)
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

type coAgentContextPolicy struct {
	maxCoreChars int
	maxRAGHits   int
	maxRAGChars  int
	maxHints     int
	maxHintChars int
}

func contextPolicyForSpecialist(role string) coAgentContextPolicy {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "researcher":
		return coAgentContextPolicy{maxCoreChars: 700, maxRAGHits: 3, maxRAGChars: 700, maxHints: 6, maxHintChars: 220}
	case "coder":
		return coAgentContextPolicy{maxCoreChars: 1200, maxRAGHits: 2, maxRAGChars: 900, maxHints: 5, maxHintChars: 220}
	case "designer":
		return coAgentContextPolicy{maxCoreChars: 400, maxRAGHits: 1, maxRAGChars: 500, maxHints: 4, maxHintChars: 180}
	case "security":
		return coAgentContextPolicy{maxCoreChars: 1000, maxRAGHits: 2, maxRAGChars: 900, maxHints: 5, maxHintChars: 220}
	case "writer":
		return coAgentContextPolicy{maxCoreChars: 700, maxRAGHits: 2, maxRAGChars: 700, maxHints: 5, maxHintChars: 220}
	default:
		return coAgentContextPolicy{maxCoreChars: 1200, maxRAGHits: 2, maxRAGChars: 800, maxHints: 6, maxHintChars: 220}
	}
}

func truncatePromptBlock(s string, maxChars int) string {
	s = strings.TrimSpace(s)
	if s == "" || maxChars <= 0 || len(s) <= maxChars {
		return s
	}
	cutoff := maxChars - 3
	if cutoff < 1 {
		cutoff = maxChars
	}
	return strings.TrimSpace(s[:cutoff]) + "..."
}

func trimCoAgentHints(hints []string, maxHints, maxHintChars int) []string {
	if len(hints) == 0 {
		return nil
	}
	if maxHints <= 0 {
		maxHints = len(hints)
	}
	trimmed := make([]string, 0, min(len(hints), maxHints))
	for _, hint := range hints {
		hint = truncatePromptBlock(hint, maxHintChars)
		if hint == "" {
			continue
		}
		trimmed = append(trimmed, hint)
		if len(trimmed) >= maxHints {
			break
		}
	}
	return trimmed
}

// buildCoAgentSystemPrompt assembles the system prompt for a co-agent.
// buildContextSnapshot assembles a lean shared context block (core memory, local
// memory hits, hints) so specialists spend more of their budget on the task itself.
func buildContextSnapshot(req CoAgentRequest, ltm memory.VectorDB, stm *memory.SQLiteMemory) string {
	policy := contextPolicyForSpecialist(req.Specialist)

	coreMem := ""
	if stm != nil {
		coreMem = truncatePromptBlock(stm.ReadCoreMemory(), policy.maxCoreChars)
	}

	var ragItems []string
	if ltm != nil && policy.maxRAGHits > 0 {
		results, _, err := ltm.SearchMemoriesOnly(req.Task, policy.maxRAGHits)
		if err == nil {
			for _, result := range results {
				result = truncatePromptBlock(result, policy.maxRAGChars)
				if result == "" {
					continue
				}
				ragItems = append(ragItems, result)
				if len(ragItems) >= policy.maxRAGHits {
					break
				}
			}
		}
	}

	hints := trimCoAgentHints(req.ContextHints, policy.maxHints, policy.maxHintChars)

	var sb strings.Builder
	if coreMem != "" {
		sb.WriteString("## Core Memory\n")
		sb.WriteString(coreMem)
		sb.WriteString("\n\n")
	}
	if len(ragItems) > 0 {
		sb.WriteString("## Relevant Context (RAG)\n")
		sb.WriteString(strings.Join(ragItems, "\n---\n"))
		sb.WriteString("\n\n")
	}
	if len(hints) > 0 {
		sb.WriteString("## Additional Hints\n")
		for _, hint := range hints {
			sb.WriteString("- ")
			sb.WriteString(hint)
			sb.WriteString("\n")
		}
	}
	return strings.TrimSpace(sb.String())
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
func buildSpecialistSystemPrompt(cfg *config.Config, role string, req CoAgentRequest, ltm memory.VectorDB, stm *memory.SQLiteMemory, cheatsheetDB *sql.DB) string {
	tmplPath := filepath.Join(cfg.Directories.PromptsDir, "templates", "specialist_"+role+".md")
	rawTmpl, ok := loadPromptTemplateExists(tmplPath)
	if !ok {
		return buildCoAgentSystemPrompt(cfg, req, ltm, stm)
	}
	tmpl := stripYAMLFrontmatter(rawTmpl)

	prompt := strings.ReplaceAll(tmpl, "{{LANGUAGE}}", cfg.Agent.SystemLanguage)
	prompt = strings.ReplaceAll(prompt, "{{CONTEXT_SNAPSHOT}}", buildContextSnapshot(req, ltm, stm))
	prompt = strings.ReplaceAll(prompt, "{{TASK}}", req.Task)

	var extras strings.Builder
	specCfg := specialistConfigByRole(cfg, role)
	if specCfg != nil {
		if specCfg.CheatsheetID != "" && cheatsheetDB != nil {
			cs, err := tools.CheatsheetGet(cheatsheetDB, specCfg.CheatsheetID)
			if err != nil {
				slog.Warn("[Specialist] Cheatsheet not found", "role", role, "cheatsheet_id", specCfg.CheatsheetID, "error", err)
			} else if cs != nil {
				extras.WriteString("\n\n<cheatsheet name=\"")
				extras.WriteString(cs.Name)
				extras.WriteString("\">\n")
				extras.WriteString(cs.Content)
				for _, att := range cs.Attachments {
					extras.WriteString("\n\n--- ")
					extras.WriteString(att.Filename)
					extras.WriteString(" ---\n")
					extras.WriteString(att.Content)
				}
				extras.WriteString("\n</cheatsheet>")
			}
		}
		if specCfg.AdditionalPrompt != "" {
			extras.WriteString("\n\n")
			extras.WriteString(specCfg.AdditionalPrompt)
		}
	}

	if extras.Len() > 0 {
		prompt += extras.String()
	}
	return prompt
}

func specialistConfigByRole(cfg *config.Config, role string) *config.SpecialistConfig {
	switch role {
	case "researcher":
		return &cfg.CoAgents.Specialists.Researcher
	case "coder":
		return &cfg.CoAgents.Specialists.Coder
	case "designer":
		return &cfg.CoAgents.Specialists.Designer
	case "security":
		return &cfg.CoAgents.Specialists.Security
	case "writer":
		return &cfg.CoAgents.Specialists.Writer
	default:
		return nil
	}
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

	explicitDelegation := coAgentContainsAny(query,
		"spawn_specialist",
		"specialist",
		"delegate",
		"delegation",
		"parallel",
		"subtask",
		"split this",
	)
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

	complexityScore := 0
	if len(roles) >= 2 {
		complexityScore++
	}
	if coAgentContainsAny(query, " and ", " then ", " meanwhile ", "while you", "after that", "compare", "with sources") {
		complexityScore++
	}
	if len(query) > 260 {
		complexityScore++
	}

	if len(roles) == 0 {
		return ""
	}

	if explicitDelegation {
		if len(roles) == 1 {
			return fmt.Sprintf("### Delegation Hint\nIf you want to delegate part of this task, `spawn_specialist` with **%s** is the best fit.", roles[0])
		}
		return fmt.Sprintf("### Delegation Hint\nIf you want to split this into parallel specialist work, the best matches are: **%s**.", strings.Join(roles, ", "))
	}

	if len(roles) < 2 || complexityScore < 2 {
		return ""
	}
	return fmt.Sprintf("### Delegation Hint\nThis looks like a multi-step cross-domain task. If it helps, `spawn_specialist` could split it between **%s**.", strings.Join(roles, ", "))
}

func coAgentContainsAny(value string, patterns ...string) bool {
	for _, pattern := range patterns {
		if strings.Contains(value, pattern) {
			return true
		}
	}
	return false
}

// BuildSpecialistsAvailable reports whether any co-agent specialist is enabled.
func BuildSpecialistsAvailable(cfg *config.Config) bool {
	return specialistsAvailable(cfg)
}

// BuildSpecialistsStatus returns a human-readable list of enabled specialists for system-prompt injection.
func BuildSpecialistsStatus(cfg *config.Config) string {
	return buildSpecialistsStatus(cfg)
}

// BuildSpecialistDelegationHint returns a delegation hint string based on the user query.
func BuildSpecialistDelegationHint(cfg *config.Config, userQuery string) string {
	return buildSpecialistDelegationHint(cfg, userQuery)
}
