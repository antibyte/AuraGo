package flows

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestServiceCreateAndSave(t *testing.T) {
	s, bridge := newServiceFixture(t, nil)
	ctx := context.Background()
	var ve *ValidationError
	if _, err := s.CreateFlow(ctx, CreateRequest{}); !errors.As(err, &ve) {
		t.Fatalf("a flow without a name = %v", err)
	}
	rec, err := s.CreateFlow(ctx, CreateRequest{Name: "Begrüßung"})
	if err != nil {
		t.Fatalf("CreateFlow: %v", err)
	}
	if m, ok := bridge.mission(rec.MissionID); !ok || m.flowID != rec.ID || m.enabled {
		t.Fatalf("mission = %+v %v", m, ok)
	}
	tpl, err := s.CreateFlow(ctx, CreateRequest{Template: "ai_news_pdf_telegram", Translate: func(k string) string { return k }})
	if err != nil || len(tpl.Draft.Nodes) != 5 || tpl.Name != "easydrag.template.ai_news_pdf_telegram.name" {
		t.Fatalf("template flow = %+v, %v", tpl, err)
	}
	imp, err := s.CreateFlow(ctx, CreateRequest{Import: simpleFlow("Import")})
	if err != nil || imp.ID == "flow_test" || len(imp.Draft.Nodes) != 2 {
		t.Fatalf("import = %+v, %v", imp, err)
	}

	rev, _, err := s.SaveDraft(ctx, rec.ID, simpleFlow("Begrüßung"), rec.DraftRevision)
	if err != nil || rev != 2 {
		t.Fatalf("SaveDraft = %d, %v", rev, err)
	}
	saved, _ := s.GetFlow(ctx, rec.ID)
	if saved.Draft.ID != rec.ID || len(saved.Draft.Nodes) != 2 {
		t.Fatalf("saved draft = %+v", saved.Draft)
	}
	bad := simpleFlow("x")
	bad.Nodes[1].Key = "start"
	_, issues, err := s.SaveDraft(ctx, rec.ID, bad, rev)
	if !errors.As(err, &ve) || findIssue(issues, IssueNodeKeyDuplicate, "") == nil {
		t.Fatalf("invalid draft = %v %+v", err, issues)
	}
	bridge.failCreate = errors.New("down")
	if _, err := s.CreateFlow(ctx, CreateRequest{Name: "x"}); err == nil {
		t.Fatal("a failing bridge must fail CreateFlow")
	}
	list, err := s.ListFlows(ctx)
	if err != nil || len(list) != 3 {
		t.Fatalf("ListFlows = %d, %v", len(list), err)
	}
	for _, sum := range list {
		if sum.ID == rec.ID && (len(sum.Preview) != 2 || len(sum.Triggers) != 1 || sum.Triggers[0] != TypeTriggerManual || sum.Enabled || sum.Published) {
			t.Fatalf("summary = %+v", sum)
		}
	}
}

func scheduleFlow(name string) (*Flow, string, string, string) {
	b := newFlow(name)
	manual := b.node("start", TypeTriggerManual, nil)
	sched := b.node("sched", TypeTriggerSchedule, map[string]any{"mode": "daily", "time": "06:00"})
	when := b.node("when", TypeTriggerDateTime, map[string]any{"at": "2026-10-03 09:00"})
	greet := b.node("greet", TypeSet, map[string]any{"fields": []any{map[string]any{"name": "x", "value": "1"}}})
	b.edge(manual, PortOut, greet)
	b.edge(sched, PortOut, greet)
	b.edge(when, PortOut, greet)
	return b.build(), sched, when, greet
}

