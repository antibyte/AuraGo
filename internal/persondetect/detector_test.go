package persondetect

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"aurago/internal/onnxrt"
)

type fakeInstaller struct {
	root     string
	mu       sync.Mutex
	runtimes []string
}

func (f *fakeInstaller) Model(_ context.Context, _ onnxrt.AssetSpec, destination string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := os.MkdirAll(filepath.Dir(destination), 0o750); err != nil {
		return err
	}
	return os.WriteFile(destination, []byte("model"), 0o600)
}

func (f *fakeInstaller) Runtime(_ context.Context, spec onnxrt.AssetSpec) (string, error) {
	f.mu.Lock()
	f.runtimes = append(f.runtimes, spec.ID)
	f.mu.Unlock()
	dir := filepath.Join(f.root, "runtimes", spec.ID)
	return dir, os.MkdirAll(dir, 0o750)
}

func (f *fakeInstaller) installed() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.runtimes...)
}

type detectorFixture struct {
	dir       string
	opts      DetectorOptions
	deps      detectorDeps
	installer *fakeInstaller
	phases    []string
	logs      *syncBuffer
}

// syncBuffer collects slog output written from several goroutines.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(data []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(data)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// restartPending reports the plain ErrWorkerRestarting a Detect returns
// during the backoff; the Detect that saw the failure returns an error that
// wraps ErrWorkerRestarting and the cause.
func restartPending(err error) bool {
	return err == ErrWorkerRestarting
}

func newDetectorFixture(t *testing.T, mode string, backend Backend) *detectorFixture {
	t.Helper()
	dir := t.TempDir()
	t.Setenv(fakeModeEnv, mode)
	t.Setenv(fakeDirEnv, dir)
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	f := &detectorFixture{dir: dir, installer: &fakeInstaller{root: filepath.Join(dir, "runtime-cache")}, logs: &syncBuffer{}}
	f.opts = DetectorOptions{
		ModelDir:        filepath.Join(dir, "models"),
		RuntimeCacheDir: filepath.Join(dir, "runtime-cache"),
		Model:           ModelStandard,
		Backend:         backend,
		Logger:          slog.New(slog.NewTextHandler(f.logs, nil)),
		OnProgress:      func(progress SetupProgress) { f.phases = append(f.phases, progress.Phase) },
		GPUProbe:        func() (bool, string) { return true, "" },
		Executable:      executable,
	}
	f.deps = detectorDeps{
		install:       f.installer,
		goos:          "linux",
		goarch:        "amd64",
		webgpuAsset:   func(string, string) (onnxrt.AssetSpec, bool) { return onnxrt.AssetSpec{ID: "ort-test-webgpu"}, true },
		cpuAsset:      func(string, string) (onnxrt.AssetSpec, bool) { return onnxrt.AssetSpec{ID: "ort-test-cpu"}, true },
		cpuThreads:    2,
		startTimeout:  30 * time.Second,
		detectTimeout: 5 * time.Second,
		backoffBase:   50 * time.Millisecond,
		backoffMax:    200 * time.Millisecond,
		maxFailures:   5,
	}
	return f
}

func (f *detectorFixture) start(t *testing.T) Detector {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	detector, err := newDetector(ctx, f.opts, f.deps)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = detector.Close() })
	return detector
}

func (f *detectorFixture) calls(name string) int {
	return fakeCallCount(f.dir, name)
}

func (f *detectorFixture) gpuManifestPath() string {
	return filepath.Join(f.opts.ModelDir, "standard", "model.webgpu-1.27.0.json")
}

func detectFrame(detector Detector) (DetectResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return detector.Detect(ctx, []byte("frame"), 0.5)
}

