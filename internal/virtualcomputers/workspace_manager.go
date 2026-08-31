package virtualcomputers

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"path"
	"sort"
	"strings"
	"sync"
	"time"

	"aurago/internal/credentials"
	"aurago/internal/security"
	"aurago/internal/uid"
)

const (
	workspaceOutputPageBytes = 64 * 1024
	workspaceControlLease    = 2 * time.Minute
)

type SecretReader interface {
	ReadSecret(key string) (string, error)
}

type GrantableCredential struct {
	ID              string   `json:"id"`
	Name            string   `json:"name"`
	Type            string   `json:"type"`
	Host            string   `json:"host,omitempty"`
	Username        string   `json:"username,omitempty"`
	Description     string   `json:"description,omitempty"`
	AvailableFields []string `json:"available_fields"`
}

type WorkspaceCredentialProvider interface {
	ListGrantable(ctx context.Context) ([]GrantableCredential, error)
	Resolve(ctx context.Context, credentialID string, requestedFields []string) (map[string]string, error)
}

type DatabaseWorkspaceCredentialProvider struct {
	db    *sql.DB
	vault SecretReader
}

func NewDatabaseWorkspaceCredentialProvider(db *sql.DB, vault SecretReader) *DatabaseWorkspaceCredentialProvider {
	return &DatabaseWorkspaceCredentialProvider{db: db, vault: vault}
}

func (p *DatabaseWorkspaceCredentialProvider) ListGrantable(_ context.Context) ([]GrantableCredential, error) {
	records, err := credentials.ListVirtualComputerAccessible(p.db)
	if err != nil {
		return nil, err
	}
	result := make([]GrantableCredential, 0, len(records))
	for _, record := range records {
		result = append(result, grantableCredentialMetadata(record))
	}
	return result, nil
}

func (p *DatabaseWorkspaceCredentialProvider) Resolve(_ context.Context, credentialID string, requestedFields []string) (map[string]string, error) {
	if p == nil || p.db == nil || p.vault == nil {
		return nil, fmt.Errorf("credential store or vault is unavailable")
	}
	record, err := credentials.GetByID(p.db, strings.TrimSpace(credentialID))
	if err != nil {
		return nil, err
	}
	if !record.AllowVirtualComputers {
		return nil, fmt.Errorf("credential is not grantable to virtual computers")
	}
	available := map[string]string{"username": record.Username}
	vaultKeys := map[string]string{
		"password":    record.PasswordVaultID,
		"certificate": record.CertificateVaultID,
		"token":       record.TokenVaultID,
	}
	fields := normalizeGrantFields(requestedFields, record)
	resolved := make(map[string]string, len(fields))
	for _, field := range fields {
		if value, ok := available[field]; ok {
			resolved[field] = value
			continue
		}
		vaultKey := strings.TrimSpace(vaultKeys[field])
		if vaultKey == "" {
			return nil, fmt.Errorf("credential field %q is unavailable", field)
		}
		value, err := p.vault.ReadSecret(vaultKey)
		if err != nil {
			return nil, fmt.Errorf("read credential field %q: %w", field, err)
		}
		security.RegisterSensitive(value)
		resolved[field] = value
	}
	return resolved, nil
}

func grantableCredentialMetadata(record credentials.Record) GrantableCredential {
	fields := make([]string, 0, 4)
	if strings.TrimSpace(record.Username) != "" {
		fields = append(fields, "username")
	}
	if record.HasPassword {
		fields = append(fields, "password")
	}
	if record.HasCertificate {
		fields = append(fields, "certificate")
	}
	if record.HasToken {
		fields = append(fields, "token")
	}
	return GrantableCredential{
		ID: record.ID, Name: record.Name, Type: record.Type, Host: record.Host,
		Username: record.Username, Description: record.Description, AvailableFields: fields,
	}
}

func normalizeGrantFields(requested []string, record credentials.Record) []string {
	allowed := map[string]bool{
		"username":    strings.TrimSpace(record.Username) != "",
		"password":    record.HasPassword,
		"certificate": record.HasCertificate,
		"token":       record.HasToken,
	}
	if len(requested) == 0 {
		requested = []string{"username", "password", "token", "certificate"}
	}
	seen := make(map[string]struct{}, len(requested))
	fields := make([]string, 0, len(requested))
	for _, field := range requested {
		field = strings.ToLower(strings.TrimSpace(field))
		if !allowed[field] {
			continue
		}
		if _, ok := seen[field]; ok {
			continue
		}
		seen[field] = struct{}{}
		fields = append(fields, field)
	}
	sort.Strings(fields)
	return fields
}

type WorkspaceManagerOptions struct {
	CredentialProvider WorkspaceCredentialProvider
	ClientFactory      func(ToolConfig) (*Client, error)
	TransportFactory   func(*Client) WorkspaceTransport
	IssueReporter      func(WorkspaceOperationalIssue)
	Now                func() time.Time
	ReconcileInterval  time.Duration
}

type WorkspaceOperationalIssue struct {
	Kind        string
	WorkspaceID string
	Detail      string
	Severity    string
}

type WorkspaceManager struct {
	ledger            *Ledger
	logger            *slog.Logger
	credentials       WorkspaceCredentialProvider
	clientFactory     func(ToolConfig) (*Client, error)
	transportFactory  func(*Client) WorkspaceTransport
	issueReporter     func(WorkspaceOperationalIssue)
	now               func() time.Time
	reconcileInterval time.Duration
	stop              chan struct{}
	done              chan struct{}
	closeOnce         sync.Once
	mu                sync.Mutex
	lastConfig        ToolConfig
	legacyImportMu    sync.Mutex
	legacyImports     map[string]*pendingLegacyVolumeImport
}

var defaultWorkspaceManager struct {
	sync.RWMutex
	manager *WorkspaceManager
}

func SetDefaultWorkspaceManager(manager *WorkspaceManager) {
	defaultWorkspaceManager.Lock()
	defaultWorkspaceManager.manager = manager
	defaultWorkspaceManager.Unlock()
}

func DefaultWorkspaceManager() *WorkspaceManager {
	defaultWorkspaceManager.RLock()
	defer defaultWorkspaceManager.RUnlock()
	return defaultWorkspaceManager.manager
}

func NewWorkspaceManager(ledger *Ledger, logger *slog.Logger, opts WorkspaceManagerOptions) (*WorkspaceManager, error) {
	if ledger == nil || ledger.db == nil {
		return nil, fmt.Errorf("virtual computers ledger is required")
	}
	if logger == nil {
		logger = slog.Default()
	}
	if opts.Now == nil {
		opts.Now = func() time.Time { return time.Now().UTC() }
	}
	if opts.ReconcileInterval <= 0 {
		opts.ReconcileInterval = 30 * time.Second
	}
	if opts.ClientFactory == nil {
		opts.ClientFactory = func(cfg ToolConfig) (*Client, error) {
			return NewClient(ClientConfig{
				BaseURL: firstNonEmpty(cfg.BoringdURL, cfg.ControlPlane.BoringdURL),
				Token:   cfg.BoringToken,
				Timeout: 30 * time.Second,
			})
		}
	}
	if opts.TransportFactory == nil {
		opts.TransportFactory = func(client *Client) WorkspaceTransport {
			return NewWebSocketWorkspaceTransport(client)
		}
	}
	manager := &WorkspaceManager{
		ledger: ledger, logger: logger, credentials: opts.CredentialProvider,
		clientFactory: opts.ClientFactory, transportFactory: opts.TransportFactory,
		issueReporter: opts.IssueReporter,
		now:           opts.Now, reconcileInterval: opts.ReconcileInterval,
		stop: make(chan struct{}), done: make(chan struct{}), legacyImports: make(map[string]*pendingLegacyVolumeImport),
	}
	go manager.run()
	return manager, nil
}

func (m *WorkspaceManager) Close() error {
	if m == nil {
		return nil
	}
	m.closeOnce.Do(func() { close(m.stop) })
	<-m.done
	return nil
}

func (m *WorkspaceManager) run() {
	defer close(m.done)
	ticker := time.NewTicker(m.reconcileInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			m.reconcileLeases(ctx)
			m.cleanupLegacyImports(ctx, false)
			cancel()
		case <-m.stop:
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			m.cleanupLegacyImports(ctx, true)
			cancel()
			return
		}
	}
}

func (m *WorkspaceManager) rememberConfig(cfg ToolConfig) {
	m.mu.Lock()
	m.lastConfig = cfg
	m.mu.Unlock()
}

func (m *WorkspaceManager) configSnapshot() ToolConfig {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.lastConfig
}

