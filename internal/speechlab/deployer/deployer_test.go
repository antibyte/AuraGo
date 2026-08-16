package deployer

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/dockerutil"
)

func TestNetworkAliasesKeepWebProxyCompatibility(t *testing.T) {
	got := networkAliases("gateway")
	want := []string{"s2s", "s2s-vulkan", "gateway"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("gateway aliases = %#v, want %#v", got, want)
	}
}

func validManifest() BundleManifest {
	const digest = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	return BundleManifest{
		SchemaVersion: 1, BundleVersion: "stable", ContractVersion: "speech-lab/v1",
		Publisher: "ghcr.io/antibyte", Network: "aurago-speech-lab",
		Images: ImageSet{
			Gateway:   "ghcr.io/antibyte/s2s-vulkan@sha256:" + digest,
			ASR:       "ghcr.io/antibyte/s2s-whisper-fw@sha256:" + digest,
			TTS:       "ghcr.io/antibyte/s2s-vulkan@sha256:" + digest,
			LLM:       "ghcr.io/antibyte/s2s-llama-granite@sha256:" + digest,
			Web:       "ghcr.io/antibyte/s2s-web@sha256:" + digest,
			ModelInit: "ghcr.io/antibyte/s2s-model-init@sha256:" + digest,
		},
		StartOrder: []string{"model_init", "asr", "llm", "tts", "gateway", "web"},
		Services:   []BundleService{{Role: "gateway", Image: "gateway"}},
	}
}

func validV2Manifest() BundleManifest {
	manifest := validManifest()
	manifest.SchemaVersion = 2
	manifest.ContractVersion = "speech-lab/v2"
	manifest.Images.Controller = manifest.Images.Gateway
	manifest.Volumes = []string{"models", "data", "control"}
	manifest.StartOrder = []string{"control_init", "controller", "gateway"}
	manifest.Services = []BundleService{
		{Role: "control_init", Image: "controller", InternalOnly: true, Environment: []string{"S2S_CONTROLLER_INIT_TOKEN_ONLY=true", "S2S_CONTROLLER_TOKEN_FILE=/control/token"}},
		{Role: "controller", Image: "controller", InternalOnly: true, DockerSocket: true},
		{Role: "gateway", Image: "gateway"},
	}
	manifest.Runtimes = []BundleRuntime{{
		BackendID: "parakeet", VariantID: "parakeet-cpu", Stage: "asr",
		Container: "s2s-parakeet-cpu", Image: manifest.Images.ASR, ImageKey: "asr",
		ImageDownloadSizeBytes: 1, Architectures: []string{runtime.GOARCH}, Accelerator: "cpu",
		Healthcheck: map[string]any{"path": "/health", "port": 8082},
		Volumes:     []map[string]any{{"name": "models"}}, Network: map[string]any{"name": manifest.Network},
		Resources: map[string]any{"memory_bytes": 1},
	}}
	return manifest
}

func TestFetchManifestV2RequiresValidEd25519Signature(t *testing.T) {
	manifest := validV2Manifest()
	raw, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signature := ed25519.Sign(privateKey, raw)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/manifest":
			_, _ = w.Write(raw)
		case "/manifest.sig":
			_, _ = w.Write(signature)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	manager := NewManager(config.SpeechLabConfig{}, false, true, false, "", nil,
		WithManifestURL(server.URL+"/manifest"), WithHTTPClient(server.Client()), WithManifestPublicKey(publicKey))
	got, _, err := manager.fetchManifest(context.Background())
	if err != nil {
		t.Fatalf("valid signed bundle rejected: %v", err)
	}
	if got.SchemaVersion != 2 {
		t.Fatalf("schema = %d", got.SchemaVersion)
	}
	signature[0] ^= 0xff
	if _, _, err := manager.fetchManifest(context.Background()); Code(err) != "speech_lab_bundle_signature_invalid" {
		t.Fatalf("tampered signature error = %v", err)
	}
}

func TestValidateManifestV2RejectsSocketAndRuntimeDrift(t *testing.T) {
	manifest := validV2Manifest()
	if err := validateManifest(manifest); err != nil {
		t.Fatalf("valid v2 manifest rejected: %v", err)
	}
	manifest.Services[2].DockerSocket = true
	if err := validateManifest(manifest); err == nil {
		t.Fatal("gateway Docker socket accepted")
	}
	manifest = validV2Manifest()
	manifest.Services[0].InternalOnly = false
	if err := validateManifest(manifest); err == nil {
		t.Fatal("externally reachable control initializer accepted")
	}
	manifest = validV2Manifest()
	manifest.Runtimes[0].Architectures = []string{"unsupported"}
	if err := validateManifest(manifest); err == nil {
		t.Fatal("unsupported runtime architecture accepted")
	}
	manifest = validV2Manifest()
	manifest.Runtimes[0].ImageKey = "gateway"
	if err := validateManifest(manifest); err == nil {
		t.Fatal("runtime image-key drift accepted")
	}
}

func TestValidateManifestRequiresAllowlistedFullDigests(t *testing.T) {
	if err := validateManifest(validManifest()); err != nil {
		t.Fatalf("valid manifest rejected: %v", err)
	}
	tests := []struct {
		name string
		edit func(*BundleManifest)
	}{
		{"wrong publisher", func(m *BundleManifest) { m.Publisher = "ghcr.io/other" }},
		{"mutable image", func(m *BundleManifest) { m.Images.Gateway = "ghcr.io/antibyte/s2s-vulkan:latest" }},
		{"short digest", func(m *BundleManifest) { m.Images.Gateway = "ghcr.io/antibyte/s2s-vulkan@sha256:abcd" }},
		{"wrong contract", func(m *BundleManifest) { m.ContractVersion = "speech-lab/v2" }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manifest := validManifest()
			tt.edit(&manifest)
			if err := validateManifest(manifest); err == nil {
				t.Fatal("invalid manifest accepted")
			}
		})
	}
}

func TestRestartPolicyIsBounded(t *testing.T) {
	if got := restartPolicy("model_init", "always"); got != "no" {
		t.Fatalf("model_init restart = %q, want no", got)
	}
	if got := restartPolicy("gateway", ""); got != "unless-stopped" {
		t.Fatalf("default restart = %q, want unless-stopped", got)
	}
	if got := restartPolicy("gateway", "evil"); got != "unless-stopped" {
		t.Fatalf("invalid restart = %q, want unless-stopped", got)
	}
}

