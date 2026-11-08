package tools

import (
	"bytes"
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"aurago/internal/config"
	"aurago/internal/dockerutil"
)

//go:embed mirage_sidecar/Dockerfile
var mirageSidecarDockerfileRaw []byte

//go:embed mirage_sidecar/server.py
var mirageSidecarServerRaw []byte

//go:embed mirage_sidecar/CACTUS_VERSION
var mirageSidecarVersionRaw []byte

const mirageFingerprintLabel = "aurago.mirage.fingerprint"

// Package vars so tests can shorten the waits.
var (
	mirageReadyTimeout           = 10 * time.Minute // first start downloads the weights
	mirageReadyPollInterval      = 2 * time.Second
	mirageContainerCheckInterval = 10 * time.Second
)

var (
	mirageEnsureMu sync.Mutex
	// mirageGeneration counts EnsureMirageRunning calls; a wait for readiness
	// ends as soon as a newer call exists (it does the waiting from then on).
	mirageGeneration   atomic.Uint64
	mirageBuildBackoff go2RTCBuildBackoff
)

// mirageNormalizeEOL strips carriage returns. Git on Windows checks files out
// with CRLF, which would break ARG/FROM lines in docker build and change the
// content hash between platforms.
func mirageNormalizeEOL(b []byte) []byte { return bytes.ReplaceAll(b, []byte("\r\n"), []byte("\n")) }

func mirageDockerfile() []byte { return mirageNormalizeEOL(mirageSidecarDockerfileRaw) }
func mirageServerPy() []byte   { return mirageNormalizeEOL(mirageSidecarServerRaw) }

func mirageCactusVersion() string { return strings.TrimSpace(string(mirageSidecarVersionRaw)) }

// mirageImageTag is content-addressed: any change to the Dockerfile, the
// server, the pinned cactus version or the base image yields a new tag and a
// rebuild.
func mirageImageTag(base string) string {
	h := sha256.New()
	for _, part := range [][]byte{mirageDockerfile(), mirageServerPy(), []byte(mirageCactusVersion()), []byte(base)} {
		h.Write(part)
		h.Write([]byte{0})
	}
	return "aurago/mirage:" + hex.EncodeToString(h.Sum(nil)[:8])
}

func mirageContainerName(cfg *config.Config) string {
	if name := strings.TrimSpace(cfg.Vision.Mirage.ContainerName); name != "" {
		return name
	}
	return config.MirageDefaultContainerName
}

func mirageDetectGPU(pref string) GPUInfo {
	pref = strings.ToLower(strings.TrimSpace(pref))
	if pref == "none" || pref == "off" {
		return GPUInfo{Backend: "none"}
	}
	return DetectGPU(pref)
}

// mirageApplyGPU adds the device passthrough for the detected backend, the same
// shapes the Ollama sidecar uses (applyGPUToCreateHost).
func mirageApplyGPU(host map[string]interface{}, gpu GPUInfo) {
	switch gpu.Backend {
	case "nvidia":
		host["DeviceRequests"] = []map[string]interface{}{{
			"Driver":       "nvidia",
			"Count":        -1,
			"Capabilities": [][]string{{"gpu"}},
		}}
	case "amd", "intel", "vulkan":
		devices := make([]map[string]string, 0, len(gpu.Devices))
		for _, path := range gpu.Devices {
			devices = append(devices, map[string]string{"PathOnHost": path, "PathInContainer": path, "CgroupPermissions": "rwm"})
		}
		if len(devices) > 0 {
			host["Devices"] = devices
		}
		if gids := hostGroupIDs([]string{"video", "render"}); len(gids) > 0 {
			host["GroupAdd"] = gids
		}
	}
}

func mirageHostConfig(cfg *config.Config, absData string, gpu GPUInfo) map[string]interface{} {
	host := map[string]interface{}{
		"RestartPolicy": map[string]interface{}{"Name": "unless-stopped"},
		"PortBindings": map[string]interface{}{
			fmt.Sprintf("%d/tcp", config.MirageInternalPort): []map[string]string{
				{"HostIp": "127.0.0.1", "HostPort": strconv.Itoa(cfg.Vision.Mirage.ContainerPort)},
			},
		},
		"Binds":     []string{absData + ":/data"},
		"Memory":    int64(2) << 30,
		"PidsLimit": 256,
	}
	mirageApplyGPU(host, gpu)
	return hardenManagedSidecarHostConfig(host)
}

