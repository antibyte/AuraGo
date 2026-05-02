package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/config"
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
			"Labels": {"org.aurago.space-agent.build-revision": "20260502-aurago-onscreen-history-bridge"}
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
			"Labels": {"org.aurago.space-agent.build-revision": "20260502-aurago-onscreen-history-bridge"}
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

func TestSpaceAgentContainerNeedsRecreateWhenBridgeEnvIsStale(t *testing.T) {
	inspect := []byte(`{
		"Config": {
			"Env": [
				"HOST=0.0.0.0",
				"PORT=3210",
				"CUSTOMWARE_PATH=/app/customware",
				"HOME=/app/home",
				"AURAGO_BRIDGE_URL=https://old.example/api/bridge",
				"AURAGO_BRIDGE_TOKEN=old-token"
			],
			"Labels": {"org.aurago.space-agent.build-revision": "20260502-aurago-onscreen-history-bridge"}
		},
		"HostConfig": {
			"PortBindings": {
				"3210/tcp": [{"HostIp": "0.0.0.0", "HostPort": "3210"}]
			}
		}
	}`)
	if !spaceAgentContainerNeedsRecreate(inspect, SpaceAgentSidecarConfig{
		Host:        "0.0.0.0",
		Port:        3210,
		BridgeURL:   "https://new.example/api/bridge",
		BridgeToken: "new-token",
	}) {
		t.Fatal("expected stale bridge env to require recreation")
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

func TestSpaceAgentInstructionsAPIEndpointRequiresBridgeToken(t *testing.T) {
	endpoint := spaceAgentInstructionsAPIEndpoint()
	for _, want := range []string{
		"export const allowAnonymous = true;",
		"export async function post(context)",
		"AURAGO_BRIDGE_TOKEN",
		"X-AuraGo-Instruction",
		"Authorization",
		"Bearer",
		"Context from AuraGo:",
		"writeFile",
		"aurago_inbox",
		"appendOnscreenAgentHistory",
		"onscreen-agent.json",
		`accepted: true`,
		`queued: true`,
	} {
		if !strings.Contains(endpoint, want) {
			t.Fatalf("instructions api endpoint missing %q:\n%s", want, endpoint)
		}
	}
}

func TestWriteSpaceAgentInstructionsAPIEndpointRemovesStaleVariants(t *testing.T) {
	source := t.TempDir()
	for _, stalePath := range []string{
		filepath.Join(source, "server", "api", "aurago_instructions.js"),
		filepath.Join(source, "api", "aurago", "instructions.py"),
		filepath.Join(source, "api", "aurago_instructions.py"),
		filepath.Join(source, "python", "api", "aurago_instructions.py"),
	} {
		if err := os.MkdirAll(filepath.Dir(stalePath), 0o750); err != nil {
			t.Fatalf("MkdirAll(%s): %v", stalePath, err)
		}
		if err := os.WriteFile(stalePath, []byte("stale"), 0o600); err != nil {
			t.Fatalf("WriteFile(%s): %v", stalePath, err)
		}
	}

	if err := writeSpaceAgentInstructionsAPIEndpoint(source); err != nil {
		t.Fatalf("writeSpaceAgentInstructionsAPIEndpoint() error = %v", err)
	}

	for _, stalePath := range []string{
		filepath.Join(source, "server", "api", "aurago_instructions.js"),
		filepath.Join(source, "api", "aurago", "instructions.py"),
		filepath.Join(source, "api", "aurago_instructions.py"),
		filepath.Join(source, "python", "api", "aurago_instructions.py"),
	} {
		if _, err := os.Stat(stalePath); !os.IsNotExist(err) {
			t.Fatalf("stale endpoint %s still exists or stat failed: %v", stalePath, err)
		}
	}
	currentPath := filepath.Join(source, "python", "api", "message_async.py")
	if _, err := os.Stat(currentPath); !os.IsNotExist(err) {
		t.Fatalf("stale Python endpoint %s still exists or stat failed: %v", currentPath, err)
	}
	currentPath = filepath.Join(source, "server", "api", "message_async.js")
	content, err := os.ReadFile(currentPath)
	if err != nil {
		t.Fatalf("ReadFile(%s): %v", currentPath, err)
	}
	if !strings.Contains(string(content), "export async function post") || !strings.Contains(string(content), "aurago_inbox") {
		t.Fatalf("current endpoint content missing active JS handler: %s", string(content))
	}
}

func TestAnnotateSpaceAgentInstructionHTTPErrorExplainsMissingEndpoint(t *testing.T) {
	result := map[string]interface{}{"status": "error", "error": "File not found"}

	annotateSpaceAgentInstructionHTTPError(result, 404)

	if result["requires_recreate"] != true {
		t.Fatalf("requires_recreate = %#v, want true", result["requires_recreate"])
	}
	message, _ := result["message"].(string)
	if !strings.Contains(message, "reachable") || !strings.Contains(message, "Recreate") {
		t.Fatalf("message does not explain missing endpoint: %q", message)
	}
}

func TestParseSpaceAgentInstructionResponseBodyIncludesPlainTextErrors(t *testing.T) {
	got := parseSpaceAgentInstructionResponseBody(500, []byte("Internal server error"))

	if got["http_status"] != 500 {
		t.Fatalf("http_status = %#v, want 500", got["http_status"])
	}
	if got["body"] != "Internal server error" || got["message"] != "Internal server error" {
		t.Fatalf("plain text error not preserved: %#v", got)
	}
}

func TestParseSpaceAgentInstructionResponseBodyPreservesJSONErrors(t *testing.T) {
	got := parseSpaceAgentInstructionResponseBody(500, []byte(`{"status":"error","message":"boom"}`))

	if got["status"] != "error" || got["message"] != "boom" {
		t.Fatalf("json error not preserved: %#v", got)
	}
	if got["http_status"] != 500 {
		t.Fatalf("http_status = %#v, want 500", got["http_status"])
	}
}

func TestParseSpaceAgentInstructionResponseBodyAddsOKStatusForSuccessfulEndpointShape(t *testing.T) {
	got := parseSpaceAgentInstructionResponseBody(200, []byte(`{"accepted":true,"delivered":"inbox"}`))

	if got["status"] != "ok" {
		t.Fatalf("status = %#v, want ok in AuraGo tool result: %#v", got["status"], got)
	}
	if got["accepted"] != true || got["delivered"] != "inbox" {
		t.Fatalf("endpoint payload not preserved: %#v", got)
	}
}

func TestSendSpaceAgentInstructionUsesMessageAsyncBridgeHeaders(t *testing.T) {
	var gotPath string
	var gotAuth string
	var gotBridgeHeader string
	var gotBody SpaceAgentInstruction
	originalClient := spaceAgentHTTPClient
	spaceAgentHTTPClient = &http.Client{Transport: spaceAgentRoundTripFunc(func(r *http.Request) (*http.Response, error) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		gotBridgeHeader = r.Header.Get("X-AuraGo-Instruction")
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			return nil, err
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(bytes.NewBufferString(`{"accepted":true,"queued":true}`)),
			Request:    r,
		}, nil
	})}
	defer func() { spaceAgentHTTPClient = originalClient }()

	cfg := &config.Config{}
	cfg.SpaceAgent.Enabled = true
	cfg.SpaceAgent.Host = "127.0.0.1"
	cfg.SpaceAgent.Port = 3100
	cfg.SpaceAgent.BridgeToken = "bridge-token"

	got := SendSpaceAgentInstruction(context.Background(), cfg, SpaceAgentInstruction{
		Instruction: "build a weather widget",
		Information: "Pforzheim",
		SessionID:   "sess-1",
	})

	if got["status"] != "ok" || got["accepted"] != true || got["queued"] != true {
		t.Fatalf("unexpected tool result: %#v", got)
	}
	if got["endpoint"] != "/api/message_async" {
		t.Fatalf("endpoint = %#v, want /api/message_async", got["endpoint"])
	}
	if gotPath != "/api/message_async" {
		t.Fatalf("path = %q, want /api/message_async", gotPath)
	}
	if gotAuth != "Bearer bridge-token" {
		t.Fatalf("Authorization = %q", gotAuth)
	}
	if gotBridgeHeader != "1" {
		t.Fatalf("X-AuraGo-Instruction = %q", gotBridgeHeader)
	}
	if gotBody.Instruction != "build a weather widget" || gotBody.Information != "Pforzheim" || gotBody.SessionID != "sess-1" {
		t.Fatalf("request body = %#v", gotBody)
	}
}

