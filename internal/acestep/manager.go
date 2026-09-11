package acestep

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"aurago/internal/config"
	"aurago/internal/dockerutil"
	"aurago/internal/security"
)

type desired struct {
	Local      config.LocalMusicConfig
	Enabled    bool
	Writable   bool
	DockerHost string
	InDocker   bool
}

// Manager serializes lifecycle transitions and generation on one owned container.
type Manager struct {
	mu                 sync.Mutex
	op                 sync.Mutex
	want               desired
	applied            string
	attempted          string
	manualStop         bool
	probeOnly          bool
	needsStop          bool
	forceQualification bool
	status             Status
	key                string
	baseURL            string
	vault              *security.Vault
	logger             *slog.Logger
	dataDir            string
	docker             *dockerutil.Client
	client             *http.Client
	ctx                context.Context
	cancel             context.CancelFunc
	jobCancel          context.CancelFunc
	setupCancel        context.CancelFunc
	wake               chan struct{}
	done               chan struct{}
	started            sync.Once
}

var defaultManager atomic.Pointer[Manager]

func SetDefault(m *Manager) { defaultManager.Store(m) }
func Default() *Manager     { return defaultManager.Load() }

func New(cfg *config.Config, vault *security.Vault, logger *slog.Logger) *Manager {
	if logger == nil {
		logger = slog.Default()
	}
	ctx, cancel := context.WithCancel(context.Background())
	m := &Manager{vault: vault, logger: logger, dataDir: cfg.Directories.DataDir, ctx: ctx, cancel: cancel, wake: make(chan struct{}, 1), done: make(chan struct{})}
	m.client = &http.Client{Timeout: 30 * time.Second, Transport: &http.Transport{Proxy: nil, ResponseHeaderTimeout: 20 * time.Second}, CheckRedirect: func(*http.Request, []*http.Request) error { return fmt.Errorf("acestep_redirect_rejected") }}
	m.status = Status{State: "disabled", Devices: []Device{}, ReleaseReady: len(manifest().Images) >= 4}
	m.Configure(cfg)
	return m
}

func (m *Manager) Configure(cfg *config.Config) {
	w := desired{Local: cfg.MusicGeneration.Local.Defaults(), Enabled: cfg.MusicGeneration.Enabled && cfg.UsesLocalMusic(), Writable: cfg.Docker.Enabled && !cfg.Docker.ReadOnly, DockerHost: cfg.Docker.Host, InDocker: cfg.Runtime.IsDocker}
	// Detach the optional reserve pointer from the published config snapshot.
	reserve := *w.Local.VRAMReserveGB
	w.Local.VRAMReserveGB = &reserve
	m.mu.Lock()
	changed := fingerprint(w) != fingerprint(m.want)
	m.want = w
	if w.Enabled {
		m.needsStop = true
	}
	m.status.Enabled = w.Enabled
	if changed {
		m.manualStop = false
		m.attempted = ""
		m.status.Pending = true
		if m.setupCancel != nil {
			m.setupCancel()
		}
		if !w.Enabled && m.jobCancel != nil {
			m.jobCancel()
		}
	}
	m.mu.Unlock()
	m.signal()
}

func (m *Manager) Start() { m.started.Do(func() { go m.run() }) }
func (m *Manager) signal() {
	select {
	case m.wake <- struct{}{}:
	default:
	}
}

func (m *Manager) Close() {
	m.cancel()
	m.Start()
	<-m.done
	m.client.CloseIdleConnections()
}

func (m *Manager) Status() Status {
	m.mu.Lock()
	defer m.mu.Unlock()
	// Status contains only sanitized values; JSON also detaches nested slices.
	b, _ := json.Marshal(m.status)
	var s Status
	_ = json.Unmarshal(b, &s)
	return s
}

// StopPending remains true until a requested deactivation has stopped the worker.
func (m *Manager) StopPending() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.needsStop
}

func (m *Manager) Action(action string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.want.Writable {
		return fmt.Errorf("docker_write_disabled")
	}
	if !m.want.Enabled {
		return fmt.Errorf("local_music_disabled")
	}
	switch action {
	case "stop":
		m.manualStop = true
		if m.jobCancel != nil {
			m.jobCancel()
		}
		if m.setupCancel != nil {
			m.setupCancel()
		}
	case "probe":
		m.probeOnly = true
	case "start", "recheck":
		m.manualStop = false
		m.attempted = ""
		if action != "start" {
			m.applied = ""
			m.forceQualification = true
		}
	default:
		return fmt.Errorf("invalid_acestep_action")
	}
	m.status.Pending = true
	m.signal()
	return nil
}

