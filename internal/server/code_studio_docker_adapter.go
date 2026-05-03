package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"aurago/internal/desktop"
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

func (a codeStudioDockerAdapter) PullImage(ctx context.Context, image string) error {
	return tools.PullImageWait(ctx, a.cfg, image, a.logger)
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
