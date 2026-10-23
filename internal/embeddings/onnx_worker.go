package embeddings

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"aurago/internal/onnxrt"
)

type workerRequest struct {
	ID      uint64   `json:"id"`
	Command string   `json:"command"`
	Texts   []string `json:"texts,omitempty"`
}

type workerResponse struct {
	ID              uint64      `json:"id,omitempty"`
	Type            string      `json:"type"`
	Vectors         [][]float32 `json:"vectors,omitempty"`
	Error           string      `json:"error,omitempty"`
	Version         string      `json:"version,omitempty"`
	Backend         string      `json:"backend,omitempty"`
	Providers       []string    `json:"providers,omitempty"`
	BackendVerified bool        `json:"backend_verified"`
}

// RunONNXWorker runs the isolated native inference mode. It must be dispatched
// before normal AuraGo initialization so native runtime failures cannot affect
// the server process.
func RunONNXWorker(args []string, input io.Reader, output io.Writer, errorOutput io.Writer) int {
	flags := flag.NewFlagSet("embedding-worker", flag.ContinueOnError)
	flags.SetOutput(errorOutput)
	runtimeRoot := flags.String("runtime", "", "ONNX Runtime directory")
	modelFile := flags.String("model", "", "ONNX model path")
	tokenizerFile := flags.String("tokenizer", "", "tokenizer.json path")
	backend := flags.String("backend", "cpu", "execution backend")
	contextSize := flags.Int("context-size", 2048, "maximum token context")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if *runtimeRoot == "" || *modelFile == "" || *tokenizerFile == "" {
		fmt.Fprintln(errorOutput, "runtime, model, and tokenizer are required")
		return 2
	}

	tokenizer, err := loadGraniteTokenizer(*tokenizerFile)
	if err != nil {
		writeWorkerResponse(output, workerResponse{Type: "hello", Error: err.Error()})
		return 1
	}
	session, err := newORTSession(*runtimeRoot, *modelFile, strings.ToLower(*backend))
	if err != nil {
		writeWorkerResponse(output, workerResponse{Type: "hello", Error: err.Error()})
		return 1
	}
	defer session.Close()
	if err := writeWorkerResponse(output, workerResponse{
		Type:            "hello",
		Version:         session.version,
		Backend:         session.backend,
		Providers:       session.providers,
		BackendVerified: session.verified,
	}); err != nil {
		fmt.Fprintf(errorOutput, "write worker hello: %v\n", err)
		return 1
	}

	decoder := json.NewDecoder(bufio.NewReader(input))
	encoder := json.NewEncoder(output)
	for {
		var request workerRequest
		if err := decoder.Decode(&request); err != nil {
			if err != io.EOF {
				fmt.Fprintf(errorOutput, "decode worker request: %v\n", err)
			}
			return 0
		}
		response := workerResponse{ID: request.ID, Type: "result"}
		switch request.Command {
		case "embed":
			batch, err := tokenizer.encodeBatch(request.Texts, *contextSize)
			if err == nil {
				response.Vectors, err = session.Embed(batch)
				response.BackendVerified = session.verified
			}
			if err != nil {
				response.Error = err.Error()
			}
		case "close":
			_ = encoder.Encode(response)
			return 0
		default:
			response.Error = "unsupported worker command"
		}
		if err := encoder.Encode(response); err != nil {
			fmt.Fprintf(errorOutput, "write worker response: %v\n", err)
			return 1
		}
	}
}

func writeWorkerResponse(output io.Writer, response workerResponse) error {
	return json.NewEncoder(output).Encode(response)
}

const (
	// onnxStartupTimeout bounds one worker start, including the model load.
	onnxStartupTimeout = 10 * time.Minute
	// onnxDrainTimeout bounds how long an abandoned request may keep the
	// worker busy before the process is considered hung and killed.
	onnxDrainTimeout = 2 * time.Minute
	// onnxCloseWaitTimeout bounds how long Close waits for a request in flight
	// (or a drain) before it stops the process anyway.
	onnxCloseWaitTimeout = 30 * time.Second
)

