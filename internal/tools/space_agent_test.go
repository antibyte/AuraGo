package tools

import (
	"encoding/json"
	"os"
	"path/filepath"
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
	labels := got["Labels"].(map[string]interface{})
	if labels["org.aurago.space-agent.build-revision"] != spaceAgentImageBuildRevision {
		t.Fatalf("build revision label = %#v", labels)
	}
	env, ok := got["Env"].([]interface{})
	if !ok {
		t.Fatalf("Env missing or wrong type: %#v", got["Env"])
	}
	for _, want := range []string{
		"HOST=0.0.0.0",
		"PORT=3210",
		"HOME=/app/home",
		"XDG_CONFIG_HOME=/app/home/.config",
		"XDG_DATA_HOME=/app/home/.local/share",
		"CUSTOMWARE_PATH=/app/customware",
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
	if len(binds) != 4 {
		t.Fatalf("bind count = %d, want 4: %#v", len(binds), binds)
	}
	bindText := strings.Join(interfaceStrings(binds), "\n")
	if !strings.Contains(bindText, "/app/.space-agent") || !strings.Contains(bindText, "/app/home") || !strings.Contains(bindText, "/app/customware") || !strings.Contains(bindText, "/app/supervisor") {
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

func TestBuildSpaceAgentCreatePayloadUsesLANReachablePublishHost(t *testing.T) {
	payload, err := buildSpaceAgentCreatePayload(SpaceAgentSidecarConfig{
		Image:          "aurago-space-agent:test",
		ContainerName:  "aurago_space_agent",
		Host:           "127.0.0.1",
		Port:           3210,
		DataPath:       `C:\aurago\data\sidecars\space-agent\data`,
		CustomwarePath: `C:\aurago\data\sidecars\space-agent\customware`,
	})
	if err != nil {
		t.Fatalf("buildSpaceAgentCreatePayload() error = %v", err)
	}

	var got map[string]interface{}
	if err := json.Unmarshal(payload, &got); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	env := got["Env"].([]interface{})
	if !containsInterfaceString(env, "HOST=0.0.0.0") {
		t.Fatalf("Space Agent must listen on all container interfaces, env = %#v", env)
	}
	if containsInterfaceString(env, "HOST=127.0.0.1") {
		t.Fatalf("Space Agent must not listen on container loopback only, env = %#v", env)
	}
	hostConfig := got["HostConfig"].(map[string]interface{})
	portBindings := hostConfig["PortBindings"].(map[string]interface{})
	bound := portBindings["3210/tcp"].([]interface{})[0].(map[string]interface{})
	if bound["HostIp"] != "0.0.0.0" {
		t.Fatalf("loopback publish host must be widened for LAN access, PortBindings = %#v", bound)
	}
}

func TestSpaceAgentContainerNeedsRecreateForLoopbackOnlyListener(t *testing.T) {
	inspect := []byte(`{
		"Config": {"Env": ["HOST=127.0.0.1", "PORT=3210"]},
		"HostConfig": {
			"PortBindings": {
				"3210/tcp": [{"HostIp": "127.0.0.1", "HostPort": "3210"}]
			}
		}
	}`)
	if !spaceAgentContainerNeedsRecreate(inspect, SpaceAgentSidecarConfig{Host: "127.0.0.1", Port: 3210}) {
		t.Fatal("expected loopback-only existing container to require recreation")
	}
}

func TestSpaceAgentContainerNeedsRecreateWhenCustomwarePathEnvMissing(t *testing.T) {
	inspect := []byte(`{
		"Config": {"Env": ["HOST=0.0.0.0", "PORT=3210"]},
		"HostConfig": {
			"PortBindings": {
				"3210/tcp": [{"HostIp": "0.0.0.0", "HostPort": "3210"}]
			}
		}
	}`)
	if !spaceAgentContainerNeedsRecreate(inspect, SpaceAgentSidecarConfig{Host: "0.0.0.0", Port: 3210}) {
		t.Fatal("expected container without CUSTOMWARE_PATH to require recreation")
	}
}

func TestSpaceAgentContainerNeedsRecreateWhenHomeEnvMissing(t *testing.T) {
	inspect := []byte(`{
		"Config": {
			"Env": ["HOST=0.0.0.0", "PORT=3210", "CUSTOMWARE_PATH=/app/customware"],
			"Labels": {"org.aurago.space-agent.build-revision": "20260501-container-user-home"}
		},
		"HostConfig": {
			"PortBindings": {
				"3210/tcp": [{"HostIp": "0.0.0.0", "HostPort": "3210"}]
			}
		}
	}`)
	if !spaceAgentContainerNeedsRecreate(inspect, SpaceAgentSidecarConfig{Host: "0.0.0.0", Port: 3210}) {
		t.Fatal("expected container without persistent HOME to require recreation")
	}
}

func TestSpaceAgentContainerNeedsRecreateAcceptsLANReachableBinding(t *testing.T) {
	inspect := []byte(`{
		"Config": {
			"Env": ["HOST=0.0.0.0", "PORT=3210", "CUSTOMWARE_PATH=/app/customware", "HOME=/app/home"],
			"Labels": {"org.aurago.space-agent.build-revision": "20260501-container-user-home"}
		},
		"HostConfig": {
			"PortBindings": {
				"3210/tcp": [{"HostIp": "0.0.0.0", "HostPort": "3210"}]
			}
		}
	}`)
	if spaceAgentContainerNeedsRecreate(inspect, SpaceAgentSidecarConfig{Host: "127.0.0.1", Port: 3210}) {
		t.Fatal("did not expect LAN-reachable existing container to require recreation")
	}
}

func TestSpaceAgentContainerNeedsRecreateWhenImageRevisionIsOld(t *testing.T) {
	inspect := []byte(`{
		"Config": {
			"Env": ["HOST=0.0.0.0", "PORT=3210", "CUSTOMWARE_PATH=/app/customware", "HOME=/app/home"],
			"Labels": {"org.aurago.space-agent.build-revision": "old"}
		},
		"HostConfig": {
			"PortBindings": {
				"3210/tcp": [{"HostIp": "0.0.0.0", "HostPort": "3210"}]
			}
		}
	}`)
	if !spaceAgentContainerNeedsRecreate(inspect, SpaceAgentSidecarConfig{Host: "127.0.0.1", Port: 3210}) {
		t.Fatal("expected old image revision to require recreation")
	}
}

func TestSpaceAgentDockerfileInstallsGit(t *testing.T) {
	dockerfile := spaceAgentDockerfile()
	for _, want := range []string{"apt-get install", "git", "openssh-client"} {
		if !strings.Contains(dockerfile, want) {
			t.Fatalf("Dockerfile missing %q:\n%s", want, dockerfile)
		}
	}
}

func TestSpaceAgentDockerfileRunsAuraGoBootstrap(t *testing.T) {
	dockerfile := spaceAgentDockerfile()
	for _, want := range []string{"aurago_space_bootstrap.mjs", "node aurago_space_bootstrap.mjs", "--state-dir /app/supervisor"} {
		if !strings.Contains(dockerfile, want) {
			t.Fatalf("Dockerfile missing %q:\n%s", want, dockerfile)
		}
	}
}

func TestSpaceAgentBootstrapScriptCreatesManagedAdminUser(t *testing.T) {
	script := spaceAgentBootstrapScript()
	for _, want := range []string{
		"SPACE_AGENT_ADMIN_USER",
		"SPACE_AGENT_ADMIN_PASSWORD",
		"loadSupervisorAuthEnv",
		"seedWorkspaceFiles(path.join(process.env.CUSTOMWARE_PATH, \"L2\", normalizeEntityId(username)))",
		"createUser(projectRoot, username, password",
		"setUserPassword(projectRoot, username, password",
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("bootstrap script missing %q:\n%s", want, script)
		}
	}
}

func TestEnsureSpaceAgentHomeSeedsExpectedWorkspaceFiles(t *testing.T) {
	home := t.TempDir()
	if err := ensureSpaceAgentHome(home); err != nil {
		t.Fatalf("ensureSpaceAgentHome() error = %v", err)
	}
	for _, dir := range []string{
		filepath.Join(home, "meta"),
		filepath.Join(home, "spaces"),
		filepath.Join(home, "dashboard"),
		filepath.Join(home, "onscreen-agent"),
		filepath.Join(home, ".config"),
		filepath.Join(home, ".local", "share"),
	} {
		if info, err := os.Stat(dir); err != nil || !info.IsDir() {
			t.Fatalf("expected seeded dir %s, info=%#v err=%v", dir, info, err)
		}
	}
	content, err := os.ReadFile(filepath.Join(home, "meta", "login_hooks.json"))
	if err != nil {
		t.Fatalf("ReadFile(login_hooks.json) error = %v", err)
	}
	if strings.TrimSpace(string(content)) != "[]" {
		t.Fatalf("login_hooks.json = %q, want []", string(content))
	}
	for path, want := range map[string]string{
		filepath.Join(home, "dashboard", "prefs.json"):               "{}",
		filepath.Join(home, "onscreen-agent", "config.json"):         "{}",
		filepath.Join(home, "onscreen-agent", "history.json"):        "[]",
		filepath.Join(home, "meta", "dashboard-prefs.json"):          "{}",
		filepath.Join(home, ".config", "onscreen-agent-config.json"): "{}",
	} {
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("ReadFile(%s) error = %v", path, err)
		}
		if strings.TrimSpace(string(content)) != want {
			t.Fatalf("%s = %q, want %s", path, string(content), want)
		}
	}
}

func TestEnsureSpaceAgentCustomwareUserHomeSeedsL2WorkspaceFiles(t *testing.T) {
	customware := t.TempDir()
	if err := ensureSpaceAgentCustomwareUserHome(customware, "admin"); err != nil {
		t.Fatalf("ensureSpaceAgentCustomwareUserHome() error = %v", err)
	}
	userHome := filepath.Join(customware, "L2", "admin")
	for path, want := range map[string]string{
		filepath.Join(userHome, "meta", "login_hooks.json"):          "[]",
		filepath.Join(userHome, ".config", "dashboard-prefs.json"):   "{}",
		filepath.Join(userHome, "onscreen-agent", "config.json"):     "{}",
		filepath.Join(userHome, "onscreen-agent", "history.json"):    "[]",
		filepath.Join(userHome, "dashboard", "dashboard-prefs.json"): "{}",
	} {
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("ReadFile(%s) error = %v", path, err)
		}
		if strings.TrimSpace(string(content)) != want {
			t.Fatalf("%s = %q, want %s", path, string(content), want)
		}
	}
}

func TestEnsureSpaceAgentCustomwareUserHomeRejectsPathTraversalUser(t *testing.T) {
	if err := ensureSpaceAgentCustomwareUserHome(t.TempDir(), "../admin"); err == nil {
		t.Fatal("expected path traversal admin user to be rejected")
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
