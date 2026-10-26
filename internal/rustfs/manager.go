package rustfs

import (
	"context"
	"crypto/rand"
	"errors"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"aurago/internal/config"
	"aurago/internal/dockerutil"
	"aurago/internal/security"
)

type Manager struct {
	prepared           bool
	preparedIAMTarget  string
	uncertain          bool
	external           context.Context
	externalCancel     context.CancelFunc
	externalGeneration uint64
	externalBlocked    bool
	managerState
	vault   secretStore
	logger  *slog.Logger
	docker  dockerEngine
	http    *http.Client
	names   resourceNames
	owner   context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	control chan struct{}
	blocked bool
	closed  bool
}

func NewManager(ctx context.Context, cfg *config.Config, vault *security.Vault, logger *slog.Logger) *Manager {
	var store secretStore
	if vault != nil {
		store = vault
	}
	return newManager(ctx, cfg, store, logger, nil, nil, defaultNames())
}
func newManager(ctx context.Context, cfg *config.Config, vault secretStore, logger *slog.Logger, engine dockerEngine, client *http.Client, names resourceNames) *Manager {
	if ctx == nil {
		ctx = context.Background()
	}
	if logger == nil {
		logger = slog.Default()
	}
	cfg = copyConfig(cfg)
	owner, cancel := context.WithCancel(ctx)
	desired, desiredCancel := context.WithCancel(owner)
	if engine == nil {
		engine = dockerutil.NewClient(cfg.Docker.Host, 30*time.Second)
	}
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second, Transport: &http.Transport{ResponseHeaderTimeout: 10 * time.Second, IdleConnTimeout: 30 * time.Second}}
	}
	state, err := loadState(vault)
	m := &Manager{managerState: managerState{cfg: cfg, desired: desired, desiredCancel: desiredCancel, generation: 1, fingerprint: configFingerprint(cfg), state: state, stateErr: err, jobs: map[string]*job{}}, vault: vault, logger: logger, docker: engine, http: client, names: names, owner: owner, cancel: cancel, control: resourceControl(names.container)}
	m.external, m.externalCancel = context.WithCancel(owner)
	m.externalGeneration = 1
	m.status = Status{State: "not_installed", AccessState: "disabled"}
	if err != nil {
		m.status.State = "recovery_required"
		m.status.Reason = ErrorCode(err)
	}
	m.wg.Add(1)
	go m.observeLoop()
	return m
}
func (m *Manager) revokeLocked() {
	m.desiredCancel()
	m.desired, m.desiredCancel = context.WithCancel(m.owner)
	m.generation++
	m.blocked = true
	m.status.Ready = false
}

