package desktop

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	codeContainerName             = "aurago-code-studio"
	defaultCodeContainerImage     = "aurago/code-studio:latest"
	codeWorkspaceInContainer      = "/workspace"
	codeWorkspaceDirName          = "code-studio-workspace"
	defaultCodeContainerMemoryMB  = 4096
	defaultCodeContainerCPUCores  = 2
	defaultCodeContainerPidsLimit = 1024
	defaultCodeContainerStopAfter = 30 * time.Minute
)

type ContainerState int

const (
	StateStopped ContainerState = iota
	StateStarting
	StateRunning
	StateError
)

type CodeContainerService struct {
	cfg           Config
	logger        *slog.Logger
	docker        CodeContainerDocker
	mu            sync.RWMutex
	state         ContainerState
	lastActivity  time.Time
	stopTimer     *time.Timer
	autoStopAfter time.Duration
}

// CodeContainerDocker is the Docker backend used by Code Studio.
type CodeContainerDocker interface {
	ListContainers(ctx context.Context, all bool) ([]CodeDockerContainer, error)
	InspectContainer(ctx context.Context, container string) (CodeDockerInspect, error)
	EnsureImage(ctx context.Context, image string) error
	CreateContainer(ctx context.Context, req CodeDockerCreateRequest) (string, error)
	ContainerAction(ctx context.Context, container, action string) error
}

// CodeDockerContainer is the subset of Docker container metadata Code Studio needs.
type CodeDockerContainer struct {
	ID    string
	Names []string
}

// CodeDockerState is the subset of Docker inspect state Code Studio needs.
type CodeDockerState struct {
	Running bool `json:"Running"`
}

// CodeDockerInspect is the subset of Docker inspect data Code Studio needs.
type CodeDockerInspect struct {
	ID    string          `json:"id"`
	Name  string          `json:"name"`
	State CodeDockerState `json:"state"`
}

// CodeDockerCreateRequest describes the container Code Studio wants to create.
type CodeDockerCreateRequest struct {
	Name      string
	Image     string
	Env       []string
	Ports     map[string]string
	Volumes   []string
	Cmd       []string
	Restart   string
	Resources *CodeContainerResources
}

// CodeContainerResources holds Docker resource limits for Code Studio.
type CodeContainerResources struct {
	MemoryMB  int
	CPUCores  int
	PidsLimit int
}

// CodeContainerStatus is safe to expose through the future Code Studio status API.
type CodeContainerStatus struct {
	Enabled                bool                   `json:"enabled"`
	State                  string                 `json:"state"`
	Running                bool                   `json:"running"`
	ContainerID            string                 `json:"container_id,omitempty"`
	Image                  string                 `json:"image"`
	WorkspaceHostPath      string                 `json:"workspace_host_path"`
	WorkspaceContainerPath string                 `json:"workspace_container_path"`
	AutoStopMinutes        int                    `json:"auto_stop_minutes"`
	LastActivity           time.Time              `json:"last_activity,omitempty"`
	Resources              CodeContainerResources `json:"resources"`
	Error                  string                 `json:"error,omitempty"`
}

func NewCodeContainerService(cfg Config, logger *slog.Logger) *CodeContainerService {
	if logger == nil {
		logger = slog.Default()
	}
	return &CodeContainerService{
		cfg:           cfg,
		logger:        logger,
		docker:        missingCodeContainerDocker{},
		state:         StateStopped,
		autoStopAfter: defaultCodeContainerStopAfter,
	}
}

// SetDockerClient installs the concrete Docker backend used by lazy startup.
func (s *CodeContainerService) SetDockerClient(docker CodeContainerDocker) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if docker == nil {
		s.docker = missingCodeContainerDocker{}
		return
	}
	s.docker = docker
}

