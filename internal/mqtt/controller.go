package mqtt

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"reflect"
	"sync"
	"sync/atomic"
	"time"

	"aurago/internal/config"
	"aurago/internal/tools"

	pahomqtt "github.com/eclipse/paho.mqtt.golang"
)

// MQTTStatus is an immutable view of the controller lifecycle. DesiredRevision
// advances as soon as UpdateConfig receives a snapshot; ActiveRevision catches
// up after the background worker applies it.
type MQTTStatus struct {
	State            string `json:"state"`
	Connected        bool   `json:"connected"`
	DesiredRevision  uint64 `json:"desired_revision"`
	ActiveRevision   uint64 `json:"active_revision"`
	DesiredEnabled   bool   `json:"desired_enabled"`
	ActiveEnabled    bool   `json:"active_enabled"`
	Generation       uint64 `json:"generation"`
	LastError        string `json:"last_error,omitempty"`
	DesiredBroker    string `json:"desired_broker,omitempty"`
	ActiveBroker     string `json:"active_broker,omitempty"`
	DesiredClientID  string `json:"desired_client_id,omitempty"`
	ActiveClientID   string `json:"active_client_id,omitempty"`
	DesiredTLS       bool   `json:"desired_tls"`
	ActiveTLS        bool   `json:"active_tls"`
	DesiredTransport string `json:"desired_transport,omitempty"`
	ActiveTransport  string `json:"active_transport,omitempty"`
	EffectiveTLS     bool   `json:"effective_tls"`
}

type mqttSnapshot struct {
	cfg     config.Config
	dataDir string
}

func cloneMQTTSnapshot(src *config.Config) *mqttSnapshot {
	if src == nil {
		return nil
	}
	copyCfg := *src
	copyCfg.MQTT = src.MQTT
	copyCfg.MQTT.Topics = append([]string(nil), src.MQTT.Topics...)
	if src.MQTT.CleanSession != nil {
		clean := *src.MQTT.CleanSession
		copyCfg.MQTT.CleanSession = &clean
	}
	// The Frigate fields used by subscriptions and relay classification are
	// scalar values. No caller-owned mutable value remains in this snapshot.
	return &mqttSnapshot{cfg: copyCfg, dataDir: src.Directories.DataDir}
}

func (s *mqttSnapshot) config() *config.Config {
	if s == nil {
		return nil
	}
	copyCfg := s.cfg
	copyCfg.MQTT.Topics = append([]string(nil), s.cfg.MQTT.Topics...)
	if s.cfg.MQTT.CleanSession != nil {
		clean := *s.cfg.MQTT.CleanSession
		copyCfg.MQTT.CleanSession = &clean
	}
	return &copyCfg
}

func mqttSnapshotsRequireReconnect(a, b *mqttSnapshot) bool {
	if a == nil || b == nil {
		return a != b
	}
	left, right := a.cfg.MQTT, b.cfg.MQTT
	if left.Enabled != right.Enabled || left.Broker != right.Broker || left.ClientID != right.ClientID ||
		left.Username != right.Username || left.Password != right.Password || left.ConnectTimeout != right.ConnectTimeout ||
		left.TLS != right.TLS || left.Availability != right.Availability || mqttCleanSession(&a.cfg) != mqttCleanSession(&b.cfg) {
		return true
	}
	return false
}

func mqttSnapshotsEquivalent(a, b *mqttSnapshot) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.dataDir == b.dataDir &&
		reflect.DeepEqual(a.cfg.MQTT, b.cfg.MQTT) &&
		a.cfg.Frigate.Enabled == b.cfg.Frigate.Enabled &&
		a.cfg.Frigate.EventRelay == b.cfg.Frigate.EventRelay &&
		a.cfg.Frigate.ReviewRelay == b.cfg.Frigate.ReviewRelay &&
		a.cfg.Frigate.MQTTTopicPrefix == b.cfg.Frigate.MQTTTopicPrefix
}

