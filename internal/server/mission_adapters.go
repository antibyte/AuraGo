package server

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"aurago/internal/config"
	"aurago/internal/memory"
	"aurago/internal/mqtt"
	"aurago/internal/tools"
	"aurago/internal/webhooks"
)

// missionWebhookAdapter adapts webhooks.Manager to tools.WebhookManagerInterface
type missionWebhookAdapter struct {
	mgr    *webhooks.Manager
	logger *slog.Logger
}

// RegisterMissionTrigger registers a callback for webhook-triggered missions
func (a *missionWebhookAdapter) RegisterMissionTrigger(webhookID string, callback func(payload []byte)) {
	a.mgr.RegisterMissionTrigger(webhookID, callback)
	a.logger.Info("[MissionWebhookAdapter] Registered mission trigger", "webhook_id", webhookID)
}

// ensure MissionV2 types are compatible with expected interfaces
var _ tools.WebhookManagerInterface = (*missionWebhookAdapter)(nil)
var _ tools.MQTTManagerInterface = (*missionMQTTAdapter)(nil)
var _ tools.KeyedMQTTManagerInterface = (*missionMQTTAdapter)(nil)

// missionMQTTAdapter adapts mqtt package to tools.MQTTManagerInterface
type missionMQTTAdapter struct {
	logger *slog.Logger
	cfg    *config.Config
}

// RegisterMissionTrigger registers a callback for MQTT-triggered missions
func (a *missionMQTTAdapter) RegisterMissionTrigger(topicFilter string, payloadContains string, minIntervalSeconds int, callback func(topic, payload string)) {
	a.RegisterMissionTriggerForKey("", topicFilter, payloadContains, minIntervalSeconds, callback)
}

// RegisterMissionTriggerForKey registers or replaces a keyed MQTT-triggered mission callback.
func (a *missionMQTTAdapter) RegisterMissionTriggerForKey(key string, topicFilter string, payloadContains string, minIntervalSeconds int, callback func(topic, payload string)) {
	if minIntervalSeconds <= 0 && a.cfg != nil && a.cfg.MQTT.TriggerMinIntervalSeconds > 0 {
		minIntervalSeconds = a.cfg.MQTT.TriggerMinIntervalSeconds
	}
	if key != "" {
		mqtt.RegisterMissionTriggerForKey(key, topicFilter, payloadContains, minIntervalSeconds, callback)
	} else {
		mqtt.RegisterMissionTrigger(topicFilter, payloadContains, minIntervalSeconds, callback)
	}
	a.logger.Info("[MissionMQTTAdapter] Registered mission trigger", "key", key, "topic_filter", topicFilter, "payload_contains", payloadContains, "min_interval_seconds", minIntervalSeconds)
}

// UnregisterMissionTrigger removes a keyed MQTT-triggered mission callback.
func (a *missionMQTTAdapter) UnregisterMissionTrigger(key string) {
	mqtt.UnregisterMissionTrigger(key)
	a.logger.Info("[MissionMQTTAdapter] Unregistered mission trigger", "key", key)
}

// extractAssistantContent parses an OpenAI-compatible chat completion JSON and
// returns the assistant message text from choices[0].message.content.
// Falls back to the raw body if parsing fails.
func extractAssistantContent(body []byte) string {
	var resp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(body, &resp); err == nil && len(resp.Choices) > 0 {
		return resp.Choices[0].Message.Content
	}
	return string(body)
}

const (
	missionSuspiciousReasonEmpty          = "empty_response"
	missionSuspiciousReasonRawToolCall    = "raw_tool_call"
	missionSuspiciousReasonToolError      = "tool_error"
	missionSuspiciousReasonNoToolProgress = "no_tool_progress"
)

type missionToolResultCount struct {
	Value int
	Known bool
}

type missionCompletionAssessment struct {
	Suspicious bool
	Reason     string
}

// resetMissionSessionToolResultBaseline clears stale mission messages before
// establishing the baseline for the new run. If the clear fails, the remaining
// count is used so old tool results are not attributed to the new run.
func resetMissionSessionToolResultBaseline(stm *memory.SQLiteMemory, sessionID string) (missionToolResultCount, error) {
	if stm == nil {
		return missionToolResultCount{}, nil
	}
	clearErr := stm.ClearSession(sessionID)
	if clearErr == nil {
		return missionToolResultCount{Known: true}, nil
	}
	count, countErr := stm.CountInternalToolResultMessages(sessionID)
	if countErr != nil {
		return missionToolResultCount{}, fmt.Errorf("clear stale mission session: %v; count remaining tool results: %w", clearErr, countErr)
	}
	return missionToolResultCount{Value: count, Known: true}, clearErr
}

func readMissionToolResultCount(stm *memory.SQLiteMemory, sessionID string) missionToolResultCount {
	if stm == nil {
		return missionToolResultCount{}
	}
	count, err := stm.CountInternalToolResultMessages(sessionID)
	if err != nil {
		return missionToolResultCount{}
	}
	return missionToolResultCount{Value: count, Known: true}
}

func missionToolResultDelta(before, after missionToolResultCount) missionToolResultCount {
	if !before.Known || !after.Known {
		return missionToolResultCount{}
	}
	delta := after.Value - before.Value
	if delta < 0 {
		delta = 0
	}
	return missionToolResultCount{Value: delta, Known: true}
}