func (m *WorkspaceManager) Open(ctx context.Context, cfg ToolConfig, identity WorkspaceIdentity, request WorkspaceOpenRequest) (Workspace, error) {
	m.rememberConfig(cfg)
	if err := validateWorkspaceFeature(cfg, identity); err != nil {
		return Workspace{}, err
	}
	if err := ValidateWorkspaceNetworkCIDRs(cfg.AgentControl.AllowedPrivateCIDRs); err != nil {
		return Workspace{}, err
	}
	if !cfg.AllowInternet {
		return Workspace{}, WorkspaceRPCError{Code: "internet_disabled", Message: "virtual_computers.allow_internet must be enabled for agent workspaces"}
	}
	if profile := strings.TrimSpace(request.NetworkProfile); profile != "" && profile != "internet_lan" {
		return Workspace{}, WorkspaceRPCError{Code: "invalid_network_policy", Message: "agent workspaces support only the internet_lan network profile"}
	}
	active, err := m.ledger.CountActiveWorkspaces(ctx)
	if err != nil {
		return Workspace{}, err
	}
	if active >= cfg.AgentControl.MaxActiveWorkspaces {
		return Workspace{}, WorkspaceRPCError{Code: "workspace_limit_reached", Message: "maximum active agent workspaces reached"}
	}
	template := firstNonEmpty(request.Template, cfg.AgentControl.DefaultTemplate, "desktop")
	if template != "desktop" && template != "python" {
		return Workspace{}, WorkspaceRPCError{Code: "invalid_template", Message: "agent workspaces support only desktop and python templates"}
	}
	if request.VolumeID != "" {
		volume, ok, err := m.workspaceVolume(ctx, request.VolumeID)
		if err != nil {
			return Workspace{}, err
		}
		if !ok || volume.Format != WorkspaceVolumeFormat {
			return Workspace{}, WorkspaceRPCError{Code: "workspace_volume_required", Message: "only workspace_v2 volumes may be attached to agent workspaces"}
		}
	}
	client, err := m.clientFactory(cfg)
	if err != nil {
		return Workspace{}, err
	}
	if err := requireWorkspaceControlPlane(ctx, client); err != nil {
		return Workspace{}, err
	}
	now := m.now()
	launchTTL := cfg.AgentControl.IdleTTLSeconds
	if launchTTL > MaxTTLSeconds {
		launchTTL = MaxTTLSeconds
	}
	machine, err := client.LaunchMachine(ctx, LaunchMachineRequest{
		Template: template, TTLSeconds: launchTTL, AllowInternet: true, VolumeID: request.VolumeID, VolumeFormat: WorkspaceVolumeFormat,
		NetworkProfile:      firstNonEmpty(request.NetworkProfile, cfg.AgentControl.NetworkProfile),
		AllowedPrivateCIDRs: append([]string(nil), cfg.AgentControl.AllowedPrivateCIDRs...),
		ProtectedCIDRs:      localProtectedCIDRs(),
	})
	if err != nil {
		if detail := strings.ToLower(err.Error()); strings.Contains(detail, "network policy") || strings.Contains(detail, "iptables") || strings.Contains(detail, "firewall") {
			m.reportIssue(WorkspaceOperationalIssue{Kind: "firewall_conflict", Detail: err.Error(), Severity: "error"})
		}
		return Workspace{}, err
	}
	workspace := Workspace{
		ID: uid.NewString(), OwnerSessionID: strings.TrimSpace(identity.SessionID), MissionID: strings.TrimSpace(identity.MissionID),
		Actor: firstNonEmpty(identity.Actor, "agent"), MachineID: machine.ID, State: WorkspaceStateOpening,
		Template: template, NetworkProfile: firstNonEmpty(request.NetworkProfile, cfg.AgentControl.NetworkProfile),
		VolumeID: request.VolumeID, ControlOwner: ControlOwnerAgent, CreatedAt: now, UpdatedAt: now, LastActivityAt: now,
		LeaseExpiresAt: now.Add(time.Duration(cfg.AgentControl.IdleTTLSeconds) * time.Second),
		MaxExpiresAt:   now.Add(time.Duration(cfg.AgentControl.MaxWorkspaceSeconds) * time.Second),
	}
	if err := m.ledger.UpsertWorkspace(ctx, workspace); err != nil {
		_ = client.DestroyMachine(context.Background(), machine.ID)
		return Workspace{}, err
	}
	transport := m.transportFactory(client)
	handshakeCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	capabilities, handshakeErr := WorkspaceHandshake(handshakeCtx, transport, machine.ID, "")
	cancel()
	if handshakeErr != nil {
		workspace.State = WorkspaceStateFailed
		workspace.LastError = security.Scrub(handshakeErr.Error())
		workspace.UpdatedAt = m.now()
		_ = m.ledger.UpsertWorkspace(context.Background(), workspace)
		_ = client.DestroyMachine(context.Background(), machine.ID)
		_ = m.recordEvent(context.Background(), workspace.ID, "workspace_open_failed", workspace.LastError, nil)
		m.reportIssue(WorkspaceOperationalIssue{Kind: "guest_protocol", WorkspaceID: workspace.ID, Detail: workspace.LastError, Severity: "error"})
		return Workspace{}, WorkspaceRPCError{Code: "workspace_agent_upgrade_required", Message: "the selected rootfs did not provide the required workspace guest protocol"}
	}
	workspace.State = WorkspaceStateReady
	workspace.InstanceNonce = capabilities.InstanceNonce
	workspace.Capabilities = append([]string(nil), capabilities.Capabilities...)
	workspace.UpdatedAt = m.now()
	if err := m.ledger.UpsertWorkspace(ctx, workspace); err != nil {
		_ = client.DestroyMachine(context.Background(), machine.ID)
		return Workspace{}, err
	}
	metadata := map[string]interface{}{"template": template, "network_profile": workspace.NetworkProfile}
	if len(cfg.AgentControl.AllowedPrivateCIDRs) == 0 {
		metadata["warning"] = "public internet only; no private LAN CIDRs are allowed"
	}
	_ = m.recordEvent(ctx, workspace.ID, "workspace_opened", "Agent workspace opened", metadata)
	return workspace, nil
}

func (m *WorkspaceManager) Get(ctx context.Context, identity WorkspaceIdentity, workspaceID string) (Workspace, error) {
	workspace, err := m.ownedWorkspace(ctx, identity, workspaceID)
	if err != nil {
		return Workspace{}, err
	}
	return workspace, nil
}

func (m *WorkspaceManager) ListJobs(ctx context.Context, identity WorkspaceIdentity, workspaceID string) ([]WorkspaceJob, error) {
	workspace, err := m.ownedWorkspace(ctx, identity, workspaceID)
	if err != nil {
		return nil, err
	}
	return m.ledger.ListWorkspaceJobs(ctx, workspace.ID, 500)
}

func (m *WorkspaceManager) ListBrowserSessions(ctx context.Context, identity WorkspaceIdentity, workspaceID string) ([]BrowserSession, error) {
	workspace, err := m.ownedWorkspace(ctx, identity, workspaceID)
	if err != nil {
		return nil, err
	}
	return m.ledger.ListBrowserSessions(ctx, workspace.ID)
}

func (m *WorkspaceManager) ListCredentialGrants(ctx context.Context, identity WorkspaceIdentity, workspaceID string) ([]CredentialGrant, error) {
	workspace, err := m.ownedWorkspace(ctx, identity, workspaceID)
	if err != nil {
		return nil, err
	}
	return m.ledger.ListCredentialGrants(ctx, workspace.ID)
}

func (m *WorkspaceManager) ListEvents(ctx context.Context, identity WorkspaceIdentity, workspaceID string, afterID int64, limit int) ([]WorkspaceEvent, error) {
	workspace, err := m.ownedWorkspace(ctx, identity, workspaceID)
	if err != nil {
		return nil, err
	}
	return m.ledger.ListWorkspaceEvents(ctx, workspace.ID, afterID, limit)
}

func (m *WorkspaceManager) List(ctx context.Context, identity WorkspaceIdentity, includeClosed bool) ([]Workspace, error) {
	owner := strings.TrimSpace(identity.SessionID)
	if identity.Admin {
		owner = ""
	}
	return m.ledger.ListWorkspaces(ctx, owner, includeClosed, 200)
}

func (m *WorkspaceManager) CloseWorkspace(ctx context.Context, cfg ToolConfig, identity WorkspaceIdentity, workspaceID string) error {
	m.rememberConfig(cfg)
	workspace, err := m.ownedWorkspace(ctx, identity, workspaceID)
	if err != nil {
		return err
	}
	if workspace.State == WorkspaceStateClosed {
		return nil
	}
	workspace.State = WorkspaceStateClosing
	workspace.UpdatedAt = m.now()
	_ = m.ledger.UpsertWorkspace(ctx, workspace)
	client, err := m.clientFactory(cfg)
	if err != nil {
		return err
	}
	transport := m.transportFactory(client)
	closeCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	_ = transport.Call(closeCtx, workspace.MachineID, "workspace.close", map[string]interface{}{"revoke_credentials": true}, nil)
	cancel()
	destroyErr := client.DestroyMachine(ctx, workspace.MachineID)
	now := m.now()
	workspace.State = WorkspaceStateClosed
	workspace.UpdatedAt = now
	workspace.LastActivityAt = now
	if destroyErr != nil {
		workspace.State = WorkspaceStateLost
		workspace.LastError = security.Scrub(destroyErr.Error())
	}
	if err := m.ledger.UpsertWorkspace(ctx, workspace); err != nil {
		return err
	}
	_ = m.ledger.InterruptActiveWorkspaceJobs(ctx, workspace.ID, "workspace closed")
	m.closeBrowserSessions(ctx, workspace.ID)
	m.revokeWorkspaceGrants(ctx, workspace.ID, now)
	_ = m.recordEvent(ctx, workspace.ID, "workspace_closed", "Agent workspace closed", nil)
	return destroyErr
}

type workspaceExecRPCResult struct {
	Job    WorkspaceJob `json:"job"`
	Output string       `json:"output"`
}

