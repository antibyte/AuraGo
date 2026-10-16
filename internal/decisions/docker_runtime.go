package decisions

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"aurago/internal/config"
	"aurago/internal/dockerutil"
)

const decisionsDockerPort = 18765
const decisionsDockerCleanupTimeout = 5 * time.Second

type decisionsDocker struct {
	client *dockerutil.Client
	host   string
}

type dockerImageMetadata struct {
	ID           string `json:"Id"`
	OS           string `json:"Os"`
	Architecture string `json:"Architecture"`
	Config       struct {
		Labels map[string]string `json:"Labels"`
	} `json:"Config"`
}

type dockerContainerMetadata struct {
	ID    string `json:"Id"`
	Image string `json:"Image"`
	State struct {
		ExitCode int  `json:"ExitCode"`
		Running  bool `json:"Running"`
	} `json:"State"`
	Mounts []struct {
		Type        string `json:"Type"`
		Name        string `json:"Name"`
		Source      string `json:"Source"`
		Destination string `json:"Destination"`
	} `json:"Mounts"`
}

func newDecisionsDocker(host string) *decisionsDocker {
	return &decisionsDocker{client: dockerutil.NewClient(host, 45*time.Second), host: dockerutil.NormalizeHost(host)}
}

func dockerBackendOrder() []string { return []string{"cuda", "vulkan", "cpu"} }

func (docker *decisionsDocker) image(ctx context.Context, backend string) (dockerImageMetadata, error) {
	var info dockerImageMetadata
	image := "aurago-decisions-runtime:" + backend
	if pinned := releaseRuntimeCatalog.image(backend); pinned != "" {
		image = pinned
	}
	_, err := docker.client.DoJSON(ctx, http.MethodGet, "images/"+url.PathEscape(image)+"/json", nil, &info)
	if err != nil {
		return dockerImageMetadata{}, err
	}
	labels := info.Config.Labels
	if !validDockerImageID(info.ID) || !strings.EqualFold(info.OS, "linux") || !strings.EqualFold(info.Architecture, runtime.GOARCH) ||
		labels["org.aurago.decisions.protocol"] != "v1" || labels["org.aurago.decisions.backend"] != backend ||
		labels["org.aurago.decisions.llama_cpp_patch_sha256"] != llamaCPPPatchSHA256 ||
		labels["org.opencontainers.image.source"] != "https://github.com/antibyte/AuraGo" || labels["org.opencontainers.image.revision"] != llamaCPPCommit {
		return dockerImageMetadata{}, NewError(CodeRuntimeAttestationFailed, fmt.Errorf("local decisions image labels do not match runtime pins"))
	}
	return info, nil
}

func verifyLocalDockerRuntimeImages(ctx context.Context, access RuntimeAccess, _ []string) error {
	if !dockerMayMutate(access) {
		return NewError(CodeDeploymentDenied, nil)
	}
	docker := newDecisionsDocker(access.DockerHost)
	found := false
	for _, backend := range dockerBackendOrder() {
		if _, err := docker.image(ctx, backend); err == nil {
			found = true
		}
	}
	if !found {
		return NewError(CodeRuntimeArtifactMissing, fmt.Errorf("no locally built, labeled decisions runtime image is available"))
	}
	return nil
}

