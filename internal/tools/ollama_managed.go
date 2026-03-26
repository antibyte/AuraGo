package tools

import (
	"encoding/json"
	"fmt"
	"runtime"
	"strings"

	"aurago/internal/config"
)

const ollamaManagedContainerName = "aurago_ollama_managed"

// EnsureOllamaManagedRunning ensures a managed Ollama container is running
// for the main Ollama integration. Follows the same idempotent pattern as
// EnsureOllamaEmbeddingsRunning but uses a separate container name, allows
// persistent volume mounts, memory limits and multiple default models.
func EnsureOllamaManagedRunning(cfg *config.Config, logger interface {
	Info(string, ...any)
	Warn(string, ...any)
	Error(string, ...any)
}) {
	mi := cfg.Ollama.ManagedInstance
	if !mi.Enabled {
		logger.Info("[Ollama Managed] Skipping auto-start (disabled in config)")
		return
	}

	dockerCfg := DockerConfig{Host: cfg.Docker.Host}
	port := mi.ContainerPort
	if port <= 0 {
		port = 11434
	}
	portStr := fmt.Sprintf("%d", port)

	// Detect GPU if requested
	var gpu GPUInfo
	if mi.UseHostGPU {
		gpu = DetectGPU(mi.GPUBackend)
		if gpu.Backend == "none" {
			logger.Warn("[Ollama Managed] use_host_gpu enabled but no supported GPU detected, falling back to CPU",
				"os", runtime.GOOS, "requested_backend", mi.GPUBackend)
		} else {
			logger.Info("[Ollama Managed] GPU detected", "backend", gpu.Backend, "name", gpu.Name)
		}
	}

	image := ollamaImageForGPU(gpu)

	// Check if ANY ollama container is already serving on our port
	listData, listCode, listErr := dockerRequest(dockerCfg, "GET",
		fmt.Sprintf(`/containers/json?filters={"status":["running"],"ancestor":[%q]}`, image), "")
	if listErr == nil && listCode == 200 {
		var containers []map[string]interface{}
		if json.Unmarshal(listData, &containers) == nil && len(containers) > 0 {
			logger.Info("[Ollama Managed] Container already running (external)", "count", len(containers))
			waitForOllamaReady(port, logger)
			pullManagedModels(port, mi.DefaultModels, logger)
			return
		}
	}

	// Inspect our managed container
	data, code, err := dockerRequest(dockerCfg, "GET", "/containers/"+ollamaManagedContainerName+"/json", "")
	if err != nil {
		logger.Warn("[Ollama Managed] Docker unavailable, skipping auto-start", "error", err)
		return
	}

	if code == 200 {
		// Container exists — check if running
		var info map[string]interface{}
		if json.Unmarshal(data, &info) == nil {
			if state, ok := info["State"].(map[string]interface{}); ok {
				if running, _ := state["Running"].(bool); running {
					logger.Info("[Ollama Managed] Container already running")
					waitForOllamaReady(port, logger)
					pullManagedModels(port, mi.DefaultModels, logger)
					return
				}
			}
		}
		// Exists but stopped — start it
		_, startCode, startErr := dockerRequest(dockerCfg, "POST", "/containers/"+ollamaManagedContainerName+"/start", "")
		if startErr != nil || (startCode != 204 && startCode != 304) {
			logger.Error("[Ollama Managed] Failed to start existing container", "code", startCode, "error", startErr)
			return
		}
		logger.Info("[Ollama Managed] Container started")
		waitForOllamaReady(port, logger)
		pullManagedModels(port, mi.DefaultModels, logger)
		return
	}

	if code != 404 {
		logger.Warn("[Ollama Managed] Unexpected Docker inspect response, skipping", "code", code)
		return
	}

	// Container does not exist — create and start
	hostConfig := map[string]interface{}{
		"RestartPolicy": map[string]interface{}{"Name": "unless-stopped"},
		"PortBindings": map[string]interface{}{
			"11434/tcp": []map[string]string{{"HostIp": "127.0.0.1", "HostPort": portStr}},
		},
	}

	// Memory limit
	if mi.MemoryLimit != "" {
		if memBytes, ok := parseMemoryLimit(mi.MemoryLimit); ok {
			hostConfig["Memory"] = memBytes
		}
	}

	// Persistent volume for model storage
	var binds []string
	if mi.VolumePath != "" {
		binds = append(binds, mi.VolumePath+":/root/.ollama")
	}
	if len(binds) > 0 {
		hostConfig["Binds"] = binds
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
	_, createCode, createErr := dockerRequest(dockerCfg, "POST", "/containers/create?name="+ollamaManagedContainerName, string(body))
	if createCode == 404 {
		// Image not present locally — pull it, then retry the create once.
		logger.Info("[Ollama Managed] Image not found locally, pulling...", "image", image)
		if pullErr := pullDockerImage(dockerCfg, image); pullErr != nil {
			logger.Error("[Ollama Managed] Image pull failed", "image", image, "error", pullErr)
			return
		}
		logger.Info("[Ollama Managed] Image pulled successfully", "image", image)
		_, createCode, createErr = dockerRequest(dockerCfg, "POST", "/containers/create?name="+ollamaManagedContainerName, string(body))
	}
	if createErr != nil || createCode != 201 {
		logger.Error("[Ollama Managed] Failed to create container", "code", createCode, "error", createErr)
		return
	}

	_, startCode, startErr := dockerRequest(dockerCfg, "POST", "/containers/"+ollamaManagedContainerName+"/start", "")
	if startErr != nil || (startCode != 204 && startCode != 304) {
		logger.Error("[Ollama Managed] Failed to start new container", "code", startCode, "error", startErr)
		return
	}
	logger.Info("[Ollama Managed] Container created and started", "image", image, "port", port, "gpu", gpu.Backend)

	waitForOllamaReady(port, logger)
	pullManagedModels(port, mi.DefaultModels, logger)
}

// StopOllamaManagedContainer stops the managed Ollama container.
func StopOllamaManagedContainer(dockerHost string) string {
	dockerCfg := DockerConfig{Host: dockerHost}
	_, code, err := dockerRequest(dockerCfg, "POST", "/containers/"+ollamaManagedContainerName+"/stop", "")
	if err != nil {
		return fmt.Sprintf(`{"status":"error","message":"Docker unavailable: %s"}`, err)
	}
	if code == 204 || code == 304 {
		return `{"status":"ok","message":"Container stopped."}`
	}
	if code == 404 {
		return `{"status":"error","message":"Managed Ollama container not found."}`
	}
	return fmt.Sprintf(`{"status":"error","message":"Unexpected response code: %d"}`, code)
}

// StartOllamaManagedContainer starts a previously stopped managed Ollama container.
func StartOllamaManagedContainer(dockerHost string) string {
	dockerCfg := DockerConfig{Host: dockerHost}
	_, code, err := dockerRequest(dockerCfg, "POST", "/containers/"+ollamaManagedContainerName+"/start", "")
	if err != nil {
		return fmt.Sprintf(`{"status":"error","message":"Docker unavailable: %s"}`, err)
	}
	if code == 204 || code == 304 {
		return `{"status":"ok","message":"Container started."}`
	}
	if code == 404 {
		return `{"status":"error","message":"Managed Ollama container not found."}`
	}
	return fmt.Sprintf(`{"status":"error","message":"Unexpected response code: %d"}`, code)
}

// RestartOllamaManagedContainer restarts the managed Ollama container.
func RestartOllamaManagedContainer(dockerHost string) string {
	dockerCfg := DockerConfig{Host: dockerHost}
	_, code, err := dockerRequest(dockerCfg, "POST", "/containers/"+ollamaManagedContainerName+"/restart", "")
	if err != nil {
		return fmt.Sprintf(`{"status":"error","message":"Docker unavailable: %s"}`, err)
	}
	if code == 204 {
		return `{"status":"ok","message":"Container restarted."}`
	}
	if code == 404 {
		return `{"status":"error","message":"Managed Ollama container not found."}`
	}
	return fmt.Sprintf(`{"status":"error","message":"Unexpected response code: %d"}`, code)
}

// OllamaManagedContainerStatus returns the status of the managed Ollama container.
func OllamaManagedContainerStatus(dockerHost string) string {
	dockerCfg := DockerConfig{Host: dockerHost}
	data, code, err := dockerRequest(dockerCfg, "GET", "/containers/"+ollamaManagedContainerName+"/json", "")
	if err != nil {
		return fmt.Sprintf(`{"status":"error","message":"Docker unavailable: %s"}`, err)
	}
	if code == 404 {
		return `{"status":"not_found","message":"Managed Ollama container does not exist."}`
	}
	if code != 200 {
		return fmt.Sprintf(`{"status":"error","message":"Unexpected response code: %d"}`, code)
	}
	var info map[string]interface{}
	if json.Unmarshal(data, &info) != nil {
		return `{"status":"error","message":"Failed to parse container info."}`
	}
	state, _ := info["State"].(map[string]interface{})
	running, _ := state["Running"].(bool)
	stateStatus, _ := state["Status"].(string)
	result := map[string]interface{}{
		"status":           "ok",
		"container_name":   ollamaManagedContainerName,
		"running":          running,
		"container_status": stateStatus,
	}
	out, _ := json.Marshal(result)
	return string(out)
}

// OllamaManagedContainerLogs returns the last N lines of the managed Ollama container logs.
func OllamaManagedContainerLogs(dockerHost string, tail int) string {
	if tail <= 0 {
		tail = 100
	}
	dockerCfg := DockerConfig{Host: dockerHost}
	data, code, err := dockerRequest(dockerCfg, "GET",
		fmt.Sprintf("/containers/%s/logs?stdout=true&stderr=true&tail=%d", ollamaManagedContainerName, tail), "")
	if err != nil {
		return fmt.Sprintf(`{"status":"error","message":"Docker unavailable: %s"}`, err)
	}
	if code == 404 {
		return `{"status":"error","message":"Managed Ollama container not found."}`
	}
	if code != 200 {
		return fmt.Sprintf(`{"status":"error","message":"Unexpected response code: %d"}`, code)
	}
	// Docker log stream has 8-byte header per frame; strip for readability
	logs := stripDockerLogHeaders(data)
	result := map[string]interface{}{
		"status": "ok",
		"logs":   logs,
	}
	out, _ := json.Marshal(result)
	return string(out)
}

// pullManagedModels pulls all configured default models into the Ollama instance.
func pullManagedModels(port int, models []string, logger interface {
	Info(string, ...any)
	Warn(string, ...any)
	Error(string, ...any)
}) {
	for _, model := range models {
		model = strings.TrimSpace(model)
		if model == "" {
			continue
		}
		pullModelIfNeeded(port, model, logger)
	}
}

// parseMemoryLimit converts a human-readable memory string (e.g. "8g", "512m")
// to bytes. Returns (bytes, true) on success.
func parseMemoryLimit(s string) (int64, bool) {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return 0, false
	}

	multiplier := int64(1)
	switch {
	case strings.HasSuffix(s, "g"):
		multiplier = 1024 * 1024 * 1024
		s = s[:len(s)-1]
	case strings.HasSuffix(s, "m"):
		multiplier = 1024 * 1024
		s = s[:len(s)-1]
	case strings.HasSuffix(s, "k"):
		multiplier = 1024
		s = s[:len(s)-1]
	}

	var n int64
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, false
		}
		n = n*10 + int64(c-'0')
	}
	if n <= 0 {
		return 0, false
	}
	return n * multiplier, true
}
