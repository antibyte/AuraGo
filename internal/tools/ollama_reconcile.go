package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"aurago/internal/config"
)

func runOllamaReconcile(work *ollamaWork, cfg *config.Config, embeddings, force bool, logger ollamaEnsureLogger) error {
	if err := validateOllamaLifecycleConfig(cfg); err != nil {
		return err
	}
	phase := func(state string) { ollamaRuntimeWork.phase(work, state) }
	modelState := func(model, state string) {
		ollamaRuntimeWork.mu.Lock()
		defer ollamaRuntimeWork.mu.Unlock()
		if work.Models == nil {
			work.Models = map[string]string{}
		}
		work.Models[model] = state
	}
	phase("checking")
	network, err := ollamaNetwork(cfg, work.ctx)
	if err != nil {
		return err
	}
	name, port, models, useGPU, backend := ollamaManagedContainerName, cfg.Ollama.ManagedInstance.ContainerPort, cfg.Ollama.ManagedInstance.DefaultModels, cfg.Ollama.ManagedInstance.UseHostGPU, cfg.Ollama.ManagedInstance.GPUBackend
	if embeddings {
		name, port, models, useGPU, backend = ollamaEmbContainerName, cfg.Embeddings.LocalOllama.ContainerPort, []string{cfg.Embeddings.LocalOllama.Model}, cfg.Embeddings.LocalOllama.UseHostGPU, cfg.Embeddings.LocalOllama.GPUBackend
	}
	var gpu GPUInfo
	if useGPU {
		gpu = DetectGPU(backend)
		if gpu.Backend == "none" {
			logger.Warn("[Ollama] GPU is not visible to AuraGo", "container", name)
			if cfg.Runtime.IsDocker {
				return fmt.Errorf("GPU access cannot be proven from this container; expose the GPU to AuraGo for detection or explicitly disable use_host_gpu")
			}
		}
	}
	var payload []byte
	if embeddings {
		payload, err = ollamaEmbeddingsCreatePayload(port, gpu)
	} else {
		payload, err = ollamaManagedCreatePayload(port, cfg.Ollama.ManagedInstance.VolumePath, cfg.Ollama.ManagedInstance.MemoryLimit, gpu, useGPU)
	}
	if err != nil {
		return err
	}
	payload, err = ollamaPlacePayload(payload, network)
	if err != nil {
		return err
	}
	return reconcileNamedOllamaContainer(ollamaReconcile{Label: "Ollama", ContainerName: name, Port: port, Image: ollamaImageForGPU(gpu), GPUBackend: gpu.Backend, Payload: payload, Docker: DockerConfig{Context: work.ctx, Host: cfg.Docker.Host}, Logger: logger, Models: models, BaseURL: config.ManagedOllamaURL(cfg.Runtime.IsDocker, embeddings, port), Force: force, RequireGPU: useGPU, InDocker: cfg.Runtime.IsDocker, Phase: phase, ModelState: modelState})
}

