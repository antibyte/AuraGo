package acestep

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/dockerutil"
	"aurago/internal/security"
)

func testManager(t *testing.T) *Manager {
	t.Helper()
	cfg := &config.Config{}
	cfg.Directories.DataDir = t.TempDir()
	cfg.Docker.Enabled = true
	cfg.MusicGeneration.Enabled = true
	cfg.MusicGeneration.Provider = config.LocalMusicProviderID
	m := New(cfg, nil, nil)
	t.Cleanup(m.cancel)
	m.key = "test-private-key"
	m.status = Status{State: "ready", Ready: true, Enabled: true, Profile: &Profile{Model: "acestep-v15-turbo", MaxDuration: 120, Fingerprint: "qualified"}}
	return m
}

func TestParamsAndSeedZero(t *testing.T) {
	var p Params
	if err := json.Unmarshal([]byte(`{"prompt":"Piano","instrumental":true,"seed":0}`), &p); err != nil {
		t.Fatal(err)
	}
	valid, err := p.Validate(120, false)
	if err != nil || valid.Seed == nil || *valid.Seed != 0 || valid.DurationSeconds != 120 {
		t.Fatalf("zero/default lost: %+v %v", valid, err)
	}
	for _, raw := range []string{`{"duration_seconds":601}`, `{"duration_seconds":9}`, `{"bpm":29}`, `{"bpm":301}`, `{"seed":-1}`, `{"vocal_language":"../../en"}`, `{"instrumental":false}`} {
		bad := p
		seed := *p.Seed
		bad.Seed = &seed
		if err := json.Unmarshal([]byte(raw), &bad); err != nil {
			t.Fatal(err)
		}
		if _, err := bad.Validate(120, false); err == nil {
			t.Errorf("accepted %s", raw)
		}
	}
	p.Instrumental = false
	p.Lyrics = "[Verse] Hello"
	if _, err := p.Validate(120, false); err != nil {
		t.Fatal(err)
	}
	p.Lyrics = ""
	if _, err := p.Validate(120, true); err != nil {
		t.Fatal(err)
	}
}

func TestAudioDownloadConfinement(t *testing.T) {
	valid := "/v1/audio?path=" + url.QueryEscape("/app/.cache/acestep/tmp/api_audio/a.mp3")
	if _, err := audioDownloadPath(valid); err != nil {
		t.Fatal(err)
	}
	for _, raw := range []string{"https://evil.test" + valid, "//evil.test" + valid, valid + "&path=x", valid + "&redirect=x", "/v1/audio?path=/etc/passwd", "/v1/audio?path=/app/.cache/acestep/tmp/api_audio/../a.mp3", valid + "#fragment"} {
		if _, err := audioDownloadPath(raw); err == nil {
			t.Errorf("accepted %q", raw)
		}
	}
	m := testManager(t)
	var called atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called.Add(1) }))
	defer target.Close()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, target.URL, 302) }))
	defer server.Close()
	m.baseURL = server.URL
	if _, err := m.downloadAudio(context.Background(), valid, 10, "turbo"); err == nil {
		t.Fatal("redirect accepted")
	}
	if called.Load() != 0 {
		t.Fatal("followed foreign download URL")
	}
}

