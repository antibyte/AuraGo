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
	Topic            string `json:"topic"`
	Payload          string `json:"payload"`
	QoS              int    `json:"qos"`
	Retained         bool   `json:"retained"`
	Timestamp        string `json:"timestamp"`
	PayloadBytes     int    `json:"payload_bytes,omitempty"`
	PayloadTruncated bool   `json:"payload_truncated,omitempty"`
}

// MQTTUnsubscribeOwner identifies a non-manual owner that keeps a topic
// subscribed after an agent removes its manual subscription. It mirrors the
// runtime detail without importing the mqtt package across the bridge.
type MQTTUnsubscribeOwner struct {
	Kind string `json:"kind"`
	Key  string `json:"key,omitempty"`
	QoS  int    `json:"qos"`
}

// MQTTUnsubscribeResult is optional detail returned by controllers that track
// shared subscription ownership. Legacy bridges still receive the compatible
// error-only MQTTUnsubscribe result.
type MQTTUnsubscribeResult struct {
	Topic           string                 `json:"topic"`
	Removed         bool                   `json:"removed"`
	RemainingOwners []MQTTUnsubscribeOwner `json:"remaining_owners,omitempty"`
}

var (
	mqttMu              sync.RWMutex
	mqttPublishFunc     func(topic, payload string, qos int, retain bool, logger *slog.Logger) error
	mqttSubscribeFunc   func(topic string, qos int, logger *slog.Logger) error
	mqttUnsubFunc       func(topic string, logger *slog.Logger) error
	mqttUnsubDetailFunc func(topic string, logger *slog.Logger) (MQTTUnsubscribeResult, error)
	mqttMessagesFunc    func(topic string, limit int, logger *slog.Logger) ([]MQTTMessage, error)
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
	// A bridge replacement belongs to a new controller generation. Do not let
	// an optional detail callback from the previous controller outlive it; the
	// replacing runtime registers its detail callback after this exchange.
	mqttUnsubDetailFunc = nil
	mqttMessagesFunc = messages
}

// RegisterMQTTUnsubscribeDetail installs an optional controller-owned
// unsubscribe result. It is separate from RegisterMQTTBridge so old fixtures
// and embedders can keep the four-function bridge unchanged.
func RegisterMQTTUnsubscribeDetail(fn func(topic string, logger *slog.Logger) (MQTTUnsubscribeResult, error)) {
	mqttMu.Lock()
	mqttUnsubDetailFunc = fn
	mqttMu.Unlock()
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
	if err := requireMQTTMutationPermission(); err != nil {
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
	_, err := MQTTUnsubscribeWithDetails(topic, logger)
	return err
}

// MQTTUnsubscribeWithDetails removes the manual owner and optionally reports
// the remaining runtime owners. The permission gate is evaluated once, before
// selecting either the detail or legacy bridge callback.
func MQTTUnsubscribeWithDetails(topic string, logger *slog.Logger) (MQTTUnsubscribeResult, error) {
	result := MQTTUnsubscribeResult{Topic: topic}
	if err := requireMQTTMutationPermission(); err != nil {
		return result, err
	}
	mqttMu.RLock()
	detailFn := mqttUnsubDetailFunc
	legacyFn := mqttUnsubFunc
	mqttMu.RUnlock()
	if detailFn != nil {
		return detailFn(topic, logger)
	}
	if legacyFn == nil {
		return result, fmt.Errorf("MQTT client is not connected")
	}
	err := legacyFn(topic, logger)
	result.Removed = err == nil
	return result, err
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
