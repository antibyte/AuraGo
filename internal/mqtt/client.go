package mqtt

import (
	"fmt"
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
)

// missionTriggerEntry holds a registered mission trigger filter + callback.
type missionTriggerEntry struct {
	topicFilter     string
	payloadContains string
	minInterval     time.Duration
	lastFired       time.Time
	callback        func(topic, payload string)
}

// RegisterMissionTrigger registers a callback that fires when a message matches
// the given topic filter and optional payload substring.
func RegisterMissionTrigger(topicFilter string, payloadContains string, minIntervalSeconds int, callback func(topic, payload string)) {
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
	missionTriggers = append(missionTriggers, missionTriggerEntry{
		topicFilter:     topicFilter,
		payloadContains: payloadContains,
		minInterval:     minInterval,
		callback:        callback,
	})
	if logger != nil {
		logger.Info("[MQTT] Mission trigger registered", "topic_filter", topicFilter, "payload_contains", payloadContains, "min_interval", minInterval.String())
	}
}

// ── Public API ──────────────────────────────────────────────────────────────

// StartClient connects to the MQTT broker and subscribes to configured topics.
// It registers the MQTT bridge so the agent can use publish/subscribe/get tools.
func StartClient(cfg *config.Config, log *slog.Logger) {
	if !cfg.MQTT.Enabled || cfg.MQTT.Broker == "" {
		return
	}

	logger = log
	buffer.Configure(cfg.MQTT.Buffer.MaxMessages, cfg.MQTT.Buffer.MaxAgeHours, cfg.MQTT.Buffer.MaxPayloadBytes)

	logger.Info("[MQTT] Connecting", "broker", cfg.MQTT.Broker, "client_id", cfg.MQTT.ClientID)

	opts, err := newClientOptions(cfg, logger)
	if err != nil {
		recordError(err)
		logger.Error("[MQTT] Failed to configure client", "error", err)
		return
	}
	startRelayWorker(defaultRelayQueueSize)
	opts.SetOnConnectHandler(func(c pahomqtt.Client) {
		logger.Info("[MQTT] Connected to broker")
		recordConnected()
		publishAvailability(c, cfg, logger)
		subscribeConfiguredTopics(c, cfg)
	}).SetConnectionLostHandler(func(c pahomqtt.Client, err error) {
		recordDisconnected(err)
		logger.Warn("[MQTT] Connection lost", "error", err)
	})

	connectTimeout := time.Duration(cfg.MQTT.ConnectTimeout) * time.Second
	if connectTimeout <= 0 {
		connectTimeout = 15 * time.Second
	}

	c := pahomqtt.NewClient(opts)
	token := c.Connect()
	go func() {
		if token.WaitTimeout(connectTimeout) {
			if token.Error() != nil {
				recordError(token.Error())
				logger.Error("[MQTT] Failed to connect", "error", token.Error())
				return
			}
		} else {
			err := fmt.Errorf("MQTT connect timed out after %s", connectTimeout)
			recordError(err)
			logger.Warn("[MQTT] Connect timed out, will retry in background")
		}
	}()

	mu.Lock()
	client = c
	mu.Unlock()

	// Register bridge functions
	tools.RegisterMQTTBridge(publish, subscribe, unsubscribe, getMessages)
	logger.Info("[MQTT] Bridge registered")
}

// StopClient disconnects the MQTT client gracefully.
func StopClient() {
	mu.Lock()
	c := client
	client = nil
	mu.Unlock()

	if c != nil && c.IsConnected() {
		c.Disconnect(1000)
		recordDisconnected(nil)
		if logger != nil {
			logger.Info("[MQTT] Disconnected")
		}
	}
	stopRelayWorker()
}

// ── Bridge implementations ──────────────────────────────────────────────────

func publish(topic, payload string, qos int, retain bool, log *slog.Logger) error {
	mu.RLock()
	c := client
	mu.RUnlock()

	if c == nil || !c.IsConnected() {
		return fmt.Errorf("MQTT client is not connected")
	}
	if err := validatePublishTopic(topic); err != nil {
		atomic.AddUint64(&stats.publishErrors, 1)
		return err
	}
	if maxPayloadBytes := buffer.currentMaxPayloadBytes(); maxPayloadBytes > 0 && len([]byte(payload)) > maxPayloadBytes {
		atomic.AddUint64(&stats.publishErrors, 1)
		return fmt.Errorf("MQTT payload exceeds %d byte limit", maxPayloadBytes)
	}

	token := c.Publish(topic, byte(qos), retain, payload)
	if !token.WaitTimeout(10 * time.Second) {
		atomic.AddUint64(&stats.publishErrors, 1)
		return fmt.Errorf("MQTT publish timed out")
	}
	if token.Error() != nil {
		atomic.AddUint64(&stats.publishErrors, 1)
		return fmt.Errorf("MQTT publish failed: %w", token.Error())
	}

	atomic.AddUint64(&stats.publishedMessages, 1)
	log.Info("[MQTT] Published", "topic", topic, "retain", retain, "payload_len", len(payload))
	return nil
}

