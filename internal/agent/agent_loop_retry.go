package agent

import (
	"fmt"
	"strings"

	"aurago/internal/i18n"
	"aurago/internal/security"

	"github.com/sashabaranov/go-openai"
)

// handleAgentLoopRecoveries runs all recovery and retry handlers after an LLM response.
// It returns the possibly-modified content and tc, a bool indicating whether
// the caller should continue to the next loop iteration, and a bool indicating
// whether an XML fallback was handled this turn.
func handleAgentLoopRecoveries(s *agentLoopState, content string, tc ToolCall, parsedToolResp ParsedToolResponse, useNativePath bool, emotionPolicy emotionBehaviorPolicy) (string, ToolCall, bool, bool) {
	cfg := s.runCfg.Config
	shortTermMem := s.runCfg.ShortTermMem
	historyManager := s.runCfg.HistoryManager
	sessionID := s.runCfg.SessionID
	broker := s.broker
	currentLogger := s.currentLogger

	// Clear explicit tools after they've been consumed (they were injected this iteration)
	if len(s.explicitTools) > 0 {
		s.explicitTools = s.explicitTools[:0]
	}

	// Detect <workflow_plan>["tool1","tool2"]</workflow_plan> in the response
	if s.workflowPlanCount < 10 {
		if parsed, stripped := parseWorkflowPlan(content); len(parsed) > 0 {
			s.workflowPlanCount++
			s.explicitTools = parsed
			currentLogger.Info("[Sync] Workflow plan detected, loading tool guides", "tools", parsed, "attempt", s.workflowPlanCount)
			broker.Send("workflow_plan", strings.Join(parsed, ", "))

			strippedContent := strings.TrimSpace(stripped)
			if strippedContent != "" {
				id, err := shortTermMem.InsertMessage(sessionID, openai.ChatMessageRoleAssistant, strippedContent, false, false)
				if err != nil {
					currentLogger.Error("Failed to persist workflow plan message", "error", err)
				}
				if sessionID == "default" {
					historyManager.Add(openai.ChatMessageRoleAssistant, strippedContent, id, false, false)
				}
				s.req.Messages = append(s.req.Messages, openai.ChatCompletionMessage{Role: openai.ChatMessageRoleAssistant, Content: strippedContent})
			}

			nudge := fmt.Sprintf("Tool manuals loaded for: %s. Proceed with your plan.", strings.Join(parsed, ", "))
			s.req.Messages = append(s.req.Messages, openai.ChatCompletionMessage{Role: openai.ChatMessageRoleUser, Content: nudge})
			return content, tc, true, false
		}
	}

	if tc.RawCodeDetected && s.rawCodeCount < 2 {
		s.rawCodeCount++
		currentLogger.Warn("[Sync] Raw code detected, sending corrective feedback", "attempt", s.rawCodeCount)
		feedbackMsg := applyEmotionRecoveryNudge(FormatRawCodeFeedback(), emotionPolicy)
		msgs := s.recoverySession.PersistRecoveryMessages(PersistRecoveryParams{
			SessionID:        sessionID,
			AssistantContent: content,
			FeedbackMsg:      feedbackMsg,
			BrokerEventType:  "error_recovery",
			I18nKey:          "backend.stream_error_recovery_raw_code",
		}, shortTermMem, historyManager)
		s.req.Messages = append(s.req.Messages, msgs...)
		return content, tc, true, false
	}

	if tc.XMLFallbackDetected && tc.IsTool && s.xmlFallbackCount < 2 {
		s.xmlFallbackCount++
		currentLogger.Warn("[Sync] XML fallback tool call detected, sending corrective feedback",
			"attempt", s.xmlFallbackCount, "action", tc.Action)
		broker.Send("error_recovery", i18n.T(cfg.Server.UILanguage, "backend.stream_error_recovery_xml_format"))

		displayContent := security.StripThinkingTags(content)
		for _, marker := range []string{"minimax:tool_call", "<action>", "<invoke", "<tool_call"} {
			if idx := strings.Index(strings.ToLower(displayContent), marker); idx != -1 {
				displayContent = strings.TrimSpace(displayContent[:idx])
				break
			}
		}
		if displayContent != "" {
			id, err := shortTermMem.InsertMessage(sessionID, openai.ChatMessageRoleAssistant, displayContent, false, true)
			if err != nil {
				currentLogger.Error("Failed to persist assistant message to SQLite", "error", err)
			}
			if sessionID == "default" {
				historyManager.Add(openai.ChatMessageRoleAssistant, displayContent, id, false, true)
			}
		}

		const xmlFallbackContentMaxBytes = 500
		xmlFeedback := fmt.Sprintf(
			"NOTE: You called '%s' using a proprietary XML format (minimax:tool_call). "+
				"The tool has already been executed and the action is COMPLETE — do NOT repeat it. "+
				"Continue with the next step of the task. "+
				"For future calls, always use the native function-calling API instead. "+
				"If a tool is not in your current tool list, use discover_tools first so it can be re-added "+
				"to your active tool list on the next turn.",
			tc.Action)
		if len(content) > xmlFallbackContentMaxBytes {
			xmlFeedback += " IMPORTANT: To edit existing files, use the `file_editor` tool with" +
				" `str_replace` or `insert_after` — it modifies only the targeted section and" +
				" never requires sending the complete file content."
		}
		xmlFeedback = applyEmotionRecoveryNudge(xmlFeedback, emotionPolicy)
		id, err := shortTermMem.InsertMessage(sessionID, openai.ChatMessageRoleUser, xmlFeedback, false, true)
		if err != nil {
			currentLogger.Error("Failed to persist XML feedback message to SQLite", "error", err)
		}
		if sessionID == "default" {
			historyManager.Add(openai.ChatMessageRoleUser, xmlFeedback, id, false, true)
		}

		xmlAssistantContent := content
		if len(xmlAssistantContent) > xmlFallbackContentMaxBytes {
			xmlAssistantContent = fmt.Sprintf(`{"action":"%s"}`, tc.Action)
		}
		s.req.Messages = append(s.req.Messages, openai.ChatCompletionMessage{Role: openai.ChatMessageRoleAssistant, Content: xmlAssistantContent})
		s.req.Messages = append(s.req.Messages, openai.ChatCompletionMessage{Role: openai.ChatMessageRoleUser, Content: xmlFeedback})
		// Don't continue — fall through so the tool is actually executed this turn.
		// XML fallback handled this turn — propagate to tool execution so it doesn't duplicate the assistant message.
		return content, tc, false, true
	}

	if useNativePath && tc.NativeArgsMalformed && s.invalidNativeToolCount < 2 {
		s.invalidNativeToolCount++
		currentLogger.Warn("[Sync] Invalid native tool call detected, requesting corrected function call",
			"attempt", s.invalidNativeToolCount,
			"action", tc.Action,
			"error", tc.NativeArgsError)
		recoveryTool := tc.Action
		if strings.TrimSpace(recoveryTool) == "" {
			recoveryTool = "the requested tool"
		} else {
			recoveryTool = Truncate(strings.ReplaceAll(strings.ReplaceAll(recoveryTool, "\n", " "), "\r", " "), 80)
		}
		feedbackMsg := applyEmotionRecoveryNudge(FormatInvalidNativeToolFeedback(recoveryTool), emotionPolicy)
		msgs := s.recoverySession.PersistRecoveryMessages(PersistRecoveryParams{
			SessionID:            sessionID,
			FeedbackMsg:          feedbackMsg,
			BrokerEventType:      "error_recovery",
			SkipAssistantPersist: true,
			I18nKey:              "backend.stream_error_recovery_invalid_native",
		}, shortTermMem, historyManager)
		s.req.Messages = append(s.req.Messages, msgs...)
		s.lastResponseWasTool = false
		return content, tc, true, false
	}

	if s.useNativeFunctions &&
		tc.IsTool &&
		!useNativePath &&
		(parsedToolResp.ParseSource == ToolCallParseSourceReasoningCleanJSON || parsedToolResp.ParseSource == ToolCallParseSourceContentJSON) &&
		s.invalidNativeToolCount < 2 {
		if shouldAcceptParsedTextToolCallsInNativeMode(s.req.Tools, parsedToolResp.ParseSource, tc, parsedToolResp.PendingToolCalls) {
			currentLogger.Warn("[Sync] Accepting parsed text tool call in native mode because actions are known tool actions",
				"action", tc.Action,
				"source", parsedToolResp.ParseSource,
				"pending_count", len(parsedToolResp.PendingToolCalls))
			return content, tc, false, false
		}
		s.invalidNativeToolCount++
		currentLogger.Warn("[Sync] Non-native text tool call detected while native function calling is enabled, requesting corrected native function call",
			"attempt", s.invalidNativeToolCount,
			"action", tc.Action,
			"source", parsedToolResp.ParseSource)
		feedbackMsg := applyEmotionRecoveryNudge(FormatNonNativeToolCallFeedback(tc.Action), emotionPolicy)
		msgs := s.recoverySession.PersistRecoveryMessages(PersistRecoveryParams{
			SessionID:            sessionID,
			FeedbackMsg:          feedbackMsg,
			BrokerEventType:      "error_recovery",
			SkipAssistantPersist: true,
			I18nKey:              "backend.stream_error_recovery_invalid_native",
		}, shortTermMem, historyManager)
		s.req.Messages = append(s.req.Messages, msgs...)
		s.lastResponseWasTool = false
		return content, ToolCall{}, true, false
	}

	if parsedToolResp.IncompleteToolCall && !tc.IsTool && s.incompleteToolCallCount < 3 {
		s.incompleteToolCallCount++
		currentLogger.Warn("[Sync] Incomplete <tool_call> tag detected, nudging model to emit actual tool call", "attempt", s.incompleteToolCallCount)
		feedbackMsg := applyEmotionRecoveryNudge(FormatIncompleteToolCallFeedback(s.useNativeFunctions, s.incompleteToolCallCount), emotionPolicy)
		msgs := s.recoverySession.PersistRecoveryMessages(PersistRecoveryParams{
			SessionID:        sessionID,
			AssistantContent: content,
			FeedbackMsg:      feedbackMsg,
			BrokerEventType:  "error_recovery",
			I18nKey:          "backend.stream_error_recovery_incomplete_tool_call",
		}, shortTermMem, historyManager)
		if len(msgs) > 0 {
			cleanedContent := bareToolCallTagRe.ReplaceAllString(content, "")
			cleanedContent = strings.TrimSpace(cleanedContent)
			if cleanedContent == "" {
				cleanedContent = parsedToolResp.SanitizedContent
			}
			msgs[0].Content = cleanedContent
		}
		s.req.Messages = append(s.req.Messages, msgs...)
		s.lastResponseWasTool = false
		return content, tc, true, false
	}

	// Language-agnostic recovery: if the PREVIOUS iteration was a tool call (mid-task)
	// and the model now outputs only text without <done/> and without a new tool call,
	// it is stuck.
	announcementContent := parsedToolResp.SanitizedContent
	xmlFallbackPostToolChain := s.xmlFallbackCount > 0 && s.lastResponseWasTool
	midTaskTextOnly := announcementContent != "" &&
		!parsedToolResp.IsFinished &&
		!tc.IsTool &&
		s.lastResponseWasTool &&
		!xmlFallbackPostToolChain
	if midTaskTextOnly {
		const midTaskSubstantiveThreshold = 300
		if len(announcementContent) >= midTaskSubstantiveThreshold {
			currentLogger.Info("[Sync] Mid-task text-only response without <done/> — treating as implicit completion (substantive content)", "content_len", len(announcementContent))
			parsedToolResp.IsFinished = true
			if !strings.Contains(content, "<done/>") {
				content += "\n<done/>"
			}
			content = strings.TrimSpace(strings.ReplaceAll(content, "<done/>", ""))
		} else if containsCompletionEvidence(strings.ToLower(announcementContent)) &&
			!isAnnouncementOnlyResponse(announcementContent, tc, useNativePath, s.lastResponseWasTool, s.lastUserMsg) {
			currentLogger.Info("[Sync] Mid-task text-only response with completion evidence — treating as implicit completion", "content_len", len(announcementContent))
			parsedToolResp.IsFinished = true
			if !strings.Contains(content, "<done/>") {
				content += "\n<done/>"
			}
			content = strings.TrimSpace(strings.ReplaceAll(content, "<done/>", ""))
		} else if s.announcementCount < cfg.Agent.AnnouncementDetector.MaxRetries {
			s.announcementCount++
			currentLogger.Warn("[Sync] Mid-task text-only response without <done/> — requesting tool call or completion signal", "attempt", s.announcementCount, "content_preview", Truncate(announcementContent, 120))
			feedbackMsg := applyEmotionRecoveryNudge(FormatAnnouncementFeedback(s.useNativeFunctions, s.recentTools), emotionPolicy)
			msgs := s.recoverySession.PersistRecoveryMessages(PersistRecoveryParams{
				SessionID:        sessionID,
				AssistantContent: content,
				FeedbackMsg:      feedbackMsg,
				BrokerEventType:  "error_recovery",
				I18nKey:          "backend.stream_error_recovery_announcement_no_action",
			}, shortTermMem, historyManager)
			s.req.Messages = append(s.req.Messages, msgs...)
			return content, tc, true, false
		}
	}

	announcementOnly := announcementContent != "" &&
		!parsedToolResp.IsFinished &&
		!tc.IsTool &&
		isAnnouncementOnlyResponse(announcementContent, tc, useNativePath, s.lastResponseWasTool, s.lastUserMsg)
	if announcementOnly && s.announcementCount < cfg.Agent.AnnouncementDetector.MaxRetries {
		s.announcementCount++
		currentLogger.Warn("[Sync] Announcement-only text response detected, requesting tool call or completion signal",
			"attempt", s.announcementCount,
			"last_user_msg", Truncate(s.lastUserMsg, 120),
			"content_preview", Truncate(announcementContent, 120))
		feedbackMsg := applyEmotionRecoveryNudge(FormatAnnouncementFeedback(s.useNativeFunctions, s.recentTools), emotionPolicy)
		msgs := s.recoverySession.PersistRecoveryMessages(PersistRecoveryParams{
			SessionID:        sessionID,
			AssistantContent: content,
			FeedbackMsg:      feedbackMsg,
			BrokerEventType:  "error_recovery",
			I18nKey:          "backend.stream_error_recovery_announcement_no_action",
		}, shortTermMem, historyManager)
		s.req.Messages = append(s.req.Messages, msgs...)
		return content, tc, true, false
	}

	if !tc.IsTool && !tc.RawCodeDetected && s.missedToolCount < 2 &&
		(strings.Contains(content, "```") || strings.Contains(content, "{")) &&
		(strings.Contains(content, `"action"`) || strings.Contains(content, `'action'`)) {
		s.missedToolCount++
		currentLogger.Warn("[Sync] Missed tool call in fence, sending corrective feedback", "attempt", s.missedToolCount, "content_preview", Truncate(content, 150))
		feedbackMsg := applyEmotionRecoveryNudge(FormatMissedToolInFenceFeedback(), emotionPolicy)
		msgs := s.recoverySession.PersistRecoveryMessages(PersistRecoveryParams{
			SessionID:        sessionID,
			AssistantContent: content,
			FeedbackMsg:      feedbackMsg,
			BrokerEventType:  "error_recovery",
			I18nKey:          "backend.stream_error_recovery_fence_json",
		}, shortTermMem, historyManager)
		s.req.Messages = append(s.req.Messages, msgs...)
		return content, tc, true, false
	}

	if !tc.IsTool && !useNativePath && s.orphanedBracketTagCount < 2 {
		lowerForTagCheck := strings.ToLower(content)
		hasOpenTag := strings.Contains(lowerForTagCheck, "[tool_call]")
		hasCloseTag := strings.Contains(lowerForTagCheck, "[/tool_call]")
		if hasOpenTag && !hasCloseTag {
			s.orphanedBracketTagCount++
			currentLogger.Warn("[Sync] Orphaned [TOOL_CALL] tag detected without closing tag, requesting corrective tool call", "attempt", s.orphanedBracketTagCount, "content_preview", Truncate(content, 150))
			feedbackMsg := applyEmotionRecoveryNudge(FormatOrphanedBracketTagFeedback(s.useNativeFunctions), emotionPolicy)
			msgs := s.recoverySession.PersistRecoveryMessages(PersistRecoveryParams{
				SessionID:        sessionID,
				AssistantContent: content,
				FeedbackMsg:      feedbackMsg,
				BrokerEventType:  "error_recovery",
				I18nKey:          "backend.stream_error_recovery_incomplete_tag",
			}, shortTermMem, historyManager)
			s.req.Messages = append(s.req.Messages, msgs...)
			return content, tc, true, false
		}
	}

	if !tc.IsTool && s.useNativeFunctions && s.orphanedXMLTagCount < 2 {
		lowerContent := strings.ToLower(parsedToolResp.SanitizedContent + content)
		if strings.Contains(lowerContent, "<tool_call") || strings.Contains(lowerContent, "minimax:tool_call") {
			s.orphanedXMLTagCount++
			currentLogger.Warn("[Sync] Bare <tool_call> XML in native mode, requesting native function call", "attempt", s.orphanedXMLTagCount, "content_preview", Truncate(content, 150))
			feedbackMsg := applyEmotionRecoveryNudge(FormatBareXMLInNativeModeFeedback(), emotionPolicy)
			msgs := s.recoverySession.PersistRecoveryMessages(PersistRecoveryParams{
				SessionID:        sessionID,
				AssistantContent: content,
				FeedbackMsg:      feedbackMsg,
				BrokerEventType:  "error_recovery",
				I18nKey:          "backend.stream_error_recovery_xml_in_native_mode",
			}, shortTermMem, historyManager)
			s.req.Messages = append(s.req.Messages, msgs...)
			return content, tc, true, false
		}
	}

	return content, tc, false, false
}
