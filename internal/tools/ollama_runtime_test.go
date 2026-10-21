package tools

import (
	"aurago/internal/config"
	"aurago/internal/dockerutil"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestOllamaJobsDeduplicateCancelAndFence(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	w := &ollamaRuntime{ctx: ctx, revision: "current", jobs: map[string]*ollamaWork{}, active: map[string]*ollamaWork{}}
	var calls atomic.Int32
	run := func(job *ollamaWork) error { calls.Add(1); <-job.ctx.Done(); return nil }
	a, err := w.start("current", "one", "model", "", run)
	if err != nil {
		t.Fatal(err)
	}
	b, err := w.start("current", "one", "model", "", run)
	if err != nil || a.ID != b.ID {
		t.Fatal(a, b, err)
	}
	if _, err := w.start("stale", "two", "", "", run); err == nil {
		t.Fatal("stale snapshot admitted")
	}
	cancel()
	w.wg.Wait()
	if calls.Load() != 1 || w.jobs[a.ID].State != "cancelled" {
		t.Fatal(calls.Load(), w.jobs[a.ID].State)
	}
}

func TestOllamaPullWaitsForBodyAndChecksError(t *testing.T) {
	for _, body := range []string{`{"status":"success"}`, `{"error":"interrupted"}`, `{"status":"success"`} {
		release := make(chan struct{})
		started := make(chan struct{})
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(200)
			w.(http.Flusher).Flush()
			close(started)
			<-release
			fmt.Fprint(w, body)
		}))
		done := make(chan error, 1)
		go func() { done <- ollamaPullVerified(OllamaConfig{URL: srv.URL}, "model") }()
		<-started
		select {
		case err := <-done:
			t.Fatal("completed before body", err)
		default:
		}
		close(release)
		err := <-done
		srv.Close()
		if (err == nil) != (body == `{"status":"success"}`) {
			t.Fatal(body, err)
		}
	}
}

func TestOllamaVersionRejectsForeignHTTP200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/version" {
			t.Error(r.URL.Path)
		}
		fmt.Fprint(w, `{"healthy":true}`)
	}))
	defer srv.Close()
	if _, err := OllamaVersion(context.Background(), srv.URL); err == nil {
		t.Fatal("foreign HTTP 200 was accepted")
	}
}

func TestOllamaControlCharactersAreJSONEncoded(t *testing.T) {
	name := "model\x01\"\\"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body["model"] != name {
			t.Error(body, err)
		}
		fmt.Fprint(w, `{"details":{}}`)
	}))
	defer srv.Close()
	if got := OllamaShowModel(OllamaConfig{URL: srv.URL}, name); !strings.Contains(got, `"status":"ok"`) {
		t.Fatal(got)
	}
}

func TestOllamaOwnershipAndNamespaces(t *testing.T) {
	desired, err := ollamaManagedCreatePayload(11434, "", "", GPUInfo{}, false)
	if err != nil {
		t.Fatal(err)
	}
	if ollamaOwnsInspect([]byte(`{"Config":{"Image":"nginx:latest"}}`), desired) {
		t.Fatal("foreign container adopted")
	}
	legacy := []byte(`{"Config":{"Image":"ollama/ollama:0.40.3"},"HostConfig":{"Binds":["aurago_ollama_managed_models:/root/.ollama"],"PortBindings":{"11434/tcp":[{"HostIp":"127.0.0.1","HostPort":"11434"}]}}}`)
	if !ollamaOwnsInspect(legacy, desired) {
		t.Fatal("known legacy spec rejected")
	}
	placed, err := ollamaPlacePayload(desired, "app_default")
	if err != nil {
		t.Fatal(err)
	}
	var body ollamaCreateBody
	json.Unmarshal(placed, &body)
	if len(body.HostConfig.PortBindings) != 0 || body.HostConfig.NetworkMode != "app_default" || !dockerutil.ManagedBy(body.Labels, dockerutil.OllamaOwner) {
		t.Fatal(string(placed))
	}
	if config.ManagedOllamaURL(true, true, 12345) != "http://aurago_ollama_embeddings:11434" {
		t.Fatal("wrong bridge endpoint")
	}
	var cfg config.Config
	cfg.Docker.Host = "tcp://remote.example:2375"
	if _, err := ollamaNetwork(&cfg); err == nil {
		t.Fatal("native remote Engine was guessed local")
	}
}

