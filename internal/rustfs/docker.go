package rustfs

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"io"

	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"

	"aurago/internal/config"
	"aurago/internal/dockerutil"
)

type containerInfo struct {
	ID     string `json:"Id"`
	Name   string
	Image  string
	Config struct {
		Image, User string
		Labels      map[string]string
	}
	State      struct{ Running bool }
	HostConfig struct {
		NetworkMode string
		Privileged  bool
	}
	Mounts []struct {
		Type, Name, Source, Destination string
		RW                              bool
	}
	NetworkSettings struct {
		Ports map[string][]struct {
			HostIP   string `json:"HostIp"`
			HostPort string
		}
		Networks map[string]struct{ NetworkID string }
	}
}
type volumeInfo struct {
	Mountpoint string
	Name       string
	CreatedAt  string
	Labels     map[string]string
	Options    map[string]string
}
type placement struct{ engine, network, endpoint string }

func (m *Manager) labels(role string, s persistedState) map[string]string {
	labels := dockerutil.ManagedLabels(dockerutil.RustFSOwner, "rustfs", role, "")
	labels["aurago.instance"] = s.Instance
	return labels
}
func owns(labels map[string]string, s persistedState, role string) bool {
	return s.Instance != "" && dockerutil.ManagedBy(labels, dockerutil.RustFSOwner) && labels["aurago.instance"] == s.Instance && labels["aurago.role"] == role
}
func (m *Manager) inspectContainer(ctx context.Context, s persistedState) (containerInfo, bool, error) {
	var info containerInfo
	code, err := m.docker.DoJSON(ctx, http.MethodGet, "containers/"+url.PathEscape(m.names.container)+"/json", nil, &info)
	if code == http.StatusNotFound && s.ContainerID != "" {
		code, err = m.docker.DoJSON(ctx, http.MethodGet, "containers/"+url.PathEscape(s.ContainerID)+"/json", nil, &info)
	}
	if code == http.StatusNotFound {
		return info, false, nil
	}
	if err != nil {
		return info, false, failure("docker_unavailable", err)
	}
	if info.ID == "" || !owns(info.Config.Labels, s, "runtime") || s.ContainerID != "" && info.ID != s.ContainerID {
		return info, true, failure("foreign_resource", nil)
	}
	return info, true, nil
}
func (m *Manager) validateRuntimeSpec(info containerInfo, currentImage bool) error {
	if currentImage && info.Config.Image != Image || info.Config.User != "10001:10001" || info.HostConfig.Privileged || info.HostConfig.NetworkMode == "host" || strings.HasPrefix(info.HostConfig.NetworkMode, "container:") {
		return failure("runtime_spec_changed", nil)
	}
	data, credentials := false, false
	for _, mount := range info.Mounts {
		switch mount.Destination {
		case "/data":
			data = mount.Type == "volume" && mount.Name == m.names.data && mount.RW
		case "/run/aurago-rustfs":
			credentials = mount.Type == "volume" && mount.Name == m.names.credentials && !mount.RW
		default:
			if mount.Type == "bind" || mount.Type == "volume" {
				return failure("runtime_spec_changed", nil)
			}
		}
	}
	if !data || !credentials {
		return failure("runtime_spec_changed", nil)
	}
	return nil
}
func (m *Manager) observedEndpoint(cfg *config.Config, info containerInfo, p placement) (string, error) {
	if info.HostConfig.NetworkMode != p.network {
		return "", failure("unsupported_placement", nil)
	}
	if cfg.Runtime.IsDocker {
		if _, ok := info.NetworkSettings.Networks[p.network]; !ok || len(info.NetworkSettings.Ports["9000/tcp"]) != 0 {
			return "", failure("unsupported_placement", nil)
		}
		return p.endpoint, nil
	}
	bindings := info.NetworkSettings.Ports["9000/tcp"]
	if len(bindings) != 1 || bindings[0].HostIP != "127.0.0.1" || bindings[0].HostPort == "" {
		return "", failure("unsupported_placement", nil)
	}
	return "http://127.0.0.1:" + bindings[0].HostPort, nil
}
func (m *Manager) inspectVolume(ctx context.Context, name, role, created string, s persistedState) (volumeInfo, bool, error) {
	var volume volumeInfo
	code, err := m.docker.DoJSON(ctx, http.MethodGet, "volumes/"+url.PathEscape(name), nil, &volume)
	if code == http.StatusNotFound {
		return volume, false, nil
	}
	if err != nil {
		return volume, false, failure("docker_unavailable", err)
	}
	if volume.Name != name || !owns(volume.Labels, s, role) || created != "" && created != volume.CreatedAt || len(volume.Options) > 0 {
		return volume, true, failure("foreign_resource", nil)
	}
	dockerutil.RegisterRustFSVolumePath(volume.Mountpoint)
	return volume, true, nil
}
func (m *Manager) provePlacement(ctx context.Context, cfg *config.Config, s persistedState) (placement, error) {
	var info struct{ ID, OSType string }
	if _, err := m.docker.DoJSON(ctx, http.MethodGet, "info", nil, &info); err != nil {
		return placement{}, failure("docker_unavailable", err)
	}
	if info.ID == "" || info.OSType != "linux" {
		return placement{}, failure("unsupported_placement", nil)
	}
	if s.EngineID != "" && s.EngineID != info.ID {
		return placement{}, failure("recovery_required", nil)
	}
	if dockerutil.NormalizeHost(cfg.Docker.Host) != m.docker.Host() {
		return placement{}, failure("unsupported_placement", nil)
	}
	p := placement{engine: info.ID, network: "bridge"}
	if !cfg.Runtime.IsDocker {
		host := m.docker.Host()
		// A loopback TCP listener can tunnel another host's Engine; its published ports are not proven local.
		if !strings.HasPrefix(host, "unix://") && !strings.HasPrefix(host, "npipe://") {
			return p, failure("unsupported_placement", nil)
		}
		return p, nil
	}
	self := dockerutil.OwnContainerID(os.ReadFile)
	if self == "" {
		hostname, _ := os.Hostname()
		self = dockerutil.DefaultContainerHostname(hostname)
	}
	if self == "" {
		return p, failure("unsupported_placement", nil)
	}
	var own containerInfo
	if _, err := m.docker.DoJSON(ctx, http.MethodGet, "containers/"+url.PathEscape(self)+"/json", nil, &own); err != nil || !strings.HasPrefix(own.ID, self) || own.HostConfig.NetworkMode == "host" || strings.HasPrefix(own.HostConfig.NetworkMode, "container:") {
		return p, failure("unsupported_placement", nil)
	}
	var containers []struct {
		ID         string `json:"Id"`
		HostConfig struct{ NetworkMode string }
	}
	if _, err := m.docker.DoJSON(ctx, http.MethodGet, "containers/json?all=1", nil, &containers); err != nil {
		return p, failure("unsupported_placement", nil)
	}
	for _, c := range containers {
		ref := strings.TrimPrefix(c.HostConfig.NetworkMode, "container:")
		if ref != c.HostConfig.NetworkMode && (ref == own.ID || len(ref) >= 12 && strings.HasPrefix(own.ID, ref)) {
			return p, failure("unsupported_placement", nil)
		}
	}
	type candidate struct {
		name     string
		internal bool
	}
	var networks []candidate
	for name, attached := range own.NetworkSettings.Networks {
		if name == "bridge" || strings.Contains(strings.ReplaceAll(name, "_", "-"), "docker-control") {
			continue
		}
		var network struct {
			ID, Driver string
			Internal   bool
		}
		if _, err := m.docker.DoJSON(ctx, http.MethodGet, "networks/"+url.PathEscape(name), nil, &network); err == nil && network.Driver == "bridge" && network.ID != "" && network.ID == attached.NetworkID {
			networks = append(networks, candidate{name, network.Internal})
		}
	}
	sort.Slice(networks, func(i, j int) bool {
		if networks[i].internal != networks[j].internal {
			return networks[i].internal
		}
		return networks[i].name < networks[j].name
	})
	if len(networks) == 0 {
		return p, failure("unsupported_placement", nil)
	}
	p.network = networks[0].name
	p.endpoint = "http://" + m.names.container + ":9000"
	return p, nil
}
func (m *Manager) Preflight(ctx context.Context) (Preflight, error) {
	m.mu.Lock()
	cfg := copyConfig(m.cfg)
	s := m.state
	stateErr := m.stateErr
	generation := m.generation
	m.mu.Unlock()
	p := Preflight{DataVolume: m.names.data, CredentialVolume: m.names.credentials, MutationAllowed: cfg.Docker.Enabled && !cfg.Docker.ReadOnly}
	fail := func(err error) (Preflight, error) {
		p.Reason = ErrorCode(err)
		m.mu.Lock()
		if m.generation == generation && ctx.Err() == nil {
			m.status.State = p.Reason
			m.status.Reason = p.Reason
			m.status.Ready = false
		}
		m.mu.Unlock()
		return p, err
	}
	if !cfg.Docker.Enabled {
		return fail(failure("docker_disabled", nil))
	}
	if stateErr != nil {
		return fail(stateErr)
	}
	place, err := m.provePlacement(ctx, cfg, s)
	if err != nil {
		return fail(err)
	}
	p.DockerAvailable = true
	p.Supported = true
	p.Placement = place.network
	info, found, err := m.inspectContainer(ctx, s)
	if err != nil {
		return fail(err)
	}
	p.Installed = found
	if found {
		m.mu.Lock()
		if m.generation == generation {
			m.status.Installed = true
			m.status.Running = info.State.Running
		}
		m.mu.Unlock()
		if err = m.validateRuntimeSpec(info, true); err != nil {
			return fail(err)
		}
		if info.State.Running {
			endpoint, e := m.observedEndpoint(cfg, info, place)
			if e != nil {
				return fail(e)
			}
			if s.Endpoint != endpoint {
				return fail(failure("recovery_required", nil))
			}
		}
	}
	_, data, err := m.inspectVolume(ctx, m.names.data, "data", s.DataCreated, s)
	if err != nil {
		return fail(err)
	}
	p.DataRetained = data
	_, _, err = m.inspectVolume(ctx, m.names.credentials, "credentials", s.CredentialsCreated, s)
	if err != nil {
		return fail(err)
	}
	p.Endpoint = s.Endpoint
	ready := false
	if found && info.State.Running && s.Endpoint != "" {
		ready = m.ready(ctx, s.Endpoint) == nil
		if ready && s.Provisioned {
			if err := m.inspectEstablishedIAM(ctx, cfg, s); err != nil {
				return fail(err)
			}
		}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if generation != m.generation || configFingerprint(cfg) != m.fingerprint || ctx.Err() != nil {
		return p, failure("stale_target", nil)
	}
	m.status.Installed = found
	m.status.Running = found && info.State.Running
	m.status.Provisioned = s.Provisioned
	m.status.ContainerID = info.ID
	m.status.Endpoint = s.Endpoint
	m.status.Ready = ready && s.Provisioned
	m.status.State = "not_installed"
	if found {
		m.status.State = "stopped"
	}
	if m.status.Running {
		m.status.State = "starting"
	}
	if m.status.Ready {
		m.status.State = "ready"
	}
	m.status.Reason = ""
	m.status.AccessState = "disabled"
	if hasDataPermissions(effectivePermissions(cfg)) {
		m.status.AccessState = "pending"
		if m.status.Ready && !m.blocked && s.PolicyHash == policyHash(generatePolicy(effectivePermissions(cfg))) {
			m.status.AccessState = "ready"
		}
	}
	return p, nil
}
func (m *Manager) ready(ctx context.Context, endpoint string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint+"/health/ready", nil)
	if err != nil {
		return failure("invalid_endpoint", nil)
	}
	response, err := m.http.Do(req)
	if err != nil {
		return failure("runtime_unavailable", nil)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return failure("runtime_unavailable", nil)
	}
	var result struct {
		Ready bool `json:"ready"`
	}
	if json.NewDecoder(io.LimitReader(response.Body, 64<<10)).Decode(&result) != nil || !result.Ready {
		return failure("runtime_unavailable", nil)
	}
	return nil
}
func (m *Manager) waitReady(ctx context.Context, endpoint string) error {
	ctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for {
		if m.ready(ctx, endpoint) == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return failure("runtime_unavailable", ctx.Err())
		case <-ticker.C:
		}
	}
}
func (m *Manager) ensureImage(ctx context.Context) error {
	var image struct {
		ID string `json:"Id"`
	}
	code, err := m.docker.DoJSON(ctx, http.MethodGet, "images/"+url.PathEscape(Image)+"/json", nil, &image)
	if err == nil && image.ID != "" {
		return nil
	}
	if code != http.StatusNotFound {
		return failure("image_unavailable", err)
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, dockerutil.Endpoint("images/create?fromImage="+url.QueryEscape(Image)), nil)
	client := *m.docker.HTTPClient()
	client.Timeout = 10 * time.Minute
	response, err := client.Do(req)
	if err != nil {
		return failure("image_pull_failed", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return failure("image_pull_failed", nil)
	}
	if err = dockerutil.DrainJSONMessages(response.Body); err != nil {
		return failure("image_pull_failed", nil)
	}
	return nil
}
func (m *Manager) ensureVolume(ctx context.Context, name, role, created string, s persistedState) (string, error) {
	volume, found, err := m.inspectVolume(ctx, name, role, created, s)
	if err != nil {
		return "", err
	}
	if !found && (created != "" || s.Provisioned) {
		return "", failure("recovery_required", nil)
	}
	if !found {
		code, err := m.docker.DoJSON(ctx, http.MethodPost, "volumes/create", map[string]any{"Name": name, "Labels": m.labels(role, s)}, nil)
		if err != nil {
			return "", mutationError(code, err)
		}
		volume, _, err = m.inspectVolume(ctx, name, role, "", s)
		if err != nil {
			return "", err
		}
	}
	if volume.CreatedAt == "" {
		return "", failure("recovery_required", nil)
	}
	return volume.CreatedAt, nil
}
func mutationError(code int, err error) error {
	if code == 0 || code >= 200 && code < 300 {
		return failure("outcome_unknown", err)
	}
	return failure("docker_rejected", err)
}
func (m *Manager) mutate(ctx context.Context, method, path string, body, result any) error {
	code, err := m.docker.DoJSON(ctx, method, path, body, result)
	if err != nil {
		return mutationError(code, err)
	}
	return nil
}
func (m *Manager) seedCredentials(ctx context.Context, s persistedState) error {
	var seed containerInfo
	code, err := m.docker.DoJSON(ctx, http.MethodGet, "containers/"+url.PathEscape(m.names.seed)+"/json", nil, &seed)
	if code != http.StatusNotFound && err != nil {
		return failure("docker_unavailable", err)
	}
	if code != http.StatusNotFound && (!owns(seed.Config.Labels, s, "credential-seed") || seed.State.Running || seed.ID == "" || seed.Config.Image != Image || seed.Config.User != "10001:10001" || seed.HostConfig.NetworkMode != "none" || seed.HostConfig.Privileged || !m.validSeedMounts(seed)) {
		return failure("foreign_resource", nil)
	}
	if code == http.StatusNotFound {
		spec := map[string]any{"Image": Image, "User": "10001:10001", "Labels": m.labels("credential-seed", s), "HostConfig": map[string]any{
			"ReadonlyRootfs": true, "NetworkMode": "none", "CapDrop": []string{"ALL"}, "SecurityOpt": []string{"no-new-privileges:true"}, "PidsLimit": 16,
			"Mounts": []map[string]any{{"Type": "volume", "Source": m.names.credentials, "Target": "/run/aurago-rustfs"}, {"Type": "volume", "Source": m.names.data, "Target": "/data", "ReadOnly": true}}, "LogConfig": map[string]any{"Type": "none"}}}
		if err = m.mutate(ctx, http.MethodPost, "containers/create?name="+url.QueryEscape(m.names.seed), spec, &seed); err != nil {
			return err
		}
		if seed.ID == "" {
			return failure("outcome_unknown", nil)
		}
	}
	var archive bytes.Buffer
	writer := tar.NewWriter(&archive)
	for name, value := range map[string]string{"access-key": s.RootAccess, "secret-key": s.RootSecret} {
		payload := []byte(value + "\n")
		if err = writer.WriteHeader(&tar.Header{Name: name, Mode: 0400, Uid: 10001, Gid: 10001, Size: int64(len(payload))}); err != nil {
			return failure("credential_seed_failed", nil)
		}
		if _, err = writer.Write(payload); err != nil {
			return failure("credential_seed_failed", nil)
		}
	}
	if err = writer.Close(); err != nil {
		return failure("credential_seed_failed", nil)
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodPut, dockerutil.Endpoint("containers/"+url.PathEscape(seed.ID)+"/archive?path=/run/aurago-rustfs&copyUIDGID=true"), bytes.NewReader(archive.Bytes()))
	req.Header.Set("Content-Type", "application/x-tar")
	response, err := m.docker.HTTPClient().Do(req)
	if err != nil {
		return failure("outcome_unknown", nil)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return failure("credential_seed_failed", nil)
	}
	if err = m.mutate(ctx, http.MethodDelete, "containers/"+url.PathEscape(seed.ID), nil, nil); err != nil {
		return err
	}
	return nil
}
func (m *Manager) runtimeSpec(cfg *config.Config, s persistedState, p placement) map[string]any {
	host := map[string]any{
		"ReadonlyRootfs": true, "NetworkMode": p.network, "CapDrop": []string{"ALL"}, "SecurityOpt": []string{"no-new-privileges:true"}, "PidsLimit": 512, "Memory": int64(2 << 30), "NanoCpus": int64(2e9),
		"Tmpfs":         map[string]string{"/tmp": "rw,noexec,nosuid,nodev,size=64m", "/logs": "rw,noexec,nosuid,nodev,size=16m"},
		"RestartPolicy": map[string]any{"Name": "unless-stopped"},
		"LogConfig":     map[string]any{"Type": "json-file", "Config": map[string]string{"max-size": "10m", "max-file": "3"}},
		"Mounts":        []map[string]any{{"Type": "volume", "Source": m.names.data, "Target": "/data"}, {"Type": "volume", "Source": m.names.credentials, "Target": "/run/aurago-rustfs", "ReadOnly": true}},
	}
	if !cfg.Runtime.IsDocker {
		port := s.Port
		if port == "" {
			port = "0"
		}
		host["PortBindings"] = map[string]any{"9000/tcp": []map[string]string{{"HostIp": "127.0.0.1", "HostPort": port}}}
	}
	spec := map[string]any{"Image": Image, "User": "10001:10001", "Labels": m.labels("runtime", s), "ExposedPorts": map[string]any{"9000/tcp": map[string]any{}}, "HostConfig": host,
		"Env": []string{"RUSTFS_ACCESS_KEY_FILE=/run/aurago-rustfs/access-key", "RUSTFS_SECRET_KEY_FILE=/run/aurago-rustfs/secret-key", "RUSTFS_ADDRESS=:9000", "RUSTFS_CONSOLE_ENABLE=false", "RUSTFS_OBS_LOGGER_LEVEL=error", "RUSTFS_OBS_LOG_DIRECTORY=", "RUSTFS_OBS_USE_STDOUT=true"},
	}
	if cfg.Runtime.IsDocker {
		spec["NetworkingConfig"] = map[string]any{"EndpointsConfig": map[string]any{p.network: map[string]any{"Aliases": []string{m.names.container}}}}
	}
	return spec
}
func (m *Manager) ensureRuntime(ctx context.Context, cfg *config.Config, j *job) error {
	m.mu.Lock()
	s := m.state
	stateErr := m.stateErr
	m.mu.Unlock()
	if stateErr != nil {
		return stateErr
	}
	p, err := m.provePlacement(ctx, cfg, s)
	if err != nil {
		return err
	}
	info, found, err := m.inspectContainer(ctx, s)
	if err != nil {
		return err
	}
	if s.Instance == "" {
		for _, name := range []string{m.names.data, m.names.credentials} {
			var v any
			code, readErr := m.docker.DoJSON(ctx, http.MethodGet, "volumes/"+url.PathEscape(name), nil, &v)
			if code != http.StatusNotFound {
				return failure("recovery_required", readErr)
			}
		}
		s = newState()
		s.InitialBucket = cfg.RustFS.Bucket
		s.EngineID = p.engine
		if err = m.saveState(s); err != nil {
			return err
		}
	}
	if s.EngineID == "" {
		s.EngineID = p.engine
		if err = m.saveState(s); err != nil {
			return err
		}
	}
	if found {
		if err = m.validateRuntimeSpec(info, j.Action != ActionRecreate); err != nil {
			return err
		}
	}
	if s.Provisioned && found && info.State.Running {
		endpoint, e := m.observedEndpoint(cfg, info, p)
		if e != nil {
			return e
		}
		if endpoint != s.Endpoint {
			return failure("recovery_required", nil)
		}
		if err = m.inspectEstablishedIAM(ctx, cfg, s); err != nil {
			return err
		}
	}
	m.phase(j, "image_pull")
	if err = m.ensureImage(ctx); err != nil {
		return err
	}
	m.phase(j, "credentials")
	if s.DataCreated, err = m.ensureVolume(ctx, m.names.data, "data", s.DataCreated, s); err != nil {
		return err
	}
	if s.CredentialsCreated, err = m.ensureVolume(ctx, m.names.credentials, "credentials", s.CredentialsCreated, s); err != nil {
		return err
	}
	if err = m.saveState(s); err != nil {
		return err
	}
	if !s.Seeded {
		if err = m.seedCredentials(ctx, s); err != nil {
			return err
		}
		s.Seeded = true
		if err = m.saveState(s); err != nil {
			return err
		}
	}
	if j.Action == ActionRecreate && found {
		if err = m.stopRuntime(ctx, s, false); err != nil {
			return err
		}
		if err = m.mutate(ctx, http.MethodDelete, "containers/"+url.PathEscape(info.ID), nil, nil); err != nil {
			return err
		}
		s.ContainerID = ""
		found = false
		if err = m.saveState(s); err != nil {
			return err
		}
	}
	if !found {
		m.phase(j, "creating")
		var created struct {
			ID string `json:"Id"`
		}
		if err = m.mutate(ctx, http.MethodPost, "containers/create?name="+url.QueryEscape(m.names.container), m.runtimeSpec(cfg, s, p), &created); err != nil {
			return err
		}
		if created.ID == "" {
			return failure("outcome_unknown", nil)
		}
		s.ContainerID = created.ID
		if err = m.saveState(s); err != nil {
			return err
		}
		info, found, err = m.inspectContainer(ctx, s)
		if err != nil || !found {
			return failure("outcome_unknown", err)
		}
	}
	m.phase(j, "starting")
	if !info.State.Running {
		if err = m.mutate(ctx, http.MethodPost, "containers/"+url.PathEscape(info.ID)+"/start", nil, nil); err != nil {
			return err
		}
	}
	info, found, err = m.inspectContainer(ctx, s)
	if err != nil || !found || !info.State.Running {
		return failure("outcome_unknown", err)
	}
	s.DesiredRunning = true
	if cfg.Runtime.IsDocker {
		s.Endpoint = p.endpoint
	} else {
		bindings := info.NetworkSettings.Ports["9000/tcp"]
		if len(bindings) != 1 || bindings[0].HostIP != "127.0.0.1" || bindings[0].HostPort == "" {
			return failure("unsupported_placement", nil)
		}
		s.Port = bindings[0].HostPort
		s.Endpoint = "http://127.0.0.1:" + s.Port
	}
	if err = m.saveState(s); err != nil {
		return err
	}
	if err = m.waitReady(ctx, s.Endpoint); err != nil {
		return err
	}
	if err = m.checkJob(j, cfg); err != nil {
		return err
	}
	m.phase(j, "iam")
	if err = m.reconcileIAM(ctx, cfg, &s); err != nil {
		return err
	}
	if err = m.saveState(s); err != nil {
		return err
	}
	if err = m.checkJob(j, cfg); err != nil {
		return err
	}
	m.phase(j, "verifying")
	if err = m.publishReady(j, cfg); err != nil {
		return err
	}
	_, err = m.Preflight(ctx)
	return err
}
func (m *Manager) stopRuntime(ctx context.Context, s persistedState, recordIntent bool) error {
	if s.Instance == "" {
		return nil
	}
	m.mu.Lock()
	cfg := copyConfig(m.cfg)
	m.mu.Unlock()
	if _, err := m.provePlacement(ctx, cfg, s); err != nil {
		return err
	}
	info, found, err := m.inspectContainer(ctx, s)
	if err != nil {
		return err
	}
	if found && info.State.Running {
		_, stopErr := m.docker.DoJSON(ctx, http.MethodPost, "containers/"+url.PathEscape(info.ID)+"/stop?t=15", nil, nil)
		observed, exists, readErr := m.inspectContainer(ctx, s)
		if readErr != nil || exists && observed.State.Running {
			if stopErr != nil {
				return failure("outcome_unknown", stopErr)
			}
			return failure("stop_unconfirmed", readErr)
		}
	}
	if recordIntent {
		s.DesiredRunning = false
		if err = m.saveState(s); err != nil {
			return err
		}
	}
	m.mu.Lock()
	m.status.Running = false
	m.status.Ready = false
	m.status.State = "stopped"
	m.status.AccessState = "disabled"
	m.mu.Unlock()
	return nil
}
func (m *Manager) removeRuntime(ctx context.Context, s persistedState) error {
	info, found, err := m.inspectContainer(ctx, s)
	if err != nil {
		return err
	}
	if found {
		if err = m.mutate(ctx, http.MethodDelete, "containers/"+url.PathEscape(info.ID), nil, nil); err != nil {
			return err
		}
	}
	s.ContainerID = ""
	s.DesiredRunning = false
	if err = m.saveState(s); err != nil {
		return err
	}
	m.mu.Lock()
	m.status.Installed = false
	m.status.State = "not_installed"
	m.mu.Unlock()
	return nil
}

func (m *Manager) validSeedMounts(seed containerInfo) bool {
	credentials, data := false, false
	for _, mount := range seed.Mounts {
		switch mount.Destination {
		case "/run/aurago-rustfs":
			credentials = mount.Type == "volume" && mount.Name == m.names.credentials && mount.RW
		case "/data":
			data = mount.Type == "volume" && mount.Name == m.names.data && !mount.RW
		default:
			if mount.Type == "bind" || mount.Type == "volume" {
				return false
			}
		}
	}
	return credentials && data
}
