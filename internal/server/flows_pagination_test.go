package server

import (
	"net/http"
	"testing"
)

func TestFlowRunPagination(t *testing.T) {
	s, token := newFlowsTestServer(t)
	rec := createTestFlow(t, s, greetFlowJSON)
	for range 2 {
		waitFlowRunStatus(t, s, token, c18StartTestRun(t, s, token, rec, ""), "success")
	}
	base := "/api/desktop/flows/" + rec.ID + "/runs?mode=test&limit=1"
	first := flowsCall(t, s, http.MethodGet, base, token, "")
	page := flowsBody(t, first)
	if first.Code != http.StatusOK || page["has_more"] != true || page["next_offset"] != float64(1) || len(page["runs"].([]any)) != 1 {
		t.Fatalf("first page = %d %s", first.Code, first.Body.String())
	}
	second := flowsCall(t, s, http.MethodGet, base+"&offset=1", token, "")
	last := flowsBody(t, second)
	if second.Code != http.StatusOK || last["has_more"] != false || last["next_offset"] != nil || len(last["runs"].([]any)) != 1 {
		t.Fatalf("last page = %d %s", second.Code, second.Body.String())
	}
	if page["runs"].([]any)[0].(map[string]any)["id"] == last["runs"].([]any)[0].(map[string]any)["id"] {
		t.Fatal("pages contain the same run")
	}
}
