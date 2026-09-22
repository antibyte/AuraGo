package mqtt

import (
	"context"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"aurago/internal/config"

	pahomqtt "github.com/eclipse/paho.mqtt.golang"
)

// subscriptionRegressionClient makes the broker acknowledgement explicit so
// these tests can exercise partial SUBACKs without changing the shared fake.
type subscriptionRegressionClient struct {
	*fakeMQTTClient
	ack           map[string]byte
	block         bool
	subscribeOnce sync.Once
	subscribeDone chan struct{}
}

func newSubscriptionRegressionClient(ack map[string]byte) *subscriptionRegressionClient {
	copyAck := make(map[string]byte, len(ack))
	for topic, code := range ack {
		copyAck[topic] = code
	}
	return &subscriptionRegressionClient{
		fakeMQTTClient: newFakeMQTTClient(),
		ack:            copyAck,
		subscribeDone:  make(chan struct{}),
	}
}

func cloneSubscriptionRegressionACK(src map[string]byte) map[string]byte {
	dst := make(map[string]byte, len(src))
	for topic, code := range src {
		dst[topic] = code
	}
	return dst
}

func (c *subscriptionRegressionClient) SubscribeMultiple(filters map[string]byte, _ pahomqtt.MessageHandler) pahomqtt.Token {
	c.subscribeMultipleRuns++
	c.subscribeOnce.Do(func() { close(c.subscribeDone) })
	if c.block {
		return &blockingSubscriptionRegressionToken{done: make(chan struct{}), result: cloneSubscriptionRegressionACK(c.ack)}
	}
	ack := cloneSubscriptionRegressionACK(c.ack)
	for topic, code := range c.ack {
		if _, expected := filters[topic]; expected && code <= 2 {
			if code > filters[topic] {
				code = filters[topic]
				ack[topic] = code
			}
			c.subscriptions[topic] = code
		}
	}
	return newFakeMQTTSubscribeToken(ack)
}

type blockingSubscriptionRegressionToken struct {
	done   chan struct{}
	result map[string]byte
}

func (t *blockingSubscriptionRegressionToken) Wait() bool { return false }

func (t *blockingSubscriptionRegressionToken) WaitTimeout(time.Duration) bool { return false }

func (t *blockingSubscriptionRegressionToken) Done() <-chan struct{} { return t.done }

func (t *blockingSubscriptionRegressionToken) Error() error { return nil }

func (t *blockingSubscriptionRegressionToken) Result() map[string]byte { return t.result }

func subscriptionRegressionConfig(topics ...string) *config.Config {
	cfg := &config.Config{}
	cfg.MQTT.Enabled = true
	cfg.MQTT.Broker = "tcp://broker.test:1883"
	cfg.MQTT.ClientID = "subscription-regression"
	cfg.MQTT.Topics = append([]string(nil), topics...)
	cfg.MQTT.QoS = 1
	return cfg
}

func newSubscriptionRegressionFixture(t *testing.T, cfg *config.Config, client pahomqtt.Client) (*MQTTController, *mqttGeneration) {
	t.Helper()
	controller := NewMQTTController(mqttTestLogger())
	workCtx, cancelWork := context.WithCancel(context.Background())
	gen := &mqttGeneration{
		ctx:          workCtx,
		cancel:       cancelWork,
		client:       client,
		done:         make(chan struct{}),
		missionQueue: make(chan missionJob, 256),
	}
	controller.initializeGenerationSubscriptions(gen, cfg)
	controller.mu.Lock()
	controller.current = gen
	controller.desired = cloneMQTTSnapshot(cfg)
	controller.desiredRevision = 1
	controller.active = cloneMQTTSnapshot(cfg)
	controller.activeRevision = 1
	controller.state = "connected"
	controller.mu.Unlock()

	// Keep direct fixtures from leaking a worker or a generation into later
	// tests. The runtime worker remains the only owner of production teardown.
	t.Cleanup(func() {
		gen.closed.Store(true)
		cancelWork()
		controller.mu.Lock()
		if controller.current == gen {
			controller.current = nil
		}
		controller.mu.Unlock()
	})
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := controller.Stop(ctx); err != nil {
			t.Errorf("stop subscription regression fixture: %v", err)
		}
	})
	return controller, gen
}

func subscriptionRegressionStatus(t *testing.T, gen *mqttGeneration, filter string) MQTTSubscriptionStatus {
	t.Helper()
	gen.subscriptionMu.Lock()
	defer gen.subscriptionMu.Unlock()
	status, ok := gen.subscriptions[filter]
	if !ok {
		t.Fatalf("subscription status for %q is missing: %+v", filter, gen.subscriptions)
	}
	return status
}

