package ruview

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"aurago/internal/config"
	"aurago/internal/security"
)

type Status struct {
	Enabled          bool       `json:"enabled"`
	ReadOnly         bool       `json:"readonly"`
	AgentAccess      bool       `json:"agent_access"`
	Mode             string     `json:"mode"`
	Source           string     `json:"source"`
	Revision         string     `json:"revision"`
	RevisionVerified bool       `json:"revision_verified"`
	Ready            bool       `json:"ready"`
	DataState        string     `json:"data_state"`
	Signal           string     `json:"signal"`
	Simulated        bool       `json:"simulated"`
	Fresh            bool       `json:"fresh"`
	ObservedAt       *time.Time `json:"observed_at"`
	FrameAgeMS       *int64     `json:"last_frame_age_ms"`
	Error            string     `json:"error,omitempty"`
	StorageLimit     bool       `json:"storage_limit"`
}

type Manager struct {
	config        func() *config.Config
	vault         *security.Vault
	logger        *slog.Logger
	mu            sync.Mutex
	opMu          sync.Mutex
	status        Status
	observed      time.Time
	generation    context.Context
	cancel        context.CancelFunc
	previous      config.RuViewConfig
	lifetime      context.Context
	stop          context.CancelFunc
	done          chan struct{}
	process       *nativeProcess
	managed       bool
	runningConfig *config.Config
}

var defaultManager atomic.Pointer[Manager]

func SetDefault(m *Manager) { defaultManager.Store(m) }
func Default() *Manager     { return defaultManager.Load() }

func New(snapshot func() *config.Config, vault *security.Vault, logger *slog.Logger) *Manager {
	ctx, cancel := context.WithCancel(context.Background())
	return &Manager{config: snapshot, vault: vault, logger: logger, generation: ctx, cancel: cancel, previous: snapshot().RuView}
}

// Configure revokes active streams immediately; it never installs or starts a runtime.
func (m *Manager) Configure() {
	c := m.config().RuView
	m.mu.Lock()
	defer m.mu.Unlock()
	if !reflect.DeepEqual(m.previous, c) {
		m.cancel()
		m.generation, m.cancel = context.WithCancel(context.Background())
		m.previous = c
		m.status = Status{}
		m.observed = time.Time{}
	}
}

func (m *Manager) bound(parent context.Context) (context.Context, func()) {
	m.mu.Lock()
	generation := m.generation
	m.mu.Unlock()
	ctx, cancel := context.WithCancel(parent)
	stop := context.AfterFunc(generation, cancel)
	return ctx, func() { stop(); cancel() }
}

func (m *Manager) Status() Status {
	c := m.config().RuView
	m.mu.Lock()
	s := m.status
	s.RevisionVerified = m.managed
	observed := m.observed
	m.mu.Unlock()
	s.Enabled = c.Enabled
	s.ReadOnly = c.ReadOnly
	s.AgentAccess = c.AgentAccess
	s.Mode = c.Mode
	s.Revision = Revision
	if s.Source == "" {
		s.Source = c.Source
	}
	if s.DataState == "" {
		s.DataState = "unchecked"
	}
	if !c.Enabled {
		s.Ready = false
		s.Fresh = false
		s.DataState = "disabled"
		return s
	}
	if !observed.IsZero() && s.FrameAgeMS != nil && *s.FrameAgeMS >= 0 {
		age := *s.FrameAgeMS + time.Since(observed).Milliseconds()
		s.FrameAgeMS = &age
		if s.DataState == "live_unverified" && age > 10000 {
			s.Fresh = false
			s.DataState = "stale"
		}
	}
	return s
}

func (m *Manager) Test(parent context.Context) (Status, error) {
	m.mu.Lock()
	generation := m.generation
	m.mu.Unlock()
	c := m.config()
	if !c.RuView.Enabled {
		return m.Status(), fmt.Errorf("RuView is disabled")
	}
	ctx, cancel := m.bound(parent)
	defer cancel()
	data, err := m.request(ctx, c, "GET", "/api/v1/status", nil)
	now := time.Now().UTC()
	s := Status{ObservedAt: &now, DataState: "disconnected"}
	if err == nil {
		var raw struct {
			Status  string `json:"status"`
			Source  string `json:"source"`
			State   string `json:"source_state"`
			Age     *int64 `json:"last_frame_age_ms"`
			Waiting bool   `json:"waiting_for_frames"`
		}
		if json.Unmarshal(data, &raw) != nil || raw.Status != "ready" {
			err = fmt.Errorf("RuView status contract is unsupported")
		} else {
			s.Ready = true
			s.Source = strings.SplitN(raw.Source, ":", 2)[0]
			s.DataState = raw.State
			s.FrameAgeMS = raw.Age
			s.Simulated = raw.State == "synthetic"
			s.Fresh = raw.State == "live_unverified" && raw.Age != nil && *raw.Age >= 0 && *raw.Age <= 10000 && !raw.Waiting
			if s.Simulated {
				s.Signal = "demo"
			} else if s.Source == "wifi" {
				s.Signal = "RSSI"
			} else if s.Source == "esp32" {
				s.Signal = "CSI"
			}
			if s.Source != c.RuView.Source {
				s.Fresh = false
				s.Error = "source_mismatch"
			}
			if s.Simulated && c.RuView.Source != "simulated" {
				s.Fresh = false
				s.Error = "unexpected_simulation"
			}
		}
	}
	if err != nil {
		s.Error = err.Error()
	}
	m.mu.Lock()
	current := ctx.Err() == nil && generation == m.generation && generation.Err() == nil
	if current {
		s.StorageLimit = m.status.StorageLimit
		m.status = s
		m.observed = now
	}
	m.mu.Unlock()
	if !current {
		return m.Status(), context.Canceled
	}
	return m.Status(), err
}

