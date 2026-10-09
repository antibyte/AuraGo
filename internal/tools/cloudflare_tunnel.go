package tools

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"aurago/internal/config"
	"aurago/internal/dockerutil"
	"aurago/internal/fileutil"
	"aurago/internal/sandbox"
	"aurago/internal/security"

	"gopkg.in/yaml.v3"
)

var cfHTTPClient = &http.Client{Timeout: 30 * time.Second}

// CloudflareTunnelConfig holds the merged config for tunnel management.
type CloudflareTunnelConfig struct {
	Enabled              bool
	ReadOnly             bool
	Mode                 string // "auto", "docker", "native"
	AutoStart            bool
	AuthMethod           string // "token", "named", "quick"
	TunnelName           string
	AccountID            string
	ExposeWebUI          bool
	ExposeHomepage       bool
	CustomIngress        []CloudflareIngress
	MetricsPort          int
	LogLevel             string
	DockerEnabled        bool
	DockerHost           string // inherited from docker.host
	WebUIPort            int    // from server.port (plain HTTP port)
	HomepagePort         int    // from homepage.webserver_port
	DataDir              string // for storing config files
	HomepageEnabled      bool
	HomepageServing      bool
	HTTPRedirectPort     int
	HomepageWorkspace    string
	HomepageRegistryPath string
	QuickProjectDir      string
	quickOrigin          *homepageQuickOrigin

	configuredQuickProjectDir string // Saved selection, even when a tool explicitly chooses another project.
	// HTTPS fields: when HTTPS is enabled AuraGo no longer listens on WebUIPort.
	// The tunnel must connect to the HTTPS endpoint instead.
	HTTPSEnabled bool   // from server.https.enabled
	HTTPSPort    int    // from server.https.https_port (default 443)
	TunnelID     string // optional explicit tunnel UUID (for API-based noTLSVerify config)
	// LoopbackPort: when > 0, AuraGo also listens on http://127.0.0.1:LoopbackPort.
	// cloudflared uses this plain-HTTP endpoint instead of the HTTPS port so no
	// TLS verification is needed at all — the traffic stays on the loopback interface.
	LoopbackPort int // from cloudflare_tunnel.loopback_port
}

// CloudflareIngress mirrors config.CloudflareIngressRule for the tools package.
type CloudflareIngress struct {
	Hostname string
	Service  string
	Path     string
}

// CloudflareTunnelConfigFromConfig builds the runtime tool config from app config.
func CloudflareTunnelConfigFromConfig(cfg *config.Config) CloudflareTunnelConfig {
	if cfg == nil {
		return CloudflareTunnelConfig{}
	}
	cfgCopy := *cfg
	config.NormalizeCloudflareTunnelConfig(&cfgCopy)

	mode := strings.TrimSpace(cfgCopy.CloudflareTunnel.Mode)
	if !cfgCopy.Docker.Enabled && strings.EqualFold(mode, "auto") {
		mode = "native"
	}
	tunnelCfg := CloudflareTunnelConfig{
		Enabled:              cfgCopy.CloudflareTunnel.Enabled,
		HomepageEnabled:      cfgCopy.Homepage.Enabled,
		HomepageServing:      cfgCopy.Homepage.WebServerEnabled,
		HTTPRedirectPort:     cfgCopy.Server.HTTPS.HTTPPort,
		HomepageWorkspace:    cfgCopy.Homepage.WorkspacePath,
		HomepageRegistryPath: cfgCopy.SQLite.HomepageRegistryPath,
		QuickProjectDir:      cfgCopy.CloudflareTunnel.QuickProjectDir,
		ReadOnly:             cfgCopy.CloudflareTunnel.ReadOnly,
		Mode:                 mode,
		AutoStart:            cfgCopy.CloudflareTunnel.AutoStart,
		AuthMethod:           cfgCopy.CloudflareTunnel.AuthMethod,
		TunnelName:           cfgCopy.CloudflareTunnel.TunnelName,
		AccountID:            cfgCopy.CloudflareTunnel.AccountID,
		TunnelID:             cfgCopy.CloudflareTunnel.TunnelID,
		LoopbackPort:         cfgCopy.CloudflareTunnel.LoopbackPort,
		ExposeWebUI:          cfgCopy.CloudflareTunnel.ExposeWebUI,
		ExposeHomepage:       cfgCopy.CloudflareTunnel.ExposeHomepage,
		MetricsPort:          cfgCopy.CloudflareTunnel.MetricsPort,
		LogLevel:             cfgCopy.CloudflareTunnel.LogLevel,
		DockerEnabled:        cfgCopy.Docker.Enabled,
		DockerHost:           cfgCopy.Docker.Host,
		WebUIPort:            cfgCopy.Server.Port,
		HomepagePort:         cfgCopy.Homepage.WebServerPort,
		DataDir:              cfgCopy.Directories.DataDir,
		HTTPSEnabled:         cfgCopy.Server.HTTPS.Enabled,
		HTTPSPort:            cfgCopy.Server.HTTPS.HTTPSPort,

		configuredQuickProjectDir: cfgCopy.CloudflareTunnel.QuickProjectDir,
	}
	for _, r := range cfgCopy.CloudflareTunnel.CustomIngress {
		tunnelCfg.CustomIngress = append(tunnelCfg.CustomIngress, CloudflareIngress{
			Hostname: r.Hostname,
			Service:  r.Service,
			Path:     r.Path,
		})
	}
	if tunnelCfg.HomepageWorkspace == "" {
		tunnelCfg.HomepageWorkspace = filepath.Join(cfgCopy.Directories.DataDir, "homepage")
	}
	return tunnelCfg
}

const (
	cfdContainerName = "aurago-cloudflared"
	cfdImageName     = "cloudflare/cloudflared:2026.10.0@sha256:9b49eed8f62806d5d45ddf59ecefb5710429598ea6d3fcccd2af938f621b2b07"
	cfdBinaryName    = "cloudflared"
)

// tunnelState tracks the running cloudflared process/container.
var (
	tunnelMu          sync.Mutex
	tunnelPID         int    // native mode: PID in registry
	tunnelMode        string // "docker", "native", "quick", or ""
	tunnelURL         string // quick tunnel: the public URL
	tunnelStarted     time.Time
	tunnelStopping    bool
	tunnelQuickOrigin *homepageQuickOrigin
	tunnelExit        <-chan struct{}
	tunnelDockerHost  string
	tunnelContainerID string
	tunnelAuth        string
	tunnelWarnings    []cloudflareWarning
)

var cloudflareRandRead = rand.Read

func cloudflareTunnelReadOnlyError(cfg CloudflareTunnelConfig) string {
	if !cfg.ReadOnly {
		return ""
	}
	return errJSON("Cloudflare Tunnel is in read-only mode. Disable cloudflare_tunnel.readonly to allow changes.")
}

// ──────────────────────────────────────────────────────────────────────────
// Lifecycle
// ──────────────────────────────────────────────────────────────────────────

