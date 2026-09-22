package mqtt

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"aurago/internal/config"

	pahomqtt "github.com/eclipse/paho.mqtt.golang"
)

// TestConnection opens a short-lived MQTT connection using the current config.
func TestConnection(cfg *config.Config, log *slog.Logger) error {
	return TestConnectionContext(context.Background(), cfg, log)
}

// TestConnectionContext opens a short-lived MQTT connection and tears down the
// actual Paho client on every return path, including caller cancellation and a
// delayed CONNACK after the configured timeout.
func TestConnectionContext(ctx context.Context, cfg *config.Config, log *slog.Logger) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if cfg == nil || !cfg.MQTT.Enabled {
		return fmt.Errorf("MQTT integration is not enabled")
	}
	if cfg.MQTT.Broker == "" {
		return fmt.Errorf("MQTT broker URL is not configured")
	}
	testCfg := *cfg
	testCfg.MQTT = cfg.MQTT
	testCfg.MQTT.Topics = append([]string(nil), cfg.MQTT.Topics...)
	if cfg.MQTT.CleanSession != nil {
		cleanSession := *cfg.MQTT.CleanSession
		testCfg.MQTT.CleanSession = &cleanSession
	}
	testCfg.MQTT.ClientID = fmt.Sprintf("%s-test-%d", cfg.MQTT.ClientID, time.Now().UnixNano())
	cleanSession := true
	testCfg.MQTT.CleanSession = &cleanSession
	testCfg.MQTT.Availability.Enabled = false

	connectTimeout := mqttConnectTimeout(cfg)
	connectCtx, cancelConnect := context.WithCancel(ctx)
	defer cancelConnect()
	opts, err := newClientOptionsContext(connectCtx, &testCfg, log)
	if err != nil {
		recordError(err)
		return err
	}
	opts.SetAutoReconnect(false).SetConnectRetry(false).SetConnectTimeout(connectTimeout)
	c := pahomqtt.NewClient(opts)
	defer func() {
		// Disconnect is intentionally unconditional. Paho also uses this path
		// to cancel a dial/handshake that has not delivered CONNACK yet.
		c.Disconnect(0)
	}()
	token := c.Connect()
	timer := time.NewTimer(connectTimeout)
	defer timer.Stop()
	select {
	case <-token.Done():
		if err := token.Error(); err != nil {
			recordError(err)
			return fmt.Errorf("MQTT connection test failed: %w", err)
		}
		if !c.IsConnectionOpen() {
			err := fmt.Errorf("MQTT connection test completed without an open connection")
			recordError(err)
			return err
		}
		return nil
	case <-timer.C:
		err := fmt.Errorf("MQTT connection test timed out after %s", connectTimeout)
		recordError(err)
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}
