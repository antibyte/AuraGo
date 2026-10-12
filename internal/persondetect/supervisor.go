package persondetect

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// workerDetector supervises one serve worker.
//
// Frames are analyzed one at a time. Every request the worker has not
// answered yet is tracked with its send time: the oldest one bounds the
// worker's time budget (deps.detectTimeout), also across Detect calls whose
// caller gave up early, so a hung worker is killed once its oldest request is
// overdue, whatever the callers' contexts. A request line is only written
// after the previous write returned, so at most one write is pending (lines
// may still queue in the pipe; the budget bounds that).
type workerDetector struct {
	logger *slog.Logger
	launch workerLaunch
	deps   detectorDeps
	info   DetectorInfo
	ctx    context.Context // cancelled by Close
	cancel context.CancelFunc

	mu        sync.Mutex
	worker    *workerProcess // nil while restarting or failed
	nextID    uint64
	pending   []pendingRequest // unanswered requests to worker, oldest first
	lastWrite *requestWrite    // the newest request line written to worker
	failures  int              // consecutive worker failures
	failed    error            // set after maxFailures consecutive failures
	closed    bool
	timer     *time.Timer
	restarts  sync.WaitGroup
	closeOnce sync.Once
}

// pendingRequest is a request the current worker has not answered yet.
type pendingRequest struct {
	id   uint64
	sent time.Time
}

func (d *workerDetector) Info() DetectorInfo {
	info := d.info
	info.Providers = append([]string(nil), d.info.Providers...)
	return info
}

// Detect sends one frame to the worker and waits for its result.
//
// Errors: ErrDetectorClosed after Close; an error wrapping ErrDetectorFailed
// once maxFailures consecutive worker failures happened; ErrWorkerRestarting
// while a restart is pending, and an error wrapping ErrWorkerRestarting and
// the cause for the worker failure that triggered it (a crash, a timeout, an
// error line); ctx.Err() when the caller gives up (no failure is counted;
// the worker still answers the frame later, and that answer is skipped); any
// other error is about this frame only (an undecodable JPEG, for example).
// A frame is not sent when ctx is already done.
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
	if err := ctx.Err(); err != nil {
		return DetectResult{}, err
	}
	if err := d.collectLocked(worker); err != nil {
		return DetectResult{}, err
	}
	if len(d.pending) > 0 && !time.Now().Before(d.deadlineLocked()) {
		return DetectResult{}, d.failLocked(d.timeoutError())
	}
	timer := time.NewTimer(time.Until(d.deadlineLocked()))
	defer timer.Stop()

	previous := d.lastWrite
	var (
		id        uint64
		write     *requestWrite // this frame's request line, nil until sent
		writeDone <-chan struct{}
	)
	for {
		var previousDone <-chan struct{}
		if write == nil {
			if previous != nil && !previous.finished() {
				// The previous write has not returned yet.
				previousDone = previous.done
			} else {
				if previous != nil && previous.err != nil {
					return DetectResult{}, d.failLocked(d.writeError(worker, previous.err))
				}
				if err := ctx.Err(); err != nil {
					return DetectResult{}, err
				}
				d.nextID++
				id = d.nextID
				d.pending = append(d.pending, pendingRequest{id: id, sent: time.Now()})
				write = worker.sendAsync(workerRequest{ID: id, Command: "detect", JPEG: jpegData, MinScore: minScore})
				writeDone = write.done
				d.lastWrite = write
				timer.Reset(time.Until(d.deadlineLocked()))
			}
		}
		select {
		case <-previousDone:
		case <-writeDone:
			writeDone = nil
			if write.err != nil {
				return DetectResult{}, d.failLocked(d.writeError(worker, write.err))
			}
		case message := <-worker.messages:
			if isWorkerError(message) {
				return DetectResult{}, d.failLocked(workerError(message))
			}
			if message.Type != "result" {
				continue
			}
			if write != nil && message.ID == id {
				return d.finishAnswerLocked(worker, message)
			}
			d.answeredLocked(message.ID)
			// An answer to an abandoned frame: the budget now runs from the
			// oldest request still unanswered.
			timer.Reset(time.Until(d.deadlineLocked()))
		case <-worker.done:
			// Lines printed before the exit are all queued by now.
			if message, ok := worker.buffered(func(message workerMessage) bool {
				return isWorkerError(message) || (write != nil && message.Type == "result" && message.ID == id)
			}); ok {
				if isWorkerError(message) {
					return DetectResult{}, d.failLocked(workerError(message))
				}
				return d.finishAnswerLocked(worker, message)
			}
			return DetectResult{}, d.failLocked(d.exitCause(worker))
		case <-timer.C:
			return DetectResult{}, d.failLocked(d.timeoutError())
		case <-ctx.Done():
			return DetectResult{}, ctx.Err()
		case <-d.ctx.Done():
			return DetectResult{}, ErrDetectorClosed
		}
	}
}

