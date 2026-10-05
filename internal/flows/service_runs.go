package flows

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"
)

// ErrTestDataTooLarge refuses trigger sample data to remember (SaveTriggerSample,
// StartTestRun with RememberData) whose JSON encoding exceeds MaxStoredOutputBytes. The
// store keeps test data without a limit of its own.
var ErrTestDataTooLarge = fmt.Errorf("the test data is larger than %d KiB", MaxStoredOutputBytes>>10)

// Trigger types stored with a run. TriggerFromMission accepts only maxTriggerTypeBytes
// of [a-z0-9_] from Mission Control and records anything else as unknownTriggerType.
const (
	maxTriggerTypeBytes = 40
	unknownTriggerType  = "unknown"
)

// TestRunRequest starts a test run of the draft. Without TriggerData the remembered
// sample (or the trigger's built-in sample) is used.
type TestRunRequest struct {
	TriggerNode  string
	TriggerData  map[string]any
	OnlyNode     string
	RememberData bool
}

// RunDetail is a run with its steps (and optionally the document it executed).
type RunDetail struct {
	Run   *RunRecord   `json:"run"`
	Steps []StepRecord `json:"steps"`
	Doc   *Flow        `json:"doc,omitempty"`
}

// StartTestRun runs the draft from a trigger (the manual trigger or the first one by default).
//
// The draft is validated with the draft rules first, because the engine only checks
// the structure it needs to run: errors refuse the run with a *ValidationError holding
// all issues, warnings do not. An OnlyNode that is not in the draft is refused the same
// way (IssueNodeNotFound). Data to remember is refused with ErrTestDataTooLarge above
// MaxStoredOutputBytes, before anything is stored; data that fits is stored before the
// run starts, so it stays remembered when the runner then refuses the run. The
// runner's ErrQueueFull and ErrRunnerClosed are returned unchanged.
func (s *Service) StartTestRun(ctx context.Context, id string, req TestRunRequest) (StartResult, error) {
	rec, err := s.store.GetFlow(ctx, id)
	if err != nil {
		return StartResult{}, err
	}
	if issues := Validate(rec.Draft, s.reg, s.vc(ModeDraft)); HasErrors(issues) {
		return StartResult{}, &ValidationError{Issues: issues}
	}
	trigger := s.pickTrigger(rec.Draft, req.TriggerNode)
	if trigger == "" {
		return StartResult{}, ErrNoTrigger
	}
	if req.OnlyNode != "" && rec.Draft.NodeByID(req.OnlyNode) == nil {
		return StartResult{}, &ValidationError{Issues: []Issue{{Code: IssueNodeNotFound, Severity: SeverityError,
			Message: "the node to test " + quoteForError(req.OnlyNode) + " does not exist"}}}
	}
	data := req.TriggerData
	if data == nil {
		if data, err = s.TriggerSampleData(ctx, id, trigger); err != nil {
			return StartResult{}, err
		}
	} else if req.RememberData {
		if err := s.SaveTriggerSample(ctx, id, trigger, data); err != nil {
			return StartResult{}, err
		}
	}
	return s.runner.Start(StartRequest{Flow: rec.Draft, Revision: rec.DraftRevision, Mode: ModeTest,
		TriggerNode: trigger, TriggerType: "test", TriggerData: data, OnlyNode: req.OnlyNode})
}

// TriggerSampleData returns the remembered sample data of a trigger node, or its built-in sample.
func (s *Service) TriggerSampleData(ctx context.Context, flowID, nodeID string) (map[string]any, error) {
	data, ok, err := s.store.GetTestData(ctx, flowID, nodeID, TestDataTriggerSample)
	if err != nil {
		return nil, err
	}
	if ok {
		return data, nil
	}
	rec, err := s.store.GetFlow(ctx, flowID)
	if err != nil {
		return nil, err
	}
	n := rec.Draft.NodeByID(nodeID)
	if n == nil {
		return map[string]any{}, nil
	}
	return TriggerSample(n), nil
}

// RunNow starts the published flow from its manual trigger (or its first trigger). The
// runner's ErrQueueFull and ErrRunnerClosed are returned unchanged.
func (s *Service) RunNow(ctx context.Context, id string) (StartResult, error) {
	rec, err := s.store.GetFlow(ctx, id)
	if err != nil {
		return StartResult{}, err
	}
	return s.startLive(rec, "", "manual", nil)
}

