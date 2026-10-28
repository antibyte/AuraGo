package ruview

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"aurago/internal/config"
	"aurago/internal/security"
)

type Status struct {
	Enabled            bool       `json:"enabled"`
	ReadOnly           bool       `json:"readonly"`
	AgentAccess        bool       `json:"agent_access"`
	Mode               string     `json:"mode"`
	Source             string     `json:"source"`
	Revision           string     `json:"revision"`
	RevisionVerified   bool       `json:"revision_verified"`
	Ready              bool       `json:"ready"`
	DataState          string     `json:"data_state"`
	Signal             string     `json:"signal"`
	Simulated          bool       `json:"simulated"`
	Fresh              bool       `json:"fresh"`
	WaitingForFrames   bool       `json:"waiting_for_frames"`
	ObservedAt         *time.Time `json:"observed_at"`
	FrameAgeMS         *int64     `json:"last_frame_age_ms"`
	Error              string     `json:"error,omitempty"`
	StorageLimit       bool       `json:"storage_limit"`
	UDPEnabled         bool       `json:"udp_enabled"`
	UDPReady           bool       `json:"udp_ready"`
	UDPScope           string     `json:"udp_scope"`
	ReceiverConfigured bool       `json:"receiver_configured"`
	RecordingSession   string     `json:"recording_session"`
	RecordingAck       string     `json:"recording_ack,omitempty"`

	Runtime            string     `json:"runtime"`              // Runtime* (lifecycle.go); always set
	Phase              string     `json:"phase"`                // Phase* or ""
	PhaseSince         *time.Time `json:"phase_since"`          // when Phase was entered; null when Phase == ""
	ErrorCode          string     `json:"error_code"`           // Code* or ""
	ErrorDetail        string     `json:"error_detail"`         // scrubbed, at most 240 bytes, no URL/path; "" if none
	LastFreshAt        *time.Time `json:"last_fresh_at"`        // last stored probe with fresh && !simulated; reset only by a runtime-relevant Configure
	StopReason         string     `json:"stop_reason"`          // Stop* or ""
	StoppedAt          *time.Time `json:"stopped_at"`           // time of StopReason
	StorageUsedBytes   *int64     `json:"storage_used_bytes"`   // last successful walk; null for external, before the first walk, or after a failed walk
	StorageBudgetBytes *int64     `json:"storage_budget_bytes"` // StorageMB<<20 for docker/native; null for external
	RecordingAt        *time.Time `json:"recording_at"`         // time of the last confirmed (2xx) forwarded recording start/stop
	Hints              []string   `json:"hints"`                // open hint names, sorted; [] when none (never null)
}

type Manager struct {
	config           func() *config.Config
	vault            *security.Vault
	logger           *slog.Logger
	mu               sync.Mutex
	opMu             opLock
	hintMu           sync.Mutex
	nodesMu          sync.Mutex
	status           Status
	observed         time.Time
	generation       context.Context
	cancel           context.CancelFunc
	previous         configKey
	lifetime         context.Context
	stop             context.CancelFunc
	done             chan struct{}
	process          *nativeProcess
	managed          bool
	runningConfig    *config.Config
	recordingSession string
	recordingAck     string
	hintFn           func(string, bool)
	hintOpen         map[string]bool
	budgetLimited    bool

	// Lifecycle (see the lock protocol in lifecycle.go). All guarded by mu.
	op          string // "", opStarting or opInstalling: the single long-work slot
	phase       string // Phase* of the work in flight, or of the wait for the first ready probe
	phaseSince  time.Time
	stopReason  string // Stop* once the runtime stopped, exited or was cancelled; cleared by a start
	stoppedAt   time.Time
	lastFreshAt time.Time
	recordingAt time.Time
	// startFailed marks status.Error as the outcome of a failed start or install.
	// Such an error is sticky: a probe or an unrelated Configure keeps it while
	// the runtime is not running; an explicit Start, Stop or Install or a
	// runtime-relevant Configure replaces it.
	startFailed bool
	runSeq      uint64 // counts commits; runID is the commit that owns the managed runtime
	runID       uint64
	lockWait    time.Duration // how long Start, Install and watch wait for opMu
	stopWait    time.Duration // how long stopRuntime waits for a killed process

	// Throttled storage and Docker sampling (storage.go, lifecycle.go).
	storageAt    time.Time
	dockerAt     time.Time
	storageEvery time.Duration
	dockerEvery  time.Duration
	storageUsed  int64
	storageKnown bool
	storageErr   bool
	walkStorage  func(context.Context, string) (int64, error)

	// stopNote carries a stop outcome from stopRuntime to flushStopNote.
	stopNote stopNote
}

