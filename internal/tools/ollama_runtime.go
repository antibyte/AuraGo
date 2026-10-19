package tools

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"aurago/internal/config"
	"aurago/internal/dockerutil"
	"aurago/internal/security"
)

// OllamaOperation is a bounded, process-local receipt. After restart an absent
// receipt is unknown; it is never evidence of successful completion.
type OllamaOperation struct {
	ID        string            `json:"id"`
	State     string            `json:"state"`
	Model     string            `json:"model,omitempty"`
	Models    map[string]string `json:"models,omitempty"`
	Container string            `json:"container,omitempty"`
	Error     string            `json:"error,omitempty"`
	Updated   time.Time         `json:"updated"`
}

type ollamaWork struct {
	OllamaOperation
	key       string
	revision  string
	ctx       context.Context
	done      chan struct{}
	apiReady  chan struct{}
	readyOnce sync.Once
}

type ollamaUnknownOutcome struct{ err error }

func (e *ollamaUnknownOutcome) Error() string { return e.err.Error() }
func (e *ollamaUnknownOutcome) Unwrap() error { return e.err }

// Caller holds the runtime mutex; receipt maps must not alias worker state.
func ollamaReceipt(job *ollamaWork) OllamaOperation {
	receipt := job.OllamaOperation
	if job.Models != nil {
		receipt.Models = map[string]string{}
		for model, state := range job.Models {
			receipt.Models[model] = state
		}
	}
	return receipt
}

type ollamaRuntime struct {
	mu           sync.Mutex
	ctx          context.Context
	cancel       context.CancelFunc
	revision     string
	stopped      bool
	cfg          *config.Config
	sequence     uint64
	jobs         map[string]*ollamaWork
	active       map[string]*ollamaWork
	pendingStops map[string]bool
	locks        sync.Map
	wg           sync.WaitGroup
}

var ollamaRuntimeWork = &ollamaRuntime{jobs: map[string]*ollamaWork{}, active: map[string]*ollamaWork{}}
var ollamaReceiptPrefix = rand.Text()[:12]

