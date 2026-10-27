package tools

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"time"

	"aurago/internal/config"
	"aurago/internal/dockerutil"
)

const (
	gotenbergContainerName = "aurago_gotenberg"
	gotenbergImage         = "gotenberg/gotenberg:8.38.0@sha256:c36cdd730893e3941d27a196ff8b68675e473642d067ec7ee3668b9adaa37b75"
	gotenbergOwnerLabel    = "aurago.managed-by"
	gotenbergOwner         = "document-creator"
	gotenbergProfileLabel  = "aurago.gotenberg.profile"
	gotenbergProfile       = "1"
)

var ErrGotenbergRestartRequired = errors.New("Gotenberg requires an AuraGo restart to apply the hardened container profile")
var gotenbergLifecycleGate = make(chan struct{}, 1)

type gotenbergPortBinding struct {
	HostIP   string `json:"HostIp"`
	HostPort string
}

type gotenbergContainerConfig struct {
	Image      string
	Env        []string
	Cmd        []string
	Entrypoint []string
	User       string
	Labels     map[string]string
}

type gotenbergContainerInfo struct {
	ID         string `json:"Id"`
	Name       string
	Image      string
	Config     gotenbergContainerConfig
	State      struct{ Running bool }
	Mounts     []json.RawMessage
	HostConfig struct {
		CapAdd, CapDrop, SecurityOpt, Binds []string
		Memory, NanoCpus, ShmSize           int64
		Privileged, AutoRemove              bool
		NetworkMode, PidMode                string
		PortBindings                        map[string][]gotenbergPortBinding
		RestartPolicy                       struct{ Name string }
	}
}

func gotenbergManagedTarget(cfg *config.Config) (hostIP, port string, managed bool) {
	if cfg == nil || cfg.Runtime.IsDocker || !cfg.Docker.Enabled {
		return "", "", false
	}
	u, err := url.Parse(cfg.Tools.DocumentCreator.Gotenberg.URL)
	if err != nil || u.Scheme != "http" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
		return "", "", false
	}
	loopback := func(host string) bool {
		return strings.EqualFold(host, "localhost") || (net.ParseIP(host) != nil && net.ParseIP(host).IsLoopback())
	}
	if !loopback(u.Hostname()) {
		return "", "", false
	}
	engine, err := url.Parse(dockerutil.NormalizeHost(cfg.Docker.Host))
	if err != nil || (engine.Scheme != "unix" && engine.Scheme != "npipe" &&
		!((engine.Scheme == "tcp" || engine.Scheme == "http" || engine.Scheme == "https") && loopback(engine.Hostname()))) {
		return "", "", false
	}
	port = u.Port()
	if port == "" {
		port = "80"
	}
	n, err := strconv.Atoi(port)
	if err != nil || n < 1 || n > 65535 {
		return "", "", false
	}
	hostIP = u.Hostname()
	if strings.EqualFold(hostIP, "localhost") {
		hostIP = "127.0.0.1"
	}
	return hostIP, port, true
}

func gotenbergEnvironment() []string {
	return []string{
		"API_ENABLE_DEBUG_ROUTE=true", "CHROMIUM_DENY_PRIVATE_IPS=true", "LIBREOFFICE_DENY_PRIVATE_IPS=true",
		"WEBHOOK_DENY_PRIVATE_IPS=true", "API_DOWNLOAD_FROM_DENY_PRIVATE_IPS=true",
	}
}

func gotenbergCreatePayload(hostIP, port string) map[string]interface{} {
	return map[string]interface{}{
		"Image":        gotenbergImage,
		"Env":          gotenbergEnvironment(),
		"Cmd":          []string{"gotenberg"},
		"Labels":       map[string]string{gotenbergOwnerLabel: gotenbergOwner, gotenbergProfileLabel: gotenbergProfile},
		"ExposedPorts": map[string]interface{}{"3000/tcp": struct{}{}},
		"HostConfig": hardenManagedSidecarHostConfig(map[string]interface{}{
			"CapAdd": []string{"CHOWN", "FOWNER"}, "Memory": int64(1 << 30), "NanoCpus": int64(1_000_000_000),
			"ShmSize": int64(256 << 20), "RestartPolicy": map[string]string{"Name": "unless-stopped"},
			"PortBindings": map[string][]gotenbergPortBinding{"3000/tcp": {{HostIP: hostIP, HostPort: port}}},
		}),
	}
}

func inspectGotenberg(ctx context.Context, docker DockerConfig, name string) (*gotenbergContainerInfo, error) {
	data, code, err := DockerRequestContext(ctx, docker, http.MethodGet, "/containers/"+url.PathEscape(name)+"/json", "")
	if err != nil {
		return nil, err
	}
	if code == 404 {
		return nil, nil
	}
	if code != 200 {
		return nil, fmt.Errorf("Gotenberg container inspect returned HTTP %d", code)
	}
	var info gotenbergContainerInfo
	if json.Unmarshal(data, &info) != nil || !validGotenbergContainerID(info.ID) {
		return nil, fmt.Errorf("Gotenberg container inspect was incomplete")
	}
	return &info, nil
}

