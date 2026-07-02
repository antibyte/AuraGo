package mqtt

import (
	"io"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/tools"

	pahomqtt "github.com/eclipse/paho.mqtt.golang"
)

type fakeMQTTToken struct {
	err  error
	done chan struct{}
}

func newFakeMQTTToken(err error) *fakeMQTTToken {
	done := make(chan struct{})
	close(done)
	return &fakeMQTTToken{err: err, done: done}
}

func (t *fakeMQTTToken) Wait() bool { return true }
func (t *fakeMQTTToken) WaitTimeout(time.Duration) bool {
	return true
}
func (t *fakeMQTTToken) Done() <-chan struct{} { return t.done }
func (t *fakeMQTTToken) Error() error          { return t.err }

type fakeMQTTPublish struct {
	topic   string
	payload interface{}
	qos     byte
	retain  bool
}

type fakeMQTTClient struct {
	connected             bool
	disconnected          bool
	events                []string
	published             []fakeMQTTPublish
	subscriptions         map[string]byte
	subscribeMultipleRuns int
	unsubscribed          []string
}

func newFakeMQTTClient() *fakeMQTTClient {
	return &fakeMQTTClient{
		connected:     true,
		subscriptions: make(map[string]byte),
	}
}

func (c *fakeMQTTClient) IsConnected() bool      { return c.connected }
func (c *fakeMQTTClient) IsConnectionOpen() bool { return c.connected }
func (c *fakeMQTTClient) Connect() pahomqtt.Token {
	return newFakeMQTTToken(nil)
}
func (c *fakeMQTTClient) Disconnect(uint) {
	c.events = append(c.events, "disconnect")
	c.disconnected = true
	c.connected = false
}
func (c *fakeMQTTClient) Publish(topic string, qos byte, retained bool, payload interface{}) pahomqtt.Token {
	c.events = append(c.events, "publish:"+topic)
	c.published = append(c.published, fakeMQTTPublish{topic: topic, payload: payload, qos: qos, retain: retained})
	return newFakeMQTTToken(nil)
}
func (c *fakeMQTTClient) Subscribe(topic string, qos byte, callback pahomqtt.MessageHandler) pahomqtt.Token {
	c.subscriptions[topic] = qos
	return newFakeMQTTToken(nil)
}
func (c *fakeMQTTClient) SubscribeMultiple(filters map[string]byte, callback pahomqtt.MessageHandler) pahomqtt.Token {
	c.subscribeMultipleRuns++
	for topic, qos := range filters {
		c.subscriptions[topic] = qos
	}
	return newFakeMQTTToken(nil)
}
func (c *fakeMQTTClient) Unsubscribe(topics ...string) pahomqtt.Token {
	c.unsubscribed = append(c.unsubscribed, topics...)
	return newFakeMQTTToken(nil)
}
func (c *fakeMQTTClient) AddRoute(topic string, callback pahomqtt.MessageHandler) {}
func (c *fakeMQTTClient) OptionsReader() pahomqtt.ClientOptionsReader {
	return pahomqtt.ClientOptionsReader{}
}

func withMQTTClientForTest(t *testing.T, c pahomqtt.Client) {
	t.Helper()
	mu.Lock()
	oldClient := client
	client = c
	mu.Unlock()
	t.Cleanup(func() {
		mu.Lock()
		client = oldClient
		mu.Unlock()
	})
}

func mqttTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestMessageBufferLenUsesFullBufferAndPrunesByAge(t *testing.T) {
	b := newMessageBuffer()
	b.Configure(3, 1, 0)

	b.Add(tools.MQTTMessage{Topic: "home/a", Payload: "old", Timestamp: time.Now().UTC().Add(-2 * time.Hour).Format(time.RFC3339)})
	b.Add(tools.MQTTMessage{Topic: "home/a", Payload: "one", Timestamp: time.Now().UTC().Format(time.RFC3339)})
	b.Add(tools.MQTTMessage{Topic: "home/b", Payload: "two", Timestamp: time.Now().UTC().Format(time.RFC3339)})

	if got := b.Len(); got != 2 {
		t.Fatalf("Len() = %d, want 2", got)
	}
	if got := len(b.Get("", 0)); got != 2 {
		t.Fatalf("Get default count = %d, want 2", got)
	}
}

func TestMessageBufferTruncatesOversizedPayloads(t *testing.T) {
	b := newMessageBuffer()
	b.Configure(10, 0, 5)

	msg := b.Add(tools.MQTTMessage{Topic: "home/a", Payload: "abcdef", Timestamp: time.Now().UTC().Format(time.RFC3339)})
	if !msg.PayloadTruncated {
		t.Fatal("expected payload to be marked truncated")
	}
	if msg.Payload != "abcde" {
		t.Fatalf("payload = %q, want abcde", msg.Payload)
	}
	if msg.PayloadBytes != 6 {
		t.Fatalf("payload bytes = %d, want 6", msg.PayloadBytes)
	}
}

func TestValidateMQTTTopics(t *testing.T) {
	if err := validatePublishTopic("home/+/temperature"); err == nil {
		t.Fatal("publish topic with wildcard was accepted")
	}
	if err := validateTopicFilter("home/#/bad"); err == nil {
		t.Fatal("topic filter with misplaced multi-level wildcard was accepted")
	}
	if err := validateTopicFilter("home/+/temperature"); err != nil {
		t.Fatalf("valid topic filter rejected: %v", err)
	}
}

