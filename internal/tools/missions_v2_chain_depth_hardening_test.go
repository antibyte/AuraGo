package tools

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

// Task 1c-19b: mission_completed chains are bounded by a chain depth that the trigger data
// carries from run to run (maxCompletionChainDepth).

// enqueueCompletionDependentsLocked and FlowRunFinished keep the signatures that the 1c-08
// tests call (plan tests stay byte-identical). They run at chain depth 0, the depth of a run
// that no completion started.
func (m *MissionManagerV2) enqueueCompletionDependentsLocked(sourceID, result, output string, outputs json.RawMessage) int {
	return m.enqueueCompletionDependentsAtDepthLocked(sourceID, result, output, outputs, 0)
}

func (m *MissionManagerV2) FlowRunFinished(missionID, historyID, result, output string, outputs map[string]any) {
	m.FlowRunFinishedAtDepth(missionID, historyID, result, output, outputs, 0)
}

// c19bStopNote is the start of the note that a stopped chain leaves on its last mission.
var c19bStopNote = fmt.Sprintf(completionChainStoppedNote, maxCompletionChainDepth)

// c19bCompleteEveryRun registers an agent callback that completes every prompt mission run
// with success at once, the stub the other tools tests use.
func c19bCompleteEveryRun(mm *MissionManagerV2, callbacks *sync.WaitGroup) {
	mm.SetCallback(func(_ string, missionID string) {
		callbacks.Add(1)
		defer callbacks.Done()
		mm.OnMissionComplete(missionID, MissionResultSuccess, "ok")
	})
}

// c19bDriveQueue runs the agent queue every 2 ms until the test ends (Start's processQueue
// ticks every 500 ms). The cleanup stops the driver, waits until no item is queued or
// running (every dispatched callback has then called callbacks.Add) and for the callbacks.
func c19bDriveQueue(t *testing.T, mm *MissionManagerV2, callbacks *sync.WaitGroup) {
	t.Helper()
	done, stopped := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(stopped)
		ticker := time.NewTicker(2 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				mm.processNext()
			}
		}
	}()
	t.Cleanup(func() {
		close(done)
		<-stopped
		c07Await(t, "the mission queue to drain", func() bool {
			queue, running := mm.GetQueue()
			return len(queue.List()) == 0 && running == ""
		})
		callbacks.Wait()
		mm.Stop()
	})
}

// c19bFlowRuns is a FlowHooks that runs every flow it starts at once and reports it like the
// flow bridge: FlowRunStarted, then FlowRunFinishedAtDepth with the depth of the trigger
// data, on the caller's goroutine (RunNow, or the manager's event dispatcher). It records
// the depth of every run per mission.
type c19bFlowRuns struct {
	mm     *MissionManagerV2
	mu     sync.Mutex
	depths map[string][]int
}

func c19bFlowManager(t *testing.T) (*MissionManagerV2, *c19bFlowRuns) {
	t.Helper()
	mm := NewMissionManagerV2(tempSystemTaskDir(t), nil)
	hooks := &c19bFlowRuns{mm: mm, depths: map[string][]int{}}
	mm.SetFlowHooks(hooks)
	return mm, hooks
}

func (h *c19bFlowRuns) StartFlowRun(missionID, _, triggerType, data string) error {
	depth := completionChainDepthRaw(triggerType, data)
	h.mu.Lock()
	h.depths[missionID] = append(h.depths[missionID], depth)
	h.mu.Unlock()
	run := h.mm.FlowRunStarted(missionID, triggerType, data)
	h.mm.FlowRunFinishedAtDepth(missionID, run, MissionResultSuccess, "ok", map[string]any{"x": 1}, depth)
	return nil
}

func (h *c19bFlowRuns) FlowMissionDeleted(string)            {}
func (h *c19bFlowRuns) FlowEnabledChanged(string, bool)      {}
func (h *c19bFlowRuns) NextFlowRun(string) (time.Time, bool) { return time.Time{}, false }

func (h *c19bFlowRuns) runDepths(missionID string) []int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]int(nil), h.depths[missionID]...)
}

// c19bFollower publishes a flow mission that runs when sourceID completes.
func c19bFollower(t *testing.T, mm *MissionManagerV2, sourceID string) string {
	t.Helper()
	return publishTestFlow(t, mm, FlowTriggerSpec{NodeID: "n_bbbbbbbb", TriggerType: TriggerMissionCompleted,
		TriggerConfig: &TriggerConfig{SourceMissionID: sourceID}})
}

