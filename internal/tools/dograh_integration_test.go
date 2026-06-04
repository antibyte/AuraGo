package tools

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"aurago/internal/config"
)

func TestResolveDograhStackConfigRequiresManagedSecrets(t *testing.T) {
	cfg := &config.Config{}
	cfg.Dograh.Enabled = true
	cfg.Dograh.Mode = "managed"

	_, err := ResolveDograhStackConfig(cfg, false)
	if err == nil {
		t.Fatal("ResolveDograhStackConfig() error = nil, want missing secret error")
	}
	if !strings.Contains(err.Error(), "oss jwt secret") {
		t.Fatalf("error = %v, want missing OSS JWT secret", err)
	}
}

func TestResolveDograhStackConfigMigratesLegacyDockerHubDefaultImages(t *testing.T) {
	cfg := dograhTestConfig()
	cfg.Dograh.APIImage = "dograhai/dograh-api:latest"
	cfg.Dograh.UIImage = "dograhai/dograh-ui:latest"

	stack, err := ResolveDograhStackConfig(cfg, false)
	if err != nil {
		t.Fatalf("ResolveDograhStackConfig() error = %v", err)
	}
	if stack.APIImage != dograhDefaultAPIImage {
		t.Fatalf("APIImage = %q, want %q", stack.APIImage, dograhDefaultAPIImage)
	}
	if stack.UIImage != dograhDefaultUIImage {
		t.Fatalf("UIImage = %q, want %q", stack.UIImage, dograhDefaultUIImage)
	}
}

func TestResolveDograhStackConfigKeepsExplicitPinnedImages(t *testing.T) {
	cfg := dograhTestConfig()
	cfg.Dograh.APIImage = "dograhai/dograh-api:1.30.1"
	cfg.Dograh.UIImage = "registry.example/dograh-ui:custom"

	stack, err := ResolveDograhStackConfig(cfg, false)
	if err != nil {
		t.Fatalf("ResolveDograhStackConfig() error = %v", err)
	}
	if stack.APIImage != "dograhai/dograh-api:1.30.1" {
		t.Fatalf("APIImage = %q, want explicit pinned image", stack.APIImage)
	}
	if stack.UIImage != "registry.example/dograh-ui:custom" {
		t.Fatalf("UIImage = %q, want explicit custom image", stack.UIImage)
	}
}

