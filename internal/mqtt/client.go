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
	id              uint64
	key             string
	topicFilter     string
	payloadContains string
	minInterval     time.Duration
	lastFired       time.Time
	callback        func(topic, payload string)
}

var nextMissionTriggerID uint64

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
	entry := missionTriggerEntry{
		id:              atomic.AddUint64(&nextMissionTriggerID, 1),
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
				missionTriggerMu.Unlock()
				missionTriggerChanged()
				return
			}
		}
	}
	missionTriggers = append(missionTriggers, entry)
	if logger != nil {
		logger.Info("[MQTT] Mission trigger registered", "key", key, "topic_filter", topicFilter, "payload_contains", payloadContains, "min_interval", minInterval.String())
	}
	missionTriggerMu.Unlock()
	missionTriggerChanged()
}

// UnregisterMissionTrigger removes a keyed mission trigger callback.
func UnregisterMissionTrigger(key string) {
	if key == "" {
		return
	}
	missionTriggerMu.Lock()
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
	missionTriggerMu.Unlock()
	if removed > 0 {
		missionTriggerChanged()
	}
}

// ── Public API ──────────────────────────────────────────────────────────────

// StartClient connects to the MQTT broker and subscribes to configured topics.
// It registers the MQTT bridge so the agent can use publish/subscribe/get tools.
func StartClient(cfg *config.Config, log *slog.Logger) {
	if log != nil {
		logger = log
	}
	controller := defaultControllerInstance()
	controller.mu.RLock()
	stopped := controller.stopped
	controller.mu.RUnlock()
	if stopped {
		controller = NewMQTTController(log)
		controller.legacyBridge = true
		SetDefaultController(controller)
	}
	controller.UpdateConfig(cfg)
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
	if c == nil || cfg == nil {
		return
	}
	if handler == nil {
		handler = messageHandler
	}
	owners := subscriptionOwnersForConfig(cfg, runtimeSubscriptionSnapshot())
	topicMap := mergeSubscriptionOwners(owners)
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

func messageHandler(clientRef pahomqtt.Client, msg pahomqtt.Message) {
	controller := currentDefaultController()
	if controller == nil {
		return
	}
	controller.mu.RLock()
	gen := controller.current
	controller.mu.RUnlock()
	if gen == nil || !controller.generationCurrent(gen) {
		return
	}
	if clientRef != nil && gen.client != nil && clientRef != gen.client {
		return
	}
	// Route legacy Paho callbacks through the active generation's bounded relay
	// and mission queues. A callback without an active generation is dropped.
	controller.messageHandler(gen)(clientRef, msg)
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
		triggerID := trigger.id
		triggerCallback := trigger.callback
		entry := *trigger
		entry.callback = func(matchedTopic, matchedPayload string) {
			if !claimMissionTrigger(triggerID, nowUTC()) {
				return
			}
			if triggerCallback != nil {
				triggerCallback(matchedTopic, matchedPayload)
			}
		}
		triggers = append(triggers, entry)
	}
	return triggers
}

func nowUTC() time.Time { return time.Now().UTC() }

// claimMissionTrigger records the execution start time, rather than the
// enqueue time. This keeps interval throttling correct when a bounded queue
// is backlogged and rejects callbacks from replaced/unregistered entries.
func claimMissionTrigger(id uint64, now time.Time) bool {
	activeInterval := activeMissionTriggerInterval()
	missionTriggerMu.Lock()
	defer missionTriggerMu.Unlock()
	for index := range missionTriggers {
		trigger := &missionTriggers[index]
		if trigger.id != id {
			continue
		}
		interval := trigger.minInterval
		if interval <= 0 {
			interval = activeInterval
		}
		if interval > 0 && !trigger.lastFired.IsZero() && now.Sub(trigger.lastFired) < interval {
			return false
		}
		trigger.lastFired = now
		return true
	}
	return false
}

func activeMissionTriggerInterval() time.Duration {
	controller := currentDefaultController()
	if controller == nil {
		return 0
	}
	controller.mu.RLock()
	active := controller.active
	controller.mu.RUnlock()
	if active == nil || active.cfg.MQTT.TriggerMinIntervalSeconds <= 0 {
		return 0
	}
	return time.Duration(active.cfg.MQTT.TriggerMinIntervalSeconds) * time.Second
}

func currentDefaultController() *MQTTController {
	defaultControllerMu.RLock()
	controller := defaultController
	defaultControllerMu.RUnlock()
	return controller
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
	if strings.HasPrefix(topic, "$") {
		first := strings.SplitN(filter, "/", 2)[0]
		if first == "#" || first == "+" {
			return false
		}
	}
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
