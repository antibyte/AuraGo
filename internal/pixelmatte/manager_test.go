package pixelmatte

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"aurago/internal/onnxrt"
)

func TestPinnedModelSpecsAreValid(t *testing.T) {
	seen := map[string]bool{}
	for _, spec := range Models() {
		if err := validateSpec(spec); err != nil {
			t.Errorf("%v", err)
		}
		if !ValidKind(string(spec.Kind)) || seen[spec.Asset.ID] || spec.Name == "" || spec.License == "" {
			t.Errorf("model %s: bad or duplicate metadata", spec.Kind)
		}
		seen[spec.Asset.ID] = true
		if !strings.HasPrefix(spec.Asset.URL, "https://github.com/danielgatis/rembg/releases/download/v0.0.0/") {
			t.Errorf("model %s: unexpected URL %s", spec.Kind, spec.Asset.URL)
		}
	}
	if len(seen) != 2 {
		t.Fatalf("expected 2 models, got %d", len(seen))
	}
	if _, ok := Model("nope"); ok || ValidKind("nope") || ValidKind("") {
		t.Fatal("unknown kinds must not validate")
	}
	if _, ok := onnxrt.CPURuntimeAsset("windows", "amd64"); !ok {
		t.Fatal("the CPU runtime asset must exist for windows/amd64")
	}
}

func TestDefaultThreadsIsBounded(t *testing.T) {
	if n := DefaultThreads(); n < 1 || n > 8 {
		t.Fatalf("DefaultThreads() = %d", n)
	}
}

// fakeInstaller records what the manager asks for. EnsureModel/EnsureRuntime
// block on gate (when set) so a test can observe a running download.
type fakeInstaller struct {
	mu           sync.Mutex
	runtimeReady bool
	modelReady   map[string]bool
	runtimeCalls int
	modelCalls   int
	runtimeErr   error
	modelErr     error
	gate         chan struct{}
	entered      chan struct{}
}

func newFakeInstaller() *fakeInstaller {
	return &fakeInstaller{modelReady: map[string]bool{}, entered: make(chan struct{}, 8)}
}

func (f *fakeInstaller) ModelReady(spec onnxrt.AssetSpec, _ string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.modelReady[spec.ID]
}

func (f *fakeInstaller) RuntimeReady(spec onnxrt.AssetSpec) (string, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return filepath.Join("runtimes", spec.ID), f.runtimeReady
}

