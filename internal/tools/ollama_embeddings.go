package tools

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"

	"aurago/internal/config"
)

const ollamaEmbContainerName = "aurago_ollama_embeddings"

// GPUInfo describes a detected GPU backend for container passthrough.
type GPUInfo struct {
	Backend string // "nvidia", "amd", "intel", "vulkan", "none"
	Devices []string
	Name    string   // human-readable GPU name if detected
	Env     []string // extra environment variables to pass into the container
}

// DetectGPU probes the host for available GPU hardware.
// Only meaningful on Linux; returns "none" on other platforms.
func DetectGPU(preferred string) GPUInfo {
	if runtime.GOOS != "linux" {
		return GPUInfo{Backend: "none"}
	}

	// If a specific backend is requested (not "auto"), only check that one.
	if preferred != "" && preferred != "auto" {
		switch preferred {
		case "nvidia":
			if info, ok := detectNVIDIA(); ok {
				return info
			}
		case "amd":
			if info, ok := detectAMD(); ok {
				return info
			}
		case "intel":
			if info, ok := detectIntel(); ok {
				return info
			}
		case "vulkan":
			if info, ok := detectVulkan(); ok {
				return info
			}
		}
		return GPUInfo{Backend: "none"}
	}

	// Auto-detect: try NVIDIA → AMD → Intel (ordered by Ollama support maturity)
	if info, ok := detectNVIDIA(); ok {
		return info
	}
	if info, ok := detectAMD(); ok {
		return info
	}
	if info, ok := detectIntel(); ok {
		return info
	}
	// Vulkan: generic DRI fallback for older GPUs (pre-ROCm AMD, older iGPUs, etc.)
	if info, ok := detectVulkan(); ok {
		return info
	}
	return GPUInfo{Backend: "none"}
}

func detectNVIDIA() (GPUInfo, bool) {
	// NVIDIA exposes /dev/nvidia0, /dev/nvidia1, etc. via the kernel driver.
	if _, err := os.Stat("/dev/nvidia0"); err != nil {
		return GPUInfo{}, false
	}
	return GPUInfo{
		Backend: "nvidia",
		Name:    "NVIDIA GPU",
	}, true
}

// driDeviceNodes returns only true character-device paths under /dev/dri,
// skipping subdirectories such as by-path and by-id.
func driDeviceNodes() []string {
	entries, err := os.ReadDir("/dev/dri")
	if err != nil {
		return nil
	}
	var nodes []string
	for _, e := range entries {
		p := "/dev/dri/" + e.Name()
		fi, err := os.Stat(p) // follows symlinks
		if err != nil {
			continue
		}
		if fi.Mode()&os.ModeDevice == 0 {
			continue // skip directories (by-path, by-id) and other non-devices
		}
		nodes = append(nodes, p)
	}
	return nodes
}

func detectAMD() (GPUInfo, bool) {
	// AMD ROCm uses /dev/kfd (kernel fusion driver) and /dev/dri/*.
	if _, err := os.Stat("/dev/kfd"); err != nil {
		return GPUInfo{}, false
	}
	devices := append([]string{"/dev/kfd"}, driDeviceNodes()...)
	return GPUInfo{
		Backend: "amd",
		Devices: devices,
		Name:    "AMD GPU (ROCm)",
	}, true
}

func detectIntel() (GPUInfo, bool) {
	// Intel iGPU/Arc use /dev/dri/renderD128 etc.
	// Verify the vendor is Intel (0x8086) via sysfs.
	entries, err := os.ReadDir("/sys/class/drm")
	if err != nil {
		return GPUInfo{}, false
	}
	for _, e := range entries {
		if !strings.HasPrefix(e.Name(), "card") || strings.Contains(e.Name(), "-") {
			continue
		}
		vendorPath := "/sys/class/drm/" + e.Name() + "/device/vendor"
		data, err := os.ReadFile(vendorPath)
		if err != nil {
			continue
		}
		if strings.TrimSpace(string(data)) == "0x8086" {
			return GPUInfo{
				Backend: "intel",
				Devices: driDeviceNodes(),
				Name:    "Intel GPU",
			}, true
		}
	}
	return GPUInfo{}, false
}