func TestSendSpaceAgentInstructionFallsBackToMessageEndpointOn404(t *testing.T) {
	var gotPaths []string
	originalClient := spaceAgentHTTPClient
	spaceAgentHTTPClient = &http.Client{Transport: spaceAgentRoundTripFunc(func(r *http.Request) (*http.Response, error) {
		gotPaths = append(gotPaths, r.URL.Path)
		if r.URL.Path == "/api/message_async" {
			return &http.Response{
				StatusCode: http.StatusNotFound,
				Body:       io.NopCloser(bytes.NewBufferString(`{"status":"error","error":"File not found"}`)),
				Request:    r,
			}, nil
		}
		if r.Header.Get("X-AuraGo-Instruction") != "1" {
			t.Fatalf("fallback request missing bridge header")
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(bytes.NewBufferString(`{"accepted":true,"queued":true}`)),
			Request:    r,
		}, nil
	})}
	defer func() { spaceAgentHTTPClient = originalClient }()

	cfg := &config.Config{}
	cfg.SpaceAgent.Enabled = true
	cfg.SpaceAgent.Host = "127.0.0.1"
	cfg.SpaceAgent.Port = 3100
	cfg.SpaceAgent.BridgeToken = "bridge-token"

	got := SendSpaceAgentInstruction(context.Background(), cfg, SpaceAgentInstruction{Instruction: "build a weather widget"})

	if got["status"] != "ok" || got["endpoint"] != "/api/message" {
		t.Fatalf("unexpected fallback result: %#v", got)
	}
	if strings.Join(gotPaths, ",") != "/api/message_async,/api/message" {
		t.Fatalf("paths = %#v", gotPaths)
	}
}