// CloudflareTunnelStart starts the cloudflared tunnel.
func CloudflareTunnelStart(cfg CloudflareTunnelConfig, vault *security.Vault, registry *ProcessRegistry, logger *slog.Logger) string {
	if msg := cloudflareTunnelReadOnlyError(cfg); msg != "" {
		return msg
	}
	tunnelLifecycleMu.Lock()
	defer tunnelLifecycleMu.Unlock()
	result := startCloudflareLocked(cfg, vault, registry, logger)
	if warnings := cloudflareSnapshot().Warnings; len(warnings) > 0 {
		if body, ok := decodeCloudflareTunnelToolResult(result).(map[string]interface{}); ok {
			body["warnings"] = warnings
			out, _ := json.Marshal(body)
			return string(out)
		}
	}
	return result
}

func startCloudflareLocked(cfg CloudflareTunnelConfig, vault *security.Vault, registry *ProcessRegistry, logger *slog.Logger) string {
	if msg := cloudflarePolicyError(cfg, true); msg != "" {
		return msg
	}
	if err := validateCloudflareRuntime(cfg); err != nil {
		return errJSON("%v", err)
	}
	if result := reconcileCloudflareLocked(cfg, logger, true); !cloudflareTunnelToolResultOK(result) {
		return result
	}
	if state := cloudflareSnapshot(); state.Mode != "" {
		if state.Mode == "docker" {
			container, err := inspectCloudflareContainer(state.Host, state.ContainerID)
			if err != nil {
				return errJSON("%v", err)
			}
			if container == nil || !container.State.Running {
				if result := stopDockerTunnel(cfg, logger); !cloudflareTunnelToolResultOK(result) {
					return result
				}
			} else {
				return errJSON("Tunnel already running (mode=%s). Stop it first.", state.Mode)
			}
		} else {
			return errJSON("Tunnel already running (mode=%s). Stop it first.", state.Mode)
		}
	}
	tunnelMu.Lock()
	tunnelWarnings = nil
	tunnelMu.Unlock()
	switch cfg.AuthMethod {
	case "token":
		return startTokenTunnel(cfg, vault, registry, logger)
	case "named":
		return startNamedTunnel(cfg, vault, registry, logger)
	case "quick":
		return startQuickTunnel(cfg, registry, logger, 0)
	default:
		return errJSON("Unknown auth_method: %q. Use: token, named, quick", cfg.AuthMethod)
	}
}

// CloudflareTunnelStop stops the running tunnel.
func CloudflareTunnelStop(cfg CloudflareTunnelConfig, registry *ProcessRegistry, logger *slog.Logger) string {
	if msg := cloudflareTunnelReadOnlyError(cfg); msg != "" {
		return msg
	}
	tunnelLifecycleMu.Lock()
	defer tunnelLifecycleMu.Unlock()
	if msg := cloudflarePolicyError(cfg, false); msg != "" {
		return msg
	}
	return stopCloudflareTunnelLocked(cfg, registry, logger)
}

// CloudflareTunnelShutdown is reserved for server shutdown and permission revocation.
func CloudflareTunnelShutdown(cfg CloudflareTunnelConfig, registry *ProcessRegistry, logger *slog.Logger, quickOnly bool) string {
	tunnelLifecycleMu.Lock()
	defer tunnelLifecycleMu.Unlock()
	state := cloudflareSnapshot()
	if quickOnly && state.Origin == nil && state.Auth != "quick" {
		return okJSON("No managed quick publication is active")
	}
	return stopCloudflareTunnelLocked(cfg, registry, logger)
}

func stopCloudflareTunnelLocked(cfg CloudflareTunnelConfig, registry *ProcessRegistry, logger *slog.Logger) string {
	state := cloudflareSnapshot()
	if state.Origin != nil {
		state.Origin.disabled.Store(true)
	}
	if state.Mode == "" || state.Mode == "docker" && state.ContainerID == "" {
		if result := reconcileCloudflareLocked(cfg, logger, false); !cloudflareTunnelToolResultOK(result) {
			return result
		}
		state = cloudflareSnapshot()
		if state.Mode == "" {
			clearCloudflareState()
			return okJSON("No tunnel is running")
		}
	}
	tunnelMu.Lock()
	tunnelStopping = true
	tunnelMu.Unlock()
	defer func() { tunnelMu.Lock(); tunnelStopping = false; tunnelMu.Unlock() }()
	var result string
	switch state.Mode {
	case "docker":
		result = stopDockerTunnel(cfg, logger)
	case "native", "quick":
		result = stopNativeTunnel(cfg, registry, logger)
	default:
		return errJSON("Unknown tunnel mode: %s", state.Mode)
	}
	// Keep a disabled listener bound when termination is uncertain. Releasing
	// its port could let another local service inherit the surviving tunnel.
	if cloudflareTunnelToolResultOK(result) {
		clearCloudflareState()
	}
	return result
}

// CloudflareTunnelRestart stops and restarts the tunnel.
func CloudflareTunnelRestart(cfg CloudflareTunnelConfig, vault *security.Vault, registry *ProcessRegistry, logger *slog.Logger) string {
	if msg := cloudflareTunnelReadOnlyError(cfg); msg != "" {
		return msg
	}
	tunnelLifecycleMu.Lock()
	defer tunnelLifecycleMu.Unlock()
	if msg := cloudflarePolicyError(cfg, true); msg != "" {
		return msg
	}
	if err := validateCloudflareRuntime(cfg); err != nil {
		return errJSON("%v", err)
	}
	cfg.Mode = resolveMode(cfg)
	if cfg.AuthMethod != "quick" {
		key := "cloudflared_token"
		if cfg.AuthMethod == "named" {
			key = "cloudflared_credentials"
		}
		if vault == nil {
			return errJSON("Cloudflare credentials vault is unavailable")
		}
		credential, err := vault.ReadSecret(key)
		if err != nil || credential == "" {
			return errJSON("Cloudflare credentials are missing; the existing tunnel was not stopped")
		}
	}
	if cfg.Mode == "native" && findCloudflaredBinary(cfg.DataDir) == "" {
		if _, err := cloudflaredDownloadMetadata(runtime.GOOS, runtime.GOARCH); err != nil {
			return errJSON("%v", err)
		}
	}
	if cfg.AuthMethod == "quick" {
		// Validate before interrupting an existing tunnel.
		probe, err := newHomepageQuickOrigin(cfg, cfg.Mode)
		if err != nil {
			return errJSON("%v", err)
		}
		defer probe.Close()
		if cfg.Mode == "docker" {
			if _, _, err := cloudflareQuickDockerNetwork(cfg); err != nil {
				return errJSON("%v", err)
			}
		}
	}

	stopResult := stopCloudflareTunnelLocked(cfg, registry, logger)
	startResult := errJSON("Restart cancelled because tunnel termination is unconfirmed")
	if cloudflareTunnelToolResultOK(stopResult) {
		startResult = startCloudflareLocked(cfg, vault, registry, logger)
	}
	status := "ok"
	message := "Cloudflare tunnel restarted"
	if !cloudflareTunnelToolResultOK(startResult) {
		status = "error"
		message = cloudflareTunnelToolResultMessage(startResult)
		if message == "" {
			message = "Cloudflare tunnel restart failed"
		}
	}
	result := map[string]interface{}{
		"status":   status,
		"message":  message,
		"stop":     decodeCloudflareTunnelToolResult(stopResult),
		"start":    decodeCloudflareTunnelToolResult(startResult),
		"warnings": cloudflareSnapshot().Warnings,
	}
	if status != "ok" {
		result["error"] = message
	}
	out, _ := json.Marshal(result)
	return string(out)
}

