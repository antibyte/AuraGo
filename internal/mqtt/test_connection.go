package mqtt

import (
	"fmt"
	"log/slog"
	"time"

	"aurago/internal/config"

	pahomqtt "github.com/eclipse/paho.mqtt.golang"
)

// TestConnection opens a short-lived MQTT connection using the current config.
func TestConnection(cfg *config.Config, log *slog.Logger) error {
	if cfg == nil || !cfg.MQTT.Enabled {
		return fmt.Errorf("MQTT integration is not enabled")
	}
	if cfg.MQTT.Broker == "" {
		return fmt.Errorf("MQTT broker URL is not configured")
	}
	testCfg := *cfg
	testCfg.MQTT.ClientID = fmt.Sprintf("%s-test-%d", cfg.MQTT.ClientID, time.Now().UnixNano())
	cleanSession := true
	testCfg.MQTT.CleanSession = &cleanSession
	testCfg.MQTT.Availability.Enabled = false

	opts, err := newClientOptions(&testCfg, log)
	if err != nil {
		recordError(err)
		return err
	}
	opts.SetAutoReconnect(false)
	opts.SetConnectRetry(false)

	connectTimeout := time.Duration(cfg.MQTT.ConnectTimeout) * time.Second
	if connectTimeout <= 0 {
		connectTimeout = 15 * time.Second
	}
	c := pahomqtt.NewClient(opts)
	token := c.Connect()
	if !token.WaitTimeout(connectTimeout) {
		err := fmt.Errorf("MQTT connection test timed out after %s", connectTimeout)
		recordError(err)
		return err
	}
	if err := token.Error(); err != nil {
		recordError(err)
		return fmt.Errorf("MQTT connection test failed: %w", err)
	}
	c.Disconnect(250)
	return nil
}