// TriggerFromMission starts a live run when Mission Control fires one of the flow's
// triggers. An empty nodeID picks the manual trigger or the first trigger.
//
// triggerType is recorded with the run only when it is 1 to 40 characters of
// [a-z0-9_]; anything else is recorded as "unknown". data comes from webhooks, mail,
// MQTT and the like and is not trusted: the run header keeps at most
// MaxStoredOutputBytes of it (a {"_preview": …} beyond, see Store.CreateRun), and the
// engine replaces data whose trigger output would exceed MaxOutputBytes with {}. Until
// the run executes, the data is held in memory in full, so the caller bounds it at the
// source (NormalizeTriggerData caps raw text at 8 MiB). The runner's ErrQueueFull and
// ErrRunnerClosed are returned unchanged.
//
// It takes no flow lock (see startLive), so Mission Control may call it synchronously,
// also from inside MissionBridge.FlowRunFinished when a finished run fires dependent
// missions.
func (s *Service) TriggerFromMission(missionID, nodeID, triggerType string, data map[string]any) (StartResult, error) {
	rec, err := s.store.GetFlowByMission(context.Background(), missionID)
	if err != nil {
		return StartResult{}, err
	}
	return s.startLive(rec, nodeID, triggerType, data)
}

// startLive starts a run of rec's live revision.
//
// Starting versus deleting the flow: no start path (RunNow, TriggerFromMission,
// StartTestRun, onTimerFired) takes the flow's lock, and none needs it. Runner.Start
// records the run with Store.CreateRun, which inserts only while the flow row exists,
// and admits it, all under the runner's start lock; DeleteFlow calls Runner.CancelFlow,
// which waits for that lock, before and after the store delete. So a start either
// records its run before the store delete, and the delete has cancelled it when
// DeleteFlow returns, or it fails with ErrNotFound (the flow read, or "record run").
// The flow lock must stay out of these paths: TriggerFromMission can run inside
// MissionBridge.FlowRunFinished, which runs inside the delete of another flow while that
// flow's lock is held, and two flows deleted at once could then wait for each other.
func (s *Service) startLive(rec *FlowRecord, nodeID, triggerType string, data map[string]any) (StartResult, error) {
	if rec.Live == nil {
		return StartResult{}, ErrNotPublished
	}
	trigger := s.pickTrigger(rec.Live, nodeID)
	if trigger == "" {
		return StartResult{}, ErrNoTrigger
	}
	if data == nil {
		data = map[string]any{}
		if n := rec.Live.NodeByID(trigger); n != nil && n.Type == TypeTriggerManual {
			data = TriggerSample(n)
		}
	}
	return s.runner.Start(StartRequest{Flow: rec.Live, Revision: rec.LiveRevision, Mode: ModeLive,
		TriggerNode: trigger, TriggerType: cleanTriggerType(triggerType), TriggerData: data})
}

// cleanTriggerType returns t when it is 1 to maxTriggerTypeBytes characters of
// [a-z0-9_], and unknownTriggerType otherwise. The type is stored with the run and shown
// in the run list, and Mission Control passes whatever its trigger says.
func cleanTriggerType(t string) string {
	if t == "" || len(t) > maxTriggerTypeBytes {
		return unknownTriggerType
	}
	for i := 0; i < len(t); i++ {
		c := t[i]
		if (c < 'a' || c > 'z') && (c < '0' || c > '9') && c != '_' {
			return unknownTriggerType
		}
	}
	return t
}

// pickTrigger returns nodeID when it is an enabled trigger; with an empty nodeID the manual
// trigger, else the first enabled trigger; "" when none fits.
func (s *Service) pickTrigger(f *Flow, nodeID string) string {
	isTrigger := func(n *Node) bool {
		def, ok := s.reg.Lookup(n.Type)
		return ok && def.Trigger && !n.Settings.Disabled
	}
	if nodeID != "" {
		if n := f.NodeByID(nodeID); n != nil && isTrigger(n) {
			return nodeID
		}
		return ""
	}
	first := ""
	for i := range f.Nodes {
		n := &f.Nodes[i]
		if !isTrigger(n) {
			continue
		}
		if n.Type == TypeTriggerManual {
			return n.ID
		}
		if first == "" {
			first = n.ID
		}
	}
	return first
}

// Runs lists runs of a flow.
func (s *Service) Runs(ctx context.Context, flowID string, f RunFilter) ([]RunRecord, error) {
	runs, err := s.store.ListRuns(ctx, flowID, f)
	if runs == nil {
		runs = []RunRecord{}
	}
	return runs, err
}