func TestNativeGenerationContract(t *testing.T) {
	m := testManager(t)
	seed := int64(0)
	audioPath := "/v1/audio?path=" + url.QueryEscape("/app/.cache/acestep/tmp/api_audio/test.mp3")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-private-key" {
			t.Error("missing runtime auth")
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/release_task":
			var b map[string]any
			_ = json.NewDecoder(r.Body).Decode(&b)
			if b["seed"] != float64(0) || b["use_random_seed"] != false || b["batch_size"] != float64(1) || b["audio_duration"] != float64(120) || b["thinking"] != false {
				t.Errorf("payload: %v", b)
			}
			io.WriteString(w, `{"code":200,"data":{"task_id":"job1"}}`)
		case "/query_result":
			result, _ := json.Marshal([]any{map[string]any{"file": audioPath, "status": 1, "metas": map[string]any{"duration": 120}}})
			_ = json.NewEncoder(w).Encode(map[string]any{"code": 200, "data": []any{map[string]any{"task_id": "job1", "status": 1, "result": string(result)}}})
		case "/v1/audio":
			w.Header().Set("Content-Type", "audio/mpeg")
			io.WriteString(w, "ID3testaudio")
		default:
			t.Errorf("unexpected endpoint %s", r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	m.baseURL = server.URL
	result, err := m.Generate(context.Background(), Params{Prompt: "Piano", Instrumental: true, Seed: &seed})
	if err != nil {
		t.Fatal(err)
	}
	if result.DurationMs != 120000 || filepath.Dir(result.Path) != filepath.Join(m.dataDir, "audio") {
		t.Fatalf("bad result %+v", result)
	}
	entries, _ := os.ReadDir(filepath.Dir(result.Path))
	if len(entries) != 1 || strings.HasSuffix(entries[0].Name(), ".part") {
		t.Fatal("non-atomic audio output")
	}
}

func TestCancellationStopsOnlyOwnedWorkerAndBusyDoesNotSubmit(t *testing.T) {
	for _, mode := range []string{"cancel", "deadline"} {
		t.Run(mode, func(t *testing.T) {
			m := testManager(t)
			submitted := make(chan struct{})
			var stops atomic.Int32
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/release_task" {
					t.Error("unexpected request")
				}
				close(submitted)
				io.WriteString(w, `{"code":200,"data":{"task_id":"one"}}`)
			}))
			defer upstream.Close()
			m.baseURL = upstream.URL
			docker := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.Method {
				case "GET":
					_ = json.NewEncoder(w).Encode(map[string]any{"Id": "owned123", "Config": map[string]any{"Labels": dockerutil.ManagedLabels(Owner, "music", "runtime", "")}, "State": map[string]any{"Running": true}})
				case "POST":
					if !strings.Contains(r.URL.Path, "owned123/stop") {
						t.Errorf("wrong target %s", r.URL.Path)
					}
					stops.Add(1)
					w.WriteHeader(204)
				default:
					t.Errorf("unexpected Docker mutation %s", r.Method)
				}
			}))
			defer docker.Close()
			m.docker = dockerutil.NewClient(strings.Replace(docker.URL, "http://", "tcp://", 1), time.Second)
			ctx, cancel := context.WithCancel(context.Background())
			wantError := context.Canceled
			if mode == "deadline" {
				cancel()
				ctx, cancel = context.WithTimeout(context.Background(), 100*time.Millisecond)
				wantError = context.DeadlineExceeded
			}
			defer cancel()
			done := make(chan error, 1)
			go func() { _, err := m.Generate(ctx, Params{Prompt: "Piano", Instrumental: true}); done <- err }()
			<-submitted
			if _, err := m.Generate(context.Background(), Params{Prompt: "Other", Instrumental: true}); err == nil || err.Error() != "acestep_busy" {
				t.Fatalf("concurrent result %v", err)
			}
			if mode == "cancel" {
				cancel()
			}
			select {
			case err := <-done:
				if !errors.Is(err, wantError) {
					t.Fatal(err)
				}
			case <-time.After(3 * time.Second):
				t.Fatal("cancel did not complete")
			}
			if stops.Load() != 1 {
				t.Fatalf("worker stop count %d status %+v", stops.Load(), m.Status())
			}
		})
	}
}

func TestConfigChangesPreserveRunningJobExceptDisable(t *testing.T) {
	m := testManager(t)
	var canceled atomic.Int32
	m.jobCancel = func() { canceled.Add(1) }
	cfg := &config.Config{}
	cfg.Docker.Enabled = true
	cfg.MusicGeneration.Enabled = true
	cfg.MusicGeneration.Provider = config.LocalMusicProviderID
	cfg.MusicGeneration.Local.Backend = "cuda"
	m.Configure(cfg)
	if canceled.Load() != 0 {
		t.Fatal("profile change canceled running generation")
	}
	cfg.MusicGeneration.Enabled = false
	m.Configure(cfg)
	if canceled.Load() != 1 {
		t.Fatal("disable did not cancel generation")
	}
	if !m.StopPending() {
		t.Fatal("Docker writes released before worker stopped")
	}
	m.want.Writable = false
	if err := m.Action("start"); err == nil {
		t.Fatal("read-only action accepted")
	}
}

func TestOwnedResourceConflictFailsClosed(t *testing.T) {
	m := testManager(t)
	var writes atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			writes.Add(1)
		}
		io.WriteString(w, `{"Id":"foreign","Config":{"Labels":{}},"State":{"Running":true}}`)
	}))
	defer server.Close()
	m.docker = dockerutil.NewClient(strings.Replace(server.URL, "http://", "tcp://", 1), time.Second)
	if err := m.removeOwned(context.Background(), ContainerName); err == nil {
		t.Fatal("foreign container accepted")
	}
	if writes.Load() != 0 {
		t.Fatal("foreign container mutated")
	}
	host := baseHostConfig()
	if host["ReadonlyRootfs"] != true {
		t.Fatal("writable root")
	}
	if err := addGPU(host, "xpu", &Device{RenderNodes: []string{"/etc"}}); err == nil {
		t.Fatal("unsafe device mount")
	}
}

