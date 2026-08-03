package deployer

// Package deployer provisions the allow-listed Speech Lab OCI bundle. It is
// deliberately small and Docker-API based: AuraGo never executes a user
// supplied Compose file or arbitrary Docker command.

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"aurago/internal/config"
	"aurago/internal/dockerutil"
)

const (
	ManifestURL             = "https://github.com/antibyte/s2s/releases/latest/download/speech-lab-bundle.json"
	PublisherPrefix         = "ghcr.io/antibyte/"
	ManifestMaxBytes        = 1 << 20
	DockerPullMaxBytes      = 8 << 20
	DockerPullTimeout       = 30 * time.Minute
	DefaultReadinessTimeout = 180 * time.Second
	OwnerLabel              = "speech-lab"
)

type ImageSet struct {
	Gateway   string `json:"gateway"`
	ASR       string `json:"asr"`
	TTS       string `json:"tts"`
	LLM       string `json:"llm"`
	Web       string `json:"web"`
	ModelInit string `json:"model_init"`
}

type BundleService struct {
	Role        string   `json:"role"`
	Image       string   `json:"image"`
	Command     []string `json:"command,omitempty"`
	Environment []string `json:"environment,omitempty"`
	HealthPath  string   `json:"health_path,omitempty"`
	Port        int      `json:"port,omitempty"`
	Restart     string   `json:"restart,omitempty"`
}

type BundleManifest struct {
	SchemaVersion   int                 `json:"schema_version"`
	BundleVersion   string              `json:"bundle_version"`
	ContractVersion string              `json:"contract_version"`
	Publisher       string              `json:"publisher"`
	Images          ImageSet            `json:"images"`
	Network         string              `json:"network"`
	Ports           map[string]int      `json:"ports"`
	Volumes         []string            `json:"volumes"`
	StartOrder      []string            `json:"start_order"`
	Services        []BundleService     `json:"services"`
	StableBackends  map[string][]string `json:"stable_backends"`
	Hardware        map[string]any      `json:"hardware"`
	Models          map[string]any      `json:"models"`
}

type State struct {
	SchemaVersion    int                    `json:"schema_version,omitempty"`
	Mode             string                 `json:"mode"`
	Managed          bool                   `json:"managed"`
	State            string                 `json:"state"`
	Bundle           string                 `json:"bundle"`
	Digest           string                 `json:"digest,omitempty"`
	NetworkID        string                 `json:"network_id,omitempty"`
	ContainerIDs     []string               `json:"container_ids,omitempty"`
	DockerHost       string                 `json:"docker_host,omitempty"`
	ReadinessBaseURL string                 `json:"readiness_base_url,omitempty"`
	LastErrorCode    string                 `json:"last_error_code,omitempty"`
	LastError        string                 `json:"last_error,omitempty"`
	Progress         int                    `json:"progress"`
	LastCheck        time.Time              `json:"last_check,omitempty"`
	Transaction      *DeploymentTransaction `json:"transaction,omitempty"`
}

// DeploymentTransaction is persisted only in deployment.json. It records
// enough information to finish cleanup or restore the last published bundle
// after AuraGo is interrupted during a managed update.
type DeploymentTransaction struct {
	ID                       string            `json:"id,omitempty"`
	Phase                    string            `json:"phase"`
	PreviousState            string            `json:"previous_state,omitempty"`
	PreviousProgress         int               `json:"previous_progress,omitempty"`
	PreviousBundle           string            `json:"previous_bundle,omitempty"`
	PreviousDigest           string            `json:"previous_digest,omitempty"`
	PreviousNetworkID        string            `json:"previous_network_id,omitempty"`
	PreviousContainerIDs     []string          `json:"previous_container_ids,omitempty"`
	PreviousDockerHost       string            `json:"previous_docker_host,omitempty"`
	PreviousReadinessBaseURL string            `json:"previous_readiness_base_url,omitempty"`
	Backups                  []ContainerBackup `json:"backups,omitempty"`
	NewContainerIDs          []string          `json:"new_container_ids,omitempty"`
	StartedContainerIDs      []string          `json:"started_container_ids,omitempty"`
	CreatedNetworkID         string            `json:"created_network_id,omitempty"`
	CreatedNetworkName       string            `json:"created_network_name,omitempty"`
	CreatedVolumes           []string          `json:"created_volumes,omitempty"`
	AuraGoContainerID        string            `json:"aurago_container_id,omitempty"`
	AuraGoNetworkID          string            `json:"aurago_network_id,omitempty"`
	AuraGoConnected          bool              `json:"aurago_connected,omitempty"`
	DockerHost               string            `json:"docker_host,omitempty"`
	ReadinessBaseURL         string            `json:"readiness_base_url,omitempty"`
}

type ContainerBackup struct {
	ID          string `json:"id"`
	StableName  string `json:"stable_name"`
	BackupName  string `json:"backup_name"`
	NetworkName string `json:"network_name,omitempty"`
	WasRunning  bool   `json:"was_running"`
	WasAttached bool   `json:"was_attached"`
}

// PublicState deliberately omits container and network identifiers from the
// HTTP status surface. Those identifiers are only needed for local lifecycle
// reconciliation and remain in the private deployment state file.
type PublicState struct {
	Mode             string    `json:"mode"`
	Managed          bool      `json:"managed"`
	State            string    `json:"state"`
	Bundle           string    `json:"bundle"`
	RequestedBundle  string    `json:"requested_bundle"`
	Digest           string    `json:"digest,omitempty"`
	LastErrorCode    string    `json:"last_error_code,omitempty"`
	LastError        string    `json:"last_error,omitempty"`
	Progress         int       `json:"progress"`
	LastCheck        time.Time `json:"last_check,omitempty"`
	RecoveryPending  bool      `json:"recovery_pending"`
	CleanupPending   bool      `json:"cleanup_pending"`
	CleanupAvailable bool      `json:"cleanup_available"`
}

type Error struct {
	Code string
	Err  error
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	return e.Code + ": " + e.Err.Error()
}
func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func Code(err error) string {
	var coded *Error
	if errors.As(err, &coded) && coded.Code != "" {
		return coded.Code
	}
	return "speech_lab_deployment_failed"
}

type Option func(*Manager)

func WithHTTPClient(client *http.Client) Option {
	return func(m *Manager) {
		if client != nil {
			m.httpClient = client
		}
	}
}
func WithManifestURL(raw string) Option {
	return func(m *Manager) {
		if strings.TrimSpace(raw) != "" {
			m.manifestURL = raw
			m.requireManifestChecksum = false
		}
	}
}
func WithDockerClient(client *dockerutil.Client) Option {
	return func(m *Manager) { m.docker = client }
}
func WithReadinessTimeout(timeout time.Duration) Option {
	return func(m *Manager) {
		if timeout > 0 {
			m.readinessTimeout = timeout
		}
	}
}

type Manager struct {
	mu                      sync.Mutex
	operationMu             sync.Mutex
	persistMu               sync.Mutex
	cfg                     config.SpeechLabConfig
	runningInDocker         bool
	dockerEnabled           bool
	dockerReadOnly          bool
	dataPath                string
	persistEnabled          bool
	manifestURL             string
	requireManifestChecksum bool
	httpClient              *http.Client
	docker                  *dockerutil.Client
	logger                  *slog.Logger
	readinessTimeout        time.Duration
	stateWriter             func(string, []byte, os.FileMode) error
	state                   State
}

// RuntimeAccess is an immutable Docker permission/transport snapshot supplied
// by the server whenever the configuration is replaced.
type RuntimeAccess struct {
	DockerEnabled  bool
	DockerReadOnly bool
	Docker         *dockerutil.Client
}

type operationSnapshot struct {
	cfg              config.SpeechLabConfig
	runningInDocker  bool
	dockerEnabled    bool
	dockerReadOnly   bool
	docker           *dockerutil.Client
	dockerHost       string
	readinessBaseURL string
}

func NewManager(cfg config.SpeechLabConfig, runningInDocker, dockerEnabled, dockerReadOnly bool, dataDir string, logger *slog.Logger, options ...Option) *Manager {
	if logger == nil {
		logger = slog.Default()
	}
	persistEnabled := strings.TrimSpace(dataDir) != ""
	if !persistEnabled {
		dataDir = "data"
	}
	m := &Manager{
		cfg: cfg, runningInDocker: runningInDocker, dockerEnabled: dockerEnabled, dockerReadOnly: dockerReadOnly, dataPath: filepath.Join(dataDir, "speech_lab", "deployment.json"), persistEnabled: persistEnabled,
		manifestURL: ManifestURL, requireManifestChecksum: true, httpClient: &http.Client{Timeout: 20 * time.Second},
		docker: dockerutil.NewClient("", 30*time.Second), logger: logger,
		readinessTimeout: DefaultReadinessTimeout,
		stateWriter:      config.WriteFileAtomic,
		state:            State{SchemaVersion: 2, Mode: cfg.Deployment.Mode, Managed: cfg.Deployment.Mode == "managed", State: "disabled"},
	}
	for _, option := range options {
		if option != nil {
			option(m)
		}
	}
	m.loadState()
	return m
}

