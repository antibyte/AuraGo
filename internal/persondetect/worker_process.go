package persondetect

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
	"time"

	"aurago/internal/onnxrt"
)

const (
	// workerLogBytes is how much of a worker's stderr (plus stray non-JSON
	// stdout) is kept. stderr is diagnostics only: ONNX Runtime warnings such
	// as "Some nodes were not assigned to the preferred execution providers"
	// are never an error by themselves. The whole tail goes to the slog
	// warnings only (workerLog); error texts and fallback reasons carry the
	// last line, cut to maxWorkerErrorBytes.
	workerLogBytes = 4 << 10
	// maxWorkerErrorBytes bounds worker-provided text (the last log line, a
	// hello, panic or prepare error) inside error messages.
	maxWorkerErrorBytes = 400
	// maxWorkerLineBytes bounds one stdout line of a worker; longer lines are
	// skipped (a result holds at most 300 boxes, a few KiB).
	maxWorkerLineBytes = 1 << 20
	// workerStopWait bounds the wait for a killed worker to exit.
	workerStopWait = 5 * time.Second
	// workerCloseWait is how long shutdown lets a worker answer "close".
	workerCloseWait = 2 * time.Second
	// workerWaitDelay bounds the wait for a worker's output pipes after it
	// exited (exec.Cmd.WaitDelay).
	workerWaitDelay = 2 * time.Second
)

// workerLaunch is everything needed to (re)start a serve worker.
type workerLaunch struct {
	executable  string
	runtimeRoot string
	provider    string   // cpu | webgpu
	args        []string // after --person-detect-worker
}

// workerProcess is one running worker subprocess.
type workerProcess struct {
	cmd      *exec.Cmd
	stdin    io.WriteCloser
	writeMu  sync.Mutex // keeps request lines whole
	messages chan workerMessage
	done     chan struct{} // closed after the process exited and stdout is drained
	quit     chan struct{} // closed by stop; the reader then drops messages
	log      *tailBuffer
	waitErr  error // valid once done is closed
	stopOnce sync.Once
}

func startWorkerProcess(launch workerLaunch) (*workerProcess, error) {
	command := exec.Command(launch.executable, append([]string{workerFlag}, launch.args...)...)
	onnxrt.ConfigureWorkerCommand(command, launch.runtimeRoot)
	command.WaitDelay = workerWaitDelay
	stdin, err := command.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("open worker stdin: %w", err)
	}
	stdout, err := command.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		return nil, fmt.Errorf("open worker stdout: %w", err)
	}
	log := newTailBuffer(workerLogBytes)
	command.Stderr = log
	if err := command.Start(); err != nil {
		_ = stdin.Close()
		return nil, fmt.Errorf("start worker: %w", err)
	}
	worker := &workerProcess{
		cmd:      command,
		stdin:    stdin,
		messages: make(chan workerMessage, 8),
		done:     make(chan struct{}),
		quit:     make(chan struct{}),
		log:      log,
	}
	go worker.read(stdout)
	return worker, nil
}

// read forwards JSON lines until stdout ends, then waits for the process.
// Non-JSON lines (a native library printing to stdout) are not messages and
// only go to the log tail; over-long lines are skipped.
func (w *workerProcess) read(stdout io.Reader) {
	reader := bufio.NewReaderSize(stdout, 64<<10)
	for {
		line, overlong, err := readWorkerLine(reader, maxWorkerLineBytes)
		if overlong {
			fmt.Fprintf(w.log, "[skipped a stdout line of more than %d bytes]\n", maxWorkerLineBytes)
		} else if trimmed := bytes.TrimSpace(line); len(trimmed) > 0 {
			var message workerMessage
			if trimmed[0] == '{' && json.Unmarshal(trimmed, &message) == nil {
				select {
				case w.messages <- message:
				case <-w.quit:
				}
			} else {
				_, _ = w.log.Write(append(trimmed, '\n'))
			}
		}
		if err != nil {
			break
		}
	}
	w.waitErr = w.cmd.Wait()
	close(w.done)
}

// readWorkerLine returns the next stdout line. A line longer than limit is
// consumed up to its newline and reported as overlong, without its content.
func readWorkerLine(reader *bufio.Reader, limit int) (line []byte, overlong bool, err error) {
	for {
		chunk, err := reader.ReadSlice('\n')
		if !overlong {
			if len(line)+len(chunk) > limit {
				line, overlong = nil, true
			} else {
				line = append(line, chunk...)
			}
		}
		if !errors.Is(err, bufio.ErrBufferFull) {
			return line, overlong, err
		}
	}
}

