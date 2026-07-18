package embeddings

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	osuser "os/user"
	pathpkg "path"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"aurago/internal/dockerutil"
)

const (
	llamaDockerPort          = 8080
	llamaDockerContainerBase = "aurago-granite-embeddings"
	dockerGPUGroupIDsEnv     = "AURAGO_GPU_GROUP_IDS"
)

type dockerAPIClient struct {
	httpClient *http.Client
	apiVersion string
}

func newDockerAPIClient(host string) *dockerAPIClient {
	normalizedHost := dockerutil.NormalizeHost(host)
	transport := &http.Transport{
		MaxIdleConns:    8,
		IdleConnTimeout: 30 * time.Second,
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return dockerutil.DialContext(ctx, normalizedHost)
		},
	}
	return &dockerAPIClient{
		httpClient: &http.Client{Transport: transport},
		apiVersion: dockerutil.APIVersion,
	}
}

func (client *dockerAPIClient) request(
	ctx context.Context,
	method string,
	endpoint string,
	body []byte,
) ([]byte, int, error) {
	return client.requestURL(ctx, method, "/"+client.apiVersion+endpoint, body)
}

func (client *dockerAPIClient) requestUnversioned(
	ctx context.Context,
	method string,
	endpoint string,
	body []byte,
) ([]byte, int, error) {
	return client.requestURL(ctx, method, endpoint, body)
}

func (client *dockerAPIClient) requestURL(
	ctx context.Context,
	method string,
	endpoint string,
	body []byte,
) ([]byte, int, error) {
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	request, err := http.NewRequestWithContext(
		ctx,
		method,
		"http://docker"+endpoint,
		reader,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("build Docker request: %w", err)
	}
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := client.httpClient.Do(request)
	if err != nil {
		return nil, 0, fmt.Errorf("call Docker API: %w", err)
	}
	defer response.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(response.Body, 16<<20))
	if err != nil {
		return nil, response.StatusCode, fmt.Errorf("read Docker response: %w", err)
	}
	return raw, response.StatusCode, nil
}

type dockerLlamaEmbedder struct {
	mu          sync.Mutex
	docker      *dockerAPIClient
	dockerHost  string
	modelPath   string
	backend     string
	contextSize int
	batchSize   int
	gpuGroupIDs []string
	apiKey      string
	image       string
	containerID string
	container   string
	selfID      string
	networkID   string
	network     string
	baseURL     string
	client      *http.Client
	restarts    int
	closed      bool
}

type dockerModelMount struct {
	Type          string
	Source        string
	Target        string
	VolumeSubpath string
}

type dockerContainerMount struct {
	Type        string `json:"Type"`
	Name        string `json:"Name"`
	Source      string `json:"Source"`
	Destination string `json:"Destination"`
}

func newDockerLlamaEmbedder(
	ctx context.Context,
	dockerHost string,
	modelPath string,
	backend string,
	contextSize int,
	batchSize int,
) (*dockerLlamaEmbedder, error) {
	image, ok := llamaDockerImage(backend)
	if !ok {
		return nil, fmt.Errorf("no pinned llama.cpp Docker image for backend %q", backend)
	}
	var gpuGroupIDs []string
	if backend == "vulkan" {
		var err error
		gpuGroupIDs, err = dockerVulkanGroupIDs()
		if err != nil {
			return nil, fmt.Errorf("resolve Vulkan container groups: %w", err)
		}
	}
	apiKey, err := randomAPIKey()
	if err != nil {
		return nil, err
	}
	suffix := make([]byte, 5)
	if _, err := rand.Read(suffix); err != nil {
		return nil, fmt.Errorf("generate sidecar name: %w", err)
	}
	embedder := &dockerLlamaEmbedder{
		docker:      newDockerAPIClient(dockerHost),
		dockerHost:  dockerHost,
		modelPath:   modelPath,
		backend:     backend,
		contextSize: contextSize,
		batchSize:   batchSize,
		gpuGroupIDs: gpuGroupIDs,
		apiKey:      apiKey,
		image:       image,
		container:   llamaDockerContainerBase + "-" + backend + "-" + hex.EncodeToString(suffix),
		client:      &http.Client{Timeout: 2 * time.Minute},
	}
	if err := embedder.createAndStartLocked(ctx); err != nil {
		_ = embedder.removeLocked(context.Background())
		return nil, err
	}
	return embedder, nil
}

