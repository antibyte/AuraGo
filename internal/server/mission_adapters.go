package server

import (
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
	// Note: This is a simplified adapter. In production, you'd want to
	// store the callback and invoke it when the webhook fires.
	// The actual webhook handling is done in the webhook handler.
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
