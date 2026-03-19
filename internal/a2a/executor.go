package a2a

import (
	"context"
	"database/sql"
	"fmt"
	"iter"
	"log/slog"
	"strings"

	"aurago/internal/agent"
	"aurago/internal/budget"
	"aurago/internal/config"
	"aurago/internal/llm"
	"aurago/internal/memory"
	"aurago/internal/security"
	"aurago/internal/tools"

	"github.com/a2aproject/a2a-go/v2/a2a"
	"github.com/a2aproject/a2a-go/v2/a2asrv"
	"github.com/sashabaranov/go-openai"
)

// ExecutorDeps holds the shared dependencies for the A2A executor.
type ExecutorDeps struct {
	Config       *config.Config
	Logger       *slog.Logger
	LLMClient    llm.ChatClient
	ShortTermMem *memory.SQLiteMemory
	LongTermMem  memory.VectorDB
	Vault        *security.Vault
	Registry     *tools.ProcessRegistry
	Manifest     *tools.Manifest
	KG           *memory.KnowledgeGraph
	InventoryDB  *sql.DB
	Budget       *budget.Tracker
}

// Executor implements a2asrv.AgentExecutor, bridging A2A requests to AuraGo's agent loop.
type Executor struct {
	deps *ExecutorDeps
}

// NewExecutor creates a new A2A executor.
func NewExecutor(deps *ExecutorDeps) *Executor {
	return &Executor{deps: deps}
}

// Ensure Executor implements a2asrv.AgentExecutor.
var _ a2asrv.AgentExecutor = (*Executor)(nil)

// Execute handles an incoming A2A message by running the AuraGo agent loop.
func (e *Executor) Execute(ctx context.Context, execCtx *a2asrv.ExecutorContext) iter.Seq2[a2a.Event, error] {
	return func(yield func(a2a.Event, error) bool) {
		logger := e.deps.Logger.With("component", "a2a-executor", "task_id", execCtx.TaskID)
		logger.Info("A2A Execute started", "context_id", execCtx.ContextID)

		// Extract text from the incoming message parts
		userText := extractTextFromMessage(execCtx.Message)
		if userText == "" {
			evt := a2a.NewStatusUpdateEvent(execCtx, a2a.TaskStateFailed,
				a2a.NewMessage(a2a.MessageRoleAgent, a2a.NewTextPart("No text content in message")))
			yield(evt, nil)
			return
		}

		// Emit working status
		workingEvt := a2a.NewStatusUpdateEvent(execCtx, a2a.TaskStateWorking, nil)
		if !yield(workingEvt, nil) {
			return
		}

		// Build a dedicated LLM client for A2A requests (uses resolved A2A LLM config)
		a2aClient := e.buildLLMClient()
		model := e.deps.Config.A2A.LLM.Model

		// Build system prompt for A2A context
		systemPrompt := buildA2ASystemPrompt(e.deps.Config)

		llmReq := openai.ChatCompletionRequest{
			Model: model,
			Messages: []openai.ChatCompletionMessage{
				{Role: openai.ChatMessageRoleSystem, Content: systemPrompt},
				{Role: openai.ChatMessageRoleUser, Content: userText},
			},
		}

		// Ephemeral history for A2A requests
		historyMgr := memory.NewEphemeralHistoryManager()
		sessionID := fmt.Sprintf("a2a-%s", execCtx.TaskID)

		// Build config with A2A-specific overrides
		a2aCfg := *e.deps.Config
		a2aCfg.LLM.APIKey = e.deps.Config.A2A.LLM.APIKey
		a2aCfg.LLM.BaseURL = e.deps.Config.A2A.LLM.BaseURL
		a2aCfg.LLM.Model = model
		a2aCfg.Agent.PersonalityEngine = false
		a2aCfg.FallbackLLM.Enabled = false
		if a2aCfg.CircuitBreaker.MaxToolCalls <= 0 {
			a2aCfg.CircuitBreaker.MaxToolCalls = 10
		}

		runCfg := agent.RunConfig{
			Config:          &a2aCfg,
			Logger:          logger,
			LLMClient:       a2aClient,
			ShortTermMem:    e.deps.ShortTermMem,
			HistoryManager:  historyMgr,
			LongTermMem:     e.deps.LongTermMem,
			KG:              e.deps.KG,
			InventoryDB:     e.deps.InventoryDB,
			Vault:           e.deps.Vault,
			Registry:        e.deps.Registry,
			Manifest:        e.deps.Manifest,
			CronManager:     nil, // A2A agents cannot manage cron
			CoAgentRegistry: nil, // A2A agents cannot spawn co-agents
			BudgetTracker:   e.deps.Budget,
			SessionID:       sessionID,
			IsMaintenance:   false,
		}

		broker := &agent.NoopBroker{}
		resp, err := agent.ExecuteAgentLoop(ctx, llmReq, runCfg, false, broker)

		if err != nil {
			logger.Error("A2A agent loop failed", "error", err)
			errMsg := a2a.NewMessage(a2a.MessageRoleAgent, a2a.NewTextPart(fmt.Sprintf("Agent error: %v", err)))
			evt := a2a.NewStatusUpdateEvent(execCtx, a2a.TaskStateFailed, errMsg)
			yield(evt, nil)
			return
		}

		// Extract response text
		result := ""
		if len(resp.Choices) > 0 {
			result = resp.Choices[0].Message.Content
		}
		if result == "" {
			result = "Task completed with no output."
		}

		logger.Info("A2A Execute completed", "result_len", len(result))

		// Emit artifact with the response
		artifactEvt := a2a.NewArtifactEvent(execCtx, a2a.NewTextPart(result))
		if !yield(artifactEvt, nil) {
			return
		}

		// Emit completed status
		completedMsg := a2a.NewMessage(a2a.MessageRoleAgent, a2a.NewTextPart(result))
		completedEvt := a2a.NewStatusUpdateEvent(execCtx, a2a.TaskStateCompleted, completedMsg)
		yield(completedEvt, nil)
	}
}