func (m *Manager) Status() State { m.mu.Lock(); defer m.mu.Unlock(); return cloneState(m.state) }
func (m *Manager) PublicStatus() PublicState {
	cfg, state := m.snapshot()
	phase := ""
	if state.Transaction != nil {
		phase = state.Transaction.Phase
	}
	return PublicState{
		Mode: cfg.Deployment.Mode, Managed: cfg.Deployment.Mode == "managed", State: state.State,
		Bundle: state.Bundle, RequestedBundle: cfg.Deployment.Bundle, Digest: state.Digest,
		LastErrorCode: state.LastErrorCode, LastError: state.LastError, Progress: state.Progress, LastCheck: state.LastCheck,
		RecoveryPending:  state.Transaction != nil && phase != "commit_cleanup",
		CleanupPending:   state.Transaction != nil && phase == "commit_cleanup",
		CleanupAvailable: len(state.ContainerIDs) > 0 || state.NetworkID != "" || state.Transaction != nil,
	}
}
func cloneState(in State) State {
	in.ContainerIDs = append([]string(nil), in.ContainerIDs...)
	if in.Transaction != nil {
		copy := *in.Transaction
		copy.PreviousContainerIDs = append([]string(nil), in.Transaction.PreviousContainerIDs...)
		copy.NewContainerIDs = append([]string(nil), in.Transaction.NewContainerIDs...)
		copy.StartedContainerIDs = append([]string(nil), in.Transaction.StartedContainerIDs...)
		copy.Backups = append([]ContainerBackup(nil), in.Transaction.Backups...)
		copy.CreatedVolumes = append([]string(nil), in.Transaction.CreatedVolumes...)
		in.Transaction = &copy
	}
	return in
}

func (m *Manager) Reconfigure(cfg config.SpeechLabConfig, accesses ...RuntimeAccess) {
	m.mu.Lock()
	m.cfg = cfg
	m.state.Mode = cfg.Deployment.Mode
	m.state.Managed = cfg.Deployment.Mode == "managed"
	if len(accesses) > 0 {
		access := accesses[0]
		oldDocker := m.docker
		m.dockerEnabled = access.DockerEnabled
		m.dockerReadOnly = access.DockerReadOnly
		if access.Docker != nil {
			m.docker = access.Docker
		}
		if oldDocker != nil && oldDocker != m.docker {
			oldDocker.CloseIdleConnections()
		}
	}
	m.mu.Unlock()
}

func (m *Manager) AutoStart(ctx context.Context) error {
	if !m.operationMu.TryLock() {
		return &Error{Code: "speech_lab_deployment_busy", Err: fmt.Errorf("deployment operation already running")}
	}
	defer m.finishOperation()
	op := m.operationSnapshot()
	_, state := m.snapshot()
	if state.Transaction != nil {
		if err := m.requireDockerWrite(op); err != nil {
			m.fail(err)
			return err
		}
		if err := m.recoverTransaction(ctx, op); err != nil {
			m.fail(err)
			return err
		}
		_, state = m.snapshot()
	}
	cfg := op.cfg
	installed := cfg.Deployment.Mode == "managed" && len(state.ContainerIDs) > 0
	autoUpdate := installed && cfg.Deployment.AutoUpdate
	if autoUpdate {
		previous := state
		m.setOperationState("pulling")
		return m.installLocked(ctx, op, true, previous)
	}
	if !installed || !cfg.Deployment.AutoStart {
		return nil
	}
	installedOp, closeInstalledOp := operationForDockerHost(op, state.DockerHost)
	defer closeInstalledOp()
	if state.ReadinessBaseURL != "" {
		installedOp.readinessBaseURL = state.ReadinessBaseURL
	}
	allRunning, err := m.containersRunning(ctx, installedOp, state.ContainerIDs)
	if err != nil {
		return err
	}
	if allRunning {
		return nil
	}
	m.setOperationState("starting")
	return m.startLocked(ctx, op, state)
}

func (m *Manager) Install(ctx context.Context) error { return m.install(ctx, false) }
func (m *Manager) Update(ctx context.Context) error  { return m.install(ctx, true) }

func (m *Manager) install(ctx context.Context, update bool) error {
	previous, op, err := m.begin("pulling")
	if err != nil {
		return err
	}
	defer m.finishOperation()
	return m.installLocked(ctx, op, update, previous)
}

func (m *Manager) installLocked(ctx context.Context, op operationSnapshot, update bool, previous State) (result error) {
	if err := m.recoverTransaction(ctx, op); err != nil {
		m.fail(err)
		return err
	}
	_, currentState := m.snapshot()
	oldState := previous
	if oldState.Transaction != nil {
		oldState = currentState
	}
	if err := m.requireManaged(op); err != nil {
		m.fail(err)
		return err
	}
	if currentState.DockerHost != "" && currentState.DockerHost != op.dockerHost && (len(currentState.ContainerIDs) > 0 || currentState.NetworkID != "") {
		err := &Error{Code: "speech_lab_cleanup_required", Err: fmt.Errorf("the installed Speech Lab bundle belongs to a different Docker host; remove it before installing on the new host")}
		m.fail(err)
		return err
	}
	manifest, raw, err := m.fetchManifest(ctx)
	if err != nil {
		m.fail(err)
		return err
	}
	if err := validateManifest(manifest); err != nil {
		err = &Error{Code: "speech_lab_bundle_incompatible", Err: err}
		m.fail(err)
		return err
	}
	sum := sha256.Sum256(raw)
	digest := hex.EncodeToString(sum[:])
	if update {
		m.logger.Info("Updating managed Speech Lab bundle", "bundle", manifest.BundleVersion)
	}
	if ids, identical, err := m.matchingDeployment(ctx, op, manifest, digest); err != nil {
		m.fail(err)
		return err
	} else if identical {
		if err := m.persist(); err != nil {
			wrapped := &Error{Code: "speech_lab_state_persist_failed", Err: err}
			m.fail(wrapped)
			return wrapped
		}
		for _, id := range ids {
			if err := m.containerAction(ctx, op, http.MethodPost, "/containers/"+url.PathEscape(id)+"/start"); err != nil {
				m.fail(err)
				return err
			}
		}
		if err := m.waitReady(ctx, op); err != nil {
			m.fail(err)
			return err
		}
		m.mu.Lock()
		m.state.Bundle, m.state.Digest, m.state.ContainerIDs, m.state.DockerHost = manifest.BundleVersion, digest, ids, op.dockerHost
		m.state.ReadinessBaseURL = op.readinessBaseURL
		m.state.State, m.state.Progress, m.state.LastErrorCode, m.state.LastError = "ready", 100, "", ""
		m.mu.Unlock()
		if err := m.persist(); err != nil {
			return &Error{Code: "speech_lab_state_persist_failed", Err: err}
		}
		return nil
	}
	previousDockerHost := oldState.DockerHost
	if previousDockerHost == "" && (len(oldState.ContainerIDs) > 0 || oldState.NetworkID != "") {
		previousDockerHost = op.dockerHost
	}
	previousReadinessBaseURL := oldState.ReadinessBaseURL
	if previousReadinessBaseURL == "" && (len(oldState.ContainerIDs) > 0 || oldState.NetworkID != "") {
		previousReadinessBaseURL = op.readinessBaseURL
	}
	transaction := &DeploymentTransaction{
		ID: transactionID(), Phase: "preparing_resources", PreviousState: oldState.State, PreviousProgress: oldState.Progress,
		PreviousBundle: oldState.Bundle, PreviousDigest: oldState.Digest,
		PreviousNetworkID: oldState.NetworkID, PreviousContainerIDs: append([]string(nil), oldState.ContainerIDs...),
		PreviousDockerHost:       previousDockerHost,
		PreviousReadinessBaseURL: previousReadinessBaseURL,
		DockerHost:               op.dockerHost, ReadinessBaseURL: op.readinessBaseURL,
	}
	if err := m.setTransaction(transaction, 50); err != nil {
		m.fail(err)
		return err
	}
	defer func() {
		if result != nil {
			rollbackCtx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			rollbackErr := m.rollbackTransaction(rollbackCtx, op)
			cancel()
			m.mu.Lock()
			m.state.LastErrorCode = Code(result)
			m.state.LastError = safeError(result)
			if rollbackErr != nil {
				m.state.State = "error"
				m.state.LastError = safeError(fmt.Errorf("%v; rollback: %w", result, rollbackErr))
			}
			m.mu.Unlock()
			if persistErr := m.persist(); persistErr != nil && m.logger != nil {
				m.logger.Error("Failed to persist Speech Lab rollback status", "error", persistErr)
			}
		}
	}()
	for _, image := range uniqueImages(manifest.Images) {
		if err := m.pull(ctx, op, image); err != nil {
			m.fail(err)
			return err
		}
	}
	if err := m.prepareResources(ctx, op, manifest); err != nil {
		m.fail(err)
		return err
	}
	m.mu.Lock()
	if m.state.Transaction != nil {
		m.state.Transaction.Phase = "replacing"
	}
	m.mu.Unlock()
	if err := m.persist(); err != nil {
		return &Error{Code: "speech_lab_state_persist_failed", Err: err}
	}
	ids, identical, err := m.replaceServices(ctx, op, manifest, digest)
	if err != nil {
		return err
	}
	if identical {
		m.logger.Debug("Managed Speech Lab bundle is already identical", "bundle", manifest.BundleVersion)
	}
	if err := m.waitReady(ctx, op); err != nil {
		return err
	}
	m.mu.Lock()
	m.state.Bundle, m.state.Digest, m.state.ContainerIDs, m.state.DockerHost = manifest.BundleVersion, digest, append([]string(nil), ids...), op.dockerHost
	m.state.ReadinessBaseURL = op.readinessBaseURL
	m.state.State, m.state.Progress, m.state.LastErrorCode, m.state.LastError = "ready", 100, "", ""
	if m.state.Transaction != nil {
		m.state.Transaction.Phase = "commit_cleanup"
	}
	m.mu.Unlock()
	if err := m.persist(); err != nil {
		return &Error{Code: "speech_lab_state_persist_failed", Err: err}
	}
	cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Minute)
	err = m.commitTransaction(cleanupCtx, op)
	cleanupCancel()
	if err != nil {
		m.logger.Warn("Speech Lab update succeeded but rollback-container cleanup is pending", "error", err)
		return nil
	}
	return nil
}