var errONNXWorkerClosed = errors.New("ONNX worker is closed")

// onnxWorkerConn is one live worker process as the embedder sees it. Tests
// replace the process with in-memory pipes and a fake kill.
type onnxWorkerConn struct {
	stdin   io.Writer
	decoder *json.Decoder
	stderr  *limitedBuffer
	hello   workerResponse
	// kill ends the process and reaps it. stop makes it run only once.
	kill     func() error
	stopOnce sync.Once
	stopErr  error
}

func (conn *onnxWorkerConn) stop() error {
	conn.stopOnce.Do(func() {
		if conn.kill != nil {
			conn.stopErr = conn.kill()
		}
	})
	return conn.stopErr
}

func (conn *onnxWorkerConn) log() string {
	if conn.stderr == nil {
		return ""
	}
	return conn.stderr.String()
}

// onnxWorkerEmbedder talks to an isolated worker process. One request is in
// flight at a time: slot is a one-token semaphore, so waiting for it can honor
// the caller's context (a mutex could not). stateMu only guards the fields
// below and is never held across I/O, so Status never waits for an inference.
//
// A caller whose context ends after the request was sent gets its error
// immediately, but the worker is not killed: a drain goroutine keeps the slot
// until the late response has been read and discarded. The process is only
// replaced after a protocol or I/O error, a crash, or a drain timeout, and
// only until Close is called.
type onnxWorkerEmbedder struct {
	options      onnxWorkerOptions
	start        func(context.Context, onnxWorkerOptions) (*onnxWorkerConn, error)
	respawns     *restartBudget
	drainTimeout time.Duration
	closeWait    time.Duration
	logger       *slog.Logger
	slot         chan struct{}
	nextID       atomic.Uint64
	fingerprint  string

	stateMu         sync.Mutex
	conn            *onnxWorkerConn // nil after the process died; Embed starts a new one
	backend         string
	version         string
	providers       []string
	backendVerified bool
	closed          bool // set by Close only; final
}

type onnxWorkerOptions struct {
	RuntimeRoot   string
	ModelPath     string
	TokenizerPath string
	Backend       string
	ContextSize   int
	// Logger receives worker supervision warnings; nil means slog.Default().
	Logger *slog.Logger
}

func newONNXWorkerEmbedder(ctx context.Context, options onnxWorkerOptions) (*onnxWorkerEmbedder, error) {
	return newONNXWorkerEmbedderWith(
		ctx,
		options,
		startONNXWorkerConn,
		newRestartBudget(maxProcessRestarts, processRestartWindow),
	)
}

func newONNXWorkerEmbedderWith(
	ctx context.Context,
	options onnxWorkerOptions,
	start func(context.Context, onnxWorkerOptions) (*onnxWorkerConn, error),
	respawns *restartBudget,
) (*onnxWorkerEmbedder, error) {
	conn, err := start(ctx, options)
	if err != nil {
		return nil, err
	}
	logger := options.Logger
	if logger == nil {
		logger = slog.Default()
	}
	worker := &onnxWorkerEmbedder{
		logger:       logger,
		options:      options,
		start:        start,
		respawns:     respawns,
		drainTimeout: onnxDrainTimeout,
		closeWait:    onnxCloseWaitTimeout,
		slot:         make(chan struct{}, 1),
		backend:      options.Backend,
		fingerprint:  onnxFingerprint(),
	}
	worker.adoptLocked(conn)
	return worker, nil
}