func decodeCloudflareTunnelToolResult(raw string) interface{} {
	var value interface{}
	if err := json.Unmarshal([]byte(raw), &value); err == nil {
		return value
	}
	return raw
}

func cloudflareTunnelToolResultOK(raw string) bool {
	var body map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &body); err != nil {
		return false
	}
	status, _ := body["status"].(string)
	return status == "ok"
}

func cloudflareTunnelToolResultMessage(raw string) string {
	var body map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &body); err != nil {
		return ""
	}
	if msg, _ := body["error"].(string); msg != "" {
		return msg
	}
	if msg, _ := body["message"].(string); msg != "" {
		return msg
	}
	return ""
}

// CloudflareTunnelStatus returns the current tunnel status.
func CloudflareTunnelStatus(cfg CloudflareTunnelConfig, registry *ProcessRegistry, logger *slog.Logger) string {
	out, _ := json.Marshal(cloudflareStatusResult(cfg, registry))
	return string(out)
}

// CloudflareTunnelQuickTunnel publishes a registered Homepage static snapshot.
// Quick tunnels use TryCloudflare and don't require any Cloudflare account.
func CloudflareTunnelQuickTunnel(cfg CloudflareTunnelConfig, registry *ProcessRegistry, logger *slog.Logger, port int) string {
	if msg := cloudflareTunnelReadOnlyError(cfg); msg != "" {
		return msg
	}
	if port != 0 {
		return errJSON("quick_tunnel no longer accepts a port; select a registered project_dir")
	}
	cfg.AuthMethod = "quick"
	return CloudflareTunnelStart(cfg, nil, registry, logger)
}

// CloudflareTunnelLogs returns recent log output from the tunnel process.
func CloudflareTunnelLogs(registry *ProcessRegistry, logger *slog.Logger) string {
	state := cloudflareSnapshot()

	if state.Mode == "" {
		return errJSON("No tunnel running")
	}

	if state.Mode == "docker" {
		// Docker logs are not captured in ProcessRegistry; hint user to use docker logs
		out, _ := json.Marshal(map[string]interface{}{
			"status":  "ok",
			"message": "Tunnel running in Docker mode. Use docker tool to view container logs.",
			"mode":    "docker",
		})
		return string(out)
	}

	if state.PID <= 0 {
		return errJSON("No process PID tracked")
	}
	info, ok := registry.Get(state.PID)
	if !ok {
		return errJSON("Process %d not found in registry", state.PID)
	}

	output := info.ReadOutput()
	// Truncate to last 4KB for readability
	if len(output) > 4096 {
		start := len(output) - 4096
		for start < len(output) && !utf8.RuneStart(output[start]) {
			start++
		}
		output = output[start:]
	}
	out, _ := json.Marshal(map[string]interface{}{
		"status": "ok",
		"pid":    state.PID,
		"logs":   output,
	})
	return string(out)
}

// CloudflareTunnelListRoutes returns the currently configured ingress rules.
func CloudflareTunnelListRoutes(cfg CloudflareTunnelConfig, logger *slog.Logger) string {
	rules := buildIngressRules(cfg)
	out, _ := json.Marshal(map[string]interface{}{
		"status": "ok",
		"rules":  rules,
	})
	return string(out)
}

// CloudflareTunnelInstall downloads the cloudflared binary for the current platform.
func CloudflareTunnelInstall(cfg CloudflareTunnelConfig, logger *slog.Logger) string {
	if msg := cloudflareTunnelReadOnlyError(cfg); msg != "" {
		return msg
	}
	binPath := cfdBinaryPath(cfg.DataDir)
	return installCloudflaredBinary(binPath, logger)
}

// ──────────────────────────────────────────────────────────────────────────
// Token Tunnel (Connector Token via CF Dashboard)
// ──────────────────────────────────────────────────────────────────────────
// Cloudflare API – programmatic tunnel configuration
// ──────────────────────────────────────────────────────────────────────────

// cfTunnelConfig is the remotely-managed cloudflared ingress/origin config.
// Preserve provider fields when updating one origin's TLS policy.
type cfTunnelConfig map[string]any

type cfAPIError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type cfAPIGenericResponse struct {
	Success bool         `json:"success"`
	Errors  []cfAPIError `json:"errors"`
}

type cfTunnelListResponse struct {
	Result  []cfTunnelListItem `json:"result"`
	Success bool               `json:"success"`
	Errors  []cfAPIError       `json:"errors"`
}

type cfTunnelListItem struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type cfTunnelConfigResponse struct {
	Result  cfTunnelConfigResult `json:"result"`
	Success bool                 `json:"success"`
	Errors  []cfAPIError         `json:"errors"`
}

type cfTunnelConfigResult struct {
	Config cfTunnelConfig `json:"config"`
}

// applyNoTLSVerifyViaAPI reads the current remotely-managed tunnel configuration
// from the Cloudflare API and scopes noTLSVerify to AuraGo local HTTPS rules. This is necessary when HTTPS is enabled because cloudflared
// overrides local CLI flags with the Dashboard-pushed ingress configuration.
//
// The vault secret "cloudflare_api_token" must be set with a Zero Trust write
// capable API token. Failures produce sanitized warnings without blocking the connector.
func applyNoTLSVerifyViaAPI(ctx context.Context, cfg CloudflareTunnelConfig, apiToken string, logger *slog.Logger) *cloudflareWarning {
	warning := func(code string) *cloudflareWarning {
		return &cloudflareWarning{Code: code, Message: "TLS origin configuration could not be verified; configure a loopback HTTP origin or check the Cloudflare origin settings."}
	}
	if cfg.AccountID == "" || apiToken == "" || cfg.TunnelID == "" && cfg.TunnelName == "" {
		return warning("tls_prerequisites")
	}
	security.RegisterSensitive(apiToken)
	tunnelID := cfg.TunnelID
	if tunnelID == "" {
		var err error
		tunnelID, err = cfLookupTunnelID(ctx, cfg.AccountID, apiToken, cfg.TunnelName)
		if err != nil {
			if ctx.Err() != nil {
				return warning("tls_timeout")
			}
			return warning("tls_lookup")
		}
	}
	current, err := cfGetTunnelConfig(ctx, cfg.AccountID, apiToken, tunnelID)
	if err != nil {
		if ctx.Err() != nil {
			return warning("tls_timeout")
		}
		return warning("tls_get")
	}
	matched := false
	if rules, ok := (*current)["ingress"].([]any); ok {
		for _, value := range rules {
			if rule, ok := value.(map[string]any); ok {
				service, _ := rule["service"].(string)
				matched = matched || cloudflareLocalHTTPSOrigin(cfg, service)
			}
		}
	}
	if !matched {
		return warning("tls_origin")
	}
	if !scopeCloudflareTLSException(*current, cfg) {
		return nil
	}
	if err := cfPutTunnelConfig(ctx, cfg.AccountID, apiToken, tunnelID, current); err != nil {
		if ctx.Err() != nil {
			return warning("tls_timeout")
		}
		return warning("tls_put")
	}
	return nil
}

