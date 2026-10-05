package flows

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"math"
	"strings"
	"sync"
	"testing"
	"time"
)

// svcRunBridge is svcBridge that keeps every RunFinishedInfo, so a test can check what
// Mission Control hears. Its own fields are guarded by runMu.
type svcRunBridge struct {
	*svcBridge
	runMu  sync.Mutex
	infos  []RunFinishedInfo
	notify chan struct{}
}

func newSvcRunBridge() *svcRunBridge {
	return &svcRunBridge{svcBridge: newSvcBridge(), notify: make(chan struct{}, 1)}
}

func (b *svcRunBridge) FlowRunFinished(info RunFinishedInfo) {
	b.runMu.Lock()
	b.infos = append(b.infos, info)
	b.runMu.Unlock()
	select {
	case b.notify <- struct{}{}:
	default: // a wake-up is pending already
	}
}

// reports returns every info reported for runID, in order.
func (b *svcRunBridge) reports(runID string) []RunFinishedInfo {
	b.runMu.Lock()
	defer b.runMu.Unlock()
	var out []RunFinishedInfo
	for _, info := range b.infos {
		if info.Record.ID == runID {
			out = append(out, info)
		}
	}
	return out
}

// waitInfo waits until runID was reported once and returns that report.
func (b *svcRunBridge) waitInfo(t *testing.T, runID string) RunFinishedInfo {
	t.Helper()
	deadline := time.After(10 * time.Second)
	for {
		if got := b.reports(runID); len(got) > 0 {
			if len(got) > 1 {
				t.Fatalf("run %s was reported %d times", runID, len(got))
			}
			return got[0]
		}
		select {
		case <-b.notify:
		case <-deadline:
			t.Fatalf("run %s was not reported to Mission Control", runID)
		}
	}
}

// svcRunObserver records every run the runner finishes, test runs included, after the
// Service's own hook has run.
type svcRunObserver struct {
	mu       sync.Mutex
	finished map[string]RunRecord
	notify   chan struct{}
}

// svcRunObserve installs the observer; call it before the first run starts.
func svcRunObserve(s *Service) *svcRunObserver {
	o := &svcRunObserver{finished: map[string]RunRecord{}, notify: make(chan struct{}, 1)}
	hook := s.runner.hooks.OnRunFinished
	s.runner.hooks.OnRunFinished = func(rec RunRecord, res RunResult) {
		if hook != nil {
			hook(rec, res)
		}
		o.mu.Lock()
		o.finished[rec.ID] = rec
		o.mu.Unlock()
		select {
		case o.notify <- struct{}{}:
		default:
		}
	}
	return o
}

func (o *svcRunObserver) get(runID string) (RunRecord, bool) {
	o.mu.Lock()
	defer o.mu.Unlock()
	rec, ok := o.finished[runID]
	return rec, ok
}

func (o *svcRunObserver) wait(t *testing.T, runID string) RunRecord {
	t.Helper()
	deadline := time.After(10 * time.Second)
	for {
		if rec, ok := o.get(runID); ok {
			return rec
		}
		select {
		case <-o.notify:
		case <-deadline:
			t.Fatalf("run %s did not finish", runID)
		}
	}
}

// svcRunNewService is a Service with the full catalog on tools, a real clock and cfg,
// shut down at the end of the test. Register a blocking tool's release after this call,
// so it runs before the Shutdown.
func svcRunNewService(t *testing.T, tools ToolInvoker, bridge MissionBridge, logger *slog.Logger, cfg ServiceConfig) *Service {
	t.Helper()
	if logger == nil {
		logger = discardLogger()
	}
	s := NewService(openTestStore(t), catalogRegistry(t, fullEnv()), &Services{Tools: tools, Location: time.UTC}, bridge, cfg, logger)
	t.Cleanup(func() { _ = s.Shutdown(context.Background()) })
	return s
}

