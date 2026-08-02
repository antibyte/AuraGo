package deployer

// Package deployer provisions the allow-listed Speech Lab OCI bundle. It is
// deliberately small and Docker-API based: AuraGo never executes a user
// supplied Compose file or arbitrary Docker command.

import (
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
	Mode          string    `json:"mode"`
	Managed       bool      `json:"managed"`
	State         string    `json:"state"`
	Bundle        string    `json:"bundle"`
	Digest        string    `json:"digest,omitempty"`
	NetworkID     string    `json:"network_id,omitempty"`
	ContainerIDs  []string  `json:"container_ids,omitempty"`
	LastErrorCode string    `json:"last_error_code,omitempty"`
	LastError     string    `json:"last_error,omitempty"`
	Progress      int       `json:"progress"`
	LastCheck     time.Time `json:"last_check,omitempty"`
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

type Manager struct {
	mu                      sync.Mutex
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
		state: State{Mode: cfg.Deployment.Mode, Managed: cfg.Deployment.Mode == "managed", State: "disabled", Bundle: cfg.Deployment.Bundle},
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
	m.mu.Lock()
	installed := m.cfg.Deployment.Mode == "managed" && len(m.state.ContainerIDs) > 0
	should := installed && m.cfg.Deployment.AutoStart && m.state.State != "ready"
	autoUpdate := installed && m.cfg.Deployment.AutoUpdate
	m.mu.Unlock()
	if autoUpdate {
		return m.Update(ctx)
	}
	if !should {
		return nil
	}
	return m.Start(ctx)
}

func (m *Manager) Install(ctx context.Context) error { return m.install(ctx, false) }
func (m *Manager) Update(ctx context.Context) error  { return m.install(ctx, true) }

func (m *Manager) install(ctx context.Context, update bool) (result error) {
	oldState := m.Status()
	defer func() {
		if result != nil && update {
			// Keep the last known good deployment visible after a failed update.
			// Container creation is label-scoped, so rollback never touches foreign
			// resources; a subsequent retry can reconcile the bundle explicitly.
			m.mu.Lock()
			m.state = oldState
			m.state.LastErrorCode = Code(result)
			m.state.LastError = safeError(result)
			m.mu.Unlock()
			m.persist()
		}
	}()
	if err := m.begin("pulling"); err != nil {
		return err
	}
	defer m.finish()
	if err := m.requireManaged(); err != nil {
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
	m.mu.Lock()
	m.state.Bundle, m.state.Digest, m.state.Progress = manifest.BundleVersion, digest, 75
	m.mu.Unlock()
	m.persist()
	if err := m.startServices(ctx, manifest); err != nil {
		m.fail(err)
		return err
	}
	if err := m.waitReady(ctx); err != nil {
		m.fail(err)
		return err
	}
	m.mu.Lock()
	m.state.State, m.state.Progress, m.state.LastErrorCode, m.state.LastError = "ready", 100, "", ""
	m.mu.Unlock()
	m.persist()
	return nil
}

func (m *Manager) Start(ctx context.Context) error {
	if err := m.begin("starting"); err != nil {
		return err
	}
	defer m.finish()
	if err := m.requireManaged(); err != nil {
		m.fail(err)
		return err
	}
	m.mu.Lock()
	ids := append([]string(nil), m.state.ContainerIDs...)
	m.mu.Unlock()
	if len(ids) == 0 {
		m.mu.Lock()
		m.state.State = "stopped"
		m.mu.Unlock()
		return m.install(ctx, false)
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
	defer m.finish()
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
	defer m.finish()
	m.mu.Lock()
	ids, network := append([]string(nil), m.state.ContainerIDs...), m.state.NetworkID
	m.mu.Unlock()
	for _, id := range ids {
		_ = m.containerAction(ctx, http.MethodDelete, "/containers/"+url.PathEscape(id)+"?force=true")
	}
	if network != "" {
		_ = m.containerAction(ctx, http.MethodDelete, "/networks/"+url.PathEscape(network))
	}
	m.mu.Lock()
	m.state = State{Mode: m.cfg.Deployment.Mode, Managed: true, State: "disabled", Bundle: m.cfg.Deployment.Bundle}
	m.mu.Unlock()
	m.persist()
	return nil
}

func (m *Manager) requireManaged() error {
	if m.cfg.Deployment.Mode != "managed" {
		return &Error{Code: "speech_lab_bundle_unavailable", Err: fmt.Errorf("Speech Lab is configured as external")}
	}
	if !m.cfg.Enabled {
		return &Error{Code: "speech_lab_bundle_unavailable", Err: fmt.Errorf("Speech Lab is disabled")}
	}
	if m.docker == nil || !m.dockerEnabled || m.dockerReadOnly {
		return &Error{Code: "speech_lab_docker_unavailable", Err: fmt.Errorf("Docker lifecycle is disabled")}
	}
	return nil
}

func (m *Manager) begin(state string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.state.State == "pulling" || m.state.State == "starting" {
		return &Error{Code: "speech_lab_deployment_busy", Err: fmt.Errorf("deployment operation already running")}
	}
	m.state.State, m.state.LastErrorCode, m.state.LastError = state, "", ""
	return nil
}
func (m *Manager) finish() { m.persist() }
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
	resp, err := m.docker.HTTPClient().Do(req)
	if err != nil {
		return &Error{Code: "speech_lab_pull_failed", Err: err}
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &Error{Code: "speech_lab_pull_failed", Err: fmt.Errorf("Docker pull returned HTTP %d", resp.StatusCode)}
	}
	if _, err := io.Copy(io.Discard, io.LimitReader(resp.Body, DockerPullMaxBytes)); err != nil {
		return &Error{Code: "speech_lab_pull_failed", Err: err}
	}
	return nil
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
	m.state.ContainerIDs = nil
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

func (m *Manager) startServices(ctx context.Context, manifest BundleManifest) error {
	images := map[string]string{"gateway": manifest.Images.Gateway, "asr": manifest.Images.ASR, "tts": manifest.Images.TTS, "llm": manifest.Images.LLM, "web": manifest.Images.Web, "model_init": manifest.Images.ModelInit}
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
			return &Error{Code: "speech_lab_bundle_incompatible", Err: fmt.Errorf("service %q has no image", role)}
		}
		name := "aurago-speech-lab-" + strings.ReplaceAll(role, "_", "-")
		hostConfig := map[string]any{"NetworkMode": manifest.Network}
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
		aliases := []string{}
		if role == "gateway" {
			aliases = []string{"s2s-vulkan", "gateway"}
		}
		if role == "web" {
			aliases = []string{"s2s-web", "web"}
		}
		environment := append([]string(nil), spec.Environment...)
		if role == "gateway" {
			environment = append(environment, "S2S_BUNDLE_VERSION="+manifest.BundleVersion)
		}
		body := map[string]any{"Image": image, "Cmd": spec.Command, "Env": environment, "Labels": dockerutil.ManagedLabels(OwnerLabel, "speech-lab", role, manifest.BundleVersion), "HostConfig": hostConfig, "NetworkingConfig": map[string]any{"EndpointsConfig": map[string]any{manifest.Network: map[string]any{"Aliases": aliases}}}}
		var created struct {
			ID string `json:"Id"`
		}
		_, err := m.docker.DoJSON(ctx, http.MethodPost, "/containers/create?name="+url.QueryEscape(name), body, &created)
		if err != nil {
			var existing struct {
				ID     string `json:"Id"`
				Config struct {
					Labels map[string]string `json:"Labels"`
				} `json:"Config"`
			}
			if _, inspectErr := m.docker.DoJSON(ctx, http.MethodGet, "/containers/"+url.PathEscape(name)+"/json", nil, &existing); inspectErr != nil || !dockerutil.ManagedBy(existing.Config.Labels, OwnerLabel) {
				return &Error{Code: "speech_lab_start_failed", Err: err}
			}
			created.ID = existing.ID
		}
		if created.ID != "" {
			m.mu.Lock()
			m.state.ContainerIDs = append(m.state.ContainerIDs, created.ID)
			m.mu.Unlock()
			if err := m.containerAction(ctx, http.MethodPost, "/containers/"+url.PathEscape(created.ID)+"/start"); err != nil {
				return &Error{Code: "speech_lab_start_failed", Err: err}
			}
		}
	}
	m.persist()
	return nil
}

func (m *Manager) containerAction(ctx context.Context, method, path string) error {
	_, err := m.docker.DoJSON(ctx, method, path, nil, nil)
	return err
}

func (m *Manager) waitReady(ctx context.Context) error {
	base := strings.TrimRight(strings.TrimSpace(m.cfg.BaseURL), "/")
	if base == "" {
		if m.runningInDocker {
			base = "http://s2s-vulkan:8765"
		} else {
			base = config.DefaultManagedSpeechLabBaseURL
		}
	}
	probeCtx, cancel := context.WithTimeout(ctx, DefaultReadinessTimeout)
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