func (m *Manager) Start(ctx context.Context) error {
	previous, op, err := m.begin("starting")
	if err != nil {
		return err
	}
	defer m.finishOperation()
	return m.startLocked(ctx, op, previous)
}

func (m *Manager) startLocked(ctx context.Context, op operationSnapshot, previous State) error {
	if err := m.recoverTransaction(ctx, op); err != nil {
		m.fail(err)
		return err
	}
	_, state := m.snapshot()
	if err := m.requireManaged(op); err != nil {
		m.fail(err)
		return err
	}
	installedOp, closeInstalledOp := operationForDockerHost(op, state.DockerHost)
	defer closeInstalledOp()
	if state.ReadinessBaseURL != "" {
		installedOp.readinessBaseURL = state.ReadinessBaseURL
	}
	ids := append([]string(nil), state.ContainerIDs...)
	if len(ids) == 0 {
		return m.installLocked(ctx, op, false, previous)
	}
	if err := m.persist(); err != nil {
		wrapped := &Error{Code: "speech_lab_state_persist_failed", Err: err}
		m.fail(wrapped)
		return wrapped
	}
	for _, id := range ids {
		if err := m.containerAction(ctx, installedOp, http.MethodPost, "/containers/"+url.PathEscape(id)+"/start"); err != nil {
			m.fail(err)
			return err
		}
	}
	if err := m.waitReady(ctx, installedOp); err != nil {
		m.fail(err)
		return err
	}
	m.mu.Lock()
	m.state.State, m.state.Progress = "ready", 100
	m.mu.Unlock()
	if err := m.persist(); err != nil {
		return &Error{Code: "speech_lab_state_persist_failed", Err: err}
	}
	return nil
}

func (m *Manager) Stop(ctx context.Context) error {
	_, op, err := m.begin("stopping")
	if err != nil {
		return err
	}
	defer m.finishOperation()
	if err := m.requireDockerWrite(op); err != nil {
		m.fail(err)
		return err
	}
	if err := m.recoverTransaction(ctx, op); err != nil {
		m.fail(err)
		return err
	}
	m.mu.Lock()
	ids := append([]string(nil), m.state.ContainerIDs...)
	installedHost := m.state.DockerHost
	m.mu.Unlock()
	installedOp, closeInstalledOp := operationForDockerHost(op, installedHost)
	defer closeInstalledOp()
	if err := m.persist(); err != nil {
		wrapped := &Error{Code: "speech_lab_state_persist_failed", Err: err}
		m.fail(wrapped)
		return wrapped
	}
	var stopErrors []error
	for _, id := range ids {
		if err := m.containerAction(ctx, installedOp, http.MethodPost, "/containers/"+url.PathEscape(id)+"/stop?t=10"); err != nil {
			stopErrors = append(stopErrors, fmt.Errorf("stop container %q: %w", id, err))
		}
	}
	if err := errors.Join(stopErrors...); err != nil {
		wrapped := &Error{Code: "speech_lab_start_failed", Err: err}
		m.fail(wrapped)
		return wrapped
	}
	m.mu.Lock()
	m.state.State, m.state.Progress = "stopped", 0
	m.mu.Unlock()
	if err := m.persist(); err != nil {
		return &Error{Code: "speech_lab_state_persist_failed", Err: err}
	}
	return nil
}

func (m *Manager) Remove(ctx context.Context) error {
	_, op, err := m.begin("removing")
	if err != nil {
		return err
	}
	defer m.finishOperation()
	if err := m.requireDockerWrite(op); err != nil {
		m.fail(err)
		return err
	}
	if err := m.recoverTransaction(ctx, op); err != nil {
		m.fail(err)
		return err
	}
	m.mu.Lock()
	ids, network := append([]string(nil), m.state.ContainerIDs...), m.state.NetworkID
	installedHost := m.state.DockerHost
	m.mu.Unlock()
	installedOp, closeInstalledOp := operationForDockerHost(op, installedHost)
	defer closeInstalledOp()
	if err := m.persist(); err != nil {
		wrapped := &Error{Code: "speech_lab_state_persist_failed", Err: err}
		m.fail(wrapped)
		return wrapped
	}
	for _, id := range ids {
		if err := m.removeOwnedContainer(ctx, installedOp, id); err != nil {
			m.fail(err)
			return err
		}
	}
	if network != "" {
		var inspected struct {
			Labels map[string]string `json:"Labels"`
		}
		status, err := installedOp.docker.DoJSON(ctx, http.MethodGet, "/networks/"+url.PathEscape(network), nil, &inspected)
		if err != nil && status != http.StatusNotFound {
			m.fail(err)
			return err
		}
		if status != http.StatusNotFound {
			if !dockerutil.ManagedBy(inspected.Labels, OwnerLabel) {
				err := &Error{Code: "speech_lab_start_failed", Err: fmt.Errorf("refusing to remove unowned network %q", network)}
				m.fail(err)
				return err
			}
			if installedOp.runningInDocker {
				containerID := strings.TrimSpace(os.Getenv("HOSTNAME"))
				if containerID != "" {
					container, found, inspectErr := m.inspectContainer(ctx, installedOp, containerID)
					if inspectErr != nil {
						m.fail(inspectErr)
						return inspectErr
					}
					if found {
						attached, inspectErr := m.networkContainsContainer(ctx, installedOp, network, container.ID)
						if inspectErr != nil {
							m.fail(inspectErr)
							return inspectErr
						}
						if attached {
							status, disconnectErr := installedOp.docker.DoJSON(ctx, http.MethodPost, "/networks/"+url.PathEscape(network)+"/disconnect", map[string]any{"Container": container.ID, "Force": true}, nil)
							if disconnectErr != nil && status != http.StatusNotFound {
								err := &Error{Code: "speech_lab_start_failed", Err: disconnectErr}
								m.fail(err)
								return err
							}
							if stillAttached, verifyErr := m.networkContainsContainer(ctx, installedOp, network, container.ID); verifyErr != nil || stillAttached {
								if verifyErr == nil {
									verifyErr = fmt.Errorf("AuraGo container is still attached to Speech Lab network %q", network)
								}
								err := &Error{Code: "speech_lab_start_failed", Err: verifyErr}
								m.fail(err)
								return err
							}
						}
					}
				}
			}
			if err := m.containerAction(ctx, installedOp, http.MethodDelete, "/networks/"+url.PathEscape(network)); err != nil {
				m.fail(err)
				return err
			}
			var verify any
			status, verifyErr := installedOp.docker.DoJSON(ctx, http.MethodGet, "/networks/"+url.PathEscape(network), nil, &verify)
			if status != http.StatusNotFound {
				if verifyErr == nil {
					verifyErr = fmt.Errorf("Speech Lab network %q still exists after removal", network)
				}
				err := &Error{Code: "speech_lab_start_failed", Err: verifyErr}
				m.fail(err)
				return err
			}
		}
	}
	m.mu.Lock()
	m.state = State{SchemaVersion: 2, Mode: m.cfg.Deployment.Mode, Managed: m.cfg.Deployment.Mode == "managed", State: "disabled"}
	m.mu.Unlock()
	if err := m.persist(); err != nil {
		return &Error{Code: "speech_lab_state_persist_failed", Err: err}
	}
	return nil
}

