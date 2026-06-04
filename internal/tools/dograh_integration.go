package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"aurago/internal/config"
)

const (
	dograhDefaultAPIImage              = "ghcr.io/dograh-hq/dograh-api:latest"
	dograhDefaultUIImage               = "ghcr.io/dograh-hq/dograh-ui:latest"
	dograhDefaultPostgresImage         = "pgvector/pgvector:pg17"
	dograhDefaultRedisImage            = "redis:7"
	dograhDefaultMinioImage            = "minio/minio:latest"
	dograhDefaultCoturnImage           = "coturn/coturn:4.8.0"
	dograhDefaultAPIContainerName      = "aurago_dograh_api"
	dograhDefaultUIContainerName       = "aurago_dograh_ui"
	dograhDefaultPostgresContainerName = "aurago_dograh_postgres"
	dograhDefaultRedisContainerName    = "aurago_dograh_redis"
	dograhDefaultMinioContainerName    = "aurago_dograh_minio"
	dograhDefaultCoturnContainerName   = "aurago_dograh_coturn"
	dograhDefaultNetworkName           = "aurago_dograh"
	dograhDefaultAPIAlias              = "dograh-api"
	dograhDefaultUIAlias               = "dograh-ui"
	dograhDefaultPostgresAlias         = "dograh-postgres"
	dograhDefaultRedisAlias            = "dograh-redis"
	dograhDefaultMinioAlias            = "dograh-minio"
	dograhDefaultCoturnAlias           = "dograh-coturn"
	dograhDefaultPostgresVolume        = "aurago_dograh_pgdata"
	dograhDefaultRedisVolume           = "aurago_dograh_redisdata"
	dograhDefaultMinioVolume           = "aurago_dograh_minio"
	dograhPostgresDataPath             = "/var/lib/postgresql/data"
	dograhRedisDataPath                = "/data"
	dograhMinioDataPath                = "/data"
	dograhMinioAPIPort                 = 9000
	dograhMinioConsolePort             = 9001
	dograhCoturnPort                   = 3478
	dograhStatusSetupRequired          = "setup_required"
	dograhHealthProbeTimeout           = 3 * time.Second
	dograhStackRevision                = "20260604-ghcr-images"
	dograhStackRevisionLabel           = "org.aurago.dograh.stack-revision"
)

// DograhStackConfig is the resolved runtime configuration for Dograh's managed stack.
type DograhStackConfig struct {
	Mode                  string
	InternalAPIURL        string
	BrowserAPIURL         string
	UIURL                 string
	HealthPath            string
	Host                  string
	APIPort               int
	APIHostPort           int
	UIPort                int
	UIHostPort            int
	NetworkName           string
	APIContainerName      string
	UIContainerName       string
	PostgresContainerName string
	RedisContainerName    string
	MinioContainerName    string
	CoturnContainerName   string
	APIAlias              string
	UIAlias               string
	PostgresAlias         string
	RedisAlias            string
	MinioAlias            string
	CoturnAlias           string
	APIImage              string
	UIImage               string
	PostgresImage         string
	RedisImage            string
	MinioImage            string
	CoturnImage           string
	PostgresUser          string
	PostgresDatabase      string
	PostgresVolume        string
	PostgresPassword      string
	RedisVolume           string
	RedisPassword         string
	MinioVolume           string
	MinioRootUser         string
	MinioRootPassword     string
	MinioBucket           string
	OSSJWTSecret          string
	TelemetryEnabled      bool
	TurnEnabled           bool
	RunningInDocker       bool
}

// DograhStatus reports the managed Dograh stack status in a UI-friendly shape.
type DograhStatus struct {
	Enabled            bool              `json:"enabled"`
	Mode               string            `json:"mode"`
	Status             string            `json:"status"`
	Running            bool              `json:"running"`
	APIURL             string            `json:"api_url"`
	UIURL              string            `json:"ui_url"`
	MCPURL             string            `json:"mcp_url"`
	Containers         map[string]string `json:"containers,omitempty"`
	SetupRequired      bool              `json:"setup_required,omitempty"`
	AdminSetupRequired bool              `json:"admin_setup_required,omitempty"`
	Message            string            `json:"message,omitempty"`
}

// DograhConnectionTestResult is returned by Dograh API health/setup tests.
type DograhConnectionTestResult struct {
	Status        string `json:"status"`
	Message       string `json:"message,omitempty"`
	NodeTypeCount int    `json:"node_type_count,omitempty"`
}

// DograhToolRegistrationResult describes a Dograh MCP tool registration attempt.
type DograhToolRegistrationResult struct {
	Status   string `json:"status"`
	Message  string `json:"message,omitempty"`
	ToolUUID string `json:"tool_uuid,omitempty"`
}

