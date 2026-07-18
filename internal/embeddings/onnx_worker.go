package embeddings

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"aurago/internal/sandbox"
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

type onnxWorkerEmbedder struct {
	mu              sync.Mutex
	command         *exec.Cmd
	stdin           io.WriteCloser
	decoder         *json.Decoder
	stderr          *limitedBuffer
	nextID          atomic.Uint64
	backend         string
	version         string
	providers       []string
	backendVerified bool
	fingerprint     string
	closed          bool
}

type onnxWorkerOptions struct {
	RuntimeRoot   string
	ModelPath     string
	TokenizerPath string
	Backend       string
	ContextSize   int
}

func newONNXWorkerEmbedder(ctx context.Context, options onnxWorkerOptions) (*onnxWorkerEmbedder, error) {
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
	configureHiddenProcess(command)
	command.Env = runtimeEnvironment(options.RuntimeRoot)
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

	worker := &onnxWorkerEmbedder{
		command:     command,
		stdin:       stdin,
		decoder:     json.NewDecoder(bufio.NewReader(stdout)),
		stderr:      stderr,
		backend:     options.Backend,
		fingerprint: onnxFingerprint(),
	}
	hello, err := worker.decodeWithContext(ctx)
	if err != nil {
		_ = worker.forceClose()
		return nil, fmt.Errorf("start ONNX worker: %w; log: %s", err, stderr.String())
	}
	if hello.Type != "hello" {
		_ = worker.forceClose()
		return nil, fmt.Errorf("ONNX worker returned %q instead of hello", hello.Type)
	}
	if hello.Error != "" {
		_ = worker.forceClose()
		return nil, fmt.Errorf("ONNX worker initialization failed: %s", hello.Error)
	}
	worker.version = hello.Version
	worker.providers = hello.Providers
	worker.backendVerified = hello.BackendVerified
	return worker, nil
}

func (worker *onnxWorkerEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, fmt.Errorf("at least one text is required")
	}
	worker.mu.Lock()
	defer worker.mu.Unlock()
	if worker.closed || worker.command == nil || worker.command.Process == nil {
		return nil, fmt.Errorf("ONNX worker is closed")
	}
	request := workerRequest{ID: worker.nextID.Add(1), Command: "embed", Texts: texts}
	if err := json.NewEncoder(worker.stdin).Encode(request); err != nil {
		_ = worker.forceCloseLocked()
		return nil, fmt.Errorf("send ONNX worker request: %w", err)
	}
	response, err := worker.decodeWithContext(ctx)
	if err != nil {
		_ = worker.forceCloseLocked()
		return nil, fmt.Errorf("read ONNX worker response: %w; log: %s", err, worker.stderr.String())
	}
	if response.ID != request.ID {
		_ = worker.forceCloseLocked()
		return nil, fmt.Errorf("ONNX worker response ID %d, want %d", response.ID, request.ID)
	}
	if response.Error != "" {
		return nil, fmt.Errorf("%s", response.Error)
	}
	worker.backendVerified = response.BackendVerified
	if len(response.Vectors) != len(texts) {
		return nil, fmt.Errorf("ONNX worker returned %d vectors for %d texts", len(response.Vectors), len(texts))
	}
	for i := range response.Vectors {
		if err := validateGraniteVector(response.Vectors[i]); err != nil {
			return nil, fmt.Errorf("validate ONNX vector %d: %w", i, err)
		}
	}
	return response.Vectors, nil
}

func (worker *onnxWorkerEmbedder) decodeWithContext(ctx context.Context) (workerResponse, error) {
	type decodeResult struct {
		response workerResponse
		err      error
	}
	resultChannel := make(chan decodeResult, 1)
	go func() {
		var response workerResponse
		err := worker.decoder.Decode(&response)
		resultChannel <- decodeResult{response: response, err: err}
	}()
	select {
	case <-ctx.Done():
		return workerResponse{}, ctx.Err()
	case result := <-resultChannel:
		return result.response, result.err
	}
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
	worker.mu.Lock()
	defer worker.mu.Unlock()
	state := "ready"
	if worker.closed {
		state = "closed"
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

func (worker *onnxWorkerEmbedder) Close() error {
	worker.mu.Lock()
	defer worker.mu.Unlock()
	return worker.forceCloseLocked()
}

func (worker *onnxWorkerEmbedder) forceClose() error {
	worker.mu.Lock()
	defer worker.mu.Unlock()
	return worker.forceCloseLocked()
}

func (worker *onnxWorkerEmbedder) forceCloseLocked() error {
	if worker.closed {
		return nil
	}
	worker.closed = true
	if worker.stdin != nil {
		_ = worker.stdin.Close()
	}
	if worker.command == nil || worker.command.Process == nil {
		return nil
	}
	_ = worker.command.Process.Kill()
	err := worker.command.Wait()
	if err != nil && !strings.Contains(strings.ToLower(err.Error()), "killed") {
		return fmt.Errorf("wait for ONNX worker: %w", err)
	}
	return nil
}

func runtimeEnvironment(runtimeRoots ...string) []string {
	environment := sandbox.FilterEnv(os.Environ())
	var directories []string
	for _, runtimeRoot := range runtimeRoots {
		_ = filepath.WalkDir(runtimeRoot, func(path string, entry os.DirEntry, err error) error {
			if err == nil && entry.IsDir() {
				directories = append(directories, path)
			}
			return nil
		})
	}
	prefix := strings.Join(directories, string(os.PathListSeparator))
	keys := []string{"PATH"}
	switch goruntimeGOOS() {
	case "linux":
		keys = append(keys, "LD_LIBRARY_PATH")
	case "darwin":
		keys = append(keys, "DYLD_LIBRARY_PATH")
	}
	for _, key := range keys {
		current := environmentValue(environment, key)
		value := prefix
		if current != "" {
			value += string(os.PathListSeparator) + current
		}
		environment = setEnvironmentValue(environment, key, value)
	}
	return environment
}

func goruntimeGOOS() string {
	return runtimeGOOS
}

var runtimeGOOS = func() string {
	// Assigned through a small variable to keep runtimeEnvironment directly
	// testable without mutating the process environment.
	return runtime.GOOS
}()

func environmentValue(environment []string, key string) string {
	prefix := strings.ToUpper(key) + "="
	for _, entry := range environment {
		if strings.HasPrefix(strings.ToUpper(entry), prefix) {
			return entry[len(prefix):]
		}
	}
	return ""
}

func setEnvironmentValue(environment []string, key, value string) []string {
	prefix := strings.ToUpper(key) + "="
	for index, entry := range environment {
		if strings.HasPrefix(strings.ToUpper(entry), prefix) {
			environment[index] = key + "=" + value
			return environment
		}
	}
	return append(environment, key+"="+value)
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
