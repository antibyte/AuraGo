package server

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"aurago/internal/config"
	"aurago/internal/memory"
	"aurago/internal/mqtt"
	"aurago/internal/security"
	"aurago/internal/tools"
	"aurago/internal/webhooks"
)

// missionWebhookAdapter adapts webhooks.Manager to tools.WebhookManagerInterface
type missionWebhookAdapter struct {
	mgr    *webhooks.Manager
	logger *slog.Logger
}

func (a *missionWebhookAdapter) RegisterMissionTriggerForKey(key, webhookID string, callback func([]byte)) {
	a.mgr.RegisterMissionTriggerForKey(key, webhookID, callback)
}

func (a *missionWebhookAdapter) UnregisterMissionTrigger(key string) {
	a.mgr.UnregisterMissionTrigger(key)
}

// ensure MissionV2 types are compatible with expected interfaces
var _ tools.WebhookManagerInterface = (*missionWebhookAdapter)(nil)
var _ tools.MQTTManagerInterface = (*missionMQTTAdapter)(nil)
var _ tools.KeyedMQTTManagerInterface = (*missionMQTTAdapter)(nil)

// missionMQTTAdapter adapts mqtt package to tools.MQTTManagerInterface
type missionMQTTAdapter struct {
	logger *slog.Logger
	// config returns the live configuration. Every trigger delivery passes
	// the MQTT relay gate against it, so reloads apply without
	// re-registration; without a config source every delivery is refused.
	config func() *config.Config
	// register installs admit and the mission callback in the mqtt package;
	// nil uses registerMQTTMissionTrigger. Tests set it to capture both.
	register func(key, topicFilter, payloadContains string, minIntervalSeconds int, admit func(topic, payload string) bool, callback func(topic, payload string))
	// guardian returns the server Guardian that scans every admitted delivery
	// before a mission starts; nil (or a nil result) scans with a default one.
	guardian func() *security.Guardian
	gate     mqttRelayGate
}

// RegisterMissionTrigger registers a callback for MQTT-triggered missions
func (a *missionMQTTAdapter) RegisterMissionTrigger(topicFilter string, payloadContains string, minIntervalSeconds int, callback func(topic, payload string)) {
	a.RegisterMissionTriggerForKey("", topicFilter, payloadContains, minIntervalSeconds, callback)
}

// admitDelivery decides whether one broker delivery may start a mission. It
// must pass mqttRelayAuthorized (trusted TLS with a broker login,
// allow_unauthenticated_relay, or allow_untrusted_transport) against the live
// config, then the local injection scan.
func (a *missionMQTTAdapter) admitDelivery(topic, payload string) bool {
	var cfg *config.Config
	if a.config != nil {
		cfg = a.config()
	}
	if !a.gate.admit(cfg, a.logger, mqttRelayPathMissionTrigger) {
		return false
	}
	return a.passesInjectionScan(topic, payload)
}

// mqttMissionTriggerText is the topic and the whole payload as the mission run
// reads them in its trigger data. Unlike the relay prompt the mission path does
// not cut the payload, so the scan reads all of it.
func mqttMissionTriggerText(topic, payload string) string {
	return fmt.Sprintf("topic: %s\npayload: %s", topic, payload)
}

// passesInjectionScan scans what the mission run would read, locally and
// whatever the LLM Guardian toggle says: a broker client with publish rights on
// the subscribed topic is not the owner. At ThreatHigh (an incomplete scan reads
// High) the trigger is dropped and logged, so the mission does not start and
// the delivery does not consume the trigger interval.
func (a *missionMQTTAdapter) passesInjectionScan(topic, payload string) bool {
	var guardian *security.Guardian
	if a.guardian != nil {
		guardian = a.guardian()
	}
	scan := guardian.ScanForInjectionLocal(mqttMissionTriggerText(topic, payload))
	if scan.Level < security.ThreatHigh {
		return true
	}
	if a.logger != nil {
		a.logger.Warn("[MQTT] Mission trigger dropped after prompt-injection scan", "topic", topic, "level", scan.Level.String(), "patterns", scan.Patterns)
	}
	return false
}

// gateMissionTrigger wraps a mission callback so that broker traffic starts
// the mission only while admitDelivery allows it.
func (a *missionMQTTAdapter) gateMissionTrigger(callback func(topic, payload string)) func(topic, payload string) {
	return func(topic, payload string) {
		if !a.admitDelivery(topic, payload) {
			return
		}
		callback(topic, payload)
	}
}

// registerMQTTMissionTrigger installs a gated mission callback in the mqtt
// package. An empty key stays on that same gated path.
func registerMQTTMissionTrigger(key, topicFilter, payloadContains string, minIntervalSeconds int, admit func(topic, payload string) bool, callback func(topic, payload string)) {
	mqtt.RegisterGatedMissionTriggerForKey(key, topicFilter, payloadContains, minIntervalSeconds, admit, callback)
}

// RegisterMissionTriggerForKey registers or replaces a keyed MQTT-triggered mission callback.
func (a *missionMQTTAdapter) RegisterMissionTriggerForKey(key string, topicFilter string, payloadContains string, minIntervalSeconds int, callback func(topic, payload string)) {
	register := a.register
	if register == nil {
		register = registerMQTTMissionTrigger
	}
	// The controller resolves the default interval from its current snapshot.
	// Capturing the startup config here would ignore later interval changes.
	// admit reads the live snapshot; the mission callback stays unwrapped.
	register(key, topicFilter, payloadContains, minIntervalSeconds, a.admitDelivery, callback)
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
		if status, ok := payload["status"].(string); ok {
			switch strings.ToLower(strings.TrimSpace(status)) {
			case "error", "failed", "blocked", "fehlgeschlagen", "blockiert":
				return true
			}
		}
		if _, ok := payload["error"]; ok {
			return true
		}
	}

	lowerContent := strings.ToLower(strings.TrimSpace(structural))
	fence := ""
	for _, line := range strings.Split(lowerContent, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "```") || strings.HasPrefix(line, "~~~") {
			if fence == "" {
				fence = line[:3]
			} else if strings.HasPrefix(line, fence) {
				fence = ""
			}
			continue
		}
		if fence != "" {
			continue
		}
		line = strings.TrimLeft(line, "#*_` ")
		if status, ok := strings.CutPrefix(line, "status:"); ok {
			fields := strings.Fields(strings.TrimLeft(status, "*_` "))
			if len(fields) > 0 {
				switch strings.Trim(fields[0], "*_`.,;:") {
				case "failed", "blocked", "fehlgeschlagen", "blockiert":
					return true
				}
			}
		}
	}
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
