package embeddings

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
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
)

type dockerAPIClient struct {
	httpClient *http.Client
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
	}
}

func (client *dockerAPIClient) request(
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
		"http://docker/"+dockerutil.APIVersion+endpoint,
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
	apiKey      string
	image       string
	containerID string
	container   string
	baseURL     string
	client      *http.Client
	restarts    int
	closed      bool
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
	if err := embedder.ensureImageLocked(ctx); err != nil {
		return err
	}
	selfID, network, err := embedder.selfContainerNetworkLocked(ctx)
	if err != nil {
		return err
	}
	payload, err := dockerLlamaContainerPayload(
		embedder.image,
		embedder.modelPath,
		embedder.apiKey,
		embedder.backend,
		embedder.contextSize,
		embedder.batchSize,
		selfID,
		network,
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

func (embedder *dockerLlamaEmbedder) selfContainerNetworkLocked(ctx context.Context) (string, string, error) {
	selfID, err := os.Hostname()
	if err != nil || strings.TrimSpace(selfID) == "" {
		return "", "", fmt.Errorf("resolve AuraGo container ID: %w", err)
	}
	data, code, err := embedder.docker.request(
		ctx,
		http.MethodGet,
		"/containers/"+url.PathEscape(selfID)+"/json",
		nil,
	)
	if err != nil {
		return "", "", fmt.Errorf("inspect AuraGo container: %w", err)
	}
	if code != http.StatusOK {
		return "", "", fmt.Errorf("inspect AuraGo container returned HTTP %d", code)
	}
	var inspected struct {
		ID              string `json:"Id"`
		NetworkSettings struct {
			Networks map[string]json.RawMessage `json:"Networks"`
		} `json:"NetworkSettings"`
	}
	if err := json.Unmarshal(data, &inspected); err != nil {
		return "", "", fmt.Errorf("decode AuraGo container inspection: %w", err)
	}
	if inspected.ID != "" {
		selfID = inspected.ID
	}
	var networks []string
	for name := range inspected.NetworkSettings.Networks {
		networks = append(networks, name)
	}
	sort.Strings(networks)
	for _, name := range networks {
		if !strings.Contains(strings.ToLower(name), "docker-control") {
			return selfID, name, nil
		}
	}
	if len(networks) == 0 {
		return "", "", fmt.Errorf("AuraGo container has no Docker network")
	}
	return selfID, networks[0], nil
}

func dockerLlamaContainerPayload(
	image string,
	modelPath string,
	apiKey string,
	backend string,
	contextSize int,
	batchSize int,
	selfID string,
	network string,
) (map[string]any, error) {
	if image == "" || modelPath == "" || apiKey == "" || selfID == "" || network == "" {
		return nil, fmt.Errorf("incomplete llama.cpp sidecar configuration")
	}
	if contextSize <= 0 {
		contextSize = 2048
	}
	if batchSize <= 0 {
		batchSize = 2048
	}
	gpuLayers := "0"
	if backend != "cpu" {
		gpuLayers = "999"
	}
	command := []string{
		"--model", modelPath,
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
	hostConfig := map[string]any{
		"AutoRemove":     false,
		"ReadonlyRootfs": true,
		"Memory":         int64(2 << 30),
		"PidsLimit":      int64(128),
		"CapDrop":        []string{"ALL"},
		"SecurityOpt":    []string{"no-new-privileges"},
		"VolumesFrom":    []string{selfID + ":ro"},
		"NetworkMode":    network,
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
	if embedder.containerID == "" {
		return nil
	}
	ctx, cancel := context.WithTimeout(parent, 15*time.Second)
	defer cancel()
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
		return fmt.Errorf("remove llama.cpp sidecar: %w", err)
	}
	if code != http.StatusNoContent && code != http.StatusNotFound {
		return fmt.Errorf("remove llama.cpp sidecar returned HTTP %d", code)
	}
	return nil
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