// ResolveDograhStackConfig resolves managed Dograh URLs, container names, images, and secrets.
func ResolveDograhStackConfig(cfg *config.Config, runningInDocker bool) (DograhStackConfig, error) {
	if cfg == nil {
		return DograhStackConfig{}, fmt.Errorf("config is required")
	}
	mode := strings.ToLower(strings.TrimSpace(cfg.Dograh.Mode))
	if mode == "" {
		mode = "managed"
	}
	if mode != "managed" {
		apiURL := strings.TrimRight(strings.TrimSpace(cfg.Dograh.APIURL), "/")
		return DograhStackConfig{
			Mode:           mode,
			InternalAPIURL: apiURL,
			BrowserAPIURL:  apiURL,
			UIURL:          strings.TrimRight(strings.TrimSpace(cfg.Dograh.UIURL), "/"),
			HealthPath:     defaultString(cfg.Dograh.HealthPath, "/api/v1/health"),
		}, nil
	}
	ossJWTSecret := strings.TrimSpace(cfg.Dograh.OSSJWTSecret)
	if ossJWTSecret == "" {
		return DograhStackConfig{}, fmt.Errorf("dograh oss jwt secret is required in the vault")
	}
	postgresPassword := strings.TrimSpace(cfg.Dograh.PostgresPassword)
	if postgresPassword == "" {
		return DograhStackConfig{}, fmt.Errorf("dograh postgres password is required in the vault")
	}
	redisPassword := strings.TrimSpace(cfg.Dograh.RedisPassword)
	if redisPassword == "" {
		return DograhStackConfig{}, fmt.Errorf("dograh redis password is required in the vault")
	}
	minioRootPassword := strings.TrimSpace(cfg.Dograh.MinioRootPassword)
	if minioRootPassword == "" {
		return DograhStackConfig{}, fmt.Errorf("dograh minio root password is required in the vault")
	}

	apiPort := cfg.Dograh.APIPort
	if apiPort <= 0 {
		apiPort = 8000
	}
	apiHostPort := cfg.Dograh.APIHostPort
	if apiHostPort <= 0 {
		apiHostPort = apiPort
	}
	uiPort := cfg.Dograh.UIPort
	if uiPort <= 0 {
		uiPort = 3010
	}
	uiHostPort := cfg.Dograh.UIHostPort
	if uiHostPort <= 0 {
		uiHostPort = uiPort
	}
	host := defaultString(cfg.Dograh.Host, "127.0.0.1")
	internalAPIURL := strings.TrimRight(strings.TrimSpace(cfg.Dograh.APIURL), "/")
	if internalAPIURL == "" {
		internalAPIURL = defaultDograhBaseURL(runningInDocker, dograhDefaultAPIAlias, apiPort)
	}
	uiURL := strings.TrimRight(strings.TrimSpace(cfg.Dograh.UIURL), "/")
	if uiURL == "" {
		uiURL = defaultDograhBaseURL(runningInDocker, dograhDefaultUIAlias, uiPort)
	}
	browserHost := host
	if browserHost == "" || browserHost == "0.0.0.0" || browserHost == "::" {
		browserHost = "127.0.0.1"
	}
	browserAPIURL := fmt.Sprintf("http://%s:%d", browserHost, apiHostPort)
	browserUIURL := fmt.Sprintf("http://%s:%d", browserHost, uiHostPort)
	if !runningInDocker {
		uiURL = browserUIURL
	}

	return DograhStackConfig{
		Mode:                  "managed",
		InternalAPIURL:        internalAPIURL,
		BrowserAPIURL:         browserAPIURL,
		UIURL:                 uiURL,
		HealthPath:            defaultString(cfg.Dograh.HealthPath, "/api/v1/health"),
		Host:                  host,
		APIPort:               apiPort,
		APIHostPort:           apiHostPort,
		UIPort:                uiPort,
		UIHostPort:            uiHostPort,
		NetworkName:           defaultString(cfg.Dograh.NetworkName, dograhDefaultNetworkName),
		APIContainerName:      defaultString(cfg.Dograh.APIContainerName, dograhDefaultAPIContainerName),
		UIContainerName:       defaultString(cfg.Dograh.UIContainerName, dograhDefaultUIContainerName),
		PostgresContainerName: defaultString(cfg.Dograh.PostgresContainerName, dograhDefaultPostgresContainerName),
		RedisContainerName:    defaultString(cfg.Dograh.RedisContainerName, dograhDefaultRedisContainerName),
		MinioContainerName:    defaultString(cfg.Dograh.MinioContainerName, dograhDefaultMinioContainerName),
		CoturnContainerName:   defaultString(cfg.Dograh.CoturnContainerName, dograhDefaultCoturnContainerName),
		APIAlias:              dograhDefaultAPIAlias,
		UIAlias:               dograhDefaultUIAlias,
		PostgresAlias:         dograhDefaultPostgresAlias,
		RedisAlias:            dograhDefaultRedisAlias,
		MinioAlias:            dograhDefaultMinioAlias,
		CoturnAlias:           dograhDefaultCoturnAlias,
		APIImage:              defaultString(cfg.Dograh.APIImage, dograhDefaultAPIImage),
		UIImage:               defaultString(cfg.Dograh.UIImage, dograhDefaultUIImage),
		PostgresImage:         defaultString(cfg.Dograh.PostgresImage, dograhDefaultPostgresImage),
		RedisImage:            defaultString(cfg.Dograh.RedisImage, dograhDefaultRedisImage),
		MinioImage:            defaultString(cfg.Dograh.MinioImage, dograhDefaultMinioImage),
		CoturnImage:           defaultString(cfg.Dograh.CoturnImage, dograhDefaultCoturnImage),
		PostgresUser:          defaultString(cfg.Dograh.PostgresUser, "postgres"),
		PostgresDatabase:      defaultString(cfg.Dograh.PostgresDatabase, "postgres"),
		PostgresVolume:        defaultString(cfg.Dograh.PostgresVolume, dograhDefaultPostgresVolume),
		PostgresPassword:      postgresPassword,
		RedisVolume:           defaultString(cfg.Dograh.RedisVolume, dograhDefaultRedisVolume),
		RedisPassword:         redisPassword,
		MinioVolume:           defaultString(cfg.Dograh.MinioVolume, dograhDefaultMinioVolume),
		MinioRootUser:         defaultString(cfg.Dograh.MinioRootUser, "minioadmin"),
		MinioRootPassword:     minioRootPassword,
		MinioBucket:           defaultString(cfg.Dograh.MinioBucket, "dograh"),
		OSSJWTSecret:          ossJWTSecret,
		TelemetryEnabled:      cfg.Dograh.TelemetryEnabled,
		TurnEnabled:           cfg.Dograh.TurnEnabled,
		RunningInDocker:       runningInDocker,
	}, nil
}