func TestSendSpaceAgentInstructionWritesMailboxWhenInboundAPIIsMissing(t *testing.T) {
	var gotPaths []string
	originalClient := spaceAgentHTTPClient
	spaceAgentHTTPClient = &http.Client{Transport: spaceAgentRoundTripFunc(func(r *http.Request) (*http.Response, error) {
		gotPaths = append(gotPaths, r.URL.Path)
		return &http.Response{
			StatusCode: http.StatusNotFound,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(bytes.NewBufferString(`{"status":"error","error":"File not found"}`)),
			Request:    r,
		}, nil
	})}
	defer func() { spaceAgentHTTPClient = originalClient }()

	dataPath := t.TempDir()
	customwarePath := t.TempDir()
	cfg := &config.Config{}
	cfg.SpaceAgent.Enabled = true
	cfg.SpaceAgent.Host = "127.0.0.1"
	cfg.SpaceAgent.Port = 3100
	cfg.SpaceAgent.BridgeToken = "bridge-token"
	cfg.SpaceAgent.DataPath = dataPath
	cfg.SpaceAgent.CustomwarePath = customwarePath
	cfg.SpaceAgent.AdminUser = "admin"

	got := SendSpaceAgentInstruction(context.Background(), cfg, SpaceAgentInstruction{
		Instruction: "build a weather widget",
		Information: "Pforzheim",
		SessionID:   "sess-1",
	})

	if got["status"] != "ok" || got["delivered"] != "mailbox" || got["auto_execution"] != false {
		t.Fatalf("unexpected mailbox result: %#v", got)
	}
	if strings.Join(gotPaths, ",") != "/api/message_async,/api/message" {
		t.Fatalf("paths = %#v", gotPaths)
	}
	latest := filepath.Join(dataPath, "home", "aurago_inbox", "latest_instruction.json")
	content, err := os.ReadFile(latest)
	if err != nil {
		t.Fatalf("ReadFile(%s): %v", latest, err)
	}
	if !strings.Contains(string(content), "build a weather widget") || !strings.Contains(string(content), `"auto_execution": false`) {
		t.Fatalf("mailbox content missing instruction details: %s", string(content))
	}
	userLatest := filepath.Join(customwarePath, "L2", "admin", "aurago_inbox", "latest_instruction.json")
	userContent, err := os.ReadFile(userLatest)
	if err != nil {
		t.Fatalf("ReadFile(%s): %v", userLatest, err)
	}
	if !strings.Contains(string(userContent), "build a weather widget") || !strings.Contains(string(userContent), `"session_id": "sess-1"`) {
		t.Fatalf("user mailbox content missing latest instruction details: %s", string(userContent))
	}
	userHistory := filepath.Join(customwarePath, "L2", "admin", "hist", "onscreen-agent.json")
	userHistoryContent, err := os.ReadFile(userHistory)
	if err != nil {
		t.Fatalf("ReadFile(%s): %v", userHistory, err)
	}
	if !strings.Contains(string(userHistoryContent), "build a weather widget") || !strings.Contains(string(userHistoryContent), `"role": "user"`) {
		t.Fatalf("user onscreen history missing delivered instruction: %s", string(userHistoryContent))
	}
}

