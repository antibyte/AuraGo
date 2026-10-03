package agent

import (
	"aurago/internal/prompts"
	"context"
	"fmt"
	"strings"
	"time"

	"aurago/internal/i18n"
	"aurago/internal/memory"
	"aurago/internal/security"

	"github.com/sashabaranov/go-openai"
)

// processPendingToolCalls executes one queued pending tool call without invoking the LLM.
// It returns true if a pending call was processed and the caller should continue to the
// next loop iteration.
func processPendingToolCalls(s *agentLoopState, ctx context.Context, lastUserMsg string) bool {
	if len(s.pendingTCs) == 0 {
		return false
	}

	shortTermMem := s.runCfg.ShortTermMem
	historyManager := s.runCfg.HistoryManager
	sessionID := s.runCfg.SessionID
	broker := s.broker
	currentLogger := s.currentLogger

	dispatchCtx := s.makeDispatchContext(currentLogger)

	ptc := prepareToolCall(s.pendingTCs[0], dispatchCtx)
	s.pendingTCs = s.pendingTCs[1:]
	supervisorRouted := s.currentToolRoute.matches(ptc) && !s.currentToolRouteExecuted
	s.toolCallCount++
	s.progressFeedback.StepStarted()
	if isHomepageRuleTool(ptc.Action) {
		s.homepageUsedInChain = true
	}
	actionLedger, toolAction := beginAgentToolAction(s, ptc, agentActionTurnID(sessionID, len(s.req.Messages), s.toolCallCount))
	broker.Send("thinking", fmt.Sprintf("[%d] Running %s...", s.toolCallCount, ptc.Action))
	ptcJSON := ptc.RawJSON
	if ptcJSON == "" {
		ptcJSON = fmt.Sprintf(`{"action":"%s"}`, ptc.Action)
	}
	var id int64
	var idErr error
	if ptc.NativeCallID == "" {
		id, idErr = shortTermMem.InsertMessage(sessionID, openai.ChatMessageRoleAssistant, ptcJSON, false, true)
		if idErr != nil {
			currentLogger.Error("Failed to persist queued tool-call message", "error", idErr)
		}
		if sessionID == "default" && ShouldAppendHistoryMessage(id, idErr) {
			historyManager.Add(openai.ChatMessageRoleAssistant, ptcJSON, id, false, true)
		}
	}
	broker.Send("tool_call", ptcJSON)
	broker.Send("tool_start", ptc.Action)
	if ptc.Action != "" {
		s.sessionUsedTools[ptc.Action] = true
	}

	pResultContent := ""
	actionBlocked := false
	circuitBreakerOpen := false
	recoveryMessageStart := len(s.req.Messages)
	if ptc.PreparationError != "" {
		pResultContent = ptc.PreparationError
	} else if preload, blocked := ensureTaskRulesBeforeToolExecution(s, ptc, lastUserMsg); blocked {
		pResultContent = preload
		actionBlocked = true
	} else if s.blockDuplicateToolCall(ptc) {
		pResultContent = blockedToolOutputFromRequest(&s.req)
		actionBlocked = true
		circuitBreakerOpen = true
	} else if precheckResult, prechecked := precheckVirtualDesktopAppOpen(ptc, &s.recoveryState); prechecked {
		pResultContent = precheckResult
		actionBlocked = true
	} else if precheckResult, prechecked := precheckMessagingToolArgs(ptc, s.runCfg, sessionID); prechecked {
		pResultContent = precheckResult
	} else {
		toolAction = startAgentToolAction(currentLogger, actionLedger, toolAction)
		pResultContent = DispatchToolCall(ctx, &ptc, dispatchCtx, lastUserMsg)
	}
	pResultContent, deferredRecoveryMessages := s.applyToolOutcome(ctx, ptc, pResultContent, toolOutcomeOptions{
		Blocked:              actionBlocked,
		NativeResult:         ptc.NativeCallID != "",
		SupervisorRetry:      supervisorRouted && s.currentToolRoute.ExplicitRetry,
		RecoveryMessageStart: recoveryMessageStart,
		ActionLedger:         actionLedger,
		ToolAction:           toolAction,
		ExecutionTimeMs:      dispatchCtx.ExecutionTimeMs,
	})
	if ptc.NativeCallID != "" {
		// Match batched native tool handling: the originating assistant message with
		// all tool_calls is already in req.Messages from the first tool in the batch.
		s.req.Messages = append(s.req.Messages, openai.ChatCompletionMessage{
			Role:       openai.ChatMessageRoleTool,
			Content:    pResultContent,
			ToolCallID: ptc.NativeCallID,
		})
		if circuitBreakerOpen {
			appendSkippedNativeResults(s, shortTermMem, historyManager, sessionID, broker, notExecutedDueToCircuitBreakerResult())
		}
		s.req.Messages = append(s.req.Messages, deferredRecoveryMessages...)
	} else {
		s.req.Messages = append(s.req.Messages, deferredRecoveryMessages...)
		voiceModeActive := !s.voiceOutputSuppressed && (s.runCfg.VoiceOutputActive || GetVoiceMode()) && !isAutonomousAgentRun(s.runCfg, s.runCfg.SessionID) && !s.runCfg.IsMission
		followUpContent := toolResultFollowUpContent(ptc, pResultContent, voiceModeActive)
		s.req.Messages = append(s.req.Messages, openai.ChatCompletionMessage{Role: openai.ChatMessageRoleAssistant, Content: ptcJSON})
		s.req.Messages = append(s.req.Messages, openai.ChatCompletionMessage{Role: openai.ChatMessageRoleUser, Content: followUpContent})
	}
	if supervisorRouted {
		s.currentToolRouteExecuted = true
		s.pendingTCs = nil
		s.req.ToolChoice = "none"
		s.flags.CurrentToolRoute = ""
	}
	if circuitBreakerOpen {
		s.pendingTCs = nil
		s.restrictToolsAfterCircuitBreaker()
	}
	s.lastResponseWasTool = true
	return true
}

