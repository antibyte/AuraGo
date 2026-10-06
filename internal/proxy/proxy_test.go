package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
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
	// trusted holds the trusted binds of each createTrusted call.
	trusted map[string][]string
	builds  []string
	pulls   []string
	handle  func(method, endpoint, body string) ([]byte, int, error)
	build   func(image string, dockerfile []byte) error
	pull    func(image string) error
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
		createTrusted: func(_ context.Context, _ tools.DockerConfig, endpoint, body string, trusted []string) ([]byte, int, error) {
			f.mu.Lock()
			if f.trusted == nil {
				f.trusted = map[string][]string{}
			}
			f.trusted["POST "+endpoint] = trusted
			f.mu.Unlock()
			return f.record("POST", endpoint, body)
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
		pull: func(_ context.Context, _ tools.DockerConfig, image string, _ *slog.Logger) error {
			f.mu.Lock()
			// Recorded like the Engine request it stands for.
			f.calls = append(f.calls, "POST /images/create?fromImage="+url.QueryEscape(image))
			f.pulls = append(f.pulls, image)
			pull := f.pull
			f.mu.Unlock()
			if pull != nil {
				return pull(image)
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

// TestSecurityProxyCreatePayloadIsHardened: Caddy keeps root and a writable
// root filesystem but only the capabilities it uses, in every placement (K20).
func TestSecurityProxyCreatePayloadIsHardened(t *testing.T) {
	binds := []string{
		"/srv/aurago/data/proxy/Caddyfile:/etc/caddy/Caddyfile",
		"/srv/aurago/data/proxy/caddy_data:/data",
		"/srv/aurago/data/proxy/caddy_config:/config",
	}
	mounts := []map[string]interface{}{
		{"Type": "volume", "Source": "aurago_aurago_data", "Target": "/etc/caddy", "VolumeOptions": map[string]interface{}{"Subpath": "proxy"}},
	}
	for name, place := range map[string]placement{
		"native":           {binds: binds, upstream: "172.17.0.1:8088"},
		"AuraGo in Docker": {mounts: mounts, network: "aurago_default", upstream: "aurago:8088"},
	} {
		t.Run(name, func(t *testing.T) {
			for _, image := range []string{imageName, rateLimitImageName} {
				payload := securityProxyCreatePayload(image, place, 8443, 8080)
				if _, ok := payload["User"]; ok {
					t.Fatal("User must stay unset: certificates from earlier root containers are root-owned 0600")
				}
				hostConfig := payload["HostConfig"].(map[string]interface{})
				if got := hostConfig["SecurityOpt"]; !reflect.DeepEqual(got, []string{"no-new-privileges:true"}) {
					t.Fatalf("SecurityOpt = %#v", got)
				}
				if got := hostConfig["CapDrop"]; !reflect.DeepEqual(got, []string{"ALL"}) {
					t.Fatalf("CapDrop = %#v", got)
				}
				if got := hostConfig["CapAdd"]; !reflect.DeepEqual(got, []string{"NET_BIND_SERVICE", "DAC_OVERRIDE", "CHOWN", "FOWNER"}) {
					t.Fatalf("CapAdd = %#v", got)
				}
				for _, key := range []string{"ReadonlyRootfs", "Privileged"} {
					if _, ok := hostConfig[key]; ok {
						t.Fatalf("%s must stay unset", key)
					}
				}
			}
		})
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
	before := statWithFileID(t, path)

	if err := writeCaddyfile(path, []byte("new\n")); err != nil {
		t.Fatalf("writeCaddyfile() error = %v", err)
	}
	after := statWithFileID(t, path)
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
	m.selfIDs = func() ([]string, string) { return selfContainerIDsFrom(noProcFiles, hostnameIs("lxc-host")) }

	if err := m.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	payload := decodeCreatePayload(t, fake.body("POST /containers/create?name="+containerName))
	hostConfig := payload["HostConfig"].(map[string]interface{})
	if _, ok := hostConfig["Binds"]; !ok {
		t.Fatalf("HostConfig = %#v, want native binds when the engine has no AuraGo container", hostConfig)
	}
	// As before M1: the custom hostname is looked up once and answers 404.
	if got := countCalls(fake, "GET /containers/lxc-host/json"); got != 1 {
		t.Fatalf("inspect of the hostname = %d, want 1", got)
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
	m.selfIDs = func() ([]string, string) { return []string{"4f1c0ffee"}, "" }

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

// writeRunningCaddyfile stands in for the Caddyfile the running proxy loaded.
func writeRunningCaddyfile(t *testing.T, cfg *config.Config) (string, []byte) {
	t.Helper()
	path := filepath.Join(cfg.Directories.DataDir, "proxy", "Caddyfile")
	previous := []byte("previous working config\n")
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, previous, 0o600); err != nil {
		t.Fatal(err)
	}
	return path, previous
}

// assertRunningProxyUntouched fails when the Caddyfile on disk changed or a
// request stopped, removed, replaced or reloaded the running container.
func assertRunningProxyUntouched(t *testing.T, fake *fakeEngine, path string, previous []byte) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read Caddyfile: %v", err)
	}
	if !bytes.Equal(data, previous) {
		t.Fatalf("Caddyfile = %q, want the unchanged %q the running proxy loaded", data, previous)
	}
	for _, call := range []string{
		"POST /containers/" + containerName + "/stop",
		"DELETE /containers/" + containerName,
		"POST /containers/create",
		"POST /containers/" + containerName + "/start",
		"POST /containers/" + containerName + "/exec",
	} {
		if fake.called(call) {
			t.Fatalf("%s reached the engine although the image is unavailable", call)
		}
	}
}

// imageFailureCases make ensureImage fail: the rate-limit build, or the pull
// of the official image.
var imageFailureCases = []struct {
	name      string
	rateLimit bool
	build     func(string, []byte) error
	pull      func(string) error
	want      error
	reason    string
}{
	{
		name:      "rate-limit build fails",
		rateLimit: true,
		build:     func(string, []byte) error { return errors.New("HTTP 403: build is disabled") },
		want:      ErrRateLimitImageUnavailable,
		reason:    "build is disabled",
	},
	{
		name:   "official image pull fails",
		pull:   func(string) error { return errors.New("failed to register layer: no space left on device") },
		reason: "no space left on device",
	},
}

func TestManagerStartKeepsRunningProxyWhenImageUnavailable(t *testing.T) {
	for _, tc := range imageFailureCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := proxyConfig()
			cfg.SecurityProxy.RateLimiting.Enabled = tc.rateLimit
			// The running container uses the other image, so Start must make
			// the new one available first.
			runningImage := rateLimitImageName
			if tc.rateLimit {
				runningImage = imageName
			}
			running := runningEngine(runningImage)
			fake := &fakeEngine{build: tc.build, pull: tc.pull}
			fake.handle = func(method, endpoint, body string) ([]byte, int, error) {
				if method == "GET" && strings.HasPrefix(endpoint, "/images/") {
					return nil, 404, nil
				}
				return running(method, endpoint, body)
			}
			m := testManager(t, cfg, fake)
			path, previous := writeRunningCaddyfile(t, cfg)

			err := m.Start()
			if err == nil || !strings.Contains(err.Error(), tc.reason) {
				t.Fatalf("Start() error = %v, want the image failure %q", err, tc.reason)
			}
			if tc.want != nil && !errors.Is(err, tc.want) {
				t.Fatalf("Start() error = %v, want %v", err, tc.want)
			}
			assertRunningProxyUntouched(t, fake, path, previous)
		})
	}
}

