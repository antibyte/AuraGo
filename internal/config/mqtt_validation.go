package config

import (
	"fmt"
	"net/url"
	"os"
	"reflect"
	"strconv"
	"strings"
)

const (
	MQTTPasswordVaultKey = "mqtt_password"

	MQTTPasswordSourceVault = "vault"
	MQTTPasswordSourceEnv   = "env"
	MQTTPasswordSourceEmpty = "empty"
)

// MQTTEffectiveTLS returns whether the configured broker transport uses TLS.
// Secure broker schemes always use TLS, even when mqtt.tls.enabled is false.
// Plain transports cannot be used with mqtt.tls.enabled=true because Paho
// selects the transport from the broker URL scheme.
func MQTTEffectiveTLS(cfg *Config) (bool, error) {
	if cfg == nil {
		return false, fmt.Errorf("mqtt config is required")
	}
	broker := cfg.MQTT.Broker
	if broker == "" {
		if cfg.MQTT.TLS.Enabled {
			return false, fmt.Errorf("mqtt.tls.enabled requires a secure broker URL")
		}
		return false, nil
	}
	u, err := url.Parse(broker)
	if err != nil {
		return false, fmt.Errorf("invalid MQTT broker URL: %w", err)
	}
	scheme := strings.ToLower(u.Scheme)
	if mqttSecureScheme(scheme) {
		return true, nil
	}
	if mqttPlainScheme(scheme) {
		if cfg.MQTT.TLS.Enabled {
			return false, fmt.Errorf("mqtt.tls.enabled requires a secure broker URL; %s is plaintext", scheme)
		}
		return false, nil
	}
	return false, fmt.Errorf("unsupported MQTT broker URL scheme %q", u.Scheme)
}

func mqttSecureScheme(scheme string) bool {
	switch scheme {
	case "ssl", "tls", "mqtts", "mqtt+ssl", "tcps", "wss":
		return true
	default:
		return false
	}
}

func mqttPlainScheme(scheme string) bool {
	switch scheme {
	case "tcp", "mqtt", "ws", "unix":
		return true
	default:
		return false
	}
}

// ValidateMQTTConfig checks MQTT syntax, types and bounded values without
// touching the filesystem. Runtime TLS material is checked by mqtt.ValidateConfig.
func ValidateMQTTConfig(cfg *Config) error {
	if cfg == nil {
		return fmt.Errorf("mqtt config is required")
	}
	m := &cfg.MQTT
	if m.Broker != "" {
		if err := validateMQTTBrokerURL(m.Broker); err != nil {
			return err
		}
	}
	if m.Enabled && m.Broker == "" {
		return fmt.Errorf("mqtt.broker is required when MQTT is enabled")
	}
	if _, err := MQTTEffectiveTLS(cfg); err != nil {
		return err
	}
	if m.QoS < 0 || m.QoS > 2 {
		return fmt.Errorf("mqtt.qos must be between 0 and 2")
	}
	if m.ConnectTimeout < 0 || m.ConnectTimeout > 120 {
		return fmt.Errorf("mqtt.connect_timeout must be between 0 and 120 seconds")
	}
	if m.TriggerMinIntervalSeconds < 0 || m.TriggerMinIntervalSeconds > 86400 {
		return fmt.Errorf("mqtt.trigger_min_interval_seconds must be between 0 and 86400 seconds")
	}
	if m.Buffer.MaxMessages < 0 || m.Buffer.MaxMessages > 10000 {
		return fmt.Errorf("mqtt.buffer.max_messages must be between 0 and 10000")
	}
	if m.Buffer.MaxAgeHours < 0 || m.Buffer.MaxAgeHours > 8760 {
		return fmt.Errorf("mqtt.buffer.max_age_hours must be between 0 and 8760")
	}
	if m.Buffer.MaxPayloadBytes != 0 && (m.Buffer.MaxPayloadBytes < 1024 || m.Buffer.MaxPayloadBytes > 10*1024*1024) {
		return fmt.Errorf("mqtt.buffer.max_payload_bytes must be 0 or between 1024 and 10485760")
	}
	if m.Availability.QoS < 0 || m.Availability.QoS > 2 {
		return fmt.Errorf("mqtt.availability.qos must be between 0 and 2")
	}
	if m.Availability.Topic != "" {
		if err := validateMQTTPublishTopic(m.Availability.Topic); err != nil {
			return fmt.Errorf("mqtt.availability.topic: %w", err)
		}
	}
	for i, topic := range m.Topics {
		if err := validateMQTTTopicFilter(topic); err != nil {
			return fmt.Errorf("mqtt.topics[%d]: %w", i, err)
		}
	}
	return nil
}