func mirageEnv(cfg *config.Config, gpu GPUInfo) []string {
	m := cfg.Vision.Mirage
	return []string{
		"MIRAGE_PORT=" + strconv.Itoa(config.MirageInternalPort),
		"MIRAGE_MAX_OBJECTS=" + strconv.Itoa(m.MaxObjects),
		"MIRAGE_MAX_TEXT_LINES=" + strconv.Itoa(m.MaxTextLines),
		"MIRAGE_GPU_BACKEND=" + gpu.Backend,
		"MIRAGE_LANGUAGE=" + strings.TrimSpace(m.Language),
		"HOME=/data",
	}
}

func mirageFingerprint(image string, cfg *config.Config, gpu GPUInfo, absData string) string {
	m := cfg.Vision.Mirage
	parts := []string{image, strconv.Itoa(m.ContainerPort), gpu.Backend, strings.Join(gpu.Devices, ","), absData,
		strconv.Itoa(m.MaxObjects), strconv.Itoa(m.MaxTextLines), strings.TrimSpace(m.Language)}
	sum := sha256.Sum256([]byte(strings.Join(parts, "|")))
	return hex.EncodeToString(sum[:8])
}

// mirageCurrentFingerprint recomputes the fingerprint EnsureMirageRunning would
// stamp on a new container for this config (used by tests).
func mirageCurrentFingerprint(cfg *config.Config) string {
	absData, _ := filepath.Abs(cfg.Vision.Mirage.DataPath)
	return mirageFingerprint(mirageImageTag(cfg.Vision.Mirage.Image), cfg, mirageDetectGPU(cfg.Vision.Mirage.GPU), absData)
}

type mirageContainerState struct {
	Exists      bool
	Running     bool
	Fingerprint string
}

func mirageInspect(docker DockerConfig, name string) (mirageContainerState, error) {
	data, code, err := dockerRequest(docker, http.MethodGet, "/containers/"+name+"/json", "")
	if err != nil {
		return mirageContainerState{}, err
	}
	if code == http.StatusNotFound {
		return mirageContainerState{}, nil
	}
	if code != http.StatusOK {
		return mirageContainerState{}, fmt.Errorf("docker inspect of %s returned status %d", name, code)
	}
	var info struct {
		State struct {
			Running bool `json:"Running"`
		} `json:"State"`
		Config struct {
			Labels map[string]string `json:"Labels"`
		} `json:"Config"`
	}
	if err := json.Unmarshal(data, &info); err != nil {
		return mirageContainerState{}, fmt.Errorf("docker inspect of %s was not valid JSON: %w", name, err)
	}
	return mirageContainerState{Exists: true, Running: info.State.Running, Fingerprint: info.Config.Labels[mirageFingerprintLabel]}, nil
}

func ensureMirageImage(ctx context.Context, docker DockerConfig, image, base string, logger *slog.Logger) error {
	if _, code, err := dockerRequest(docker, http.MethodGet, "/images/"+url.PathEscape(image)+"/json", ""); err == nil && code == http.StatusOK {
		mirageBuildBackoff.reset()
		return nil
	}
	if retryAt, cause, paused := mirageBuildBackoff.paused(image); paused {
		return fmt.Errorf("mirage image build is paused until %s after an earlier failure: %w", retryAt.Format(time.RFC3339), cause)
	}
	logger.Info("[Mirage] Building sidecar image", "image", image, "cactus_version", mirageCactusVersion())
	err := BuildImageContextWait(ctx, docker, image, "Dockerfile", mirageDockerfile(),
		map[string][]byte{"server.py": mirageServerPy()},
		map[string]string{"BASE_IMAGE": base, "CACTUS_VERSION": mirageCactusVersion()}, logger)
	if err != nil {
		failures, delay, retryAt := mirageBuildBackoff.failed(image, err)
		logger.Warn("[Mirage] Sidecar image build failed; automatic retries back off",
			"error", err, "failed_attempts", failures, "retry_in", delay.String(), "next_retry_at", retryAt.UTC().Format(time.RFC3339))
		return fmt.Errorf("mirage image build failed: %w", err)
	}
	mirageBuildBackoff.reset()
	return nil
}