func dograhRunsInDocker(cfg *config.Config) bool {
	return (cfg != nil && cfg.Runtime.IsDocker) || browserAutomationRunsInDocker()
}

func defaultDograhBaseURL(runningInDocker bool, service string, port int) string {
	if runningInDocker {
		return fmt.Sprintf("http://%s:%d", service, port)
	}
	return fmt.Sprintf("http://127.0.0.1:%d", port)
}

// DograhMCPURL returns Dograh's streamable HTTP MCP endpoint.
func DograhMCPURL(apiURL string) string {
	base := strings.TrimRight(strings.TrimSpace(apiURL), "/")
	if base == "" {
		return ""
	}
	return base + "/api/v1/mcp/"
}

func dograhDatabaseURL(stack DograhStackConfig) string {
	return fmt.Sprintf("postgresql+asyncpg://%s@%s:5432/%s",
		url.UserPassword(stack.PostgresUser, stack.PostgresPassword).String(),
		stack.PostgresAlias,
		url.PathEscape(stack.PostgresDatabase),
	)
}

func dograhRedisURL(stack DograhStackConfig) string {
	return fmt.Sprintf("redis://:%s@%s:6379", url.QueryEscape(stack.RedisPassword), stack.RedisAlias)
}

func dograhMinioEndpoint(stack DograhStackConfig) string {
	return fmt.Sprintf("%s:%d", stack.MinioAlias, dograhMinioAPIPort)
}

func dograhMinioPublicEndpoint(stack DograhStackConfig) string {
	host := dograhPublicURLHost(stack.Host)
	return (&url.URL{
		Scheme: "http",
		Host:   net.JoinHostPort(host, strconv.Itoa(dograhMinioAPIPort)),
	}).String()
}

func dograhPublicURLHost(host string) string {
	host = strings.Trim(strings.TrimSpace(host), "[]")
	if host == "" || strings.EqualFold(host, "localhost") || host == "0.0.0.0" || host == "::" || host == "::1" {
		return "127.0.0.1"
	}
	return host
}

func buildDograhPostgresCreatePayload(stack DograhStackConfig, networkName string) ([]byte, error) {
	if err := validateDockerName(stack.PostgresContainerName); err != nil {
		return nil, err
	}
	if err := validateDockerName(stack.PostgresImage); err != nil {
		return nil, err
	}
	if networkName == "" {
		networkName = stack.NetworkName
	}
	payload := map[string]interface{}{
		"Image": stack.PostgresImage,
		"Env": []string{
			"POSTGRES_USER=" + stack.PostgresUser,
			"POSTGRES_PASSWORD=" + stack.PostgresPassword,
			"POSTGRES_DB=" + stack.PostgresDatabase,
		},
		"HostConfig": map[string]interface{}{
			"RestartPolicy": map[string]interface{}{"Name": "unless-stopped"},
			"Binds":         []string{stack.PostgresVolume + ":" + dograhPostgresDataPath},
			"NetworkMode":   networkName,
		},
		"NetworkingConfig": manifestNetworkingConfig(networkName, stack.PostgresAlias),
	}
	return json.Marshal(payload)
}

func buildDograhRedisCreatePayload(stack DograhStackConfig, networkName string) ([]byte, error) {
	if err := validateDockerName(stack.RedisContainerName); err != nil {
		return nil, err
	}
	if err := validateDockerName(stack.RedisImage); err != nil {
		return nil, err
	}
	if networkName == "" {
		networkName = stack.NetworkName
	}
	payload := map[string]interface{}{
		"Image": stack.RedisImage,
		"Cmd":   []string{"redis-server", "--requirepass", stack.RedisPassword},
		"HostConfig": map[string]interface{}{
			"RestartPolicy": map[string]interface{}{"Name": "unless-stopped"},
			"Binds":         []string{stack.RedisVolume + ":" + dograhRedisDataPath},
			"NetworkMode":   networkName,
		},
		"NetworkingConfig": manifestNetworkingConfig(networkName, stack.RedisAlias),
	}
	return json.Marshal(payload)
}

func buildDograhMinioCreatePayload(stack DograhStackConfig, networkName string) ([]byte, error) {
	if err := validateDockerName(stack.MinioContainerName); err != nil {
		return nil, err
	}
	if err := validateDockerName(stack.MinioImage); err != nil {
		return nil, err
	}
	if networkName == "" {
		networkName = stack.NetworkName
	}
	payload := map[string]interface{}{
		"Image": stack.MinioImage,
		"Env": []string{
			"MINIO_ROOT_USER=" + stack.MinioRootUser,
			"MINIO_ROOT_PASSWORD=" + stack.MinioRootPassword,
			"MINIO_API_CORS_ALLOW_ORIGIN=*",
		},
		"Cmd": []string{"server", dograhMinioDataPath, "--console-address", fmt.Sprintf(":%d", dograhMinioConsolePort)},
		"ExposedPorts": map[string]interface{}{
			fmt.Sprintf("%d/tcp", dograhMinioAPIPort):     struct{}{},
			fmt.Sprintf("%d/tcp", dograhMinioConsolePort): struct{}{},
		},
		"HostConfig": map[string]interface{}{
			"RestartPolicy": map[string]interface{}{"Name": "unless-stopped"},
			"Binds":         []string{stack.MinioVolume + ":" + dograhMinioDataPath},
			"NetworkMode":   networkName,
			"PortBindings": map[string]interface{}{
				fmt.Sprintf("%d/tcp", dograhMinioAPIPort): []map[string]string{
					{"HostIp": dograhPublishHost(stack.Host), "HostPort": strconv.Itoa(dograhMinioAPIPort)},
				},
				fmt.Sprintf("%d/tcp", dograhMinioConsolePort): []map[string]string{
					{"HostIp": dograhPublishHost(stack.Host), "HostPort": strconv.Itoa(dograhMinioConsolePort)},
				},
			},
		},
		"NetworkingConfig": manifestNetworkingConfig(networkName, stack.MinioAlias),
	}
	return json.Marshal(payload)
}

