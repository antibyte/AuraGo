package tools

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func awaitIntegration(t *testing.T, ch <-chan struct{}) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(3 * time.Second):
		t.Fatal("integration did not stop")
	}
}

func TestMissionManagerStopDrainsInvocationsAndRejectsNewWork(t *testing.T) {
	m := NewMissionManagerV2(t.TempDir(), nil)
	m.missions["m"] = &MissionV2{ID: "m", Name: "shutdown", Enabled: true, Prompt: "test", Priority: "normal"}
	started, cancelled, release := make(chan struct{}), make(chan struct{}), make(chan struct{})
	m.SetCallback(func(_ string, id string) {
		close(started)
		<-m.Context().Done()
		close(cancelled)
		<-release
		m.SetResult(id, "error", "cancelled during shutdown")
	})
	parent, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := m.StartContext(parent); err != nil {
		t.Fatal(err)
	}
	if err := m.TriggerMission("m", "manual", ""); err != nil {
		t.Fatal(err)
	}
	awaitIntegration(t, started)
	cancel()
	awaitIntegration(t, cancelled)
	stopped := make(chan struct{})
	go func() { m.Stop(); close(stopped) }()
	select {
	case <-stopped:
		t.Fatal("Stop returned with an active callback")
	case <-time.After(20 * time.Millisecond):
	}
	close(release)
	awaitIntegration(t, stopped)
	if m.runAsync(func() { t.Error("work after stop") }) {
		t.Fatal("accepted work after stop")
	}
	if err := m.TriggerMission("m", "manual", ""); err == nil {
		t.Fatal("trigger accepted after stop")
	}
	if err := m.Start(); err == nil {
		t.Fatal("a stopped manager restarted")
	}
	m.Stop()
}

func TestHomeAssistantPollerEscapesEntityAndCancelsActiveRequest(t *testing.T) {
	started, cancelled := make(chan struct{}), make(chan struct{})
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.EscapedPath() != "/api/states/sensor.office%2Fdoor" {
			t.Errorf("unescaped path: %s", r.URL.EscapedPath())
		}
		close(started)
		<-r.Context().Done()
		close(cancelled)
	}))
	defer remote.Close()
	m := NewMissionManagerV2(t.TempDir(), nil)
	defer m.Stop()
	m.missions["ha"] = &MissionV2{ID: "ha", Enabled: true, ExecutionType: ExecutionTriggered, TriggerType: TriggerHomeAssistantState, TriggerConfig: &TriggerConfig{HAEntityID: "sensor.office/door"}}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() {
		defer close(done)
		StartHomeAssistantPoller(ctx, HAConfig{URL: remote.URL, AccessToken: "fixture-ha-token"}, m, slog.New(slog.NewTextHandler(io.Discard, nil)))
	}()
	awaitIntegration(t, started)
	cancel()
	awaitIntegration(t, cancelled)
	awaitIntegration(t, done)
}

func TestHomeAssistantCredentialsDoNotFollowForeignRedirect(t *testing.T) {
	var requests atomic.Int32
	foreign := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { requests.Add(1) }))
	defer foreign.Close()
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, foreign.URL, 302) }))
	defer origin.Close()
	if _, _, err := haRequestContext(context.Background(), HAConfig{URL: origin.URL, AccessToken: "fixture-ha-token"}, "GET", "/api/states", ""); err == nil {
		t.Fatal("foreign redirect accepted")
	}
	if requests.Load() != 0 {
		t.Fatal("foreign origin contacted")
	}
}
