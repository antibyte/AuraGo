package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/infra"
	"aurago/internal/inventory"
	"aurago/internal/tools"
)

func TestDesktopInfraSnapshotReturnsCachedStateWhileRefreshing(t *testing.T) {
	s, cfg, store, runtime := newDesktopInfraRefreshFixture(t)
	if _, err := store.CreateNode(context.Background(), infra.Node{
		Kind: infra.NodeDevice, Name: "cached router", Icon: "router",
		Profile: infra.Profile{Tags: []string{}, Interfaces: []infra.NetworkInterface{}, Links: []infra.Link{}, AppBindings: []infra.AppLink{}, Custom: map[string]string{}},
	}); err != nil {
		t.Fatal(err)
	}

	release := make(chan struct{})
	t.Cleanup(func() { closeInfraRefreshTestChannel(release) })
	started := make(chan context.Context, 2)
	providerDone := make(chan struct{}, 2)
	runtime.dockerRequest = func(callCtx context.Context, _ tools.DockerConfig, method, endpoint, _ string) ([]byte, int, error) {
		if method != http.MethodGet || endpoint != "/containers/json?all=true" {
			t.Errorf("unexpected Docker request: %s %s", method, endpoint)
		}
		started <- callCtx
		select {
		case <-release:
			providerDone <- struct{}{}
			return []byte("[]"), http.StatusOK, nil
		case <-callCtx.Done():
			providerDone <- struct{}{}
			return nil, 0, callCtx.Err()
		}
	}

	callSnapshot := func(query string) <-chan *httptest.ResponseRecorder {
		done := make(chan *httptest.ResponseRecorder, 1)
		request := httptest.NewRequest(http.MethodGet, desktopInfraBasePath+"/snapshot"+query, nil)
		go func() {
			response := httptest.NewRecorder()
			desktopInfraSnapshot(response, request, s, cfg, store, runtime)
			done <- response
		}()
		return done
	}

	response := callSnapshot("")
	var first *httptest.ResponseRecorder
	select {
	case first = <-response:
	case <-time.After(time.Second):
		t.Fatal("ordinary snapshot waited for the provider refresh")
	}
	if first.Code != http.StatusOK {
		t.Fatalf("snapshot status=%d body=%s", first.Code, first.Body.String())
	}
	var snapshot infra.Snapshot
	if err := json.Unmarshal(first.Body.Bytes(), &snapshot); err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Nodes) != 1 || snapshot.Nodes[0].Name != "cached router" {
		t.Fatalf("snapshot did not return cached Infra state: %#v", snapshot.Nodes)
	}

	var providerCtx context.Context
	select {
	case providerCtx = <-started:
	case <-time.After(time.Second):
		t.Fatal("background refresh did not start")
	}
	deadline, ok := providerCtx.Deadline()
	if !ok || time.Until(deadline) > 15*time.Second {
		t.Fatalf("background provider call has no 15-second bound: deadline=%v present=%t", deadline, ok)
	}

	second := callSnapshot("")
	select {
	case result := <-second:
		if result.Code != http.StatusOK {
			t.Fatalf("second snapshot status=%d body=%s", result.Code, result.Body.String())
		}
	case <-time.After(time.Second):
		t.Fatal("second cached snapshot waited for the in-flight refresh")
	}
	select {
	case <-started:
		t.Fatal("concurrent snapshots started duplicate provider calls")
	case <-time.After(50 * time.Millisecond):
	}
	close(release)
	select {
	case <-providerDone:
	case <-time.After(time.Second):
		t.Fatal("background provider refresh did not finish")
	}
	waitForInfraRefresh(t, runtime)

	release = make(chan struct{})
	explicitCtx, cancelExplicit := context.WithCancel(context.Background())
	t.Cleanup(cancelExplicit)
	explicitDone := make(chan *httptest.ResponseRecorder, 1)
	explicitRequest := httptest.NewRequest(http.MethodGet, desktopInfraBasePath+"/snapshot?refresh=1", nil).WithContext(explicitCtx)
	go func() {
		result := httptest.NewRecorder()
		desktopInfraSnapshot(result, explicitRequest, s, cfg, store, runtime)
		explicitDone <- result
	}()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("explicit refresh did not call the provider")
	}
	select {
	case <-explicitDone:
		t.Fatal("explicit refresh returned before the provider completed")
	case <-time.After(50 * time.Millisecond):
	}
	close(release)
	select {
	case result := <-explicitDone:
		if result.Code != http.StatusOK {
			t.Fatalf("explicit snapshot status=%d body=%s", result.Code, result.Body.String())
		}
	case <-time.After(time.Second):
		t.Fatal("explicit refresh did not return after the provider completed")
	}
	waitForInfraRefresh(t, runtime)
}

