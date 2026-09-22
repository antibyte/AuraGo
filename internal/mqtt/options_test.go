package mqtt

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/config"
)

func mqttOptionsTestConfig(broker string) *config.Config {
	cfg := &config.Config{}
	cfg.MQTT.Broker = broker
	cfg.MQTT.ConnectTimeout = 15
	cfg.MQTT.Buffer.MaxMessages = 500
	cfg.MQTT.Buffer.MaxPayloadBytes = 262144
	return cfg
}

func TestNewClientOptionsSecureSchemeAlwaysBuildsTLSConfig(t *testing.T) {
	cfg := mqttOptionsTestConfig("mqtts://broker.example:8883")
	cfg.MQTT.TLS.Enabled = false
	opts, err := newClientOptions(cfg, nil)
	if err != nil {
		t.Fatalf("newClientOptions() error = %v", err)
	}
	if opts.TLSConfig == nil {
		t.Fatal("secure broker did not receive a TLS config")
	}
	if len(opts.Servers) != 1 || opts.Servers[0].String() != cfg.MQTT.Broker {
		t.Fatalf("broker URL was rewritten: %#v", opts.Servers)
	}
}

func TestValidateConfigRejectsTLSFlagOnPlainTransport(t *testing.T) {
	cfg := mqttOptionsTestConfig("tcp://127.0.0.1:1883")
	cfg.MQTT.TLS.Enabled = true
	if err := ValidateConfig(cfg); err == nil || !strings.Contains(strings.ToLower(err.Error()), "plaintext") {
		t.Fatalf("ValidateConfig() error = %v, want plaintext transport rejection", err)
	}
}

func TestValidateConfigRejectsInvalidTLSMaterialBeforeConnect(t *testing.T) {
	t.Run("invalid CA PEM", func(t *testing.T) {
		caFile := filepath.Join(t.TempDir(), "ca.pem")
		if err := os.WriteFile(caFile, []byte("not a certificate"), 0o600); err != nil {
			t.Fatal(err)
		}
		cfg := mqttOptionsTestConfig("mqtts://broker.example:8883")
		cfg.MQTT.TLS.CAFile = caFile
		if err := ValidateConfig(cfg); err == nil || !strings.Contains(strings.ToLower(err.Error()), "ca certificate") {
			t.Fatalf("ValidateConfig() error = %v, want invalid CA error", err)
		}
	})
	t.Run("partial client certificate", func(t *testing.T) {
		cfg := mqttOptionsTestConfig("mqtts://broker.example:8883")
		cfg.MQTT.TLS.CertFile = filepath.Join(t.TempDir(), "client.crt")
		if err := ValidateConfig(cfg); err == nil || !strings.Contains(strings.ToLower(err.Error()), "certificate and key") {
			t.Fatalf("ValidateConfig() error = %v, want partial cert/key error", err)
		}
	})
}
