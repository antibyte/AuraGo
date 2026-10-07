package flows

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"
)

// svcBridge is fakeBridge with switches for the hardening tests: SetFlowMissionEnabled
// can wait at a gate, SyncFlowMission can fail, and the calls are logged in order. Every
// field is guarded by mu, because the Service calls the bridge from several goroutines.
type svcBridge struct {
	*fakeBridge
	mu           sync.Mutex
	calls        []string
	syncFailures int
	gates        map[string]chan struct{}
	entered      chan string
	enabledReads int
	// enableDelay widens the window between a SetEnabled's read of the flow and its
	// timers, so a randomized test can catch a missing lock. It is not used for ordering.
	enableDelay time.Duration
}

func newSvcBridge() *svcBridge {
	return &svcBridge{fakeBridge: newFakeBridge(), gates: map[string]chan struct{}{}, entered: make(chan string, 8)}
}

func (b *svcBridge) record(call string) {
	b.mu.Lock()
	b.calls = append(b.calls, call)
	b.mu.Unlock()
}

func (b *svcBridge) SyncFlowMission(missionID, name string, bindings []TriggerBinding) error {
	b.mu.Lock()
	fail := b.syncFailures > 0
	if fail {
		b.syncFailures--
	}
	b.calls = append(b.calls, "sync:"+missionID)
	b.mu.Unlock()
	if fail {
		return errors.New("mission control is not reachable")
	}
	return b.fakeBridge.SyncFlowMission(missionID, name, bindings)
}

func (b *svcBridge) SetFlowMissionEnabled(missionID string, enabled bool) error {
	b.mu.Lock()
	gate, delay := b.gates[missionID], b.enableDelay
	b.mu.Unlock()
	if gate != nil {
		b.entered <- missionID
		<-gate
	}
	if delay > 0 {
		time.Sleep(delay)
	}
	err := b.fakeBridge.SetFlowMissionEnabled(missionID, enabled)
	b.record(fmt.Sprintf("enabled:%s:%v", missionID, enabled))
	return err
}

func (b *svcBridge) FlowMissionEnabled(missionID string) bool {
	b.mu.Lock()
	b.enabledReads++
	b.mu.Unlock()
	return b.fakeBridge.FlowMissionEnabled(missionID)
}

// FlowRunFinished logs the run instead of sending it on fakeBridge's channel, which
// blocks once 16 results are unread.
func (b *svcBridge) FlowRunFinished(info RunFinishedInfo) { b.record("finished:" + info.Record.ID) }

// failSyncs makes the next n SyncFlowMission calls fail.
func (b *svcBridge) failSyncs(n int) {
	b.mu.Lock()
	b.syncFailures = n
	b.mu.Unlock()
}

// hold makes SetFlowMissionEnabled for missionID announce itself on entered and wait
// until the returned release is called.
func (b *svcBridge) hold(missionID string) (release func()) {
	gate := make(chan struct{})
	b.mu.Lock()
	b.gates[missionID] = gate
	b.mu.Unlock()
	var once sync.Once
	return func() {
		once.Do(func() {
			b.mu.Lock()
			delete(b.gates, missionID)
			b.mu.Unlock()
			close(gate)
		})
	}
}

func (b *svcBridge) waitEntered(t *testing.T, missionID string) {
	t.Helper()
	select {
	case got := <-b.entered:
		if got != missionID {
			t.Fatalf("SetFlowMissionEnabled entered for %s, want %s", got, missionID)
		}
	case <-time.After(3 * time.Second):
		t.Fatalf("SetFlowMissionEnabled(%s) was not called", missionID)
	}
}

func (b *svcBridge) callLog() []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]string(nil), b.calls...)
}

func (b *svcBridge) enabledReadCount() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.enabledReads
}

func (b *svcBridge) missionCount() int {
	b.fakeBridge.mu.Lock()
	defer b.fakeBridge.mu.Unlock()
	return len(b.fakeBridge.missions)
}

