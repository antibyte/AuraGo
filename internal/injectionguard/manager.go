package injectionguard

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"aurago/internal/onnxrt"
)

const ScanTimeout = 20 * time.Second

type Status struct {
	Available        bool                  `json:"available"`
	Enabled          bool                  `json:"enabled"`
	Action           string                `json:"action"`
	Ready            bool                  `json:"ready"`
	State            string                `json:"state"`
	Error            string                `json:"error,omitempty"`
	Model            string                `json:"model"`
	Revision         string                `json:"revision"`
	SizeBytes        int64                 `json:"size_bytes"`
	RuntimeSizeBytes int64                 `json:"runtime_size_bytes"`
	Progress         onnxrt.DownloadStatus `json:"progress"`
}

type Manager struct {
	mu                         sync.Mutex
	ctx                        context.Context
	cancel                     context.CancelFunc
	root, modelDir, executable string
	runtime                    onnxrt.AssetSpec
	status                     Status
	generation                 uint64
	worker                     scorer
	slot                       chan struct{}
	admission                  chan struct{}
	jobCancel                  context.CancelFunc
	jobDone                    chan struct{}
	closed                     bool
	start                      func(context.Context, string, string, string) (scorer, error)
}

func NewManager(parent context.Context, dataDir, executable string) *Manager {
	ctx, cancel := context.WithCancel(parent)
	if executable == "" {
		executable, _ = os.Executable()
	}
	spec, ok := onnxrt.CPURuntimeAsset(runtime.GOOS, runtime.GOARCH)
	m := &Manager{ctx: ctx, cancel: cancel, root: filepath.Join(dataDir, "embeddings"), modelDir: filepath.Join(dataDir, "injectionguard", "models"), executable: executable, runtime: spec, slot: make(chan struct{}, 1), admission: make(chan struct{}, 9), start: startWorker}
	m.status = Status{Available: ok, Action: "quarantine", State: "not_installed", Model: ModelName, Revision: Revision, SizeBytes: ModelAsset.Size + TokenizerAsset.Size, RuntimeSizeBytes: spec.Size}
	if !ok {
		m.status.State = "unsupported"
	}
	return m
}

func (m *Manager) Status() Status { m.mu.Lock(); defer m.mu.Unlock(); return m.status }

// Configure revokes in-flight decisions even when only the response action changes.
func (m *Manager) Configure(enabled bool, action string) {
	if action == "" {
		action = "quarantine"
	}
	m.mu.Lock()
	if m.closed || (m.status.Enabled == enabled && m.status.Action == action) {
		m.mu.Unlock()
		return
	}
	m.generation++
	m.status.Enabled = enabled
	m.status.Action = action
	w := m.worker
	m.worker = nil
	if m.status.Ready {
		m.status.State = "ready"
	}
	m.mu.Unlock()
	if w != nil {
		w.Close()
	}
}

func (m *Manager) acquire(ctx context.Context) error {
	select {
	case m.admission <- struct{}{}:
	default:
		return fmt.Errorf("classifier busy")
	}
	select {
	case m.slot <- struct{}{}:
		return nil
	case <-ctx.Done():
		<-m.admission
		return ctx.Err()
	case <-m.ctx.Done():
		<-m.admission
		return m.ctx.Err()
	}
}
func (m *Manager) release() { <-m.slot; <-m.admission }

func (m *Manager) connection(ctx context.Context) (scorer, error) {
	m.mu.Lock()
	w := m.worker
	generation := m.generation
	closed := m.closed
	m.mu.Unlock()
	if closed {
		return nil, context.Canceled
	}
	if w != nil {
		return w, nil
	}
	rt, err := onnxrt.NewAssetCache(m.root, "", nil).VerifyRuntime(ctx, m.runtime)
	if err != nil {
		return nil, err
	}
	w, err = m.start(ctx, m.executable, rt, m.modelDir)
	if err != nil {
		return nil, err
	}
	m.mu.Lock()
	if m.closed || generation != m.generation || ctx.Err() != nil {
		m.mu.Unlock()
		w.Close()
		return nil, context.Canceled
	}
	m.worker = w
	m.mu.Unlock()
	return w, nil
}

// Evaluate satisfies security.ExternalContentScanner without importing security or config.
func (m *Manager) Evaluate(ctx context.Context, text string) (score float64, action string, enabled bool, err error) {
	m.mu.Lock()
	enabled, action = m.status.Enabled, m.status.Action
	generation := m.generation
	ready := m.status.Ready
	m.mu.Unlock()
	if !enabled {
		return 0, action, false, nil
	}
	if !ready || len(text) > MaxTextBytes {
		return 0, action, true, fmt.Errorf("classifier unavailable")
	}
	ctx, cancel := context.WithTimeout(ctx, ScanTimeout)
	defer cancel()
	stop := context.AfterFunc(m.ctx, cancel)
	defer stop()
	if err = m.acquire(ctx); err != nil {
		return
	}
	defer m.release()
	m.mu.Lock()
	valid := generation == m.generation && !m.closed
	m.mu.Unlock()
	if !valid {
		err = context.Canceled
		return
	}
	var w scorer
	w, err = m.connection(ctx)
	if err == nil {
		score, err = w.Score(ctx, text)
	}
	m.mu.Lock()
	stale := generation != m.generation || m.closed
	if stale || ctx.Err() != nil {
		err = context.Canceled
	}
	if err != nil {
		if m.worker == w {
			m.worker = nil
		}
		if !stale {
			m.status.Error = "classification_failed"
			if m.status.Ready {
				m.status.State = "error"
			}
		}
	} else {
		m.status.State = "active"
		m.status.Error = ""
	}
	m.mu.Unlock()
	if err != nil && w != nil {
		w.Close()
	}
	return
}

