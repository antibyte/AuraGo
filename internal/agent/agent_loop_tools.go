package agent

import (
	"context"
	"fmt"
	"strings"
	"time"

	"aurago/internal/i18n"
	"aurago/internal/memory"
	"aurago/internal/security"
	"aurago/internal/services/optimizer"

	"github.com/sashabaranov/go-openai"
)

// processPendingToolCalls executes one queued pending tool call without invoking the LLM.
// It returns true if a pending call was processed and the caller should continue to the
// next loop iteration.
func processPendingToolCalls(s *agentLoopState, ctx context.Context, lastUserMsg string) bool {
	if len(s.pendingTCs) == 0 {
		return false
	}

	cfg := s.runCfg.Config
	shortTermMem := s.runCfg.ShortTermMem
	historyManager := s.runCfg.HistoryManager
	sessionID := s.runCfg.SessionID
	broker := s.broker
	currentLogger := s.currentLogger

	dispatchCtx := s.makeDispatchContext(currentLogger)
	if s.helperManager != nil && len(s.pendingSummaryBatch) == 0 && !s.runCfg.IsMission && !s.runCfg.IsCoAgent && !isAutonomousAgentRun(s.runCfg, s.runCfg.SessionID) {
		s.pendingSummaryBatch = maybeBuildPendingSummaryBatch(ctx, s.pendingTCs, dispatchCtx, s.helperManager, lastUserMsg)
	}

	ptc := s.pendingTCs[0]
	s.pendingTCs = s.pendingTCs[1:]
	supervisorRouted := s.currentToolRoute.matches(ptc) && !s.currentToolRouteExecuted
	s.toolCallCount++
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
	if preload, blocked := ensureTaskRulesBeforeToolExecution(s, ptc, lastUserMsg); blocked {
		pResultContent = preload
		actionBlocked = true
	} else if precomputed, ok := s.pendingSummaryBatch[pendingSummaryBatchKey(ptc)]; ok {
		pResultContent = precomputed
		delete(s.pendingSummaryBatch, pendingSummaryBatchKey(ptc))
		if len(s.pendingSummaryBatch) == 0 {
			s.pendingSummaryBatch = nil
		}
	} else if s.recoveryState.handleDuplicateToolCall(ptc, &s.req, currentLogger, s.telemetryScope) {
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
	policyResult := finalizeToolExecution(ctx, ptc, pResultContent, ptc.GuardianBlocked, cfg, shortTermMem, sessionID,
		&s.recoveryState, &s.req, currentLogger, s.telemetryScope, optimizer.GetToolPromptVersion(ptc.Action),
		dispatchCtx.ExecutionTimeMs, s.runCfg)
	pResultContent = policyResult.Content
	recordVirtualDesktopAppVerification(ptc, pResultContent, policyResult.Failed, &s.recoveryState)
	deferredRecoveryMessages := detachNewSystemMessages(&s.req, recoveryMessageStart)
	invalidateTurnSnapshotAfterTool(s, ptc, policyResult.Failed)
	pEventContent := policyResult.EventContent
	if pEventContent == "" {
		pEventContent = pResultContent
	}
	if policyResult.Failed {
		recordToolFailureOperationalIssue(s.runCfg, ptc, pResultContent, currentLogger)
	} else {
		resolveToolFailureOperationalIssue(s.runCfg, ptc, currentLogger)
	}
	if supervisorRouted && s.currentToolRoute.ExplicitRetry {
		pResultContent = appendControlledRetryReport(pResultContent, s.currentToolRoute, ptc, s.initialUserMsg, policyResult.Failed)
	}
	if actionBlocked {
		toolAction = blockAgentToolAction(currentLogger, actionLedger, toolAction, pResultContent)
	} else {
		toolAction = completeAgentToolAction(currentLogger, actionLedger, toolAction, policyResult, dispatchCtx.ExecutionTimeMs)
	}
	trackActivityTool(&s.turnToolNames, &s.turnToolSummaries, ptc.Action, pResultContent)
	recordPlanToolProgress(shortTermMem, sessionID, ptc, pResultContent, currentLogger)
	recordLearnedRuleOutcome(shortTermMem, s.flags.InjectedLearnedRules, ptc.Action, policyResult.Failed, currentLogger)
	broker.Send("tool_output", pResultContent)
	emitMediaSSEEvents(broker, ptc.Action, pEventContent, cfg.Directories.DataDir)
	broker.Send("tool_end", ptc.Action)
	s.lastActivity = time.Now()
	if ptc.Todo != "" {
		s.sessionTodoList = string(ptc.Todo)
		broker.Send("todo_update", s.sessionTodoList)
	}
	if ptc.Action == "manage_plan" {
		emitSessionPlanUpdate(broker, shortTermMem, sessionID, currentLogger)
	}
	if ptc.Action == "manage_memory" || ptc.Action == "core_memory" {
		s.coreMemDirty = true
	}
	found := false
	for _, rt := range s.recentTools {
		if rt == ptc.Action {
			found = true
			break
		}
	}
	if !found {
		s.recentTools = append(s.recentTools, ptc.Action)
		if len(s.recentTools) > 5 {
			s.recentTools = s.recentTools[len(s.recentTools)-5:]
		}
	}
	toolResultRole := openai.ChatMessageRoleUser
	if ptc.NativeCallID != "" {
		toolResultRole = openai.ChatMessageRoleTool
	}
	id, idErr = shortTermMem.InsertMessage(sessionID, toolResultRole, pResultContent, false, true)
	if idErr != nil {
		currentLogger.Error("Failed to persist queued tool-result message", "error", idErr)
	}
	if sessionID == "default" && ShouldAppendHistoryMessage(id, idErr) {
		if ptc.NativeCallID != "" {
			historyManager.AddMessage(openai.ChatCompletionMessage{
				Role:       openai.ChatMessageRoleTool,
				Content:    pResultContent,
				ToolCallID: ptc.NativeCallID,
			}, id, false, true)
		} else {
			historyManager.Add(openai.ChatMessageRoleUser, pResultContent, id, false, true)
		}
	}
	if ptc.NativeCallID != "" {
		// Match batched native tool handling: the originating assistant message with
		// all tool_calls is already in req.Messages from the first tool in the batch.
		s.req.Messages = append(s.req.Messages, openai.ChatCompletionMessage{
			Role:       openai.ChatMessageRoleTool,
			Content:    pResultContent,
			ToolCallID: ptc.NativeCallID,
		})
		if circuitBreakerOpen {
			appendCircuitBreakerSkippedNativeResults(s, shortTermMem, historyManager, sessionID, broker)
			s.req.Tools = nil
			s.req.ToolChoice = "none"
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
		s.req.Tools = nil
		s.req.ToolChoice = "none"
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
	cfg := s.runCfg.Config
	shortTermMem := s.runCfg.ShortTermMem
	historyManager := s.runCfg.HistoryManager
	sessionID := s.runCfg.SessionID
	broker := s.broker
	currentLogger := s.currentLogger

	s.toolCallCount++
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

	preloadedRules, rulesBlocked := ensureTaskRulesBeforeToolExecution(s, tc, lastUserMsg)
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
	if s.recoveryState.handleDuplicateToolCall(tc, &s.req, currentLogger, s.telemetryScope) {
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
			appendCircuitBreakerSkippedNativeResults(s, shortTermMem, historyManager, sessionID, broker)
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
		s.req.Tools = nil
		s.req.ToolChoice = "none"
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
	if precheckResult, prechecked := precheckVirtualDesktopAppOpen(tc, &s.recoveryState); prechecked {
		resultContent = precheckResult
	} else if precheckResult, prechecked := precheckMessagingToolArgs(tc, s.runCfg, sessionID); prechecked {
		resultContent = precheckResult
	} else {
		toolAction = startAgentToolAction(currentLogger, actionLedger, toolAction)
		resultContent = DispatchToolCall(ctx, &tc, dispatchCtx, lastUserMsg)
	}
	policyResult := finalizeToolExecution(ctx, tc, resultContent, tc.GuardianBlocked, cfg, shortTermMem, sessionID, &s.recoveryState, &s.req, currentLogger, s.telemetryScope, optimizer.GetToolPromptVersion(tc.Action), dispatchCtx.ExecutionTimeMs, s.runCfg)
	resultContent = policyResult.Content
	recordVirtualDesktopAppVerification(tc, resultContent, policyResult.Failed, &s.recoveryState)
	invalidateTurnSnapshotAfterTool(s, tc, policyResult.Failed)
	eventContent := policyResult.EventContent
	if eventContent == "" {
		eventContent = resultContent
	}
	if policyResult.Failed {
		recordToolFailureOperationalIssue(s.runCfg, tc, resultContent, currentLogger)
	} else {
		resolveToolFailureOperationalIssue(s.runCfg, tc, currentLogger)
	}
	toolAction = completeAgentToolAction(currentLogger, actionLedger, toolAction, policyResult, dispatchCtx.ExecutionTimeMs)
	trackActivityTool(&s.turnToolNames, &s.turnToolSummaries, tc.Action, resultContent)
	recordPlanToolProgress(shortTermMem, sessionID, tc, resultContent, currentLogger)
	recordLearnedRuleOutcome(shortTermMem, s.flags.InjectedLearnedRules, tc.Action, policyResult.Failed, currentLogger)

	broker.Send("tool_output", resultContent)
	emitMediaSSEEvents(broker, tc.Action, eventContent, cfg.Directories.DataDir)

	broker.Send("tool_end", tc.Action)
	s.lastActivity = time.Now()
	s.lastResponseWasTool = true

	if tc.Todo != "" {
		s.sessionTodoList = string(tc.Todo)
		broker.Send("todo_update", s.sessionTodoList)
	}
	if tc.Action == "manage_plan" {
		emitSessionPlanUpdate(broker, shortTermMem, sessionID, currentLogger)
	}

	if tc.Action == "manage_memory" {
		s.coreMemDirty = true
	}

	if s.lastTool != "" {
		_ = shortTermMem.RecordToolTransition(s.lastTool, tc.Action)
	}
	s.lastTool = tc.Action
	found := false
	for _, rt := range s.recentTools {
		if rt == tc.Action {
			found = true
			break
		}
	}
	if !found {
		s.recentTools = append(s.recentTools, tc.Action)
		if len(s.recentTools) > 5 {
			s.recentTools = s.recentTools[len(s.recentTools)-5:]
		}
	}

	if cfg.Agent.WorkflowFeedback && !s.flags.IsCoAgent && sessionID == "default" {
		s.stepsSinceLastFeedback++
		if s.stepsSinceLastFeedback >= 3 {
			s.stepsSinceLastFeedback = 0
			broker.Send("progress", i18n.T(cfg.Server.UILanguage, "backend.workflow_feedback"))
		}
	}

	toolEmotionTrigger, toolEmotionDetail := detectToolEmotionTrigger(tc, s.recoveryState.ConsecutiveErrorCount, s.toolCallCount-s.recoveryState.ConsecutiveErrorCount)
	if s.personalityEnabled && shortTermMem != nil && toolEmotionTrigger != "" {
		emitAffectFromTrigger(shortTermMem, cfg, currentLogger, toolEmotionTrigger, toolEmotionDetail, "tool")
	}

	// Skip personality side-effects for missions, heartbeats, co-agents, and maintenance.
	if s.personalityEnabled && shortTermMem != nil && !isAutonomousAgentRun(s.runCfg, sessionID) && !s.runCfg.IsMission && !s.flags.IsMission && !s.runCfg.IsCoAgent && !s.runCfg.IsMaintenance && sessionID != "maintenance" {
		triggerInfo := triggerValue
		if strings.Contains(resultContent, "ERROR") || strings.Contains(resultContent, "error") {
			triggerInfo = triggerValue + " [tool error]"
		}

		if cfg.Personality.EngineV2 {
			recentMsgs := s.req.Messages
			launchAsyncPersonalityV2Analysis(
				sessionID,
				cfg,
				currentLogger,
				s.runCfg.LLMClient,
				shortTermMem,
				s.emotionSynthesizer,
				recentMsgs,
				triggerInfo,
				toolEmotionTrigger,
				toolEmotionDetail,
				0,
				"Tool Result",
				resultContent,
				s.meta,
				cfg.Personality.UserProfiling,
				s.recoveryState.ConsecutiveErrorCount,
				s.recoveryState.TotalErrorCount,
				s.toolCallCount-s.recoveryState.ConsecutiveErrorCount,
				s.flags.IsMission,
				s.flags.IsCoAgent,
			)
		} else {
			mood, traitDeltas := memory.DetectMood(lastUserMsg, resultContent, s.meta)
			currentTraits, _ := shortTermMem.GetTraits()
			if s.emotionSynthesizer != nil {
				mood = memory.ApplyEmotionBias(mood, s.emotionSynthesizer.GetLastEmotion(), currentTraits)
			}
			_ = shortTermMem.LogMood(mood, triggerInfo)
			for trait, delta := range traitDeltas {
				_ = shortTermMem.UpdateTrait(trait, dampenTraitDelta(currentTraits[trait], delta))
			}
		}
		s.flags.PersonalityLine = shortTermMem.GetPersonalityLineWithMeta(cfg.Personality.EngineV2, s.meta)

		if emotionDescription := latestEmotionDescription(shortTermMem, s.emotionSynthesizer); emotionDescription != "" {
			s.flags.EmotionDescription = emotionDescription
		}
	}

	if tc.NotifyOnCompletion {
		resultContent = fmt.Sprintf(
			"[TOOL COMPLETION NOTIFICATION]\nAction: %s\nStatus: Completed\nTimestamp: %s\nOutput:\n%s",
			tc.Action,
			time.Now().Format(time.RFC3339),
			resultContent,
		)
	}
	if tc.Action == "execute_python" {
		if strings.Contains(resultContent, "[EXECUTION ERROR]") || strings.Contains(resultContent, "TIMEOUT") {
			s.flags.IsErrorState = true
			broker.Send("error_recovery", "Script error detected, retrying...")
		} else {
			s.flags.IsErrorState = false
		}
	}
	toolResultPersistRole := openai.ChatMessageRoleTool
	if !useNativePath {
		toolResultPersistRole = openai.ChatMessageRoleUser
	}
	id, err = shortTermMem.InsertMessage(sessionID, toolResultPersistRole, resultContent, false, true)
	if err != nil {
		currentLogger.Error("Failed to persist tool-result message to SQLite", "error", err)
	}
	if sessionID == "default" && ShouldAppendHistoryMessage(id, err) {
		if toolResultPersistRole == openai.ChatMessageRoleTool {
			historyManager.AddMessage(openai.ChatCompletionMessage{
				Role:       openai.ChatMessageRoleTool,
				Content:    resultContent,
				ToolCallID: tc.NativeCallID,
			}, id, false, true)
		} else {
			historyManager.Add(toolResultPersistRole, resultContent, id, false, true)
		}
	}

	if useNativePath {
		s.req.Messages = append(s.req.Messages, nativeAssistantMsg)
		s.req.Messages = append(s.req.Messages, openai.ChatCompletionMessage{
			Role:       openai.ChatMessageRoleTool,
			Content:    resultContent,
			ToolCallID: tc.NativeCallID,
		})

		var nativePendingSummaryBatch map[string]string
		var deferredRecoveryMessages []openai.ChatCompletionMessage
		circuitBreakerOpen := false
		nativeDispatchCtx := s.makeDispatchContext(currentLogger)
		for len(s.pendingTCs) > 0 && s.pendingTCs[0].NativeCallID != "" {
			if s.helperManager != nil && len(nativePendingSummaryBatch) == 0 && !s.runCfg.IsMission && !s.runCfg.IsCoAgent && !isAutonomousAgentRun(s.runCfg, s.runCfg.SessionID) {
				nativePendingSummaryBatch = maybeBuildPendingSummaryBatch(ctx, s.pendingTCs, nativeDispatchCtx, s.helperManager, lastUserMsg)
			}

			btc := s.pendingTCs[0]
			s.pendingTCs = s.pendingTCs[1:]
			s.toolCallCount++
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
			} else if preload, blocked := ensureTaskRulesBeforeToolExecution(s, btc, lastUserMsg); blocked {
				bResult = preload
				batchedBlocked = true
			} else if precomputed, ok := nativePendingSummaryBatch[pendingSummaryBatchKey(btc)]; ok {
				bResult = precomputed
				delete(nativePendingSummaryBatch, pendingSummaryBatchKey(btc))
				if len(nativePendingSummaryBatch) == 0 {
					nativePendingSummaryBatch = nil
				}
			} else if s.recoveryState.handleDuplicateToolCall(btc, &s.req, currentLogger, s.telemetryScope) {
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
			policyResult := toolExecutionResult{Content: bResult, Failed: true, Outcome: ExecutionOutcomeFailed}
			if !notExecuted {
				policyResult = finalizeToolExecution(ctx, btc, bResult, btc.GuardianBlocked, cfg, shortTermMem, sessionID, &s.recoveryState, &s.req, currentLogger, s.telemetryScope, optimizer.GetToolPromptVersion(btc.Action), nativeDispatchCtx.ExecutionTimeMs, s.runCfg)
			}
			bResult = policyResult.Content
			recordVirtualDesktopAppVerification(btc, bResult, policyResult.Failed, &s.recoveryState)
			deferredRecoveryMessages = append(deferredRecoveryMessages, detachNewSystemMessages(&s.req, recoveryMessageStart)...)
			invalidateTurnSnapshotAfterTool(s, btc, policyResult.Failed)
			bEventContent := policyResult.EventContent
			if bEventContent == "" {
				bEventContent = bResult
			}
			if notExecuted {
				// A declared native call still needs one protocol result, but it
				// was never dispatched and must not create a tool-failure issue.
			} else if policyResult.Failed {
				recordToolFailureOperationalIssue(s.runCfg, btc, bResult, currentLogger)
			} else {
				resolveToolFailureOperationalIssue(s.runCfg, btc, currentLogger)
			}
			if batchedBlocked {
				batchedAction = blockAgentToolAction(currentLogger, batchedLedger, batchedAction, bResult)
			} else {
				batchedAction = completeAgentToolAction(currentLogger, batchedLedger, batchedAction, policyResult, nativeDispatchCtx.ExecutionTimeMs)
			}
			trackActivityTool(&s.turnToolNames, &s.turnToolSummaries, btc.Action, bResult)
			recordPlanToolProgress(shortTermMem, sessionID, btc, bResult, currentLogger)
			recordLearnedRuleOutcome(shortTermMem, s.flags.InjectedLearnedRules, btc.Action, policyResult.Failed, currentLogger)
			broker.Send("tool_output", bResult)
			emitMediaSSEEvents(broker, btc.Action, bEventContent, cfg.Directories.DataDir)
			broker.Send("tool_end", btc.Action)
			if btc.Action == "manage_plan" {
				emitSessionPlanUpdate(broker, shortTermMem, sessionID, currentLogger)
			}
			s.lastActivity = time.Now()

			if btc.Action == "manage_memory" || btc.Action == "core_memory" {
				s.coreMemDirty = true
			}
			found := false
			for _, rt := range s.recentTools {
				if rt == btc.Action {
					found = true
					break
				}
			}
			if !found {
				s.recentTools = append(s.recentTools, btc.Action)
				if len(s.recentTools) > 5 {
					s.recentTools = s.recentTools[len(s.recentTools)-5:]
				}
			}

			resultID, resultErr := shortTermMem.InsertMessage(sessionID, openai.ChatMessageRoleTool, bResult, false, true)
			if resultErr != nil {
				currentLogger.Error("Failed to persist batched tool-result message", "error", resultErr)
			}
			if sessionID == "default" && ShouldAppendHistoryMessage(resultID, resultErr) {
				historyManager.AddMessage(openai.ChatCompletionMessage{
					Role:       openai.ChatMessageRoleTool,
					Content:    bResult,
					ToolCallID: btc.NativeCallID,
				}, resultID, false, true)
			}

			s.req.Messages = append(s.req.Messages, openai.ChatCompletionMessage{
				Role:       openai.ChatMessageRoleTool,
				Content:    bResult,
				ToolCallID: btc.NativeCallID,
			})
		}
		if circuitBreakerOpen {
			s.pendingTCs = nil
			s.req.Tools = nil
			s.req.ToolChoice = "none"
		}
		s.req.Messages = append(s.req.Messages, deferredRecoveryMessages...)
	} else {
		if !xmlFallbackHandledThisTurn {
			s.req.Messages = append(s.req.Messages, openai.ChatCompletionMessage{Role: openai.ChatMessageRoleAssistant, Content: content})
		}
		voiceModeActive := !s.voiceOutputSuppressed && (s.runCfg.VoiceOutputActive || GetVoiceMode()) && !isAutonomousAgentRun(s.runCfg, s.runCfg.SessionID) && !s.runCfg.IsMission
		followUpContent := toolResultFollowUpContent(tc, resultContent, voiceModeActive)
		s.req.Messages = append(s.req.Messages, openai.ChatCompletionMessage{Role: openai.ChatMessageRoleUser, Content: followUpContent})
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

func appendCircuitBreakerSkippedNativeResults(s *agentLoopState, stm *memory.SQLiteMemory, history *memory.HistoryManager, sessionID string, broker FeedbackBroker) {
	for len(s.pendingTCs) > 0 && strings.TrimSpace(s.pendingTCs[0].NativeCallID) != "" {
		tc := s.pendingTCs[0]
		s.pendingTCs = s.pendingTCs[1:]
		content := notExecutedDueToCircuitBreakerResult()
		resultID, resultErr := stm.InsertMessage(sessionID, openai.ChatMessageRoleTool, content, false, true)
		if resultErr != nil && s.currentLogger != nil {
			s.currentLogger.Error("Failed to persist circuit-breaker skipped tool result", "error", resultErr)
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
