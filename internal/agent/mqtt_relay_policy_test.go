package agent

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"aurago/internal/config"
	"aurago/internal/security"
	"aurago/internal/tools"
)

func mqttRelayPolicyFixture() (*config.Config, *DispatchContext) {
	cfg := &config.Config{}
	cfg.MQTT.Enabled = true
	cfg.MQTT.Broker = "mqtt://broker.invalid:1883"
	cfg.MQTT.AllowUnauthenticatedRelay, cfg.MQTT.AllowUntrustedTransport = true, true
	cfg.MQTT.RelayTopics = []string{"home/input"}
	cfg.MQTT.RelayGrants = []config.ActionPolicyRule{{Tool: "mqtt_publish", Operation: "publish", Target: "mqtt:home/light/set"}}
	return cfg, &DispatchContext{Cfg: cfg, Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), SessionID: "mqtt", MessageSource: "mqtt", MQTTRelayTopic: "home/input"}
}

func TestMQTTRelayActionGrantsApplyWithGeneralPolicyDisabled(t *testing.T) {
	cfg, dc := mqttRelayPolicyFixture()
	if cfg.Agent.ActionPolicy.Enabled {
		t.Fatal("fixture must keep the general action policy off")
	}
	for _, tc := range []struct {
		topic   string
		allowed bool
	}{{"home/light/set", true}, {"home/door/unlock", false}, {"home/light/set ", false}, {"", false}} {
		call := ToolCall{Action: "mqtt_publish", Operation: "read", Params: map[string]any{"topic": tc.topic, "payload": "ON"}}
		action := DescribeAction(call, dc)
		if action.Operation != "publish" || !action.HighRisk || !action.Mutating {
			t.Fatal("model-written operation weakened MQTT publish classification")
		}
		if allowed := checkMQTTRelayAction(call, dc, action) == nil; allowed != tc.allowed {
			t.Fatalf("topic=%q allowed=%v want=%v", tc.topic, allowed, tc.allowed)
		}
	}
	call := ToolCall{Action: "execute_shell", Code: "whoami"}
	if checkMQTTRelayAction(call, dc, DescribeAction(call, dc)) == nil {
		t.Fatal("relay gained code execution")
	}
}

func TestMQTTRelayRevocationIntersectsCapturedAndLiveGrants(t *testing.T) {
	captured, dc := mqttRelayPolicyFixture()
	live := *captured
	live.MQTT.RelayGrants = append([]config.ActionPolicyRule(nil), captured.MQTT.RelayGrants...)
	captured.AuthorizationSnapshots = func() (*config.Config, *config.Config) { return captured, &live }
	call := ToolCall{Action: "mqtt_publish", Params: map[string]any{"topic": "home/light/set", "payload": "ON"}}
	if checkMQTTRelayAction(call, dc, DescribeAction(call, dc)) != nil {
		t.Fatal("matching live grant refused")
	}
	live.MQTT.RelayGrants = nil
	if checkMQTTRelayAction(call, dc, DescribeAction(call, dc)) == nil {
		t.Fatal("revoked grant survived in-flight")
	}
	live.MQTT.RelayGrants = captured.MQTT.RelayGrants
	live.MQTT.RelayTopics = []string{"home/other"}
	if checkMQTTRelayAction(call, dc, DescribeAction(call, dc)) == nil {
		t.Fatal("revoked ingress topic survived in-flight")
	}
	live.MQTT.RelayTopics = captured.MQTT.RelayTopics
	live.MQTT.RelayGrants = []config.ActionPolicyRule{{Tool: "mqtt_publish", Operation: "publish", Target: "mqtt:home/new"}}
	call.Params["topic"] = "home/new"
	if checkMQTTRelayAction(call, dc, DescribeAction(call, dc)) == nil {
		t.Fatal("a live grant widened a captured run")
	}
}