// svcNewService builds a Service on a fresh store and the full catalog that is shut
// down at the end of the test.
func svcNewService(t *testing.T, services *Services, bridge MissionBridge, logger *slog.Logger) *Service {
	t.Helper()
	if logger == nil {
		logger = discardLogger()
	}
	s := NewService(openTestStore(t), catalogRegistry(t, fullEnv()), services, bridge, ServiceConfig{}, logger)
	t.Cleanup(func() { _ = s.Shutdown(context.Background()) })
	return s
}

// svcFixture is a Service on a fake clock (2026-10-03 07:00 UTC) with an svcBridge.
type svcFixture struct {
	s      *Service
	bridge *svcBridge
	clock  *fakeClock
}

func newSvcFixture(t *testing.T) *svcFixture {
	t.Helper()
	clock := newFakeClock(time.Date(2026, 10, 3, 7, 0, 0, 0, time.UTC))
	bridge := newSvcBridge()
	s := svcNewService(t, &Services{Tools: &fakeTools{}, Clock: clock, Location: time.UTC}, bridge, nil)
	return &svcFixture{s: s, bridge: bridge, clock: clock}
}

// published creates a flow, saves doc as its draft and publishes it.
func (fx *svcFixture) published(t *testing.T, name string, doc *Flow) *FlowRecord {
	t.Helper()
	ctx := context.Background()
	rec, err := fx.s.CreateFlow(ctx, CreateRequest{Name: name})
	if err != nil {
		t.Fatalf("CreateFlow: %v", err)
	}
	rev, _, err := fx.s.SaveDraft(ctx, rec.ID, doc, rec.DraftRevision)
	if err != nil {
		t.Fatalf("SaveDraft: %v", err)
	}
	pub, _, err := fx.s.Publish(ctx, rec.ID, rev)
	if err != nil {
		t.Fatalf("Publish: %v", err)
	}
	return pub
}

// svcDateTimeFlow is a manual trigger and a Date/Time trigger with node id
// testNodeID(idx) (use idx >= 3), both feeding a set node.
func svcDateTimeFlow(name string, idx int, at string) *Flow {
	b := newFlow(name)
	manual := b.node("start", TypeTriggerManual, nil)
	set := b.node("greet", TypeSet, map[string]any{"fields": []any{map[string]any{"name": "x", "value": "1"}}})
	when := testNodeID(idx)
	b.f.Nodes = append(b.f.Nodes, Node{ID: when, Key: "when", Type: TypeTriggerDateTime, TypeVersion: 1, Label: "when",
		Params: map[string]any{"at": at}})
	b.edge(manual, PortOut, set)
	b.edge(when, PortOut, set)
	return b.build()
}

// svcTimers lists the stored timers as "node@time", sorted.
func svcTimers(t *testing.T, s *Service) []string {
	t.Helper()
	list, err := s.Store().ListTimers(context.Background())
	if err != nil {
		t.Fatalf("ListTimers: %v", err)
	}
	out := make([]string, 0, len(list))
	for _, tm := range list {
		out = append(out, tm.NodeID+"@"+tm.FireAt.UTC().Format(time.RFC3339))
	}
	sort.Strings(out)
	return out
}

// svcBindingNodes lists the node ids of a mission's bindings, sorted.
func svcBindingNodes(m fakeMission) []string {
	out := make([]string, 0, len(m.bindings))
	for _, b := range m.bindings {
		out = append(out, b.NodeID)
	}
	sort.Strings(out)
	return out
}

// svcWithin runs fn on a goroutine and fails when it does not return within d.
func svcWithin(t *testing.T, d time.Duration, what string, fn func() error) error {
	t.Helper()
	done := make(chan error, 1)
	go func() { done <- fn() }()
	select {
	case err := <-done:
		return err
	case <-time.After(d):
		t.Fatalf("%s did not return within %s", what, d)
		return nil
	}
}

