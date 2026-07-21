package tools

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"aurago/internal/config"
	"aurago/internal/dockerutil"
)

const go2RTCFingerprintLabel = "aurago.go2rtc.fingerprint"
const go2RTCOwnerLabel = "aurago.go2rtc.owner"

// StartContainer creates or starts the hardened managed go2rtc sidecar.
func (m *Go2RTCManager) StartContainer(ctx context.Context) error {
	if m == nil {
		return fmt.Errorf("go2rtc manager is not initialized")
	}
	m.lifecycleMu.Lock()
	defer m.lifecycleMu.Unlock()
	return m.startContainerLocked(ctx)
}

func (m *Go2RTCManager) startContainerLocked(ctx context.Context) error {
	m.mu.Lock()
	m.manualStop = false
	m.mu.Unlock()
	cfg := m.Config()
	if !cfg.Enabled {
		return fmt.Errorf("go2rtc integration is disabled")
	}
	m.mu.RLock()
	inDocker := m.inDocker
	m.mu.RUnlock()
	if err := ValidateGo2RTCInternalURL(cfg.URL, cfg.APIHostPort, inDocker); err != nil {
		return err
	}
	if err := validateGo2RTCWebRTC(cfg.WebRTC); err != nil {
		return err
	}
	password, err := m.ensureAPIPassword()
	if err != nil {
		return err
	}
	configBytes := renderGo2RTCConfig(cfg)
	configPath, err := m.writeConfig(configBytes)
	if err != nil {
		return err
	}
	owner := m.go2RTCOwner()
	fingerprint := go2RTCFingerprint(cfg, configBytes, owner)
	m.mu.RLock()
	dockerCfg := m.docker
	m.mu.RUnlock()

	inspect, code, err := dockerRequest(dockerCfg, http.MethodGet, "/containers/"+url.PathEscape(cfg.ContainerName)+"/json", "")
	if err != nil {
		return fmt.Errorf("inspect go2rtc container: %w", err)
	}
	if code == http.StatusOK {
		var info struct {
			ID     string `json:"Id"`
			Config struct {
				Labels map[string]string `json:"Labels"`
			} `json:"Config"`
			State struct {
				Running bool `json:"Running"`
			} `json:"State"`
		}
		if err := json.Unmarshal(inspect, &info); err != nil {
			return fmt.Errorf("decode go2rtc container inspection: %w", err)
		}
		if info.Config.Labels["aurago.managed"] != "go2rtc" || info.Config.Labels[go2RTCOwnerLabel] != owner {
			return fmt.Errorf("container name %q is already used by another workload or AuraGo instance", cfg.ContainerName)
		}
		if info.Config.Labels[go2RTCFingerprintLabel] != fingerprint {
			if err := m.removeContainer(); err != nil {
				return err
			}
			code = http.StatusNotFound
		} else if info.State.Running {
			if err := m.waitForAPI(ctx); err == nil {
				return nil
			}
			if err := m.removeContainer(); err != nil {
				return err
			}
			code = http.StatusNotFound
		} else {
			_, startCode, startErr := dockerRequest(dockerCfg, http.MethodPost, "/containers/"+url.PathEscape(info.ID)+"/start", "")
			if startErr != nil {
				return fmt.Errorf("start go2rtc container: %w", startErr)
			}
			if startCode != http.StatusNoContent && startCode != http.StatusNotModified {
				return fmt.Errorf("start go2rtc container returned HTTP %d", startCode)
			}
			if err := m.waitForAPI(ctx); err == nil {
				return nil
			}
			if err := m.removeContainer(); err != nil {
				return err
			}
			code = http.StatusNotFound
		}
	}
	if code != http.StatusNotFound {
		return fmt.Errorf("inspect go2rtc container returned HTTP %d", code)
	}

	if _, imageCode, imageErr := dockerRequest(dockerCfg, http.MethodGet, "/images/"+url.PathEscape(cfg.Image)+"/json", ""); imageErr != nil || imageCode != http.StatusOK {
		_, pullCode, pullErr := dockerRequest(dockerCfg, http.MethodPost, "/images/create?fromImage="+url.QueryEscape(cfg.Image), "")
		if pullErr != nil {
			return fmt.Errorf("pull go2rtc image: %w", pullErr)
		}
		if pullCode != http.StatusOK && pullCode != http.StatusCreated {
			return fmt.Errorf("pull go2rtc image returned HTTP %d", pullCode)
		}
	}

	payload, err := m.go2RTCContainerPayload(cfg, configPath, password, fingerprint)
	if err != nil {
		return err
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode go2rtc container spec: %w", err)
	}
	_, createCode, createErr := dockerRequest(dockerCfg, http.MethodPost, "/containers/create?name="+url.QueryEscape(cfg.ContainerName), string(body))
	if createErr != nil {
		return fmt.Errorf("create go2rtc container: %w", createErr)
	}
	if createCode != http.StatusCreated {
		return fmt.Errorf("create go2rtc container returned HTTP %d", createCode)
	}
	_, startCode, startErr := dockerRequest(dockerCfg, http.MethodPost, "/containers/"+url.PathEscape(cfg.ContainerName)+"/start", "")
	if startErr != nil {
		_ = m.removeContainer()
		return fmt.Errorf("start go2rtc container: %w", startErr)
	}
	if startCode != http.StatusNoContent && startCode != http.StatusNotModified {
		_ = m.removeContainer()
		return fmt.Errorf("start go2rtc container returned HTTP %d", startCode)
	}
	m.logger.Info("[go2rtc] Managed sidecar started", "container", cfg.ContainerName, "image", cfg.Image)
	return m.waitForAPI(ctx)
}

