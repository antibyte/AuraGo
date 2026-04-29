package server

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
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
