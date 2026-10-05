package flows

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

type fakeMission struct {
	flowID   string
	name     string
	enabled  bool
	bindings []TriggerBinding
}

type fakeBridge struct {
	mu         sync.Mutex
	next       int
	missions   map[string]*fakeMission
	started    []RunRecord
	finished   chan RunFinishedInfo
	failCreate error
}

func newFakeBridge() *fakeBridge {
	return &fakeBridge{missions: map[string]*fakeMission{}, finished: make(chan RunFinishedInfo, 16)}
}

func (b *fakeBridge) CreateFlowMission(flowID, name string) (string, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.failCreate != nil {
		return "", b.failCreate
	}
	b.next++
	id := fmt.Sprintf("mission_%d", b.next)
	b.missions[id] = &fakeMission{flowID: flowID, name: name}
	return id, nil
}

func (b *fakeBridge) SyncFlowMission(missionID, name string, bindings []TriggerBinding) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	m := b.missions[missionID]
	if m == nil {
		return errors.New("unknown mission")
	}
	m.name, m.bindings = name, bindings
	return nil
}

func (b *fakeBridge) SetFlowMissionEnabled(missionID string, enabled bool) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	m := b.missions[missionID]
	if m == nil {
		return errors.New("unknown mission")
	}
	m.enabled = enabled
	return nil
}

func (b *fakeBridge) FlowMissionEnabled(missionID string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	m := b.missions[missionID]
	return m != nil && m.enabled
}

func (b *fakeBridge) DeleteFlowMission(missionID string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.missions, missionID)
	return nil
}

func (b *fakeBridge) FlowRunStarted(missionID string, rec RunRecord) string {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.started = append(b.started, rec)
	return "hist_" + rec.ID
}

func (b *fakeBridge) FlowRunFinished(info RunFinishedInfo) { b.finished <- info }

func (b *fakeBridge) mission(id string) (fakeMission, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	m, ok := b.missions[id]
	if !ok {
		return fakeMission{}, false
	}
	return *m, true
}

func (b *fakeBridge) startedRuns() []RunRecord {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]RunRecord(nil), b.started...)
}

func (b *fakeBridge) waitFinished(t *testing.T) RunFinishedInfo {
	t.Helper()
	select {
	case info := <-b.finished:
		return info
	case <-time.After(3 * time.Second):
		t.Fatal("no live run finished")
	}
	return RunFinishedInfo{}
}

func newServiceFixture(t *testing.T, clock Clock) (*Service, *fakeBridge) {
	t.Helper()
	bridge := newFakeBridge()
	svc := &Services{Tools: &fakeTools{}, Clock: clock, Location: time.UTC}
	s := NewService(openTestStore(t), catalogRegistry(t, fullEnv()), svc, bridge, ServiceConfig{}, discardLogger())
	t.Cleanup(func() { _ = s.Shutdown(context.Background()) })
	return s, bridge
}

// simpleFlow: manual trigger (data {"name":"Welt"}) -> set node greeting = "Hallo {{trigger.data.name}}".
func simpleFlow(name string) *Flow {
	b := newFlow(name)
	start := b.node("start", TypeTriggerManual, map[string]any{"data": map[string]any{"name": "Welt"}})
	greet := b.node("greet", TypeSet, map[string]any{"fields": []any{map[string]any{"name": "greeting", "value": "Hallo {{trigger.data.name}}"}}})
	b.edge(start, PortOut, greet)
	return b.build()
}

func waitRun(t *testing.T, s *Service, runID string) RunRecord {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for {
		detail, err := s.Run(context.Background(), runID, false)
		if err == nil && detail.Run.Status.Terminal() {
			return *detail.Run
		}
		if time.Now().After(deadline) {
			t.Fatalf("run %s did not finish", runID)
		}
		time.Sleep(5 * time.Millisecond)
	}
}
