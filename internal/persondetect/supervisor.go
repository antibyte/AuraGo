package persondetect

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// workerDetector supervises one serve worker.
type workerDetector struct {
	logger *slog.Logger
	launch workerLaunch
	deps   detectorDeps
	info   DetectorInfo
	ctx    context.Context // cancelled by Close
	cancel context.CancelFunc

	mu       sync.Mutex
	worker   *workerProcess // nil while restarting or failed
	nextID   uint64
	failures int   // consecutive worker failures
	failed   error // set after maxFailures consecutive failures
	closed   bool
	timer    *time.Timer
	restarts sync.WaitGroup
}

func (d *workerDetector) Info() DetectorInfo {
	info := d.info
	info.Providers = append([]string(nil), d.info.Providers...)
	return info
}

// Detect sends one frame to the worker and waits for its result. Frames are
// analyzed one at a time. The detect timeout covers sending the frame and the
// answer; when it expires the worker is killed. A cancelled ctx abandons the
// frame without counting a failure (the worker still answers it, and that
// answer is skipped).
func (d *workerDetector) Detect(ctx context.Context, jpegData []byte, minScore float64) (DetectResult, error) {
	if len(jpegData) == 0 {
		return DetectResult{}, fmt.Errorf("frame is empty")
	}
	if len(jpegData) > maxFrameBytes {
		return DetectResult{}, fmt.Errorf("frame has %d bytes, limit %d", len(jpegData), maxFrameBytes)
	}
	if !(minScore > 0 && minScore <= 1) {
		return DetectResult{}, fmt.Errorf("min score must be in (0, 1]")
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.closed {
		return DetectResult{}, ErrDetectorClosed
	}
	if d.failed != nil {
		return DetectResult{}, d.failed
	}
	worker := d.worker
	if worker == nil {
		return DetectResult{}, ErrWorkerRestarting
	}
	d.nextID++
	id := d.nextID
	timer := time.NewTimer(d.deps.detectTimeout)
	defer timer.Stop()
	sent := worker.sendAsync(workerRequest{ID: id, Command: "detect", JPEG: jpegData, MinScore: minScore})
	answer := func(message workerMessage) bool {
		return isWorkerError(message) || (message.Type == "result" && message.ID == id)
	}
	for {
		select {
		case err := <-sent:
			sent = nil
			if err != nil {
				return DetectResult{}, d.failLocked(fmt.Errorf("send frame: %w", err))
			}
		case message := <-worker.messages:
			if answer(message) {
				return d.finishLocked(message)
			}
		case <-worker.done:
			if message, ok := worker.buffered(answer); ok {
				return d.finishLocked(message)
			}
			return DetectResult{}, d.failLocked(fmt.Errorf("worker exited: %s", worker.exitSummary()))
		case <-timer.C:
			return DetectResult{}, d.failLocked(fmt.Errorf("detect timed out after %s", d.deps.detectTimeout))
		case <-ctx.Done():
			return DetectResult{}, ctx.Err()
		case <-d.ctx.Done():
			return DetectResult{}, ErrDetectorClosed
		}
	}
}

// finishLocked turns the worker's answer into a result. A frame-level error
// (an undecodable JPEG, for example) is returned without counting a worker
// failure; an id-less error line is a worker failure.
func (d *workerDetector) finishLocked(message workerMessage) (DetectResult, error) {
	if isWorkerError(message) {
		return DetectResult{}, d.failLocked(workerError(message))
	}
	if message.Error != "" {
		return DetectResult{}, fmt.Errorf("detect: %s", message.Error)
	}
	d.failures = 0
	boxes := message.Boxes
	if boxes == nil {
		boxes = []Box{}
	}
	return DetectResult{Boxes: boxes, Width: message.Width, Height: message.Height, WallMS: message.WallMS, CPUMS: message.CPUMS}, nil
}

// failLocked stops the current worker, counts a consecutive failure and
// either schedules a restart with backoff or marks the detector failed.
func (d *workerDetector) failLocked(cause error) error {
	if d.worker != nil {
		d.worker.stop()
		d.worker = nil
	}
	d.failures++
	d.logger.Warn("Person detection worker failed", "backend", d.info.Active, "failures", d.failures, "error", cause)
	if d.failures >= d.deps.maxFailures {
		d.failed = fmt.Errorf("%w after %d consecutive failures: %v", ErrDetectorFailed, d.failures, cause)
		return d.failed
	}
	delay := backoffDelay(d.deps.backoffBase, d.deps.backoffMax, d.failures)
	d.timer = time.AfterFunc(delay, d.restart)
	return fmt.Errorf("person detection worker failed (restart in %s): %w", delay, cause)
}

// backoffDelay is base·2^(failures−1), capped at limit.
func backoffDelay(base, limit time.Duration, failures int) time.Duration {
	delay := base
	for i := 1; i < failures && delay < limit; i++ {
		delay *= 2
	}
	if delay > limit {
		delay = limit
	}
	return delay
}

func (d *workerDetector) restart() {
	d.mu.Lock()
	if d.closed || d.failed != nil || d.worker != nil {
		d.mu.Unlock()
		return
	}
	d.restarts.Add(1)
	d.mu.Unlock()
	defer d.restarts.Done()

	worker, hello, err := startServeWorker(d.ctx, d.launch, d.deps.startTimeout, nil)
	if err == nil {
		if err = acceptHello(d.launch.provider, hello); err != nil {
			worker.stop()
			worker = nil
		}
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.closed {
		if worker != nil {
			worker.stop()
		}
		return
	}
	if err != nil {
		_ = d.failLocked(fmt.Errorf("restart: %w", err))
		return
	}
	d.worker = worker
	d.logger.Info("Person detection worker restarted", "backend", d.info.Active)
}

// Close stops the worker; afterwards every Detect returns ErrDetectorClosed.
func (d *workerDetector) Close() error {
	d.cancel()
	d.mu.Lock()
	if d.closed {
		d.mu.Unlock()
		return nil
	}
	d.closed = true
	if d.timer != nil {
		d.timer.Stop()
	}
	worker := d.worker
	d.worker = nil
	d.mu.Unlock()
	d.restarts.Wait()
	if worker != nil {
		worker.shutdown()
	}
	return nil
}
