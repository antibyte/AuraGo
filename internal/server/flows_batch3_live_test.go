package server

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"aurago/internal/flows"
	"aurago/internal/tools"
)

const b3SecretFlowJSON = `{"schema":1,"name":"Partner","nodes":[
 {"id":"n_aaaaaaaa","key":"mail","type":"trigger.email","type_version":1,"label":"Mail","position":{"x":0,"y":0},"params":{}},
 {"id":"n_bbbbbbbb","key":"call","type":"http.request","type_version":1,"label":"Call","position":{"x":300,"y":0},
  "params":{"url":"{{trigger.data.link}}","auth_secret":"partner_token"}}],
 "edges":[{"id":"e_aaaaaaaa","source":{"node":"n_aaaaaaaa","port":"out"},"target":{"node":"n_bbbbbbbb","port":"in"}}]}`

// R2b: a live revision published under older rules shows live_issues in the list and the flow
// record, and Mission Control notes it on the flow mission.
func TestB3FlowsAPIReportsLiveIssues(t *testing.T) {
	s, token := newFlowsTestServer(t)
	ctx := context.Background()
	good := c16PublishedFlow(t, s, greetFlowJSON)
	bad := createTestFlow(t, s, b3SecretFlowJSON)
	if _, err := s.Flows.Store().Publish(ctx, bad.ID, bad.DraftRevision, time.Now()); err != nil {
		t.Fatalf("store publish: %v", err)
	}
	reports, err := s.Flows.CheckLiveRevisions(ctx)
	if err != nil || len(reports) != 1 || reports[0].FlowID != bad.ID {
		t.Fatalf("CheckLiveRevisions = %+v, %v; want the one flow published under older rules", reports, err)
	}
	list := flowsBody(t, flowsCall(t, s, http.MethodGet, "/api/desktop/flows", token, ""))
	cards, _ := list["flows"].([]any)
	issuesOf := map[string]int{}
	for _, c := range cards {
		card, _ := c.(map[string]any)
		id, _ := card["id"].(string)
		issues, _ := card["live_issues"].([]any)
		issuesOf[id] = len(issues)
	}
	if issuesOf[bad.ID] == 0 || issuesOf[good.ID] != 0 {
		t.Errorf("live issues per flow = %v, want some for %s only", issuesOf, bad.ID)
	}
	one := flowsBody(t, flowsCall(t, s, http.MethodGet, "/api/desktop/flows/"+bad.ID, token, ""))
	if issues, _ := one["live_issues"].([]any); len(issues) == 0 {
		t.Errorf("GET flow live_issues = %v, want the findings", one["live_issues"])
	}
	m, _ := s.MissionManagerV2.Get(bad.MissionID)
	if m.LastResult != tools.MissionResultError || !strings.Contains(m.LastOutput, "publish it again") {
		t.Errorf("Mission Control shows %q %q, want the republish note", m.LastResult, m.LastOutput)
	}
}

// R2b: publishing the flow again takes the republish note off its mission.
func TestB3RepublishClearsTheLiveRevisionNote(t *testing.T) {
	s, _ := newFlowsTestServer(t)
	ctx := context.Background()
	bad := createTestFlow(t, s, b3SecretFlowJSON)
	if _, err := s.Flows.Store().Publish(ctx, bad.ID, bad.DraftRevision, time.Now()); err != nil {
		t.Fatalf("store publish: %v", err)
	}
	if _, err := s.Flows.CheckLiveRevisions(ctx); err != nil {
		t.Fatal(err)
	}
	if m, _ := s.MissionManagerV2.Get(bad.MissionID); m.LastOutput == "" {
		t.Fatal("the note did not appear")
	}
	fixed := greetFlowJSON
	cur, err := s.Flows.GetFlow(ctx, bad.ID)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := flows.ParseFlow([]byte(fixed))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	rev, _, err := s.Flows.SaveDraft(ctx, bad.ID, doc, cur.DraftRevision)
	if err != nil {
		t.Fatalf("SaveDraft: %v", err)
	}
	if _, _, err := s.Flows.Publish(ctx, bad.ID, rev); err != nil {
		t.Fatalf("Publish: %v", err)
	}
	if m, _ := s.MissionManagerV2.Get(bad.MissionID); m.LastResult != "" || m.LastOutput != "" {
		t.Errorf("Mission Control still shows %q %q after the republish", m.LastResult, m.LastOutput)
	}
}
