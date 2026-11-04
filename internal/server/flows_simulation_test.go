package server

import (
	"fmt"
	"net/http"
	"testing"
)

func TestFlowSimulationAPI(t *testing.T) {
	s, token := newFlowsTestServer(t)
	rec := createTestFlow(t, s, waitFlowJSON)
	path := "/api/desktop/flows/" + rec.ID + "/simulate"
	for _, body := range []string{`{"simulation":true}`, `{"simulate":true}`, `{"mode":"simulation"}`} {
		w := flowsCall(t, s, http.MethodPost, "/api/desktop/flows/"+rec.ID+"/test", token, body)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("mislabeled real test = %d %s", w.Code, w.Body.String())
		}
	}
	call := func(body string) map[string]any {
		t.Helper()
		w := flowsCall(t, s, http.MethodPost, path, token, body)
		if w.Code != http.StatusOK {
			t.Fatalf("simulate = %d %s", w.Code, w.Body.String())
		}
		return flowsBody(t, w)
	}
	if w := flowsCall(t, s, http.MethodPost, path, token, `{}`); w.Code != http.StatusBadRequest {
		t.Fatalf("simulation without revision = %d %s", w.Code, w.Body.String())
	}
	missing := call(fmt.Sprintf(`{"base_revision":%d}`, rec.DraftRevision))
	if missing["mode"] != "simulation" || missing["status"] != "error" || missing["error_code"] != "FLOW_SIMULATION_FIXTURE_REQUIRED" {
		t.Fatalf("missing fixture = %v", missing)
	}
	simulated := call(fmt.Sprintf(`{"base_revision":%d,"fixtures":{"n_bbbbbbbb":{"output":{"waited":true}}}}`, rec.DraftRevision))
	if simulated["mode"] != "simulation" || simulated["status"] != "success" {
		t.Fatalf("fixture = %v", simulated)
	}
	list := flowsCall(t, s, http.MethodGet, "/api/desktop/flows/"+rec.ID+"/runs", token, "")
	if runs := flowsBody(t, list)["runs"].([]any); len(runs) != 0 {
		t.Fatalf("simulation persisted %d runs", len(runs))
	}
	for body, status := range map[string]int{
		`{"base_revision":999}`: http.StatusConflict,
		fmt.Sprintf(`{"base_revision":%d,"fixtures":{"n_bbbbbbbb":{"error_code":"invalid"}}}`, rec.DraftRevision): http.StatusBadRequest,
	} {
		w := flowsCall(t, s, http.MethodPost, path, token, body)
		if w.Code != status {
			t.Errorf("%s = %d %s", body, w.Code, w.Body.String())
		}
	}
}
