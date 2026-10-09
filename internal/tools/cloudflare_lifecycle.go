package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"aurago/internal/dockerutil"
)

// Lifecycle operations serialize remote mutations; status never waits for them.
var tunnelLifecycleMu sync.Mutex

// The published policy rejects queued operations carrying a revoked snapshot.
var tunnelPolicy *CloudflareTunnelConfig

func cloudflarePolicyError(cfg CloudflareTunnelConfig, starting bool) string {
	if msg := cloudflareTunnelReadOnlyError(cfg); msg != "" {
		return msg
	}
	if tunnelPolicy != nil {
		if tunnelPolicy.ReadOnly {
			return errJSON("Cloudflare Tunnel is in read-only mode")
		}
		if starting && (!tunnelPolicy.Enabled || cfg.AuthMethod == "quick" && (!tunnelPolicy.HomepageEnabled || tunnelPolicy.configuredQuickProjectDir != cfg.configuredQuickProjectDir || tunnelPolicy.HomepageWorkspace != cfg.HomepageWorkspace || tunnelPolicy.HomepageRegistryPath != cfg.HomepageRegistryPath)) {
			return errJSON("Cloudflare publication permission was revoked; reload the saved configuration")
		}
	}
	if starting && !cfg.Enabled {
		return errJSON("Cloudflare Tunnel is disabled")
	}
	return ""
}

// CloudflareTunnelPrepareConfigChange holds lifecycle ownership until publication.
// finish must be called on every path; false preserves the previous policy.
func CloudflareTunnelPrepareConfigChange(old, next CloudflareTunnelConfig, registry *ProcessRegistry, logger *slog.Logger, revokeDocker bool) (finish func(bool), err error) {
	tunnelLifecycleMu.Lock()
	revokeQuick := next.ReadOnly || !next.HomepageEnabled || old.QuickProjectDir != next.QuickProjectDir || old.HomepageWorkspace != next.HomepageWorkspace || old.HomepageRegistryPath != next.HomepageRegistryPath
	state := cloudflareSnapshot()
	if revokeQuick && state.Mode == "" && old.Enabled {
		result := reconcileCloudflareLocked(old, logger, false)
		if !cloudflareTunnelToolResultOK(result) {
			tunnelLifecycleMu.Unlock()
			return nil, fmt.Errorf("Cloudflare revocation state is unknown: %s", cloudflareTunnelToolResultMessage(result))
		}
		state = cloudflareSnapshot()
	}
	if !next.Enabled || revokeDocker || old.DockerHost != next.DockerHost || revokeQuick && (state.Origin != nil || state.Auth == "quick") {
		result := stopCloudflareTunnelLocked(old, registry, logger)
		if !cloudflareTunnelToolResultOK(result) {
			tunnelLifecycleMu.Unlock()
			return nil, fmt.Errorf("Cloudflare configuration change refused: %s", cloudflareTunnelToolResultMessage(result))
		}
	}
	var once sync.Once
	return func(published bool) {
		once.Do(func() {
			if published {
				policy := next
				tunnelPolicy = &policy
			}
			tunnelLifecycleMu.Unlock()
		})
	}, nil
}

type cloudflareWarning struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type cloudflareState struct {
	Mode, Host, ContainerID, Auth, URL string
	PID                                int
	Started                            time.Time
	Origin                             *homepageQuickOrigin
	Exit                               <-chan struct{}
	Warnings                           []cloudflareWarning
}

func cloudflareSnapshot() cloudflareState {
	tunnelMu.Lock()
	defer tunnelMu.Unlock()
	return cloudflareState{tunnelMode, tunnelDockerHost, tunnelContainerID, tunnelAuth, tunnelURL, tunnelPID, tunnelStarted, tunnelQuickOrigin, tunnelExit, append([]cloudflareWarning(nil), tunnelWarnings...)}
}

type cloudflareContainer struct {
	ID     string `json:"Id"`
	Name   string `json:"Name"`
	Config struct {
		Image  string            `json:"Image"`
		Cmd    []string          `json:"Cmd"`
		Labels map[string]string `json:"Labels"`
	} `json:"Config"`
	State struct {
		Running   bool      `json:"Running"`
		StartedAt time.Time `json:"StartedAt"`
	} `json:"State"`
}

