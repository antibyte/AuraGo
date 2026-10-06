package server

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"aurago/internal/config"
)

func TestMQTTRelayLimiterDebouncesPerTopic(t *testing.T) {
	limiter := newMQTTRelayLimiter(2 * time.Second)
	now := time.Unix(100, 0)

	if !limiter.Allow("home/a", now) {
		t.Fatal("first message for topic should be allowed")
	}
	if limiter.Allow("home/a", now.Add(time.Second)) {
		t.Fatal("second message inside debounce window should be blocked")
	}
	if !limiter.Allow("home/b", now.Add(time.Second)) {
		t.Fatal("different topic should have its own debounce window")
	}
	if !limiter.Allow("home/a", now.Add(2*time.Second)) {
		t.Fatal("message at debounce boundary should be allowed")
	}
	if !limiter.Allow("home/c", now.Add(4*time.Second)) {
		t.Fatal("new topic should be allowed after prune interval")
	}
	limiter.mu.Lock()
	_, staleTopicKept := limiter.lastByTopic["home/b"]
	_, newTopicKept := limiter.lastByTopic["home/c"]
	limiter.mu.Unlock()
	if staleTopicKept {
		t.Fatal("stale topic should be pruned from debounce state")
	}
	if !newTopicKept {
		t.Fatal("new topic should remain in debounce state")
	}
}

func TestMQTTRelayAuthorizedRequiresCredentialsOrExplicitFlag(t *testing.T) {
	if mqttRelayAuthorized(nil) {
		t.Fatal("a missing config must not authorize the relay")
	}
	cfg := &config.Config{}
	cfg.MQTT.Enabled, cfg.MQTT.RelayToAgent = true, true
	cfg.MQTT.Broker = "tcp://broker.lan:1883"
	if mqttRelayAuthorized(cfg) {
		t.Fatal("anonymous relay must be refused by default")
	}
	cfg.MQTT.Username = "  "
	if mqttRelayAuthorized(cfg) {
		t.Fatal("a blank username must not authorize the relay")
	}
	cfg.MQTT.Username = "iot"
	if !mqttRelayAuthorized(cfg) {
		t.Fatal("username must authorize the relay")
	}
	cfg.MQTT.Username = ""
	cfg.MQTT.TLS.CertFile = "client.crt"
	if mqttRelayAuthorized(cfg) {
		t.Fatal("a client certificate on a tcp:// broker is never presented and must not authorize the relay")
	}
	cfg.MQTT.Broker = "mqtts://broker.lan:8883"
	if !mqttRelayAuthorized(cfg) {
		t.Fatal("client certificate over TLS must authorize the relay")
	}
	cfg.MQTT.Broker = "tcp://broker.lan:1883"
	cfg.MQTT.TLS.CertFile = ""
	cfg.MQTT.AllowUnauthenticatedRelay = true
	if !mqttRelayAuthorized(cfg) {
		t.Fatal("explicit flag must authorize the relay")
	}
}

// syncBuffer is a log sink that tolerates the concurrent writes slog may issue.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// refusals counts logged refusals of either path.
func (b *syncBuffer) refusals() int {
	return strings.Count(b.String(), " delivery refused: ")
}

func newMQTTRelayTestLogger() (*slog.Logger, *syncBuffer) {
	sink := &syncBuffer{}
	return slog.New(slog.NewTextHandler(sink, nil)), sink
}

func anonymousMQTTRelayConfig() *config.Config {
	cfg := &config.Config{}
	cfg.MQTT.Enabled = true
	cfg.MQTT.Broker = "tcp://broker.lan:1883"
	cfg.MQTT.RelayToAgent = true
	cfg.Frigate.Enabled = true
	cfg.Frigate.EventRelay = true
	return cfg
}

func TestMQTTRelayRefusalMessagesNamePathAndReason(t *testing.T) {
	for _, tc := range []struct {
		path       string
		haveConfig bool
		want       string
	}{
		{mqttRelayPathRelay, true, "[MQTT] relay delivery refused: broker has no login (username or TLS client certificate) and mqtt.allow_unauthenticated_relay is false"},
		{mqttRelayPathMissionTrigger, true, "[MQTT] mission trigger delivery refused: broker has no login (username or TLS client certificate) and mqtt.allow_unauthenticated_relay is false"},
		{mqttRelayPathRelay, false, "[MQTT] relay delivery refused: no config source"},
		{mqttRelayPathMissionTrigger, false, "[MQTT] mission trigger delivery refused: no config source"},
	} {
		if got := mqttRelayRefusalMessage(tc.path, tc.haveConfig); got != tc.want {
			t.Fatalf("mqttRelayRefusalMessage(%q, %v) = %q, want %q", tc.path, tc.haveConfig, got, tc.want)
		}
	}
}