func validGotenbergContainerID(id string) bool {
	_, err := hex.DecodeString(id)
	return len(id) == 64 && err == nil
}

func gotenbergContainerOwned(ctx context.Context, docker DockerConfig, info *gotenbergContainerInfo) bool {
	if info == nil || info.Name != "/"+gotenbergContainerName || len(info.Mounts) != 0 || len(info.HostConfig.Binds) != 0 ||
		info.HostConfig.Privileged || info.HostConfig.AutoRemove || info.HostConfig.PidMode != "" ||
		(info.HostConfig.NetworkMode != "" && info.HostConfig.NetworkMode != "default" && info.HostConfig.NetworkMode != "bridge") {
		return false
	}
	for name := range info.Config.Labels {
		if strings.HasPrefix(name, "com.docker.compose.") {
			return false
		}
	}
	if info.Config.Labels[gotenbergOwnerLabel] == gotenbergOwner && strings.HasPrefix(info.Config.Image, "gotenberg/gotenberg:") {
		return true
	}
	if info.Config.Image != "gotenberg/gotenberg:8" || info.Image == "" ||
		!reflect.DeepEqual(info.HostConfig.CapAdd, []string{"SYS_ADMIN"}) || len(info.HostConfig.CapDrop) != 0 || len(info.HostConfig.SecurityOpt) != 0 ||
		info.HostConfig.Memory != 0 || info.HostConfig.NanoCpus != 0 || info.HostConfig.ShmSize != 256<<20 ||
		info.HostConfig.RestartPolicy.Name != "unless-stopped" ||
		!reflect.DeepEqual(info.HostConfig.PortBindings, map[string][]gotenbergPortBinding{"3000/tcp": {{HostIP: "127.0.0.1", HostPort: "3000"}}}) {
		return false
	}
	data, code, err := DockerRequestContext(ctx, docker, "GET", "/images/"+url.PathEscape(info.Image)+"/json", "")
	if err != nil || code != 200 {
		return false
	}
	var image struct{ Config gotenbergContainerConfig }
	if json.Unmarshal(data, &image) != nil {
		return false
	}
	return reflect.DeepEqual(info.Config.Env, image.Config.Env) && reflect.DeepEqual(info.Config.Cmd, image.Config.Cmd) &&
		reflect.DeepEqual(info.Config.Entrypoint, image.Config.Entrypoint) && info.Config.User == image.Config.User &&
		reflect.DeepEqual(info.Config.Labels, image.Config.Labels)
}