func (c cloudflareContainer) auth() (string, error) {
	image := strings.TrimPrefix(c.Config.Image, "docker.io/")
	if c.Name != "/"+cfdContainerName || !(strings.HasPrefix(image, "cloudflare/cloudflared:") || strings.HasPrefix(image, "cloudflare/cloudflared@")) || len(c.ID) != 64 || strings.Trim(c.ID, "0123456789abcdef") != "" {
		return "", fmt.Errorf("reserved Cloudflare container ownership is unverified")
	}
	cmd := c.Config.Cmd
	if len(cmd) < 2 || cmd[0] != "tunnel" {
		return "", fmt.Errorf("reserved Cloudflare container command is unverified")
	}
	auth := ""
	for i, arg := range cmd {
		if arg == "--url" {
			auth = "quick"
			break
		}
		if arg == "run" {
			auth = "token"
			if i+1 < len(cmd) && !strings.HasPrefix(cmd[i+1], "--") {
				auth = "named"
			}
		}
	}
	if auth == "" || (c.Config.Labels["aurago.managed-by"] != "" && c.Config.Labels["aurago.managed-by"] != "cloudflare_tunnel") || (c.Config.Labels["aurago.cloudflare.auth"] != "" && c.Config.Labels["aurago.cloudflare.auth"] != auth) {
		return "", fmt.Errorf("reserved Cloudflare container identity is unverified")
	}
	return auth, nil
}

