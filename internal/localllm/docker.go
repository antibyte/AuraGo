package localllm

import (
	"archive/tar"
	"bufio"
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"aurago/internal/config"
	"aurago/internal/dockerutil"
)

const managedContainerName = dockerutil.LocalLLMContainerName

const (
	runtimeKeyVolumeName = dockerutil.LocalLLMKeyVolumeName
	runtimeKeySeedName   = dockerutil.LocalLLMKeySeedName
)

type dockerEngine interface {
	DoJSON(context.Context, string, string, any, any) (int, error)
	HTTPClient() *http.Client
}

type dockerContainerSpec struct {
	Image      string            `json:"Image"`
	User       string            `json:"User"`
	Env        []string          `json:"Env"`
	Labels     map[string]string `json:"Labels"`
	HostConfig dockerHostConfig  `json:"HostConfig"`
}

type dockerHostConfig struct {
	ReadonlyRootfs bool                        `json:"ReadonlyRootfs"`
	CapDrop        []string                    `json:"CapDrop"`
	SecurityOpt    []string                    `json:"SecurityOpt"`
	PidsLimit      int64                       `json:"PidsLimit"`
	Tmpfs          map[string]string           `json:"Tmpfs"`
	Mounts         []dockerMount               `json:"Mounts"`
	Devices        []dockerDevice              `json:"Devices,omitempty"`
	DeviceRequests []dockerDeviceRequest       `json:"DeviceRequests,omitempty"`
	GroupAdd       []string                    `json:"GroupAdd,omitempty"`
	NetworkMode    string                      `json:"NetworkMode"`
	PortBindings   map[string][]dockerPortBind `json:"PortBindings,omitempty"`
	LogConfig      dockerLogConfig             `json:"LogConfig"`
}

type dockerMount struct {
	Type     string `json:"Type"`
	Source   string `json:"Source"`
	Target   string `json:"Target"`
	ReadOnly bool   `json:"ReadOnly"`
}

type dockerDevice struct {
	PathOnHost        string `json:"PathOnHost"`
	PathInContainer   string `json:"PathInContainer"`
	CgroupPermissions string `json:"CgroupPermissions"`
}

type dockerDeviceRequest struct {
	Driver       string            `json:"Driver"`
	DeviceIDs    []string          `json:"DeviceIDs,omitempty"`
	Capabilities [][]string        `json:"Capabilities"`
	Options      map[string]string `json:"Options,omitempty"`
}

type dockerPortBind struct {
	HostIP   string `json:"HostIp"`
	HostPort string `json:"HostPort"`
}

type dockerLogConfig struct {
	Type   string            `json:"Type"`
	Config map[string]string `json:"Config"`
}

func (m *Manager) containerSpec(profile HardwareProfile, model Artifact, draft *Artifact, image Image) (dockerContainerSpec, error) {
	m.mu.Lock()
	cfg := m.cfg
	fingerprint := m.desiredFingerprint
	m.mu.Unlock()
	return m.containerSpecValues(cfg, fingerprint, profile, model, draft, image)
}

func (m *Manager) containerSpecFor(plan runtimePlan) (dockerContainerSpec, error) {
	return m.containerSpecValues(
		plan.Config,
		plan.Fingerprint,
		plan.Profile,
		plan.Model,
		plan.Draft,
		plan.Image,
	)
}