func discoverDockerDevices(ctx context.Context, access RuntimeAccess, paths modelPaths, resolveSelf func(context.Context, string) (string, error)) ([]runtimeDevice, error) {
	if !dockerMayMutate(access) {
		return nil, NewError(CodeDeploymentDenied, nil)
	}
	docker := newDecisionsDocker(access.DockerHost)
	var devices []runtimeDevice
	imagesFound := false
	for _, backend := range dockerBackendOrder() {
		image, err := docker.image(ctx, backend)
		if err != nil {
			continue
		}
		imagesFound = true
		output, err := docker.runDiscover(ctx, backend, image.ID, paths, access, resolveSelf)
		if err != nil {
			continue
		}
		var result struct {
			Protocol    string           `json:"protocol"`
			Build       runtimeBuildInfo `json:"build"`
			Devices     []runtimeDevice  `json:"devices"`
			ModelLayers int              `json:"model_layers"`
		}
		if json.Unmarshal(output, &result) != nil || result.Protocol != runtimeProtocol || result.Build.LlamaCPPCommit != llamaCPPCommit ||
			result.Build.LlamaCPPPatchSHA256 != llamaCPPPatchSHA256 ||
			!strings.EqualFold(result.Build.Backend, backend) || !validSHA256(result.Build.BinarySHA256) {
			continue
		}
		for _, device := range result.Devices {
			if device.ID == "" || device.Name == "" || !strings.EqualFold(device.Backend, backend) {
				continue
			}
			device.BinarySHA = result.Build.BinarySHA256
			device.ModelLayers = result.ModelLayers
			device.DockerImageID = image.ID
			if strings.EqualFold(device.Type, "IGPU") {
				device.Type = "GPU"
			}
			devices = append(devices, device)
		}
	}
	if !imagesFound {
		return nil, NewError(CodeRuntimeArtifactMissing, fmt.Errorf("no locally built decisions runtime image is installed"))
	}
	return devices, nil
}

func (docker *decisionsDocker) runDiscover(ctx context.Context, backend, imageID string, paths modelPaths, access RuntimeAccess, resolveSelf func(context.Context, string) (string, error)) ([]byte, error) {
	name := "aurago-decisions-discover-" + backend + "-" + randomSuffix()
	var mounts []map[string]any
	args := []string{"--discover"}
	if paths.Model != "" {
		modelMount, _, err := dockerResolvedModelMount(ctx, docker, access, filepath.Dir(paths.Model), resolveSelf)
		if err != nil {
			return nil, err
		}
		mounts = append(mounts, modelMount)
		args = append(args, "--model", "/models/"+filepath.Base(paths.Model))
	}
	container, err := docker.createContainer(ctx, name, imageID, args, mounts, dockerHostConfig(backend, "none", nil))
	if err != nil {
		return nil, err
	}
	defer docker.removeContainerBounded(container.ID)
	if err := docker.startContainer(ctx, container.ID); err != nil {
		return nil, err
	}
	if _, err := docker.waitContainer(ctx, container.ID); err != nil {
		return nil, err
	}
	var current dockerContainerMetadata
	if _, err := docker.client.DoJSON(ctx, http.MethodGet, "containers/"+url.PathEscape(container.ID)+"/json", nil, &current); err != nil {
		return nil, err
	}
	if !dockerContainerImageMatches(imageID, current.Image) || current.State.ExitCode != 0 {
		return nil, fmt.Errorf("runtime discovery process exited unsuccessfully")
	}
	return docker.containerLogs(ctx, container.ID, 1)
}

func (docker *decisionsDocker) createContainer(ctx context.Context, name, image string, command []string, mounts []map[string]any, hostConfig map[string]any) (dockerContainerMetadata, error) {
	configBody := dockerContainerCreateBody(image, command, mounts, hostConfig)
	configBody["Labels"] = map[string]string{"aurago.managed": "decisions", "aurago.component": "local-decisions-runtime", "org.aurago.decisions.protocol": "v1"}
	configBody["ExposedPorts"] = map[string]any{fmt.Sprintf("%d/tcp", decisionsDockerPort): map[string]any{}}
	var created struct {
		ID string `json:"Id"`
	}
	_, err := docker.client.DoJSON(ctx, http.MethodPost, "containers/create?name="+url.QueryEscape(name), configBody, &created)
	if err != nil || created.ID == "" {
		if err == nil {
			err = fmt.Errorf("Docker did not return a container id")
		}
		return dockerContainerMetadata{}, err
	}
	return dockerContainerMetadata{ID: created.ID}, nil
}

func dockerContainerCreateBody(image string, command []string, mounts []map[string]any, hostConfig map[string]any) map[string]any {
	copyHostConfig := make(map[string]any, len(hostConfig)+1)
	for key, value := range hostConfig {
		copyHostConfig[key] = value
	}
	if len(mounts) > 0 {
		copyHostConfig["Mounts"] = mounts
	}
	configBody := map[string]any{
		"Image": image, "Cmd": command, "Tty": false, "HostConfig": copyHostConfig,
	}
	return configBody
}

