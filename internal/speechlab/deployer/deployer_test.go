package deployer

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
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

type fakeDocker struct {
	mu               sync.Mutex
	server           *httptest.Server
	manifest         []byte
	ready            bool
	pullError        string
	startNotModified bool
	nextID           int
	containers       map[string]*fakeContainer
	pulls            int
	creates          int
}

func newFakeDocker(t *testing.T, manifest BundleManifest) *fakeDocker {
	t.Helper()
	raw, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	fake := &fakeDocker{manifest: raw, ready: true, containers: make(map[string]*fakeContainer), nextID: 1}
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
	case r.Method == http.MethodPost && path == "/images/create":
		f.pulls++
		if f.pullError != "" {
			_, _ = fmt.Fprintf(w, "{\"errorDetail\":{\"message\":%q}}\n", f.pullError)
			return
		}
		_, _ = io.WriteString(w, "{\"status\":\"done\"}\n")
		return
	case r.Method == http.MethodPost && path == "/networks/create":
		w.WriteHeader(http.StatusCreated)
		_, _ = io.WriteString(w, `{"Id":"network-id"}`)
		return
	case r.Method == http.MethodGet && strings.HasPrefix(path, "/networks/"):
		_, _ = io.WriteString(w, `{"Id":"network-id","Labels":{"aurago.managed":"speech-lab"}}`)
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
		w.WriteHeader(http.StatusCreated)
		_, _ = io.WriteString(w, `{}`)
		return
	case r.Method == http.MethodPost && path == "/containers/create":
		var body struct {
			Image      string            `json:"Image"`
			Labels     map[string]string `json:"Labels"`
			HostConfig struct {
				RestartPolicy struct{ Name string } `json:"RestartPolicy"`
			} `json:"HostConfig"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
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
	return NewManager(cfg, false, true, false, dataDir, nil,
		WithManifestURL(f.server.URL+"/manifest"), WithHTTPClient(f.server.Client()),
		WithDockerClient(dockerutil.NewClient(host, time.Second)), WithReadinessTimeout(40*time.Millisecond))
}

func managedSpeechLabConfig(baseURL string) config.SpeechLabConfig {
	return config.SpeechLabConfig{Enabled: true, BaseURL: baseURL, Deployment: config.SpeechLabDeploymentConfig{Mode: "managed", Bundle: "stable", AutoStart: true}}
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
	fake.addContainer(&fakeContainer{ID: "old", Name: "aurago-speech-lab-gateway", Image: manifest.Images.Gateway, Labels: dockerutil.ManagedLabels(OwnerLabel, "speech-lab", "gateway", manifestDigest(t, manifest)), Running: true, Attached: true})
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

func TestUpdateRollsBackAfterReadinessFailure(t *testing.T) {
	manifest := validManifest()
	fake := newFakeDocker(t, manifest)
	fake.ready = false
	fake.addContainer(&fakeContainer{ID: "old", Name: "aurago-speech-lab-gateway", Image: "ghcr.io/antibyte/old@sha256:" + strings.Repeat("a", 64), Labels: dockerutil.ManagedLabels(OwnerLabel, "speech-lab", "gateway", "old"), Running: true, Attached: true})
	manager := fake.manager(t, managedSpeechLabConfig(fake.server.URL), "")
	manager.state = State{Mode: "managed", Managed: true, State: "ready", Bundle: "old", Digest: "old", NetworkID: "network-id", ContainerIDs: []string{"old"}}
	if err := manager.Update(context.Background()); err == nil {
		t.Fatal("Update() succeeded, want readiness error")
	}
	old := fake.find("old")
	if old == nil || old.Name != "aurago-speech-lab-gateway" || !old.Running || !old.Attached {
		t.Fatalf("old container was not restored: %#v", old)
	}
	if state := manager.Status(); state.Bundle != "old" || state.Transaction != nil || !reflect.DeepEqual(state.ContainerIDs, []string{"old"}) {
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
	if err := manager.recoverTransaction(context.Background()); err != nil {
		t.Fatalf("recoverTransaction() error = %v", err)
	}
	restored := fake.find("backup")
	if restored == nil || restored.Name != "aurago-speech-lab-gateway" || !restored.Running || !restored.Attached {
		t.Fatalf("backup was not restored: %#v", restored)
	}
}