// cloudflareLocalHTTPSOrigin matches only the configured AuraGo TLS listener.
// A port match alone must never disable TLS for a different upstream host.
func cloudflareLocalHTTPSOrigin(cfg CloudflareTunnelConfig, service string) bool {
	if !cfg.HTTPSEnabled || cfg.LoopbackPort != 0 {
		return false
	}
	u, err := url.Parse(service)
	if err != nil || u.Scheme != "https" || u.User != nil {
		return false
	}
	switch strings.ToLower(u.Hostname()) {
	case "localhost", "127.0.0.1", "::1", "host.docker.internal":
	default:
		return false
	}
	port := u.Port()
	if port == "" {
		port = "443"
	}
	return port == fmt.Sprint(effectiveHTTPSPort(cfg))
}

func scopeCloudflareTLSException(current cfTunnelConfig, cfg CloudflareTunnelConfig) bool {
	rules, ok := current["ingress"].([]any)
	if !ok {
		return false
	}
	changed, matched := false, false
	for _, value := range rules {
		rule, ok := value.(map[string]any)
		if !ok {
			continue
		}
		service, _ := rule["service"].(string)
		if !cloudflareLocalHTTPSOrigin(cfg, service) {
			continue
		}
		matched = true
		origin, _ := rule["originRequest"].(map[string]any)
		if origin == nil {
			origin = make(map[string]any)
			rule["originRequest"] = origin
		}
		if origin["noTLSVerify"] != true {
			origin["noTLSVerify"] = true
			changed = true
		}
	}
	// Migrate the former global exception only when this is an AuraGo tunnel.
	if matched {
		if origin, ok := current["originRequest"].(map[string]any); ok && origin["noTLSVerify"] == true {
			delete(origin, "noTLSVerify")
			changed = true
		}
	}
	return changed
}

// cfLookupTunnelID resolves a tunnel UUID by its display name.
func cfLookupTunnelID(ctx context.Context, accountID, apiToken, name string) (string, error) {
	reqURL := fmt.Sprintf(
		"https://api.cloudflare.com/client/v4/accounts/%s/cfd_tunnel?name=%s&is_deleted=false",
		url.PathEscape(accountID),
		url.QueryEscape(name),
	)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+apiToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := cfHTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var r cfTunnelListResponse
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return "", fmt.Errorf("decode: %w", err)
	}
	if !r.Success {
		return "", fmt.Errorf("API error: %+v", r.Errors)
	}
	if len(r.Result) == 0 {
		return "", fmt.Errorf("no tunnel found with name %q", name)
	}
	return r.Result[0].ID, nil
}

// cfGetTunnelConfig fetches the current remotely-managed config for a tunnel.
func cfGetTunnelConfig(ctx context.Context, accountID, apiToken, tunnelID string) (*cfTunnelConfig, error) {
	reqURL := fmt.Sprintf(
		"https://api.cloudflare.com/client/v4/accounts/%s/cfd_tunnel/%s/configurations",
		url.PathEscape(accountID),
		url.PathEscape(tunnelID),
	)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+apiToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := cfHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var r cfTunnelConfigResponse
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	if !r.Success {
		return nil, fmt.Errorf("API error: %+v", r.Errors)
	}
	return &r.Result.Config, nil
}

// cfPutTunnelConfig replaces the remotely-managed config for a tunnel.
func cfPutTunnelConfig(ctx context.Context, accountID, apiToken, tunnelID string, config *cfTunnelConfig) error {
	body, err := json.Marshal(map[string]any{"config": config})
	if err != nil {
		return err
	}
	reqURL := fmt.Sprintf(
		"https://api.cloudflare.com/client/v4/accounts/%s/cfd_tunnel/%s/configurations",
		url.PathEscape(accountID),
		url.PathEscape(tunnelID),
	)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, reqURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+apiToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := cfHTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var r cfAPIGenericResponse
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return fmt.Errorf("decode: %w", err)
	}
	if !r.Success {
		return fmt.Errorf("API error: %+v", r.Errors)
	}
	return nil
}

// ──────────────────────────────────────────────────────────────────────────

// buildLocalURL returns the URL cloudflared should use to reach AuraGo.
// When LoopbackPort is set the tunnel uses a plain-HTTP loopback endpoint
// (http://127.0.0.1:LoopbackPort) regardless of whether HTTPS is enabled,
// so no TLS verification is required on the cloudflared side.
//
// For local-origin tunnel modes the --url flag accepts a SINGLE origin URL; all
// traffic at the Cloudflare edge is forwarded there regardless of the
// incoming hostname. That is why ExposeWebUI and ExposeHomepage are mutually
// exclusive in those modes:
//   - If only ExposeHomepage is true → use HomepagePort (plain HTTP)
//   - Otherwise (ExposeWebUI or default) → use WebUIPort / HTTPS / Loopback
func buildLocalURL(cfg CloudflareTunnelConfig, host string) string {
	// Loopback port always wins — plain HTTP on 127.0.0.1 (host-network Docker or process).
	// The loopback listener inside AuraGo dynamically routes to Web UI or Homepage
	// based on the current config, so cloudflared never needs to change its target URL.
	if cfg.LoopbackPort > 0 {
		return fmt.Sprintf("http://127.0.0.1:%d", cfg.LoopbackPort)
	}
	// No loopback port: route directly to Homepage or Web UI.
	if cfg.ExposeHomepage && !cfg.ExposeWebUI && cfg.HomepagePort > 0 {
		return fmt.Sprintf("http://%s:%d", host, cfg.HomepagePort)
	}
	if cfg.HTTPSEnabled {
		port := cfg.HTTPSPort
		if port <= 0 {
			port = 443
		}
		return fmt.Sprintf("https://%s:%d", host, port)
	}
	return fmt.Sprintf("http://%s:%d", host, cfg.WebUIPort)
}

func tokenTunnelArgs() []string {
	return []string{"tunnel", "run"}
}

func startTokenTunnel(cfg CloudflareTunnelConfig, vault *security.Vault, registry *ProcessRegistry, logger *slog.Logger) string {
	if vault == nil {
		return errJSON("Cloudflare credentials vault is unavailable")
	}
	token, err := vault.ReadSecret("cloudflared_token")
	if err != nil || token == "" {
		return errJSON("Cloudflare connector token not found in vault. Store it with key 'cloudflared_token' via the Config UI.")
	}

	if cfg.HTTPSEnabled && cfg.LoopbackPort == 0 {
		apiToken, _ := vault.ReadSecret("cloudflare_api_token")
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		warning := applyNoTLSVerifyViaAPI(ctx, cfg, apiToken, logger)
		cancel()
		if warning != nil {
			tunnelMu.Lock()
			tunnelWarnings = []cloudflareWarning{*warning}
			tunnelMu.Unlock()
		}
	}
	security.RegisterSensitive(token)

	mode := resolveMode(cfg)
	logger.Info("[CloudflareTunnel] Starting token tunnel", "mode", mode)

	tunnelArgs := tokenTunnelArgs()

	switch mode {
	case "docker":
		containerEnv := []string{"TUNNEL_TOKEN=" + token}
		return startDockerTunnel(cfg, tunnelArgs, containerEnv, nil, logger)
	case "native":
		// Pass token as env var to avoid exposure in process listings (ps aux / /proc)
		return startNativeTunnel(cfg, registry, tunnelArgs, []string{"TUNNEL_TOKEN=" + token}, logger)
	default:
		return errJSON("Could not determine runtime mode. Docker not available and native binary not found.")
	}
}

