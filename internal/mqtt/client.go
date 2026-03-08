package mqtt

import (
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"aurago/internal/config"
	"aurago/internal/tools"

	pahomqtt "github.com/eclipse/paho.mqtt.golang"
)

// ── Ring buffer for incoming messages ───────────────────────────────────────

const maxBufferSize = 500

type messageBuffer struct {
	mu       sync.RWMutex
	messages []tools.MQTTMessage
}

func (b *messageBuffer) Add(msg tools.MQTTMessage) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.messages = append(b.messages, msg)
	if len(b.messages) > maxBufferSize {
		b.messages = b.messages[len(b.messages)-maxBufferSize:]
	}
}

func (b *messageBuffer) Get(topic string, limit int) []tools.MQTTMessage {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if limit <= 0 {
		limit = 50
	}

	var result []tools.MQTTMessage
	for i := len(b.messages) - 1; i >= 0 && len(result) < limit; i-- {
		if topic == "" || topic == "#" || b.messages[i].Topic == topic {
			result = append(result, b.messages[i])
		}
	}
	// reverse so oldest first
	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}
	return result
}

// ── Package-level state ─────────────────────────────────────────────────────

var (
	mu     sync.RWMutex
	client pahomqtt.Client
	buffer = &messageBuffer{}
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
	callback        func(topic, payload string)
}

// RegisterMissionTrigger registers a callback that fires when a message matches
// the given topic filter and optional payload substring.
func RegisterMissionTrigger(topicFilter string, payloadContains string, callback func(topic, payload string)) {
	missionTriggerMu.Lock()
	defer missionTriggerMu.Unlock()
	missionTriggers = append(missionTriggers, missionTriggerEntry{
		topicFilter:     topicFilter,
		payloadContains: payloadContains,
		callback:        callback,
	})
	if logger != nil {
		logger.Info("[MQTT] Mission trigger registered", "topic_filter", topicFilter, "payload_contains", payloadContains)
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
	logger.Info("[MQTT] Connecting", "broker", cfg.MQTT.Broker, "client_id", cfg.MQTT.ClientID)

	opts := pahomqtt.NewClientOptions().
		AddBroker(cfg.MQTT.Broker).
		SetClientID(cfg.MQTT.ClientID).
		SetAutoReconnect(true).
		SetConnectRetry(true).
		SetConnectRetryInterval(10 * time.Second).
		SetKeepAlive(30 * time.Second).
		SetOnConnectHandler(func(c pahomqtt.Client) {
			logger.Info("[MQTT] Connected to broker")
			// Re-subscribe after reconnect
			subscribeConfiguredTopics(c, cfg)
		}).
		SetConnectionLostHandler(func(c pahomqtt.Client, err error) {
			logger.Warn("[MQTT] Connection lost", "error", err)
		})

	if cfg.MQTT.Username != "" {
		opts.SetUsername(cfg.MQTT.Username)
	}
	if cfg.MQTT.Password != "" {
		opts.SetPassword(cfg.MQTT.Password)
	}

	c := pahomqtt.NewClient(opts)
	token := c.Connect()
	go func() {
		if token.WaitTimeout(15 * time.Second) {
			if token.Error() != nil {
				logger.Error("[MQTT] Failed to connect", "error", token.Error())
				return
			}
		} else {
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
		if logger != nil {
			logger.Info("[MQTT] Disconnected")
		}
	}
}

// ── Bridge implementations ──────────────────────────────────────────────────

func publish(topic, payload string, qos int, retain bool, log *slog.Logger) error {
	mu.RLock()
	c := client
	mu.RUnlock()

	if c == nil || !c.IsConnected() {
		return fmt.Errorf("MQTT client is not connected")
	}

	token := c.Publish(topic, byte(qos), retain, payload)
	if !token.WaitTimeout(10 * time.Second) {
		return fmt.Errorf("MQTT publish timed out")
	}
	if token.Error() != nil {
		return fmt.Errorf("MQTT publish failed: %w", token.Error())
	}

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

	token := c.Subscribe(topic, byte(qos), messageHandler)
	if !token.WaitTimeout(10 * time.Second) {
		return fmt.Errorf("MQTT subscribe timed out")
	}
	if token.Error() != nil {
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

	token := c.Unsubscribe(topic)
	if !token.WaitTimeout(10 * time.Second) {
		return fmt.Errorf("MQTT unsubscribe timed out")
	}
	if token.Error() != nil {
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
	for _, topic := range cfg.MQTT.Topics {
		token := c.Subscribe(topic, byte(cfg.MQTT.QoS), messageHandler)
		if token.WaitTimeout(10*time.Second) && token.Error() == nil {
			logger.Info("[MQTT] Subscribed to configured topic", "topic", topic)
		} else {
			logger.Warn("[MQTT] Failed to subscribe", "topic", topic, "error", token.Error())
		}
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
	buffer.Add(m)

	if logger != nil {
		logger.Debug("[MQTT] Message received", "topic", m.Topic, "payload_len", len(m.Payload))
	}

	if RelayCallback != nil {
		RelayCallback(m.Topic, m.Payload)
	}

	// Check mission triggers
	missionTriggerMu.RLock()
	triggers := make([]missionTriggerEntry, len(missionTriggers))
	copy(triggers, missionTriggers)
	missionTriggerMu.RUnlock()

	for _, t := range triggers {
		if !topicMatches(t.topicFilter, m.Topic) {
			continue
		}
		if t.payloadContains != "" && !strings.Contains(m.Payload, t.payloadContains) {
			continue
		}
		go t.callback(m.Topic, m.Payload)
	}
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
	return len(buffer.Get("", 0))
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
