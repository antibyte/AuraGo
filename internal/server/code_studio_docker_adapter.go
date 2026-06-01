package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"aurago/internal/desktop"
	"aurago/internal/tools"
)

type codeStudioDockerAdapter struct {
	cfg    tools.DockerConfig
	logger *slog.Logger
}

const legacyLocalCodeStudioImage = "aurago/code-studio:latest"
const publishedCodeStudioImage = "ghcr.io/antibyte/aurago-code-studio:latest"
const runtimeFallbackCodeStudioImage = "aurago/code-studio-runtime:latest"

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
		Mounts  []struct {
			Source      string `json:"Source"`
			Destination string `json:"Destination"`
		} `json:"mounts"`
	}
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		return desktop.CodeDockerInspect{}, fmt.Errorf("parse docker inspect response: %w", err)
	}
	if resp.Status != "ok" {
		return desktop.CodeDockerInspect{}, dockerAdapterError(resp.Message)
	}
	mounts := make([]desktop.CodeDockerMount, 0, len(resp.Mounts))
	for _, mount := range resp.Mounts {
		mounts = append(mounts, desktop.CodeDockerMount{
			Source:      mount.Source,
			Destination: mount.Destination,
		})
	}
	return desktop.CodeDockerInspect{ID: resp.ID, Name: resp.Name, State: resp.State, Mounts: mounts}, nil
}

func (a codeStudioDockerAdapter) EnsureImage(ctx context.Context, image string) error {
	exists, err := a.imageExists(ctx, image)
	if err != nil {
		return err
	}
	if strings.EqualFold(image, legacyLocalCodeStudioImage) {
		if exists {
			return nil
		}
		if err := a.buildDefaultImage(ctx, image); err != nil {
			return fmt.Errorf("build default code studio image: %w", err)
		}
		return nil
	}
	if strings.EqualFold(image, publishedCodeStudioImage) {
		return a.refreshPublishedCodeStudioImage(ctx, image, exists)
	}
	if strings.EqualFold(image, runtimeFallbackCodeStudioImage) {
		if err := a.buildDefaultImage(ctx, image); err != nil {
			if exists {
				if a.logger != nil {
					a.logger.Warn("code studio runtime fallback image rebuild failed; using cached image", "image", image, "error", err)
				}
				return nil
			}
			return fmt.Errorf("build code studio runtime fallback image: %w", err)
		}
		return nil
	}
	if exists {
		return nil
	}
	return tools.PullImageWait(ctx, a.cfg, image, a.logger)
}

func (a codeStudioDockerAdapter) refreshPublishedCodeStudioImage(ctx context.Context, image string, exists bool) error {
	if a.logger != nil {
		a.logger.Info("Pulling published Code Studio image; Docker build API is not required", "image", image)
	}
	if err := tools.PullImageWait(ctx, a.cfg, image, a.logger); err != nil {
		if exists {
			if a.logger != nil {
				a.logger.Warn("published Code Studio image refresh failed; using cached image", "image", image, "error", err)
			}
			return nil
		}
		return err
	}
	return nil
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
	content, err := os.ReadFile(dockerfile)
	if err != nil {
		return fmt.Errorf("read code studio Dockerfile %s: %w", dockerfile, err)
	}
	buildCtx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()
	if err := tools.BuildImageWait(buildCtx, a.cfg, image, "Dockerfile", content, map[string]string{"TARGETARCH": codeStudioDockerTargetArch()}, a.logger); err != nil {
		return fmt.Errorf("docker API build failed: %w", err)
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
	options := tools.ContainerCreateOptions{
		User:        req.User,
		SecurityOpt: req.SecurityOpt,
		CapDrop:     req.CapDrop,
		NetworkMode: req.NetworkMode,
	}
	raw := tools.DockerCreateContainerWithOptions(a.cfg, req.Name, req.Image, req.Env, req.Ports, req.Volumes, req.Cmd, req.Restart, resources, options)
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
	raw := tools.DockerContainerAction(a.cfg, container, action, action == "remove" || action == "rm")
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

func (a codeStudioDockerAdapter) ExecContainer(ctx context.Context, container string, cmd []string, user string, timeout time.Duration) (desktop.CodeDockerExecResult, error) {
	result, err := a.exec(ctx, container, cmd, user, timeout)
	if err != nil {
		return desktop.CodeDockerExecResult{}, err
	}
	return desktop.CodeDockerExecResult{ExitCode: result.ExitCode, Output: result.Output}, nil
}

func dockerAdapterError(message string) error {
	message = strings.TrimSpace(message)
	if message == "" {
		message = "docker request failed"
	}
	return fmt.Errorf("%s", message)
}