// startONNXWorkerConn launches the worker process and waits for its hello.
func startONNXWorkerConn(ctx context.Context, options onnxWorkerOptions) (*onnxWorkerConn, error) {
	executable, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("resolve AuraGo executable: %w", err)
	}
	command := exec.Command(
		executable,
		"--embedding-worker",
		"--runtime", options.RuntimeRoot,
		"--model", options.ModelPath,
		"--tokenizer", options.TokenizerPath,
		"--backend", options.Backend,
		"--context-size", fmt.Sprintf("%d", options.ContextSize),
	)
	onnxrt.ConfigureWorkerCommand(command, options.RuntimeRoot)
	stdin, err := command.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("open ONNX worker stdin: %w", err)
	}
	stdout, err := command.StdoutPipe()
	if err != nil {
		stdin.Close()
		return nil, fmt.Errorf("open ONNX worker stdout: %w", err)
	}
	stderr := newLimitedBuffer(32 << 10)
	command.Stderr = stderr
	if err := command.Start(); err != nil {
		stdin.Close()
		return nil, fmt.Errorf("start ONNX worker: %w", err)
	}

	conn := &onnxWorkerConn{
		stdin:   stdin,
		decoder: json.NewDecoder(bufio.NewReader(stdout)),
		stderr:  stderr,
		kill: func() error {
			_ = stdin.Close()
			if command.Process == nil {
				return nil
			}
			_ = command.Process.Kill()
			err := command.Wait()
			if err != nil && !strings.Contains(strings.ToLower(err.Error()), "killed") {
				return fmt.Errorf("wait for ONNX worker: %w", err)
			}
			return nil
		},
	}
	hello, err := decodeWorkerResponse(ctx, conn.decoder)
	if err != nil {
		_ = conn.stop()
		return nil, fmt.Errorf("start ONNX worker: %w; log: %s", err, stderr.String())
	}
	if hello.Type != "hello" {
		_ = conn.stop()
		return nil, fmt.Errorf("ONNX worker returned %q instead of hello", hello.Type)
	}
	if hello.Error != "" {
		_ = conn.stop()
		return nil, fmt.Errorf("ONNX worker initialization failed: %s", hello.Error)
	}
	conn.hello = hello
	return conn, nil
}

// decodeWorkerResponse reads one response without outliving ctx. On ctx expiry
// the reader goroutine is left to end when the caller kills the process.
func decodeWorkerResponse(ctx context.Context, decoder *json.Decoder) (workerResponse, error) {
	exchange := make(chan workerExchange, 1)
	go func() {
		var response workerResponse
		err := decoder.Decode(&response)
		exchange <- workerExchange{response: response, err: err}
	}()
	select {
	case <-ctx.Done():
		return workerResponse{}, ctx.Err()
	case result := <-exchange:
		return result.response, result.err
	}
}

// workerExchange is the outcome of one request/response round trip.
type workerExchange struct {
	response workerResponse
	err      error
}

func (worker *onnxWorkerEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, fmt.Errorf("%w: at least one text is required", ErrInvalidInput)
	}
	if worker.isClosed() {
		return nil, errONNXWorkerClosed
	}
	select {
	case worker.slot <- struct{}{}:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	handedToDrain := false
	defer func() {
		if !handedToDrain {
			<-worker.slot
		}
	}()
	// The slot and a done context can win the select above together; a
	// request must never be sent for a caller that is already gone.
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	conn, err := worker.connection(ctx)
	if err != nil {
		return nil, err
	}

	request := workerRequest{ID: worker.nextID.Add(1), Command: "embed", Texts: texts}
	result := worker.exchange(conn, request)
	select {
	case exchange := <-result:
		return worker.finish(conn, request, exchange)
	case <-ctx.Done():
		// The request is out. Keep the slot until its response has been read
		// so the next request cannot read it by mistake, and keep the process.
		handedToDrain = true
		go worker.drain(conn, request.ID, result)
		return nil, ctx.Err()
	}
}

// exchange sends request and reads its response on its own goroutine, so a
// stuck pipe write or read cannot hold the caller past its context.
func (worker *onnxWorkerEmbedder) exchange(conn *onnxWorkerConn, request workerRequest) <-chan workerExchange {
	result := make(chan workerExchange, 1)
	go func() {
		if err := json.NewEncoder(conn.stdin).Encode(request); err != nil {
			result <- workerExchange{err: fmt.Errorf("send ONNX worker request: %w", err)}
			return
		}
		var response workerResponse
		if err := conn.decoder.Decode(&response); err != nil {
			result <- workerExchange{err: fmt.Errorf("read ONNX worker response: %w; log: %s", err, conn.log())}
			return
		}
		result <- workerExchange{response: response}
	}()
	return result
}