func (m *WorkspaceManager) Exec(ctx context.Context, cfg ToolConfig, identity WorkspaceIdentity, workspaceID string, request WorkspaceExecRequest) (WorkspaceJob, string, error) {
	workspace, client, transport, err := m.readyWorkspace(ctx, cfg, identity, workspaceID)
	if err != nil {
		return WorkspaceJob{}, "", err
	}
	if strings.TrimSpace(request.Command) == "" {
		return WorkspaceJob{}, "", WorkspaceRPCError{Code: "invalid_argument", Message: "command is required"}
	}
	running, err := m.ledger.CountActiveWorkspaceJobs(ctx, workspace.ID)
	if err != nil {
		return WorkspaceJob{}, "", err
	}
	if running >= cfg.AgentControl.JobsPerWorkspace {
		return WorkspaceJob{}, "", WorkspaceRPCError{Code: "job_limit_reached", Message: "maximum parallel jobs for this workspace reached"}
	}
	request.TimeoutSeconds = clampWorkspaceJobTimeout(request.TimeoutSeconds, cfg.AgentControl.MaxJobSeconds)
	request.MaxOutputBytes = cfg.AgentControl.MaxJobOutputBytes
	if request.MaxOutputBytes <= 0 || request.MaxOutputBytes > workspaceMaxWireMessageBytes/2 {
		request.MaxOutputBytes = workspaceMaxWireMessageBytes / 2
	}
	var result workspaceExecRPCResult
	if err := transport.Call(ctx, workspace.MachineID, "shell.exec", request, &result); err != nil {
		return WorkspaceJob{}, "", err
	}
	result.Output = security.Scrub(result.Output)
	now := m.now()
	if result.Job.ID == "" {
		result.Job.ID = uid.NewString()
	}
	result.Job.WorkspaceID = workspace.ID
	result.Job.Mode = JobModeSync
	result.Job.Error = security.Scrub(result.Job.Error)
	result.Job.CommandHash, result.Job.CommandSummary = auditCommand(request.Command)
	if result.Job.CreatedAt.IsZero() {
		result.Job.CreatedAt = now
	}
	result.Job.UpdatedAt = now
	if err := m.ledger.UpsertWorkspaceJob(ctx, result.Job); err != nil {
		return WorkspaceJob{}, "", err
	}
	m.touchBestEffort(ctx, cfg, client, workspace)
	_ = m.recordEvent(ctx, workspace.ID, "shell_exec", result.Job.CommandSummary, map[string]interface{}{
		"job_id": result.Job.ID, "command_hash": result.Job.CommandHash, "exit_code": result.Job.ExitCode,
	})
	return result.Job, result.Output, nil
}

func (m *WorkspaceManager) StartJob(ctx context.Context, cfg ToolConfig, identity WorkspaceIdentity, workspaceID string, request WorkspaceStartJobRequest) (WorkspaceJob, error) {
	workspace, client, transport, err := m.readyWorkspace(ctx, cfg, identity, workspaceID)
	if err != nil {
		return WorkspaceJob{}, err
	}
	if strings.TrimSpace(request.Command) == "" {
		return WorkspaceJob{}, WorkspaceRPCError{Code: "invalid_argument", Message: "command is required"}
	}
	running, err := m.ledger.CountActiveWorkspaceJobs(ctx, workspace.ID)
	if err != nil {
		return WorkspaceJob{}, err
	}
	if running >= cfg.AgentControl.JobsPerWorkspace {
		return WorkspaceJob{}, WorkspaceRPCError{Code: "job_limit_reached", Message: "maximum parallel jobs for this workspace reached"}
	}
	request.TimeoutSeconds = clampWorkspaceJobTimeout(request.TimeoutSeconds, cfg.AgentControl.MaxJobSeconds)
	request.MaxOutputBytes = cfg.AgentControl.MaxJobOutputBytes
	var job WorkspaceJob
	if err := transport.Call(ctx, workspace.MachineID, "job.start", request, &job); err != nil {
		return WorkspaceJob{}, err
	}
	now := m.now()
	if job.ID == "" {
		return WorkspaceJob{}, WorkspaceRPCError{Code: "workspace_invalid_response", Message: "guest omitted job id"}
	}
	job.WorkspaceID = workspace.ID
	job.Error = security.Scrub(job.Error)
	if request.PTY {
		job.Mode = JobModePTY
	} else {
		job.Mode = JobModeSync
	}
	job.CommandHash, job.CommandSummary = auditCommand(request.Command)
	if job.CreatedAt.IsZero() {
		job.CreatedAt = now
	}
	job.UpdatedAt = now
	if err := m.ledger.UpsertWorkspaceJob(ctx, job); err != nil {
		return WorkspaceJob{}, err
	}
	m.touchBestEffort(ctx, cfg, client, workspace)
	_ = m.recordEvent(ctx, workspace.ID, "job_started", job.CommandSummary, map[string]interface{}{"job_id": job.ID, "command_hash": job.CommandHash, "pty": request.PTY})
	return job, nil
}

func (m *WorkspaceManager) JobStatus(ctx context.Context, cfg ToolConfig, identity WorkspaceIdentity, workspaceID, jobID string) (WorkspaceJob, error) {
	workspace, client, transport, err := m.readyWorkspace(ctx, cfg, identity, workspaceID)
	if err != nil {
		return WorkspaceJob{}, err
	}
	if err := m.ensureJobOwnership(ctx, workspace.ID, jobID); err != nil {
		return WorkspaceJob{}, err
	}
	previous, _, _ := m.ledger.GetWorkspaceJob(ctx, jobID)
	var job WorkspaceJob
	if err := transport.Call(ctx, workspace.MachineID, "job.status", map[string]string{"job_id": jobID}, &job); err != nil {
		return WorkspaceJob{}, err
	}
	job.WorkspaceID = workspace.ID
	job.Error = security.Scrub(job.Error)
	job.UpdatedAt = m.now()
	if err := m.ledger.UpsertWorkspaceJob(ctx, job); err != nil {
		return WorkspaceJob{}, err
	}
	if (previous.State == JobStateQueued || previous.State == JobStateRunning) && (job.State == JobStateFailed || job.State == JobStateInterrupted) {
		m.reportRepeatedJobFailures(ctx, workspace.ID)
	}
	if job.State == JobStateRunning {
		m.touchBestEffort(ctx, cfg, client, workspace)
	} else if isTerminalJobState(job.State) {
		m.revokeJobGrantRuntime(ctx, transport, workspace, job.ID)
		_ = m.ledger.CompleteJobCredentialGrants(ctx, workspace.ID, job.ID, GrantConsumed, m.now())
	}
	return job, nil
}

func (m *WorkspaceManager) reportRepeatedJobFailures(ctx context.Context, workspaceID string) {
	jobs, err := m.ledger.ListWorkspaceJobs(ctx, workspaceID, 3)
	if err != nil || len(jobs) < 3 {
		return
	}
	for _, job := range jobs {
		if job.State != JobStateFailed && job.State != JobStateInterrupted {
			return
		}
	}
	m.reportIssue(WorkspaceOperationalIssue{
		Kind: "repeated_job_failures", WorkspaceID: workspaceID,
		Detail: "three consecutive workspace jobs failed or were interrupted", Severity: "warning",
	})
}

func (m *WorkspaceManager) JobOutput(ctx context.Context, cfg ToolConfig, identity WorkspaceIdentity, workspaceID, jobID string, cursor int64, limit int) (WorkspaceJobOutput, error) {
	workspace, client, transport, err := m.readyWorkspace(ctx, cfg, identity, workspaceID)
	if err != nil {
		return WorkspaceJobOutput{}, err
	}
	if err := m.ensureJobOwnership(ctx, workspace.ID, jobID); err != nil {
		return WorkspaceJobOutput{}, err
	}
	if cursor < 0 {
		cursor = 0
	}
	if limit <= 0 || limit > workspaceOutputPageBytes {
		limit = workspaceOutputPageBytes
	}
	var output WorkspaceJobOutput
	if err := transport.Call(ctx, workspace.MachineID, "job.output", map[string]interface{}{
		"job_id": jobID, "cursor": cursor, "limit": limit,
	}, &output); err != nil {
		return WorkspaceJobOutput{}, err
	}
	output.Data = security.Scrub(output.Data)
	m.touchBestEffort(ctx, cfg, client, workspace)
	return output, nil
}

func (m *WorkspaceManager) JobInput(ctx context.Context, cfg ToolConfig, identity WorkspaceIdentity, workspaceID, jobID, input string, rows, cols uint16) error {
	workspace, client, transport, err := m.readyWorkspace(ctx, cfg, identity, workspaceID)
	if err != nil {
		return err
	}
	if err := m.ensureJobOwnership(ctx, workspace.ID, jobID); err != nil {
		return err
	}
	err = transport.Call(ctx, workspace.MachineID, "job.input", map[string]interface{}{
		"job_id": jobID, "input": input, "rows": rows, "cols": cols,
	}, nil)
	if err == nil {
		m.touchBestEffort(ctx, cfg, client, workspace)
	}
	return err
}

func (m *WorkspaceManager) CancelJob(ctx context.Context, cfg ToolConfig, identity WorkspaceIdentity, workspaceID, jobID string) error {
	workspace, _, transport, err := m.readyWorkspace(ctx, cfg, identity, workspaceID)
	if err != nil {
		return err
	}
	if err := m.ensureJobOwnership(ctx, workspace.ID, jobID); err != nil {
		return err
	}
	if err := transport.Call(ctx, workspace.MachineID, "job.cancel", map[string]string{"job_id": jobID}, nil); err != nil {
		return err
	}
	m.revokeJobGrantRuntime(ctx, transport, workspace, jobID)
	job, ok, _ := m.ledger.GetWorkspaceJob(ctx, jobID)
	if ok {
		now := m.now()
		job.State = JobStateCanceled
		job.UpdatedAt = now
		job.CompletedAt = &now
		_ = m.ledger.UpsertWorkspaceJob(ctx, job)
	}
	_ = m.ledger.CompleteJobCredentialGrants(ctx, workspace.ID, jobID, GrantRevoked, m.now())
	_ = m.recordEvent(ctx, workspace.ID, "job_canceled", "Workspace job canceled", map[string]interface{}{"job_id": jobID})
	return nil
}