func TestMQTTRelayRouteRefusesAnonymousBrokerAndLogsOnce(t *testing.T) {
	logger, logs := newMQTTRelayTestLogger()
	gate := &mqttRelayGate{}
	cfg := anonymousMQTTRelayConfig()

	for _, topic := range []string{"home/door", "frigate/events", "home/door", "frigate/events"} {
		if source, _, ok := gate.route(cfg, topic, logger); ok {
			t.Fatalf("anonymous broker relayed %s as %q", topic, source)
		}
	}
	if got := logs.refusals(); got != 1 {
		t.Fatalf("refusal logged %d times, want once", got)
	}
	if want := mqttRelayRefusalMessage(mqttRelayPathRelay, true); !strings.Contains(logs.String(), want) {
		t.Fatalf("relay refusal must log %q:\n%s", want, logs.String())
	}

	cfg.MQTT.Username = "iot"
	if source, kind, ok := gate.route(cfg, "frigate/events", logger); !ok || source != "frigate" || kind != "event" {
		t.Fatalf("authenticated Frigate relay = (%q, %q, %v), want (frigate, event, true)", source, kind, ok)
	}
	if source, kind, ok := gate.route(cfg, "home/door", logger); !ok || source != "mqtt" || kind != "" {
		t.Fatalf("authenticated generic relay = (%q, %q, %v), want (mqtt, \"\", true)", source, kind, ok)
	}

	// A refusal after the gate was open in between is reported again.
	cfg.MQTT.Username = ""
	if _, _, ok := gate.route(cfg, "home/door", logger); ok {
		t.Fatal("relay must be refused again once the credentials are gone")
	}
	if got := logs.refusals(); got != 2 {
		t.Fatalf("refusal logged %d times after reopening, want 2", got)
	}

	cfg.MQTT.AllowUnauthenticatedRelay = true
	if source, _, ok := gate.route(cfg, "home/door", logger); !ok || source != "mqtt" {
		t.Fatalf("explicitly allowed anonymous relay = (%q, %v), want (mqtt, true)", source, ok)
	}
}

func TestMQTTRelayRouteFrigateOnlyAndClientCertificate(t *testing.T) {
	logger, logs := newMQTTRelayTestLogger()
	gate := &mqttRelayGate{}
	cfg := anonymousMQTTRelayConfig()
	cfg.MQTT.RelayToAgent = false
	cfg.Frigate.ReviewRelay = true

	if _, _, ok := gate.route(cfg, "frigate/events", logger); ok {
		t.Fatal("anonymous Frigate-only relay must be refused")
	}
	// A topic no relay takes is not a refusal and must not be logged as one.
	if _, _, ok := gate.route(cfg, "home/door", logger); ok {
		t.Fatal("generic relay is off; home/door must not be relayed")
	}
	if got := logs.refusals(); got != 1 {
		t.Fatalf("refusal logged %d times, want once", got)
	}

	// On tcp:// the client certificate is never presented.
	cfg.MQTT.TLS.CertFile = "client.crt"
	if _, _, ok := gate.route(cfg, "frigate/reviews", logger); ok {
		t.Fatal("Frigate relay with a client certificate over tcp:// must be refused")
	}

	cfg.MQTT.Broker = "ssl://broker.lan:8883"
	if source, kind, ok := gate.route(cfg, "frigate/events", logger); !ok || source != "frigate" || kind != "event" {
		t.Fatalf("Frigate event relay with client certificate over TLS = (%q, %q, %v), want (frigate, event, true)", source, kind, ok)
	}
	if source, kind, ok := gate.route(cfg, "frigate/reviews", logger); !ok || source != "frigate" || kind != "review" {
		t.Fatalf("Frigate review relay with client certificate over TLS = (%q, %q, %v), want (frigate, review, true)", source, kind, ok)
	}

	cfg.MQTT.Enabled = false
	if _, _, ok := gate.route(cfg, "frigate/events", logger); ok {
		t.Fatal("disabled MQTT must not relay")
	}
	if _, _, ok := gate.route(nil, "frigate/events", logger); ok {
		t.Fatal("a missing config must not relay")
	}
}