// c19bRuns returns the counted runs of the missions and whether all of them are idle.
func c19bRuns(mm *MissionManagerV2, missionIDs ...string) (int, bool) {
	n, idle := 0, true
	for _, id := range missionIDs {
		if m, ok := mm.Get(id); ok {
			n += m.RunCount
			idle = idle && m.Status == MissionStatusIdle
		}
	}
	return n, idle
}

// c19bAwaitStop waits until the missions ran want times and are idle, then checks that no
// further run follows. More than want runs fail at once.
func c19bAwaitStop(t *testing.T, mm *MissionManagerV2, want int, missionIDs ...string) {
	t.Helper()
	deadline := time.Now().Add(c07DeadlockGuard)
	for {
		n, idle := c19bRuns(mm, missionIDs...)
		if n > want {
			t.Fatalf("the missions ran %d times, more than %d: the chain was not stopped", n, want)
		}
		if n == want && idle {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("the missions ran %d of %d times (idle %v)", n, want, idle)
		}
		time.Sleep(2 * time.Millisecond)
	}
	time.Sleep(200 * time.Millisecond)
	if n, _ := c19bRuns(mm, missionIDs...); n != want {
		t.Fatalf("the missions ran %d times after stopping at %d", n, want)
	}
}

func c19bStops(logs *c07LogBuffer) int {
	return strings.Count(logs.String(), "Stopped a chain of missions")
}

// The depth helpers: 0 for other triggers, the clamped number, and 1 for mission_completed
// data without one (written before chain depths existed).
func TestC19bCompletionChainDepthReadsTheTriggerData(t *testing.T) {
	const mc = string(TriggerMissionCompleted)
	for _, c := range []struct {
		name        string
		triggerType string
		data        map[string]any
		want        int
	}{
		{"manual", "manual", map[string]any{"chain_depth": float64(5)}, 0},
		{"webhook", "webhook", nil, 0},
		{"stored depth", mc, map[string]any{"chain_depth": float64(4)}, 4},
		{"manager int", mc, map[string]any{"chain_depth": 7}, 7},
		{"fraction", mc, map[string]any{"chain_depth": 2.9}, 2},
		{"old data", mc, map[string]any{"source_mission": "m", "result": "success"}, 1},
		{"nil data", mc, nil, 1},
		{"zero", mc, map[string]any{"chain_depth": float64(0)}, 1},
		{"negative", mc, map[string]any{"chain_depth": float64(-3)}, 1},
		{"NaN", mc, map[string]any{"chain_depth": math.NaN()}, 1},
		{"huge", mc, map[string]any{"chain_depth": 1e300}, maxCompletionChainDepth},
		{"text", mc, map[string]any{"chain_depth": "3"}, 1},
	} {
		if got := CompletionChainDepth(c.triggerType, c.data); got != c.want {
			t.Errorf("%s: CompletionChainDepth = %d, want %d", c.name, got, c.want)
		}
	}
	for _, c := range []struct {
		name, triggerType, data string
		want                    int
	}{
		{"manual", "manual", `{"chain_depth":5}`, 0},
		{"restart recovery", "restart_recovery", "", 0},
		{"stored depth", mc, `{"chain_depth":3,"output":"x","result":"success","source_mission":"m"}`, 3},
		{"old queue item", mc, `{"source_mission":"m","result":"success"}`, 1},
		{"empty", mc, "", 1},
		{"not JSON", mc, "kaputt", 1},
		{"not an object", mc, `[1,2]`, 1},
		{"null", mc, `null`, 1},
		{"huge", mc, `{"chain_depth":1e9}`, maxCompletionChainDepth},
	} {
		if got := completionChainDepthRaw(c.triggerType, c.data); got != c.want {
			t.Errorf("%s: completionChainDepthRaw = %d, want %d", c.name, got, c.want)
		}
	}
}