func (m *Manager) requireManaged(op operationSnapshot) error {
	if op.cfg.Deployment.Mode != "managed" {
		return &Error{Code: "speech_lab_bundle_unavailable", Err: fmt.Errorf("Speech Lab is configured as external")}
	}
	if !op.cfg.Enabled {
		return &Error{Code: "speech_lab_bundle_unavailable", Err: fmt.Errorf("Speech Lab is disabled")}
	}
	return m.requireDockerWrite(op)
}

func (m *Manager) requireDockerWrite(op operationSnapshot) error {
	if op.docker == nil || !op.dockerEnabled || op.dockerReadOnly {
		return &Error{Code: "speech_lab_docker_unavailable", Err: fmt.Errorf("Docker lifecycle is disabled")}
	}
	return nil
}

func (m *Manager) operationSnapshot() operationSnapshot {
	m.mu.Lock()
	defer m.mu.Unlock()
	return operationSnapshot{
		cfg: m.cfg, runningInDocker: m.runningInDocker,
		dockerEnabled: m.dockerEnabled, dockerReadOnly: m.dockerReadOnly, docker: m.docker,
		dockerHost: dockerClientHost(m.docker), readinessBaseURL: deploymentReadinessBase(m.cfg, m.runningInDocker),
	}
}

func (m *Manager) setOperationState(state string) {
	m.mu.Lock()
	m.state.State, m.state.LastErrorCode, m.state.LastError = state, "", ""
	m.mu.Unlock()
}

func (m *Manager) begin(state string) (State, operationSnapshot, error) {
	if !m.operationMu.TryLock() {
		return State{}, operationSnapshot{}, &Error{Code: "speech_lab_deployment_busy", Err: fmt.Errorf("deployment operation already running")}
	}
	m.mu.Lock()
	previous := cloneState(m.state)
	op := operationSnapshot{
		cfg: m.cfg, runningInDocker: m.runningInDocker,
		dockerEnabled: m.dockerEnabled, dockerReadOnly: m.dockerReadOnly, docker: m.docker,
		dockerHost: dockerClientHost(m.docker), readinessBaseURL: deploymentReadinessBase(m.cfg, m.runningInDocker),
	}
	m.state.State, m.state.LastErrorCode, m.state.LastError = state, "", ""
	m.mu.Unlock()
	return previous, op, nil
}
func (m *Manager) finishOperation() {
	if err := m.persist(); err != nil && m.logger != nil {
		m.logger.Error("Failed to persist Speech Lab deployment state", "error", err)
	}
	m.operationMu.Unlock()
}
func (m *Manager) fail(err error) {
	m.mu.Lock()
	m.state.State, m.state.Progress, m.state.LastErrorCode, m.state.LastError = "error", 0, Code(err), safeError(err)
	m.mu.Unlock()
	if persistErr := m.persist(); persistErr != nil && m.logger != nil {
		m.logger.Error("Failed to persist Speech Lab deployment failure", "error", persistErr)
	}
}

func transactionID() string {
	seed := sha256.Sum256([]byte(fmt.Sprintf("%d", time.Now().UnixNano())))
	return hex.EncodeToString(seed[:8])
}

func dockerClientHost(client *dockerutil.Client) string {
	if client == nil {
		return ""
	}
	return client.Host()
}

func operationForDockerHost(op operationSnapshot, host string) (operationSnapshot, func()) {
	host = strings.TrimSpace(host)
	if host == "" || host == op.dockerHost {
		return op, func() {}
	}
	client := dockerutil.NewClient(host, 30*time.Second)
	op.docker = client
	op.dockerHost = client.Host()
	return op, client.CloseIdleConnections
}

func deploymentReadinessBase(cfg config.SpeechLabConfig, runningInDocker bool) string {
	base := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if base == "" {
		if runningInDocker {
			return "http://s2s-vulkan:8765"
		}
		return config.DefaultManagedSpeechLabBaseURL
	}
	return base
}
func safeError(err error) string {
	if err == nil {
		return ""
	}
	text := err.Error()
	if len(text) > 512 {
		text = text[:512]
	}
	return text
}

func (m *Manager) fetchManifest(ctx context.Context) (BundleManifest, []byte, error) {
	var out BundleManifest
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, m.manifestURL, nil)
	if err != nil {
		return out, nil, &Error{Code: "speech_lab_bundle_unavailable", Err: err}
	}
	resp, err := m.httpClient.Do(req)
	if err != nil {
		return out, nil, &Error{Code: "speech_lab_bundle_unavailable", Err: err}
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return out, nil, &Error{Code: "speech_lab_bundle_unavailable", Err: fmt.Errorf("manifest returned HTTP %d", resp.StatusCode)}
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, ManifestMaxBytes+1))
	if err != nil {
		return out, nil, &Error{Code: "speech_lab_bundle_unavailable", Err: err}
	}
	if len(raw) > ManifestMaxBytes {
		return out, nil, &Error{Code: "speech_lab_bundle_incompatible", Err: fmt.Errorf("manifest exceeds 1 MiB")}
	}
	if m.requireManifestChecksum {
		checksumReq, checksumErr := http.NewRequestWithContext(ctx, http.MethodGet, m.manifestURL+".sha256", nil)
		if checksumErr != nil {
			return out, nil, &Error{Code: "speech_lab_bundle_unavailable", Err: checksumErr}
		}
		checksumResp, checksumErr := m.httpClient.Do(checksumReq)
		if checksumErr != nil {
			return out, nil, &Error{Code: "speech_lab_bundle_unavailable", Err: checksumErr}
		}
		checksumBytes, readErr := io.ReadAll(io.LimitReader(checksumResp.Body, 4096))
		checksumResp.Body.Close()
		if checksumResp.StatusCode != http.StatusOK || readErr != nil {
			return out, nil, &Error{Code: "speech_lab_bundle_incompatible", Err: fmt.Errorf("bundle checksum is unavailable")}
		}
		parts := strings.Fields(string(checksumBytes))
		sum := sha256.Sum256(raw)
		if len(parts) == 0 || !strings.EqualFold(parts[0], hex.EncodeToString(sum[:])) {
			return out, nil, &Error{Code: "speech_lab_bundle_incompatible", Err: fmt.Errorf("bundle checksum mismatch")}
		}
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return out, nil, &Error{Code: "speech_lab_bundle_incompatible", Err: err}
	}
	return out, raw, nil
}

func validateManifest(m BundleManifest) error {
	if m.SchemaVersion != 1 || strings.TrimSpace(m.ContractVersion) != "speech-lab/v1" {
		return fmt.Errorf("unsupported bundle schema or contract")
	}
	publisher := strings.ToLower(strings.TrimSpace(m.Publisher))
	if publisher != strings.TrimSuffix(PublisherPrefix, "/") && !strings.HasPrefix(publisher, PublisherPrefix) {
		return fmt.Errorf("publisher is not allow-listed")
	}
	if strings.TrimSpace(m.BundleVersion) == "" || strings.TrimSpace(m.Network) == "" {
		return fmt.Errorf("bundle version and network are required")
	}
	for _, image := range uniqueImages(m.Images) {
		if err := validateImage(image); err != nil {
			return err
		}
	}
	if len(m.Services) == 0 || len(m.StartOrder) == 0 {
		return fmt.Errorf("bundle has no services")
	}
	return nil
}
func validateImage(ref string) error {
	ref = strings.TrimSpace(ref)
	if !strings.HasPrefix(strings.ToLower(ref), PublisherPrefix) || !strings.Contains(ref, "@sha256:") {
		return fmt.Errorf("image reference must use allow-listed GHCR digest: %q", ref)
	}
	digest := strings.TrimPrefix(strings.SplitN(ref, "@", 2)[1], "sha256:")
	if len(digest) != 64 {
		return fmt.Errorf("image reference must contain a full sha256 digest: %q", ref)
	}
	for _, char := range digest {
		if !strings.ContainsRune("0123456789abcdefABCDEF", char) {
			return fmt.Errorf("image reference contains an invalid sha256 digest: %q", ref)
		}
	}
	if len(ref) > 512 {
		return fmt.Errorf("image reference is too long")
	}
	return nil
}
func uniqueImages(images ImageSet) []string {
	seen := map[string]bool{}
	out := make([]string, 0, 6)
	for _, image := range []string{images.Gateway, images.ASR, images.TTS, images.LLM, images.Web, images.ModelInit} {
		if image != "" && !seen[image] {
			seen[image] = true
			out = append(out, image)
		}
	}
	return out
}