func dockerContainerImageMatches(expected, actual string) bool {
	return expected != "" && actual == expected
}

func validDockerImageID(value string) bool {
	if !strings.HasPrefix(value, "sha256:") || len(value) != len("sha256:")+hex.EncodedLen(32) {
		return false
	}
	_, err := hex.DecodeString(strings.TrimPrefix(value, "sha256:"))
	return err == nil
}

func dockerHostConfig(backend, network string, portBindings map[string]any) map[string]any {
	// 8 GiB covers the pinned Q4 weights (1,274,396,448 bytes), the projector
	// (671,372,608 bytes), KV and compute for the shared 8192-token budget, and
	// a consented CPU fallback that keeps those weights in host RAM.
	// MemorySwap == Memory means the cgroup adds no swap, matching the CYD
	// builder pattern (Memory and MemorySwap equal) at a size that fits this
	// model rather than the builder's 2 GiB.
	//
	// Docker Memory limits host RAM, not GPU VRAM. Device ids from the runtime
	// are ggml_backend_dev_name values (device_json in decisions_runtime/src/main.cpp),
	// not NVIDIA indexes or DRM render-node paths. Do not set DeviceIDs or narrow
	// /dev/dri in this task. A wrong filter fails attestation and surfaces as missing hardware.
	host := map[string]any{
		"AutoRemove": false, "ReadonlyRootfs": true, "PidsLimit": int64(128), "CapDrop": []string{"ALL"},
		"SecurityOpt": []string{"no-new-privileges"}, "NetworkMode": network,
		"Memory": int64(8 << 30), "MemorySwap": int64(8 << 30),
		"Tmpfs":     map[string]string{"/tmp": "rw,noexec,nosuid,size=64m"},
		"LogConfig": map[string]any{"Type": "json-file", "Config": map[string]string{"max-size": "1m", "max-file": "2"}},
	}
	if portBindings != nil {
		host["PortBindings"] = portBindings
	}
	switch strings.ToLower(backend) {
	case "cuda":
		host["DeviceRequests"] = []map[string]any{{"Driver": "nvidia", "Count": -1, "Capabilities": [][]string{{"gpu"}}}}
	case "vulkan":
		host["Devices"] = []map[string]string{{"PathOnHost": "/dev/dri", "PathInContainer": "/dev/dri", "CgroupPermissions": "rwm"}}
	}
	return host
}

func (docker *decisionsDocker) startContainer(ctx context.Context, id string) error {
	_, err := docker.client.DoJSON(ctx, http.MethodPost, "containers/"+url.PathEscape(id)+"/start", nil, nil)
	return err
}

func (docker *decisionsDocker) waitContainer(ctx context.Context, id string) (int, error) {
	var result struct {
		StatusCode int `json:"StatusCode"`
	}
	_, err := docker.client.DoJSON(ctx, http.MethodPost, "containers/"+url.PathEscape(id)+"/wait?condition=not-running", nil, &result)
	return result.StatusCode, err
}

func (docker *decisionsDocker) removeContainer(ctx context.Context, id string) {
	_, _ = docker.client.DoJSON(ctx, http.MethodDelete, "containers/"+url.PathEscape(id)+"?force=true&v=true", nil, nil)
}

func (docker *decisionsDocker) removeContainerBounded(id string) {
	docker.removeContainerWithin(id, decisionsDockerCleanupTimeout)
}

func (docker *decisionsDocker) removeContainerWithin(id string, timeout time.Duration) {
	withDockerCleanupContext(timeout, func(ctx context.Context) {
		docker.removeContainer(ctx, id)
	})
}

func withDockerCleanupContext(timeout time.Duration, cleanup func(context.Context)) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cleanup(ctx)
}

func (docker *decisionsDocker) containerLogs(ctx context.Context, id string, stream byte) ([]byte, error) {
	query := "stdout=1&stderr=0"
	if stream == 2 {
		query = "stdout=0&stderr=1&tail=64"
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, dockerutil.Endpoint("containers/"+url.PathEscape(id)+"/logs?"+query), nil)
	if err != nil {
		return nil, err
	}
	response, err := docker.client.HTTPClient().Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Docker runtime log request returned HTTP %d", response.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, (1<<20)+1))
	if err != nil {
		return nil, err
	}
	if len(data) > 1<<20 {
		return nil, fmt.Errorf("Docker runtime discovery output exceeded limit")
	}
	stdout, err := dockerLogStream(data, stream)
	if err != nil {
		return nil, err
	}
	return stdout, nil
}