// svcRunPublish creates a flow, saves doc as its draft and publishes it.
func svcRunPublish(t *testing.T, s *Service, doc *Flow) *FlowRecord {
	t.Helper()
	ctx := context.Background()
	rec, err := s.CreateFlow(ctx, CreateRequest{Name: doc.Name})
	if err != nil {
		t.Fatalf("CreateFlow: %v", err)
	}
	rev, _, err := s.SaveDraft(ctx, rec.ID, doc, rec.DraftRevision)
	if err != nil {
		t.Fatalf("SaveDraft: %v", err)
	}
	pub, _, err := s.Publish(ctx, rec.ID, rev)
	if err != nil {
		t.Fatalf("Publish: %v", err)
	}
	return pub
}

// svcRunSearchFlow: manual trigger "start" -> web search "search" -> set node leafKey
// with the field v = leafKey. The search is where blocking tools hold a run.
func svcRunSearchFlow(name, leafKey string) *Flow {
	b := newFlow(name)
	start := b.node("start", TypeTriggerManual, nil)
	search := b.node("search", TypeWebSearch, map[string]any{"query": "wetter"})
	leaf := b.node(leafKey, TypeSet, map[string]any{"fields": []any{map[string]any{"name": "v", "value": leafKey}}})
	b.edge(start, PortOut, search)
	b.edge(search, PortOut, leaf)
	return b.build()
}

// svcRunSampleOf returns sample data whose stored JSON encoding is exactly size bytes.
func svcRunSampleOf(t *testing.T, size int) map[string]any {
	t.Helper()
	data := map[string]any{"x": strings.Repeat("a", size-len(`{"x":""}`))}
	if encoded, err := marshalMap(data); err != nil || len(encoded) != size {
		t.Fatalf("sample encodes to %d bytes, want %d (%v)", len(encoded), size, err)
	}
	return data
}

// Remembered trigger data is capped at MaxStoredOutputBytes of compact JSON, on both
// paths that store it; a refused sample stores nothing and starts no run.
func TestServiceTestDataIsCapped(t *testing.T) {
	s := svcRunNewService(t, &fakeTools{}, newSvcRunBridge(), nil, ServiceConfig{})
	runs := svcRunObserve(s)
	ctx := context.Background()
	rec, err := s.CreateFlow(ctx, CreateRequest{Name: "Probe"})
	if err != nil {
		t.Fatal(err)
	}
	doc := simpleFlow("Probe")
	if _, _, err := s.SaveDraft(ctx, rec.ID, doc, rec.DraftRevision); err != nil {
		t.Fatal(err)
	}
	start := doc.Nodes[0].ID
	atCap, overCap := svcRunSampleOf(t, MaxStoredOutputBytes), svcRunSampleOf(t, MaxStoredOutputBytes+1)

	if err := s.SaveTriggerSample(ctx, rec.ID, start, atCap); err != nil {
		t.Fatalf("SaveTriggerSample at the cap: %v", err)
	}
	if err := s.SaveTriggerSample(ctx, rec.ID, start, overCap); !errors.Is(err, ErrTestDataTooLarge) {
		t.Fatalf("SaveTriggerSample above the cap = %v", err)
	}
	if got, err := s.TriggerSampleData(ctx, rec.ID, start); err != nil || got["x"] != atCap["x"] {
		t.Fatalf("after a refused sample the stored one must stay (err %v)", err)
	}
	if err := s.SaveTriggerSample(ctx, rec.ID, start, map[string]any{"n": math.NaN()}); err == nil || errors.Is(err, ErrTestDataTooLarge) {
		t.Fatalf("SaveTriggerSample with data that is no JSON = %v", err)
	}

	if _, err := s.StartTestRun(ctx, rec.ID, TestRunRequest{TriggerData: overCap, RememberData: true}); !errors.Is(err, ErrTestDataTooLarge) {
		t.Fatalf("StartTestRun remembering data above the cap = %v", err)
	}
	if list, _ := s.Runs(ctx, rec.ID, RunFilter{}); len(list) != 0 {
		t.Fatalf("a refused test run recorded %d runs", len(list))
	}
	if err := s.SaveTriggerSample(ctx, rec.ID, start, map[string]any{"name": "Klein"}); err != nil {
		t.Fatal(err)
	}
	res, err := s.StartTestRun(ctx, rec.ID, TestRunRequest{TriggerData: atCap, RememberData: true})
	if err != nil {
		t.Fatalf("StartTestRun remembering data at the cap: %v", err)
	}
	if run := runs.wait(t, res.RunID); run.Status != RunSuccess {
		t.Fatalf("test run at the cap = %+v", run)
	}
	if got, err := s.TriggerSampleData(ctx, rec.ID, start); err != nil || got["x"] != atCap["x"] {
		t.Fatalf("the sample at the cap was not remembered (err %v)", err)
	}
}