// requestWrite is one request line being written to a worker.
type requestWrite struct {
	done chan struct{} // closed when the write returned
	err  error         // valid once done is closed
}

func (r *requestWrite) finished() bool {
	select {
	case <-r.done:
		return true
	default:
		return false
	}
}

// sendAsync writes one request line in the background. Lines never
// interleave. A write blocks while the worker does not read (busy with an
// earlier frame, or hung), so callers bound the wait with their own timer and
// stop the worker when it expires; the kill then fails the pending write.
func (w *workerProcess) sendAsync(request workerRequest) *requestWrite {
	write := &requestWrite{done: make(chan struct{})}
	line, err := json.Marshal(request)
	if err != nil {
		write.err = err
		close(write.done)
		return write
	}
	line = append(line, '\n')
	go func() {
		w.writeMu.Lock()
		defer w.writeMu.Unlock()
		_, write.err = w.stdin.Write(line)
		close(write.done)
	}()
	return write
}

// buffered returns an already received message accepted by want, without
// blocking; other buffered messages are discarded.
func (w *workerProcess) buffered(want func(workerMessage) bool) (workerMessage, bool) {
	for {
		select {
		case message := <-w.messages:
			if want(message) {
				return message, true
			}
		default:
			return workerMessage{}, false
		}
	}
}

// isWorkerError reports the id-less {"type":"error","error":…} line RunWorker
// prints when it recovers a Go panic; the worker exits right after it, so it
// is fatal whenever it arrives.
func isWorkerError(message workerMessage) bool {
	return message.Type == "error"
}

func workerError(message workerMessage) error {
	if message.Error == "" {
		return errors.New("worker failed")
	}
	return fmt.Errorf("worker failed: %s", truncateText(message.Error, maxWorkerErrorBytes))
}

func isHelloOrError(message workerMessage) bool {
	return message.Type == "hello" || isWorkerError(message)
}

func helloResult(message workerMessage) (workerMessage, error) {
	if isWorkerError(message) {
		return message, workerError(message)
	}
	if message.Error != "" {
		return message, errors.New(truncateText(message.Error, maxWorkerErrorBytes))
	}
	return message, nil
}

// awaitHello waits for the hello line (the worker runs its self-test first;
// the first WebGPU run compiles shaders, so timeout is the start-up timeout,
// never the per-frame one).
func (w *workerProcess) awaitHello(ctx context.Context, timeout time.Duration) (workerMessage, error) {
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	for {
		select {
		case message := <-w.messages:
			if isHelloOrError(message) {
				return helloResult(message)
			}
		case <-w.done:
			if message, ok := w.buffered(isHelloOrError); ok {
				return helloResult(message)
			}
			return workerMessage{}, w.exitError("worker exited before it was ready")
		case <-timer.C:
			return workerMessage{}, withLastLogLine(fmt.Sprintf("worker was not ready within %s", timeout), w.log.String())
		case <-ctx.Done():
			return workerMessage{}, ctx.Err()
		}
	}
}

// exitStatus is how the worker ended ("exit status 0" for a clean exit);
// only valid once done is closed.
func (w *workerProcess) exitStatus() string {
	if w.waitErr == nil {
		return "exit status 0"
	}
	return w.waitErr.Error()
}

// exitError is "<what> (<exit status>); last log line: …"; only valid once
// done is closed.
func (w *workerProcess) exitError(what string) error {
	return withLastLogLine(fmt.Sprintf("%s (%s)", what, w.exitStatus()), w.log.String())
}

// stop kills the worker and waits up to workerStopWait for it to exit. The
// kill comes before closing stdin: closing a pipe with a pending write can
// wait for that write, which only fails once the worker is gone.
func (w *workerProcess) stop() {
	w.stopOnce.Do(func() {
		close(w.quit)
		if w.cmd.Process != nil {
			_ = w.cmd.Process.Kill()
		}
		_ = w.stdin.Close()
	})
	select {
	case <-w.done:
	case <-time.After(workerStopWait):
	}
}

// shutdown asks the worker to exit and stops it after workerCloseWait at the
// latest.
func (w *workerProcess) shutdown() {
	deadline := time.NewTimer(workerCloseWait)
	defer deadline.Stop()
	write := w.sendAsync(workerRequest{Command: "close"})
	select {
	case <-write.done:
		if write.err == nil {
			select {
			case <-w.done:
			case <-deadline.C:
			}
		}
	case <-w.done:
	case <-deadline.C:
	}
	w.stop()
}