type spaceAgentRoundTripFunc func(*http.Request) (*http.Response, error)

func (f spaceAgentRoundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func TestSpaceAgentBootstrapScriptCreatesManagedAdminUser(t *testing.T) {
	script := spaceAgentBootstrapScript()
	for _, want := range []string{
		"SPACE_AGENT_ADMIN_USER",
		"SPACE_AGENT_ADMIN_PASSWORD",
		"loadSupervisorAuthEnv",
		"aurago_managed_user.json",
		"password_sha256",
		"bridgeHelperContent(bridgeHelperESMTemplate)",
		"bridgeConfigJSON()",
		"bridgeURLUsesLoopback",
		"browser_bridge_url_strategy",
		"process.env.AURAGO_BRIDGE_URL",
		"process.env.AURAGO_BRIDGE_TOKEN",
		"seedWorkspaceFiles(path.join(process.env.CUSTOMWARE_PATH, \"L2\", normalizedUsername))",
		"writeFile(path.join(rootPath, \"AGENTS.md\")",
		"writeFile(path.join(rootPath, \"docs\", \"aurago-bridge.md\")",
		"writeFile(path.join(process.env.CUSTOMWARE_PATH, \"aurago_bridge.js\")",
		"writeFile(path.join(rootPath, \"aurago_bridge.js\")",
		"aurago_inbox",
		"clearInvalidatedUserCrypto(normalizedUsername)",
		"createUser(projectRoot, username, password",
		"setUserPassword(projectRoot, username, password",
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("bootstrap script missing %q:\n%s", want, script)
		}
	}
}

