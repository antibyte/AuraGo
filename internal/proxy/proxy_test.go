package proxy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"testing"

	"aurago/internal/config"
	"aurago/internal/dockerutil"
	"aurago/internal/tools"
)

// fakeEngine records Docker Engine API calls and answers them from handle.
type fakeEngine struct {
	mu     sync.Mutex
	calls  []string
	bodies map[string]string
	builds []string
	handle func(method, endpoint, body string) ([]byte, int, error)
	build  func(image string, dockerfile []byte) error
}

func (f *fakeEngine) record(method, endpoint, body string) ([]byte, int, error) {
	f.mu.Lock()
	f.calls = append(f.calls, method+" "+endpoint)
	if f.bodies == nil {
		f.bodies = map[string]string{}
	}
	f.bodies[method+" "+endpoint] = body
	handle := f.handle
	f.mu.Unlock()
	if handle == nil {
		return nil, 404, nil
	}
	return handle(method, endpoint, body)
}

func (f *fakeEngine) engine() engine {
	return engine{
		ping: func(string) error { return nil },
		request: func(_ tools.DockerConfig, method, endpoint, body string) ([]byte, int, error) {
			return f.record(method, endpoint, body)
		},
		requestContext: func(_ context.Context, _ tools.DockerConfig, method, endpoint, body string) ([]byte, int, error) {
			return f.record(method, endpoint, body)
		},
		build: func(_ context.Context, _ tools.DockerConfig, image, _ string, dockerfile []byte, _ map[string]string, _ *slog.Logger) error {
			f.mu.Lock()
			f.builds = append(f.builds, image)
			build := f.build
			f.mu.Unlock()
			if build != nil {
				return build(image, dockerfile)
			}
			return nil
		},
	}
}

func (f *fakeEngine) called(prefix string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, call := range f.calls {
		if strings.HasPrefix(call, prefix) {
			return true
		}
	}
	return false
}

func (f *fakeEngine) body(call string) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.bodies[call]
}

// runningEngine answers like an engine where both proxy images exist and the
// container starts and keeps running.
func runningEngine(image string) func(method, endpoint, body string) ([]byte, int, error) {
	return func(method, endpoint, body string) ([]byte, int, error) {
		switch {
		case method == "GET" && strings.HasPrefix(endpoint, "/images/"):
			return []byte(`{}`), 200, nil
		case method == "POST" && strings.HasPrefix(endpoint, "/containers/create"):
			return []byte(`{"Id":"c1"}`), 201, nil
		case method == "POST" && strings.HasSuffix(endpoint, "/start"):
			return nil, 204, nil
		case method == "GET" && strings.HasSuffix(endpoint, "/json"):
			return []byte(fmt.Sprintf(`{"RestartCount":0,"State":{"Status":"running","Running":true},"Config":{"Image":%q}}`, image)), 200, nil
		}
		return nil, 204, nil
	}
}

func testManager(t *testing.T, cfg *config.Config, fake *fakeEngine) *Manager {
	t.Helper()
	if cfg.Directories.DataDir == "" {
		cfg.Directories.DataDir = t.TempDir()
	}
	m := NewManager(cfg, slog.New(slog.NewTextHandler(ioDiscard{}, nil)))
	m.engine = fake.engine()
	m.inDocker = func() bool { return false }
	m.settle = 0
	return m
}

type ioDiscard struct{}

func (ioDiscard) Write(p []byte) (int, error) { return len(p), nil }

func proxyConfig() *config.Config {
	cfg := &config.Config{}
	cfg.Server.Port = 8088
	cfg.SecurityProxy.Enabled = true
	cfg.SecurityProxy.HTTPSPort = 443
	cfg.SecurityProxy.HTTPPort = 80
	cfg.SecurityProxy.RateLimiting.RequestsPerSecond = 10
	cfg.SecurityProxy.RateLimiting.Burst = 50
	return cfg
}

func decodeCreatePayload(t *testing.T, raw string) map[string]interface{} {
	t.Helper()
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		t.Fatalf("create payload is not JSON: %v\n%s", err, raw)
	}
	return payload
}