// ──────────────────────────────────────────────────────────────────────────
// Named Tunnel (credentials.json)
// ──────────────────────────────────────────────────────────────────────────

func startNamedTunnel(cfg CloudflareTunnelConfig, vault *security.Vault, registry *ProcessRegistry, logger *slog.Logger) string {
	if vault == nil {
		return errJSON("Cloudflare credentials vault is unavailable")
	}
	if cfg.TunnelName == "" {
		return errJSON("tunnel_name is required for named tunnel auth method")
	}

	// Read credentials from vault
	credJSON, err := vault.ReadSecret("cloudflared_credentials")
	if err != nil || credJSON == "" {
		return errJSON("Cloudflare tunnel credentials not found in vault. Store the credentials.json content with key 'cloudflared_credentials'.")
	}

	// Write credentials to a temp file
	credDir := filepath.Join(cfg.DataDir, "cloudflared")
	if err := os.MkdirAll(credDir, 0700); err != nil {
		return errJSON("Failed to create cloudflared config dir: %v", err)
	}
	credPath := filepath.Join(credDir, "credentials.json")
	if err := os.WriteFile(credPath, []byte(credJSON), 0600); err != nil {
		return errJSON("Failed to write credentials file: %v", err)
	}

	// Generate config with ingress rules
	configPath := filepath.Join(credDir, "config.yml")
	if err := writeNamedTunnelConfig(cfg, credPath, configPath); err != nil {
		return errJSON("Failed to write tunnel config: %v", err)
	}

	mode := resolveMode(cfg)
	logger.Info("[CloudflareTunnel] Starting named tunnel", "mode", mode, "tunnel", cfg.TunnelName)

	switch mode {
	case "docker":
		return startDockerNamedTunnel(cfg, credDir, configPath, logger)
	case "native":
		return startNativeTunnel(cfg, registry, []string{
			"tunnel", "--config", configPath, "run", cfg.TunnelName,
		}, nil, logger)
	default:
		return errJSON("Could not determine runtime mode. Docker not available and native binary not found.")
	}
}

// ──────────────────────────────────────────────────────────────────────────
// Quick Tunnel (TryCloudflare, no account)
// ──────────────────────────────────────────────────────────────────────────

func effectiveHTTPSPort(cfg CloudflareTunnelConfig) int {
	if cfg.HTTPSPort > 0 {
		return cfg.HTTPSPort
	}
	return 443
}

func quickTunnelOriginURL(cfg CloudflareTunnelConfig, host string, port int) string {
	if cfg.quickOrigin == nil || port != 0 {
		return ""
	}
	return cfg.quickOrigin.URL(host)
}

func quickTunnelArgs(cfg CloudflareTunnelConfig, host string, port int) []string {
	args := []string{"tunnel", "--url", quickTunnelOriginURL(cfg, host, port)}
	if cfg.quickOrigin != nil {
		args = append(args, "--http-host-header", cfg.quickOrigin.hostHeader)
	}
	return args
}

func startQuickTunnel(cfg CloudflareTunnelConfig, registry *ProcessRegistry, logger *slog.Logger, port int) string {
	if port != 0 {
		return errJSON("quick_tunnel no longer accepts a port; select a registered project_dir")
	}
	mode := resolveMode(cfg)
	if mode != "native" && mode != "docker" {
		return errJSON("No cloudflared runtime is available")
	}
	origin, err := newHomepageQuickOrigin(cfg, mode)
	if err != nil {
		return errJSON("%v", err)
	}
	cfg.quickOrigin = origin
	if _, err := RecordHomepageEvent(origin.db, origin.projectID, "quick_publication_requested", "cloudflare_tunnel", "Publish immutable static snapshot", map[string]interface{}{"publication_id": origin.publicationID, "artifact_hash": origin.artifactHash, "build_dir": origin.buildDir}); err != nil {
		origin.Close()
		return errJSON("Cannot record Homepage publication: %v", err)
	}
	logger.Info("[CloudflareTunnel] Publishing managed Homepage snapshot", "project_dir", origin.projectDir, "mode", mode)
	var result string
	if mode == "docker" {
		result = startDockerQuickTunnel(cfg, 0, logger)
	} else {
		result = startNativeQuickTunnel(cfg, registry, quickTunnelArgs(cfg, "127.0.0.1", 0), logger)
	}
	tunnelMu.Lock()
	retain := tunnelMode == "docker" || cloudflareTunnelToolResultOK(result) && tunnelMode == "native" && tunnelQuickOrigin == origin
	if tunnelMode == "docker" {
		// A lost start response may still leave a live container. Reserve the
		// port until stopping that exact daemon confirms termination.
		if !cloudflareTunnelToolResultOK(result) {
			origin.disabled.Store(true)
		}
		tunnelQuickOrigin = origin
	}
	tunnelMu.Unlock()
	if !retain {
		origin.Close()
	}
	return result
}

// ──────────────────────────────────────────────────────────────────────────
// Docker Backend
// ──────────────────────────────────────────────────────────────────────────

func startDockerTunnel(cfg CloudflareTunnelConfig, cmd []string, containerEnv []string, extraBinds []string, logger *slog.Logger) string {
	dockerCfg := DockerConfig{Host: cfg.DockerHost}

	// Pull image if not present
	pullImage(dockerCfg, cfdImageName, logger)

	// Remove old container if exists
	if result := stopCloudflareDockerBeforeCreate(cfg, logger); !cloudflareTunnelToolResultOK(result) {
		return result
	}

	hostCfg := map[string]interface{}{
		"NetworkMode":   "host",
		"RestartPolicy": map[string]string{"Name": "unless-stopped"},
	}
	if len(extraBinds) > 0 {
		hostCfg["Binds"] = extraBinds
	}

	payload := map[string]interface{}{
		"Image":      cfdImageName,
		"Cmd":        cmd,
		"HostConfig": hostCfg,
	}
	if len(containerEnv) > 0 {
		payload["Env"] = containerEnv
	}
	payload["Cmd"] = cloudflareRuntimeArgs(cfg, cmd)

	return createAndStartContainer(dockerCfg, cfdContainerName, payload, logger, "token")
}