func (m *Manager) containerSpecValues(cfg config.LocalLLMConfig, fingerprint string, profile HardwareProfile, model Artifact, draft *Artifact, image Image) (dockerContainerSpec, error) {
	digestIndex := strings.LastIndex(image.Reference, "@sha256:")
	if digestIndex < 0 {
		return dockerContainerSpec{}, fmt.Errorf("image_digest_unavailable")
	}
	imageDigest := image.Reference[digestIndex+1:]
	imageDigestHex := strings.TrimPrefix(imageDigest, "sha256:")
	if len(imageDigestHex) != 64 || imageDigestHex != strings.ToLower(imageDigestHex) {
		return dockerContainerSpec{}, fmt.Errorf("image_digest_unavailable")
	}
	if _, err := hex.DecodeString(imageDigestHex); err != nil {
		return dockerContainerSpec{}, fmt.Errorf("image_digest_unavailable")
	}
	resolvedParametersJSON, err := json.Marshal(resolvedParametersForPlan(cfg, draft != nil, profile))
	if err != nil {
		return dockerContainerSpec{}, fmt.Errorf("resolved_parameters_unavailable")
	}
	draftSHA256 := ""
	if draft != nil {
		draftSHA256 = draft.SHA256
	}
	env := []string{
		"AURAGO_MODEL=/models/" + model.Name,
		"AURAGO_HOST=0.0.0.0",
		"AURAGO_PORT=8080",
		"AURAGO_ALIAS=aurago-qwen",
		"AURAGO_BACKEND=" + profile.SelectedBackend,
		"AURAGO_API_KEY_FILE=/run/aurago-local-llm/api-key",
		"AURAGO_FIT=off",
		"AURAGO_KV_OFFLOAD=on",
		"AURAGO_REASONING=off",
		"AURAGO_CONTEXT_SIZE=" + strconv.Itoa(cfg.ContextSize),
		"AURAGO_PARALLEL=1",
		"AURAGO_IMAGE_DIGEST=" + imageDigest,
		"AURAGO_TARGET_SHA256=" + model.SHA256,
		"AURAGO_DRAFT_SHA256=" + draftSHA256,
		"AURAGO_PHYSICAL_DEVICE=" + profile.SelectedDevice,
		"AURAGO_RESOLVED_PARAMETERS_JSON=" + string(resolvedParametersJSON),
	}
	runtimeDevice := resolvedRuntimeDevice(profile)
	if profile.SelectedBackend != "cpu" {
		if runtimeDevice == "" {
			return dockerContainerSpec{}, fmt.Errorf("runtime_device_unavailable")
		}
		env = append(env,
			"AURAGO_DEVICE="+runtimeDevice,
			"AURAGO_GPU_LAYERS=all",
		)
		if profile.SelectedBackend == "sycl" {
			env = append(env, "ONEAPI_DEVICE_SELECTOR=level_zero:gpu")
		}
	}
	if draft != nil {
		draftNGL := "999"
		if profile.SelectedBackend == "cpu" {
			draftNGL = "0"
		}
		env = append(env,
			"AURAGO_SPEC_TYPE=draft-mtp",
			"AURAGO_DRAFT_MODEL=/models/"+draft.Name,
			"AURAGO_DRAFT_DEVICE="+runtimeDevice,
			"AURAGO_SPEC_DRAFT_N_MAX=2",
			"AURAGO_SPEC_DRAFT_N_MIN=0",
			"AURAGO_SPEC_DRAFT_P_MIN=0.80",
			"AURAGO_SPEC_DRAFT_NGL="+draftNGL,
		)
	} else {
		env = append(env, "AURAGO_SPEC_TYPE=none")
	}
	host := dockerHostConfig{
		ReadonlyRootfs: true,
		CapDrop:        []string{"ALL"},
		SecurityOpt:    []string{"no-new-privileges"},
		PidsLimit:      512,
		Tmpfs:          map[string]string{"/tmp": "rw,noexec,nosuid,nodev,size=12g"},
		LogConfig: dockerLogConfig{
			Type:   "json-file",
			Config: map[string]string{"max-size": "10m", "max-file": "3"},
		},
	}
	if m.runningInDocker {
		host.NetworkMode = "aurago-app"
		host.Mounts = []dockerMount{
			{Type: "volume", Source: "aurago_models", Target: "/models", ReadOnly: true},
			{Type: "volume", Source: runtimeKeyVolumeName, Target: "/run/aurago-local-llm", ReadOnly: true},
		}
	} else {
		host.NetworkMode = "bridge"
		host.Mounts = []dockerMount{
			{Type: "bind", Source: filepath.Clean(m.modelDir), Target: "/models", ReadOnly: true},
			{Type: "volume", Source: runtimeKeyVolumeName, Target: "/run/aurago-local-llm", ReadOnly: true},
		}
		host.PortBindings = map[string][]dockerPortBind{
			"8080/tcp": {{HostIP: "127.0.0.1", HostPort: strconv.Itoa(cfg.ListenPort)}},
		}
	}
	gpu, err := profile.selectedGPU()
	if cfg.Backend != "cpu" && err != nil {
		return dockerContainerSpec{}, err
	}
	if cfg.Backend != "cpu" {
		if profile.SelectedBackend == "cuda" {
			if gpu.DockerID == "" {
				return dockerContainerSpec{}, fmt.Errorf("nvidia_toolkit_identity_unavailable")
			}
			host.DeviceRequests = []dockerDeviceRequest{{
				Driver: "nvidia", DeviceIDs: []string{gpu.DockerID}, Capabilities: [][]string{{"gpu", "compute", "utility"}},
			}}
		} else {
			if gpu.RenderNode == "" {
				return dockerContainerSpec{}, fmt.Errorf("gpu_render_node_unavailable")
			}
			host.Devices = []dockerDevice{{
				PathOnHost: gpu.RenderNode, PathInContainer: gpu.RenderNode, CgroupPermissions: "rwm",
			}}
			host.GroupAdd = m.gpuGroupIDs(gpu.RenderNode)
			if len(host.GroupAdd) == 0 {
				return dockerContainerSpec{}, fmt.Errorf("gpu_group_ids_unavailable")
			}
		}
	}
	return dockerContainerSpec{
		Image: image.Reference, User: "65532:65532", Env: env,
		Labels:     dockerutil.ManagedLabels(dockerutil.LocalLLMOwner, "aurago-qwen", "runtime", fingerprint),
		HostConfig: host,
	}, nil
}