func TestManagerReloadKeepsRunningProxyWhenNewImageUnavailable(t *testing.T) {
	for _, tc := range imageFailureCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := proxyConfig()
			cfg.SecurityProxy.RateLimiting.Enabled = tc.rateLimit
			// Toggling rate limiting changes the image, so Reload takes the
			// recreate path through startLocked.
			runningImage := rateLimitImageName
			if tc.rateLimit {
				runningImage = imageName
			}
			current := reloadEngine(runningImage, 0, "")
			fake := &fakeEngine{build: tc.build, pull: tc.pull}
			fake.handle = func(method, endpoint, body string) ([]byte, int, error) {
				if method == "GET" && strings.HasPrefix(endpoint, "/images/") {
					return nil, 404, nil
				}
				return current(method, endpoint, body)
			}
			m := testManager(t, cfg, fake)
			path, previous := writeRunningCaddyfile(t, cfg)

			err := m.Reload()
			if err == nil || !strings.Contains(err.Error(), tc.reason) {
				t.Fatalf("Reload() error = %v, want the image failure %q", err, tc.reason)
			}
			if tc.want != nil && !errors.Is(err, tc.want) {
				t.Fatalf("Reload() error = %v, want %v", err, tc.want)
			}
			assertRunningProxyUntouched(t, fake, path, previous)
		})
	}
}

// rootInstallBinds are the binds nativePlacement builds on Linux for the
// default install.sh directory ($HOME/aurago) of an install run as root.
var rootInstallBinds = []string{
	"/root/aurago/data/proxy/Caddyfile:/etc/caddy/Caddyfile",
	"/root/aurago/data/proxy/caddy_data:/data",
	"/root/aurago/data/proxy/caddy_config:/config",
}

// homeInstallBinds are the binds of an install outside the sensitive host
// paths, which could always start the proxy.
var homeInstallBinds = []string{
	"/home/user/aurago/data/proxy/Caddyfile:/etc/caddy/Caddyfile",
	"/home/user/aurago/data/proxy/caddy_data:/data",
	"/home/user/aurago/data/proxy/caddy_config:/config",
}

// nativeHomeCreateBody is the create body of the /home/user/aurago install:
// the bytes the proxy sent before the trusted-bind routing (PX3), plus the K20
// hardening (CapAdd, CapDrop, SecurityOpt) and the I5 managed labels.
const nativeHomeCreateBody = `{"ExposedPorts":{"443/tcp":{},"80/tcp":{}},"HostConfig":{"Binds":["/home/user/aurago/data/proxy/Caddyfile:/etc/caddy/Caddyfile","/home/user/aurago/data/proxy/caddy_data:/data","/home/user/aurago/data/proxy/caddy_config:/config"],"CapAdd":["NET_BIND_SERVICE","DAC_OVERRIDE","CHOWN","FOWNER"],"CapDrop":["ALL"],"ExtraHosts":["host.docker.internal:host-gateway"],"PortBindings":{"443/tcp":[{"HostIp":"0.0.0.0","HostPort":"443"}],"80/tcp":[{"HostIp":"0.0.0.0","HostPort":"80"}]},"RestartPolicy":{"Name":"unless-stopped"},"SecurityOpt":["no-new-privileges:true"]},"Image":"aurago-proxy:latest","Labels":{"aurago.component":"caddy","aurago.managed":"security-proxy","aurago.role":"proxy"}}`

