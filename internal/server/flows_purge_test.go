package server

import (
	"context"
	"net/http"
	"strconv"
	"testing"
	"time"
)

func TestFlowsRollbackOfAMarkedVersionAnswersFlowVersionPurged(t *testing.T) {
	s, token := newFlowsTestServer(t)
	ctx := context.Background()
	rec := createTestFlow(t, s, greetFlowJSON)
	first, _, err := s.Flows.Publish(ctx, rec.ID, rec.DraftRevision)
	if err != nil {
		t.Fatal(err)
	}
	first.Draft.Name = "Zweite"
	draft, _, err := s.Flows.SaveDraft(ctx, rec.ID, first.Draft, first.DraftRevision)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.Flows.Publish(ctx, rec.ID, draft); err != nil { // revisions 1 and 2
		t.Fatal(err)
	}
	if err := s.Flows.Store().MarkVersionsForPurge(ctx, rec.ID, []int{1}, time.Now()); err != nil {
		t.Fatal(err)
	}
	w := flowsCall(t, s, http.MethodPost, "/api/desktop/flows/"+rec.ID+"/versions", token,
		`{"revision":1,"base_revision":`+strconv.Itoa(draft)+`}`)
	if w.Code != http.StatusConflict || flowsBody(t, w)["code"] != "FLOW_VERSION_PURGED" {
		t.Fatalf("rollback of a marked version = %d %s, want 409 FLOW_VERSION_PURGED", w.Code, w.Body.String())
	}
}