// dockerLogStream keeps discovery stdout separate from startup diagnostics on stderr.
func dockerLogStream(data []byte, wanted byte) ([]byte, error) {
	var stdout bytes.Buffer
	for len(data) > 0 {
		if len(data) < 8 {
			return nil, fmt.Errorf("Docker runtime discovery log frame is truncated")
		}
		stream := data[0]
		if stream != 1 && stream != 2 {
			return nil, fmt.Errorf("Docker runtime discovery log frame has invalid stream")
		}
		if data[1] != 0 || data[2] != 0 || data[3] != 0 {
			return nil, fmt.Errorf("Docker runtime discovery log frame has invalid reserved bytes")
		}
		frameLengthBytes := binary.BigEndian.Uint32(data[4:8])
		data = data[8:]
		if uint64(frameLengthBytes) > uint64(len(data)) {
			return nil, fmt.Errorf("Docker runtime discovery log frame payload is truncated")
		}
		frameLength := int(frameLengthBytes)
		if stream == wanted {
			_, _ = stdout.Write(data[:frameLength])
		}
		data = data[frameLength:]
	}
	return stdout.Bytes(), nil
}

func startDockerRuntime(ctx context.Context, device runtimeDevice, paths modelPaths, cfg config.DecisionsConfig, access RuntimeAccess, dataDir string, resolveSelf func(context.Context, string) (string, error), progress func(string)) (runtimeSession, error) {
	if !dockerMayMutate(access) {
		return nil, NewError(CodeDeploymentDenied, nil)
	}
	docker := newDecisionsDocker(access.DockerHost)
	image, err := docker.image(ctx, strings.ToLower(device.Backend))
	if err != nil {
		return nil, NewError(CodeRuntimeArtifactMissing, err)
	}
	if !dockerContainerImageMatches(device.DockerImageID, image.ID) {
		return nil, NewError(CodeRuntimeAttestationFailed, fmt.Errorf("local decisions image changed after discovery"))
	}
	port, err := availableLoopbackPort()
	if err != nil {
		return nil, NewError(CodeRuntimeStartFailed, err)
	}
	tokenFile, err := writeRuntimeToken(filepath.Join(dataDir, "decisions"))
	if err != nil {
		return nil, NewError(CodeRuntimeStartFailed, err)
	}
	token, err := os.ReadFile(tokenFile)
	if err != nil {
		_ = os.Remove(tokenFile)
		return nil, NewError(CodeRuntimeStartFailed, err)
	}
	mounts, modelPath, tokenPath, network, hostConfig, cleanup, err := prepareDockerRuntime(ctx, docker, access, paths, tokenFile, port, device.Backend, resolveSelf)
	if err != nil {
		_ = os.Remove(tokenFile)
		return nil, NewError(CodeRuntimeStartFailed, err)
	}
	args := []string{"--model", modelPath, "--model-revision", modelRevision, "--model-sha256", DefaultManifest().Model.SHA256,
		"--mmproj", filepath.ToSlash(filepath.Join(filepath.Dir(modelPath), DefaultManifest().Projector.Filename)), "--mmproj-revision", modelRevision,
		"--mmproj-sha256", DefaultManifest().Projector.SHA256, "--bind", "0.0.0.0", "--port", fmt.Sprint(decisionsDockerPort), "--token-file", tokenPath}
	if strings.EqualFold(device.Type, "CPU") {
		if !cfg.CPUFallbackAllowed {
			cleanup()
			_ = os.Remove(tokenFile)
			return nil, NewError(CodeCPUConsentRequired, nil)
		}
		args = append(args, "--allow-cpu")
	} else if device.GPULayers > 0 {
		if !cfg.CPUFallbackAllowed {
			cleanup()
			_ = os.Remove(tokenFile)
			return nil, NewError(CodeCPUConsentRequired, nil)
		}
		args = append(args, "--device", device.ID, "--allow-cpu-offload", "--gpu-layers", fmt.Sprint(device.GPULayers))
	} else {
		args = append(args, "--device", device.ID)
	}
	name := "aurago-decisions-runtime-" + randomSuffix()
	hostConfig["NetworkMode"] = network
	// Use the immutable ID returned by image inspection; a tag can be moved
	// between discovery and container creation.
	container, err := docker.createContainer(ctx, name, image.ID, args, mounts, hostConfig)
	if err != nil {
		cleanup()
		_ = os.Remove(tokenFile)
		return nil, NewError(CodeRuntimeStartFailed, err)
	}
	if err := docker.startContainer(ctx, container.ID); err != nil {
		docker.removeContainerBounded(container.ID)
		cleanup()
		_ = os.Remove(tokenFile)
		return nil, NewError(CodeRuntimeStartFailed, err)
	}
	baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)
	if access.IsDocker {
		baseURL = fmt.Sprintf("http://%s:%d", name, decisionsDockerPort)
	}
	session := &dockerRuntimeSession{nativeRuntimeSession: &nativeRuntimeSession{
		baseURL: baseURL, token: tokenFile, client: &http.Client{Timeout: 2 * time.Minute, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }},
		device: device, deployment: config.DecisionsDeploymentDocker, cpuAllowed: cfg.CPUFallbackAllowed,
	}, docker: docker, containerID: container.ID, cleanup: cleanup, imageID: image.ID, tokenValue: strings.TrimSpace(string(token))}
	var running dockerContainerMetadata
	if _, inspectErr := docker.client.DoJSON(ctx, http.MethodGet, "containers/"+url.PathEscape(container.ID)+"/json", nil, &running); inspectErr != nil || !dockerContainerImageMatches(image.ID, running.Image) {
		_ = session.Close(context.Background())
		return nil, NewError(CodeRuntimeAttestationFailed, fmt.Errorf("running container image differs from inspected image"))
	}
	if err := docker.waitRuntimeReady(ctx, session, progress); err != nil {
		_ = session.Close(context.Background())
		return nil, err
	}
	return session, nil
}