func mirageHealthURL(cfg *config.Config) string {
	return strings.TrimSuffix(strings.TrimRight(mirageBaseURLFor(cfg), "/"), "/v1") + "/health"
}

func waitForMirageReady(cfg *config.Config, logger *slog.Logger, gen uint64) error {
	deadline := time.Now().Add(mirageReadyTimeout)
	client := &http.Client{Timeout: 3 * time.Second}
	docker := DockerConfig{Host: cfg.Docker.Host}
	name := mirageContainerName(cfg)
	lastCheck := time.Now()
	for time.Now().Before(deadline) {
		if mirageGeneration.Load() != gen {
			logger.Info("[Mirage] A newer start request superseded this readiness wait")
			return nil
		}
		resp, err := client.Get(mirageHealthURL(cfg))
		if err == nil {
			var body struct {
				Ready bool   `json:"ready"`
				Error string `json:"error"`
			}
			decodeErr := json.NewDecoder(io.LimitReader(resp.Body, 64<<10)).Decode(&body)
			resp.Body.Close()
			if decodeErr == nil && body.Ready {
				return nil
			}
			if decodeErr == nil && body.Error != "" {
				return fmt.Errorf("mirage model failed to load: %s", body.Error)
			}
		}
		if time.Since(lastCheck) >= mirageContainerCheckInterval {
			lastCheck = time.Now()
			if st, err := mirageInspect(docker, name); err == nil && (!st.Exists || !st.Running) {
				return fmt.Errorf("mirage sidecar container stopped while waiting for it to become ready")
			}
		}
		time.Sleep(mirageReadyPollInterval)
	}
	logger.Warn("[Mirage] Sidecar did not become ready in time")
	return fmt.Errorf("mirage sidecar did not become ready within %s", mirageReadyTimeout)
}

// EnsureMirageRunning builds the sidecar image if needed and makes sure a
// container with the current settings is running. userInitiated lifts an active
// build back-off (a settings save should retry at once; boot should not).
func EnsureMirageRunning(cfg *config.Config, logger *slog.Logger, userInitiated bool) error {
	if cfg == nil || !cfg.Vision.Mirage.Enabled || !cfg.Docker.Enabled {
		return nil
	}
	if logger == nil {
		logger = slog.Default()
	}
	gen := mirageGeneration.Add(1)
	if err := startMirageContainer(cfg, logger, userInitiated); err != nil {
		return err
	}
	// The (possibly long) readiness wait runs without the lock so later saves
	// are not queued behind it.
	return waitForMirageReady(cfg, logger, gen)
}

// startMirageContainer builds, inspects, recreates and starts the container
// under mirageEnsureMu, without waiting for readiness.
func startMirageContainer(cfg *config.Config, logger *slog.Logger, userInitiated bool) error {
	mirageEnsureMu.Lock()
	defer mirageEnsureMu.Unlock()
	if userInitiated {
		mirageBuildBackoff.reset()
	}

	m := cfg.Vision.Mirage
	docker := DockerConfig{Host: cfg.Docker.Host}
	name := mirageContainerName(cfg)
	absData, err := filepath.Abs(m.DataPath)
	if err != nil {
		return fmt.Errorf("mirage data path: %w", err)
	}
	if err := os.MkdirAll(absData, 0o755); err != nil {
		return fmt.Errorf("mirage data path: %w", err)
	}
	gpu := mirageDetectGPU(m.GPU)
	image := mirageImageTag(m.Image)
	if err := ensureMirageImage(context.Background(), docker, image, m.Image, logger); err != nil {
		return err
	}
	fp := mirageFingerprint(image, cfg, gpu, absData)

	state, err := mirageInspect(docker, name)
	if err != nil {
		return fmt.Errorf("mirage: docker unavailable: %w", err)
	}
	if state.Exists && state.Fingerprint == fp {
		if !state.Running {
			if _, code, err := dockerRequest(docker, http.MethodPost, "/containers/"+name+"/start", ""); err != nil || (code != http.StatusNoContent && code != http.StatusNotModified) {
				return fmt.Errorf("mirage: starting the existing container failed (status %d, err %v)", code, err)
			}
		}
		return nil
	}
	if state.Exists {
		logger.Info("[Mirage] Container settings changed; recreating", "container", name)
		if _, code, err := dockerRequest(docker, http.MethodDelete, "/containers/"+name+"?force=true", ""); err != nil || (code != http.StatusNoContent && code != http.StatusNotFound) {
			return fmt.Errorf("mirage: removing the old container failed (status %d, err %v)", code, err)
		}
	}

	labels := map[string]string{}
	for k, v := range dockerutil.ManagedLabels(dockerutil.MirageOwner, "mirage", "vision", fp) {
		labels[k] = v
	}
	labels[mirageFingerprintLabel] = fp
	payload := mirageCreatePayload(image, cfg, gpu, absData, managedContainerUserSpec())
	payload["Labels"] = labels
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("mirage create payload: %w", err)
	}
	if _, code, err := dockerRequest(docker, http.MethodPost, "/containers/create?name="+url.QueryEscape(name), string(body)); err != nil || code != http.StatusCreated {
		return fmt.Errorf("mirage: creating the container failed (status %d, err %v)", code, err)
	}
	if _, code, err := dockerRequest(docker, http.MethodPost, "/containers/"+name+"/start", ""); err != nil || (code != http.StatusNoContent && code != http.StatusNotModified) {
		return fmt.Errorf("mirage: starting the container failed (status %d, err %v)", code, err)
	}
	logger.Info("[Mirage] Sidecar started", "container", name, "gpu_requested", gpu.Backend)
	return nil
}

