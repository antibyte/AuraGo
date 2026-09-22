package mqtt

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"aurago/internal/config"
	"aurago/internal/tools"

	pahomqtt "github.com/eclipse/paho.mqtt.golang"
)

// ── Package-level state ─────────────────────────────────────────────────────

var (
	mu     sync.RWMutex
	client pahomqtt.Client
	buffer = newMessageBuffer()
	logger *slog.Logger

	// RelayCallback is called for every incoming message when relay_to_agent is enabled.
	// Set by the server package before calling StartClient.
	RelayCallback func(topic, payload string)

	// Mission trigger callbacks
	missionTriggerMu sync.RWMutex
	missionTriggers  []missionTriggerEntry

	activeConfigMu     sync.RWMutex
	activeAvailability *mqttAvailabilitySnapshot

	runtimeSubscriptionsMu sync.RWMutex
	runtimeSubscriptions   = make(map[string]byte)
)

// missionTriggerEntry holds a registered mission trigger filter + callback.
type missionTriggerEntry struct {
	key             string
	topicFilter     string
	payloadContains string
	minInterval     time.Duration
	lastFired       time.Time
	callback        func(topic, payload string)
}

// RegisterMissionTrigger registers a callback that fires when a message matches
// the given topic filter and optional payload substring.
func RegisterMissionTrigger(topicFilter string, payloadContains string, minIntervalSeconds int, callback func(topic, payload string)) {
	registerMissionTrigger("", topicFilter, payloadContains, minIntervalSeconds, callback)
}

// RegisterMissionTriggerForKey registers or replaces a mission trigger callback
// associated with a stable key.
func RegisterMissionTriggerForKey(key string, topicFilter string, payloadContains string, minIntervalSeconds int, callback func(topic, payload string)) {
	registerMissionTrigger(key, topicFilter, payloadContains, minIntervalSeconds, callback)
}

func registerMissionTrigger(key string, topicFilter string, payloadContains string, minIntervalSeconds int, callback func(topic, payload string)) {
	if err := validateTopicFilter(topicFilter); err != nil {
		if logger != nil {
			logger.Warn("[MQTT] Mission trigger rejected invalid topic filter", "topic_filter", topicFilter, "error", err)
		}
		return
	}
	var minInterval time.Duration
	if minIntervalSeconds > 0 {
		minInterval = time.Duration(minIntervalSeconds) * time.Second
	}
	missionTriggerMu.Lock()
	defer missionTriggerMu.Unlock()
	entry := missionTriggerEntry{
		key:             key,
		topicFilter:     topicFilter,
		payloadContains: payloadContains,
		minInterval:     minInterval,
		callback:        callback,
	}
	if key != "" {
		for index := range missionTriggers {
			if missionTriggers[index].key == key {
				missionTriggers[index] = entry
				if logger != nil {
					logger.Info("[MQTT] Mission trigger replaced", "key", key, "topic_filter", topicFilter, "payload_contains", payloadContains, "min_interval", minInterval.String())
				}
				return
			}
		}
	}
	missionTriggers = append(missionTriggers, entry)
	if logger != nil {
		logger.Info("[MQTT] Mission trigger registered", "key", key, "topic_filter", topicFilter, "payload_contains", payloadContains, "min_interval", minInterval.String())
	}
}

// UnregisterMissionTrigger removes a keyed mission trigger callback.
func UnregisterMissionTrigger(key string) {
	if key == "" {
		return
	}
	missionTriggerMu.Lock()
	defer missionTriggerMu.Unlock()
	filtered := make([]missionTriggerEntry, 0, len(missionTriggers))
	removed := 0
	for _, trigger := range missionTriggers {
		if trigger.key == key {
			removed++
			continue
		}
		filtered = append(filtered, trigger)
	}
	missionTriggers = filtered
	if removed > 0 && logger != nil {
		logger.Info("[MQTT] Mission trigger unregistered", "key", key, "removed", removed)
	}
}

// ── Public API ──────────────────────────────────────────────────────────────