func TestMQTTSubscriptionsMixedSUBACKKeepsGrantedOwner(t *testing.T) {
	const (
		granted  = "mixed/granted"
		missing  = "mixed/missing"
		rejected = "mixed/rejected"
	)
	client := newSubscriptionRegressionClient(map[string]byte{
		granted:  1,
		rejected: 0x80,
	})
	cfg := subscriptionRegressionConfig(granted, missing, rejected)
	controller, gen := newSubscriptionRegressionFixture(t, cfg, client)

	controller.reconcileGenerationSubscriptions(gen, cfg)

	if client.subscribeMultipleRuns != 1 {
		t.Fatalf("SubscribeMultiple calls = %d, want 1", client.subscribeMultipleRuns)
	}
	gen.subscriptionMu.Lock()
	gotGranted, hasGranted := gen.grantedFilters[granted]
	gotDesired, hasDesired := gen.grantedDesired[granted]
	gen.subscriptionMu.Unlock()
	if !hasGranted || gotGranted != 1 || !hasDesired || gotDesired != 1 {
		t.Fatalf("partial SUBACK lost granted filter: granted=%v desired=%v", gen.grantedFilters, gen.grantedDesired)
	}
	status := subscriptionRegressionStatus(t, gen, granted)
	if status.Pending || status.Failed || status.GrantedQoS != 1 {
		t.Fatalf("granted status = %+v, want active grant", status)
	}
	for _, filter := range []string{missing, rejected} {
		status := subscriptionRegressionStatus(t, gen, filter)
		if !status.Pending || !status.Failed || status.LastError == "" {
			t.Fatalf("partial SUBACK status for %q = %+v, want pending failure", filter, status)
		}
	}
	if status := subscriptionRegressionStatus(t, gen, missing); !strings.Contains(status.LastError, "omitted") {
		t.Fatalf("missing SUBACK error = %q, want omitted-topic detail", status.LastError)
	}
}

func TestMQTTSubscriptionsUnexpectedSUBACKTopicIsProtocolError(t *testing.T) {
	const granted = "protocol/granted"
	client := newSubscriptionRegressionClient(map[string]byte{
		granted:          1,
		"protocol/extra": 0,
	})
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	result := subscribeMultipleResult(client, map[string]byte{granted: 1}, nil, ctx, time.Second)
	if result.err == nil || !strings.Contains(result.err.Error(), "unexpected topic") {
		t.Fatalf("unexpected SUBACK topic error = %v, want protocol error", result.err)
	}
	if len(result.granted) != 0 {
		t.Fatalf("extra SUBACK topic was accepted with grants: %+v", result.granted)
	}
}

func TestMQTTSubscriptionOwnerQoSAndManualUnsubscribeRetainConfigOwner(t *testing.T) {
	const topic = "shared/topic"
	client := newSubscriptionRegressionClient(map[string]byte{topic: 2})
	cfg := subscriptionRegressionConfig(topic)
	controller, gen := newSubscriptionRegressionFixture(t, cfg, client)
	controller.setManualTopic(topic, 2)

	owners, filters := desiredSubscriptionState(controller, cfg)
	if got := filters[topic]; got != 2 {
		t.Fatalf("desired QoS = %d, want maximum owner QoS 2 (owners=%+v)", got, owners[topic])
	}
	if len(owners[topic]) != 2 {
		t.Fatalf("owners before unsubscribe = %+v, want config and manual owners", owners[topic])
	}
	controller.updateGenerationSubscriptions(gen, cfg)
	controller.reconcileGenerationSubscriptions(gen, cfg)

	result, err := controller.unsubscribeDetail(topic, mqttTestLogger())
	if err != nil {
		t.Fatalf("unsubscribe manual owner: %v", err)
	}
	if result.Removed {
		t.Fatalf("manual unsubscribe removed shared broker filter: %+v", result)
	}
	if client.subscribeMultipleRuns != 2 {
		t.Fatalf("SubscribeMultiple calls after QoS owner removal = %d, want initial QoS2 plus config QoS1", client.subscribeMultipleRuns)
	}
	if len(result.RemainingOwners) != 1 || result.RemainingOwners[0].Kind != SubscriptionOwnerConfig {
		t.Fatalf("remaining owners = %+v, want one config owner", result.RemainingOwners)
	}
	for _, unsubscribed := range client.unsubscribed {
		if unsubscribed == topic {
			t.Fatalf("manual unsubscribe sent broker unsubscribe while config owner remained: %v", client.unsubscribed)
		}
	}
	status := subscriptionRegressionStatus(t, gen, topic)
	if status.Pending || status.Failed || status.QoS != 1 || status.GrantedQoS != 1 {
		t.Fatalf("shared filter status after manual unsubscribe = %+v", status)
	}
}