func TestNativePlacementKeepsHostBindMounts(t *testing.T) {
	dir := t.TempDir()
	cfg := proxyConfig()
	cfg.Server.Port = 9090

	got := nativePlacement(cfg, dir)
	want := []string{
		dockerutil.FormatBindMount(filepath.Join(dir, "Caddyfile"), "/etc/caddy/Caddyfile"),
		dockerutil.FormatBindMount(filepath.Join(dir, "caddy_data"), "/data"),
		dockerutil.FormatBindMount(filepath.Join(dir, "caddy_config"), "/config"),
	}
	if !reflect.DeepEqual(got.binds, want) {
		t.Fatalf("binds = %#v, want %#v", got.binds, want)
	}
	if got.mounts != nil || got.network != "" {
		t.Fatalf("native placement must not use mounts or networks: %#v", got)
	}
	wantUpstream := "host.docker.internal:9090"
	if runtime.GOOS == "linux" {
		wantUpstream = "172.17.0.1:9090"
	}
	if got.upstream != wantUpstream {
		t.Fatalf("upstream = %q, want %q", got.upstream, wantUpstream)
	}
}

func TestSecurityProxyCreatePayloadNativeIsUnchanged(t *testing.T) {
	binds := []string{
		"/srv/aurago/data/proxy/Caddyfile:/etc/caddy/Caddyfile",
		"/srv/aurago/data/proxy/caddy_data:/data",
		"/srv/aurago/data/proxy/caddy_config:/config",
	}
	payload := securityProxyCreatePayload(imageName, placement{binds: binds, upstream: "172.17.0.1:8088"}, 8443, 8080)

	if payload["Image"] != imageName {
		t.Fatalf("Image = %#v, want %q", payload["Image"], imageName)
	}
	for _, key := range []string{"Env", "User", "NetworkingConfig"} {
		if _, ok := payload[key]; ok {
			t.Fatalf("native payload must not set %s: %#v", key, payload[key])
		}
	}
	exposed, ok := payload["ExposedPorts"].(map[string]interface{})
	if !ok || len(exposed) != 2 || exposed["443/tcp"] == nil || exposed["80/tcp"] == nil {
		t.Fatalf("ExposedPorts = %#v", payload["ExposedPorts"])
	}
	hostConfig := payload["HostConfig"].(map[string]interface{})
	if got := hostConfig["Binds"]; !reflect.DeepEqual(got, binds) {
		t.Fatalf("Binds = %#v, want %#v", got, binds)
	}
	for _, key := range []string{"Mounts", "NetworkMode"} {
		if _, ok := hostConfig[key]; ok {
			t.Fatalf("native HostConfig must not set %s", key)
		}
	}
	wantPorts := map[string]interface{}{
		"443/tcp": []map[string]string{{"HostIp": "0.0.0.0", "HostPort": "8443"}},
		"80/tcp":  []map[string]string{{"HostIp": "0.0.0.0", "HostPort": "8080"}},
	}
	if got := hostConfig["PortBindings"]; !reflect.DeepEqual(got, wantPorts) {
		t.Fatalf("PortBindings = %#v, want %#v", got, wantPorts)
	}
	if got := hostConfig["RestartPolicy"]; !reflect.DeepEqual(got, map[string]string{"Name": "unless-stopped"}) {
		t.Fatalf("RestartPolicy = %#v", got)
	}
	if got := hostConfig["ExtraHosts"]; !reflect.DeepEqual(got, []string{"host.docker.internal:host-gateway"}) {
		t.Fatalf("ExtraHosts = %#v", got)
	}
}

func TestSecurityProxyCreatePayloadDockerJoinsAuraGoNetwork(t *testing.T) {
	mounts := []map[string]interface{}{
		{"Type": "volume", "Source": "aurago_aurago_data", "Target": "/etc/caddy", "VolumeOptions": map[string]interface{}{"Subpath": "proxy"}},
	}
	payload := securityProxyCreatePayload(imageName, placement{mounts: mounts, network: "aurago_default", upstream: "aurago:8088"}, 443, 80)

	hostConfig := payload["HostConfig"].(map[string]interface{})
	if _, ok := hostConfig["Binds"]; ok {
		t.Fatal("Docker placement must not bind AuraGo container paths as host paths")
	}
	if got := hostConfig["Mounts"]; !reflect.DeepEqual(got, mounts) {
		t.Fatalf("Mounts = %#v, want %#v", got, mounts)
	}
	if hostConfig["NetworkMode"] != "aurago_default" {
		t.Fatalf("NetworkMode = %#v, want aurago_default", hostConfig["NetworkMode"])
	}
	want := map[string]interface{}{"EndpointsConfig": map[string]interface{}{"aurago_default": map[string]interface{}{}}}
	if got := payload["NetworkingConfig"]; !reflect.DeepEqual(got, want) {
		t.Fatalf("NetworkingConfig = %#v, want %#v", got, want)
	}
}