var defaultManager atomic.Pointer[Manager]

func SetDefault(m *Manager) { defaultManager.Store(m) }
func Default() *Manager     { return defaultManager.Load() }

func New(snapshot func() *config.Config, vault *security.Vault, logger *slog.Logger) *Manager {
	ctx, cancel := context.WithCancel(context.Background())
	return &Manager{
		config: snapshot, vault: vault, logger: logger, generation: ctx, cancel: cancel, previous: configKeyOf(snapshot()),
		opMu:         make(opLock, 1),
		storageEvery: 30 * time.Second, dockerEvery: 5 * time.Second, walkStorage: directoryBytes,
		lockWait: defaultLockWait, stopWait: defaultStopWait,
	}
}

// runtimeFields keeps only the settings that change what the managed runtime
// is or how it is reached. Agent access, read-only and the storage budget are
// policy that is checked per request, so they are zeroed (M27).
func runtimeFields(c config.RuViewConfig) config.RuViewConfig {
	c.ReadOnly, c.AgentAccess, c.StorageMB = false, false, 0
	return c
}

// configKey is everything that makes a managed runtime what it is: the RuView
// section plus the data directory and the Docker settings it is created with.
type configKey struct {
	RuView         config.RuViewConfig
	DataDir        string
	DockerHost     string
	DockerEnabled  bool
	DockerReadOnly bool
}

func configKeyOf(c *config.Config) configKey {
	return configKey{RuView: c.RuView, DataDir: c.Directories.DataDir, DockerHost: c.Docker.Host, DockerEnabled: c.Docker.Enabled, DockerReadOnly: c.Docker.ReadOnly}
}

// runtime drops the per-request policy settings (runtimeFields).
func (k configKey) runtime() configKey {
	k.RuView = runtimeFields(k.RuView)
	return k
}