func TestMQTTSubscriptionsGrantedQoSZeroDoesNotResubscribeUnchangedDesired(t *testing.T) {
	const topic = "qos-zero/topic"
	client := newSubscriptionRegressionClient(map[string]byte{topic: 0})
	cfg := subscriptionRegressionConfig(topic)
	controller, gen := newSubscriptionRegressionFixture(t, cfg, client)

	controller.reconcileGenerationSubscriptions(gen, cfg)
	controller.reconcileGenerationSubscriptions(gen, cfg)

	if client.subscribeMultipleRuns != 1 {
		t.Fatalf("SubscribeMultiple calls for unchanged QoS0 grant = %d, want 1", client.subscribeMultipleRuns)
	}
	status := subscriptionRegressionStatus(t, gen, topic)
	if status.Pending || status.Failed || status.QoS != 1 || status.GrantedQoS != 0 {
		t.Fatalf("unchanged QoS0 subscription status = %+v", status)
	}
}

func TestMQTTSubscriptionsMissingSUBACKStopsOnGenerationCancelAndStatusRemainsReachable(t *testing.T) {
	for _, mode := range []string{"stop", "disable"} {
		t.Run(mode, func(t *testing.T) {
			cfg := subscriptionRegressionConfig("cancel/missing")
			client := newSubscriptionRegressionClient(nil)
			client.block = true
			controller, gen := newSubscriptionRegressionFixture(t, cfg, client)
			reconcileDone := make(chan struct{})
			go func() {
				controller.reconcileGenerationSubscriptions(gen, cfg)
				close(reconcileDone)
			}()
			select {
			case <-client.subscribeDone:
			case <-time.After(time.Second):
				t.Fatal("reconcile did not reach the pending SUBACK")
			}
			statusBeforeDone := make(chan MQTTStatus, 1)
			go func() { statusBeforeDone <- controller.Status() }()
			select {
			case status := <-statusBeforeDone:
				if len(status.Subscriptions) == 0 {
					t.Fatalf("status while SUBACK was pending lost subscription state: %+v", status)
				}
			case <-time.After(time.Second):
				t.Fatal("MQTT status was blocked by pending SUBACK")
			}

			switch mode {
			case "stop":
				if err := controller.Stop(context.Background()); err != nil {
					t.Fatalf("public MQTT controller stop: %v", err)
				}
			case "disable":
				disabled := subscriptionRegressionConfig("cancel/missing")
				disabled.MQTT.Enabled = false
				controller.UpdateConfig(disabled)
			}
			select {
			case <-reconcileDone:
			case <-time.After(time.Second):
				t.Fatal("pending SUBACK did not abort after generation stop/disable")
			}

			statusDone := make(chan MQTTStatus, 1)
			go func() { statusDone <- controller.Status() }()
			select {
			case status := <-statusDone:
				if mode == "disable" && status.DesiredEnabled {
					t.Fatalf("disabled controller still reports desired MQTT enabled: %+v", status)
				}
			case <-time.After(time.Second):
				t.Fatal("MQTT status became unreachable after pending SUBACK cancellation")
			}
		})
	}
}

func TestMQTTMissionQueueIsBoundedAndDropsStaleGenerationCallbacks(t *testing.T) {
	cfg := subscriptionRegressionConfig("mission/queue")
	client := newSubscriptionRegressionClient(nil)
	controller, gen := newSubscriptionRegressionFixture(t, cfg, client)
	trigger := missionTriggerEntry{topicFilter: "mission/#", callback: func(string, string) {}}

	droppedBefore := atomic.LoadUint64(&stats.droppedMissionJobs)
	for i := 0; i < cap(gen.missionQueue)+1; i++ {
		controller.enqueueMission(gen, trigger, "mission/queue", "payload")
	}
	if got := len(gen.missionQueue); got != cap(gen.missionQueue) {
		t.Fatalf("mission queue length = %d, want bounded capacity %d", got, cap(gen.missionQueue))
	}
	if got := atomic.LoadUint64(&stats.droppedMissionJobs); got <= droppedBefore {
		t.Fatalf("mission queue overflow did not increment dropped count: before=%d after=%d", droppedBefore, got)
	}

	gen.closed.Store(true)
	gen.cancel()
	controller.enqueueMission(gen, trigger, "mission/stale", "payload")
	if got := len(gen.missionQueue); got != cap(gen.missionQueue) {
		t.Fatalf("stale generation callback changed full queue length to %d", got)
	}
}