func (m *Manager) setState(state, code string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.status.State, m.status.ErrorCode = state, code
	m.status.Ready = state == "ready" || state == "busy"
}

func (m *Manager) run() {
	defer close(m.done)
	tick := time.NewTicker(30 * time.Second)
	defer tick.Stop()
	for {
		select {
		case <-m.ctx.Done():
			m.op.Lock()
			m.mu.Lock()
			writable := m.want.Writable
			m.mu.Unlock()
			if m.docker != nil {
				if writable {
					ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
					_ = m.stopOwned(ctx, ContainerName)
					cancel()
				}
				m.docker.CloseIdleConnections()
			}
			m.op.Unlock()
			return
		case <-m.wake:
		case <-tick.C:
		}
		if !m.op.TryLock() {
			continue
		}
		m.reconcile()
		m.op.Unlock()
	}
}

func (m *Manager) reconcile() {
	m.mu.Lock()
	w := m.want
	stopped := m.manualStop
	applied := m.applied
	attempted := m.attempted
	probeOnly := m.probeOnly
	m.probeOnly = false
	m.mu.Unlock()
	revision := fingerprint(w)
	defer func() {
		m.mu.Lock()
		defer m.mu.Unlock()
		m.status.Pending = fingerprint(m.want) != revision || m.manualStop != stopped || m.probeOnly
	}()
	if err := w.Local.Validate(); err != nil {
		m.setState("error", "invalid_local_music_config")
		return
	}
	if !w.Writable {
		m.setState("disabled", "docker_write_disabled")
		return
	}
	if m.docker == nil || m.docker.Host() != dockerutil.NormalizeHost(w.DockerHost) {
		// A Docker host change cannot orphan the previous owned runtime.
		if m.docker != nil {
			if err := m.stopOwned(m.ctx, ContainerName); err != nil {
				m.setState("error", "previous_runtime_stop_failed")
				return
			}
			m.docker.CloseIdleConnections()
		}
		m.docker = dockerutil.NewClient(w.DockerHost, 30*time.Second)
	}
	if probeOnly && w.Enabled {
		previous := m.Status()
		ctx, cancel := context.WithTimeout(m.ctx, 30*time.Minute)
		m.mu.Lock()
		m.setupCancel = cancel
		m.mu.Unlock()
		_, err := m.detect(ctx, w)
		cancel()
		m.mu.Lock()
		m.setupCancel = nil
		m.status.Pending = false
		m.mu.Unlock()
		m.setState(previous.State, previous.ErrorCode)
		if err != nil {
			m.setState(previous.State, safeError(err))
		}
		return
	}
	if !w.Enabled || stopped {
		if err := m.stopOwned(m.ctx, ContainerName); err != nil {
			m.setState("error", "runtime_stop_failed")
			return
		}
		m.mu.Lock()
		m.applied = ""
		m.needsStop = false
		m.status.Pending = false
		m.mu.Unlock()
		if stopped {
			m.setState("stopped", "")
		} else {
			m.setState("disabled", "")
		}
		return
	}
	if applied == revision {
		if err := m.testConnection(m.ctx); err == nil {
			m.setState("ready", "")
			return
		}
		m.setState("error", "runtime_unavailable")
		// Recreate only the known owned runtime; Docker's restart policy handles crashes.
		return
	}
	if attempted == revision {
		return
	}
	m.mu.Lock()
	m.attempted = revision
	m.mu.Unlock()
	ctx, cancel := context.WithTimeout(m.ctx, 6*time.Hour)
	defer cancel()
	m.mu.Lock()
	m.setupCancel = cancel
	m.mu.Unlock()
	defer func() { m.mu.Lock(); m.setupCancel = nil; m.mu.Unlock() }()
	if err := m.install(ctx, w); err != nil {
		code := safeError(err)
		m.mu.Lock()
		m.status.Pending = false
		ready := m.status.Ready
		m.status.ErrorCode = code
		m.mu.Unlock()
		if !ready {
			m.setState("error", code)
		}
		m.logger.Warn("[ACE-Step] Runtime setup failed", "code", code)
		return
	}
	m.mu.Lock()
	m.applied = revision
	m.forceQualification = false
	m.status.Pending = fingerprint(m.want) != revision
	m.mu.Unlock()
	m.setState("ready", "")
}