func TestDetectorAcceptsGPUAndCachesPreparedModel(t *testing.T) {
	f := newDetectorFixture(t, "gpu_cpu_ms=10,cpu_cpu_ms=100", BackendAuto)
	detector := f.start(t)
	info := detector.Info()
	if info.Active != BackendGPU || info.Requested != BackendAuto || info.FallbackReason != "" ||
		info.Runtime != "onnxruntime 1.27.0" || info.ModelName != "D-FINE-N" || info.Model != ModelStandard ||
		!info.SelfTest.Passed || info.SelfTest.GPUCPUMS != 10 || !slices.Equal(info.Providers, []string{"CPUExecutionProvider"}) {
		t.Fatalf("info = %+v", info)
	}
	wantPhases := []string{phaseDownloadingModel, phaseDownloadingRuntime, phasePreparingGPUModel, phaseStartingWorker, phaseSelfTest}
	if !slices.Equal(f.phases, wantPhases) {
		t.Fatalf("phases = %v, want %v", f.phases, wantPhases)
	}
	result, err := detectFrame(detector)
	if err != nil || len(result.Boxes) != 1 || result.Width != 64 || result.Height != 48 || result.WallMS != 5 || result.CPUMS != 4 {
		t.Fatalf("Detect = %+v, %v", result, err)
	}
	spec, _ := Model(ModelStandard)
	raw, err := os.ReadFile(f.gpuManifestPath())
	if err != nil {
		t.Fatal(err)
	}
	if want := fmt.Sprintf(`{"source_sha256":"%s","ort_version":"1.27.0","patched_nodes":1}`, spec.Asset.SHA256); string(raw) != want {
		t.Fatalf("GPU model manifest = %s, want %s", raw, want)
	}
	if f.calls("prepare") != 1 || f.calls("serve-webgpu") != 1 || f.calls("serve-cpu") != 0 {
		t.Fatalf("calls prepare=%d webgpu=%d cpu=%d", f.calls("prepare"), f.calls("serve-webgpu"), f.calls("serve-cpu"))
	}
	if err := detector.Close(); err != nil {
		t.Fatal(err)
	}

	again := f.start(t)
	if again.Info().Active != BackendGPU || f.calls("prepare") != 1 {
		t.Fatalf("cached GPU model not reused: active %s, prepare calls %d", again.Info().Active, f.calls("prepare"))
	}
	if installed := f.installer.installed(); !slices.Equal(installed, []string{"ort-test-webgpu", "ort-test-webgpu"}) {
		t.Fatalf("installed runtimes = %v", installed)
	}
}

func TestDetectorRejectsGPUThatSavesTooLittleCPU(t *testing.T) {
	f := newDetectorFixture(t, "gpu_cpu_ms=61,cpu_cpu_ms=100", BackendAuto)
	info := f.start(t).Info()
	if info.Active != BackendCPU || info.FallbackReason != "GPU path used 61% of the CPU path's CPU time" {
		t.Fatalf("info = %+v", info)
	}
	if f.calls("serve-webgpu") != 1 || f.calls("serve-cpu") != 1 {
		t.Fatalf("calls webgpu=%d cpu=%d", f.calls("serve-webgpu"), f.calls("serve-cpu"))
	}
	if _, err := os.Stat(f.gpuManifestPath()); err != nil {
		t.Fatalf("a CPU-time rejection must keep the prepared model: %v", err)
	}
}

func TestDetectorAcceptsGPUWithoutGPUCPUTime(t *testing.T) {
	// gpu_cpu_ms 0 is omitted from the hello (omitempty) and reads as 0.
	f := newDetectorFixture(t, "gpu_cpu_ms=0,cpu_cpu_ms=100", BackendGPU)
	info := f.start(t).Info()
	if info.Active != BackendGPU || info.FallbackReason != "" || info.SelfTest.GPUCPUMS != 0 || info.SelfTest.CPUCPUMS != 100 {
		t.Fatalf("info = %+v", info)
	}
}

func TestDetectorFallsBackWhenGPUSelfTestFails(t *testing.T) {
	f := newDetectorFixture(t, "gpu_selftest_fail", BackendGPU)
	info := f.start(t).Info()
	if info.Active != BackendCPU || info.Requested != BackendGPU ||
		info.FallbackReason != "self-test: GPU/CPU detections differ: count 3 vs 4" {
		t.Fatalf("info = %+v", info)
	}
	if _, err := os.Stat(f.gpuManifestPath()); !os.IsNotExist(err) {
		t.Fatalf("suspect GPU model was kept: %v", err)
	}
}

func TestDetectorFallsBackWhenPrepareFails(t *testing.T) {
	f := newDetectorFixture(t, "prepare_fail", BackendAuto)
	info := f.start(t).Info()
	if info.Active != BackendCPU || info.FallbackReason != "GPU model preparation: fake prepare failure" {
		t.Fatalf("info = %+v", info)
	}
	if f.calls("serve-webgpu") != 0 || f.calls("serve-cpu") != 1 {
		t.Fatalf("calls webgpu=%d cpu=%d", f.calls("serve-webgpu"), f.calls("serve-cpu"))
	}
}