// ValidateMQTTPatch validates the mqtt subtree of a full JSON config patch
// before it is merged into YAML. A patch without an mqtt key is unrelated and
// therefore valid; callers must pass the complete patch map so that unrelated
// config saves remain unaffected.
func ValidateMQTTPatch(patch map[string]interface{}) error {
	if patch == nil {
		return nil
	}
	value, hasMQTT := patch["mqtt"]
	if !hasMQTT {
		return nil
	}
	if value == nil {
		return fmt.Errorf("mqtt must be an object")
	}
	m, ok := value.(map[string]interface{})
	if !ok {
		return fmt.Errorf("mqtt must be an object")
	}
	for key, raw := range m {
		if err := validateMQTTPatchField(key, raw); err != nil {
			return fmt.Errorf("mqtt.%s: %w", key, err)
		}
	}
	return nil
}

func validateMQTTPatchField(key string, raw interface{}) error {
	if raw == nil {
		return fmt.Errorf("must not be null")
	}
	switch key {
	case "enabled", "readonly", "relay_to_agent":
		if _, ok := raw.(bool); !ok {
			return fmt.Errorf("must be a boolean")
		}
	case "broker", "client_id", "username", "password":
		if _, ok := raw.(string); !ok {
			return fmt.Errorf("must be a string")
		}
	case "topics":
		if err := validateMQTTPatchTopics(raw); err != nil {
			return err
		}
	case "qos":
		return validateMQTTPatchInt(raw, 0, 2)
	case "connect_timeout":
		return validateMQTTPatchInt(raw, 0, 120)
	case "trigger_min_interval_seconds":
		return validateMQTTPatchInt(raw, 0, 86400)
	case "clean_session":
		if _, ok := raw.(bool); !ok {
			return fmt.Errorf("must be a boolean")
		}
	case "tls":
		return validateMQTTPatchObject(raw, map[string]string{
			"enabled":              "bool",
			"ca_file":              "string",
			"cert_file":            "string",
			"key_file":             "string",
			"insecure_skip_verify": "bool",
		})
	case "buffer":
		return validateMQTTPatchObject(raw, map[string]string{
			"max_messages":      "max_messages",
			"max_age_hours":     "max_age_hours",
			"max_payload_bytes": "max_payload_bytes",
		})
	case "availability":
		return validateMQTTPatchObject(raw, map[string]string{
			"enabled":         "bool",
			"topic":           "string",
			"online_payload":  "string",
			"offline_payload": "string",
			"qos":             "qos",
			"retain":          "bool",
		})
	default:
		return fmt.Errorf("unknown field")
	}
	return nil
}

func validateMQTTPatchTopics(raw interface{}) error {
	switch values := raw.(type) {
	case []string:
		for i, value := range values {
			if err := validateMQTTTopicFilter(value); err != nil {
				return fmt.Errorf("item %d: %w", i, err)
			}
		}
		return nil
	case []interface{}:
		for i, value := range values {
			text, ok := value.(string)
			if !ok {
				return fmt.Errorf("item %d must be a string", i)
			}
			if err := validateMQTTTopicFilter(text); err != nil {
				return fmt.Errorf("item %d: %w", i, err)
			}
		}
		return nil
	default:
		return fmt.Errorf("must be an array of topic filters")
	}
}

func validateMQTTPatchObject(raw interface{}, types map[string]string) error {
	object, ok := raw.(map[string]interface{})
	if !ok {
		return fmt.Errorf("must be an object")
	}
	for key, value := range object {
		kind, known := types[key]
		if !known {
			return fmt.Errorf("unknown field %q", key)
		}
		if value == nil {
			return fmt.Errorf("%s must not be null", key)
		}
		switch kind {
		case "bool":
			if _, ok := value.(bool); !ok {
				return fmt.Errorf("%s must be a boolean", key)
			}
		case "string":
			if _, ok := value.(string); !ok {
				return fmt.Errorf("%s must be a string", key)
			}
		case "qos":
			if err := validateMQTTPatchInt(value, 0, 2); err != nil {
				return fmt.Errorf("%s: %w", key, err)
			}
		case "max_messages":
			if err := validateMQTTPatchInt(value, 0, 10000); err != nil {
				return fmt.Errorf("%s: %w", key, err)
			}
		case "max_age_hours":
			if err := validateMQTTPatchInt(value, 0, 8760); err != nil {
				return fmt.Errorf("%s: %w", key, err)
			}
		case "max_payload_bytes":
			if err := validateMQTTPatchPayloadBytes(value); err != nil {
				return fmt.Errorf("%s: %w", key, err)
			}
		}
	}
	return nil
}

func validateMQTTPatchPayloadBytes(raw interface{}) error {
	if err := validateMQTTPatchInt(raw, 0, 10*1024*1024); err != nil {
		return err
	}
	value, err := mqttPatchIntValue(raw)
	if err != nil {
		return err
	}
	if value != 0 && value < 1024 {
		return fmt.Errorf("must be 0 or between 1024 and 10485760")
	}
	return nil
}