func TestEnsureSpaceAgentHomeRefreshesManagedBridgeGuidance(t *testing.T) {
	home := t.TempDir()
	for _, dir := range []string{
		filepath.Join(home, "conf"),
		filepath.Join(home, "docs"),
	} {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			t.Fatalf("MkdirAll(%s): %v", dir, err)
		}
	}
	staleFiles := []string{
		filepath.Join(home, "AGENTS.md"),
		filepath.Join(home, "conf", "aurago.system.include.md"),
		filepath.Join(home, "docs", "aurago-bridge.md"),
	}
	for _, path := range staleFiles {
		if err := os.WriteFile(path, []byte("stale bridge is broken\n"), 0o600); err != nil {
			t.Fatalf("WriteFile(%s): %v", path, err)
		}
	}

	if err := ensureSpaceAgentHome(home); err != nil {
		t.Fatalf("ensureSpaceAgentHome() error = %v", err)
	}

	for _, path := range staleFiles {
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("ReadFile(%s): %v", path, err)
		}
		text := string(content)
		if strings.Contains(text, "stale bridge is broken") {
			t.Fatalf("%s was not refreshed: %q", path, text)
		}
		if !strings.Contains(text, "Memory") && !strings.Contains(text, "memory") {
			t.Fatalf("%s missing memory freshness guidance: %q", path, text)
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
		filepath.Join(home, "conf"),
		filepath.Join(home, "hist"),
		filepath.Join(home, "docs"),
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
		filepath.Join(home, "conf", "dashboard.yaml"):                "{}",
		filepath.Join(home, "conf", "onscreen-agent.yaml"):           "{}",
		filepath.Join(home, "hist", "onscreen-agent.json"):           "[]",
		filepath.Join(home, "docs", "aurago-bridge.md"):              "contains:AuraGo Bridge",
		filepath.Join(home, "conf", "aurago.system.include.md"):      "contains:latest_instruction.json",
		filepath.Join(home, "onscreen-agent", "config.json"):         "{}",
		filepath.Join(home, "onscreen-agent", "history.json"):        "[]",
		filepath.Join(home, "meta", "dashboard-prefs.json"):          "{}",
		filepath.Join(home, ".config", "onscreen-agent-config.json"): "{}",
	} {
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("ReadFile(%s) error = %v", path, err)
		}
		if strings.HasPrefix(want, "contains:") {
			if !strings.Contains(string(content), strings.TrimPrefix(want, "contains:")) {
				t.Fatalf("%s = %q, want content containing %s", path, string(content), strings.TrimPrefix(want, "contains:"))
			}
			continue
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
		filepath.Join(userHome, "conf", "dashboard.yaml"):            "{}",
		filepath.Join(userHome, "conf", "onscreen-agent.yaml"):       "{}",
		filepath.Join(userHome, "hist", "onscreen-agent.json"):       "[]",
		filepath.Join(userHome, "docs", "aurago-bridge.md"):          "contains:AuraGo Bridge",
		filepath.Join(userHome, "conf", "aurago.system.include.md"):  "contains:latest_instruction.json",
		filepath.Join(userHome, ".config", "dashboard-prefs.json"):   "{}",
		filepath.Join(userHome, "onscreen-agent", "config.json"):     "{}",
		filepath.Join(userHome, "onscreen-agent", "history.json"):    "[]",
		filepath.Join(userHome, "dashboard", "dashboard-prefs.json"): "{}",
	} {
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("ReadFile(%s) error = %v", path, err)
		}
		if strings.HasPrefix(want, "contains:") {
			if !strings.Contains(string(content), strings.TrimPrefix(want, "contains:")) {
				t.Fatalf("%s = %q, want content containing %s", path, string(content), strings.TrimPrefix(want, "contains:"))
			}
			continue
		}
		if strings.TrimSpace(string(content)) != want {
			t.Fatalf("%s = %q, want %s", path, string(content), want)
		}
	}
}

func TestWriteSpaceAgentBridgeCustomwareSeedsRootAndUserHelpers(t *testing.T) {
	customware := t.TempDir()
	if err := writeSpaceAgentBridgeCustomware(customware, "admin", "https://aurago.example/api/space-agent/bridge/messages", "bridge-secret"); err != nil {
		t.Fatalf("writeSpaceAgentBridgeCustomware() error = %v", err)
	}

	for _, path := range []string{
		filepath.Join(customware, "aurago_bridge.js"),
		filepath.Join(customware, "aurago_bridge.cjs"),
		filepath.Join(customware, "aurago_bridge_config.json"),
		filepath.Join(customware, "aurago_bridge.md"),
		filepath.Join(customware, "L2", "admin", "aurago_bridge.js"),
		filepath.Join(customware, "L2", "admin", "aurago_bridge.cjs"),
		filepath.Join(customware, "L2", "admin", "aurago_bridge_config.json"),
		filepath.Join(customware, "L2", "admin", "aurago_bridge.md"),
	} {
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("ReadFile(%s) error = %v", path, err)
		}
		text := string(content)
		if !strings.Contains(text, "sendToAuraGo") {
			if strings.HasSuffix(path, ".json") && strings.Contains(text, "bridge-secret") {
				continue
			}
			t.Fatalf("%s does not contain bridge helper content: %q", path, text)
		}
	}
}