func ollamaConfigRevision(cfg *config.Config) string {
	b, _ := json.Marshal([]any{cfg.Ollama, cfg.Embeddings.LocalOllama, cfg.Docker, cfg.Runtime.IsDocker})
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

// ConfigureOllamaRuntime publishes the admission fence before config side effects.
// Unrelated saves preserve work; changed targets or grants cancel obsolete jobs.
func ConfigureOllamaRuntime(owner context.Context, cfg *config.Config) {
	w := ollamaRuntimeWork
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.stopped {
		return
	}
	rev := ollamaConfigRevision(cfg)
	if w.revision != rev || w.ctx == nil || w.ctx.Err() != nil {
		if w.cancel != nil {
			w.cancel()
		}
		if owner == nil {
			owner = context.Background()
		}
		w.ctx, w.cancel = context.WithCancel(owner)
		w.revision = rev
	}
	copy := *cfg
	if w.pendingStops == nil {
		w.pendingStops = map[string]bool{}
	}
	if w.cfg != nil {
		if w.cfg.Ollama.ManagedInstance.Enabled && !cfg.Ollama.ManagedInstance.Enabled {
			w.pendingStops[ollamaManagedContainerName] = true
		}
		if w.cfg.Embeddings.LocalOllama.Enabled && !cfg.Embeddings.LocalOllama.Enabled {
			w.pendingStops[ollamaEmbContainerName] = true
		}
	}
	w.cfg = &copy
}

func OllamaStopsPending() bool {
	w := ollamaRuntimeWork
	w.mu.Lock()
	defer w.mu.Unlock()
	return len(w.pendingStops) > 0
}

// ConfirmOllamaStops resolves a failed/uncertain stop with safe reads only.
func ConfirmOllamaStops(ctx context.Context, host string) error {
	w := ollamaRuntimeWork
	w.mu.Lock()
	revision := w.revision
	names := []string{}
	for name := range w.pendingStops {
		names = append(names, name)
	}
	w.mu.Unlock()
	for _, name := range names {
		data, code, err := dockerRequest(DockerConfig{Host: host, Context: ctx}, "GET", "/containers/"+name+"/json", "")
		if err != nil {
			return err
		}
		var info struct{ State struct{ Running *bool } }
		if code != 404 && (code != 200 || json.Unmarshal(data, &info) != nil || info.State.Running == nil || *info.State.Running) {
			return fmt.Errorf("managed Ollama stop is not confirmed; check the administrator container page")
		}
		w.mu.Lock()
		if w.revision == revision {
			delete(w.pendingStops, name)
		}
		w.mu.Unlock()
	}
	return nil
}

// ShutdownOllamaRuntime cancels and drains HTTP and Docker work before stores close.
func ShutdownOllamaRuntime() {
	w := ollamaRuntimeWork
	w.mu.Lock()
	w.stopped = true
	if w.cancel != nil {
		w.cancel()
	}
	w.mu.Unlock()
	w.wg.Wait()
}

func (w *ollamaRuntime) start(revision, key, model, container string, run func(*ollamaWork) error, explicitRecovery ...bool) (OllamaOperation, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.stopped || w.ctx == nil || w.ctx.Err() != nil || revision != w.revision {
		return OllamaOperation{}, fmt.Errorf("Ollama configuration changed or runtime stopped; refresh saved settings")
	}
	if job := w.active[key]; job != nil && job.ctx.Err() == nil {
		return ollamaReceipt(job), nil
	}
	if len(explicitRecovery) == 0 || !explicitRecovery[0] {
		var latest *ollamaWork
		for _, job := range w.jobs {
			if (job.key == key || container != "" && job.Container == container) && (latest == nil || latest.Updated.Before(job.Updated)) {
				latest = job
			}
		}
		if latest != nil && latest.State == "unknown" {
			return ollamaReceipt(latest), nil
		}
	}
	if len(w.active) >= 4 {
		return OllamaOperation{}, fmt.Errorf("Ollama is busy; wait for active operations")
	}
	// Retain terminal receipts until the bounded history is full. Active jobs
	// are never evicted and cannot turn into a false successful receipt.
	if len(w.jobs) >= 128 {
		var oldest *ollamaWork
		for _, job := range w.jobs {
			select {
			case <-job.done:
				if job.State == "unknown" {
					continue
				}
				if oldest == nil || job.Updated.Before(oldest.Updated) {
					oldest = job
				}
			default:
			}
		}
		if oldest != nil {
			delete(w.jobs, oldest.ID)
		} else {
			return OllamaOperation{}, fmt.Errorf("Ollama operation history is full; resolve uncertain operations before starting more work")
		}
	}
	w.sequence++
	ctx, cancel := context.WithTimeout(w.ctx, 30*time.Minute)
	job := &ollamaWork{OllamaOperation: OllamaOperation{ID: fmt.Sprintf("ollama-%s-%d", ollamaReceiptPrefix, w.sequence), State: "queued", Model: model, Container: container, Updated: time.Now().UTC()}, key: key, revision: revision, ctx: ctx, done: make(chan struct{}), apiReady: make(chan struct{})}
	w.jobs[job.ID], w.active[key] = job, job
	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		defer cancel()
		err := run(job)
		w.mu.Lock()
		var unknown *ollamaUnknownOutcome
		if err == nil && ctx.Err() != nil {
			err = ctx.Err()
		}
		if err == nil {
			if job.State != "stopped" {
				job.State = "ready"
			}
		} else if errors.As(err, &unknown) {
			job.State = "unknown"
		} else if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) || ctx.Err() != nil {
			job.State = "cancelled"
		} else {
			job.State = "failed"
		}
		if err != nil {
			job.Error = dockerutil.SanitizeOneLine(security.RedactSensitiveInfo(err.Error()), 400)
		}
		job.Updated = time.Now().UTC()
		if w.active[key] == job {
			delete(w.active, key)
		}
		if job.State == "stopped" {
			delete(w.pendingStops, job.Container)
		}
		job.readyOnce.Do(func() { close(job.apiReady) })
		close(job.done)
		w.mu.Unlock()
	}()
	return ollamaReceipt(job), nil
}

func (w *ollamaRuntime) phase(job *ollamaWork, state string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	job.State, job.Updated = state, time.Now().UTC()
	if state == "api_ready" {
		job.readyOnce.Do(func() { close(job.apiReady) })
	}
}

// OllamaOperations returns passive receipt copies for one endpoint/container.
func OllamaOperations(container string) []OllamaOperation {
	w := ollamaRuntimeWork
	w.mu.Lock()
	defer w.mu.Unlock()
	result := []OllamaOperation{}
	for _, job := range w.jobs {
		if container != "" && job.revision != w.revision {
			continue
		}
		if container == "" || job.Container == container {
			result = append(result, ollamaReceipt(job))
		}
	}
	return result
}

