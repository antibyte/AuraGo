package server

import (
	"context"
	"net/http"
	"strconv"
	"testing"
)

func TestFB4TestRunRefusesAStaleBaseRevision(t *testing.T) {
	s, token := newFlowsTestServer(t)
	rec := createTestFlow(t, s, greetFlowJSON)
	stale := rec.DraftRevision
	fresh, _, err := s.Flows.SaveDraft(context.Background(), rec.ID, rec.Draft, rec.DraftRevision) // another window saved
	if err != nil {
		t.Fatal(err)
	}
	path := "/api/desktop/flows/" + rec.ID + "/test"
	if w := flowsCall(t, s, http.MethodPost, path, token, `{"base_revision":`+strconv.Itoa(stale)+`}`); w.Code != http.StatusConflict ||
		flowsBody(t, w)["code"] != "FLOW_REVISION_CONFLICT" {
		t.Fatalf("stale test = %d %s", w.Code, w.Body.String())
	}
	if w := flowsCall(t, s, http.MethodPost, path, token, `{"base_revision":`+strconv.Itoa(fresh)+`}`); w.Code != http.StatusAccepted {
		t.Fatalf("current test = %d %s", w.Code, w.Body.String())
	}
}
