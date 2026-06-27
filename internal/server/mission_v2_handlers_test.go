package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"aurago/internal/tools"
)

type fakeHandlerRemoteMissionClient struct {
	deleteErr error
}

func (f *fakeHandlerRemoteMissionClient) SyncMission(ctx context.Context, mission tools.MissionV2, promptSnapshot string) error {
	return nil
}

func (f *fakeHandlerRemoteMissionClient) DeleteMission(ctx context.Context, mission tools.MissionV2) error {
	return f.deleteErr
}

func (f *fakeHandlerRemoteMissionClient) RunMission(ctx context.Context, mission tools.MissionV2, triggerType, triggerData string) error {
	return nil
}

func allowMissionMutationsForTest(t *testing.T) {
	t.Helper()
	tools.ConfigureRuntimePermissions(tools.RuntimePermissions{
		AllowShell:           true,
		AllowPython:          true,
		AllowFilesystemWrite: true,
		AllowNetworkRequests: true,
		DockerEnabled:        true,
		SchedulerEnabled:     true,
		MissionsEnabled:      true,
	})
}

func TestHandleMissionDeleteV2UsesMissionErrorStatus(t *testing.T) {
	allowMissionMutationsForTest(t)

	mgr := tools.NewMissionManagerV2(t.TempDir(), nil)
	client := &fakeHandlerRemoteMissionClient{deleteErr: errors.New("remote nest nest-1 is not connected")}
	mgr.SetRemoteMissionClient(client)
	if err := mgr.Create(&tools.MissionV2{
		ID:            "mission_remote_delete",
		Name:          "Remote",
		Prompt:        "x",
		ExecutionType: tools.ExecutionManual,
		Enabled:       true,
		RunnerType:    tools.MissionRunnerRemote,
		RemoteNestID:  "nest-1",
		RemoteEggID:   "egg-1",
	}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	s := &Server{MissionManagerV2: mgr}
	req := httptest.NewRequest(http.MethodDelete, "/api/missions/v2/mission_remote_delete", nil)
	rr := httptest.NewRecorder()

	handleMissionDeleteV2(s, rr, req, "mission_remote_delete")

	if rr.Code != http.StatusConflict {
		t.Fatalf("status = %d body=%s, want 409", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "not connected") {
		t.Fatalf("body = %s, want remote error detail", rr.Body.String())
	}
}

func TestHandleMissionDeleteV2ForceDeletesRemoteMission(t *testing.T) {
	allowMissionMutationsForTest(t)

	mgr := tools.NewMissionManagerV2(t.TempDir(), nil)
	client := &fakeHandlerRemoteMissionClient{deleteErr: errors.New("remote nest nest-1 is not connected")}
	mgr.SetRemoteMissionClient(client)
	if err := mgr.Create(&tools.MissionV2{
		ID:            "mission_remote_force_delete",
		Name:          "Remote",
		Prompt:        "x",
		ExecutionType: tools.ExecutionManual,
		Enabled:       true,
		RunnerType:    tools.MissionRunnerRemote,
		RemoteNestID:  "nest-1",
		RemoteEggID:   "egg-1",
	}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	s := &Server{MissionManagerV2: mgr}
	req := httptest.NewRequest(http.MethodDelete, "/api/missions/v2/mission_remote_force_delete?force=true", nil)
	rr := httptest.NewRecorder()

	handleMissionDeleteV2(s, rr, req, "mission_remote_force_delete")

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s, want 200", rr.Code, rr.Body.String())
	}
	if _, ok := mgr.Get("mission_remote_force_delete"); ok {
		t.Fatal("mission still exists after force delete")
	}
}

func TestMissionV2AcceptsSecondsFieldCron(t *testing.T) {
	allowMissionMutationsForTest(t)

	dir := t.TempDir()
	cronMgr := tools.NewCronManager(dir)
	t.Cleanup(func() { _ = cronMgr.Close() })
	mgr := tools.NewMissionManagerV2(dir, cronMgr)
	s := &Server{
		MissionManagerV2: mgr,
		Logger:           slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})),
	}
	body, _ := json.Marshal(map[string]interface{}{
		"name":           "Seconds cron mission",
		"prompt":         "run the mission",
		"execution_type": "scheduled",
		"schedule":       "0 */15 * * * *",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/missions/v2", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handleCreateMissionV2(s).ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d body=%s, want 201", rr.Code, rr.Body.String())
	}
	if len(cronMgr.GetJobs()) != 1 {
		t.Fatalf("cron jobs = %+v, want one registered scheduled mission", cronMgr.GetJobs())
	}
}

func TestHandleMissionRemoveFromQueuePersistsState(t *testing.T) {
	allowMissionMutationsForTest(t)

	dir := t.TempDir()
	mgr := tools.NewMissionManagerV2(dir, nil)
	if err := mgr.Create(&tools.MissionV2{
		ID:            "mission_queue_remove",
		Name:          "Queue remove",
		Prompt:        "run",
		ExecutionType: tools.ExecutionManual,
		Enabled:       true,
	}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := mgr.RunNow("mission_queue_remove"); err != nil {
		t.Fatalf("RunNow: %v", err)
	}

	s := &Server{MissionManagerV2: mgr}
	req := httptest.NewRequest(http.MethodDelete, "/api/missions/v2/mission_queue_remove/queue", nil)
	rr := httptest.NewRecorder()
	handleMissionRemoveFromQueue(s, rr, req, "mission_queue_remove")

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s, want 200", rr.Code, rr.Body.String())
	}

	restarted := tools.NewMissionManagerV2(dir, nil)
	if err := restarted.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer restarted.Stop()

	got, ok := restarted.Get("mission_queue_remove")
	if !ok {
		t.Fatal("mission missing after restart")
	}
	if got.Status != tools.MissionStatusIdle {
		t.Fatalf("status = %q, want idle", got.Status)
	}
	queue, _ := restarted.GetQueue()
	if len(queue.List()) != 0 {
		t.Fatalf("queue = %+v, want empty after persisted remove", queue.List())
	}
}

func TestHandleMissionRemoveFromQueueReturnsNotFound(t *testing.T) {
	allowMissionMutationsForTest(t)

	mgr := tools.NewMissionManagerV2(t.TempDir(), nil)
	s := &Server{MissionManagerV2: mgr}
	req := httptest.NewRequest(http.MethodDelete, "/api/missions/v2/missing_mission/queue", nil)
	rr := httptest.NewRecorder()
	handleMissionRemoveFromQueue(s, rr, req, "missing_mission")

	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d body=%s, want 404", rr.Code, rr.Body.String())
	}
}