func TestManagerStartUsesConfigFromUpdateConfig(t *testing.T) {
	startup := proxyConfig()
	fake := &fakeEngine{handle: runningEngine(imageName)}
	m := testManager(t, startup, fake)

	saved := proxyConfig()
	saved.Directories.DataDir = startup.Directories.DataDir
	saved.SecurityProxy.Domain = "aurago.example.com"
	saved.SecurityProxy.HTTPSPort = 8443
	m.UpdateConfig(saved)

	if got := m.Config(); got != saved {
		t.Fatalf("Config() = %p, want the updated config %p", got, saved)
	}
	if err := m.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	payload := decodeCreatePayload(t, fake.body("POST /containers/create?name="+containerName))
	ports := payload["HostConfig"].(map[string]interface{})["PortBindings"].(map[string]interface{})
	if got := ports["443/tcp"].([]interface{})[0].(map[string]interface{})["HostPort"]; got != "8443" {
		t.Fatalf("HTTPS host port = %#v, want the saved 8443", got)
	}
	caddyfile, err := os.ReadFile(filepath.Join(saved.Directories.DataDir, "proxy", "Caddyfile"))
	if err != nil {
		t.Fatalf("read Caddyfile: %v", err)
	}
	if !strings.Contains(string(caddyfile), "aurago.example.com {") {
		t.Fatalf("Caddyfile does not use the saved domain:\n%s", caddyfile)
	}
}

func TestManagerStartWritesPrivateCaddyfileWithVaultCredentials(t *testing.T) {
	cfg := proxyConfig()
	cfg.SecurityProxy.BasicAuth.Enabled = true
	cfg.SecurityProxy.BasicAuth.Username = "admin"
	cfg.SecurityProxy.BasicAuth.Password = "vault-password"
	fake := &fakeEngine{handle: runningEngine(imageName)}
	m := testManager(t, cfg, fake)

	if err := m.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	path := filepath.Join(cfg.Directories.DataDir, "proxy", "Caddyfile")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read Caddyfile: %v", err)
	}
	if !strings.Contains(string(data), "\t\t\"admin\" $2") {
		t.Fatalf("Caddyfile lacks the hashed Vault credentials:\n%s", data)
	}
	if strings.Contains(fake.body("POST /containers/create?name="+containerName), "PROXY_BASIC_AUTH") {
		t.Fatal("credentials must not travel through the container environment")
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("stat Caddyfile: %v", err)
		}
		if mode := info.Mode().Perm(); mode != 0o600 {
			t.Fatalf("Caddyfile mode = %o, want 600", mode)
		}
	}
}

func TestManagerStartRefusesBasicAuthWithoutCredentials(t *testing.T) {
	cfg := proxyConfig()
	cfg.SecurityProxy.BasicAuth.Enabled = true
	fake := &fakeEngine{handle: runningEngine(imageName)}
	m := testManager(t, cfg, fake)

	err := m.Start()
	if !errors.Is(err, ErrBasicAuthCredentialsMissing) {
		t.Fatalf("Start() error = %v, want ErrBasicAuthCredentialsMissing", err)
	}
	if fake.called("POST /containers/create") {
		t.Fatal("Start created a container although basic auth cannot be configured")
	}
}

