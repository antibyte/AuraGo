package server

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/tools"
)

func TestFritzWidgetRetiresClientOnlyAfterLastReader(t *testing.T) {
	first, second := &fakeFritzBackend{}, &fakeFritzBackend{}
	cache := newFritzWidgetCache(nil)
	created := 0
	cache.newBackend = func(*config.Config) (fritzWidgetBackend, error) {
		created++
		if created == 1 {
			return first, nil
		}
		return second, nil
	}
	cfg := fritzWidgetTestConfig()
	one, err := cache.backendFor(cfg)
	if err != nil {
		t.Fatal(err)
	}
	two, err := cache.backendFor(cfg)
	if err != nil {
		t.Fatal(err)
	}
	changed := cfg.Clone()
	changed.FritzBox.Password = "rotated-fixture-password"
	three, err := cache.backendFor(changed)
	if err != nil {
		t.Fatal(err)
	}
	if first.closed.Load() != 0 || created != 2 {
		t.Fatal("active old client closed or changed credential reused")
	}
	cache.releaseBackend(one)
	if first.closed.Load() != 0 {
		t.Fatal("client closed while another reader uses it")
	}
	cache.releaseBackend(two)
	if first.closed.Load() != 1 {
		t.Fatal("retired client not closed exactly once")
	}
	done := make(chan struct{})
	go func() { cache.close(); close(done) }()
	select {
	case <-done:
		t.Fatal("shutdown returned with active reader")
	case <-time.After(20 * time.Millisecond):
	}
	if second.closed.Load() != 0 {
		t.Fatal("shutdown closed active client")
	}
	cache.releaseBackend(three)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("shutdown did not finish")
	}
	if second.closed.Load() != 1 {
		t.Fatal("last client not closed exactly once")
	}
	if _, err := cache.backendFor(cfg); err == nil {
		t.Fatal("client created after shutdown")
	}
}

func TestFritzPollerConfigPublicationCancelsOldGeneration(t *testing.T) {
	started, cancelled := make(chan struct{}), make(chan struct{})
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		close(started)
		<-r.Context().Done()
		close(cancelled)
	}))
	defer remote.Close()
	u, _ := url.Parse(remote.URL)
	port, _ := strconv.Atoi(u.Port())
	cfg := fritzWidgetTestConfig()
	cfg.FritzBox.Host = u.Hostname()
	cfg.FritzBox.Port = port
	cfg.FritzBox.WebPort = port
	cfg.FritzBox.Telephony.Enabled = true
	cfg.FritzBox.Telephony.Polling.Enabled = true
	m := tools.NewMissionManagerV2(t.TempDir(), nil)
	defer m.Stop()
	s := &Server{Cfg: cfg, Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), MissionManagerV2: m, integrationCtx: context.Background()}
	s.initConfigSnapshot()
	s.configureFritzPoller()
	defer s.stopFritzPoller()
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("poller not started")
	}
	changed := cfg.Clone()
	changed.FritzBox.Enabled = false
	s.CfgMu.Lock()
	s.replaceConfigSnapshot(changed)
	s.CfgMu.Unlock()
	select {
	case <-cancelled:
	case <-time.After(3 * time.Second):
		t.Fatal("config publication did not cancel request")
	}
	s.configureFritzPoller()
	if s.fritzPoller.Load() != nil {
		t.Fatal("disabled poller restarted")
	}
	s.stopFritzPoller()
	s.CfgMu.Lock()
	s.replaceConfigSnapshot(cfg)
	s.CfgMu.Unlock()
	s.configureFritzPoller()
	if s.fritzPoller.Load() != nil {
		t.Fatal("shutdown allowed a new poller")
	}
}