func (m *Manager) pull(ctx context.Context, op operationSnapshot, image string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, dockerutil.Endpoint("images/create?fromImage="+url.QueryEscape(image)), nil)
	if err != nil {
		return &Error{Code: "speech_lab_pull_failed", Err: err}
	}
	client := op.docker.HTTPClientWithTimeout(DockerPullTimeout)
	if client == nil {
		return &Error{Code: "speech_lab_pull_failed", Err: fmt.Errorf("Docker client is unavailable")}
	}
	resp, err := client.Do(req)
	if err != nil {
		return &Error{Code: "speech_lab_pull_failed", Err: err}
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &Error{Code: "speech_lab_pull_failed", Err: fmt.Errorf("Docker pull returned HTTP %d", resp.StatusCode)}
	}
	scanner := bufio.NewScanner(io.LimitReader(resp.Body, DockerPullMaxBytes))
	scanner.Buffer(make([]byte, 64<<10), 1<<20)
	for scanner.Scan() {
		var event struct {
			Error       string `json:"error"`
			ErrorDetail struct {
				Message string `json:"message"`
			} `json:"errorDetail"`
		}
		if json.Unmarshal(scanner.Bytes(), &event) != nil {
			continue
		}
		message := strings.TrimSpace(event.ErrorDetail.Message)
		if message == "" {
			message = strings.TrimSpace(event.Error)
		}
		if message != "" {
			return &Error{Code: "speech_lab_pull_failed", Err: fmt.Errorf("Docker pull failed: %s", safeDockerDetail(message))}
		}
	}
	if err := scanner.Err(); err != nil {
		return &Error{Code: "speech_lab_pull_failed", Err: err}
	}
	return nil
}

func safeDockerDetail(value string) string {
	value = strings.TrimSpace(value)
	if len(value) > 256 {
		value = value[:256]
	}
	return value
}

type networkCreateResponse struct {
	ID string `json:"Id"`
}

func (m *Manager) prepareResources(ctx context.Context, op operationSnapshot, manifest BundleManifest) error {
	_, state := m.snapshot()
	if state.Transaction == nil {
		return &Error{Code: "speech_lab_state_persist_failed", Err: fmt.Errorf("deployment transaction is unavailable")}
	}
	transaction := state.Transaction

	var existingNetwork struct {
		ID     string            `json:"Id"`
		Labels map[string]string `json:"Labels"`
	}
	status, inspectErr := op.docker.DoJSON(ctx, http.MethodGet, "/networks/"+url.PathEscape(manifest.Network), nil, &existingNetwork)
	var networkID string
	switch {
	case inspectErr == nil:
		if !dockerutil.ManagedBy(existingNetwork.Labels, OwnerLabel) {
			return &Error{Code: "speech_lab_start_failed", Err: fmt.Errorf("network %q is not owned by AuraGo Speech Lab", manifest.Network)}
		}
		networkID = existingNetwork.ID
	case status == http.StatusNotFound:
		m.mu.Lock()
		m.state.Transaction.CreatedNetworkName = manifest.Network
		m.mu.Unlock()
		if err := m.persist(); err != nil {
			return &Error{Code: "speech_lab_state_persist_failed", Err: err}
		}
		labels := dockerutil.ManagedLabels(OwnerLabel, "speech-lab", "network", manifest.BundleVersion)
		labels["aurago.transaction"] = transaction.ID
		var created networkCreateResponse
		if _, err := op.docker.DoJSON(ctx, http.MethodPost, "/networks/create", map[string]any{"Name": manifest.Network, "Driver": "bridge", "Labels": labels}, &created); err != nil {
			return &Error{Code: "speech_lab_start_failed", Err: err}
		}
		networkID = created.ID
		m.mu.Lock()
		m.state.Transaction.CreatedNetworkID = created.ID
		m.mu.Unlock()
		if err := m.persist(); err != nil {
			return &Error{Code: "speech_lab_state_persist_failed", Err: err}
		}
	default:
		return &Error{Code: "speech_lab_start_failed", Err: inspectErr}
	}
	m.mu.Lock()
	m.state.NetworkID = networkID
	m.mu.Unlock()
	if err := m.persist(); err != nil {
		return &Error{Code: "speech_lab_state_persist_failed", Err: err}
	}

	if op.runningInDocker {
		// The AuraGo container must join the private network so the managed
		// gateway hostname is reachable without publishing a host port.
		if containerID := strings.TrimSpace(os.Getenv("HOSTNAME")); containerID != "" {
			container, found, err := m.inspectContainer(ctx, op, containerID)
			if err != nil {
				return err
			}
			_, alreadyAttached := container.NetworkSettings.Networks[manifest.Network]
			if found && !alreadyAttached {
				m.mu.Lock()
				m.state.Transaction.AuraGoContainerID = containerID
				m.state.Transaction.AuraGoNetworkID = networkID
				m.state.Transaction.AuraGoConnected = true
				m.mu.Unlock()
				if err := m.persist(); err != nil {
					return &Error{Code: "speech_lab_state_persist_failed", Err: err}
				}
				status, connectErr := op.docker.DoJSON(ctx, http.MethodPost, "/networks/"+url.PathEscape(manifest.Network)+"/connect", map[string]any{"Container": containerID}, nil)
				if connectErr != nil && status != http.StatusConflict {
					return &Error{Code: "speech_lab_start_failed", Err: connectErr}
				}
			}
		}
	}

	for _, volume := range manifest.Volumes {
		var existing struct {
			Name   string            `json:"Name"`
			Labels map[string]string `json:"Labels"`
		}
		status, inspectErr := op.docker.DoJSON(ctx, http.MethodGet, "/volumes/"+url.PathEscape(volume), nil, &existing)
		if inspectErr == nil {
			if !dockerutil.ManagedBy(existing.Labels, OwnerLabel) {
				return &Error{Code: "speech_lab_start_failed", Err: fmt.Errorf("volume %q is not owned by AuraGo Speech Lab", volume)}
			}
			continue
		}
		if status != http.StatusNotFound {
			return &Error{Code: "speech_lab_start_failed", Err: inspectErr}
		}
		m.mu.Lock()
		m.state.Transaction.CreatedVolumes = append(m.state.Transaction.CreatedVolumes, volume)
		m.mu.Unlock()
		if err := m.persist(); err != nil {
			return &Error{Code: "speech_lab_state_persist_failed", Err: err}
		}
		labels := dockerutil.ManagedLabels(OwnerLabel, "speech-lab", "volume", manifest.BundleVersion)
		labels["aurago.transaction"] = transaction.ID
		var ignored any
		if _, err := op.docker.DoJSON(ctx, http.MethodPost, "/volumes/create", map[string]any{"Name": volume, "Labels": labels}, &ignored); err != nil {
			return &Error{Code: "speech_lab_start_failed", Err: err}
		}
	}
	return nil
}

func networkAliases(role string) []string {
	switch role {
	case "gateway":
		// The bundled web image keeps the historical `s2s:8765` upstream,
		// while AuraGo's runtime uses the explicit s2s-vulkan service name.
		return []string{"s2s", "s2s-vulkan", "gateway"}
	case "web":
		return []string{"s2s-web", "web"}
	case "asr":
		return []string{"whisper-tiny"}
	case "tts":
		return []string{"supertonic"}
	case "llm":
		return []string{"llama-fallback"}
	default:
		return nil
	}
}

func (m *Manager) matchingDeployment(ctx context.Context, op operationSnapshot, manifest BundleManifest, fingerprint string) ([]string, bool, error) {
	images := map[string]string{"gateway": manifest.Images.Gateway, "asr": manifest.Images.ASR, "tts": manifest.Images.TTS, "llm": manifest.Images.LLM, "web": manifest.Images.Web, "model_init": manifest.Images.ModelInit}
	ids := make([]string, 0, len(manifest.StartOrder))
	matched := 0
	for _, role := range manifest.StartOrder {
		var spec *BundleService
		for index := range manifest.Services {
			if manifest.Services[index].Role == role {
				spec = &manifest.Services[index]
				break
			}
		}
		if spec == nil {
			continue
		}
		matched++
		image := images[spec.Image]
		if image == "" {
			image = images[role]
		}
		name := "aurago-speech-lab-" + strings.ReplaceAll(role, "_", "-")
		container, found, err := m.inspectContainer(ctx, op, name)
		if err != nil {
			return nil, false, err
		}
		if !found || !dockerutil.ManagedBy(container.Config.Labels, OwnerLabel) ||
			container.Config.Labels["aurago.role"] != role || container.Config.Image != image ||
			container.Config.Labels["aurago.fingerprint"] != fingerprint {
			return nil, false, nil
		}
		ids = append(ids, container.ID)
	}
	return ids, matched > 0 && len(ids) == matched, nil
}