func (m *Manager) Read(ctx context.Context, operation string) (any, error) {
	c := m.config()
	if !c.RuView.Enabled || !c.RuView.AgentAccess {
		return nil, fmt.Errorf("RuView agent read access is disabled")
	}
	if operation == "status" {
		return m.Test(ctx)
	}
	p, ok := readOperations[operation]
	if !ok {
		return nil, fmt.Errorf("unsupported RuView read operation")
	}
	bound, done := m.bound(ctx)
	defer done()
	data, err := m.request(bound, c, "GET", p, nil)
	if err != nil {
		return nil, err
	}
	status, err := m.Test(bound)
	if err != nil {
		return nil, err
	}
	return map[string]any{"source": status.Source, "observed_at": time.Now().UTC(), "revision": Revision, "fresh": status.Fresh, "simulated": status.Simulated, "data_state": status.DataState, "data": data}, nil
}

func (m *Manager) StartBackground(parent context.Context) {
	m.mu.Lock()
	if m.stop != nil {
		m.mu.Unlock()
		return
	}
	m.lifetime, m.stop = context.WithCancel(parent)
	ctx := m.lifetime
	m.done = make(chan struct{})
	m.mu.Unlock()
	go func() {
		defer close(m.done)
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				m.watch(ctx)
			}
		}
	}()
}

func (m *Manager) watch(ctx context.Context) {
	m.Configure()
	m.opMu.Lock()
	defer m.opMu.Unlock()
	m.mu.Lock()
	m.reapNativeLocked()
	managed := m.managed
	running := m.runningConfig
	m.mu.Unlock()
	if !managed {
		return
	}
	c := m.config()
	if !c.RuView.Enabled || running == nil || !sameRuntime(running, c) {
		_ = m.stopRuntime(ctx)
		return
	}
	limited := storageLimited(c)
	if limited {
		bounded, cancel := context.WithTimeout(ctx, 5*time.Second)
		_, _ = m.request(bounded, c, "POST", "/api/v1/recording/stop", nil)
		cancel()
	}
	m.mu.Lock()
	m.status.StorageLimit = limited
	m.mu.Unlock()
}

// Called with mu held, and opMu held by lifecycle callers. The current pointer
// ensures an old process completion cannot clear a replacement process.
func (m *Manager) reapNativeLocked() {
	if m.process == nil {
		return
	}
	select {
	case <-m.process.done:
		m.process = nil
		m.managed = false
		m.runningConfig = nil
		m.cancel()
		m.generation, m.cancel = context.WithCancel(context.Background())
		m.status = Status{DataState: "disconnected", Error: "process_exited"}
		m.observed = time.Time{}
	default:
	}
}

func sameRuntime(a, b *config.Config) bool {
	x, y := a.RuView, b.RuView
	x.ReadOnly, y.ReadOnly = false, false
	x.AgentAccess, y.AgentAccess = false, false
	x.StorageMB, y.StorageMB = 0, 0
	return reflect.DeepEqual(x, y) && a.Directories.DataDir == b.Directories.DataDir && a.Docker.Host == b.Docker.Host && a.Docker.Enabled == b.Docker.Enabled && a.Docker.ReadOnly == b.Docker.ReadOnly
}

func storageLimited(c *config.Config) bool {
	usage, err := directoryBytes(filepath.Join(c.Directories.DataDir, "ruview", "data"))
	return err != nil || usage >= c.RuView.StorageMB<<20
}

func directoryBytes(root string) (int64, error) {
	var size int64
	err := filepath.WalkDir(root, func(_ string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("RuView data directory contains a link")
		}
		if !d.IsDir() {
			i, e := d.Info()
			if e != nil {
				return e
			}
			if !i.Mode().IsRegular() {
				return fmt.Errorf("RuView data contains a special file")
			}
			size += i.Size()
		}
		return nil
	})
	return size, err
}

func (m *Manager) Close() {
	m.mu.Lock()
	m.cancel()
	stop, done := m.stop, m.done
	m.mu.Unlock()
	if stop != nil {
		stop()
		<-done
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	_ = m.Stop(ctx)
	defaultManager.CompareAndSwap(m, nil)
}