func TestDetectorCPUBackendSkipsGPU(t *testing.T) {
	f := newDetectorFixture(t, "", BackendCPU)
	info := f.start(t).Info()
	if info.Active != BackendCPU || info.Requested != BackendCPU || info.FallbackReason != "" || !info.SelfTest.Passed {
		t.Fatalf("info = %+v", info)
	}
	if installed := f.installer.installed(); !slices.Equal(installed, []string{"ort-test-cpu"}) {
		t.Fatalf("installed runtimes = %v", installed)
	}
	if f.calls("prepare") != 0 || f.calls("serve-webgpu") != 0 {
		t.Fatal("CPU backend touched the GPU path")
	}
}

func TestDetectorReportsMissingWebGPURuntime(t *testing.T) {
	f := newDetectorFixture(t, "", BackendGPU)
	f.deps.goos, f.deps.goarch = "linux", "arm64"
	f.deps.webgpuAsset = onnxrt.WebGPURuntimeAsset
	info := f.start(t).Info()
	if info.Active != BackendCPU || info.FallbackReason != "no WebGPU runtime for linux/arm64" {
		t.Fatalf("info = %+v", info)
	}
}

func TestDetectorReportsInaccessibleGPU(t *testing.T) {
	f := newDetectorFixture(t, "", BackendAuto)
	f.opts.GPUProbe = func() (bool, string) { return false, "no accessible Vulkan render device under /dev/dri" }
	info := f.start(t).Info()
	if info.Active != BackendCPU || info.FallbackReason != "GPU not accessible: no accessible Vulkan render device under /dev/dri" {
		t.Fatalf("info = %+v", info)
	}
	if installed := f.installer.installed(); !slices.Equal(installed, []string{"ort-test-cpu"}) {
		t.Fatalf("installed runtimes = %v", installed)
	}
}

func TestDetectorFailsWhenCPUSelfTestFails(t *testing.T) {
	f := newDetectorFixture(t, "cpu_selftest_fail", BackendCPU)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	detector, err := newDetector(ctx, f.opts, f.deps)
	if err == nil {
		_ = detector.Close()
		t.Fatal("detector started despite a failing CPU self-test")
	}
	if !strings.Contains(err.Error(), "CPU worker: self-test: no person with score >= 0.5") {
		t.Fatalf("error = %v", err)
	}
}

func TestDetectorValidatesOptionsAndFrames(t *testing.T) {
	f := newDetectorFixture(t, "", BackendCPU)
	ctx := context.Background()
	for name, mutate := range map[string]func(*DetectorOptions){
		"model":     func(o *DetectorOptions) { o.Model = "huge" },
		"backend":   func(o *DetectorOptions) { o.Backend = "cuda" },
		"model dir": func(o *DetectorOptions) { o.ModelDir = "" },
	} {
		opts := f.opts
		mutate(&opts)
		if detector, err := newDetector(ctx, opts, f.deps); err == nil {
			_ = detector.Close()
			t.Errorf("%s: invalid options accepted", name)
		}
	}
	detector := f.start(t)
	if _, err := detector.Detect(ctx, nil, 0.5); err == nil {
		t.Error("empty frame accepted")
	}
	if _, err := detector.Detect(ctx, []byte("frame"), 0); err == nil {
		t.Error("min score 0 accepted")
	}
	if _, err := detector.Detect(ctx, []byte("frame"), 1.5); err == nil {
		t.Error("min score 1.5 accepted")
	}
}

func TestDetectorRestartsAfterCrash(t *testing.T) {
	f := newDetectorFixture(t, "crash_first_detect", BackendCPU)
	f.deps.backoffBase, f.deps.backoffMax = 3*time.Second, 3*time.Second
	detector := f.start(t)
	_, err := detectFrame(detector)
	if err == nil || errors.Is(err, ErrDetectorFailed) || !errors.Is(err, ErrWorkerRestarting) || restartPending(err) ||
		err.Error() != "person detector worker is restarting in 3s: worker exited unexpectedly (exit status 3)" {
		t.Fatalf("first Detect error = %v", err)
	}
	if _, err := detectFrame(detector); !restartPending(err) {
		t.Fatalf("Detect during backoff = %v, want ErrWorkerRestarting", err)
	}
	deadline := time.Now().Add(15 * time.Second)
	for {
		result, err := detectFrame(detector)
		if err == nil {
			if len(result.Boxes) != 1 {
				t.Fatalf("result after restart = %+v", result)
			}
			break
		}
		if !restartPending(err) || time.Now().After(deadline) {
			t.Fatalf("Detect after restart = %v", err)
		}
		time.Sleep(20 * time.Millisecond)
	}
	if f.calls("serve-cpu") != 2 {
		t.Fatalf("worker starts = %d, want 2", f.calls("serve-cpu"))
	}
}