// composeCreateBody is the create body of the default docker-compose.yml
// deployment: the bytes before PX3, plus the K20 hardening and the I5 labels.
const composeCreateBody = `{"ExposedPorts":{"443/tcp":{},"80/tcp":{}},"HostConfig":{"CapAdd":["NET_BIND_SERVICE","DAC_OVERRIDE","CHOWN","FOWNER"],"CapDrop":["ALL"],"ExtraHosts":["host.docker.internal:host-gateway"],"Mounts":[{"ReadOnly":true,"Source":"aurago_aurago_data","Target":"/etc/caddy","Type":"volume","VolumeOptions":{"Subpath":"proxy"}},{"Source":"aurago_aurago_data","Target":"/data","Type":"volume","VolumeOptions":{"Subpath":"proxy/caddy_data"}},{"Source":"aurago_aurago_data","Target":"/config","Type":"volume","VolumeOptions":{"Subpath":"proxy/caddy_config"}}],"NetworkMode":"aurago_default","PortBindings":{"443/tcp":[{"HostIp":"0.0.0.0","HostPort":"443"}],"80/tcp":[{"HostIp":"0.0.0.0","HostPort":"80"}]},"RestartPolicy":{"Name":"unless-stopped"},"SecurityOpt":["no-new-privileges:true"]},"Image":"aurago-proxy:latest","Labels":{"aurago.component":"caddy","aurago.managed":"security-proxy","aurago.role":"proxy"},"NetworkingConfig":{"EndpointsConfig":{"aurago_default":{}}}}`

func TestInstallBindFixturesMatchNativePlacement(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("nativePlacement makes these directories drive-absolute on Windows")
	}
	for dir, want := range map[string][]string{
		"/root/aurago/data/proxy":      rootInstallBinds,
		"/home/user/aurago/data/proxy": homeInstallBinds,
	} {
		if got := nativePlacement(proxyConfig(), dir).binds; !reflect.DeepEqual(got, want) {
			t.Fatalf("nativePlacement(%q).binds = %#v, want %#v", dir, got, want)
		}
	}
}

// proxyDaemon is a fake Docker daemon behind the production engine wiring:
// the proxy image exists, stop and remove of the proxy container answer 404,
// inspect always reports the proxy container as running (an old one before
// the create, the new one after the start), and create and start succeed. It
// records every request and every create body.
type proxyDaemon struct {
	host     string
	mu       sync.Mutex
	requests []string
	creates  []daemonCreate
}

type daemonCreate struct {
	query       string
	contentType string
	body        string
}

func newProxyDaemon(t *testing.T, image string) *proxyDaemon {
	t.Helper()
	d := &proxyDaemon{}
	proxyPath := "/containers/" + containerName
	d.host = fakeDockerDaemon(t, func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		d.mu.Lock()
		d.requests = append(d.requests, r.Method+" "+path)
		d.mu.Unlock()
		switch {
		case r.Method == http.MethodGet && strings.HasSuffix(path, "/_ping"):
			_, _ = w.Write([]byte("OK"))
		case r.Method == http.MethodGet && strings.HasSuffix(path, "/images/"+image+"/json"):
			_, _ = w.Write([]byte(`{}`))
		case r.Method == http.MethodPost && strings.HasSuffix(path, "/containers/create"):
			body, _ := io.ReadAll(r.Body)
			d.mu.Lock()
			d.creates = append(d.creates, daemonCreate{query: r.URL.RawQuery, contentType: r.Header.Get("Content-Type"), body: string(body)})
			d.mu.Unlock()
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"Id":"c1"}`))
		case r.Method == http.MethodPost && strings.HasSuffix(path, proxyPath+"/start"):
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodGet && strings.HasSuffix(path, proxyPath+"/json"):
			_, _ = fmt.Fprintf(w, `{"RestartCount":0,"State":{"Status":"running","Running":true},"Config":{"Image":%q}}`, image)
		case r.Method == http.MethodPost && strings.HasSuffix(path, proxyPath+"/stop"),
			r.Method == http.MethodDelete && strings.HasSuffix(path, proxyPath),
			r.Method == http.MethodGet && strings.HasSuffix(path, "/containers/lxc-host/json"):
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"message":"No such container"}`))
		default:
			t.Errorf("unexpected Docker request: %s %s", r.Method, r.URL.String())
			w.WriteHeader(http.StatusInternalServerError)
		}
	})
	return d
}

func (d *proxyDaemon) createRequests() []daemonCreate {
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]daemonCreate(nil), d.creates...)
}

// mutations lists the requests other than GET and HEAD.
func (d *proxyDaemon) mutations() []string {
	d.mu.Lock()
	defer d.mu.Unlock()
	var out []string
	for _, request := range d.requests {
		if !strings.HasPrefix(request, http.MethodGet+" ") && !strings.HasPrefix(request, http.MethodHead+" ") {
			out = append(out, request)
		}
	}
	return out
}

// realEngineManager returns a manager on the production engine wiring, so the
// tools Docker gates and the create bind policy apply. Its native placement
// carries binds; the Caddyfile still goes to the test's data directory. The
// returned func lists the trusted binds of every trusted create.
func realEngineManager(t *testing.T, cfg *config.Config, binds []string) (*Manager, func() [][]string) {
	t.Helper()
	m := testManager(t, cfg, &fakeEngine{})
	m.engine = dockerEngine
	var mu sync.Mutex
	var trusted [][]string
	create := dockerEngine.createTrusted
	m.engine.createTrusted = func(ctx context.Context, dockerCfg tools.DockerConfig, endpoint, body string, list []string) ([]byte, int, error) {
		mu.Lock()
		trusted = append(trusted, append([]string(nil), list...))
		mu.Unlock()
		return create(ctx, dockerCfg, endpoint, body, list)
	}
	m.native = func(cfg *config.Config, proxyDir string) placement {
		place := nativePlacement(cfg, proxyDir)
		place.binds = append([]string(nil), binds...)
		return place
	}
	return m, func() [][]string {
		mu.Lock()
		defer mu.Unlock()
		return append([][]string(nil), trusted...)
	}
}