func TestEnsureDograhStackRunningUsesCurrentImagesForLegacyRuntimeConfig(t *testing.T) {
	cfg := dograhTestConfig()
	cfg.Dograh.APIImage = "dograhai/dograh-api:latest"
	cfg.Dograh.UIImage = "dograhai/dograh-ui:latest"

	pulledImages := map[string]bool{}
	createdImages := map[string]string{}
	dockerAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		switch {
		case r.Method == http.MethodGet && path == "/"+dockerAPIVersion+"/networks/aurago_dograh":
			writeDograhTestJSON(w, http.StatusOK, map[string]any{"Name": "aurago_dograh"})
		case r.Method == http.MethodPost && path == "/"+dockerAPIVersion+"/images/create":
			pulledImages[r.URL.Query().Get("fromImage")] = true
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"pulled"}` + "\n"))
		case r.Method == http.MethodGet && strings.HasPrefix(path, "/"+dockerAPIVersion+"/containers/") && strings.HasSuffix(path, "/json"):
			writeDograhTestJSON(w, http.StatusNotFound, map[string]string{"message": "not found"})
		case r.Method == http.MethodGet && strings.HasPrefix(path, "/"+dockerAPIVersion+"/images/") && strings.HasSuffix(path, "/json"):
			writeDograhTestJSON(w, http.StatusOK, map[string]any{"Id": "image-id"})
		case r.Method == http.MethodPost && path == "/"+dockerAPIVersion+"/containers/create":
			raw, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("read create payload: %v", err)
			}
			var payload struct {
				Image string `json:"Image"`
			}
			if err := json.Unmarshal(raw, &payload); err != nil {
				t.Fatalf("parse create payload: %v", err)
			}
			createdImages[r.URL.Query().Get("name")] = payload.Image
			writeDograhTestJSON(w, http.StatusCreated, map[string]string{"Id": "container-id"})
		case r.Method == http.MethodPost && strings.HasPrefix(path, "/"+dockerAPIVersion+"/containers/") && strings.HasSuffix(path, "/start"):
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("unexpected Docker request: %s %s", r.Method, r.URL.String())
		}
	}))
	defer dockerAPI.Close()

	dockerHost := "tcp://" + strings.TrimPrefix(dockerAPI.URL, "http://")
	if err := EnsureDograhStackRunning(context.Background(), dockerHost, cfg, nil); err != nil {
		t.Fatalf("EnsureDograhStackRunning() error = %v", err)
	}

	if !pulledImages[dograhDefaultAPIImage] {
		t.Fatalf("pulled images = %#v, want %q", pulledImages, dograhDefaultAPIImage)
	}
	if !pulledImages[dograhDefaultUIImage] {
		t.Fatalf("pulled images = %#v, want %q", pulledImages, dograhDefaultUIImage)
	}
	if createdImages[cfg.Dograh.APIContainerName] != dograhDefaultAPIImage {
		t.Fatalf("API create image = %q, want %q", createdImages[cfg.Dograh.APIContainerName], dograhDefaultAPIImage)
	}
	if createdImages[cfg.Dograh.UIContainerName] != dograhDefaultUIImage {
		t.Fatalf("UI create image = %q, want %q", createdImages[cfg.Dograh.UIContainerName], dograhDefaultUIImage)
	}
	if createdImages[dograhDefaultUIProxyContainerName] != dograhDefaultUIProxyImage {
		t.Fatalf("UI proxy create image = %q, want %q", createdImages[dograhDefaultUIProxyContainerName], dograhDefaultUIProxyImage)
	}
}

func TestStopDograhStackRemovesUIProxyBeforeUI(t *testing.T) {
	var calls []string
	dockerAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.Method+" "+r.URL.String())
		w.WriteHeader(http.StatusNoContent)
	}))
	defer dockerAPI.Close()

	cfg := dograhTestConfig()
	dockerHost := "tcp://" + strings.TrimPrefix(dockerAPI.URL, "http://")
	if err := StopDograhStack(context.Background(), dockerHost, cfg, nil); err != nil {
		t.Fatalf("StopDograhStack() error = %v", err)
	}

	got := strings.Join(calls, "\n")
	for _, want := range []string{
		"POST /v1.45/containers/aurago_dograh_ui_proxy/stop?t=5",
		"DELETE /v1.45/containers/aurago_dograh_ui_proxy?force=true",
		"POST /v1.45/containers/aurago_dograh_ui/stop?t=5",
		"DELETE /v1.45/containers/aurago_dograh_ui?force=true",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("Docker calls missing %q:\n%s", want, got)
		}
	}
	if strings.Index(got, "aurago_dograh_ui_proxy") > strings.Index(got, "aurago_dograh_ui/stop") {
		t.Fatalf("UI proxy must be removed before UI:\n%s", got)
	}
}

func TestBuildDograhAPICreatePayloadMatchesUpstreamContract(t *testing.T) {
	cfg := dograhTestConfig()
	sidecar, err := ResolveDograhStackConfig(cfg, false)
	if err != nil {
		t.Fatalf("ResolveDograhStackConfig() error = %v", err)
	}

	raw, err := buildDograhAPICreatePayload(sidecar, sidecar.NetworkName)
	if err != nil {
		t.Fatalf("buildDograhAPICreatePayload() error = %v", err)
	}
	var payload struct {
		Image      string   `json:"Image"`
		Env        []string `json:"Env"`
		HostConfig struct {
			PortBindings map[string][]struct {
				HostIP   string `json:"HostIp"`
				HostPort string `json:"HostPort"`
			} `json:"PortBindings"`
		} `json:"HostConfig"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	env := map[string]string{}
	for _, item := range payload.Env {
		key, value, ok := strings.Cut(item, "=")
		if ok {
			env[key] = value
		}
	}
	if payload.Image != "ghcr.io/dograh-hq/dograh-api:latest" {
		t.Fatalf("Image = %q, want Dograh API image", payload.Image)
	}
	if env["DATABASE_URL"] != "postgresql+asyncpg://postgres:pg-secret@dograh-postgres:5432/postgres" {
		t.Fatalf("DATABASE_URL = %q", env["DATABASE_URL"])
	}
	if env["REDIS_URL"] != "redis://:redis-secret@dograh-redis:6379" {
		t.Fatalf("REDIS_URL = %q", env["REDIS_URL"])
	}
	if env["ENABLE_AWS_S3"] != "false" {
		t.Fatalf("ENABLE_AWS_S3 = %q, want false", env["ENABLE_AWS_S3"])
	}
	if env["MINIO_ENDPOINT"] != "dograh-minio:9000" {
		t.Fatalf("MINIO_ENDPOINT = %q", env["MINIO_ENDPOINT"])
	}
	if env["MINIO_PUBLIC_ENDPOINT"] != "http://127.0.0.1:9000" {
		t.Fatalf("MINIO_PUBLIC_ENDPOINT = %q", env["MINIO_PUBLIC_ENDPOINT"])
	}
	if env["MINIO_BUCKET"] != "dograh" {
		t.Fatalf("MINIO_BUCKET = %q", env["MINIO_BUCKET"])
	}
	if _, ok := env["MINIO_BUCKET_NAME"]; ok {
		t.Fatalf("MINIO_BUCKET_NAME should not be set for Dograh API payload")
	}
	if env["MINIO_SECURE"] != "false" {
		t.Fatalf("MINIO_SECURE = %q, want false", env["MINIO_SECURE"])
	}
	if env["OSS_JWT_SECRET"] != "jwt-secret" {
		t.Fatalf("OSS_JWT_SECRET = %q", env["OSS_JWT_SECRET"])
	}
	if env["ENABLE_TELEMETRY"] != "false" {
		t.Fatalf("ENABLE_TELEMETRY = %q, want false", env["ENABLE_TELEMETRY"])
	}
	bindings := payload.HostConfig.PortBindings["8000/tcp"]
	if len(bindings) != 1 || bindings[0].HostPort != "8000" {
		t.Fatalf("API port bindings = %#v, want host port 8000", bindings)
	}
}

