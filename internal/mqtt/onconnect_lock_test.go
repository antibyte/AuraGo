package mqtt

import (
	"context"
	"testing"
	"time"

	"aurago/internal/config"
)

func TestOnConnectReleasesCallbackLockBeforeBrokerWait(t *testing.T) {
	broker, _ := lifecycleBroker(t)
	controller := NewMQTTController(nil)
	t.Cleanup(func() { _ = controller.Stop(context.Background()) })

	observed := make(chan bool, 1)
	controller.reconcileHook = func(gen *mqttGeneration, _ *config.Config) {
		released := gen.callbackMu.TryLock()
		if released {
			gen.callbackMu.Unlock()
		}
		observed <- released
	}

	cfg := &config.Config{}
	cfg.MQTT.Enabled = true
	cfg.MQTT.Broker = broker
	controller.UpdateConfig(cfg)

	select {
	case released := <-observed:
		if !released {
			t.Fatal("callbackMu held while reconcile waits on the broker")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("reconcile hook did not run")
	}
}