func prepareDockerRuntime(ctx context.Context, docker *decisionsDocker, access RuntimeAccess, paths modelPaths, tokenFile string, port int, backend string, resolveSelf func(context.Context, string) (string, error)) ([]map[string]any, string, string, string, map[string]any, func(), error) {
	modelDirectory := filepath.Dir(paths.Model)
	modelMount, selfID, err := dockerResolvedModelMount(ctx, docker, access, modelDirectory, resolveSelf)
	if err != nil {
		return nil, "", "", "", nil, func() {}, err
	}
	tokenMount, tokenTarget, _, tokenSelfID, err := dockerResolvedPathMount(ctx, docker, access, filepath.Dir(tokenFile), "/run/decisions", resolveSelf)
	if err != nil {
		return nil, "", "", "", nil, func() {}, err
	}
	if selfID != tokenSelfID {
		return nil, "", "", "", nil, func() {}, NewError(CodeDeploymentDenied, fmt.Errorf("model and token mounts resolve to different AuraGo containers"))
	}
	networkName := "aurago-decisions-internal-" + randomSuffix()
	var networkID string
	cleanup := func() {
		withDockerCleanupContext(decisionsDockerCleanupTimeout, func(cleanupCtx context.Context) {
			if selfID != "" && networkID != "" {
				_, _ = docker.client.DoJSON(cleanupCtx, http.MethodPost, "networks/"+url.PathEscape(networkID)+"/disconnect", map[string]any{"Container": selfID, "Force": true}, nil)
			}
			if networkID != "" {
				_, _ = docker.client.DoJSON(cleanupCtx, http.MethodDelete, "networks/"+url.PathEscape(networkID), nil, nil)
			}
		})
	}
	var network struct {
		ID string `json:"Id"`
	}
	_, err = docker.client.DoJSON(ctx, http.MethodPost, "networks/create", map[string]any{"Name": networkName, "Internal": true, "Attachable": false, "Labels": map[string]string{"aurago.managed": "decisions", "aurago.component": "local-decisions-runtime"}}, &network)
	if err != nil || network.ID == "" {
		cleanup()
		return nil, "", "", "", nil, func() {}, fmt.Errorf("create private decisions network")
	}
	networkID = network.ID
	if selfID != "" {
		_, err = docker.client.DoJSON(ctx, http.MethodPost, "networks/"+url.PathEscape(networkID)+"/connect", map[string]any{"Container": selfID}, nil)
		if err != nil {
			cleanup()
			return nil, "", "", "", nil, func() {}, fmt.Errorf("connect AuraGo to decisions network")
		}
	}
	modelPath := "/models/" + filepath.Base(paths.Model)
	tokenPath := "/run/decisions/" + filepath.Base(tokenFile)
	mounts := []map[string]any{modelMount, tokenMount}
	bindings := map[string]any(nil)
	containerNetwork := networkName
	if !access.IsDocker {
		bindings = map[string]any{fmt.Sprintf("%d/tcp", decisionsDockerPort): []map[string]string{{"HostIp": "127.0.0.1", "HostPort": fmt.Sprint(port)}}}
	}
	_ = tokenTarget
	return mounts, modelPath, tokenPath, containerNetwork, dockerHostConfig(backend, containerNetwork, bindings), cleanup, nil
}

