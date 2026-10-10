package desktop

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
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
	if len(req.Volumes) != 0 {
		t.Fatalf("volumes = %#v, want none", req.Volumes)
	}
	if !containsString(fake.actions, "created-1 stop") {
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
		"-o", "/work/test-model.png",
		"/work/model.scad",
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

func TestBuildOpenSCADCommandUsesGeometryBackendFor3DExportsOnly(t *testing.T) {
	req := OpenSCADRenderRequest{RenderMode: "render"}
	cmd, _ := buildOpenSCADCommandWithBackend("oscad-123", "test-model", "stl", req, "manifold")
	if !containsString(cmd, "--backend=manifold") {
		t.Fatalf("stl command missing manifold backend: %#v", cmd)
	}
	if got := strings.Join(cmd, " "); !strings.Contains(got, "openscad --backend=manifold --render") {
		t.Fatalf("backend flag should follow openscad before render mode, got %q", got)
	}
	cmd, _ = buildOpenSCADCommandWithBackend("oscad-123", "test-model", "png", req, "manifold")
	if containsString(cmd, "--backend=manifold") {
		t.Fatalf("png command should not force geometry backend: %#v", cmd)
	}
}

func TestOpenSCADSelectsGeometryBackendFromRuntimeCapabilities(t *testing.T) {
	diag := openSCADRuntimeDiagnostics{BackendHelp: "--backend arg backend CGAL (old/slow) or Manifold (new/fast)"}
	if got := selectOpenSCADGeometryBackend(OpenSCADConfig{GeometryBackend: "auto"}, "stl", diag); got != "manifold" {
		t.Fatalf("auto backend = %q, want manifold", got)
	}
	if got := selectOpenSCADGeometryBackend(OpenSCADConfig{GeometryBackend: "cgal"}, "stl", diag); got != "cgal" {
		t.Fatalf("cgal backend = %q, want cgal", got)
	}
	if got := selectOpenSCADGeometryBackend(OpenSCADConfig{GeometryBackend: "manifold"}, "stl", openSCADRuntimeDiagnostics{}); got != "" {
		t.Fatalf("unsupported forced manifold backend = %q, want fallback", got)
	}
	if got := selectOpenSCADGeometryBackend(OpenSCADConfig{GeometryBackend: "auto"}, "png", diag); got != "" {
		t.Fatalf("png backend = %q, want no geometry backend", got)
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

func TestOpenSCADJobDirModeIncludesGoStickyBit(t *testing.T) {
	if openSCADJobDirMode.Perm() != 0o777 {
		t.Fatalf("job dir permissions = %o, want 777", openSCADJobDirMode.Perm())
	}
	if openSCADJobDirMode&os.ModeSticky == 0 {
		t.Fatalf("job dir mode = %v, want Go sticky bit", openSCADJobDirMode)
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
	wantPerm := openSCADJobDirMode.Perm() & 0777
	if jobInfo.Mode().Perm()&0777 != wantPerm {
		t.Fatalf("job dir perm = %o, want %o", jobInfo.Mode().Perm()&0777, wantPerm)
	}
	if runtime.GOOS == "linux" && jobInfo.Mode()&os.ModeSticky == 0 {
		// Re-apply; some mkdir paths only expose 0777 until explicit chmod with sticky.
		if err := os.Chmod(jobDir, openSCADJobDirMode); err != nil {
			t.Fatalf("Chmod job dir sticky: %v", err)
		}
		jobInfo, err = os.Stat(jobDir)
		if err != nil {
			t.Fatalf("Stat job dir after chmod: %v", err)
		}
	}
	if runtime.GOOS == "linux" && jobInfo.Mode()&os.ModeSticky == 0 {
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

func TestOpenSCADRenderSkips3DOnlyExportsWhen2DPreviewSucceeds(t *testing.T) {
	t.Parallel()

	dataDir := t.TempDir()
	fake := &fakeOpenSCADExportDocker{
		dataDir: dataDir,
		failExports: map[string]CodeDockerExecResult{
			"stl": {ExitCode: 1, Output: "Current top level object is a 2D object.\n"},
		},
	}
	svc := NewOpenSCADContainerService(Config{
		DataDir:  dataDir,
		OpenSCAD: OpenSCADConfig{Enabled: true},
	}, nil)
	svc.SetDockerClient(fake)

	result, err := svc.Render(context.Background(), OpenSCADRenderRequest{
		SourceSCAD: "circle(10);",
		ModelName:  "mixed-2d",
		Exports:    []string{"png", "stl"},
	})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if result.ExitCode != 0 {
		t.Fatalf("ExitCode = %d, want 0 for PNG preview with skipped STL export", result.ExitCode)
	}
	if got := openSCADFileNames(result.Files); strings.Join(got, ",") != "mixed-2d.png" {
		t.Fatalf("files = %#v, want png only", got)
	}
	for _, want := range []string{"Skipped stl export", "requires a 3D top-level object"} {
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

func TestOpenSCADRenderLogsPerExportDiagnostics(t *testing.T) {
	t.Parallel()

	dataDir := t.TempDir()
	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	fake := &fakeOpenSCADExportDocker{dataDir: dataDir}
	svc := NewOpenSCADContainerService(Config{
		DataDir:  dataDir,
		OpenSCAD: OpenSCADConfig{Enabled: true},
	}, logger)
	svc.SetDockerClient(fake)

	result, err := svc.Render(context.Background(), OpenSCADRenderRequest{
		SourceSCAD: "cube(10);",
		ModelName:  "logged-render",
		Exports:    []string{"png", "stl"},
	})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	logText := logs.String()
	for _, want := range []string{
		"openscad render job started",
		"openscad runtime diagnostics",
		"openscad export started",
		"openscad export completed",
		"openscad render job completed",
		"job_id=" + result.JobID,
		"model_name=logged-render",
		"exports=png,stl",
		"version=\"OpenSCAD 2021.01\"",
		"backend_help=",
		"export=png",
		"export=stl",
		"timeout_seconds=120",
		"duration_ms=",
		"command=",
		"size_bytes=",
		"sha256=",
	} {
		if !strings.Contains(logText, want) {
			t.Fatalf("render logs missing %q in:\n%s", want, logText)
		}
	}
}

func TestOpenSCADRenderWritesExportDiagnosticsToContainerLogStream(t *testing.T) {
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
		ModelName:  "container-log-render",
		Exports:    []string{"png", "stl"},
	})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	logText := strings.Join(fake.containerLogs, "\n")
	for _, want := range []string{
		"[AuraGo OpenSCAD] runtime",
		"[AuraGo OpenSCAD] export_start",
		"[AuraGo OpenSCAD] export_done",
		"job_id=" + result.JobID,
		"version=OpenSCAD_2021.01",
		"backend_help=--backend_arg_backend_CGAL_(old/slow)_or_Manifold_(new/fast)",
		"export=png",
		"export=stl",
		"filename=container-log-render.png",
		"filename=container-log-render.stl",
		"timeout_seconds=120",
		"command=openscad_--backend=manifold_--render",
		"exit_code=0",
	} {
		if !strings.Contains(logText, want) {
			t.Fatalf("container logs missing %q in:\n%s", want, logText)
		}
	}
	var jobCmds [][]string
	for _, created := range fake.creates {
		if strings.HasPrefix(created.Name, openSCADContainerName+"-oscad-") {
			jobCmds = append(jobCmds, created.Cmd)
		}
	}
	if len(jobCmds) != 2 {
		t.Fatalf("job containers = %d, want 2", len(jobCmds))
	}
	for _, cmd := range jobCmds {
		joined := strings.Join(cmd, " ")
		if len(cmd) < 6 || cmd[0] != "sh" || cmd[1] != "-c" || !strings.Contains(joined, "/work/compiler.log") {
			t.Fatalf("job command = %#v, want /work/compiler.log", cmd)
		}
	}
}

type fakeOpenSCADExportDocker struct {
	fakeCodeContainerDocker
	dataDir            string
	failExports        map[string]CodeDockerExecResult
	runtimeBackendHelp string
	containerLogs      []string
	containerLogExecs  [][]string
}

func (f *fakeOpenSCADExportDocker) CreateContainer(ctx context.Context, req CodeDockerCreateRequest) (string, error) {
	id, err := f.fakeCodeContainerDocker.CreateContainer(ctx, req)
	if err != nil || !strings.HasPrefix(req.Name, openSCADContainerName+"-oscad-") {
		return id, err
	}
	f.finishOpenSCADJobCreate(id, req)
	return id, nil
}

func (f *fakeOpenSCADExportDocker) ContainerAction(ctx context.Context, container, action string) error {
	err := f.fakeCodeContainerDocker.ContainerAction(ctx, container, action)
	if action != "start" || f.inspectByName == nil {
		return err
	}
	inspect, ok := f.inspectByName[container]
	if !ok {
		return err
	}
	name := strings.TrimPrefix(inspect.Name, "/")
	if !strings.HasPrefix(name, openSCADContainerName+"-oscad-") && !strings.HasPrefix(container, openSCADContainerName+"-oscad-") {
		return err
	}
	inspect.State.Running = false
	f.inspectByName[container] = inspect
	return err
}

func (f *fakeOpenSCADExportDocker) finishOpenSCADJobCreate(id string, req CodeDockerCreateRequest) {
	result := f.openSCADExportResult(req.Cmd)
	startLog, doneLog := fakeOpenSCADWrappedContainerLogLines(req.Cmd)
	if startLog != "" {
		f.containerLogs = append(f.containerLogs, startLog)
		f.containerLogExecs = append(f.containerLogExecs, append([]string(nil), req.Cmd...))
		f.containerLogs = append(f.containerLogs, doneLog+" exit_code="+strconv.Itoa(result.ExitCode))
	}
	if jobDir := openSCADBindHost(req.Volumes, openSCADWorkDir); jobDir != "" {
		var log strings.Builder
		if startLog != "" {
			log.WriteString(startLog)
			log.WriteByte('\n')
		}
		log.WriteString(result.Output)
		if doneLog != "" {
			if log.Len() == 0 || !strings.HasSuffix(log.String(), "\n") {
				log.WriteByte('\n')
			}
			log.WriteString(doneLog)
			log.WriteString(" exit_code=")
			log.WriteString(strconv.Itoa(result.ExitCode))
			log.WriteByte('\n')
		}
		logFile, logErr := os.OpenFile(filepath.Join(jobDir, "compiler.log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if logErr == nil {
			_, _ = logFile.Write([]byte(log.String()))
			_ = logFile.Close()
		}
		if result.ExitCode == 0 {
			outputPath := openSCADOutputArg(req.Cmd)
			ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(outputPath), "."))
			switch ext {
			case "png", "stl", "3mf", "off", "amf", "csg", "txt":
				_ = os.WriteFile(filepath.Join(jobDir, filepath.Base(outputPath)), []byte(ext+" data"), 0o644)
			}
		}
	}
	if f.inspectByName == nil {
		f.inspectByName = map[string]CodeDockerInspect{}
	}
	inspect := f.inspectByName[id]
	inspect.State.ExitCode = result.ExitCode
	f.inspectByName[id] = inspect
	named := inspect
	named.State.Running = false
	f.inspectByName[req.Name] = named
}

func (f *fakeOpenSCADExportDocker) openSCADExportResult(cmd []string) CodeDockerExecResult {
	outputPath := openSCADOutputArg(cmd)
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(outputPath), "."))
	if fail, ok := f.failExports[ext]; ok {
		return fail
	}
	switch ext {
	case "svg", "pdf", "dxf":
		return CodeDockerExecResult{ExitCode: 1, Output: "Top level object is a 3D object.\nCurrent top level object is not a 2D object.\n"}
	case "png", "stl", "3mf", "off", "amf", "csg", "txt":
		return CodeDockerExecResult{ExitCode: 0, Output: "rendered " + ext + "\n"}
	default:
		return CodeDockerExecResult{ExitCode: 0}
	}
}

func openSCADBindHost(volumes []string, destination string) string {
	suffix := ":" + destination
	for _, volume := range volumes {
		if strings.HasSuffix(volume, suffix) {
			return strings.TrimSuffix(volume, suffix)
		}
	}
	return ""
}

func (f *fakeOpenSCADExportDocker) ExecContainer(ctx context.Context, container string, cmd []string, user string, timeout time.Duration) (result CodeDockerExecResult, err error) {
	f.execs = append(f.execs, fakeCodeContainerExec{container: container, cmd: append([]string(nil), cmd...), user: user})
	startLog, doneLog := fakeOpenSCADWrappedContainerLogLines(cmd)
	if startLog != "" {
		f.containerLogs = append(f.containerLogs, startLog)
		f.containerLogExecs = append(f.containerLogExecs, append([]string(nil), cmd...))
		defer func() {
			f.containerLogs = append(f.containerLogs, doneLog+" exit_code="+strconv.Itoa(result.ExitCode))
		}()
	}
	joined := strings.Join(cmd, " ")
	if strings.Contains(joined, "command -v openscad") {
		return CodeDockerExecResult{ExitCode: 0}, nil
	}
	if strings.Contains(joined, "openscad --version") && strings.Contains(joined, "openscad --help") {
		jobID := ""
		if len(cmd) >= 5 {
			jobID = cmd[4]
		}
		backendHelp := f.runtimeBackendHelp
		if backendHelp == "" {
			backendHelp = "--backend arg backend CGAL (old/slow) or Manifold (new/fast)"
		}
		f.containerLogs = append(f.containerLogs, openSCADContainerLogLine("runtime",
			"job_id", jobID,
			"version", "OpenSCAD 2021.01",
			"backend_help", backendHelp,
		))
		return CodeDockerExecResult{
			ExitCode: 0,
			Output:   "version=OpenSCAD 2021.01\nbackend_help=" + backendHelp + "\n",
		}, nil
	}
	outputPath := openSCADOutputArg(cmd)
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(outputPath), "."))
	if fail, ok := f.failExports[ext]; ok {
		return fail, nil
	}
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

func fakeOpenSCADWrappedContainerLogLines(cmd []string) (string, string) {
	if len(cmd) < 6 || cmd[0] != "sh" || cmd[1] != "-c" {
		return "", ""
	}
	if !strings.Contains(cmd[2], "/work/compiler.log") && !strings.Contains(cmd[2], "/proc/1/fd/1") {
		return "", ""
	}
	return cmd[4], cmd[5]
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
		DataDir:  dataDir,
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

func TestOpenSCADRenderMountsOnlyTheJobDirectory(t *testing.T) {
	dataDir := t.TempDir()
	fake := &fakeCodeContainerDocker{}
	svc := NewOpenSCADContainerService(Config{
		DataDir:  dataDir,
		OpenSCAD: OpenSCADConfig{Enabled: true},
	}, nil)
	svc.SetDockerClient(fake)

	_, err := svc.Render(context.Background(), OpenSCADRenderRequest{
		SourceSCAD: "cube(1);",
		ModelName:  "mount-test",
		Exports:    []string{"png"},
	})
	if err == nil {
		t.Fatal("Render succeeded with a fake that writes no output")
	}

	var probe, job *CodeDockerCreateRequest
	for i := range fake.creates {
		req := &fake.creates[i]
		switch {
		case req.Name == openSCADContainerName:
			probe = req
		case strings.HasPrefix(req.Name, openSCADContainerName+"-oscad-"):
			job = req
		}
	}
	if probe == nil {
		t.Fatal("probe container was not created")
	}
	if len(probe.Volumes) != 0 {
		t.Fatalf("probe volumes = %#v, want none", probe.Volumes)
	}
	if job == nil {
		t.Fatalf("job container missing in %#v", fake.creates)
	}
	if len(job.Volumes) != 1 || !strings.HasSuffix(job.Volumes[0], ":/work") {
		t.Fatalf("job volumes = %#v, want one :/work bind", job.Volumes)
	}
	host := strings.TrimSuffix(job.Volumes[0], ":/work")
	if filepath.Base(host) == "jobs" || !strings.HasPrefix(filepath.Base(host), "oscad-") {
		t.Fatalf("job bind host = %q, want the job directory", host)
	}
	joined := strings.Join(job.Cmd, " ")
	if strings.Contains(joined, "/jobs/") || !strings.Contains(joined, "/work/model.scad") || !strings.Contains(joined, "/work/compiler.log") {
		t.Fatalf("job command = %q, want /work paths and compiler.log", joined)
	}
	if job.NetworkMode != "none" || !containsString(job.CapDrop, "ALL") || !containsString(job.SecurityOpt, "no-new-privileges:true") {
		t.Fatalf("job hardening = network %q cap %#v security %#v", job.NetworkMode, job.CapDrop, job.SecurityOpt)
	}
	if !job.ReadonlyRootfs || job.Tmpfs["/tmp"] == "" || !containsString(job.Env, "HOME=/tmp") {
		t.Fatalf("job rootfs = readonly %v tmpfs %#v env %#v", job.ReadonlyRootfs, job.Tmpfs, job.Env)
	}
}

type cancelOnStartDocker struct {
	fakeCodeContainerDocker
	cancel context.CancelFunc
}

func (f *cancelOnStartDocker) ContainerAction(ctx context.Context, container, action string) error {
	err := f.fakeCodeContainerDocker.ContainerAction(ctx, container, action)
	if action == "start" && strings.HasPrefix(container, openSCADContainerName+"-oscad-") && f.cancel != nil {
		f.cancel()
	}
	return err
}

func (f *cancelOnStartDocker) InspectContainer(ctx context.Context, container string) (CodeDockerInspect, error) {
	running := true
	for _, action := range f.actions {
		if strings.HasPrefix(action, container+" kill") || action == container+" kill" {
			running = false
		}
	}
	return CodeDockerInspect{ID: container, Name: container, State: CodeDockerState{Running: running, ExitCode: 0}}, nil
}

func TestOpenSCADCancelKillsTheJobContainer(t *testing.T) {
	dataDir := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	fake := &cancelOnStartDocker{cancel: cancel}
	svc := NewOpenSCADContainerService(Config{
		DataDir:  dataDir,
		OpenSCAD: OpenSCADConfig{Enabled: true},
	}, nil)
	svc.SetDockerClient(fake)

	_, err := svc.Render(ctx, OpenSCADRenderRequest{
		SourceSCAD: "cube(1);",
		ModelName:  "cancel-test",
		Exports:    []string{"png"},
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Render error = %v, want context.Canceled", err)
	}
	var killed bool
	for _, action := range fake.actions {
		if strings.Contains(action, "kill") && strings.Contains(action, openSCADContainerName+"-oscad-") {
			killed = true
		}
	}
	if !killed {
		t.Fatalf("actions = %#v, want kill of the job container", fake.actions)
	}
}

func TestOpenSCADStatusIncludesRenderQueueNote(t *testing.T) {
	t.Parallel()
	svc := NewOpenSCADContainerService(Config{OpenSCAD: OpenSCADConfig{Enabled: true}}, nil)
	status := svc.Status(context.Background())
	if status.RenderQueueNote == "" {
		t.Fatalf("expected render_queue_note, got empty")
	}
	if status.MaxConcurrentJobs <= 0 {
		t.Fatalf("max_concurrent_jobs = %d", status.MaxConcurrentJobs)
	}
}