func startDockerNamedTunnel(cfg CloudflareTunnelConfig, credDir, configPath string, logger *slog.Logger) string {
	dockerCfg := DockerConfig{Host: cfg.DockerHost}

	pullImage(dockerCfg, cfdImageName, logger)
	if result := stopCloudflareDockerBeforeCreate(cfg, logger); !cloudflareTunnelToolResultOK(result) {
		return result
	}

	payload := map[string]interface{}{
		"Image": cfdImageName,
		"Cmd":   cloudflareRuntimeArgs(cfg, []string{"tunnel", "--config", "/etc/cloudflared/config.yml", "run", cfg.TunnelName}),
		"HostConfig": map[string]interface{}{
			"NetworkMode":   "host",
			"RestartPolicy": map[string]string{"Name": "unless-stopped"},
			"Binds": []string{
				dockerutil.FormatBindMount(credDir, "/etc/cloudflared", "ro"),
			},
		},
	}

	return createAndStartContainer(dockerCfg, cfdContainerName, payload, logger, "named")
}

func startDockerQuickTunnel(cfg CloudflareTunnelConfig, port int, logger *slog.Logger) string {
	if cfg.quickOrigin == nil || port != 0 {
		return errJSON("A managed Homepage origin is required")
	}
	dockerCfg := DockerConfig{Host: cfg.DockerHost}

	host, hostCfg, err := cloudflareQuickDockerNetwork(cfg)
	if err != nil {
		return errJSON("%v", err)
	}
	cmd := cloudflareRuntimeArgs(cfg, quickTunnelArgs(cfg, host, port))

	payload := map[string]interface{}{
		"Image":      cfdImageName,
		"Cmd":        cmd,
		"HostConfig": hostCfg,
	}

	pullImage(dockerCfg, cfdImageName, logger)
	if result := stopCloudflareDockerBeforeCreate(cfg, logger); !cloudflareTunnelToolResultOK(result) {
		return result
	}
	result := createAndStartContainer(dockerCfg, cfdContainerName, payload, logger, "quick")
	if !cloudflareTunnelToolResultOK(result) {
		return result
	}

	// Try to capture the quick tunnel URL from container logs after a few seconds
	containerID := cloudflareSnapshot().ContainerID
	go func() {
		time.Sleep(5 * time.Second)
		url := captureQuickTunnelURLDocker(dockerCfg, containerID, logger)
		if url != "" {
			recordCloudflareQuickURL(cfg.quickOrigin, url, logger)
		}
	}()

	return result
}

func createAndStartContainer(dockerCfg DockerConfig, name string, payload map[string]interface{}, logger *slog.Logger, authMethod string) string {
	payload["Labels"] = map[string]string{"aurago.managed-by": "cloudflare_tunnel", "aurago.cloudflare.auth": authMethod}
	body, _ := json.Marshal(payload)
	// A lost create response can still leave a connector; retain its engine and origin.
	tunnelMu.Lock()
	tunnelMode, tunnelDockerHost, tunnelAuth = "docker", dockerutil.NormalizeHost(dockerCfg.Host), authMethod
	tunnelContainerID = ""
	tunnelMu.Unlock()
	data, code, err := dockerRequest(dockerCfg, "POST", "/containers/create?name="+name, string(body))
	if err != nil {
		return errJSON("Failed to create cloudflared container: %v", err)
	}
	if code != 201 {
		return errJSON("Failed to create cloudflared container: HTTP %d — %s", code, string(data))
	}

	var created struct {
		ID string `json:"Id"`
	}
	if json.Unmarshal(data, &created) != nil || len(created.ID) != 64 || strings.Trim(created.ID, "0123456789abcdef") != "" {
		return errJSON("Cloudflare container creation identity is unconfirmed")
	}
	tunnelMu.Lock()
	tunnelContainerID, tunnelAuth = created.ID, authMethod
	tunnelMode = "docker"
	tunnelDockerHost = dockerutil.NormalizeHost(dockerCfg.Host)
	tunnelStarted = time.Now()
	tunnelPID = 0
	tunnelMu.Unlock()
	_, startCode, startErr := dockerRequest(dockerCfg, "POST", "/containers/"+created.ID+"/start", "")
	if startErr != nil || (startCode != 204 && startCode != 304) {
		return errJSON("Cloudflared start is unconfirmed; stop the tunnel before retrying: code=%d err=%v", startCode, startErr)
	}

	logger.Info("[CloudflareTunnel] Container started", "container", name, "auth", authMethod)

	out, _ := json.Marshal(map[string]interface{}{
		"status":    "ok",
		"message":   "Cloudflare tunnel started (Docker)",
		"container": name,
		"mode":      "docker",
		"auth":      authMethod,
		"warnings":  cloudflareSnapshot().Warnings,
	})
	return string(out)
}

func stopDockerTunnel(cfg CloudflareTunnelConfig, logger *slog.Logger) string {
	state := cloudflareSnapshot()
	if state.ContainerID == "" {
		return errJSON("Tunnel container identity is unconfirmed")
	}
	dockerCfg := DockerConfig{Host: state.Host}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	_, code, err := DockerRequestContext(ctx, dockerCfg, "POST", "/containers/"+state.ContainerID+"/stop?t=10", "")
	if err != nil || code != 204 && code != 304 && code != 404 {
		return errJSON("Tunnel termination is unconfirmed: HTTP %d", code)
	}
	_, code, err = DockerRequestContext(ctx, dockerCfg, "DELETE", "/containers/"+state.ContainerID, "")
	if err != nil || code != 204 && code != 404 {
		return errJSON("Tunnel removal is unconfirmed: HTTP %d", code)
	}
	clearCloudflareState()
	if err := os.Remove(filepath.Join(cfg.DataDir, "cloudflared", "credentials.json")); err != nil && !os.IsNotExist(err) {
		logger.Warn("[CloudflareTunnel] Credential cleanup failed", "error", err)
	}
	return okJSON("Cloudflare tunnel stopped")
}

// ──────────────────────────────────────────────────────────────────────────
// Native Binary Backend
// ──────────────────────────────────────────────────────────────────────────

