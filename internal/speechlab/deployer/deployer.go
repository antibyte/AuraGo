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
	Mode          string                 `json:"mode"`
	Managed       bool                   `json:"managed"`
	State         string                 `json:"state"`
	Bundle        string                 `json:"bundle"`
	Digest        string                 `json:"digest,omitempty"`
	NetworkID     string                 `json:"network_id,omitempty"`
	ContainerIDs  []string               `json:"container_ids,omitempty"`
	LastErrorCode string                 `json:"last_error_code,omitempty"`
	LastError     string                 `json:"last_error,omitempty"`
	Progress      int                    `json:"progress"`
	LastCheck     time.Time              `json:"last_check,omitempty"`
	Transaction   *DeploymentTransaction `json:"transaction,omitempty"`
}

// DeploymentTransaction is persisted only in deployment.json. It records
// enough information to finish cleanup or restore the last published bundle
// after AuraGo is interrupted during a managed update.
type DeploymentTransaction struct {
	Phase                string            `json:"phase"`
	PreviousBundle       string            `json:"previous_bundle,omitempty"`
	PreviousDigest       string            `json:"previous_digest,omitempty"`
	PreviousNetworkID    string            `json:"previous_network_id,omitempty"`
	PreviousContainerIDs []string          `json:"previous_container_ids,omitempty"`
	Backups              []ContainerBackup `json:"backups,omitempty"`
	NewContainerIDs      []string          `json:"new_container_ids,omitempty"`
}

type ContainerBackup struct {
	ID          string `json:"id"`
	StableName  string `json:"stable_name"`
	BackupName  string `json:"backup_name"`
	WasRunning  bool   `json:"was_running"`
	WasAttached bool   `json:"was_attached"`
}

// PublicState deliberately omits container and network identifiers from the
// HTTP status surface. Those identifiers are only needed for local lifecycle
// reconciliation and remain in the private deployment state file.
type PublicState struct {
	Mode          string    `json:"mode"`
	Managed       bool      `json:"managed"`
	State         string    `json:"state"`
	Bundle        string    `json:"bundle"`
	Digest        string    `json:"digest,omitempty"`
	LastErrorCode string    `json:"last_error_code,omitempty"`
	LastError     string    `json:"last_error,omitempty"`
	Progress      int       `json:"progress"`
	LastCheck     time.Time `json:"last_check,omitempty"`
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
	state                   State
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
		state:            State{Mode: cfg.Deployment.Mode, Managed: cfg.Deployment.Mode == "managed", State: "disabled", Bundle: cfg.Deployment.Bundle},
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
	state := m.Status()
	return PublicState{Mode: state.Mode, Managed: state.Managed, State: state.State, Bundle: state.Bundle, Digest: state.Digest, LastErrorCode: state.LastErrorCode, LastError: state.LastError, Progress: state.Progress, LastCheck: state.LastCheck}
}
func cloneState(in State) State {
	in.ContainerIDs = append([]string(nil), in.ContainerIDs...)
	if in.Transaction != nil {
		copy := *in.Transaction
		copy.PreviousContainerIDs = append([]string(nil), in.Transaction.PreviousContainerIDs...)
		copy.NewContainerIDs = append([]string(nil), in.Transaction.NewContainerIDs...)
		copy.Backups = append([]ContainerBackup(nil), in.Transaction.Backups...)
		in.Transaction = &copy
	}
	return in
}

func (m *Manager) Reconfigure(cfg config.SpeechLabConfig) {
	m.mu.Lock()
	m.cfg = cfg
	m.state.Mode = cfg.Deployment.Mode
	m.state.Managed = cfg.Deployment.Mode == "managed"
	m.state.Bundle = cfg.Deployment.Bundle
	m.mu.Unlock()
	m.persist()
}

func (m *Manager) AutoStart(ctx context.Context) error {
	cfg, state := m.snapshot()
	installed := cfg.Deployment.Mode == "managed" && len(state.ContainerIDs) > 0
	autoUpdate := installed && cfg.Deployment.AutoUpdate
	if autoUpdate {
		return m.Update(ctx)
	}
	if !installed || !cfg.Deployment.AutoStart {
		return nil
	}
	allRunning, err := m.containersRunning(ctx, state.ContainerIDs)
	if err != nil {
		return err
	}
	if allRunning {
		return nil
	}
	return m.Start(ctx)
}

