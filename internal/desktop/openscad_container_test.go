package desktop

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestOpenSCADEnsureInstalledCreatesNoNetworkContainerWithLimits(t *testing.T) {
	t.Parallel()

	dataDir := t.TempDir()
	fake := &fakeCodeContainerDocker{}
	svc := NewOpenSCADContainerService(Config{
		DataDir: dataDir,
		OpenSCAD: OpenSCADConfig{
			Enabled:         true,
			Image:           "openscad/openscad:latest",
			MaxMemoryMB:     1024,
			MaxCPUCores:     1,
			AutoStopMinutes: 20,
		},
	}, nil)
	svc.SetDockerClient(fake)

	if err := svc.EnsureInstalled(context.Background()); err != nil {
		t.Fatalf("EnsureInstalled: %v", err)
	}
	if len(fake.ensuredImages) != 1 || fake.ensuredImages[0] != "openscad/openscad:latest" {
		t.Fatalf("ensured images = %#v", fake.ensuredImages)
	}
	if len(fake.creates) != 1 {
		t.Fatalf("creates = %d, want 1", len(fake.creates))
	}
	req := fake.creates[0]
	if req.Name != openSCADContainerName || req.NetworkMode != "none" || len(req.Ports) != 0 {
		t.Fatalf("container isolation = name %q network %q ports %#v", req.Name, req.NetworkMode, req.Ports)
	}
	if len(req.SecurityOpt) != 1 || req.SecurityOpt[0] != "no-new-privileges:true" {
		t.Fatalf("security options = %#v", req.SecurityOpt)
	}
	if len(req.CapDrop) != 1 || req.CapDrop[0] != "ALL" {
		t.Fatalf("cap drop = %#v", req.CapDrop)
	}
	if req.Resources == nil || req.Resources.MemoryMB != 1024 || req.Resources.CPUCores != 1 || req.Resources.PidsLimit != defaultOpenSCADPidsLimit {
		t.Fatalf("resources = %#v", req.Resources)
	}
	wantMount := filepath.Join(dataDir, "openscad", "jobs") + ":" + openSCADJobsInContainer
	if len(req.Volumes) != 1 || req.Volumes[0] != wantMount {
		t.Fatalf("volumes = %#v, want %q", req.Volumes, wantMount)
	}
	if !containsString(fake.actions, "created-1:stop") {
		t.Fatalf("install validation should stop container when auto_start is false; actions=%#v", fake.actions)
	}
}

func TestBuildOpenSCADCommandUsesSeparateArgs(t *testing.T) {
	req := OpenSCADRenderRequest{
		RenderMode: "render",
		Defines: []OpenSCADDefine{
			{Name: "height", Value: "42"},
			{Name: "label", Value: `"A B"`},
		},
	}
	cmd, filename := buildOpenSCADCommand("oscad-123", "test-model", "png", req)
	want := []string{
		"xvfb-run", "-a", "openscad", "--render",
		"-D", "height=42",
		"-D", `label="A B"`,
		"-o", "/jobs/oscad-123/test-model.png",
		"/jobs/oscad-123/model.scad",
	}
	if filename != "test-model.png" {
		t.Fatalf("filename = %q", filename)
	}
	if len(cmd) != len(want) {
		t.Fatalf("cmd length = %d, want %d: %#v", len(cmd), len(want), cmd)
	}
	for i := range want {
		if cmd[i] != want[i] {
			t.Fatalf("cmd[%d] = %q, want %q in %#v", i, cmd[i], want[i], cmd)
		}
	}
}

func TestOpenSCADOutputFileExposesSeparatePreviewAndDownloadURLs(t *testing.T) {
	t.Parallel()

	jobDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(jobDir, "model.png"), []byte("png-data"), 0o600); err != nil {
		t.Fatalf("write output: %v", err)
	}
	svc := NewOpenSCADContainerService(Config{
		DataDir: t.TempDir(),
		OpenSCAD: OpenSCADConfig{
			Enabled: true,
		},
	}, nil)

	file, err := svc.outputFile(jobDir, "oscad-urltest", "model.png", "png")
	if err != nil {
		t.Fatalf("outputFile: %v", err)
	}
	if file.PreviewURL != "/api/openscad/jobs/oscad-urltest/files/model.png" {
		t.Fatalf("PreviewURL = %q", file.PreviewURL)
	}
	if file.DownloadURL != "/api/openscad/jobs/oscad-urltest/files/model.png?download=1" {
		t.Fatalf("DownloadURL = %q", file.DownloadURL)
	}
}