func dockerEmbeddingSidecarsAvailable() bool {
	if strings.TrimSpace(os.Getenv("DOCKER_HOST")) == "" {
		return false
	}
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return true
	}
	return strings.EqualFold(strings.TrimSpace(os.Getenv("AURAGO_EMBEDDINGS_DOCKER_SIDECAR")), "true")
}

func dockerVulkanGroupIDs() ([]string, error) {
	if configured := strings.TrimSpace(os.Getenv(dockerGPUGroupIDsEnv)); configured != "" {
		return normalizeDockerGPUGroupIDs([]string{configured})
	}
	if runtime.GOOS != "linux" {
		return nil, nil
	}
	if _, err := os.Stat("/.dockerenv"); err == nil {
		// The container's /etc/group describes the AuraGo image, not the Docker
		// host. Docker deployments must receive the host GIDs through the
		// installer-generated AURAGO_GPU_GROUP_IDS environment value.
		return nil, nil
	}
	var groupIDs []string
	for _, name := range []string{"render", "video"} {
		group, err := osuser.LookupGroup(name)
		if err == nil {
			groupIDs = append(groupIDs, group.Gid)
		}
	}
	return normalizeDockerGPUGroupIDs(groupIDs)
}

func normalizeDockerGPUGroupIDs(values []string) ([]string, error) {
	seen := make(map[string]struct{}, len(values))
	var normalized []string
	for _, raw := range values {
		for _, value := range strings.Fields(strings.ReplaceAll(raw, ",", " ")) {
			groupID, err := strconv.ParseUint(value, 10, 32)
			if err != nil || groupID == 0 {
				return nil, fmt.Errorf("invalid GPU group ID %q", value)
			}
			canonical := strconv.FormatUint(groupID, 10)
			if _, exists := seen[canonical]; exists {
				continue
			}
			seen[canonical] = struct{}{}
			normalized = append(normalized, canonical)
		}
	}
	return normalized, nil
}

func augmentDockerHardware(ctx context.Context, hardware *hardwareInfo) string {
	if hardware == nil || !dockerEmbeddingSidecarsAvailable() {
		return ""
	}
	client := newDockerAPIClient(os.Getenv("DOCKER_HOST"))
	probeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	raw, code, err := client.request(probeCtx, http.MethodGet, "/info", nil)
	if err != nil || code != http.StatusOK {
		return ""
	}
	var info struct {
		ID              string                     `json:"ID"`
		ServerVersion   string                     `json:"ServerVersion"`
		KernelVersion   string                     `json:"KernelVersion"`
		Driver          string                     `json:"Driver"`
		OperatingSystem string                     `json:"OperatingSystem"`
		Runtimes        map[string]json.RawMessage `json:"Runtimes"`
	}
	if json.Unmarshal(raw, &info) != nil {
		return ""
	}
	var runtimes []string
	for name := range info.Runtimes {
		runtimes = append(runtimes, strings.ToLower(name))
	}
	sort.Strings(runtimes)
	for _, name := range runtimes {
		if name == "nvidia" {
			hardware.NVIDIA = true
			hardware.NVIDIAReason = "Docker host exposes the NVIDIA container runtime"
		}
	}
	if runtimeIsLinuxContainer(info.OperatingSystem) {
		// Docker does not expose a portable inventory API for DRM devices. The
		// isolated Vulkan probe is authoritative and cleanly skips the backend
		// when /dev/dri is absent or unusable on the host.
		hardware.Vulkan = true
		hardware.VulkanReason = "Docker host Vulkan device probe"
	}
	fingerprintInput := strings.Join([]string{
		info.ID,
		info.ServerVersion,
		info.KernelVersion,
		info.Driver,
		info.OperatingSystem,
		strings.Join(runtimes, ","),
	}, "|")
	sum := sha256.Sum256([]byte(fingerprintInput))
	return "docker=" + hex.EncodeToString(sum[:])
}