func (m *Go2RTCManager) waitForAPI(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	waitCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	var lastErr error
	for {
		if _, err := m.Test(waitCtx); err == nil {
			return nil
		} else {
			lastErr = err
		}
		select {
		case <-waitCtx.Done():
			if lastErr != nil {
				return fmt.Errorf("wait for go2rtc API: %w", lastErr)
			}
			return fmt.Errorf("wait for go2rtc API: %w", waitCtx.Err())
		case <-ticker.C:
		}
	}
}

// StopContainer stops the managed sidecar without deleting its spec.
func (m *Go2RTCManager) StopContainer() error {
	if m == nil {
		return fmt.Errorf("go2rtc manager is not initialized")
	}
	m.lifecycleMu.Lock()
	defer m.lifecycleMu.Unlock()
	return m.stopContainerLocked()
}

func (m *Go2RTCManager) stopContainerLocked() error {
	m.mu.RLock()
	dockerCfg := m.docker
	m.mu.RUnlock()
	containerID, exists, err := m.ownedContainerID()
	if err != nil {
		return err
	}
	if !exists {
		m.mu.Lock()
		m.manualStop = true
		m.mu.Unlock()
		m.setAPIStatus(false, "", "")
		return nil
	}
	_, code, err := dockerRequest(dockerCfg, http.MethodPost, "/containers/"+url.PathEscape(containerID)+"/stop?t=10", "")
	if err != nil {
		return fmt.Errorf("stop go2rtc container: %w", err)
	}
	if code != http.StatusNoContent && code != http.StatusNotModified && code != http.StatusNotFound {
		return fmt.Errorf("stop go2rtc container returned HTTP %d", code)
	}
	m.mu.Lock()
	m.manualStop = true
	m.mu.Unlock()
	m.setAPIStatus(false, "", "")
	return nil
}