func TestOllamaPullCancellationDrainsHTTP(t *testing.T) {
	started := make(chan struct{})
	drained := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		close(started)
		<-r.Context().Done()
		close(drained)
	}))
	defer srv.Close()
	defer srv.CloseClientConnections()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- ollamaPullVerified(OllamaConfig{URL: srv.URL, Context: ctx}, "model") }()
	<-started
	cancel()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("cancelled pull succeeded")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("HTTP pull did not drain")
	}
	select {
	case <-drained:
	case <-time.After(3 * time.Second):
		t.Fatal("remote request not cancelled")
	}
}

func TestOllamaBridgeSelectionAndUnsupportedPlacement(t *testing.T) {
	configureDockerSecurityTestPermissions(t, false)
	host := fakeDockerHost(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "docker-control") {
			t.Fatal("control network inspected for application placement")
		}
		if strings.HasSuffix(r.URL.Path, "/app_default") {
			fmt.Fprint(w, `{"Driver":"bridge","Internal":false}`)
		} else {
			fmt.Fprint(w, `{"Driver":"bridge","Internal":true}`)
		}
	})
	self := "abcdef123456"
	doc := func(mode string) []byte {
		return []byte(fmt.Sprintf(`{"Id":"abcdef1234567890","HostConfig":{"NetworkMode":%q},"NetworkSettings":{"Networks":{"app_default":{},"aurago-app":{},"stack_docker-control":{}}}}`, mode))
	}
	if network, err := ollamaNetworkFromInspect(context.Background(), DockerConfig{Host: host}, self, doc("stack_default")); err != nil || network != "app_default" {
		t.Fatal(network, err)
	}
	for _, mode := range []string{"host", "container:other"} {
		if _, err := ollamaNetworkFromInspect(context.Background(), DockerConfig{Host: host}, self, doc(mode)); err == nil {
			t.Fatal("accepted", mode)
		}
	}
	if _, err := ollamaNetworkFromInspect(context.Background(), DockerConfig{Host: host}, "custom-host", doc("bridge")); err == nil {
		t.Fatal("unproven daemon identity accepted")
	}
}

func TestOllamaForeignReservedNameIsNeverMutated(t *testing.T) {
	configureDockerSecurityTestPermissions(t, false)
	host := fakeDockerHost(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" || !strings.Contains(r.URL.Path, "/containers/") {
			t.Errorf("foreign workload mutated: %s %s", r.Method, r.URL.Path)
		}
		fmt.Fprint(w, `{"Id":"foreign","Config":{"Image":"nginx:latest"}}`)
	})
	payload, _ := ollamaManagedCreatePayload(11434, "", "", GPUInfo{}, false)
	if err := reconcileNamedOllamaContainer(ollamaReconcile{ContainerName: ollamaManagedContainerName, Docker: DockerConfig{Host: host}, Payload: payload, Logger: ollamaTestLogger{}}); err == nil {
		t.Fatal("foreign container accepted")
	}
}