func runtimeIsLinuxContainer(operatingSystem string) bool {
	if runtime.GOOS != "linux" {
		return false
	}
	return strings.TrimSpace(operatingSystem) != ""
}

func llamaDockerImage(backend string) (string, bool) {
	switch backend {
	case "cpu":
		return llamaDockerCPUImage, true
	case "cuda":
		return llamaDockerCUDAImage, true
	case "vulkan":
		return llamaDockerVulkanImage, true
	default:
		return "", false
	}
}

func removeDockerLlamaImage(ctx context.Context, dockerHost, backend string) error {
	image, ok := llamaDockerImage(backend)
	if !ok {
		return nil
	}
	cleanupCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	_, code, err := newDockerAPIClient(dockerHost).request(
		cleanupCtx,
		http.MethodDelete,
		"/images/"+url.PathEscape(image),
		nil,
	)
	if err != nil {
		return err
	}
	if code != http.StatusOK && code != http.StatusNotFound && code != http.StatusConflict {
		return fmt.Errorf("Docker image removal returned HTTP %d", code)
	}
	return nil
}

func (embedder *dockerLlamaEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, fmt.Errorf("at least one text is required")
	}
	embedder.mu.Lock()
	defer embedder.mu.Unlock()
	if embedder.closed {
		return nil, fmt.Errorf("llama.cpp Docker embedder is closed")
	}
	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		vectors, err := embedder.embedLocked(ctx, texts)
		if err == nil {
			for i := range vectors {
				if err := l2Normalize(vectors[i]); err != nil {
					return nil, fmt.Errorf("normalize Docker llama.cpp vector %d: %w", i, err)
				}
				if err := validateGraniteVector(vectors[i]); err != nil {
					return nil, fmt.Errorf("validate Docker llama.cpp vector %d: %w", i, err)
				}
			}
			return vectors, nil
		}
		lastErr = err
		if embedder.restarts >= 2 {
			break
		}
		embedder.restarts++
		if restartErr := embedder.restartLocked(ctx); restartErr != nil {
			lastErr = fmt.Errorf("%v; restart failed: %w", lastErr, restartErr)
		}
	}
	return nil, fmt.Errorf("managed llama-server failed after restart limit: %w", lastErr)
}