// RestartContainer recreates the sidecar so removed/disabled sources cannot remain in memory.
func (m *Go2RTCManager) RestartContainer(ctx context.Context) error {
	if m == nil {
		return fmt.Errorf("go2rtc manager is not initialized")
	}
	m.lifecycleMu.Lock()
	defer m.lifecycleMu.Unlock()
	if err := m.removeContainer(); err != nil {
		return err
	}
	if err := m.startContainerLocked(ctx); err != nil {
		return err
	}
	_, err := m.reconcileStreamsLocked(ctx)
	return err
}

// ReconfigureContainer removes the sidecar through its previous Docker target
// before publishing the new identity. This prevents name or daemon changes from
// orphaning a container that still contains the previous runtime sources.
func (m *Go2RTCManager) ReconfigureContainer(ctx context.Context, oldCfg, newCfg *config.Config) error {
	if m == nil || oldCfg == nil || newCfg == nil {
		return fmt.Errorf("go2rtc reconfiguration requires old and new config")
	}
	m.lifecycleMu.Lock()
	defer m.lifecycleMu.Unlock()
	m.configureLocked(oldCfg)
	if err := m.removeContainer(); err != nil {
		return err
	}
	m.configureLocked(newCfg)
	if err := m.startContainerLocked(ctx); err != nil {
		return err
	}
	_, err := m.reconcileStreamsLocked(ctx)
	return err
}

func (m *Go2RTCManager) removeContainer() error {
	m.mu.RLock()
	dockerCfg := m.docker
	m.mu.RUnlock()
	containerID, exists, err := m.ownedContainerID()
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}
	_, _, _ = dockerRequest(dockerCfg, http.MethodPost, "/containers/"+url.PathEscape(containerID)+"/stop?t=10", "")
	_, code, err := dockerRequest(dockerCfg, http.MethodDelete, "/containers/"+url.PathEscape(containerID)+"?force=true&v=false", "")
	if err != nil {
		return fmt.Errorf("remove go2rtc container: %w", err)
	}
	if code != http.StatusNoContent && code != http.StatusNotFound {
		return fmt.Errorf("remove go2rtc container returned HTTP %d", code)
	}
	return nil
}

func (m *Go2RTCManager) ownedContainerID() (string, bool, error) {
	cfg := m.Config()
	m.mu.RLock()
	dockerCfg := m.docker
	m.mu.RUnlock()
	data, code, err := dockerRequest(dockerCfg, http.MethodGet, "/containers/"+url.PathEscape(cfg.ContainerName)+"/json", "")
	if err != nil {
		return "", false, fmt.Errorf("inspect go2rtc container ownership: %w", err)
	}
	if code == http.StatusNotFound {
		return "", false, nil
	}
	if code != http.StatusOK {
		return "", false, fmt.Errorf("inspect go2rtc container ownership returned HTTP %d", code)
	}
	var info struct {
		ID     string `json:"Id"`
		Config struct {
			Labels map[string]string `json:"Labels"`
		} `json:"Config"`
	}
	if err := json.Unmarshal(data, &info); err != nil {
		return "", false, fmt.Errorf("decode go2rtc container ownership: %w", err)
	}
	if info.Config.Labels["aurago.managed"] != "go2rtc" || info.Config.Labels[go2RTCOwnerLabel] != m.go2RTCOwner() {
		return "", false, fmt.Errorf("container name %q is owned by another workload or AuraGo instance", cfg.ContainerName)
	}
	if strings.TrimSpace(info.ID) == "" {
		return "", false, fmt.Errorf("owned go2rtc container has no Docker ID")
	}
	return info.ID, true, nil
}

