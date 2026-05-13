package tools

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"aurago/internal/config"
)

func manifestTestConfig() *config.Config {
	cfg := &config.Config{}
	cfg.Manifest.Enabled = true
	cfg.Manifest.AutoStart = true
	cfg.Manifest.Mode = "managed"
	cfg.Manifest.URL = "http://127.0.0.1:2099"
	cfg.Manifest.ContainerName = "aurago_manifest"
	cfg.Manifest.Image = "manifestdotbuild/manifest:5"
	cfg.Manifest.Host = "127.0.0.1"
	cfg.Manifest.Port = 2099
	cfg.Manifest.HostPort = 2099
	cfg.Manifest.NetworkName = "aurago_manifest"
	cfg.Manifest.PostgresContainerName = "aurago_manifest_postgres"
	cfg.Manifest.PostgresImage = "postgres:15-alpine"
	cfg.Manifest.PostgresUser = "manifest"
	cfg.Manifest.PostgresDatabase = "manifest"
	cfg.Manifest.PostgresVolume = "aurago_manifest_pgdata"
	cfg.Manifest.PostgresPassword = "pg-secret"
	cfg.Manifest.BetterAuthSecret = "better-auth-secret"
	return cfg
}

func decodeManifestPayload(t *testing.T, raw []byte) map[string]interface{} {
	t.Helper()
	var payload map[string]interface{}
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v\n%s", err, string(raw))
	}
	return payload
}

func TestManifestManagedURLHost(t *testing.T) {
	tests := []struct {
		name            string
		raw             string
		containerName   string
		runningInDocker bool
		want            string
	}{
		{name: "host loopback", raw: "http://127.0.0.1:2099", runningInDocker: false, want: "127.0.0.1"},
		{name: "docker service host", raw: "http://manifest:2099", runningInDocker: true, want: "manifest"},
		{name: "custom container host", raw: "http://aurago_manifest:2099", containerName: "aurago_manifest", runningInDocker: true, want: "aurago_manifest"},
		{name: "external host is unmanaged", raw: "https://manifest.example.test", runningInDocker: true, want: ""},
	}

	for _, tt := range tests {
		if got := ManifestManagedURLHost(tt.raw, tt.containerName, tt.runningInDocker); got != tt.want {
			t.Fatalf("%s: ManifestManagedURLHost() = %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestResolveManifestSidecarConfigRequiresManagedSecrets(t *testing.T) {
	cfg := manifestTestConfig()
	cfg.Manifest.PostgresPassword = ""

	_, err := ResolveManifestSidecarConfig(cfg, false)
	if err == nil || !strings.Contains(err.Error(), "postgres password") {
		t.Fatalf("ResolveManifestSidecarConfig() error = %v, want missing postgres password", err)
	}

	cfg = manifestTestConfig()
	cfg.Manifest.BetterAuthSecret = ""

	_, err = ResolveManifestSidecarConfig(cfg, false)
	if err == nil || !strings.Contains(err.Error(), "better auth secret") {
		t.Fatalf("ResolveManifestSidecarConfig() error = %v, want missing better auth secret", err)
	}
}

func TestResolveManifestSidecarConfigBuildsURLs(t *testing.T) {
	cfg := manifestTestConfig()

	sidecar, err := ResolveManifestSidecarConfig(cfg, false)
	if err != nil {
		t.Fatalf("ResolveManifestSidecarConfig() error = %v", err)
	}

	if sidecar.InternalBaseURL != "http://127.0.0.1:2099" {
		t.Fatalf("InternalBaseURL = %q, want loopback managed URL", sidecar.InternalBaseURL)
	}
	if sidecar.BrowserBaseURL != "http://127.0.0.1:2099" {
		t.Fatalf("BrowserBaseURL = %q, want published browser URL", sidecar.BrowserBaseURL)
	}
	if got := manifestDatabaseURL(sidecar); got != "postgresql://manifest:pg-secret@manifest-postgres:5432/manifest" {
		t.Fatalf("database URL = %q, want internal Postgres alias", got)
	}

	cfg = manifestTestConfig()
	cfg.Manifest.URL = "http://manifest:2099"
	sidecar, err = ResolveManifestSidecarConfig(cfg, true)
	if err != nil {
		t.Fatalf("ResolveManifestSidecarConfig(docker) error = %v", err)
	}
	if sidecar.InternalBaseURL != "http://manifest:2099" {
		t.Fatalf("docker InternalBaseURL = %q, want service URL", sidecar.InternalBaseURL)
	}
	if sidecar.BrowserBaseURL != "http://127.0.0.1:2099" {
		t.Fatalf("docker BrowserBaseURL = %q, want host-published browser URL", sidecar.BrowserBaseURL)
	}
}

func TestResolveManifestSidecarConfigNormalizesExternalEndpoint(t *testing.T) {
	cfg := manifestTestConfig()
	cfg.Manifest.Mode = "external"
	cfg.Manifest.ExternalBaseURL = "https://manifest.example.test/v1/chat/completions"

	sidecar, err := ResolveManifestSidecarConfig(cfg, false)
	if err != nil {
		t.Fatalf("ResolveManifestSidecarConfig(external) error = %v", err)
	}
	if sidecar.ProviderBaseURL != "https://manifest.example.test/v1" {
		t.Fatalf("ProviderBaseURL = %q, want normalized OpenAI-compatible endpoint", sidecar.ProviderBaseURL)
	}
}

func TestStopManifestSidecarsDoesNotRequireManagedSecrets(t *testing.T) {
	ConfigureRuntimePermissions(RuntimePermissions{DockerEnabled: true})
	defer ClearRuntimePermissionsForTest()

	var calls []string
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.Method+" "+r.URL.String())
		w.WriteHeader(http.StatusNoContent)
	}))
	defer api.Close()

	cfg := manifestTestConfig()
	cfg.Manifest.PostgresPassword = ""
	cfg.Manifest.BetterAuthSecret = ""

	host := "tcp://" + strings.TrimPrefix(api.URL, "http://")
	if err := StopManifestSidecars(t.Context(), host, cfg, nil); err != nil {
		t.Fatalf("StopManifestSidecars() error = %v", err)
	}

	got := strings.Join(calls, "\n")
	for _, want := range []string{
		"POST /v1.45/containers/aurago_manifest/stop?t=5",
		"DELETE /v1.45/containers/aurago_manifest?force=true",
		"POST /v1.45/containers/aurago_manifest_postgres/stop?t=5",
		"DELETE /v1.45/containers/aurago_manifest_postgres?force=true",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("Docker calls missing %q:\n%s", want, got)
		}
	}
}

