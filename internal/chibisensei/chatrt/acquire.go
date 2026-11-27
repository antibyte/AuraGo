package chatrt

import (
	"context"
	"errors"
	"fmt"
	"os"
	"runtime"
	"sync/atomic"
	"time"

	"aurago/internal/security"
)

const startTimeout = 6 * time.Minute

// Instance is a lease on one running model: one request at a time. Call Release when done.
type Instance struct {
	m        *Manager
	in       *instance
	released atomic.Bool
}

// state answers the refusal that Status would show, in the same precedence. It never downloads.
func (m *Manager) state(ctx context.Context) error {
	if err := m.precheck(); err != nil {
		return err
	}
	if m.resolve(ctx).unsupported {
		return ErrUnsupported
	}
	m.mu.Lock()
	installing := m.inst != nil
	m.mu.Unlock()
	if installing {
		return ErrInstalling
	}
	if !m.Installed() {
		return ErrNotInstalled
	}
	return nil
}

// Admit is the pre-stream check: it answers ErrClosed, ErrDisabled, ErrUnsupported, ErrInstalling or
// ErrNotInstalled, and ErrBusy when starting the models of keys would need more than MaxInstances slots.
func (m *Manager) Admit(keys ...string) error {
	if err := m.state(m.life); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return ErrClosed
	}
	wanted := map[string]bool{}
	for _, k := range keys {
		wanted[k] = true
	}
	need, freeable := 0, 0
	for k := range wanted {
		if _, ok := m.instances[k]; !ok {
			need++
		}
	}
	for key, in := range m.instances {
		if !wanted[key] && evictable(in) {
			freeable++
		}
	}
	if len(m.instances)+m.draining-freeable+need > MaxInstances {
		return ErrBusy
	}
	return nil
}

// evictable is an idle, fully started instance nobody waits for. Called with m.mu held.
func evictable(in *instance) bool {
	return !in.stopping && isReady(in) && len(in.slot) == 0 && in.waiters == 0
}

// Acquire returns a lease on the model of ref, starting it when needed. It waits up to 30 s for a busy
// instance of the same key and never downloads.
func (m *Manager) Acquire(ctx context.Context, ref ModelRef) (*Instance, error) {
	if err := m.state(ctx); err != nil {
		return nil, err
	}
	if !modelFileOK(ref) {
		return nil, ErrModelMissing
	}
	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		in, err := m.attach(ref)
		if err != nil {
			return nil, err
		}
		lease, err := m.lease(ctx, in)
		if err == nil {
			return lease, nil
		}
		if !errors.Is(err, errRetry) {
			return nil, err
		}
		lastErr = err
	}
	return nil, fmt.Errorf("%w: %v", ErrStopped, lastErr)
}

// modelFileOK checks the model file right before a start without following a link: a regular file (no symbolic
// link, directory or device) whose size equals ref.Bytes when ref.Bytes > 0.
func modelFileOK(ref ModelRef) bool {
	st, err := os.Lstat(ref.Path)
	if err != nil || !st.Mode().IsRegular() {
		return false
	}
	return ref.Bytes <= 0 || st.Size() == ref.Bytes
}

// instanceThreads is the thread count of one instance: a quarter of the logical processors, 1 to 8. Two instances
// together use about half of the logical processors, which is the physical core count on SMT machines.
func instanceThreads(logical int) int { return min(max(logical/4, 1), 8) }

var errRetry = errors.New("instance gone")

// attach finds or creates the instance of ref and counts the caller as a waiter.
func (m *Manager) attach(ref ModelRef) (*instance, error) {
	cfg := m.cfg()
	res := m.resolve(m.life)
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return nil, ErrClosed
	}
	if in, ok := m.instances[ref.Key]; ok {
		in.waiters++
		m.mu.Unlock()
		return in, nil
	}
	var evict *instance
	if len(m.instances)+m.draining >= MaxInstances { // instances being stopped still hold a process
		for _, in := range m.instances {
			if evictable(in) && (evict == nil || in.lastUsed.Before(evict.lastUsed)) {
				evict = in
			}
		}
		if evict == nil {
			m.mu.Unlock()
			return nil, ErrBusy
		}
		evict.stopping = true
		delete(m.instances, evict.key)
	}
	startCtx, cancel := context.WithTimeout(m.life, startTimeout)
	now := m.now()
	in := &instance{
		key: ref.Key, started: now, lastUsed: now, ready: make(chan struct{}), slot: make(chan struct{}, 1),
		waiters: 1, cancelStart: cancel,
	}
	m.instances[ref.Key] = in
	m.wg.Add(1)
	m.mu.Unlock()
	go m.startInstance(startCtx, in, ref, evict, cfg, res)
	return in, nil
}

// startInstance runs in its own goroutine, bounded by startCtx (6 minutes, Close and StopAll cancel it).
func (m *Manager) startInstance(ctx context.Context, in *instance, ref ModelRef, evict *instance, cfg Config, res resolution) {
	defer m.wg.Done()
	defer in.cancelStart()
	if evict != nil && evict.proc != nil {
		_ = evict.proc.Stop()
	}
	proc, backend, verified, err := m.launchWithFallback(ctx, ref, cfg, res)
	var late Process // started after the instance was taken out: stopped here, then the drain ends
	m.mu.Lock()
	switch {
	case err != nil:
		in.startErr = fmt.Errorf("start chat runtime: %w", err)
		if m.instances[in.key] == in {
			delete(m.instances, in.key)
		}
	case in.stopping:
		in.startErr = ErrStopped
		late = proc
	default:
		in.proc, in.backend, in.gpuVerified = proc, backend, verified
	}
	m.mu.Unlock()
	close(in.ready)
	if late != nil {
		_ = late.Stop()
	}
	m.mu.Lock()
	if in.drainStart {
		in.drainStart = false
		m.draining--
	}
	m.mu.Unlock()
}

