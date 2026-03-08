package agent

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
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

	// 1. Create a timeout context for this co-agent — use Background() so the
	// co-agent survives after the parent HTTP request/main-agent turn ends.
	timeout := time.Duration(cfg.CoAgents.CircuitBreaker.TimeoutSeconds) * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)

	// 2. Register — checks slot availability
	coID, err := coRegistry.Register(req.Task, cancel)
	if err != nil {
		cancel()
		return "", err
	}

	// 3. Build co-agent LLM client
	coModel := coAgentModel(cfg)
	coClient := newCoAgentLLMClient(cfg, logger)

	// 4. Build system prompt
	systemPrompt := buildCoAgentSystemPrompt(cfg, req, longTermMem, shortTermMem)

	// 5. Ephemeral history manager (in-memory only)
	coHistoryMgr := memory.NewEphemeralHistoryManager()

	// 6. Launch goroutine
	go func() {
		defer cancel()

		coLogger := logger.With("component", "co-agent", "co_id", coID)
		coLogger.Info("Co-Agent started", "task", truncateStr(req.Task, 100), "model", coModel, "timeout", timeout)

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
		coCfg.CircuitBreaker.MaxToolCalls = cfg.CoAgents.CircuitBreaker.MaxToolCalls
		coCfg.Agent.PersonalityEngine = false // No personality influence
		coCfg.LLM.Model = coModel             // Use co-agent model for loop
		// Raise token budget so the system prompt is not shed on every iteration.
		// Co-agents inherit the main agent's budget (often 1200) which is too low.
		if cfg.CoAgents.CircuitBreaker.MaxTokens > 0 {
			coCfg.Agent.SystemPromptTokenBudget = cfg.CoAgents.CircuitBreaker.MaxTokens
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

		coLogger.Info("Co-Agent completed", "tokens", tokensUsed, "result_len", len(result))
		coRegistry.Complete(coID, result, tokensUsed, 0)
	}()

	return coID, nil
}

// coAgentModel returns the model name to use for co-agents.
// ResolveProviders already falls back to main LLM model when CoAgents.LLM.Provider is empty.
func coAgentModel(cfg *config.Config) string {
	return cfg.CoAgents.LLM.Model
}

// newCoAgentLLMClient creates an LLM client for a co-agent that benefits from
// the same retry logic as the main agent. It uses a FailoverManager in
// passthrough mode (no fallback configured) to get automatic retry support.
func newCoAgentLLMClient(cfg *config.Config, logger *slog.Logger) llm.ChatClient {
	// Resolved by config.ResolveProviders (falls back to main LLM)
	apiKey := cfg.CoAgents.LLM.APIKey
	baseURL := cfg.CoAgents.LLM.BaseURL

	// Build a temporary config for the FailoverManager with co-agent overrides.
	// Fallback is disabled so the FailoverManager acts as a retry-capable wrapper.
	coCfg := *cfg
	coCfg.LLM.APIKey = apiKey
	coCfg.LLM.BaseURL = baseURL
	coCfg.LLM.Model = coAgentModel(cfg)
	coCfg.FallbackLLM.Enabled = false

	return llm.NewFailoverManager(&coCfg, logger.With("component", "co-agent-llm"))
}

// buildCoAgentSystemPrompt assembles the system prompt for a co-agent.
func buildCoAgentSystemPrompt(cfg *config.Config, req CoAgentRequest, ltm memory.VectorDB, stm *memory.SQLiteMemory) string {
	// 1. Load template
	tmplPath := filepath.Join(cfg.Directories.PromptsDir, "templates", "coagent_system.md")
	tmplBytes, err := os.ReadFile(tmplPath)
	if err != nil {
		// Fallback inline template
		tmplBytes = []byte("You are a Co-Agent helper. Complete the task and return the result.\nLanguage: {{LANGUAGE}}\n\n{{CONTEXT_SNAPSHOT}}\n\nTask: {{TASK}}")
	}
	tmpl := string(tmplBytes)
	// Strip YAML frontmatter (---...---) if present
	if strings.HasPrefix(tmpl, "---") {
		inner := tmpl[3:]
		inner = strings.TrimLeft(inner, "\r\n")
		if idx := strings.Index(inner, "\n---"); idx >= 0 {
			// Skip past the closing --- and any trailing newline
			end := idx + 4
			if end < len(inner) && inner[end] == '\r' {
				end++
			}
			if end < len(inner) && inner[end] == '\n' {
				end++
			}
			tmpl = strings.TrimSpace(inner[end:])
		}
	}

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