func inspectCloudflareContainer(host, id string) (*cloudflareContainer, error) {
	if id == "" {
		id = cfdContainerName
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	raw, code, err := DockerRequestContext(ctx, DockerConfig{Host: host}, "GET", "/containers/"+id+"/json", "")
	if code == 404 {
		return nil, nil
	}
	if err != nil || code != 200 {
		return nil, fmt.Errorf("Cloudflare container state is unknown (HTTP %d)", code)
	}
	var c cloudflareContainer
	if err := json.Unmarshal(raw, &c); err != nil {
		return nil, fmt.Errorf("decode Cloudflare container state: %w", err)
	}
	if _, err := c.auth(); err != nil {
		return nil, err
	}
	return &c, nil
}

func cloudflareShouldInspect(cfg CloudflareTunnelConfig, state cloudflareState) bool {
	return state.Mode == "docker" || cfg.DockerEnabled || cfg.Mode == "docker" || cfg.Mode == "auto"
}

// reconcileCloudflareLocked is called only while tunnelLifecycleMu is held.
func reconcileCloudflareLocked(cfg CloudflareTunnelConfig, logger *slog.Logger, enforce bool) string {
	state := cloudflareSnapshot()
	if !cloudflareShouldInspect(cfg, state) || (state.Mode != "" && state.Mode != "docker") {
		return okJSON("No Docker tunnel to reconcile")
	}
	host := cfg.DockerHost
	if state.Mode == "docker" {
		host = state.Host
	}
	c, err := inspectCloudflareContainer(host, state.ContainerID)
	if err != nil {
		return errJSON("%v", err)
	}
	if c == nil {
		if state.Mode == "docker" {
			clearCloudflareState()
		}
		return okJSON("No Docker tunnel exists")
	}
	auth, _ := c.auth()
	tunnelMu.Lock()
	tunnelMode, tunnelDockerHost, tunnelContainerID, tunnelAuth = "docker", dockerutil.NormalizeHost(host), c.ID, auth
	tunnelStarted = c.State.StartedAt
	tunnelMu.Unlock()
	if enforce && (!cfg.Enabled || auth == "quick" && state.Origin == nil) {
		return stopDockerTunnel(cfg, logger)
	}
	return okJSON("Existing Cloudflare container recognized")
}

// CloudflareTunnelReconcile discovers surviving containers before auto-start.
func CloudflareTunnelReconcile(cfg CloudflareTunnelConfig, logger *slog.Logger) string {
	tunnelLifecycleMu.Lock()
	defer tunnelLifecycleMu.Unlock()
	policy := cfg
	tunnelPolicy = &policy
	result := reconcileCloudflareLocked(cfg, logger, true)
	state := cloudflareSnapshot()
	if !cloudflareTunnelToolResultOK(result) || !cfg.Enabled || !cfg.AutoStart || cfg.ReadOnly || state.Mode != "docker" || state.Auth == "quick" {
		return result
	}
	c, err := inspectCloudflareContainer(state.Host, state.ContainerID)
	if err != nil {
		return errJSON("%v", err)
	}
	if c == nil || c.State.Running {
		return result
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	_, code, err := DockerRequestContext(ctx, DockerConfig{Host: state.Host}, "POST", "/containers/"+state.ContainerID+"/start", "")
	if err != nil || code != 204 && code != 304 {
		return errJSON("Existing Cloudflare container start is unconfirmed: HTTP %d", code)
	}
	return okJSON("Existing Cloudflare container started")
}

func clearCloudflareState() {
	tunnelMu.Lock()
	origin := tunnelQuickOrigin
	tunnelMode, tunnelDockerHost, tunnelContainerID, tunnelAuth, tunnelURL = "", "", "", "", ""
	tunnelPID, tunnelExit, tunnelQuickOrigin, tunnelWarnings = 0, nil, nil, nil
	tunnelStarted = time.Time{}
	tunnelMu.Unlock()
	if origin != nil {
		origin.Close()
	}
}

func recordCloudflareQuickURL(origin *homepageQuickOrigin, publicURL string, logger *slog.Logger) {
	tunnelLifecycleMu.Lock()
	defer tunnelLifecycleMu.Unlock()
	tunnelMu.Lock()
	current := origin != nil && tunnelQuickOrigin == origin && tunnelMode != "" && !origin.disabled.Load()
	if current {
		tunnelURL = publicURL
	}
	tunnelMu.Unlock()
	if current {
		if err := origin.recordPublication(publicURL); err != nil {
			logger.Error("[CloudflareTunnel] Homepage publication ledger failed", "error", err)
		}
		logger.Info("[CloudflareTunnel] Quick tunnel URL captured", "url", publicURL)
	}
}

func cloudflareStatusResult(cfg CloudflareTunnelConfig, registry *ProcessRegistry) map[string]interface{} {
	state := cloudflareSnapshot()
	result := map[string]interface{}{"status": "ok", "running": state.Mode != "", "state_known": true, "mode": state.Mode, "auth_method": state.Auth, "warnings": state.Warnings}
	if state.Auth == "" {
		result["auth_method"] = cfg.AuthMethod
	}
	if !state.Started.IsZero() {
		result["started"] = state.Started.Format(time.RFC3339)
		result["uptime"] = fmt.Sprintf("%.0fs", time.Since(state.Started).Seconds())
	}
	if state.URL != "" {
		result["tunnel_url"] = state.URL
	}
	if state.Origin != nil {
		result["publication_disabled"] = state.Origin.disabled.Load()
	}
	if cloudflareShouldInspect(cfg, state) && (state.Mode == "" || state.Mode == "docker") {
		host := cfg.DockerHost
		if state.Mode == "docker" {
			host = state.Host
		}
		c, err := inspectCloudflareContainer(host, state.ContainerID)
		if err != nil {
			result["status"], result["state_known"], result["message"] = "error", false, err.Error()
			result["running"] = nil
		} else if c == nil {
			result["running"] = false
		} else {
			auth, _ := c.auth()
			result["running"], result["container_running"], result["mode"], result["auth_method"] = c.State.Running, c.State.Running, "docker", auth
		}
	} else if state.PID > 0 && registry != nil {
		if info, ok := registry.Get(state.PID); ok {
			info.mu.Lock()
			alive := info.Alive
			info.mu.Unlock()
			result["pid"], result["process_alive"], result["running"] = state.PID, alive, alive
		}
	}
	if result["running"] == false {
		result["message"] = "No tunnel running"
	}
	return result
}