// The installed relay handler reads the live config on every delivery and
// returns before the debounce limiter (and so before any agent run) when the
// broker is anonymous.
func TestMQTTRelayHandlerRefusesAnonymousBrokerBeforeAgentRun(t *testing.T) {
	logger, logs := newMQTTRelayTestLogger()
	cfg := anonymousMQTTRelayConfig()
	cfg.MQTT.RelayToAgent = false
	cfg.Frigate.Enabled = false
	s := &Server{Cfg: cfg, Logger: logger}
	handler := s.newMQTTRelayHandler()
	topic := "e4-relay-refusal/" + t.Name()

	handler(context.Background(), topic, "on")
	if got := logs.refusals(); got != 0 {
		t.Fatalf("no relay is enabled, yet a refusal was logged %d times", got)
	}

	cfg.MQTT.RelayToAgent = true
	for i := 0; i < 3; i++ {
		handler(context.Background(), topic, "on")
	}
	if got := logs.refusals(); got != 1 {
		t.Fatalf("refusal logged %d times, want once", got)
	}
	defaultMQTTRelayLimiter.mu.Lock()
	_, reachedLimiter := defaultMQTTRelayLimiter.lastByTopic[topic]
	defaultMQTTRelayLimiter.mu.Unlock()
	if reachedLimiter {
		t.Fatal("refused delivery reached the relay limiter and agent dispatch")
	}
}

func TestMissionMQTTAdapterGatesTriggersOnLiveConfig(t *testing.T) {
	logger, logs := newMQTTRelayTestLogger()
	current := anonymousMQTTRelayConfig()
	current.MQTT.RelayToAgent = false
	current.Frigate.Enabled = false
	adapter := &missionMQTTAdapter{logger: logger, config: func() *config.Config { return current }}
	fired := 0
	trigger := adapter.gateMissionTrigger(func(topic, payload string) {
		if topic != "home/door" || payload != "open" {
			t.Fatalf("callback got (%q, %q)", topic, payload)
		}
		fired++
	})

	trigger("home/door", "open")
	trigger("home/door", "open")
	if fired != 0 {
		t.Fatalf("anonymous broker started %d missions", fired)
	}
	if got := logs.refusals(); got != 1 {
		t.Fatalf("refusal logged %d times, want once", got)
	}
	if want := mqttRelayRefusalMessage(mqttRelayPathMissionTrigger, true); !strings.Contains(logs.String(), want) {
		t.Fatalf("mission trigger refusal must log %q:\n%s", want, logs.String())
	}

	withUser := current.Clone()
	withUser.MQTT.Username = "iot"
	current = withUser
	trigger("home/door", "open")
	if fired != 1 {
		t.Fatalf("authenticated broker started %d missions, want 1", fired)
	}

	allowed := current.Clone()
	allowed.MQTT.Username = ""
	allowed.MQTT.AllowUnauthenticatedRelay = true
	current = allowed
	trigger("home/door", "open")
	if fired != 2 {
		t.Fatalf("explicitly allowed anonymous broker started %d missions, want 2", fired)
	}

	unconfiguredLogger, unconfiguredLogs := newMQTTRelayTestLogger()
	unconfigured := &missionMQTTAdapter{logger: unconfiguredLogger}
	unconfigured.gateMissionTrigger(func(string, string) { fired++ })("home/door", "open")
	if fired != 2 {
		t.Fatal("an adapter without a config source must refuse mission triggers")
	}
	if want := mqttRelayRefusalMessage(mqttRelayPathMissionTrigger, false); !strings.Contains(unconfiguredLogs.String(), want) {
		t.Fatalf("refusal without config source must log %q:\n%s", want, unconfiguredLogs.String())
	}
}

// Both registration entry points install the gated callback in the mqtt
// package, never the mission manager's raw callback.
func TestMissionMQTTAdapterRegistersGatedCallbacks(t *testing.T) {
	type registration struct {
		key      string
		callback func(topic, payload string)
	}
	var installed []registration
	logger, logs := newMQTTRelayTestLogger()
	current := anonymousMQTTRelayConfig()
	adapter := &missionMQTTAdapter{
		logger: logger,
		config: func() *config.Config { return current },
		register: func(key, _, _ string, _ int, callback func(topic, payload string)) {
			installed = append(installed, registration{key: key, callback: callback})
		},
	}
	fired := 0
	adapter.RegisterMissionTriggerForKey("mission-1|mqtt_message", "home/#", "", 0, func(string, string) { fired++ })
	adapter.RegisterMissionTrigger("garage/#", "", 0, func(string, string) { fired++ })
	if len(installed) != 2 || installed[0].key != "mission-1|mqtt_message" || installed[1].key != "" {
		t.Fatalf("installed registrations = %+v", installed)
	}

	for _, reg := range installed {
		reg.callback("home/door", "open")
	}
	if fired != 0 {
		t.Fatalf("anonymous broker started %d missions through registered triggers", fired)
	}
	if got := logs.refusals(); got != 1 {
		t.Fatalf("refusal logged %d times, want once", got)
	}

	authorized := current.Clone()
	authorized.MQTT.Broker = "mqtts://broker.lan:8883"
	authorized.MQTT.TLS.CertFile = "client.crt"
	current = authorized
	for _, reg := range installed {
		reg.callback("home/door", "open")
	}
	if fired != 2 {
		t.Fatalf("broker with client certificate over TLS started %d missions, want 2", fired)
	}
}