func TestManagerStartNativeInstallUnderRootTrustsItsOwnBinds(t *testing.T) {
	for name, lxcGuest := range map[string]bool{
		"native install": false,
		// /.dockerenv, but the engine does not know AuraGo: host paths too.
		"LXC guest": true,
	} {
		t.Run(name, func(t *testing.T) {
			allowDockerForTest(t, false)
			daemon := newProxyDaemon(t, imageName)
			cfg := proxyConfig()
			cfg.SecurityProxy.DockerHost = daemon.host
			m, trusted := realEngineManager(t, cfg, rootInstallBinds)
			if lxcGuest {
				m.inDocker = func() bool { return true }
				m.selfIDs = func() ([]string, string) { return nil, "lxc-host" }
			}

			if err := m.Start(); err != nil {
				t.Fatalf("Start() error = %v, want the proxy of an install under /root to start", err)
			}
			creates := daemon.createRequests()
			if len(creates) != 1 {
				t.Fatalf("create requests = %d, want 1", len(creates))
			}
			payload := decodeCreatePayload(t, creates[0].body)
			hostConfig := payload["HostConfig"].(map[string]interface{})
			binds, _ := hostConfig["Binds"].([]interface{})
			got := make([]string, 0, len(binds))
			for _, bind := range binds {
				got = append(got, fmt.Sprint(bind))
			}
			if !reflect.DeepEqual(got, rootInstallBinds) {
				t.Fatalf("Binds = %#v, want the native binds %#v", got, rootInstallBinds)
			}
			if want := [][]string{rootInstallBinds}; !reflect.DeepEqual(trusted(), want) {
				t.Fatalf("trusted binds = %#v, want exactly the native binds %#v", trusted(), want)
			}
		})
	}
}

func TestSecurityProxyCreateStillRejectsForeignBinds(t *testing.T) {
	allowDockerForTest(t, false)
	daemon := newProxyDaemon(t, imageName)
	dockerCfg := tools.DockerConfig{Host: daemon.host}
	endpoint := "/containers/create?name=" + url.QueryEscape(containerName)
	body := func(binds ...string) string {
		data, err := json.Marshal(securityProxyCreatePayload(imageName, placement{binds: binds}, 443, 80))
		if err != nil {
			t.Fatalf("marshal create payload: %v", err)
		}
		return string(data)
	}
	withExtra := func(extra string) []string {
		return append(append([]string(nil), rootInstallBinds...), extra)
	}

	for name, binds := range map[string][]string{
		"extra bind under /root": withExtra("/root/.ssh:/ssh:ro"),
		"extra host /etc":        withExtra("/etc:/host-etc:ro"),
		"altered proxy bind":     {rootInstallBinds[0] + ":rw", rootInstallBinds[1], rootInstallBinds[2]},
	} {
		_, _, err := dockerEngine.createTrusted(context.Background(), dockerCfg, endpoint, body(binds...), rootInstallBinds)
		if err == nil || !strings.Contains(err.Error(), "mounting sensitive host path") {
			t.Fatalf("%s: error = %v, want the sensitive-path denial", name, err)
		}
	}
	// Without the trust list the proxy's binds meet the policy like any others.
	if _, _, err := dockerEngine.request(dockerCfg, http.MethodPost, endpoint, body(rootInstallBinds...)); err == nil || !strings.Contains(err.Error(), "mounting sensitive host path") {
		t.Fatalf("untrusted create error = %v, want the sensitive-path denial", err)
	}
	if creates := daemon.createRequests(); len(creates) != 0 {
		t.Fatalf("rejected creates reached Docker: %#v", creates)
	}
}

func TestManagerStartNativeHomeInstallCreateIsByteIdentical(t *testing.T) {
	allowDockerForTest(t, false)
	daemon := newProxyDaemon(t, imageName)
	cfg := proxyConfig()
	cfg.SecurityProxy.DockerHost = daemon.host
	m, _ := realEngineManager(t, cfg, homeInstallBinds)

	if err := m.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	creates := daemon.createRequests()
	if len(creates) != 1 {
		t.Fatalf("create requests = %d, want 1", len(creates))
	}
	create := creates[0]
	if create.query != "name="+containerName || create.contentType != "application/json" {
		t.Fatalf("create query = %q, content type = %q", create.query, create.contentType)
	}
	if create.body != nativeHomeCreateBody {
		t.Fatalf("create body changed:\n got %s\nwant %s", create.body, nativeHomeCreateBody)
	}
}

func TestManagerStartComposeCreateIsUnchanged(t *testing.T) {
	cfg := proxyConfig()
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
			return []byte(fmt.Sprintf(`{"Internal":%t}`, endpoint != "/networks/net-default")), 200, nil
		}
		return running(method, endpoint, body)
	}}
	m := testManager(t, cfg, fake)
	m.inDocker = func() bool { return true }
	m.selfIDs = func() ([]string, string) { return []string{"4f1c0ffee"}, "" }

	if err := m.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	call := "POST /containers/create?name=" + containerName
	if got := fake.body(call); got != composeCreateBody {
		t.Fatalf("compose create body changed:\n got %s\nwant %s", got, composeCreateBody)
	}
	fake.mu.Lock()
	trusted := fake.trusted[call]
	fake.mu.Unlock()
	if len(trusted) != 0 {
		t.Fatalf("compose create trusted binds = %#v, want none", trusted)
	}
}