// startServeWorker starts a serve worker and waits for its hello. On error
// the worker has been stopped and the error carries its log tail
// (workerLog).
func startServeWorker(ctx context.Context, launch workerLaunch, timeout time.Duration, started func()) (*workerProcess, workerMessage, error) {
	worker, err := startWorkerProcess(launch)
	if err != nil {
		return nil, workerMessage{}, err
	}
	if started != nil {
		started()
	}
	hello, err := worker.awaitHello(ctx, timeout)
	if err != nil {
		worker.stop()
		return nil, workerMessage{}, withWorkerLog(err, worker.log.String())
	}
	return worker, hello, nil
}

// runPrepare runs `prepare` in a worker subprocess and returns the number of
// patched nodes. It returns only after the subprocess has exited, also when
// ctx is cancelled or the timeout expires (the process is killed then).
// Errors carry the subprocess's log tail (workerLog).
func runPrepare(ctx context.Context, executable, runtimeRoot, input, output string, timeout time.Duration) (int, error) {
	prepareCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	command := exec.CommandContext(prepareCtx, executable, workerFlag, "prepare", "--runtime", runtimeRoot, "--in", input, "--out", output)
	onnxrt.ConfigureWorkerCommand(command, runtimeRoot)
	command.WaitDelay = workerWaitDelay
	stdout := newTailBuffer(64 << 10)
	stderr := newTailBuffer(workerLogBytes)
	command.Stdout = stdout
	command.Stderr = stderr
	runErr := command.Run()
	log := stderr.String()
	if message, ok := lastJSONLine(stdout.Bytes()); ok {
		if runErr == nil && message.Type == "prepared" && message.PatchedNodes > 0 {
			return message.PatchedNodes, nil
		}
		if message.Error != "" {
			return 0, withWorkerLog(errors.New(truncateText(message.Error, maxWorkerErrorBytes)), log)
		}
	}
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	if prepareCtx.Err() != nil {
		return 0, withLastLogLine(fmt.Sprintf("prepare did not finish within %s", timeout), log)
	}
	status := "exit status 0, no result"
	if runErr != nil {
		status = runErr.Error()
	}
	return 0, withLastLogLine(fmt.Sprintf("prepare worker failed (%s)", status), log)
}

func lastJSONLine(output []byte) (workerMessage, bool) {
	lines := bytes.Split(output, []byte("\n"))
	for i := len(lines) - 1; i >= 0; i-- {
		line := bytes.TrimSpace(lines[i])
		if len(line) == 0 || line[0] != '{' {
			continue
		}
		var message workerMessage
		if json.Unmarshal(line, &message) == nil {
			return message, true
		}
	}
	return workerMessage{}, false
}

// workerLogError carries a worker's log tail next to a bounded error.
type workerLogError struct {
	err error
	log string
}

func (e *workerLogError) Error() string { return e.err.Error() }
func (e *workerLogError) Unwrap() error { return e.err }

func withWorkerLog(err error, log string) error {
	if err == nil || log == "" {
		return err
	}
	return &workerLogError{err: err, log: log}
}

// workerLog is the log tail carried by err (or "").
func workerLog(err error) string {
	var logged *workerLogError
	if errors.As(err, &logged) {
		return logged.log
	}
	return ""
}

// withLastLogLine is message plus "; last log line: …" (when log has one),
// carrying the whole log for workerLog.
func withLastLogLine(message, log string) error {
	if line := lastLogLine(log); line != "" {
		message += "; last log line: " + line
	}
	return withWorkerLog(errors.New(message), log)
}

// lastLogLine is the last non-empty line of log, cut to maxWorkerErrorBytes.
func lastLogLine(log string) string {
	log = strings.TrimSpace(log)
	if index := strings.LastIndexByte(log, '\n'); index >= 0 {
		log = strings.TrimSpace(log[index+1:])
	}
	return truncateText(log, maxWorkerErrorBytes)
}

// tailBuffer keeps the last limit bytes written to it.
type tailBuffer struct {
	mu    sync.Mutex
	limit int
	data  []byte
}

func newTailBuffer(limit int) *tailBuffer {
	return &tailBuffer{limit: limit}
}

func (b *tailBuffer) Write(data []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.data = append(b.data, data...)
	if len(b.data) > b.limit {
		b.data = append([]byte(nil), b.data[len(b.data)-b.limit:]...)
	}
	return len(data), nil
}

// Bytes returns a copy of the kept bytes.
func (b *tailBuffer) Bytes() []byte {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]byte(nil), b.data...)
}

func (b *tailBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return strings.TrimSpace(string(b.data))
}
