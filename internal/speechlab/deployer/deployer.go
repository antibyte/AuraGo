package deployer

// Package deployer provisions the allow-listed Speech Lab OCI bundle. It is
// deliberately small and Docker-API based: AuraGo never executes a user
// supplied Compose file or arbitrary Docker command.

import (
	"bufio"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
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
	"runtime"
	"strconv"
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

// speechLabManifestPublicKeyB64 pins the official Speech Lab release signer.
// It may be overridden at release build time with
// -X aurago/internal/speechlab/deployer.speechLabManifestPublicKeyB64=...
// The environment override is intended for development and private releases.
var speechLabManifestPublicKeyB64 = "zNBMSpDXLlcTLrkRYAkIt095X5FN2SsxGI8zWPbOxZI="

type ImageSet struct {
	Gateway       string `json:"gateway"`
	Controller    string `json:"controller"`
	ASR           string `json:"asr,omitempty"`
	TTS           string `json:"tts,omitempty"`
	LLM           string `json:"llm,omitempty"`
	Whisper       string `json:"whisper,omitempty"`
	ParakeetCPU   string `json:"parakeet_cpu,omitempty"`
	ParakeetCUDA  string `json:"parakeet_cuda,omitempty"`
	VoxtralCPU    string `json:"voxtral_cpu,omitempty"`
	VoxtralCUDA   string `json:"voxtral_cuda,omitempty"`
	QwenSYCLAOT   string `json:"qwen_sycl_aot,omitempty"`
	QwenSYCLJIT   string `json:"qwen_sycl_jit,omitempty"`
	QwenVulkan    string `json:"qwen_vulkan,omitempty"`
	QwenCUDA      string `json:"qwen_cuda,omitempty"`
	Kokoro        string `json:"kokoro,omitempty"`
	VibeVoiceCPU  string `json:"vibevoice_cpu,omitempty"`
	VibeVoiceCUDA string `json:"vibevoice_cuda,omitempty"`
	HiggsCUDA     string `json:"higgs_cuda,omitempty"`
	LlamaCPU      string `json:"llama_cpu,omitempty"`
	LlamaCUDA     string `json:"llama_cuda,omitempty"`
	Web           string `json:"web"`
	ModelInit     string `json:"model_init"`
}

type BundleService struct {
	Role         string   `json:"role"`
	Image        string   `json:"image"`
	Command      []string `json:"command,omitempty"`
	Environment  []string `json:"environment,omitempty"`
	HealthPath   string   `json:"health_path,omitempty"`
	Port         int      `json:"port,omitempty"`
	Restart      string   `json:"restart,omitempty"`
	InternalOnly bool     `json:"internal_only,omitempty"`
	DockerSocket bool     `json:"docker_socket,omitempty"`
}

type BundleRuntime struct {
	BackendID              string            `json:"backend_id"`
	VariantID              string            `json:"variant_id"`
	Stage                  string            `json:"stage"`
	Container              string            `json:"container"`
	Image                  string            `json:"image"`
	ImageKey               string            `json:"image_key,omitempty"`
	ImageDownloadSizeBytes int64             `json:"image_download_size_bytes"`
	Architectures          []string          `json:"architectures"`
	Accelerator            string            `json:"accelerator"`
	Experimental           bool              `json:"experimental,omitempty"`
	Command                []string          `json:"command,omitempty"`
	Environment            map[string]string `json:"environment,omitempty"`
	Aliases                []string          `json:"aliases,omitempty"`
	ModelMounts            []string          `json:"model_mounts,omitempty"`
	Healthcheck            map[string]any    `json:"healthcheck"`
	Volumes                []map[string]any  `json:"volumes"`
	Network                map[string]any    `json:"network"`
	Resources              map[string]any    `json:"resources"`
	License                string            `json:"license,omitempty"`
	AuthRequired           bool              `json:"auth_required,omitempty"`
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
	Runtimes        []BundleRuntime     `json:"runtimes,omitempty"`
}

type State struct {
	SchemaVersion int    `json:"schema_version,omitempty"`
	Mode          string `json:"mode"`
	Managed       bool   `json:"managed"`
	State         string `json:"state"`
	Bundle        string `json:"bundle"`
	Digest        string `json:"digest,omitempty"`
	// GPUBackend is the effective profile of the published containers. It may
	// be CPU when the requested Auto profile could not receive GPU passthrough.
	GPUBackend                string                 `json:"gpu_backend,omitempty"`
	NetworkID                 string                 `json:"network_id,omitempty"`
	ContainerIDs              []string               `json:"container_ids,omitempty"`
	ModuleContainerIDs        []string               `json:"module_container_ids,omitempty"`
	RunningModuleContainerIDs []string               `json:"running_module_container_ids,omitempty"`
	DockerHost                string                 `json:"docker_host,omitempty"`
	ReadinessBaseURL          string                 `json:"readiness_base_url,omitempty"`
	LastErrorCode             string                 `json:"last_error_code,omitempty"`
	LastError                 string                 `json:"last_error,omitempty"`
	Progress                  int                    `json:"progress"`
	LastCheck                 time.Time              `json:"last_check,omitempty"`
	Transaction               *DeploymentTransaction `json:"transaction,omitempty"`
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
	PreviousGPUBackend       string            `json:"previous_gpu_backend,omitempty"`
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
	Mode                string    `json:"mode"`
	Managed             bool      `json:"managed"`
	State               string    `json:"state"`
	Bundle              string    `json:"bundle"`
	RequestedBundle     string    `json:"requested_bundle"`
	RequestedGPUBackend string    `json:"requested_gpu_backend"`
	ActiveGPUBackend    string    `json:"active_gpu_backend"`
	Digest              string    `json:"digest,omitempty"`
	LastErrorCode       string    `json:"last_error_code,omitempty"`
	LastError           string    `json:"last_error,omitempty"`
	Progress            int       `json:"progress"`
	LastCheck           time.Time `json:"last_check,omitempty"`
	RecoveryPending     bool      `json:"recovery_pending"`
	CleanupPending      bool      `json:"cleanup_pending"`
	CleanupAvailable    bool      `json:"cleanup_available"`
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

func WithManifestPublicKey(publicKey ed25519.PublicKey) Option {
	return func(m *Manager) {
		m.manifestPublicKey = append(ed25519.PublicKey(nil), publicKey...)
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
	manifestPublicKey       ed25519.PublicKey
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
	encodedPublicKey := strings.TrimSpace(os.Getenv("AURAGO_SPEECH_LAB_MANIFEST_PUBLIC_KEY_B64"))
	if encodedPublicKey == "" {
		encodedPublicKey = strings.TrimSpace(speechLabManifestPublicKeyB64)
	}
	if encoded := encodedPublicKey; encoded != "" {
		if decoded, err := base64.StdEncoding.DecodeString(encoded); err == nil && len(decoded) == ed25519.PublicKeySize {
			m.manifestPublicKey = ed25519.PublicKey(decoded)
		}
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
		Bundle: state.Bundle, RequestedBundle: cfg.Deployment.Bundle,
		RequestedGPUBackend: config.NormalizeSpeechLabGPUBackend(cfg.Deployment.GPUBackend),
		ActiveGPUBackend: func() string {
			if state.Managed {
				return state.GPUBackend
			}
			return ""
		}(),
		Digest:        state.Digest,
		LastErrorCode: state.LastErrorCode, LastError: state.LastError, Progress: state.Progress, LastCheck: state.LastCheck,
		RecoveryPending:  state.Transaction != nil && phase != "commit_cleanup",
		CleanupPending:   state.Transaction != nil && phase == "commit_cleanup",
		CleanupAvailable: len(state.ContainerIDs) > 0 || state.NetworkID != "" || state.Transaction != nil,
	}
}
func cloneState(in State) State {
	in.ContainerIDs = append([]string(nil), in.ContainerIDs...)
	in.ModuleContainerIDs = append([]string(nil), in.ModuleContainerIDs...)
	in.RunningModuleContainerIDs = append([]string(nil), in.RunningModuleContainerIDs...)
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
	gpuBackend := config.NormalizeSpeechLabGPUBackend(op.cfg.Deployment.GPUBackend)
	gpuHostConfig, err := speechLabGPUHostConfig(gpuBackend)
	if err != nil {
		m.fail(err)
		return err
	}
	sum := sha256.Sum256(raw)
	digest := hex.EncodeToString(sum[:])
	fingerprint := speechLabDeploymentFingerprint(digest, gpuBackend)
	if update {
		m.logger.Info("Updating managed Speech Lab bundle", "bundle", manifest.BundleVersion, "gpu_backend", gpuBackend)
	}
	if ids, identical, err := m.matchingDeployment(ctx, op, manifest, fingerprint); err != nil {
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
		m.state.Bundle, m.state.Digest, m.state.GPUBackend, m.state.ContainerIDs, m.state.DockerHost = manifest.BundleVersion, digest, speechLabActiveGPUBackend(gpuBackend, gpuHostConfig), ids, op.dockerHost
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
		PreviousBundle: oldState.Bundle, PreviousDigest: oldState.Digest, PreviousGPUBackend: oldState.GPUBackend,
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
	for _, image := range serviceImages(manifest) {
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
	ids, identical, err := m.replaceServices(ctx, op, manifest, fingerprint, gpuHostConfig)
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
	m.state.Bundle, m.state.Digest, m.state.GPUBackend, m.state.ContainerIDs, m.state.DockerHost = manifest.BundleVersion, digest, speechLabActiveGPUBackend(gpuBackend, gpuHostConfig), append([]string(nil), ids...), op.dockerHost
	m.state.ModuleContainerIDs = nil
	m.state.RunningModuleContainerIDs = nil
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
	runningModuleIDs := append([]string(nil), state.RunningModuleContainerIDs...)
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
	for _, id := range runningModuleIDs {
		container, found, inspectErr := m.inspectContainer(ctx, installedOp, id)
		if inspectErr != nil {
			m.fail(inspectErr)
			return inspectErr
		}
		if !found {
			continue
		}
		if validationErr := validateManagedModule(container.Config.Image, container.Config.Labels, state.Bundle); validationErr != nil {
			wrapped := &Error{Code: "speech_lab_start_failed", Err: fmt.Errorf("refusing to start module container %q: %w", id, validationErr)}
			m.fail(wrapped)
			return wrapped
		}
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
	modules, err := m.listOwnedModuleContainers(ctx, installedOp)
	if err != nil {
		m.fail(err)
		return err
	}
	runningModules := make([]string, 0, len(modules))
	for _, module := range modules {
		ids = append(ids, module.ID)
		if strings.EqualFold(module.State, "running") {
			runningModules = append(runningModules, module.ID)
		}
	}
	m.mu.Lock()
	m.state.ModuleContainerIDs = make([]string, 0, len(modules))
	for _, module := range modules {
		m.state.ModuleContainerIDs = append(m.state.ModuleContainerIDs, module.ID)
	}
	m.state.RunningModuleContainerIDs = runningModules
	m.mu.Unlock()
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
	modules, listErr := m.listOwnedModuleContainers(ctx, installedOp)
	if listErr != nil {
		m.fail(listErr)
		return listErr
	}
	for _, module := range modules {
		ids = append(ids, module.ID)
	}
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
	if out.SchemaVersion == 2 {
		if len(m.manifestPublicKey) != ed25519.PublicKeySize {
			return out, nil, &Error{Code: "speech_lab_bundle_signature_invalid", Err: fmt.Errorf("Speech Lab bundle public key is not configured")}
		}
		signatureReq, signatureErr := http.NewRequestWithContext(ctx, http.MethodGet, m.manifestURL+".sig", nil)
		if signatureErr != nil {
			return out, nil, &Error{Code: "speech_lab_bundle_unavailable", Err: signatureErr}
		}
		signatureResp, signatureErr := m.httpClient.Do(signatureReq)
		if signatureErr != nil {
			return out, nil, &Error{Code: "speech_lab_bundle_unavailable", Err: signatureErr}
		}
		signature, readErr := io.ReadAll(io.LimitReader(signatureResp.Body, ed25519.SignatureSize+1))
		signatureResp.Body.Close()
		if signatureResp.StatusCode != http.StatusOK || readErr != nil || len(signature) != ed25519.SignatureSize || !ed25519.Verify(m.manifestPublicKey, raw, signature) {
			return out, nil, &Error{Code: "speech_lab_bundle_signature_invalid", Err: fmt.Errorf("Speech Lab bundle signature verification failed")}
		}
	}
	return out, raw, nil
}

func validateManifest(m BundleManifest) error {
	if !((m.SchemaVersion == 1 && strings.TrimSpace(m.ContractVersion) == "speech-lab/v1") ||
		(m.SchemaVersion == 2 && strings.TrimSpace(m.ContractVersion) == "speech-lab/v2")) {
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
	seenServices := make(map[string]struct{}, len(m.Services))
	for _, service := range m.Services {
		if strings.TrimSpace(service.Role) == "" {
			return fmt.Errorf("bundle service role is empty")
		}
		if _, exists := seenServices[service.Role]; exists {
			return fmt.Errorf("duplicate bundle service %q", service.Role)
		}
		seenServices[service.Role] = struct{}{}
		if image := imageByKey(m.Images, service.Image); image == "" {
			return fmt.Errorf("bundle service %q has an unknown image key %q", service.Role, service.Image)
		}
	}
	if m.SchemaVersion == 2 {
		if len(m.Runtimes) == 0 || strings.TrimSpace(m.Images.Controller) == "" {
			return fmt.Errorf("bundle v2 has no controller or module runtimes")
		}
		if len(m.Volumes) < 3 {
			return fmt.Errorf("bundle v2 has no dedicated control volume")
		}
		controllerServices := 0
		controlInitServices := 0
		seenVariants := make(map[string]struct{}, len(m.Runtimes))
		for _, service := range m.Services {
			if service.DockerSocket && service.Role != "controller" {
				return fmt.Errorf("service %q must not receive the Docker socket", service.Role)
			}
			if service.Role == "controller" {
				controllerServices++
				if !service.InternalOnly || !service.DockerSocket {
					return fmt.Errorf("controller must be internal-only and own the Docker socket")
				}
			}
			if service.Role == "control_init" {
				controlInitServices++
				if !service.InternalOnly || service.DockerSocket || service.Image != "controller" {
					return fmt.Errorf("control_init must be internal-only, socket-free, and use the controller image")
				}
			}
		}
		if controllerServices != 1 {
			return fmt.Errorf("bundle v2 must define exactly one controller")
		}
		if controlInitServices != 1 || roleIndex(m.StartOrder, "control_init") < 0 || roleIndex(m.StartOrder, "control_init") > roleIndex(m.StartOrder, "controller") {
			return fmt.Errorf("bundle v2 must initialize the read-only controller token before controller startup")
		}
		for _, module := range m.Runtimes {
			if strings.TrimSpace(module.BackendID) == "" || strings.TrimSpace(module.VariantID) == "" || strings.TrimSpace(module.Container) == "" {
				return fmt.Errorf("module runtime identity is incomplete")
			}
			if _, exists := seenVariants[module.VariantID]; exists {
				return fmt.Errorf("duplicate module runtime %q", module.VariantID)
			}
			seenVariants[module.VariantID] = struct{}{}
			if module.Stage != "asr" && module.Stage != "tts" && module.Stage != "llm" {
				return fmt.Errorf("module runtime %q has invalid stage", module.VariantID)
			}
			if err := validateImage(module.Image); err != nil {
				return fmt.Errorf("module runtime %q: %w", module.VariantID, err)
			}
			if expected := imageByKey(m.Images, module.ImageKey); expected == "" || expected != module.Image {
				return fmt.Errorf("module runtime %q image key does not match its digest", module.VariantID)
			}
			architectureOK := false
			for _, architecture := range module.Architectures {
				if architecture == runtime.GOARCH {
					architectureOK = true
				}
			}
			if !architectureOK {
				return fmt.Errorf("module runtime %q is not published for %s", module.VariantID, runtime.GOARCH)
			}
			if module.ImageDownloadSizeBytes <= 0 || len(module.Healthcheck) == 0 || len(module.Volumes) == 0 || len(module.Network) == 0 || len(module.Resources) == 0 {
				return fmt.Errorf("module runtime %q has incomplete v2 metadata", module.VariantID)
			}
		}
	}
	return nil
}

func roleIndex(roles []string, target string) int {
	for index, role := range roles {
		if role == target {
			return index
		}
	}
	return -1
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
	out := make([]string, 0, 20)
	for _, image := range imageValues(images) {
		if image != "" && !seen[image] {
			seen[image] = true
			out = append(out, image)
		}
	}
	return out
}

func serviceImages(manifest BundleManifest) []string {
	seen := map[string]struct{}{}
	result := make([]string, 0, len(manifest.Services))
	for _, service := range manifest.Services {
		image := imageByKey(manifest.Images, service.Image)
		if image == "" {
			image = imageByKey(manifest.Images, service.Role)
		}
		if image == "" {
			continue
		}
		if _, exists := seen[image]; exists {
			continue
		}
		seen[image] = struct{}{}
		result = append(result, image)
	}
	return result
}

func imageValues(images ImageSet) []string {
	return []string{
		images.Gateway, images.Controller, images.ASR, images.TTS, images.LLM,
		images.Whisper, images.ParakeetCPU, images.ParakeetCUDA, images.VoxtralCPU,
		images.VoxtralCUDA, images.QwenSYCLAOT, images.QwenSYCLJIT, images.QwenVulkan,
		images.QwenCUDA, images.Kokoro, images.VibeVoiceCPU, images.VibeVoiceCUDA,
		images.HiggsCUDA, images.LlamaCPU, images.LlamaCUDA, images.Web, images.ModelInit,
	}
}

func imageByKey(images ImageSet, key string) string {
	switch key {
	case "gateway":
		return images.Gateway
	case "tts":
		if images.TTS != "" {
			return images.TTS
		}
		return images.Gateway
	case "controller":
		return images.Controller
	case "asr":
		if images.ASR != "" {
			return images.ASR
		}
		return images.Whisper
	case "llm":
		if images.LLM != "" {
			return images.LLM
		}
		return images.LlamaCPU
	case "whisper":
		return images.Whisper
	case "parakeet_cpu":
		return images.ParakeetCPU
	case "parakeet_cuda":
		return images.ParakeetCUDA
	case "voxtral_cpu":
		return images.VoxtralCPU
	case "voxtral_cuda":
		return images.VoxtralCUDA
	case "qwen_sycl_aot":
		return images.QwenSYCLAOT
	case "qwen_sycl_jit":
		return images.QwenSYCLJIT
	case "qwen_vulkan":
		return images.QwenVulkan
	case "qwen_cuda":
		return images.QwenCUDA
	case "kokoro":
		return images.Kokoro
	case "vibevoice_cpu":
		return images.VibeVoiceCPU
	case "vibevoice_cuda":
		return images.VibeVoiceCUDA
	case "higgs_cuda":
		return images.HiggsCUDA
	case "llama_cpu":
		return images.LlamaCPU
	case "llama_cuda":
		return images.LlamaCUDA
	case "web":
		return images.Web
	case "model_init":
		return images.ModelInit
	default:
		return ""
	}
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
	case "controller":
		return []string{"speech-lab-controller", "controller"}
	default:
		return nil
	}
}

func controllerPolicy(manifest BundleManifest) (string, error) {
	policy := struct {
		BundleVersion string          `json:"bundle_version"`
		Runtimes      []BundleRuntime `json:"runtimes"`
	}{BundleVersion: manifest.BundleVersion, Runtimes: manifest.Runtimes}
	raw, err := json.Marshal(policy)
	if err != nil {
		return "", fmt.Errorf("encode controller policy: %w", err)
	}
	return base64.StdEncoding.EncodeToString(raw), nil
}

const speechLabGPUGroupIDsEnv = "AURAGO_GPU_GROUP_IDS"

func speechLabDeploymentFingerprint(manifestDigest, gpuBackend string) string {
	input := manifestDigest + "\x00" + config.NormalizeSpeechLabGPUBackend(gpuBackend)
	sum := sha256.Sum256([]byte(input))
	return hex.EncodeToString(sum[:])
}

func speechLabActiveGPUBackend(gpuBackend string, hostConfig map[string]any) string {
	switch gpuBackend = config.NormalizeSpeechLabGPUBackend(gpuBackend); gpuBackend {
	case config.SpeechLabGPUBackendCPU:
		return config.SpeechLabGPUBackendCPU
	case config.SpeechLabGPUBackendVulkan:
		return config.SpeechLabGPUBackendVulkan
	default:
		if hostConfig == nil {
			return config.SpeechLabGPUBackendCPU
		}
		return config.SpeechLabGPUBackendAuto
	}
}

func overlaySpeechLabGPUEnvironment(environment []string, gpuBackend string) ([]string, error) {
	gpuBackend = config.NormalizeSpeechLabGPUBackend(gpuBackend)
	if err := config.ValidateSpeechLabGPUBackend(gpuBackend); err != nil {
		return nil, err
	}
	result := make([]string, 0, len(environment)+2)
	for _, entry := range environment {
		key := strings.TrimSpace(strings.SplitN(entry, "=", 2)[0])
		if key == "S2S_GPU" || key == "GGML_BACKEND" {
			continue
		}
		result = append(result, entry)
	}
	result = append(result, "S2S_GPU="+gpuBackend)
	if gpuBackend == config.SpeechLabGPUBackendCPU {
		result = append(result, "GGML_BACKEND=CPU")
	}
	return result, nil
}

func speechLabGPUHostConfig(gpuBackend string) (map[string]any, error) {
	gpuBackend = config.NormalizeSpeechLabGPUBackend(gpuBackend)
	if err := config.ValidateSpeechLabGPUBackend(gpuBackend); err != nil {
		return nil, &Error{Code: "speech_lab_bundle_incompatible", Err: err}
	}
	if gpuBackend == config.SpeechLabGPUBackendCPU {
		return nil, nil
	}
	if runtime.GOOS != "linux" {
		if gpuBackend == config.SpeechLabGPUBackendVulkan {
			return nil, &Error{Code: "speech_lab_gpu_unavailable", Err: fmt.Errorf("Vulkan passthrough requires a Linux /dev/dri device in the managed container runtime")}
		}
		return nil, nil
	}
	devices, err := filepath.Glob("/dev/dri/renderD*")
	if err != nil {
		return nil, &Error{Code: "speech_lab_gpu_unavailable", Err: fmt.Errorf("inspect Vulkan render devices: %w", err)}
	}
	// In a container AuraGo cannot see the host's /dev/dri tree. The
	// installer/Compose contract forwards the host render/video GIDs through
	// AURAGO_GPU_GROUP_IDS; treat that explicit, validated hint as sufficient
	// to ask Docker to bind the host device path. Without either signal Auto
	// stays a safe CPU fallback and explicit Vulkan gets an actionable error.
	hostGroupHint := strings.TrimSpace(os.Getenv(speechLabGPUGroupIDsEnv)) != ""
	if len(devices) == 0 && !hostGroupHint {
		if gpuBackend == config.SpeechLabGPUBackendVulkan {
			return nil, &Error{Code: "speech_lab_gpu_unavailable", Err: fmt.Errorf("no accessible Vulkan render device was found under /dev/dri")}
		}
		return nil, nil
	}
	groupIDs, err := speechLabGPUGroupIDs()
	if err != nil {
		return nil, &Error{Code: "speech_lab_gpu_unavailable", Err: err}
	}
	return speechLabVulkanHostConfig(groupIDs), nil
}

func speechLabVulkanHostConfig(groupIDs []string) map[string]any {
	hostConfig := map[string]any{
		"Devices": []map[string]string{{
			"PathOnHost":        "/dev/dri",
			"PathInContainer":   "/dev/dri",
			"CgroupPermissions": "rwm",
		}},
	}
	if len(groupIDs) > 0 {
		hostConfig["GroupAdd"] = append([]string(nil), groupIDs...)
	}
	return hostConfig
}

func speechLabGPUGroupIDs() ([]string, error) {
	if configured := strings.TrimSpace(os.Getenv(speechLabGPUGroupIDsEnv)); configured != "" {
		return normalizeSpeechLabGPUGroupIDs([]string{configured})
	}
	if runtime.GOOS != "linux" {
		return nil, nil
	}
	if _, err := os.Stat("/.dockerenv"); err == nil {
		// The group database in the AuraGo image describes the container, not
		// the Docker host. Host IDs must be supplied explicitly by the
		// installer or Compose environment.
		return nil, nil
	}
	data, err := os.ReadFile("/etc/group")
	if err != nil {
		return nil, nil
	}
	var ids []string
	for _, line := range strings.Split(string(data), "\n") {
		parts := strings.SplitN(line, ":", 4)
		if len(parts) < 3 || (parts[0] != "render" && parts[0] != "video") {
			continue
		}
		ids = append(ids, parts[2])
	}
	return normalizeSpeechLabGPUGroupIDs(ids)
}

func normalizeSpeechLabGPUGroupIDs(values []string) ([]string, error) {
	const maxGroups = 16
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, raw := range values {
		for _, value := range strings.Fields(strings.ReplaceAll(raw, ",", " ")) {
			id, err := strconv.ParseUint(value, 10, 31)
			if err != nil || id == 0 {
				return nil, fmt.Errorf("invalid GPU group ID %q", value)
			}
			canonical := strconv.FormatUint(id, 10)
			if _, exists := seen[canonical]; exists {
				continue
			}
			if len(result) == maxGroups {
				return nil, fmt.Errorf("too many GPU group IDs")
			}
			seen[canonical] = struct{}{}
			result = append(result, canonical)
		}
	}
	return result, nil
}

func speechLabGPUServiceRole(role string) bool {
	switch role {
	case "gateway", "asr", "llm", "tts":
		return true
	default:
		return false
	}
}

func (m *Manager) matchingDeployment(ctx context.Context, op operationSnapshot, manifest BundleManifest, fingerprint string) ([]string, bool, error) {
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
		image := imageByKey(manifest.Images, spec.Image)
		if image == "" {
			image = imageByKey(manifest.Images, role)
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

func (m *Manager) replaceServices(ctx context.Context, op operationSnapshot, manifest BundleManifest, fingerprint string, gpuHostConfig map[string]any) ([]string, bool, error) {
	_, state := m.snapshot()
	if state.Transaction == nil {
		return nil, false, &Error{Code: "speech_lab_state_persist_failed", Err: fmt.Errorf("deployment transaction is unavailable")}
	}
	transactionID := state.Transaction.ID
	ids := make([]string, 0, len(manifest.StartOrder))
	allIdentical := true
	modules, err := m.listOwnedModuleContainers(ctx, op)
	if err != nil {
		return nil, false, err
	}
	for _, module := range modules {
		inspected, found, inspectErr := m.inspectContainer(ctx, op, module.ID)
		if inspectErr != nil {
			return nil, false, inspectErr
		}
		if !found {
			continue
		}
		name := strings.TrimPrefix(inspected.Name, "/")
		if name == "" {
			return nil, false, &Error{Code: "speech_lab_start_failed", Err: fmt.Errorf("managed module %q has no container name", module.ID)}
		}
		allIdentical = false
		if err := m.backupContainer(ctx, op, manifest.Network, name, inspected); err != nil {
			return nil, false, err
		}
	}
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
		image := imageByKey(manifest.Images, spec.Image)
		if image == "" {
			image = imageByKey(manifest.Images, role)
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
		if speechLabGPUServiceRole(role) {
			for key, value := range gpuHostConfig {
				hostConfig[key] = value
			}
		}
		if len(manifest.Volumes) > 0 && role != "web" && role != "controller" && role != "control_init" {
			hostConfig["Binds"] = []string{manifest.Volumes[0] + ":/models"}
			if len(manifest.Volumes) > 1 {
				hostConfig["Binds"] = append(hostConfig["Binds"].([]string), manifest.Volumes[1]+":/data")
			}
		}
		if manifest.SchemaVersion == 2 && role == "controller" {
			if len(manifest.Volumes) < 3 {
				return nil, false, &Error{Code: "speech_lab_bundle_incompatible", Err: fmt.Errorf("bundle v2 control volume is missing")}
			}
			hostConfig["Binds"] = []string{
				"/var/run/docker.sock:/var/run/docker.sock:ro",
				manifest.Volumes[2] + ":/control:ro",
			}
			hostConfig["ReadonlyRootfs"] = true
			hostConfig["CapDrop"] = []string{"ALL"}
			hostConfig["SecurityOpt"] = []string{"no-new-privileges:true"}
		}
		if manifest.SchemaVersion == 2 && role == "control_init" {
			if len(manifest.Volumes) < 3 {
				return nil, false, &Error{Code: "speech_lab_bundle_incompatible", Err: fmt.Errorf("bundle v2 control volume is missing")}
			}
			hostConfig["Binds"] = []string{manifest.Volumes[2] + ":/control"}
			hostConfig["ReadonlyRootfs"] = true
			hostConfig["CapDrop"] = []string{"ALL"}
			hostConfig["SecurityOpt"] = []string{"no-new-privileges:true"}
		}
		if manifest.SchemaVersion == 2 && role == "gateway" {
			if len(manifest.Volumes) < 3 {
				return nil, false, &Error{Code: "speech_lab_bundle_incompatible", Err: fmt.Errorf("bundle v2 control volume is missing")}
			}
			binds, _ := hostConfig["Binds"].([]string)
			hostConfig["Binds"] = append(binds, manifest.Volumes[2]+":/control:ro")
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
		environment, err := overlaySpeechLabGPUEnvironment(spec.Environment, op.cfg.Deployment.GPUBackend)
		if err != nil {
			return nil, false, &Error{Code: "speech_lab_bundle_incompatible", Err: err}
		}
		if role == "gateway" {
			environment = append(environment, "S2S_BUNDLE_VERSION="+manifest.BundleVersion)
			if manifest.SchemaVersion == 2 {
				environment = append(environment,
					"S2S_DOCKER_PROXY_URL=http://speech-lab-controller:2375",
					"S2S_DOCKER_PROXY_TOKEN_FILE=/control/token",
				)
			}
		}
		if role == "controller" {
			policy, policyErr := controllerPolicy(manifest)
			if policyErr != nil {
				return nil, false, &Error{Code: "speech_lab_bundle_incompatible", Err: policyErr}
			}
			environment = append(environment,
				"S2S_CONTROLLER_POLICY_B64="+policy,
				"S2S_CONTROLLER_TOKEN_FILE=/control/token",
				"S2S_CONTROLLER_TOKEN_READ_ONLY=true",
				"S2S_CONTROLLER_NETWORK="+manifest.Network,
				"S2S_CONTROLLER_MODELS_VOLUME="+manifest.Volumes[0],
				"S2S_CONTROLLER_DATA_VOLUME="+manifest.Volumes[1],
				"S2S_CONTROLLER_BUNDLE_DIGEST="+fingerprint,
			)
		}
		labels := dockerutil.ManagedLabels(OwnerLabel, "speech-lab", role, fingerprint)
		labels["aurago.bundle"] = manifest.BundleVersion
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
	if role == "model_init" || role == "control_init" {
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

type moduleContainer struct {
	ID     string            `json:"Id"`
	Names  []string          `json:"Names"`
	Image  string            `json:"Image"`
	State  string            `json:"State"`
	Labels map[string]string `json:"Labels"`
}

func (m *Manager) listOwnedModuleContainers(ctx context.Context, op operationSnapshot) ([]moduleContainer, error) {
	m.mu.Lock()
	bundle := m.state.Bundle
	m.mu.Unlock()
	var containers []moduleContainer
	status, err := op.docker.DoJSON(ctx, http.MethodGet, "/containers/json?all=1", nil, &containers)
	if status == http.StatusNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, &Error{Code: "speech_lab_start_failed", Err: err}
	}
	result := make([]moduleContainer, 0)
	for _, container := range containers {
		if !dockerutil.ManagedBy(container.Labels, OwnerLabel) || container.Labels["aurago.role"] != "module" {
			continue
		}
		if err := validateManagedModule(container.Image, container.Labels, bundle); err != nil {
			return nil, &Error{Code: "speech_lab_start_failed", Err: fmt.Errorf("refusing malformed managed module %q: %w", container.ID, err)}
		}
		result = append(result, container)
	}
	return result, nil
}

func validateManagedModule(image string, labels map[string]string, bundle string) error {
	if labels["aurago.component"] != "speech-lab" || labels["s2s.lab.managed"] != "true" {
		return fmt.Errorf("ownership labels do not match")
	}
	if bundle == "" || labels["aurago.bundle"] != bundle {
		return fmt.Errorf("bundle label does not match the installed deployment")
	}
	if labels["backend-id"] == "" || labels["variant-id"] == "" {
		return fmt.Errorf("backend or variant label is missing")
	}
	if labels["stage"] != "asr" && labels["stage"] != "tts" && labels["stage"] != "llm" {
		return fmt.Errorf("stage label is invalid")
	}
	if labels["s2s.image"] != image {
		return fmt.Errorf("image label does not match the container image")
	}
	if err := validateImage(image); err != nil {
		return err
	}
	return nil
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
	m.state.GPUBackend = transaction.PreviousGPUBackend
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
