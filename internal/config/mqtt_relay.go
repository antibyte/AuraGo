package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

// MQTTTopicMatches implements MQTT filters, including the reserved $ topic rule.
func MQTTTopicMatches(filter, topic string) bool {
	if validateMQTTTopicFilter(filter) != nil {
		return false
	}
	filterParts := strings.Split(filter, "/")
	if strings.HasPrefix(topic, "$") && (filterParts[0] == "#" || filterParts[0] == "+") {
		return false
	}
	topicParts := strings.Split(topic, "/")
	for i, part := range filterParts {
		if part == "#" {
			return true
		}
		if i >= len(topicParts) || part != "+" && part != topicParts[i] {
			return false
		}
	}
	return len(filterParts) == len(topicParts)
}

// MQTTRelayTopicAllowed only selects traffic already received from the broker.
func MQTTRelayTopicAllowed(cfg *Config, topic string) bool {
	if cfg != nil {
		for _, filter := range cfg.MQTT.RelayTopics {
			if MQTTTopicMatches(filter, topic) {
				return true
			}
		}
	}
	return false
}

// MQTTAgentRunsAuthorized is shared by queue admission and live server delivery.
func MQTTAgentRunsAuthorized(cfg *Config) bool {
	return cfg != nil && cfg.MQTT.Enabled && !cfg.EggMode.Enabled &&
		(MQTTBrokerAuthenticated(cfg) || cfg.MQTT.AllowUnauthenticatedRelay) &&
		(MQTTRelayTransportTrusted(cfg) || cfg.MQTT.AllowUntrustedTransport)
}

// MQTTRelayPauseReason exposes a stable diagnosis without credentials or payloads.
func MQTTRelayPauseReason(cfg *Config) string {
	if cfg == nil || !cfg.MQTT.Enabled || cfg.EggMode.Enabled {
		return "disabled"
	}
	if !MQTTBrokerAuthenticated(cfg) && !cfg.MQTT.AllowUnauthenticatedRelay {
		return "credentials"
	}
	if !MQTTRelayTransportTrusted(cfg) && !cfg.MQTT.AllowUntrustedTransport {
		return "transport"
	}
	if len(cfg.MQTT.RelayTopics) == 0 {
		return "topics"
	}
	if len(cfg.MQTT.RelayGrants) == 0 {
		return "grants"
	}
	return ""
}

// ValidateMQTTRelayGrants permits only bounded, concrete relay actions.
func ValidateMQTTRelayGrants(rules []ActionPolicyRule) error {
	if err := ValidateActionPolicy(ActionPolicyConfig{Allow: rules}); err != nil {
		return err
	}
	for i, rule := range rules {
		if rule.RunType != "" && rule.RunType != "background" || rule.ScopeID != "" ||
			strings.TrimSpace(rule.Tool) != rule.Tool || strings.TrimSpace(rule.Operation) != rule.Operation ||
			strings.TrimSpace(rule.Target) != rule.Target || strings.ContainsAny(rule.Target, "*?\x00") {
			return fmt.Errorf("relay grant %d requires a concrete tool, operation and target", i+1)
		}
		valid := false
		switch rule.Tool {
		case "mqtt_get_messages":
			valid = rule.Operation == "read" && rule.Target == "tool:mqtt_get_messages"
		case "mqtt_publish":
			topic, found := strings.CutPrefix(rule.Target, "mqtt:")
			valid = found && rule.Operation == "publish" && validateMQTTPublishTopic(topic) == nil
		case "mqtt_subscribe", "mqtt_unsubscribe":
			filter, found := strings.CutPrefix(rule.Target, "mqtt_filter:")
			op := strings.TrimPrefix(rule.Tool, "mqtt_")
			valid = found && rule.Operation == op && validateMQTTTopicFilter(filter) == nil
		case "home_assistant":
			target, found := strings.CutPrefix(rule.Target, "ha:")
			entity, service, hasService := strings.Cut(target, "/")
			valid = found && validMQTTHAIdentifier(entity) &&
				(rule.Operation == "get_state" && !hasService || rule.Operation == "call_service" && hasService && validMQTTHAIdentifier(service))
		case "send_notification":
			channel, found := strings.CutPrefix(rule.Target, "notification:")
			switch channel {
			case "ntfy", "pushover", "telegram", "whatsapp", "discord", "push", "telnyx", "cyd":
				valid = found && rule.Operation == "execute"
			}
		}
		if !valid {
			return fmt.Errorf("relay grant %d has an unsupported tool, operation or target", i+1)
		}
	}
	return nil
}

func validMQTTHAIdentifier(value string) bool {
	domain, name, found := strings.Cut(value, ".")
	if !found || domain == "" || name == "" {
		return false
	}
	for _, part := range []string{domain, name} {
		for _, ch := range part {
			if !(ch >= 'a' && ch <= 'z' || ch >= '0' && ch <= '9' || ch == '_') {
				return false
			}
		}
	}
	return true
}

func validateMQTTPatchGrants(raw any) error {
	data, err := json.Marshal(raw)
	if err != nil || len(data) == 0 || data[0] != '[' {
		return fmt.Errorf("must be an array of action grants")
	}
	var rules []ActionPolicyRule
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&rules); err != nil {
		return fmt.Errorf("invalid relay grants: %w", err)
	}
	return ValidateMQTTRelayGrants(rules)
}
