package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"strings"
	"testing"
	"time"

	"aurago/internal/flows"
	"aurago/internal/tools"
)

// Task 1c-19b: mission_completed chains are bounded by a chain depth that the trigger data
// carries from run to run (tools.maxCompletionChainDepth).

// c19bMaxDepth returns tools.maxCompletionChainDepth through the exported clamp.
func c19bMaxDepth() int {
	return tools.CompletionChainDepth(string(tools.TriggerMissionCompleted), map[string]any{"chain_depth": float64(math.MaxInt32)})
}

// c19bStopNote is the start of the note that a stopped chain leaves on its last mission.
func c19bStopNote() string {
	return fmt.Sprintf("Stopped a chain of missions triggered by completions after %d steps", c19bMaxDepth())
}

// c19bDoc builds a flow whose logic.set node follows a manual trigger (when manual) and
// one mission_completed trigger per source mission (the 1c-19 review's loop probe).
func c19bDoc(t *testing.T, name string, manual bool, sources ...string) *flows.Flow {
	t.Helper()
	var nodes, edges []string
	ids := []string{"n_cccccccc", "n_dddddddd", "n_eeeeeeee"}
	eids := []string{"e_cccccccc", "e_dddddddd", "e_eeeeeeee"}
	nodes = append(nodes, `{"id":"n_bbbbbbbb","key":"work","type":"logic.set","type_version":1,"label":"Set","position":{"x":300,"y":0},"params":{"fields":[{"name":"x","value":"1"}]}}`)
	if manual {
		nodes = append(nodes, `{"id":"n_aaaaaaaa","key":"start","type":"trigger.manual","type_version":1,"label":"Start","position":{"x":0,"y":0},"params":{}}`)
		edges = append(edges, `{"id":"e_aaaaaaaa","source":{"node":"n_aaaaaaaa","port":"out"},"target":{"node":"n_bbbbbbbb","port":"in"}}`)
	}
	for i, src := range sources {
		nodes = append(nodes, `{"id":"`+ids[i]+`","key":"when_`+string(rune('a'+i))+`","type":"trigger.mission_completed","type_version":1,"label":"When","position":{"x":0,"y":100},"params":{"source":"`+src+`"}}`)
		edges = append(edges, `{"id":"`+eids[i]+`","source":{"node":"`+ids[i]+`","port":"out"},"target":{"node":"n_bbbbbbbb","port":"in"}}`)
	}
	doc := `{"schema":1,"name":"` + name + `","nodes":[` + strings.Join(nodes, ",") + `],"edges":[` + strings.Join(edges, ",") + `]}`
	f, err := flows.ParseFlow([]byte(doc))
	if err != nil {
		t.Fatalf("ParseFlow: %v", err)
	}
	return f
}

// c19bPublish saves doc as the draft of rec, publishes and enables it.
func c19bPublish(t *testing.T, s *Server, rec *flows.FlowRecord, doc *flows.Flow) {
	t.Helper()
	ctx := context.Background()
	rev, issues, err := s.Flows.SaveDraft(ctx, rec.ID, doc, rec.DraftRevision)
	if err != nil {
		t.Fatalf("SaveDraft: %v %+v", err, issues)
	}
	if _, issues, err := s.Flows.Publish(ctx, rec.ID, rev); err != nil {
		t.Fatalf("Publish: %v %+v", err, issues)
	}
	if err := s.Flows.SetEnabled(ctx, rec.ID, true); err != nil {
		t.Fatalf("SetEnabled: %v", err)
	}
}

// c19bRuns returns the counted runs of the missions and whether all of them are idle.
func c19bRuns(s *Server, missionIDs ...string) (int, bool) {
	n, idle := 0, true
	for _, id := range missionIDs {
		if m, ok := s.MissionManagerV2.Get(id); ok {
			n += m.RunCount
			idle = idle && m.Status == tools.MissionStatusIdle
		}
	}
	return n, idle
}