func safeError(err error) string {
	if errors.Is(err, context.Canceled) {
		return "acestep_canceled"
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "acestep_timeout"
	}
	if err != nil {
		code := err.Error()
		if len(code) < 80 && !strings.ContainsAny(code, " \r\n:/\\") {
			return code
		}
	}
	return "acestep_runtime_failed"
}

func (m *Manager) runtimeKey() error {
	if m.vault == nil {
		return fmt.Errorf("acestep_vault_unavailable")
	}
	key, err := m.vault.ReadSecret(VaultKey)
	if err != nil || key == "" {
		b := make([]byte, 32)
		if _, err = rand.Read(b); err != nil {
			return err
		}
		key = hex.EncodeToString(b)
		if err = m.vault.WriteSecret(VaultKey, key); err != nil {
			return err
		}
	}
	security.RegisterSensitive(key)
	m.key = key
	return nil
}

func (m *Manager) TestConnection(ctx context.Context) error {
	if !m.op.TryLock() {
		if m.Status().State == "busy" {
			return fmt.Errorf("acestep_busy")
		}
		return fmt.Errorf("acestep_starting")
	}
	defer m.op.Unlock()
	if !m.Status().Ready {
		return fmt.Errorf("acestep_not_ready")
	}
	return m.testConnection(ctx)
}

func (m *Manager) testConnection(ctx context.Context) error {
	if m.baseURL == "" || m.key == "" {
		return fmt.Errorf("acestep_not_ready")
	}
	var info runtimeStatus
	if err := m.request(ctx, "GET", "/aurago/status", nil, &info); err != nil {
		return err
	}
	if !info.Ready || info.Profile.Fingerprint == "" {
		return fmt.Errorf("acestep_models_not_ready")
	}
	current := m.Status().Profile
	if current != nil && current.Fingerprint != info.Profile.Fingerprint {
		return fmt.Errorf("acestep_profile_changed")
	}
	return nil
}

func (m *Manager) Generate(ctx context.Context, params Params) (Audio, error) {
	if !m.op.TryLock() {
		return Audio{}, fmt.Errorf("acestep_busy")
	}
	defer m.op.Unlock()
	m.mu.Lock()
	w := m.want
	s := m.status
	stopped := m.manualStop
	m.mu.Unlock()
	if !w.Enabled || !w.Writable || stopped || !s.Ready || s.Profile == nil {
		return Audio{}, fmt.Errorf("acestep_not_ready")
	}
	p, err := params.Validate(s.Profile.MaxDuration, s.Profile.LMModel != "")
	if err != nil {
		return Audio{}, err
	}
	jobCtx, cancel := context.WithTimeout(ctx, time.Duration(w.Local.TimeoutSeconds)*time.Second)
	stopParent := context.AfterFunc(m.ctx, cancel)
	defer stopParent()
	defer cancel()
	m.mu.Lock()
	m.jobCancel = cancel
	m.mu.Unlock()
	m.setState("busy", "")
	defer func() { m.mu.Lock(); m.jobCancel = nil; m.mu.Unlock(); m.signal() }()
	audio, err := m.generate(jobCtx, p, *s.Profile)
	if err != nil {
		// Upstream has no cancellation API. Stop the sole owned worker before
		// accepting another job, even when the submission response was lost.
		stopCtx, stopCancel := context.WithTimeout(context.Background(), 30*time.Second)
		stopErr := m.stopOwned(stopCtx, ContainerName)
		stopCancel()
		m.mu.Lock()
		m.applied = ""
		m.attempted = ""
		if stopErr != nil {
			m.manualStop = true
		}
		m.mu.Unlock()
		if stopErr != nil {
			m.setState("error", "acestep_stop_failed")
		} else {
			m.setState("starting", safeError(err))
		}
		return Audio{}, err
	}
	m.setState("ready", "")
	return audio, nil
}

func (m *Manager) outputDir() (string, error) {
	dir := filepath.Join(m.dataDir, "audio")
	return dir, os.MkdirAll(dir, 0755)
}