// finish turns a completed round trip into vectors. Protocol and I/O errors
// drop the process (the next request starts a new one); errors the worker
// reports itself leave it running.
func (worker *onnxWorkerEmbedder) finish(conn *onnxWorkerConn, request workerRequest, exchange workerExchange) ([][]float32, error) {
	if exchange.err != nil {
		worker.dropConn(conn)
		return nil, exchange.err
	}
	response := exchange.response
	if response.ID != request.ID {
		worker.dropConn(conn)
		return nil, fmt.Errorf("ONNX worker response ID %d, want %d", response.ID, request.ID)
	}
	if response.Error != "" {
		return nil, fmt.Errorf("%s", response.Error)
	}
	worker.stateMu.Lock()
	worker.backendVerified = response.BackendVerified
	worker.stateMu.Unlock()
	if len(response.Vectors) != len(request.Texts) {
		return nil, fmt.Errorf("ONNX worker returned %d vectors for %d texts", len(response.Vectors), len(request.Texts))
	}
	for i := range response.Vectors {
		if err := validateGraniteVector(response.Vectors[i]); err != nil {
			return nil, fmt.Errorf("validate ONNX vector %d: %w", i, err)
		}
	}
	return response.Vectors, nil
}

// drain owns the slot of an abandoned request: it waits for the response
// (bounded by drainTimeout), discards it and frees the slot. A broken response
// or a worker that stays silent past the timeout gets the process killed, with
// one warning: no caller is left to receive the error.
func (worker *onnxWorkerEmbedder) drain(conn *onnxWorkerConn, requestID uint64, result <-chan workerExchange) {
	defer func() { <-worker.slot }()
	timer := time.NewTimer(worker.drainTimeout)
	defer timer.Stop()
	select {
	case exchange := <-result:
		switch {
		case exchange.err != nil:
			worker.dropAfterDrain(conn, requestID, exchange.err.Error())
		case exchange.response.ID != requestID:
			worker.dropAfterDrain(conn, requestID, fmt.Sprintf("response ID %d, want %d", exchange.response.ID, requestID))
		}
	case <-timer.C:
		worker.dropAfterDrain(conn, requestID, fmt.Sprintf("no response within %s", worker.drainTimeout))
	}
}

// dropAfterDrain logs why a drained worker is dropped, then drops it. After
// Close the process is stopped on purpose, so that failure is not worth a warning.
func (worker *onnxWorkerEmbedder) dropAfterDrain(conn *onnxWorkerConn, requestID uint64, reason string) {
	if !worker.isClosed() {
		worker.logger.Warn("Abandoned ONNX worker request ended badly; dropping the worker process",
			"request_id", requestID,
			"reason", reason,
			"log", conn.log())
	}
	worker.dropConn(conn)
}

// connection returns the live process, starting a new one when the previous
// died on its own and the restart budget allows it. Caller holds the slot.
func (worker *onnxWorkerEmbedder) connection(ctx context.Context) (*onnxWorkerConn, error) {
	worker.stateMu.Lock()
	closed, conn := worker.closed, worker.conn
	worker.stateMu.Unlock()
	if closed {
		return nil, errONNXWorkerClosed
	}
	if conn != nil {
		return conn, nil
	}
	if !worker.respawns.take() {
		return nil, fmt.Errorf("ONNX worker restart limit reached (%d per %s)", maxProcessRestarts, processRestartWindow)
	}
	startCtx, cancel := context.WithTimeout(ctx, onnxStartupTimeout)
	defer cancel()
	fresh, err := worker.start(startCtx, worker.options)
	if err != nil {
		if isCallerError(ctx, err) {
			// Cut short by the caller; says nothing about the worker.
			worker.respawns.release()
			return nil, err
		}
		return nil, fmt.Errorf("restart ONNX worker: %w", err)
	}
	worker.stateMu.Lock()
	if worker.closed {
		worker.stateMu.Unlock()
		_ = fresh.stop()
		return nil, errONNXWorkerClosed
	}
	worker.adoptLocked(fresh)
	worker.stateMu.Unlock()
	return fresh, nil
}

