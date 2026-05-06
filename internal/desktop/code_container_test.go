package desktop

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

type fakeCodeContainerDocker struct {
	containers    []CodeDockerContainer
	inspectByName map[string]CodeDockerInspect
	ensuredImages []string
	creates       []CodeDockerCreateRequest
	actions       []string
}

func TestCodeContainerEnsureStartedSeedsDefaultWorkspace(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	fake := &fakeCodeContainerDocker{}
	svc := NewCodeContainerService(Config{
		WorkspaceDir: workspace,
		CodeStudio:   CodeStudioConfig{Enabled: true},
	}, nil)
	svc.docker = fake

	if err := svc.EnsureStarted(context.Background()); err != nil {
		t.Fatalf("EnsureStarted returned error: %v", err)
	}
	readme := filepath.Join(workspace, codeWorkspaceDirName, "README.md")
	if _, err := os.Stat(readme); err != nil {
		t.Fatalf("default README was not created: %v", err)
	}
	mainGo := filepath.Join(workspace, codeWorkspaceDirName, "hello.go")
	if _, err := os.Stat(mainGo); err != nil {
		t.Fatalf("default Go example was not created: %v", err)
	}
}

func TestCodeContainerEnsureStartedSeedsWorkspaceWhenAlreadyRunning(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	fake := &fakeCodeContainerDocker{}
	svc := NewCodeContainerService(Config{
		WorkspaceDir: workspace,
		CodeStudio:   CodeStudioConfig{Enabled: true},
	}, nil)
	svc.docker = fake
	svc.state = StateRunning

	if err := svc.EnsureStarted(context.Background()); err != nil {
		t.Fatalf("EnsureStarted returned error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(workspace, codeWorkspaceDirName, "README.md")); err != nil {
		t.Fatalf("default README was not created for already-running service: %v", err)
	}
}

func (f *fakeCodeContainerDocker) ListContainers(ctx context.Context, all bool) ([]CodeDockerContainer, error) {
	return append([]CodeDockerContainer(nil), f.containers...), nil
}

func (f *fakeCodeContainerDocker) InspectContainer(ctx context.Context, container string) (CodeDockerInspect, error) {
	if f.inspectByName == nil {
		return CodeDockerInspect{}, nil
	}
	return f.inspectByName[container], nil
}

func (f *fakeCodeContainerDocker) EnsureImage(ctx context.Context, image string) error {
	f.ensuredImages = append(f.ensuredImages, image)
	return nil
}

func (f *fakeCodeContainerDocker) CreateContainer(ctx context.Context, req CodeDockerCreateRequest) (string, error) {
	f.creates = append(f.creates, req)
	f.containers = append(f.containers, CodeDockerContainer{ID: "created-1", Names: []string{"/" + codeContainerName}})
	if f.inspectByName == nil {
		f.inspectByName = map[string]CodeDockerInspect{}
	}
	f.inspectByName["created-1"] = CodeDockerInspect{ID: "created-1", Name: "/" + codeContainerName, State: CodeDockerState{Running: true}}
	return "created-1", nil
}

func (f *fakeCodeContainerDocker) ContainerAction(ctx context.Context, container, action string) error {
	f.actions = append(f.actions, container+":"+action)
	if f.inspectByName != nil {
		inspect := f.inspectByName[container]
		inspect.State.Running = action == "start"
		f.inspectByName[container] = inspect
	}
	return nil
}

func TestCodeContainerEnsureStartedCreatesContainerWithNoPortsAndLimits(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	fake := &fakeCodeContainerDocker{}
	svc := NewCodeContainerService(Config{
		WorkspaceDir: workspace,
		DockerHost:   "npipe:////./pipe/docker_engine",
		CodeStudio: CodeStudioConfig{
			Enabled:         true,
			Image:           "custom/code-studio:test",
			AutoStopMinutes: 30,
			MaxMemoryMB:     2048,
			MaxCPUCores:     1,
		},
	}, nil)
	svc.docker = fake

	if err := svc.EnsureStarted(context.Background()); err != nil {
		t.Fatalf("EnsureStarted returned error: %v", err)
	}
	if !svc.IsRunning() {
		t.Fatal("service should report running after successful create/start")
	}
	if len(fake.creates) != 1 {
		t.Fatalf("create count = %d, want 1", len(fake.creates))
	}
	if len(fake.ensuredImages) != 1 || fake.ensuredImages[0] != "custom/code-studio:test" {
		t.Fatalf("ensured images = %#v, want custom image ensured before create", fake.ensuredImages)
	}
	req := fake.creates[0]
	if req.Image != "custom/code-studio:test" {
		t.Fatalf("image = %q, want custom image", req.Image)
	}
	if len(req.Ports) != 0 {
		t.Fatalf("ports = %#v, want no host ports", req.Ports)
	}
	if req.User != "developer" {
		t.Fatalf("user = %q, want developer", req.User)
	}
	if len(req.SecurityOpt) != 1 || req.SecurityOpt[0] != "no-new-privileges:true" {
		t.Fatalf("security opts = %#v, want no-new-privileges", req.SecurityOpt)
	}
	if len(req.CapDrop) != 1 || req.CapDrop[0] != "ALL" {
		t.Fatalf("cap drop = %#v, want ALL", req.CapDrop)
	}
	if req.Resources == nil || req.Resources.MemoryMB != 2048 || req.Resources.CPUCores != 1 || req.Resources.PidsLimit != defaultCodeContainerPidsLimit {
		t.Fatalf("resources = %#v, want configured memory/cpu and default pids", req.Resources)
	}
	wantBind := filepath.Join(workspace, codeWorkspaceDirName) + ":" + codeWorkspaceInContainer
	if len(req.Volumes) != 1 || req.Volumes[0] != wantBind {
		t.Fatalf("volumes = %#v, want %q", req.Volumes, wantBind)
	}
}

func TestCodeContainerEnsureStartedStartsExistingStoppedContainer(t *testing.T) {
	t.Parallel()

	fake := &fakeCodeContainerDocker{
		containers: []CodeDockerContainer{{ID: "abc123", Names: []string{"/" + codeContainerName}}},
		inspectByName: map[string]CodeDockerInspect{
			"abc123": {ID: "abc123", Name: "/" + codeContainerName, State: CodeDockerState{Running: false}},
		},
	}
	svc := NewCodeContainerService(Config{
		WorkspaceDir: t.TempDir(),
		DockerHost:   "unix:///var/run/docker.sock",
		CodeStudio:   CodeStudioConfig{Enabled: true, AutoStopMinutes: 30},
	}, nil)
	svc.docker = fake

	if err := svc.EnsureStarted(context.Background()); err != nil {
		t.Fatalf("EnsureStarted returned error: %v", err)
	}
	if len(fake.creates) != 0 {
		t.Fatalf("create count = %d, want existing container start only", len(fake.creates))
	}
	if len(fake.actions) != 1 || fake.actions[0] != "abc123:start" {
		t.Fatalf("actions = %#v, want abc123:start", fake.actions)
	}
}

func TestCodeContainerAutoStopTimerStopsAfterInactivity(t *testing.T) {
	t.Parallel()

	fake := &fakeCodeContainerDocker{}
	svc := NewCodeContainerService(Config{
		WorkspaceDir: t.TempDir(),
		CodeStudio:   CodeStudioConfig{Enabled: true},
	}, nil)
	svc.docker = fake
	svc.autoStopAfter = 20 * time.Millisecond

	if err := svc.EnsureStarted(context.Background()); err != nil {
		t.Fatalf("EnsureStarted returned error: %v", err)
	}

	deadline := time.After(500 * time.Millisecond)
	for {
		if !svc.IsRunning() {
			return
		}
		select {
		case <-deadline:
			t.Fatal("container did not auto-stop before deadline")
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}
}

func TestCodeContainerStatusUsesDefaultsWithoutStartingContainer(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	fake := &fakeCodeContainerDocker{}
	svc := NewCodeContainerService(Config{
		WorkspaceDir: workspace,
		CodeStudio:   CodeStudioConfig{Enabled: true},
	}, nil)
	svc.docker = fake

	status := svc.Status(context.Background())
	if !status.Enabled {
		t.Fatal("status should preserve enabled flag")
	}
	if status.State != "stopped" || status.Running {
		t.Fatalf("status state/running = %q/%v, want stopped/false", status.State, status.Running)
	}
	if status.Image != defaultCodeContainerImage {
		t.Fatalf("image = %q, want default image", status.Image)
	}
	if status.WorkspaceHostPath != filepath.Join(workspace, codeWorkspaceDirName) {
		t.Fatalf("workspace host path = %q", status.WorkspaceHostPath)
	}
	if status.WorkspaceContainerPath != codeWorkspaceInContainer {
		t.Fatalf("workspace container path = %q", status.WorkspaceContainerPath)
	}
	if status.Resources.MemoryMB != defaultCodeContainerMemoryMB ||
		status.Resources.CPUCores != defaultCodeContainerCPUCores ||
		status.Resources.PidsLimit != defaultCodeContainerPidsLimit {
		t.Fatalf("resources = %+v, want safe defaults", status.Resources)
	}
}

func TestCodeContainerStatusReflectsDockerRunningState(t *testing.T) {
	t.Parallel()

	fake := &fakeCodeContainerDocker{
		containers: []CodeDockerContainer{{ID: "running-1", Names: []string{"/" + codeContainerName}}},
		inspectByName: map[string]CodeDockerInspect{
			"running-1": {ID: "running-1", Name: "/" + codeContainerName, State: CodeDockerState{Running: true}},
		},
	}
	svc := NewCodeContainerService(Config{
		WorkspaceDir: t.TempDir(),
		CodeStudio: CodeStudioConfig{
			Enabled:         true,
			Image:           "custom/code-studio:test",
			AutoStopMinutes: 45,
			MaxMemoryMB:     1024,
			MaxCPUCores:     1,
		},
	}, nil)
	svc.docker = fake

	status := svc.Status(context.Background())
	if status.ContainerID != "running-1" {
		t.Fatalf("container id = %q, want running-1", status.ContainerID)
	}
	if !status.Running || status.State != "running" {
		t.Fatalf("status state/running = %q/%v, want running/true", status.State, status.Running)
	}
	if status.Image != "custom/code-studio:test" {
		t.Fatalf("image = %q, want configured image", status.Image)
	}
	if status.AutoStopMinutes != 45 {
		t.Fatalf("auto stop = %d, want 45", status.AutoStopMinutes)
	}
	if status.Resources.MemoryMB != 1024 || status.Resources.CPUCores != 1 {
		t.Fatalf("resources = %+v, want configured resource limits", status.Resources)
	}
}

func TestCodeContainerDisabledDoesNotTouchDocker(t *testing.T) {
	t.Parallel()

	fake := &fakeCodeContainerDocker{}
	svc := NewCodeContainerService(Config{
		WorkspaceDir: t.TempDir(),
		CodeStudio:   CodeStudioConfig{Enabled: false},
	}, nil)
	svc.docker = fake

	if err := svc.EnsureStarted(context.Background()); err == nil {
		t.Fatal("expected disabled error")
	}
	if len(fake.ensuredImages) != 0 || len(fake.creates) != 0 || len(fake.actions) != 0 {
		t.Fatalf("docker was touched despite disabled code studio: ensured=%v creates=%v actions=%v", fake.ensuredImages, fake.creates, fake.actions)
	}
}