// assessMissionCompletion separates structural response failures from the
// no-tool progress heuristic. Structural failures are invalid regardless of
// tool activity; progress text is invalid only when a zero count is known.
func assessMissionCompletion(content string, toolResults missionToolResultCount) missionCompletionAssessment {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return missionCompletionAssessment{Suspicious: true, Reason: missionSuspiciousReasonEmpty}
	}

	if missionResponseIsRawToolCall(trimmed) {
		return missionCompletionAssessment{Suspicious: true, Reason: missionSuspiciousReasonRawToolCall}
	}
	if missionResponseContainsFailureSignal(trimmed) {
		return missionCompletionAssessment{Suspicious: true, Reason: missionSuspiciousReasonToolError}
	}
	if !toolResults.Known || toolResults.Value != 0 {
		return missionCompletionAssessment{}
	}

	lower := strings.ToLower(trimmed)
	progressMarkers := []string{
		"the user is asking me to",
		"the user wants me to",
		"the user asked me to",
		"according to the plan",
		"according to the mission plan",
		"follow the mission plan",
		"first, ",
		"first ",
		"next, ",
		"next ",
		"let me ",
		"now i need",
		"i need to ",
		"i should ",
		"i will ",
		"i'll ",
		"i am going to",
		"i can now",
		"my next step",
		"the next step",
		"before i",
		"i'll start",
		"let me start",
		"let me check",
		"let me verify",
		"let me search",
		"let me execute",
		"search is running",
		"deploying",
		"lass mich ",
		"ich werde ",
		"ich muss ",
		"ich sollte ",
		"laut dem plan",
		"gemäß dem plan",
		"als nächstes",
		"mein nächster schritt",
		"der nächste schritt",
		"bevor ich",
		"jetzt werde ",
		"jetzt prüfe ",
		"jetzt suche ",
		"jetzt erstelle ",
		"jetzt deploye ",
		"suche läuft",
		"deploye",
	}
	for _, marker := range progressMarkers {
		if strings.Contains(lower, marker) {
			return missionCompletionAssessment{Suspicious: true, Reason: missionSuspiciousReasonNoToolProgress}
		}
	}

	return missionCompletionAssessment{}
}

func missionResponseIsRawToolCall(content string) bool {
	structural := unwrapMissionResponseCodeFence(content)
	lower := strings.ToLower(strings.TrimSpace(structural))
	if strings.HasPrefix(lower, "<tool_call") && strings.HasSuffix(lower, "</tool_call>") {
		return true
	}
	if strings.HasPrefix(lower, "<function=") && strings.Contains(lower, ">") && strings.HasSuffix(lower, "</function>") {
		return true
	}
	var payload map[string]json.RawMessage
	if err := json.Unmarshal([]byte(structural), &payload); err != nil {
		return false
	}
	action, ok := payload["action"]
	return ok && len(strings.TrimSpace(string(action))) > 0
}

func unwrapMissionResponseCodeFence(content string) string {
	trimmed := strings.TrimSpace(content)
	if !strings.HasPrefix(trimmed, "```") || !strings.HasSuffix(trimmed, "```") {
		return trimmed
	}
	if firstLineEnd := strings.IndexByte(trimmed, '\n'); firstLineEnd >= 0 {
		return strings.TrimSpace(strings.TrimSuffix(trimmed[firstLineEnd+1:], "```"))
	}
	return trimmed
}

func missionResponseContainsFailureSignal(content string) bool {
	structural := unwrapMissionResponseCodeFence(content)
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(structural), &payload); err == nil {
		if status, ok := payload["status"].(string); ok && strings.EqualFold(strings.TrimSpace(status), "error") {
			return true
		}
		if _, ok := payload["error"]; ok {
			return true
		}
	}

	lowerContent := strings.ToLower(strings.TrimSpace(structural))
	failureMarkers := []string{
		"[error]",
		"tool output:",
		"query is required",
		"prompt' is required",
		"'prompt' is required",
	}
	for _, marker := range failureMarkers {
		if strings.Contains(lowerContent, marker) {
			return true
		}
	}
	return strings.HasPrefix(lowerContent, "failed to ") || strings.HasPrefix(lowerContent, "permission denied")
}

func missionSuspiciousCompletionDetail(reason, output string) string {
	detail := "Mission response looked incomplete: the final assistant reply was not a verified completed result."
	switch reason {
	case missionSuspiciousReasonEmpty:
		detail = "Mission response looked incomplete: the final assistant reply was empty."
	case missionSuspiciousReasonRawToolCall:
		detail = "Mission response looked incomplete: the final assistant reply contained only a raw tool invocation instead of a completed result."
	case missionSuspiciousReasonToolError:
		detail = "Mission response looked incomplete: the final assistant reply contained an explicit tool error instead of a completed result."
	case missionSuspiciousReasonNoToolProgress:
		detail = "Mission response looked incomplete: no executed tools were recorded and the final assistant reply resembled a progress update instead of a finished result."
	}
	if strings.TrimSpace(output) == "" {
		return detail
	}
	return detail + "\n\n" + output
}