func buildDograhCoturnCreatePayload(stack DograhStackConfig, networkName string) ([]byte, error) {
	if err := validateDockerName(stack.CoturnContainerName); err != nil {
		return nil, err
	}
	if err := validateDockerName(stack.CoturnImage); err != nil {
		return nil, err
	}
	if networkName == "" {
		networkName = stack.NetworkName
	}
	tcpPort := fmt.Sprintf("%d/tcp", dograhCoturnPort)
	udpPort := fmt.Sprintf("%d/udp", dograhCoturnPort)
	portBinding := []map[string]string{{"HostIp": dograhPublishHost(stack.Host), "HostPort": strconv.Itoa(dograhCoturnPort)}}
	payload := map[string]interface{}{
		"Image": stack.CoturnImage,
		"Cmd": []string{
			"-n",
			"--log-file=stdout",
			"--no-cli",
			"--fingerprint",
			"--use-auth-secret",
			"--static-auth-secret=" + stack.OSSJWTSecret,
			"--realm=dograh.local",
		},
		"ExposedPorts": map[string]interface{}{
			tcpPort: struct{}{},
			udpPort: struct{}{},
		},
		"HostConfig": map[string]interface{}{
			"RestartPolicy": map[string]interface{}{"Name": "unless-stopped"},
			"NetworkMode":   networkName,
			"PortBindings": map[string]interface{}{
				tcpPort: portBinding,
				udpPort: portBinding,
			},
		},
		"NetworkingConfig": manifestNetworkingConfig(networkName, stack.CoturnAlias),
	}
	return json.Marshal(payload)
}

func buildDograhAPICreatePayload(stack DograhStackConfig, networkName string) ([]byte, error) {
	if err := validateDockerName(stack.APIContainerName); err != nil {
		return nil, err
	}
	if err := validateDockerName(stack.APIImage); err != nil {
		return nil, err
	}
	if networkName == "" {
		networkName = stack.NetworkName
	}
	containerPort := fmt.Sprintf("%d/tcp", stack.APIPort)
	env := []string{
		"ENVIRONMENT=local",
		"DEPLOYMENT_MODE=oss",
		"AUTH_PROVIDER=local",
		"LOG_LEVEL=INFO",
		"DATABASE_URL=" + dograhDatabaseURL(stack),
		"REDIS_URL=" + dograhRedisURL(stack),
		"ENABLE_AWS_S3=false",
		"MINIO_ENDPOINT=" + dograhMinioEndpoint(stack),
		"MINIO_PUBLIC_ENDPOINT=" + dograhMinioPublicEndpoint(stack),
		"MINIO_ACCESS_KEY=" + stack.MinioRootUser,
		"MINIO_SECRET_KEY=" + stack.MinioRootPassword,
		"MINIO_BUCKET=" + stack.MinioBucket,
		"MINIO_SECURE=false",
		"OSS_JWT_SECRET=" + stack.OSSJWTSecret,
		"BACKEND_API_ENDPOINT=" + stack.BrowserAPIURL,
		"ENABLE_TELEMETRY=" + strconv.FormatBool(stack.TelemetryEnabled),
		"FASTAPI_WORKERS=1",
	}
	payload := map[string]interface{}{
		"Image":  stack.APIImage,
		"Env":    env,
		"Labels": dograhStackLabels("api"),
		"ExposedPorts": map[string]interface{}{
			containerPort: struct{}{},
		},
		"HostConfig": map[string]interface{}{
			"RestartPolicy": map[string]interface{}{"Name": "unless-stopped"},
			"NetworkMode":   networkName,
			"PortBindings": map[string]interface{}{
				containerPort: []map[string]string{{"HostIp": dograhPublishHost(stack.Host), "HostPort": strconv.Itoa(stack.APIHostPort)}},
			},
		},
		"NetworkingConfig": manifestNetworkingConfig(networkName, stack.APIAlias),
	}
	return json.Marshal(payload)
}

func buildDograhUICreatePayload(stack DograhStackConfig, networkName string) ([]byte, error) {
	if err := validateDockerName(stack.UIContainerName); err != nil {
		return nil, err
	}
	if err := validateDockerName(stack.UIImage); err != nil {
		return nil, err
	}
	if networkName == "" {
		networkName = stack.NetworkName
	}
	containerPort := fmt.Sprintf("%d/tcp", stack.UIPort)
	payload := map[string]interface{}{
		"Image": stack.UIImage,
		"Env": []string{
			"BACKEND_URL=http://" + stack.APIAlias + ":" + strconv.Itoa(stack.APIPort),
			"NODE_ENV=oss",
			"NEXT_PUBLIC_NODE_ENV=oss",
			"DEPLOYMENT_MODE=oss",
			"AUTH_PROVIDER=local",
			"ENABLE_TELEMETRY=" + strconv.FormatBool(stack.TelemetryEnabled),
		},
		"Labels": dograhStackLabels("ui"),
		"ExposedPorts": map[string]interface{}{
			containerPort: struct{}{},
		},
		"HostConfig": map[string]interface{}{
			"RestartPolicy": map[string]interface{}{"Name": "unless-stopped"},
			"NetworkMode":   networkName,
			"PortBindings": map[string]interface{}{
				containerPort: []map[string]string{{"HostIp": dograhPublishHost(stack.Host), "HostPort": strconv.Itoa(stack.UIHostPort)}},
			},
		},
		"NetworkingConfig": manifestNetworkingConfig(networkName, stack.UIAlias),
	}
	return json.Marshal(payload)
}