func validateMQTTPatchInt(raw interface{}, min, max int) error {
	value, err := mqttPatchIntValue(raw)
	if err != nil {
		return err
	}
	if value < min || value > max {
		return fmt.Errorf("must be between %d and %d", min, max)
	}
	return nil
}

func mqttPatchIntValue(raw interface{}) (int, error) {
	var value int
	switch typed := raw.(type) {
	case int:
		value = typed
	case int64:
		if int64(int(typed)) != typed {
			return 0, fmt.Errorf("must be an integer")
		}
		value = int(typed)
	case float64:
		if typed != float64(int(typed)) {
			return 0, fmt.Errorf("must be an integer")
		}
		value = int(typed)
	case float32:
		if typed != float32(int(typed)) {
			return 0, fmt.Errorf("must be an integer")
		}
		value = int(typed)
	default:
		return 0, fmt.Errorf("must be a number")
	}
	return value, nil
}

func validateMQTTBrokerURL(broker string) error {
	u, err := url.Parse(broker)
	if err != nil {
		return fmt.Errorf("invalid MQTT broker URL: %w", err)
	}
	if u.User != nil {
		return fmt.Errorf("mqtt.broker must not contain credentials; use mqtt.username and the Vault password")
	}
	scheme := strings.ToLower(u.Scheme)
	if !mqttSecureScheme(scheme) && !mqttPlainScheme(scheme) {
		return fmt.Errorf("unsupported MQTT broker URL scheme %q", u.Scheme)
	}
	if scheme == "unix" {
		if u.Host == "" && u.Path == "" {
			return fmt.Errorf("MQTT unix broker URL requires a socket path")
		}
	} else if u.Hostname() == "" {
		return fmt.Errorf("MQTT broker URL requires a host")
	}
	if port := u.Port(); port != "" {
		parsed, parseErr := strconv.Atoi(port)
		if parseErr != nil || parsed < 1 || parsed > 65535 {
			return fmt.Errorf("MQTT broker URL port must be between 1 and 65535")
		}
	}
	return nil
}

func validateMQTTTopicFilter(topic string) error {
	if topic == "" {
		return fmt.Errorf("topic is required")
	}
	if len([]byte(topic)) > 65535 {
		return fmt.Errorf("topic exceeds MQTT length limit")
	}
	if strings.ContainsRune(topic, '\x00') {
		return fmt.Errorf("topic must not contain null bytes")
	}
	levels := strings.Split(topic, "/")
	for index, level := range levels {
		if strings.Contains(level, "#") && (level != "#" || index != len(levels)-1) {
			return fmt.Errorf("multi-level wildcard must occupy the final topic level")
		}
		if strings.Contains(level, "+") && level != "+" {
			return fmt.Errorf("single-level wildcard must occupy an entire topic level")
		}
	}
	return nil
}

func validateMQTTPublishTopic(topic string) error {
	if err := validateMQTTTopicFilter(topic); err != nil {
		return err
	}
	if strings.ContainsAny(topic, "+#") {
		return fmt.Errorf("publish topic must not contain MQTT wildcards")
	}
	return nil
}

// ResolveMQTTPassword applies the credential precedence used by the runtime:
// Vault mqtt_password, then raw MQTT_PASSWORD, then an empty value. Password
// bytes are returned verbatim; in particular, surrounding whitespace is data.
func ResolveMQTTPassword(vault SecretReader) (string, string, error) {
	if mqttSecretReaderAvailable(vault) {
		value, err := vault.ReadSecret(MQTTPasswordVaultKey)
		if err == nil {
			if value != "" {
				return value, MQTTPasswordSourceVault, nil
			}
		} else if !mqttSecretMissingError(err) {
			return "", "", fmt.Errorf("read MQTT password from vault: %w", err)
		}
	}
	if value, ok := os.LookupEnv("MQTT_PASSWORD"); ok && value != "" {
		return value, MQTTPasswordSourceEnv, nil
	}
	return "", MQTTPasswordSourceEmpty, nil
}

func mqttSecretReaderAvailable(vault SecretReader) bool {
	if vault == nil {
		return false
	}
	rv := reflect.ValueOf(vault)
	switch rv.Kind() {
	case reflect.Pointer, reflect.Interface, reflect.Map, reflect.Slice, reflect.Func:
		return !rv.IsNil()
	default:
		return true
	}
}

func mqttSecretMissingError(err error) bool {
	if err == nil {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(err.Error())) {
	case "not found", "secret not found":
		return true
	default:
		return false
	}
}

// ResolveMQTTPasswordSource reports only the selected credential source.
func ResolveMQTTPasswordSource(vault SecretReader) (string, error) {
	_, source, err := ResolveMQTTPassword(vault)
	return source, err
}