// StartClient connects to the MQTT broker and subscribes to configured topics.
// It registers the MQTT bridge so the agent can use publish/subscribe/get tools.
func StartClient(cfg *config.Config, log *slog.Logger) {
	if log != nil {
		logger = log
	}
	defaultControllerInstance().UpdateConfig(cfg)
}

// StopClient disconnects the MQTT client gracefully.
func StopClient() {
	if err := defaultControllerInstance().Stop(context.Background()); err != nil && logger != nil {
		logger.Warn("[MQTT] Stop failed", "error", err)
	}
}

// ── Bridge implementations ──────────────────────────────────────────────────

func publish(topic, payload string, qos int, retain bool, log *slog.Logger) error {
	return defaultControllerInstance().publish(topic, payload, qos, retain, log)
}

func subscribe(topic string, qos int, log *slog.Logger) error {
	return defaultControllerInstance().subscribe(topic, qos, log)
}

func unsubscribe(topic string, log *slog.Logger) error {
	return defaultControllerInstance().unsubscribe(topic, log)
}

func getMessages(topic string, limit int, log *slog.Logger) ([]tools.MQTTMessage, error) {
	return buffer.Get(topic, limit), nil
}

// ── Internal helpers ────────────────────────────────────────────────────────

func subscribeConfiguredTopics(c pahomqtt.Client, cfg *config.Config) {
	subscribeConfiguredTopicsWithHandler(c, cfg, messageHandler)
}

func subscribeConfiguredTopicsWithHandler(c pahomqtt.Client, cfg *config.Config, handler pahomqtt.MessageHandler) {
	if handler == nil {
		handler = messageHandler
	}
	topicMap := make(map[string]byte, len(cfg.MQTT.Topics))
	for _, topic := range cfg.MQTT.Topics {
		if err := validateTopicFilter(topic); err != nil {
			atomic.AddUint64(&stats.subscribeErrors, 1)
			if logger != nil {
				logger.Warn("[MQTT] Skipping invalid configured topic", "topic", topic, "error", err)
			}
			continue
		}
		topicMap[topic] = mqttQoS(cfg.MQTT.QoS, 0)
	}
	for _, topic := range FrigateRelayTopics(cfg) {
		if err := validateTopicFilter(topic); err != nil {
			atomic.AddUint64(&stats.subscribeErrors, 1)
			if logger != nil {
				logger.Warn("[MQTT] Skipping invalid Frigate relay topic", "topic", topic, "error", err)
			}
			continue
		}
		topicMap[topic] = mqttQoS(cfg.MQTT.QoS, 0)
	}
	for topic, qos := range runtimeSubscriptionSnapshot() {
		if err := validateTopicFilter(topic); err != nil {
			atomic.AddUint64(&stats.subscribeErrors, 1)
			if logger != nil {
				logger.Warn("[MQTT] Skipping invalid runtime subscription topic", "topic", topic, "error", err)
			}
			continue
		}
		topicMap[topic] = qos
	}
	if len(topicMap) == 0 {
		return
	}
	token := c.SubscribeMultiple(topicMap, handler)
	if err := waitAndValidateSubscribe(token, topicMap, 10*time.Second); err == nil {
		if logger != nil {
			logger.Info("[MQTT] Subscribed to configured topics", "count", len(topicMap))
		}
	} else {
		atomic.AddUint64(&stats.subscribeErrors, 1)
		if logger != nil {
			logger.Warn("[MQTT] Failed to subscribe configured topics", "error", err)
		}
	}
}

func messageHandler(_ pahomqtt.Client, msg pahomqtt.Message) {
	m := makeMQTTMessage(msg)
	m = storeMQTTMessage(m)

	if RelayCallback != nil {
		enqueueRelayMessage(m)
	}

	triggers := matchingMissionTriggers(m.Topic, m.Payload)
	for _, t := range triggers {
		go t.callback(m.Topic, m.Payload)
	}
}

