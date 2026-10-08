package server

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"aurago/internal/flows"
)

// Audit 2026-10-08, finding 1.2: each reason why the flows API cannot serve a request has a
// code of its own, so the editor can say what happened. FLOWS_DISABLED is left for flows
// that are switched off or not available at all (flows.enabled, missions off, a store that
// did not open).

// audit12Expect checks an answer's status and code.
func audit12Expect(t *testing.T, what string, w *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	body := flowsBody(t, w)
	if msg, _ := body["error"].(string); w.Code != status || body["code"] != code || msg == "" {
		t.Fatalf("%s = %d %s, want %d %s", what, w.Code, w.Body.String(), status, code)
	}
}

// The mapper gives every cause its code, wrapped or not.
func TestAudit12ErrorMapNamesEachUnavailableCause(t *testing.T) {
	s, _ := newFlowsTestServer(t)
	for _, tc := range []struct {
		err  error
		code string
	}{
		{flows.ErrMissionControlUnavailable, "FLOW_MISSION_CONTROL_UNAVAILABLE"},
		{fmt.Errorf("run now: %w", flows.ErrMissionControlUnavailable), "FLOW_MISSION_CONTROL_UNAVAILABLE"},
		{flows.ErrRunnerClosed, "FLOW_RUNNER_STOPPED"},
		{fmt.Errorf("start: %w", flows.ErrRunnerClosed), "FLOW_RUNNER_STOPPED"},
	} {
		w := httptest.NewRecorder()
		s.flowsErrorFrom(w, httptest.NewRequest(http.MethodPost, "/api/desktop/flows/flow_x/run", nil), tc.err)
		audit12Expect(t, tc.err.Error(), w, http.StatusServiceUnavailable, tc.code)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	w := httptest.NewRecorder()
	s.flowsErrorFrom(w, httptest.NewRequest(http.MethodGet, "/api/desktop/flows", nil).WithContext(ctx), context.Canceled)
	audit12Expect(t, "a cancelled request", w, http.StatusServiceUnavailable, "FLOW_REQUEST_CANCELLED")
}

// The same causes through the API routes, next to the flows that are switched off.
func TestAudit12APIRoutesNameEachUnavailableCause(t *testing.T) {
	s, token := newFlowsTestServer(t)
	ctx := context.Background()
	rec := createTestFlow(t, s, greetFlowJSON)
	if _, _, err := s.Flows.Publish(ctx, rec.ID, rec.DraftRevision); err != nil {
		t.Fatalf("Publish: %v", err)
	}
	if err := s.Flows.SetEnabled(ctx, rec.ID, true); err != nil {
		t.Fatalf("SetEnabled: %v", err)
	}

	// A write the server cancelled while the client still waits (the shutdown drain, a
	// revoked Desktop grant).
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	r := httptest.NewRequest(http.MethodDelete, "/api/desktop/flows/"+rec.ID, nil).WithContext(cancelled)
	r.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	s.handleFlows(w, r)
	audit12Expect(t, "a cancelled delete", w, http.StatusServiceUnavailable, "FLOW_REQUEST_CANCELLED")

	// The flow secrets without a vault.
	vault := s.Vault
	s.Vault = nil
	w = flowsCall(t, s, http.MethodGet, "/api/desktop/flows/secrets", token, "")
	s.Vault = vault
	audit12Expect(t, "the secrets without a vault", w, http.StatusServiceUnavailable, "FLOW_VAULT_UNAVAILABLE")

	// Run now without Mission Control.
	mm := s.MissionManagerV2
	s.MissionManagerV2 = nil
	w = flowsCall(t, s, http.MethodPost, "/api/desktop/flows/"+rec.ID+"/run", token, "")
	s.MissionManagerV2 = mm
	audit12Expect(t, "run now without Mission Control", w, http.StatusServiceUnavailable, "FLOW_MISSION_CONTROL_UNAVAILABLE")

	// Run now while the flow service shuts down.
	if err := s.Flows.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
	w = flowsCall(t, s, http.MethodPost, "/api/desktop/flows/"+rec.ID+"/run", token, "")
	audit12Expect(t, "run now after the runner stopped", w, http.StatusServiceUnavailable, "FLOW_RUNNER_STOPPED")

	// Flows switched off keep FLOWS_DISABLED.
	s.Cfg.Flows.Enabled = false
	w = flowsCall(t, s, http.MethodGet, "/api/desktop/flows", token, "")
	audit12Expect(t, "flows switched off", w, http.StatusServiceUnavailable, "FLOWS_DISABLED")
}