func dockerResolvedModelMount(ctx context.Context, docker *decisionsDocker, access RuntimeAccess, modelDir string, resolveSelf func(context.Context, string) (string, error)) (map[string]any, string, error) {
	mount, _, _, selfID, err := dockerResolvedPathMount(ctx, docker, access, modelDir, "/models", resolveSelf)
	return mount, selfID, err
}

func dockerResolvedPathMount(ctx context.Context, docker *decisionsDocker, access RuntimeAccess, sourceDir, target string, resolveSelf func(context.Context, string) (string, error)) (map[string]any, string, string, string, error) {
	if !access.IsDocker {
		if isRemoteDockerHost(docker.host) {
			return nil, "", "", "", NewError(CodeDeploymentDenied, fmt.Errorf("remote Docker hosts cannot safely map local decision files"))
		}
		sourceAbs, err := filepath.Abs(sourceDir)
		if err != nil {
			return nil, "", "", "", err
		}
		return map[string]any{"Type": "bind", "Source": sourceAbs, "Target": target, "ReadOnly": true}, sourceAbs, "", "", nil
	}
	if resolveSelf == nil {
		return nil, "", "", "", NewError(CodeDeploymentDenied, fmt.Errorf("verified AuraGo container identity is unavailable"))
	}
	selfID, err := resolveSelf(ctx, access.DockerHost)
	if err != nil || selfID == "" {
		return nil, "", "", "", NewError(CodeDeploymentDenied, fmt.Errorf("verified AuraGo container identity could not be resolved"))
	}
	var self dockerContainerMetadata
	_, err = docker.client.DoJSON(ctx, http.MethodGet, "containers/"+url.PathEscape(selfID)+"/json", nil, &self)
	if err != nil || self.ID == "" || self.ID != selfID || !self.State.Running {
		return nil, "", "", "", NewError(CodeDeploymentDenied, fmt.Errorf("verified AuraGo container is not running"))
	}
	hostSource, subpath, err := resolveContainerPath(sourceDir, self.Mounts)
	if err != nil {
		return nil, "", "", "", err
	}
	return dockerReadonlyMount(hostSource, target, subpath), hostSource, subpath, self.ID, nil
}