// The cap counts the bytes the store writes: json.Marshal escapes "<" as a six-byte
// unicode escape, so a sample of "<" reaches the cap with a sixth of the characters.
func TestServiceTestDataCapCountsTheStoredEncoding(t *testing.T) {
	s := svcRunNewService(t, &fakeTools{}, newSvcRunBridge(), nil, ServiceConfig{})
	ctx := context.Background()
	rec, err := s.CreateFlow(ctx, CreateRequest{Name: "Spitz"})
	if err != nil {
		t.Fatal(err)
	}
	doc := simpleFlow("Spitz")
	if _, _, err := s.SaveDraft(ctx, rec.ID, doc, rec.DraftRevision); err != nil {
		t.Fatal(err)
	}
	start := doc.Nodes[0].ID
	sample := func(n int) map[string]any { return map[string]any{"x": strings.Repeat("<", n)} }
	one, err := marshalMap(sample(1))
	if err != nil {
		t.Fatal(err)
	}
	perChar := len(one) - len(`{"x":""}`)
	if perChar != 6 {
		t.Fatalf("one '<' encodes to %d bytes, want the six-byte escape", perChar)
	}
	under := (MaxStoredOutputBytes - len(`{"x":""}`)) / perChar
	if err := s.SaveTriggerSample(ctx, rec.ID, start, sample(under+1)); !errors.Is(err, ErrTestDataTooLarge) {
		t.Fatalf("SaveTriggerSample of %d '<' (%d bytes stored) = %v", under+1, len(`{"x":""}`)+perChar*(under+1), err)
	}
	if err := s.SaveTriggerSample(ctx, rec.ID, start, sample(under)); err != nil {
		t.Fatalf("SaveTriggerSample of %d '<': %v", under, err)
	}
	var stored int
	if err := s.Store().db.QueryRowContext(ctx, `SELECT length(json) FROM flow_test_data WHERE flow_id = ?`, rec.ID).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if stored > MaxStoredOutputBytes || stored < MaxStoredOutputBytes-6 {
		t.Fatalf("stored %d bytes, want just under the cap of %d", stored, MaxStoredOutputBytes)
	}
}

// svcRunSizeBridge records how large the trigger data is that FlowRunStarted receives.
type svcRunSizeBridge struct {
	*svcRunBridge
	sizeMu  sync.Mutex
	started []int
}

func (b *svcRunSizeBridge) FlowRunStarted(missionID string, rec RunRecord) string {
	data, err := json.Marshal(rec.TriggerData)
	if err != nil {
		panic(err)
	}
	b.sizeMu.Lock()
	b.started = append(b.started, len(data))
	b.sizeMu.Unlock()
	return b.svcRunBridge.FlowRunStarted(missionID, rec)
}

