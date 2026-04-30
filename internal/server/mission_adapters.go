package server

import (
	"encoding/json"
	"log/slog"
	"strings"

	"aurago/internal/config"
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
var _ tools.EmailWatcherInterface = (*dummyEmailWatcher)(nil)
var _ tools.WebhookManagerInterface = (*missionWebhookAdapter)(nil)
var _ tools.MQTTManagerInterface = (*missionMQTTAdapter)(nil)

// dummyEmailWatcher is a placeholder for email watcher integration
type dummyEmailWatcher struct {
	logger *slog.Logger
}

func (d *dummyEmailWatcher) RegisterMissionTrigger(folder, subjectContains, fromContains string, callback func(subject, from, body string)) {
	d.logger.Info("[EmailWatcherAdapter] Registered mission trigger",
		"folder", folder,
		"subject", subjectContains,
		"from", fromContains)
}

// missionMQTTAdapter adapts mqtt package to tools.MQTTManagerInterface
type missionMQTTAdapter struct {
	logger *slog.Logger
	cfg    *config.Config
}

// RegisterMissionTrigger registers a callback for MQTT-triggered missions
func (a *missionMQTTAdapter) RegisterMissionTrigger(topicFilter string, payloadContains string, minIntervalSeconds int, callback func(topic, payload string)) {
	if minIntervalSeconds <= 0 && a.cfg != nil && a.cfg.MQTT.TriggerMinIntervalSeconds > 0 {
		minIntervalSeconds = a.cfg.MQTT.TriggerMinIntervalSeconds
	}
	mqtt.RegisterMissionTrigger(topicFilter, payloadContains, minIntervalSeconds, callback)
	a.logger.Info("[MissionMQTTAdapter] Registered mission trigger", "topic_filter", topicFilter, "payload_contains", payloadContains, "min_interval_seconds", minIntervalSeconds)
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

// missionResponseLooksIncomplete flags assistant replies that resemble a
// planning/progress update instead of a finished mission result. It is only
// used when no tool execution was recorded for the mission session.
func missionResponseLooksIncomplete(content string, toolResultCount int) bool {
	if toolResultCount > 0 {
		return false
	}

	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return true
	}

	lower := strings.ToLower(trimmed)
	if missionResponseContainsFailureSignal(lower) {
		return true
	}

	if strings.Contains(lower, "```") && strings.Contains(lower, `"action"`) {
		return true
	}

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
			return true
		}
	}

	return false
}

func missionResponseContainsFailureSignal(lowerContent string) bool {
	failureMarkers := []string{
		`"status":"error"`,
		`"status": "error"`,
		"[error]",
		"tool output:",
		"query is required",
		"prompt' is required",
		"'prompt' is required",
		"failed to ",
		"permission denied",
	}
	for _, marker := range failureMarkers {
		if strings.Contains(lowerContent, marker) {
			return true
		}
	}
	return false
}