// One point sets the depth for both kinds of dependents: a prompt mission's queue item and a
// flow's trigger data carry depth + 1. At the limit neither fires, and the source shows why.
func TestC19bDependentsRunOneDeeper(t *testing.T) {
	logs := c07CaptureWarnings(t)
	mm, hooks, _ := newFlowTestManager(t)
	source := publishTestFlow(t, mm, FlowTriggerSpec{NodeID: "n_aaaaaaaa", TriggerType: FlowTriggerManual})
	c08AddPromptDependent(mm, "c19b_prompt", source)
	follower := c19bFollower(t, mm, source)

	run := mm.FlowRunStarted(source, string(TriggerMissionCompleted), `{"chain_depth":3}`)
	mm.FlowRunFinishedAtDepth(source, run, MissionResultSuccess, "Fertig.", map[string]any{"x": 1}, 3)
	items := mm.queue.List()
	if len(items) != 1 || items[0].MissionID != "c19b_prompt" || completionChainDepthRaw(items[0].TriggerType, items[0].TriggerData) != 4 {
		t.Fatalf("queued dependents = %+v", items)
	}
	if c := hooks.waitStart(t); c.missionID != follower || completionChainDepthRaw(c.triggerType, c.data) != 4 ||
		!strings.Contains(c.data, `"chain_depth":4`) {
		t.Fatalf("follower start = %+v", c)
	}

	mm.mu.Lock()
	mm.queue.Remove("c19b_prompt")
	mm.missions["c19b_prompt"].Status = MissionStatusIdle
	mm.mu.Unlock()
	run = mm.FlowRunStarted(source, string(TriggerMissionCompleted), "")
	mm.FlowRunFinishedAtDepth(source, run, MissionResultSuccess, "Fertig.", nil, maxCompletionChainDepth)
	if items := mm.queue.List(); len(items) != 0 {
		t.Fatalf("a dependent was queued beyond the limit: %+v", items)
	}
	hooks.expectNoStart(t)
	if m, _ := mm.Get(source); m.LastOutput != c19bStopNote+"\n\nFertig." || m.LastResult != MissionResultSuccess || m.RunCount != 2 {
		t.Fatalf("source after the stop = %+v", m)
	}
	if m, _ := mm.Get("c19b_prompt"); m.Status != MissionStatusIdle {
		t.Fatalf("prompt dependent after the stop = %s", m.Status)
	}
	if n := c19bStops(logs); n != 1 || !strings.Contains(logs.String(), "mission_id="+source) {
		t.Fatalf("warnings: %s", logs.String())
	}
	saved, err := os.ReadFile(mm.file)
	if err != nil || !strings.Contains(string(saved), "Stopped a chain of missions") {
		t.Fatalf("the note was not saved: %v", err)
	}
}

// A linear chain of exactly maxCompletionChainDepth links runs whole, without a false stop;
// one more link is the first that does not run.
func TestC19bLinearChainOfMaxLinksRunsWhole(t *testing.T) {
	logs := c07CaptureWarnings(t)
	mm, hooks := c19bFlowManager(t)
	t.Cleanup(mm.Stop)
	chain := []string{publishTestFlow(t, mm, FlowTriggerSpec{NodeID: "n_aaaaaaaa", TriggerType: FlowTriggerManual})}
	for len(chain) <= maxCompletionChainDepth {
		chain = append(chain, c19bFollower(t, mm, chain[len(chain)-1]))
	}
	if err := mm.RunNow(chain[0]); err != nil {
		t.Fatalf("RunNow: %v", err)
	}
	c19bAwaitStop(t, mm, maxCompletionChainDepth+1, chain...)
	for i, id := range chain {
		m, _ := mm.Get(id)
		if depths := hooks.runDepths(id); m.RunCount != 1 || len(depths) != 1 || depths[0] != i || m.LastOutput != "ok" {
			t.Fatalf("link %d: runs %d, depths %v, output %q", i, m.RunCount, depths, m.LastOutput)
		}
	}
	if n := c19bStops(logs); n != 0 {
		t.Fatalf("a chain within the limit was stopped: %s", logs.String())
	}

	last := chain[len(chain)-1]
	extra := c19bFollower(t, mm, last)
	if err := mm.RunNow(chain[0]); err != nil {
		t.Fatalf("RunNow: %v", err)
	}
	c19bAwaitStop(t, mm, 2*(maxCompletionChainDepth+1), append(chain, extra)...)
	if m, _ := mm.Get(extra); m.RunCount != 0 {
		t.Fatalf("the link beyond the limit ran %d times", m.RunCount)
	}
	if m, _ := mm.Get(last); m.LastOutput != c19bStopNote+"\n\nok" {
		t.Fatalf("last link LastOutput = %q", m.LastOutput)
	}
	if n := c19bStops(logs); n != 1 || !strings.Contains(logs.String(), "mission_id="+last) {
		t.Fatalf("warnings: %s", logs.String())
	}
}