func (f *fakeInstaller) wait(ctx context.Context) error {
	if f.gate == nil {
		return nil
	}
	f.entered <- struct{}{}
	select {
	case <-f.gate:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (f *fakeInstaller) EnsureRuntime(ctx context.Context, spec onnxrt.AssetSpec) (string, error) {
	f.mu.Lock()
	f.runtimeCalls++
	f.mu.Unlock()
	if err := f.wait(ctx); err != nil {
		return "", err
	}
	if f.runtimeErr != nil {
		return "", f.runtimeErr
	}
	f.mu.Lock()
	f.runtimeReady = true
	f.mu.Unlock()
	return filepath.Join("runtimes", spec.ID), nil
}

func (f *fakeInstaller) EnsureModel(ctx context.Context, spec onnxrt.AssetSpec, _ string) error {
	f.mu.Lock()
	f.modelCalls++
	f.mu.Unlock()
	if err := f.wait(ctx); err != nil {
		return err
	}
	if f.modelErr != nil {
		return f.modelErr
	}
	f.mu.Lock()
	f.modelReady[spec.ID] = true
	f.mu.Unlock()
	return nil
}

type fakeRunner struct {
	mu      sync.Mutex
	request RunRequest
	started chan struct{}
	release chan struct{}
	result  []byte
	err     error
}

func (f *fakeRunner) Run(ctx context.Context, request RunRequest) ([]byte, error) {
	f.mu.Lock()
	f.request = request
	f.mu.Unlock()
	if f.started != nil {
		f.started <- struct{}{}
		select {
		case <-f.release:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return f.result, f.err
}

func newTestManager(installer *fakeInstaller, runner Runner) *Manager {
	return NewManager(Options{
		DataDir:    filepath.Join("data"),
		Executable: "aurago-test",
		Installer:  installer,
		Runner:     runner,
		Logger:     slog.New(slog.NewTextHandler(io.Discard, nil)),
		GOOS:       "windows",
		GOARCH:     "amd64",
		Threads:    3,
	})
}

func awaitStatus(t *testing.T, m *Manager, done func(Status) bool) Status {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		status := m.Status()
		if done(status) {
			return status
		}
		if time.Now().After(deadline) {
			t.Fatalf("status never reached the expected state: %+v", status)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestStatusListsBothModelsWithoutDownloading(t *testing.T) {
	installer := newFakeInstaller()
	status := newTestManager(installer, &fakeRunner{}).Status()
	if !status.Available || status.RuntimeReady || status.Busy || len(status.Models) != 2 {
		t.Fatalf("status = %+v", status)
	}
	for i, id := range []string{"standard", "precise"} {
		model := status.Models[i]
		spec, _ := Model(Kind(id))
		if model.ID != id || model.Name != spec.Name || model.License != spec.License || model.SizeBytes != spec.Asset.Size ||
			model.Ready || model.Downloading || model.Progress != 0 || model.Error != "" {
			t.Fatalf("model %d = %+v", i, model)
		}
	}
	if installer.runtimeCalls+installer.modelCalls != 0 {
		t.Fatal("Status must not download anything")
	}
}

func TestStatusUnavailablePlatformAndEmptyDataDir(t *testing.T) {
	installer := newFakeInstaller()
	installer.runtimeReady = true
	for name, opts := range map[string]Options{
		"platform": {DataDir: "data", GOOS: "plan9", GOARCH: "386", Installer: installer},
		"data dir": {DataDir: " ", GOOS: "windows", GOARCH: "amd64", Installer: installer},
	} {
		m := NewManager(opts)
		status := m.Status()
		if status.Available || status.RuntimeReady || m.Available() {
			t.Errorf("%s: status = %+v", name, status)
		}
		if _, err := m.Prepare(context.Background(), KindStandard); !errors.Is(err, ErrUnavailable) {
			t.Errorf("%s: Prepare error = %v", name, err)
		}
		if _, err := m.Cutout(context.Background(), KindStandard, []byte("x")); !errors.Is(err, ErrUnavailable) {
			t.Errorf("%s: Cutout error = %v", name, err)
		}
	}
}

func TestPrepareIsSingleFlightAndReportsProgress(t *testing.T) {
	installer := newFakeInstaller()
	installer.gate = make(chan struct{})
	m := newTestManager(installer, &fakeRunner{})

	state, err := m.Prepare(context.Background(), KindStandard)
	if err != nil || state != PrepareDownloading {
		t.Fatalf("first Prepare = %q, %v", state, err)
	}
	<-installer.entered // inside EnsureRuntime
	for i := 0; i < 3; i++ {
		if state, err = m.Prepare(context.Background(), KindStandard); err != nil || state != PrepareDownloading {
			t.Fatalf("joining Prepare = %q, %v", state, err)
		}
	}
	status := m.Status()
	if !status.Models[0].Downloading || status.Models[0].Ready || status.Models[1].Downloading {
		t.Fatalf("status while downloading = %+v", status.Models)
	}

	// Half of the runtime bundle arrived: progress is the share of runtime
	// plus model bytes, never 1 before the job ends.
	asset, _ := onnxrt.CPURuntimeAsset("windows", "amd64")
	model, _ := Model(KindStandard)
	m.onProgress(onnxrt.DownloadStatus{Asset: asset.ID, Downloaded: asset.Size / 2, Total: asset.Size})
	want := float64(asset.Size/2) / float64(asset.Size+model.Asset.Size)
	if got := m.Status().Models[0].Progress; got < want-1e-9 || got > want+1e-9 {
		t.Fatalf("runtime phase progress = %v, want %v", got, want)
	}

	installer.gate <- struct{}{} // runtime done
	<-installer.entered          // inside EnsureModel
	m.onProgress(onnxrt.DownloadStatus{Asset: model.Asset.ID, Downloaded: model.Asset.Size, Total: model.Asset.Size})
	if got := m.Status().Models[0].Progress; got <= 0.5 || got >= 1 {
		t.Fatalf("model phase progress = %v, want within (0.5, 1)", got)
	}
	installer.gate <- struct{}{} // model done

	status = awaitStatus(t, m, func(s Status) bool { return s.Models[0].Ready })
	if !status.RuntimeReady || status.Models[0].Downloading || status.Models[0].Progress != 1 || status.Models[1].Ready {
		t.Fatalf("status after download = %+v", status)
	}
	if installer.runtimeCalls != 1 || installer.modelCalls != 1 {
		t.Fatalf("runtime downloaded %d times, model %d times; want once each", installer.runtimeCalls, installer.modelCalls)
	}
	if state, err = m.Prepare(context.Background(), KindStandard); err != nil || state != PrepareReady {
		t.Fatalf("Prepare when present = %q, %v", state, err)
	}
	if installer.runtimeCalls != 1 || installer.modelCalls != 1 {
		t.Fatal("Prepare of a present model must not download again")
	}
	// The second model reuses the runtime.
	installer.gate = nil
	if state, err = m.Prepare(context.Background(), KindPrecise); err != nil || state != PrepareDownloading {
		t.Fatalf("Prepare precise = %q, %v", state, err)
	}
	awaitStatus(t, m, func(s Status) bool { return s.Models[1].Ready })
	if installer.runtimeCalls != 1 || installer.modelCalls != 2 {
		t.Fatalf("runtime %d, model %d downloads; want 1 and 2", installer.runtimeCalls, installer.modelCalls)
	}
}

func TestPrepareFailureIsReportedAndRetried(t *testing.T) {
	installer := newFakeInstaller()
	installer.runtimeReady = true
	installer.modelErr = errors.New("download " + strings.Repeat("x", 500))
	m := newTestManager(installer, &fakeRunner{})

	if _, err := m.Prepare(context.Background(), KindPrecise); err != nil {
		t.Fatal(err)
	}
	status := awaitStatus(t, m, func(s Status) bool { return !s.Models[1].Downloading })
	failed := status.Models[1]
	if failed.Ready || failed.Error == "" || len(failed.Error) > maxStatusError+3 || !strings.Contains(failed.Error, "BiRefNet") {
		t.Fatalf("failed model = %+v", failed)
	}

	installer.mu.Lock()
	installer.modelErr = nil
	installer.mu.Unlock()
	if _, err := m.Prepare(context.Background(), KindPrecise); err != nil {
		t.Fatal(err)
	}
	status = awaitStatus(t, m, func(s Status) bool { return s.Models[1].Ready })
	if status.Models[1].Error != "" {
		t.Fatalf("error was not cleared: %+v", status.Models[1])
	}
}

func TestPrepareStopsWhenTheServerContextEnds(t *testing.T) {
	installer := newFakeInstaller()
	installer.gate = make(chan struct{})
	m := newTestManager(installer, &fakeRunner{})
	ctx, cancel := context.WithCancel(context.Background())
	if _, err := m.Prepare(ctx, KindStandard); err != nil {
		t.Fatal(err)
	}
	<-installer.entered
	cancel()
	status := awaitStatus(t, m, func(s Status) bool { return !s.Models[0].Downloading })
	if status.Models[0].Error != "" {
		t.Fatalf("a shutdown is not a download error: %+v", status.Models[0])
	}
}

func TestPrepareRejectsUnknownModel(t *testing.T) {
	m := newTestManager(newFakeInstaller(), &fakeRunner{})
	if _, err := m.Prepare(context.Background(), "huge"); !errors.Is(err, ErrUnknownModel) {
		t.Fatalf("error = %v", err)
	}
	if _, err := m.Cutout(context.Background(), "huge", nil); !errors.Is(err, ErrUnknownModel) {
		t.Fatalf("error = %v", err)
	}
}

func readyInstaller() *fakeInstaller {
	installer := newFakeInstaller()
	installer.runtimeReady = true
	spec, _ := Model(KindStandard)
	installer.modelReady[spec.Asset.ID] = true
	return installer
}

func TestCutoutRunsTheWorkerWithTheManagedPaths(t *testing.T) {
	runner := &fakeRunner{result: []byte("matte")}
	m := newTestManager(readyInstaller(), runner)
	result, err := m.Cutout(context.Background(), KindStandard, []byte("image"))
	if err != nil || string(result.PNG) != "matte" || result.Duration < 0 {
		t.Fatalf("Cutout = %+v, %v", result, err)
	}
	spec, _ := Model(KindStandard)
	asset, _ := onnxrt.CPURuntimeAsset("windows", "amd64")
	request := runner.request
	if request.Executable != "aurago-test" || request.Kind != KindStandard || request.Threads != 3 || string(request.Image) != "image" ||
		request.RuntimeRoot != filepath.Join("runtimes", asset.ID) ||
		request.ModelPath != filepath.Join("data", "pixelmatte", "models", spec.Asset.FileName) {
		t.Fatalf("request = %+v", request)
	}
}

func TestCutoutNeedsDownloadedModelAndRuntime(t *testing.T) {
	spec, _ := Model(KindStandard)
	for name, setup := range map[string]func(*fakeInstaller){
		"nothing":     func(*fakeInstaller) {},
		"no runtime":  func(f *fakeInstaller) { f.modelReady[spec.Asset.ID] = true },
		"no model":    func(f *fakeInstaller) { f.runtimeReady = true },
		"other model": func(f *fakeInstaller) { f.runtimeReady = true; f.modelReady["birefnet-general-lite"] = true },
	} {
		installer := newFakeInstaller()
		setup(installer)
		runner := &fakeRunner{result: []byte("x")}
		if _, err := newTestManager(installer, runner).Cutout(context.Background(), KindStandard, []byte("i")); !errors.Is(err, ErrNotReady) {
			t.Errorf("%s: error = %v, want ErrNotReady", name, err)
		}
		if runner.request.Executable != "" {
			t.Errorf("%s: the worker must not start", name)
		}
	}
}

func TestCutoutAllowsOneRunAtATime(t *testing.T) {
	runner := &fakeRunner{result: []byte("m"), started: make(chan struct{}, 1), release: make(chan struct{})}
	m := newTestManager(readyInstaller(), runner)
	if m.Busy() {
		t.Fatal("Busy must be false before any run")
	}
	done := make(chan error, 1)
	go func() {
		_, err := m.Cutout(context.Background(), KindStandard, []byte("a"))
		done <- err
	}()
	<-runner.started
	if !m.Status().Busy || !m.Busy() {
		t.Fatal("status and Busy must report busy while a cutout runs")
	}
	if _, err := m.Cutout(context.Background(), KindStandard, []byte("b")); !errors.Is(err, ErrBusy) {
		t.Fatalf("second cutout error = %v, want ErrBusy", err)
	}
	close(runner.release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if m.Status().Busy || m.Busy() {
		t.Fatal("busy must clear after the run")
	}
	if _, err := m.Cutout(context.Background(), KindStandard, []byte("c")); err != nil {
		t.Fatalf("cutout after the first finished: %v", err)
	}
}

func TestCutoutTimeoutAndCancellation(t *testing.T) {
	runner := &fakeRunner{started: make(chan struct{}, 1), release: make(chan struct{})}
	m := NewManager(Options{
		DataDir: "data", Executable: "x", Installer: readyInstaller(), Runner: runner,
		GOOS: "windows", GOARCH: "amd64", Timeout: 50 * time.Millisecond,
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if _, err := m.Cutout(context.Background(), KindStandard, []byte("a")); !errors.Is(err, ErrTimeout) {
		t.Fatalf("error = %v, want ErrTimeout", err)
	}
	<-runner.started
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := m.Cutout(ctx, KindStandard, []byte("a"))
	if !errors.Is(err, context.Canceled) || errors.Is(err, ErrTimeout) {
		t.Fatalf("cancelled request error = %v, want context.Canceled", err)
	}
}

func TestCacheInstallerProbesAreCheap(t *testing.T) {
	dir := t.TempDir()
	installer := &cacheInstaller{runtimeRoot: dir}
	spec := onnxrt.AssetSpec{ID: "ort-test", Size: 5}
	model := filepath.Join(dir, "m.onnx")

	if installer.ModelReady(spec, model) {
		t.Fatal("a missing model is not ready")
	}
	if err := os.WriteFile(model, []byte("1234"), 0o600); err != nil {
		t.Fatal(err)
	}
	if installer.ModelReady(spec, model) {
		t.Fatal("a model of the wrong size is not ready")
	}
	if err := os.WriteFile(model, []byte("12345"), 0o600); err != nil {
		t.Fatal(err)
	}
	if !installer.ModelReady(spec, model) {
		t.Fatal("a model of the pinned size is ready")
	}

	runtimeDir := onnxrt.RuntimeDir(dir, spec)
	if err := os.MkdirAll(runtimeDir, 0o750); err != nil {
		t.Fatal(err)
	}
	if _, ok := installer.RuntimeReady(spec); ok {
		t.Fatal("a runtime without its marker is not ready")
	}
	if err := os.WriteFile(filepath.Join(runtimeDir, ".complete.json"), []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, ok := installer.RuntimeReady(spec); ok {
		t.Fatal("a runtime without its library is not ready")
	}
	if err := os.WriteFile(filepath.Join(runtimeDir, onnxrt.LibraryName()), []byte("lib"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got, ok := installer.RuntimeReady(spec); !ok || got != runtimeDir {
		t.Fatalf("RuntimeReady = %q, %v; want %q, true", got, ok, runtimeDir)
	}
}