func TestHardwareSelectionUsesFreeMemoryAndManualChoice(t *testing.T) {
	devices := []Device{{ID: "cuda:0", FreeGB: 5, TotalGB: 24, Verified: true}, {ID: "cuda:1", FreeGB: 10, TotalGB: 12, Verified: true}, {ID: "cuda:2", FreeGB: 80, Verified: false}}
	chosen, err := selectDevice(devices, "auto")
	if err != nil || chosen.ID != "cuda:1" {
		t.Fatalf("selection %+v %v", chosen, err)
	}
	chosen, err = selectDevice(devices, "cuda:0")
	if err != nil || chosen.ID != "cuda:0" {
		t.Fatal("manual selection lost")
	}
	if _, err = selectDevice(devices, "cpu"); err == nil {
		t.Fatal("silent CPU fallback")
	}
	if _, err = selectDevice(devices, "cuda:2"); err == nil {
		t.Fatal("unverified GPU selected")
	}
}

func TestRollbackRetainsPreviousImageAfterInterruptedUpdate(t *testing.T) {
	oldImage := "ghcr.io/antibyte/aurago-acestep-cpu@sha256:" + strings.Repeat("1", 64)
	newImage := "ghcr.io/antibyte/aurago-acestep-cpu@sha256:" + strings.Repeat("2", 64)
	oldRelease := releaseJSON
	t.Cleanup(func() { releaseJSON = oldRelease })
	releaseJSON, _ = json.Marshal(releaseManifest{Images: map[string]string{"cpu": newImage, "cuda": newImage, "rocm": newImage, "xpu": newImage}})
	m := testManager(t)
	var err error
	m.vault, err = security.NewVault(strings.Repeat("a", 64), filepath.Join(t.TempDir(), "vault.bin"))
	if err != nil {
		t.Fatal(err)
	}
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(runtimeStatus{Ready: true, ImagePin: oldImage, Profile: Profile{Model: "acestep-v15-turbo", Fingerprint: "old-qualified", Device: Device{Backend: "cpu", Verified: true}}})
	}))
	defer api.Close()
	m.baseURL = api.URL
	var writes []string
	restored := false
	docker := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if r.Method != "GET" {
			writes = append(writes, r.Method+" "+path)
			if strings.HasSuffix(path, "/old/rename") {
				restored = true
			}
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if strings.HasSuffix(path, "/info") {
			io.WriteString(w, `{"OSType":"unsupported","Architecture":"amd64"}`)
			return
		}
		id := "failed-candidate"
		if strings.Contains(path, "-previous/") || restored {
			id = "old"
		}
		json.NewEncoder(w).Encode(map[string]any{"Id": id, "Config": map[string]any{"Image": oldImage, "Labels": dockerutil.ManagedLabels(Owner, "music", "runtime", "old")}, "State": map[string]any{"Running": false}})
	}))
	defer docker.Close()
	m.docker = dockerutil.NewClient(strings.Replace(docker.URL, "http://", "tcp://", 1), time.Second)
	if err := m.install(context.Background(), m.want); err == nil {
		t.Fatal("unsupported new environment accepted")
	}
	status := m.Status()
	if !status.Ready || status.Image != oldImage || status.Profile.Fingerprint != "old-qualified" {
		t.Fatalf("old qualified runtime not restored: %+v", status)
	}
	joined := strings.Join(writes, "\n")
	if !strings.Contains(joined, "/failed-candidate") || !strings.Contains(joined, "/old/rename") || !strings.Contains(joined, "/old/start") || strings.Contains(joined, "DELETE /old") {
		t.Fatalf("unsafe rollback operations: %s", joined)
	}
	if err := m.waitReady(context.Background(), newImage); err == nil {
		t.Fatal("unrelated image attestation accepted")
	}
}

func TestHardwareProbeDoesNotStartStoppedRuntime(t *testing.T) {
	m := testManager(t)
	m.manualStop = true
	m.status.State, m.status.Ready = "stopped", false
	var writes atomic.Int32
	docker := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			writes.Add(1)
		}
		io.WriteString(w, `{"OSType":"unsupported"}`)
	}))
	defer docker.Close()
	m.want.DockerHost = strings.Replace(docker.URL, "http://", "tcp://", 1)
	if err := m.Action("probe"); err != nil {
		t.Fatal(err)
	}
	m.reconcile()
	if !m.manualStop || m.Status().State != "stopped" || writes.Load() != 0 || m.forceQualification {
		t.Fatalf("hardware probe started qualification: %+v", m.Status())
	}
}

func TestReadinessRejectsCrashLoop(t *testing.T) {
	m := testManager(t)
	api := httptest.NewServer(http.NotFoundHandler())
	defer api.Close()
	m.baseURL = api.URL
	docker := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"Id": "worker", "RestartCount": 3,
			"Config": map[string]any{"Labels": dockerutil.ManagedLabels(Owner, "music", "runtime", "test")},
			"State":  map[string]any{"Running": true}})
	}))
	defer docker.Close()
	m.docker = dockerutil.NewClient(strings.Replace(docker.URL, "http://", "tcp://", 1), time.Second)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := m.waitReady(ctx, "unused"); err == nil || err.Error() != "acestep_start_failed" {
		t.Fatalf("crash loop was not rejected: %v", err)
	}
}