// reconcileNamedOllamaContainer performs one serialized, cancellation-bound
// transaction. A lost mutation response is returned, never automatically replayed.
func reconcileNamedOllamaContainer(job ollamaReconcile) error {
	ctx := nativeHTTPContext(job.Docker.Context)
	phase := job.Phase
	if phase == nil {
		phase = func(string) {}
	}
	if err := requireDockerMutationPermission(); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	data, code, err := dockerRequest(job.Docker, "GET", "/containers/"+job.ContainerName+"/json", "")
	if err != nil {
		return fmt.Errorf("inspect Ollama container: %w", err)
	}
	if code != 200 && code != 404 {
		return fmt.Errorf("inspect Ollama returned HTTP %d", code)
	}
	if code == 200 && !ollamaOwnsInspect(data, job.Payload) {
		return fmt.Errorf("container %s belongs to an unknown workload; leaving it untouched", job.ContainerName)
	}
	if code == 404 || job.Force || ollamaManagedSpecDrift(data, job.Payload) {
		// Device paths belong to the daemon. A local stat is valid only for a
		// native process on the same Engine host; never discard an existing GPU
		// container because /dev is not mounted in AuraGo's own namespace.
		var desired ollamaCreateBody
		if err := json.Unmarshal(job.Payload, &desired); err != nil {
			return fmt.Errorf("decode desired Ollama spec: %w", err)
		}
		var old struct {
			ID         string `json:"Id"`
			HostConfig ollamaCreateHostConfig
		}
		if code == 200 {
			if err := json.Unmarshal(data, &old); err != nil {
				return err
			}
			if job.RequireGPU && (len(old.HostConfig.Devices) > 0 || len(old.HostConfig.DeviceRequests) > 0) && len(desired.HostConfig.Devices) == 0 && len(desired.HostConfig.DeviceRequests) == 0 {
				return fmt.Errorf("GPU detection changed; existing Ollama container was preserved")
			}
			if job.InDocker && len(desired.HostConfig.Devices) > 0 {
				return fmt.Errorf("cannot verify daemon device availability before replacement; existing GPU container was preserved")
			}
		}
		if !job.InDocker {
			for _, device := range desired.HostConfig.Devices {
				if _, err := os.Stat(device.PathOnHost); err != nil {
					return fmt.Errorf("replacement GPU device unavailable: %w", err)
				}
			}
		}
		phase("image_pull")
		if err := ensureOllamaImage(job.Docker, job.Image); err != nil {
			return fmt.Errorf("replacement image unavailable; existing container preserved: %w", err)
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if code == 200 {
			if old.ID == "" {
				return fmt.Errorf("missing Docker container ID; replacement refused")
			}
			if err := removeOllamaContainerBestEffort(job.Docker, old.ID); err != nil {
				return err
			}
		}
		phase("creating")
		created, createCode, createErr := dockerRequest(job.Docker, "POST", "/containers/create?name="+job.ContainerName, string(job.Payload))
		if createErr != nil {
			return &ollamaUnknownOutcome{fmt.Errorf("create Ollama container (outcome unknown; inspect before retry): %w", createErr)}
		}
		if createCode != 201 {
			return fmt.Errorf("create Ollama container: %s", dockerBodyMessage(createCode, created))
		}
		code = 404
	}
	if code == 404 || !ollamaInspectRunning(data) {
		phase("starting")
		body, status, err := dockerRequest(job.Docker, "POST", "/containers/"+job.ContainerName+"/start", "")
		if err != nil {
			return &ollamaUnknownOutcome{fmt.Errorf("start Ollama container (outcome unknown; inspect before retry): %w", err)}
		}
		if status != 204 && status != 304 {
			return fmt.Errorf("start Ollama container: %s", dockerBodyMessage(status, body))
		}
	}
	base := job.BaseURL
	if base == "" {
		base = config.ManagedOllamaURL(false, job.ContainerName == ollamaEmbContainerName, job.Port)
	}
	phase("starting")
	if err := ollamaWaitReady(ctx, base); err != nil {
		return err
	}
	phase("api_ready")
	for _, model := range job.Models {
		model = strings.TrimSpace(model)
		if model == "" {
			continue
		}
		phase("model_pull")
		if job.ModelState != nil {
			job.ModelState(model, "model_pull")
		}
		if err := requireDockerMutationPermission(); err != nil {
			return err
		}
		if err := ollamaEnsureModel(OllamaConfig{Context: ctx, URL: base}, model); err != nil {
			if job.ModelState != nil {
				job.ModelState(model, "failed")
			}
			return err
		}
		if job.ModelState != nil {
			job.ModelState(model, "ready")
		}
	}
	return nil
}

func ollamaWaitReady(ctx context.Context, base string) error {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		if _, err := OllamaVersion(ctx, base); err == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("Ollama API did not become ready: %w", ctx.Err())
		case <-ticker.C:
		}
	}
}

func OllamaVersion(ctx context.Context, base string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	data, code, err := ollamaRequest(OllamaConfig{Context: ctx, URL: base}, http.MethodGet, "/api/version", "")
	if err != nil {
		return "", err
	}
	var response struct {
		Version string `json:"version"`
	}
	if code != 200 || json.Unmarshal(data, &response) != nil || strings.TrimSpace(response.Version) == "" {
		return "", fmt.Errorf("endpoint did not return an Ollama version")
	}
	return response.Version, nil
}

func ollamaEnsureModel(cfg OllamaConfig, model string) error {
	data, code, err := ollamaRequest(cfg, "POST", "/api/show", marshalToolJSON(map[string]any{"model": model}))
	if err != nil {
		return err
	}
	if code == 200 {
		var response map[string]json.RawMessage
		if json.Unmarshal(data, &response) == nil && response["error"] == nil && (response["details"] != nil || response["model_info"] != nil) {
			return nil
		}
		return fmt.Errorf("invalid Ollama model metadata")
	}
	if code != 404 {
		return fmt.Errorf("Ollama model availability returned HTTP %d", code)
	}
	return ollamaPullShared(cfg, model)
}