func (m *Go2RTCManager) go2RTCContainerPayload(cfg config.Go2RTCConfig, configPath, password, fingerprint string) (map[string]interface{}, error) {
	m.mu.RLock()
	dockerCfg := m.docker
	m.mu.RUnlock()
	hostConfig := map[string]interface{}{
		"ReadonlyRootfs": true,
		"SecurityOpt":    []string{"no-new-privileges:true"},
		"CapDrop":        []string{"ALL"},
		"Tmpfs":          map[string]string{"/tmp": "rw,noexec,nosuid,size=64m,mode=1777"},
		"PidsLimit":      int64(100),
		"Memory":         int64(256 << 20),
		"NanoCpus":       int64(500_000_000),
		"RestartPolicy":  map[string]interface{}{"Name": "unless-stopped"},
	}
	exposed := map[string]interface{}{"1984/tcp": struct{}{}}
	portBindings := make(map[string][]map[string]string)
	payload := map[string]interface{}{
		"Image": cfg.Image,
		"User":  "65532:65532",
		"Env":   []string{"AURAGO_GO2RTC_API_PASSWORD=" + password},
		"Labels": map[string]string{
			"aurago.managed":       "go2rtc",
			go2RTCFingerprintLabel: fingerprint,
			go2RTCOwnerLabel:       m.go2RTCOwner(),
		},
		"ExposedPorts": exposed,
		"HostConfig":   hostConfig,
	}
	if browserAutomationRunsInDocker() {
		networkName, configMount, err := go2RTCCurrentContainerPlacement(dockerCfg, filepath.Dir(configPath))
		if err != nil {
			return nil, err
		}
		hostConfig["NetworkMode"] = networkName
		hostConfig["Mounts"] = []map[string]interface{}{configMount}
		payload["NetworkingConfig"] = map[string]interface{}{
			"EndpointsConfig": map[string]interface{}{
				networkName: map[string]interface{}{"Aliases": []string{"go2rtc"}},
			},
		}
	} else {
		bind := dockerutil.FormatBindMount(filepath.Dir(configPath), "/config") + ":ro"
		if err := validateDockerBindMount(dockerCfg, bind); err != nil {
			return nil, fmt.Errorf("validate go2rtc config mount: %w", err)
		}
		hostConfig["Binds"] = []string{bind}
		portBindings["1984/tcp"] = []map[string]string{{"HostIp": "127.0.0.1", "HostPort": strconv.Itoa(cfg.APIHostPort)}}
	}
	if cfg.WebRTC.Enabled {
		hostIP := strings.TrimSpace(cfg.WebRTC.BindAddress)
		hostPort := strconv.Itoa(cfg.WebRTC.Port)
		exposed["8555/tcp"] = struct{}{}
		exposed["8555/udp"] = struct{}{}
		portBindings["8555/tcp"] = []map[string]string{{"HostIp": hostIP, "HostPort": hostPort}}
		portBindings["8555/udp"] = []map[string]string{{"HostIp": hostIP, "HostPort": hostPort}}
	}
	if len(portBindings) > 0 {
		hostConfig["PortBindings"] = portBindings
	}
	return payload, nil
}

func (m *Go2RTCManager) go2RTCOwner() string {
	rawOwner := ""
	if browserAutomationRunsInDocker() {
		if hostname, err := os.Hostname(); err == nil && strings.TrimSpace(hostname) != "" {
			rawOwner = "container:" + strings.TrimSpace(hostname)
		}
	}
	if rawOwner == "" {
		m.mu.RLock()
		configDir := m.configDir
		m.mu.RUnlock()
		if absolute, err := filepath.Abs(configDir); err == nil {
			configDir = absolute
		}
		rawOwner = "native:" + filepath.Clean(configDir)
	}
	sum := sha256.Sum256([]byte(rawOwner))
	return "sha256:" + hex.EncodeToString(sum[:16])
}

type go2RTCContainerInspection struct {
	NetworkSettings struct {
		Networks map[string]struct{} `json:"Networks"`
	} `json:"NetworkSettings"`
	Mounts []struct {
		Type        string `json:"Type"`
		Name        string `json:"Name"`
		Source      string `json:"Source"`
		Destination string `json:"Destination"`
	} `json:"Mounts"`
}

