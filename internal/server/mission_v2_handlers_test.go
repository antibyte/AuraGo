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
	"time"

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

func TestHandleMissionTriggerV2ReturnsSkippedWhenRateLimited(t *testing.T) {
	allowMissionMutationsForTest(t)

	mgr := tools.NewMissionManagerV2(t.TempDir(), nil)
	if err := mgr.Create(&tools.MissionV2{
		ID:            "mission_rate_limit",
		Name:          "Rate limit",
		Prompt:        "run",
		ExecutionType: tools.ExecutionTriggered,
		TriggerType:   tools.TriggerWebhook,
		TriggerConfig: &tools.TriggerConfig{
			WebhookID:          "hook-1",
			MinIntervalSeconds: 60,
		},
		Enabled: true,
	}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := mgr.TriggerMission("mission_rate_limit", "api", `{"first":true}`); err != nil {
		t.Fatalf("first trigger: %v", err)
	}

	s := &Server{MissionManagerV2: mgr}
	body, _ := json.Marshal(map[string]string{"trigger_data": `{"second":true}`})
	req := httptest.NewRequest(http.MethodPost, "/api/missions/v2/mission_rate_limit/trigger", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handleMissionTriggerV2(s, rr, req, "mission_rate_limit")

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s, want 200", rr.Code, rr.Body.String())
	}
	var resp map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["status"] != "skipped" || resp["reason"] != "rate_limited" {
		t.Fatalf("response = %#v, want skipped/rate_limited", resp)
	}
}

func TestHandleMissionRunV2ReturnsRunningForRemoteMission(t *testing.T) {
	allowMissionMutationsForTest(t)

	mgr := tools.NewMissionManagerV2(t.TempDir(), nil)
	mgr.SetRemoteMissionClient(&fakeHandlerRemoteMissionClient{})
	if err := mgr.Create(&tools.MissionV2{
		ID:            "mission_remote_run",
		Name:          "Remote run",
		Prompt:        "run",
		ExecutionType: tools.ExecutionManual,
		RunnerType:    tools.MissionRunnerRemote,
		RemoteNestID:  "nest-1",
		RemoteEggID:   "egg-1",
		Enabled:       true,
	}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	s := &Server{MissionManagerV2: mgr}
	req := httptest.NewRequest(http.MethodPost, "/api/missions/v2/mission_remote_run/run", nil)
	rr := httptest.NewRecorder()
	handleMissionRunV2(s, rr, req, "mission_remote_run")

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s, want 200", rr.Code, rr.Body.String())
	}
	var resp map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["status"] != "running" {
		t.Fatalf("response = %#v, want running", resp)
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

func TestMissionV2PayloadsIncludeNextRunForScheduledMissions(t *testing.T) {
	allowMissionMutationsForTest(t)
	dir := t.TempDir()
	cronMgr := tools.NewCronManager(dir)
	cronMgr.Start(func(string) {})
	t.Cleanup(func() { _ = cronMgr.Close() })
	mgr := tools.NewMissionManagerV2(dir, cronMgr)
	if err := mgr.Create(&tools.MissionV2{ID: "m_sched", Name: "Sched", Prompt: "p", ExecutionType: tools.ExecutionScheduled, Schedule: "0 9 * * *", Enabled: true}); err != nil {
		t.Fatalf("create scheduled: %v", err)
	}
	if err := mgr.Create(&tools.MissionV2{ID: "m_manual", Name: "Manual", Prompt: "p", ExecutionType: tools.ExecutionManual, Enabled: true}); err != nil {
		t.Fatalf("create manual: %v", err)
	}
	s := &Server{MissionManagerV2: mgr}

	rr := httptest.NewRecorder()
	handleListMissionsV2(s).ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/missions/v2", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("list status %d: %s", rr.Code, rr.Body.String())
	}
	var list struct {
		Missions []map[string]json.RawMessage `json:"missions"`
		Queue    map[string]json.RawMessage   `json:"queue"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &list); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(list.Missions) != 2 {
		t.Fatalf("expected 2 missions, got %d", len(list.Missions))
	}
	if _, ok := list.Queue["items"]; !ok {
		t.Fatalf("queue.items missing from list payload")
	}
	for _, m := range list.Missions {
		var id string
		_ = json.Unmarshal(m["id"], &id)
		_, hasNext := m["next_run"]
		switch id {
		case "m_sched":
			if !hasNext {
				t.Fatalf("scheduled mission must include next_run")
			}
			var next time.Time
			if err := json.Unmarshal(m["next_run"], &next); err != nil || next.IsZero() {
				t.Fatalf("next_run must be an RFC3339 timestamp, got %s (%v)", m["next_run"], err)
			}
		case "m_manual":
			if hasNext {
				t.Fatalf("manual mission must omit next_run")
			}
		default:
			t.Fatalf("unexpected mission id %q", id)
		}
	}

	rr = httptest.NewRecorder()
	handleMissionGetV2(s, rr, httptest.NewRequest(http.MethodGet, "/api/missions/v2/m_sched", nil), "m_sched")
	var single map[string]json.RawMessage
	if err := json.Unmarshal(rr.Body.Bytes(), &single); err != nil {
		t.Fatalf("decode single: %v", err)
	}
	if _, ok := single["next_run"]; !ok {
		t.Fatalf("by-id payload must include next_run for scheduled mission")
	}
	if _, ok := single["name"]; !ok {
		t.Fatalf("by-id payload must keep the embedded mission fields")
	}
}

func TestMissionV2BroadcastIncludesNextRunForScheduledMissions(t *testing.T) {
	allowMissionMutationsForTest(t)
	dir := t.TempDir()
	cronMgr := tools.NewCronManager(dir)
	if err := cronMgr.Start(func(string) {}); err != nil {
		t.Fatalf("start cron: %v", err)
	}
	t.Cleanup(func() { _ = cronMgr.Close() })
	mgr := tools.NewMissionManagerV2(dir, cronMgr)
	if err := mgr.Create(&tools.MissionV2{ID: "m_sched", Name: "Sched", Prompt: "p", ExecutionType: tools.ExecutionScheduled, Schedule: "0 9 * * *", Enabled: true}); err != nil {
		t.Fatalf("create scheduled: %v", err)
	}
	// Create always enables a mission; disable it through Update afterwards.
	disabled := &tools.MissionV2{ID: "m_disabled", Name: "Disabled", Prompt: "p", ExecutionType: tools.ExecutionScheduled, Schedule: "0 9 * * *"}
	if err := mgr.Create(disabled); err != nil {
		t.Fatalf("create disabled: %v", err)
	}
	disabled.Enabled = false
	if err := mgr.Update("m_disabled", disabled); err != nil {
		t.Fatalf("disable: %v", err)
	}
	sse := NewSSEBroadcaster()
	events := sse.subscribe()
	t.Cleanup(func() { sse.unsubscribe(events) })
	s := &Server{MissionManagerV2: mgr, SSE: sse}

	broadcastMissionState(s)

	var raw string
	select {
	case raw = <-events:
	case <-time.After(2 * time.Second):
		t.Fatal("no mission_update event broadcast")
	}
	var event struct {
		Type    string `json:"type"`
		Payload struct {
			Missions []map[string]json.RawMessage `json:"missions"`
			Queue    map[string]json.RawMessage   `json:"queue"`
		} `json:"payload"`
	}
	if err := json.Unmarshal([]byte(raw), &event); err != nil {
		t.Fatalf("decode event: %v (%s)", err, raw)
	}
	if event.Type != string(EventMissionUpdate) {
		t.Fatalf("event type = %q, want %q", event.Type, EventMissionUpdate)
	}
	if _, ok := event.Payload.Queue["items"]; !ok {
		t.Fatalf("queue.items missing from broadcast payload")
	}
	if len(event.Payload.Missions) != 2 {
		t.Fatalf("expected 2 missions in broadcast, got %d", len(event.Payload.Missions))
	}
	for _, m := range event.Payload.Missions {
		var id string
		_ = json.Unmarshal(m["id"], &id)
		next, hasNext := m["next_run"]
		switch id {
		case "m_sched":
			if !hasNext {
				t.Fatalf("broadcast must include next_run for enabled scheduled mission")
			}
			var parsed time.Time
			if err := json.Unmarshal(next, &parsed); err != nil || parsed.IsZero() {
				t.Fatalf("next_run must be an RFC3339 timestamp, got %s (%v)", next, err)
			}
		case "m_disabled":
			if hasNext {
				t.Fatalf("broadcast must omit next_run for disabled scheduled mission, got %s", next)
			}
		default:
			t.Fatalf("unexpected mission id %q", id)
		}
	}
}

func TestHandleMissionCancelV2(t *testing.T) {
	allowMissionMutationsForTest(t)
	mgr := tools.NewMissionManagerV2(t.TempDir(), nil)
	if err := mgr.Create(&tools.MissionV2{ID: "m_cancel", Name: "Cancel", Prompt: "p", ExecutionType: tools.ExecutionManual, Enabled: true}); err != nil {
		t.Fatalf("create: %v", err)
	}
	mgr.SetRemoteMissionClient(&fakeHandlerRemoteMissionClient{})
	if err := mgr.Create(&tools.MissionV2{ID: "m_remote", Name: "Remote", Prompt: "p", ExecutionType: tools.ExecutionManual, Enabled: true, RunnerType: tools.MissionRunnerRemote, RemoteNestID: "n", RemoteEggID: "e"}); err != nil {
		t.Fatalf("create remote: %v", err)
	}
	s := &Server{MissionManagerV2: mgr}
	post := func(id string) *httptest.ResponseRecorder {
		rr := httptest.NewRecorder()
		handleMissionV2ByID(s).ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/missions/v2/"+id+"/cancel", nil))
		return rr
	}

	if rr := post("missing"); rr.Code != http.StatusNotFound {
		t.Fatalf("unknown mission: status %d body %s", rr.Code, rr.Body.String())
	}
	if rr := post("m_cancel"); rr.Code != http.StatusConflict {
		t.Fatalf("idle mission: expected 409, got %d body %s", rr.Code, rr.Body.String())
	}
	if rr := post("m_remote"); rr.Code != http.StatusBadRequest || !strings.Contains(rr.Body.String(), "not supported") {
		t.Fatalf("remote mission: expected 400 'not supported', got %d body %s", rr.Code, rr.Body.String())
	}

	rr := httptest.NewRecorder()
	handleMissionV2ByID(s).ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/missions/v2/m_cancel/cancel", nil))
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET cancel: expected 405, got %d", rr.Code)
	}
}

// TestHandleMissionCancelV2RunningMission drives the manager through its real
// queue dispatcher (Start + RunNow with a no-op callback) so the mission
// genuinely reaches the running state that the cancel route requires.
func TestHandleMissionCancelV2RunningMission(t *testing.T) {
	allowMissionMutationsForTest(t)
	mgr := tools.NewMissionManagerV2(t.TempDir(), nil)
	if err := mgr.Create(&tools.MissionV2{ID: "m_cancel", Name: "Cancel", Prompt: "p", ExecutionType: tools.ExecutionManual, Enabled: true}); err != nil {
		t.Fatalf("create: %v", err)
	}
	// The callback stands in for the chat completion; it never completes, so
	// the mission stays running until the test ends.
	mgr.SetCallback(func(prompt, missionID string) {})
	if err := mgr.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(mgr.Stop)
	if err := mgr.RunNow("m_cancel"); err != nil {
		t.Fatalf("RunNow: %v", err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		if mission, ok := mgr.Get("m_cancel"); ok && mission.Status == tools.MissionStatusRunning {
			break
		}
		if time.Now().After(deadline) {
			mission, _ := mgr.Get("m_cancel")
			t.Fatalf("mission did not reach running state via the queue dispatcher (status %q)", mission.Status)
		}
		time.Sleep(50 * time.Millisecond)
	}

	s := &Server{MissionManagerV2: mgr}
	post := func() *httptest.ResponseRecorder {
		rr := httptest.NewRecorder()
		handleMissionV2ByID(s).ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/missions/v2/m_cancel/cancel", nil))
		return rr
	}

	// Running per the manager, but the chat handler has not registered a
	// context yet: the run cannot be cancelled at this moment.
	if rr := post(); rr.Code != http.StatusConflict || !strings.Contains(rr.Body.String(), "cannot be cancelled yet") {
		t.Fatalf("running mission without registered context: expected 409 'cannot be cancelled yet', got %d %s", rr.Code, rr.Body.String())
	}

	ctx, release := s.missionRunTracker().begin("m_cancel")
	defer release()
	rr := post()
	if rr.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d body %s", rr.Code, rr.Body.String())
	}
	var body map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil || body["status"] != "cancelling" {
		t.Fatalf("expected {\"status\":\"cancelling\"}, got %s", rr.Body.String())
	}
	if ctx.Err() == nil {
		t.Fatalf("cancel must cancel the registered run context")
	}
	// A repeated cancel of the same run is not a second cancellation.
	if rr := post(); rr.Code != http.StatusConflict {
		t.Fatalf("repeated cancel: expected 409, got %d body %s", rr.Code, rr.Body.String())
	}
	// The completion callback classifies the failure exactly once.
	if !s.missionRunTracker().consumeCancelled("m_cancel") {
		t.Fatalf("cancelled run must be reported to the completion callback")
	}
	if s.missionRunTracker().consumeCancelled("m_cancel") {
		t.Fatalf("cancellation must be consumed only once")
	}
}
