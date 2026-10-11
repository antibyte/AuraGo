package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"aurago/internal/agent"
	"aurago/internal/config"
	"aurago/internal/cydcreator"
	"aurago/internal/i18n"
	"aurago/internal/llm"
	"aurago/internal/security"

	"github.com/sashabaranov/go-openai"
)

// cydCreatorAgentRunner runs one CYD Creator job phase as an isolated agent
// loop (pattern gameMakerAgentRunner.RunGameMakerJob): dedicated session,
// prepared prompt profile per phase, phase-restricted tools, server-owned
// round ends and a private conversation checkpoint for resume.
type cydCreatorAgentRunner struct {
	server  *Server
	service *cydcreator.Service
}

// cydCreatorConversation is the private checkpoint stored in cc_agent_context.
type cydCreatorConversation struct {
	ProviderID string                         `json:"provider_id"`
	Model      string                         `json:"model"`
	Messages   []openai.ChatCompletionMessage `json:"messages"`
}

// cydCreatorJobContext is the untrusted data block of the phase request.
func cydCreatorJobContext(run cydcreator.JobRun, toolLimit int) map[string]any {
	data := map[string]any{
		"job_id": run.Job.ID, "phase": run.Phase, "mode": run.Job.Mode, "resumed": run.Resumed,
		"project": map[string]any{"id": run.Project.ID, "name": run.Project.Name, "board": run.Project.Board, "theme": run.Project.Theme},
	}
	if run.Plan != nil {
		data["plan"] = run.Plan
	}
	if run.UserChanges != nil {
		data["user_changes"] = run.UserChanges
	}
	if len(run.Attachments) > 0 {
		data["attachments"] = run.Attachments
	}
	if run.Phase == cydcreator.PhaseCoding || run.Phase == cydcreator.PhaseRepairing {
		data["hooks"] = run.Hooks
	}
	if run.Phase == cydcreator.PhaseRepairing {
		data["repair"] = map[string]any{"attempt": run.RepairAttempt, "max_attempts": run.MaxRepairAttempts,
			"error_code": run.BuildErrorCode, "diagnostics": run.Diagnostics}
	}
	budget := map[string]any{"tool_calls": toolLimit}
	if run.TimeRemaining > 0 {
		// The job timeout is a timer (no ctx deadline): the orchestrator passes
		// what is left of the segment, rounded down to 30 s.
		budget["seconds_remaining"] = int(run.TimeRemaining.Seconds()) / 30 * 30
	}
	data["budget"] = budget
	return data
}