func dograhStackLabels(service string) map[string]string {
	return map[string]string{
		"org.aurago.integration":            "dograh",
		"org.aurago.dograh.service":         service,
		dograhStackRevisionLabel:            dograhStackRevision,
		"org.aurago.dograh.auth-provider":   "local",
		"org.aurago.dograh.deployment-mode": "oss",
	}
}

func dograhPublishHost(host string) string {
	host = strings.TrimSpace(host)
	if host == "" || strings.EqualFold(host, "localhost") || host == "::1" {
		return "127.0.0.1"
	}
	return host
}

type dograhContainerPlan struct {
	name          string
	image         string
	payload       func() ([]byte, error)
	needsRecreate func([]byte) bool
}

// EnsureDograhStackRunning creates and starts Dograh's managed Docker stack.
func EnsureDograhStackRunning(ctx context.Context, dockerHost string, cfg *config.Config, logger interface {
	Info(string, ...any)
	Warn(string, ...any)
	Error(string, ...any)
}) error {
	if cfg == nil || !cfg.Dograh.Enabled || strings.EqualFold(strings.TrimSpace(cfg.Dograh.Mode), "external") {
		return nil
	}
	stack, err := ResolveDograhStackConfig(cfg, dograhRunsInDocker(cfg))
	if err != nil {
		return err
	}
	dockerCfg := DockerConfig{Host: dockerHost}
	networkName, err := ensureDograhDockerNetwork(dockerCfg, stack)
	if err != nil {
		if logger != nil {
			logger.Warn("[Dograh] Failed to ensure Docker network", "error", err)
		}
		return err
	}
	containers := []dograhContainerPlan{
		{stack.PostgresContainerName, stack.PostgresImage, func() ([]byte, error) { return buildDograhPostgresCreatePayload(stack, networkName) }, func(data []byte) bool { return !dograhContainerAttached(data, networkName) }},
		{stack.RedisContainerName, stack.RedisImage, func() ([]byte, error) { return buildDograhRedisCreatePayload(stack, networkName) }, func(data []byte) bool { return !dograhContainerAttached(data, networkName) }},
		{stack.MinioContainerName, stack.MinioImage, func() ([]byte, error) { return buildDograhMinioCreatePayload(stack, networkName) }, func(data []byte) bool {
			return dograhMinioContainerNeedsRecreate(data, networkName, stack)
		}},
		{stack.APIContainerName, stack.APIImage, func() ([]byte, error) { return buildDograhAPICreatePayload(stack, networkName) }, func(data []byte) bool {
			return dograhAPIContainerNeedsRecreate(data, networkName, stack)
		}},
		{stack.UIContainerName, stack.UIImage, func() ([]byte, error) { return buildDograhUICreatePayload(stack, networkName) }, func(data []byte) bool {
			return dograhUIContainerNeedsRecreate(data, networkName, stack)
		}},
	}
	if stack.TurnEnabled {
		containers = append(containers[:3], append([]dograhContainerPlan{
			{stack.CoturnContainerName, stack.CoturnImage, func() ([]byte, error) { return buildDograhCoturnCreatePayload(stack, networkName) }, func(data []byte) bool {
				return dograhPortContainerNeedsRecreate(data, networkName, dograhCoturnPort, dograhCoturnPort, stack.Host)
			}},
		}, containers[3:]...)...)
	}
	for _, image := range []string{stack.APIImage, stack.UIImage} {
		if err := dograhPullFloatingImage(ctx, dockerCfg, image, logger); err != nil {
			return err
		}
	}
	for _, container := range containers {
		if err := ensureManifestContainerWithRecreate(ctx, dockerCfg, container.name, container.image, container.payload, container.needsRecreate); err != nil {
			if logger != nil {
				logger.Error("[Dograh] Failed to ensure container", "container", container.name, "error", err)
			}
			return err
		}
	}
	if logger != nil {
		logger.Info("[Dograh] Stack is running", "api", stack.APIContainerName, "ui", stack.UIContainerName)
	}
	return nil
}

func dograhPullFloatingImage(ctx context.Context, dockerCfg DockerConfig, image string, logger interface {
	Warn(string, ...any)
}) error {
	if !dograhImageUsesFloatingTag(image) {
		return nil
	}
	if err := PullImageForce(ctx, dockerCfg, image, nil); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if logger != nil {
			logger.Warn("[Dograh] Failed to pull floating Docker image; using the local image if available", "image", image, "error", err)
		}
	}
	return nil
}

func dograhImageUsesFloatingTag(image string) bool {
	ref := strings.TrimSpace(image)
	if ref == "" || strings.Contains(ref, "@sha256:") {
		return false
	}
	lastSlash := strings.LastIndex(ref, "/")
	lastColon := strings.LastIndex(ref, ":")
	if lastColon <= lastSlash {
		return true
	}
	return strings.EqualFold(ref[lastColon+1:], "latest")
}

func ensureDograhDockerNetwork(dockerCfg DockerConfig, stack DograhStackConfig) (string, error) {
	if stack.RunningInDocker {
		networkName, err := browserAutomationCurrentContainerNetwork(dockerCfg)
		if err == nil && strings.TrimSpace(networkName) != "" {
			return networkName, nil
		}
		return "", err
	}
	networkName := defaultString(stack.NetworkName, dograhDefaultNetworkName)
	data, code, err := dockerRequest(dockerCfg, http.MethodGet, "/networks/"+url.PathEscape(networkName), "")
	if err != nil {
		return "", err
	}
	if code == http.StatusOK {
		_ = data
		return networkName, nil
	}
	if code != http.StatusNotFound {
		return "", fmt.Errorf("inspect Docker network %q returned HTTP %d", networkName, code)
	}
	body, _ := json.Marshal(map[string]interface{}{"Name": networkName, "Driver": "bridge"})
	_, createCode, createErr := dockerRequest(dockerCfg, http.MethodPost, "/networks/create", string(body))
	if createErr != nil {
		return "", createErr
	}
	if createCode != http.StatusCreated && createCode != http.StatusOK {
		return "", fmt.Errorf("create Docker network %q returned HTTP %d", networkName, createCode)
	}
	return networkName, nil
}