func TestServiceLifecycle(t *testing.T) {
	ctx := context.Background()
	t.Run("Start twice and concurrently starts one loop", func(t *testing.T) {
		s := svcNewService(t, nil, newSvcBridge(), nil)
		if err := s.Start(ctx); err != nil {
			t.Fatalf("Start: %v", err)
		}
		begin := make(chan struct{})
		errs := make(chan error, 8)
		for i := 0; i < 8; i++ {
			go func() {
				<-begin
				errs <- s.Start(ctx)
			}()
		}
		close(begin)
		for i := 0; i < 8; i++ {
			if err := <-errs; err != nil {
				t.Fatalf("a repeated Start = %v, want nil", err)
			}
		}
		// A second retention loop would close done twice and panic here.
		if err := svcWithin(t, 10*time.Second, "Shutdown", func() error { return s.Shutdown(ctx) }); err != nil {
			t.Fatalf("Shutdown: %v", err)
		}
	})
	t.Run("Start after Shutdown fails", func(t *testing.T) {
		s := svcNewService(t, nil, newSvcBridge(), nil)
		if err := s.Shutdown(ctx); err != nil {
			t.Fatalf("Shutdown: %v", err)
		}
		if err := s.Start(ctx); !errors.Is(err, ErrRunnerClosed) {
			t.Fatalf("Start after Shutdown = %v, want ErrRunnerClosed", err)
		}
	})
	t.Run("Shutdown is idempotent", func(t *testing.T) {
		s := svcNewService(t, nil, newSvcBridge(), nil)
		if err := s.Start(ctx); err != nil {
			t.Fatalf("Start: %v", err)
		}
		for i := 0; i < 3; i++ {
			if err := svcWithin(t, 10*time.Second, "Shutdown", func() error { return s.Shutdown(ctx) }); err != nil {
				t.Fatalf("Shutdown #%d: %v", i+1, err)
			}
		}
	})
	t.Run("Shutdown without Start returns at once", func(t *testing.T) {
		s := svcNewService(t, nil, newSvcBridge(), nil)
		if err := svcWithin(t, 10*time.Second, "Shutdown", func() error { return s.Shutdown(ctx) }); err != nil {
			t.Fatalf("Shutdown: %v", err)
		}
	})
}

func TestDiffFlowsNilDocumentsAndEdgeSets(t *testing.T) {
	live := simpleFlow("A")
	if d := DiffFlows(nil, nil); d != (DiffSummary{FirstPublish: true}) {
		t.Fatalf("nil, nil = %+v", d)
	}
	if d := DiffFlows(nil, live); d != (DiffSummary{FirstPublish: true, AddedNodes: 2}) {
		t.Fatalf("first publish = %+v", d)
	}
	if d := DiffFlows(live, nil); d != (DiffSummary{RemovedNodes: 2, ChangedEdges: 1}) {
		t.Fatalf("a nil draft removes everything: %+v", d)
	}
	dup, _ := live.Clone()
	dup.Edges = append(dup.Edges, Edge{ID: "e_copy", Source: dup.Edges[0].Source, Target: dup.Edges[0].Target})
	if d := DiffFlows(live, dup); d != (DiffSummary{}) {
		t.Fatalf("an edge with the same ports collapses: %+v", d)
	}
	rewired, _ := live.Clone()
	rewired.Edges[0].Source.Port = PortError
	if d := DiffFlows(live, rewired); d != (DiffSummary{ChangedEdges: 2}) {
		t.Fatalf("a rewired edge = %+v", d)
	}
}

// A flow whose draft holds a node type that is no longer registered (a removed generic
// tool) still lists, with category "unknown", and the mission switch is read once per flow.
func TestServiceListFlowsWithUnknownNodeType(t *testing.T) {
	fx := newSvcFixture(t)
	ctx := context.Background()
	rec, err := fx.s.CreateFlow(ctx, CreateRequest{Name: "Alt"})
	if err != nil {
		t.Fatal(err)
	}
	doc := simpleFlow("Alt")
	doc.ID = rec.ID
	doc.Nodes = append(doc.Nodes, Node{ID: testNodeID(40), Key: "gone", Type: "tool.removed_tool", TypeVersion: 1,
		Position: Point{X: 7, Y: 8}, Params: map[string]any{}})
	if _, err := fx.s.Store().SaveDraft(ctx, rec.ID, doc, rec.DraftRevision, fx.clock.Now()); err != nil {
		t.Fatalf("store SaveDraft: %v", err)
	}
	if _, err := fx.s.CreateFlow(ctx, CreateRequest{Name: "Zweiter"}); err != nil {
		t.Fatal(err)
	}
	before := fx.bridge.enabledReadCount()
	list, err := fx.s.ListFlows(ctx)
	if err != nil || len(list) != 2 {
		t.Fatalf("ListFlows = %d, %v", len(list), err)
	}
	if n := fx.bridge.enabledReadCount() - before; n != 2 {
		t.Fatalf("FlowMissionEnabled was called %d times for 2 flows", n)
	}
	var sum *FlowSummary
	for i := range list {
		if list[i].ID == rec.ID {
			sum = &list[i]
		}
	}
	if sum == nil || len(sum.Preview) != 3 || sum.Preview[2] != (PreviewNode{X: 7, Y: 8, Category: "unknown"}) ||
		len(sum.Triggers) != 1 || sum.Triggers[0] != TypeTriggerManual {
		t.Fatalf("summary = %+v", sum)
	}
}