// executeAgentToolTurn runs a single tool-call turn including batched native calls.
// It returns the (possibly unchanged) response, an error, and a bool indicating
// whether the caller should continue to the next loop iteration.  If the bool is
// false and err is nil, the caller should return resp directly.
func executeAgentToolTurn(
	s *agentLoopState,
	ctx context.Context,
	tc ToolCall,
	resp openai.ChatCompletionResponse,
	content string,
	useNativePath bool,
	nativeAssistantMsg openai.ChatCompletionMessage,
	lastUserMsg string,
	triggerValue string,
	xmlFallbackHandledThisTurn bool,
) (openai.ChatCompletionResponse, error, bool) {
	tc = prepareToolCall(tc, s.makeDispatchContext(s.currentLogger))
	cfg := s.runCfg.Config
	shortTermMem := s.runCfg.ShortTermMem
	historyManager := s.runCfg.HistoryManager
	sessionID := s.runCfg.SessionID
	broker := s.broker
	currentLogger := s.currentLogger

	s.toolCallCount++
	s.progressFeedback.StepStarted()
	if isHomepageRuleTool(tc.Action) {
		s.homepageUsedInChain = true
	}
	actionLedger, toolAction := beginAgentToolAction(s, tc, agentActionTurnID(sessionID, len(s.req.Messages), s.toolCallCount))
	broker.Send("thinking", fmt.Sprintf("[%d] Running %s...", s.toolCallCount, tc.Action))

	// Persist tool call to history: native path synthesizes a text representation
	histContent := content
	histContent = security.StripThinkingTags(histContent)

	if !useNativePath {
		if jsonIdx := strings.Index(histContent, "{"); jsonIdx > 0 {
			textPart := strings.TrimSpace(histContent[:jsonIdx])
			if textPart != "" {
				histContent = textPart
			}
		}
	}
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(histContent)), "minimax:tool_call") {
		histContent = fmt.Sprintf(`{"action":"%s"}`, tc.Action)
	}

	isMsgInternal := true

	if useNativePath && histContent == "" && len(nativeAssistantMsg.ToolCalls) > 0 {
		nc := nativeAssistantMsg.ToolCalls[0]
		histContent = fmt.Sprintf("{\"action\": \"%s\"}", nc.Function.Name)
		if nc.Function.Arguments != "" && len(nc.Function.Arguments) > 2 {
			args := strings.TrimSpace(nc.Function.Arguments)
			if strings.HasPrefix(args, "{") && strings.HasSuffix(args, "}") {
				inner := args[1 : len(args)-1]
				if inner != "" {
					histContent = fmt.Sprintf("{\"action\": \"%s\", %s}", nc.Function.Name, inner)
				}
			}
		}
	}
	id, err := shortTermMem.InsertMessage(sessionID, openai.ChatMessageRoleAssistant, histContent, false, isMsgInternal)
	if err != nil {
		currentLogger.Error("Failed to persist tool-call message to SQLite", "error", err)
	}
	if sessionID == "default" && ShouldAppendHistoryMessage(id, err) {
		if useNativePath {
			nativeMsg := nativeAssistantMsg
			if nativeMsg.Role == "" {
				nativeMsg.Role = openai.ChatMessageRoleAssistant
			}
			historyManager.AddMessage(nativeMsg, id, false, isMsgInternal)
		} else {
			historyManager.Add(openai.ChatMessageRoleAssistant, histContent, id, false, isMsgInternal)
		}
	}

	sseToolContent := histContent
	if !useNativePath {
		if tc.RawJSON != "" {
			sseToolContent = tc.RawJSON
		} else {
			sseToolContent = fmt.Sprintf(`{"action":"%s"}`, tc.Action)
		}
	}
	broker.Send("tool_call", sseToolContent)
	broker.Send("tool_start", tc.Action)

	if tc.Action != "" {
		s.sessionUsedTools[tc.Action] = true
	}

	preloadedRules, rulesBlocked := tc.PreparationError, tc.PreparationError != ""
	if !rulesBlocked {
		preloadedRules, rulesBlocked = ensureTaskRulesBeforeToolExecution(s, tc, lastUserMsg)
	}
	if rulesBlocked {
		resultContent := preloadedRules
		toolAction = blockAgentToolAction(currentLogger, actionLedger, toolAction, resultContent)
		if useNativePath && tc.NativeCallID != "" {
			s.req.Messages = append(s.req.Messages, nativeAssistantMsg)
			s.req.Messages = append(s.req.Messages, openai.ChatCompletionMessage{
				Role:       openai.ChatMessageRoleTool,
				Content:    resultContent,
				ToolCallID: tc.NativeCallID,
			})
			resultID, resultErr := shortTermMem.InsertMessage(sessionID, openai.ChatMessageRoleTool, resultContent, false, true)
			if resultErr != nil {
				currentLogger.Error("Failed to persist task-rule tool-result message", "error", resultErr)
			}
			if sessionID == "default" && ShouldAppendHistoryMessage(resultID, resultErr) {
				historyManager.AddMessage(openai.ChatCompletionMessage{
					Role:       openai.ChatMessageRoleTool,
					Content:    resultContent,
					ToolCallID: tc.NativeCallID,
				}, resultID, false, true)
			}
		} else {
			resultID, resultErr := shortTermMem.InsertMessage(sessionID, openai.ChatMessageRoleUser, resultContent, false, true)
			if resultErr != nil {
				currentLogger.Error("Failed to persist task-rule preload message", "error", resultErr)
			}
			if sessionID == "default" && ShouldAppendHistoryMessage(resultID, resultErr) {
				historyManager.Add(openai.ChatMessageRoleUser, resultContent, resultID, false, true)
			}
			s.req.Messages = append(s.req.Messages, openai.ChatCompletionMessage{Role: openai.ChatMessageRoleAssistant, Content: histContent})
			s.req.Messages = append(s.req.Messages, openai.ChatCompletionMessage{Role: openai.ChatMessageRoleUser, Content: resultContent})
		}
		broker.Send("tool_output", resultContent)
		broker.Send("tool_end", tc.Action)
		s.lastResponseWasTool = true
		return resp, nil, true
	}

	recoveryMessageStart := len(s.req.Messages)
	if s.blockDuplicateToolCall(tc) {
		syntheticResult := blockedToolOutputFromRequest(&s.req)
		deferredRecoveryMessages := detachNewSystemMessages(&s.req, recoveryMessageStart)
		toolAction = blockAgentToolAction(currentLogger, actionLedger, toolAction, syntheticResult)
		if useNativePath && tc.NativeCallID != "" {
			s.req.Messages = append(s.req.Messages, nativeAssistantMsg)
			s.req.Messages = append(s.req.Messages, openai.ChatCompletionMessage{
				Role:       openai.ChatMessageRoleTool,
				Content:    syntheticResult,
				ToolCallID: tc.NativeCallID,
			})
			resultID, resultErr := shortTermMem.InsertMessage(sessionID, openai.ChatMessageRoleTool, syntheticResult, false, true)
			if resultErr != nil {
				currentLogger.Error("Failed to persist duplicate-tool synthetic result", "error", resultErr)
			}
			if sessionID == "default" && ShouldAppendHistoryMessage(resultID, resultErr) {
				historyManager.AddMessage(openai.ChatCompletionMessage{
					Role:       openai.ChatMessageRoleTool,
					Content:    syntheticResult,
					ToolCallID: tc.NativeCallID,
				}, resultID, false, true)
			}
			appendSkippedNativeResults(s, shortTermMem, historyManager, sessionID, broker, notExecutedDueToCircuitBreakerResult())
		} else {
			resultID, resultErr := shortTermMem.InsertMessage(sessionID, openai.ChatMessageRoleUser, syntheticResult, false, true)
			if resultErr != nil {
				currentLogger.Error("Failed to persist duplicate-tool synthetic result", "error", resultErr)
			}
			if sessionID == "default" && ShouldAppendHistoryMessage(resultID, resultErr) {
				historyManager.Add(openai.ChatMessageRoleUser, syntheticResult, resultID, false, true)
			}
			s.req.Messages = append(s.req.Messages,
				openai.ChatCompletionMessage{Role: openai.ChatMessageRoleAssistant, Content: histContent},
				openai.ChatCompletionMessage{Role: openai.ChatMessageRoleUser, Content: syntheticResult},
			)
		}
		s.pendingTCs = nil
		s.restrictToolsAfterCircuitBreaker()
		s.req.Messages = append(s.req.Messages, deferredRecoveryMessages...)
		broker.Send("tool_output", syntheticResult)
		broker.Send("tool_end", tc.Action)
		s.lastResponseWasTool = false
		return resp, nil, true
	}

	if tc.Action == "execute_python" {
		s.flags.RequiresCoding = true
		broker.Send("coding", i18n.T(cfg.Server.UILanguage, "backend.stream_coding_executing"))
	}

	if (tc.Action == "co_agent" || tc.Action == "co_agents") &&
		(tc.Operation == "spawn" || tc.Operation == "start" || tc.Operation == "create") {
		taskPreview := tc.Task
		if taskPreview == "" {
			taskPreview = tc.Content
		}
		if len(taskPreview) > 80 {
			taskPreview = taskPreview[:80] + "…"
		}
		broker.Send("co_agent_spawn", taskPreview)
	}

	dispatchCtx := s.makeDispatchContext(currentLogger)
	var resultContent string
	primaryBlocked := false
	if precheckResult, prechecked := precheckVirtualDesktopAppOpen(tc, &s.recoveryState); prechecked {
		resultContent = precheckResult
		primaryBlocked = true
	} else if precheckResult, prechecked := precheckMessagingToolArgs(tc, s.runCfg, sessionID); prechecked {
		resultContent = precheckResult
	} else {
		toolAction = startAgentToolAction(currentLogger, actionLedger, toolAction)
		resultContent = DispatchToolCall(ctx, &tc, dispatchCtx, lastUserMsg)
	}
	resultContent, primaryRecoveryMessages := s.applyToolOutcome(ctx, tc, resultContent, toolOutcomeOptions{
		Blocked:              primaryBlocked,
		NativeResult:         useNativePath,
		RecoveryMessageStart: recoveryMessageStart,
		ActionLedger:         actionLedger,
		ToolAction:           toolAction,
		ExecutionTimeMs:      dispatchCtx.ExecutionTimeMs,
	})
	s.lastResponseWasTool = true

	if useNativePath {
		s.req.Messages = append(s.req.Messages, nativeAssistantMsg)
		s.req.Messages = append(s.req.Messages, openai.ChatCompletionMessage{
			Role:       openai.ChatMessageRoleTool,
			Content:    resultContent,
			ToolCallID: tc.NativeCallID,
		})

		// Recovery guidance follows every declared tool result (AGENTS.md
		// message-order contract), including the primary call's guidance.
		deferredRecoveryMessages := primaryRecoveryMessages
		circuitBreakerOpen := false
		nativeDispatchCtx := s.makeDispatchContext(currentLogger)
		for len(s.pendingTCs) > 0 && s.pendingTCs[0].NativeCallID != "" {
			if finishCompletedRun(s) {
				break
			}

			btc := prepareToolCall(s.pendingTCs[0], nativeDispatchCtx)
			s.pendingTCs = s.pendingTCs[1:]
			s.toolCallCount++
			s.progressFeedback.StepStarted()
			if isHomepageRuleTool(btc.Action) {
				s.homepageUsedInChain = true
			}
			batchedLedger, batchedAction := beginAgentToolAction(s, btc, agentActionTurnID(sessionID, len(s.req.Messages), s.toolCallCount))
			broker.Send("thinking", fmt.Sprintf("[%d] Running %s (batched)...", s.toolCallCount, btc.Action))
			broker.Send("tool_start", btc.Action)
			if btc.Action != "" {
				s.sessionUsedTools[btc.Action] = true
			}

			bResult := ""
			batchedBlocked := false
			notExecuted := false
			recoveryMessageStart := len(s.req.Messages)
			if circuitBreakerOpen {
				bResult = notExecutedDueToCircuitBreakerResult()
				batchedBlocked = true
				notExecuted = true
			} else if btc.PreparationError != "" {
				bResult = btc.PreparationError
			} else if preload, blocked := ensureTaskRulesBeforeToolExecution(s, btc, lastUserMsg); blocked {
				bResult = preload
				batchedBlocked = true
			} else if s.blockDuplicateToolCall(btc) {
				bResult = blockedToolOutputFromRequest(&s.req)
				batchedBlocked = true
				circuitBreakerOpen = true
			} else if precheckResult, prechecked := precheckVirtualDesktopAppOpen(btc, &s.recoveryState); prechecked {
				bResult = precheckResult
				batchedBlocked = true
			} else if precheckResult, prechecked := precheckMessagingToolArgs(btc, s.runCfg, sessionID); prechecked {
				bResult = precheckResult
			} else {
				batchedAction = startAgentToolAction(currentLogger, batchedLedger, batchedAction)
				bResult = DispatchToolCall(ctx, &btc, nativeDispatchCtx, lastUserMsg)
			}
			bResult, batchRecoveryMessages := s.applyToolOutcome(ctx, btc, bResult, toolOutcomeOptions{
				Blocked:              batchedBlocked,
				NotExecuted:          notExecuted,
				NativeResult:         true,
				RecoveryMessageStart: recoveryMessageStart,
				ActionLedger:         batchedLedger,
				ToolAction:           batchedAction,
				ExecutionTimeMs:      nativeDispatchCtx.ExecutionTimeMs,
			})
			deferredRecoveryMessages = append(deferredRecoveryMessages, batchRecoveryMessages...)

			s.req.Messages = append(s.req.Messages, openai.ChatCompletionMessage{
				Role:       openai.ChatMessageRoleTool,
				Content:    bResult,
				ToolCallID: btc.NativeCallID,
			})
		}
		if circuitBreakerOpen {
			s.pendingTCs = nil
			s.restrictToolsAfterCircuitBreaker()
		}
		s.req.Messages = append(s.req.Messages, deferredRecoveryMessages...)
	} else {
		// Text-mode order is unchanged: guidance precedes the assistant/user pair.
		s.req.Messages = append(s.req.Messages, primaryRecoveryMessages...)
		if !xmlFallbackHandledThisTurn {
			s.req.Messages = append(s.req.Messages, openai.ChatCompletionMessage{Role: openai.ChatMessageRoleAssistant, Content: content})
		}
		voiceModeActive := !s.voiceOutputSuppressed && (s.runCfg.VoiceOutputActive || GetVoiceMode()) && !isAutonomousAgentRun(s.runCfg, s.runCfg.SessionID) && !s.runCfg.IsMission
		followUpContent := toolResultFollowUpContent(tc, resultContent, voiceModeActive)
		s.req.Messages = append(s.req.Messages, openai.ChatCompletionMessage{Role: openai.ChatMessageRoleUser, Content: followUpContent})
	}

	if finishCompletedRun(s) {
		return openai.ChatCompletionResponse{}, ctx.Err(), false
	}
	select {
	case <-time.After(time.Duration(cfg.Agent.StepDelaySeconds) * time.Second):
		return resp, nil, true
	case <-ctx.Done():
		return resp, ctx.Err(), false
	}
}