// Configure revokes active streams, probes and a running start immediately when
// a runtime-relevant setting changed; it never installs or starts a runtime.
// Runtime-relevant means the RuView section without the per-request policy
// (agent access, read-only, storage budget: M27, which change nothing that was
// already admitted) or the data directory or Docker settings the runtime is
// created with (sameRuntime uses the same key).
// A lifecycle error survives a configuration change: only an explicit Start or
// Stop confirms the runtime state, so toggling a setting must not re-enable
// health probing of a runtime whose stop was never confirmed.
func (m *Manager) Configure() {
	c := configKeyOf(m.config())
	m.mu.Lock()
	defer m.mu.Unlock()
	if reflect.DeepEqual(m.previous, c) {
		return
	}
	relevant := !reflect.DeepEqual(m.previous.runtime(), c.runtime())
	m.previous = c
	if !relevant {
		return
	}
	if m.op == opStarting || m.managed {
		// The cancelled start or the next watch stops the runtime; record why.
		m.stopReason, m.stoppedAt = StopConfigChanged, time.Now().UTC()
	}
	m.cancel()
	m.generation, m.cancel = context.WithCancel(context.Background())
	previous := m.status
	m.status = Status{}
	if lifecycleError(previous.Error) {
		m.status.Error, m.status.ErrorCode, m.status.ErrorDetail = previous.Error, previous.ErrorCode, previous.ErrorDetail
	}
	m.observed, m.lastFreshAt = time.Time{}, time.Time{}
	m.startFailed = false // a start failure belongs to the configuration that failed
	if m.op == "" {
		m.clearPhaseLocked()
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
	running := m.runningLocked()
	observed := m.observed
	session, ack := m.recordingSession, m.recordingAck
	s.Runtime = m.runtimeLocked(c.Mode, running)
	s.Phase = m.phase
	s.PhaseSince = timePointer(m.phaseSince)
	s.StopReason = m.stopReason
	s.StoppedAt = timePointer(m.stoppedAt)
	s.LastFreshAt = timePointer(m.lastFreshAt)
	s.RecordingAt = timePointer(m.recordingAt)
	s.Hints = m.openHintsLocked()
	storageKnown, storageUsed, storageErr := m.storageKnown, m.storageUsed, m.storageErr
	m.mu.Unlock()
	s.Enabled = c.Enabled
	s.ReadOnly = c.ReadOnly
	s.AgentAccess = c.AgentAccess
	s.Mode = c.Mode
	s.Revision = Revision
	s = applyPassiveStatus(s, c, running, session, ack)
	if s.Source == "" {
		s.Source = c.Source
	}
	if s.DataState == "" {
		s.DataState = "unchecked"
	}
	if c.Mode != "external" {
		budget := c.StorageMB << 20
		s.StorageBudgetBytes = &budget
		if storageKnown && !storageErr {
			s.StorageUsedBytes = &storageUsed
		}
	}
	if s.ErrorCode == "" && s.Error != "" {
		s.ErrorCode = knownCodeOr(s.Error, CodeUnknown)
	}
	if s.ErrorCode == "" && storageErr && c.Mode != "external" {
		// Lowest precedence: it never hides a lifecycle, start or probe error.
		s.ErrorCode = CodeStorageUnreadable
	}
	if !c.Enabled {
		s.Ready = false
		s.Fresh = false
		s.WaitingForFrames = false
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

// timePointer returns nil for the zero time, so JSON shows null.
func timePointer(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}

// runningLocked reports whether this process owns a managed runtime that has
// not exited. It only reads: reaping an exited native process mutates state and
// needs opMu, so passive readers must not do it. Called with mu held.
// For Docker this means "started by this process and not stopped", not
// "container alive": the watch loop asks the Engine (inspectExit, M24) and
// marks a container that ended on its own as exited.
func (m *Manager) runningLocked() bool {
	if !m.managed {
		return false
	}
	if m.process != nil {
		select {
		case <-m.process.done:
			return false
		default:
		}
	}
	return true
}

// applyPassiveStatus copies saved configuration, whether this process runs the
// managed runtime, and this process's recording marker. It never dials, resolves
// DNS, or reads the node inventory.
func applyPassiveStatus(s Status, c config.RuViewConfig, running bool, session, ack string) Status {
	s.ReceiverConfigured = config.RuViewPrivateIPv4(c.ReceiverIP)
	s.UDPEnabled = c.Mode != "external" && c.UDPEnabled
	s.UDPReady = false
	switch {
	case !c.Enabled:
		s.UDPScope = "off"
	case c.Mode == "external":
		s.UDPScope = "external"
		s.UDPEnabled = false
	default:
		s.UDPScope = "managed"
		// Saved settings alone do not receive packets: a stopped or exited runtime listens on nothing.
		s.UDPReady = running && s.UDPEnabled && config.RuViewPrivateIPv4(c.UDPBind) && len(c.UDPAllow) > 0
	}
	s.RecordingSession = session
	if s.RecordingSession == "" {
		s.RecordingSession = "unknown"
	}
	s.RecordingAck = ack
	return s
}

// noteRecording remembers only a forwarded start or stop. A local reject never
// reaches this helper. A non-2xx or a missing status leaves the last confirmed
// session in place and marks the acknowledgement unconfirmed.
func (m *Manager) noteRecording(p string, code int) {
	if p != "/api/v1/recording/start" && p != "/api/v1/recording/stop" {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if code >= 200 && code < 300 {
		m.recordingSession = "stopped"
		if p == "/api/v1/recording/start" {
			m.recordingSession = "started"
		}
		m.recordingAck = ""
		m.recordingAt = time.Now().UTC()
		return
	}
	m.recordingAck = "unconfirmed"
}

func (m *Manager) Test(parent context.Context) (Status, error) {
	m.mu.Lock()
	generation := m.generation
	m.mu.Unlock()
	return m.test(parent, generation)
}

func (m *Manager) test(parent context.Context, generation context.Context) (Status, error) {
	c := m.config()
	if !c.RuView.Enabled {
		return m.Status(), fail(CodeDisabled, "RuView is disabled", nil)
	}
	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	stop := context.AfterFunc(generation, cancel)
	defer stop()
	if generation.Err() != nil || ctx.Err() != nil {
		return m.Status(), context.Canceled
	}
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
			err = fail(CodeContractUnsupported, "RuView status contract is unsupported", nil)
		} else {
			s.Ready = true
			s.Source = strings.SplitN(raw.Source, ":", 2)[0]
			s.DataState = raw.State
			s.FrameAgeMS = raw.Age
			s.Simulated = raw.State == "synthetic"
			s.Fresh = raw.State == "live_unverified" && raw.Age != nil && *raw.Age >= 0 && *raw.Age <= 10000 && !raw.Waiting
			s.WaitingForFrames = raw.Waiting
			if s.Simulated {
				s.Signal = "demo"
			} else if s.Source == "wifi" {
				s.Signal = "RSSI"
			} else if s.Source == "esp32" {
				s.Signal = "CSI"
			}
			if s.Source != c.RuView.Source {
				s.Fresh = false
				s.Error, s.ErrorCode = CodeSourceMismatch, CodeSourceMismatch
			}
			if s.Simulated && c.RuView.Source != "simulated" {
				s.Fresh = false
				s.Error, s.ErrorCode = CodeUnexpectedSimulation, CodeUnexpectedSimulation
			}
		}
	}
	if err != nil {
		e := AsError(err)
		s.Error, s.ErrorCode, s.ErrorDetail = e.Error(), e.Code, e.Detail
	}
	var previous Status
	var stored bool
	m.mu.Lock()
	current := ctx.Err() == nil && generation == m.generation && generation.Err() == nil
	if current {
		previous = m.status
		s.StorageLimit = m.status.StorageLimit
		// A probe reports what the service answers; it cannot prove the runtime
		// lifecycle recovered. A lifecycle failure keeps its code (and keeps the
		// watch guard on "stop_failed" effective) until an explicit Start or Stop
		// replaces the status. So does the failure of a start or install while
		// nothing runs: the probe would only add "unreachable" on top of the real
		// cause. The rest of the probe data is still stored.
		if lifecycleError(previous.Error) || m.startFailed && !m.runningLocked() {
			s.Error, s.ErrorCode, s.ErrorDetail = previous.Error, previous.ErrorCode, previous.ErrorDetail
		}
		m.status = s
		m.observed = now
		if s.Fresh && !s.Simulated {
			m.lastFreshAt = now
		}
		if s.Ready && m.phase == PhaseWaitingReady {
			m.clearPhaseLocked()
		}
		stored = true
	}
	m.mu.Unlock()
	if stored {
		m.noteSensing(previous, s, c.RuView)
	}
	if !current {
		return m.Status(), context.Canceled
	}
	return m.Status(), err
}

// lifecycleError reports the status error codes written by stopRuntime and
// markExitedLocked. Only start and stopRuntime replace them.
func lifecycleError(code string) bool {
	return code == CodeStopFailed || code == CodeProcessExited
}

func (m *Manager) Read(ctx context.Context, operation string) (any, error) {
	c := m.config()
	if !c.RuView.Enabled || !c.RuView.AgentAccess {
		return nil, fmt.Errorf("RuView agent read access is disabled")
	}
	bound, done := m.bound(ctx)
	defer done()
	if operation == "status" {
		return m.Test(bound)
	}
	if operation == "presence" {
		result, err := m.readPresence(bound)
		if err != nil {
			return nil, err
		}
		return result, nil
	}
	p, ok := readOperations[operation]
	if !ok {
		return nil, fmt.Errorf("unsupported RuView read operation")
	}
	data, err := m.request(bound, c, "GET", p, nil)
	if err != nil {
		return nil, err
	}
	envelope := payloadRead{Data: data, Revision: Revision}
	m.attachStatus(bound, &envelope.Source, &envelope.Fresh, &envelope.Simulated, &envelope.DataState, &envelope.StatusError)
	envelope.ObservedAt = time.Now().UTC()
	return envelope, nil
}

func (m *Manager) StartBackground(parent context.Context) {
	m.mu.Lock()
	if m.stop != nil {
		m.mu.Unlock()
		return
	}
	m.lifetime, m.stop = context.WithCancel(parent)
	ctx := m.lifetime
	generation := m.generation
	m.done = make(chan struct{})
	m.mu.Unlock()
	go func() {
		defer close(m.done)
		c := m.config().RuView
		if c.Enabled && !c.ReadOnly && c.Mode != "external" {
			startup, cancel := context.WithTimeout(ctx, 10*time.Minute)
			err := m.start(startup, generation)
			cancel()
			if err != nil && ctx.Err() == nil && generation.Err() == nil {
				m.logger.Warn("RuView automatic start failed", "error", err)
			}
		}
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		var checked time.Time
		// Touched only by the single probe goroutine at a time (probeDone orders them).
		var listState string
		probeDone := make(chan struct{}, 1)
		probing := false
		defer func() {
			if probing {
				<-probeDone
			}
		}()
		for {
			select {
			case <-ctx.Done():
				return
			case <-probeDone:
				probing = false
				checked = time.Now()
			case <-ticker.C:
				if generation := m.watch(ctx); generation != nil && !probing && time.Since(checked) >= 3*time.Second {
					probing = true
					// Network latency must not delay process reaping or the storage limit.
					go func() {
						probe, cancel := context.WithCancel(ctx)
						stop := context.AfterFunc(generation, cancel)
						defer cancel()
						defer stop()
						status, err := m.test(probe, generation)
						if err == nil && status.Ready && status.Source == "esp32" && !status.Simulated && generation.Err() == nil {
							list, err := m.Nodes(probe)
							if err != nil && probe.Err() == nil {
								m.logger.Warn("RuView device inventory refresh failed", "error", err)
							} else if err == nil {
								// Warn on entering a broken state, not on every 3-second probe.
								if broken := list.LiveState == liveStateUnreachable || list.LiveState == liveStateInvalid; broken && list.LiveState != listState {
									m.logger.Warn("RuView live device list unavailable", "live_state", list.LiveState)
								}
								listState = list.LiveState
							}
						}
						probeDone <- struct{}{}
					}()
				}
			}
		}
	}()
}

// watch is one tick of the background loop. It stops a runtime whose
// configuration no longer matches, notices an exit (native process or Docker
// container), enforces the storage budget and ends the wait for the first
// ready probe. It returns the generation to probe under, or nil when there is
// nothing to probe. Slow work (the Engine inspect, the storage walk, the
// recording stop) runs outside opMu so it never delays Stop or Start. The
// generation is captured with the runtime under mu and re-checked at the end:
// a Stop or Configure in the unlocked part revokes it, and the tick then
// schedules no probe (a probe under a newer generation would report a
// deliberately stopped service as unreachable).
func (m *Manager) watch(ctx context.Context) context.Context {
	m.Configure()
	if m.opMu.acquire(ctx, m.lockWait) != nil {
		return nil
	}
	m.mu.Lock()
	m.reapNativeLocked()
	managed, running, run, stopFailed := m.managed, m.runningConfig, m.runID, m.status.Error == CodeStopFailed
	generation := m.generation
	m.mu.Unlock()
	if !managed || stopFailed || generation.Err() != nil {
		m.opMu.release()
		return nil
	}
	c := m.config()
	if !c.RuView.Enabled || running == nil || !sameRuntime(running, c) {
		// Bounded: a hung Engine or process must not wedge the loop that reaps exits.
		stopCtx, cancel := context.WithTimeout(ctx, watchStopTimeout)
		_ = m.stopRuntime(stopCtx, StopConfigChanged) // M27: the stop reason survives for the UI
		cancel()
		m.opMu.release()
		m.flushStopNote()
		return nil
	}
	m.opMu.release()
	if exited, detail := m.inspectExit(ctx, running); exited { // M24, outside opMu
		if m.opMu.acquire(ctx, m.lockWait) != nil {
			return nil
		}
		m.mu.Lock()
		if m.managed && m.runID == run { // a Stop, or a Stop and a new Start, in between wins
			m.markExitedLocked(detail)
		}
		m.mu.Unlock()
		m.opMu.release()
		return nil
	}
	m.mu.Lock()
	if m.phase == PhaseWaitingReady && time.Since(m.phaseSince) > waitingReadyTimeout {
		m.clearPhaseLocked()
	}
	if m.managed && m.op == "" && m.stopReason == StopConfigChanged {
		// Configure recorded a change that was reverted before this tick. The
		// runtime still matches its configuration, so nothing stopped.
		m.stopReason, m.stoppedAt = "", time.Time{}
	}
	m.mu.Unlock()
	if used, sampled, err := m.sampleStorage(ctx, c); sampled { // M28: at most every storageEvery, outside opMu
		limited := err == nil && used >= c.RuView.StorageMB<<20
		if limited {
			bounded, cancel := context.WithTimeout(ctx, 5*time.Second)
			_, _ = m.request(bounded, c, "POST", "/api/v1/recording/stop", nil)
			cancel()
		}
		if ctx.Err() != nil {
			return nil // shutting down: a cancelled walk says nothing about the storage
		}
		m.mu.Lock()
		m.status.StorageLimit = limited
		m.mu.Unlock()
		if err == nil { // an unreadable directory neither opens nor closes the hint
			m.noteStorage(c.RuView.Mode, limited)
		}
	}
	if generation.Err() != nil {
		return nil
	}
	return generation
}

// reapNativeLocked notices a native process that ended. Called with mu held,
// and opMu held by lifecycle callers. The current pointer ensures an old
// process completion cannot clear a replacement process.
func (m *Manager) reapNativeLocked() {
	if m.process == nil {
		return
	}
	select {
	case <-m.process.done:
		m.markExitedLocked(exitDetail(m.process.err))
	default:
	}
}

// sameRuntime reports whether two configurations describe the same managed
// runtime. Settings that are checked per request (runtimeFields) do not count.
func sameRuntime(a, b *config.Config) bool {
	return reflect.DeepEqual(configKeyOf(a).runtime(), configKeyOf(b).runtime())
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
	_ = m.stopWithReason(ctx, StopAuraGoShutdown)
	defaultManager.CompareAndSwap(m, nil)
}