// detectVulkan detects any DRI render node as a generic Vulkan-capable GPU.
// This covers older AMD GPUs (pre-ROCm / pre-RDNA), older integrated GPUs,
// and any other GPU that supports Vulkan 1.2+ but not vendor-specific compute APIs.
func detectVulkan() (GPUInfo, bool) {
	devices := driDeviceNodes()
	if len(devices) == 0 {
		return GPUInfo{}, false
	}
	return GPUInfo{
		Backend: "vulkan",
		Devices: devices,
		Name:    "Vulkan GPU (DRI)",
		Env:     []string{"OLLAMA_GPU_BACKEND=vulkan"},
	}, true
}

// ollamaImageForGPU returns the appropriate Ollama Docker image tag for the GPU backend.
func ollamaImageForGPU(gpu GPUInfo) string {
	switch gpu.Backend {
	case "amd":
		return "ollama/ollama:rocm"
	default:
		// NVIDIA, Intel, Vulkan, and CPU all work with the standard image.
		// Vulkan uses OLLAMA_GPU_BACKEND=vulkan env var (set in GPUInfo.Env).
		return "ollama/ollama:latest"
	}
}

// EnsureOllamaEmbeddingsRunning ensures a managed Ollama container is running
// for local embeddings. Follows the same idempotent pattern as EnsureGotenbergRunning.
func EnsureOllamaEmbeddingsRunning(cfg *config.Config, logger interface {
	Info(string, ...any)
	Warn(string, ...any)
	Error(string, ...any)
}) {
	lo := cfg.Embeddings.LocalOllama
	if !lo.Enabled {
		logger.Info("[Ollama Embeddings] Skipping auto-start (disabled in config)")
		return
	}

	dockerCfg := DockerConfig{Host: cfg.Docker.Host}
	port := lo.ContainerPort
	if port <= 0 {
		port = 11435
	}
	portStr := fmt.Sprintf("%d", port)

	// Detect GPU if requested
	var gpu GPUInfo
	if lo.UseHostGPU {
		gpu = DetectGPU(lo.GPUBackend)
		if gpu.Backend == "none" {
			logger.Warn("[Ollama Embeddings] use_host_gpu enabled but no supported GPU detected, falling back to CPU",
				"os", runtime.GOOS, "requested_backend", lo.GPUBackend)
		} else {
			logger.Info("[Ollama Embeddings] GPU detected", "backend", gpu.Backend, "name", gpu.Name)
		}
	}

	image := ollamaImageForGPU(gpu)

	// Check if ANY ollama container is already serving on our port
	listData, listCode, listErr := dockerRequest(dockerCfg, "GET",
		fmt.Sprintf(`/containers/json?filters={"status":["running"],"ancestor":[%q]}`, image), "")
	if listErr == nil && listCode == 200 {
		var containers []map[string]interface{}
		if json.Unmarshal(listData, &containers) == nil && len(containers) > 0 {
			logger.Info("[Ollama Embeddings] Container already running (external)", "count", len(containers))
			waitForOllamaReady(port, logger)
			pullModelIfNeeded(port, lo.Model, logger)
			return
		}
	}

	// Inspect our managed container
	data, code, err := dockerRequest(dockerCfg, "GET", "/containers/"+ollamaEmbContainerName+"/json", "")
	if err != nil {
		logger.Warn("[Ollama Embeddings] Docker unavailable, skipping auto-start", "error", err)
		return
	}

	if code == 200 {
		if ollamaContainerNeedsRecreateForHostDevices(data) {
			logger.Warn("[Ollama Embeddings] Existing container references missing host GPU devices; recreating")
			removeOllamaContainerBestEffort(dockerCfg, ollamaEmbContainerName)
			code = 404
		} else {
			// Container exists — check if running
			var info map[string]interface{}
			if json.Unmarshal(data, &info) == nil {
				if state, ok := info["State"].(map[string]interface{}); ok {
					if running, _ := state["Running"].(bool); running {
						logger.Info("[Ollama Embeddings] Container already running")
						waitForOllamaReady(port, logger)
						pullModelIfNeeded(port, lo.Model, logger)
						return
					}
				}
			}
			// Exists but stopped — start it
			_, startCode, startErr := dockerRequest(dockerCfg, "POST", "/containers/"+ollamaEmbContainerName+"/start", "")
			if startErr != nil || (startCode != 204 && startCode != 304) {
				logger.Error("[Ollama Embeddings] Failed to start existing container", "code", startCode, "error", startErr)
				return
			}
			logger.Info("[Ollama Embeddings] Container started")
			waitForOllamaReady(port, logger)
			pullModelIfNeeded(port, lo.Model, logger)
			return
		}
	}

	if code != 404 {
		logger.Warn("[Ollama Embeddings] Unexpected Docker inspect response, skipping", "code", code)
		return
	}

	// Container does not exist — create and start
	hostConfig := map[string]interface{}{
		"RestartPolicy": map[string]interface{}{"Name": "unless-stopped"},
		"PortBindings": map[string]interface{}{
			"11434/tcp": []map[string]string{{"HostIp": "127.0.0.1", "HostPort": portStr}},
		},
	}

	// GPU passthrough
	applyGPUConfig(gpu, hostConfig)

	payload := map[string]interface{}{
		"Image":      image,
		"HostConfig": hostConfig,
		"ExposedPorts": map[string]interface{}{
			"11434/tcp": struct{}{},
		},
	}
	// Pass backend-specific environment variables (e.g. OLLAMA_GPU_BACKEND=vulkan).
	if len(gpu.Env) > 0 {
		payload["Env"] = gpu.Env
	}
	body, _ := json.Marshal(payload)
	_, createCode, createErr := dockerRequest(dockerCfg, "POST", "/containers/create?name="+ollamaEmbContainerName, string(body))
	if createCode == 404 {
		// Image not present locally — pull it, then retry the create once.
		logger.Info("[Ollama Embeddings] Image not found locally, pulling...", "image", image)
		if pullErr := pullDockerImage(dockerCfg, image); pullErr != nil {
			logger.Error("[Ollama Embeddings] Image pull failed", "image", image, "error", pullErr)
			return
		}
		logger.Info("[Ollama Embeddings] Image pulled successfully", "image", image)
		_, createCode, createErr = dockerRequest(dockerCfg, "POST", "/containers/create?name="+ollamaEmbContainerName, string(body))
	}
	if createErr != nil || createCode != 201 {
		logger.Error("[Ollama Embeddings] Failed to create container", "code", createCode, "error", createErr)
		return
	}

	_, startCode, startErr := dockerRequest(dockerCfg, "POST", "/containers/"+ollamaEmbContainerName+"/start", "")
	if startErr != nil || (startCode != 204 && startCode != 304) {
		logger.Error("[Ollama Embeddings] Failed to start new container", "code", startCode, "error", startErr)
		return
	}
	logger.Info("[Ollama Embeddings] Container created and started", "image", image, "port", port, "gpu", gpu.Backend)

	waitForOllamaReady(port, logger)
	pullModelIfNeeded(port, lo.Model, logger)
}