// collectLocked takes the answers worker printed while no Detect was waiting
// (to frames whose callers gave up) and fails the worker when it printed an
// error line or exited. A worker that exited with every frame answered is
// reported in the log only: the calling Detect, which has sent nothing, gets
// the plain ErrWorkerRestarting.
func (d *workerDetector) collectLocked(worker *workerProcess) error {
	for {
		select {
		case message := <-worker.messages:
			if isWorkerError(message) {
				return d.failLocked(workerError(message))
			}
			if message.Type == "result" {
				d.answeredLocked(message.ID)
			}
			continue
		default:
		}
		select {
		case <-worker.done:
			// The reader queues every line before it closes done.
			if len(worker.messages) > 0 {
				continue
			}
			idle := len(d.pending) == 0
			err := d.failLocked(d.exitCause(worker))
			if idle && errors.Is(err, ErrWorkerRestarting) {
				return ErrWorkerRestarting
			}
			return err
		default:
			return nil
		}
	}
}

// answeredLocked drops the requests up to id: the worker answers in order.
func (d *workerDetector) answeredLocked(id uint64) {
	kept := d.pending[:0]
	for _, request := range d.pending {
		if request.id > id {
			kept = append(kept, request)
		}
	}
	d.pending = kept
}

// deadlineLocked is when the oldest unanswered request (or a request sent
// now) exceeds the detect timeout.
func (d *workerDetector) deadlineLocked() time.Time {
	if len(d.pending) == 0 {
		return time.Now().Add(d.deps.detectTimeout)
	}
	return d.pending[0].sent.Add(d.deps.detectTimeout)
}

func (d *workerDetector) timeoutError() error {
	return fmt.Errorf("detect timed out after %s", d.deps.detectTimeout)
}

// exitCause describes a worker that exited; only valid once done is closed.
func (d *workerDetector) exitCause(worker *workerProcess) error {
	return worker.exitError("worker exited unexpectedly")
}

// writeError describes a failed request write. The write fails because the
// worker went away, so its exit is reported when it follows within a second
// (Close does not wait for that).
func (d *workerDetector) writeError(worker *workerProcess, err error) error {
	select {
	case <-worker.done:
		return d.exitCause(worker)
	case <-time.After(time.Second):
	case <-d.ctx.Done():
	}
	return fmt.Errorf("send frame: %w", err)
}

// finishAnswerLocked returns the answer to the calling Detect's frame. A
// worker that has exited already is failed right away (logged, counted, its
// restart scheduled), so the next Detect does not find it dead.
func (d *workerDetector) finishAnswerLocked(worker *workerProcess, message workerMessage) (DetectResult, error) {
	d.answeredLocked(message.ID)
	result, err := d.finishLocked(message)
	select {
	case <-worker.done:
		_ = d.failLocked(d.exitCause(worker))
	default:
	}
	return result, err
}

// finishLocked turns the worker's answer into a result. A frame-level error
// (an undecodable JPEG, for example) is returned without counting a worker
// failure.
func (d *workerDetector) finishLocked(message workerMessage) (DetectResult, error) {
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
// either schedules a restart with backoff (the error wraps
// ErrWorkerRestarting and cause) or marks the detector failed (the error
// wraps ErrDetectorFailed and cause). The slog warning carries the worker's
// whole log tail; the error only its last line. While Close runs, nothing is
// counted or logged.
func (d *workerDetector) failLocked(cause error) error {
	log := workerLog(cause)
	if d.worker != nil {
		d.worker.stop()
		if log == "" {
			log = d.worker.log.String()
		}
		d.worker = nil
	}
	d.pending, d.lastWrite = nil, nil
	if d.ctx.Err() != nil {
		return ErrDetectorClosed
	}
	d.failures++
	args := []any{"backend", d.info.Active, "failures", d.failures, "error", cause}
	if log != "" {
		args = append(args, "worker_log", log)
	}
	d.logger.Warn("Person detection worker failed", args...)
	if d.failures >= d.deps.maxFailures {
		d.failed = fmt.Errorf("%w after %d consecutive failures: %w", ErrDetectorFailed, d.failures, cause)
		return d.failed
	}
	delay := backoffDelay(d.deps.backoffBase, d.deps.backoffMax, d.failures)
	d.timer = time.AfterFunc(delay, d.restart)
	return fmt.Errorf("%w in %s: %w", ErrWorkerRestarting, delay, cause)
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

// restart starts a new worker with the same launch. A restarted worker only
// needs a passed self-test (acceptSelfTest), also on the GPU path.
func (d *workerDetector) restart() {
	d.mu.Lock()
	if d.closed || d.failed != nil || d.worker != nil || d.ctx.Err() != nil {
		d.mu.Unlock()
		return
	}
	d.restarts.Add(1)
	d.mu.Unlock()
	defer d.restarts.Done()

	worker, hello, err := startServeWorker(d.ctx, d.launch, d.deps.startTimeout, nil)
	if err == nil {
		if err = acceptSelfTest(hello); err != nil {
			worker.stop()
			worker = nil
		}
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.closed || d.ctx.Err() != nil {
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
	d.pending, d.lastWrite = nil, nil
	d.logger.Info("Person detection worker restarted", "backend", d.info.Active)
}

// Close stops the worker; afterwards every Detect returns ErrDetectorClosed.
// Every Close call, also concurrent ones, returns only after the worker has
// been stopped.
func (d *workerDetector) Close() error {
	d.cancel()
	d.closeOnce.Do(func() {
		d.mu.Lock()
		d.closed = true
		if d.timer != nil {
			d.timer.Stop()
		}
		worker := d.worker
		d.worker, d.pending, d.lastWrite = nil, nil, nil
		d.mu.Unlock()
		d.restarts.Wait()
		if worker != nil {
			worker.shutdown()
		}
	})
	return nil
}