// conversation loads the private checkpoint of the job (or of the draft it
// resumes) and returns the bounded request view plus the checkpoint hook.
// Only user/assistant/tool messages are kept; generated system prompts are
// rebuilt from the current profile and never restored.
func (r *cydCreatorAgentRunner) conversation(ctx context.Context, cfg *config.Config, run cydcreator.JobRun) ([]openai.ChatCompletionMessage, func([]openai.ChatCompletionMessage) error, error) {
	raw, err := r.service.LoadAgentContext(ctx, run.Job.ID)
	if err != nil {
		return nil, nil, err
	}
	var saved cydCreatorConversation
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &saved); err != nil {
			// A damaged checkpoint must not block every resume: start fresh.
			r.logger().Warn("CYD Creator conversation checkpoint is unreadable; continuing without it", "job_id", run.Job.ID, "error", err)
			saved = cydCreatorConversation{}
		}
	}
	history := saved.Messages
	if saved.ProviderID != cfg.LLM.Provider || saved.Model != cfg.LLM.Model {
		// Provider-specific reasoning cannot be replayed to another route.
		for i := range history {
			history[i].ReasoningContent = ""
		}
	}
	view := gameMakerRequestView(history)
	// The archive continues from the request view, not the whole saved
	// history, so it does not grow with every phase and resume; each save is
	// also capped (cydCreatorTrimConversation).
	durable := append([]openai.ChatCompletionMessage(nil), view...)
	seen := map[string]bool{}
	phaseUserKey := ""
	checkpoint := func(messages []openai.ChatCompletionMessage) error {
		if phaseUserKey == "" {
			for i := len(messages) - 1; i >= 0; i-- {
				if messages[i].Role == openai.ChatMessageRoleUser {
					phaseUserKey = gameMakerMessageKey(messages[i])
					break
				}
			}
		}
		start := -1
		for i := len(messages) - 1; i >= 0; i-- {
			if gameMakerMessageKey(messages[i]) == phaseUserKey {
				start = i
				break
			}
		}
		if start < 0 {
			return nil
		}
		var private []openai.ChatCompletionMessage
		for _, msg := range messages[start:] {
			if msg.Role != openai.ChatMessageRoleUser && msg.Role != openai.ChatMessageRoleAssistant && msg.Role != openai.ChatMessageRoleTool {
				continue
			}
			msg.Content = security.Scrub(msg.Content)
			msg.ReasoningContent = security.Scrub(msg.ReasoningContent)
			msg.ToolCalls = append([]openai.ToolCall(nil), msg.ToolCalls...)
			for i := range msg.ToolCalls {
				msg.ToolCalls[i].Function.Arguments = security.Scrub(msg.ToolCalls[i].Function.Arguments)
			}
			private = append(private, msg)
		}
		private, _ = agent.SanitizeToolMessages(private)
		for _, msg := range private {
			key := gameMakerMessageKey(msg)
			if seen[key] {
				continue
			}
			seen[key] = true
			durable = append(durable, msg)
		}
		durable = cydCreatorTrimConversation(durable, cydCreatorMaxConversationBytes)
		data, err := json.Marshal(cydCreatorConversation{ProviderID: cfg.LLM.Provider, Model: cfg.LLM.Model, Messages: durable})
		if err != nil {
			return err
		}
		// Cancellation and timeout must still leave the last checkpoint.
		saveCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		return r.service.SaveAgentContext(saveCtx, run.Job.ID, data)
	}
	return view, checkpoint, nil
}

// cydCreatorMaxConversationBytes caps the messages of one private checkpoint
// (the service refuses more than 16 MiB, and a refused checkpoint fails the
// run). A resume only ever sends the newest rounds (gameMakerRequestView).
const cydCreatorMaxConversationBytes = 2 << 20

// cydCreatorTrimConversation drops the oldest messages until the encoded
// messages fit maxBytes. A dropped assistant tool call takes its results along,
// so the archive never starts with an orphaned tool result.
func cydCreatorTrimConversation(messages []openai.ChatCompletionMessage, maxBytes int) []openai.ChatCompletionMessage {
	sizes := make([]int, len(messages))
	total := 2 // the enclosing brackets
	for i, msg := range messages {
		data, _ := json.Marshal(msg)
		sizes[i] = len(data) + 1 // plus the separating comma
		total += sizes[i]
	}
	if total <= maxBytes {
		return messages
	}
	start := 0
	for total > maxBytes && start < len(messages) {
		total -= sizes[start]
		start++
		for start < len(messages) && messages[start].Role == openai.ChatMessageRoleTool {
			total -= sizes[start]
			start++
		}
	}
	trimmed, _ := agent.SanitizeToolMessages(append([]openai.ChatCompletionMessage(nil), messages[start:]...))
	return trimmed
}

// cydCreatorAgentFailure is a failed agent loop as the job chat reports it
// ("The agent stopped: ..."). Provider errors can carry endpoint URLs, LAN
// addresses, account ids in URL paths and provider response bodies, and the
// chat reaches read-only sessions: Error() is a generic category text, the
// raw error goes to the server log and stays in the chain (Unwrap).
type cydCreatorAgentFailure struct {
	public string
	err    error
}

func (e cydCreatorAgentFailure) Error() string { return e.public }
func (e cydCreatorAgentFailure) Unwrap() error { return e.err }

