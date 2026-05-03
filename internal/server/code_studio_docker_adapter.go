package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"aurago/internal/desktop"
	"aurago/internal/sandbox"
	"aurago/internal/tools"
)

type codeStudioDockerAdapter struct {
	cfg    tools.DockerConfig
	logger *slog.Logger
}

func newCodeStudioDockerAdapter(cfg desktop.Config, logger *slog.Logger) codeStudioDockerAdapter {
	return codeStudioDockerAdapter{
		cfg: tools.DockerConfig{
			Host:         cfg.DockerHost,
			WorkspaceDir: cfg.WorkspaceDir,
		},
		logger: logger,
	}
}

func (a codeStudioDockerAdapter) ListContainers(ctx context.Context, all bool) ([]desktop.CodeDockerContainer, error) {
	raw := tools.DockerListContainers(a.cfg, all)
	var resp struct {
		Status     string `json:"status"`
		Message    string `json:"message"`
		Containers []struct {
			ID    string   `json:"id"`
			Names []string `json:"names"`
		} `json:"containers"`
	}
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		return nil, fmt.Errorf("parse docker containers response: %w", err)
	}
	if resp.Status != "ok" {
		return nil, dockerAdapterError(resp.Message)
	}
	containers := make([]desktop.CodeDockerContainer, 0, len(resp.Containers))
	for _, container := range resp.Containers {
		containers = append(containers, desktop.CodeDockerContainer{ID: container.ID, Names: container.Names})
	}
	return containers, nil
}

func (a codeStudioDockerAdapter) InspectContainer(ctx context.Context, container string) (desktop.CodeDockerInspect, error) {
	raw := tools.DockerInspectContainer(a.cfg, container)
	var resp struct {
		Status  string                  `json:"status"`
		Message string                  `json:"message"`
		ID      string                  `json:"id"`
		Name    string                  `json:"name"`
		State   desktop.CodeDockerState `json:"state"`
	}
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		return desktop.CodeDockerInspect{}, fmt.Errorf("parse docker inspect response: %w", err)
	}
	if resp.Status != "ok" {
		return desktop.CodeDockerInspect{}, dockerAdapterError(resp.Message)
	}
	return desktop.CodeDockerInspect{ID: resp.ID, Name: resp.Name, State: resp.State}, nil
}

func (a codeStudioDockerAdapter) EnsureImage(ctx context.Context, image string) error {
	exists, err := a.imageExists(ctx, image)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	if image == "aurago/code-studio:latest" {
		if err := a.buildDefaultImage(ctx, image); err != nil {
			return fmt.Errorf("build default code studio image: %w", err)
		}
		return nil
	}
	return tools.PullImageWait(ctx, a.cfg, image, a.logger)
}

func (a codeStudioDockerAdapter) imageExists(ctx context.Context, image string) (bool, error) {
	data, code, err := tools.DockerRequestContext(ctx, a.cfg, "GET", "/images/"+url.PathEscape(image)+"/json", "")
	if err != nil {
		return false, fmt.Errorf("inspect code studio image %s: %w", image, err)
	}
	if code == 200 {
		return true, nil
	}
	if code == 404 {
		return false, nil
	}
	return false, fmt.Errorf("inspect code studio image %s returned HTTP %d: %s", image, code, strings.TrimSpace(string(data)))
}

func (a codeStudioDockerAdapter) buildDefaultImage(ctx context.Context, image string) error {
	dockerfile := filepath.Join("deploy", "docker", "Dockerfile.code-studio")
	if _, err := os.Stat(dockerfile); err != nil {
		return fmt.Errorf("code studio Dockerfile is missing at %s: %w", dockerfile, err)
	}
	if a.logger != nil {
		a.logger.Info("Building Code Studio image from local Dockerfile", "image", image, "dockerfile", dockerfile)
	}
	buildCtx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(buildCtx, "docker", "build",
		"--build-arg", "TARGETARCH="+codeStudioDockerTargetArch(),
		"-f", dockerfile,
		"-t", image,
		".",
	)
	dockerCfgDir := filepath.Join("data", ".docker")
	_ = os.MkdirAll(dockerCfgDir, 0o700)
	cmd.Env = append(sandbox.FilterEnv(os.Environ()), "DOCKER_CONFIG="+dockerCfgDir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker build failed: %w\n%s", err, strings.TrimSpace(string(out)))
	}
	if a.logger != nil {
		a.logger.Info("Code Studio image built successfully", "image", image)
	}
	return nil
}

func codeStudioDockerTargetArch() string {
	switch runtime.GOARCH {
	case "amd64", "arm64":
		return runtime.GOARCH
	default:
		return "amd64"
	}
}

func (a codeStudioDockerAdapter) CreateContainer(ctx context.Context, req desktop.CodeDockerCreateRequest) (string, error) {
	var resources *tools.ContainerResources
	if req.Resources != nil {
		resources = &tools.ContainerResources{
			MemoryMB:  req.Resources.MemoryMB,
			CPUCores:  req.Resources.CPUCores,
			PidsLimit: req.Resources.PidsLimit,
		}
	}
	raw := tools.DockerCreateContainer(a.cfg, req.Name, req.Image, req.Env, req.Ports, req.Volumes, req.Cmd, req.Restart, resources)
	var resp struct {
		Status  string `json:"status"`
		Message string `json:"message"`
		ID      string `json:"id"`
	}
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		return "", fmt.Errorf("parse docker create response: %w", err)
	}
	if resp.Status != "ok" {
		return "", dockerAdapterError(resp.Message)
	}
	return resp.ID, nil
}

func (a codeStudioDockerAdapter) ContainerAction(ctx context.Context, container, action string) error {
	raw := tools.DockerContainerAction(a.cfg, container, action, false)
	var resp struct {
		Status  string `json:"status"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		return fmt.Errorf("parse docker action response: %w", err)
	}
	if resp.Status != "ok" {
		return dockerAdapterError(resp.Message)
	}
	return nil
}

func dockerAdapterError(message string) error {
	message = strings.TrimSpace(message)
	if message == "" {
		message = "docker request failed"
	}
	return fmt.Errorf("%s", message)
}
