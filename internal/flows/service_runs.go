package flows

import (
	"context"
	"time"
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
func (s *Service) StartTestRun(ctx context.Context, id string, req TestRunRequest) (StartResult, error) {
	rec, err := s.store.GetFlow(ctx, id)
	if err != nil {
		return StartResult{}, err
	}
	trigger := s.pickTrigger(rec.Draft, req.TriggerNode)
	if trigger == "" {
		return StartResult{}, ErrNoTrigger
	}
	data := req.TriggerData
	if data == nil {
		if data, err = s.TriggerSampleData(ctx, id, trigger); err != nil {
			return StartResult{}, err
		}
	} else if req.RememberData {
		if err := s.store.PutTestData(ctx, id, trigger, TestDataTriggerSample, data, s.now()); err != nil {
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

// RunNow starts the published flow from its manual trigger (or its first trigger).
func (s *Service) RunNow(ctx context.Context, id string) (StartResult, error) {
	rec, err := s.store.GetFlow(ctx, id)
	if err != nil {
		return StartResult{}, err
	}
	return s.startLive(rec, "", "manual", nil)
}

// TriggerFromMission starts a live run when Mission Control fires one of the flow's
// triggers. An empty nodeID picks the manual trigger or the first trigger.
func (s *Service) TriggerFromMission(missionID, nodeID, triggerType string, data map[string]any) (StartResult, error) {
	rec, err := s.store.GetFlowByMission(context.Background(), missionID)
	if err != nil {
		return StartResult{}, err
	}
	return s.startLive(rec, nodeID, triggerType, data)
}

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
		TriggerNode: trigger, TriggerType: triggerType, TriggerData: data})
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

// SaveTriggerSample remembers sample data for a trigger node.
func (s *Service) SaveTriggerSample(ctx context.Context, flowID, nodeID string, data map[string]any) error {
	return s.store.PutTestData(ctx, flowID, nodeID, TestDataTriggerSample, data, s.now())
}

func (s *Service) onRunStarted(rec RunRecord) {
	if rec.Mode == ModeTest {
		return
	}
	fr, err := s.store.GetFlow(context.Background(), rec.FlowID)
	if err != nil || fr.MissionID == "" {
		return
	}
	historyID := s.bridge.FlowRunStarted(fr.MissionID, rec)
	s.mu.Lock()
	s.history[rec.ID] = historyID
	s.mu.Unlock()
}

func (s *Service) onRunFinished(rec RunRecord, res RunResult) {
	if rec.Mode == ModeTest {
		return
	}
	s.mu.Lock()
	historyID := s.history[rec.ID]
	delete(s.history, rec.ID)
	s.mu.Unlock()
	fr, err := s.store.GetFlow(context.Background(), rec.FlowID)
	if err != nil {
		return
	}
	doc := fr.Live
	if doc == nil {
		doc = fr.Draft
	}
	s.bridge.FlowRunFinished(RunFinishedInfo{
		MissionID: fr.MissionID, HistoryID: historyID, FlowName: fr.Name,
		NotifyOnError: doc.Settings.NotifyOnError, Record: rec, Result: res, Outputs: leafOutputs(doc, res),
	})
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

func (s *Service) onTimerFired(flowID, nodeID string, scheduledFor time.Time) {
	rec, err := s.store.GetFlow(context.Background(), flowID)
	if err != nil || rec.Live == nil || !s.bridge.FlowMissionEnabled(rec.MissionID) {
		return
	}
	data := map[string]any{"scheduled_for": scheduledFor.UTC().Format(time.RFC3339)}
	if _, err := s.startLive(rec, nodeID, "datetime", data); err != nil {
		s.logger.Warn("a date and time trigger could not start its flow", "flow", flowID, "node", nodeID, "error", err)
	}
}

func (s *Service) onTimerMissed(flowID, nodeID string, scheduledFor time.Time) {
	s.logger.Warn("a date and time trigger was missed while AuraGo was off", "flow", flowID, "node", nodeID,
		"scheduled_for", scheduledFor.Format(time.RFC3339))
}