func gotenbergProfileMatches(info *gotenbergContainerInfo, hostIP, port string) bool {
	if info.Config.Image != gotenbergImage || info.Config.Labels[gotenbergProfileLabel] != gotenbergProfile ||
		!reflect.DeepEqual(info.Config.Cmd, []string{"gotenberg"}) || info.HostConfig.RestartPolicy.Name != "unless-stopped" ||
		!reflect.DeepEqual(info.HostConfig.CapDrop, []string{"ALL"}) ||
		!reflect.DeepEqual(info.HostConfig.CapAdd, []string{"CHOWN", "FOWNER"}) ||
		!reflect.DeepEqual(info.HostConfig.SecurityOpt, []string{"no-new-privileges:true"}) ||
		info.HostConfig.Memory != 1<<30 || info.HostConfig.NanoCpus != 1_000_000_000 || info.HostConfig.ShmSize != 256<<20 ||
		!reflect.DeepEqual(info.HostConfig.PortBindings, map[string][]gotenbergPortBinding{"3000/tcp": {{HostIP: hostIP, HostPort: port}}}) {
		return false
	}
	for _, required := range gotenbergEnvironment() {
		found := false
		for _, value := range info.Config.Env {
			if value == required {
				found = true
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// GotenbergRestartRequired is a read-only observation; it never adopts a sidecar.
func GotenbergRestartRequired(ctx context.Context, cfg *config.Config) bool {
	hostIP, port, managed := gotenbergManagedTarget(cfg)
	if !managed {
		return false
	}
	docker := DockerConfig{Host: cfg.Docker.Host}
	info, err := inspectGotenberg(ctx, docker, gotenbergContainerName)
	return err == nil && gotenbergContainerOwned(ctx, docker, info) && !gotenbergProfileMatches(info, hostIP, port)
}

func gotenbergMutation(ctx context.Context, docker DockerConfig, method, endpoint, body string, expected ...int) ([]byte, error) {
	data, code, err := DockerRequestContext(ctx, docker, method, endpoint, body)
	if err != nil {
		return nil, err
	}
	for _, want := range expected {
		if code == want {
			return data, nil
		}
	}
	return nil, fmt.Errorf("Gotenberg Docker operation returned HTTP %d", code)
}

func ensureGotenbergImage(ctx context.Context, docker DockerConfig) error {
	_, code, err := DockerRequestContext(ctx, docker, "GET", "/images/"+url.PathEscape(gotenbergImage)+"/json", "")
	if err != nil {
		return err
	}
	if code == 200 {
		return nil
	}
	if code != 404 {
		return fmt.Errorf("Gotenberg image inspect returned HTTP %d", code)
	}
	if err := requireDockerMutationPermission(); err != nil {
		return err
	}
	return pullDockerImageStream(ctx, docker, gotenbergImage)
}

func waitForGotenberg(ctx context.Context, cfg *config.GotenbergConfig) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	for {
		var status struct {
			Status           string
			Message          string
			Reachable        bool `json:"reachable"`
			SecurityVerified bool `json:"security_verified"`
		}
		if json.Unmarshal([]byte(GotenbergHealth(ctx, cfg)), &status) == nil && status.Status == "success" && status.SecurityVerified {
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("Gotenberg did not become healthy and security-verified (%s): %w", status.Message, ctx.Err())
		case <-time.After(250 * time.Millisecond):
		}
	}
}

// allowRecreate is true only at server startup, never during a live config save.
func EnsureGotenbergRunning(ctx context.Context, cfg *config.Config, allowRecreate bool, logger *slog.Logger) error {
	hostIP, port, managed := gotenbergManagedTarget(cfg)
	if !managed {
		return nil
	}
	if cfg.Docker.ReadOnly {
		return ErrDockerReadOnly
	}
	select {
	case gotenbergLifecycleGate <- struct{}{}:
	case <-ctx.Done():
		return ctx.Err()
	}
	defer func() { <-gotenbergLifecycleGate }()
	ctx, cancel := context.WithTimeout(ctx, dockerPullFallbackTimeout)
	defer cancel()
	docker := DockerConfig{Host: cfg.Docker.Host}
	info, err := inspectGotenberg(ctx, docker, gotenbergContainerName)
	if err != nil {
		return err
	}
	if info != nil && !gotenbergContainerOwned(ctx, docker, info) {
		return fmt.Errorf("Gotenberg container ownership is unverified; refusing to modify it")
	}
	drift := info != nil && !gotenbergProfileMatches(info, hostIP, port)
	if drift && !allowRecreate {
		return ErrGotenbergRestartRequired
	}
	if info != nil && !drift {
		if !info.State.Running {
			if _, err := gotenbergMutation(ctx, docker, "POST", "/containers/"+info.ID+"/start", "", 204, 304); err != nil {
				return err
			}
		}
		return waitForGotenberg(ctx, &cfg.Tools.DocumentCreator.Gotenberg)
	}
	if err := ensureGotenbergImage(ctx, docker); err != nil {
		return err
	}
	previousID := ""
	if drift {
		previousID = info.ID
		backupName := gotenbergContainerName + "_previous_" + info.ID
		if backup, err := inspectGotenberg(ctx, docker, backupName); err != nil || backup != nil {
			return fmt.Errorf("Gotenberg migration backup name is unavailable")
		}
		if _, err := gotenbergMutation(ctx, docker, "POST", "/containers/"+info.ID+"/stop?t=30", "", 204, 304); err != nil {
			return err
		}
		if _, err := gotenbergMutation(ctx, docker, "POST", "/containers/"+info.ID+"/rename?name="+url.QueryEscape(backupName), "", 204); err != nil {
			return err
		}
	}
	payload, _ := json.Marshal(gotenbergCreatePayload(hostIP, port))
	data, err := gotenbergMutation(ctx, docker, "POST", "/containers/create?name="+gotenbergContainerName, string(payload), 201)
	if err != nil {
		return fmt.Errorf("create hardened Gotenberg (previous container retained stopped): %w", err)
	}
	var created struct {
		ID string `json:"Id"`
	}
	if json.Unmarshal(data, &created) != nil || !validGotenbergContainerID(created.ID) {
		return fmt.Errorf("Gotenberg create did not return a container identity; previous container retained stopped")
	}
	if _, err := gotenbergMutation(ctx, docker, "POST", "/containers/"+created.ID+"/start", "", 204, 304); err != nil {
		return err
	}
	if err := waitForGotenberg(ctx, &cfg.Tools.DocumentCreator.Gotenberg); err != nil {
		cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		_, _ = gotenbergMutation(cleanup, docker, "POST", "/containers/"+created.ID+"/stop?t=1", "", 204, 304)
		return err
	}
	if previousID != "" {
		if _, err := gotenbergMutation(ctx, docker, "DELETE", "/containers/"+previousID+"?v=false", "", 204); err != nil {
			return fmt.Errorf("hardened Gotenberg is ready; previous stopped container retained: %w", err)
		}
	}
	if logger != nil {
		logger.Info("[Gotenberg] Hardened sidecar ready", "port", port, "migrated", previousID != "")
	}
	return nil
}
