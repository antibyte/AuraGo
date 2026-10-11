package persondetect

import (
	"context"
	"errors"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

// waitForCalls waits until the fake workers recorded want calls of name.
func waitForCalls(t *testing.T, f *detectorFixture, name string, want int) {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for f.calls(name) < want {
		if time.Now().After(deadline) {
			t.Fatalf("%s calls = %d, want %d", name, f.calls(name), want)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// currentWorker returns the detector's running worker.
func currentWorker(t *testing.T, detector Detector) *workerProcess {
	t.Helper()
	d := detector.(*workerDetector)
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.worker == nil {
		t.Fatal("no running worker")
	}
	return d.worker
}

func workerGone(worker *workerProcess) bool {
	select {
	case <-worker.done:
		return true
	default:
		return false
	}
}

// waitForGoroutines waits until at most limit goroutines run.
func waitForGoroutines(t *testing.T, limit int) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for runtime.NumGoroutine() > limit {
		if time.Now().After(deadline) {
			buf := make([]byte, 1<<20)
			t.Fatalf("goroutines = %d, want <= %d\n%s", runtime.NumGoroutine(), limit, buf[:runtime.Stack(buf, true)])
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestDetectorCloseReleasesInFlightDetect(t *testing.T) {
	f := newDetectorFixture(t, "hang_detect", BackendCPU)
	f.deps.detectTimeout = time.Minute
	detector := f.start(t)
	worker := currentWorker(t, detector)
	result := make(chan error, 1)
	go func() {
		_, err := detector.Detect(context.Background(), []byte("frame"), 0.5)
		result <- err
	}()
	waitForCalls(t, f, "detect", 1)
	closed := make(chan struct{})
	go func() {
		_ = detector.Close()
		close(closed)
	}()
	select {
	case err := <-result:
		if !errors.Is(err, ErrDetectorClosed) {
			t.Fatalf("in-flight Detect = %v, want ErrDetectorClosed", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Close did not release the in-flight Detect")
	}
	select {
	case <-closed:
	case <-time.After(15 * time.Second):
		t.Fatal("Close hung")
	}
	if !workerGone(worker) {
		t.Fatal("Close returned before the worker exited")
	}
}

func TestDetectorCloseDuringRestart(t *testing.T) {
	f := newDetectorFixture(t, "crash_first_detect,restart_hang", BackendCPU)
	f.deps.backoffBase = 20 * time.Millisecond
	detector := f.start(t)
	if _, err := detectFrame(detector); !errors.Is(err, ErrWorkerRestarting) {
		t.Fatalf("Detect = %v, want a worker failure", err)
	}
	waitForCalls(t, f, "serve-cpu", 2) // the restarted worker hangs before its hello
	start := time.Now()
	if err := detector.Close(); err != nil {
		t.Fatal(err)
	}
	if elapsed := time.Since(start); elapsed > 10*time.Second {
		t.Fatalf("Close during a restart took %s", elapsed)
	}
	if _, err := detectFrame(detector); !errors.Is(err, ErrDetectorClosed) {
		t.Fatalf("Detect after Close = %v", err)
	}
	// Only the crash is a failure; the restart Close interrupted is not.
	if logs := f.logs.String(); strings.Count(logs, "Person detection worker failed") != 1 || strings.Contains(logs, "restart:") {
		t.Fatalf("logs:\n%s", logs)
	}
}

func TestDetectorCloseDuringBackoff(t *testing.T) {
	f := newDetectorFixture(t, "crash_first_detect", BackendCPU)
	f.deps.backoffBase = 300 * time.Millisecond
	detector := f.start(t)
	if _, err := detectFrame(detector); !errors.Is(err, ErrWorkerRestarting) {
		t.Fatalf("Detect = %v, want a worker failure", err)
	}
	if err := detector.Close(); err != nil {
		t.Fatal(err)
	}
	time.Sleep(800 * time.Millisecond)
	if starts := f.calls("serve-cpu"); starts != 1 {
		t.Fatalf("worker starts after Close = %d, want 1", starts)
	}
}

func TestDetectorCloseCallsAllWaitForTheWorker(t *testing.T) {
	// The worker never reads its stdin: shutdown waits for the close answer
	// and then kills it.
	f := newDetectorFixture(t, "ignore_close", BackendCPU)
	detector := f.start(t)
	worker := currentWorker(t, detector)
	var wg sync.WaitGroup
	early := make(chan int, 3)
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = detector.Close()
			if !workerGone(worker) {
				early <- i
			}
		}()
	}
	wg.Wait()
	close(early)
	for i := range early {
		t.Errorf("Close call %d returned before the worker exited", i)
	}
}

func TestDetectorSkipsAnswersToAbandonedFrames(t *testing.T) {
	f := newDetectorFixture(t, "echo_id,detect_sleep_ms=300", BackendCPU)
	detector := f.start(t)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	_, err := detector.Detect(ctx, []byte("frame-1"), 0.5)
	cancel()
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("first Detect = %v", err)
	}
	result, err := detectFrame(detector)
	if err != nil || result.Width != 2 || result.Height != len("frame") {
		t.Fatalf("second Detect = %+v, %v (want the answer to request 2)", result, err)
	}
	if f.calls("serve-cpu") != 1 || f.calls("detect") != 2 {
		t.Fatalf("serve starts %d, frames %d", f.calls("serve-cpu"), f.calls("detect"))
	}
}

func TestDetectorDoesNotSendWithDoneContext(t *testing.T) {
	f := newDetectorFixture(t, "echo_id", BackendCPU)
	detector := f.start(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for i := 0; i < 20; i++ {
		if _, err := detector.Detect(ctx, []byte("frame"), 0.5); !errors.Is(err, context.Canceled) {
			t.Fatalf("Detect with a cancelled context = %v", err)
		}
	}
	// The first frame sent gets id 1: no request went out before.
	if result, err := detectFrame(detector); err != nil || result.Width != 1 || f.calls("detect") != 1 {
		t.Fatalf("Detect = %+v, %v; frames sent %d", result, err, f.calls("detect"))
	}
}

func TestDetectorReportsCleanWorkerExit(t *testing.T) {
	f := newDetectorFixture(t, "exit_after_hello", BackendCPU)
	detector := f.start(t)
	worker := currentWorker(t, detector)
	select {
	case <-worker.done:
	case <-time.After(10 * time.Second):
		t.Fatal("worker did not exit after its hello")
	}
	_, err := detectFrame(detector)
	if !errors.Is(err, ErrWorkerRestarting) || !strings.HasSuffix(err.Error(), ": worker exited unexpectedly (exit status 0)") {
		t.Fatalf("Detect on an exited worker = %v", err)
	}
	if f.calls("detect") != 0 {
		t.Fatal("a frame was sent to an exited worker")
	}
	deadline := time.Now().Add(15 * time.Second)
	for {
		_, err := detectFrame(detector)
		if err == nil {
			break
		}
		if !restartPending(err) || time.Now().After(deadline) {
			t.Fatalf("Detect after restart = %v", err)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// A worker that never reads stdin while every caller gives up before the
// detect timeout: the frames' budget still runs out and the worker is killed;
// the pending writes and the reader goroutines go away with it.
func TestDetectorKillsHungWorkerAcrossShortCallerContexts(t *testing.T) {
	f := newDetectorFixture(t, "no_read", BackendCPU)
	f.deps.detectTimeout = time.Second
	f.deps.backoffBase = 50 * time.Millisecond
	detector := f.start(t)
	baseline := runtime.NumGoroutine()
	frames := [][]byte{make([]byte, maxFrameBytes), make([]byte, 1<<20)}
	start := time.Now()
	calls := 0
	for {
		if time.Since(start) > 15*time.Second {
			t.Fatal("hung worker was never killed")
		}
		ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		_, err := detector.Detect(ctx, frames[min(calls, 1)], 0.5)
		cancel()
		calls++
		if errors.Is(err, context.DeadlineExceeded) {
			continue
		}
		if !errors.Is(err, ErrWorkerRestarting) || !strings.HasSuffix(err.Error(), ": detect timed out after 1s") {
			t.Fatalf("Detect %d = %v", calls, err)
		}
		break
	}
	if elapsed := time.Since(start); elapsed < 900*time.Millisecond || elapsed > 6*time.Second {
		t.Fatalf("worker killed after %s, want ≈1s", elapsed)
	}
	deadline := time.Now().Add(15 * time.Second)
	for {
		_, err := detectFrame(detector)
		if err == nil {
			break
		}
		if !restartPending(err) || time.Now().After(deadline) {
			t.Fatalf("Detect after restart = %v", err)
		}
		time.Sleep(10 * time.Millisecond)
	}
	if f.calls("serve-cpu") != 2 {
		t.Fatalf("worker starts = %d, want 2", f.calls("serve-cpu"))
	}
	if err := detector.Close(); err != nil {
		t.Fatal(err)
	}
	waitForGoroutines(t, baseline)
}

func TestDetectorRestartsGPUWorkerWithoutCPUTimeRule(t *testing.T) {
	// The restarted GPU worker measures 90 % of the CPU path's CPU time (CPU
	// contention): it still passes its self-test and is used.
	f := newDetectorFixture(t, "gpu_cpu_ms=10,cpu_cpu_ms=100,gpu_cpu_ms_restart=90,crash_first_detect", BackendGPU)
	detector := f.start(t)
	if info := detector.Info(); info.Active != BackendGPU {
		t.Fatalf("info = %+v", info)
	}
	if _, err := detectFrame(detector); !errors.Is(err, ErrWorkerRestarting) {
		t.Fatalf("Detect = %v, want a worker failure", err)
	}
	deadline := time.Now().Add(15 * time.Second)
	for {
		result, err := detectFrame(detector)
		if err == nil {
			if len(result.Boxes) != 1 {
				t.Fatalf("result = %+v", result)
			}
			break
		}
		if !restartPending(err) || time.Now().After(deadline) {
			t.Fatalf("Detect after restart = %v", err)
		}
		time.Sleep(10 * time.Millisecond)
	}
	if f.calls("serve-webgpu") != 2 || f.calls("serve-cpu") != 0 || detector.Info().Active != BackendGPU {
		t.Fatalf("webgpu starts %d, cpu starts %d, info %+v", f.calls("serve-webgpu"), f.calls("serve-cpu"), detector.Info())
	}
}

func TestDetectorBoundsWorkerLogInErrors(t *testing.T) {
	f := newDetectorFixture(t, "stderr_flood,crash_first_detect", BackendCPU)
	f.deps.backoffBase, f.deps.backoffMax = 3*time.Second, 3*time.Second
	detector := f.start(t)
	_, err := detectFrame(detector)
	if !errors.Is(err, ErrWorkerRestarting) {
		t.Fatalf("Detect = %v", err)
	}
	message := err.Error()
	_, line, found := strings.Cut(message, "; last log line: ")
	if !found || !strings.HasPrefix(line, "last line xxx") || len(line) > maxWorkerErrorBytes+len("…") || strings.Contains(message, "flood line") {
		t.Fatalf("error has %d bytes: %q", len(message), message)
	}
	// The slog warning carries the whole tail.
	if logs := f.logs.String(); !strings.Contains(logs, "worker_log=") || !strings.Contains(logs, "flood line 0") || !strings.Contains(logs, "flood line 99") {
		t.Fatalf("logs:\n%s", logs)
	}
}

func TestStartServeWorkerTimesOut(t *testing.T) {
	f := newDetectorFixture(t, "stderr_flood,hello_hang", BackendCPU)
	launch := workerLaunch{executable: f.opts.Executable, provider: "cpu", args: []string{"serve", "--provider", "cpu"}}
	start := time.Now()
	worker, _, err := startServeWorker(context.Background(), launch, 500*time.Millisecond, nil)
	if err == nil {
		worker.stop()
		t.Fatal("a worker without hello was accepted")
	}
	if elapsed := time.Since(start); elapsed > 10*time.Second {
		t.Fatalf("timeout took %s", elapsed)
	}
	if !strings.HasPrefix(err.Error(), "worker was not ready within 500ms; last log line: last line xxx") || len(err.Error()) > 600 {
		t.Fatalf("error = %q", err)
	}
	if log := workerLog(err); !strings.Contains(log, "flood line 99") {
		t.Fatalf("worker log = %q", log)
	}
}

func TestRunPrepareTimesOut(t *testing.T) {
	f := newDetectorFixture(t, "prepare_hang", BackendGPU)
	start := time.Now()
	_, err := runPrepare(context.Background(), f.opts.Executable, f.dir, filepath.Join(f.dir, "in.onnx"), filepath.Join(f.dir, "out.onnx"), 500*time.Millisecond)
	if err == nil || err.Error() != "prepare did not finish within 500ms" {
		t.Fatalf("runPrepare = %v", err)
	}
	if elapsed := time.Since(start); elapsed > 10*time.Second {
		t.Fatalf("timeout took %s", elapsed)
	}
	if f.calls("prepare") != 1 {
		t.Fatalf("prepare calls = %d", f.calls("prepare"))
	}
}

func TestDetectorConcurrentDetectInfoClose(t *testing.T) {
	f := newDetectorFixture(t, "echo_id,crash_second_detect", BackendCPU)
	f.deps.backoffBase = 10 * time.Millisecond
	detector := f.start(t)
	var wg sync.WaitGroup
	stop := make(chan struct{})
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
				_, _ = detector.Detect(ctx, []byte("frame"), 0.5)
				cancel()
				_ = detector.Info()
			}
		}()
	}
	time.Sleep(1500 * time.Millisecond)
	closed := make(chan struct{})
	go func() {
		_ = detector.Close()
		close(stop)
		wg.Wait()
		close(closed)
	}()
	select {
	case <-closed:
	case <-time.After(20 * time.Second):
		t.Fatal("Close with concurrent Detect calls hung")
	}
	if _, err := detectFrame(detector); !errors.Is(err, ErrDetectorClosed) {
		t.Fatalf("Detect after Close = %v", err)
	}
}