func (m *Manager) replaceServices(ctx context.Context, op operationSnapshot, manifest BundleManifest, fingerprint string) ([]string, bool, error) {
	_, state := m.snapshot()
	if state.Transaction == nil {
		return nil, false, &Error{Code: "speech_lab_state_persist_failed", Err: fmt.Errorf("deployment transaction is unavailable")}
	}
	transactionID := state.Transaction.ID
	images := map[string]string{"gateway": manifest.Images.Gateway, "asr": manifest.Images.ASR, "tts": manifest.Images.TTS, "llm": manifest.Images.LLM, "web": manifest.Images.Web, "model_init": manifest.Images.ModelInit}
	ids := make([]string, 0, len(manifest.StartOrder))
	allIdentical := true
	for _, role := range manifest.StartOrder {
		var spec *BundleService
		for i := range manifest.Services {
			if manifest.Services[i].Role == role {
				spec = &manifest.Services[i]
				break
			}
		}
		if spec == nil {
			continue
		}
		image := images[spec.Image]
		if image == "" {
			image = images[role]
		}
		if image == "" {
			return nil, false, &Error{Code: "speech_lab_bundle_incompatible", Err: fmt.Errorf("service %q has no image", role)}
		}
		name := "aurago-speech-lab-" + strings.ReplaceAll(role, "_", "-")
		existing, found, err := m.inspectContainer(ctx, op, name)
		if err != nil {
			return nil, false, err
		}
		if found {
			if !dockerutil.ManagedBy(existing.Config.Labels, OwnerLabel) || existing.Config.Labels["aurago.role"] != role {
				return nil, false, &Error{Code: "speech_lab_start_failed", Err: fmt.Errorf("container %q is not owned by AuraGo Speech Lab", name)}
			}
			if existing.Config.Image == image && existing.Config.Labels["aurago.fingerprint"] == fingerprint {
				if !existing.State.Running {
					m.mu.Lock()
					m.state.Transaction.StartedContainerIDs = append(m.state.Transaction.StartedContainerIDs, existing.ID)
					m.mu.Unlock()
					if err := m.persist(); err != nil {
						return nil, false, &Error{Code: "speech_lab_state_persist_failed", Err: err}
					}
				}
				if err := m.containerAction(ctx, op, http.MethodPost, "/containers/"+url.PathEscape(existing.ID)+"/start"); err != nil {
					return nil, false, &Error{Code: "speech_lab_start_failed", Err: err}
				}
				ids = append(ids, existing.ID)
				continue
			}
			allIdentical = false
			if err := m.backupContainer(ctx, op, manifest.Network, name, existing); err != nil {
				return nil, false, err
			}
		} else {
			allIdentical = false
		}
		hostConfig := map[string]any{"NetworkMode": manifest.Network, "RestartPolicy": map[string]any{"Name": restartPolicy(role, spec.Restart)}}
		if len(manifest.Volumes) > 0 && role != "web" {
			hostConfig["Binds"] = []string{manifest.Volumes[0] + ":/models"}
			if len(manifest.Volumes) > 1 {
				hostConfig["Binds"] = append(hostConfig["Binds"].([]string), manifest.Volumes[1]+":/data")
			}
		}
		containerPort := spec.Port
		if containerPort > 0 && !op.runningInDocker && (role == "gateway" || role == "web") {
			hostPort := containerPort
			if role == "web" {
				hostPort = 8766
			}
			hostConfig["PortBindings"] = map[string][]map[string]string{fmt.Sprintf("%d/tcp", containerPort): {{"HostIp": "127.0.0.1", "HostPort": fmt.Sprintf("%d", hostPort)}}}
		}
		aliases := networkAliases(role)
		environment := append([]string(nil), spec.Environment...)
		if role == "gateway" {
			environment = append(environment, "S2S_BUNDLE_VERSION="+manifest.BundleVersion)
		}
		labels := dockerutil.ManagedLabels(OwnerLabel, "speech-lab", role, fingerprint)
		labels["aurago.transaction"] = transactionID
		body := map[string]any{"Image": image, "Cmd": spec.Command, "Env": environment, "Labels": labels, "HostConfig": hostConfig, "NetworkingConfig": map[string]any{"EndpointsConfig": map[string]any{manifest.Network: map[string]any{"Aliases": aliases}}}}
		var created struct {
			ID string `json:"Id"`
		}
		_, err = op.docker.DoJSON(ctx, http.MethodPost, "/containers/create?name="+url.QueryEscape(name), body, &created)
		if err != nil {
			return nil, false, &Error{Code: "speech_lab_start_failed", Err: err}
		}
		if created.ID == "" {
			createdContainer, found, inspectErr := m.inspectContainer(ctx, op, name)
			if inspectErr != nil || !found || createdContainer.Config.Labels["aurago.transaction"] != transactionID {
				if inspectErr == nil {
					inspectErr = fmt.Errorf("Docker did not return an ID for service %q", role)
				}
				return nil, false, &Error{Code: "speech_lab_start_failed", Err: inspectErr}
			}
			created.ID = createdContainer.ID
		}
		if created.ID != "" {
			m.mu.Lock()
			m.state.Transaction.NewContainerIDs = append(m.state.Transaction.NewContainerIDs, created.ID)
			m.mu.Unlock()
			if err := m.persist(); err != nil {
				cleanupCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				_ = m.removeTransactionContainer(cleanupCtx, op, created.ID, transactionID)
				cancel()
				return nil, false, &Error{Code: "speech_lab_state_persist_failed", Err: err}
			}
			if err := m.containerAction(ctx, op, http.MethodPost, "/containers/"+url.PathEscape(created.ID)+"/start"); err != nil {
				return nil, false, &Error{Code: "speech_lab_start_failed", Err: err}
			}
			ids = append(ids, created.ID)
		}
	}
	if err := m.persist(); err != nil {
		return nil, false, &Error{Code: "speech_lab_state_persist_failed", Err: err}
	}
	return ids, allIdentical, nil
}

func restartPolicy(role, requested string) string {
	if role == "model_init" {
		return "no"
	}
	switch strings.ToLower(strings.TrimSpace(requested)) {
	case "no", "always", "unless-stopped", "on-failure":
		return strings.ToLower(strings.TrimSpace(requested))
	default:
		return "unless-stopped"
	}
}

func (m *Manager) containerAction(ctx context.Context, op operationSnapshot, method, path string) error {
	status, err := op.docker.DoJSON(ctx, method, path, nil, nil)
	if status == http.StatusNotModified {
		return nil
	}
	return err
}

type containerInspect struct {
	ID     string `json:"Id"`
	Name   string `json:"Name"`
	Config struct {
		Image  string            `json:"Image"`
		Labels map[string]string `json:"Labels"`
	} `json:"Config"`
	State struct {
		Running bool `json:"Running"`
	} `json:"State"`
	NetworkSettings struct {
		Networks map[string]json.RawMessage `json:"Networks"`
	} `json:"NetworkSettings"`
}

func (m *Manager) inspectContainer(ctx context.Context, op operationSnapshot, id string) (containerInspect, bool, error) {
	var out containerInspect
	status, err := op.docker.DoJSON(ctx, http.MethodGet, "/containers/"+url.PathEscape(id)+"/json", nil, &out)
	if status == http.StatusNotFound {
		return out, false, nil
	}
	if err != nil {
		return out, false, &Error{Code: "speech_lab_start_failed", Err: err}
	}
	return out, true, nil
}

func (m *Manager) networkContainsContainer(ctx context.Context, op operationSnapshot, network, containerID string) (bool, error) {
	var inspected struct {
		Containers map[string]json.RawMessage `json:"Containers"`
	}
	status, err := op.docker.DoJSON(ctx, http.MethodGet, "/networks/"+url.PathEscape(network), nil, &inspected)
	if status == http.StatusNotFound {
		return false, nil
	}
	if err != nil {
		return false, &Error{Code: "speech_lab_start_failed", Err: err}
	}
	_, attached := inspected.Containers[containerID]
	return attached, nil
}

func (m *Manager) containersRunning(ctx context.Context, op operationSnapshot, ids []string) (bool, error) {
	for _, id := range ids {
		container, found, err := m.inspectContainer(ctx, op, id)
		if err != nil {
			return false, err
		}
		if !found || !dockerutil.ManagedBy(container.Config.Labels, OwnerLabel) || !container.State.Running {
			return false, nil
		}
	}
	return len(ids) > 0, nil
}

func (m *Manager) backupContainer(ctx context.Context, op operationSnapshot, network, stableName string, container containerInspect) error {
	backupName := fmt.Sprintf("%s-rollback-%d", stableName, time.Now().UnixNano())
	record := ContainerBackup{
		ID: container.ID, StableName: stableName, BackupName: backupName, NetworkName: network,
		WasRunning: container.State.Running,
	}
	_, record.WasAttached = container.NetworkSettings.Networks[network]
	m.mu.Lock()
	m.state.Transaction.Backups = append(m.state.Transaction.Backups, record)
	m.mu.Unlock()
	if err := m.persist(); err != nil {
		return &Error{Code: "speech_lab_state_persist_failed", Err: err}
	}
	if record.WasRunning {
		if err := m.containerAction(ctx, op, http.MethodPost, "/containers/"+url.PathEscape(container.ID)+"/stop?t=10"); err != nil {
			return &Error{Code: "speech_lab_start_failed", Err: err}
		}
	}
	if record.WasAttached {
		status, err := op.docker.DoJSON(ctx, http.MethodPost, "/networks/"+url.PathEscape(network)+"/disconnect", map[string]any{"Container": container.ID, "Force": true}, nil)
		if err != nil && status != http.StatusNotFound {
			return &Error{Code: "speech_lab_start_failed", Err: err}
		}
	}
	if _, err := op.docker.DoJSON(ctx, http.MethodPost, "/containers/"+url.PathEscape(container.ID)+"/rename?name="+url.QueryEscape(backupName), nil, nil); err != nil {
		return &Error{Code: "speech_lab_start_failed", Err: err}
	}
	return nil
}

