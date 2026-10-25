package embeddings

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"aurago/internal/onnxrt"
)

type llamaEmbedder struct {
	mu           sync.Mutex
	runtimeRoot  string
	libraryRoots []string
	serverPath   string
	modelPath    string
	backend      string
	contextSize  int
	batchSize    int
	apiKey       string
	baseURL      string
	command      *exec.Cmd
	stderr       *limitedBuffer
	client       *http.Client
	restarts     *restartBudget
	// hangs counts the requests that ended at the caller's deadline; mu guards it.
	hangs llamaHangState
	// lifecycle is what Close and Status reach without the lock; see llamaLifecycle.
	lifecycle llamaLifecycle
	// logger is set by the manager right after construction; nil means slog.Default().
	logger *slog.Logger
	closed bool
}

func newLlamaEmbedder(
	ctx context.Context,
	runtimeRoot string,
	modelPath string,
	backend string,
	contextSize int,
	batchSize int,
	supplementalRuntimeRoots ...string,
) (*llamaEmbedder, error) {
	serverPath, err := findLlamaServer(runtimeRoot)
	if err != nil {
		return nil, err
	}
	apiKey, err := randomAPIKey()
	if err != nil {
		return nil, err
	}
	embedder := &llamaEmbedder{
		runtimeRoot:  runtimeRoot,
		libraryRoots: append([]string{runtimeRoot}, supplementalRuntimeRoots...),
		serverPath:   serverPath,
		modelPath:    modelPath,
		backend:      backend,
		contextSize:  contextSize,
		batchSize:    batchSize,
		apiKey:       apiKey,
		client:       &http.Client{Timeout: 2 * time.Minute},
		restarts:     newRestartBudget(maxProcessRestarts, processRestartWindow),
	}
	if err := embedder.startLocked(ctx); err != nil {
		return nil, err
	}
	return embedder, nil
}

func (embedder *llamaEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, fmt.Errorf("%w: at least one text is required", ErrInvalidInput)
	}
	if err := lockContext(ctx, &embedder.mu); err != nil {
		return nil, err
	}
	// A restart that outlives this caller keeps running and takes the lock with
	// it (hold below); it frees the lock when the start is over.
	lockHandedOver := false
	defer func() {
		if !lockHandedOver {
			embedder.mu.Unlock()
		}
	}()
	if embedder.closed {
		return nil, fmt.Errorf("llama.cpp embedder is closed")
	}
	if embedder.restarts == nil {
		embedder.restarts = newRestartBudget(maxProcessRestarts, processRestartWindow)
	}
	vectors, err := embedWithRestarts(ctx, embedder.restarts, llamaServerOps{
		name: "llama-server",
		running: func() bool {
			return embedder.command != nil && embedder.command.Process != nil
		},
		start: embedder.startLocked,
		embed: embedder.embedLocked,
		stop:  func() { _ = embedder.stopLocked() },
		hangs: &embedder.hangs,
		hold: func(finished <-chan struct{}) {
			lockHandedOver = true
			go func() {
				<-finished
				embedder.mu.Unlock()
			}()
		},
		logger:    embedder.logger,
		lifecycle: &embedder.lifecycle,
	}, texts)
	if err != nil {
		return nil, err
	}
	for i := range vectors {
		if err := l2Normalize(vectors[i]); err != nil {
			return nil, fmt.Errorf("normalize llama.cpp vector %d: %w", i, err)
		}
		if err := validateGraniteVector(vectors[i]); err != nil {
			return nil, fmt.Errorf("validate llama.cpp vector %d: %w", i, err)
		}
	}
	return vectors, nil
}

func (embedder *llamaEmbedder) embedLocked(ctx context.Context, texts []string) ([][]float32, error) {
	payload := struct {
		Model          string   `json:"model"`
		Input          []string `json:"input"`
		EncodingFormat string   `json:"encoding_format"`
	}{
		Model:          GraniteModelID,
		Input:          texts,
		EncodingFormat: "float",
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal llama.cpp request: %w", err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, embedder.baseURL+"/v1/embeddings", bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("create llama.cpp request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+embedder.apiKey)
	response, err := embedder.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("call llama-server: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		var body bytes.Buffer
		_, _ = body.ReadFrom(io.LimitReader(response.Body, 16<<10))
		return nil, fmt.Errorf("llama-server returned %s: %s", response.Status, strings.TrimSpace(body.String()))
	}
	var decoded struct {
		Data []struct {
			Embedding []float32 `json:"embedding"`
			Index     int       `json:"index"`
		} `json:"data"`
	}
	if err := json.NewDecoder(response.Body).Decode(&decoded); err != nil {
		return nil, fmt.Errorf("decode llama.cpp response: %w", err)
	}
	if len(decoded.Data) != len(texts) {
		return nil, fmt.Errorf("llama-server returned %d vectors for %d texts", len(decoded.Data), len(texts))
	}
	sort.Slice(decoded.Data, func(i, j int) bool {
		return decoded.Data[i].Index < decoded.Data[j].Index
	})
	vectors := make([][]float32, len(decoded.Data))
	for i := range decoded.Data {
		vectors[i] = decoded.Data[i].Embedding
	}
	return vectors, nil
}

