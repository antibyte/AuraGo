package services

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"

	"aurago/internal/config"
	"aurago/internal/httporigin"
	"aurago/internal/tools"
)

// Same-origin redirect policy: the mission preparation call carries the provider key and
// the mission prompt, so it must not follow a redirect off the configured
// origin. Server A is the configured provider and answers with a 307 to server
// B on another loopback port; B must never be reached.
func TestPrepareMissionDoesNotFollowCrossOriginRedirect(t *testing.T) {
	var secondHits atomic.Int32
	second := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		secondHits.Add(1)
	}))
	defer second.Close()
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", second.URL+r.URL.Path)
		w.WriteHeader(http.StatusTemporaryRedirect)
	}))
	defer provider.Close()

	tools.ConfigureRuntimePermissions(tools.RuntimePermissions{SchedulerEnabled: true, MissionsEnabled: true})
	t.Cleanup(tools.ClearRuntimePermissionsForTest)

	dataDir := t.TempDir()
	cronMgr := tools.NewCronManager(dataDir)
	if err := cronMgr.Start(func(string) {}); err != nil {
		t.Fatalf("start cron manager: %v", err)
	}
	t.Cleanup(func() {
		cronMgr.Stop()
		_ = cronMgr.Close()
	})
	missionMgr := tools.NewMissionManagerV2(dataDir, cronMgr)
	prepDB, err := tools.InitPreparedMissionsDB(filepath.Join(dataDir, "prepared_missions.db"))
	if err != nil {
		t.Fatalf("init prepared missions db: %v", err)
	}
	t.Cleanup(func() { _ = prepDB.Close() })
	missionMgr.SetPreparedDB(prepDB)
	if err := missionMgr.Create(&tools.MissionV2{
		ID:            "mission_redirect_fixture",
		Name:          "Redirect fixture",
		Prompt:        "SECRET-MISSION-PROMPT",
		ExecutionType: tools.ExecutionManual,
		Priority:      "medium",
		Enabled:       true,
	}); err != nil {
		t.Fatalf("create mission: %v", err)
	}

	cfg := &config.Config{}
	cfg.MissionPreparation.Enabled = true
	cfg.MissionPreparation.TimeoutSeconds = 5
	cfg.MissionPreparation.MaxEssentialTools = 5
	cfg.MissionPreparation.APIKey = "sk-mission"
	cfg.MissionPreparation.BaseURL = provider.URL + "/v1"
	cfg.MissionPreparation.Model = "m"
	var cfgMu sync.RWMutex
	service := NewMissionPreparationService(cfg, &cfgMu, prepDB, missionMgr, slog.New(slog.NewTextHandler(io.Discard, nil)))

	_, err = service.PrepareMission(context.Background(), "mission_redirect_fixture")
	if hits := secondHits.Load(); hits != 0 {
		t.Fatalf("second origin received %d request(s)", hits)
	}
	if !errors.Is(err, httporigin.ErrCrossOriginRedirect) {
		t.Fatalf("error = %v, want httporigin.ErrCrossOriginRedirect", err)
	}
}
