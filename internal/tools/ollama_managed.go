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
	if err := validateOllamaLifecycleConfig(cfg); err != nil {
		logger.Error("[Ollama Managed] Configuration refused", "error", err)
		return
	}

	dockerCfg := DockerConfig{Host: cfg.Docker.Host}
	port := mi.ContainerPort
	if port <= 0 {
		port = 11434
	}

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
	payload, err := ollamaManagedCreatePayload(port, mi.VolumePath, mi.MemoryLimit, gpu, mi.UseHostGPU)
	if err != nil {
		logger.Error("[Ollama Managed] Failed to build container payload", "error", err)
		return
	}
	reconcileNamedOllamaContainer(ollamaReconcile{
		Label:         "Ollama Managed",
		ContainerName: ollamaManagedContainerName,
		Port:          port,
		Image:         image,
		GPUBackend:    gpu.Backend,
		Payload:       payload,
		Docker:        dockerCfg,
		Logger:        logger,
		Models:        mi.DefaultModels,
	})
}

func ollamaContainerListHasHostPort(data []byte, port int) bool {
	if port <= 0 {
		return false
	}
	var containers []struct {
		Ports []struct {
			PrivatePort int    `json:"PrivatePort"`
			PublicPort  int    `json:"PublicPort"`
			Type        string `json:"Type"`
		} `json:"Ports"`
	}
	if err := json.Unmarshal(data, &containers); err != nil {
		return false
	}
	for _, container := range containers {
		for _, mapping := range container.Ports {
			if mapping.PublicPort == port && strings.EqualFold(mapping.Type, "tcp") {
				return true
			}
		}
	}
	return false
}

func removeOllamaContainerBestEffort(dockerCfg DockerConfig, containerName string) error {
	data, code, err := dockerRequest(dockerCfg, "POST", "/containers/"+containerName+"/stop?t=5", "")
	if err != nil {
		return fmt.Errorf("stop Ollama container: %w", err)
	}
	if code != 204 && code != 304 && code != 404 {
		return fmt.Errorf("stop Ollama container: %s", dockerBodyMessage(code, data))
	}
	data, code, err = dockerRequest(dockerCfg, "DELETE", "/containers/"+containerName, "")
	if err != nil {
		return fmt.Errorf("remove Ollama container: %w", err)
	}
	if code != 204 && code != 404 {
		return fmt.Errorf("remove Ollama container: %s", dockerBodyMessage(code, data))
	}
	return nil
}

// RemoveOllamaManagedContainer stops and deletes aurago_ollama_managed.
func RemoveOllamaManagedContainer(dockerHost string) {
	removeOllamaContainerBestEffort(DockerConfig{Host: dockerHost}, ollamaManagedContainerName)
}

// StopOllamaManagedContainer stops the managed Ollama container.
func StopOllamaManagedContainer(dockerHost string) string {
	dockerCfg := DockerConfig{Host: dockerHost}
	_, code, err := dockerRequest(dockerCfg, "POST", "/containers/"+ollamaManagedContainerName+"/stop", "")
	if err != nil {
		return ollamaErrJSON("Docker unavailable: " + err.Error())
	}
	if code == 204 || code == 304 {
		return `{"status":"ok","message":"Container stopped."}`
	}
	if code == 404 {
		return ollamaErrJSON("Managed Ollama container not found.")
	}
	return ollamaErrJSON(fmt.Sprintf("Unexpected response code: %d", code))
}

// StartOllamaManagedContainer starts a previously stopped managed Ollama container.
func StartOllamaManagedContainer(dockerHost string) string {
	dockerCfg := DockerConfig{Host: dockerHost}
	_, code, err := dockerRequest(dockerCfg, "POST", "/containers/"+ollamaManagedContainerName+"/start", "")
	if err != nil {
		return ollamaErrJSON("Docker unavailable: " + err.Error())
	}
	if code == 204 || code == 304 {
		return `{"status":"ok","message":"Container started."}`
	}
	if code == 404 {
		return ollamaErrJSON("Managed Ollama container not found.")
	}
	return ollamaErrJSON(fmt.Sprintf("Unexpected response code: %d", code))
}