// c19bAwaitStop waits until the missions ran want times and are idle, then checks that no
// further run follows. More than want runs fail at once.
func c19bAwaitStop(t *testing.T, s *Server, want int, missionIDs ...string) {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for {
		n, idle := c19bRuns(s, missionIDs...)
		if n > want {
			t.Fatalf("the missions ran %d times, more than %d: the chain was not stopped", n, want)
		}
		if n == want && idle {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("the missions ran %d of %d times (idle %v)", n, want, idle)
		}
		time.Sleep(5 * time.Millisecond)
	}
	time.Sleep(500 * time.Millisecond)
	if n, _ := c19bRuns(s, missionIDs...); n != want {
		t.Fatalf("the missions ran %d times after stopping at %d", n, want)
	}
}

// A ↔ B: two flows that start each other on completion run maxCompletionChainDepth + 1
// times in total; the completion that is not passed on warns and shows a note on its
// mission in Mission Control.
func TestC19bFlowLoopStopsAtTheChainLimit(t *testing.T) {
	logs := c12CaptureDefault(t, slog.LevelWarn)
	s, _ := newFlowsTestServer(t)
	a := createTestFlow(t, s, greetFlowJSON)
	b := createTestFlow(t, s, greetFlowJSON)
	c19bPublish(t, s, a, c19bDoc(t, "A", true, b.MissionID))
	c19bPublish(t, s, b, c19bDoc(t, "B", false, a.MissionID))
	if _, err := s.Flows.RunNow(context.Background(), a.ID); err != nil {
		t.Fatal(err)
	}
	max := c19bMaxDepth()
	c19bAwaitStop(t, s, max+1, a.MissionID, b.MissionID)

	// A ran at the even depths 0, 2, …, max: its last completion was not passed on.
	ma, _ := s.MissionManagerV2.Get(a.MissionID)
	mb, _ := s.MissionManagerV2.Get(b.MissionID)
	if ma.RunCount != max/2+1 || mb.RunCount != max/2 {
		t.Fatalf("runs A=%d B=%d", ma.RunCount, mb.RunCount)
	}
	if !strings.HasPrefix(ma.LastOutput, c19bStopNote()) || strings.Contains(mb.LastOutput, "Stopped a chain") {
		t.Fatalf("LastOutput A=%q B=%q", ma.LastOutput, mb.LastOutput)
	}
	out := logs.String()
	if strings.Count(out, "Stopped a chain of missions") != 1 || !strings.Contains(out, "mission_id="+a.MissionID) {
		t.Fatalf("warnings: %s", out)
	}
}

// A → (B, C) → A: a loop that fans out stops as well, at a bounded number of runs (at most
// 2^(max/2+1) - 1 runs of A plus 2^(max/2) - 1 each of B and C; per-flow queue limits can
// drop some).
func TestC19bFanOutLoopStops(t *testing.T) {
	s, _ := newFlowsTestServer(t)
	a := createTestFlow(t, s, greetFlowJSON)
	b := createTestFlow(t, s, greetFlowJSON)
	c := createTestFlow(t, s, greetFlowJSON)
	c19bPublish(t, s, a, c19bDoc(t, "A", true, b.MissionID, c.MissionID))
	c19bPublish(t, s, b, c19bDoc(t, "B", false, a.MissionID))
	c19bPublish(t, s, c, c19bDoc(t, "C", false, a.MissionID))
	if _, err := s.Flows.RunNow(context.Background(), a.ID); err != nil {
		t.Fatal(err)
	}
	half := c19bMaxDepth() / 2
	bound := (1<<(half+1) - 1) + 2*(1<<half-1)
	deadline := time.Now().Add(20 * time.Second)
	last, quietSince := -1, time.Now()
	for {
		n, idle := c19bRuns(s, a.MissionID, b.MissionID, c.MissionID)
		if n > bound {
			t.Fatalf("the fan-out loop ran %d times, more than %d", n, bound)
		}
		if n != last || !idle {
			last, quietSince = n, time.Now()
		} else if time.Since(quietSince) >= 500*time.Millisecond {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("the fan-out loop did not stop: %d runs", n)
		}
		time.Sleep(5 * time.Millisecond)
	}
	if last < c19bMaxDepth()+1 {
		t.Fatalf("the fan-out loop stopped after %d runs, before the chain limit", last)
	}
}

