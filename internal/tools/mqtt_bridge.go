package tools

import (
	"fmt"
	"log/slog"
	"sync"
)

// ── MQTT Bridge ─────────────────────────────────────────────────────────────
//
// Breaks the import cycle between agent ↔ mqtt.
// The mqtt package registers its functions here at startup.
// The agent package calls them through these function pointers.

// MQTTMessage represents a received MQTT message (cycle-safe DTO).
type MQTTMessage struct {
	Topic     string `json:"topic"`
	Payload   string `json:"payload"`
	QoS       int    `json:"qos"`
	Retained  bool   `json:"retained"`
	Timestamp string `json:"timestamp"`
}

var (
	mqttMu            sync.RWMutex
	mqttPublishFunc   func(topic, payload string, qos int, retain bool, logger *slog.Logger) error
	mqttSubscribeFunc func(topic string, qos int, logger *slog.Logger) error
	mqttUnsubFunc     func(topic string, logger *slog.Logger) error
	mqttMessagesFunc  func(topic string, limit int, logger *slog.Logger) ([]MQTTMessage, error)
)

// RegisterMQTTBridge is called by the mqtt package at startup.
func RegisterMQTTBridge(
	publish func(topic, payload string, qos int, retain bool, logger *slog.Logger) error,
	subscribe func(topic string, qos int, logger *slog.Logger) error,
	unsub func(topic string, logger *slog.Logger) error,
	messages func(topic string, limit int, logger *slog.Logger) ([]MQTTMessage, error),
) {
	mqttMu.Lock()
	defer mqttMu.Unlock()
	mqttPublishFunc = publish
	mqttSubscribeFunc = subscribe
	mqttUnsubFunc = unsub
	mqttMessagesFunc = messages
}

// MQTTPublish publishes a message to an MQTT topic via the registered bridge.
func MQTTPublish(topic, payload string, qos int, retain bool, logger *slog.Logger) error {
	if err := requireMQTTPublishPermission(); err != nil {
		return err
	}
	mqttMu.RLock()
	fn := mqttPublishFunc
	mqttMu.RUnlock()
	if fn == nil {
		return fmt.Errorf("MQTT client is not connected")
	}
	return fn(topic, payload, qos, retain, logger)
}

// MQTTSubscribe subscribes to an MQTT topic via the registered bridge.
func MQTTSubscribe(topic string, qos int, logger *slog.Logger) error {
	if err := requireMQTTPermission(); err != nil {
		return err
	}
	mqttMu.RLock()
	fn := mqttSubscribeFunc
	mqttMu.RUnlock()
	if fn == nil {
		return fmt.Errorf("MQTT client is not connected")
	}
	return fn(topic, qos, logger)
}

// MQTTUnsubscribe unsubscribes from an MQTT topic via the registered bridge.
func MQTTUnsubscribe(topic string, logger *slog.Logger) error {
	if err := requireMQTTPermission(); err != nil {
		return err
	}
	mqttMu.RLock()
	fn := mqttUnsubFunc
	mqttMu.RUnlock()
	if fn == nil {
		return fmt.Errorf("MQTT client is not connected")
	}
	return fn(topic, logger)
}

// MQTTGetMessages retrieves recently received MQTT messages via the registered bridge.
func MQTTGetMessages(topic string, limit int, logger *slog.Logger) ([]MQTTMessage, error) {
	if err := requireMQTTPermission(); err != nil {
		return nil, err
	}
	mqttMu.RLock()
	fn := mqttMessagesFunc
	mqttMu.RUnlock()
	if fn == nil {
		return nil, fmt.Errorf("MQTT client is not connected")
	}
	return fn(topic, limit, logger)
}