func go2RTCCurrentContainerPlacement(dockerCfg DockerConfig, configDir string) (string, map[string]interface{}, error) {
	selfID, err := os.Hostname()
	if err != nil {
		return "", nil, fmt.Errorf("resolve current AuraGo container hostname: %w", err)
	}
	data, code, err := dockerRequest(dockerCfg, http.MethodGet, "/containers/"+url.PathEscape(selfID)+"/json", "")
	if err != nil {
		return "", nil, fmt.Errorf("inspect current AuraGo container %q: %w", selfID, err)
	}
	if code != http.StatusOK {
		return "", nil, fmt.Errorf("inspect current AuraGo container %q returned HTTP %d", selfID, code)
	}
	return go2RTCPlacementFromInspection(data, configDir)
}

func go2RTCPlacementFromInspection(data []byte, configDir string) (string, map[string]interface{}, error) {
	var info go2RTCContainerInspection
	if err := json.Unmarshal(data, &info); err != nil {
		return "", nil, fmt.Errorf("decode current AuraGo container placement: %w", err)
	}
	networks := make([]string, 0, len(info.NetworkSettings.Networks))
	for name := range info.NetworkSettings.Networks {
		lower := strings.ToLower(name)
		if strings.TrimSpace(name) != "" && !strings.Contains(lower, "docker-control") && !strings.Contains(lower, "docker_control") {
			networks = append(networks, name)
		}
	}
	sort.Strings(networks)
	if len(networks) == 0 {
		return "", nil, fmt.Errorf("AuraGo has no non-control Docker network available for go2rtc")
	}
	cleanConfigDir := filepath.Clean(configDir)
	for _, mount := range info.Mounts {
		destination := filepath.Clean(mount.Destination)
		relative, relErr := filepath.Rel(destination, cleanConfigDir)
		if relErr != nil || relative == "." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || relative == ".." {
			continue
		}
		switch strings.ToLower(mount.Type) {
		case "volume":
			if strings.TrimSpace(mount.Name) == "" {
				continue
			}
			return networks[0], map[string]interface{}{
				"Type":     "volume",
				"Source":   mount.Name,
				"Target":   "/config",
				"ReadOnly": true,
				"VolumeOptions": map[string]interface{}{
					"Subpath": filepath.ToSlash(relative),
				},
			}, nil
		case "bind":
			if strings.TrimSpace(mount.Source) == "" {
				continue
			}
			return networks[0], map[string]interface{}{
				"Type":     "bind",
				"Source":   filepath.Join(mount.Source, relative),
				"Target":   "/config",
				"ReadOnly": true,
			}, nil
		}
	}
	return "", nil, fmt.Errorf("AuraGo data mount containing %q was not found for go2rtc", configDir)
}

func (m *Go2RTCManager) writeConfig(data []byte) (string, error) {
	m.mu.RLock()
	configDir := m.configDir
	m.mu.RUnlock()
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		return "", fmt.Errorf("create go2rtc config directory: %w", err)
	}
	if err := os.Chmod(configDir, 0o755); err != nil {
		return "", fmt.Errorf("make go2rtc config directory readable by sidecar: %w", err)
	}
	target := filepath.Join(configDir, "go2rtc.yaml")
	tmp, err := os.CreateTemp(configDir, ".go2rtc-*.yaml")
	if err != nil {
		return "", fmt.Errorf("create go2rtc config: %w", err)
	}
	tmpPath := tmp.Name()
	cleanup := true
	defer func() {
		_ = tmp.Close()
		if cleanup {
			_ = os.Remove(tmpPath)
		}
	}()
	if _, err := tmp.Write(data); err != nil {
		return "", fmt.Errorf("write go2rtc config: %w", err)
	}
	if err := tmp.Chmod(0o644); err != nil {
		return "", fmt.Errorf("set go2rtc config permissions: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return "", fmt.Errorf("close go2rtc config: %w", err)
	}
	if err := os.Rename(tmpPath, target); err != nil {
		return "", fmt.Errorf("publish go2rtc config: %w", err)
	}
	cleanup = false
	return target, nil
}