func (embedder *dockerLlamaEmbedder) embedLocked(ctx context.Context, texts []string) ([][]float32, error) {
	payload := struct {
		Model          string   `json:"model"`
		Input          []string `json:"input"`
		EncodingFormat string   `json:"encoding_format"`
	}{
		Model:          GraniteModelID,
		Input:          texts,
		EncodingFormat: "float",
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal Docker llama.cpp request: %w", err)
	}
	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		embedder.baseURL+"/v1/embeddings",
		bytes.NewReader(raw),
	)
	if err != nil {
		return nil, fmt.Errorf("create Docker llama.cpp request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+embedder.apiKey)
	response, err := embedder.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("call managed llama-server: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		rawBody, _ := io.ReadAll(io.LimitReader(response.Body, 16<<10))
		return nil, fmt.Errorf("managed llama-server returned %s: %s", response.Status, strings.TrimSpace(string(rawBody)))
	}
	var decoded struct {
		Data []struct {
			Embedding []float32 `json:"embedding"`
			Index     int       `json:"index"`
		} `json:"data"`
	}
	if err := json.NewDecoder(response.Body).Decode(&decoded); err != nil {
		return nil, fmt.Errorf("decode managed llama-server response: %w", err)
	}
	if len(decoded.Data) != len(texts) {
		return nil, fmt.Errorf("managed llama-server returned %d vectors for %d texts", len(decoded.Data), len(texts))
	}
	sort.Slice(decoded.Data, func(i, j int) bool {
		return decoded.Data[i].Index < decoded.Data[j].Index
	})
	vectors := make([][]float32, len(decoded.Data))
	for i := range decoded.Data {
		vectors[i] = decoded.Data[i].Embedding
	}
	return vectors, nil
}

func (embedder *dockerLlamaEmbedder) createAndStartLocked(ctx context.Context) error {
	engineAPIVersion, err := embedder.negotiateDockerVersionLocked(ctx)
	if err != nil {
		return err
	}
	selfID, modelMount, err := embedder.selfContainerModelMountLocked(ctx, engineAPIVersion)
	if err != nil {
		return err
	}
	if err := embedder.ensureImageLocked(ctx); err != nil {
		return err
	}
	if err := embedder.createPrivateNetworkLocked(ctx, selfID); err != nil {
		return err
	}
	payload, err := dockerLlamaContainerPayload(
		embedder.image,
		embedder.modelPath,
		embedder.apiKey,
		embedder.backend,
		embedder.contextSize,
		embedder.batchSize,
		embedder.gpuGroupIDs,
		modelMount,
		embedder.network,
	)
	if err != nil {
		return err
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal llama.cpp sidecar config: %w", err)
	}
	data, code, err := embedder.docker.request(
		ctx,
		http.MethodPost,
		"/containers/create?name="+url.QueryEscape(embedder.container),
		raw,
	)
	if err != nil {
		return fmt.Errorf("create llama.cpp sidecar: %w", err)
	}
	if code != http.StatusCreated {
		return fmt.Errorf("create llama.cpp sidecar returned HTTP %d: %s", code, dockerErrorMessage(data))
	}
	var created struct {
		ID string `json:"Id"`
	}
	if err := json.Unmarshal(data, &created); err != nil || created.ID == "" {
		return fmt.Errorf("decode llama.cpp sidecar create response")
	}
	embedder.containerID = created.ID
	_, code, err = embedder.docker.request(ctx, http.MethodPost, "/containers/"+url.PathEscape(created.ID)+"/start", nil)
	if err != nil {
		return fmt.Errorf("start llama.cpp sidecar: %w", err)
	}
	if code != http.StatusNoContent && code != http.StatusNotModified {
		return fmt.Errorf("start llama.cpp sidecar returned HTTP %d", code)
	}
	embedder.baseURL = fmt.Sprintf("http://%s:%d", embedder.container, llamaDockerPort)
	if err := embedder.waitForHealthLocked(ctx); err != nil {
		logs := embedder.logsLocked(context.Background())
		return fmt.Errorf("wait for managed llama-server: %w; log: %s", err, logs)
	}
	return nil
}

func (embedder *dockerLlamaEmbedder) ensureImageLocked(ctx context.Context) error {
	_, code, err := embedder.docker.request(
		ctx,
		http.MethodGet,
		"/images/"+url.PathEscape(embedder.image)+"/json",
		nil,
	)
	if err == nil && code == http.StatusOK {
		return nil
	}
	_, code, err = embedder.docker.request(
		ctx,
		http.MethodPost,
		"/images/create?fromImage="+url.QueryEscape(embedder.image),
		nil,
	)
	if err != nil {
		return fmt.Errorf("pull pinned llama.cpp image: %w", err)
	}
	if code != http.StatusOK {
		return fmt.Errorf("pull pinned llama.cpp image returned HTTP %d", code)
	}
	return nil
}

func (embedder *dockerLlamaEmbedder) negotiateDockerVersionLocked(ctx context.Context) (string, error) {
	data, code, err := embedder.docker.requestUnversioned(ctx, http.MethodGet, "/version", nil)
	if err != nil {
		return "", fmt.Errorf("read Docker Engine version: %w", err)
	}
	if code != http.StatusOK {
		return "", fmt.Errorf("read Docker Engine version returned HTTP %d", code)
	}
	var version struct {
		APIVersion string `json:"ApiVersion"`
	}
	if err := json.Unmarshal(data, &version); err != nil || strings.TrimSpace(version.APIVersion) == "" {
		return "", fmt.Errorf("decode Docker Engine API version")
	}
	version.APIVersion = strings.TrimPrefix(strings.TrimSpace(version.APIVersion), "v")
	if compareDockerAPIVersions(version.APIVersion, "1.25") < 0 {
		return "", fmt.Errorf("Docker Engine API %s is too old for the managed embedding sidecar", version.APIVersion)
	}
	if compareDockerAPIVersions(version.APIVersion, strings.TrimPrefix(dockerutil.APIVersion, "v")) < 0 {
		embedder.docker.apiVersion = "v" + version.APIVersion
	}
	return version.APIVersion, nil
}

func (embedder *dockerLlamaEmbedder) selfContainerModelMountLocked(
	ctx context.Context,
	engineAPIVersion string,
) (string, dockerModelMount, error) {
	selfID, err := os.Hostname()
	if err != nil || strings.TrimSpace(selfID) == "" {
		return "", dockerModelMount{}, fmt.Errorf("resolve AuraGo container ID: %w", err)
	}
	data, code, err := embedder.docker.request(
		ctx,
		http.MethodGet,
		"/containers/"+url.PathEscape(selfID)+"/json",
		nil,
	)
	if err != nil {
		return "", dockerModelMount{}, fmt.Errorf("inspect AuraGo container: %w", err)
	}
	if code != http.StatusOK {
		return "", dockerModelMount{}, fmt.Errorf("inspect AuraGo container returned HTTP %d", code)
	}
	var inspected struct {
		ID     string                 `json:"Id"`
		Mounts []dockerContainerMount `json:"Mounts"`
	}
	if err := json.Unmarshal(data, &inspected); err != nil {
		return "", dockerModelMount{}, fmt.Errorf("decode AuraGo container inspection: %w", err)
	}
	if inspected.ID != "" {
		selfID = inspected.ID
	}
	modelDir := pathpkg.Clean(filepath.ToSlash(filepath.Dir(embedder.modelPath)))
	selected, err := selectDockerModelMount(modelDir, engineAPIVersion, inspected.Mounts)
	if err != nil {
		return "", dockerModelMount{}, err
	}
	return selfID, selected, nil
}

func selectDockerModelMount(
	modelDir string,
	engineAPIVersion string,
	mounts []dockerContainerMount,
) (dockerModelMount, error) {
	bestDestination := ""
	var selected dockerModelMount
	for _, mount := range mounts {
		relative, ok := containerPathRelative(mount.Destination, modelDir)
		if !ok || len(pathpkg.Clean(mount.Destination)) < len(bestDestination) {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(mount.Type)) {
		case "bind":
			selected = dockerModelMount{
				Type:   "bind",
				Source: pathpkg.Join(filepath.ToSlash(mount.Source), relative),
				Target: modelDir,
			}
		case "volume":
			source := strings.TrimSpace(mount.Name)
			if source == "" {
				source = strings.TrimSpace(mount.Source)
			}
			if source == "" {
				continue
			}
			if relative != "." && compareDockerAPIVersions(engineAPIVersion, "1.45") < 0 {
				embeddingRoot := pathpkg.Dir(pathpkg.Dir(modelDir))
				if pathpkg.Clean(mount.Destination) != embeddingRoot {
					return dockerModelMount{}, fmt.Errorf(
						"Docker Engine API %s cannot mount the embedding model subdirectory read-only; mount %s as a separate volume or use the ONNX CPU fallback",
						engineAPIVersion,
						embeddingRoot,
					)
				}
				selected = dockerModelMount{
					Type:   "volume",
					Source: source,
					Target: embeddingRoot,
				}
				bestDestination = pathpkg.Clean(mount.Destination)
				continue
			}
			selected = dockerModelMount{
				Type:          "volume",
				Source:        source,
				Target:        modelDir,
				VolumeSubpath: strings.TrimPrefix(relative, "./"),
			}
			if relative == "." {
				selected.VolumeSubpath = ""
			}
		default:
			continue
		}
		bestDestination = pathpkg.Clean(mount.Destination)
	}
	if selected.Source == "" {
		return dockerModelMount{}, fmt.Errorf(
			"the local embedding model directory %s is not backed by a safe Docker bind or volume mount; using the native CPU fallback",
			modelDir,
		)
	}
	return selected, nil
}

func (embedder *dockerLlamaEmbedder) createPrivateNetworkLocked(ctx context.Context, selfID string) error {
	networkName := embedder.container + "-internal"
	payload, err := json.Marshal(dockerPrivateNetworkPayload(networkName))
	if err != nil {
		return fmt.Errorf("marshal embedding sidecar network: %w", err)
	}
	data, code, err := embedder.docker.request(ctx, http.MethodPost, "/networks/create", payload)
	if err != nil {
		return fmt.Errorf("create embedding sidecar network: %w", err)
	}
	if code != http.StatusCreated {
		return fmt.Errorf("create embedding sidecar network returned HTTP %d: %s", code, dockerErrorMessage(data))
	}
	var created struct {
		ID string `json:"Id"`
	}
	if err := json.Unmarshal(data, &created); err != nil || created.ID == "" {
		return fmt.Errorf("decode embedding sidecar network response")
	}
	embedder.selfID = selfID
	embedder.networkID = created.ID
	embedder.network = networkName
	connectPayload, err := json.Marshal(map[string]any{"Container": selfID})
	if err != nil {
		return fmt.Errorf("marshal AuraGo network connection: %w", err)
	}
	data, code, err = embedder.docker.request(
		ctx,
		http.MethodPost,
		"/networks/"+url.PathEscape(created.ID)+"/connect",
		connectPayload,
	)
	if err != nil {
		return fmt.Errorf("connect AuraGo to embedding sidecar network: %w", err)
	}
	if code != http.StatusOK {
		return fmt.Errorf("connect AuraGo to embedding sidecar network returned HTTP %d: %s", code, dockerErrorMessage(data))
	}
	return nil
}

func dockerPrivateNetworkPayload(networkName string) map[string]any {
	return map[string]any{
		"Name":           networkName,
		"CheckDuplicate": false,
		"Internal":       true,
		"Attachable":     false,
		"Labels": map[string]string{
			"aurago.managed":   "true",
			"aurago.component": "local-granite-embeddings",
		},
	}
}

func dockerLlamaContainerPayload(
	image string,
	modelPath string,
	apiKey string,
	backend string,
	contextSize int,
	batchSize int,
	gpuGroupIDs []string,
	modelMount dockerModelMount,
	network string,
) (map[string]any, error) {
	if image == "" || modelPath == "" || apiKey == "" ||
		modelMount.Type == "" || modelMount.Source == "" || modelMount.Target == "" ||
		network == "" {
		return nil, fmt.Errorf("incomplete llama.cpp sidecar configuration")
	}
	if contextSize <= 0 {
		contextSize = 2048
	}
	if batchSize <= 0 {
		batchSize = 2048
	}
	var normalizedGPUGroupIDs []string
	if backend == "vulkan" {
		var err error
		normalizedGPUGroupIDs, err = normalizeDockerGPUGroupIDs(gpuGroupIDs)
		if err != nil {
			return nil, fmt.Errorf("validate Vulkan container groups: %w", err)
		}
	}
	gpuLayers := "0"
	if backend != "cpu" {
		gpuLayers = "999"
	}
	containerModelPath := filepath.ToSlash(modelPath)
	command := []string{
		"--model", containerModelPath,
		"--alias", GraniteModelID,
		"--host", "0.0.0.0",
		"--port", strconv.Itoa(llamaDockerPort),
		"--api-key", apiKey,
		"--embedding",
		"--pooling", "cls",
		"--embd-normalize", "2",
		"--ctx-size", strconv.Itoa(contextSize),
		"--batch-size", strconv.Itoa(batchSize),
		"--ubatch-size", strconv.Itoa(batchSize),
		"--parallel", "1",
		"--n-gpu-layers", gpuLayers,
	}
	mount := map[string]any{
		"Type":     modelMount.Type,
		"Source":   modelMount.Source,
		"Target":   modelMount.Target,
		"ReadOnly": true,
	}
	if modelMount.Type == "volume" && modelMount.VolumeSubpath != "" {
		mount["VolumeOptions"] = map[string]any{"Subpath": modelMount.VolumeSubpath}
	}
	hostConfig := map[string]any{
		"AutoRemove":     false,
		"ReadonlyRootfs": true,
		"Memory":         int64(2 << 30),
		"PidsLimit":      int64(128),
		"CapDrop":        []string{"ALL"},
		"SecurityOpt":    []string{"no-new-privileges"},
		"NetworkMode":    network,
		"Mounts":         []map[string]any{mount},
		"Tmpfs": map[string]string{
			"/tmp": "rw,noexec,nosuid,size=64m",
		},
		"LogConfig": map[string]any{
			"Type": "json-file",
			"Config": map[string]string{
				"max-size": "1m",
				"max-file": "2",
			},
		},
	}
	switch backend {
	case "cuda":
		hostConfig["DeviceRequests"] = []map[string]any{{
			"Driver":       "nvidia",
			"Count":        -1,
			"Capabilities": [][]string{{"gpu"}},
		}}
	case "vulkan":
		hostConfig["Devices"] = []map[string]string{{
			"PathOnHost":        "/dev/dri",
			"PathInContainer":   "/dev/dri",
			"CgroupPermissions": "rwm",
		}}
		if len(normalizedGPUGroupIDs) > 0 {
			hostConfig["GroupAdd"] = normalizedGPUGroupIDs
		}
	}
	return map[string]any{
		"Image": image,
		"Cmd":   command,
		"Labels": map[string]string{
			"aurago.managed":   "true",
			"aurago.component": "local-granite-embeddings",
			"aurago.runtime":   llamaRuntimeBuild,
		},
		"ExposedPorts": map[string]any{
			strconv.Itoa(llamaDockerPort) + "/tcp": map[string]any{},
		},
		"HostConfig": hostConfig,
		"NetworkingConfig": map[string]any{
			"EndpointsConfig": map[string]any{
				network: map[string]any{},
			},
		},
	}, nil
}

func (embedder *dockerLlamaEmbedder) waitForHealthLocked(parent context.Context) error {
	ctx, cancel := context.WithTimeout(parent, 2*time.Minute)
	defer cancel()
	ticker := time.NewTicker(300 * time.Millisecond)
	defer ticker.Stop()
	for {
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, embedder.baseURL+"/health", nil)
		if err != nil {
			return err
		}
		response, err := embedder.client.Do(request)
		if err == nil {
			response.Body.Close()
			if response.StatusCode == http.StatusOK {
				return nil
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func (embedder *dockerLlamaEmbedder) restartLocked(ctx context.Context) error {
	if embedder.containerID == "" {
		return fmt.Errorf("managed llama-server container is missing")
	}
	_, code, err := embedder.docker.request(
		ctx,
		http.MethodPost,
		"/containers/"+url.PathEscape(embedder.containerID)+"/restart?t=5",
		nil,
	)
	if err != nil {
		return err
	}
	if code != http.StatusNoContent {
		return fmt.Errorf("Docker restart returned HTTP %d", code)
	}
	return embedder.waitForHealthLocked(ctx)
}

func (embedder *dockerLlamaEmbedder) logsLocked(ctx context.Context) string {
	if embedder.containerID == "" {
		return ""
	}
	logCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	raw, code, err := embedder.docker.request(
		logCtx,
		http.MethodGet,
		"/containers/"+url.PathEscape(embedder.containerID)+"/logs?stdout=1&stderr=1&tail=200",
		nil,
	)
	if err != nil || code != http.StatusOK {
		return ""
	}
	if len(raw) > 64<<10 {
		raw = raw[len(raw)-(64<<10):]
	}
	return string(raw)
}

func (embedder *dockerLlamaEmbedder) Dimensions() int {
	return GraniteDimensions
}

func (embedder *dockerLlamaEmbedder) ModelID() string {
	return GraniteModelID
}

func (embedder *dockerLlamaEmbedder) Fingerprint() string {
	return ggufFingerprint()
}

func (embedder *dockerLlamaEmbedder) Status() Status {
	embedder.mu.Lock()
	defer embedder.mu.Unlock()
	state := "ready"
	if embedder.closed {
		state = "closed"
	}
	logs := embedder.logsLocked(context.Background())
	verified := embedder.backend != "cpu" && llamaGPUVerified(embedder.backend, logs)
	return Status{
		State:        state,
		Provider:     LocalGraniteProvider,
		ModelID:      GraniteModelID,
		Dimensions:   GraniteDimensions,
		Backend:      embedder.backend,
		Runtime:      "llama.cpp-docker",
		RuntimeBuild: llamaRuntimeBuild,
		GPU:          embedder.backend != "cpu",
		GPUVerified:  verified,
		Fingerprint:  ggufFingerprint(),
		UpdatedAt:    time.Now().UTC(),
	}
}

func (embedder *dockerLlamaEmbedder) Close() error {
	embedder.mu.Lock()
	defer embedder.mu.Unlock()
	if embedder.closed {
		return nil
	}
	embedder.closed = true
	return embedder.removeLocked(context.Background())
}

func (embedder *dockerLlamaEmbedder) removeLocked(parent context.Context) error {
	ctx, cancel := context.WithTimeout(parent, 15*time.Second)
	defer cancel()
	var cleanupErrors []error
	if embedder.containerID != "" {
		_, _, _ = embedder.docker.request(
			ctx,
			http.MethodPost,
			"/containers/"+url.PathEscape(embedder.containerID)+"/stop?t=5",
			nil,
		)
		_, code, err := embedder.docker.request(
			ctx,
			http.MethodDelete,
			"/containers/"+url.PathEscape(embedder.containerID)+"?force=true&v=true",
			nil,
		)
		embedder.containerID = ""
		if err != nil {
			cleanupErrors = append(cleanupErrors, fmt.Errorf("remove llama.cpp sidecar: %w", err))
		} else if code != http.StatusNoContent && code != http.StatusNotFound {
			cleanupErrors = append(cleanupErrors, fmt.Errorf("remove llama.cpp sidecar returned HTTP %d", code))
		}
	}
	if embedder.networkID != "" {
		if embedder.selfID != "" {
			disconnectPayload, _ := json.Marshal(map[string]any{
				"Container": embedder.selfID,
				"Force":     true,
			})
			_, code, err := embedder.docker.request(
				ctx,
				http.MethodPost,
				"/networks/"+url.PathEscape(embedder.networkID)+"/disconnect",
				disconnectPayload,
			)
			if err != nil {
				cleanupErrors = append(cleanupErrors, fmt.Errorf("disconnect AuraGo from embedding sidecar network: %w", err))
			} else if code != http.StatusOK && code != http.StatusNotFound {
				cleanupErrors = append(cleanupErrors, fmt.Errorf("disconnect AuraGo from embedding sidecar network returned HTTP %d", code))
			}
		}
		_, code, err := embedder.docker.request(
			ctx,
			http.MethodDelete,
			"/networks/"+url.PathEscape(embedder.networkID),
			nil,
		)
		if err != nil {
			cleanupErrors = append(cleanupErrors, fmt.Errorf("remove embedding sidecar network: %w", err))
		} else if code != http.StatusNoContent && code != http.StatusNotFound {
			cleanupErrors = append(cleanupErrors, fmt.Errorf("remove embedding sidecar network returned HTTP %d", code))
		}
		embedder.networkID = ""
		embedder.network = ""
		embedder.selfID = ""
	}
	return errors.Join(cleanupErrors...)
}

func containerPathRelative(parent, child string) (string, bool) {
	parent = pathpkg.Clean(filepath.ToSlash(parent))
	child = pathpkg.Clean(filepath.ToSlash(child))
	if parent == "." || child == "." || !strings.HasPrefix(parent, "/") || !strings.HasPrefix(child, "/") {
		return "", false
	}
	if child == parent {
		return ".", true
	}
	prefix := strings.TrimSuffix(parent, "/") + "/"
	if !strings.HasPrefix(child, prefix) {
		return "", false
	}
	return strings.TrimPrefix(child, prefix), true
}

func compareDockerAPIVersions(left, right string) int {
	parse := func(value string) (int, int) {
		parts := strings.SplitN(strings.TrimPrefix(strings.TrimSpace(value), "v"), ".", 3)
		if len(parts) < 2 {
			return 0, 0
		}
		major, _ := strconv.Atoi(parts[0])
		minor, _ := strconv.Atoi(parts[1])
		return major, minor
	}
	leftMajor, leftMinor := parse(left)
	rightMajor, rightMinor := parse(right)
	if leftMajor != rightMajor {
		if leftMajor < rightMajor {
			return -1
		}
		return 1
	}
	if leftMinor < rightMinor {
		return -1
	}
	if leftMinor > rightMinor {
		return 1
	}
	return 0
}

func dockerErrorMessage(raw []byte) string {
	var decoded struct {
		Message string `json:"message"`
	}
	if json.Unmarshal(raw, &decoded) == nil && decoded.Message != "" {
		return decoded.Message
	}
	return strings.TrimSpace(string(raw))
}