// Cancel handles a cancellation request for an A2A task.
func (e *Executor) Cancel(ctx context.Context, execCtx *a2asrv.ExecutorContext) iter.Seq2[a2a.Event, error] {
	return func(yield func(a2a.Event, error) bool) {
		e.deps.Logger.Info("A2A task cancelled", "task_id", execCtx.TaskID)
		evt := a2a.NewStatusUpdateEvent(execCtx, a2a.TaskStateCanceled, nil)
		yield(evt, nil)
	}
}

// buildLLMClient creates an LLM client for A2A requests.
func (e *Executor) buildLLMClient() llm.ChatClient {
	cfg := e.deps.Config
	a2aCfg := *cfg
	a2aCfg.LLM.APIKey = cfg.A2A.LLM.APIKey
	a2aCfg.LLM.BaseURL = cfg.A2A.LLM.BaseURL
	a2aCfg.LLM.Model = cfg.A2A.LLM.Model
	a2aCfg.FallbackLLM.Enabled = false

	return llm.NewFailoverManager(&a2aCfg, e.deps.Logger.With("component", "a2a-llm"))
}

// extractTextFromMessage extracts all text parts from an A2A message.
func extractTextFromMessage(msg *a2a.Message) string {
	if msg == nil {
		return ""
	}
	var parts []string
	for _, p := range msg.Parts {
		if t, ok := p.Content.(a2a.Text); ok {
			parts = append(parts, string(t))
		}
	}
	return strings.Join(parts, "\n")
}

// buildA2ASystemPrompt creates a system prompt for A2A agent interactions.
func buildA2ASystemPrompt(cfg *config.Config) string {
	name := cfg.A2A.Server.AgentName
	if name == "" {
		name = "AuraGo"
	}
	return fmt.Sprintf(`You are %s, an AI agent responding to an Agent-to-Agent (A2A) protocol request.
You are being invoked by another AI agent or system. Respond concisely and precisely.
Focus on completing the requested task and returning structured, useful results.
Do not include pleasantries or conversational filler.`, name)
}
