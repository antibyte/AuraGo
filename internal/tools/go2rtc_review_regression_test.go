package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"aurago/internal/config"
)

func TestGo2RTCMediaRevisionNeverReturnsAfterRemoval(t *testing.T) {
	for _, remove := range []bool{false, true} {
		t.Run(strconv.FormatBool(remove), func(t *testing.T) {
			m, cfg := mediaTestManager(t, false)
			cfg.Directories.DataDir = t.TempDir()
			m.Configure(cfg)
			old := cfg.Go2RTC.Streams[0]
			lease, err := m.AcquireMedia(context.Background(), old.ID, "main", false)
			if err != nil {
				t.Fatal(err)
			}
			defer lease.Release()
			other := m.StreamRevision("two")
			if err := m.RememberHLS(old.ID, "main", "ABCDEFGH", lease.Revision); err != nil {
				t.Fatal(err)
			}
			if remove {
				cfg.Go2RTC.Streams = cfg.Go2RTC.Streams[1:]
			} else {
				cfg.Go2RTC.Streams[0].Enabled = false
			}
			m.Configure(cfg)
			select {
			case <-lease.Context.Done():
			case <-time.After(time.Second):
				t.Fatal("removed camera did not cancel its old lease")
			}
			if m.MediaRevisionCurrent(old.ID, lease.Revision) {
				t.Fatal("removed camera kept its old lease")
			}
			replacement := old
			replacement.Source = "rtsp://192.168.1.3/replacement"
			if remove {
				cfg.Go2RTC.Streams = append(cfg.Go2RTC.Streams, replacement)
			} else {
				cfg.Go2RTC.Streams[0] = replacement
			}
			m.Configure(cfg)
			if _, err := m.ReconcileStreams(context.Background()); err != nil {
				t.Fatal(err)
			}
			if m.MediaRevisionCurrent(old.ID, lease.Revision) || m.StreamRevision(old.ID) <= lease.Revision || m.mediaAliases(old.ID)["main"] == lease.Alias {
				t.Fatal("revoked camera generation was reused")
			}
			if m.StreamRevision("two") != other {
				t.Fatal("unrelated camera changed")
			}
			if hls, err := m.HLSLease(context.Background(), "ABCDEFGH"); err == nil {
				hls.Release()
				t.Fatal("old HLS session became valid again")
			}
			if _, err := m.buildSnapshotResult(old, go2RTCTestJPEG(16, 8), Go2RTCSnapshotOptions{Store: true}, lease.Revision); err == nil {
				t.Fatal("late snapshot from revoked source was published")
			}
		})
	}
}

func TestGo2RTCPreloadCleanupRetriesFailures(t *testing.T) {
	for _, action := range []string{"source", "disable_preload", "shutdown"} {
		t.Run(action, func(t *testing.T) {
			var mu sync.Mutex
			aliases, preloads := map[string]bool{}, map[string]bool{}
			failDelete, oldAlias := false, ""
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				mu.Lock()
				defer mu.Unlock()
				name := r.URL.Query().Get("src")
				switch strings.TrimPrefix(r.URL.Path, "/api/go2rtc/proxy") {
				case "/api/streams":
					switch r.Method {
					case "GET":
						json.NewEncoder(w).Encode(aliases)
					case "PATCH":
						aliases[r.URL.Query().Get("name")] = true
					case "DELETE":
						delete(aliases, name)
					}
				case "/api/preload":
					switch r.Method {
					case "GET":
						json.NewEncoder(w).Encode(preloads)
					case "PUT":
						preloads[name] = true
					case "DELETE":
						if failDelete && name == oldAlias {
							w.WriteHeader(503)
							return
						}
						if !preloads[name] {
							w.WriteHeader(500)
							return
						} // pinned upstream's absent-preload response
						delete(preloads, name)
					}
				}
			}))
			defer upstream.Close()
			u, _ := url.Parse(upstream.URL)
			port, _ := strconv.Atoi(u.Port())
			cfg := &config.Config{Go2RTC: config.Go2RTCConfig{Enabled: true, URL: upstream.URL, APIHostPort: port, APIPassword: "fixture", Streams: []config.Go2RTCStreamConfig{{ID: "one", Enabled: true, Preload: true, Source: "rtsp://192.168.1.2/old"}}}}
			m := NewGo2RTCManager(cfg, nil, nil, nil)
			defer m.Close()
			if _, err := m.ReconcileStreams(context.Background()); err != nil {
				t.Fatal(err)
			}
			oldAlias = m.mediaAliases("one")["main"]
			mu.Lock()
			failDelete = true
			mu.Unlock()
			apply := func() error { _, err := m.ReconcileStreams(context.Background()); return err }
			switch action {
			case "source":
				cfg.Go2RTC.Streams[0].Source = "rtsp://192.168.1.3/new"
				m.Configure(cfg)
			case "disable_preload":
				cfg.Go2RTC.Streams[0].Preload = false
				m.Configure(cfg)
			case "shutdown":
				apply = func() error { return m.clearPreloads(context.Background()) }
			}
			if err := apply(); err == nil {
				t.Fatal("failed cleanup was hidden")
			}
			mu.Lock()
			failDelete = false
			mu.Unlock()
			if err := apply(); err != nil {
				t.Fatal(err)
			}
			mu.Lock()
			orphan := preloads[oldAlias]
			oldExists := aliases[oldAlias]
			warm := len(preloads)
			mu.Unlock()
			if orphan || action == "source" && (oldExists || warm != 1) || action != "source" && warm != 0 {
				t.Fatalf("cleanup lost: old preload=%v alias=%v warm=%d", orphan, oldExists, warm)
			}
			if err := apply(); err != nil {
				t.Fatalf("absent cleanup is not idempotent: %v", err)
			}
		})
	}
}