func renderGo2RTCConfig(cfg config.Go2RTCConfig) []byte {
	webrtcListen := `""`
	candidates := "[]"
	if cfg.WebRTC.Enabled {
		webrtcListen = `":8555"`
		candidates = fmt.Sprintf("[\"%s:%d\"]", cfg.WebRTC.BindAddress, cfg.WebRTC.Port)
	}
	return []byte(fmt.Sprintf(`api:
  listen: ":1984"
  base_path: "/api/go2rtc/proxy"
  username: "aurago"
  password: "${AURAGO_GO2RTC_API_PASSWORD}"
  local_auth: true
  allow_paths:
    - "/api/go2rtc/proxy/"
    - "/api/go2rtc/proxy/api"
    - "/api/go2rtc/proxy/api/streams"
    - "/api/go2rtc/proxy/api/webrtc"
    - "/api/go2rtc/proxy/api/frame.jpeg"
    - "/api/go2rtc/proxy/api/ws"
    - "/api/go2rtc/proxy/api/stream.m3u8"
    - "/api/go2rtc/proxy/api/stream.mp4"
    - "/api/go2rtc/proxy/api/stream.mjpeg"
    - "/api/go2rtc/proxy/api/hls/playlist.m3u8"
    - "/api/go2rtc/proxy/api/hls/segment.ts"
    - "/api/go2rtc/proxy/api/hls/init.mp4"
    - "/api/go2rtc/proxy/api/hls/segment.m4s"
rtsp:
  listen: "127.0.0.1:8554"
webrtc:
  listen: %s
  candidates: %s
  ice_servers: []
rtmp:
  listen: ""
srtp:
  listen: ""
log:
  # Runtime stream URLs are Vault-only and go2rtc includes producer URLs in
  # warning messages. Disable upstream logs so Docker logging cannot persist
  # camera credentials; AuraGo exposes sanitized lifecycle status instead.
  level: "disabled"
streams: {}
`, webrtcListen, candidates))
}

func validateGo2RTCWebRTC(cfg config.Go2RTCWebRTCConfig) error {
	if !cfg.Enabled {
		return nil
	}
	ip := net.ParseIP(strings.TrimSpace(cfg.BindAddress))
	if ip == nil || ip.IsUnspecified() || ip.IsLoopback() || ip.IsMulticast() || !ip.IsPrivate() {
		return fmt.Errorf("go2rtc WebRTC requires a concrete private LAN bind/candidate IP")
	}
	if cfg.Port < 1 || cfg.Port > 65535 {
		return fmt.Errorf("go2rtc WebRTC port is invalid")
	}
	return nil
}

func go2RTCFingerprint(cfg config.Go2RTCConfig, rendered []byte, owner string) string {
	input := append([]byte(cfg.Image+"\x00"+cfg.URL+"\x00"+strconv.Itoa(cfg.APIHostPort)+"\x00"+owner+"\x00"), rendered...)
	sum := sha256.Sum256(input)
	return hex.EncodeToString(sum[:])
}

func go2RTCContainerRunning(dockerCfg DockerConfig, containerName, owner string) bool {
	if strings.TrimSpace(containerName) == "" {
		return false
	}
	data, code, err := dockerRequest(dockerCfg, http.MethodGet, "/containers/"+url.PathEscape(containerName)+"/json", "")
	if err != nil || code != http.StatusOK {
		return false
	}
	var info struct {
		Config struct {
			Labels map[string]string `json:"Labels"`
		} `json:"Config"`
		State struct {
			Running bool `json:"Running"`
		} `json:"State"`
	}
	return json.Unmarshal(data, &info) == nil &&
		info.Config.Labels["aurago.managed"] == "go2rtc" &&
		info.Config.Labels[go2RTCOwnerLabel] == owner &&
		info.State.Running
}