func dograhContainerAttached(data []byte, networkName string) bool {
	var info struct {
		NetworkSettings struct {
			Networks map[string]interface{} `json:"Networks"`
		} `json:"NetworkSettings"`
	}
	if err := json.Unmarshal(data, &info); err != nil {
		return true
	}
	return manifestContainerAttachedToNetwork(info.NetworkSettings.Networks, networkName)
}

func dograhMinioContainerNeedsRecreate(data []byte, networkName string, stack DograhStackConfig) bool {
	if dograhPortContainerNeedsRecreate(data, networkName, dograhMinioAPIPort, dograhMinioAPIPort, stack.Host) {
		return true
	}
	if dograhPortContainerNeedsRecreate(data, networkName, dograhMinioConsolePort, dograhMinioConsolePort, stack.Host) {
		return true
	}
	return dograhContainerEnvValue(data, "MINIO_API_CORS_ALLOW_ORIGIN") != "*"
}

func dograhAPIContainerNeedsRecreate(data []byte, networkName string, stack DograhStackConfig) bool {
	if dograhPortContainerNeedsRecreate(data, networkName, stack.APIPort, stack.APIHostPort, stack.Host) {
		return true
	}
	if dograhContainerImageNeedsRecreate(data, stack.APIImage) {
		return true
	}
	if dograhContainerLabelValue(data, dograhStackRevisionLabel) != dograhStackRevision {
		return true
	}
	expected := map[string]string{
		"ENABLE_AWS_S3":         "false",
		"MINIO_ENDPOINT":        dograhMinioEndpoint(stack),
		"MINIO_PUBLIC_ENDPOINT": dograhMinioPublicEndpoint(stack),
		"MINIO_ACCESS_KEY":      stack.MinioRootUser,
		"MINIO_SECRET_KEY":      stack.MinioRootPassword,
		"MINIO_BUCKET":          stack.MinioBucket,
		"MINIO_SECURE":          "false",
		"OSS_JWT_SECRET":        stack.OSSJWTSecret,
		"BACKEND_API_ENDPOINT":  stack.BrowserAPIURL,
		"ENABLE_TELEMETRY":      strconv.FormatBool(stack.TelemetryEnabled),
		"FASTAPI_WORKERS":       "1",
		"DATABASE_URL":          dograhDatabaseURL(stack),
		"REDIS_URL":             dograhRedisURL(stack),
		"ENVIRONMENT":           "local",
		"DEPLOYMENT_MODE":       "oss",
		"AUTH_PROVIDER":         "local",
		"LOG_LEVEL":             "INFO",
	}
	for key, want := range expected {
		if got := dograhContainerEnvValue(data, key); got != want {
			return true
		}
	}
	return false
}

func dograhUIContainerNeedsRecreate(data []byte, networkName string, stack DograhStackConfig) bool {
	if dograhPortContainerNeedsRecreate(data, networkName, stack.UIPort, stack.UIHostPort, stack.Host) {
		return true
	}
	if dograhContainerImageNeedsRecreate(data, stack.UIImage) {
		return true
	}
	if dograhContainerLabelValue(data, dograhStackRevisionLabel) != dograhStackRevision {
		return true
	}
	expected := map[string]string{
		"BACKEND_URL":          "http://" + stack.APIAlias + ":" + strconv.Itoa(stack.APIPort),
		"NODE_ENV":             "oss",
		"NEXT_PUBLIC_NODE_ENV": "oss",
		"DEPLOYMENT_MODE":      "oss",
		"AUTH_PROVIDER":        "local",
		"ENABLE_TELEMETRY":     strconv.FormatBool(stack.TelemetryEnabled),
	}
	for key, want := range expected {
		if got := dograhContainerEnvValue(data, key); got != want {
			return true
		}
	}
	for _, key := range []string{"NEXT_PUBLIC_STACK_PROJECT_ID", "NEXT_PUBLIC_STACK_PUBLISHABLE_CLIENT_KEY", "STACK_SECRET_SERVER_KEY", "SECRET_SERVER_KEY"} {
		if dograhContainerEnvValue(data, key) != "" {
			return true
		}
	}
	return false
}

func dograhContainerImageNeedsRecreate(data []byte, expectedImage string) bool {
	got := strings.TrimSpace(dograhContainerImageValue(data))
	want := strings.TrimSpace(expectedImage)
	return got != "" && want != "" && !strings.EqualFold(got, want)
}

func dograhContainerImageValue(data []byte) string {
	var info struct {
		Config struct {
			Image string `json:"Image"`
		} `json:"Config"`
	}
	if err := json.Unmarshal(data, &info); err != nil {
		return ""
	}
	return info.Config.Image
}

func dograhContainerEnvValue(data []byte, key string) string {
	var info struct {
		Config struct {
			Env []string `json:"Env"`
		} `json:"Config"`
	}
	if err := json.Unmarshal(data, &info); err != nil {
		return ""
	}
	return manifestEnvValue(info.Config.Env, key)
}

func dograhContainerLabelValue(data []byte, key string) string {
	var info struct {
		Config struct {
			Labels map[string]string `json:"Labels"`
		} `json:"Config"`
	}
	if err := json.Unmarshal(data, &info); err != nil {
		return ""
	}
	return strings.TrimSpace(info.Config.Labels[key])
}