// statWithFileID stats path and loads its file ID at once. On Windows
// os.Stat reads the volume serial number and file index lazily, by path, on
// the first os.SameFile: an identity check against a file that was replaced in
// between would compare the replacement with itself and always pass.
func statWithFileID(t *testing.T, path string) os.FileInfo {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	os.SameFile(info, info) // load the file ID now, see above
	return info
}

// runningCaddyfile writes the Caddyfile the running proxy loaded and returns
// its file identity.
func runningCaddyfile(t *testing.T, cfg *config.Config) (string, []byte, os.FileInfo) {
	t.Helper()
	path, previous := writeRunningCaddyfile(t, cfg)
	return path, previous, statWithFileID(t, path)
}

// assertCaddyfileRestored fails unless the Caddyfile holds previous again, in
// the same file (a native install bind-mounts the inode) with mode 0600.
func assertCaddyfileRestored(t *testing.T, path string, previous []byte, before os.FileInfo) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read Caddyfile: %v", err)
	}
	if !bytes.Equal(data, previous) {
		t.Fatalf("Caddyfile = %q, want the restored %q the old container loads", data, previous)
	}
	after := statWithFileID(t, path)
	if !os.SameFile(before, after) {
		t.Fatal("the Caddyfile was replaced instead of rewritten in place")
	}
	if runtime.GOOS != "windows" && after.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %o, want 600", after.Mode().Perm())
	}
}

// assertNotCalled fails when one of calls reached the engine.
func assertNotCalled(t *testing.T, fake *fakeEngine, calls ...string) {
	t.Helper()
	for _, call := range calls {
		if fake.called(call) {
			t.Fatalf("%s reached the engine", call)
		}
	}
}

func TestManagerStartRestoresCaddyfileWhenReadOnlyRefusesRemoval(t *testing.T) {
	// docker.read_only with an already built rate-limit image: Start gets
	// past ensureImage, but tools refuses to stop or remove the old proxy.
	allowDockerForTest(t, true)
	daemon := newProxyDaemon(t, rateLimitImageName)
	cfg := proxyConfig()
	cfg.Docker.ReadOnly = true
	cfg.SecurityProxy.RateLimiting.Enabled = true
	cfg.SecurityProxy.DockerHost = daemon.host
	m, _ := realEngineManager(t, cfg, homeInstallBinds)
	path, previous, before := runningCaddyfile(t, cfg)

	err := m.Start()
	if !errors.Is(err, tools.ErrDockerReadOnly) || !strings.Contains(err.Error(), "remove the old container") {
		t.Fatalf("Start() error = %v, want the read-only refusal of the removal", err)
	}
	assertCaddyfileRestored(t, path, previous, before)
	if got := daemon.mutations(); len(got) != 0 {
		t.Fatalf("Docker mutations reached the engine: %v", got)
	}
}

func TestManagerStartRestoresCaddyfileWhenOldContainerCannotBeRemoved(t *testing.T) {
	for name, deleteFails := range map[string]func() ([]byte, int, error){
		"engine refuses the removal": func() ([]byte, int, error) {
			return []byte(`{"message":"driver \"overlay2\" failed to remove root filesystem: device or resource busy"}`), 500, nil
		},
		"removal answer lost, container still there": func() ([]byte, int, error) {
			return nil, 0, errors.New("docker request failed: unexpected EOF")
		},
	} {
		t.Run(name, func(t *testing.T) {
			cfg := proxyConfig()
			running := runningEngine(imageName)
			fake := &fakeEngine{handle: func(method, endpoint, body string) ([]byte, int, error) {
				if method == "DELETE" {
					return deleteFails()
				}
				return running(method, endpoint, body)
			}}
			m := testManager(t, cfg, fake)
			path, previous, before := runningCaddyfile(t, cfg)

			if err := m.Start(); err == nil || !strings.Contains(err.Error(), "remove the old container") {
				t.Fatalf("Start() error = %v, want the failed removal", err)
			}
			if !fake.called("DELETE /containers/" + containerName) {
				t.Fatal("Start did not try to remove the old container")
			}
			assertCaddyfileRestored(t, path, previous, before)
			assertNotCalled(t, fake, "POST /containers/create", "POST /containers/"+containerName+"/start")
		})
	}
}

func TestManagerStartProceedsWhenRemovalAnswerIsLostButContainerIsGone(t *testing.T) {
	cfg := proxyConfig()
	running := runningEngine(imageName)
	var mu sync.Mutex
	removed, started := false, false
	fake := &fakeEngine{handle: func(method, endpoint, body string) ([]byte, int, error) {
		mu.Lock()
		defer mu.Unlock()
		switch {
		case method == "DELETE":
			removed = true
			return nil, 0, errors.New("docker request failed: unexpected EOF")
		case method == "GET" && endpoint == "/containers/"+containerName+"/json" && removed && !started:
			return []byte(`{"message":"No such container"}`), 404, nil
		case method == "POST" && strings.HasSuffix(endpoint, "/start"):
			started = true
		}
		return running(method, endpoint, body)
	}}
	m := testManager(t, cfg, fake)

	if err := m.Start(); err != nil {
		t.Fatalf("Start() error = %v, want the start to go on once the old container is gone", err)
	}
	if !fake.called("POST /containers/create") {
		t.Fatal("Start did not create the new container")
	}
}

