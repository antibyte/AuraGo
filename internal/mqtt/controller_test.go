package mqtt

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/tools"
)

type controllerTestMessage struct {
	topic   string
	payload []byte
}

func (m controllerTestMessage) Duplicate() bool { return false }
func (m controllerTestMessage) Qos() byte       { return 0 }
func (m controllerTestMessage) Retained() bool  { return false }
func (m controllerTestMessage) Topic() string   { return m.topic }
func (m controllerTestMessage) MessageID() uint16 {
	return 1
}
func (m controllerTestMessage) Payload() []byte { return m.payload }
func (m controllerTestMessage) Ack()            {}

func TestTestConnectionContextCancelsDelayedCONNACK(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	closed := make(chan struct{})
	accepted := make(chan struct{})
	go func() {
		conn, acceptErr := listener.Accept()
		if acceptErr != nil {
			close(closed)
			return
		}
		close(accepted)
		defer conn.Close()
		_, _ = io.Copy(io.Discard, conn) // hold the socket without sending CONNACK
		close(closed)
	}()

	cfg := &config.Config{}
	cfg.MQTT.Enabled = true
	cfg.MQTT.Broker = "tcp://127.0.0.1:" + strconv.Itoa(listener.Addr().(*net.TCPAddr).Port)
	cfg.MQTT.ConnectTimeout = 30
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	err = TestConnectionContext(ctx, cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("TestConnectionContext error = %v, want context deadline", err)
	}
	select {
	case <-accepted:
	case <-time.After(2 * time.Second):
		t.Fatal("test client never reached loopback broker")
	}
	select {
	case <-closed:
	case <-time.After(2 * time.Second):
		t.Fatal("test client socket remained open after context cancellation")
	}
}

func TestMQTTControllerIgnoresUnrelatedConfigChanges(t *testing.T) {
	controller := NewMQTTController(slog.New(slog.NewTextHandler(io.Discard, nil)))
	defer controller.Stop(context.Background())
	cfg := &config.Config{}
	controller.UpdateConfig(cfg)
	deadline := time.Now().Add(2 * time.Second)
	for controller.Status().DesiredRevision == 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	first := controller.Status().DesiredRevision
	if first == 0 {
		t.Fatal("initial MQTT snapshot was not recorded")
	}
	cfg.LLM.Provider = strings.Repeat("unrelated", 2)
	controller.UpdateConfig(cfg)
	if got := controller.Status().DesiredRevision; got != first {
		t.Fatalf("unrelated config changed MQTT revision from %d to %d", first, got)
	}
}

func TestMQTTControllerStopCancelsRelayHandlerBeforeJoining(t *testing.T) {
	controller := &MQTTController{state: "connected"}
	ctx, cancel := context.WithCancel(context.Background())
	gen := &mqttGeneration{
		ctx:    ctx,
		cancel: cancel,
		queue:  make(chan tools.MQTTMessage, 1),
		done:   make(chan struct{}),
	}
	controller.current = gen
	started := make(chan struct{})
	finished := make(chan struct{})
	controller.SetRelayHandler(func(handlerCtx context.Context, _, _ string) {
		close(started)
		<-handlerCtx.Done()
		close(finished)
	})
	gen.group.Add(1)
	go controller.relayLoop(gen)
	gen.queue <- tools.MQTTMessage{Topic: "test/topic", Payload: "payload"}
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("relay handler did not start")
	}
	stopCtx, stopCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer stopCancel()
	controller.stopCurrentGeneration(stopCtx)
	select {
	case <-finished:
	case <-time.After(2 * time.Second):
		t.Fatal("relay handler did not observe cancellation")
	}
}

func TestMQTTControllerStopClosesRuntimeDelayedCONNACK(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	accepted := make(chan struct{})
	closed := make(chan struct{})
	go func() {
		conn, acceptErr := listener.Accept()
		if acceptErr != nil {
			close(closed)
			return
		}
		close(accepted)
		defer close(closed)
		defer conn.Close()
		_, _ = io.Copy(io.Discard, conn) // deliberately never send CONNACK
	}()

	controller := NewMQTTController(slog.New(slog.NewTextHandler(io.Discard, nil)))
	cfg := &config.Config{}
	cfg.MQTT.Enabled = true
	cfg.MQTT.Broker = "tcp://" + listener.Addr().String()
	cfg.MQTT.ConnectTimeout = 30
	controller.UpdateConfig(cfg)
	select {
	case <-accepted:
	case <-time.After(2 * time.Second):
		controller.Stop(context.Background())
		t.Fatal("runtime did not reach loopback broker")
	}
	stopCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := controller.Stop(stopCtx); err != nil {
		t.Fatalf("controller Stop: %v", err)
	}
	select {
	case <-closed:
	case <-time.After(2 * time.Second):
		t.Fatal("runtime socket remained open after Stop")
	}
}