func TestDetectorFailsAfterFiveConsecutiveFailures(t *testing.T) {
	f := newDetectorFixture(t, "crash_detect", BackendCPU)
	detector := f.start(t)
	failures := 0
	deadline := time.Now().Add(30 * time.Second)
	for {
		_, err := detectFrame(detector)
		if err == nil {
			t.Fatal("crashing worker answered")
		}
		if restartPending(err) {
			if time.Now().After(deadline) {
				t.Fatal("worker never restarted")
			}
			time.Sleep(10 * time.Millisecond)
			continue
		}
		failures++
		if errors.Is(err, ErrDetectorFailed) {
			if errors.Is(err, ErrWorkerRestarting) || !strings.Contains(err.Error(), "after 5 consecutive failures: worker exited unexpectedly") {
				t.Fatalf("final failure = %v", err)
			}
			break
		}
		if !errors.Is(err, ErrWorkerRestarting) || failures > 5 {
			t.Fatalf("failure %d = %v", failures, err)
		}
	}
	if failures != 5 {
		t.Fatalf("failures = %d, want 5", failures)
	}
	if _, err := detectFrame(detector); !errors.Is(err, ErrDetectorFailed) {
		t.Fatalf("Detect after failure = %v", err)
	}
	if f.calls("serve-cpu") != 5 {
		t.Fatalf("worker starts = %d, want 5 (initial + 4 restarts)", f.calls("serve-cpu"))
	}
}

func TestDetectorSuccessResetsFailureCount(t *testing.T) {
	// Every worker answers its first frame and crashes on the second: six
	// crashes, each after a success, never add up to five consecutive ones.
	f := newDetectorFixture(t, "crash_second_detect", BackendCPU)
	detector := f.start(t)
	crashes, successes := 0, 0
	deadline := time.Now().Add(45 * time.Second)
	for crashes < 6 {
		if time.Now().After(deadline) {
			t.Fatalf("timed out after %d crashes, %d successes", crashes, successes)
		}
		_, err := detectFrame(detector)
		switch {
		case err == nil:
			successes++
		case restartPending(err):
			time.Sleep(10 * time.Millisecond)
		case errors.Is(err, ErrWorkerRestarting):
			crashes++
		default:
			t.Fatalf("Detect after %d crashes, %d successes = %v", crashes, successes, err)
		}
	}
	if successes < 6 {
		t.Fatalf("successes = %d, want at least 6", successes)
	}
}

func TestDetectorFrameErrorIsNotAWorkerFailure(t *testing.T) {
	f := newDetectorFixture(t, "frame_error", BackendCPU)
	detector := f.start(t)
	for i := 0; i < 6; i++ {
		_, err := detectFrame(detector)
		if err == nil || err.Error() != "detect: decode JPEG header: fake frame error" || errors.Is(err, ErrWorkerRestarting) {
			t.Fatalf("Detect %d = %v", i, err)
		}
	}
	if f.calls("serve-cpu") != 1 {
		t.Fatalf("worker starts = %d, want 1", f.calls("serve-cpu"))
	}
}

func TestDetectorTreatsIDLessErrorLineAsWorkerFailure(t *testing.T) {
	f := newDetectorFixture(t, "panic_detect", BackendCPU)
	f.deps.backoffBase, f.deps.backoffMax = 3*time.Second, 3*time.Second
	detector := f.start(t)
	_, err := detectFrame(detector)
	if err == nil || errors.Is(err, ErrDetectorFailed) || !errors.Is(err, ErrWorkerRestarting) ||
		!strings.HasSuffix(err.Error(), ": worker failed: person detection worker panic: fake panic") {
		t.Fatalf("Detect error = %v", err)
	}
	if _, err := detectFrame(detector); !restartPending(err) {
		t.Fatalf("Detect after the panic line = %v, want ErrWorkerRestarting", err)
	}
}

