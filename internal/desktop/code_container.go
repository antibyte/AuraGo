package desktop

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	codeContainerName                 = "aurago-code-studio"
	defaultCodeContainerImage         = "ghcr.io/antibyte/aurago-code-studio:latest"
	runtimeFallbackCodeContainerImage = "aurago/code-studio-runtime:latest"
	codeWorkspaceInContainer          = "/workspace"
	codeWorkspaceDirName              = "code-studio-workspace"
	defaultCodeContainerMemoryMB      = 4096
	defaultCodeContainerCPUCores      = 2
	defaultCodeContainerPidsLimit     = 1024
	defaultCodeContainerStopAfter     = 30 * time.Minute
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
	ExecContainer(ctx context.Context, container string, cmd []string, user string, timeout time.Duration) (CodeDockerExecResult, error)
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
	ID     string            `json:"id"`
	Name   string            `json:"name"`
	State  CodeDockerState   `json:"state"`
	Mounts []CodeDockerMount `json:"mounts,omitempty"`
}

type CodeDockerMount struct {
	Source      string `json:"source"`
	Destination string `json:"destination"`
}

// CodeDockerExecResult is the subset of Docker exec output Code Studio needs.
type CodeDockerExecResult struct {
	ExitCode int
	Output   string
}

// CodeDockerCreateRequest describes the container Code Studio wants to create.
type CodeDockerCreateRequest struct {
	Name        string
	Image       string
	Env         []string
	Ports       map[string]string
	Volumes     []string
	Cmd         []string
	Restart     string
	NetworkMode string
	User        string
	SecurityOpt []string
	CapDrop     []string
	Resources   *CodeContainerResources
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
		if _, err := s.ensureWorkspaceLocked(); err != nil {
			s.state = StateError
			return err
		}
		containerID, running, err := s.findContainerLocked(ctx)
		if err != nil {
			s.state = StateError
			return err
		}
		if running {
			if err := seedCodeStudioContainerWorkspace(ctx, s.docker, containerID); err != nil {
				s.state = StateError
				return err
			}
		}
		s.touchLocked()
		return nil
	}
	if s.state == StateStarting {
		return fmt.Errorf("code studio container is already starting")
	}

	s.state = StateStarting
	codeWorkspaceDir, err := s.ensureWorkspaceLocked()
	if err != nil {
		s.state = StateError
		return err
	}

	containerID, running, err := s.findContainerLocked(ctx)
	if err != nil {
		s.state = StateError
		return err
	}
	if containerID != "" {
		currentMount, err := s.usesCurrentWorkspaceMountLocked(ctx, containerID, codeWorkspaceDir)
		if err != nil {
			s.state = StateError
			return err
		}
		if !currentMount {
			if err := s.docker.ContainerAction(ctx, containerID, "remove"); err != nil {
				s.state = StateError
				return fmt.Errorf("replace legacy code studio container: %w", err)
			}
			containerID = ""
			running = false
		}
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
		runtimeMissing, err := s.defaultContainerRuntimeMissingLocked(ctx, containerID)
		if err != nil {
			s.state = StateError
			return err
		}
		if runtimeMissing {
			if err := s.docker.ContainerAction(ctx, containerID, "remove"); err != nil {
				s.state = StateError
				return fmt.Errorf("replace code studio container with missing runtime tools: %w", err)
			}
			containerID = ""
			running = false
		}
	}
	if containerID != "" {
		if err := seedCodeStudioContainerWorkspace(ctx, s.docker, containerID); err != nil {
			s.state = StateError
			return err
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

	createdID, err := s.createCodeStudioContainerLocked(ctx, image, codeWorkspaceDir)
	if err != nil {
		s.state = StateError
		return err
	}
	runtimeMissing, err := s.defaultContainerRuntimeMissingLocked(ctx, createdID)
	if err != nil {
		s.state = StateError
		return err
	}
	if runtimeMissing {
		if err := s.docker.ContainerAction(ctx, createdID, "remove"); err != nil {
			s.state = StateError
			return fmt.Errorf("remove code studio container with missing runtime tools: %w", err)
		}
		fallbackImage := runtimeFallbackCodeContainerImage
		if err := s.docker.EnsureImage(ctx, fallbackImage); err != nil {
			s.state = StateError
			return fmt.Errorf("prepare code studio runtime fallback image %s: %w", fallbackImage, err)
		}
		createdID, err = s.createCodeStudioContainerLocked(ctx, fallbackImage, codeWorkspaceDir)
		if err != nil {
			s.state = StateError
			return err
		}
		runtimeMissing, err = s.containerRuntimeMissingLocked(ctx, createdID)
		if err != nil {
			s.state = StateError
			return err
		}
		if runtimeMissing {
			s.state = StateError
			return fmt.Errorf("code studio runtime fallback image %s is missing required runtime tools", fallbackImage)
		}
	}
	if err := seedCodeStudioContainerWorkspace(ctx, s.docker, createdID); err != nil {
		s.state = StateError
		return err
	}
	s.state = StateRunning
	s.touchLocked()
	return nil
}

func (s *CodeContainerService) createCodeStudioContainerLocked(ctx context.Context, image, codeWorkspaceDir string) (string, error) {
	createdID, err := s.docker.CreateContainer(ctx, CodeDockerCreateRequest{
		Name:        codeContainerName,
		Image:       image,
		Env:         codeContainerEnv(),
		Ports:       map[string]string{},
		Volumes:     []string{codeWorkspaceDir + ":" + codeWorkspaceInContainer},
		Cmd:         []string{"sleep", "infinity"},
		Restart:     "no",
		NetworkMode: "none",
		User:        "developer",
		SecurityOpt: []string{"no-new-privileges:true"},
		CapDrop:     []string{"ALL"},
		Resources:   codeStudioResourcesPtr(s.cfg.CodeStudio),
	})
	if err != nil {
		return "", fmt.Errorf("create code studio container: %w", err)
	}
	if createdID == "" {
		createdID = codeContainerName
	}
	if err := s.docker.ContainerAction(ctx, createdID, "start"); err != nil {
		return "", fmt.Errorf("start code studio container: %w", err)
	}
	if err := s.waitForRunningLocked(ctx, createdID); err != nil {
		return "", err
	}
	return createdID, nil
}

func (s *CodeContainerService) defaultContainerRuntimeMissingLocked(ctx context.Context, containerID string) (bool, error) {
	if !strings.EqualFold(codeStudioImage(s.cfg.CodeStudio), defaultCodeContainerImage) {
		return false, nil
	}
	return s.containerRuntimeMissingLocked(ctx, containerID)
}

func (s *CodeContainerService) containerRuntimeMissingLocked(ctx context.Context, containerID string) (bool, error) {
	result, err := s.docker.ExecContainer(ctx, containerID, []string{"sh", "-lc", buildCodeStudioRuntimeProbeScript()}, "", 30*time.Second)
	if err != nil {
		return false, fmt.Errorf("check code studio container runtime tools: %w", err)
	}
	if result.ExitCode == 0 {
		return false, nil
	}
	if s.logger != nil {
		s.logger.Warn("code studio default container missing required runtime tools", "container", containerID, "output", strings.TrimSpace(result.Output))
	}
	return true, nil
}

func buildCodeStudioRuntimeProbeScript() string {
	return strings.Join([]string{
		"command -v node >/dev/null 2>&1 || { echo 'node not found'; exit 127; }",
		"command -v python3 >/dev/null 2>&1 || { echo 'python3 not found'; exit 127; }",
		"command -v go >/dev/null 2>&1 || { echo 'go not found'; exit 127; }",
	}, "\n")
}

func (s *CodeContainerService) ensureWorkspaceLocked() (string, error) {
	workspaceDir := s.cfg.WorkspaceDir
	if strings.TrimSpace(workspaceDir) == "" {
		return "", fmt.Errorf("desktop workspace directory is required")
	}
	codeWorkspaceDir := workspaceDir
	if err := os.MkdirAll(codeWorkspaceDir, 0o700); err != nil {
		return "", fmt.Errorf("create code studio workspace: %w", err)
	}
	_ = os.Chmod(codeWorkspaceDir, 0o700)
	if err := seedCodeStudioWorkspace(codeWorkspaceDir); err != nil {
		return "", fmt.Errorf("seed code studio workspace: %w", err)
	}
	return codeWorkspaceDir, nil
}

func seedCodeStudioWorkspace(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	if len(entries) > 0 {
		return nil
	}
	for name, content := range defaultCodeStudioWorkspaceFiles() {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
			return err
		}
	}
	return nil
}

