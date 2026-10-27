package tools

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/testutil"
)

type gotenbergEngineFixture struct {
	mu                                       sync.Mutex
	containers                               map[string]*gotenbergContainerInfo
	operations                               []string
	imagePresent                             bool
	pullFailure, createFailure, startFailure bool
	pullStarted                              chan struct{}
	stallPull                                bool
	verified                                 atomic.Bool
	unsafe                                   atomic.Bool
	base                                     gotenbergContainerConfig
}

const gotenbergLegacyTestID = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
const gotenbergNewTestID = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"

func newGotenbergEngineFixture(t *testing.T, legacy bool) (*gotenbergEngineFixture, *config.Config) {
	t.Helper()
	configureDockerSecurityTestPermissions(t, false)
	f := &gotenbergEngineFixture{containers: make(map[string]*gotenbergContainerInfo), imagePresent: true,
		base: gotenbergContainerConfig{Env: []string{"PATH=/usr/bin"}, Cmd: []string{"gotenberg"}, User: "gotenberg"}}
	if legacy {
		info := &gotenbergContainerInfo{ID: gotenbergLegacyTestID, Name: "/" + gotenbergContainerName, Image: "sha256:legacy", Config: f.base}
		info.Config.Image = "gotenberg/gotenberg:8"
		info.State.Running = true
		info.HostConfig.CapAdd = []string{"SYS_ADMIN"}
		info.HostConfig.ShmSize = 256 << 20
		info.HostConfig.RestartPolicy.Name = "unless-stopped"
		info.HostConfig.PortBindings = map[string][]gotenbergPortBinding{"3000/tcp": {{HostIP: "127.0.0.1", HostPort: "3000"}}}
		f.containers[gotenbergContainerName] = info
	}
	sidecar := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			_, _ = io.WriteString(w, `{"status":"up"}`)
			return
		}
		if f.unsafe.Load() {
			http.Error(w, "diagnostics disabled", 404)
			return
		}
		f.verified.Store(true)
		testutil.ServeGotenbergDebug(w, r)
	}))
	t.Cleanup(sidecar.Close)
	host := fakeDockerHost(t, f.serve(t))
	cfg := &config.Config{}
	cfg.Docker.Enabled = true
	cfg.Docker.Host = host
	cfg.Tools.DocumentCreator.Enabled = true
	cfg.Tools.DocumentCreator.Backend = "gotenberg"
	cfg.Tools.DocumentCreator.Gotenberg = config.GotenbergConfig{URL: sidecar.URL, Timeout: 2}
	return f, cfg
}

func (f *gotenbergEngineFixture) find(ref string) *gotenbergContainerInfo {
	if info := f.containers[ref]; info != nil {
		return info
	}
	for _, info := range f.containers {
		if info.ID == ref {
			return info
		}
	}
	return nil
}

