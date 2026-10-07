package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// A run cancel stops work. Main's Desktop contract lets stop and cancel routes through a
// read-only desktop (desktopStop, as Mission Control's cancel); read-only missions still
// refuse it, mirroring CancelCheck, and the FF1 admin bearer scope still applies.

// fuStartWaitingRun publishes and switches on the waiting flow, starts a live run and waits
// until it runs. It returns the run id.
func fuStartWaitingRun(t *testing.T, s *Server, token string) string {
	t.Helper()
	rec := createTestFlow(t, s, waitFlowJSON)
	if w := flowsCall(t, s, http.MethodPost, "/api/desktop/flows/"+rec.ID+"/publish", token, `{"base_revision":1}`); w.Code != http.StatusOK {
		t.Fatalf("publish = %d %s", w.Code, w.Body.String())
	}
	if w := flowsCall(t, s, http.MethodPost, "/api/desktop/flows/"+rec.ID+"/enabled", token, `{"enabled":true}`); w.Code != http.StatusOK {
		t.Fatalf("enable = %d %s", w.Code, w.Body.String())
	}
	w := flowsCall(t, s, http.MethodPost, "/api/desktop/flows/"+rec.ID+"/run", token, "")
	if w.Code != http.StatusAccepted {
		t.Fatalf("run = %d %s", w.Code, w.Body.String())
	}
	runID := flowsBody(t, w)["run_id"].(string)
	waitFlowRunStatus(t, s, token, runID, "running")
	return runID
}

func TestFUReadOnlyDesktopStillCancelsFlowRuns(t *testing.T) {
	s, token := newFlowsTestServer(t)
	runID := fuStartWaitingRun(t, s, token)
	s.Cfg.VirtualDesktop.ReadOnly = true

	// Other writes stay refused with the flows API's code.
	if w := flowsCall(t, s, http.MethodPost, "/api/desktop/flows", token, `{"name":"x"}`); w.Code != http.StatusForbidden || flowsBody(t, w)["code"] != "FLOW_PERMISSION_DENIED" {
		t.Fatalf("create on a read-only desktop = %d %s", w.Code, w.Body.String())
	}
	// The cancel still needs the admin scope (FF1).
	writeToken, _, err := s.TokenManager.Create("fu write", []string{desktopScopeRead, desktopScopeWrite}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if w := flowsCall(t, s, http.MethodPost, "/api/desktop/flows/runs/"+runID+"/cancel", writeToken, ""); w.Code != http.StatusForbidden {
		t.Fatalf("cancel with a write token = %d %s, want 403", w.Code, w.Body.String())
	}
	if w := flowsCall(t, s, http.MethodPost, "/api/desktop/flows/runs/"+runID+"/cancel", token, ""); w.Code != http.StatusAccepted || flowsBody(t, w)["cancelled"] != true {
		t.Fatalf("cancel on a read-only desktop = %d %s, want 202", w.Code, w.Body.String())
	}
	waitFlowRunStatus(t, s, token, runID, "cancelled")
}

func TestFUReadOnlyMissionsRefuseFlowRunCancel(t *testing.T) {
	s, token := newFlowsTestServer(t)
	runID := fuStartWaitingRun(t, s, token)
	s.Cfg.Tools.Missions.ReadOnly = true
	w := flowsCall(t, s, http.MethodPost, "/api/desktop/flows/runs/"+runID+"/cancel", token, "")
	if w.Code != http.StatusForbidden || flowsBody(t, w)["code"] != "FLOW_PERMISSION_DENIED" {
		t.Fatalf("cancel with read-only missions = %d %s, want 403 FLOW_PERMISSION_DENIED", w.Code, w.Body.String())
	}
	s.Cfg.Tools.Missions.ReadOnly = false
	if w := flowsCall(t, s, http.MethodPost, "/api/desktop/flows/runs/"+runID+"/cancel", token, ""); w.Code != http.StatusAccepted {
		t.Fatalf("cancel after missions are writable again = %d %s", w.Code, w.Body.String())
	}
	waitFlowRunStatus(t, s, token, runID, "cancelled")
}

func TestFUFlowsDesktopOperation(t *testing.T) {
	for _, tc := range []struct {
		method, path string
		want         desktopOperation
	}{
		{http.MethodGet, "/api/desktop/flows/f1", desktopRead},
		{http.MethodHead, "/api/desktop/flows/node-types", desktopRead},
		{http.MethodPost, "/api/desktop/flows/validate", desktopRead},
		{http.MethodPost, "/api/desktop/flows/runs/run_a/cancel", desktopStop},
		{http.MethodGet, "/api/desktop/flows/runs/run_a/cancel", desktopRead},
		{http.MethodPost, "/api/desktop/flows/runs/run_a/cancel/x", desktopWrite},
		{http.MethodPost, "/api/desktop/flows/f1/cancel", desktopWrite},
		{http.MethodPost, "/api/desktop/flows/f1/run", desktopWrite},
		{http.MethodDelete, "/api/desktop/flows/f1", desktopWrite},
	} {
		r := httptest.NewRequest(tc.method, tc.path, nil)
		if got := flowsDesktopOperation(r, flowsPathParts(tc.path)); got != tc.want {
			t.Errorf("flowsDesktopOperation(%s %s) = %v, want %v", tc.method, tc.path, got, tc.want)
		}
	}
}