func TestBuildDograhUICreatePayloadUsesOSSLocalAuthContract(t *testing.T) {
	cfg := dograhTestConfig()
	sidecar, err := ResolveDograhStackConfig(cfg, false)
	if err != nil {
		t.Fatalf("ResolveDograhStackConfig() error = %v", err)
	}

	raw, err := buildDograhUICreatePayload(sidecar, sidecar.NetworkName)
	if err != nil {
		t.Fatalf("buildDograhUICreatePayload() error = %v", err)
	}
	var payload struct {
		Image      string            `json:"Image"`
		Env        []string          `json:"Env"`
		Labels     map[string]string `json:"Labels"`
		HostConfig struct {
			PortBindings map[string]interface{} `json:"PortBindings"`
		} `json:"HostConfig"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	env := map[string]string{}
	for _, item := range payload.Env {
		key, value, ok := strings.Cut(item, "=")
		if ok {
			env[key] = value
		}
	}
	if payload.Image != "ghcr.io/dograh-hq/dograh-ui:latest" {
		t.Fatalf("Image = %q, want Dograh UI image", payload.Image)
	}
	if env["BACKEND_URL"] != "http://dograh-api:8000" {
		t.Fatalf("BACKEND_URL = %q", env["BACKEND_URL"])
	}
	if env["NODE_ENV"] != "oss" {
		t.Fatalf("NODE_ENV = %q, want oss", env["NODE_ENV"])
	}
	if env["NEXT_PUBLIC_NODE_ENV"] != "oss" {
		t.Fatalf("NEXT_PUBLIC_NODE_ENV = %q, want oss", env["NEXT_PUBLIC_NODE_ENV"])
	}
	if env["DEPLOYMENT_MODE"] != "oss" {
		t.Fatalf("DEPLOYMENT_MODE = %q, want oss", env["DEPLOYMENT_MODE"])
	}
	if env["AUTH_PROVIDER"] != "local" {
		t.Fatalf("AUTH_PROVIDER = %q, want local", env["AUTH_PROVIDER"])
	}
	for _, key := range []string{"NEXT_PUBLIC_STACK_PROJECT_ID", "NEXT_PUBLIC_STACK_PUBLISHABLE_CLIENT_KEY", "STACK_SECRET_SERVER_KEY", "SECRET_SERVER_KEY"} {
		if got := env[key]; got != "" {
			t.Fatalf("%s = %q, want empty/unset for OSS local auth", key, got)
		}
	}
	if payload.Labels[dograhStackRevisionLabel] != dograhStackRevision {
		t.Fatalf("stack revision label = %q, want %q", payload.Labels[dograhStackRevisionLabel], dograhStackRevision)
	}
	if len(payload.HostConfig.PortBindings) != 0 {
		t.Fatalf("UI container must stay internal; port bindings = %#v", payload.HostConfig.PortBindings)
	}
}

func TestBuildDograhUIProxyCreatePayloadPublishesUIAndShimsConfigRoutes(t *testing.T) {
	cfg := dograhTestConfig()
	sidecar, err := ResolveDograhStackConfig(cfg, false)
	if err != nil {
		t.Fatalf("ResolveDograhStackConfig() error = %v", err)
	}

	raw, err := buildDograhUIProxyCreatePayload(sidecar, sidecar.NetworkName)
	if err != nil {
		t.Fatalf("buildDograhUIProxyCreatePayload() error = %v", err)
	}
	var payload struct {
		Image      string   `json:"Image"`
		Cmd        []string `json:"Cmd"`
		HostConfig struct {
			PortBindings map[string][]struct {
				HostIP   string `json:"HostIp"`
				HostPort string `json:"HostPort"`
			} `json:"PortBindings"`
		} `json:"HostConfig"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	cmd := strings.Join(payload.Cmd, "\n")
	if payload.Image != dograhDefaultUIProxyImage {
		t.Fatalf("Image = %q, want UI proxy image", payload.Image)
	}
	for _, want := range []string{
		`return 200 '{"provider":"local"}';`,
		`return 200 '{"enabled":false,"dsn":"","environment":"production"}';`,
		`return 200 '{"enabled":false,"key":"","host":"/ingest","uiHost":"https://us.posthog.com"}';`,
		"location /api/v1/",
		"proxy_pass $dograh_api;",
		"proxy_pass $dograh_ui;",
	} {
		if !strings.Contains(cmd, want) {
			t.Fatalf("proxy command missing %q:\n%s", want, cmd)
		}
	}
	if strings.Contains(cmd, "location /api/ {") {
		t.Fatalf("proxy command must not route all /api/* to Dograh API:\n%s", cmd)
	}
	bindings := payload.HostConfig.PortBindings["3010/tcp"]
	if len(bindings) != 1 || bindings[0].HostIP != "127.0.0.1" || bindings[0].HostPort != "3010" {
		t.Fatalf("proxy port bindings = %#v, want host 127.0.0.1:3010", bindings)
	}
}

func TestBuildDograhMinioCreatePayloadExposesLocalS3Endpoint(t *testing.T) {
	cfg := dograhTestConfig()
	sidecar, err := ResolveDograhStackConfig(cfg, false)
	if err != nil {
		t.Fatalf("ResolveDograhStackConfig() error = %v", err)
	}

	raw, err := buildDograhMinioCreatePayload(sidecar, sidecar.NetworkName)
	if err != nil {
		t.Fatalf("buildDograhMinioCreatePayload() error = %v", err)
	}
	var payload struct {
		Env          []string            `json:"Env"`
		ExposedPorts map[string]struct{} `json:"ExposedPorts"`
		HostConfig   struct {
			PortBindings map[string][]struct {
				HostIP   string `json:"HostIp"`
				HostPort string `json:"HostPort"`
			} `json:"PortBindings"`
		} `json:"HostConfig"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if !containsExactString(payload.Env, "MINIO_API_CORS_ALLOW_ORIGIN=*") {
		t.Fatalf("Env = %#v, want MINIO CORS for browser access", payload.Env)
	}
	for _, port := range []string{"9000/tcp", "9001/tcp"} {
		if _, ok := payload.ExposedPorts[port]; !ok {
			t.Fatalf("ExposedPorts = %#v, want %s", payload.ExposedPorts, port)
		}
		bindings := payload.HostConfig.PortBindings[port]
		if len(bindings) != 1 || bindings[0].HostIP != "127.0.0.1" || bindings[0].HostPort != strings.TrimSuffix(port, "/tcp") {
			t.Fatalf("%s port bindings = %#v, want loopback binding", port, bindings)
		}
	}
}

func TestDograhAPIContainerNeedsRecreateWhenMinioPublicEndpointMissing(t *testing.T) {
	cfg := dograhTestConfig()
	sidecar, err := ResolveDograhStackConfig(cfg, false)
	if err != nil {
		t.Fatalf("ResolveDograhStackConfig() error = %v", err)
	}
	inspect := []byte(`{
		"Config": {"Env": [
			"MINIO_ENDPOINT=http://dograh-minio:9000",
			"MINIO_BUCKET_NAME=dograh"
		]},
		"HostConfig": {"PortBindings": {"8000/tcp": [{"HostIp": "127.0.0.1", "HostPort": "8000"}]}},
		"NetworkSettings": {"Networks": {"aurago_dograh": {}}}
	}`)

	if !dograhAPIContainerNeedsRecreate(inspect, sidecar.NetworkName, sidecar) {
		t.Fatal("dograhAPIContainerNeedsRecreate() = false, want true for legacy MinIO env")
	}
}

func TestDograhAPIContainerNeedsRecreateWhenLegacyDockerHubImageIsPresent(t *testing.T) {
	cfg := dograhTestConfig()
	sidecar, err := ResolveDograhStackConfig(cfg, false)
	if err != nil {
		t.Fatalf("ResolveDograhStackConfig() error = %v", err)
	}
	inspect := []byte(`{
		"Config": {
			"Image": "dograhai/dograh-api:latest",
			"Env": [
				"ENABLE_AWS_S3=false",
				"MINIO_ENDPOINT=dograh-minio:9000",
				"MINIO_PUBLIC_ENDPOINT=http://127.0.0.1:9000",
				"MINIO_ACCESS_KEY=minioadmin",
				"MINIO_SECRET_KEY=minio-secret",
				"MINIO_BUCKET=dograh",
				"MINIO_SECURE=false",
				"OSS_JWT_SECRET=jwt-secret",
				"BACKEND_API_ENDPOINT=http://127.0.0.1:8000",
				"ENABLE_TELEMETRY=false",
				"FASTAPI_WORKERS=1",
				"DATABASE_URL=postgresql+asyncpg://postgres:pg-secret@dograh-postgres:5432/postgres",
				"REDIS_URL=redis://:redis-secret@dograh-redis:6379",
				"ENVIRONMENT=local",
				"DEPLOYMENT_MODE=oss",
				"AUTH_PROVIDER=local",
				"LOG_LEVEL=INFO"
			],
			"Labels": {"org.aurago.dograh.stack-revision": "` + dograhStackRevision + `"}
		},
		"HostConfig": {"PortBindings": {"8000/tcp": [{"HostIp": "127.0.0.1", "HostPort": "8000"}]}},
		"NetworkSettings": {"Networks": {"aurago_dograh": {}}}
	}`)

	if !dograhAPIContainerNeedsRecreate(inspect, sidecar.NetworkName, sidecar) {
		t.Fatal("dograhAPIContainerNeedsRecreate() = false, want true for legacy DockerHub image")
	}
}

func TestDograhUIContainerNeedsRecreateWhenLegacyStackAuthContractIsPresent(t *testing.T) {
	cfg := dograhTestConfig()
	sidecar, err := ResolveDograhStackConfig(cfg, false)
	if err != nil {
		t.Fatalf("ResolveDograhStackConfig() error = %v", err)
	}
	inspect := []byte(`{
		"Config": {"Env": [
			"BACKEND_URL=http://dograh-api:8000",
			"NODE_ENV=production",
			"NEXT_PUBLIC_STACK_PROJECT_ID=stack-project"
		]},
		"HostConfig": {"PortBindings": {"3010/tcp": [{"HostIp": "127.0.0.1", "HostPort": "3010"}]}},
		"NetworkSettings": {"Networks": {"aurago_dograh": {}}}
	}`)

	if !dograhUIContainerNeedsRecreate(inspect, sidecar.NetworkName, sidecar) {
		t.Fatal("dograhUIContainerNeedsRecreate() = false, want true for stale Stack Auth UI contract")
	}
}

func TestDograhUIContainerNeedsRecreateAcceptsCurrentOSSContract(t *testing.T) {
	cfg := dograhTestConfig()
	sidecar, err := ResolveDograhStackConfig(cfg, false)
	if err != nil {
		t.Fatalf("ResolveDograhStackConfig() error = %v", err)
	}
	inspect := []byte(`{
		"Config": {
			"Image": "ghcr.io/dograh-hq/dograh-ui:latest",
			"Env": [
				"BACKEND_URL=http://dograh-api:8000",
				"NODE_ENV=oss",
				"NEXT_PUBLIC_NODE_ENV=oss",
				"DEPLOYMENT_MODE=oss",
				"AUTH_PROVIDER=local",
				"ENABLE_TELEMETRY=false"
			],
			"Labels": {
				"org.aurago.integration": "dograh",
				"org.aurago.dograh.stack-revision": "` + dograhStackRevision + `"
			}
		},
		"HostConfig": {},
		"NetworkSettings": {"Networks": {"aurago_dograh": {}}}
	}`)

	if dograhUIContainerNeedsRecreate(inspect, sidecar.NetworkName, sidecar) {
		t.Fatal("dograhUIContainerNeedsRecreate() = true, want false for current OSS UI contract")
	}
}

func TestDograhUIContainerNeedsRecreateWhenUIStillPublishesHostPort(t *testing.T) {
	cfg := dograhTestConfig()
	sidecar, err := ResolveDograhStackConfig(cfg, false)
	if err != nil {
		t.Fatalf("ResolveDograhStackConfig() error = %v", err)
	}
	inspect := []byte(`{
		"Config": {
			"Image": "ghcr.io/dograh-hq/dograh-ui:latest",
			"Env": [
				"BACKEND_URL=http://dograh-api:8000",
				"NODE_ENV=oss",
				"NEXT_PUBLIC_NODE_ENV=oss",
				"DEPLOYMENT_MODE=oss",
				"AUTH_PROVIDER=local",
				"ENABLE_TELEMETRY=false"
			],
			"Labels": {"org.aurago.dograh.stack-revision": "` + dograhStackRevision + `"}
		},
		"HostConfig": {"PortBindings": {"3010/tcp": [{"HostIp": "127.0.0.1", "HostPort": "3010"}]}},
		"NetworkSettings": {"Networks": {"aurago_dograh": {}}}
	}`)

	if !dograhUIContainerNeedsRecreate(inspect, sidecar.NetworkName, sidecar) {
		t.Fatal("dograhUIContainerNeedsRecreate() = false, want true for legacy UI host port")
	}
}

func TestDograhUIContainerNeedsRecreateWhenLegacyDockerHubImageIsPresent(t *testing.T) {
	cfg := dograhTestConfig()
	sidecar, err := ResolveDograhStackConfig(cfg, false)
	if err != nil {
		t.Fatalf("ResolveDograhStackConfig() error = %v", err)
	}
	inspect := []byte(`{
		"Config": {
			"Image": "dograhai/dograh-ui:latest",
			"Env": [
				"BACKEND_URL=http://dograh-api:8000",
				"NODE_ENV=oss",
				"NEXT_PUBLIC_NODE_ENV=oss",
				"DEPLOYMENT_MODE=oss",
				"AUTH_PROVIDER=local",
				"ENABLE_TELEMETRY=false"
			],
			"Labels": {"org.aurago.dograh.stack-revision": "` + dograhStackRevision + `"}
		},
		"HostConfig": {"PortBindings": {"3010/tcp": [{"HostIp": "127.0.0.1", "HostPort": "3010"}]}},
		"NetworkSettings": {"Networks": {"aurago_dograh": {}}}
	}`)

	if !dograhUIContainerNeedsRecreate(inspect, sidecar.NetworkName, sidecar) {
		t.Fatal("dograhUIContainerNeedsRecreate() = false, want true for legacy DockerHub image")
	}
}

func TestDograhUIProxyContainerNeedsRecreateAcceptsCurrentProxyConfig(t *testing.T) {
	cfg := dograhTestConfig()
	sidecar, err := ResolveDograhStackConfig(cfg, false)
	if err != nil {
		t.Fatalf("ResolveDograhStackConfig() error = %v", err)
	}
	cmd := dograhUIProxyStartupScript(sidecar)
	inspect, err := json.Marshal(map[string]any{
		"Config": map[string]any{
			"Image":  dograhDefaultUIProxyImage,
			"Cmd":    []string{"sh", "-c", cmd},
			"Labels": map[string]string{dograhStackRevisionLabel: dograhStackRevision},
		},
		"HostConfig": map[string]any{
			"PortBindings": map[string]any{
				"3010/tcp": []map[string]string{{"HostIp": "127.0.0.1", "HostPort": "3010"}},
			},
		},
		"NetworkSettings": map[string]any{"Networks": map[string]any{"aurago_dograh": map[string]any{}}},
	})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	if dograhUIProxyContainerNeedsRecreate(inspect, sidecar.NetworkName, sidecar) {
		t.Fatal("dograhUIProxyContainerNeedsRecreate() = true, want false for current proxy config")
	}
}

func TestDograhUIProxyContainerNeedsRecreateWhenAuthConfigShimMissing(t *testing.T) {
	cfg := dograhTestConfig()
	sidecar, err := ResolveDograhStackConfig(cfg, false)
	if err != nil {
		t.Fatalf("ResolveDograhStackConfig() error = %v", err)
	}
	inspect := []byte(`{
		"Config": {
			"Image": "nginx:1.27-alpine",
			"Cmd": ["sh", "-c", "proxy_pass $dograh_ui;"],
			"Labels": {"org.aurago.dograh.stack-revision": "` + dograhStackRevision + `"}
		},
		"HostConfig": {"PortBindings": {"3010/tcp": [{"HostIp": "127.0.0.1", "HostPort": "3010"}]}},
		"NetworkSettings": {"Networks": {"aurago_dograh": {}}}
	}`)

	if !dograhUIProxyContainerNeedsRecreate(inspect, sidecar.NetworkName, sidecar) {
		t.Fatal("dograhUIProxyContainerNeedsRecreate() = false, want true when auth config shim is missing")
	}
}

func TestDograhUIProxyContainerNeedsRecreateWhenAllAPIIsProxiedToBackend(t *testing.T) {
	cfg := dograhTestConfig()
	sidecar, err := ResolveDograhStackConfig(cfg, false)
	if err != nil {
		t.Fatalf("ResolveDograhStackConfig() error = %v", err)
	}
	cmd := strings.ReplaceAll(dograhUIProxyStartupScript(sidecar), "location /api/v1/", "location /api/")
	inspect, err := json.Marshal(map[string]any{
		"Config": map[string]any{
			"Image":  dograhDefaultUIProxyImage,
			"Cmd":    []string{"sh", "-c", cmd},
			"Labels": map[string]string{dograhStackRevisionLabel: dograhStackRevision},
		},
		"HostConfig": map[string]any{
			"PortBindings": map[string]any{
				"3010/tcp": []map[string]string{{"HostIp": "127.0.0.1", "HostPort": "3010"}},
			},
		},
		"NetworkSettings": map[string]any{"Networks": map[string]any{"aurago_dograh": map[string]any{}}},
	})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	if !dograhUIProxyContainerNeedsRecreate(inspect, sidecar.NetworkName, sidecar) {
		t.Fatal("dograhUIProxyContainerNeedsRecreate() = false, want true when all /api routes are proxied to backend")
	}
}

func TestDograhImageUsesFloatingTag(t *testing.T) {
	tests := []struct {
		image string
		want  bool
	}{
		{"ghcr.io/dograh-hq/dograh-ui:latest", true},
		{"dograhai/dograh-ui", true},
		{"dograhai/dograh-ui:1.34.0", false},
		{"dograhai/dograh-ui@sha256:abc123", false},
	}
	for _, tt := range tests {
		if got := dograhImageUsesFloatingTag(tt.image); got != tt.want {
			t.Fatalf("dograhImageUsesFloatingTag(%q) = %v, want %v", tt.image, got, tt.want)
		}
	}
}

func TestBuildDograhCoturnCreatePayloadUsesTurnDefaults(t *testing.T) {
	cfg := dograhTestConfig()
	cfg.Dograh.TurnEnabled = true
	sidecar, err := ResolveDograhStackConfig(cfg, false)
	if err != nil {
		t.Fatalf("ResolveDograhStackConfig() error = %v", err)
	}

	raw, err := buildDograhCoturnCreatePayload(sidecar, sidecar.NetworkName)
	if err != nil {
		t.Fatalf("buildDograhCoturnCreatePayload() error = %v", err)
	}
	var payload struct {
		Image        string              `json:"Image"`
		Cmd          []string            `json:"Cmd"`
		ExposedPorts map[string]struct{} `json:"ExposedPorts"`
		HostConfig   struct {
			PortBindings map[string][]struct {
				HostIP   string `json:"HostIp"`
				HostPort string `json:"HostPort"`
			} `json:"PortBindings"`
		} `json:"HostConfig"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if payload.Image != "coturn/coturn:4.8.0" {
		t.Fatalf("Image = %q, want coturn default image", payload.Image)
	}
	if !containsExactString(payload.Cmd, "--static-auth-secret=jwt-secret") {
		t.Fatalf("Cmd = %#v, want static-auth-secret from vault secret", payload.Cmd)
	}
	if _, ok := payload.ExposedPorts["3478/tcp"]; !ok {
		t.Fatalf("ExposedPorts = %#v, want 3478/tcp", payload.ExposedPorts)
	}
	bindings := payload.HostConfig.PortBindings["3478/udp"]
	if len(bindings) != 1 || bindings[0].HostIP != "127.0.0.1" || bindings[0].HostPort != "3478" {
		t.Fatalf("TURN UDP port bindings = %#v, want loopback 3478", bindings)
	}
}

func TestDograhMCPURLUsesAPIV1MCPPath(t *testing.T) {
	if got := DograhMCPURL("http://dograh.local:8000/"); got != "http://dograh.local:8000/api/v1/mcp/" {
		t.Fatalf("DograhMCPURL() = %q", got)
	}
}

func containsExactString(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}

func dograhTestConfig() *config.Config {
	cfg := &config.Config{}
	cfg.Dograh.Enabled = true
	cfg.Dograh.Mode = "managed"
	cfg.Dograh.APIURL = "http://127.0.0.1:8000"
	cfg.Dograh.UIURL = "http://127.0.0.1:3010"
	cfg.Dograh.APIImage = "ghcr.io/dograh-hq/dograh-api:latest"
	cfg.Dograh.UIImage = "ghcr.io/dograh-hq/dograh-ui:latest"
	cfg.Dograh.PostgresImage = "pgvector/pgvector:pg17"
	cfg.Dograh.RedisImage = "redis:7"
	cfg.Dograh.MinioImage = "minio/minio:latest"
	cfg.Dograh.APIContainerName = "aurago_dograh_api"
	cfg.Dograh.UIContainerName = "aurago_dograh_ui"
	cfg.Dograh.PostgresContainerName = "aurago_dograh_postgres"
	cfg.Dograh.RedisContainerName = "aurago_dograh_redis"
	cfg.Dograh.MinioContainerName = "aurago_dograh_minio"
	cfg.Dograh.NetworkName = "aurago_dograh"
	cfg.Dograh.Host = "127.0.0.1"
	cfg.Dograh.APIPort = 8000
	cfg.Dograh.APIHostPort = 8000
	cfg.Dograh.UIPort = 3010
	cfg.Dograh.UIHostPort = 3010
	cfg.Dograh.PostgresUser = "postgres"
	cfg.Dograh.PostgresDatabase = "postgres"
	cfg.Dograh.PostgresVolume = "aurago_dograh_pgdata"
	cfg.Dograh.RedisVolume = "aurago_dograh_redisdata"
	cfg.Dograh.MinioVolume = "aurago_dograh_minio"
	cfg.Dograh.MinioRootUser = "minioadmin"
	cfg.Dograh.MinioBucket = "dograh"
	cfg.Dograh.OSSJWTSecret = "jwt-secret"
	cfg.Dograh.PostgresPassword = "pg-secret"
	cfg.Dograh.RedisPassword = "redis-secret"
	cfg.Dograh.MinioRootPassword = "minio-secret"
	return cfg
}

func writeDograhTestJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