// Untrusted trigger data reaches the bridge bounded like the run header, at the start
// and at the end, while the run itself sees all of it.
func TestServiceBridgeRecordsAreBounded(t *testing.T) {
	bridge := &svcRunSizeBridge{svcRunBridge: newSvcRunBridge()}
	s := svcRunNewService(t, &fakeTools{}, bridge, nil, ServiceConfig{})
	b := newFlow("Bote")
	start := b.node("start", TypeTriggerManual, nil)
	echo := b.node("echo", TypeSet, map[string]any{"fields": []any{map[string]any{"name": "raw", "value": "{{trigger.data.raw}}"}}})
	b.edge(start, PortOut, echo)
	pub := svcRunPublish(t, s, b.build())
	raw := strings.Repeat("a", 2<<20)
	res, err := s.TriggerFromMission(pub.MissionID, "", "webhook", map[string]any{"raw": raw})
	if err != nil {
		t.Fatal(err)
	}
	info := bridge.waitInfo(t, res.RunID)
	if out, _ := info.Outputs["echo"].(map[string]any); out["raw"] != raw || !info.Started {
		t.Fatalf("the run must see all of the data and be reported as started (%v)", info.Started)
	}
	finished, err := json.Marshal(info.Record.TriggerData)
	if err != nil {
		t.Fatal(err)
	}
	bridge.sizeMu.Lock()
	started := append([]int(nil), bridge.started...)
	bridge.sizeMu.Unlock()
	if len(started) != 1 || started[0] > MaxStoredOutputBytes || len(finished) > MaxStoredOutputBytes {
		t.Fatalf("trigger data to the bridge: %v bytes at the start, %d at the end; the cap is %d", started, len(finished), MaxStoredOutputBytes)
	}
	if _, ok := info.Record.TriggerData["_preview"].(string); !ok {
		t.Fatalf("the finished record's trigger data has %d keys, want a preview", len(info.Record.TriggerData))
	}
}

func TestBridgeRecord(t *testing.T) {
	small := map[string]any{"name": "Welt"}
	if got := bridgeRecord(RunRecord{ID: "r", TriggerData: small}); len(got.TriggerData) != 1 || got.TriggerData["name"] != "Welt" {
		t.Fatalf("small data = %v", got.TriggerData)
	}
	big := map[string]any{"raw": strings.Repeat("ü", MaxStoredOutputBytes)}
	got := bridgeRecord(RunRecord{ID: "r", TriggerData: big})
	preview, ok := got.TriggerData["_preview"].(string)
	if !ok || len(got.TriggerData) != 1 || len(preview) > storedPreviewBytes || got.ID != "r" {
		t.Fatalf("large data = %d keys, preview of %d bytes", len(got.TriggerData), len(preview))
	}
	if len(big) != 1 || len(big["raw"].(string)) != 2*MaxStoredOutputBytes {
		t.Fatal("bridgeRecord modified the run's trigger data")
	}
	if got := bridgeRecord(RunRecord{TriggerData: map[string]any{"n": math.Inf(1)}}); got.TriggerData == nil || len(got.TriggerData) != 0 {
		t.Fatalf("data that is no JSON = %v", got.TriggerData)
	}
	if got := bridgeRecord(RunRecord{}); got.TriggerData != nil {
		t.Fatalf("no data = %v", got.TriggerData)
	}
}

func TestCleanTriggerType(t *testing.T) {
	for in, want := range map[string]string{
		"api": "api", "datetime": "datetime", "mission_completed": "mission_completed", "ha_state2": "ha_state2",
		strings.Repeat("a", 40): strings.Repeat("a", 40), strings.Repeat("a", 41): "unknown",
		"": "unknown", "API": "unknown", "web-hook": "unknown", "web hook": "unknown", "üml": "unknown",
		"a\x00b": "unknown", "trigger.webhook": "unknown",
	} {
		if got := cleanTriggerType(in); got != want {
			t.Errorf("cleanTriggerType(%q) = %q, want %q", in, got, want)
		}
	}
}

// Mission Control's trigger type is a free string; only a short [a-z0-9_] name reaches
// the run record and the history, anything else is recorded as "unknown".
func TestServiceTriggerFromMissionCleansTheTriggerType(t *testing.T) {
	bridge := newSvcRunBridge()
	s := svcRunNewService(t, &fakeTools{}, bridge, nil, ServiceConfig{})
	pub := svcRunPublish(t, s, simpleFlow("Typ"))
	for in, want := range map[string]string{"webhook": "webhook", "Web Hook!" + strings.Repeat("x", 5000): "unknown"} {
		res, err := s.TriggerFromMission(pub.MissionID, "", in, map[string]any{"name": "Typ"})
		if err != nil {
			t.Fatalf("TriggerFromMission: %v", err)
		}
		info := bridge.waitInfo(t, res.RunID)
		run, err := s.Run(context.Background(), res.RunID, false)
		if err != nil || run.Run.TriggerType != want || info.Record.TriggerType != want {
			t.Fatalf("trigger type %.20q: stored %+v (%v), reported %q; want %q", in, run, err, info.Record.TriggerType, want)
		}
	}
	for _, rec := range bridge.startedRuns() {
		if rec.TriggerType != "webhook" && rec.TriggerType != "unknown" {
			t.Fatalf("FlowRunStarted saw trigger type %.20q", rec.TriggerType)
		}
	}
}