// mirageCreatePayload builds the container create body. With CapDrop ALL the
// container's root cannot write the bind-mounted /data that AuraGo created as
// its own user, so on Linux the container runs as the managed user spec.
func mirageCreatePayload(image string, cfg *config.Config, gpu GPUInfo, absData, userSpec string) map[string]interface{} {
	payload := map[string]interface{}{
		"Image":        image,
		"Env":          mirageEnv(cfg, gpu),
		"ExposedPorts": map[string]interface{}{fmt.Sprintf("%d/tcp", config.MirageInternalPort): struct{}{}},
		"HostConfig":   mirageHostConfig(cfg, absData, gpu),
	}
	if userSpec != "" {
		payload["User"] = userSpec
	}
	return payload
}

// MirageStatusTarget is a lock-free snapshot of what MirageStatus needs, so a
// handler can release the config lock before the network call.
type MirageStatusTarget struct {
	Enabled       bool
	HealthURL     string
	DockerEnabled bool
	DockerHost    string
	ContainerName string
}

func MirageStatusTargetFor(cfg *config.Config) MirageStatusTarget {
	return MirageStatusTarget{
		Enabled:       cfg.Vision.Mirage.Enabled,
		HealthURL:     mirageHealthURL(cfg),
		DockerEnabled: cfg.Docker.Enabled,
		DockerHost:    cfg.Docker.Host,
		ContainerName: mirageContainerName(cfg),
	}
}

type MirageHealth struct {
	Status       string `json:"status"` // disabled | running | starting | stopped | error
	Ready        bool   `json:"ready"`
	Backend      string `json:"backend,omitempty"`
	GPURequested string `json:"gpu_requested,omitempty"`
	Message      string `json:"message,omitempty"`
}

func MirageStatus(t MirageStatusTarget) MirageHealth {
	if !t.Enabled {
		return MirageHealth{Status: "disabled"}
	}
	client := &http.Client{Timeout: 3 * time.Second}
	if resp, err := client.Get(t.HealthURL); err == nil {
		defer resp.Body.Close()
		var body struct {
			Ready        bool   `json:"ready"`
			Backend      string `json:"backend"`
			GPURequested string `json:"gpu_requested"`
			Error        string `json:"error"`
		}
		if resp.StatusCode == http.StatusOK && json.NewDecoder(io.LimitReader(resp.Body, 64<<10)).Decode(&body) == nil {
			h := MirageHealth{Status: "starting", Ready: body.Ready, Backend: body.Backend, GPURequested: body.GPURequested, Message: body.Error}
			switch {
			case body.Ready:
				h.Status = "running"
			case body.Error != "":
				h.Status = "error"
			}
			return h
		}
	}
	if t.DockerEnabled {
		if st, err := mirageInspect(DockerConfig{Host: t.DockerHost}, t.ContainerName); err == nil && st.Running {
			return MirageHealth{Status: "starting"}
		}
	}
	return MirageHealth{Status: "stopped"}
}