func TestOpenSCADJobsRootRepairsContainerReadablePermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix permission bits are not meaningful on Windows")
	}

	dataDir := t.TempDir()
	root := filepath.Join(dataDir, "openscad", "jobs")
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.Chmod(root, 0o700); err != nil {
		t.Fatalf("Chmod setup: %v", err)
	}
	svc := NewOpenSCADContainerService(Config{
		DataDir:  dataDir,
		OpenSCAD: OpenSCADConfig{Enabled: true},
	}, nil)

	got, err := svc.ensureJobsRoot()
	if err != nil {
		t.Fatalf("ensureJobsRoot: %v", err)
	}
	if got != root {
		t.Fatalf("jobs root = %q, want %q", got, root)
	}
	info, err := os.Stat(root)
	if err != nil {
		t.Fatalf("Stat root: %v", err)
	}
	if info.Mode().Perm() != openSCADJobsRootMode {
		t.Fatalf("jobs root mode = %v, want %v", info.Mode().Perm(), openSCADJobsRootMode)
	}
}

func TestOpenSCADRenderPreparesContainerWritableJobFiles(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix permission bits are not meaningful on Windows")
	}

	dataDir := t.TempDir()
	fake := &fakeCodeContainerDocker{}
	svc := NewOpenSCADContainerService(Config{
		DataDir: dataDir,
		OpenSCAD: OpenSCADConfig{
			Enabled: true,
		},
	}, nil)
	svc.SetDockerClient(fake)

	_, err := svc.Render(context.Background(), OpenSCADRenderRequest{
		SourceSCAD: "cube(1);",
		ModelName:  "perm-test",
		Exports:    []string{"png"},
	})
	if err == nil || !strings.Contains(err.Error(), "openscad output perm-test.png missing") {
		t.Fatalf("Render error = %v, want missing fake output after job preparation", err)
	}

	root := filepath.Join(dataDir, "openscad", "jobs")
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("ReadDir jobs root: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("job entries = %d, want 1", len(entries))
	}
	jobDir := filepath.Join(root, entries[0].Name())
	jobInfo, err := os.Stat(jobDir)
	if err != nil {
		t.Fatalf("Stat job dir: %v", err)
	}
	if jobInfo.Mode().Perm() != openSCADJobDirMode.Perm() || jobInfo.Mode()&os.ModeSticky == 0 {
		t.Fatalf("job dir mode = %v, want sticky %v", jobInfo.Mode(), openSCADJobDirMode)
	}
	sourceInfo, err := os.Stat(filepath.Join(jobDir, "model.scad"))
	if err != nil {
		t.Fatalf("Stat source: %v", err)
	}
	if sourceInfo.Mode().Perm() != openSCADSourceFileMode {
		t.Fatalf("source mode = %v, want %v", sourceInfo.Mode().Perm(), openSCADSourceFileMode)
	}
}

func TestOpenSCADRenderSkips2DOnlyExportsWhen3DOutputsSucceed(t *testing.T) {
	t.Parallel()

	dataDir := t.TempDir()
	fake := &fakeOpenSCADExportDocker{dataDir: dataDir}
	svc := NewOpenSCADContainerService(Config{
		DataDir:  dataDir,
		OpenSCAD: OpenSCADConfig{Enabled: true},
	}, nil)
	svc.SetDockerClient(fake)

	result, err := svc.Render(context.Background(), OpenSCADRenderRequest{
		SourceSCAD: "cube(10);",
		ModelName:  "mixed-exports",
		Exports:    []string{"png", "stl", "svg", "pdf"},
	})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if result.ExitCode != 0 {
		t.Fatalf("ExitCode = %d, want 0 for partial success with skipped 2D exports", result.ExitCode)
	}
	if got := openSCADFileNames(result.Files); strings.Join(got, ",") != "mixed-exports.png,mixed-exports.stl" {
		t.Fatalf("files = %#v, want png and stl only", got)
	}
	for _, want := range []string{"Skipped svg export", "Skipped pdf export", "requires a 2D top-level object"} {
		if !strings.Contains(result.Stderr, want) {
			t.Fatalf("stderr %q missing %q", result.Stderr, want)
		}
	}
}