// Run returns one run; includeDoc adds the executed document when it is still available.
func (s *Service) Run(ctx context.Context, runID string, includeDoc bool) (*RunDetail, error) {
	rec, steps, err := s.store.GetRun(ctx, runID)
	if err != nil {
		return nil, err
	}
	if steps == nil {
		steps = []StepRecord{}
	}
	detail := &RunDetail{Run: rec, Steps: steps}
	if includeDoc {
		if doc, err := s.store.GetRunDoc(ctx, runID); err == nil {
			detail.Doc = doc
		}
	}
	return detail, nil
}

// Cancel stops or dequeues a run.
func (s *Service) Cancel(runID string) bool { return s.runner.Cancel(runID) }

// Subscribe returns the event backlog after afterSeq and a channel for new events.
func (s *Service) Subscribe(runID string, afterSeq int) ([]RunEvent, <-chan RunEvent, func(), bool) {
	return s.runner.Subscribe(runID, afterSeq)
}

// SaveTriggerSample remembers sample data for a trigger node. Data whose JSON encoding,
// as the store writes it (HTML-escaped, so "<" takes six bytes), exceeds
// MaxStoredOutputBytes is refused with ErrTestDataTooLarge (wrapped), data that cannot
// be encoded with an error; nothing is stored then.
func (s *Service) SaveTriggerSample(ctx context.Context, flowID, nodeID string, data map[string]any) error {
	encoded, err := marshalMap(data) // the encoding PutTestData stores
	if err != nil {
		return fmt.Errorf("the test data cannot be stored as JSON: %w", err)
	}
	if len(encoded) > MaxStoredOutputBytes {
		return fmt.Errorf("%w (%d bytes)", ErrTestDataTooLarge, len(encoded))
	}
	return s.store.PutTestData(ctx, flowID, nodeID, TestDataTriggerSample, data, s.now())
}

// runHistory is what onRunStarted learned for onRunFinished. onRunStarted stores one
// for every live run that starts, so a run without one never started. Keeping the
// mission and the name here lets onRunFinished complete the history entry, with the
// flow's name, when the flow is gone by then.
type runHistory struct {
	known     bool // the flow's mission and name were read at the start
	reported  bool // FlowRunStarted was called; historyID may still be empty
	missionID string
	flowName  string
	historyID string
}

// The run hooks and the timer callbacks below run on the runner's and the timer
// service's goroutines, and the run hooks also inside Runner.CancelFlow, Cancel and
// Shutdown, so inside a DeleteFlow that holds the flow's lock. They never take a flow
// lock (that would deadlock against the delete), call the bridge only without s.mu, and
// treat a flow that is gone as normal: it was deleted meanwhile, which is logged at
// Debug; other store errors are logged at Warn.

// onRunStarted records a live run in the mission history. It reads only the flow's
// mission and name (no documents), and remembers them for onRunFinished; the entry it
// stores also marks the run as started.
func (s *Service) onRunStarted(rec RunRecord) {
	if rec.Mode == ModeTest {
		return
	}
	var h runHistory
	missionID, name, err := s.store.flowMissionAndName(context.Background(), rec.FlowID)
	if err != nil {
		s.logLookup("the flow of a started run could not be read; Mission Control does not record the run", rec.ID, err)
	} else {
		h.known, h.missionID, h.flowName = true, missionID, name
		if missionID != "" {
			h.historyID = s.bridge.FlowRunStarted(missionID, bridgeRecord(rec))
			h.reported = true
		}
	}
	s.mu.Lock()
	s.history[rec.ID] = h
	s.mu.Unlock()
}

