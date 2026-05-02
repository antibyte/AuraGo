package mqtt

import (
	"strings"

	"aurago/internal/config"
)

// FrigateRelayTopics returns the MQTT topics that should be subscribed for Frigate relays.
func FrigateRelayTopics(cfg *config.Config) []string {
	if cfg == nil || !cfg.Frigate.Enabled {
		return nil
	}
	prefix := frigateTopicPrefix(cfg)
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
	prefix := frigateTopicPrefix(cfg)
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

// FrigateRelayEnabled reports whether any Frigate-specific relay is active.
func FrigateRelayEnabled(cfg *config.Config) bool {
	return len(FrigateRelayTopics(cfg)) > 0
}

func frigateTopicPrefix(cfg *config.Config) string {
	prefix := strings.Trim(strings.TrimSpace(cfg.Frigate.MQTTTopicPrefix), "/")
	if prefix == "" {
		return "frigate"
	}
	return prefix
}