func (m *WorkspaceManager) ListFiles(ctx context.Context, cfg ToolConfig, identity WorkspaceIdentity, workspaceID, relativePath string) ([]WorkspaceFileEntry, error) {
	workspace, client, transport, err := m.readyWorkspace(ctx, cfg, identity, workspaceID)
	if err != nil {
		return nil, err
	}
	var entries []WorkspaceFileEntry
	if err := transport.Call(ctx, workspace.MachineID, "file.list", map[string]string{"path": cleanWorkspacePath(relativePath)}, &entries); err != nil {
		return nil, err
	}
	m.touchBestEffort(ctx, cfg, client, workspace)
	return entries, nil
}

func (m *WorkspaceManager) ReadFile(ctx context.Context, cfg ToolConfig, identity WorkspaceIdentity, workspaceID, relativePath string, offset, limit int64) ([]byte, bool, error) {
	workspace, client, transport, err := m.readyWorkspace(ctx, cfg, identity, workspaceID)
	if err != nil {
		return nil, false, err
	}
	if limit <= 0 || limit > workspaceMaxWireMessageBytes/2 {
		limit = workspaceMaxWireMessageBytes / 2
	}
	var response struct {
		DataBase64 string `json:"data_base64"`
		EOF        bool   `json:"eof"`
	}
	if err := transport.Call(ctx, workspace.MachineID, "file.read", map[string]interface{}{
		"path": cleanWorkspacePath(relativePath), "offset": offset, "limit": limit,
	}, &response); err != nil {
		return nil, false, err
	}
	data, err := base64.StdEncoding.DecodeString(response.DataBase64)
	if err != nil {
		return nil, false, fmt.Errorf("decode workspace file: %w", err)
	}
	m.touchBestEffort(ctx, cfg, client, workspace)
	return data, response.EOF, nil
}

func (m *WorkspaceManager) WriteFile(ctx context.Context, cfg ToolConfig, identity WorkspaceIdentity, workspaceID, relativePath string, data []byte, appendMode bool) error {
	workspace, client, transport, err := m.readyWorkspace(ctx, cfg, identity, workspaceID)
	if err != nil {
		return err
	}
	if len(data) > workspaceMaxWireMessageBytes/2 {
		return WorkspaceRPCError{Code: "message_too_large", Message: "write_file payload exceeds the 4 MiB request limit"}
	}
	err = transport.Call(ctx, workspace.MachineID, "file.write", map[string]interface{}{
		"path": cleanWorkspacePath(relativePath), "data_base64": base64.StdEncoding.EncodeToString(data), "append": appendMode,
	}, nil)
	if err == nil {
		m.touchBestEffort(ctx, cfg, client, workspace)
		_ = m.recordEvent(ctx, workspace.ID, "file_written", "Workspace file written", map[string]interface{}{"path": cleanWorkspacePath(relativePath), "bytes": len(data)})
	}
	return err
}

func (m *WorkspaceManager) Checkpoint(ctx context.Context, cfg ToolConfig, identity WorkspaceIdentity, workspaceID string, ttlSeconds int) (Volume, error) {
	if !cfg.AllowVolumes {
		return Volume{}, WorkspaceRPCError{Code: "volumes_disabled", Message: "virtual computer volumes are disabled"}
	}
	workspace, client, transport, err := m.readyWorkspace(ctx, cfg, identity, workspaceID)
	if err != nil {
		return Volume{}, err
	}
	if ttlSeconds <= 0 {
		ttlSeconds = 30 * 24 * 60 * 60
	}
	volumeID := workspace.VolumeID
	var volume Volume
	if volumeID == "" {
		volume, err = client.CreateVolume(ctx, ttlSeconds)
		if err != nil {
			return Volume{}, err
		}
		volumeID = volume.ID
	} else if existing, ok, lookupErr := m.workspaceVolume(ctx, volumeID); lookupErr != nil {
		return Volume{}, lookupErr
	} else if !ok || existing.Format != WorkspaceVolumeFormat {
		return Volume{}, WorkspaceRPCError{Code: "workspace_volume_required", Message: "checkpoint target is not a workspace_v2 volume"}
	} else {
		volume = existing
	}
	if err := transport.Call(ctx, workspace.MachineID, "workspace.prepare_checkpoint", map[string]interface{}{
		"volume_id": volumeID, "include": []string{"/workspace"}, "exclude": []string{"/root", "/run", "/tmp"},
	}, nil); err != nil {
		return Volume{}, err
	}
	if _, err := client.SaveMachineWithFormat(ctx, workspace.MachineID, volumeID, WorkspaceVolumeFormat); err != nil {
		return Volume{}, err
	}
	volume.ID = volumeID
	volume.Format = WorkspaceVolumeFormat
	volume.VerificationStatus = "verified"
	volume.Availability = "available"
	now := m.now()
	volume.LastVerifiedAt = &now
	if err := m.ledger.UpsertVolume(ctx, volume); err != nil {
		return Volume{}, err
	}
	workspace.VolumeID = volumeID
	workspace.UpdatedAt = now
	if err := m.ledger.UpsertWorkspace(ctx, workspace); err != nil {
		return Volume{}, err
	}
	_ = m.recordEvent(ctx, workspace.ID, "checkpoint_created", "Workspace checkpoint created", map[string]interface{}{"volume_id": volumeID, "format": WorkspaceVolumeFormat})
	return volume, nil
}

func (m *WorkspaceManager) BrowserAction(ctx context.Context, cfg ToolConfig, identity WorkspaceIdentity, workspaceID string, request BrowserActionRequest) (BrowserActionResult, error) {
	workspace, client, transport, err := m.readyWorkspace(ctx, cfg, identity, workspaceID)
	if err != nil {
		return BrowserActionResult{}, err
	}
	if workspace.Template != "desktop" {
		return BrowserActionResult{}, WorkspaceRPCError{Code: "browser_unavailable", Message: "browser control requires a desktop workspace"}
	}
	if workspace.ControlOwner == ControlOwnerHuman && workspace.ControlLeaseExpiresAt != nil && workspace.ControlLeaseExpiresAt.After(m.now()) && browserActionRequiresControl(request.Operation) {
		return BrowserActionResult{}, WorkspaceRPCError{Code: "browser_human_control", Message: "browser input is paused while the user controls the workspace"}
	}
	if request.Operation == "open" {
		sessions, err := m.ledger.ListBrowserSessions(ctx, workspace.ID)
		if err != nil {
			return BrowserActionResult{}, err
		}
		open := 0
		for _, session := range sessions {
			if session.State == BrowserStateOpen {
				open++
			}
		}
		if open >= cfg.AgentControl.BrowserSessionsPerWorkspace {
			return BrowserActionResult{}, WorkspaceRPCError{Code: "browser_session_limit_reached", Message: "maximum browser sessions for this workspace reached"}
		}
	}
	var result BrowserActionResult
	if err := transport.Call(ctx, workspace.MachineID, "browser."+strings.TrimSpace(request.Operation), request, &result); err != nil {
		return BrowserActionResult{}, err
	}
	result.Data = scrubWorkspaceData(result.Data)
	if result.Session.ID != "" {
		result.Session.WorkspaceID = workspace.ID
		result.Session.ControlOwner = workspace.ControlOwner
		result.Session.ControlLeaseExpiresAt = workspace.ControlLeaseExpiresAt
		result.Session.UpdatedAt = m.now()
		if result.Session.CreatedAt.IsZero() {
			result.Session.CreatedAt = result.Session.UpdatedAt
		}
		if err := m.ledger.UpsertBrowserSession(ctx, result.Session); err != nil {
			return BrowserActionResult{}, err
		}
	}
	m.touchBestEffort(ctx, cfg, client, workspace)
	origin := browserOrigin(request.URL)
	if origin == "" {
		origin = result.Session.URLOrigin
	}
	_ = m.recordEvent(ctx, workspace.ID, "browser_action", "Browser "+request.Operation, map[string]interface{}{"operation": request.Operation, "origin": origin})
	return result, nil
}

func browserActionRequiresControl(operation string) bool {
	switch strings.TrimSpace(operation) {
	case "inspect", "list_tabs", "screenshot", "list_downloads", "wait":
		return false
	default:
		return true
	}
}

func (m *WorkspaceManager) TakeControl(ctx context.Context, identity WorkspaceIdentity, workspaceID string, human bool) (Workspace, error) {
	if !identity.Admin {
		return Workspace{}, WorkspaceRPCError{Code: "forbidden", Message: "administrator permission is required to take workspace control"}
	}
	workspace, err := m.ownedWorkspace(ctx, identity, workspaceID)
	if err != nil {
		return Workspace{}, err
	}
	now := m.now()
	workspace.ControlOwner = ControlOwnerAgent
	workspace.ControlLeaseExpiresAt = nil
	if human {
		expires := now.Add(workspaceControlLease)
		workspace.ControlOwner = ControlOwnerHuman
		workspace.ControlLeaseExpiresAt = &expires
	}
	workspace.UpdatedAt = now
	if err := m.ledger.UpsertWorkspace(ctx, workspace); err != nil {
		return Workspace{}, err
	}
	m.syncBrowserControl(ctx, workspace)
	_ = m.recordEvent(ctx, workspace.ID, "control_changed", "Workspace control changed", map[string]interface{}{"control_owner": workspace.ControlOwner})
	return workspace, nil
}

func (m *WorkspaceManager) ListGrantableCredentials(ctx context.Context, cfg ToolConfig) ([]GrantableCredential, error) {
	if !cfg.AgentControl.CredentialsEnabled {
		return nil, WorkspaceRPCError{Code: "credential_grants_disabled", Message: "credential grants for virtual computers are disabled"}
	}
	if m.credentials == nil {
		return nil, fmt.Errorf("credential provider is unavailable")
	}
	return m.credentials.ListGrantable(ctx)
}