func TestManagerStartRestoresCaddyfileWhenCreateFails(t *testing.T) {
	for name, createFails := range map[string]func() ([]byte, int, error){
		"engine refuses the create": func() ([]byte, int, error) {
			return []byte(`{"message":"Conflict. The container name \"/aurago-security-proxy\" is already in use"}`), 409, nil
		},
		"create transport error": func() ([]byte, int, error) {
			return nil, 0, errors.New("docker request failed: connection reset by peer")
		},
	} {
		t.Run(name, func(t *testing.T) {
			cfg := proxyConfig()
			running := runningEngine(imageName)
			fake := &fakeEngine{handle: func(method, endpoint, body string) ([]byte, int, error) {
				if method == "POST" && strings.HasPrefix(endpoint, "/containers/create") {
					return createFails()
				}
				return running(method, endpoint, body)
			}}
			m := testManager(t, cfg, fake)
			path, previous, before := runningCaddyfile(t, cfg)

			if err := m.Start(); err == nil || !strings.Contains(err.Error(), "create container") {
				t.Fatalf("Start() error = %v, want the failed create", err)
			}
			assertCaddyfileRestored(t, path, previous, before)
			assertNotCalled(t, fake, "POST /containers/"+containerName+"/start")
		})
	}
}

func TestManagerStartRefusesBasicAuthBeforeAnyImageWork(t *testing.T) {
	for name, rateLimit := range map[string]bool{
		"rate-limit image build": true,
		"official image pull":    false,
	} {
		t.Run(name, func(t *testing.T) {
			cfg := proxyConfig()
			cfg.SecurityProxy.RateLimiting.Enabled = rateLimit
			cfg.SecurityProxy.BasicAuth.Enabled = true
			fake := &fakeEngine{handle: missingImagesEngine(proxyImage(cfg))}
			m := testManager(t, cfg, fake)

			if err := m.Start(); !errors.Is(err, ErrBasicAuthCredentialsMissing) {
				t.Fatalf("Start() error = %v, want ErrBasicAuthCredentialsMissing", err)
			}
			fake.mu.Lock()
			defer fake.mu.Unlock()
			if len(fake.builds) != 0 || len(fake.pulls) != 0 {
				t.Fatalf("builds = %#v, pulls = %#v, want none before the credentials are usable", fake.builds, fake.pulls)
			}
			for _, call := range fake.calls {
				if !strings.HasPrefix(call, "GET ") {
					t.Fatalf("Docker mutation %q before the credentials are usable", call)
				}
			}
		})
	}
}

func TestManagerStartWritesCaddyfileBeforeCreate(t *testing.T) {
	cfg := proxyConfig()
	cfg.Directories.DataDir = t.TempDir()
	dir := filepath.Join(cfg.Directories.DataDir, "proxy")
	want, err := GenerateCaddyfile(cfg, nativePlacement(cfg, dir).upstream)
	if err != nil {
		t.Fatal(err)
	}
	running := runningEngine(imageName)
	created := false
	fake := &fakeEngine{handle: func(method, endpoint, body string) ([]byte, int, error) {
		if method == "POST" && strings.HasPrefix(endpoint, "/containers/create") {
			created = true
			data, err := os.ReadFile(filepath.Join(dir, "Caddyfile"))
			if err != nil || string(data) != want {
				t.Errorf("Caddyfile at create = %q (err %v), want the new content %q", data, err, want)
			}
		}
		return running(method, endpoint, body)
	}}
	m := testManager(t, cfg, fake)
	writeRunningCaddyfile(t, cfg)

	if err := m.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if !created {
		t.Fatal("Start did not create the container")
	}
}

// The proxy container carries AuraGo's managed labels, so the container API
// and the agent docker tool treat it as a protected AuraGo container (I5).
func TestSecurityProxyCreatePayloadCarriesManagedLabels(t *testing.T) {
	want := dockerutil.ManagedLabels(dockerutil.SecurityProxyOwner, "caddy", "proxy", "")
	for name, place := range map[string]placement{
		"native":           {binds: homeInstallBinds},
		"AuraGo in Docker": {mounts: []map[string]interface{}{{"Type": "volume", "Source": "aurago_aurago_data", "Target": "/etc/caddy"}}, network: "aurago_default"},
	} {
		payload := securityProxyCreatePayload(imageName, place, 443, 80)
		if got := payload["Labels"]; !reflect.DeepEqual(got, want) {
			t.Fatalf("%s: Labels = %#v, want %#v", name, got, want)
		}
	}
	if containerName != dockerutil.SecurityProxyContainerName {
		t.Fatalf("containerName = %q, want the reserved %q", containerName, dockerutil.SecurityProxyContainerName)
	}
}

// A proxy container that an older AuraGo created without labels is still
// found and managed by its name.
func TestManagerManagesUnlabeledProxyContainerByName(t *testing.T) {
	cfg := proxyConfig()
	fake := &fakeEngine{handle: reloadEngine(imageName, 0, "")}
	m := testManager(t, cfg, fake)

	status, err := m.Status()
	if err != nil || !status.Running {
		t.Fatalf("Status() = %+v, %v; want the unlabeled container running", status, err)
	}
	if err := m.Reload(); err != nil {
		t.Fatalf("Reload() error = %v", err)
	}
	if !fake.called("POST /containers/" + containerName + "/exec") {
		t.Fatal("Reload did not exec caddy reload in the container by name")
	}
	if err := m.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if !fake.called("DELETE /containers/" + containerName + "?force=true&v=true") {
		t.Fatal("Start did not remove the unlabeled container by name")
	}
	payload := decodeCreatePayload(t, fake.body("POST /containers/create?name="+containerName))
	if labels, _ := payload["Labels"].(map[string]interface{}); labels["aurago.managed"] != dockerutil.SecurityProxyOwner {
		t.Fatalf("recreated container Labels = %#v, want the managed labels", payload["Labels"])
	}
}