type mqttGeneration struct {
	id              uint64
	ctx             context.Context
	cancel          context.CancelFunc
	transportCtx    context.Context
	transportCancel context.CancelFunc
	client          pahomqtt.Client
	queue           chan tools.MQTTMessage
	done            chan struct{}
	group           sync.WaitGroup
	closed          atomic.Bool
	callbackMu      sync.Mutex
}

// MQTTController owns one server MQTT runtime. Configuration updates are
// cloned and reconciled by a worker, so callers never perform network work
// while holding a configuration lock.
type MQTTController struct {
	mu sync.RWMutex

	log *slog.Logger

	desired         *mqttSnapshot
	desiredRevision uint64
	active          *mqttSnapshot
	activeRevision  uint64
	generation      uint64
	current         *mqttGeneration
	state           string
	lastError       string

	relayHandler func(context.Context, string, string)

	reconcileCh  chan struct{}
	workerDone   chan struct{}
	stopped      bool
	legacyBridge bool
}

const defaultControllerRelayQueueSize = 100

// NewMQTTController creates a server-owned MQTT runtime controller.
func NewMQTTController(log *slog.Logger) *MQTTController {
	c := &MQTTController{log: log, state: "disabled"}
	c.startWorkerLocked()
	return c
}

func (c *MQTTController) startWorkerLocked() {
	if c.reconcileCh != nil && c.workerDone != nil {
		return
	}
	c.reconcileCh = make(chan struct{}, 1)
	c.workerDone = make(chan struct{})
	c.stopped = false
	go c.reconcileLoop(c.reconcileCh, c.workerDone)
}

func (c *MQTTController) signalReconcile() {
	c.mu.RLock()
	ch := c.reconcileCh
	c.mu.RUnlock()
	if ch == nil {
		return
	}
	select {
	case ch <- struct{}{}:
	default:
	}
}

// UpdateConfig records a cloned desired snapshot and returns immediately. A
// burst of updates is coalesced by the single reconciliation worker.
func (c *MQTTController) UpdateConfig(cfg *config.Config) {
	if c == nil {
		return
	}
	desired := cloneMQTTSnapshot(cfg)
	c.mu.Lock()
	if c.stopped {
		c.mu.Unlock()
		return
	}
	if mqttSnapshotsEquivalent(c.desired, desired) {
		c.mu.Unlock()
		return
	}
	c.desired = desired
	c.desiredRevision++
	c.mu.Unlock()
	c.signalReconcile()
}

// Status returns a point-in-time lifecycle snapshot. Connected is derived from
// Paho's IsConnectionOpen, never from reconnect intent or IsConnected.
func (c *MQTTController) Status() MQTTStatus {
	if c == nil {
		return MQTTStatus{State: "disabled"}
	}
	c.mu.RLock()
	status := MQTTStatus{
		State:           c.state,
		DesiredRevision: c.desiredRevision,
		ActiveRevision:  c.activeRevision,
		DesiredEnabled:  c.desired != nil && c.desired.cfg.MQTT.Enabled && c.desired.cfg.MQTT.Broker != "",
		ActiveEnabled:   c.active != nil && c.active.cfg.MQTT.Enabled && c.active.cfg.MQTT.Broker != "",
		Generation:      c.generation,
		LastError:       c.lastError,
	}
	if c.desired != nil {
		status.DesiredBroker = c.desired.cfg.MQTT.Broker
		status.DesiredClientID = c.desired.cfg.MQTT.ClientID
		status.DesiredTLS = mqttEffectiveTLS(&c.desired.cfg)
		status.DesiredTransport = mqttTransportScheme(c.desired.cfg.MQTT.Broker)
	}
	if c.active != nil {
		status.ActiveBroker = c.active.cfg.MQTT.Broker
		status.ActiveClientID = c.active.cfg.MQTT.ClientID
		status.ActiveTLS = mqttEffectiveTLS(&c.active.cfg)
		status.ActiveTransport = mqttTransportScheme(c.active.cfg.MQTT.Broker)
	}
	gen := c.current
	if gen != nil && gen.client != nil {
		status.Connected = gen.client.IsConnectionOpen()
		if status.Connected {
			status.State = "connected"
		}
	}
	if status.Connected && c.active != nil {
		status.EffectiveTLS = mqttEffectiveTLS(&c.active.cfg)
	}
	c.mu.RUnlock()
	return status
}

