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
	if s.helperManager != nil && len(s.pendingSummaryBatch) == 0 {
		s.pendingSummaryBatch = maybeBuildPendingSummaryBatch(ctx, s.pendingTCs, dispatchCtx, s.helperManager, lastUserMsg)
	}

	ptc := s.pendingTCs[0]
	s.pendingTCs = s.pendingTCs[1:]
	s.toolCallCount++
	if ptc.Action == "homepage" || ptc.Action == "homepage_tool" {
		s.homepageUsedInChain = true
	}
	broker.Send("thinking", fmt.Sprintf("[%d] Running %s...", s.toolCallCount, ptc.Action))
	ptcJSON := ptc.RawJSON
	if ptcJSON == "" {
		ptcJSON = fmt.Sprintf(`{"action":"%s"}`, ptc.Action)
	}
	id, idErr := shortTermMem.InsertMessage(sessionID, openai.ChatMessageRoleAssistant, ptcJSON, false, true)
	if idErr != nil {
		currentLogger.Error("Failed to persist queued tool-call message", "error", idErr)
	}
	if sessionID == "default" {
		historyManager.Add(openai.ChatMessageRoleAssistant, ptcJSON, id, false, true)
	}
	broker.Send("tool_call", ptcJSON)
	broker.Send("tool_start", ptc.Action)
	if ptc.Action != "" {
		s.sessionUsedTools[ptc.Action] = true
	}

	pResultContent := ""
	if precomputed, ok := s.pendingSummaryBatch[pendingSummaryBatchKey(ptc)]; ok {
		pResultContent = precomputed
		delete(s.pendingSummaryBatch, pendingSummaryBatchKey(ptc))
		if len(s.pendingSummaryBatch) == 0 {
			s.pendingSummaryBatch = nil
		}
	} else if s.recoveryState.handleDuplicateToolCall(ptc, &s.req, currentLogger, s.telemetryScope) {
		pResultContent = blockedToolOutputFromRequest(&s.req)
	} else {
		pResultContent = DispatchToolCall(ctx, &ptc, dispatchCtx, lastUserMsg)
	}
	policyResult := finalizeToolExecution(ptc, pResultContent, ptc.GuardianBlocked, cfg, shortTermMem, sessionID,
		&s.recoveryState, &s.req, currentLogger, s.telemetryScope, optimizer.GetToolPromptVersion(ptc.Action),
		dispatchCtx.ExecutionTimeMs)
	pResultContent = policyResult.Content
	trackActivityTool(&s.turnToolNames, &s.turnToolSummaries, ptc.Action, pResultContent)
	recordPlanToolProgress(shortTermMem, sessionID, ptc, pResultContent, currentLogger)
	broker.Send("tool_output", pResultContent)
	emitMediaSSEEvents(broker, ptc.Action, pResultContent, cfg.Directories.DataDir)
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
	id, idErr = shortTermMem.InsertMessage(sessionID, openai.ChatMessageRoleUser, pResultContent, false, true)
	if idErr != nil {
		currentLogger.Error("Failed to persist queued tool-result message", "error", idErr)
	}
	if sessionID == "default" {
		historyManager.Add(openai.ChatMessageRoleUser, pResultContent, id, false, true)
	}
	s.req.Messages = append(s.req.Messages, openai.ChatCompletionMessage{Role: openai.ChatMessageRoleAssistant, Content: ptcJSON})
	s.req.Messages = append(s.req.Messages, openai.ChatCompletionMessage{Role: openai.ChatMessageRoleUser, Content: pResultContent})
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
	if tc.Action == "homepage" || tc.Action == "homepage_tool" {
		s.homepageUsedInChain = true
	}
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
	if sessionID == "default" {
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

	if s.recoveryState.handleDuplicateToolCall(tc, &s.req, currentLogger, s.telemetryScope) {
		if useNativePath && tc.NativeCallID != "" {
			syntheticResult := blockedToolOutputFromRequest(&s.req)
			s.req.Messages = append(s.req.Messages, nativeAssistantMsg)
			s.req.Messages = append(s.req.Messages, openai.ChatCompletionMessage{
				Role:       openai.ChatMessageRoleTool,
				Content:    syntheticResult,
				ToolCallID: tc.NativeCallID,
			})
			resultID, _ := shortTermMem.InsertMessage(sessionID, openai.ChatMessageRoleTool, syntheticResult, false, true)
			if sessionID == "default" {
				historyManager.AddMessage(openai.ChatCompletionMessage{
					Role:       openai.ChatMessageRoleTool,
					Content:    syntheticResult,
					ToolCallID: tc.NativeCallID,
				}, resultID, false, true)
			}
		}
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
	resultContent := DispatchToolCall(ctx, &tc, dispatchCtx, lastUserMsg)
	policyResult := finalizeToolExecution(tc, resultContent, tc.GuardianBlocked, cfg, shortTermMem, sessionID, &s.recoveryState, &s.req, currentLogger, s.telemetryScope, optimizer.GetToolPromptVersion(tc.Action), dispatchCtx.ExecutionTimeMs)
	resultContent = policyResult.Content
	trackActivityTool(&s.turnToolNames, &s.turnToolSummaries, tc.Action, resultContent)
	recordPlanToolProgress(shortTermMem, sessionID, tc, resultContent, currentLogger)

	broker.Send("tool_output", resultContent)
	emitMediaSSEEvents(broker, tc.Action, resultContent, cfg.Directories.DataDir)

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
		if s.stepsSinceLastFeedback >= 4 {
			s.stepsSinceLastFeedback = 0
			feedbackPhrases := []string{
				"Ich brauche noch einen Moment, bin aber dran...",
				"Die Analyse läuft noch, einen Augenblick bitte...",
				"Ich suche noch nach weiteren Informationen...",
				"Bin gleich fertig mit der Bearbeitung...",
				"Das dauert einen Moment länger als erwartet, bleib dran...",
				"Ich verarbeite die Daten noch...",
			}
			phrase := feedbackPhrases[time.Now().Unix()%int64(len(feedbackPhrases))]
			broker.Send("progress", phrase)
		}
	}

	if s.personalityEnabled && shortTermMem != nil {
		triggerInfo := triggerValue
		if strings.Contains(resultContent, "ERROR") || strings.Contains(resultContent, "error") {
			triggerInfo = triggerValue + " [tool error]"
		}

		if cfg.Personality.EngineV2 {
			recentMsgs := s.req.Messages
			toolEmotionTrigger, toolEmotionDetail := detectToolEmotionTrigger(tc, s.recoveryState.ConsecutiveErrorCount, s.toolCallCount-s.recoveryState.ConsecutiveErrorCount)
			launchAsyncPersonalityV2Analysis(
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
			if s.emotionSynthesizer != nil {
				traits, _ := shortTermMem.GetTraits()
				mood = memory.ApplyEmotionBias(mood, s.emotionSynthesizer.GetLastEmotion(), traits)
			}
			_ = shortTermMem.LogMood(mood, triggerInfo)
			for trait, delta := range traitDeltas {
				_ = shortTermMem.UpdateTrait(trait, delta)
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
	if sessionID == "default" {
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

	if strings.Contains(resultContent, "Maintenance Mode activated") {
		currentLogger.Info("Handover sentinel detected, Sidecar taking over...")
		if len(resp.Choices) == 0 {
			return resp, nil, false
		}
		id, err := shortTermMem.InsertMessage(sessionID, resp.Choices[0].Message.Role, content, false, false)
		if err != nil {
			currentLogger.Error("Failed to persist handover message to SQLite", "error", err)
		}
		if sessionID == "default" {
			historyManager.Add(resp.Choices[0].Message.Role, content, id, false, false)
		}
		return resp, nil, false
	}

	if useNativePath {
		s.req.Messages = append(s.req.Messages, nativeAssistantMsg)
		s.req.Messages = append(s.req.Messages, openai.ChatCompletionMessage{
			Role:       openai.ChatMessageRoleTool,
			Content:    resultContent,
			ToolCallID: tc.NativeCallID,
		})

		var nativePendingSummaryBatch map[string]string
		nativeDispatchCtx := s.makeDispatchContext(currentLogger)
		for len(s.pendingTCs) > 0 && s.pendingTCs[0].NativeCallID != "" {
			if s.helperManager != nil && len(nativePendingSummaryBatch) == 0 {
				nativePendingSummaryBatch = maybeBuildPendingSummaryBatch(ctx, s.pendingTCs, nativeDispatchCtx, s.helperManager, lastUserMsg)
			}

			btc := s.pendingTCs[0]
			s.pendingTCs = s.pendingTCs[1:]
			s.toolCallCount++
			if btc.Action == "homepage" || btc.Action == "homepage_tool" {
				s.homepageUsedInChain = true
			}
			broker.Send("thinking", fmt.Sprintf("[%d] Running %s (batched)...", s.toolCallCount, btc.Action))
			broker.Send("tool_start", btc.Action)
			if btc.Action != "" {
				s.sessionUsedTools[btc.Action] = true
			}

			bResult := ""
			if precomputed, ok := nativePendingSummaryBatch[pendingSummaryBatchKey(btc)]; ok {
				bResult = precomputed
				delete(nativePendingSummaryBatch, pendingSummaryBatchKey(btc))
				if len(nativePendingSummaryBatch) == 0 {
					nativePendingSummaryBatch = nil
				}
			} else if s.recoveryState.handleDuplicateToolCall(btc, &s.req, currentLogger, s.telemetryScope) {
				bResult = blockedToolOutputFromRequest(&s.req)
			} else {
				bResult = DispatchToolCall(ctx, &btc, nativeDispatchCtx, lastUserMsg)
			}
			policyResult := finalizeToolExecution(btc, bResult, btc.GuardianBlocked, cfg, shortTermMem, sessionID, &s.recoveryState, &s.req, currentLogger, s.telemetryScope, optimizer.GetToolPromptVersion(btc.Action), nativeDispatchCtx.ExecutionTimeMs)
			bResult = policyResult.Content
			trackActivityTool(&s.turnToolNames, &s.turnToolSummaries, btc.Action, bResult)
			recordPlanToolProgress(shortTermMem, sessionID, btc, bResult, currentLogger)
			broker.Send("tool_output", bResult)
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

			bHistContent := fmt.Sprintf(`{"action": "%s"}`, btc.Action)
			bID, bErr := shortTermMem.InsertMessage(sessionID, openai.ChatMessageRoleAssistant, bHistContent, false, true)
			if bErr != nil {
				currentLogger.Error("Failed to persist batched tool-call message", "error", bErr)
			}
			bID, bErr = shortTermMem.InsertMessage(sessionID, openai.ChatMessageRoleTool, bResult, false, true)
			if bErr != nil {
				currentLogger.Error("Failed to persist batched tool-result message", "error", bErr)
			}
			if sessionID == "default" {
				historyManager.AddMessage(openai.ChatCompletionMessage{
					Role:       openai.ChatMessageRoleTool,
					Content:    bResult,
					ToolCallID: btc.NativeCallID,
				}, bID, false, true)
			}

			s.req.Messages = append(s.req.Messages, openai.ChatCompletionMessage{
				Role:       openai.ChatMessageRoleTool,
				Content:    bResult,
				ToolCallID: btc.NativeCallID,
			})
		}
	} else {
		if !xmlFallbackHandledThisTurn {
			s.req.Messages = append(s.req.Messages, openai.ChatCompletionMessage{Role: openai.ChatMessageRoleAssistant, Content: content})
		}
		s.req.Messages = append(s.req.Messages, openai.ChatCompletionMessage{Role: openai.ChatMessageRoleUser, Content: resultContent})
	}

	if strings.Contains(resultContent, "[LIFEBOAT_EXIT_SIGNAL]") {
		currentLogger.Info("[Sync] Early exit signal received, stopping loop.")
		return resp, nil, false
	}

	select {
	case <-time.After(time.Duration(cfg.Agent.StepDelaySeconds) * time.Second):
		return resp, nil, true
	case <-ctx.Done():
		return resp, ctx.Err(), false
	}
}
