package tools

import (
	"encoding/json"
	"fmt"
	"strings"

	"aurago/internal/config"
)

const ollamaManagedContainerName = "aurago_ollama_managed"

// EnsureOllamaManagedRunning ensures a managed Ollama container is running
// for the main Ollama integration. Follows the same idempotent pattern as
// EnsureOllamaEmbeddingsRunning but uses a separate container name, allows
// persistent volume mounts, memory limits and multiple default models.
func EnsureOllamaManagedRunning(cfg *config.Config, logger ollamaEnsureLogger) {
	ensureOllamaRuntime(cfg, false, logger)
}

func removeOllamaContainerBestEffort(dockerCfg DockerConfig, containerName string) error {
	data, code, err := dockerRequest(dockerCfg, "POST", "/containers/"+containerName+"/stop?t=5", "")
	if err != nil {
		return &ollamaUnknownOutcome{fmt.Errorf("stop Ollama container outcome unknown: %w", err)}
	}
	if code != 204 && code != 304 && code != 404 {
		return fmt.Errorf("stop Ollama container: %s", dockerBodyMessage(code, data))
	}
	data, code, err = dockerRequest(dockerCfg, "DELETE", "/containers/"+containerName, "")
	if err != nil {
		return &ollamaUnknownOutcome{fmt.Errorf("remove Ollama container outcome unknown: %w", err)}
	}
	if code != 204 && code != 404 {
		return fmt.Errorf("remove Ollama container: %s", dockerBodyMessage(code, data))
	}
	return nil
}

// OllamaManagedContainerStatus returns the status of the managed Ollama container.
func OllamaManagedContainerStatus(dockerHost string) string {
	return OllamaContainerStatus(dockerHost, ollamaManagedContainerName)
}

func OllamaContainerStatus(dockerHost, containerName string) string {
	if containerName != ollamaManagedContainerName && containerName != ollamaEmbContainerName {
		return ollamaErrJSON("Unknown managed Ollama container")
	}
	dockerCfg := DockerConfig{Host: dockerHost}
	data, code, err := dockerRequest(dockerCfg, "GET", "/containers/"+containerName+"/json", "")
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
		"container_name":   containerName,
		"running":          running,
		"container_status": stateStatus,
	}
	out, _ := json.Marshal(result)
	return string(out)
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