func TestServiceCreateFlowImportsAndTemplates(t *testing.T) {
	fx := newSvcFixture(t)
	ctx := context.Background()

	// Colliding node ids are a validation error; nothing reaches the bridge or the store.
	clash := simpleFlow("Doppelt")
	clash.Nodes[1].ID = clash.Nodes[0].ID
	var ve *ValidationError
	if _, err := fx.s.CreateFlow(ctx, CreateRequest{Import: clash}); !errors.As(err, &ve) ||
		findIssue(ve.Issues, IssueNodeIDDuplicate, "") == nil {
		t.Fatalf("import with duplicate node ids = %v", err)
	}
	if n := fx.bridge.missionCount(); n != 0 {
		t.Fatalf("a refused import created %d missions", n)
	}
	if list, _ := fx.s.ListFlows(ctx); len(list) != 0 {
		t.Fatalf("a refused import stored %d flows", len(list))
	}

	// The imported document is copied, not adopted.
	doc := simpleFlow("Kopie")
	rec, err := fx.s.CreateFlow(ctx, CreateRequest{Import: doc})
	if err != nil || rec.ID == doc.ID || doc.ID != "flow_test" || rec.Draft.ID != rec.ID {
		t.Fatalf("import = %+v, %v (document id now %q)", rec, err, doc.ID)
	}

	// An unknown template names the id, bounded.
	id := strings.Repeat("x", 5000)
	_, err = fx.s.CreateFlow(ctx, CreateRequest{Template: id})
	if !errors.Is(err, ErrUnknownTemplate) || !strings.Contains(err.Error(), `"xxxx`) || utf8.RuneCountInString(err.Error()) > 100 {
		t.Fatalf("unknown template = %v", err)
	}
}

// A mission_completed trigger on the flow's own mission would restart the flow after
// every run; the publish dialog and Publish refuse it.
func TestServicePublishRefusesItsOwnMission(t *testing.T) {
	fx := newSvcFixture(t)
	ctx := context.Background()
	rec, err := fx.s.CreateFlow(ctx, CreateRequest{Name: "Schleife"})
	if err != nil {
		t.Fatal(err)
	}
	loop := func(source string) *Flow {
		b := newFlow("Schleife")
		done := b.node("done", TypeTriggerMission, map[string]any{"source": source})
		set := b.node("greet", TypeSet, map[string]any{"fields": []any{map[string]any{"name": "x", "value": "1"}}})
		b.edge(done, PortOut, set)
		return b.build()
	}
	rev, _, err := fx.s.SaveDraft(ctx, rec.ID, loop(rec.MissionID), rec.DraftRevision)
	if err != nil {
		t.Fatalf("a self-triggering draft must still save: %v", err)
	}
	preview, err := fx.s.PublishPreview(ctx, rec.ID)
	if err != nil || preview.CanPublish || findIssue(preview.Issues, IssueParamInvalid, testNodeID(1)) == nil {
		t.Fatalf("preview = %+v, %v", preview, err)
	}
	var ve *ValidationError
	if _, issues, err := fx.s.Publish(ctx, rec.ID, rev); !errors.As(err, &ve) {
		t.Fatalf("Publish = %v", err)
	} else if is := findIssue(issues, IssueParamInvalid, testNodeID(1)); is == nil || is.Param != "source" {
		t.Fatalf("issues = %+v", issues)
	}
	rev, _, err = fx.s.SaveDraft(ctx, rec.ID, loop("mission_other"), rev)
	if err != nil {
		t.Fatal(err)
	}
	pub, _, err := fx.s.Publish(ctx, rec.ID, rev)
	if err != nil {
		t.Fatalf("Publish with another source: %v", err)
	}
	if m, _ := fx.bridge.mission(pub.MissionID); len(m.bindings) != 1 || m.bindings[0].Config["source_mission_id"] != "mission_other" {
		t.Fatalf("bindings = %+v", m.bindings)
	}
}