// onRunFinished tells Mission Control that a live run ended. It reports every live run,
// also when the flow or the run's document is gone, so no history entry stays
// "running". For a run that executed, the outputs and the notify setting come from the
// document the run executed (it may differ from the live revision by now), else from
// the live revision, else from the draft; without any document the outputs are empty.
// A run that never started has no outputs: it reads no document, only the flow's
// mission and name, because it ends inside Runner.CancelFlow (DeleteFlow, with the flow
// lock held), Cancel or Shutdown, once per queued run.
func (s *Service) onRunFinished(rec RunRecord, res RunResult) {
	if rec.Mode == ModeTest {
		return
	}
	s.mu.Lock()
	h, executed := s.history[rec.ID]
	delete(s.history, rec.ID)
	s.mu.Unlock()
	ctx := context.Background()
	info := RunFinishedInfo{MissionID: h.missionID, HistoryID: h.historyID, FlowName: h.flowName, Started: h.reported,
		Record: bridgeRecord(rec), Result: res, Outputs: map[string]any{}}
	if !executed {
		if missionID, name, err := s.store.flowMissionAndName(ctx, rec.FlowID); err != nil {
			s.logLookup("the flow of a run that never started could not be read", rec.ID, err)
		} else {
			info.MissionID, info.FlowName = missionID, name
		}
		s.bridge.FlowRunFinished(info)
		return
	}
	doc, err := s.store.GetRunDoc(ctx, rec.ID)
	if err != nil {
		s.logLookup("the document of a finished run could not be read", rec.ID, err)
		doc = nil
	}
	switch {
	case doc == nil:
		fr, err := s.store.GetFlow(ctx, rec.FlowID)
		if err != nil {
			s.logLookup("the flow of a finished run could not be read", rec.ID, err)
			break
		}
		if doc = fr.Live; doc == nil {
			doc = fr.Draft
		}
		if !h.known {
			info.MissionID, info.FlowName = fr.MissionID, fr.Name
		}
	case !h.known:
		if missionID, name, err := s.store.flowMissionAndName(ctx, rec.FlowID); err != nil {
			s.logLookup("the flow of a finished run could not be read", rec.ID, err)
		} else {
			info.MissionID, info.FlowName = missionID, name
		}
	}
	if doc != nil {
		if info.FlowName == "" {
			info.FlowName = doc.Name
		}
		info.NotifyOnError = doc.Settings.NotifyOnError
		info.Outputs = leafOutputs(doc, res)
	}
	s.bridge.FlowRunFinished(info)
}

// bridgeRecord returns rec with its trigger data bounded like the run header (see
// Store.CreateRun): data whose JSON encoding exceeds MaxStoredOutputBytes becomes
// {"_preview": …}, data that cannot be encoded becomes {}. The data comes from webhooks,
// mail or MQTT, and the bridge keeps the record in the mission history. rec's map is not
// modified; data within the bound is the run's own map, which the bridge only reads.
func bridgeRecord(rec RunRecord) RunRecord {
	if rec.TriggerData == nil {
		return rec
	}
	data, err := json.Marshal(rec.TriggerData)
	if err != nil {
		rec.TriggerData = map[string]any{}
		return rec
	}
	rec.TriggerData, _ = storedOutput(rec.TriggerData, data)
	return rec
}

// logLookup logs a failed read in a run hook or timer callback: at Debug when the flow
// or run is gone (deleted meanwhile, or a pruned version), else at Warn.
func (s *Service) logLookup(msg, runID string, err error, attrs ...any) {
	level := slog.LevelWarn
	if errors.Is(err, ErrNotFound) || errors.Is(err, ErrRunNotFound) {
		level = slog.LevelDebug
	}
	if runID != "" {
		attrs = append([]any{"run", runID}, attrs...)
	}
	s.logger.Log(context.Background(), level, msg, append(attrs, "error", err)...)
}

// leafOutputs returns the outputs of nodes without successors, keyed by node key.
func leafOutputs(f *Flow, res RunResult) map[string]any {
	g := buildGraph(f)
	out := map[string]any{}
	for _, id := range g.order {
		if len(g.outgoing[id]) > 0 {
			continue
		}
		n := g.nodes[id]
		if o, ok := res.Outputs[n.Key]; ok {
			out[n.Key] = o
		}
	}
	return out
}

// onTimerFired starts a live run for a due Date/Time trigger of an enabled flow.
func (s *Service) onTimerFired(flowID, nodeID string, scheduledFor time.Time) {
	rec, err := s.store.GetFlow(context.Background(), flowID)
	if err != nil {
		s.logLookup("a date and time trigger could not read its flow", "", err, "flow", flowID, "node", nodeID)
		return
	}
	if rec.Live == nil || !s.bridge.FlowMissionEnabled(rec.MissionID) {
		return
	}
	data := map[string]any{"scheduled_for": scheduledFor.UTC().Format(time.RFC3339)}
	if _, err := s.startLive(rec, nodeID, "datetime", data); err != nil {
		// ErrNotFound: the flow was deleted after the read, before the run was recorded.
		s.logLookup("a date and time trigger could not start its flow", "", err, "flow", flowID, "node", nodeID)
	}
}

func (s *Service) onTimerMissed(flowID, nodeID string, scheduledFor time.Time) {
	s.logger.Warn("a date and time trigger was missed while AuraGo was off", "flow", flowID, "node", nodeID,
		"scheduled_for", scheduledFor.Format(time.RFC3339))
}