func (embedder *llamaEmbedder) startLocked(ctx context.Context) error {
	// Close may have ended the start before the process exists.
	if err := ctx.Err(); err != nil {
		return err
	}
	port, err := availableLoopbackPort()
	if err != nil {
		return err
	}
	batchSize := embedder.batchSize
	if batchSize <= 0 {
		batchSize = 2048
	}
	contextSize := embedder.contextSize
	if contextSize <= 0 {
		contextSize = 2048
	}
	gpuLayers := "0"
	if embedder.backend != "cpu" {
		gpuLayers = "999"
	}
	args := []string{
		"--model", embedder.modelPath,
		"--alias", GraniteModelID,
		"--host", "127.0.0.1",
		"--port", strconv.Itoa(port),
		"--api-key", embedder.apiKey,
		"--embedding",
		"--pooling", "cls",
		"--embd-normalize", "2",
		"--ctx-size", strconv.Itoa(contextSize),
		"--batch-size", strconv.Itoa(batchSize),
		"--ubatch-size", strconv.Itoa(batchSize),
		"--parallel", "1",
		"--n-gpu-layers", gpuLayers,
	}
	command := exec.Command(embedder.serverPath, args...)
	onnxrt.ConfigureWorkerCommand(command, embedder.libraryRoots...)
	stderr := newLimitedBuffer(64 << 10)
	command.Stdout = stderr
	command.Stderr = stderr
	if err := command.Start(); err != nil {
		return fmt.Errorf("start llama-server: %w", err)
	}
	embedder.command = command
	embedder.stderr = stderr
	embedder.baseURL = fmt.Sprintf("http://127.0.0.1:%d", port)
	if err := embedder.waitForHealthLocked(ctx); err != nil {
		_ = embedder.stopLocked()
		return fmt.Errorf("wait for llama-server: %w; log: %s", err, stderr.String())
	}
	return nil
}

func (embedder *llamaEmbedder) waitForHealthLocked(parent context.Context) error {
	ctx, cancel := context.WithTimeout(parent, 2*time.Minute)
	defer cancel()
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for {
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, embedder.baseURL+"/health", nil)
		if err != nil {
			return err
		}
		response, err := embedder.client.Do(request)
		if err == nil {
			response.Body.Close()
			if response.StatusCode == http.StatusOK {
				return nil
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func (embedder *llamaEmbedder) Dimensions() int {
	return GraniteDimensions
}

func (embedder *llamaEmbedder) ModelID() string {
	return GraniteModelID
}

func (embedder *llamaEmbedder) Fingerprint() string {
	return ggufFingerprint()
}

// Status never waits for the embedder lock: a start can hold it for minutes
// and the config UI polls this. While it is taken the last status seen is
// returned, reported as restarting during a start.
func (embedder *llamaEmbedder) Status() Status {
	if !embedder.mu.TryLock() {
		return embedder.lifecycle.busy(embedder.statusWith("ready", false))
	}
	defer embedder.mu.Unlock()
	state := "ready"
	if embedder.closed {
		state = "closed"
	}
	verified := embedder.backend == "cpu" || llamaGPUVerified(embedder.backend, embedder.stderr.String())
	status := embedder.statusWith(state, embedder.backend != "cpu" && verified)
	embedder.lifecycle.remember(status)
	return status
}

// statusWith fills in what is fixed at construction, so it needs no lock.
func (embedder *llamaEmbedder) statusWith(state string, gpuVerified bool) Status {
	return Status{
		State:        state,
		Provider:     LocalGraniteProvider,
		ModelID:      GraniteModelID,
		Dimensions:   GraniteDimensions,
		Backend:      embedder.backend,
		Runtime:      "llama.cpp",
		RuntimeBuild: llamaRuntimeBuild,
		GPU:          embedder.backend != "cpu",
		GPUVerified:  gpuVerified,
		Fingerprint:  ggufFingerprint(),
		UpdatedAt:    time.Now().UTC(),
	}
}

func (embedder *llamaEmbedder) Close() error {
	// Before the lock: a start in flight holds it, and would keep Close waiting
	// for a health check that can run for two minutes.
	embedder.lifecycle.shutdown()
	embedder.mu.Lock()
	defer embedder.mu.Unlock()
	if embedder.closed {
		return nil
	}
	embedder.closed = true
	return embedder.stopLocked()
}

func (embedder *llamaEmbedder) stopLocked() error {
	if embedder.command == nil || embedder.command.Process == nil {
		embedder.command = nil
		return nil
	}
	_ = embedder.command.Process.Kill()
	err := embedder.command.Wait()
	embedder.command = nil
	if err != nil && !strings.Contains(strings.ToLower(err.Error()), "killed") {
		return fmt.Errorf("wait for llama-server: %w", err)
	}
	return nil
}

func findLlamaServer(root string) (string, error) {
	expected := "llama-server"
	if filepath.Separator == '\\' {
		expected += ".exe"
	}
	var result string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !entry.IsDir() && strings.EqualFold(entry.Name(), expected) {
			result = path
			return filepath.SkipAll
		}
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("scan llama.cpp runtime: %w", err)
	}
	if result == "" {
		return "", fmt.Errorf("%s not found below %s", expected, root)
	}
	return result, nil
}

func llamaGPUVerified(backend, logs string) bool {
	lower := strings.ToLower(logs)
	backendEvidence := strings.Contains(lower, strings.ToLower(backend))
	if backend == "metal" {
		backendEvidence = strings.Contains(lower, "metal")
	}
	offloadEvidence := strings.Contains(lower, "offloaded") ||
		strings.Contains(lower, "using device") ||
		strings.Contains(lower, "gpu layers")
	return backendEvidence && offloadEvidence
}

func availableLoopbackPort() (int, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, fmt.Errorf("reserve llama-server loopback port: %w", err)
	}
	defer listener.Close()
	address, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		return 0, fmt.Errorf("loopback listener did not return a TCP address")
	}
	return address.Port, nil
}

func randomAPIKey() (string, error) {
	data := make([]byte, 32)
	if _, err := rand.Read(data); err != nil {
		return "", fmt.Errorf("generate llama-server API key: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(data), nil
}