func dograhPortContainerNeedsRecreate(data []byte, networkName string, containerPort, hostPort int, host string) bool {
	var info struct {
		HostConfig struct {
			PortBindings map[string][]struct {
				HostIP   string `json:"HostIp"`
				HostPort string `json:"HostPort"`
			} `json:"PortBindings"`
		} `json:"HostConfig"`
		NetworkSettings struct {
			Networks map[string]interface{} `json:"Networks"`
		} `json:"NetworkSettings"`
	}
	if err := json.Unmarshal(data, &info); err != nil {
		return false
	}
	if !manifestContainerAttachedToNetwork(info.NetworkSettings.Networks, networkName) {
		return true
	}
	bindings := info.HostConfig.PortBindings[fmt.Sprintf("%d/tcp", containerPort)]
	wantHost := dograhPublishHost(host)
	wantPort := strconv.Itoa(hostPort)
	for _, binding := range bindings {
		hostIP := strings.TrimSpace(binding.HostIP)
		if hostIP == "" {
			hostIP = "0.0.0.0"
		}
		if hostIP == wantHost && strings.TrimSpace(binding.HostPort) == wantPort {
			return false
		}
	}
	return true
}

// StopDograhStack stops and removes managed Dograh containers without deleting named volumes.
func StopDograhStack(ctx context.Context, dockerHost string, cfg *config.Config, logger interface {
	Info(string, ...any)
	Warn(string, ...any)
	Error(string, ...any)
}) error {
	if cfg == nil {
		return fmt.Errorf("config is required")
	}
	if !cfg.Dograh.Enabled {
		return nil
	}
	mode := strings.ToLower(strings.TrimSpace(cfg.Dograh.Mode))
	if mode == "" {
		mode = "managed"
	}
	if mode != "managed" {
		return nil
	}
	names := []string{
		defaultString(cfg.Dograh.UIContainerName, dograhDefaultUIContainerName),
		defaultString(cfg.Dograh.APIContainerName, dograhDefaultAPIContainerName),
		defaultString(cfg.Dograh.CoturnContainerName, dograhDefaultCoturnContainerName),
		defaultString(cfg.Dograh.MinioContainerName, dograhDefaultMinioContainerName),
		defaultString(cfg.Dograh.RedisContainerName, dograhDefaultRedisContainerName),
		defaultString(cfg.Dograh.PostgresContainerName, dograhDefaultPostgresContainerName),
	}
	dockerCfg := DockerConfig{Host: dockerHost}
	for _, name := range names {
		if err := validateDockerName(name); err != nil {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		_, _, _ = dockerRequest(dockerCfg, http.MethodPost, "/containers/"+url.PathEscape(name)+"/stop?t=5", "")
		_, _, _ = dockerRequest(dockerCfg, http.MethodDelete, "/containers/"+url.PathEscape(name)+"?force=true", "")
	}
	if logger != nil {
		logger.Info("[Dograh] Stack stopped", "api", cfg.Dograh.APIContainerName, "ui", cfg.Dograh.UIContainerName)
	}
	return nil
}

// DograhStackStatus returns a best-effort Docker and health status.
func DograhStackStatus(ctx context.Context, dockerHost string, cfg *config.Config) (DograhStatus, error) {
	if cfg == nil {
		return DograhStatus{Status: dograhStatusSetupRequired, Message: "config is required"}, fmt.Errorf("config is required")
	}
	if !cfg.Dograh.Enabled {
		return DograhStatus{Enabled: false, Mode: cfg.Dograh.Mode, Status: "disabled"}, nil
	}
	stack, err := ResolveDograhStackConfig(cfg, dograhRunsInDocker(cfg))
	if err != nil {
		return DograhStatus{Enabled: true, Mode: cfg.Dograh.Mode, Status: dograhStatusSetupRequired, SetupRequired: true, Message: err.Error()}, nil
	}
	status := DograhStatus{
		Enabled:            true,
		Mode:               stack.Mode,
		Status:             manifestStatusStarting,
		APIURL:             stack.BrowserAPIURL,
		UIURL:              stack.UIURL,
		MCPURL:             DograhMCPURL(stack.InternalAPIURL),
		AdminSetupRequired: strings.TrimSpace(cfg.Dograh.APIKey) == "",
		Containers: map[string]string{
			"api":      stack.APIContainerName,
			"ui":       stack.UIContainerName,
			"postgres": stack.PostgresContainerName,
			"redis":    stack.RedisContainerName,
			"minio":    stack.MinioContainerName,
		},
	}
	if stack.TurnEnabled {
		status.Containers["coturn"] = stack.CoturnContainerName
	}
	if stack.Mode != "managed" {
		status.Status = manifestStatusUnknown
		status.APIURL = stack.InternalAPIURL
		return status, nil
	}
	data, code, err := dockerRequest(DockerConfig{Host: dockerHost}, http.MethodGet, "/containers/"+url.PathEscape(stack.APIContainerName)+"/json", "")
	if err != nil {
		status.Message = err.Error()
		return status, nil
	}
	if code == http.StatusNotFound {
		status.Status = manifestStatusStopped
		return status, nil
	}
	if code != http.StatusOK {
		status.Status = manifestStatusUnknown
		status.Message = fmt.Sprintf("docker inspect returned HTTP %d", code)
		return status, nil
	}
	if manifestDockerContainerRunning(data) {
		status.Status = manifestDockerRunningStatus
		status.Running = true
		if ok, msg := ProbeDograhHealth(ctx, stack); !ok {
			status.Status = manifestStatusStarting
			status.Running = false
			status.Message = msg
		}
	}
	return status, nil
}

// ProbeDograhHealth checks the Dograh API health endpoint, then falls back to TCP reachability.
func ProbeDograhHealth(ctx context.Context, stack DograhStackConfig) (bool, string) {
	base := strings.TrimRight(stack.InternalAPIURL, "/")
	if base == "" {
		return false, "Dograh API URL is not configured"
	}
	path := defaultString(stack.HealthPath, "/api/v1/health")
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	client := &http.Client{Timeout: dograhHealthProbeTimeout}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+path, nil)
	if err == nil {
		resp, doErr := client.Do(req)
		if doErr == nil {
			_ = resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 500 {
				return true, ""
			}
		}
	}
	parsed, err := url.Parse(base)
	if err != nil {
		return false, "Dograh health endpoint was not confirmed"
	}
	addr := parsed.Host
	if !strings.Contains(addr, ":") {
		addr += ":80"
	}
	conn, err := (&net.Dialer{Timeout: dograhHealthProbeTimeout}).DialContext(ctx, "tcp", addr)
	if err != nil {
		return false, "Dograh HTTP health endpoint and TCP fallback were not reachable"
	}
	_ = conn.Close()
	return true, "Dograh TCP port is reachable, but no HTTP health endpoint was confirmed"
}

