package config

import (
	"errors"
	"os"
	"strings"
	"testing"
)

type mqttValidationVault struct {
	value string
	err   error
}

func (v mqttValidationVault) ReadSecret(string) (string, error) {
	return v.value, v.err
}

func TestMQTTEffectiveTLSUsesTransportScheme(t *testing.T) {
	tests := []struct {
		name       string
		broker     string
		configured bool
		want       bool
		wantErr    bool
	}{
		{name: "secure scheme enables TLS", broker: "mqtts://broker.example:8883", want: true},
		{name: "secure scheme ignores false flag", broker: "ssl://broker.example:8883", want: true},
		{name: "plain scheme remains plain", broker: "tcp://broker.example:1883", want: false},
		{name: "TLS flag rejects plaintext", broker: "tcp://broker.example:1883", configured: true, wantErr: true},
		{name: "unix is plaintext", broker: "unix:///var/run/mosquitto.sock", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{}
			cfg.MQTT.Broker = tt.broker
			cfg.MQTT.TLS.Enabled = tt.configured
			got, err := MQTTEffectiveTLS(cfg)
			if (err != nil) != tt.wantErr {
				t.Fatalf("MQTTEffectiveTLS() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err == nil && got != tt.want {
				t.Fatalf("MQTTEffectiveTLS() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidateMQTTConfigRejectsInvalidTopicsAndBounds(t *testing.T) {
	valid := &Config{}
	valid.MQTT.Enabled = true
	valid.MQTT.Broker = "tcp://localhost:1883"
	valid.MQTT.ConnectTimeout = 15
	valid.MQTT.Buffer.MaxMessages = 500
	valid.MQTT.Buffer.MaxPayloadBytes = 262144
	valid.MQTT.Topics = []string{"home/#", "sensors/+"}
	if err := ValidateMQTTConfig(valid); err != nil {
		t.Fatalf("valid MQTT config rejected: %v", err)
	}

	cases := []struct {
		name string
		edit func(*Config)
	}{
		{name: "wildcard placement", edit: func(c *Config) { c.MQTT.Topics = []string{"home/#/bad"} }},
		{name: "availability wildcard", edit: func(c *Config) { c.MQTT.Availability.Topic = "aurago/+" }},
		{name: "qos", edit: func(c *Config) { c.MQTT.QoS = 3 }},
		{name: "timeout", edit: func(c *Config) { c.MQTT.ConnectTimeout = 121 }},
		{name: "payload bound", edit: func(c *Config) { c.MQTT.Buffer.MaxPayloadBytes = 1023 }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := *valid
			tc.edit(&cfg)
			if err := ValidateMQTTConfig(&cfg); err == nil {
				t.Fatal("ValidateMQTTConfig unexpectedly accepted invalid config")
			}
		})
	}
}

func TestValidateMQTTPatchRequiresTypedMQTTValues(t *testing.T) {
	if err := ValidateMQTTPatch(map[string]interface{}{"agent": map[string]interface{}{"enabled": true}}); err != nil {
		t.Fatalf("unrelated patch rejected: %v", err)
	}
	valid := map[string]interface{}{"mqtt": map[string]interface{}{
		"topics": []interface{}{"home/#", "sensors/+"},
		"qos":    float64(0),
	}}
	if err := ValidateMQTTPatch(valid); err != nil {
		t.Fatalf("valid typed patch rejected: %v", err)
	}
	for _, patch := range []map[string]interface{}{
		{"mqtt": map[string]interface{}{"topics": "home/#"}},
		{"mqtt": map[string]interface{}{"qos": "0"}},
		{"mqtt": map[string]interface{}{"tls": map[string]interface{}{"enabled": "true"}}},
		{"mqtt": map[string]interface{}{"buffer": map[string]interface{}{"max_payload_bytes": float64(5)}}},
	} {
		if err := ValidateMQTTPatch(patch); err == nil {
			t.Fatalf("invalid MQTT patch accepted: %#v", patch)
		}
	}
}

func TestResolveMQTTPasswordPrecedencePreservesWhitespace(t *testing.T) {
	t.Setenv("MQTT_PASSWORD", " env password ")
	value, source, err := ResolveMQTTPassword(mqttValidationVault{value: " vault password "})
	if err != nil || value != " vault password " || source != MQTTPasswordSourceVault {
		t.Fatalf("vault result = %q, %q, %v", value, source, err)
	}
	value, source, err = ResolveMQTTPassword(mqttValidationVault{err: errors.New("secret not found")})
	if err != nil || value != " env password " || source != MQTTPasswordSourceEnv {
		t.Fatalf("env result = %q, %q, %v", value, source, err)
	}
	value, source, err = ResolveMQTTPassword(mqttValidationVault{err: errors.New("vault I/O failed")})
	if err == nil || value != "" || source != "" {
		t.Fatalf("vault I/O result = %q, %q, %v", value, source, err)
	}
	t.Setenv("MQTT_PASSWORD", "")
	value, source, err = ResolveMQTTPassword(mqttValidationVault{err: errors.New("not found")})
	if err != nil || value != "" || source != MQTTPasswordSourceEmpty {
		t.Fatalf("empty result = %q, %q, %v", value, source, err)
	}
}

func TestApplyVaultSecretsUsesMQTTResolver(t *testing.T) {
	t.Setenv("MQTT_PASSWORD", " env fallback ")
	cfg := &Config{}
	cfg.ApplyVaultSecrets(mqttValidationVault{value: "  vault exact  "})
	if cfg.MQTT.Password != "  vault exact  " {
		t.Fatalf("MQTT password = %q, want exact Vault value", cfg.MQTT.Password)
	}
	cfg.ApplyVaultSecrets(mqttValidationVault{err: errors.New("secret not found")})
	if cfg.MQTT.Password != " env fallback " {
		t.Fatalf("MQTT password = %q, want exact environment value", cfg.MQTT.Password)
	}
	if strings.TrimSpace(cfg.MQTT.Password) != "env fallback" {
		t.Fatal("test environment setup unexpectedly changed password")
	}
	_ = os.Getenv("MQTT_PASSWORD")
}