func TestDetectorReportsWorkerPanicAtStart(t *testing.T) {
	f := newDetectorFixture(t, "panic_start", BackendAuto)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	detector, err := newDetector(ctx, f.opts, f.deps)
	if err == nil {
		_ = detector.Close()
		t.Fatal("detector started although every worker panicked")
	}
	if want := "start CPU worker: worker failed: person detection worker panic: fake panic"; err.Error() != want {
		t.Fatalf("error = %v, want %s", err, want)
	}
	if f.calls("serve-webgpu") != 1 {
		t.Fatalf("webgpu starts = %d, want 1", f.calls("serve-webgpu"))
	}
	if _, statErr := os.Stat(f.gpuManifestPath()); !os.IsNotExist(statErr) {
		t.Fatalf("GPU model of a worker that failed to start was kept: %v", statErr)
	}
}

func TestDetectorIgnoresWorkerNoise(t *testing.T) {
	f := newDetectorFixture(t, "noise", BackendAuto)
	detector := f.start(t)
	if info := detector.Info(); info.Active != BackendGPU || info.FallbackReason != "" {
		t.Fatalf("info = %+v", info)
	}
	for i := 0; i < 3; i++ {
		if result, err := detectFrame(detector); err != nil || len(result.Boxes) != 1 {
			t.Fatalf("Detect %d = %+v, %v", i, result, err)
		}
	}
	if f.calls("serve-webgpu") != 1 {
		t.Fatalf("webgpu starts = %d, want 1", f.calls("serve-webgpu"))
	}
}

func TestDetectorSerializesConcurrentPrepares(t *testing.T) {
	f := newDetectorFixture(t, "prepare_sleep_ms=500", BackendGPU)
	f.opts.OnProgress = nil
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	var wg sync.WaitGroup
	detectors := make([]Detector, 2)
	errs := make([]error, 2)
	for i := range detectors {
		wg.Add(1)
		go func() {
			defer wg.Done()
			detectors[i], errs[i] = newDetector(ctx, f.opts, f.deps)
		}()
	}
	wg.Wait()
	for _, detector := range detectors {
		if detector != nil {
			t.Cleanup(func() { _ = detector.Close() })
		}
	}
	for i, detector := range detectors {
		if errs[i] != nil {
			t.Fatalf("setup %d: %v", i, errs[i])
		}
		if info := detector.Info(); info.Active != BackendGPU {
			t.Fatalf("setup %d info = %+v", i, info)
		}
	}
	// The second setup waits for the first one's prepare and reuses its model.
	if f.calls("prepare") != 1 || f.calls("serve-webgpu") != 2 {
		t.Fatalf("calls prepare=%d webgpu=%d", f.calls("prepare"), f.calls("serve-webgpu"))
	}
}

func TestDetectorCancelDuringPrepare(t *testing.T) {
	f := newDetectorFixture(t, "prepare_hang", BackendGPU)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		deadline := time.Now().Add(30 * time.Second)
		for f.calls("prepare") == 0 && time.Now().Before(deadline) {
			time.Sleep(10 * time.Millisecond)
		}
		cancel()
	}()
	start := time.Now()
	detector, err := newDetector(ctx, f.opts, f.deps)
	if err == nil {
		_ = detector.Close()
		t.Fatal("setup succeeded with a hanging prepare")
	}
	if !errors.Is(err, context.Canceled) || f.calls("prepare") != 1 {
		t.Fatalf("error = %v, prepare calls %d", err, f.calls("prepare"))
	}
	if elapsed := time.Since(start); elapsed > 20*time.Second {
		t.Fatalf("cancelled setup took %s", elapsed)
	}
	if f.calls("serve-cpu") != 0 {
		t.Fatal("a cancelled setup fell back to the CPU")
	}
	// The model lock was released: the next setup prepares again.
	t.Setenv(fakeModeEnv, "")
	if info := f.start(t).Info(); info.Active != BackendGPU || f.calls("prepare") != 2 {
		t.Fatalf("setup after cancel: info %+v, prepare calls %d", info, f.calls("prepare"))
	}
}