func (s *CodeContainerService) EnsureStarted(ctx context.Context) error {
	if !s.cfg.CodeStudio.Enabled {
		return fmt.Errorf("code studio is disabled")
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state == StateRunning {
		s.touchLocked()
		return nil
	}
	if s.state == StateStarting {
		return fmt.Errorf("code studio container is already starting")
	}

	s.state = StateStarting
	workspaceDir := s.cfg.WorkspaceDir
	if strings.TrimSpace(workspaceDir) == "" {
		s.state = StateError
		return fmt.Errorf("desktop workspace directory is required")
	}
	codeWorkspaceDir := filepath.Join(workspaceDir, codeWorkspaceDirName)
	if err := os.MkdirAll(codeWorkspaceDir, 0o755); err != nil {
		s.state = StateError
		return fmt.Errorf("create code studio workspace: %w", err)
	}

	containerID, running, err := s.findContainerLocked(ctx)
	if err != nil {
		s.state = StateError
		return err
	}
	if containerID != "" {
		if !running {
			if err := s.docker.ContainerAction(ctx, containerID, "start"); err != nil {
				s.state = StateError
				return fmt.Errorf("start code studio container: %w", err)
			}
			if err := s.waitForRunningLocked(ctx, containerID); err != nil {
				s.state = StateError
				return err
			}
		}
		s.state = StateRunning
		s.touchLocked()
		return nil
	}

	image := codeStudioImage(s.cfg.CodeStudio)
	if err := s.docker.EnsureImage(ctx, image); err != nil {
		s.state = StateError
		return fmt.Errorf("prepare code studio image %s: %w", image, err)
	}

	createdID, err := s.docker.CreateContainer(ctx, CodeDockerCreateRequest{
		Name:      codeContainerName,
		Image:     image,
		Env:       codeContainerEnv(),
		Ports:     map[string]string{},
		Volumes:   []string{codeWorkspaceDir + ":" + codeWorkspaceInContainer},
		Cmd:       []string{"sleep", "infinity"},
		Restart:   "unless-stopped",
		Resources: codeStudioResourcesPtr(s.cfg.CodeStudio),
	})
	if err != nil {
		s.state = StateError
		return fmt.Errorf("create code studio container: %w", err)
	}
	if createdID == "" {
		createdID = codeContainerName
	}
	if err := s.docker.ContainerAction(ctx, createdID, "start"); err != nil {
		s.state = StateError
		return fmt.Errorf("start code studio container: %w", err)
	}
	if err := s.waitForRunningLocked(ctx, createdID); err != nil {
		s.state = StateError
		return err
	}
	s.state = StateRunning
	s.touchLocked()
	return nil
}

func (s *CodeContainerService) Stop(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stopTimer != nil {
		s.stopTimer.Stop()
		s.stopTimer = nil
	}
	if s.state != StateRunning {
		return nil
	}
	if err := s.docker.ContainerAction(ctx, codeContainerName, "stop"); err != nil {
		return fmt.Errorf("stop code studio container: %w", err)
	}
	s.state = StateStopped
	return nil
}

func (s *CodeContainerService) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state == StateRunning
}

func (s *CodeContainerService) State() ContainerState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state
}

func (s *CodeContainerService) ContainerID(ctx context.Context) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	containerID, _, err := s.findContainerLocked(ctx)
	if err != nil {
		return "", err
	}
	if containerID == "" {
		return "", fmt.Errorf("code studio container not found")
	}
	return containerID, nil
}

// Status returns cached service state plus best-effort Docker state.
func (s *CodeContainerService) Status(ctx context.Context) CodeContainerStatus {
	s.mu.RLock()
	status := CodeContainerStatus{
		Enabled:                s.cfg.CodeStudio.Enabled,
		State:                  s.state.String(),
		Running:                s.state == StateRunning,
		Image:                  codeStudioImage(s.cfg.CodeStudio),
		WorkspaceHostPath:      filepath.Join(s.cfg.WorkspaceDir, codeWorkspaceDirName),
		WorkspaceContainerPath: codeWorkspaceInContainer,
		AutoStopMinutes:        s.cfg.CodeStudio.AutoStopMinutes,
		LastActivity:           s.lastActivity,
		Resources:              codeStudioResources(s.cfg.CodeStudio),
	}
	s.mu.RUnlock()

	containerID, running, err := s.findContainer(ctx)
	if err != nil {
		status.Error = err.Error()
		return status
	}
	status.ContainerID = containerID
	status.Running = running
	if running {
		status.State = StateRunning.String()
	}
	return status
}

func (s *CodeContainerService) findContainer(ctx context.Context) (string, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.findContainerLocked(ctx)
}

func (s ContainerState) String() string {
	switch s {
	case StateStopped:
		return "stopped"
	case StateStarting:
		return "starting"
	case StateRunning:
		return "running"
	case StateError:
		return "error"
	default:
		return "unknown"
	}
}