// TestTrustedNativeBindsSkipSymlinkedLeaves: a native bind is trusted only
// while its host-side leaf exists and is no symlink. Anything else meets the
// full create bind policy, as before PX3.
func TestTrustedNativeBindsSkipSymlinkedLeaves(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "Caddyfile"), []byte("{\n}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "caddy_data"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(t.TempDir(), filepath.Join(dir, "caddy_config")); err != nil {
		t.Skipf("symlinks are not available here: %v", err)
	}
	place := nativePlacement(proxyConfig(), dir)
	if got, want := trustedNativeBinds(place), place.binds[:2]; !reflect.DeepEqual(got, want) {
		t.Fatalf("trusted = %#v, want the two binds without the symlinked caddy_config %#v", got, want)
	}
	if err := os.Remove(filepath.Join(dir, "Caddyfile")); err != nil {
		t.Fatal(err)
	}
	if got, want := trustedNativeBinds(place), place.binds[1:2]; !reflect.DeepEqual(got, want) {
		t.Fatalf("trusted = %#v, want only caddy_data while the Caddyfile is missing", got)
	}
	if got := trustedNativeBinds(placement{mounts: []map[string]interface{}{{"Type": "volume"}}, network: "aurago_default"}); got != nil {
		t.Fatalf("Docker placement trusted = %#v, want nothing", got)
	}
	if got := trustedNativeBinds(placement{binds: place.binds}); got != nil {
		t.Fatalf("binds without sources trusted = %#v, want nothing", got)
	}
}

// TestManagerStartDoesNotTrustASymlinkedNativeBind: on an install under an
// always-allowed path, a caddy_config that links to /etc must not be mounted
// read-write through the trust list (PX3 review).
func TestManagerStartDoesNotTrustASymlinkedNativeBind(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("needs Linux host paths and a symlink to /etc")
	}
	allowDockerForTest(t, false)
	daemon := newProxyDaemon(t, imageName)
	cfg := proxyConfig()
	cfg.SecurityProxy.DockerHost = daemon.host
	m := testManager(t, cfg, &fakeEngine{})
	m.engine = dockerEngine
	dir := filepath.Join(cfg.Directories.DataDir, "proxy")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("/etc", filepath.Join(dir, "caddy_config")); err != nil {
		t.Fatal(err)
	}

	err := m.Start()
	if err == nil || !strings.Contains(err.Error(), `mounting sensitive host path "/etc"`) {
		t.Fatalf("Start() error = %v, want the sensitive-path denial for the symlinked caddy_config", err)
	}
	if creates := daemon.createRequests(); len(creates) != 0 {
		t.Fatalf("the create reached Docker: %#v", creates)
	}
}

// failingCaddyfileWrite stands in for a write that fails after writeCaddyfile
// truncated the file, e.g. on a full disk: half of the new content is left.
func failingCaddyfileWrite(path string, data []byte) error {
	f, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := f.Truncate(0); err != nil {
		return err
	}
	_, _ = f.Write(data[:len(data)/2])
	return errors.New("no space left on device")
}

func TestManagerStartRestoresCaddyfileWhenWriteFails(t *testing.T) {
	cfg := proxyConfig()
	fake := &fakeEngine{handle: runningEngine(imageName)}
	m := testManager(t, cfg, fake)
	m.writeConfig = failingCaddyfileWrite
	path, previous, before := runningCaddyfile(t, cfg)

	if err := m.Start(); err == nil || !strings.Contains(err.Error(), "no space left on device") {
		t.Fatalf("Start() error = %v, want the failed write", err)
	}
	assertCaddyfileRestored(t, path, previous, before)
	assertNotCalled(t, fake, "POST /containers/"+containerName+"/stop", "DELETE /containers/"+containerName, "POST /containers/create")
}

func TestManagerReloadRestoresCaddyfileWhenWriteFails(t *testing.T) {
	cfg := proxyConfig()
	fake := &fakeEngine{handle: reloadEngine(imageName, 0, "")}
	m := testManager(t, cfg, fake)
	m.writeConfig = failingCaddyfileWrite
	path, previous, before := runningCaddyfile(t, cfg)

	if err := m.Reload(); err == nil || !strings.Contains(err.Error(), "no space left on device") {
		t.Fatalf("Reload() error = %v, want the failed write", err)
	}
	assertCaddyfileRestored(t, path, previous, before)
	assertNotCalled(t, fake, "POST /containers/"+containerName+"/exec")
}

// Docker answers 409 while another removal of the container runs; once it has
// finished, inspect no longer knows the container and Start goes on.
func TestManagerStartProceedsWhenRemovalIsAlreadyInProgress(t *testing.T) {
	cfg := proxyConfig()
	running := runningEngine(imageName)
	var mu sync.Mutex
	removed, started := false, false
	fake := &fakeEngine{handle: func(method, endpoint, body string) ([]byte, int, error) {
		mu.Lock()
		defer mu.Unlock()
		switch {
		case method == "DELETE":
			removed = true
			return []byte(`{"message":"removal of container aurago-security-proxy is already in progress"}`), 409, nil
		case method == "GET" && endpoint == "/containers/"+containerName+"/json" && removed && !started:
			return []byte(`{"message":"No such container: aurago-security-proxy"}`), 404, nil
		case method == "POST" && strings.HasSuffix(endpoint, "/start"):
			started = true
		}
		return running(method, endpoint, body)
	}}
	m := testManager(t, cfg, fake)

	if err := m.Start(); err != nil {
		t.Fatalf("Start() error = %v, want the start to go on once the removal finished", err)
	}
	if !fake.called("POST /containers/create") {
		t.Fatal("Start did not create the new container")
	}
}