// hostGroupIDs resolves group names to their numeric GIDs from the host's
// /etc/group file. Using numeric GIDs avoids "no matching entries in group
// file" errors when the group name does not exist inside the container image.
// Groups that are not present on the host are silently skipped.
func hostGroupIDs(names []string) []string {
	data, err := os.ReadFile("/etc/group")
	if err != nil {
		return nil
	}
	want := make(map[string]bool, len(names))
	for _, n := range names {
		want[n] = true
	}
	var ids []string
	for _, line := range strings.Split(string(data), "\n") {
		// Format: name:password:gid:members
		parts := strings.SplitN(line, ":", 4)
		if len(parts) < 3 {
			continue
		}
		if want[parts[0]] {
			ids = append(ids, parts[2])
		}
	}
	return ids
}

// applyGPUConfig adds GPU-specific HostConfig fields based on the detected GPU.
func applyGPUConfig(gpu GPUInfo, hostConfig map[string]interface{}) {
	switch gpu.Backend {
	case "nvidia":
		// Docker Engine DeviceRequests for nvidia-container-toolkit
		hostConfig["DeviceRequests"] = []map[string]interface{}{
			{
				"Driver":       "nvidia",
				"Count":        -1, // all GPUs
				"Capabilities": [][]string{{"gpu"}},
			},
		}
	case "amd":
		// AMD ROCm: bind /dev/kfd + /dev/dri/* as devices, set group_add for video/render
		var devices []map[string]string
		for _, d := range gpu.Devices {
			devices = append(devices, map[string]string{
				"PathOnHost":        d,
				"PathInContainer":   d,
				"CgroupPermissions": "rwm",
			})
		}
		hostConfig["Devices"] = devices
		if gids := hostGroupIDs([]string{"video", "render"}); len(gids) > 0 {
			hostConfig["GroupAdd"] = gids
		}
	case "intel":
		// Intel iGPU/Arc: bind /dev/dri/* as devices
		var devices []map[string]string
		for _, d := range gpu.Devices {
			devices = append(devices, map[string]string{
				"PathOnHost":        d,
				"PathInContainer":   d,
				"CgroupPermissions": "rwm",
			})
		}
		hostConfig["Devices"] = devices
		if gids := hostGroupIDs([]string{"video", "render"}); len(gids) > 0 {
			hostConfig["GroupAdd"] = gids
		}
	case "vulkan":
		// Vulkan compute: bind DRI devices, no vendor-specific toolkit needed.
		// The OLLAMA_GPU_BACKEND=vulkan env var is passed via GPUInfo.Env.
		var devices []map[string]string
		for _, d := range gpu.Devices {
			devices = append(devices, map[string]string{
				"PathOnHost":        d,
				"PathInContainer":   d,
				"CgroupPermissions": "rwm",
			})
		}
		if len(devices) > 0 {
			hostConfig["Devices"] = devices
		}
		if gids := hostGroupIDs([]string{"video", "render"}); len(gids) > 0 {
			hostConfig["GroupAdd"] = gids
		}
	}
}

