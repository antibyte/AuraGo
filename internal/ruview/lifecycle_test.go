package ruview

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"aurago/internal/config"
)

func nativeFixture(t *testing.T) (*Manager, *nativeProcess) {
	t.Helper()
	c := &config.Config{RuView: config.DefaultRuViewConfig()}
	c.RuView.Enabled, c.RuView.ReadOnly, c.RuView.Mode = true, false, "native"
	c.Directories.DataDir = t.TempDir()
	m := New(func() *config.Config { return c }, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
	p := &nativeProcess{done: make(chan struct{}), kill: func() error { return nil }}
	m.process, m.managed, m.runningConfig = p, true, c
	m.status = Status{Ready: true, Fresh: true, DataState: "live_unverified"}
	return m, p
}

func TestNativeExitReleasesStartAndInvalidatesStatus(t *testing.T) {
	m, p := nativeFixture(t)
	p.err = errors.New("exited")
	close(p.done)
	m.watch(context.Background())
	if s := m.Status(); s.Ready || s.Fresh || s.RevisionVerified || s.Error != "process_exited" {
		t.Fatalf("exit retained live status: %+v", s)
	}
	if m.process != nil || m.runningConfig != nil {
		t.Fatal("exit retained runtime ownership")
	}
	if err := m.Start(context.Background()); err == nil || strings.Contains(err.Error(), "already started") {
		t.Fatalf("restart did not reach normal installation checks: %v", err)
	}
	// A previous completion must not affect a replacement process.
	replacement := &nativeProcess{done: make(chan struct{})}
	m.process, m.managed = replacement, true
	m.mu.Lock()
	m.reapNativeLocked()
	m.mu.Unlock()
	if m.process != replacement || !m.managed {
		t.Fatal("old exit cleared new process")
	}
}

func TestStopRevokesPendingTest(t *testing.T) {
	entered := make(chan struct{})
	m, _ := fixture(t, func(w http.ResponseWriter, r *http.Request) {
		close(entered)
		<-r.Context().Done()
		io.WriteString(w, `{"status":"ready","source":"esp32","source_state":"live_unverified","last_frame_age_ms":0}`)
	})
	done := make(chan error, 1)
	go func() { _, err := m.Test(context.Background()); done <- err }()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("probe not started")
	}
	if err := m.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("revoked probe succeeded")
		}
	case <-time.After(time.Second):
		t.Fatal("probe not revoked")
	}
	if s := m.Status(); s.Ready || s.Fresh || s.DataState != "stopped" {
		t.Fatalf("late probe won: %+v", s)
	}
}

func TestNativeStopFailureAndConcurrentCompletion(t *testing.T) {
	for _, fail := range []bool{true, false} {
		t.Run(map[bool]string{true: "kill-error", false: "timeout"}[fail], func(t *testing.T) {
			m, p := nativeFixture(t)
			if fail {
				p.kill = func() error { return errors.New("kill denied") }
			}
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
			defer cancel()
			if err := m.Stop(ctx); err == nil {
				t.Fatal("uncertain stop succeeded")
			}
			if s := m.Status(); s.Ready || s.Fresh || s.DataState == "stopped" || s.Error != "stop_failed" {
				t.Fatalf("false stop: %+v", s)
			}
			if !m.managed || m.process != p {
				t.Fatal("lost process needed for another stop")
			}
			close(p.done)
			var wg sync.WaitGroup
			for i := 0; i < 3; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					if err := m.Stop(context.Background()); err != nil {
						t.Error(err)
					}
				}()
			}
			wg.Wait()
			if s := m.Status(); s.DataState != "stopped" || s.RevisionVerified {
				t.Fatalf("completion not observed: %+v", s)
			}
		})
	}
}

func TestReadOnlyStopsRemainNarrowAndTimedRecordingIsRejected(t *testing.T) {
	calls := 0
	m, c := fixture(t, func(w http.ResponseWriter, r *http.Request) { calls++; io.WriteString(w, `{"success":true}`) })
	for _, tc := range []struct {
		method, path string
		want         int
	}{
		{"POST", "/api/v1/recording/stop", 200}, {"POST", "/api/v1/train/stop", 200},
		{"POST", "/api/v1/recording/start", 403}, {"POST", "/api/v1/train/start", 403},
		{"POST", "/api/v1/train/pretrain", 403}, {"POST", "/api/v1/train/lora", 403},
		{"POST", "/api/v1/calibration/stop", 403}, {"DELETE", "/api/v1/recording/example", 403},
		{"POST", "/api/v1/train/stop/", 404}, {"GET", "/api/v1/train/stop", 404},
	} {
		w := httptest.NewRecorder()
		m.Proxy(w, httptest.NewRequest(tc.method, "/", strings.NewReader(`{}`)), tc.path)
		if w.Code != tc.want {
			t.Errorf("%s %s: %d", tc.method, tc.path, w.Code)
		}
	}
	c.RuView.ReadOnly = false
	for _, body := range []string{`{"duration_secs":60}`, `{"duration_secs":null}`, `{"duration_secs":0}`} {
		w := httptest.NewRecorder()
		m.Proxy(w, httptest.NewRequest("POST", "/", strings.NewReader(body)), "/api/v1/recording/start")
		if w.Code != 400 {
			t.Fatalf("timed recording accepted: %d", w.Code)
		}
	}
	if calls != 2 {
		t.Fatalf("unexpected upstream mutations: %d", calls)
	}
}