func TestManagerDestroyReportsSuccessWhenRemovalIsRefused(t *testing.T) {
	cfg := proxyConfig()
	running := runningEngine(imageName)
	fake := &fakeEngine{handle: func(method, endpoint, body string) ([]byte, int, error) {
		if method == "DELETE" {
			return []byte(`{"message":"driver \"overlay2\" failed to remove root filesystem: device or resource busy"}`), 500, nil
		}
		return running(method, endpoint, body)
	}}
	m := testManager(t, cfg, fake)

	if err := m.Destroy(); err != nil {
		t.Fatalf("Destroy() error = %v, want nil as before; the refused removal is only logged", err)
	}
	if !fake.called("DELETE /containers/" + containerName) {
		t.Fatal("Destroy did not try to remove the container")
	}
}

// noProcFiles stands in for a runtime whose /proc names no container (gVisor,
// Kata) or for a host without /proc.
func noProcFiles(string) ([]byte, error) { return nil, errors.New("no such file") }

func hostnameIs(name string) func() (string, error) {
	return func() (string, error) { return name, nil }
}

// countCalls counts the engine calls equal to call.
func countCalls(fake *fakeEngine, call string) int {
	fake.mu.Lock()
	defer fake.mu.Unlock()
	n := 0
	for _, c := range fake.calls {
		if c == call {
			n++
		}
	}
	return n
}

// selfLookupEngine answers like the default compose deployment, with the
// AuraGo container known only as id and created with Config.Hostname
// configHostname. Every other container reference but the proxy is unknown.
func selfLookupEngine(cfg *config.Config, id, configHostname string) *fakeEngine {
	inspection := strings.ReplaceAll(composeSelfInspection, `"/app/data"`, strconvQuote(filepath.ToSlash(cfg.Directories.DataDir)))
	inspection = strings.Replace(inspection, `"Name": "/aurago",`, `"Name": "/aurago", "Config": {"Hostname": `+strconvQuote(configHostname)+`},`, 1)
	running := runningEngine(imageName)
	return &fakeEngine{handle: func(method, endpoint, body string) ([]byte, int, error) {
		switch {
		case method == "GET" && endpoint == "/containers/"+id+"/json":
			return []byte(inspection), 200, nil
		case method == "GET" && strings.HasPrefix(endpoint, "/containers/") && strings.HasSuffix(endpoint, "/json") &&
			endpoint != "/containers/"+containerName+"/json":
			return []byte(`{"message":"No such container"}`), 404, nil
		case method == "GET" && endpoint == "/version":
			return []byte(`{"ApiVersion":"1.47"}`), 200, nil
		case method == "GET" && strings.HasPrefix(endpoint, "/networks/"):
			return []byte(fmt.Sprintf(`{"Internal":%t}`, endpoint != "/networks/net-default")), 200, nil
		}
		return running(method, endpoint, body)
	}}
}

// startedPlacement starts the proxy and reports whether the create used the
// Docker placement (AuraGo's network, no binds).
func startedInDockerPlacement(t *testing.T, m *Manager, fake *fakeEngine) bool {
	t.Helper()
	if err := m.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	hostConfig := decodeCreatePayload(t, fake.body("POST /containers/create?name="+containerName))["HostConfig"].(map[string]interface{})
	_, binds := hostConfig["Binds"]
	inDocker := hostConfig["NetworkMode"] == "aurago_default"
	if binds == inDocker {
		t.Fatalf("HostConfig = %#v, want either native binds or the Docker placement", hostConfig)
	}
	return inDocker
}

// Without a container ID in /proc (gVisor, Kata) a compose hostname equal to
// AuraGo's container name still finds AuraGo, as before M1, but only when the
// container's own Config.Hostname confirms it.
func TestManagerFindsAuraGoByCustomHostnameAsLastResort(t *testing.T) {
	for name, tc := range map[string]struct {
		configHostname string
		wantDocker     bool
	}{
		"hostname confirmed by the container":    {"aurago", true},
		"another container answers the hostname": {"something-else", false},
	} {
		t.Run(name, func(t *testing.T) {
			cfg := proxyConfig()
			cfg.Directories.DataDir = t.TempDir()
			fake := selfLookupEngine(cfg, "aurago", tc.configHostname)
			m := testManager(t, cfg, fake)
			m.inDocker = func() bool { return true }
			m.selfIDs = func() ([]string, string) { return selfContainerIDsFrom(noProcFiles, hostnameIs("aurago")) }

			if got := startedInDockerPlacement(t, m, fake); got != tc.wantDocker {
				t.Fatalf("Docker placement = %v, want %v", got, tc.wantDocker)
			}
			if got := countCalls(fake, "GET /containers/aurago/json"); got != 1 {
				t.Fatalf("inspect of the hostname = %d, want 1", got)
			}
		})
	}
}

func TestManagerNeverInspectsTheCustomHostnameWhenProcNamesAuraGo(t *testing.T) {
	cfg := proxyConfig()
	cfg.Directories.DataDir = t.TempDir()
	fake := selfLookupEngine(cfg, proxySelfContainerID, "aurago")
	m := testManager(t, cfg, fake)
	m.inDocker = func() bool { return true }
	mountinfo := etcMountinfo("/var/lib/docker/containers/" + proxySelfContainerID)
	m.selfIDs = func() ([]string, string) {
		return selfContainerIDsFrom(func(path string) ([]byte, error) {
			if path == "/proc/self/mountinfo" {
				return []byte(mountinfo), nil
			}
			return nil, errors.New("no such file")
		}, hostnameIs("aurago"))
	}

	if !startedInDockerPlacement(t, m, fake) {
		t.Fatal("the /proc container ID did not give the Docker placement")
	}
	if got := countCalls(fake, "GET /containers/aurago/json"); got != 0 {
		t.Fatalf("inspect of the custom hostname = %d, want none", got)
	}
}