func TestGo2RTCHardwareProbeRejectsChangedIdentity(t *testing.T) {
	for _, change := range []string{"host", "url", "host_aba", "late_failure"} {
		t.Run(change, func(t *testing.T) {
			entered, finish := make(chan struct{}), make(chan struct{})
			var mu sync.Mutex
			calls := 0
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				mu.Lock()
				calls++
				first := calls == 1
				mu.Unlock()
				if first {
					close(entered)
					<-finish
					if change == "late_failure" {
						w.WriteHeader(503)
						return
					}
				}
				w.Header().Set("Content-Type", "application/json")
				w.Write([]byte(`{"sources":[{"name":"OK","url":"ffmpeg:fixture#video=h264#hardware=vaapi"}]}`))
			}))
			defer upstream.Close()
			u, _ := url.Parse(upstream.URL)
			port, _ := strconv.Atoi(u.Port())
			cfg := &config.Config{Go2RTC: config.Go2RTCConfig{Enabled: true, URL: upstream.URL, APIHostPort: port, APIPassword: "fixture", Hardware: config.Go2RTCHardwareConfig{Backend: "vaapi", RenderNode: "/dev/dri/renderD128", RenderGroup: "993"}}}
			cfg.Directories.DataDir = t.TempDir()
			cfg.Docker.Host = "tcp://192.168.1.10:2375"
			m := NewGo2RTCManager(cfg, nil, nil, nil)
			defer m.Close()
			result := make(chan Go2RTCHardwareResult, 1)
			go func() { r, _ := m.CheckHardware(context.Background()); result <- r }()
			<-entered
			if change == "url" {
				cfg.Go2RTC.URL += "/replacement"
			} else {
				cfg.Docker.Host = "tcp://192.168.1.20:2375"
			}
			m.Configure(cfg)
			if change == "host_aba" {
				cfg.Docker.Host = "tcp://192.168.1.10:2375"
				m.Configure(cfg)
			}
			if change == "late_failure" {
				if r, err := m.CheckHardware(context.Background()); err != nil || !r.Verified {
					close(finish)
					t.Fatalf("new identity could not be verified: %+v %v", r, err)
				}
			}
			close(finish)
			if probe := <-result; probe.Verified || probe.ErrorCode != "hardware_changed" {
				t.Fatalf("stale probe accepted: %+v", probe)
			}
			wantVerified := change == "late_failure"
			if m.HardwareStatus().Verified != wantVerified {
				t.Fatal("stale probe changed the new identity's verification")
			}
			reloaded := NewGo2RTCManager(cfg, nil, nil, nil)
			defer reloaded.Close()
			if reloaded.HardwareStatus().Verified != wantVerified {
				t.Fatal("stale probe changed the persisted verification")
			}
		})
	}
}

func TestGo2RTCHardwareProbeDoesNotWaitForContainerLifecycle(t *testing.T) {
	m, cfg := mediaTestManager(t, false)
	cfg.Go2RTC.Hardware.Backend = "vaapi"
	m.Configure(cfg)
	m.lifecycleMu.Lock()
	defer m.lifecycleMu.Unlock()
	done := make(chan Go2RTCHardwareResult, 1)
	go func() { result, _ := m.CheckHardware(context.Background()); done <- result }()
	select {
	case result := <-done:
		if result.Verified || result.ErrorCode != "hardware_busy" {
			t.Fatalf("busy lifecycle accepted a probe: %+v", result)
		}
	case <-time.After(time.Second):
		t.Fatal("hardware probe waited behind a possibly slow container operation")
	}
}
