package mqtt

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log/slog"
	"os"
	"time"

	"aurago/internal/config"

	pahomqtt "github.com/eclipse/paho.mqtt.golang"
)

func newClientOptions(cfg *config.Config, log *slog.Logger) (*pahomqtt.ClientOptions, error) {
	clientID := cfg.MQTT.ClientID
	if clientID == "" {
		clientID = "aurago"
	}
	opts := pahomqtt.NewClientOptions().
		AddBroker(cfg.MQTT.Broker).
		SetClientID(clientID).
		SetAutoReconnect(true).
		SetConnectRetry(true).
		SetConnectRetryInterval(10 * time.Second).
		SetKeepAlive(30 * time.Second).
		SetCleanSession(mqttCleanSession(cfg)).
		SetOrderMatters(false)

	if cfg.MQTT.Username != "" {
		opts.SetUsername(cfg.MQTT.Username)
	}
	if cfg.MQTT.Password != "" {
		opts.SetPassword(cfg.MQTT.Password)
	}
	if cfg.MQTT.Availability.Enabled {
		if err := configureAvailabilityWill(opts, cfg); err != nil {
			return nil, err
		}
	}
	if cfg.MQTT.TLS.Enabled {
		tlsConfig, err := mqttTLSConfig(cfg, log)
		if err != nil {
			return nil, err
		}
		opts.SetTLSConfig(tlsConfig)
		if log != nil {
			log.Info("[MQTT] TLS enabled", "ca_file", cfg.MQTT.TLS.CAFile,
				"cert_file", cfg.MQTT.TLS.CertFile, "insecure_skip_verify", cfg.MQTT.TLS.InsecureSkipVerify)
		}
	}
	return opts, nil
}

func mqttCleanSession(cfg *config.Config) bool {
	if cfg.MQTT.CleanSession == nil {
		return true
	}
	return *cfg.MQTT.CleanSession
}

func configureAvailabilityWill(opts *pahomqtt.ClientOptions, cfg *config.Config) error {
	topic := mqttAvailabilityTopic(cfg)
	if err := validatePublishTopic(topic); err != nil {
		return fmt.Errorf("invalid MQTT availability topic: %w", err)
	}
	offlinePayload := cfg.MQTT.Availability.OfflinePayload
	if offlinePayload == "" {
		offlinePayload = "offline"
	}
	opts.SetWill(topic, offlinePayload, mqttQoS(cfg.MQTT.Availability.QoS, 1), cfg.MQTT.Availability.Retain)
	return nil
}

func mqttTLSConfig(cfg *config.Config, log *slog.Logger) (*tls.Config, error) {
	tlsConfig := &tls.Config{InsecureSkipVerify: cfg.MQTT.TLS.InsecureSkipVerify} //nolint:gosec // Explicit user setting for self-signed brokers.
	if cfg.MQTT.TLS.CAFile != "" {
		caCert, err := os.ReadFile(cfg.MQTT.TLS.CAFile)
		if err != nil {
			return nil, fmt.Errorf("read CA certificate %s: %w", cfg.MQTT.TLS.CAFile, err)
		}
		caCertPool, err := x509.SystemCertPool()
		if err != nil {
			return nil, fmt.Errorf("load system certificate pool: %w", err)
		}
		if caCertPool == nil {
			caCertPool = x509.NewCertPool()
		}
		if !caCertPool.AppendCertsFromPEM(caCert) && log != nil {
			log.Warn("[MQTT] No certificates appended from CA file")
		}
		tlsConfig.RootCAs = caCertPool
	}
	if cfg.MQTT.TLS.CertFile != "" && cfg.MQTT.TLS.KeyFile != "" {
		cert, err := tls.LoadX509KeyPair(cfg.MQTT.TLS.CertFile, cfg.MQTT.TLS.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("load client certificate: %w", err)
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}
	return tlsConfig, nil
}

func mqttAvailabilityTopic(cfg *config.Config) string {
	if cfg.MQTT.Availability.Topic != "" {
		return cfg.MQTT.Availability.Topic
	}
	return "aurago/status"
}

func mqttQoS(value, fallback int) byte {
	if value < 0 || value > 2 {
		value = fallback
	}
	if value < 0 || value > 2 {
		value = 0
	}
	return byte(value)
}

func publishAvailability(c pahomqtt.Client, cfg *config.Config, log *slog.Logger) {
	if !cfg.MQTT.Availability.Enabled || c == nil || !c.IsConnected() {
		return
	}
	topic := mqttAvailabilityTopic(cfg)
	if err := validatePublishTopic(topic); err != nil {
		recordError(err)
		if log != nil {
			log.Warn("[MQTT] Invalid availability topic", "error", err)
		}
		return
	}
	payload := cfg.MQTT.Availability.OnlinePayload
	if payload == "" {
		payload = "online"
	}
	token := c.Publish(topic, mqttQoS(cfg.MQTT.Availability.QoS, 1), cfg.MQTT.Availability.Retain, payload)
	if !token.WaitTimeout(5 * time.Second) {
		err := fmt.Errorf("MQTT availability publish timed out")
		recordError(err)
		if log != nil {
			log.Warn("[MQTT] Availability publish timed out", "topic", topic)
		}
		return
	}
	if err := token.Error(); err != nil {
		recordError(err)
		if log != nil {
			log.Warn("[MQTT] Availability publish failed", "topic", topic, "error", err)
		}
	}
}
