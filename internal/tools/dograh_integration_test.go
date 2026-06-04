package tools

import (
	"encoding/json"
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
	if payload.Image != "dograhai/dograh-api:latest" {
		t.Fatalf("Image = %q, want Dograh API image", payload.Image)
	}
	if env["DATABASE_URL"] != "postgresql+asyncpg://postgres:pg-secret@dograh-postgres:5432/postgres" {
		t.Fatalf("DATABASE_URL = %q", env["DATABASE_URL"])
	}
	if env["REDIS_URL"] != "redis://:redis-secret@dograh-redis:6379" {
		t.Fatalf("REDIS_URL = %q", env["REDIS_URL"])
	}
	if env["MINIO_ENDPOINT"] != "http://dograh-minio:9000" {
		t.Fatalf("MINIO_ENDPOINT = %q", env["MINIO_ENDPOINT"])
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
	cfg.Dograh.APIImage = "dograhai/dograh-api:latest"
	cfg.Dograh.UIImage = "dograhai/dograh-ui:latest"
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