func (s *CodeContainerService) touchLocked() {
	s.lastActivity = time.Now()
	s.resetAutoStopTimerLocked()
}

func (s *CodeContainerService) resetAutoStopTimerLocked() {
	if s.stopTimer != nil {
		s.stopTimer.Stop()
	}
	timeout := s.autoStopAfter
	if s.cfg.CodeStudio.AutoStopMinutes > 0 {
		timeout = time.Duration(s.cfg.CodeStudio.AutoStopMinutes) * time.Minute
	}
	if timeout <= 0 {
		s.stopTimer = nil
		return
	}
	s.stopTimer = time.AfterFunc(timeout, func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := s.Stop(ctx); err != nil {
			s.logger.Warn("code studio auto-stop failed", "error", err)
		}
	})
}

func (s *CodeContainerService) findContainerLocked(ctx context.Context) (string, bool, error) {
	containers, err := s.docker.ListContainers(ctx, true)
	if err != nil {
		return "", false, fmt.Errorf("list docker containers: %w", err)
	}
	for _, container := range containers {
		if !codeContainerMatches(container) {
			continue
		}
		inspect, err := s.docker.InspectContainer(ctx, container.ID)
		if err != nil {
			return "", false, fmt.Errorf("inspect code studio container: %w", err)
		}
		return container.ID, inspect.State.Running, nil
	}
	return "", false, nil
}

func (s *CodeContainerService) waitForRunningLocked(ctx context.Context, containerID string) error {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	deadline := time.After(30 * time.Second)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline:
			return fmt.Errorf("timeout waiting for code studio container to start")
		case <-ticker.C:
			inspect, err := s.docker.InspectContainer(ctx, containerID)
			if err == nil && inspect.State.Running {
				return nil
			}
		}
	}
}

func codeContainerMatches(container CodeDockerContainer) bool {
	if container.ID == codeContainerName {
		return true
	}
	for _, name := range container.Names {
		if strings.TrimPrefix(name, "/") == codeContainerName {
			return true
		}
	}
	return false
}

func codeContainerEnv() []string {
	return []string{
		"HOME=/home/developer",
		"USER=developer",
		"GOPATH=/home/developer/go",
		"CARGO_HOME=/usr/local/cargo",
	}
}

func codeStudioImage(cfg CodeStudioConfig) string {
	image := strings.TrimSpace(cfg.Image)
	if image == "" {
		return defaultCodeContainerImage
	}
	return image
}

func codeStudioResources(cfg CodeStudioConfig) CodeContainerResources {
	return CodeContainerResources{
		MemoryMB:  codeStudioMemoryMB(cfg),
		CPUCores:  codeStudioCPUCores(cfg),
		PidsLimit: defaultCodeContainerPidsLimit,
	}
}

func codeStudioResourcesPtr(cfg CodeStudioConfig) *CodeContainerResources {
	resources := codeStudioResources(cfg)
	return &resources
}

func codeStudioMemoryMB(cfg CodeStudioConfig) int {
	if cfg.MaxMemoryMB > 0 {
		return cfg.MaxMemoryMB
	}
	return defaultCodeContainerMemoryMB
}

func codeStudioCPUCores(cfg CodeStudioConfig) int {
	if cfg.MaxCPUCores > 0 {
		return cfg.MaxCPUCores
	}
	return defaultCodeContainerCPUCores
}

type missingCodeContainerDocker struct{}

func (missingCodeContainerDocker) ListContainers(ctx context.Context, all bool) ([]CodeDockerContainer, error) {
	return nil, fmt.Errorf("code studio Docker backend is not configured")
}

func (missingCodeContainerDocker) InspectContainer(ctx context.Context, container string) (CodeDockerInspect, error) {
	return CodeDockerInspect{}, fmt.Errorf("code studio Docker backend is not configured")
}

func (missingCodeContainerDocker) EnsureImage(ctx context.Context, image string) error {
	return fmt.Errorf("code studio Docker backend is not configured")
}

func (missingCodeContainerDocker) CreateContainer(ctx context.Context, req CodeDockerCreateRequest) (string, error) {
	return "", fmt.Errorf("code studio Docker backend is not configured")
}

func (missingCodeContainerDocker) ContainerAction(ctx context.Context, container, action string) error {
	return fmt.Errorf("code studio Docker backend is not configured")
}