// cydCreatorAgentFailureText picks the reason by category: the English texts
// of the normal chat (chatCompletionErrorMessage), plus an "unreachable" text
// for transport failures and the quota text for rate limits.
func cydCreatorAgentFailureText(err error) string {
	category := llm.ClassifyError(err)
	switch {
	case agent.IsToolLimitFinalResponseInvalid(err), agent.IsContextBudgetExceeded(err):
	case category == llm.ErrCategoryTemporaryTransport:
		return "The LLM provider could not be reached or did not answer in time. Check that it is running and reachable, then try again."
	case category == llm.ErrCategoryRateLimit:
		return i18n.T("en", "backend.handler_llm_quota_error")
	}
	return chatCompletionErrorMessage("en", err)
}

func (r *cydCreatorAgentRunner) logger() *slog.Logger {
	if r.server != nil && r.server.Logger != nil {
		return r.server.Logger
	}
	return slog.Default()
}

// RunCYDJob implements cydcreator.Runner.
func (r *cydCreatorAgentRunner) RunCYDJob(ctx context.Context, run cydcreator.JobRun) error {
	s := r.server
	if s == nil || s.Cfg == nil || s.LLMClient == nil {
		return cydcreator.NewError(cydcreator.CodeDisabled, "the CYD Creator agent has no LLM configured", nil)
	}
	ctx, release, err := s.beginDesktopRun(ctx)
	if err != nil {
		return err
	}
	defer release()
	s.CfgMu.RLock()
	cfg := *s.Cfg
	s.CfgMu.RUnlock()

	client := s.LLMClient
	if providerID := strings.TrimSpace(run.Model); providerID != "" {
		provider := cfg.FindProvider(providerID)
		if provider == nil {
			return cydcreator.NewError(cydcreator.CodeDisabled, fmt.Sprintf("the CYD Creator provider %q is not configured", providerID), nil)
		}
		cfg.LLM.Provider = provider.ID
		cfg.LLM.ProviderType = provider.Type
		cfg.LLM.BaseURL = provider.BaseURL
		cfg.LLM.APIKey = provider.APIKey
		cfg.LLM.AccountID = provider.AccountID
		cfg.LLM.Model = provider.Model
		client = llm.WrapOpenAIClient(llm.NewClientFromProviderWithConfig(&cfg, provider.Type, provider.BaseURL, provider.APIKey, provider.AccountID))
	}
	if strings.TrimSpace(cfg.LLM.Model) == "" {
		return cydcreator.NewError(cydcreator.CodeDisabled, "the CYD Creator model is empty", nil)
	}
	cfg.LLM.UseNativeFunctions = true
	profile, err := cydCreatorPromptProfile(run.Phase)
	if err != nil {
		return err
	}
	history, checkpoint, err := r.conversation(ctx, &cfg, run)
	if err != nil {
		return err
	}
	sessionID := cydCreatorAgentSessionPrefix + run.Job.ID
	runCfg := buildDesktopRunConfigForSession(s, &cfg, client, sessionID, "cyd_creator")
	runCfg.TrustedPromptAddenda = nil
	runCfg.PreparedPrompt = profile
	runCfg.NativeToolSchemas = profile.Tools()
	runCfg.AllowedTools = []string{}
	for _, definition := range runCfg.NativeToolSchemas {
		runCfg.AllowedTools = append(runCfg.AllowedTools, definition.Function.Name)
	}
	runCfg.UserIntent = run.Prompt
	runCfg.Checkpoint = checkpoint
	runCfg.PreserveReasoning = true
	runCfg.StableSystemPrompt = true
	runCfg.RequireCompleteStream = true
	runCfg.RetryStreamIdle = true
	// Like Game Maker: a slow model that keeps streaming may outlast the fixed
	// per-call timeout (never past the job's own budget).
	runCfg.StreamThroughput = &gameMakerStreamThroughput
	// Same server-owned budget as Game Maker: max(40, ceil(1.25 × circuit breaker)).
	runCfg.ToolCallLimit = gameMakerToolCallLimit(cfg.CircuitBreaker.MaxToolCalls)
	runCfg.SuppressTurnSideEffects = true
	runCfg.IsMission = true
	runCfg.VoiceOutputActive = false
	switch run.Phase {
	case cydcreator.PhasePlanning:
		runCfg.RunComplete = func() bool { return r.service.PlanAccepted(run.Job.ID) }
	case cydcreator.PhaseCoding, cydcreator.PhaseRepairing:
		runCfg.RunComplete = func() bool { return r.service.JobFinished(run.Job.ID) }
	}
	// encoding/json escapes angle brackets, so data cannot close the wrapper.
	data, _ := json.Marshal(cydCreatorJobContext(run, runCfg.ToolCallLimit))
	req := openai.ChatCompletionRequest{
		Model: cfg.LLM.Model,
		Messages: append(history, openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleUser,
			Content: run.Prompt + "\n\nJob context (data):\n<external_data>\n" + string(data) + "\n</external_data>",
		}),
		Stream: true,
	}
	defer func() {
		if s.ShortTermMem != nil {
			if err := s.ShortTermMem.PurgeChatSession(sessionID); err != nil && s.Logger != nil {
				s.Logger.Warn("Failed to purge transient CYD Creator agent session", "session_id", sessionID, "error", err)
			}
		}
	}()
	slog.Info("cyd creator job phase starting", "job_id", run.Job.ID, "phase", run.Phase, "provider_id", cfg.LLM.Provider,
		"model", cfg.LLM.Model, "tool_limit", runCfg.ToolCallLimit)
	broker := &cydCreatorBroker{}
	callCtx := llm.WithStreamAttemptTimeout(cydcreator.WithJobContext(ctx, run.Job.ID), time.Duration(cfg.CircuitBreaker.LLMTimeoutSeconds)*time.Second)
	response, err := agent.ExecuteAgentLoop(callCtx, req, runCfg, true, broker)
	if err != nil {
		if agent.IsToolLimitFinalResponseInvalid(err) && ctx.Err() == nil && run.Phase != cydcreator.PhasePlanning {
			// The extra call stays unexecuted; the orchestrator builds what was saved.
			return nil
		}
		// The raw error (provider URL, address, response body) stays in the
		// server log; the job chat gets the generic reason.
		r.logger().Warn("CYD Creator agent loop failed", "job_id", run.Job.ID, "phase", run.Phase, "error", err)
		return cydCreatorAgentFailure{public: cydCreatorAgentFailureText(err), err: err}
	}
	answer := ""
	if len(response.Choices) > 0 {
		answer = strings.TrimSpace(response.Choices[0].Message.Content)
	}
	if answer == "" {
		answer = strings.TrimSpace(broker.text())
	}
	if answer != "" && answer != "[Empty Response]" {
		r.service.NoteAgentReply(run.Job.ID, answer)
	}
	return nil
}

// cydCreatorBroker keeps only the final answer event. Reasoning, stream
// deltas, tool arguments and token data never leave the run; progress reaches
// the UI only through server-written job_step events.
type cydCreatorBroker struct {
	mu       sync.Mutex
	response strings.Builder
}

func (b *cydCreatorBroker) Send(event, message string) {
	if event != "final_response" || strings.TrimSpace(message) == "" {
		return
	}
	b.mu.Lock()
	if b.response.Len() == 0 {
		b.response.WriteString(message)
	}
	b.mu.Unlock()
}

func (b *cydCreatorBroker) SendJSON(string) {}

func (b *cydCreatorBroker) SendLLMStreamDelta(string, string, string, int, string) {}

func (b *cydCreatorBroker) SendLLMStreamDone(string) {}

func (b *cydCreatorBroker) SendTokenUpdate(int, int, int, int, int, bool, bool, string) {}

func (b *cydCreatorBroker) SendThinkingBlock(string, string, string) {}

func (b *cydCreatorBroker) text() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.response.String()
}
