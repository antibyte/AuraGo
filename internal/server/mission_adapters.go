package server

import (
	"encoding/json"
	"log/slog"

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
}

// RegisterMissionTrigger registers a callback for MQTT-triggered missions
func (a *missionMQTTAdapter) RegisterMissionTrigger(topicFilter string, payloadContains string, callback func(topic, payload string)) {
	mqtt.RegisterMissionTrigger(topicFilter, payloadContains, callback)
	a.logger.Info("[MissionMQTTAdapter] Registered mission trigger", "topic_filter", topicFilter, "payload_contains", payloadContains)
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