func TestMQTTControllerInitialStartConfiguresBufferLimits(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	go func() {
		conn, acceptErr := listener.Accept()
		if acceptErr != nil {
			return
		}
		defer conn.Close()
		_, _ = io.Copy(io.Discard, conn) // hold the connection without CONNACK
	}()

	buffer.Configure(0, 0, 0)
	t.Cleanup(func() { buffer.Configure(0, 0, 0) })
	controller := NewMQTTController(mqttTestLogger())
	cfg := &config.Config{}
	cfg.MQTT.Enabled = true
	cfg.MQTT.Broker = "tcp://" + listener.Addr().String()
	cfg.MQTT.ConnectTimeout = 30
	cfg.MQTT.Buffer.MaxMessages = 7
	cfg.MQTT.Buffer.MaxAgeHours = 1
	cfg.MQTT.Buffer.MaxPayloadBytes = 1024
	controller.UpdateConfig(cfg)

	deadline := time.Now().Add(2 * time.Second)
	for buffer.currentMaxPayloadBytes() != 1024 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if got := buffer.currentMaxPayloadBytes(); got != 1024 {
		t.Fatalf("initial buffer payload limit = %d, want 1024", got)
	}
	stopCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := controller.Stop(stopCtx); err != nil {
		t.Fatalf("controller Stop: %v", err)
	}
}

func TestMQTTControllerStopPublishesOfflineBeforeTransportCancel(t *testing.T) {
	cfg := &config.Config{}
	cfg.MQTT.Enabled = true
	cfg.MQTT.Availability.Enabled = true
	cfg.MQTT.Availability.Topic = "aurago/status"
	cfg.MQTT.Availability.OfflinePayload = "offline"
	cfg.MQTT.Availability.QoS = 1
	cfg.MQTT.Availability.Retain = true

	workCtx, cancelWork := context.WithCancel(context.Background())
	transportCtx, cancelTransport := context.WithCancel(context.Background())
	fakeClient := newFakeMQTTClient()
	gen := &mqttGeneration{
		ctx:             workCtx,
		cancel:          cancelWork,
		transportCtx:    transportCtx,
		transportCancel: cancelTransport,
		client:          fakeClient,
		done:            make(chan struct{}),
	}
	controller := &MQTTController{
		state:   "connected",
		current: gen,
		active:  cloneMQTTSnapshot(cfg),
	}
	controller.stopCurrentGeneration(context.Background())
	if len(fakeClient.events) < 2 || fakeClient.events[0] != "publish:aurago/status" || fakeClient.events[1] != "disconnect" {
		t.Fatalf("events = %v, want offline publish before disconnect", fakeClient.events)
	}
	if err := transportCtx.Err(); !errors.Is(err, context.Canceled) {
		t.Fatalf("transport context error = %v, want context canceled", err)
	}
	if err := workCtx.Err(); !errors.Is(err, context.Canceled) {
		t.Fatalf("work context error = %v, want context canceled", err)
	}
}

func TestMQTTControllerRelayHandlerDoesNotBlockIncomingCallbacks(t *testing.T) {
	controller := &MQTTController{state: "connected"}
	workCtx, cancelWork := context.WithCancel(context.Background())
	gen := &mqttGeneration{
		ctx:            workCtx,
		cancel:         cancelWork,
		queue:          make(chan tools.MQTTMessage, 1),
		done:           make(chan struct{}),
		desiredFilters: map[string]byte{"#": 0},
	}
	cfg := &config.Config{}
	cfg.MQTT.Enabled = true
	cfg.MQTT.RelayToAgent = true
	cfg.MQTT.Topics = []string{"#"}
	controller.current = gen
	controller.active = cloneMQTTSnapshot(cfg)
	started := make(chan struct{})
	controller.SetRelayHandler(func(ctx context.Context, _, _ string) {
		close(started)
		<-ctx.Done()
	})
	gen.group.Add(1)
	go controller.relayLoop(gen)
	gen.queue <- tools.MQTTMessage{Topic: "first", Payload: "payload"}
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("relay handler did not start")
	}

	handler := controller.messageHandler(gen)
	call := func(topic string) {
		done := make(chan struct{})
		go func() {
			handler(nil, controllerTestMessage{topic: topic, payload: []byte("payload")})
			close(done)
		}()
		select {
		case <-done:
		case <-time.After(500 * time.Millisecond):
			t.Fatalf("incoming callback blocked while relay handler was running for %q", topic)
		}
	}
	call("second")
	call("third")
	if got := len(gen.queue); got != 1 {
		t.Fatalf("relay queue length = %d, want bounded length 1 after drop", got)
	}

	stopCtx, stopCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer stopCancel()
	controller.stopCurrentGeneration(stopCtx)
}