func startNativeTunnel(cfg CloudflareTunnelConfig, registry *ProcessRegistry, args []string, extraEnv []string, logger *slog.Logger) string {
	binPath := findCloudflaredBinary(cfg.DataDir)
	if binPath == "" {
		// Try auto-install
		logger.Info("[CloudflareTunnel] Binary not found, attempting auto-install")
		installResult := installCloudflaredBinary(cfdBinaryPath(cfg.DataDir), logger)
		var ir map[string]interface{}
		if json.Unmarshal([]byte(installResult), &ir) == nil {
			if s, _ := ir["status"].(string); s == "error" {
				return installResult
			}
		}
		binPath = findCloudflaredBinary(cfg.DataDir)
		if binPath == "" {
			return errJSON("cloudflared binary not found after install attempt")
		}
	}

	args = cloudflareRuntimeArgs(cfg, args)

	cmd := exec.Command(binPath, args...)
	info := &ProcessInfo{
		StartedAt: time.Now(),
		Alive:     true,
	}
	cmd.Stdout = info
	cmd.Stderr = info
	cmd.Env = sandbox.FilterEnv(os.Environ())
	if len(extraEnv) > 0 {
		cmd.Env = append(cmd.Env, extraEnv...)
	}

	if err := cmd.Start(); err != nil {
		return errJSON("Failed to start cloudflared: %v", err)
	}

	info.PID = cmd.Process.Pid
	info.Process = cmd.Process
	registry.Register(info)

	tunnelMu.Lock()
	tunnelAuth = cfg.AuthMethod
	tunnelMode = "native"
	tunnelPID = cmd.Process.Pid
	tunnelStarted = time.Now()
	tunnelQuickOrigin = cfg.quickOrigin
	logger.Info("[CloudflareTunnel] Native process started", "pid", cmd.Process.Pid)

	// Wait for the actual process exit before releasing a managed origin port.
	exited := make(chan struct{})
	tunnelExit = exited
	tunnelMu.Unlock()
	go func() {
		_ = cmd.Wait()
		close(exited)
		info.mu.Lock()
		info.Alive = false
		info.mu.Unlock()

		var origin *homepageQuickOrigin
		tunnelMu.Lock()
		if !tunnelStopping && tunnelPID == cmd.Process.Pid {
			origin, tunnelQuickOrigin = tunnelQuickOrigin, nil
			tunnelMode = ""
			tunnelPID = 0
			tunnelURL = ""
			tunnelExit = nil
		}
		tunnelMu.Unlock()
		if origin != nil {
			origin.Close()
		}
		logger.Info("[CloudflareTunnel] Native process exited", "pid", cmd.Process.Pid)
	}()

	out, _ := json.Marshal(map[string]interface{}{
		"status":   "ok",
		"message":  "Cloudflare tunnel started (native)",
		"pid":      cmd.Process.Pid,
		"mode":     "native",
		"warnings": cloudflareSnapshot().Warnings,
	})
	return string(out)
}

func startNativeQuickTunnel(cfg CloudflareTunnelConfig, registry *ProcessRegistry, args []string, logger *slog.Logger) string {
	if cfg.quickOrigin == nil {
		return errJSON("A managed Homepage origin is required")
	}
	result := startNativeTunnel(cfg, registry, args, nil, logger)

	// Parse to check success
	var r map[string]interface{}
	if json.Unmarshal([]byte(result), &r) == nil && r["status"] == "ok" {
		// Try to capture quick tunnel URL from process output
		pid := cloudflareSnapshot().PID
		go func() {
			if pid <= 0 {
				return
			}
			info, ok := registry.Get(pid)
			if !ok {
				return
			}
			// Poll output for the URL (appears within ~5 seconds)
			for i := 0; i < 20; i++ {
				time.Sleep(500 * time.Millisecond)
				output := info.ReadOutput()
				if url := extractQuickTunnelURL(output); url != "" {
					recordCloudflareQuickURL(cfg.quickOrigin, url, logger)
					return
				}
			}
			logger.Warn("[CloudflareTunnel] Could not capture quick tunnel URL within timeout")
		}()
	}

	return result
}

func stopNativeTunnel(cfg CloudflareTunnelConfig, registry *ProcessRegistry, logger *slog.Logger) string {
	state := cloudflareSnapshot()
	if state.PID > 0 {
		if err := registry.Terminate(state.PID); err != nil {
			return errJSON("Tunnel termination is unconfirmed: %v", err)
		}
	}
	if state.PID > 0 {
		if state.Exit == nil {
			return errJSON("Tunnel termination is unconfirmed: no process-exit acknowledgement")
		}
		select {
		case <-state.Exit:
		case <-time.After(3 * time.Second):
			info, ok := registry.Get(state.PID)
			if !ok {
				return errJSON("Tunnel termination is unconfirmed: process handle missing")
			}
			info.mu.Lock()
			process := info.Process
			info.mu.Unlock()
			if process == nil {
				return errJSON("Tunnel termination is unconfirmed: process handle missing")
			}
			if err := killProcess(process); err != nil {
				return errJSON("Tunnel termination is unconfirmed: %v", err)
			}
			select {
			case <-state.Exit:
			case <-time.After(3 * time.Second):
				return errJSON("Tunnel termination is unconfirmed after kill")
			}
		}
	}

	clearCloudflareState()

	// Clean up credential file written for named tunnel auth
	credPath := filepath.Join(cfg.DataDir, "cloudflared", "credentials.json")
	if _, err := os.Stat(credPath); err == nil {
		if err := os.Remove(credPath); err != nil {
			logger.Warn("[CloudflareTunnel] Failed to remove credential file", "path", credPath, "error", err)
		} else {
			logger.Info("[CloudflareTunnel] Credential file removed", "path", credPath)
		}
	}

	logger.Info("[CloudflareTunnel] Native tunnel stopped")

	out, _ := json.Marshal(map[string]interface{}{
		"status":  "ok",
		"message": "Cloudflare tunnel stopped",
	})
	return string(out)
}

// ──────────────────────────────────────────────────────────────────────────
// Config Generation (Named Tunnel)
// ──────────────────────────────────────────────────────────────────────────

func buildIngressRules(cfg CloudflareTunnelConfig) []map[string]string {
	var rules []map[string]string

	if cfg.AuthMethod != "named" && cfg.ExposeWebUI && cfg.WebUIPort > 0 {
		rules = append(rules, map[string]string{
			"service":  buildLocalURL(cfg, "localhost"),
			"hostname": "(auto — from CF dashboard)",
			"note":     "AuraGo Web UI",
		})
	}
	if cfg.AuthMethod != "named" && cfg.ExposeHomepage && cfg.HomepagePort > 0 {
		rules = append(rules, map[string]string{
			"service":  fmt.Sprintf("http://localhost:%d", cfg.HomepagePort),
			"hostname": "(auto — from CF dashboard)",
			"note":     "Homepage Web Server",
		})
	}
	for _, r := range cfg.CustomIngress {
		entry := map[string]string{
			"hostname": r.Hostname,
			"service":  r.Service,
		}
		if r.Path != "" {
			entry["path"] = r.Path
		}
		rules = append(rules, entry)
	}
	// Catch-all is always required for named tunnel config
	rules = append(rules, map[string]string{
		"service": "http_status:404",
		"note":    "catch-all (required)",
	})
	return rules
}

func validateCustomIngress(rules []CloudflareIngress) error {
	for _, r := range rules {
		if strings.ContainsAny(r.Hostname+r.Path+r.Service, "\r\n") {
			return fmt.Errorf("ingress fields must not contain newlines")
		}
		if (r.Hostname == "" || r.Hostname == "*") && r.Path == "" {
			return fmt.Errorf("ingress rules require a hostname or path; the final 404 rule is automatic")
		}
		if r.Hostname != "" && r.Hostname != "*" {
			host := strings.TrimPrefix(r.Hostname, "*.")
			if len(host) > 253 || strings.Contains(host, "*") {
				return fmt.Errorf("invalid ingress hostname")
			}
			for _, label := range strings.Split(host, ".") {
				if !cloudflareDNSLabel.MatchString(label) {
					return fmt.Errorf("invalid ingress hostname")
				}
			}
		}
		if r.Path != "" {
			if _, err := regexp.Compile(r.Path); err != nil {
				return fmt.Errorf("invalid ingress path regex: %w", err)
			}
		}
		u, err := url.Parse(r.Service)
		if err != nil || u == nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.User != nil || u.Path != "" || u.ForceQuery || strings.ContainsAny(r.Service, "?#") {
			return fmt.Errorf("ingress service must be an HTTP(S) origin without credentials, path, query or fragment")
		}
		if u.Port() != "" {
			port, err := strconv.Atoi(u.Port())
			if err != nil || port < 1 || port > 65535 {
				return fmt.Errorf("invalid ingress service port")
			}
		} else if strings.HasSuffix(u.Host, ":") {
			return fmt.Errorf("invalid ingress service port")
		}
		switch u.Port() {
		case "22", "23", "3389", "5900", "5901":
			return fmt.Errorf("ingress service targets a sensitive port")
		}
	}
	return nil
}

