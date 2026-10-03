package agent

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/sashabaranov/go-openai"
)

// toolOutcomeOptions carries the facts a dispatch path knows before
// post-processing. Everything else is shared by applyToolOutcome.
type toolOutcomeOptions struct {
	// Blocked marks results produced without running the handler (task rules,
	// duplicate calls, prechecks) for the action ledger and progress state.
	Blocked bool
	// NotExecuted marks a declared native call skipped after a circuit breaker.
	// It still receives one protocol result but no execution bookkeeping.
	NotExecuted bool
	// NativeResult persists the result as a tool-role message bound to the
	// call's NativeCallID; otherwise it is persisted as a user message.
	NativeResult bool
	// SupervisorRetry appends the controlled-retry report of a supervisor route.
	SupervisorRetry bool
	// RecoveryMessageStart is len(s.req.Messages) before dispatch. System
	// guidance added since then is detached and returned to the caller, which
	// appends it after every declared tool result.
	RecoveryMessageStart int
	ActionLedger         *agentActionLedger
	ToolAction           AgentActionEvent
	ExecutionTimeMs      int64
}

// applyToolOutcome is the single post-dispatch path for queued, primary and
// batched tool calls. It returns the model-facing result and the detached
// recovery guidance; callers own only the protocol message order.
func (s *agentLoopState) applyToolOutcome(ctx context.Context, tc ToolCall, rawResult string, opts toolOutcomeOptions) (string, []openai.ChatCompletionMessage) {
	cfg := s.runCfg.Config
	shortTermMem := s.runCfg.ShortTermMem
	sessionID := s.runCfg.SessionID
	broker := s.broker
	logger := s.currentLogger
	executed := !opts.Blocked && !opts.NotExecuted

	policyResult := toolExecutionResult{Content: rawResult, Failed: true, Outcome: ExecutionOutcomeFailed}
	if !opts.NotExecuted {
		policyResult = finalizeToolExecution(ctx, tc, rawResult, tc.GuardianBlocked, cfg, shortTermMem, sessionID,
			&s.recoveryState, &s.req, logger, s.telemetryScope, s.toolPromptVersion(tc.Action),
			opts.ExecutionTimeMs, s.runCfg)
	}
	content := policyResult.Content
	s.noteGameMakerToolProgress(policyResult.Failed, opts.Blocked)
	recordVirtualDesktopAppVerification(tc, content, policyResult.Failed, &s.recoveryState)
	deferredRecoveryMessages := detachNewSystemMessages(&s.req, opts.RecoveryMessageStart)
	invalidateTurnSnapshotAfterTool(s, tc, policyResult.Status != ToolResultSuccess)
	eventContent := policyResult.EventContent
	if eventContent == "" {
		eventContent = content
	}
	switch {
	case opts.NotExecuted:
		// A declared native call still needs one protocol result, but it was
		// never dispatched and must not create a tool-failure issue.
	case policyResult.Failed:
		recordToolFailureOperationalIssue(s.runCfg, tc, content, logger)
	case policyResult.Status == ToolResultSuccess:
		resolveToolFailureOperationalIssue(s.runCfg, tc, logger)
	}
	if opts.SupervisorRetry {
		content = appendControlledRetryReport(content, s.currentToolRoute, tc, s.initialUserMsg, policyResult.Failed)
	}
	if opts.Blocked {
		blockAgentToolAction(logger, opts.ActionLedger, opts.ToolAction, content)
	} else {
		completeAgentToolAction(logger, opts.ActionLedger, opts.ToolAction, policyResult, opts.ExecutionTimeMs)
	}
	trackActivityTool(&s.turnToolNames, &s.turnToolSummaries, tc.Action, content)
	recordPlanToolProgress(shortTermMem, sessionID, tc, content, logger)
	if policyResult.Status == ToolResultSuccess || policyResult.Status == ToolResultFailed {
		recordLearnedRuleOutcome(shortTermMem, s.flags.InjectedLearnedRules, tc.Action, policyResult.Failed, logger)
	}
	broker.Send("tool_output", content)
	emitMediaSSEEvents(broker, tc.Action, eventContent, cfg.Directories.DataDir)
	broker.Send("tool_end", tc.Action)
	s.lastActivity = time.Now()
	if tc.Todo != "" {
		s.sessionTodoList = string(tc.Todo)
		broker.Send("todo_update", s.sessionTodoList)
	}
	if tc.Action == "manage_plan" {
		emitSessionPlanUpdate(broker, shortTermMem, sessionID, logger)
	}
	if tc.Action == "manage_memory" || tc.Action == "core_memory" {
		s.coreMemDirty = true
	}
	if executed {
		if s.lastTool != "" && shortTermMem != nil {
			_ = shortTermMem.RecordToolTransition(s.lastTool, tc.Action)
		}
		s.lastTool = tc.Action
	}
	s.rememberRecentTool(tc.Action)
	if executed && s.personalityEnabled && shortTermMem != nil {
		s.flags.PersonalityLine = shortTermMem.GetPersonalityLineWithMeta(cfg.Personality.EngineV2, s.meta)
		s.flags.EmotionDescription = latestEmotionDescription(shortTermMem, s.emotionSynthesizer)
	}
	if executed && tc.NotifyOnCompletion {
		content = fmt.Sprintf(
			"[TOOL COMPLETION NOTIFICATION]\nAction: %s\nStatus: Completed\nTimestamp: %s\nOutput:\n%s",
			tc.Action,
			time.Now().Format(time.RFC3339),
			content,
		)
	}
	if executed && tc.Action == "execute_python" {
		if strings.Contains(content, "[EXECUTION ERROR]") || strings.Contains(content, "TIMEOUT") {
			s.flags.IsErrorState = true
			broker.Send("error_recovery", "Script error detected, retrying...")
		} else {
			s.flags.IsErrorState = false
		}
	}
	s.persistToolResult(tc, content, opts.NativeResult)
	return content, deferredRecoveryMessages
}

// persistToolResult stores one tool result in SQLite and, for the default
// session, in the in-memory history with the matching protocol role.
func (s *agentLoopState) persistToolResult(tc ToolCall, content string, native bool) {
	shortTermMem := s.runCfg.ShortTermMem
	if shortTermMem == nil {
		return
	}
	sessionID := s.runCfg.SessionID
	role := openai.ChatMessageRoleUser
	if native {
		role = openai.ChatMessageRoleTool
	}
	id, err := shortTermMem.InsertMessage(sessionID, role, content, false, true)
	if err != nil {
		s.currentLogger.Error("Failed to persist tool-result message", "tool", tc.Action, "error", err)
	}
	if sessionID != "default" || !ShouldAppendHistoryMessage(id, err) {
		return
	}
	if native {
		s.runCfg.HistoryManager.AddMessage(openai.ChatCompletionMessage{
			Role:       openai.ChatMessageRoleTool,
			Content:    content,
			ToolCallID: tc.NativeCallID,
		}, id, false, true)
		return
	}
	s.runCfg.HistoryManager.Add(openai.ChatMessageRoleUser, content, id, false, true)
}

// rememberRecentTool keeps the five most recently used distinct tool names.
func (s *agentLoopState) rememberRecentTool(action string) {
	for _, recent := range s.recentTools {
		if recent == action {
			return
		}
	}
	s.recentTools = append(s.recentTools, action)
	if len(s.recentTools) > 5 {
		s.recentTools = s.recentTools[len(s.recentTools)-5:]
	}
}