// DograhAPIClient performs SDK-compatible Dograh REST calls using X-API-Key auth.
type DograhAPIClient struct {
	BaseURL    string
	APIKey     string
	HTTPClient *http.Client
}

func (c DograhAPIClient) do(ctx context.Context, method, path string, body interface{}, out interface{}) (int, error) {
	base := strings.TrimRight(strings.TrimSpace(c.BaseURL), "/")
	if base == "" {
		return 0, fmt.Errorf("Dograh API URL is required")
	}
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return 0, err
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, base+path, reader)
	if err != nil {
		return 0, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if strings.TrimSpace(c.APIKey) != "" {
		req.Header.Set("X-API-Key", strings.TrimSpace(c.APIKey))
	}
	client := c.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return resp.StatusCode, fmt.Errorf("Dograh API returned HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	if out != nil && len(raw) > 0 {
		if err := json.Unmarshal(raw, out); err != nil {
			return resp.StatusCode, err
		}
	}
	return resp.StatusCode, nil
}

// TestDograhConnection performs an SDK-equivalent read-only REST check.
func TestDograhConnection(ctx context.Context, apiURL, apiKey string) DograhConnectionTestResult {
	if strings.TrimSpace(apiKey) == "" {
		return DograhConnectionTestResult{Status: dograhStatusSetupRequired, Message: "Dograh API key is required"}
	}
	var nodeTypes []interface{}
	client := DograhAPIClient{BaseURL: apiURL, APIKey: apiKey}
	if _, err := client.do(ctx, http.MethodGet, "/api/v1/node-types", nil, &nodeTypes); err != nil {
		return DograhConnectionTestResult{Status: "error", Message: err.Error()}
	}
	return DograhConnectionTestResult{Status: "ok", NodeTypeCount: len(nodeTypes)}
}

// RegisterDograhAuraGoMCPTool creates a Dograh MCP tool pointing back to AuraGo's /mcp endpoint.
func RegisterDograhAuraGoMCPTool(ctx context.Context, dograh config.DograhConfig, auragoMCPURL string) (DograhToolRegistrationResult, error) {
	if strings.TrimSpace(dograh.APIKey) == "" {
		return DograhToolRegistrationResult{Status: dograhStatusSetupRequired, Message: "Dograh API key is required"}, nil
	}
	if strings.TrimSpace(dograh.AuraGoMCPCredentialUUID) == "" {
		return DograhToolRegistrationResult{Status: dograhStatusSetupRequired, Message: "Dograh credential UUID for AuraGo MCP is required"}, nil
	}
	if strings.TrimSpace(auragoMCPURL) == "" {
		return DograhToolRegistrationResult{Status: dograhStatusSetupRequired, Message: "AuraGo MCP URL is required"}, nil
	}
	name := defaultString(dograh.AuraGoMCPToolName, "AuraGo")
	body := map[string]interface{}{
		"name":        name,
		"description": "AuraGo MCP server exposed as a Dograh MCP tool",
		"category":    "mcp",
		"icon":        "plug",
		"icon_color":  "#2563eb",
		"definition": map[string]interface{}{
			"type": "mcp",
			"config": map[string]interface{}{
				"transport":             "streamable_http",
				"url":                   strings.TrimSpace(auragoMCPURL),
				"credential_uuid":       strings.TrimSpace(dograh.AuraGoMCPCredentialUUID),
				"tools_filter":          append([]string(nil), dograh.AuraGoMCPAllowedTools...),
				"timeout_secs":          30,
				"sse_read_timeout_secs": 300,
			},
		},
	}
	var out map[string]interface{}
	client := DograhAPIClient{BaseURL: dograh.APIURL, APIKey: dograh.APIKey}
	if _, err := client.do(ctx, http.MethodPost, "/api/v1/tools/", body, &out); err != nil {
		return DograhToolRegistrationResult{Status: "error", Message: err.Error()}, err
	}
	uuid := dograhToolUUID(out)
	if uuid != "" {
		_, _ = client.do(ctx, http.MethodPost, "/api/v1/tools/"+url.PathEscape(uuid)+"/mcp/refresh", nil, nil)
	}
	return DograhToolRegistrationResult{Status: "ok", ToolUUID: uuid}, nil
}

func dograhToolUUID(out map[string]interface{}) string {
	for _, key := range []string{"tool_uuid", "uuid", "id"} {
		if v, ok := out[key].(string); ok && strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	if data, ok := out["data"].(map[string]interface{}); ok {
		return dograhToolUUID(data)
	}
	return ""
}