func (m *WorkspaceManager) RequestCredentialGrant(ctx context.Context, cfg ToolConfig, identity WorkspaceIdentity, workspaceID string, request CredentialGrantRequest) (CredentialGrant, error) {
	if !cfg.AgentControl.CredentialsEnabled {
		return CredentialGrant{}, WorkspaceRPCError{Code: "credential_grants_disabled", Message: "credential grants for virtual computers are disabled"}
	}
	workspace, err := m.ownedWorkspace(ctx, identity, workspaceID)
	if err != nil {
		return CredentialGrant{}, err
	}
	if request.UsageType != GrantUsageShell && request.UsageType != GrantUsageBrowser {
		return CredentialGrant{}, WorkspaceRPCError{Code: "invalid_grant_usage", Message: "usage_type must be shell or browser"}
	}
	if request.UsageType == GrantUsageShell {
		if strings.TrimSpace(request.JobID) == "" {
			return CredentialGrant{}, WorkspaceRPCError{Code: "job_binding_required", Message: "shell grants require a job_id"}
		}
		if err := m.ensureJobOwnership(ctx, workspace.ID, request.JobID); err != nil {
			return CredentialGrant{}, err
		}
		job, ok, err := m.ledger.GetWorkspaceJob(ctx, request.JobID)
		if err != nil {
			return CredentialGrant{}, err
		}
		if !ok || job.State != JobStateQueued {
			return CredentialGrant{}, WorkspaceRPCError{Code: "credential_job_not_queued", Message: "shell grants require a queued job created with wait_for_credential_grant"}
		}
	}
	if request.UsageType == GrantUsageBrowser {
		normalizedOrigin, err := normalizeGrantOrigin(request.Origin)
		if err != nil {
			return CredentialGrant{}, err
		}
		request.Origin = normalizedOrigin
	}
	available, err := m.ListGrantableCredentials(ctx, cfg)
	if err != nil {
		return CredentialGrant{}, err
	}
	var found *GrantableCredential
	for i := range available {
		if available[i].ID == strings.TrimSpace(request.CredentialID) {
			found = &available[i]
			break
		}
	}
	if found == nil {
		return CredentialGrant{}, WorkspaceRPCError{Code: "credential_not_grantable", Message: "credential is not grantable to virtual computers"}
	}
	fields := filterRequestedGrantMetadataFields(request.FieldNames, found.AvailableFields)
	if len(request.FieldNames) > 0 && len(fields) == 0 {
		return CredentialGrant{}, WorkspaceRPCError{Code: "invalid_grant_fields", Message: "none of the requested credential fields are grantable"}
	}
	if len(fields) == 0 {
		fields = append([]string(nil), found.AvailableFields...)
	}
	purpose := security.Scrub(strings.TrimSpace(request.Purpose))
	if runes := []rune(purpose); len(runes) > 240 {
		purpose = string(runes[:240])
	}
	now := m.now()
	grant := CredentialGrant{
		ID: uid.NewString(), WorkspaceID: workspace.ID, CredentialID: found.ID, UsageType: request.UsageType,
		Origin: request.Origin, JobID: request.JobID, FieldNames: fields, Purpose: purpose,
		Status: GrantPending, RequestedBy: firstNonEmpty(identity.Actor, "agent"), CreatedAt: now, UpdatedAt: now,
		ExpiresAt: now.Add(time.Duration(cfg.AgentControl.CredentialGrantTTLSeconds) * time.Second),
	}
	if err := m.ledger.UpsertCredentialGrant(ctx, grant); err != nil {
		return CredentialGrant{}, err
	}
	_ = m.recordEvent(ctx, workspace.ID, "credential_grant_requested", "Credential grant awaits user approval", map[string]interface{}{
		"grant_id": grant.ID, "credential_id": grant.CredentialID, "usage_type": grant.UsageType,
		"origin": grant.Origin, "job_id": grant.JobID, "field_names": grant.FieldNames, "expires_at": grant.ExpiresAt,
	})
	return grant, nil
}

func (m *WorkspaceManager) ApproveCredentialGrant(ctx context.Context, cfg ToolConfig, identity WorkspaceIdentity, grantID string) (CredentialGrant, error) {
	if !identity.Admin {
		return CredentialGrant{}, WorkspaceRPCError{Code: "forbidden", Message: "an authenticated user must approve credential grants"}
	}
	grant, ok, err := m.ledger.GetCredentialGrant(ctx, grantID)
	if err != nil {
		return CredentialGrant{}, err
	}
	if !ok {
		return CredentialGrant{}, WorkspaceRPCError{Code: "not_found", Message: "credential grant was not found"}
	}
	workspace, err := m.ownedWorkspace(ctx, identity, grant.WorkspaceID)
	if err != nil {
		return CredentialGrant{}, err
	}
	if grant.Status != GrantPending || !grant.ExpiresAt.After(m.now()) {
		return CredentialGrant{}, WorkspaceRPCError{Code: "grant_not_pending", Message: "credential grant is no longer pending"}
	}
	var shellTransport WorkspaceTransport
	if grant.UsageType == GrantUsageShell {
		values, err := m.credentials.Resolve(ctx, grant.CredentialID, grant.FieldNames)
		if err != nil {
			return CredentialGrant{}, err
		}
		credentialName := grant.CredentialID
		grantable, listErr := m.credentials.ListGrantable(ctx)
		if listErr != nil {
			clearStringMap(values)
			return CredentialGrant{}, listErr
		}
		for _, candidate := range grantable {
			if candidate.ID == grant.CredentialID {
				credentialName = candidate.Name
				break
			}
		}
		client, err := m.clientFactory(cfg)
		if err != nil {
			clearStringMap(values)
			return CredentialGrant{}, err
		}
		if err := requireWorkspaceControlPlane(ctx, client); err != nil {
			clearStringMap(values)
			return CredentialGrant{}, err
		}
		shellTransport = m.transportFactory(client)
		if err := shellTransport.Call(ctx, workspace.MachineID, "credential.activate", map[string]interface{}{
			"grant_id": grant.ID, "usage_type": grant.UsageType, "job_id": grant.JobID,
			"credential_name": credentialName, "field_names": grant.FieldNames, "values": values, "expires_at": grant.ExpiresAt,
		}, nil); err != nil {
			clearStringMap(values)
			return CredentialGrant{}, err
		}
		clearStringMap(values)
	}
	grant.Status = GrantActive
	grant.ApprovedBy = firstNonEmpty(identity.Actor, "user")
	grant.UpdatedAt = m.now()
	if err := m.ledger.UpsertCredentialGrant(ctx, grant); err != nil {
		if shellTransport != nil {
			cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
			_ = shellTransport.Call(cleanupCtx, workspace.MachineID, "job.cancel", map[string]string{"job_id": grant.JobID}, nil)
			_ = shellTransport.Call(cleanupCtx, workspace.MachineID, "credential.revoke", map[string]string{"grant_id": grant.ID}, nil)
			cleanupCancel()
		}
		return CredentialGrant{}, err
	}
	_ = m.recordEvent(ctx, workspace.ID, "credential_grant_approved", "Credential grant approved", map[string]interface{}{
		"grant_id": grant.ID, "credential_id": grant.CredentialID, "usage_type": grant.UsageType,
		"origin": grant.Origin, "job_id": grant.JobID, "field_names": grant.FieldNames, "expires_at": grant.ExpiresAt,
	})
	return grant, nil
}

func (m *WorkspaceManager) RevokeCredentialGrant(ctx context.Context, cfg ToolConfig, identity WorkspaceIdentity, workspaceID, grantID string) error {
	workspace, err := m.ownedWorkspace(ctx, identity, workspaceID)
	if err != nil {
		return err
	}
	grant, ok, err := m.ledger.GetCredentialGrant(ctx, grantID)
	if err != nil {
		return err
	}
	if !ok || grant.WorkspaceID != workspace.ID {
		return WorkspaceRPCError{Code: "not_found", Message: "credential grant was not found"}
	}
	if grant.Status == GrantActive && grant.UsageType == GrantUsageShell {
		client, err := m.clientFactory(cfg)
		if err != nil {
			return err
		}
		if err := requireWorkspaceControlPlane(ctx, client); err != nil {
			return err
		}
		transport := m.transportFactory(client)
		if grant.JobID != "" {
			if err := transport.Call(ctx, workspace.MachineID, "job.cancel", map[string]string{"job_id": grant.JobID}, nil); err != nil {
				return fmt.Errorf("cancel credential-bound job: %w", err)
			}
		}
		if err := transport.Call(ctx, workspace.MachineID, "credential.revoke", map[string]string{"grant_id": grant.ID}, nil); err != nil {
			return fmt.Errorf("revoke guest credential material: %w", err)
		}
	}
	now := m.now()
	grant.Status = GrantRevoked
	grant.UpdatedAt = now
	grant.RevokedAt = &now
	if err := m.ledger.UpsertCredentialGrant(ctx, grant); err != nil {
		return err
	}
	_ = m.recordEvent(ctx, workspace.ID, "credential_grant_revoked", "Credential grant revoked", map[string]interface{}{"grant_id": grant.ID, "credential_id": grant.CredentialID})
	return nil
}