func TestOllamaToolPullIsAsyncAndRevokedByConfigChange(t *testing.T) {
	previous := ollamaRuntimeWork
	ollamaRuntimeWork = &ollamaRuntime{jobs: map[string]*ollamaWork{}, active: map[string]*ollamaWork{}}
	t.Cleanup(func() { ShutdownOllamaRuntime(); ollamaRuntimeWork = previous })
	started := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.Copy(io.Discard, r.Body)
		close(started)
		<-r.Context().Done()
	}))
	defer srv.Close()
	defer srv.CloseClientConnections()
	var cfg config.Config
	cfg.Ollama.Enabled = true
	cfg.Ollama.URL = srv.URL
	ConfigureOllamaRuntime(context.Background(), &cfg)
	var a, b struct {
		Status    string
		Operation OllamaOperation
	}
	if err := json.Unmarshal([]byte(StartOllamaToolPull(OllamaConfig{URL: srv.URL}, "fixture")), &a); err != nil || a.Operation.ID == "" {
		t.Fatal(a, err)
	}
	<-started
	json.Unmarshal([]byte(StartOllamaToolPull(OllamaConfig{URL: srv.URL}, "fixture")), &b)
	if a.Operation.ID != b.Operation.ID {
		t.Fatal("overlapping tool pulls were not collapsed")
	}
	cfg.Ollama.ReadOnly = true
	ConfigureOllamaRuntime(context.Background(), &cfg)
	ollamaRuntimeWork.wg.Wait()
	operations := OllamaOperations("")
	if len(operations) != 1 || operations[0].State == "ready" {
		t.Fatal(operations)
	}
	if got := StartOllamaToolPull(OllamaConfig{URL: srv.URL}, "fixture"); !strings.Contains(got, `"status":"error"`) {
		t.Fatal(got)
	}
}

func TestOllamaUncertainStopReconcilesWithReadsOnly(t *testing.T) {
	configureDockerSecurityTestPermissions(t, false)
	previous := ollamaRuntimeWork
	ollamaRuntimeWork = &ollamaRuntime{jobs: map[string]*ollamaWork{}, active: map[string]*ollamaWork{}}
	t.Cleanup(func() { ShutdownOllamaRuntime(); ollamaRuntimeWork = previous })
	var cfg config.Config
	cfg.Ollama.ManagedInstance.Enabled = true
	ConfigureOllamaRuntime(context.Background(), &cfg)
	cfg.Ollama.ManagedInstance.Enabled = false
	ConfigureOllamaRuntime(context.Background(), &cfg)
	if !OllamaStopsPending() {
		t.Fatal("disable did not fence Docker revocation")
	}
	host := fakeDockerHost(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Fatal("stop was replayed")
		}
		fmt.Fprint(w, `{"State":{"Running":false}}`)
	})
	if err := ConfirmOllamaStops(context.Background(), host); err != nil {
		t.Fatal(err)
	}
	if OllamaStopsPending() {
		t.Fatal("confirmed stop remained pending")
	}
}

func TestOllamaUnknownOperationIsNotAutomaticallyReplayed(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	w := &ollamaRuntime{ctx: ctx, revision: "current", jobs: map[string]*ollamaWork{}, active: map[string]*ollamaWork{}}
	var calls atomic.Int32
	run := func(*ollamaWork) error { calls.Add(1); return &ollamaUnknownOutcome{fmt.Errorf("outcome unknown")} }
	a, err := w.start("current", "key", "model", "", run)
	if err != nil {
		t.Fatal(err)
	}
	w.wg.Wait()
	b, err := w.start("current", "key", "model", "", run)
	if err != nil || a.ID != b.ID || calls.Load() != 1 {
		t.Fatal(a, b, err, calls.Load())
	}
}

func TestOllamaHistoryNeverEvictsUncertainWrites(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	w := &ollamaRuntime{ctx: ctx, revision: "current", jobs: map[string]*ollamaWork{}, active: map[string]*ollamaWork{}}
	for i := 0; i < 128; i++ {
		done := make(chan struct{})
		close(done)
		id := fmt.Sprint(i)
		w.jobs[id] = &ollamaWork{OllamaOperation: OllamaOperation{ID: id, State: "unknown"}, key: id, done: done}
	}
	if _, err := w.start("current", "new", "", "", func(*ollamaWork) error { t.Error("uncertain ledger was discarded"); return nil }); err == nil {
		t.Fatal("uncertain receipts were evicted to admit new work")
	}
}