func (m *Manager) removeOwnedContainer(ctx context.Context, op operationSnapshot, id string) error {
	container, found, err := m.inspectContainer(ctx, op, id)
	if err != nil {
		return err
	}
	if !found {
		return nil
	}
	if !dockerutil.ManagedBy(container.Config.Labels, OwnerLabel) {
		return &Error{Code: "speech_lab_start_failed", Err: fmt.Errorf("refusing to remove unowned container %q", id)}
	}
	status, err := op.docker.DoJSON(ctx, http.MethodDelete, "/containers/"+url.PathEscape(id)+"?force=true", nil, nil)
	if err != nil && status != http.StatusNotFound {
		return &Error{Code: "speech_lab_start_failed", Err: err}
	}
	if _, found, err := m.inspectContainer(ctx, op, id); err != nil || found {
		if err == nil {
			err = fmt.Errorf("container %q still exists after removal", id)
		}
		return &Error{Code: "speech_lab_start_failed", Err: err}
	}
	return nil
}

func (m *Manager) removeTransactionContainer(ctx context.Context, op operationSnapshot, id, transactionID string) error {
	container, found, err := m.inspectContainer(ctx, op, id)
	if err != nil || !found {
		return err
	}
	if !dockerutil.ManagedBy(container.Config.Labels, OwnerLabel) ||
		(transactionID != "" && container.Config.Labels["aurago.transaction"] != transactionID) {
		return &Error{Code: "speech_lab_start_failed", Err: fmt.Errorf("refusing to remove container %q outside the active Speech Lab transaction", id)}
	}
	status, err := op.docker.DoJSON(ctx, http.MethodDelete, "/containers/"+url.PathEscape(id)+"?force=true", nil, nil)
	if err != nil && status != http.StatusNotFound {
		return &Error{Code: "speech_lab_start_failed", Err: err}
	}
	if _, found, err := m.inspectContainer(ctx, op, id); err != nil || found {
		if err == nil {
			err = fmt.Errorf("container %q still exists after rollback cleanup", id)
		}
		return &Error{Code: "speech_lab_start_failed", Err: err}
	}
	return nil
}

func (m *Manager) snapshot() (config.SpeechLabConfig, State) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.cfg, cloneState(m.state)
}

func (m *Manager) setTransaction(transaction *DeploymentTransaction, progress int) error {
	m.mu.Lock()
	m.state.Transaction = transaction
	m.state.Progress = progress
	m.mu.Unlock()
	if err := m.persist(); err != nil {
		m.mu.Lock()
		if m.state.Transaction == transaction {
			m.state.Transaction = nil
		}
		m.mu.Unlock()
		return &Error{Code: "speech_lab_state_persist_failed", Err: err}
	}
	return nil
}