func (m *WorkspaceManager) UseBrowserGrant(ctx context.Context, cfg ToolConfig, identity WorkspaceIdentity, workspaceID string, request BrowserActionRequest) (BrowserActionResult, error) {
	workspace, client, transport, err := m.readyWorkspace(ctx, cfg, identity, workspaceID)
	if err != nil {
		return BrowserActionResult{}, err
	}
	if workspace.ControlOwner == ControlOwnerHuman && workspace.ControlLeaseExpiresAt != nil && workspace.ControlLeaseExpiresAt.After(m.now()) {
		return BrowserActionResult{}, WorkspaceRPCError{Code: "browser_human_control", Message: "browser input is paused while the user controls the workspace"}
	}
	grant, ok, err := m.ledger.GetCredentialGrant(ctx, request.GrantID)
	if err != nil {
		return BrowserActionResult{}, err
	}
	if !ok || grant.WorkspaceID != workspace.ID || grant.UsageType != GrantUsageBrowser || grant.Status != GrantActive || !grant.ExpiresAt.After(m.now()) {
		return BrowserActionResult{}, WorkspaceRPCError{Code: "invalid_credential_grant", Message: "browser credential grant is unavailable"}
	}
	origin := browserOrigin(request.URL)
	if origin == "" {
		origin = browserOrigin(grant.Origin)
	}
	if origin != grant.Origin {
		return BrowserActionResult{}, WorkspaceRPCError{Code: "credential_origin_mismatch", Message: "browser credential grant is bound to another origin"}
	}
	values, err := m.credentials.Resolve(ctx, grant.CredentialID, grant.FieldNames)
	if err != nil {
		return BrowserActionResult{}, err
	}
	request.Fields = values
	request.Operation = "credential_fill"
	request.URL = grant.Origin
	var result BrowserActionResult
	err = transport.Call(ctx, workspace.MachineID, "browser.credential_fill", request, &result)
	clearStringMap(values)
	request.Fields = nil
	if err != nil {
		return BrowserActionResult{}, err
	}
	result.Data = scrubWorkspaceData(result.Data)
	grant.Status = GrantConsumed
	grant.UpdatedAt = m.now()
	_ = m.ledger.UpsertCredentialGrant(ctx, grant)
	m.touchBestEffort(ctx, cfg, client, workspace)
	_ = m.recordEvent(ctx, workspace.ID, "credential_grant_consumed", "Browser credential grant consumed", map[string]interface{}{
		"grant_id": grant.ID, "credential_id": grant.CredentialID, "origin": grant.Origin, "field_names": grant.FieldNames,
	})
	return result, nil
}

func (m *WorkspaceManager) Reconcile(ctx context.Context, cfg ToolConfig) error {
	m.rememberConfig(cfg)
	workspaces, err := m.ledger.ListWorkspaces(ctx, "", false, 500)
	if err != nil {
		return err
	}
	client, err := m.clientFactory(cfg)
	if err != nil {
		return err
	}
	if err := requireWorkspaceControlPlane(ctx, client); err != nil {
		m.reportIssue(WorkspaceOperationalIssue{Kind: "guest_protocol", Detail: err.Error(), Severity: "error"})
		return err
	}
	transport := m.transportFactory(client)
	for _, workspace := range workspaces {
		if workspace.State != WorkspaceStateOpening && workspace.State != WorkspaceStateReady && workspace.State != WorkspaceStateClosing {
			continue
		}
		if _, err := client.GetMachine(ctx, workspace.MachineID); err != nil {
			workspace.State = WorkspaceStateLost
			workspace.LastError = "machine unavailable after AuraGo restart"
			workspace.UpdatedAt = m.now()
			_ = m.ledger.UpsertWorkspace(ctx, workspace)
			_ = m.ledger.InterruptActiveWorkspaceJobs(ctx, workspace.ID, workspace.LastError)
			m.closeBrowserSessions(ctx, workspace.ID)
			m.revokeWorkspaceGrants(ctx, workspace.ID, m.now())
			_ = m.recordEvent(ctx, workspace.ID, "workspace_lost", workspace.LastError, nil)
			m.reportIssue(WorkspaceOperationalIssue{Kind: "machine_lost", WorkspaceID: workspace.ID, Detail: workspace.LastError, Severity: "error"})
			continue
		}
		capabilities, err := WorkspaceHandshake(ctx, transport, workspace.MachineID, workspace.InstanceNonce)
		if err != nil {
			workspace.State = WorkspaceStateLost
			workspace.LastError = security.Scrub(err.Error())
			workspace.UpdatedAt = m.now()
			_ = m.ledger.UpsertWorkspace(ctx, workspace)
			_ = m.ledger.InterruptActiveWorkspaceJobs(ctx, workspace.ID, "guest workspace agent did not confirm running jobs")
			m.closeBrowserSessions(ctx, workspace.ID)
			m.revokeWorkspaceGrants(ctx, workspace.ID, m.now())
			m.reportIssue(WorkspaceOperationalIssue{Kind: "guest_protocol", WorkspaceID: workspace.ID, Detail: workspace.LastError, Severity: "error"})
			continue
		}
		if workspace.InstanceNonce != "" && workspace.InstanceNonce != capabilities.InstanceNonce {
			_ = m.ledger.InterruptActiveWorkspaceJobs(ctx, workspace.ID, "guest instance nonce changed after restore or fork")
			m.closeBrowserSessions(ctx, workspace.ID)
			m.revokeWorkspaceGrants(ctx, workspace.ID, m.now())
			_ = m.recordEvent(ctx, workspace.ID, "workspace_instance_reset", "Guest runtime state reset after restore or fork", nil)
		}
		workspace.InstanceNonce = capabilities.InstanceNonce
		workspace.Capabilities = capabilities.Capabilities
		workspace.State = WorkspaceStateReady
		workspace.UpdatedAt = m.now()
		_ = m.ledger.UpsertWorkspace(ctx, workspace)
		jobs, _ := m.ledger.ListWorkspaceJobs(ctx, workspace.ID, 500)
		for _, storedJob := range jobs {
			if storedJob.State != JobStateRunning && storedJob.State != JobStateQueued {
				continue
			}
			var current WorkspaceJob
			if err := transport.Call(ctx, workspace.MachineID, "job.status", map[string]string{"job_id": storedJob.ID}, &current); err != nil {
				now := m.now()
				storedJob.State = JobStateInterrupted
				storedJob.Error = "guest did not confirm job after AuraGo restart"
				storedJob.UpdatedAt = now
				storedJob.CompletedAt = &now
				_ = m.ledger.UpsertWorkspaceJob(ctx, storedJob)
				continue
			}
			current.WorkspaceID = workspace.ID
			current.Error = security.Scrub(current.Error)
			current.CommandHash = storedJob.CommandHash
			current.CommandSummary = storedJob.CommandSummary
			_ = m.ledger.UpsertWorkspaceJob(ctx, current)
		}
	}
	return nil
}

func (m *WorkspaceManager) reconcileLeases(ctx context.Context) {
	cfg := m.configSnapshot()
	if !cfg.AgentControl.Enabled {
		return
	}
	workspaces, err := m.ledger.ListWorkspaces(ctx, "", false, 500)
	if err != nil {
		m.logger.Warn("[VirtualWorkspace] lease scan failed", "error", err)
		return
	}
	client, clientErr := m.clientFactory(cfg)
	var transport WorkspaceTransport
	if clientErr == nil {
		transport = m.transportFactory(client)
	}
	for _, workspace := range workspaces {
		grants, _ := m.ledger.ListCredentialGrants(ctx, workspace.ID)
		for _, grant := range grants {
			if (grant.Status != GrantActive && grant.Status != GrantPending) || grant.ExpiresAt.After(m.now()) || grant.UsageType != GrantUsageShell || grant.JobID == "" {
				continue
			}
			if transport == nil {
				detail := "workspace transport is unavailable"
				if clientErr != nil {
					detail = clientErr.Error()
				}
				m.reportIssue(WorkspaceOperationalIssue{Kind: "credential_expiry_cleanup_failed", WorkspaceID: workspace.ID, Detail: detail, Severity: "error"})
				continue
			}
			cancelErr := transport.Call(ctx, workspace.MachineID, "job.cancel", map[string]string{"job_id": grant.JobID}, nil)
			var revokeErr error
			if grant.Status == GrantActive {
				revokeErr = transport.Call(ctx, workspace.MachineID, "credential.revoke", map[string]string{"grant_id": grant.ID}, nil)
			}
			if cancelErr != nil || revokeErr != nil {
				detail := fmt.Sprintf("cancel job: %v; revoke credential: %v", cancelErr, revokeErr)
				m.reportIssue(WorkspaceOperationalIssue{Kind: "credential_expiry_cleanup_failed", WorkspaceID: workspace.ID, Detail: detail, Severity: "error"})
				continue
			}
			if job, ok, _ := m.ledger.GetWorkspaceJob(ctx, grant.JobID); ok && (job.State == JobStateQueued || job.State == JobStateRunning) {
				now := m.now()
				job.State = JobStateCanceled
				job.Error = "credential grant expired"
				job.UpdatedAt = now
				job.CompletedAt = &now
				_ = m.ledger.UpsertWorkspaceJob(ctx, job)
			}
		}
		if workspace.ControlOwner == ControlOwnerHuman && workspace.ControlLeaseExpiresAt != nil && !workspace.ControlLeaseExpiresAt.After(m.now()) {
			workspace.ControlOwner = ControlOwnerAgent
			workspace.ControlLeaseExpiresAt = nil
			workspace.UpdatedAt = m.now()
			_ = m.ledger.UpsertWorkspace(ctx, workspace)
			m.syncBrowserControl(ctx, workspace)
		}
		hasActiveJob := m.refreshWorkspaceJobs(ctx, workspace, transport)
		if hasActiveJob && clientErr == nil && workspace.MaxExpiresAt.After(m.now()) {
			if err := m.touch(ctx, cfg, client, workspace); err != nil {
				m.reportIssue(WorkspaceOperationalIssue{Kind: "lease_extension_failed", WorkspaceID: workspace.ID, Detail: err.Error(), Severity: "warning"})
			} else {
				now := m.now()
				workspace.LastActivityAt = now
				workspace.LeaseExpiresAt = now.Add(time.Duration(cfg.AgentControl.IdleTTLSeconds) * time.Second)
				if workspace.LeaseExpiresAt.After(workspace.MaxExpiresAt) {
					workspace.LeaseExpiresAt = workspace.MaxExpiresAt
				}
			}
		}
		if !workspace.MaxExpiresAt.After(m.now()) || !workspace.LeaseExpiresAt.After(m.now()) {
			identity := WorkspaceIdentity{SessionID: workspace.OwnerSessionID, MissionID: workspace.MissionID, Actor: "workspace_reaper", Admin: true}
			if err := m.CloseWorkspace(ctx, cfg, identity, workspace.ID); err != nil {
				m.logger.Warn("[VirtualWorkspace] automatic close failed", "workspace_id", workspace.ID, "error", err)
				m.reportIssue(WorkspaceOperationalIssue{Kind: "lease_close_failed", WorkspaceID: workspace.ID, Detail: err.Error(), Severity: "warning"})
			}
		}
	}
	_ = m.ledger.ExpireCredentialGrants(ctx, m.now())
}

