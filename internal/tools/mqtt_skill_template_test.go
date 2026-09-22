package tools

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestMQTTPublisherTemplateChecksConnectionAndOperationAcknowledgements(t *testing.T) {
	template := tplMQTT
	for _, marker := range []string{
		"timeout must be greater than zero",
		"connected.wait(timeout)",
		"MQTT CONNACK timed out",
		"def numeric_rc(value):",
		"publish_rc",
		"result.is_published()",
		"subscribe_rc",
		"subscribed.wait(timeout)",
		"MQTT subscription rejected",
		"if len(messages) < 50",
		"client.loop_stop()",
		"client.disconnect()",
	} {
		if !strings.Contains(template, marker) {
			t.Fatalf("MQTT publisher template missing acknowledgement/cleanup marker %q", marker)
		}
	}
}

func TestMQTTPublisherTemplateRejectsMissingAndMalformedSUBACK(t *testing.T) {
	pythonCmd := findPythonForSkillTemplateTest(t)
	script := `import json
import os
import sys
import time
import types

class FakeClient:
    last = None
    def __init__(self, *args, **kwargs):
        self.unsubscribed = False
        self.loop_stopped = False
        self.disconnected = False
        FakeClient.last = self
    def connect(self, *args, **kwargs):
        mode = os.environ.get("MQTT_TEMPLATE_TEST_MODE")
        if mode == "connect_error": return 4
        if mode == "connack_timeout": return 0
        reason = 5 if mode == "connack_error" else 0
        self.on_connect(self, None, {}, reason, None)
        return 0
    def loop_start(self): pass
    def loop_stop(self): self.loop_stopped = True
    def disconnect(self): self.disconnected = True; return 0
    def publish(self, *args, **kwargs):
        mode = os.environ.get("MQTT_TEMPLATE_TEST_MODE")
        return PublishResult(4 if mode == "publish_rc" else 0, mode != "publish_timeout")
    def subscribe(self, *args, **kwargs):
        mode = os.environ.get("MQTT_TEMPLATE_TEST_MODE")
        if mode == "subscribe_rc": return (4, 1)
        if mode == "missing": return (0, 1)
        granted = None if mode == "malformed" else (2 if mode == "excess_qos" else (128 if mode == "rejected" else 0))
        self.on_subscribe(self, None, 1, [granted], None)
        if mode == "valid":
            for index in range(100):
                self.on_message(self, None, types.SimpleNamespace(topic="test/topic", payload=b"message", qos=0))
        return (0, 1)
    def unsubscribe(self, *args, **kwargs): self.unsubscribed = True; return (0, 2)

class PublishResult:
    def __init__(self, rc, published): self.rc, self.published = rc, published
    def wait_for_publish(self, timeout=None): pass
    def is_published(self): return self.published

paho = types.ModuleType("paho")
mqtt = types.ModuleType("paho.mqtt")
client_module = types.ModuleType("paho.mqtt.client")
client_module.Client = FakeClient
client_module.CallbackAPIVersion = types.SimpleNamespace(VERSION2=2)
client_module.MQTT_ERR_SUCCESS = 0
paho.mqtt = mqtt
mqtt.client = client_module
sys.modules.update({"paho": paho, "paho.mqtt": mqtt, "paho.mqtt.client": client_module})

scope = {}
exec(source.replace("{{.FunctionName}}", "probe").replace("{{.Description}}", "test"), scope)
def run(mode, action, expected=None):
    os.environ["MQTT_TEMPLATE_TEST_MODE"] = mode
    result = scope["probe"](action, "test/topic", "payload", timeout=0.01)
    if expected is None:
        assert result["status"] == "success", result
    else:
        assert result["status"] == "error", result
        assert expected in result["message"], result
    assert FakeClient.last.disconnected, (mode, result)
    if mode != "connect_error":
        assert FakeClient.last.loop_stopped, (mode, result)
    if action == "subscribe" and mode != "subscribe_rc":
        assert FakeClient.last.unsubscribed, (mode, result)
    return result

for mode, expected in (("connect_error", "MQTT connect failed"), ("connack_error", "MQTT CONNACK rejected"),
                       ("connack_timeout", "MQTT CONNACK timed out"),
                       ("subscribe_rc", "MQTT subscribe failed"), ("missing", "MQTT SUBACK timed out"),
                       ("malformed", "MQTT subscription rejected"), ("excess_qos", "MQTT subscription rejected"),
                       ("rejected", "MQTT subscription rejected"), ("publish_rc", "MQTT publish failed"),
                       ("publish_timeout", "MQTT publish did not complete")):
    run(mode, "subscribe" if mode in ("subscribe_rc", "missing", "malformed", "excess_qos", "rejected") else "publish", expected)

os.environ["MQTT_TEMPLATE_TEST_MODE"] = "valid"
result = run("valid", "subscribe")
assert result["result"]["messages_received"] == 50, result
assert len(result["result"]["messages"]) == 50, result
assert FakeClient.last.unsubscribed and FakeClient.last.loop_stopped and FakeClient.last.disconnected
result = run("valid", "publish")
print(json.dumps(result))
`
	script = "source=" + strconv.Quote(tplMQTT) + "\n" + script
	path := filepath.Join(t.TempDir(), "mqtt_template_probe.py")
	if err := os.WriteFile(path, []byte(script), 0o600); err != nil {
		t.Fatalf("write template probe: %v", err)
	}
	cmd := pythonCmd.command(path)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("MQTT template acknowledgement probe failed: %v\n%s", err, output)
	}
}