func TestBindingIssueIsBounded(t *testing.T) {
	is := bindingIssue(errors.New(strings.Repeat("ä", 5000)))
	if is.Code != IssueParamInvalid || is.Severity != SeverityError || utf8.RuneCountInString(is.Message) != maxIssueMessageRunes+1 {
		t.Fatalf("issue = %s %s, %d runes", is.Code, is.Severity, utf8.RuneCountInString(is.Message))
	}
}

func TestServiceWiresTheTimerZone(t *testing.T) {
	berlin, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Skip("tzdata missing")
	}
	if loc := svcNewService(t, &Services{Location: berlin}, newSvcBridge(), nil).timers.location(); loc != berlin {
		t.Fatalf("timer zone = %v, want Europe/Berlin", loc)
	}
	if loc := svcNewService(t, nil, newSvcBridge(), nil).timers.location(); loc != time.Local {
		t.Fatalf("timer zone without a location = %v, want time.Local", loc)
	}
}

// When Mission Control cannot be updated after the store published, Publish returns the
// record and an error. Publishing the same revision again syncs the bindings and the
// timers, without a new version.
func TestServicePublishHealsAFailedMissionSync(t *testing.T) {
	fx := newSvcFixture(t)
	ctx := context.Background()
	first := fx.published(t, "Termin", svcDateTimeFlow("Termin", 10, "2026-10-04 09:00"))
	if err := fx.s.SetEnabled(ctx, first.ID, true); err != nil {
		t.Fatal(err)
	}
	if got := svcTimers(t, fx.s); len(got) != 1 || got[0] != testNodeID(10)+"@2026-10-04T09:00:00Z" {
		t.Fatalf("timers of revision 1 = %v", got)
	}
	rev, _, err := fx.s.SaveDraft(ctx, first.ID, svcDateTimeFlow("Termin", 11, "2026-10-05 10:00"), first.DraftRevision)
	if err != nil {
		t.Fatal(err)
	}

	fx.bridge.failSyncs(1)
	pub, _, err := fx.s.Publish(ctx, first.ID, rev)
	if err == nil || pub == nil || pub.LiveRevision != 2 {
		t.Fatalf("Publish with a failing sync = %+v, %v; want the new record and an error", pub, err)
	}
	m, _ := fx.bridge.mission(first.MissionID)
	if got := svcBindingNodes(m); len(got) != 2 || got[1] != testNodeID(10) {
		t.Fatalf("bindings after the failed sync = %v, want those of revision 1", got)
	}

	pub, _, err = fx.s.Publish(ctx, first.ID, rev)
	if err != nil || pub.LiveRevision != 2 {
		t.Fatalf("Publish again = %+v, %v", pub, err)
	}
	m, _ = fx.bridge.mission(first.MissionID)
	if got := svcBindingNodes(m); len(got) != 2 || got[1] != testNodeID(11) {
		t.Fatalf("bindings after the retry = %v, want those of revision 2", got)
	}
	if got := svcTimers(t, fx.s); len(got) != 1 || got[0] != testNodeID(11)+"@2026-10-05T10:00:00Z" {
		t.Fatalf("timers after the retry = %v", got)
	}
	if _, err := fx.s.Store().GetVersion(ctx, first.ID, 3); !errors.Is(err, ErrNotFound) {
		t.Fatalf("the retry must not add a version: %v", err)
	}
}