func TestDesktopInfraBackgroundRefreshStopsOnConfigRevocation(t *testing.T) {
	s, cfg, store, runtime := newDesktopInfraRefreshFixture(t)
	started := make(chan context.Context, 1)
	providerDone := make(chan struct{}, 1)
	runtime.dockerRequest = func(callCtx context.Context, _ tools.DockerConfig, _, _, _ string) ([]byte, int, error) {
		started <- callCtx
		<-callCtx.Done()
		providerDone <- struct{}{}
		return nil, 0, callCtx.Err()
	}
	desktopInfraSnapshot(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, desktopInfraBasePath+"/snapshot", nil), s, cfg, store, runtime)
	var providerCtx context.Context
	select {
	case providerCtx = <-started:
	case <-time.After(time.Second):
		t.Fatal("background refresh did not start")
	}

	next := *cfg
	next.Docker.Host = "mock://replacement"
	s.cancelDesktopInfraOperations(&next)
	select {
	case <-providerCtx.Done():
	case <-time.After(time.Second):
		t.Fatal("config revocation did not cancel the provider context")
	}
	select {
	case <-providerDone:
	case <-time.After(time.Second):
		t.Fatal("revoked provider did not exit")
	}
	waitForInfraRefresh(t, runtime)
}

func TestDesktopInfraRefreshDrainsBeforeServerShutdown(t *testing.T) {
	s, cfg, store, runtime := newDesktopInfraRefreshFixture(t)
	started := make(chan context.Context, 1)
	providerDone := make(chan struct{}, 1)
	runtime.dockerRequest = func(callCtx context.Context, _ tools.DockerConfig, _, _, _ string) ([]byte, int, error) {
		started <- callCtx
		<-callCtx.Done()
		providerDone <- struct{}{}
		return nil, 0, callCtx.Err()
	}
	desktopInfraSnapshot(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, desktopInfraBasePath+"/snapshot", nil), s, cfg, store, runtime)
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("background refresh did not start")
	}

	s.beginHTTPDrain()
	select {
	case <-providerDone:
	default:
		t.Fatal("server shutdown returned before the provider refresh exited")
	}
	runtime.mu.Lock()
	done := runtime.refreshDone
	runtime.mu.Unlock()
	if done != nil {
		t.Fatal("server shutdown returned before releasing the refresh slot")
	}
}

func newDesktopInfraRefreshFixture(t *testing.T) (*Server, *config.Config, *infra.Store, *desktopInfraRuntime) {
	t.Helper()
	db, err := inventory.InitDB(filepath.Join(t.TempDir(), "inventory.db"))
	if err != nil {
		t.Fatal(err)
	}
	store, err := infra.New(db)
	if err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{}
	cfg.VirtualDesktop.Enabled = true
	cfg.VirtualDesktop.Infra.Enabled = true
	cfg.Docker.Enabled = true
	cfg.Docker.Host = "mock://docker"
	lifetime, cancel := context.WithCancel(context.Background())
	s := &Server{Cfg: cfg, InventoryDB: db, integrationCtx: lifetime}
	s.setServerLifetimeCancel(cancel)
	s.initConfigSnapshot()
	runtime := &desktopInfraRuntime{store: store, cache: make(map[string]*desktopInfraProviderCache)}
	s.infraRuntime = runtime
	t.Cleanup(func() {
		cancel()
		s.drainDesktopInfraRefresh()
		_ = db.Close()
	})
	return s, cfg, store, runtime
}

func waitForInfraRefresh(t *testing.T, runtime *desktopInfraRuntime) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		runtime.mu.Lock()
		busy := runtime.refreshDone != nil
		runtime.mu.Unlock()
		if !busy {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("Infra refresh did not release its scheduling slot")
}

func closeInfraRefreshTestChannel(channel chan struct{}) {
	select {
	case <-channel:
	default:
		close(channel)
	}
}