func resolveContainerPath(target string, mounts []struct {
	Type        string `json:"Type"`
	Name        string `json:"Name"`
	Source      string `json:"Source"`
	Destination string `json:"Destination"`
}) (string, string, error) {
	cleanTarget := filepath.ToSlash(filepath.Clean(target))
	best := -1
	var selected struct {
		Type        string `json:"Type"`
		Name        string `json:"Name"`
		Source      string `json:"Source"`
		Destination string `json:"Destination"`
	}
	var relative string
	for _, mount := range mounts {
		destination := filepath.ToSlash(filepath.Clean(mount.Destination))
		// Reject a textual sibling of a nested mount (for example
		// /app/data-evil when /app/data is separately mounted). Although a
		// broader /app mount could technically resolve it, silently mapping it
		// through that parent makes container path intent ambiguous.
		if len(cleanTarget) > len(destination) && strings.HasPrefix(cleanTarget, destination) && cleanTarget[len(destination)] != '/' {
			return "", "", NewError(CodeDeploymentDenied, fmt.Errorf("container path is adjacent to a mounted decisions path"))
		}
		if cleanTarget != destination && !strings.HasPrefix(cleanTarget, strings.TrimSuffix(destination, "/")+"/") {
			continue
		}
		if len(destination) <= best {
			continue
		}
		selected = mount
		relative = strings.TrimPrefix(strings.TrimPrefix(cleanTarget, destination), "/")
		best = len(destination)
	}
	if best < 0 {
		return "", "", NewError(CodeDeploymentDenied, fmt.Errorf("decisions data is not backed by a Docker bind or volume"))
	}
	switch strings.ToLower(selected.Type) {
	case "bind":
		return filepath.ToSlash(filepath.Join(selected.Source, filepath.FromSlash(relative))), "", nil
	case "volume":
		source := selected.Name
		if source == "" {
			source = selected.Source
		}
		if source == "" {
			return "", "", fmt.Errorf("decisions Docker volume source is missing")
		}
		return source, relative, nil
	default:
		return "", "", NewError(CodeDeploymentDenied, fmt.Errorf("unsupported Docker decisions mount type"))
	}
}

func dockerReadonlyMount(source, target, subpath string) map[string]any {
	mount := map[string]any{"Type": "bind", "Source": source, "Target": target, "ReadOnly": true}
	if !filepath.IsAbs(source) {
		mount["Type"] = "volume"
		if subpath != "" {
			mount["VolumeOptions"] = map[string]any{"Subpath": subpath}
		}
	}
	return mount
}

func isRemoteDockerHost(host string) bool {
	return strings.HasPrefix(host, "tcp://") || strings.HasPrefix(host, "http://") || strings.HasPrefix(host, "https://")
}

type dockerRuntimeSession struct {
	*nativeRuntimeSession
	docker      *decisionsDocker
	containerID string
	cleanup     func()
	imageID     string
	tokenValue  string
}

func (session *dockerRuntimeSession) Close(ctx context.Context) error {
	if session.containerID != "" {
		_, stopErr := session.docker.client.DoJSON(ctx, http.MethodPost, "containers/"+url.PathEscape(session.containerID)+"/stop?t=2", nil, nil)
		session.docker.removeContainerBounded(session.containerID)
		session.containerID = ""
		if session.cleanup != nil {
			session.cleanup()
		}
		if session.token != "" {
			_ = os.Remove(session.token)
		}
		if stopErr != nil {
			return stopErr
		}
	}
	return nil
}

func (docker *decisionsDocker) waitRuntimeReady(ctx context.Context, session *dockerRuntimeSession, progress func(string)) error {
	return waitRuntimeReady(ctx, runtimeStartupTimeout, func(probeCtx context.Context) (bool, error) {
		var container dockerContainerMetadata
		if _, err := docker.client.DoJSON(probeCtx, http.MethodGet, "containers/"+url.PathEscape(session.containerID)+"/json", nil, &container); err != nil {
			return false, nil
		}
		if !dockerContainerImageMatches(session.imageID, container.Image) {
			return false, NewError(CodeRuntimeAttestationFailed, nil)
		}
		startup := &nativeStartupStderr{}
		if logs, err := docker.containerLogs(probeCtx, session.containerID, 2); err == nil {
			_, _ = startup.Write(logs)
		}
		if phase := startup.progress(); progress != nil && phase != "" {
			progress(phase)
		}
		if !container.State.Running {
			return false, runtimeStartupError(startup.code())
		}
		metadata, err := session.nativeRuntimeSession.metadata(probeCtx)
		if err == nil && metadata.Ready {
			session.cached = metadata
			return true, nil
		}
		return false, nil
	})
}

func randomSuffix() string {
	return hex.EncodeToString(randomBytes(6))
}

func randomBytes(size int) []byte {
	data := make([]byte, size)
	if _, err := rand.Read(data); err != nil {
		return []byte(fmt.Sprintf("%012x", time.Now().UnixNano()))
	}
	return data
}

var _ = os.ErrNotExist