// prepareRuntimeKeyVolume materializes the secret without exposing it through
// container environment variables, arguments, host paths, or Docker logs.
func (m *Manager) prepareRuntimeKeyVolume(ctx context.Context, imageReference, key, fingerprint string) error {
	_, _ = m.docker.DoJSON(ctx, http.MethodPost, "containers/"+managedContainerName+"/stop?t=5", nil, nil)
	_ = m.deleteContainer(ctx, managedContainerName)
	_ = m.deleteContainer(ctx, runtimeKeySeedName)
	_ = m.deleteRuntimeKeyVolume(ctx)
	if _, err := m.docker.DoJSON(ctx, http.MethodPost, "volumes/create", map[string]any{
		"Name":   runtimeKeyVolumeName,
		"Labels": dockerutil.ManagedLabels(dockerutil.LocalLLMOwner, "aurago-qwen", "runtime-key-volume", fingerprint),
	}, nil); err != nil {
		return fmt.Errorf("runtime_volume_create_failed: %w", err)
	}
	seedSpec := dockerContainerSpec{
		Image:  imageReference,
		User:   "65532:65532",
		Labels: dockerutil.ManagedLabels(dockerutil.LocalLLMOwner, "aurago-qwen", "runtime-key-seed", fingerprint),
		HostConfig: dockerHostConfig{
			ReadonlyRootfs: true,
			CapDrop:        []string{"ALL"},
			SecurityOpt:    []string{"no-new-privileges"},
			PidsLimit:      16,
			NetworkMode:    "none",
			Mounts: []dockerMount{{
				Type: "volume", Source: runtimeKeyVolumeName,
				Target: "/run/aurago-local-llm", ReadOnly: false,
			}},
			LogConfig: dockerLogConfig{Type: "none"},
		},
	}
	if _, err := m.docker.DoJSON(ctx, http.MethodPost, "containers/create?name="+runtimeKeySeedName, seedSpec, nil); err != nil {
		_ = m.deleteRuntimeKeyVolume(ctx)
		return fmt.Errorf("runtime_key_seed_create_failed: %w", err)
	}
	defer m.deleteContainer(context.Background(), runtimeKeySeedName)
	if err := m.copyRuntimeKeyArchive(ctx, key); err != nil {
		_ = m.deleteContainer(context.Background(), runtimeKeySeedName)
		_ = m.deleteRuntimeKeyVolume(context.Background())
		return err
	}
	if err := m.deleteContainer(ctx, runtimeKeySeedName); err != nil {
		return fmt.Errorf("runtime_key_seed_cleanup_failed: %w", err)
	}
	return nil
}