func seedCodeStudioContainerWorkspace(ctx context.Context, docker CodeContainerDocker, containerID string) error {
	if strings.TrimSpace(containerID) == "" {
		return fmt.Errorf("code studio container id is required")
	}
	if err := repairCodeStudioContainerWorkspace(ctx, docker, containerID); err != nil {
		return err
	}
	script := buildCodeStudioContainerSeedScript()
	result, err := docker.ExecContainer(ctx, containerID, []string{"sh", "-lc", script}, "", 30*time.Second)
	if err != nil {
		return fmt.Errorf("seed code studio container workspace: %w", err)
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("seed code studio container workspace failed: %s", strings.TrimSpace(result.Output))
	}
	return nil
}

func repairCodeStudioContainerWorkspace(ctx context.Context, docker CodeContainerDocker, containerID string) error {
	script := buildCodeStudioContainerWorkspaceRepairScript()
	result, err := docker.ExecContainer(ctx, containerID, []string{"sh", "-lc", script}, "0:0", 30*time.Second)
	if err != nil {
		return fmt.Errorf("repair code studio container workspace permissions: %w", err)
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("repair code studio container workspace permissions failed: %s", strings.TrimSpace(result.Output))
	}
	return nil
}

func buildCodeStudioContainerWorkspaceRepairScript() string {
	return strings.Join([]string{
		"set -eu",
		"mkdir -p /workspace",
		"if [ ! -w /workspace ]; then",
		"  chmod 0777 /workspace 2>/dev/null || true",
		"fi",
		"if [ ! -w /workspace ]; then",
		"  echo 'workspace is not writable after permission repair'",
		"  exit 1",
		"fi",
	}, "\n")
}