func TestBuildManifestPostgresPayloadDoesNotPublishPort(t *testing.T) {
	sidecar, err := ResolveManifestSidecarConfig(manifestTestConfig(), false)
	if err != nil {
		t.Fatalf("ResolveManifestSidecarConfig() error = %v", err)
	}

	raw, err := buildManifestPostgresCreatePayload(sidecar, "aurago_manifest")
	if err != nil {
		t.Fatalf("buildManifestPostgresCreatePayload() error = %v", err)
	}
	payload := decodeManifestPayload(t, raw)
	hostConfig := payload["HostConfig"].(map[string]interface{})
	if _, exists := hostConfig["PortBindings"]; exists {
		t.Fatalf("Postgres payload must not publish ports: %#v", hostConfig["PortBindings"])
	}
	if got := strings.Join(interfaceSliceToStrings(hostConfig["Binds"]), "\n"); !strings.Contains(got, "aurago_manifest_pgdata:/var/lib/postgresql/data") {
		t.Fatalf("Postgres binds missing named volume: %s", got)
	}
}

func TestBuildManifestPayloadPublishesOnlyManifestPort(t *testing.T) {
	sidecar, err := ResolveManifestSidecarConfig(manifestTestConfig(), false)
	if err != nil {
		t.Fatalf("ResolveManifestSidecarConfig() error = %v", err)
	}

	raw, err := buildManifestCreatePayload(sidecar, "aurago_manifest")
	if err != nil {
		t.Fatalf("buildManifestCreatePayload() error = %v", err)
	}
	payload := decodeManifestPayload(t, raw)
	env := strings.Join(interfaceSliceToStrings(payload["Env"]), "\n")
	for _, want := range []string{
		"PORT=2099",
		"MANIFEST_TELEMETRY_DISABLED=1",
		"BETTER_AUTH_SECRET=better-auth-secret",
		"BETTER_AUTH_URL=http://127.0.0.1:2099",
		"DATABASE_URL=postgresql://manifest:pg-secret@manifest-postgres:5432/manifest",
	} {
		if !strings.Contains(env, want) {
			t.Fatalf("Manifest env missing %q:\n%s", want, env)
		}
	}
	hostConfig := payload["HostConfig"].(map[string]interface{})
	portBindings := hostConfig["PortBindings"].(map[string]interface{})
	bound := portBindings["2099/tcp"].([]interface{})[0].(map[string]interface{})
	if bound["HostIp"] != "0.0.0.0" || bound["HostPort"] != "2099" {
		t.Fatalf("Manifest port binding = %#v, want LAN-reachable 0.0.0.0:2099", bound)
	}
}

