package server

import (
	"aurago/internal/flows"
	"aurago/internal/security"
	"context"
	"net/http"
	"strconv"
	"strings"
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

const tokenFlowJSON = `{"schema":1,"name":"Tokens","nodes":[
 {"id":"n_aaaaaaaa","key":"start","type":"trigger.manual","type_version":1,"label":"Start","position":{"x":0,"y":0},"params":{}},
 {"id":"n_bbbbbbbb","key":"req","type":"http.request","type_version":1,"label":"Anfrage","position":{"x":300,"y":0},
  "params":{"method":"GET","url":"https://api.example.com/v1?api_key=QRYTOKEN-0123456789abcdef","headers":{"Authorization":"Bearer HDRTOKEN-0123456789abcdef"}}}],
 "edges":[{"id":"e_aaaaaaaa","source":{"node":"n_aaaaaaaa","port":"out"},"target":{"node":"n_bbbbbbbb","port":"in"}}]}`

func TestCredentialScanEndpointNeedsAdminAndNeverReturnsAValue(t *testing.T) {
	s, token := newFlowsTestServer(t)
	doc, err := flows.ParseFlow([]byte(tokenFlowJSON))
	if err != nil {
		t.Fatal(err)
	}
	doc.ID = "flow_tokenflow1"
	if _, err := s.Flows.Store().CreateFlow(context.Background(), doc, "", time.Now()); err != nil {
		t.Fatal(err)
	}
	readToken, _, err := s.TokenManager.Create("reader", []string{desktopScopeRead}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if w := flowsCall(t, s, http.MethodGet, "/api/desktop/flows/credentials/scan", readToken, ""); w.Code != http.StatusForbidden {
		t.Fatalf("a read token got %d %s, want 403", w.Code, w.Body.String())
	}
	w := flowsCall(t, s, http.MethodGet, "/api/desktop/flows/credentials/scan?fresh=1", token, "")
	if w.Code != http.StatusOK {
		t.Fatalf("scan = %d %s", w.Code, w.Body.String())
	}
	if strings.Contains(w.Body.String(), "QRYTOKEN") || strings.Contains(w.Body.String(), "HDRTOKEN") {
		t.Fatal("the scan answer carries a credential value")
	}
	report := flowsBody(t, w)["report"].(map[string]any)
	if drafts, _ := report["drafts"].([]any); len(drafts) != 1 {
		t.Fatalf("drafts = %v, want the one flow", report["drafts"])
	}
	if w := flowsCall(t, s, http.MethodPost, "/api/desktop/flows/credentials/scan", token, "{}"); w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST scan = %d, want 405", w.Code)
	}
}

func TestCredentialPurgeEndpointWithAndWithoutAMasterKey(t *testing.T) {
	s, token := newFlowsTestServer(t)
	doc, err := flows.ParseFlow([]byte(tokenFlowJSON))
	if err != nil {
		t.Fatal(err)
	}
	doc.ID = "flow_tokenflow2"
	if _, err := s.Flows.Store().CreateFlow(context.Background(), doc, "", time.Now()); err != nil {
		t.Fatal(err)
	}
	security.RetainProcessMasterKey("")
	if w := flowsCall(t, s, http.MethodPost, "/api/desktop/flows/credentials/purge", token, `{"snapshots":[]}`); w.Code != http.StatusUnprocessableEntity || flowsBody(t, w)["code"] != "FLOW_PURGE_NO_KEY" {
		t.Fatalf("purge without a master key = %d %s, want 422 FLOW_PURGE_NO_KEY", w.Code, w.Body.String())
	}
	security.RetainProcessMasterKey(strings.Repeat("0123456789abcdef", 4))
	t.Cleanup(func() { security.RetainProcessMasterKey("") })
	if w := flowsCall(t, s, http.MethodPost, "/api/desktop/flows/credentials/purge", token, `{"snapshots":[".flows-v7-backup-nothere"]}`); w.Code != http.StatusBadRequest || flowsBody(t, w)["code"] != "FLOW_PURGE_SNAPSHOT_INVALID" {
		t.Fatalf("purge of a missing snapshot = %d %s", w.Code, w.Body.String())
	}
	if w := flowsCall(t, s, http.MethodPost, "/api/desktop/flows/credentials/purge", token, `{"snapshots":[]}`); w.Code != http.StatusAccepted {
		t.Fatalf("purge = %d %s, want 202", w.Code, w.Body.String())
	}
	deadline := time.Now().Add(20 * time.Second)
	for {
		w := flowsCall(t, s, http.MethodGet, "/api/desktop/flows/credentials/purge", token, "")
		state := flowsBody(t, w)["state"].(map[string]any)
		if state["status"] == "done" {
			break
		}
		if state["status"] == "failed" || time.Now().After(deadline) {
			t.Fatalf("purge state = %v", state)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestFlowsBlockVersionOfAMarkedRevisionAnswersFlowVersionPurged(t *testing.T) {
	s, token := newFlowsTestServer(t)
	ctx := context.Background()
	block, err := s.Flows.CreateBlock(ctx, "Marked block")
	if err != nil {
		t.Fatal(err)
	}
	first, _, err := s.Flows.PublishBlock(ctx, block.ID, block.DraftRevision)
	if err != nil {
		t.Fatal(err)
	}
	first.Draft.Name = "Marked block 2"
	draft, _, err := s.Flows.SaveBlockDraft(ctx, block.ID, first.Draft, first.DraftRevision)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.Flows.PublishBlock(ctx, block.ID, draft); err != nil {
		t.Fatal(err)
	}
	path := flowBlocksPath + "/" + block.ID + "/versions/" + strconv.Itoa(first.LiveRevision)
	if w := flowBlockCall(t, s, http.MethodGet, path, token, nil); w.Code != http.StatusOK {
		t.Fatalf("an unmarked block version = %d %s", w.Code, w.Body.String())
	}
	if err := s.Flows.Store().MarkVersionsForPurge(ctx, block.ID, []int{first.LiveRevision}, time.Now()); err != nil {
		t.Fatal(err)
	}
	w := flowBlockCall(t, s, http.MethodGet, path, token, nil)
	if w.Code != http.StatusConflict || flowsBody(t, w)["code"] != "FLOW_VERSION_PURGED" {
		t.Fatalf("a marked block version = %d %s, want 409 FLOW_VERSION_PURGED", w.Code, w.Body.String())
	}
}
