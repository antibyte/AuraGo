package server

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"aurago/internal/services"
	"aurago/internal/tools"
)

// Mission Control (desktop) reaches missions through /api/desktop/integrations/missions/v2/…,
// which runs under a Desktop grant. Its owner path (QueueOwnedMission) queues agent missions;
// a flow mission must start a flow run instead, through RunNow or TriggerMission.

// mergeIntegrationsHandler is the integrations route over the real mission router, as
// server_routes.go wires it.
func mergeIntegrationsHandler(s *Server) http.HandlerFunc {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/missions/v2/", handleMissionV2ByID(s))
	return desktopIntegrationHandler(s, mux)
}

// mergeIntegrationsCall posts to /api/desktop/integrations/missions/v2/<id>/<action> with an
// admin desktop token.
func mergeIntegrationsCall(t *testing.T, s *Server, token, missionID, action, body string) *httptest.ResponseRecorder {
	t.Helper()
	var r *http.Request
	path := "/api/desktop/integrations/missions/v2/" + missionID + "/" + action
	if body != "" {
		r = httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
	} else {
		r = httptest.NewRequest(http.MethodPost, path, nil)
	}
	r.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	mergeIntegrationsHandler(s)(w, r)
	return w
}

// mergeWaitFlowRuns waits until the flow mission counted n finished runs, the last one a
// success, and is idle again.
func mergeWaitFlowRuns(t *testing.T, s *Server, missionID string, n int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		m, _ := s.MissionManagerV2.Get(missionID)
		if m != nil && m.RunCount == n && m.LastResult == tools.MissionResultSuccess && m.Status == tools.MissionStatusIdle {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("flow mission after the integrations call = %+v, want %d successful runs", m, n)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// mergeExpectNoAgentWork fails when the agent ran a mission or the agent queue holds one.
func mergeExpectNoAgentWork(t *testing.T, s *Server, agent <-chan string) {
	t.Helper()
	select {
	case id := <-agent:
		t.Fatalf("the agent ran mission %s", id)
	case <-time.After(100 * time.Millisecond):
	}
	queue, running := s.MissionManagerV2.GetQueue()
	if items := queue.List(); len(items) != 0 || running != "" {
		t.Fatalf("agent queue = %+v, running %q; a flow mission never enters it", items, running)
	}
}

func TestMergeIntegrationsRunAndTriggerStartTheFlow(t *testing.T) {
	s, token := newFlowsTestServer(t)
	rec := c16PublishedFlow(t, s, greetFlowJSON)
	if err := s.Flows.SetEnabled(context.Background(), rec.ID, true); err != nil {
		t.Fatalf("SetEnabled: %v", err)
	}
	agent := make(chan string, 4)
	s.MissionManagerV2.SetCallback(func(_ string, id string) { agent <- id })

	w := mergeIntegrationsCall(t, s, token, rec.MissionID, "run", "")
	if w.Code != http.StatusOK {
		t.Fatalf("integrations run = %d %s", w.Code, w.Body.String())
	}
	mergeWaitFlowRuns(t, s, rec.MissionID, 1)
	mergeExpectNoAgentWork(t, s, agent)

	w = mergeIntegrationsCall(t, s, token, rec.MissionID, "trigger", `{"trigger_data":"{\"name\":\"Mars\"}"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("integrations trigger = %d %s", w.Code, w.Body.String())
	}
	mergeWaitFlowRuns(t, s, rec.MissionID, 2)
	mergeExpectNoAgentWork(t, s, agent)
}

// A disabled flow is refused like a disabled agent mission, and nothing is queued.
func TestMergeIntegrationsRunRefusesADisabledFlow(t *testing.T) {
	s, token := newFlowsTestServer(t)
	rec := c16PublishedFlow(t, s, greetFlowJSON)
	agent := make(chan string, 4)
	s.MissionManagerV2.SetCallback(func(_ string, id string) { agent <- id })
	w := mergeIntegrationsCall(t, s, token, rec.MissionID, "run", "")
	if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "disabled") {
		t.Fatalf("integrations run of a disabled flow = %d %s, want 400 disabled", w.Code, w.Body.String())
	}
	mergeExpectNoAgentWork(t, s, agent)
	if m, _ := s.MissionManagerV2.Get(rec.MissionID); m.RunCount != 0 || m.Status != tools.MissionStatusIdle {
		t.Fatalf("flow mission after the refused run = %+v", m)
	}
}

// Cancel through the integrations route reaches the flow's live run (desktopStop admission).
func TestMergeIntegrationsCancelStopsTheFlowRun(t *testing.T) {
	s, token := newFlowsTestServer(t)
	rec := c16PublishedFlow(t, s, waitFlowJSON)
	if err := s.Flows.SetEnabled(context.Background(), rec.ID, true); err != nil {
		t.Fatalf("SetEnabled: %v", err)
	}
	if w := mergeIntegrationsCall(t, s, token, rec.MissionID, "run", ""); w.Code != http.StatusOK {
		t.Fatalf("integrations run = %d %s", w.Code, w.Body.String())
	}
	c16WaitStatus(t, s, rec.MissionID, tools.MissionStatusRunning)
	w := mergeIntegrationsCall(t, s, token, rec.MissionID, "cancel", "")
	if w.Code != http.StatusAccepted {
		t.Fatalf("integrations cancel = %d %s, want 202", w.Code, w.Body.String())
	}
	c16WaitStatus(t, s, rec.MissionID, tools.MissionStatusIdle)
}

// Mission preparation builds context for a mission's prompt; a flow mission has none, so both
// the mission API and Mission Control's integrations route refuse it with a clear 400.
func TestMergePrepareRefusesFlowMissions(t *testing.T) {
	s, token := newFlowsTestServer(t)
	s.Cfg.MissionPreparation.Enabled = true
	s.PreparationService = services.NewMissionPreparationService(s.Cfg, &s.CfgMu, nil, s.MissionManagerV2, slog.New(slog.NewTextHandler(io.Discard, nil)))
	rec := createTestFlow(t, s, greetFlowJSON)
	w := httptest.NewRecorder()
	handleMissionPrepare(s, w, httptest.NewRequest(http.MethodPost, "/api/missions/v2/"+rec.MissionID+"/prepare", nil), rec.MissionID)
	if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "managed in EasyDrag") {
		t.Fatalf("prepare of a flow mission = %d %s, want 400 managed in EasyDrag", w.Code, w.Body.String())
	}
	if w := mergeIntegrationsCall(t, s, token, rec.MissionID, "prepare", ""); w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "managed in EasyDrag") {
		t.Fatalf("integrations prepare of a flow mission = %d %s", w.Code, w.Body.String())
	}
	if _, err := s.PreparationService.PrepareMission(context.Background(), rec.MissionID); err != tools.ErrFlowMissionManaged {
		t.Fatalf("PrepareMission(flow) = %v, want ErrFlowMissionManaged", err)
	}
}