func (f *gotenbergEngineFixture) serve(t *testing.T) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/"+dockerAPIVersion)
		if path == "/images/create" && f.stallPull {
			close(f.pullStarted)
			w.WriteHeader(200)
			_, _ = io.WriteString(w, `{"status":"Pulling"}`+"\n")
			w.(http.Flusher).Flush()
			<-r.Context().Done()
			return
		}
		f.mu.Lock()
		defer f.mu.Unlock()
		if r.Method != http.MethodGet {
			f.operations = append(f.operations, r.Method+" "+path)
		}
		write := func(code int, value any) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(code)
			_ = json.NewEncoder(w).Encode(value)
		}
		if strings.HasPrefix(path, "/images/") && strings.HasSuffix(path, "/json") {
			if strings.Contains(path, "sha256:legacy") {
				write(200, map[string]any{"Config": f.base})
				return
			}
			if !f.imagePresent {
				w.WriteHeader(404)
				return
			}
			write(200, map[string]string{"Id": "sha256:pinned"})
			return
		}
		if path == "/images/create" {
			if r.URL.Query().Get("fromImage") != gotenbergImage || r.URL.Query().Get("tag") != "" {
				t.Error("pull lost the digest pin")
			}
			if f.pullFailure {
				_, _ = io.WriteString(w, pullStreamFailure)
				return
			}
			f.imagePresent = true
			_, _ = io.WriteString(w, pullStreamSuccess)
			return
		}
		if path == "/containers/create" {
			if f.createFailure {
				w.WriteHeader(500)
				return
			}
			var created gotenbergContainerInfo
			var payload struct {
				Image      string
				Env, Cmd   []string
				Labels     map[string]string
				HostConfig json.RawMessage
			}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Error(err)
				w.WriteHeader(400)
				return
			}
			created.ID = gotenbergNewTestID
			created.Name = "/" + gotenbergContainerName
			created.Config = gotenbergContainerConfig{Image: payload.Image, Env: payload.Env, Cmd: payload.Cmd, Labels: payload.Labels}
			if err := json.Unmarshal(payload.HostConfig, &created.HostConfig); err != nil {
				t.Error(err)
			}
			f.containers[gotenbergContainerName] = &created
			write(201, map[string]string{"Id": created.ID})
			return
		}
		parts := strings.Split(strings.TrimPrefix(path, "/containers/"), "/")
		if len(parts) < 1 {
			t.Errorf("unexpected request: %s", path)
			w.WriteHeader(400)
			return
		}
		info := f.find(parts[0])
		if info == nil {
			w.WriteHeader(404)
			return
		}
		if len(parts) == 2 && parts[1] == "json" {
			write(200, info)
			return
		}
		if r.Method == http.MethodDelete {
			if !f.verified.Load() || r.URL.Query().Get("v") != "false" {
				t.Error("removed old container before verification or requested volume deletion")
			}
			delete(f.containers, strings.TrimPrefix(info.Name, "/"))
			w.WriteHeader(204)
			return
		}
		if len(parts) != 2 {
			t.Error("bad container operation")
			w.WriteHeader(400)
			return
		}
		switch parts[1] {
		case "stop":
			info.State.Running = false
		case "start":
			if f.startFailure {
				w.WriteHeader(500)
				return
			}
			info.State.Running = true
		case "rename":
			delete(f.containers, strings.TrimPrefix(info.Name, "/"))
			info.Name = "/" + r.URL.Query().Get("name")
			f.containers[strings.TrimPrefix(info.Name, "/")] = info
		default:
			t.Errorf("unexpected operation: %s", path)
			w.WriteHeader(400)
			return
		}
		w.WriteHeader(204)
	}
}

func TestGotenbergManagedCreateAndMigration(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		t.Run(strconv.FormatBool(legacy), func(t *testing.T) {
			f, cfg := newGotenbergEngineFixture(t, legacy)
			if err := EnsureGotenbergRunning(t.Context(), cfg, true, slog.Default()); err != nil {
				t.Fatal(err)
			}
			f.mu.Lock()
			defer f.mu.Unlock()
			if len(f.containers) != 1 || !f.containers[gotenbergContainerName].State.Running {
				t.Fatal("replacement did not become the only running owned container")
			}
			u, _ := url.Parse(cfg.Tools.DocumentCreator.Gotenberg.URL)
			if !gotenbergProfileMatches(f.containers[gotenbergContainerName], "127.0.0.1", u.Port()) {
				t.Fatal("created profile differs from the required profile")
			}
			ops := strings.Join(f.operations, "\n")
			if legacy {
				stop := strings.Index(ops, "/stop")
				create := strings.Index(ops, "/containers/create")
				remove := strings.Index(ops, "DELETE")
				if stop < 0 || create < stop || remove < create {
					t.Fatalf("bad migration order: %s", ops)
				}
			}
		})
	}
}

func TestGotenbergMigrationFailuresKeepTheOldContainer(t *testing.T) {
	for _, failure := range []string{"pull", "create", "start", "security"} {
		t.Run(failure, func(t *testing.T) {
			f, cfg := newGotenbergEngineFixture(t, true)
			switch failure {
			case "pull":
				f.imagePresent = false
				f.pullFailure = true
			case "create":
				f.createFailure = true
			case "start":
				f.startFailure = true
			case "security":
				f.unsafe.Store(true)
			}
			ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
			defer cancel()
			if err := EnsureGotenbergRunning(ctx, cfg, true, slog.Default()); err == nil {
				t.Fatal("migration unexpectedly succeeded")
			}
			f.mu.Lock()
			defer f.mu.Unlock()
			old := f.find(gotenbergLegacyTestID)
			if old == nil {
				t.Fatal("lost the old container")
			}
			if failure == "pull" {
				if !old.State.Running {
					t.Fatal("stopped old container before the replacement image was available")
				}
			} else if old.State.Running {
				t.Fatal("old unsafe container was restarted")
			}
			if strings.Contains(strings.Join(f.operations, "\n"), "DELETE") {
				t.Fatal("removed recovery container after failure")
			}
		})
	}
}