func (m *Manager) Install(ctx context.Context) error { return m.install(ctx, false) }
func (m *Manager) Update(ctx context.Context) error  { return m.install(ctx, true) }

func (m *Manager) install(ctx context.Context, update bool) error {
	if err := m.begin("pulling"); err != nil {
		return err
	}
	defer m.finishOperation()
	return m.installLocked(ctx, update)
}

func (m *Manager) installLocked(ctx context.Context, update bool) (result error) {
	if err := m.recoverTransaction(ctx); err != nil {
		m.fail(err)
		return err
	}
	cfg, oldState := m.snapshot()
	if err := m.requireManaged(cfg); err != nil {
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
	if ids, identical, err := m.matchingDeployment(ctx, manifest, digest); err != nil {
		m.fail(err)
		return err
	} else if identical {
		for _, id := range ids {
			if err := m.containerAction(ctx, http.MethodPost, "/containers/"+url.PathEscape(id)+"/start"); err != nil {
				m.fail(err)
				return err
			}
		}
		if err := m.waitReady(ctx); err != nil {
			m.fail(err)
			return err
		}
		m.mu.Lock()
		m.state.Bundle, m.state.Digest, m.state.ContainerIDs = manifest.BundleVersion, digest, ids
		m.state.State, m.state.Progress, m.state.LastErrorCode, m.state.LastError = "ready", 100, "", ""
		m.mu.Unlock()
		m.persist()
		return nil
	}
	for _, image := range uniqueImages(manifest.Images) {
		if err := m.pull(ctx, image); err != nil {
			m.fail(err)
			return err
		}
	}
	if err := m.prepareResources(ctx, manifest); err != nil {
		m.fail(err)
		return err
	}
	transaction := &DeploymentTransaction{
		Phase: "replacing", PreviousBundle: oldState.Bundle, PreviousDigest: oldState.Digest,
		PreviousNetworkID: oldState.NetworkID, PreviousContainerIDs: append([]string(nil), oldState.ContainerIDs...),
	}
	m.setTransaction(transaction, 60)
	defer func() {
		if result != nil {
			rollbackErr := m.rollbackTransaction(ctx)
			m.mu.Lock()
			m.state.LastErrorCode = Code(result)
			m.state.LastError = safeError(result)
			if rollbackErr != nil {
				m.state.State = "error"
				m.state.LastError = safeError(fmt.Errorf("%v; rollback: %w", result, rollbackErr))
			}
			m.mu.Unlock()
			m.persist()
		}
	}()
	ids, identical, err := m.replaceServices(ctx, manifest, digest)
	if err != nil {
		return err
	}
	if identical {
		m.logger.Debug("Managed Speech Lab bundle is already identical", "bundle", manifest.BundleVersion)
	}
	if err := m.waitReady(ctx); err != nil {
		return err
	}
	m.mu.Lock()
	m.state.Bundle, m.state.Digest, m.state.ContainerIDs = manifest.BundleVersion, digest, append([]string(nil), ids...)
	m.state.State, m.state.Progress, m.state.LastErrorCode, m.state.LastError = "ready", 100, "", ""
	if m.state.Transaction != nil {
		m.state.Transaction.Phase = "commit_cleanup"
	}
	m.mu.Unlock()
	m.persist()
	if err := m.commitTransaction(ctx); err != nil {
		m.logger.Warn("Speech Lab update succeeded but rollback-container cleanup is pending", "error", err)
		return nil
	}
	return nil
}

func (m *Manager) Start(ctx context.Context) error {
	if err := m.begin("starting"); err != nil {
		return err
	}
	defer m.finishOperation()
	if err := m.recoverTransaction(ctx); err != nil {
		m.fail(err)
		return err
	}
	cfg, state := m.snapshot()
	if err := m.requireManaged(cfg); err != nil {
		m.fail(err)
		return err
	}
	ids := append([]string(nil), state.ContainerIDs...)
	if len(ids) == 0 {
		return m.installLocked(ctx, false)
	}
	for _, id := range ids {
		if err := m.containerAction(ctx, http.MethodPost, "/containers/"+url.PathEscape(id)+"/start"); err != nil {
			m.fail(err)
			return err
		}
	}
	if err := m.waitReady(ctx); err != nil {
		m.fail(err)
		return err
	}
	m.mu.Lock()
	m.state.State, m.state.Progress = "ready", 100
	m.mu.Unlock()
	m.persist()
	return nil
}

func (m *Manager) Stop(ctx context.Context) error {
	if err := m.begin("stopped"); err != nil {
		return err
	}
	defer m.finishOperation()
	if err := m.recoverTransaction(ctx); err != nil {
		m.fail(err)
		return err
	}
	m.mu.Lock()
	ids := append([]string(nil), m.state.ContainerIDs...)
	m.mu.Unlock()
	for _, id := range ids {
		_ = m.containerAction(ctx, http.MethodPost, "/containers/"+url.PathEscape(id)+"/stop?t=10")
	}
	m.mu.Lock()
	m.state.State, m.state.Progress = "stopped", 0
	m.mu.Unlock()
	m.persist()
	return nil
}

func (m *Manager) Remove(ctx context.Context) error {
	if err := m.begin("stopped"); err != nil {
		return err
	}
	defer m.finishOperation()
	if err := m.requireDockerWrite(); err != nil {
		m.fail(err)
		return err
	}
	if err := m.recoverTransaction(ctx); err != nil {
		m.fail(err)
		return err
	}
	m.mu.Lock()
	ids, network := append([]string(nil), m.state.ContainerIDs...), m.state.NetworkID
	m.mu.Unlock()
	for _, id := range ids {
		if err := m.removeOwnedContainer(ctx, id); err != nil {
			m.fail(err)
			return err
		}
	}
	if network != "" {
		var inspected struct {
			Labels map[string]string `json:"Labels"`
		}
		status, err := m.docker.DoJSON(ctx, http.MethodGet, "/networks/"+url.PathEscape(network), nil, &inspected)
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
			if err := m.containerAction(ctx, http.MethodDelete, "/networks/"+url.PathEscape(network)); err != nil {
				m.fail(err)
				return err
			}
		}
	}
	m.mu.Lock()
	m.state = State{Mode: m.cfg.Deployment.Mode, Managed: m.cfg.Deployment.Mode == "managed", State: "disabled", Bundle: m.cfg.Deployment.Bundle}
	m.mu.Unlock()
	m.persist()
	return nil
}