func (m *WorkspaceManager) refreshWorkspaceJobs(ctx context.Context, workspace Workspace, transport WorkspaceTransport) bool {
	jobs, err := m.ledger.ListWorkspaceJobs(ctx, workspace.ID, 500)
	if err != nil {
		m.logger.Warn("[VirtualWorkspace] job reconciliation failed", "workspace_id", workspace.ID, "error", err)
		return true
	}
	hasActive := false
	for _, stored := range jobs {
		if stored.State != JobStateQueued && stored.State != JobStateRunning {
			continue
		}
		if transport == nil || workspace.State != WorkspaceStateReady {
			hasActive = true
			continue
		}
		var current WorkspaceJob
		if err := transport.Call(ctx, workspace.MachineID, "job.status", map[string]string{"job_id": stored.ID}, &current); err != nil {
			hasActive = true
			continue
		}
		current.WorkspaceID = workspace.ID
		current.Error = security.Scrub(current.Error)
		current.CommandHash = stored.CommandHash
		current.CommandSummary = stored.CommandSummary
		if current.Mode == "" {
			current.Mode = stored.Mode
		}
		if current.CreatedAt.IsZero() {
			current.CreatedAt = stored.CreatedAt
		}
		current.UpdatedAt = m.now()
		if err := m.ledger.UpsertWorkspaceJob(ctx, current); err != nil {
			hasActive = true
			continue
		}
		if current.State == JobStateQueued || current.State == JobStateRunning {
			hasActive = true
			continue
		}
		if (current.State == JobStateFailed || current.State == JobStateInterrupted) && (stored.State == JobStateQueued || stored.State == JobStateRunning) {
			m.reportRepeatedJobFailures(ctx, workspace.ID)
		}
		if isTerminalJobState(current.State) {
			m.revokeJobGrantRuntime(ctx, transport, workspace, current.ID)
			_ = m.ledger.CompleteJobCredentialGrants(ctx, workspace.ID, current.ID, GrantConsumed, m.now())
		}
	}
	return hasActive
}

func (m *WorkspaceManager) readyWorkspace(ctx context.Context, cfg ToolConfig, identity WorkspaceIdentity, workspaceID string) (Workspace, *Client, WorkspaceTransport, error) {
	m.rememberConfig(cfg)
	if err := validateWorkspaceFeature(cfg, identity); err != nil {
		return Workspace{}, nil, nil, err
	}
	workspace, err := m.ownedWorkspace(ctx, identity, workspaceID)
	if err != nil {
		return Workspace{}, nil, nil, err
	}
	if workspace.State != WorkspaceStateReady {
		return Workspace{}, nil, nil, WorkspaceRPCError{Code: "workspace_not_ready", Message: "workspace is not ready"}
	}
	if !workspace.MaxExpiresAt.After(m.now()) {
		return Workspace{}, nil, nil, WorkspaceRPCError{Code: "workspace_expired", Message: "workspace reached its maximum runtime"}
	}
	if !workspace.LeaseExpiresAt.After(m.now()) {
		return Workspace{}, nil, nil, WorkspaceRPCError{Code: "workspace_expired", Message: "workspace idle lease expired"}
	}
	client, err := m.clientFactory(cfg)
	if err != nil {
		return Workspace{}, nil, nil, err
	}
	if err := requireWorkspaceControlPlane(ctx, client); err != nil {
		return Workspace{}, nil, nil, err
	}
	return workspace, client, m.transportFactory(client), nil
}

func requireWorkspaceControlPlane(ctx context.Context, client *Client) error {
	if client == nil {
		return WorkspaceRPCError{Code: "workspace_agent_upgrade_required", Message: "workspace control plane is unavailable"}
	}
	status, err := client.WorkspaceCapabilities(ctx)
	if err != nil || status.ProtocolVersion != WorkspaceProtocolVersion || status.AssetFingerprint != WorkspaceAssetFingerprint() {
		return WorkspaceRPCError{Code: "workspace_agent_upgrade_required", Message: "boringd, rootfs, and guest workspace assets must be upgraded together"}
	}
	return nil
}

func (m *WorkspaceManager) ownedWorkspace(ctx context.Context, identity WorkspaceIdentity, workspaceID string) (Workspace, error) {
	workspace, ok, err := m.ledger.GetWorkspace(ctx, strings.TrimSpace(workspaceID))
	if err != nil {
		return Workspace{}, err
	}
	if !ok {
		return Workspace{}, WorkspaceRPCError{Code: "not_found", Message: "workspace was not found"}
	}
	if !identity.Admin && workspace.OwnerSessionID != strings.TrimSpace(identity.SessionID) {
		return Workspace{}, WorkspaceRPCError{Code: "not_found", Message: "workspace was not found"}
	}
	if !identity.Admin && workspace.MissionID != strings.TrimSpace(identity.MissionID) {
		return Workspace{}, WorkspaceRPCError{Code: "not_found", Message: "workspace was not found"}
	}
	return workspace, nil
}

func (m *WorkspaceManager) ensureJobOwnership(ctx context.Context, workspaceID, jobID string) error {
	job, ok, err := m.ledger.GetWorkspaceJob(ctx, strings.TrimSpace(jobID))
	if err != nil {
		return err
	}
	if !ok || job.WorkspaceID != workspaceID {
		return WorkspaceRPCError{Code: "not_found", Message: "workspace job was not found"}
	}
	return nil
}

func (m *WorkspaceManager) touch(ctx context.Context, cfg ToolConfig, client *Client, workspace Workspace) error {
	now := m.now()
	lease := now.Add(time.Duration(cfg.AgentControl.IdleTTLSeconds) * time.Second)
	if lease.After(workspace.MaxExpiresAt) {
		lease = workspace.MaxExpiresAt
	}
	workspace.LastActivityAt = now
	workspace.UpdatedAt = now
	workspace.LeaseExpiresAt = lease
	remaining := int(lease.Sub(now).Seconds())
	if remaining < MinTTLSeconds {
		remaining = MinTTLSeconds
	}
	if remaining > MaxTTLSeconds {
		remaining = MaxTTLSeconds
	}
	if _, err := client.ExtendMachine(ctx, workspace.MachineID, remaining); err != nil {
		return err
	}
	return m.ledger.UpsertWorkspace(ctx, workspace)
}

func (m *WorkspaceManager) touchBestEffort(ctx context.Context, cfg ToolConfig, client *Client, workspace Workspace) {
	if err := m.touch(ctx, cfg, client, workspace); err != nil {
		m.reportIssue(WorkspaceOperationalIssue{
			Kind: "lease_extension_failed", WorkspaceID: workspace.ID,
			Detail: err.Error(), Severity: "warning",
		})
	}
}

func (m *WorkspaceManager) syncBrowserControl(ctx context.Context, workspace Workspace) {
	sessions, err := m.ledger.ListBrowserSessions(ctx, workspace.ID)
	if err != nil {
		return
	}
	for _, session := range sessions {
		session.ControlOwner = workspace.ControlOwner
		session.ControlLeaseExpiresAt = workspace.ControlLeaseExpiresAt
		session.UpdatedAt = m.now()
		_ = m.ledger.UpsertBrowserSession(ctx, session)
	}
}

func (m *WorkspaceManager) closeBrowserSessions(ctx context.Context, workspaceID string) {
	sessions, err := m.ledger.ListBrowserSessions(ctx, workspaceID)
	if err != nil {
		return
	}
	now := m.now()
	for _, session := range sessions {
		if session.State == BrowserStateClosed {
			continue
		}
		session.State = BrowserStateClosed
		session.ControlOwner = ControlOwnerAgent
		session.ControlLeaseExpiresAt = nil
		session.UpdatedAt = now
		_ = m.ledger.UpsertBrowserSession(ctx, session)
	}
}

func (m *WorkspaceManager) revokeWorkspaceGrants(ctx context.Context, workspaceID string, now time.Time) {
	grants, err := m.ledger.ListCredentialGrants(ctx, workspaceID)
	if err != nil {
		return
	}
	for _, grant := range grants {
		if grant.Status != GrantPending && grant.Status != GrantActive {
			continue
		}
		grant.Status = GrantRevoked
		grant.UpdatedAt = now
		grant.RevokedAt = &now
		_ = m.ledger.UpsertCredentialGrant(ctx, grant)
	}
}

