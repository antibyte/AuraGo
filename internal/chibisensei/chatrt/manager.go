// Package chatrt supervises the llama.cpp runtime of the Chibi Sensei test chat: it installs the pinned
// llama-server into its own cache, starts at most MaxInstances loopback child processes and streams chat
// completions from them. It imports no other Chibi Sensei package.
package chatrt

import (
	"context"
	"log/slog"
	"sort"
	"sync"
	"time"

	"aurago/internal/fileutil"
	"aurago/internal/llamacpp"
)

// Manager owns the install state and the child processes. The mutex is never held across a launcher call.
type Manager struct {
	cfg      func() Config
	launcher Launcher
	log      *slog.Logger
	now      func() time.Time

	reapEvery time.Duration
	busyWait  time.Duration
	freeDisk  func(path string) (int64, error)

	life       context.Context
	lifeCancel context.CancelFunc
	wg         sync.WaitGroup // install and start goroutines

	mu            sync.Mutex
	res           *resolution
	installed     *bool
	verifyMu      sync.Mutex
	inst          *installOp
	lastErrCode   string
	gpuFailed     bool
	failedFor     string // backend the sticky fallback belongs to
	instances     map[string]*instance
	draining      int // instances taken out of the map whose process is not yet stopped; guarded by mu, counted against MaxInstances
	closed        bool
	reaperStarted bool
	reaperDone    chan struct{}
}

// instance is one child process and its bookkeeping. Fields below ready are guarded by Manager.mu.
type instance struct {
	key         string
	backend     string
	gpuVerified bool
	proc        Process
	started     time.Time
	lastUsed    time.Time
	ready       chan struct{} // closed when the start ended (startErr tells how)
	startErr    error
	slot        chan struct{} // capacity 1: held by the lease that runs a request
	waiters     int
	stopping    bool
	drainStart  bool // removed while still starting: startInstance ends the drain after its process is stopped
	cancelStart context.CancelFunc
}

// NewManager builds a manager. cfg is read on every call, so configuration changes apply at once.
func NewManager(cfg func() Config, launcher Launcher, log *slog.Logger) *Manager {
	if log == nil {
		log = slog.Default()
	}
	life, cancel := context.WithCancel(context.Background())
	return &Manager{
		cfg: cfg, launcher: launcher, log: log, now: time.Now,
		reapEvery: 30 * time.Second, busyWait: 30 * time.Second, freeDisk: fileutil.FreeDiskBytes,
		life: life, lifeCancel: cancel, instances: map[string]*instance{},
	}
}

// Start runs the idle reaper until ctx ends or the manager is closed. It is idempotent.
func (m *Manager) Start(ctx context.Context) {
	m.mu.Lock()
	if m.reaperStarted || m.closed {
		m.mu.Unlock()
		return
	}
	m.reaperStarted = true
	m.reaperDone = make(chan struct{})
	done, every := m.reaperDone, m.reapEvery
	m.mu.Unlock()
	go func() {
		defer close(done)
		ticker := time.NewTicker(every)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-m.life.Done():
				return
			case <-ticker.C:
				m.reapOnce()
			}
		}
	}()
}

// reapOnce stops idle instances (every instance when the runtime is disabled) and drops dead ones.
func (m *Manager) reapOnce() {
	cfg := m.cfg()
	idle := time.Duration(cfg.IdleMinutes) * time.Minute
	now := m.now()
	var stop []Process
	m.mu.Lock()
	for key, in := range m.instances {
		if in.stopping {
			continue
		}
		var reap bool
		switch {
		case !cfg.Enabled:
			reap = true
		case isReady(in) && procDead(in):
			reap = true
		case isReady(in) && len(in.slot) == 0 && in.waiters == 0 && now.Sub(in.lastUsed) >= idle:
			reap = true
		}
		if reap {
			if p := m.takeOutLocked(key, in); p != nil {
				stop = append(stop, p)
			}
		}
	}
	m.mu.Unlock()
	m.stopDraining(stop)
}

