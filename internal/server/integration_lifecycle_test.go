package server

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/tools"
)

func TestMissionRunShutdownCancelsAndWaitsForAllGenerations(t *testing.T) {
	s := &Server{}
	first, releaseFirst := missionRunBaseContext(s, "same")
	second, releaseSecond := missionRunBaseContext(s, "same")
	if first.Err() == nil {
		t.Fatal("replaced run remains active")
	}
	done := make(chan struct{})
	go func() { s.missionRunTracker().close(); close(done) }()
	select {
	case <-second.Done():
	case <-time.After(time.Second):
		t.Fatal("current run not cancelled")
	}
	releaseSecond()
	select {
	case <-done:
		t.Fatal("shutdown did not await replaced handler")
	case <-time.After(20 * time.Millisecond):
	}
	releaseFirst()
	releaseFirst()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("shutdown did not finish")
	}
	third, release := missionRunBaseContext(s, "new")
	defer release()
	if third.Err() == nil {
		t.Fatal("new run accepted after shutdown")
	}
}

func TestHomeAssistantConfigPublicationRevokesPoller(t *testing.T) {
	tools.ClearRuntimePermissionsForTest()
	tools.ConfigureRuntimePermissions(tools.RuntimePermissions{MissionsEnabled: true})
	t.Cleanup(tools.ClearRuntimePermissionsForTest)
	started, cancelled := make(chan struct{}), make(chan struct{})
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { close(started); <-r.Context().Done(); close(cancelled) }))
	defer remote.Close()
	cfg := &config.Config{}
	cfg.HomeAssistant.Enabled = true
	cfg.HomeAssistant.URL = remote.URL
	cfg.HomeAssistant.AccessToken = "fixture-ha-token"
	m := tools.NewMissionManagerV2(t.TempDir(), nil)
	defer m.Stop()
	if err := m.Create(&tools.MissionV2{ID: "ha", Name: "HA test", Enabled: true, ExecutionType: tools.ExecutionTriggered, TriggerType: tools.TriggerHomeAssistantState, TriggerConfig: &tools.TriggerConfig{HAEntityID: "sensor.office"}}); err != nil {
		t.Fatal(err)
	}
	s := &Server{Cfg: cfg, Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), MissionManagerV2: m, integrationCtx: context.Background()}
	s.initConfigSnapshot()
	s.configureHomeAssistantPoller()
	defer s.stopHomeAssistantPoller()
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("poll not started")
	}
	changed := cfg.Clone()
	changed.HomeAssistant.AccessToken = "rotated-fixture-token"
	s.CfgMu.Lock()
	s.replaceConfigSnapshot(changed)
	s.CfgMu.Unlock()
	select {
	case <-cancelled:
	case <-time.After(3 * time.Second):
		t.Fatal("old token request survived publication")
	}
	s.stopHomeAssistantPoller()
	s.configureHomeAssistantPoller()
	if s.haPoller.Load() != nil {
		t.Fatal("poller restarted after shutdown")
	}
}