func (m *WorkspaceManager) workspaceVolume(ctx context.Context, id string) (Volume, bool, error) {
	volumes, err := m.ledger.ListVolumes(ctx)
	if err != nil {
		return Volume{}, false, err
	}
	for _, volume := range volumes {
		if volume.ID == strings.TrimSpace(id) {
			return volume, true, nil
		}
	}
	return Volume{}, false, nil
}

func (m *WorkspaceManager) recordEvent(ctx context.Context, workspaceID, eventType, summary string, metadata map[string]interface{}) error {
	return m.ledger.RecordWorkspaceEvent(ctx, WorkspaceEvent{
		WorkspaceID: workspaceID, Type: eventType, Summary: security.Scrub(summary), Metadata: metadata, CreatedAt: m.now(),
	})
}

func (m *WorkspaceManager) reportIssue(issue WorkspaceOperationalIssue) {
	if m == nil || m.issueReporter == nil {
		return
	}
	issue.Kind = strings.TrimSpace(issue.Kind)
	issue.WorkspaceID = strings.TrimSpace(issue.WorkspaceID)
	issue.Detail = security.Scrub(strings.TrimSpace(issue.Detail))
	if issue.Severity == "" {
		issue.Severity = "warning"
	}
	m.issueReporter(issue)
}

func (m *WorkspaceManager) revokeJobGrantRuntime(ctx context.Context, transport WorkspaceTransport, workspace Workspace, jobID string) {
	grants, err := m.ledger.ListCredentialGrants(ctx, workspace.ID)
	if err != nil {
		return
	}
	for _, grant := range grants {
		if grant.JobID == jobID && grant.UsageType == GrantUsageShell && grant.Status == GrantActive {
			_ = transport.Call(ctx, workspace.MachineID, "credential.revoke", map[string]string{"grant_id": grant.ID}, nil)
		}
	}
}

func validateWorkspaceFeature(cfg ToolConfig, identity WorkspaceIdentity) error {
	if !cfg.Enabled || !cfg.ToolGate {
		return WorkspaceRPCError{Code: "disabled", Message: "virtual computers are disabled"}
	}
	if !cfg.AgentControl.Enabled {
		return WorkspaceRPCError{Code: "agent_control_disabled", Message: "virtual computer agent control is disabled"}
	}
	if cfg.ReadOnly {
		return WorkspaceRPCError{Code: "readonly", Message: "virtual computers are in read-only mode"}
	}
	if strings.TrimSpace(identity.SessionID) == "" {
		return WorkspaceRPCError{Code: "missing_owner", Message: "trusted workspace session identity is required"}
	}
	return nil
}

func ValidateWorkspaceNetworkCIDRs(cidrs []string) error {
	private := []*net.IPNet{mustCIDR("10.0.0.0/8"), mustCIDR("172.16.0.0/12"), mustCIDR("192.168.0.0/16")}
	protected := []*net.IPNet{
		mustCIDR("0.0.0.0/8"), mustCIDR("100.64.0.0/10"), mustCIDR("127.0.0.0/8"), mustCIDR("169.254.0.0/16"),
		mustCIDR("224.0.0.0/4"), mustCIDR("240.0.0.0/4"), mustCIDR("::/128"), mustCIDR("::1/128"),
		mustCIDR("fe80::/10"), mustCIDR("ff00::/8"),
	}
	for _, raw := range cidrs {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			return fmt.Errorf("allowed private CIDR entries must not be empty")
		}
		_, candidate, err := net.ParseCIDR(raw)
		if err != nil {
			return fmt.Errorf("invalid allowed private CIDR %q: %w", raw, err)
		}
		allowed := false
		for _, network := range private {
			if sameIPFamily(candidate.IP, network.IP) && network.Contains(candidate.IP) {
				candidateBits, _ := candidate.Mask.Size()
				networkBits, _ := network.Mask.Size()
				if candidateBits >= networkBits {
					allowed = true
					break
				}
			}
		}
		if !allowed {
			return fmt.Errorf("allowed private CIDR %q is not an IPv4 RFC1918 network", raw)
		}
		for _, blocked := range protected {
			if cidrsOverlap(candidate, blocked) {
				return fmt.Errorf("allowed private CIDR %q overlaps a protected network", raw)
			}
		}
	}
	return nil
}

func mustCIDR(raw string) *net.IPNet {
	_, network, _ := net.ParseCIDR(raw)
	return network
}

func cidrsOverlap(a, b *net.IPNet) bool {
	if a == nil || b == nil || !sameIPFamily(a.IP, b.IP) {
		return false
	}
	return a.Contains(b.IP) || b.Contains(a.IP)
}

func sameIPFamily(a, b net.IP) bool {
	return (a.To4() != nil) == (b.To4() != nil)
}

func localProtectedCIDRs() []string {
	addresses, err := net.InterfaceAddrs()
	if err != nil {
		return nil
	}
	seen := make(map[string]struct{}, len(addresses))
	protected := make([]string, 0, len(addresses))
	for _, address := range addresses {
		var ip net.IP
		switch value := address.(type) {
		case *net.IPNet:
			ip = value.IP
		case *net.IPAddr:
			ip = value.IP
		}
		if ip == nil || ip.IsUnspecified() {
			continue
		}
		ip = ip.To4()
		if ip == nil {
			continue
		}
		cidr := ip.String() + "/32"
		if _, ok := seen[cidr]; ok {
			continue
		}
		seen[cidr] = struct{}{}
		protected = append(protected, cidr)
	}
	sort.Strings(protected)
	return protected
}

func clampWorkspaceJobTimeout(requested, maximum int) int {
	if maximum <= 0 {
		maximum = 3600
	}
	if requested <= 0 || requested > maximum {
		return maximum
	}
	return requested
}

func isTerminalJobState(state string) bool {
	switch state {
	case JobStateCompleted, JobStateFailed, JobStateCanceled, JobStateInterrupted:
		return true
	default:
		return false
	}
}

func auditCommand(command string) (string, string) {
	normalized := strings.Join(strings.Fields(strings.TrimSpace(command)), " ")
	hash := sha256.Sum256([]byte(normalized))
	summary := redactAuditCommand(normalized)
	if runes := []rune(summary); len(runes) > 160 {
		summary = string(runes[:160]) + "…"
	}
	return hex.EncodeToString(hash[:]), summary
}

func redactAuditCommand(command string) string {
	words := strings.Fields(security.RedactSensitiveInfo(security.Scrub(command)))
	redactNext := false
	for index, word := range words {
		trimmed := strings.Trim(word, `"'`)
		lower := strings.ToLower(trimmed)
		if redactNext {
			words[index] = "[redacted]"
			redactNext = false
			continue
		}
		if lower == "bearer" {
			redactNext = true
			continue
		}
		if lower == "-u" || lower == "--user" || lower == "-p" {
			redactNext = true
			continue
		}
		separator := strings.IndexAny(lower, "=:")
		key := strings.TrimLeft(lower, "-")
		if separator >= 0 {
			key = strings.TrimLeft(lower[:separator], "-")
		}
		if !isAuditSecretKey(key) {
			continue
		}
		if separator >= 0 {
			words[index] = trimmed[:separator+1] + "[redacted]"
		} else {
			redactNext = true
		}
	}
	return strings.Join(words, " ")
}

func isAuditSecretKey(key string) bool {
	switch strings.ReplaceAll(key, "-", "_") {
	case "password", "passwd", "pwd", "token", "access_token", "api_key", "secret", "secret_key", "credential", "client_secret":
		return true
	default:
		return false
	}
}

func cleanWorkspacePath(value string) string {
	cleaned := path.Clean("/" + strings.ReplaceAll(strings.TrimSpace(value), "\\", "/"))
	if cleaned == "/" {
		return "."
	}
	return strings.TrimPrefix(cleaned, "/")
}

func browserOrigin(raw string) string {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return ""
	}
	return strings.ToLower(parsed.Scheme) + "://" + strings.ToLower(parsed.Host)
}

func normalizeGrantOrigin(raw string) (string, error) {
	origin := browserOrigin(raw)
	if origin == "" {
		return "", WorkspaceRPCError{Code: "invalid_origin", Message: "browser grants require an absolute http or https origin"}
	}
	parsed, _ := url.Parse(origin)
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", WorkspaceRPCError{Code: "invalid_origin", Message: "browser grants require an http or https origin"}
	}
	return origin, nil
}

func filterRequestedGrantMetadataFields(requested, available []string) []string {
	allowed := make(map[string]struct{}, len(available))
	for _, field := range available {
		allowed[strings.ToLower(strings.TrimSpace(field))] = struct{}{}
	}
	result := make([]string, 0, len(requested))
	seen := make(map[string]struct{}, len(requested))
	for _, field := range requested {
		field = strings.ToLower(strings.TrimSpace(field))
		if _, ok := allowed[field]; !ok {
			continue
		}
		if _, ok := seen[field]; ok {
			continue
		}
		seen[field] = struct{}{}
		result = append(result, field)
	}
	sort.Strings(result)
	return result
}

func clearStringMap(values map[string]string) {
	for key, value := range values {
		bytes := []byte(value)
		clear(bytes)
		delete(values, key)
	}
}

func scrubWorkspaceData(values map[string]interface{}) map[string]interface{} {
	for key, value := range values {
		values[key] = scrubWorkspaceValue(value)
	}
	return values
}

func scrubWorkspaceValue(value interface{}) interface{} {
	switch typed := value.(type) {
	case string:
		return security.Scrub(typed)
	case []interface{}:
		for index := range typed {
			typed[index] = scrubWorkspaceValue(typed[index])
		}
		return typed
	case map[string]interface{}:
		return scrubWorkspaceData(typed)
	default:
		return value
	}
}