// takeOutLocked removes an instance from the map for stopping and keeps it counted (draining) until its process
// has stopped. It returns the process to stop, or nil while the instance still starts: startInstance then stops
// the new process itself and ends the drain. Called with m.mu held.
func (m *Manager) takeOutLocked(key string, in *instance) Process {
	in.stopping = true
	in.cancelStart()
	delete(m.instances, key)
	m.draining++
	if in.proc == nil {
		in.drainStart = true
		return nil
	}
	return in.proc
}

// stopDraining stops processes taken out by takeOutLocked, without holding m.mu, and ends their drain.
func (m *Manager) stopDraining(procs []Process) {
	for _, p := range procs {
		_ = p.Stop()
		m.mu.Lock()
		m.draining--
		m.mu.Unlock()
	}
}

// isReady reports a finished, successful start. Guarded by Manager.mu for startErr and proc.
func isReady(in *instance) bool {
	select {
	case <-in.ready:
		return in.startErr == nil && in.proc != nil
	default:
		return false
	}
}

func procDead(in *instance) bool {
	if in.proc == nil {
		return false
	}
	select {
	case <-in.proc.Done():
		return true
	default:
		return false
	}
}

// Status reports the runtime state. It may run the hardware probe once (auto backend) and the install check once.
func (m *Manager) Status(ctx context.Context) Status {
	cfg := m.cfg()
	st := Status{Build: llamacpp.Build, Requested: normalizeBackend(cfg.Backend)}
	var res resolution
	if cfg.Enabled {
		res = m.resolve(ctx)
		st.Selected = res.selected
		st.Download.Bytes = res.totalBytes()
		st.Download.Host = res.host()
	}
	installed := cfg.Enabled && !res.unsupported && m.Installed()
	m.mu.Lock()
	defer m.mu.Unlock()
	st.Fallback = m.gpuFailed
	st.Download.ErrorCode = m.lastErrCode
	if m.inst != nil {
		st.Download.DoneBytes = m.inst.doneTotal()
	}
	switch {
	case !cfg.Enabled:
		st.State = "disabled"
	case res.unsupported:
		st.State = "unsupported"
	case m.inst != nil:
		st.State = "installing"
	case m.lastErrCode != "":
		st.State = "failed"
	case installed:
		st.State = "ready"
	default:
		st.State = "not_installed"
	}
	now := m.now()
	for _, in := range m.instances {
		st.Instances = append(st.Instances, m.instanceStatus(in, now, res.selected))
	}
	sort.Slice(st.Instances, func(i, j int) bool { return st.Instances[i].Key < st.Instances[j].Key })
	return st
}

// instanceStatus is called with m.mu held.
func (m *Manager) instanceStatus(in *instance, now time.Time, selected string) InstanceStatus {
	s := InstanceStatus{Key: in.key, Backend: in.backend, GPUVerified: in.gpuVerified, Started: in.started.UTC().Format(time.RFC3339)}
	if s.Backend == "" {
		s.Backend = selected
	}
	switch {
	case in.stopping:
		s.State = "stopping"
	case !isReady(in):
		s.State = "starting"
	case len(in.slot) > 0:
		s.State = "busy"
	default:
		s.State = "ready"
		s.IdleSeconds = max(0, int(now.Sub(in.lastUsed).Seconds()))
	}
	return s
}

// StopAll stops every instance. Busy chats end with ErrStopped. It is idempotent.
func (m *Manager) StopAll(ctx context.Context) {
	var stop []Process
	m.mu.Lock()
	for key, in := range m.instances {
		if p := m.takeOutLocked(key, in); p != nil {
			stop = append(stop, p)
		}
	}
	m.mu.Unlock()
	m.stopDraining(stop)
}

// Close ends installs, starts and children and waits for the reaper. Afterwards every call answers ErrClosed.
func (m *Manager) Close() {
	m.mu.Lock()
	m.closed = true
	done := m.reaperDone
	m.mu.Unlock()
	m.lifeCancel()
	m.StopAll(context.Background())
	if done != nil {
		select {
		case <-done:
		case <-time.After(5 * time.Second):
		}
	}
	finished := make(chan struct{})
	go func() { m.wg.Wait(); close(finished) }()
	select {
	case <-finished:
	case <-time.After(10 * time.Second):
		m.log.Warn("Chibi Sensei chat runtime close timed out waiting for background work")
	}
}