// A flow and an agent mission that start each other share the counter: the loop runs
// maxCompletionChainDepth + 1 times, then warns and shows the note on the flow, which ran
// at the even depths.
func TestC19bFlowAndPromptMissionLoopStops(t *testing.T) {
	logs := c07CaptureWarnings(t)
	mm, hooks := c19bFlowManager(t)
	var callbacks sync.WaitGroup
	c19bCompleteEveryRun(mm, &callbacks)
	flow := publishTestFlow(t, mm, FlowTriggerSpec{NodeID: "n_aaaaaaaa", TriggerType: FlowTriggerManual},
		FlowTriggerSpec{NodeID: "n_bbbbbbbb", TriggerType: TriggerMissionCompleted, TriggerConfig: &TriggerConfig{SourceMissionID: "c19b_agent"}})
	c08AddPromptDependent(mm, "c19b_agent", flow)
	c19bDriveQueue(t, mm, &callbacks)
	if err := mm.RunNow(flow); err != nil {
		t.Fatalf("RunNow: %v", err)
	}
	c19bAwaitStop(t, mm, maxCompletionChainDepth+1, flow, "c19b_agent")
	f, _ := mm.Get(flow)
	a, _ := mm.Get("c19b_agent")
	if f.RunCount != maxCompletionChainDepth/2+1 || a.RunCount != maxCompletionChainDepth/2 {
		t.Fatalf("runs: flow %d, agent %d", f.RunCount, a.RunCount)
	}
	if got, want := fmt.Sprint(hooks.runDepths(flow)), "[0 2 4 6 8 10]"; got != want {
		t.Fatalf("flow run depths %s, want %s", got, want)
	}
	if f.LastOutput != c19bStopNote+"\n\nok" || a.LastOutput != "ok" {
		t.Fatalf("LastOutput flow %q, agent %q", f.LastOutput, a.LastOutput)
	}
	if n := c19bStops(logs); n != 1 || !strings.Contains(logs.String(), "mission_id="+flow) {
		t.Fatalf("warnings: %s", logs.String())
	}
	mm.mu.RLock()
	left := len(mm.activeChainDepth)
	mm.mu.RUnlock()
	if left != 0 {
		t.Fatalf("%d active chain depths left after the runs", left)
	}
}

// An agent mission whose mission_completed source is itself runs maxCompletionChainDepth + 1
// times, then warns and shows the note.
func TestC19bPromptMissionOnItsOwnCompletionStops(t *testing.T) {
	logs := c07CaptureWarnings(t)
	mm := NewMissionManagerV2(tempSystemTaskDir(t), nil)
	var callbacks sync.WaitGroup
	c19bCompleteEveryRun(mm, &callbacks)
	c08AddPromptDependent(mm, "c19b_self", "c19b_self")
	c19bDriveQueue(t, mm, &callbacks)
	if err := mm.RunNow("c19b_self"); err != nil {
		t.Fatalf("RunNow: %v", err)
	}
	c19bAwaitStop(t, mm, maxCompletionChainDepth+1, "c19b_self")
	m, _ := mm.Get("c19b_self")
	if m.LastOutput != c19bStopNote+"\n\nok" || m.LastResult != MissionResultSuccess {
		t.Fatalf("mission after the stop: result %q, output %q", m.LastResult, m.LastOutput)
	}
	if n := c19bStops(logs); n != 1 || !strings.Contains(logs.String(), "mission_id=c19b_self") {
		t.Fatalf("warnings: %s", logs.String())
	}
}

// A queue item persisted before chain depths existed (mission_completed data without
// chain_depth) runs at depth 1, so its dependent is queued at depth 2.
func TestC19bOldQueueItemCountsAsDepthOne(t *testing.T) {
	mm := NewMissionManagerV2(tempSystemTaskDir(t), nil)
	t.Cleanup(mm.Stop)
	var callbacks sync.WaitGroup
	c19bCompleteEveryRun(mm, &callbacks)
	c08AddPromptDependent(mm, "c19b_old", "c19b_source")
	c08AddPromptDependent(mm, "c19b_next", "c19b_old")
	old := `{"items":[{"mission_id":"c19b_old","priority":2,"enqueued_at":"2026-10-01T08:00:00Z","trigger_type":"mission_completed",` +
		`"trigger_data":"{\"source_mission\":\"c19b_source\",\"result\":\"success\"}"}]}`
	if err := os.WriteFile(mm.queueFile, []byte(old), 0o600); err != nil {
		t.Fatal(err)
	}
	mm.mu.Lock()
	_, err := mm.loadQueueLocked()
	mm.mu.Unlock()
	if err != nil {
		t.Fatalf("loadQueueLocked: %v", err)
	}

	mm.processNext()
	var next QueueItem
	eventually(t, "the dependent of the old item", func() bool {
		for _, item := range mm.queue.List() {
			if item.MissionID == "c19b_next" {
				next = item
				return true
			}
		}
		return false
	})
	callbacks.Wait()
	if got := completionChainDepthRaw(next.TriggerType, next.TriggerData); got != 2 || !strings.Contains(next.TriggerData, `"chain_depth":2`) {
		t.Fatalf("dependent of the old item: depth %d, data %s", got, next.TriggerData)
	}
}