// Configure runs inside config publication: it performs no I/O and cancels stale authority immediately.
func (m *Manager) Configure(next *config.Config) {
	next = copyConfig(next)
	m.mu.Lock()
	if m.closed || m.fingerprint == configFingerprint(next) && !m.prepared {
		m.mu.Unlock()
		return
	}
	preparedIAMTarget := m.preparedIAMTarget == configFingerprint(next)
	m.preparedIAMTarget = ""
	m.prepared = false
	previous := m.cfg
	if externalScopeFingerprint(previous) != externalScopeFingerprint(next) || m.externalBlocked {
		m.externalCancel()
		m.external, m.externalCancel = context.WithCancel(m.owner)
		m.externalGeneration++
		m.externalBlocked = false
	}
	wasReady := m.status.Ready
	m.revokeLocked()
	m.cfg = next
	m.fingerprint = configFingerprint(next)
	old := m.active
	if old != nil {
		old.cancel()
	}
	action := Action("")
	if !next.RustFS.Enabled && m.status.Running && next.Docker.Enabled && !next.Docker.ReadOnly {
		action = ActionStop
	}
	if m.state.Provisioned && m.state.DesiredRunning && m.status.Running && next.RustFS.Enabled && next.Docker.Enabled && !next.Docker.ReadOnly && m.state.PolicyHash != policyHash(generatePolicy(effectivePermissions(next))) {
		action = ActionStart
		if m.state.IAMPending && !preparedIAMTarget {
			action = ""
		}
	}
	if m.uncertain {
		action = ""
	}
	if action == "" {
		m.blocked = m.uncertain
		m.status.Ready = wasReady && targetFingerprint(previous) == targetFingerprint(next)
	} else {
		m.status.AccessState = "pending"
		m.wg.Add(1)
	}
	generation := m.generation
	m.mu.Unlock()
	if action != "" {
		go func() {
			defer m.wg.Done()
			if old != nil {
				select {
				case <-old.done:
				case <-m.owner.Done():
					return
				}
			}
			m.mu.Lock()
			current := !m.closed && m.generation == generation
			m.mu.Unlock()
			if current {
				_, _ = m.Begin(action)
			}
		}()
	}
}
func (m *Manager) PrepareReconfigure(ctx context.Context, next *config.Config) (result error) {
	defer func() {
		if ErrorCode(result) == "outcome_unknown" || ErrorCode(result) == "credential_revocation_failed" {
			m.mu.Lock()
			m.uncertain = true
			m.blocked = true
			m.status.Ready = false
			m.status.AccessState = "pending"
			m.status.Reason = ErrorCode(result)
			m.mu.Unlock()
		}
	}()
	next = copyConfig(next)
	m.mu.Lock()
	old := copyConfig(m.cfg)
	if m.state.Provisioned && next.RustFS.Bucket != m.state.InitialBucket {
		m.mu.Unlock()
		return failure("bucket_locked", nil)
	}
	if configFingerprint(old) == configFingerprint(next) {
		m.mu.Unlock()
		return nil
	}
	if externalScopeFingerprint(old) != externalScopeFingerprint(next) {
		m.externalCancel()
		m.external, m.externalCancel = context.WithCancel(m.owner)
		m.externalGeneration++
		m.externalBlocked = true
	}
	m.prepared = true
	m.revokeLocked()
	active := m.active
	if active != nil {
		active.cancel()
	}
	m.mu.Unlock()
	if active != nil {
		select {
		case <-active.done:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	if err := m.acquireControl(ctx); err != nil {
		return err
	}
	defer m.releaseControl()
	m.mu.Lock()
	s := m.state
	m.mu.Unlock()
	if (!old.Docker.Enabled || old.Docker.ReadOnly) && next.Docker.Enabled && !next.Docker.ReadOnly && old.Docker.Host == next.Docker.Host {
		return nil
	}
	if s.Instance == "" {
		return nil
	}
	info, found, err := m.inspectContainer(ctx, s)
	if err != nil {
		return err
	}
	if !found || !info.State.Running {
		return nil
	}
	mustStop := !next.RustFS.Enabled || !next.Docker.Enabled || next.Docker.ReadOnly || next.Docker.Host != old.Docker.Host || next.Runtime.IsDocker != old.Runtime.IsDocker
	if !old.Docker.Enabled || old.Docker.ReadOnly {
		return failure("docker_readonly", nil)
	}
	if s.Provisioned && !mustStop && (targetFingerprint(old) != targetFingerprint(next) || policyHash(generatePolicy(effectivePermissions(old))) != policyHash(generatePolicy(effectivePermissions(next)))) {
		endpoint, refreshErr := m.refreshOwnedEndpoint(ctx, old, s)
		if refreshErr != nil {
			return refreshErr
		}
		s.Endpoint = endpoint
		nextPolicyHash := policyHash(generatePolicy(effectivePermissions(next)))
		if s.IAMPending && s.PendingPolicyHash != nextPolicyHash {
			return failure("recovery_required", nil)
		}
		if err = m.inspectEstablishedIAM(ctx, old, s, s.IAMPending); err != nil {
			return err
		}
		if !s.IAMPending {
			s.IAMPending = true
			s.PendingPolicyHash = nextPolicyHash
		}
		if err = m.saveState(s); err != nil {
			return err
		}
		if err = m.disableChild(ctx, s); err != nil {
			return err
		}
		m.mu.Lock()
		m.preparedIAMTarget = configFingerprint(next)
		m.mu.Unlock()
	}
	if mustStop {
		return m.stopRuntime(ctx, s, true)
	}
	return nil
}
func (m *Manager) acquireControl(ctx context.Context) error {
	select {
	case m.control <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-m.owner.Done():
		return m.owner.Err()
	}
}
func (m *Manager) releaseControl() { <-m.control }
func (m *Manager) Begin(action Action) (Operation, error) {
	switch action {
	case ActionInstall, ActionStart, ActionStop, ActionRecreate, ActionRemove:
	default:
		return Operation{}, failure("invalid_action", nil)
	}
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return Operation{}, failure("manager_closed", nil)
	}
	cfg := copyConfig(m.cfg)
	if m.prepared && action != ActionStop && action != ActionRemove {
		m.mu.Unlock()
		return Operation{}, failure("configuration_pending", nil)
	}
	if !cfg.Docker.Enabled {
		m.mu.Unlock()
		return Operation{}, failure("docker_disabled", nil)
	}
	if cfg.Docker.ReadOnly {
		m.mu.Unlock()
		return Operation{}, failure("docker_readonly", nil)
	}
	if !cfg.RustFS.Enabled && action != ActionStop && action != ActionRemove {
		m.mu.Unlock()
		return Operation{}, failure("rustfs_disabled", nil)
	}
	if (action == ActionStart || action == ActionRecreate) && (m.state.Instance == "" || !m.state.Provisioned) {
		m.mu.Unlock()
		return Operation{}, failure("not_installed", nil)
	}
	if m.active != nil && action != ActionStop {
		m.mu.Unlock()
		return Operation{}, failure("lifecycle_busy", nil)
	}
	if m.active != nil {
		m.active.cancel()
	}
	m.revokeLocked()
	ctx, cancel := context.WithTimeout(m.owner, 15*time.Minute)
	j := &job{Operation: Operation{ID: rand.Text(), Action: action, State: "accepted", Phase: "checking", Created: time.Now().UTC(), Updated: time.Now().UTC()}, ctx: ctx, cancel: cancel, done: make(chan struct{})}
	j.generation = m.generation
	m.jobs[j.ID] = j
	m.active = j
	m.status.OperationID = j.ID
	m.pruneJobsLocked()
	m.wg.Add(1)
	receipt := j.Operation
	m.mu.Unlock()
	go m.run(j, cfg)
	return receipt, nil
}
func (m *Manager) pruneJobsLocked() {
	if len(m.jobs) <= 32 {
		return
	}
	var oldest *job
	for _, j := range m.jobs {
		select {
		case <-j.done:
			if oldest == nil || j.Created.Before(oldest.Created) {
				oldest = j
			}
		default:
		}
	}
	if oldest != nil {
		delete(m.jobs, oldest.ID)
	}
}
func (m *Manager) phase(j *job, phase string) {
	m.mu.Lock()
	j.State = "running"
	j.Phase = phase
	j.Updated = time.Now().UTC()
	m.mu.Unlock()
}
func (m *Manager) run(j *job, cfg *config.Config) {
	defer m.wg.Done()
	defer j.cancel()
	defer close(j.done)
	err := m.acquireControl(j.ctx)
	if err == nil {
		m.phase(j, "checking")
		switch j.Action {
		case ActionInstall, ActionStart, ActionRecreate:
			err = m.ensureRuntime(j.ctx, cfg, j)
		case ActionStop, ActionRemove:
			m.phase(j, "stopping")
			m.mu.Lock()
			s := m.state
			m.mu.Unlock()
			err = m.stopRuntime(j.ctx, s, true)
			if err == nil && j.Action == ActionRemove {
				m.phase(j, "removing")
				err = m.removeRuntime(j.ctx, s)
			}
		}
		m.releaseControl()
	}
	m.mu.Lock()
	j.Updated = time.Now().UTC()
	j.State = "succeeded"
	if err == nil {
		j.Phase = "complete"
	}
	if err != nil {
		j.err = err
		j.Error = ErrorCode(err)
		j.State = "failed"
		if errors.Is(err, context.Canceled) {
			j.State = "cancelled"
		}
		if ErrorCode(err) == "outcome_unknown" {
			j.State = "unknown"
			m.uncertain = true
		}
	}
	if m.active == j {
		m.active = nil
		m.status.OperationID = ""
		if err != nil {
			m.status.State = j.Error
			m.status.Reason = j.Error
			m.status.Ready = false
			m.status.AccessState = "pending"
		}
	}
	m.mu.Unlock()
	if err != nil {
		m.logger.Warn("[RustFS] lifecycle operation ended", "action", j.Action, "code", ErrorCode(err))
	}
}
func (m *Manager) Operation(id string) (Operation, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	j, ok := m.jobs[id]
	if !ok {
		return Operation{}, false
	}
	return j.Operation, true
}
func (m *Manager) Status() Status {
	m.mu.Lock()
	defer m.mu.Unlock()
	s := m.status
	s.Enabled = m.cfg.RustFS.Enabled
	s.AgentAccess = m.cfg.RustFS.AgentAccess
	s.Bucket = m.cfg.RustFS.Bucket
	s.Image = Image
	s.DataVolume = m.names.data
	s.CredentialVolume = m.names.credentials
	s.Generation = m.generationIDLocked()
	s.Updated = time.Now().UTC()
	if !s.Enabled && s.State == "not_installed" {
		s.State = "disabled"
	}
	return s
}
func (m *Manager) generationIDLocked() string {
	return hashValues(m.fingerprint, m.generation, m.state.ChildAccess, m.state.PolicyHash)
}
func (m *Manager) Availability(cfg *config.Config) config.FeatureAvailability {
	cfg = copyConfig(cfg)
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed || m.owner.Err() != nil {
		return config.FeatureAvailability{Reason: "manager_closed"}
	}
	if !cfg.S3.Enabled {
		return config.FeatureAvailability{Reason: "s3_disabled"}
	}
	if targetFingerprint(cfg) != targetFingerprint(m.cfg) {
		return config.FeatureAvailability{Reason: "stale_target"}
	}
	if cfg.S3.Provider == config.S3ProviderExternal {
		if m.externalBlocked {
			return config.FeatureAvailability{Reason: "permission_pending"}
		}
		return config.FeatureAvailability{Available: true}
	}
	if !cfg.RustFS.Enabled {
		return config.FeatureAvailability{Reason: "rustfs_disabled"}
	}
	if !cfg.RustFS.AgentAccess {
		return config.FeatureAvailability{Reason: "agent_access_disabled"}
	}
	if m.blocked || !m.status.Ready || m.status.AccessState != "ready" {
		return config.FeatureAvailability{Reason: "permission_pending"}
	}
	if !hasDataPermissions(intersectPermissions(effectivePermissions(cfg), effectivePermissions(m.cfg))) {
		return config.FeatureAvailability{Reason: "permission_denied"}
	}
	if m.externalBlocked {
		return config.FeatureAvailability{Reason: "permission_pending"}
	}
	return config.FeatureAvailability{Available: true}
}
func (m *Manager) Shutdown(ctx context.Context) error {
	m.mu.Lock()
	if !m.closed {
		m.closed = true
		m.revokeLocked()
		m.cancel()
		if m.active != nil {
			m.active.cancel()
		}
	}
	m.mu.Unlock()
	done := make(chan struct{})
	go func() { m.wg.Wait(); close(done) }()
	select {
	case <-done:
		m.docker.CloseIdleConnections()
		m.http.CloseIdleConnections()
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
func (m *Manager) observeLoop() {
	defer m.wg.Done()
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		ctx, cancel := context.WithTimeout(m.owner, 30*time.Second)
		m.observe(ctx)
		cancel()
		select {
		case <-m.owner.Done():
			return
		case <-ticker.C:
		}
	}
}
func (m *Manager) AcquireS3(ctx context.Context, cfg *config.Config, agent bool) (*config.RustFSS3Lease, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	cfg = copyConfig(cfg)
	m.mu.Lock()
	if m.closed || m.owner.Err() != nil || ctx.Err() != nil {
		m.mu.Unlock()
		return nil, failure("manager_closed", nil)
	}
	if !cfg.S3.Enabled || !m.cfg.S3.Enabled {
		m.mu.Unlock()
		return nil, failure("s3_disabled", nil)
	}
	if targetFingerprint(cfg) != targetFingerprint(m.cfg) || cfg.Agent.AllowFilesystemWrite && !m.cfg.Agent.AllowFilesystemWrite || localRootsFingerprint(cfg) != localRootsFingerprint(m.cfg) || cfg.S3.Provider == config.S3ProviderExternal && !cfg.S3.ReadOnly && m.cfg.S3.ReadOnly {
		m.mu.Unlock()
		return nil, failure("stale_target", nil)
	}
	generation := m.generationIDLocked()
	desired := m.desired
	managed := cfg.S3.Provider == config.S3ProviderRustFS
	var managedState persistedState
	if !managed {
		generation = m.externalIDLocked()
		desired = m.external
		if m.externalBlocked {
			m.mu.Unlock()
			return nil, failure("permission_pending", nil)
		}
	}
	lease := &config.RustFSS3Lease{Managed: managed, Endpoint: cfg.S3.Endpoint, Region: cfg.S3.Region, Bucket: cfg.S3.Bucket, AccessKey: cfg.S3.AccessKey, SecretKey: cfg.S3.SecretKey, Generation: generation}
	if managed {
		if !cfg.RustFS.Enabled || !m.cfg.RustFS.Enabled {
			m.mu.Unlock()
			return nil, failure("rustfs_disabled", nil)
		}
		if agent && (!cfg.RustFS.AgentAccess || !m.cfg.RustFS.AgentAccess) {
			m.mu.Unlock()
			return nil, failure("agent_access_disabled", nil)
		}
		if m.blocked || !m.status.Ready || agent && m.status.AccessState != "ready" {
			m.mu.Unlock()
			return nil, failure("permission_pending", nil)
		}
		lease.Permissions = intersectPermissions(effectivePermissions(cfg), effectivePermissions(m.cfg))
		managedState = m.state
		lease.Region = Region
		lease.Bucket = cfg.RustFS.Bucket
	}
	m.mu.Unlock()
	if managed {
		endpoint, err := m.refreshOwnedEndpoint(ctx, cfg, managedState)
		if err != nil {
			return nil, err
		}
		m.mu.Lock()
		if m.closed || m.owner.Err() != nil || ctx.Err() != nil || m.generationIDLocked() != generation || m.state.Instance != managedState.Instance || m.state.ContainerID != managedState.ContainerID || m.blocked || !m.status.Ready {
			m.mu.Unlock()
			return nil, failure("stale_target", nil)
		}
		if endpoint != m.state.Endpoint {
			m.updateManagedEndpointLocked(endpoint)
		}
		desired = m.desired
		generation = m.generationIDLocked()
		lease.Endpoint = endpoint
		lease.AccessKey = m.state.ChildAccess
		lease.SecretKey = m.state.ChildSecret
		lease.Permissions = intersectPermissions(effectivePermissions(cfg), effectivePermissions(m.cfg))
		lease.Generation = generation
		m.mu.Unlock()
	}
	if !managed && (lease.AccessKey == "" || lease.SecretKey == "") {
		if m.vault == nil {
			return nil, failure("s3_credentials_unavailable", nil)
		}
		access, e1 := m.vault.ReadSecret("s3_access_key")
		secret, e2 := m.vault.ReadSecret("s3_secret_key")
		if e1 != nil || e2 != nil {
			return nil, failure("s3_credentials_unavailable", nil)
		}
		if lease.AccessKey != "" && lease.AccessKey != access || lease.SecretKey != "" && lease.SecretKey != secret {
			return nil, failure("stale_target", nil)
		}
		lease.AccessKey = access
		lease.SecretKey = secret
	}
	if lease.AccessKey == "" || lease.SecretKey == "" {
		return nil, failure("s3_credentials_unavailable", nil)
	}
	security.RegisterSensitive(lease.AccessKey)
	security.RegisterSensitive(lease.SecretKey)
	callCtx, cancel := context.WithCancel(ctx)
	stop := context.AfterFunc(desired, cancel)
	lease.Context = callCtx
	lease.Release = func() { stop(); cancel() }
	checkLocked := func() error {
		current := m.generationIDLocked()
		blocked := m.blocked
		if !managed {
			current = m.externalIDLocked()
			blocked = m.externalBlocked
		}
		if m.closed || m.owner.Err() != nil || blocked || generation != current || callCtx.Err() != nil {
			return failure("stale_target", nil)
		}
		return nil
	}
	lease.Check = func() error { m.mu.Lock(); defer m.mu.Unlock(); return checkLocked() }
	lease.Publish = func(publish func() error) error {
		m.mu.Lock()
		defer m.mu.Unlock()
		if err := checkLocked(); err != nil {
			return err
		}
		return publish()
	}
	if err := lease.Check(); err != nil {
		lease.Release()
		return nil, err
	}
	return lease, nil
}

func (m *Manager) observe(ctx context.Context) {
	m.mu.Lock()
	skip := m.closed || m.prepared || m.uncertain || m.state.Instance == "" || m.active != nil
	m.mu.Unlock()
	if skip {
		return
	}
	_, err := m.Preflight(ctx)
	m.mu.Lock()
	cfg := copyConfig(m.cfg)
	s := m.state
	active := m.active
	running := m.status.Running
	closed := m.closed || m.prepared || m.uncertain
	m.mu.Unlock()
	if closed || s.Instance == "" || !s.DesiredRunning || active != nil {
		return
	}
	if !cfg.Docker.Enabled || cfg.Docker.ReadOnly {
		m.mu.Lock()
		if !cfg.RustFS.Enabled {
			m.status.State = "stop_unconfirmed"
			m.status.Reason = "stop_unconfirmed"
		} else {
			m.status.AccessState = "pending"
		}
		m.mu.Unlock()
		return
	}
	if err != nil {
		return
	}
	if !cfg.RustFS.Enabled && running {
		_, _ = m.Begin(ActionStop)
		return
	}
	if cfg.RustFS.Enabled && running && s.Provisioned && s.PolicyHash != policyHash(generatePolicy(effectivePermissions(cfg))) {
		_, _ = m.Begin(ActionStart)
	}
}
func (m *Manager) externalIDLocked() string {
	return hashValues(externalScopeFingerprint(m.cfg), m.externalGeneration)
}
func (m *Manager) checkJob(j *job, cfg *config.Config) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed || m.prepared || m.active != j || m.generation != j.generation || j.ctx.Err() != nil || m.fingerprint != configFingerprint(cfg) {
		return failure("stale_target", j.ctx.Err())
	}
	return nil
}

func (m *Manager) publishReady(j *job, cfg *config.Config) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed || m.prepared || m.active != j || m.generation != j.generation || j.ctx.Err() != nil || m.fingerprint != configFingerprint(cfg) {
		return failure("stale_target", j.ctx.Err())
	}
	m.uncertain = false
	m.blocked = false
	m.status.Ready = true
	return nil
}

var runtimeControls sync.Map

func resourceControl(name string) chan struct{} {
	value, _ := runtimeControls.LoadOrStore(name, make(chan struct{}, 1))
	return value.(chan struct{})
}