func TestGotenbergLiveConfigRequiresRestartAndHonorsReadOnly(t *testing.T) {
	f, cfg := newGotenbergEngineFixture(t, true)
	if !GotenbergRestartRequired(t.Context(), cfg) {
		t.Fatal("legacy profile was not marked pending")
	}
	if err := EnsureGotenbergRunning(t.Context(), cfg, false, slog.Default()); !errors.Is(err, ErrGotenbergRestartRequired) {
		t.Fatal(err)
	}
	cfg.Docker.ReadOnly = true
	if err := EnsureGotenbergRunning(t.Context(), cfg, true, slog.Default()); !errors.Is(err, ErrDockerReadOnly) {
		t.Fatal(err)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.operations) != 0 {
		t.Fatalf("live save/read-only mutated Docker: %v", f.operations)
	}
}

func TestGotenbergStartupWaitsForHealthyRenderer(t *testing.T) {
	var health atomic.Int64
	sidecar := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			if health.Add(1) == 1 {
				http.Error(w, "starting", http.StatusServiceUnavailable)
				return
			}
			_, _ = io.WriteString(w, `{"status":"up"}`)
			return
		}
		testutil.ServeGotenbergDebug(w, r)
	}))
	defer sidecar.Close()
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	if err := waitForGotenberg(ctx, &config.GotenbergConfig{URL: sidecar.URL, Timeout: 1}); err != nil {
		t.Fatal(err)
	}
	if health.Load() != 2 {
		t.Fatalf("health attempts=%d want=2", health.Load())
	}
}

func TestGotenbergRefusesForeignAndAmbiguousContainers(t *testing.T) {
	for _, kind := range []string{"compose", "mount", "auto-remove", "custom env", "custom name", "unlabelled newer image"} {
		t.Run(kind, func(t *testing.T) {
			f, cfg := newGotenbergEngineFixture(t, true)
			info := f.containers[gotenbergContainerName]
			switch kind {
			case "compose":
				info.Config.Labels = map[string]string{"com.docker.compose.service": "gotenberg"}
			case "mount":
				info.Mounts = []json.RawMessage{json.RawMessage(`{"Type":"volume"}`)}
			case "auto-remove":
				info.Config.Labels = map[string]string{gotenbergOwnerLabel: gotenbergOwner}
				info.HostConfig.AutoRemove = true
			case "custom env":
				info.Config.Env = []string{"PATH=/usr/bin", "CUSTOM=yes"}
			case "custom name":
				info.Name = "/external"
			case "unlabelled newer image":
				info.Config.Image = "gotenberg/gotenberg:8.38.0"
			}
			if err := EnsureGotenbergRunning(t.Context(), cfg, true, slog.Default()); err == nil {
				t.Fatal("adopted unverified container")
			}
			f.mu.Lock()
			defer f.mu.Unlock()
			if len(f.operations) != 0 {
				t.Fatalf("modified foreign container: %v", f.operations)
			}
		})
	}
}

func TestGotenbergManagedTargetIsLocalOnly(t *testing.T) {
	_, cfg := newGotenbergEngineFixture(t, false)
	for _, raw := range []string{"http://gotenberg:3000", "http://192.168.1.1:3000", "https://127.0.0.1:3000", "http://127.0.0.1:3000/prefix"} {
		cfg.Tools.DocumentCreator.Gotenberg.URL = raw
		if _, _, managed := gotenbergManagedTarget(cfg); managed {
			t.Errorf("adopted %s", raw)
		}
	}
	cfg.Tools.DocumentCreator.Gotenberg.URL = "http://127.0.0.1:3100"
	if _, port, managed := gotenbergManagedTarget(cfg); !managed || port != "3100" {
		t.Fatal("lost custom local port")
	}
	cfg.Runtime.IsDocker = true
	if _, _, managed := gotenbergManagedTarget(cfg); managed {
		t.Fatal("auto-created from inside Docker")
	}
	cfg.Runtime.IsDocker = false
	cfg.Docker.Host = "tcp://192.168.1.1:2375"
	if _, _, managed := gotenbergManagedTarget(cfg); managed {
		t.Fatal("auto-created on remote Engine")
	}
}

func TestGotenbergPullEndsWithServerContext(t *testing.T) {
	f, cfg := newGotenbergEngineFixture(t, false)
	f.imagePresent = false
	f.stallPull = true
	f.pullStarted = make(chan struct{})
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- EnsureGotenbergRunning(ctx, cfg, true, slog.Default()) }()
	select {
	case <-f.pullStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("pull did not start")
	}
	cancel()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("canceled pull succeeded")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("pull outlived server cancellation")
	}
}
