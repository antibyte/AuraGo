package mqtt

import (
	"strings"

	"aurago/internal/config"
)

// FrigateRelayTopics returns the MQTT topics that should be subscribed for Frigate relays.
// An unusable prefix returns nil so nothing is subscribed.
func FrigateRelayTopics(cfg *config.Config) []string {
	if cfg == nil || !cfg.Frigate.Enabled {
		return nil
	}
	prefix, err := frigateTopicPrefix(cfg)
	if err != nil {
		return nil
	}
	topics := make([]string, 0, 2)
	if cfg.Frigate.EventRelay {
		topics = append(topics, prefix+"/events")
	}
	if cfg.Frigate.ReviewRelay {
		topics = append(topics, prefix+"/reviews")
	}
	return topics
}

// FrigateRelayKind returns the Frigate relay kind for a received topic.
func FrigateRelayKind(cfg *config.Config, topic string) (string, bool) {
	if cfg == nil || !cfg.Frigate.Enabled {
		return "", false
	}
	prefix, err := frigateTopicPrefix(cfg)
	if err != nil {
		return "", false
	}
	switch strings.TrimSpace(topic) {
	case prefix + "/events":
		if cfg.Frigate.EventRelay {
			return "event", true
		}
	case prefix + "/reviews":
		if cfg.Frigate.ReviewRelay {
			return "review", true
		}
	}
	return "", false
}

// FrigateLiteralPrefix reports the trimmed prefix when both relay topics are literal MQTT topics.
func FrigateLiteralPrefix(cfg *config.Config) (string, bool) {
	if cfg == nil {
		return "", false
	}
	prefix, err := frigateTopicPrefix(cfg)
	if err != nil {
		return "", false
	}
	return prefix, true
}

// FrigateRelayEnabled reports whether any Frigate-specific relay is active.
func FrigateRelayEnabled(cfg *config.Config) bool {
	return len(FrigateRelayTopics(cfg)) > 0
}

func frigateTopicPrefix(cfg *config.Config) (string, error) {
	prefix := strings.Trim(strings.TrimSpace(cfg.Frigate.MQTTTopicPrefix), "/")
	if prefix == "" {
		prefix = "frigate"
	}
	if err := config.ValidateLiteralMQTTTopic(prefix + "/events"); err != nil {
		return "", err
	}
	if err := config.ValidateLiteralMQTTTopic(prefix + "/reviews"); err != nil {
		return "", err
	}
	return prefix, nil
}