func TestValidateMQTTQoS(t *testing.T) {
	for _, qos := range []int{0, 1, 2} {
		if err := validateQoS(qos); err != nil {
			t.Fatalf("valid qos %d rejected: %v", qos, err)
		}
	}
	for _, qos := range []int{-1, 3} {
		if err := validateQoS(qos); err == nil {
			t.Fatalf("invalid qos %d accepted", qos)
		}
	}
}

func TestMissionTriggerRateLimit(t *testing.T) {
	missionTriggerMu.Lock()
	oldTriggers := missionTriggers
	missionTriggers = nil
	missionTriggerMu.Unlock()
	t.Cleanup(func() {
		missionTriggerMu.Lock()
		missionTriggers = oldTriggers
		missionTriggerMu.Unlock()
	})

	var fired int32
	RegisterMissionTrigger("home/#", "", 60, func(topic, payload string) {
		atomic.AddInt32(&fired, 1)
	})

	for _, trigger := range matchingMissionTriggers("home/sensor", "on") {
		trigger.callback("home/sensor", "on")
	}
	for _, trigger := range matchingMissionTriggers("home/sensor", "on") {
		trigger.callback("home/sensor", "on")
	}

	if got := atomic.LoadInt32(&fired); got != 1 {
		t.Fatalf("fired = %d, want 1", got)
	}
}

func TestRuntimeSubscriptionsReplayAndUnsubscribeRegistry(t *testing.T) {
	firstClient := newFakeMQTTClient()
	withMQTTClientForTest(t, firstClient)

	if err := subscribe("home/runtime", 1, mqttTestLogger()); err != nil {
		t.Fatalf("subscribe runtime topic: %v", err)
	}

	reconnectClient := newFakeMQTTClient()
	cfg := &config.Config{}
	cfg.MQTT.Topics = []string{"home/configured"}
	subscribeConfiguredTopics(reconnectClient, cfg)

	if got := reconnectClient.subscriptions["home/runtime"]; got != 1 {
		t.Fatalf("runtime subscription qos = %d, want 1; subscriptions=%v", got, reconnectClient.subscriptions)
	}
	if got := reconnectClient.subscriptions["home/configured"]; got != 0 {
		t.Fatalf("configured subscription qos = %d, want 0", got)
	}

	if err := unsubscribe("home/runtime", mqttTestLogger()); err != nil {
		t.Fatalf("unsubscribe runtime topic: %v", err)
	}

	afterUnsubscribeClient := newFakeMQTTClient()
	subscribeConfiguredTopics(afterUnsubscribeClient, cfg)
	if _, ok := afterUnsubscribeClient.subscriptions["home/runtime"]; ok {
		t.Fatalf("runtime subscription replayed after unsubscribe: %v", afterUnsubscribeClient.subscriptions)
	}
}

func TestStopClientPublishesOfflineAvailabilityBeforeDisconnect(t *testing.T) {
	cfg := &config.Config{}
	cfg.MQTT.Availability.Enabled = true
	cfg.MQTT.Availability.Topic = "aurago/status"
	cfg.MQTT.Availability.OfflinePayload = "offline"
	cfg.MQTT.Availability.QoS = 1
	cfg.MQTT.Availability.Retain = true
	setActiveConfig(cfg)
	t.Cleanup(func() { setActiveConfig(nil) })

	fakeClient := newFakeMQTTClient()
	withMQTTClientForTest(t, fakeClient)

	StopClient()

	if len(fakeClient.published) == 0 {
		t.Fatal("expected offline availability publish before disconnect")
	}
	publish := fakeClient.published[0]
	if publish.topic != "aurago/status" || publish.payload != "offline" || publish.qos != 1 || !publish.retain {
		t.Fatalf("offline publish = %+v, want aurago/status offline qos=1 retain=true", publish)
	}
	if !fakeClient.disconnected {
		t.Fatal("expected client to be disconnected")
	}
	if len(fakeClient.events) < 2 || fakeClient.events[0] != "publish:aurago/status" || fakeClient.events[1] != "disconnect" {
		t.Fatalf("events = %v, want offline publish before disconnect", fakeClient.events)
	}
}

func TestKeyedMissionTriggerReplacesAndUnregisters(t *testing.T) {
	missionTriggerMu.Lock()
	oldTriggers := missionTriggers
	missionTriggers = nil
	missionTriggerMu.Unlock()
	t.Cleanup(func() {
		missionTriggerMu.Lock()
		missionTriggers = oldTriggers
		missionTriggerMu.Unlock()
	})

	var oldFired int32
	var newFired int32
	RegisterMissionTriggerForKey("mission-1", "home/#", "old", 0, func(topic, payload string) {
		atomic.AddInt32(&oldFired, 1)
	})
	RegisterMissionTriggerForKey("mission-1", "home/#", "new", 0, func(topic, payload string) {
		atomic.AddInt32(&newFired, 1)
	})

	for _, trigger := range matchingMissionTriggers("home/sensor", "old") {
		trigger.callback("home/sensor", "old")
	}
	for _, trigger := range matchingMissionTriggers("home/sensor", "new") {
		trigger.callback("home/sensor", "new")
	}

	if got := atomic.LoadInt32(&oldFired); got != 0 {
		t.Fatalf("old trigger fired %d times, want 0", got)
	}
	if got := atomic.LoadInt32(&newFired); got != 1 {
		t.Fatalf("new trigger fired %d times, want 1", got)
	}

	UnregisterMissionTrigger("mission-1")
	for _, trigger := range matchingMissionTriggers("home/sensor", "new") {
		trigger.callback("home/sensor", "new")
	}
	if got := atomic.LoadInt32(&newFired); got != 1 {
		t.Fatalf("unregistered trigger fired again, count=%d", got)
	}
}