func TestWriteSpaceAgentBridgeCustomwareTreatsUserHomeAsBestEffort(t *testing.T) {
	customware := t.TempDir()
	if err := os.MkdirAll(filepath.Join(customware, "L2", "admin"), 0o750); err != nil {
		t.Fatalf("MkdirAll(user dir): %v", err)
	}
	if err := os.WriteFile(filepath.Join(customware, "L2", "admin", "aurago_bridge.js"), []byte("stale"), 0o400); err != nil {
		t.Fatalf("WriteFile(stale user helper): %v", err)
	}

	if err := writeSpaceAgentBridgeCustomware(customware, "admin", "https://aurago.example/api/space-agent/bridge/messages", "bridge-secret"); err != nil {
		t.Fatalf("writeSpaceAgentBridgeCustomware() error = %v", err)
	}

	rootHelper, err := os.ReadFile(filepath.Join(customware, "aurago_bridge.js"))
	if err != nil {
		t.Fatalf("ReadFile(root helper): %v", err)
	}
	if !strings.Contains(string(rootHelper), "sendToAuraGo") {
		t.Fatalf("root helper was not written: %q", string(rootHelper))
	}
}

func TestSpaceAgentBridgeESMWorksInBrowserContext(t *testing.T) {
	helper := spaceAgentBridgeHelperESM("https://aurago.example/api/space-agent/bridge/messages", "bridge-secret")
	for _, want := range []string{
		"const EMBEDDED_BRIDGE_URL = \"https://aurago.example/api/space-agent/bridge/messages\";",
		"const EMBEDDED_BRIDGE_TOKEN = \"bridge-secret\";",
		"typeof process !== \"undefined\"",
		"options.bridgeUrl",
		"globalThis[name]",
		"deriveBrowserAuraGoBridgeURL",
		"isLoopbackBridgeURL",
		"bridgeUrlCandidates(options)",
		"-space-agent",
		"bridgeUrlCandidates",
		"export async function sendToAuraGo(message = {}, options = {})",
	} {
		if !strings.Contains(helper, want) {
			t.Fatalf("ESM bridge helper missing %q:\n%s", want, helper)
		}
	}
	if strings.Contains(helper, "const bridgeUrl = process.env.AURAGO_BRIDGE_URL") {
		t.Fatalf("ESM bridge helper still directly dereferences process.env:\n%s", helper)
	}
}

func TestSpaceAgentBridgeHelperFiltersLoopbackURLsInBrowserContext(t *testing.T) {
	helper := spaceAgentBridgeHelperESM("http://127.0.0.1:18080/api/space-agent/bridge/messages", "bridge-secret")
	for _, want := range []string{
		"typeof window === \"undefined\"",
		"return candidates.filter((candidate) => !isLoopbackBridgeURL(candidate));",
		"host === \"127.0.0.1\"",
		"host.startsWith(\"127.\")",
	} {
		if !strings.Contains(helper, want) {
			t.Fatalf("ESM bridge helper missing %q:\n%s", want, helper)
		}
	}
}

func TestSpaceAgentBridgeConfigOmitsLoopbackURLForBrowserRuntimes(t *testing.T) {
	cfgJSON := spaceAgentBridgeConfigJSON("http://127.0.0.1:18080/api/space-agent/bridge/messages", "bridge-secret")
	if strings.Contains(cfgJSON, "127.0.0.1") {
		t.Fatalf("browser bridge config leaked loopback URL: %s", cfgJSON)
	}
	if !strings.Contains(cfgJSON, "bridge-secret") || !strings.Contains(cfgJSON, "browser_bridge_url_strategy") {
		t.Fatalf("bridge config missing expected browser guidance: %s", cfgJSON)
	}
}

func TestSpaceAgentBridgeReadmeDocumentsImportableHelper(t *testing.T) {
	readme := spaceAgentAuraGoBridgeReadme()
	for _, want := range []string{
		"file:///app/customware/aurago_bridge.js",
		"sendToAuraGo",
		"Fast path",
		"response.answer",
		"do not wait for a second callback",
		"Browser-style Space Agent code often cannot access process.env",
		"AuraGo seeds both locations",
	} {
		if !strings.Contains(readme, want) {
			t.Fatalf("bridge readme missing %q:\n%s", want, readme)
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
