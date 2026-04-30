package mqtt

import (
	"fmt"
	"strings"
)

const maxMQTTTopicBytes = 65535

func validatePublishTopic(topic string) error {
	if err := validateTopicBase(topic); err != nil {
		return err
	}
	if strings.ContainsAny(topic, "+#") {
		return fmt.Errorf("publish topic must not contain MQTT wildcards")
	}
	return nil
}

func validateTopicFilter(topic string) error {
	if err := validateTopicBase(topic); err != nil {
		return err
	}
	levels := strings.Split(topic, "/")
	for index, level := range levels {
		if strings.Contains(level, "#") {
			if level != "#" {
				return fmt.Errorf("multi-level wildcard must occupy an entire topic level")
			}
			if index != len(levels)-1 {
				return fmt.Errorf("multi-level wildcard must be the final topic level")
			}
		}
		if strings.Contains(level, "+") && level != "+" {
			return fmt.Errorf("single-level wildcard must occupy an entire topic level")
		}
	}
	return nil
}

func validateTopicBase(topic string) error {
	if topic == "" {
		return fmt.Errorf("topic is required")
	}
	if len([]byte(topic)) > maxMQTTTopicBytes {
		return fmt.Errorf("topic exceeds MQTT length limit")
	}
	if strings.ContainsRune(topic, '\x00') {
		return fmt.Errorf("topic must not contain null bytes")
	}
	return nil
}