func TestOpenSCADRenderReturnsActionableErrorWhenOnly2DExportsFailFor3D(t *testing.T) {
	t.Parallel()

	dataDir := t.TempDir()
	fake := &fakeOpenSCADExportDocker{dataDir: dataDir}
	svc := NewOpenSCADContainerService(Config{
		DataDir:  dataDir,
		OpenSCAD: OpenSCADConfig{Enabled: true},
	}, nil)
	svc.SetDockerClient(fake)

	result, err := svc.Render(context.Background(), OpenSCADRenderRequest{
		SourceSCAD: "cube(10);",
		ModelName:  "only-2d",
		Exports:    []string{"svg", "pdf"},
	})
	if err == nil {
		t.Fatal("Render succeeded with only incompatible 2D exports")
	}
	if len(result.Files) != 0 {
		t.Fatalf("files = %#v, want none", result.Files)
	}
	for _, want := range []string{"Skipped svg export", "Skipped pdf export", "current model is 3D"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error %q missing %q", err.Error(), want)
		}
	}
}

type fakeOpenSCADExportDocker struct {
	fakeCodeContainerDocker
	dataDir string
}

func (f *fakeOpenSCADExportDocker) ExecContainer(ctx context.Context, container string, cmd []string, user string, timeout time.Duration) (CodeDockerExecResult, error) {
	f.execs = append(f.execs, fakeCodeContainerExec{container: container, cmd: append([]string(nil), cmd...), user: user})
	joined := strings.Join(cmd, " ")
	if strings.Contains(joined, "command -v openscad") {
		return CodeDockerExecResult{ExitCode: 0}, nil
	}
	outputPath := openSCADOutputArg(cmd)
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(outputPath), "."))
	switch ext {
	case "svg", "pdf", "dxf":
		return CodeDockerExecResult{ExitCode: 1, Output: "Top level object is a 3D object.\nCurrent top level object is not a 2D object.\n"}, nil
	case "png", "stl", "3mf", "off", "amf", "csg", "txt":
		if err := f.writeOutputFile(outputPath, []byte(ext+" data")); err != nil {
			return CodeDockerExecResult{}, err
		}
		return CodeDockerExecResult{ExitCode: 0, Output: "rendered " + ext + "\n"}, nil
	default:
		return CodeDockerExecResult{ExitCode: 0}, nil
	}
}

func (f *fakeOpenSCADExportDocker) writeOutputFile(containerPath string, data []byte) error {
	rel := strings.TrimPrefix(strings.TrimPrefix(containerPath, openSCADJobsInContainer), "/")
	hostPath := filepath.Join(f.dataDir, "openscad", "jobs", filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(hostPath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(hostPath, data, 0o644)
}

func openSCADOutputArg(cmd []string) string {
	for i, item := range cmd {
		if item == "-o" && i+1 < len(cmd) {
			return cmd[i+1]
		}
	}
	return ""
}

func openSCADFileNames(files []OpenSCADFile) []string {
	names := make([]string, 0, len(files))
	for _, file := range files {
		names = append(names, file.Name)
	}
	return names
}

func TestOpenSCADPruneOldJobsRemovesExpiredDirectories(t *testing.T) {
	t.Parallel()
	dataDir := t.TempDir()
	jobsRoot := filepath.Join(dataDir, "openscad", "jobs")
	if err := os.MkdirAll(jobsRoot, openSCADJobsRootMode); err != nil {
		t.Fatalf("mkdir jobs: %v", err)
	}
	oldJob := filepath.Join(jobsRoot, "oscad-old")
	if err := os.MkdirAll(oldJob, openSCADJobDirMode); err != nil {
		t.Fatalf("mkdir old job: %v", err)
	}
	oldMeta := OpenSCADRenderResult{
		JobID:     "oscad-old",
		CreatedAt: time.Now().UTC().Add(-48 * time.Hour),
	}
	data, err := json.Marshal(oldMeta)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(filepath.Join(oldJob, "job.json"), data, 0o600); err != nil {
		t.Fatalf("write job.json: %v", err)
	}
	keepJob := filepath.Join(jobsRoot, "oscad-keep")
	if err := os.MkdirAll(keepJob, openSCADJobDirMode); err != nil {
		t.Fatalf("mkdir keep job: %v", err)
	}
	svc := NewOpenSCADContainerService(Config{
		DataDir: dataDir,
		OpenSCAD: OpenSCADConfig{JobRetentionDays: 1},
	}, nil)
	svc.pruneOldOpenSCADJobs(jobsRoot, "oscad-keep")
	if _, err := os.Stat(oldJob); !os.IsNotExist(err) {
		t.Fatalf("old job dir should be removed, stat err=%v", err)
	}
	if _, err := os.Stat(keepJob); err != nil {
		t.Fatalf("keep job dir should remain: %v", err)
	}
}