type fakeContainer struct {
	ID          string
	Name        string
	Image       string
	Labels      map[string]string
	Running     bool
	Attached    bool
	RestartName string
}

type fakeResource struct {
	ID     string
	Name   string
	Labels map[string]string
}

type fakeDocker struct {
	mu                sync.Mutex
	server            *httptest.Server
	manifest          []byte
	manifestSignature []byte
	manifestPublicKey ed25519.PublicKey
	ready             bool
	pullError         string
	pullStarted       chan struct{}
	releasePull       chan struct{}
	startNotModified  bool
	readyOnRollback   bool
	nextID            int
	containers        map[string]*fakeContainer
	networks          map[string]*fakeResource
	volumes           map[string]*fakeResource
	pulls             int
	pullImages        []string
	creates           int
	createPayloads    []map[string]any
}

func newFakeDocker(t *testing.T, manifest BundleManifest) *fakeDocker {
	t.Helper()
	raw, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	fake := &fakeDocker{
		manifest: raw, ready: true, containers: make(map[string]*fakeContainer),
		networks: map[string]*fakeResource{
			"aurago-speech-lab": {ID: "network-id", Name: "aurago-speech-lab", Labels: map[string]string{"aurago.managed": "speech-lab"}},
		},
		volumes: make(map[string]*fakeResource), nextID: 1,
	}
	if manifest.SchemaVersion == 2 {
		publicKey, privateKey, keyErr := ed25519.GenerateKey(rand.Reader)
		if keyErr != nil {
			t.Fatal(keyErr)
		}
		fake.manifestPublicKey = publicKey
		fake.manifestSignature = ed25519.Sign(privateKey, raw)
	}
	fake.server = httptest.NewServer(http.HandlerFunc(fake.serveHTTP))
	t.Cleanup(fake.server.Close)
	return fake
}

func (f *fakeDocker) addContainer(container *fakeContainer) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.containers[container.ID] = container
}

func (f *fakeDocker) find(value string) *fakeContainer {
	value, _ = url.PathUnescape(value)
	for _, container := range f.containers {
		if container.ID == value || container.Name == value {
			return container
		}
	}
	return nil
}

func (f *fakeDocker) serveHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if r.URL.Path == "/manifest" {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(f.manifest)
		return
	}
	if r.URL.Path == "/manifest.sig" && len(f.manifestSignature) > 0 {
		_, _ = w.Write(f.manifestSignature)
		return
	}
	if r.URL.Path == "/ready" {
		if !f.ready {
			http.Error(w, "not ready", http.StatusServiceUnavailable)
			return
		}
		_, _ = io.WriteString(w, `{"ready":true}`)
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/"+dockerutil.APIVersion)
	switch {
	case r.Method == http.MethodGet && path == "/containers/json":
		rows := make([]map[string]any, 0, len(f.containers))
		for _, container := range f.containers {
			rows = append(rows, map[string]any{
				"Id": container.ID, "Names": []string{"/" + container.Name}, "Image": container.Image,
				"State": map[bool]string{true: "running", false: "exited"}[container.Running], "Labels": container.Labels,
			})
		}
		_ = json.NewEncoder(w).Encode(rows)
		return
	case r.Method == http.MethodPost && path == "/images/create":
		f.pulls++
		f.pullImages = append(f.pullImages, r.URL.Query().Get("fromImage"))
		if f.pullStarted != nil && f.pulls == 1 {
			close(f.pullStarted)
			<-f.releasePull
		}
		if f.pullError != "" {
			_, _ = fmt.Fprintf(w, "{\"errorDetail\":{\"message\":%q}}\n", f.pullError)
			return
		}
		_, _ = io.WriteString(w, "{\"status\":\"done\"}\n")
		return
	case r.Method == http.MethodPost && path == "/networks/create":
		var body struct {
			Name   string
			Labels map[string]string
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		resource := &fakeResource{ID: fmt.Sprintf("network-%d", f.nextID), Name: body.Name, Labels: body.Labels}
		f.nextID++
		f.networks[body.Name] = resource
		w.WriteHeader(http.StatusCreated)
		_, _ = fmt.Fprintf(w, `{"Id":%q}`, resource.ID)
		return
	case r.Method == http.MethodGet && strings.HasPrefix(path, "/networks/"):
		name, _ := url.PathUnescape(strings.TrimPrefix(path, "/networks/"))
		var resource *fakeResource
		for _, candidate := range f.networks {
			if candidate.Name == name || candidate.ID == name {
				resource = candidate
				break
			}
		}
		if resource == nil {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		containers := make(map[string]any)
		for _, container := range f.containers {
			if container.Attached {
				containers[container.ID] = map[string]any{}
			}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"Id": resource.ID, "Name": resource.Name, "Labels": resource.Labels, "Containers": containers})
		return
	case r.Method == http.MethodPost && strings.HasSuffix(path, "/disconnect"):
		var body struct{ Container string }
		_ = json.NewDecoder(r.Body).Decode(&body)
		if container := f.find(body.Container); container != nil {
			container.Attached = false
		}
		w.WriteHeader(http.StatusOK)
		return
	case r.Method == http.MethodPost && strings.HasSuffix(path, "/connect"):
		var body struct{ Container string }
		_ = json.NewDecoder(r.Body).Decode(&body)
		if container := f.find(body.Container); container != nil {
			container.Attached = true
		}
		w.WriteHeader(http.StatusOK)
		return
	case r.Method == http.MethodPost && path == "/volumes/create":
		var body struct {
			Name   string
			Labels map[string]string
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		f.volumes[body.Name] = &fakeResource{Name: body.Name, Labels: body.Labels}
		w.WriteHeader(http.StatusCreated)
		_, _ = io.WriteString(w, `{}`)
		return
	case r.Method == http.MethodGet && strings.HasPrefix(path, "/volumes/"):
		name, _ := url.PathUnescape(strings.TrimPrefix(path, "/volumes/"))
		resource := f.volumes[name]
		if resource == nil {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"Name": resource.Name, "Labels": resource.Labels})
		return
	case r.Method == http.MethodDelete && strings.HasPrefix(path, "/volumes/"):
		name, _ := url.PathUnescape(strings.TrimPrefix(path, "/volumes/"))
		delete(f.volumes, name)
		w.WriteHeader(http.StatusNoContent)
		return
	case r.Method == http.MethodPost && path == "/containers/create":
		var body struct {
			Image      string            `json:"Image"`
			Labels     map[string]string `json:"Labels"`
			HostConfig struct {
				RestartPolicy struct{ Name string } `json:"RestartPolicy"`
			} `json:"HostConfig"`
		}
		payload, _ := io.ReadAll(r.Body)
		var raw map[string]any
		_ = json.Unmarshal(payload, &raw)
		f.createPayloads = append(f.createPayloads, raw)
		_ = json.Unmarshal(payload, &body)
		id := fmt.Sprintf("new-%d", f.nextID)
		f.nextID++
		name := r.URL.Query().Get("name")
		f.containers[id] = &fakeContainer{ID: id, Name: name, Image: body.Image, Labels: body.Labels, Attached: true, RestartName: body.HostConfig.RestartPolicy.Name}
		f.creates++
		w.WriteHeader(http.StatusCreated)
		_, _ = fmt.Fprintf(w, `{"Id":%q}`, id)
		return
	case strings.HasPrefix(path, "/containers/"):
		f.serveContainer(w, r, path)
		return
	case r.Method == http.MethodDelete && strings.HasPrefix(path, "/networks/"):
		name, _ := url.PathUnescape(strings.TrimPrefix(path, "/networks/"))
		for key, resource := range f.networks {
			if resource.Name == name || resource.ID == name {
				delete(f.networks, key)
			}
		}
		w.WriteHeader(http.StatusNoContent)
		return
	default:
		http.Error(w, "not found", http.StatusNotFound)
	}
}

func (f *fakeDocker) serveContainer(w http.ResponseWriter, r *http.Request, path string) {
	tail := strings.TrimPrefix(path, "/containers/")
	id := strings.SplitN(tail, "/", 2)[0]
	container := f.find(id)
	if container == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	switch {
	case r.Method == http.MethodGet && strings.HasSuffix(path, "/json"):
		networks := map[string]any{}
		if container.Attached {
			networks["aurago-speech-lab"] = map[string]any{}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"Id": container.ID, "Name": "/" + container.Name, "Config": map[string]any{"Image": container.Image, "Labels": container.Labels}, "State": map[string]any{"Running": container.Running}, "NetworkSettings": map[string]any{"Networks": networks}})
	case r.Method == http.MethodPost && strings.HasSuffix(path, "/start"):
		container.Running = true
		if f.startNotModified {
			w.WriteHeader(http.StatusNotModified)
		} else {
			w.WriteHeader(http.StatusNoContent)
		}
	case r.Method == http.MethodPost && strings.HasSuffix(path, "/stop"):
		container.Running = false
		w.WriteHeader(http.StatusNotModified)
	case r.Method == http.MethodPost && strings.HasSuffix(path, "/rename"):
		container.Name = r.URL.Query().Get("name")
		if f.readyOnRollback && !strings.Contains(container.Name, "-rollback-") {
			f.ready = true
		}
		w.WriteHeader(http.StatusNoContent)
	case r.Method == http.MethodDelete:
		delete(f.containers, container.ID)
		w.WriteHeader(http.StatusNoContent)
	default:
		http.Error(w, "not found", http.StatusNotFound)
	}
}