func writeNamedTunnelConfig(cfg CloudflareTunnelConfig, credPath, configPath string) error {
	if len(cfg.CustomIngress) == 0 {
		return fmt.Errorf("named tunnels require explicit custom_ingress routes; configure a hostname or use token authentication")
	}
	if err := validateCustomIngress(cfg.CustomIngress); err != nil {
		return fmt.Errorf("invalid ingress configuration: %w", err)
	}
	if strings.ContainsAny(cfg.TunnelName, "\r\n") {
		return fmt.Errorf("tunnel name must not contain newline characters")
	}
	document := map[string]interface{}{"tunnel": cfg.TunnelName, "credentials-file": credPath}
	if cfg.LogLevel != "" {
		document["loglevel"] = cfg.LogLevel
	}
	if cfg.MetricsPort > 0 {
		document["metrics"] = fmt.Sprintf("localhost:%d", cfg.MetricsPort)
	}
	rules := make([]map[string]interface{}, 0, len(cfg.CustomIngress)+1)
	for _, r := range cfg.CustomIngress {
		rule := map[string]interface{}{"service": r.Service}
		if r.Hostname != "" {
			rule["hostname"] = r.Hostname
		}
		if r.Path != "" {
			rule["path"] = r.Path
		}
		if cloudflareLocalHTTPSOrigin(cfg, r.Service) {
			rule["originRequest"] = map[string]bool{"noTLSVerify": true}
		}
		rules = append(rules, rule)
	}
	document["ingress"] = append(rules, map[string]interface{}{"service": "http_status:404"})
	data, err := yaml.Marshal(document)
	if err != nil {
		return fmt.Errorf("marshal named tunnel configuration: %w", err)
	}
	return fileutil.WriteFileContext(context.Background(), configPath, data, 0600)
}

// ──────────────────────────────────────────────────────────────────────────
// Binary Management
// ──────────────────────────────────────────────────────────────────────────

func cfdBinaryPath(dataDir string) string {
	binDir := filepath.Join(filepath.Dir(dataDir), "bin")
	if runtime.GOOS == "windows" {
		return filepath.Join(binDir, "cloudflared.exe")
	}
	return filepath.Join(binDir, "cloudflared")
}

func findCloudflaredBinary(dataDir string) string {
	// Check in AuraGo bin/ dir first
	binPath := cfdBinaryPath(dataDir)
	if _, err := os.Stat(binPath); err == nil {
		return binPath
	}
	// Check system PATH
	if p, err := exec.LookPath(cfdBinaryName); err == nil {
		return p
	}
	return ""
}

// ──────────────────────────────────────────────────────────────────────────
// Helpers
// ──────────────────────────────────────────────────────────────────────────

// resolveMode determines whether to use Docker or native binary.
func resolveMode(cfg CloudflareTunnelConfig) string {
	switch cfg.Mode {
	case "docker":
		return "docker"
	case "native":
		return "native"
	default: // "auto"
		if checkDockerAvailable(cfg.DockerHost) {
			return "docker"
		}
		if findCloudflaredBinary(cfg.DataDir) != "" {
			return "native"
		}
		// Try to auto-install native binary
		return "native"
	}
}

// Image pulls hold lifecycle ownership, never the short state lock.
var cloudflaredPullTimeout = 60 * time.Second

// pullImage pulls image unless it is already present. A failed pull is logged
// and returned; callers still create the container, as before, and the create
// reports a missing image itself.
func pullImage(dockerCfg DockerConfig, image string, logger *slog.Logger) error {
	// Check if image exists
	filterURL := fmt.Sprintf("/images/json?filters=%%7B%%22reference%%22%%3A%%5B%%22%s%%22%%5D%%7D", image)
	data, code, err := dockerRequest(dockerCfg, "GET", filterURL, "")
	if err == nil && code == 200 {
		var images []interface{}
		if json.Unmarshal(data, &images) == nil && len(images) > 0 {
			return nil // Image already exists
		}
	}

	logger.Info("[CloudflareTunnel] Pulling image", "image", image)
	ctx, cancel := context.WithTimeout(context.Background(), cloudflaredPullTimeout)
	defer cancel()
	if err := pullImageBestEffort(ctx, dockerCfg, image); err != nil {
		logger.Warn("[CloudflareTunnel] Image pull failed; trying to create the container anyway", "image", image, "error", err)
		return err
	}
	return nil
}

func removeContainer(dockerCfg DockerConfig, name string) {
	// Stop if running
	dockerRequest(dockerCfg, "POST", "/containers/"+name+"/stop?t=5", "")
	// Remove
	dockerRequest(dockerCfg, "DELETE", "/containers/"+name+"?force=true", "")
}

func captureQuickTunnelURLDocker(dockerCfg DockerConfig, containerID string, logger *slog.Logger) string {
	// Read container logs
	for i := 0; i < 20; i++ {
		time.Sleep(500 * time.Millisecond)
		data, code, _ := dockerRequest(dockerCfg, "GET", "/containers/"+containerID+"/logs?stdout=true&stderr=true&tail=50", "")
		if code == 200 {
			output := stripDockerLogHeaders(data)
			if url := extractQuickTunnelURL(output); url != "" {
				return url
			}
		}
	}
	return ""
}

// extractQuickTunnelURL finds the trycloudflare.com URL in cloudflared output.
func extractQuickTunnelURL(output string) string {
	for _, word := range strings.Fields(output) {
		u, err := url.Parse(word)
		if err != nil || u.Scheme != "https" || u.User != nil || u.Host != u.Hostname() || u.ForceQuery || strings.ContainsAny(word, "?#") || u.Path != "" && u.Path != "/" {
			continue
		}
		host := u.Hostname()
		label := strings.TrimSuffix(host, ".trycloudflare.com")
		if label == host || !cloudflareDNSLabel.MatchString(label) {
			continue
		}
		return "https://" + host
	}
	return ""
}

func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := cloudflareRandRead(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// IsTunnelRunning returns true if a cloudflared tunnel is currently active.
func IsTunnelRunning() bool {
	tunnelMu.Lock()
	defer tunnelMu.Unlock()
	return tunnelMode != ""
}

// GetTunnelURL returns the current tunnel URL (if any, mainly for quick tunnels).
func GetTunnelURL() string {
	tunnelMu.Lock()
	defer tunnelMu.Unlock()
	return tunnelURL
}