func makeMQTTMessage(msg pahomqtt.Message) tools.MQTTMessage {
	return tools.MQTTMessage{
		Topic:     msg.Topic(),
		Payload:   string(msg.Payload()),
		QoS:       int(msg.Qos()),
		Retained:  msg.Retained(),
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}
}

func storeMQTTMessage(m tools.MQTTMessage) tools.MQTTMessage {
	m = buffer.Add(m)
	atomic.AddUint64(&stats.receivedMessages, 1)
	if m.PayloadTruncated {
		atomic.AddUint64(&stats.droppedPayloadMessages, 1)
	}
	if logger != nil {
		logger.Debug("[MQTT] Message received", "topic", m.Topic, "payload_len", len(m.Payload))
	}
	return m
}

func matchingMissionTriggers(topic, payload string) []missionTriggerEntry {
	now := time.Now().UTC()
	missionTriggerMu.Lock()
	defer missionTriggerMu.Unlock()

	triggers := make([]missionTriggerEntry, 0, len(missionTriggers))
	for index := range missionTriggers {
		trigger := &missionTriggers[index]
		if !topicMatches(trigger.topicFilter, topic) {
			continue
		}
		if trigger.payloadContains != "" && !strings.Contains(payload, trigger.payloadContains) {
			continue
		}
		if trigger.minInterval > 0 && !trigger.lastFired.IsZero() && now.Sub(trigger.lastFired) < trigger.minInterval {
			if logger != nil {
				logger.Debug("[MQTT] Mission trigger rate-limited", "topic_filter", trigger.topicFilter, "topic", topic)
			}
			continue
		}
		trigger.lastFired = now
		triggers = append(triggers, *trigger)
	}
	return triggers
}

// IsConnected returns whether the MQTT client is currently connected to the broker.
func IsConnected() bool {
	return defaultControllerInstance().Status().Connected
}

// BufferLen returns the number of messages currently held in the ring buffer.
func BufferLen() int {
	return buffer.Len()
}

// GetMessages returns buffered MQTT messages, optionally filtered by topic.
func GetMessages(topic string, limit int) []tools.MQTTMessage {
	return buffer.Get(topic, limit)
}

// topicMatches checks if an MQTT topic matches a filter pattern
// supporting + (single level) and # (multi level) wildcards.
func topicMatches(filter, topic string) bool {
	if filter == "#" {
		return true
	}
	filterParts := strings.Split(filter, "/")
	topicParts := strings.Split(topic, "/")

	for i, fp := range filterParts {
		if fp == "#" {
			return true // # matches everything remaining
		}
		if i >= len(topicParts) {
			return false
		}
		if fp != "+" && fp != topicParts[i] {
			return false
		}
	}
	return len(filterParts) == len(topicParts)
}

func setActiveConfig(cfg *config.Config) {
	activeConfigMu.Lock()
	defer activeConfigMu.Unlock()
	activeAvailability = mqttAvailabilitySnapshotFromConfig(cfg)
}

func currentAvailabilitySnapshot() *mqttAvailabilitySnapshot {
	activeConfigMu.RLock()
	defer activeConfigMu.RUnlock()
	if activeAvailability == nil {
		return nil
	}
	snapshot := *activeAvailability
	return &snapshot
}

func rememberRuntimeSubscription(topic string, qos byte) {
	runtimeSubscriptionsMu.Lock()
	defer runtimeSubscriptionsMu.Unlock()
	runtimeSubscriptions[topic] = qos
}

func forgetRuntimeSubscription(topic string) {
	runtimeSubscriptionsMu.Lock()
	defer runtimeSubscriptionsMu.Unlock()
	delete(runtimeSubscriptions, topic)
}

func runtimeSubscriptionSnapshot() map[string]byte {
	runtimeSubscriptionsMu.RLock()
	defer runtimeSubscriptionsMu.RUnlock()
	snapshot := make(map[string]byte, len(runtimeSubscriptions))
	for topic, qos := range runtimeSubscriptions {
		snapshot[topic] = qos
	}
	return snapshot
}