// waitForOllamaReady polls the Ollama API until it responds or times out.
func waitForOllamaReady(port int, logger interface {
	Info(string, ...any)
	Warn(string, ...any)
}) {
	url := fmt.Sprintf("http://127.0.0.1:%d/", port)
	deadline := time.Now().Add(60 * time.Second)
	client := &http.Client{Timeout: 2 * time.Second}

	for time.Now().Before(deadline) {
		resp, err := client.Get(url)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == 200 {
				logger.Info("[Ollama Embeddings] Container ready")
				return
			}
		}
		time.Sleep(2 * time.Second)
	}
	logger.Warn("[Ollama Embeddings] Container did not become ready within 60s")
}

// pullModelIfNeeded ensures the configured embedding model is available in the Ollama instance.
func pullModelIfNeeded(port int, model string, logger interface {
	Info(string, ...any)
	Warn(string, ...any)
	Error(string, ...any)
}) {
	if model == "" {
		model = "nomic-embed-text"
	}

	baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)
	client := &http.Client{Timeout: 5 * time.Second}

	// Check if model already exists via /api/show
	showBody := fmt.Sprintf(`{"name":%q}`, model)
	showResp, err := client.Post(baseURL+"/api/show", "application/json", strings.NewReader(showBody))
	if err == nil {
		showResp.Body.Close()
		if showResp.StatusCode == 200 {
			logger.Info("[Ollama Embeddings] Model already available", "model", model)
			return
		}
	}

	// Model not present — pull it (this can take a while)
	logger.Info("[Ollama Embeddings] Pulling model (this may take several minutes on first run)", "model", model)
	pullBody := fmt.Sprintf(`{"name":%q,"stream":false}`, model)
	pullClient := &http.Client{Timeout: 30 * time.Minute} // model downloads can be large
	pullResp, err := pullClient.Post(baseURL+"/api/pull", "application/json", strings.NewReader(pullBody))
	if err != nil {
		logger.Error("[Ollama Embeddings] Failed to pull model", "model", model, "error", err)
		return
	}
	pullResp.Body.Close()

	if pullResp.StatusCode == 200 {
		logger.Info("[Ollama Embeddings] Model pulled successfully", "model", model)
	} else {
		logger.Error("[Ollama Embeddings] Model pull returned unexpected status", "model", model, "status", pullResp.StatusCode)
	}
}