func buildCodeStudioContainerSeedScript() string {
	var b strings.Builder
	b.WriteString("set -eu\n")
	b.WriteString("mkdir -p /workspace\n")
	b.WriteString("if [ -z \"$(find /workspace -mindepth 1 -maxdepth 1 -print -quit 2>/dev/null)\" ]; then\n")
	names := make([]string, 0, len(defaultCodeStudioWorkspaceFiles()))
	for name := range defaultCodeStudioWorkspaceFiles() {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		b.WriteString("cat > /workspace/")
		b.WriteString(name)
		b.WriteString(" <<'AURAGO_CODE_STUDIO_SAMPLE'\n")
		b.WriteString(defaultCodeStudioWorkspaceFiles()[name])
		if !strings.HasSuffix(defaultCodeStudioWorkspaceFiles()[name], "\n") {
			b.WriteString("\n")
		}
		b.WriteString("AURAGO_CODE_STUDIO_SAMPLE\n")
	}
	b.WriteString("fi\n")
	return b.String()
}

func defaultCodeStudioWorkspaceFiles() map[string]string {
	return map[string]string{
		"README.md": "# Code Studio Workspace\n\nThis workspace is mounted at `/workspace` inside the Code Studio container.\n\nTry running:\n\n```sh\ngo run hello.go\npython3 hello.py\n```\n",
		"hello.go":  "package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Println(\"Hello from Code Studio\")\n}\n",
		"hello.py":  "print(\"Hello from Code Studio\")\n",
	}
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
		WorkspaceHostPath:      s.cfg.WorkspaceDir,
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

func (s *CodeContainerService) usesCurrentWorkspaceMountLocked(ctx context.Context, containerID, expectedWorkspaceDir string) (bool, error) {
	inspect, err := s.docker.InspectContainer(ctx, containerID)
	if err != nil {
		return false, fmt.Errorf("inspect code studio container mounts: %w", err)
	}
	if len(inspect.Mounts) == 0 {
		return true, nil
	}
	for _, mount := range inspect.Mounts {
		if path.Clean(strings.TrimSpace(mount.Destination)) != codeWorkspaceInContainer {
			continue
		}
		source := strings.TrimSpace(mount.Source)
		if codeStudioHostPathMatches(source, expectedWorkspaceDir) {
			return true, nil
		}
		if codeStudioHostPathLooksLegacy(source) {
			return false, nil
		}
		return true, nil
	}
	return false, nil
}

func codeStudioHostPathMatches(source, expected string) bool {
	source = filepath.Clean(strings.TrimSpace(source))
	expected = filepath.Clean(strings.TrimSpace(expected))
	if source == "" || expected == "" {
		return false
	}
	if strings.EqualFold(source, expected) {
		return true
	}
	sourceAbs, sourceErr := filepath.Abs(source)
	expectedAbs, expectedErr := filepath.Abs(expected)
	return sourceErr == nil && expectedErr == nil && strings.EqualFold(filepath.Clean(sourceAbs), filepath.Clean(expectedAbs))
}

func codeStudioHostPathLooksLegacy(source string) bool {
	source = filepath.ToSlash(filepath.Clean(strings.TrimSpace(source)))
	return path.Base(source) == codeWorkspaceDirName || strings.HasSuffix(source, "/"+codeWorkspaceDirName)
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

func (missingCodeContainerDocker) ExecContainer(ctx context.Context, container string, cmd []string, user string, timeout time.Duration) (CodeDockerExecResult, error) {
	return CodeDockerExecResult{}, fmt.Errorf("code studio Docker backend is not configured")
}
