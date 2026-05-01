package tools

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestBuildSpaceAgentCreatePayload(t *testing.T) {
	payload, err := buildSpaceAgentCreatePayload(SpaceAgentSidecarConfig{
		Image:          "aurago-space-agent:test",
		ContainerName:  "aurago_space_agent",
		Host:           "0.0.0.0",
		Port:           3210,
		DataPath:       `C:\aurago\data\sidecars\space-agent\data`,
		CustomwarePath: `C:\aurago\data\sidecars\space-agent\customware`,
		AdminUser:      "admin",
		AdminPassword:  "admin-secret",
		BridgeURL:      "http://127.0.0.1:8088/api/space-agent/bridge/messages",
		BridgeToken:    "bridge-secret",
	})
	if err != nil {
		t.Fatalf("buildSpaceAgentCreatePayload() error = %v", err)
	}

	raw := string(payload)
	for _, leaked := range []string{"sk-should-not-leak", "OPENAI_API_KEY", "LLM_API_KEY"} {
		if strings.Contains(raw, leaked) {
			t.Fatalf("payload leaked provider secret marker %q: %s", leaked, raw)
		}
	}

	var got map[string]interface{}
	if err := json.Unmarshal(payload, &got); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if got["Image"] != "aurago-space-agent:test" {
		t.Fatalf("Image = %v", got["Image"])
	}
	env, ok := got["Env"].([]interface{})
	if !ok {
		t.Fatalf("Env missing or wrong type: %#v", got["Env"])
	}
	for _, want := range []string{
		"HOST=0.0.0.0",
		"PORT=3210",
		"SPACE_AGENT_ADMIN_USER=admin",
		"SPACE_AGENT_ADMIN_PASSWORD=admin-secret",
		"AURAGO_BRIDGE_URL=http://127.0.0.1:8088/api/space-agent/bridge/messages",
		"AURAGO_BRIDGE_TOKEN=bridge-secret",
	} {
		if !containsInterfaceString(env, want) {
			t.Fatalf("Env missing %q in %#v", want, env)
		}
	}

	hostConfig := got["HostConfig"].(map[string]interface{})
	restart := hostConfig["RestartPolicy"].(map[string]interface{})
	if restart["Name"] != "unless-stopped" {
		t.Fatalf("restart policy = %#v", restart)
	}
	binds := hostConfig["Binds"].([]interface{})
	if len(binds) != 2 {
		t.Fatalf("bind count = %d, want 2: %#v", len(binds), binds)
	}
	bindText := strings.Join(interfaceStrings(binds), "\n")
	if !strings.Contains(bindText, "/app/.space-agent") || !strings.Contains(bindText, "/app/customware") {
		t.Fatalf("binds missing expected container paths: %s", bindText)
	}
	ports := got["ExposedPorts"].(map[string]interface{})
	if _, ok := ports["3210/tcp"]; !ok {
		t.Fatalf("ExposedPorts missing 3210/tcp: %#v", ports)
	}
	portBindings := hostConfig["PortBindings"].(map[string]interface{})
	bound := portBindings["3210/tcp"].([]interface{})[0].(map[string]interface{})
	if bound["HostIp"] != "0.0.0.0" || bound["HostPort"] != "3210" {
		t.Fatalf("PortBindings = %#v", bound)
	}
}

func containsInterfaceString(values []interface{}, want string) bool {
	for _, v := range values {
		if s, ok := v.(string); ok && s == want {
			return true
		}
	}
	return false
}

func interfaceStrings(values []interface{}) []string {
	out := make([]string, 0, len(values))
	for _, v := range values {
		if s, ok := v.(string); ok {
			out = append(out, s)
		}
	}
	return out
}