// Untrusted trigger data of 2 MiB runs in full, but the run header keeps only a preview.
func TestServiceTriggerFromMissionBoundsStoredData(t *testing.T) {
	bridge := newSvcRunBridge()
	s := svcRunNewService(t, &fakeTools{}, bridge, nil, ServiceConfig{})
	b := newFlow("Gross")
	start := b.node("start", TypeTriggerManual, nil)
	echo := b.node("echo", TypeSet, map[string]any{"fields": []any{map[string]any{"name": "raw", "value": "{{trigger.data.raw}}"}}})
	b.edge(start, PortOut, echo)
	pub := svcRunPublish(t, s, b.build())
	raw := strings.Repeat("a", 2<<20)
	res, err := s.TriggerFromMission(pub.MissionID, "", "webhook", map[string]any{"raw": raw})
	if err != nil {
		t.Fatalf("TriggerFromMission with 2 MiB: %v", err)
	}
	info := bridge.waitInfo(t, res.RunID)
	out, _ := info.Outputs["echo"].(map[string]any)
	if got, _ := out["raw"].(string); info.Result.Status != RunSuccess || got != raw {
		t.Fatalf("the run must see all of the data: status %s, %d bytes", info.Result.Status, len(got))
	}
	var stored int
	if err := s.Store().db.QueryRowContext(context.Background(),
		`SELECT length(trigger_data_json) FROM flow_runs WHERE id = ?`, res.RunID).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if stored > MaxStoredOutputBytes {
		t.Fatalf("the run header stores %d bytes of trigger data, the cap is %d", stored, MaxStoredOutputBytes)
	}
	detail, err := s.Run(context.Background(), res.RunID, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := detail.Run.TriggerData["_preview"].(string); !ok || len(detail.Run.TriggerData) != 1 {
		t.Fatalf("stored trigger data keys = %d, want only a preview", len(detail.Run.TriggerData))
	}
	if len(detail.Steps) != 2 || !detail.Steps[0].OutputTruncated || !detail.Steps[1].OutputTruncated {
		t.Fatalf("the trigger and echo steps must be stored as previews: %d steps", len(detail.Steps))
	}
}

// A test run validates the draft with the draft rules: errors refuse the run, warnings
// do not, and the node to test must exist.
func TestServiceTestRunsValidateTheDraft(t *testing.T) {
	s := svcRunNewService(t, &fakeTools{}, newSvcRunBridge(), nil, ServiceConfig{})
	runs := svcRunObserve(s)
	ctx := context.Background()
	rec, err := s.CreateFlow(ctx, CreateRequest{Name: "Pruefung"})
	if err != nil {
		t.Fatal(err)
	}
	broken := simpleFlow("Pruefung")
	broken.ID = rec.ID
	broken.Nodes[1].Key = "start" // a duplicate key, which SaveDraft would refuse
	rev, err := s.Store().SaveDraft(ctx, rec.ID, broken, rec.DraftRevision, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	var ve *ValidationError
	if _, err := s.StartTestRun(ctx, rec.ID, TestRunRequest{}); !errors.As(err, &ve) || findIssue(ve.Issues, IssueNodeKeyDuplicate, "") == nil {
		t.Fatalf("a test run of a draft with errors = %v", err)
	}
	if list, _ := s.Runs(ctx, rec.ID, RunFilter{}); len(list) != 0 {
		t.Fatalf("the refused test run recorded %d runs", len(list))
	}

	warned := simpleFlow("Pruefung")
	warned.Nodes = append(warned.Nodes, Node{ID: testNodeID(9), Key: "extra", Type: TypeSet, TypeVersion: 1,
		Params: map[string]any{"fields": []any{map[string]any{"name": "x", "value": "1"}}}})
	_, issues, err := s.SaveDraft(ctx, rec.ID, warned, rev)
	if err != nil || findIssue(issues, IssueNodeUnreachable, testNodeID(9)) == nil {
		t.Fatalf("the draft must save with an unreachable-node warning: %v %+v", err, issues)
	}
	res, err := s.StartTestRun(ctx, rec.ID, TestRunRequest{})
	if err != nil {
		t.Fatalf("a test run of a draft with warnings only: %v", err)
	}
	if run := runs.wait(t, res.RunID); run.Status != RunSuccess {
		t.Fatalf("test run with warnings = %+v", run)
	}

	if _, err := s.StartTestRun(ctx, rec.ID, TestRunRequest{OnlyNode: "n_zzzzzzzz"}); !errors.As(err, &ve) ||
		findIssue(ve.Issues, IssueNodeNotFound, "") == nil {
		t.Fatalf("a test run of a node that is not in the draft = %v", err)
	}
	res, err = s.StartTestRun(ctx, rec.ID, TestRunRequest{OnlyNode: warned.Nodes[1].ID})
	if err != nil {
		t.Fatalf("a test run of one node: %v", err)
	}
	if run := runs.wait(t, res.RunID); run.Status != RunSuccess {
		t.Fatalf("test run of one node = %+v", run)
	}
	if _, err := s.StartTestRun(ctx, "flow_missing", TestRunRequest{}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("a test run of a missing flow = %v", err)
	}
}

// The runner's refusals reach the caller unchanged, so the API can answer 429 and 503.
func TestServiceRunStartsPassRunnerErrorsOn(t *testing.T) {
	tools := newSvcBlockingTools()
	s := svcRunNewService(t, tools, newSvcRunBridge(), nil, ServiceConfig{MaxQueuedPerFlow: 1})
	t.Cleanup(tools.letGo) // runs before the Shutdown
	ctx := context.Background()
	pub := svcRunPublish(t, s, svcRunSearchFlow("Voll", "done"))
	first, err := s.RunNow(ctx, pub.ID)
	if err != nil || first.Status != StartStarted {
		t.Fatalf("first RunNow = %+v, %v", first, err)
	}
	select {
	case <-tools.called:
	case <-time.After(5 * time.Second):
		t.Fatal("the first run did not reach its tool")
	}
	if second, err := s.RunNow(ctx, pub.ID); err != nil || second.Status != StartQueued {
		t.Fatalf("second RunNow = %+v, %v", second, err)
	}
	if _, err := s.RunNow(ctx, pub.ID); !errors.Is(err, ErrQueueFull) {
		t.Fatalf("RunNow on a full queue = %v, want ErrQueueFull", err)
	}
	if _, err := s.TriggerFromMission(pub.MissionID, "", "api", nil); !errors.Is(err, ErrQueueFull) {
		t.Fatalf("TriggerFromMission on a full queue = %v, want ErrQueueFull", err)
	}

	tools.letGo()
	if err := svcWithin(t, 10*time.Second, "Shutdown", func() error { return s.Shutdown(ctx) }); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
	if _, err := s.RunNow(ctx, pub.ID); !errors.Is(err, ErrRunnerClosed) {
		t.Fatalf("RunNow after Shutdown = %v, want ErrRunnerClosed", err)
	}
	if _, err := s.TriggerFromMission(pub.MissionID, "", "api", nil); !errors.Is(err, ErrRunnerClosed) {
		t.Fatalf("TriggerFromMission after Shutdown = %v, want ErrRunnerClosed", err)
	}
	if _, err := s.StartTestRun(ctx, pub.ID, TestRunRequest{}); !errors.Is(err, ErrRunnerClosed) {
		t.Fatalf("StartTestRun after Shutdown = %v, want ErrRunnerClosed", err)
	}
}