func TestServicePublishEnableAndPreview(t *testing.T) {
	clock := newFakeClock(time.Date(2026, 10, 3, 7, 0, 0, 0, time.UTC))
	s, bridge := newServiceFixture(t, clock)
	ctx := context.Background()
	rec, _ := s.CreateFlow(ctx, CreateRequest{Name: "Plan"})
	doc, sched, when, greet := scheduleFlow("Plan")
	rev, _, err := s.SaveDraft(ctx, rec.ID, doc, rec.DraftRevision)
	if err != nil {
		t.Fatal(err)
	}
	preview, err := s.PublishPreview(ctx, rec.ID)
	if err != nil || !preview.CanPublish || !preview.Diff.FirstPublish || preview.Diff.AddedNodes != 4 {
		t.Fatalf("preview = %+v, %v", preview, err)
	}
	if _, _, err := s.Publish(ctx, rec.ID, rev-1); !errors.Is(err, ErrRevisionConflict) {
		t.Fatalf("stale publish = %v", err)
	}
	pub, _, err := s.Publish(ctx, rec.ID, rev)
	if err != nil || pub.LiveRevision != 1 {
		t.Fatalf("Publish = %+v, %v", pub, err)
	}
	m, _ := bridge.mission(pub.MissionID)
	if m.name != "Plan" || len(m.bindings) != 3 || m.bindings[1].NodeID != sched || m.bindings[1].Schedule != "0 6 * * *" {
		t.Fatalf("bindings = %+v", m.bindings)
	}
	if timers, _ := s.Store().ListTimers(ctx); len(timers) != 0 {
		t.Fatalf("a disabled flow arms no timers: %+v", timers)
	}
	if err := s.SetEnabled(ctx, rec.ID, true); err != nil {
		t.Fatalf("enable: %v", err)
	}
	timers, _ := s.Store().ListTimers(ctx)
	if m, _ := bridge.mission(pub.MissionID); !m.enabled || len(timers) != 1 || timers[0].NodeID != when ||
		!timers[0].FireAt.Equal(time.Date(2026, 10, 3, 9, 0, 0, 0, time.UTC)) {
		t.Fatalf("enabled: mission %+v timers %+v", m, timers)
	}
	if err := s.SetEnabled(ctx, rec.ID, false); err != nil {
		t.Fatal(err)
	}
	if timers, _ := s.Store().ListTimers(ctx); len(timers) != 0 {
		t.Fatalf("disabling clears timers: %+v", timers)
	}

	doc.NodeByID(greet).Params["fields"] = []any{map[string]any{"name": "x", "value": "2"}}
	doc.NodeByID(sched).Position = Point{X: 999}
	if _, _, err := s.SaveDraft(ctx, rec.ID, doc, pub.DraftRevision); err != nil {
		t.Fatal(err)
	}
	preview, _ = s.PublishPreview(ctx, rec.ID)
	if preview.Diff != (DiffSummary{ChangedNodes: 1}) {
		t.Fatalf("diff = %+v", preview.Diff)
	}

	broken, _ := doc.Clone()
	broken.Nodes = append(broken.Nodes, Node{ID: testNodeID(50), Key: "hook", Type: TypeTriggerWebhook, Params: map[string]any{}})
	broken.Edges = append(broken.Edges, Edge{ID: "e_hook", Source: PortRef{Node: testNodeID(50), Port: PortOut}, Target: PortRef{Node: greet, Port: PortIn}})
	cur, _ := s.GetFlow(ctx, rec.ID)
	rev3, _, err := s.SaveDraft(ctx, rec.ID, broken, cur.DraftRevision)
	if err != nil {
		t.Fatalf("an incomplete draft must still save: %v", err)
	}
	var ve *ValidationError
	if _, issues, err := s.Publish(ctx, rec.ID, rev3); !errors.As(err, &ve) || findIssue(issues, IssueParamRequired, testNodeID(50)) == nil {
		t.Fatalf("publish of an incomplete draft = %v %+v", err, issues)
	}
	other, _ := s.CreateFlow(ctx, CreateRequest{Name: "Neu"})
	if err := s.SetEnabled(ctx, other.ID, true); !errors.Is(err, ErrNotPublished) {
		t.Fatalf("enable unpublished = %v", err)
	}
}

func TestServiceDelete(t *testing.T) {
	s, bridge := newServiceFixture(t, nil)
	ctx := context.Background()
	rec, _ := s.CreateFlow(ctx, CreateRequest{Name: "Weg"})
	if err := s.DeleteFlow(ctx, rec.ID); err != nil {
		t.Fatal(err)
	}
	if _, ok := bridge.mission(rec.MissionID); ok {
		t.Fatal("the mission must be deleted with the flow")
	}
	if _, err := s.GetFlow(ctx, rec.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetFlow after delete = %v", err)
	}
	if err := s.DeleteFlow(ctx, rec.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("second delete = %v", err)
	}
	rec2, _ := s.CreateFlow(ctx, CreateRequest{Name: "Weg 2"})
	if err := s.DeleteFlowForMission(ctx, rec2.MissionID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetFlow(ctx, rec2.ID); !errors.Is(err, ErrNotFound) {
		t.Fatal("DeleteFlowForMission must delete the flow")
	}
	if err := s.DeleteFlowForMission(ctx, "mission_unknown"); err != nil {
		t.Fatalf("unknown missions are ignored: %v", err)
	}
}

func TestDiffFlows(t *testing.T) {
	live := simpleFlow("A")
	draft, _ := live.Clone()
	if d := DiffFlows(live, draft); d != (DiffSummary{}) {
		t.Fatalf("identical = %+v", d)
	}
	draft.Nodes[1].Position = Point{X: 5, Y: 5}
	if d := DiffFlows(live, draft); d != (DiffSummary{}) {
		t.Fatalf("moving is not a change: %+v", d)
	}
	draft.Nodes[1].Label = "neu"
	draft.Nodes = append(draft.Nodes, Node{ID: testNodeID(30), Key: "extra", Type: TypeSet})
	draft.Edges = append(draft.Edges, Edge{ID: "e_x", Source: PortRef{Node: draft.Nodes[1].ID, Port: PortOut}, Target: PortRef{Node: testNodeID(30), Port: PortIn}})
	if d := DiffFlows(live, draft); d != (DiffSummary{AddedNodes: 1, ChangedNodes: 1, ChangedEdges: 1}) {
		t.Fatalf("changes = %+v", d)
	}
	if d := DiffFlows(draft, live); d.RemovedNodes != 1 {
		t.Fatalf("removed = %+v", d)
	}
}