// RestartOllamaManagedContainer restarts the managed Ollama container.
func RestartOllamaManagedContainer(dockerHost string) string {
	dockerCfg := DockerConfig{Host: dockerHost}
	_, code, err := dockerRequest(dockerCfg, "POST", "/containers/"+ollamaManagedContainerName+"/restart", "")
	if err != nil {
		return ollamaErrJSON("Docker unavailable: " + err.Error())
	}
	if code == 204 {
		return `{"status":"ok","message":"Container restarted."}`
	}
	if code == 404 {
		return ollamaErrJSON("Managed Ollama container not found.")
	}
	return ollamaErrJSON(fmt.Sprintf("Unexpected response code: %d", code))
}

// OllamaManagedContainerStatus returns the status of the managed Ollama container.
func OllamaManagedContainerStatus(dockerHost string) string {
	dockerCfg := DockerConfig{Host: dockerHost}
	data, code, err := dockerRequest(dockerCfg, "GET", "/containers/"+ollamaManagedContainerName+"/json", "")
	if err != nil {
		return ollamaErrJSON("Docker unavailable: " + err.Error())
	}
	if code == 404 {
		out, marshalErr := json.Marshal(map[string]string{"status": "not_found", "message": "Managed Ollama container does not exist."})
		if marshalErr != nil {
			return ollamaErrJSON("internal error")
		}
		return string(out)
	}
	if code != 200 {
		return ollamaErrJSON(fmt.Sprintf("Unexpected response code: %d", code))
	}
	var info map[string]interface{}
	if json.Unmarshal(data, &info) != nil {
		return ollamaErrJSON("Failed to parse container info.")
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
		return ollamaErrJSON("Docker unavailable: " + err.Error())
	}
	if code == 404 {
		return ollamaErrJSON("Managed Ollama container not found.")
	}
	if code != 200 {
		return ollamaErrJSON(fmt.Sprintf("Unexpected response code: %d", code))
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
func pullManagedModels(port int, models []string, label string, logger interface {
	Info(string, ...any)
	Warn(string, ...any)
	Error(string, ...any)
}) {
	for _, model := range models {
		model = strings.TrimSpace(model)
		if model == "" {
			continue
		}
		startOllamaModelPull(port, model, label, logger)
	}
}

// parseMemoryLimit converts a human-readable memory string (e.g. "8g", "512m")
// to bytes. Returns (bytes, true) on success.
func parseMemoryLimit(s string) (int64, bool) {
	return config.ParseOllamaMemoryLimit(s)
}

// ValidateManagedOllamaInputs performs non-mutating preflight for config saves.
func ValidateManagedOllamaInputs(cfg *config.Config) error {
	if err := config.ValidateOllamaSettings(*cfg); err != nil {
		return err
	}
	if cfg.Ollama.ManagedInstance.Enabled && strings.TrimSpace(cfg.Ollama.ManagedInstance.VolumePath) != "" {
		if err := validateDockerBindMount(DockerConfig{Host: cfg.Docker.Host}, cfg.Ollama.ManagedInstance.VolumePath+":/root/.ollama"); err != nil {
			return fmt.Errorf("ollama.managed_instance.volume_path: %w", err)
		}
	}
	return nil
}

func validateOllamaLifecycleConfig(cfg *config.Config) error {
	if !cfg.Docker.Enabled {
		return fmt.Errorf("Docker integration is disabled")
	}
	if cfg.Docker.ReadOnly {
		return ErrDockerReadOnly
	}
	if err := ValidateManagedOllamaInputs(cfg); err != nil {
		return err
	}
	return requireDockerMutationPermission()
}