func notExecutedDueToCircuitBreakerResult() string {
	return `{"status":"error","code":"not_executed_due_to_circuit_breaker","message":"Tool call was declared but not executed because the duplicate-call circuit breaker terminated this tool chain."}`
}

func finishCompletedRun(s *agentLoopState) bool {
	if s.runCfg.RunComplete == nil || !s.runCfg.RunComplete() {
		return false
	}
	appendSkippedNativeResults(s, s.runCfg.ShortTermMem, s.runCfg.HistoryManager, s.runCfg.SessionID, s.broker,
		`{"status":"skipped","code":"not_executed_after_run_completion","message":"The server completed this phase; remaining calls were not executed."}`)
	s.pendingTCs = nil
	return true
}

func appendSkippedNativeResults(s *agentLoopState, stm *memory.SQLiteMemory, history *memory.HistoryManager, sessionID string, broker FeedbackBroker, content string) {
	for len(s.pendingTCs) > 0 && strings.TrimSpace(s.pendingTCs[0].NativeCallID) != "" {
		tc := s.pendingTCs[0]
		s.pendingTCs = s.pendingTCs[1:]
		resultID, resultErr := stm.InsertMessage(sessionID, openai.ChatMessageRoleTool, content, false, true)
		if resultErr != nil && s.currentLogger != nil {
			s.currentLogger.Error("Failed to persist skipped tool result", "error", resultErr)
		}
		if sessionID == "default" && ShouldAppendHistoryMessage(resultID, resultErr) {
			history.AddMessage(openai.ChatCompletionMessage{Role: openai.ChatMessageRoleTool, Content: content, ToolCallID: tc.NativeCallID}, resultID, false, true)
		}
		s.req.Messages = append(s.req.Messages, openai.ChatCompletionMessage{
			Role: openai.ChatMessageRoleTool, Content: content, ToolCallID: tc.NativeCallID,
		})
		broker.Send("tool_start", tc.Action)
		broker.Send("tool_output", content)
		broker.Send("tool_end", tc.Action)
	}
}

func detachNewSystemMessages(req *openai.ChatCompletionRequest, start int) []openai.ChatCompletionMessage {
	if req == nil || start < 0 || start >= len(req.Messages) {
		return nil
	}
	tail := req.Messages[start:]
	deferred := make([]openai.ChatCompletionMessage, 0, len(tail))
	kept := make([]openai.ChatCompletionMessage, 0, len(tail))
	for _, msg := range tail {
		if msg.Role == openai.ChatMessageRoleSystem {
			deferred = append(deferred, msg)
			continue
		}
		kept = append(kept, msg)
	}
	req.Messages = append(req.Messages[:start], kept...)
	return deferred
}

func (s *agentLoopState) toolPromptVersion(tool string) string {
	if id := s.promptGuideVersions[prompts.ToolManualID(tool)]; id != "" {
		return id
	}
	return "unobserved"
}