func mqttEffectiveTLS(cfg *config.Config) bool {
	effective, err := config.MQTTEffectiveTLS(cfg)
	return err == nil && effective
}

func mqttTransportScheme(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return parsed.Scheme
}

// SetRelayHandler installs the context-aware relay sink for this controller.
// A nil handler disables relay delivery. The handler is read under the same
// mutex as lifecycle state, so updates cannot race callback generation setup.
func (c *MQTTController) SetRelayHandler(handler func(context.Context, string, string)) {
	if c == nil {
		return
	}
	c.mu.Lock()
	c.relayHandler = handler
	c.mu.Unlock()
}

// Stop cancels callbacks, closes the active Paho connection and joins the
// controller workers before returning.
func (c *MQTTController) Stop(ctx context.Context) error {
	if c == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	c.mu.Lock()
	if c.stopped {
		done := c.workerDone
		legacyBridge := c.legacyBridge
		c.mu.Unlock()
		if legacyBridge {
			stopLegacyInjectedClient()
		}
		if done == nil {
			return nil
		}
		select {
		case <-done:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	c.stopped = true
	c.desired = nil
	c.desiredRevision++
	ch, done := c.reconcileCh, c.workerDone
	c.mu.Unlock()
	if ch != nil {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
	select {
	case <-done:
		c.mu.Lock()
		if c.workerDone == done {
			c.reconcileCh = nil
			c.workerDone = nil
		}
		c.mu.Unlock()
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (c *MQTTController) reconcileLoop(ch <-chan struct{}, done chan<- struct{}) {
	defer close(done)
	for range ch {
		c.reconcile()
		c.mu.RLock()
		stopped := c.stopped
		c.mu.RUnlock()
		if stopped {
			return
		}
	}
}

func (c *MQTTController) reconcile() {
	c.mu.RLock()
	desired, revision, active, gen := c.desired, c.desiredRevision, c.active, c.current
	c.mu.RUnlock()
	if desired == nil || !desired.cfg.MQTT.Enabled || desired.cfg.MQTT.Broker == "" {
		c.stopCurrentGeneration(context.Background())
		c.mu.Lock()
		c.active = nil
		c.activeRevision = revision
		c.state = "disabled"
		c.mu.Unlock()
		return
	}
	if gen == nil || mqttSnapshotsRequireReconnect(active, desired) {
		c.stopCurrentGeneration(context.Background())
		c.mu.RLock()
		latest, latestRevision := c.desired, c.desiredRevision
		c.mu.RUnlock()
		if latest != desired || latestRevision != revision {
			c.signalReconcile()
			return
		}
		c.startGeneration(desired, revision)
		return
	}
	// Logical changes retain the transport and are applied through the current
	// callback generation. The next reconnect will use this cloned snapshot too.
	c.mu.Lock()
	c.active = desired
	c.activeRevision = revision
	current := c.current
	c.mu.Unlock()
	buffer.Configure(desired.cfg.MQTT.Buffer.MaxMessages, desired.cfg.MQTT.Buffer.MaxAgeHours, desired.cfg.MQTT.Buffer.MaxPayloadBytes)
	if current != nil && current.client != nil && current.client.IsConnectionOpen() {
		subscribeConfiguredTopicsWithHandler(current.client, desired.config(), c.messageHandler(current))
	}
}

func (c *MQTTController) startGeneration(snapshot *mqttSnapshot, revision uint64) {
	cfg := snapshot.config()
	log := c.log
	if log == nil {
		log = slog.Default()
	}
	// Buffer limits belong to the active generation even before the first
	// callback/reconnect. Configure them before opening the transport so an
	// immediately delivered message observes the requested limits.
	buffer.Configure(cfg.MQTT.Buffer.MaxMessages, cfg.MQTT.Buffer.MaxAgeHours, cfg.MQTT.Buffer.MaxPayloadBytes)
	transportCtx, cancelTransport := context.WithCancel(context.Background())
	opts, err := newClientOptionsContext(transportCtx, cfg, log)
	if err != nil {
		cancelTransport()
		recordError(err)
		c.mu.Lock()
		c.active = snapshot
		c.activeRevision = revision
		c.state = "error"
		c.lastError = err.Error()
		c.mu.Unlock()
		return
	}
	genCtx, cancel := context.WithCancel(context.Background())

	c.mu.Lock()
	c.generation++
	id := c.generation
	gen := &mqttGeneration{
		id:              id,
		ctx:             genCtx,
		cancel:          cancel,
		transportCtx:    transportCtx,
		transportCancel: cancelTransport,
		done:            make(chan struct{}),
		queue:           make(chan tools.MQTTMessage, defaultControllerRelayQueueSize),
	}

	opts.SetOnConnectHandler(func(pahoClient pahomqtt.Client) {
		if !c.enterGenerationCallback(gen) {
			return
		}
		defer gen.callbackMu.Unlock()
		c.mu.RLock()
		currentSnapshot := c.active
		c.mu.RUnlock()
		if currentSnapshot == nil || !c.generationCurrent(gen) {
			return
		}
		if log != nil {
			log.Info("[MQTT] Connected to broker")
		}
		recordConnected()
		publishAvailability(pahoClient, currentSnapshot.config(), log)
		subscribeConfiguredTopicsWithHandler(pahoClient, currentSnapshot.config(), c.messageHandler(gen))
		c.mu.Lock()
		if c.generationCurrentLocked(gen) {
			c.state = "connected"
		}
		c.mu.Unlock()
	})
	opts.SetConnectionLostHandler(func(pahoClient pahomqtt.Client, lostErr error) {
		if !c.enterGenerationCallback(gen) {
			return
		}
		defer gen.callbackMu.Unlock()
		recordDisconnected(lostErr)
		c.mu.Lock()
		if c.generationCurrentLocked(gen) {
			c.state = "reconnecting"
			if lostErr != nil {
				c.lastError = lostErr.Error()
			}
		}
		c.mu.Unlock()
		if log != nil {
			log.Warn("[MQTT] Connection lost", "error", lostErr)
		}
	})
	opts.SetReconnectingHandler(func(_ pahomqtt.Client, _ *pahomqtt.ClientOptions) {
		if !c.enterGenerationCallback(gen) {
			return
		}
		defer gen.callbackMu.Unlock()
		c.mu.Lock()
		if c.generationCurrentLocked(gen) {
			c.state = "reconnecting"
		}
		c.mu.Unlock()
	})
	clientRef := pahomqtt.NewClient(opts)
	gen.client = clientRef
	c.current = gen
	c.active = snapshot
	c.activeRevision = revision
	c.state = "connecting"
	c.lastError = ""
	c.mu.Unlock()

	gen.group.Add(2)
	token := clientRef.Connect()
	go c.monitorConnection(gen, clientRef, token, mqttConnectTimeout(cfg), log)
	go c.relayLoop(gen)
}

func (c *MQTTController) monitorConnection(gen *mqttGeneration, clientRef pahomqtt.Client, token pahomqtt.Token, timeout time.Duration, log *slog.Logger) {
	defer gen.group.Done()
	select {
	case <-token.Done():
		if err := token.Error(); err != nil && c.generationCurrent(gen) {
			recordError(err)
			c.mu.Lock()
			if c.generationCurrentLocked(gen) {
				c.state = "error"
				c.lastError = err.Error()
			}
			c.mu.Unlock()
			if log != nil {
				log.Warn("[MQTT] Failed to connect", "error", err)
			}
		}
	case <-time.After(timeout):
		err := fmt.Errorf("MQTT connect timed out after %s", timeout)
		if c.generationCurrent(gen) {
			recordError(err)
			c.mu.Lock()
			if c.generationCurrentLocked(gen) {
				c.state = "reconnecting"
				c.lastError = err.Error()
			}
			c.mu.Unlock()
			if log != nil {
				log.Warn("[MQTT] Connect timed out, will retry in background")
			}
		}
	case <-gen.transportCtx.Done():
		// Disconnect below also cancels Paho's delayed CONNACK/reconnect loop.
	}
}

func (c *MQTTController) generationCurrent(gen *mqttGeneration) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.generationCurrentLocked(gen)
}

func (c *MQTTController) enterGenerationCallback(gen *mqttGeneration) bool {
	if gen == nil {
		return false
	}
	gen.callbackMu.Lock()
	if gen.closed.Load() {
		gen.callbackMu.Unlock()
		return false
	}
	c.mu.RLock()
	current := c.generationCurrentLocked(gen)
	c.mu.RUnlock()
	if !current {
		gen.callbackMu.Unlock()
		return false
	}
	return true
}

func (c *MQTTController) generationCurrentLocked(gen *mqttGeneration) bool {
	return gen != nil && c.current == gen && !gen.closed.Load() && !c.stopped && gen.ctx.Err() == nil
}

func (c *MQTTController) stopCurrentGeneration(ctx context.Context) {
	c.mu.RLock()
	gen := c.current
	c.mu.RUnlock()
	if gen == nil {
		// Preserve the historical test seam where package tests inject a fake
		// client into the legacy global.
		if c.legacyBridge {
			stopLegacyInjectedClient()
		}
		return
	}
	// Invalidate and cancel first so an in-flight relay handler can observe its
	// canceled context. Then serialize callback completion; acquiring the lock
	// before cancellation would deadlock on a handler waiting for ctx.Done().
	gen.closed.Store(true)
	gen.cancel()
	c.mu.Lock()
	active := c.active
	if c.current != gen {
		c.mu.Unlock()
		return
	}
	c.current = nil
	c.state = "stopping"
	c.mu.Unlock()

	// Drain callbacks while the transport is still usable. The work context is
	// already canceled, so a relay handler waiting on it can return and no new
	// generation callback can enter the effect section.
	gen.callbackMu.Lock()
	gen.callbackMu.Unlock()
	if gen.client != nil {
		if gen.client.IsConnectionOpen() {
			if active != nil {
				publishOfflineAvailability(gen.client, mqttAvailabilitySnapshotFromConfig(active.config()), c.log)
			}
		}
	}
	if gen.transportCancel != nil {
		gen.transportCancel()
	}
	if gen.client != nil {
		gen.client.Disconnect(1000)
	}
	go func() {
		gen.group.Wait()
		close(gen.done)
	}()
	if ctx == nil {
		ctx = context.Background()
	}
	select {
	case <-gen.done:
	case <-ctx.Done():
		return
	}
}

func (c *MQTTController) relayLoop(gen *mqttGeneration) {
	defer gen.group.Done()
	for {
		select {
		case <-gen.ctx.Done():
			return
		case msg := <-gen.queue:
			if !c.enterGenerationCallback(gen) {
				continue
			}
			c.mu.RLock()
			handler := c.relayHandler
			c.mu.RUnlock()
			legacyHandler := RelayCallback
			// callbackMu only guards generation effects and handler capture. Keep
			// the lock free while user code runs so incoming MQTT callbacks can
			// continue to enqueue and bounded queues can apply backpressure.
			gen.callbackMu.Unlock()
			if gen.ctx.Err() != nil || gen.closed.Load() {
				continue
			}
			if handler != nil {
				handler(gen.ctx, msg.Topic, msg.Payload)
			} else if legacyHandler != nil {
				legacyHandler(msg.Topic, msg.Payload)
			}
		}
	}
}

func (c *MQTTController) messageHandler(gen *mqttGeneration) pahomqtt.MessageHandler {
	return func(_ pahomqtt.Client, msg pahomqtt.Message) {
		if !c.enterGenerationCallback(gen) {
			return
		}
		defer gen.callbackMu.Unlock()
		m := makeMQTTMessage(msg)
		m = storeMQTTMessage(m)
		c.mu.RLock()
		active := c.active
		c.mu.RUnlock()
		if active != nil && (active.cfg.MQTT.RelayToAgent || FrigateRelayEnabled(&active.cfg)) {
			c.enqueueRelay(gen, m)
		}
		for _, trigger := range matchingMissionTriggers(m.Topic, m.Payload) {
			go c.runMissionTrigger(gen, trigger, m.Topic, m.Payload)
		}
	}
}

func (c *MQTTController) runMissionTrigger(gen *mqttGeneration, trigger missionTriggerEntry, topic, payload string) {
	if !c.enterGenerationCallback(gen) {
		return
	}
	defer gen.callbackMu.Unlock()
	trigger.callback(topic, payload)
}

func (c *MQTTController) enqueueRelay(gen *mqttGeneration, msg tools.MQTTMessage) {
	if !c.generationCurrent(gen) {
		return
	}
	select {
	case gen.queue <- msg:
	default:
		RecordDroppedRelayMessage()
	}
}

func (c *MQTTController) activeClient() pahomqtt.Client {
	c.mu.RLock()
	gen := c.current
	c.mu.RUnlock()
	if gen != nil && c.generationCurrent(gen) {
		return gen.client
	}
	if c == defaultControllerInstance() {
		mu.RLock()
		legacy := client
		mu.RUnlock()
		return legacy
	}
	return nil
}

func (c *MQTTController) currentConfig() *config.Config {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.active == nil {
		return nil
	}
	return c.active.config()
}

func (c *MQTTController) publish(topic, payload string, qos int, retain bool, log *slog.Logger) error {
	clientRef := c.activeClient()
	if clientRef == nil || !clientRef.IsConnectionOpen() {
		return fmt.Errorf("MQTT client is not connected")
	}
	if err := validateQoS(qos); err != nil {
		atomic.AddUint64(&stats.publishErrors, 1)
		return err
	}
	if err := validatePublishTopic(topic); err != nil {
		atomic.AddUint64(&stats.publishErrors, 1)
		return err
	}
	if maxPayloadBytes := buffer.currentMaxPayloadBytes(); maxPayloadBytes > 0 && len([]byte(payload)) > maxPayloadBytes {
		atomic.AddUint64(&stats.publishErrors, 1)
		return fmt.Errorf("MQTT payload exceeds %d byte limit", maxPayloadBytes)
	}
	token := clientRef.Publish(topic, byte(qos), retain, payload)
	if !token.WaitTimeout(10 * time.Second) {
		atomic.AddUint64(&stats.publishErrors, 1)
		return fmt.Errorf("MQTT publish timed out")
	}
	if err := token.Error(); err != nil {
		atomic.AddUint64(&stats.publishErrors, 1)
		return fmt.Errorf("MQTT publish failed: %w", err)
	}
	atomic.AddUint64(&stats.publishedMessages, 1)
	if log != nil {
		log.Info("[MQTT] Published", "topic", topic, "retain", retain, "payload_len", len(payload))
	}
	return nil
}

func (c *MQTTController) subscribe(topic string, qos int, log *slog.Logger) error {
	clientRef := c.activeClient()
	if clientRef == nil || !clientRef.IsConnectionOpen() {
		return fmt.Errorf("MQTT client is not connected")
	}
	if err := validateQoS(qos); err != nil {
		atomic.AddUint64(&stats.subscribeErrors, 1)
		return err
	}
	if err := validateTopicFilter(topic); err != nil {
		atomic.AddUint64(&stats.subscribeErrors, 1)
		return err
	}
	token := clientRef.Subscribe(topic, byte(qos), c.messageHandlerForCurrent())
	if err := waitAndValidateSubscribe(token, map[string]byte{topic: byte(qos)}, 10*time.Second); err != nil {
		atomic.AddUint64(&stats.subscribeErrors, 1)
		return err
	}
	rememberRuntimeSubscription(topic, byte(qos))
	if log != nil {
		log.Info("[MQTT] Subscribed", "topic", topic, "qos", qos)
	}
	return nil
}

func (c *MQTTController) messageHandlerForCurrent() pahomqtt.MessageHandler {
	c.mu.RLock()
	gen := c.current
	c.mu.RUnlock()
	if gen == nil {
		return messageHandler
	}
	return c.messageHandler(gen)
}

func (c *MQTTController) unsubscribe(topic string, log *slog.Logger) error {
	clientRef := c.activeClient()
	if clientRef == nil || !clientRef.IsConnectionOpen() {
		return fmt.Errorf("MQTT client is not connected")
	}
	if err := validateTopicFilter(topic); err != nil {
		atomic.AddUint64(&stats.subscribeErrors, 1)
		return err
	}
	token := clientRef.Unsubscribe(topic)
	if !token.WaitTimeout(10 * time.Second) {
		atomic.AddUint64(&stats.subscribeErrors, 1)
		return fmt.Errorf("MQTT unsubscribe timed out")
	}
	if err := token.Error(); err != nil {
		atomic.AddUint64(&stats.subscribeErrors, 1)
		return fmt.Errorf("MQTT unsubscribe failed: %w", err)
	}
	forgetRuntimeSubscription(topic)
	if log != nil {
		log.Info("[MQTT] Unsubscribed", "topic", topic)
	}
	return nil
}

func waitAndValidateSubscribe(token pahomqtt.Token, expected map[string]byte, timeout time.Duration) error {
	if token == nil || !token.WaitTimeout(timeout) {
		return fmt.Errorf("MQTT subscribe timed out")
	}
	if err := token.Error(); err != nil {
		return fmt.Errorf("MQTT subscribe failed: %w", err)
	}
	resultToken, ok := token.(interface{ Result() map[string]byte })
	if !ok {
		return fmt.Errorf("MQTT subscribe did not return SUBACK results")
	}
	result := resultToken.Result()
	for topic := range expected {
		code, ok := result[topic]
		if !ok {
			return fmt.Errorf("MQTT SUBACK omitted topic %q", topic)
		}
		if code > 2 {
			return fmt.Errorf("MQTT subscription rejected for topic %q (return code 0x%02x)", topic, code)
		}
	}
	return nil
}

var (
	defaultControllerMu sync.RWMutex
	defaultController   *MQTTController
)

func newDefaultController() *MQTTController {
	c := NewMQTTController(nil)
	c.legacyBridge = true
	tools.RegisterMQTTBridge(publish, subscribe, unsubscribe, getMessages)
	return c
}

func defaultControllerInstance() *MQTTController {
	defaultControllerMu.Lock()
	if defaultController == nil {
		defaultController = newDefaultController()
	}
	c := defaultController
	defaultControllerMu.Unlock()
	return c
}

// SetDefaultController makes the given server-owned controller the target of
// legacy package-level APIs and bridge callbacks.
func SetDefaultController(c *MQTTController) {
	if c == nil {
		c = NewMQTTController(nil)
	}
	defaultControllerMu.Lock()
	defaultController = c
	defaultControllerMu.Unlock()
	tools.RegisterMQTTBridge(publish, subscribe, unsubscribe, getMessages)
}

// SetRelayHandler configures the default controller's context-aware relay.
func SetRelayHandler(handler func(context.Context, string, string)) {
	defaultControllerInstance().SetRelayHandler(handler)
}

func stopLegacyInjectedClient() {
	mu.Lock()
	legacy := client
	client = nil
	mu.Unlock()
	if legacy == nil {
		setActiveConfig(nil)
		return
	}
	if legacy.IsConnectionOpen() {
		publishOfflineAvailability(legacy, currentAvailabilitySnapshot(), logger)
	}
	legacy.Disconnect(1000)
	recordDisconnected(nil)
	setActiveConfig(nil)
	stopRelayWorker()
}