func StartOllamaReconcile(cfg *config.Config, embeddings, force bool, logger ollamaEnsureLogger) (OllamaOperation, error) {
	if err := validateOllamaLifecycleConfig(cfg); err != nil {
		return OllamaOperation{}, err
	}
	name := ollamaManagedContainerName
	if embeddings {
		name = ollamaEmbContainerName
	}
	if !embeddings && !cfg.Ollama.ManagedInstance.Enabled || embeddings && !cfg.Embeddings.LocalOllama.Enabled {
		return OllamaOperation{}, fmt.Errorf("managed Ollama instance is disabled")
	}
	key := cfg.Docker.Host + "\x00" + name
	return ollamaRuntimeWork.start(ollamaConfigRevision(cfg), key, "", name, func(job *ollamaWork) error {
		// Two reserved-name locks also serialize aliases for the same Engine.
		lock, _ := ollamaRuntimeWork.locks.LoadOrStore(name, &sync.Mutex{})
		mu := lock.(*sync.Mutex)
		mu.Lock()
		defer mu.Unlock()
		if err := job.ctx.Err(); err != nil {
			return err
		}
		err := runOllamaReconcile(job, cfg, embeddings, force, logger)
		if err != nil && logger != nil {
			logger.Error("[Ollama] Managed initialization failed", "container", name, "error", dockerutil.SanitizeOneLine(security.RedactSensitiveInfo(err.Error()), 400))
		}
		return err
	}, force)
}

func StartOllamaStop(cfg *config.Config, embeddings bool) (OllamaOperation, error) {
	if !cfg.Docker.Enabled || cfg.Docker.ReadOnly {
		return OllamaOperation{}, ErrDockerReadOnly
	}
	name := ollamaManagedContainerName
	if embeddings {
		name = ollamaEmbContainerName
	}
	key := cfg.Docker.Host + "\x00" + name
	return ollamaRuntimeWork.start(ollamaConfigRevision(cfg), key, "", name, func(job *ollamaWork) error {
		lock, _ := ollamaRuntimeWork.locks.LoadOrStore(name, &sync.Mutex{})
		mu := lock.(*sync.Mutex)
		mu.Lock()
		defer mu.Unlock()
		if err := job.ctx.Err(); err != nil {
			return err
		}
		docker := DockerConfig{Host: cfg.Docker.Host, Context: job.ctx}
		data, code, err := dockerRequest(docker, "GET", "/containers/"+name+"/json", "")
		if err != nil {
			return err
		}
		if code == 404 {
			ollamaRuntimeWork.phase(job, "stopped")
			return nil
		}
		if code != 200 {
			return fmt.Errorf("inspect Ollama before stop: HTTP %d", code)
		}
		if !ollamaOwnsInspect(data, []byte(`{}`)) {
			return fmt.Errorf("Ollama ownership is unverified; stop refused")
		}
		_, code, err = dockerRequest(docker, "POST", "/containers/"+name+"/stop", "")
		if err != nil {
			return &ollamaUnknownOutcome{fmt.Errorf("stop outcome unknown: %w", err)}
		}
		if code != 204 && code != 304 {
			return fmt.Errorf("Ollama stop returned HTTP %d", code)
		}
		ollamaRuntimeWork.phase(job, "stopped")
		return nil
	}, true)
}

func ensureOllamaRuntime(cfg *config.Config, embeddings bool, logger ollamaEnsureLogger) {
	op, err := StartOllamaReconcile(cfg, embeddings, false, logger)
	if err != nil {
		logger.Error("[Ollama] Lifecycle refused", "error", err)
		return
	}
	w := ollamaRuntimeWork
	w.mu.Lock()
	job := w.jobs[op.ID]
	w.mu.Unlock()
	if job == nil {
		return
	}
	timer := time.NewTimer(60 * time.Second)
	defer timer.Stop()
	select {
	case <-job.apiReady:
	case <-timer.C:
		logger.Warn("[Ollama] Startup continues while managed initialization is pending", "operation", op.ID)
	}
}

// StartOllamaToolPull accepts a server-owned download without blocking the turn.
// Status is exposed through the existing running operation; no schema expansion.
func StartOllamaToolPull(cfg OllamaConfig, model string) string {
	if err := nativeHTTPContext(cfg.Context).Err(); err != nil {
		return ollamaErrJSON(err.Error())
	}
	if denied := ollamaReadOnlyMutationError(cfg, "pull"); denied != "" {
		return denied
	}
	model = strings.TrimSpace(model)
	if model == "" {
		return ollamaErrJSON("model name is required")
	}
	w := ollamaRuntimeWork
	w.mu.Lock()
	live := w.cfg
	revision := w.revision
	allowed := live != nil && live.Ollama.Enabled && !live.Ollama.ReadOnly && strings.TrimRight(live.Ollama.URL, "/") == strings.TrimRight(cfg.URL, "/")
	w.mu.Unlock()
	if !allowed {
		return ollamaErrJSON("Ollama model writes are disabled or endpoint changed")
	}
	op, err := w.start(revision, "pull\x00"+strings.TrimRight(cfg.URL, "/")+"\x00"+model, model, "", func(job *ollamaWork) error {
		w.phase(job, "model_pull")
		cfg.Context = job.ctx
		return ollamaPullShared(cfg, model)
	})
	if err != nil {
		return ollamaErrJSON(err.Error())
	}
	return marshalToolJSON(map[string]any{"status": "ok", "operation": op, "message": "Download receipt. Use operation=running to check completion; only ready confirms success."})
}
