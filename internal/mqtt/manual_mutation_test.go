package mqtt

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

func TestMQTTManualSubscribePendingRestartPreservesOnlyPreviousConfirmation(t *testing.T) {
	zero := byte(0)
	for _, tc := range []struct {
		name     string
		previous *byte
	}{
		{name: "new owner stays unconfirmed"},
		{name: "existing qos zero survives change", previous: &zero},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := ledgerTestConfig(t, "tcp://broker.test:1883", "manual-pending-restart")
			cfg.MQTT.Enabled = true
			const topic = "manual/pending-change"
			if tc.previous != nil {
				writeLedgerTestEntries(t, cfg, []subscriptionLedgerEntry{
					ledgerTestEntry(topic, SubscriptionOwnerManual, topic, *tc.previous, *tc.previous, "active", false, false),
				})
			}
			client := newSubscriptionRegressionClient(nil)
			client.block = true
			controller, gen := newSubscriptionRegressionFixture(t, cfg, client)
			controller.loadManualLedger(cfg)
			controller.initializeGenerationSubscriptions(gen, cfg)
			finished := make(chan error, 1)
			go func() { finished <- controller.subscribe(topic, 2, nil) }()
			select {
			case <-client.subscribeDone:
			case <-time.After(time.Second):
				t.Fatal("manual mutation did not reach broker")
			}
			// Simulate a restart after the request reached the broker but before
			// its SUBACK. The last confirmed owner is the only durable intent.
			restarted, _, _ := newLedgerTestGeneration(t, cfg)
			actual, restored := restarted.manualTopic(topic)
			if tc.previous == nil && restored {
				t.Errorf("unconfirmed new owner restored with QoS %d", actual)
			}
			if tc.previous != nil && (!restored || actual != *tc.previous) {
				t.Errorf("previous confirmed QoS %d lost: restored=%v qos=%d", *tc.previous, restored, actual)
			}
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			if err := controller.Stop(ctx); err != nil {
				t.Fatal(err)
			}
			select {
			case err := <-finished:
				if err == nil {
					t.Error("unconfirmed operation returned success")
				}
			case <-time.After(time.Second):
				t.Fatal("manual operation did not exit on cancellation")
			}
		})
	}
}

func TestMQTTSubscriptionLedgerNeverPersistsRawBrokerErrors(t *testing.T) {
	cfg := ledgerTestConfig(t, "tcp://broker.test:1883", "error-redaction")
	entry := ledgerTestEntry("errors/topic", SubscriptionOwnerConfig, "errors/topic", 1, 0, "failed", true, true)
	entry.LastError = "broker rejected mqtts://user:credential-sentinel@host with payload-sentinel"
	writeLedgerTestEntries(t, cfg, []subscriptionLedgerEntry{entry})
	data, err := os.ReadFile(ManualSubscriptionLedgerPath(cfg))
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"credential-sentinel", "payload-sentinel", "last_error"} {
		if strings.Contains(string(data), forbidden) {
			t.Fatalf("ledger retained raw error content %q", forbidden)
		}
	}
}

func TestMQTTManualUnsubscribeConfirmationSurvivesRestartWithSharedOwner(t *testing.T) {
	cfg := ledgerTestConfig(t, "tcp://broker.test:1883", "manual-remove-restart")
	cfg.MQTT.Enabled = true
	const topic = "shared/confirmed-removal"
	cfg.MQTT.Topics = []string{topic}
	writeLedgerTestEntries(t, cfg, []subscriptionLedgerEntry{
		ledgerTestEntry(topic, SubscriptionOwnerManual, topic, 1, 1, "active", false, false),
	})
	controller, _, _ := newLedgerTestGeneration(t, cfg)
	result, err := controller.unsubscribeDetail(topic, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Removed || len(result.RemainingOwners) != 1 {
		t.Fatalf("shared owner lost: %+v", result)
	}
	restarted, _, _ := newLedgerTestGeneration(t, cfg)
	if _, restored := restarted.manualTopic(topic); restored {
		t.Fatal("confirmed manual removal was undone by restart")
	}
}

func TestMQTTManualUnsubscribeReportsRejectedSharedQoSChange(t *testing.T) {
	cfg := ledgerTestConfig(t, "tcp://broker.test:1883", "manual-rejected-removal")
	cfg.MQTT.Enabled = true
	const topic = "shared/rejected-removal"
	cfg.MQTT.Topics = []string{topic}
	writeLedgerTestEntries(t, cfg, []subscriptionLedgerEntry{
		ledgerTestEntry(topic, SubscriptionOwnerManual, topic, 1, 1, "active", false, false),
	})
	client := newSubscriptionRegressionClient(map[string]byte{topic: 0x80})
	controller, gen := newSubscriptionRegressionFixture(t, cfg, client)
	controller.loadManualLedger(cfg)
	controller.initializeGenerationSubscriptions(gen, cfg)
	if _, err := controller.unsubscribeDetail(topic, nil); err == nil {
		t.Fatal("broker rejection was reported as a successful unsubscribe")
	}
	if qos, exists := controller.manualTopic(topic); !exists || qos != 1 {
		t.Fatalf("failed removal lost the confirmed owner: qos=%d exists=%v", qos, exists)
	}
	if qos, exists := desiredFilterSnapshot(gen)[topic]; !exists || qos != 1 {
		t.Fatalf("failed removal left obsolete desired state: qos=%d exists=%v", qos, exists)
	}
}

func TestMQTTCleanSessionManualSubscriptionsRemainInMemory(t *testing.T) {
	cfg := ledgerTestConfig(t, "tcp://broker.test:1883", "clean-session-manual")
	clean := true
	cfg.MQTT.CleanSession = &clean
	controller, _, _ := newLedgerTestGeneration(t, cfg)
	if err := controller.subscribe("manual/volatile", 0, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(ManualSubscriptionLedgerPath(cfg)); !os.IsNotExist(err) {
		t.Fatalf("clean session unexpectedly created a persistent ledger: %v", err)
	}
	if _, exists := controller.manualTopic("manual/volatile"); !exists {
		t.Fatal("manual subscription missing from active runtime")
	}
}
