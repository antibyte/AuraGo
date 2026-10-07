package server

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"aurago/internal/tools"
)

func waitFlowRunStatus(t *testing.T, s *Server, token, runID, want string) map[string]any {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		w := flowsCall(t, s, http.MethodGet, "/api/desktop/flows/runs/"+runID, token, "")
		if w.Code == http.StatusOK {
			body := flowsBody(t, w)
			if run, _ := body["run"].(map[string]any); run != nil && run["status"] == want {
				return body
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("run %s did not reach %s: %d %s", runID, want, w.Code, w.Body.String())
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestFlowsAPITestRunAndEvents(t *testing.T) {
	s, token := newFlowsTestServer(t)
	rec := createTestFlow(t, s, greetFlowJSON)
	w := flowsCall(t, s, http.MethodPost, "/api/desktop/flows/"+rec.ID+"/test", token, `{"trigger_data":{"name":"Andi"},"remember_data":true}`)
	if w.Code != http.StatusAccepted {
		t.Fatalf("test = %d %s", w.Code, w.Body.String())
	}
	runID := flowsBody(t, w)["run_id"].(string)
	detail := waitFlowRunStatus(t, s, token, runID, "success")
	if steps, _ := detail["steps"].([]any); len(steps) == 0 {
		t.Fatalf("run detail without steps: %+v", detail)
	}
	ev := flowsCall(t, s, http.MethodGet, "/api/desktop/flows/runs/"+runID+"/events", token, "")
	body := ev.Body.String()
	if ev.Code != http.StatusOK || !strings.Contains(ev.Header().Get("Content-Type"), "text/event-stream") ||
		!strings.Contains(body, "event: snapshot") || !strings.Contains(body, "event: end") {
		t.Fatalf("events = %d %q", ev.Code, body)
	}
	sample := flowsBody(t, flowsCall(t, s, http.MethodGet, "/api/desktop/flows/"+rec.ID+"/test-data/n_aaaaaaaa", token, ""))
	if data, _ := sample["data"].(map[string]any); data["name"] != "Andi" {
		t.Fatalf("remembered sample = %+v", sample)
	}
	if runs := flowsBody(t, flowsCall(t, s, http.MethodGet, "/api/desktop/flows/"+rec.ID+"/runs?mode=test", token, ""))["runs"].([]any); len(runs) != 1 {
		t.Fatalf("runs = %+v", runs)
	}
	if w := flowsCall(t, s, http.MethodPost, "/api/desktop/flows/runs/run_unknown1234/cancel", token, ""); w.Code != http.StatusNotFound {
		t.Fatalf("cancel unknown = %d", w.Code)
	}
	if w := flowsCall(t, s, http.MethodGet, "/api/desktop/flows/runs/run_unknown1234", token, ""); w.Code != http.StatusNotFound {
		t.Fatalf("unknown run = %d %s", w.Code, w.Body.String())
	}
}

func TestFlowsAPILiveRunReachesMissionControl(t *testing.T) {
	s, token := newFlowsTestServer(t)
	rec := createTestFlow(t, s, greetFlowJSON)
	if w := flowsCall(t, s, http.MethodPost, "/api/desktop/flows/"+rec.ID+"/run", token, ""); w.Code != http.StatusConflict ||
		flowsBody(t, w)["code"] != "FLOW_NOT_PUBLISHED" {
		t.Fatalf("run before publish = %d %s", w.Code, w.Body.String())
	}
	if w := flowsCall(t, s, http.MethodPost, "/api/desktop/flows/"+rec.ID+"/publish", token, `{"base_revision":1}`); w.Code != http.StatusOK {
		t.Fatalf("publish = %d %s", w.Code, w.Body.String())
	}
	if w := flowsCall(t, s, http.MethodPost, "/api/desktop/flows/"+rec.ID+"/enabled", token, `{"enabled":true}`); w.Code != http.StatusOK { // FF1: Run now needs the flow switched on
		t.Fatalf("enable = %d %s", w.Code, w.Body.String())
	}
	w := flowsCall(t, s, http.MethodPost, "/api/desktop/flows/"+rec.ID+"/run", token, "")
	if w.Code != http.StatusAccepted {
		t.Fatalf("run = %d %s", w.Code, w.Body.String())
	}
	waitFlowRunStatus(t, s, token, flowsBody(t, w)["run_id"].(string), "success")
	deadline := time.Now().Add(3 * time.Second)
	for {
		m, _ := s.MissionManagerV2.Get(rec.MissionID)
		if m.RunCount == 1 && m.LastResult == tools.MissionResultSuccess && m.Status == tools.MissionStatusIdle {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("mission after the live run = %+v", m)
		}
		time.Sleep(10 * time.Millisecond)
	}
}