// The size-bound pin: mission_completed trigger data at its largest (outputs at the 64 KiB
// bound, an output text that JSON escapes six-fold) stays below flows.MaxStoredOutputBytes,
// so the follower's run record keeps chain_depth whole and the bridge reads it there, not
// from the 16 KiB history copy. The follower B, started at the maximum depth, fires no
// dependent and notes the stop. Were the record a {"_preview"}, B would count as depth 1
// and start C.
func TestC19bChainDepthSurvivesTheRunRecord(t *testing.T) {
	logs := c12CaptureDefault(t, slog.LevelWarn)
	s, _ := newFlowsTestServer(t)
	ctx := context.Background()
	src := createTestFlow(t, s, greetFlowJSON)
	b := createTestFlow(t, s, greetFlowJSON)
	c := createTestFlow(t, s, greetFlowJSON)
	c19bPublish(t, s, src, c19bDoc(t, "Quelle", true))
	c19bPublish(t, s, b, c19bDoc(t, "B", false, src.MissionID))
	c19bPublish(t, s, c, c19bDoc(t, "C", false, b.MissionID))
	max := c19bMaxDepth()

	// The source's last completion that may fire, with the largest data Mission Control
	// hands on: "<" is escaped to six bytes, in the outputs and in the output text.
	lt, _ := json.Marshal("<")
	report := strings.Repeat("<", (64<<10-len(`{"report":""}`))/(len(lt)-len(`""`)))
	mm := s.MissionManagerV2
	run := mm.FlowRunStarted(src.MissionID, "manual", "")
	mm.FlowRunFinishedAtDepth(src.MissionID, run, tools.MissionResultSuccess, strings.Repeat("<", 4000),
		map[string]any{"report": report}, max-1)
	c19bAwaitStop(t, s, 1, b.MissionID, c.MissionID)

	if mc, _ := mm.Get(c.MissionID); mc.RunCount != 0 {
		t.Fatalf("C ran %d times: B's chain depth was lost", mc.RunCount)
	}
	if mb, _ := mm.Get(b.MissionID); !strings.HasPrefix(mb.LastOutput, c19bStopNote()) {
		t.Fatalf("B LastOutput = %.200q", mb.LastOutput)
	}
	if out := logs.String(); !strings.Contains(out, "mission_id="+b.MissionID) {
		t.Fatalf("warnings: %s", out)
	}
	runs, err := s.Flows.Runs(ctx, b.ID, flows.RunFilter{Limit: 5})
	if err != nil || len(runs) != 1 {
		t.Fatalf("B runs = %d, %v", len(runs), err)
	}
	detail, err := s.Flows.Run(ctx, runs[0].ID, false)
	if err != nil {
		t.Fatal(err)
	}
	enc, _ := json.Marshal(detail.Run.TriggerData)
	if detail.Run.TriggerType != string(tools.TriggerMissionCompleted) || detail.Run.TriggerData["chain_depth"] != float64(max) {
		t.Fatalf("B's run record: type %q, chain_depth %v, %d bytes", detail.Run.TriggerType, detail.Run.TriggerData["chain_depth"], len(enc))
	}
	// The data is really the largest case, and it fits.
	t.Logf("largest mission_completed trigger data: %d bytes; the run record keeps %d", len(enc), flows.MaxStoredOutputBytes)
	if len(enc) < 70<<10 || len(enc) > flows.MaxStoredOutputBytes {
		t.Fatalf("mission_completed trigger data has %d bytes; the record keeps %d", len(enc), flows.MaxStoredOutputBytes)
	}
}