func TestMQTTRelayHomeAssistantGrantBindsEveryEntityAndService(t *testing.T) {
	cfg, dc := mqttRelayPolicyFixture()
	cfg.HomeAssistant.Enabled = true
	cfg.MQTT.RelayGrants = []config.ActionPolicyRule{{Tool: "home_assistant", Operation: "call_service", Target: "ha:light.desk/light.turn_on"}}
	for _, tc := range []struct {
		entity, service string
		data            map[string]any
		allowed         bool
	}{
		{"light.desk", "turn_on", nil, true},
		{"light.desk", "turn_off", nil, false},
		{"light.other", "turn_on", nil, false},
		{"light.desk,light.other", "turn_on", nil, false},
		{"", "turn_on", nil, false},
		{"light.desk", "turn_on", map[string]any{"area_id": "all"}, false},
	} {
		call := ToolCall{Action: "home_assistant", Operation: "call_service", Domain: "light", EntityID: tc.entity, Service: tc.service, ServiceData: tc.data}
		if allowed := checkMQTTRelayAction(call, dc, DescribeAction(call, dc)) == nil; allowed != tc.allowed {
			t.Fatalf("entity=%q service=%q data=%v allowed=%v want=%v", tc.entity, tc.service, tc.data, allowed, tc.allowed)
		}
	}
}

func TestMQTTRelayLiveHomeAssistantReadOnlyAndDestinationRevocation(t *testing.T) {
	cfg, dc := mqttRelayPolicyFixture()
	cfg.HomeAssistant.Enabled = true
	cfg.HomeAssistant.URL = "http://ha.fixture.invalid"
	cfg.MQTT.RelayGrants = []config.ActionPolicyRule{{Tool: "home_assistant", Operation: "call_service", Target: "ha:light.desk/light.turn_on"}}
	live := *cfg
	cfg.AuthorizationSnapshots = func() (*config.Config, *config.Config) { return cfg, &live }
	call := ToolCall{Action: "home_assistant", Operation: "call_service", Domain: "light", Service: "turn_on", EntityID: "light.desk"}
	if checkMQTTRelayAction(call, dc, DescribeAction(call, dc)) != nil {
		t.Fatal("granted service was refused")
	}
	live.HomeAssistant.ReadOnly = true
	if checkMQTTRelayAction(call, dc, DescribeAction(call, dc)) == nil {
		t.Fatal("live Home Assistant read-only was bypassed")
	}
	live.HomeAssistant.ReadOnly = false
	live.HomeAssistant.URL = "http://replacement.fixture.invalid"
	if checkMQTTRelayAction(call, dc, DescribeAction(call, dc)) == nil {
		t.Fatal("in-flight run kept a replaced Home Assistant destination")
	}
}

func TestMQTTRelayPublishNeverSendsRegisteredSecrets(t *testing.T) {
	cfg, dc := mqttRelayPolicyFixture()
	useRuntimePermissionsForTest(t, cfg)
	const secret = "mqtt-egress-fixture-secret-7483"
	security.RegisterSensitive(secret)
	call := ToolCall{Action: "mqtt_publish", Params: map[string]any{"topic": "home/light/set", "payload": secret}}
	if !DescribeAction(call, dc).egressSecret {
		t.Fatal("MQTT publish was not classified as registered-secret egress")
	}
	if refusal := checkDispatchActionPolicy(context.Background(), call, dc); refusal == nil {
		t.Fatal("explicit relay grant bypassed registered-secret egress protection")
	}
}

func TestMQTTRelayPublishDispatchRetainsReadOnlyAndAliasChecks(t *testing.T) {
	cfg, dc := mqttRelayPolicyFixture()
	useRuntimePermissionsForTest(t, cfg)
	dc.ToolScopeRestricted = true
	dc.AllowedTools = normalizedAllowedToolSet([]string{"mqtt_publish", "invoke_tool"})
	called := 0
	tools.RegisterMQTTBridge(func(string, string, int, bool, *slog.Logger) error { called++; return nil }, nil, nil, nil)
	t.Cleanup(func() { tools.RegisterMQTTBridge(nil, nil, nil, nil) })
	call := ToolCall{Action: "mqtt_publish", Params: map[string]any{"topic": "home/light/set", "payload": "ON"}}
	result := DispatchToolCallResult(context.Background(), &call, dc, "fixture")
	if result.IsError || called != 1 {
		t.Fatalf("explicit publish: called=%d result=%+v", called, result)
	}
	cfg.MQTT.ReadOnly = true
	DispatchToolCallResult(context.Background(), &call, dc, "fixture")
	if called != 1 {
		t.Fatal("relay grant overrode MQTT read-only")
	}
	cfg.MQTT.ReadOnly = false
	wrapped := ToolCall{Action: "invoke_tool", Params: map[string]any{"tool_name": "mqtt_publish", "arguments": map[string]any{"topic": "home/door/unlock", "payload": "ON"}}}
	DispatchToolCallResult(context.Background(), &wrapped, dc, "fixture")
	if called != 1 {
		t.Fatal("invoke_tool bypassed target grants")
	}
}