// launchWithFallback starts the model on the selected backend. A GPU start that fails or shows no GPU evidence
// is stopped and the model starts again on CPU; the fallback is sticky until the next install or backend change.
func (m *Manager) launchWithFallback(ctx context.Context, ref ModelRef, cfg Config, res resolution) (Process, string, bool, error) {
	spec := LaunchSpec{
		Key: ref.Key, CacheDir: cfg.CacheDir, ModelPath: ref.Path, ContextSize: cfg.ContextSize,
		Threads: instanceThreads(runtime.NumCPU()),
	}
	m.mu.Lock()
	useCPU := m.gpuFailed && m.failedFor == res.selected
	m.mu.Unlock()
	if res.selected != "cpu" && !useCPU {
		spec.Backend, spec.Assets = res.selected, res.launchAssets(res.selected)
		proc, err := m.launcher.Launch(ctx, spec)
		if err == nil && proc.GPUVerified() {
			return proc, res.selected, true, nil
		}
		if err == nil {
			_ = proc.Stop()
			err = errors.New("no GPU evidence in the server log")
		}
		if ctx.Err() != nil {
			return nil, "", false, err
		}
		m.log.Warn("Chibi Sensei chat runtime fell back to CPU", "backend", res.selected, "error", security.Scrub(err.Error()))
		m.mu.Lock()
		m.gpuFailed, m.failedFor = true, res.selected
		m.mu.Unlock()
	}
	spec.Backend, spec.Assets = "cpu", res.launchAssets("cpu")
	proc, err := m.launcher.Launch(ctx, spec)
	if err != nil {
		return nil, "", false, err
	}
	return proc, "cpu", false, nil
}

// lease waits for the instance to be ready, then for its slot.
func (m *Manager) lease(ctx context.Context, in *instance) (*Instance, error) {
	done := func() { m.mu.Lock(); in.waiters--; m.mu.Unlock() }
	select {
	case <-in.ready:
	case <-ctx.Done():
		done()
		return nil, ctx.Err()
	}
	m.mu.Lock()
	startErr := in.startErr
	m.mu.Unlock()
	if startErr != nil {
		done()
		return nil, startErr
	}
	timer := time.NewTimer(m.busyWait)
	defer timer.Stop()
	select {
	case in.slot <- struct{}{}:
	case <-timer.C:
		done()
		return nil, ErrBusy
	case <-ctx.Done():
		done()
		return nil, ctx.Err()
	}
	m.mu.Lock()
	in.waiters--
	gone := in.stopping || procDead(in)
	if gone {
		if m.instances[in.key] == in {
			delete(m.instances, in.key)
		}
		in.stopping = true
	}
	m.mu.Unlock()
	if gone {
		<-in.slot
		return nil, errRetry
	}
	return &Instance{m: m, in: in}, nil
}

// Backend names the backend that answers: cpu, vulkan, cuda or metal.
func (i *Instance) Backend() string {
	if i == nil || i.m == nil || i.in == nil {
		return ""
	}
	i.m.mu.Lock()
	defer i.m.mu.Unlock()
	return i.in.backend
}

// Chat streams one answer. ctx ends the generation at once (the upstream request is closed).
func (i *Instance) Chat(ctx context.Context, req ChatRequest, onDelta func(text string)) (ChatStats, error) {
	if i == nil || i.m == nil || i.in == nil {
		return ChatStats{}, errors.New("chat runtime: instance not held")
	}
	if i.released.Load() {
		return ChatStats{}, errors.New("chat runtime: instance not held")
	}
	i.m.mu.Lock()
	proc, backend := i.in.proc, i.in.backend
	i.m.mu.Unlock()
	stats, err := proc.Chat(ctx, req, onDelta)
	if err == nil {
		stats.Backend = backend
		return stats, nil
	}
	if ctx.Err() != nil {
		return ChatStats{}, ctx.Err()
	}
	i.m.mu.Lock()
	stopping := i.in.stopping
	i.m.mu.Unlock()
	if stopping {
		return ChatStats{}, ErrStopped
	}
	if errors.Is(err, ErrContext) {
		return ChatStats{}, err
	}
	if procDead(i.in) {
		i.m.mu.Lock()
		if i.m.instances[i.in.key] == i.in {
			delete(i.m.instances, i.in.key)
		}
		i.in.stopping = true
		i.m.mu.Unlock()
		return ChatStats{}, fmt.Errorf("chat runtime process ended: %w", err)
	}
	return ChatStats{}, err
}

// Release gives the lease back. It is safe to call more than once and after Close.
func (i *Instance) Release() {
	if i == nil || i.in == nil {
		return
	}
	if !i.released.CompareAndSwap(false, true) {
		return
	}
	i.m.mu.Lock()
	i.in.lastUsed = i.m.now()
	i.m.mu.Unlock()
	select {
	case <-i.in.slot:
	default:
	}
}