func TestDetectorKillsHungWorker(t *testing.T) {
	f := newDetectorFixture(t, "hang_detect", BackendCPU)
	f.deps.detectTimeout = 300 * time.Millisecond
	f.deps.backoffBase, f.deps.backoffMax = 3*time.Second, 3*time.Second
	detector := f.start(t)
	start := time.Now()
	_, err := detectFrame(detector)
	if err == nil || !errors.Is(err, ErrWorkerRestarting) || !strings.HasSuffix(err.Error(), ": detect timed out after 300ms") {
		t.Fatalf("Detect error = %v", err)
	}
	if elapsed := time.Since(start); elapsed > 8*time.Second {
		t.Fatalf("hung worker took %s to kill", elapsed)
	}
	if _, err := detectFrame(detector); !restartPending(err) {
		t.Fatalf("Detect after kill = %v, want ErrWorkerRestarting", err)
	}
}

func TestDetectorCloseIsFinal(t *testing.T) {
	f := newDetectorFixture(t, "", BackendCPU)
	detector := f.start(t)
	if err := detector.Close(); err != nil {
		t.Fatal(err)
	}
	if err := detector.Close(); err != nil {
		t.Fatalf("second Close = %v", err)
	}
	if _, err := detectFrame(detector); !errors.Is(err, ErrDetectorClosed) {
		t.Fatalf("Detect after Close = %v", err)
	}
}

func TestBackoffDelay(t *testing.T) {
	cases := map[int]time.Duration{1: time.Second, 2: 2 * time.Second, 3: 4 * time.Second, 6: 32 * time.Second, 7: time.Minute, 20: time.Minute}
	for failures, want := range cases {
		if got := backoffDelay(time.Second, time.Minute, failures); got != want {
			t.Errorf("backoffDelay(%d) = %s, want %s", failures, got, want)
		}
	}
}

func TestGPURejection(t *testing.T) {
	cases := []struct {
		selfTest *SelfTest
		reason   string
		suspect  bool
	}{
		{nil, "self-test: worker reported no self-test", true},
		{&SelfTest{Reason: "GPU/CPU detections differ: count 1 vs 2"}, "self-test: GPU/CPU detections differ: count 1 vs 2", true},
		{&SelfTest{Passed: true, GPUCPUMS: 10}, "self-test: CPU reference time is missing", false},
		{&SelfTest{Passed: true, GPUCPUMS: 50, CPUCPUMS: 100}, "", false},
		{&SelfTest{Passed: true, CPUCPUMS: 100}, "", false}, // missing GPU CPU time reads as 0
		{&SelfTest{Passed: true, GPUCPUMS: 61, CPUCPUMS: 100}, "GPU path used 61% of the CPU path's CPU time", false},
	}
	for _, tc := range cases {
		reason, suspect := gpuRejection(tc.selfTest)
		if reason != tc.reason || suspect != tc.suspect {
			t.Errorf("gpuRejection(%+v) = %q, %v; want %q, %v", tc.selfTest, reason, suspect, tc.reason, tc.suspect)
		}
	}
}

func TestReadWorkerLineSkipsOverlongLines(t *testing.T) {
	input := "short\n" + strings.Repeat("x", 100) + "\n" + "tail without newline"
	reader := bufio.NewReaderSize(strings.NewReader(input), 16)
	type read struct {
		line     string
		overlong bool
		err      error
	}
	var reads []read
	for {
		line, overlong, err := readWorkerLine(reader, 32)
		reads = append(reads, read{string(line), overlong, err})
		if err != nil {
			break
		}
	}
	want := []read{{"short\n", false, nil}, {"", true, nil}, {"tail without newline", false, io.EOF}}
	if !slices.Equal(reads, want) {
		t.Fatalf("reads = %+v, want %+v", reads, want)
	}
}

func TestGPUModelPathsFollowTheAssetFileName(t *testing.T) {
	spec, _ := Model(ModelAccurate)
	spec.Asset.FileName = "dfine.onnx"
	s := &setup{opts: DetectorOptions{ModelDir: filepath.Join("data", "models")}, spec: spec}
	gpuPath, manifestPath := s.gpuModelPaths()
	base := filepath.Join("data", "models", "accurate", "dfine.webgpu-"+onnxrt.WebGPURuntimeVersion)
	if gpuPath != base+".onnx" || manifestPath != base+".json" {
		t.Fatalf("paths = %s, %s", gpuPath, manifestPath)
	}
}