func (m *Manager) recoverTransaction(_ context.Context, op operationSnapshot) error {
	_, state := m.snapshot()
	if state.Transaction == nil {
		return nil
	}
	if err := m.requireDockerWrite(op); err != nil {
		return err
	}
	if state.Transaction.DockerHost != "" && state.Transaction.DockerHost != op.dockerHost {
		recoveryClient := dockerutil.NewClient(state.Transaction.DockerHost, 30*time.Second)
		defer recoveryClient.CloseIdleConnections()
		op.docker = recoveryClient
		op.dockerHost = state.Transaction.DockerHost
	}
	if state.Transaction.ReadinessBaseURL != "" {
		op.readinessBaseURL = state.Transaction.ReadinessBaseURL
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	if state.Transaction.Phase == "commit_cleanup" {
		return m.commitTransaction(ctx, op)
	}
	return m.rollbackTransaction(ctx, op)
}

func (m *Manager) commitTransaction(ctx context.Context, op operationSnapshot) error {
	_, state := m.snapshot()
	if state.Transaction == nil {
		return nil
	}
	for _, backup := range state.Transaction.Backups {
		if err := m.removeOwnedContainer(ctx, op, backup.ID); err != nil {
			return err
		}
		if _, found, err := m.inspectContainer(ctx, op, backup.ID); err != nil || found {
			if err == nil {
				err = fmt.Errorf("backup container %q still exists after cleanup", backup.ID)
			}
			return err
		}
	}
	transaction := state.Transaction
	m.mu.Lock()
	m.state.Transaction = nil
	m.mu.Unlock()
	if err := m.persist(); err != nil {
		m.mu.Lock()
		if m.state.Transaction == nil {
			m.state.Transaction = transaction
		}
		m.mu.Unlock()
		return &Error{Code: "speech_lab_state_persist_failed", Err: err}
	}
	return nil
}

func (m *Manager) rollbackTransaction(ctx context.Context, op operationSnapshot) error {
	_, state := m.snapshot()
	transaction := state.Transaction
	if transaction == nil {
		return nil
	}
	m.mu.Lock()
	m.state.Transaction.Phase = "rollback_pending"
	m.mu.Unlock()
	if err := m.persist(); err != nil {
		return &Error{Code: "speech_lab_state_persist_failed", Err: err}
	}
	var rollbackErrors []error
	for _, id := range transaction.NewContainerIDs {
		if err := m.removeTransactionContainer(ctx, op, id, transaction.ID); err != nil {
			rollbackErrors = append(rollbackErrors, err)
		}
	}
	for _, id := range transaction.StartedContainerIDs {
		container, found, err := m.inspectContainer(ctx, op, id)
		if err != nil {
			rollbackErrors = append(rollbackErrors, err)
			continue
		}
		if !found {
			rollbackErrors = append(rollbackErrors, fmt.Errorf("started container %q disappeared during rollback", id))
			continue
		}
		if !dockerutil.ManagedBy(container.Config.Labels, OwnerLabel) {
			rollbackErrors = append(rollbackErrors, fmt.Errorf("refusing to stop unowned container %q during rollback", id))
			continue
		}
		if err := m.containerAction(ctx, op, http.MethodPost, "/containers/"+url.PathEscape(id)+"/stop?t=10"); err != nil {
			rollbackErrors = append(rollbackErrors, err)
			continue
		}
		if verified, found, err := m.inspectContainer(ctx, op, id); err != nil || !found || verified.State.Running {
			if err == nil {
				err = fmt.Errorf("container %q did not return to its stopped state", id)
			}
			rollbackErrors = append(rollbackErrors, err)
		}
	}
	restoredIDs := append([]string(nil), transaction.PreviousContainerIDs...)
	for index := len(transaction.Backups) - 1; index >= 0; index-- {
		backup := transaction.Backups[index]
		current, found, err := m.inspectContainer(ctx, op, backup.ID)
		if err != nil {
			rollbackErrors = append(rollbackErrors, err)
			continue
		}
		if !found || !dockerutil.ManagedBy(current.Config.Labels, OwnerLabel) {
			rollbackErrors = append(rollbackErrors, fmt.Errorf("rollback container %q is unavailable", backup.BackupName))
			continue
		}
		currentName := strings.TrimPrefix(current.Name, "/")
		if currentName != backup.StableName {
			if _, err := op.docker.DoJSON(ctx, http.MethodPost, "/containers/"+url.PathEscape(backup.ID)+"/rename?name="+url.QueryEscape(backup.StableName), nil, nil); err != nil {
				rollbackErrors = append(rollbackErrors, err)
				continue
			}
		}
		if backup.WasAttached && transaction.PreviousNetworkID != "" {
			status, err := op.docker.DoJSON(ctx, http.MethodPost, "/networks/"+url.PathEscape(transaction.PreviousNetworkID)+"/connect", map[string]any{"Container": backup.ID}, nil)
			if err != nil && status != http.StatusConflict {
				rollbackErrors = append(rollbackErrors, err)
			}
		}
		if backup.WasRunning {
			if err := m.containerAction(ctx, op, http.MethodPost, "/containers/"+url.PathEscape(backup.ID)+"/start"); err != nil {
				rollbackErrors = append(rollbackErrors, err)
			}
		}
		verified, found, err := m.inspectContainer(ctx, op, backup.ID)
		if err != nil || !found {
			if err == nil {
				err = fmt.Errorf("restored backup container %q is missing", backup.ID)
			}
			rollbackErrors = append(rollbackErrors, err)
		} else {
			_, attached := verified.NetworkSettings.Networks[backup.NetworkName]
			if strings.TrimPrefix(verified.Name, "/") != backup.StableName || verified.State.Running != backup.WasRunning || (backup.NetworkName != "" && attached != backup.WasAttached) {
				rollbackErrors = append(rollbackErrors, fmt.Errorf("backup container %q was not fully restored", backup.ID))
			}
		}
		restoredIDs = append(restoredIDs, backup.ID)
	}
	if transaction.AuraGoConnected && transaction.AuraGoContainerID != "" && transaction.AuraGoNetworkID != "" {
		status, err := op.docker.DoJSON(ctx, http.MethodPost, "/networks/"+url.PathEscape(transaction.AuraGoNetworkID)+"/disconnect", map[string]any{"Container": transaction.AuraGoContainerID, "Force": true}, nil)
		if err != nil && status != http.StatusNotFound {
			rollbackErrors = append(rollbackErrors, err)
		} else if attached, verifyErr := m.networkContainsContainer(ctx, op, transaction.AuraGoNetworkID, transaction.AuraGoContainerID); verifyErr != nil || attached {
			if verifyErr == nil {
				verifyErr = fmt.Errorf("AuraGo container is still attached after rollback")
			}
			rollbackErrors = append(rollbackErrors, verifyErr)
		}
	}
	for _, volume := range transaction.CreatedVolumes {
		if err := m.removeTransactionVolume(ctx, op, volume, transaction.ID); err != nil {
			rollbackErrors = append(rollbackErrors, err)
		}
	}
	if transaction.CreatedNetworkName != "" {
		if err := m.removeTransactionNetwork(ctx, op, transaction.CreatedNetworkName, transaction.ID); err != nil {
			rollbackErrors = append(rollbackErrors, err)
		}
	}
	restoredIDs = uniqueOwnedContainerIDs(ctx, m, op, restoredIDs, &rollbackErrors)
	if len(rollbackErrors) == 0 && transaction.PreviousState == "ready" && len(restoredIDs) > 0 {
		if err := m.waitReady(ctx, op); err != nil {
			rollbackErrors = append(rollbackErrors, fmt.Errorf("restored Speech Lab stack is not ready: %w", err))
		}
	}
	if len(rollbackErrors) > 0 {
		m.mu.Lock()
		m.state.State = "error"
		m.state.Progress = 0
		m.mu.Unlock()
		if err := m.persist(); err != nil {
			rollbackErrors = append(rollbackErrors, err)
		}
		return errors.Join(rollbackErrors...)
	}
	m.mu.Lock()
	m.state.Bundle = transaction.PreviousBundle
	m.state.Digest = transaction.PreviousDigest
	m.state.NetworkID = transaction.PreviousNetworkID
	m.state.ContainerIDs = restoredIDs
	m.state.DockerHost = transaction.PreviousDockerHost
	m.state.ReadinessBaseURL = transaction.PreviousReadinessBaseURL
	m.state.Transaction = nil
	m.state.State = transaction.PreviousState
	m.state.Progress = transaction.PreviousProgress
	if m.state.State == "" || m.state.State == "pulling" || m.state.State == "starting" {
		if len(restoredIDs) == 0 {
			m.state.State, m.state.Progress = "disabled", 0
		} else {
			m.state.State, m.state.Progress = "stopped", 0
		}
	}
	m.mu.Unlock()
	if err := m.persist(); err != nil {
		m.mu.Lock()
		m.state.Transaction = transaction
		m.state.Transaction.Phase = "rollback_pending"
		m.mu.Unlock()
		return &Error{Code: "speech_lab_state_persist_failed", Err: err}
	}
	return nil
}

func uniqueOwnedContainerIDs(ctx context.Context, m *Manager, op operationSnapshot, candidates []string, rollbackErrors *[]error) []string {
	seen := make(map[string]bool, len(candidates))
	result := make([]string, 0, len(candidates))
	for _, id := range candidates {
		id = strings.TrimSpace(id)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		container, found, err := m.inspectContainer(ctx, op, id)
		if err != nil {
			*rollbackErrors = append(*rollbackErrors, err)
			continue
		}
		if !found {
			continue
		}
		if !dockerutil.ManagedBy(container.Config.Labels, OwnerLabel) {
			*rollbackErrors = append(*rollbackErrors, fmt.Errorf("restored container %q is not owned by AuraGo Speech Lab", id))
			continue
		}
		result = append(result, id)
	}
	return result
}

func (m *Manager) removeTransactionVolume(ctx context.Context, op operationSnapshot, name, transactionID string) error {
	var inspected struct {
		Labels map[string]string `json:"Labels"`
	}
	status, err := op.docker.DoJSON(ctx, http.MethodGet, "/volumes/"+url.PathEscape(name), nil, &inspected)
	if status == http.StatusNotFound {
		return nil
	}
	if err != nil {
		return err
	}
	if !dockerutil.ManagedBy(inspected.Labels, OwnerLabel) || inspected.Labels["aurago.transaction"] != transactionID {
		return fmt.Errorf("refusing to remove volume %q outside the active Speech Lab transaction", name)
	}
	status, err = op.docker.DoJSON(ctx, http.MethodDelete, "/volumes/"+url.PathEscape(name), nil, nil)
	if err != nil && status != http.StatusNotFound {
		return err
	}
	status, err = op.docker.DoJSON(ctx, http.MethodGet, "/volumes/"+url.PathEscape(name), nil, &inspected)
	if status != http.StatusNotFound {
		if err == nil {
			err = fmt.Errorf("volume %q still exists after rollback cleanup", name)
		}
		return err
	}
	return nil
}

func (m *Manager) removeTransactionNetwork(ctx context.Context, op operationSnapshot, name, transactionID string) error {
	var inspected struct {
		Labels map[string]string `json:"Labels"`
	}
	status, err := op.docker.DoJSON(ctx, http.MethodGet, "/networks/"+url.PathEscape(name), nil, &inspected)
	if status == http.StatusNotFound {
		return nil
	}
	if err != nil {
		return err
	}
	if !dockerutil.ManagedBy(inspected.Labels, OwnerLabel) || inspected.Labels["aurago.transaction"] != transactionID {
		return fmt.Errorf("refusing to remove network %q outside the active Speech Lab transaction", name)
	}
	status, err = op.docker.DoJSON(ctx, http.MethodDelete, "/networks/"+url.PathEscape(name), nil, nil)
	if err != nil && status != http.StatusNotFound {
		return err
	}
	status, err = op.docker.DoJSON(ctx, http.MethodGet, "/networks/"+url.PathEscape(name), nil, &inspected)
	if status != http.StatusNotFound {
		if err == nil {
			err = fmt.Errorf("network %q still exists after rollback cleanup", name)
		}
		return err
	}
	return nil
}

func (m *Manager) waitReady(ctx context.Context, op operationSnapshot) error {
	base := strings.TrimRight(strings.TrimSpace(op.readinessBaseURL), "/")
	if base == "" {
		base = deploymentReadinessBase(op.cfg, op.runningInDocker)
	}
	probeCtx, cancel := context.WithTimeout(ctx, m.readinessTimeout)
	defer cancel()
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		req, _ := http.NewRequestWithContext(probeCtx, http.MethodGet, base+"/ready", nil)
		resp, err := m.httpClient.Do(req)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				m.mu.Lock()
				m.state.LastCheck = time.Now()
				m.mu.Unlock()
				return nil
			}
		}
		select {
		case <-probeCtx.Done():
			return &Error{Code: "speech_lab_not_ready", Err: fmt.Errorf("Speech Lab did not become ready")}
		case <-ticker.C:
		}
	}
}

func (m *Manager) loadState() {
	if !m.persistEnabled {
		return
	}
	data, err := os.ReadFile(m.dataPath)
	if err != nil {
		return
	}
	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		if m.logger != nil {
			m.logger.Warn("Failed to load Speech Lab deployment state", "error", err)
		}
		return
	}
	state.SchemaVersion = 2
	state.Mode = m.cfg.Deployment.Mode
	state.Managed = m.cfg.Deployment.Mode == "managed"
	if len(state.ContainerIDs) == 0 && state.NetworkID == "" && state.Digest == "" && state.Transaction == nil {
		state.Bundle = ""
	}
	m.state = state
}
func (m *Manager) persist() error {
	if !m.persistEnabled {
		return nil
	}
	m.persistMu.Lock()
	defer m.persistMu.Unlock()
	m.mu.Lock()
	state := cloneState(m.state)
	state.SchemaVersion = 2
	m.mu.Unlock()
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("encode Speech Lab deployment state: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(m.dataPath), 0o700); err != nil {
		return fmt.Errorf("create Speech Lab state directory: %w", err)
	}
	if m.stateWriter == nil {
		return fmt.Errorf("write Speech Lab deployment state: state writer is unavailable")
	}
	if err := m.stateWriter(m.dataPath, append(data, '\n'), 0o600); err != nil {
		return fmt.Errorf("write Speech Lab deployment state: %w", err)
	}
	return nil
}