func (m *Manager) requireManaged(cfg config.SpeechLabConfig) error {
	if cfg.Deployment.Mode != "managed" {
		return &Error{Code: "speech_lab_bundle_unavailable", Err: fmt.Errorf("Speech Lab is configured as external")}
	}
	if !cfg.Enabled {
		return &Error{Code: "speech_lab_bundle_unavailable", Err: fmt.Errorf("Speech Lab is disabled")}
	}
	return m.requireDockerWrite()
}

func (m *Manager) requireDockerWrite() error {
	m.mu.Lock()
	docker, enabled, readOnly := m.docker, m.dockerEnabled, m.dockerReadOnly
	m.mu.Unlock()
	if docker == nil || !enabled || readOnly {
		return &Error{Code: "speech_lab_docker_unavailable", Err: fmt.Errorf("Docker lifecycle is disabled")}
	}
	return nil
}

func (m *Manager) begin(state string) error {
	if !m.operationMu.TryLock() {
		return &Error{Code: "speech_lab_deployment_busy", Err: fmt.Errorf("deployment operation already running")}
	}
	m.mu.Lock()
	m.state.State, m.state.LastErrorCode, m.state.LastError = state, "", ""
	m.mu.Unlock()
	return nil
}
func (m *Manager) finishOperation() { m.persist(); m.operationMu.Unlock() }
func (m *Manager) fail(err error) {
	m.mu.Lock()
	m.state.State, m.state.Progress, m.state.LastErrorCode, m.state.LastError = "error", 0, Code(err), safeError(err)
	m.mu.Unlock()
	m.persist()
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

func (m *Manager) pull(ctx context.Context, image string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, dockerutil.Endpoint("images/create?fromImage="+url.QueryEscape(image)), nil)
	if err != nil {
		return &Error{Code: "speech_lab_pull_failed", Err: err}
	}
	client := m.docker.HTTPClientWithTimeout(DockerPullTimeout)
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

func (m *Manager) prepareResources(ctx context.Context, manifest BundleManifest) error {
	var network networkCreateResponse
	_, err := m.docker.DoJSON(ctx, http.MethodPost, "/networks/create", map[string]any{"Name": manifest.Network, "Driver": "bridge", "Labels": dockerutil.ManagedLabels(OwnerLabel, "speech-lab", "network", manifest.BundleVersion)}, &network)
	if err != nil {
		// A pre-existing resource is safe only when its ownership label matches.
		var existing struct {
			ID     string            `json:"Id"`
			Labels map[string]string `json:"Labels"`
		}
		if _, inspectErr := m.docker.DoJSON(ctx, http.MethodGet, "/networks/"+url.PathEscape(manifest.Network), nil, &existing); inspectErr != nil || !dockerutil.ManagedBy(existing.Labels, OwnerLabel) {
			return &Error{Code: "speech_lab_start_failed", Err: err}
		}
		network.ID = existing.ID
	}
	m.mu.Lock()
	if network.ID != "" {
		m.state.NetworkID = network.ID
	}
	m.mu.Unlock()
	if m.runningInDocker {
		// The AuraGo container must join the private network so the managed
		// gateway hostname is reachable without publishing a host port.
		if containerID := strings.TrimSpace(os.Getenv("HOSTNAME")); containerID != "" {
			_, connectErr := m.docker.DoJSON(ctx, http.MethodPost, "/networks/"+url.PathEscape(manifest.Network)+"/connect", map[string]any{"Container": containerID}, nil)
			if connectErr != nil && !strings.Contains(strings.ToLower(connectErr.Error()), "already connected") {
				return &Error{Code: "speech_lab_start_failed", Err: connectErr}
			}
		}
	}
	for _, volume := range manifest.Volumes {
		var ignored any
		if _, err := m.docker.DoJSON(ctx, http.MethodPost, "/volumes/create", map[string]any{"Name": volume, "Labels": dockerutil.ManagedLabels(OwnerLabel, "speech-lab", "volume", manifest.BundleVersion)}, &ignored); err != nil {
			var existing struct {
				Name   string            `json:"Name"`
				Labels map[string]string `json:"Labels"`
			}
			if _, inspectErr := m.docker.DoJSON(ctx, http.MethodGet, "/volumes/"+url.PathEscape(volume), nil, &existing); inspectErr != nil || !dockerutil.ManagedBy(existing.Labels, OwnerLabel) {
				return &Error{Code: "speech_lab_start_failed", Err: err}
			}
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

func (m *Manager) matchingDeployment(ctx context.Context, manifest BundleManifest, fingerprint string) ([]string, bool, error) {
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
		container, found, err := m.inspectContainer(ctx, name)
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

func (m *Manager) replaceServices(ctx context.Context, manifest BundleManifest, fingerprint string) ([]string, bool, error) {
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
		existing, found, err := m.inspectContainer(ctx, name)
		if err != nil {
			return nil, false, err
		}
		if found {
			if !dockerutil.ManagedBy(existing.Config.Labels, OwnerLabel) || existing.Config.Labels["aurago.role"] != role {
				return nil, false, &Error{Code: "speech_lab_start_failed", Err: fmt.Errorf("container %q is not owned by AuraGo Speech Lab", name)}
			}
			if existing.Config.Image == image && existing.Config.Labels["aurago.fingerprint"] == fingerprint {
				if err := m.containerAction(ctx, http.MethodPost, "/containers/"+url.PathEscape(existing.ID)+"/start"); err != nil {
					return nil, false, &Error{Code: "speech_lab_start_failed", Err: err}
				}
				ids = append(ids, existing.ID)
				continue
			}
			allIdentical = false
			if err := m.backupContainer(ctx, manifest.Network, name, existing); err != nil {
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
		if containerPort > 0 && !m.runningInDocker && (role == "gateway" || role == "web") {
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
		body := map[string]any{"Image": image, "Cmd": spec.Command, "Env": environment, "Labels": dockerutil.ManagedLabels(OwnerLabel, "speech-lab", role, fingerprint), "HostConfig": hostConfig, "NetworkingConfig": map[string]any{"EndpointsConfig": map[string]any{manifest.Network: map[string]any{"Aliases": aliases}}}}
		var created struct {
			ID string `json:"Id"`
		}
		_, err = m.docker.DoJSON(ctx, http.MethodPost, "/containers/create?name="+url.QueryEscape(name), body, &created)
		if err != nil {
			return nil, false, &Error{Code: "speech_lab_start_failed", Err: err}
		}
		if created.ID != "" {
			m.mu.Lock()
			m.state.Transaction.NewContainerIDs = append(m.state.Transaction.NewContainerIDs, created.ID)
			m.mu.Unlock()
			m.persist()
			if err := m.containerAction(ctx, http.MethodPost, "/containers/"+url.PathEscape(created.ID)+"/start"); err != nil {
				return nil, false, &Error{Code: "speech_lab_start_failed", Err: err}
			}
			ids = append(ids, created.ID)
		}
	}
	m.persist()
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

func (m *Manager) containerAction(ctx context.Context, method, path string) error {
	status, err := m.docker.DoJSON(ctx, method, path, nil, nil)
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

func (m *Manager) inspectContainer(ctx context.Context, id string) (containerInspect, bool, error) {
	var out containerInspect
	status, err := m.docker.DoJSON(ctx, http.MethodGet, "/containers/"+url.PathEscape(id)+"/json", nil, &out)
	if status == http.StatusNotFound {
		return out, false, nil
	}
	if err != nil {
		return out, false, &Error{Code: "speech_lab_start_failed", Err: err}
	}
	return out, true, nil
}

func (m *Manager) containersRunning(ctx context.Context, ids []string) (bool, error) {
	for _, id := range ids {
		container, found, err := m.inspectContainer(ctx, id)
		if err != nil {
			return false, err
		}
		if !found || !dockerutil.ManagedBy(container.Config.Labels, OwnerLabel) || !container.State.Running {
			return false, nil
		}
	}
	return len(ids) > 0, nil
}

func (m *Manager) backupContainer(ctx context.Context, network, stableName string, container containerInspect) error {
	backupName := fmt.Sprintf("%s-rollback-%d", stableName, time.Now().UnixNano())
	record := ContainerBackup{
		ID: container.ID, StableName: stableName, BackupName: backupName,
		WasRunning: container.State.Running,
	}
	_, record.WasAttached = container.NetworkSettings.Networks[network]
	m.mu.Lock()
	m.state.Transaction.Backups = append(m.state.Transaction.Backups, record)
	m.mu.Unlock()
	m.persist()
	if record.WasRunning {
		if err := m.containerAction(ctx, http.MethodPost, "/containers/"+url.PathEscape(container.ID)+"/stop?t=10"); err != nil {
			return &Error{Code: "speech_lab_start_failed", Err: err}
		}
	}
	if record.WasAttached {
		status, err := m.docker.DoJSON(ctx, http.MethodPost, "/networks/"+url.PathEscape(network)+"/disconnect", map[string]any{"Container": container.ID, "Force": true}, nil)
		if err != nil && status != http.StatusNotFound {
			return &Error{Code: "speech_lab_start_failed", Err: err}
		}
	}
	if _, err := m.docker.DoJSON(ctx, http.MethodPost, "/containers/"+url.PathEscape(container.ID)+"/rename?name="+url.QueryEscape(backupName), nil, nil); err != nil {
		return &Error{Code: "speech_lab_start_failed", Err: err}
	}
	return nil
}

func (m *Manager) removeOwnedContainer(ctx context.Context, id string) error {
	container, found, err := m.inspectContainer(ctx, id)
	if err != nil {
		return err
	}
	if !found {
		return nil
	}
	if !dockerutil.ManagedBy(container.Config.Labels, OwnerLabel) {
		return &Error{Code: "speech_lab_start_failed", Err: fmt.Errorf("refusing to remove unowned container %q", id)}
	}
	status, err := m.docker.DoJSON(ctx, http.MethodDelete, "/containers/"+url.PathEscape(id)+"?force=true", nil, nil)
	if err != nil && status != http.StatusNotFound {
		return &Error{Code: "speech_lab_start_failed", Err: err}
	}
	return nil
}

func (m *Manager) snapshot() (config.SpeechLabConfig, State) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.cfg, cloneState(m.state)
}

func (m *Manager) setTransaction(transaction *DeploymentTransaction, progress int) {
	m.mu.Lock()
	m.state.Transaction = transaction
	m.state.Progress = progress
	m.mu.Unlock()
	m.persist()
}

func (m *Manager) recoverTransaction(ctx context.Context) error {
	_, state := m.snapshot()
	if state.Transaction == nil {
		return nil
	}
	if state.Transaction.Phase == "commit_cleanup" {
		return m.commitTransaction(ctx)
	}
	return m.rollbackTransaction(ctx)
}

func (m *Manager) commitTransaction(ctx context.Context) error {
	_, state := m.snapshot()
	if state.Transaction == nil {
		return nil
	}
	for _, backup := range state.Transaction.Backups {
		if err := m.removeOwnedContainer(ctx, backup.ID); err != nil {
			return err
		}
	}
	m.mu.Lock()
	m.state.Transaction = nil
	m.mu.Unlock()
	m.persist()
	return nil
}

func (m *Manager) rollbackTransaction(ctx context.Context) error {
	_, state := m.snapshot()
	transaction := state.Transaction
	if transaction == nil {
		return nil
	}
	var rollbackErrors []error
	for _, id := range transaction.NewContainerIDs {
		if err := m.removeOwnedContainer(ctx, id); err != nil {
			rollbackErrors = append(rollbackErrors, err)
		}
	}
	for index := len(transaction.Backups) - 1; index >= 0; index-- {
		backup := transaction.Backups[index]
		current, found, err := m.inspectContainer(ctx, backup.ID)
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
			if _, err := m.docker.DoJSON(ctx, http.MethodPost, "/containers/"+url.PathEscape(backup.ID)+"/rename?name="+url.QueryEscape(backup.StableName), nil, nil); err != nil {
				rollbackErrors = append(rollbackErrors, err)
				continue
			}
		}
		if backup.WasAttached && state.NetworkID != "" {
			status, err := m.docker.DoJSON(ctx, http.MethodPost, "/networks/"+url.PathEscape(state.NetworkID)+"/connect", map[string]any{"Container": backup.ID}, nil)
			if err != nil && status != http.StatusConflict {
				rollbackErrors = append(rollbackErrors, err)
			}
		}
		if backup.WasRunning {
			if err := m.containerAction(ctx, http.MethodPost, "/containers/"+url.PathEscape(backup.ID)+"/start"); err != nil {
				rollbackErrors = append(rollbackErrors, err)
			}
		}
	}
	m.mu.Lock()
	m.state.Bundle = transaction.PreviousBundle
	m.state.Digest = transaction.PreviousDigest
	m.state.NetworkID = transaction.PreviousNetworkID
	m.state.ContainerIDs = append([]string(nil), transaction.PreviousContainerIDs...)
	m.state.Transaction = nil
	if len(rollbackErrors) == 0 {
		m.state.State = "ready"
		m.state.Progress = 100
	}
	m.mu.Unlock()
	m.persist()
	return errors.Join(rollbackErrors...)
}

func (m *Manager) waitReady(ctx context.Context) error {
	cfg, _ := m.snapshot()
	base := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if base == "" {
		if m.runningInDocker {
			base = "http://s2s-vulkan:8765"
		} else {
			base = config.DefaultManagedSpeechLabBaseURL
		}
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
	if json.Unmarshal(data, &state) == nil && state.Managed {
		m.state = state
	}
}
func (m *Manager) persist() {
	if !m.persistEnabled {
		return
	}
	m.mu.Lock()
	state := cloneState(m.state)
	m.mu.Unlock()
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return
	}
	_ = os.MkdirAll(filepath.Dir(m.dataPath), 0o700)
	_ = config.WriteFileAtomic(m.dataPath, append(data, '\n'), 0o600)
}
