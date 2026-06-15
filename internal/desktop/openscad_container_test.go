package desktop

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
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
