package config

import (
	"encoding/json"
	"testing"
)

func TestMQTTAgentRunsRequireIndependentAuthenticationAndTransport(t *testing.T) {
	for _, secure := range []bool{false, true} {
		for _, login := range []bool{false, true} {
			for _, anonymous := range []bool{false, true} {
				for _, untrusted := range []bool{false, true} {
					cfg := &Config{}
					cfg.MQTT.Enabled = true
					cfg.MQTT.Broker = "mqtt://broker.invalid:1883"
					if secure {
						cfg.MQTT.Broker = "mqtts://broker.invalid:8883"
					}
					cfg.MQTT.Username = "operator"
					if login {
						cfg.MQTT.Password = "fixture-password"
					}
					cfg.MQTT.AllowUnauthenticatedRelay = anonymous
					cfg.MQTT.AllowUntrustedTransport = untrusted
					want := (login || anonymous) && (secure || untrusted)
					if got := MQTTAgentRunsAuthorized(cfg); got != want {
						t.Fatalf("secure=%v login=%v anonymous=%v untrusted=%v: authorized=%v want=%v", secure, login, anonymous, untrusted, got, want)
					}
				}
			}
		}
	}
	var cfg Config
	cfg.MQTT.Enabled, cfg.MQTT.AllowUnauthenticatedRelay, cfg.MQTT.AllowUntrustedTransport = true, true, true
	cfg.EggMode.Enabled = true
	if MQTTAgentRunsAuthorized(&cfg) {
		t.Fatal("egg mode must refuse MQTT agent runs")
	}
}

func TestMQTTRelayGrantsRejectBroadAndUnresolvedActions(t *testing.T) {
	valid := []ActionPolicyRule{
		{Tool: "mqtt_publish", Operation: "publish", Target: "mqtt:home/light/set"},
		{Tool: "mqtt_subscribe", Operation: "subscribe", Target: "mqtt_filter:home/+/state"},
		{Tool: "mqtt_get_messages", Operation: "read", Target: "tool:mqtt_get_messages"},
		{Tool: "home_assistant", Operation: "call_service", Target: "ha:light.desk/light.turn_on"},
		{Tool: "home_assistant", Operation: "get_state", Target: "ha:light.desk"},
		{Tool: "send_notification", Operation: "execute", Target: "notification:push"},
	}
	if err := ValidateMQTTRelayGrants(valid); err != nil {
		t.Fatal(err)
	}
	for _, rule := range []ActionPolicyRule{
		{Tool: "mqtt_publish", Operation: "publish", Target: "*"},
		{Tool: "mqtt_publish", Operation: "*", Target: "mqtt:home/light/set"},
		{Tool: "mqtt_publish", Operation: "publish", Target: "mqtt:home/#"},
		{Tool: "home_assistant", Operation: "call_service", Target: "tool:home_assistant"},
		{Tool: "home_assistant", Operation: "call_service", Target: "ha:all/light.turn_on"},
		{Tool: "execute_shell", Operation: "execute", Target: "host:local"},
		{Tool: "send_notification", Operation: "execute", Target: "notification:all"},
	} {
		if ValidateMQTTRelayGrants([]ActionPolicyRule{rule}) == nil {
			t.Fatalf("accepted broad or unsupported grant: %+v", rule)
		}
	}
	var patch map[string]any
	if err := json.Unmarshal([]byte(`{"mqtt":{"relay_grants":[{"tool":"mqtt_publish","operation":"publish","target":"mqtt:home/set","unexpected":true}]}}`), &patch); err != nil {
		t.Fatal(err)
	}
	if ValidateMQTTPatch(patch) == nil {
		t.Fatal("unknown relay grant fields must be rejected before persistence")
	}
}

func TestMQTTRelayFiltersRequireLiteralPrefix(t *testing.T) {
	for _, filter := range []string{"#", "+/state", "/home/#"} {
		cfg := &Config{}
		cfg.MQTT.RelayTopics = []string{filter}
		if ValidateMQTTConfig(cfg) == nil {
			t.Fatalf("accepted unscoped relay filter %q", filter)
		}
	}
	cfg := &Config{}
	cfg.MQTT.RelayTopics = []string{"home/+/state", "frigate/#"}
	if err := ValidateMQTTConfig(cfg); err != nil {
		t.Fatal(err)
	}
}