func (m *Manager) copyRuntimeKeyArchive(ctx context.Context, key string) error {
	var archive bytes.Buffer
	writer := tar.NewWriter(&archive)
	payload := []byte(key + "\n")
	header := &tar.Header{
		Name: "api-key", Mode: 0o600, Uid: 65532, Gid: 65532,
		Size: int64(len(payload)),
	}
	if err := writer.WriteHeader(header); err != nil {
		return fmt.Errorf("runtime_key_archive_failed")
	}
	if _, err := writer.Write(payload); err != nil {
		return fmt.Errorf("runtime_key_archive_failed")
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("runtime_key_archive_failed")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPut,
		dockerutil.Endpoint("containers/"+runtimeKeySeedName+"/archive?path=/run/aurago-local-llm"),
		bytes.NewReader(archive.Bytes()))
	if err != nil {
		return fmt.Errorf("runtime_key_copy_failed")
	}
	req.Header.Set("Content-Type", "application/x-tar")
	resp, err := m.docker.HTTPClient().Do(req)
	if err != nil {
		return fmt.Errorf("runtime_key_copy_failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("runtime_key_copy_failed")
	}
	return nil
}

func (m *Manager) deleteContainer(ctx context.Context, name string) error {
	_, err := m.docker.DoJSON(ctx, http.MethodDelete, "containers/"+name+"?force=true", nil, nil)
	if err != nil && !strings.Contains(err.Error(), "404") {
		return err
	}
	return nil
}

func (m *Manager) deleteRuntimeKeyVolume(ctx context.Context) error {
	_, err := m.docker.DoJSON(ctx, http.MethodDelete, "volumes/"+runtimeKeyVolumeName+"?force=true", nil, nil)
	if err != nil && !strings.Contains(err.Error(), "404") {
		return err
	}
	return nil
}

var statRenderNode = statFile

func renderNodeGID(path string) string {
	info, err := statRenderNode(path)
	if err != nil {
		return ""
	}
	return info.groupID
}

func (m *Manager) gpuGroupIDs(renderNode string) []string {
	if m.runningInDocker {
		return dockerutil.ParseNumericGroupIDs(os.Getenv("AURAGO_GPU_GROUP_IDS"))
	}
	if gid := renderNodeGID(renderNode); gid != "" && gid != "0" {
		return []string{gid}
	}
	return nil
}

func (m *Manager) pullImage(ctx context.Context, reference string) error {
	path := "images/create?fromImage=" + url.QueryEscape(reference)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, dockerutil.Endpoint(path), nil)
	if err != nil {
		return err
	}
	resp, err := m.docker.HTTPClient().Do(req)
	if err != nil {
		return fmt.Errorf("pull_image_failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("pull_image_failed: Docker returned %d", resp.StatusCode)
	}
	scanner := bufio.NewScanner(io.LimitReader(resp.Body, 64<<20))
	scanner.Buffer(make([]byte, 64<<10), 1<<20)
	for scanner.Scan() {
		var event struct {
			Error string `json:"error"`
		}
		if json.Unmarshal(scanner.Bytes(), &event) == nil && event.Error != "" {
			return fmt.Errorf("pull_image_failed")
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("pull_image_failed: %w", err)
	}
	return nil
}

func (m *Manager) ensureImageAvailable(ctx context.Context, reference string) error {
	var response struct {
		ID string `json:"Id"`
	}
	if _, err := m.docker.DoJSON(ctx, http.MethodGet, "images/"+url.PathEscape(reference)+"/json", nil, &response); err != nil {
		return err
	}
	if response.ID == "" {
		return fmt.Errorf("image_not_installed")
	}
	return nil
}

func (m *Manager) recreateContainer(ctx context.Context, spec dockerContainerSpec) error {
	_, _ = m.docker.DoJSON(ctx, http.MethodPost, "containers/"+managedContainerName+"/stop?t=15", nil, nil)
	_, _ = m.docker.DoJSON(ctx, http.MethodDelete, "containers/"+managedContainerName+"?force=true", nil, nil)
	if err := ensureHostPortsAvailable(ctx, spec.HostConfig.PortBindings); err != nil {
		return err
	}
	if _, err := m.docker.DoJSON(ctx, http.MethodPost, "containers/create?name="+managedContainerName, spec, nil); err != nil {
		return fmt.Errorf("container_create_failed: %w", err)
	}
	if _, err := m.docker.DoJSON(ctx, http.MethodPost, "containers/"+managedContainerName+"/start", nil, nil); err != nil {
		if isDockerPortConflict(err) {
			return fmt.Errorf("listen_port_unavailable")
		}
		return fmt.Errorf("container_start_failed: %w", err)
	}
	return nil
}

func ensureHostPortsAvailable(ctx context.Context, bindings map[string][]dockerPortBind) error {
	var listenConfig net.ListenConfig
	for _, portBindings := range bindings {
		for _, binding := range portBindings {
			hostIP := strings.TrimSpace(binding.HostIP)
			hostPort := strings.TrimSpace(binding.HostPort)
			if hostIP == "" || hostPort == "" {
				continue
			}
			listener, err := listenConfig.Listen(ctx, "tcp", net.JoinHostPort(hostIP, hostPort))
			if err != nil {
				return fmt.Errorf("listen_port_unavailable")
			}
			if err := listener.Close(); err != nil {
				return fmt.Errorf("listen_port_unavailable")
			}
		}
	}
	return nil
}

func isDockerPortConflict(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	for _, marker := range []string{
		"address already in use",
		"failed to bind host port",
		"port is already allocated",
	} {
		if strings.Contains(message, marker) {
			return true
		}
	}
	return false
}
