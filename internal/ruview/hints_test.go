package ruview

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"

	"aurago/internal/config"
)

type hintCall struct {
	event string
	open  bool
}

type hintLog struct {
	mu    sync.Mutex
	calls []hintCall
}

func (h *hintLog) add(event string, open bool) {
	h.mu.Lock()
	h.calls = append(h.calls, hintCall{event, open})
	h.mu.Unlock()
}

func (h *hintLog) snapshot() []hintCall {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]hintCall(nil), h.calls...)
}

func requireHints(t *testing.T, got, want []hintCall) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("hints = %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("hints = %+v, want %+v", got, want)
		}
	}
}

func TestRuViewOperationalHintsFireOncePerTransition(t *testing.T) {
	t.Run("sensing", func(t *testing.T) {
		var phase atomic.Value
		phase.Store("fresh")
		m, c := fixture(t, func(w http.ResponseWriter, r *http.Request) {
			if phase.Load().(string) == "down" {
				http.Error(w, "unavailable", http.StatusServiceUnavailable)
				return
			}
			body := `{"status":"ready","source":"esp32","source_state":"live_unverified","last_frame_age_ms":20}`
			switch phase.Load().(string) {
			case "stale":
				body = `{"status":"ready","source":"esp32","source_state":"stale","last_frame_age_ms":30000}`
			case "old":
				body = `{"status":"ready","source":"esp32","source_state":"live_unverified","last_frame_age_ms":15000}`
			case "wifi":
				body = `{"status":"ready","source":"wifi:hidden","source_state":"stale","last_frame_age_ms":30000}`
			case "sim":
				body = `{"status":"ready","source":"simulated","source_state":"synthetic"}`
			}
			_, _ = io.WriteString(w, body)
		})
		log := &hintLog{}
		m.SetOperationalHint(log.add)
		poll := func(n int) {
			t.Helper()
			for i := 0; i < n; i++ {
				_, err := m.Test(context.Background())
				if phase.Load().(string) == "down" {
					if err == nil {
						t.Fatal("disconnected probe succeeded")
					}
					continue
				}
				if err != nil {
					t.Fatal(err)
				}
			}
		}
		poll(3)
		requireHints(t, log.snapshot(), nil)
		phase.Store("stale")
		poll(3)
		requireHints(t, log.snapshot(), []hintCall{{hintSensingStale, true}})
		c.RuView.Enabled = false
		if _, err := m.Test(context.Background()); err == nil {
			t.Fatal("disabled probe succeeded")
		}
		c.RuView.Enabled = true
		requireHints(t, log.snapshot(), []hintCall{{hintSensingStale, true}})
		phase.Store("fresh")
		poll(3)
		requireHints(t, log.snapshot(), []hintCall{{hintSensingStale, true}, {hintSensingStale, false}})
		c.RuView.Source = "wifi"
		phase.Store("down")
		poll(3)
		c.RuView.Source = "esp32"
		requireHints(t, log.snapshot(), []hintCall{{hintSensingStale, true}, {hintSensingStale, false}})
		phase.Store("fresh")
		poll(1)
		phase.Store("wifi")
		poll(3)
		phase.Store("sim")
		poll(3)
		requireHints(t, log.snapshot(), []hintCall{{hintSensingStale, true}, {hintSensingStale, false}})
		phase.Store("fresh")
		poll(1)
		phase.Store("old")
		poll(3)
		requireHints(t, log.snapshot(), []hintCall{{hintSensingStale, true}, {hintSensingStale, false}, {hintSensingStale, true}})
		phase.Store("fresh")
		poll(1)
		phase.Store("down")
		poll(3)
		requireHints(t, log.snapshot(), []hintCall{
			{hintSensingStale, true}, {hintSensingStale, false}, {hintSensingStale, true}, {hintSensingStale, false}, {hintSensingStale, true},
		})
		phase.Store("fresh")
		poll(2)
		requireHints(t, log.snapshot(), []hintCall{
			{hintSensingStale, true}, {hintSensingStale, false}, {hintSensingStale, true}, {hintSensingStale, false}, {hintSensingStale, true}, {hintSensingStale, false},
		})
	})

	t.Run("sensing before any live probe", func(t *testing.T) {
		m, _ := fixture(t, func(w http.ResponseWriter, r *http.Request) {
			_, _ = io.WriteString(w, `{"status":"ready","source":"esp32","source_state":"stale","last_frame_age_ms":30000}`)
		})
		log := &hintLog{}
		m.SetOperationalHint(log.add)
		for i := 0; i < 3; i++ {
			if _, err := m.Test(context.Background()); err != nil {
				t.Fatal(err)
			}
		}
		requireHints(t, log.snapshot(), nil)
	})

	t.Run("storage", func(t *testing.T) {
		m, c, log := managedBudget(t, "native")
		m.storageEvery = 0 // sample on every watch
		c.RuView.StorageMB = 1
		data := filepath.Join(c.Directories.DataDir, "ruview", "data")
		if err := os.MkdirAll(data, 0o700); err != nil {
			t.Fatal(err)
		}
		full := filepath.Join(data, "recording.bin")
		if err := os.WriteFile(full, make([]byte, 1<<20+1), 0o600); err != nil {
			t.Fatal(err)
		}
		for i := 0; i < 3; i++ {
			if m.watch(context.Background()) == nil {
				t.Fatal("managed runtime did not schedule a probe")
			}
		}
		requireHints(t, log.snapshot(), []hintCall{{hintStorageLimit, true}})
		c.RuView.Enabled = false
		if m.watch(context.Background()) != nil {
			t.Fatal("disabled runtime stayed scheduled")
		}
		requireHints(t, log.snapshot(), []hintCall{{hintStorageLimit, true}})
		c.RuView.Enabled = true
		m.mu.Lock()
		m.managed = true
		m.runningConfig = c
		m.mu.Unlock()
		if err := os.Remove(full); err != nil {
			t.Fatal(err)
		}
		for i := 0; i < 3; i++ {
			if m.watch(context.Background()) == nil {
				t.Fatal("managed runtime did not schedule a probe")
			}
		}
		requireHints(t, log.snapshot(), []hintCall{{hintStorageLimit, true}, {hintStorageLimit, false}})
	})

	t.Run("external storage", func(t *testing.T) {
		m, _, log := managedBudget(t, "external")
		for i := 0; i < 3; i++ {
			if m.watch(context.Background()) == nil {
				t.Fatal("managed runtime did not schedule a probe")
			}
		}
		requireHints(t, log.snapshot(), nil)
	})

	t.Run("stop", func(t *testing.T) {
		m, p := nativeFixture(t)
		log := &hintLog{}
		m.SetOperationalHint(log.add)
		p.kill = func() error { return errors.New("kill denied") }
		for i := 0; i < 3; i++ {
			if err := m.Stop(context.Background()); err == nil {
				t.Fatal("uncertain stop succeeded")
			}
		}
		requireHints(t, log.snapshot(), []hintCall{{hintStopUnconfirmed, true}})
		m.config().RuView.Enabled = false
		if _, err := m.Test(context.Background()); err == nil {
			t.Fatal("disabled probe succeeded")
		}
		m.config().RuView.Enabled = true
		requireHints(t, log.snapshot(), []hintCall{{hintStopUnconfirmed, true}})
		close(p.done)
		if err := m.Stop(context.Background()); err != nil {
			t.Fatal(err)
		}
		if err := m.Stop(context.Background()); err != nil {
			t.Fatal(err)
		}
		requireHints(t, log.snapshot(), []hintCall{{hintStopUnconfirmed, true}, {hintStopUnconfirmed, false}})
	})

	t.Run("external stop", func(t *testing.T) {
		m, _ := fixture(t, func(w http.ResponseWriter, r *http.Request) {})
		log := &hintLog{}
		m.SetOperationalHint(log.add)
		for i := 0; i < 3; i++ {
			if err := m.Stop(context.Background()); err != nil {
				t.Fatal(err)
			}
		}
		requireHints(t, log.snapshot(), nil)
	})
}

func managedBudget(t *testing.T, mode string) (*Manager, *config.Config, *hintLog) {
	t.Helper()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(upstream.Close)
	c := &config.Config{RuView: config.DefaultRuViewConfig()}
	c.RuView.Enabled = true
	c.RuView.ReadOnly = false
	c.RuView.AllowPrivate = true
	c.RuView.Mode = mode
	c.Directories.DataDir = t.TempDir()
	if mode == "external" {
		c.RuView.URL = upstream.URL
	} else {
		parsed, err := url.Parse(upstream.URL)
		if err != nil {
			t.Fatal(err)
		}
		port, err := strconv.Atoi(parsed.Port())
		if err != nil || port < 1024 {
			t.Fatalf("test server port: %s", parsed.Port())
		}
		c.RuView.HTTPPort = port
	}
	m := New(func() *config.Config { return c }, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
	t.Cleanup(m.Close)
	log := &hintLog{}
	m.SetOperationalHint(log.add)
	m.mu.Lock()
	m.managed = true
	m.runningConfig = c
	m.mu.Unlock()
	return m, c, log
}