// adoptLocked makes conn the live process. Callers other than the constructor
// hold stateMu.
func (worker *onnxWorkerEmbedder) adoptLocked(conn *onnxWorkerConn) {
	worker.conn = conn
	worker.version = conn.hello.Version
	worker.providers = conn.hello.Providers
	worker.backendVerified = conn.hello.BackendVerified
}

// dropConn kills a process that can no longer be trusted. The embedder keeps
// working: the next request starts a new process within the restart budget.
func (worker *onnxWorkerEmbedder) dropConn(conn *onnxWorkerConn) {
	worker.stateMu.Lock()
	if worker.conn == conn {
		worker.conn = nil
	}
	worker.stateMu.Unlock()
	_ = conn.stop()
}

func (worker *onnxWorkerEmbedder) isClosed() bool {
	worker.stateMu.Lock()
	defer worker.stateMu.Unlock()
	return worker.closed
}

func (worker *onnxWorkerEmbedder) Dimensions() int {
	return GraniteDimensions
}

func (worker *onnxWorkerEmbedder) ModelID() string {
	return GraniteModelID
}

func (worker *onnxWorkerEmbedder) Fingerprint() string {
	return worker.fingerprint
}

func (worker *onnxWorkerEmbedder) Status() Status {
	worker.stateMu.Lock()
	defer worker.stateMu.Unlock()
	state := "ready"
	switch {
	case worker.closed:
		state = "closed"
	case worker.conn == nil && worker.respawns.remaining() == 0:
		// The process is gone and the next Embed may not start another one.
		state = "unavailable"
	}
	return Status{
		State:        state,
		Provider:     LocalGraniteProvider,
		ModelID:      GraniteModelID,
		Dimensions:   GraniteDimensions,
		Backend:      worker.backend,
		Runtime:      "onnxruntime",
		RuntimeBuild: onnxRuntimeVersion,
		GPU:          worker.backend != "cpu",
		GPUVerified:  worker.backendVerified,
		Fingerprint:  worker.fingerprint,
		UpdatedAt:    time.Now().UTC(),
	}
}

// Close ends the worker for good; unlike a crash it is never followed by a
// restart. It admits no new request, then waits (bounded by closeWait) for the
// request in flight or the drain in progress, so a backend swap does not fail
// the callers still embedding on the old one. After the wait, or when it runs
// out, the process is stopped anyway. Safe to call more than once.
func (worker *onnxWorkerEmbedder) Close() error {
	worker.stateMu.Lock()
	worker.closed = true
	worker.stateMu.Unlock()

	timer := time.NewTimer(worker.closeWait)
	defer timer.Stop()
	select {
	case worker.slot <- struct{}{}:
		// Owning the slot means no request is running when the process stops.
		defer func() { <-worker.slot }()
	case <-timer.C:
	}

	worker.stateMu.Lock()
	conn := worker.conn
	worker.conn = nil
	worker.stateMu.Unlock()
	if conn == nil {
		return nil
	}
	return conn.stop()
}

type limitedBuffer struct {
	mu    sync.Mutex
	limit int
	data  []byte
}

func newLimitedBuffer(limit int) *limitedBuffer {
	return &limitedBuffer{limit: limit}
}

func (buffer *limitedBuffer) Write(data []byte) (int, error) {
	buffer.mu.Lock()
	defer buffer.mu.Unlock()
	buffer.data = append(buffer.data, data...)
	if len(buffer.data) > buffer.limit {
		buffer.data = append([]byte(nil), buffer.data[len(buffer.data)-buffer.limit:]...)
	}
	return len(data), nil
}

func (buffer *limitedBuffer) String() string {
	buffer.mu.Lock()
	defer buffer.mu.Unlock()
	return strings.TrimSpace(string(buffer.data))
}