func subscribe(topic string, qos int, log *slog.Logger) error {
	mu.RLock()
	c := client
	mu.RUnlock()

	if c == nil || !c.IsConnected() {
		return fmt.Errorf("MQTT client is not connected")
	}
	if err := validateTopicFilter(topic); err != nil {
		atomic.AddUint64(&stats.subscribeErrors, 1)
		return err
	}

	token := c.Subscribe(topic, byte(qos), messageHandler)
	if !token.WaitTimeout(10 * time.Second) {
		atomic.AddUint64(&stats.subscribeErrors, 1)
		return fmt.Errorf("MQTT subscribe timed out")
	}
	if token.Error() != nil {
		atomic.AddUint64(&stats.subscribeErrors, 1)
		return fmt.Errorf("MQTT subscribe failed: %w", token.Error())
	}

	log.Info("[MQTT] Subscribed", "topic", topic, "qos", qos)
	return nil
}

func unsubscribe(topic string, log *slog.Logger) error {
	mu.RLock()
	c := client
	mu.RUnlock()

	if c == nil || !c.IsConnected() {
		return fmt.Errorf("MQTT client is not connected")
	}
	if err := validateTopicFilter(topic); err != nil {
		atomic.AddUint64(&stats.subscribeErrors, 1)
		return err
	}

	token := c.Unsubscribe(topic)
	if !token.WaitTimeout(10 * time.Second) {
		atomic.AddUint64(&stats.subscribeErrors, 1)
		return fmt.Errorf("MQTT unsubscribe timed out")
	}
	if token.Error() != nil {
		atomic.AddUint64(&stats.subscribeErrors, 1)
		return fmt.Errorf("MQTT unsubscribe failed: %w", token.Error())
	}

	log.Info("[MQTT] Unsubscribed", "topic", topic)
	return nil
}

func getMessages(topic string, limit int, log *slog.Logger) ([]tools.MQTTMessage, error) {
	return buffer.Get(topic, limit), nil
}

// ── Internal helpers ────────────────────────────────────────────────────────

func subscribeConfiguredTopics(c pahomqtt.Client, cfg *config.Config) {
	topicMap := make(map[string]byte, len(cfg.MQTT.Topics))
	for _, topic := range cfg.MQTT.Topics {
		if err := validateTopicFilter(topic); err != nil {
			atomic.AddUint64(&stats.subscribeErrors, 1)
			logger.Warn("[MQTT] Skipping invalid configured topic", "topic", topic, "error", err)
			continue
		}
		topicMap[topic] = mqttQoS(cfg.MQTT.QoS, 0)
	}
	for _, topic := range FrigateRelayTopics(cfg) {
		if err := validateTopicFilter(topic); err != nil {
			atomic.AddUint64(&stats.subscribeErrors, 1)
			logger.Warn("[MQTT] Skipping invalid Frigate relay topic", "topic", topic, "error", err)
			continue
		}
		topicMap[topic] = mqttQoS(cfg.MQTT.QoS, 0)
	}
	if len(topicMap) == 0 {
		return
	}
	token := c.SubscribeMultiple(topicMap, messageHandler)
	if token.WaitTimeout(10*time.Second) && token.Error() == nil {
		logger.Info("[MQTT] Subscribed to configured topics", "count", len(topicMap))
	} else {
		atomic.AddUint64(&stats.subscribeErrors, 1)
		logger.Warn("[MQTT] Failed to subscribe configured topics", "error", token.Error())
	}
}

func messageHandler(_ pahomqtt.Client, msg pahomqtt.Message) {
	m := tools.MQTTMessage{
		Topic:     msg.Topic(),
		Payload:   string(msg.Payload()),
		QoS:       int(msg.Qos()),
		Retained:  msg.Retained(),
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}
	m = buffer.Add(m)
	atomic.AddUint64(&stats.receivedMessages, 1)
	if m.PayloadTruncated {
		atomic.AddUint64(&stats.droppedPayloadMessages, 1)
	}

	if logger != nil {
		logger.Debug("[MQTT] Message received", "topic", m.Topic, "payload_len", len(m.Payload))
	}

	if RelayCallback != nil {
		enqueueRelayMessage(m)
	}

	triggers := matchingMissionTriggers(m.Topic, m.Payload)
	for _, t := range triggers {
		go t.callback(m.Topic, m.Payload)
	}
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
	mu.RLock()
	c := client
	mu.RUnlock()
	return c != nil && c.IsConnected()
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