func TestManifestContainerNeedsRecreateForLoopbackOnlyBinding(t *testing.T) {
	inspect := []byte(`{
		"Config": {
			"Env": [
				"PORT=2099",
				"BETTER_AUTH_URL=http://127.0.0.1:2099",
				"MANIFEST_TELEMETRY_DISABLED=1"
			]
		},
		"HostConfig": {
			"PortBindings": {
				"2099/tcp": [{"HostIp": "127.0.0.1", "HostPort": "2099"}]
			}
		}
	}`)
	sidecar, err := ResolveManifestSidecarConfig(manifestTestConfig(), false)
	if err != nil {
		t.Fatalf("ResolveManifestSidecarConfig() error = %v", err)
	}
	if !manifestContainerNeedsRecreate(inspect, sidecar, sidecar.NetworkName) {
		t.Fatal("expected loopback-only existing Manifest container to require recreation")
	}
}

func TestManifestContainerNeedsRecreateAcceptsLANReachableBinding(t *testing.T) {
	inspect := []byte(`{
		"Config": {
			"Env": [
				"PORT=2099",
				"BETTER_AUTH_URL=http://127.0.0.1:2099",
				"MANIFEST_TELEMETRY_DISABLED=1"
			]
		},
		"HostConfig": {
			"PortBindings": {
				"2099/tcp": [{"HostIp": "0.0.0.0", "HostPort": "2099"}]
			}
		},
		"NetworkSettings": {
			"Networks": {
				"aurago_manifest": {}
			}
		}
	}`)
	sidecar, err := ResolveManifestSidecarConfig(manifestTestConfig(), false)
	if err != nil {
		t.Fatalf("ResolveManifestSidecarConfig() error = %v", err)
	}
	if manifestContainerNeedsRecreate(inspect, sidecar, sidecar.NetworkName) {
		t.Fatal("did not expect LAN-reachable existing Manifest container to require recreation")
	}
}

func TestManifestContainerNeedsRecreateForWrongDockerNetwork(t *testing.T) {
	inspect := []byte(`{
		"HostConfig": {
			"PortBindings": {
				"2099/tcp": [{"HostIp": "0.0.0.0", "HostPort": "2099"}]
			}
		},
		"NetworkSettings": {
			"Networks": {
				"old_manifest_network": {}
			}
		}
	}`)
	sidecar, err := ResolveManifestSidecarConfig(manifestTestConfig(), false)
	if err != nil {
		t.Fatalf("ResolveManifestSidecarConfig() error = %v", err)
	}
	if !manifestContainerNeedsRecreate(inspect, sidecar, sidecar.NetworkName) {
		t.Fatal("expected Manifest container on wrong Docker network to require recreation")
	}
}

func TestManifestPostgresContainerNeedsRecreateForWrongDockerNetwork(t *testing.T) {
	inspect := []byte(`{
		"NetworkSettings": {
			"Networks": {
				"old_manifest_network": {}
			}
		}
	}`)
	sidecar, err := ResolveManifestSidecarConfig(manifestTestConfig(), false)
	if err != nil {
		t.Fatalf("ResolveManifestSidecarConfig() error = %v", err)
	}
	if !manifestPostgresContainerNeedsRecreate(inspect, sidecar.NetworkName) {
		t.Fatal("expected Manifest Postgres container on wrong Docker network to require recreation")
	}
}

func interfaceSliceToStrings(v interface{}) []string {
	items, _ := v.([]interface{})
	out := make([]string, 0, len(items))
	for _, item := range items {
		if s, ok := item.(string); ok {
			out = append(out, s)
		}
	}
	return out
}
