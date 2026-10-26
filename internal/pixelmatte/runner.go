package pixelmatte

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"aurago/internal/onnxrt"
)

const (
	// maxMatteBytes bounds the PNG a worker may write: a 32 Mi pixel 8-bit
	// matte is 32 MiB raw, and PNG overhead stays well below this.
	maxMatteBytes = 48 << 20
	// stderrTailBytes is how much of a worker's stderr is kept.
	stderrTailBytes = 4 << 10
	// maxErrorBytes bounds worker text inside an error message.
	maxErrorBytes = 300
	// workerWaitDelay bounds how long Wait waits for the output pipes after
	// the process ended or was killed.
	workerWaitDelay = 2 * time.Second
)

// RunRequest is one matting job for a Runner.
type RunRequest struct {
	Executable  string // the AuraGo binary that hosts the worker
	RuntimeRoot string // ONNX Runtime directory
	ModelPath   string
	Kind        Kind
	Threads     int
	Image       []byte // PNG or JPEG
}

// Runner runs one matting job and returns the matte PNG. Production uses the
// worker subprocess; tests replace it.
type Runner interface {
	Run(ctx context.Context, request RunRequest) ([]byte, error)
}

// processRunner starts `<executable> --pixel-matte-worker run ...` for every
// job. The process is killed when ctx ends.
type processRunner struct{}

func (processRunner) Run(parent context.Context, request RunRequest) ([]byte, error) {
	if request.Executable == "" {
		return nil, errors.New("no worker executable")
	}
	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	command := exec.CommandContext(ctx, request.Executable, WorkerFlag, "run",
		"--runtime", request.RuntimeRoot,
		"--model", request.ModelPath,
		"--kind", string(request.Kind),
		"--threads", strconv.Itoa(request.Threads))
	onnxrt.ConfigureWorkerCommand(command, request.RuntimeRoot)
	command.WaitDelay = workerWaitDelay
	command.Stdin = bytes.NewReader(request.Image)
	stdout := &cappedBuffer{limit: maxMatteBytes, onOverflow: cancel}
	stderr := &tailBuffer{limit: stderrTailBytes}
	command.Stdout = stdout
	command.Stderr = stderr
	runErr := command.Run()
	if stdout.overflowed() {
		return nil, fmt.Errorf("worker output exceeds %d bytes", maxMatteBytes)
	}
	if runErr != nil {
		// The caller's context ended and killed the worker: that is the
		// caller's to report (cancelled, or timed out).
		if err := parent.Err(); err != nil {
			return nil, err
		}
		message := lastLine(stderr.String())
		if message == "" {
			message = runErr.Error()
		}
		return nil, fmt.Errorf("pixel matte worker failed (%v): %s", runErr, message)
	}
	if stdout.Len() == 0 {
		return nil, errors.New("pixel matte worker wrote no matte")
	}
	return stdout.Bytes(), nil
}

// lastLine is the last non-empty line of a worker's stderr, as valid UTF-8 cut
// to maxErrorBytes.
func lastLine(log string) string {
	log = strings.TrimSpace(strings.ToValidUTF8(log, ""))
	if index := strings.LastIndexByte(log, '\n'); index >= 0 {
		log = strings.TrimSpace(log[index+1:])
	}
	if len(log) <= maxErrorBytes {
		return log
	}
	cut := maxErrorBytes
	for cut > 0 && !utf8.RuneStart(log[cut]) {
		cut--
	}
	return log[:cut] + "..."
}

// cappedBuffer collects a worker's stdout up to limit bytes; the first byte
// beyond calls onOverflow (which kills the worker) and fails the write.
type cappedBuffer struct {
	mu         sync.Mutex
	data       bytes.Buffer
	limit      int
	over       bool
	onOverflow func()
}

func (b *cappedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.over || b.data.Len()+len(p) > b.limit {
		if !b.over {
			b.over = true
			if b.onOverflow != nil {
				b.onOverflow()
			}
		}
		return 0, errors.New("worker output limit exceeded")
	}
	return b.data.Write(p)
}

func (b *cappedBuffer) Len() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.data.Len()
}

func (b *cappedBuffer) Bytes() []byte {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.data.Bytes()
}

func (b *cappedBuffer) overflowed() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.over
}

// tailBuffer keeps the last limit bytes written to it.
type tailBuffer struct {
	mu    sync.Mutex
	limit int
	data  []byte
}

func (b *tailBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.data = append(b.data, p...)
	if len(b.data) > b.limit {
		b.data = append([]byte(nil), b.data[len(b.data)-b.limit:]...)
	}
	return len(p), nil
}

func (b *tailBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return string(b.data)
}