func (f *fakeDocker) manager(t *testing.T, cfg config.SpeechLabConfig, dataDir string) *Manager {
	t.Helper()
	host := "tcp://" + f.server.Listener.Addr().String()
	options := []Option{
		WithManifestURL(f.server.URL + "/manifest"), WithHTTPClient(f.server.Client()),
		WithDockerClient(dockerutil.NewClient(host, time.Second)), WithReadinessTimeout(40 * time.Millisecond),
	}
	if len(f.manifestPublicKey) > 0 {
		options = append(options, WithManifestPublicKey(f.manifestPublicKey))
	}
	return NewManager(cfg, false, true, false, dataDir, nil, options...)
}

func managedSpeechLabConfig(baseURL string) config.SpeechLabConfig {
	return config.SpeechLabConfig{Enabled: true, BaseURL: baseURL, Deployment: config.SpeechLabDeploymentConfig{Mode: "managed", Bundle: "stable", GPUBackend: config.SpeechLabGPUBackendAuto, AutoStart: true}}
}

func manifestDigest(t *testing.T, manifest BundleManifest) string {
	t.Helper()
	raw, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func TestUpdateIdenticalDeploymentIsNoOpAndAccepts304(t *testing.T) {
	manifest := validManifest()
	fake := newFakeDocker(t, manifest)
	fake.startNotModified = true
	fingerprint := speechLabDeploymentFingerprint(manifestDigest(t, manifest), config.SpeechLabGPUBackendAuto)
	fake.addContainer(&fakeContainer{ID: "old", Name: "aurago-speech-lab-gateway", Image: manifest.Images.Gateway, Labels: dockerutil.ManagedLabels(OwnerLabel, "speech-lab", "gateway", fingerprint), Running: true, Attached: true})
	manager := fake.manager(t, managedSpeechLabConfig(fake.server.URL), "")
	if err := manager.Update(context.Background()); err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if fake.pulls != 0 || fake.creates != 0 {
		t.Fatalf("identical update pulled=%d created=%d, want zero", fake.pulls, fake.creates)
	}
	if state := manager.Status(); state.State != "ready" || !reflect.DeepEqual(state.ContainerIDs, []string{"old"}) {
		t.Fatalf("unexpected state: %#v", state)
	}
}

func TestOverlaySpeechLabGPUEnvironment(t *testing.T) {
	base := []string{"KEEP=1", "S2S_GPU=stale", "GGML_BACKEND=CPU", "NO_EQUALS"}
	for _, tc := range []struct {
		backend string
		want    []string
	}{
		{config.SpeechLabGPUBackendAuto, []string{"KEEP=1", "NO_EQUALS", "S2S_GPU=auto"}},
		{config.SpeechLabGPUBackendVulkan, []string{"KEEP=1", "NO_EQUALS", "S2S_GPU=vulkan"}},
		{config.SpeechLabGPUBackendCPU, []string{"KEEP=1", "NO_EQUALS", "S2S_GPU=cpu", "GGML_BACKEND=CPU"}},
	} {
		t.Run(tc.backend, func(t *testing.T) {
			got, err := overlaySpeechLabGPUEnvironment(base, tc.backend)
			if err != nil {
				t.Fatalf("overlay error: %v", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("overlay = %#v, want %#v", got, tc.want)
			}
		})
	}
	if _, err := overlaySpeechLabGPUEnvironment(nil, "amd"); err == nil {
		t.Fatal("unsupported GPU backend was accepted by the environment overlay")
	}
}

func TestSpeechLabDeploymentFingerprintIncludesHardwareProfile(t *testing.T) {
	digest := strings.Repeat("a", 64)
	auto := speechLabDeploymentFingerprint(digest, config.SpeechLabGPUBackendAuto)
	if auto != speechLabDeploymentFingerprint(digest, " AUTO ") {
		t.Fatal("GPU backend normalization changed the stable fingerprint")
	}
	for _, backend := range []string{config.SpeechLabGPUBackendVulkan, config.SpeechLabGPUBackendCPU} {
		if auto == speechLabDeploymentFingerprint(digest, backend) {
			t.Fatalf("fingerprint did not change for %s", backend)
		}
	}
}

func TestSpeechLabActiveGPUBackendReportsAutoCPUFallback(t *testing.T) {
	if got := speechLabActiveGPUBackend(config.SpeechLabGPUBackendAuto, nil); got != config.SpeechLabGPUBackendCPU {
		t.Fatalf("auto without GPU host config = %q, want cpu", got)
	}
	if got := speechLabActiveGPUBackend(config.SpeechLabGPUBackendAuto, map[string]any{"Devices": true}); got != config.SpeechLabGPUBackendAuto {
		t.Fatalf("auto with GPU host config = %q, want auto", got)
	}
	if got := speechLabActiveGPUBackend(config.SpeechLabGPUBackendVulkan, map[string]any{}); got != config.SpeechLabGPUBackendVulkan {
		t.Fatalf("vulkan active profile = %q", got)
	}
}

func TestSpeechLabVulkanHostConfigUsesDriAndValidatedGroups(t *testing.T) {
	hostConfig := speechLabVulkanHostConfig([]string{"44", "109"})
	devices, ok := hostConfig["Devices"].([]map[string]string)
	if !ok || len(devices) != 1 {
		t.Fatalf("unexpected Vulkan devices: %#v", hostConfig["Devices"])
	}
	if devices[0]["PathOnHost"] != "/dev/dri" || devices[0]["PathInContainer"] != "/dev/dri" || devices[0]["CgroupPermissions"] != "rwm" {
		t.Fatalf("unexpected Vulkan device mapping: %#v", devices[0])
	}
	if got := hostConfig["GroupAdd"].([]string); !reflect.DeepEqual(got, []string{"44", "109"}) {
		t.Fatalf("group_add = %#v", got)
	}
	if _, err := normalizeSpeechLabGPUGroupIDs([]string{"44", "not-a-number"}); err == nil {
		t.Fatal("non-numeric GPU group ID was accepted")
	}
}

func TestManagedDeploymentAppliesGPUEnvironmentOverlay(t *testing.T) {
	manifest := validManifest()
	manifest.Services[0].Environment = []string{"KEEP=1", "S2S_GPU=stale", "GGML_BACKEND=CPU"}
	fake := newFakeDocker(t, manifest)
	manager := fake.manager(t, managedSpeechLabConfig(fake.server.URL), "")
	if err := manager.Install(context.Background()); err != nil {
		t.Fatalf("Install() error = %v", err)
	}
	if len(fake.createPayloads) != 1 {
		t.Fatalf("captured %d create payloads, want one", len(fake.createPayloads))
	}
	environment, ok := fake.createPayloads[0]["Env"].([]any)
	if !ok {
		t.Fatalf("create payload has unexpected Env: %#v", fake.createPayloads[0]["Env"])
	}
	values := make([]string, 0, len(environment))
	for _, item := range environment {
		values = append(values, item.(string))
	}
	if !reflect.DeepEqual(values, []string{"KEEP=1", "S2S_GPU=auto", "S2S_BUNDLE_VERSION=stable"}) {
		t.Fatalf("managed environment = %#v", values)
	}
}

func TestV2DeploymentIsolatesDockerSocketAndDoesNotPreinstallModules(t *testing.T) {
	manifest := validV2Manifest()
	fake := newFakeDocker(t, manifest)
	manager := fake.manager(t, managedSpeechLabConfig(fake.server.URL), "")
	if err := manager.Install(context.Background()); err != nil {
		t.Fatalf("Install() error = %v", err)
	}
	for _, image := range fake.pullImages {
		if image == manifest.Runtimes[0].Image {
			t.Fatalf("module runtime was preinstalled: %#v", fake.pullImages)
		}
	}
	roles := make(map[string]map[string]any)
	socketOwners := 0
	for _, payload := range fake.createPayloads {
		labels := payload["Labels"].(map[string]any)
		role := labels["aurago.role"].(string)
		roles[role] = payload
		hostConfig := payload["HostConfig"].(map[string]any)
		binds, _ := hostConfig["Binds"].([]any)
		for _, bind := range binds {
			if strings.HasPrefix(bind.(string), "/var/run/docker.sock:") {
				socketOwners++
				if role != "controller" {
					t.Fatalf("Docker socket assigned to %s", role)
				}
			}
		}
	}
	if socketOwners != 1 {
		t.Fatalf("Docker socket owner count = %d", socketOwners)
	}
	for _, role := range []string{"control_init", "controller", "gateway"} {
		if roles[role] == nil {
			t.Fatalf("missing create payload for %s", role)
		}
	}
	controllerHost := roles["controller"]["HostConfig"].(map[string]any)
	if controllerHost["ReadonlyRootfs"] != true {
		t.Fatal("controller root filesystem is writable")
	}
	assertEnvironmentContains(t, roles["controller"], "S2S_CONTROLLER_TOKEN_READ_ONLY=true")
	assertEnvironmentContains(t, roles["gateway"], "S2S_DOCKER_PROXY_TOKEN_FILE=/control/token")
}

func TestManagedModuleContainersFollowStopStartAndRemove(t *testing.T) {
	manifest := validManifest()
	fake := newFakeDocker(t, manifest)
	manager := fake.manager(t, managedSpeechLabConfig(fake.server.URL), "")
	if err := manager.Install(context.Background()); err != nil {
		t.Fatal(err)
	}
	module := &fakeContainer{ID: "module-1", Name: "s2s-parakeet-cpu", Image: manifest.Images.ASR, Running: true, Attached: true, Labels: map[string]string{
		"aurago.managed": OwnerLabel, "aurago.component": "speech-lab", "aurago.role": "module", "aurago.bundle": manifest.BundleVersion,
		"s2s.lab.managed": "true", "stage": "asr", "backend-id": "parakeet", "variant-id": "parakeet-cpu", "s2s.image": manifest.Images.ASR,
	}}
	fake.addContainer(module)
	if err := manager.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	if module.Running {
		t.Fatal("module remained running after Stop")
	}
	state := manager.Status()
	if !reflect.DeepEqual(state.RunningModuleContainerIDs, []string{"module-1"}) {
		t.Fatalf("running module snapshot = %#v", state.RunningModuleContainerIDs)
	}
	if err := manager.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !module.Running {
		t.Fatal("previously running module was not restarted")
	}
	if err := manager.Remove(context.Background()); err != nil {
		t.Fatal(err)
	}
	if fake.find("module-1") != nil {
		t.Fatal("managed module remained after Remove")
	}
}

func TestStopRejectsManagedModuleWithImageDrift(t *testing.T) {
	manifest := validManifest()
	fake := newFakeDocker(t, manifest)
	manager := fake.manager(t, managedSpeechLabConfig(fake.server.URL), "")
	if err := manager.Install(context.Background()); err != nil {
		t.Fatal(err)
	}
	fake.addContainer(&fakeContainer{ID: "module-drift", Name: "s2s-parakeet-cpu", Image: manifest.Images.ASR, Running: true, Labels: map[string]string{
		"aurago.managed": OwnerLabel, "aurago.component": "speech-lab", "aurago.role": "module", "aurago.bundle": manifest.BundleVersion,
		"s2s.lab.managed": "true", "stage": "asr", "backend-id": "parakeet", "variant-id": "parakeet-cpu", "s2s.image": manifest.Images.TTS,
	}})
	if err := manager.Stop(context.Background()); Code(err) != "speech_lab_start_failed" {
		t.Fatalf("Stop() error = %v", err)
	}
}

func assertEnvironmentContains(t *testing.T, payload map[string]any, want string) {
	t.Helper()
	for _, value := range payload["Env"].([]any) {
		if value == want {
			return
		}
	}
	t.Fatalf("environment does not contain %q: %#v", want, payload["Env"])
}

func TestUpdateRecreatesDeploymentWhenHardwareProfileChanges(t *testing.T) {
	manifest := validManifest()
	fake := newFakeDocker(t, manifest)
	oldFingerprint := speechLabDeploymentFingerprint(manifestDigest(t, manifest), config.SpeechLabGPUBackendAuto)
	fake.addContainer(&fakeContainer{ID: "old", Name: "aurago-speech-lab-gateway", Image: manifest.Images.Gateway, Labels: dockerutil.ManagedLabels(OwnerLabel, "speech-lab", "gateway", oldFingerprint), Running: true, Attached: true})
	cfg := managedSpeechLabConfig(fake.server.URL)
	cfg.Deployment.GPUBackend = config.SpeechLabGPUBackendCPU
	manager := fake.manager(t, cfg, "")
	if err := manager.Update(context.Background()); err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if fake.creates != 1 {
		t.Fatalf("profile update created %d containers, want one replacement", fake.creates)
	}
	if state := manager.Status(); state.GPUBackend != config.SpeechLabGPUBackendCPU || state.State != "ready" {
		t.Fatalf("profile update state = %#v", state)
	}
}

func TestUpdateRollsBackAfterReadinessFailure(t *testing.T) {
	manifest := validManifest()
	fake := newFakeDocker(t, manifest)
	fake.ready = false
	fake.readyOnRollback = true
	fake.addContainer(&fakeContainer{ID: "old", Name: "aurago-speech-lab-gateway", Image: "ghcr.io/antibyte/old@sha256:" + strings.Repeat("a", 64), Labels: dockerutil.ManagedLabels(OwnerLabel, "speech-lab", "gateway", "old"), Running: true, Attached: true})
	manager := fake.manager(t, managedSpeechLabConfig(fake.server.URL), "")
	manager.state = State{Mode: "managed", Managed: true, State: "ready", Bundle: "old", Digest: "old", GPUBackend: config.SpeechLabGPUBackendAuto, NetworkID: "network-id", ContainerIDs: []string{"old"}}
	if err := manager.Update(context.Background()); err == nil {
		t.Fatal("Update() succeeded, want readiness error")
	}
	old := fake.find("old")
	if old == nil || old.Name != "aurago-speech-lab-gateway" || !old.Running || !old.Attached {
		t.Fatalf("old container was not restored: %#v", old)
	}
	if state := manager.Status(); state.Bundle != "old" || state.GPUBackend != config.SpeechLabGPUBackendAuto || state.Transaction != nil || !reflect.DeepEqual(state.ContainerIDs, []string{"old"}) {
		t.Fatalf("rollback state = %#v", state)
	}
}

func TestPullStreamErrorFailsBeforeContainerCreation(t *testing.T) {
	manifest := validManifest()
	fake := newFakeDocker(t, manifest)
	fake.pullError = "registry denied"
	manager := fake.manager(t, managedSpeechLabConfig(fake.server.URL), "")
	err := manager.Install(context.Background())
	if err == nil || Code(err) != "speech_lab_pull_failed" {
		t.Fatalf("Install() error = %v, want speech_lab_pull_failed", err)
	}
	if fake.creates != 0 {
		t.Fatalf("created %d containers after pull error", fake.creates)
	}
}

func TestRemoveRejectsUnownedContainerAfterManagedModeDisabled(t *testing.T) {
	manifest := validManifest()
	fake := newFakeDocker(t, manifest)
	fake.addContainer(&fakeContainer{ID: "foreign", Name: "foreign", Labels: map[string]string{"aurago.managed": "someone-else"}})
	cfg := managedSpeechLabConfig(fake.server.URL)
	cfg.Enabled = false
	cfg.Deployment.Mode = "external"
	manager := fake.manager(t, cfg, "")
	manager.state.ContainerIDs = []string{"foreign"}
	manager.state.NetworkID = ""
	if err := manager.Remove(context.Background()); err == nil {
		t.Fatal("Remove() accepted an unowned target")
	}
}

func TestRecoverPersistedTransactionRestoresBackup(t *testing.T) {
	manifest := validManifest()
	fake := newFakeDocker(t, manifest)
	fake.addContainer(&fakeContainer{ID: "backup", Name: "aurago-speech-lab-gateway-rollback", Image: manifest.Images.Gateway, Labels: dockerutil.ManagedLabels(OwnerLabel, "speech-lab", "gateway", "old"), Running: false})
	manager := fake.manager(t, managedSpeechLabConfig(fake.server.URL), "")
	manager.state = State{Mode: "managed", Managed: true, State: "pulling", NetworkID: "network-id", Transaction: &DeploymentTransaction{Phase: "replacing", PreviousBundle: "old", PreviousDigest: "old-digest", PreviousNetworkID: "network-id", PreviousContainerIDs: []string{"backup"}, Backups: []ContainerBackup{{ID: "backup", StableName: "aurago-speech-lab-gateway", BackupName: "aurago-speech-lab-gateway-rollback", WasRunning: true, WasAttached: true}}}}
	if err := manager.recoverTransaction(context.Background(), manager.operationSnapshot()); err != nil {
		t.Fatalf("recoverTransaction() error = %v", err)
	}
	restored := fake.find("backup")
	if restored == nil || restored.Name != "aurago-speech-lab-gateway" || !restored.Running || !restored.Attached {
		t.Fatalf("backup was not restored: %#v", restored)
	}
}

func TestRecoverTransactionIgnoresCancelledRequestContext(t *testing.T) {
	manifest := validManifest()
	fake := newFakeDocker(t, manifest)
	fake.addContainer(&fakeContainer{ID: "backup", Name: "aurago-speech-lab-gateway-rollback", Image: manifest.Images.Gateway, Labels: dockerutil.ManagedLabels(OwnerLabel, "speech-lab", "gateway", "old")})
	manager := fake.manager(t, managedSpeechLabConfig(fake.server.URL), "")
	manager.state = State{SchemaVersion: 2, Mode: "managed", Managed: true, State: "error", NetworkID: "network-id", Transaction: &DeploymentTransaction{ID: "tx", Phase: "rollback_pending", PreviousState: "stopped", PreviousNetworkID: "network-id", PreviousContainerIDs: []string{"backup"}, Backups: []ContainerBackup{{ID: "backup", StableName: "aurago-speech-lab-gateway", BackupName: "aurago-speech-lab-gateway-rollback"}}}}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if err := manager.recoverTransaction(cancelled, manager.operationSnapshot()); err != nil {
		t.Fatalf("recoverTransaction() error = %v", err)
	}
	state := manager.Status()
	if state.Transaction != nil || state.State != "stopped" {
		t.Fatalf("recovered state = %#v", state)
	}
}

func TestRollbackFailureRetainsRecoveryJournal(t *testing.T) {
	manifest := validManifest()
	fake := newFakeDocker(t, manifest)
	manager := fake.manager(t, managedSpeechLabConfig(fake.server.URL), "")
	manager.state = State{SchemaVersion: 2, Mode: "managed", Managed: true, State: "error", Transaction: &DeploymentTransaction{ID: "tx", Phase: "replacing", NewContainerIDs: []string{"new"}}}
	fake.server.Close()
	if err := manager.rollbackTransaction(context.Background(), manager.operationSnapshot()); err == nil {
		t.Fatal("rollbackTransaction() succeeded after Docker became unavailable")
	}
	state := manager.Status()
	if state.Transaction == nil || state.Transaction.Phase != "rollback_pending" {
		t.Fatalf("rollback journal was discarded: %#v", state.Transaction)
	}
}

func TestRollbackKeepsJournalUntilRestoredStackIsReady(t *testing.T) {
	manifest := validManifest()
	fake := newFakeDocker(t, manifest)
	fake.ready = false
	fake.addContainer(&fakeContainer{
		ID: "backup", Name: "aurago-speech-lab-gateway-rollback", Image: manifest.Images.Gateway,
		Labels: dockerutil.ManagedLabels(OwnerLabel, "speech-lab", "gateway", "old"), Attached: true,
	})
	manager := fake.manager(t, managedSpeechLabConfig(fake.server.URL), "")
	manager.state = State{
		SchemaVersion: 2, State: "error", NetworkID: "network-id",
		Transaction: &DeploymentTransaction{
			ID: "tx", Phase: "rollback_pending", PreviousState: "ready", PreviousNetworkID: "network-id",
			PreviousContainerIDs: []string{"backup"}, ReadinessBaseURL: fake.server.URL,
			Backups: []ContainerBackup{{ID: "backup", StableName: "aurago-speech-lab-gateway", NetworkName: manifest.Network, WasRunning: true, WasAttached: true}},
		},
	}
	if err := manager.recoverTransaction(context.Background(), manager.operationSnapshot()); err == nil {
		t.Fatal("recovery succeeded while the restored stack was not ready")
	}
	state := manager.Status()
	if state.Transaction == nil || state.Transaction.Phase != "rollback_pending" || state.State != "error" {
		t.Fatalf("not-ready rollback journal = %#v", state)
	}
}

func TestExternalRestartRetainsManagedCleanupTargets(t *testing.T) {
	manifest := validManifest()
	fake := newFakeDocker(t, manifest)
	fake.addContainer(&fakeContainer{ID: "old", Name: "aurago-speech-lab-gateway", Labels: dockerutil.ManagedLabels(OwnerLabel, "speech-lab", "gateway", "old")})
	dataDir := t.TempDir()
	managedCfg := managedSpeechLabConfig(fake.server.URL)
	manager := fake.manager(t, managedCfg, dataDir)
	manager.state = State{SchemaVersion: 2, Mode: "managed", Managed: true, State: "stopped", Bundle: "v1", Digest: "digest", NetworkID: "network-id", ContainerIDs: []string{"old"}}
	if err := manager.persist(); err != nil {
		t.Fatal(err)
	}
	externalCfg := managedCfg
	externalCfg.Enabled = false
	externalCfg.Deployment.Mode = "external"
	manager.Reconfigure(externalCfg)
	restarted := fake.manager(t, externalCfg, dataDir)
	public := restarted.PublicStatus()
	if public.Managed || !public.CleanupAvailable || public.Bundle != "v1" {
		t.Fatalf("external cleanup status = %#v", public)
	}
	if err := restarted.Remove(context.Background()); err != nil {
		t.Fatalf("Remove() error = %v", err)
	}
	if fake.find("old") != nil || restarted.PublicStatus().CleanupAvailable {
		t.Fatal("managed targets survived external cleanup")
	}
}

func TestReconfigureSeparatesRequestedAndInstalledBundleAndDockerSnapshot(t *testing.T) {
	manifest := validManifest()
	fake := newFakeDocker(t, manifest)
	manager := fake.manager(t, managedSpeechLabConfig(fake.server.URL), "")
	manager.state.Bundle = "release-2026.08.03"
	active := manager.operationSnapshot()
	nextCfg := active.cfg
	nextCfg.Deployment.Bundle = "nightly"
	nextDocker := dockerutil.NewClient("tcp://127.0.0.1:1", time.Second)
	manager.Reconfigure(nextCfg, RuntimeAccess{DockerEnabled: true, DockerReadOnly: true, Docker: nextDocker})
	next := manager.operationSnapshot()
	if active.docker == next.docker || active.dockerReadOnly || !next.dockerReadOnly {
		t.Fatalf("Docker snapshots were not isolated: active=%#v next=%#v", active, next)
	}
	public := manager.PublicStatus()
	if public.Bundle != "release-2026.08.03" || public.RequestedBundle != "nightly" {
		t.Fatalf("bundle status = %#v", public)
	}
	if err := manager.Install(context.Background()); err == nil || Code(err) != "speech_lab_docker_unavailable" {
		t.Fatalf("Install() error = %v, want current read-only policy", err)
	}
}

func TestConcurrentReconfigureAppliesOnlyToNextDeploymentOperation(t *testing.T) {
	manifest := validManifest()
	fake := newFakeDocker(t, manifest)
	fake.pullStarted = make(chan struct{})
	fake.releasePull = make(chan struct{})
	manager := fake.manager(t, managedSpeechLabConfig(fake.server.URL), "")
	errCh := make(chan error, 1)
	go func() { errCh <- manager.Install(context.Background()) }()
	select {
	case <-fake.pullStarted:
	case <-time.After(time.Second):
		t.Fatal("deployment did not reach Docker pull")
	}
	nextCfg := managedSpeechLabConfig("http://unreachable.invalid")
	nextCfg.Deployment.Mode = "external"
	nextCfg.Deployment.Bundle = "nightly"
	manager.Reconfigure(nextCfg, RuntimeAccess{
		DockerEnabled: true, DockerReadOnly: true,
		Docker: dockerutil.NewClient("tcp://127.0.0.1:1", time.Second),
	})
	close(fake.releasePull)
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("active deployment did not retain its start snapshot: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("deployment did not complete")
	}
	public := manager.PublicStatus()
	if public.Managed || public.RequestedBundle != "nightly" || public.Bundle != manifest.BundleVersion || !public.CleanupAvailable {
		t.Fatalf("post-reconfigure status = %#v", public)
	}
}

func TestDockerHostSwitchRequiresCleanupAndRemoveUsesInstalledHost(t *testing.T) {
	manifest := validManifest()
	installedDocker := newFakeDocker(t, manifest)
	installedDocker.addContainer(&fakeContainer{
		ID: "installed", Name: "aurago-speech-lab-gateway",
		Labels: dockerutil.ManagedLabels(OwnerLabel, "speech-lab", "gateway", "old"),
	})
	replacementDocker := newFakeDocker(t, manifest)
	dataDir := t.TempDir()
	managedCfg := managedSpeechLabConfig(installedDocker.server.URL)
	manager := installedDocker.manager(t, managedCfg, dataDir)
	installedHost := manager.operationSnapshot().dockerHost
	manager.state = State{
		SchemaVersion: 2, Mode: "managed", Managed: true, State: "stopped", Bundle: "old", Digest: "old",
		NetworkID: "network-id", ContainerIDs: []string{"installed"}, DockerHost: installedHost,
		ReadinessBaseURL: installedDocker.server.URL,
	}
	if err := manager.persist(); err != nil {
		t.Fatal(err)
	}
	replacementCfg := managedSpeechLabConfig(replacementDocker.server.URL)
	manager.Reconfigure(replacementCfg, RuntimeAccess{
		DockerEnabled: true,
		Docker:        replacementDocker.manager(t, replacementCfg, "").operationSnapshot().docker,
	})
	if err := manager.Update(context.Background()); err == nil || Code(err) != "speech_lab_cleanup_required" {
		t.Fatalf("Update() error = %v, want speech_lab_cleanup_required", err)
	}

	externalCfg := replacementCfg
	externalCfg.Enabled = false
	externalCfg.Deployment.Mode = "external"
	restarted := replacementDocker.manager(t, externalCfg, dataDir)
	if err := restarted.Remove(context.Background()); err != nil {
		t.Fatalf("Remove() on installed Docker host error = %v", err)
	}
	if installedDocker.find("installed") != nil {
		t.Fatal("installed container survived cleanup after Docker host switch")
	}
	if _, exists := replacementDocker.networks[manifest.Network]; !exists {
		t.Fatal("cleanup touched the replacement Docker host")
	}
}

func TestJournalPersistenceFailurePreventsDockerMutation(t *testing.T) {
	manifest := validManifest()
	fake := newFakeDocker(t, manifest)
	manager := fake.manager(t, managedSpeechLabConfig(fake.server.URL), t.TempDir())
	manager.stateWriter = func(string, []byte, os.FileMode) error { return errors.New("disk full") }
	err := manager.Install(context.Background())
	if err == nil || Code(err) != "speech_lab_state_persist_failed" {
		t.Fatalf("Install() error = %v, want speech_lab_state_persist_failed", err)
	}
	if fake.pulls != 0 || fake.creates != 0 {
		t.Fatalf("Docker mutated after journal persistence failed: pulls=%d creates=%d", fake.pulls, fake.creates)
	}
}

func TestStartPersistenceFailurePreventsContainerMutation(t *testing.T) {
	manifest := validManifest()
	fake := newFakeDocker(t, manifest)
	fake.addContainer(&fakeContainer{
		ID: "stopped", Name: "aurago-speech-lab-gateway", Image: manifest.Images.Gateway,
		Labels: dockerutil.ManagedLabels(OwnerLabel, "speech-lab", "gateway", manifestDigest(t, manifest)),
	})
	manager := fake.manager(t, managedSpeechLabConfig(fake.server.URL), t.TempDir())
	manager.state.ContainerIDs = []string{"stopped"}
	manager.stateWriter = func(string, []byte, os.FileMode) error { return errors.New("disk full") }
	err := manager.Start(context.Background())
	if err == nil || Code(err) != "speech_lab_state_persist_failed" {
		t.Fatalf("Start() error = %v, want speech_lab_state_persist_failed", err)
	}
	if container := fake.find("stopped"); container == nil || container.Running {
		t.Fatalf("container changed after state persistence failure: %#v", container)
	}
}

func TestRollbackRemovesOnlyResourcesCreatedByTransaction(t *testing.T) {
	manifest := validManifest()
	manifest.Network = "speech-lab-transaction-network"
	manifest.Volumes = []string{"speech-lab-transaction-models", "speech-lab-transaction-data"}
	fake := newFakeDocker(t, manifest)
	fake.ready = false
	manager := fake.manager(t, managedSpeechLabConfig(fake.server.URL), "")
	if err := manager.Install(context.Background()); err == nil {
		t.Fatal("Install() succeeded, want readiness error")
	}
	fake.mu.Lock()
	defer fake.mu.Unlock()
	if _, exists := fake.networks[manifest.Network]; exists {
		t.Fatal("transaction-created network survived rollback")
	}
	if _, exists := fake.networks["aurago-speech-lab"]; !exists {
		t.Fatal("pre-existing managed network was removed by rollback")
	}
	for _, volume := range manifest.Volumes {
		if _, exists := fake.volumes[volume]; exists {
			t.Fatalf("transaction-created volume %q survived rollback", volume)
		}
	}
	if manager.Status().Transaction != nil {
		t.Fatal("successful resource rollback retained the journal")
	}
}

func TestUpdateRollbackPreservesStoppedState(t *testing.T) {
	manifest := validManifest()
	fake := newFakeDocker(t, manifest)
	fake.ready = false
	fake.addContainer(&fakeContainer{ID: "old", Name: "aurago-speech-lab-gateway", Image: "ghcr.io/antibyte/old@sha256:" + strings.Repeat("a", 64), Labels: dockerutil.ManagedLabels(OwnerLabel, "speech-lab", "gateway", "old"), Attached: true})
	manager := fake.manager(t, managedSpeechLabConfig(fake.server.URL), "")
	manager.state = State{SchemaVersion: 2, Mode: "managed", Managed: true, State: "stopped", Bundle: "old", Digest: "old", NetworkID: "network-id", ContainerIDs: []string{"old"}}
	if err := manager.Update(context.Background()); err == nil {
		t.Fatal("Update() succeeded, want readiness error")
	}
	state := manager.Status()
	if state.State != "stopped" || state.Progress != 0 || state.Transaction != nil {
		t.Fatalf("stopped rollback state = %#v", state)
	}
	if old := fake.find("old"); old == nil || old.Running {
		t.Fatalf("stopped backup was started: %#v", old)
	}
}

func TestRollbackRestopsUnchangedServiceStartedDuringPartialUpdate(t *testing.T) {
	manifest := validManifest()
	manifest.StartOrder = []string{"gateway", "asr"}
	manifest.Services = []BundleService{{Role: "gateway", Image: "gateway"}, {Role: "asr", Image: "asr"}}
	fake := newFakeDocker(t, manifest)
	fake.ready = false
	fingerprint := manifestDigest(t, manifest)
	fake.addContainer(&fakeContainer{
		ID: "gateway", Name: "aurago-speech-lab-gateway", Image: manifest.Images.Gateway,
		Labels: dockerutil.ManagedLabels(OwnerLabel, "speech-lab", "gateway", fingerprint), Attached: true,
	})
	fake.addContainer(&fakeContainer{
		ID: "asr", Name: "aurago-speech-lab-asr", Image: "ghcr.io/antibyte/old@sha256:" + strings.Repeat("b", 64),
		Labels: dockerutil.ManagedLabels(OwnerLabel, "speech-lab", "asr", "old"), Attached: true,
	})
	manager := fake.manager(t, managedSpeechLabConfig(fake.server.URL), "")
	manager.state = State{
		SchemaVersion: 2, Mode: "managed", Managed: true, State: "stopped", Bundle: "old", Digest: "old",
		NetworkID: "network-id", ContainerIDs: []string{"gateway", "asr"},
	}
	if err := manager.Update(context.Background()); err == nil {
		t.Fatal("Update() succeeded, want readiness error")
	}
	for _, id := range []string{"gateway", "asr"} {
		container := fake.find(id)
		if container == nil || container.Running {
			t.Fatalf("container %q did not return to stopped state: %#v", id, container)
		}
	}
	if state := manager.Status(); state.Transaction != nil || state.State != "stopped" {
		t.Fatalf("partial rollback state = %#v", state)
	}
}

func TestExternalStartupRecoversJournalBeforeSkippingAutoStart(t *testing.T) {
	manifest := validManifest()
	fake := newFakeDocker(t, manifest)
	replacementDocker := newFakeDocker(t, manifest)
	fake.addContainer(&fakeContainer{
		ID: "backup", Name: "aurago-speech-lab-gateway-rollback", Image: manifest.Images.Gateway,
		Labels: dockerutil.ManagedLabels(OwnerLabel, "speech-lab", "gateway", "old"), Attached: true,
	})
	dataDir := t.TempDir()
	managedCfg := managedSpeechLabConfig(fake.server.URL)
	manager := fake.manager(t, managedCfg, dataDir)
	installedHost := manager.operationSnapshot().dockerHost
	manager.state = State{
		SchemaVersion: 2, Mode: "managed", Managed: true, State: "error", Bundle: "old", Digest: "old-digest", NetworkID: "network-id",
		Transaction: &DeploymentTransaction{
			ID: "persisted", Phase: "rollback_pending", PreviousState: "stopped", PreviousBundle: "old", PreviousDigest: "old-digest",
			PreviousNetworkID: "network-id", PreviousContainerIDs: []string{"backup"}, PreviousDockerHost: installedHost,
			DockerHost: installedHost, ReadinessBaseURL: fake.server.URL,
			Backups: []ContainerBackup{{ID: "backup", StableName: "aurago-speech-lab-gateway", NetworkName: manifest.Network, WasAttached: true}},
		},
	}
	if err := manager.persist(); err != nil {
		t.Fatal(err)
	}
	externalCfg := managedCfg
	externalCfg.Enabled = false
	externalCfg.Deployment.Mode = "external"
	externalCfg.BaseURL = replacementDocker.server.URL
	restarted := replacementDocker.manager(t, externalCfg, dataDir)
	if err := restarted.AutoStart(context.Background()); err != nil {
		t.Fatalf("AutoStart() recovery error = %v", err)
	}
	state := restarted.Status()
	if state.Transaction != nil || state.State != "stopped" || !reflect.DeepEqual(state.ContainerIDs, []string{"backup"}) {
		t.Fatalf("external startup recovery state = %#v", state)
	}
	if public := restarted.PublicStatus(); public.RecoveryPending || !public.CleanupAvailable || public.Managed {
		t.Fatalf("external startup public state = %#v", public)
	}
	if backup := fake.find("backup"); backup == nil || backup.Name != "aurago-speech-lab-gateway" {
		t.Fatalf("journal was not recovered on its original Docker host: %#v", backup)
	}
}