func TestWriteCaddyfileTightensModeAndKeepsFileIdentity(t *testing.T) {
	path := filepath.Join(t.TempDir(), "Caddyfile")
	if err := os.WriteFile(path, []byte("old config that is longer than the new one\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	before, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}

	if err := writeCaddyfile(path, []byte("new\n")); err != nil {
		t.Fatalf("writeCaddyfile() error = %v", err)
	}
	after, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	// A single-file bind mount follows the inode; a rename would leave Caddy
	// reading the old file.
	if !os.SameFile(before, after) {
		t.Fatal("writeCaddyfile replaced the file instead of rewriting it in place")
	}
	data, _ := os.ReadFile(path)
	if string(data) != "new\n" {
		t.Fatalf("content = %q, want %q", data, "new\n")
	}
	if runtime.GOOS != "windows" && after.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %o, want 600", after.Mode().Perm())
	}
}

func TestManagerStartBuildsRateLimitImageWhenMissing(t *testing.T) {
	cfg := proxyConfig()
	cfg.SecurityProxy.RateLimiting.Enabled = true
	running := runningEngine(rateLimitImageName)
	fake := &fakeEngine{}
	fake.handle = func(method, endpoint, body string) ([]byte, int, error) {
		if method == "GET" && strings.HasPrefix(endpoint, "/images/") {
			if len(fake.builds) == 0 {
				return nil, 404, nil
			}
		}
		return running(method, endpoint, body)
	}
	var dockerfile string
	fake.build = func(image string, content []byte) error {
		dockerfile = string(content)
		return nil
	}
	m := testManager(t, cfg, fake)

	if err := m.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if len(fake.builds) != 1 || fake.builds[0] != rateLimitImageName {
		t.Fatalf("builds = %#v, want one build of %s", fake.builds, rateLimitImageName)
	}
	for _, want := range []string{
		"FROM caddy:" + caddyBuildVersion + "-builder AS builder",
		"xcaddy build --with " + rateLimitModule + "@" + rateLimitModuleVersion,
		"FROM caddy:" + caddyBuildVersion + "\n",
		"COPY --from=builder /usr/bin/caddy /usr/bin/caddy",
	} {
		if !strings.Contains(dockerfile, want) {
			t.Fatalf("Dockerfile lacks %q:\n%s", want, dockerfile)
		}
	}
	payload := decodeCreatePayload(t, fake.body("POST /containers/create?name="+containerName))
	if payload["Image"] != rateLimitImageName {
		t.Fatalf("Image = %#v, want %s", payload["Image"], rateLimitImageName)
	}
	if fake.called("POST /images/create") {
		t.Fatal("rate limiting must not pull the official image without the module")
	}
}

func TestManagerStartReportsRateLimitImageFailure(t *testing.T) {
	cfg := proxyConfig()
	cfg.SecurityProxy.RateLimiting.Enabled = true
	running := runningEngine(rateLimitImageName)
	fake := &fakeEngine{handle: func(method, endpoint, body string) ([]byte, int, error) {
		if method == "GET" && strings.HasPrefix(endpoint, "/images/") {
			return nil, 404, nil
		}
		return running(method, endpoint, body)
	}}
	fake.build = func(string, []byte) error { return errors.New("HTTP 403: build is disabled") }
	m := testManager(t, cfg, fake)

	err := m.Start()
	if !errors.Is(err, ErrRateLimitImageUnavailable) {
		t.Fatalf("Start() error = %v, want ErrRateLimitImageUnavailable", err)
	}
	if !strings.Contains(err.Error(), "build is disabled") {
		t.Fatalf("Start() error = %v, want the build failure reason", err)
	}
	if fake.called("POST /containers/create") {
		t.Fatal("Start created a container without the rate limit module")
	}
}

func TestManagerStartKeepsOfficialImageWithoutRateLimiting(t *testing.T) {
	cfg := proxyConfig()
	fake := &fakeEngine{handle: runningEngine(imageName)}
	m := testManager(t, cfg, fake)

	if err := m.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if len(fake.builds) != 0 {
		t.Fatalf("builds = %#v, want none without rate limiting", fake.builds)
	}
	payload := decodeCreatePayload(t, fake.body("POST /containers/create?name="+containerName))
	if payload["Image"] != imageName {
		t.Fatalf("Image = %#v, want %s", payload["Image"], imageName)
	}
}

func TestManagerStartReportsCaddyThatStopsImmediately(t *testing.T) {
	cfg := proxyConfig()
	running := runningEngine(imageName)
	fake := &fakeEngine{handle: func(method, endpoint, body string) ([]byte, int, error) {
		switch {
		case method == "GET" && endpoint == "/containers/"+containerName+"/json":
			return []byte(`{"RestartCount":2,"State":{"Status":"restarting","Running":true,"Restarting":true},"Config":{"Image":"aurago-proxy:latest"}}`), 200, nil
		case method == "GET" && strings.HasPrefix(endpoint, "/containers/"+containerName+"/logs"):
			return dockerLogFrame(`{"level":"error","msg":"adapting config using caddyfile: /etc/caddy/Caddyfile:12: unrecognized directive: oops"}` + "\n"), 200, nil
		}
		return running(method, endpoint, body)
	}}
	m := testManager(t, cfg, fake)

	err := m.Start()
	if !errors.Is(err, ErrCaddyExited) {
		t.Fatalf("Start() error = %v, want ErrCaddyExited", err)
	}
	if !strings.Contains(err.Error(), "unrecognized directive: oops") {
		t.Fatalf("Start() error = %v, want Caddy's reason", err)
	}
}

func strconvQuote(s string) string {
	quoted, _ := json.Marshal(s)
	return string(quoted)
}

func dockerLogFrame(line string) []byte {
	frame := []byte{2, 0, 0, 0, 0, 0, 0, 0}
	n := len(line)
	frame[4], frame[5], frame[6], frame[7] = byte(n>>24), byte(n>>16), byte(n>>8), byte(n)
	return append(frame, line...)
}

// reloadEngine answers like a running proxy container whose exec of
// `caddy reload` finishes with exitCode and output.
func reloadEngine(image string, exitCode int, output string) func(method, endpoint, body string) ([]byte, int, error) {
	running := runningEngine(image)
	return func(method, endpoint, body string) ([]byte, int, error) {
		switch {
		case method == "POST" && endpoint == "/containers/"+containerName+"/exec":
			return []byte(`{"Id":"exec1"}`), 201, nil
		case method == "POST" && endpoint == "/exec/exec1/start":
			return dockerLogFrame(output), 200, nil
		case method == "GET" && endpoint == "/exec/exec1/json":
			return []byte(fmt.Sprintf(`{"Running":false,"ExitCode":%d}`, exitCode)), 200, nil
		}
		return running(method, endpoint, body)
	}
}

func TestManagerReloadAppliesNewCaddyfile(t *testing.T) {
	cfg := proxyConfig()
	fake := &fakeEngine{handle: reloadEngine(imageName, 0, "")}
	m := testManager(t, cfg, fake)
	cfg.SecurityProxy.Domain = "aurago.example.com"

	if err := m.Reload(); err != nil {
		t.Fatalf("Reload() error = %v", err)
	}
	var exec struct {
		Cmd []string `json:"Cmd"`
	}
	if err := json.Unmarshal([]byte(fake.body("POST /containers/"+containerName+"/exec")), &exec); err != nil {
		t.Fatalf("exec payload: %v", err)
	}
	if want := []string{"caddy", "reload", "--config", "/etc/caddy/Caddyfile"}; !reflect.DeepEqual(exec.Cmd, want) {
		t.Fatalf("exec Cmd = %#v, want %#v", exec.Cmd, want)
	}
	if body := fake.body("POST /exec/exec1/start"); !strings.Contains(body, `"Detach":false`) {
		t.Fatalf("exec start body = %s, want an attached exec so the exit code is known", body)
	}
	data, _ := os.ReadFile(filepath.Join(cfg.Directories.DataDir, "proxy", "Caddyfile"))
	if !strings.Contains(string(data), "aurago.example.com {") {
		t.Fatalf("Caddyfile not updated:\n%s", data)
	}
	if fake.called("POST /containers/create") {
		t.Fatal("Reload recreated the container although the image is unchanged")
	}
}

func TestManagerReloadReportsRejectedConfigAndKeepsPreviousCaddyfile(t *testing.T) {
	cfg := proxyConfig()
	fake := &fakeEngine{handle: reloadEngine(imageName, 1, "Error: adapting config using caddyfile: Caddyfile:9: unrecognized directive: oops\n")}
	m := testManager(t, cfg, fake)
	path := filepath.Join(cfg.Directories.DataDir, "proxy", "Caddyfile")
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("previous working config\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	err := m.Reload()
	if !errors.Is(err, ErrConfigRejected) {
		t.Fatalf("Reload() error = %v, want ErrConfigRejected", err)
	}
	if !strings.Contains(err.Error(), "unrecognized directive: oops") {
		t.Fatalf("Reload() error = %v, want Caddy's reason", err)
	}
	data, _ := os.ReadFile(path)
	if string(data) != "previous working config\n" {
		t.Fatalf("Caddyfile = %q, want the previous config Caddy still runs", data)
	}
}

func TestManagerReloadRecreatesContainerWhenRateLimitingChanged(t *testing.T) {
	cfg := proxyConfig()
	cfg.SecurityProxy.RateLimiting.Enabled = true
	current := reloadEngine(imageName, 0, "")
	recreated := false
	fake := &fakeEngine{}
	fake.handle = func(method, endpoint, body string) ([]byte, int, error) {
		if method == "POST" && strings.HasPrefix(endpoint, "/containers/create") {
			recreated = true
		}
		if method == "GET" && endpoint == "/containers/"+containerName+"/json" && recreated {
			return runningEngine(rateLimitImageName)(method, endpoint, body)
		}
		return current(method, endpoint, body)
	}
	m := testManager(t, cfg, fake)

	if err := m.Reload(); err != nil {
		t.Fatalf("Reload() error = %v", err)
	}
	if !recreated {
		t.Fatal("Reload kept a container whose image lacks the rate limit module")
	}
	if fake.called("POST /containers/" + containerName + "/exec") {
		t.Fatal("Reload executed caddy reload in the outdated container")
	}
	payload := decodeCreatePayload(t, fake.body("POST /containers/create?name="+containerName))
	if payload["Image"] != rateLimitImageName {
		t.Fatalf("Image = %#v, want %s", payload["Image"], rateLimitImageName)
	}
}

func TestManagerReloadReportsMissingContainer(t *testing.T) {
	cfg := proxyConfig()
	fake := &fakeEngine{handle: func(method, endpoint, body string) ([]byte, int, error) {
		return []byte(`{"message":"No such container"}`), 404, nil
	}}
	m := testManager(t, cfg, fake)

	if err := m.Reload(); !errors.Is(err, ErrNotRunning) {
		t.Fatalf("Reload() error = %v, want ErrNotRunning", err)
	}
}

func TestManagerFallsBackToNativePlacementWhenEngineDoesNotKnowAuraGo(t *testing.T) {
	cfg := proxyConfig()
	running := runningEngine(imageName)
	fake := &fakeEngine{handle: func(method, endpoint, body string) ([]byte, int, error) {
		if method == "GET" && endpoint == "/containers/lxc-host/json" {
			return []byte(`{"message":"No such container: lxc-host"}`), 404, nil
		}
		return running(method, endpoint, body)
	}}
	m := testManager(t, cfg, fake)
	m.inDocker = func() bool { return true }
	m.selfIDs = func() []string { return []string{"lxc-host"} }

	if err := m.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	payload := decodeCreatePayload(t, fake.body("POST /containers/create?name="+containerName))
	hostConfig := payload["HostConfig"].(map[string]interface{})
	if _, ok := hostConfig["Binds"]; !ok {
		t.Fatalf("HostConfig = %#v, want native binds when the engine has no AuraGo container", hostConfig)
	}
}

func TestManagerStartInComposeSharesVolumeAndNetwork(t *testing.T) {
	cfg := proxyConfig()
	// The data directory stands in for /app/data inside the AuraGo container.
	cfg.Directories.DataDir = t.TempDir()
	inspection := strings.ReplaceAll(composeSelfInspection, `"/app/data"`, strconvQuote(filepath.ToSlash(cfg.Directories.DataDir)))
	running := runningEngine(imageName)
	fake := &fakeEngine{handle: func(method, endpoint, body string) ([]byte, int, error) {
		switch {
		case method == "GET" && endpoint == "/containers/4f1c0ffee/json":
			return []byte(inspection), 200, nil
		case method == "GET" && endpoint == "/version":
			return []byte(`{"ApiVersion":"1.47"}`), 200, nil
		case method == "GET" && strings.HasPrefix(endpoint, "/networks/"):
			internal := endpoint != "/networks/net-default"
			return []byte(fmt.Sprintf(`{"Internal":%t}`, internal)), 200, nil
		}
		return running(method, endpoint, body)
	}}
	m := testManager(t, cfg, fake)
	m.inDocker = func() bool { return true }
	m.selfIDs = func() []string { return []string{"4f1c0ffee"} }

	if err := m.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	payload := decodeCreatePayload(t, fake.body("POST /containers/create?name="+containerName))
	hostConfig := payload["HostConfig"].(map[string]interface{})
	if hostConfig["NetworkMode"] != "aurago_default" {
		t.Fatalf("NetworkMode = %#v, want aurago_default", hostConfig["NetworkMode"])
	}
	if _, ok := hostConfig["Binds"]; ok {
		t.Fatalf("compose deployment must not bind container paths: %#v", hostConfig["Binds"])
	}
	mounts, _ := hostConfig["Mounts"].([]interface{})
	if len(mounts) != 3 {
		t.Fatalf("Mounts = %#v, want the three proxy paths from the data volume", hostConfig["Mounts"])
	}
	caddyfile, err := os.ReadFile(filepath.Join(cfg.Directories.DataDir, "proxy", "Caddyfile"))
	if err != nil {
		t.Fatalf("read Caddyfile: %v", err)
	}
	if !strings.Contains(string(caddyfile), "reverse_proxy aurago:8088 {") {
		t.Fatalf("Caddyfile upstream is not the AuraGo container:\n%s", caddyfile)
	}
}