// Prepare is explicit. Only this path may download; it outlives the HTTP request.
func (m *Manager) Prepare() error { return m.startPreparation(true) }

// Inspect checks an existing installation in the background, without network access.
func (m *Manager) Inspect() {
	if _, err := os.Stat(filepath.Join(m.modelDir, ModelAsset.FileName)); err == nil {
		_ = m.startPreparation(false)
	}
}

func (m *Manager) startPreparation(download bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed || !m.status.Available {
		return fmt.Errorf("classifier unavailable")
	}
	if m.jobCancel != nil {
		return fmt.Errorf("installation busy")
	}
	m.generation++
	ctx, cancel := context.WithTimeout(m.ctx, 30*time.Minute)
	m.jobCancel = cancel
	m.jobDone = make(chan struct{})
	m.status.Ready = false
	m.status.Error = ""
	m.status.State = "verifying"
	if download {
		m.status.State = "downloading"
	}
	go m.prepare(ctx, download, m.jobDone)
	return nil
}

func (m *Manager) prepare(ctx context.Context, download bool, done chan struct{}) {
	defer close(done)
	progress := func(p onnxrt.DownloadStatus) { m.mu.Lock(); m.status.Progress = p; m.mu.Unlock() }
	cache := onnxrt.NewAssetCache(m.root, "AuraGo", progress)
	err := m.acquire(ctx)
	if err == nil {
		defer m.release()
		m.mu.Lock()
		w := m.worker
		m.worker = nil
		m.mu.Unlock()
		if w != nil {
			w.Close()
		}
		if download {
			models := onnxrt.NewAssetCache(filepath.Dir(m.modelDir), "AuraGo", progress)
			for _, spec := range []onnxrt.AssetSpec{ModelAsset, TokenizerAsset} {
				if err = models.EnsureFile(ctx, spec, filepath.Join(m.modelDir, spec.FileName)); err != nil {
					break
				}
			}
			if err == nil {
				_, err = cache.EnsureRuntime(ctx, m.runtime)
			}
		}
		if err == nil {
			m.mu.Lock()
			m.status.State = "verifying"
			m.mu.Unlock()
			verifyCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
			_, err = m.connection(verifyCtx)
			cancel()
		}
	}
	m.mu.Lock()
	if ctx.Err() != nil {
		err = ctx.Err()
	}
	m.jobCancel()
	m.jobCancel = nil
	if err == nil && !m.closed {
		m.status.Ready = true
		m.status.State = "ready"
		m.status.Error = ""
	} else {
		m.status.Ready = false
		m.status.State = "error"
		m.status.Error = "installation_failed"
		if errors.Is(err, context.Canceled) {
			m.status.Error = "cancelled"
		}
	}
	w := m.worker
	if !m.status.Enabled || err != nil {
		m.worker = nil
	} else {
		w = nil
	}
	m.mu.Unlock()
	if w != nil {
		w.Close()
	}
}

func (m *Manager) CancelPrepare() {
	m.mu.Lock()
	if m.jobCancel != nil {
		m.jobCancel()
	}
	m.mu.Unlock()
}

func (m *Manager) SelfTest(ctx context.Context) (err error) {
	ctx, cancel := context.WithTimeout(ctx, ScanTimeout)
	defer cancel()
	stop := context.AfterFunc(m.ctx, cancel)
	defer stop()
	if !m.Status().Ready {
		return fmt.Errorf("model not ready")
	}
	if err := m.acquire(ctx); err != nil {
		return err
	}
	defer m.release()
	w, err := m.connection(ctx)
	if err != nil {
		return err
	}
	m.mu.Lock()
	generation := m.generation
	m.mu.Unlock()
	defer func() {
		m.mu.Lock()
		stale := generation != m.generation || m.closed || ctx.Err() != nil
		if stale {
			err = context.Canceled
		}
		keep := err == nil && m.status.Enabled
		if !keep && m.worker == w {
			m.worker = nil
		}
		if err != nil && !stale {
			m.status.Ready = false
			m.status.State, m.status.Error = "error", "self_test_failed"
		}
		m.mu.Unlock()
		if !keep {
			w.Close()
		}
	}()
	a, err := w.Score(ctx, safeSample)
	if err != nil {
		return err
	}
	b, err := w.Score(ctx, attackSample)
	if err != nil || a >= Threshold || b < Threshold {
		return fmt.Errorf("self-test failed")
	}
	return nil
}

func (m *Manager) Close() {
	m.mu.Lock()
	m.closed = true
	m.generation++
	m.cancel()
	w := m.worker
	m.worker = nil
	done := m.jobDone
	m.mu.Unlock()
	if w != nil {
		w.Close()
	}
	if done != nil {
		<-done
	}
}
